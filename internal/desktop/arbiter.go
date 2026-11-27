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
)

// LeaseState is what viewers are told: who controls, and whether the person at
// the Mac is using it.
type LeaseState struct {
	Holder   ViewerID
	Physical bool
}

type arbiter struct {
	now  func() time.Time
	phys PhysicalSource

	mu     sync.Mutex
	holder ViewerID
}

func newArbiter(now func() time.Time, phys PhysicalSource) *arbiter {
	return &arbiter{now: now, phys: phys}
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
	if r.Seen && r.Idle < physicalHold {
		return false, reasonPhysical, a.takeHolderLocked()
	}
	return true, "", nil
}
