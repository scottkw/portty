package webserver

import (
	"context"
	"encoding/json"
	"net/http"
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
