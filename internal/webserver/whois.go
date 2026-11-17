package webserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"time"

	"github.com/scottkw/portty/internal/tailnet"
	"tailscale.com/client/local"
	"tailscale.com/client/tailscale/apitype"
	"tailscale.com/ipn/ipnstate"
)

// whoIsClient is the part of tailscale.com/client/local.Client the same-user
// check needs. Production uses &ws.lc; tests install a fake.
type whoIsClient interface {
	WhoIs(ctx context.Context, remoteAddr string) (*apitype.WhoIsResponse, error)
	StatusWithoutPeers(ctx context.Context) (*ipnstate.Status, error)
}

// SetWhoIsClientForTest replaces the Tailscale WhoIs client. Test-only.
func (ws *WebServer) SetWhoIsClientForTest(c whoIsClient) { ws.whois = c }

// SetShareTokenProvider installs the lookup the peer capability route uses:
// it returns the share's link token, or false when the session is not shared.
// Set once before Start; not mutex-protected.
func (ws *WebServer) SetShareTokenProvider(fn func(sessionID string) (string, bool)) {
	ws.shareToken = fn
}

// sameUser reports whether the requesting node is logged in as the same
// Tailscale user as this Mac. It compares numeric user IDs (login names are
// display-only) and refuses tagged nodes, nodes shared in from another
// tailnet, a tagged or shared-in self, and any missing data.
func sameUser(who *apitype.WhoIsResponse, self *ipnstate.PeerStatus) bool {
	if who == nil || who.Node == nil || self == nil {
		return false
	}
	if who.Node.IsTagged() || who.Node.Sharer != 0 {
		return false
	}
	if self.Tags != nil && self.Tags.Len() > 0 {
		return false
	}
	if self.AltSharerUserID != 0 || self.UserID == 0 {
		return false
	}
	return who.Node.User == self.UserID
}

// refuseOtherUser writes the generic refusal. The X-Portty marker lets the
// viewing Mac tell "Portty, other account" from "not Portty"; the body never
// names an account.
func refuseOtherUser(w http.ResponseWriter) {
	w.Header().Set(tailnet.MarkerHeader, tailnet.MarkerValue)
	http.Error(w, "forbidden", http.StatusForbidden)
}

// refuseLocalFault refuses when this Mac's own tailscaled can't answer. It
// carries no marker, so the viewer reads it as unreachable, not as another
// account (WR-08).
func refuseLocalFault(w http.ResponseWriter) {
	http.Error(w, "unavailable", http.StatusServiceUnavailable)
}

// requireSameUser admits only callers whose tailnet node is logged in as this
// Mac's Tailscale user. Every error fails closed: 403 with the marker when the
// answer says another account (or no tailnet peer), 503 when this Mac's
// tailscaled fails to answer.
func (ws *WebServer) requireSameUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		st, err := ws.whois.StatusWithoutPeers(ctx)
		if err != nil || st == nil || st.Self == nil {
			slog.Debug("webserver: same-user check: no self status", "err", err)
			refuseLocalFault(w)
			return
		}
		who, err := ws.whois.WhoIs(ctx, r.RemoteAddr)
		if err != nil && !errors.Is(err, local.ErrPeerNotFound) {
			slog.Debug("webserver: same-user check: whois failed", "err", err)
			refuseLocalFault(w)
			return
		}
		if err != nil || !sameUser(who, st.Self) {
			var peerUser int64
			if who != nil && who.Node != nil {
				peerUser = int64(who.Node.User)
			}
			// Numeric IDs only, no names (RESEARCH A2: the two-Mac UAT reads this).
			slog.Debug("webserver: same-user check refused", "peer_user", peerUser, "self_user", int64(st.Self.UserID), "err", err)
			refuseOtherUser(w)
			return
		}
		next(w, r)
	}
}

// handlePeerCap handles POST /api/peer/sessions/{id}/cap: a same-account
// peer receives the share's capability with nothing to type. POST plus a
// required JSON content type forces a CORS preflight that this server never
// answers, so a cross-site page cannot read the token.
func (ws *WebServer) handlePeerCap(w http.ResponseWriter, r *http.Request) {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mt != "application/json" {
		http.Error(w, "content type must be application/json", http.StatusUnsupportedMediaType)
		return
	}
	_, _ = io.Copy(io.Discard, http.MaxBytesReader(w, r.Body, 8<<10))
	if ws.shareToken == nil {
		http.NotFound(w, r)
		return
	}
	token, ok := ws.shareToken(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(map[string]string{"cap": token}) //nolint:errcheck
}
