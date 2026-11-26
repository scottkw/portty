package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/updater"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// moverFix is an App in mover mode's reach: temp system and home folders, a fake app on a fake
// volume, fake update tools, and recorders for the dialog, the open, the job and the quit.
type moverFix struct {
	t         *testing.T
	a         *App
	system    string
	home      string
	run       updater.DMGRun
	installed string // the installed copy's CFBundleShortVersionString; "unreadable" fails plutil
	fail      string // a tool step that fails: codesign
	answer    string // the dialog's answer

	mu      sync.Mutex
	calls   []string
	dialogs []runtime.MessageDialogOptions
	opened  []string
	jobs    []updater.Job
	quit    chan struct{}
}

func moverBundle(t *testing.T, dir string) string {
	t.Helper()
	app := filepath.Join(dir, updater.BundleName)
	if err := os.MkdirAll(filepath.Join(app, "Contents", "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "Contents", "MacOS", "Portty"), []byte("bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	return app
}

func newMoverFix(t *testing.T) *moverFix {
	t.Helper()
	f := &moverFix{t: t, system: t.TempDir(), home: t.TempDir(), quit: make(chan struct{})}
	vol := t.TempDir()
	f.run = updater.DMGRun{Bundle: moverBundle(t, vol), MountPoint: vol, ImagePath: filepath.Join(t.TempDir(), "portty-v9.9.9-darwin-universal.dmg")}
	var once sync.Once
	f.a = &App{
		startupDone: make(chan struct{}),
		messageDialogFunc: func(o runtime.MessageDialogOptions) (string, error) {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.dialogs = append(f.dialogs, o)
			if o.Type == runtime.QuestionDialog {
				return f.answer, nil
			}
			return "OK", nil
		},
		ensureDaemonFunc: func(string) error { t.Error("mover mode ensured the daemon"); return nil },
		showWindowFunc:   func() { t.Error("mover mode showed the window") },
		quitFunc:         func() { once.Do(func() { close(f.quit) }) },
		detectMoverFunc:  func() (updater.DMGRun, bool) { return f.run, true },
	}
	f.a.moverStartFunc = func(j updater.Job) error {
		f.mu.Lock()
		f.jobs = append(f.jobs, j)
		f.mu.Unlock()
		f.a.onTerminate() // startInstaller quits the GUI once the installer runs
		return nil
	}
	oldRoots, oldOpen, oldRunner, oldVersion := moverRoots, openAppPathFunc, updateRunner, Version
	moverRoots = func() (string, string) { return f.system, f.home }
	openAppPathFunc = func(_ context.Context, app string) error {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.opened = append(f.opened, app)
		return nil
	}
	updateRunner = f.tools
	Version = "v9.9.9"
	t.Setenv("HOME", t.TempDir()) // update.log goes to a temp config folder
	t.Cleanup(func() { moverRoots, openAppPathFunc, updateRunner, Version = oldRoots, oldOpen, oldRunner, oldVersion })
	return f
}

// install puts a copy of version in the system Applications folder.
func (f *moverFix) install(version string) string {
	f.installed = version
	return moverBundle(f.t, f.system)
}

func (f *moverFix) tools(_ context.Context, name string, args ...string) ([]byte, error) {
	f.mu.Lock()
	f.calls = append(f.calls, name+" "+strings.Join(args, " "))
	f.mu.Unlock()
	last := ""
	if len(args) > 0 {
		last = args[len(args)-1]
	}
	installed := filepath.Join(f.system, updater.BundleName) + "/"
	switch base := filepath.Base(name); {
	case base == "codesign" && slices.Contains(args, "-dvv"):
		return []byte("Identifier=" + updater.AppIdentifier + "\nTeamIdentifier=" + updater.TeamID + "\n"), nil
	case base == "codesign":
		if f.fail == "codesign" {
			return nil, errors.New("exit status 1")
		}
		return nil, nil
	case base == "spctl":
		if f.fail == "spctl" {
			return nil, errors.New("exit status 1")
		}
		return []byte(last + ": accepted\nsource=Notarized Developer ID\n"), nil
	case base == "xattr":
		return nil, nil
	case base == "ditto":
		return nil, os.CopyFS(last, os.DirFS(args[len(args)-2]))
	case base == "plutil" && slices.Contains(args, "CFBundleShortVersionString"):
		if strings.HasPrefix(last, installed) {
			if f.installed == "unreadable" {
				return nil, errors.New("exit status 1")
			}
			return []byte(f.installed + "\n"), nil
		}
		return []byte("9.9.9\n"), nil
	case base == "plutil":
		return []byte("14.0\n"), nil
	case base == "sw_vers":
		return []byte("27.2\n"), nil
	case strings.HasSuffix(name, "/"+updater.BundleExecutable) && slices.Contains(args, "--version"):
		return []byte("v9.9.9\n"), nil
	case base == "open" && slices.Contains(args, "-R"):
		return nil, nil
	}
	return nil, errors.New("unexpected tool " + name)
}

func (f *moverFix) waitQuit() {
	f.t.Helper()
	select {
	case <-f.quit:
	case <-time.After(10 * time.Second):
		f.t.Fatal("mover mode did not quit")
	}
}

func (f *moverFix) runMover() {
	f.t.Helper()
	f.a.runMover(f.run)
	f.waitQuit()
}

func (f *moverFix) questions() []runtime.MessageDialogOptions {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.DeleteFunc(slices.Clone(f.dialogs), func(o runtime.MessageDialogOptions) bool { return o.Type != runtime.QuestionDialog })
}

func TestMoverStartup_DMGRunSkipsDaemonTrayAndWindow(t *testing.T) {
	f := newMoverFix(t)
	f.a.startup(context.Background())
	if !f.a.moverMode.Load() {
		t.Fatal("moverMode not set")
	}
	select {
	case <-f.a.startupDone:
	default:
		t.Error("startupDone not closed in mover mode")
	}
	if f.a.trayInit {
		t.Error("the tray was started in mover mode")
	}
	f.waitQuit()
	f.a.domReady(context.Background()) // must return without showing the window
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.jobs) != 1 || f.jobs[0].Kind != updater.KindMove {
		t.Errorf("jobs = %+v, want one move job from runMover", f.jobs)
	}
}

func TestMoverStartup_NoRunKeepsNormalPath(t *testing.T) {
	f := newMoverFix(t)
	f.a.detectMoverFunc = func() (updater.DMGRun, bool) { return updater.DMGRun{}, false }
	if f.a.startMover() {
		t.Fatal("startMover took a normal launch")
	}
	if f.a.moverMode.Load() {
		t.Error("moverMode set on a normal launch")
	}
	select {
	case <-f.a.startupDone:
		t.Fatal("startupDone closed before the daemon was ensured")
	default:
	}
	ensured := 0
	f.a.ensureDaemonFunc = func(string) error { ensured++; return nil }
	f.a.startDaemon("/tmp/portty-mover-test.sock") // the rest of startup reaches the ensure seam (Pitfall 8)
	if ensured != 1 {
		t.Errorf("ensure seam called %d times, want 1", ensured)
	}
}

func TestRunMover_NoCopyMovesWithoutAsking(t *testing.T) {
	f := newMoverFix(t)
	f.runMover()
	if q := f.questions(); len(q) != 0 {
		t.Errorf("asked %+v with no installed copy", q)
	}
	if len(f.jobs) != 1 {
		t.Fatalf("jobs = %+v", f.jobs)
	}
	j := f.jobs[0]
	want := filepath.Join(f.system, updater.BundleName)
	if j.Kind != updater.KindMove || j.Target != want || j.OldVersion != "" || j.NewVersion != "9.9.9" || j.OldPID != os.Getpid() ||
		j.ImagePath != f.run.ImagePath || j.MountPoint != f.run.MountPoint || j.DMG != "" {
		t.Errorf("job = %+v", j)
	}
	if filepath.Dir(filepath.Dir(j.Staged)) != f.system || !strings.HasPrefix(filepath.Base(filepath.Dir(j.Staged)), updater.StagingPrefix) || !isRealDir(j.Staged) {
		t.Errorf("staged = %q, want a copy in a staging folder beside the target", j.Staged)
	}
}

func TestRunMover_UnwritableSystemUsesHomeApplications(t *testing.T) {
	f := newMoverFix(t)
	if err := os.Chmod(f.system, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(f.system, 0o755) })
	f.runMover()
	if want := filepath.Join(f.home, "Applications", updater.BundleName); len(f.jobs) != 1 || f.jobs[0].Target != want {
		t.Errorf("jobs = %+v, want target %q", f.jobs, want)
	}
}

func TestRunMover_OlderCopyAsks(t *testing.T) {
	t.Run("replace", func(t *testing.T) {
		f := newMoverFix(t)
		existing := f.install("1.0.0")
		f.answer = "Replace"
		f.runMover()
		q := f.questions()
		if len(q) != 1 || q[0].Title != "Replace Portty 1.0.0 in Applications?" || q[0].Message != "This copy is version 9.9.9." ||
			!slices.Equal(q[0].Buttons, []string{"Replace", "Cancel"}) || q[0].DefaultButton != "Replace" || q[0].CancelButton != "Cancel" {
			t.Fatalf("questions = %+v", q)
		}
		if len(f.jobs) != 1 || f.jobs[0].Kind != updater.KindMove || f.jobs[0].Target != existing || f.jobs[0].OldVersion != "1.0.0" {
			t.Errorf("jobs = %+v", f.jobs)
		}
	})
	t.Run("cancel", func(t *testing.T) {
		f := newMoverFix(t)
		existing := f.install("1.0.0")
		f.answer = "Cancel"
		f.runMover()
		if len(f.jobs) != 0 {
			t.Errorf("jobs = %+v after Cancel", f.jobs)
		}
		if !slices.Equal(f.opened, []string{existing}) {
			t.Errorf("opened = %q, want the installed copy", f.opened)
		}
		staging, _ := filepath.Glob(filepath.Join(f.system, updater.StagingPrefix+"*"))
		if len(staging) != 0 {
			t.Errorf("staging left after Cancel: %v", staging)
		}
	})
}

func TestRunMover_SameOrNewerOpensInstalled(t *testing.T) {
	for _, v := range []string{"9.9.9", "10.0.0"} {
		t.Run(v, func(t *testing.T) {
			f := newMoverFix(t)
			existing := f.install(v)
			f.runMover()
			if q := f.questions(); len(q) != 0 {
				t.Errorf("asked %+v", q)
			}
			if len(f.jobs) != 1 {
				t.Fatalf("jobs = %+v", f.jobs)
			}
			j := f.jobs[0]
			if j.Kind != updater.KindOpenInstalled || j.Target != existing || j.Staged != "" || j.OldVersion != v || j.NewVersion != "9.9.9" || j.ImagePath != f.run.ImagePath {
				t.Errorf("job = %+v", j)
			}
		})
	}
}

func TestRunMover_UnreadableVersionAsks(t *testing.T) {
	f := newMoverFix(t)
	f.install("unreadable")
	f.answer = "Cancel"
	f.runMover()
	if q := f.questions(); len(q) != 1 || q[0].Title != "Replace the Portty in Applications?" {
		t.Errorf("questions = %+v", q)
	}
}

func TestRunMover_StagingFailureAlerts(t *testing.T) {
	f := newMoverFix(t)
	f.fail = "spctl" // after the copy, so the staging folder must be removed
	f.runMover()
	if len(f.jobs) != 0 {
		t.Errorf("jobs = %+v after a failed check", f.jobs)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var alert *runtime.MessageDialogOptions
	for i := range f.dialogs {
		if f.dialogs[i].Type == runtime.InfoDialog {
			alert = &f.dialogs[i]
		}
	}
	if alert == nil || alert.Title != "Portty couldn't move itself to the Applications folder." ||
		alert.Message != "The new version didn't pass the safety check, so it wasn't installed. Drag Portty to the Applications folder to install it." {
		t.Fatalf("dialogs = %+v", f.dialogs)
	}
	if !slices.Contains(f.calls, "/usr/bin/open -R "+f.run.Bundle) {
		t.Errorf("calls = %q, want a Reveal of the volume's app", f.calls)
	}
	staging, _ := filepath.Glob(filepath.Join(f.system, updater.StagingPrefix+"*"))
	if len(staging) != 0 {
		t.Errorf("staging left after a failure: %v", staging)
	}
}

// WR-03: an installer that never started must not write a path into update.log, while the
// alert still finds the cause's sentence through the wrapped error.
func TestRunMover_InstallerStartFailureLogsNoPath(t *testing.T) {
	for _, tc := range []struct {
		name, alert string
		bundle      error
		copyErr     error
	}{
		{name: "bundle", bundle: &os.PathError{Op: "lstat", Path: "/home-fake/Applications/Portty.app", Err: os.ErrNotExist},
			alert: "Portty couldn't install the update."},
		{name: "copy-check", copyErr: fmt.Errorf("%w: app-codesign", updater.ErrSafety),
			alert: "The new version didn't pass the safety check, so it wasn't installed."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newMoverFix(t)
			f.a.moverStartFunc = nil // the real startInstaller, with failing seams
			running := bcFakeBundle(t)
			bcSwap(t,
				func() (string, error) { return running, tc.bundle },
				func(context.Context, string, string) (string, error) {
					return "copy-check", tc.copyErr
				},
				func(string, string) (*os.Process, error) {
					t.Error("spawned an installer")
					return nil, errors.New("no")
				})
			f.runMover()
			b, err := os.ReadFile(bcLogPath())
			if err != nil || !strings.Contains(string(b), "mover: installer not started: "+tc.name) || strings.Contains(string(b), "/") {
				t.Errorf("update.log = %q, %v; want the step and no path", b, err)
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if n := len(f.dialogs); n == 0 || !strings.HasPrefix(f.dialogs[n-1].Message, tc.alert) {
				t.Errorf("dialogs = %+v, want the alert to start with %q", f.dialogs, tc.alert)
			}
		})
	}
}
