package webserver

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/scottkw/portty/internal/relay"
)

// The share server keeps coder/websocket's 32768-byte default read limit as
// its input DoS guard; senders split around it (HARD-02).

func TestShare_InputFrameAtLimitReachesPTY(t *testing.T) {
	_, conn, input := dialShared(t, "limit-at")
	payload := bytes.Repeat([]byte("a"), relay.MaxInputPayload)
	if err := conn.Write(context.Background(), websocket.MessageBinary, relay.MakeInputFrame(payload)); err != nil {
		t.Fatalf("write %d-byte frame: %v", len(payload)+1, err)
	}
	got := make(chan []byte, 1)
	go func() {
		buf := make([]byte, len(payload))
		_, _ = io.ReadFull(input, buf)
		got <- buf
	}()
	select {
	case b := <-got:
		if !bytes.Equal(b, payload) {
			t.Fatalf("PTY got a different %d-byte payload", len(b))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a 32768-byte input frame never reached the PTY")
	}
}

func TestShare_InputFrameOverLimitDropped(t *testing.T) {
	_, conn, input := dialShared(t, "limit-over")
	payload := bytes.Repeat([]byte("a"), relay.MaxInputPayload+1)
	if err := conn.Write(context.Background(), websocket.MessageBinary, relay.MakeInputFrame(payload)); err != nil {
		t.Fatalf("write %d-byte frame: %v", len(payload)+1, err)
	}
	got := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 256)
		n, _ := input.Read(buf)
		got <- buf[:n]
	}()
	select {
	case b := <-got:
		if len(b) > 0 {
			t.Fatalf("an over-limit input frame reached the PTY: %d bytes", len(b))
		}
	case <-time.After(500 * time.Millisecond):
	}
	ce := readUntilClose(t, conn)
	t.Logf("socket closed: status %d reason %q", ce.Code, ce.Reason)
	if ce.Code != websocket.StatusMessageTooBig {
		t.Errorf("close status = %d; want %d (the read limit)", ce.Code, websocket.StatusMessageTooBig)
	}
}
