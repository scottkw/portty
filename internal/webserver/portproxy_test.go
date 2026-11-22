package webserver

// Phase 13 (URL-01, URL-02): the host's port grant and proxy routes. Every
// refusal row carries a tripwire: a listener on the requested loopback port
// that counts connections, because "refused" must also mean "never dialled".
// Nothing here touches the tailnet: WhoIs is a fake and the share server is a
// loopback server with a test certificate.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
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
	status int
	hdr    http.Header
	body   string
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
	return answer{resp.StatusCode, resp.Header, string(raw)}
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
