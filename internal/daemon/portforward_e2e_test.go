package daemon

// Phase 13 tracer: two daemons in one process, the viewing daemon's forward
// talking to the host's real share server (webserver port routes, real
// capability and port token), which proxies to a dev server on loopback.
// Nothing reaches the tailnet: the share server is a loopback server with a
// self-signed certificate and a same-user WhoIs fake.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/scottkw/portty/internal/relay"
	"github.com/scottkw/portty/internal/tailnet"
	"github.com/scottkw/portty/internal/webserver"
)

func TestPortForwardE2E_HostDevServer(t *testing.T) {
	n := freeLoopbackPort(t) // the port the session prints and the browser opens

	// The dev server, on a port of its own. The host's dialer maps n to it
	// because both Macs share this machine's loopback.
	var mu sync.Mutex
	var seenHost, seenURI string
	var seenHdr http.Header
	dev := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seenHost, seenURI, seenHdr = r.Host, r.RequestURI, r.Header.Clone()
		mu.Unlock()
		io.WriteString(w, "dev-server-body")
	}))
	defer dev.Close()
	devAddr := dev.Listener.Addr().String()

	// Host Mac: a share server whose proxy dials the dev server for port n.
	host, hostClient := shareTestDaemon(t)
	host.SetShareSeamsForTest(healthyTailnet, func(h webserver.TailscaleHealth, m *relay.HubManager) (*webserver.WebServer, error) {
		ws, err := loopbackShareServer(h, m)
		if err != nil {
			return nil, err
		}
		ws.SetPortDialerForTest(func(ctx context.Context, port int) (net.Conn, error) {
			if port != n {
				return nil, fmt.Errorf("unexpected port %d", port)
			}
			return (&net.Dialer{}).DialContext(ctx, "tcp", devAddr)
		})
		return ws, nil
	}, noWarm)
	host.engine.SetSpawnCommandForTest("/bin/sh", []string{"-c", fmt.Sprintf("echo 'Serving on http://localhost:%d/'; exec /bin/cat", n)})
	id := newShareSession(t, hostClient)
	link := mustShare(t, hostClient, id)
	linkURL, linkToken := linkParts(t, link.URL)

	hub, ok := host.engine.Manager().Get(id)
	if !ok {
		t.Fatal("no hub for the shared session")
	}
	deadline := time.Now().Add(5 * time.Second)
	for !hub.PortPrinted(n) {
		if time.Now().After(deadline) {
			t.Fatalf("the session never printed port %d", n)
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Viewing Mac: holds the link's capability, dials the host's share server.
	viewAPI, _, viewSocket := testDaemon(t)
	viewAPI.SetPeerClientForTest(insecureNoRedirectClient())
	if err := viewAPI.remoteCaps.Put(id, "https://"+linkURL.Host, "", linkToken); err != nil {
		t.Fatal(err)
	}
	view := &portEnv{api: viewAPI, socket: viewSocket, client: unixClient(viewSocket)}

	code, got, errCode := view.open(t, id, localhostURL(n, "/"), true)
	if code != http.StatusOK || got.Status != PortStatusOpen || got.Port != n {
		t.Fatalf("open = %d %+v %q", code, got, errCode)
	}

	resp, err := http.Get("http://127.0.0.1:" + strconv.Itoa(n) + "/hello?q=1")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(body) != "dev-server-body" {
		t.Fatalf("GET through the forward = %d %q", resp.StatusCode, body)
	}
	mu.Lock()
	defer mu.Unlock()
	if seenHost != "localhost:"+strconv.Itoa(n) {
		t.Errorf("dev server saw Host %q, want localhost:%d", seenHost, n)
	}
	if seenURI != "/hello?q=1" {
		t.Errorf("dev server saw RequestURI %q, want /hello?q=1", seenURI)
	}
	for k := range seenHdr {
		if strings.HasPrefix(k, "X-Portty") || strings.HasPrefix(k, "X-Forwarded") || k == "Forwarded" {
			t.Errorf("dev server saw header %s", k)
		}
	}

	// A port the session never printed is refused at the grant.
	other := freeLoopbackPort(t)
	code, _, errCode = view.open(t, id, localhostURL(other, "/"), true)
	if code != http.StatusForbidden || errCode != PortErrRefused {
		t.Errorf("unprinted port = %d %q, want 403 %q", code, errCode, PortErrRefused)
	}
	if listening(other) {
		t.Errorf("a forward listens on %d after a refused grant", other)
	}
}

// e2eEnv is the Phase 13 rig for streams and revocation: a host daemon sharing
// a session that printed port n, a dev server behind that port that echoes
// WebSockets and streams SSE, and a viewing daemon holding the share's
// capability with the forward for n open.
type e2eEnv struct {
	n          int
	host       *API
	hostClient *DaemonClient
	id         string
	devHits    atomic.Int64
	view       *portEnv
	relaySrv   *httptest.Server
	baseHost   string // host:port of the share server the viewing daemon holds
}

// newE2E builds the rig. keeper shares a second session first, so the share
// server keeps running (same port, same capability key) when id is unshared.
func newE2E(t *testing.T, keeper bool) *e2eEnv {
	t.Helper()
	e := &e2eEnv{n: freeLoopbackPort(t)}
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		e.devHits.Add(1)
		c, err := websocket.Accept(w, r, nil) // default same-origin check: Origin host must equal Host
		if err != nil {
			return
		}
		defer c.CloseNow()
		for {
			typ, data, err := c.Read(r.Context())
			if err != nil {
				return
			}
			if err := c.Write(r.Context(), typ, data); err != nil {
				return
			}
		}
	})
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		e.devHits.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: first\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done() // hold until the client or the proxy goes away
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		e.devHits.Add(1)
		io.WriteString(w, "dev-server-body")
	})
	dev := httptest.NewServer(mux)
	t.Cleanup(func() { dev.CloseClientConnections(); dev.Close() })
	devAddr := dev.Listener.Addr().String()

	var hostClient *DaemonClient
	e.host, hostClient = shareTestDaemon(t)
	e.hostClient = hostClient
	e.host.SetShareSeamsForTest(healthyTailnet, func(h webserver.TailscaleHealth, m *relay.HubManager) (*webserver.WebServer, error) {
		ws, err := loopbackShareServer(h, m)
		if err != nil {
			return nil, err
		}
		ws.SetPortDialerForTest(func(ctx context.Context, port int) (net.Conn, error) {
			if port != e.n {
				return nil, fmt.Errorf("unexpected port %d", port)
			}
			return (&net.Dialer{}).DialContext(ctx, "tcp", devAddr)
		})
		return ws, nil
	}, noWarm)
	e.host.engine.SetSpawnCommandForTest("/bin/sh", []string{"-c", fmt.Sprintf("echo 'Serving on http://localhost:%d/'; exec /bin/cat", e.n)})
	if keeper {
		mustShare(t, hostClient, newShareSession(t, hostClient))
	}
	e.id = newShareSession(t, hostClient)
	link := mustShare(t, hostClient, e.id)
	e.waitPrinted(t)

	viewAPI, _, viewSocket := testDaemon(t)
	viewAPI.SetPeerClientForTest(insecureNoRedirectClient())
	e.view = &portEnv{api: viewAPI, socket: viewSocket, client: unixClient(viewSocket)}
	e.holdLink(t, link)
	e.relaySrv = httptest.NewServer(viewAPI.RelayHandler())
	t.Cleanup(e.relaySrv.Close)

	code, got, errCode := e.view.open(t, e.id, localhostURL(e.n, "/"), true)
	if code != http.StatusOK || got.Status != PortStatusOpen {
		t.Fatalf("open = %d %+v %q", code, got, errCode)
	}
	return e
}

func (e *e2eEnv) waitPrinted(t *testing.T) {
	t.Helper()
	hub, ok := e.host.engine.Manager().Get(e.id)
	if !ok {
		t.Fatal("no hub for the shared session")
	}
	deadline := time.Now().Add(5 * time.Second)
	for !hub.PortPrinted(e.n) {
		if time.Now().After(deadline) {
			t.Fatalf("the session never printed port %d", e.n)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// holdLink stores the link's capability in the viewing daemon without going
// through /peers/connect (so no connect-time revoke runs).
func (e *e2eEnv) holdLink(t *testing.T, link ShareResponse) {
	t.Helper()
	u, tok := linkParts(t, link.URL)
	e.baseHost = u.Host
	if err := e.view.api.remoteCaps.Put(e.id, "https://"+u.Host, "", tok); err != nil {
		t.Fatal(err)
	}
}

// connect is POST /peers/connect against the host's real share server: it
// fetches the current capability and stores it, revoking forwards when it
// differs from the one held. link is the share link just minted.
func (e *e2eEnv) connect(t *testing.T, link ShareResponse) {
	t.Helper()
	u, _ := linkParts(t, link.URL)
	e.baseHost = u.Host
	peer := tailnet.Peer{Hostname: "office-air", DNSName: "office-air.tail1234.ts.net.", Online: true}
	e.view.api.SetPeerSeamsForTest(
		func(context.Context) ([]tailnet.Peer, error) { return []tailnet.Peer{peer}, nil },
		nil,
		func(tailnet.Peer) string { return "https://" + u.Host },
	)
	b, _ := json.Marshal(PeerConnectRequest{Host: "office-air", Session: e.id})
	resp, err := e.view.client.Post("http://daemon/peers/connect", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST /peers/connect = %d %s", resp.StatusCode, raw)
	}
}

// openWS dials the forward the way a browser page on http://127.0.0.1:<n>
// would: Origin names the typed host, which the host must rewrite to match
// the Host the dev server sees, or the default same-origin check refuses it.
func (e *e2eEnv) openWS(t *testing.T) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws://127.0.0.1:"+strconv.Itoa(e.n)+"/ws", &websocket.DialOptions{
		HTTPHeader: http.Header{"Origin": {"http://127.0.0.1:" + strconv.Itoa(e.n)}},
	})
	if err != nil {
		t.Fatalf("WebSocket through both hops: %v", err)
	}
	t.Cleanup(func() { conn.CloseNow() })
	if err := conn.Write(ctx, websocket.MessageText, []byte("ping")); err != nil {
		t.Fatal(err)
	}
	if _, data, err := conn.Read(ctx); err != nil || string(data) != "ping" {
		t.Fatalf("echo = %q %v", data, err)
	}
	return conn
}

// openSSE GETs /events through the forward and waits for the first event.
func (e *e2eEnv) openSSE(t *testing.T) io.ReadCloser {
	t.Helper()
	resp, err := http.Get("http://127.0.0.1:" + strconv.Itoa(e.n) + "/events")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	first := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(resp.Body).ReadString('\n')
		first <- line
	}()
	select {
	case line := <-first:
		if line != "data: first\n" {
			t.Fatalf("first SSE line = %q", line)
		}
	case <-time.After(time.Second):
		t.Fatal("no SSE event within 1 s through both hops")
	}
	return resp.Body
}

// stopped reports how long after t0 each open stream ended, failing the test
// for any that outlived 2 s. No request is sent to the forward meanwhile.
type stopped struct {
	session, ws, sse time.Duration
	sessionErr       error
}

// openStreams opens a WebSocket, an SSE stream and the peer tab's session
// stream (through the viewing daemon's relay handler at /api/relay/remote/<id>/ws,
// the stream a desktop peer tab holds), and returns a func that
// waits for all three to end after the trigger at t0.
func (e *e2eEnv) openStreams(t *testing.T) func(t0 time.Time) stopped {
	t.Helper()
	wsConn := e.openWS(t)
	sse := e.openSSE(t)
	sess := dialRemoteWS(t, e.relaySrv.URL, e.id)
	hub, _ := e.host.engine.Manager().Get(e.id)
	for deadline := time.Now().Add(5 * time.Second); hub.RemoteViewerCount() == 0; time.Sleep(5 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the session stream never reached the host")
		}
	}
	return func(t0 time.Time) stopped {
		t.Helper()
		type end struct {
			which string
			after time.Duration
			err   error
		}
		out := make(chan end, 3)
		ctx, cancel := context.WithDeadline(context.Background(), t0.Add(2*time.Second))
		defer cancel()
		go func() { // the host replays scrollback first; the terminal close is the first error
			var err error
			for err == nil {
				_, _, err = sess.Read(ctx)
			}
			out <- end{"session", time.Since(t0), err}
		}()
		go func() { _, _, err := wsConn.Read(ctx); out <- end{"ws", time.Since(t0), err} }()
		go func() { _, err := io.Copy(io.Discard, sse); out <- end{"sse", time.Since(t0), err} }()
		var s stopped
		for i := 0; i < 3; i++ {
			select {
			case r := <-out:
				if r.after >= 2*time.Second || (r.which != "sse" && ctx.Err() != nil) {
					t.Errorf("%s stream still open %v after the trigger (%v)", r.which, r.after, r.err)
				}
				switch r.which {
				case "session":
					s.session, s.sessionErr = r.after, r.err
				case "ws":
					s.ws = r.after
				case "sse":
					s.sse = r.after
				}
			case <-time.After(3 * time.Second):
				t.Fatal("a stream never ended")
			}
		}
		return s
	}
}

// assertGone: the forward stops listening within 2 s of t0 and the viewing
// daemon logged the revoke.
func (e *e2eEnv) assertGone(t *testing.T, t0 time.Time, logs *lockedBuffer) time.Duration {
	t.Helper()
	for listening(e.n) {
		if time.Since(t0) > 2*time.Second {
			t.Fatalf("127.0.0.1:%d still listens %v after the trigger", e.n, time.Since(t0))
		}
		time.Sleep(10 * time.Millisecond)
	}
	d := time.Since(t0)
	if !logs.has("outcome=revoked") {
		t.Errorf("no outcome=revoked line in the viewing daemon's log")
	}
	return d
}

// clickAsksConsent: a click with no consent answers consent, never open.
func (e *e2eEnv) clickAsksConsent(t *testing.T) {
	t.Helper()
	if _, got, errCode := e.view.open(t, e.id, localhostURL(e.n, "/"), false); got.Status != PortStatusConsent {
		t.Fatalf("click = %+v %q, want consent", got, errCode)
	}
}

func TestPortForwardE2E_WebSocketAndSSE(t *testing.T) {
	e := newE2E(t, false)
	e.openWS(t)
	e.openSSE(t)
}

func TestPortForwardE2E_StopSharing(t *testing.T) {
	// The share-off subtests turn the share off; "KeepShareServer" leaves
	// another shared session so only the grant revoke (not the server's Stop)
	// can end the streams.
	cases := []struct {
		name    string
		keeper  bool
		trigger func(t *testing.T, e *e2eEnv)
		reason  string
	}{
		{"ShareOff", false, func(t *testing.T, e *e2eEnv) {
			if _, err := e.hostClient.SetSessionShared(e.id, false); err != nil {
				t.Fatal(err)
			}
		}, relay.CloseReasonShareStopped},
		{"ShareOffKeepShareServer", true, func(t *testing.T, e *e2eEnv) {
			if _, err := e.hostClient.SetSessionShared(e.id, false); err != nil {
				t.Fatal(err)
			}
		}, relay.CloseReasonShareStopped},
		{"SessionKilled", false, func(t *testing.T, e *e2eEnv) {
			if err := e.hostClient.KillSession(e.id); err != nil {
				t.Fatal(err)
			}
		}, relay.CloseReasonSessionEnded},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			logs := captureLog(t)
			e := newE2E(t, c.keeper)
			wait := e.openStreams(t)
			t0 := time.Now()
			c.trigger(t, e)
			s := wait(t0)
			var ce websocket.CloseError
			if !errors.As(s.sessionErr, &ce) || ce.Code != websocket.StatusNormalClosure || ce.Reason != c.reason {
				t.Errorf("session stream ended with %v, want 1000 %q", s.sessionErr, c.reason)
			}
			gone := e.assertGone(t, t0, logs)
			t.Logf("ended after trigger: session stream %v, WebSocket %v, SSE %v, forward closed %v", s.session, s.ws, s.sse, gone)
			if c.name == "SessionKilled" {
				return
			}
			// Re-share: a click with no request in between asks again.
			e.connect(t, mustShare(t, e.hostClient, e.id))
			e.clickAsksConsent(t)
		})
	}
}

func TestPortForwardE2E_ReshareWithoutStream(t *testing.T) {
	t.Run("PeerConnectAsksConsent", func(t *testing.T) {
		logs := captureLog(t)
		e := newE2E(t, true) // keeper: the share server survives the share-off
		if _, err := e.hostClient.SetSessionShared(e.id, false); err != nil {
			t.Fatal(err)
		}
		link := mustShare(t, e.hostClient, e.id)
		if !listening(e.n) {
			t.Fatal("the forward closed without any signal")
		}
		t0 := time.Now()
		e.connect(t, link) // the new capability: closes the forward at once
		e.assertGone(t, t0, logs)
		e.clickAsksConsent(t)
		if listening(e.n) {
			t.Error("the stale forward came back before consent")
		}
	})
	t.Run("NoConnectStoppedPage", func(t *testing.T) {
		e := newE2E(t, true) // keeper: the old capability reaches a live server and is refused
		if _, err := e.hostClient.SetSessionShared(e.id, false); err != nil {
			t.Fatal(err)
		}
		mustShare(t, e.hostClient, e.id)
		hits := e.devHits.Load()
		code, got, errCode := e.view.open(t, e.id, localhostURL(e.n, "/"), false)
		if code != http.StatusForbidden || errCode != PortErrRefused || got.Status != "" {
			t.Fatalf("click with the old capability = %d %+v %q, want 403 %q and no consent", code, got, errCode, PortErrRefused)
		}
		if !listening(e.n) {
			t.Fatal("the refused click closed the forward (it has no signal to)")
		}
		resp, err := http.Get("http://127.0.0.1:" + strconv.Itoa(e.n) + "/")
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadGateway || !strings.Contains(string(body), "stopped sharing") {
			t.Fatalf("first request = %d %q, want 502 with the stopped page", resp.StatusCode, body)
		}
		waitClosed(t, e.n)
		if n := e.devHits.Load(); n != hits {
			t.Errorf("the dev server received %d requests after the share-off", n-hits)
		}
	})
}
