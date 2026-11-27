package desktop

import (
	"time"

	"github.com/pion/webrtc/v4"
)

// SetTimerForTest replaces the admit timeout's timer, so a test fires the
// timeout by hand and no test sleeps five minutes.
func (g *GuestShare) SetTimerForTest(f func(d time.Duration) (<-chan time.Time, func())) {
	g.timer = f
}

// AttachForTest records a live connection for key, as a joined guest does.
func (g *GuestShare) AttachForTest(key AdmissionKey, id ViewerID, closeFn func()) {
	g.attach(key, id, closeFn)
}

// CheckMediaIdentityForTest runs the media check for viewer id on a selected
// pair whose remote candidate is addr and port, as pion's pair-change callback
// does; withPair false runs it as if pion had no selected pair. It reports
// false when no such viewer is joined.
func (m *Manager) CheckMediaIdentityForTest(id ViewerID, withPair bool, addr string, port uint16) bool {
	m.vmu.Lock()
	v := m.viewers[id]
	m.vmu.Unlock()
	if v == nil {
		return false
	}
	var pair *webrtc.ICECandidatePair
	if withPair {
		pair = &webrtc.ICECandidatePair{Remote: &webrtc.ICECandidate{Address: addr, Port: port}}
	}
	m.checkMediaIdentity(v, pair)
	return true
}
