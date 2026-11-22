package webserver

// Phase 13 (URL-01, URL-02): the host's port grant and proxy routes. Every
// refusal row carries a tripwire: a listener on the requested loopback port
// that counts connections, because "refused" must also mean "never dialled".
// Nothing here touches the tailnet: WhoIs is a fake and the share server is a
// loopback server with a test certificate.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/capability"
	"github.com/scottkw/portty/internal/relay"
	"github.com/scottkw/portty/internal/tailnet"
	"tailscale.com/tailcfg"
)

// tripwire counts connections accepted on one loopback port, on both families
// (the host dials [::1] first, then 127.0.0.1).
type tripwire struct {
	port int
	n    atomic.Int64
	lns  []net.Listener
}

// newTripwire listens on a port free on 127.0.0.1 and, when this Mac has IPv6
// loopback, on [::1] too.
func newTripwire(t *testing.T) *tripwire {
	t.Helper()
	for i := 0; i < 50; i++ {
		l4, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		port := l4.Addr().(*net.TCPAddr).Port
		tw := &tripwire{port: port, lns: []net.Listener{l4}}
		if l6, err := net.Listen("tcp6", "[::1]:"+strconv.Itoa(port)); err == nil {
			tw.lns = append(tw.lns, l6)
		} else if hasIPv6Loopback() {
			l4.Close()
			continue // the v6 port is taken by someone else; try another
		}
		tw.serve()
		t.Cleanup(tw.close)
		return tw
	}
	t.Fatal("no loopback port free")
	return nil
}

// tripwireOn is newTripwire for one fixed port; nil when the port is taken.
func tripwireOn(t *testing.T, port int) *tripwire {
	t.Helper()
	l4, err := net.Listen("tcp4", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		return nil
	}
	tw := &tripwire{port: port, lns: []net.Listener{l4}}
	if l6, err := net.Listen("tcp6", "[::1]:"+strconv.Itoa(port)); err == nil {
		tw.lns = append(tw.lns, l6)
	}
	tw.serve()
	t.Cleanup(tw.close)
	return tw
}

func (tw *tripwire) serve() {
	for _, l := range tw.lns {
		go func(l net.Listener) {
			for {
				c, err := l.Accept()
				if err != nil {
					return
				}
				tw.n.Add(1)
				c.Close()
			}
		}(l)
	}
}

func (tw *tripwire) close() {
	for _, l := range tw.lns {
		l.Close()
	}
}

func hasIPv6Loopback() bool {
	l, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		return false
	}
	l.Close()
	return true
}

// closedPort returns a port nothing listens on.
func closedPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// ppEnv is a share server with one live session and a write capability for it.
type ppEnv struct {
	ws     *WebServer
	client *http.Client
	sid    string
	ptyW   *io.PipeWriter
	capTok string
}

type ppOpts struct {
	who    whoIsClient
	denied []int
}

const ppSID = "pp-session"

// newPPEnv builds a started share server for ppSID. WhoIs and the deny list
// are installed before Start, as in production.
func newPPEnv(t *testing.T, o ppOpts) *ppEnv {
	t.Helper()
	manager := relay.NewHubManager()
	ptyR, ptyW := io.Pipe()
	_, inW := io.Pipe()
	manager.Create(ppSID, ptyR, inW, nil)
	tlsCfg, client := selfSignedTLSForTest(t)
	ws, err := NewWebServer(Config{BindIP: "127.0.0.1", Port: 0, FQDN: "127.0.0.1", TLSConfig: tlsCfg}, manager)
	if err != nil {
		t.Fatal(err)
	}
	if o.who == nil {
		o.who = SameUserWhoIsForTest()
	}
	ws.SetWhoIsClientForTest(o.who)
	ws.SetDeniedPorts(o.denied...)
	ws.SetSigningKey(capTestKey)
	if err := ws.Start(); err != nil {
		t.Fatal(err)
	}
	ws.EnableSession(ppSID)
	t.Cleanup(func() {
		_ = ws.Stop()
		manager.Shutdown()
		_ = ptyW.Close()
		_ = inW.Close()
	})
	c := *client
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	e := &ppEnv{ws: ws, client: &c, sid: ppSID, ptyW: ptyW}
	e.capTok = issueCapFor(t, ws, ppSID, "read,write")
	return e
}

// print makes the session print a dev-server banner for each port and waits
// until the hub's printed check sees it.
func (e *ppEnv) print(t *testing.T, ports ...int) {
	t.Helper()
	hub, ok := e.ws.manager.Get(e.sid)
	if !ok {
		t.Fatal("no hub")
	}
	for _, p := range ports {
		if _, err := io.WriteString(e.ptyW, fmt.Sprintf("Serving on http://localhost:%d/\r\n", p)); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(5 * time.Second)
		for !hub.PortPrinted(p) {
			if time.Now().After(deadline) {
				t.Fatalf("hub never saw port %d", p)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// ownPort is the share server's own listener port.
func (e *ppEnv) ownPort(t *testing.T) int {
	t.Helper()
	u, err := url.Parse(e.ws.BaseURL())
	if err != nil {
		t.Fatal(err)
	}
	p, _ := strconv.Atoi(u.Port())
	return p
}

// answer is what a refusal is compared by: status, headers without Date, body.
type answer struct {
	status     int
	hdr        http.Header
	body       string
	protoMajor int
}

func (a answer) sig() string {
	h := a.hdr.Clone()
	h.Del("Date")
	var keys []string
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	fmt.Fprintf(&b, "%d\n", a.status)
	for _, k := range keys {
		fmt.Fprintf(&b, "%s: %s\n", k, strings.Join(h[k], ","))
	}
	return b.String() + "\n" + a.body
}

func (e *ppEnv) send(t *testing.T, method, path string, hdr http.Header, body io.Reader) answer {
	t.Helper()
	req, err := http.NewRequest(method, e.ws.BaseURL()+path, body)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range hdr {
		req.Header[k] = v
	}
	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return answer{resp.StatusCode, resp.Header, string(raw), resp.ProtoMajor}
}

func grantHdr(capTok, contentType string) http.Header {
	h := http.Header{}
	if contentType != "" {
		h.Set("Content-Type", contentType)
	}
	if capTok != "" {
		h.Set("X-Portty-Cap", capTok)
	}
	return h
}

func (e *ppEnv) grant(t *testing.T, port int, capTok string) answer {
	t.Helper()
	return e.send(t, http.MethodPost, "/api/peer/sessions/"+e.sid+"/ports", grantHdr(capTok, "application/json"),
		strings.NewReader(fmt.Sprintf(`{"port": %d}`, port)))
}

// assertRefusedAnswer: 403, the marker, the fixed body, no CORS.
func assertRefusedAnswer(t *testing.T, a answer) {
	t.Helper()
	if a.status != http.StatusForbidden || a.hdr.Get(tailnet.MarkerHeader) != tailnet.MarkerValue || a.body != "forbidden\n" {
		t.Fatalf("answer = %d marker %q body %q; want 403 with the marker and %q", a.status, a.hdr.Get(tailnet.MarkerHeader), a.body, "forbidden\n")
	}
	for k := range a.hdr {
		if strings.HasPrefix(k, "Access-Control-") {
			t.Fatalf("refusal carries %s", k)
		}
	}
}

func TestPortGrant_Granted(t *testing.T) {
	tw := newTripwire(t)
	e := newPPEnv(t, ppOpts{})
	e.print(t, tw.port)

	a := e.grant(t, tw.port, e.capTok)
	if a.status != http.StatusOK || a.hdr.Get("Cache-Control") != "no-store" || !strings.HasPrefix(a.hdr.Get("Content-Type"), "application/json") {
		t.Fatalf("grant = %d %v", a.status, a.hdr)
	}
	for k := range a.hdr {
		if strings.HasPrefix(k, "Access-Control-") {
			t.Errorf("grant carries %s", k)
		}
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal([]byte(a.body), &out); err != nil || out.Token == "" {
		t.Fatalf("body %q: %v", a.body, err)
	}
	claims, err := capability.VerifyPort(out.Token, capTestKey)
	if err != nil {
		t.Fatalf("VerifyPort: %v", err)
	}
	if claims.SID != e.sid || claims.Port != tw.port || claims.GrantID != "grant-"+e.sid+"-read,write" {
		t.Errorf("claims = %+v", claims)
	}
	if n := tw.n.Load(); n != 0 {
		t.Errorf("a grant dialled the port: %d connections", n)
	}
}

// grantCase describes one refused grant. Zero values are the happy request.
type grantCase struct {
	name        string
	who         tailcfg.UserID // 0 = same user
	cap         string         // "" write, "read", "none", "garbage", "othersession"
	revoke      bool
	disable     bool
	unprinted   bool
	rawBody     string // overrides the JSON body when set
	noBody      bool
	contentType string // "" = application/json
	deny        string // "own", "7444", "set"
}

func TestPortGrant_Refusals(t *testing.T) {
	cases := []grantCase{
		{name: "another user", who: 7},
		{name: "no capability", cap: "none"},
		{name: "garbage capability", cap: "garbage"},
		{name: "capability for another session", cap: "othersession"},
		{name: "read-only capability", cap: "read"},
		{name: "revoked grant", revoke: true},
		{name: "disabled session", disable: true},
		{name: "port never printed", unprinted: true},
		{name: "port 0", rawBody: `{"port": 0}`},
		{name: "port 65536", rawBody: `{"port": 65536}`},
		{name: "port not an integer", rawBody: `{"port": "80"}`},
		{name: "port fractional", rawBody: `{"port": 80.5}`},
		{name: "missing body", noBody: true},
		{name: "text/plain content type", contentType: "text/plain"},
		{name: "body over 1 KiB", rawBody: `{"port": %PORT%, "pad": "` + strings.Repeat("a", 1100) + `"}`},
		{name: "the share server's own port", deny: "own"},
		{name: "port 7444", deny: "7444"},
		{name: "a SetDeniedPorts port", deny: "set"},
	}
	var ref string
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tw := newTripwire(t)
			port := tw.port
			var denied []int
			var trip = tw
			switch c.deny {
			case "7444":
				port = tailnet.DefaultProbePort
				trip = tripwireOn(t, port) // best effort: the port may be taken
			case "set":
				denied = []int{port}
			}
			who := whoIsAs(testSelfUser)
			if c.who != 0 {
				who = whoIsAs(c.who)
			}
			e := newPPEnv(t, ppOpts{who: who, denied: denied})
			if c.deny == "own" {
				port, trip = e.ownPort(t), nil
			}
			if !c.unprinted {
				e.print(t, port)
			}

			capTok := e.capTok
			switch c.cap {
			case "read":
				capTok = issueCapFor(t, e.ws, e.sid, "read")
			case "none":
				capTok = ""
			case "garbage":
				capTok = "not.a-token"
			case "othersession":
				capTok = issueCapFor(t, e.ws, "other-session", "read,write")
			}
			if c.revoke {
				e.ws.ClearGrants(e.sid)
			}
			if c.disable {
				e.ws.DisableSession(e.sid)
			}

			ct := c.contentType
			if ct == "" {
				ct = "application/json"
			}
			var body io.Reader
			switch {
			case c.noBody:
			case c.rawBody != "":
				body = strings.NewReader(strings.ReplaceAll(c.rawBody, "%PORT%", strconv.Itoa(port)))
			default:
				body = strings.NewReader(fmt.Sprintf(`{"port": %d}`, port))
			}
			a := e.send(t, http.MethodPost, "/api/peer/sessions/"+e.sid+"/ports", grantHdr(capTok, ct), body)
			assertRefusedAnswer(t, a)
			if ref == "" {
				ref = a.sig()
			} else if a.sig() != ref {
				t.Errorf("answer differs from the first refusal:\n%s\nvs\n%s", a.sig(), ref)
			}
			if trip != nil {
				if n := trip.n.Load(); n != 0 {
					t.Errorf("tripwire on %d counted %d connections", trip.port, n)
				}
			}
		})
	}
}

// An unprinted port that is listening and one that is closed must be
// indistinguishable, or the grant route is a port scanner.
func TestPortGrant_IdenticalOpenOrClosed(t *testing.T) {
	open := newTripwire(t)
	closed := closedPort(t)
	e := newPPEnv(t, ppOpts{})

	a, b := e.grant(t, open.port, e.capTok), e.grant(t, closed, e.capTok)
	assertRefusedAnswer(t, a)
	if a.sig() != b.sig() {
		t.Errorf("open and closed answers differ:\n%s\nvs\n%s", a.sig(), b.sig())
	}
	if n := open.n.Load(); n != 0 {
		t.Errorf("tripwire counted %d connections", n)
	}
}

func TestPortGrant_LocalFault(t *testing.T) {
	e := newPPEnv(t, ppOpts{who: &fakeWhoIs{err: errors.New("tailscaled down")}})
	a := e.grant(t, 8000, e.capTok)
	if a.status != http.StatusServiceUnavailable || a.hdr.Get(tailnet.MarkerHeader) != "" {
		t.Fatalf("answer = %d marker %q; want 503 with no marker", a.status, a.hdr.Get(tailnet.MarkerHeader))
	}
}

func TestPortGrant_LogsNoSecrets(t *testing.T) {
	var buf syncBuf
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })

	tw := newTripwire(t)
	e := newPPEnv(t, ppOpts{})
	e.print(t, tw.port)
	a := e.grant(t, tw.port, e.capTok)
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal([]byte(a.body), &out); err != nil || out.Token == "" {
		t.Fatalf("grant = %d %q", a.status, a.body)
	}
	assertRefusedAnswer(t, e.grant(t, closedPort(t), e.capTok))

	logs := buf.String()
	for _, secret := range []string{e.capTok, out.Token} {
		if strings.Contains(logs, secret) {
			t.Errorf("log leaks %q", secret)
		}
	}
	for _, want := range []string{"portproxy: grant session=" + relay.DiagID(e.sid), "outcome=granted", "outcome=refused"} {
		if !strings.Contains(logs, want) {
			t.Errorf("log lacks %q:\n%s", want, logs)
		}
	}
}

// ---- the proxy route ----

// portToken signs a port token the way the grant route does, for a chosen
// session, port, grant and key (a hand-minted one for the deny rows).
func portToken(t *testing.T, sid string, port int, grantID string, key []byte) string {
	t.Helper()
	tok, err := capability.SignPort(capability.PortClaims{
		SID: sid, Port: port, IAT: time.Now().Unix(), GrantID: grantID,
		Kind: capability.PortTokenKind, V: 1,
	}, key)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

const writeGrantID = "grant-" + ppSID + "-read,write"

// proxyReq is one request to the proxy route; the zero value of every field
// but port is filled in as a good request by good().
type proxyReq struct {
	method   string
	portStr  string
	capTok   string
	token    string
	target   string
	noTarget bool
	hdr      http.Header
	body     io.Reader
}

// good is a request the host should proxy: write capability, a token for port,
// target /x.
func (e *ppEnv) good(t *testing.T, port int) proxyReq {
	return proxyReq{
		method:  http.MethodGet,
		portStr: strconv.Itoa(port),
		capTok:  e.capTok,
		token:   portToken(t, e.sid, port, writeGrantID, capTestKey),
		target:  "/x",
	}
}

func (e *ppEnv) proxy(t *testing.T, r proxyReq) answer {
	t.Helper()
	h := http.Header{}
	for k, v := range r.hdr {
		h[k] = v
	}
	if r.capTok != "" {
		h.Set("X-Portty-Cap", r.capTok)
	}
	if r.token != "" {
		h.Set("X-Portty-Port-Token", r.token)
	}
	if !r.noTarget {
		h.Set("X-Portty-Target", r.target)
	}
	return e.send(t, r.method, "/api/peer/sessions/"+e.sid+"/ports/"+r.portStr, h, r.body)
}

func TestPortProxy_Refusals(t *testing.T) {
	otherKey := bytes.Repeat([]byte{9}, 32)
	cases := []struct {
		name   string
		who    tailcfg.UserID
		denied bool
		prep   func(t *testing.T, e *ppEnv, r *proxyReq, port int)
	}{
		{name: "no port token", prep: func(t *testing.T, e *ppEnv, r *proxyReq, port int) { r.token = "" }},
		{name: "token for another port", prep: func(t *testing.T, e *ppEnv, r *proxyReq, port int) {
			r.token = portToken(t, e.sid, port+1, writeGrantID, capTestKey)
		}},
		{name: "token for another session", prep: func(t *testing.T, e *ppEnv, r *proxyReq, port int) {
			r.token = portToken(t, "other-session", port, writeGrantID, capTestKey)
		}},
		{name: "token from before a key change", prep: func(t *testing.T, e *ppEnv, r *proxyReq, port int) {
			old := portToken(t, e.sid, port, writeGrantID, capTestKey)
			e.ws.SetSigningKey(otherKey)
			cap, err := capability.Sign(capability.Claims{SID: e.sid, Perms: "read,write", IAT: time.Now().Unix(), GrantID: "g-new", V: 1}, otherKey)
			if err != nil {
				t.Fatal(err)
			}
			e.ws.AddGrant(e.sid, "g-new")
			r.capTok, r.token = cap, old
		}},
		{name: "token whose grant was cleared and replaced", prep: func(t *testing.T, e *ppEnv, r *proxyReq, port int) {
			e.ws.ClearGrants(e.sid)
			r.capTok = issueCapFor(t, e.ws, e.sid, "write,read") // a different grant id
		}},
		{name: "disabled session", prep: func(t *testing.T, e *ppEnv, r *proxyReq, port int) { e.ws.DisableSession(e.sid) }},
		{name: "no capability", prep: func(t *testing.T, e *ppEnv, r *proxyReq, port int) { r.capTok = "" }},
		{name: "read-only capability", prep: func(t *testing.T, e *ppEnv, r *proxyReq, port int) {
			r.capTok = issueCapFor(t, e.ws, e.sid, "read")
			r.token = portToken(t, e.sid, port, "grant-"+e.sid+"-read", capTestKey) // only the permission refuses
		}},
		{name: "another user", who: 7},
		{name: "denied port with a hand-minted token", denied: true},
		{name: "missing target", prep: func(t *testing.T, e *ppEnv, r *proxyReq, port int) { r.noTarget = true }},
		{name: "target x", prep: func(t *testing.T, e *ppEnv, r *proxyReq, port int) { r.target = "x" }},
		{name: "absolute target", prep: func(t *testing.T, e *ppEnv, r *proxyReq, port int) { r.target = "http://evil.example/" }},
		{name: "9000-byte target", prep: func(t *testing.T, e *ppEnv, r *proxyReq, port int) { r.target = "/" + strings.Repeat("a", 9000) }},
		{name: "port 8000x", prep: func(t *testing.T, e *ppEnv, r *proxyReq, port int) { r.portStr = "8000x" }},
		{name: "port 0", prep: func(t *testing.T, e *ppEnv, r *proxyReq, port int) { r.portStr = "0" }},
		{name: "port 08000", prep: func(t *testing.T, e *ppEnv, r *proxyReq, port int) { r.portStr = "08000" }},
		{name: "port 65536", prep: func(t *testing.T, e *ppEnv, r *proxyReq, port int) { r.portStr = "65536" }},
	}
	var ref string
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tw := newTripwire(t)
			who := whoIsAs(testSelfUser)
			if c.who != 0 {
				who = whoIsAs(c.who)
			}
			var denied []int
			if c.denied {
				denied = []int{tw.port}
			}
			e := newPPEnv(t, ppOpts{who: who, denied: denied})
			r := e.good(t, tw.port)
			if c.prep != nil {
				c.prep(t, e, &r, tw.port)
			}
			a := e.proxy(t, r)
			assertRefusedAnswer(t, a)
			if ref == "" {
				ref = a.sig()
			} else if a.sig() != ref {
				t.Errorf("answer differs from the first refusal:\n%s\nvs\n%s", a.sig(), ref)
			}
			if n := tw.n.Load(); n != 0 {
				t.Errorf("tripwire on %d counted %d connections", tw.port, n)
			}
		})
	}
}

// seen is what a dev server recorded about the last request.
type seen struct {
	mu         sync.Mutex
	method     string
	uri, host  string
	proto      string
	hdr        http.Header
	body       string
	respHeader func(http.Header)
}

func (s *seen) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		s.method, s.uri, s.host, s.proto, s.hdr, s.body = r.Method, r.RequestURI, r.Host, r.Proto, r.Header.Clone(), string(b)
		fn := s.respHeader
		s.mu.Unlock()
		w.Header().Set("X-Seen-Method", r.Method)
		if fn != nil {
			fn(w.Header())
		}
		w.Write(b) //nolint:errcheck
	})
}

func (s *seen) get() (method, uri, host, proto string, hdr http.Header, body string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.method, s.uri, s.host, s.proto, s.hdr, s.body
}

func TestPortProxy_PathAndHeaders(t *testing.T) {
	var rec seen
	rec.respHeader = func(h http.Header) {
		h.Set("X-Portty", "1")
		h.Set("X-Portty-Debug", "leak")
		h.Set("Content-Security-Policy", "default-src 'none'")
		h.Add("Set-Cookie", "sid=abc; Path=/")
	}
	dev := httptest.NewServer(rec.handler())
	defer dev.Close()
	port := dev.Listener.Addr().(*net.TCPAddr).Port
	e := newPPEnv(t, ppOpts{})
	own := "http://127.0.0.1:" + strconv.Itoa(port)

	const target = "/a//b/%2e%2e/c%2Fd?x=1&cap=page"
	r := e.good(t, port)
	r.target = target
	r.hdr = http.Header{
		"X-Forwarded-For":  {"1.2.3.4"},
		"X-Forwarded-Port": {"9"},
		"Forwarded":        {"for=1.2.3.4"},
		"X-Portty-Extra":   {"smuggled"},
	}
	a := e.proxy(t, r)
	if a.status != http.StatusOK || a.body != "" {
		t.Fatalf("proxy = %d %q", a.status, a.body)
	}
	_, uri, host, proto, hdr, _ := rec.get()
	if uri != target {
		t.Errorf("dev server RequestURI = %q, want %q byte for byte", uri, target)
	}
	if host != "localhost:"+strconv.Itoa(port) {
		t.Errorf("dev server Host = %q, want localhost:%d", host, port)
	}
	if proto != "HTTP/1.1" || a.protoMajor != 1 {
		t.Errorf("hop proto = %q, client proto major = %d; want HTTP/1.1", proto, a.protoMajor)
	}
	for k := range hdr {
		if strings.HasPrefix(k, "X-Portty") || strings.HasPrefix(k, "X-Forwarded") || k == "Forwarded" {
			t.Errorf("dev server saw header %s", k)
		}
	}
	// Response side: the dev server's marker never reaches the viewer, its own
	// CSP and cookies do, and the share server adds none of its own.
	for k := range a.hdr {
		if strings.HasPrefix(k, "X-Portty") {
			t.Errorf("response carries %s", k)
		}
		if strings.HasPrefix(k, "Access-Control-") {
			t.Errorf("response carries %s", k)
		}
	}
	if got := a.hdr.Values("Content-Security-Policy"); len(got) != 1 || got[0] != "default-src 'none'" {
		t.Errorf("Content-Security-Policy = %q, want only the dev server's", got)
	}
	if got := a.hdr.Values("Set-Cookie"); len(got) != 1 || got[0] != "sid=abc; Path=/" {
		t.Errorf("Set-Cookie = %q", got)
	}

	// Origin: only the typed-127.0.0.1 form of this very port is rewritten.
	for _, o := range []struct{ in, want string }{
		{own, "http://localhost:" + strconv.Itoa(port)},
		{"https://evil.example", "https://evil.example"},
		{"http://localhost:" + strconv.Itoa(port), "http://localhost:" + strconv.Itoa(port)},
		{"http://127.0.0.1:" + strconv.Itoa(port+1), "http://127.0.0.1:" + strconv.Itoa(port+1)},
		{"", ""},
	} {
		r := e.good(t, port)
		if o.in != "" {
			r.hdr = http.Header{"Origin": {o.in}}
		}
		if a := e.proxy(t, r); a.status != http.StatusOK {
			t.Fatalf("Origin %q: status %d", o.in, a.status)
		}
		_, _, _, _, hdr, _ := rec.get()
		if got := hdr.Get("Origin"); got != o.want {
			t.Errorf("Origin %q reached the dev server as %q, want %q", o.in, got, o.want)
		}
	}
}

// listenBoth listens on one port on 127.0.0.1 and [::1]; false when this Mac
// has no IPv6 loopback.
func listenBoth(t *testing.T) (l4, l6 net.Listener, ok bool) {
	t.Helper()
	if !hasIPv6Loopback() {
		return nil, nil, false
	}
	for i := 0; i < 50; i++ {
		l4, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		l6, err := net.Listen("tcp6", "[::1]:"+strconv.Itoa(l4.Addr().(*net.TCPAddr).Port))
		if err == nil {
			t.Cleanup(func() { l4.Close(); l6.Close() })
			return l4, l6, true
		}
		l4.Close()
	}
	t.Fatal("no port free on both loopbacks")
	return nil, nil, false
}

// serveOn serves handler on an already-open listener, closed with the test.
func serveOn(t *testing.T, l net.Listener, h http.Handler) {
	t.Helper()
	s := httptest.NewUnstartedServer(h)
	s.Listener.Close()
	s.Listener = l
	s.Start()
	t.Cleanup(s.Close)
}

func TestPortProxy_DialOrder(t *testing.T) {
	e := newPPEnv(t, ppOpts{})
	body := func(port int) answer { return e.proxy(t, e.good(t, port)) }
	text := func(s string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, s) })
	}

	t.Run("::1 only", func(t *testing.T) {
		if !hasIPv6Loopback() {
			t.Skip("no IPv6 loopback on this Mac")
		}
		l, err := net.Listen("tcp6", "[::1]:0")
		if err != nil {
			t.Fatal(err)
		}
		serveOn(t, l, text("v6"))
		if a := body(l.Addr().(*net.TCPAddr).Port); a.status != 200 || a.body != "v6" {
			t.Fatalf("::1-only server = %d %q", a.status, a.body)
		}
	})
	t.Run("127.0.0.1 only", func(t *testing.T) {
		l, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		serveOn(t, l, text("v4"))
		if a := body(l.Addr().(*net.TCPAddr).Port); a.status != 200 || a.body != "v4" {
			t.Fatalf("127.0.0.1-only server = %d %q", a.status, a.body)
		}
	})
	t.Run("::1 wins when both answer", func(t *testing.T) {
		l4, l6, ok := listenBoth(t)
		if !ok {
			t.Skip("no IPv6 loopback on this Mac")
		}
		serveOn(t, l4, text("v4"))
		serveOn(t, l6, text("v6"))
		if a := body(l4.Addr().(*net.TCPAddr).Port); a.body != "v6" {
			t.Fatalf("body = %q, want the [::1] server first", a.body)
		}
	})
	t.Run("nothing answering", func(t *testing.T) {
		a := body(closedPort(t))
		if a.status != http.StatusBadGateway || a.hdr.Get(tailnet.MarkerHeader) != tailnet.MarkerValue || a.body != "nothing is answering\n" {
			t.Fatalf("answer = %d marker %q body %q", a.status, a.hdr.Get(tailnet.MarkerHeader), a.body)
		}
		for _, leak := range []string{"127.0.0.1", "::1", "dial", "refused", "connect"} {
			if strings.Contains(a.body, leak) {
				t.Errorf("502 body leaks %q", leak)
			}
		}
	})
}

func TestPortProxy_Methods(t *testing.T) {
	var rec seen
	dev := httptest.NewServer(rec.handler())
	defer dev.Close()
	port := dev.Listener.Addr().(*net.TCPAddr).Port
	e := newPPEnv(t, ppOpts{})

	for _, m := range []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD"} {
		r := e.good(t, port)
		r.method = m
		if m == "POST" || m == "PUT" || m == "PATCH" {
			r.body = strings.NewReader("payload-" + m)
		}
		a := e.proxy(t, r)
		method, _, _, _, _, body := rec.get()
		if a.status != http.StatusOK || a.hdr.Get("X-Seen-Method") != m || method != m {
			t.Errorf("%s: status %d, dev server saw %q", m, a.status, method)
		}
		if m == "POST" || m == "PUT" || m == "PATCH" {
			if body != "payload-"+m || a.body != "payload-"+m {
				t.Errorf("%s: body at dev server %q, echoed %q", m, body, a.body)
			}
		}
	}
}
