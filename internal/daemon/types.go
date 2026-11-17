package daemon

// SessionInfo is the JSON-serialisable representation of a session.
type SessionInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	State       string `json:"state"`
	Status      string `json:"status"` // running/exited/errored, derived from State + ExitCode
	CreatedAt   string `json:"createdAt"`
	Hostname    string `json:"hostname"`
	WebEnabled  bool   `json:"webEnabled"`
	ViewerCount int    `json:"viewerCount"`        // Phase 168 / FIX-04, D-03: number of remote (web-origin) viewers only — excludes the app's own internal Origin=="local" WebSocket subscribers
	ExitCode    *int   `json:"exitCode,omitempty"` // nil while running; set when State is "stopped"
	Duration    *int   `json:"duration,omitempty"` // seconds since CreatedAt; set when State is "stopped"
	WorkDir     string `json:"workDir"`            // Phase 131 / GRID-02: EvalSymlinks-resolved session working directory; populated from engine.sessionWorkDirs map; enables Hub card grouping by directory
}

// CreateRequest is the request body for POST /sessions. Legacy "cli" and
// "args" keys from older clients are ignored: every session is a login shell.
type CreateRequest struct {
	Name    string `json:"name"`
	WorkDir string `json:"workDir"`
	Cols    int    `json:"cols,omitempty"`
	Rows    int    `json:"rows,omitempty"`
}

// CreateResponse is the response body for POST /sessions.
type CreateResponse struct {
	ID string `json:"id"`
}

// RenameRequest is the request body for PATCH /sessions/{id}/name.
type RenameRequest struct {
	Name string `json:"name"`
}

// TailLinesResponse is the response body for GET /sessions/{id}/tail.
// Phase 132 / CARD-07.
type TailLinesResponse struct {
	Lines []string `json:"lines"`
}

// StyledSpan is one styled cell in the VT grid — the wire type for
// GET /sessions/{id}/styled-tail. All color values are hex strings
// ("#rrggbb") or ANSI index markers ("ansi:N") or "" for terminal default.
// Phase 139 / CARD-05.
type StyledSpan struct {
	Char string `json:"c"`            // Unicode grapheme cluster
	FG   string `json:"fg,omitempty"` // "#rrggbb", "ansi:N", or "" (default fg)
	BG   string `json:"bg,omitempty"` // "#rrggbb", "ansi:N", or "" (default bg)
	Bold bool   `json:"b,omitempty"`  // true when bold attribute set
}

// StyledTailLinesResponse is the response body for GET /sessions/{id}/styled-tail.
// Phase 139 / CARD-05.
type StyledTailLinesResponse struct {
	Lines [][]StyledSpan `json:"lines"`
}

// HealthResponse is the response body for GET /health.
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version,omitempty"`
}

// RelayPortResponse is the response body for GET /relay-port.
type RelayPortResponse struct {
	Port int `json:"port"`
}

// WebServerStartRequest is the request body for POST /webserver/start.
type WebServerStartRequest struct {
	IP       string `json:"ip"`
	Port     int    `json:"port"`
	FQDN     string `json:"fqdn"`
	Mode     string `json:"mode"`     // "tailscale" | "local"
	Password string `json:"password"` // non-empty for local mode
}

// WebServerStartResponse is the response body for POST /webserver/start.
type WebServerStartResponse struct {
	URL string `json:"url"`
}

// WebServerStatusResponse is the response body for GET /webserver/status.
type WebServerStatusResponse struct {
	Running bool   `json:"running"`
	URL     string `json:"url"`
	Addr    string `json:"addr"`
	Mode    string `json:"mode"` // "tailscale" | "local" | ""
}

// WebServeRequest is the request body for POST /sessions/{id}/web-serve.
type WebServeRequest struct {
	Enabled bool `json:"enabled"`
}

// --- Phase 87 capability types (D-06, D-07, D-09, D-11) ------------------

// IssueCapabilitiesResponse is the response body for POST
// /sessions/{id}/capabilities. Each call produces TWO capabilities (D-07):
// one read-only link and one read-write link. Each capability is paired with
// a single-use 5-minute join code (D-09/D-11).
type IssueCapabilitiesResponse struct {
	ReadURL   string `json:"readUrl"`
	WriteURL  string `json:"writeUrl"`
	ReadCode  string `json:"readCode"`
	WriteCode string `json:"writeCode"`
}

// ExchangeJoinCodeRequest is the body for POST /join/exchange.
type ExchangeJoinCodeRequest struct {
	Code string `json:"code"`
}

// ExchangeJoinCodeResponse is the response body for POST /join/exchange.
// URL carries the resolved session URL with the `?cap=<token>` query
// parameter, which the caller follows to reach the gated session.
type ExchangeJoinCodeResponse struct {
	URL string `json:"url"`
}

// TerminalFont is the persisted Settings ▸ General ▸ Terminal Font value
// (quick 261116-ku9). An empty Family means the default stack (SF Mono first).
type TerminalFont struct {
	Family string `json:"family"`
	Size   int    `json:"size"`
}

// Terminal font bounds. The size range matches the per-tab ⌘+/⌘− zoom clamp.
const (
	DefaultTerminalFontSize  = 13
	MinTerminalFontSize      = 6
	MaxTerminalFontSize      = 32
	MaxTerminalFontFamilyLen = 64 // runes
)
