package webserver

import (
	"context"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/scottkw/portty/internal/relay"
	"github.com/scottkw/portty/internal/testutil"
)

// joinViewer dials the share server and reads through the join sequence
// (grid frame, replay, offset meta), so the viewer is subscribed on return.
func joinViewer(t *testing.T, sid string, every, timeout time.Duration) (*websocket.Conn, *relay.Hub) {
	t.Helper()
	ws, client, _, hub, token := resumeTestServer(t, sid)
	ws.heartbeatEvery, ws.heartbeatTimeout = every, timeout
	conn := dialResume(t, ws, client, sid, token, "")
	if typ, _ := readWSFrame(t, conn); typ != relay.MsgResize {
		t.Fatalf("first frame type 0x%02x, want grid frame", typ)
	}
	readReplay(t, conn)
	return conn, hub
}

// TestWSS_PingGetsPong: the app-level MsgPing gets a MsgPong back, which is how
// a browser (no access to control frames) sees the far end is alive (SHAR-07).
func TestWSS_PingGetsPong(t *testing.T) {
	conn, _ := joinViewer(t, "liveness-pong", 0, 0)
	if err := conn.Write(context.Background(), websocket.MessageBinary, []byte{relay.MsgPing}); err != nil {
		t.Fatalf("write ping: %v", err)
	}
	for {
		typ, _ := readWSFrame(t, conn) // fails the test after 3s without a pong
		if typ == 0x04 {
			return
		}
	}
}

// TestWSS_ReapsSilentViewer: a viewer that stops reading never answers the
// server's WebSocket pings (coder/websocket answers them inside Read), which is
// what a half-open TCP peer looks like. The server drops it from the fan-out.
func TestWSS_ReapsSilentViewer(t *testing.T) {
	_, hub := joinViewer(t, "liveness-reap", 50*time.Millisecond, 100*time.Millisecond)
	if n := hub.RemoteViewerCount(); n != 1 {
		t.Fatalf("viewer count %d after join, want 1", n)
	}
	testutil.WaitFor(t, 2*time.Second, func() bool {
		return hub.RemoteViewerCount() == 0
	}, "silent viewer was never reaped")
}

// TestWSS_KeepsReadingViewer: a viewer that keeps reading answers every ping
// and stays subscribed across many heartbeat intervals.
func TestWSS_KeepsReadingViewer(t *testing.T) {
	conn, hub := joinViewer(t, "liveness-keep", 50*time.Millisecond, 100*time.Millisecond)
	go func() {
		for {
			if _, _, err := conn.Read(context.Background()); err != nil {
				return
			}
		}
	}()
	time.Sleep(time.Second)
	if n := hub.RemoteViewerCount(); n != 1 {
		t.Fatalf("reading viewer count %d after 1s, want 1", n)
	}
}
