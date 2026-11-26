package webserver

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"

	"github.com/scottkw/portty/internal/capability"
)

// fakeDesktopBackend is a DesktopBackend with a toggleable link and viewer cap.
type fakeDesktopBackend struct {
	grant string
	off   atomic.Bool
	full  atomic.Bool
	calls atomic.Int32
}

func (b *fakeDesktopBackend) DesktopGrant() (string, bool) { return b.grant, !b.off.Load() }
func (b *fakeDesktopBackend) Full() bool                   { return b.full.Load() }
func (b *fakeDesktopBackend) ServeDesktopViewer(_ context.Context, c *websocket.Conn) {
	b.calls.Add(1)
	_ = c.Close(websocket.StatusNormalClosure, "")
}

// desktopEnv is a started share server with a static bundle, a signing key and
// a desktop backend for grant "g1".
type desktopEnv struct {
	ws      *WebServer
	client  *http.Client
	backend *fakeDesktopBackend
}

func newDesktopEnv(t *testing.T) *desktopEnv {
	t.Helper()
	ws, client := testServer(t)
	ws.SetSigningKey(capTestKey)
	manifest, err := os.ReadFile("../../frontend/public/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	ws.SetStaticAppFS(fstest.MapFS{
		"index.html":          {Data: []byte(fakeIndexHTML)},
		"manifest.json":       {Data: manifest},
		"assets/index-abc.js": {Data: []byte("// js")},
	})
	b := &fakeDesktopBackend{grant: "g1"}
	ws.SetDesktopBackend(b)
	c := *client
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &desktopEnv{ws: ws, client: &c, backend: b}
}

func (e *desktopEnv) get(t *testing.T, path string) (*http.Response, []byte) {
	t.Helper()
	resp, err := e.client.Get(e.ws.BaseURL() + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, body
}

// upgrade sends a browser-shaped WebSocket upgrade request to /desktop/ws.
func (e *desktopEnv) upgrade(t *testing.T, path string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, e.ws.BaseURL()+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	req.Header.Set("Origin", e.ws.BaseURL())
	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatalf("upgrade %s: %v", path, err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func desktopToken(t *testing.T, grant string) string {
	t.Helper()
	tok, err := capability.SignDesktop(capability.DesktopClaims{
		Kind: capability.DesktopTokenKind, Mode: capability.DesktopModeAlways,
		GrantID: grant, IAT: time.Now().Unix(), V: 1,
	}, capTestKey)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func sessionCapToken(t *testing.T, sid string) string {
	t.Helper()
	tok, err := capability.Sign(capability.Claims{SID: sid, Perms: "read,write", GrantID: "grant-" + sid, V: 1}, capTestKey)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func TestDesktopRoutes_Outcomes(t *testing.T) {
	e := newDesktopEnv(t)
	good := desktopToken(t, "g1")
	session := sessionCapToken(t, "s1")
	stale := desktopToken(t, "old")

	// Page route: a good token gets the index; each refusal gets its own fixed
	// status, header and static page.
	resp, body := e.get(t, "/desktop/?cap="+good)
	if resp.StatusCode != http.StatusOK || string(body) != fakeIndexHTML {
		t.Fatalf("good token: %d %q, want 200 and the index", resp.StatusCode, body)
	}
	for name, tok := range map[string]string{"session capability": session, "stale grant": stale, "no token": "", "garbage": "not-a-token"} {
		resp, body := e.get(t, "/desktop/?cap="+tok)
		if resp.StatusCode != http.StatusUnauthorized || resp.Header.Get("X-Portty-Desktop") != "token" {
			t.Errorf("%s: %d header %q, want 401 token", name, resp.StatusCode, resp.Header.Get("X-Portty-Desktop"))
		}
		if !bytes.Equal(body, desktopLinkInvalidPage) {
			t.Errorf("%s: body is not the invalid-link page", name)
		}
		if resp.Header.Get("X-Portty") != "" {
			t.Errorf("%s: carries the X-Portty marker; only the user refusal may", name)
		}
	}

	e.backend.off.Store(true)
	resp, body = e.get(t, "/desktop/?cap="+good)
	if resp.StatusCode != http.StatusGone || resp.Header.Get("X-Portty-Desktop") != "off" || !bytes.Equal(body, desktopOffPage) {
		t.Errorf("off: %d header %q, want 410 off and the off page", resp.StatusCode, resp.Header.Get("X-Portty-Desktop"))
	}
	e.backend.off.Store(false)

	// The two pages differ, and carry no script and no link but the stylesheet.
	if bytes.Equal(desktopOffPage, desktopLinkInvalidPage) {
		t.Error("off and invalid pages are identical")
	}
	for name, page := range map[string][]byte{"off": desktopOffPage, "invalid": desktopLinkInvalidPage} {
		s := string(page)
		if strings.Contains(s, "<script") || strings.Count(s, "href=") != 1 || !strings.Contains(s, `href="/assets/dashboard.css"`) {
			t.Errorf("%s page has a script or a link besides the stylesheet", name)
		}
	}

	// Check route: 204, 401 token, 410 off, with the code in the header and body.
	check := func(path string, status int, reason string) {
		t.Helper()
		resp, body := e.get(t, path)
		if resp.StatusCode != status || resp.Header.Get("X-Portty-Desktop") != reason {
			t.Errorf("%s: %d header %q, want %d %q", path, resp.StatusCode, resp.Header.Get("X-Portty-Desktop"), status, reason)
		}
		if want := `{"reason":"` + reason + `"}`; reason != "" && string(body) != want {
			t.Errorf("%s: body %q, want %q", path, body, want)
		}
		if reason == "" && len(body) != 0 {
			t.Errorf("%s: 204 has a body %q", path, body)
		}
	}
	check("/desktop/check?cap="+good, http.StatusNoContent, "")
	check("/desktop/check?cap="+session, http.StatusUnauthorized, "token")
	check("/desktop/check?cap="+stale, http.StatusUnauthorized, "token")
	e.backend.off.Store(true)
	check("/desktop/check?cap="+good, http.StatusGone, "off")
	e.backend.off.Store(false)

	// Socket route: the same refusals before any upgrade, and the viewer cap.
	if r := e.upgrade(t, "/desktop/ws?cap="+session); r.StatusCode != http.StatusUnauthorized || r.Header.Get("X-Portty-Desktop") != "token" || r.Header.Get("Upgrade") != "" {
		t.Errorf("ws with a session capability: %d %q, want 401 token", r.StatusCode, r.Header.Get("X-Portty-Desktop"))
	}
	e.backend.off.Store(true)
	if r := e.upgrade(t, "/desktop/ws?cap="+good); r.StatusCode != http.StatusGone || r.Header.Get("X-Portty-Desktop") != "off" {
		t.Errorf("ws when off: %d %q, want 410 off", r.StatusCode, r.Header.Get("X-Portty-Desktop"))
	}
	e.backend.off.Store(false)
	e.backend.full.Store(true)
	if r := e.upgrade(t, "/desktop/ws?cap="+good); r.StatusCode != http.StatusTooManyRequests || r.Header.Get("Upgrade") != "" {
		t.Errorf("ws when full: %d upgrade %q, want 429 and no upgrade", r.StatusCode, r.Header.Get("Upgrade"))
	}
	if n := e.backend.calls.Load(); n != 0 {
		t.Errorf("backend saw %d sockets from refused requests, want 0", n)
	}
	e.backend.full.Store(false)
	if r := e.upgrade(t, "/desktop/ws?cap="+good); r.StatusCode != http.StatusSwitchingProtocols {
		t.Errorf("ws with room: %d, want 101", r.StatusCode)
	}
}

// A nil backend means desktop links are off: a validly signed token gets the
// off answer, not the invalid one.
func TestDesktopRoutes_NoBackendIsOff(t *testing.T) {
	e := newDesktopEnv(t)
	e.ws.SetDesktopBackend(nil)
	resp, _ := e.get(t, "/desktop/check?cap="+desktopToken(t, "g1"))
	if resp.StatusCode != http.StatusGone || resp.Header.Get("X-Portty-Desktop") != "off" {
		t.Errorf("no backend: %d %q, want 410 off", resp.StatusCode, resp.Header.Get("X-Portty-Desktop"))
	}
}
