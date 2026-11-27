package webserver

import (
	"context"
	"net/http"

	"github.com/coder/websocket"

	"github.com/scottkw/portty/internal/capability"
)

// DesktopGuestBackend is what the /desktop/guest/ routes need from the owner of
// the guest share. It is an optional interface on the DesktopBackend installed
// with SetDesktopBackend (the Full() precedent in handleDesktopWS), so
// DesktopBackend and every fake of it stay as they are; a nil backend or one
// without these methods means no share is active.
//
// GuestShareID returns the running share's ID and whether a share is active.
// ServeGuestViewer runs one guest's signalling session; who is the identity
// WhoIs gave for that connection, and admit reports whether the link still
// opens (same share, same signing key), which the backend calls again just
// before the guest joins.
type DesktopGuestBackend interface {
	GuestShareID() (shareID string, active bool)
	ServeGuestViewer(ctx context.Context, c *websocket.Conn, admit func() bool, who PeerIdentity)
}

// guestTokenOutcome decides whether r carries a guest token that opens the
// link now. Verification is under DesktopGuestKey, so no other family's token
// passes; the share ID must be the active share's, so a stopped share's link,
// an older share's link and a pre-regenerate link all fail.
func (ws *WebServer) guestTokenOutcome(r *http.Request) desktopOutcome {
	return ws.guestTokenOutcomeFor(r.URL.Query().Get("cap"))
}

func (ws *WebServer) guestTokenOutcomeFor(token string) desktopOutcome {
	key := ws.currentSigningKey()
	if key == nil || token == "" {
		return desktopInvalid
	}
	claims, err := capability.VerifyDesktopGuest(token, key)
	if err != nil {
		return desktopInvalid
	}
	g, ok := ws.desktop.(DesktopGuestBackend)
	if !ok {
		return desktopOff
	}
	id, active := g.GuestShareID()
	if !active {
		return desktopOff
	}
	if claims.ShareID != id {
		return desktopInvalid
	}
	return desktopOK
}

// setupDesktopGuestRoutes registers the /desktop/guest/ family, the only
// routes behind requireTailnetPeer: every other route keeps requireSameUser.
func (ws *WebServer) setupDesktopGuestRoutes(mux *http.ServeMux) {
	// Origin, then identity, then token, all before Accept: a refused request
	// never reaches a 101.
	mux.HandleFunc("GET /desktop/guest/ws",
		ws.requireAllowedOrigin(ws.requireTailnetPeer(ws.handleDesktopGuestWS)))
}

// handleDesktopGuestWS checks the token and the viewer cap, then upgrades and
// hands the socket and the caller's identity to the backend. Origin and
// identity were checked by the wrappers.
func (ws *WebServer) handleDesktopGuestWS(w http.ResponseWriter, r *http.Request) {
	who, ok := peerIdentityFrom(r.Context())
	if !ok {
		refuseOtherUser(w) // reached without the gate: fail closed
		return
	}
	if o := ws.guestTokenOutcome(r); o != desktopOK {
		writeDesktopRefusal(w, o, false)
		return
	}
	g := ws.desktop.(DesktopGuestBackend) // guestTokenOutcome proved it
	if f, ok := g.(interface{ Full() bool }); ok && f.Full() {
		http.Error(w, "too many viewers", http.StatusTooManyRequests)
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: ws.allowedOrigins(),
	})
	if err != nil {
		return
	}
	token := r.URL.Query().Get("cap")
	admit := func() bool { return ws.guestTokenOutcomeFor(token) == desktopOK }
	g.ServeGuestViewer(r.Context(), conn, admit, who)
}
