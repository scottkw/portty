package webserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net"
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

// PeerIdentity is who is on the other end of a guest connection, taken from
// WhoIs on that connection's remote address and never from a token. The text
// fields are raw WhoIs strings: whatever shows them (the GUI, the audit log)
// sanitizes them first, and this package never logs a name.
type PeerIdentity struct {
	UserID      int64
	LoginName   string
	DisplayName string
	NodeName    string
	StableID    string
	IP          string
}

// tailnetPeer is the guest-family identity decision: any person on the tailnet
// whose device the Mac can name. It refuses a tagged node, a node shared in from
// another tailnet, a zero user ID, a missing profile (the Admit prompt could not
// name the person) and, as sameUser does, a tagged or shared-in Mac of its own.
// It does not compare the caller's user with self: that is the widening, and
// sameUser stays the gate for every other route.
func tailnetPeer(who *apitype.WhoIsResponse, self *ipnstate.PeerStatus) (PeerIdentity, bool) {
	if who == nil || who.Node == nil || who.UserProfile == nil || self == nil {
		return PeerIdentity{}, false
	}
	if who.Node.IsTagged() || who.Node.Sharer != 0 || who.Node.User == 0 {
		return PeerIdentity{}, false
	}
	if self.Tags != nil && self.Tags.Len() > 0 {
		return PeerIdentity{}, false
	}
	if self.AltSharerUserID != 0 || self.UserID == 0 {
		return PeerIdentity{}, false
	}
	return PeerIdentity{
		UserID:      int64(who.Node.User),
		LoginName:   who.UserProfile.LoginName,
		DisplayName: who.UserProfile.DisplayName,
		NodeName:    who.Node.ComputedName,
		StableID:    string(who.Node.StableID),
	}, true
}

// tailnetPeerStatus is the one guest-family decision, shaped like
// sameUserStatus: 0 admits the caller and returns who they are, 403 means
// refused (another tailnet, a tagged node, no profile, or no tailnet peer at
// all), 503 means this Mac's tailscaled failed to answer. Every error fails
// closed.
func (ws *WebServer) tailnetPeerStatus(r *http.Request) (int, PeerIdentity) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	st, err := ws.whois.StatusWithoutPeers(ctx)
	if err != nil || st == nil || st.Self == nil {
		slog.Debug("webserver: tailnet-peer check: no self status", "err", err)
		return http.StatusServiceUnavailable, PeerIdentity{}
	}
	who, err := ws.whois.WhoIs(ctx, r.RemoteAddr)
	if err != nil && !errors.Is(err, local.ErrPeerNotFound) {
		slog.Debug("webserver: tailnet-peer check: whois failed", "err", err)
		return http.StatusServiceUnavailable, PeerIdentity{}
	}
	var id PeerIdentity
	ok := false
	if err == nil {
		id, ok = tailnetPeer(who, st.Self)
	}
	if !ok {
		var peerUser int64
		if who != nil && who.Node != nil {
			peerUser = int64(who.Node.User)
		}
		// Numeric IDs only, no names.
		slog.Debug("webserver: tailnet-peer check refused", "peer_user", peerUser, "self_user", int64(st.Self.UserID), "err", err)
		return http.StatusForbidden, PeerIdentity{}
	}
	if host, _, splitErr := net.SplitHostPort(r.RemoteAddr); splitErr == nil {
		id.IP = host
	} else {
		id.IP = r.RemoteAddr
	}
	return 0, id
}

type peerIdentityKey struct{}

func withPeerIdentity(ctx context.Context, id PeerIdentity) context.Context {
	return context.WithValue(ctx, peerIdentityKey{}, id)
}

// peerIdentityFrom returns the identity requireTailnetPeer stored; false means
// the handler was reached without the gate, which the caller must refuse.
func peerIdentityFrom(ctx context.Context) (PeerIdentity, bool) {
	id, ok := ctx.Value(peerIdentityKey{}).(PeerIdentity)
	return id, ok
}

// requireTailnetPeer admits a person on this Mac's tailnet whose device
// tailnetPeer accepts, and hands the handler their identity in the request
// context. Only the /desktop/guest/ family uses it (TestGuestRouteTable_AST
// pins that). 403 with the marker for a refusal, 503 with none when this Mac's
// tailscaled cannot answer.
func (ws *WebServer) requireTailnetPeer(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch status, id := ws.tailnetPeerStatus(r); status {
		case 0:
			next(w, r.WithContext(withPeerIdentity(r.Context(), id)))
		case http.StatusForbidden:
			refuseOtherUser(w)
		default:
			refuseLocalFault(w)
		}
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

// handlePeerDesktopCap handles POST /api/peer/desktop/cap, the twin of
// handlePeerCap: a same-account peer daemon receives the standing
// always-available desktop token with nothing to type. The request checks are the
// same (JSON content type forces a CORS preflight this server never answers; body
// capped at 8 KiB). The token comes from an optional DesktopToken method on the
// backend, so DesktopBackend and its fakes do not change; no backend, no method or
// "off" is a 404. Cache-Control: no-store; nothing is logged, so the token never
// reaches a log line.
func (ws *WebServer) handlePeerDesktopCap(w http.ResponseWriter, r *http.Request) {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mt != "application/json" {
		http.Error(w, "content type must be application/json", http.StatusUnsupportedMediaType)
		return
	}
	_, _ = io.Copy(io.Discard, http.MaxBytesReader(w, r.Body, 8<<10))
	p, ok := ws.desktop.(interface{ DesktopToken() (string, bool) })
	if !ok {
		http.NotFound(w, r)
		return
	}
	token, ok := p.DesktopToken()
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(map[string]string{"cap": token}) //nolint:errcheck
}
