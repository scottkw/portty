package desktop_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/scottkw/portty/internal/desktop"
	"github.com/scottkw/portty/internal/screencap"
)

// sentinelPassword is a made-up string, not a real password.
const sentinelPassword = "Zq7-sentinel-xk29"

// unlockEvent is one thing the fake bridge was asked to do, with the fake time.
type unlockEvent struct {
	At    time.Time
	Op    string // "wake", "type", "return"
	VK    uint16
	Flags uint64
}

// fakeUnlock is the UnlockBridge of a test: it maps each byte to the key of the
// same number, records what is posted with the fake clock's time, and unlocks
// the fake Mac when Return is pressed for the nth time (unlockOnReturn).
type fakeUnlock struct {
	clk    *fakeClock
	locked *atomic.Bool

	mu             sync.Mutex
	events         []unlockEvent
	buf            []byte // the very slice MapRunes was handed: a test checks it is zeroed
	caps           bool
	mapErr         error
	capsErr        error
	lockErr        error
	typeErr        error
	typePanic      bool
	returns        int
	unlockOnReturn int // the 1-based Return that unlocks; 0 never
}

func (f *fakeUnlock) record(op string, vk uint16, flags uint64) {
	f.events = append(f.events, unlockEvent{f.clk.Now(), op, vk, flags})
}

func (f *fakeUnlock) MapRunes(b []byte) ([]screencap.KeyStroke, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.buf = b
	if f.mapErr != nil {
		return nil, f.mapErr
	}
	out := make([]screencap.KeyStroke, len(b))
	for i, c := range b {
		out[i] = screencap.KeyStroke{VK: uint16(c)}
	}
	return out, nil
}

func (f *fakeUnlock) CapsLockOn() (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.caps, f.capsErr
}

func (f *fakeUnlock) Wake(vk uint16) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("wake", vk, 0)
	return nil
}

func (f *fakeUnlock) Type(s screencap.KeyStroke) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.typePanic {
		panic("type exploded on " + sentinelPassword)
	}
	if f.typeErr != nil {
		return f.typeErr
	}
	f.record("type", s.VK, s.Flags)
	return nil
}

func (f *fakeUnlock) Return() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.returns++
	f.record("return", 0, 0)
	if f.unlockOnReturn != 0 && f.returns == f.unlockOnReturn {
		f.locked.Store(false)
	}
	return nil
}

func (f *fakeUnlock) SessionLocked() (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.locked.Load(), f.lockErr
}

func (f *fakeUnlock) snapshot() []unlockEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]unlockEvent(nil), f.events...)
}

// ops lists the recorded operations in order, one word each ("wake", "type", "return").
func (f *fakeUnlock) ops() []string {
	var out []string
	for _, e := range f.snapshot() {
		out = append(out, e.Op)
	}
	return out
}

// bufferZeroed reports whether the buffer MapRunes was handed is all zero now.
func (f *fakeUnlock) bufferZeroed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.buf == nil {
		return false
	}
	for _, c := range f.buf {
		if c != 0 {
			return false
		}
	}
	return true
}

// unlockRig is a guestEnv (own-account and guest viewers on one Manager) whose
// lock state, unlock bridge and unlock waits are fakes on one fake clock.
type unlockRig struct {
	*guestEnv
	fb     *fakeUnlock
	clk    *fakeClock
	locked atomic.Bool
}

func newUnlockRig(t *testing.T) *unlockRig {
	t.Helper()
	r := &unlockRig{clk: newFakeClock()}
	r.locked.Store(true)
	r.fb = &fakeUnlock{clk: r.clk, locked: &r.locked, unlockOnReturn: 1}
	r.guestEnv = newGuestEnv(t, func(d *desktop.Deps) {
		d.Now = r.clk.Now
		d.Lock = func() (bool, error) { return r.locked.Load(), nil }
		d.Unlock = r.fb
		d.UnlockSleep = r.clk.Advance
		d.Input = &fakeInputBridge{}
		d.DisplayBounds = testDisplay
	})
	return r
}

// unlockViewer is an own-account viewer with an open input channel and the
// server hello it was given.
type unlockViewer struct {
	*surfViewer
	hello map[string]any
}

// ownUnlockViewer joins an own-account viewer through a bare route, like
// guestEnv.ownViewer, and keeps its hello.
func (e *guestEnv) ownUnlockViewer(ctx context.Context, t *testing.T) *unlockViewer {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		e.mgr.ServeDesktopViewer(r.Context(), c, func() bool { return true })
	}))
	t.Cleanup(srv.Close)
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.CloseNow() })
	writeJSON(ctx, t, c, map[string]any{"type": "hello", "v": 1, "caps": map[string]int{"video": 1, "input": 1}})
	hello := readType(ctx, t, c, "hello")
	lc := newLoopbackClient(t)
	dc, err := lc.pc.CreateDataChannel("input", nil)
	if err != nil {
		t.Fatal(err)
	}
	open := make(chan struct{})
	dc.OnOpen(func() { close(open) })
	writeJSON(ctx, t, c, map[string]any{"type": "offer", "sdp": lc.offer(t)})
	lc.accept(t, readType(ctx, t, c, "answer")["sdp"].(string))
	v := &surfViewer{id: hello["viewer"].(string), conn: c, dc: dc}
	go v.read(ctx)
	select {
	case <-open:
	case <-time.After(10 * time.Second):
		t.Fatal("the input channel did not open")
	}
	return &unlockViewer{surfViewer: v, hello: hello}
}

// submit sends one unlock request: the type byte, then the password bytes.
func (v *surfViewer) submit(t *testing.T, password string) {
	t.Helper()
	if err := v.dc.Send(append([]byte{0x01}, password...)); err != nil {
		t.Fatal(err)
	}
}

func unlockState(state string) func(map[string]any) bool {
	return func(m map[string]any) bool { return m["state"] == state }
}

func helloCap(hello map[string]any, name string) float64 {
	caps, _ := hello["caps"].(map[string]any)
	n, _ := caps[name].(float64)
	return n
}

// TestUnlock_Tracer: an own-account viewer's hello carries caps unlock 1; its
// binary unlock request is mapped, the wake key precedes the first stroke by
// the settle time, the strokes are 60 ms apart, Return follows, the viewer is
// told unlocking and then unlocked by Portty, and the buffer is zero after.
func TestUnlock_Tracer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	r := newUnlockRig(t)
	v := r.ownUnlockViewer(ctx, t)
	if got := helloCap(v.hello, "unlock"); got != 1 {
		t.Fatalf("own-account hello caps unlock = %v, want 1 (hello %v)", got, v.hello)
	}

	v.submit(t, sentinelPassword)
	v.wait(t, "unlock", unlockState("unlocking"))
	done := v.wait(t, "unlock", unlockState("unlocked"))
	if done["by_portty"] != true || done["failures"] != float64(0) {
		t.Errorf("unlocked message %v, want by_portty true and failures 0", done)
	}

	ev := r.fb.snapshot()
	n := len(sentinelPassword)
	if len(ev) != n+2 || ev[0].Op != "wake" || ev[n+1].Op != "return" {
		t.Fatalf("bridge saw %v, want wake, %d strokes, return", r.fb.ops(), n)
	}
	if ev[0].VK != 0x38 {
		t.Errorf("wake key %#x, want kVK_Shift 0x38", ev[0].VK)
	}
	if gap := ev[1].At.Sub(ev[0].At); gap != time.Second {
		t.Errorf("wake to first stroke %v, want the 1 s settle", gap)
	}
	for i := 1; i <= n; i++ {
		if ev[i].Op != "type" || ev[i].VK != uint16(sentinelPassword[i-1]) {
			t.Fatalf("event %d = %+v, want the stroke of %q", i, ev[i], sentinelPassword[i-1])
		}
		if i > 1 {
			if gap := ev[i].At.Sub(ev[i-1].At); gap != 60*time.Millisecond {
				t.Errorf("stroke %d is %v after the previous, want 60 ms", i, gap)
			}
		}
	}
	eventually(t, 5*time.Second, "the buffer to be zeroed", r.fb.bufferZeroed)
	if st := r.mgr.UnlockStats(); st.State != "idle" || st.Failures != 0 {
		t.Errorf("UnlockStats = %+v after a success", st)
	}
}

// TestUnlock_GuestNeverUnlocks: a guest granted control and clipboard sends the
// same message; nothing is typed, nothing is sent back, and its hello has no
// unlock capability.
func TestUnlock_GuestNeverUnlocks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	r := newUnlockRig(t)
	g := r.joinGuest(ctx, t, 1001, "nodeA")
	if got := helloCap(g.hello, "unlock"); got != 0 {
		t.Errorf("guest hello caps unlock = %v, want absent", got)
	}
	if err := r.mgr.SetRights(g.viewerID(), true, true); err != nil {
		t.Fatal(err)
	}
	g.wait(t, "rights", rightsIs(true, true))

	g.submit(t, sentinelPassword)
	eventually(t, 5*time.Second, "unlock_forbidden counted", func() bool {
		return r.mgr.InputCounters()["unlock_forbidden"] == 1
	})
	time.Sleep(200 * time.Millisecond) // a wrongly accepted request would be typing by now
	if ops := r.fb.ops(); len(ops) != 0 {
		t.Errorf("the bridge was asked to %v for a guest", ops)
	}
	if n := g.count("unlock"); n != 0 {
		t.Errorf("the guest was sent %d unlock messages, want 0", n)
	}
	if !r.locked.Load() {
		t.Error("the fake Mac was unlocked by a guest")
	}
}
