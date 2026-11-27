package daemon

import (
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/desktop"
)

// lapseNotes records what the daemon's notifier was asked to post.
type lapseNotes struct {
	mu  sync.Mutex
	ids []string
}

func (n *lapseNotes) add(id, _, _ string) {
	n.mu.Lock()
	n.ids = append(n.ids, id)
	n.mu.Unlock()
}

func (n *lapseNotes) count() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.ids)
}

// lapseEnv turns the desktop on with a recording daemon notifier and returns the
// real Manager, ready to be driven into a lapse.
func lapseEnv(t *testing.T) (*desktopEnv, *desktop.Manager, *lapseNotes) {
	t.Helper()
	notes := &lapseNotes{}
	SetDesktopNotifier(notes.add)
	t.Cleanup(func() { SetDesktopNotifier(nil) })
	e := newDesktopEnv(t)
	e.mustOn(t)
	var mgr *desktop.Manager
	eventually(t, 5*time.Second, "the desktop manager", func() bool {
		e.api.mu.RLock()
		defer e.api.mu.RUnlock()
		mgr = e.api.desktopMgr
		return mgr != nil
	})
	return e, mgr, notes
}

// TestDesktopLapse_PresenceAndClaim: only gui=1 marks the GUI present (seen
// through whether the daemon posts at the transition), and the status carries
// the claimed sequence.
func TestDesktopLapse_PresenceAndClaim(t *testing.T) {
	e, mgr, notes := lapseEnv(t)

	// Settings polls without gui=1: no presence, so the daemon is the sender.
	if _, err := e.client.DesktopStatus(); err != nil {
		t.Fatal(err)
	}
	mgr.EnterAwaitingForTest("test")
	if notes.count() != 1 {
		t.Fatalf("a poll without gui=1 must not count as the GUI: %d daemon notifications, want 1", notes.count())
	}
	if st := e.status(t); st.LapseSeq != 1 || st.LapseClaimedSeq != 1 {
		t.Fatalf("after the daemon's own claim: lapseSeq %d claimed %d, want 1 and 1", st.LapseSeq, st.LapseClaimedSeq)
	}
	// The daemon already holds this claim.
	if ok, err := e.client.ClaimDesktopLapse(1); err != nil || ok {
		t.Fatalf("claim of a lapse the daemon posted = %v, %v; want false", ok, err)
	}
}

func TestDesktopLapse_ClaimIsAtomicAndExact(t *testing.T) {
	e, mgr, notes := lapseEnv(t)

	if st, err := e.client.DesktopStatusAsGUI(); err != nil || st.LapseSeq != 0 {
		t.Fatalf("DesktopStatusAsGUI = %+v, %v", st, err)
	}
	mgr.EnterAwaitingForTest("test")
	if notes.count() != 0 {
		t.Fatalf("the daemon posted although the GUI polled with gui=1: %d", notes.count())
	}
	st, err := e.client.DesktopStatusAsGUI()
	if err != nil || st.LapseSeq != 1 || st.LapseClaimedSeq != 0 || st.State != "awaiting_approval" {
		t.Fatalf("status = %+v, %v; want awaiting_approval, lapseSeq 1, claimed 0", st, err)
	}
	for _, stale := range []int{0, 2, -1} {
		if ok, err := e.client.ClaimDesktopLapse(stale); err != nil || ok {
			t.Fatalf("claim of seq %d = %v, %v; want false", stale, ok, err)
		}
	}
	if ok, err := e.client.ClaimDesktopLapse(1); err != nil || !ok {
		t.Fatalf("first claim of the current lapse = %v, %v; want true", ok, err)
	}
	if ok, err := e.client.ClaimDesktopLapse(1); err != nil || ok {
		t.Fatalf("second claim = %v, %v; want false", ok, err)
	}
	if st := e.status(t); st.LapseClaimedSeq != 1 {
		t.Fatalf("lapseClaimedSeq = %d, want 1", st.LapseClaimedSeq)
	}
	// An unknown field is refused outright.
	err = e.client.doJSON(http.MethodPost, "/desktop/lapse-claim", map[string]any{"lapseSeq": 1, "extra": true}, nil)
	if err == nil || !strings.Contains(err.Error(), "status 400") {
		t.Fatalf("unknown field: %v, want status 400", err)
	}
}

// TestDesktopLapse_GUIRunningOneNotification: with the GUI polling (gui=1) and
// claiming, the daemon never posts and exactly one claim succeeds. The daemon's
// own 15 s fallback is pinned with a fake clock in internal/desktop
// (TestLapseSender_GUIClaims), because this Manager runs on the real clock.
func TestDesktopLapse_GUIRunningOneNotification(t *testing.T) {
	e, mgr, notes := lapseEnv(t)
	if _, err := e.client.DesktopStatusAsGUI(); err != nil {
		t.Fatal(err)
	}
	mgr.EnterAwaitingForTest("test")
	won := 0
	for i := 0; i < 3; i++ { // three tray polls see the same lapse
		ok, err := e.client.ClaimDesktopLapse(1)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			won++
		}
	}
	if won != 1 || notes.count() != 0 {
		t.Fatalf("claims won %d (want 1), daemon notifications %d (want 0)", won, notes.count())
	}
}

// TestDesktopLapse_GUIQuitDaemonPosts: with no GUI poll the daemon posts once at
// the transition, and a repeat of the transition adds nothing.
func TestDesktopLapse_GUIQuitDaemonPosts(t *testing.T) {
	e, mgr, notes := lapseEnv(t)
	mgr.EnterAwaitingForTest("test")
	mgr.EnterAwaitingForTest("test")
	if notes.count() != 1 {
		t.Fatalf("daemon notifications = %d, want 1", notes.count())
	}
	notes.mu.Lock()
	id := notes.ids[0]
	notes.mu.Unlock()
	if id != "portty.desktop-approval-lapse" {
		t.Errorf("identifier = %q", id)
	}
	if ok, err := e.client.ClaimDesktopLapse(1); err != nil || ok {
		t.Fatalf("a GUI that arrives late must lose the claim: %v, %v", ok, err)
	}
}

// With no Manager (always-available off) a claim is simply not granted.
func TestDesktopLapse_NoManagerClaimsNothing(t *testing.T) {
	e := newDesktopEnv(t)
	if ok, err := e.client.ClaimDesktopLapse(1); err != nil || ok {
		t.Fatalf("claim with no Manager = %v, %v; want false", ok, err)
	}
	if _, err := e.client.DesktopStatusAsGUI(); err != nil {
		t.Fatalf("gui=1 with no Manager: %v", err)
	}
}
