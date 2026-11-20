package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	goruntime "runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/daemon"
	"github.com/scottkw/portty/internal/tailnet"
	"github.com/scottkw/portty/internal/webserver"
)

var testSockSeq atomic.Int64

// testApp creates an App wired for testing — no Wails GUI, but all bound
// methods are functional.  It starts an in-process daemon API on a temp socket
// and wires the client, simulating what startup() does without the subprocess.
func testApp(t *testing.T) *App {
	t.Helper()
	return testAppWithSpawn(t, "/bin/cat", nil) // deterministic echo process instead of a login shell
}

// testAppWithSpawn is testApp with every session spawning path with args.
func testAppWithSpawn(t *testing.T, path string, args []string) *App {
	t.Helper()
	// Isolate config directory so settings.json doesn't leak between tests.
	if goruntime.GOOS == "darwin" {
		t.Setenv("HOME", t.TempDir())
	} else {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	}

	// Start an in-process daemon API for tests (no real subprocess).
	engine := daemon.NewSessionEngine()
	engine.SetSpawnCommandForTest(path, args)
	api := daemon.NewAPI(engine)

	seq := testSockSeq.Add(1)
	socketPath := fmt.Sprintf("/tmp/aht%d_%d.sock", os.Getpid(), seq)
	_ = os.Remove(socketPath)
	if err := api.Start(socketPath); err != nil {
		t.Fatalf("testApp: api.Start: %v", err)
	}

	// Start relay inside daemon API so GetRelayPort works.
	if _, err := api.StartRelay(); err != nil {
		t.Fatalf("testApp: StartRelay: %v", err)
	}

	client := daemon.NewDaemonClient(socketPath)
	// Brief sleep for server goroutine to be ready.
	time.Sleep(10 * time.Millisecond)

	app := &App{
		ctx:    context.Background(),
		client: client,
	}

	t.Cleanup(func() {
		engine.Manager().Shutdown()
		api.Stop()
		_ = os.Remove(socketPath)
	})
	return app
}

// testAppWithAPI creates an App + daemon API pair (like testApp) and returns
// the daemon API and SessionEngine so callers can inject test doubles.
// Phase 165 / T-165-15.
func testAppWithAPI(t *testing.T) (*App, *daemon.API, *daemon.SessionEngine) {
	t.Helper()
	if goruntime.GOOS == "darwin" {
		t.Setenv("HOME", t.TempDir())
	} else {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	}
	engine := daemon.NewSessionEngine()
	engine.SetSpawnCommandForTest("/bin/cat", nil) // deterministic echo process instead of a login shell
	api := daemon.NewAPI(engine)
	seq := testSockSeq.Add(1)
	socketPath := fmt.Sprintf("/tmp/aht%d_%d.sock", os.Getpid(), seq)
	_ = os.Remove(socketPath)
	if err := api.Start(socketPath); err != nil {
		t.Fatalf("testAppWithAPI: api.Start: %v", err)
	}
	if _, err := api.StartRelay(); err != nil {
		t.Fatalf("testAppWithAPI: StartRelay: %v", err)
	}
	client := daemon.NewDaemonClient(socketPath)
	time.Sleep(10 * time.Millisecond)
	app := &App{
		ctx:    context.Background(),
		client: client,
	}
	t.Cleanup(func() {
		engine.Manager().Shutdown()
		api.Stop()
		_ = os.Remove(socketPath)
	})
	return app, api, engine
}

func TestListSessionsEmpty(t *testing.T) {
	app := testApp(t)
	sessions := app.ListSessions()
	if sessions == nil {
		t.Fatal("ListSessions on fresh App returned nil, want empty slice")
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestCreateSession(t *testing.T) {
	app := testApp(t)
	id, err := app.CreateSession("test-tab", "", 0, 0)
	if err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}
	if id == "" {
		t.Fatal("CreateSession returned empty ID")
	}

	sessions := app.ListSessions()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].ID != id {
		t.Errorf("session ID mismatch: got %q, want %q", sessions[0].ID, id)
	}
	if sessions[0].Name != "test-tab" {
		t.Errorf("session name mismatch: got %q, want %q", sessions[0].Name, "test-tab")
	}
	if sessions[0].Hostname == "" {
		t.Error("expected non-empty Hostname from ListSessions, got empty string")
	}
}

// TestListSessions_PropagatesWorkDir guards the App.ListSessions binding against
// silently dropping daemon SessionInfo fields (the UAT class of bug that once hit
// HomeDir/BrowseEnabled, both removed with the file browser). WorkDir drives Hub
// card grouping (GRID-02).
func TestListSessions_PropagatesWorkDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skipf("no usable home dir: %v", err)
	}

	app := testApp(t)
	id, err := app.CreateSession("home-session", home, 0, 0)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// The daemon's own view (source of truth) for the same session.
	rawSessions, err := app.client.ListSessions()
	if err != nil {
		t.Fatalf("daemon ListSessions: %v", err)
	}
	var raw *daemon.SessionInfo
	for i := range rawSessions {
		if rawSessions[i].ID == id {
			raw = &rawSessions[i]
			break
		}
	}
	if raw == nil {
		t.Fatalf("session %q not found in daemon ListSessions", id)
	}

	// The App binding's view (what the GUI consumes).
	sessions := app.ListSessions()
	var got *SessionInfo
	for i := range sessions {
		if sessions[i].ID == id {
			got = &sessions[i]
			break
		}
	}
	if got == nil {
		t.Fatalf("session %q not found in App.ListSessions", id)
	}

	if raw.WorkDir == "" {
		t.Fatal("daemon recorded empty WorkDir for a session created in the home dir")
	}
	if got.WorkDir != raw.WorkDir {
		t.Errorf("ListSessions dropped WorkDir: binding=%q daemon=%q", got.WorkDir, raw.WorkDir)
	}
}

func TestRenameSession(t *testing.T) {
	app := testApp(t)
	id, err := app.CreateSession("original", "", 0, 0)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if err := app.RenameSession(id, "renamed"); err != nil {
		t.Fatalf("RenameSession: %v", err)
	}

	sessions := app.ListSessions()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Name != "renamed" {
		t.Errorf("expected name %q, got %q", "renamed", sessions[0].Name)
	}
}

func TestKillSession(t *testing.T) {
	app := testApp(t)
	id, err := app.CreateSession("kill-me", "", 0, 0)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if err := app.KillSession(id); err != nil {
		t.Fatalf("KillSession: %v", err)
	}

	// Give the session a moment to be removed.
	time.Sleep(50 * time.Millisecond)

	sessions := app.ListSessions()
	for _, s := range sessions {
		if s.ID == id {
			t.Errorf("killed session %q still appears in ListSessions", id)
		}
	}
}

func TestGetRelayPort(t *testing.T) {
	app := testApp(t)
	port := app.GetRelayPort()
	if port <= 0 {
		t.Errorf("GetRelayPort returned %d, want port > 0", port)
	}
}

// WR-04: the checked bindings report a failed read as an error, so the
// frontend promise rejects; the plain ones keep resolving [] and 0.
func TestCheckedBindings_FailedReadIsAnError(t *testing.T) {
	down := &App{ctx: context.Background(), client: daemon.NewDaemonClient("/tmp/portty-no-such-daemon.sock")}
	for name, a := range map[string]*App{"unreachable daemon": down, "no client": {ctx: context.Background()}} {
		if s := a.ListSessions(); s == nil || len(s) != 0 {
			t.Errorf("%s: ListSessions = %v, want []", name, s)
		}
		if _, err := a.ListSessionsChecked(); err == nil {
			t.Errorf("%s: ListSessionsChecked returned no error", name)
		}
		if p := a.GetRelayPort(); p != 0 {
			t.Errorf("%s: GetRelayPort = %d, want 0", name, p)
		}
		if _, err := a.GetRelayPortChecked(); err == nil {
			t.Errorf("%s: GetRelayPortChecked returned no error", name)
		}
	}
	app := testApp(t)
	if s, err := app.ListSessionsChecked(); err != nil || s == nil || len(s) != 0 {
		t.Errorf("ListSessionsChecked on a live daemon = %v, %v; want [], nil", s, err)
	}
	if p, err := app.GetRelayPortChecked(); err != nil || p <= 0 || p != app.GetRelayPort() {
		t.Errorf("GetRelayPortChecked on a live daemon = %d, %v; want %d", p, err, app.GetRelayPort())
	}
}

// The share bindings flip the daemon's one share switch and never throw:
// failures come back as ShareResult.ErrorCode (Portty 04-02).
func TestSessionShareBindings(t *testing.T) {
	api, client := testShareSetup(t)
	app := &App{ctx: context.Background(), client: client}
	id, err := client.CreateSession("bind", "/tmp", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := app.GetSessionShare(id); got.Shared || got.ErrorCode != "" {
		t.Fatalf("GetSessionShare before share = %+v", got)
	}
	on := app.SetSessionShared(id, true)
	if !on.Shared || !strings.Contains(on.URL, "?cap=") || !on.FirstShare || on.ErrorCode != "" {
		t.Fatalf("SetSessionShared(true) = %+v", on)
	}
	if got := app.GetSessionShare(id); !got.Shared || got.URL != on.URL {
		t.Fatalf("GetSessionShare while shared = %+v; want %q", got, on.URL)
	}
	if off := app.SetSessionShared(id, false); off.Shared || off.ErrorCode != "" {
		t.Fatalf("SetSessionShared(false) = %+v", off)
	}

	api.SetShareSeamsForTest(func(context.Context) webserver.TailscaleHealth { return webserver.TailscaleHealth{} }, loopbackShareServerForTest, func(context.Context, string) error { return nil })
	if got := app.SetSessionShared(id, true); got.Shared || got.ErrorCode != daemon.ShareErrTailscaleOff || got.ErrorDetail == "" {
		t.Fatalf("SetSessionShared with Tailscale off = %+v; want errorCode %s", got, daemon.ShareErrTailscaleOff)
	}
}

func TestSessionShareBindings_NilClient(t *testing.T) {
	app := &App{}
	if got := app.SetSessionShared("x", true); got.ErrorCode != "start_failed" || got.ErrorDetail == "" {
		t.Errorf("share-on with no daemon = %+v; want start_failed", got)
	}
	if got := app.SetSessionShared("x", false); got.ErrorCode != "stop_failed" {
		t.Errorf("share-off with no daemon = %+v; want stop_failed", got)
	}
	if got := app.GetSessionShare("x"); got.Shared || got.ErrorCode == "" {
		t.Errorf("GetSessionShare with no daemon = %+v; want an errorCode", got)
	}
}

func TestGetTailscaleStatus(t *testing.T) {
	app := testApp(t)
	h := app.GetTailscaleStatus()
	// On machines without tailscaled, Connected will be false.
	// On machines with tailscaled, any combination is valid.
	// The key assertion is that the method doesn't panic and returns a struct.
	_ = h.Connected
	_ = h.HasCerts
	_ = h.IP
	_ = h.Domain
}

// TestCheckTailscaleHealth_EmitsFirstResultAndChanges: the first result is
// emitted even when it is the zero value (not connected), an unchanged result
// is not re-emitted, and every change (connect, disconnect) is emitted (NET-01).
func TestCheckTailscaleHealth_EmitsFirstResultAndChanges(t *testing.T) {
	var next webserver.TailscaleHealth
	var emitted []webserver.TailscaleHealth
	app := &App{
		tailscaleCheckFunc: func(context.Context) webserver.TailscaleHealth { return next },
		emitEventFunc: func(name string, data any) {
			if name != "tailscale:health" {
				t.Errorf("unexpected event %q", name)
			}
			emitted = append(emitted, data.(webserver.TailscaleHealth))
		},
	}
	connected := webserver.TailscaleHealth{Connected: true, HasCerts: true, IP: "100.64.0.1", Domain: "h.ts.net"}

	steps := []struct {
		name      string
		h         webserver.TailscaleHealth
		wantEmits int
	}{
		{"first not-connected result emits", webserver.TailscaleHealth{}, 1},
		{"identical result does not emit", webserver.TailscaleHealth{}, 1},
		{"connected emits", connected, 2},
		{"identical connected does not emit", connected, 2},
		{"not connected again emits", webserver.TailscaleHealth{}, 3},
	}
	for _, st := range steps {
		next = st.h
		app.checkTailscaleHealth(context.Background())
		if len(emitted) != st.wantEmits {
			t.Fatalf("%s: got %d emits, want %d", st.name, len(emitted), st.wantEmits)
		}
		if emitted[len(emitted)-1] != st.h {
			t.Fatalf("%s: last emit %+v, want %+v", st.name, emitted[len(emitted)-1], st.h)
		}
	}
}

// TestGetTailscaleStatus_SharesPollerState (CR-01): a transient not-connected
// result from GetTailscaleStatus (focus recheck) is recorded and emitted, so
// the next connected poll is a change and emits the recovery.
func TestGetTailscaleStatus_SharesPollerState(t *testing.T) {
	connected := webserver.TailscaleHealth{Connected: true, HasCerts: true, IP: "100.64.0.1", Domain: "h.ts.net"}
	next := connected
	var emitted []webserver.TailscaleHealth
	app := &App{
		tailscaleCheckFunc: func(context.Context) webserver.TailscaleHealth { return next },
		emitEventFunc: func(_ string, data any) {
			emitted = append(emitted, data.(webserver.TailscaleHealth))
		},
	}
	app.checkTailscaleHealth(context.Background()) // poller: connected

	next = webserver.TailscaleHealth{}
	if got := app.GetTailscaleStatus(); got.Connected {
		t.Fatalf("GetTailscaleStatus = %+v, want not connected", got)
	}
	next = connected
	app.checkTailscaleHealth(context.Background()) // poller: recovered

	want := []webserver.TailscaleHealth{connected, {}, connected}
	if len(emitted) != len(want) {
		t.Fatalf("emitted %+v, want %+v", emitted, want)
	}
	for i := range want {
		if emitted[i] != want[i] {
			t.Fatalf("emit %d = %+v, want %+v", i, emitted[i], want[i])
		}
	}
}

func TestHealthPollerStops(t *testing.T) {
	app := testApp(t)
	app.tailscaleCheckFunc = func(context.Context) webserver.TailscaleHealth { return webserver.TailscaleHealth{} }
	ctx, cancel := context.WithCancel(context.Background())
	app.startHealthPoller(ctx)
	// Cancel immediately -- the goroutine should exit without blocking.
	cancel()
	// Give the goroutine a moment to observe the cancellation.
	time.Sleep(100 * time.Millisecond)
	// If the goroutine leaked, the race detector (run with -race) will catch it
	// on test cleanup. No explicit assertion needed beyond "no hang, no panic".
}

// testAppNoDaemon returns an App with no daemon client — simulates a startup
// failure where EnsureDaemon returned an error before the client was created.
func testAppNoDaemon(t *testing.T) *App {
	t.Helper()
	return &App{ctx: context.Background()}
}

// --- Nil-client guard tests ---

// TestNilClientListSessions verifies ListSessions returns an empty slice (not nil,
// no panic) when the daemon client is nil.
func TestNilClientListSessions(t *testing.T) {
	app := testAppNoDaemon(t)
	sessions := app.ListSessions()
	if sessions == nil {
		t.Fatal("ListSessions with nil client returned nil, want empty slice")
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

// TestNilClientGetRelayPort verifies GetRelayPort returns 0 (no panic) when
// the daemon client is nil.
func TestNilClientGetRelayPort(t *testing.T) {
	app := testAppNoDaemon(t)
	port := app.GetRelayPort()
	if port != 0 {
		t.Errorf("expected GetRelayPort to return 0 with nil client, got %d", port)
	}
}

// TestNilClientCreateSession verifies CreateSession returns an error (no panic)
// when the daemon client is nil.
func TestNilClientCreateSession(t *testing.T) {
	app := testAppNoDaemon(t)
	_, err := app.CreateSession("tab", "", 0, 0)
	if err == nil {
		t.Error("expected CreateSession to return error with nil client")
	}
}

// TestNilClientKillSession verifies KillSession returns an error (no panic)
// when the daemon client is nil.
func TestNilClientKillSession(t *testing.T) {
	app := testAppNoDaemon(t)
	err := app.KillSession("any-id")
	if err == nil {
		t.Error("expected KillSession to return error with nil client")
	}
}

// TestConnectPeerSession (SHAR-06, SHAR-01): the binding returns the daemon's
// connect result, a typed errorCode for refusals and "unreachable" with no daemon;
// it never throws.
func TestConnectPeerSession(t *testing.T) {
	forbidden := false
	peer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if forbidden {
			w.Header().Set(tailnet.MarkerHeader, tailnet.MarkerValue)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		switch r.URL.Path {
		case "/api/sessions/meta":
			_, _ = w.Write([]byte(`[{"id":"aaaa1111","name":"build","status":"running"}]`))
		case "/api/peer/sessions/aaaa1111/cap":
			_, _ = w.Write([]byte(`{"cap":"TOKEN"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer peer.Close()
	api, client := testShareSetup(t)
	api.SetPeerClientForTest(peer.Client())
	api.SetPeerSeamsForTest(
		func(context.Context) ([]tailnet.Peer, error) {
			return []tailnet.Peer{{Hostname: "Office", DNSName: "office-air.tail1.ts.net.", Online: true}}, nil
		},
		func(context.Context) []tailnet.Peer { return nil },
		func(tailnet.Peer) string { return peer.URL },
	)
	app := &App{ctx: context.Background(), client: client}

	got := app.ConnectPeerSession("office-air", "build")
	if got.ErrorCode != "" || got.SessionID != "aaaa1111" || got.Name != "build" || got.Host != "office-air" || !strings.HasSuffix(got.URL, "/sessions/aaaa1111?cap=TOKEN") {
		t.Fatalf("ConnectPeerSession = %+v", got)
	}
	forbidden = true
	if got := app.ConnectPeerSession("office-air", "build"); got.ErrorCode != daemon.PeerErrForbidden || got.URL != "" {
		t.Fatalf("other account: %+v; want forbidden", got)
	}
	if got := app.ConnectPeerSession("garage", ""); got.ErrorCode != daemon.PeerErrUnknownHost {
		t.Fatalf("unknown host: %+v", got)
	}
	if got := (&App{}).ConnectPeerSession("office-air", "build"); got.ErrorCode != daemon.PeerErrUnreachable {
		t.Fatalf("no daemon: %+v; want unreachable", got)
	}
}

// --- RetryDaemon tests ---

// TestRetryDaemonFail verifies RetryDaemon returns a non-nil error, leaves
// a.client nil, and sets a.daemonErr when the daemon cannot start.
// We force failure by redirecting HOME to a read-only path so the socket
// directory cannot be created and EnsureDaemon times out.
func TestRetryDaemonFail(t *testing.T) {
	// Override HOME so DefaultSocketPath resolves to a dir that can't be created.
	t.Setenv("HOME", "/nonexistent-test-dir-that-cannot-exist")
	t.Setenv("XDG_CONFIG_HOME", "/nonexistent-test-dir-that-cannot-exist")

	app := testAppNoDaemon(t)
	err := app.RetryDaemon()
	if err == nil {
		t.Error("expected RetryDaemon to return error when daemon cannot start")
	}
	if app.client != nil {
		t.Error("expected app.client to remain nil after failed RetryDaemon")
	}
	if app.daemonErr.Load() == nil {
		t.Error("expected app.daemonErr to be set after failed RetryDaemon")
	}
}

// TestRetryDaemonSuccess_KeepsClientAndPollers (WR-05): a successful retry
// while the pollers run must not replace a.client (a data race under -race)
// or start more pollers.
func TestRetryDaemonSuccess_KeepsClientAndPollers(t *testing.T) {
	home, err := os.MkdirTemp("/tmp", "rd") // short: the socket path must fit sun_path
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(home) })
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home)

	engine := daemon.NewSessionEngine()
	engine.ConfigDirForTest(t.TempDir())
	api := daemon.NewAPI(engine)
	socketPath := daemon.DefaultSocketPath()
	if err := api.Start(socketPath); err != nil {
		t.Fatalf("api.Start: %v", err)
	}
	if _, err := api.StartRelay(); err != nil {
		t.Fatalf("StartRelay: %v", err)
	}
	t.Cleanup(func() { api.Stop() })

	client := daemon.NewDaemonClient(socketPath)
	app := &App{client: client}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app.startExitWatcher(ctx) // reads a.client concurrently with the retry

	if err := client.SetAppearance("dark"); err != nil {
		t.Fatalf("client.SetAppearance(dark): %v", err)
	}
	got := recordAppearance(app)

	if err := app.RetryDaemon(); err != nil {
		t.Fatalf("RetryDaemon: %v", err)
	}
	if app.client != client {
		t.Fatal("RetryDaemon replaced a.client")
	}
	if err := app.daemonErr.Load(); err != nil {
		t.Fatalf("daemonErr = %v, want nil", *err)
	}
	// WR-03: startup skipped the stored Appearance while the daemon was down.
	if len(*got) != 1 || (*got)[0] != "dark" {
		t.Errorf("applied after retry: want [dark], got %v", *got)
	}
}

// -----------------------------------------------------------------------
// Wave 0 RED test for Phase 130 RB-01 — GetRemoteSessionsWithMeta Wails RPC.
//
// GetRemoteSessionsWithMeta does not exist yet. This test will fail to compile
// until plan 130-03 adds it to app.go (alongside adding Reachable bool to
// RemotePeerSessions). It encodes the contract:
//   - Returns []RemotePeerSessions with a Reachable field per peer
//   - An unreachable peer maps to Reachable=false with empty Sessions
//   - With nil client, returns empty slice (no panic)
// -----------------------------------------------------------------------

// TestGetRemoteSessionsWithMeta_ReachableField verifies that
// GetRemoteSessionsWithMeta returns []RemotePeerSessions where each element
// carries a Reachable bool. With a nil client (no daemon), it must return an
// empty non-nil slice — the same nil-guard contract as GetRemoteSessions.
//
// With a live daemon and no tailnet peers, the result is empty — but the
// function must exist and compile. The Reachable field assertion is exercised
// here by constructing a nil-client App and verifying the type carries the
// field (compile-time proof). The runtime contract (unreachable peer →
// Reachable=false) is validated by the tailnet-level tests in
// internal/tailnet/tailnet_test.go.
func TestGetRemoteSessionsWithMeta_ReachableField(t *testing.T) {
	// Case 1: nil client — must return empty non-nil slice, no panic.
	app := testAppNoDaemon(t)
	peers := app.GetRemoteSessionsWithMeta()
	if peers == nil {
		t.Fatal("GetRemoteSessionsWithMeta with nil client returned nil, want empty slice")
	}
	if len(peers) != 0 {
		t.Errorf("expected 0 peers with nil client, got %d", len(peers))
	}

	// Case 2: verify RemotePeerSessions carries a Reachable bool field.
	// This is a compile-time assertion: if RemotePeerSessions does not have a
	// Reachable field, this assignment fails to compile.
	var r RemotePeerSessions
	r.Reachable = true
	r.Reachable = false
	_ = r.Reachable

	// Case 3: live daemon, no tailnet peers — result is empty but non-nil.
	appLive := testApp(t)
	livePeers := appLive.GetRemoteSessionsWithMeta()
	if livePeers == nil {
		t.Fatal("GetRemoteSessionsWithMeta with live daemon returned nil, want empty slice")
	}
	// Each peer in the result must have a Reachable field (struct-level assertion).
	for _, p := range livePeers {
		// Accessing p.Reachable compiles only if the field exists.
		_ = p.Reachable
	}
}

// TestOpenTailscale_OpensApp: the app is opened by its fixed name and the
// download page is not touched (NET-01, T-02-30).
func TestOpenTailscale_OpensApp(t *testing.T) {
	var gotName string
	urlCalled := false
	app := &App{
		openAppFunc: func(name string) error { gotName = name; return nil },
		openURLFunc: func(string) { urlCalled = true },
	}
	if err := app.OpenTailscale(); err != nil {
		t.Fatalf("OpenTailscale: %v", err)
	}
	if gotName != "Tailscale" {
		t.Errorf("app name = %q, want %q", gotName, "Tailscale")
	}
	if urlCalled {
		t.Error("download URL opened although the app opened")
	}
}

// TestOpenAppCmd_AbsolutePath: the default opener runs /usr/bin/open, never a
// PATH lookup (IN-05), with the fixed argv.
func TestOpenAppCmd_AbsolutePath(t *testing.T) {
	cmd := openAppCmd("Tailscale")
	if cmd.Path != "/usr/bin/open" {
		t.Errorf("cmd.Path = %q, want /usr/bin/open", cmd.Path)
	}
	if want := []string{"/usr/bin/open", "-a", "Tailscale"}; !reflect.DeepEqual(cmd.Args, want) {
		t.Errorf("cmd.Args = %v, want %v", cmd.Args, want)
	}
}

// TestOpenTailscale_FallsBackToDownloadURL: when the app cannot be opened,
// the fixed download URL opens instead and no error reaches the frontend.
func TestOpenTailscale_FallsBackToDownloadURL(t *testing.T) {
	var gotURL string
	app := &App{
		openAppFunc: func(string) error { return fmt.Errorf("Unable to find application named 'Tailscale'") },
		openURLFunc: func(url string) { gotURL = url },
	}
	if err := app.OpenTailscale(); err != nil {
		t.Fatalf("OpenTailscale: %v", err)
	}
	if gotURL != "https://tailscale.com/download/mac" {
		t.Errorf("fallback URL = %q, want the Tailscale download page", gotURL)
	}
}

// recordAppearance installs a recording applyAppearanceFunc seam (Portty 02-13).
func recordAppearance(a *App) *[]string {
	var got []string
	a.applyAppearanceFunc = func(mode string) { got = append(got, mode) }
	return &got
}

func TestSetAppearance_PersistsAndApplies(t *testing.T) {
	a := testApp(t)
	got := recordAppearance(a)
	if v := a.GetAppearance(); v != "system" {
		t.Errorf("default GetAppearance: want system, got %q", v)
	}
	if err := a.SetAppearance("dark"); err != nil {
		t.Fatalf("SetAppearance(dark): %v", err)
	}
	if len(*got) != 1 || (*got)[0] != "dark" {
		t.Errorf("applied: want [dark], got %v", *got)
	}
	if v := a.GetAppearance(); v != "dark" {
		t.Errorf("GetAppearance after set: want dark, got %q", v)
	}
}

func TestSetAppearance_RejectsInvalid(t *testing.T) {
	a := testApp(t)
	if err := a.SetAppearance("light"); err != nil {
		t.Fatalf("SetAppearance(light): %v", err)
	}
	got := recordAppearance(a)
	if err := a.SetAppearance("purple"); err == nil {
		t.Error("SetAppearance(purple): want error, got nil")
	}
	if len(*got) != 0 {
		t.Errorf("applied after rejection: want none, got %v", *got)
	}
	if v := a.GetAppearance(); v != "light" {
		t.Errorf("stored value after rejection: want light, got %q", v)
	}
}

func TestApplyStoredAppearance_AppliesSavedValue(t *testing.T) {
	a := testApp(t)
	if err := a.client.SetAppearance("light"); err != nil {
		t.Fatalf("client.SetAppearance(light): %v", err)
	}
	got := recordAppearance(a)
	a.applyStoredAppearance()
	if len(*got) != 1 || (*got)[0] != "light" {
		t.Errorf("applied: want [light], got %v", *got)
	}
}

// Quick 261116-ku9: Terminal Font bindings forward to the daemon, which owns validation.
func TestGetTerminalFont_NoDaemonDefault(t *testing.T) {
	a := &App{}
	if got := a.GetTerminalFont(); got != (daemon.TerminalFont{Family: "", Size: 13}) {
		t.Errorf("no daemon: want {\"\" 13}, got %+v", got)
	}
	if err := a.SetTerminalFont("Menlo", 14); err == nil {
		t.Error("SetTerminalFont with no daemon: want error, got nil")
	}
}

func TestSetTerminalFont_RoundTrip(t *testing.T) {
	a := testApp(t)
	if err := a.SetTerminalFont("Menlo", 14); err != nil {
		t.Fatalf("SetTerminalFont: %v", err)
	}
	if got := a.GetTerminalFont(); got != (daemon.TerminalFont{Family: "Menlo", Size: 14}) {
		t.Errorf("GetTerminalFont: got %+v", got)
	}
	if err := a.SetTerminalFont("Me;nlo", 14); err == nil {
		t.Error("SetTerminalFont(invalid family): want error, got nil")
	}
}

// WR-05: domReady waits for startup to close startupDone, but only up to
// startupWait, because it runs on the Wails message loop.
func TestDomReady_WaitsForStartupDone(t *testing.T) {
	a := testApp(t)
	if err := a.client.SetStartMinimized(true); err != nil { // skip WindowShow (no Wails ctx in tests)
		t.Fatalf("SetStartMinimized: %v", err)
	}
	a.startupDone = make(chan struct{})
	returned := make(chan struct{})
	go func() { a.domReady(context.Background()); close(returned) }()
	select {
	case <-returned:
		t.Fatal("domReady returned before startupDone closed")
	case <-time.After(100 * time.Millisecond):
	}
	close(a.startupDone)
	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		t.Fatal("domReady did not return after startupDone closed")
	}
}

// WR-07: past startupWait, domReady shows the window without another RPC. The
// client points at a socket that accepts and never answers, so any RPC would
// block for the 30 s client timeout.
func TestDomReady_TimeoutShowsWindowWithoutRPC(t *testing.T) {
	sock := fmt.Sprintf("/tmp/ahw%d.sock", os.Getpid())
	_ = os.Remove(sock)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close(); _ = os.Remove(sock) })
	var accepts atomic.Int32
	go func() {
		var held []net.Conn // hold every connection open, never answer
		defer func() {
			for _, c := range held {
				c.Close()
			}
		}()
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			accepts.Add(1)
			held = append(held, c)
		}
	}()

	shown := make(chan struct{}, 1)
	a := &App{
		client:         daemon.NewDaemonClient(sock),
		startupDone:    make(chan struct{}), // never closed: startup is wedged
		showWindowFunc: func() { shown <- struct{}{} },
	}
	prev := startupWait
	startupWait = 50 * time.Millisecond
	t.Cleanup(func() { startupWait = prev })

	returned := make(chan struct{})
	go func() { a.domReady(context.Background()); close(returned) }()
	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		t.Fatal("domReady blocked past startupWait")
	}
	select {
	case <-shown:
	default:
		t.Fatal("domReady did not show the window on timeout")
	}
	if n := accepts.Load(); n != 0 {
		t.Fatalf("domReady dialed the daemon %d times on timeout, want 0", n)
	}
}

// WR-07: startup that outlasts domReady's bound still gives the frontend init
// a working daemon. init awaits GetDaemonError, which waits for startupDone,
// before GetRelayPort, so init reads the final state (relay port > 0, no error).
func TestSlowStartup_InitWaitsForStartup(t *testing.T) {
	a := testApp(t)
	a.startupDone = make(chan struct{})
	a.showWindowFunc = func() {}
	a.applyAppearanceFunc = func(string) {}
	prev := startupWait
	startupWait = 20 * time.Millisecond
	t.Cleanup(func() { startupWait = prev })

	release := make(chan struct{})
	a.ensureDaemonFunc = func(string) error { <-release; return nil }
	go a.startDaemon("unused")

	a.domReady(context.Background()) // times out: startup is still running

	type initResult struct {
		daemonErr string
		port      int
	}
	got := make(chan initResult, 1)
	go func() { // the frontend init sequence
		e := a.GetDaemonError()
		got <- initResult{e, a.GetRelayPort()}
	}()
	select {
	case r := <-got:
		t.Fatalf("init returned before startup finished: %+v", r)
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	select {
	case r := <-got:
		if r.daemonErr != "" || r.port <= 0 {
			t.Fatalf("init after slow startup: daemonErr=%q port=%d, want \"\" and > 0", r.daemonErr, r.port)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("init did not return after startup finished")
	}
	if ss := a.ListSessions(); ss == nil {
		t.Fatal("ListSessions returned nil after startup")
	}
}

// WR-08: ListSessions never waits for startup. The Quit confirm calls it, so a
// gate there left Quit doing nothing visible during a slow startup.
func TestListSessions_DoesNotWaitForStartup(t *testing.T) {
	a := testApp(t)
	a.startupDone = make(chan struct{}) // never closed: startup still running
	got := make(chan []SessionInfo, 1)
	go func() { got <- a.ListSessions() }()
	select {
	case ss := <-got:
		if ss == nil {
			t.Fatal("ListSessions returned nil, want a slice")
		}
	case <-time.After(time.Second):
		t.Fatal("ListSessions waited on startupDone")
	}
}

// WR-07: a slow startup that fails surfaces its error to init.
func TestSlowStartup_FailureReachesInit(t *testing.T) {
	a := &App{startupDone: make(chan struct{}), emitEventFunc: func(string, any) {}}
	release := make(chan struct{})
	a.ensureDaemonFunc = func(string) error { <-release; return errors.New("spawn failed") }
	go a.startDaemon("unused")
	got := make(chan string, 1)
	go func() { got <- a.GetDaemonError() }()
	time.Sleep(50 * time.Millisecond)
	close(release)
	select {
	case e := <-got:
		if e != "spawn failed" {
			t.Fatalf("GetDaemonError = %q, want %q", e, "spawn failed")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("GetDaemonError did not return after startup failed")
	}
}

// WR-07: GetDaemonError reports a timeout instead of waiting forever.
func TestGetDaemonError_BoundedWhenStartupHangs(t *testing.T) {
	a := &App{startupDone: make(chan struct{})}
	prev := startupBindingWait
	startupBindingWait = 20 * time.Millisecond
	t.Cleanup(func() { startupBindingWait = prev })
	if e := a.GetDaemonError(); e != errStartupTimeout.Error() {
		t.Fatalf("GetDaemonError = %q, want %q", e, errStartupTimeout.Error())
	}
}

// WR-07: startDaemon closes startupDone on both the success and error paths,
// and emits daemon:error only on failure.
func TestStartDaemon_AlwaysClosesStartupDone(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{{"success", nil}, {"failure", errors.New("boom")}} {
		t.Run(tc.name, func(t *testing.T) {
			var emitted []string
			a := &App{
				startupDone:         make(chan struct{}),
				ensureDaemonFunc:    func(string) error { return tc.err },
				emitEventFunc:       func(name string, _ any) { emitted = append(emitted, name) },
				applyAppearanceFunc: func(string) {},
			}
			a.startDaemon("unused")
			select {
			case <-a.startupDone:
			default:
				t.Fatal("startupDone not closed")
			}
			if (tc.err != nil) != (len(emitted) == 1 && emitted[0] == "daemon:error") {
				t.Fatalf("emitted %v for err=%v", emitted, tc.err)
			}
			if (a.daemonErr.Load() != nil) != (tc.err != nil) {
				t.Fatalf("daemonErr set = %t, want %v", a.daemonErr.Load() != nil, tc.err)
			}
		})
	}
}

// DEBT-08: the GUI keeps a version-mismatched daemon with live sessions and
// asks once. "Not Now" keeps it for the rest of the launch with no new prompt.
func TestRestartConsent_NotNowKeepsDaemonWithoutAskingAgain(t *testing.T) {
	var events []string
	ensures := 0
	a := &App{
		emitEventFunc:    func(name string, data any) { events = append(events, fmt.Sprint(name, " ", data)) },
		ensureDaemonFunc: func(string) error { ensures++; return nil },
	}
	if a.restartConsent(question(2)) {
		t.Fatal("the GUI policy restarted without consent")
	}
	if got := a.GetDaemonRestartPrompt(); got != 2 {
		t.Fatalf("prompt = %d, want 2", got)
	}
	if strings.Join(events, "|") != "daemon:restart-needed 2" {
		t.Fatalf("events = %q", events)
	}
	if err := a.AnswerDaemonRestart(false); err != nil {
		t.Fatal(err)
	}
	if got := a.GetDaemonRestartPrompt(); got != 0 || ensures != 0 {
		t.Fatalf("after Not Now: prompt %d, ensures %d; want 0, 0", got, ensures)
	}
	if a.restartConsent(question(3)) || a.GetDaemonRestartPrompt() != 0 || len(events) != 1 {
		t.Fatalf("asked again after Not Now: prompt %d, events %q", a.GetDaemonRestartPrompt(), events)
	}
}

// DEBT-08: "Restart Now" runs EnsureDaemon with the restart approved, then
// tells the frontend to reload. A failed restart keeps the prompt; Quit All wins.
func TestAnswerDaemonRestart_RestartNow(t *testing.T) {
	var events []string
	allowed := false
	var ensureErr error
	a := &App{emitEventFunc: func(name string, data any) { events = append(events, name) }}
	a.ensureDaemonFunc = func(string) error { allowed = a.restartConsent(question(2)); return ensureErr }
	a.restartConsent(question(2))

	ensureErr = errors.New("spawn failed")
	if err := a.AnswerDaemonRestart(true); err == nil || !allowed {
		t.Fatalf("failed restart: err %v, allowed %v", err, allowed)
	}
	if a.GetDaemonRestartPrompt() != 2 {
		t.Fatal("a failed restart dropped the prompt")
	}

	ensureErr = nil
	if err := a.AnswerDaemonRestart(true); err != nil || !allowed {
		t.Fatalf("restart: err %v, allowed %v", err, allowed)
	}
	if a.GetDaemonRestartPrompt() != 0 || events[len(events)-1] != "daemon:recovered" {
		t.Fatalf("after Restart Now: prompt %d, events %q", a.GetDaemonRestartPrompt(), events)
	}
	if a.restartConsent(question(2)) {
		t.Fatal("the approval outlived its restart")
	}

	a.quitting.Store(true)
	allowed = false
	if err := a.AnswerDaemonRestart(true); err == nil || allowed {
		t.Fatalf("while quitting: err %v, ensure ran %v", err, allowed)
	}
}

// 08-06-REVIEW WR-01: "Restart Now" approves the session count the dialog
// showed. A restart that finds more sessions, or a count that turned unknown
// (or known after an unknown one), keeps the daemon and asks again.
func TestAnswerDaemonRestart_ApprovesShownCountOnly(t *testing.T) {
	for _, tc := range []struct {
		name        string
		shown, live int
		allowed     bool
	}{
		{"same count", 2, 2, true},
		{"fewer", 2, 1, true},
		{"grew", 2, 5, false},
		{"known count turned unknown", 2, -1, false},
		{"unknown, still unknown", -1, -1, true},
		{"unknown, now known", -1, 3, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var events []string
			a := &App{emitEventFunc: func(name string, data any) { events = append(events, fmt.Sprint(name, " ", data)) }}
			allowed := false
			a.ensureDaemonFunc = func(string) error { allowed = a.restartConsent(question(tc.live)); return nil }
			a.restartConsent(question(tc.shown))
			err := a.AnswerDaemonRestart(true)
			if allowed != tc.allowed {
				t.Fatalf("dialog showed %d, restart found %d: allowed %v, want %v", tc.shown, tc.live, allowed, tc.allowed)
			}
			if tc.allowed {
				if err != nil || a.GetDaemonRestartPrompt() != 0 {
					t.Fatalf("restart: err %v, prompt %d", err, a.GetDaemonRestartPrompt())
				}
				return
			}
			if !errors.Is(err, errRestartCountChanged) || a.GetDaemonRestartPrompt() != tc.live {
				t.Fatalf("re-ask: err %v, prompt %d; want %v, %d", err, a.GetDaemonRestartPrompt(), errRestartCountChanged, tc.live)
			}
			if last := events[len(events)-1]; last != fmt.Sprint("daemon:restart-needed ", tc.live) {
				t.Fatalf("last event %q, want the new count asked", last)
			}
			// Confirming the new count goes through.
			if err := a.AnswerDaemonRestart(true); err != nil || !allowed {
				t.Fatalf("second Restart Now: err %v, allowed %v", err, allowed)
			}
		})
	}
}

// 08-06-REVIEW IN-08: with no ensureDaemonFunc, App.ensure hands EnsureDaemon
// the GUI's restart consent: the policy raises the prompt and says no.
func TestEnsure_DefaultSeamPassesRestartConsent(t *testing.T) {
	policy := swapEnsureDaemon(t)
	a := &App{emitEventFunc: func(string, any) {}}
	if err := a.ensure()("sock"); err != nil {
		t.Fatal(err)
	}
	if *policy == nil || (*policy)(question(2)) || a.GetDaemonRestartPrompt() != 2 {
		t.Fatalf("the default seam's policy is not restartConsent (prompt %d)", a.GetDaemonRestartPrompt())
	}
}

// 08-06-REVIEW IN-01: Restart Now with no open question does nothing. It never
// overrides a Not Now after the fact, and never runs EnsureDaemon.
func TestAnswerDaemonRestart_NoPendingPrompt(t *testing.T) {
	events, ensures := 0, 0
	a := &App{emitEventFunc: func(string, any) { events++ }}
	a.ensureDaemonFunc = func(string) error { ensures++; a.restartConsent(question(3)); return nil }
	if err := a.AnswerDaemonRestart(true); !errors.Is(err, errNoRestartPending) || ensures != 0 {
		t.Fatalf("no prompt: err %v, ensures %d", err, ensures)
	}
	a.restartConsent(question(3))
	_ = a.AnswerDaemonRestart(false) // Not Now
	if err := a.AnswerDaemonRestart(true); !errors.Is(err, errNoRestartPending) || ensures != 0 || events != 1 || a.GetDaemonRestartPrompt() != 0 {
		t.Fatalf("after Not Now: err %v, ensures %d, events %d, prompt %d", err, ensures, events, a.GetDaemonRestartPrompt())
	}
}

// 08-06-REVIEW IN-02: a Restart Now that fails after the old daemon stopped
// (its sessions are gone) closes the question and hands over to the
// daemon:error banner and Retry. One that fails with the old daemon still up
// keeps the question (TestAnswerDaemonRestart_RestartNow).
func TestAnswerDaemonRestart_FailedAfterStopRaisesDaemonError(t *testing.T) {
	sock := respawnHome(t) // nothing listens: the old daemon is gone
	var events []string
	a := &App{client: daemon.NewDaemonClient(sock), emitEventFunc: func(name string, data any) { events = append(events, fmt.Sprint(name, " ", data)) }}
	a.ensureDaemonFunc = func(string) error { a.restartConsent(question(2)); return errors.New("daemon exited during startup") }
	a.restartConsent(question(2))
	err := a.AnswerDaemonRestart(true)
	want := "Restarting Portty's background service failed: daemon exited during startup"
	if err == nil || err.Error() != want {
		t.Fatalf("err %v, want %q", err, want)
	}
	if a.GetDaemonRestartPrompt() != 0 || a.GetDaemonError() != want {
		t.Fatalf("prompt %d, daemon error %q", a.GetDaemonRestartPrompt(), a.GetDaemonError())
	}
	if got := strings.Join(events[1:], "|"); got != "daemon:restart-needed 0|daemon:error "+want {
		t.Fatalf("events after the prompt = %q", got)
	}
}

// 08-06-REVIEW WR-03: a restart question is never left in a hidden or
// minimized window (start minimized, Quit GUI Only): raising it shows the
// window and the Dock icon. No prompt (Not Now, or an approved count) shows nothing.
func TestRestartConsent_ShowsWindowWhenAsking(t *testing.T) {
	shown := 0
	a := &App{emitEventFunc: func(string, any) {}, showWindowFunc: func() { shown++ }}
	a.restartConsent(question(2))
	if shown != 1 {
		t.Fatalf("window shown %d times for the prompt, want 1", shown)
	}
	allowed := false
	a.ensureDaemonFunc = func(string) error { allowed = a.restartConsent(question(3)); return nil }
	if err := a.AnswerDaemonRestart(true); !errors.Is(err, errRestartCountChanged) || allowed || shown != 2 {
		t.Fatalf("re-ask: err %v, allowed %v, shown %d; want the window shown again", err, allowed, shown)
	}
	a.ensureDaemonFunc = func(string) error { allowed = a.restartConsent(question(3)); return nil }
	if err := a.AnswerDaemonRestart(true); err != nil || !allowed || shown != 2 {
		t.Fatalf("approved restart: err %v, allowed %v, shown %d", err, allowed, shown)
	}
	b := &App{emitEventFunc: func(string, any) {}, showWindowFunc: func() { shown++ }}
	b.restartConsent(question(2))
	_ = b.AnswerDaemonRestart(false)
	if b.restartConsent(question(2)); shown != 3 {
		t.Fatalf("shown %d; want no window after Not Now", shown)
	}
}

// 08-06-REVIEW WR-02: the CreateSession binding never runs beside a restart
// that has counted the daemon's sessions. It waits for startup, for an open
// restart question, and for an EnsureDaemon under daemonMu.
func TestCreateSession_WaitsOutRestarts(t *testing.T) {
	created := func(a *App) <-chan error {
		done := make(chan error, 1)
		go func() { _, err := a.CreateSession("w", "", 0, 0); done <- err }()
		return done
	}
	blocked := func(t *testing.T, done <-chan error, what string) {
		t.Helper()
		select {
		case err := <-done:
			t.Fatalf("CreateSession ran %s (err %v)", what, err)
		case <-time.After(150 * time.Millisecond):
		}
	}
	ran := func(t *testing.T, done <-chan error) {
		t.Helper()
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("CreateSession still waiting")
		}
	}

	a := testApp(t)
	a.emitEventFunc = func(string, any) {}
	a.startupDone = make(chan struct{})
	done := created(a)
	blocked(t, done, "before startup finished")
	close(a.startupDone)
	ran(t, done)

	a.restartConsent(question(2))
	done = created(a)
	blocked(t, done, "while the restart question was open")
	if err := a.AnswerDaemonRestart(false); err != nil {
		t.Fatal(err)
	}
	ran(t, done)

	a.daemonMu.Lock()
	done = created(a)
	blocked(t, done, "beside an EnsureDaemon holding daemonMu")
	a.daemonMu.Unlock()
	ran(t, done)
}

// UPGR-05 (08-06 IN-07): Quit All cancels a running EnsureDaemon instead of
// waiting behind it on daemonMu (up to ~30 s before).
func TestQuitAll_CancelsRunningEnsure(t *testing.T) {
	entered := make(chan struct{})
	sawCancel := make(chan error, 1)
	orig := ensureDaemon
	ensureDaemon = func(ctx context.Context, _ string, _ daemon.EnsureOptions) error {
		close(entered)
		<-ctx.Done()
		sawCancel <- ctx.Err()
		return ctx.Err()
	}
	t.Cleanup(func() { ensureDaemon = orig })
	var quits atomic.Int64
	a := &App{
		ctx:           context.Background(),
		client:        daemon.NewDaemonClient(fmt.Sprintf("/tmp/qa%d_%d.sock", os.Getpid(), time.Now().UnixNano()%100000)), // nothing listens
		emitEventFunc: func(string, any) {},
		quitFunc:      func() { quits.Add(1) },
	}
	go a.RetryDaemon() //nolint:errcheck // holds daemonMu while its ensure runs
	<-entered
	start := time.Now()
	a.QuitAll()
	if d := time.Since(start); d > 500*time.Millisecond {
		t.Fatalf("QuitAll took %v behind a running ensure, want within 500 ms", d)
	}
	select {
	case err := <-sawCancel:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("the ensure saw %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("the running ensure was never cancelled")
	}
	if quits.Load() != 1 || !a.quitting.Load() {
		t.Fatalf("quits %d, quitting %v; want 1, true", quits.Load(), a.quitting.Load())
	}
}

// hungDaemonSocket listens on a fresh socket, accepts every connection and
// never answers, like a SIGSTOPped daemon (V-05b). accepted counts connections.
func hungDaemonSocket(t *testing.T) (path string, accepted *atomic.Int64) {
	path = fmt.Sprintf("/tmp/hd%d_%d.sock", os.Getpid(), time.Now().UnixNano()%100000)
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	accepted = new(atomic.Int64)
	conns := make(chan net.Conn, 64)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			conns <- c // held open, never read or answered
			accepted.Add(1)
		}
	}()
	t.Cleanup(func() {
		ln.Close()
		os.Remove(path)
		for len(conns) > 0 {
			(<-conns).Close()
		}
	})
	return path, accepted
}

// V-05b: Quit All with a daemon that accepts connections and never answers
// returns within 5 s, whether daemonMu is held by a running ensure (cancelled;
// its in-flight call ends with the ctx) or by a hung CreateSession (bounded
// lock wait), and ShutdownDaemon gives up on its own.
func TestQuitAll_HungDaemon(t *testing.T) {
	waitAccepted := func(t *testing.T, accepted *atomic.Int64) {
		for deadline := time.Now().Add(2 * time.Second); accepted.Load() == 0; time.Sleep(10 * time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatal("nothing called the hung daemon")
			}
		}
	}
	quitWithin := func(t *testing.T, a *App) {
		start := time.Now()
		a.QuitAll()
		if d := time.Since(start); d > 5*time.Second {
			t.Fatalf("QuitAll took %v against a hung daemon, want within 5 s", d)
		}
	}
	t.Run("behind a running ensure", func(t *testing.T) {
		path, accepted := hungDaemonSocket(t)
		ensured := make(chan error, 1)
		orig := ensureDaemon
		ensureDaemon = func(ctx context.Context, _ string, o daemon.EnsureOptions) error {
			err := orig(ctx, path, o) // the real EnsureDaemon, against the hung daemon
			ensured <- err
			return err
		}
		t.Cleanup(func() { ensureDaemon = orig })
		var quits atomic.Int64
		a := &App{ctx: context.Background(), client: daemon.NewDaemonClient(path), emitEventFunc: func(string, any) {}, quitFunc: func() { quits.Add(1) }}
		go a.RetryDaemon() //nolint:errcheck // holds daemonMu while its ensure runs
		waitAccepted(t, accepted)
		quitWithin(t, a)
		select {
		case err := <-ensured:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("the ensure returned %v, want context.Canceled", err)
			}
		case <-time.After(time.Second):
			t.Fatal("the ensure was still blocked on the hung daemon after Quit All")
		}
		if quits.Load() != 1 {
			t.Fatalf("quits %d, want 1", quits.Load())
		}
	})
	t.Run("behind a hung CreateSession", func(t *testing.T) {
		path, accepted := hungDaemonSocket(t)
		var quits atomic.Int64
		a := &App{ctx: context.Background(), client: daemon.NewDaemonClient(path), emitEventFunc: func(string, any) {}, quitFunc: func() { quits.Add(1) }}
		go a.CreateSession("s", "", 0, 0) //nolint:errcheck // holds daemonMu on the hung daemon
		waitAccepted(t, accepted)
		quitWithin(t, a)
		if quits.Load() != 1 {
			t.Fatalf("quits %d, want 1", quits.Load())
		}
	})
}

// UPGR-03: a restart question that says the sessions couldn't be moved raises
// the prompt as before, and the dialog reads MoveFailed when it opens. Not
// Now clears both.
func TestRestartConsent_MoveFailed(t *testing.T) {
	var events []string
	a := &App{emitEventFunc: func(name string, data any) { events = append(events, fmt.Sprint(name, " ", data)) }}
	if a.restartConsent(daemon.RestartQuestion{DaemonVersion: "v1", Sessions: 2, MoveFailed: "the new version exited"}) {
		t.Fatal("restarted without consent")
	}
	if a.GetDaemonRestartPrompt() != 2 || !a.GetDaemonRestartMoveFailed() || strings.Join(events, "|") != "daemon:restart-needed 2" {
		t.Fatalf("prompt %d, moveFailed %v, events %q", a.GetDaemonRestartPrompt(), a.GetDaemonRestartMoveFailed(), events)
	}
	if err := a.AnswerDaemonRestart(false); err != nil {
		t.Fatal(err)
	}
	if a.GetDaemonRestartPrompt() != 0 || a.GetDaemonRestartMoveFailed() {
		t.Fatalf("after Not Now: prompt %d, moveFailed %v", a.GetDaemonRestartPrompt(), a.GetDaemonRestartMoveFailed())
	}
	b := &App{emitEventFunc: func(string, any) {}}
	b.restartConsent(question(2))
	if b.GetDaemonRestartMoveFailed() {
		t.Fatal("a plain question reads as couldn't-move")
	}
}

// question is a DEBT-08 restart question from daemon v1 with n sessions.
func question(n int) daemon.RestartQuestion {
	return daemon.RestartQuestion{DaemonVersion: "v1", Sessions: n}
}

// UPGR-05: SIGTERM quits the GUI only. Wails' own handler showed the quit
// modal (or nothing, window hidden) and swallowed later SIGTERMs (G-01).
func TestOnTerminate_QuitsGUIOnly(t *testing.T) {
	path := fmt.Sprintf("/tmp/ot%d_%d.sock", os.Getpid(), time.Now().UnixNano()%100000)
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close(); os.Remove(path) })
	var shutdowns atomic.Int64
	go http.Serve(ln, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { //nolint:errcheck
		if r.URL.Path == "/shutdown" {
			shutdowns.Add(1)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	quits := 0
	a := &App{client: daemon.NewDaemonClient(path), quitFunc: func() { quits++ }}
	a.onTerminate()
	if quits != 1 || !a.quitting.Load() || shutdowns.Load() != 0 {
		t.Fatalf("quits %d, quitting %v, /shutdown sent %d; want 1, true, 0", quits, a.quitting.Load(), shutdowns.Load())
	}
	if a.beforeClose(context.Background()) {
		t.Fatal("beforeClose blocked the quit after SIGTERM")
	}
}

// A macOS logout, restart or shutdown quits the GUI without the quit dialog
// (Wails' applicationShouldTerminate cancelled it), and leaves the daemon alone.
func TestSystemPowerOff_AllowsQuit(t *testing.T) {
	var events []string
	a := &App{emitEventFunc: func(name string, _ any) { events = append(events, name) }}
	a.onSystemPowerOff()
	if !a.quitting.Load() || a.beforeClose(context.Background()) || len(events) != 0 {
		t.Fatalf("quitting %v, events %v; want true and a quit allowed with no dialog", a.quitting.Load(), events)
	}
}

// UPGR-04: after an ensure, a daemon below MinDaemonAPI raises the Update
// needed banner; with the default minimum nothing is raised.
func TestDaemonUpdateNeeded(t *testing.T) {
	origEnsure, origHealth, origMin := ensureDaemon, daemonHealthOf, daemon.MinDaemonAPI
	t.Cleanup(func() { ensureDaemon, daemonHealthOf, daemon.MinDaemonAPI = origEnsure, origHealth, origMin })
	ensureDaemon = func(context.Context, string, daemon.EnsureOptions) error { return nil }
	daemonHealthOf = func(string) (daemon.HealthResponse, error) {
		return daemon.HealthResponse{Status: "ok", Version: "v1.2.0", API: 2}, nil
	}
	var events []string
	emit := func(name string, data any) { events = append(events, fmt.Sprint(name, " ", data)) }

	daemon.MinDaemonAPI = 3
	a := &App{emitEventFunc: emit}
	if err := a.ensure()("sock"); err != nil {
		t.Fatal(err)
	}
	if got := a.GetDaemonUpdateNeeded(); got != "v1.2.0" || strings.Join(events, "|") != "daemon:update-needed v1.2.0" {
		t.Fatalf("update needed %q, events %q", got, events)
	}

	daemon.MinDaemonAPI = 1
	events = nil
	b := &App{emitEventFunc: emit}
	if err := b.ensure()("sock"); err != nil {
		t.Fatal(err)
	}
	if got := b.GetDaemonUpdateNeeded(); got != "" || len(events) != 0 {
		t.Fatalf("compatible daemon: update needed %q, events %q", got, events)
	}
}
