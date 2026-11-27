package desktop_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/scottkw/portty/internal/desktop"
)

// joinStalled joins a viewer whose socket nobody reads after the handshake, so
// a close frame sent to it is never answered: the server's close handshake
// blocks (it is bounded at seconds), as it does with a viewer on a slow path.
func (r *rig) joinStalled() *websocket.Conn {
	r.t.Helper()
	n := r.src.count()
	conn, _, err := r.env.dial(r.ctx, r.env.token(r.t, "g1"))
	if err != nil {
		r.t.Fatalf("dial /desktop/ws: %v", err)
	}
	r.t.Cleanup(func() { _ = conn.CloseNow() })
	writeJSON(r.ctx, r.t, conn, map[string]any{"type": "hello", "v": 1, "caps": map[string]int{"video": 1}})
	readType(r.ctx, r.t, conn, "hello")
	lc := newLoopbackClient(r.t)
	writeJSON(r.ctx, r.t, conn, map[string]any{"type": "offer", "sdp": lc.offer(r.t)})
	lc.accept(r.t, readType(r.ctx, r.t, conn, "answer")["sdp"].(string))
	eventually(r.t, 5*time.Second, "capture start", func() bool { return r.src.count() > n && r.env.mgr.Viewers() > 0 })
	return conn
}

func captureActive(t *testing.T, dir string) bool {
	t.Helper()
	rs, err := desktop.ReadRuntimeStateForTest(dir)
	if err != nil {
		t.Fatal(err)
	}
	return rs.CaptureActive
}

// A clean Close with a viewer attached must not be counted as a crash by the
// next process. The viewer's close handshake can take seconds and the process
// can exit before it ends (Quit All, an upgrade handoff's 1 s drain), so
// captureActive has to be false before the viewers are closed, not after.
func TestClose_ClearsCaptureActiveBeforeViewersClose(t *testing.T) {
	dir := t.TempDir()
	r := newRig(t, withConfigDir(dir))
	conn := r.joinStalled()
	eventually(t, 3*time.Second, "captureActive recorded", func() bool { return captureActive(t, dir) })

	closed := make(chan error, 1)
	go func() { closed <- r.env.mgr.Close() }()
	t.Cleanup(func() { _ = conn.CloseNow(); <-closed })

	eventually(t, 2*time.Second, "captureActive cleared while the viewer's close handshake is still pending", func() bool { return !captureActive(t, dir) })
	select {
	case <-closed:
		t.Fatal("Close returned before the stalled viewer's handshake: the test no longer proves the ordering")
	default:
	}

	// The process dies here. A fresh Manager in the same directory must count nothing.
	r2 := newRig(t, withConfigDir(dir))
	rs, err := desktop.ReadRuntimeStateForTest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.AbnormalExits) != 0 || r2.state() == desktop.StateCapturePaused {
		t.Fatalf("a clean Close was counted: abnormalExits %v, state %s", rs.AbnormalExits, r2.state())
	}
}

// WR-01: a capture start can block for about 35 s (a pending macOS dialog
// holds SCK). It must not hold the lifecycle lock: a clean stop clears
// captureActive at once, and the start that finishes after it does not write
// the flag back, so the next process counts no abnormal exit.
func TestClose_ClearsCaptureActiveDuringBlockedStart(t *testing.T) {
	dir := t.TempDir()
	r := newRig(t, withConfigDir(dir))
	r.join()
	if !captureActive(t, dir) {
		t.Fatal("captureActive must be true while capturing")
	}
	block := make(chan struct{})
	entered := r.src.setBlock(block)
	var once sync.Once
	release := func() { once.Do(func() { close(block) }) }
	t.Cleanup(release)

	_ = r.src.last().Stop() // the source ends by itself; the state loop restarts it and the new Start blocks
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the state loop did not restart the ended source")
	}

	closed := make(chan error, 1)
	go func() { closed <- r.env.mgr.Close() }()
	eventually(t, 2*time.Second, "captureActive cleared while a capture start is blocked", func() bool { return !captureActive(t, dir) })
	release()
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not return after the blocked start finished")
	}
	if captureActive(t, dir) {
		t.Fatal("the start that finished after Close wrote captureActive back")
	}
	newRig(t, withConfigDir(dir))
	rs, err := desktop.ReadRuntimeStateForTest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.AbnormalExits) != 0 {
		t.Fatalf("a clean Close during a blocked start was counted: abnormalExits %v", rs.AbnormalExits)
	}
}

// WR-01: while the first viewer's capture start is blocked, a second viewer
// is still answered: the start does not hold the lifecycle lock.
func TestJoin_NotBlockedByAnotherViewersStart(t *testing.T) {
	r := newRig(t)
	block := make(chan struct{})
	entered := r.src.setBlock(block)
	var once sync.Once
	release := func() { once.Do(func() { close(block) }) }
	t.Cleanup(release)

	conn, _, err := r.env.dial(r.ctx, r.env.token(t, "g1"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.CloseNow() })
	writeJSON(r.ctx, t, conn, map[string]any{"type": "hello", "v": 1})
	readType(r.ctx, t, conn, "hello")
	lc := newLoopbackClient(t)
	writeJSON(r.ctx, t, conn, map[string]any{"type": "offer", "sdp": lc.offer(t)})
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the first viewer's capture start never began")
	}

	ctx, cancel := context.WithTimeout(r.ctx, 5*time.Second)
	defer cancel()
	r.env.join(ctx, t, r.env.token(t, "g1")) // fails the test if no answer arrives
	release()
	readType(r.ctx, t, conn, "answer")
	if n := r.env.mgr.Viewers(); n != 2 {
		t.Errorf("viewers = %d, want 2", n)
	}
	if n := r.src.count(); n != 1 {
		t.Errorf("capture starts = %d, want 1 (the second viewer rides on the start in flight)", n)
	}
}

// Guard: a Manager that never got to stop (kill -9, a crash) still counts as
// exactly one abnormal exit.
func TestUncleanEnd_StillCountsOneAbnormalExit(t *testing.T) {
	dir := t.TempDir()
	r := newRig(t, withConfigDir(dir))
	r.join()
	if !captureActive(t, dir) {
		t.Fatal("captureActive must be true while capturing")
	}
	// No Close: the first Manager is simply gone as far as the file goes.
	newRig(t, withConfigDir(dir))
	rs, err := desktop.ReadRuntimeStateForTest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.AbnormalExits) != 1 {
		t.Fatalf("abnormalExits %v, want exactly one for the unclean end", rs.AbnormalExits)
	}
}
