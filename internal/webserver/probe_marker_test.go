package webserver

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/scottkw/portty/internal/tailnet"
)

// Peer discovery (internal/tailnet) accepts a host only when its GET
// /api/sessions reply carries X-Portty: 1, so the header must be on the 401
// an unauthenticated probe gets, on the 200, and on no other route.

func TestProbeMarker_UnauthenticatedSessionsCarriesMarker(t *testing.T) {
	ws, client := testServer(t)
	ws.SetSigningKey(capTestKey)
	ws.EnableSession("s-marker")

	resp, err := client.Get(ws.BaseURL() + "/api/sessions")
	if err != nil {
		t.Fatalf("GET /api/sessions: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without a cap, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "capability required") {
		t.Errorf("expected body 'capability required', got %q", string(body))
	}
	if got := resp.Header.Get(tailnet.MarkerHeader); got != tailnet.MarkerValue {
		t.Errorf("X-Portty on 401 = %q, want %q", got, tailnet.MarkerValue)
	}
}

func TestProbeMarker_AuthorizedSessionsCarriesMarker(t *testing.T) {
	ws, client := testServer(t)
	ws.SetSigningKey(capTestKey)
	ws.EnableSession("s-marker")
	token := issueCapFor(t, ws, "s-marker", "read")

	resp, err := client.Get(ws.BaseURL() + "/api/sessions?cap=" + token)
	if err != nil {
		t.Fatalf("GET /api/sessions?cap=: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with a valid cap, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get(tailnet.MarkerHeader); got != tailnet.MarkerValue {
		t.Errorf("X-Portty on 200 = %q, want %q", got, tailnet.MarkerValue)
	}
}

func TestProbeMarker_OtherRoutesHaveNoMarker(t *testing.T) {
	ws, client := testServer(t)
	ws.SetSigningKey(capTestKey)
	ws.EnableSession("s-marker")

	for _, path := range []string{"/api/sessions/meta", "/dashboard"} {
		resp, err := client.Get(ws.BaseURL() + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		resp.Body.Close()
		if got := resp.Header.Get(tailnet.MarkerHeader); got != "" {
			t.Errorf("GET %s: X-Portty = %q, want none", path, got)
		}
	}
}
