package webserver

import (
	"net/http"

	"github.com/scottkw/portty/internal/capability"
	webfs "github.com/scottkw/portty/web"
)

// requireCapability returns a middleware that gates access to a handler on a
// valid HMAC-signed capability token supplied via the ?cap= query parameter.
//
// Enforcement sequence (RESEARCH Pattern 3):
//  1. If no ?cap= is present, respond 401 "capability required".
//  2. If the server has no signing key loaded (SetSigningKey not yet called),
//     respond 401. This defends against a misconfigured daemon that never
//     bootstrapped FileKeyStore — we refuse to authenticate rather than
//     accept any token under a nil key.
//  3. Verify the token. Any failure (malformed, bad sig, bad claims) maps to
//     a single generic 401 body — do NOT distinguish between error classes to
//     the caller (information-disclosure defense, T-87-08).
//  4. If the route carries a path parameter "id", reject with 403 "capability
//     does not match session" when claims.SID != pathID (SEC-03).
//  5. Check that claims.GrantID is still in the session's grant list; if not,
//     respond 403 "capability has been revoked" (SEC-04 via D-15).
//  6. Cross-check that the session is still web-enabled. Toggle-off clears
//     grants AND disables web-serving, so either path blocks; this is a
//     defense-in-depth redundancy (in case grants were cleared without
//     disabling, or vice versa).
//  7. On success, attach the verified Claims to the request context so
//     downstream handlers can read Perms / SID / GrantID without re-parsing.
//
// Ordering (RESEARCH Pitfall 5): this middleware MUST wrap mux.HandleFunc
// registrations so it runs BEFORE the handler body — particularly before
// websocket.Accept in handleWSSRelay. Placing the check after Accept is
// ineffective because the 101 Switching Protocols response has already been
// committed by then.
func (ws *WebServer) requireCapability(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, status, msg := ws.verifyCapability(r)
		if status != 0 {
			http.Error(w, msg, status)
			return
		}
		next(w, r.WithContext(capability.WithClaims(r.Context(), claims)))
	}
}

// requireCapabilityPage is requireCapability for the browser-facing link
// route: on failure it writes the same status with the static "Link No
// Longer Works" page. Nothing from the request is rendered into it.
func (ws *WebServer) requireCapabilityPage(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, status, _ := ws.verifyCapability(r)
		if status != 0 {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.WriteHeader(status)
			_, _ = w.Write(linkInvalidPage)
			return
		}
		next(w, r.WithContext(capability.WithClaims(r.Context(), claims)))
	}
}

// linkInvalidPage is the embedded web/link-invalid.html.
var linkInvalidPage = func() []byte {
	b, err := webfs.WebFS.ReadFile("link-invalid.html")
	if err != nil {
		panic("webserver: embedded link-invalid.html: " + err.Error())
	}
	return b
}()

// verifyCapability runs the enforcement sequence above. On failure it returns
// a non-zero status and the plain-text body requireCapability writes.
func (ws *WebServer) verifyCapability(r *http.Request) (capability.Claims, int, string) {
	return ws.verifyCapabilityToken(r.URL.Query().Get("cap"), r.PathValue("id"))
}

// verifyCapabilityToken is verifyCapability's sequence for a token the caller
// already read: the ?cap= query for the page routes, the X-Portty-Cap header
// for the Phase 13 port routes. pathID is the {id} path value ("" skips the
// session match).
func (ws *WebServer) verifyCapabilityToken(token, pathID string) (capability.Claims, int, string) {
	if token == "" {
		return capability.Claims{}, http.StatusUnauthorized, "capability required"
	}
	key := ws.currentSigningKey()
	if key == nil {
		return capability.Claims{}, http.StatusUnauthorized, "capability required"
	}
	claims, err := capability.Verify(token, key)
	if err != nil {
		// Collapse all verify failures to a single 401 body — do not leak
		// whether the token was malformed, had a bad signature, or had
		// unparseable claims (T-87-08).
		return capability.Claims{}, http.StatusUnauthorized, "capability required"
	}
	if pathID != "" && claims.SID != pathID {
		return capability.Claims{}, http.StatusForbidden, "capability does not match session"
	}
	if !ws.isGrantActive(claims.SID, claims.GrantID) {
		return capability.Claims{}, http.StatusForbidden, "capability has been revoked"
	}
	if !ws.IsSessionEnabled(claims.SID) {
		// Defense in depth: toggle-off clears grants AND disables web
		// serving. Either alone would block this request; checking both
		// guards against a future code path that touches only one.
		return capability.Claims{}, http.StatusForbidden, "capability has been revoked"
	}
	return claims, 0, ""
}
