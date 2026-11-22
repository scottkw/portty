package daemon

// The viewing Mac's loopback forwards for remote `localhost` URLs. A forward
// listens on 127.0.0.1:P and [::1]:P only and reverse-proxies each browser
// request to the host's port route (hop 1 of two), carrying the capability and
// a port token in headers the daemon sets, never in a URL. Approvals (the
// user's per-(Mac, port) consent) and forwards live in memory only: nothing
// here writes to disk, and nothing logs a request target, capability or token.

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/scottkw/portty/internal/tailnet"
)

// maxForwardTarget is the longest request target the host's port route takes.
const maxForwardTarget = 8192

var (
	errPortInUse = errors.New("port in use")
	// errStopped and errNothingAnswering are what ModifyResponse hands the
	// ErrorHandler for a host 403 / 502 that carries the X-Portty marker.
	errStopped          = errors.New("host stopped sharing")
	errNothingAnswering = errors.New("nothing answering on the host's port")
)

// approvalKey is (Mac, port); host is the lower-cased base-URL hostname.
type approvalKey struct {
	host string
	port int
}

// approval records what the user agreed to: the session it was granted under
// and the SHA-256 of the capability the grant was made with. It is live only
// while that session still holds the same capability.
type approval struct {
	sid   string
	capFP [32]byte
}

// PortForwards holds the approvals and the forwards (keyed by port) of one
// daemon. Memory only.
type PortForwards struct {
	mu        sync.Mutex
	approvals map[approvalKey]approval
	forwards  map[int]*portForward
}

func newPortForwards() *PortForwards {
	return &PortForwards{approvals: map[approvalKey]approval{}, forwards: map[int]*portForward{}}
}

// dropStaleLocked reports whether key has a live approval. One whose session
// lost or replaced its capability is deleted, with its forward closed.
func (p *PortForwards) dropStaleLocked(a *API, key approvalKey) bool {
	ap, ok := p.approvals[key]
	if !ok {
		return false
	}
	if _, _, c, held := a.remoteCaps.Get(ap.sid); held && sha256.Sum256([]byte(c)) == ap.capFP {
		return true
	}
	delete(p.approvals, key)
	if f := p.forwards[key.port]; f != nil && f.host == key.host {
		p.closeLocked(f)
	}
	logPortOutcome(ap.sid, key.port, "revoked")
	return false
}

// closeLocked removes f and its approval and closes it.
func (p *PortForwards) closeLocked(f *portForward) {
	if p.forwards[f.port] == f {
		delete(p.forwards, f.port)
	}
	delete(p.approvals, approvalKey{host: f.host, port: f.port})
	f.close()
}

// closeForward closes f after a request found its share gone.
func (p *PortForwards) closeForward(f *portForward, outcome string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.forwards[f.port] != f {
		return
	}
	sid, _ := f.creds()
	p.closeLocked(f)
	logPortOutcome(sid, f.port, outcome)
}

// CloseAll closes every forward and drops every approval (daemon Stop, handoff).
func (p *PortForwards) CloseAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, f := range p.forwards {
		sid, _ := f.creds()
		p.closeLocked(f)
		logPortOutcome(sid, f.port, "closed")
	}
	clear(p.approvals)
}

// RevokeSession closes the forwards of host (compared case-insensitively) that
// were granted under sid and drops their approvals. It needs no request to
// reach a forward: a share-off or a re-minted link calls it when the session
// stream closes or the capability changes.
func (p *PortForwards) RevokeSession(host, sid string) {
	host = strings.ToLower(host)
	p.mu.Lock()
	defer p.mu.Unlock()
	for key, ap := range p.approvals {
		if key.host != host || ap.sid != sid {
			continue
		}
		delete(p.approvals, key)
		if f := p.forwards[key.port]; f != nil && f.host == host {
			p.closeLocked(f)
		}
		logPortOutcome(sid, key.port, "revoked")
	}
}

// portForward is one open forward.
type portForward struct {
	pfs   *PortForwards
	port  int
	host  string // lower-cased base-URL hostname
	label string // MagicDNS short label, for pages
	base  *url.URL

	mu         sync.Mutex // guards sid and token
	sid, token string

	proxy     *httputil.ReverseProxy
	srv       *http.Server
	transport *http.Transport
	lns       []net.Listener
	ctx       context.Context
	cancel    context.CancelFunc
	closed    atomic.Bool
	closeOnce sync.Once
}

func (f *portForward) creds() (sid, token string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sid, f.token
}

// rebind points the forward at a session and port token (a second click).
func (f *portForward) rebind(sid, token string) {
	f.mu.Lock()
	f.sid, f.token = sid, token
	f.mu.Unlock()
}

// close stops listening at once, aborts in-flight requests and upgraded
// tunnels, and shuts the server down in the background.
func (f *portForward) close() {
	f.closeOnce.Do(func() {
		f.closed.Store(true)
		f.cancel()
		for _, ln := range f.lns {
			_ = ln.Close()
		}
		f.rebind("", "")
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = f.srv.Shutdown(ctx)
			_ = f.srv.Close()
			f.transport.CloseIdleConnections()
		}()
	})
}

func (f *portForward) serve(lns []net.Listener) {
	f.lns = lns
	for _, ln := range lns {
		go f.srv.Serve(ln) //nolint:errcheck // ErrServerClosed or a closed listener
	}
}

// portFree reports errPortInUse unless dialing both loopbacks is refused. A
// strict SO_REUSEADDR=0 bind would also detect a holder but fails for about
// 30 s after the forward's own connections close. [::1] may be absent.
func portFree(port int) error {
	p := strconv.Itoa(port)
	for i, addr := range []string{"127.0.0.1:" + p, "[::1]:" + p} {
		c, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			_ = c.Close()
			return errPortInUse
		}
		switch {
		case errors.Is(err, syscall.ECONNREFUSED):
		case i == 1 && (errors.Is(err, syscall.EADDRNOTAVAIL) || errors.Is(err, syscall.ENETUNREACH) || errors.Is(err, syscall.EAFNOSUPPORT)):
		default:
			return errPortInUse // timeout, reset, anything else: fail closed
		}
	}
	return nil
}

// bindLoopback listens on 127.0.0.1:port and [::1]:port (both, so a request to
// `localhost` cannot reach a ::1-only holder). Any failure closes what opened.
func bindLoopback(port int) ([]net.Listener, error) {
	p := strconv.Itoa(port)
	l4, err := net.Listen("tcp4", "127.0.0.1:"+p)
	if err != nil {
		return nil, err
	}
	l6, err := net.Listen("tcp6", "[::1]:"+p)
	if err != nil {
		if errors.Is(err, syscall.EADDRNOTAVAIL) || errors.Is(err, syscall.EAFNOSUPPORT) {
			return []net.Listener{l4}, nil
		}
		_ = l4.Close()
		return nil, err
	}
	return []net.Listener{l4, l6}, nil
}

// forwardHostAllowed accepts only the Host a browser sends for localhost:port
// or 127.0.0.1:port (bare forms when the port is 80), so a DNS-rebinding page
// cannot use the forward.
func forwardHostAllowed(host string, port int) bool {
	p := ":" + strconv.Itoa(port)
	switch host {
	case "localhost" + p, "127.0.0.1" + p:
		return true
	case "localhost", "127.0.0.1":
		return port == 80
	}
	return false
}

// loopbackOrigin reports whether an Origin header names localhost, 127.0.0.1
// or [::1] on any port. "null" and anything unparseable are not loopback.
func loopbackOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	switch u.Hostname() {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}

// forwardTransport is the transport for hop 1: no total timeout (dev servers
// stream and upgrade), HTTP/1.1 only. In tests it clones the test client's.
func (a *API) forwardTransport(fqdn, ip string) *http.Transport {
	var t *http.Transport
	switch {
	case a.peerClientForTest != nil:
		t = a.peerClientForTest.Transport.(*http.Transport).Clone()
	case !tailnet.LiveAllowed(): // WR-07: a test binary never dials a tailnet peer
		t = &http.Transport{DialContext: func(context.Context, string, string) (net.Conn, error) {
			return nil, tailnet.ErrLiveTailnetInTest
		}}
	default:
		addr := ""
		if ip != "" {
			addr = net.JoinHostPort(ip, strconv.Itoa(tailnet.DefaultProbePort))
		}
		t = peerTransport(fqdn, addr)
	}
	t.DisableCompression = true
	t.TLSHandshakeTimeout = 10 * time.Second
	t.IdleConnTimeout = 30 * time.Second
	t.MaxIdleConnsPerHost = 8
	t.ResponseHeaderTimeout = 0
	t.ForceAttemptHTTP2 = false
	return t
}

// newPortForward builds a forward (not yet listening) bound to sid and token.
func (a *API) newPortForward(pfs *PortForwards, port int, host string, base *url.URL, fqdn, ip, label, sid, token string) *portForward {
	f := &portForward{pfs: pfs, port: port, host: host, label: label, base: base, sid: sid, token: token}
	f.ctx, f.cancel = context.WithCancel(context.Background())
	f.transport = a.forwardTransport(fqdn, ip)
	f.proxy = &httputil.ReverseProxy{
		Transport:     f.transport,
		FlushInterval: -1,
		ErrorLog:      log.New(io.Discard, "", 0),
		Rewrite: func(pr *httputil.ProxyRequest) {
			out := pr.Out
			for k := range out.Header {
				if strings.HasPrefix(http.CanonicalHeaderKey(k), "X-Portty") {
					delete(out.Header, k)
				}
			}
			sid, token := f.creds()
			_, _, capToken, _ := a.remoteCaps.Get(sid) // read per request: a peer-tab redial refreshes it
			out.URL.Scheme = "https"
			out.URL.Host = f.base.Host
			out.URL.Path = "/api/peer/sessions/" + sid + "/ports/" + strconv.Itoa(f.port)
			out.URL.RawPath = "/api/peer/sessions/" + url.PathEscape(sid) + "/ports/" + strconv.Itoa(f.port)
			out.URL.RawQuery = ""
			out.Host = ""
			out.Header.Set("X-Portty-Target", pr.In.URL.RequestURI())
			out.Header.Set("X-Portty-Cap", capToken)
			out.Header.Set("X-Portty-Port-Token", token)
		},
		ModifyResponse: func(resp *http.Response) error {
			if resp.Header.Get(tailnet.MarkerHeader) == tailnet.MarkerValue {
				switch resp.StatusCode {
				case http.StatusForbidden:
					return errStopped
				case http.StatusBadGateway:
					return errNothingAnswering
				}
			}
			for k := range resp.Header {
				if strings.HasPrefix(http.CanonicalHeaderKey(k), "X-Portty") {
					delete(resp.Header, k)
				}
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			switch {
			case errors.Is(err, errNothingAnswering):
				f.page(w, "nothing")
			case errors.Is(err, errStopped), errors.Is(err, syscall.ECONNREFUSED):
				f.page(w, "stopped")
				f.pfs.closeForward(f, "stopped")
			default:
				f.page(w, "unreachable")
			}
		},
	}
	f.srv = &http.Server{
		Handler:           f,
		ReadHeaderTimeout: 10 * time.Second,
		ErrorLog:          log.New(io.Discard, "", 0),
		BaseContext:       func(net.Listener) context.Context { return f.ctx },
	}
	return f
}

func refuseForward(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

// ServeHTTP is the forward's handler: closed check, Host check, cross-site
// check, then the hop-1 proxy. Refusals have fixed bodies that reflect nothing.
func (f *portForward) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if f.closed.Load() {
		f.page(w, "stopped")
		return
	}
	if !forwardHostAllowed(r.Host, f.port) {
		refuseForward(w, http.StatusForbidden, "forbidden\n")
		return
	}
	// A page on another site could drive the dev server through the forward
	// (no-cors requests, WebSocket hijacking) once Host and Origin look local.
	// A cross-site navigation (an OAuth callback) still passes.
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" && r.Header.Get("Sec-Fetch-Mode") != "navigate" {
		refuseForward(w, http.StatusForbidden, "forbidden\n")
		return
	}
	// Sec-Fetch-* depends on the browser; Origin is always sent on a WebSocket
	// handshake and a cross-origin POST. Any loopback port is fine (a multi-port
	// app), any other site is not.
	if o := r.Header.Get("Origin"); o != "" && r.Header.Get("Sec-Fetch-Mode") != "navigate" && !loopbackOrigin(o) {
		refuseForward(w, http.StatusForbidden, "forbidden\n")
		return
	}
	// The host's route refuses a longer target with the same 403 as a stopped
	// share, which would close this forward for a long URL.
	target := r.URL.RequestURI()
	if !strings.HasPrefix(target, "/") {
		refuseForward(w, http.StatusBadRequest, "bad request\n")
		return
	}
	if len(target) > maxForwardTarget {
		refuseForward(w, http.StatusRequestURITooLong, "request too long\n")
		return
	}
	f.proxy.ServeHTTP(w, r)
}

// page writes one of the three fixed error pages: kind is "stopped",
// "nothing" or "unreachable". Only the escaped host label and the numeric port
// appear in it; nothing comes from the request.
func (f *portForward) page(w http.ResponseWriter, kind string) {
	host := html.EscapeString(f.label)
	var heading, para string
	switch kind {
	case "stopped":
		heading = host + " stopped sharing"
		para = fmt.Sprintf("Pages from %s's port %d are closed. Click the link in Portty again once %s shares the session.", host, f.port, host)
	case "nothing":
		heading = fmt.Sprintf("Nothing is answering on %s's port %d", host, f.port)
		para = fmt.Sprintf("Start the server on %s, then reload this page.", host)
	default:
		heading = "Couldn't reach " + host
		para = "Check that both Macs are on your tailnet, then reload this page."
	}
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Security-Policy", "default-src 'none'")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusBadGateway)
	_, _ = fmt.Fprintf(w, "<!doctype html><html><head><meta charset=\"utf-8\"><title>%s</title></head><body><h1>%s</h1><p>%s</p></body></html>\n", heading, heading, para)
}
