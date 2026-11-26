package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/updater"
)

// install-update is internal: the GUI's update spawns it, nobody types it, so --help never lists it.
func TestInstallUpdate_HiddenFromUsage(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	usage()
	_ = w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	if strings.Contains(buf.String(), installUpdateCmd) {
		t.Errorf("usage mentions %s:\n%s", installUpdateCmd, buf.String())
	}
}

func TestInstallUpdate_LaunchModeIsCLI(t *testing.T) {
	mode, args, err := launchMode([]string{installUpdateCmd, "/x/job.json"})
	if mode != modeCLI || err != nil || len(args) != 2 || args[0] != installUpdateCmd {
		t.Errorf("launchMode = %d, %v, %v; want the CLI with the arguments kept", mode, args, err)
	}
}

func TestInstallUpdate_BadJobExits1(t *testing.T) {
	dir := t.TempDir()
	malformed := filepath.Join(dir, "malformed.json")
	if err := os.WriteFile(malformed, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	loose := filepath.Join(dir, "loose.json")
	if err := os.WriteFile(loose, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{filepath.Join(dir, "missing.json"), malformed, loose} {
		if rc := runInstallUpdate([]string{p}); rc != 1 {
			t.Errorf("%s: exit %d, want 1", filepath.Base(p), rc)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "started")); err == nil {
		t.Error("a refused job wrote the started marker")
	}
	for _, args := range [][]string{nil, {"a", "b"}} {
		if rc := runInstallUpdate(args); rc != 2 {
			t.Errorf("args %v: exit %d, want 2", args, rc)
		}
	}
}

func TestSpawnInstaller_RefusesUnderTest(t *testing.T) {
	proc, err := spawnInstaller("/usr/bin/true", filepath.Join(t.TempDir(), "job.json"))
	if err == nil || proc != nil {
		t.Errorf("spawnInstaller = %v, %v; want a refusal", proc, err)
	}
}

func TestOpenApp_RefusesUnderTest(t *testing.T) {
	if err := openApp(context.Background(), filepath.Join(t.TempDir(), "Portty.app")); err == nil {
		t.Error("openApp ran from a test binary")
	}
}

func TestBundleFromExecutable(t *testing.T) {
	home := os.Getenv("HOME") // TestMain's temporary home
	for exe, want := range map[string]string{
		"/Applications/Portty.app/Contents/MacOS/Portty":        "/Applications/Portty.app",
		home + "/Applications/Portty.app/Contents/MacOS/Portty": home + "/Applications/Portty.app",
	} {
		if got, err := bundleFromExecutable(exe); err != nil || got != want {
			t.Errorf("bundleFromExecutable(%q) = %q, %v; want %q", exe, got, err, want)
		}
	}
	for _, exe := range []string{
		"build/bin/Portty",
		"/Applications/Other.app/Contents/MacOS/Portty",
		"/Applications/Portty.app/Contents/MacOS/portty-installer",
		"/Applications/Portty.app/Contents/Resources/Portty",
		"/tmp/portty.test",
	} {
		if got, err := bundleFromExecutable(exe); err == nil {
			t.Errorf("bundleFromExecutable(%q) = %q, want an error", exe, got)
		}
	}
}

// bcReset clears the update work folder and update.log so each test starts empty.
func bcReset(t *testing.T) {
	t.Helper()
	_ = os.RemoveAll(updateWorkDir())
	_ = os.Remove(bcLogPath())
}

func bcLogPath() string { return filepath.Join(configDir(), "update.log") }

// bcFakeBundle makes a minimal Portty.app under a temp folder and returns its path.
func bcFakeBundle(t *testing.T) string {
	t.Helper()
	app := filepath.Join(t.TempDir(), "Volumes", updater.BundleName)
	exe := filepath.Join(app, updater.BundleExecutable)
	if err := os.MkdirAll(filepath.Dir(exe), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, []byte("fake"), 0o700); err != nil {
		t.Fatal(err)
	}
	return app
}

// bcSwap points every startInstaller seam at the given fakes and restores them afterwards.
func bcSwap(t *testing.T, bundle func() (string, error), copyFn func(context.Context, string, string) (string, error), spawn func(string, string) (*os.Process, error)) {
	t.Helper()
	oldB, oldC, oldS, oldW := runningBundleFunc, copyBundleFunc, spawnInstallerFunc, installerStartWaitFor
	t.Cleanup(func() {
		runningBundleFunc, copyBundleFunc, spawnInstallerFunc, installerStartWaitFor = oldB, oldC, oldS, oldW
	})
	runningBundleFunc, copyBundleFunc, spawnInstallerFunc = bundle, copyFn, spawn
}

// bcLastLogLine is the last line of update.log.
func bcLastLogLine(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(bcLogPath())
	if err != nil {
		t.Fatalf("update.log: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	return lines[len(lines)-1]
}

// bcRunDirs lists the run-* folders under updateWorkDir().
func bcRunDirs(t *testing.T) []string {
	t.Helper()
	m, err := filepath.Glob(filepath.Join(updateWorkDir(), "run-*"))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func bcStart(t *testing.T, name string, args ...string) *os.Process {
	t.Helper()
	cmd := exec.Command(name, args...)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	return cmd.Process
}

func bcJob() updater.Job {
	return updater.Job{V: updater.JobVersion, Kind: updater.KindUpdate, OldPID: os.Getpid(),
		Target: "/Applications/Portty.app", Staged: "/Applications/.Portty-update-x/Portty.app",
		OldVersion: "1.1.91", NewVersion: "1.2.0", DMG: "/tmp/portty-v1.2.0-darwin-universal.dmg"}
}

func TestStartInstaller_CopiesRunningBundle(t *testing.T) {
	bcReset(t)
	running := bcFakeBundle(t)
	var copySrc, copyDst, spawnBin, spawnJob string
	bcSwap(t,
		func() (string, error) { return running, nil },
		func(_ context.Context, src, dst string) (string, error) {
			copySrc, copyDst = src, dst
			return "", os.CopyFS(dst, os.DirFS(src))
		},
		func(bin, jobPath string) (*os.Process, error) {
			spawnBin, spawnJob = bin, jobPath
			if err := os.WriteFile(filepath.Join(filepath.Dir(jobPath), updater.StartedMarker), nil, 0o600); err != nil {
				return nil, err
			}
			return bcStart(t, "/usr/bin/true"), nil
		})
	quits := 0
	a := &App{quitFunc: func() { quits++ }}
	if err := a.startInstaller(bcJob()); err != nil {
		t.Fatalf("startInstaller: %v", err)
	}
	if copySrc != running {
		t.Errorf("copy source %q, want the running bundle %q (never j.Target)", copySrc, running)
	}
	run := filepath.Dir(copyDst)
	if filepath.Dir(run) != updateWorkDir() || !strings.HasPrefix(filepath.Base(run), "run-") || filepath.Base(copyDst) != updater.BundleName {
		t.Errorf("copy destination %q, want RUN/Portty.app with RUN a run-* folder under %q", copyDst, updateWorkDir())
	}
	if want := filepath.Join(copyDst, updater.BundleExecutable); spawnBin != want {
		t.Errorf("spawned %q, want %q", spawnBin, want)
	}
	if want := filepath.Join(run, "job.json"); spawnJob != want {
		t.Errorf("job path %q, want %q", spawnJob, want)
	}
	if _, err := os.Stat(spawnJob); err != nil {
		t.Errorf("job file: %v", err)
	}
	if quits != 1 {
		t.Errorf("quitFunc called %d times, want 1", quits)
	}
	if line := bcLastLogLine(t); !strings.HasPrefix(line, "update: installer started (copy ") || strings.Contains(line, "/") {
		t.Errorf("update.log ends with %q", line)
	}
}

func TestStartInstaller_FailureCleansUp(t *testing.T) {
	boom := errors.New("boom")
	for _, tc := range []struct {
		step     string
		bundleOK bool
		copyStep string
		spawnErr bool
		noMarker bool
	}{
		{step: "bundle"},
		{step: "copy-check", bundleOK: true, copyStep: "copy-check"},
		{step: "spawn", bundleOK: true, spawnErr: true},
		{step: "no-marker", bundleOK: true, noMarker: true},
	} {
		t.Run(tc.step, func(t *testing.T) {
			bcReset(t)
			running := bcFakeBundle(t)
			spawned := 0
			var sleeper *os.Process
			bcSwap(t,
				func() (string, error) {
					if !tc.bundleOK {
						return "", boom
					}
					return running, nil
				},
				func(_ context.Context, src, dst string) (string, error) {
					if tc.copyStep != "" {
						return tc.copyStep, boom
					}
					return "", os.CopyFS(dst, os.DirFS(src))
				},
				func(bin, jobPath string) (*os.Process, error) {
					spawned++
					if tc.spawnErr {
						return nil, boom
					}
					sleeper = bcStart(t, "/bin/sleep", "30")
					return sleeper, nil
				})
			installerStartWaitFor = 200 * time.Millisecond
			quits := 0
			a := &App{quitFunc: func() { quits++ }}
			if err := a.startInstaller(bcJob()); err == nil {
				t.Fatal("startInstaller returned nil")
			}
			if d := bcRunDirs(t); len(d) != 0 {
				t.Errorf("run folders left: %v", d)
			}
			if quits != 0 {
				t.Errorf("quitFunc called %d times on a failure", quits)
			}
			if want, got := "update: installer not started: "+tc.step, bcLastLogLine(t); got != want {
				t.Errorf("update.log ends with %q, want %q", got, want)
			}
			if tc.copyStep != "" && spawned != 0 {
				t.Error("an installer was spawned after the copy failed")
			}
			if tc.noMarker {
				// startInstaller killed and reaped it: a 30 s sleep is otherwise still running.
				if err := syscall.Kill(sleeper.Pid, 0); !errors.Is(err, syscall.ESRCH) {
					_ = sleeper.Kill()
					t.Errorf("the installer that never started is still there (kill 0: %v)", err)
				}
			}
		})
	}
}

// bcRecorder swaps updateRunner for a fake that records every call and answers from fn.
func bcRecorder(t *testing.T, fn func(name string, args []string) ([]byte, error)) *[]string {
	t.Helper()
	var calls []string
	old := updateRunner
	t.Cleanup(func() { updateRunner = old })
	updateRunner = func(_ context.Context, name string, args ...string) ([]byte, error) {
		calls = append(calls, strings.Join(append([]string{name}, args...), " "))
		return fn(name, args)
	}
	return &calls
}

func TestCopyBundle_ChecksBeforeUnquarantine(t *testing.T) {
	const src, dst = "/Volumes/Portty/Portty.app", "/cfg/update/run-1/Portty.app"
	good := "Identifier=app.portty.Portty\nTeamIdentifier=" + updater.TeamID + "\n"
	answer := func(dvv, listing string, dittoErr error) func(string, []string) ([]byte, error) {
		return func(name string, args []string) ([]byte, error) {
			switch {
			case name == "/usr/bin/ditto":
				return nil, dittoErr
			case name == "/usr/bin/codesign" && args[0] == "-dvv":
				return []byte(dvv), nil
			case name == "/usr/bin/xattr" && args[0] == "-lr":
				return []byte(listing), nil
			}
			return nil, nil
		}
	}

	calls := bcRecorder(t, answer(good, "", nil))
	if step, err := copyBundle(context.Background(), src, dst); step != "" || err != nil {
		t.Fatalf("copyBundle = %q, %v; want success", step, err)
	}
	want := []string{
		"/usr/bin/ditto " + src + " " + dst,
		"/usr/bin/codesign --verify --deep --strict " + dst,
		"/usr/bin/codesign -dvv " + dst,
		"/usr/bin/xattr -dr com.apple.quarantine " + dst,
		"/usr/bin/xattr -lr " + dst,
	}
	if got := strings.Join(*calls, "\n"); got != strings.Join(want, "\n") {
		t.Errorf("calls:\n%s\nwant:\n%s", got, strings.Join(want, "\n"))
	}
	for _, c := range (*calls)[1:] {
		if strings.Contains(c, src) {
			t.Errorf("a tool other than ditto named the running bundle: %s", c)
		}
	}

	for _, tc := range []struct {
		name, step, dvv, listing string
		dittoErr                 error
		maxCalls                 int
	}{
		{name: "ditto fails", step: "copy", dvv: good, dittoErr: errors.New("ditto"), maxCalls: 1},
		{name: "no team", step: "copy-check", dvv: "Identifier=app.portty.Portty\n", maxCalls: 3},
		{name: "flag stays", step: "quarantine", dvv: good, listing: dst + ": com.apple.quarantine: 0081;0;x;", maxCalls: 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := bcRecorder(t, answer(tc.dvv, tc.listing, tc.dittoErr))
			if step, err := copyBundle(context.Background(), src, dst); step != tc.step || err == nil {
				t.Errorf("copyBundle = %q, %v; want step %q with an error", step, err, tc.step)
			}
			if len(*calls) > tc.maxCalls {
				t.Errorf("%d calls, want at most %d: %v", len(*calls), tc.maxCalls, *calls)
			}
			for _, c := range *calls {
				if tc.step != "quarantine" && strings.HasPrefix(c, "/usr/bin/xattr") {
					t.Errorf("xattr ran on a copy that failed the copy or the check: %s", c)
				}
			}
		})
	}
}
