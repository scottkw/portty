package desktop_test

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/pion/webrtc/v4"

	"github.com/scottkw/portty/internal/desktop"
	"github.com/scottkw/portty/internal/screencap"
)

// fakeClock drives Deps.Now so the keyframe coalescer is tested without
// sleeping through its window.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock { return &fakeClock{t: time.Unix(1_700_000_000, 0)} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func withClock(c *fakeClock) tracerOpt {
	return func(_ *tracerEnv, d *desktop.Deps) { d.Now = c.Now }
}

// fanViewer is one full viewer: signalling socket plus pion client. A reader
// goroutine keeps the latest state and size messages and notes how the socket
// closed.
type fanViewer struct {
	conn      *websocket.Conn
	lc        *loopbackClient
	answerSDP string

	mu        sync.Mutex
	viewers   int
	state     string
	locked    bool
	w, h      int
	sizeCount int
	closeCode websocket.StatusCode
	closed    chan struct{}
}

// join runs hello, offer and answer for a new viewer and starts its reader.
// extra runs on the client's PeerConnection before the offer is made (to add a
// data channel).
func (e *tracerEnv) join(ctx context.Context, t *testing.T, tok string, extra ...func(*loopbackClient)) *fanViewer {
	t.Helper()
	conn, _, err := e.dial(ctx, tok)
	if err != nil {
		t.Fatalf("dial /desktop/ws: %v", err)
	}
	t.Cleanup(func() { _ = conn.CloseNow() })
	writeJSON(ctx, t, conn, map[string]any{"type": "hello", "v": 1, "caps": map[string]int{"video": 1}})
	readType(ctx, t, conn, "hello")
	lc := newLoopbackClient(t)
	for _, f := range extra {
		f(lc)
	}
	writeJSON(ctx, t, conn, map[string]any{"type": "offer", "sdp": lc.offer(t)})
	answerSDP := readType(ctx, t, conn, "answer")["sdp"].(string)
	lc.accept(t, answerSDP)
	v := &fanViewer{conn: conn, lc: lc, answerSDP: answerSDP, closed: make(chan struct{})}
	go v.read(ctx)
	return v
}

func (v *fanViewer) read(ctx context.Context) {
	defer close(v.closed)
	for {
		_, b, err := v.conn.Read(ctx)
		if err != nil {
			v.mu.Lock()
			v.closeCode = websocket.CloseStatus(err)
			v.mu.Unlock()
			return
		}
		var m map[string]any
		if json.Unmarshal(b, &m) != nil {
			continue
		}
		v.mu.Lock()
		switch m["type"] {
		case "state":
			v.viewers = int(m["viewers"].(float64))
			v.state, _ = m["state"].(string)
			v.locked, _ = m["locked"].(bool)
		case "size":
			v.w, v.h = int(m["w"].(float64)), int(m["h"].(float64))
			v.sizeCount++
		}
		v.mu.Unlock()
	}
}

// lastState returns the latest state message's state, viewer count and lock flag.
func (v *fanViewer) lastState() (state string, viewers int, locked bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.state, v.viewers, v.locked
}

func (v *fanViewer) stateViewers() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.viewers
}

func (v *fanViewer) size() (int, int) {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.w, v.h
}

// firstAU waits for the next access unit the client depacketized.
func (v *fanViewer) firstAU(t *testing.T, within time.Duration) []byte {
	t.Helper()
	select {
	case au := <-v.lc.aus:
		return au
	case <-time.After(within):
		t.Fatalf("no access unit within %v", within)
		return nil
	}
}

// requireKeyframe fails unless au holds SPS, PPS and an IDR slice.
func requireKeyframe(t *testing.T, who string, au []byte) {
	t.Helper()
	if types := nalTypes(au); !hasAll(types, 7, 8, 5) {
		t.Fatalf("%s: access unit has NAL types %v, want SPS+PPS+IDR (7, 8, 5)", who, types)
	}
}

// TestFanout_ThreeViewersEachGetKeyframe: three viewers join one after
// another on one source. Each one's first access unit is a keyframe (a viewer
// gets no delta before its first keyframe) within 2 s of its own join, all see
// the same picture size, and each sees the viewer count of the others.
func TestFanout_ThreeViewersEachGetKeyframe(t *testing.T) {
	clk := newFakeClock()
	env := newTracerServer(t, withClock(clk))
	tok := env.token(t, "g1")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var vs []*fanViewer
	for i, name := range []string{"A", "B", "C"} {
		if i > 0 {
			clk.Advance(2 * desktop.KeyframeCoalesce) // past the coalescing window
		}
		start := time.Now()
		v := env.join(ctx, t, tok)
		requireKeyframe(t, name, v.firstAU(t, 2*time.Second))
		if d := time.Since(start); d > 2*time.Second {
			t.Errorf("viewer %s took %v to its first keyframe, want under 2s", name, d)
		}
		vs = append(vs, v)
	}

	for _, v := range vs {
		eventually(t, 2*time.Second, "viewer count 3", func() bool { return v.stateViewers() == 3 })
		eventually(t, 2*time.Second, "size message", func() bool { w, h := v.size(); return w == 1280 && h == 800 })
	}
	if n := env.sourceCount(); n != 1 {
		t.Errorf("source started %d times, want 1", n)
	}
	if n := env.lastSource().KeyframesForced(); n != 3 {
		t.Errorf("keyframes forced %d, want 3 (one per viewer, none coalesced away)", n)
	}
	if got := len(env.mgr.ViewerList()); got != 3 {
		t.Fatalf("ViewerList has %d entries, want 3", got)
	}
	for _, rec := range env.mgr.ViewerList() {
		if rec.Rights != desktop.RightsViewOnly || rec.ID == "" || rec.JoinedAt.IsZero() {
			t.Errorf("viewer record %+v, want an ID, view-only rights and a join time", rec)
		}
	}
}

// drain empties a viewer's access-unit channel.
func (v *fanViewer) drain() {
	for {
		select {
		case <-v.lc.aus:
		default:
			return
		}
	}
}

// keyframeWithin waits up to d for an access unit holding SPS, PPS and an IDR.
func (v *fanViewer) keyframeWithin(t *testing.T, who string, d time.Duration) {
	t.Helper()
	deadline := time.After(d)
	for {
		select {
		case au := <-v.lc.aus:
			if hasAll(nalTypes(au), 5) {
				requireKeyframe(t, who, au)
				return
			}
		case <-deadline:
			t.Fatalf("%s: no keyframe within %v", who, d)
		}
	}
}

func withSource(w, h int) tracerOpt {
	return func(e *tracerEnv, _ *desktop.Deps) { e.srcW, e.srcH = w, h }
}

// TestFanout_SizeEventOnRebuild: a Rebuild tells every connected viewer the
// new size and sends a keyframe at it; a viewer joining later is told the
// current size right after its answer.
func TestFanout_SizeEventOnRebuild(t *testing.T) {
	clk := newFakeClock()
	env := newTracerServer(t, withClock(clk), withSource(1920, 1200))
	tok := env.token(t, "g1")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	a := env.join(ctx, t, tok)
	requireKeyframe(t, "A", a.firstAU(t, 2*time.Second))
	clk.Advance(2 * desktop.KeyframeCoalesce)
	b := env.join(ctx, t, tok)
	requireKeyframe(t, "B", b.firstAU(t, 2*time.Second))
	for name, v := range map[string]*fanViewer{"A": a, "B": b} {
		eventually(t, 2*time.Second, name+" size 1920x1200", func() bool { w, h := v.size(); return w == 1920 && h == 1200 })
	}
	a.drain()
	b.drain()

	if err := env.lastSource().Rebuild(1280, 800); err != nil {
		t.Fatal(err)
	}
	for name, v := range map[string]*fanViewer{"A": a, "B": b} {
		eventually(t, 2*time.Second, name+" size 1280x800", func() bool { w, h := v.size(); return w == 1280 && h == 800 })
		v.keyframeWithin(t, name, 3*time.Second)
	}

	clk.Advance(2 * desktop.KeyframeCoalesce)
	c := env.join(ctx, t, tok)
	eventually(t, 2*time.Second, "C size 1280x800 after its answer", func() bool { w, h := c.size(); return w == 1280 && h == 800 })
}

// TestFanout_DataChannelHook: a data channel the viewer creates before its
// offer reaches the server's hook with the viewer's identity (view-only); with
// no hook the Phase 19 default closes it. The answer carries the 8192-byte
// SCTP message limit.
func TestFanout_DataChannelHook(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	addChannel := func(lc *loopbackClient, closed chan struct{}) {
		dc, err := lc.pc.CreateDataChannel("input", nil)
		if err != nil {
			t.Fatal(err)
		}
		dc.OnClose(func() { close(closed) })
	}

	type got struct {
		v     desktop.Viewer
		label string
	}
	hooked := make(chan got, 1)
	env := newTracerServer(t, func(_ *tracerEnv, d *desktop.Deps) {
		d.OnDataChannel = func(v desktop.Viewer, dc *webrtc.DataChannel) { hooked <- got{v, dc.Label()} }
	})
	closed := make(chan struct{})
	v := env.join(ctx, t, env.token(t, "g1"), func(lc *loopbackClient) { addChannel(lc, closed) })
	if !strings.Contains(v.answerSDP, "m=application") {
		t.Error("answer has no application m-line")
	}
	if !strings.Contains(v.answerSDP, "a=max-message-size:8192") {
		t.Error("answer does not carry max-message-size 8192")
	}
	select {
	case g := <-hooked:
		list := env.mgr.ViewerList()
		if g.label != "input" || g.v.Rights != desktop.RightsViewOnly || len(list) != 1 || g.v.ID != list[0].ID {
			t.Errorf("hook got %+v, viewer list %+v; want label input, view-only, the connected viewer", g, list)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the data-channel hook was not called")
	}
	if got := desktop.EngineSummaryForTest().SCTPMax; got != 8192 {
		t.Errorf("SCTP max message size %d, want 8192", got)
	}

	// No hook: the default closes the channel.
	env2 := newTracerServer(t)
	closed2 := make(chan struct{})
	env2.join(ctx, t, env2.token(t, "g1"), func(lc *loopbackClient) { addChannel(lc, closed2) })
	select {
	case <-closed2:
	case <-time.After(2 * time.Second):
		t.Fatal("the default hook did not close the data channel within 2s")
	}
}

// TestFanout_HelloCapsForwardCompatible: a hello that offers a capability this
// server does not have is accepted, and the reply lists only what it has.
func TestFanout_HelloCapsForwardCompatible(t *testing.T) {
	env := newTracerServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tok := env.token(t, "g1")

	c, _, err := env.dial(ctx, tok)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.CloseNow() }()
	writeJSON(ctx, t, c, map[string]any{"type": "hello", "v": 1, "caps": map[string]int{"video": 1, "input": 1}})
	caps, _ := readType(ctx, t, c, "hello")["caps"].(map[string]any)
	if len(caps) != 1 || caps["video"] != float64(1) {
		t.Errorf("hello reply caps %v, want exactly {video:1}", caps)
	}

	c2, _, err := env.dial(ctx, tok)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c2.CloseNow() }()
	writeJSON(ctx, t, c2, map[string]any{"type": "hello", "v": 2})
	expectRejected(ctx, t, c2, "version")
}

// TestFanout_ViewerCap: the ninth viewer is closed with 4429 "full", before a
// PeerConnection exists, and the eight keep streaming.
func TestFanout_ViewerCap(t *testing.T) {
	env := newTracerServer(t)
	tok := env.token(t, "g1")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	var vs []*fanViewer
	for range desktop.MaxViewers {
		vs = append(vs, env.join(ctx, t, tok))
	}
	for i, v := range vs {
		v.keyframeWithin(t, "viewer "+strconv.Itoa(i), 10*time.Second)
	}
	if !env.mgr.Full() || env.mgr.Viewers() != desktop.MaxViewers {
		t.Fatalf("Full()=%v Viewers()=%d, want full at %d", env.mgr.Full(), env.mgr.Viewers(), desktop.MaxViewers)
	}

	c, _, err := env.dial(ctx, tok)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.CloseNow() }()
	_, _, err = c.Read(ctx)
	var ce websocket.CloseError
	if !errors.As(err, &ce) || ce.Code != desktop.CloseFull || ce.Reason != "full" {
		t.Fatalf("ninth viewer: %v, want close 4429 \"full\"", err)
	}

	if env.mgr.Viewers() != desktop.MaxViewers {
		t.Errorf("Viewers() = %d after the refusal, want %d", env.mgr.Viewers(), desktop.MaxViewers)
	}
	for i, v := range vs {
		v.drain()
		v.firstAU(t, 2*time.Second) // still streaming
		if got := v.stateViewers(); got != desktop.MaxViewers {
			t.Errorf("viewer %d last saw %d viewers, want %d", i, got, desktop.MaxViewers)
		}
	}
}

// TestFanout_ReconnectWhileJoining: one viewer reconnects while another joins
// at the same moment; everyone ends up streaming with the right count, the
// source never restarts, and the keyframe requests are coalesced. A third
// viewer stays throughout so the source keeps running.
func TestFanout_ReconnectWhileJoining(t *testing.T) {
	clk := newFakeClock()
	env := newTracerServer(t, withClock(clk))
	tok := env.token(t, "g1")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	z := env.join(ctx, t, tok)
	requireKeyframe(t, "Z", z.firstAU(t, 2*time.Second))
	clk.Advance(2 * desktop.KeyframeCoalesce)
	a := env.join(ctx, t, tok)
	requireKeyframe(t, "A", a.firstAU(t, 2*time.Second))
	clk.Advance(2 * desktop.KeyframeCoalesce)
	before := env.lastSource().KeyframesForced()

	var a2, b *fanViewer
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { // A closes and joins again. Failures here end only this goroutine.
		defer wg.Done()
		_ = a.conn.Close(websocket.StatusNormalClosure, "")
		_ = a.lc.pc.Close()
		a2 = env.join(ctx, t, tok)
	}()
	go func() {
		defer wg.Done()
		b = env.join(ctx, t, tok)
	}()
	wg.Wait()
	if a2 == nil || b == nil {
		t.Fatal("a viewer failed to join")
	}

	a2.keyframeWithin(t, "A again", 5*time.Second)
	b.keyframeWithin(t, "B", 5*time.Second)
	for name, v := range map[string]*fanViewer{"Z": z, "A again": a2, "B": b} {
		eventually(t, 3*time.Second, name+" sees 3 viewers", func() bool { return v.stateViewers() == 3 })
	}
	if n := env.mgr.Viewers(); n != 3 {
		t.Errorf("Viewers() = %d, want 3", n)
	}
	if n := env.sourceCount(); n != 1 {
		t.Errorf("source started %d times, want 1", n)
	}
	time.Sleep(1500 * time.Millisecond) // let a deferred keyframe fire
	if n := env.lastSource().KeyframesForced() - before; n < 1 || n > 2 {
		t.Errorf("%d keyframes forced for two simultaneous joins, want 1 or 2 (coalesced)", n)
	}
}

// TestFanout_PLIStormCoalesced: 50 PLIs inside one window cost at most two
// forced keyframes (one at once, one deferred to the window's end).
func TestFanout_PLIStormCoalesced(t *testing.T) {
	clk := newFakeClock()
	env := newTracerServer(t, withClock(clk))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	v := env.join(ctx, t, env.token(t, "g1"))
	requireKeyframe(t, "viewer", v.firstAU(t, 2*time.Second))
	clk.Advance(2 * desktop.KeyframeCoalesce)
	before := env.lastSource().KeyframesForced()

	ssrc := v.lc.ssrc.Load()
	if ssrc == 0 {
		t.Fatal("client never learned the track SSRC")
	}
	for range 50 {
		if err := v.lc.pc.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: ssrc}}); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(1500 * time.Millisecond) // past the deferred keyframe
	n := env.lastSource().KeyframesForced() - before
	if n < 1 || n > 2 {
		t.Errorf("50 PLIs forced %d keyframes, want 1 or 2", n)
	}
}

// WR-02: each unit carries the gap since the previous one, so its RTP stamp
// must be the previous stamp plus its own gap: a frame after a 5 s idle gap is
// stamped 5 s later, not the frame after it. A gap past the 32-bit tick range
// (about 13.25 h) wraps instead of saturating.
func TestFanout_RTPTimestampFollowsCaptureTime(t *testing.T) {
	pkt := rtp.NewPacketizer(1200, 96, 1, &codecs.H264Payloader{}, rtp.NewRandomSequencer(), 90000)
	unit := func(d time.Duration) uint32 {
		ps := desktop.PacketizeForTest(pkt, screencap.AccessUnit{Data: []byte{0, 0, 0, 1, 0x41, 0x9a}, Duration: d})
		if len(ps) == 0 {
			t.Fatal("no packets")
		}
		return ps[0].Timestamp
	}
	a := unit(33 * time.Millisecond)
	b := unit(5 * time.Second) // arrived after a 5 s still screen
	c := unit(33 * time.Millisecond)
	if got := b - a; got != 450000 {
		t.Errorf("stamp step after a 5 s gap = %d ticks, want 450000", got)
	}
	if got := c - b; got != 2970 {
		t.Errorf("stamp step after a 33 ms gap = %d ticks, want 2970", got)
	}
	long := 14 * time.Hour
	d := unit(long)
	if got, want := d-c, uint32(uint64(long/time.Second)*90000); got != want {
		t.Errorf("stamp step after a 14 h gap = %d, want %d (mod 2^32)", got, want)
	}
}

// WR-06: a closed Errors channel is not an error. The fan-out stops reading it
// instead of logging "capture error <nil>" on every pass (a busy loop when
// only that channel is closed), and frames keep flowing.
func TestFanout_ClosedErrorsChannelIsQuiet(t *testing.T) {
	logs := captureLogs(t)
	r := newRig(t)
	r.join()
	r.src.last().closeErrs()
	time.Sleep(100 * time.Millisecond)
	if n := logs.count("capture error"); n != 0 {
		t.Fatalf("logged %d capture errors for a closed Errors channel, want 0", n)
	}
	before := r.env.mgr.FramesSeenForTest()
	r.src.last().emit(false)
	eventually(t, 5*time.Second, "a frame after the Errors channel closed", func() bool { return r.env.mgr.FramesSeenForTest() > before })
}
