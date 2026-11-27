package desktop_test

import (
	"context"
	"encoding/json"
	"math"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/desktop"
	"github.com/scottkw/portty/internal/screencap"
)

const (
	controlOKMsg    = `{"t":"control","input":"ok","window":""}`
	controlNeedsMsg = `{"t":"control","input":"needs_accessibility","window":""}`
)

// withNudge injects a fake Deps.Nudge and counts its calls, so a recovery test
// never touches the pointer.
func withNudge(result *atomic.Bool, calls *atomic.Int32) tracerOpt {
	return func(_ *tracerEnv, d *desktop.Deps) {
		d.Nudge = func() (bool, error) {
			calls.Add(1)
			return result.Load(), nil
		}
	}
}

func joinWithControl(t *testing.T, fb *fakeInputBridge, extra ...tracerOpt) (*tracerEnv, *inputViewer, *fakeClock, *atomic.Bool) {
	t.Helper()
	var locked atomic.Bool
	clk := newFakeClock()
	env := newTracerServer(t, append([]tracerOpt{withFakeInput(fb, &locked, clk)}, extra...)...)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	return env, env.joinInput(ctx, t, env.token(t, "g1")), clk, &locked
}

// moveAndWait sends one plain move and waits for its effect check to be counted
// (a hit or a miss), so the next move starts a new check.
func moveAndWait(t *testing.T, env *tracerEnv, v *inputViewer, x float64) {
	t.Helper()
	before := counter(env, "effect_hit") + counter(env, "effect_miss")
	v.send(t, map[string]any{"t": "move", "x": x, "y": 0.5, "e": 1})
	eventually(t, 5*time.Second, "effect check", func() bool {
		return counter(env, "effect_hit")+counter(env, "effect_miss") > before
	})
}

func eventuallyReceived(t *testing.T, v *inputViewer, want string) {
	t.Helper()
	eventually(t, 5*time.Second, "message "+want, func() bool { return slices.Contains(v.received(), want) })
}

// TestControl_NeedsAccessibilityAfterThreeMisses: three posted moves that the
// pointer never follows set needs_accessibility, tell the viewer, and make
// presses drop with their own reason.
func TestControl_NeedsAccessibilityAfterThreeMisses(t *testing.T) {
	fb := &fakeInputBridge{stuck: true}
	env, v, _, _ := joinWithControl(t, fb)
	eventuallyReceived(t, v, controlOKMsg)

	for i := range 2 {
		moveAndWait(t, env, v, 0.1*float64(i+1))
	}
	if in, _ := env.mgr.ControlState(); in != desktop.ControlOK {
		t.Fatalf("control input after two misses = %q, want ok", in)
	}
	moveAndWait(t, env, v, 0.5)
	eventuallyReceived(t, v, controlNeedsMsg)
	if in, win := env.mgr.ControlState(); in != desktop.ControlNeedsAccessibility || win != "" {
		t.Errorf("ControlState = (%q, %q), want (needs_accessibility, \"\")", in, win)
	}

	n := len(fb.snapshot())
	v.send(t, map[string]any{"t": "key", "code": "KeyA", "d": true, "m": 0})
	eventually(t, 5*time.Second, "no_accessibility counted", func() bool { return counter(env, "no_accessibility") == 1 })
	if got := len(fb.snapshot()); got != n {
		t.Errorf("%d events posted while control needs Accessibility, want 0", got-n)
	}
}

// TestControl_OffDisplayMovesAreNotMisses: moves onto a part of the shared
// surface that lies past every display edge are clamped by the WindowServer, so
// the read-back differs for a good reason; they never count as misses and never
// set needs_accessibility.
func TestControl_OffDisplayMovesAreNotMisses(t *testing.T) {
	fb := &fakeInputBridge{stuck: true}
	env, v, _, _ := joinWithControl(t, fb, func(_ *tracerEnv, d *desktop.Deps) {
		d.PointOnDisplay = func(float64, float64) (bool, error) { return false, nil }
	})
	eventuallyReceived(t, v, controlOKMsg)
	for i := range 4 {
		v.send(t, map[string]any{"t": "move", "x": 0.1 * float64(i+1), "y": 0.5, "e": 1})
		eventually(t, 5*time.Second, "off-display check", func() bool { return counter(env, "effect_offscreen") == uint64(i+1) })
	}
	if n := counter(env, "effect_miss"); n != 0 {
		t.Errorf("effect_miss = %d, want 0", n)
	}
	if in, _ := env.mgr.ControlState(); in != desktop.ControlOK {
		t.Errorf("control input = %q, want ok", in)
	}
}

// TestControl_TwoMissesThenHitStaysOK: a hit resets the miss count.
func TestControl_TwoMissesThenHitStaysOK(t *testing.T) {
	fb := &fakeInputBridge{stuck: true}
	env, v, _, _ := joinWithControl(t, fb)
	moveAndWait(t, env, v, 0.1)
	moveAndWait(t, env, v, 0.2)
	fb.mu.Lock()
	fb.stuck = false
	fb.mu.Unlock()
	moveAndWait(t, env, v, 0.3)
	fb.mu.Lock()
	fb.stuck = true
	fb.mu.Unlock()
	moveAndWait(t, env, v, 0.4)
	moveAndWait(t, env, v, 0.5)
	if in, _ := env.mgr.ControlState(); in != desktop.ControlOK {
		t.Errorf("control input = %q, want ok (two misses, a hit, two misses)", in)
	}
}

// TestControl_RecoversByNudge: while control needs Accessibility and a channel is
// open the daemon nudges the pointer every 5 s (fake clock) and a nudge that
// returns sets ok again; it never nudges while ok or with no channel open.
func TestControl_RecoversByNudge(t *testing.T) {
	var result atomic.Bool
	var calls atomic.Int32
	fb := &fakeInputBridge{}
	env, v, clk, _ := joinWithControl(t, fb, withNudge(&result, &calls))

	time.Sleep(700 * time.Millisecond)
	if n := calls.Load(); n != 0 {
		t.Fatalf("%d nudges while the state was ok, want 0", n)
	}
	env.mgr.SetControlInputForTest(desktop.ControlNeedsAccessibility)
	eventuallyReceived(t, v, controlNeedsMsg)
	eventually(t, 5*time.Second, "first nudge", func() bool { return calls.Load() == 1 })
	time.Sleep(700 * time.Millisecond)
	if n := calls.Load(); n != 1 {
		t.Fatalf("%d nudges within 5 s, want 1", n)
	}
	clk.Advance(5 * time.Second)
	eventually(t, 5*time.Second, "second nudge", func() bool { return calls.Load() == 2 })
	result.Store(true)
	clk.Advance(5 * time.Second)
	eventuallyReceived(t, v, controlOKMsg) // the second ok: the snapshot at open was the first
	eventually(t, 5*time.Second, "ok", func() bool {
		in, _ := env.mgr.ControlState()
		return in == desktop.ControlOK
	})
	if n := slices.Index(v.received(), controlNeedsMsg); n < 0 || slices.Index(v.received(), controlOKMsg) >= n {
		t.Errorf("messages = %v, want ok, needs_accessibility, ok", v.received())
	}
}

// TestControl_NoNudgeWithoutChannel: no input channel is open, so nothing nudges.
func TestControl_NoNudgeWithoutChannel(t *testing.T) {
	var result atomic.Bool
	var calls atomic.Int32
	var locked atomic.Bool
	env := newTracerServer(t, withFakeInput(&fakeInputBridge{}, &locked, newFakeClock()), withNudge(&result, &calls))
	env.mgr.SetControlInputForTest(desktop.ControlNeedsAccessibility)
	time.Sleep(700 * time.Millisecond)
	if n := calls.Load(); n != 0 {
		t.Errorf("%d nudges with no input channel open, want 0", n)
	}
}

// TestControl_SnapshotOnOpen: a channel that opens while control needs
// Accessibility gets that state as its first message.
func TestControl_SnapshotOnOpen(t *testing.T) {
	var locked atomic.Bool
	var result atomic.Bool
	var calls atomic.Int32
	env := newTracerServer(t, withFakeInput(&fakeInputBridge{}, &locked, newFakeClock()), withNudge(&result, &calls))
	env.mgr.SetControlInputForTest(desktop.ControlNeedsAccessibility)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	v := env.joinInput(ctx, t, env.token(t, "g1"))
	eventuallyReceived(t, v, controlNeedsMsg)
	if got := v.received()[0]; got != controlNeedsMsg {
		t.Errorf("first message = %s, want %s", got, controlNeedsMsg)
	}
}

// TestControl_WindowStatePush: a window value reaches the open channel with the
// input value beside it, and setting the same value again sends nothing.
func TestControl_WindowStatePush(t *testing.T) {
	env, v, _, _ := joinWithControl(t, &fakeInputBridge{})
	eventuallyReceived(t, v, controlOKMsg)
	env.mgr.SetControlWindowForTest(desktop.WindowRaiseFailed)
	eventuallyReceived(t, v, `{"t":"control","input":"ok","window":"raise_failed"}`)
	env.mgr.SetControlWindowForTest(desktop.WindowRaiseFailed)
	time.Sleep(200 * time.Millisecond)
	if n := len(v.received()); n != 2 {
		t.Errorf("%d messages, want 2 (a repeated value is not sent again)", n)
	}
	if in, win := env.mgr.ControlState(); in != desktop.ControlOK || win != desktop.WindowRaiseFailed {
		t.Errorf("ControlState = (%q, %q)", in, win)
	}
}

// TestControl_CursorReportAfterRmove: a relative move is answered with the
// posted point as normalized coordinates, at most every 33 ms.
func TestControl_CursorReportAfterRmove(t *testing.T) {
	fb := &fakeInputBridge{curX: -735, curY: 239}
	_, v, clk, _ := joinWithControl(t, fb)
	cursors := func() (out []msgCursorT) {
		for _, m := range v.received() {
			var c msgCursorT
			if json.Unmarshal([]byte(m), &c) == nil && c.T == "cursor" {
				out = append(out, c)
			}
		}
		return out
	}

	v.send(t, map[string]any{"t": "rmove", "dx": 0.1, "dy": 0, "e": 1})
	eventually(t, 5*time.Second, "cursor report", func() bool { return len(cursors()) == 1 })
	if c := cursors()[0]; math.Abs(c.X-0.6) > 1e-9 || math.Abs(c.Y-0.25) > 1e-3 || c.E != 1 {
		t.Errorf("cursor report = %+v, want x 0.6 y 0.25 e 1", c)
	}
	for range 9 {
		v.send(t, map[string]any{"t": "rmove", "dx": 0.001, "dy": 0, "e": 1})
	}
	fb.waitFor(t, 10)
	// The nine moves inside the 33 ms window get one trailing report, with the last point.
	eventually(t, 5*time.Second, "trailing report", func() bool { return len(cursors()) == 2 })
	time.Sleep(100 * time.Millisecond)
	if n := len(cursors()); n != 2 {
		t.Errorf("%d reports for 10 rmoves inside 33 ms, want 2 (the first and one trailing)", n)
	}
	last := fb.snapshot()[9]
	if c := cursors()[1]; math.Abs(c.X-(last.X+1470)/1470) > 1e-9 {
		t.Errorf("trailing report x = %v, want the last posted point %v", c.X, (last.X+1470)/1470)
	}
	clk.Advance(40 * time.Millisecond)
	v.send(t, map[string]any{"t": "rmove", "dx": 0.001, "dy": 0, "e": 1})
	eventually(t, 5*time.Second, "third report", func() bool { return len(cursors()) == 3 })
	if !strings.Contains(strings.Join(v.received(), ""), `"t":"cursor"`) {
		t.Error("no cursor message received")
	}
}

// TestControl_PhoneTapThenDrag is the iPhone's tap-then-drag as the daemon sees it
// (M-195): relative moves put the pointer on a title bar, the phone presses at the
// last cursor report, more relative moves drag, and a button-up drops. The press
// must land where the pointer is, not at a report older than the last move (the
// window-share title bar is 28 points tall), and every move while held is a drag.
func TestControl_PhoneTapThenDrag(t *testing.T) {
	fb := &fakeInputBridge{curX: -735, curY: 239}
	env, v, _, _ := joinWithControl(t, fb)
	cursors := func() (out []msgCursorT) {
		for _, m := range v.received() {
			var c msgCursorT
			if json.Unmarshal([]byte(m), &c) == nil && c.T == "cursor" {
				out = append(out, c)
			}
		}
		return out
	}

	// The pointer goes to the title bar: one reported move, then four quick ones inside 33 ms.
	v.send(t, map[string]any{"t": "rmove", "dx": 0.1, "dy": 0, "e": 1})
	for range 4 {
		v.send(t, map[string]any{"t": "rmove", "dx": 0.01, "dy": 0, "e": 1})
	}
	pos := fb.waitFor(t, 5)[4] // where the pointer is now
	var at msgCursorT
	eventually(t, 2*time.Second, "a report of where the pointer is", func() bool {
		cs := cursors()
		if len(cs) == 0 {
			return false
		}
		at = cs[len(cs)-1]
		return math.Abs(at.X*1470-1470-pos.X) < 0.5
	})

	// Tap then drag: the phone presses at the report, three moves, the lift.
	v.send(t, map[string]any{"t": "btn", "b": 0, "d": true, "x": at.X, "y": at.Y, "e": 1, "ts": 1000})
	for range 3 {
		v.send(t, map[string]any{"t": "rmove", "dx": 0.02, "dy": 0, "e": 1})
	}
	v.send(t, map[string]any{"t": "btn", "b": 0, "d": false, "x": at.X, "y": at.Y, "e": 1, "ts": 1100})
	calls := fb.waitFor(t, 10)[5:]
	if len(calls) != 5 {
		t.Fatalf("%d calls after the moves, want 5 (down, 3 drags, up): %+v", len(calls), calls)
	}
	if c := calls[0]; c.Kind != screencap.MouseDown || c.Button != 0 || c.X != pos.X || c.Y != pos.Y {
		t.Errorf("press = %+v, want a left down at the pointer (%v, %v)", c, pos.X, pos.Y)
	}
	x := pos.X
	for i, c := range calls[1:4] {
		if c.Kind != screencap.MouseDragged || c.Button != 0 || c.X <= x {
			t.Errorf("move %d while held = %+v, want a left drag right of %v", i, c, x)
		}
		x = c.X
	}
	if c := calls[4]; c.Kind != screencap.MouseUp || c.Button != 0 || c.X != x {
		t.Errorf("lift = %+v, want a left up at the last drag point %v", c, x)
	}
	if n := counter(env, "drag"); n != 3 {
		t.Errorf("drag counter = %d, want 3 (the summary tells a drag from a plain move)", n)
	}
}

type msgCursorT struct {
	T string  `json:"t"`
	X float64 `json:"x"`
	Y float64 `json:"y"`
	E uint32  `json:"e"`
}
