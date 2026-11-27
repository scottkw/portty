package desktop_test

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/webrtc/v4"

	"github.com/scottkw/portty/internal/desktop"
	"github.com/scottkw/portty/internal/screencap"
)

// bridgeCall is one recorded call of the fake input bridge.
type bridgeCall struct {
	Op      string // "mouse", "key", "text", "scroll", "cursor"
	Kind    screencap.MouseKind
	X, Y    float64
	Button  int
	Click   int
	DX, DY  int
	Flags   uint64
	VK      uint16
	Down    bool
	Repeat  bool
	Units   []uint16
	CursorX float64
	CursorY float64
}

// fakeInputBridge is the one fake desktop.InputBridge for this package's tests:
// it records every call, including the key, text and scroll posts later plans
// add to the interface, so no test posts an event on the host and no plan
// defines a second fake.
type fakeInputBridge struct {
	mu     sync.Mutex
	calls  []bridgeCall
	curX   float64
	curY   float64
	curErr error
	stuck  bool // the pointer ignores posted moves (Accessibility not granted)
}

func (f *fakeInputBridge) record(c bridgeCall) {
	f.mu.Lock()
	f.calls = append(f.calls, c)
	f.mu.Unlock()
}

func (f *fakeInputBridge) PostMouse(kind screencap.MouseKind, x, y float64, button, clickState, dx, dy int, flags uint64) error {
	f.mu.Lock()
	if !f.stuck && (kind == screencap.MouseMoved || kind == screencap.MouseDragged) {
		f.curX, f.curY = x, y
	}
	f.mu.Unlock()
	f.record(bridgeCall{Op: "mouse", Kind: kind, X: x, Y: y, Button: button, Click: clickState, DX: dx, DY: dy, Flags: flags})
	return nil
}

func (f *fakeInputBridge) PostKey(vk uint16, down bool, flags uint64, autorepeat bool) error {
	f.record(bridgeCall{Op: "key", VK: vk, Down: down, Flags: flags, Repeat: autorepeat})
	return nil
}

func (f *fakeInputBridge) PostText(units []uint16) error {
	f.record(bridgeCall{Op: "text", Units: append([]uint16(nil), units...)})
	return nil
}

func (f *fakeInputBridge) PostScroll(dx, dy int, flags uint64) error {
	f.record(bridgeCall{Op: "scroll", DX: dx, DY: dy, Flags: flags})
	return nil
}

func (f *fakeInputBridge) CursorLocation() (x, y float64, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.curX, f.curY, f.curErr
}

func (f *fakeInputBridge) snapshot() []bridgeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]bridgeCall(nil), f.calls...)
}

// waitFor waits until at least n calls are recorded and returns them.
func (f *fakeInputBridge) waitFor(t *testing.T, n int) []bridgeCall {
	t.Helper()
	var got []bridgeCall
	eventually(t, 5*time.Second, "bridge calls", func() bool {
		got = f.snapshot()
		return len(got) >= n
	})
	return got
}

// testDisplay is a display left of the main one, to prove negative origins.
func testDisplay(uint32) (x, y, w, h float64, err error) { return -1470, 0, 1470, 956, nil }

// withFakeInput injects the fake bridge, the test display and a switchable
// lock state (read through clk, because Locked() caches for 250 ms).
func withFakeInput(fb *fakeInputBridge, locked *atomic.Bool, clk *fakeClock) tracerOpt {
	return func(_ *tracerEnv, d *desktop.Deps) {
		d.Input = fb
		d.DisplayBounds = testDisplay
		d.Lock = func() (bool, error) { return locked.Load(), nil }
		d.Now = clk.Now
	}
}

// setLocked switches the lock state and steps the clock past the cache.
func setLocked(locked *atomic.Bool, clk *fakeClock, v bool) {
	locked.Store(v)
	clk.Advance(300 * time.Millisecond)
}

// inputViewer is a fanViewer that declared caps input 1 and opened the "input"
// channel before its offer.
type inputViewer struct {
	*fanViewer
	dc   *webrtc.DataChannel
	open chan struct{}

	mu   sync.Mutex
	msgs []string // what the daemon sent on the channel, in order
}

// received returns a copy of the daemon's messages so far.
func (iv *inputViewer) received() []string {
	iv.mu.Lock()
	defer iv.mu.Unlock()
	return append([]string(nil), iv.msgs...)
}

func (e *tracerEnv) joinInput(ctx context.Context, t *testing.T, tok string) *inputViewer {
	t.Helper()
	iv := &inputViewer{open: make(chan struct{})}
	iv.fanViewer = e.joinCaps(ctx, t, tok, map[string]int{"video": 1, "input": 1}, func(lc *loopbackClient) {
		dc, err := lc.pc.CreateDataChannel("input", nil)
		if err != nil {
			t.Fatal(err)
		}
		dc.OnOpen(func() { close(iv.open) })
		dc.OnMessage(func(m webrtc.DataChannelMessage) {
			iv.mu.Lock()
			iv.msgs = append(iv.msgs, string(m.Data))
			iv.mu.Unlock()
		})
		iv.dc = dc
	})
	select {
	case <-iv.open:
	case <-time.After(10 * time.Second):
		t.Fatal("the input channel did not open")
	}
	return iv
}

// send writes one input message on the channel.
func (iv *inputViewer) send(t *testing.T, msg map[string]any) {
	t.Helper()
	b, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	if err := iv.dc.SendText(string(b)); err != nil {
		t.Fatal(err)
	}
}

func counter(env *tracerEnv, name string) uint64 { return env.mgr.InputCounters()[name] }

// TestInputTracer_ClickReachesPoster: a click sent on the input channel by a
// loopback viewer reaches the bridge as a move, a left-down with click state 1
// and a left-up at the right global point of a display with a negative origin;
// a stale epoch and a locked screen drop presses, a held button's release still
// posts, a viewer that did not declare input has its channel closed, and a
// viewer that leaves with a button held gets it released.
func TestInputTracer_ClickReachesPoster(t *testing.T) {
	fb := &fakeInputBridge{}
	var locked atomic.Bool
	clk := newFakeClock()
	env := newTracerServer(t, withFakeInput(fb, &locked, clk))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	tok := env.token(t, "g1")
	v := env.joinInput(ctx, t, tok)

	v.send(t, map[string]any{"t": "move", "x": 0.5, "y": 0.25, "e": 1})
	v.send(t, map[string]any{"t": "btn", "b": 0, "d": true, "x": 0.5, "y": 0.25, "e": 1, "ts": 1000})
	v.send(t, map[string]any{"t": "btn", "b": 0, "d": false, "x": 0.5, "y": 0.25, "e": 1, "ts": 1080})
	calls := fb.waitFor(t, 3)
	want := []struct {
		kind  screencap.MouseKind
		click int
	}{{screencap.MouseMoved, 0}, {screencap.MouseDown, 1}, {screencap.MouseUp, 1}}
	for i, w := range want {
		c := calls[i]
		if c.Op != "mouse" || c.Kind != w.kind || c.Click != w.click || c.Button != 0 || c.X != -735 || c.Y != 239 {
			t.Errorf("call %d = %+v, want mouse kind %d click %d button 0 at (-735, 239)", i, c, w.kind, w.click)
		}
	}

	// A stale epoch is dropped and counted.
	v.send(t, map[string]any{"t": "move", "x": 0.1, "y": 0.1, "e": 2})
	eventually(t, 5*time.Second, "stale_epoch counted", func() bool { return counter(env, "stale_epoch") == 1 })
	if n := len(fb.snapshot()); n != 3 {
		t.Errorf("%d bridge calls after a stale-epoch move, want 3", n)
	}

	// Locked: a press is dropped, a held button's release still posts.
	v.send(t, map[string]any{"t": "btn", "b": 0, "d": true, "x": 0.5, "y": 0.25, "e": 1})
	fb.waitFor(t, 4)
	setLocked(&locked, clk, true)
	v.send(t, map[string]any{"t": "btn", "b": 2, "d": true, "x": 0.5, "y": 0.25, "e": 1})
	eventually(t, 5*time.Second, "locked counted", func() bool { return counter(env, "locked") == 1 })
	v.send(t, map[string]any{"t": "btn", "b": 0, "d": false, "x": 0.5, "y": 0.25, "e": 1})
	calls = fb.waitFor(t, 5)
	if c := calls[4]; c.Kind != screencap.MouseUp || c.Button != 0 {
		t.Errorf("release under lock = %+v, want a left MouseUp", c)
	}
	if len(calls) != 5 {
		t.Errorf("%d bridge calls, want 5 (the locked press must not post)", len(calls))
	}

	// A viewer that did not declare input gets its channel closed.
	closed := make(chan struct{})
	env.join(ctx, t, tok, func(lc *loopbackClient) {
		dc, err := lc.pc.CreateDataChannel("input", nil)
		if err != nil {
			t.Fatal(err)
		}
		dc.OnClose(func() { close(closed) })
	})
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("a channel from a viewer without caps input was not closed within 2s")
	}

	// A viewer that leaves with a button held releases it.
	setLocked(&locked, clk, false)
	v.send(t, map[string]any{"t": "btn", "b": 0, "d": true, "x": 0.5, "y": 0.25, "e": 1})
	fb.waitFor(t, 6)
	_ = v.conn.CloseNow()
	calls = fb.waitFor(t, 7)
	if c := calls[6]; c.Kind != screencap.MouseUp || c.Button != 0 || c.X != -735 || c.Y != 239 {
		t.Errorf("release on leave = %+v, want a left MouseUp at (-735, 239)", c)
	}
}

// TestInput_EpochBumpDropsOldCoordinates: after a surface change bumps the
// epoch, a move sent with the old epoch is dropped and one with the new posts.
func TestInput_EpochBumpDropsOldCoordinates(t *testing.T) {
	fb := &fakeInputBridge{}
	var locked atomic.Bool
	env := newTracerServer(t, withFakeInput(fb, &locked, newFakeClock()))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	v := env.joinInput(ctx, t, env.token(t, "g1"))

	if e := env.mgr.BumpEpochForTest(); e != 2 {
		t.Fatalf("epoch after one bump = %d, want 2", e)
	}
	v.send(t, map[string]any{"t": "move", "x": 0.5, "y": 0.5, "e": 1})
	// Sent after the first is handled: a queued move is replaced by a later one.
	eventually(t, 5*time.Second, "stale_epoch counted", func() bool { return counter(env, "stale_epoch") == 1 })
	v.send(t, map[string]any{"t": "move", "x": 0.5, "y": 0.5, "e": 2})
	fb.waitFor(t, 1)
	if n := len(fb.snapshot()); n != 1 {
		t.Errorf("%d bridge calls, want 1 (only the current-epoch move)", n)
	}
}

// TestInput_CloseReleasesHeld: Manager.Close posts every held release before it
// returns.
func TestInput_CloseReleasesHeld(t *testing.T) {
	fb := &fakeInputBridge{}
	var locked atomic.Bool
	env := newTracerServer(t, withFakeInput(fb, &locked, newFakeClock()))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	v := env.joinInput(ctx, t, env.token(t, "g1"))

	v.send(t, map[string]any{"t": "btn", "b": 2, "d": true, "x": 0.5, "y": 0.25, "e": 1})
	fb.waitFor(t, 1)
	if err := env.mgr.Close(); err != nil {
		t.Fatal(err)
	}
	calls := fb.snapshot()
	if len(calls) != 2 || calls[1].Kind != screencap.MouseUp || calls[1].Button != 1 {
		t.Errorf("calls after Close = %+v, want the right-button press then its MouseUp", calls)
	}
}

// TestInputTracer_KeyHeldReleasedOnClose: a key typed in the viewer is pressed
// with the right kVK and flags, released when its input channel closes, and a
// closing socket releases keys before the modifiers that were held under them.
func TestInputTracer_KeyHeldReleasedOnClose(t *testing.T) {
	fb := &fakeInputBridge{}
	var locked atomic.Bool
	env := newTracerServer(t, withFakeInput(fb, &locked, newFakeClock()))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	v := env.joinInput(ctx, t, env.token(t, "g1"))
	v.send(t, map[string]any{"t": "key", "code": "KeyA", "d": true, "r": false, "m": 0, "k": "a"})
	calls := fb.waitFor(t, 1)
	if c := calls[0]; c.Op != "key" || c.VK != 0x00 || !c.Down || c.Flags != 0 || c.Repeat {
		t.Fatalf("key down = %+v, want KeyA (0x00) down with flags 0", c)
	}
	if err := v.dc.Close(); err != nil {
		t.Fatal(err)
	}
	calls = fb.waitFor(t, 2)
	if c := calls[1]; c.Op != "key" || c.VK != 0x00 || c.Down {
		t.Errorf("after the channel closed: %+v, want KeyA up", c)
	}

	v2 := env.joinInput(ctx, t, env.token(t, "g1"))
	v2.send(t, map[string]any{"t": "key", "code": "MetaLeft", "d": true, "m": 8})
	v2.send(t, map[string]any{"t": "key", "code": "KeyA", "d": true, "m": 8})
	fb.waitFor(t, 4)
	_ = v2.conn.CloseNow()
	calls = fb.waitFor(t, 6)
	meta, a := uint16(0x37), uint16(0x00)
	want := []struct {
		vk    uint16
		down  bool
		flags uint64
	}{{meta, true, screencap.FlagCommand}, {a, true, screencap.FlagCommand}, {a, false, screencap.FlagCommand}, {meta, false, 0}}
	for i, w := range want {
		if c := calls[2+i]; c.Op != "key" || c.VK != w.vk || c.Down != w.down || c.Flags != w.flags {
			t.Errorf("call %d = %+v, want key %#x down=%v flags %#x", 2+i, c, w.vk, w.down, w.flags)
		}
	}
}

// TestInput_TextChunksToBridge: text is posted in chunks of at most 20 UTF-16
// units; an empty text and one over 1024 bytes post nothing and are counted.
func TestInput_TextChunksToBridge(t *testing.T) {
	fb := &fakeInputBridge{}
	var locked atomic.Bool
	env := newTracerServer(t, withFakeInput(fb, &locked, newFakeClock()))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	v := env.joinInput(ctx, t, env.token(t, "g1"))

	v.send(t, map[string]any{"t": "text", "s": strings.Repeat("a", 45)})
	v.send(t, map[string]any{"t": "text", "s": ""})
	v.send(t, map[string]any{"t": "text", "s": strings.Repeat("a", 1025)})
	v.send(t, map[string]any{"t": "text", "s": "\u00e9"})
	calls := fb.waitFor(t, 4)
	for i, n := range []int{20, 20, 5, 1} {
		if c := calls[i]; c.Op != "text" || len(c.Units) != n {
			t.Errorf("call %d = %+v, want a text post of %d units", i, c, n)
		}
	}
	if n := counter(env, "malformed"); n != 2 {
		t.Errorf("malformed = %d, want 2 (the empty and the 1025-byte text)", n)
	}
	if n := len(fb.snapshot()); n != 4 {
		t.Errorf("%d bridge calls, want 4", n)
	}
}

// syncInput is a Manager with the fake bridge and one unregistered viewer whose
// messages are handled synchronously: gate and counter tests with no network.
type syncInput struct {
	env    *tracerEnv
	fb     *fakeInputBridge
	clk    *fakeClock
	locked *atomic.Bool
	v      *desktop.InputViewerForTest
}

func newSyncInput(t *testing.T) *syncInput {
	t.Helper()
	s := &syncInput{fb: &fakeInputBridge{}, clk: newFakeClock(), locked: &atomic.Bool{}}
	s.env = newTracerServer(t, withFakeInput(s.fb, s.locked, s.clk))
	s.v = desktop.NewInputViewerForTest("viewer-0001", desktop.RightsOwnAccount)
	return s
}

func (s *syncInput) do(t *testing.T, format string, args ...any) {
	t.Helper()
	if err := s.env.mgr.HandleInputForTest(s.v, fmt.Sprintf(format, args...)); err != nil {
		t.Fatal(err)
	}
}

// TestInput_ReleasesBypassGates: with a Shift key and a button held, each gate in
// turn drops new presses with its own reason while the releases still post.
func TestInput_ReleasesBypassGates(t *testing.T) {
	s := newSyncInput(t)
	mgr := s.env.mgr
	epoch := 1
	gates := []struct {
		reason string
		on     func()
		off    func()
	}{
		{"locked", func() { setLocked(s.locked, s.clk, true) }, func() { setLocked(s.locked, s.clk, false) }},
		{"stale_epoch", func() { mgr.BumpEpochForTest(); epoch++ }, func() {}},
		{"no_right", func() { s.v.Rights = desktop.RightsViewOnly }, func() { s.v.Rights = desktop.RightsOwnAccount }},
		{"no_accessibility", func() { mgr.SetControlInputForTest(desktop.ControlNeedsAccessibility) }, func() { mgr.SetControlInputForTest(desktop.ControlOK) }},
		{"rate", func() {
			for range 300 {
				s.do(t, `{"t":"move","x":0.5,"y":0.5,"e":%d}`, epoch)
			}
		}, func() { s.clk.Advance(time.Second) }},
	}
	for _, g := range gates {
		s.do(t, `{"t":"key","code":"ShiftLeft","d":true,"m":1}`)
		s.do(t, `{"t":"btn","b":0,"d":true,"x":0.5,"y":0.5,"e":%d}`, epoch)
		g.on()
		before, n0 := counter(s.env, g.reason), len(s.fb.snapshot())
		badEpoch := epoch
		if g.reason == "stale_epoch" {
			badEpoch = epoch - 1
		}
		s.do(t, `{"t":"btn","b":2,"d":true,"x":0.5,"y":0.5,"e":%d}`, badEpoch)
		if n := len(s.fb.snapshot()); n != n0 {
			t.Errorf("%s: %d presses posted under the gate", g.reason, n-n0)
		}
		if got := counter(s.env, g.reason); got < before+1 {
			t.Errorf("%s: counter %d, want a drop counted (was %d)", g.reason, got, before)
		}
		s.do(t, `{"t":"btn","b":0,"d":false,"x":0.5,"y":0.5,"e":%d}`, badEpoch)
		s.do(t, `{"t":"key","code":"ShiftLeft","d":false,"m":0}`)
		var up, keyUp bool
		for _, c := range s.fb.snapshot()[n0:] {
			up = up || (c.Op == "mouse" && c.Kind == screencap.MouseUp && c.Button == 0)
			keyUp = keyUp || (c.Op == "key" && c.VK == 0x38 && !c.Down)
		}
		if !up || !keyUp {
			t.Errorf("%s: releases under the gate: mouse up %v, shift up %v", g.reason, up, keyUp)
		}
		g.off()
	}
}

// TestInput_LockGateAndReleaseOnLock: a held key is released when the screen
// locks with no message from the viewer, and presses stay dropped while locked.
func TestInput_LockGateAndReleaseOnLock(t *testing.T) {
	fb := &fakeInputBridge{}
	var locked atomic.Bool
	clk := newFakeClock()
	env := newTracerServer(t, withFakeInput(fb, &locked, clk))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	v := env.joinInput(ctx, t, env.token(t, "g1"))

	v.send(t, map[string]any{"t": "key", "code": "KeyA", "d": true, "m": 0})
	fb.waitFor(t, 1)
	setLocked(&locked, clk, true)
	calls := fb.waitFor(t, 2)
	if c := calls[1]; c.Op != "key" || c.VK != 0 || c.Down {
		t.Errorf("after the lock: %+v, want KeyA up", c)
	}
	v.send(t, map[string]any{"t": "key", "code": "KeyB", "d": true, "m": 0})
	eventually(t, 5*time.Second, "locked counted", func() bool { return counter(env, "locked") >= 1 })
	if n := len(fb.snapshot()); n != 2 {
		t.Errorf("%d bridge calls, want 2 (the locked press must not post)", n)
	}
	setLocked(&locked, clk, false)
	v.send(t, map[string]any{"t": "key", "code": "KeyB", "d": true, "m": 0})
	if c := fb.waitFor(t, 3)[2]; c.VK != 0x0B || !c.Down {
		t.Errorf("after unlock: %+v, want KeyB down", c)
	}
}

func keyDownPosted(fb *fakeInputBridge, vk uint16) bool {
	for _, c := range fb.snapshot() {
		if c.Op == "key" && c.VK == vk && c.Down {
			return true
		}
	}
	return false
}

// TestInput_OptimisticLockOnChord: posting Ctrl+Cmd+Q refuses presses at once,
// before the lock poll could see the lock, and for 3 s only.
func TestInput_OptimisticLockOnChord(t *testing.T) {
	fb := &fakeInputBridge{}
	var locked atomic.Bool
	clk := newFakeClock()
	env := newTracerServer(t, withFakeInput(fb, &locked, clk))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	v := env.joinInput(ctx, t, env.token(t, "g1"))

	v.send(t, map[string]any{"t": "key", "code": "ControlLeft", "d": true, "m": 2})
	v.send(t, map[string]any{"t": "key", "code": "MetaLeft", "d": true, "m": 10})
	v.send(t, map[string]any{"t": "key", "code": "KeyQ", "d": true, "m": 10})
	fb.waitFor(t, 3)
	v.send(t, map[string]any{"t": "key", "code": "KeyA", "d": true, "m": 10})
	eventually(t, 5*time.Second, "locked counted", func() bool { return counter(env, "locked") == 1 })
	if keyDownPosted(fb, 0x00) {
		t.Error("KeyA posted during the optimistic lock although Locked() reads false")
	}
	clk.Advance(3100 * time.Millisecond)
	v.send(t, map[string]any{"t": "key", "code": "KeyA", "d": true, "m": 0})
	eventually(t, 5*time.Second, "KeyA posted after the lock window", func() bool { return keyDownPosted(fb, 0x00) })
}

// TestInput_DeadManReleases: a viewer that holds a key and sends nothing for 3 s
// (by Deps.Now) has it released; a heartbeat pushes the deadline.
func TestInput_DeadManReleases(t *testing.T) {
	fb := &fakeInputBridge{}
	var locked atomic.Bool
	clk := newFakeClock()
	env := newTracerServer(t, withFakeInput(fb, &locked, clk))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	v := env.joinInput(ctx, t, env.token(t, "g1"))

	v.send(t, map[string]any{"t": "key", "code": "KeyA", "d": true, "m": 0})
	fb.waitFor(t, 1)
	clk.Advance(2900 * time.Millisecond)
	time.Sleep(700 * time.Millisecond) // two input ticks
	if n := len(fb.snapshot()); n != 1 {
		t.Fatalf("%d calls at 2.9 s, want 1 (still held)", n)
	}
	clk.Advance(100 * time.Millisecond)
	if c := fb.waitFor(t, 2)[1]; c.Op != "key" || c.Down {
		t.Errorf("at 3.0 s: %+v, want KeyA up", c)
	}

	// The viewer can input again afterwards, and a heartbeat pushes the deadline.
	v.send(t, map[string]any{"t": "key", "code": "KeyA", "d": true, "m": 0})
	fb.waitFor(t, 3)
	clk.Advance(2500 * time.Millisecond)
	v.send(t, map[string]any{"t": "hb"})
	eventually(t, 5*time.Second, "hb counted", func() bool { return counter(env, "hb") == 1 })
	clk.Advance(1500 * time.Millisecond) // 4.0 s since the key, 1.5 s since the hb
	time.Sleep(700 * time.Millisecond)
	if n := len(fb.snapshot()); n != 3 {
		t.Fatalf("%d calls 1.5 s after a heartbeat, want 3 (still held)", n)
	}
	clk.Advance(1500 * time.Millisecond)
	if c := fb.waitFor(t, 4)[3]; c.Op != "key" || c.Down {
		t.Errorf("3 s after the heartbeat: %+v, want KeyA up", c)
	}
}

// TestInput_ModifierReconcile: a key message whose mask says Cmd is up releases a
// Cmd the viewer still holds before the key posts.
func TestInput_ModifierReconcile(t *testing.T) {
	s := newSyncInput(t)
	s.do(t, `{"t":"key","code":"MetaLeft","d":true,"m":8}`)
	s.do(t, `{"t":"key","code":"KeyC","d":true,"m":0}`)
	calls := s.fb.snapshot()
	if len(calls) != 3 {
		t.Fatalf("%d calls, want 3: %+v", len(calls), calls)
	}
	if c := calls[1]; c.VK != 0x37 || c.Down || c.Flags != 0 {
		t.Errorf("call 1 = %+v, want Meta up with no flags", c)
	}
	if c := calls[2]; c.VK != 0x08 || !c.Down || c.Flags != 0 {
		t.Errorf("call 2 = %+v, want KeyC down with no flags", c)
	}
	if n := counter(s.env, "reconciled"); n != 1 {
		t.Errorf("reconciled = %d, want 1", n)
	}
}

// TestInput_RateCapAndCounters: a burst of 300 moves posts 200 and counts the
// rest as rate drops, a release is never charged to the bucket, and no counter
// name carries a key code or a coordinate.
func TestInput_RateCapAndCounters(t *testing.T) {
	s := newSyncInput(t)
	s.do(t, `{"t":"btn","b":0,"d":true,"x":0.5,"y":0.5,"e":1}`) // one token
	for range 300 {
		s.do(t, `{"t":"move","x":0.5,"y":0.5,"e":1}`)
	}
	s.do(t, `{"t":"key","code":"KeyZ","d":true,"m":0}`) // dropped: rate
	if n := counter(s.env, "rate"); n != 102 {
		t.Errorf("rate drops = %d, want 102 (101 moves and a key)", n)
	}
	n := len(s.fb.snapshot())
	s.do(t, `{"t":"btn","b":0,"d":false,"x":0.5,"y":0.5,"e":1}`)
	if got := len(s.fb.snapshot()); got != n+1 {
		t.Errorf("release with an empty bucket posted %d calls, want 1", got-n)
	}
	if n != 200 {
		t.Errorf("%d calls before the release, want 200 (burst)", n)
	}
	plain := regexp.MustCompile(`^[a-z_]+$`)
	for name := range s.env.mgr.InputCounters() {
		if !plain.MatchString(name) {
			t.Errorf("counter name %q is not a plain kind or reason", name)
		}
	}
}

// TestInput_PointerWheelAndClicks: clicks count from viewer timestamps and the
// up reuses the down's count, a move with a button held drags, a relative move
// integrates from the last point, and the wheel posts inverted pixels with the
// held modifiers.
func TestInput_PointerWheelAndClicks(t *testing.T) {
	s := newSyncInput(t)
	s.do(t, `{"t":"btn","b":0,"d":true,"x":0.5,"y":0.25,"e":1,"ts":1000}`)
	s.do(t, `{"t":"btn","b":0,"d":false,"x":0.5,"y":0.25,"e":1,"ts":1050}`)
	s.do(t, `{"t":"btn","b":0,"d":true,"x":0.5,"y":0.25,"e":1,"ts":1300}`)
	s.do(t, `{"t":"move","x":0.6,"y":0.25,"e":1}`)
	s.do(t, `{"t":"btn","b":0,"d":false,"x":0.6,"y":0.25,"e":1,"ts":1400}`)
	s.do(t, `{"t":"rmove","dx":0.1,"dy":0,"e":1}`)
	s.do(t, `{"t":"key","code":"ShiftLeft","d":true,"m":1}`)
	s.do(t, `{"t":"wheel","dx":0,"dy":120,"e":1}`)
	s.do(t, `{"t":"wheel","dx":0,"dy":0,"e":1}`) // nothing to scroll
	calls := s.fb.snapshot()
	type want struct {
		kind  screencap.MouseKind
		click int
		x     float64
	}
	for i, w := range []want{
		{screencap.MouseDown, 1, -735}, {screencap.MouseUp, 1, -735},
		{screencap.MouseDown, 2, -735}, {screencap.MouseDragged, 0, -588},
		{screencap.MouseUp, 2, -588}, {screencap.MouseMoved, 0, -441},
	} {
		c := calls[i]
		if c.Op != "mouse" || c.Kind != w.kind || c.Click != w.click || c.X != w.x {
			t.Errorf("call %d = %+v, want %+v", i, c, w)
		}
	}
	if len(calls) != 8 {
		t.Fatalf("%d calls, want 8: %+v", len(calls), calls)
	}
	if c := calls[7]; c.Op != "scroll" || c.DX != 0 || c.DY != -120 || c.Flags != screencap.FlagShift {
		t.Errorf("wheel = %+v, want scroll (0, -120) with Shift", c)
	}
}
