package updater

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// skipFile, in the Service's Dir, names the one version whose install rolled back or failed, so
// Check (18-02) does not offer it again automatically. The cache folder already holds its DMG.
const skipFile = "skip-version"

func rolledBackText(old, new string) string {
	return "Portty " + new + " didn't start, so Portty put version " + old + " back. The download is shown in Finder."
}

func failedText(new string) string {
	return "Portty couldn't install version " + new + ". The download is shown in Finder."
}

// ShowResult tells the user about a rolled-back or failed install the installer left behind
// (UPD-07): it records the version as skipped, then, when the kept DMG is Portty's own file in
// Dir, writes its quarantine back, reveals it and sets the revealed status with the text. False
// when nothing was revealed (any other outcome, no usable DMG, or a failed reveal).
func (s *Service) ShowResult(ctx context.Context, r Result) bool {
	var text string
	switch r.Outcome {
	case OutcomeRolledBack:
		text = rolledBackText(r.OldVersion, r.NewVersion)
	case OutcomeFailed:
		text = failedText(r.NewVersion)
	default:
		return false
	}
	s.skipVersion(r.NewVersion) // first, so the record exists even when the DMG is gone
	base := filepath.Base(r.DMG)
	if match, _ := filepath.Match("portty-v*-darwin-*.dmg", base); !match || s.Dir == "" || filepath.Dir(r.DMG) != s.Dir {
		return false
	}
	if fi, err := os.Lstat(r.DMG); err != nil || !fi.Mode().IsRegular() {
		return false
	}
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := Quarantine(ctx, s.Run, r.DMG, now()); err != nil {
		log.Printf("update: quarantine restore failed")
	}
	if err := Reveal(ctx, s.Run, r.DMG); err != nil {
		log.Printf("update: reveal failed")
		return false
	}
	log.Printf("update: revealed %s after %s install", base, r.Outcome)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rel = Release{Version: r.NewVersion, DMG: Asset{Name: base}} // so Update reveals it again (revealKept)
	s.set(Status{State: "revealed", Latest: r.NewVersion, Error: text})
	return true
}

// skipVersion writes v to the skip record through a 0600 temp file and a rename. A failure only
// logs: the worst case is one more offer of that version.
func (s *Service) skipVersion(v string) {
	if s.Dir == "" {
		log.Printf("update: skip record not written: no cache folder")
		return
	}
	if _, err := ParseVersion(v); err != nil {
		log.Printf("update: skip record not written: not a version")
		return
	}
	err := os.MkdirAll(s.Dir, 0o700)
	var f *os.File
	if err == nil {
		f, err = os.CreateTemp(s.Dir, ".skip-version-")
	}
	if err == nil {
		_, err = f.WriteString(v + "\n")
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err == nil {
			err = os.Rename(f.Name(), filepath.Join(s.Dir, skipFile))
		}
		if err != nil {
			_ = os.Remove(f.Name())
		}
	}
	if err != nil {
		log.Printf("update: skip record not written")
	}
}

// skippedVersion returns the recorded version, or ("", nil) only when there is no record. A
// record that exists but cannot be read or does not hold a version is an error (never a silent
// "nothing skipped"); its text names the failure, never the path.
func (s *Service) skippedVersion() (string, error) {
	if s.Dir == "" {
		return "", errors.New("skip record: no cache folder")
	}
	b, err := os.ReadFile(filepath.Join(s.Dir, skipFile))
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		var pe *fs.PathError
		if errors.As(err, &pe) {
			return "", fmt.Errorf("skip record: %s: %w", pe.Op, pe.Err)
		}
		return "", errors.New("skip record: unreadable")
	}
	v := strings.TrimSpace(string(b))
	if _, err := ParseVersion(v); err != nil {
		return "", errors.New("skip record: not a version")
	}
	return v, nil
}
