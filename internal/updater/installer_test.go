package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
)

// rbSig is one Signal call; rbOpen is one Open call with what the result file held at that moment.
type rbSig struct {
	pid int
	sig syscall.Signal
	at  time.Duration // fake clock, since the start
}

type rbOpen struct {
	app     string
	hasRes  bool
	res     Result
	newApp  bool // the target held the new version when it was opened
	retries int  // Sleep(OpenRetryWait) calls before this Open
	env     []string
}

// rbFix drives RunInstaller with a fake clock, a fake process table and real bundles on disk
// (target and staged, so Swap is the real RENAME_SWAP exchange).
type rbFix struct {
	t       *testing.T
	job     Job
	start   time.Time
	now     time.Time
	sleeps  []time.Duration
	gui     []int        // what GUIPids lists now
	listed  map[int]bool // every pid GUIPids ever returned
	alive   map[int]bool // Alive answers; pids not here are dead
	termOK  map[int]bool // pids that survive SIGTERM
	killOK  map[int]bool // pids that survive SIGKILL
	signals []rbSig
	opens   []rbOpen
	swaps   int
	runner  Runner
	logs    []string
	daemonV func() (string, error)
	// onOpen runs on a successful Open; the default starts GUI 500 and a healthy new daemon.
	onOpen  func(newApp bool)
	openErr func(newApp bool, n int) error // n counts Opens from 1
	swapErr func(n int) error
	guiFn   func() []int // when set, GUIPids lists this instead of gui
	// Trash moves the path into trashDir (a fake Finder Trash) unless trashErr is set.
	trashDir string
	trashErr error
	trashed  []rbTrash
	// detaches holds, for each hdiutil detach the mover kinds run, how many swaps came before it.
	detaches []int
	rel      *siRelease // the fake tools behind runner (newMoveFix)
	resDir   string     // holds the result file (a mover job has no cache DMG beside it)
}

// rbMarker is siMarker for a path that may not be a fake bundle (a DMG, or nothing): "" then.
func rbMarker(path string) string {
	b, _ := os.ReadFile(filepath.Join(path, "version"))
	return string(b)
}

// rbTrash is one Trash call with what the bundles held at that moment.
type rbTrash struct {
	path      string
	at        time.Duration
	pathVer   string // marker of the bundle being trashed
	targetVer string // marker of the installed bundle
}

const rbDaemonPid = 300

func newRbFix(t *testing.T) *rbFix {
	t.Helper()
	root := t.TempDir()
	target := siBundle(t, root, "1.0.0")
	stagingDir := filepath.Join(root, StagingPrefix+"1")
	staged := siBundle(t, stagingDir, "9.9.9")
	dmg := filepath.Join(t.TempDir(), "portty-v9.9.9-darwin-universal.dmg")
	if err := os.WriteFile(dmg, []byte("dmg"), 0o644); err != nil {
		t.Fatal(err)
	}
	f := &rbFix{
		t:        t,
		job:      Job{V: JobVersion, Kind: KindUpdate, OldPID: 400, Target: target, Staged: staged, OldVersion: "1.0.0", NewVersion: "9.9.9", DMG: dmg},
		start:    time.Unix(1000, 0),
		listed:   map[int]bool{},
		alive:    map[int]bool{rbDaemonPid: true},
		termOK:   map[int]bool{},
		killOK:   map[int]bool{},
		runner:   (&siRelease{}).run,
		trashDir: t.TempDir(),
		resDir:   filepath.Dir(dmg),
	}
	f.now = f.start
	daemon := "v1.0.0"
	f.daemonV = func() (string, error) { return daemon, nil }
	f.onOpen = func(newApp bool) {
		f.gui = []int{500}
		f.alive[500] = true
		if newApp {
			daemon = "v9.9.9"
		}
	}
	return f
}

func (f *rbFix) resPath() string { return filepath.Join(f.resDir, "update-result.json") }

func (f *rbFix) env() InstallerEnv {
	return InstallerEnv{
		Run: f.runner,
		Swap: func(staged, target string) error {
			f.swaps++
			if f.swapErr != nil {
				if err := f.swapErr(f.swaps); err != nil {
					return err
				}
			}
			return Swap(staged, target)
		},
		Open: func(ctx context.Context, app string, launchEnv ...string) error {
			if _, ok := ctx.Deadline(); !ok {
				f.t.Error("Open called without a per-try deadline")
			}
			o := rbOpen{app: app, newApp: siMarker(f.t, f.job.Target) == f.job.NewVersion, env: launchEnv}
			for _, d := range f.sleeps {
				if d == OpenRetryWait {
					o.retries++
				}
			}
			if b, err := os.ReadFile(f.resPath()); err == nil {
				o.hasRes = true
				o.res = rbDecode(f.t, b)
			}
			f.opens = append(f.opens, o)
			if f.openErr != nil {
				if err := f.openErr(o.newApp, len(f.opens)); err != nil {
					return err
				}
			}
			f.onOpen(o.newApp)
			return nil
		},
		Alive: func(pid int) bool { return f.alive[pid] },
		Signal: func(pid int, sig syscall.Signal) error {
			f.signals = append(f.signals, rbSig{pid, sig, f.now.Sub(f.start)})
			survives := (sig == syscall.SIGTERM && f.termOK[pid]) || (sig == syscall.SIGKILL && f.killOK[pid])
			if !survives {
				delete(f.alive, pid)
				f.gui = slices.DeleteFunc(f.gui, func(p int) bool { return p == pid })
			}
			return nil
		},
		GUIPids: func(exe string) ([]int, error) {
			if exe != filepath.Join(f.job.Target, BundleExecutable) {
				f.t.Errorf("GUIPids(%q), want the target's executable", exe)
			}
			pids := f.gui
			if f.guiFn != nil {
				pids = f.guiFn()
			}
			for _, p := range pids {
				f.listed[p] = true
			}
			return slices.Clone(pids), nil
		},
		DaemonVersion: func() (string, error) { return f.daemonV() },
		Trash: func(path string) error {
			f.trashed = append(f.trashed, rbTrash{path, f.now.Sub(f.start), rbMarker(path), rbMarker(f.job.Target)})
			if f.trashErr != nil {
				return f.trashErr
			}
			return os.Rename(path, filepath.Join(f.trashDir, fmt.Sprintf("%s %d", BundleName, len(f.trashed))))
		},
		ResultPath: f.resPath(),
		Now:        func() time.Time { return f.now },
		Sleep: func(d time.Duration) {
			f.sleeps = append(f.sleeps, d)
			f.now = f.now.Add(d)
		},
		Logf: func(format string, args ...any) { f.logs = append(f.logs, fmt.Sprintf(format, args...)) },
	}
}

func (f *rbFix) run() Result {
	f.t.Helper()
	r := RunInstaller(context.Background(), f.job, f.env())
	f.checkSignals()
	return r
}

// checkSignals is the T-18-14 invariant every test gets: each signalled pid came from GUIPids,
// and the daemon is never signalled.
func (f *rbFix) checkSignals() {
	f.t.Helper()
	for _, s := range f.signals {
		if !f.listed[s.pid] || s.pid == rbDaemonPid {
			f.t.Errorf("signal %v to pid %d, which GUIPids never listed", s.sig, s.pid)
		}
	}
}

func rbDecode(t *testing.T, b []byte) Result {
	t.Helper()
	var r Result
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatalf("result file: %v", err)
	}
	return r
}

func (f *rbFix) fileResult() (Result, bool) {
	b, err := os.ReadFile(f.resPath())
	if err != nil {
		return Result{}, false
	}
	return rbDecode(f.t, b), true
}

func (f *rbFix) stagingGone() bool {
	_, err := os.Lstat(filepath.Dir(f.job.Staged))
	return errors.Is(err, os.ErrNotExist)
}

func (f *rbFix) logged(sub string) int {
	n := 0
	for _, l := range f.logs {
		if strings.Contains(l, sub) {
			n++
		}
	}
	return n
}

func TestInstaller_HealthDeadlineRollsBack(t *testing.T) {
	f := newRbFix(t)
	f.onOpen = func(bool) { // N+1's window comes up, its daemon never answers at v9.9.9
		f.gui = []int{500, 501}
		f.alive[500], f.alive[501] = true, true
	}
	f.termOK[500] = true // ignores SIGTERM
	r := f.run()

	if r.Outcome != OutcomeRolledBack || r.Reason != "health" || !r.KeepDaemon {
		t.Errorf("result = %+v", r)
	}
	var term, kill []rbSig
	for _, s := range f.signals {
		switch s.sig {
		case syscall.SIGTERM:
			term = append(term, s)
		case syscall.SIGKILL:
			kill = append(kill, s)
		}
	}
	if len(term) != 2 || len(kill) != 1 || kill[0].pid != 500 {
		t.Fatalf("signals = %+v; want SIGTERM to 500 and 501, then SIGKILL to 500 only", f.signals)
	}
	if kill[0].at-term[0].at < StopWait {
		t.Errorf("SIGKILL %v after SIGTERM, want at least StopWait", kill[0].at-term[0].at)
	}
	if term[0].at < HealthTimeout {
		t.Errorf("rollback started at %v, before the health deadline", term[0].at)
	}
	if f.swaps != 2 || siMarker(t, f.job.Target) != "1.0.0" {
		t.Errorf("swaps = %d, target = %s; want the old version swapped back", f.swaps, siMarker(t, f.job.Target))
	}
	if !f.stagingGone() {
		t.Error("the failed new copy's staging folder is still there")
	}
	if len(f.opens) != 2 || f.opens[1].newApp || !f.opens[1].hasRes || f.opens[1].res.Outcome != OutcomeRolledBack {
		t.Errorf("opens = %+v; want N+1, then N with the rolled-back result already on disk", f.opens)
	}
	if _, err := os.Stat(f.job.DMG); err != nil {
		t.Error("the DMG was removed on a rollback")
	}
}

func TestInstaller_HealthNeedsGUIAndSteadyDaemon(t *testing.T) {
	t.Run("daemon answers but no window", func(t *testing.T) {
		f := newRbFix(t)
		open := f.onOpen
		f.onOpen = func(n bool) { open(n); f.gui = nil }
		if r := f.run(); r.Outcome != OutcomeRolledBack || r.Reason != "health" {
			t.Errorf("result = %+v, want rolled back for health", r)
		}
	})
	t.Run("a daemon error resets the stable window", func(t *testing.T) {
		f := newRbFix(t)
		healthy := false
		f.onOpen = func(bool) { f.gui = []int{500}; f.alive[500] = true; healthy = true }
		f.daemonV = func() (string, error) {
			if !healthy {
				return "v1.0.0", nil
			}
			if el := f.now.Sub(f.start); el >= 2*time.Second && el < 2*time.Second+installerPoll {
				return "", errors.New("connection refused")
			}
			return "v9.9.9", nil
		}
		r := f.run()
		if r.Outcome != OutcomeUpdated {
			t.Fatalf("result = %+v", r)
		}
		if el := f.now.Sub(f.start); el < 2*time.Second+HealthStable {
			t.Errorf("updated after %v; the error at 2 s should have restarted the %v window", el, HealthStable)
		}
	})
}

func TestInstaller_GUIExitRollsBackEarly(t *testing.T) {
	f := newRbFix(t)
	var openedAt time.Time
	f.onOpen = func(newApp bool) {
		openedAt = f.now
		if !newApp {
			f.guiFn = func() []int { return []int{600} } // N's own window after the rollback
		}
	}
	f.guiFn = func() []int { // N+1's window shows for one second, then dies
		if !openedAt.IsZero() && f.now.Sub(openedAt) < time.Second {
			return []int{500}
		}
		return nil
	}
	r := f.run()
	if r.Outcome != OutcomeRolledBack || r.Reason != "gui-exited" || !r.KeepDaemon {
		t.Errorf("result = %+v", r)
	}
	if el := openedAt.Sub(f.start); el > 10*time.Second {
		t.Errorf("rolled back %v after the start, want well before the %v deadline", el, HealthTimeout)
	}
	if len(f.signals) != 0 {
		t.Errorf("signals = %+v, want none (the GUI was already gone)", f.signals)
	}
	if siMarker(t, f.job.Target) != "1.0.0" || !f.stagingGone() {
		t.Error("old version not back in place, or the failed copy kept")
	}
}

func TestInstaller_NeverSignalsDaemon(t *testing.T) {
	for name, set := range map[string]func(f *rbFix){
		"health rollback, survivors": func(f *rbFix) {
			f.onOpen = func(bool) { f.gui = []int{500, 501}; f.alive[500], f.alive[501] = true, true }
			f.termOK[500], f.termOK[501], f.killOK[501] = true, true, true
		},
		"quit timeout, old GUI listed": func(f *rbFix) {
			f.alive[400] = true
			f.gui = []int{400}
		},
		"quit timeout, old GUI not listed": func(f *rbFix) {
			f.alive[400] = true
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newRbFix(t)
			set(f)
			f.run() // checkSignals asserts every pid came from GUIPids and never the daemon
			for _, s := range f.signals {
				if s.pid == rbDaemonPid {
					t.Errorf("daemon signalled: %+v", s)
				}
			}
		})
	}
}

func TestInstaller_ResultWrittenBeforeOpen(t *testing.T) {
	cases := []struct {
		name    string
		set     func(f *rbFix)
		outcome string
		reason  string
		forward int // leading Opens of N+1, which must see no result file
	}{
		{"reverify", func(f *rbFix) {
			rel := &siRelease{fail: "codesign"}
			f.runner = rel.run
		}, OutcomeFailed, "reverify", 0},
		{"swap", func(f *rbFix) { f.swapErr = func(int) error { return errors.New("EPERM") } }, OutcomeFailed, "swap", 0},
		{"health", func(f *rbFix) {
			f.onOpen = func(bool) { f.gui = []int{500}; f.alive[500] = true }
		}, OutcomeRolledBack, "health", 1},
		{"swap-back", func(f *rbFix) {
			f.onOpen = func(bool) { f.gui = []int{500}; f.alive[500] = true }
			f.swapErr = func(n int) error {
				if n == 2 {
					return errors.New("EPERM")
				}
				return nil
			}
		}, OutcomeFailed, "swap-back", 1},
		{"open", func(f *rbFix) {
			f.openErr = func(newApp bool, _ int) error {
				if newApp {
					return errors.New("exit status 1")
				}
				return nil
			}
		}, OutcomeRolledBack, "open", OpenAttempts},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newRbFix(t)
			c.set(f)
			r := f.run()
			if r.Outcome != c.outcome || r.Reason != c.reason {
				t.Fatalf("result = %+v, want %s %s", r, c.outcome, c.reason)
			}
			if len(f.opens) != c.forward+1 {
				t.Fatalf("opens = %+v, want %d forward then 1 relaunch", f.opens, c.forward)
			}
			for i, o := range f.opens {
				if i < c.forward && o.hasRes {
					t.Errorf("open %d of N+1 saw a result file", i+1)
				}
				if i >= c.forward && (!o.hasRes || o.res.Outcome != c.outcome || o.res.Reason != c.reason) {
					t.Errorf("relaunch open %d: result on disk %v %+v", i+1, o.hasRes, o.res)
				}
				if o.app != f.job.Target {
					t.Errorf("open %d of %s, want the target", i+1, o.app)
				}
			}
			if got, ok := f.fileResult(); !ok || got != r {
				t.Errorf("result file = %+v, %v; want %+v", got, ok, r)
			}
			if _, err := os.Stat(f.job.DMG); err != nil {
				t.Error("DMG removed on a non-updated outcome")
			}
		})
	}
}

// WR-04: keepDaemon must reach the relaunched old GUI even when the result file cannot be
// written (a full disk): it then rides in the launch environment. Any other open carries none.
func TestInstaller_RollbackKeepDaemonWithoutResultFile(t *testing.T) {
	keep := []string{KeepDaemonEnv + "=1"}
	t.Run("result not written", func(t *testing.T) {
		f := newRbFix(t)
		blocker := filepath.Join(t.TempDir(), "file")
		if err := os.WriteFile(blocker, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		f.resDir = filepath.Join(blocker, "sub") // WriteResult cannot create its temp file here
		f.onOpen = func(bool) { f.gui = []int{500}; f.alive[500] = true }
		r := f.run()
		if r.Outcome != OutcomeRolledBack || !r.KeepDaemon {
			t.Fatalf("result = %+v", r)
		}
		if len(f.opens) != 2 || f.opens[0].env != nil || !f.opens[0].newApp || f.opens[1].newApp || !slices.Equal(f.opens[1].env, keep) {
			t.Errorf("opens = %+v; want the new app with no env, then the old app with %v", f.opens, keep)
		}
		if f.logged("installer: result not written") == 0 || f.logged(KeepDaemonEnv) != 1 {
			t.Errorf("logs = %q", f.logs)
		}
	})
	t.Run("result written", func(t *testing.T) {
		f := newRbFix(t)
		f.onOpen = func(bool) { f.gui = []int{500}; f.alive[500] = true }
		if r := f.run(); r.Outcome != OutcomeRolledBack {
			t.Fatalf("result = %+v", r)
		}
		for i, o := range f.opens {
			if o.env != nil {
				t.Errorf("open %d env = %v, want none", i+1, o.env)
			}
		}
	})
}

func TestInstaller_QuitTimeoutKillsOnlyOldGUI(t *testing.T) {
	t.Run("listed: killed, install continues", func(t *testing.T) {
		f := newRbFix(t)
		f.alive[400] = true
		f.gui = []int{400}
		r := f.run()
		if r.Outcome != OutcomeUpdated {
			t.Errorf("result = %+v", r)
		}
		if len(f.signals) != 1 || f.signals[0] != (rbSig{400, syscall.SIGKILL, f.signals[0].at}) || f.signals[0].at < QuitWait {
			t.Errorf("signals = %+v; want one SIGKILL to 400 after QuitWait", f.signals)
		}
	})
	for name, set := range map[string]func(f *rbFix){
		"not listed": func(f *rbFix) { f.gui = []int{401} },
		"survives":   func(f *rbFix) { f.gui = []int{400}; f.killOK[400] = true },
	} {
		t.Run(name, func(t *testing.T) {
			f := newRbFix(t)
			f.alive[400] = true
			set(f)
			r := f.run()
			if r.Outcome != OutcomeFailed || r.Reason != "quit-timeout" {
				t.Errorf("result = %+v", r)
			}
			for _, s := range f.signals {
				if s.pid != 400 {
					t.Errorf("signal to %d, want none or only the old GUI", s.pid)
				}
			}
			if name == "not listed" && len(f.signals) != 0 {
				t.Errorf("signals = %+v, want none for an unlisted pid", f.signals)
			}
			if f.swaps != 0 || len(f.opens) != 0 || siMarker(t, f.job.Target) != "1.0.0" || f.stagingGone() {
				t.Errorf("touched the install: swaps %d, opens %d", f.swaps, len(f.opens))
			}
		})
	}
}

func TestInstaller_OpenRetriesThenSucceeds(t *testing.T) {
	f := newRbFix(t)
	f.openErr = func(_ bool, n int) error {
		if n <= 2 {
			return errors.New("exit status 1") // open's -600 procNotFound (row A9)
		}
		return nil
	}
	r := f.run()
	if r.Outcome != OutcomeUpdated || f.swaps != 1 {
		t.Errorf("result = %+v, swaps = %d", r, f.swaps)
	}
	if len(f.opens) != 3 {
		t.Fatalf("opens = %+v, want 3", f.opens)
	}
	for i, o := range f.opens {
		if o.app != f.job.Target || o.retries != i {
			t.Errorf("open %d: %s after %d retry waits, want the target after %d", i+1, o.app, o.retries, i)
		}
	}
	if f.logged("installer: open attempt") != 2 {
		t.Errorf("logs = %q, want two open attempt lines", f.logs)
	}
	for _, l := range f.logs {
		if strings.Contains(l, "/") {
			t.Errorf("log line carries a path: %q", l)
		}
	}
}

func TestInstaller_OpenPersistentFailureRollsBack(t *testing.T) {
	f := newRbFix(t)
	f.openErr = func(newApp bool, _ int) error {
		if newApp {
			return errors.New("exit status 1")
		}
		return nil
	}
	r := f.run()
	if r.Outcome != OutcomeRolledBack || r.Reason != "open" || !r.KeepDaemon {
		t.Errorf("result = %+v", r)
	}
	if len(f.opens) != OpenAttempts+1 || f.opens[OpenAttempts].newApp || !f.opens[OpenAttempts].hasRes {
		t.Errorf("opens = %+v; want %d of N+1, then N with the result on disk", f.opens, OpenAttempts)
	}
	if f.swaps != 2 || siMarker(t, f.job.Target) != "1.0.0" || !f.stagingGone() {
		t.Errorf("swaps = %d; want the old version back and the failed copy removed", f.swaps)
	}
}

func TestInstaller_RollbackRelaunchFailureWritesFailed(t *testing.T) {
	t.Run("rollback", func(t *testing.T) {
		f := newRbFix(t)
		f.openErr = func(bool, int) error { return errors.New("exit status 1") }
		r := f.run()
		if len(f.opens) != 2*OpenAttempts {
			t.Errorf("opens = %d, want %d", len(f.opens), 2*OpenAttempts)
		}
		got, ok := f.fileResult()
		if !ok || got.Outcome != OutcomeFailed || got.Reason != "open" || !got.KeepDaemon || got != r {
			t.Errorf("result file = %+v (%v), returned %+v", got, ok, r)
		}
		if f.logged("installer: relaunch failed, result kept for the next launch") != 1 {
			t.Errorf("logs = %q", f.logs)
		}
	})
	t.Run("first swap", func(t *testing.T) {
		f := newRbFix(t)
		f.swapErr = func(int) error { return errors.New("EPERM") }
		f.openErr = func(bool, int) error { return errors.New("exit status 1") }
		r := f.run()
		got, ok := f.fileResult()
		if !ok || got.Outcome != OutcomeFailed || got.Reason != "swap" || got != r {
			t.Errorf("result file = %+v (%v), returned %+v", got, ok, r)
		}
		if len(f.opens) != OpenAttempts || f.logged("installer: relaunch failed, result kept for the next launch") != 1 {
			t.Errorf("opens = %d, logs = %q", len(f.opens), f.logs)
		}
	})
}

func TestParseGUIPids(t *testing.T) {
	exe := "/Applications/Portty.app/Contents/MacOS/Portty"
	run := "/home-fake/Library/Application Support/portty/update/run-123/"
	ps := strings.Join([]string{
		"  PID   UID ARGS",
		"  101   501 " + exe,
		"  102   501 " + exe + " -psn_0_12345",
		"  103   501 " + exe + " daemon",
		"  104   501 " + exe + " daemon --takeover",
		"  105   501 " + exe + " install-update /tmp/job.json",
		"  106   501 /Applications/Portty.app.old/Contents/MacOS/Portty",
		"  107   501 " + exe + "X",
		"  108   501 " + run + "Portty.app/Contents/MacOS/Portty install-update " + run + "job.json",
		"  109   501 " + exe + " attach main", // a CLI client run from the bundle is not a GUI
		"  110   501 /bin/zsh",
		"  111   502 " + exe, // WR-01: another account's GUI (fast user switching) is never this user's
		"  112   502 " + exe + " -psn_0_999",
		"  113     0 " + exe,
		"  114  x501 " + exe,
		"  115 " + exe, // no uid column
		"",
		"garbage",
	}, "\n")
	if got := ParseGUIPids([]byte(ps), exe, 501); !slices.Equal(got, []int{101, 102}) {
		t.Errorf("ParseGUIPids = %v, want [101 102]", got)
	}
	if got := ParseGUIPids([]byte(ps), exe, 502); !slices.Equal(got, []int{111, 112}) {
		t.Errorf("uid 502: ParseGUIPids = %v, want [111 112]", got)
	}
	spaced := "/Volumes/My Apps/Portty.app/Contents/MacOS/Portty"
	out := "  201 501 " + spaced + "\n  202 501 " + spaced + " daemon\n  203 501 /Volumes/My\n"
	if got := ParseGUIPids([]byte(out), spaced, 501); !slices.Equal(got, []int{201}) {
		t.Errorf("path with a space: ParseGUIPids = %v, want [201]", got)
	}
}

func TestInstaller_TrashesOldAfterHealthOnly(t *testing.T) {
	t.Run("healthy", func(t *testing.T) {
		f := newRbFix(t)
		r := f.run()
		if r.Outcome != OutcomeUpdated {
			t.Fatalf("result = %+v", r)
		}
		if len(f.trashed) != 1 {
			t.Fatalf("Trash calls = %+v, want exactly one", f.trashed)
		}
		tr := f.trashed[0]
		if tr.path != f.job.Staged || tr.pathVer != "1.0.0" || tr.targetVer != "9.9.9" {
			t.Errorf("trashed %+v; want the old 1.0.0 bundle at the staged path with 9.9.9 installed", tr)
		}
		if tr.at < HealthStable {
			t.Errorf("trashed at %v, before the %v health window could pass", tr.at, HealthStable)
		}
		if !f.stagingGone() {
			t.Error("the empty staging folder was not removed")
		}
		if siMarker(t, f.job.Target) != "9.9.9" {
			t.Error("the new version is not installed")
		}
	})
	for name, set := range map[string]func(f *rbFix){
		"health rollback": func(f *rbFix) { f.onOpen = func(bool) { f.gui = []int{500}; f.alive[500] = true } },
		"open rollback": func(f *rbFix) {
			f.openErr = func(newApp bool, _ int) error {
				if newApp {
					return errors.New("exit status 1")
				}
				return nil
			}
		},
		"gui exited": func(f *rbFix) {
			f.onOpen = func(bool) {}
			n := 0
			f.guiFn = func() []int {
				if n++; n == 1 {
					return []int{500}
				}
				return nil
			}
		},
		"swap failed":  func(f *rbFix) { f.swapErr = func(int) error { return errors.New("EPERM") } },
		"reverify":     func(f *rbFix) { f.runner = (&siRelease{fail: "codesign"}).run },
		"quit timeout": func(f *rbFix) { f.alive[400] = true },
	} {
		t.Run(name, func(t *testing.T) {
			f := newRbFix(t)
			set(f)
			if r := f.run(); r.Outcome == OutcomeUpdated {
				t.Fatalf("result = %+v, want a non-updated outcome", r)
			}
			if len(f.trashed) != 0 {
				t.Errorf("Trash calls = %+v on a non-updated outcome", f.trashed)
			}
		})
	}
	t.Run("staged path outside a staging folder", func(t *testing.T) {
		f := newRbFix(t)
		other := filepath.Join(filepath.Dir(f.job.Target), "elsewhere")
		if err := os.Rename(filepath.Dir(f.job.Staged), other); err != nil {
			t.Fatal(err)
		}
		f.job.Staged = filepath.Join(other, BundleName)
		if r := f.run(); r.Outcome != OutcomeUpdated {
			t.Fatalf("result = %+v", r)
		}
		if len(f.trashed) != 0 {
			t.Errorf("Trash calls = %+v for a path whose parent lacks the staging prefix", f.trashed)
		}
	})
}

func TestInstaller_TrashFailureKeepsOldAside(t *testing.T) {
	f := newRbFix(t)
	f.trashErr = errors.New("trash refused")
	r := f.run()
	if r.Outcome != OutcomeUpdated {
		t.Fatalf("result = %+v; a refused Trash must not change the outcome", r)
	}
	if got, ok := f.fileResult(); !ok || got.Outcome != OutcomeUpdated {
		t.Errorf("result file = %+v, %v", got, ok)
	}
	if len(f.trashed) != 1 || f.stagingGone() || siMarker(t, f.job.Staged) != "1.0.0" {
		t.Errorf("trashed %+v; want one attempt and the old bundle still aside in its staging folder", f.trashed)
	}
	if f.logged("installer: old copy kept aside") != 1 {
		t.Errorf("logs = %q", f.logs)
	}
	for _, l := range f.logs {
		if strings.Contains(l, "/") {
			t.Errorf("log line carries a path: %q", l)
		}
	}
}

// newMoveFix is rbFix with a mover job (UPD-08). The target's folder is the only allowed
// Applications folder, the volume is a fake mount in a temp "/Volumes/", and the image is a
// user-owned DMG in a temp folder. For KindMove, existing keeps the installed 1.0.0 (a Replace);
// otherwise there is no installed copy. KindOpenInstalled keeps the installed copy, has no staged
// copy and offers the same version.
func newMoveFix(t *testing.T, kind string, existing bool) *rbFix {
	t.Helper()
	f := newRbFix(t)
	root := filepath.Dir(f.job.Target)
	vols := t.TempDir() + "/"
	oldParents, oldVols, oldPause := moveParents, volumesRoot, detachPause
	moveParents = func() []string { return []string{root} }
	volumesRoot, detachPause = vols, 0
	t.Cleanup(func() { moveParents, volumesRoot, detachPause = oldParents, oldVols, oldPause })

	mnt := filepath.Join(vols, "Portty")
	siBundle(t, mnt, "9.9.9")
	img := filepath.Join(t.TempDir(), "portty-v9.9.9-darwin-universal.dmg")
	if err := os.WriteFile(img, []byte("dmg"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.rel = &siRelease{}
	f.runner = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "/usr/bin/hdiutil" && len(args) > 0 && args[0] == "detach" {
			f.detaches = append(f.detaches, f.swaps)
		}
		return f.rel.run(ctx, name, args...)
	}
	f.job.Kind, f.job.DMG, f.job.ImagePath, f.job.MountPoint = kind, "", img, mnt
	switch {
	case kind == KindOpenInstalled:
		if err := os.RemoveAll(filepath.Dir(f.job.Staged)); err != nil {
			t.Fatal(err)
		}
		f.job.Staged, f.job.OldVersion, f.job.NewVersion = "", "1.0.0", "1.0.0"
	case !existing:
		if err := os.RemoveAll(f.job.Target); err != nil {
			t.Fatal(err)
		}
		f.job.OldVersion = ""
	}
	return f
}

func (f *rbFix) imageTrashed() int {
	n := 0
	for _, tr := range f.trashed {
		if tr.path == f.job.ImagePath {
			n++
		}
	}
	return n
}

func (f *rbFix) mountGone() bool {
	_, err := os.Lstat(f.job.MountPoint)
	return errors.Is(err, os.ErrNotExist)
}

func TestJob_MoveKinds(t *testing.T) {
	read := func(t *testing.T, j Job) error {
		t.Helper()
		path := filepath.Join(t.TempDir(), "job.json")
		if err := WriteJob(path, j); err != nil {
			t.Fatal(err)
		}
		got, err := ReadJob(path)
		if err == nil && got != j {
			t.Errorf("round trip = %+v, want %+v", got, j)
		}
		return err
	}
	for _, c := range []struct {
		name     string
		kind     string
		existing bool
		mutate   func(t *testing.T, f *rbFix)
		ok       bool
	}{
		{name: "move, no copy", kind: KindMove, ok: true},
		{name: "move, older copy", kind: KindMove, existing: true, ok: true},
		{name: "open-installed, same version", kind: KindOpenInstalled, ok: true},
		{name: "open-installed, newer installed", kind: KindOpenInstalled, ok: true,
			mutate: func(_ *testing.T, f *rbFix) { f.job.OldVersion = "2.0.0" }},

		{name: "move, no copy but an old version", kind: KindMove,
			mutate: func(_ *testing.T, f *rbFix) { f.job.OldVersion = "1.0.0" }},
		{name: "move, copy but no old version", kind: KindMove, existing: true,
			mutate: func(_ *testing.T, f *rbFix) { f.job.OldVersion = "" }},
		{name: "move, downgrade", kind: KindMove, existing: true,
			mutate: func(_ *testing.T, f *rbFix) { f.job.OldVersion = "9.9.9" }},
		{name: "open-installed, older installed", kind: KindOpenInstalled,
			mutate: func(_ *testing.T, f *rbFix) { f.job.OldVersion = "0.9.0" }},
		{name: "open-installed with a staged copy", kind: KindOpenInstalled,
			mutate: func(t *testing.T, f *rbFix) {
				f.job.Staged = siBundle(t, filepath.Join(filepath.Dir(f.job.Target), StagingPrefix+"x"), "1.0.0")
			}},
		{name: "open-installed, nothing installed", kind: KindOpenInstalled,
			mutate: func(t *testing.T, f *rbFix) { _ = os.RemoveAll(f.job.Target) }},
		{name: "target outside the Applications folders", kind: KindMove,
			mutate: func(t *testing.T, f *rbFix) {
				other := t.TempDir()
				staged := siBundle(t, filepath.Join(other, StagingPrefix+"y"), "9.9.9")
				f.job.Target, f.job.Staged = filepath.Join(other, BundleName), staged
			}},
		{name: "target a symlink", kind: KindMove, existing: true,
			mutate: func(t *testing.T, f *rbFix) {
				_ = os.RemoveAll(f.job.Target)
				if err := os.Symlink(t.TempDir(), f.job.Target); err != nil {
					t.Fatal(err)
				}
			}},
		{name: "staged not beside the target", kind: KindMove,
			mutate: func(t *testing.T, f *rbFix) {
				f.job.Staged = siBundle(t, filepath.Join(t.TempDir(), StagingPrefix+"z"), "9.9.9")
			}},
		{name: "a dmg cache path", kind: KindMove, mutate: func(_ *testing.T, f *rbFix) { f.job.DMG = f.job.ImagePath }},
		{name: "image path not a release asset", kind: KindMove, ok: true,
			mutate: func(t *testing.T, f *rbFix) {
				p := filepath.Join(t.TempDir(), "notes.dmg")
				_ = os.WriteFile(p, nil, 0o644)
				f.job.ImagePath = p
			}},
		{name: "image path a directory", kind: KindMove, ok: true,
			mutate: func(t *testing.T, f *rbFix) {
				p := filepath.Join(t.TempDir(), "portty-v9.9.9-darwin-universal.dmg")
				_ = os.Mkdir(p, 0o755)
				f.job.ImagePath = p
			}},
		{name: "image path a symlink", kind: KindMove, ok: true,
			mutate: func(t *testing.T, f *rbFix) {
				p := filepath.Join(t.TempDir(), "portty-v9.9.9-darwin-universal.dmg")
				_ = os.Symlink(f.job.ImagePath, p)
				f.job.ImagePath = p
			}},
		{name: "image path missing", kind: KindOpenInstalled, ok: true,
			mutate: func(_ *testing.T, f *rbFix) { _ = os.Remove(f.job.ImagePath) }},
		{name: "image path in the update cache", kind: KindMove, ok: true,
			mutate: func(t *testing.T, f *rbFix) {
				dir := filepath.Join(t.TempDir(), AppIdentifier, "updates")
				_ = os.MkdirAll(dir, 0o700)
				p := filepath.Join(dir, "portty-v9.9.9-darwin-universal.dmg")
				_ = os.WriteFile(p, nil, 0o600)
				f.job.ImagePath = p
			}},
		{name: "image path relative", kind: KindMove,
			mutate: func(_ *testing.T, f *rbFix) { f.job.ImagePath = "portty-v9.9.9-darwin-universal.dmg" }},
		{name: "image path not clean", kind: KindMove,
			mutate: func(_ *testing.T, f *rbFix) { f.job.ImagePath += "/../x.dmg" }},
		{name: "image path empty", kind: KindMove, ok: true,
			mutate: func(_ *testing.T, f *rbFix) { f.job.ImagePath = "" }},
		{name: "mount point outside /Volumes", kind: KindMove,
			mutate: func(t *testing.T, f *rbFix) { f.job.MountPoint = t.TempDir() }},
		{name: "mount point not clean", kind: KindOpenInstalled,
			mutate: func(_ *testing.T, f *rbFix) { f.job.MountPoint += "/../Portty" }},
		{name: "mount point is the volumes folder", kind: KindMove,
			mutate: func(_ *testing.T, f *rbFix) { f.job.MountPoint = filepath.Clean(volumesRoot) }},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newMoveFix(t, c.kind, c.existing)
			if c.mutate != nil {
				c.mutate(t, f)
			}
			err := read(t, f.job)
			if c.ok && err != nil {
				t.Fatalf("ReadJob refused a good job: %v", err)
			}
			if !c.ok && !errors.Is(err, ErrJob) {
				t.Fatalf("ReadJob = %v, want an ErrJob refusal", err)
			}
			if err != nil && strings.Contains(err.Error(), "/") {
				t.Errorf("error carries a path: %v", err)
			}
		})
	}
}

func TestInstaller_MoveNoCopyLeavesNoStagingDir(t *testing.T) {
	f := newMoveFix(t, KindMove, false)
	r := f.run()
	if r.Outcome != OutcomeUpdated {
		t.Fatalf("result = %+v", r)
	}
	if len(f.detaches) == 0 || f.detaches[0] != 0 || !f.mountGone() {
		t.Errorf("detaches = %v (swaps before each), mount gone %v; want the volume ejected before the swap", f.detaches, f.mountGone())
	}
	if siMarker(t, f.job.Target) != "9.9.9" {
		t.Error("the new copy is not installed")
	}
	left, _ := filepath.Glob(filepath.Join(filepath.Dir(f.job.Target), StagingPrefix+"*"))
	if len(left) != 0 {
		t.Errorf("staging dirs left beside the target: %v", left)
	}
	if len(f.opens) != 1 || f.opens[0].app != f.job.Target {
		t.Errorf("opens = %+v, want exactly one of the target", f.opens)
	}
	if len(f.trashed) != 1 || f.imageTrashed() != 1 {
		t.Errorf("trashed = %+v, want only the image path", f.trashed)
	}
	if len(f.signals) != 0 {
		t.Errorf("signals = %+v; a first install stops nothing", f.signals)
	}
}

func TestInstaller_MoveReplaceOlder(t *testing.T) {
	f := newMoveFix(t, KindMove, true)
	f.gui = []int{450} // the installed copy's GUI is running (P4)
	f.alive[450] = true
	f.swapErr = func(n int) error {
		if n == 1 && f.alive[450] {
			t.Error("swapped while the installed GUI still ran")
		}
		return nil
	}
	r := f.run()
	if r.Outcome != OutcomeUpdated {
		t.Fatalf("result = %+v", r)
	}
	if len(f.signals) == 0 || f.signals[0].pid != 450 || f.signals[0].sig != syscall.SIGTERM {
		t.Errorf("signals = %+v; want SIGTERM to the installed GUI first", f.signals)
	}
	if len(f.detaches) == 0 || f.detaches[0] != 0 {
		t.Errorf("detaches = %v; want the volume ejected before the swap", f.detaches)
	}
	if len(f.trashed) != 2 || f.trashed[0].path != f.job.Staged || f.trashed[0].pathVer != "1.0.0" || f.trashed[1].path != f.job.ImagePath {
		t.Fatalf("trashed = %+v; want the old 1.0.0 bundle, then the image", f.trashed)
	}
	if f.trashed[0].at < HealthStable {
		t.Errorf("old copy trashed at %v, before health", f.trashed[0].at)
	}
}

func TestInstaller_MoveReplaceUnhealthyRollsBack(t *testing.T) {
	f := newMoveFix(t, KindMove, true)
	f.onOpen = func(bool) { f.gui = []int{500}; f.alive[500] = true } // the daemon never reaches 9.9.9
	r := f.run()
	if r.Outcome != OutcomeRolledBack || r.Reason != "health" || !r.KeepDaemon {
		t.Fatalf("result = %+v, want rolled-back health with keepDaemon", r)
	}
	if siMarker(t, f.job.Target) != "1.0.0" || !f.stagingGone() {
		t.Error("the old copy was not put back, or the failed copy was left")
	}
	if f.imageTrashed() != 0 || len(f.trashed) != 0 {
		t.Errorf("trashed = %+v on a rollback", f.trashed)
	}
}

func TestInstaller_MoveFirstInstallUnhealthy(t *testing.T) {
	f := newMoveFix(t, KindMove, false)
	f.onOpen = func(bool) { f.gui = []int{500}; f.alive[500] = true }
	r := f.run()
	if r.Outcome != OutcomeFailed || r.Reason != "health" {
		t.Fatalf("result = %+v, want failed health", r)
	}
	if siMarker(t, f.job.Target) != "9.9.9" {
		t.Error("the verified new copy was not left installed")
	}
	if len(f.trashed) != 0 || len(f.signals) != 0 {
		t.Errorf("trashed %+v, signals %+v; want neither", f.trashed, f.signals)
	}
	if got, ok := f.fileResult(); !ok || got.Outcome != OutcomeFailed {
		t.Errorf("result file = %+v, %v", got, ok)
	}
}

func TestInstaller_MoveDetachFailureNeverTrashesImage(t *testing.T) {
	for _, kind := range []string{KindMove, KindOpenInstalled} {
		t.Run(kind, func(t *testing.T) {
			f := newMoveFix(t, kind, false)
			f.rel.fail = "detach"
			r := f.run()
			if want := map[string]string{KindMove: OutcomeUpdated, KindOpenInstalled: OutcomeOpened}[kind]; r.Outcome != want {
				t.Fatalf("result = %+v, want %s: a failed eject does not stop the install", r, want)
			}
			if f.imageTrashed() != 0 {
				t.Errorf("trashed = %+v after a failed detach", f.trashed)
			}
			if f.logged("detach") == 0 {
				t.Errorf("logs = %q, want the failed detach logged", f.logs)
			}
		})
	}
}

func TestInstaller_MoveImageGuard(t *testing.T) {
	for _, kind := range []string{KindMove, KindOpenInstalled} {
		t.Run(kind+" asset name", func(t *testing.T) {
			f := newMoveFix(t, kind, false)
			p := filepath.Join(filepath.Dir(f.job.ImagePath), "Notes.dmg")
			if err := os.Rename(f.job.ImagePath, p); err != nil {
				t.Fatal(err)
			}
			f.job.ImagePath = p
			if r := f.run(); r.Outcome == OutcomeFailed {
				t.Fatalf("result = %+v", r)
			}
			if f.imageTrashed() != 0 {
				t.Errorf("trashed %+v: a file without the asset name", f.trashed)
			}
		})
		t.Run(kind+" image owned by someone else", func(t *testing.T) {
			f := newMoveFix(t, kind, false)
			old := wantUID
			wantUID = func() int { return old() + 1 }
			t.Cleanup(func() { wantUID = old })
			if r := f.run(); r.Outcome == OutcomeFailed {
				t.Fatalf("result = %+v", r)
			}
			if f.imageTrashed() != 0 {
				t.Errorf("trashed %+v: a file this user does not own", f.trashed)
			}
		})
		// CR-01: the Reveal fallback shows the user the DMG in Portty's own update cache, so a
		// launch from it must still move; only the Trash skips that DMG.
		t.Run(kind+" image in the update cache", func(t *testing.T) {
			f := newMoveFix(t, kind, false)
			dir := filepath.Join(t.TempDir(), AppIdentifier, "updates")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			p := filepath.Join(dir, filepath.Base(f.job.ImagePath))
			if err := os.Rename(f.job.ImagePath, p); err != nil {
				t.Fatal(err)
			}
			f.job.ImagePath = p
			jobPath := filepath.Join(t.TempDir(), "job.json")
			if err := WriteJob(jobPath, f.job); err != nil {
				t.Fatal(err)
			}
			if _, err := ReadJob(jobPath); err != nil {
				t.Fatalf("ReadJob refused a launch from the update cache: %v", err)
			}
			if r := f.run(); r.Outcome != moverOutcome[kind] {
				t.Fatalf("result = %+v, want %s", r, moverOutcome[kind])
			}
			if kind == KindMove && (f.swaps != 1 || siMarker(t, f.job.Target) != "9.9.9") {
				t.Errorf("swaps = %d; want the new copy installed", f.swaps)
			}
			if _, err := os.Lstat(p); err != nil || f.imageTrashed() != 0 {
				t.Errorf("cache DMG trashed or gone (%v), trashed %+v", err, f.trashed)
			}
		})
		t.Run(kind+" image gone", func(t *testing.T) {
			f := newMoveFix(t, kind, false)
			on := f.onOpen
			f.onOpen = func(newApp bool) { _ = os.Remove(f.job.ImagePath); on(newApp) } // the user trashed it meanwhile
			r := f.run()
			if want := map[string]string{KindMove: OutcomeUpdated, KindOpenInstalled: OutcomeOpened}[kind]; r.Outcome != want {
				t.Fatalf("result = %+v, want %s", r, want)
			}
			if f.logged("dmg kept") != 0 {
				t.Errorf("logs = %q; a DMG already gone is not an error", f.logs)
			}
		})
	}
}

func TestInstaller_MoveQuitTimeoutTouchesNothing(t *testing.T) {
	f := newMoveFix(t, KindMove, true)
	f.alive[400] = true // the DMG's GUI never exits
	f.gui = []int{400}  // even if ps listed it from the target, the mover never kills it
	r := f.run()
	if r.Outcome != OutcomeFailed || r.Reason != "quit-timeout" {
		t.Fatalf("result = %+v", r)
	}
	if len(f.signals) != 0 || f.swaps != 0 || len(f.detaches) != 0 || len(f.opens) != 0 || len(f.trashed) != 0 {
		t.Errorf("signals %+v, swaps %d, detaches %v, opens %+v, trashed %+v; want nothing changed", f.signals, f.swaps, f.detaches, f.opens, f.trashed)
	}
	if siMarker(t, f.job.Target) != "1.0.0" || f.mountGone() {
		t.Error("the installed copy or the volume was touched")
	}
}

func TestInstaller_OpenInstalledTrashesDMG(t *testing.T) {
	f := newMoveFix(t, KindOpenInstalled, false)
	r := f.run()
	if r.Outcome != OutcomeOpened {
		t.Fatalf("result = %+v", r)
	}
	if len(f.detaches) == 0 || !f.mountGone() {
		t.Error("the volume was not ejected")
	}
	if len(f.opens) != 1 || f.opens[0].app != f.job.Target {
		t.Errorf("opens = %+v, want exactly one of the installed copy", f.opens)
	}
	if len(f.trashed) != 1 || f.imageTrashed() != 1 {
		t.Errorf("trashed = %+v, want the image path exactly once and never the installed bundle", f.trashed)
	}
	if f.swaps != 0 || len(f.signals) != 0 || siMarker(t, f.job.Target) != "1.0.0" {
		t.Error("the installed copy was touched")
	}
	if got, ok := f.fileResult(); !ok || got.Outcome != OutcomeOpened {
		t.Errorf("result file = %+v, %v", got, ok)
	}
}

func TestInstaller_OpenInstalledOpenFailure(t *testing.T) {
	f := newMoveFix(t, KindOpenInstalled, false)
	f.openErr = func(bool, int) error { return errors.New("exit status 1") }
	r := f.run()
	if r.Outcome != OutcomeFailed || r.Reason != "open" {
		t.Fatalf("result = %+v", r)
	}
	if len(f.opens) != OpenAttempts {
		t.Errorf("opens = %d, want %d tries", len(f.opens), OpenAttempts)
	}
	if len(f.trashed) != 0 {
		t.Errorf("trashed = %+v after a failed open", f.trashed)
	}
}

// moverOutcome is the outcome a successful mover job of each kind ends with.
var moverOutcome = map[string]string{KindMove: OutcomeUpdated, KindOpenInstalled: OutcomeOpened}

// The Trash of a DMG in ~/Downloads can wait on the system's Downloads prompt (18-06 R4-smoke,
// decision option A), so the final result is written before it. The run folder holds the
// installer's own executable, and tccd refuses without a prompt once that is gone
// (R4-smoke-rundir-first), so the folder is removed only after the Trash returns.
func TestInstaller_MoverResultBeforeTrashRunDirAfter(t *testing.T) {
	for _, c := range []struct {
		name     string
		kind     string
		existing bool
	}{{"move, no copy", KindMove, false}, {"move, replace", KindMove, true}, {"open-installed", KindOpenInstalled, false}} {
		t.Run(c.name, func(t *testing.T) {
			f := newMoveFix(t, c.kind, c.existing)
			runDir := filepath.Join(t.TempDir(), "run-1")
			if err := os.Mkdir(runDir, 0o700); err != nil {
				t.Fatal(err)
			}
			env := f.env()
			env.RemoveRunDir = func() { _ = os.RemoveAll(runDir) }
			trash := env.Trash
			var atTrash Result
			var hadResult, runDirAtTrash bool
			env.Trash = func(p string) error {
				if p == f.job.ImagePath {
					b, err := os.ReadFile(f.resPath())
					if hadResult = err == nil; hadResult {
						atTrash = rbDecode(t, b)
					}
					_, err = os.Lstat(runDir)
					runDirAtTrash = err == nil
				}
				return trash(p)
			}
			r := RunInstaller(context.Background(), f.job, env)
			if r.Outcome != moverOutcome[c.kind] {
				t.Fatalf("result = %+v", r)
			}
			if f.imageTrashed() != 1 {
				t.Fatalf("trashed = %+v, want the image once", f.trashed)
			}
			if !hadResult || atTrash.Outcome != r.Outcome {
				t.Errorf("result file at the image Trash = %+v (present %v), want %s written first", atTrash, hadResult, r.Outcome)
			}
			if !runDirAtTrash {
				t.Error("run folder already removed when the image Trash was called (tccd cannot prompt)")
			}
			if _, err := os.Lstat(runDir); err == nil {
				t.Error("run folder left after the Trash returned")
			}
			if len(f.opens) != 1 {
				t.Errorf("opens = %+v, want one relaunch before the Trash", f.opens)
			}
		})
	}
}

// A refused Trash (Don't Allow on the Downloads prompt) keeps the DMG and changes no outcome.
func TestInstaller_MoverTrashFailureKeepsOutcome(t *testing.T) {
	for _, kind := range []string{KindMove, KindOpenInstalled} {
		t.Run(kind, func(t *testing.T) {
			f := newMoveFix(t, kind, false)
			f.trashErr = errors.New("you don't have permission")
			r := f.run()
			if r.Outcome != moverOutcome[kind] {
				t.Fatalf("result = %+v, want %s", r, moverOutcome[kind])
			}
			if got, ok := f.fileResult(); !ok || got.Outcome != moverOutcome[kind] {
				t.Errorf("result file = %+v, %v", got, ok)
			}
			if _, err := os.Lstat(f.job.ImagePath); err != nil {
				t.Errorf("DMG gone after a refused Trash: %v", err)
			}
			if f.logged("dmg kept") != 1 {
				t.Errorf("logs = %q, want one dmg kept", f.logs)
			}
		})
	}
}

// An unanswered Downloads prompt must not hold the installer: it gives up after trashWait.
func TestInstaller_MoverTrashBounded(t *testing.T) {
	old := trashWait
	trashWait = 20 * time.Millisecond
	t.Cleanup(func() { trashWait = old })
	f := newMoveFix(t, KindMove, false)
	env := f.env()
	removed := false
	env.RemoveRunDir = func() { removed = true }
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	env.Trash = func(string) error { <-release; return nil }
	done := make(chan Result, 1)
	go func() { done <- RunInstaller(context.Background(), f.job, env) }()
	var r Result
	select {
	case r = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("RunInstaller still waiting on the Trash")
	}
	if r.Outcome != OutcomeUpdated {
		t.Fatalf("result = %+v", r)
	}
	if got, ok := f.fileResult(); !ok || got.Outcome != OutcomeUpdated {
		t.Errorf("result file = %+v, %v", got, ok)
	}
	if f.logged("dmg kept") != 1 {
		t.Errorf("logs = %q, want one dmg kept", f.logs)
	}
	if !removed {
		t.Error("run folder not removed after the Trash wait ran out")
	}
}
