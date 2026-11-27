package desktop_test

import (
	"context"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/desktop"
	"github.com/scottkw/portty/internal/screencap"
)

// recSource is the synthetic source plus the live-quality setters, recording
// every call so a test sees what the controller asked of the encoder.
type recSource struct {
	*screencap.SyntheticSource

	mu       sync.Mutex
	bitrates []int
	fps      []int
	rebuilds [][2]int
}

func (s *recSource) SetBitrate(bps int) error {
	s.mu.Lock()
	s.bitrates = append(s.bitrates, bps)
	s.mu.Unlock()
	return nil
}

func (s *recSource) SetFPS(fps int) error {
	s.mu.Lock()
	s.fps = append(s.fps, fps)
	s.mu.Unlock()
	return nil
}

func (s *recSource) Rebuild(w, h int) error {
	s.mu.Lock()
	s.rebuilds = append(s.rebuilds, [2]int{w, h})
	s.mu.Unlock()
	return s.SyntheticSource.Rebuild(w, h)
}

func (s *recSource) lastBitrate() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.bitrates) == 0 {
		return 0
	}
	return s.bitrates[len(s.bitrates)-1]
}

func (s *recSource) sawBitrate(bps int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Contains(s.bitrates, bps)
}

// qualityRec collects the recording sources a Manager built.
type qualityRec struct {
	mu  sync.Mutex
	all []*recSource
}

func (r *qualityRec) last() *recSource {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.all) == 0 {
		return nil
	}
	return r.all[len(r.all)-1]
}

// withQualitySource makes the Manager's sources recording sources of the
// env's synthetic size.
func withQualitySource(r *qualityRec) tracerOpt {
	return func(env *tracerEnv, d *desktop.Deps) {
		d.NewSource = func() screencap.Source {
			s := &recSource{SyntheticSource: screencap.NewSyntheticSource(env.srcW, env.srcH)}
			r.mu.Lock()
			r.all = append(r.all, s)
			r.mu.Unlock()
			return s
		}
	}
}

// onlyViewerID is the ID of the one connected viewer.
func onlyViewerID(t *testing.T, m *desktop.Manager) desktop.ViewerID {
	t.Helper()
	list := m.ViewerList()
	if len(list) != 1 {
		t.Fatalf("ViewerList has %d viewers, want 1", len(list))
	}
	return list[0].ID
}

// TestQuality_TracerEstimateSetsBitrate: a viewer joins and the encoder is told
// 8 Mbps (start at the ceiling); when that viewer's estimate is 2 Mbps the
// encoder follows within a second; when the viewer leaves its estimator goes.
func TestQuality_TracerEstimateSetsBitrate(t *testing.T) {
	rec := &qualityRec{}
	env := newTracerServer(t, withQualitySource(rec))
	tok := env.token(t, "g1")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	v := env.join(ctx, t, tok)
	requireKeyframe(t, "viewer", v.firstAU(t, 5*time.Second))
	eventually(t, 2*time.Second, "the initial 8 Mbps", func() bool {
		s := rec.last()
		return s != nil && s.sawBitrate(8_000_000)
	})
	if n := env.mgr.QualityViewersForTest(); n != 1 {
		t.Fatalf("controller counts %d viewers, want 1", n)
	}

	env.mgr.SetEstimateForTest(onlyViewerID(t, env.mgr), func() int { return 2_000_000 })
	eventually(t, time.Second, "the encoder at or below 2 Mbps", func() bool {
		b := rec.last().lastBitrate()
		return b > 0 && b <= 2_000_000
	})
	if got := env.mgr.QualityStats().TargetBps; got != 2_000_000 {
		t.Errorf("QualityStats target %d, want 2000000", got)
	}

	_ = v.conn.CloseNow()
	eventually(t, 5*time.Second, "the estimator unregistered", func() bool { return env.mgr.QualityViewersForTest() == 0 })
}

// TestQuality_StepKeepsInputEpoch: a picture-size step is not a surface switch.
// The epoch viewers stamp positional input with is the same before and after the
// ladder rebuilds the encoder at a smaller size, and a move sent with that epoch
// during the step is posted, not dropped as stale.
func TestQuality_StepKeepsInputEpoch(t *testing.T) {
	fb := &fakeInputBridge{}
	var locked atomic.Bool
	clk := newFakeClock()
	rec := &qualityRec{}
	env := newTracerServer(t, withFakeInput(fb, &locked, clk), withQualitySource(rec), withSource(2560, 1600))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	v := env.joinInput(ctx, t, env.token(t, "g1"))
	eventually(t, 5*time.Second, "the 2560x1600 picture", func() bool { w, h := v.size(); return w == 2560 && h == 1600 })
	id := onlyViewerID(t, env.mgr)
	env.mgr.SetEstimateForTest(id, func() int { return 1_000_000 })

	epoch := env.mgr.EpochForTest()
	v.send(t, map[string]any{"t": "move", "x": 0.5, "y": 0.5, "e": epoch})
	fb.waitFor(t, 1)

	// Poor for long enough to reach the first size step: frame rate at 2 s, size at 12 s.
	// The clock moves in seconds with a pause for a frame, so the stall watchdog sees life.
	step := func(d time.Duration) {
		for e := time.Duration(0); e < d; e += time.Second {
			clk.Advance(time.Second)
			time.Sleep(60 * time.Millisecond)
			env.mgr.QualityTickForTest()
		}
	}
	step(14 * time.Second)
	eventually(t, 5*time.Second, "the encoder rebuilt at 1920x1200", func() bool {
		s := rec.last()
		s.mu.Lock()
		defer s.mu.Unlock()
		return slices.Contains(s.rebuilds, [2]int{1920, 1200})
	})
	eventually(t, 5*time.Second, "viewers told the new size", func() bool { w, h := v.size(); return w == 1920 && h == 1200 })

	if got := env.mgr.EpochForTest(); got != epoch {
		t.Fatalf("input epoch %d after a size step, want %d: a quality step is not a surface switch", got, epoch)
	}
	v.send(t, map[string]any{"t": "move", "x": 0.25, "y": 0.25, "e": epoch})
	fb.waitFor(t, 2)
	if n := counter(env, "stale_epoch"); n != 0 {
		t.Fatalf("%d moves dropped as stale across a size step, want 0", n)
	}
}

// TestQuality_EstimatorBinding: viewers answered at the same moment each get
// their own estimator (the construction mutex pairs every PeerConnection with
// the estimator pion built for it).
func TestQuality_EstimatorBinding(t *testing.T) {
	rec := &qualityRec{}
	env := newTracerServer(t, withQualitySource(rec))
	tok := env.token(t, "g1")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	const n = 4
	var wg sync.WaitGroup
	vs := make([]*fanViewer, n)
	for i := range vs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			vs[i] = env.join(ctx, t, tok)
		}()
	}
	wg.Wait()
	if t.Failed() {
		return
	}
	eventually(t, 5*time.Second, "every viewer counted", func() bool { return env.mgr.QualityViewersForTest() == n })
	if got := env.mgr.QualityDistinctEstimatorsForTest(); got != n {
		t.Fatalf("%d distinct estimators for %d viewers, want one each", got, n)
	}
}
