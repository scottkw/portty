package desktop_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

// relockWho is the own-account identity the unlock tests connect as.
var relockWho = desktop.NewIdentity(42, "ken@example.com", "Ken", "home-air", "nHOME1", "100.64.0.9")

// physFake is the physical-input source of a test: the reading is whatever the
// test set last.
type physFake struct {
	mu sync.Mutex
	r  screencap.PhysicalReading
}

func (p *physFake) read() screencap.PhysicalReading {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.r
}

func (p *physFake) set(r screencap.PhysicalReading) {
	p.mu.Lock()
	p.r = r
	p.mu.Unlock()
}

// usedAt is a healthy tap that saw untagged input idle ago.
func usedAt(idle time.Duration) screencap.PhysicalReading {
	return screencap.PhysicalReading{Healthy: true, Seen: true, Idle: idle}
}

// relockRig is an unlockRig with a real audit file and runtime state file under
// one temp directory, a physical-input fake, tap counters and an input bridge
// whose Ctrl+Cmd+Q locks the fake Mac.
type relockRig struct {
	*unlockRig
	dir           string
	phys          *physFake
	keys          *macKeys
	starts, stops atomic.Int32
}

// macKeys is the relock rig's input bridge: the package's fake bridge, where a
// Ctrl+Cmd+Q press locks the fake Mac the way macOS does, unless the test made
// chords get lost (RL-01r: load and a busy app in front) or every post fail.
type macKeys struct {
	*fakeInputBridge
	locked     *atomic.Bool
	lost, fail atomic.Bool
}

func (k *macKeys) PostKey(vk uint16, down bool, flags uint64, repeat bool) error {
	if k.fail.Load() {
		return errors.New("posting refused")
	}
	const both = screencap.FlagControl | screencap.FlagCommand
	if vk == 0x0C && down && flags&both == both && !k.lost.Load() {
		k.locked.Store(true)
	}
	return k.fakeInputBridge.PostKey(vk, down, flags, repeat)
}

func newRelockRig(t *testing.T, mod func(*unlockRig, *desktop.Deps)) *relockRig {
	return newRelockRigIn(t, t.TempDir(), mod)
}

func newRelockRigIn(t *testing.T, dir string, mod func(*unlockRig, *desktop.Deps)) *relockRig {
	t.Helper()
	rr := &relockRig{dir: dir, phys: &physFake{r: screencap.PhysicalReading{Healthy: true}}}
	a, err := desktop.OpenAudit(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	rr.unlockRig = newUnlockRigWith(t, func(r *unlockRig, d *desktop.Deps) {
		rr.keys = &macKeys{fakeInputBridge: r.fi, locked: &r.locked}
		d.Input = rr.keys
		d.ConfigDir = dir
		d.Physical = rr.phys.read
		d.StartTap = func() error { rr.starts.Add(1); return nil }
		d.StopTap = func() { rr.stops.Add(1) }
		d.TapState = func() (bool, bool) { return true, true }
		d.SecureInputOn = func() bool { return false }
		d.Audit = func(event string, id desktop.Identity, reason string) {
			a.Record(event, "", desktop.AuditModeOwn, id, reason)
		}
		if mod != nil {
			mod(r, d)
		}
	})
	return rr
}

// audit returns the audit file's lines, oldest first.
func (rr *relockRig) audit(t *testing.T) []map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(rr.dir, "desktop-audit.jsonl"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if l == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("audit line %q: %v", l, err)
		}
		out = append(out, m)
	}
	return out
}

func (rr *relockRig) auditEvents(t *testing.T) []string {
	t.Helper()
	var ev []string
	for _, m := range rr.audit(t) {
		ev = append(ev, m["event"].(string))
	}
	return ev
}

// armed is the state file's own record: the rule's persisted flag.
func (rr *relockRig) persistedArmed(t *testing.T) bool {
	t.Helper()
	rs, err := desktop.ReadRuntimeStateForTest(rr.dir)
	if err != nil {
		t.Fatal(err)
	}
	return rs.RelockArmed
}

// leave closes the viewer's socket and waits until the Manager has let it go.
func (rr *relockRig) leave(t *testing.T, v *unlockViewer) {
	t.Helper()
	n := rr.leaves.Load()
	_ = v.conn.CloseNow()
	eventually(t, 10*time.Second, "the viewer to leave", func() bool { return rr.leaves.Load() > n })
}

// unlockAs connects an own-account viewer as relockWho and unlocks the fake Mac
// through it.
func (rr *relockRig) unlockAs(ctx context.Context, t *testing.T) *unlockViewer {
	t.Helper()
	v := rr.ownUnlockViewerAs(ctx, t, relockWho)
	v.submit(t, sentinelPassword)
	v.waitUnlockStates(t, "unlocking", "unlocked")
	if !rr.mgr.UnlockStats().RelockArmed {
		t.Fatal("a Portty unlock did not arm the re-lock rule")
	}
	return v
}

// advanceAndPoll moves the fake clock and runs one pass of the rule.
func (rr *relockRig) advanceAndPoll(d time.Duration) {
	rr.clk.Advance(d)
	rr.mgr.RelockPollForTest()
}

// chord returns the key events posted through the input bridge, in order.
func (rr *relockRig) chord() []bridgeCall {
	var out []bridgeCall
	for _, c := range rr.fi.snapshot() {
		if c.Op == "key" {
			out = append(out, c)
		}
	}
	return out
}

// requireChord fails unless exactly one Ctrl+Cmd+Q was posted, in order, with
// both modifiers held across the Q.
func (rr *relockRig) requireChord(t *testing.T) {
	t.Helper()
	const ctrlF, bothF = screencap.FlagControl, screencap.FlagControl | screencap.FlagCommand
	want := []bridgeCall{
		{Op: "key", VK: 0x3B, Down: true, Flags: ctrlF}, {Op: "key", VK: 0x37, Down: true, Flags: bothF},
		{Op: "key", VK: 0x0C, Down: true, Flags: bothF}, {Op: "key", VK: 0x0C, Flags: bothF},
		{Op: "key", VK: 0x37, Flags: ctrlF}, {Op: "key", VK: 0x3B},
	}
	got := rr.chord()
	if len(got) != len(want) {
		t.Fatalf("posted %d key events %+v, want the Lock Screen chord (%d)", len(got), got, len(want))
	}
	for i := range want {
		if got[i].VK != want[i].VK || got[i].Down != want[i].Down || got[i].Flags != want[i].Flags {
			t.Fatalf("key event %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func (rr *relockRig) requireNoChord(t *testing.T, when string) {
	t.Helper()
	if got := rr.chord(); len(got) != 0 {
		t.Fatalf("%s: key events were posted: %+v", when, got)
	}
}

// TestRelock_Tracer: a Portty unlock is audited (own mode, the viewer's
// identity, no password), the only own-account viewer leaves, nothing is posted
// at 29 s, and at 30 s the Lock Screen chord is posted and a relocked line is
// written.
func TestRelock_Tracer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	rr := newRelockRig(t, nil)
	v := rr.unlockAs(ctx, t)

	lines := rr.audit(t)
	if len(lines) != 1 || lines[0]["event"] != "unlocked" || lines[0]["mode"] != "own" {
		t.Fatalf("audit after the unlock = %v, want one unlocked line in mode own", lines)
	}
	if lines[0]["login_name"] != "ken@example.com" || lines[0]["user_id"] != float64(42) || lines[0]["share_id"] != "" {
		t.Errorf("unlocked line %v does not carry the viewer's own-account identity", lines[0])
	}
	if !rr.persistedArmed(t) {
		t.Error("the arm was not written to the state file")
	}

	rr.leave(t, v)
	rr.advanceAndPoll(29 * time.Second)
	rr.requireNoChord(t, "29 s after the last viewer left")
	rr.advanceAndPoll(time.Second)
	rr.requireChord(t)
	rr.advanceAndPoll(time.Second) // the next pass reads the Mac locked

	if got := rr.auditEvents(t); !slices.Equal(got, []string{"unlocked", "relocked"}) {
		t.Fatalf("audit events %v, want unlocked then relocked", got)
	}
	if rl := rr.audit(t)[1]; rl["mode"] != "own" || rl["login_name"] != "ken@example.com" {
		t.Errorf("relocked line %v, want mode own and the last viewer's identity", rl)
	}
	if rr.mgr.UnlockStats().RelockArmed || rr.persistedArmed(t) {
		t.Error("the rule is still armed after it locked the Mac")
	}
	rr.advanceAndPoll(time.Minute)
	if got := len(rr.chord()); got != 6 {
		t.Errorf("%d key events after another minute, want the one chord (6)", got)
	}
}

// TestRelock_ReturnCancels: an own-account viewer back at 20 s cancels the
// timer; leaving again starts a fresh 30 s.
func TestRelock_ReturnCancels(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	rr := newRelockRig(t, nil)
	v := rr.unlockAs(ctx, t)
	rr.leave(t, v)

	rr.advanceAndPoll(20 * time.Second)
	back := rr.ownUnlockViewerAs(ctx, t, relockWho)
	rr.advanceAndPoll(40 * time.Second) // well past 30 s from the first leave
	rr.requireNoChord(t, "with a viewer back")
	if !rr.mgr.UnlockStats().RelockArmed {
		t.Fatal("the rule disarmed itself while a viewer was back")
	}

	rr.leave(t, back)
	rr.advanceAndPoll(29 * time.Second)
	rr.requireNoChord(t, "29 s after the second leave")
	rr.advanceAndPoll(time.Second)
	rr.requireChord(t)
}

// dialToOffer connects an own-account viewer as ident through a bare route and
// returns its socket and its offer message, not yet sent.
func (rr *relockRig) dialToOffer(ctx context.Context, t *testing.T, ident desktop.Identity) (*websocket.Conn, []byte) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		rr.mgr.ServeDesktopViewerAs(r.Context(), c, func() bool { return true }, ident)
	}))
	t.Cleanup(srv.Close)
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.CloseNow() })
	writeJSON(ctx, t, c, map[string]any{"type": "hello", "v": 1, "caps": map[string]int{"video": 1, "input": 1}})
	readType(ctx, t, c, "hello")
	offer, err := json.Marshal(map[string]any{"type": "offer", "sdp": newLoopbackClient(t).offer(t)})
	if err != nil {
		t.Fatal(err)
	}
	return c, offer
}

// refused sends the offer and returns nil once the Manager closed the socket as
// a failed join; by then the Manager has let the viewer go.
func refused(ctx context.Context, c *websocket.Conn, offer []byte) error {
	if err := c.Write(ctx, websocket.MessageText, offer); err != nil {
		return err
	}
	for {
		if _, _, err := c.Read(ctx); err != nil {
			if st := websocket.CloseStatus(err); st != websocket.StatusInternalError {
				return fmt.Errorf("the socket ended with %v (status %d), want a failed join", err, st)
			}
			return nil
		}
	}
}

// errWedged is the start error of a wedged encoder service (U-load).
var errWedged = errors.New("screencap: starting the encoder failed (code -12902)")

// TestRelock_FailedJoinsDoNotRestartTheGrace (U-load): with the encoder wedged,
// own-account joins that fail at once neither restart nor disarm the grace; the
// Mac locks 30 s after the last viewer that was really connected left, and the
// relocked line names that viewer, not a device whose joins failed.
func TestRelock_FailedJoinsDoNotRestartTheGrace(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	src := &sources{}
	rr := newRelockRig(t, func(_ *unlockRig, d *desktop.Deps) { d.NewSource = src.factory() })
	v := rr.unlockAs(ctx, t)
	src.setStartErr(errWedged)
	rr.leave(t, v)

	phone := desktop.NewIdentity(42, "ken@example.com", "Ken", "home-phone", "nPHONE1", "100.64.0.10")
	for _, d := range []time.Duration{10 * time.Second, 10 * time.Second, 9 * time.Second} {
		rr.advanceAndPoll(d)
		c, offer := rr.dialToOffer(ctx, t, phone)
		if err := refused(ctx, c, offer); err != nil {
			t.Fatal(err)
		}
	}
	rr.mgr.RelockPollForTest()
	rr.requireNoChord(t, "29 s after the last viewer left")
	if !rr.mgr.UnlockStats().RelockArmed {
		t.Fatal("failed joins disarmed the re-lock rule")
	}
	rr.advanceAndPoll(time.Second)
	rr.requireChord(t)
	rr.advanceAndPoll(time.Second) // the next pass reads the Mac locked
	if got := rr.auditEvents(t); !slices.Equal(got, []string{"unlocked", "relocked"}) {
		t.Fatalf("audit events %v, want unlocked then relocked", got)
	}
	if node := rr.audit(t)[1]["node_name"]; node != "home-air" {
		t.Errorf("relocked line names node %v, want home-air (the last viewer that was connected)", node)
	}
}

// TestRelock_JoinStillStartingDoesNotHoldOffTheLock: an own-account join whose
// capture start is still in flight when the grace ends is not a viewer yet; the
// Mac locks on time, and the start's later failure changes nothing.
func TestRelock_JoinStillStartingDoesNotHoldOffTheLock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	src := &sources{}
	rr := newRelockRig(t, func(_ *unlockRig, d *desktop.Deps) { d.NewSource = src.factory() })
	v := rr.unlockAs(ctx, t)
	src.setStartErr(errWedged)
	block := make(chan struct{})
	entered := src.setBlock(block)
	rr.leave(t, v)

	rr.advanceAndPoll(25 * time.Second)
	c, offer := rr.dialToOffer(ctx, t, relockWho)
	done := make(chan error, 1)
	go func() { done <- refused(ctx, c, offer) }()
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("the join's capture start did not begin")
	}
	rr.advanceAndPoll(5 * time.Second)
	rr.requireChord(t)

	close(block)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	rr.advanceAndPoll(time.Minute)
	if got := len(rr.chord()); got != 6 {
		t.Errorf("%d key events, want the one chord (6)", got)
	}
	if rr.mgr.UnlockStats().RelockArmed {
		t.Error("the rule is still armed after it locked the Mac")
	}
}

// TestRelock_PhysicalInputDisarms: a healthy tap that saw untagged input after
// the unlock keeps the Mac unlocked; the rule ends and nothing is posted.
func TestRelock_PhysicalInputDisarms(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	rr := newRelockRig(t, nil)
	v := rr.unlockAs(ctx, t)
	rr.leave(t, v)

	rr.phys.set(usedAt(time.Second))
	rr.advanceAndPoll(5 * time.Second)
	rr.advanceAndPoll(25 * time.Second) // the grace is over
	rr.requireNoChord(t, "someone using the Mac")
	if rr.mgr.UnlockStats().RelockArmed || rr.persistedArmed(t) {
		t.Error("the rule is still armed after physical input ended it")
	}
	if got := rr.auditEvents(t); !slices.Equal(got, []string{"unlocked"}) {
		t.Errorf("audit events %v, want only unlocked (no relocked line)", got)
	}
}

// TestRelock_InputBeforeTheUnlockDoesNotCount: untagged input older than the
// unlock is not someone using the Mac now.
func TestRelock_InputBeforeTheUnlockDoesNotCount(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	rr := newRelockRig(t, nil)
	v := rr.unlockAs(ctx, t)
	rr.leave(t, v)

	rr.phys.set(usedAt(time.Hour)) // the last key was an hour ago, long before the unlock
	rr.advanceAndPoll(30 * time.Second)
	rr.requireChord(t)
}

// TestRelock_UnhealthyTapLocks: a tap that is unhealthy at any poll in the
// armed window makes the decision lock, even when a later reading shows input,
// and the daemon says so in one fixed line.
func TestRelock_UnhealthyTapLocks(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	log := captureLog(t)
	rr := newRelockRig(t, nil)
	v := rr.unlockAs(ctx, t)
	rr.leave(t, v)

	rr.phys.set(screencap.PhysicalReading{}) // not healthy: nothing can be said about the person at the Mac
	rr.advanceAndPoll(5 * time.Second)
	rr.phys.set(usedAt(time.Second)) // healthy again, and input seen: the missing signal still decides
	rr.advanceAndPoll(25 * time.Second)
	rr.requireChord(t)
	rr.advanceAndPoll(time.Second) // the next pass reads the Mac locked
	if n := strings.Count(log.String(), "re-locking the Mac without a physical-input signal"); n != 1 {
		t.Errorf("the no-signal line was logged %d times, want once:\n%s", n, log.String())
	}
	if got := rr.auditEvents(t); !slices.Equal(got, []string{"unlocked", "relocked"}) {
		t.Errorf("audit events %v", got)
	}
}

// TestRelock_TapStaysUpWhileArmed: with no controller connected the tap keeps
// running for the rule, and stops once the rule ends.
func TestRelock_TapStaysUpWhileArmed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	rr := newRelockRig(t, nil)
	v := rr.unlockAs(ctx, t)
	eventually(t, 5*time.Second, "the tap to start for the controller", func() bool { return rr.starts.Load() >= 1 })
	rr.leave(t, v)

	rr.mgr.PollTapForTest()
	rr.advanceAndPoll(10 * time.Second)
	rr.mgr.PollTapForTest()
	if rr.stops.Load() != 0 {
		t.Fatalf("the tap was stopped %d times while re-lock was armed and nobody controlled", rr.stops.Load())
	}
	rr.advanceAndPoll(20 * time.Second)
	rr.requireChord(t)
	rr.advanceAndPoll(time.Second) // the next pass reads the Mac locked
	rr.mgr.PollTapForTest()
	if rr.stops.Load() != 1 {
		t.Errorf("the tap was stopped %d times after the rule ended, want 1", rr.stops.Load())
	}
}

// writeArmedState writes the state file a daemon that was killed while the rule
// was armed leaves behind.
func writeArmedState(t *testing.T, dir string) {
	t.Helper()
	err := desktop.WriteRuntimeStateForTest(dir, desktop.RuntimeState{RelockArmed: true, RelockSince: time.Unix(1_699_000_000, 0)})
	if err != nil {
		t.Fatal(err)
	}
}

// TestRelock_RestartResumes: a daemon that starts with the rule armed and the
// Mac unlocked locks it 30 s after the start unless input is seen in that
// window; a Mac that is already locked clears the flag.
func TestRelock_RestartResumes(t *testing.T) {
	unlocked := func(r *unlockRig, _ *desktop.Deps) { r.locked.Store(false) }

	t.Run("locks after 30 s", func(t *testing.T) {
		dir := t.TempDir()
		writeArmedState(t, dir)
		rr := newRelockRigIn(t, dir, unlocked)
		if !rr.mgr.UnlockStats().RelockArmed {
			t.Fatal("the persisted arm was not resumed")
		}
		eventually(t, 5*time.Second, "the tap to start for the rule", func() bool { return rr.starts.Load() >= 1 })
		rr.advanceAndPoll(29 * time.Second)
		rr.requireNoChord(t, "29 s after the start")
		rr.advanceAndPoll(time.Second)
		rr.requireChord(t)
		rr.advanceAndPoll(time.Second) // the next pass reads the Mac locked
		if rr.persistedArmed(t) {
			t.Error("the flag was not cleared after the lock")
		}
		if got := rr.auditEvents(t); !slices.Equal(got, []string{"relocked"}) {
			t.Errorf("audit events %v, want relocked", got)
		}
	})

	t.Run("input in the window disarms", func(t *testing.T) {
		dir := t.TempDir()
		writeArmedState(t, dir)
		rr := newRelockRigIn(t, dir, unlocked)
		rr.phys.set(usedAt(time.Second))
		rr.advanceAndPoll(5 * time.Second)
		rr.advanceAndPoll(25 * time.Second)
		rr.requireNoChord(t, "someone at the Mac after the restart")
		if rr.mgr.UnlockStats().RelockArmed || rr.persistedArmed(t) {
			t.Error("the rule is still armed after input ended it")
		}
	})

	t.Run("a locked Mac clears the flag", func(t *testing.T) {
		dir := t.TempDir()
		writeArmedState(t, dir)
		rr := newRelockRigIn(t, dir, nil) // the rig starts locked
		if rr.mgr.UnlockStats().RelockArmed {
			t.Error("the rule armed on a locked Mac")
		}
		if rr.persistedArmed(t) {
			t.Error("the flag stayed in the state file on a locked Mac")
		}
	})
}

// TestRelock_LockedElsewhereDisarms: a Mac that locks by any other means before
// the timer fires ends the rule: nothing posted, no relocked line.
func TestRelock_LockedElsewhereDisarms(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	rr := newRelockRig(t, nil)
	v := rr.unlockAs(ctx, t)
	rr.leave(t, v)

	rr.advanceAndPoll(10 * time.Second)
	rr.locked.Store(true) // the screen saver, the lid, or someone at the keyboard
	rr.advanceAndPoll(25 * time.Second)
	rr.requireNoChord(t, "a Mac that locked itself")
	if rr.mgr.UnlockStats().RelockArmed || rr.persistedArmed(t) {
		t.Error("the rule is still armed on a locked Mac")
	}
	if got := rr.auditEvents(t); !slices.Equal(got, []string{"unlocked"}) {
		t.Errorf("audit events %v, want only unlocked", got)
	}
}

// TestRelock_NotArmedByAnUnlockAtTheMac: a viewer that joins and leaves an
// already unlocked Mac (it was unlocked at the keyboard) never gets it locked.
func TestRelock_NotArmedByAnUnlockAtTheMac(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	rr := newRelockRig(t, func(r *unlockRig, _ *desktop.Deps) { r.locked.Store(false) })
	v := rr.ownUnlockViewerAs(ctx, t, relockWho)
	if rr.mgr.UnlockStats().RelockArmed {
		t.Fatal("the rule armed without a Portty unlock")
	}
	rr.leave(t, v)
	rr.advanceAndPoll(2 * time.Minute)
	rr.requireNoChord(t, "an unlock nobody did through Portty")
	if got := rr.auditEvents(t); len(got) != 0 {
		t.Errorf("audit events %v, want none", got)
	}
}

// TestRelock_ChordFailureStaysArmed: a Lock Screen chord that could not be
// posted is not a locked Mac; the rule stays armed and tries again.
func TestRelock_ChordFailureStaysArmed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	rr := newRelockRig(t, nil)
	rr.keys.fail.Store(true)
	v := rr.unlockAs(ctx, t)
	rr.leave(t, v)

	rr.advanceAndPoll(30 * time.Second)
	if !rr.mgr.UnlockStats().RelockArmed || !rr.persistedArmed(t) {
		t.Fatal("the rule ended although no chord was posted")
	}
	if got := rr.auditEvents(t); !slices.Equal(got, []string{"unlocked"}) {
		t.Fatalf("audit events %v, want no relocked line yet", got)
	}
	if rr.mgr.InputCounters()["relock_failed"] == 0 {
		t.Error("the failed post was not counted")
	}
	rr.keys.fail.Store(false)
	rr.advanceAndPoll(time.Second)
	rr.requireChord(t)
	rr.advanceAndPoll(time.Second) // the next pass reads the Mac locked
	if got := rr.auditEvents(t); !slices.Equal(got, []string{"unlocked", "relocked"}) {
		t.Errorf("audit events %v after the retry", got)
	}
}

// requireOpen fails unless, at when, the rule has posted wantPosts chords, is
// still armed (in memory and on disk) and has not claimed relocked: the Mac
// reads unlocked.
func (rr *relockRig) requireOpen(t *testing.T, when string, wantPosts int) {
	t.Helper()
	if got := len(rr.chord()); got != 6*wantPosts {
		t.Fatalf("%s: %d key events posted, want %d chords (%d)", when, got, wantPosts, 6*wantPosts)
	}
	if !rr.mgr.UnlockStats().RelockArmed || !rr.persistedArmed(t) {
		t.Fatalf("%s: the rule ended on a Mac that reads unlocked", when)
	}
	if slices.Contains(rr.auditEvents(t), "relocked") || rr.mgr.InputCounters()["relocked"] != 0 {
		t.Fatalf("%s: relocked was claimed on a Mac that reads unlocked", when)
	}
}

// TestRelock_LostChordIsRetriedNotClaimed (RL-01r): a posted chord that did not
// lock the Mac is not a re-lock. Nothing claims relocked while the Mac reads
// unlocked; the chord is posted again 5 s apart, after three unconfirmed posts
// the failure is logged, counted and audited as relock_failed (once), and the
// rule keeps posting every 30 s until the Mac reads locked.
func TestRelock_LostChordIsRetriedNotClaimed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	log := captureLog(t)
	rr := newRelockRig(t, nil)
	v := rr.unlockAs(ctx, t)
	rr.leave(t, v)
	rr.keys.lost.Store(true)

	rr.advanceAndPoll(30 * time.Second)
	rr.requireOpen(t, "the grace's end", 1)
	rr.advanceAndPoll(4 * time.Second)
	rr.requireOpen(t, "4 s after the chord", 1)
	rr.advanceAndPoll(time.Second)
	rr.requireOpen(t, "5 s after the chord", 2)
	rr.advanceAndPoll(5 * time.Second)
	rr.requireOpen(t, "10 s after the chord", 3)
	if got := rr.auditEvents(t); !slices.Equal(got, []string{"unlocked"}) {
		t.Fatalf("audit events %v while chords are still being confirmed, want only unlocked", got)
	}
	rr.advanceAndPoll(5 * time.Second)
	rr.requireOpen(t, "three unconfirmed chords", 4)
	if got := rr.auditEvents(t); !slices.Equal(got, []string{"unlocked", "relock_failed"}) {
		t.Fatalf("audit events %v, want unlocked then relock_failed", got)
	}
	if n := rr.mgr.InputCounters()["relock_failed"]; n != 1 {
		t.Errorf("relock_failed counted %d times, want 1", n)
	}
	if n := strings.Count(log.String(), "the Mac did not lock after the re-lock keys"); n != 1 {
		t.Errorf("the did-not-lock warning was logged %d times, want once:\n%s", n, log.String())
	}

	rr.advanceAndPoll(29 * time.Second)
	rr.requireOpen(t, "29 s into the backoff", 4)
	rr.advanceAndPoll(time.Second)
	rr.requireOpen(t, "30 s into the backoff", 5)

	rr.keys.lost.Store(false) // the load is gone: the next chord locks
	rr.advanceAndPoll(30 * time.Second)
	if got := len(rr.chord()); got != 36 {
		t.Fatalf("%d key events, want 6 chords (36)", got)
	}
	rr.advanceAndPoll(time.Second)
	if got := rr.auditEvents(t); !slices.Equal(got, []string{"unlocked", "relock_failed", "relocked"}) {
		t.Fatalf("audit events %v, want unlocked, relock_failed, relocked", got)
	}
	if rr.mgr.UnlockStats().RelockArmed || rr.persistedArmed(t) {
		t.Error("the rule is still armed on a Mac that reads locked")
	}
	rr.advanceAndPoll(time.Minute)
	if got := len(rr.chord()); got != 36 {
		t.Errorf("%d key events after the lock was confirmed, want no more chords (36)", got)
	}
}

// TestRelock_LockReadErrorIsNotLocked: after the chord, a lock read that fails
// is not a locked Mac, even when it comes with locked=true: no relocked line,
// the rule stays armed and posts again; a clean locked read then ends it.
func TestRelock_LockReadErrorIsNotLocked(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	rr := newRelockRig(t, nil)
	v := rr.unlockAs(ctx, t)
	rr.leave(t, v)

	rr.advanceAndPoll(30 * time.Second)
	rr.requireChord(t)
	rr.fb.set(func() { rr.fb.lockErr = errors.New("session read failed") })
	rr.advanceAndPoll(time.Second)
	rr.requireOpen(t, "a failed lock read", 1)
	rr.advanceAndPoll(4 * time.Second)
	rr.requireOpen(t, "5 s of failed lock reads", 2)

	rr.fb.set(func() { rr.fb.lockErr = nil })
	rr.advanceAndPoll(time.Second)
	if got := rr.auditEvents(t); !slices.Equal(got, []string{"unlocked", "relocked"}) {
		t.Fatalf("audit events %v, want unlocked then relocked", got)
	}
	if rr.mgr.UnlockStats().RelockArmed {
		t.Error("the rule is still armed on a Mac that reads locked")
	}
}

// TestRelock_ViewerBackPausesTheRetries: an own-account viewer back after a
// lost chord stops the re-posting; the rule stays armed and that viewer's
// leaving starts a fresh 30 s grace.
func TestRelock_ViewerBackPausesTheRetries(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	rr := newRelockRig(t, nil)
	v := rr.unlockAs(ctx, t)
	rr.leave(t, v)
	rr.keys.lost.Store(true)

	rr.advanceAndPoll(30 * time.Second)
	rr.requireChord(t)
	back := rr.ownUnlockViewerAs(ctx, t, relockWho)
	rr.advanceAndPoll(time.Minute)
	rr.requireOpen(t, "a viewer back on the unlocked Mac", 1)

	rr.keys.lost.Store(false)
	rr.leave(t, back)
	rr.advanceAndPoll(29 * time.Second)
	rr.requireOpen(t, "29 s after the viewer left again", 1)
	rr.advanceAndPoll(time.Second)
	rr.requireOpen(t, "30 s after the viewer left again", 2)
	rr.advanceAndPoll(time.Second)
	if got := rr.auditEvents(t); !slices.Equal(got, []string{"unlocked", "relocked"}) {
		t.Fatalf("audit events %v, want unlocked then relocked", got)
	}
}

// TestRelock_LockWithAViewerBackIsNotClaimed: after a lost chord, a lock that
// comes while an own-account viewer is back (its own Lock Screen keys) ends the
// rule but is not the rule's re-lock: no relocked line.
func TestRelock_LockWithAViewerBackIsNotClaimed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	rr := newRelockRig(t, nil)
	v := rr.unlockAs(ctx, t)
	rr.leave(t, v)
	rr.keys.lost.Store(true)

	rr.advanceAndPoll(30 * time.Second)
	rr.requireChord(t)
	rr.ownUnlockViewerAs(ctx, t, relockWho)
	rr.locked.Store(true)
	rr.advanceAndPoll(time.Second)
	if got := rr.auditEvents(t); !slices.Equal(got, []string{"unlocked"}) {
		t.Fatalf("audit events %v, want only unlocked (the viewer locked it, not the rule)", got)
	}
	if rr.mgr.UnlockStats().RelockArmed {
		t.Error("the rule is still armed on a Mac that reads locked")
	}
}

// TestRelock_FailedUnlockIsAuditedAndDoesNotArm: an attempt that typed and left
// the Mac locked writes unlock_failed with a fixed reason and arms nothing.
func TestRelock_FailedUnlockIsAuditedAndDoesNotArm(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	rr := newRelockRig(t, nil)
	rr.fb.unlockOnReturn = 0
	v := rr.ownUnlockViewerAs(ctx, t, relockWho)
	v.submit(t, sentinelPassword)
	v.waitUnlockStates(t, "unlocking", "retrying", "failed")

	lines := rr.audit(t)
	if len(lines) != 1 || lines[0]["event"] != "unlock_failed" || lines[0]["reason"] != "still_locked" || lines[0]["mode"] != "own" {
		t.Fatalf("audit = %v, want one unlock_failed line, reason still_locked, mode own", lines)
	}
	if lines[0]["login_name"] != "ken@example.com" {
		t.Errorf("unlock_failed line %v lacks the viewer's identity", lines[0])
	}
	if rr.mgr.UnlockStats().RelockArmed {
		t.Error("a failed unlock armed the re-lock rule")
	}
}
