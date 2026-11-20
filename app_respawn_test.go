package main

import (
	"errors"
	"fmt"
	"net"
	"net/http"
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

// Review it3 IN-01: a Retry that lands after Quit All's shutdown never spawns
// a daemon that would outlive the app.
func TestRetryDaemon_QuittingSkipsEnsure(t *testing.T) {
	respawnHome(t)
	rec := &respawnEnsures{}
	app := &App{startupDone: closedChan(), ensureDaemonFunc: rec.ensure}
	app.quitting.Store(true)

	if err := app.RetryDaemon(); err == nil {
		t.Error("RetryDaemon = nil while quitting; want an error")
	}
	if rec.calls() != 0 {
		t.Fatal("ensure ran while quitting")
	}
}

// IN-08: Retry's write of the daemon error and init's read of it race unless
// they are synchronized (run with -race).
func TestDaemonError_RetryAndReadConcurrently(t *testing.T) {
	respawnHome(t)
	app := &App{startupDone: closedChan(), ensureDaemonFunc: func(string) error { return errors.New("boom") }}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); _ = app.RetryDaemon() }()
		go func() { defer wg.Done(); _ = app.GetDaemonError() }()
	}
	wg.Wait()
	if got := app.GetDaemonError(); got != "boom" {
		t.Fatalf("GetDaemonError = %q, want boom", got)
	}
}

// IN-01: an error that is showing (startup failed, or the third respawn
// raised daemon:error) clears once the daemon is healthy and the streak has
// ended, and the frontend gets daemon:recovered to reload like Retry does.
func TestRespawnDaemon_RecoveryClearsError(t *testing.T) {
	sock := respawnHome(t)
	startTestDaemon(t, sock)
	var mu sync.Mutex
	var events []string
	app := &App{
		client:              daemon.NewDaemonClient(sock),
		startupDone:         closedChan(),
		ensureDaemonFunc:    func(string) error { return nil },
		applyAppearanceFunc: func(string) {},
		emitEventFunc: func(name string, _ any) {
			mu.Lock()
			defer mu.Unlock()
			events = append(events, name)
		},
	}
	boom := errors.New(respawnKeepsStoppingMsg)
	app.daemonErr.Store(&boom)
	t0 := time.Now()
	app.respawnAttempts, app.lastRespawn = respawnErrorAfter, t0

	app.respawnDaemonIfDown(t0.Add(30 * time.Second)) // healthy, but the streak is still on
	if app.GetDaemonError() == "" {
		t.Fatal("error cleared 30 s after a third respawn; want it kept until the streak ends")
	}
	app.respawnDaemonIfDown(t0.Add(time.Minute))
	if got := app.GetDaemonError(); got != "" {
		t.Fatalf("GetDaemonError after a healthy minute = %q, want empty", got)
	}
	app.respawnDaemonIfDown(t0.Add(time.Minute + 5*time.Second))
	mu.Lock()
	defer mu.Unlock()
	if len(events) != 1 || events[0] != "daemon:recovered" {
		t.Fatalf("events = %q, want one daemon:recovered", events)
	}
}

// 08-06-REVIEW IN-05: a daemon that answers again after a failed startup is
// adopted only through startup's version check and restart policy (ensure).
// One from this build is adopted as before, without ensure; a failed ensure
// keeps the error.
func TestRespawnDaemon_RecoveryChecksVersion(t *testing.T) {
	old := daemon.BuildVersion
	daemon.BuildVersion = "v2"
	t.Cleanup(func() { daemon.BuildVersion = old })
	for _, tc := range []struct {
		name          string
		ver           string
		ensureErr     error
		wantEnsures   int
		wantRecovered bool
	}{
		{"another build goes through ensure", "v1", nil, 1, true},
		{"another build whose ensure fails stays an error", "v1", errors.New("lock held"), 1, false},
		{"this build is adopted directly", "v2", nil, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sock := respawnHome(t)
			ln, err := net.Listen("unix", sock)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { ln.Close() })
			go http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { //nolint:errcheck
				if r.URL.Path != "/health" {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"status":"ok","version":%q}`, tc.ver)
			}))
			rec := &respawnEnsures{err: tc.ensureErr}
			var events []string
			app := &App{
				client:              daemon.NewDaemonClient(sock),
				startupDone:         closedChan(),
				ensureDaemonFunc:    rec.ensure,
				applyAppearanceFunc: func(string) {},
				emitEventFunc:       func(name string, _ any) { events = append(events, name) },
			}
			boom := errors.New("EnsureDaemon: daemon lock still held after 15s")
			app.daemonErr.Store(&boom)
			app.respawnDaemonIfDown(time.Now())
			if rec.calls() != tc.wantEnsures {
				t.Fatalf("ensure ran %d times, want %d", rec.calls(), tc.wantEnsures)
			}
			recovered := len(events) == 1 && events[0] == "daemon:recovered"
			if recovered != tc.wantRecovered || (app.daemonErr.Load() == nil) != tc.wantRecovered {
				t.Fatalf("events %q, daemon error %v; want recovered %v", events, app.daemonErr.Load() != nil, tc.wantRecovered)
			}
		})
	}
}

// IN-01: the third respawn's daemon:error is also what GetDaemonError reports.
func TestRespawnDaemon_ThirdAttemptSetsError(t *testing.T) {
	sock := respawnHome(t)
	app := &App{client: daemon.NewDaemonClient(sock), startupDone: closedChan(), ensureDaemonFunc: func(string) error { return errors.New("spawn boom") }}
	t0 := time.Now()
	for _, off := range []time.Duration{0, 10, 30} {
		app.respawnDaemonIfDown(t0.Add(off * time.Second))
	}
	if got := app.GetDaemonError(); got != "spawn boom" {
		t.Fatalf("GetDaemonError = %q, want spawn boom", got)
	}
}

// HARD-04: a health blip in the first 10 s after a successful startup spends
// no respawn attempt and raises nothing.
func TestRespawnDaemon_LaunchGraceSkips(t *testing.T) {
	sock := respawnHome(t)
	rec := &respawnEnsures{}
	ev := &respawnEvents{}
	t0 := time.Now()
	app := &App{client: daemon.NewDaemonClient(sock), startupDone: closedChan(), launchedAt: t0, ensureDaemonFunc: rec.ensure, emitEventFunc: ev.emit}

	app.respawnDaemonIfDown(t0)
	app.respawnDaemonIfDown(t0.Add(5 * time.Second))

	if rec.calls() != 0 {
		t.Fatalf("ensure ran %d times inside the launch grace", rec.calls())
	}
	if app.respawnAttempts != 0 || len(ev.got()) != 0 {
		t.Fatalf("attempts = %d, events = %q; want 0 and none", app.respawnAttempts, ev.got())
	}
}

// HARD-04 / R6: a daemon that died right after launch still comes back once
// the grace ends.
func TestRespawnDaemon_LaunchGraceEnds(t *testing.T) {
	sock := respawnHome(t)
	rec := &respawnEnsures{}
	t0 := time.Now()
	app := &App{client: daemon.NewDaemonClient(sock), startupDone: closedChan(), launchedAt: t0, ensureDaemonFunc: rec.ensure}

	app.respawnDaemonIfDown(t0)
	app.respawnDaemonIfDown(t0.Add(respawnLaunchGrace))

	if rec.calls() != 1 {
		t.Fatalf("ensure calls = %d, want 1 once the grace ended", rec.calls())
	}
}

// R6: after the first healthy tick there is no grace, even inside the window.
func TestRespawnDaemon_HealthySeenEndsGrace(t *testing.T) {
	sock := respawnHome(t)
	stop := startTestDaemon(t, sock)
	rec := &respawnEnsures{}
	t0 := time.Now()
	app := &App{client: daemon.NewDaemonClient(sock), startupDone: closedChan(), launchedAt: t0, ensureDaemonFunc: rec.ensure}

	app.respawnDaemonIfDown(t0) // healthy
	stop()
	app.client = daemon.NewDaemonClient(sock) // api.Stop keeps the old keep-alive conn open
	app.respawnDaemonIfDown(t0.Add(5 * time.Second))

	if rec.calls() != 1 {
		t.Fatalf("ensure calls = %d, want 1 (no grace after a healthy tick)", rec.calls())
	}
}

// A failed startup leaves launchedAt zero, so the poller retries at once.
func TestRespawnDaemon_FailedStartupNoGrace(t *testing.T) {
	sock := respawnHome(t)
	rec := &respawnEnsures{}
	app := &App{client: daemon.NewDaemonClient(sock), startupDone: closedChan(), ensureDaemonFunc: rec.ensure}
	boom := errors.New("boom")
	app.daemonErr.Store(&boom)

	app.respawnDaemonIfDown(time.Now())

	if rec.calls() != 1 {
		t.Fatalf("ensure calls = %d, want 1 after a failed startup", rec.calls())
	}
}

func TestStartDaemon_SetsLaunchedAtOnSuccess(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{{"success", nil}, {"failure", errors.New("boom")}} {
		t.Run(tc.name, func(t *testing.T) {
			a := &App{
				startupDone:         make(chan struct{}),
				ensureDaemonFunc:    func(string) error { return tc.err },
				emitEventFunc:       func(string, any) {},
				applyAppearanceFunc: func(string) {},
			}
			a.startDaemon("unused")
			if a.launchedAt.IsZero() != (tc.err != nil) {
				t.Fatalf("launchedAt = %v for err=%v", a.launchedAt, tc.err)
			}
		})
	}
}
