//go:build !windows

package daemon

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/relay"
)

const shimWait = 15 * time.Second

func requireZsh(t *testing.T) {
	t.Helper()
	if _, err := os.Stat("/bin/zsh"); err != nil {
		t.Skip("/bin/zsh not present")
	}
}

// markerFiles returns the four user startup files; each appends its own name
// to orderLog, and the .zshrc exists so zsh-newuser-install stays quiet.
func markerFiles(orderLog string) map[string]string {
	m := map[string]string{}
	for _, n := range []string{".zshenv", ".zprofile", ".zshrc", ".zlogin"} {
		m[n] = "print -r -- " + n + " >> '" + orderLog + "'\n"
	}
	return m
}

// shimHome makes a temp HOME holding files (name -> content), points HOME at
// it, and unsets any ZDOTDIR the test binary inherited. It returns the
// resolved HOME path.
func shimHome(t *testing.T, files map[string]string) string {
	t.Helper()
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(home, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("ZDOTDIR", "") // registers the restore of the inherited value
	if err := os.Unsetenv("ZDOTDIR"); err != nil {
		t.Fatal(err)
	}
	return home
}

// newShimEngine returns an engine that runs the real /bin/zsh -l with the
// shim installed under a config dir whose name has a space, like
// "Application Support"; its sessions are killed in cleanup.
func newShimEngine(t *testing.T) (*SessionEngine, string) {
	t.Helper()
	requireZsh(t)
	e := NewSessionEngine()
	cfg := filepath.Join(t.TempDir(), "Application Support", "portty")
	if err := os.MkdirAll(cfg, 0700); err != nil {
		t.Fatal(err)
	}
	e.ConfigDirForTest(cfg)
	t.Cleanup(func() { killAllSessions(e) })
	return e, cfg
}

// startShimSession starts a session in workDir and returns its id.
func startShimSession(t *testing.T, e *SessionEngine, workDir string) string {
	t.Helper()
	id, err := e.CreateSession(context.Background(), "shim", workDir, 300, 24, nil)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	return id
}

// typeLine types line (a newline is appended) into the session.
func typeLine(t *testing.T, e *SessionEngine, id, line string) {
	t.Helper()
	hub, ok := e.manager.Get(id)
	if !ok {
		t.Fatal("hub not found")
	}
	if err := hub.OpenInput(nil).Write([]byte(line + "\n")); err != nil {
		t.Fatalf("input Write: %v", err)
	}
}

// rawOutput is everything the session printed so far, escape bytes included.
func rawOutput(t *testing.T, e *SessionEngine, id string) string {
	t.Helper()
	hub, ok := e.manager.Get(id)
	if !ok {
		t.Fatal("hub not found")
	}
	var b strings.Builder
	for _, c := range hub.ScrollbackSnapshot() {
		if c != relay.MsgOutput {
			b.WriteByte(c)
		}
	}
	return b.String()
}

// waitShimOutput fails the test unless marker shows up in the session output.
func waitShimOutput(t *testing.T, e *SessionEngine, id, marker string) {
	t.Helper()
	if !hubReadUntil(t, e, id, marker, shimWait) {
		t.Fatalf("%q never appeared; output so far: %q", marker, rawOutput(t, e, id))
	}
}

// syncShell types cmd followed by a marker that only real output can contain
// (the typed echo shows the unexpanded $((…)) form), waits for it and returns
// the raw output so far. tag must be unique per call in one session.
func syncShell(t *testing.T, e *SessionEngine, id, cmd, tag string) string {
	t.Helper()
	typeLine(t, e, id, cmd+"; print -r -- "+tag+"_$((40+2))")
	waitShimOutput(t, e, id, tag+"_42")
	return rawOutput(t, e, id)
}

func readOrderLog(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("order.log: %v", err)
	}
	return strings.Fields(string(b))
}

// A new Portty zsh session reports its working directory with OSC 7 with no
// setup by the user, and the user's startup files run as without Portty.
func TestZshShim_SessionReportsCwd(t *testing.T) {
	requireZsh(t)
	orderLog := filepath.Join(t.TempDir(), "order.log")
	home := shimHome(t, markerFiles(orderLog))
	e, cfg := newShimEngine(t)
	id := startShimSession(t, e, home)

	out := syncShell(t, e, id, `print -r -- "Z=[$ZDOTDIR] P=[${PORTTY_ZSH_ZDOTDIR-unset}]"`, "VARS")
	if !strings.Contains(out, "\x1b]7;file:///") {
		t.Errorf("no OSC 7 report at startup; output %q", out)
	}
	if !strings.Contains(out, "Z=[] P=[unset]") {
		t.Errorf("ZDOTDIR/PORTTY_ZSH_ZDOTDIR not restored inside the shell; output %q", out)
	}

	out = syncShell(t, e, id, "cd /usr", "CDUSR")
	if !strings.Contains(out, "\x1b]7;file:///usr\x07") {
		t.Errorf("cd /usr did not report /usr; output %q", out)
	}

	if got := strings.Join(readOrderLog(t, orderLog), " "); got != ".zshenv .zprofile .zshrc .zlogin" {
		t.Errorf("startup order = %q", got)
	}

	shimDir := filepath.Join(cfg, "zsh")
	if fi, err := os.Stat(shimDir); err != nil || fi.Mode().Perm() != 0700 {
		t.Errorf("shim dir mode = %v (%v), want 0700", fi, err)
	}
	f := filepath.Join(shimDir, ".zshenv")
	if fi, err := os.Stat(f); err != nil || fi.Mode().Perm() != 0600 {
		t.Errorf(".zshenv mode = %v (%v), want 0600", fi, err)
	}
	if b, err := os.ReadFile(f); err != nil || string(b) != zshShim {
		t.Errorf(".zshenv differs from the embedded script (%v)", err)
	}
}
