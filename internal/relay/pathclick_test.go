package relay

// Phase 12 (PATH-02): a viewer that may write clicks a path the session
// printed and the host previews it (pathclick.go). Hub-level cases feed bytes
// through feedHubs and wait on the resolve seam; the end-to-end ones go
// through the loopback relay server.

import (
	"bytes"
	"context"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/scottkw/portty/internal/testutil"
)

// pcPNG is the smallest file previewfile.MIME takes for a PNG.
var pcPNG = append([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"), bytes.Repeat([]byte{7}, 64)...)

// pcWrite writes data to dir/name (making dir) and returns the path.
func pcWrite(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// pcSeam installs the resolve seam on hub and returns its channel.
func pcSeam(hub *Hub) chan string {
	ch := make(chan string, 8)
	hub.pathClickResolved = func(outcome string) { ch <- outcome }
	return ch
}

// pcWait returns the next resolve outcome.
func pcWait(t *testing.T, ch chan string) string {
	t.Helper()
	select {
	case o := <-ch:
		return o
	case <-time.After(5 * time.Second):
		t.Fatal("the click never resolved")
		return ""
	}
}

// pcWaitNone fails if a resolve outcome arrives within a short window.
func pcWaitNone(t *testing.T, ch chan string) {
	t.Helper()
	select {
	case o := <-ch:
		t.Fatalf("unexpected resolve outcome %q", o)
	case <-time.After(150 * time.Millisecond):
	}
}

// pcClick sends text as sub's click, which must start a resolve, and returns its outcome.
func pcClick(t *testing.T, hub *Hub, ch chan string, sub *Subscriber, text string) string {
	t.Helper()
	if !hub.PathClick(sub, []byte(text)) {
		t.Fatalf("PathClick(%q) refused", text)
	}
	return pcWait(t, ch)
}

// pcHub is a hub at cols x rows with the seam installed.
func pcHub(t *testing.T, cols, rows int) (*Hub, chan string) {
	t.Helper()
	hub := cwdHub(t, cols, rows)
	return hub, pcSeam(hub)
}

// pcServer is setupTestServer with an optional read-only seam. It also drains
// the PTY input pipe so a typed character never blocks.
func pcServer(t *testing.T, readOnly bool) (*httptest.Server, *Hub, *io.PipeWriter, string) {
	t.Helper()
	ptyR, ptyW := io.Pipe()
	inR, inW := io.Pipe()
	const sid = "pc-session"
	mgr := NewHubManager()
	hub := mgr.Create(sid, ptyR, inW, nil)
	api := NewServer(mgr, nil)
	api.testReadOnly = readOnly
	srv := httptest.NewServer(api)
	go io.Copy(io.Discard, inR) //nolint:errcheck // ends when the pipe closes
	t.Cleanup(func() {
		srv.Close()
		mgr.Shutdown()
		ptyW.Close()
		inW.Close()
	})
	return srv, hub, ptyW, sid
}

// pcPrint writes s as PTY output and waits until the hub has recorded it.
func pcPrint(t *testing.T, hub *Hub, w io.Writer, s string) {
	t.Helper()
	want := hub.scrollback.End() + uint64(1+len(s))
	if _, err := io.WriteString(w, s); err != nil {
		t.Fatal(err)
	}
	testutil.WaitFor(t, 2*time.Second, func() bool { return hub.scrollback.End() >= want }, "hub never recorded the output")
}

func pcSend(t *testing.T, conn *websocket.Conn, frame []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageBinary, frame); err != nil {
		t.Fatal(err)
	}
}

// pcPreviewHeader reads frames until a preview header.
func pcPreviewHeader(t *testing.T, conn *websocket.Conn) PreviewHeader {
	t.Helper()
	for {
		typ, payload := readFrame(t, conn)
		if typ != MsgPreview {
			continue
		}
		if hd, err := ParsePreviewHeader(append([]byte{typ}, payload...)); err == nil {
			return hd
		}
	}
}

// pcFence sends a ping and reads until its pong, failing on a preview frame
// first: the pump handles frames in order, so anything the earlier frames
// caused synchronously has happened.
func pcFence(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	pcSend(t, conn, []byte{MsgPing})
	for {
		typ, _ := readFrame(t, conn)
		switch typ {
		case MsgPreview:
			t.Fatal("a preview frame reached the viewer")
		case MsgPong:
			return
		}
	}
}

func TestPathClick_LocalViewerAbsolute(t *testing.T) {
	srv, hub, ptyW, sid := pcServer(t, false)
	png := pcWrite(t, t.TempDir(), "shot.png", pcPNG)

	a := dialWS(t, srv.URL, sid)
	readJoinReplay(t, a)
	// A reports a size and types, so A is the last typist (the owner).
	pcSend(t, a, []byte{MsgResize2, 0, 200, 0, 24})
	pcSend(t, a, MakeInputFrame([]byte("x")))
	testutil.WaitFor(t, 2*time.Second, func() bool {
		hub.mu.Lock()
		defer hub.mu.Unlock()
		return hub.owner != nil
	}, "A never became the owner")
	pcPrint(t, hub, ptyW, "% ls\r\n"+png+"\r\n% ")
	seam := pcSeam(hub)

	b := dialWS(t, srv.URL, sid)
	readJoinReplay(t, b)
	pcSend(t, b, MakePathClickFrame(png))

	ha, hb := pcPreviewHeader(t, a), pcPreviewHeader(t, b)
	if got := pcWait(t, seam); got != pcPreview {
		t.Fatalf("outcome %q, want %q", got, pcPreview)
	}
	for name, hd := range map[string]PreviewHeader{"a": ha, "b": hb} {
		if hd.Name != "shot.png" || hd.MIME != "image/png" || hd.Size != len(pcPNG) {
			t.Errorf("%s header: %+v", name, hd)
		}
	}
	if ha.Focus || !hb.Focus {
		t.Errorf("focus a=%v b=%v, want false/true: the clicker takes it although A typed last", ha.Focus, hb.Focus)
	}
}

func TestPathClick_NotPrintedAbsolute(t *testing.T) {
	srv, hub, ptyW, sid := pcServer(t, false)
	png := pcWrite(t, t.TempDir(), "secret.png", pcPNG)
	pcPrint(t, hub, ptyW, "% ls\r\nsomething else\r\n% ")
	seam := pcSeam(hub)

	a := dialWS(t, srv.URL, sid)
	readJoinReplay(t, a)
	b := dialWS(t, srv.URL, sid)
	readJoinReplay(t, b)
	pcSend(t, b, MakePathClickFrame(png))
	if got := pcWait(t, seam); got != pcNotPrinted {
		t.Fatalf("outcome %q, want %q", got, pcNotPrinted)
	}
	pcFence(t, a)
	pcFence(t, b)
}

func TestPathClick_ReturnsAtOnce(t *testing.T) {
	hub, seam := pcHub(t, 200, 24)
	png := pcWrite(t, t.TempDir(), "a.png", pcPNG)
	feedHubs(png+"\r\n", 4096, hub)
	sub, _ := previewSub()
	hub.Subscribe(sub)

	hub.emuMu.Lock() // the snapshot cannot proceed: the resolve is still running
	started := hub.PathClick(sub, []byte(png))
	pcWaitNone(t, seam)
	busy := hub.pathClickBusy.Load()
	hub.emuMu.Unlock()
	if !started || !busy {
		t.Fatalf("PathClick returned %v with the flag %v while the resolve was held, want true/true", started, busy)
	}
	if got := pcWait(t, seam); got != pcPreview {
		t.Errorf("outcome %q, want %q", got, pcPreview)
	}
}

func TestPathClick_RelayPumpReadOnly(t *testing.T) {
	srv, hub, ptyW, sid := pcServer(t, true)
	png := pcWrite(t, t.TempDir(), "a.png", pcPNG)
	pcPrint(t, hub, ptyW, png+"\r\n")
	seam := pcSeam(hub)

	v := dialWS(t, srv.URL, sid)
	readJoinReplay(t, v)
	pcSend(t, v, MakePathClickFrame(png))
	pcFence(t, v) // the click was handled before the ping
	pcWaitNone(t, seam)
	if hub.pathClickBusy.Load() {
		t.Error("a read-only click started a resolve")
	}
}
