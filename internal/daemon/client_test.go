package daemon

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientHealth(t *testing.T) {
	_, client, _ := testDaemon(t)
	if err := client.Health(); err != nil {
		t.Errorf("Health: unexpected error: %v", err)
	}
}

func TestClientListSessions(t *testing.T) {
	_, client, _ := testDaemon(t)
	sessions, err := client.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if sessions == nil {
		t.Fatal("ListSessions returned nil, want empty slice")
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestClientCreateSession(t *testing.T) {
	_, client, _ := testDaemon(t)
	id, err := client.CreateSession("test-tab", "", 0, 0)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if id == "" {
		t.Fatal("CreateSession returned empty ID")
	}
	t.Cleanup(func() { client.KillSession(id) })
}

func TestClientKillSession(t *testing.T) {
	_, client, _ := testDaemon(t)
	id, err := client.CreateSession("kill-me", "", 0, 0)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if err := client.KillSession(id); err != nil {
		t.Errorf("KillSession: %v", err)
	}
}

func TestClientRenameSession(t *testing.T) {
	_, client, _ := testDaemon(t)
	id, err := client.CreateSession("original", "", 0, 0)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	t.Cleanup(func() { client.KillSession(id) })

	if err := client.RenameSession(id, "renamed"); err != nil {
		t.Fatalf("RenameSession: %v", err)
	}

	sessions, err := client.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions after rename: %v", err)
	}
	var found SessionInfo
	for _, s := range sessions {
		if s.ID == id {
			found = s
			break
		}
	}
	if found.ID == "" {
		t.Fatal("session not found after rename")
	}
	if found.Name != "renamed" {
		t.Errorf("name after rename: got %q, want %q", found.Name, "renamed")
	}
}

func TestClientGetRelayPort(t *testing.T) {
	api, client, _ := testDaemon(t)
	port, err := api.StartRelay()
	if err != nil {
		t.Fatalf("StartRelay: %v", err)
	}
	if port <= 0 {
		t.Fatalf("StartRelay returned invalid port: %d", port)
	}

	got, err := client.GetRelayPort()
	if err != nil {
		t.Fatalf("GetRelayPort: %v", err)
	}
	if got <= 0 {
		t.Errorf("GetRelayPort: want > 0, got %d", got)
	}
	if got != port {
		t.Errorf("GetRelayPort: got %d, want %d", got, port)
	}
}

func TestClientWebServerStatus(t *testing.T) {
	_, client, _ := testDaemon(t)
	resp, err := client.GetWebServerStatus()
	if err != nil {
		t.Fatalf("GetWebServerStatus: %v", err)
	}
	if resp.Running {
		t.Errorf("GetWebServerStatus: want Running=false, got true")
	}
}

func TestShutdownDaemon(t *testing.T) {
	// Create a test HTTP server that records the shutdown call
	var shutdownCalled bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/shutdown" {
			shutdownCalled = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	// Create client pointing to test server (TCP, not Unix socket)
	client := &DaemonClient{
		http: srv.Client(),
		base: srv.URL,
	}
	err := client.ShutdownDaemon()
	if err != nil {
		t.Fatalf("ShutdownDaemon returned error: %v", err)
	}
	if !shutdownCalled {
		t.Error("expected /shutdown endpoint to be called")
	}
}

// TestClientFullLifecycle tests the full round-trip: create -> list -> rename -> list -> kill -> list.
func TestClientFullLifecycle(t *testing.T) {
	_, client, _ := testDaemon(t)

	// Create.
	id, err := client.CreateSession("tab-one", "", 0, 0)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// List — should have 1 session.
	sessions, err := client.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Name != "tab-one" {
		t.Errorf("name: got %q, want %q", sessions[0].Name, "tab-one")
	}

	// Rename.
	if err := client.RenameSession(id, "tab-renamed"); err != nil {
		t.Fatalf("RenameSession: %v", err)
	}

	// List — should show new name.
	sessions, err = client.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions after rename: %v", err)
	}
	if len(sessions) != 1 || sessions[0].Name != "tab-renamed" {
		t.Errorf("after rename: got %+v", sessions)
	}

	// Kill.
	if err := client.KillSession(id); err != nil {
		t.Fatalf("KillSession: %v", err)
	}

	// Give the session a moment to be removed.
	time.Sleep(50 * time.Millisecond)

	// List — should be empty.
	sessions, err = client.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions after kill: %v", err)
	}
	for _, s := range sessions {
		if s.ID == id {
			t.Errorf("killed session %q still in list", id)
		}
	}
}

// ─── Phase 123-02 Task 1: TestExchangeJoinCode_* ─────────────────────────────
//
// Tests for DaemonClient.ExchangeJoinCodeAtURL (FSW-10 / TD-5). The function
// must parse a 303 See Other Location header to extract the ?cap=<token>, map
// error-shape Locations (/join?error=<kind>) onto the modal error-substring
// contract, and NOT auto-follow the redirect.

// TestExchangeJoinCode_303Success: 303 with Location "/sessions/abc?cap=TOKEN123" → "TOKEN123", nil.
func TestExchangeJoinCode_303Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/join/exchange" {
			http.Error(w, "unexpected", http.StatusBadRequest)
			return
		}
		http.Redirect(w, r, "/sessions/abc?cap=TOKEN123", http.StatusSeeOther)
	}))
	defer srv.Close()

	c := &DaemonClient{}
	tok, err := c.ExchangeJoinCodeAtURL(srv.URL, "ABCDE")
	if err != nil {
		t.Fatalf("ExchangeJoinCodeAtURL: unexpected error: %v", err)
	}
	if tok != "TOKEN123" {
		t.Errorf("token = %q, want %q", tok, "TOKEN123")
	}
}

// TestExchangeJoinCode_NoAutoFollow: the redirect target (/sessions/abc) must NOT be hit.
func TestExchangeJoinCode_NoAutoFollow(t *testing.T) {
	redirectHit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/join/exchange":
			http.Redirect(w, r, "/sessions/abc?cap=CAPS99", http.StatusSeeOther)
		case "/sessions/abc":
			redirectHit = true
			w.WriteHeader(http.StatusOK)
		default:
			http.Error(w, "unexpected path: "+r.URL.Path, http.StatusBadRequest)
		}
	}))
	defer srv.Close()

	c := &DaemonClient{}
	tok, err := c.ExchangeJoinCodeAtURL(srv.URL, "ABCDE")
	if err != nil {
		t.Fatalf("ExchangeJoinCodeAtURL: unexpected error: %v", err)
	}
	if tok != "CAPS99" {
		t.Errorf("token = %q, want %q", tok, "CAPS99")
	}
	if redirectHit {
		t.Error("redirect target /sessions/abc was followed; CheckRedirect: ErrUseLastResponse must be set")
	}
}

// TestExchangeJoinCode_ErrorLocation: Location /join?error=<kind> → error containing that substring.
func TestExchangeJoinCode_ErrorLocation(t *testing.T) {
	cases := []struct {
		kind    string
		wantSub string
	}{
		{"expired", "expired"},
		{"invalid", "invalid"},
		{"not-found", "not-found"},
		{"session-gone", "session-gone"},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "/join?error="+tc.kind, http.StatusSeeOther)
			}))
			defer srv.Close()

			c := &DaemonClient{}
			_, err := c.ExchangeJoinCodeAtURL(srv.URL, "ABCDE")
			if err == nil {
				t.Fatalf("want error containing %q, got nil", tc.wantSub)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantSub)
			}
		})
	}
}

// TestExchangeJoinCode_EmptyCap: 303 with Location lacking a cap query → error containing "invalid".
func TestExchangeJoinCode_EmptyCap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/sessions/abc", http.StatusSeeOther)
	}))
	defer srv.Close()

	c := &DaemonClient{}
	_, err := c.ExchangeJoinCodeAtURL(srv.URL, "ABCDE")
	if err == nil {
		t.Fatal("want error containing 'invalid', got nil")
	}
	if !strings.Contains(err.Error(), "invalid") {
		t.Errorf("error %q does not contain 'invalid'", err.Error())
	}
}

// TestExchangeJoinCode_SharedClientUnchanged: remoteFilesHTTPClient.CheckRedirect is still nil after the call.
func TestExchangeJoinCode_SharedClientUnchanged(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/sessions/x?cap=TOK", http.StatusSeeOther)
	}))
	defer srv.Close()

	c := &DaemonClient{}
	if _, err := c.ExchangeJoinCodeAtURL(srv.URL, "ABCDE"); err != nil {
		t.Fatalf("ExchangeJoinCodeAtURL: %v", err)
	}
	if remoteFilesHTTPClient.CheckRedirect != nil {
		t.Error("remoteFilesHTTPClient.CheckRedirect was mutated; it must remain nil")
	}
}

// TestExchangeJoinCode_AbsoluteLocationHostMismatch: 303 with an absolute
// Location pointing at a different host → error containing "invalid" (WR-04).
// With InsecureSkipVerify on the dedicated transport, a tailnet MITM could
// return Location: https://attacker/sessions/x?cap=STEAL — the host check
// must reject it before returning the cap token.
func TestExchangeJoinCode_AbsoluteLocationHostMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return an absolute Location whose host differs from the request target.
		http.Redirect(w, r, "https://attacker.example.com/sessions/x?cap=STOLEN", http.StatusSeeOther)
	}))
	defer srv.Close()

	c := &DaemonClient{}
	_, err := c.ExchangeJoinCodeAtURL(srv.URL, "ABCDE")
	if err == nil {
		t.Fatal("want error for mismatched Location host, got nil")
	}
	if !strings.Contains(err.Error(), "invalid") {
		t.Errorf("error %q should contain 'invalid' for mismatched host", err.Error())
	}
}

// TestExchangeJoinCode_AbsoluteLocationSameHost: absolute Location with the
// same host and scheme as the request target must succeed (WR-04 positive control).
func TestExchangeJoinCode_AbsoluteLocationSameHost(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/join/exchange" {
			// Return an absolute Location pointing at the same host.
			http.Redirect(w, r, "http://"+r.Host+"/sessions/abc?cap=TOKEN_ABS", http.StatusSeeOther)
		}
	}))
	defer srv.Close()

	c := &DaemonClient{}
	tok, err := c.ExchangeJoinCodeAtURL(srv.URL, "ABCDE")
	if err != nil {
		t.Fatalf("ExchangeJoinCodeAtURL with same-host absolute Location: %v", err)
	}
	if tok != "TOKEN_ABS" {
		t.Errorf("token = %q, want %q", tok, "TOKEN_ABS")
	}
}

// TestExchangeJoinCode_WR05_AbsoluteErrorLocation: absolute Location of the
// form "http://host/join?error=expired" must extract error kind via url.Parse
// (WR-05). The old strings.TrimPrefix approach would return the full URL as
// the kind; the new url.Parse approach extracts just "expired".
func TestExchangeJoinCode_WR05_AbsoluteErrorLocation(t *testing.T) {
	cases := []struct {
		kind    string
		wantSub string
	}{
		{"expired", "expired"},
		{"session-gone", "session-gone"},
		{"not-found", "not-found"},
		{"invalid", "invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Return an ABSOLUTE error Location (same host, different path).
				http.Redirect(w, r, "http://"+r.Host+"/join?error="+tc.kind, http.StatusSeeOther)
			}))
			defer srv.Close()

			c := &DaemonClient{}
			_, err := c.ExchangeJoinCodeAtURL(srv.URL, "ABCDE")
			if err == nil {
				t.Fatalf("want error containing %q, got nil", tc.wantSub)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error %q does not contain %q (WR-05: absolute error Location not parsed correctly)", err.Error(), tc.wantSub)
			}
		})
	}
}

// TestDaemonClient_SetSessionFunnel verifies that DaemonClient.SetSessionFunnel
// POSTs to /sessions/{id}/funnel with the correct JSON body {enabled, expiresIn}.
// Phase 165 / FNL-01.
func TestDaemonClient_SetSessionFunnel(t *testing.T) {
	var (
		gotMethod  string
		gotPath    string
		gotBody    []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r.Body)
		gotBody = buf.Bytes()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &DaemonClient{
		http: srv.Client(),
		base: srv.URL,
	}

	if err := c.SetSessionFunnel("sess-abc", true, 3600); err != nil {
		t.Fatalf("SetSessionFunnel: unexpected error: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %q; want POST", gotMethod)
	}
	if gotPath != "/sessions/sess-abc/funnel" {
		t.Errorf("path = %q; want /sessions/sess-abc/funnel", gotPath)
	}
	wantBody := `{"enabled":true,"expiresIn":3600}`
	if strings.TrimSpace(string(gotBody)) != wantBody {
		t.Errorf("body = %q; want %q", strings.TrimSpace(string(gotBody)), wantBody)
	}
}

// TestDaemonClient_SetSessionFunnel_ErrorStatus verifies that a non-2xx response
// from the daemon surfaces as a non-nil error (mirrors ToggleWebServing behaviour).
// Phase 165 / FNL-01.
func TestDaemonClient_SetSessionFunnel_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := &DaemonClient{
		http: srv.Client(),
		base: srv.URL,
	}

	err := c.SetSessionFunnel("sess-abc", true, 0)
	if err == nil {
		t.Fatal("SetSessionFunnel with 500 response: expected error, got nil")
	}
}
