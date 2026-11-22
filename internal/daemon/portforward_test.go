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
	"strconv"
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
	api, _, socket := testDaemon(t)
	stub := newStubHost(t)
	api.SetPeerClientForTest(stub.srv.Client())
	if err := api.remoteCaps.Put("s1", stub.srv.URL, "", "CAP"); err != nil {
		t.Fatal(err)
	}
	return &portEnv{api: api, stub: stub, socket: socket, client: unixClient(socket)}
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
