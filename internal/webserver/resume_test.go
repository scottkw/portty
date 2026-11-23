package webserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/scottkw/portty/internal/relay"
	"github.com/scottkw/portty/internal/testutil"
)

// wireMeta mirrors the replay fields of a MsgMeta payload as they appear on the
// wire (SHAR-07), so these tests pin the JSON contract, not a Go struct.
type wireMeta struct {
	Replay *string `json:"replay"`
	Offset *uint64 `json:"offset"`
}

// readWSFrame reads one binary frame or fails the test after 3s.
func readWSFrame(t *testing.T, conn *websocket.Conn) (byte, []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, msg, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read frame: %v", err)
	}
	if len(msg) == 0 {
		t.Fatal("empty frame")
	}
	return msg[0], msg[1:]
}

// readReplay reads the join sequence up to the offset meta and returns the
// replay mode, the concatenated replay output and the offset. It fails if
// output arrives before the replay meta or the offset meta precedes it.
func readReplay(t *testing.T, conn *websocket.Conn) (string, []byte, uint64) {
	t.Helper()
	mode := ""
	var out []byte
	for {
		typ, payload := readWSFrame(t, conn)
		switch typ {
		case relay.MsgOutput:
			if mode == "" {
				t.Fatalf("output frame before the replay meta: %q", payload)
			}
			if len(payload) > 16<<10 { // relay.ReplayChunk, the wire bound
				t.Fatalf("replay chunk of %d bytes exceeds 16 KiB", len(payload))
			}
			out = append(out, payload...)
		case relay.MsgMeta:
			var m wireMeta
			if err := json.Unmarshal(payload, &m); err != nil {
				t.Fatalf("meta json: %v", err)
			}
			if m.Replay != nil {
				if mode != "" {
					t.Fatalf("second replay meta %q", *m.Replay)
				}
				mode = *m.Replay
			}
			if m.Offset != nil {
				if mode == "" {
					t.Fatal("offset meta before the replay meta")
				}
				return mode, out, *m.Offset
			}
		}
	}
}

// readLiveUntil reads live frames until the output contains want and returns
// the frame bytes of every MsgOutput read, which is how far a client's resume
// offset advances (the ring stores whole frames).
func readLiveUntil(t *testing.T, conn *websocket.Conn, want string) uint64 {
	t.Helper()
	var seen []byte
	var n uint64
	for !bytes.Contains(seen, []byte(want)) {
		typ, payload := readWSFrame(t, conn)
		if typ == relay.MsgOutput {
			seen = append(seen, payload...)
			n += uint64(1 + len(payload))
		}
	}
	return n
}

func resumeTestServer(t *testing.T, sessionID string) (*WebServer, *http.Client, io.Writer, *relay.Hub, string) {
	t.Helper()
	ws, client, ptyW, _ := testServerWithHub(t, sessionID)
	ws.SetSigningKey(capTestKey)
	token := issueCapFor(t, ws, sessionID, "read,write")
	hub, _ := ws.manager.Get(sessionID)
	return ws, client, ptyW, hub, token
}

// writeOutput feeds the hub's PTY stream and waits until the hub has recorded it.
func writeOutput(t *testing.T, w io.Writer, hub *relay.Hub, s string) {
	t.Helper()
	if _, err := io.WriteString(w, s); err != nil {
		t.Fatalf("pty write: %v", err)
	}
	testutil.WaitFor(t, 2*time.Second, func() bool {
		return bytes.Contains(hub.ScrollbackSnapshot(), []byte(s))
	}, "hub never recorded %q", s)
}

func dialResume(t *testing.T, ws *WebServer, client *http.Client, sessionID, token, query string) *websocket.Conn {
	t.Helper()
	return dialResumePing(t, ws, client, sessionID, token, query, nil)
}

// dialResumePing is dialResume with an OnPingReceived hook. A nil hook is the
// library default; a non-nil hook must return true or no pong is sent.
func dialResumePing(t *testing.T, ws *WebServer, client *http.Client, sessionID, token, query string, onPing func(context.Context, []byte) bool) *websocket.Conn {
	t.Helper()
	hdr := http.Header{}
	hdr.Set("Origin", ws.BaseURL())
	conn := dialWebServerWSPing(t, client, ws.BaseURL(), "/sessions/"+sessionID+"/ws?cap="+token+query, hdr, onPing)
	conn.SetReadLimit(relay.ViewerReadLimit)
	return conn
}

// TestWSS_ResumeDeltaAfterReconnect: a viewer that reconnects with the offset
// it had reached gets only the output it missed (SHAR-07).
func TestWSS_ResumeDeltaAfterReconnect(t *testing.T) {
	const sid = "resume-delta"
	ws, client, ptyW, hub, token := resumeTestServer(t, sid)

	conn1 := dialResume(t, ws, client, sid, token, "")
	if typ, _ := readWSFrame(t, conn1); typ != relay.MsgResize {
		t.Fatalf("first frame type 0x%02x, want grid frame", typ)
	}
	_, _, offset := readReplay(t, conn1)
	writeOutput(t, ptyW, hub, "FIRST-OUTPUT\r\n")
	offset += readLiveUntil(t, conn1, "FIRST-OUTPUT")
	conn1.Close(websocket.StatusNormalClosure, "")

	writeOutput(t, ptyW, hub, "MISSED-OUTPUT\r\n")

	conn2 := dialResume(t, ws, client, sid, token, fmt.Sprintf("&resume=%d", offset))
	if typ, _ := readWSFrame(t, conn2); typ != relay.MsgResize {
		t.Fatalf("first frame type 0x%02x, want grid frame", typ)
	}
	mode, out, end := readReplay(t, conn2)
	if mode != "delta" {
		t.Fatalf("replay mode %q, want delta (resume=%d)", mode, offset)
	}
	if !bytes.Contains(out, []byte("MISSED-OUTPUT")) || bytes.Contains(out, []byte("FIRST-OUTPUT")) {
		t.Fatalf("delta replay %q: want MISSED-OUTPUT and not FIRST-OUTPUT", out)
	}
	if end <= offset {
		t.Fatalf("offset meta %d, want > resume %d", end, offset)
	}

	if _, err := io.WriteString(ptyW, "LIVE-OUTPUT\r\n"); err != nil {
		t.Fatal(err)
	}
	readLiveUntil(t, conn2, "LIVE-OUTPUT")
}

// TestWSS_ResumeFullWithoutResume: a first join gets a marked full replay,
// bracketed by the replay meta and the offset meta.
func TestWSS_ResumeFullWithoutResume(t *testing.T) {
	const sid = "resume-full"
	ws, client, ptyW, hub, token := resumeTestServer(t, sid)
	writeOutput(t, ptyW, hub, "HELLO-HISTORY\r\n")

	conn := dialResume(t, ws, client, sid, token, "")
	if typ, _ := readWSFrame(t, conn); typ != relay.MsgResize {
		t.Fatalf("first frame type 0x%02x, want grid frame", typ)
	}
	mode, out, end := readReplay(t, conn)
	if mode != "full" {
		t.Fatalf("replay mode %q, want full", mode)
	}
	if !bytes.Contains(out, []byte("HELLO-HISTORY")) {
		t.Fatalf("full replay %q lacks the history", out)
	}
	if want := uint64(len(hub.ScrollbackSnapshot())); end != want {
		t.Fatalf("offset meta %d, want ring end %d", end, want)
	}
}

// TestWSS_InputAfterWrappedReplay: after the ring wraps, a read-write viewer
// that reads a full (rendered) replay can still type into the shell (05-08:
// the reattached tab "did not respond").
func TestWSS_InputAfterWrappedReplay(t *testing.T) {
	const sid = "input-after-wrap"
	ws, client, ptyW, inputR := testServerWithHub(t, sid)
	ws.SetSigningKey(capTestKey)
	token := issueCapFor(t, ws, sid, "read,write")
	hub, _ := ws.manager.Get(sid)

	var b strings.Builder
	for i := 0; i < 10000; i++ { // about 300 KiB: wraps the 256 KiB ring
		fmt.Fprintf(&b, "WRAP-%06d %s\r\n", i, strings.Repeat("w", 16))
		if b.Len() > 15<<10 || i == 9999 {
			writeOutput(t, ptyW, hub, b.String())
			b.Reset()
		}
	}

	conn := dialResume(t, ws, client, sid, token, "")
	if typ, _ := readWSFrame(t, conn); typ != relay.MsgResize {
		t.Fatalf("first frame type 0x%02x, want grid frame", typ)
	}
	mode, out, _ := readReplay(t, conn)
	if mode != "full" || !bytes.Contains(out, []byte("WRAP-009999")) {
		t.Fatalf("replay mode %q (%d bytes), want a full replay ending at WRAP-009999", mode, len(out))
	}
	if bytes.IndexByte(out, relay.MsgOutput) >= 0 {
		t.Fatal("replay carries raw ring frame bytes: the ring did not wrap, so this is not the wrapped path")
	}

	const typed = "typed-after-replay\r"
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageBinary, relay.MakeInputFrame([]byte(typed))); err != nil {
		t.Fatalf("send input: %v", err)
	}
	var got []byte
	for len(got) < len(typed) {
		got = append(got, readPipeWithTimeout(t, inputR, 2*time.Second)...)
	}
	if string(got) != typed {
		t.Fatalf("PTY received %q, want %q", got, typed)
	}
}
