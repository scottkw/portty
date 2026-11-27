package desktop_test

import (
	"context"
	"math/rand"
	"net"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/desktop"
	"github.com/scottkw/portty/internal/screencap"
)

// shapedLink is a userspace UDP forwarder between one loopback viewer and the
// daemon's media port, so a test can give that viewer a bad link without root,
// pfctl or dnctl. The viewer is told (by rewriting the daemon's candidate in its
// answer) that the daemon lives at the forwarder's port. Daemon to viewer is
// shaped: a token-bucket rate, a bounded queue that drops its tail, a fixed
// delay and random loss from a fixed seed; viewer to daemon only pays the delay.
// RESEARCH Pattern 10 (the candidate rewrite; pion's vnet was not needed).
type shapedLink struct {
	front, back *net.UDPConn
	daemon      *net.UDPAddr
	rate        int           // daemon to viewer, bits per second
	delay       time.Duration // each way
	lossPct     float64
	maxQueue    time.Duration // longest the bottleneck's queue may hold a packet

	mu       sync.Mutex
	client   *net.UDPAddr
	nextFree time.Time
	rng      *rand.Rand

	up, down      *lane
	sent, dropped atomic.Int64 // daemon-to-viewer packets passed and dropped
}

// lane delivers packets in order, each no earlier than its time.
type lane struct {
	ch    chan lanePkt
	write func([]byte)
	done  chan struct{}
}

type lanePkt struct {
	b  []byte
	at time.Time
}

func newLane(write func([]byte)) *lane {
	l := &lane{ch: make(chan lanePkt, 8192), write: write, done: make(chan struct{})}
	go func() {
		for {
			select {
			case p := <-l.ch:
				if d := time.Until(p.at); d > 0 {
					time.Sleep(d)
				}
				l.write(p.b)
			case <-l.done:
				return
			}
		}
	}()
	return l
}

func (l *lane) push(b []byte, at time.Time) bool {
	select {
	case l.ch <- lanePkt{b: b, at: at}:
		return true
	default:
		return false
	}
}

func newShapedLink(t *testing.T, daemonPort int, shape linkShape) *shapedLink {
	t.Helper()
	lo := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)}
	front, err := net.ListenUDP("udp4", lo)
	if err != nil {
		t.Fatal(err)
	}
	back, err := net.ListenUDP("udp4", lo)
	if err != nil {
		t.Fatal(err)
	}
	s := &shapedLink{
		front: front, back: back, rate: shape.rate, delay: shape.delay, lossPct: shape.lossPct,
		daemon:   &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: daemonPort},
		maxQueue: shape.maxQueue,
		rng:      rand.New(rand.NewSource(1)),
	}
	s.up = newLane(func(b []byte) { _, _ = back.WriteToUDP(b, s.daemon) })
	s.down = newLane(func(b []byte) {
		s.mu.Lock()
		c := s.client
		s.mu.Unlock()
		if c != nil {
			_, _ = front.WriteToUDP(b, c)
		}
	})
	t.Cleanup(func() {
		close(s.up.done)
		close(s.down.done)
		_ = front.Close()
		_ = back.Close()
	})
	go s.readViewer()
	go s.readDaemon()
	return s
}

func (s *shapedLink) frontPort() int { return s.front.LocalAddr().(*net.UDPAddr).Port }

// candidateRe matches the daemon's host candidate line of an answer.
var candidateRe = regexp.MustCompile(`(a=candidate:\S+ \d+ \w+ \d+ \S+ )\d+( typ host)`)

// pointAt rewrites an answer so the viewer reaches the daemon through the link.
func (s *shapedLink) pointAt(answer string) string {
	return candidateRe.ReplaceAllString(answer, "${1}"+strconv.Itoa(s.frontPort())+"${2}")
}

func (s *shapedLink) readViewer() {
	buf := make([]byte, 2048)
	for {
		n, from, err := s.front.ReadFromUDP(buf)
		if err != nil {
			return
		}
		s.mu.Lock()
		if s.client == nil {
			s.client = from
		}
		s.mu.Unlock()
		s.up.push(slices.Clone(buf[:n]), time.Now().Add(s.delay))
	}
}

func (s *shapedLink) readDaemon() {
	buf := make([]byte, 2048)
	for {
		n, _, err := s.back.ReadFromUDP(buf)
		if err != nil {
			return
		}
		now := time.Now()
		s.mu.Lock()
		if s.rng.Float64()*100 < s.lossPct {
			s.mu.Unlock()
			s.dropped.Add(1)
			continue
		}
		start := s.nextFree
		if start.Before(now) {
			start = now
		}
		end := start.Add(time.Duration(float64(n*8) / float64(s.rate) * float64(time.Second)))
		if end.Sub(now) > s.maxQueue {
			s.mu.Unlock()
			s.dropped.Add(1) // the bottleneck's queue is full: drop the tail
			continue
		}
		s.nextFree = end
		s.mu.Unlock()
		s.sent.Add(1)
		s.down.push(slices.Clone(buf[:n]), end.Add(s.delay))
	}
}

// loadSource is a source whose output rate is its bitrate: every unit is as big
// as one frame of the current target, so a viewer behind a slow link really is
// sent more than it can take, and a bitrate change really changes the load. It
// implements QualitySource and records what it was told.
type loadSource struct {
	mu       sync.Mutex
	frames   chan screencap.AccessUnit
	errs     chan error
	stop     chan struct{}
	done     chan struct{}
	started  bool
	w, h     int
	bitrates []int

	bps     atomic.Int64
	forceKF atomic.Bool
	paused  atomic.Bool // no new picture (a still screen); a forced keyframe still goes out
	nudged  atomic.Bool // the watchdog moved the pointer: one frame goes out even while paused
}

var (
	loadSPS = []byte{0, 0, 0, 1, 0x67, 0x42, 0xe0, 0x1f, 0xda, 0x05, 0x00, 0x7d, 0xb0, 0x11}
	loadPPS = []byte{0, 0, 0, 1, 0x68, 0xce, 0x3c, 0x80}
)

func (s *loadSource) Start(ctx context.Context, cfg screencap.Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return nil
	}
	s.started = true
	s.bps.Store(int64(cfg.BitrateBps))
	s.forceKF.Store(true)
	s.frames, s.errs = make(chan screencap.AccessUnit, 8), make(chan error, 1)
	s.stop, s.done = make(chan struct{}), make(chan struct{})
	go s.run(ctx, s.frames, s.stop, s.done)
	return nil
}

func (s *loadSource) run(ctx context.Context, frames chan<- screencap.AccessUnit, stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	defer close(frames)
	every := time.Second / screencap.DefaultFPS
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-t.C:
		case <-stop:
			return
		case <-ctx.Done():
			return
		}
		key := s.forceKF.Swap(false)
		if s.paused.Load() && !key && !s.nudged.Swap(false) {
			continue
		}
		s.mu.Lock()
		w, h := s.w, s.h
		s.mu.Unlock()
		n := max(int(s.bps.Load())/8/screencap.DefaultFPS, 200)
		au := screencap.AccessUnit{Width: w, Height: h, Duration: every}
		if key {
			au.Keyframe = true
			au.Data = append(slices.Concat(loadSPS, loadPPS, []byte{0, 0, 0, 1, 0x65}), slices.Repeat([]byte{0x55}, 3*n)...)
		} else {
			au.Data = append([]byte{0, 0, 0, 1, 0x41}, slices.Repeat([]byte{0x55}, n)...)
		}
		select {
		case frames <- au:
		case <-stop:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (s *loadSource) Stop() error {
	s.mu.Lock()
	stop, done := s.stop, s.done
	s.stop = nil
	s.mu.Unlock()
	if stop != nil {
		close(stop)
		<-done
	}
	return nil
}

func (s *loadSource) Frames() <-chan screencap.AccessUnit {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.frames
}

func (s *loadSource) Errors() <-chan error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.errs
}

func (s *loadSource) SelectSurface(screencap.Surface) error { return nil }
func (s *loadSource) ForceKeyframe()                        { s.forceKF.Store(true) }

func (s *loadSource) Rebuild(w, h int) error {
	s.mu.Lock()
	s.w, s.h = w, h
	s.mu.Unlock()
	s.forceKF.Store(true)
	return nil
}

func (s *loadSource) SetBitrate(bps int) error {
	s.bps.Store(int64(bps))
	s.mu.Lock()
	s.bitrates = append(s.bitrates, bps)
	s.mu.Unlock()
	return nil
}

func (s *loadSource) SetFPS(int) error { return nil }

// withLoadSource makes the Manager's source a loadSource of the given size and
// keeps the latest one for the test to read.
func withLoadSource(cur *atomic.Pointer[loadSource], w, h int) tracerOpt {
	return func(_ *tracerEnv, d *desktop.Deps) {
		d.NewSource = func() screencap.Source {
			s := &loadSource{w: w, h: h}
			cur.Store(s)
			return s
		}
		// A still screen is told from a frozen stream by the Phase 19 watchdog's
		// pointer nudge, which makes a real capture deliver a frame. The fake does
		// the same, so a quiet source is not restarted as stalled.
		d.Probe = func(context.Context) (screencap.Permissions, error) {
			return screencap.Permissions{SR: true, AX: true, PE: true}, nil
		}
		d.Nudge = func() (bool, error) {
			if s := cur.Load(); s != nil {
				s.nudged.Store(true)
			}
			return true, nil
		}
	}
}

// linkShape is the bad link of a shaped scene.
type linkShape struct {
	rate     int           // bits per second
	delay    time.Duration // each way
	maxQueue time.Duration
	lossPct  float64
}

// slowLink is about 300 kbps with 50 ms of round trip and 2 percent loss.
var slowLink = linkShape{rate: 300_000, delay: 25 * time.Millisecond, maxQueue: 400 * time.Millisecond, lossPct: 2}

// shapedScene is two viewers on one daemon: A direct, B behind a bad link.
type shapedScene struct {
	env  *tracerEnv
	src  *atomic.Pointer[loadSource]
	link *shapedLink
	a, b *fanViewer
	aID  desktop.ViewerID

	// What the process looked like with the server and the link up and no viewer.
	baseGoroutines int
	baseHeap       uint64
}

func newShapedScene(ctx context.Context, t *testing.T, shape linkShape) *shapedScene {
	t.Helper()
	cur := &atomic.Pointer[loadSource]{}
	env := newTracerServer(t, withLoadSource(cur, 1920, 1080))
	tok := env.token(t, "g1")
	link := newShapedLink(t, env.mediaPort, shape)
	sc := &shapedScene{env: env, src: cur, link: link}
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	sc.baseGoroutines, sc.baseHeap = runtime.NumGoroutine(), ms.HeapInuse
	sc.a = env.join(ctx, t, tok)
	requireKeyframe(t, "A", sc.a.firstAU(t, 10*time.Second))
	sc.aID = onlyViewerID(t, env.mgr)
	sc.b = env.join(ctx, t, tok, func(lc *loopbackClient) { lc.answerMunge = link.pointAt })
	eventually(t, 20*time.Second, "both viewers counted", func() bool { return env.mgr.QualityViewersForTest() == 2 })
	return sc
}

// estimateA is viewer A's own GCC estimate (not the encoder's target).
func (sc *shapedScene) estimateA() int {
	bps, _ := sc.env.mgr.ViewerEstimateForTest(sc.aID)
	return bps
}

func (sc *shapedScene) bSlow() bool { return slices.Contains(sc.b.qualityMsgs(), true) }

// sample is one second of the run.
type sample struct {
	at       time.Duration
	aUnits   uint64 // units A received in the second
	estA     int
	target   int
	heap     uint64
	bSlow    bool
	linkDrop int64
}

// run samples once a second for d and returns the samples.
func (sc *shapedScene) run(t *testing.T, d time.Duration) []sample {
	t.Helper()
	var out []sample
	start, last := time.Now(), sc.a.lc.units.Load()
	for time.Since(start) < d {
		time.Sleep(time.Second)
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		u := sc.a.lc.units.Load()
		s := sample{
			at: time.Since(start).Round(time.Second), aUnits: u - last, estA: sc.estimateA(),
			target: sc.env.mgr.QualityStats().TargetBps, heap: ms.HeapInuse, bSlow: sc.bSlow(), linkDrop: sc.link.dropped.Load(),
		}
		last = u
		out = append(out, s)
		t.Logf("t=%2.0fs  A units/s=%2d  A estimate=%7d  encoder target=%7d  B slow=%-5v  heap=%3d MiB  link drops=%d",
			s.at.Seconds(), s.aUnits, s.estA, s.target, s.bSlow, s.heap>>20, s.linkDrop)
	}
	return out
}

// leave closes both viewers, client side included, so every goroutine of the
// run can end.
func (sc *shapedScene) leave() {
	for _, v := range []*fanViewer{sc.a, sc.b} {
		_ = v.lc.pc.Close()
		_ = v.conn.CloseNow()
	}
}

func goroutinesSettled(t *testing.T, base int, within time.Duration) {
	t.Helper()
	var n int
	eventually(t, within, "goroutines back to their starting count", func() bool {
		n = runtime.NumGoroutine()
		return n <= base
	})
	t.Logf("goroutines: start %d, after both viewers left %d", base, n)
}

// TestQualityShaped_SlowViewerDoesNotStallOthers: viewer B sits behind a link
// of about 300 kbps. Within 15 s of joining it is left out of the minimum and
// told slow, viewer A keeps receiving a full picture rate and its own estimate
// stays high, the encoder's load is lowered while B counts and recovers once it
// does not, a PLI storm from B costs nobody a keyframe (see the policy test), no
// pacer queue grows without bound (heap stays under 64 MiB over the start) and
// every goroutine ends when both viewers leave. Threats T-23-04, T-23-05.
func TestQualityShaped_SlowViewerDoesNotStallOthers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	sc := newShapedScene(ctx, t, slowLink)

	samples := sc.run(t, 15*time.Second)
	if !sc.bSlow() {
		t.Fatalf("B was not told slow within 15 s of joining on a 300 kbps link; link dropped %d of %d packets", sc.link.dropped.Load(), sc.link.dropped.Load()+sc.link.sent.Load())
	}
	samples = append(samples, sc.run(t, 13*time.Second)...)
	if got := sc.b.qualityMsgs(); !slices.Equal(got, []bool{true}) {
		t.Errorf("B was told %v, want one slow true and no flapping", got)
	}

	var peak uint64
	low := 0
	for i, s := range samples {
		peak = max(peak, s.heap)
		if i >= 3 && s.aUnits < 15 {
			t.Errorf("t=%v: viewer A received %d units in a second, want about 30 (stalled by the slow viewer)", s.at, s.aUnits)
		}
		// GCC on a clean loopback link jitters under -race and CPU load: a sample at
		// or under 4 Mbps now and then is that, a viewer dragged down would be under
		// it for most of the run.
		if s.estA <= 4_000_000 || s.target <= 4_000_000 {
			low++
			t.Logf("t=%v: A's estimate %d, encoder target %d: at or under 4 Mbps", s.at, s.estA, s.target)
		}
	}
	if low*10 > len(samples) {
		t.Errorf("%d of %d samples had A's estimate or the encoder target at or under 4 Mbps (B must not drag the others down)", low, len(samples))
	}
	if grew := int64(peak) - int64(sc.baseHeap); grew > 64<<20 {
		t.Errorf("heap in use grew %d MiB over the start, want under 64 (an unbounded pacer queue)", grew>>20)
	}

	sc.leave()
	goroutinesSettled(t, sc.baseGoroutines, 30*time.Second)
}

// TestQualityShaped_MediumLinkPullsTheTargetDown: the closed loop with real
// estimates. Viewer B's link carries about 3 Mbps while the encoder starts at 8:
// B's transport-cc feedback lowers its GCC estimate, the controller lowers the
// encoder's bitrate within seconds, and viewer A, on a clean link, keeps its full
// picture rate throughout.
func TestQualityShaped_MediumLinkPullsTheTargetDown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	sc := newShapedScene(ctx, t, linkShape{rate: 3_000_000, delay: 10 * time.Millisecond, maxQueue: 100 * time.Millisecond})
	samples := sc.run(t, 10*time.Second)
	lowest := samples[0].target
	for _, s := range samples {
		lowest = min(lowest, s.target)
		if s.aUnits < 15 {
			t.Errorf("t=%v: viewer A received %d units in a second, want about 30", s.at, s.aUnits)
		}
	}
	if lowest > 4_000_000 {
		t.Fatalf("lowest encoder target %d over 10 s with a 3 Mbps viewer, want it pulled under 4 Mbps", lowest)
	}
	src := sc.src.Load()
	src.mu.Lock()
	told := append([]int(nil), src.bitrates...)
	src.mu.Unlock()
	t.Logf("encoder was told: %v", told)
	if len(told) == 0 || told[0] != 8_000_000 {
		t.Errorf("encoder told %v, want it to start at 8000000", told)
	}
	sc.leave()
}

// TestQualityShaped_StillScreenDoesNotCollapseEstimate: after 10 s with no new
// frames, motion starts again with the unshaped viewer's own estimate still at
// least 80 percent of the ceiling: a still screen sends nothing, and an estimate
// that follows what was sent would have collapsed (RESEARCH Pitfall 1).
func TestQualityShaped_StillScreenDoesNotCollapseEstimate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	sc := newShapedScene(ctx, t, slowLink)
	sc.run(t, 6*time.Second)

	sc.src.Load().paused.Store(true)
	t.Log("screen is still")
	before := sc.a.lc.units.Load()
	sc.run(t, 10*time.Second)
	if got := sc.a.lc.units.Load() - before; got > 5 {
		t.Fatalf("%d units reached A in 10 s of a still screen, want almost none", got)
	}

	sc.src.Load().paused.Store(false)
	t.Log("motion again")
	after := sc.run(t, 4*time.Second)
	// The first seconds of motion are the ones that matter: a collapsed estimate
	// would show right there, in both samples. GCC's jitter under load is not what
	// this tests, so the better of the two counts.
	if best := max(after[0].estA, after[1].estA); best < 6_400_000 {
		t.Fatalf("A's estimate %d when motion resumed, want at least 80 percent of the ceiling (6400000)", best)
	}
	if last := after[len(after)-1]; last.aUnits < 20 {
		t.Fatalf("A received %d units in the last second of motion, want a full picture rate", last.aUnits)
	}
	sc.leave()
}
