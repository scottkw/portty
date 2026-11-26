package webserver

import (
	"context"
	"io/fs"
	"net/http"
	"strings"

	"github.com/coder/websocket"

	"github.com/scottkw/portty/internal/capability"
)

// DesktopBackend is what the /desktop/ routes need from the desktop Manager's
// owner. DesktopGrant returns the current always-available grant ID and whether
// the link is on; ServeDesktopViewer runs one viewer's signalling session.
type DesktopBackend interface {
	DesktopGrant() (grantID string, on bool)
	ServeDesktopViewer(ctx context.Context, c *websocket.Conn)
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

// setupDesktopRoutes registers the /desktop/ route family: the SPA bundle and
// the signalling WebSocket. GET /desktop (no slash) is left to ServeMux, which
// redirects it to /desktop/ with the query kept.
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
			// shortcut: one plain refusal for every outcome; 19-04 replaces it
			// with the static off/invalid pages and distinct codes.
			if ws.desktopTokenOutcome(r) != desktopOK {
				http.Error(w, "desktop link refused", http.StatusUnauthorized)
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

	// Origin, then account, then token, all before Accept: a refused request
	// never reaches a 101.
	mux.HandleFunc("GET /desktop/ws",
		ws.requireAllowedOrigin(ws.requireSameUser(ws.handleDesktopWS)))
}

// handleDesktopWS checks the token, then upgrades and hands the socket to the
// backend. Origin and account were checked by the wrappers.
func (ws *WebServer) handleDesktopWS(w http.ResponseWriter, r *http.Request) {
	if ws.desktopTokenOutcome(r) != desktopOK {
		http.Error(w, "desktop link refused", http.StatusUnauthorized)
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: ws.allowedOrigins(),
	})
	if err != nil {
		return
	}
	ws.desktop.ServeDesktopViewer(r.Context(), conn)
}
