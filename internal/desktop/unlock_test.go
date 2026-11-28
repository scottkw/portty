package desktop_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
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
	unlockOnReturn int           // the 1-based Return that unlocks; 0 never
	unlockDelay    time.Duration // when set, that Return unlocks the fake Mac this long after it (a slow authorization)
	unlockAt       time.Time     // when the delayed unlock lands, zero when none is pending
	flip           func() bool   // when set and true at a lock read, the fake Mac unlocks before the read answers
	wakeGate       chan struct{} // when set, Wake waits for it to close (an attempt held mid-way)
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
	if g := f.wakeGate; g != nil {
		<-g
	}
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
		if f.unlockDelay > 0 {
			f.unlockAt = f.clk.Now().Add(f.unlockDelay)
		} else {
			f.locked.Store(false)
		}
	}
	return nil
}

func (f *fakeUnlock) SessionLocked() (bool, error) {
	f.mu.Lock()
	flip := f.flip
	f.mu.Unlock()
	if flip != nil && flip() { // outside f.mu: the test's check may read the Manager
		f.locked.Store(false)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.unlockAt.IsZero() && !f.clk.Now().Before(f.unlockAt) {
		f.unlockAt = time.Time{}
		f.locked.Store(false)
	}
	return f.locked.Load(), f.lockErr
}

// set changes the fake's configuration while attempts may be running.
func (f *fakeUnlock) set(change func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	change()
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
	fi     *fakeInputBridge
	clk    *fakeClock
	locked atomic.Bool
}

func newUnlockRig(t *testing.T) *unlockRig { return newUnlockRigWith(t, nil) }

// newUnlockRigWith is newUnlockRig that lets a test adjust the rig's fakes and
// the Deps last (the Manager is built right after).
func newUnlockRigWith(t *testing.T, mod func(*unlockRig, *desktop.Deps)) *unlockRig {
	t.Helper()
	r := &unlockRig{clk: newFakeClock(), fi: &fakeInputBridge{}}
	r.locked.Store(true)
	r.fb = &fakeUnlock{clk: r.clk, locked: &r.locked, unlockOnReturn: 1}
	r.guestEnv = newGuestEnv(t, func(d *desktop.Deps) {
		d.Now = r.clk.Now
		d.Lock = func() (bool, error) { return r.locked.Load(), nil }
		d.Unlock = r.fb
		d.UnlockSleep = r.clk.Advance
		d.Input = r.fi
		d.DisplayBounds = testDisplay
		if mod != nil {
			mod(r, d)
		}
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
	return e.ownUnlockViewerAs(ctx, t, desktop.Identity{})
}

// ownUnlockViewerAs is ownUnlockViewer for an own-account viewer with an identity.
func (e *guestEnv) ownUnlockViewerAs(ctx context.Context, t *testing.T, ident desktop.Identity) *unlockViewer {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		e.mgr.ServeDesktopViewerAs(r.Context(), c, func() bool { return true }, ident)
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

// unlockStates returns the state of every unlock message the viewer has heard,
// in order, without the idle one a viewer is sent when it joins a locked Mac.
func (v *surfViewer) unlockStates() []string {
	v.mu.Lock()
	defer v.mu.Unlock()
	var out []string
	seen := 0
	for _, m := range v.msgs {
		if m["type"] != "unlock" {
			continue
		}
		seen++
		s, _ := m["state"].(string)
		if seen == 1 && s == "idle" {
			continue
		}
		out = append(out, s)
	}
	return out
}

// lastUnlock returns the latest unlock message, nil when there is none.
func (v *surfViewer) lastUnlock() map[string]any {
	v.mu.Lock()
	defer v.mu.Unlock()
	for i := len(v.msgs) - 1; i >= 0; i-- {
		if v.msgs[i]["type"] == "unlock" {
			return v.msgs[i]
		}
	}
	return nil
}

// waitUnlockStates waits until the viewer's unlock messages are exactly want.
func (v *surfViewer) waitUnlockStates(t *testing.T, want ...string) {
	t.Helper()
	eventually(t, 10*time.Second, "unlock states "+strings.Join(want, ","), func() bool {
		got := v.unlockStates()
		if len(got) != len(want) {
			return false
		}
		for i := range got {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	})
}

// settle gives a wrongly accepted request time to show itself.
func settle() { time.Sleep(200 * time.Millisecond) }

// TestUnlock_RetriesOnceThenFails: still locked 10 s after Return, the same
// strokes are typed once more and the submit fails with failures 1; there is
// never a second retry.
func TestUnlock_RetriesOnceThenFails(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	r := newUnlockRig(t)
	r.fb.unlockOnReturn = 0
	v := r.ownUnlockViewer(ctx, t)

	v.submit(t, sentinelPassword)
	v.waitUnlockStates(t, "unlocking", "retrying", "failed")
	if m := v.lastUnlock(); m["failures"] != float64(1) {
		t.Errorf("failed message %v, want failures 1", m)
	}
	settle()
	ev := r.fb.snapshot()
	n := len(sentinelPassword)
	if len(ev) != 1+n+1+n+1 {
		t.Fatalf("bridge saw %v, want wake, %d strokes, return, the same again, return and nothing more", r.fb.ops(), n)
	}
	for i := 0; i < n; i++ {
		if ev[1+i].VK != ev[n+2+i].VK {
			t.Fatalf("retried stroke %d is %#x, first time %#x: the strokes are not identical", i, ev[n+2+i].VK, ev[1+i].VK)
		}
	}
	if gap := ev[n+2].At.Sub(ev[n+1].At); gap != 10*time.Second {
		t.Errorf("Return to the retry %v, want the 10 s check", gap)
	}
	if st := r.mgr.UnlockStats(); st.Failures != 1 || st.State != "idle" {
		t.Errorf("UnlockStats = %+v, want failures 1 and idle", st)
	}
	eventually(t, 5*time.Second, "the buffer to be zeroed", r.fb.bufferZeroed)
}

// TestUnlock_RetrySucceeds: the second typing unlocks; the count stays 0.
func TestUnlock_RetrySucceeds(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	r := newUnlockRig(t)
	r.fb.unlockOnReturn = 2
	v := r.ownUnlockViewer(ctx, t)

	v.submit(t, sentinelPassword)
	v.waitUnlockStates(t, "unlocking", "retrying", "unlocked")
	if m := v.lastUnlock(); m["by_portty"] != true || m["failures"] != float64(0) {
		t.Errorf("unlocked message %v, want by_portty true and failures 0", m)
	}
}

// opCounts counts the bridge's wake, type and return calls.
func opCounts(f *fakeUnlock) (wakes, types, returns int) {
	for _, op := range f.ops() {
		switch op {
		case "wake":
			wakes++
		case "type":
			types++
		case "return":
			returns++
		}
	}
	return
}

// requireTypings fails unless the bridge saw one wake and the whole password
// typed n times, each followed by Return, and nothing else.
func requireTypings(t *testing.T, f *fakeUnlock, n int) {
	t.Helper()
	wakes, types, returns := opCounts(f)
	if wakes != 1 || types != n*len(sentinelPassword) || returns != n {
		t.Fatalf("bridge saw %d wakes, %d strokes, %d returns (%v); want 1 wake and %d typings of %d strokes each with Return", wakes, types, returns, f.ops(), n, len(sentinelPassword))
	}
}

// requireUnlockedByPortty checks a submit ended as a Portty unlock: the last
// message says unlocked by Portty with no failure, the count is 0, the re-lock
// rule is armed, and the audit lines are want.
func requireUnlockedByPortty(t *testing.T, rr *relockRig, v *unlockViewer, want ...string) {
	t.Helper()
	if m := v.lastUnlock(); m["state"] != "unlocked" || m["by_portty"] != true || m["failures"] != float64(0) {
		t.Errorf("last unlock message %v, want unlocked by Portty with failures 0", m)
	}
	if st := rr.mgr.UnlockStats(); st.Failures != 0 || !st.RelockArmed || st.State != "idle" {
		t.Errorf("UnlockStats = %+v, want idle, failures 0 and the re-lock armed", st)
	}
	if got := rr.auditEvents(t); !slices.Equal(got, want) {
		t.Errorf("audit events %v, want %v", got, want)
	}
}

// TestUnlock_LateSuccessIsOneTyping (23-10 F-late-success): a loaded Mac took
// 2.3 s to accept the password, and its lock flag clears later still. A Mac
// that reads unlocked anywhere inside the check after Return is one typing, no
// retype: unlocked by Portty, audited, the re-lock armed, no failure counted.
func TestUnlock_LateSuccessIsOneTyping(t *testing.T) {
	for _, after := range []time.Duration{2300 * time.Millisecond, 4500 * time.Millisecond, 9750 * time.Millisecond} {
		t.Run(after.String(), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			rr := newRelockRig(t, func(r *unlockRig, _ *desktop.Deps) { r.fb.unlockDelay = after })
			v := rr.ownUnlockViewerAs(ctx, t, relockWho)

			v.submit(t, sentinelPassword)
			v.waitUnlockStates(t, "unlocking", "unlocked")
			settle()
			requireTypings(t, rr.fb, 1)
			requireUnlockedByPortty(t, rr, v, "unlocked")
		})
	}
}

// TestUnlock_UnlockedBeforeTheRetypeTypesNothing: the check ends with the Mac
// still locked and the first typing's success lands after it (U-10: the retype
// began the second the Mac read unlocked, into the live desktop). A fresh lock
// read before every key stops the retype: no key and no Return once the Mac
// reads unlocked, and the submit is the first typing's success.
func TestUnlock_UnlockedBeforeTheRetypeTypesNothing(t *testing.T) {
	t.Run("between the verdict and the retype", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		rr := newRelockRig(t, func(r *unlockRig, _ *desktop.Deps) { r.fb.unlockOnReturn = 0 })
		// The Mac unlocks at the first lock read after the daemon decided to retry.
		rr.fb.set(func() { rr.fb.flip = func() bool { return rr.mgr.UnlockStats().State == "retrying" } })
		v := rr.ownUnlockViewerAs(ctx, t, relockWho)

		v.submit(t, sentinelPassword)
		v.waitUnlockStates(t, "unlocking", "retrying", "unlocked")
		settle()
		requireTypings(t, rr.fb, 1)
		requireUnlockedByPortty(t, rr, v, "unlocked")
	})

	t.Run("in the middle of the retype", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		// Unlocked 100 ms after the 10 s check ended: two retyped strokes went to the
		// still-locked screen, then the reads stop the rest and Return.
		rr := newRelockRig(t, func(r *unlockRig, _ *desktop.Deps) { r.fb.unlockDelay = 10*time.Second + 100*time.Millisecond })
		v := rr.ownUnlockViewerAs(ctx, t, relockWho)

		v.submit(t, sentinelPassword)
		v.waitUnlockStates(t, "unlocking", "retrying", "unlocked")
		settle()
		if wakes, types, returns := opCounts(rr.fb); wakes != 1 || returns != 1 || types >= 2*len(sentinelPassword) {
			t.Fatalf("bridge saw %v; want one Return and the retype stopped short", rr.fb.ops())
		}
		requireUnlockedByPortty(t, rr, v, "unlocked")
	})

	t.Run("before the retype's Return", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		rr := newRelockRig(t, func(r *unlockRig, _ *desktop.Deps) { r.fb.unlockOnReturn = 0 })
		// The Mac unlocks at the first lock read after the last retyped stroke.
		rr.fb.set(func() {
			rr.fb.flip = func() bool { _, types, _ := opCounts(rr.fb); return types == 2*len(sentinelPassword) }
		})
		v := rr.ownUnlockViewerAs(ctx, t, relockWho)

		v.submit(t, sentinelPassword)
		v.waitUnlockStates(t, "unlocking", "retrying", "unlocked")
		settle()
		if _, _, returns := opCounts(rr.fb); returns != 1 {
			t.Fatalf("bridge saw %v; want no Return after the Mac read unlocked", rr.fb.ops())
		}
		requireUnlockedByPortty(t, rr, v, "unlocked")
	})
}

// TestUnlock_WrongPasswordStaysFailed: a Mac that never unlocks gets exactly two
// typed tries, one unlock_failed line, failures 1 and no re-lock; once the late
// watch is over, an unlock by someone at the Mac is not taken for Portty's.
func TestUnlock_WrongPasswordStaysFailed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	rr := newRelockRig(t, func(r *unlockRig, _ *desktop.Deps) { r.fb.unlockOnReturn = 0 })
	v := rr.ownUnlockViewerAs(ctx, t, relockWho)

	v.submit(t, sentinelPassword)
	v.waitUnlockStates(t, "unlocking", "retrying", "failed")
	settle()
	requireTypings(t, rr.fb, 2)
	if st := rr.mgr.UnlockStats(); st.Failures != 1 || st.RelockArmed {
		t.Errorf("UnlockStats = %+v, want failures 1 and no re-lock", st)
	}
	if got := rr.auditEvents(t); !slices.Equal(got, []string{"unlock_failed"}) {
		t.Errorf("audit events %v, want one unlock_failed", got)
	}

	rr.clk.Advance(31 * time.Second)
	rr.mgr.UnlockLatePollForTest() // the watch ends here
	rr.locked.Store(false)         // someone at the Mac
	rr.mgr.UnlockLatePollForTest()
	settle()
	if st := rr.mgr.UnlockStats(); st.Failures != 1 || st.RelockArmed {
		t.Errorf("UnlockStats = %+v after an unlock at the Mac, want failures 1 and no re-lock", st)
	}
	if got := v.unlockStates(); !slices.Equal(got, []string{"unlocking", "retrying", "failed"}) {
		t.Errorf("unlock states %v, want nothing after failed", got)
	}
	requireTypings(t, rr.fb, 2)
}

// TestUnlock_UnlockedBeforeAnyKeyIsNotPorttys: a Mac unlocked at its own
// keyboard after the submit was taken but before the wake key gets no key at
// all, the wake key included; every unlock viewer is told idle, and nothing is
// counted, audited or armed.
func TestUnlock_UnlockedBeforeAnyKeyIsNotPorttys(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	rr := newRelockRig(t, nil)
	rr.fb.set(func() { rr.fb.flip = func() bool { return rr.mgr.UnlockStats().State == "unlocking" } })
	v := rr.ownUnlockViewerAs(ctx, t, relockWho)

	v.submit(t, sentinelPassword)
	v.waitUnlockStates(t, "unlocking", "idle")
	settle()
	if ops := rr.fb.ops(); len(ops) != 0 {
		t.Errorf("the bridge was asked to %v on a Mac that read unlocked", ops)
	}
	if st := rr.mgr.UnlockStats(); st.Failures != 0 || st.RelockArmed || st.State != "idle" {
		t.Errorf("UnlockStats = %+v, want idle, failures 0 and no re-lock", st)
	}
	if got := rr.auditEvents(t); len(got) != 0 {
		t.Errorf("audit events %v, want none", got)
	}
}

// TestUnlock_SuccessAfterTheVerdictCountsAsSuccess: the Mac accepts the retyped
// password only after the check has ended and failed (or paused) was reported.
// The daemon still sees it: unlocked by Portty, the failure count and any pause
// taken back, the re-lock armed, so the Mac is never left open with no re-lock.
func TestUnlock_SuccessAfterTheVerdictCountsAsSuccess(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	t.Run("after the first failure", func(t *testing.T) {
		rr := newRelockRig(t, func(r *unlockRig, _ *desktop.Deps) {
			r.fb.unlockOnReturn, r.fb.unlockDelay = 2, 12*time.Second
		})
		v := rr.ownUnlockViewerAs(ctx, t, relockWho)

		v.submit(t, sentinelPassword)
		v.waitUnlockStates(t, "unlocking", "retrying", "failed")
		if st := rr.mgr.UnlockStats(); st.Failures != 1 || st.RelockArmed {
			t.Fatalf("UnlockStats = %+v at the verdict, want failures 1 and no re-lock yet", st)
		}
		rr.clk.Advance(2 * time.Second)
		// No test call here: the lease loop's own one-second tick must catch it.
		v.waitUnlockStates(t, "unlocking", "retrying", "failed", "unlocked")
		requireTypings(t, rr.fb, 2)
		requireUnlockedByPortty(t, rr, v, "unlock_failed", "unlocked")
	})

	t.Run("after the third failure", func(t *testing.T) {
		rr := newRelockRig(t, func(r *unlockRig, _ *desktop.Deps) { r.fb.unlockOnReturn = 0 })
		v := rr.ownUnlockViewerAs(ctx, t, relockWho)
		failSubmit(t, rr.unlockRig, v, "unlocking", "retrying", "failed")
		failSubmit(t, rr.unlockRig, v, failedTwice...)
		rr.fb.set(func() { rr.fb.unlockOnReturn, rr.fb.unlockDelay = rr.fb.returns+2, 12*time.Second })
		paused := append(append([]string(nil), failedTwice...), "unlocking", "retrying", "paused")

		v.submit(t, sentinelPassword)
		v.waitUnlockStates(t, paused...)
		rr.clk.Advance(2 * time.Second)
		rr.mgr.UnlockLatePollForTest()
		v.waitUnlockStates(t, append(paused, "unlocked")...)
		if _, _, returns := opCounts(rr.fb); returns != 6 {
			t.Errorf("Return pressed %d times, want two per submit", returns)
		}
		requireUnlockedByPortty(t, rr, v, "unlock_failed", "unlock_failed", "unlock_failed", "unlocked")
	})
}

// TestUnlock_RefusalsTypeNothingAndCostNothing: Caps Lock on, a character with
// no key, Accessibility missing and a Mac that is not locked each refuse before
// anything is typed, count no failure, and leave the next submit free to run at
// once (no spacing starts).
func TestUnlock_RefusalsTypeNothingAndCostNothing(t *testing.T) {
	cases := []struct {
		name   string
		setup  func(r *unlockRig)
		undo   func(r *unlockRig)
		state  string
		reason string
	}{
		{"caps lock", func(r *unlockRig) { r.fb.set(func() { r.fb.caps = true }) }, func(r *unlockRig) { r.fb.set(func() { r.fb.caps = false }) }, "refused", "caps_lock"},
		{"unmappable", func(r *unlockRig) { r.fb.set(func() { r.fb.mapErr = desktop.ErrUnmappable }) }, func(r *unlockRig) { r.fb.set(func() { r.fb.mapErr = nil }) }, "refused", "unmappable"},
		{"accessibility", func(r *unlockRig) { r.mgr.SetControlInputForTest("needs_accessibility") }, func(r *unlockRig) { r.mgr.SetControlInputForTest("ok") }, "unavailable", "accessibility"},
		{"not locked", func(r *unlockRig) { r.locked.Store(false) }, func(r *unlockRig) { r.locked.Store(true) }, "idle", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			r := newUnlockRig(t)
			v := r.ownUnlockViewer(ctx, t)
			c.setup(r)

			v.submit(t, sentinelPassword)
			v.waitUnlockStates(t, c.state)
			m := v.lastUnlock()
			if reason, _ := m["reason"].(string); reason != c.reason || m["failures"] != float64(0) {
				t.Errorf("message %v, want reason %q and failures 0", m, c.reason)
			}
			settle()
			if ops := r.fb.ops(); len(ops) != 0 {
				t.Errorf("the bridge was asked to %v on a refusal", ops)
			}
			if st := r.mgr.UnlockStats(); st.Failures != 0 || st.State != "idle" {
				t.Errorf("UnlockStats = %+v after a refusal", st)
			}

			// No clock step: a refusal must not have started the spacing window.
			c.undo(r)
			v.submit(t, sentinelPassword)
			v.waitUnlockStates(t, c.state, "unlocking", "unlocked")
		})
	}
}

// TestUnlock_RateLimited: a submit while one runs, from another viewer, and one
// within 5 s of an attempt that typed, are refused rate_limited with the wait
// counting down; nothing is typed for them; after the wait the next one runs.
func TestUnlock_RateLimited(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	t.Run("while one runs", func(t *testing.T) {
		r := newUnlockRig(t)
		r.fb.wakeGate = make(chan struct{})
		a := r.ownUnlockViewer(ctx, t)
		b := r.ownUnlockViewer(ctx, t)
		a.submit(t, sentinelPassword)
		// The second own-account viewer sees Unlocking without having typed.
		b.waitUnlockStates(t, "unlocking")
		b.submit(t, sentinelPassword)
		b.waitUnlockStates(t, "unlocking", "rate_limited")
		if m := b.lastUnlock(); m["wait_ms"] != float64(5000) {
			t.Errorf("rate_limited message %v, want wait_ms 5000", m)
		}
		if got := a.unlockStates(); len(got) != 1 {
			t.Errorf("the first viewer heard %v, want only its own unlocking (the refusal is for the submitter)", got)
		}
		close(r.fb.wakeGate)
		a.waitUnlockStates(t, "unlocking", "unlocked")
		b.waitUnlockStates(t, "unlocking", "rate_limited", "unlocked")
		r.fb.mu.Lock()
		returns := r.fb.returns
		r.fb.mu.Unlock()
		if returns != 1 {
			t.Errorf("Return pressed %d times, want exactly one attempt", returns)
		}
	})

	t.Run("after one that typed", func(t *testing.T) {
		r := newUnlockRig(t)
		r.fb.unlockOnReturn = 0
		a := r.ownUnlockViewer(ctx, t)
		b := r.ownUnlockViewer(ctx, t)
		a.submit(t, sentinelPassword)
		a.waitUnlockStates(t, "unlocking", "retrying", "failed")
		typed := len(r.fb.snapshot())

		b.submit(t, sentinelPassword)
		b.waitUnlockStates(t, "unlocking", "retrying", "failed", "rate_limited")
		if m := b.lastUnlock(); m["wait_ms"] != float64(5000) {
			t.Errorf("rate_limited message %v, want wait_ms 5000", m)
		}
		r.clk.Advance(2 * time.Second)
		a.submit(t, sentinelPassword)
		a.waitUnlockStates(t, "unlocking", "retrying", "failed", "rate_limited")
		if m := a.lastUnlock(); m["wait_ms"] != float64(3000) {
			t.Errorf("rate_limited message %v, want wait_ms 3000 two seconds on", m)
		}
		settle()
		if n := len(r.fb.snapshot()); n != typed {
			t.Errorf("a refused submit typed: %d events, want %d", n, typed)
		}

		r.clk.Advance(3 * time.Second)
		a.submit(t, sentinelPassword)
		a.waitUnlockStates(t, "unlocking", "retrying", "failed", "rate_limited", "unlocking", "retrying", "failed")
	})
}

// failSubmit sends one failing submit, waits for the viewer to have heard want
// in all, then steps the clock past the spacing window.
func failSubmit(t *testing.T, r *unlockRig, v *unlockViewer, want ...string) {
	t.Helper()
	v.submit(t, sentinelPassword)
	v.waitUnlockStates(t, want...)
	r.clk.Advance(5 * time.Second)
}

// failedTwice is what a viewer has heard after two failed submits.
var failedTwice = []string{"unlocking", "retrying", "failed", "unlocking", "retrying", "failed"}

// TestUnlock_ThreeFailuresPause: three failed submits in a row pause unlock for
// every viewer for about 15 minutes; a fourth is refused paused; after the
// pause the next one runs, and a success resets the count.
func TestUnlock_ThreeFailuresPause(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	r := newUnlockRig(t)
	r.fb.unlockOnReturn = 0
	a := r.ownUnlockViewer(ctx, t)
	b := r.ownUnlockViewer(ctx, t)

	failSubmit(t, r, a, "unlocking", "retrying", "failed")
	failSubmit(t, r, a, failedTwice...)
	if m := a.lastUnlock(); m["failures"] != float64(2) {
		t.Errorf("second failure message %v, want failures 2", m)
	}
	a.submit(t, sentinelPassword)
	a.waitUnlockStates(t, append(append([]string(nil), failedTwice...), "unlocking", "retrying", "paused")...)
	pause := a.lastUnlock()
	if pause["wait_ms"] != float64(15*60*1000) || pause["failures"] != float64(3) {
		t.Errorf("pause message %v, want wait_ms 900000 and failures 3", pause)
	}
	// Every viewer is told, the one that did not submit too.
	eventually(t, 5*time.Second, "the other viewer to hear the pause", func() bool {
		m := b.lastUnlock()
		return m != nil && m["state"] == "paused"
	})
	if st := r.mgr.UnlockStats(); st.State != "paused" || st.Failures != 3 {
		t.Errorf("UnlockStats = %+v, want paused with 3 failures", st)
	}

	typed := len(r.fb.snapshot())
	r.clk.Advance(5 * time.Second)
	b.submit(t, sentinelPassword)
	eventually(t, 5*time.Second, "the refusal while paused", func() bool {
		m := b.lastUnlock()
		return m != nil && m["state"] == "paused" && m["wait_ms"] == float64(15*60*1000-5000)
	})
	settle()
	if n := len(r.fb.snapshot()); n != typed {
		t.Errorf("a submit while paused typed: %d events, want %d", n, typed)
	}

	// After the pause the next submit runs, and a success resets the count.
	r.clk.Advance(15 * time.Minute)
	r.fb.set(func() { r.fb.unlockOnReturn = r.fb.returns + 1 })
	a.submit(t, sentinelPassword)
	eventually(t, 10*time.Second, "the unlock after the pause", func() bool {
		m := a.lastUnlock()
		return m != nil && m["state"] == "unlocked" && m["failures"] == float64(0)
	})
	if st := r.mgr.UnlockStats(); st.Failures != 0 || st.State != "idle" {
		t.Errorf("UnlockStats = %+v after the success, want idle and 0 failures", st)
	}
}

// TestUnlock_SuccessResetsFailures: two failed submits then a success, then a
// failure counts as the first again.
func TestUnlock_SuccessResetsFailures(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	r := newUnlockRig(t)
	r.fb.unlockOnReturn = 0
	v := r.ownUnlockViewer(ctx, t)

	failSubmit(t, r, v, "unlocking", "retrying", "failed")
	failSubmit(t, r, v, failedTwice...)
	r.fb.set(func() { r.fb.unlockOnReturn = r.fb.returns + 1 })
	v.submit(t, sentinelPassword)
	eventually(t, 10*time.Second, "the success", func() bool {
		m := v.lastUnlock()
		return m != nil && m["state"] == "unlocked" && m["failures"] == float64(0)
	})

	r.clk.Advance(5 * time.Second)
	r.locked.Store(true) // the Mac locks again
	r.clk.Advance(time.Second)
	r.fb.set(func() { r.fb.unlockOnReturn = 0 })
	v.submit(t, sentinelPassword)
	eventually(t, 10*time.Second, "the failure after the reset", func() bool {
		m := v.lastUnlock()
		return m != nil && m["state"] == "failed" && m["failures"] == float64(1)
	})
}

// TestUnlock_JoinerHearsCurrentState: a viewer that joins while the Mac is
// locked is told where unlock stands once, an attempt running or the pause; one
// that joins while the Mac is unlocked is told nothing.
func TestUnlock_JoinerHearsCurrentState(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	t.Run("attempt running", func(t *testing.T) {
		r := newUnlockRig(t)
		r.fb.wakeGate = make(chan struct{})
		a := r.ownUnlockViewer(ctx, t)
		a.submit(t, sentinelPassword)
		a.waitUnlockStates(t, "unlocking")
		late := r.ownUnlockViewer(ctx, t)
		late.waitUnlockStates(t, "unlocking")
		close(r.fb.wakeGate)
		late.waitUnlockStates(t, "unlocking", "unlocked")
	})

	t.Run("paused", func(t *testing.T) {
		r := newUnlockRig(t)
		r.fb.unlockOnReturn = 0
		a := r.ownUnlockViewer(ctx, t)
		failSubmit(t, r, a, "unlocking", "retrying", "failed")
		failSubmit(t, r, a, failedTwice...)
		a.submit(t, sentinelPassword)
		a.waitUnlockStates(t, append(append([]string(nil), failedTwice...), "unlocking", "retrying", "paused")...)
		r.clk.Advance(time.Minute)
		late := r.ownUnlockViewer(ctx, t)
		late.waitUnlockStates(t, "paused")
		if m := late.lastUnlock(); m["wait_ms"] != float64(14*60*1000) || m["failures"] != float64(3) {
			t.Errorf("joiner heard %v, want paused with 14 minutes left and failures 3", m)
		}
	})

	t.Run("idle", func(t *testing.T) {
		r := newUnlockRig(t)
		late := r.ownUnlockViewer(ctx, t)
		m := late.wait(t, "unlock", unlockState("idle"))
		if m["failures"] != float64(0) {
			t.Errorf("joiner heard %v, want idle with failures 0", m)
		}
		if n := late.count("unlock"); n != 1 {
			t.Errorf("joiner heard %d unlock messages, want one", n)
		}
	})

	t.Run("not locked", func(t *testing.T) {
		r := newUnlockRig(t)
		r.locked.Store(false)
		r.clk.Advance(time.Second) // past the lock cache
		late := r.ownUnlockViewer(ctx, t)
		settle()
		if got := late.unlockStates(); len(got) != 0 {
			t.Errorf("a viewer joining an unlocked Mac heard %v", got)
		}
	})
}

// pressesOf is one key press, one text message and one button press: the three
// kinds of input the lock gate must drop.
func pressesOf(code string) []map[string]any {
	return []map[string]any{
		keyDown(code),
		{"t": "text", "s": "hi"},
		{"t": "btn", "b": 0, "d": true, "x": 0.5, "y": 0.25, "e": 1, "ts": 1000},
	}
}

// newLockGateRig is an unlock rig whose Mac starts unlocked, with one
// own-account viewer and one guest granted control and clipboard.
func newLockGateRig(ctx context.Context, t *testing.T) (r *unlockRig, own *unlockViewer, guest *guestSession) {
	t.Helper()
	r = newUnlockRigWith(t, func(r *unlockRig, _ *desktop.Deps) { r.locked.Store(false) })
	own = r.ownUnlockViewer(ctx, t)
	guest = r.joinGuest(ctx, t, 1001, "nodeA")
	if err := r.mgr.SetRights(guest.viewerID(), true, true); err != nil {
		t.Fatal(err)
	}
	guest.wait(t, "rights", rightsIs(true, true))
	return r, own, guest
}

// countCalls counts the input bridge's calls that match.
func countCalls(fb *fakeInputBridge, match func(bridgeCall) bool) int {
	n := 0
	for _, c := range fb.snapshot() {
		if match(c) {
			n++
		}
	}
	return n
}

// TestUnlockLockGate_DropsEveryViewersPresses: while the Mac reads locked, an
// own-account viewer's key press, text and button press are dropped and counted
// locked, and so are a guest's with control granted; nothing posts. Once the
// Mac is unlocked a press posts again, so the gate is the lock and nothing else.
func TestUnlockLockGate_DropsEveryViewersPresses(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	r, own, guest := newLockGateRig(ctx, t)
	var guests, owns int
	for _, v := range r.mgr.ViewerList() {
		switch v.Mode {
		case desktop.ModeGuest:
			guests++
		case desktop.ModeOwn:
			owns++
		}
	}
	if guests != 1 || owns != 1 {
		t.Fatalf("connected viewers: %d guest, %d own-account, want one of each", guests, owns)
	}

	r.locked.Store(true)
	r.clk.Advance(300 * time.Millisecond) // past the lock cache
	for _, msg := range pressesOf("KeyB") {
		own.input(t, msg)
	}
	for _, msg := range pressesOf("KeyC") {
		guest.input(t, msg)
	}
	eventually(t, 5*time.Second, "all six presses counted locked", func() bool { return r.mgr.InputCounters()["locked"] == 6 })
	settle()
	if calls := r.fi.snapshot(); len(calls) != 0 {
		t.Fatalf("a press posted while the Mac was locked: %+v", calls)
	}

	r.locked.Store(false)
	r.clk.Advance(300 * time.Millisecond)
	own.input(t, keyDown("KeyB"))
	eventually(t, 5*time.Second, "a press after the unlock", func() bool { return keyDownPosted(r.fi, 0x0B) })
}

// TestUnlockLockGate_LockBeginReleasesHeldOnce: when the screen locks while a
// viewer holds a key and a button, the daemon lets go of both, once; the
// viewer's own later releases are not posted a second time. (The lease lets one
// viewer hold at a time, so one holder with another viewer connected is the
// most a Mac can have.)
func TestUnlockLockGate_LockBeginReleasesHeldOnce(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	r, own, _ := newLockGateRig(ctx, t)

	own.input(t, keyDown("KeyA"))
	own.input(t, map[string]any{"t": "btn", "b": 0, "d": true, "x": 0.5, "y": 0.25, "e": 1, "ts": 1000})
	eventually(t, 5*time.Second, "the key and the button down", func() bool {
		return keyDownPosted(r.fi, 0x00) && countCalls(r.fi, func(c bridgeCall) bool { return c.Op == "mouse" && c.Kind == screencap.MouseDown }) == 1
	})
	before := len(r.fi.snapshot())

	r.locked.Store(true)
	r.clk.Advance(300 * time.Millisecond)
	// The input goroutine's own tick sees the lock and lets go (Phase 20, kept).
	eventually(t, 5*time.Second, "the key and the button released", func() bool { return len(r.fi.snapshot()) >= before+2 })
	own.input(t, map[string]any{"t": "key", "d": false, "code": "KeyA", "m": 0})
	own.input(t, map[string]any{"t": "btn", "b": 0, "d": false, "x": 0.5, "y": 0.25, "e": 1, "ts": 1100})
	settle()

	if n := countCalls(r.fi, func(c bridgeCall) bool { return c.Op == "key" && !c.Down && c.VK == 0x00 }); n != 1 {
		t.Errorf("the key was released %d times, want once", n)
	}
	if n := countCalls(r.fi, func(c bridgeCall) bool { return c.Op == "mouse" && c.Kind == screencap.MouseUp }); n != 1 {
		t.Errorf("the button was released %d times, want once", n)
	}
	if got := len(r.fi.snapshot()); got != before+2 {
		t.Errorf("%d posts after the lock, want exactly the two releases", got-before)
	}
}

// TestUnlockLockGate_AttemptDoesNotLoosenIt: presses keep being dropped while an
// unlock attempt runs; only the attempt's own typing reaches the lock screen,
// and it never goes through the input bridge.
func TestUnlockLockGate_AttemptDoesNotLoosenIt(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	r := newUnlockRig(t) // the Mac starts locked
	gate := make(chan struct{})
	r.fb.wakeGate = gate // the attempt waits at the wake key
	v := r.ownUnlockViewer(ctx, t)

	v.submit(t, sentinelPassword)
	v.waitUnlockStates(t, "unlocking")
	for _, msg := range pressesOf("KeyB") {
		v.input(t, msg)
	}
	eventually(t, 5*time.Second, "the presses counted locked mid-attempt", func() bool { return r.mgr.InputCounters()["locked"] == 3 })
	if calls := r.fi.snapshot(); len(calls) != 0 {
		t.Fatalf("a press posted during an unlock attempt: %+v", calls)
	}
	close(gate)
	v.waitUnlockStates(t, "unlocking", "unlocked")
	if calls := r.fi.snapshot(); len(calls) != 0 {
		t.Errorf("the input bridge posted %+v; the attempt types through the unlock bridge only", calls)
	}
	if got := len(r.fb.snapshot()); got != len(sentinelPassword)+2 {
		t.Errorf("unlock bridge saw %d events, want wake, the strokes and return", got)
	}
}
