package webserver

// Phase 12 (PATH-02): a viewer that arrives through the share server (a
// browser at /app/, or a peer daemon proxying the stream verbatim) can click a
// path the session printed, but only with a capability that can write. These
// tests drive the real share read pump and hub; the printed-token rules are
// pinned in internal/relay/pathclick_test.go.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/scottkw/portty/internal/relay"
)

// wpcPNG is the smallest file previewfile takes for a PNG.
var wpcPNG = append([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"), bytes.Repeat([]byte{7}, 64)...)

// wpcPNGFile writes a PNG under a short directory (a long one would soft-wrap
// at the hub's default width and test the wrap rules instead of the pump).
func wpcPNGFile(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "wpc")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	p := filepath.Join(dir, "shot.png")
	if err := os.WriteFile(p, wpcPNG, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func wpcDial(t *testing.T, ws *WebServer, client *http.Client, sid, perms string) *websocket.Conn {
	t.Helper()
	token := issueCapFor(t, ws, sid, perms)
	headers := http.Header{}
	headers.Set("Origin", ws.BaseURL())
	return dialWebServerWS(t, client, ws.BaseURL(), "/sessions/"+sid+"/ws?cap="+token, headers)
}

// wpcRead reads frames until fn reports true, failing after 5 seconds.
func wpcRead(t *testing.T, conn *websocket.Conn, what string, fn func(typ byte, payload []byte) bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		_, msg, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("waiting for %s: %v", what, err)
		}
		typ, payload, err := relay.ParseFrame(msg)
		if err != nil {
			continue
		}
		if fn(typ, payload) {
			return
		}
	}
}

// wpcReplayMeta returns the replay meta the viewer gets on joining.
func wpcReplayMeta(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	var meta map[string]any
	wpcRead(t, conn, "the replay meta", func(typ byte, payload []byte) bool {
		var m map[string]any
		if typ == relay.MsgMeta && json.Unmarshal(payload, &m) == nil && m["replay"] != nil {
			meta = m
			return true
		}
		return false
	})
	return meta
}

// wpcPrint makes the session print s and waits until conn has seen it, so the
// hub's emulator has recorded it before any click.
func wpcPrint(t *testing.T, conn *websocket.Conn, ptyW io.Writer, s, marker string) {
	t.Helper()
	if _, err := io.WriteString(ptyW, s); err != nil {
		t.Fatal(err)
	}
	var seen []byte
	wpcRead(t, conn, "the printed output", func(typ byte, payload []byte) bool {
		if typ == relay.MsgOutput {
			seen = append(seen, payload...)
		}
		return strings.Contains(string(seen), marker)
	})
}

func wpcClick(t *testing.T, conn *websocket.Conn, text string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageBinary, relay.MakePathClickFrame(text)); err != nil {
		t.Fatalf("write path click: %v", err)
	}
}

func wpcPreviewHeader(t *testing.T, conn *websocket.Conn) relay.PreviewHeader {
	t.Helper()
	var hd relay.PreviewHeader
	wpcRead(t, conn, "a preview header", func(typ byte, payload []byte) bool {
		h, err := relay.ParsePreviewHeader(append([]byte{typ}, payload...))
		if typ == relay.MsgPreview && err == nil {
			hd = h
			return true
		}
		return false
	})
	return hd
}

func TestWebPathClick_WriteCapPreviews(t *testing.T) {
	const sid = "sess-pc-write"
	ws, client, ptyW, _ := testServerWithHub(t, sid)
	ws.SetSigningKey(capTestKey)
	png := wpcPNGFile(t)

	conn := wpcDial(t, ws, client, sid, "read,write")
	if m := wpcReplayMeta(t, conn); m["pathClick"] != true {
		t.Fatalf("write capability replay meta %v, want pathClick true", m)
	}
	wpcPrint(t, conn, ptyW, "% ls\r\n"+png+"\r\n% ", "shot.png")
	wpcClick(t, conn, png)

	hd := wpcPreviewHeader(t, conn)
	if hd.Name != "shot.png" || hd.MIME != "image/png" || hd.Size != len(wpcPNG) || !hd.Focus {
		t.Errorf("preview header %+v, want shot.png image/png %d bytes with focus", hd, len(wpcPNG))
	}
}

func TestWebPathClick_ReadCapNoCapabilityBit(t *testing.T) {
	const sid = "sess-pc-nobit"
	ws, client, _, _ := testServerWithHub(t, sid)
	ws.SetSigningKey(capTestKey)

	conn := wpcDial(t, ws, client, sid, "read")
	if m := wpcReplayMeta(t, conn); m["pathClick"] != nil {
		t.Errorf("read-only replay meta %v, want no pathClick key", m)
	}
}

func TestWebPathClick_ReadCapRefused(t *testing.T) {
	const sid = "sess-pc-refused"
	ws, client, ptyW, _ := testServerWithHub(t, sid)
	ws.SetSigningKey(capTestKey)
	png := wpcPNGFile(t)

	ro := wpcDial(t, ws, client, sid, "read")
	wpcReplayMeta(t, ro)
	wpcPrint(t, ro, ptyW, "% ls\r\n"+png+"\r\n% ", "shot.png")

	// The click must produce neither a preview nor a notice for anyone.
	wpcClick(t, ro, png)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	for {
		_, msg, err := ro.Read(ctx)
		if err != nil {
			break // the 500 ms window ended with nothing
		}
		typ, payload, perr := relay.ParseFrame(msg)
		if perr != nil {
			continue
		}
		var m map[string]any
		if typ == relay.MsgPreview || (typ == relay.MsgMeta && json.Unmarshal(payload, &m) == nil && m["notice"] != nil) {
			t.Fatalf("a read-only capability's path click produced frame type 0x%02x", typ)
		}
	}

	// Positive control on the same hub: only the capability differs.
	rw := wpcDial(t, ws, client, sid, "read,write")
	wpcReplayMeta(t, rw)
	wpcClick(t, rw, png)
	if hd := wpcPreviewHeader(t, rw); hd.Name != "shot.png" {
		t.Errorf("write capability's click on the same hub got header %+v, want shot.png", hd)
	}
}
