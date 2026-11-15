package daemon

// Phase 146-05 — Go test for the daemon read path: GET /api/remote-files/caps/{sessionID}/open-url,
// plus the cap deposit route POST /api/remote-files/caps (moved here from the
// removed remote_files_test.go; GUI remote session open depends on both).
//
// Behavior-level tests (not source-grep) per WR-02 plan requirement.
// Exercises handleRemoteSessionOpenURL via the HTTP API directly:
//   - Held cap: deposit then call → expect JSON {"url": "baseURL/sessions/sessionID?cap=TOKEN"}
//   - No cap: absent sessionID → expect 404 + JSON {"error": "no cap registered for session"}

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// capTestAPI builds a minimal *API + httptest server over the daemon socket mux.
func capTestAPI(t *testing.T) (*API, *httptest.Server) {
	t.Helper()
	engine := NewSessionEngine()
	engine.configDir = t.TempDir()
	api := NewAPI(engine)
	srv := httptest.NewServer(api.mux)
	t.Cleanup(srv.Close)
	return api, srv
}

// TestRemoteSessionOpenURL_HeldCap verifies that when a cap has been deposited
// for a session, the open-url handler returns HTTP 200 with a JSON body
// {"url": "baseURL/sessions/sessionID?cap=TOKEN"}.
func TestRemoteSessionOpenURL_HeldCap(t *testing.T) {
	api, srv := capTestAPI(t)

	// Deposit a cap with a fixed https baseURL and token.
	// RemoteCapStore.Put validates https://; use a hardcoded peer URL to match
	// the handler's composition.
	if err := api.remoteCaps.Put("sess-1", "https://peer:8443", "TOK"); err != nil {
		t.Fatalf("Put: %v", err)
	}

	resp, err := http.Get(srv.URL + "/api/remote-files/caps/sess-1/open-url")
	if err != nil {
		t.Fatalf("GET open-url: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200; got %d; body=%s", resp.StatusCode, body)
	}

	var got map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	wantURL := "https://peer:8443/sessions/sess-1?cap=TOK"
	if got["url"] != wantURL {
		t.Fatalf("url mismatch:\n  got  %q\n  want %q", got["url"], wantURL)
	}
}

// TestRemoteSessionOpenURL_NoCap verifies that when no cap is deposited for a
// session, the open-url handler returns HTTP 404 with a JSON body containing
// "no cap registered for session".
func TestRemoteSessionOpenURL_NoCap(t *testing.T) {
	_, srv := capTestAPI(t)

	resp, err := http.Get(srv.URL + "/api/remote-files/caps/sess-x/open-url")
	if err != nil {
		t.Fatalf("GET open-url: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 404; got %d; body=%s", resp.StatusCode, body)
	}

	var got map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if got["error"] != "no cap registered for session" {
		t.Fatalf("error mismatch: got %q; want %q", got["error"], "no cap registered for session")
	}
}

// ---- POST /api/remote-files/caps -----------------------------------------

func TestRegisterRemoteCap_HappyPath(t *testing.T) {
	api, srv := capTestAPI(t)

	body := bytes.NewBufferString(`{"sessionId":"sid-1","baseUrl":"https://peer.example","capToken":"tok-abc"}`)
	resp, err := http.Post(srv.URL+"/api/remote-files/caps", "application/json", body)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d; body=%s", resp.StatusCode, b)
	}
	var out map[string]bool
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if !out["ok"] {
		t.Fatalf("expected {ok:true}; got %v", out)
	}
	base, tok, ok := api.remoteCaps.Get("sid-1")
	if !ok || base != "https://peer.example" || tok != "tok-abc" {
		t.Fatalf("cap not stored: (%q, %q, %v)", base, tok, ok)
	}
}

func TestRegisterRemoteCap_MalformedBody(t *testing.T) {
	_, srv := capTestAPI(t)

	resp, err := http.Post(srv.URL+"/api/remote-files/caps", "application/json", bytes.NewBufferString("{not json"))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 on malformed body; got %d", resp.StatusCode)
	}
}

func TestRegisterRemoteCap_EmptyFieldsRejected(t *testing.T) {
	_, srv := capTestAPI(t)

	cases := []string{
		`{"sessionId":"","baseUrl":"https://p","capToken":"t"}`,
		`{"sessionId":"s","baseUrl":"","capToken":"t"}`,
		`{"sessionId":"s","baseUrl":"https://p","capToken":""}`,
	}
	for _, body := range cases {
		resp, err := http.Post(srv.URL+"/api/remote-files/caps", "application/json", bytes.NewBufferString(body))
		if err != nil {
			t.Fatalf("POST: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("body=%s expected 400; got %d", body, resp.StatusCode)
		}
	}
}

// TestRegisterRemoteCap_IntegrationWithDaemonClient confirms
// DaemonClient.RegisterRemoteCap (client_remote_files.go) posts to the route.
func TestRegisterRemoteCap_IntegrationWithDaemonClient(t *testing.T) {
	api, srv := capTestAPI(t)

	client := &DaemonClient{http: &http.Client{}, base: srv.URL}
	if err := client.RegisterRemoteCap("sid-integration", "https://peer.example", "tok-int"); err != nil {
		t.Fatalf("RegisterRemoteCap: %v", err)
	}
	base, tok, ok := api.remoteCaps.Get("sid-integration")
	if !ok || base != "https://peer.example" || tok != "tok-int" {
		t.Fatalf("cap not stored via DaemonClient: (%q, %q, %v)", base, tok, ok)
	}
}
