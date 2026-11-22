package webserver

// Phase 13 (URL-01, URL-02): the host Mac's half of a remote localhost URL.
//
// POST /api/peer/sessions/{id}/ports mints a port token for a port the session
// itself printed. <METHOD> /api/peer/sessions/{id}/ports/{port} proxies to this
// Mac's own loopback, WebSocket upgrades included. The wire contract (paths,
// header names, the 403 and 502 rules, the grant JSON) is what an API 5 viewing
// daemon calls, so it is one-way once released.
//
// Rule: no check may move after a dial. Every credential is verified, and every
// refusal looks the same (refusePort), before anything connects to a loopback
// port; a grant never connects at all. A caller who could tell an open port
// from a closed one, or a printed one from an unprinted one, could scan this
// Mac through its owner's share.

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/scottkw/portty/internal/capability"
	"github.com/scottkw/portty/internal/relay"
	"github.com/scottkw/portty/internal/tailnet"
)

const (
	hdrCap       = "X-Portty-Cap"
	hdrPortToken = "X-Portty-Port-Token"
	hdrTarget    = "X-Portty-Target"

	maxGrantBody = 1 << 10
	maxTarget    = 8192
)

// refusePort is the one answer every refused grant or proxy request gets:
// exactly what refuseOtherUser writes, whatever the cause.
func refusePort(w http.ResponseWriter) { refuseOtherUser(w) }

// portDenied reports ports the routes never grant or proxy: 0, this share
// server's own listener, the Portty share port, and SetDeniedPorts' ports.
func (ws *WebServer) portDenied(port int) bool {
	if port == 0 || port == tailnet.DefaultProbePort {
		return true
	}
	if _, ok := ws.deniedPorts[port]; ok {
		return true
	}
	if _, p, err := net.SplitHostPort(ws.Addr()); err == nil {
		if own, err := strconv.Atoi(p); err == nil && own == port {
			return true
		}
	}
	return false
}

// portCapability checks the X-Portty-Cap header the way a session route checks
// ?cap=: valid for this session, grant active, session enabled; and the holder
// must have write permission.
func (ws *WebServer) portCapability(r *http.Request, id string) (capability.Claims, bool) {
	claims, status, _ := ws.verifyCapabilityToken(r.Header.Get(hdrCap), id)
	if status != 0 || !capability.HasPerm(claims.Perms, "write") {
		return capability.Claims{}, false
	}
	return claims, true
}

// handlePortGrant handles POST /api/peer/sessions/{id}/ports. Check order is
// fixed: content type, body, capability, deny list, printed check, mint.
func (ws *WebServer) handlePortGrant(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	port, token := 0, ""
	defer func() {
		outcome := "refused"
		if token != "" {
			outcome = "granted"
		}
		log.Printf("portproxy: grant session=%s port=%d outcome=%s", relay.DiagID(id), port, outcome)
	}()

	if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err != nil || mt != "application/json" {
		refusePort(w)
		return
	}
	// ReadAll over a 1 KiB limit: an oversized body is an error, and the
	// allocation is bounded.
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxGrantBody))
	var body struct {
		Port int `json:"port"`
	}
	if err != nil || json.Unmarshal(raw, &body) != nil || body.Port < 1 || body.Port > 65535 {
		refusePort(w)
		return
	}
	port = body.Port
	claims, ok := ws.portCapability(r, id)
	if !ok || ws.portDenied(port) {
		refusePort(w)
		return
	}
	hub, ok := ws.manager.Get(id)
	if !ok || !hub.PortPrinted(port) {
		refusePort(w)
		return
	}
	key := ws.currentSigningKey()
	tok, err := capability.SignPort(capability.PortClaims{
		SID: id, Port: port, IAT: time.Now().Unix(), GrantID: claims.GrantID,
		Kind: capability.PortTokenKind, V: 1,
	}, key)
	if err != nil {
		refusePort(w)
		return
	}
	token = tok
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(map[string]string{"token": token}) //nolint:errcheck
}

// parsePortParam reads the {port} path value: decimal digits only, no leading
// zero, 1..65535. Atoi alone would accept a sign.
func parsePortParam(s string) (int, bool) {
	if s == "" || s[0] == '0' || len(s) > 5 {
		return 0, false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(s)
	return n, err == nil && n >= 1 && n <= 65535
}

// handlePortProxy handles <METHOD> /api/peer/sessions/{id}/ports/{port}. Check
// order is fixed and every check precedes the first dial: port, capability,
// port token (session, port and grant must all match), deny list, target.
func (ws *WebServer) handlePortProxy(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	port, ok := parsePortParam(r.PathValue("port"))
	if !ok {
		refusePort(w)
		return
	}
	claims, ok := ws.portCapability(r, id)
	if !ok {
		refusePort(w)
		return
	}
	key := ws.currentSigningKey()
	pc, err := capability.VerifyPort(r.Header.Get(hdrPortToken), key)
	if err != nil || pc.SID != id || pc.Port != port || pc.GrantID != claims.GrantID || ws.portDenied(port) {
		refusePort(w)
		return
	}
	target := r.Header.Get(hdrTarget)
	if target == "" || len(target) > maxTarget || target[0] != '/' {
		refusePort(w)
		return
	}
	tu, err := url.ParseRequestURI(target)
	if err != nil {
		refusePort(w)
		return
	}

	// The proxied request ends when the share is revoked (ClearGrants: share
	// off, session exit, key regeneration) or the server stops (handoff and
	// shutdown call Stop without ClearGrants). ReverseProxy closes the upstream
	// and a hijacked upgrade connection when the context ends. Re-check the
	// grant after taking the signal: a ClearGrants that ran just before is seen
	// here, one that runs after closes the channel.
	sig := ws.revokeSignal(id)
	if !ws.isGrantActive(id, claims.GrantID) {
		refusePort(w)
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	go func() {
		select {
		case <-sig:
			cancel()
		case <-ws.stopCh:
			cancel()
		case <-ctx.Done():
		}
	}()
	ws.newPortReverseProxy(port, id, tu).ServeHTTP(w, r.WithContext(ctx))
}

// newPortReverseProxy proxies one request to localhost:port with target as its
// request line. The URL host is never dialled: the transport's dialer takes the
// port and connects only to this Mac's loopback.
func (ws *WebServer) newPortReverseProxy(port int, sid string, target *url.URL) *httputil.ReverseProxy {
	hostPort := "localhost:" + strconv.Itoa(port)
	return &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			out := pr.Out
			out.URL.Scheme = "http"
			out.URL.Host = hostPort
			out.URL.Path, out.URL.RawPath, out.URL.RawQuery = target.Path, target.RawPath, target.RawQuery
			out.Host = hostPort
			// ReverseProxy already drops Forwarded and X-Forwarded-For/Host/Proto;
			// any other X-Forwarded-* and every Portty header goes too.
			for k := range out.Header {
				if strings.HasPrefix(k, "X-Portty") || strings.HasPrefix(k, "X-Forwarded") || k == "Forwarded" {
					delete(out.Header, k)
				}
			}
			// The browser's Origin for the forward is already http://localhost:<P>.
			// Only a typed 127.0.0.1 URL needs it rewritten. Any other Origin
			// reaches the dev server unchanged, so its own cross-site check still
			// sees a cross-site page as cross-site.
			if out.Header.Get("Origin") == "http://127.0.0.1:"+strconv.Itoa(port) {
				out.Header.Set("Origin", "http://"+hostPort)
			}
		},
		Transport:     ws.portTransport,
		FlushInterval: -1,
		ModifyResponse: func(resp *http.Response) error {
			// A dev server must not forge the marker the viewing Mac trusts.
			for k := range resp.Header {
				if strings.HasPrefix(k, "X-Portty") {
					delete(resp.Header, k)
				}
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			// Never the dial error text: it names addresses.
			log.Printf("portproxy: upstream session=%s port=%d outcome=nothing-answering", relay.DiagID(sid), port)
			w.Header().Set(tailnet.MarkerHeader, tailnet.MarkerValue)
			http.Error(w, "nothing is answering", http.StatusBadGateway)
		},
		ErrorLog: log.New(io.Discard, "", 0),
	}
}

// newPortTransport is the one HTTP/1.1 transport for every proxied request.
// Its dialer ignores the host of the address (the proxy URL always says
// localhost) and takes only the port, which the handler validated.
func (ws *WebServer) newPortTransport() *http.Transport {
	return &http.Transport{
		DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
			_, p, err := net.SplitHostPort(addr)
			port, perr := strconv.Atoi(p)
			if err != nil || perr != nil || port < 1 || port > 65535 || ws.portDenied(port) {
				return nil, net.ErrClosed
			}
			return ws.dialLoopback(ctx, port)
		},
		DisableCompression:  true,
		ForceAttemptHTTP2:   false,
		MaxIdleConnsPerHost: 8,
		IdleConnTimeout:     30 * time.Second,
	}
}

// dialLoopback connects to this Mac's own loopback only: [::1] first (Node's
// listen(port, "localhost") binds it alone), then 127.0.0.1, 3 s each.
func (ws *WebServer) dialLoopback(ctx context.Context, port int) (net.Conn, error) {
	if ws.portDialForTest != nil {
		return ws.portDialForTest(ctx, port)
	}
	var first error
	for _, host := range []string{"::1", "127.0.0.1"} {
		d := net.Dialer{Timeout: 3 * time.Second}
		c, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
		if err == nil {
			return c, nil
		}
		if first == nil {
			first = err
		}
		if ctx.Err() != nil {
			break
		}
	}
	return nil, first
}
