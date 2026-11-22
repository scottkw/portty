package webserver

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/coder/websocket"
	"github.com/scottkw/portty/internal/capability"
	"github.com/scottkw/portty/internal/relay"
	"github.com/scottkw/portty/internal/tailnet"
	webfs "github.com/scottkw/portty/web"
	"tailscale.com/client/local"
)

// Config holds configuration for the WebServer.
type Config struct {
	// BindIP is the IP address to bind the HTTPS listener on (Tailscale IP, e.g. 100.x.x.x).
	BindIP string
	// Port is the HTTPS port. 0 binds a random port (tests); a taken port is ErrPortInUse.
	Port int
	// FQDN is the Tailscale MagicDNS hostname (from CertDomains[0]), used in BaseURL.
	FQDN string
	// TLSConfig is an override for tests; nil in production (uses lc.GetCertificate).
	TLSConfig *tls.Config
	// APIVersion is reported as "api" on GET /api/sessions/{id}/info so peers
	// can gate on it (Phase 9, UPGR-04). 0 omits the field.
	APIVersion int
}

// sessionMetaItem is the JSON shape returned by GET /api/sessions/meta.
// The endpoint needs no capability but is gated by requireSameUser: only a
// tailnet node logged in as this Mac's Tailscale user gets it. It must NEVER
// include cap tokens, grants, content, or signing keys (RB-03).
type sessionMetaItem struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	URL    string `json:"url"`
}

// sessionListItem is the JSON shape returned by GET /api/sessions and GET /api/sessions/{id}/info.
// Perms is populated only on the /info endpoint (from the verified capability
// claims, D-19/D-23 — terminal.html uses this to suppress the input caret on
// read-only capabilities). It is omitted on /api/sessions listings because a
// single-item self-describe response does not need to re-export the caller's
// own perms.
type sessionListItem struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Hostname string `json:"hostname"`
	Perms    string `json:"perms,omitempty"`
	API      int    `json:"api,omitempty"` // only on /info (Config.APIVersion)
}

// WebServer serves the Portty dashboard and relays terminal I/O over WSS to
// remote browser clients. It listens on the tailnet only; every session route
// also needs a capability token, and the peer routes a same-user WhoIs check.
type WebServer struct {
	config  Config
	manager *relay.HubManager

	// lc is the Tailscale local client; it backs the TLS certificate hook
	// (GetCertificate) in Start.
	// Stored by value (zero-value usable — local.Client has no internal
	// pointers that require explicit init). Anti-Pattern 2: never store by ptr.
	lc local.Client

	mu         sync.RWMutex
	webEnabled map[string]bool // sessionID -> enabled (WEB-01 toggle)
	listener   net.Listener    // guarded by ws.mu; the TLS listener (tailscale cert or self-signed)
	rawLn      net.Listener    // guarded by ws.mu; the TCP listener under it, exported for a handoff
	srv        *http.Server    // guarded by ws.mu; Stop closes it with its connections
	mux        *http.ServeMux

	// grants tracks the set of active grant_ids per session (D-14). Populated
	// on toggle-on by Plan 04, cleared on toggle-off or session exit. Guarded
	// by ws.mu.
	grants map[string]map[string]struct{}

	// signingKey is the 32-byte HMAC-SHA256 key used by requireCapability
	// (D-04/D-16). Guarded by ws.mu; swapped race-free via SetSigningKey.
	// Read via currentSigningKey which returns the slice header under RLock.
	signingKey []byte

	// sessionResolver is set once before Start() and is not mutex-protected.
	sessionResolver func(sessionID string) (name, status, hostname string, ok bool)

	// pluginSettingsProvider returns pre-marshaled JSON for the daemon's
	// current plugin settings. Set once before Start() via
	// SetPluginSettingsProvider; not mutex-protected. Phase 93 PLUG-04.
	//
	// Uses func() []byte (not daemon.PluginSettings) to avoid the daemon→
	// webserver→daemon circular import (see 93-RESEARCH Q1).
	pluginSettingsProvider func() []byte

	// staticAppFS is the embedded React frontend bundle served under /app/.
	// Wired from the daemon's share server start (share.go startShareServer)
	// via SetStaticAppFS. Read at request time inside
	// the route closure (Pitfall 2 — setupRoutes runs before SetStaticAppFS
	// has a chance to set it). nil → /app/ returns 503; this is the
	// expected dev-build state (assets stub).
	staticAppFS fs.FS

	// pluginConfigSubscribers is the set of active SSE subscribers for
	// /api/plugin-config/stream. Each subscriber gets a buffered channel;
	// BroadcastPluginConfig non-blocking-sends to each. Drop-on-slow-consumer.
	// Phase 93 PLUG-04 push channel — closes ROADMAP SC#4.
	pluginConfigMu          sync.RWMutex
	pluginConfigSubscribers map[chan []byte]struct{}

	// whois backs requireSameUser (whois.go). &lc in production.
	whois whoIsClient

	// shareToken returns a shared session's link token for the peer
	// capability route. Set once before Start via SetShareTokenProvider.
	shareToken func(sessionID string) (string, bool)

	// revokeSignals holds one channel per session, closed by ClearGrants so
	// long-lived streams end at once on revoke. Guarded by ws.mu.
	revokeSignals map[string]chan struct{}

	// beforeSubscribeHookForTest runs just before hub.Subscribe in
	// handleWSSRelay when non-nil. Test-only seam for the subscribe race.
	beforeSubscribeHookForTest func()

	// heartbeatEvery / heartbeatTimeout override relay.PingEvery and
	// relay.PingTimeout for viewer heartbeats when non-zero. Test-only.
	heartbeatEvery, heartbeatTimeout time.Duration

	// deniedPorts are loopback ports the Phase 13 port routes never grant or
	// proxy, on top of 0, this listener's own port and 7444. Set once before
	// Start via SetDeniedPorts; not mutex-protected.
	deniedPorts map[int]struct{}

	// stopCh is closed once by Stop. A proxied request (a WebSocket or SSE
	// stream especially) ends with it, because a handoff or shutdown stops the
	// server without ClearGrants.
	stopCh   chan struct{}
	stopOnce sync.Once

	// portDialForTest replaces dialLoopback when non-nil. Test-only.
	portDialForTest func(ctx context.Context, port int) (net.Conn, error)

	// portTransport carries every proxied request to the host's loopback
	// (portproxy.go); Stop closes its idle connections.
	portTransport *http.Transport
}

// NewWebServer creates a WebServer and sets up routes.
// Does NOT start the listener — call Start() to begin serving.
func NewWebServer(cfg Config, manager *relay.HubManager) (*WebServer, error) {
	ws := &WebServer{
		config:                  cfg,
		manager:                 manager,
		webEnabled:              make(map[string]bool),
		grants:                  make(map[string]map[string]struct{}),
		revokeSignals:           make(map[string]chan struct{}),
		mux:                     http.NewServeMux(),
		pluginConfigSubscribers: make(map[chan []byte]struct{}),
		stopCh:                  make(chan struct{}),
	}
	ws.portTransport = ws.newPortTransport()
	ws.whois = &ws.lc
	if !tailnet.LiveAllowed() {
		ws.whois = noLiveWhoIs{}
	}
	ws.setupRoutes()
	return ws, nil
}

// SetSessionResolver sets the callback used by handleListSessions and
// handleSessionInfo to resolve session metadata (name, status, hostname) and
// whether the session exists (ok). Must be called before Start().
func (ws *WebServer) SetSessionResolver(fn func(string) (string, string, string, bool)) {
	ws.sessionResolver = fn
}

// SetPluginSettingsProvider sets the callback used by handleGetPluginConfig
// to source the daemon's current plugin settings as pre-marshaled JSON.
// Plan 93-04 calls this at daemon startup so GET /api/plugin-config can
// serve capability-bearing web clients without a per-request RPC into the
// daemon. Phase 93 PLUG-04.
func (ws *WebServer) SetPluginSettingsProvider(fn func() []byte) {
	ws.pluginSettingsProvider = fn
}

// SetStaticAppFS installs the React frontend bundle fs.FS used to serve the
// /app/ route. Phase 120-04 wires this from the daemon at both web-start
// sites. nil → /app/ returns 503; that is the
// expected state under `go build` (no wailsassets tag → assets_stub.go's
// nil StaticAppFS). Production builds (`wails build` / `go build -tags
// wailsassets`) get the real embed.FS.
//
// Single setter, set once before Start(). Not mutex-protected — mirrors
// SetSessionResolver pattern.
func (ws *WebServer) SetStaticAppFS(appFS fs.FS) {
	ws.staticAppFS = appFS
}

// SetDeniedPorts adds ports the Phase 13 port routes refuse to grant or proxy.
// The daemon passes its relay port: the relay accepts Origins naming its own
// port, so a proxied request would satisfy that check. Set once before Start.
// 0 is ignored: it means "not known yet", and portDenied already refuses 0.
func (ws *WebServer) SetDeniedPorts(ports ...int) {
	ws.deniedPorts = make(map[int]struct{}, len(ports))
	for _, p := range ports {
		if p != 0 {
			ws.deniedPorts[p] = struct{}{}
		}
	}
}

// SetPortDialerForTest replaces the loopback dialer the port proxy uses.
// Test-only: the cross-daemon test runs both Macs on one machine, so the
// browser-facing port and the dev server's real port differ.
func (ws *WebServer) SetPortDialerForTest(fn func(ctx context.Context, port int) (net.Conn, error)) {
	ws.portDialForTest = fn
}

// EnableSession marks a session as web-served (WEB-01 toggle).
func (ws *WebServer) EnableSession(sessionID string) {
	ws.mu.Lock()
	ws.webEnabled[sessionID] = true
	ws.mu.Unlock()
}

// DisableSession removes a session from web-serving (WEB-01 toggle).
func (ws *WebServer) DisableSession(sessionID string) {
	ws.mu.Lock()
	delete(ws.webEnabled, sessionID)
	ws.mu.Unlock()
}

// IsSessionEnabled returns true if the session is marked as web-enabled.
func (ws *WebServer) IsSessionEnabled(sessionID string) bool {
	ws.mu.RLock()
	ok := ws.webEnabled[sessionID]
	ws.mu.RUnlock()
	return ok
}

// webEnabledSessions returns a snapshot of all web-enabled session IDs.
func (ws *WebServer) webEnabledSessions() []string {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	ids := make([]string, 0, len(ws.webEnabled))
	for id, enabled := range ws.webEnabled {
		if enabled {
			ids = append(ids, id)
		}
	}
	return ids
}

// AddGrant records grantID as an active grant for sessionID (D-14). Called by
// the daemon (Plan 04) after issuing a capability on toggle-on. Idempotent:
// adding an already-present grant is a no-op.
func (ws *WebServer) AddGrant(sessionID, grantID string) {
	ws.mu.Lock()
	if ws.grants[sessionID] == nil {
		ws.grants[sessionID] = make(map[string]struct{})
	}
	ws.grants[sessionID][grantID] = struct{}{}
	ws.mu.Unlock()
}

// ClearGrants removes every grant for sessionID (D-15). Called on toggle-off
// and by the session onExit callback (RESEARCH Pitfall 1). Safe to call when
// the session has no outstanding grants.
func (ws *WebServer) ClearGrants(sessionID string) {
	ws.mu.Lock()
	delete(ws.grants, sessionID)
	if ch, ok := ws.revokeSignals[sessionID]; ok {
		close(ch)
		delete(ws.revokeSignals, sessionID)
	}
	ws.mu.Unlock()
}

// revokeSignal returns the channel the next ClearGrants for sessionID closes.
// Callers re-check isGrantActive after taking it: a ClearGrants that ran just
// before is seen there, one that runs after closes the channel.
func (ws *WebServer) revokeSignal(sessionID string) <-chan struct{} {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ch, ok := ws.revokeSignals[sessionID]
	if !ok {
		ch = make(chan struct{})
		ws.revokeSignals[sessionID] = ch
	}
	return ch
}

// isGrantActive reports whether grantID is currently in sessionID's active
// grant set. Read-only; uses the RLock path to avoid blocking concurrent
// requireCapability calls against other sessions.
func (ws *WebServer) isGrantActive(sessionID, grantID string) bool {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	if ws.grants[sessionID] == nil {
		return false
	}
	_, ok := ws.grants[sessionID][grantID]
	return ok
}

// SetSigningKey installs the HMAC-SHA256 signing key used by subsequent
// requireCapability verifications (D-04/D-16). Plan 04 calls this at daemon
// startup; the RegenerateSigningKey handler calls it to rotate keys. The swap
// is race-free against concurrent currentSigningKey readers because both
// methods acquire ws.mu.
func (ws *WebServer) SetSigningKey(key []byte) {
	ws.mu.Lock()
	ws.signingKey = key
	ws.mu.Unlock()
}

// currentSigningKey returns the current signing key under RLock. The returned
// slice header shares the backing array with the field; callers MUST NOT
// mutate it. SetSigningKey only ever reassigns the slice — the backing array
// is never mutated in place — so callers observing a pre-swap slice see
// stable bytes.
func (ws *WebServer) currentSigningKey() []byte {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return ws.signingKey
}

// tailscaleCGNAT is the Tailscale 100.64.0.0/10 carrier-grade NAT range that
// MagicDNS / tailnet IPs are drawn from. The share listener is expected to be
// bound to an address in this range (resolved #86).
var tailscaleCGNAT = netip.MustParsePrefix("100.64.0.0/10")

// assertTailnetBindIP is a defense-in-depth, non-fatal startup check (WR-03).
// The cap-free /api/sessions/meta route is gated by requireSameUser (WhoIs),
// which assumes callers arrive over the tailnet. We WARN — not
// refuse — when BindIP is neither loopback (allowed for tests/dev) nor within
// the Tailscale CGNAT range, so a future misconfiguration that binds the open
// metadata endpoint to a LAN/0.0.0.0 interface surfaces a loud log line rather
// than silently becoming an unauthenticated enumeration surface. Refusing to
// start was rejected: it would break the many loopback-bound tests and is more
// invasive than the hardening this finding calls for.
func (ws *WebServer) assertTailnetBindIP() {
	bind := ws.config.BindIP
	addr, err := netip.ParseAddr(bind)
	if err != nil {
		// Empty or non-IP bind (e.g. "" → all interfaces). This is the most
		// dangerous case; warn explicitly.
		slog.Warn("webserver: bind IP is not a parseable address; "+
			"the share listener expects a tailnet bind IP",
			"bind_ip", bind)
		return
	}
	if addr.IsLoopback() {
		return // loopback is the standard test/dev bind; not a tailnet exposure
	}
	if tailscaleCGNAT.Contains(addr) {
		return // expected: bound to the Tailscale 100.x address
	}
	slog.Warn("webserver: bind IP is outside the Tailscale CGNAT range "+
		"(100.64.0.0/10); the share listener expects a tailnet-only bind IP",
		"bind_ip", bind)
}

// CheckListener refuses a listener this server would not have opened itself
// (Phase 9 WR-05): it must be TCP on the configured bind IP and, when one is
// set, the configured port. A listener handed over by another process could
// otherwise be bound anywhere while assertTailnetBindIP checks the config.
// An unparseable bind IP is left to assertTailnetBindIP's warning.
func (ws *WebServer) CheckListener(raw net.Listener) error {
	ta, ok := raw.Addr().(*net.TCPAddr)
	if !ok {
		return fmt.Errorf("webserver: listener on %v is not TCP", raw.Addr())
	}
	if bind := net.ParseIP(ws.config.BindIP); bind != nil && !ta.IP.Equal(bind) {
		return fmt.Errorf("webserver: listener on %v, want bind IP %s", ta, ws.config.BindIP)
	}
	if ws.config.Port != 0 && ta.Port != ws.config.Port {
		return fmt.Errorf("webserver: listener on %v, want port %d", ta, ws.config.Port)
	}
	return nil
}

// Start opens the TCP listener on the tailnet bind IP and serves TLS on it
// (see StartOn). A taken port is ErrPortInUse.
func (ws *WebServer) Start() error {
	raw, err := ws.listenTCP()
	if err != nil {
		return err
	}
	if err := ws.StartOn(raw); err != nil {
		_ = raw.Close() // a refused listener must not keep the share port bound
		return err
	}
	return nil
}

// StartOn serves TLS on raw, a TCP listener opened by Start or handed over
// from another process (Phase 9, Pattern 7), using Tailscale's
// lc.GetCertificate hook (or the TLSConfig override).
func (ws *WebServer) StartOn(raw net.Listener) error {
	if raw == nil {
		return errors.New("webserver: StartOn: nil listener")
	}
	if err := ws.CheckListener(raw); err != nil {
		return err
	}
	// WR-03: belt-and-suspenders trust-boundary check for the open meta route.
	// CheckListener made the bind IP the address actually served.
	ws.assertTailnetBindIP()

	tlsCfg := ws.config.TLSConfig
	if tlsCfg == nil {
		getCert := ws.lc.GetCertificate
		if !tailnet.LiveAllowed() {
			getCert = func(*tls.ClientHelloInfo) (*tls.Certificate, error) { return nil, tailnet.ErrLiveTailnetInTest }
		}
		tlsCfg = &tls.Config{
			GetCertificate: getCert,
			MinVersion:     tls.VersionTLS12,
		}
	}

	ln := tls.NewListener(raw, tlsCfg)

	// Timeouts bound slow header reads and idle keep-alives from tailnet peers.
	srv := &http.Server{Handler: ws.mux, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	ws.mu.Lock()
	ws.listener = ln
	ws.rawLn = raw
	ws.srv = srv
	ws.mu.Unlock()

	go srv.Serve(ln) //nolint:errcheck

	return nil
}

// ErrPortInUse is returned by Start when the configured port is taken. There
// is no random-port fallback: tailnet peers only probe the fixed share port.
var ErrPortInUse = errors.New("address already in use")

// listenTCP binds the configured port. Port 0 (tests) binds a random port.
func (ws *WebServer) listenTCP() (net.Listener, error) {
	port := ws.config.Port
	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", ws.config.BindIP, port))
	if err != nil {
		if port != 0 && errors.Is(err, syscall.EADDRINUSE) {
			return nil, fmt.Errorf("webserver: port %d is in use: %w", port, ErrPortInUse)
		}
		return nil, fmt.Errorf("webserver: listen: %w", err)
	}
	return ln, nil
}

// Stop closes the listener and every open connection, so keep-alive sockets
// stop reaching this server's mux (WR-01). Hijacked WebSockets survive Close;
// callers kick viewers before Stop.
func (ws *WebServer) Stop() error {
	ws.stopOnce.Do(func() { close(ws.stopCh) })
	ws.portTransport.CloseIdleConnections()
	// Snapshot under RLock and close after releasing it (Close() may block).
	ws.mu.Lock()
	srv := ws.srv
	ws.rawLn = nil
	ws.mu.Unlock()
	if srv == nil {
		return nil
	}
	return srv.Close()
}

// Listener returns the raw TCP listener under TLS, for a handoff to send its
// descriptor (Phase 9), or nil when the server is not running. The caller
// must not close it; Stop does.
func (ws *WebServer) Listener() net.Listener {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	return ws.rawLn
}

// Addr returns the listener's network address (host:port).
func (ws *WebServer) Addr() string {
	ws.mu.RLock()
	ln := ws.listener
	ws.mu.RUnlock()
	if ln == nil {
		return ""
	}
	return ln.Addr().String()
}

// BaseURL returns the base HTTPS URL for the server, built from the FQDN
// (e.g. https://hostname.ts.net:7444).
func (ws *WebServer) BaseURL() string {
	ws.mu.RLock()
	ln := ws.listener
	ws.mu.RUnlock()
	if ln == nil {
		return ""
	}
	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		return ""
	}
	return fmt.Sprintf("https://%s:%s", ws.config.FQDN, port)
}

// setupRoutes registers all HTTP routes on the server mux.
//
// Route access model (Phase 87, SEC-02..SEC-05):
//   - /dashboard and / — open (landing page; Plan 06 rewrites the dashboard
//     to have no session list).
//   - /api/sessions, /api/sessions/{id}/info, /api/plugin-config,
//     /api/plugin-config/stream, /sessions/{id}, /sessions/{id}/ws — gated by
//     requireCapability. The session-bound cap enforces enumeration,
//     cross-session isolation, and write permission via claims.Perms (D-24).
//   - MOB-05: the discovery probe, info, both plugin-config routes and the
//     WebSocket are also behind requireSameUser, outermost (the WebSocket:
//     Origin, then WhoIs, then capability), so another account is refused
//     before any capability work. /assets/, / and /dashboard stay open: they
//     carry no session data.
//   - /api/sessions/meta and POST /api/peer/sessions/{id}/cap — gated by
//     requireSameUser (same Tailscale user via WhoIs, fail closed).
//   - POST /api/peer/sessions/{id}/ports and /api/peer/sessions/{id}/ports/{port}
//     (Phase 13) — requireSameUser, then a write capability in X-Portty-Cap;
//     the proxy route also needs the port token (portproxy.go).
func (ws *WebServer) setupRoutes() {
	mux := ws.mux

	// GET / → redirect to /dashboard
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/dashboard", http.StatusFound)
			return
		}
		http.NotFound(w, r)
	})

	// GET /dashboard — open landing page (no session list per D-17, finalized
	// by Plan 06).
	mux.HandleFunc("GET /dashboard", ws.cspHeaders(ws.handleDashboard))

	// GET /api/sessions/meta — no capability, but only a node logged in as
	// this Mac's Tailscale user (WhoIs, numeric user ID) gets the shared
	// sessions' metadata (id, name, status, url). Anyone else gets 403 with
	// the X-Portty marker so the viewing Mac drops the group. Never returns
	// cap tokens, grants, or session content (RB-03).
	mux.HandleFunc("GET /api/sessions/meta", ws.requireSameUser(ws.handleSessionsMeta))

	// POST /api/peer/sessions/{id}/cap — a same-account peer receives the
	// share's capability with nothing to type. POST + application/json only, no
	// Access-Control-* headers, Cache-Control: no-store (whois.go handlePeerCap).
	mux.HandleFunc("POST /api/peer/sessions/{id}/cap", ws.requireSameUser(ws.handlePeerCap))

	// Phase 13, URL-02: the host half of a remote localhost URL. The grant mints
	// a port token for a port the session printed; the proxy route forwards to
	// this Mac's own loopback. Both sit behind requireSameUser, then check the
	// X-Portty-Cap write capability (and, for the proxy, the port token) before
	// anything is dialled (portproxy.go has the check order).
	//
	// They are NOT wrapped in cspHeaders or porttyMarker: the proxied app's own
	// Content-Security-Policy must reach the browser untouched, and every
	// refusal sets the marker itself. Registered per method because a
	// method-less pattern conflicts with the existing "GET /" and panics.
	mux.HandleFunc("POST /api/peer/sessions/{id}/ports", ws.requireSameUser(ws.handlePortGrant))
	for _, m := range []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"} {
		mux.HandleFunc(m+" /api/peer/sessions/{id}/ports/{port}", ws.requireSameUser(ws.handlePortProxy))
	}

	// GET /api/sessions — the discovery probe. Same-user gated (MOB-05, user
	// decision 2026-11-22: it returns the session name, status and hostname),
	// then capability-gated; handleListSessions returns ONLY the single session
	// bound to the cap (D-18). requireSameUser is outermost so a local tailscaled
	// fault is a bare 503 with no marker (WR-08) and another account gets the
	// marker 403; porttyMarker sits outside the cap check so the unauthenticated
	// 401 a same-user peer's discovery sees carries it too. Accepted cost: while
	// this Mac's tailscaled cannot answer, peers get the 503, which the probe
	// rejects, and drop this host's group until their next refresh (5-30 s).
	mux.HandleFunc("GET /api/sessions", ws.requireSameUser(porttyMarker(ws.requireCapability(ws.handleListSessions))))

	// GET /api/sessions/{id}/info — same-user first (MOB-05: another account
	// is refused with 403 + marker before the capability is read), then
	// capability-gated; cap must match {id} (SEC-03). Used by the terminal page
	// to populate status bar + perms (D-19, D-23).
	mux.HandleFunc("GET /api/sessions/{id}/info", ws.requireSameUser(ws.requireCapability(ws.handleSessionInfo)))

	// Phase 93 PLUG-04: same-user then capability-gated plugin config endpoint. Per
	// capability_mw.go path-ID logic, an empty path-ID short-circuits the
	// SID check, so any valid cap passes. Plugin config is global (not
	// session-specific) but the caller must still hold a verified cap.
	mux.HandleFunc("GET /api/plugin-config", ws.requireSameUser(ws.requireCapability(ws.handleGetPluginConfig)))

	// Phase 93 PLUG-04 push channel — SSE stream of plugin-config changes.
	// Closes ROADMAP SC#4 ("no manual page reload for hot-swappable plugins").
	// Gated like the read endpoint above (same-user, then capability).
	mux.HandleFunc("GET /api/plugin-config/stream", ws.requireSameUser(ws.requireCapability(ws.handleStreamPluginConfig)))

	// GET /sessions/{id} — capability-gated terminal HTML page. The old
	// webEnabled-only pre-check is removed — requireCapability's grant-list
	// lookup already implies web-enabled (grants are cleared on toggle-off).
	// MOB-05: same-user first, so another account sees the refusal page (403,
	// marker) and a tailscaled fault the retry page (503) before any capability
	// work; then a failed capability check shows the static "Link No Longer
	// Works" page.
	mux.HandleFunc("GET /sessions/{id}",
		ws.cspHeaders(ws.requireSameUserPage(ws.requireCapabilityPage(ws.handleTerminalPage))))

	// GET /sessions/{id}/ws — Origin allowlist + same-user + capability-gated
	// WebSocket upgrade. Phase 88 (D-10) wraps requireAllowedOrigin OUTSIDE
	// requireCapability so a cross-site Origin is rejected BEFORE any HMAC
	// verification work runs; MOB-05 puts requireSameUser between them, so
	// another account is refused by WhoIs before any HMAC work too. The wrapper
	// ordering matches composition outermost->innermost: basicAuth (local only)
	// -> requireAllowedOrigin -> requireSameUser -> requireCapability ->
	// handleWSSRelay.
	mux.HandleFunc("GET /sessions/{id}/ws",
		ws.requireAllowedOrigin(ws.requireSameUser(ws.requireCapability(ws.handleWSSRelay))))

	// GET /assets/ and /assets/xterm/ — public static assets from the embedded web.WebFS.
	// Per D-14: http.FileServerFS mounted via fs.Sub. Two separate mounts because
	// vendored xterm files live at web/vendor/xterm/ on disk (user decision D-01/D-02)
	// but must be served under the /assets/xterm/ URL prefix (matches HTML references
	// from Plan 02 and keeps the URL space uniform). First-party extracted JS/CSS
	// live at web/assets/*. Go 1.22+ mux chooses the longest-matching pattern, so
	// /assets/xterm/xterm.js resolves via xtermFS and /assets/terminal.js via assetsFS.
	// Public tier (D-15): no capability gate. Cache-Control: no-store (D-16) so
	// xterm upgrades take effect immediately.
	assetsFS, err := fs.Sub(webfs.WebFS, "assets")
	if err != nil {
		panic(fmt.Sprintf("webserver: fs.Sub assets: %v", err))
	}
	xtermFS, err := fs.Sub(webfs.WebFS, "vendor/xterm")
	if err != nil {
		panic(fmt.Sprintf("webserver: fs.Sub vendor/xterm: %v", err))
	}
	mux.Handle("GET /assets/xterm/", http.StripPrefix("/assets/xterm/",
		assetsNoStore(http.FileServerFS(xtermFS))))
	mux.Handle("GET /assets/", http.StripPrefix("/assets/",
		assetsNoStore(http.FileServerFS(assetsFS))))

	// Phase 120-04 — /app/ serves the React frontend SPA for remote/web-share viewers. The fs.FS is wired from
	// the daemon at web-start time via SetStaticAppFS; the closure reads
	// ws.staticAppFS at request time so a missing wiring surfaces as a 503
	// instead of a nil-deref panic. No capability gate here — the React bundle
	// is static client code, and any /api/ call it makes is independently gated.
	//
	// MOB-05: the whole bundle (index.html, chunks, manifest, icons) sits behind
	// requireSameUserPage, inside cspHeaders, so another account gets the static
	// refusal page (or the retry page on a tailscaled fault) before the SPA, any
	// /info request or any WebSocket. /assets/ stays open so those pages can load
	// /assets/dashboard.css.
	//
	// Phase 176 BUG-06 — wrapped in ws.cspHeaders (D-05): the guest SPA
	// previously served with NO Content-Security-Policy header. Reuses the existing strict policy
	// verbatim (same as /dashboard, /sessions/{id}); no capability
	// gate exists here to nest inside, so cspHeaders wraps the handler
	// literal outermost.
	//
	// Phase 120 WR-02 — caching policy: index.html is forced Cache-Control:
	// no-store via serveIndex so clients always pick up the latest entry
	// point. Other /app/* assets (hashed JS/CSS bundles) inherit Go's
	// FileServerFS default (Last-Modified from the embed.FS) with NO
	// Cache-Control header. This is intentional and safe because Vite
	// content-hashes every asset URL (e.g. index-aB12.js); the hash IS the
	// cache key, so stale browser caches cannot serve a mismatched bundle
	// against a fresh index.html. If a future deploy ships unhashed assets
	// under /app/, this contract breaks — wrap the FileServerFS call in
	// assetsNoStore (as /assets/ does) before that lands.
	//
	// NOTE (Phase 176): this route is wrapped in ws.cspHeaders, which sets
	// Cache-Control: no-store on all responses. The hashed-asset branch below
	// deletes that header (w.Header().Del) so this content-hash caching contract
	// still holds; do not remove that Del without re-homing the CSP header.
	mux.HandleFunc("GET /app/", ws.cspHeaders(ws.requireSameUserPage(func(w http.ResponseWriter, r *http.Request) {
		appFS := ws.staticAppFS
		if appFS == nil {
			http.Error(w, "app bundle not configured", http.StatusServiceUnavailable)
			return
		}
		// Strip the /app/ prefix and delegate to FileServerFS. SPA fallback:
		// any path that doesn't resolve to a real file in the bundle returns
		// index.html so client-side routing handles it.
		stripped := http.StripPrefix("/app/", http.FileServerFS(appFS))
		// Test for file existence before serving; on miss, serve index.html.
		path := r.URL.Path
		if path == "/app" || path == "/app/" {
			serveIndex(w, r, appFS)
			return
		}
		rel := path[len("/app/"):]
		// Phase 120 WR-01: block directory-index requests. http.FileServerFS
		// renders a styled HTML listing of an embed.FS directory when given a
		// trailing-slash path, leaking the bundle's file names + mtimes. The
		// /assets/* routes guard against this via assetsNoStore; /app/ now
		// mirrors that defence by falling through to the SPA fallback (which
		// serves index.html) for any trailing-slash sub-path.
		if rel != "" && rel[len(rel)-1] == '/' {
			serveIndex(w, r, appFS)
			return
		}
		if _, err := fs.Stat(appFS, rel); err != nil {
			// SPA fallback for unknown paths.
			serveIndex(w, r, appFS)
			return
		}
		// Phase 176 WR-02: cspHeaders (which now wraps this route) sets
		// Cache-Control: no-store on every response. That is correct for the
		// index.html branches (serveIndex re-sets no-store itself) but WRONG for
		// the hashed JS/CSS bundles served here — they must inherit FileServerFS's
		// default (no Cache-Control) so browsers cache by content-hash, per the
		// Phase 120 WR-02 contract documented above. Drop the middleware's
		// no-store on this branch only to preserve that contract.
		w.Header().Del("Cache-Control")
		stripped.ServeHTTP(w, r)
	})))
}

// serveIndex serves the React bundle's index.html with the correct
// Content-Type. Used as the SPA fallback for /app/* paths that don't map
// to a real file in the embedded FS.
func serveIndex(w http.ResponseWriter, _ *http.Request, appFS fs.FS) {
	data, err := fs.ReadFile(appFS, "index.html")
	if err != nil {
		http.Error(w, "app bundle missing index.html", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}

// handleDashboard serves the embedded dashboard.html.
func (ws *WebServer) handleDashboard(w http.ResponseWriter, r *http.Request) {
	data, err := webfs.WebFS.ReadFile("dashboard.html")
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data) //nolint:errcheck
}

// handleListSessions handles GET /api/sessions. Per D-18 the response
// contains ONLY the single session that the caller's capability is bound to
// — there is no listing-scoped capability, so no caller can ever receive a
// list longer than one via HTTPS. This collapses the endpoint from
// "enumeration" to "self-describe".
//
// requireCapability has already verified claims and attached them to the
// request context; if the context does not carry claims, treat as 401
// (defense in depth — should be unreachable given the middleware ordering).
func (ws *WebServer) handleListSessions(w http.ResponseWriter, r *http.Request) {
	claims, ok := capability.ClaimsFromContext(r.Context())
	if !ok {
		http.Error(w, "capability required", http.StatusUnauthorized)
		return
	}
	items := make([]sessionListItem, 0, 1)
	if ws.IsSessionEnabled(claims.SID) {
		name, st, hostname := "", "", ""
		if ws.sessionResolver != nil {
			name, st, hostname, _ = ws.sessionResolver(claims.SID)
		}
		if name == "" {
			name = claims.SID
		}
		items = append(items, sessionListItem{
			ID: claims.SID, Name: name, Status: st, Hostname: hostname,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items) //nolint:errcheck
}

// handleSessionsMeta handles GET /api/sessions/meta (behind requireSameUser).
// Returns cap-free metadata for all web-enabled sessions; it must NEVER
// include cap tokens, grants, or any credential material (RB-03).
func (ws *WebServer) handleSessionsMeta(w http.ResponseWriter, r *http.Request) {
	ids := ws.webEnabledSessions()
	items := make([]sessionMetaItem, 0, len(ids))
	for _, id := range ids {
		name, st := "", ""
		if ws.sessionResolver != nil {
			name, st, _, _ = ws.sessionResolver(id)
		}
		if name == "" {
			name = id
		}
		sessionURL := fmt.Sprintf("%s/sessions/%s", ws.BaseURL(), id)
		items = append(items, sessionMetaItem{
			ID:     id,
			Name:   name,
			Status: st,
			URL:    sessionURL,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items) //nolint:errcheck
}

// handleSessionInfo handles GET /api/sessions/{id}/info.
// Returns full session metadata for a single web-enabled session. requireCapability
// has already verified the cap, matched its SID to the path {id}, and attached
// the Claims to the request context; we lift Perms out of the claims so the
// terminal page can fail-safely determine its read-only state from the
// server-verified capability (D-19 / D-23 / SEC-04).
func (ws *WebServer) handleSessionInfo(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	claims, ok := capability.ClaimsFromContext(r.Context())
	if !ok {
		// Should be unreachable — requireCapability attaches claims on success.
		http.Error(w, "capability required", http.StatusUnauthorized)
		return
	}
	// requireCapability already cross-checked IsSessionEnabled before we got
	// here. Retain the resolver-based 404 for the "session ID isn't registered
	// with the engine" path (distinct from the 403 "revoked" response produced
	// by the middleware).
	if ws.sessionResolver == nil {
		http.NotFound(w, r)
		return
	}
	name, status, hostname, found := ws.sessionResolver(id)
	if !found {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessionListItem{
		ID:       id,
		Name:     name,
		Status:   status,
		Hostname: hostname,
		Perms:    claims.Perms,
		API:      ws.config.APIVersion,
	}) //nolint:errcheck
}

// handleTerminalPage redirects remote guests to the chat-capable React SPA.
//
// Phase 159 (WEBCHAT-01): The share link that the daemon mints is
// /sessions/{id}?cap=TOKEN, which previously served the vanilla-JS
// terminal.html viewer — a surface with no chat UI that silently discards
// chat frames 0x30–0x34. This handler now issues an HTTP 302 to
// /app/?session={id}&cap={token} so guests land on WebShareSessionView
// (the chat-capable SPA built in Phase 155).
//
// Security invariants (WEBCHAT-02, T-159-01, T-159-02):
//   - The redirect target is a server-controlled RELATIVE path (/app/?…).
//     sessionID and token are forwarded as query params, never as the host
//     or scheme — this is NOT an open redirect.
//   - requireCapability always runs BEFORE this handler (route registration
//     is unchanged). A missing or invalid cap returns 401/403 before any
//     redirect fires.
func (ws *WebServer) handleTerminalPage(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")
	token := r.URL.Query().Get("cap")
	target := fmt.Sprintf("/app/?session=%s&cap=%s",
		url.QueryEscape(sessionID),
		url.QueryEscape(token),
	)
	http.Redirect(w, r, target, http.StatusFound)
}

// handleWSSRelay upgrades to WebSocket and relays frames between the hub and the browser.
// Hub.Join subscribes and captures the replay atomically, so no frame is missed or duplicated.
//
// Phase 87: requireCapability has already verified the cap and attached the
// Claims to r.Context(). Subscriber.ReadOnly is sourced from claims.Perms
// (D-24 / SEC-04) — the old ?readonly=1 client-asserted hint has been
// removed from the write-gate path.
func (ws *WebServer) handleWSSRelay(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")

	// D-24 / SEC-04: readonly is bound to the signed capability, NOT the
	// query string. A caller with perms="read" cannot promote themselves to
	// write by omitting or fabricating ?readonly=; a caller with
	// perms="read,write" always has write even if someone appends
	// ?readonly=1 to the URL.
	//
	// SEC-01: gate on the whole "write" token, NOT an exact-string match.
	// The daemon now mints only "read" / "read,write", but tokens minted
	// before the file browser was removed may still carry legacy files perms
	// (e.g. "read,files.read") — an exact `== "read"` check would treat that
	// read-only viewer as read-write and let them inject to the PTY. HasPerm
	// uses whole-token semantics, so both "read" and "read,files.read" are
	// read-only while "read,write" is not.
	claims, _ := capability.ClaimsFromContext(r.Context())
	readonly := !capability.HasPerm(claims.Perms, "write")

	// MC-05: client name is still a benign view hint from the query string.
	clientName := r.URL.Query().Get("client")
	if len(clientName) > 64 {
		clientName = clientName[:64] // cap identity name to prevent injection
	}

	hub, ok := ws.manager.Get(sessionID)
	if !ok {
		// Session is web-enabled but not yet in the hub (e.g. just enabled, not started)
		// Accept the connection anyway and close cleanly
		http.Error(w, "session not found in hub", http.StatusNotFound)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// Phase 88 (D-12) belt-and-suspenders: the requireAllowedOrigin
		// middleware already rejected wrong-Origin requests with 403
		// before we got here. Setting OriginPatterns to the same strict
		// allowlist ensures the library-layer check ALSO rejects if a
		// future route-wiring change ever bypasses the middleware.
		// ws.allowedOrigins() returns []string{ws.BaseURL()} — the
		// library does a case-insensitive path.Match against
		// u.Scheme+"://"+u.Host when the pattern contains "://", which
		// matches our canonical BaseURL form exactly.
		OriginPatterns: ws.allowedOrigins(),
	})
	if err != nil {
		return
	}

	ctx := r.Context()

	sub := &relay.Subscriber{
		Msgs:     make(chan []byte, 256),
		ReadOnly: readonly,
		Name:     clientName,
		Origin:   "web", // a web report never bootstraps the grid (Hub.ResizeClient)
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		sub.Remote = host // viewer-diag only
	}
	sub.CloseSlow = func() {
		conn.Close(websocket.StatusPolicyViolation, "too slow")
	}
	// Share-off revokes with a fixed reason the clients recognise.
	sub.Kick = func(reason string) {
		conn.Close(websocket.StatusNormalClosure, reason)
	}
	// An upgrade handoff's commit (RestartViewers): retryable, so the browser
	// reconnects to the new daemon with resume.
	sub.Restart = func() {
		conn.Close(websocket.StatusServiceRestart, relay.CloseReasonServiceRestart)
	}

	if ws.beforeSubscribeHookForTest != nil {
		ws.beforeSubscribeHookForTest()
	}
	// Join subscribes and captures the catch-up replay atomically (SHAR-07):
	// every frame reaches this viewer exactly once, in the replay or in Msgs.
	resume, hasResume := relay.ParseResume(r.URL.Query().Get("resume"))
	replay := hub.Join(sub, resume, hasResume)
	joined := time.Now()
	relay.NotifyViewerCount(hub) // push updated viewer count to all clients
	defer func() {
		hub.Unsubscribe(sub)
		relay.NotifyViewerCount(hub)
	}()
	defer conn.CloseNow()

	// Re-check after subscribing: share-off clears grants before it kicks
	// subscribers, so a viewer that passed requireCapability just before the
	// revoke either is kicked or sees its grant gone here (TOCTOU). It runs
	// before any replay byte is written, so a revoked viewer gets no history.
	if !ws.isGrantActive(claims.SID, claims.GrantID) || !ws.IsSessionEnabled(sessionID) {
		conn.Close(websocket.StatusNormalClosure, relay.CloseReasonShareStopped)
		return
	}

	// VIEW-03: push authoritative host grid BEFORE scrollback replay so that
	// replayed raw bytes land in a correctly-sized terminal grid. Direct conn.Write
	// (not sub.Msgs) guarantees ordering before any queued live output.
	if c, r := hub.Cols(), hub.Rows(); c > 0 && r > 0 {
		if err := conn.Write(ctx, websocket.MessageBinary, relay.MakeResizeFrame(uint16(c), uint16(r))); err != nil {
			return
		}
	}

	// Phase 175 (BUG-04, #119) / Phase 5 (SHAR-07): the catch-up replay, bracketed
	// by MsgMeta{replay} and MsgMeta{offset}, in chunks of at most
	// relay.ReplayChunk. A full replay is byte-faithful raw scrollback while the
	// ring is intact (#109) and the live emulator's current screen once it
	// wrapped (M-51); a delta is only the ring bytes after the viewer's resume
	// offset. See Hub.Join.
	for _, f := range replay.Frames() {
		if err := conn.Write(ctx, websocket.MessageBinary, f); err != nil {
			return
		}
	}
	// Live frames held while the replay was written, then Msgs from here on (WR-01).
	for frames := hub.TakePending(sub); len(frames) > 0; frames = hub.TakePending(sub) {
		for _, f := range frames {
			if err := conn.Write(ctx, websocket.MessageBinary, f); err != nil {
				return
			}
		}
	}
	log.Printf("viewer-diag: replay session=%s conn=%d mode=%s bytes=%d resume=%t",
		relay.DiagID(sessionID), sub.ID, replay.Mode, replay.Bytes(), hasResume)

	// Read pump — client → PTY
	readDone := make(chan struct{})
	absorber := &relay.InputAbsorber{} // Phase 111 / Issue #54: absorb OSC 10/11/DA1 replies. Phase 115: type moved to relay package.
	in := hub.OpenInput(sub)           // INPT-01: queue input so the pump never blocks on the PTY
	pongs := relay.NewPongOwed()
	go func() {
		defer close(readDone)
		defer in.Close() // queued frames are still delivered after the socket goes
		// viewer-diag counts only: input payloads are never logged (they can be passwords).
		var inFrames, inBytes, resizes int
		defer func() {
			if inFrames > 0 || resizes > 0 {
				log.Printf("viewer-diag: input total session=%s conn=%d frames=%d bytes=%d resizes=%d", relay.DiagID(sessionID), sub.ID, inFrames, inBytes, resizes)
			}
		}()
		for {
			_, msg, err := conn.Read(ctx)
			if err != nil {
				return
			}
			msgType, payload, err := relay.ParseFrame(msg)
			if err != nil {
				continue
			}
			// 0x30-0x37 are RETIRED (chat, presence, typing, alias, inject, self):
			// no case and no default arm, so old clients' frames are dropped.
			switch msgType {
			case relay.MsgInput:
				passed := 0
				if !sub.ReadOnly { // MC-03: discard input for read-only clients
					filtered := absorber.Filter(payload)
					passed = len(filtered)
					if len(filtered) > 0 {
						if relay.IsUserInput(filtered) { // INPT-06: typing claims; terminal auto-replies never do
							_ = hub.NoteTyping(sub)
						}
						// ErrInputClosed (the session is ending, or its PTY write failed) is not
						// an exit: the write pump closes with "session ended" on hub.Done, and
						// returning here would race that with an abnormal close or drop a
						// live hub's viewer into a reconnect loop (WR-04). Input is dropped.
						_ = in.Write(filtered)
					}
				}
				inFrames++
				inBytes += len(payload)
				if inFrames == 1 {
					// Input within ms of joining, before any typing, is a terminal answering replayed queries (04-07 H-replies).
					log.Printf("viewer-diag: first input session=%s conn=%d client=%q after=%s bytes=%d passed=%d",
						relay.DiagID(sessionID), sub.ID, sub.Name, time.Since(joined), len(payload), passed)
				}
			case relay.MsgResize2:
				// INPT-06 ("viewer takes over size"): a write-cap viewer's claim takes the
				// PTY grid; a plain report (every older client) only stores its size and
				// never moves a known grid. This replaces VIEW-02 / T-157-02, which kept
				// a web guest from driving or DoS-ing the grid: a read-only cap is still
				// skipped here and in the hub (T-10-12), sizes are clamped in the hub
				// (T-10-13), and a write-cap holder already has the shell.
				// Logged once per connection (CR-01): any link holder can send these frames
				// at line rate; the total goes in the input total line.
				resizes++
				cols, rows, claim, ok := relay.ParseResize(payload)
				if !ok {
					break
				}
				if resizes == 1 {
					log.Printf("viewer-diag: web resize session=%s conn=%d cols=%d rows=%d claim=%t",
						relay.DiagID(sessionID), sub.ID, cols, rows, claim)
				}
				if !sub.ReadOnly {
					if claim {
						_ = hub.ClaimGrid(sub, cols, rows)
					} else {
						_ = hub.ResizeClient(sub, cols, rows)
					}
				}
			case relay.MsgPathClick:
				// Phase 12: a path click needs the same capability as typing (MC-03 shape);
				// Hub.PathClick refuses read-only subscribers again. Not logged per click:
				// any link holder can send these at line rate (the hub logs one outcome line).
				if !sub.ReadOnly {
					hub.PathClick(sub, payload)
				}
			case relay.MsgPing:
				pongs.Owe() // INPT-02: the write pump writes it ahead of queued output
			}
		}
	}()

	// SHAR-07: drop a viewer that stops answering WebSocket pings (half-open TCP,
	// frozen process). Started after the read pump, which answers pings.
	every, timeout := relay.PingEvery, relay.PingTimeout
	if ws.heartbeatEvery > 0 {
		every, timeout = ws.heartbeatEvery, ws.heartbeatTimeout
	}
	go func() {
		if relay.Heartbeat(ctx, conn, every, timeout) {
			log.Printf("viewer-diag: heartbeat timeout session=%s conn=%d", relay.DiagID(sessionID), sub.ID)
		}
	}()

	// Write pump — hub → browser
	relay.WritePump(ctx, conn, hub, sub, pongs, readDone)
}

// assetsNoStore wraps an http.Handler with Cache-Control: no-store (D-16) and
// blocks directory listing responses. http.FileServerFS returns an HTML directory
// index for requests ending with "/" when no index.html is present; we 404
// those requests because /assets/* serves individual files only.
// Scoped to /assets/* only — keeps embedded xterm + extracted JS/CSS
// fresh across deploys without content-hashing the URLs. Negligible
// bandwidth cost at single-page-load-per-session.
func assetsNoStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block directory listing: any request whose (already-stripped) path
		// is empty or ends with "/" is a directory index request — return 404.
		if r.URL.Path == "" || r.URL.Path == "/" || len(r.URL.Path) > 0 && r.URL.Path[len(r.URL.Path)-1] == '/' {
			http.NotFound(w, r)
			return
		}
		// fs.Sub rejects non-UTF-8 names with fs.ErrInvalid, which FileServerFS
		// turns into a 500; a client-supplied bad path is a 404 (found by FuzzWebServerRoutes).
		if !utf8.ValidString(r.URL.Path) {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// porttyMarker sets the Portty marker header (X-Portty: 1) before
// calling next. Only GET /api/sessions uses it: peer discovery probes that
// route and accepts only marked replies. It identifies Portty; it grants nothing.
func porttyMarker(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(tailnet.MarkerHeader, tailnet.MarkerValue)
		next(w, r)
	}
}
