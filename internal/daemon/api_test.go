package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/capability"
	"github.com/scottkw/portty/internal/relay"
	"github.com/scottkw/portty/internal/tailnet"
)

// testDaemon creates an API server with a fresh engine on a temp socket.
// Returns the API, a DaemonClient connected to it, and the socket path.
func testDaemon(t *testing.T) (*API, *DaemonClient, string) {
	t.Helper()
	engine := NewSessionEngine()
	// Isolate from real settings.json that NewSessionEngine may have loaded.
	// NewSessionEngine() unconditionally reads os.UserConfigDir()/portty/settings.json
	// at construction time (engine.go: loadSettingsFromDisk(cfgDir)). Any field
	// the real settings file populates leaks into the test unless explicitly
	// reset here. Phase 116 / TEST-04..06: every field that loadSettingsFromDisk
	// touches must appear in this reset block. Future engine fields require an
	// addition here OR a refactor that decouples NewSessionEngine from disk.
	engine.configDir = t.TempDir()
	engine.startMinimized = false
	engine.pluginSettings = defaultPluginSettings()
	engine.SetSpawnCommandForTest("/bin/cat", nil) // deterministic echo process instead of a login shell
	api := NewAPI(engine)
	// Use short socket path — macOS t.TempDir() paths exceed the 103-char limit.
	socketPath := shortSocketPath(t, "api.sock")
	if err := api.Start(socketPath); err != nil {
		t.Fatalf("api.Start: %v", err)
	}
	t.Cleanup(func() { api.Stop() })
	client := NewDaemonClient(socketPath)
	// Brief sleep to let server goroutine start accepting.
	time.Sleep(10 * time.Millisecond)
	return api, client, socketPath
}

// dialUnix returns an http.Transport dialer that dials the given Unix socket.
func dialUnix(socketPath string) func(context.Context, string, string) (net.Conn, error) {
	return func(_ context.Context, _, _ string) (net.Conn, error) {
		return net.DialTimeout("unix", socketPath, 2*time.Second)
	}
}

// rawGet performs GET http://daemon{path} via Unix socket and returns status code + body bytes.
func rawGet(t *testing.T, socketPath, path string) (int, []byte) {
	t.Helper()
	client := &http.Client{Transport: &http.Transport{DialContext: dialUnix(socketPath)}}
	resp, err := client.Get("http://daemon" + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, body
}

// rawPost performs POST http://daemon{path} with the given JSON body.
func rawPost(t *testing.T, socketPath, path, body string) (int, []byte) {
	t.Helper()
	client := &http.Client{Transport: &http.Transport{DialContext: dialUnix(socketPath)}}
	resp, err := client.Post("http://daemon"+path, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, respBody
}

// rawDelete performs DELETE http://daemon{path} via Unix socket.
func rawDelete(t *testing.T, socketPath, path string) (int, []byte) {
	t.Helper()
	client := &http.Client{Transport: &http.Transport{DialContext: dialUnix(socketPath)}}
	req, err := http.NewRequest(http.MethodDelete, "http://daemon"+path, nil)
	if err != nil {
		t.Fatalf("NewRequest DELETE %s: %v", path, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", path, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, respBody
}

// rawPatch performs PATCH http://daemon{path} with the given JSON body.
func rawPatch(t *testing.T, socketPath, path, body string) (int, []byte) {
	t.Helper()
	client := &http.Client{Transport: &http.Transport{DialContext: dialUnix(socketPath)}}
	req, err := http.NewRequest(http.MethodPatch, "http://daemon"+path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest PATCH %s: %v", path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("PATCH %s: %v", path, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, respBody
}

func TestAPIHealth(t *testing.T) {
	old := BuildVersion
	BuildVersion = "v2.1.2-test"
	t.Cleanup(func() { BuildVersion = old })

	_, _, socketPath := testDaemon(t)
	status, body := rawGet(t, socketPath, "/health")
	if status != 200 {
		t.Errorf("GET /health: want 200, got %d", status)
	}
	var h HealthResponse
	if err := json.Unmarshal(body, &h); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if h.Status != "ok" {
		t.Errorf("health status: want %q, got %q", "ok", h.Status)
	}
	if h.Version != "v2.1.2-test" {
		t.Errorf("health version: want %q, got %q", "v2.1.2-test", h.Version)
	}
}

func TestAPIListSessionsEmpty(t *testing.T) {
	_, _, socketPath := testDaemon(t)
	status, body := rawGet(t, socketPath, "/sessions")
	if status != 200 {
		t.Errorf("GET /sessions: want 200, got %d", status)
	}
	var sessions []SessionInfo
	if err := json.Unmarshal(body, &sessions); err != nil {
		t.Fatalf("decode sessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("want 0 sessions, got %d", len(sessions))
	}
}

func TestAPICreateSession(t *testing.T) {
	_, _, socketPath := testDaemon(t)
	status, body := rawPost(t, socketPath, "/sessions", `{"cli":"cat","name":"test","workDir":""}`)
	if status != 201 {
		t.Errorf("POST /sessions: want 201, got %d; body: %s", status, body)
	}
	var resp CreateResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if resp.ID == "" {
		t.Error("create response: empty ID")
	}
}

// TestHandleCreateSession_LegacyBodyIgnored (T-01-15): an old client posting
// cli/args still gets the login shell with exactly ["-l"]; caller-supplied
// args never reach the spawned process.
func TestHandleCreateSession_LegacyBodyIgnored(t *testing.T) {
	spy := &spyBackend{}
	engine := NewSessionEngine()
	engine.configDir = t.TempDir()
	engine.backend = spy
	api := NewAPI(engine)
	socketPath := shortSocketPath(t, "legacy.sock")
	if err := api.Start(socketPath); err != nil {
		t.Fatalf("api.Start: %v", err)
	}
	t.Cleanup(func() { api.Stop() })

	status, body := rawPost(t, socketPath, "/sessions", `{"cli":"claude","args":["--evil"],"name":"legacy","workDir":"/tmp"}`)
	if status != 201 {
		t.Fatalf("POST /sessions: want 201, got %d; body: %s", status, body)
	}
	if len(spy.lastReq.Args) != 1 || spy.lastReq.Args[0] != "-l" {
		t.Errorf("spawned Args = %v, want exactly [-l]", spy.lastReq.Args)
	}
	if spy.lastReq.CLI != "/bin/zsh" {
		t.Errorf("spawned CLI = %q, want /bin/zsh", spy.lastReq.CLI)
	}
}

func TestAPIGetSession(t *testing.T) {
	_, _, socketPath := testDaemon(t)
	_, createBody := rawPost(t, socketPath, "/sessions", `{"cli":"cat","name":"my-tab","workDir":""}`)
	var cr CreateResponse
	if err := json.Unmarshal(createBody, &cr); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	status, body := rawGet(t, socketPath, fmt.Sprintf("/sessions/%s", cr.ID))
	if status != 200 {
		t.Errorf("GET /sessions/%s: want 200, got %d", cr.ID, status)
	}
	var info SessionInfo
	if err := json.Unmarshal(body, &info); err != nil {
		t.Fatalf("decode session info: %v", err)
	}
	if info.ID != cr.ID {
		t.Errorf("session ID mismatch: got %q, want %q", info.ID, cr.ID)
	}
	if info.Name != "my-tab" {
		t.Errorf("session name: got %q, want %q", info.Name, "my-tab")
	}
}

func TestAPIGetSessionNotFound(t *testing.T) {
	_, _, socketPath := testDaemon(t)
	status, _ := rawGet(t, socketPath, "/sessions/nonexistent-id")
	if status != 404 {
		t.Errorf("GET /sessions/nonexistent-id: want 404, got %d", status)
	}
}

func TestAPIDeleteSession(t *testing.T) {
	_, _, socketPath := testDaemon(t)
	_, createBody := rawPost(t, socketPath, "/sessions", `{"cli":"cat","name":"del-me","workDir":""}`)
	var cr CreateResponse
	if err := json.Unmarshal(createBody, &cr); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	status, _ := rawDelete(t, socketPath, fmt.Sprintf("/sessions/%s", cr.ID))
	if status != 204 {
		t.Errorf("DELETE /sessions/%s: want 204, got %d", cr.ID, status)
	}

	// Subsequent GET should return 404.
	status, _ = rawGet(t, socketPath, fmt.Sprintf("/sessions/%s", cr.ID))
	if status != 404 {
		t.Errorf("GET after DELETE: want 404, got %d", status)
	}
}

func TestAPIRenameSession(t *testing.T) {
	_, _, socketPath := testDaemon(t)
	_, createBody := rawPost(t, socketPath, "/sessions", `{"cli":"cat","name":"old-name","workDir":""}`)
	var cr CreateResponse
	if err := json.Unmarshal(createBody, &cr); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	t.Cleanup(func() { rawDelete(t, socketPath, fmt.Sprintf("/sessions/%s", cr.ID)) })

	status, _ := rawPatch(t, socketPath, fmt.Sprintf("/sessions/%s/name", cr.ID), `{"name":"new-name"}`)
	if status != 204 {
		t.Errorf("PATCH /sessions/%s/name: want 204, got %d", cr.ID, status)
	}

	// Subsequent GET shows new name.
	_, body := rawGet(t, socketPath, fmt.Sprintf("/sessions/%s", cr.ID))
	var info SessionInfo
	if err := json.Unmarshal(body, &info); err != nil {
		t.Fatalf("decode session info after rename: %v", err)
	}
	if info.Name != "new-name" {
		t.Errorf("name after rename: got %q, want %q", info.Name, "new-name")
	}
}

func TestAPIRelayPort(t *testing.T) {
	api, _, socketPath := testDaemon(t)
	port, err := api.StartRelay()
	if err != nil {
		t.Fatalf("StartRelay: %v", err)
	}
	if port <= 0 {
		t.Fatalf("StartRelay returned invalid port: %d", port)
	}

	status, body := rawGet(t, socketPath, "/relay-port")
	if status != 200 {
		t.Errorf("GET /relay-port: want 200, got %d", status)
	}
	var resp RelayPortResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode relay port response: %v", err)
	}
	if resp.Port <= 0 {
		t.Errorf("relay port: want > 0, got %d", resp.Port)
	}
	if resp.Port != port {
		t.Errorf("relay port mismatch: got %d, want %d", resp.Port, port)
	}
}

func TestAPICreateSessionWithArgs(t *testing.T) {
	_, _, socketPath := testDaemon(t)
	status, body := rawPost(t, socketPath, "/sessions",
		`{"cli":"cat","name":"args-test","workDir":"","args":["--flag","value"]}`)
	if status != 201 {
		t.Errorf("POST /sessions with args: want 201, got %d; body: %s", status, body)
	}
	var resp CreateResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if resp.ID == "" {
		t.Error("create response with args: empty ID")
	}
}

func TestAPIListSessionsHostname(t *testing.T) {
	_, _, socketPath := testDaemon(t)
	rawPost(t, socketPath, "/sessions", `{"cli":"cat","name":"h-test","workDir":""}`)

	_, body := rawGet(t, socketPath, "/sessions")
	var sessions []SessionInfo
	if err := json.Unmarshal(body, &sessions); err != nil {
		t.Fatalf("decode sessions: %v", err)
	}
	if len(sessions) == 0 {
		t.Fatal("expected at least 1 session")
	}
	if sessions[0].Hostname == "" {
		t.Error("SessionInfo.Hostname is empty — want non-empty hostname")
	}
	// STRP-01: sessions no longer carry an agent/CLI identity on the wire.
	var raw []map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("decode raw sessions: %v", err)
	}
	if _, ok := raw[0]["cli"]; ok {
		t.Errorf("GET /sessions JSON still has a \"cli\" key: %s", body)
	}
}

func TestCreateSession_NoAutoEnable(t *testing.T) {
	_, _, socketPath := testDaemon(t)
	// No web server set — create a session.
	status, body := rawPost(t, socketPath, "/sessions", `{"cli":"cat","name":"no-web","workDir":""}`)
	if status != 201 {
		t.Fatalf("POST /sessions: want 201, got %d; body: %s", status, body)
	}
	var cr CreateResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	// GET /sessions should show WebEnabled=false.
	_, listBody := rawGet(t, socketPath, "/sessions")
	var sessions []SessionInfo
	if err := json.Unmarshal(listBody, &sessions); err != nil {
		t.Fatalf("decode sessions: %v", err)
	}
	for _, s := range sessions {
		if s.ID == cr.ID && s.WebEnabled {
			t.Errorf("session %s: want WebEnabled=false, got true", cr.ID)
		}
	}
}

func TestHandleTailnetPeers(t *testing.T) {
	t.Run("returns cached peers as JSON array", func(t *testing.T) {
		api, _, socketPath := testDaemon(t)

		testPeers := []tailnet.Peer{
			{Hostname: "mac1", DNSName: "mac1.tail.ts.net.", TailscaleIPs: []string{"100.64.0.1"}, OS: "macOS", Online: true},
			{Hostname: "linux2", DNSName: "linux2.tail.ts.net.", TailscaleIPs: []string{"100.64.0.2"}, OS: "linux", Online: true},
		}
		// Pre-populate cache to avoid needing a live Tailscale daemon.
		api.tailnetCache.set(testPeers)

		status, body := rawGet(t, socketPath, "/tailnet/peers")
		if status != 200 {
			t.Errorf("GET /tailnet/peers: want 200, got %d; body: %s", status, body)
		}

		var peers []tailnet.Peer
		if err := json.Unmarshal(body, &peers); err != nil {
			t.Fatalf("decode peers response: %v", err)
		}
		if len(peers) != 2 {
			t.Errorf("want 2 peers, got %d", len(peers))
		}
		if peers[0].Hostname != "mac1" {
			t.Errorf("first peer hostname: want %q, got %q", "mac1", peers[0].Hostname)
		}
	})

	t.Run("returns empty array when no Tailscale daemon available", func(t *testing.T) {
		api, _, socketPath := testDaemon(t)

		// Pre-set cache with empty slice to simulate unavailable Tailscale.
		api.tailnetCache.set([]tailnet.Peer{})

		status, body := rawGet(t, socketPath, "/tailnet/peers")
		if status != 200 {
			t.Errorf("GET /tailnet/peers empty: want 200, got %d; body: %s", status, body)
		}

		var peers []tailnet.Peer
		if err := json.Unmarshal(body, &peers); err != nil {
			t.Fatalf("decode empty peers response: %v", err)
		}
		if len(peers) != 0 {
			t.Errorf("want 0 peers, got %d", len(peers))
		}
	})

	t.Run("client ListTailnetPeers returns cached peers", func(t *testing.T) {
		api, client, _ := testDaemon(t)

		testPeers := []tailnet.Peer{
			{Hostname: "node1", DNSName: "node1.tail.ts.net.", TailscaleIPs: []string{"100.64.0.3"}, OS: "linux", Online: true},
		}
		api.tailnetCache.set(testPeers)

		peers, err := client.ListTailnetPeers()
		if err != nil {
			t.Fatalf("ListTailnetPeers: %v", err)
		}
		if len(peers) != 1 {
			t.Errorf("want 1 peer, got %d", len(peers))
		}
		if peers[0].Hostname != "node1" {
			t.Errorf("peer hostname: want %q, got %q", "node1", peers[0].Hostname)
		}
	})
}

// TestAPI_ListSessionsViewerCount verifies that ViewerCount in the session list
// API response reflects the hub's remote (web-origin) viewer count (MC-04;
// updated Phase 168 / FIX-04, #121 — a subscriber must have Origin=="web" to
// count, since local-origin subscribers no longer contribute to ViewerCount).
func TestAPI_ListSessionsViewerCount(t *testing.T) {
	api, _, socketPath := testDaemon(t)

	// Create a session via the API.
	status, body := rawPost(t, socketPath, "/sessions", `{"cli":"cat","name":"vc-test","workDir":""}`)
	if status != 201 {
		t.Fatalf("POST /sessions: want 201, got %d; body: %s", status, body)
	}
	var cr CreateResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	t.Cleanup(func() { rawDelete(t, socketPath, fmt.Sprintf("/sessions/%s", cr.ID)) })

	// Resolve the hub up front and read the ViewerCount via the API helper below.
	hub, ok := api.engine.Manager().Get(cr.ID)
	if !ok {
		t.Fatalf("hub for session %s not found in manager", cr.ID)
	}

	// viewerCount fetches the current ViewerCount for this session via the API.
	viewerCount := func() int {
		t.Helper()
		_, listBody := rawGet(t, socketPath, "/sessions")
		var sessions []SessionInfo
		if err := json.Unmarshal(listBody, &sessions); err != nil {
			t.Fatalf("decode sessions: %v", err)
		}
		for i := range sessions {
			if sessions[i].ID == cr.ID {
				return sessions[i].ViewerCount
			}
		}
		t.Fatalf("session %s not found in list", cr.ID)
		return 0
	}

	// waitViewerCount polls until ViewerCount reaches want (subscribe/unsubscribe
	// are observed asynchronously through the listing).
	waitViewerCount := func(want int) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		var got int
		for time.Now().Before(deadline) {
			if got = viewerCount(); got == want {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Errorf("ViewerCount: want %d, got %d (timed out)", want, got)
	}

	// Wait for the detector subscription to settle so the baseline is stable: the
	// count must hold steady for a short window before we trust it.
	baseline := viewerCount()
	stableSince := time.Now()
	settleDeadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(settleDeadline) {
		time.Sleep(20 * time.Millisecond)
		if c := viewerCount(); c != baseline {
			baseline = c
			stableSince = time.Now()
			continue
		}
		if time.Since(stableSince) >= 300*time.Millisecond {
			break
		}
	}

	// Subscribe a web-origin client to the session's hub; ViewerCount should
	// rise by 1. Origin must be "web" (Phase 168 / FIX-04) — local-origin
	// subscribers (the status detector, etc.) are excluded from ViewerCount.
	sub := &relay.Subscriber{
		Msgs:      make(chan []byte, 256),
		CloseSlow: func() {},
		Origin:    "web",
	}
	hub.Subscribe(sub)
	defer hub.Unsubscribe(sub)
	waitViewerCount(baseline + 1)

	// Unsubscribe; ViewerCount should return to the baseline.
	hub.Unsubscribe(sub)
	waitViewerCount(baseline)
}

// TestAPIGetStartMinimized verifies GET /settings/start-minimized returns 200
// with {"startMinimized": false} when no value has been set (TRAY-02).
func TestAPIGetStartMinimized(t *testing.T) {
	_, _, socketPath := testDaemon(t)
	status, body := rawGet(t, socketPath, "/settings/start-minimized")
	if status != 200 {
		t.Errorf("GET /settings/start-minimized: want 200, got %d; body: %s", status, body)
	}
	var resp map[string]bool
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode start-minimized response: %v", err)
	}
	if val, ok := resp["startMinimized"]; !ok {
		t.Error("response missing 'startMinimized' key")
	} else if val {
		t.Errorf("initial startMinimized: want false, got true")
	}
}

// TestAPISetStartMinimized verifies PATCH /settings/start-minimized returns 204
// and the subsequent GET reflects the updated value (TRAY-02 / TRAY-03).
func TestAPISetStartMinimized(t *testing.T) {
	_, _, socketPath := testDaemon(t)

	// PATCH to true — expect 204.
	patchStatus, patchBody := rawPatch(t, socketPath, "/settings/start-minimized", `{"startMinimized":true}`)
	if patchStatus != 204 {
		t.Errorf("PATCH /settings/start-minimized: want 204, got %d; body: %s", patchStatus, patchBody)
	}

	// GET — expect true.
	getStatus, getBody := rawGet(t, socketPath, "/settings/start-minimized")
	if getStatus != 200 {
		t.Errorf("GET /settings/start-minimized after PATCH: want 200, got %d", getStatus)
	}
	var resp map[string]bool
	if err := json.Unmarshal(getBody, &resp); err != nil {
		t.Fatalf("decode start-minimized response after PATCH: %v", err)
	}
	if !resp["startMinimized"] {
		t.Errorf("startMinimized after PATCH true: want true, got false")
	}
}

// TestAPISetStartMinimizedInvalidBody verifies PATCH /settings/start-minimized
// with invalid JSON returns 400 (TRAY-02 input validation).
func TestAPISetStartMinimizedInvalidBody(t *testing.T) {
	_, _, socketPath := testDaemon(t)
	status, _ := rawPatch(t, socketPath, "/settings/start-minimized", `not-json`)
	if status != 400 {
		t.Errorf("PATCH /settings/start-minimized (invalid body): want 400, got %d", status)
	}
}

// --- Phase 87 Plan 04 tests ---------------------------------------------

// TestStartup_LoadsOrGeneratesSigningKey: starting the API with an empty
// config directory generates capability.key on disk, and sets api.signingKey
// (and ws.signingKey) to non-nil.
func TestStartup_LoadsOrGeneratesSigningKey(t *testing.T) {
	// Use an isolated temp config directory so this test doesn't clobber
	// the real os.UserConfigDir()/portty/capability.key.
	tmpDir := t.TempDir()
	engine := NewSessionEngine()
	engine.configDir = tmpDir
	engine.startMinimized = false
	api := NewAPI(engine)

	// Bootstrap the capability state (this is the production startup call
	// that task 87-04-01 introduces on *API).
	if err := api.BootstrapCapabilityState(); err != nil {
		t.Fatalf("BootstrapCapabilityState: %v", err)
	}

	// capability.key must now exist in the config dir.
	keyPath := filepath.Join(tmpDir, "capability.key")
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("capability.key not found at %s: %v", keyPath, err)
	}
	if info.Size() != 32 {
		t.Errorf("capability.key size: got %d, want 32", info.Size())
	}

	// api.signingKey must be non-nil.
	api.signingKeyMu.RLock()
	key := api.signingKey
	api.signingKeyMu.RUnlock()
	if key == nil {
		t.Fatalf("api.signingKey is nil after BootstrapCapabilityState")
	}
	if len(key) != 32 {
		t.Errorf("signing key length: got %d, want 32", len(key))
	}

	// A subsequent call must reload the same key (not regenerate).
	api2 := NewAPI(engine)
	if err := api2.BootstrapCapabilityState(); err != nil {
		t.Fatalf("second BootstrapCapabilityState: %v", err)
	}
	api2.signingKeyMu.RLock()
	key2 := api2.signingKey
	api2.signingKeyMu.RUnlock()
	if key2 == nil {
		t.Fatalf("api2.signingKey is nil after reload")
	}
	for i := range key {
		if key[i] != key2[i] {
			t.Errorf("signing key mismatch after reload: keys differ at byte %d", i)
			break
		}
	}
}

// --- Task 2 (IPC handler) tests -----------------------------------------

// TestIPCHandlers_RegenerateSigningKey_SwapsKey: POST
// /capability/regenerate-key replaces the on-disk key AND the in-memory key
// (share re-minting is covered by TestShare_KeyRegenRemintsLinks).
func TestIPCHandlers_RegenerateSigningKey_SwapsKey(t *testing.T) {
	tmpDir := t.TempDir()
	engine := NewSessionEngine()
	engine.configDir = tmpDir
	api := NewAPI(engine)
	if err := api.BootstrapCapabilityState(); err != nil {
		t.Fatalf("BootstrapCapabilityState: %v", err)
	}
	api.signingKeyMu.RLock()
	initialKey := append([]byte(nil), api.signingKey...)
	api.signingKeyMu.RUnlock()

	// Read current file bytes.
	initialFile, err := os.ReadFile(filepath.Join(tmpDir, "capability.key"))
	if err != nil {
		t.Fatalf("read capability.key: %v", err)
	}

	// Start the API on a socket, then POST /capability/regenerate-key.
	socketPath := shortSocketPath(t, "regen.sock")
	if err := api.Start(socketPath); err != nil {
		t.Fatalf("api.Start: %v", err)
	}
	t.Cleanup(func() { api.Stop() })
	time.Sleep(10 * time.Millisecond)

	st, regenBody := rawPost(t, socketPath, "/capability/regenerate-key", ``)
	if st != http.StatusOK {
		t.Fatalf("POST /capability/regenerate-key: want 200, got %d; body: %s", st, regenBody)
	}

	// On-disk key bytes must have changed.
	newFile, err := os.ReadFile(filepath.Join(tmpDir, "capability.key"))
	if err != nil {
		t.Fatalf("read capability.key after regen: %v", err)
	}
	if sameBytes(initialFile, newFile) {
		t.Errorf("capability.key unchanged after regenerate")
	}

	// In-memory signing key must have changed.
	api.signingKeyMu.RLock()
	newKey := api.signingKey
	api.signingKeyMu.RUnlock()
	if sameBytes(initialKey, newKey) {
		t.Errorf("api.signingKey unchanged after regenerate")
	}
}

// --- shared helpers for Task 2 tests ------------------------------------

// sameBytes compares two byte slices for equality.
func sameBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// extractCapToken pulls the value of the ?cap= query parameter from a URL.
func extractCapToken(urlStr string) string {
	i := strings.Index(urlStr, "?cap=")
	if i < 0 {
		return ""
	}
	tok := urlStr[i+len("?cap="):]
	if amp := strings.Index(tok, "&"); amp >= 0 {
		tok = tok[:amp]
	}
	return tok
}

// --- Phase 100 Plan 04: shell-session lifecycle ---------------------------

// TestShellSessionLifecycle_StatusOnlyRunningOrStopped (SHELL-09 end-to-end).
// Drives a full create→list-poll→kill cycle for a bash session and asserts
// Status only takes values in {"running", "stopped"} — never
// "waiting"/"error"/"errored"/"idle". POSIX-only because cli="bash" requires
// a POSIX shell binary on PATH.
func TestShellSessionLifecycle_StatusOnlyRunningOrStopped(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not installed on PATH")
	}

	_, client, _ := testDaemon(t)

	id, err := client.CreateSession("lifecycle-test", "", 80, 24)
	if err != nil {
		t.Fatalf("client.CreateSession: %v", err)
	}
	if id == "" {
		t.Fatal("CreateSession returned empty ID")
	}

	// Forbidden statuses per SHELL-09: shell sessions must never surface
	// AI-CLI heuristic states.
	forbidden := map[string]bool{
		"waiting": true,
		"error":   true,
		"errored": true,
		"idle":    true,
	}

	// findStatus returns the SessionInfo for the test session, or (zero, false)
	// if it has been removed from the registry (e.g. after KillSession).
	findStatus := func(t *testing.T) (SessionInfo, bool) {
		t.Helper()
		sessions, err := client.ListSessions()
		if err != nil {
			t.Fatalf("client.ListSessions: %v", err)
		}
		for _, s := range sessions {
			if s.ID == id {
				return s, true
			}
		}
		return SessionInfo{}, false
	}

	// Brief wait for the session to register.
	time.Sleep(100 * time.Millisecond)

	// Sample 5 times over ~1s while the bash PTY is alive: Status must always
	// be "running" (never flicker to waiting/idle/error/errored).
	for i := 0; i < 5; i++ {
		info, ok := findStatus(t)
		if !ok {
			t.Fatalf("session %s missing during running-phase sample %d", id, i)
		}
		if forbidden[info.Status] {
			t.Errorf("sample %d: Status=%q is forbidden for shell sessions (SHELL-09); want \"running\" or \"stopped\"", i, info.Status)
		}
		if info.Status != "running" && info.Status != "stopped" {
			t.Errorf("sample %d: Status=%q not in {running,stopped}", i, info.Status)
		}
		if info.Status != "running" {
			// The bash PTY is alive — Status should be "running".
			t.Errorf("sample %d: expected Status=\"running\" while bash PTY alive, got %q", i, info.Status)
		}
		time.Sleep(200 * time.Millisecond)
	}

	// Kill the session and poll until either (a) the session is removed from
	// the registry, or (b) its State transitions to "stopped". Throughout the
	// poll, Status must remain in {running,stopped} and never appear in
	// `forbidden`.
	if err := client.KillSession(id); err != nil {
		t.Fatalf("client.KillSession: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	terminated := false
	for time.Now().Before(deadline) {
		info, ok := findStatus(t)
		if !ok {
			// KillSession removes the session from the registry — this is the
			// expected terminal state for an explicitly-killed shell session.
			terminated = true
			break
		}
		if forbidden[info.Status] {
			t.Errorf("post-kill sample: Status=%q is forbidden for shell sessions (SHELL-09)", info.Status)
		}
		if info.Status != "running" && info.Status != "stopped" {
			t.Errorf("post-kill sample: Status=%q not in {running,stopped}", info.Status)
		}
		if info.State == "stopped" {
			terminated = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !terminated {
		t.Errorf("session %s did not terminate (state=stopped or removed) within 2s of KillSession", id)
	}
}

// extractClaimsFromURL pulls the cap= value from a URL string and verifies
// it against key, returning the decoded Claims.
func extractClaimsFromURL(t *testing.T, urlStr string, key []byte) capability.Claims {
	t.Helper()
	tok := extractCapToken(urlStr)
	if tok == "" {
		t.Fatalf("no cap token in URL: %q", urlStr)
	}
	claims, err := capability.Verify(tok, key)
	if err != nil {
		t.Fatalf("capability.Verify: %v", err)
	}
	return claims
}

// TestBrowseRoute_Removed asserts the per-session browse toggle route is gone
// from the daemon socket mux (STRP-02): POST /sessions/{id}/browse is 404/405.
func TestBrowseRoute_Removed(t *testing.T) {
	_, _, sock := testDaemon(t)
	status, body := rawPost(t, sock, "/sessions/any-id/browse", `{"enabled":true}`)
	if status != http.StatusNotFound && status != http.StatusMethodNotAllowed {
		t.Fatalf("POST /sessions/{id}/browse = %d, body=%s; want 404 or 405", status, string(body))
	}
}

func TestHandleGetSessionTailLines_ClampN(t *testing.T) {
	_, _, sock := testDaemon(t)

	// Create a session with enough content to observe clamping.
	// We don't need a real session here — an unknown session returns [].
	// To verify clamping we create a session with content > 20 lines.
	engine := NewSessionEngine()
	pr, pw := io.Pipe()
	hub := engine.manager.Create("clamp-sess", pr, pw, nil)
	// Write 30 lines of content into the hub.
	for i := 0; i < 30; i++ {
		fmt.Fprintf(pw, "line%02d\n", i)
	}
	pw.Close()
	<-hub.Done()

	// Use a second API server wired to our engine (testDaemon creates its own engine).
	engine.configDir = t.TempDir()
	engine.pluginSettings = defaultPluginSettings()
	api2 := NewAPI(engine)
	sock2 := shortSocketPath(t, "clamp.sock")
	if err := api2.Start(sock2); err != nil {
		t.Fatalf("api2.Start: %v", err)
	}
	t.Cleanup(func() { api2.Stop() })
	time.Sleep(10 * time.Millisecond)
	_ = sock // first daemon unused here

	// Request n=1000 — should be clamped to 20.
	status, body := rawGet(t, sock2, "/sessions/clamp-sess/tail?n=1000")
	if status != http.StatusOK {
		t.Fatalf("GET tail: status %d, body %s", status, string(body))
	}
	var resp TailLinesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode tail response: %v; raw=%s", err, string(body))
	}
	if len(resp.Lines) > 20 {
		t.Errorf("clamp failed: got %d lines with n=1000, want at most 20", len(resp.Lines))
	}
}

// ========================================================================
// Phase 139 Plan 01 — TestHandleGetSessionStyledTailLines (CARD-05).
//
// RED until Plan 03 adds:
//   - GET /sessions/{id}/styled-tail handler in api.go
//   - StyledTailLinesResponse type in types.go
//
// Verifies: 200 status, JSON decodable into StyledTailLinesResponse with
// non-nil Lines field, and that n=999 (clamped) also returns 200.
// ========================================================================

// TestHandleGetSessionStyledTailLines: GET /sessions/{id}/styled-tail returns
// 200 + valid JSON { "lines": [...] } with non-nil Lines field.
// Also asserts that n=999 (over the 20-line clamp) returns 200 (no error).
// RED until Plan 03 adds handleGetSessionStyledTailLines to api.go.
func TestHandleGetSessionStyledTailLines(t *testing.T) {
	_, _, socketPath := testDaemon(t)

	// Create a session to use as the target.
	_, createBody := rawPost(t, socketPath, "/sessions", `{"cli":"cat","name":"styled-tail-tab","workDir":""}`)
	var cr CreateResponse
	if err := json.Unmarshal(createBody, &cr); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	t.Cleanup(func() { rawDelete(t, socketPath, fmt.Sprintf("/sessions/%s", cr.ID)) })

	// GET /sessions/{id}/styled-tail?n=5 → 200 + valid StyledTailLinesResponse.
	status, body := rawGet(t, socketPath, fmt.Sprintf("/sessions/%s/styled-tail?n=5", cr.ID))
	if status != 200 {
		t.Errorf("GET /sessions/%s/styled-tail: want 200, got %d; body: %s", cr.ID, status, string(body))
	}
	var stResp StyledTailLinesResponse
	if err := json.Unmarshal(body, &stResp); err != nil {
		t.Fatalf("decode StyledTailLinesResponse: %v; raw=%s", err, string(body))
	}
	// Lines must be non-nil (may be empty for a fresh session with no scrollback).
	if stResp.Lines == nil {
		t.Errorf("StyledTailLinesResponse.Lines must be non-nil (got nil)")
	}

	// Defense-in-depth: n=999 (above the 20-line clamp) must still return 200.
	status2, _ := rawGet(t, socketPath, fmt.Sprintf("/sessions/%s/styled-tail?n=999", cr.ID))
	if status2 != 200 {
		t.Errorf("GET /sessions/%s/styled-tail?n=999: want 200, got %d", cr.ID, status2)
	}
}
