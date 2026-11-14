package relay

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
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
// the per-write counter. The counting writer increments on every Write call so
// tests can assert PTY writes were (or were not) performed.
//
// A minimal in-memory chatAppendFn is wired on the hub so that HandleChatSend can
// persist and BroadcastChat fires.
// The function fills in missing ChatMessage fields (ID, TimestampMs, SchemaVersion,
// SessionID) so the frame round-trips as a well-formed ChatMessage.
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
	hub := manager.Create(sessionID, ptyOutputR, countingWriter, nil)

	// Wire a simple in-memory chatAppendFn — stands in for daemon.ChatStore.AppendMessage
	// without importing daemon. Fills the fields ChatStore would normally fill.
	hub.SetChatAppendFn(func(msg ChatMessage) (ChatMessage, error) {
		if msg.ID == "" {
			msg.ID = "test-id"
		}
		if msg.SessionID == "" {
			msg.SessionID = sessionID
		}
		if msg.SchemaVersion == 0 {
			msg.SchemaVersion = ChatSchemaVersion
		}
		if msg.TimestampMs == 0 {
			msg.TimestampMs = 1
		}
		return msg, nil
	})

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

// waitForFrameType reads frames from conn (skipping frames of other types) and
// returns the first frame whose type byte matches msgType. Fails on timeout.
// Timeout: 3 seconds (sub-second in practice).
func waitForFrameType(t *testing.T, conn *websocket.Conn, msgType byte, label string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for {
		_, rawMsg, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("waitForFrameType(%s): %v", label, err)
		}
		if len(rawMsg) == 0 {
			continue
		}
		if rawMsg[0] == msgType {
			return rawMsg
		}
		// Skip frames of other types (e.g. MsgMeta, MsgPresence).
	}
}

// retiredInjectFrameType is the byte value of the retired chat→PTY inject frame
// (0x35, CHAT-01). The constant is gone from protocol.go; old clients may still
// send it, so tests use the raw byte.
const retiredInjectFrameType byte = 0x35

// assertInjectFrameDropped sends a raw 0x35 frame on conn, then a chat-send
// frame, and asserts: the chat send is persisted and broadcast back (so the
// socket stayed open and the read pump moved past the 0x35 frame), and the PTY
// received zero writes. The read pump handles frames in order, so seeing the
// chat broadcast proves the 0x35 frame was already processed.
func assertInjectFrameDropped(t *testing.T, conn *websocket.Conn, ptyWriteCount *atomic.Int32) {
	t.Helper()
	ctx := context.Background()

	injectFrame := append([]byte{retiredInjectFrameType}, []byte(`{"text":"evil command"}`)...)
	if err := conn.Write(ctx, websocket.MessageBinary, injectFrame); err != nil {
		t.Fatalf("write 0x35 frame: %v", err)
	}

	chatPayload, _ := json.Marshal(ChatSendPayload{Content: "after inject frame"})
	if err := conn.Write(ctx, websocket.MessageBinary, append([]byte{MsgChatSend}, chatPayload...)); err != nil {
		t.Fatalf("write chat-send after 0x35 frame (socket closed?): %v", err)
	}

	rawChat := waitForFrameType(t, conn, MsgChat, "chat broadcast after 0x35 frame")
	_, payload, err := ParseFrame(rawChat)
	if err != nil {
		t.Fatalf("ParseFrame MsgChat: %v", err)
	}
	var msg ChatMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		t.Fatalf("unmarshal ChatMessage: %v", err)
	}
	if msg.Content != "after inject frame" {
		t.Errorf("chat after 0x35: Content = %q, want %q", msg.Content, "after inject frame")
	}
	if count := ptyWriteCount.Load(); count != 0 {
		t.Errorf("PTY write count = %d after 0x35 frame + chat send, want 0 (CHAT-01: no chat path reaches the PTY)", count)
	}
}

// TestInjectFrame_RO_RelayPath_NoPTYWrite: a read-only relay client's 0x35
// frame writes nothing to the PTY and a following chat send still works.
func TestInjectFrame_RO_RelayPath_NoPTYWrite(t *testing.T) {
	ts, _, sessionID, ptyWriteCount := setupInjectTestServer(t)
	conn := dialInjectWS(t, ts, sessionID, "readonly=1")
	time.Sleep(50 * time.Millisecond)
	assertInjectFrameDropped(t, conn, ptyWriteCount)
}

// TestInjectFrame_RW_RelayPath_NoPTYWrite: a read-write relay client's 0x35
// frame (which used to reach the PTY) now writes nothing and a following chat
// send still works.
func TestInjectFrame_RW_RelayPath_NoPTYWrite(t *testing.T) {
	ts, _, sessionID, ptyWriteCount := setupInjectTestServer(t)
	conn := dialInjectWS(t, ts, sessionID, "")
	time.Sleep(50 * time.Millisecond)
	assertInjectFrameDropped(t, conn, ptyWriteCount)
}

// TestInjectFrame_ChatSendAndStrayFrame_NoPTYWrite: a chat-send frame and an
// unknown frame type never write to the PTY.
func TestInjectFrame_ChatSendAndStrayFrame_NoPTYWrite(t *testing.T) {
	ts, _, sessionID, ptyWriteCount := setupInjectTestServer(t)

	conn := dialInjectWS(t, ts, sessionID, "")
	time.Sleep(50 * time.Millisecond)

	chatSendPayload, _ := json.Marshal(map[string]string{"content": "hello PTY"})
	ctx := context.Background()
	if err := conn.Write(ctx, websocket.MessageBinary, append([]byte{MsgChatSend}, chatSendPayload...)); err != nil {
		t.Fatalf("write MsgChatSend: %v", err)
	}
	if err := conn.Write(ctx, websocket.MessageBinary, []byte{0x50, 0x01, 0x02, 0x03}); err != nil {
		t.Fatalf("write stray frame: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if count := ptyWriteCount.Load(); count != 0 {
		t.Errorf("PTY write count = %d after MsgChatSend/stray frame, want 0", count)
	}
}
