package webserver

// Phase 13 (URL-02): every way the share ends must end the proxied streams the
// host holds open for it. A WebSocket upgrade is hijacked, so Server.Close does
// not reach it; only the revoke-bound request context does (ClearGrants through
// revokeSignal, Stop through stopCh). Nothing here touches the tailnet.

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/scottkw/portty/internal/capability"
)

// revokeDev is a dev server with a WebSocket echo, an SSE stream that sends one
// event and holds, and a long poll whose request context end is observable.
type revokeDev struct {
	port      int
	pollEnded chan struct{}
}

func newRevokeDev(t *testing.T) *revokeDev {
	t.Helper()
	d := &revokeDev{pollEnded: make(chan struct{}, 1)}
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.CloseNow()
		for {
			typ, data, err := c.Read(r.Context())
			if err != nil || c.Write(r.Context(), typ, data) != nil {
				return
			}
		}
	})
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: first\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})
	mux.HandleFunc("/poll", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "polling\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		d.pollEnded <- struct{}{}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })
	d.port = srv.Listener.Addr().(*net.TCPAddr).Port
	return d
}

// proxyHdr is the header set of a good proxy request to target.
func (e *ppEnv) proxyHdr(t *testing.T, port int, target string) http.Header {
	t.Helper()
	r := e.good(t, port)
	return http.Header{
		"X-Portty-Cap":        {r.capTok},
		"X-Portty-Port-Token": {r.token},
		"X-Portty-Target":     {target},
	}
}

func (e *ppEnv) proxyURL(port int) string {
	return e.ws.BaseURL() + "/api/peer/sessions/" + e.sid + "/ports/" + strconv.Itoa(port)
}

// openedStreams are a WebSocket and an SSE stream open through the proxy route.
type openedStreams struct {
	ws  *websocket.Conn
	sse io.ReadCloser
}

// openStreams opens both and waits until each has carried data to the dev
// server and back, so both hops are established.
func (e *ppEnv) openStreams(t *testing.T, port int) openedStreams {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	wsURL := "wss" + e.proxyURL(port)[len("https"):]
	c, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPClient: e.client, HTTPHeader: e.proxyHdr(t, port, "/ws")})
	if err != nil {
		t.Fatalf("WebSocket through the proxy route: %v", err)
	}
	t.Cleanup(func() { c.CloseNow() })
	if err := c.Write(ctx, websocket.MessageText, []byte("ping")); err != nil {
		t.Fatal(err)
	}
	if _, data, err := c.Read(ctx); err != nil || string(data) != "ping" {
		t.Fatalf("echo = %q %v", data, err)
	}

	req, _ := http.NewRequest(http.MethodGet, e.proxyURL(port), nil)
	req.Header = e.proxyHdr(t, port, "/events")
	// Its own client: a keep-alive connection on the shared one would outlive
	// the stream and look like a leak.
	resp, err := (&http.Client{Transport: e.client.Transport.(*http.Transport).Clone()}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	buf := make([]byte, len("data: first\n\n"))
	if _, err := io.ReadFull(resp.Body, buf); err != nil || string(buf) != "data: first\n\n" {
		t.Fatalf("first SSE event = %q %v", buf, err)
	}
	return openedStreams{ws: c, sse: resp.Body}
}

// waitEnded fails unless both streams end within 2 s of t0.
func (s openedStreams) waitEnded(t *testing.T, t0 time.Time) {
	t.Helper()
	type end struct {
		which string
		err   error
	}
	ctx, cancel := context.WithDeadline(context.Background(), t0.Add(2*time.Second))
	defer cancel()
	out := make(chan end, 2)
	go func() { _, _, err := s.ws.Read(ctx); out <- end{"WebSocket", err} }()
	go func() { _, err := io.Copy(io.Discard, s.sse); out <- end{"SSE", err} }()
	for i := 0; i < 2; i++ {
		select {
		case r := <-out:
			if r.which == "WebSocket" && ctx.Err() != nil {
				t.Errorf("WebSocket still open %v after the trigger (%v)", time.Since(t0), r.err)
			}
			t.Logf("%s ended %v after the trigger", r.which, time.Since(t0))
		case <-time.After(3 * time.Second):
			t.Fatalf("a stream never ended")
		}
	}
	if time.Since(t0) >= 2*time.Second {
		t.Errorf("streams ended %v after the trigger, want under 2 s", time.Since(t0))
	}
}

// goroutinesSettle waits up to 5 s for the goroutine count to fall back to
// baseline, closing every idle connection that could still hold one.
func (e *ppEnv) goroutinesSettle(t *testing.T, baseline int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		e.client.CloseIdleConnections()
		e.ws.portTransport.CloseIdleConnections()
		n := runtime.NumGoroutine()
		if n <= baseline {
			return
		}
		if time.Now().After(deadline) {
			buf := make([]byte, 1<<16)
			t.Fatalf("%d goroutines, baseline %d:\n%s", n, baseline, buf[:runtime.Stack(buf, true)])
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// baseline warms the proxy path once (so transport and connection goroutines
// from a first request are not counted as leaks) and returns the goroutine
// count with everything idle closed.
func (e *ppEnv) baseline(t *testing.T, port int) int {
	t.Helper()
	r := e.good(t, port)
	r.target = "/none"
	if a := e.proxy(t, r); a.status != http.StatusNotFound {
		t.Fatalf("warm-up = %d", a.status)
	}
	e.client.CloseIdleConnections()
	e.ws.portTransport.CloseIdleConnections()
	time.Sleep(200 * time.Millisecond)
	return runtime.NumGoroutine()
}

func TestPortProxyRevoke_ClearGrants(t *testing.T) {
	dev := newRevokeDev(t)
	e := newPPEnv(t, ppOpts{})
	base := e.baseline(t, dev.port)
	s := e.openStreams(t, dev.port)
	t0 := time.Now()
	e.ws.ClearGrants(e.sid)
	s.waitEnded(t, t0)
	s.ws.CloseNow()
	s.sse.Close()
	e.goroutinesSettle(t, base)
}

func TestPortProxyRevoke_Stop(t *testing.T) {
	dev := newRevokeDev(t)
	e := newPPEnv(t, ppOpts{})
	s := e.openStreams(t, dev.port)
	t0 := time.Now()
	_ = e.ws.Stop()
	s.waitEnded(t, t0)
}

// TestPortProxyRevoke_KeyRegeneration does what the daemon's re-mint does:
// install the new key, clear the grants, add the grant the new capability
// names. The new capability keeps the old grant id so that the old port token
// differs from a good request in its key alone.
func TestPortProxyRevoke_KeyRegeneration(t *testing.T) {
	dev := newRevokeDev(t)
	e := newPPEnv(t, ppOpts{})
	oldToken := e.good(t, dev.port).token
	s := e.openStreams(t, dev.port)

	newKey := append([]byte(nil), capTestKey...)
	newKey[0] ^= 0xff
	t0 := time.Now()
	e.ws.SetSigningKey(newKey)
	e.ws.ClearGrants(e.sid)
	claims := capability.Claims{SID: e.sid, Perms: "read,write", IAT: time.Now().Unix(), GrantID: writeGrantID, V: 1}
	newCap, err := capability.Sign(claims, newKey)
	if err != nil {
		t.Fatal(err)
	}
	e.ws.AddGrant(e.sid, writeGrantID)
	s.waitEnded(t, t0)

	// The old port token against a capability that is good under the new key.
	old := e.good(t, dev.port)
	old.capTok, old.token = newCap, oldToken
	refused := e.proxy(t, old)
	assertRefusedAnswer(t, refused)
	junk := old
	junk.token = "not-a-token"
	if got, want := refused.sig(), e.proxy(t, junk).sig(); got != want {
		t.Errorf("old-token refusal differs from a junk-token refusal:\n%s\nvs\n%s", got, want)
	}
	// Control: a token minted under the new key with the same capability passes,
	// so the refusal above is the key and nothing else.
	fresh := old
	fresh.token = portToken(t, e.sid, dev.port, writeGrantID, newKey)
	fresh.target = "/none"
	if a := e.proxy(t, fresh); a.status != http.StatusNotFound {
		t.Fatalf("a new-key token = %d, want the dev server's 404", a.status)
	}
}

func TestPortProxyRevoke_ClientGone(t *testing.T) {
	dev := newRevokeDev(t)
	e := newPPEnv(t, ppOpts{})
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, e.proxyURL(dev.port), nil)
	req.Header = e.proxyHdr(t, dev.port, "/poll")
	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, len("polling\n"))
	if _, err := io.ReadFull(resp.Body, buf); err != nil {
		t.Fatalf("poll never started: %v", err)
	}
	cancel()
	resp.Body.Close()
	select {
	case <-dev.pollEnded:
	case <-time.After(2 * time.Second):
		t.Fatal("the dev server's request was still running 2 s after the client went away")
	}
}
