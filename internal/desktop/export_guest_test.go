package desktop

import "time"

// SetTimerForTest replaces the admit timeout's timer, so a test fires the
// timeout by hand and no test sleeps five minutes.
func (g *GuestShare) SetTimerForTest(f func(d time.Duration) (<-chan time.Time, func())) {
	g.timer = f
}

// AttachForTest records a live connection for key, as a joined guest does.
func (g *GuestShare) AttachForTest(key AdmissionKey, id ViewerID, closeFn func()) {
	g.attach(key, id, closeFn)
}
