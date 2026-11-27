package desktop

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// lease.go: the arbiter's place in the daemon. pressAllowed asks decideLease
// after every other drop check, so the lease is taken only by input that would
// otherwise post. The lease loop tells controlling viewers who holds control and
// runs the input tap's life cycle: the tap exists only while somebody remote can
// control (or the re-lock rule is armed, relock.go), because it needs
// Accessibility and a listen-only tap on every key the person at the Mac types.

// Lease states a controlling viewer is told (the lease message's state).
const (
	typeLease = "lease"

	leaseYou         = "you"         // this viewer holds control
	leaseOther       = "other"       // another viewer holds it (a guest holder is named)
	leaseOwner       = "owner"       // an own-account viewer holds it; shown to guests, never named
	leasePhysical    = "physical"    // the person at the Mac is using it
	leaseUnavailable = "unavailable" // the input tap is unhealthy: every press is dropped
	leaseFree        = "free"
)

// tapPoll is how often the loop re-reads the tap's state and Secure Event Input.
const tapPoll = time.Second

// msgLease tells a controlling viewer who holds control now. Name is set only
// for a guest holder (sanitized display name), never for an own-account holder.
type msgLease struct {
	Type  string `json:"type"`
	State string `json:"state"`
	Name  string `json:"name,omitempty"`
}

// leaseCtl is the Manager's lease and tap state.
type leaseCtl struct {
	kick chan struct{} // a lease change may have happened: re-tell viewers now
	done chan struct{} // closed when leaseLoop has ended; set by Start under Manager.mu

	mu   sync.Mutex
	sent map[ViewerID]msgLease // the last message each viewer was sent

	tapMu     sync.Mutex // guards tapOn and tapFailed; held across the Start and Stop seams
	tapOn     bool       // StartTap succeeded and StopTap has not been called since
	tapFailed bool       // the last StartTap failed (logged once per streak)

	secure atomic.Bool // Secure Event Input was on at the last poll while the tap ran
}

// decideLease runs the arbiter for v's input msg. A physical drop (the person at
// the Mac, or a tap that cannot show nobody is there) lets go of everything every
// viewer holds; a lease change lets go of the previous holder's input first. The
// caller counts a drop.
func (m *Manager) decideLease(v *viewer, _ inputMsg) (ok bool, reason string) {
	ok, why, rel := m.arb.decide(v.ID, m.d.Now())
	if why == reasonPhysical || why == reasonPhysicalUnknown {
		m.releaseAllHeld("physical")
	}
	for _, id := range rel {
		if hv := m.lookupViewer(id); hv != nil {
			m.releaseHeld(hv, "lease")
		}
	}
	m.kickLease()
	return ok, why
}

// releaseAllHeld lets go of everything every viewer holds (the input goroutine).
func (m *Manager) releaseAllHeld(reason string) {
	m.vmu.Lock()
	vs := make([]*viewer, 0, len(m.viewers))
	for _, v := range m.viewers {
		vs = append(vs, v)
	}
	m.vmu.Unlock()
	for _, v := range vs {
		m.releaseHeld(v, reason)
	}
}

// kickLease asks the lease loop to re-tell viewers now; it never blocks.
func (m *Manager) kickLease() {
	select {
	case m.lease.kick <- struct{}{}:
	default:
	}
}

// isController reports whether v may send input.
func (m *Manager) isController(v *viewer) bool { return m.rightsOf(v)&RightsControl != 0 }

// AnyController reports whether some connected viewer can control: any
// own-account viewer, or a guest holding a control grant.
func (m *Manager) AnyController() bool {
	m.vmu.Lock()
	defer m.vmu.Unlock()
	for _, v := range m.viewers {
		if m.isController(v) {
			return true
		}
	}
	return false
}

// SecureInput reports whether Secure Event Input was on at the last poll while
// the tap runs. While it is, the tap cannot see the person at the Mac's keys.
func (m *Manager) SecureInput() bool { return m.lease.secure.Load() }

// syncTap starts the tap when a controller exists (or re-lock is armed) and it
// is not running, and stops it when neither does. It runs at once when a controller joins, is granted
// control or leaves, and every poll (which retries a start that failed, for
// instance before Accessibility was granted).
func (m *Manager) syncTap() {
	c := &m.lease
	c.tapMu.Lock()
	defer c.tapMu.Unlock()
	want := !m.closed.Load() && (m.AnyController() || m.relockArmed())
	switch {
	case want && !c.tapOn:
		if err := m.d.StartTap(); err != nil {
			if !c.tapFailed {
				slog.Warn("desktop: input tap did not start; remote input is held back until it does", "err", err)
			}
			c.tapFailed = true
			return
		}
		c.tapOn, c.tapFailed = true, false
	case !want && c.tapOn:
		m.d.StopTap()
		c.tapOn, c.tapFailed = false, false
		c.secure.Store(false)
	case !want:
		c.tapFailed = false
	}
}

// pollTap is one pass of the once-a-second poll: the tap's life cycle, its
// enabled state (a disabled tap is rebuilt by the read) and Secure Event Input.
func (m *Manager) pollTap() {
	m.syncTap()
	c := &m.lease
	c.tapMu.Lock()
	on := c.tapOn
	c.tapMu.Unlock()
	if !on {
		return
	}
	_, _ = m.d.TapState()
	c.secure.Store(m.d.SecureInputOn())
}

// leaseFor is what viewer r is told when the lease is st and hv (nil when
// absent) holds it.
func leaseFor(st LeaseState, r, hv *viewer) msgLease {
	msg := msgLease{Type: typeLease}
	switch {
	case st.Unavailable:
		msg.State = leaseUnavailable
	case st.Physical:
		msg.State = leasePhysical
	case st.Holder == "" || hv == nil:
		msg.State = leaseFree
	case hv == r:
		msg.State = leaseYou
	case hv.Mode == ModeGuest:
		msg.State = leaseOther
		if msg.Name = hv.Identity.DisplayName; msg.Name == "" {
			msg.Name = hv.Identity.LoginName
		}
	case r.Mode == ModeGuest:
		msg.State = leaseOwner
	default:
		msg.State = leaseOther
	}
	return msg
}

// pushLease tells every controlling viewer the lease state, each only when it
// differs from what that viewer was last told. A viewer that is not a controller
// is never sent one and is forgotten, so it hears the state when it becomes one.
func (m *Manager) pushLease(ctx context.Context) {
	st := m.arb.state(m.d.Now())
	type to struct {
		v   *viewer
		msg msgLease
	}
	var out []to
	live := make(map[ViewerID]bool)
	m.vmu.Lock()
	hv := m.viewers[st.Holder]
	for id, v := range m.viewers {
		if m.isController(v) && v.leaseReady.Load() {
			live[id] = true
			out = append(out, to{v, leaseFor(st, v, hv)})
		}
	}
	m.vmu.Unlock()

	c := &m.lease
	c.mu.Lock()
	for id := range c.sent {
		if !live[id] {
			delete(c.sent, id)
		}
	}
	fresh := out[:0]
	for _, t := range out {
		if last, ok := c.sent[t.v.ID]; !ok || last != t.msg {
			c.sent[t.v.ID] = t.msg
			fresh = append(fresh, t)
		}
	}
	c.mu.Unlock()
	for _, t := range fresh {
		_ = send(ctx, t.v.ws, t.msg)
	}
}

// leaseLoop re-tells viewers on a kick and polls the tap every tapPoll.
func (m *Manager) leaseLoop(ctx context.Context, done chan<- struct{}) {
	defer close(done)
	t := time.NewTicker(tapPoll)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.lease.kick:
			m.leasePass(ctx, false)
		case <-t.C:
			m.leasePass(ctx, true)
		}
	}
}

// leasePass is one pass; a panic costs the pass, never the loop.
func (m *Manager) leasePass(ctx context.Context, poll bool) {
	defer recoverLog()
	if poll {
		m.pollTap()
		m.relockPoll(m.d.Now()) // after the tap's own poll, so a tap started this pass is read
	}
	m.pushLease(ctx)
}
