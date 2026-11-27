package webserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/scottkw/portty/internal/tailnet"
)

// TestDesktopMeta_Tracer: the office meta answer carries X-Portty-Desktop:
// always exactly while the desktop is on, the body stays a bare array, and the
// tailnet reader turns the header into Desktop.
func TestDesktopMeta_Tracer(t *testing.T) {
	e := newDesktopEnv(t)
	ctx := context.Background()

	resp, body := e.get(t, "/api/sessions/meta")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("meta = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get(tailnet.DesktopHeader); got != tailnet.DesktopAlways {
		t.Fatalf("%s = %q with the desktop on, want %q", tailnet.DesktopHeader, got, tailnet.DesktopAlways)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(body, &items); err != nil {
		t.Fatalf("body %q is not a bare JSON array: %v", body, err)
	}
	m, err := tailnet.FetchPeerMetaWithClient(ctx, e.ws.BaseURL(), e.client)
	if err != nil || m.Status != tailnet.MetaOK || !m.Desktop {
		t.Fatalf("FetchPeerMetaWithClient = %+v, %v; want OK with Desktop true", m, err)
	}

	e.backend.off.Store(true)
	resp, _ = e.get(t, "/api/sessions/meta")
	if got := resp.Header.Get(tailnet.DesktopHeader); got != "" {
		t.Errorf("%s = %q with the desktop off, want none", tailnet.DesktopHeader, got)
	}
	m, err = tailnet.FetchPeerMetaWithClient(ctx, e.ws.BaseURL(), e.client)
	if err != nil || m.Status != tailnet.MetaOK || m.Desktop {
		t.Errorf("desktop off: %+v, %v; want OK with Desktop false", m, err)
	}
}

// With no backend at all the header is absent and the reader says Desktop false.
func TestDesktopMeta_NoBackend(t *testing.T) {
	e := newDesktopEnv(t)
	e.ws.SetDesktopBackend(nil)
	resp, _ := e.get(t, "/api/sessions/meta")
	if got := resp.Header.Get(tailnet.DesktopHeader); got != "" {
		t.Fatalf("%s = %q with no backend, want none", tailnet.DesktopHeader, got)
	}
	m, err := tailnet.FetchPeerMetaWithClient(context.Background(), e.ws.BaseURL(), e.client)
	if err != nil || m.Status != tailnet.MetaOK || m.Desktop {
		t.Fatalf("no backend: %+v, %v; want OK with Desktop false", m, err)
	}
}

// A peer whose only content is its desktop is discoverable: an empty session
// list still carries the header.
func TestDesktopMeta_EmptyListStillAnnounces(t *testing.T) {
	e := newDesktopEnv(t)
	e.ws.DisableSession(ppSID)
	m, err := tailnet.FetchPeerMetaWithClient(context.Background(), e.ws.BaseURL(), e.client)
	if err != nil || m.Status != tailnet.MetaOK || len(m.Sessions) != 0 || !m.Desktop {
		t.Fatalf("empty list: %+v, %v; want OK, zero sessions, Desktop true", m, err)
	}
}

// Another account is refused before the header is written: no Desktop, and the
// answer reads as forbidden.
func TestDesktopMeta_OtherAccountLearnsNothing(t *testing.T) {
	e := newDesktopEnv(t)
	e.ws.SetWhoIsClientForTest(whoIsAs(7))
	resp, _ := e.get(t, "/api/sessions/meta")
	assertRefused(t, resp)
	if got := resp.Header.Get(tailnet.DesktopHeader); got != "" {
		t.Errorf("a different account saw %s = %q", tailnet.DesktopHeader, got)
	}
	m, err := tailnet.FetchPeerMetaWithClient(context.Background(), e.ws.BaseURL(), e.client)
	if err != nil || m.Status != tailnet.MetaForbidden || m.Desktop {
		t.Errorf("different account: %+v, %v; want Forbidden with Desktop false", m, err)
	}
}

// tokenBackend is the fake desktop backend plus the optional DesktopToken
// method the cap route looks for. The token is only offered while the link is on.
type tokenBackend struct {
	*fakeDesktopBackend
	tok string
}

func (b tokenBackend) DesktopToken() (string, bool) { return b.tok, !b.off.Load() }

// post sends a POST with the given content type and body.
func (e *desktopEnv) post(t *testing.T, path, contentType, body string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, e.ws.BaseURL()+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp, b
}

const peerDesktopCapPath = "/api/peer/desktop/cap"

func newCapEnv(t *testing.T) (*desktopEnv, string) {
	t.Helper()
	e := newDesktopEnv(t)
	tok := desktopToken(t, "g1")
	e.ws.SetDesktopBackend(tokenBackend{e.backend, tok})
	return e, tok
}

func TestPeerDesktopCap_Success(t *testing.T) {
	e, tok := newCapEnv(t)
	resp, body := e.post(t, peerDesktopCapPath, "application/json", "{}")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	assertNoCORS(t, resp)
	var out map[string]string
	if err := json.Unmarshal(body, &out); err != nil || out["cap"] != tok || len(out) != 1 {
		t.Errorf("body = %q (%v), want only the standing token", body, err)
	}
}

func TestPeerDesktopCap_ContentTypeAndBody(t *testing.T) {
	e, _ := newCapEnv(t)
	for _, ct := range []string{"", "text/plain", "application/x-www-form-urlencoded"} {
		if resp, _ := e.post(t, peerDesktopCapPath, ct, "{}"); resp.StatusCode != http.StatusUnsupportedMediaType {
			t.Errorf("content type %q: status = %d, want 415", ct, resp.StatusCode)
		}
	}
	// A 9 KiB body is cut by MaxBytesReader; the route still answers cleanly.
	big := `{"x":"` + strings.Repeat("a", 9<<10) + `"}`
	if resp, _ := e.post(t, peerDesktopCapPath, "application/json; charset=utf-8", big); resp.StatusCode >= 500 {
		t.Errorf("9 KiB body: status = %d, want no server error", resp.StatusCode)
	}
	// The catch-all "GET /" answers 404 before the mux can say 405; either way a
	// GET never reaches the handler (the same as the session cap route).
	if resp, body := e.get(t, peerDesktopCapPath); resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET: status = %d %q, want 404 or 405", resp.StatusCode, body)
	}
}

func TestPeerDesktopCap_NotAvailable(t *testing.T) {
	// Off: the backend has the method but reports no token.
	e, _ := newCapEnv(t)
	e.backend.off.Store(true)
	if resp, _ := e.post(t, peerDesktopCapPath, "application/json", "{}"); resp.StatusCode != http.StatusNotFound {
		t.Errorf("off: status = %d, want 404", resp.StatusCode)
	}
	// A backend without DesktopToken.
	e = newDesktopEnv(t)
	if resp, _ := e.post(t, peerDesktopCapPath, "application/json", "{}"); resp.StatusCode != http.StatusNotFound {
		t.Errorf("no DesktopToken method: status = %d, want 404", resp.StatusCode)
	}
	// No backend at all.
	e.ws.SetDesktopBackend(nil)
	if resp, _ := e.post(t, peerDesktopCapPath, "application/json", "{}"); resp.StatusCode != http.StatusNotFound {
		t.Errorf("nil backend: status = %d, want 404", resp.StatusCode)
	}
}

func TestPeerDesktopCap_RefusesOtherAccountAndFault(t *testing.T) {
	e, tok := newCapEnv(t)
	e.ws.SetWhoIsClientForTest(whoIsAs(7))
	resp, body := e.post(t, peerDesktopCapPath, "application/json", "{}")
	assertRefused(t, resp)
	if strings.Contains(string(body), tok) {
		t.Error("the refusal carries the token")
	}
	e.ws.SetWhoIsClientForTest(&fakeWhoIs{err: errors.New("tailscaled down")})
	resp, body = e.post(t, peerDesktopCapPath, "application/json", "{}")
	if resp.StatusCode != http.StatusServiceUnavailable || resp.Header.Get(tailnet.MarkerHeader) != "" {
		t.Errorf("fault: status = %d marker %q, want 503 with no marker", resp.StatusCode, resp.Header.Get(tailnet.MarkerHeader))
	}
	if strings.Contains(string(body), tok) {
		t.Error("the fault answer carries the token")
	}
}

// The token the route hands out opens only the desktop (DSHR-08), and dies with
// always-available.
func TestPeerDesktopCap_TokenOnlyOpensDesktop(t *testing.T) {
	e, _ := newCapEnv(t)
	_, body := e.post(t, peerDesktopCapPath, "application/json", "{}")
	var out map[string]string
	if err := json.Unmarshal(body, &out); err != nil || out["cap"] == "" {
		t.Fatalf("no token in %q: %v", body, err)
	}
	tok := out["cap"]
	if resp, _ := e.get(t, "/desktop/check?cap="+tok); resp.StatusCode != http.StatusNoContent {
		t.Errorf("/desktop/check = %d, want 204", resp.StatusCode)
	}
	if resp, _ := e.get(t, "/api/sessions?cap="+tok); resp.StatusCode < 400 {
		t.Errorf("/api/sessions accepted the desktop token: %d", resp.StatusCode)
	}
	if resp := e.upgrade(t, "/sessions/"+ppSID+"/ws?cap="+tok); resp.StatusCode < 400 {
		t.Errorf("/sessions/{id}/ws accepted the desktop token: %d", resp.StatusCode)
	}
	e.backend.off.Store(true)
	if resp, _ := e.get(t, "/desktop/check?cap="+tok); resp.StatusCode != http.StatusGone {
		t.Errorf("after off, /desktop/check = %d, want 410", resp.StatusCode)
	}
}
