package desktop

import (
	"sync"
	"time"

	"github.com/scottkw/portty/internal/screencap"
)

// arbiter.go: who may type into the Mac right now. Physical input wins: while
// the person at the Mac touched anything in the last physicalHold, remote input
// is dropped and what remote viewers hold is released. Among remote viewers one
// holds a lease. The arbiter is pure: the clock and the physical-input source
// are injected, and it posts nothing.

// PhysicalSource reports what is known about the person at the Mac. The
// production value is TapPhysicalSource.
type PhysicalSource func() screencap.PhysicalReading

// TapPhysicalSource reads the listen-only input tap.
func TapPhysicalSource() PhysicalSource { return screencap.PhysicalState }

// The drop reasons the arbiter names, in the style of the gate reasons.
const (
	reasonPhysical        = "physical"
	reasonPhysicalUnknown = "physical_unknown"
	reasonLeaseHeld       = "lease_held"
)

// Fixed rules, not settings.
const (
	physicalHold  = time.Second     // remote input is dropped this long after physical input
	leaseHold     = 2 * time.Second // a lease lasts this long after its holder's last input
	physicalDwell = 2 * time.Second // a physical episode is reported at least this long
	// unhealthyShown is how long the tap must stay unhealthy before viewers are
	// told control is unavailable: longer than the tap's one-second settle after
	// a start, so a normal start never flashes it.
	unhealthyShown = 2 * time.Second
)

// LeaseState is what viewers are told: who controls, whether the person at
// the Mac is using it, and whether the tap has been unhealthy for
// unhealthyShown (every press is dropped then, physical_unknown).
type LeaseState struct {
	Holder      ViewerID
	Physical    bool
	Unavailable bool
}

type arbiter struct {
	now  func() time.Time
	phys PhysicalSource

	mu       sync.Mutex
	holder   ViewerID
	leaseEnd time.Time // the holder's lease is live while now is before this

	// The current physical episode, for the state viewers see: it is reported
	// until physEnd, which is the later of lastPhys+physicalHold and
	// physStart+physicalDwell, so a single touch cannot make it flicker.
	physStart, lastPhys, physEnd time.Time

	unhealthySince time.Time // when state first saw the current unhealthy streak; zero while healthy
}

func newArbiter(now func() time.Time, phys PhysicalSource) *arbiter {
	return &arbiter{now: now, phys: phys}
}

// physicalActiveLocked reports whether r shows untagged input inside the hold,
// and folds it into the physical episode.
func (a *arbiter) physicalActiveLocked(r screencap.PhysicalReading, now time.Time) bool {
	if !r.Healthy || !r.Seen || r.Idle >= physicalHold {
		return false
	}
	if !now.Before(a.physEnd) {
		a.physStart = now
	}
	a.lastPhys = now.Add(-r.Idle)
	a.physEnd = a.lastPhys.Add(physicalHold)
	if dwellEnd := a.physStart.Add(physicalDwell); dwellEnd.After(a.physEnd) {
		a.physEnd = dwellEnd
	}
	return true
}

// takeHolderLocked clears the lease and returns who held it, for release.
func (a *arbiter) takeHolderLocked() []ViewerID {
	h := a.holder
	a.holder = ""
	if h == "" {
		return nil
	}
	return []ViewerID{h}
}

// decide says whether v's input may be posted. A drop for a physical reason
// also returns every remote holder, who must be released, and clears the lease.
// Physical knowledge fails closed: a tap that is not healthy cannot show that
// nobody is at the Mac, so every remote input is dropped; a healthy tap that
// has seen nothing allows it, so an idle person never blocks control.
func (a *arbiter) decide(v ViewerID, now time.Time) (ok bool, reason string, release []ViewerID) {
	a.mu.Lock()
	defer a.mu.Unlock()
	r := a.phys()
	if !r.Healthy {
		return false, reasonPhysicalUnknown, a.takeHolderLocked()
	}
	if a.physicalActiveLocked(r, now) {
		return false, reasonPhysical, a.takeHolderLocked()
	}
	// Any controlling viewer's input of any kind takes a free or expired lease,
	// and the holder's input renews it; another viewer's is dropped meanwhile.
	if a.holder == v {
		a.leaseEnd = now.Add(leaseHold)
		return true, "", nil
	}
	if a.holder != "" && now.Before(a.leaseEnd) {
		return false, reasonLeaseHeld, nil
	}
	prev := a.takeHolderLocked() // a lease change releases what the old holder held
	a.holder = v
	a.leaseEnd = now.Add(leaseHold)
	return true, "", prev
}

// forget drops v's claim to the lease (it left or lost its right to control)
// and reports whether v held it, so the caller releases what v holds.
func (a *arbiter) forget(v ViewerID) (released bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.holder != v {
		return false
	}
	a.holder = ""
	return true
}

// state is what viewers are told at now.
func (a *arbiter) state(now time.Time) LeaseState {
	a.mu.Lock()
	defer a.mu.Unlock()
	r := a.phys()
	a.physicalActiveLocked(r, now)
	s := LeaseState{Physical: now.Before(a.physEnd)}
	switch {
	case r.Healthy:
		a.unhealthySince = time.Time{}
	case a.unhealthySince.IsZero():
		a.unhealthySince = now
	}
	s.Unavailable = !r.Healthy && now.Sub(a.unhealthySince) >= unhealthyShown
	if a.holder != "" && now.Before(a.leaseEnd) {
		s.Holder = a.holder
	}
	return s
}
