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
	webfs "github.com/scottkw/portty/web"
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

// noLiveWhoIs fails every lookup with ErrLiveTailnetInTest, so a test binary
// that forgets SetWhoIsClientForTest is refused (503) instead of asking the
// user's tailscaled (WR-07).
type noLiveWhoIs struct{}

func (noLiveWhoIs) WhoIs(context.Context, string) (*apitype.WhoIsResponse, error) {
	return nil, tailnet.ErrLiveTailnetInTest
}

func (noLiveWhoIs) StatusWithoutPeers(context.Context) (*ipnstate.Status, error) {
	return nil, tailnet.ErrLiveTailnetInTest
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

// sameUserStatus is the one same-user decision: 0 admits the caller, 403 means
// another account (or no tailnet peer), 503 means this Mac's tailscaled failed
// to answer. Every error fails closed.
func (ws *WebServer) sameUserStatus(r *http.Request) int {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	st, err := ws.whois.StatusWithoutPeers(ctx)
	if err != nil || st == nil || st.Self == nil {
		slog.Debug("webserver: same-user check: no self status", "err", err)
		return http.StatusServiceUnavailable
	}
	who, err := ws.whois.WhoIs(ctx, r.RemoteAddr)
	if err != nil && !errors.Is(err, local.ErrPeerNotFound) {
		slog.Debug("webserver: same-user check: whois failed", "err", err)
		return http.StatusServiceUnavailable
	}
	if err != nil || !sameUser(who, st.Self) {
		var peerUser int64
		if who != nil && who.Node != nil {
			peerUser = int64(who.Node.User)
		}
		// Numeric IDs only, no names (RESEARCH A2: the two-Mac UAT reads this).
		slog.Debug("webserver: same-user check refused", "peer_user", peerUser, "self_user", int64(st.Self.UserID), "err", err)
		return http.StatusForbidden
	}
	return 0
}

// requireSameUser admits only callers whose tailnet node is logged in as this
// Mac's Tailscale user. Every error fails closed: 403 with the marker when the
// answer says another account (or no tailnet peer), 503 when this Mac's
// tailscaled fails to answer.
func (ws *WebServer) requireSameUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch ws.sameUserStatus(r) {
		case 0:
			next(w, r)
		case http.StatusForbidden:
			refuseOtherUser(w)
		default:
			refuseLocalFault(w)
		}
	}
}

// requireSameUserPage is requireSameUser for the browser-facing page routes:
// the refusal is a fixed static page (403 with the marker, or 503 with a retry
// link and no marker). Nothing from the request is rendered into either page.
func (ws *WebServer) requireSameUserPage(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := ws.sameUserStatus(r)
		if status == 0 {
			next(w, r)
			return
		}
		page := serviceRetryPage
		if status == http.StatusForbidden {
			w.Header().Set(tailnet.MarkerHeader, tailnet.MarkerValue)
			page = userRefusedPage
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(status)
		_, _ = w.Write(page)
	}
}

// userRefusedPage and serviceRetryPage are the embedded web/user-refused.html
// and web/service-retry.html.
var (
	userRefusedPage  = mustWebFile("user-refused.html")
	serviceRetryPage = mustWebFile("service-retry.html")
)

func mustWebFile(name string) []byte {
	b, err := webfs.WebFS.ReadFile(name)
	if err != nil {
		panic("webserver: embedded " + name + ": " + err.Error())
	}
	return b
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
