package desktop

// quality.go: the shared encoder follows the network (DESK-07). Each viewer's
// PeerConnection carries a Google Congestion Control estimate built from its
// transport-cc feedback; the controller turns the estimates into one target
// bitrate for the one shared encoder and applies it to the running encoder.

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/cc"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/scottkw/portty/internal/screencap"
)

// Policy constants. They are tuning points, not settings: there is no
// user-facing quality control.
const (
	ceilingBps = 8_000_000 // the most the encoder is ever asked for; also where every estimate starts
	floorBps   = 500_000   // the least; a viewer that cannot hold it is slow
	downRatio  = 0.85      // lower the target when the minimum estimate falls under this share of it

	qualityTick = 250 * time.Millisecond // controller cadence; decisions compare Deps.Now
)

// bweSlot hands the estimator pion builds inside NewPeerConnection to the answer
// that is building that PeerConnection (newPCWithBWE holds mu around the build).
type bweSlot struct {
	mu  sync.Mutex
	cur *viewerBWE
}

// offer is the cc interceptor's OnNewPeerConnection callback. It runs inside
// NewPeerConnection, under the mutex newPCWithBWE holds, so it takes no lock.
func (s *bweSlot) offer(_ string, e cc.BandwidthEstimator) {
	if b, ok := e.(*viewerBWE); ok {
		s.cur = b
	}
}

// viewerBWE is one viewer's estimator. It wraps pion's GCC to learn whether the
// viewer ever sends transport-cc feedback: with none, GCC's estimate never moves
// and must not be mistaken for a measurement (RESEARCH Pitfall 2).
type viewerBWE struct {
	cc.BandwidthEstimator
	now       func() time.Time
	firstSent atomic.Int64 // unix nanos of the first outgoing packet; 0 before it
	feedback  atomic.Bool  // a transport-cc report has come back
}

func newViewerBWE(e cc.BandwidthEstimator, now func() time.Time) *viewerBWE {
	return &viewerBWE{BandwidthEstimator: e, now: now}
}

// AddStream notes when the first packet leaves, then lets pion do its work.
func (b *viewerBWE) AddStream(info *interceptor.StreamInfo, w interceptor.RTPWriter) interceptor.RTPWriter {
	inner := b.BandwidthEstimator.AddStream(info, w)
	return interceptor.RTPWriterFunc(func(h *rtp.Header, p []byte, a interceptor.Attributes) (int, error) {
		if b.firstSent.Load() == 0 {
			b.firstSent.CompareAndSwap(0, b.now().UnixNano())
		}
		return inner.Write(h, p, a)
	})
}

// WriteRTCP notes transport-cc feedback and passes everything on.
func (b *viewerBWE) WriteRTCP(pkts []rtcp.Packet, a interceptor.Attributes) error {
	for _, p := range pkts {
		if _, ok := p.(*rtcp.TransportLayerCC); ok {
			b.feedback.Store(true)
			break
		}
	}
	return b.BandwidthEstimator.WriteRTCP(pkts, a)
}

// ladderState is what the pure policy carries from one tick to the next.
type ladderState struct {
	Target int // the encoder's target bitrate in bits per second
}

type policyInput struct {
	Now  time.Time
	Min  int // the lowest estimate among the viewers that count
	Prev ladderState
}

type policyOutput struct{ Next ladderState }

// decide is the whole policy as a pure function of the estimates, the clock and
// the previous state, so it is tested with a fake clock and no pion or VideoToolbox.
func decide(in policyInput) policyOutput {
	m := min(max(in.Min, floorBps), ceilingBps)
	next := in.Prev
	if float64(m) < downRatio*float64(next.Target) {
		next.Target = m
	}
	return policyOutput{Next: next}
}

// QualityStats is what the controller currently asks of the encoder and how many
// viewers it is protecting the others from.
type QualityStats struct {
	TargetBps  int
	FPS        int
	LongEdge   int
	Step       int
	Slow       int
	NoFeedback int
}

// qViewer is the controller's record of one viewer.
type qViewer struct {
	ws     *websocket.Conn
	bwe    *viewerBWE
	stub   func() int // a test replaces the estimate
	joined time.Time
}

// estimate is the viewer's bandwidth estimate; a viewer with no feedback counts
// as the ceiling, so it never holds the others down.
func (v *qViewer) estimate(now time.Time) int {
	if v.stub != nil {
		return v.stub()
	}
	if fs := v.bwe.firstSent.Load(); fs != 0 && !v.bwe.feedback.Load() && now.Sub(time.Unix(0, fs)) >= 5*time.Second {
		return ceilingBps
	}
	return v.bwe.GetTargetBitrate()
}

// qualityController owns the policy state and applies it to the running source.
type qualityController struct {
	m         *Manager
	tickEvery time.Duration // tests shorten it before Start
	tickMu    sync.Mutex    // one tick at a time
	lastUnit  atomic.Int64  // unix nanos of the last access unit (23-03's still machine builds on it)

	mu         sync.Mutex // guards everything below
	viewers    map[ViewerID]*qViewer
	state      ladderState
	appliedBps int
	src        screencap.Source
	cancel     context.CancelFunc
	done       chan struct{}
}

func newQualityController(m *Manager) *qualityController {
	return &qualityController{m: m, tickEvery: qualityTick, viewers: make(map[ViewerID]*qViewer)}
}

// register starts counting viewer v's estimate.
func (q *qualityController) register(v *viewer, bwe *viewerBWE) {
	q.mu.Lock()
	q.viewers[v.ID] = &qViewer{ws: v.ws, bwe: bwe, joined: q.m.d.Now()}
	q.mu.Unlock()
}

func (q *qualityController) unregister(id ViewerID) {
	q.mu.Lock()
	delete(q.viewers, id)
	q.mu.Unlock()
}

// start begins a run on src, at the ceiling: the normal state is high until
// congestion says otherwise (GCC cannot raise an estimate above 1.5 times what
// was sent, so a low start would never recover on a still screen).
func (q *qualityController) start(ctx context.Context, src screencap.Source) {
	lctx, cancel := context.WithCancel(ctx)
	q.mu.Lock()
	q.src, q.state, q.appliedBps = src, ladderState{Target: ceilingBps}, 0
	q.cancel, q.done = cancel, make(chan struct{})
	done := q.done
	q.mu.Unlock()
	go q.loop(lctx, done)
}

// stop ends the run and waits for a tick in flight, which may be inside a setter
// on the source (the caller stops the source afterwards).
func (q *qualityController) stop() {
	q.mu.Lock()
	cancel, done := q.cancel, q.done
	q.cancel, q.done, q.src = nil, nil, nil
	q.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
}

func (q *qualityController) loop(ctx context.Context, done chan<- struct{}) {
	defer close(done)
	defer recoverLog()
	t := time.NewTicker(q.tickEvery)
	defer t.Stop()
	for {
		q.tick()
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// tick reads every counted viewer's estimate, runs the policy and applies a
// changed target to the encoder.
func (q *qualityController) tick() {
	q.tickMu.Lock()
	defer q.tickMu.Unlock()
	now := q.m.d.Now()
	q.mu.Lock()
	src := q.src
	if src == nil || len(q.viewers) == 0 {
		q.mu.Unlock()
		return
	}
	lowest := ceilingBps
	for _, v := range q.viewers {
		lowest = min(lowest, v.estimate(now))
	}
	q.state = decide(policyInput{Now: now, Min: lowest, Prev: q.state}).Next
	want, changed := q.state.Target, q.state.Target != q.appliedBps
	q.mu.Unlock()
	if !changed {
		return
	}
	if qs, ok := src.(screencap.QualitySource); ok {
		if err := qs.SetBitrate(want); err != nil {
			slog.Warn("desktop: bitrate change failed", "err", err)
			return
		}
	}
	q.mu.Lock()
	q.appliedBps = want
	q.mu.Unlock()
}

// noteUnit is the fan-out's per-unit hook. It records the unit's time and reports
// whether the unit counts as a captured frame for the Phase 19 stall watchdog.
func (m *Manager) noteUnit(screencap.AccessUnit) bool {
	m.q.lastUnit.Store(m.d.Now().UnixNano())
	return true
}

// QualityStats reports what the controller currently asks of the encoder.
func (m *Manager) QualityStats() QualityStats {
	m.q.mu.Lock()
	defer m.q.mu.Unlock()
	return QualityStats{TargetBps: m.q.state.Target}
}
