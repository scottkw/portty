package main

import (
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/daemon"
)

// respawnHome points HOME at a short /tmp dir so DefaultSocketPath fits
// sun_path, and returns that socket path. Nothing listens on it yet.
func respawnHome(t *testing.T) string {
	t.Helper()
	home, err := os.MkdirTemp("/tmp", "rs")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(home) })
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	return daemon.DefaultSocketPath()
}

// startTestDaemon serves a real daemon API (relay started) on socketPath and
// returns its stop func.
func startTestDaemon(t *testing.T, socketPath string) func() {
	t.Helper()
	engine := daemon.NewSessionEngine()
	engine.ConfigDirForTest(t.TempDir())
	api := daemon.NewAPI(engine)
	if err := api.Start(socketPath); err != nil {
		t.Fatalf("api.Start: %v", err)
	}
	if _, err := api.StartRelay(); err != nil {
		t.Fatalf("StartRelay: %v", err)
	}
	var once sync.Once
	stop := func() { once.Do(func() { _ = api.Stop() }) }
	t.Cleanup(stop)
	return stop
}

// respawnEnsures counts ensure-seam calls and the paths they got.
type respawnEnsures struct {
	mu    sync.Mutex
	paths []string
	err   error
}

func (r *respawnEnsures) ensure(path string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.paths = append(r.paths, path)
	return r.err
}

func (r *respawnEnsures) calls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.paths)
}

// respawnEvents keeps every daemon:error payload.
type respawnEvents struct {
	mu   sync.Mutex
	errs []string
}

func (r *respawnEvents) emit(name string, data any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if name == "daemon:error" {
		r.errs = append(r.errs, data.(string))
	}
}

func (r *respawnEvents) got() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.errs...)
}

func closedChan() chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

func TestRespawnDaemon_DeadDaemonRunsEnsure(t *testing.T) {
	sock := respawnHome(t)
	rec := &respawnEnsures{}
	app := &App{client: daemon.NewDaemonClient(sock), startupDone: closedChan(), ensureDaemonFunc: rec.ensure}

	app.respawnDaemonIfDown(time.Now())

	if rec.calls() != 1 || rec.paths[0] != daemon.DefaultSocketPath() {
		t.Fatalf("ensure calls = %v, want one with %q", rec.paths, daemon.DefaultSocketPath())
	}
}

func TestRespawnDaemon_HealthyDaemonSkipsEnsure(t *testing.T) {
	sock := respawnHome(t)
	startTestDaemon(t, sock)
	rec := &respawnEnsures{}
	app := &App{client: daemon.NewDaemonClient(sock), startupDone: closedChan(), ensureDaemonFunc: rec.ensure}

	t0 := time.Now()
	for i := 0; i < 4; i++ {
		app.respawnDaemonIfDown(t0.Add(time.Duration(i) * 5 * time.Second))
	}
	if rec.calls() != 0 {
		t.Fatalf("ensure ran %d times on a healthy daemon", rec.calls())
	}
}

func TestRespawnDaemon_BeforeStartupSkipsEnsure(t *testing.T) {
	sock := respawnHome(t)
	rec := &respawnEnsures{}
	app := &App{client: daemon.NewDaemonClient(sock), startupDone: make(chan struct{}), ensureDaemonFunc: rec.ensure}

	app.respawnDaemonIfDown(time.Now())

	if rec.calls() != 0 {
		t.Fatal("ensure ran before startup finished")
	}
}

func TestRespawnDaemon_SerializedWithRetry(t *testing.T) {
	sock := respawnHome(t)
	var inFlight, maxInFlight, calls atomic.Int32
	entered := make(chan struct{})
	release := make(chan struct{})
	ensure := func(string) error {
		n := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			m := maxInFlight.Load()
			if n <= m || maxInFlight.CompareAndSwap(m, n) {
				break
			}
		}
		if calls.Add(1) == 1 {
			close(entered)
			<-release
		}
		return nil
	}
	app := &App{
		client:              daemon.NewDaemonClient(sock),
		startupDone:         closedChan(),
		ensureDaemonFunc:    ensure,
		applyAppearanceFunc: func(string) {},
		respawnAttempts:     2, // a streak in progress
		lastRespawn:         time.Now().Add(-time.Hour),
	}

	done := make(chan error, 1)
	go func() { done <- app.RetryDaemon() }()
	<-entered

	app.respawnDaemonIfDown(time.Now()) // Retry holds daemonMu: must skip

	close(release)
	if err := <-done; err != nil {
		t.Fatalf("RetryDaemon: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("ensure ran %d times, want 1 (respawn must skip while Retry runs)", calls.Load())
	}
	if maxInFlight.Load() != 1 {
		t.Fatalf("max concurrent ensure calls = %d, want 1", maxInFlight.Load())
	}
	app.daemonMu.Lock()
	attempts := app.respawnAttempts
	app.daemonMu.Unlock()
	if attempts != 0 {
		t.Fatalf("respawn streak after a successful Retry = %d, want 0", attempts)
	}
}

func TestRespawnDaemon_RateLimitedAndSurfaced(t *testing.T) {
	cases := []struct {
		name    string
		seamErr error
		wantMsg string
	}{
		{"ensure fails", errors.New("spawn boom"), "spawn boom"},
		{"ensure succeeds but daemon keeps stopping", nil, respawnKeepsStoppingMsg},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sock := respawnHome(t)
			rec := &respawnEnsures{err: tc.seamErr}
			ev := &respawnEvents{}
			app := &App{client: daemon.NewDaemonClient(sock), startupDone: closedChan(), ensureDaemonFunc: rec.ensure, emitEventFunc: ev.emit}

			t0 := time.Now()
			var ranAt []time.Duration
			for _, off := range []time.Duration{0, 5, 10, 15, 30, 35, 70} {
				before := rec.calls()
				app.respawnDaemonIfDown(t0.Add(off * time.Second))
				if rec.calls() > before {
					ranAt = append(ranAt, off)
				}
				if off == 15 && len(ev.got()) != 0 {
					t.Fatalf("daemon:error before the third attempt: %v", ev.got())
				}
			}
			want := []time.Duration{0, 10, 30, 70}
			if len(ranAt) != len(want) {
				t.Fatalf("ensure ran at %v s, want %v s", ranAt, want)
			}
			for i := range want {
				if ranAt[i] != want[i] {
					t.Fatalf("ensure ran at %v s, want %v s", ranAt, want)
				}
			}
			if got := ev.got(); len(got) != 1 || got[0] != tc.wantMsg {
				t.Fatalf("daemon:error events = %q, want one %q", got, tc.wantMsg)
			}
		})
	}
}

func TestRespawnDaemon_HealthyMinuteResetsStreak(t *testing.T) {
	sock := respawnHome(t)
	rec := &respawnEnsures{err: errors.New("spawn boom")}
	ev := &respawnEvents{}
	app := &App{client: daemon.NewDaemonClient(sock), startupDone: closedChan(), ensureDaemonFunc: rec.ensure, emitEventFunc: ev.emit}

	t0 := time.Now()
	for _, off := range []time.Duration{0, 10, 30} {
		app.respawnDaemonIfDown(t0.Add(off * time.Second))
	}
	if len(ev.got()) != 1 {
		t.Fatalf("after three failures: events = %q, want 1", ev.got())
	}

	stop := startTestDaemon(t, sock)
	app.respawnDaemonIfDown(t0.Add(90 * time.Second)) // healthy, 60 s after the last attempt
	stop()
	// api.Stop leaves the client's keep-alive connection open, which kill -9
	// would not; a fresh client sees the daemon as gone.
	app.client = daemon.NewDaemonClient(sock)

	for _, off := range []time.Duration{100, 110, 130} {
		app.respawnDaemonIfDown(t0.Add(off * time.Second))
	}
	if rec.calls() != 6 {
		t.Fatalf("ensure calls = %d, want 6", rec.calls())
	}
	if len(ev.got()) != 2 {
		t.Fatalf("events = %q, want a second daemon:error after the reset", ev.got())
	}
}

// WR-04: once Quit All has begun, the poller never brings the daemon back.
func TestRespawnDaemon_QuittingSkipsEnsure(t *testing.T) {
	sock := respawnHome(t)
	rec := &respawnEnsures{}
	app := &App{client: daemon.NewDaemonClient(sock), startupDone: closedChan(), ensureDaemonFunc: rec.ensure}
	app.quitting.Store(true)

	app.respawnDaemonIfDown(time.Now())

	if rec.calls() != 0 {
		t.Fatal("ensure ran while quitting")
	}
}
