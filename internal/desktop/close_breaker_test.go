package desktop_test

import (
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
