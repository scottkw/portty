package daemon

// Phase 134-06 — Cap-gated remote terminal WebSocket reverse proxy.
//
// Phase 134's interactive modal reuses TerminalPanel/RelayClient, which for a
// LOCAL session attaches the local relay at ws://127.0.0.1:<relayPort>/sessions/
// {id}/ws. For a REMOTE session that session id only exists on the remote peer,
// so the local attach 404s (134-REVIEW CR-01/CR-02). The webview can only reach
// 127.0.0.1:<relayPort> — never the peer — so the daemon must proxy.
//
// This file adds GET /api/relay/remote/{sessionID}/ws on the relay loopback
// surface. It looks up the cap /peers/connect stored in the RemoteCapStore,
// dials the peer's already-cap-gated wss://<baseURL>/sessions/{sid}/ws?cap=T
// through the same verified peer client /peers/connect used (peerClient: the
// stored Tailscale IP, certificate checked against the baseURL host), and
// copies opaque WS frames both ways. The cap stays
// server-side: the webview's URL carries NO cap (T-134-06-03 info-disclosure).
//
// Critical correctness gotchas (134-RESEARCH Pitfalls 1 & 4):
//  1. A Go WS dialer sends no Origin header; the peer's requireAllowedOrigin
//     rejects an empty Origin with 403. We inject Origin: <baseURL> byte-exact.
//  2. The bidirectional copy loop MUST use the long-lived request context, NOT
//     the 10s dial-timeout context — otherwise a healthy terminal dies after 10s.
//
// Phase 5 (SHAR-07): the proxy pings the upstream every relay.PingEvery and, when
// it stops answering, closes both sides with no close frame, so the GUI tab or
// `portty attach` sees an abnormal close and reconnects.

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/scottkw/portty/internal/relay"
)

// handleRemoteSessionWS proxies an inbound webview WebSocket to a remote peer's
// cap-gated terminal WS. It is a dumb pipe: the peer enforces HMAC cap
// verification, SID-match, grant-active, and read/write perms — this handler
// only forwards the cap (server-side) and copies frames.
func (a *API) handleRemoteSessionWS(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("sessionID")
	if sid == "" {
		http.Error(w, "missing sessionID", http.StatusBadRequest)
		return
	}
	if a.remoteCaps == nil {
		http.Error(w, "remote cap store not initialised", http.StatusInternalServerError)
		return
	}

	// Cap lookup BEFORE the upgrade so the no-cap path returns the same JSON 404
	// contract as proxyRemoteFiles (the frontend treats this marker as "no cap
	// held"). A bare route-miss would be Go's "404 page not found".
	baseURL, ip, capToken, ok := a.remoteCaps.Get(sid)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no cap registered for session"})
		return
	}

	// 1. Accept the inbound (webview) WS. Reuse the relay's loopback/Wails Origin
	//    allowlist (T-134-06-01): the inbound conn is still loopback-only, so the
	//    same allowlist handleSession uses applies here. WS upgrades are not
	//    CORS-preflighted, so no cross-origin CORS wrapper is involved (Pattern 3).
	// The client side keeps coder/websocket's 32 KiB read limit: it only carries
	// viewer input, which the host's share server limits to 32 KiB anyway.
	clientConn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: relay.LoopbackOriginPatterns(r.Host),
	})
	if err != nil {
		// websocket.Accept already wrote the HTTP error response.
		return
	}
	defer clientConn.CloseNow()

	// 2. Build the upstream wss:// URL. RemoteCapStore.Put enforces an https://
	//    baseURL with a non-empty host, so swapping the scheme to wss is safe.
	u, perr := url.Parse(baseURL)
	if perr != nil {
		clientConn.Close(websocket.StatusInternalError, "bad base url")
		return
	}
	u.Scheme = "wss"
	u.Path = "/sessions/" + sid + "/ws"
	// SHAR-07: forward the viewer's resume offset only when it parses as a
	// decimal uint64, re-encoded from the number so only digits reach the peer.
	q := url.Values{"cap": {capToken}}
	if resume, ok := relay.ParseResume(r.URL.Query().Get("resume")); ok {
		q.Set("resume", strconv.FormatUint(resume, 10))
	}
	u.RawQuery = q.Encode()

	// 3. Dial the peer with a BOUNDED dial context (10s) but the long-lived
	//    request context as its parent. CRITICAL (Pitfall 1): inject
	//    Origin: <baseURL> — the peer's requireAllowedOrigin rejects an empty
	//    Origin, which a Go WS dialer would otherwise send. Use the verified
	//    peer client (stored IP, certificate checked against the base URL host).
	// viewer-diag lines log only the upstream hostname, never the URL or its
	// query. redact strips the whole query first (the dial error quotes the URL),
	// then the raw token.
	id8, host := relay.DiagID(sid), u.Hostname()
	redact := func(err error) string {
		return redactCapTokenFromError(errors.New(strings.ReplaceAll(err.Error(), u.RawQuery, "<redacted>")), capToken)
	}
	dialStart := time.Now()
	dialCtx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	hdr := http.Header{}
	hdr.Set("Origin", strings.TrimRight(baseURL, "/")) // byte-exact match to the peer's BaseURL()
	upstream, _, derr := websocket.Dial(dialCtx, u.String(), &websocket.DialOptions{
		HTTPClient: a.peerClient(u.Hostname(), ip),
		HTTPHeader: hdr,
	})
	if derr != nil {
		// Never surface the cap token in a close reason or log (T-134-06-03).
		// The reason string is a fixed literal; the log line carries the redacted error.
		log.Printf("viewer-diag: proxy dial failed session=%s host=%s err=%s", id8, host, redact(derr))
		clientConn.Close(websocket.StatusTryAgainLater, "remote unreachable")
		return
	}
	defer upstream.CloseNow()
	// A live 32 KiB PTY frame is over the 32 KiB default (UAT gap 4).
	upstream.SetReadLimit(relay.ViewerReadLimit)
	// A slow first dial points at a cold or DERP-relayed tailnet path (04-07 H-net).
	log.Printf("viewer-diag: proxy open session=%s host=%s dial=%s", id8, host, time.Since(dialStart))
	opened := time.Now()

	// 4. Bidirectional opaque copy on the REQUEST context (Pitfall 4 — NOT the
	//    10s dialCtx, which would kill a healthy long-lived terminal). The first
	//    side to error/close tears down both via the deferred CloseNow calls.
	ctx := r.Context()
	errc := make(chan copyEnd, 2)
	go copyWS(ctx, upstream, clientConn, "client", errc) // webview → peer (input/resize/ping)
	go copyWS(ctx, clientConn, upstream, "peer", errc)   // peer → webview (scrollback/output)
	// SHAR-07: drop a silent upstream (half-open after sleep or a network
	// change). The peer-side copyWS above is the reader Heartbeat needs. The
	// loopback clientConn is not pinged: a dead local client closes its socket.
	every, timeout := relay.PingEvery, relay.PingTimeout
	if a.heartbeatEvery > 0 {
		every, timeout = a.heartbeatEvery, a.heartbeatTimeout
	}
	go func() {
		if relay.Heartbeat(ctx, upstream, every, timeout) {
			log.Printf("viewer-diag: proxy heartbeat timeout session=%s host=%s", id8, host)
		}
	}()
	end := <-errc
	err = end.err
	code := int(websocket.CloseStatus(err))
	if code == -1 {
		log.Printf("viewer-diag: proxy end session=%s first=%s code=-1 reason=\"\" after=%s err=%s", id8, end.side, time.Since(opened), redact(err))
	} else {
		var ce websocket.CloseError
		errors.As(err, &ce)
		log.Printf("viewer-diag: proxy end session=%s first=%s code=%d reason=%q after=%s", id8, end.side, code, ce.Reason, time.Since(opened))
	}
	// Forward a close code and reason (e.g. 1000 "share stopped") so the viewer
	// can say why it ended. Reasons come from the peer's fixed constants and
	// never carry the cap.
	var ce websocket.CloseError
	if errors.As(err, &ce) {
		_ = clientConn.Close(ce.Code, ce.Reason)
	}
}

// copyEnd is how one copy direction ended; side names the conn it read from
// ("peer" or "client") for the viewer-diag proxy end line.
type copyEnd struct {
	side string
	err  error
}

// copyWS reads whole messages from src and writes them verbatim to dst until
// either side errors. The PTY frame protocol is end-to-end between xterm and the
// peer hub, so the proxy is transport-only: it never parses or evaluates frames.
func copyWS(ctx context.Context, dst, src *websocket.Conn, side string, errc chan<- copyEnd) {
	for {
		typ, data, err := src.Read(ctx)
		if err != nil {
			errc <- copyEnd{side, err}
			return
		}
		if err := dst.Write(ctx, typ, data); err != nil {
			errc <- copyEnd{side, err}
			return
		}
	}
}
