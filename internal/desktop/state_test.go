package desktop_test

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/desktop"
	"github.com/scottkw/portty/internal/screencap"
)

// TestState_PushTracer: the lock and the display state reach a connected viewer
// as state messages within 1.5 s, and Status() says the same (the default
// one-second loop and the real clock, on purpose).
func TestState_PushTracer(t *testing.T) {
	w := newWorld()
	env := newTracerServer(t, func(_ *tracerEnv, d *desktop.Deps) {
		w.opt()(nil, d)
		d.Now = nil // real clock: the Locked() cache must expire in real time
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	v := env.join(ctx, t, env.token(t, "g1"))

	expect := func(what, state string, locked bool) {
		t.Helper()
		eventually(t, 1500*time.Millisecond, what, func() bool {
			s, n, l := v.lastState()
			return s == state && n == 1 && l == locked
		})
		if st := env.mgr.Status(); st.State != desktop.State(state) || st.Viewers != 1 || st.Locked != locked {
			t.Fatalf("%s: Status() = %+v, want state %s viewers 1 locked %v", what, st, state, locked)
		}
	}
	expect("streaming with one viewer", "streaming", false)

	w.set(func() { w.locked = true })
	expect("locked", "locked", true)

	w.set(func() { w.asleep = true })
	expect("display asleep wins over locked", "display_asleep", true)

	w.set(func() { w.locked, w.asleep = false, false })
	expect("streaming again", "streaming", false)
}

// TestState_Precedence walks every flag combination against the precedence
// capture_paused > needs_permission > display_asleep > awaiting_approval >
// locked > streaming > idle.
func TestState_Precedence(t *testing.T) {
	for bits := 0; bits < 32; bits++ {
		for _, viewers := range []int{0, 2} {
			paused, perm, asleep, awaiting, locked := bits&1 != 0, bits&2 != 0, bits&4 != 0, bits&8 != 0, bits&16 != 0
			want := desktop.StateIdle
			switch {
			case paused:
				want = desktop.StateCapturePaused
			case perm:
				want = desktop.StateNeedsPermission
			case asleep:
				want = desktop.StateDisplayAsleep
			case awaiting:
				want = desktop.StateAwaitingApproval
			case locked:
				want = desktop.StateLocked
			case viewers > 0:
				want = desktop.StateStreaming
			}
			if got := desktop.PickStateForTest(paused, perm, asleep, awaiting, locked, viewers); got != want {
				t.Errorf("paused=%v perm=%v asleep=%v awaiting=%v locked=%v viewers=%d: got %s, want %s",
					paused, perm, asleep, awaiting, locked, viewers, got, want)
			}
		}
	}
}

func newBareManager(t *testing.T, w *world) *desktop.Manager {
	t.Helper()
	d := desktop.Deps{
		Bind: netip.MustParseAddr("100.64.0.1"), MediaPort: 7444,
		NewSource: func() screencap.Source { return screencap.NewSyntheticSource(64, 64) },
	}
	w.opt()(nil, &d)
	m, err := desktop.NewManager(d)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// TestLocked_CacheAndFailClosed: two calls inside 250 ms read once; a read
// error returns true (Phase 20's input gate fails closed) and sets a reason.
func TestLocked_CacheAndFailClosed(t *testing.T) {
	w := newWorld()
	m := newBareManager(t, w)

	if m.Locked() {
		t.Fatal("unlocked Mac read as locked")
	}
	w.set(func() { w.locked = true })
	w.clock.Advance(100 * time.Millisecond)
	if m.Locked() {
		t.Fatal("cache should still answer false inside 250 ms")
	}
	w.mu.Lock()
	reads := w.lockReads
	w.mu.Unlock()
	if reads != 1 {
		t.Fatalf("two calls inside the TTL read the lock %d times, want 1", reads)
	}
	w.clock.Advance(200 * time.Millisecond)
	if !m.Locked() {
		t.Fatal("after the TTL the new value (locked) must be read")
	}

	w.set(func() { w.lockErr = errors.New("no window server session") })
	w.clock.Advance(time.Second)
	if !m.Locked() {
		t.Fatal("a failed read must return true (fail closed)")
	}
	if r := m.Status().Reason; r == "" {
		t.Fatal("a failed lock read must be recorded in Status.Reason")
	}
	w.set(func() { w.lockErr, w.locked = nil, false })
	w.clock.Advance(time.Second)
	if m.Locked() {
		t.Fatal("recovered read must return the real value")
	}
	if r := m.Status().Reason; r != "" {
		t.Fatalf("reason %q should clear once the lock reads again", r)
	}
}

// TestProber_StrictAndCached: the child's JSON is decoded strictly and cached
// for a second; any failure is an error, never a default false.
func TestProber_StrictAndCached(t *testing.T) {
	cases := []struct {
		out string
		ok  bool
	}{
		{`{"sr":true,"ax":false,"pe":true}`, true},
		{`{"sr":true,"ax":false}`, false},
		{`{"sr":true,"ax":false,"pe":true,"x":1}`, false},
		{`{"sr":true,"ax":false,"pe":true} junk`, false},
		{`not json`, false},
		{``, false},
	}
	for _, c := range cases {
		got, err := desktop.DecodeProbeForTest([]byte(c.out))
		if (err == nil) != c.ok {
			t.Errorf("decode %q: err = %v, want ok=%v", c.out, err, c.ok)
		}
		if c.ok && (!got.SR || got.AX || !got.PE) {
			t.Errorf("decode %q: got %+v", c.out, got)
		}
	}

	calls := 0
	clk := newFakeClock()
	p := desktop.NewProberForTest(clk.Now, func(_ context.Context, args ...string) ([]byte, error) {
		calls++
		if len(args) != 2 || args[0] != "daemon" || args[1] != "perm-probe" {
			t.Errorf("child args %v, want [daemon perm-probe]", args)
		}
		return []byte(`{"sr":true,"ax":true,"pe":false}`), nil
	})
	for i := 0; i < 3; i++ {
		if _, err := p.Read(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Fatalf("three reads inside a second spawned %d children, want 1", calls)
	}
	clk.Advance(1100 * time.Millisecond)
	if _, err := p.Read(context.Background()); err != nil || calls != 2 {
		t.Fatalf("a read after the TTL must spawn again (calls %d, err %v)", calls, err)
	}

	boom := errors.New("spawn failed")
	p = desktop.NewProberForTest(clk.Now, func(context.Context, ...string) ([]byte, error) { return nil, boom })
	if got, err := p.Read(context.Background()); !errors.Is(err, boom) || got != (screencap.Permissions{}) {
		t.Fatalf("a failed spawn must be an error, got %+v, %v", got, err)
	}
}

// TestIdleProbe_NeedsPermission: with no viewer the loop probes through the
// child path at most every five minutes; SR false is needs_permission and a
// probe error leaves the previous answer.
func TestIdleProbe_NeedsPermission(t *testing.T) {
	w := newWorld()
	env := newTracerServer(t, w.opt())
	probes := func() int {
		w.mu.Lock()
		defer w.mu.Unlock()
		return w.probes
	}
	eventually(t, 2*time.Second, "first idle probe on the first tick", func() bool { return probes() == 1 })

	w.set(func() { w.perms.SR = false })
	w.clock.Advance(4 * time.Minute)
	time.Sleep(100 * time.Millisecond)
	if probes() != 1 || env.mgr.Status().State != desktop.StateIdle {
		t.Fatalf("inside the 5 minute cadence: probes %d, state %s; want 1 and idle", probes(), env.mgr.Status().State)
	}
	w.clock.Advance(61 * time.Second)
	eventually(t, 2*time.Second, "needs_permission after the cadence", func() bool {
		return env.mgr.Status().State == desktop.StateNeedsPermission
	})

	w.set(func() { w.perms.SR, w.probeErr = true, errors.New("child failed") })
	w.clock.Advance(6 * time.Minute)
	eventually(t, 2*time.Second, "probe error recorded", func() bool { return env.mgr.Status().Reason != "" })
	if st := env.mgr.Status().State; st != desktop.StateNeedsPermission {
		t.Fatalf("a probe error must keep the previous answer, state %s", st)
	}

	w.set(func() { w.probeErr = nil })
	w.clock.Advance(6 * time.Minute)
	eventually(t, 2*time.Second, "idle again once SR reads true", func() bool {
		st := env.mgr.Status()
		return st.State == desktop.StateIdle && st.Reason == ""
	})
}
