package webserver

// CHAT-01 web-path guard: the chat→PTY inject frame (0x35) is retired. A client
// holding a read-only OR read-write capability JWT that still sends a raw 0x35
// frame over the webserver WebSocket must cause zero PTY writes, the socket must
// stay open, and a following chat send must still be persisted and broadcast.
// The relay-path twin lives in internal/relay/server_inject_test.go; testing both
// entry points keeps one path from silently regaining a PTY write.

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/scottkw/agenthub/internal/relay"
)

// retiredInjectFrameType is the byte value of the retired inject frame; the
// relay constant is gone, so the test sends the raw byte an old client would.
const retiredInjectFrameType byte = 0x35

// TestInjectFrame_WebPath_NoPTYWrite is table-driven over RO perm shapes (the
// daemon's "read" and the legacy "read,files.read") and RW ("read,write").
func TestInjectFrame_WebPath_NoPTYWrite(t *testing.T) {
	cases := []struct {
		name  string
		perms string
	}{
		{"RO_read", "read"},
		{"RO_legacy_files_read", "read,files.read"},
		{"RW", "read,write"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ws, client, sessionID, ptyWriteCount := setupChatSendTestServer(t)
			token := issueCapFor(t, ws, sessionID, tc.perms)

			headers := http.Header{}
			headers.Set("Origin", ws.BaseURL())
			conn := dialWebServerWS(t, client, ws.BaseURL(),
				"/sessions/"+sessionID+"/ws?cap="+token, headers)
			time.Sleep(50 * time.Millisecond)

			ctx := context.Background()
			injectFrame := append([]byte{retiredInjectFrameType}, []byte(`{"text":"evil command"}`)...)
			if err := conn.Write(ctx, websocket.MessageBinary, injectFrame); err != nil {
				t.Fatalf("perms=%q: write 0x35 frame: %v", tc.perms, err)
			}

			chatPayload, _ := json.Marshal(relay.ChatSendPayload{Content: "after inject frame"})
			if err := conn.Write(ctx, websocket.MessageBinary, append([]byte{relay.MsgChatSend}, chatPayload...)); err != nil {
				t.Fatalf("perms=%q: write chat-send after 0x35 (socket closed?): %v", tc.perms, err)
			}

			// The read pump handles frames in order, so the chat broadcast proves
			// the 0x35 frame was already processed and the socket stayed open.
			chatFrame := waitForMsgChatWebServer(t, conn, 3*time.Second)
			if chatFrame == nil {
				t.Fatalf("perms=%q: no MsgChat broadcast after 0x35 + chat send", tc.perms)
			}
			_, payload, err := relay.ParseFrame(chatFrame)
			if err != nil {
				t.Fatalf("perms=%q: ParseFrame: %v", tc.perms, err)
			}
			var msg relay.ChatMessage
			if err := json.Unmarshal(payload, &msg); err != nil {
				t.Fatalf("perms=%q: unmarshal ChatMessage: %v", tc.perms, err)
			}
			if msg.Content != "after inject frame" {
				t.Errorf("perms=%q: Content = %q, want %q", tc.perms, msg.Content, "after inject frame")
			}
			if count := ptyWriteCount.Load(); count != 0 {
				t.Errorf("perms=%q: PTY write count = %d after 0x35 frame + chat send, want 0 (CHAT-01)", tc.perms, count)
			}
		})
	}
}
