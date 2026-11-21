package relay

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/scottkw/portty/internal/testutil"
)

// TestRelay_PasteBacklogKeepsPonging (INPT-01, INPT-02): a local tab pastes
// 8 MiB into a PTY that is not reading, then pings. The local relay pump queues
// the paste and keeps reading, so the pong arrives while the PTY is still
// blocked (before, the pump sat in the PTY write and the local watchdog gave
// up). Once the PTY drains, every byte arrives in order.
func TestRelay_PasteBacklogKeepsPonging(t *testing.T) {
	srv, manager, _, input, sid := setupTestServer(t)
	hub, _ := manager.Get(sid)
	conn := dialWS(t, srv.URL, sid)
	readJoinReplay(t, conn)

	const frames, size = 256, MaxInputPayload
	payload := make([]byte, frames*size)
	for i := range payload {
		payload[i] = byte('!' + (i/size*7+i)%90) // printable: nothing the absorber strips
	}

	pinged := make(chan time.Time, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for off := 0; off < len(payload); off += size {
			if err := conn.Write(ctx, websocket.MessageBinary, append([]byte{MsgInput}, payload[off:off+size]...)); err != nil {
				return
			}
		}
		if err := conn.Write(ctx, websocket.MessageBinary, []byte{MsgPing}); err == nil {
			pinged <- time.Now()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for {
		_, msg, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("no pong while the paste is queued: %v", err)
		}
		if len(msg) > 0 && msg[0] == MsgPong {
			break
		}
	}
	select {
	case at := <-pinged:
		if d := time.Since(at); d > time.Second {
			t.Fatalf("pong took %s, want under 1s", d)
		}
	default:
		t.Fatal("pong before the ping was sent")
	}
	if idle, _ := hub.WaitInputIdle(0); idle {
		t.Fatal("input idle at the pong, want the paste still queued behind the blocked PTY")
	}

	got := make([]byte, len(payload))
	if _, err := io.ReadFull(input, got); err != nil {
		t.Fatalf("read PTY: %v", err)
	}
	if sha256.Sum256(got) != sha256.Sum256(payload) {
		t.Fatal("PTY bytes differ from the paste (reordered or lost)")
	}
}

// TestRelay_InputClosedDefersToSessionEnd (WR-04): a pump blocked at its input
// quota when the session ends must not race the write pump's "session ended"
// close with an abnormal one; the viewer always gets 1000 session ended.
func TestRelay_InputClosedDefersToSessionEnd(t *testing.T) {
	srv, manager, _, _, sid := setupTestServer(t)
	hub, _ := manager.Get(sid)
	conn := dialWS(t, srv.URL, sid)
	readJoinReplay(t, conn)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for i := 0; i < InputMaxFrames+50; i++ { // the PTY never reads: the pump ends up blocked in Write
		if err := conn.Write(ctx, websocket.MessageBinary, []byte{MsgInput, 'k'}); err != nil {
			t.Fatal(err)
		}
	}
	testutil.WaitFor(t, 5*time.Second, func() bool {
		_, queued := hub.WaitInputIdle(0)
		return queued >= InputMaxFrames
	}, "the input queue never filled")

	hub.Shutdown()
	for {
		_, _, err := conn.Read(ctx)
		if err == nil {
			continue
		}
		var ce websocket.CloseError
		if !errors.As(err, &ce) || ce.Code != websocket.StatusNormalClosure || ce.Reason != CloseReasonSessionEnded {
			t.Fatalf("close = %v, want 1000 %q", err, CloseReasonSessionEnded)
		}
		return
	}
}

// TestRelay_DeadInputKeepsViewer (WR-04): a PTY write error on a live hub kills
// the input, but the viewer stays connected (output and pongs go on) instead of
// being dropped on each keystroke into a reconnect loop.
func TestRelay_DeadInputKeepsViewer(t *testing.T) {
	srv, manager, _, input, sid := setupTestServer(t)
	hub, _ := manager.Get(sid)
	conn := dialWS(t, srv.URL, sid)
	readJoinReplay(t, conn)

	_ = input.Close() // the PTY write now fails
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageBinary, []byte{MsgInput, 'a'}); err != nil {
		t.Fatal(err)
	}
	testutil.WaitFor(t, 5*time.Second, func() bool {
		hub.inMu.Lock()
		defer hub.inMu.Unlock()
		return hub.inDead
	}, "the input never died")
	for _, f := range [][]byte{{MsgInput, 'b'}, {MsgInput, 'c'}, {MsgPing}} {
		if err := conn.Write(ctx, websocket.MessageBinary, f); err != nil {
			t.Fatal(err)
		}
	}
	for {
		_, msg, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("viewer dropped after the input died: %v", err)
		}
		if len(msg) > 0 && msg[0] == MsgPong {
			return
		}
	}
}
