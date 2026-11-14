package daemon

// Parent mux for the relay loopback surface.
//
// The Wails desktop GUI reaches the daemon over the relay loopback TCP server
// (127.0.0.1:<relayPort>), never the daemon Unix socket — the webview cannot
// reach the socket. The remote terminal WS reverse proxy (handleRemoteSessionWS)
// is an *API method (it needs the RemoteCapStore + outbound HTTPS client), so it
// cannot be registered inside relay.NewServer without an import cycle (daemon
// imports relay). Instead we wrap the relay server in a parent mux that owns
// that route and falls through ("/") to the relay server for everything else.

import (
	"net/http"
)

// wrapRelayWithRemoteFiles returns a handler that serves the remote terminal WS
// proxy route and falls through to the given relay server for all other paths
// (sessions WS, health). The name is historical: it used to also mount the
// removed remote file-browser proxy routes.
func (a *API) wrapRelayWithRemoteFiles(relayServer http.Handler) http.Handler {
	mux := http.NewServeMux()

	// Phase 134-06: cap-gated remote terminal WS reverse proxy. NO CORS wrapper
	// and NO OPTIONS preflight: a WebSocket upgrade is not a CORS-preflighted
	// request; the inbound Origin is enforced at websocket.Accept
	// (relay.LoopbackOriginPatterns), and the outbound Origin is injected on the
	// upstream dial (134-RESEARCH Pattern 3).
	mux.HandleFunc("GET /api/relay/remote/{sessionID}/ws", a.handleRemoteSessionWS)

	// Everything else falls through to the relay server. Go 1.22+ mux: the "/"
	// pattern is the least-specific match, so the explicit route above wins.
	mux.Handle("/", relayServer)
	return mux
}
