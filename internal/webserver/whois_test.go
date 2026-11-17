package webserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/scottkw/portty/internal/tailnet"
	"tailscale.com/client/local"
	"tailscale.com/client/tailscale/apitype"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/tailcfg"
	"tailscale.com/types/views"
)

const testSelfUser tailcfg.UserID = 42

// fakeWhoIs answers WhoIs by the host part of the remote address; unknown
// hosts get local.ErrPeerNotFound. A non-nil err fails both calls.
type fakeWhoIs struct {
	self   *ipnstate.Status
	byAddr map[string]*apitype.WhoIsResponse
	err    error
}

func (f *fakeWhoIs) WhoIs(_ context.Context, remoteAddr string) (*apitype.WhoIsResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	if who, ok := f.byAddr[host]; ok {
		return who, nil
	}
	return nil, local.ErrPeerNotFound
}

func (f *fakeWhoIs) StatusWithoutPeers(context.Context) (*ipnstate.Status, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.self, nil
}

// whoIsAs makes every loopback test request look like a node of peerUser.
func whoIsAs(peerUser tailcfg.UserID) *fakeWhoIs {
	return &fakeWhoIs{
		self:   &ipnstate.Status{Self: &ipnstate.PeerStatus{UserID: testSelfUser}},
		byAddr: map[string]*apitype.WhoIsResponse{"127.0.0.1": {Node: &tailcfg.Node{User: peerUser}}},
	}
}

// SameUserWhoIsForTest is a fake WhoIs client under which every loopback
// request comes from this Mac's own Tailscale user. Exported for the
// webserver_test package.
func SameUserWhoIsForTest() *fakeWhoIs { return whoIsAs(testSelfUser) }

func TestSameUser(t *testing.T) {
	tags := views.SliceOf([]string{"tag:server"})
	self := func() *ipnstate.PeerStatus { return &ipnstate.PeerStatus{UserID: testSelfUser} }
	who := func(n *tailcfg.Node) *apitype.WhoIsResponse { return &apitype.WhoIsResponse{Node: n} }
	cases := []struct {
		name string
		who  *apitype.WhoIsResponse
		self *ipnstate.PeerStatus
		want bool
	}{
		{"same user allowed", who(&tailcfg.Node{User: testSelfUser}), self(), true},
		{"other user", who(&tailcfg.Node{User: 7}), self(), false},
		{"tagged peer", who(&tailcfg.Node{User: testSelfUser, Tags: []string{"tag:ci"}}), self(), false},
		{"tagged self", who(&tailcfg.Node{User: testSelfUser}), &ipnstate.PeerStatus{UserID: testSelfUser, Tags: &tags}, false},
		{"peer shared in", who(&tailcfg.Node{User: testSelfUser, Sharer: 9}), self(), false},
		{"self shared in", who(&tailcfg.Node{User: testSelfUser}), &ipnstate.PeerStatus{UserID: testSelfUser, AltSharerUserID: 9}, false},
		{"self user zero", who(&tailcfg.Node{User: 0}), &ipnstate.PeerStatus{UserID: 0}, false},
		{"nil node", who(nil), self(), false},
		{"nil response", nil, self(), false},
		{"nil self", who(&tailcfg.Node{User: testSelfUser}), nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sameUser(c.who, c.self); got != c.want {
				t.Fatalf("sameUser = %v, want %v", got, c.want)
			}
		})
	}
}

func assertRefused(t *testing.T, resp *http.Response) {
	t.Helper()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
	if got := resp.Header.Get(tailnet.MarkerHeader); got != tailnet.MarkerValue {
		t.Fatalf("X-Portty = %q, want %q on the refusal", got, tailnet.MarkerValue)
	}
}

func assertNoCORS(t *testing.T, resp *http.Response) {
	t.Helper()
	for k := range resp.Header {
		if strings.HasPrefix(k, "Access-Control-") {
			t.Fatalf("response carries %s; WhoIs-gated routes must send no CORS headers", k)
		}
	}
}

func TestWhoIs_MetaSameUser(t *testing.T) {
	ws, client := testServer(t)
	ws.SetWhoIsClientForTest(SameUserWhoIsForTest())
	ws.EnableSession("sess-a")
	resp, err := client.Get(ws.BaseURL() + "/api/sessions/meta")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	assertNoCORS(t, resp)
	var items []sessionMetaItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil || len(items) != 1 || items[0].ID != "sess-a" {
		t.Fatalf("items = %+v, %v; want the one enabled session", items, err)
	}
}

func TestWhoIs_MetaRefused(t *testing.T) {
	statusErr := whoIsAs(testSelfUser)
	statusErr.self = nil
	for name, fake := range map[string]*fakeWhoIs{
		"other user":     whoIsAs(7),
		"whois error":    {self: whoIsAs(testSelfUser).self, byAddr: nil},
		"local api":      {err: errors.New("tailscaled down")},
		"status no self": statusErr,
	} {
		t.Run(name, func(t *testing.T) {
			ws, client := testServer(t)
			ws.SetWhoIsClientForTest(fake)
			ws.EnableSession("sess-a")
			resp, err := client.Get(ws.BaseURL() + "/api/sessions/meta")
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			assertRefused(t, resp)
			assertNoCORS(t, resp)
			var buf bytes.Buffer
			_, _ = buf.ReadFrom(resp.Body)
			if bytes.Contains(buf.Bytes(), []byte("sess-a")) {
				t.Fatalf("refusal body leaks the session: %q", buf.String())
			}
		})
	}
}

func postPeerCap(t *testing.T, ws *WebServer, client *http.Client, id, contentType string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, ws.BaseURL()+"/api/peer/sessions/"+id+"/cap", bytes.NewReader([]byte("{}")))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", contentType)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func TestPeerCap_Routes(t *testing.T) {
	ws, client := testServer(t)
	ws.SetWhoIsClientForTest(SameUserWhoIsForTest())
	ws.SetShareTokenProvider(func(id string) (string, bool) {
		if id == "shared" {
			return "tok-shared", true
		}
		return "", false
	})

	t.Run("same user gets the token", func(t *testing.T) {
		resp := postPeerCap(t, ws, client, "shared", "application/json; charset=utf-8")
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		if got := resp.Header.Get("Cache-Control"); got != "no-store" {
			t.Fatalf("Cache-Control = %q, want no-store", got)
		}
		assertNoCORS(t, resp)
		var body struct{ Cap string }
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil || body.Cap != "tok-shared" {
			t.Fatalf("body = %+v, %v; want cap tok-shared", body, err)
		}
	})
	t.Run("not shared is 404", func(t *testing.T) {
		resp := postPeerCap(t, ws, client, "other", "application/json")
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", resp.StatusCode)
		}
		assertNoCORS(t, resp)
	})
	t.Run("text/plain is 415", func(t *testing.T) {
		resp := postPeerCap(t, ws, client, "shared", "text/plain")
		if resp.StatusCode != http.StatusUnsupportedMediaType {
			t.Fatalf("status = %d, want 415", resp.StatusCode)
		}
		assertNoCORS(t, resp)
	})
	t.Run("GET never returns the token", func(t *testing.T) {
		resp, err := client.Get(ws.BaseURL() + "/api/peer/sessions/shared/cap")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		// The GET / catch-all answers 404; the point is that GET never yields the token.
		if resp.StatusCode == http.StatusOK {
			t.Fatalf("GET status = 200; the capability route must be POST-only")
		}
	})
	t.Run("other user is 403", func(t *testing.T) {
		ws, client := testServer(t)
		ws.SetWhoIsClientForTest(whoIsAs(7))
		ws.SetShareTokenProvider(func(string) (string, bool) { return "tok-shared", true })
		resp := postPeerCap(t, ws, client, "shared", "application/json")
		assertRefused(t, resp)
		assertNoCORS(t, resp)
	})
}

func TestPeerCap_NoProviderIs404(t *testing.T) {
	ws, client := testServer(t)
	ws.SetWhoIsClientForTest(SameUserWhoIsForTest())
	if resp := postPeerCap(t, ws, client, "any", "application/json"); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 with no provider", resp.StatusCode)
	}
}
