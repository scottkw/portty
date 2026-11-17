package relay

import (
	"encoding/json"
	"log"
	"net"
	"net/http"

	"github.com/coder/websocket"
	"github.com/scottkw/portty/internal/pty"
)

// Server is an HTTP handler that exposes relay sessions over WebSocket.
type Server struct {
	manager *HubManager
	backend pty.SessionBackend
	mux     *http.ServeMux
}

// NewServer creates a Server backed by the given HubManager and SessionBackend
// and registers routes. backend is used to forward resize events from connected
// WebSocket clients to the underlying PTY.
func NewServer(manager *HubManager, backend pty.SessionBackend) *Server {
	s := &Server{
		manager: manager,
		backend: backend,
		mux:     http.NewServeMux(),
	}
	s.mux.HandleFunc("GET /sessions/{id}/ws", s.handleSession)
	s.mux.HandleFunc("GET /sessions", s.handleListSessions)
	return s
}

// ServeHTTP implements http.Handler by delegating to the internal mux.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// handleSession upgrades the HTTP connection to WebSocket and starts pumping
// messages between the client and its session Hub.
//
// Hub.Join subscribes and captures the catch-up replay as one atomic step under
// the lock the drain loop records and broadcasts under, so every frame reaches
// the client exactly once: in the replay or via the Msgs channel. A ?resume=
// offset the ring still covers gets only the missed output (SHAR-07).
func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("id")

	// MC-05: parse client metadata from URL query params at upgrade time.
	// Local subscribers are never read-only; only the webserver sets ReadOnly,
	// from the verified capability's write perm.
	clientName := r.URL.Query().Get("client")
	if len(clientName) > 64 {
		clientName = clientName[:64] // cap identity name to prevent injection
	}

	hub, ok := s.manager.Get(sessionID)
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// Phase 88 (CONTEXT D-08/D-09): loopback-only Origin allowlist
		// replaces InsecureSkipVerify. The relay is bound to 127.0.0.1
		// by daemon/api.go, so in normal operation all clients are
		// loopback. This allowlist is belt-and-suspenders for the
		// landmine case where the listener is ever rebound to a
		// non-loopback interface — a future maintainer must consciously
		// add the new origin, which is the friction we want.
		//
		// Port derivation (RESEARCH Pitfall 7 option a): the Server
		// struct does not own its listener (daemon/api.go does), so we
		// read the port from r.Host. Empty port -> empty allowlist ->
		// library falls back to same-Host check, which is also correct
		// for a loopback-only deployment.
		OriginPatterns: loopbackOriginPatterns(r.Host),
	})
	if err != nil {
		// websocket.Accept already wrote an HTTP error response.
		return
	}

	ctx := r.Context()

	// Create subscriber with a buffered channel. CloseSlow disconnects the client
	// when the buffer fills up, preventing a slow client from blocking fan-out.
	sub := &Subscriber{
		Msgs: make(chan []byte, 256),
		Name: clientName,
	}
	sub.CloseSlow = func() {
		conn.Close(websocket.StatusPolicyViolation, "too slow")
	}

	sub.Origin = "local" // read by the host-authority resize arbiter

	// Join subscribes and captures the catch-up replay atomically (SHAR-07):
	// every frame reaches this viewer exactly once, in the replay or in Msgs.
	resume, hasResume := ParseResume(r.URL.Query().Get("resume"))
	replay := hub.Join(sub, resume, hasResume)
	NotifyViewerCount(hub) // push updated viewer count to all clients
	defer func() {
		hub.Unsubscribe(sub)
		NotifyViewerCount(hub)
	}()
	defer conn.CloseNow()

	// VIEW-03: push authoritative host grid BEFORE scrollback replay so that
	// replayed raw bytes land in a correctly-sized terminal grid. Direct conn.Write
	// (not sub.Msgs) guarantees ordering before any queued live output.
	if c, r := hub.Cols(), hub.Rows(); c > 0 && r > 0 {
		if err := conn.Write(ctx, websocket.MessageBinary, MakeResizeFrame(uint16(c), uint16(r))); err != nil {
			return
		}
	}

	// Phase 175 (BUG-04, #119) / Phase 5 (SHAR-07): the catch-up replay, bracketed
	// by MsgMeta{replay} and MsgMeta{offset}. A full replay is byte-faithful raw
	// scrollback while the ring is intact (#109) and the live emulator's current
	// screen once it wrapped (M-51); a delta is only the ring bytes after the
	// client's resume offset. See Hub.Join.
	for _, f := range replay.Frames() {
		if err := conn.Write(ctx, websocket.MessageBinary, f); err != nil {
			return
		}
	}
	log.Printf("viewer-diag: replay session=%s conn=%d mode=%s bytes=%d resume=%t",
		DiagID(sessionID), sub.ID, replay.Mode, replay.Bytes(), hasResume)

	// Read pump — runs in a separate goroutine, parsing client -> PTY frames.
	//
	// Phase 115 / Issue #60: per-subscriber InputAbsorber filters OSC 10/11
	// + DA1 replies emitted by xterm.js in response to terminal queries
	// before they reach PTY stdin. The webserver-layer handleWSSRelay
	// applies the same absorber for the web-share path; this one catches
	// the daemon-direct attach path used by the Wails desktop GUI and by
	// CLI `portty attach`. State must persist across MsgInput frames
	// because envelopes may split across frames — the zero-value
	// &InputAbsorber{} is the legitimate "outside, empty buffers" state.
	readDone := make(chan struct{})
	absorber := &InputAbsorber{}
	go func() {
		defer close(readDone)
		for {
			_, msg, err := conn.Read(ctx)
			if err != nil {
				return
			}
			msgType, payload, err := ParseFrame(msg)
			if err != nil {
				continue
			}
			// 0x30-0x37 are RETIRED (chat, presence, typing, alias, inject, self):
			// no case and no default arm, so old clients' frames are dropped.
			switch msgType {
			case MsgInput:
				if !sub.ReadOnly { // MC-03: discard input for read-only clients
					filtered := absorber.Filter(payload)
					if len(filtered) > 0 {
						_ = hub.WriteInput(filtered)
					}
				}
			case MsgResize2:
				if len(payload) >= 4 {
					cols := uint16(payload[0])<<8 | uint16(payload[1])
					rows := uint16(payload[2])<<8 | uint16(payload[3])
					_ = hub.ResizeClient(sub, int(cols), int(rows)) // host-authority arbiter (VIEW-01/VIEW-02)
				}
			case MsgPing:
				// Answer without blocking: a full queue means a "too slow" kick is
				// coming (T-05-06). No WebSocket heartbeat on this loopback relay.
				select {
				case sub.Msgs <- PongFrame:
				default:
				}
			}
		}
	}()

	// Write pump — forwards Hub broadcast frames to the WebSocket client.
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
			conn.Close(websocket.StatusNormalClosure, CloseReasonSessionEnded)
			return
		case <-readDone:
			return
		}
	}
}

// loopbackOriginPatterns returns the allowlist of origin patterns
// authorised to open a relay WebSocket. Three classes of caller exist:
//
//  1. Browsers connecting via http(s)://127.0.0.1:<port> or
//     http(s)://localhost:<port> — the dashboard and (in future) any
//     locally-served HTML.
//  2. The Wails desktop webview, whose origin is wails://wails.localhost
//     (macOS, Linux) or http://wails.localhost (Windows). Production
//     builds have no port; dev builds expose a Vite HMR port. Both
//     forms must be matched. The wails.localhost host is reserved by
//     the Wails runtime and cannot be impersonated by an external
//     browser, so allow-listing it does not weaken the loopback
//     security boundary.
//  3. CLI / Go HTTP clients (no Origin header) — coder/websocket allows
//     these unconditionally per its Accept() docs ("empty Origin
//     header is allowed").
//
// If host cannot be split into host:port, the loopback IP-port
// patterns are omitted but the Wails patterns are kept — the daemon
// always binds the relay to 127.0.0.1 anyway, so the GUI must still
// reach it across the wails:// → 127.0.0.1 origin boundary.
//
// CONTEXT D-09: schemes included because the relay may be fronted by
// TLS in a future deployment; both "localhost" and "127.0.0.1" included
// because browsers and tools emit either form.
func loopbackOriginPatterns(host string) []string {
	// Production startURL by platform (Wails v2.12.0):
	//   macOS / Linux: wails://wails/         → Origin: wails://wails
	//   Windows:       http://wails.localhost → Origin: http://wails.localhost
	// Dev mode appends ".localhost" + the Vite HMR port:
	//   macOS / Linux: wails://wails.localhost:<port>
	//   Windows:       http://wails.localhost:<port>
	// All four combinations must be permitted; only `wails.localhost` and
	// `wails` are reserved Wails-runtime hosts that an external browser
	// cannot impersonate.
	wails := []string{
		"wails://wails",             // production macOS / Linux
		"wails://wails.localhost:*", // dev macOS / Linux
		"http://wails.localhost",    // production Windows
		"http://wails.localhost:*",  // dev Windows
	}
	_, port, err := net.SplitHostPort(host)
	if err != nil || port == "" {
		return wails
	}
	return append(wails,
		"http://localhost:"+port,
		"http://127.0.0.1:"+port,
		"https://localhost:"+port,
		"https://127.0.0.1:"+port,
	)
}

// LoopbackOriginPatterns is the exported wrapper around loopbackOriginPatterns,
// letting the daemon's remote-WS proxy (internal/daemon/remote_ws_proxy.go) reuse
// the exact same inbound-Origin allowlist that handleSession uses for the local
// relay WebSocket. It delegates to the unexported helper so the pattern slice is
// defined in exactly one place (T-134-06-01 spoofing mitigation: the proxy's
// inbound websocket.Accept must apply the same loopback/Wails allowlist).
func LoopbackOriginPatterns(host string) []string { return loopbackOriginPatterns(host) }

// NotifyViewerCount pushes a MsgMeta frame with the current viewer count
// to all subscribers. Must be called AFTER Subscribe/Unsubscribe returns
// (outside hub.mu) to avoid deadlock.
func NotifyViewerCount(hub *Hub) {
	count := hub.SubscriberCount()
	frame := MakeMeta(MetaPayload{ViewerCount: &count})
	hub.BroadcastMeta(frame)
}

// handleListSessions returns a JSON array of currently registered session IDs.
func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	ids := s.manager.SessionIDs()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(ids); err != nil {
		http.Error(w, "failed to encode sessions", http.StatusInternalServerError)
	}
}
