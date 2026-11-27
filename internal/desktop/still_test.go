package desktop

import (
	"sync"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/screencap"
)

// stillSrc is the recording source plus the still half: it notes when each
// refine and idle refresh was asked for and at what bitrate.
type stillSrc struct {
	*qSrc
	now func() time.Time

	smu     sync.Mutex
	refines []int
	refineT []time.Time
	idleT   []time.Time
}

func (s *stillSrc) Refine(bps int) error {
	s.smu.Lock()
	s.refines = append(s.refines, bps)
	s.refineT = append(s.refineT, s.now())
	s.smu.Unlock()
	return nil
}

func (s *stillSrc) IdleRefresh() error {
	s.smu.Lock()
	s.idleT = append(s.idleT, s.now())
	s.smu.Unlock()
	return nil
}

func (s *stillSrc) counts() (refines, idles int) {
	s.smu.Lock()
	defer s.smu.Unlock()
	return len(s.refines), len(s.idleT)
}

// newStillTest is a qTest whose source is a stillSrc and which has one viewer
// with its media path up.
func newStillTest(t *testing.T, est int) (*qTest, *stillSrc) {
	t.Helper()
	qt := newQTest(t)
	ss := &stillSrc{qSrc: qt.src, now: qt.clk.Now}
	qt.q.mu.Lock()
	qt.q.src = ss
	qt.q.mu.Unlock()
	qt.viewer("v1", est)
	qt.m.vmu.Lock()
	qt.m.viewers["v1"] = &viewer{Viewer: Viewer{ID: "v1"}, ready: true}
	qt.m.vmu.Unlock()
	return qt, ss
}

// moving feeds a fresh unit every 33 ms for d and ticks the controller each time.
func (qt *qTest) moving(d time.Duration) {
	for e := time.Duration(0); e < d; e += 33 * time.Millisecond {
		qt.m.noteUnit(screencap.AccessUnit{Kind: screencap.UnitFresh})
		qt.q.tick()
		qt.clk.Advance(33 * time.Millisecond)
	}
}

// The machine alone: one refine 300 ms after the last fresh unit, an idle refresh
// every 2 s after that, and a fresh unit starts it over.
func TestStill_MachineRefinesOnceThenIdles(t *testing.T) {
	clk := newPClock()
	var s stillMachine
	if a := s.step(clk.Now(), true, false); a != stillNone {
		t.Fatalf("before any capture the machine did %v", a)
	}
	s.fresh(clk.Now())
	clk.Advance(stillAfter - time.Millisecond)
	if a := s.step(clk.Now(), true, false); a != stillNone {
		t.Fatalf("just before stillAfter the machine did %v", a)
	}
	clk.Advance(time.Millisecond)
	if a := s.step(clk.Now(), true, false); a != stillRefine {
		t.Fatalf("at stillAfter the machine did %v, want refine", a)
	}
	clk.Advance(idleRefreshEvery - time.Millisecond)
	if a := s.step(clk.Now(), true, false); a != stillNone {
		t.Fatalf("just before the idle period the machine did %v", a)
	}
	clk.Advance(time.Millisecond)
	if a := s.step(clk.Now(), true, false); a != stillIdleRefresh {
		t.Fatalf("after the idle period the machine did %v, want idle refresh", a)
	}
	s.fresh(clk.Now())
	if s.still {
		t.Fatal("a fresh capture left the machine still")
	}
	clk.Advance(stillAfter)
	if a := s.step(clk.Now(), true, false); a != stillRefine {
		t.Fatalf("after new motion and stillness the machine did %v, want refine", a)
	}
}

// Frames stop: the source is asked to refine once, 300 to 550 ms later, at four
// times the target (below the ceiling), then to idle-refresh about every 2 s.
func TestStill_ControllerRefinesOnceThenIdleRefreshes(t *testing.T) {
	qt, ss := newStillTest(t, 1_000_000) // the one viewer's estimate is 1 Mbps: target 1 Mbps
	qt.moving(time.Second)
	if r, i := ss.counts(); r != 0 || i != 0 {
		t.Fatalf("while moving: %d refines, %d idle refreshes, want none", r, i)
	}
	last := qt.clk.Now().Add(-33 * time.Millisecond) // the last fresh unit
	qt.run(6500 * time.Millisecond)

	r, i := ss.counts()
	if r != 1 {
		t.Fatalf("%d refines over 6.5 s of stillness, want exactly 1", r)
	}
	if got, want := ss.refines[0], 4_000_000; got != want {
		t.Errorf("refine bitrate %d, want %d (4 times the 1 Mbps target)", got, want)
	}
	if after := ss.refineT[0].Sub(last); after < 300*time.Millisecond || after > 550*time.Millisecond {
		t.Errorf("refine came %v after the last fresh unit, want 300 to 550 ms", after)
	}
	if i < 2 || i > 4 {
		t.Errorf("%d idle refreshes over 6.5 s, want about one per 2 s (2 to 4)", i)
	}
	if !qt.m.QualityStats().Still {
		t.Error("QualityStats.Still is false while still")
	}
}

// The refine bitrate never exceeds the ceiling.
func TestStill_RefineBitrateIsBoundedByTheCeiling(t *testing.T) {
	if got := refineBps(ceilingBps); got != ceilingBps {
		t.Errorf("refineBps(ceiling) = %d, want the ceiling", got)
	}
	if got := refineBps(floorBps); got != refineFactor*floorBps {
		t.Errorf("refineBps(floor) = %d, want %d", got, refineFactor*floorBps)
	}
}

// Re-encodes of the cached frame are not captures: a refine or idle unit neither
// ends stillness nor counts as a frame, a forced (join) keyframe counts but does
// not end stillness, and the next fresh unit returns to moving with no refine.
func TestStill_CachedReencodesAreNotCaptures(t *testing.T) {
	qt, ss := newStillTest(t, ceilingBps)
	qt.moving(500 * time.Millisecond)
	qt.run(time.Second)
	if r, _ := ss.counts(); r != 1 {
		t.Fatalf("%d refines before the check, want 1", r)
	}
	for kind, wantCount := range map[screencap.UnitKind]bool{
		screencap.UnitRefine: false,
		screencap.UnitIdle:   false,
		screencap.UnitForced: true,
	} {
		if got := qt.m.noteUnit(screencap.AccessUnit{Kind: kind}); got != wantCount {
			t.Errorf("noteUnit(kind %d) counts as a frame = %v, want %v", kind, got, wantCount)
		}
		if !qt.m.QualityStats().Still {
			t.Errorf("a unit of kind %d ended stillness", kind)
		}
	}

	if !qt.m.noteUnit(screencap.AccessUnit{Kind: screencap.UnitFresh}) {
		t.Error("a fresh unit does not count as a frame")
	}
	qt.q.tick()
	if qt.m.QualityStats().Still {
		t.Error("a fresh unit did not return the machine to moving")
	}
	if r, _ := ss.counts(); r != 1 {
		t.Errorf("returning to moving made a refine (%d total), want still 1", r)
	}
}

// Neither refine nor idle refresh runs while the display is asleep or with no
// viewer whose media path is up; both resume when the condition clears.
func TestStill_NotWhileAsleepOrWithoutAReadyViewer(t *testing.T) {
	qt, ss := newStillTest(t, ceilingBps)
	qt.moving(300 * time.Millisecond)

	qt.m.smu.Lock()
	qt.m.asleep = true
	qt.m.smu.Unlock()
	qt.run(5 * time.Second)
	if r, i := ss.counts(); r != 0 || i != 0 {
		t.Fatalf("asleep: %d refines, %d idle refreshes, want none", r, i)
	}
	qt.m.smu.Lock()
	qt.m.asleep = false
	qt.m.smu.Unlock()

	qt.m.vmu.Lock()
	qt.m.viewers["v1"].ready = false
	qt.m.vmu.Unlock()
	qt.run(5 * time.Second)
	if r, i := ss.counts(); r != 0 || i != 0 {
		t.Fatalf("no ready viewer: %d refines, %d idle refreshes, want none", r, i)
	}

	qt.m.vmu.Lock()
	qt.m.viewers["v1"].ready = true
	qt.m.vmu.Unlock()
	qt.run(time.Second)
	if r, _ := ss.counts(); r != 1 {
		t.Fatalf("%d refines once the viewer is ready, want 1", r)
	}
}
