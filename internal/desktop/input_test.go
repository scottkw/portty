package desktop_test

import (
	"context"
	"encoding/json"
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
}

func (f *fakeInputBridge) record(c bridgeCall) {
	f.mu.Lock()
	f.calls = append(f.calls, c)
	f.mu.Unlock()
}

func (f *fakeInputBridge) PostMouse(kind screencap.MouseKind, x, y float64, button, clickState, dx, dy int, flags uint64) error {
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
