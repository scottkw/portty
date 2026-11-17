package webserver

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"sync"
	"syscall"
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
}

// WebServer serves the Portty dashboard and relays terminal I/O over WSS to
// remote browser clients. Access control is provided at the network level by
// Tailscale — no application-layer authentication is required.
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
}

// NewWebServer creates a WebServer and sets up routes.
// Does NOT start the listener — call Start() to begin serving.
func NewWebServer(cfg Config, manager *relay.HubManager) (*WebServer, error) {
	ws := &WebServer{
		config:                  cfg,
		manager:                 manager,
		webEnabled:              make(map[string]bool),
		grants:                  make(map[string]map[string]struct{}),
		mux:                     http.NewServeMux(),
		pluginConfigSubscribers: make(map[chan []byte]struct{}),
	}
	ws.whois = &ws.lc
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
	ws.mu.Unlock()
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

// Start opens the TLS listener on the tailnet bind IP using Tailscale's
// lc.GetCertificate hook (or the TLSConfig override) and begins serving.
// A taken port is ErrPortInUse.
func (ws *WebServer) Start() error {
	// WR-03: belt-and-suspenders trust-boundary check for the open meta route.
	ws.assertTailnetBindIP()

	tlsCfg := ws.config.TLSConfig
	if tlsCfg == nil {
		tlsCfg = &tls.Config{
			GetCertificate: ws.lc.GetCertificate,
			MinVersion:     tls.VersionTLS12,
		}
	}

	ln, err := ws.listenTLS(tlsCfg)
	if err != nil {
		return err
	}

	ws.mu.Lock()
	ws.listener = ln
	ws.mu.Unlock()

	go http.Serve(ln, ws.mux) //nolint:errcheck

	return nil
}

// ErrPortInUse is returned by Start when the configured port is taken. There
// is no random-port fallback: tailnet peers only probe the fixed share port.
var ErrPortInUse = errors.New("address already in use")

// listenTLS binds the configured port. Port 0 (tests) binds a random port.
func (ws *WebServer) listenTLS(tlsCfg *tls.Config) (net.Listener, error) {
	port := ws.config.Port
	ln, err := tls.Listen("tcp", fmt.Sprintf("%s:%d", ws.config.BindIP, port), tlsCfg)
	if err != nil {
		if port != 0 && errors.Is(err, syscall.EADDRINUSE) {
			return nil, fmt.Errorf("webserver: port %d is in use: %w", port, ErrPortInUse)
		}
		return nil, fmt.Errorf("webserver: listen: %w", err)
	}
	return ln, nil
}

// Stop closes the TLS listener, stopping the HTTP server.
func (ws *WebServer) Stop() error {
	// Snapshot under RLock and close after releasing it (Close() may block).
	ws.mu.RLock()
	ln := ws.listener
	ws.mu.RUnlock()
	if ln == nil {
		return nil
	}
	return ln.Close()
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
//   - /api/sessions, /api/sessions/{id}/info, /sessions/{id}, /sessions/{id}/ws
//     — gated by requireCapability. The session-bound cap enforces enumeration,
//     cross-session isolation, and write permission via claims.Perms (D-24).
//   - /api/sessions/meta and POST /api/peer/sessions/{id}/cap — gated by
//     requireSameUser (same Tailscale user via WhoIs, fail closed).
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
	// share's capability with no join code. POST + application/json only, no
	// Access-Control-* headers, Cache-Control: no-store (whois.go handlePeerCap).
	mux.HandleFunc("POST /api/peer/sessions/{id}/cap", ws.requireSameUser(ws.handlePeerCap))

	// GET /api/sessions — capability-gated; handleListSessions returns ONLY
	// the single session bound to the cap (D-18). porttyMarker sits outside the
	// cap check so the unauthenticated 401 that peer discovery sees carries it too.
	mux.HandleFunc("GET /api/sessions", porttyMarker(ws.requireCapability(ws.handleListSessions)))

	// GET /api/sessions/{id}/info — capability-gated; cap must match {id}
	// (SEC-03). Used by the terminal page to populate status bar + perms
	// (D-19, D-23).
	mux.HandleFunc("GET /api/sessions/{id}/info", ws.requireCapability(ws.handleSessionInfo))

	// Phase 93 PLUG-04: capability-gated plugin config endpoint. Per
	// capability_mw.go path-ID logic, an empty path-ID short-circuits the
	// SID check, so any valid cap passes. Plugin config is global (not
	// session-specific) but the caller must still hold a verified cap.
	mux.HandleFunc("GET /api/plugin-config", ws.requireCapability(ws.handleGetPluginConfig))

	// Phase 93 PLUG-04 push channel — SSE stream of plugin-config changes.
	// Closes ROADMAP SC#4 ("no manual page reload for hot-swappable plugins").
	// Capability-gated like the read endpoint above.
	mux.HandleFunc("GET /api/plugin-config/stream", ws.requireCapability(ws.handleStreamPluginConfig))

	// GET /sessions/{id} — capability-gated terminal HTML page. The old
	// webEnabled-only pre-check is removed — requireCapability's grant-list
	// lookup already implies web-enabled (grants are cleared on toggle-off).
	mux.HandleFunc("GET /sessions/{id}",
		ws.cspHeaders(ws.requireCapability(ws.handleTerminalPage)))

	// GET /sessions/{id}/ws — Origin allowlist + capability-gated WebSocket
	// upgrade. Phase 88 (D-10) wraps requireAllowedOrigin OUTSIDE
	// requireCapability so a cross-site Origin is rejected BEFORE any HMAC
	// verification work runs. The wrapper ordering matches composition
	// outermost->innermost: basicAuth (local only) -> requireAllowedOrigin
	// -> requireCapability -> handleWSSRelay.
	mux.HandleFunc("GET /sessions/{id}/ws",
		ws.requireAllowedOrigin(ws.requireCapability(ws.handleWSSRelay)))

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
	// instead of a nil-deref panic. Public
	// tier: no capability gate here — the React bundle is static client code,
	// and any /api/ call it makes is independently gated.
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
	mux.HandleFunc("GET /app/", ws.cspHeaders(func(w http.ResponseWriter, r *http.Request) {
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
	}))
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
// Uses subscribe-before-snapshot pattern to avoid missing frames.
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
		Origin:   "web", // read by the host-authority resize arbiter
	}
	sub.CloseSlow = func() {
		conn.Close(websocket.StatusPolicyViolation, "too slow")
	}

	// Subscribe FIRST — frames arrive in Msgs from now on, so the snapshot
	// replayed below cannot leave a gap in the output stream.
	hub.Subscribe(sub)
	relay.NotifyViewerCount(hub) // push updated viewer count to all clients
	defer func() {
		hub.Unsubscribe(sub)
		relay.NotifyViewerCount(hub)
	}()
	defer conn.CloseNow()

	// VIEW-03: push authoritative host grid BEFORE scrollback replay so that
	// replayed raw bytes land in a correctly-sized terminal grid. Direct conn.Write
	// (not sub.Msgs) guarantees ordering before any queued live output.
	if c, r := hub.Cols(), hub.Rows(); c > 0 && r > 0 {
		if err := conn.Write(ctx, websocket.MessageBinary, relay.MakeResizeFrame(uint16(c), uint16(r))); err != nil {
			return
		}
	}

	// Phase 175 (BUG-04, #119): replay a catch-up preamble. ReconnectPreamble
	// picks BYTE-FAITHFUL raw scrollback by default (keeps #109's guest-layout
	// fix — the guest xterm stays in lockstep with the host TUI) and only falls
	// back to the emulator-derived current-screen snapshot when the raw 256 KiB
	// ring has actually wrapped past a full-screen TUI's one-time ESC[?1049h
	// (alt-screen enter) marker, where raw replay would otherwise be
	// blank/garbled. EnsureLiveEmulator (continuously fed by Hub.Run) keeps that
	// fallback available and is idempotent — a no-op after the first connection.
	hub.EnsureLiveEmulator()
	if snapshot := hub.ReconnectPreamble(); len(snapshot) > 0 {
		if err := conn.Write(ctx, websocket.MessageBinary, snapshot); err != nil {
			return
		}
	}

	// Read pump — client → PTY
	readDone := make(chan struct{})
	absorber := &relay.InputAbsorber{} // Phase 111 / Issue #54: absorb OSC 10/11/DA1 replies. Phase 115: type moved to relay package.
	go func() {
		defer close(readDone)
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
				if !sub.ReadOnly { // MC-03: discard input for read-only clients
					filtered := absorber.Filter(payload)
					if len(filtered) > 0 {
						_ = hub.WriteInput(filtered)
					}
				}
			case relay.MsgResize2:
				// VIEW-02 / T-157-02: a web guest is a read-only viewer of the host PTY grid.
				// The host-authority arbiter also rejects non-local origin, but dropping the
				// call here avoids a needless lock acquisition per guest resize and makes the
				// intent explicit: a remote web client never drives the shared PTY size.
			case relay.MsgPing:
				// Keep-alive — no-op.
			}
		}
	}()

	// Write pump — hub → browser
	for {
		select {
		case frame := <-sub.Msgs:
			if err := conn.Write(ctx, websocket.MessageBinary, frame); err != nil {
				return
			}
		case <-ctx.Done():
			return
		case <-hub.Done():
			// BUG-02 (#125): the owner ended/stopped the session — tell every
			// connected viewer instead of a bare return (which the deferred
			// conn.CloseNow() would then turn into a silent abnormal close
			// with no code/reason). Fixed generic reason only — never a raw
			// Go error string or a user-set session name (IN-01 convention,
			// T-175-06-01).
			conn.Close(websocket.StatusNormalClosure, "session ended")
			return
		case <-readDone:
			return
		}
	}
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
