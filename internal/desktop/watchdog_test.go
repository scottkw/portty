package desktop_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/desktop"
	"github.com/scottkw/portty/internal/screencap"
)

// logSink collects slog messages so a test can count a log line.
type logSink struct {
	mu    sync.Mutex
	lines []string
}

func (l *logSink) Enabled(context.Context, slog.Level) bool { return true }
func (l *logSink) Handle(_ context.Context, r slog.Record) error {
	l.mu.Lock()
	l.lines = append(l.lines, r.Message)
	l.mu.Unlock()
	return nil
}
func (l *logSink) WithAttrs([]slog.Attr) slog.Handler { return l }
func (l *logSink) WithGroup(string) slog.Handler      { return l }

func (l *logSink) count(sub string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, s := range l.lines {
		if strings.Contains(s, sub) {
			n++
		}
	}
	return n
}

func captureLogs(t *testing.T) *logSink {
	t.Helper()
	old, sink := slog.Default(), &logSink{}
	slog.SetDefault(slog.New(sink))
	t.Cleanup(func() { slog.SetDefault(old) })
	return sink
}

// rig is a Manager on fake sources and a fake world with one viewer joined.
type rig struct {
	t   *testing.T
	w   *world
	src *sources
	env *tracerEnv
	ctx context.Context
}

func newRig(t *testing.T, opts ...tracerOpt) *rig {
	t.Helper()
	w, src := newWorld(), &sources{}
	env := newTracerServer(t, append([]tracerOpt{w.opt(), src.opt()}, opts...)...)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	return &rig{t: t, w: w, src: src, env: env, ctx: ctx}
}

// join connects a viewer and lets the first source emit a keyframe.
func (r *rig) join() *fanViewer {
	r.t.Helper()
	n := r.src.count()
	v := r.env.join(r.ctx, r.t, r.env.token(r.t, "g1"))
	eventually(r.t, 5*time.Second, "capture start", func() bool { return r.src.count() > n && r.env.mgr.Viewers() > 0 })
	r.src.last().emit(true)
	return v
}

func (r *rig) leave(v *fanViewer) {
	r.t.Helper()
	_ = v.conn.CloseNow()
	eventually(r.t, 5*time.Second, "viewer leave", func() bool { return r.env.mgr.Viewers() == 0 })
}

// settle moves the fake clock and gives the 10 ms state loop time to act on it.
func (r *rig) settle(d time.Duration) {
	r.w.clock.Advance(d)
	time.Sleep(120 * time.Millisecond)
}

func (r *rig) nudges() int {
	r.w.mu.Lock()
	defer r.w.mu.Unlock()
	return r.w.nudges
}

func (r *rig) state() desktop.State { return r.env.mgr.Status().State }

// untilNudges waits until the Manager has nudged n times.
func (r *rig) untilNudges(n int) {
	r.t.Helper()
	eventually(r.t, 3*time.Second, "nudge count", func() bool { return r.nudges() == n })
}

// drive runs the stall to the point where the watchdog classifies awaiting
// approval: a nudge, a restart that starts, a second nudge, no frame.
func (r *rig) driveToAwaiting() {
	r.t.Helper()
	before := r.nudges()
	r.settle(6 * time.Second)
	r.untilNudges(before + 1)
	r.settle(1100 * time.Millisecond)
	r.settle(2100 * time.Millisecond)
	r.untilNudges(before + 2)
	r.settle(2100 * time.Millisecond)
}

func TestWatchdog_StillScreenNudges(t *testing.T) {
	r := newRig(t)
	r.join()
	r.w.set(func() { r.w.nudgeFn = func() { r.src.last().emit(false) } })

	r.settle(4 * time.Second)
	if r.nudges() != 0 {
		t.Fatalf("nudged after 4 s of silence, want none before 5 s")
	}
	r.settle(2 * time.Second)
	r.untilNudges(1)
	r.settle(1100 * time.Millisecond)
	if r.src.count() != 1 {
		t.Fatalf("a frame after the nudge is a still screen: no restart, got %d sources", r.src.count())
	}
	r.settle(29 * time.Second)
	if r.nudges() != 1 {
		t.Fatalf("second nudge after only 29 s, want a 30 s wait (nudges %d)", r.nudges())
	}
	r.settle(2 * time.Second)
	r.untilNudges(2)
	if r.state() != desktop.StateStreaming {
		t.Fatalf("a still screen must stay streaming, got %s", r.state())
	}
}

func TestWatchdog_FrozenRestartRecovers(t *testing.T) {
	logs := captureLogs(t)
	r := newRig(t)
	v := r.join()

	r.settle(6 * time.Second)
	r.untilNudges(1)
	r.settle(1100 * time.Millisecond)
	eventually(t, 3*time.Second, "restart after a nudge with no frame", func() bool { return r.src.count() == 2 })
	if !r.src.first().isStopped() {
		t.Fatal("the frozen source was not stopped")
	}

	// A delta first, then a keyframe: the viewer must see the keyframe first.
	for len(v.lc.aus) > 0 {
		<-v.lc.aus
	}
	var first []byte
	eventually(t, 5*time.Second, "first unit after the restart", func() bool {
		r.src.last().emit(false)
		r.src.last().emit(true)
		select {
		case first = <-v.lc.aus:
			return true
		default:
			return false
		}
	})
	requireKeyframe(t, "viewer after restart", first)

	eventually(t, 3*time.Second, "recovered", func() bool { return logs.count("stall recovered") == 1 })
	if r.state() != desktop.StateStreaming {
		t.Fatalf("state %s after recovery, want streaming", r.state())
	}
	r.settle(time.Second)
	if n := logs.count("stall recovered"); n != 1 {
		t.Fatalf("%d recovered log lines, want 1", n)
	}
	if got := r.w.notified(); len(got) != 0 {
		t.Fatalf("a recovered stall must not notify, got %v", got)
	}
}

func TestWatchdog_RestartPermissionDenied(t *testing.T) {
	r := newRig(t)
	r.join()
	r.settle(6 * time.Second)
	r.untilNudges(1)
	r.src.setStartErr(screencap.ErrPermissionDenied)
	r.settle(1100 * time.Millisecond)
	eventually(t, 3*time.Second, "needs_permission", func() bool { return r.state() == desktop.StateNeedsPermission })
	if got := r.w.notified(); len(got) != 0 {
		t.Fatalf("needs_permission must not notify, got %v", got)
	}
}

func TestWatchdog_RestartNoDisplay(t *testing.T) {
	r := newRig(t)
	r.join()
	r.settle(6 * time.Second)
	r.untilNudges(1)
	r.src.setStartErr(screencap.ErrNoDisplay)
	r.settle(1100 * time.Millisecond)
	eventually(t, 3*time.Second, "display_asleep", func() bool { return r.state() == desktop.StateDisplayAsleep })
}

func TestWatchdog_AwaitingApprovalOnce(t *testing.T) {
	r := newRig(t)
	v := r.join()
	r.driveToAwaiting()
	eventually(t, 3*time.Second, "awaiting_approval", func() bool { return r.state() == desktop.StateAwaitingApproval })

	st := r.env.mgr.Status()
	if st.LapseSeq != 1 || st.LapseClaimedSeq != 1 {
		t.Fatalf("LapseSeq %d claimed %d, want 1 and 1", st.LapseSeq, st.LapseClaimedSeq)
	}
	want := notification{"portty.desktop-approval-lapse", "Portty", "Portty desktop needs approval at the Mac."}
	if got := r.w.notified(); len(got) != 1 || got[0] != want {
		t.Fatalf("notifications %v, want exactly %v", got, want)
	}
	if s, _, _ := v.lastState(); s != "awaiting_approval" {
		t.Fatalf("viewer's state %q, want awaiting_approval", s)
	}

	// Three more cycles, 60 s apart: nothing new.
	for i := 0; i < 3; i++ {
		n := r.src.count()
		r.settle(60 * time.Second)
		eventually(t, 3*time.Second, "restart retry", func() bool { return r.src.count() > n })
		r.settle(2100 * time.Millisecond)
		r.settle(2100 * time.Millisecond)
		if st := r.env.mgr.Status(); st.LapseSeq != 1 || st.State != desktop.StateAwaitingApproval {
			t.Fatalf("cycle %d: %+v, want still awaiting with LapseSeq 1", i, st)
		}
	}
	if got := r.w.notified(); len(got) != 1 {
		t.Fatalf("%d notifications after repeated cycles, want 1", len(got))
	}

	// Frames resume: streaming again, still one notification.
	eventually(t, 5*time.Second, "streaming after frames resume", func() bool {
		r.src.last().emit(true)
		return r.state() == desktop.StateStreaming
	})
	r.settle(time.Second)
	if got := r.w.notified(); len(got) != 1 {
		t.Fatalf("recovery posted a notification: %v", got)
	}
}

func TestWatchdog_NoAXNeverAwaiting(t *testing.T) {
	logs := captureLogs(t)
	r := newRig(t)
	r.join()
	r.w.set(func() { r.w.perms.AX = false })

	for i := 0; i < 3; i++ {
		n := r.src.count()
		r.settle(61 * time.Second)
		eventually(t, 3*time.Second, "restart without a nudge", func() bool { return r.src.count() > n })
		r.settle(2100 * time.Millisecond)
		r.settle(2100 * time.Millisecond)
	}
	if r.nudges() != 0 {
		t.Fatalf("nudged without Accessibility %d times", r.nudges())
	}
	if st := r.state(); st != desktop.StateStreaming {
		t.Fatalf("state %s, want streaming (a still screen cannot be told from a frozen one)", st)
	}
	if n := logs.count("cannot be told from a frozen"); n != 1 {
		t.Fatalf("no-Accessibility log lines = %d, want exactly 1", n)
	}
}

func TestWatchdog_NoNudgeWithoutViewer(t *testing.T) {
	r := newRig(t)
	r.settle(10 * time.Minute)
	r.settle(10 * time.Minute)
	if r.nudges() != 0 {
		t.Fatalf("nudged with no viewer connected")
	}
}

func TestLapseSender_GUIClaims(t *testing.T) {
	r := newRig(t)
	r.join()
	r.env.mgr.NoteGUISeen()
	r.w.clock.Advance(5 * time.Second)
	r.env.mgr.EnterAwaitingForTest("watchdog")
	r.w.clock.Advance(3 * time.Second)
	if !r.env.mgr.ClaimLapse(1, "gui") {
		t.Fatal("the GUI's claim of lapse 1 must succeed")
	}
	r.settle(20 * time.Second)
	if got := r.w.notified(); len(got) != 0 {
		t.Fatalf("daemon posted a lapse the GUI claimed: %v", got)
	}
	if st := r.env.mgr.Status(); st.LapseClaimedSeq != 1 {
		t.Fatalf("LapseClaimedSeq %d, want 1", st.LapseClaimedSeq)
	}
}

func TestLapseSender_GUISilentDaemonPosts(t *testing.T) {
	r := newRig(t)
	r.join()
	r.env.mgr.NoteGUISeen()
	r.w.clock.Advance(5 * time.Second)
	r.env.mgr.EnterAwaitingForTest("watchdog")
	r.settle(14 * time.Second)
	if got := r.w.notified(); len(got) != 0 {
		t.Fatalf("daemon posted after 14 s with a GUI running: %v", got)
	}
	r.settle(1100 * time.Millisecond)
	eventually(t, 3*time.Second, "daemon post after the claim wait", func() bool { return len(r.w.notified()) == 1 })
	if r.env.mgr.ClaimLapse(1, "gui") {
		t.Fatal("a late GUI claim must fail once the daemon posted")
	}
}

func TestLapseSender_NoGUIDaemonPostsAtOnce(t *testing.T) {
	r := newRig(t)
	r.join()
	r.env.mgr.NoteGUISeen()
	r.w.clock.Advance(30 * time.Second)
	r.env.mgr.EnterAwaitingForTest("watchdog")
	if got := r.w.notified(); len(got) != 1 {
		t.Fatalf("notifications %v, want the daemon to post at the transition", got)
	}
}

func TestLapseSender_ClaimRace(t *testing.T) {
	r := newRig(t)
	r.join()
	for i := 1; i <= 15; i++ {
		before := len(r.w.notified())
		r.env.mgr.NoteGUISeen()
		r.env.mgr.EnterAwaitingForTest("watchdog")
		r.w.clock.Advance(15*time.Second + time.Millisecond)

		var wg sync.WaitGroup
		start := make(chan struct{})
		var gui bool
		wg.Add(2)
		go func() { defer wg.Done(); <-start; r.env.mgr.TickForTest(r.ctx) }()
		go func() { defer wg.Done(); <-start; gui = r.env.mgr.ClaimLapse(i, "gui") }()
		close(start)
		wg.Wait()
		time.Sleep(30 * time.Millisecond) // a loop tick may also run the daemon side
		posted := len(r.w.notified()) - before
		if gui && posted != 0 || !gui && posted != 1 {
			t.Fatalf("lapse %d: gui claimed %v but the daemon posted %d times; exactly one sender", i, gui, posted)
		}
		if r.env.mgr.ClaimLapse(i, "gui") || r.env.mgr.ClaimLapse(i+1, "gui") || r.env.mgr.ClaimLapse(0, "gui") {
			t.Fatalf("lapse %d: a second, future or zero claim succeeded", i)
		}
		// Frames resume, ending the lapse so the next iteration is a new one.
		eventually(t, 3*time.Second, "recovery", func() bool {
			r.src.last().emit(true)
			return r.state() != desktop.StateAwaitingApproval
		})
	}
}

func TestWatchdog_ConcurrentDetectorsOneTransition(t *testing.T) {
	r := newRig(t)
	r.join()
	var wg sync.WaitGroup
	start := make(chan struct{})
	for _, who := range []string{"watchdog", "probe"} {
		wg.Add(1)
		go func() { defer wg.Done(); <-start; r.env.mgr.EnterAwaitingForTest(who) }()
	}
	close(start)
	wg.Wait()
	if st := r.env.mgr.Status(); st.LapseSeq != 1 || st.State != desktop.StateAwaitingApproval {
		t.Fatalf("%+v, want one transition to awaiting_approval with LapseSeq 1", st)
	}
	if got := r.w.notified(); len(got) != 1 {
		t.Fatalf("%d notifications from two simultaneous detectors, want 1", len(got))
	}
}

func withConfigDir(dir string) tracerOpt {
	return func(_ *tracerEnv, d *desktop.Deps) { d.ConfigDir = dir }
}

func TestApprovalRecord(t *testing.T) {
	dir := t.TempDir()
	r := newRig(t, withConfigDir(dir))
	t0 := r.w.clock.Now()
	day := 24 * time.Hour
	seen := func() int64 {
		rs, err := desktop.ReadRuntimeStateForTest(dir)
		if err != nil {
			t.Fatal(err)
		}
		return rs.ApprovalSeenAt
	}
	approaching := func() bool { return r.env.mgr.Status().ApproachingLapse }

	v := r.join()
	if seen() != t0.Unix() {
		t.Fatalf("no record and a capture start: approvalSeenAt %d, want %d", seen(), t0.Unix())
	}
	r.leave(v)

	r.settle(27 * day)
	eventually(t, 3*time.Second, "approaching at day 27", approaching)
	v = r.join()
	if seen() != t0.Unix() {
		t.Fatalf("a record under 30 days old must stay, got %d", seen())
	}
	r.leave(v)
	r.settle(2 * day)
	eventually(t, 3*time.Second, "approaching at day 29", approaching)

	r.settle(day)
	v = r.join()
	if want := r.w.clock.Now().Unix(); seen() != want {
		t.Fatalf("30 days and a capture start: approvalSeenAt %d, want reset to %d", seen(), want)
	}
	r.settle(time.Second)
	if approaching() {
		t.Fatal("ApproachingLapse must be false right after a reset")
	}
	r.leave(v)

	// Signal (a): a readable store overrides the own record; an error changes nothing.
	r.w.set(func() { r.w.approvalDate, r.w.approvalErr = r.w.clock.Now().Add(-28*day), nil })
	r.settle(6 * time.Minute)
	eventually(t, 3*time.Second, "store date overrides", approaching)
	r.w.set(func() { r.w.approvalErr = errors.New("no Full Disk Access") })
	r.settle(6 * time.Minute)
	eventually(t, 3*time.Second, "store error falls back to the own record", func() bool { return !approaching() })
}

func writeState(t *testing.T, dir string, rs desktop.RuntimeState) {
	t.Helper()
	if err := desktop.WriteRuntimeStateForTest(dir, rs); err != nil {
		t.Fatal(err)
	}
}

func TestBreaker(t *testing.T) {
	t.Run("three exits in an hour pause capture", func(t *testing.T) {
		dir := t.TempDir()
		t0 := newWorld().clock.Now().Unix()
		writeState(t, dir, desktop.RuntimeState{SchemaVersion: 1, CaptureActive: true, AbnormalExits: []int64{t0 - 600, t0 - 300}})
		r := newRig(t, withConfigDir(dir))
		if st := r.state(); st != desktop.StateCapturePaused {
			t.Fatalf("state %s after the third abnormal exit, want capture_paused", st)
		}
		rs, _ := desktop.ReadRuntimeStateForTest(dir)
		if len(rs.AbnormalExits) != 3 || rs.CaptureActive {
			t.Fatalf("persisted %+v, want 3 exits and captureActive false", rs)
		}
		v := r.env.join(r.ctx, t, r.env.token(t, "g1"))
		eventually(t, 3*time.Second, "viewer told capture_paused", func() bool { s, _, _ := v.lastState(); return s == "capture_paused" })
		if r.src.count() != 0 {
			t.Fatal("a paused Manager started a capture source")
		}
		r.leave(v)

		if err := r.env.mgr.ResetBreaker(); err != nil {
			t.Fatal(err)
		}
		if st := r.state(); st == desktop.StateCapturePaused {
			t.Fatal("still capture_paused after ResetBreaker")
		}
		if rs, _ := desktop.ReadRuntimeStateForTest(dir); len(rs.AbnormalExits) != 0 {
			t.Fatalf("exits %v not cleared", rs.AbnormalExits)
		}
		r.join()
		if r.src.count() != 1 {
			t.Fatalf("after a reset the next join must start capture, sources %d", r.src.count())
		}
		if rs, _ := desktop.ReadRuntimeStateForTest(dir); !rs.CaptureActive {
			t.Fatal("captureActive must be true while capturing")
		}
		_ = r.env.mgr.Close()
		if rs, _ := desktop.ReadRuntimeStateForTest(dir); rs.CaptureActive {
			t.Fatal("a clean Close must leave captureActive false")
		}
	})
	t.Run("entries older than an hour are dropped", func(t *testing.T) {
		dir := t.TempDir()
		t0 := newWorld().clock.Now().Unix()
		writeState(t, dir, desktop.RuntimeState{SchemaVersion: 1, CaptureActive: true, AbnormalExits: []int64{t0 - 7200, t0 - 5400, t0 - 300}})
		r := newRig(t, withConfigDir(dir))
		if st := r.state(); st == desktop.StateCapturePaused {
			t.Fatal("two recent exits must not open the breaker")
		}
		rs, _ := desktop.ReadRuntimeStateForTest(dir)
		if len(rs.AbnormalExits) != 2 {
			t.Fatalf("exits %v, want the two within the hour", rs.AbnormalExits)
		}
	})
}

func TestRuntimeState_File(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "desktop-state.json")
	writeState(t, dir, desktop.RuntimeState{SchemaVersion: 1, ApprovalSeenAt: 5})
	fi, err := os.Stat(path)
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("state file mode %v, err %v; want 0600", fi.Mode(), err)
	}
	if ents, _ := os.ReadDir(dir); len(ents) != 1 {
		t.Fatalf("%d files in the config dir, want only the state file (no temp left)", len(ents))
	}
	if rs, err := desktop.ReadRuntimeStateForTest(dir); err != nil || rs.ApprovalSeenAt != 5 {
		t.Fatalf("round trip: %+v, %v", rs, err)
	}

	bad := map[string]func(){
		"symlink": func() {
			_ = os.Remove(path)
			other := filepath.Join(t.TempDir(), "real.json")
			_ = os.WriteFile(other, []byte(`{"schemaVersion":1}`), 0o600)
			if err := os.Symlink(other, path); err != nil {
				t.Fatal(err)
			}
		},
		"loose mode": func() {
			_ = os.Remove(path)
			_ = os.WriteFile(path, []byte(`{"schemaVersion":1}`), 0o644)
		},
		"unknown schema": func() {
			_ = os.Remove(path)
			_ = os.WriteFile(path, []byte(`{"schemaVersion":2}`), 0o600)
		},
		"unknown field": func() {
			_ = os.Remove(path)
			_ = os.WriteFile(path, []byte(`{"schemaVersion":1,"x":1}`), 0o600)
		},
	}
	for name, setup := range bad {
		setup()
		if _, err := desktop.ReadRuntimeStateForTest(dir); err == nil {
			t.Errorf("%s: read succeeded, want an error", name)
		}
		before, _ := os.ReadFile(path)
		m := newBareManager(t, newWorld(), func(d *desktop.Deps) { d.ConfigDir = dir })
		if m.Status().Reason == "" {
			t.Errorf("%s: no Status.Reason for an unreadable state file", name)
		}
		if after, _ := os.ReadFile(path); string(after) != string(before) {
			t.Errorf("%s: an unreadable state file was silently rewritten", name)
		}
	}
	if missing, err := desktop.ReadRuntimeStateForTest(t.TempDir()); err != nil || missing.SchemaVersion != 1 {
		t.Fatalf("a missing file must read as an empty state, got %+v, %v", missing, err)
	}
}

func TestNoteTestCapture(t *testing.T) {
	dir := t.TempDir()
	t0 := newWorld().clock.Now()
	writeState(t, dir, desktop.RuntimeState{SchemaVersion: 1, AbnormalExits: []int64{t0.Unix() - 30, t0.Unix() - 20, t0.Unix() - 10}})
	if err := desktop.NoteTestCapture(dir, t0); err != nil {
		t.Fatal(err)
	}
	rs, _ := desktop.ReadRuntimeStateForTest(dir)
	if len(rs.AbnormalExits) != 0 || rs.ApprovalSeenAt != t0.Unix() {
		t.Fatalf("after a test capture: %+v, want no exits and approvalSeenAt %d", rs, t0.Unix())
	}
	if err := desktop.NoteTestCapture(dir, t0.Add(10*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if rs, _ := desktop.ReadRuntimeStateForTest(dir); rs.ApprovalSeenAt != t0.Unix() {
		t.Fatalf("a second test capture 10 days later moved approvalSeenAt to %d", rs.ApprovalSeenAt)
	}
}
