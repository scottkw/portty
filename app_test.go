package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"reflect"
	goruntime "runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/scottkw/agenthub/internal/daemon"
	"github.com/scottkw/agenthub/internal/webserver"
	"tailscale.com/ipn"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/tailcfg"
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

// testAppWithDirectWebServer creates an App + daemon API pair and returns a
// setup function that injects a TLS web server directly into the daemon API
// (bypassing the Tailscale prerequisite). Used for tests that need a running
// web server without a real Tailscale connection.
func testAppWithDirectWebServer(t *testing.T, tlsCfg *tls.Config) (*App, func(sessionID string) error) {
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
		t.Fatalf("testAppWithDirectWebServer: api.Start: %v", err)
	}
	if _, err := api.StartRelay(); err != nil {
		t.Fatalf("testAppWithDirectWebServer: StartRelay: %v", err)
	}

	client := daemon.NewDaemonClient(socketPath)
	time.Sleep(10 * time.Millisecond)

	app := &App{
		ctx:    context.Background(),
		client: client,
	}

	var wsRef *webserver.WebServer

	setup := func(sessionID string) error {
		ws, err := webserver.NewWebServer(webserver.Config{
			BindIP:    "127.0.0.1",
			Port:      0,
			FQDN:      "localhost",
			TLSConfig: tlsCfg,
		}, engine.Manager())
		if err != nil {
			return fmt.Errorf("NewWebServer: %w", err)
		}
		ws.SetSessionResolver(func(id string) (string, string, string, bool) {
			return "", "", "", false
		})
		if err := ws.Start(); err != nil {
			return fmt.Errorf("ws.Start: %w", err)
		}
		ws.EnableSession(sessionID)
		wsRef = ws
		// Inject the running webserver directly into the daemon API.
		api.SetWebServerForTest(ws)
		return nil
	}

	t.Cleanup(func() {
		if wsRef != nil {
			_ = wsRef.Stop()
		}
		engine.Manager().Shutdown()
		api.Stop()
		_ = os.Remove(socketPath)
	})
	return app, setup
}

// testAppWithAPI creates an App + daemon API pair (like testApp) and returns
// the daemon API and SessionEngine so callers can inject test doubles
// (e.g. SetWebServerForTest). Phase 165 / T-165-15.
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

// appTestFakeFunnelClient is a minimal webserver.FunnelClientForTest double for
// app_test.go — mirrors daemonFakeFunnelClient in internal/daemon/funnel_test.go.
// It stores the last SetServeConfig value so EnableFunnel / DisableFunnel can do
// the full ETag read-modify-write without a live tailscaled daemon. Thread-safe.
type appTestFakeFunnelClient struct {
	mu           sync.Mutex
	storedConfig *ipn.ServeConfig
}

func (f *appTestFakeFunnelClient) GetServeConfig(_ context.Context) (*ipn.ServeConfig, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.storedConfig, nil
}

func (f *appTestFakeFunnelClient) SetServeConfig(_ context.Context, cfg *ipn.ServeConfig) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.storedConfig = cfg
	return nil
}

func (f *appTestFakeFunnelClient) StatusWithoutPeers(_ context.Context) (*ipnstate.Status, error) {
	return &ipnstate.Status{
		Self: &ipnstate.PeerStatus{
			DNSName: "apptest.ts.net.",
			CapMap: tailcfg.NodeCapMap{
				tailcfg.CapabilityHTTPS: nil,
				tailcfg.NodeAttrFunnel:  nil,
				tailcfg.NodeCapability("https://tailscale.com/cap/funnel-ports?ports=443,8443,10000"): nil,
			},
		},
	}, nil
}

// Compile-time assertion: appTestFakeFunnelClient implements the exported seam.
var _ webserver.FunnelClientForTest = (*appTestFakeFunnelClient)(nil)

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

// TestApp_SetSessionFunnel_NilClient asserts that App.SetSessionFunnel returns
// the "daemon not connected" error when a.client is nil (mirrors ToggleWebServing
// nil-guard). Phase 165 / T-165-14.
func TestApp_SetSessionFunnel_NilClient(t *testing.T) {
	app := &App{} // no client wired
	err := app.SetSessionFunnel("sess-1", true, 3600)
	if err == nil {
		t.Fatal("SetSessionFunnel with nil client: expected error, got nil")
	}
	if err.Error() != "daemon not connected" {
		t.Errorf("SetSessionFunnel nil-client error = %q; want %q", err.Error(), "daemon not connected")
	}
}

// TestListSessions_PropagatesFunnelActive is the regression guard for
// Phase 165 / T-165-15: App.ListSessions must propagate SessionInfo.FunnelActive
// from the daemon source of truth so the frontend can poll funnel state.
// false must stay false (NOT dropped by omitempty); true must stay true.
//
// Uses a real daemon + WebServer with an injected fake funnelClient so
// App.SetSessionFunnel can succeed without a live tailscaled daemon.
func TestListSessions_PropagatesFunnelActive(t *testing.T) {
	app, api, engine := testAppWithAPI(t)

	// Wire a WebServer with a fake funnel client so EnableFunnel can succeed.
	tlsCfg, err := webserver.GenerateSelfSignedCert("127.0.0.1")
	if err != nil {
		t.Fatalf("GenerateSelfSignedCert: %v", err)
	}
	ws, err := webserver.NewWebServer(webserver.Config{
		BindIP:    "127.0.0.1",
		Port:      0,
		FQDN:      "apptest.ts.net",
		TLSConfig: tlsCfg,
	}, engine.Manager())
	if err != nil {
		t.Fatalf("NewWebServer: %v", err)
	}
	fake := &appTestFakeFunnelClient{}
	ws.SetFunnelClientForTest(fake)
	if err := ws.Start(); err != nil {
		t.Fatalf("ws.Start: %v", err)
	}
	t.Cleanup(func() { _ = ws.Stop() })
	api.SetWebServerForTest(ws)

	// Create a session to work with.
	id, err := app.CreateSession("funnel-propagation-session", "", 0, 0)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Initially FunnelActive must be false (not yet enabled) and must propagate
	// as false — not silently dropped as a zero value.
	sessions := app.ListSessions()
	var got *SessionInfo
	for i := range sessions {
		if sessions[i].ID == id {
			got = &sessions[i]
			break
		}
	}
	if got == nil {
		t.Fatalf("session %q not found in App.ListSessions (initial)", id)
	}
	if got.FunnelActive {
		t.Error("ListSessions: FunnelActive should be false before SetSessionFunnel (not dropped by omitempty)")
	}

	// Enable Funnel; FunnelActive should propagate as true.
	if err := app.SetSessionFunnel(id, true, 0); err != nil {
		t.Fatalf("SetSessionFunnel enable: %v", err)
	}

	sessions = app.ListSessions()
	got = nil
	for i := range sessions {
		if sessions[i].ID == id {
			got = &sessions[i]
			break
		}
	}
	if got == nil {
		t.Fatalf("session %q not found in App.ListSessions (after enable)", id)
	}
	if !got.FunnelActive {
		t.Error("App.ListSessions dropped FunnelActive=true after SetSessionFunnel(enable) (frontend poll would never see active funnel)")
	}
}

// TestSessionInfo_MirrorsDaemonFunnelFields is the regression guard demanded
// by Phase 177 / FNL-09 D-05: it fails if a future edit adds (or renames) a
// daemon.SessionInfo funnel-exposure field without mirroring it onto the
// app.go Wails bridge's SessionInfo. This exercises the REAL Go struct types
// via reflection — never the App.d.ts stub TEXT, which is exactly the blind
// spot funnelBinding.contract.test.tsx left open (it only asserts the
// hand-authored TypeScript stub, not the Go bridge that actually produces
// the JSON the frontend receives). Any daemon "Funnel*" field without a
// matching json-tagged field on main.SessionInfo turns this test red.
func TestSessionInfo_MirrorsDaemonFunnelFields(t *testing.T) {
	daemonType := reflect.TypeOf(daemon.SessionInfo{})
	appType := reflect.TypeOf(SessionInfo{})

	// Index app.SessionInfo's json tags (bare name, options stripped) by Go field name.
	appJSONTags := make(map[string]string, appType.NumField())
	for i := 0; i < appType.NumField(); i++ {
		f := appType.Field(i)
		tag := f.Tag.Get("json")
		if tag == "" {
			continue
		}
		appJSONTags[strings.Split(tag, ",")[0]] = f.Name
	}

	found := 0
	for i := 0; i < daemonType.NumField(); i++ {
		f := daemonType.Field(i)
		if !strings.HasPrefix(f.Name, "Funnel") {
			continue
		}
		found++
		tag := f.Tag.Get("json")
		if tag == "" {
			t.Errorf("daemon.SessionInfo.%s has no json tag", f.Name)
			continue
		}
		name := strings.Split(tag, ",")[0]
		if _, ok := appJSONTags[name]; !ok {
			t.Errorf("daemon.SessionInfo.%s (json tag %q) has no matching field on app.SessionInfo — "+
				"the app.go Wails bridge silently drops this funnel-exposure field from the native GUI (T-177-03)",
				f.Name, name)
		}
	}
	if found == 0 {
		t.Fatal("no daemon.SessionInfo fields matched the \"Funnel\" prefix — the enumeration matched nothing, " +
			"so this test would silently guard against nothing")
	}
}

// TestListSessions_PropagatesFunnelWriteActive is the regression guard for
// Phase 177 / FNL-09 (D-05): App.ListSessions must propagate
// SessionInfo.FunnelWriteActive from the daemon source of truth, and the
// SERIALIZED JSON must carry the "funnelWriteActive" key even when false
// (NOT omitempty). Mirrors TestListSessions_PropagatesFunnelActive
// (T-165-15) for the write-share sibling field. This is the load-bearing
// guard: App.js's ListSessions binding is a raw Call passthrough with no
// createFrom/field filtering, so the JSON asserted here directly is exactly
// what the frontend consumers (FULL ACCESS badge, tab icon, share-modal
// resync) receive over the wire.
func TestListSessions_PropagatesFunnelWriteActive(t *testing.T) {
	app, api, engine := testAppWithAPI(t)

	if err := api.BootstrapCapabilityState(); err != nil {
		t.Fatalf("BootstrapCapabilityState: %v", err)
	}

	tlsCfg, err := webserver.GenerateSelfSignedCert("127.0.0.1")
	if err != nil {
		t.Fatalf("GenerateSelfSignedCert: %v", err)
	}
	ws, err := webserver.NewWebServer(webserver.Config{
		BindIP:    "127.0.0.1",
		Port:      0,
		FQDN:      "apptest.ts.net",
		TLSConfig: tlsCfg,
	}, engine.Manager())
	if err != nil {
		t.Fatalf("NewWebServer: %v", err)
	}
	fake := &appTestFakeFunnelClient{}
	ws.SetFunnelClientForTest(fake)
	ws.SetSigningKey(api.CurrentSigningKey())
	ws.SetJoinCodes(api.JoinCodes())
	if err := ws.Start(); err != nil {
		t.Fatalf("ws.Start: %v", err)
	}
	t.Cleanup(func() { _ = ws.Stop() })
	api.SetWebServerForTest(ws)

	id, err := app.CreateSession("funnel-write-propagation-session", "", 0, 0)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Before minting a write cap, FunnelWriteActive must propagate as false —
	// and the serialized JSON must still carry the key (not dropped by omitempty).
	sessions := app.ListSessions()
	var got *SessionInfo
	for i := range sessions {
		if sessions[i].ID == id {
			got = &sessions[i]
			break
		}
	}
	if got == nil {
		t.Fatalf("session %q not found in App.ListSessions (initial)", id)
	}
	if got.FunnelWriteActive {
		t.Error("ListSessions: FunnelWriteActive should be false before SetSessionFunnelWrite (not dropped by omitempty)")
	}
	rawJSON, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("json.Marshal(initial SessionInfo): %v", err)
	}
	if !strings.Contains(string(rawJSON), `"funnelWriteActive":false`) {
		t.Errorf("serialized SessionInfo JSON missing \"funnelWriteActive\":false — got %s "+
			"(the frontend poll could never detect a true->false write-share teardown flip if this key were omitted)", rawJSON)
	}

	// Mint the gate-minted public write cap; FunnelWriteActive should
	// propagate as true, and the serialized JSON must contain the true value.
	if _, err := app.SetSessionFunnelWrite(id, 900); err != nil {
		t.Fatalf("SetSessionFunnelWrite: %v", err)
	}

	sessions = app.ListSessions()
	got = nil
	for i := range sessions {
		if sessions[i].ID == id {
			got = &sessions[i]
			break
		}
	}
	if got == nil {
		t.Fatalf("session %q not found in App.ListSessions (after mint)", id)
	}
	if !got.FunnelWriteActive {
		t.Fatal("App.ListSessions dropped FunnelWriteActive=true after SetSessionFunnelWrite " +
			"(the FULL ACCESS badge/tab-icon/share-modal-resync would never render — this is the original bug)")
	}
	rawJSON, err = json.Marshal(got)
	if err != nil {
		t.Fatalf("json.Marshal(post-mint SessionInfo): %v", err)
	}
	if !strings.Contains(string(rawJSON), `"funnelWriteActive":true`) {
		t.Errorf("serialized SessionInfo JSON missing \"funnelWriteActive\":true — got %s "+
			"(this is exactly the JSON App.js passes through raw to the FULL ACCESS consumers)", rawJSON)
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

func TestToggleWebServingErrorsWhenNotRunning(t *testing.T) {
	app := testApp(t)
	// web server is not running — ToggleWebServing should return an error.
	err := app.ToggleWebServing("some-session-id", true)
	if err == nil {
		t.Error("expected ToggleWebServing to return error when web server is not running")
	}
}

func TestStartWebServerLocalModeFallback(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	app := testApp(t)
	// StartWebServer uses Tailscale mode with certs, local mode when connected
	// without certs, and refuses when Tailscale is not connected (NET-01). In CI
	// any of these errors is expected and correct behavior.
	err := app.StartWebServer(0)
	if err == nil {
		// Tailscale is connected and server started — clean up.
		_ = app.StopWebServer()
		return
	}
	// Any error is acceptable — Tailscale not found or local mode password
	// missing are both valid outcomes in a test environment.
	t.Logf("StartWebServer error (expected in CI): %s", err.Error())
}

// NET-01 / Amendment 3: a start request while Tailscale is not connected is
// refused with the NET-01 sentence instead of falling back to LAN mode.
func TestStartWebServerRefusedWhileTailscaleOff(t *testing.T) {
	app := testApp(t)
	app.tailscaleCheckFunc = func(context.Context) webserver.TailscaleHealth { return webserver.TailscaleHealth{} }
	err := app.StartWebServer(0)
	if err == nil || err.Error() != "Tailscale isn't connected. Remote access and sharing are off until it is." {
		t.Fatalf("StartWebServer err = %v, want the NET-01 sentence", err)
	}
	if app.IsWebServerRunning() {
		t.Fatal("web server started while Tailscale was not connected")
	}
}

func TestIsWebServerRunning(t *testing.T) {
	app := testApp(t)
	if app.IsWebServerRunning() {
		t.Error("expected IsWebServerRunning to be false before StartWebServer")
	}
}

// selfSignedTLSForAppTest generates an in-memory self-signed TLS config
// for use in app-level tests where Tailscale is not available.
func selfSignedTLSForAppTest(t *testing.T) *tls.Config {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("selfSignedTLSForAppTest: generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:     []string{"localhost"},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("selfSignedTLSForAppTest: create cert: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("selfSignedTLSForAppTest: marshal key: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("selfSignedTLSForAppTest: key pair: %v", err)
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}}
}

func TestGetWebServerQRCode(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	tlsCfg := selfSignedTLSForAppTest(t)
	app, startWebServer := testAppWithDirectWebServer(t, tlsCfg)

	if err := startWebServer("test-session-id"); err != nil {
		t.Fatalf("startWebServer: %v", err)
	}

	b64, err := app.GetWebServerQRCode()
	if err != nil {
		t.Fatalf("GetWebServerQRCode: %v", err)
	}
	if b64 == "" {
		t.Fatal("GetWebServerQRCode returned empty string")
	}

	pngBytes, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	if len(pngBytes) < 4 {
		t.Fatalf("decoded PNG too short: %d bytes", len(pngBytes))
	}
	if pngBytes[0] != 0x89 || pngBytes[1] != 'P' || pngBytes[2] != 'N' || pngBytes[3] != 'G' {
		t.Errorf("expected PNG magic bytes, got %v", pngBytes[:4])
	}
}

func TestGetWebServerQRCode_NoServer(t *testing.T) {
	app := testApp(t)
	_, err := app.GetWebServerQRCode()
	if err == nil {
		t.Error("expected GetWebServerQRCode to return error when web server is not running")
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

// TestNilClientGetRemoteSessions verifies GetRemoteSessions returns an empty slice
// (not nil, no panic) when the daemon client is nil.
func TestNilClientGetRemoteSessions(t *testing.T) {
	app := testAppNoDaemon(t)
	peers := app.GetRemoteSessions()
	if peers == nil {
		t.Fatal("GetRemoteSessions with nil client returned nil, want empty slice")
	}
	if len(peers) != 0 {
		t.Errorf("expected 0 peers, got %d", len(peers))
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
	if app.daemonErr == nil {
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

	if err := app.RetryDaemon(); err != nil {
		t.Fatalf("RetryDaemon: %v", err)
	}
	if app.client != client {
		t.Fatal("RetryDaemon replaced a.client")
	}
	if app.daemonErr != nil {
		t.Fatalf("daemonErr = %v, want nil", app.daemonErr)
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
