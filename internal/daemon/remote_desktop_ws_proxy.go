package daemon

// Phase 21-06 — loopback WebSocket proxy for a peer's desktop signalling.
//
// The Wails web view can only reach 127.0.0.1, never the peer, so the daemon
// proxies GET /api/relay/remote-desktop/{host}/ws to the peer's
// wss://<peer>:7444/desktop/ws?cap=<token>, the same pattern handleRemoteSessionWS
// uses for terminal tabs. The token comes from DesktopCapStore and never reaches
// the web view's URL, a close reason, a response body or a log line. Only
// signalling crosses this proxy: media goes UDP straight from the peer to the
// web view.
//
// Both legs keep coder/websocket's 32768-byte default read limit: Phase 19 offers
// are about 5 KB and Phase 20 sizes clipboard frames to fit it.

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/scottkw/portty/internal/relay"
)

// handleRemoteDesktopWS proxies an inbound web view WebSocket to the peer's
// desktop signalling socket. It is a dumb pipe: the peer enforces the token,
// the same-user check and the always-available switch.
func (a *API) handleRemoteDesktopWS(w http.ResponseWriter, r *http.Request) {
	hostKey := desktopCapKey(r.PathValue("host"))
	if hostKey == "" {
		http.Error(w, "missing host", http.StatusBadRequest)
		return
	}
	// Lookup BEFORE the upgrade so a miss is the JSON 404 the terminal proxy uses.
	baseURL, ip, token, ok := a.desktopCaps.Get(hostKey)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no desktop token registered for host"})
		return
	}

	clientConn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: relay.LoopbackOriginPatterns(r.Host),
	})
	if err != nil {
		return // Accept already wrote the HTTP error response
	}
	defer clientConn.CloseNow()

	u, perr := url.Parse(baseURL)
	if perr != nil {
		clientConn.Close(websocket.StatusInternalError, "bad base url")
		return
	}
	u.Scheme = "wss"
	u.Path = "/desktop/ws"
	u.RawQuery = url.Values{"cap": {token}}.Encode()

	// viewer-diag lines log only the upstream hostname, never the URL or its query.
	host := u.Hostname()
	redact := func(err error) string {
		return redactCapTokenFromError(errors.New(strings.ReplaceAll(err.Error(), u.RawQuery, "<redacted>")), token)
	}
	dialStart := time.Now()
	dialCtx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	hdr := http.Header{}
	hdr.Set("Origin", strings.TrimRight(baseURL, "/")) // byte-exact match to the peer's BaseURL()
	upstream, _, derr := websocket.Dial(dialCtx, u.String(), &websocket.DialOptions{
		HTTPClient: a.peerClient(host, ip),
		HTTPHeader: hdr,
	})
	if derr != nil {
		log.Printf("viewer-diag: desktop proxy dial failed host=%s err=%s", host, redact(derr))
		clientConn.Close(websocket.StatusTryAgainLater, "remote unreachable")
		return
	}
	defer upstream.CloseNow()
	log.Printf("viewer-diag: desktop proxy open host=%s dial=%s", host, time.Since(dialStart))
	opened := time.Now()

	// The copy loops run on the REQUEST context, not the 10 s dial context.
	ctx := r.Context()
	errc := make(chan copyEnd, 2)
	go copyWS(ctx, upstream, clientConn, "client", errc)
	go copyWS(ctx, clientConn, upstream, "peer", errc)
	every, timeout := relay.PingEvery, relay.PingTimeout
	if a.heartbeatEvery > 0 {
		every, timeout = a.heartbeatEvery, a.heartbeatTimeout
	}
	go func() {
		if relay.Heartbeat(ctx, upstream, every, timeout) {
			log.Printf("viewer-diag: desktop proxy heartbeat timeout host=%s", host)
		}
	}()
	end := <-errc
	if end.side == "client" && end.write {
		// The peer may have just closed: wait briefly for its close frame.
		select {
		case peerEnd := <-errc:
			if websocket.CloseStatus(peerEnd.err) != -1 {
				end = peerEnd
			}
		case <-time.After(peerCloseGrace):
		}
	}
	var ce websocket.CloseError
	if errors.As(end.err, &ce) {
		log.Printf("viewer-diag: desktop proxy end host=%s first=%s code=%d reason=%q after=%s", host, end.side, ce.Code, ce.Reason, time.Since(opened))
		_ = clientConn.Close(ce.Code, ce.Reason)
		return
	}
	log.Printf("viewer-diag: desktop proxy end host=%s first=%s code=-1 after=%s err=%s", host, end.side, time.Since(opened), redact(end.err))
}
