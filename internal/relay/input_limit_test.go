package relay

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// wireLimit is coder/websocket's default read limit, which the relay server
// keeps as its input DoS guard (HARD-02). Senders split input below it.
const wireLimit = 32768

// readPTY reads n bytes from the PTY input pipe in a goroutine (io.Pipe blocks
// the hub's write until read) and returns what arrived within wait, or nil.
func readPTY(r io.Reader, n int, wait time.Duration) []byte {
	got := make(chan []byte, 1)
	go func() {
		buf := make([]byte, n)
		if _, err := io.ReadFull(r, buf); err == nil {
			got <- buf
		}
	}()
	select {
	case b := <-got:
		return b
	case <-time.After(wait):
		return nil
	}
}

// readUntilClosed drains conn until its Read fails; it reports false if the
// socket is still open after wait.
func readUntilClosed(conn *websocket.Conn, wait time.Duration) (error, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	for {
		if _, _, err := conn.Read(ctx); err != nil {
			return err, ctx.Err() == nil
		}
	}
}

func TestServer_InputFrameAtLimitReachesPTY(t *testing.T) {
	srv, _, _, inputRead, sessionID := setupTestServer(t)
	conn := dialWS(t, srv.URL, sessionID)

	payload := bytes.Repeat([]byte("a"), wireLimit-1)
	if err := conn.Write(context.Background(), websocket.MessageBinary, MakeInputFrame(payload)); err != nil {
		t.Fatalf("write %d-byte frame: %v", wireLimit, err)
	}
	if got := readPTY(inputRead, len(payload), 5*time.Second); !bytes.Equal(got, payload) {
		t.Fatalf("PTY got %d bytes; want the whole %d-byte payload", len(got), len(payload))
	}
}

func TestServer_InputFrameOverLimitDropped(t *testing.T) {
	srv, _, _, inputRead, sessionID := setupTestServer(t)
	conn := dialWS(t, srv.URL, sessionID)

	payload := bytes.Repeat([]byte("a"), wireLimit)
	if err := conn.Write(context.Background(), websocket.MessageBinary, MakeInputFrame(payload)); err != nil {
		t.Fatalf("write %d-byte frame: %v", wireLimit+1, err)
	}
	if got := readPTY(inputRead, 1, 500*time.Millisecond); got != nil {
		t.Fatalf("an over-limit input frame reached the PTY: %q", got)
	}
	err, closed := readUntilClosed(conn, 5*time.Second)
	if !closed {
		t.Fatal("the server kept the socket open after an over-limit input frame")
	}
	t.Logf("socket closed: %v (status %d)", err, websocket.CloseStatus(err))
}
