package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// shortHome points HOME at a short /tmp dir so DefaultSocketPath fits the
// sun_path limit and each test has its own socket and lock.
func shortHome(t *testing.T) string {
	t.Helper()
	d, err := os.MkdirTemp("/tmp", "pdh-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(d) })
	t.Setenv("HOME", d)
	t.Setenv("XDG_CONFIG_HOME", d)
	t.Setenv("XPC_SERVICE_NAME", "")
	return d
}

// recordExit replaces daemonExit with a recorder of the codes it was given.
func recordExit(t *testing.T) *[]int {
	t.Helper()
	var codes []int
	old := daemonExit
	daemonExit = func(c int) { codes = append(codes, c) }
	t.Cleanup(func() { daemonExit = old })
	return &codes
}

// okSteps makes the four fatal startup steps succeed without doing anything.
func okSteps(t *testing.T) {
	t.Helper()
	c, b, r, a := cleanupStaleSocket, bootstrapCapabilityState, startRelay, startAPI
	cleanupStaleSocket = func(string) error { return nil }
	bootstrapCapabilityState = func(*API) error { return nil }
	startRelay = func(*API) (int, error) { return 0, nil }
	startAPI = func(*API, string) error { return nil }
	t.Cleanup(func() { cleanupStaleSocket, bootstrapCapabilityState, startRelay, startAPI = c, b, r, a })
}

// Research Pitfall 3 (T-19-07): launchd restarts on a non-zero exit only, so
// every real startup failure exits 1, and "already running" exits 0.
func TestRunDaemonCore_ExitCodes(t *testing.T) {
	boom := errors.New("boom")
	fail := map[string]func(t *testing.T){
		"stale socket cleanup": func(t *testing.T) { cleanupStaleSocket = func(string) error { return boom } },
		"capability bootstrap": func(t *testing.T) { bootstrapCapabilityState = func(*API) error { return boom } },
		"relay start":          func(t *testing.T) { startRelay = func(*API) (int, error) { return 0, boom } },
		"api start":            func(t *testing.T) { startAPI = func(*API, string) error { return boom } },
		"lock file unusable": func(t *testing.T) { // a directory where the lock file belongs
			if err := os.MkdirAll(filepath.Join(filepath.Dir(DefaultSocketPath()), "daemon.lock"), 0o700); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, breakStep := range fail {
		t.Run(name, func(t *testing.T) {
			shortHome(t)
			okSteps(t)
			codes := recordExit(t)
			breakStep(t)
			runDaemonCore(context.Background())
			if len(*codes) != 1 || (*codes)[0] != 1 {
				t.Fatalf("daemonExit calls = %v, want [1]", *codes)
			}
		})
	}
	t.Run("lock held by another daemon", func(t *testing.T) {
		shortHome(t)
		codes := recordExit(t)
		old := lockWait
		lockWait = 20 * time.Millisecond
		t.Cleanup(func() { lockWait = old })
		lock, err := lockDaemon(filepath.Join(filepath.Dir(DefaultSocketPath()), "daemon.lock"), 0)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = lock.Close() }()
		runDaemonCore(context.Background())
		if len(*codes) != 0 {
			t.Fatalf("daemonExit calls = %v, want none (exit 0, launchd leaves it down)", *codes)
		}
	})
}

func TestShouldRehome(t *testing.T) {
	ok := HealthResponse{Status: "ok", Version: "v1.2.0", Handoff: HandoffVersion}
	with := func(f func(*HealthResponse)) HealthResponse { h := ok; f(&h); return h }
	for _, tc := range []struct {
		name  string
		h     HealthResponse
		build string
		want  bool
	}{
		{"same build", ok, "v1.2.0", true},
		{"older holder", ok, "v1.3.0", true},
		{"newer holder", ok, "v1.1.0", false},
		{"already launchd-managed", with(func(h *HealthResponse) { h.Launchd = true }), "v1.2.0", false},
		{"cannot hand off", with(func(h *HealthResponse) { h.Handoff = 0 }), "v1.2.0", false},
		{"not ok", with(func(h *HealthResponse) { h.Status = "starting" }), "v1.2.0", false},
		{"dev builds", with(func(h *HealthResponse) { h.Version = "dev" }), "dev", true},
	} {
		if got := shouldRehome(tc.h, tc.build); got != tc.want {
			t.Errorf("%s: shouldRehome = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// healthyHolder is a healthy daemon answering /health at the default socket while a
// test holds the daemon lock, like the unmanaged daemon an upgrade leaves.
func healthyHolder(t *testing.T, h HealthResponse) {
	t.Helper()
	sock := DefaultSocketPath()
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { //nolint:errcheck
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(h)
	}))
	lock, err := lockDaemon(filepath.Join(filepath.Dir(sock), "daemon.lock"), 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lock.Close() })
	old := lockWait
	lockWait = 20 * time.Millisecond
	t.Cleanup(func() { lockWait = old })
}

// rehomeRig records the in-process takeover and serve seams.
type rehomeRig struct {
	takeovers []takeoverDeps
	serves    int
	exits     *[]int
}

func newRehomeRig(t *testing.T, code int) *rehomeRig {
	t.Helper()
	r := &rehomeRig{exits: recordExit(t)}
	oldTakeover, oldServe, oldManaged := runTakeoverStep, serveStep, launchdManaged
	runTakeoverStep = func(_ context.Context, d takeoverDeps) int { r.takeovers = append(r.takeovers, d); return code }
	serveStep = func(context.Context, *API, *SessionEngine, string) { r.serves++ }
	t.Cleanup(func() { runTakeoverStep, serveStep, launchdManaged = oldTakeover, oldServe, oldManaged })
	return r
}

// DSHR-01 re-home: the launchd job's daemon finds a healthy unmanaged daemon
// holding the lock and takes its sessions over in-process, then serves them.
func TestRunDaemonCore_RehomeCommitted(t *testing.T) {
	shortHome(t)
	t.Setenv("XPC_SERVICE_NAME", LaunchdLabel)
	useBuild(t, "v2")
	healthyHolder(t, HealthResponse{Status: "ok", Version: "v2", Handoff: HandoffVersion})
	r := newRehomeRig(t, takeoverCommitted)
	runDaemonCore(context.Background())
	if len(r.takeovers) != 1 {
		t.Fatalf("runTakeover calls = %d, want 1", len(r.takeovers))
	}
	d := r.takeovers[0]
	if d.socketPath != DefaultSocketPath() || d.lockPath != filepath.Join(filepath.Dir(DefaultSocketPath()), "daemon.lock") || d.build != "v2" || d.api == nil {
		t.Errorf("takeoverDeps = %+v, want the real socket and lock paths, build v2 and an API", d)
	}
	if r.serves != 1 || len(*r.exits) != 0 {
		t.Errorf("serves %d, exits %v; want 1 serve and no exit", r.serves, *r.exits)
	}
	if !launchdManaged {
		t.Error("the re-homed daemon does not report launchd true")
	}
}

// A refused, busy or failed re-home exits 0 (launchd would otherwise retry
// every 10 s and freeze writes each time); nothing else re-homes.
func TestRunDaemonCore_RehomeRefusedExitsZero(t *testing.T) {
	for _, code := range []int{takeoverRefused, takeoverBusy, takeoverFailed} {
		t.Run(moveFailedReason(code), func(t *testing.T) {
			shortHome(t)
			t.Setenv("XPC_SERVICE_NAME", LaunchdLabel)
			healthyHolder(t, HealthResponse{Status: "ok", Handoff: HandoffVersion})
			r := newRehomeRig(t, code)
			runDaemonCore(context.Background())
			if len(r.takeovers) != 1 || r.serves != 0 || len(*r.exits) != 0 {
				t.Fatalf("takeovers %d, serves %d, exits %v; want 1, 0, none", len(r.takeovers), r.serves, *r.exits)
			}
		})
	}
	for name, tc := range map[string]struct {
		managed bool
		h       HealthResponse
	}{
		"not launchd-managed":       {false, HealthResponse{Status: "ok", Handoff: HandoffVersion}},
		"holder is launchd-managed": {true, HealthResponse{Status: "ok", Handoff: HandoffVersion, Launchd: true}},
		"holder cannot hand off":    {true, HealthResponse{Status: "ok"}},
		"holder is a newer build":   {true, HealthResponse{Status: "ok", Version: "v9.0.0", Handoff: HandoffVersion}},
	} {
		t.Run(name, func(t *testing.T) {
			shortHome(t)
			useBuild(t, "v1.0.0")
			if tc.managed {
				t.Setenv("XPC_SERVICE_NAME", LaunchdLabel)
			}
			healthyHolder(t, tc.h)
			r := newRehomeRig(t, takeoverCommitted)
			runDaemonCore(context.Background())
			if len(r.takeovers) != 0 || r.serves != 0 || len(*r.exits) != 0 {
				t.Fatalf("takeovers %d, serves %d, exits %v; want none", len(r.takeovers), r.serves, *r.exits)
			}
		})
	}
	t.Run("holder does not answer", func(t *testing.T) {
		shortHome(t)
		t.Setenv("XPC_SERVICE_NAME", LaunchdLabel)
		old := lockWait
		lockWait = 20 * time.Millisecond
		t.Cleanup(func() { lockWait = old })
		lock, err := lockDaemon(filepath.Join(filepath.Dir(DefaultSocketPath()), "daemon.lock"), 0)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = lock.Close() }()
		r := newRehomeRig(t, takeoverCommitted)
		runDaemonCore(context.Background())
		if len(r.takeovers) != 0 || len(*r.exits) != 0 {
			t.Fatalf("takeovers %d, exits %v; want none", len(r.takeovers), *r.exits)
		}
	})
}
