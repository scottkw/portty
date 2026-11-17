// Phase 134-06 — Cap-gated remote terminal WebSocket reverse-proxy tests.
//
// These integration tests drive the daemon RELAY surface (api.RelayHandler())
// exactly as the Wails webview does: the webview opens
// ws://127.0.0.1:<relayPort>/api/relay/remote/{sid}/ws WITHOUT a cap, and the
// daemon proxy looks up the Phase 122 cap, dials the peer's already-cap-gated
// wss://<baseURL>/sessions/{sid}/ws?cap=T (injecting the Origin the peer
// requires), and copies opaque WS frames both ways.
//
// They use a fixture TLS peer + newDaemonAPIWithUpstreamCert +
// depositCap (defined below); the fixture peer here serves a cap-guarded WS echo endpoint at /sessions/{id}/ws that asserts the
// ?cap= query param AND a non-empty Origin equal to its own base URL.
//
// Lives in package daemon_test.

package daemon_test

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/scottkw/portty/internal/daemon"
	"github.com/scottkw/portty/internal/relay"
	"github.com/scottkw/portty/internal/testutil"
)

// fixtureCap is the canonical cap token the fixture peer accepts.
const fixtureCap = "FIXTURE_CAP"

// newDaemonAPIWithUpstreamCert constructs a daemon API with a tempdir configDir
// and injects the upstream's self-signed cert via SetPeerClientForTest
// so the proxy's outbound HTTPS/WSS dials succeed.
func newDaemonAPIWithUpstreamCert(t *testing.T, upstream *httptest.Server) *daemon.API {
	t.Helper()
	engine := daemon.NewSessionEngine()
	engine.ConfigDirForTest(t.TempDir())
	api := daemon.NewAPI(engine)
	api.SetPeerClientForTest(upstream.Client())
	return api
}

// depositCap stores the (sessionID, baseURL, capToken) tuple in the API's
// RemoteCapStore, as POST /peers/connect does after fetching the peer's cap.
func depositCap(t *testing.T, api *daemon.API, sessionID, baseURL, cap string) {
	t.Helper()
	if err := api.PutRemoteCapForTest(sessionID, baseURL, cap); err != nil {
		t.Fatalf("cap deposit: %v", err)
	}
}

// fixtureWSPeer is a cap-guarded WebSocket echo peer that mimics the remote
// webserver's /sessions/{id}/ws contract for the proxy tests. It records the
// Origin header and cap query param it observed on the most recent upgrade so
// tests can assert the proxy injected them correctly.
type fixtureWSPeer struct {
	srv *httptest.Server

	mu         sync.Mutex
	gotOrigin  string
	gotCap     string
	sawUpgrade bool
	gotResume  string // resume query value on the last upgrade
	hasResume  bool   // whether a resume parameter was present at all
	silent     bool   // accept, then never read (so never answer pings): a half-open peer
	done       chan struct{}
}

// newFixtureRemotePeerWithWS spins up an httptest.NewTLSServer that serves a
// cap-guarded WS echo at GET /sessions/{id}/ws. The handler:
//   - rejects an empty Origin with 403 (mirrors the peer's requireAllowedOrigin)
//   - rejects a missing/mismatched ?cap= with 401 (mirrors requireCapability)
//   - otherwise accepts the upgrade (no inbound Origin allowlist — the dialer is
//     the daemon, which injects the peer's own base URL) and echoes every frame
//     back verbatim until the client closes.
func newFixtureRemotePeerWithWS(t *testing.T) *fixtureWSPeer {
	t.Helper()

	peer := &fixtureWSPeer{done: make(chan struct{})}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /sessions/{id}/ws", func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		cap := r.URL.Query().Get("cap")

		peer.mu.Lock()
		peer.gotOrigin = origin
		peer.gotCap = cap
		peer.sawUpgrade = true
		peer.gotResume = r.URL.Query().Get("resume")
		peer.hasResume = r.URL.Query().Has("resume")
		silent := peer.silent
		peer.mu.Unlock()

		// Mirror the peer's requireAllowedOrigin: empty Origin is rejected.
		if origin == "" {
			http.Error(w, "empty origin forbidden", http.StatusForbidden)
			return
		}
		// Mirror requireCapability: the cap arrives as the ?cap= query param.
		if cap != fixtureCap {
			http.Error(w, "cap rejected", http.StatusUnauthorized)
			return
		}

		// Accept with no OriginPatterns restriction — coder/websocket requires
		// the Origin host to match the request host unless we opt out. The proxy
		// injects Origin == baseURL, which is this server's own host, so the
		// default same-host check would already pass; "*" is belt-and-suspenders.
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			OriginPatterns: []string{"*"},
		})
		if err != nil {
			return
		}
		defer conn.CloseNow()

		ctx := r.Context()
		if silent {
			select {
			case <-ctx.Done():
			case <-peer.done:
			}
			return
		}
		for {
			typ, data, rerr := conn.Read(ctx)
			if rerr != nil {
				return
			}
			if werr := conn.Write(ctx, typ, data); werr != nil {
				return
			}
		}
	})

	peer.srv = httptest.NewTLSServer(mux)
	t.Cleanup(peer.srv.Close)
	t.Cleanup(func() { close(peer.done) }) // runs first: releases silent handlers
	return peer
}

func (p *fixtureWSPeer) observed() (origin, cap string, sawUpgrade bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.gotOrigin, p.gotCap, p.sawUpgrade
}

// dialProxy opens the inbound (webview-side) WS to the daemon relay proxy at
// ws://<relayHost>/api/relay/remote/{sid}/ws with the given Origin header.
func dialProxy(ctx context.Context, t *testing.T, relayURL, sid, origin string) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	wsURL := "ws://" + strings.TrimPrefix(relayURL, "http://") + "/api/relay/remote/" + sid + "/ws"
	hdr := http.Header{}
	if origin != "" {
		hdr.Set("Origin", origin)
	}
	return websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: hdr})
}

// TestRemoteSessionWS_MountedOnRelay (WS-PROXY-01): the route is mounted on the
// relay surface and the handler runs (a successful upgrade proves it, since a
// route-miss would 404 the upgrade).
func TestRemoteSessionWS_MountedOnRelay(t *testing.T) {
	peer := newFixtureRemotePeerWithWS(t)
	api := newDaemonAPIWithUpstreamCert(t, peer.srv)

	socketSrv := httptest.NewServer(api.Handler())
	defer socketSrv.Close()
	relaySrv := httptest.NewServer(api.RelayHandler())
	defer relaySrv.Close()

	depositCap(t, api, "sid1", peer.srv.URL, fixtureCap)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := dialProxy(ctx, t, relaySrv.URL, "sid1", "wails://wails")
	if err != nil {
		t.Fatalf("dial proxy: %v (route not mounted on relay surface?)", err)
	}
	defer conn.CloseNow()

	// Round-trip one frame to guarantee the proxy completed the upstream upgrade
	// before we inspect what the peer observed (the upstream Accept runs in a
	// separate goroutine on the peer side).
	if err := conn.Write(ctx, websocket.MessageBinary, []byte("ping")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, _, err := conn.Read(ctx); err != nil {
		t.Fatalf("read: %v", err)
	}

	if _, _, saw := peer.observed(); !saw {
		t.Fatalf("upstream peer never saw the upgrade — proxy handler did not dial through")
	}
}

// TestRemoteSessionWS_NoCap (WS-PROXY-02): with no cap deposited the handler is
// still reached and returns the "no cap registered" 404 contract,
// distinguishable from a bare route-miss 404.
func TestRemoteSessionWS_NoCap(t *testing.T) {
	peer := newFixtureRemotePeerWithWS(t)
	api := newDaemonAPIWithUpstreamCert(t, peer.srv) // no cap deposited
	relaySrv := httptest.NewServer(api.RelayHandler())
	defer relaySrv.Close()

	// Use a plain HTTP GET (no upgrade) so we can read the JSON 404 body — the
	// handler writes the no-cap response before any Accept.
	resp, err := http.Get(relaySrv.URL + "/api/relay/remote/sid1/ws")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	body := make([]byte, 4096)
	n, _ := resp.Body.Read(body)
	got := string(body[:n])

	if strings.Contains(got, "404 page not found") {
		t.Fatalf("relay returned the bare route-miss 404 — WS route not mounted; body=%s", got)
	}
	if !strings.Contains(got, "no cap registered") {
		t.Errorf("no-cap request: want proxy 'no cap registered' marker (handler reached); body=%s", got)
	}
}

// TestRemoteSessionWS_FrameCopy (WS-PROXY-03): with a cap, frames written by the
// webview-side conn arrive at the peer and the peer's echo frames arrive back.
func TestRemoteSessionWS_FrameCopy(t *testing.T) {
	peer := newFixtureRemotePeerWithWS(t)
	api := newDaemonAPIWithUpstreamCert(t, peer.srv)

	socketSrv := httptest.NewServer(api.Handler())
	defer socketSrv.Close()
	relaySrv := httptest.NewServer(api.RelayHandler())
	defer relaySrv.Close()

	depositCap(t, api, "sid1", peer.srv.URL, fixtureCap)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := dialProxy(ctx, t, relaySrv.URL, "sid1", "wails://wails")
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	defer conn.CloseNow()

	payload := []byte("hello-remote-terminal")
	if err := conn.Write(ctx, websocket.MessageBinary, payload); err != nil {
		t.Fatalf("write to proxy: %v", err)
	}
	typ, got, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read echo from proxy: %v", err)
	}
	if typ != websocket.MessageBinary {
		t.Errorf("echo message type = %v; want binary", typ)
	}
	if string(got) != string(payload) {
		t.Errorf("echo payload = %q; want %q (bidirectional copy broken)", got, payload)
	}
}

// TestRemoteSessionWS_InjectsOrigin (WS-PROXY-04): the peer asserts it received
// a non-empty Origin equal to its base URL on the upstream dial, plus the cap.
func TestRemoteSessionWS_InjectsOrigin(t *testing.T) {
	peer := newFixtureRemotePeerWithWS(t)
	api := newDaemonAPIWithUpstreamCert(t, peer.srv)

	socketSrv := httptest.NewServer(api.Handler())
	defer socketSrv.Close()
	relaySrv := httptest.NewServer(api.RelayHandler())
	defer relaySrv.Close()

	depositCap(t, api, "sid1", peer.srv.URL, fixtureCap)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := dialProxy(ctx, t, relaySrv.URL, "sid1", "wails://wails")
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	defer conn.CloseNow()

	// Round-trip one frame to guarantee the upstream upgrade completed before we
	// inspect what the peer observed.
	if err := conn.Write(ctx, websocket.MessageBinary, []byte("x")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, _, err := conn.Read(ctx); err != nil {
		t.Fatalf("read: %v", err)
	}

	origin, cap, saw := peer.observed()
	if !saw {
		t.Fatalf("peer never saw the upgrade")
	}
	wantOrigin := strings.TrimRight(peer.srv.URL, "/")
	if origin == "" {
		t.Errorf("peer saw EMPTY Origin — proxy must inject Origin: <baseURL> (Pitfall 1)")
	}
	if origin != wantOrigin {
		t.Errorf("peer Origin = %q; want %q (byte-exact baseURL)", origin, wantOrigin)
	}
	if cap != fixtureCap {
		t.Errorf("peer cap = %q; want %q forwarded as ?cap=", cap, fixtureCap)
	}
}

// TestRemoteSessionWS_RejectsCrossSiteOrigin (WS-PROXY-05): a cross-site inbound
// Origin is rejected at websocket.Accept (mirror relay origin_test.go). Use raw
// HTTP so the non-101 status is observable (websocket.Dial hides it).
func TestRemoteSessionWS_RejectsCrossSiteOrigin(t *testing.T) {
	peer := newFixtureRemotePeerWithWS(t)
	api := newDaemonAPIWithUpstreamCert(t, peer.srv)

	socketSrv := httptest.NewServer(api.Handler())
	defer socketSrv.Close()
	relaySrv := httptest.NewServer(api.RelayHandler())
	defer relaySrv.Close()

	depositCap(t, api, "sid1", peer.srv.URL, fixtureCap)

	req, err := http.NewRequest("GET", relaySrv.URL+"/api/relay/remote/sid1/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("client.Do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusSwitchingProtocols {
		t.Error("proxy accepted cross-site Origin — inbound loopback allowlist not enforced (T-134-06-01)")
	}
}

// TestRemoteSessionWS_LongLived (WS-PROXY-06): the copy loop uses r.Context(),
// not the 10s dial-timeout context, so a healthy terminal survives past 10s.
// Rather than sleeping 10+ real seconds, this test asserts the connection is
// still live and echoing after a delay comfortably exceeding any dial deadline
// derivation that would have leaked into the copy loop.
func TestRemoteSessionWS_LongLived(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long-lived WS test in -short mode")
	}
	peer := newFixtureRemotePeerWithWS(t)
	api := newDaemonAPIWithUpstreamCert(t, peer.srv)

	socketSrv := httptest.NewServer(api.Handler())
	defer socketSrv.Close()
	relaySrv := httptest.NewServer(api.RelayHandler())
	defer relaySrv.Close()

	depositCap(t, api, "sid1", peer.srv.URL, fixtureCap)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	conn, _, err := dialProxy(ctx, t, relaySrv.URL, "sid1", "wails://wails")
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	defer conn.CloseNow()

	// Prime the connection.
	if err := conn.Write(ctx, websocket.MessageBinary, []byte("first")); err != nil {
		t.Fatalf("write first: %v", err)
	}
	if _, _, err := conn.Read(ctx); err != nil {
		t.Fatalf("read first: %v", err)
	}

	// Wait past the 10s dial deadline. If the copy loop wrongly used the dial
	// context, the upstream side would have torn down at ~10s and this second
	// round-trip would fail.
	time.Sleep(11 * time.Second)

	if err := conn.Write(ctx, websocket.MessageBinary, []byte("after-deadline")); err != nil {
		t.Fatalf("write after 11s: %v (copy loop used the dial deadline, not r.Context()?)", err)
	}
	typ, got, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read after 11s: %v (terminal died at the 10s dial deadline — Pitfall 4)", err)
	}
	if typ != websocket.MessageBinary || string(got) != "after-deadline" {
		t.Errorf("post-deadline echo = (%v, %q); want (binary, %q)", typ, got, "after-deadline")
	}
}

// TestRemoteWSProxy_ForwardsClose (SHAR-04): when the peer closes the session
// WebSocket with a code and reason (share stopped, session ended), the viewer on
// the proxy's client side receives the same code and reason, so the GUI and
// `portty attach` can say why it ended.
func TestRemoteWSProxy_ForwardsClose(t *testing.T) {
	for _, reason := range []string{relay.CloseReasonShareStopped, relay.CloseReasonSessionEnded} {
		t.Run(reason, func(t *testing.T) {
			upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"*"}})
				if err != nil {
					return
				}
				_ = conn.Close(websocket.StatusNormalClosure, reason)
			}))
			defer upstream.Close()
			api := newDaemonAPIWithUpstreamCert(t, upstream)
			socketSrv := httptest.NewServer(api.Handler())
			defer socketSrv.Close()
			relaySrv := httptest.NewServer(api.RelayHandler())
			defer relaySrv.Close()
			depositCap(t, api, "sid1", upstream.URL, fixtureCap)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			conn, _, err := dialProxy(ctx, t, relaySrv.URL, "sid1", "")
			if err != nil {
				t.Fatalf("dial proxy: %v", err)
			}
			defer conn.CloseNow()
			_, _, err = conn.Read(ctx)
			var ce websocket.CloseError
			if !errors.As(err, &ce) {
				t.Fatalf("read err = %v, want a CloseError", err)
			}
			if ce.Code != websocket.StatusNormalClosure || ce.Reason != reason {
				t.Fatalf("close = (%d, %q), want (1000, %q)", ce.Code, ce.Reason, reason)
			}
		})
	}
}

// diagLog captures the standard logger for one test (not parallel-safe: it
// swaps the global output) and is readable while proxy goroutines write.
type diagLog struct {
	mu sync.Mutex
	b  strings.Builder
}

func (d *diagLog) Write(p []byte) (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.b.Write(p)
}

func (d *diagLog) String() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.b.String()
}

func captureDiagLog(t *testing.T) *diagLog {
	t.Helper()
	d := &diagLog{}
	prevOut, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(d)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(prevOut)
		log.SetFlags(prevFlags)
	})
	return d
}

// TestRemoteWSProxy_DiagLog (04-08): the viewing daemon's proxy logs its dial
// time, a failed dial and how a connection ended, and never the cap.
func TestRemoteWSProxy_DiagLog(t *testing.T) {
	buf := captureDiagLog(t)

	// Peer closes with 1000 "share stopped".
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"*"}})
		if err != nil {
			return
		}
		_ = conn.Close(websocket.StatusNormalClosure, relay.CloseReasonShareStopped)
	}))
	defer upstream.Close()
	api := newDaemonAPIWithUpstreamCert(t, upstream)
	relaySrv := httptest.NewServer(api.RelayHandler())
	defer relaySrv.Close()
	depositCap(t, api, "sid1", upstream.URL, fixtureCap)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := dialProxy(ctx, t, relaySrv.URL, "sid1", "")
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	_, _, _ = conn.Read(ctx)
	conn.CloseNow()
	testutil.WaitFor(t, 2*time.Second, func() bool { return strings.Contains(buf.String(), "viewer-diag: proxy end") }, "no proxy end line:\n%s", buf.String())
	if !regexp.MustCompile(`viewer-diag: proxy open session=sid1 host=127\.0\.0\.1 dial=\S+`).MatchString(buf.String()) {
		t.Fatalf("no proxy open line with a dial duration:\n%s", buf.String())
	}
	if !regexp.MustCompile(`viewer-diag: proxy end session=sid1 first=peer code=1000 reason="share stopped" after=\S+`).MatchString(buf.String()) {
		t.Fatalf("no proxy end line first=peer code=1000:\n%s", buf.String())
	}

	// Dial to an unreachable peer.
	dead := httptest.NewTLSServer(http.NotFoundHandler())
	deadURL := dead.URL
	dead.Close()
	depositCap(t, api, "sid2", deadURL, fixtureCap)
	conn2, _, err := dialProxy(ctx, t, relaySrv.URL, "sid2", "")
	if err != nil {
		t.Fatalf("dial proxy (dead peer): %v", err)
	}
	_, _, _ = conn2.Read(ctx)
	conn2.CloseNow()
	testutil.WaitFor(t, 2*time.Second, func() bool {
		return strings.Contains(buf.String(), "viewer-diag: proxy dial failed session=sid2 host=127.0.0.1 err=")
	}, "no proxy dial failed line:\n%s", buf.String())

	if got := buf.String(); strings.Contains(got, fixtureCap) || strings.Contains(got, "cap=") {
		t.Fatalf("proxy log leaks the cap:\n%s", got)
	}

	// A resume query must not reach the logs either (T-05-04).
	wsURL := "ws://" + strings.TrimPrefix(relaySrv.URL, "http://") + "/api/relay/remote/sid2/ws?resume=4242"
	conn3, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("dial proxy (resume): %v", err)
	}
	_, _, _ = conn3.Read(ctx)
	conn3.CloseNow()
	testutil.WaitFor(t, 2*time.Second, func() bool {
		return strings.Count(buf.String(), "viewer-diag: proxy dial failed session=sid2") >= 2
	}, "no second proxy dial failed line:\n%s", buf.String())
	if got := buf.String(); strings.Contains(got, "resume=") || strings.Contains(got, "4242") {
		t.Fatalf("proxy log leaks the resume query:\n%s", got)
	}
}

// TestRemoteWSProxy_ForwardsResume: the proxy forwards ?resume= upstream only
// when it parses as a decimal uint64, re-encoded from the number, and always
// forwards the cap (SHAR-07).
func TestRemoteWSProxy_ForwardsResume(t *testing.T) {
	for _, tc := range []struct {
		name, query, want string
		has               bool
	}{
		{"numeric", "?resume=123", "123", true},
		{"non_numeric", "?resume=abc", "", false},
		{"absent", "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			peer := newFixtureRemotePeerWithWS(t)
			api := newDaemonAPIWithUpstreamCert(t, peer.srv)
			relaySrv := httptest.NewServer(api.RelayHandler())
			defer relaySrv.Close()
			depositCap(t, api, "sid1", peer.srv.URL, fixtureCap)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			wsURL := "ws://" + strings.TrimPrefix(relaySrv.URL, "http://") + "/api/relay/remote/sid1/ws" + tc.query
			conn, _, err := websocket.Dial(ctx, wsURL, nil)
			if err != nil {
				t.Fatalf("dial proxy: %v", err)
			}
			defer conn.CloseNow()
			// Round-trip one frame so the upstream upgrade has completed.
			if err := conn.Write(ctx, websocket.MessageBinary, []byte("ping")); err != nil {
				t.Fatalf("write: %v", err)
			}
			if _, _, err := conn.Read(ctx); err != nil {
				t.Fatalf("read: %v", err)
			}

			peer.mu.Lock()
			gotCap, gotResume, hasResume := peer.gotCap, peer.gotResume, peer.hasResume
			peer.mu.Unlock()
			if gotCap != fixtureCap {
				t.Fatalf("cap %q, want %q", gotCap, fixtureCap)
			}
			if hasResume != tc.has || gotResume != tc.want {
				t.Fatalf("upstream resume %q (present=%t), want %q (present=%t)", gotResume, hasResume, tc.want, tc.has)
			}
		})
	}
}

// proxyWithHeartbeat wires a daemon relay to peer with a short upstream
// heartbeat and dials the proxy for sid1.
func proxyWithHeartbeat(t *testing.T, peer *fixtureWSPeer) *websocket.Conn {
	t.Helper()
	api := newDaemonAPIWithUpstreamCert(t, peer.srv)
	api.SetHeartbeatForTest(50*time.Millisecond, 100*time.Millisecond)
	relaySrv := httptest.NewServer(api.RelayHandler())
	t.Cleanup(relaySrv.Close)
	depositCap(t, api, "sid1", peer.srv.URL, fixtureCap)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := dialProxy(ctx, t, relaySrv.URL, "sid1", "")
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	t.Cleanup(func() { conn.CloseNow() })
	return conn
}

// TestRemoteWSProxy_SilentUpstreamClosesClient (SHAR-07): when the peer stops
// answering pings, the proxy drops both sides with no close frame, so the GUI
// tab or `portty attach` sees an abnormal close (not 1000) and reconnects.
func TestRemoteWSProxy_SilentUpstreamClosesClient(t *testing.T) {
	peer := newFixtureRemotePeerWithWS(t)
	peer.mu.Lock()
	peer.silent = true
	peer.mu.Unlock()
	conn := proxyWithHeartbeat(t, peer)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _, err := conn.Read(ctx)
	if ctx.Err() != nil {
		t.Fatalf("proxy never dropped the silent upstream within 2s (err=%v)", err)
	}
	if code := websocket.CloseStatus(err); code != -1 {
		t.Fatalf("close status %d, want -1 (no close frame)", code)
	}
}

// TestRemoteWSProxy_LiveUpstreamStaysOpen: a peer that keeps reading answers
// every ping, so the proxy leaves it open across many heartbeat intervals.
func TestRemoteWSProxy_LiveUpstreamStaysOpen(t *testing.T) {
	conn := proxyWithHeartbeat(t, newFixtureRemotePeerWithWS(t))
	time.Sleep(time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageBinary, []byte("still-here")); err != nil {
		t.Fatalf("write after 1s: %v", err)
	}
	if _, got, err := conn.Read(ctx); err != nil || string(got) != "still-here" {
		t.Fatalf("echo after 1s = %q, %v", got, err)
	}
}
