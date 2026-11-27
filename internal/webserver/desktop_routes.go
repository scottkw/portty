package webserver

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"

	"github.com/scottkw/portty/internal/capability"
)

// DesktopBackend is what the /desktop/ routes need from the desktop Manager's
// owner. DesktopGrant returns the current always-available grant ID and whether
// the link is on; ServeDesktopViewer runs one viewer's signalling session.
// admit reports whether the viewer's token still verifies under the current
// signing key; the backend calls it again just before the viewer joins, so a
// key regenerated during the handshake cuts that viewer off too (CR-01).
type DesktopBackend interface {
	DesktopGrant() (grantID string, on bool)
	ServeDesktopViewer(ctx context.Context, c *websocket.Conn, admit func() bool)
}

// SetDesktopBackend installs the desktop backend. Set once before Start; not
// mutex-protected, like SetShareTokenProvider. A nil backend means desktop
// links are off.
func (ws *WebServer) SetDesktopBackend(b DesktopBackend) {
	ws.desktop = b
}

type desktopOutcome int

const (
	desktopOK      desktopOutcome = iota // a valid token for the current grant
	desktopInvalid                       // missing, malformed, wrong key, wrong kind or stale grant
	desktopOff                           // the link is switched off
)

// desktopTokenOutcome decides whether r carries a desktop token that opens the
// link now. Verification is under DesktopKey, so a session capability or a port
// token never passes, whatever its claims say (DSHR-08).
func (ws *WebServer) desktopTokenOutcome(r *http.Request) desktopOutcome {
	key := ws.currentSigningKey()
	token := r.URL.Query().Get("cap")
	if key == nil || token == "" {
		return desktopInvalid
	}
	claims, err := capability.VerifyDesktop(token, key)
	if err != nil {
		return desktopInvalid
	}
	if ws.desktop == nil {
		return desktopOff
	}
	grant, on := ws.desktop.DesktopGrant()
	if !on {
		return desktopOff
	}
	if claims.GrantID != grant {
		return desktopInvalid
	}
	return desktopOK
}

// desktopReasonHeader carries the fixed refusal code on every refused /desktop/
// request, so a viewer or a peer proxy branches on a value and never on text
// (token: rejected link, 401; off: sharing switched off, 410). Neither status
// is 403, so the user-refusal answer alone carries the X-Portty marker.
const desktopReasonHeader = "X-Portty-Desktop"

// The static first-load pages (web/desktop-off.html, web/desktop-link-invalid.html).
var (
	desktopOffPage         = mustWebFile("desktop-off.html")
	desktopLinkInvalidPage = mustWebFile("desktop-link-invalid.html")
)

// writeDesktopRefusal answers a refused /desktop/ request: the reason header,
// 401 for a rejected token or 410 for desktop off, and the matching static page
// (page routes) or a {"reason":...} JSON body (API routes). Nothing from the
// request appears in either.
func writeDesktopRefusal(w http.ResponseWriter, o desktopOutcome, page bool) {
	reason, status, html := "token", http.StatusUnauthorized, desktopLinkInvalidPage
	if o == desktopOff {
		reason, status, html = "off", http.StatusGone, desktopOffPage
	}
	h := w.Header()
	h.Set(desktopReasonHeader, reason)
	h.Set("X-Content-Type-Options", "nosniff")
	body := []byte(`{"reason":"` + reason + `"}`)
	if page {
		h.Set("Content-Type", "text/html; charset=utf-8")
		body = html
	} else {
		h.Set("Content-Type", "application/json")
		h.Set("Cache-Control", "no-store")
	}
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// setupDesktopRoutes registers the /desktop/ route family: the SPA bundle, the
// signalling WebSocket and the check endpoint. GET /desktop (no slash) redirects
// to /desktop/ with the query kept.
func (ws *WebServer) setupDesktopRoutes(mux *http.ServeMux) {
	// The bundle sits behind cspHeaders and the same-user page gate, exactly as
	// /sessions/{id} does, before any token work. Only the entry document needs
	// the token: the hashed assets are fetched without ?cap=.
	mux.HandleFunc("GET /desktop/", ws.cspHeaders(ws.requireSameUserPage(func(w http.ResponseWriter, r *http.Request) {
		appFS := ws.staticAppFS
		if appFS == nil {
			http.Error(w, "app bundle not configured", http.StatusServiceUnavailable)
			return
		}
		index := func() {
			if o := ws.desktopTokenOutcome(r); o != desktopOK {
				writeDesktopRefusal(w, o, true)
				return
			}
			serveIndex(w, r, appFS)
		}
		rel := strings.TrimPrefix(r.URL.Path, "/desktop/")
		if rel == "" || strings.HasSuffix(rel, "/") {
			index()
			return
		}
		if _, err := fs.Stat(appFS, rel); err != nil {
			index()
			return
		}
		// Same caching contract as /app/: cspHeaders sets no-store, hashed
		// assets drop it, everything else revalidates.
		if strings.HasPrefix(rel, "assets/") {
			w.Header().Del("Cache-Control")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		http.StripPrefix("/desktop/", http.FileServerFS(appFS)).ServeHTTP(w, r)
	})))

	// ServeMux's own subtree redirect answers 307 here; the contract is 301 with
	// the query kept byte for byte. A fixed same-origin Location, so no open
	// redirect, and it carries nothing the request did not already hold.
	mux.HandleFunc("GET /desktop", func(w http.ResponseWriter, r *http.Request) {
		loc := "/desktop/"
		if r.URL.RawQuery != "" {
			loc += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, loc, http.StatusMovedPermanently)
	})

	// Origin, then account, then token, all before Accept: a refused request
	// never reaches a 101.
	mux.HandleFunc("GET /desktop/ws",
		ws.requireAllowedOrigin(ws.requireSameUser(ws.handleDesktopWS)))

	// Browsers do not expose a failed handshake's status, so a viewer whose
	// socket failed asks here which refusal it was. Same-user first, as on
	// every /desktop/ route.
	mux.HandleFunc("GET /desktop/check", ws.requireSameUser(func(w http.ResponseWriter, r *http.Request) {
		if o := ws.desktopTokenOutcome(r); o != desktopOK {
			writeDesktopRefusal(w, o, false)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNoContent)
	}))
}

// LookupPeer asks tailscaled who owns addr (host:port), for the media binding of
// a guest's selected ICE pair (desktop.Deps.PeerIdentity). It returns the numeric
// user ID and the stable node ID, and passes an error through unchanged: the
// caller fails closed on any error.
func (ws *WebServer) LookupPeer(ctx context.Context, addr string) (userID int64, stableID string, err error) {
	who, err := ws.whois.WhoIs(ctx, addr)
	if err != nil {
		return 0, "", err
	}
	if who == nil || who.Node == nil {
		return 0, "", errors.New("webserver: whois answered no node")
	}
	return int64(who.Node.User), string(who.Node.StableID), nil
}

// handleDesktopWS checks the token and the viewer cap, then upgrades and hands
// the socket to the backend. Origin and account were checked by the wrappers.
func (ws *WebServer) handleDesktopWS(w http.ResponseWriter, r *http.Request) {
	if o := ws.desktopTokenOutcome(r); o != desktopOK {
		writeDesktopRefusal(w, o, false)
		return
	}
	// The backend (19-03) reports a full viewer set; refuse before the upgrade.
	if f, ok := ws.desktop.(interface{ Full() bool }); ok && f.Full() {
		http.Error(w, "too many viewers", http.StatusTooManyRequests)
		return
	}
	// A backend that can name its own-account viewers is told who this one is.
	// The same-user gate already passed, so the lookup never refuses: a failure
	// gives an empty identity and the viewer still joins.
	as, namesViewers := ws.desktop.(interface {
		ServeDesktopViewerAs(ctx context.Context, c *websocket.Conn, admit func() bool, who PeerIdentity)
	})
	var who PeerIdentity
	if namesViewers {
		who = ws.lookupPeerLenient(r)
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: ws.allowedOrigins(),
	})
	if err != nil {
		return
	}
	// Only the key is re-checked: it takes a leaf lock, safe under the Manager's
	// locks. A grant change (off, then on) closes the old Manager anyway.
	token := r.URL.Query().Get("cap")
	admit := func() bool {
		_, err := capability.VerifyDesktop(token, ws.currentSigningKey())
		return err == nil
	}
	if namesViewers {
		as.ServeDesktopViewerAs(r.Context(), conn, admit, who)
		return
	}
	ws.desktop.ServeDesktopViewer(r.Context(), conn, admit)
}

// lookupPeerLenient is WhoIs on the request's connection as a PeerIdentity, for
// naming an own-account viewer in the audit log. Unlike the guest gate it
// accepts whatever tailscaled answers and returns the zero identity on any error.
func (ws *WebServer) lookupPeerLenient(r *http.Request) PeerIdentity {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	who, err := ws.whois.WhoIs(ctx, r.RemoteAddr)
	if err != nil || who == nil || who.Node == nil {
		return PeerIdentity{}
	}
	id := PeerIdentity{
		UserID:   int64(who.Node.User),
		NodeName: who.Node.ComputedName,
		StableID: string(who.Node.StableID),
	}
	if who.UserProfile != nil {
		id.LoginName, id.DisplayName = who.UserProfile.LoginName, who.UserProfile.DisplayName
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		id.IP = host
	} else {
		id.IP = r.RemoteAddr
	}
	return id
}
