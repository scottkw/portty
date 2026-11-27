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
	"github.com/scottkw/portty/internal/tailnet"
)

// Close codes the proxy gives the web view when the peer refuses the handshake.
// Their reasons are fixed literals: a token never reaches a close frame.
const (
	closeDesktopToken     websocket.StatusCode = 4401 // token refused, also after one refresh
	closeDesktopOff       websocket.StatusCode = 4410 // the peer's always-available desktop is off
	closeDesktopForbidden websocket.StatusCode = 4403 // the peer runs Portty on another account

	// desktopReasonHeader is the header the peer's refusals carry (webserver).
	desktopReasonHeader = "X-Portty-Desktop"
)

var desktopCloseReasons = map[websocket.StatusCode]string{
	closeDesktopToken:             "token",
	closeDesktopOff:               "off",
	closeDesktopForbidden:         "forbidden",
	websocket.StatusTryAgainLater: "remote unreachable",
}

func closeDesktopRefused(c *websocket.Conn, code websocket.StatusCode) {
	_ = c.Close(code, desktopCloseReasons[code])
}

// refusalCode maps a refused upstream handshake to the web view's close code.
func refusalCode(status int, marker bool, reason string) websocket.StatusCode {
	switch {
	case status == http.StatusUnauthorized && reason == "token":
		return closeDesktopToken
	case status == http.StatusGone:
		return closeDesktopOff
	case status == http.StatusForbidden && marker:
		return closeDesktopForbidden
	}
	return websocket.StatusTryAgainLater
}

// refreshFailureStatus maps a failed token refresh the same way.
func refreshFailureStatus(err error) websocket.StatusCode {
	var f *peerDesktopFail
	if errors.As(err, &f) {
		switch f.body.Error {
		case PeerErrDesktopOff:
			return closeDesktopOff
		case PeerErrForbidden:
			return closeDesktopForbidden
		}
	}
	return websocket.StatusTryAgainLater
}

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
	defer func() { _ = clientConn.CloseNow() }()

	u, perr := url.Parse(baseURL)
	if perr != nil {
		_ = clientConn.Close(websocket.StatusInternalError, "bad base url")
		return
	}
	u.Scheme = "wss"
	u.Path = "/desktop/ws"
	host := u.Hostname()

	// viewer-diag lines log only the upstream hostname, never the URL or its
	// query. redact strips the whole query first (a dial error quotes the URL),
	// then the raw token.
	var rawQuery, curTok string
	redact := func(err error) string {
		return redactCapTokenFromError(errors.New(strings.ReplaceAll(err.Error(), rawQuery, "<redacted>")), curTok)
	}
	hdr := http.Header{}
	hdr.Set("Origin", strings.TrimRight(baseURL, "/")) // byte-exact match to the peer's BaseURL()
	dial := func(tok string) (*websocket.Conn, *http.Response, error) {
		curTok = tok
		u.RawQuery = url.Values{"cap": {tok}}.Encode()
		rawQuery = u.RawQuery
		// 10 s dial context under the long-lived request context.
		dialCtx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		return websocket.Dial(dialCtx, u.String(), &websocket.DialOptions{
			HTTPClient: a.peerClient(host, ip),
			HTTPHeader: hdr,
		})
	}
	dialStart := time.Now()
	upstream, resp, derr := dial(token)
	if derr != nil && resp != nil && resp.StatusCode == http.StatusUnauthorized && resp.Header.Get(desktopReasonHeader) == "token" {
		// A stale token heals once: fetch a fresh one and redial. A second refusal is final.
		if rerr := a.refreshPeerDesktopCap(r.Context(), hostKey); rerr != nil {
			log.Printf("viewer-diag: desktop proxy token refresh failed host=%s err=%s", host, rerr)
			closeDesktopRefused(clientConn, refreshFailureStatus(rerr))
			return
		}
		if _, _, fresh, ok := a.desktopCaps.Get(hostKey); ok {
			upstream, resp, derr = dial(fresh)
		}
	}
	if derr != nil {
		log.Printf("viewer-diag: desktop proxy dial failed host=%s err=%s", host, redact(derr))
		code := websocket.StatusTryAgainLater
		if resp != nil {
			code = refusalCode(resp.StatusCode, resp.Header.Get(tailnet.MarkerHeader) == tailnet.MarkerValue, resp.Header.Get(desktopReasonHeader))
		}
		closeDesktopRefused(clientConn, code)
		return
	}
	defer func() { _ = upstream.CloseNow() }()
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
