package webserver

import (
	"context"
	"io/fs"
	"net/http"
	"strings"

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

// desktopGuestLinkInvalidPage is the embedded web/desktop-guest-link-invalid.html.
var desktopGuestLinkInvalidPage = mustWebFile("desktop-guest-link-invalid.html")

// writeGuestPageRefusal answers a refused guest page request with the static
// "link no longer works" page: 401 for a bad token, 410 when no share is active,
// the reason header set, no X-Portty marker (that belongs to identity refusals).
// Nothing from the request appears in it.
func writeGuestPageRefusal(w http.ResponseWriter, o desktopOutcome) {
	reason, status := "token", http.StatusUnauthorized
	if o == desktopOff {
		reason, status = "off", http.StatusGone
	}
	h := w.Header()
	h.Set(desktopReasonHeader, reason)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(desktopGuestLinkInvalidPage)
}

// setupDesktopGuestRoutes registers the /desktop/guest/ family: the SPA page and
// its assets, the signalling WebSocket and the check endpoint. These are the
// only routes behind requireTailnetPeer(Page); every other route keeps
// requireSameUser(Page).
func (ws *WebServer) setupDesktopGuestRoutes(mux *http.ServeMux) {
	// Page and assets: behind the tailnet gate, with no token needed for the
	// hashed assets (a guest's browser fetches them without ?cap=), so a person
	// the gate refuses never receives the bundle (RESEARCH Finding 5).
	mux.HandleFunc("GET /desktop/guest/", ws.cspHeaders(ws.requireTailnetPeerPage(ws.handleDesktopGuestPage)))

	// A fixed same-origin Location with the query kept byte for byte, as for
	// GET /desktop: ServeMux's own subtree redirect would answer 307.
	mux.HandleFunc("GET /desktop/guest", func(w http.ResponseWriter, r *http.Request) {
		loc := "/desktop/guest/"
		if r.URL.RawQuery != "" {
			loc += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, loc, http.StatusMovedPermanently)
	})

	// Origin, then identity, then token, all before Accept: a refused request
	// never reaches a 101.
	mux.HandleFunc("GET /desktop/guest/ws",
		ws.requireAllowedOrigin(ws.requireTailnetPeer(ws.handleDesktopGuestWS)))

	// A browser does not expose a failed handshake's status, so a guest whose
	// socket failed asks here which refusal it was.
	mux.HandleFunc("GET /desktop/guest/check", ws.requireTailnetPeer(func(w http.ResponseWriter, r *http.Request) {
		if o := ws.guestTokenOutcome(r); o != desktopOK {
			writeDesktopRefusal(w, o, false)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNoContent)
	}))
}

// handleDesktopGuestPage serves the SPA bundle under /desktop/guest/. Only the
// entry document needs the guest token; the identity gate ran in the wrapper.
func (ws *WebServer) handleDesktopGuestPage(w http.ResponseWriter, r *http.Request) {
	// cspHeaders sets this too; the link carries a bearer token, so it is
	// stated here where the token is read.
	w.Header().Set("Referrer-Policy", "no-referrer")
	appFS := ws.staticAppFS
	if appFS == nil {
		http.Error(w, "app bundle not configured", http.StatusServiceUnavailable)
		return
	}
	index := func() {
		if o := ws.guestTokenOutcome(r); o != desktopOK {
			writeGuestPageRefusal(w, o)
			return
		}
		serveIndex(w, r, appFS)
	}
	rel := strings.TrimPrefix(r.URL.Path, "/desktop/guest/")
	if rel == "" || strings.HasSuffix(rel, "/") {
		index()
		return
	}
	if _, err := fs.Stat(appFS, rel); err != nil {
		index()
		return
	}
	// Same caching contract as /desktop/: cspHeaders sets no-store, hashed assets
	// drop it, everything else revalidates.
	if strings.HasPrefix(rel, "assets/") {
		w.Header().Del("Cache-Control")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	http.StripPrefix("/desktop/guest/", http.FileServerFS(appFS)).ServeHTTP(w, r)
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
