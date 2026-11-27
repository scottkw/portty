package desktop_test

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/desktop"
	"github.com/scottkw/portty/internal/screencap"
)

var windowSel = screencap.Surface{Kind: screencap.SurfaceWindow, ID: windowID}

// newWindowSync is a synchronous Manager (no network) that shares the window,
// with the shared window focused and on top.
func newWindowSync(t *testing.T) (*syncInput, *fakeWindow) {
	t.Helper()
	fw := newFakeWindow()
	s := &syncInput{fb: &fakeInputBridge{}, clk: newFakeClock(), locked: &atomic.Bool{}}
	s.env = newTracerServer(t, withFakeInput(s.fb, s.locked, s.clk), withWindow(fw))
	s.v = desktop.NewInputViewerForTest("viewer-0001", desktop.RightsOwnAccount)
	s.env.mgr.SetSurfaceForTest(windowSel, sharedWindow())
	fw.focusWindow()
	return s, fw
}

func press(x, y float64) string {
	return fmt.Sprintf(`{"t":"btn","x":%g,"y":%g,"e":1,"b":0,"d":true,"ts":1000}`, x, y)
}

// lastMouse returns the last mouse call the fake bridge saw.
func lastMouse(t *testing.T, fb *fakeInputBridge) bridgeCall {
	t.Helper()
	calls := fb.snapshot()
	for i := len(calls) - 1; i >= 0; i-- {
		if calls[i].Op == "mouse" {
			return calls[i]
		}
	}
	t.Fatal("no mouse call recorded")
	return bridgeCall{}
}

// TestWindow_ClampMoves: a move far past the right and top edges posts inside the frame.
func TestWindow_ClampMoves(t *testing.T) {
	s, _ := newWindowSync(t)
	s.do(t, `{"t":"move","x":1.4,"y":-0.2,"e":1}`)
	if c := lastMouse(t, s.fb); c.Kind != screencap.MouseMoved || c.X != 1379 || c.Y != 100 {
		t.Errorf("clamped move = %+v, want a move at (1379, 100)", c)
	}
}

// TestWindow_DropOutsidePress: the window moved since the last read, so the
// point the press was mapped to is outside the fresh frame; the press is dropped
// and the next one maps onto the new frame.
func TestWindow_DropOutsidePress(t *testing.T) {
	s, fw := newWindowSync(t)
	s.do(t, `{"t":"move","x":0.5,"y":0.5,"e":1}`) // reads the frame
	fw.set(func(f *fakeWindow) { f.infoFrame = [4]float64{1500, 100, 1280, 800} })
	fw.focusWindow()
	s.do(t, "%s", press(0.5, 0.5))
	if n := counter(s.env, "outside"); n != 1 {
		t.Fatalf("outside drops = %d, want 1", n)
	}
	if c := lastMouse(t, s.fb); c.Kind == screencap.MouseDown {
		t.Fatalf("a press outside the fresh frame posted: %+v", c)
	}
	s.do(t, "%s", press(0.5, 0.5))
	if c := lastMouse(t, s.fb); c.Kind != screencap.MouseDown || c.X != 2140 || c.Y != 500 {
		t.Errorf("press on the refreshed frame = %+v, want a MouseDown at (2140, 500)", c)
	}
}

// TestWindow_ReleaseAtClampedPoint: the window shrinks while a button is held;
// the up still posts, clamped into what is left of the window.
func TestWindow_ReleaseAtClampedPoint(t *testing.T) {
	s, fw := newWindowSync(t)
	s.do(t, "%s", press(1, 1))
	if c := lastMouse(t, s.fb); c.Kind != screencap.MouseDown || c.X != 1379 || c.Y != 899 {
		t.Fatalf("press = %+v, want a MouseDown at (1379, 899)", c)
	}
	fw.set(func(f *fakeWindow) { f.infoFrame = [4]float64{100, 100, 640, 400} })
	s.do(t, `{"t":"btn","x":1,"y":1,"e":1,"b":0,"d":false,"ts":1100}`)
	if c := lastMouse(t, s.fb); c.Kind != screencap.MouseUp || c.X != 739 || c.Y != 499 {
		t.Errorf("release = %+v, want a MouseUp at (739, 499)", c)
	}
}

// TestWindow_TopmostHitDrops: a press whose point another window covers is
// dropped; with the shared window on top it posts.
func TestWindow_TopmostHitDrops(t *testing.T) {
	s, fw := newWindowSync(t)
	fw.set(func(f *fakeWindow) { f.top = 9999 })
	s.do(t, "%s", press(0.5, 0.5))
	if n := counter(s.env, "covered"); n != 1 || len(s.fb.snapshot()) != 0 {
		t.Fatalf("covered = %d, bridge calls = %d, want 1 and 0", n, len(s.fb.snapshot()))
	}
	fw.set(func(f *fakeWindow) { f.top = windowID })
	s.do(t, "%s", press(0.5, 0.5))
	if c := lastMouse(t, s.fb); c.Kind != screencap.MouseDown {
		t.Errorf("press with the window on top = %+v, want a MouseDown", c)
	}
	fw.set(func(f *fakeWindow) { f.topErr = errBoom("window list") })
	s.do(t, "%s", press(0.5, 0.5))
	if n := counter(s.env, "window_unavailable"); n != 1 {
		t.Errorf("an unreadable window list: window_unavailable = %d, want 1 (fail closed)", n)
	}
}

// TestWindow_SameAppPopupAllowed: a menu or pop-up of the shared app (a pop-up
// menu layer, 101 and up) is its own window; a press on it posts. Everything
// else on top covers (T-20-21): another app's window at any layer, and the
// shared app's own sheets, panels, utility windows and other document windows,
// which a single-window capture does not show.
func TestWindow_SameAppPopupAllowed(t *testing.T) {
	cases := []struct {
		name  string
		set   func(f *fakeWindow)
		drops bool
	}{
		{"menu layer", func(f *fakeWindow) { f.top, f.topPID, f.topLayer = 9999, winPID, 101 }, false},
		{"menu outside the frame", func(f *fakeWindow) {
			f.top, f.topPID, f.topLayer, f.topFrame = 9999, winPID, 101, [4]float64{1300, 800, 300, 400}
		}, false},
		{"above the menu layer", func(f *fakeWindow) { f.top, f.topPID, f.topLayer = 9999, winPID, 102 }, false},
		{"same-app contained layer-0 window", func(f *fakeWindow) {
			f.top, f.topPID, f.topFrame = 9999, winPID, [4]float64{300, 100, 390, 218}
		}, true},
		{"same-app floating panel, layer 3", func(f *fakeWindow) {
			f.top, f.topPID, f.topLayer, f.topFrame = 9999, winPID, 3, [4]float64{300, 100, 390, 218}
		}, true},
		{"same-app modal panel, layer 8", func(f *fakeWindow) { f.top, f.topPID, f.topLayer = 9999, winPID, 8 }, true},
		{"same-app layer 100", func(f *fakeWindow) { f.top, f.topPID, f.topLayer = 9999, winPID, 100 }, true},
		{"same-app negative layer", func(f *fakeWindow) { f.top, f.topPID, f.topLayer = 9999, winPID, -1 }, true},
		{"other document window", func(f *fakeWindow) {
			f.top, f.topPID, f.topFrame = 9999, winPID, [4]float64{900, 500, 800, 600}
		}, true},
		{"other app, normal layer", func(f *fakeWindow) {
			f.top, f.topPID, f.topFrame = 9999, otherPID, [4]float64{300, 100, 390, 218}
		}, true},
		{"other app, menu layer", func(f *fakeWindow) { f.top, f.topPID, f.topLayer = 9999, otherPID, 101 }, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, fw := newWindowSync(t)
			fw.set(c.set)
			s.do(t, "%s", press(0.5, 0.5))
			posted := len(s.fb.snapshot()) > 0
			if posted == c.drops || (counter(s.env, "covered") == 1) != c.drops {
				t.Errorf("posted = %v, covered = %d, drops = %v", posted, counter(s.env, "covered"), c.drops)
			}
		})
	}
}

// TestWindow_WheelConfined: a scroll lands where the pointer is, so the pointer
// must be inside the shared window and the window on top there.
func TestWindow_WheelConfined(t *testing.T) {
	s, fw := newWindowSync(t)
	wheel := `{"t":"wheel","dx":0,"dy":10,"e":1}`
	setCursor := func(x, y float64) {
		s.fb.mu.Lock()
		s.fb.curX, s.fb.curY = x, y
		s.fb.mu.Unlock()
	}
	setCursor(50, 50) // outside the window
	s.do(t, "%s", wheel)
	setCursor(500, 500)
	fw.set(func(f *fakeWindow) { f.top = 9999 })
	s.do(t, "%s", wheel)
	if counter(s.env, "outside") != 1 || counter(s.env, "covered") != 1 || len(s.fb.snapshot()) != 0 {
		t.Fatalf("outside = %d, covered = %d, calls = %d, want 1, 1, 0", counter(s.env, "outside"), counter(s.env, "covered"), len(s.fb.snapshot()))
	}
	fw.set(func(f *fakeWindow) { f.top = windowID })
	s.do(t, "%s", wheel)
	if calls := s.fb.snapshot(); len(calls) != 1 || calls[0].Op != "scroll" {
		t.Errorf("scroll over the shared window = %+v, want one scroll", calls)
	}
	setCursor(0, 0)
	s.fb.mu.Lock()
	s.fb.curErr = errBoom("cursor")
	s.fb.mu.Unlock()
	s.do(t, "%s", wheel)
	if n := counter(s.env, "window_unavailable"); n != 1 {
		t.Errorf("unreadable pointer: window_unavailable = %d, want 1", n)
	}
}

// TestWindow_ChordsDropped: Cmd+Tab, Cmd+Space and Ctrl+Cmd+Q are dropped in a
// window share and post in a display share; Tab alone posts.
func TestWindow_ChordsDropped(t *testing.T) {
	s, _ := newWindowSync(t)
	tab, _ := screencap.KeyCode("Tab")
	space, _ := screencap.KeyCode("Space")
	q, _ := screencap.KeyCode("KeyQ")

	s.do(t, `{"t":"key","code":"MetaLeft","d":true,"m":8}`)
	s.do(t, `{"t":"key","code":"Tab","d":true,"m":8}`)
	s.do(t, `{"t":"key","code":"MetaLeft","d":false,"m":0}`)
	s.do(t, `{"t":"key","code":"MetaRight","d":true,"m":8}`)
	s.do(t, `{"t":"key","code":"Space","d":true,"m":8}`)
	s.do(t, `{"t":"key","code":"MetaRight","d":false,"m":0}`)
	s.do(t, `{"t":"key","code":"ControlLeft","d":true,"m":2}`)
	s.do(t, `{"t":"key","code":"MetaLeft","d":true,"m":10}`)
	s.do(t, `{"t":"key","code":"KeyQ","d":true,"m":10}`)
	s.do(t, `{"t":"key","code":"MetaLeft","d":false,"m":2}`)
	s.do(t, `{"t":"key","code":"ControlLeft","d":false,"m":0}`)
	if n := counter(s.env, "window_chord"); n != 3 {
		t.Fatalf("window_chord = %d, want 3", n)
	}
	for name, vk := range map[string]uint16{"Tab": tab, "Space": space, "KeyQ": q} {
		if keyDownPosted(s.fb, vk) {
			t.Errorf("%s posted as a chord in a window share", name)
		}
	}
	s.do(t, `{"t":"key","code":"Tab","d":true,"m":0}`)
	if !keyDownPosted(s.fb, tab) {
		t.Error("Tab without Command did not post in a window share")
	}

	s.env.mgr.SetSurfaceForTest(screencap.Surface{}, screencap.SurfaceInfo{})
	s.do(t, `{"t":"key","code":"MetaLeft","d":true,"m":8}`)
	s.do(t, `{"t":"key","code":"Space","d":true,"m":8}`)
	if !keyDownPosted(s.fb, space) {
		t.Error("Cmd+Space did not post in a display share")
	}
}

// lastControl returns the last control message the daemon sent a viewer.
func lastControl(v *inputViewer) string {
	last := ""
	for _, m := range v.received() {
		if strings.HasPrefix(m, `{"t":"control"`) {
			last = m
		}
	}
	return last
}

const (
	controlRaiseFailed = `{"t":"control","input":"ok","window":"raise_failed"}`
	controlMinimized   = `{"t":"control","input":"ok","window":"minimized"}`
)

// joinWindow joins a viewer with an open input channel to a Manager that
// shares the window.
func joinWindow(t *testing.T, fw *fakeWindow) (*tracerEnv, *inputViewer, *fakeInputBridge) {
	t.Helper()
	fb := &fakeInputBridge{}
	env, v, _, _ := joinWithControl(t, fb, withWindow(fw))
	eventuallyReceived(t, v, controlOKMsg)
	env.mgr.SetSurfaceForTest(windowSel, sharedWindow())
	return env, v, fb
}

// TestWindow_RaiseFailedState: a raise that fails tells the viewer raise_failed,
// the next press retries, and a raise that works clears it.
func TestWindow_RaiseFailedState(t *testing.T) {
	fw := newFakeWindow()
	fw.raiseHook = func(*fakeWindow) (int, error) { return 0, errBoom("no rung worked") }
	env, v, _ := joinWindow(t, fw)
	down := map[string]any{"t": "btn", "x": 0.5, "y": 0.5, "e": 1, "b": 0, "d": true}
	v.send(t, down)
	eventuallyReceived(t, v, controlRaiseFailed)

	fw.set(func(f *fakeWindow) { f.raiseHook = nil }) // the next raise focuses the window
	eventually(t, 5*time.Second, "the first raise to end", func() bool { return !env.mgr.WindowRaisingForTest() })
	v.send(t, down)
	eventually(t, 5*time.Second, "the retry", func() bool { return fw.raiseCount() == 2 })
	eventually(t, 5*time.Second, "raise_failed cleared", func() bool { return lastControl(v) == controlOKMsg })
}

// TestWindow_RaiseTimeout: a raise that has not finished within raiseTimeout is
// reported as failed, and a later success clears it. Input stays dropped meanwhile.
func TestWindow_RaiseTimeout(t *testing.T) {
	fw := newFakeWindow()
	block := make(chan struct{})
	fw.raiseBlock = block
	env, v, fb := joinWindow(t, fw)
	down := map[string]any{"t": "btn", "x": 0.5, "y": 0.5, "e": 1, "b": 0, "d": true}
	v.send(t, down)
	eventuallyReceived(t, v, controlRaiseFailed)
	if !env.mgr.WindowRaisingForTest() {
		t.Error("the raise ended before it was released")
	}
	v.send(t, down)
	eventually(t, 5*time.Second, "a press during the raise to drop", func() bool { return counter(env, "raising") >= 2 })
	close(block)
	eventually(t, 5*time.Second, "raise_failed cleared", func() bool { return lastControl(v) == controlOKMsg })
	if n := len(fb.snapshot()); n != 0 {
		t.Errorf("%d bridge calls while the raise was running, want 0", n)
	}
	if n := fw.raiseCount(); n != 1 {
		t.Errorf("Raise called %d times, want 1 (one at a time)", n)
	}
}

// TestWindow_MinimizedState: a minimized shared window is reported without a
// press; a press raises (and unminimizes) it and is dropped; the state clears
// once the window is back on screen.
func TestWindow_MinimizedState(t *testing.T) {
	fw := newFakeWindow()
	fw.infoOnScr, fw.minimized = false, true
	fw.raiseHook = func(f *fakeWindow) (int, error) {
		f.set(func(f *fakeWindow) { f.infoOnScr, f.minimized = true, false })
		f.focusWindow()
		return 1, nil
	}
	_, v, fb := joinWindow(t, fw)
	eventuallyReceived(t, v, controlMinimized)

	v.send(t, map[string]any{"t": "btn", "x": 0.5, "y": 0.5, "e": 1, "b": 0, "d": true})
	eventually(t, 5*time.Second, "the raise", func() bool { return fw.raiseCount() == 1 })
	if n := len(fb.snapshot()); n != 0 {
		t.Errorf("%d bridge calls on the press that raised, want 0", n)
	}
	eventually(t, 5*time.Second, "minimized cleared", func() bool { return lastControl(v) == controlOKMsg })
}

// TestWindow_FrameRefreshForMoves: the frame is re-read about 5 times a second,
// so within a second moves map onto a window that moved, with no epoch change
// (the moves keep their epoch and still post).
func TestWindow_FrameRefreshForMoves(t *testing.T) {
	fw := newFakeWindow()
	_, v, fb := joinWindow(t, fw)
	move := map[string]any{"t": "move", "x": 0.5, "y": 0.5, "e": 1}
	v.send(t, move)
	fb.waitFor(t, 1)
	if c := lastMouse(t, fb); c.X != 740 {
		t.Fatalf("first move at x = %v, want 740", c.X)
	}
	fw.set(func(f *fakeWindow) { f.infoFrame = [4]float64{150, 100, 1280, 800} })
	eventually(t, time.Second, "moves to follow the window", func() bool {
		v.send(t, move)
		time.Sleep(60 * time.Millisecond)
		return lastMouse(t, fb).X == 790
	})
}

// TestWindow_SurfaceChangeForgetsState: the frame and any window state belong to
// the window, so switching away clears them.
func TestWindow_SurfaceChangeForgetsState(t *testing.T) {
	fw := newFakeWindow()
	fw.infoOnScr, fw.minimized = false, true
	env, v, _ := joinWindow(t, fw)
	eventuallyReceived(t, v, controlMinimized)
	env.mgr.SetSurfaceForTest(screencap.Surface{}, screencap.SurfaceInfo{})
	eventually(t, 5*time.Second, "the state to clear", func() bool { return lastControl(v) == controlOKMsg })
}
