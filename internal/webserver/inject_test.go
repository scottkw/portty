package webserver

// CHAT-01 web-path guard: frame bytes 0x30-0x37 (chat, chat-send, presence,
// typing, alias, inject, inject NAK, self) are retired. A client holding a
// read-only or read-write capability that still sends them must cause zero PTY
// writes and keep its socket, and the server must never send one back. The
// relay-path twin lives in internal/relay/server_inject_test.go; testing both
// entry points keeps one path from silently regaining a retired arm.

import (
	"context"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/scottkw/portty/internal/relay"
	"github.com/scottkw/portty/internal/webserver/webservertest"
)

// writerFuncWeb adapts a func to io.Writer for the counting PTY writer.
type writerFuncWeb func(p []byte) (int, error)

func (f writerFuncWeb) Write(p []byte) (int, error) { return f(p) }

// setupInjectWebServer creates a tailscale-mode WebServer backed by a hub with
// a counting PTY writer. Returns: server, HTTP client, session ID, write counter.
func setupInjectWebServer(t *testing.T) (*WebServer, *http.Client, string, *atomic.Int32) {
	t.Helper()

	const sessionID = "inject-web-session"

	var ptyWriteCount atomic.Int32
	countingWriter := writerFuncWeb(func(p []byte) (int, error) {
		ptyWriteCount.Add(1)
		return len(p), nil
	})

	manager := relay.NewHubManager()
	ptyOutputR, ptyOutputW := io.Pipe()
	manager.Create(sessionID, ptyOutputR, countingWriter, nil)

	tlsCfg, client := selfSignedTLSForTest(t)
	ws, err := NewWebServer(Config{
		BindIP:    "127.0.0.1",
		Port:      0,
		FQDN:      "127.0.0.1",
		TLSConfig: tlsCfg,
	}, manager)
	if err != nil {
		t.Fatalf("NewWebServer: %v", err)
	}
	ws.SetWhoIsClientForTest(webservertest.SameUser())
	if err := ws.Start(); err != nil {
		t.Fatalf("ws.Start: %v", err)
	}
	ws.SetSigningKey(capTestKey)
	ws.EnableSession(sessionID)

	t.Cleanup(func() {
		_ = ws.Stop()
		manager.Shutdown()
		_ = ptyOutputW.Close()
	})
	return ws, client, sessionID, &ptyWriteCount
}

func dialInjectWeb(t *testing.T, ws *WebServer, client *http.Client, sessionID, perms string) *websocket.Conn {
	t.Helper()
	headers := http.Header{}
	headers.Set("Origin", ws.BaseURL())
	return dialWebServerWS(t, client, ws.BaseURL(),
		"/sessions/"+sessionID+"/ws?cap="+issueCapFor(t, ws, sessionID, perms), headers)
}

// retiredPayload carries every field the old chat/presence/alias/inject
// handlers would have decoded, so a regression that re-adds any arm acts on it.
const retiredPayload = `{"content":"evil","typing":true,"alias":"evil","text":"evil command\r"}`

// sendRetiredAndInput writes every retired byte 0x30-0x37 twice (idempotency
// edge), then one MsgInput frame.
func sendRetiredAndInput(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		for b := byte(0x30); b <= 0x37; b++ {
			if err := conn.Write(ctx, websocket.MessageBinary, append([]byte{b}, retiredPayload...)); err != nil {
				t.Fatalf("write retired frame 0x%02x: %v", b, err)
			}
		}
	}
	if err := conn.Write(ctx, websocket.MessageBinary, relay.MakeInputFrame([]byte("ls\r"))); err != nil {
		t.Fatalf("write MsgInput after retired frames (socket closed?): %v", err)
	}
}

// waitForPTYWrites polls until counter == want (2 s deadline), then checks it
// stays there, so a late extra write is still caught.
func waitForPTYWrites(t *testing.T, counter *atomic.Int32, want int32) {
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

// TestRetiredFrames_WebPath: doubled retired frames write nothing for any cap;
// the MsgInput after them reaches the PTY once for read,write and never for
// read-only perm shapes.
func TestRetiredFrames_WebPath(t *testing.T) {
	cases := []struct {
		perms string
		want  int32
	}{
		{"read", 0},
		{"read,files.read", 0},
		{"read,write", 1},
	}
	for _, tc := range cases {
		t.Run(tc.perms, func(t *testing.T) {
			ws, client, sessionID, ptyWriteCount := setupInjectWebServer(t)
			conn := dialInjectWeb(t, ws, client, sessionID, tc.perms)
			sendRetiredAndInput(t, conn)
			waitForPTYWrites(t, ptyWriteCount, tc.want)
		})
	}
}

// TestRetiredFrames_NoServerFrames_WebPath: nothing the server sends after
// connect, retired frames or input has a type byte in 0x30-0x37.
func TestRetiredFrames_NoServerFrames_WebPath(t *testing.T) {
	ws, client, sessionID, ptyWriteCount := setupInjectWebServer(t)
	conn := dialInjectWeb(t, ws, client, sessionID, "read,write")

	// coder/websocket closes the conn when a Read context expires, so read in
	// one goroutine with a background context until cleanup closes the conn.
	var mu sync.Mutex
	var frames [][]byte
	go func() {
		for {
			_, msg, err := conn.Read(context.Background())
			if err != nil {
				return
			}
			mu.Lock()
			frames = append(frames, msg)
			mu.Unlock()
		}
	}()

	time.Sleep(500 * time.Millisecond)
	sendRetiredAndInput(t, conn)
	waitForPTYWrites(t, ptyWriteCount, 1)
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(frames) == 0 {
		t.Fatal("no frames received at all; expected at least the MsgMeta viewer count")
	}
	for _, f := range frames {
		if len(f) > 0 && f[0] >= 0x30 && f[0] <= 0x37 {
			t.Errorf("server sent retired frame type 0x%02x: %q", f[0], f[1:])
		}
	}
}
