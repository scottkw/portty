package desktop

// quality.go: the shared encoder follows the network (DESK-07). Each viewer's
// PeerConnection carries a Google Congestion Control estimate built from its
// transport-cc feedback; the controller turns the estimates into one target
// bitrate, frame rate and picture size for the one shared encoder and applies
// them to the running source. A viewer that cannot hold the floor is left out of
// the minimum and told so, so one slow link never drags the others down.

import (
	"context"
	"log/slog"
	"math"
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
	ladderBps  = 2_000_000 // below this the ladder lowers the frame rate, then the size
	downRatio  = 0.85      // lower the target when the minimum estimate falls under this share of it
	upRatio    = 1.2       // raise it only while the minimum stays over this multiple of it
	raiseBy    = 1.10      // each raise is about 10 percent

	upHold     = 5 * time.Second  // the minimum must stay over upRatio this long before the first raise
	raiseEvery = 2 * time.Second  // then one raise per this
	lowHold    = 2 * time.Second  // a target under ladderBps this long steps the ladder down
	stepHold   = 10 * time.Second // at most one ladder step per this, either way

	slowEnter       = 3 * time.Second // below the floor this long: left out of the minimum and told slow
	slowLeave       = 5 * time.Second // above the floor this long: counted again and told not slow
	joinWindow      = 3 * time.Second // a viewer's PLI and FIR always count in its first seconds
	noFeedbackAfter = 5 * time.Second // no transport-cc feedback this long after the first packet
	lossLimit       = 0.20            // fraction lost that makes a no-feedback viewer slow
	pliEvery        = 2 * time.Second // keyframes asked for by PLI or FIR after the join window

	qualityTick = 250 * time.Millisecond // controller cadence; decisions compare Deps.Now

	maxLevel = 3 // ladder positions: 0 full, 1 half frame rate, 2 and 3 smaller picture
)

// ladderEdges are the long edges of the picture by size step. Ladder position 0
// is full quality and position 1 only halves the frame rate, so both are size
// step 0; positions 2 and 3 are steps 1 and 2.
var ladderEdges = [...]int{2560, 1920, 1280}

// resStep is the size step (the index into ladderEdges) of a ladder position.
func resStep(level int) int { return max(level-1, 0) }

// fpsFor is the frame rate at a ladder position: full at 0, half above.
func fpsFor(level, full int) int {
	if level == 0 {
		return full
	}
	return max(full/2, 1)
}

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
	sent      atomic.Int64 // bytes sent, headers included
	cut       atomic.Bool  // GCC was cutting at a report since the controller last looked
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
		b.sent.Add(int64(h.MarshalSize() + len(p)))
		return inner.Write(h, p, a)
	})
}

// WriteRTCP notes transport-cc feedback and passes everything on. A cut can be
// over between two controller ticks, so each report latches it.
func (b *viewerBWE) WriteRTCP(pkts []rtcp.Packet, a interceptor.Attributes) error {
	fb := false
	for _, p := range pkts {
		if _, ok := p.(*rtcp.TransportLayerCC); ok {
			b.feedback.Store(true)
			fb = true
			break
		}
	}
	err := b.BandwidthEstimator.WriteRTCP(pkts, a)
	if fb && b.gccCutting() {
		b.cut.Store(true)
	}
	return err
}

// gccCutting reports GCC's own congestion signal: its delay controller saw the
// queue grow and cut. pion writes its stats only on increase and decrease
// updates, so the state stays "decrease" through the hold after a cut.
func (b *viewerBWE) gccCutting() bool { return b.GetStats()["state"] == "decrease" }

// cutting reports whether GCC was cutting at any report since the last call, or
// is now. q.mu held (one reader).
func (b *viewerBWE) cutting() bool { return b.cut.Swap(false) || b.gccCutting() }

// noFeedback reports that the first packet left at least noFeedbackAfter ago and
// no feedback has come back.
func (b *viewerBWE) noFeedback(now time.Time) bool {
	fs := b.firstSent.Load()
	return fs != 0 && !b.feedback.Load() && now.Sub(time.Unix(0, fs)) >= noFeedbackAfter
}

// ladderState is what the pure policy carries from one tick to the next.
type ladderState struct {
	Target int // the encoder's target bitrate in bits per second
	Level  int // ladder position, 0 to maxLevel

	AboveSince time.Time // the minimum has been over upRatio times Target since then
	LastRaise  time.Time
	LowSince   time.Time // the target has been under ladderBps since then
	OKSince    time.Time // the target has been over upRatio times ladderBps since then
	LastStep   time.Time
}

type policyInput struct {
	Now  time.Time
	Min  int // the lowest estimate among the viewers that count
	Prev ladderState
}

type policyOutput struct{ Next ladderState }

// decide is the whole policy as a pure function of the estimates, the clock and
// the previous state, so it is tested with a fake clock and no pion or VideoToolbox.
//
// Bitrate: follow the minimum down at once when it falls under downRatio of the
// target; follow it up about 10 percent per raiseEvery, and only after it has
// stayed over upRatio of the target for upHold. Ladder: it fits the picture to
// the bitrate the encoder is asked for, the new target, not the minimum (which
// is the ceiling between GCC's cuts on a slow link). A target under ladderBps for
// lowHold lowers the frame rate, then the picture size in two steps; a target
// over upRatio times ladderBps for stepHold restores them in reverse. Any two
// steps are at least stepHold apart, so a flapping estimate cannot cause a storm
// of keyframes (a size step makes one).
func decide(in policyInput) policyOutput {
	s, now := in.Prev, in.Now
	m := min(max(in.Min, floorBps), ceilingBps)

	// An estimate at the ceiling is GCC saying "as much as you like" (it is capped
	// there), so it counts as room even when the target is within 120 percent of
	// it: otherwise the target would settle near ceiling/1.2 and never arrive.
	room := float64(m) > upRatio*float64(s.Target) || (m == ceilingBps && s.Target < ceilingBps)
	switch {
	case float64(m) < downRatio*float64(s.Target):
		s.Target, s.AboveSince = m, time.Time{}
	case room:
		if s.AboveSince.IsZero() {
			s.AboveSince = now
		}
		if now.Sub(s.AboveSince) >= upHold && now.Sub(s.LastRaise) >= raiseEvery {
			s.Target = min(int(float64(s.Target)*raiseBy), m)
			s.LastRaise = now
		}
	default:
		s.AboveSince = time.Time{}
	}

	s.LowSince = sinceWhile(s.LowSince, now, s.Target < ladderBps)
	s.OKSince = sinceWhile(s.OKSince, now, float64(s.Target) >= upRatio*ladderBps)
	stepOK := s.LastStep.IsZero() || now.Sub(s.LastStep) >= stepHold
	switch {
	case stepOK && s.Level < maxLevel && !s.LowSince.IsZero() && now.Sub(s.LowSince) >= lowHold:
		s.Level++
		s.LastStep, s.LowSince, s.OKSince = now, now, time.Time{}
	case stepOK && s.Level > 0 && !s.OKSince.IsZero() && now.Sub(s.OKSince) >= stepHold:
		s.Level--
		s.LastStep, s.OKSince, s.LowSince = now, now, time.Time{}
	}
	return policyOutput{Next: s}
}

// sinceWhile keeps the time a condition began: it starts at now when the
// condition becomes true and is cleared when it stops.
func sinceWhile(since, now time.Time, cond bool) time.Time {
	switch {
	case !cond:
		return time.Time{}
	case since.IsZero():
		return now
	}
	return since
}

// slowState is one viewer's place in the slow set. Since is when the condition
// that would change it began.
type slowState struct {
	Slow  bool
	Since time.Time
}

// stepSlow moves a viewer in or out of the slow set. A viewer that is bad for
// slowEnter becomes slow; a slow one that is good for slowLeave is counted again.
// bad and good are not opposites: between them nothing changes and the clock
// restarts.
func stepSlow(s slowState, now time.Time, bad, good bool) (slowState, bool) {
	if s.Slow {
		s.Since = sinceWhile(s.Since, now, good)
		if !s.Since.IsZero() && now.Sub(s.Since) >= slowLeave {
			return slowState{}, true
		}
		return s, false
	}
	s.Since = sinceWhile(s.Since, now, bad)
	if !s.Since.IsZero() && now.Sub(s.Since) >= slowEnter {
		return slowState{Slow: true}, true
	}
	return s, false
}

// ladderSize is the picture size at long edge edge for a native size: both sides
// scaled together, each rounded down to even, never above native.
func ladderSize(nativeW, nativeH, edge int) (w, h int) {
	long := max(nativeW, nativeH)
	if edge >= long {
		return nativeW, nativeH
	}
	f := float64(edge) / float64(long)
	return int(math.Round(float64(nativeW)*f)) &^ 1, int(math.Round(float64(nativeH)*f)) &^ 1
}

// typeQuality tells one viewer it is (or no longer is) left out of the quality
// decision because its link is too slow to hold the floor.
const typeQuality = "quality"

type msgQuality struct {
	Type string `json:"type"`
	Slow bool   `json:"slow"`
}

// QualityStats is what the controller currently asks of the encoder and how many
// viewers are in the slow set or have sent no feedback. LongEdge is the ladder
// position's nominal long edge (a smaller surface is never scaled up).
type QualityStats struct {
	TargetBps  int  `json:"target_bps"`
	FPS        int  `json:"fps"`
	LongEdge   int  `json:"long_edge"`
	Step       int  `json:"step"`
	Slow       int  `json:"slow"`
	NoFeedback int  `json:"no_feedback"`
	Still      bool `json:"-"` // the screen is still: the picture was refined and is being kept alive (Stats carries it)
}

// qViewer is the controller's record of one viewer.
type qViewer struct {
	ws     *websocket.Conn
	bwe    *viewerBWE
	stub   func() int // a test replaces the estimate
	joined time.Time
	slow   slowState
	loss   float64 // the last receiver-report fraction lost, 0 to 1
	noFB   bool    // no feedback as of the last tick

	// sentBps is what the viewer was sent over the last whole second, measured
	// from the byte count mark at markAt.
	sentBps int
	mark    int64
	markAt  time.Time

	// held is the estimate the viewer counts with, kept while the screen is
	// still; raw and cutting are what GCC said at the last tick (logged).
	held    int
	raw     int
	cutting bool
}

// sentRate is what v was sent over the last whole second, in bits per second.
// Until a second has been measured it is the ceiling, so the estimate counts as
// it is. q.mu held.
func (v *qViewer) sentRate(now time.Time) int {
	if d := now.Sub(v.markAt); d >= time.Second {
		n := v.bwe.sent.Load()
		v.sentBps = int(float64(n-v.mark) * 8 / d.Seconds())
		v.mark, v.markAt = n, now
	}
	return v.sentBps
}

// qualityController owns the policy state and applies it to the running source.
type qualityController struct {
	m         *Manager
	tickEvery time.Duration                // tests shorten it before Start
	tickMu    sync.Mutex                   // one tick at a time
	told      func(id ViewerID, slow bool) // set by tests: sees every slow message

	mu      sync.Mutex // guards everything below
	viewers map[ViewerID]*qViewer
	src     screencap.Source
	cancel  context.CancelFunc
	done    chan struct{}
	state   ladderState
	fullFPS int          // the capture's frame rate at ladder position 0
	still   stillMachine // sharp when still (still.go)

	// What the source has actually been told, so a tick sends only a change.
	appliedBps  int
	appliedFPS  int
	appliedStep int       // the resolution step the capture is at
	retryAt     time.Time // no frame-rate or size retry before this after a failure

	// baseW, baseH is the picture size at step 0 (the native size under the 2560
	// cap); a surface change gives a new one.
	baseW, baseH int

	lastPLI    time.Time // the last keyframe a PLI or FIR asked for
	pliPending bool      // a deferred keyframe is already scheduled
}

func newQualityController(m *Manager) *qualityController {
	return &qualityController{m: m, tickEvery: qualityTick, viewers: make(map[ViewerID]*qViewer)}
}

// register starts counting viewer v's estimate.
func (q *qualityController) register(v *viewer, bwe *viewerBWE) {
	now := q.m.d.Now()
	q.mu.Lock()
	q.viewers[v.ID] = &qViewer{ws: v.ws, bwe: bwe, joined: now, sentBps: ceilingBps, markAt: now, held: ceilingBps}
	q.mu.Unlock()
}

func (q *qualityController) unregister(id ViewerID) {
	q.mu.Lock()
	delete(q.viewers, id)
	q.mu.Unlock()
}

// noteLoss records the fraction lost a viewer's receiver report names.
func (q *qualityController) noteLoss(id ViewerID, frac float64) {
	q.mu.Lock()
	if v := q.viewers[id]; v != nil {
		v.loss = frac
	}
	q.mu.Unlock()
}

// start begins a run on src, at the ceiling and full quality: the normal state is
// high until congestion says otherwise (GCC cannot raise an estimate above 1.5
// times what was sent, so a low start would never recover on a still screen).
func (q *qualityController) start(ctx context.Context, src screencap.Source) {
	lctx, cancel := context.WithCancel(ctx)
	full := q.m.d.SourceConfig.FPS
	if full <= 0 {
		full = screencap.DefaultFPS
	}
	q.mu.Lock()
	q.src, q.state, q.fullFPS = src, ladderState{Target: ceilingBps}, full
	q.appliedBps, q.appliedFPS, q.appliedStep, q.retryAt = 0, full, 0, time.Time{}
	q.baseW, q.baseH = 0, 0
	q.still = stillMachine{}
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

// slowNotice is a slow-set change to tell one viewer about, outside every lock.
type slowNotice struct {
	id   ViewerID
	ws   *websocket.Conn
	slow bool
}

// plan is what a tick decided the source should now be told.
type plan struct {
	src     screencap.Source
	bps     int  // changed bitrate, 0 for none
	fps     int  // changed frame rate, 0 for none
	w, h    int  // changed picture size, 0 for none
	step    int  // the size step w x h belongs to
	resize  bool // w x h is a change
	notices []slowNotice
}

// tick reads every viewer's estimate, moves viewers in and out of the slow set,
// runs the policy over the viewers that count and applies what changed.
func (q *qualityController) tick() {
	q.tickMu.Lock()
	defer q.tickMu.Unlock()
	now := q.m.d.Now()
	pw, ph, sized := q.m.size()
	p, ok := q.decideLocked(now, pw, ph, sized)
	if !ok {
		return
	}
	for _, n := range p.notices {
		q.tell(n)
	}
	q.apply(now, p)
	q.stepStill(now, p.src)
}

// stepStill runs the still machine and asks the source for what it says, outside
// q.mu: the source may take a moment in the native layer.
func (q *qualityController) stepStill(now time.Time, src screencap.Source) {
	ss, ok := src.(screencap.StillSource)
	if !ok {
		return
	}
	ready, asleep := q.m.anyReady(), q.m.isAsleep()
	q.mu.Lock()
	act, target := q.still.step(now, ready, asleep), q.state.Target
	q.mu.Unlock()
	var err error
	switch act {
	case stillRefine:
		err = ss.Refine(refineBps(target))
	case stillIdleRefresh:
		err = ss.IdleRefresh()
	}
	if err != nil {
		slog.Warn("desktop: still refresh failed", "err", err)
	}
}

// anyReady reports whether any viewer's media path is up.
func (m *Manager) anyReady() bool {
	m.vmu.Lock()
	defer m.vmu.Unlock()
	for _, v := range m.viewers {
		if v.ready {
			return true
		}
	}
	return false
}

// decideLocked is the lock-holding half of a tick.
func (q *qualityController) decideLocked(now time.Time, pw, ph int, sized bool) (plan, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	p := plan{src: q.src}
	if p.src == nil || len(q.viewers) == 0 {
		return p, false
	}
	lowest, lowestAll, counted := ceilingBps, ceilingBps, 0
	var lowID, lowAllID ViewerID
	for id, v := range q.viewers {
		// Heavy loss in the viewer's own receiver reports is bad whatever the
		// estimate says: GCC keeps only the last 250 packets of history, so a viewer
		// whose feedback arrives later than that (a flood into a slow link) yields
		// no acknowledgments and an estimate that never moves.
		lossy := v.loss > lossLimit
		est := ceilingBps
		switch {
		case v.stub != nil:
			est = v.stub()
			v.noFB = false
		case v.bwe.noFeedback(now):
			// GCC's estimate never moved, so it measures nothing: the viewer does not
			// constrain the target, and only its loss reports can make it slow.
			v.noFB = true
		default:
			v.noFB = false
			v.raw, v.cutting = v.bwe.GetTargetBitrate(), v.bwe.cutting()
			sent := v.sentRate(now)
			// GCC learns only from what it is sent. Once it has cut a few times at about
			// the rate it is sent now, it rises additively and never past 1.5 times what
			// came back in the last 500 ms, so with bursty screen content its estimate
			// sits at or under what was sent though nothing is congested; read as a
			// limit it held the target at the floor (VM row Q-03, lone viewer). So an
			// estimate constrains only while GCC is cutting (its own congestion signal)
			// or the viewer loses heavily (a full drop-tail queue shows no delay trend),
			// and not even then with room over what this viewer was really sent. While
			// the screen is still next to nothing is sent and nothing is measured: the
			// estimate is the one the viewer last counted with.
			if !q.still.still {
				v.held = v.raw
				if !(v.cutting || lossy) || float64(v.raw) >= upRatio*float64(sent) {
					v.held = ceilingBps
				}
			}
			est = v.held
		}
		bad, good := est <= floorBps || lossy, float64(est) > upRatio*floorBps && !lossy
		if v.noFB {
			bad, good = lossy, !lossy
		}
		var changed bool
		if v.slow, changed = stepSlow(v.slow, now, bad, good); changed {
			p.notices = append(p.notices, slowNotice{id: id, ws: v.ws, slow: v.slow.Slow})
		}
		if est <= lowestAll {
			lowestAll, lowAllID = est, id
		}
		if !v.slow.Slow {
			if est <= lowest {
				lowest, lowID = est, id
			}
			counted++
		}
	}
	if counted == 0 {
		lowest, lowID = lowestAll, lowAllID // every viewer is slow: a lone slow viewer still gets the floor
	}
	old := q.state.Target
	q.state = decide(policyInput{Now: now, Min: lowest, Prev: q.state}).Next
	if v := q.viewers[lowID]; v != nil && q.state.Target != old {
		// What the VM rows need to see: which viewer's estimate moved the target, and why.
		slog.Info("desktop: target changed", "from", old, "to", q.state.Target, "min", lowest,
			"viewer", shortID(lowID), "est", v.raw, "sent", v.sentBps, "cutting", v.cutting, "still", q.still.still)
	}

	if q.state.Target != q.appliedBps {
		p.bps = q.state.Target
	}
	if fps := fpsFor(q.state.Level, q.fullFPS); fps != q.appliedFPS && !now.Before(q.retryAt) {
		p.fps = fps
	}
	q.trackBase(pw, ph, sized)
	if step := resStep(q.state.Level); sized && step != q.appliedStep && !now.Before(q.retryAt) {
		p.w, p.h = ladderSize(q.baseW, q.baseH, ladderEdges[step])
		p.step, p.resize = step, true
	}
	return p, true
}

// trackBase keeps the native picture size. A size the ladder itself makes is one
// of ours; any other is a new surface (Phase 20), whose capture starts at its own
// native size, so the step it is at is 0 until the controller re-applies the
// current one. q.mu held.
func (q *qualityController) trackBase(w, h int, sized bool) {
	if !sized {
		return
	}
	if q.baseW == 0 {
		q.baseW, q.baseH = w, h
		return
	}
	if w == q.baseW && h == q.baseH {
		return
	}
	for _, edge := range ladderEdges {
		if sw, sh := ladderSize(q.baseW, q.baseH, edge); sw == w && sh == h {
			return
		}
	}
	q.baseW, q.baseH, q.appliedStep = w, h, 0
}

// apply tells the source what changed, outside q.mu: a size change can take
// seconds in the native layer. A failed frame-rate or size change is not retried
// for stepHold (a size change forces a keyframe for everyone); a failed bitrate
// change is retried on the next tick.
func (q *qualityController) apply(now time.Time, p plan) {
	qs, isQS := p.src.(screencap.QualitySource)
	if p.bps != 0 {
		if isQS {
			if err := qs.SetBitrate(p.bps); err != nil {
				slog.Warn("desktop: bitrate change failed", "err", err)
				p.bps = 0
			}
		}
		if p.bps != 0 {
			q.mu.Lock()
			q.appliedBps = p.bps
			q.mu.Unlock()
		}
	}
	if p.fps != 0 {
		var err error
		if isQS {
			err = qs.SetFPS(p.fps)
		}
		q.mu.Lock()
		if err != nil {
			slog.Warn("desktop: frame rate change failed", "err", err)
			q.retryAt = now.Add(stepHold)
		} else {
			q.appliedFPS = p.fps
		}
		q.mu.Unlock()
	}
	if p.resize {
		err := p.src.Rebuild(p.w, p.h)
		q.mu.Lock()
		if err != nil {
			slog.Warn("desktop: picture size change failed", "err", err)
			q.retryAt = now.Add(stepHold)
		} else {
			q.appliedStep = p.step
		}
		q.mu.Unlock()
	}
}

// tell sends one slow message in its own goroutine: the viewer is slow by
// definition, so its socket may be the one that stalls, and the tick must not.
func (q *qualityController) tell(n slowNotice) {
	slog.Info("desktop: viewer slow state changed", "viewer", shortID(n.id), "slow", n.slow)
	if q.told != nil {
		q.told(n.id, n.slow)
	}
	if n.ws == nil {
		return
	}
	go func() {
		defer recoverLog()
		_ = send(context.Background(), n.ws, msgQuality{Type: typeQuality, Slow: n.slow})
	}()
}

func shortID(id ViewerID) string { return string(id)[:min(len(id), 8)] }

// onKeyframeRequest is what a viewer's PLI or FIR does. In the first joinWindow
// it always counts. After that a slow viewer's request is ignored (its loss must
// not cost everyone else a large keyframe), and the others' are coalesced to one
// keyframe per pliEvery, the rest riding on one deferred keyframe.
func (q *qualityController) onKeyframeRequest(id ViewerID) {
	now := q.m.d.Now()
	q.mu.Lock()
	v := q.viewers[id]
	switch {
	case v == nil || now.Sub(v.joined) < joinWindow:
		q.mu.Unlock()
	case v.slow.Slow:
		q.mu.Unlock()
		return
	default:
		if since := now.Sub(q.lastPLI); !q.lastPLI.IsZero() && since < pliEvery {
			if !q.pliPending {
				q.pliPending = true
				time.AfterFunc(pliEvery-since, q.firePLI)
			}
			q.mu.Unlock()
			return
		}
		q.lastPLI = now
		q.mu.Unlock()
	}
	q.m.requestKeyframe("pli")
}

// firePLI is the deferred half of onKeyframeRequest.
func (q *qualityController) firePLI() {
	defer recoverLog()
	q.mu.Lock()
	q.pliPending = false
	q.lastPLI = q.m.d.Now()
	q.mu.Unlock()
	q.m.requestKeyframe("pli")
}

// noteUnit is the fan-out's per-unit hook. A fresh capture ends stillness; refine
// and idle-refresh units re-encode the cached picture, so they neither end it nor
// count as a captured frame for the Phase 19 stall watchdog (the return value).
// Join and PLI keyframes keep their Phase 19 counting.
func (m *Manager) noteUnit(au screencap.AccessUnit) bool {
	switch au.Kind {
	case screencap.UnitRefine, screencap.UnitIdle:
		return false
	case screencap.UnitFresh:
		now := m.d.Now()
		m.q.mu.Lock()
		m.q.still.fresh(now)
		m.q.mu.Unlock()
	}
	return true
}

// QualityStats reports what the controller currently asks of the encoder.
func (m *Manager) QualityStats() QualityStats {
	q := m.q
	q.mu.Lock()
	defer q.mu.Unlock()
	st := QualityStats{
		TargetBps: q.state.Target,
		FPS:       fpsFor(q.state.Level, max(q.fullFPS, 1)),
		LongEdge:  ladderEdges[resStep(q.state.Level)],
		Step:      q.state.Level,
		Still:     q.still.still,
	}
	for _, v := range q.viewers {
		if v.slow.Slow {
			st.Slow++
		}
		if v.noFB {
			st.NoFeedback++
		}
	}
	return st
}
