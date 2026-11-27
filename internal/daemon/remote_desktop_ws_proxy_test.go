package daemon

// Phase 21-06 — the loopback desktop proxy against a fake TLS peer: Origin and
// token handling, refusal mapping, the single token refresh, long-lived streams
// and redaction. The real-share-server end to end is TestPeerDesktop_Tracer.

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// fakeDesktopPeer is a TLS peer with a desktop cap route and a /desktop/ws
// whose behaviour each test sets. It records every upstream handshake.
type fakeDesktopPeer struct {
	srv *httptest.Server

	mu        sync.Mutex
	capPosts  int
	freshCap  string           // handed out by the cap route
	capStatus int              // the cap route answers this instead of 200 when set
	seen      []http.Header    // headers of each /desktop/ws request
	queries   []string         // raw query of each /desktop/ws request
	ws        http.HandlerFunc // /desktop/ws behaviour
}

func newFakeDesktopPeer(t *testing.T, ws http.HandlerFunc) *fakeDesktopPeer {
	t.Helper()
	p := &fakeDesktopPeer{freshCap: "tok-fresh", ws: ws}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/peer/desktop/cap", func(w http.ResponseWriter, _ *http.Request) {
		p.mu.Lock()
		p.capPosts++
		c, st := p.freshCap, p.capStatus
		p.mu.Unlock()
		if st != 0 {
			w.WriteHeader(st)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"cap": c})
	})
	mux.HandleFunc("GET /desktop/ws", func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		p.seen = append(p.seen, r.Header.Clone())
		p.queries = append(p.queries, r.URL.RawQuery)
		h := p.ws
		p.mu.Unlock()
		h(w, r)
	})
	p.srv = httptest.NewTLSServer(mux)
	t.Cleanup(p.srv.Close)
	return p
}

func (p *fakeDesktopPeer) capPostCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.capPosts
}

func (p *fakeDesktopPeer) requests() (headers []http.Header, queries []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]http.Header(nil), p.seen...), append([]string(nil), p.queries...)
}

// echoWS accepts and echoes frames until the client goes away.
func echoWS(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"*"}})
	if err != nil {
		return
	}
	defer func() { _ = c.CloseNow() }()
	for {
		typ, b, err := c.Read(r.Context())
		if err != nil {
			return
		}
		if c.Write(r.Context(), typ, b) != nil {
			return
		}
	}
}

// refuseWS answers the handshake with status (and the X-Portty-Desktop reason
// and X-Portty marker when set).
func refuseWS(status int, reason string, marker bool) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if reason != "" {
			w.Header().Set("X-Portty-Desktop", reason)
		}
		if marker {
			w.Header().Set("X-Portty", "1")
		}
		w.WriteHeader(status)
	}
}

// tokenWS accepts only the want token and answers 401 token to any other.
func tokenWS(want string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("cap") != want {
			refuseWS(http.StatusUnauthorized, "token", false)(w, r)
			return
		}
		echoWS(w, r)
	}
}

// desktopProxyEnv is a viewing daemon holding oldTok for the fake peer, with
// its relay proxy served.
type desktopProxyEnv struct {
	api   *API
	relay *httptest.Server
	peer  *fakeDesktopPeer
}

func newDesktopProxyEnv(t *testing.T, peer *fakeDesktopPeer, oldTok string) *desktopProxyEnv {
	t.Helper()
	api := newViewerFor(t, peer.srv.URL)
	if err := api.desktopCaps.Put(officeFQDN, peer.srv.URL, "", oldTok); err != nil {
		t.Fatal(err)
	}
	relaySrv := httptest.NewServer(api.wrapRelayWithRemoteFiles(http.NotFoundHandler()))
	t.Cleanup(relaySrv.Close)
	return &desktopProxyEnv{api: api, relay: relaySrv, peer: peer}
}

func (e *desktopProxyEnv) dial(ctx context.Context, t *testing.T) *websocket.Conn {
	t.Helper()
	c, _, err := dialDesktopProxy(ctx, e.relay.URL, officeFQDN)
	if err != nil {
		t.Fatalf("dial desktop proxy: %v", err)
	}
	t.Cleanup(func() { _ = c.CloseNow() })
	return c
}

// wantClose reads until the proxy closes the client socket and returns its code
// and reason.
func wantClose(ctx context.Context, t *testing.T, c *websocket.Conn) (websocket.StatusCode, string) {
	t.Helper()
	_, _, err := c.Read(ctx)
	code := websocket.CloseStatus(err)
	if code == -1 {
		t.Fatalf("read ended without a close frame: %v", err)
	}
	var ce websocket.CloseError
	if !errors.As(err, &ce) {
		t.Fatalf("not a CloseError: %v", err)
	}
	return code, ce.Reason
}

func wantEcho(ctx context.Context, t *testing.T, c *websocket.Conn, msg string) {
	t.Helper()
	if err := c.Write(ctx, websocket.MessageText, []byte(msg)); err != nil {
		t.Fatalf("write %q: %v", msg, err)
	}
	_, got, err := c.Read(ctx)
	if err != nil || string(got) != msg {
		t.Fatalf("echo = %q, %v; want %q", got, err, msg)
	}
}

type desktopLogBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (d *desktopLogBuf) Write(p []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.b.Write(p)
}

func (d *desktopLogBuf) String() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.b.String()
}

func captureDesktopLog(t *testing.T) *desktopLogBuf {
	t.Helper()
	d := &desktopLogBuf{}
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(d)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(prevOut); log.SetFlags(prevFlags) })
	return d
}

func testCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestRemoteDesktopWS_InjectsOriginAndOnlyCap(t *testing.T) {
	peer := newFakeDesktopPeer(t, echoWS)
	env := newDesktopProxyEnv(t, peer, "tok-old")
	ctx := testCtx(t)

	// A cap the web view puts on its own URL must not reach the peer.
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(env.relay.URL, "http")+"/api/relay/remote-desktop/"+officeFQDN+"/ws?cap=from-client&x=1", &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": {"wails://wails"}},
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = c.CloseNow() }()
	wantEcho(ctx, t, c, "hi")

	hdrs, queries := peer.requests()
	if len(hdrs) != 1 {
		t.Fatalf("peer saw %d handshakes; want 1", len(hdrs))
	}
	if got := hdrs[0].Get("Origin"); got != peer.srv.URL {
		t.Fatalf("upstream Origin = %q; want %q", got, peer.srv.URL)
	}
	if queries[0] != "cap=tok-old" {
		t.Fatalf("upstream query = %q; want only the stored cap", queries[0])
	}
}

func TestRemoteDesktopWS_RejectsCrossSiteOrigin(t *testing.T) {
	peer := newFakeDesktopPeer(t, echoWS)
	env := newDesktopProxyEnv(t, peer, "tok-old")
	ctx := testCtx(t)

	_, resp, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(env.relay.URL, "http")+"/api/relay/remote-desktop/"+officeFQDN+"/ws", &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": {"https://evil.example"}},
	})
	if err == nil {
		t.Fatal("a cross-site origin was accepted")
	}
	if resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("response = %v; want 403", resp)
	}
	if hdrs, _ := peer.requests(); len(hdrs) != 0 {
		t.Fatalf("peer was dialled %d times for a refused origin", len(hdrs))
	}
}

func TestRemoteDesktopWS_UnknownHostNeverDials(t *testing.T) {
	peer := newFakeDesktopPeer(t, echoWS)
	env := newDesktopProxyEnv(t, peer, "tok-old")
	ctx := testCtx(t)

	_, resp, err := dialDesktopProxy(ctx, env.relay.URL, "other-air.example")
	if err == nil || resp == nil || resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown host: err=%v resp=%v; want a 404", err, resp)
	}
	if hdrs, _ := peer.requests(); len(hdrs) != 0 {
		t.Fatalf("peer was dialled for an unheld host")
	}
}

func TestRemoteDesktopWS_RefreshesStaleTokenOnce(t *testing.T) {
	peer := newFakeDesktopPeer(t, tokenWS("tok-fresh"))
	env := newDesktopProxyEnv(t, peer, "tok-stale")
	ctx := testCtx(t)

	wantEcho(ctx, t, env.dial(ctx, t), "after-refresh")
	if n := peer.capPostCount(); n != 1 {
		t.Fatalf("cap route saw %d POSTs; want exactly 1", n)
	}
	_, queries := peer.requests()
	if len(queries) != 2 || queries[0] != "cap=tok-stale" || queries[1] != "cap=tok-fresh" {
		t.Fatalf("upstream queries = %v; want the stale token then the fresh one", queries)
	}
}

func TestRemoteDesktopWS_TokenRefusedTwiceCloses4401(t *testing.T) {
	peer := newFakeDesktopPeer(t, refuseWS(http.StatusUnauthorized, "token", false))
	env := newDesktopProxyEnv(t, peer, "tok-stale")
	ctx := testCtx(t)

	code, reason := wantClose(ctx, t, env.dial(ctx, t))
	if code != closeDesktopToken || reason != "token" {
		t.Fatalf("close = %d %q; want 4401 token", code, reason)
	}
	if n := peer.capPostCount(); n != 1 {
		t.Fatalf("cap route saw %d POSTs; want exactly 1", n)
	}
}

func TestRemoteDesktopWS_RefreshFailureMapsToFixedCloses(t *testing.T) {
	cases := []struct {
		name      string
		capStatus int
		code      websocket.StatusCode
		reason    string
	}{
		{"off", http.StatusNotFound, closeDesktopOff, "off"},
		{"forbidden", http.StatusForbidden, closeDesktopForbidden, "forbidden"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			peer := newFakeDesktopPeer(t, refuseWS(http.StatusUnauthorized, "token", false))
			peer.mu.Lock()
			peer.capStatus = tc.capStatus
			peer.mu.Unlock()
			env := newDesktopProxyEnv(t, peer, "tok-stale")
			ctx := testCtx(t)
			code, reason := wantClose(ctx, t, env.dial(ctx, t))
			if code != tc.code || reason != tc.reason {
				t.Fatalf("close = %d %q; want %d %q", code, reason, tc.code, tc.reason)
			}
			if n := peer.capPostCount(); n != 1 {
				t.Fatalf("cap route saw %d POSTs; want exactly 1", n)
			}
		})
	}
}

func TestRemoteDesktopWS_RefusalsMapToFixedCloses(t *testing.T) {
	cases := []struct {
		name   string
		ws     http.HandlerFunc
		code   websocket.StatusCode
		reason string
	}{
		{"off", refuseWS(http.StatusGone, "off", false), closeDesktopOff, "off"},
		{"forbidden", refuseWS(http.StatusForbidden, "", true), closeDesktopForbidden, "forbidden"},
		{"forbidden without marker", refuseWS(http.StatusForbidden, "", false), websocket.StatusTryAgainLater, "remote unreachable"},
		{"rate limited", refuseWS(http.StatusTooManyRequests, "", false), websocket.StatusTryAgainLater, "remote unreachable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newDesktopProxyEnv(t, newFakeDesktopPeer(t, tc.ws), "tok-old")
			ctx := testCtx(t)
			code, reason := wantClose(ctx, t, env.dial(ctx, t))
			if code != tc.code || reason != tc.reason {
				t.Fatalf("close = %d %q; want %d %q", code, reason, tc.code, tc.reason)
			}
			if n := env.peer.capPostCount(); n != 0 {
				t.Fatalf("cap route saw %d POSTs; only a 401 token refreshes", n)
			}
		})
	}
}

func TestRemoteDesktopWS_ConnectionRefusedCloses1013(t *testing.T) {
	peer := newFakeDesktopPeer(t, echoWS)
	env := newDesktopProxyEnv(t, peer, "tok-old")
	peer.srv.Close()
	ctx := testCtx(t)

	code, reason := wantClose(ctx, t, env.dial(ctx, t))
	if code != websocket.StatusTryAgainLater || reason != "remote unreachable" {
		t.Fatalf("close = %d %q; want 1013 remote unreachable", code, reason)
	}
}

func TestRemoteDesktopWS_ForwardsPeerClose(t *testing.T) {
	peer := newFakeDesktopPeer(t, func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"*"}})
		if err != nil {
			return
		}
		_ = c.Close(closeDesktopOff, "off")
	})
	env := newDesktopProxyEnv(t, peer, "tok-old")
	ctx := testCtx(t)

	code, reason := wantClose(ctx, t, env.dial(ctx, t))
	if code != closeDesktopOff || reason != "off" {
		t.Fatalf("close = %d %q; want the peer's 4410 off", code, reason)
	}
}

// TestRemoteDesktopWS_LongLived: the copy loops use the request context, so a
// stream held past the 10 s dial timeout keeps working.
func TestRemoteDesktopWS_LongLived(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-lived WS test in -short mode")
	}
	env := newDesktopProxyEnv(t, newFakeDesktopPeer(t, echoWS), "tok-old")
	env.api.SetHeartbeatForTest(300*time.Millisecond, 3*time.Second)
	ctx := testCtx(t)

	c := env.dial(ctx, t)
	wantEcho(ctx, t, c, "first")
	time.Sleep(11 * time.Second)
	wantEcho(ctx, t, c, "after-deadline")
}

// TestRemoteDesktopWS_NeverLogsTheToken drives the refusal, refresh and dead
// peer paths and checks neither the log nor a close reason holds a token or its
// query.
func TestRemoteDesktopWS_NeverLogsTheToken(t *testing.T) {
	buf := captureDesktopLog(t)
	const stale = "tok-secret-stale"
	peer := newFakeDesktopPeer(t, refuseWS(http.StatusUnauthorized, "token", false))
	peer.freshCap = "tok-secret-fresh"
	env := newDesktopProxyEnv(t, peer, stale)
	ctx := testCtx(t)

	_, reason := wantClose(ctx, t, env.dial(ctx, t))
	peer.srv.Close()
	_, reason2 := wantClose(ctx, t, env.dial(ctx, t))

	time.Sleep(200 * time.Millisecond) // let the proxy's end lines land
	logs := buf.String()
	for _, secret := range []string{stale, "tok-secret-fresh", "cap="} {
		if strings.Contains(logs, secret) || strings.Contains(reason, secret) || strings.Contains(reason2, secret) {
			t.Fatalf("a token or query leaked (%q):\nlogs: %s\nreasons: %q %q", secret, logs, reason, reason2)
		}
	}
	if !strings.Contains(logs, "viewer-diag: desktop proxy") {
		t.Fatalf("no proxy log lines were captured:\n%s", logs)
	}
}

// TestRemoteDesktopWS_StoresDoNotCross: a session cap never answers a desktop
// lookup and the reverse.
func TestRemoteDesktopWS_StoresDoNotCross(t *testing.T) {
	a := newViewerFor(t, "https://127.0.0.1:1")
	if err := a.remoteCaps.Put(officeFQDN, "https://127.0.0.1:1", "", "session-tok"); err != nil {
		t.Fatal(err)
	}
	if _, _, _, ok := a.desktopCaps.Get(officeFQDN); ok {
		t.Fatal("DesktopCapStore answered for a session cap")
	}
	if err := a.desktopCaps.Put("host-b.example", "https://127.0.0.1:1", "", "desktop-tok"); err != nil {
		t.Fatal(err)
	}
	if _, _, _, ok := a.remoteCaps.Get("host-b.example"); ok {
		t.Fatal("RemoteCapStore answered for a desktop token")
	}
}
