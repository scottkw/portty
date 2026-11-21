package relay

import (
	"context"
	"crypto/sha256"
	"io"
	"testing"
	"time"

	"github.com/coder/websocket"
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
