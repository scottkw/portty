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

// APIVersion is the daemon API this build serves (UPGR-04). v1.0 and v1.1
// send no "api" field and count as 1. 3 adds the preview route (PreviewAPI).
const APIVersion = 3

// PreviewAPI is the lowest daemon API that serves POST /sessions/{id}/preview;
// `portty open` needs it (CONTEXT "Session id").
const PreviewAPI = 3

// PreviewResponse is the 202 body of POST /sessions/{id}/preview: how many
// viewers the preview streams to.
type PreviewResponse struct {
	Viewers int `json:"viewers"`
}

// HealthResponse is the response body for GET /health.
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version,omitempty"`
	API     int    `json:"api,omitempty"`     // absent: 1, the v1.0/v1.1 route set
	Handoff int    `json:"handoff,omitempty"` // absent: cannot hand off its sessions
}

// RelayPortResponse is the response body for GET /relay-port.
type RelayPortResponse struct {
	Port int `json:"port"`
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

// ShareRequest is the request body for PUT /sessions/{id}/share.
type ShareRequest struct {
	Shared bool `json:"shared"`
}

// ShareResponse is the response body for GET and PUT /sessions/{id}/share.
// URL carries the full-control capability; treat it like a password.
type ShareResponse struct {
	Shared     bool   `json:"shared"`
	URL        string `json:"url,omitempty"`
	FirstShare bool   `json:"firstShare,omitempty"`
}

// ShareErrorResponse is the error body for PUT /sessions/{id}/share.
type ShareErrorResponse struct {
	Error  string `json:"error"`
	Detail string `json:"detail,omitempty"`
}

// Share error codes (ShareErrorResponse.Error).
const (
	ShareErrNotFound     = "not_found"
	ShareErrTailscaleOff = "tailscale_off"
	ShareErrHTTPSOff     = "https_off"
	ShareErrPortInUse    = "port_in_use"
	ShareErrStartFailed  = "start_failed"
)

// ShareError is a share failure with a stable code for the CLI and GUI copy.
type ShareError struct {
	Code   string
	Detail string
}

func (e *ShareError) Error() string {
	if e.Detail == "" {
		return e.Code
	}
	return e.Code + ": " + e.Detail
}

// PeerConnectRequest is the request body for POST /peers/connect. Session may be
// empty: the host's only shared session is picked, or the list comes back.
type PeerConnectRequest struct {
	Host    string `json:"host"`
	Session string `json:"session"`
}

// PeerConnectResponse is the success body for POST /peers/connect. The capability
// is stored in the daemon; URL carries it for the owner's GUI only.
type PeerConnectResponse struct {
	SessionID string `json:"sessionId"`
	Name      string `json:"name"`
	Host      string `json:"host"`
	URL       string `json:"url"`
}

// PeerSessionChoice names one shared session in a PeerConnectErrorResponse.
type PeerSessionChoice struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// PeerConnectErrorResponse is the error body for POST /peers/connect.
type PeerConnectErrorResponse struct {
	Error    string              `json:"error"`
	Detail   string              `json:"detail,omitempty"`
	Hosts    []string            `json:"hosts,omitempty"`
	Sessions []PeerSessionChoice `json:"sessions,omitempty"`
}

// Peer connect error codes (PeerConnectErrorResponse.Error).
const (
	PeerErrTailscaleOff  = "tailscale_off"
	PeerErrUnknownHost   = "unknown_host"
	PeerErrNotSharing    = "not_sharing"
	PeerErrNoSuchSession = "no_such_session"
	PeerErrAmbiguous     = "ambiguous"
	PeerErrMultiple      = "multiple"
	PeerErrForbidden     = "forbidden"
	PeerErrUnreachable   = "unreachable"
)

// PeerConnectError is a POST /peers/connect failure with a stable code plus the
// host or session lists the CLI and GUI copy need.
type PeerConnectError struct {
	Code     string
	Detail   string
	Hosts    []string
	Sessions []PeerSessionChoice
}

func (e *PeerConnectError) Error() string {
	if e.Detail == "" {
		return e.Code
	}
	return e.Code + ": " + e.Detail
}
