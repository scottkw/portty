package desktop_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/desktop"
	"github.com/scottkw/portty/internal/screencap"
)

// The shared window every window test uses: owned by PID 501, frame
// (100, 100, 1280 x 800), and a process (777) that holds focus instead.
const (
	winPID   = 501
	otherPID = 777
)

func sharedWindow() screencap.SurfaceInfo {
	return screencap.SurfaceInfo{
		Kind: screencap.SurfaceWindow, ID: windowID, PID: winPID, App: "Terminal", Title: "zsh", Name: "Terminal — zsh",
		X: 100, Y: 100, W: 1280, H: 800, OnScreen: true,
	}
}

type raiseCall struct {
	PID        int32
	X, Y, W, H float64
	Title      string
}

// fakeWindow is the one fake desktop.WindowBridge: focus, the window's frame
// and on-screen state, the topmost window and the minimized state are all
// settable, and every Raise is recorded. No test reads Accessibility.
type fakeWindow struct {
	mu sync.Mutex

	focusPID   int32
	focusFrame [4]float64
	focusOK    bool
	focusErr   error

	infoPID    int32
	infoFrame  [4]float64
	infoOnScr  bool
	infoErr    error
	minimized  bool
	minErr     error
	top        uint32
	topErr     error
	topPoints  [][2]float64
	raises     []raiseCall
	raiseHook  func(f *fakeWindow) (int, error) // runs with mu released; nil focuses the window
	raiseBlock chan struct{}                    // when set, Raise waits for it to close
}

func newFakeWindow() *fakeWindow {
	w := sharedWindow()
	return &fakeWindow{
		focusPID: otherPID, focusFrame: [4]float64{0, 0, 800, 600}, focusOK: true,
		infoPID: winPID, infoFrame: [4]float64{w.X, w.Y, w.W, w.H}, infoOnScr: true,
		top: windowID,
	}
}

func (f *fakeWindow) Focus() (int32, float64, float64, float64, float64, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fr := f.focusFrame
	return f.focusPID, fr[0], fr[1], fr[2], fr[3], f.focusOK, f.focusErr
}

func (f *fakeWindow) WindowInfo(uint32) (int32, float64, float64, float64, float64, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fr := f.infoFrame
	return f.infoPID, fr[0], fr[1], fr[2], fr[3], f.infoOnScr, f.infoErr
}

func (f *fakeWindow) Minimized(int32, float64, float64, float64, float64, string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.minimized, f.minErr
}

func (f *fakeWindow) Raise(pid int32, x, y, w, h float64, title string) (int, error) {
	f.mu.Lock()
	f.raises = append(f.raises, raiseCall{pid, x, y, w, h, title})
	hook, block := f.raiseHook, f.raiseBlock
	f.mu.Unlock()
	if block != nil {
		<-block
	}
	if hook != nil {
		return hook(f)
	}
	f.focusWindow()
	return 1, nil
}

func (f *fakeWindow) TopmostAt(x, y float64) (uint32, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.topPoints = append(f.topPoints, [2]float64{x, y})
	return f.top, f.topErr
}

// focusWindow makes the shared window the focused one (frame as WindowInfo has it).
func (f *fakeWindow) focusWindow() {
	f.mu.Lock()
	f.focusPID, f.focusFrame, f.focusOK = winPID, f.infoFrame, true
	f.mu.Unlock()
}

func (f *fakeWindow) raiseCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.raises)
}

func withWindow(fw desktop.WindowBridge) tracerOpt {
	return func(_ *tracerEnv, d *desktop.Deps) { d.Window = fw }
}

// TestWindowTracer_RaiseThenPost: a click on a background shared window raises
// it and is dropped; once focus is confirmed the next click lands at the right
// point inside the window and its release posts.
func TestWindowTracer_RaiseThenPost(t *testing.T) {
	world := newSurfaceWorld(mainSurface(), sharedWindow())
	fb := &fakeInputBridge{}
	fw := newFakeWindow()
	env := newTracerServer(t, withSurfaces(world, fb), withWindow(fw))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	a := env.joinSurf(ctx, t, env.token(t, "g1"))
	a.ask(ctx, t, map[string]any{"type": "select", "kind": "window", "id": windowID})
	a.wait(t, "surface", epochIs(2))

	down := map[string]any{"t": "btn", "x": 0.5, "y": 0.5, "e": 2, "b": 0, "d": true, "ts": 1000}
	a.input(t, down)
	eventually(t, 5*time.Second, "the raise", func() bool { return fw.raiseCount() == 1 })
	eventually(t, 5*time.Second, "the raise to finish", func() bool { return !env.mgr.WindowRaisingForTest() })
	fw.mu.Lock()
	got := fw.raises[0]
	fw.mu.Unlock()
	if got != (raiseCall{winPID, 100, 100, 1280, 800, "zsh"}) {
		t.Errorf("Raise = %+v, want the shared window's pid, frame and title", got)
	}
	if n := len(fb.snapshot()); n != 0 {
		t.Fatalf("%d bridge calls before focus was confirmed, want 0: the triggering press is dropped", n)
	}
	if n := counter(env, "raising"); n != 1 {
		t.Errorf("raising drops = %d, want 1", n)
	}

	a.input(t, down)
	calls := fb.waitFor(t, 1)
	if c := calls[0]; c.Op != "mouse" || c.Kind != screencap.MouseDown || c.X != 740 || c.Y != 500 {
		t.Fatalf("press after the raise = %+v, want a MouseDown at (740, 500)", c)
	}
	a.input(t, map[string]any{"t": "btn", "x": 0.5, "y": 0.5, "e": 2, "b": 0, "d": false, "ts": 1100})
	calls = fb.waitFor(t, 2)
	if c := calls[1]; c.Kind != screencap.MouseUp || c.X != 740 || c.Y != 500 {
		t.Errorf("release = %+v, want a MouseUp at (740, 500)", c)
	}
	if n := fw.raiseCount(); n != 1 {
		t.Errorf("Raise called %d times, want 1", n)
	}
}

// TestWindow_NilBridgeFailsClosed: with no window bridge every press to a
// window share is dropped, while a display share posts as before.
func TestWindow_NilBridgeFailsClosed(t *testing.T) {
	s := newSyncInput(t)
	s.env.mgr.SetSurfaceForTest(screencap.Surface{Kind: screencap.SurfaceWindow, ID: windowID}, sharedWindow())
	s.do(t, `{"t":"btn","x":0.5,"y":0.5,"e":1,"b":0,"d":true}`)
	s.do(t, `{"t":"key","code":"KeyA","d":true,"m":0}`)
	s.do(t, `{"t":"text","s":"hi"}`)
	s.do(t, `{"t":"wheel","dx":0,"dy":10,"e":1}`)
	if n := counter(s.env, "window_unavailable"); n != 4 {
		t.Errorf("window_unavailable = %d, want 4", n)
	}
	if n := len(s.fb.snapshot()); n != 0 {
		t.Errorf("%d bridge calls on a window share without a bridge, want 0", n)
	}
	s.env.mgr.SetSurfaceForTest(screencap.Surface{}, screencap.SurfaceInfo{})
	s.do(t, `{"t":"btn","x":0.5,"y":0.5,"e":1,"b":0,"d":true}`)
	if calls := s.fb.snapshot(); len(calls) != 1 || calls[0].Kind != screencap.MouseDown {
		t.Errorf("display share press = %+v, want one MouseDown", calls)
	}
}
