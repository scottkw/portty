package main

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/daemon"
)

// fakeLaunchctl is a fake launchd behind daemon's launchctl seam: it records
// every call and never touches the real launchd. loaded and disabled describe
// Portty's job; a kickstart runs onKickstart (which brings a test daemon up).
type fakeLaunchctl struct {
	mu          sync.Mutex
	calls       []string // subcommands, in order
	loaded      bool
	disabled    bool
	onKickstart func()
}

func newFakeLaunchctl(t *testing.T) *fakeLaunchctl {
	t.Helper()
	f := &fakeLaunchctl{}
	t.Cleanup(daemon.SetLaunchctlForTest(f.run))
	return f
}

func (f *fakeLaunchctl) run(_ context.Context, args ...string) ([]byte, error) {
	f.mu.Lock()
	f.calls = append(f.calls, args[0])
	loaded, disabled, onKick := f.loaded, f.disabled, f.onKickstart
	f.mu.Unlock()
	switch args[0] {
	case "print":
		if !loaded {
			return []byte(`Could not find service "` + daemon.LaunchdLabel + `" in domain`), errors.New("exit status 113")
		}
		return []byte("job = {\n\tstate = not running\n}\n"), nil
	case "print-disabled":
		out := "disabled services = {\n"
		if disabled {
			out += "\t\"" + daemon.LaunchdLabel + "\" => disabled\n"
		}
		return []byte(out + "}\n"), nil
	case "kickstart":
		if onKick != nil {
			onKick()
		}
	}
	return nil, nil
}

func (f *fakeLaunchctl) count(sub string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if c == sub {
			n++
		}
	}
	return n
}

func (f *fakeLaunchctl) set(loaded, disabled bool) {
	f.mu.Lock()
	f.loaded, f.disabled = loaded, disabled
	f.mu.Unlock()
}

// installedApp makes the running executable look like ~/Applications/Portty.app
// under the test HOME, with the agent seam recording its calls.
func installedApp(t *testing.T) (agentCalls *[]string) {
	t.Helper()
	exe := filepath.Join(os.Getenv("HOME"), "Applications/Portty.app/Contents/MacOS/Portty")
	if err := os.MkdirAll(filepath.Dir(exe), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	oldExe, oldAgent := executablePathFunc, ensureAgentFunc
	t.Cleanup(func() { executablePathFunc, ensureAgentFunc = oldExe, oldAgent })
	executablePathFunc = func() (string, error) { return exe, nil }
	agentCalls = new([]string)
	ensureAgentFunc = func(_ context.Context, e string) (daemon.AgentState, error) {
		*agentCalls = append(*agentCalls, e)
		return daemon.AgentReady, nil
	}
	return agentCalls
}

// fallbackRecorder replaces the detached spawn: it counts calls and brings a
// test daemon up at sock, as a real fallback daemon would be.
func fallbackRecorder(t *testing.T, sock string) *int {
	t.Helper()
	n := new(int)
	t.Cleanup(daemon.SetFallbackSpawnForTest(func(string) (<-chan struct{}, func(), error) {
		*n++
		startTestDaemon(t, sock)
		return nil, nil, nil
	}))
	return n
}

func quietApp() *App {
	return &App{
		startupDone:         make(chan struct{}),
		emitEventFunc:       func(string, any) {},
		applyAppearanceFunc: func(string) {},
	}
}

// A GUI start with Login Items turned off still starts the daemon, through the
// fallback, and marks only the desktop for repair: no daemon error appears.
func TestStartDaemon_DisabledAgentFallsBackAndMarksRepair(t *testing.T) {
	sock := respawnHome(t)
	installedApp(t)
	ensureAgentFunc = func(context.Context, string) (daemon.AgentState, error) {
		return daemon.AgentReady, daemon.ErrAgentDisabled
	}
	f := newFakeLaunchctl(t)
	f.set(false, true)
	fallbacks := fallbackRecorder(t, sock)

	a := quietApp()
	a.startDaemon(sock)

	if got := a.GetDaemonError(); got != "" {
		t.Errorf("GetDaemonError = %q, want none while the fallback daemon runs", got)
	}
	if *fallbacks != 1 {
		t.Errorf("fallback spawns = %d, want 1", *fallbacks)
	}
	if got := a.GetDaemonRepair(); got != "login_items" {
		t.Errorf("GetDaemonRepair = %q, want login_items", got)
	}
	if n := f.count("kickstart"); n != 0 {
		t.Errorf("kickstart ran %d times though the agent is disabled", n)
	}
}

func TestOpenLoginItems_FixedURL(t *testing.T) {
	old := openSystemURL
	t.Cleanup(func() { openSystemURL = old })
	var got []string
	openSystemURL = func(url string) error { got = append(got, url); return nil }
	if err := (&App{}).OpenLoginItems(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{openURLLoginItems}) {
		t.Errorf("opened %v, want exactly %q", got, openURLLoginItems)
	}
	if !strings.HasPrefix(openURLLoginItems, "x-apple.systempreferences:com.apple.LoginItems") {
		t.Errorf("unexpected Login Items URL %q", openURLLoginItems)
	}
	openSystemURL = old
	if err := (&App{}).OpenLoginItems(); err == nil {
		t.Error("the real open ran under the test binary")
	}
}

// ---- Task 2: respawn, retry, re-home, legacy cleanup, installed-bundle rule ----

// rehomeRig drives startDaemon through the ensure seam with recorders for the
// re-home kickstart, the agent state, the legacy cleanup and /health.
type rehomeRig struct {
	mu         sync.Mutex
	kicks      int
	loaded     bool
	disabled   bool
	health     daemon.HealthResponse
	healthErr  error
	cleanupPID []int
	cleanupOK  bool
}

func newRehomeRig(t *testing.T) *rehomeRig {
	t.Helper()
	respawnHome(t)
	r := &rehomeRig{loaded: true, health: daemon.HealthResponse{Status: "ok", PID: 4242}}
	oldK, oldL, oldC, oldH := kickstartFunc, agentLoadedFunc, legacyCleanupFunc, daemonHealthOf
	t.Cleanup(func() {
		kickstartFunc, agentLoadedFunc, legacyCleanupFunc, daemonHealthOf = oldK, oldL, oldC, oldH
	})
	kickstartFunc = func(context.Context) error { r.mu.Lock(); r.kicks++; r.mu.Unlock(); return nil }
	agentLoadedFunc = func(context.Context) (bool, bool, int, error) {
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.loaded, r.disabled, 0, nil
	}
	legacyCleanupFunc = func(_ context.Context, pid int) (bool, error) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.cleanupPID = append(r.cleanupPID, pid)
		return r.cleanupOK, nil
	}
	daemonHealthOf = func(string) (daemon.HealthResponse, error) {
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.health, r.healthErr
	}
	return r
}

// app is an App whose ensure succeeds at once (nil startupDone: startDaemon may run twice).
func (r *rehomeRig) app() *App {
	return &App{
		emitEventFunc:       func(string, any) {},
		applyAppearanceFunc: func(string) {},
		ensureDaemonFunc:    func(string) error { return nil },
	}
}

func (r *rehomeRig) kickCount() int { r.mu.Lock(); defer r.mu.Unlock(); return r.kicks }

func TestRespawnDaemonIfDown_KickstartFirstFallback(t *testing.T) {
	t.Run("loaded and enabled kickstarts and never falls back", func(t *testing.T) {
		sock := respawnHome(t)
		f := newFakeLaunchctl(t)
		f.set(true, false)
		f.onKickstart = func() { startTestDaemon(t, sock) }
		fallbacks := fallbackRecorder(t, sock)
		rehomeKicks := 0
		oldK := kickstartFunc
		t.Cleanup(func() { kickstartFunc = oldK })
		kickstartFunc = func(context.Context) error { rehomeKicks++; return nil }
		a := &App{client: daemon.NewDaemonClient(sock), startupDone: closedChan(), emitEventFunc: func(string, any) {}}

		a.respawnDaemonIfDown(time.Now())

		if n := f.count("kickstart"); n != 1 {
			t.Errorf("kickstarts = %d, want 1", n)
		}
		if *fallbacks != 0 {
			t.Errorf("fallback spawns = %d, want 0", *fallbacks)
		}
		if rehomeKicks != 0 {
			t.Error("respawnDaemonIfDown called the re-home kickstart itself")
		}
		if a.GetDaemonRepair() != "" || a.daemonErr.Load() != nil {
			t.Errorf("repair %q, error %v; want none", a.GetDaemonRepair(), a.daemonErr.Load())
		}
	})
	t.Run("disabled falls back once and reports no daemon error", func(t *testing.T) {
		sock := respawnHome(t)
		f := newFakeLaunchctl(t)
		f.set(false, true)
		fallbacks := fallbackRecorder(t, sock)
		ev := &respawnEvents{}
		a := &App{client: daemon.NewDaemonClient(sock), startupDone: closedChan(), emitEventFunc: ev.emit}

		a.respawnDaemonIfDown(time.Now())

		if *fallbacks != 1 {
			t.Errorf("fallback spawns = %d, want 1", *fallbacks)
		}
		if n := f.count("kickstart"); n != 0 {
			t.Errorf("kickstarts = %d, want 0", n)
		}
		if got := a.GetDaemonRepair(); got != "login_items" {
			t.Errorf("GetDaemonRepair = %q, want login_items", got)
		}
		if got := ev.got(); len(got) != 0 || a.daemonErr.Load() != nil {
			t.Errorf("daemon errors %v / %v, want none", got, a.daemonErr.Load())
		}
	})
}

func TestRetryDaemon_UsesLaunchdFirst(t *testing.T) {
	sock := respawnHome(t)
	f := newFakeLaunchctl(t)
	f.set(true, false) // the user turned Login Items back on
	f.onKickstart = func() { startTestDaemon(t, sock) }
	fallbacks := fallbackRecorder(t, sock)
	a := quietApp()
	close(a.startupDone)
	a.repairKind.Store("login_items")
	downErr := errors.New("daemon down")
	a.daemonErr.Store(&downErr)

	if err := a.RetryDaemon(); err != nil {
		t.Fatalf("RetryDaemon: %v", err)
	}
	if n := f.count("kickstart"); n != 1 || *fallbacks != 0 {
		t.Errorf("kickstarts %d, fallbacks %d; want 1 and 0", n, *fallbacks)
	}
	if got := a.GetDaemonRepair(); got != "" {
		t.Errorf("GetDaemonRepair = %q after a launchd start, want none", got)
	}
}

func TestApp_RehomeRetryBackoff(t *testing.T) {
	r := newRehomeRig(t)
	a := r.app()
	a.startDaemon("unused")
	start := time.Now()

	if got := r.kickCount(); got != 1 {
		t.Fatalf("kicks at start = %d, want 1", got)
	}
	if got := a.GetDaemonRepair(); got != "rehome_pending" {
		t.Errorf("GetDaemonRepair = %q while the re-home retry is armed, want rehome_pending", got)
	}
	// Gaps after attempts 1..4 are 30 s, 1, 2, 4 min, then every 10 min; 8 in total.
	gaps := []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute}
	for range 3 {
		gaps = append(gaps, 10*time.Minute)
	}
	at := start
	want := 1
	for _, g := range gaps {
		at = at.Add(g)
		a.rehomeIfUnmanaged(at.Add(-time.Second))
		if got := r.kickCount(); got != want {
			t.Fatalf("a second early at %v: kicks = %d, want %d", at.Sub(start), got, want)
		}
		a.rehomeIfUnmanaged(at)
		want++
		if got := r.kickCount(); got != want {
			t.Fatalf("at %v: kicks = %d, want %d", at.Sub(start), got, want)
		}
	}
	if want != rehomeMaxAttempts {
		t.Fatalf("schedule covers %d attempts, want %d", want, rehomeMaxAttempts)
	}
	a.rehomeIfUnmanaged(at.Add(24 * time.Hour))
	if got := r.kickCount(); got != rehomeMaxAttempts {
		t.Errorf("kicks = %d after the cap, want %d", got, rehomeMaxAttempts)
	}
	if got := a.GetDaemonRepair(); got != "" {
		t.Errorf("GetDaemonRepair = %q after the retries ran out, want none", got)
	}

	// respawn only ever calls the ensure.
	before := r.kickCount()
	dead := &App{client: daemon.NewDaemonClient(filepath.Join(t.TempDir(), "none.sock")), startupDone: closedChan(), ensureDaemonFunc: func(string) error { return nil }}
	dead.respawnDaemonIfDown(time.Now())
	if r.kickCount() != before {
		t.Error("respawnDaemonIfDown called the re-home kickstart")
	}
}

func TestApp_RehomeStops(t *testing.T) {
	t.Run("health reports launchd true", func(t *testing.T) {
		r := newRehomeRig(t)
		a := r.app()
		a.startDaemon("unused")
		r.mu.Lock()
		r.health.Launchd = true
		r.mu.Unlock()
		a.rehomeIfUnmanaged(time.Now().Add(time.Hour))
		a.rehomeIfUnmanaged(time.Now().Add(2 * time.Hour))
		if got := r.kickCount(); got != 1 {
			t.Errorf("kicks = %d, want only the start's", got)
		}
		if got := a.GetDaemonRepair(); got != "" {
			t.Errorf("GetDaemonRepair = %q once launchd manages the daemon", got)
		}
	})
	t.Run("agent disabled", func(t *testing.T) {
		r := newRehomeRig(t)
		r.disabled = true
		a := r.app()
		a.startDaemon("unused")
		a.rehomeIfUnmanaged(time.Now().Add(time.Hour))
		if got := r.kickCount(); got != 0 {
			t.Errorf("kicks = %d with the agent disabled", got)
		}
		if got := a.GetDaemonRepair(); got != "login_items" {
			t.Errorf("GetDaemonRepair = %q, want login_items", got)
		}
	})
	t.Run("launchd already manages the daemon at start", func(t *testing.T) {
		r := newRehomeRig(t)
		r.health.Launchd = true
		a := r.app()
		a.repairKind.Store("login_items") // a stale kind from an earlier fallback
		a.startDaemon("unused")
		if got := r.kickCount(); got != 0 {
			t.Errorf("kicks = %d for a managed daemon", got)
		}
		if got := a.GetDaemonRepair(); got != "" {
			t.Errorf("GetDaemonRepair = %q for a managed daemon, want none", got)
		}
	})
}

func TestApp_LegacyCleanupAtStart(t *testing.T) {
	r := newRehomeRig(t)
	r.health.Launchd = true
	r.cleanupOK = false
	a := r.app()
	a.startDaemon("unused")
	a.startDaemon("unused") // the next GUI start
	if !slices.Equal(r.cleanupPID, []int{4242, 4242}) {
		t.Errorf("legacy cleanup pids = %v, want the health pid on every start", r.cleanupPID)
	}
	if got := a.GetDaemonError(); got != "" {
		t.Errorf("GetDaemonError = %q, a pending cleanup must never fail startup", got)
	}

	r.mu.Lock()
	r.healthErr = errors.New("no daemon")
	r.mu.Unlock()
	a.startDaemon("unused")
	if len(r.cleanupPID) != 2 {
		t.Errorf("cleanup ran with no readable health: %v", r.cleanupPID)
	}
}

func TestInstalledExecutable(t *testing.T) {
	respawnHome(t)
	home := os.Getenv("HOME")
	homeExe := filepath.Join(home, "Applications/Portty.app/Contents/MacOS/Portty")
	if err := os.MkdirAll(filepath.Dir(homeExe), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(homeExe, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := executablePathFunc
	t.Cleanup(func() { executablePathFunc = old })

	buildTree := filepath.Join(home, "src/portty/build/bin/Portty.app/Contents/MacOS/Portty")
	cases := []struct {
		name, path string
		ok         bool
	}{
		{"/Applications", "/Applications/Portty.app/Contents/MacOS/Portty", true},
		{"~/Applications", homeExe, true},
		{"build tree", buildTree, false},
		{"go run", "/var/folders/zz/T/go-build123/b001/exe/portty", false},
		{"wails dev", filepath.Join(home, "src/portty/build/bin/portty"), false},
		{"disk image", "/Volumes/Portty/Portty.app/Contents/MacOS/Portty", false},
		{"translocated", "/private/var/folders/zz/AppTranslocation/ABC/d/Portty.app/Contents/MacOS/Portty", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			executablePathFunc = func() (string, error) { return tc.path, nil }
			exe, ok, reason := installedExecutable()
			if ok != tc.ok {
				t.Fatalf("ok = %v (reason %q), want %v", ok, reason, tc.ok)
			}
			if ok && exe == "" {
				t.Error("installed but no executable returned")
			}
			if !ok && (reason == "" || strings.Contains(reason, "/")) {
				t.Errorf("reason %q must be non-empty and name no path", reason)
			}
		})
	}

	t.Run("a non-installed executable never writes the agent", func(t *testing.T) {
		r := newRehomeRig(t)
		var buf strings.Builder
		oldOut := log.Writer()
		log.SetOutput(&buf)
		t.Cleanup(func() { log.SetOutput(oldOut) })
		calls := installedApp(t)
		executablePathFunc = func() (string, error) { return buildTree, nil }
		r.app().startDaemon("unused")
		if len(*calls) != 0 {
			t.Errorf("ensureAgentFunc called for a build-tree bundle: %v", *calls)
		}
		if !strings.Contains(buf.String(), "launchd agent not written") {
			t.Errorf("log %q lacks 'launchd agent not written'", buf.String())
		}
	})
}

func TestDaemonRepair_Kinds(t *testing.T) {
	wrapped := errors.Join(daemon.ErrAgentLaunchctl, errors.New("print: exit 1"))
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"disabled", daemon.ErrAgentDisabled, "login_items"},
		{"refused", daemon.ErrAgentRefused, "login_items"},
		{"launchctl", wrapped, "login_items"},
		{"missing", daemon.ErrAgentMissing, "open_app"},
		{"unsafe path", daemon.ErrAgentUnsafePath, "move_to_applications"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := &App{}
			a.repairKind.Store(repairKindFor(tc.err))
			if got := a.GetDaemonRepair(); got != tc.want {
				t.Errorf("GetDaemonRepair = %q, want %q", got, tc.want)
			}
		})
	}
	a := &App{}
	if got := a.GetDaemonRepair(); got != "" {
		t.Errorf("fresh app: %q, want none", got)
	}
	a.rehomeArmed = true
	if got := a.GetDaemonRepair(); got != "rehome_pending" {
		t.Errorf("armed with no fallback: %q, want rehome_pending", got)
	}
	a.repairKind.Store("login_items")
	if got := a.GetDaemonRepair(); got != "login_items" {
		t.Errorf("a recorded fallback outranks the retry: %q", got)
	}
	a.diskImageLaunch.Store(true)
	if got := a.GetDaemonRepair(); got != "move_to_applications" {
		t.Errorf("a disk image launch outranks everything: %q", got)
	}
}

func TestStartDaemon_DiskImageMoveToApplications(t *testing.T) {
	sock := respawnHome(t)
	calls := installedApp(t)
	executablePathFunc = func() (string, error) { return "/Volumes/Portty/Portty.app/Contents/MacOS/Portty", nil }
	f := newFakeLaunchctl(t)
	// The ensure runs the test binary's own path, so the agent is made unusable
	// by Login Items; the disk image flag must still win over the kind it records.
	f.set(false, true)
	fallbacks := fallbackRecorder(t, sock)

	a := quietApp()
	a.startDaemon(sock)

	if len(*calls) != 0 {
		t.Errorf("ensureAgentFunc called from a disk image: %v", *calls)
	}
	if *fallbacks != 1 {
		t.Errorf("fallback spawns = %d, want 1", *fallbacks)
	}
	if got := a.GetDaemonRepair(); got != "move_to_applications" {
		t.Errorf("GetDaemonRepair = %q, want move_to_applications", got)
	}
	if a.GetDaemonError() != "" {
		t.Error("a disk image launch must still start terminals")
	}
}
