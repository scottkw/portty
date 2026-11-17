package relay

// CHAT-01 relay-path guard: frame bytes 0x30-0x37 (chat, chat-send, presence,
// typing, alias, inject, inject NAK, self) are retired. An old client that still
// sends them must cause zero PTY writes and keep its socket, and the server must
// never send one back. The webserver twin lives in internal/webserver/inject_test.go.
// Retired types are raw byte literals: the constants are gone from protocol.go.

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// writerFunc is a func([]byte)(int,error) adapter that implements io.Writer.
// Used to build a counting PTY writer without an io.Discard sink.
type writerFunc func(p []byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

// setupInjectTestServer creates a test relay.Server with a counting PTY writer
// and returns: the httptest.Server, HubManager, the session ID, and a pointer to
// the per-write counter.
func setupInjectTestServer(t *testing.T) (*httptest.Server, *HubManager, string, *atomic.Int32) {
	t.Helper()

	var ptyWriteCount atomic.Int32
	ptyOutputR, ptyOutputW := io.Pipe()

	countingWriter := writerFunc(func(p []byte) (int, error) {
		ptyWriteCount.Add(1)
		return len(p), nil
	})

	const sessionID = "inject-session"
	manager := NewHubManager()
	manager.Create(sessionID, ptyOutputR, countingWriter, nil)

	srv := NewServer(manager, nil)
	ts := httptest.NewServer(srv)
	t.Cleanup(func() {
		ts.Close()
		manager.Shutdown()
		ptyOutputW.Close()
	})
	return ts, manager, sessionID, &ptyWriteCount
}

// dialInjectWS dials a WebSocket to the inject test server with optional query string.
func dialInjectWS(t *testing.T, ts *httptest.Server, sessionID, query string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/sessions/" + sessionID + "/ws"
	if query != "" {
		wsURL += "?" + query
	}
	conn, _, err := websocket.Dial(context.Background(), wsURL, nil)
	if err != nil {
		t.Fatalf("dialInjectWS: %v", err)
	}
	t.Cleanup(func() { conn.CloseNow() })
	return conn
}

// retiredPayload carries every field the old chat/presence/alias/inject
// handlers would have decoded, so a regression that re-adds any arm acts on it.
const retiredPayload = `{"content":"evil","typing":true,"alias":"evil","text":"evil command\r"}`

// sendRetiredFrames writes every retired byte 0x30-0x37 twice (idempotency edge).
func sendRetiredFrames(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	for i := 0; i < 2; i++ {
		for b := byte(0x30); b <= 0x37; b++ {
			if err := conn.Write(context.Background(), websocket.MessageBinary, append([]byte{b}, retiredPayload...)); err != nil {
				t.Fatalf("write retired frame 0x%02x: %v", b, err)
			}
		}
	}
}

func sendInput(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	if err := conn.Write(context.Background(), websocket.MessageBinary, MakeInputFrame([]byte("ls\r"))); err != nil {
		t.Fatalf("write MsgInput (socket closed?): %v", err)
	}
}

// waitForCount polls until counter == want (2 s deadline), then checks it stays
// there, so a late extra write is still caught.
func waitForCount(t *testing.T, counter *atomic.Int32, want int32) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for counter.Load() != want && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	if got := counter.Load(); got != want {
		t.Fatalf("PTY write count = %d, want %d", got, want)
	}
}

// frameCollector reads every frame the server sends until the conn closes.
// coder/websocket closes the conn when a Read context expires, so reads use a
// background context in their own goroutine instead of per-read deadlines.
type frameCollector struct {
	mu     sync.Mutex
	frames [][]byte
}

func collectFrames(conn *websocket.Conn) *frameCollector {
	c := &frameCollector{}
	go func() {
		for {
			_, msg, err := conn.Read(context.Background())
			if err != nil {
				return
			}
			c.mu.Lock()
			c.frames = append(c.frames, msg)
			c.mu.Unlock()
		}
	}()
	return c
}

func (c *frameCollector) snapshot() [][]byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([][]byte(nil), c.frames...)
}

func (c *frameCollector) assertNoRetired(t *testing.T) {
	t.Helper()
	for _, f := range c.snapshot() {
		if len(f) > 0 && f[0] >= 0x30 && f[0] <= 0x37 {
			t.Errorf("server sent retired frame type 0x%02x: %q", f[0], f[1:])
		}
	}
}

// waitForViewerCount waits until a MsgMeta frame reports want viewers.
func (c *frameCollector) waitForViewerCount(t *testing.T, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, f := range c.snapshot() {
			if len(f) == 0 || f[0] != MsgMeta {
				continue
			}
			var mp MetaPayload
			if json.Unmarshal(f[1:], &mp) == nil && mp.ViewerCount != nil && *mp.ViewerCount == want {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no MsgMeta with viewerCount=%d (socket closed by a retired frame?)", want)
}

// TestRetiredFrames_RW_RelayPath: doubled retired frames write nothing; the
// MsgInput after them reaches the PTY exactly once.
func TestRetiredFrames_RW_RelayPath(t *testing.T) {
	ts, _, sessionID, ptyWriteCount := setupInjectTestServer(t)
	conn := dialInjectWS(t, ts, sessionID, "")

	sendRetiredFrames(t, conn)
	sendInput(t, conn)
	waitForCount(t, ptyWriteCount, 1)
}

// TestRetiredFrames_NoServerFrames_RelayPath: nothing the server sends after
// connect, retired frames or input has a type byte in 0x30-0x37.
func TestRetiredFrames_NoServerFrames_RelayPath(t *testing.T) {
	ts, _, sessionID, ptyWriteCount := setupInjectTestServer(t)
	conn := dialInjectWS(t, ts, sessionID, "")
	frames := collectFrames(conn)

	time.Sleep(500 * time.Millisecond)
	sendRetiredFrames(t, conn)
	sendInput(t, conn)
	waitForCount(t, ptyWriteCount, 1)
	time.Sleep(500 * time.Millisecond)

	frames.assertNoRetired(t)
	if len(frames.snapshot()) == 0 {
		t.Fatal("no frames received at all; expected at least the MsgMeta viewer count")
	}
}

// TestRetiredFrames_Concurrent_RelayPath: retired frames sent concurrently from
// two connections never write; one MsgInput from each still does.
func TestRetiredFrames_Concurrent_RelayPath(t *testing.T) {
	ts, _, sessionID, ptyWriteCount := setupInjectTestServer(t)
	a := dialInjectWS(t, ts, sessionID, "")
	b := dialInjectWS(t, ts, sessionID, "")

	var wg sync.WaitGroup
	for _, c := range []*websocket.Conn{a, b} {
		wg.Add(1)
		go func(c *websocket.Conn) {
			defer wg.Done()
			for typ := byte(0x30); typ <= 0x37; typ++ {
				_ = c.Write(context.Background(), websocket.MessageBinary, append([]byte{typ}, retiredPayload...))
			}
		}(c)
	}
	wg.Wait()
	sendInput(t, a)
	sendInput(t, b)
	waitForCount(t, ptyWriteCount, 2)
}
