package webserver

import (
	"bytes"
	"context"
	"encoding/json"
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
func (b *fakeDesktopBackend) ServeDesktopViewer(_ context.Context, c *websocket.Conn, _ func() bool) {
	b.calls.Add(1)
	_ = c.Close(websocket.StatusNormalClosure, "")
}

// desktopEnv is a started share server with a static bundle, a signing key and
// a desktop backend for grant "g1".
type desktopEnv struct {
	ws      *WebServer
	client  *http.Client
	backend *fakeDesktopBackend
	pp      *ppEnv
}

// newDesktopEnv builds on the port-proxy env, so one server also has a live
// session (ppSID) with a write capability, plugin settings and the port routes:
// everything the cross-token matrix needs next to the desktop routes.
func newDesktopEnv(t *testing.T) *desktopEnv {
	t.Helper()
	pp := newPPEnv(t, ppOpts{})
	ws := pp.ws
	ws.SetSessionResolver(func(id string) (string, string, string, bool) {
		return "Session", "running", "host", id == ppSID
	})
	ws.SetPluginSettingsProvider(func() []byte { return []byte(`{}`) })
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
	return &desktopEnv{ws: ws, client: pp.client, backend: b, pp: pp}
}

func (e *desktopEnv) get(t *testing.T, path string) (*http.Response, []byte) {
	t.Helper()
	resp, err := e.client.Get(e.ws.BaseURL() + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
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
	t.Cleanup(func() { _ = resp.Body.Close() })
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

// TestDesktopRoutes_CSPPinned: /desktop/ carries exactly the policy of /app/ and
// /sessions/{id}, and no media-src: the desktop needs no CSP amendment.
func TestDesktopRoutes_CSPPinned(t *testing.T) {
	e := newDesktopEnv(t)
	desktop, _ := e.get(t, "/desktop/?cap="+desktopToken(t, "g1"))
	app, _ := e.get(t, "/app/")
	session, _ := e.get(t, "/sessions/"+ppSID+"?cap="+e.pp.capTok)
	want := desktop.Header.Get("Content-Security-Policy")
	if want == "" {
		t.Fatal("/desktop/ has no Content-Security-Policy")
	}
	if got := app.Header.Get("Content-Security-Policy"); got != want {
		t.Errorf("/app/ CSP %q differs from /desktop/ %q", got, want)
	}
	if got := session.Header.Get("Content-Security-Policy"); got != want {
		t.Errorf("/sessions/{id} CSP %q differs from /desktop/ %q", got, want)
	}
	if strings.Contains(want, "media-src") {
		t.Errorf("/desktop/ CSP names media-src: %q", want)
	}
}

// TestDesktopRoutes_RedirectNoSlash: the no-slash URL redirects with the query
// kept byte for byte, so a token with an encoded character survives.
func TestDesktopRoutes_RedirectNoSlash(t *testing.T) {
	e := newDesktopEnv(t)
	resp, _ := e.get(t, "/desktop?cap=a%2Bb")
	if resp.StatusCode != http.StatusMovedPermanently {
		t.Fatalf("status %d, want 301", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/desktop/?cap=a%2Bb" {
		t.Errorf("Location %q, want /desktop/?cap=a%%2Bb", loc)
	}
}

// TestDesktopRoutes_ManifestAndCaching: the manifest has no start_url and no id,
// so a Home Screen launch opens the exact saved link with its token; only the
// entry document is no-store, hashed assets carry no Cache-Control override.
func TestDesktopRoutes_ManifestAndCaching(t *testing.T) {
	e := newDesktopEnv(t)
	good := desktopToken(t, "g1")

	resp, body := e.get(t, "/desktop/manifest.json")
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
		t.Fatalf("manifest: %d %q, want 200 JSON", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	if m["scope"] != "./" || m["display"] != "standalone" {
		t.Errorf("manifest scope %v display %v, want ./ and standalone", m["scope"], m["display"])
	}
	for _, k := range []string{"start_url", "id"} {
		if _, ok := m[k]; ok {
			t.Errorf("manifest has %q: a saved Home Screen link would lose its token", k)
		}
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-cache" {
		t.Errorf("manifest Cache-Control %q, want no-cache", got)
	}

	if resp, _ := e.get(t, "/desktop/assets/index-abc.js"); resp.StatusCode != http.StatusOK || resp.Header.Get("Cache-Control") != "" {
		t.Errorf("hashed asset: %d Cache-Control %q, want 200 and none", resp.StatusCode, resp.Header.Get("Cache-Control"))
	}

	resp, _ = e.get(t, "/desktop/?cap="+good)
	if resp.Header.Get("Cache-Control") != "no-store" || resp.Header.Get("Referrer-Policy") != "no-referrer" {
		t.Errorf("entry: Cache-Control %q Referrer-Policy %q, want no-store and no-referrer",
			resp.Header.Get("Cache-Control"), resp.Header.Get("Referrer-Policy"))
	}

	// A directory URL is the entry document, never a listing.
	resp, body = e.get(t, "/desktop/assets/?cap="+good)
	if resp.StatusCode != http.StatusOK || string(body) != fakeIndexHTML {
		t.Errorf("/desktop/assets/: %d %q, want the index", resp.StatusCode, body)
	}
	if resp, body = e.get(t, "/desktop/assets/"); strings.Contains(string(body), "index-abc.js") || resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("/desktop/assets/ without a token: %d, want 401 and no listing", resp.StatusCode)
	}
}
