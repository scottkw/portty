//go:build !windows

package daemon

import (
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
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

// captureLog sends the standard logger to a buffer for one test.
func captureLog(t *testing.T) *lockedBuffer {
	t.Helper()
	buf := &lockedBuffer{}
	prev := log.Writer()
	log.SetOutput(buf)
	t.Cleanup(func() { log.SetOutput(prev) })
	return buf
}

// assertLink fails unless binDir/portty is a symlink resolving to want.
func assertLink(t *testing.T, binDir, want string) {
	t.Helper()
	link := filepath.Join(binDir, "portty")
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("%s is not a symlink: %v %v", link, fi, err)
	}
	if got, err := filepath.EvalSymlinks(link); err != nil || got != want {
		t.Fatalf("%s resolves to %q (%v), want %q", link, got, err, want)
	}
}

// The link follows the executable when it moves (an app update, a dev build).
func TestCLILink_RelinksWhenTargetChanges(t *testing.T) {
	e, cfg := newEnvEngine(t)
	binDir := filepath.Join(cfg, "bin")
	setCLIExecutable(t, fakeExe(t))
	envSession(t, e)
	exe2 := fakeExe(t)
	setCLIExecutable(t, exe2)
	envSession(t, e)
	assertLink(t, binDir, exe2)
}

// A dangling link or a regular file at bin/portty is replaced.
func TestCLILink_ReplacesStaleEntry(t *testing.T) {
	for name, plant := range map[string]func(string) error{
		"dangling symlink": func(p string) error { return os.Symlink(filepath.Join(filepath.Dir(p), "gone"), p) },
		"regular file":     func(p string) error { return os.WriteFile(p, []byte("#!/bin/sh\n"), 0700) },
	} {
		t.Run(name, func(t *testing.T) {
			exe := fakeExe(t)
			setCLIExecutable(t, exe)
			binDir := filepath.Join(t.TempDir(), "bin")
			if err := os.MkdirAll(binDir, 0700); err != nil {
				t.Fatal(err)
			}
			if err := plant(filepath.Join(binDir, "portty")); err != nil {
				t.Fatal(err)
			}
			if err := refreshCLILink(binDir); err != nil {
				t.Fatalf("refreshCLILink: %v", err)
			}
			assertLink(t, binDir, exe)
		})
	}
}

// Concurrent CreateSessions leave a working link and no temporary names.
func TestCLILink_ConcurrentRefresh(t *testing.T) {
	binDir := filepath.Join(t.TempDir(), "bin")
	exe := fakeExe(t)
	for round, target := range []string{exe, fakeExe(t)} { // first creates, second replaces
		setCLIExecutable(t, target)
		var wg sync.WaitGroup
		errs := make(chan error, 8)
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := refreshCLILink(binDir); err != nil {
					errs <- err
				}
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			t.Errorf("round %d: refreshCLILink: %v", round, err)
		}
		assertLink(t, binDir, target)
		ents, err := os.ReadDir(binDir)
		if err != nil {
			t.Fatal(err)
		}
		if len(ents) != 1 || ents[0].Name() != "portty" {
			var names []string
			for _, en := range ents {
				names = append(names, en.Name())
			}
			t.Errorf("round %d: bin dir holds %v, want only portty", round, names)
		}
	}
}

// 11-REVIEW IN-03: a daemon started from inside a Portty session inherits a PATH that
// already holds the bin dir; new sessions get it once, first, with the rest in order.
func TestCLILink_PathHasBinDirOnce(t *testing.T) {
	e, cfg := newEnvEngine(t)
	setCLIExecutable(t, fakeExe(t))
	binDir := filepath.Join(cfg, "bin")
	t.Setenv("PATH", binDir+":/usr/bin:"+binDir+":/bin")
	env := e.cliPathEnv()
	if want := "PATH=" + binDir + ":/usr/bin:/bin"; len(env) != 1 || env[0] != want {
		t.Errorf("cliPathEnv = %v, want [%s]", env, want)
	}
}

// A target inside the bin dir would make portty a link to itself (or to a
// file the link dir owns): refused, logged, no PATH entry.
func TestCLILink_RefusesTargetInsideBin(t *testing.T) {
	logs := captureLog(t)
	e, cfg := newEnvEngine(t)
	binDir := filepath.Join(cfg, "bin")
	if err := os.MkdirAll(binDir, 0700); err != nil {
		t.Fatal(err)
	}
	realBin, err := filepath.EvalSymlinks(binDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{filepath.Join(realBin, "portty"), filepath.Join(realBin, "Portty"), realBin} {
		setCLIExecutable(t, target)
		if env := e.cliPathEnv(); env != nil {
			t.Errorf("target %s: cliPathEnv = %v, want none", target, env)
		}
	}
	if _, err := os.Lstat(filepath.Join(binDir, "portty")); !os.IsNotExist(err) {
		t.Errorf("bin/portty exists after a refused refresh: %v", err)
	}
	if !logs.has("cli link: refusing target") {
		t.Errorf("no cli link log line; log = %q", logs.bytes())
	}
}

// A link failure never stops a session: it starts without the PATH entry and
// the log says why.
func TestCLILink_FailureStillSpawns(t *testing.T) {
	logs := captureLog(t)
	setCLIExecutable(t, fakeExe(t))
	e, cfg := newEnvEngine(t)
	binDir := filepath.Join(cfg, "bin")
	if err := os.MkdirAll(binDir, 0500); err != nil { // no write: the link can't be made
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(binDir, 0700) }) // before TempDir's RemoveAll (LIFO)

	id, env := envSession(t, e)

	if env["PORTTY_SESSION_ID"] != id {
		t.Errorf("session did not run env: PORTTY_SESSION_ID = %q, want %q", env["PORTTY_SESSION_ID"], id)
	}
	if strings.Contains(env["PATH"], binDir) {
		t.Errorf("session PATH %q has the bin dir after a failed link", env["PATH"])
	}
	if !logs.has("cli link: ") {
		t.Errorf("no cli link log line; log = %q", logs.bytes())
	}
}

// A real zsh login shell keeps the bin dir on its PATH after /etc/zprofile
// runs path_helper (it moves after the system dirs), so portty resolves.
// The dir name has a space, like "Application Support".
func TestCLILink_SurvivesZshLoginPathHelper(t *testing.T) {
	for _, p := range []string{"/bin/zsh", "/usr/libexec/path_helper"} {
		if _, err := os.Stat(p); err != nil {
			t.Skipf("%s missing: %v", p, err)
		}
	}
	setCLIExecutable(t, fakeExe(t))
	binDir := filepath.Join(t.TempDir(), "Application Support", "portty", "bin")
	if err := refreshCLILink(binDir); err != nil {
		t.Fatalf("refreshCLILink: %v", err)
	}
	cmd := exec.Command("/bin/zsh", "-l", "-c", "print -r -- $PATH; command -v portty")
	cmd.Env = []string{
		"PATH=" + binDir + ":/usr/bin:/bin",
		"ZDOTDIR=" + t.TempDir(), // empty: no user rc file runs
		"HOME=" + t.TempDir(),
	}
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("zsh -l: %v (output %q)", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 2 {
		t.Fatalf("zsh output = %q, want PATH and the portty path", out)
	}
	dirs := strings.Split(lines[0], ":")
	bi, ui := slices.Index(dirs, binDir), slices.Index(dirs, "/usr/bin")
	if bi < 0 || ui < 0 || bi < ui {
		t.Errorf("login PATH = %q, want %q after /usr/bin", lines[0], binDir)
	}
	if want := filepath.Join(binDir, "portty"); lines[1] != want {
		t.Errorf("command -v portty = %q, want %q", lines[1], want)
	}
}

// PREV-05 with the shim: ZDOTDIR on the real shim dir (as CreateSession sets
// it) changes nothing about where portty resolves after path_helper.
func TestCLILink_SurvivesZshLoginPathHelperWithShim(t *testing.T) {
	for _, p := range []string{"/bin/zsh", "/usr/libexec/path_helper"} {
		if _, err := os.Stat(p); err != nil {
			t.Skipf("%s missing: %v", p, err)
		}
	}
	setCLIExecutable(t, fakeExe(t))
	root := filepath.Join(t.TempDir(), "Application Support", "portty")
	binDir := filepath.Join(root, "bin")
	shimDir := filepath.Join(root, "zsh")
	if err := refreshCLILink(binDir); err != nil {
		t.Fatalf("refreshCLILink: %v", err)
	}
	if err := refreshZshShim(shimDir); err != nil {
		t.Fatalf("refreshZshShim: %v", err)
	}
	cmd := exec.Command("/bin/zsh", "-l", "-c", "print -r -- $PATH; command -v portty")
	cmd.Env = []string{
		"PATH=" + binDir + ":/usr/bin:/bin",
		"ZDOTDIR=" + shimDir,
		"PORTTY_ZSH_ZDOTDIR=",
		"HOME=" + t.TempDir(),
	}
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("zsh -l: %v (output %q)", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 2 {
		t.Fatalf("zsh output = %q, want PATH and the portty path", out)
	}
	dirs := strings.Split(lines[0], ":")
	bi, ui := slices.Index(dirs, binDir), slices.Index(dirs, "/usr/bin")
	if bi < 0 || ui < 0 || bi < ui {
		t.Errorf("login PATH = %q, want %q after /usr/bin", lines[0], binDir)
	}
	if want := filepath.Join(binDir, "portty"); lines[1] != want {
		t.Errorf("command -v portty = %q, want %q", lines[1], want)
	}
}
