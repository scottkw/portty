package updater

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// NewDownloadClient follows at most 5 redirects, each of which allow must accept. No overall
// timeout: the caller's context bounds a download.
func NewDownloadClient(allow func(*url.URL) bool) *http.Client {
	return &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("too many redirects")
		}
		if !allow(req.URL) {
			return errors.New("redirect not allowed")
		}
		return nil
	}}
}

// Download fetches a into dir/a.Name (0600, dir 0700) through a .part file. The byte count must
// equal a.Size and the SHA-256 a.SHA256 (else ErrDigest); on any error nothing is left behind.
// progress (may be nil) is called at most every 100 ms and once at the end.
func Download(ctx context.Context, client *http.Client, a Asset, dir string, progress func(done, total int64)) (path string, err error) {
	if a.Size > MaxDMGBytes {
		return "", ErrTooLarge
	}
	if a.Name == "" || filepath.Base(a.Name) != a.Name {
		return "", fmt.Errorf("%w: bad asset name", ErrNotInstallable)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	final := filepath.Join(dir, a.Name)
	part := final + ".part"
	_ = os.Remove(part)
	f, err := os.OpenFile(part, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			_ = f.Close()
			_ = os.Remove(part)
			_ = os.Remove(final)
		}
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Portty")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download: status %d", resp.StatusCode)
	}
	h := sha256.New()
	pw := &progressWriter{total: a.Size, fn: progress}
	n, err := io.Copy(io.MultiWriter(f, h, pw), io.LimitReader(resp.Body, a.Size+1))
	if err != nil {
		return "", err
	}
	if n != a.Size {
		return "", fmt.Errorf("download: got %d bytes, want %d", n, a.Size)
	}
	if hex.EncodeToString(h.Sum(nil)) != a.SHA256 {
		return "", ErrDigest
	}
	if err = f.Close(); err != nil {
		return "", err
	}
	if err = os.Rename(part, final); err != nil {
		return "", err
	}
	if progress != nil {
		progress(a.Size, a.Size)
	}
	return final, nil
}

type progressWriter struct {
	done, total int64
	last        time.Time
	fn          func(done, total int64)
}

func (p *progressWriter) Write(b []byte) (int, error) {
	p.done += int64(len(b))
	if p.fn != nil && time.Since(p.last) >= 100*time.Millisecond {
		p.last = time.Now()
		p.fn(min(p.done, p.total), p.total)
	}
	return len(b), nil
}

// Runner runs a tool by absolute path and returns its combined output.
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

// ExecRunner runs the real tool (no shell).
func ExecRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

// hasLine reports whether out has line as a whole line; whitespace around a line is ignored.
func hasLine(out []byte, line string) bool {
	return slices.ContainsFunc(strings.Split(string(out), "\n"), func(l string) bool { return strings.TrimSpace(l) == line })
}

// Verify checks the downloaded DMG's signature, Team ID, signing identifier and notarization, then
// quarantines it so it behaves like a browser download. The identifier must be assetName without
// .dmg (release.yml signs with it), which binds the file to this product and version: a genuine
// older Portty DMG or another product from the same team is refused. Every failure wraps
// ErrVerify with the step name.
func Verify(ctx context.Context, run Runner, path, teamID, assetName string, now time.Time) error {
	if _, err := run(ctx, "/usr/bin/codesign", "--verify", "--strict", path); err != nil {
		return fmt.Errorf("%w: codesign", ErrVerify)
	}
	out, err := run(ctx, "/usr/bin/codesign", "-dvv", path)
	if err != nil || !hasLine(out, "TeamIdentifier="+teamID) {
		return fmt.Errorf("%w: team", ErrVerify)
	}
	if !hasLine(out, "Identifier="+strings.TrimSuffix(assetName, ".dmg")) {
		return fmt.Errorf("%w: identifier", ErrVerify)
	}
	out, err = run(ctx, "/usr/sbin/spctl", "-a", "-t", "open", "--context", "context:primary-signature", "-vv", path)
	if err != nil || !bytes.Contains(out, []byte(": accepted")) || !hasLine(out, "source=Notarized Developer ID") {
		return fmt.Errorf("%w: spctl", ErrVerify)
	}
	if err := Quarantine(ctx, run, path, now); err != nil {
		return fmt.Errorf("%w: quarantine", ErrVerify)
	}
	return nil
}

// Quarantine flags path as downloaded by Portty at now, the way a browser would.
func Quarantine(ctx context.Context, run Runner, path string, now time.Time) error {
	flag := fmt.Sprintf("0081;%x;Portty;", now.Unix())
	_, err := run(ctx, "/usr/bin/xattr", "-w", "com.apple.quarantine", flag, path)
	return err
}

// ClearQuarantine removes com.apple.quarantine from path and everything below it. Callers do
// this only after the DMG verify chain (UPD-09) or the app signature and Team check passed.
func ClearQuarantine(ctx context.Context, run Runner, path string) error {
	_, _ = run(ctx, "/usr/bin/xattr", "-dr", "com.apple.quarantine", path)
	// ditto --noqtn does not strip the flag and xattr -dr can miss files, so the listing is the
	// proof (Pitfall 3). A listing that fails counts as a flag still there: fail closed.
	out, err := run(ctx, "/usr/bin/xattr", "-lr", path)
	if err != nil || bytes.Contains(out, []byte("com.apple.quarantine")) {
		return fmt.Errorf("%w: quarantine", ErrInstall)
	}
	return nil
}

// SafeRunner is base, except that a test binary never runs hdiutil, open, ditto or osascript:
// those mount disk images, start apps, copy over the user's files and post notifications (T-18-08). ExecRunner itself is
// unchanged for the env-gated TestRealDMG.
func SafeRunner(base Runner) Runner {
	return func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if testing.Testing() {
			switch name {
			case "/usr/bin/hdiutil", "/usr/bin/open", "/usr/bin/ditto", "/usr/bin/osascript":
				return nil, fmt.Errorf("refusing to run %s from a test binary", filepath.Base(name))
			}
		}
		return base(ctx, name, args...)
	}
}

// Reveal selects the verified DMG in Finder (open -R). UPD-03 says reveal, where GIFsmith opened
// the DMG; the quarantine flag Verify set stays, so Gatekeeper still checks the app on first run.
func Reveal(ctx context.Context, run Runner, path string) error {
	if _, err := run(ctx, "/usr/bin/open", "-R", path); err != nil {
		return fmt.Errorf("%w: %v", ErrReveal, err)
	}
	return nil
}

// Notify posts a notification titled Portty with osascript. The mover's installer uses it when no
// GUI is left to tell a failure (quick 261126-dfn): started straight from its run-* copy, outside
// LaunchServices and with no run loop, it cannot use UNUserNotificationCenter, which also only
// shows a permission prompt to an app never authorized (a first install). body goes in as argv,
// never into the script text.
func Notify(ctx context.Context, run Runner, body string) error {
	_, err := run(ctx, "/usr/bin/osascript", "-e", "on run argv", "-e",
		`display notification (item 1 of argv) with title "Portty"`, "-e", "end run", body)
	return err
}

// ResultText is the sentence a failed or rolled-back result tells the user; false for any other
// outcome.
func ResultText(r Result) (string, bool) {
	switch r.Outcome {
	case OutcomeRolledBack:
		return "The update to Portty " + r.NewVersion + " didn't take. Portty " + r.OldVersion + " is still installed.", true
	case OutcomeFailed:
		return "Portty couldn't install version " + r.NewVersion + ".", true
	}
	return "", false
}

// Clean removes Portty's own update files (every DMGAssetNames pattern and its .part) from dir;
// other files stay. A missing dir is fine.
func Clean(dir string) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	ours := func(n string) bool {
		for _, pat := range DMGAssetNames("*") {
			dmg, _ := filepath.Match(pat, n)
			part, _ := filepath.Match(pat+".part", n)
			if dmg || part {
				return true
			}
		}
		return false
	}
	for _, e := range entries {
		if n := e.Name(); ours(n) && e.Type().IsRegular() {
			if err := os.Remove(filepath.Join(dir, n)); err != nil {
				return err
			}
		}
	}
	return nil
}
