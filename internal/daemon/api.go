package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/scottkw/portty/internal/capability"
	"github.com/scottkw/portty/internal/desktop"
	"github.com/scottkw/portty/internal/relay"
	"github.com/scottkw/portty/internal/screencap"
	"github.com/scottkw/portty/internal/tailnet"
	"github.com/scottkw/portty/internal/webserver"
)

// API serves the daemon HTTP API over a Unix socket or Windows named pipe.
type API struct {
	engine       *SessionEngine
	mux          *http.ServeMux
	handler      http.Handler // mux behind the handoff freeze gate (freezeGate); what the socket serves
	ln           net.Listener
	relayPort    int                  // TCP port the relay server is listening on
	relayLn      net.Listener         // TCP listener for the relay server
	mu           sync.RWMutex         // guards webServer, shares and the share seams
	webServer    *webserver.WebServer // the share server; nil when nothing is shared
	startMu      sync.Mutex           // serializes share-on, start through mint (taken before mu, never under it)
	tailnetCache *tailnetCache

	// shares holds one record per shared session (guarded by mu). The share
	// server runs exactly while this map is non-empty. See share.go.
	shares map[string]shareRecord
	// Share seams (guarded by mu): Tailscale health, share server factory and
	// certificate warm-up. Copied from the package defaults in NewAPI.
	shareHealth    func(context.Context) webserver.TailscaleHealth
	newShareServer func(webserver.TailscaleHealth, *relay.HubManager) (*webserver.WebServer, error)
	warmCert       func(context.Context, string) error

	// --- Phase 19 desktop (desktop_api.go), all guarded by mu --------------
	// desktopCfg is desktop.json as loaded or last saved; desktopErr is why a
	// file that exists was not usable (the desktop then stays off, the file is
	// left alone). desktopMgr is nil while the desktop is off or not yet up.
	desktopCfg desktop.Config
	desktopErr error
	desktopMgr *desktop.Manager
	// The tailnet address and name the Manager and the share server were built
	// for, so the poller can tell when they changed. Empty: unknown.
	desktopBuiltIP, desktopBuiltDomain string
	shareBuiltIP, shareBuiltDomain     string
	desktopTS                          string // last Tailscale verdict: ok, off, https_off, userspace; "" before the first check
	// Seams, copied from package defaults in NewAPI (SetDesktopSeamsForTest).
	interfaceAddrs   func() ([]net.Addr, error)
	newDesktopSource func() screencap.Source
	desktopMediaPort int // 0: desktop.MediaPortDefault
	// The poller runs for the API's life: desktopCtx ends in Stop, and it is
	// also the Manager's lifetime (a Manager keeps the context it is started with).
	desktopCtx       context.Context
	desktopCancel    context.CancelFunc
	desktopPollOnce  sync.Once
	desktopPollEvery time.Duration

	// --- Phase 22 click-to-share (desktop_share_api.go), guarded by mu -----
	// guestShare is the one active guest share, nil when there is none; it lives
	// in memory only. guestIAT is the issued-at of its link. guiStream is the
	// GUI's events stream, held by one GUI at a time: its end is the liveness
	// signal that ends the share.
	guestShare *desktop.GuestShare
	guestIAT   int64
	guiStream  *shareStream

	// --- Phase 87 capability state (D-04/D-06/D-14/D-15/D-16) -----------
	// signingKey is the 32-byte HMAC-SHA256 key used to Sign capabilities.
	// Bootstrapped from disk in BootstrapCapabilityState; replaced atomically
	// by handleRegenerateSigningKey. All reads/writes go under signingKeyMu
	// (a dedicated lock separate from a.mu so a capability request does not
	// block an unrelated operation on webServer/shares).
	signingKeyMu sync.RWMutex
	signingKey   []byte

	// remoteCaps caches per-session (baseURL, capToken) tuples used to open
	// remote sessions and by the remote terminal WS proxy. In-memory only;
	// daemon restart wipes the cache (Phase 122-01 / REMOTE-01).
	remoteCaps *RemoteCapStore

	// desktopCaps holds each peer's standing desktop token for the loopback
	// desktop proxy and the peer-desktop connect route. In-memory only (desktop_caps.go).
	desktopCaps *DesktopCapStore

	// portForwards holds the loopback forwards for remote localhost URLs and
	// the consent behind them (portforward.go). In-memory only.
	portForwards *PortForwards

	// peerClientForTest, when non-nil, replaces the verified per-peer client
	// (peer_client.go) for /peers/connect and the remote terminal WS proxy.
	// Tests inject httptest.NewTLSServer's Client(); production leaves it nil.
	peerClientForTest *http.Client

	// heartbeatEvery / heartbeatTimeout override relay.PingEvery and
	// relay.PingTimeout for the remote WS proxy's upstream heartbeat when non-zero.
	heartbeatEvery, heartbeatTimeout time.Duration

	// Peer connect seams (peer_connect.go), copied from package defaults in
	// NewAPI; tests replace them with SetPeerSeamsForTest.
	discoverPeers func(context.Context) ([]tailnet.Peer, error)
	probePeers    func(context.Context) []tailnet.Peer
	peerBaseURL   func(tailnet.Peer) string

	// --- Phase 9 handoff (handoff.go) ------------------------------------
	// lock is the open daemon.lock; a handoff sends it to the takeover, so
	// the lock never drops while sessions move (flock follows the open file).
	lock *os.File
	// frozen is the freeze gate: mutating routes answer 503 Retry-After.
	frozen atomic.Bool
	// writeGate makes the gate a barrier (WR-01): every mutating request
	// holds it for reading while it runs, and a handoff holds it for writing
	// from the drain before FREEZE until it resumes or exits.
	writeGate sync.RWMutex
	// handoffDone is non-nil while a handoff runs and closes at its end
	// (guarded by handoffMu): one handoff at a time, and the SIGTERM guard
	// waits on it.
	handoffMu   sync.Mutex
	handoffDone chan struct{}
	// committed: COMMIT was sent and not rolled back. handedOff: the takeover
	// answered COMMITTED, so this daemon exits without unlinking the socket
	// or ending a session.
	committed, handedOff atomic.Bool
}

// NewAPI creates an API wired to the given SessionEngine and registers all routes.
func NewAPI(engine *SessionEngine) *API {
	desktopCtx, desktopCancel := context.WithCancel(context.Background())
	a := &API{
		desktopCtx: desktopCtx, desktopCancel: desktopCancel, desktopPollEvery: desktopPollEvery,
		interfaceAddrs: defaultInterfaceAddrs, newDesktopSource: defaultNewDesktopSource,
		engine:       engine,
		mux:          http.NewServeMux(),
		tailnetCache: &tailnetCache{},
		remoteCaps:   NewRemoteCapStore(),
		desktopCaps:  NewDesktopCapStore(),
		portForwards: newPortForwards(),

		shares:         map[string]shareRecord{},
		shareHealth:    defaultShareHealth,
		newShareServer: defaultNewShareServer,
		warmCert:       defaultWarmCert,

		discoverPeers: tailnet.DiscoverPeers,
		peerBaseURL:   defaultPeerBaseURL,
	}
	a.probePeers = func(ctx context.Context) []tailnet.Peer {
		return a.tailnetCache.getOrRefresh(ctx, tailnet.DiscoverAndProbe)
	}
	// An unusable desktop.json leaves the desktop off with the reason kept for
	// GET /desktop; the file is never rewritten here.
	if engine.configDir != "" {
		a.desktopCfg, a.desktopErr = desktop.LoadConfig(engine.configDir)
		if a.desktopErr != nil {
			a.desktopCfg = desktop.Config{}
		}
	}
	a.registerRoutes()
	a.handler = a.freezeGate(a.mux)
	return a
}

// freezeGate answers 503 with Retry-After: 1 to every mutating request while
// a handoff has the sessions frozen; reads keep serving. 09-01's client waits
// out such a 503, so a write (and /shutdown: Quit All, a DEBT-08 restart)
// lands once the handoff committed or rolled back. The handoff route itself
// passes, so a second takeover is told it is busy.
//
// WR-01: a write holds writeGate while it runs and the handoff drains them
// before it exports (drainWrites), so no write that passed this check lands
// after the snapshot.
//
// WR-01 (it2): the drain is short, so no slow route may hold the gate.
// /peers/connect and /peers/desktop only cache a peer's token, which no handoff
// moves; PUT share, PUT /desktop and POST /desktop/share take the gate themselves after their slow health check and
// cert warm-up.
func (a *API) freezeGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.URL.Path == "/handoff" ||
			r.URL.Path == "/peers/connect" || r.URL.Path == "/peers/desktop" || r.Method == http.MethodPost && r.URL.Path == "/desktop/share" || r.Method == http.MethodPut && (strings.HasSuffix(r.URL.Path, "/share") || r.URL.Path == "/desktop") {
			next.ServeHTTP(w, r)
			return
		}
		if !a.enterWrite() {
			writeFrozen(w)
			return
		}
		defer a.writeGate.RUnlock()
		next.ServeHTTP(w, r)
	})
}

// enterWrite holds writeGate for a write unless a handoff has the sessions
// frozen; the caller releases it with a.writeGate.RUnlock.
func (a *API) enterWrite() bool { return !a.frozen.Load() && a.writeGate.TryRLock() }

// writeFrozen is the 503 a frozen daemon gives a write; the client retries it.
func writeFrozen(w http.ResponseWriter) {
	w.Header().Set("Retry-After", "1")
	http.Error(w, "the daemon is handing its sessions to a new version", http.StatusServiceUnavailable)
}

// drainWrites waits up to max for the writes in flight to finish, then holds
// writeGate so none starts until the handoff resumes. The caller has set
// frozen, so no new write joins the wait.
func (a *API) drainWrites(max time.Duration) bool {
	deadline := time.Now().Add(max)
	for !a.writeGate.TryLock() {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(5 * time.Millisecond)
	}
	return true
}

// registerRoutes wires all HTTP routes using Go 1.22+ path parameters.
func (a *API) registerRoutes() {
	a.mux.HandleFunc("GET /health", a.handleHealth)
	a.mux.HandleFunc("GET /sessions", a.handleListSessions)
	a.mux.HandleFunc("POST /sessions", a.handleCreateSession)
	a.mux.HandleFunc("GET /sessions/{id}", a.handleGetSession)
	a.mux.HandleFunc("DELETE /sessions/{id}", a.handleDeleteSession)
	a.mux.HandleFunc("PATCH /sessions/{id}/name", a.handleRenameSession)
	// One on/off share switch per session (share.go). Owner-only socket.
	a.mux.HandleFunc("GET /sessions/{id}/share", a.handleGetShare)
	a.mux.HandleFunc("PUT /sessions/{id}/share", a.handlePutShare)
	a.mux.HandleFunc("GET /sessions/{id}/tail", a.handleGetSessionTailLines)
	a.mux.HandleFunc("GET /sessions/{id}/styled-tail", a.handleGetSessionStyledTailLines)
	a.mux.HandleFunc("GET /settings/start-minimized", a.handleGetStartMinimized)
	a.mux.HandleFunc("PATCH /settings/start-minimized", a.handleSetStartMinimized)
	a.mux.HandleFunc("GET /settings/appearance", a.handleGetAppearance)
	a.mux.HandleFunc("PATCH /settings/appearance", a.handleSetAppearance)
	a.mux.HandleFunc("GET /settings/terminal-font", a.handleGetTerminalFont)
	a.mux.HandleFunc("PATCH /settings/terminal-font", a.handleSetTerminalFont)
	a.mux.HandleFunc("GET /settings/plugins", a.handleGetPluginSettings)
	a.mux.HandleFunc("PATCH /settings/plugins", a.handleSetPluginSettings)
	a.mux.HandleFunc("PATCH /settings/search-config", a.handleSetSearchConfig)
	a.mux.HandleFunc("PATCH /settings/web-links-config", a.handleSetWebLinksConfig)
	a.mux.HandleFunc("PATCH /settings/image-config", a.handleSetImageConfig)
	a.mux.HandleFunc("GET /relay-port", a.handleRelayPort)
	a.mux.HandleFunc("POST /shutdown", a.handleShutdown)
	// Phase 9 session handoff to a takeover daemon (handoff.go). Only on this
	// 0600 unix socket mux: never on the relay port or the share server.
	a.mux.HandleFunc("POST /handoff", a.handleHandoff)
	// File preview bytes for a session's viewers (preview.go). Socket only,
	// like /handoff: the relay port is reachable by browser pages.
	a.mux.HandleFunc("POST /sessions/{id}/preview", a.handlePreview)
	// Tailnet peer discovery.
	a.mux.HandleFunc("GET /tailnet/peers", a.handleTailnetPeers)
	// Connect to a session shared from another Mac (CLI attach and the GUI).
	a.mux.HandleFunc("POST /peers/connect", a.handlePeerConnect)
	// Fetch a peer's standing desktop token into memory for the loopback desktop proxy.
	a.mux.HandleFunc("POST /peers/desktop", a.handlePeerDesktop)
	// The browser link of a held peer desktop, token included. Socket only,
	// like /handoff and /preview: the relay port is reachable by browser pages.
	a.mux.HandleFunc("GET /peers/desktop/link", a.handlePeerDesktopLink)
	// Open a loopback forward for a clicked remote localhost URL. Socket only,
	// like /handoff and /preview: the relay port is reachable by browser pages.
	a.mux.HandleFunc("POST /peers/ports", a.handlePortOpen)
	// Phase 87 D-16 panic button: new signing key; shares are re-minted.
	a.mux.HandleFunc("POST /capability/regenerate-key", a.handleRegenerateSigningKey)
	a.registerDesktopRoutes(a.mux)
}

// Handler returns the API's underlying http.Handler (the registered mux).
// Exposed so external test packages (e.g. internal/daemon's package_test
// suite, the Phase 122-05 cross-surface parity test) can drive the daemon's
// routes through httptest.NewServer without needing access to the unexported
// mux field. Production callers do not use this — the daemon socket layer
// reaches into the mux directly.
func (a *API) Handler() http.Handler {
	return a.mux
}

// SetPeerClientForTest replaces the verified per-peer HTTPS client used by
// /peers/connect and the remote terminal WS proxy. Test-only: external test
// packages inject httptest.NewTLSServer's Client() so the daemon trusts the
// fake peer's self-signed certificate. Production code must never call this.
func (a *API) SetPeerClientForTest(c *http.Client) {
	a.peerClientForTest = c
}

// SetHeartbeatForTest shortens the remote WS proxy's upstream heartbeat. Tests only.
func (a *API) SetHeartbeatForTest(every, timeout time.Duration) {
	a.heartbeatEvery, a.heartbeatTimeout = every, timeout
}

// BootstrapCapabilityState loads or generates the HMAC signing key (D-04).
// Must be called once during
// daemon startup, BEFORE any web server is created — otherwise requireCapability
// would see a nil signing key (Pitfall 3) and 401 every request.
//
// Callers must pair this with ws.SetSigningKey whenever a WebServer is
// constructed (startShareServer in share.go).
// CurrentSigningKey exposes the bootstrapped key.
func (a *API) BootstrapCapabilityState() error {
	store := capability.NewFileKeyStore(a.engine.configDir)
	key, err := capability.LoadOrGenerate(store)
	if err != nil {
		return fmt.Errorf("capability bootstrap: %w", err)
	}
	a.signingKeyMu.Lock()
	a.signingKey = key
	a.signingKeyMu.Unlock()
	return nil
}

// CurrentSigningKey returns the current HMAC signing key under RLock.
// Callers MUST NOT mutate the returned slice; it shares the backing array
// with the field. Returns nil if BootstrapCapabilityState has not run.
func (a *API) CurrentSigningKey() []byte {
	a.signingKeyMu.RLock()
	defer a.signingKeyMu.RUnlock()
	return a.signingKey
}

// RelayHandler returns the http.Handler served on the relay loopback TCP port
// — the surface the Wails desktop GUI reaches over 127.0.0.1:<relayPort> (it
// cannot reach the daemon Unix socket). This is the relay server (sessions WS)
// wrapped with the remote terminal WS proxy route (/api/relay/remote/{sid}/ws),
// an *API method that cannot live in the relay package without an import
// cycle. Exposed so external tests can drive the relay surface exactly as the
// webview does (mirrors Handler()).
func (a *API) RelayHandler() http.Handler {
	server := relay.NewServer(a.engine.Manager(), a.engine.Backend())
	return a.wrapRelayWithRemoteFiles(server)
}

// StartRelay creates the relay HTTP server and starts it on a random TCP port.
// Returns the allocated port. Must be called after NewAPI.
func (a *API) StartRelay() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("relay listener: %w", err)
	}
	a.serveRelay(ln)
	return a.relayPort, nil
}

// serveRelay serves the relay on ln: one StartRelay opened, or one received
// in a handoff (so the port stays the same).
func (a *API) serveRelay(ln net.Listener) {
	a.relayPort = ln.Addr().(*net.TCPAddr).Port
	a.relayLn = ln
	go http.Serve(ln, a.RelayHandler()) //nolint:errcheck
}

// serveOn serves the API on a unix listener and the relay on a TCP listener
// received in a handoff (the counterpart of Start and StartRelay).
func (a *API) serveOn(ln, relayLn net.Listener) {
	a.ln = ln
	go http.Serve(ln, a.handler) //nolint:errcheck
	a.serveRelay(relayLn)
	a.startDesktopPoller()
}

// Start validates the socket path, cleans up any stale socket, and begins
// serving on the platform daemon socket.
func (a *API) Start(socketPath string) error {
	if err := ValidateSocketPath(socketPath); err != nil {
		return err
	}
	if err := CleanupStaleSocket(socketPath); err != nil {
		return err
	}
	ln, err := listenDaemonSocket(socketPath)
	if err != nil {
		return err
	}
	a.ln = ln
	go http.Serve(ln, a.handler) //nolint:errcheck
	a.startDesktopPoller()
	return nil
}

// Stop closes the listener and removes the socket file when the platform uses
// a filesystem-backed socket.
//
// On Windows the returned error is diagnostic-only: winio.win32PipeListener.Close()
// (upstream) always returns nil, swallowing any overlapped-I/O cancellation error,
// so a nil return on Windows does not necessarily mean the close was clean. The
// Unix net.UnixListener.Close() path does propagate errors normally.
// Correspondingly, removeDaemonSocket short-circuits on Windows named-pipe paths
// (no filesystem entry to remove), so its return is also nil there by design;
// any error it surfaces today comes from os.Remove on the Unix path.
// Tests that assert api.Stop() == nil on Windows are therefore exercising a
// tautology and should be read as smoke checks, not error-path coverage.
func (a *API) Stop() error {
	// Stop web server if running.
	a.mu.Lock()
	ws := a.webServer
	a.webServer = nil
	a.mu.Unlock()
	if ws != nil {
		_ = ws.Stop()
	}
	a.stopDesktop()

	a.portForwards.CloseAll()

	// Close relay listener if open.
	if a.relayLn != nil {
		_ = a.relayLn.Close()
		a.relayLn = nil
	}

	if a.ln == nil {
		return nil
	}
	addr := a.ln.Addr().String()
	err := a.ln.Close()
	_ = removeDaemonSocket(addr)
	a.ln = nil
	return err
}

// Addr returns the listener address (for test inspection).
func (a *API) Addr() net.Addr {
	if a.ln == nil {
		return nil
	}
	return a.ln.Addr()
}

// writeJSON writes v as a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (a *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, HealthResponse{Status: "ok", Version: BuildVersion, API: APIVersion, Handoff: HandoffVersion, Launchd: launchdManaged, PID: os.Getpid()})
}

// shutdownExit is how /shutdown ends the process; a test replaces it.
var shutdownExit = os.Exit

// shutdownDesktopGrace bounds how long /shutdown waits for the desktop to stop
// (Manager.Close clears captureActive first, so a late exit still counts clean).
// Short: a replacement daemon waits on the lock for this process to go.
const shutdownDesktopGrace = 500 * time.Millisecond

func (a *API) handleShutdown(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	go func() {
		stopped := make(chan struct{})
		go func() { defer close(stopped); a.stopDesktop() }()
		select {
		case <-stopped:
		case <-time.After(shutdownDesktopGrace):
		}
		time.Sleep(50 * time.Millisecond)
		shutdownExit(0)
	}()
}

func (a *API) handleListSessions(w http.ResponseWriter, r *http.Request) {
	sessions := a.engine.ListSessions()
	if sessions == nil {
		sessions = []SessionInfo{}
	}

	// A session is shared exactly when it has a share record.
	a.mu.RLock()
	for i := range sessions {
		_, sessions[i].WebEnabled = a.shares[sessions[i].ID]
	}
	a.mu.RUnlock()

	writeJSON(w, http.StatusOK, sessions)
}

func (a *API) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Use background context — the PTY must outlive the HTTP request.
	// r.Context() would kill the session when the response is sent.
	id, err := a.engine.CreateSession(context.Background(), req.Name, req.WorkDir, req.Cols, req.Rows, a.sessionOnExit())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Phase 87 / SEC-01: creating a session does NOT share it. The user must
	// turn sharing on via PUT /sessions/{id}/share (D-06 grant gesture).
	// This closes the SEC-01 finding: tailnet peers can no longer discover
	// newly-created sessions without an explicit share gesture.

	writeJSON(w, http.StatusCreated, CreateResponse{ID: id})
}

// sessionOnExit is the onExit callback for the share grace period (D-12).
// When the session exits naturally, drop its share after 10 seconds so web
// viewers see final output first. A no-op for sessions that were never
// shared. A handoff re-attaches it to every adopted session.
//
// Dropping the share clears its grants (D-15, RESEARCH Pitfall 1): session
// end invalidates all outstanding capabilities. Leaving grants in the map
// would leak memory AND could (theoretically) allow a recycled session ID
// to inherit stale grants.
func (a *API) sessionOnExit() func(string, int) {
	return func(sessionID string, exitCode int) {
		time.AfterFunc(10*time.Second, func() {
			a.runSessionExitCleanup(sessionID)
		})
	}
}

// runSessionExitCleanup drops the session's share (grants cleared, session
// disabled, share server stopped if it was the last share). Invoked 10 seconds after the session PTY exits (handleCreateSession
// onExit). Extracted so tests can invoke the cleanup synchronously without
// waiting for the 10-second grace timer.
func (a *API) runSessionExitCleanup(sessionID string) {
	a.mu.Lock()
	a.unshareLocked(sessionID) // D-15 also applies on natural exit (Pitfall 1)
	a.mu.Unlock()
}

// runSessionExitCleanupForTest is a test-only alias for runSessionExitCleanup.
// Documented on the test surface so the test package can invoke the exact same
// cleanup routine that the onExit callback uses, without waiting on the
// 10-second time.AfterFunc grace period.
func (a *API) runSessionExitCleanupForTest(sessionID string) {
	a.runSessionExitCleanup(sessionID)
}

func (a *API) handleGetSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sessions := a.engine.ListSessions()
	for _, s := range sessions {
		if s.ID == id {
			writeJSON(w, http.StatusOK, s)
			return
		}
	}
	http.Error(w, "session not found", http.StatusNotFound)
}

func (a *API) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := a.engine.KillSession(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Run the shared teardown synchronously on the explicit-kill path, without
	// the 10s grace period. Safe: KillSession marks the session killed, so the
	// natural-exit goroutine returns early and never arms its own cleanup.
	a.runSessionExitCleanup(id)
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handleRenameSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req RenameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := a.engine.RenameSession(id, req.Name); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleGetSessionTailLines returns the last n plain-text lines from the
// session's scrollback buffer. Phase 132 / CARD-07.
// CR-02: clamp n to [1..20] at the HTTP layer, mirroring the app.go Wails binding.
// The spec documents a [1..20] contract; without this clamp any caller on the
// Unix socket could request n=1000000 and bypass the intent of the spec.
func (a *API) handleGetSessionTailLines(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	n := 4 // default
	if nStr := r.URL.Query().Get("n"); nStr != "" {
		if parsed, err := strconv.Atoi(nStr); err == nil && parsed > 0 {
			n = parsed
		}
	}
	if n > 20 {
		n = 20 // mirror app.go clamp — enforce [1..20] at the daemon HTTP boundary
	}
	lines := a.engine.GetSessionTailLines(id, n)
	if lines == nil {
		lines = []string{}
	}
	writeJSON(w, http.StatusOK, TailLinesResponse{Lines: lines})
}

// handleGetSessionStyledTailLines returns the last n styled-cell lines from
// the session's scrollback buffer. Phase 139 / CARD-05.
// Mirrors handleGetSessionTailLines exactly — same n-clamp [1..20] defense.
func (a *API) handleGetSessionStyledTailLines(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	n := 4 // default
	if nStr := r.URL.Query().Get("n"); nStr != "" {
		if parsed, err := strconv.Atoi(nStr); err == nil && parsed > 0 {
			n = parsed
		}
	}
	if n > 20 {
		n = 20 // mirror app.go clamp — enforce [1..20] at the daemon HTTP boundary
	}
	spans := a.engine.GetSessionStyledTailLines(id, n)
	if spans == nil {
		spans = [][]StyledSpan{}
	}
	writeJSON(w, http.StatusOK, StyledTailLinesResponse{Lines: spans})
}

func (a *API) handleGetStartMinimized(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"startMinimized": a.engine.GetStartMinimized()})
}

func (a *API) handleSetStartMinimized(w http.ResponseWriter, r *http.Request) {
	var req struct {
		StartMinimized bool `json:"startMinimized"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	a.engine.SetStartMinimized(req.StartMinimized)
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handleGetAppearance(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"appearance": a.engine.GetAppearance()})
}

func (a *API) handleSetAppearance(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Appearance string `json:"appearance"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := a.engine.SetAppearance(req.Appearance); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// guestPluginConfigJSON is the /api/plugin-config payload (and its SSE
// frames): the plugin settings flattened, plus the host's Terminal Font under
// "terminalFont" so web guests render with it (quick 261116-ku9). nil on a
// marshal error, so the handler answers 503 as before.
func (a *API) guestPluginConfigJSON() []byte {
	b, err := json.Marshal(struct {
		PluginSettings
		TerminalFont TerminalFont `json:"terminalFont"`
	}{a.engine.GetPluginSettings(), a.engine.GetTerminalFont()})
	if err != nil {
		return nil
	}
	return b
}

func (a *API) handleGetTerminalFont(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.engine.GetTerminalFont())
}

func (a *API) handleSetTerminalFont(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	var req TerminalFont
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := a.engine.SetTerminalFont(req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleGetPluginSettings returns the engine's current PluginSettings as a
// JSON object (direct struct shape — no wrapper map).
func (a *API) handleGetPluginSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.engine.GetPluginSettings())
}

// handleSetPluginSettings accepts a full PluginSettings struct as the body
// (full-replace semantic; the route is PATCH for consistency with surrounding
// settings routes, not for partial-update semantics) and persists it.
//
// Defense-in-depth (RESEARCH §Security):
//   - http.MaxBytesReader caps body at 8 KiB (T-92-02 mitigation).
//   - DisallowUnknownFields rejects schema poisoning attempts (T-92-03).
//   - PluginSettings has no SchemaVersion field, so a downgrade attack via
//     body crafting is impossible (T-92-04).
func (a *API) handleSetPluginSettings(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8192)

	var req PluginSettings
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	a.engine.SetPluginSettings(req)
	w.WriteHeader(http.StatusNoContent)
}

// handleSetSearchConfig accepts a SearchConfig struct and persists ONLY that
// sub-key of PluginSettings (Phase 94-07 WR-03 gap closure). Defense-in-depth
// mirrors handleSetPluginSettings: 8 KiB body cap + DisallowUnknownFields.
//
// The route is PATCH /settings/search-config — sibling to /settings/plugins —
// chosen for symmetry with the other sub-key-style PATCH /settings/* routes
// (e.g. /settings/web-links-config).
func (a *API) handleSetSearchConfig(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8192)

	var req SearchConfig
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	a.engine.SetSearchConfig(req)
	w.WriteHeader(http.StatusNoContent)
}

// handleSetWebLinksConfig accepts a WebLinksConfig struct and persists ONLY
// that sub-key of PluginSettings (Phase 95 LNK-05 / LNK-06). Defense-in-depth
// mirrors handleSetSearchConfig: 8 KiB body cap + DisallowUnknownFields.
//
// The route is PATCH /settings/web-links-config — sibling to
// /settings/search-config — chosen for symmetry with the other sub-key-style
// PATCH routes in this API.
func (a *API) handleSetWebLinksConfig(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8192)

	var req WebLinksConfig
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	// WR-02: validate Modifier against the four documented literals so a
	// typoed value or corrupted settings.json cannot silently disable the
	// entire feature (isModifierPressed falls through every if and
	// returns false → every modifier-click is gated off with no UX
	// feedback). The struct comment in plugin_settings.go documents
	// these four values; here we enforce them at the API boundary.
	switch req.Modifier {
	case "platform", "cmd", "ctrl", "none":
		// ok
	default:
		http.Error(w, "modifier must be one of: platform, cmd, ctrl, none", http.StatusBadRequest)
		return
	}
	a.engine.SetWebLinksConfig(req)
	w.WriteHeader(http.StatusNoContent)
}

// handleSetImageConfig accepts an ImageConfig struct and persists ONLY
// that sub-key of PluginSettings (Phase 96 IMG-02). Defense-in-depth
// mirrors handleSetWebLinksConfig: 8 KiB body cap (T-96-02-03) +
// DisallowUnknownFields (T-96-02-02).
//
// The route is PATCH /settings/image-config — sibling to
// /settings/web-links-config — chosen for symmetry with the other
// sub-key-style PATCH routes in this API.
//
// IMG-02 range gate (T-96-02-01): StorageLimit must be in [1, 1000] MB.
// Reject 0 (no images render — defeat-the-feature footgun) and >1000
// (defeats tab-OOM mitigation per ROADMAP Phase 96 SC-3; the upstream
// addon-image default is 128 MB but Portty locks 16 MB by default,
// allowing user override up to 1000 MB hard cap for hypothetical future
// power-user advanced disclosure).
func (a *API) handleSetImageConfig(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8192)

	var req ImageConfig
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.StorageLimit < 1 || req.StorageLimit > 1000 {
		http.Error(w, "storageLimit must be in range [1, 1000]", http.StatusBadRequest)
		return
	}
	a.engine.SetImageConfig(req)
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handleRelayPort(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, RelayPortResponse{Port: a.relayPort})
}

// handleRegenerateSigningKey replaces capability.key on disk, swaps the
// in-memory key and re-mints every share with it (D-16 panic button): every
// old link stops verifying, shares stay on with new links.
func (a *API) handleRegenerateSigningKey(w http.ResponseWriter, r *http.Request) {
	newKey, err := capability.GenerateKey()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	store := capability.NewFileKeyStore(a.engine.configDir)
	if err := store.Save(newKey); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	a.signingKeyMu.Lock()
	a.signingKey = newKey
	a.signingKeyMu.Unlock()

	a.mu.Lock()
	if ws := a.webServer; ws != nil {
		ws.SetSigningKey(newKey)
		a.remintSharesLocked(ws)
	}
	mgr := a.desktopMgr
	a.mu.Unlock()
	// The guest link was signed under the old key too, so a running guest share
	// ends first: its guests see 4433 share_ended, not token rejected.
	guestEnded := a.endGuestShare(desktop.ReasonKeyRegenerated)
	// Every desktop viewer's link was signed under the old key. Closed after
	// a.mu is released: a close waits on the viewer's end of the handshake.
	switch {
	case mgr == nil:
	case guestEnded:
		a.closeOwnViewers(mgr, desktop.CloseTokenRejected, "token rejected")
	default:
		mgr.CloseAll(desktop.CloseTokenRejected, "token rejected")
	}
	w.WriteHeader(http.StatusOK)
}

func (a *API) handleTailnetPeers(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	peers := a.probePeers(ctx)
	writeJSON(w, http.StatusOK, peers)
}
