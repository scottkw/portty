package desktop

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"strconv"
	"time"

	"github.com/pion/webrtc/v4"
)

// identityLookupTimeout bounds the WhoIs call a media check makes.
const identityLookupTimeout = 3 * time.Second

// ErrPeerLookupStopped is what Deps.PeerIdentity returns when it cannot look
// anything up because the share server has stopped (the share is ending). It is
// no verdict, not a mismatch: Stop is already cutting the guests. Every other
// lookup error still fails closed.
var ErrPeerLookupStopped = errors.New("desktop: peer lookup stopped with the share server")

// bindGuestMedia ties a guest's media path to the node it was admitted as. The
// signalling socket proves who knocked, but the media runs over UDP to the
// address in the guest's own offer, so the ICE pair pion selects is checked
// against the admitted identity whenever it changes, and once more when the
// PeerConnection connects (checkSelectedPair). The check is by node identity
// (WhoIs), not by comparing address strings (CONTEXT After research).
func (m *Manager) bindGuestMedia(v *viewer, pc *webrtc.PeerConnection) {
	pc.SCTP().Transport().ICETransport().OnSelectedCandidatePairChange(func(p *webrtc.ICECandidatePair) {
		go func() {
			defer recoverLog()
			m.checkMediaIdentity(v, p)
		}()
	})
}

// checkSelectedPair runs the media check on whatever pair is selected now. It
// runs on its own goroutine: the lookup must not hold up pion's callbacks.
func (m *Manager) checkSelectedPair(v *viewer) {
	go func() {
		defer recoverLog()
		pair, err := v.pc.SCTP().Transport().ICETransport().GetSelectedCandidatePair()
		if err != nil {
			pair = nil // no pair to vouch for: fail closed
		}
		m.checkMediaIdentity(v, pair)
	}()
}

// checkMediaIdentity cuts a guest whose selected pair's remote address does not
// belong to the admitted user's admitted node. A missing pair, a missing
// Deps.PeerIdentity and a failed lookup all cut too (fail closed). A check that
// ends after the viewer was cut or its PeerConnection closed (it left, was
// revoked, or Stop cut it during the lookup) is no verdict. The cut is the
// ordered one (cutViewer: rights, input, media, then the socket under the
// close ceiling). The log line carries the viewer ID prefix and the kind,
// never an address.
func (m *Manager) checkMediaIdentity(v *viewer, pair *webrtc.ICECandidatePair) {
	kind := m.mediaMismatch(v, pair)
	if kind == "" || v.cut.Load() || (v.pc != nil && v.pc.ConnectionState() == webrtc.PeerConnectionStateClosed) ||
		!v.mediaCut.CompareAndSwap(false, true) {
		return
	}
	slog.Warn("desktop: guest media identity", "viewer", string(v.ID)[:8], "kind", kind)
	v.gs.emit(GuestEvent{Kind: AuditRevoked, Reason: ReasonMediaIdentityMismatch, Ident: v.Identity})
	m.cutViewer(v, CloseGuestRevoked, guestReasonRevoked)
}

// mediaMismatch returns why the pair is not the admitted node's, or "" when it is.
func (m *Manager) mediaMismatch(v *viewer, pair *webrtc.ICECandidatePair) string {
	if m.d.PeerIdentity == nil {
		return "no_lookup"
	}
	if pair == nil || pair.Remote == nil {
		return "no_pair"
	}
	addr := net.JoinHostPort(pair.Remote.Address, strconv.Itoa(int(pair.Remote.Port)))
	ctx, cancel := context.WithTimeout(context.Background(), identityLookupTimeout)
	defer cancel()
	user, node, err := m.d.PeerIdentity(ctx, addr)
	switch {
	case errors.Is(err, ErrPeerLookupStopped):
		return "" // no verdict: the share is ending and Stop cuts the guests
	case err != nil:
		return "lookup_failed"
	case user != v.Identity.UserID:
		return "wrong_user"
	case node != v.Identity.StableID:
		return "wrong_node"
	}
	return ""
}
