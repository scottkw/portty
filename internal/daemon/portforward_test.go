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
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
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
}

func newStubHost(t *testing.T) *stubHost {
	t.Helper()
	s := &stubHost{}
	mux := http.NewServeMux()
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
