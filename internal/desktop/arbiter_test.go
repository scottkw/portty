package desktop

import (
	"sync"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/screencap"
)

var arbiterT0 = time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)

// fakePhys is a PhysicalSource the test steers.
type fakePhys struct{ r screencap.PhysicalReading }

func (f *fakePhys) read() screencap.PhysicalReading { return f.r }

func healthy(idle time.Duration) screencap.PhysicalReading {
	return screencap.PhysicalReading{Healthy: true, Seen: true, Idle: idle}
}

var healthyNothingSeen = screencap.PhysicalReading{Healthy: true}

func newTestArbiter(r screencap.PhysicalReading) (*arbiter, *fakePhys) {
	p := &fakePhys{r: r}
	return newArbiter(func() time.Time { return arbiterT0 }, p.read), p
}

func TestArbiter_PhysicalWins(t *testing.T) {
	a, p := newTestArbiter(healthyNothingSeen)
	a.holder = "a"
	p.r = healthy(500 * time.Millisecond)
	ok, reason, release := a.decide("a", arbiterT0)
	if ok || reason != reasonPhysical || len(release) != 1 || release[0] != "a" {
		t.Fatalf("idle 0.5s: ok=%v reason=%q release=%v, want drop physical releasing a", ok, reason, release)
	}
	if a.holder != "" {
		t.Fatalf("holder %q after a physical drop, want the lease cleared", a.holder)
	}
	p.r = healthy(1500 * time.Millisecond)
	if ok, reason, _ := a.decide("a", arbiterT0); !ok {
		t.Fatalf("idle 1.5s dropped with %q, want allowed", reason)
	}
}

func TestArbiter_HealthyIdleAllows(t *testing.T) {
	a, _ := newTestArbiter(healthyNothingSeen)
	if ok, reason, release := a.decide("a", arbiterT0); !ok || reason != "" || release != nil {
		t.Fatalf("healthy tap, nothing seen: ok=%v reason=%q release=%v, want allowed", ok, reason, release)
	}
}

func TestArbiter_DisabledTapDrops(t *testing.T) {
	a, p := newTestArbiter(screencap.PhysicalReading{Healthy: false})
	a.holder = "a"
	ok, reason, release := a.decide("b", arbiterT0)
	if ok || reason != reasonPhysicalUnknown || len(release) != 1 || release[0] != "a" {
		t.Fatalf("unhealthy tap: ok=%v reason=%q release=%v, want drop physical_unknown releasing a", ok, reason, release)
	}
	if ok, reason, _ := a.decide("a", arbiterT0.Add(time.Second)); ok || reason != reasonPhysicalUnknown {
		t.Fatalf("second input on an unhealthy tap: ok=%v reason=%q, want drop physical_unknown", ok, reason)
	}
	p.r = healthyNothingSeen
	if ok, reason, _ := a.decide("a", arbiterT0.Add(2*time.Second)); !ok {
		t.Fatalf("healthy tap again dropped with %q, want allowed", reason)
	}
}

func at(d time.Duration) time.Time { return arbiterT0.Add(d) }

func TestArbiter_LeaseTakeRenewAndHold(t *testing.T) {
	a, _ := newTestArbiter(healthyNothingSeen)
	if ok, reason, release := a.decide("a", at(0)); !ok || reason != "" || release != nil {
		t.Fatalf("a's move on a free lease: ok=%v reason=%q release=%v, want taken", ok, reason, release)
	}
	if ok, _, _ := a.decide("a", at(1900*time.Millisecond)); !ok {
		t.Fatal("a's key at 1.9s was dropped, want it to renew")
	}
	// a renewed to 1.9s + 2s = 3.9s, so b is held out until then.
	for _, d := range []time.Duration{time.Second, 2 * time.Second, 3800 * time.Millisecond} {
		if ok, reason, release := a.decide("b", at(d)); ok || reason != reasonLeaseHeld || release != nil {
			t.Fatalf("b at %v: ok=%v reason=%q release=%v, want lease_held", d, ok, reason, release)
		}
	}
	if got := a.state(at(3800 * time.Millisecond)); got.Holder != "a" || got.Physical {
		t.Fatalf("state = %+v, want holder a, not physical", got)
	}
}

func TestArbiter_LeaseExpiryHandsOver(t *testing.T) {
	a, _ := newTestArbiter(healthyNothingSeen)
	a.decide("a", at(0))
	ok, reason, release := a.decide("b", at(leaseHold))
	if !ok || reason != "" || len(release) != 1 || release[0] != "a" {
		t.Fatalf("b 2.0s after a: ok=%v reason=%q release=%v, want taken, releasing a", ok, reason, release)
	}
	if got := a.state(at(leaseHold)); got.Holder != "b" {
		t.Fatalf("holder %q after the hand-over, want b", got.Holder)
	}
	// The same viewer coming back after its own expiry is not a lease change.
	if ok, _, release := a.decide("b", at(2*leaseHold+time.Second)); !ok || release != nil {
		t.Fatalf("b re-taking its own expired lease: ok=%v release=%v, want taken, nothing released", ok, release)
	}
	if got := a.state(at(10 * time.Second)); got.Holder != "" {
		t.Fatalf("holder %q long after the last input, want none", got.Holder)
	}
}

func TestArbiter_ForgetReleasesHolder(t *testing.T) {
	a, _ := newTestArbiter(healthyNothingSeen)
	a.decide("a", at(0))
	if a.forget("b") {
		t.Fatal("forget of a non-holder reported released")
	}
	if !a.forget("a") {
		t.Fatal("forget of the holder reported not released")
	}
	if ok, _, release := a.decide("b", at(100*time.Millisecond)); !ok || release != nil {
		t.Fatalf("b after a was forgotten: ok=%v release=%v, want a free lease", ok, release)
	}
}

func TestArbiter_PhysicalDwell(t *testing.T) {
	a, p := newTestArbiter(healthy(0))
	if ok, reason, _ := a.decide("a", at(0)); ok || reason != reasonPhysical {
		t.Fatalf("physical at t0: ok=%v reason=%q, want drop physical", ok, reason)
	}
	// Untagged input stopped at t0: held for the 2 s dwell, not just the 1 s hold.
	p.r = healthy(1500 * time.Millisecond)
	if got := a.state(at(1500 * time.Millisecond)); !got.Physical {
		t.Fatal("physical state flickered off at 1.5s, want it held for 2s")
	}
	p.r = healthy(2100 * time.Millisecond)
	if got := a.state(at(2100 * time.Millisecond)); got.Physical {
		t.Fatal("physical state still on at 2.1s")
	}
	// A new episode starting at 10s with input again at 11.3s ends 1s after that, at 12.3s.
	p.r = healthy(0)
	a.state(at(10 * time.Second))
	p.r = healthy(200 * time.Millisecond)
	if got := a.state(at(11500 * time.Millisecond)); !got.Physical {
		t.Fatal("physical state off at 1.5s of an episode with input at 1.3s")
	}
	p.r = healthy(1100 * time.Millisecond)
	if got := a.state(at(12400 * time.Millisecond)); got.Physical {
		t.Fatal("physical state still on 1.1s after the last input")
	}
}

func TestArbiter_SameInstantFirstDecidedWins(t *testing.T) {
	a, _ := newTestArbiter(healthyNothingSeen)
	ok, _, _ := a.decide("a", at(0))
	ok2, reason, _ := a.decide("b", at(0))
	if !ok || ok2 || reason != reasonLeaseHeld {
		t.Fatalf("same instant: a ok=%v, b ok=%v reason=%q, want a taken and b lease_held", ok, ok2, reason)
	}
}

func TestArbiter_ConcurrentDecideForgetState(t *testing.T) {
	a, _ := newTestArbiter(healthyNothingSeen)
	var wg sync.WaitGroup
	for i := range 4 {
		v := ViewerID(string(rune('a' + i)))
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range 200 {
				now := at(time.Duration(j) * 50 * time.Millisecond)
				a.decide(v, now)
				a.state(now)
				if j%7 == 0 {
					a.forget(v)
				}
			}
		}()
	}
	wg.Wait()
}
