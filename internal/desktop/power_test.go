package desktop_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/desktop"
	"github.com/scottkw/portty/internal/screencap"
	"github.com/scottkw/portty/internal/wakewatch"
)

const (
	viewerHold = "Portty desktop viewer connected"
	acHold     = "Portty always-available desktop on AC power"
)

func withWake(ch <-chan wakewatch.Reason) tracerOpt {
	return func(_ *tracerEnv, d *desktop.Deps) { d.Wake = ch }
}

// firstUnitIsKeyframe emits a delta then a keyframe until v receives a unit,
// and requires that unit to be the keyframe.
func firstUnitIsKeyframe(t *testing.T, r *rig, v *fanViewer, who string) {
	t.Helper()
	for len(v.lc.aus) > 0 {
		<-v.lc.aus
	}
	var first []byte
	eventually(t, 5*time.Second, who+": first unit", func() bool {
		r.src.last().emit(false)
		r.src.last().emit(true)
		select {
		case first = <-v.lc.aus:
			return true
		default:
			return false
		}
	})
	requireKeyframe(t, who, first)
}

func TestPower_ViewerAssertions(t *testing.T) {
	r := newRig(t)
	v1 := r.join()
	display, system, _ := r.w.held()
	if !reflect.DeepEqual(display, []string{viewerHold}) || !reflect.DeepEqual(system, []string{viewerHold}) {
		t.Fatalf("first join took display %v system %v, want one %q each", display, system, viewerHold)
	}
	if _, declares, _ := r.w.counts(); declares != 1 {
		t.Fatalf("declare activity %d times after the first join, want 1", declares)
	}

	v2 := r.env.join(r.ctx, t, r.env.token(t, "g1"))
	eventually(t, 3*time.Second, "second viewer", func() bool { return r.env.mgr.Viewers() == 2 })
	display, system, _ = r.w.held()
	if len(display) != 1 || len(system) != 1 {
		t.Fatalf("a second join took new assertions: display %v system %v", display, system)
	}
	if _, declares, _ := r.w.counts(); declares != 2 {
		t.Fatalf("declare activity %d times after two joins, want 2", declares)
	}

	_ = v2.conn.CloseNow()
	eventually(t, 3*time.Second, "one viewer left", func() bool { return r.env.mgr.Viewers() == 1 })
	if _, _, released := r.w.held(); released != 0 {
		t.Fatalf("released %d assertions with a viewer still connected", released)
	}
	r.leave(v1)
	eventually(t, 3*time.Second, "release on last leave", func() bool { _, _, n := r.w.held(); return n == 2 })

	// Close with a viewer connected releases both too.
	r2 := newRig(t)
	r2.join()
	_ = r2.env.mgr.Close()
	if _, _, released := r2.w.held(); released != 2 {
		t.Fatalf("Close with a viewer released %d assertions, want 2", released)
	}
}

func TestPower_ACOnly(t *testing.T) {
	w := newWorld()
	w.set(func() { w.ac = true })
	r := newRigWorld(t, w)
	if _, system, _ := w.held(); !reflect.DeepEqual(system, []string{acHold}) {
		t.Fatalf("on AC at Start the system holds are %v, want [%q]", system, acHold)
	}
	if display, _, _ := w.held(); len(display) != 0 {
		t.Fatalf("the AC assertion must not hold the display: %v", display)
	}

	w.set(func() { w.ac = false })
	r.settle(31 * time.Second)
	eventually(t, 3*time.Second, "release on battery", func() bool { _, _, n := w.held(); return n == 1 })

	w.set(func() { w.ac = true })
	r.settle(31 * time.Second)
	eventually(t, 3*time.Second, "re-take on AC", func() bool { _, s, _ := w.held(); return len(s) == 2 })

	_ = r.env.mgr.Close()
	if _, _, released := w.held(); released != 2 {
		t.Fatalf("after Close %d assertions released, want 2", released)
	}

	// Battery with no viewer: nothing held at all (the user's "only while plugged in").
	b := newRig(t)
	b.settle(31 * time.Second)
	if d, s, _ := b.w.held(); len(d)+len(s) != 0 {
		t.Fatalf("on battery with no viewer assertions were taken: %v %v", d, s)
	}
	v := b.join()
	if d, s, _ := b.w.held(); len(d) != 1 || len(s) != 1 || s[0] == acHold {
		t.Fatalf("a viewer on battery should hold only the viewer assertions: %v %v", d, s)
	}
	b.leave(v)
	eventually(t, 3*time.Second, "release", func() bool { _, _, n := b.w.held(); return n == 2 })
}

func TestPower_WakeRestartsStream(t *testing.T) {
	t.Run("display back", func(t *testing.T) {
		wake := make(chan wakewatch.Reason, 1)
		r := newRig(t, withWake(wake))
		v := r.join()
		r.w.set(func() { r.w.asleep = true })
		wake <- wakewatch.Woke
		r.settle(5 * time.Second)
		eventually(t, 3*time.Second, "display_asleep after the wake", func() bool { return r.state() == desktop.StateDisplayAsleep })
		if r.src.count() != 1 {
			t.Fatalf("restarted while the display was still asleep (%d sources)", r.src.count())
		}
		r.w.set(func() { r.w.asleep = false })
		eventually(t, 3*time.Second, "one restart once the display reads awake", func() bool { return r.src.count() == 2 })
		firstUnitIsKeyframe(t, r, v, "viewer after wake")
		eventually(t, 3*time.Second, "streaming", func() bool { return r.state() == desktop.StateStreaming })
		r.settle(time.Second)
		if r.src.count() != 2 {
			t.Fatalf("%d sources, want exactly one restart", r.src.count())
		}
	})
	t.Run("display stays asleep", func(t *testing.T) {
		wake := make(chan wakewatch.Reason, 1)
		r := newRig(t, withWake(wake))
		r.join()
		r.w.set(func() { r.w.asleep = true })
		wake <- wakewatch.Woke
		for i := 0; i < 6; i++ {
			r.settle(6 * time.Second)
		}
		if r.state() != desktop.StateDisplayAsleep || r.src.count() != 1 {
			t.Fatalf("after 36 s asleep: state %s, sources %d; want display_asleep and no restart", r.state(), r.src.count())
		}
	})
}

func TestPower_NoDisplayStopThenRestart(t *testing.T) {
	r := newRig(t)
	v1 := r.join()
	r.w.set(func() { r.w.asleep = true })
	r.src.last().fail(screencap.ErrNoDisplay)
	eventually(t, time.Second, "viewer told display_asleep at once", func() bool { s, _, _ := v1.lastState(); return s == "display_asleep" })

	// A viewer joining now sees the state and no stale picture.
	v2 := r.env.join(r.ctx, t, r.env.token(t, "g1"))
	eventually(t, 3*time.Second, "joiner told display_asleep", func() bool { s, _, _ := v2.lastState(); return s == "display_asleep" })
	for i := 0; i < 5; i++ {
		r.src.last().emit(true)
		time.Sleep(60 * time.Millisecond)
	}
	if len(v2.lc.aus) != 0 {
		t.Fatal("a joiner received media while the display was asleep")
	}

	r.w.set(func() { r.w.asleep = false })
	eventually(t, 3*time.Second, "restart once the display reads awake", func() bool { return r.src.count() == 2 })
	firstUnitIsKeyframe(t, r, v2, "joiner after the restart")
}

func TestPower_HoldFailureContinues(t *testing.T) {
	logs := captureLogs(t)
	r := newRig(t)
	r.w.set(func() {
		r.w.holdDisplayErr = errBoom("display hold refused")
		r.w.holdSystemErr = errBoom("system hold refused")
		r.w.lockErr = errBoom("no session")
		r.w.probeErr = errBoom("child failed")
	})
	v := r.join()
	r.env.mgr.RefreshPermissions(r.ctx)
	r.settle(3 * time.Second)

	reason := r.env.mgr.Status().Reason
	for _, want := range []string{"display hold", "system hold", "lock", "probe"} {
		if !strings.Contains(reason, want) {
			t.Errorf("Status.Reason %q lacks %q", reason, want)
		}
	}
	for _, line := range []string{"desktop: display hold failed", "desktop: system hold failed", "desktop: lock failed", "desktop: probe failed"} {
		if n := logs.count(line); n != 1 {
			t.Errorf("%q logged %d times, want once", line, n)
		}
	}
	firstUnitIsKeyframe(t, r, v, "viewer with every native call failing")

	// A software encoder is a diagnostic; streaming goes on.
	s := newRig(t)
	s.src.hw = true
	sv := s.join()
	eventually(t, 3*time.Second, "software encoder reason", func() bool {
		return strings.Contains(s.env.mgr.Status().Reason, "software encoder")
	})
	firstUnitIsKeyframe(t, s, sv, "viewer on a software encoder")
}

func TestPower_WallClockOnly(t *testing.T) {
	// A two hour jump (a sleep) is acted on at the next tick: the stalled
	// stream is nudged at once, with no timer waiting on a monotonic clock.
	r := newRig(t)
	r.join()
	r.w.clock.Advance(2 * time.Hour)
	r.untilNudges(1)

	// A wake that finds the display asleep gives up after 30 s of wall clock,
	// however it elapsed: the jump counts, no restart happens.
	wake := make(chan wakewatch.Reason, 1)
	w := newRig(t, withWake(wake))
	w.join()
	w.w.set(func() { w.w.asleep = true })
	wake <- wakewatch.Woke
	w.settle(time.Second)
	w.settle(2 * time.Hour)
	w.w.set(func() { w.w.asleep = false })
	time.Sleep(100 * time.Millisecond)
	if w.src.count() != 1 {
		t.Fatalf("a wake that waited out 30 s of wall clock still restarted (%d sources)", w.src.count())
	}
}
