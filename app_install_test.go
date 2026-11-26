package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/daemon"
	"github.com/scottkw/portty/internal/updater"
)

// rbNote is one notification the App sent.
type rbNote struct{ id, title, body string }

// rbApp is an App whose notifications are recorded and whose events go nowhere.
func rbApp(t *testing.T) (*App, *[]rbNote) {
	t.Helper()
	var notes []rbNote
	a := &App{
		emitEventFunc:        func(string, any) {},
		sendNotificationFunc: func(id, title, body string) { notes = append(notes, rbNote{id, title, body}) },
	}
	t.Cleanup(func() { _ = os.Remove(installResultPath()) })
	return a, &notes
}

func rbWriteResult(t *testing.T, r updater.Result) {
	t.Helper()
	r.V = 1
	if err := updater.WriteResult(installResultPath(), r); err != nil {
		t.Fatal(err)
	}
}

// rbSwapEnsure records every EnsureOptions the default ensure seam passes.
func rbSwapEnsure(t *testing.T) *[]daemon.EnsureOptions {
	t.Helper()
	var got []daemon.EnsureOptions
	orig := ensureDaemon
	ensureDaemon = func(_ context.Context, _ string, opts daemon.EnsureOptions) error {
		got = append(got, opts)
		return nil
	}
	t.Cleanup(func() { ensureDaemon = orig })
	return &got
}

func TestConsumeInstallResult_KeepDaemonPolicyOnce(t *testing.T) {
	opts := rbSwapEnsure(t)
	a, _ := rbApp(t)
	rbWriteResult(t, updater.Result{Outcome: updater.OutcomeRolledBack, OldVersion: "1.8.0", NewVersion: "1.8.1", Reason: "gui-exited", KeepDaemon: true})
	a.consumeInstallResult()
	for range 2 {
		if err := a.ensure()("sock"); err != nil {
			t.Fatal(err)
		}
	}
	if len(*opts) != 2 {
		t.Fatalf("ensures = %d", len(*opts))
	}
	first, second := (*opts)[0], (*opts)[1]
	if !first.NoHandoff || first.Restart == nil || first.Restart(question(2)) || first.Restart(question(-1)) {
		t.Errorf("first ensure after a rollback: NoHandoff %v; want NoHandoff and a policy that says no", first.NoHandoff)
	}
	if a.GetDaemonRestartPrompt() != 0 {
		t.Error("the first ensure after a rollback raised the restart question")
	}
	if second.NoHandoff || second.Restart == nil || second.Restart(question(2)) || a.GetDaemonRestartPrompt() != 2 {
		t.Errorf("second ensure: NoHandoff %v, prompt %d; want today's restartConsent", second.NoHandoff, a.GetDaemonRestartPrompt())
	}
}

// WR-04: with no result file (its write failed), the installer's launch environment still sets
// the keep-daemon policy, and the variable is gone before a daemon could inherit it.
func TestConsumeInstallResult_KeepDaemonFromLaunchEnv(t *testing.T) {
	opts := rbSwapEnsure(t)
	a, notes := rbApp(t)
	t.Setenv(updater.KeepDaemonEnv, "1")
	a.consumeInstallResult()
	if _, set := os.LookupEnv(updater.KeepDaemonEnv); set {
		t.Error("keep-daemon variable left in the GUI's environment")
	}
	if err := a.ensure()("sock"); err != nil {
		t.Fatal(err)
	}
	if first := (*opts)[0]; !first.NoHandoff || first.Restart == nil || first.Restart(question(2)) || a.GetDaemonRestartPrompt() != 0 {
		t.Errorf("first ensure: NoHandoff %v, prompt %d; want the keep-daemon policy", first.NoHandoff, a.GetDaemonRestartPrompt())
	}
	if len(*notes) != 0 {
		t.Errorf("notifications = %+v", *notes)
	}
}

func TestConsumeInstallResult_TellsUser(t *testing.T) {
	for _, c := range []struct {
		outcome, body string
	}{
		{updater.OutcomeRolledBack, "The update to Portty 1.8.1 didn't take. Portty 1.8.0 is still installed."},
		{updater.OutcomeFailed, "Portty couldn't install version 1.8.1."},
	} {
		t.Run(c.outcome, func(t *testing.T) {
			a, notes := rbApp(t)
			dir := t.TempDir()
			dmg := filepath.Join(dir, updater.DMGAssetName("1.8.1"))
			if err := os.WriteFile(dmg, []byte("dmg"), 0o600); err != nil {
				t.Fatal(err)
			}
			var calls []string
			a.updates = &updater.Service{Current: "1.8.0", Dir: dir, Run: func(_ context.Context, name string, args ...string) ([]byte, error) {
				calls = append(calls, name+" "+strings.Join(args, " "))
				return nil, nil
			}}
			rbWriteResult(t, updater.Result{Outcome: c.outcome, OldVersion: "1.8.0", NewVersion: "1.8.1", Reason: "health", DMG: dmg})
			a.consumeInstallResult()

			if st := a.updates.Status(); st.State != "revealed" || st.Latest != "1.8.1" || st.Error == "" {
				t.Errorf("status = %+v; want the banner's revealed result", st)
			}
			if len(*notes) != 1 || (*notes)[0] != (rbNote{updateResultNotificationID, "Portty", c.body}) {
				t.Errorf("notifications = %+v", *notes)
			}
			if strings.Contains((*notes)[0].body, "/") {
				t.Errorf("notification body holds a slash: %q", (*notes)[0].body)
			}
			if _, err := os.Lstat(installResultPath()); !os.IsNotExist(err) {
				t.Error("result file not consumed")
			}
		})
	}
}

// WR-02: a mover's failed first install has no old version and must still be told; a failed
// result about a version this Portty already is (an open-installed whose open failed, or a copy
// installed by hand since) is stale and only logged.
func TestConsumeInstallResult_MoverResults(t *testing.T) {
	t.Run("first install, no old version", func(t *testing.T) {
		a, notes := rbApp(t)
		a.updates = &updater.Service{Current: "1.8.0", Dir: t.TempDir()}
		rbWriteResult(t, updater.Result{Outcome: updater.OutcomeFailed, NewVersion: "1.8.1", Reason: "reverify"})
		a.consumeInstallResult()
		if len(*notes) != 1 || (*notes)[0].body != "Portty couldn't install version 1.8.1." {
			t.Errorf("notifications = %+v", *notes)
		}
	})
	t.Run("not newer than this Portty", func(t *testing.T) {
		old := Version
		Version = "v1.8.1"
		t.Cleanup(func() { Version = old })
		for _, r := range []updater.Result{
			{Outcome: updater.OutcomeFailed, OldVersion: "1.8.1", NewVersion: "1.8.1", Reason: "open"},
			{Outcome: updater.OutcomeFailed, NewVersion: "1.8.0", Reason: "swap"},
		} {
			a, notes := rbApp(t)
			a.updates = &updater.Service{Current: "1.8.1", Dir: t.TempDir()}
			rbWriteResult(t, r)
			a.consumeInstallResult()
			if len(*notes) != 0 || a.updates.Status().State != "idle" {
				t.Errorf("%+v: notifications %+v, status %+v; want nothing shown", r, *notes, a.updates.Status())
			}
		}
	})
}

func TestConsumeInstallResult_UpdatedAndBadFilesChangeNothing(t *testing.T) {
	cases := map[string]func(t *testing.T){
		"updated": func(t *testing.T) {
			rbWriteResult(t, updater.Result{Outcome: updater.OutcomeUpdated, OldVersion: "1.8.0", NewVersion: "1.8.1"})
		},
		"missing": func(*testing.T) {},
		"malformed": func(t *testing.T) {
			if err := os.WriteFile(installResultPath(), []byte("{"), 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"not ours (mode)": func(t *testing.T) {
			if err := os.WriteFile(installResultPath(), []byte(`{"v":1,"outcome":"rolled-back","keepDaemon":true}`), 0o644); err != nil {
				t.Fatal(err)
			}
			_ = os.Chmod(installResultPath(), 0o644)
		},
		"symlink": func(t *testing.T) {
			src := filepath.Join(t.TempDir(), "r.json")
			if err := os.WriteFile(src, []byte(`{"v":1,"outcome":"rolled-back","keepDaemon":true}`), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(src, installResultPath()); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, set := range cases {
		t.Run(name, func(t *testing.T) {
			a, notes := rbApp(t)
			a.updates = &updater.Service{Current: "1.8.0", Dir: t.TempDir()}
			set(t)
			a.consumeInstallResult()
			if a.keepDaemonOnce.Load() || len(*notes) != 0 || a.updates.Status().State != "idle" {
				t.Errorf("keepDaemon %v, notes %v, status %+v; want nothing changed", a.keepDaemonOnce.Load(), *notes, a.updates.Status())
			}
			if _, err := os.Lstat(installResultPath()); !os.IsNotExist(err) {
				t.Error("result file left in place")
			}
		})
	}
}

func TestTrashItem_RefusesUnderTest(t *testing.T) {
	p := filepath.Join(t.TempDir(), updater.BundleName)
	if err := os.Mkdir(p, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := trashItem(p); err == nil {
		t.Fatal("trashItem succeeded in a test binary")
	}
	if _, err := os.Lstat(p); err != nil {
		t.Errorf("the refused path was moved: %v", err)
	}
}

// swSweep is a sweep over temp folders only: trashItemFunc moves into a temp Trash (or refuses
// with trashErr), updateRunner records hdiutil calls and "detaches" by emptying the mount dir.
type swSweep struct {
	t                   *testing.T
	now                 time.Time
	parent, work, cache string
	trash               string
	trashErr            error
	trashed             []string
	runs                []string
	versions            map[string]string // app path -> CFBundleShortVersionString plutil answers; absent: an error
}

func newSwSweep(t *testing.T) *swSweep {
	t.Helper()
	s := &swSweep{t: t, now: time.Now(), parent: t.TempDir(), work: t.TempDir(), cache: t.TempDir(), trash: t.TempDir()}
	oldT, oldR := trashItemFunc, updateRunner
	t.Cleanup(func() { trashItemFunc, updateRunner = oldT, oldR })
	trashItemFunc = func(p string) error {
		s.trashed = append(s.trashed, p)
		if s.trashErr != nil {
			return s.trashErr
		}
		return os.Rename(p, filepath.Join(s.trash, filepath.Base(filepath.Dir(p))+"-"+filepath.Base(p)))
	}
	updateRunner = func(_ context.Context, name string, args ...string) ([]byte, error) {
		s.runs = append(s.runs, name+" "+strings.Join(args, " "))
		if name == "/usr/bin/hdiutil" && len(args) == 2 && args[0] == "detach" {
			return nil, os.RemoveAll(filepath.Join(args[1], updater.BundleName))
		}
		if name == "/usr/bin/plutil" && len(args) == 4 && args[1] == "CFBundleShortVersionString" {
			if v, ok := s.versions[filepath.Dir(filepath.Dir(args[3]))]; ok {
				return []byte(v + "\n"), nil
			}
			return nil, errors.New("no version")
		}
		return nil, errors.New("unexpected tool")
	}
	return s
}

func (s *swSweep) run() {
	sweepDirs(context.Background(), s.now, []string{s.parent}, s.work, s.cache)
}

// mkBundle makes dir/name as a minimal app tree, then sets dir's mtime to age ago.
func (s *swSweep) mkBundle(dir, name string, age time.Duration) string {
	s.t.Helper()
	exe := filepath.Join(dir, name, updater.BundleExecutable)
	if err := os.MkdirAll(filepath.Dir(exe), 0o700); err != nil {
		s.t.Fatal(err)
	}
	for _, f := range []string{exe, filepath.Join(dir, name, "Contents", "Info.plist")} {
		if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
			s.t.Fatal(err)
		}
	}
	s.age(dir, age)
	return filepath.Join(dir, name)
}

func (s *swSweep) age(p string, age time.Duration) {
	s.t.Helper()
	if err := os.Chtimes(p, s.now.Add(-age), s.now.Add(-age)); err != nil {
		s.t.Fatal(err)
	}
}

func swExists(p string) bool { _, err := os.Lstat(p); return err == nil }

func TestSweep_StagingDirs(t *testing.T) {
	s := newSwSweep(t)
	stale := filepath.Join(s.parent, updater.StagingPrefix+"stale")
	staleApp := s.mkBundle(stale, updater.BundleName, 2*time.Hour)
	fresh := filepath.Join(s.parent, updater.StagingPrefix+"fresh")
	s.mkBundle(fresh, updater.BundleName, sweepAge-time.Minute)
	other := filepath.Join(s.parent, "Portty-update-old")
	s.mkBundle(other, updater.BundleName, 2*time.Hour)
	elsewhere := filepath.Join(t.TempDir(), "elsewhere")
	s.mkBundle(elsewhere, updater.BundleName, 2*time.Hour)
	link := filepath.Join(s.parent, updater.StagingPrefix+"link")
	if err := os.Symlink(elsewhere, link); err != nil {
		t.Fatal(err)
	}
	// A stale staging dir whose Portty.app is a symlink to a real app: the link is not followed.
	linked := filepath.Join(s.parent, updater.StagingPrefix+"linkedapp")
	if err := os.Mkdir(linked, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(elsewhere, updater.BundleName), filepath.Join(linked, updater.BundleName)); err != nil {
		t.Fatal(err)
	}
	s.age(linked, 2*time.Hour)

	s.run()

	if want := []string{staleApp}; !slices.Equal(s.trashed, want) {
		t.Errorf("trashed %v, want %v", s.trashed, want)
	}
	if swExists(stale) {
		t.Error("an emptied stale staging dir was not removed")
	}
	for _, p := range []string{filepath.Join(fresh, updater.BundleName), filepath.Join(other, updater.BundleName), link, filepath.Join(elsewhere, updater.BundleName), filepath.Join(linked, updater.BundleName)} {
		if !swExists(p) {
			t.Errorf("%s was touched", p)
		}
	}
	if len(s.runs) != 0 {
		t.Errorf("tools run: %v", s.runs)
	}
}

// A stale staged copy strictly newer than the installed Portty.app beside it is Portty's own new
// copy that never went in: it is deleted, not sent to the user's Trash. Every copy that is not
// provably that (older, equal, unreadable) still goes to the Trash.
func TestSweep_FailedNewCopyRemoved(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // update.log goes to a temp config folder
	s := newSwSweep(t)
	installed := s.mkBundle(s.parent, updater.BundleName, 2*time.Hour)
	s.versions = map[string]string{installed: "1.0.0"}
	mk := func(name, version string, age time.Duration) (dir, app string) {
		dir = filepath.Join(s.parent, updater.StagingPrefix+name)
		app = s.mkBundle(dir, updater.BundleName, age)
		if version != "" {
			s.versions[app] = version
		}
		return dir, app
	}
	newDir, newApp := mk("new", "2.0.0", 2*time.Hour)
	_, oldApp := mk("old", "0.9.0", 2*time.Hour)
	_, unreadableApp := mk("unreadable", "", 2*time.Hour)
	_, sameApp := mk("same", "1.0.0", 2*time.Hour)
	freshDir, freshApp := mk("fresh", "2.0.0", sweepAge-time.Minute)

	s.run()

	if swExists(newDir) || slices.Contains(s.trashed, newApp) {
		t.Errorf("the newer staged copy: dir exists %v, trashed %v; want it deleted, not trashed", swExists(newDir), s.trashed)
	}
	if want := []string{oldApp, sameApp, unreadableApp}; !slices.Equal(s.trashed, want) { // sweep order is by name
		t.Errorf("trashed %v, want %v", s.trashed, want)
	}
	if !swExists(freshDir) || !swExists(freshApp) || !swExists(installed) {
		t.Error("a fresh staging dir or the installed copy was touched")
	}
	b, _ := os.ReadFile(bcLogPath())
	if strings.Contains(string(b), "/") {
		t.Errorf("update.log = %q; want no path", b)
	}
}

func TestSweep_TrashRefusedKeepsCopyAside(t *testing.T) {
	s := newSwSweep(t)
	s.trashErr = errors.New("trash refused")
	_ = os.Remove(filepath.Join(configDir(), "update.log"))
	stale := filepath.Join(s.parent, updater.StagingPrefix+"stale")
	app := s.mkBundle(stale, updater.BundleName, 2*time.Hour)
	s.run()
	if len(s.trashed) != 1 || !swExists(filepath.Join(app, updater.BundleExecutable)) {
		t.Errorf("trashed %v; want one refused attempt and the copy left whole", s.trashed)
	}
	b, err := os.ReadFile(filepath.Join(configDir(), "update.log"))
	if err != nil || !strings.Contains(string(b), "kept aside") || strings.Contains(string(b), "/") {
		t.Errorf("update.log = %q, %v; want a kept-aside line without a path", b, err)
	}
}

func TestSweep_RunDirs(t *testing.T) {
	s := newSwSweep(t)
	mk := func(name string, age time.Duration) string {
		d := filepath.Join(s.work, name)
		if err := os.Mkdir(d, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "job.json"), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
		s.age(d, age)
		return d
	}
	old, young, notRun := mk("run-old", 2*time.Hour), mk("run-young", time.Minute), mk("job-old", 2*time.Hour)
	target := filepath.Join(t.TempDir(), "kept")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	s.age(target, 2*time.Hour)
	link := filepath.Join(s.work, "run-link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	s.run()
	if swExists(old) {
		t.Error("stale run dir kept")
	}
	for _, p := range []string{young, notRun, link, target} {
		if !swExists(p) {
			t.Errorf("%s was removed", p)
		}
	}
}

func TestSweep_StaleRunDirWithBundleCopy(t *testing.T) {
	s := newSwSweep(t)
	run := filepath.Join(s.work, "run-stale1")
	if err := os.MkdirAll(run, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"job.json", updater.StartedMarker} {
		if err := os.WriteFile(filepath.Join(run, f), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	s.mkBundle(run, updater.BundleName, 2*time.Hour) // the copy 18-10's installer runs from
	s.run()
	if swExists(run) {
		t.Error("stale run dir holding a Portty.app copy was kept")
	}
	if len(s.trashed) != 0 {
		t.Errorf("trashed %v; the run dir's copy is Portty's own and is removed, never trashed", s.trashed)
	}
}

func TestSweep_Mounts(t *testing.T) {
	s := newSwSweep(t)
	mounted := filepath.Join(s.cache, "mnt-live")
	s.mkBundle(mounted, updater.BundleName, 0) // no age floor for mounts
	empty := filepath.Join(s.cache, "mnt-empty")
	if err := os.Mkdir(empty, 0o700); err != nil {
		t.Fatal(err)
	}
	notMnt := filepath.Join(s.cache, "other")
	s.mkBundle(notMnt, updater.BundleName, 2*time.Hour)
	target := filepath.Join(t.TempDir(), "vol")
	s.mkBundle(target, updater.BundleName, 2*time.Hour)
	link := filepath.Join(s.cache, "mnt-link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	s.run()
	if want := []string{"/usr/bin/hdiutil detach " + mounted}; !slices.Equal(s.runs, want) {
		t.Errorf("tools run %v, want %v", s.runs, want)
	}
	if swExists(mounted) || swExists(empty) {
		t.Error("mount dir left behind")
	}
	for _, p := range []string{filepath.Join(notMnt, updater.BundleName), link, filepath.Join(target, updater.BundleName)} {
		if !swExists(p) {
			t.Errorf("%s was touched", p)
		}
	}
	if len(s.trashed) != 0 {
		t.Errorf("trashed %v from the cache", s.trashed)
	}
}

func TestSweepParents_Dedupes(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	homeApps := filepath.Join(home, "Applications")
	old := runningBundleFunc
	t.Cleanup(func() { runningBundleFunc = old })
	for _, c := range []struct {
		bundle string
		err    error
		want   []string
	}{
		{"/Applications/Portty.app", nil, []string{"/Applications", homeApps}},
		{homeApps + "/Portty.app", nil, []string{homeApps, "/Applications"}},
		{"/Volumes/Apps/Portty.app", nil, []string{"/Volumes/Apps", "/Applications", homeApps}},
		{"", errors.New("not in a bundle"), []string{"/Applications", homeApps}},
	} {
		runningBundleFunc = func() (string, error) { return c.bundle, c.err }
		if got := sweepParents(); !slices.Equal(got, c.want) {
			t.Errorf("bundle %q: sweepParents = %v, want %v", c.bundle, got, c.want)
		}
	}
}
