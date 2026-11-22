package webserver

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"
	"github.com/scottkw/portty/internal/tailnet"
	"github.com/scottkw/portty/internal/webserver/webservertest"
)

// gateCase is what a gated route answers for one identity: other accounts get
// 403 with the X-Portty marker; a tailscaled fault gets 503 with no marker
// (MOB-05, WR-08). The same user gets the route's own answer, tested per route.
type gateCase struct {
	name   string
	id     webservertest.Identity
	status int
	marker bool
}

var gateRefusals = []gateCase{
	{"other", webservertest.Other, http.StatusForbidden, true},
	{"tagged", webservertest.Tagged, http.StatusForbidden, true},
	{"shared-in", webservertest.SharedIn, http.StatusForbidden, true},
	{"not-found", webservertest.NotFound, http.StatusForbidden, true},
	{"status-fault", webservertest.StatusFault, http.StatusServiceUnavailable, false},
	{"whois-fault", webservertest.WhoIsFault, http.StatusServiceUnavailable, false},
}

func checkGateAnswer(t *testing.T, resp *http.Response, c gateCase) {
	t.Helper()
	if resp.StatusCode != c.status {
		t.Errorf("%s: status = %d, want %d", c.name, resp.StatusCode, c.status)
	}
	if got := resp.Header.Get("X-Portty") == "1"; got != c.marker {
		t.Errorf("%s: X-Portty marker = %v, want %v", c.name, got, c.marker)
	}
}

func TestSameUserGate_SessionInfo(t *testing.T) {
	const sid = "sess-gate-info"
	ws, client, _, _ := testServerWithHub(t, sid)
	ws.SetSigningKey(capTestKey)
	tok := issueCapFor(t, ws, sid, "read,write")
	url := ws.BaseURL() + "/api/sessions/" + sid + "/info?cap="

	get := func(t *testing.T, id webservertest.Identity, cap string) *http.Response {
		t.Helper()
		ws.SetWhoIsClientForTest(webservertest.New(id))
		resp, err := client.Get(url + cap)
		if err != nil {
			t.Fatalf("GET info: %v", err)
		}
		t.Cleanup(func() { resp.Body.Close() })
		return resp
	}

	t.Run("same", func(t *testing.T) {
		if resp := get(t, webservertest.Same, tok); resp.StatusCode != http.StatusOK {
			t.Errorf("same user: status = %d, want 200", resp.StatusCode)
		}
	})
	for _, c := range gateRefusals {
		t.Run(c.name, func(t *testing.T) { checkGateAnswer(t, get(t, c.id, tok), c) })
	}
	// The gate runs before the capability: another account with no cap at all
	// still gets the marker 403, not the capability 401.
	t.Run("other-no-cap", func(t *testing.T) {
		checkGateAnswer(t, get(t, webservertest.Other, ""), gateCase{"other-no-cap", 0, http.StatusForbidden, true})
	})
}

// gateServer is a share server with one enabled session, a signing key and a
// valid capability, plus a getter that answers as a chosen identity.
func gateServer(t *testing.T, sid string) (ws *WebServer, client *http.Client, tok string) {
	t.Helper()
	ws, client, _, _ = testServerWithHub(t, sid)
	ws.SetSigningKey(capTestKey)
	ws.SetPluginSettingsProvider(func() []byte { return []byte(`{}`) })
	return ws, client, issueCapFor(t, ws, sid, "read,write")
}

func gateGet(t *testing.T, ws *WebServer, client *http.Client, id webservertest.Identity, path string) *http.Response {
	t.Helper()
	ws.SetWhoIsClientForTest(webservertest.New(id))
	resp, err := client.Get(ws.BaseURL() + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func TestSameUserGate_APIRoutes(t *testing.T) {
	ws, client, tok := gateServer(t, "sess-gate-api")
	// The stream route holds its response open, so only the status line and
	// headers are read; the t.Cleanup in gateGet closes the body.
	for _, route := range []string{"/api/plugin-config", "/api/plugin-config/stream"} {
		t.Run(route, func(t *testing.T) {
			if resp := gateGet(t, ws, client, webservertest.Same, route+"?cap="+tok); resp.StatusCode != http.StatusOK {
				t.Errorf("same user: status = %d, want 200", resp.StatusCode)
			}
			for _, c := range gateRefusals {
				checkGateAnswer(t, gateGet(t, ws, client, c.id, route+"?cap="+tok), c)
			}
			// The gate runs before the capability.
			checkGateAnswer(t, gateGet(t, ws, client, webservertest.Other, route),
				gateCase{"other-no-cap", 0, http.StatusForbidden, true})
		})
	}
}

// gateDial attempts the WebSocket upgrade and returns the handshake answer (nil
// when the dial failed without an HTTP response) and the dial error.
func gateDial(t *testing.T, ws *WebServer, client *http.Client, id webservertest.Identity, sid, cap, origin string) (*http.Response, *websocket.Conn, error) {
	t.Helper()
	ws.SetWhoIsClientForTest(webservertest.New(id))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h := http.Header{}
	if origin != "" {
		h.Set("Origin", origin)
	}
	url := "wss" + ws.BaseURL()[len("https"):] + "/sessions/" + sid + "/ws?cap=" + cap
	conn, resp, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPClient: client, HTTPHeader: h})
	if conn != nil {
		t.Cleanup(func() { conn.CloseNow() })
	}
	return resp, conn, err
}

func TestSameUserGate_WebSocket(t *testing.T) {
	const sid = "sess-gate-ws"
	ws, client, tok := gateServer(t, sid)
	hub, ok := ws.manager.Get(sid)
	if !ok {
		t.Fatal("hub missing")
	}
	good := ws.BaseURL()

	t.Run("same", func(t *testing.T) {
		resp, conn, err := gateDial(t, ws, client, webservertest.Same, sid, tok, good)
		if err != nil || conn == nil {
			t.Fatalf("same user upgrade failed: %v", err)
		}
		if resp.StatusCode != http.StatusSwitchingProtocols {
			t.Errorf("status = %d, want 101", resp.StatusCode)
		}
	})
	// Wait for the same-user subscriber (if still counted) to leave, so the
	// refusal rows measure only their own effect.
	deadline := time.Now().Add(3 * time.Second)
	for hub.SubscriberCount() > 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	base := hub.SubscriberCount()

	for _, c := range gateRefusals {
		t.Run(c.name, func(t *testing.T) {
			resp, conn, err := gateDial(t, ws, client, c.id, sid, tok, good)
			if err == nil || conn != nil || resp == nil {
				t.Fatalf("%s: dial should be refused with an HTTP answer (err=%v)", c.name, err)
			}
			checkGateAnswer(t, resp, c)
		})
	}
	if n := hub.SubscriberCount(); n != base {
		t.Errorf("subscribers after refusals = %d, want %d", n, base)
	}

	// Order: Origin, then WhoIs, then capability.
	t.Run("bad-origin-other", func(t *testing.T) {
		resp, _, err := gateDial(t, ws, client, webservertest.Other, sid, tok, "https://evil.example")
		if err == nil || resp == nil {
			t.Fatalf("expected refusal, err=%v", err)
		}
		checkGateAnswer(t, resp, gateCase{"bad-origin-other", 0, http.StatusForbidden, false})
	})
	t.Run("bad-origin-fault", func(t *testing.T) {
		resp, _, err := gateDial(t, ws, client, webservertest.StatusFault, sid, tok, "https://evil.example")
		if err == nil || resp == nil {
			t.Fatalf("expected refusal, err=%v", err)
		}
		checkGateAnswer(t, resp, gateCase{"bad-origin-fault", 0, http.StatusForbidden, false})
	})
	t.Run("other-garbage-cap", func(t *testing.T) {
		resp, _, err := gateDial(t, ws, client, webservertest.Other, sid, "garbage", good)
		if err == nil || resp == nil {
			t.Fatalf("expected refusal, err=%v", err)
		}
		checkGateAnswer(t, resp, gateCase{"other-garbage-cap", 0, http.StatusForbidden, true})
	})
	t.Run("same-garbage-cap", func(t *testing.T) {
		resp, _, err := gateDial(t, ws, client, webservertest.Same, sid, "garbage", good)
		if err == nil || resp == nil || resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("same user with a garbage cap: want 401, got resp=%v err=%v", resp, err)
		}
	})
	if n := hub.SubscriberCount(); n != base {
		t.Errorf("subscribers after order rows = %d, want %d", n, base)
	}
}

func TestSameUserGate_DiscoveryProbe(t *testing.T) {
	const sid = "sess-gate-probe"
	ws, client, tok := gateServer(t, sid)

	rows := []struct {
		gateCase
		probe bool // what tailnet.IsPorttyProbeResponse says
	}{
		{gateCase{"same-no-cap", webservertest.Same, http.StatusUnauthorized, true}, true},
		{gateCase{"other", webservertest.Other, http.StatusForbidden, true}, false},
		{gateCase{"tagged", webservertest.Tagged, http.StatusForbidden, true}, false},
		{gateCase{"shared-in", webservertest.SharedIn, http.StatusForbidden, true}, false},
		{gateCase{"not-found", webservertest.NotFound, http.StatusForbidden, true}, false},
		{gateCase{"status-fault", webservertest.StatusFault, http.StatusServiceUnavailable, false}, false},
		{gateCase{"whois-fault", webservertest.WhoIsFault, http.StatusServiceUnavailable, false}, false},
	}
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			resp := gateGet(t, ws, client, r.id, "/api/sessions")
			checkGateAnswer(t, resp, r.gateCase)
			if got := tailnet.IsPorttyProbeResponse(resp); got != r.probe {
				t.Errorf("IsPorttyProbeResponse = %v, want %v", got, r.probe)
			}
		})
	}
	t.Run("same-with-cap", func(t *testing.T) {
		resp := gateGet(t, ws, client, webservertest.Same, "/api/sessions?cap="+tok)
		checkGateAnswer(t, resp, gateCase{"same-with-cap", 0, http.StatusOK, true})
	})
}

func TestSameUserGate_OpenRoutes(t *testing.T) {
	ws, client, _ := gateServer(t, "sess-gate-open")
	noRedirect := *client
	noRedirect.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	for path, want := range map[string]int{
		"/":                     http.StatusFound,
		"/dashboard":            http.StatusOK,
		"/assets/dashboard.css": http.StatusOK,
	} {
		t.Run(path, func(t *testing.T) {
			ws.SetWhoIsClientForTest(webservertest.New(webservertest.Other))
			resp, err := noRedirect.Get(ws.BaseURL() + path)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != want {
				t.Errorf("status = %d, want %d", resp.StatusCode, want)
			}
		})
	}
}

// pageGateServer is gateServer with a static app bundle installed, so /app/
// has something to serve for the same user.
func pageGateServer(t *testing.T, sid string) (*WebServer, *http.Client, string) {
	t.Helper()
	ws, client, tok := gateServer(t, sid)
	ws.SetStaticAppFS(fstest.MapFS{
		"index.html":  {Data: []byte(fakeIndexHTML)},
		"assets/x.js": {Data: []byte("// js")},
	})
	return ws, client, tok
}

// checkPage asserts a refused page route answered with the gate's status and
// marker, the fixed embedded body for that status, and the share CSP.
func checkPage(t *testing.T, resp *http.Response, c gateCase) {
	t.Helper()
	checkGateAnswer(t, resp, c)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("%s: read body: %v", c.name, err)
	}
	want := userRefusedPage
	if c.status == http.StatusServiceUnavailable {
		want = serviceRetryPage
	}
	if !bytes.Equal(body, want) {
		t.Errorf("%s: body is not the embedded page for status %d", c.name, c.status)
	}
	if csp := resp.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'none'") {
		t.Errorf("%s: Content-Security-Policy = %q, want default-src 'none'", c.name, csp)
	}
}

func TestSameUserGate_AppPage(t *testing.T) {
	ws, client, tok := pageGateServer(t, "sess-gate-app")
	for _, path := range []string{"/app/?session=sess-gate-app&cap=" + tok, "/app/assets/x.js"} {
		t.Run(path, func(t *testing.T) {
			if resp := gateGet(t, ws, client, webservertest.Same, path); resp.StatusCode != http.StatusOK {
				t.Errorf("same user: status = %d, want 200", resp.StatusCode)
			}
			for _, c := range gateRefusals {
				checkPage(t, gateGet(t, ws, client, c.id, path), c)
			}
		})
	}
}
