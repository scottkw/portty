package desktop_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/desktop"
	"github.com/scottkw/portty/internal/screencap"
)

func guestCounter(e *guestEnv, name string) uint64 { return e.mgr.InputCounters()[name] }

// TestGuestRights_TracerControl is the plan tracer: a view-only guest's press
// does nothing, the sharer allows control and the next press posts, and turning
// control off posts the release of a held button and drops the next press.
func TestGuestRights_TracerControl(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	fb := &fakeInputBridge{}
	e := newGuestEnv(t, withGuestInput(fb))
	g := e.joinGuest(ctx, t, 1001, "nodeA")

	g.press(t, 1000)
	eventually(t, 5*time.Second, "no_right counted", func() bool { return guestCounter(e, "no_right") == 1 })
	if n := len(fb.snapshot()); n != 0 {
		t.Fatalf("%d bridge calls for a view-only guest's press, want 0", n)
	}

	if err := e.mgr.SetRights(g.viewerID(), true, false); err != nil {
		t.Fatal(err)
	}
	g.wait(t, "rights", rightsIs(true, false))
	g.press(t, 2000)
	calls := fb.waitFor(t, 1)
	if c := calls[0]; c.Op != "mouse" || c.Kind != screencap.MouseDown || c.Button != 0 {
		t.Fatalf("first post = %+v, want a left MouseDown", c)
	}

	if err := e.mgr.SetRights(g.viewerID(), false, false); err != nil {
		t.Fatal(err)
	}
	calls = fb.waitFor(t, 2)
	if c := calls[1]; c.Op != "mouse" || c.Kind != screencap.MouseUp || c.Button != 0 {
		t.Fatalf("post after control off = %+v, want the held button's MouseUp", c)
	}
	g.wait(t, "rights", rightsIs(false, false))
	g.press(t, 3000)
	eventually(t, 5*time.Second, "second no_right counted", func() bool { return guestCounter(e, "no_right") == 2 })
	if n := len(fb.snapshot()); n != 2 {
		t.Errorf("%d bridge calls, want 2 (the press after control off must not post)", n)
	}
}

// TestGuestRights_EveryKindDropsWithoutControl: every kind of input a view-only
// guest sends does nothing.
func TestGuestRights_EveryKindDropsWithoutControl(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	fb := &fakeInputBridge{}
	e := newGuestEnv(t, withGuestInput(fb))
	g := e.joinGuest(ctx, t, 1001, "nodeA")

	g.input(t, map[string]any{"t": "key", "d": true, "code": "KeyA", "m": 0})
	g.input(t, map[string]any{"t": "text", "s": "hi"})
	g.input(t, map[string]any{"t": "wheel", "dx": 0, "dy": 40, "e": 1})
	g.input(t, map[string]any{"t": "move", "x": 0.1, "y": 0.1, "e": 1})
	g.input(t, map[string]any{"t": "rmove", "dx": 1, "dy": 1, "e": 1})
	eventually(t, 5*time.Second, "five no_right drops", func() bool { return guestCounter(e, "no_right") == 5 })
	if n := len(fb.snapshot()); n != 0 {
		t.Errorf("%d bridge calls, want 0", n)
	}
}

// TestGuestRights_ClipboardIndependentOfControl: clip_get answers forbidden
// until clipboard is granted, whatever control is, and works with clipboard on
// and control off.
func TestGuestRights_ClipboardIndependentOfControl(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pb := &fakePasteboard{text: "mac text"}
	fb := &fakeInputBridge{}
	e := newGuestEnv(t, func(d *desktop.Deps) {
		withGuestInput(fb)(d)
		d.Pasteboard = pb
		d.Lock = func() (bool, error) { return false, nil }
	})
	g := e.joinGuest(ctx, t, 1001, "nodeA")
	clipOutcome := func(xid string) string {
		g.ask(ctx, t, map[string]any{"type": "clip_get", "xid": xid})
		return g.clipResult(t, xid)
	}

	if got := clipOutcome("a"); got != "forbidden" {
		t.Fatalf("clip_get as view-only = %q, want forbidden", got)
	}
	if err := e.mgr.SetRights(g.viewerID(), true, false); err != nil {
		t.Fatal(err)
	}
	if got := clipOutcome("b"); got != "forbidden" {
		t.Fatalf("clip_get with control only = %q, want forbidden", got)
	}
	if err := e.mgr.SetRights(g.viewerID(), false, true); err != nil {
		t.Fatal(err)
	}
	if got := clipOutcome("c"); got != "ok" {
		t.Fatalf("clip_get with clipboard only = %q, want ok", got)
	}
	g.wait(t, "rights", rightsIs(false, true))
	if pb.readCount() != 1 {
		t.Errorf("pasteboard read %d times, want 1 (only the allowed copy)", pb.readCount())
	}
	// Control stayed off: a press still drops.
	g.press(t, 1000)
	eventually(t, 5*time.Second, "no_right counted", func() bool { return guestCounter(e, "no_right") == 1 })
}

// TestGuestRights_OwnAccountAndUnknown: an own-account viewer's rights cannot be
// changed and an unknown ID is reported.
func TestGuestRights_OwnAccountAndUnknown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	env := newTracerServer(t)
	v := env.joinSurf(ctx, t, env.token(t, "g1"))
	if err := env.mgr.SetRights(desktop.ViewerID(v.id), false, false); !errors.Is(err, desktop.ErrOwnViewer) {
		t.Fatalf("SetRights on an own-account viewer = %v, want ErrOwnViewer", err)
	}
	list := env.mgr.ViewerList()
	if len(list) != 1 || list[0].Rights != desktop.RightsOwnAccount {
		t.Fatalf("own-account rights after the refused call = %+v, want all", list)
	}
	if err := env.mgr.SetRights("nobody", true, true); !errors.Is(err, desktop.ErrNoViewer) {
		t.Fatalf("SetRights on an unknown ID = %v, want ErrNoViewer", err)
	}
}

// TestGuestRights_SurfaceNeverGranted: SetRights with everything on leaves
// surface off, in the message and the live list.
func TestGuestRights_SurfaceNeverGranted(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	e := newGuestEnv(t, withGuestInput(&fakeInputBridge{}))
	g := e.joinGuest(ctx, t, 1001, "nodeA")
	if err := e.mgr.SetRights(g.viewerID(), true, true); err != nil {
		t.Fatal(err)
	}
	m := g.wait(t, "rights", rightsIs(true, true))
	if r := m["rights"].(map[string]any); r["surface"] != false {
		t.Errorf("rights message surface = %v, want false", r["surface"])
	}
	list := e.mgr.ViewerList()
	if len(list) != 1 || list[0].Rights != desktop.RightsControl|desktop.RightsClipboard {
		t.Errorf("ViewerList rights = %+v, want control and clipboard only", list)
	}
}

// TestGuestRights_ReconnectKeepsGrants: a reconnect from the same node after
// SetRights is admitted at once with the same switches.
func TestGuestRights_ReconnectKeepsGrants(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	e := newGuestEnv(t, withGuestInput(&fakeInputBridge{}))
	g := e.joinGuest(ctx, t, 1001, "nodeA")
	if err := e.mgr.SetRights(g.viewerID(), true, true); err != nil {
		t.Fatal(err)
	}
	g.wait(t, "rights", rightsIs(true, true))

	_ = g.conn.CloseNow()
	g.waitClosed(t, 5*time.Second)
	eventually(t, 5*time.Second, "the guest to leave", func() bool { return e.mgr.Viewers() == 0 })

	c := e.dial(ctx, t, 1001, "nodeA")
	writeJSON(ctx, t, c, map[string]any{"type": "hello", "v": 1, "caps": map[string]int{"video": 1, "input": 1}})
	readType(ctx, t, c, "hello")
	readType(ctx, t, c, "admission") // admitted, with no waiting card
	m := readType(ctx, t, c, "rights")
	if !rightsIs(true, true)(m) {
		t.Errorf("rights after reconnect = %v, want control and clipboard", m)
	}
}

// TestGuestRights_RaceWithInput: SetRights flips while input flows; under the
// race detector, and with every queued message handled, nothing is left held.
func TestGuestRights_RaceWithInput(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	fb := &fakeInputBridge{}
	e := newGuestEnv(t, withGuestInput(fb))
	g := e.joinGuest(ctx, t, 1001, "nodeA")
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := range 40 {
			_ = e.mgr.SetRights(g.viewerID(), i%2 == 0, i%3 == 0)
		}
		_ = e.mgr.SetRights(g.viewerID(), false, false)
	}()
	for i := range 40 {
		g.input(t, map[string]any{"t": "key", "d": true, "code": "KeyA", "m": 0})
		g.input(t, map[string]any{"t": "key", "d": false, "code": "KeyA", "m": 0})
		g.press(t, float64(1000+i))
	}
	<-done
	// A heartbeat is handled after every message sent before it.
	g.input(t, map[string]any{"t": "hb"})
	eventually(t, 10*time.Second, "the heartbeat", func() bool { return guestCounter(e, "hb") == 1 })
	// A repeated down of a held button posts again, so what matters is the last event.
	keyDown, btnDown := false, false
	for _, c := range fb.snapshot() {
		switch {
		case c.Op == "key":
			keyDown = c.Down
		case c.Op == "mouse" && c.Kind == screencap.MouseDown:
			btnDown = true
		case c.Op == "mouse" && c.Kind == screencap.MouseUp:
			btnDown = false
		}
	}
	if keyDown || btnDown {
		t.Errorf("held after the run: key %v, button %v, want nothing", keyDown, btnDown)
	}
}
