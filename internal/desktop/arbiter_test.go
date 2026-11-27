package desktop

import (
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
