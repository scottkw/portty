package updater

import (
	"cmp"
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Status is what the banner, the alerts and the tray render; the app emits it as update:status
// on every change. State: idle | checking | current | available | downloading | verifying |
// revealed. Error is set (with state available) after a failed download, verify or reveal.
type Status struct {
	State   string `json:"state"`
	Current string `json:"current"`
	Latest  string `json:"latest"`
	Done    int64  `json:"done"`
	Total   int64  `json:"total"`
	Error   string `json:"error"`
}

// Error is a failed check or download. Kind is check, download, verify or reveal; Text is shown
// to people and never holds a URL or a path.
type Error struct{ Kind, Text string }

func (e *Error) Error() string { return e.Text }

// Service is the update state machine (GIFsmith UpdateService, without Wails). Emit gets every
// status change, in order, and must not block: it runs under the service mutex.
type Service struct {
	Checker *Checker
	Current string // the running version, without the leading v
	Emit    func(Status)
	Dir     string       // the updates cache folder; "" makes Download fail
	Client  *http.Client // NewDownloadClient
	Run     Runner
	Now     func() time.Time // nil: time.Now

	checkMu sync.Mutex // one Check at a time; a second waits, then runs (a cheap 304)
	mu      sync.Mutex
	st      Status
	rel     Release // the available release; zero unless State is available or later
}

func (s *Service) status() Status {
	st := s.st
	st.State = cmp.Or(st.State, "idle")
	st.Current = s.Current
	return st
}

// set replaces the status and emits it; callers hold s.mu so emits keep their order.
func (s *Service) set(st Status) Status {
	s.st = st
	st = s.status()
	if s.Emit != nil {
		s.Emit(st)
	}
	return st
}

func (s *Service) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status()
}

func busy(state string) bool { return state == "downloading" || state == "verifying" }

// Check asks GitHub for the latest release. While a download runs it returns the status at once.
// A failure restores the previous status and returns *Error kind check.
func (s *Service) Check(ctx context.Context) (Status, error) {
	s.checkMu.Lock()
	defer s.checkMu.Unlock()
	s.mu.Lock()
	if busy(s.st.State) {
		defer s.mu.Unlock()
		return s.status(), nil
	}
	prev := s.st
	s.set(Status{State: "checking"})
	s.mu.Unlock()

	rel, found, err := s.Checker.Latest(ctx)
	newer := 0
	if found {
		newer, err = Compare(rel.Version, s.Current)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case errors.Is(err, ErrNotInstallable):
		log.Printf("update: release v%s not installable: %v", rel.Version, err)
	case err != nil:
		log.Printf("update: check failed: %v", err)
		text := "Portty couldn't reach GitHub. Check your internet connection and try again."
		if errors.Is(err, ErrRateLimited) {
			text = "GitHub is limiting update checks from this network. Try again in an hour."
		}
		return s.set(prev), &Error{Kind: "check", Text: text}
	case !found:
		log.Printf("update: no published release")
	case newer > 0:
		log.Printf("update: %s available", rel.Version)
		s.rel = rel
		return s.set(Status{State: "available", Latest: rel.Version}), nil
	default:
		log.Printf("update: up to date")
	}
	s.rel = Release{}
	return s.set(Status{State: "current"}), nil
}

// Download fetches the available release's DMG into Dir, verifies it (digest, signature, Team ID,
// notarization, quarantine) and only then reveals it in Finder. From any state but available or
// revealed it returns the status unchanged, so one operation runs at a time. It runs on its own
// 10-minute deadline, not the caller's. Every failure returns to available with Status.Error set;
// a file that failed verification is deleted, a verified one stays for a retry.
func (s *Service) Download() (Status, error) {
	s.mu.Lock()
	if s.st.State != "available" && s.st.State != "revealed" {
		defer s.mu.Unlock()
		return s.status(), nil
	}
	rel := s.rel
	s.set(Status{State: "downloading", Latest: rel.Version, Total: rel.DMG.Size})
	s.mu.Unlock()

	fail := func(kind, text string) (Status, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.set(Status{State: "available", Latest: rel.Version, Error: text}), &Error{Kind: kind, Text: text}
	}
	downloadFailed := func() (Status, error) {
		return fail("download", "Portty couldn't download version "+rel.Version+". Try again later.")
	}
	// Logs carry step names and file base names only, never a URL or a path.
	if s.Dir == "" {
		log.Printf("update: download failed: no cache folder")
		return downloadFailed()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if err := Clean(s.Dir); err != nil {
		log.Printf("update: clean failed")
	}
	path, err := Download(ctx, s.Client, rel.DMG, s.Dir, func(done, total int64) {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.set(Status{State: "downloading", Latest: rel.Version, Done: done, Total: total})
	})
	if errors.Is(err, ErrDigest) {
		log.Printf("update: verification failed: digest")
		return fail("verify", "The download didn't match the release's checksum, so Portty deleted it.")
	}
	if err != nil {
		log.Printf("update: download failed")
		return downloadFailed()
	}

	s.mu.Lock()
	s.set(Status{State: "verifying", Latest: rel.Version})
	s.mu.Unlock()
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	if err := Verify(ctx, s.Run, path, TeamID, rel.DMG.Name, now()); err != nil {
		log.Printf("update: %v", err) // names the failed step
		_ = os.Remove(path)
		return fail("verify", "The download didn't pass the signature check, so Portty deleted it.")
	}
	if err := Reveal(ctx, s.Run, path); err != nil {
		log.Printf("update: reveal failed")
		return fail("reveal", "Portty couldn't show the download in Finder.")
	}
	log.Printf("update: revealed %s", filepath.Base(path))
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.set(Status{State: "revealed", Latest: rel.Version}), nil
}
