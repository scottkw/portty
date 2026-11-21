package relay

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// TestPongOwed: pings owed while nobody drains coalesce into one kick, but the
// count is kept, so N pings get N pongs (relayClient matches pongs by count).
func TestPongOwed(t *testing.T) {
	p := NewPongOwed()
	const n = 5
	for i := 0; i < n; i++ {
		p.Owe()
	}
	select {
	case <-p.Kick():
	default:
		t.Fatal("no kick pending after Owe")
	}
	select {
	case <-p.Kick():
		t.Fatal("more than one kick pending")
	default:
	}
	if got := p.Take(); got != n {
		t.Fatalf("Take = %d, want %d", got, n)
	}
	if got := p.Take(); got != 0 {
		t.Fatalf("second Take = %d, want 0", got)
	}
}

// pumpServer runs WritePump for sub on a real WebSocket and returns the
// client end. The handler keeps reading so control frames are processed.
func pumpServer(t *testing.T, hub *Hub, sub *Subscriber, pongs *PongOwed) *websocket.Conn {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		readDone := make(chan struct{})
		go func() {
			defer close(readDone)
			for {
				if _, _, err := conn.Read(r.Context()); err != nil {
					return
				}
			}
		}()
		WritePump(r.Context(), conn, hub, sub, pongs, readDone)
	}))
	t.Cleanup(srv.Close)
	conn, _, err := websocket.Dial(context.Background(), "ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.CloseNow() })
	return conn
}

func pumpHub(t *testing.T) *Hub {
	h := NewHub("pump-test", bytes.NewReader(nil), nil, 1024, nil)
	t.Cleanup(h.Shutdown)
	return h
}

// TestWritePump_PongBeforeQueuedOutput (INPT-02): a ping that arrives while
// the viewer's outgoing queue is full is answered ahead of the queued output,
// not dropped and not stuck behind 256 frames.
func TestWritePump_PongBeforeQueuedOutput(t *testing.T) {
	hub := pumpHub(t)
	sub := &Subscriber{Msgs: make(chan []byte, 256)}
	for i := 0; i < cap(sub.Msgs); i++ {
		sub.Msgs <- []byte{MsgOutput, byte(i >> 8), byte(i)}
	}
	pongs := NewPongOwed()
	pongs.Owe()
	conn := pumpServer(t, hub, sub, pongs)

	if typ, _ := readFrame(t, conn); typ != MsgPong {
		t.Fatalf("first frame type 0x%02x, want the pong (0x%02x) ahead of queued output", typ, MsgPong)
	}
	for i := 0; i < 256; i++ {
		typ, payload := readFrame(t, conn)
		if typ != MsgOutput || len(payload) != 2 || int(payload[0])<<8|int(payload[1]) != i {
			t.Fatalf("output %d: type 0x%02x payload %v, want queued frame %d in order", i, typ, payload, i)
		}
	}
}

// TestWritePump_TwoPingsTwoPongs: two pings owed before the pump runs get two
// pongs, so a paste acked by the second ping is cleared.
func TestWritePump_TwoPingsTwoPongs(t *testing.T) {
	hub := pumpHub(t)
	sub := &Subscriber{Msgs: make(chan []byte, 256)}
	pongs := NewPongOwed()
	pongs.Owe()
	pongs.Owe()
	conn := pumpServer(t, hub, sub, pongs)
	for i := 0; i < 2; i++ {
		if typ, _ := readFrame(t, conn); typ != MsgPong {
			t.Fatalf("frame %d type 0x%02x, want a pong", i, typ)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, msg, err := conn.Read(ctx); err == nil {
		t.Fatalf("third frame %v, want only two pongs", msg)
	}
}
