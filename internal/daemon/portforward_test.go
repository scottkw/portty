package daemon

// Tests for the viewing daemon's loopback forwards (portforward.go,
// portforward_api.go) against a stub host that speaks the Phase 13 interface
// contract. Everything binds loopback only and never reaches the tailnet: the
// stub is an httptest TLS server and the daemon dials it through
// SetPeerClientForTest.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/scottkw/portty/internal/tailnet"
)

// freeLoopbackPort returns a port free on 127.0.0.1 and [::1].
func freeLoopbackPort(t *testing.T) int {
	t.Helper()
	for i := 0; i < 50; i++ {
		l4, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		port := l4.Addr().(*net.TCPAddr).Port
		l6, err := net.Listen("tcp6", fmt.Sprintf("[::1]:%d", port))
		l4.Close()
		if err == nil {
			l6.Close()
			return port
		}
		if !errors.Is(err, syscall.EADDRINUSE) && !hasIPv6Loopback() {
			return port
		}
	}
	t.Fatal("no loopback port free on both families")
	return 0
}

func hasIPv6Loopback() bool {
	l, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		return false
	}
	l.Close()
	return true
}

// stubHit is one request that reached the stub's proxy route.
type stubHit struct {
	method, path, rawQuery string
	hdr                    http.Header
}

// stubHost imitates a host's share server for the three routes the forward
// touches. Its knobs are read under mu on every request.
type stubHost struct {
	srv *httptest.Server

	mu          sync.Mutex
	grantStatus int  // 0 = 200 with a token
	grantMarker bool // X-Portty: 1 on a non-200 grant
	grants      []string
	tokenSeq    int
	proxyStatus int // 0 = 200 with body "stub-body"
	proxyMarker bool
	proxyHdr    http.Header // extra headers on a 200
	hits        []stubHit
	capVal      string // what POST /api/peer/sessions/{id}/cap hands out
	wsConns     chan *websocket.Conn
}

func newStubHost(t *testing.T) *stubHost {
	t.Helper()
	s := &stubHost{capVal: "CAP", wsConns: make(chan *websocket.Conn, 4)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/sessions/meta", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `[{"id":"s1","name":"build","status":"running"}]`)
	})
	mux.HandleFunc("POST /api/peer/sessions/{id}/cap", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		c := s.capVal
		s.mu.Unlock()
		fmt.Fprintf(w, `{"cap":%q}`, c)
	})
	mux.HandleFunc("GET /sessions/{id}/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"*"}})
		if err != nil {
			return
		}
		s.wsConns <- conn
		for {
			if _, _, err := conn.Read(r.Context()); err != nil {
				return
			}
		}
	})
	mux.HandleFunc("POST /api/peer/sessions/{id}/ports", s.handleGrant)
	mux.HandleFunc("/api/peer/sessions/{id}/ports/{port}", s.handleProxy)
	s.srv = httptest.NewTLSServer(mux)
	t.Cleanup(s.srv.Close)
	return s
}

func (s *stubHost) handleGrant(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var body struct {
		Port int `json:"port"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	s.grants = append(s.grants, fmt.Sprintf("%s|%s|%d|%s", r.PathValue("id"), r.Header.Get("X-Portty-Cap"), body.Port, r.Header.Get("Content-Type")))
	if s.grantStatus != 0 {
		if s.grantMarker {
			w.Header().Set("X-Portty", "1")
		}
		http.Error(w, "forbidden", s.grantStatus)
		return
	}
	s.tokenSeq++
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprintf(w, `{"token":"TOK-%d"}`, s.tokenSeq)
}

func (s *stubHost) handleProxy(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.hits = append(s.hits, stubHit{r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Clone()})
	status, marker, extra := s.proxyStatus, s.proxyMarker, s.proxyHdr
	s.mu.Unlock()
	if status != 0 {
		if marker {
			w.Header().Set("X-Portty", "1")
		}
		http.Error(w, "stub refusal", status)
		return
	}
	if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") { // an HMR-style echo
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"*"}})
		if err != nil {
			return
		}
		defer conn.CloseNow()
		for {
			typ, data, err := conn.Read(r.Context())
			if err != nil || conn.Write(r.Context(), typ, data) != nil {
				return
			}
		}
	}
	for k, v := range extra {
		w.Header()[k] = v
	}
	io.WriteString(w, "stub-body")
}

func (s *stubHost) hitCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.hits)
}

func (s *stubHost) lastHit(t *testing.T) stubHit {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.hits) == 0 {
		t.Fatal("the stub saw no request")
	}
	return s.hits[len(s.hits)-1]
}

func (s *stubHost) token() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return "TOK-" + strconv.Itoa(s.tokenSeq)
}

// portEnv is a daemon on a unix socket with a stub host and a held capability
// for session s1.
type portEnv struct {
	api    *API
	stub   *stubHost
	socket string
	client *http.Client
}

func newPortEnv(t *testing.T) *portEnv {
	t.Helper()
	stub := newStubHost(t)
	e := newPortEnvFor(t, stub.srv)
	e.stub = stub
	return e
}

// newPortEnvFor is a daemon whose session s1 is held against srv.
func newPortEnvFor(t *testing.T, srv *httptest.Server) *portEnv {
	t.Helper()
	api, _, socket := testDaemon(t)
	api.SetPeerClientForTest(srv.Client())
	if err := api.remoteCaps.Put("s1", srv.URL, "", "CAP"); err != nil {
		t.Fatal(err)
	}
	return &portEnv{api: api, socket: socket, client: unixClient(socket)}
}

func unixClient(socket string) *http.Client {
	return &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", socket)
		},
	}}
}

// open posts to /peers/ports and decodes the answer.
func (e *portEnv) open(t *testing.T, sid, rawURL string, consent bool) (int, PortOpenResponse, string) {
	t.Helper()
	b, _ := json.Marshal(PortOpenRequest{SessionID: sid, URL: rawURL, Consent: consent})
	resp, err := e.client.Post("http://daemon/peers/ports", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST /peers/ports: %v", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var ok PortOpenResponse
	var bad PortOpenErrorResponse
	if resp.StatusCode == http.StatusOK {
		_ = json.Unmarshal(raw, &ok)
	} else {
		_ = json.Unmarshal(raw, &bad)
	}
	return resp.StatusCode, ok, bad.Error
}

func localhostURL(port int, rest string) string {
	return "http://localhost:" + strconv.Itoa(port) + rest
}

// listening reports whether a TCP dial to 127.0.0.1:port connects.
func listening(port int) bool {
	c, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(port), time.Second)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

// waitClosed polls until nothing listens on 127.0.0.1:port, for up to 2 s.
func waitClosed(t *testing.T, port int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for listening(port) {
		if time.Now().After(deadline) {
			t.Fatalf("127.0.0.1:%d still listens", port)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestPortForward_OpenThenGET(t *testing.T) {
	e := newPortEnv(t)
	port := freeLoopbackPort(t)

	code, got, errCode := e.open(t, "s1", localhostURL(port, "/"), true)
	if code != http.StatusOK || got.Status != PortStatusOpen || got.URL != localhostURL(port, "/") || got.Port != port {
		t.Fatalf("open = %d %+v %q", code, got, errCode)
	}
	target := "/a//b/%2e%2e/c%2Fd?x=1&cap=page"
	resp, err := http.Get("http://127.0.0.1:" + strconv.Itoa(port) + target)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(body) != "stub-body" {
		t.Fatalf("GET = %d %q", resp.StatusCode, body)
	}
	hit := e.stub.lastHit(t)
	if hit.hdr.Get("X-Portty-Target") != target {
		t.Errorf("X-Portty-Target = %q, want %q", hit.hdr.Get("X-Portty-Target"), target)
	}
	if hit.hdr.Get("X-Portty-Cap") != "CAP" || hit.hdr.Get("X-Portty-Port-Token") != e.stub.token() {
		t.Errorf("cap %q token %q", hit.hdr.Get("X-Portty-Cap"), hit.hdr.Get("X-Portty-Port-Token"))
	}
	if want := "/api/peer/sessions/s1/ports/" + strconv.Itoa(port); hit.path != want || hit.rawQuery != "" {
		t.Errorf("stub saw %q ? %q, want %q with no query", hit.path, hit.rawQuery, want)
	}
	if g := e.stub.grants[0]; g != fmt.Sprintf("s1|CAP|%d|application/json", port) {
		t.Errorf("grant = %q", g)
	}
}

func TestPortForward_ConsentFirst(t *testing.T) {
	e := newPortEnv(t)
	port := freeLoopbackPort(t)

	code, got, errCode := e.open(t, "s1", localhostURL(port, "/"), false)
	if code != http.StatusOK || got.Status != PortStatusConsent || got.Host == "" || got.Port != port || got.URL != "" {
		t.Fatalf("consent = %d %+v %q", code, got, errCode)
	}
	if listening(port) {
		t.Fatalf("a forward listens on %d before consent", port)
	}
}

// get fetches target through the forward on port and returns status and body.
func get(t *testing.T, port int, target string) (int, string) {
	t.Helper()
	resp, err := http.Get("http://127.0.0.1:" + strconv.Itoa(port) + target)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

type localhostURLFixture struct {
	Cases []struct {
		Input string `json:"input"`
		Match bool   `json:"match"`
		Port  int    `json:"port"`
		Rest  string `json:"rest"`
		Note  string `json:"note"`
	} `json:"cases"`
}

const localhostURLFixturePath = "../../frontend/src/lib/__tests__/fixtures/localhost-url.json"

func TestClassifyLoopbackURL_Fixture(t *testing.T) {
	raw, err := os.ReadFile(localhostURLFixturePath)
	if err != nil {
		t.Fatal(err)
	}
	var f localhostURLFixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Cases) < 28 {
		t.Fatalf("fixture holds %d cases, want at least 28", len(f.Cases))
	}
	for _, c := range f.Cases {
		port, rest, ok := classifyLoopbackURL(c.Input)
		if ok != c.Match || port != c.Port || rest != c.Rest {
			t.Errorf("classifyLoopbackURL(%q) = %d %q %v, want %d %q %v (%s)", c.Input, port, rest, ok, c.Port, c.Rest, c.Match, c.Note)
		}
	}
}

func TestPortOpen_ErrorCodes(t *testing.T) {
	good := func(p int) string { return localhostURL(p, "/") }
	cases := []struct {
		name   string
		setup  func(e *portEnv)
		sid    string
		url    func(p int) string
		status int
		code   string
	}{
		{"no held cap", nil, "nope", good, 404, PortErrNoCap},
		{"https", nil, "s1", func(p int) string { return "https://localhost:" + strconv.Itoa(p) + "/" }, 400, PortErrBadURL},
		{"upper case host", nil, "s1", func(p int) string { return "http://LOCALHOST:" + strconv.Itoa(p) + "/" }, 400, PortErrBadURL},
		{"userinfo", nil, "s1", func(p int) string { return "http://localhost:" + strconv.Itoa(p) + "@evil.com/" }, 400, PortErrBadURL},
		{"host refuses", func(e *portEnv) { e.stub.grantStatus, e.stub.grantMarker = 403, true }, "s1", good, 403, PortErrRefused},
		{"403 without the marker", func(e *portEnv) { e.stub.grantStatus = 403 }, "s1", good, 502, PortErrUnreachable},
		{"503 without the marker", func(e *portEnv) { e.stub.grantStatus = 503 }, "s1", good, 502, PortErrUnreachable},
		{"host closed", func(e *portEnv) { e.stub.srv.Close() }, "s1", good, 502, PortErrUnreachable},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newPortEnv(t)
			if c.setup != nil {
				c.setup(e)
			}
			port := freeLoopbackPort(t)
			status, _, code := e.open(t, c.sid, c.url(port), true)
			if status != c.status || code != c.code {
				t.Fatalf("answer = %d %q, want %d %q", status, code, c.status, c.code)
			}
			if listening(port) {
				t.Fatalf("something listens on %d after %s", port, c.code)
			}
		})
	}

	t.Run("old host", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "old") })
		srv := httptest.NewTLSServer(mux)
		t.Cleanup(srv.Close)
		e := newPortEnvFor(t, srv)
		port := freeLoopbackPort(t)
		status, _, code := e.open(t, "s1", localhostURL(port, "/"), true)
		if status != 409 || code != PortErrHostOutdated {
			t.Fatalf("answer = %d %q, want 409 host_outdated", status, code)
		}
		if listening(port) {
			t.Fatalf("something listens on %d", port)
		}
	})
}

// holder is a foreign listener that counts accepted connections.
type holder struct {
	ln      net.Listener
	accepts chan struct{}
}

func newHolder(t *testing.T, network, addr string) *holder {
	t.Helper()
	ln, err := net.Listen(network, addr)
	if err != nil {
		t.Fatal(err)
	}
	h := &holder{ln: ln, accepts: make(chan struct{}, 64)}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
			h.accepts <- struct{}{}
		}
	}()
	return h
}

// answers reports whether a dial to addr still reaches the holder.
func (h *holder) answers(t *testing.T, addr string) bool {
	t.Helper()
	for len(h.accepts) > 0 {
		<-h.accepts
	}
	c, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		return false
	}
	c.Close()
	select {
	case <-h.accepts:
		return true
	case <-time.After(time.Second):
		return false
	}
}

func TestPortOpen_PortInUse(t *testing.T) {
	t.Run("wildcard holder", func(t *testing.T) {
		e := newPortEnv(t)
		port := freeLoopbackPort(t)
		h := newHolder(t, "tcp", ":"+strconv.Itoa(port))
		status, _, code := e.open(t, "s1", localhostURL(port, "/"), true)
		if status != 409 || code != PortErrPortInUse {
			t.Fatalf("answer = %d %q, want 409 port_in_use", status, code)
		}
		if !h.answers(t, "127.0.0.1:"+strconv.Itoa(port)) {
			t.Fatal("the foreign holder no longer answers on 127.0.0.1")
		}
		h.ln.Close()
		waitClosed(t, port) // nothing of ours is left behind
	})
	t.Run("::1 only holder", func(t *testing.T) {
		if !hasIPv6Loopback() {
			t.Skip("no IPv6 loopback on this Mac")
		}
		e := newPortEnv(t)
		port := freeLoopbackPort(t)
		h := newHolder(t, "tcp6", "[::1]:"+strconv.Itoa(port))
		status, _, code := e.open(t, "s1", localhostURL(port, "/"), true)
		if status != 409 || code != PortErrPortInUse {
			t.Fatalf("answer = %d %q, want 409 port_in_use", status, code)
		}
		if listening(port) {
			t.Fatal("a forward was bound on 127.0.0.1 next to a ::1 holder")
		}
		if !h.answers(t, "[::1]:"+strconv.Itoa(port)) {
			t.Fatal("the ::1 holder no longer answers")
		}
	})
}

func TestPortOpen_Approvals(t *testing.T) {
	e := newPortEnv(t)
	port := freeLoopbackPort(t)
	other := freeLoopbackPort(t)

	if _, got, _ := e.open(t, "s1", localhostURL(port, "/"), false); got.Status != PortStatusConsent {
		t.Fatalf("first click = %+v, want consent", got)
	}
	if _, got, errCode := e.open(t, "s1", localhostURL(port, "/"), true); got.Status != PortStatusOpen {
		t.Fatalf("consented click = %+v %q, want open", got, errCode)
	}
	if _, got, _ := e.open(t, "s1", localhostURL(port, "/x"), false); got.Status != PortStatusOpen || got.URL != localhostURL(port, "/x") {
		t.Fatalf("approved click = %+v, want open without asking", got)
	}
	if _, got, _ := e.open(t, "s1", localhostURL(other, "/"), false); got.Status != PortStatusConsent {
		t.Fatalf("another port = %+v, want consent", got)
	}

	// A second Mac (example.com, dialled to its own stub) cannot take the port.
	second := newStubHost(t)
	tr := e.stub.srv.Client().Transport.(*http.Transport).Clone()
	tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		if strings.HasPrefix(addr, "example.com:") {
			addr = second.srv.Listener.Addr().String()
		}
		return (&net.Dialer{}).DialContext(ctx, network, addr)
	}
	e.api.SetPeerClientForTest(&http.Client{Transport: tr})
	if err := e.api.remoteCaps.Put("s2", "https://example.com:1", "", "CAP-B"); err != nil {
		t.Fatal(err)
	}
	status, _, code := e.open(t, "s2", localhostURL(port, "/"), true)
	if status != 409 || code != PortErrPortInUse {
		t.Fatalf("second Mac = %d %q, want 409 port_in_use", status, code)
	}
	if n := e.stub.hitCount(); n != 0 {
		t.Fatalf("first Mac's stub saw %d requests", n)
	}
	if st, _ := get(t, port, "/"); st != 200 || e.stub.hitCount() != 1 {
		t.Fatal("the first Mac's forward no longer answers")
	}
	if second.hitCount() != 0 {
		t.Fatal("the second Mac's stub was reached through the first Mac's forward")
	}

	// The same Mac, another session, while the approval is live: rebound with no consent.
	if err := e.api.remoteCaps.Put("s3", e.stub.srv.URL, "", "CAP3"); err != nil {
		t.Fatal(err)
	}
	if _, got, errCode := e.open(t, "s3", localhostURL(port, "/"), false); got.Status != PortStatusOpen {
		t.Fatalf("same Mac, other session = %+v %q, want open", got, errCode)
	}
	get(t, port, "/")
	if hit := e.stub.lastHit(t); hit.path != "/api/peer/sessions/s3/ports/"+strconv.Itoa(port) || hit.hdr.Get("X-Portty-Cap") != "CAP3" {
		t.Fatalf("after the rebind the stub saw %q with cap %q", hit.path, hit.hdr.Get("X-Portty-Cap"))
	}
}

// With every revocation signal bypassed: a capability replaced behind the
// daemon's back (a re-share whose signals were missed) must not leave `open`
// answerable.
func TestPortOpen_ApprovalLiveCheck(t *testing.T) {
	e := newPortEnv(t)
	port := freeLoopbackPort(t)
	if _, got, errCode := e.open(t, "s1", localhostURL(port, "/"), true); got.Status != PortStatusOpen {
		t.Fatalf("open = %+v %q", got, errCode)
	}
	if err := e.api.remoteCaps.Put("s1", e.stub.srv.URL, "", "CAP2"); err != nil {
		t.Fatal(err)
	}
	if _, got, _ := e.open(t, "s1", localhostURL(port, "/"), false); got.Status != PortStatusConsent {
		t.Fatalf("after a new capability = %+v, want consent, never open", got)
	}
	if listening(port) {
		t.Fatal("the old forward still listens")
	}
	if _, got, errCode := e.open(t, "s1", localhostURL(port, "/"), true); got.Status != PortStatusOpen {
		t.Fatalf("consented again = %+v %q", got, errCode)
	}
	get(t, port, "/")
	hit := e.stub.lastHit(t)
	if hit.hdr.Get("X-Portty-Port-Token") != e.stub.token() || hit.hdr.Get("X-Portty-Cap") != "CAP2" {
		t.Fatalf("stub saw token %q cap %q, want the new grant under CAP2", hit.hdr.Get("X-Portty-Port-Token"), hit.hdr.Get("X-Portty-Cap"))
	}
}

// A port_in_use after consent must not leave an approval behind.
func TestPortOpen_NoApprovalAfterPortInUse(t *testing.T) {
	e := newPortEnv(t)
	port := freeLoopbackPort(t)
	h := newHolder(t, "tcp", ":"+strconv.Itoa(port))
	if status, _, code := e.open(t, "s1", localhostURL(port, "/"), true); status != 409 || code != PortErrPortInUse {
		t.Fatalf("answer = %d %q, want 409 port_in_use", status, code)
	}
	h.ln.Close()
	waitClosed(t, port)
	if _, got, _ := e.open(t, "s1", localhostURL(port, "/"), false); got.Status != PortStatusConsent {
		t.Fatalf("after the holder left = %+v, want consent (no approval was recorded)", got)
	}
}

func TestForwardHostAllowed(t *testing.T) {
	cases := []struct {
		host string
		port int
		want bool
	}{
		{"localhost:8000", 8000, true},
		{"127.0.0.1:8000", 8000, true},
		{"[::1]:8000", 8000, false},
		{"evil.test:8000", 8000, false},
		{"localhost:8001", 8000, false},
		{"localhost:80000", 8000, false},
		{"localhost", 8000, false},
		{"127.0.0.1", 8000, false},
		{"", 8000, false},
		{"localhost", 80, true},
		{"127.0.0.1", 80, true},
		{"localhost:80", 80, true},
		{"LOCALHOST:8000", 8000, false},
	}
	for _, c := range cases {
		if got := forwardHostAllowed(c.host, c.port); got != c.want {
			t.Errorf("forwardHostAllowed(%q, %d) = %v, want %v", c.host, c.port, got, c.want)
		}
	}
}

// openForward opens a forward on a fresh port for s1 and returns the port.
func (e *portEnv) openForward(t *testing.T) int {
	t.Helper()
	port := freeLoopbackPort(t)
	if _, got, errCode := e.open(t, "s1", localhostURL(port, "/"), true); got.Status != PortStatusOpen {
		t.Fatalf("open = %+v %q", got, errCode)
	}
	return port
}

// do sends a request to the forward on port with the given Host and headers.
func do(t *testing.T, port int, host string, hdr map[string]string) (*http.Response, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:"+strconv.Itoa(port)+"/p?q=1", nil)
	if host != "" {
		req.Host = host
	}
	for k, v := range hdr {
		req.Header[k] = []string{v} // set verbatim, even a non-canonical name
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

func TestPortForward_HostRefused(t *testing.T) {
	e := newPortEnv(t)
	port := e.openForward(t)
	resp, body := do(t, port, "evil.test:"+strconv.Itoa(port), nil)
	if resp.StatusCode != http.StatusForbidden || strings.Contains(body, "evil.test") {
		t.Fatalf("answer = %d %q, want 403 reflecting nothing", resp.StatusCode, body)
	}
	if n := e.stub.hitCount(); n != 0 {
		t.Fatalf("the stub saw %d requests", n)
	}
}

func TestPortForward_CrossSite(t *testing.T) {
	e := newPortEnv(t)
	port := e.openForward(t)
	resp, body := do(t, port, "", map[string]string{"Sec-Fetch-Site": "cross-site", "Sec-Fetch-Mode": "no-cors"})
	if resp.StatusCode != http.StatusForbidden || body != "forbidden\n" || e.stub.hitCount() != 0 {
		t.Fatalf("cross-site no-cors = %d %q with %d stub hits, want 403 and none", resp.StatusCode, body, e.stub.hitCount())
	}
	for i, hdr := range []map[string]string{
		{"Sec-Fetch-Site": "cross-site", "Sec-Fetch-Mode": "navigate"},
		{"Sec-Fetch-Site": "same-origin", "Sec-Fetch-Mode": "cors"},
		nil,
	} {
		if resp, _ := do(t, port, "", hdr); resp.StatusCode != http.StatusOK || e.stub.hitCount() != i+1 {
			t.Fatalf("%v = %d with %d stub hits, want it to reach the stub", hdr, resp.StatusCode, e.stub.hitCount())
		}
	}
}

func TestPortForward_HeaderHygiene(t *testing.T) {
	e := newPortEnv(t)
	port := e.openForward(t)
	e.stub.mu.Lock()
	e.stub.proxyHdr = http.Header{
		"X-Portty":                {"1"},
		"X-Portty-Debug":          {"x"},
		"Content-Security-Policy": {"default-src 'self'"},
		"Set-Cookie":              {"sid=abc; Path=/"},
	}
	e.stub.mu.Unlock()
	resp, _ := do(t, port, "", map[string]string{"X-Portty-Cap": "evil", "X-Portty-Target": "/evil", "x-portty-port-token": "evil", "X-Portty-Extra": "evil"})
	hit := e.stub.lastHit(t)
	if hit.hdr.Get("X-Portty-Cap") != "CAP" || hit.hdr.Get("X-Portty-Target") != "/p?q=1" || hit.hdr.Get("X-Portty-Port-Token") != e.stub.token() {
		t.Errorf("the stub saw cap %q target %q token %q", hit.hdr.Get("X-Portty-Cap"), hit.hdr.Get("X-Portty-Target"), hit.hdr.Get("X-Portty-Port-Token"))
	}
	if hit.hdr.Get("X-Portty-Extra") != "" {
		t.Error("an extra X-Portty header from the page reached the stub")
	}
	for k := range resp.Header {
		if strings.HasPrefix(k, "X-Portty") {
			t.Errorf("response header %s reached the browser", k)
		}
	}
	if resp.Header.Get("Content-Security-Policy") != "default-src 'self'" || resp.Header.Get("Set-Cookie") != "sid=abc; Path=/" {
		t.Errorf("the dev server's own headers changed: csp %q cookie %q", resp.Header.Get("Content-Security-Policy"), resp.Header.Get("Set-Cookie"))
	}
}

func checkPage(t *testing.T, resp *http.Response, body string, wants ...string) {
	t.Helper()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", resp.StatusCode)
	}
	for k, v := range map[string]string{
		"Content-Type":            "text/html; charset=utf-8",
		"Content-Security-Policy": "default-src 'none'",
		"X-Content-Type-Options":  "nosniff",
		"Cache-Control":           "no-store",
	} {
		if resp.Header.Get(k) != v {
			t.Errorf("%s = %q, want %q", k, resp.Header.Get(k), v)
		}
	}
	for _, w := range wants {
		if !strings.Contains(body, w) {
			t.Errorf("page lacks %q: %s", w, body)
		}
	}
	if strings.Contains(body, "/p?q=1") {
		t.Errorf("page reflects the request: %s", body)
	}
}

func TestPortForward_StoppedSharing(t *testing.T) {
	e := newPortEnv(t)
	port := e.openForward(t)
	e.stub.mu.Lock()
	e.stub.proxyStatus, e.stub.proxyMarker = 403, true
	e.stub.mu.Unlock()
	resp, body := do(t, port, "", map[string]string{"X-Portty": "1"})
	checkPage(t, resp, body, "127.0.0.1 stopped sharing", "port "+strconv.Itoa(port))
	if hit := e.stub.lastHit(t); hit.hdr.Get("X-Portty") != "" {
		t.Error("the page's X-Portty header reached the stub")
	}
	if resp.Header.Get("X-Portty") != "" {
		t.Error("X-Portty reached the browser")
	}
	waitClosed(t, port)
	if _, got, _ := e.open(t, "s1", localhostURL(port, "/"), false); got.Status != PortStatusConsent {
		t.Fatalf("after the stopped page = %+v, want consent", got)
	}
}

func TestPortForward_NothingAnswering(t *testing.T) {
	e := newPortEnv(t)
	port := e.openForward(t)
	e.stub.mu.Lock()
	e.stub.proxyStatus, e.stub.proxyMarker = 502, true
	e.stub.mu.Unlock()
	resp, body := do(t, port, "", nil)
	checkPage(t, resp, body, "Nothing is answering on 127.0.0.1's port "+strconv.Itoa(port))
	if resp, _ := do(t, port, "", nil); resp.StatusCode != http.StatusBadGateway || e.stub.hitCount() != 2 {
		t.Fatalf("second request: %d with %d stub hits, want the forward kept open", resp.StatusCode, e.stub.hitCount())
	}
}

func TestPortForward_HostGone(t *testing.T) {
	e := newPortEnv(t)
	port := e.openForward(t)
	e.stub.srv.Close()
	resp, body := do(t, port, "", nil)
	checkPage(t, resp, body, "stopped sharing")
	waitClosed(t, port)
}

// dialRemoteWS opens the webview-side stream to the daemon's relay proxy.
func dialRemoteWS(t *testing.T, relayURL, sid string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws://"+strings.TrimPrefix(relayURL, "http://")+"/api/relay/remote/"+sid+"/ws", nil)
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	t.Cleanup(func() { conn.CloseNow() })
	return conn
}

// streamClose has the stub close the proxied session stream with code and
// reason, and waits until the proxy forwarded it to the viewer.
func (e *portEnv) streamClose(t *testing.T, code websocket.StatusCode, reason string) {
	t.Helper()
	relaySrv := httptest.NewServer(e.api.RelayHandler())
	t.Cleanup(relaySrv.Close)
	client := dialRemoteWS(t, relaySrv.URL, "s1")
	var host *websocket.Conn
	select {
	case host = <-e.stub.wsConns:
	case <-time.After(5 * time.Second):
		t.Fatal("the proxy never reached the stub's stream")
	}
	go host.Close(code, reason)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _, err := client.Read(ctx)
	if websocket.CloseStatus(err) != code {
		t.Fatalf("viewer saw %v, want close %d", err, code)
	}
}

func TestPortForward_StreamCloseRevokes(t *testing.T) {
	for _, reason := range []string{"share stopped", "session ended"} {
		t.Run(reason, func(t *testing.T) {
			logs := captureLog(t)
			e := newPortEnv(t)
			port := e.openForward(t)
			e.streamClose(t, websocket.StatusNormalClosure, reason)
			waitClosed(t, port) // no request was ever sent to the forward
			if !strings.Contains(string(logs.bytes()), "outcome=revoked") {
				t.Errorf("no outcome=revoked line in %q", string(logs.bytes()))
			}
			if _, got, _ := e.open(t, "s1", localhostURL(port, "/"), false); got.Status != PortStatusConsent {
				t.Fatalf("click after = %+v, want consent", got)
			}
			if e.stub.hitCount() != 0 {
				t.Fatal("a request reached the stub")
			}
		})
	}
	t.Run("service restart", func(t *testing.T) {
		e := newPortEnv(t)
		port := e.openForward(t)
		e.streamClose(t, websocket.StatusServiceRestart, "service restart")
		if !listening(port) {
			t.Fatal("a 1012 close closed the forward")
		}
		if _, got, _ := e.open(t, "s1", localhostURL(port, "/"), false); got.Status != PortStatusOpen {
			t.Fatalf("click after = %+v, want open (approval still live)", got)
		}
	})
}

// connect posts /peers/connect for session s1 against the stub host.
func (e *portEnv) connect(t *testing.T) {
	t.Helper()
	peer := tailnet.Peer{Hostname: "office-air", DNSName: "office-air.tail1234.ts.net.", Online: true}
	e.api.SetPeerSeamsForTest(
		func(context.Context) ([]tailnet.Peer, error) { return []tailnet.Peer{peer}, nil },
		nil,
		func(tailnet.Peer) string { return e.stub.srv.URL },
	)
	b, _ := json.Marshal(PeerConnectRequest{Host: "office-air", Session: "s1"})
	resp, err := e.client.Post("http://daemon/peers/connect", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST /peers/connect = %d %s", resp.StatusCode, raw)
	}
}

func (e *portEnv) setStubCap(c string) {
	e.stub.mu.Lock()
	e.stub.capVal = c
	e.stub.mu.Unlock()
}

func TestPortForward_NewCapRevokes(t *testing.T) {
	e := newPortEnv(t)
	port := e.openForward(t)
	e.connect(t) // the same capability: nothing is revoked
	if !listening(port) {
		t.Fatal("a reconnect with the same capability closed the forward")
	}
	if _, got, _ := e.open(t, "s1", localhostURL(port, "/"), false); got.Status != PortStatusOpen {
		t.Fatalf("click after the same capability = %+v, want open", got)
	}
	e.setStubCap("CAP2") // a re-mint
	e.connect(t)
	waitClosed(t, port)
	if _, got, _ := e.open(t, "s1", localhostURL(port, "/"), false); got.Status != PortStatusConsent {
		t.Fatalf("click after a new capability = %+v, want consent", got)
	}
	if e.stub.hitCount() != 0 {
		t.Fatal("a request reached the stub")
	}
}

func TestPortForward_StopReshareClickAsksConsent(t *testing.T) {
	e := newPortEnv(t)
	port := e.openForward(t)
	e.streamClose(t, websocket.StatusNormalClosure, "share stopped") // stop
	e.setStubCap("CAP2")
	e.connect(t) // re-share
	if _, got, _ := e.open(t, "s1", localhostURL(port, "/"), false); got.Status != PortStatusConsent {
		t.Fatalf("click = %+v, want consent", got)
	}
	if e.stub.hitCount() != 0 {
		t.Fatal("a request reached the stub")
	}
}

func TestPortForward_CloseAll(t *testing.T) {
	t.Run("Stop", func(t *testing.T) {
		e := newPortEnv(t)
		port := e.openForward(t)
		_ = e.api.Stop()
		waitClosed(t, port)
	})
	t.Run("handoff", func(t *testing.T) {
		e := newPortEnv(t)
		port := e.openForward(t)
		e.api.closeListenerCopies()
		waitClosed(t, port)
		if len(e.api.portForwards.approvals) != 0 {
			t.Fatal("an approval survived the handoff")
		}
	})
}

func TestPortForward_NotOnRelayHandler(t *testing.T) {
	e := newPortEnv(t)
	h := e.api.RelayHandler()
	for _, method := range []string{http.MethodPost, http.MethodGet} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, "/peers/ports", strings.NewReader(`{"sessionId":"s1","url":"http://localhost:1/","consent":true}`)))
		if rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s /peers/ports on the relay handler = %d, want 404 or 405", method, rec.Code)
		}
	}
}

func TestPortForward_NoDiskWriteAPIs(t *testing.T) {
	banned := regexp.MustCompile(`os\.WriteFile|ioutil\.WriteFile|os\.Create|os\.OpenFile|os\.Mkdir`)
	comments := regexp.MustCompile(`(?m)^\s*//.*$`)
	for _, name := range []string{"portforward.go", "portforward_api.go"} {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if loc := banned.FindIndex(comments.ReplaceAll(src, nil)); loc != nil {
			t.Errorf("%s references a disk-write API", name)
		}
	}
}

func TestPortForward_LogsNoSecrets(t *testing.T) {
	logs := captureLog(t)
	e := newPortEnv(t)
	if err := e.api.remoteCaps.Put("s1", e.stub.srv.URL, "", "CAP-SECRET"); err != nil {
		t.Fatal(err)
	}
	port := e.openForward(t)
	if st, _ := get(t, port, "/hmr?token=SECRET-TARGET"); st != http.StatusOK {
		t.Fatalf("GET = %d", st)
	}
	if hit := e.stub.lastHit(t); hit.hdr.Get("X-Portty-Cap") != "CAP-SECRET" {
		t.Fatalf("the stub saw cap %q", hit.hdr.Get("X-Portty-Cap"))
	}
	out := string(logs.bytes())
	if strings.Contains(out, "SECRET") {
		t.Errorf("log leaks a secret: %q", out)
	}
	if !strings.Contains(out, "port forward: session=s1 port="+strconv.Itoa(port)+" outcome=open") {
		t.Errorf("no outcome=open line in %q", out)
	}
}

// A WebSocket upgrade goes through the forward like any request, carries the
// Portty headers, and is torn down when the forward closes.
func TestPortForward_WebSocketUpgrade(t *testing.T) {
	e := newPortEnv(t)
	port := e.openForward(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws://127.0.0.1:"+strconv.Itoa(port)+"/hmr?token=x", nil)
	if err != nil {
		t.Fatalf("upgrade through the forward: %v", err)
	}
	defer conn.CloseNow()
	if err := conn.Write(ctx, websocket.MessageText, []byte("hi")); err != nil {
		t.Fatal(err)
	}
	if _, data, err := conn.Read(ctx); err != nil || string(data) != "hi" {
		t.Fatalf("echo = %q %v", data, err)
	}
	hit := e.stub.lastHit(t)
	if hit.hdr.Get("X-Portty-Target") != "/hmr?token=x" || hit.hdr.Get("X-Portty-Cap") != "CAP" || hit.hdr.Get("X-Portty-Port-Token") != e.stub.token() {
		t.Errorf("upgrade request carried target %q cap %q token %q", hit.hdr.Get("X-Portty-Target"), hit.hdr.Get("X-Portty-Cap"), hit.hdr.Get("X-Portty-Port-Token"))
	}
	e.api.portForwards.CloseAll()
	rctx, rcancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer rcancel()
	if _, _, err := conn.Read(rctx); err == nil || rctx.Err() != nil {
		t.Fatalf("the tunnel survived the forward closing: %v", err)
	}
}
