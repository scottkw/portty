package desktop_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/pion/webrtc/v4"

	"github.com/scottkw/portty/internal/desktop"
	"github.com/scottkw/portty/internal/screencap"
)

// The fake display layout every surface test shares: the main display (ID 1),
// a second display to its right (ID 9) and a window.
const (
	mainID    = 1
	secondID  = 9
	windowID  = 4242
	windowAlt = 4243
)

func mainSurface() screencap.SurfaceInfo {
	return screencap.SurfaceInfo{Kind: screencap.SurfaceDisplay, ID: mainID, Name: "Main Display", W: 1470, H: 956, Main: true, OnScreen: true}
}

func secondSurface() screencap.SurfaceInfo {
	return screencap.SurfaceInfo{Kind: screencap.SurfaceDisplay, ID: secondID, Name: "Display 2", X: 1470, W: 1920, H: 1080, OnScreen: true}
}

func termWindow(w, h float64) screencap.SurfaceInfo {
	return screencap.SurfaceInfo{
		Kind: screencap.SurfaceWindow, ID: windowID, PID: 77, App: "Terminal", Title: "zsh", Name: "Terminal — zsh",
		X: 100, Y: 80, W: w, H: h, OnScreen: true,
	}
}

// surfaceWorld is the fake host a surface test runs against: the list
// ListSurfaces returns and every source the Manager built.
type surfaceWorld struct {
	mu      sync.Mutex
	list    []screencap.SurfaceInfo
	listErr error
	sources []*switchSource
}

func newSurfaceWorld(list ...screencap.SurfaceInfo) *surfaceWorld {
	return &surfaceWorld{list: list}
}

func (w *surfaceWorld) ListSurfaces(context.Context) ([]screencap.SurfaceInfo, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.listErr != nil {
		return nil, w.listErr
	}
	return append([]screencap.SurfaceInfo(nil), w.list...), nil
}

func (w *surfaceWorld) setList(list ...screencap.SurfaceInfo) {
	w.mu.Lock()
	w.list = list
	w.mu.Unlock()
}

// pixels is the encoded size of a surface: display points times 1 for the
// test's displays, a window at its frame.
func (w *surfaceWorld) pixels(s screencap.Surface) (int, int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, i := range w.list {
		if i.Kind == s.Kind && (i.ID == s.ID || (s.Kind == screencap.SurfaceDisplay && s.ID == 0 && i.Main)) {
			return int(i.W), int(i.H)
		}
	}
	return 1470, 956
}

func (w *surfaceWorld) last() *switchSource {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.sources) == 0 {
		return nil
	}
	return w.sources[len(w.sources)-1]
}

func (w *surfaceWorld) sourceCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.sources)
}

// switchSource is a synthetic source that switches surface the way the real one
// does: it re-creates its picture at the size of the chosen surface.
type switchSource struct {
	*screencap.SyntheticSource
	world *surfaceWorld

	mu       sync.Mutex
	started  screencap.Surface
	selects  []screencap.Surface
	failWith error
	delay    time.Duration
	entered  chan struct{} // closed-by-send when a SelectSurface begins
}

func (s *switchSource) Start(ctx context.Context, cfg screencap.Config) error {
	s.mu.Lock()
	s.started = cfg.Surface
	s.mu.Unlock()
	w, h := s.world.pixels(cfg.Surface)
	if err := s.Rebuild(w, h); err != nil {
		return err
	}
	cfg.Surface = screencap.Surface{} // the synthetic source captures displays only
	return s.SyntheticSource.Start(ctx, cfg)
}

func (s *switchSource) SelectSurface(sel screencap.Surface) error {
	s.mu.Lock()
	delay, fail := s.delay, s.failWith
	s.mu.Unlock()
	select {
	case s.entered <- struct{}{}:
	default:
	}
	time.Sleep(delay)
	if fail != nil {
		return fail
	}
	s.mu.Lock()
	s.selects = append(s.selects, sel)
	s.mu.Unlock()
	w, h := s.world.pixels(sel)
	return s.Rebuild(w, h)
}

func (s *switchSource) selected() []screencap.Surface {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]screencap.Surface(nil), s.selects...)
}

func (s *switchSource) setSwitch(delay time.Duration, fail error) {
	s.mu.Lock()
	s.delay, s.failWith = delay, fail
	s.mu.Unlock()
}

// displayBoundsByID maps the fake displays: id 0 and the main ID are the main display.
func displayBoundsByID(id uint32) (x, y, w, h float64, err error) {
	if id == secondID {
		return 1470, 0, 1920, 1080, nil
	}
	return 0, 0, 1470, 956, nil
}

// withSurfaces wires the surface world, the fake bridge and a quiet lock.
func withSurfaces(w *surfaceWorld, fb *fakeInputBridge) tracerOpt {
	return func(env *tracerEnv, d *desktop.Deps) {
		d.NewSource = func() screencap.Source {
			s := &switchSource{SyntheticSource: screencap.NewSyntheticSource(1470, 956), world: w, entered: make(chan struct{}, 1)}
			w.mu.Lock()
			w.sources = append(w.sources, s)
			w.mu.Unlock()
			return s
		}
		d.ListSurfaces = w.ListSurfaces
		d.Input = fb
		d.DisplayBounds = displayBoundsByID
		d.MainDisplayID = func() uint32 { return mainID }
	}
}

// surfViewer is a viewer with an input channel that keeps every message the
// daemon sends on its signalling socket.
type surfViewer struct {
	id   string
	conn *websocket.Conn
	dc   *webrtc.DataChannel

	mu   sync.Mutex
	msgs []map[string]any
}

func (e *tracerEnv) joinSurf(ctx context.Context, t *testing.T, tok string) *surfViewer {
	t.Helper()
	conn, _, err := e.dial(ctx, tok)
	if err != nil {
		t.Fatalf("dial /desktop/ws: %v", err)
	}
	t.Cleanup(func() { _ = conn.CloseNow() })
	writeJSON(ctx, t, conn, map[string]any{"type": "hello", "v": 1, "caps": map[string]int{"video": 1, "input": 1}})
	hello := readType(ctx, t, conn, "hello")
	lc := newLoopbackClient(t)
	dc, err := lc.pc.CreateDataChannel("input", nil)
	if err != nil {
		t.Fatal(err)
	}
	open := make(chan struct{})
	dc.OnOpen(func() { close(open) })
	writeJSON(ctx, t, conn, map[string]any{"type": "offer", "sdp": lc.offer(t)})
	lc.accept(t, readType(ctx, t, conn, "answer")["sdp"].(string))
	v := &surfViewer{id: hello["viewer"].(string), conn: conn, dc: dc}
	go v.read(ctx)
	select {
	case <-open:
	case <-time.After(10 * time.Second):
		t.Fatal("the input channel did not open")
	}
	return v
}

func (v *surfViewer) read(ctx context.Context) {
	for {
		_, b, err := v.conn.Read(ctx)
		if err != nil {
			return
		}
		var m map[string]any
		if json.Unmarshal(b, &m) == nil {
			v.mu.Lock()
			v.msgs = append(v.msgs, m)
			v.mu.Unlock()
		}
	}
}

// ask sends one signalling message.
func (v *surfViewer) ask(ctx context.Context, t *testing.T, msg map[string]any) {
	t.Helper()
	writeJSON(ctx, t, v.conn, msg)
}

func (v *surfViewer) input(t *testing.T, msg map[string]any) {
	t.Helper()
	b, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.dc.SendText(string(b)); err != nil {
		t.Fatal(err)
	}
}

// find returns the first message of type typ that match accepts (nil accepts any).
func (v *surfViewer) find(typ string, match func(map[string]any) bool) map[string]any {
	v.mu.Lock()
	defer v.mu.Unlock()
	for _, m := range v.msgs {
		if m["type"] == typ && (match == nil || match(m)) {
			return m
		}
	}
	return nil
}

// wait waits for a message of type typ that match accepts.
func (v *surfViewer) wait(t *testing.T, typ string, match func(map[string]any) bool) map[string]any {
	t.Helper()
	var got map[string]any
	eventually(t, 5*time.Second, "a "+typ+" message", func() bool {
		got = v.find(typ, match)
		return got != nil
	})
	return got
}

func (v *surfViewer) count(typ string) int {
	v.mu.Lock()
	defer v.mu.Unlock()
	n := 0
	for _, m := range v.msgs {
		if m["type"] == typ {
			n++
		}
	}
	return n
}

// epochIs matches a surface message of an epoch.
func epochIs(e float64) func(map[string]any) bool {
	return func(m map[string]any) bool { return m["e"] == e }
}

func sizeIs(w, h float64) func(map[string]any) bool {
	return func(m map[string]any) bool { return m["w"] == w && m["h"] == h }
}

// requireSurface fails unless m is a surface message with these fields.
func requireSurface(t *testing.T, who string, m map[string]any, e float64, kind string, id float64, name string, w, h float64, reason string) {
	t.Helper()
	want := map[string]any{"type": "surface", "e": e, "kind": kind, "id": id, "name": name, "w": w, "h": h, "reason": reason}
	for k, v := range want {
		if m[k] != v {
			t.Errorf("%s: surface field %s = %v, want %v (message %v)", who, k, m[k], v, m)
		}
	}
}

// TestSurfaceTracer_SelectWindowBumpsEpoch: two viewers share one capture. One
// lists the surfaces and selects a window; both get the new epoch and the new
// picture size on the same connection with no new answer, input stamped with
// the old epoch is dropped, window input waits for the window gate, and
// selecting a second display maps input onto that display.
func TestSurfaceTracer_SelectWindowBumpsEpoch(t *testing.T) {
	world := newSurfaceWorld(mainSurface(), secondSurface(), termWindow(1280, 800))
	off := termWindow(300, 200)
	off.ID, off.Title, off.Name, off.OnScreen = windowAlt, "minimized", "Terminal — minimized", false
	world.setList(mainSurface(), secondSurface(), termWindow(1280, 800), off)
	fb := &fakeInputBridge{}
	env := newTracerServer(t, withSurfaces(world, fb))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	tok := env.token(t, "g1")
	a := env.joinSurf(ctx, t, tok)
	b := env.joinSurf(ctx, t, tok)

	// A joiner is told the main display and its epoch right after its answer.
	requireSurface(t, "A join", a.wait(t, "surface", nil), 1, "display", mainID, "Main Display", 1470, 956, "")

	a.ask(ctx, t, map[string]any{"type": "surfaces"})
	list := a.wait(t, "surfaces", nil)
	displays, _ := list["displays"].([]any)
	windows, _ := list["windows"].([]any)
	if len(displays) != 2 || len(windows) != 1 {
		t.Fatalf("surfaces reply = %v, want 2 displays and the 1 on-screen window", list)
	}
	if w := windows[0].(map[string]any); w["id"] != float64(windowID) || w["name"] != "Terminal — zsh" || w["app"] != "Terminal" || w["title"] != "zsh" {
		t.Errorf("window entry = %v", w)
	}

	a.ask(ctx, t, map[string]any{"type": "select", "kind": "window", "id": windowID})
	for who, v := range map[string]*surfViewer{"A": a, "B": b} {
		requireSurface(t, who, v.wait(t, "surface", epochIs(2)), 2, "window", windowID, "Terminal — zsh", 1280, 800, "")
		v.wait(t, "size", sizeIs(1280, 800))
		if n := v.count("answer"); n != 0 {
			t.Errorf("viewer %s got %d new answers, want none: a switch does not renegotiate", who, n)
		}
	}
	if n := world.sourceCount(); n != 1 {
		t.Errorf("%d sources built, want 1 (the switch runs on the running capture)", n)
	}
	if got := world.last().selected(); len(got) != 1 || got[0] != (screencap.Surface{Kind: screencap.SurfaceWindow, ID: windowID}) {
		t.Errorf("source switched to %v, want the one window", got)
	}

	// A window share has no target yet (the window gate is a later plan), so
	// presses and moves are dropped whatever epoch they carry.
	a.input(t, map[string]any{"t": "move", "x": 0.5, "y": 0.5, "e": 1})
	eventually(t, 5*time.Second, "no_target counted", func() bool { return counter(env, "no_target") == 1 })
	a.input(t, map[string]any{"t": "move", "x": 0.5, "y": 0.5, "e": 2}) // sent after the first is handled: a queued move would be replaced
	eventually(t, 5*time.Second, "no_target counted twice", func() bool { return counter(env, "no_target") == 2 })
	if n := len(fb.snapshot()); n != 0 {
		t.Errorf("%d bridge calls on a window share, want 0", n)
	}

	// Display 2 at origin (1470, 0): input sent with the previous epoch is
	// dropped as stale, and a move to the middle at the new one posts at (2430, 540).
	a.ask(ctx, t, map[string]any{"type": "select", "kind": "display", "id": secondID})
	requireSurface(t, "A second display", a.wait(t, "surface", epochIs(3)), 3, "display", secondID, "Display 2", 1920, 1080, "")
	a.input(t, map[string]any{"t": "move", "x": 0.1, "y": 0.1, "e": 2})
	eventually(t, 5*time.Second, "stale_epoch counted", func() bool { return counter(env, "stale_epoch") == 1 })
	a.input(t, map[string]any{"t": "move", "x": 0.5, "y": 0.5, "e": 3})
	calls := fb.waitFor(t, 1)
	if c := calls[0]; c.Op != "mouse" || c.Kind != screencap.MouseMoved || c.X != 2430 || c.Y != 540 {
		t.Errorf("move on display 2 = %+v, want a MouseMoved at (2430, 540)", c)
	}
}

// TestSurface_SelectForbiddenAndBusy: a viewer without RightsSurface may neither
// list nor select, a select during a slow switch answers busy, and an id that
// is not in a fresh list (or a kind that is not one) answers not_found.
func TestSurface_SelectForbiddenAndBusy(t *testing.T) {
	world := newSurfaceWorld(mainSurface(), secondSurface(), termWindow(1280, 800))
	env := newTracerServer(t, withSurfaces(world, &fakeInputBridge{}))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	tok := env.token(t, "g1")
	a := env.joinSurf(ctx, t, tok)
	b := env.joinSurf(ctx, t, tok)
	failed := func(code string) func(map[string]any) bool {
		return func(m map[string]any) bool { return m["code"] == code }
	}

	env.mgr.SetViewerRightsForTest(desktop.ViewerID(a.id), 0)
	a.ask(ctx, t, map[string]any{"type": "select", "kind": "window", "id": windowID})
	a.wait(t, "select_failed", failed("forbidden"))
	a.ask(ctx, t, map[string]any{"type": "surfaces"})
	if got := a.wait(t, "surfaces", nil); got["error"] != "forbidden" || got["windows"] != nil {
		t.Errorf("list for a viewer without the surface right = %v, want only error forbidden", got)
	}
	if len(world.last().selected()) != 0 || b.find("surface", epochIs(2)) != nil {
		t.Error("a forbidden select changed the surface")
	}

	// Busy: B's switch is slow; A's select meanwhile is refused.
	env.mgr.SetViewerRightsForTest(desktop.ViewerID(a.id), desktop.RightsOwnAccount)
	world.last().setSwitch(800*time.Millisecond, nil)
	b.ask(ctx, t, map[string]any{"type": "select", "kind": "window", "id": windowID})
	<-world.last().entered
	a.ask(ctx, t, map[string]any{"type": "select", "kind": "display", "id": secondID})
	a.wait(t, "select_failed", failed("busy"))
	requireSurface(t, "B switch", b.wait(t, "surface", epochIs(2)), 2, "window", windowID, "Terminal — zsh", 1280, 800, "")

	// Not found: an unknown window, an unknown display, a kind that is neither.
	world.last().setSwitch(0, nil)
	b.ask(ctx, t, map[string]any{"type": "select", "kind": "window", "id": 9999})
	b.ask(ctx, t, map[string]any{"type": "select", "kind": "display", "id": 77})
	b.ask(ctx, t, map[string]any{"type": "select", "kind": "tab", "id": 1})
	eventually(t, 5*time.Second, "three not_found replies", func() bool {
		n := 0
		b.mu.Lock()
		defer b.mu.Unlock()
		for _, m := range b.msgs {
			if m["type"] == "select_failed" && m["code"] == "not_found" {
				n++
			}
		}
		return n == 3
	})
	if len(world.last().selected()) != 1 {
		t.Errorf("source switched %d times, want 1 (the not_found picks must not switch)", len(world.last().selected()))
	}
}

// TestSurface_ListFailed: a list that cannot be read answers list_failed and a
// select against it fails without touching the capture.
func TestSurface_ListFailed(t *testing.T) {
	world := newSurfaceWorld(mainSurface())
	world.listErr = errors.New("sck says no")
	env := newTracerServer(t, withSurfaces(world, &fakeInputBridge{}))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	a := env.joinSurf(ctx, t, env.token(t, "g1"))

	a.ask(ctx, t, map[string]any{"type": "surfaces"})
	if got := a.wait(t, "surfaces", nil); got["error"] != "list_failed" {
		t.Errorf("surfaces reply = %v, want error list_failed", got)
	}
	a.ask(ctx, t, map[string]any{"type": "select", "kind": "display", "id": 0})
	a.wait(t, "select_failed", func(m map[string]any) bool { return m["code"] == "switch_failed" })
	if len(world.last().selected()) != 0 {
		t.Error("a select against a failed list switched the capture")
	}
}

// TestSurface_SwitchFailedBroadcastsUnchanged: when the source cannot switch,
// the requester gets switch_failed and every viewer gets the surface it still
// shows with the new epoch, so nobody stays paused on the old epoch.
func TestSurface_SwitchFailedBroadcastsUnchanged(t *testing.T) {
	world := newSurfaceWorld(mainSurface(), termWindow(1280, 800))
	env := newTracerServer(t, withSurfaces(world, &fakeInputBridge{}))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	tok := env.token(t, "g1")
	a := env.joinSurf(ctx, t, tok)
	b := env.joinSurf(ctx, t, tok)
	world.last().setSwitch(0, errors.New("update refused"))

	a.ask(ctx, t, map[string]any{"type": "select", "kind": "window", "id": windowID})
	a.wait(t, "select_failed", func(m map[string]any) bool { return m["code"] == "switch_failed" })
	for who, v := range map[string]*surfViewer{"A": a, "B": b} {
		requireSurface(t, who, v.wait(t, "surface", epochIs(2)), 2, "display", mainID, "Main Display", 1470, 956, "")
	}
	if got := env.mgr.CurrentSurface(); got.Kind != screencap.SurfaceDisplay || got.Name != "Main Display" {
		t.Errorf("current surface after a failed switch = %+v, want the main display", got)
	}
}

func (s *switchSource) startedWith() screencap.Surface {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.started
}

// selectWindow has A pick the terminal window and waits until it is told.
func selectWindow(ctx context.Context, t *testing.T, a *surfViewer) {
	t.Helper()
	a.ask(ctx, t, map[string]any{"type": "select", "kind": "window", "id": windowID})
	a.wait(t, "surface", epochIs(2))
}

// quiet waits long enough for two polls of the surface loop.
func quiet() { time.Sleep(2500 * time.Millisecond) }

// TestSurface_FallbackWindowClosed: the shared window disappears from the list;
// every viewer is told the share fell back to the main display, with a bumped
// epoch, the source was switched and frames keep flowing.
func TestSurface_FallbackWindowClosed(t *testing.T) {
	world := newSurfaceWorld(mainSurface(), secondSurface(), termWindow(1280, 800))
	env := newTracerServer(t, withSurfaces(world, &fakeInputBridge{}))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	tok := env.token(t, "g1")
	a := env.joinSurf(ctx, t, tok)
	b := env.joinSurf(ctx, t, tok)
	selectWindow(ctx, t, a)

	world.setList(mainSurface(), secondSurface())
	for who, v := range map[string]*surfViewer{"A": a, "B": b} {
		requireSurface(t, who, v.wait(t, "surface", epochIs(3)), 3, "display", mainID, "Main Display", 1470, 956, "window_closed")
	}
	sel := world.last().selected()
	if len(sel) != 2 || sel[1] != (screencap.Surface{}) {
		t.Errorf("source switches = %v, want the window then the main display", sel)
	}
	before := env.mgr.FramesSeenForTest()
	eventually(t, 5*time.Second, "frames keep flowing", func() bool { return env.mgr.FramesSeenForTest() > before+3 })
}

// TestSurface_FallbackDisplayGone: the shared second display is unplugged.
func TestSurface_FallbackDisplayGone(t *testing.T) {
	world := newSurfaceWorld(mainSurface(), secondSurface())
	env := newTracerServer(t, withSurfaces(world, &fakeInputBridge{}))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	a := env.joinSurf(ctx, t, env.token(t, "g1"))
	a.ask(ctx, t, map[string]any{"type": "select", "kind": "display", "id": secondID})
	a.wait(t, "surface", epochIs(2))

	world.setList(mainSurface())
	requireSurface(t, "A", a.wait(t, "surface", epochIs(3)), 3, "display", mainID, "Main Display", 1470, 956, "display_gone")
}

// TestSurface_MinimizedIsNotClosed: a window that is still in the list but off
// screen is no reason to fall back; its state is recorded for the window gate.
func TestSurface_MinimizedIsNotClosed(t *testing.T) {
	world := newSurfaceWorld(mainSurface(), termWindow(1280, 800))
	env := newTracerServer(t, withSurfaces(world, &fakeInputBridge{}))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	a := env.joinSurf(ctx, t, env.token(t, "g1"))
	selectWindow(ctx, t, a)

	min := termWindow(1280, 800)
	min.OnScreen = false
	world.setList(mainSurface(), min)
	eventually(t, 5*time.Second, "the minimized state recorded", func() bool { return !env.mgr.CurrentSurface().OnScreen })
	quiet()
	if m := a.find("surface", epochIs(3)); m != nil {
		t.Errorf("a minimized window produced %v, want no fallback and no epoch bump", m)
	}
	if got := env.mgr.CurrentSurface(); got.Kind != screencap.SurfaceWindow || got.ID != windowID {
		t.Errorf("current surface = %+v, want the window still shared", got)
	}
	if len(world.last().selected()) != 1 {
		t.Errorf("source switches = %v, want only the pick", world.last().selected())
	}
}

// TestSurface_ResizeBumpsEpoch: a resize re-selects the same window at its new
// size under a new epoch; a pure move updates the recorded frame and nothing else.
func TestSurface_ResizeBumpsEpoch(t *testing.T) {
	world := newSurfaceWorld(mainSurface(), termWindow(1280, 800))
	env := newTracerServer(t, withSurfaces(world, &fakeInputBridge{}))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	a := env.joinSurf(ctx, t, env.token(t, "g1"))
	selectWindow(ctx, t, a)

	moved := termWindow(1280, 800)
	moved.X, moved.Y = 300, 300
	world.setList(mainSurface(), moved)
	eventually(t, 5*time.Second, "the new position recorded", func() bool { return env.mgr.CurrentSurface().X == 300 })
	quiet()
	if m := a.find("surface", epochIs(3)); m != nil {
		t.Errorf("a window move produced %v, want no epoch bump", m)
	}

	world.setList(mainSurface(), termWindow(1000, 700))
	requireSurface(t, "A", a.wait(t, "surface", epochIs(3)), 3, "window", windowID, "Terminal — zsh", 1000, 700, "")
	a.wait(t, "size", sizeIs(1000, 700))
	if sel := world.last().selected(); len(sel) != 2 || sel[0] != sel[1] {
		t.Errorf("source switches = %v, want the same window selected twice", sel)
	}
}

// TestSurface_ResetOnCaptureStop: the surface lives for one viewing session;
// the next viewer starts on the main display and the source is started on it.
func TestSurface_ResetOnCaptureStop(t *testing.T) {
	world := newSurfaceWorld(mainSurface(), termWindow(1280, 800))
	env := newTracerServer(t, withSurfaces(world, &fakeInputBridge{}))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	tok := env.token(t, "g1")
	a := env.joinSurf(ctx, t, tok)
	selectWindow(ctx, t, a)
	_ = a.conn.CloseNow()
	eventually(t, 5*time.Second, "the last viewer gone", func() bool { return env.mgr.Viewers() == 0 })

	b := env.joinSurf(ctx, t, tok)
	m := b.wait(t, "surface", nil)
	if m["kind"] != "display" || m["name"] != "Main Display" || m["reason"] != "" {
		t.Errorf("first surface of the next session = %v, want the main display", m)
	}
	if n := world.sourceCount(); n != 2 {
		t.Fatalf("%d sources built, want 2", n)
	}
	if got := world.last().startedWith(); got != (screencap.Surface{}) {
		t.Errorf("new source started on %+v, want the main display", got)
	}
}

// TestSurface_RestartKeepsSurface: a stream restart while a viewer is still
// there (a stall, a wake) captures the same window again.
func TestSurface_RestartKeepsSurface(t *testing.T) {
	world := newSurfaceWorld(mainSurface(), termWindow(1280, 800))
	env := newTracerServer(t, withSurfaces(world, &fakeInputBridge{}))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	a := env.joinSurf(ctx, t, env.token(t, "g1"))
	selectWindow(ctx, t, a)

	if err := env.mgr.RestartSourceForTest(); err != nil {
		t.Fatal(err)
	}
	if n := world.sourceCount(); n != 2 {
		t.Fatalf("%d sources built, want 2", n)
	}
	if got := world.last().startedWith(); got != (screencap.Surface{Kind: screencap.SurfaceWindow, ID: windowID}) {
		t.Errorf("restarted source started on %+v, want the window", got)
	}
}

// TestSurface_SwitchReleasesHeld: buttons one viewer holds are released when
// another viewer switches the surface, and the holder keeps its input life.
func TestSurface_SwitchReleasesHeld(t *testing.T) {
	world := newSurfaceWorld(mainSurface(), secondSurface())
	fb := &fakeInputBridge{}
	env := newTracerServer(t, withSurfaces(world, fb))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	tok := env.token(t, "g1")
	a := env.joinSurf(ctx, t, tok)
	b := env.joinSurf(ctx, t, tok)

	a.input(t, map[string]any{"t": "btn", "b": 0, "d": true, "x": 0.5, "y": 0.5, "e": 1})
	fb.waitFor(t, 1)
	a.input(t, map[string]any{"t": "btn", "b": 2, "d": true, "x": 0.5, "y": 0.5, "e": 1})
	fb.waitFor(t, 2)
	b.ask(ctx, t, map[string]any{"type": "select", "kind": "display", "id": secondID})
	calls := fb.waitFor(t, 4)
	for i, button := range []int{0, 1} { // CG buttons of browser buttons 0 and 2
		if c := calls[2+i]; c.Kind != screencap.MouseUp || c.Button != button {
			t.Errorf("call %d = %+v, want a MouseUp of CG button %d", 2+i, c, button)
		}
	}

	// A still sends: its input life did not end with the release.
	a.wait(t, "surface", epochIs(2))
	a.input(t, map[string]any{"t": "move", "x": 0.5, "y": 0.5, "e": 2})
	if calls := fb.waitFor(t, 5); calls[4].Kind != screencap.MouseMoved {
		t.Errorf("call 4 = %+v, want a move after the switch", calls[4])
	}
}

// TestSurface_NoDesktopJSONChange: nothing about the surface is persisted.
func TestSurface_NoDesktopJSONChange(t *testing.T) {
	dir := t.TempDir()
	if err := desktop.SaveConfig(dir, desktop.Config{SchemaVersion: desktop.ConfigSchemaVersion, AlwaysAvailable: true, GrantID: "g1", IAT: 1}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, desktop.ConfigFile)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	world := newSurfaceWorld(mainSurface(), secondSurface(), termWindow(1280, 800))
	env := newTracerServer(t, withSurfaces(world, &fakeInputBridge{}), func(_ *tracerEnv, d *desktop.Deps) { d.ConfigDir = dir })
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	a := env.joinSurf(ctx, t, env.token(t, "g1"))
	selectWindow(ctx, t, a)
	a.ask(ctx, t, map[string]any{"type": "select", "kind": "display", "id": secondID})
	a.wait(t, "surface", epochIs(3))
	_ = a.conn.CloseNow()
	eventually(t, 5*time.Second, "the last viewer gone", func() bool { return env.mgr.Viewers() == 0 })

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("desktop.json changed:\nbefore %s\nafter  %s", before, after)
	}
}
