package desktop_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/desktop"
	"github.com/scottkw/portty/internal/screencap"
)

// leaseRig is the fakes behind a lease test: a clock, a bridge, a switchable
// physical reading and lock state, and a tap that counts its starts and stops.
type leaseRig struct {
	fb     *fakeInputBridge
	clk    *fakeClock
	phys   atomic.Pointer[screencap.PhysicalReading]
	locked atomic.Bool
	secure atomic.Bool

	starts, stops atomic.Int32
}

func newLeaseEnv(t *testing.T) (*guestEnv, *leaseRig) {
	t.Helper()
	r := &leaseRig{fb: &fakeInputBridge{}, clk: newFakeClock()}
	r.setPhysical(screencap.PhysicalReading{Healthy: true})
	e := newGuestEnv(t, func(d *desktop.Deps) {
		d.Input = r.fb
		d.DisplayBounds = testDisplay
		d.Now = r.clk.Now
		d.Lock = func() (bool, error) { return r.locked.Load(), nil }
		d.Physical = func() screencap.PhysicalReading { return *r.phys.Load() }
		d.StartTap = func() error { r.starts.Add(1); return nil }
		d.StopTap = func() { r.stops.Add(1) }
		d.TapState = func() (bool, bool) { return true, true }
		d.SecureInputOn = r.secure.Load
	})
	return e, r
}

func (r *leaseRig) setPhysical(p screencap.PhysicalReading) { r.phys.Store(&p) }

// leaseIs matches a lease message with this state and name ("" means no name).
func leaseIs(state, name string) func(map[string]any) bool {
	return func(m map[string]any) bool {
		n, _ := m["name"].(string)
		return m["state"] == state && n == name
	}
}

// grant gives g control and waits until it has heard it and the lease state.
func (r *leaseRig) grant(t *testing.T, e *guestEnv, g *guestSession) {
	t.Helper()
	grantControl(t, e, g)
	g.wait(t, "lease", leaseIs("free", ""))
}

func keyDown(code string) map[string]any {
	return map[string]any{"t": "key", "d": true, "code": code, "m": 0}
}

// TestLease_TwoGuestsTakeTurns: A's input takes the lease and both hear it; B is
// dropped while it lasts; two seconds after A's last input B posts and A's held
// key is let go first.
func TestLease_TwoGuestsTakeTurns(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	e, r := newLeaseEnv(t)
	a := e.joinGuest(ctx, t, 1001, "nodeA")
	b := e.joinGuest(ctx, t, 1002, "nodeB")
	r.grant(t, e, a)
	r.grant(t, e, b)

	a.input(t, keyDown("KeyA"))
	r.fb.waitFor(t, 1)
	a.wait(t, "lease", leaseIs("you", ""))
	b.wait(t, "lease", leaseIs("other", "Ada"))

	r.clk.Advance(time.Second)
	b.input(t, keyDown("KeyB"))
	eventually(t, 5*time.Second, "lease_held counted", func() bool { return guestCounter(e, "lease_held") == 1 })
	if n := len(r.fb.snapshot()); n != 1 {
		t.Fatalf("%d bridge calls, want 1 (B is dropped while A holds the lease)", n)
	}

	r.clk.Advance(1100 * time.Millisecond) // 2.1 s after A's last input
	b.input(t, keyDown("KeyB"))
	calls := r.fb.waitFor(t, 3)
	ka, _ := screencap.KeyCode("KeyA")
	kb, _ := screencap.KeyCode("KeyB")
	want := []struct {
		vk   uint16
		down bool
	}{{ka, true}, {ka, false}, {kb, true}}
	for i, w := range want {
		if c := calls[i]; c.Op != "key" || c.VK != w.vk || c.Down != w.down {
			t.Errorf("call %d = %+v, want key %#x down=%v (A's key is released before B's posts)", i, c, w.vk, w.down)
		}
	}
	b.wait(t, "lease", leaseIs("you", ""))
	a.wait(t, "lease", leaseIs("other", "Ada"))
}

// TestLease_OwnAccountHolderIsOwnerToGuests: an own-account holder is "owner"
// with no name to a guest, and a guest holder is "other" with its name to the
// own-account viewer.
func TestLease_OwnAccountHolderIsOwnerToGuests(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	e, r := newLeaseEnv(t)
	g := e.joinGuest(ctx, t, 1001, "nodeA")
	r.grant(t, e, g)
	o := e.ownViewer(ctx, t)
	o.wait(t, "lease", leaseIs("free", ""))

	o.input(t, map[string]any{"t": "move", "x": 0.5, "y": 0.5, "e": 1})
	r.fb.waitFor(t, 1)
	m := g.wait(t, "lease", leaseIs("owner", ""))
	if _, has := m["name"]; has {
		t.Errorf("owner lease carries a name: %v", m)
	}
	o.wait(t, "lease", leaseIs("you", ""))

	r.clk.Advance(2100 * time.Millisecond)
	g.input(t, map[string]any{"t": "move", "x": 0.2, "y": 0.2, "e": 1})
	r.fb.waitFor(t, 2)
	o.wait(t, "lease", leaseIs("other", "Ada"))
	g.wait(t, "lease", leaseIs("you", ""))
}

// TestPhysical_UntaggedInputDropsAndReleases: while the person at the Mac is
// typing, every remote press drops as physical, what a remote viewer holds is
// let go, viewers are told physical, and it is shown for at least 2 s.
func TestPhysical_UntaggedInputDropsAndReleases(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	e, r := newLeaseEnv(t)
	a := e.joinGuest(ctx, t, 1001, "nodeA")
	b := e.joinGuest(ctx, t, 1002, "nodeB")
	r.grant(t, e, a)
	r.grant(t, e, b)

	a.input(t, keyDown("KeyA"))
	r.fb.waitFor(t, 1)

	r.setPhysical(screencap.PhysicalReading{Healthy: true, Seen: true, Idle: 300 * time.Millisecond})
	b.input(t, keyDown("KeyB"))
	calls := r.fb.waitFor(t, 2)
	ka, _ := screencap.KeyCode("KeyA")
	if c := calls[1]; c.Op != "key" || c.VK != ka || c.Down {
		t.Errorf("call 1 = %+v, want A's held key released", c)
	}
	if guestCounter(e, "physical") != 1 {
		t.Errorf("physical drops = %d, want 1", guestCounter(e, "physical"))
	}
	e.mgr.PushLeaseForTest(ctx)
	a.wait(t, "lease", leaseIs("physical", ""))
	b.wait(t, "lease", leaseIs("physical", ""))

	// The reading goes quiet, but the state is held for the 2 s dwell.
	told := b.count("lease")
	r.setPhysical(screencap.PhysicalReading{Healthy: true, Seen: true, Idle: 5 * time.Second})
	r.clk.Advance(1500 * time.Millisecond)
	e.mgr.PushLeaseForTest(ctx)
	time.Sleep(100 * time.Millisecond)
	if got := b.count("lease"); got != told {
		t.Errorf("b was told something new after 1.5 s (%d messages, was %d); it must still be physical", got, told)
	}
	r.clk.Advance(700 * time.Millisecond)
	e.mgr.PushLeaseForTest(ctx)
	eventually(t, 5*time.Second, "free after the dwell", func() bool { return b.lastLease() == "free" })

	b.input(t, keyDown("KeyB"))
	r.fb.waitFor(t, 3) // posts again
	if n := len(r.fb.snapshot()); n != 3 {
		t.Errorf("%d bridge calls, want 3", n)
	}
}

// TestPhysical_UnhealthyTapFailsClosed: a tap that cannot show nobody is at the
// Mac drops every remote press as physical_unknown and releases what is held;
// a healthy reading afterwards allows the next one. The viewers are not told
// "someone is using this Mac" for it.
func TestPhysical_UnhealthyTapFailsClosed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	e, r := newLeaseEnv(t)
	a := e.joinGuest(ctx, t, 1001, "nodeA")
	r.grant(t, e, a)

	a.input(t, keyDown("KeyA"))
	r.fb.waitFor(t, 1)

	r.setPhysical(screencap.PhysicalReading{Healthy: false})
	a.input(t, keyDown("KeyB"))
	calls := r.fb.waitFor(t, 2)
	ka, _ := screencap.KeyCode("KeyA")
	if c := calls[1]; c.Op != "key" || c.VK != ka || c.Down {
		t.Errorf("call 1 = %+v, want A's held key released", c)
	}
	a.input(t, keyDown("KeyC"))
	eventually(t, 5*time.Second, "physical_unknown counted twice", func() bool { return guestCounter(e, "physical_unknown") == 2 })
	if n := len(r.fb.snapshot()); n != 2 {
		t.Errorf("%d bridge calls with an unhealthy tap, want 2 (nothing posts)", n)
	}
	e.mgr.PushLeaseForTest(ctx)
	if a.find("lease", leaseIs("physical", "")) != nil {
		t.Error("an unhealthy tap was reported to viewers as someone using the Mac")
	}

	r.setPhysical(screencap.PhysicalReading{Healthy: true})
	a.input(t, keyDown("KeyD"))
	r.fb.waitFor(t, 3)
}

// TestPhysical_SecureInputStillPosts: with Secure Event Input on and a healthy
// tap, a granted guest's key still posts, and SecureInput() reports it for the
// GUI warning while the tap runs.
func TestPhysical_SecureInputStillPosts(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	e, r := newLeaseEnv(t)
	g := e.joinGuest(ctx, t, 1001, "nodeA")
	r.grant(t, e, g)
	r.secure.Store(true)
	e.mgr.PollTapForTest()
	if !e.mgr.SecureInput() {
		t.Error("SecureInput() = false after a poll with Secure Event Input on and the tap running")
	}
	g.input(t, keyDown("KeyA"))
	r.fb.waitFor(t, 1)

	r.secure.Store(false)
	e.mgr.PollTapForTest()
	if e.mgr.SecureInput() {
		t.Error("SecureInput() = true after it went off")
	}
}

// TestLease_ViewOnlyGetsNoLeaseAndDroppedPressNeverTakesIt: a view-only guest is
// sent no lease message, and a press the lock check drops does not take the lease.
func TestLease_ViewOnlyGetsNoLeaseAndDroppedPressNeverTakesIt(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	e, r := newLeaseEnv(t)
	a := e.joinGuest(ctx, t, 1001, "nodeA")
	b := e.joinGuest(ctx, t, 1002, "nodeB")
	v := e.joinGuest(ctx, t, 1003, "nodeC") // stays view-only
	r.grant(t, e, a)
	r.grant(t, e, b)

	r.locked.Store(true)
	r.clk.Advance(300 * time.Millisecond) // past the lock cache
	a.input(t, keyDown("KeyA"))
	eventually(t, 5*time.Second, "locked counted", func() bool { return guestCounter(e, "locked") == 1 })
	r.locked.Store(false)
	r.clk.Advance(300 * time.Millisecond)
	b.input(t, keyDown("KeyB")) // would be lease_held had A's dropped press taken it
	r.fb.waitFor(t, 1)
	if n := guestCounter(e, "lease_held"); n != 0 {
		t.Errorf("lease_held = %d, want 0", n)
	}
	b.wait(t, "lease", leaseIs("you", ""))
	if n := v.count("lease"); n != 0 {
		t.Errorf("a view-only guest got %d lease messages, want 0", n)
	}
}

// TestLease_TapLifeCycle: the tap starts when the first controller appears (an
// own-account viewer joining, a guest granted control) before any press, and
// stops after the last one leaves or loses control; AnyController agrees.
func TestLease_TapLifeCycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	e, r := newLeaseEnv(t)
	g := e.joinGuest(ctx, t, 1001, "nodeA")
	if e.mgr.AnyController() || r.starts.Load() != 0 {
		t.Fatalf("a view-only guest started the tap (starts %d) or counts as a controller", r.starts.Load())
	}

	if err := e.mgr.SetRights(g.viewerID(), true, false); err != nil {
		t.Fatal(err)
	}
	if !e.mgr.AnyController() || r.starts.Load() != 1 {
		t.Fatalf("after the grant: AnyController %v, starts %d, want true and 1", e.mgr.AnyController(), r.starts.Load())
	}
	if err := e.mgr.SetRights(g.viewerID(), false, true); err != nil {
		t.Fatal(err)
	}
	if e.mgr.AnyController() || r.stops.Load() != 1 {
		t.Fatalf("after control off: AnyController %v, stops %d, want false and 1", e.mgr.AnyController(), r.stops.Load())
	}

	o := e.ownViewer(ctx, t) // an own-account viewer is a controller, and it started before its answer
	if !e.mgr.AnyController() || r.starts.Load() != 2 {
		t.Fatalf("after an own-account viewer joined: AnyController %v, starts %d, want true and 2", e.mgr.AnyController(), r.starts.Load())
	}
	_ = o.conn.CloseNow()
	eventually(t, 5*time.Second, "the tap to stop after the last controller left", func() bool { return r.stops.Load() == 2 })
	if e.mgr.AnyController() {
		t.Error("AnyController is still true after the last controller left")
	}
}

// TestLease_TapStartRetriedAfterFailure: a start that fails (no Accessibility
// yet) is retried by the next poll, and remote input is held back meanwhile
// because the tap is not healthy.
func TestLease_TapStartRetriedAfterFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var starts atomic.Int32
	var fail atomic.Bool
	fail.Store(true)
	e2 := newGuestEnv(t, func(d *desktop.Deps) {
		d.Physical = healthyPhysical
		d.StartTap = func() error {
			starts.Add(1)
			if fail.Load() {
				return screencap.ErrInputTapInTests
			}
			return nil
		}
		d.StopTap = func() {}
		d.TapState = func() (bool, bool) { return true, true }
		d.SecureInputOn = func() bool { return false }
	})
	g := e2.joinGuest(ctx, t, 1001, "nodeA")
	if err := e2.mgr.SetRights(g.viewerID(), true, false); err != nil {
		t.Fatal(err)
	}
	if starts.Load() != 1 {
		t.Fatalf("starts = %d after the grant, want 1", starts.Load())
	}
	e2.mgr.PollTapForTest()
	if starts.Load() != 2 {
		t.Fatalf("starts = %d after a poll with the tap down, want 2 (retried)", starts.Load())
	}
	fail.Store(false)
	e2.mgr.PollTapForTest()
	e2.mgr.PollTapForTest()
	if starts.Load() != 3 {
		t.Errorf("starts = %d, want 3 (no more attempts once it runs)", starts.Load())
	}
}

// lastLease is the state of the last lease message the guest got.
func (g *guestSession) lastLease() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	for i := len(g.msgs) - 1; i >= 0; i-- {
		if g.msgs[i]["type"] == "lease" {
			s, _ := g.msgs[i]["state"].(string)
			return s
		}
	}
	return ""
}
