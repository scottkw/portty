package daemon

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var errInjected = errors.New("injected")

// postServe reports whether a death at step lets the takeover read the PTYs
// first (it served before dying), which the old daemon's ring then misses.
func postServe(step string) bool {
	return step == "new:after-serve-before-committed" || step == "new:committed-write-fails"
}

// checkOldServes asserts the old daemon serves again: every session listed
// as running, a write accepted, the ticker advancing with no tick missing
// (apart from what the takeover n read before it died, when gap), and the
// connected viewer still receiving.
func checkOldServes(t *testing.T, s *handoffScene, n *API, v *tickViewer, gap bool) {
	t.Helper()
	waitFor(t, "old daemon thawed", 5*time.Second, func() bool { return !s.old.frozen.Load() })
	c := NewDaemonClient(s.socket)
	for _, id := range s.ids {
		if got := listByID(t, c)[id]; got.State != "running" {
			t.Errorf("session %s after the outcome: %+v", id, got)
		}
	}
	s.createSession(c, s.old)
	for _, id := range s.ids {
		last := lastTick(ringTicks(s.old, id))
		waitFor(t, "old ticks advance", 5*time.Second, func() bool { return lastTick(ringTicks(s.old, id)) >= last+3 })
		ns := ringTicks(s.old, id)
		if gap {
			ns = union(ns, ringTicks(n, id))
		}
		checkContiguous(t, "old ring "+id, ns)
	}
	if v != nil {
		last := lastTick(v.ticks())
		waitFor(t, "viewer keeps receiving", 5*time.Second, func() bool { return lastTick(v.ticks()) >= last+3 })
		ns := v.ticks()
		if gap {
			ns = union(ns, ringTicks(n, s.ids[0]))
		}
		checkContiguous(t, "viewer on the old daemon", ns)
	}
}

// T-D2: a fault at any step before COMMITTED, on either side, leaves the old
// daemon serving with contiguous output and every received fd closed.
func TestHandoffFault_EveryStep(t *testing.T) {
	for _, step := range []string{
		"old:after-hello", "old:after-freeze", "old:after-ready-before-commit",
		"new:after-hello", "new:after-global", "new:after-state", "new:before-ready",
		"new:after-commit-before-serve", "new:after-serve-before-committed", "new:committed-write-fails",
	} {
		t.Run(step, func(t *testing.T) {
			s := newHandoffScene(t, tickScript(false))
			v := s.dialViewer(s.old.relayPort, s.ids[0], "")
			waitTicks(t, "viewer ticks", v.ticks, 3)
			out0, off0 := v.state()
			base := handoffFDCount(t, s.lockPath)
			s.failAt(step, errInjected)

			n, code := s.takeover(BuildVersion)
			if code == takeoverCommitted {
				t.Fatalf("runTakeover committed despite a fault at %s", step)
			}
			if strings.HasPrefix(step, "new:after-commit") || postServe(step) {
				s.waitExit(s.newExits, "takeover", 1)
			}
			s.setFault(nil)
			waitFor(t, "old daemon thawed", 5*time.Second, func() bool { return !s.old.frozen.Load() })
			if got := handoffFDCount(t, s.lockPath); got != base {
				t.Errorf("descriptor count %d after the outcome, %d before", got, base)
			}
			checkOldServes(t, s, n, v, postServe(step))
			if !postServe(step) {
				for _, id := range s.ids {
					if h, ok := n.engine.Manager().Get(id); ok && len(h.ScrollbackSnapshot()) > 0 {
						if extra := lastTick(ringTicks(n, id)); extra > lastTick(tickNums(out0))+200 {
							t.Errorf("the takeover's staged hub read on (tick %d)", extra)
						}
					}
				}
			}
			v2 := s.dialViewer(s.old.relayPort, s.ids[0], strconv.FormatUint(off0, 10))
			waitTicks(t, "resumed ticks", v2.ticks, 3)
			resumed, _ := v2.state()
			ns := tickNums(append(out0, resumed...))
			if postServe(step) {
				ns = union(ns, ringTicks(n, s.ids[0]))
			}
			checkContiguous(t, "resume from before the attempt", ns)
		})
	}
}

// B1: after COMMIT the old daemon thaws only once the takeover process is
// gone, whatever the connection did.
func TestHandoffFault_NoThawUntilTakeoverExits(t *testing.T) {
	for _, step := range []string{"new:committed-write-fails", "new:after-serve-before-committed"} {
		t.Run(step, func(t *testing.T) {
			s := newHandoffScene(t, tickScript(false))
			s.holdPeer = true
			s.failAt(step, errInjected)
			n, code := s.takeover(BuildVersion)
			if code != takeoverFailed {
				t.Fatalf("runTakeover = %d, want %d", code, takeoverFailed)
			}
			s.waitExit(s.newExits, "takeover", 1)
			hub, _ := s.old.engine.Manager().Get(s.ids[0])
			frozenLen := len(hub.ScrollbackSnapshot())
			for end := time.Now().Add(500 * time.Millisecond); time.Now().Before(end); time.Sleep(50 * time.Millisecond) {
				if got := len(hub.ScrollbackSnapshot()); got != frozenLen || !s.old.frozen.Load() {
					t.Fatalf("old daemon read on (ring %d -> %d) before the takeover was gone", frozenLen, got)
				}
			}
			if code, _ := rawPost(t, s.socket, "/sessions", `{}`); code != http.StatusServiceUnavailable {
				t.Errorf("POST /sessions while waiting = %d, want 503", code)
			}
			s.closePeer()
			s.setFault(nil)
			checkOldServes(t, s, n, nil, true)
		})
	}
}

// A failing COMMITTED write with the old daemon alive ends the takeover at
// once; with the old daemon gone the takeover keeps the sessions.
func TestHandoffFault_CommittedWriteFailsOldAlive(t *testing.T) {
	t.Run("old alive", func(t *testing.T) {
		s := newHandoffScene(t, tickScript(false))
		s.holdPeer = true
		s.failAt("new:committed-write-fails", errInjected)
		if _, code := s.takeover(BuildVersion); code != takeoverFailed {
			t.Fatalf("runTakeover = %d, want %d", code, takeoverFailed)
		}
		s.waitExit(s.newExits, "takeover", 1)
		// oldGoneWait: a dying old daemon gets that long to be gone (see the SUMMARY deviation).
		if d := s.at("new:exit").Sub(s.at("new:committed-write-fails")); d > oldGoneWait+150*time.Millisecond {
			t.Errorf("takeover exited %v after the failed write, want within %v", d, oldGoneWait+150*time.Millisecond)
		}
		s.closePeer()
	})
	t.Run("old gone", func(t *testing.T) {
		s := newHandoffScene(t, tickScript(false))
		s.oldAlive.Store(false)
		s.setFault(func(side, step string) error {
			switch side + ":" + step {
			case "old:after-commit-before-committed":
				return errHandoffDie
			case "new:committed-write-fails":
				return errInjected
			}
			return nil
		})
		n, code := s.takeover(BuildVersion)
		if code != takeoverCommitted {
			t.Fatalf("runTakeover = %d, want 0", code)
		}
		checkNewServes(t, s, n, s.ids)
	})
}

// checkNewServes asserts the takeover serves ids: listed on the socket,
// output advancing in its hubs, a write accepted, and no exit recorded.
func checkNewServes(t *testing.T, s *handoffScene, n *API, ids []string) {
	t.Helper()
	select {
	case code := <-s.newExits:
		t.Fatalf("takeover exited with %d", code)
	default:
	}
	c := NewDaemonClient(s.socket)
	got := listByID(t, c)
	for _, id := range ids {
		if got[id].State != "running" {
			t.Errorf("session %s on the takeover: %+v", id, got[id])
		}
		last := lastTick(ringTicks(n, id))
		waitFor(t, "takeover ticks advance", 5*time.Second, func() bool { return lastTick(ringTicks(n, id)) >= last+3 })
		checkContiguous(t, "takeover ring "+id, ringTicks(n, id))
	}
	s.createSession(c, n)
}

// T-D3: the old daemon dies after every fd went out: the takeover proceeds as
// committed and serves every session.
func TestHandoffFault_OldDiesAfterStateSent(t *testing.T) {
	s := newHandoffScene(t, tickScript(false), tickScript(true))
	s.oldAlive.Store(false)
	s.failAt("old:after-state-sent", errHandoffDie)
	n, code := s.takeover(BuildVersion)
	if code != takeoverCommitted {
		t.Fatalf("runTakeover = %d, want 0", code)
	}
	s.waitExit(s.oldExits, "old daemon", 1)
	checkNewServes(t, s, n, s.ids)
	for i, id := range s.ids {
		if sess, _ := n.engine.registry.Get(id); sess == nil || sess.Pid() != s.pids[i] {
			t.Errorf("session %s not adopted with pid %d", id, s.pids[i])
		}
	}
}

// The old daemon dies with one of two sessions sent: gone, the takeover saves
// the one it got; alive (it rolls back instead), it keeps both.
func TestHandoffFault_OldDiesMidState(t *testing.T) {
	t.Run("old gone", func(t *testing.T) {
		logs := captureDiag(t)
		s := newHandoffScene(t, tickScript(false), tickScript(false))
		s.oldAlive.Store(false)
		s.failAt("old:mid-state-send", errHandoffDie)
		n, code := s.takeover(BuildVersion)
		if code != takeoverCommitted {
			t.Fatalf("runTakeover = %d, want 0", code)
		}
		checkNewServes(t, s, n, s.ids[:1])
		if list := n.engine.ListSessions(); len(list) != 2 || list[0].ID == s.ids[1] || list[1].ID == s.ids[1] { // the one it got + the one created
			t.Errorf("takeover sessions = %+v, want %s and the new one", list, s.ids[0])
		}
		if !logs.has("1 of 2 sessions were lost") {
			t.Errorf("log lacks the lost session:\n%s", logs.bytes())
		}
	})
	t.Run("old alive", func(t *testing.T) {
		s := newHandoffScene(t, tickScript(false), tickScript(false))
		s.failAt("old:mid-state-send", errInjected)
		n, code := s.takeover(BuildVersion)
		if code != takeoverTimedOut {
			t.Fatalf("runTakeover = %d, want %d", code, takeoverTimedOut)
		}
		checkOldServes(t, s, n, nil, false)
	})
}

// W2: the old daemon dies after sending COMMIT: the takeover's COMMITTED has
// nobody to read it, the old pid is gone, and it keeps every session.
func TestHandoffFault_OldDiesAfterCommit(t *testing.T) {
	s := newHandoffScene(t, tickScript(false), tickScript(false))
	s.oldAlive.Store(false)
	s.failAt("old:after-commit-before-committed", errHandoffDie)
	n, code := s.takeover(BuildVersion)
	if code != takeoverCommitted {
		t.Fatalf("runTakeover = %d, want 0", code)
	}
	s.waitExit(s.oldExits, "old daemon", 1)
	checkNewServes(t, s, n, s.ids)
}

// A takeover that hangs after COMMIT is ended by its watchdog; the old
// daemon thaws only once it is gone.
func TestHandoffFault_NewHangsAfterCommit(t *testing.T) {
	prev := committedWatchdog
	committedWatchdog = 300 * time.Millisecond
	t.Cleanup(func() { committedWatchdog = prev })
	s := newHandoffScene(t, tickScript(false))
	s.holdPeer = true
	release := make(chan struct{})
	s.setFault(func(side, step string) error {
		if side+":"+step == "new:after-commit-before-serve" {
			<-release
		}
		return nil
	})
	codes := make(chan int, 1)
	var n atomic.Pointer[API]
	go func() {
		a, code := s.takeover(BuildVersion)
		n.Store(a)
		codes <- code
	}()
	s.waitExit(s.newExits, "takeover (watchdog)", 1)
	hub, _ := s.old.engine.Manager().Get(s.ids[0])
	frozenLen := len(hub.ScrollbackSnapshot())
	time.Sleep(200 * time.Millisecond)
	if got := len(hub.ScrollbackSnapshot()); got != frozenLen || !s.old.frozen.Load() {
		t.Fatalf("old daemon read on (ring %d -> %d) while the takeover was not known gone", frozenLen, got)
	}
	s.closePeer()
	waitFor(t, "old daemon thawed", 5*time.Second, func() bool { return !s.old.frozen.Load() })
	close(release)
	if code := <-codes; code != takeoverFailed {
		t.Errorf("runTakeover = %d, want %d", code, takeoverFailed)
	}
	s.setFault(nil)
	checkOldServes(t, s, n.Load(), nil, false)
}

// Threat I1: READY times out, the old daemon writes ABORT before it thaws,
// and the takeover never proceeds after ABORT, even with the old pid gone.
func TestHandoffFault_AbortBeforeThaw(t *testing.T) {
	prev := readyTimeout
	readyTimeout = 300 * time.Millisecond
	t.Cleanup(func() { readyTimeout = prev })
	s := newHandoffScene(t, tickScript(false))
	var atFreeze, atAbort atomic.Int64
	abortSeen := make(chan struct{})
	s.setFault(func(side, step string) error {
		hub, _ := s.old.engine.Manager().Get(s.ids[0])
		switch side + ":" + step {
		case "old:after-freeze":
			atFreeze.Store(int64(len(hub.ScrollbackSnapshot())))
		case "old:abort-written":
			time.Sleep(100 * time.Millisecond) // a Thaw before this point would have read the waiting ticks
			atAbort.Store(int64(len(hub.ScrollbackSnapshot())))
			close(abortSeen)
		case "new:before-ready":
			for end := time.Now().Add(3 * time.Second); s.old.frozen.Load() && time.Now().Before(end); {
				time.Sleep(10 * time.Millisecond)
			}
			s.oldAlive.Store(false)
		}
		return nil
	})
	n, code := s.takeover(BuildVersion)
	if code != takeoverTimedOut {
		t.Fatalf("runTakeover = %d, want %d (events %v)", code, takeoverTimedOut, s.events)
	}
	if s.idx("new:after-commit-before-serve") >= 0 {
		t.Errorf("the takeover proceeded after ABORT")
	}
	select {
	case <-abortSeen:
	case <-time.After(5 * time.Second):
		t.Fatal("the old daemon never wrote ABORT")
	}
	if atAbort.Load() != atFreeze.Load() {
		t.Errorf("old ring grew from %d to %d before ABORT was written", atFreeze.Load(), atAbort.Load())
	}
	s.setFault(nil)
	checkOldServes(t, s, n, nil, false)
}

// T-D4: two takeovers at once: one commits, the other is told busy.
func TestHandoff_OneAtATime(t *testing.T) {
	s := newHandoffScene(t, tickScript(false))
	s.holdPeer = true
	held, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	s.setFault(func(side, step string) error {
		if side+":"+step == "old:after-freeze" {
			once.Do(func() { close(held); <-release })
		}
		return nil
	})
	first := make(chan int, 1)
	go func() { _, code := s.takeover(BuildVersion); first <- code }()
	<-held
	if _, code := s.takeover(BuildVersion); code != takeoverBusy {
		t.Errorf("second takeover = %d, want %d", code, takeoverBusy)
	}
	close(release)
	if code := <-first; code != takeoverCommitted {
		t.Errorf("first takeover = %d, want 0", code)
	}
}

// T-D5: while frozen, writes answer 503 Retry-After: 1 and reads serve; a
// client write started during the freeze lands after the rollback.
func TestHandoff_FreezeGate(t *testing.T) {
	s := newHandoffScene(t, tickScript(false))
	held, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	s.setFault(func(side, step string) error {
		if side+":"+step == "old:after-freeze" {
			once.Do(func() { close(held); <-release })
			return errInjected
		}
		return nil
	})
	codes := make(chan int, 1)
	go func() { _, code := s.takeover(BuildVersion); codes <- code }()
	<-held

	hc := &http.Client{Transport: &http.Transport{DialContext: dialUnix(s.socket)}}
	resp, err := hc.Post("http://daemon/sessions", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable || resp.Header.Get("Retry-After") != "1" {
		t.Errorf("POST /sessions while frozen = %d Retry-After %q, want 503 1", resp.StatusCode, resp.Header.Get("Retry-After"))
	}
	// /shutdown through the same gate, in front of a stub: the real one exits the process.
	called := false
	rec := httptest.NewRecorder()
	s.old.freezeGate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })).
		ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/shutdown", nil))
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") != "1" || called {
		t.Errorf("POST /shutdown while frozen = %d Retry-After %q (reached handler %v), want 503 1", rec.Code, rec.Header().Get("Retry-After"), called)
	}
	if code, _ := rawGet(t, s.socket, "/sessions"); code != http.StatusOK {
		t.Errorf("GET /sessions while frozen = %d, want 200", code)
	}

	created := make(chan error, 1)
	go func() {
		_, err := NewDaemonClient(s.socket).CreateSession("during", "", 80, 24)
		created <- err
	}()
	time.Sleep(300 * time.Millisecond)
	select {
	case err := <-created:
		t.Fatalf("CreateSession returned during the freeze: %v", err)
	default:
	}
	close(release)
	if code := <-codes; code != takeoverTimedOut {
		t.Errorf("runTakeover = %d, want %d", code, takeoverTimedOut)
	}
	if err := <-created; err != nil {
		t.Errorf("CreateSession started during the freeze: %v", err)
	}
	for _, sess := range s.old.engine.registry.List() {
		s.mu.Lock()
		s.extraPids = append(s.extraPids, sess.Pid())
		s.mu.Unlock()
	}
}

// T-D6: a SIGTERM to the old daemon after COMMIT never unlinks the socket the
// takeover serves; before COMMIT the handoff rolls back, then today's
// shutdown runs.
func TestHandoff_SIGTERMAfterCommitKeepsSocket(t *testing.T) {
	t.Run("after COMMIT", func(t *testing.T) {
		s := newHandoffScene(t, tickScript(false))
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		served := make(chan struct{})
		go func() { serveDaemon(ctx, s.old, s.old.engine, s.socket); close(served) }()
		s.setFault(func(side, step string) error {
			if side+":"+step == "old:after-commit-before-committed" {
				cancel()
				time.Sleep(50 * time.Millisecond)
			}
			return nil
		})
		before, _ := os.Stat(s.socket)
		n, code := s.takeover(BuildVersion)
		if code != takeoverCommitted {
			t.Fatalf("runTakeover = %d", code)
		}
		s.waitExit(s.oldExits, "old daemon", 0)
		select {
		case <-served:
		case <-time.After(5 * time.Second):
			t.Fatal("serveDaemon did not return")
		}
		after, err := os.Stat(s.socket)
		if err != nil || !os.SameFile(before, after) {
			t.Fatalf("socket after SIGTERM: %v", err)
		}
		lctx, lcancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		defer lcancel()
		if socketLost(lctx, s.socket, before, 50*time.Millisecond) {
			t.Errorf("the takeover's socketLost fired")
		}
		hub, _ := s.old.engine.Manager().Get(s.ids[0])
		select {
		case <-hub.Done():
			t.Errorf("the old daemon's shutdown ended the session's hub")
		default:
		}
		checkNewServes(t, s, n, s.ids)
	})
	t.Run("before COMMIT", func(t *testing.T) {
		s := newHandoffScene(t, tickScript(false))
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		served := make(chan struct{})
		go func() { serveDaemon(ctx, s.old, s.old.engine, s.socket); close(served) }()
		var early atomic.Bool
		s.setFault(func(side, step string) error {
			if side+":"+step == "old:after-freeze" {
				cancel()
				select {
				case <-served:
					early.Store(true)
				case <-time.After(200 * time.Millisecond):
				}
				return errInjected
			}
			return nil
		})
		if _, code := s.takeover(BuildVersion); code != takeoverTimedOut {
			t.Fatalf("runTakeover = %d, want %d", code, takeoverTimedOut)
		}
		select {
		case <-served:
		case <-time.After(5 * time.Second):
			t.Fatal("serveDaemon did not return")
		}
		if early.Load() {
			t.Errorf("the shutdown ran before the handoff rolled back")
		}
		if _, err := os.Stat(s.socket); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("socket after the normal shutdown: %v, want removed", err)
		}
	})
}

// T-D7: the daemon lock is held at every step of a handoff and of a failed
// one, and after a commit the takeover holds it.
func TestHandoff_LockHeldThroughout(t *testing.T) {
	s := newHandoffScene(t, tickScript(false))
	if _, code := s.takeover(BuildVersion); code != takeoverCommitted {
		t.Fatalf("runTakeover = %d", code)
	}
	s.waitExit(s.oldExits, "old daemon", 0) // the old daemon closed its lock copy
	if s.lockFree.Load() || !lockHeld(s.lockPath) {
		t.Errorf("lock free during or after the handoff (events %v)", s.events)
	}
	f := newHandoffScene(t, tickScript(false))
	f.failAt("new:after-state", errInjected)
	if _, code := f.takeover(BuildVersion); code == takeoverCommitted {
		t.Fatal("committed despite the fault")
	}
	if f.lockFree.Load() || !lockHeld(f.lockPath) {
		t.Errorf("lock free during or after the failed handoff (events %v)", f.events)
	}
}

// W3: the longest freeze fits 09-01's client retry budget.
func TestHandoff_TimeoutsFitClientRetry(t *testing.T) {
	if sum := freezeTimeout + stateSendTimeout + readyTimeout + committedWatchdog; sum >= frozenRetryBudget {
		t.Errorf("freeze %v + state %v + ready %v + watchdog %v = %v, not below frozenRetryBudget %v",
			freezeTimeout, stateSendTimeout, readyTimeout, committedWatchdog, sum, frozenRetryBudget)
	}
	if maxFrozen() >= frozenRetryBudget {
		t.Errorf("maxFrozen %v >= frozenRetryBudget %v", maxFrozen(), frozenRetryBudget)
	}
}

// T-09-21: no fault hook outside the vmpreview build.
func TestHandoffFault_SeamNilInRelease(t *testing.T) {
	if handoffFault != nil {
		t.Errorf("handoffFault is set in this build")
	}
	if fmt.Sprint(moveFailedReason(takeoverTimedOut)) != "the handoff timed out" {
		t.Errorf("reason table changed")
	}
}
