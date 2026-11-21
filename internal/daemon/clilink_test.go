//go:build !windows

package daemon

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// setCLIExecutable points the link target seam at path for one test.
func setCLIExecutable(t *testing.T, path string) {
	t.Helper()
	prev := cliExecutable
	cliExecutable = func() (string, error) { return path, nil }
	t.Cleanup(func() { cliExecutable = prev })
}

// fakeExe creates an empty file standing in for this build's executable and
// returns its resolved path (t.TempDir is under /var, a symlink to /private/var).
func fakeExe(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "Portty")
	if err := os.WriteFile(p, nil, 0700); err != nil {
		t.Fatal(err)
	}
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// newEnvEngine returns an engine whose config dir is a temp dir and whose
// sessions run /usr/bin/env; every session is killed in cleanup.
func newEnvEngine(t *testing.T) (*SessionEngine, string) {
	t.Helper()
	e := NewSessionEngine()
	cfg := t.TempDir()
	e.ConfigDirForTest(cfg)
	e.SetSpawnCommandForTest("/usr/bin/env", nil)
	t.Cleanup(func() { killAllSessions(e) })
	return e, cfg
}

// envSession creates a session, waits for env to exit and returns the
// session id and its environment as printed.
func envSession(t *testing.T, e *SessionEngine) (string, map[string]string) {
	t.Helper()
	done := make(chan struct{}, 1)
	id, err := e.CreateSession(context.Background(), "env", "", 80, 24, func(string, int) { done <- struct{}{} })
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for env to exit")
	}
	env := map[string]string{}
	for _, line := range e.GetSessionTailLines(id, 10000) {
		if k, v, ok := strings.Cut(strings.TrimSpace(line), "="); ok {
			env[k] = v
		}
	}
	return id, env
}

// PREV-01 and PREV-05: a new session knows its id and finds portty first on
// its PATH, through a link to this build's executable.
func TestCLILink_SessionEnv(t *testing.T) {
	exe := fakeExe(t)
	setCLIExecutable(t, exe)
	e, cfg := newEnvEngine(t)
	binDir := filepath.Join(cfg, "bin")

	id, env := envSession(t, e)

	if got, err := filepath.EvalSymlinks(filepath.Join(binDir, "portty")); err != nil || got != exe {
		t.Errorf("bin/portty resolves to %q (%v), want %q", got, err, exe)
	}
	if fi, err := os.Lstat(filepath.Join(binDir, "portty")); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("bin/portty is not a symlink: %v %v", fi, err)
	}
	if fi, err := os.Stat(binDir); err != nil || fi.Mode().Perm() != 0700 {
		t.Errorf("bin dir mode = %v (%v), want 0700", fi, err)
	}
	if !strings.HasPrefix(env["PATH"], binDir+":") {
		t.Errorf("session PATH = %q, want it to start with %q", env["PATH"], binDir+":")
	}
	if env["PORTTY_SESSION_ID"] != id {
		t.Errorf("PORTTY_SESSION_ID = %q, want %q", env["PORTTY_SESSION_ID"], id)
	}
}
