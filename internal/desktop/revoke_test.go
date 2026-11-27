package desktop_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/scottkw/portty/internal/desktop"
	"github.com/scottkw/portty/internal/screencap"
)

// gatedBridge is the fake bridge whose first mouse post blocks until gate is
// closed, so a test can hold the input goroutine while messages queue behind it.
type gatedBridge struct {
	*fakeInputBridge
	gate    chan struct{}
	entered chan struct{}
	once    sync.Once
}

func newGatedBridge() *gatedBridge {
	return &gatedBridge{fakeInputBridge: &fakeInputBridge{}, gate: make(chan struct{}), entered: make(chan struct{})}
}

func (b *gatedBridge) PostMouse(kind screencap.MouseKind, x, y float64, button, clickState, dx, dy int, flags uint64) error {
	b.once.Do(func() {
		close(b.entered)
		<-b.gate
	})
	return b.fakeInputBridge.PostMouse(kind, x, y, button, clickState, dx, dy, flags)
}

// callsOf returns the recorded calls with op.
func callsOf(fb *fakeInputBridge, op string) []bridgeCall {
	var out []bridgeCall
	for _, c := range fb.snapshot() {
		if c.Op == op {
			out = append(out, c)
		}
	}
	return out
}

// grantControl gives the guest control and waits until it knows.
func grantControl(t *testing.T, e *guestEnv, g *guestSession) {
	t.Helper()
	if err := e.mgr.SetRights(g.viewerID(), true, false); err != nil {
		t.Fatal(err)
	}
	g.wait(t, "rights", rightsIs(true, false))
}

// TestGuestRevoke_QueuedInputNeverPostsAndHeldIsReleased: a guest holds a key
// and a button, input is queued behind a slow post, and CloseViewer returns:
// the queued input never reaches the bridge, the key and button are let go, the
// socket closes 4432 revoked and the leave hook fires once.
func TestGuestRevoke_QueuedInputNeverPostsAndHeldIsReleased(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	gb := newGatedBridge()
	e := newGuestEnv(t, func(d *desktop.Deps) {
		withGuestInput(gb.fakeInputBridge)(d)
		d.Input = gb
	})
	g := e.joinGuest(ctx, t, 1001, "nodeA")
	grantControl(t, e, g)

	g.input(t, map[string]any{"t": "key", "d": true, "code": "KeyA", "m": 0})
	gb.waitFor(t, 1)
	g.press(t, 1000) // blocks the input goroutine inside the bridge
	<-gb.entered
	for _, s := range []string{"x", "y", "z"} {
		g.input(t, map[string]any{"t": "text", "s": s})
	}
	time.Sleep(200 * time.Millisecond) // the three texts are queued behind the blocked post

	go func() {
		time.Sleep(150 * time.Millisecond)
		close(gb.gate)
	}()
	if err := e.mgr.CloseViewer(g.viewerID()); err != nil {
		t.Fatal(err)
	}

	if n := len(callsOf(gb.fakeInputBridge, "text")); n != 0 {
		t.Errorf("%d queued texts reached the bridge after CloseViewer returned, want 0", n)
	}
	vk, _ := screencap.KeyCode("KeyA")
	var ups int
	for _, c := range callsOf(gb.fakeInputBridge, "key") {
		if c.VK == vk && !c.Down {
			ups++
		}
	}
	if ups != 1 {
		t.Errorf("%d key-ups for the held key, want 1", ups)
	}
	var mouseUps int
	for _, c := range callsOf(gb.fakeInputBridge, "mouse") {
		if c.Kind == screencap.MouseUp {
			mouseUps++
		}
	}
	if mouseUps != 1 {
		t.Errorf("%d mouse-ups for the held button, want 1", mouseUps)
	}
	if code, reason := g.waitClosed(t, 5*time.Second); code != desktop.CloseGuestRevoked || reason != "revoked" {
		t.Errorf("close %d %q, want 4432 revoked", code, reason)
	}
	eventually(t, 5*time.Second, "the leave hook", func() bool { return e.leaves.Load() == 1 })
	time.Sleep(100 * time.Millisecond)
	if n := e.leaves.Load(); n != 1 {
		t.Errorf("OnViewerLeave fired %d times, want 1", n)
	}
}

// TestGuestRevoke_DenyListsTheUser: a revoked guest's person is refused for the
// rest of the share, from another device too.
func TestGuestRevoke_DenyListsTheUser(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	e := newGuestEnv(t, withGuestInput(&fakeInputBridge{}))
	g := e.joinGuest(ctx, t, 1001, "nodeA")
	if err := e.mgr.CloseViewer(g.viewerID()); err != nil {
		t.Fatal(err)
	}
	if code, reason := g.waitClosed(t, 5*time.Second); code != desktop.CloseGuestRevoked || reason != "revoked" {
		t.Fatalf("close %d %q, want 4432 revoked", code, reason)
	}
	if !e.gs.IsDenied(1001) {
		t.Error("the revoked user is not on the deny-list")
	}

	c := e.dial(ctx, t, 1001, "nodeB")
	writeJSON(ctx, t, c, map[string]any{"type": "hello", "v": 1, "caps": map[string]int{"video": 1, "input": 1}})
	if code, reason := closeOf(ctx, t, c); code != desktop.CloseGuestDenied || reason != "denied" {
		t.Errorf("a new knock from another node closed %d %q, want 4430 denied", code, reason)
	}
	// A reconnect from the same node is refused too (the stored admission is gone).
	c = e.dial(ctx, t, 1001, "nodeA")
	writeJSON(ctx, t, c, map[string]any{"type": "hello", "v": 1, "caps": map[string]int{"video": 1, "input": 1}})
	if code, _ := closeOf(ctx, t, c); code != desktop.CloseGuestDenied {
		t.Errorf("a reconnect from the revoked node closed %d, want 4430", code)
	}
}

// TestGuestRevoke_CutsEveryDeviceOfThePerson (WR-03): the person is the unit.
// Disconnecting one row of a person watching on two devices cuts both; another
// person stays.
func TestGuestRevoke_CutsEveryDeviceOfThePerson(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	e := newGuestEnv(t, withGuestInput(&fakeInputBridge{}))
	laptop := e.joinGuest(ctx, t, 1001, "nodeA")
	phone := e.joinGuest(ctx, t, 1001, "nodeB")
	other := e.joinGuest(ctx, t, 1002, "nodeC")
	if err := e.mgr.CloseViewer(laptop.viewerID()); err != nil {
		t.Fatal(err)
	}
	if code, _ := laptop.waitClosed(t, 5*time.Second); code != desktop.CloseGuestRevoked {
		t.Errorf("disconnected device closed %d, want 4432", code)
	}
	if code, _ := phone.waitClosed(t, 5*time.Second); code != desktop.CloseGuestRevoked {
		t.Errorf("the same person's other device closed %d, want 4432", code)
	}
	eventually(t, 5*time.Second, "only the other person to be left", func() bool { return e.mgr.Viewers() == 1 })
	select {
	case <-other.closed:
		t.Error("another person's connection was cut")
	default:
	}
}

// TestGuestDeny_CutsThePersonsLiveDevices (WR-03): denying a person's second
// device also cuts the device they are already watching on, so the deny-list
// and the live connections agree.
func TestGuestDeny_CutsThePersonsLiveDevices(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	e := newGuestEnv(t, withGuestInput(&fakeInputBridge{}))
	laptop := e.joinGuest(ctx, t, 1001, "nodeA")
	other := e.joinGuest(ctx, t, 1002, "nodeC")
	phone := e.knock(ctx, t, 1001, "nodeB")
	req := e.pendingN(t, 1)[0]
	if err := e.mgr.DenyGuest(e.gs, req.ID); err != nil {
		t.Fatal(err)
	}
	if code, reason := closeOf(ctx, t, phone); code != desktop.CloseGuestDenied || reason != "denied" {
		t.Errorf("denied device closed %d %q, want 4430 denied", code, reason)
	}
	if code, _ := laptop.waitClosed(t, 5*time.Second); code != desktop.CloseGuestRevoked {
		t.Errorf("the denied person's watching device closed %d, want 4432", code)
	}
	if err := e.mgr.DenyGuest(e.gs, req.ID); !errors.Is(err, desktop.ErrNoRequest) {
		t.Errorf("denying an answered request = %v, want ErrNoRequest", err)
	}
	select {
	case <-other.closed:
		t.Error("another person's connection was cut")
	default:
	}
}

// TestGuestRevoke_GrantAfterTheCutIsRefused (IN-02): a viewer being cut stays
// registered until its socket closes; a grant in that window must not switch
// control or clipboard back on.
func TestGuestRevoke_GrantAfterTheCutIsRefused(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	e := newGuestEnv(t, withGuestInput(&fakeInputBridge{}))
	g := e.joinGuestAs(ctx, t, 1001, "nodeA", false) // never answers the close: stays registered a while
	if err := e.mgr.CloseViewer(g.viewerID()); err != nil {
		t.Fatal(err)
	}
	if err := e.mgr.SetRights(g.viewerID(), true, true); !errors.Is(err, desktop.ErrNoViewer) {
		t.Errorf("SetRights on a cut viewer = %v, want ErrNoViewer", err)
	}
	for _, v := range e.mgr.ViewerList() {
		if v.ID == g.viewerID() && v.Rights != desktop.RightsViewOnly {
			t.Errorf("a cut viewer has rights %v again", v.Rights)
		}
	}
}

// TestGuestRevoke_OwnAccountClosesGoingAway: an own-account device is closed
// 1001 and is not deny-listed, so it may reconnect.
func TestGuestRevoke_OwnAccountClosesGoingAway(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	env := newTracerServer(t)
	v := env.join(ctx, t, env.token(t, "g1"))
	list := env.mgr.ViewerList()
	if len(list) != 1 {
		t.Fatalf("%d viewers, want 1", len(list))
	}
	if err := env.mgr.CloseViewer(list[0].ID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-v.closed:
	case <-time.After(5 * time.Second):
		t.Fatal("the own-account socket did not close")
	}
	v.mu.Lock()
	code := v.closeCode
	v.mu.Unlock()
	if code != websocket.StatusGoingAway {
		t.Errorf("close code %d, want 1001", code)
	}
	env.join(ctx, t, env.token(t, "g1")) // may come back
	eventually(t, 5*time.Second, "the device to be back", func() bool { return env.mgr.Viewers() >= 1 })
}

// TestGuestRevoke_UnknownViewer: an ID that is not connected is reported.
func TestGuestRevoke_UnknownViewer(t *testing.T) {
	e := newGuestEnv(t, withGuestInput(&fakeInputBridge{}))
	if err := e.mgr.CloseViewer("nobody"); !errors.Is(err, desktop.ErrNoViewer) {
		t.Fatalf("CloseViewer of an unknown ID = %v, want ErrNoViewer", err)
	}
}

// TestGuestRevoke_ConcurrentAndRepeatedCallsReleaseOnce: CloseViewer twice and
// CloseGuests at the same moment neither panic nor release a held key twice.
func TestGuestRevoke_ConcurrentAndRepeatedCallsReleaseOnce(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	fb := &fakeInputBridge{}
	e := newGuestEnv(t, withGuestInput(fb))
	g := e.joinGuest(ctx, t, 1001, "nodeA")
	grantControl(t, e, g)
	g.input(t, map[string]any{"t": "key", "d": true, "code": "KeyA", "m": 0})
	fb.waitFor(t, 1)

	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := e.mgr.CloseViewer(g.viewerID()); err != nil && !errors.Is(err, desktop.ErrNoViewer) {
				t.Errorf("CloseViewer: %v", err)
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		e.mgr.CloseGuests(desktop.CloseShareEnded, "share_ended")
	}()
	wg.Wait()
	g.waitClosed(t, 5*time.Second)
	eventually(t, 5*time.Second, "the viewer to leave", func() bool { return e.mgr.Viewers() == 0 })

	vk, _ := screencap.KeyCode("KeyA")
	var ups int
	for _, c := range callsOf(fb, "key") {
		if c.VK == vk && !c.Down {
			ups++
		}
	}
	if ups != 1 {
		t.Errorf("%d key-ups, want exactly 1", ups)
	}
	// The viewer count drops (m.leave) just before the leave hook runs, so wait
	// for the hook, then make sure it does not fire a second time.
	eventually(t, 5*time.Second, "OnViewerLeave to fire", func() bool { return e.leaves.Load() >= 1 })
	time.Sleep(50 * time.Millisecond)
	if n := e.leaves.Load(); n != 1 {
		t.Errorf("OnViewerLeave fired %d times, want 1", n)
	}
}

// TestGuestStop_ThreeSilentPeersDoNotHoldItUp: three guests that never answer
// the close handshake are cut at once and concurrently, every socket is ended
// within the ceiling, and an own-account viewer stays connected.
func TestGuestStop_ThreeSilentPeersDoNotHoldItUp(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	e := newGuestEnv(t, withGuestInput(&fakeInputBridge{}))
	e.ownViewer(ctx, t)
	for i, node := range []string{"nodeA", "nodeB", "nodeC"} {
		e.joinGuestAs(ctx, t, int64(1001+i), node, false)
	}
	eventually(t, 5*time.Second, "four viewers", func() bool { return e.mgr.Viewers() == 4 })

	e.gs.End("stopped")
	start := time.Now()
	e.mgr.CloseGuests(desktop.CloseShareEnded, "share_ended")
	if d := time.Since(start); d >= 2*time.Second {
		t.Errorf("CloseGuests took %v with three silent peers, want under 2 s", d)
	}
	// Each socket is force-closed once the ceiling passes, all three together
	// (serial closes would take 3 x the ceiling or more).
	eventually(t, 8*time.Second, "the guests to be gone", func() bool { return e.mgr.Viewers() == 1 })
	list := e.mgr.ViewerList()
	if len(list) != 1 || list[0].Mode != desktop.ModeOwn {
		t.Fatalf("remaining viewers %+v, want the one own-account viewer", list)
	}
}

// TestGuestStop_LateGuestsAreRefused: a guest that knocks after the share ended
// closes 4433, and one that was admitted but joins after the Stop fails its
// re-check and closes 4433 too.
func TestGuestStop_LateGuestsAreRefused(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	e := newGuestEnv(t, withGuestInput(&fakeInputBridge{}))

	// Admitted, offer not sent yet.
	c := e.dial(ctx, t, 1001, "nodeA")
	writeJSON(ctx, t, c, map[string]any{"type": "hello", "v": 1, "caps": map[string]int{"video": 1, "input": 1}})
	readType(ctx, t, c, "hello")
	readType(ctx, t, c, "admission")
	e.gs.Admit(e.pendingN(t, 1)[0].ID) //nolint:errcheck // the request is pending
	readType(ctx, t, c, "rights")

	e.gs.End("stopped")
	e.mgr.CloseGuests(desktop.CloseShareEnded, "share_ended") // nobody is joined yet

	lc := newLoopbackClient(t)
	writeJSON(ctx, t, c, map[string]any{"type": "offer", "sdp": lc.offer(t)})
	if code, reason := closeOf(ctx, t, c); code != desktop.CloseShareEnded || reason != "share_ended" {
		t.Errorf("the guest that joined after Stop closed %d %q, want 4433 share_ended", code, reason)
	}
	eventually(t, 5*time.Second, "no viewer left", func() bool { return e.mgr.Viewers() == 0 })

	late := e.dial(ctx, t, 1002, "nodeB")
	writeJSON(ctx, t, late, map[string]any{"type": "hello", "v": 1, "caps": map[string]int{"video": 1, "input": 1}})
	if code, reason := closeOf(ctx, t, late); code != desktop.CloseShareEnded || reason != "share_ended" {
		t.Errorf("a knock after Stop closed %d %q, want 4433 share_ended", code, reason)
	}
}
