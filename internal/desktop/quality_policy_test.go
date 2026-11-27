package desktop

import (
	"context"
	"encoding/json"
	"net/netip"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/cc"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/scottkw/portty/internal/screencap"
)

// pClock is a clock the test steps by hand.
type pClock struct {
	mu sync.Mutex
	t  time.Time
}

func newPClock() *pClock { return &pClock{t: time.Unix(1_700_000_000, 0)} }

func (c *pClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *pClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

// walk runs decide once per step of the clock with a fixed minimum estimate.
func walk(s ladderState, clk *pClock, lowest int, total, step time.Duration) ladderState {
	for elapsed := time.Duration(0); elapsed < total; elapsed += step {
		s = decide(policyInput{Now: clk.Now(), Min: lowest, Prev: s}).Next
		clk.Advance(step)
	}
	return s
}

// A fall under 85 percent of the target is followed at once; a smaller one is
// not; the target never leaves the floor and the ceiling.
func TestQuality_DecideFollowsDownAtOnce(t *testing.T) {
	clk := newPClock()
	s := ladderState{Target: ceilingBps}
	for _, c := range []struct{ lowest, want int }{
		{7_000_000, ceilingBps}, // 87.5 percent: inside the band
		{6_000_000, 6_000_000},  // under 85 percent of 8 Mbps
		{5_200_000, 6_000_000},  // 86.7 percent of 6 Mbps: inside the band
		{100_000, floorBps},     // under the floor: clamped
		{50_000_000, floorBps},  // a high minimum alone never raises at once
	} {
		s = decide(policyInput{Now: clk.Now(), Min: c.lowest, Prev: s}).Next
		if s.Target != c.want {
			t.Fatalf("minimum %d gives target %d, want %d", c.lowest, s.Target, c.want)
		}
		clk.Advance(250 * time.Millisecond)
	}
}

// A rise waits for the minimum to hold over 120 percent of the target for 5 s,
// then goes up about 10 percent per 2 s, never past the minimum or the ceiling.
func TestQuality_DecideRaiseNeedsHoldThenClimbs(t *testing.T) {
	clk := newPClock()
	s := ladderState{Target: 2_000_000}
	s = walk(s, clk, 8_000_000, 4900*time.Millisecond, 100*time.Millisecond)
	if s.Target != 2_000_000 {
		t.Fatalf("target %d after 4.9 s above 120 percent, want no raise yet", s.Target)
	}
	s = walk(s, clk, 8_000_000, 200*time.Millisecond, 100*time.Millisecond)
	if s.Target != 2_200_000 {
		t.Fatalf("target %d after 5 s, want the first raise to 2200000", s.Target)
	}
	s = walk(s, clk, 8_000_000, 1900*time.Millisecond, 100*time.Millisecond)
	if s.Target != 2_200_000 {
		t.Fatalf("target %d 1.9 s after a raise, want one raise per 2 s", s.Target)
	}
	s = walk(s, clk, 8_000_000, 200*time.Millisecond, 100*time.Millisecond)
	if s.Target != 2_420_000 {
		t.Fatalf("target %d 2 s after a raise, want 2420000", s.Target)
	}
	s = walk(s, clk, 8_000_000, 120*time.Second, 250*time.Millisecond)
	if s.Target != ceilingBps {
		t.Fatalf("target %d after two minutes of room, want the ceiling", s.Target)
	}
	// Room that is only just over the target does not raise it at all.
	low := ladderState{Target: 2_000_000}
	low = walk(low, newPClock(), 2_300_000, 30*time.Second, 250*time.Millisecond)
	if low.Target != 2_000_000 {
		t.Fatalf("target %d with a minimum under 120 percent of it, want it held", low.Target)
	}
}

// Ladder: under 2 Mbps for 2 s the frame rate halves, then the picture shrinks in
// two steps, one step per 10 s at most; recovery runs in reverse, each step after
// a 10 s hold over 2.4 Mbps.
func TestQuality_DecideLadderDownThenBackUp(t *testing.T) {
	clk := newPClock()
	s := ladderState{Target: ceilingBps}
	step := 250 * time.Millisecond

	s = walk(s, clk, 1_000_000, 1900*time.Millisecond, step)
	if s.Level != 0 {
		t.Fatalf("level %d after 1.9 s under 2 Mbps, want 0", s.Level)
	}
	s = walk(s, clk, 1_000_000, 250*time.Millisecond, step)
	if s.Level != 1 {
		t.Fatalf("level %d after 2 s under 2 Mbps, want 1 (frame rate halves)", s.Level)
	}
	s = walk(s, clk, 1_000_000, 9*time.Second, step)
	if s.Level != 1 {
		t.Fatalf("level %d 9 s after a step, want the step held for 10 s", s.Level)
	}
	s = walk(s, clk, 1_000_000, 1500*time.Millisecond, step)
	if s.Level != 2 {
		t.Fatalf("level %d 10 s after a step, want 2 (1920)", s.Level)
	}
	s = walk(s, clk, 1_000_000, 30*time.Second, step)
	if s.Level != maxLevel {
		t.Fatalf("level %d after a long poor spell, want %d (1280)", s.Level, maxLevel)
	}
	if got := fpsFor(s.Level, 30); got != 15 {
		t.Errorf("frame rate %d at the bottom, want 15", got)
	}
	if got := ladderEdges[resStep(s.Level)]; got != 1280 {
		t.Errorf("long edge %d at the bottom, want 1280", got)
	}

	// Room again: each step back needs the minimum over 2.4 Mbps for 10 s.
	for _, level := range []int{2, 1, 0} {
		s = walk(s, clk, 8_000_000, 11*time.Second, step)
		if s.Level != level {
			t.Fatalf("level %d after 11 more seconds of room, want %d", s.Level, level)
		}
	}
	if fpsFor(s.Level, 30) != 30 || ladderEdges[resStep(s.Level)] != 2560 {
		t.Fatalf("not back at full quality: %+v", s)
	}
}

// A minimum that flaps never holds long enough to step; one that holds poor
// long enough steps at most once per 10 s however it moves.
func TestQuality_DecideFlappingMakesNoStormOfSteps(t *testing.T) {
	clk := newPClock()
	s := ladderState{Target: ceilingBps}
	// One second poor, one second fine: the 2 s hold never completes.
	for i := 0; i < 120; i++ {
		est := 1_000_000
		if i%2 == 1 {
			est = 8_000_000
		}
		s = walk(s, clk, est, time.Second, 250*time.Millisecond)
	}
	if s.Level != 0 {
		t.Fatalf("level %d after two minutes of flapping, want 0", s.Level)
	}
	// Poor for 2.5 s, fine for 2.5 s: steps may happen but never twice in 10 s.
	var stepsAt []time.Duration
	start, last := clk.Now(), s.Level
	for i := 0; i < 240; i++ {
		est := 1_000_000
		if (i/10)%2 == 1 {
			est = 8_000_000
		}
		s = walk(s, clk, est, 250*time.Millisecond, 250*time.Millisecond)
		if s.Level != last {
			stepsAt = append(stepsAt, clk.Now().Sub(start))
			last = s.Level
		}
	}
	for i := 1; i < len(stepsAt); i++ {
		if gap := stepsAt[i] - stepsAt[i-1]; gap < stepHold {
			t.Fatalf("steps %v apart, want at least %v", gap, stepHold)
		}
	}
}

// A viewer below the floor for 3 s enters the slow set once; above it for 5 s it
// leaves once; the time between restarts the clock.
func TestQuality_SlowSetEnterAndLeave(t *testing.T) {
	clk := newPClock()
	var s slowState
	run := func(bad, good bool, d time.Duration) (changes int) {
		for e := time.Duration(0); e < d; e += 250 * time.Millisecond {
			var ch bool
			if s, ch = stepSlow(s, clk.Now(), bad, good); ch {
				changes++
			}
			clk.Advance(250 * time.Millisecond)
		}
		return changes
	}
	if n := run(true, false, 2750*time.Millisecond); n != 0 || s.Slow {
		t.Fatalf("slow after 2.75 s bad (changes %d), want not yet", n)
	}
	if n := run(false, false, time.Second); n != 0 {
		t.Fatalf("a neither-bad-nor-good second changed the set (%d)", n)
	}
	if n := run(true, false, 2750*time.Millisecond); n != 0 || s.Slow {
		t.Fatalf("slow although the clock restarted (changes %d)", n)
	}
	if n := run(true, false, 500*time.Millisecond); n != 1 || !s.Slow {
		t.Fatalf("not slow after 3 s bad (changes %d)", n)
	}
	if n := run(true, false, 10*time.Second); n != 0 || !s.Slow {
		t.Fatalf("a slow viewer that stays bad changed state (%d)", n)
	}
	if n := run(false, true, 4750*time.Millisecond); n != 0 || !s.Slow {
		t.Fatalf("cleared after 4.75 s good (changes %d), want 5 s", n)
	}
	if n := run(false, true, 500*time.Millisecond); n != 1 || s.Slow {
		t.Fatalf("not cleared after 5 s good (changes %d)", n)
	}
}

// Sizes the ladder makes: both sides scaled together, even, never above native.
func TestQuality_LadderSizes(t *testing.T) {
	for _, c := range []struct{ w, h, edge, ww, wh int }{
		{2560, 1600, 1920, 1920, 1200},
		{2560, 1600, 1280, 1280, 800},
		{2560, 1440, 1920, 1920, 1080},
		{1600, 1000, 1920, 1600, 1000}, // smaller than the step: left alone
		{1000, 1666, 1280, 768, 1280},  // portrait: the long edge is the height
	} {
		if w, h := ladderSize(c.w, c.h, c.edge); w != c.ww || h != c.wh {
			t.Errorf("ladderSize(%d, %d, %d) = %dx%d, want %dx%d", c.w, c.h, c.edge, w, h, c.ww, c.wh)
		}
	}
}

// qSrc is a source that records what the controller tells it.
type qSrc struct {
	mu         sync.Mutex
	bitrates   []int
	fps        []int
	rebuilds   [][2]int
	forced     int
	rebuildErr error
}

func (s *qSrc) Start(context.Context, screencap.Config) error { return nil }
func (s *qSrc) Stop() error                                   { return nil }
func (s *qSrc) Frames() <-chan screencap.AccessUnit           { return nil }
func (s *qSrc) Errors() <-chan error                          { return nil }
func (s *qSrc) SelectSurface(screencap.Surface) error         { return nil }
func (s *qSrc) ForceKeyframe() {
	s.mu.Lock()
	s.forced++
	s.mu.Unlock()
}
func (s *qSrc) Rebuild(w, h int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.rebuildErr != nil {
		return s.rebuildErr
	}
	s.rebuilds = append(s.rebuilds, [2]int{w, h})
	return nil
}
func (s *qSrc) SetBitrate(bps int) error {
	s.mu.Lock()
	s.bitrates = append(s.bitrates, bps)
	s.mu.Unlock()
	return nil
}
func (s *qSrc) SetFPS(fps int) error {
	s.mu.Lock()
	s.fps = append(s.fps, fps)
	s.mu.Unlock()
	return nil
}

func (s *qSrc) forcedCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.forced
}

func (s *qSrc) rebuilt() [][2]int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([][2]int(nil), s.rebuilds...)
}

func (s *qSrc) frameRates() []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]int(nil), s.fps...)
}

// qTest is a Manager that is never started, with its controller running on a
// recording source and a clock the test steps by hand; ticks are run by hand too.
type qTest struct {
	m     *Manager
	q     *qualityController
	src   *qSrc
	clk   *pClock
	mu    sync.Mutex
	slow  map[ViewerID][]bool
	stubs map[ViewerID]int
}

func newQTest(t *testing.T) *qTest {
	t.Helper()
	clk := newPClock()
	d := testDeps(netip.MustParseAddr("127.0.0.1"), 1)
	d.Now = clk.Now
	m, err := NewManager(d)
	if err != nil {
		t.Fatal(err)
	}
	qt := &qTest{m: m, q: m.q, src: &qSrc{}, clk: clk, slow: map[ViewerID][]bool{}, stubs: map[ViewerID]int{}}
	qt.q.told = func(id ViewerID, slow bool) {
		qt.mu.Lock()
		qt.slow[id] = append(qt.slow[id], slow)
		qt.mu.Unlock()
	}
	qt.q.mu.Lock()
	qt.q.src, qt.q.state, qt.q.fullFPS, qt.q.appliedFPS = qt.src, ladderState{Target: ceilingBps}, 30, 30
	qt.q.mu.Unlock()
	m.mu.Lock()
	m.src = qt.src // requestKeyframe reaches the recording source
	m.mu.Unlock()
	return qt
}

// viewer registers a viewer whose estimate setEst changes.
func (qt *qTest) viewer(id ViewerID, est int) {
	qt.q.register(&viewer{Viewer: Viewer{ID: id}}, nil)
	qt.setEst(id, est)
	qt.q.mu.Lock()
	qt.q.viewers[id].stub = func() int {
		qt.mu.Lock()
		defer qt.mu.Unlock()
		return qt.stubs[id]
	}
	qt.q.mu.Unlock()
}

func (qt *qTest) setEst(id ViewerID, est int) {
	qt.mu.Lock()
	qt.stubs[id] = est
	qt.mu.Unlock()
}

// run ticks the controller every 250 ms for d.
func (qt *qTest) run(d time.Duration) {
	for e := time.Duration(0); e < d; e += 250 * time.Millisecond {
		qt.q.tick()
		qt.clk.Advance(250 * time.Millisecond)
	}
}

func (qt *qTest) told(id ViewerID) []bool {
	qt.mu.Lock()
	defer qt.mu.Unlock()
	return append([]bool(nil), qt.slow[id]...)
}

// Edge, DESK-07 concurrency: viewer B sits below the floor, so after 3 s it is
// left out of the minimum and told slow once, and the target is free to climb for
// A. After 5 s back above the floor B counts again and is told it is not slow.
func TestQuality_SlowViewerIsLeftOutOfTheMinimum(t *testing.T) {
	qt := newQTest(t)
	qt.viewer("a", ceilingBps)
	qt.viewer("b", ceilingBps)
	qt.run(time.Second)

	qt.setEst("b", floorBps)
	qt.run(2 * time.Second)
	if got := qt.m.QualityStats().TargetBps; got != floorBps {
		t.Fatalf("target %d while B is under the floor but still counted, want the floor", got)
	}
	if len(qt.told("b")) != 0 {
		t.Fatal("B was told slow before 3 s")
	}
	qt.run(2 * time.Second)
	if got := qt.told("b"); len(got) != 1 || !got[0] {
		t.Fatalf("B was told %v, want exactly one slow true", got)
	}
	if got := qt.m.QualityStats().Slow; got != 1 {
		t.Errorf("QualityStats.Slow %d, want 1", got)
	}
	if len(qt.told("a")) != 0 {
		t.Errorf("A was told %v, it is not slow", qt.told("a"))
	}
	// B is out of the minimum: the target climbs again, bounded by A.
	qt.run(60 * time.Second)
	if got := qt.m.QualityStats().TargetBps; got <= 4_000_000 {
		t.Fatalf("target %d a minute after B left the minimum, want it climbing past 4 Mbps", got)
	}

	qt.setEst("b", 3_000_000)
	qt.run(4 * time.Second)
	if len(qt.told("b")) != 1 {
		t.Fatalf("B was told %v before 5 s above the floor", qt.told("b"))
	}
	qt.run(2 * time.Second)
	if got := qt.told("b"); len(got) != 2 || got[1] {
		t.Fatalf("B was told %v, want slow true then slow false", got)
	}
	if got := qt.m.QualityStats().Slow; got != 0 {
		t.Errorf("QualityStats.Slow %d after B recovered, want 0", got)
	}
}

// A lone viewer that is slow is still the only one there is: it gets the floor,
// not the ceiling.
func TestQuality_LoneSlowViewerStillGetsTheFloor(t *testing.T) {
	qt := newQTest(t)
	qt.viewer("b", floorBps)
	qt.run(6 * time.Second)
	if got := qt.m.QualityStats(); got.Slow != 1 || got.TargetBps != floorBps {
		t.Fatalf("stats %+v, want one slow viewer and the floor as target", got)
	}
}

// fakeEst is what GCC reports for a viewer that never sends feedback: its initial
// rate, forever.
type fakeEst struct{ rate int }

func (f fakeEst) AddStream(_ *interceptor.StreamInfo, w interceptor.RTPWriter) interceptor.RTPWriter {
	return w
}
func (f fakeEst) WriteRTCP([]rtcp.Packet, interceptor.Attributes) error { return nil }
func (f fakeEst) GetTargetBitrate() int                                 { return f.rate }
func (f fakeEst) OnTargetBitrateChange(func(int))                       {}
func (f fakeEst) GetStats() map[string]any                              { return nil }
func (f fakeEst) Close() error                                          { return nil }

var _ cc.BandwidthEstimator = fakeEst{}

// sendOne pushes one packet through bwe's stream writer, which starts its
// no-feedback clock.
func sendOne(t *testing.T, bwe *viewerBWE) {
	t.Helper()
	w := bwe.AddStream(&interceptor.StreamInfo{}, interceptor.RTPWriterFunc(func(*rtp.Header, []byte, interceptor.Attributes) (int, error) { return 0, nil }))
	if _, err := w.Write(&rtp.Header{}, nil, nil); err != nil {
		t.Fatal(err)
	}
}

// A viewer that sends no transport-cc feedback within 5 s of its first packet
// counts as the ceiling and is counted as no_feedback; its receiver reports can
// still make it slow (more than 20 percent loss for 3 s) and clear it again.
func TestQuality_NoFeedbackViewerCountsAsCeiling(t *testing.T) {
	qt := newQTest(t)
	qt.viewer("a", ceilingBps)
	bwe := newViewerBWE(fakeEst{rate: 900_000}, qt.clk.Now)
	qt.q.register(&viewer{Viewer: Viewer{ID: "n"}}, bwe)

	// Before its first packet there is nothing to say about feedback: its estimate counts.
	qt.run(time.Second)
	if got := qt.m.QualityStats().TargetBps; got != 900_000 {
		t.Fatalf("target %d with a viewer estimated at 900000 and no packet yet, want 900000", got)
	}
	sendOne(t, bwe)
	qt.run(4 * time.Second)
	if got := qt.m.QualityStats(); got.NoFeedback != 0 {
		t.Fatalf("no_feedback %d at 4 s after the first packet, want 0 (the limit is 5 s)", got.NoFeedback)
	}
	qt.run(2 * time.Second)
	if got := qt.m.QualityStats().NoFeedback; got != 1 {
		t.Fatalf("no_feedback %d at 6 s, want 1", got)
	}

	// Feedback arriving later clears it.
	if err := bwe.WriteRTCP([]rtcp.Packet{&rtcp.TransportLayerCC{}}, nil); err != nil {
		t.Fatal(err)
	}
	qt.run(time.Second)
	if got := qt.m.QualityStats().NoFeedback; got != 0 {
		t.Fatalf("no_feedback %d after feedback came, want 0", got)
	}

	// A viewer whose clock is its own: no feedback, then lossy, then not.
	bwe2 := newViewerBWE(fakeEst{rate: 900_000}, qt.clk.Now)
	qt.q.register(&viewer{Viewer: Viewer{ID: "m"}}, bwe2)
	sendOne(t, bwe2)
	qt.run(6 * time.Second)
	if got := qt.m.QualityStats().NoFeedback; got != 1 {
		t.Fatalf("no_feedback %d for the second viewer, want 1", got)
	}
	if len(qt.told("m")) != 0 {
		t.Fatal("a no-feedback viewer with no loss reports was told slow")
	}
	qt.q.noteLoss("m", 0.30)
	qt.run(2750 * time.Millisecond)
	if len(qt.told("m")) != 0 {
		t.Fatal("told slow before 3 s of loss")
	}
	qt.run(500 * time.Millisecond)
	if got := qt.told("m"); len(got) != 1 || !got[0] {
		t.Fatalf("told %v after 3 s of 30 percent loss, want slow true", got)
	}
	qt.q.noteLoss("m", 0.05)
	qt.run(5500 * time.Millisecond)
	if got := qt.told("m"); len(got) != 2 || got[1] {
		t.Fatalf("told %v after loss ended, want slow true then slow false", got)
	}
}

// PLI and FIR (T-23-06): any viewer's request in its first 3 s counts; later a
// slow viewer's is ignored and the others' are coalesced to one keyframe per 2 s.
func TestQuality_KeyframeRequestsAreCoalescedAndSlowOnesIgnored(t *testing.T) {
	qt := newQTest(t)
	qt.viewer("a", ceilingBps)
	qt.viewer("b", ceilingBps)

	// Inside the join window both count (the Phase 19 coalescer still allows one per second).
	qt.q.onKeyframeRequest("a")
	if got := qt.src.forcedCount(); got != 1 {
		t.Fatalf("%d keyframes for a join-window request, want 1", got)
	}
	qt.clk.Advance(1100 * time.Millisecond)
	qt.q.onKeyframeRequest("b")
	if got := qt.src.forcedCount(); got != 2 {
		t.Fatalf("%d keyframes after a second join-window request, want 2", got)
	}

	// B goes slow; both viewers are now past their join window.
	qt.setEst("b", floorBps)
	qt.run(4 * time.Second)
	if qt.m.QualityStats().Slow != 1 {
		t.Fatal("B did not become slow")
	}
	before := qt.src.forcedCount()
	for i := 0; i < 20; i++ {
		qt.q.onKeyframeRequest("b")
		qt.clk.Advance(100 * time.Millisecond)
	}
	if got := qt.src.forcedCount(); got != before {
		t.Fatalf("a slow viewer's PLIs forced %d keyframes, want 0", got-before)
	}

	// A's requests: the first goes through, the storm after it waits for the window.
	qt.q.onKeyframeRequest("a")
	if got := qt.src.forcedCount(); got != before+1 {
		t.Fatalf("A's first request forced %d keyframes, want 1", got-before)
	}
	for i := 0; i < 15; i++ {
		qt.q.onKeyframeRequest("a")
		qt.clk.Advance(50 * time.Millisecond)
	}
	if got := qt.src.forcedCount(); got != before+1 {
		t.Fatalf("a PLI storm inside 2 s forced %d keyframes, want 1 (the rest coalesced)", got-before)
	}
	qt.clk.Advance(2 * time.Second)
	qt.q.onKeyframeRequest("a")
	if got := qt.src.forcedCount(); got != before+2 {
		t.Fatalf("a request after the window forced %d keyframes, want 2 in all", got-before)
	}
}

// A size step goes through Rebuild at the ladder's size, one at a time, and a
// surface change (a size the ladder does not make) re-applies the current step.
func TestQuality_ResolutionStepsAndSurfaceChange(t *testing.T) {
	qt := newQTest(t)
	qt.viewer("a", 1_000_000)
	qt.m.noteSize(2560, 1600)

	qt.run(2500 * time.Millisecond) // level 1: frame rate only
	if got := qt.src.frameRates(); len(got) != 1 || got[0] != 15 {
		t.Fatalf("frame rates %v, want one change to 15", got)
	}
	if got := qt.src.rebuilt(); len(got) != 0 {
		t.Fatalf("rebuilds %v at level 1, want none", got)
	}
	qt.run(10 * time.Second) // level 2: 1920
	if got := qt.src.rebuilt(); len(got) != 1 || got[0] != [2]int{1920, 1200} {
		t.Fatalf("rebuilds %v, want one at 1920x1200", got)
	}
	qt.m.noteSize(1920, 1200) // the picture follows
	qt.run(11 * time.Second)  // level 3: 1280
	if got := qt.src.rebuilt(); len(got) != 2 || got[1] != [2]int{1280, 800} {
		t.Fatalf("rebuilds %v, want a second at 1280x800", got)
	}
	qt.m.noteSize(1280, 800)

	// The sharer switches to a 4:3 display: the capture is native again.
	qt.m.noteSize(2560, 1920)
	qt.run(time.Second)
	if got := qt.src.rebuilt(); len(got) != 3 || got[2] != [2]int{1280, 960} {
		t.Fatalf("rebuilds %v after a surface change, want the current step re-applied at 1280x960", got)
	}
}

// A failed size change is not retried for 10 s (it would cost everyone a keyframe
// each time), and the applied state is only advanced by a success.
func TestQuality_FailedResizeIsRetriedLater(t *testing.T) {
	qt := newQTest(t)
	qt.viewer("a", 1_000_000)
	qt.m.noteSize(2560, 1600)
	qt.src.mu.Lock()
	qt.src.rebuildErr = screencap.ErrNotRunning
	qt.src.mu.Unlock()
	qt.run(14 * time.Second) // level 2 is reached at about 12.5 s; the size change fails
	if got := qt.src.rebuilt(); len(got) != 0 {
		t.Fatalf("rebuilds %v while the source refuses them", got)
	}
	qt.src.mu.Lock()
	qt.src.rebuildErr = nil
	qt.src.mu.Unlock()
	qt.run(5 * time.Second) // 19 s: inside the 10 s wait after the failure
	if got := qt.src.rebuilt(); len(got) != 0 {
		t.Fatalf("rebuilds %v inside the retry wait, want none", got)
	}
	qt.run(6 * time.Second)
	got := qt.src.rebuilt()
	if len(got) == 0 {
		t.Fatal("the failed size change was never retried")
	}
	// The retry applies whatever step the ladder is at by then.
	w, h := ladderSize(2560, 1600, qt.m.QualityStats().LongEdge)
	if got[len(got)-1] != [2]int{w, h} {
		t.Fatalf("last rebuild %v, want the current step's %dx%d", got[len(got)-1], w, h)
	}
}

// TestQuality_GoldenLines: the slow message of the v1 wire. Both golden lines
// decode into msgQuality and marshal back equal (slow false is sent, not omitted),
// and ProtocolVersion stays 1 (the message is additive).
func TestQuality_GoldenLines(t *testing.T) {
	if ProtocolVersion != 1 {
		t.Fatalf("ProtocolVersion is %d, want 1", ProtocolVersion)
	}
	want := map[string]bool{"quality-slow-true": true, "quality-slow-false": false}
	seen := map[string]bool{}
	for _, l := range readGolden(t, "testdata/signal-v1.jsonl") {
		slow, ok := want[l.Name]
		if !ok {
			continue
		}
		var msg msgQuality
		if err := json.Unmarshal(l.Wire, &msg); err != nil || msg.Type != typeQuality || msg.Slow != slow {
			t.Errorf("%s decoded as %+v, %v", l.Name, msg, err)
		}
		back, err := json.Marshal(msg)
		if err != nil {
			t.Fatal(err)
		}
		var a, b any
		if json.Unmarshal(back, &a) != nil || json.Unmarshal(l.Wire, &b) != nil || !reflect.DeepEqual(a, b) {
			t.Errorf("%s marshals back as %s, want %s", l.Name, back, l.Wire)
		}
		seen[l.Name] = true
	}
	if len(seen) != 2 {
		t.Errorf("golden quality lines seen: %v, want both", seen)
	}
}
