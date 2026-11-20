package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/scottkw/portty/internal/tailnet"
)

// DaemonClient is a typed Go client that communicates with the daemon API
// over a Unix socket or Windows named pipe.
type DaemonClient struct {
	http *http.Client
	base string
	ctx  context.Context // nil: context.Background(); see WithContext
}

// WithContext returns a copy of c whose requests end when ctx does, so a
// cancel unblocks a call to a daemon that accepted the connection and hung
// (V-05b; Quit All cancels the ensure, IN-07).
func (c *DaemonClient) WithContext(ctx context.Context) *DaemonClient {
	cc := *c
	cc.ctx = ctx
	return &cc
}

func (c *DaemonClient) reqCtx() context.Context {
	if c.ctx == nil {
		return context.Background()
	}
	return c.ctx
}

// NewDaemonClient creates a DaemonClient that dials the given daemon socket path.
// All HTTP requests use http://daemon as the base URL — the custom transport
// rewrites the actual network connection to the platform daemon socket.
func NewDaemonClient(socketPath string) *DaemonClient {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialDaemonSocket(ctx, socketPath)
		},
	}
	return &DaemonClient{
		// A wedged daemon must not hang callers forever (review WR-05). Every
		// request here is a bounded JSON call (tailnet peers caps itself at 10 s);
		// there is no streaming endpoint on this client.
		http: &http.Client{Transport: transport, Timeout: 30 * time.Second},
		base: "http://daemon",
	}
}

// Health returns nil if the daemon is reachable and returns {"status":"ok"}.
func (c *DaemonClient) Health() error {
	var h HealthResponse
	return c.doJSON(http.MethodGet, "/health", nil, &h)
}

// GetDaemonVersion returns the version reported by the daemon's health endpoint.
// It is empty if the daemon doesn't report a version (pre-v2.1.2). A failed
// read is an error, never "" (08-06-REVIEW IN-06: unknown is not a mismatch).
func (c *DaemonClient) GetDaemonVersion() (string, error) {
	var h HealthResponse
	if err := c.doJSON(http.MethodGet, "/health", nil, &h); err != nil {
		return "", err
	}
	return h.Version, nil
}

// HealthInfo reads /health once: version, API and handoff support.
func (c *DaemonClient) HealthInfo() (HealthResponse, error) {
	var h HealthResponse
	err := c.doJSON(http.MethodGet, "/health", nil, &h)
	return h, err
}

// DaemonAPI is the API version h reports; an absent "api" means 1, the
// v1.0/v1.1 route set (08-06-REVIEW-FIX IN-03).
func DaemonAPI(h HealthResponse) int {
	if h.API == 0 {
		return 1
	}
	return h.API
}

// MinDaemonAPI is the lowest daemon API this GUI and CLI can use; below it
// they say "update needed" (UPGR-04). 1 keeps a v1.1 daemon (no "api" field)
// usable after Not Now or --keep-daemon. A var so tests and vmpreview raise it.
var MinDaemonAPI = 1

// ListSessions returns all current sessions.
func (c *DaemonClient) ListSessions() ([]SessionInfo, error) {
	var sessions []SessionInfo
	if err := c.doJSON(http.MethodGet, "/sessions", nil, &sessions); err != nil {
		return nil, err
	}
	if sessions == nil {
		sessions = []SessionInfo{}
	}
	return sessions, nil
}

// CreateSession creates a new login-shell session and returns its ID.
// Empty workDir starts in the home directory.
// cols and rows specify the initial PTY dimensions; pass 0 for defaults (80x24).
func (c *DaemonClient) CreateSession(name, workDir string, cols, rows int) (string, error) {
	req := CreateRequest{Name: name, WorkDir: workDir, Cols: cols, Rows: rows}
	var resp CreateResponse
	if err := c.doJSON(http.MethodPost, "/sessions", req, &resp); err != nil {
		return "", err
	}
	return resp.ID, nil
}

// KillSession terminates the given session.
func (c *DaemonClient) KillSession(id string) error {
	return c.doJSON(http.MethodDelete, "/sessions/"+id, nil, nil)
}

// RenameSession changes the display name of a session.
func (c *DaemonClient) RenameSession(id, name string) error {
	return c.doJSON(http.MethodPatch, "/sessions/"+id+"/name", RenameRequest{Name: name}, nil)
}

// GetSessionTailLines returns the last n plain-text lines from the session's
// scrollback buffer, with ANSI escape sequences and framing bytes stripped.
// Phase 132 / CARD-07.
func (c *DaemonClient) GetSessionTailLines(id string, n int) ([]string, error) {
	var resp TailLinesResponse
	if err := c.doJSON(http.MethodGet, fmt.Sprintf("/sessions/%s/tail?n=%d", id, n), nil, &resp); err != nil {
		return nil, err
	}
	return resp.Lines, nil
}

// GetSessionStyledTailLines returns the last n styled-cell lines from the
// session's scrollback buffer via GET /sessions/{id}/styled-tail.
// Phase 139 / CARD-05.
func (c *DaemonClient) GetSessionStyledTailLines(id string, n int) ([][]StyledSpan, error) {
	var resp StyledTailLinesResponse
	if err := c.doJSON(http.MethodGet, fmt.Sprintf("/sessions/%s/styled-tail?n=%d", id, n), nil, &resp); err != nil {
		return nil, err
	}
	return resp.Lines, nil
}

// GetStartMinimized returns the persisted start-minimized preference.
func (c *DaemonClient) GetStartMinimized() (bool, error) {
	var resp map[string]bool
	if err := c.doJSON(http.MethodGet, "/settings/start-minimized", nil, &resp); err != nil {
		return false, err
	}
	return resp["startMinimized"], nil
}

// SetStartMinimized persists the start-minimized preference.
func (c *DaemonClient) SetStartMinimized(val bool) error {
	return c.doJSON(http.MethodPatch, "/settings/start-minimized",
		map[string]bool{"startMinimized": val}, nil)
}

// GetAppearance returns the persisted Appearance override (system|light|dark).
func (c *DaemonClient) GetAppearance() (string, error) {
	var resp map[string]string
	if err := c.doJSON(http.MethodGet, "/settings/appearance", nil, &resp); err != nil {
		return "", err
	}
	return resp["appearance"], nil
}

// SetAppearance persists the Appearance override; the daemon rejects values
// outside system|light|dark.
func (c *DaemonClient) SetAppearance(v string) error {
	return c.doJSON(http.MethodPatch, "/settings/appearance",
		map[string]string{"appearance": v}, nil)
}

// GetTerminalFont returns the persisted Terminal Font.
func (c *DaemonClient) GetTerminalFont() (TerminalFont, error) {
	var f TerminalFont
	err := c.doJSON(http.MethodGet, "/settings/terminal-font", nil, &f)
	return f, err
}

// SetTerminalFont persists the Terminal Font; the daemon validates it.
func (c *DaemonClient) SetTerminalFont(f TerminalFont) error {
	return c.doJSON(http.MethodPatch, "/settings/terminal-font", f, nil)
}

// GetPluginSettings fetches the persisted plugin enable/disable preferences.
func (c *DaemonClient) GetPluginSettings() (PluginSettings, error) {
	var resp PluginSettings
	if err := c.doJSON(http.MethodGet, "/settings/plugins", nil, &resp); err != nil {
		return PluginSettings{}, err
	}
	return resp, nil
}

// SetPluginSettings persists the plugin enable/disable preferences via the
// daemon API. The full PluginSettings struct is sent in the body (full
// replace semantic; the route is PATCH for consistency with surrounding
// settings routes, not for partial-update semantics).
func (c *DaemonClient) SetPluginSettings(s PluginSettings) error {
	return c.doJSON(http.MethodPatch, "/settings/plugins", s, nil)
}

// SetSearchConfig persists ONLY the SearchConfig sub-key of PluginSettings via
// the daemon API. Phase 94-07 WR-03 gap closure — the find bar must not race
// PluginsSection's stale local edit buffer by writing the full PluginSettings
// from a snapshot prop. This sub-key writer routes to the engine's
// SetSearchConfig method which mutates only e.pluginSettings.SearchConfig
// under the engine mutex.
func (c *DaemonClient) SetSearchConfig(cfg SearchConfig) error {
	return c.doJSON(http.MethodPatch, "/settings/search-config", cfg, nil)
}

// SetWebLinksConfig persists ONLY the WebLinksConfig sub-key of PluginSettings
// via the daemon API. Phase 95 LNK-05 / LNK-06 — mirrors Phase 94-07
// SetSearchConfig. Routes to engine.SetWebLinksConfig which mutates only
// e.pluginSettings.WebLinksConfig under the engine mutex, preserving
// PluginsSection's edit buffer for unrelated plugin booleans.
func (c *DaemonClient) SetWebLinksConfig(cfg WebLinksConfig) error {
	return c.doJSON(http.MethodPatch, "/settings/web-links-config", cfg, nil)
}

// SetImageConfig persists ONLY the ImageConfig sub-key of PluginSettings via
// the daemon API. Phase 96 IMG-02 — mirrors Phase 95 SetWebLinksConfig and
// Phase 94-07 SetSearchConfig. Routes to engine.SetImageConfig which mutates
// only e.pluginSettings.ImageConfig under the engine mutex, preserving
// PluginsSection's edit buffer for unrelated plugin booleans.
//
// The daemon validates StorageLimit is in [1, 1000] MB; out-of-range values
// surface as a non-nil error from this call (HTTP 400 from the handler).
func (c *DaemonClient) SetImageConfig(cfg ImageConfig) error {
	return c.doJSON(http.MethodPatch, "/settings/image-config", cfg, nil)
}

// GetRelayPort returns the TCP port the daemon's relay server is listening on.
func (c *DaemonClient) GetRelayPort() (int, error) {
	var resp RelayPortResponse
	if err := c.doJSON(http.MethodGet, "/relay-port", nil, &resp); err != nil {
		return 0, err
	}
	return resp.Port, nil
}

// ShutdownDaemon sends POST /shutdown to terminate the daemon process.
// Connection-reset errors are expected (daemon exits before response completes)
// and are treated as success, unless a daemon still answers /health on a
// fresh connection (a takeover after a handoff): it gets /shutdown once more.
// A daemon frozen for a handoff answers 503: it is waited out (doRetrying)
// and still returned as an error once the wait is spent.
func (c *DaemonClient) ShutdownDaemon() error {
	hc := *c.http
	hc.Timeout = 2 * time.Second // per attempt
	post := func() (*http.Response, error) {
		return c.doRetrying(&hc, func() (*http.Request, error) {
			return http.NewRequestWithContext(c.reqCtx(), http.MethodPost, c.base+"/shutdown", nil)
		})
	}
	resp, err := post()
	if err != nil && c.answersAfterDrop(&hc, err) {
		resp, err = post() // IN-05: once, to the daemon that still answers
	}
	if err != nil {
		// Connection reset is expected — daemon exited. Treat as success.
		return nil
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusServiceUnavailable {
		return fmt.Errorf("daemon API POST /shutdown: status %d", resp.StatusCode)
	}
	return nil
}

// answersAfterDrop reports whether, after err ended a /shutdown, a daemon
// still answers /health on a fresh connection (IN-05). During a handoff the
// exiting old daemon drops connections while a takeover keeps the sessions,
// so a drop alone does not prove the sessions' daemon stopped. A timeout is a
// hung daemon, not a drop: Quit All does not wait on it a second time.
// Whichever version answers is the one that owns the socket now.
func (c *DaemonClient) answersAfterDrop(hc *http.Client, err error) bool {
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() || c.reqCtx().Err() != nil {
		return false
	}
	c.http.CloseIdleConnections() // a pooled connection may still lead to the exiting daemon
	req, err := http.NewRequestWithContext(c.reqCtx(), http.MethodGet, c.base+"/health", nil)
	if err != nil {
		return false
	}
	resp, err := hc.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

// frozenRetryBudget bounds how long a mutating call waits out a daemon frozen
// for a handoff. It must outlast 09-04's longest freeze, maxFrozen (18 s); a
// 09-04 test pins that sum below this. A var so tests can shrink it.
var frozenRetryBudget = 20 * time.Second

// doRetrying sends the request newReq builds with hc, and sends a mutating one
// again while the daemon answers 503 with Retry-After (09-04's handoff freeze),
// waiting Retry-After seconds (1 s if unreadable, at most 2 s) each time, until
// frozenRetryBudget is spent; then the last 503 is returned. A GET is never
// retried: a frozen daemon still serves reads. A 503 without Retry-After is
// no freeze (e.g. Tailscale off) and is returned at once.
func (c *DaemonClient) doRetrying(hc *http.Client, newReq func() (*http.Request, error)) (*http.Response, error) {
	deadline := time.Now().Add(frozenRetryBudget)
	for {
		req, err := newReq()
		if err != nil {
			return nil, err
		}
		resp, err := hc.Do(req)
		if err != nil || req.Method == http.MethodGet || resp.StatusCode != http.StatusServiceUnavailable || resp.Header.Get("Retry-After") == "" {
			return resp, err
		}
		wait := time.Second
		if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && s > 0 {
			wait = min(time.Duration(s)*time.Second, 2*time.Second)
		}
		if time.Now().Add(wait).After(deadline) {
			return resp, nil
		}
		resp.Body.Close()
		time.Sleep(wait)
	}
}

// RegenerateSigningKey rotates the HMAC signing key (D-16 panic button).
// Every old link stops verifying; shares are re-minted with new links.
func (c *DaemonClient) RegenerateSigningKey() error {
	return c.doJSON(http.MethodPost, "/capability/regenerate-key", nil, nil)
}

// ListTailnetPeers returns discovered tailnet peers running Portty.
func (c *DaemonClient) ListTailnetPeers() ([]tailnet.Peer, error) {
	var peers []tailnet.Peer
	if err := c.doJSON(http.MethodGet, "/tailnet/peers", nil, &peers); err != nil {
		return nil, err
	}
	if peers == nil {
		peers = []tailnet.Peer{}
	}
	return peers, nil
}

// GetSessionShare reports whether a session is shared and returns its link.
func (c *DaemonClient) GetSessionShare(id string) (ShareResponse, error) {
	var resp ShareResponse
	err := c.doJSON(http.MethodGet, "/sessions/"+id+"/share", nil, &resp)
	return resp, err
}

// SetSessionShared turns sharing on or off. Share failures come back as
// *ShareError carrying a stable code.
func (c *DaemonClient) SetSessionShared(id string, shared bool) (ShareResponse, error) {
	b, err := json.Marshal(ShareRequest{Shared: shared})
	if err != nil {
		return ShareResponse{}, err
	}
	req, err := http.NewRequestWithContext(c.reqCtx(), http.MethodPut, c.base+"/sessions/"+id+"/share", bytes.NewReader(b))
	if err != nil {
		return ShareResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return ShareResponse{}, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		var e ShareErrorResponse
		if json.Unmarshal(body, &e) == nil && e.Error != "" {
			return ShareResponse{}, &ShareError{Code: e.Error, Detail: e.Detail}
		}
		return ShareResponse{}, fmt.Errorf("daemon API PUT share: status %d: %s", resp.StatusCode, body)
	}
	var out ShareResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return ShareResponse{}, fmt.Errorf("decode response: %w", err)
	}
	return out, nil
}

// ConnectPeer asks the daemon to connect to a session shared from another Mac
// (POST /peers/connect). Failures come back as *PeerConnectError.
func (c *DaemonClient) ConnectPeer(host, session string) (PeerConnectResponse, error) {
	b, err := json.Marshal(PeerConnectRequest{Host: host, Session: session})
	if err != nil {
		return PeerConnectResponse{}, err
	}
	resp, err := c.http.Post(c.base+"/peers/connect", "application/json", bytes.NewReader(b))
	if err != nil {
		return PeerConnectResponse{}, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		var e PeerConnectErrorResponse
		if json.Unmarshal(body, &e) == nil && e.Error != "" {
			return PeerConnectResponse{}, &PeerConnectError{Code: e.Error, Detail: e.Detail, Hosts: e.Hosts, Sessions: e.Sessions}
		}
		return PeerConnectResponse{}, fmt.Errorf("daemon API POST /peers/connect: status %d: %s", resp.StatusCode, body)
	}
	var out PeerConnectResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return PeerConnectResponse{}, fmt.Errorf("decode response: %w", err)
	}
	return out, nil
}

// doJSON is a shared request/response helper. It marshals body (if non-nil),
// sends the request, checks the status code, and decodes result (if non-nil).
func (c *DaemonClient) doJSON(method, path string, body, result any) error {
	var b []byte
	if body != nil {
		var err error
		if b, err = json.Marshal(body); err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
	}

	resp, err := c.doRetrying(c.http, func() (*http.Request, error) {
		var reqBody io.Reader
		if body != nil {
			reqBody = bytes.NewReader(b)
		}
		req, err := http.NewRequestWithContext(c.reqCtx(), method, c.base+path, reqBody)
		if err != nil {
			return nil, fmt.Errorf("new request: %w", err)
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		return req, nil
	})
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("daemon API %s %s: status %d: %s", method, path, resp.StatusCode, string(b))
	}

	if result != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}
