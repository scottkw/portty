package webserver

import (
	"bytes"
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"tailscale.com/client/tailscale/apitype"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/tailcfg"

	"github.com/scottkw/portty/internal/capability"
	"github.com/scottkw/portty/internal/webserver/webservertest"
)

const guestShareID = "0123456789abcdef0123456789abcdef"

// fakeGuestBackend is a fakeDesktopBackend that also hosts a guest share: it
// records the identity each guest socket arrives with, then closes normally.
type fakeGuestBackend struct {
	fakeDesktopBackend
	share    string
	inactive atomic.Bool
	whoCh    chan PeerIdentity
}

func newFakeGuestBackend() *fakeGuestBackend {
	return &fakeGuestBackend{
		fakeDesktopBackend: fakeDesktopBackend{grant: "g1"},
		share:              guestShareID,
		whoCh:              make(chan PeerIdentity, 8),
	}
}

func (b *fakeGuestBackend) GuestShareID() (string, bool) { return b.share, !b.inactive.Load() }

func (b *fakeGuestBackend) ServeGuestViewer(_ context.Context, c *websocket.Conn, _ func() bool, who PeerIdentity) {
	b.whoCh <- who
	_ = c.Close(websocket.StatusNormalClosure, "")
}

// colleagueWhoIs answers every loopback caller as a second person on the same
// tailnet (user 8, with a profile), on a Mac whose own user is 42.
func colleagueWhoIs() *fakeWhoIs {
	return &fakeWhoIs{
		self: &ipnstate.Status{Self: &ipnstate.PeerStatus{UserID: testSelfUser}},
		byAddr: map[string]*apitype.WhoIsResponse{"127.0.0.1": {
			Node: &tailcfg.Node{User: 8, ComputedName: "pat-laptop", StableID: "nPAT1CNTRL"},
			UserProfile: &tailcfg.UserProfile{
				ID: 8, LoginName: "colleague@example.com", DisplayName: "Pat Colleague",
			},
		}},
	}
}

// guestEnv is a desktopEnv whose backend also hosts a guest share, answering
// every request as the colleague.
type guestEnv struct {
	*desktopEnv
	gb *fakeGuestBackend
}

func newGuestEnv(t *testing.T) *guestEnv {
	t.Helper()
	e := newDesktopEnv(t)
	gb := newFakeGuestBackend()
	e.ws.SetDesktopBackend(gb)
	e.ws.SetWhoIsClientForTest(colleagueWhoIs())
	return &guestEnv{desktopEnv: e, gb: gb}
}

func guestToken(t *testing.T, shareID string) string {
	t.Helper()
	tok, err := capability.SignDesktopGuest(capability.DesktopGuestClaims{ShareID: shareID, IAT: time.Now().Unix()}, capTestKey)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

// TestGuestRoutes_TracerWS: a person of another account reaches the guest
// socket with their WhoIs identity bound to the connection, is refused on the
// always-available socket, and a share that is not active kills the link
// before any upgrade.
func TestGuestRoutes_TracerWS(t *testing.T) {
	e := newGuestEnv(t)
	tok := guestToken(t, guestShareID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	url := "wss" + e.ws.BaseURL()[len("https"):] + "/desktop/guest/ws?cap=" + tok
	conn, resp, err := websocket.Dial(ctx, url, &websocket.DialOptions{
		HTTPClient: e.client,
		HTTPHeader: http.Header{"Origin": {e.ws.BaseURL()}},
	})
	if err != nil {
		t.Fatalf("guest dial: %v (resp %v)", err, resp)
	}
	t.Cleanup(func() { _ = conn.CloseNow() })
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("guest dial status %d, want 101", resp.StatusCode)
	}
	select {
	case who := <-e.gb.whoCh:
		want := PeerIdentity{
			UserID: 8, LoginName: "colleague@example.com", DisplayName: "Pat Colleague",
			NodeName: "pat-laptop", StableID: "nPAT1CNTRL", IP: "127.0.0.1",
		}
		if who != want {
			t.Errorf("backend identity %+v, want %+v", who, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("backend never saw the guest")
	}

	// The same person is another account on every other family.
	r := e.upgrade(t, "/desktop/ws?cap="+desktopToken(t, "g1"))
	if r.StatusCode != http.StatusForbidden || r.Header.Get("X-Portty") != "1" || r.Header.Get("Upgrade") != "" {
		t.Errorf("colleague on /desktop/ws: %d marker %q, want 403 with the marker", r.StatusCode, r.Header.Get("X-Portty"))
	}

	// No active share: 410 before any upgrade, and the backend sees nobody new.
	e.gb.inactive.Store(true)
	r = e.upgrade(t, "/desktop/guest/ws?cap="+tok)
	if r.StatusCode != http.StatusGone || r.Header.Get("Upgrade") != "" {
		t.Errorf("guest ws with no share: %d, want 410 and no upgrade", r.StatusCode)
	}
	select {
	case who := <-e.gb.whoCh:
		t.Errorf("backend saw a second guest %+v from a refused request", who)
	default:
	}
}

// setIdentity switches who every request appears to be.
func (e *guestEnv) setIdentity(id webservertest.Identity) {
	e.ws.SetWhoIsClientForTest(webservertest.New(id))
}

func TestGuestRoutes_Page(t *testing.T) {
	e := newGuestEnv(t)
	good := guestToken(t, guestShareID)

	resp, body := e.get(t, "/desktop/guest/?cap="+good)
	if resp.StatusCode != http.StatusOK || string(body) != fakeIndexHTML {
		t.Fatalf("live token: %d %q, want 200 and the index", resp.StatusCode, body)
	}
	if resp.Header.Get("Cache-Control") != "no-store" || resp.Header.Get("Referrer-Policy") != "no-referrer" {
		t.Errorf("entry: Cache-Control %q Referrer-Policy %q, want no-store and no-referrer",
			resp.Header.Get("Cache-Control"), resp.Header.Get("Referrer-Policy"))
	}
	// The CSP is exactly the policy of /app/: the guest family adds no directive.
	e.setIdentity(webservertest.Same)
	app, _ := e.get(t, "/app/")
	e.setIdentity(webservertest.Colleague)
	want := app.Header.Get("Content-Security-Policy")
	if want == "" || resp.Header.Get("Content-Security-Policy") != want {
		t.Errorf("guest CSP %q, want the /app/ policy %q", resp.Header.Get("Content-Security-Policy"), want)
	}

	// Each refused token gets the static link-invalid page: 401, no marker.
	wrongKey, err := capability.SignDesktopGuest(capability.DesktopGuestClaims{ShareID: guestShareID}, []byte("another-signing-key-of-32-bytes!!"))
	if err != nil {
		t.Fatal(err)
	}
	for name, tok := range map[string]string{
		"missing":            "",
		"malformed":          "not-a-token",
		"wrong key":          wrongKey,
		"always desktop":     desktopToken(t, "g1"),
		"session capability": sessionCapToken(t, "s1"),
		"older share":        guestToken(t, "ffffffffffffffffffffffffffffffff"),
	} {
		resp, body := e.get(t, "/desktop/guest/?cap="+tok)
		if resp.StatusCode != http.StatusUnauthorized || resp.Header.Get("X-Portty-Desktop") != "token" {
			t.Errorf("%s: %d header %q, want 401 token", name, resp.StatusCode, resp.Header.Get("X-Portty-Desktop"))
		}
		if !bytes.Equal(body, desktopGuestLinkInvalidPage) {
			t.Errorf("%s: body is not the guest link-invalid page", name)
		}
		if resp.Header.Get("X-Portty") != "" {
			t.Errorf("%s: carries the X-Portty marker; only an identity refusal may", name)
		}
	}

	// A token minted before Regenerate Signing Key dies with the old key.
	e.ws.SetSigningKey([]byte("regenerated-signing-key-32-bytes!"))
	if resp, _ := e.get(t, "/desktop/guest/?cap="+good); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("pre-regenerate token: %d, want 401", resp.StatusCode)
	}
	e.ws.SetSigningKey(capTestKey)

	// No active share: 410, the same page, no marker.
	e.gb.inactive.Store(true)
	resp, body = e.get(t, "/desktop/guest/?cap="+good)
	if resp.StatusCode != http.StatusGone || resp.Header.Get("X-Portty-Desktop") != "off" || !bytes.Equal(body, desktopGuestLinkInvalidPage) || resp.Header.Get("X-Portty") != "" {
		t.Errorf("stopped share: %d header %q, want 410 off and the link-invalid page", resp.StatusCode, resp.Header.Get("X-Portty-Desktop"))
	}
	e.gb.inactive.Store(false)

	// A refused identity gets the guest refusal page, before any token work.
	e.setIdentity(webservertest.Tagged)
	resp, body = e.get(t, "/desktop/guest/?cap="+good)
	if resp.StatusCode != http.StatusForbidden || resp.Header.Get("X-Portty") != "1" || !bytes.Equal(body, desktopGuestRefusedPage) {
		t.Errorf("tagged device: %d marker %q, want 403 with the marker and the refused page", resp.StatusCode, resp.Header.Get("X-Portty"))
	}
	e.setIdentity(webservertest.StatusFault)
	resp, body = e.get(t, "/desktop/guest/?cap="+good)
	if resp.StatusCode != http.StatusServiceUnavailable || resp.Header.Get("X-Portty") != "" || !bytes.Equal(body, serviceRetryPage) {
		t.Errorf("tailscaled fault: %d, want 503 with the retry page and no marker", resp.StatusCode)
	}
}

func TestGuestRoutes_Assets(t *testing.T) {
	e := newGuestEnv(t)
	good := guestToken(t, guestShareID)

	// Hashed files need no token and carry no Cache-Control.
	resp, _ := e.get(t, "/desktop/guest/assets/index-abc.js")
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Cache-Control") != "" {
		t.Errorf("hashed asset: %d Cache-Control %q, want 200 and none", resp.StatusCode, resp.Header.Get("Cache-Control"))
	}
	// Other files revalidate.
	if resp, _ := e.get(t, "/desktop/guest/manifest.json"); resp.StatusCode != http.StatusOK || resp.Header.Get("Cache-Control") != "no-cache" {
		t.Errorf("manifest: %d Cache-Control %q, want 200 and no-cache", resp.StatusCode, resp.Header.Get("Cache-Control"))
	}
	// A non-asset path falls back to the index, with the token check.
	resp, body := e.get(t, "/desktop/guest/some/route?cap="+good)
	if resp.StatusCode != http.StatusOK || string(body) != fakeIndexHTML {
		t.Errorf("SPA fallback with a token: %d, want the index", resp.StatusCode)
	}
	if resp, _ := e.get(t, "/desktop/guest/some/route"); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("SPA fallback without a token: %d, want 401", resp.StatusCode)
	}
	// A directory URL is the index with the token check, never a listing.
	if resp, body := e.get(t, "/desktop/guest/assets/"); resp.StatusCode != http.StatusUnauthorized || strings.Contains(string(body), "index-abc.js") {
		t.Errorf("/desktop/guest/assets/ without a token: %d, want 401 and no listing", resp.StatusCode)
	}
	// The bundle is behind the identity gate too: a refused device gets the refusal
	// page, not the file.
	for _, id := range []webservertest.Identity{webservertest.Tagged, webservertest.SharedIn, webservertest.OtherNilProfile} {
		e.setIdentity(id)
		resp, body := e.get(t, "/desktop/guest/assets/index-abc.js")
		if resp.StatusCode != http.StatusForbidden || resp.Header.Get("X-Portty") != "1" || !bytes.Equal(body, desktopGuestRefusedPage) {
			t.Errorf("identity %d: %d, want 403 with the refused page", id, resp.StatusCode)
		}
	}
}

func TestGuestRoutes_Redirect(t *testing.T) {
	e := newGuestEnv(t)
	resp, _ := e.get(t, "/desktop/guest?cap=a%2Bb")
	if resp.StatusCode != http.StatusMovedPermanently {
		t.Fatalf("status %d, want 301", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/desktop/guest/?cap=a%2Bb" {
		t.Errorf("Location %q, want /desktop/guest/?cap=a%%2Bb", loc)
	}
}

func TestGuestRoutes_Check(t *testing.T) {
	e := newGuestEnv(t)
	good := guestToken(t, guestShareID)
	check := func(tok string, status int, reason string) {
		t.Helper()
		resp, body := e.get(t, "/desktop/guest/check?cap="+tok)
		if resp.StatusCode != status || resp.Header.Get("X-Portty-Desktop") != reason {
			t.Errorf("check: %d header %q, want %d %q", resp.StatusCode, resp.Header.Get("X-Portty-Desktop"), status, reason)
		}
		if resp.Header.Get("X-Portty") != "" {
			t.Errorf("check: %d carries the marker", resp.StatusCode)
		}
		if reason == "" && len(body) != 0 {
			t.Errorf("check: 204 has a body %q", body)
		}
	}
	check(good, http.StatusNoContent, "")
	check("garbage", http.StatusUnauthorized, "token")
	check(desktopToken(t, "g1"), http.StatusUnauthorized, "token")
	e.gb.inactive.Store(true)
	check(good, http.StatusGone, "off")
	e.gb.inactive.Store(false)

	e.setIdentity(webservertest.Tagged)
	resp, _ := e.get(t, "/desktop/guest/check?cap="+good)
	if resp.StatusCode != http.StatusForbidden || resp.Header.Get("X-Portty") != "1" {
		t.Errorf("tagged on check: %d marker %q, want 403 with the marker", resp.StatusCode, resp.Header.Get("X-Portty"))
	}
}

// TestGuestRoutes_OtherAccountRefusedElsewhere: a person of another Tailscale
// account passes the identity gate on the guest family and gets the own-devices
// refusal (403 with the marker) on every other family, even holding the token
// that family wants (DSHR-03, MOB-05).
func TestGuestRoutes_OtherAccountRefusedElsewhere(t *testing.T) {
	for _, id := range []webservertest.Identity{webservertest.Other, webservertest.Colleague} {
		e := newGuestEnv(t)
		e.setIdentity(id)
		sess := "/sessions/" + ppSID
		gets := []string{
			"/app/",
			sess + "?cap=" + e.pp.capTok,
			"/api/sessions?cap=" + e.pp.capTok,
			"/api/sessions/meta",
			"/api/sessions/" + ppSID + "/info?cap=" + e.pp.capTok,
			"/api/plugin-config?cap=" + e.pp.capTok,
			"/desktop/?cap=" + desktopToken(t, "g1"),
			"/desktop/check?cap=" + desktopToken(t, "g1"),
		}
		for _, path := range gets {
			resp, _ := e.get(t, path)
			if resp.StatusCode != http.StatusForbidden || resp.Header.Get("X-Portty") != "1" {
				t.Errorf("identity %d GET %s: %d marker %q, want 403 with the marker", id, path, resp.StatusCode, resp.Header.Get("X-Portty"))
			}
		}
		for _, path := range []string{sess + "/ws?cap=" + e.pp.capTok, "/desktop/ws?cap=" + desktopToken(t, "g1")} {
			resp := e.upgrade(t, path)
			if resp.StatusCode != http.StatusForbidden || resp.Header.Get("X-Portty") != "1" || resp.Header.Get("Upgrade") != "" {
				t.Errorf("identity %d upgrade %s: %d, want 403 with the marker", id, path, resp.StatusCode)
			}
		}
		// A port route (grant) is POST-only with its own capability; the gate is first.
		req, err := http.NewRequest(http.MethodPost, e.ws.BaseURL()+"/api/peer/sessions/"+ppSID+"/ports", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Portty-Cap", e.pp.capTok)
		resp, err := e.client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden || resp.Header.Get("X-Portty") != "1" {
			t.Errorf("identity %d port grant: %d marker %q, want 403 with the marker", id, resp.StatusCode, resp.Header.Get("X-Portty"))
		}

		// And the same identity is admitted by the guest family.
		tok := guestToken(t, guestShareID)
		if resp, _ := e.get(t, "/desktop/guest/?cap="+tok); resp.StatusCode != http.StatusOK {
			t.Errorf("identity %d guest page: %d, want 200", id, resp.StatusCode)
		}
		if resp, _ := e.get(t, "/desktop/guest/check?cap="+tok); resp.StatusCode != http.StatusNoContent {
			t.Errorf("identity %d guest check: %d, want 204", id, resp.StatusCode)
		}
		if resp := e.upgrade(t, "/desktop/guest/ws?cap="+tok); resp.StatusCode != http.StatusSwitchingProtocols {
			t.Errorf("identity %d guest socket: %d, want 101", id, resp.StatusCode)
		}
	}
}

// TestGuestRouteTable_AST pins the narrowness of the widening by reading the
// source: requireTailnetPeer(Page) is called only inside setupDesktopGuestRoutes,
// and no registered share-server route is an admit, deny, grant, rights or stop
// endpoint (admitting is a prompt on the shared Mac, never a remote call).
func TestGuestRouteTable_AST(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var patterns []string
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		// Walk each top-level declaration so a call knows its enclosing function
		// ("" for a package-level initialiser).
		for _, decl := range file.Decls {
			enclosing := ""
			if fd, ok := decl.(*ast.FuncDecl); ok {
				enclosing = fd.Name.Name
			}
			ast.Inspect(decl, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				var name string
				switch fn := call.Fun.(type) {
				case *ast.SelectorExpr:
					name = fn.Sel.Name
				case *ast.Ident:
					name = fn.Name
				}
				switch name {
				case "requireTailnetPeer", "requireTailnetPeerPage":
					if enclosing != "setupDesktopGuestRoutes" {
						t.Errorf("%s: %s called in %q, only setupDesktopGuestRoutes may widen the gate",
							fset.Position(call.Pos()), name, enclosing)
					}
				case "HandleFunc", "Handle":
					if len(call.Args) == 0 {
						return true
					}
					// The first argument may be a literal or a concatenation of literals.
					ast.Inspect(call.Args[0], func(m ast.Node) bool {
						if lit, ok := m.(*ast.BasicLit); ok && lit.Kind == token.STRING {
							if s, err := strconv.Unquote(lit.Value); err == nil {
								patterns = append(patterns, s)
							}
						}
						return true
					})
				}
				return true
			})
		}
	}
	// An empty or truncated walk must not pass: the share server registers well
	// over this many patterns.
	if len(patterns) < 20 {
		t.Fatalf("walk saw %d route patterns, want at least 20", len(patterns))
	}
	for _, p := range patterns {
		lp := strings.ToLower(p)
		for _, bad := range []string{"admit", "deny", "grant", "rights", "stop"} {
			if strings.Contains(lp, bad) {
				t.Errorf("route pattern %q contains %q: there is no remote admit or grant API", p, bad)
			}
		}
	}
}
