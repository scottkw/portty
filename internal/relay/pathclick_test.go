package relay

// Phase 12 (PATH-02): a viewer that may write clicks a path the session
// printed and the host previews it (pathclick.go). Hub-level cases feed bytes
// through feedHubs and wait on the resolve seam; the end-to-end ones go
// through the loopback relay server.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/coder/websocket"
	"github.com/scottkw/portty/internal/previewfile"
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

const pathClickFixturePath = "../../frontend/src/lib/__tests__/fixtures/path-click.json"

type pathClickFixture struct {
	Tokens []struct {
		Line    string `json:"line"`
		Text    string `json:"text"`
		Printed bool   `json:"printed"`
	} `json:"tokens"`
	Detect []struct {
		Line  string `json:"line"`
		Links []struct {
			Text    string `json:"text"`
			Request string `json:"request"`
		} `json:"links"`
	} `json:"detect"`
}

func loadPathClickFixture(t *testing.T) pathClickFixture {
	t.Helper()
	raw, err := os.ReadFile(pathClickFixturePath)
	if err != nil {
		t.Fatal(err)
	}
	var f pathClickFixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestPathClick_TokenTable(t *testing.T) {
	f := loadPathClickFixture(t)
	if len(f.Tokens) < 11 {
		t.Fatalf("fixture holds %d token cases, want at least 11", len(f.Tokens))
	}
	for _, c := range f.Tokens {
		if got := hasToken(c.Line, c.Text); got != c.Printed {
			t.Errorf("hasToken(%q, %q) = %v, want %v", c.Line, c.Text, got, c.Printed)
		}
	}
}

func TestPathClick_DetectCasesAccepted(t *testing.T) {
	f := loadPathClickFixture(t)
	if len(f.Detect) < 13 {
		t.Fatalf("fixture holds %d detection cases, want at least 13", len(f.Detect))
	}
	for _, c := range f.Detect {
		for _, l := range c.Links {
			if !hasToken(c.Line, l.Request) {
				t.Errorf("line %q: the host would refuse the viewer's request %q", c.Line, l.Request)
			}
			if got := stripLineCol(l.Text); got != l.Request {
				t.Errorf("line %q: the host strips %q to %q, the viewer sends %q", c.Line, l.Text, got, l.Request)
			}
		}
	}
}

// pcIdle waits until no preview is streaming, so the next click is not dropped.
func pcIdle(t *testing.T, hub *Hub) {
	t.Helper()
	testutil.WaitFor(t, 2*time.Second, func() bool {
		hub.mu.Lock()
		defer hub.mu.Unlock()
		return !hub.previewBusy
	}, "the preview never finished")
}

// pcPreviewed clicks text, requires a preview and returns the header sub got.
func pcPreviewed(t *testing.T, hub *Hub, ch chan string, sub *Subscriber, text string) PreviewHeader {
	t.Helper()
	if got := pcClick(t, hub, ch, sub, text); got != pcPreview {
		t.Fatalf("click %q: outcome %q, want %q", text, got, pcPreview)
	}
	g := readPreview(t, sub, 0, 5*time.Second)
	pcIdle(t, hub)
	return g.header
}

func pcKey(t *testing.T, p string) string {
	t.Helper()
	k, err := previewfile.Key(p)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestPathClick_Relative(t *testing.T) {
	root := t.TempDir()
	p, q := filepath.Join(root, "p"), filepath.Join(root, "q")
	home := filepath.Join(root, "home")
	for _, d := range []string{p, q} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	diagram := pcWrite(t, filepath.Join(p, "docs"), "diagram.png", pcPNG)
	xPNG := pcWrite(t, p, "x.png", pcPNG)
	yPDF := pcWrite(t, root, "y.pdf", []byte("%PDF-1.4\n%%EOF\n"))
	zMD := pcWrite(t, home, "z.md", []byte("# hi\n"))
	old := userHomeDir
	userHomeDir = func() (string, error) { return home, nil }
	t.Cleanup(func() { userHomeDir = old })

	hub, ch := pcHub(t, 200, 24)
	sub, _ := previewSub()
	hub.Subscribe(sub)
	feedHubs(osc7(p)+"% ls\r\ndocs/diagram.png ./x.png ../y.pdf ~/z.md\r\n"+osc7(q)+"% \r\n", 4096, hub)

	for text, want := range map[string]string{
		"docs/diagram.png": diagram, // the directory in effect when printed, not now (q)
		"./x.png":          xPNG,
		"../y.pdf":         yPDF,
		"~/z.md":           zMD,
	} {
		if hd := pcPreviewed(t, hub, ch, sub, text); hd.Key != pcKey(t, want) {
			t.Errorf("click %q previewed %q (key %s), want %s", text, hd.Name, hd.Key, want)
		}
	}
	for _, text := range []string{"~", "~root/x.png"} {
		feedHubs(text+"\r\n", 4096, hub)
		if got := pcClick(t, hub, ch, sub, text); got != pcNoCwd {
			t.Errorf("click %q: outcome %q, want %q", text, got, pcNoCwd)
		}
	}

	bare, bch := pcHub(t, 200, 24)
	bsub, _ := previewSub()
	bare.Subscribe(bsub)
	feedHubs("docs/diagram.png\r\n", 4096, bare)
	if got := pcClick(t, bare, bch, bsub, "docs/diagram.png"); got != pcNoCwd {
		t.Errorf("no OSC 7: outcome %q, want %q", got, pcNoCwd)
	}
	if n := len(queued(bsub)); n != 0 {
		t.Errorf("no OSC 7: %d frames sent", n)
	}
}

func TestPathClick_NewestOccurrenceWins(t *testing.T) {
	root := t.TempDir()
	a, b := filepath.Join(root, "a"), filepath.Join(root, "b")
	pcWrite(t, a, "a.png", pcPNG)
	want := pcWrite(t, b, "a.png", append(bytes.Clone(pcPNG), 9, 9, 9))
	hub, ch := pcHub(t, 200, 24)
	sub, _ := previewSub()
	hub.Subscribe(sub)
	feedHubs(osc7(a)+"a.png\r\n"+osc7(b)+"a.png\r\n", 4096, hub)
	hd := pcPreviewed(t, hub, ch, sub, "a.png")
	if hd.Key != pcKey(t, want) || hd.Size != len(pcPNG)+3 {
		t.Errorf("previewed key %s size %d, want %s size %d (the newest print)", hd.Key, hd.Size, pcKey(t, want), len(pcPNG)+3)
	}
}

type countWriter struct{ n atomic.Int64 }

func (c *countWriter) Write(b []byte) (int, error) { c.n.Add(int64(len(b))); return len(b), nil }

func noticeOf(t *testing.T, frames [][]byte) string {
	t.Helper()
	if len(frames) != 1 || frames[0][0] != MsgMeta {
		t.Fatalf("want exactly one meta frame, got %d frames", len(frames))
	}
	var m MetaPayload
	if err := json.Unmarshal(frames[0][1:], &m); err != nil || m.Notice == nil {
		t.Fatalf("meta %q: err=%v", frames[0][1:], err)
	}
	return *m.Notice
}

func TestPathClick_Outcomes(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "folder"), 0o755); err != nil {
		t.Fatal(err)
	}
	txt := pcWrite(t, dir, "a.txt", []byte("hello"))
	big := filepath.Join(dir, "big.png")
	f, err := os.Create(big)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(PreviewMaxBytes + 1); err != nil { // sparse: nothing large is written
		t.Fatal(err)
	}
	f.Close()
	missing, folder := filepath.Join(dir, "nope.png"), filepath.Join(dir, "folder")

	cw := new(countWriter)
	hub := NewHub("pc", strings.NewReader(""), cw, DefaultScrollbackBytes, nil)
	resizeHub(t, hub, 200, 24)
	ch := pcSeam(hub)
	clicker, _ := previewSub()
	other, _ := previewSub()
	hub.Subscribe(clicker)
	hub.Subscribe(other)
	feedHubs(strings.Join([]string{missing, folder, txt, big}, "\r\n")+"\r\n", 4096, hub)

	for _, c := range []struct{ text, outcome, notice string }{
		{missing, pcMissing, ""},
		{folder, pcMissing, ""},
		{txt, pcUnsupported, NoticePathUnsupported},
		{big, pcTooLarge, NoticePathTooLarge},
	} {
		if got := pcClick(t, hub, ch, clicker, c.text); got != c.outcome {
			t.Errorf("%s: outcome %q, want %q", filepath.Base(c.text), got, c.outcome)
		}
		got := queued(clicker)
		if c.notice == "" && len(got) != 0 {
			t.Errorf("%s: %d frames sent to the clicker, want none", filepath.Base(c.text), len(got))
		} else if c.notice != "" && noticeOf(t, got) != c.notice {
			t.Errorf("%s: notice %q, want %q", filepath.Base(c.text), noticeOf(t, got), c.notice)
		}
		if n := len(queued(other)); n != 0 {
			t.Errorf("%s: the other viewer got %d frames", filepath.Base(c.text), n)
		}
	}
	if n := cw.n.Load(); n != 0 {
		t.Errorf("%d bytes written to the PTY", n)
	}
}

func TestPathClick_Refusals(t *testing.T) {
	hub, ch := pcHub(t, 200, 24)
	png := pcWrite(t, t.TempDir(), "a.png", pcPNG)
	feedHubs(png+"\r\n", 4096, hub)
	sub, _ := previewSub()
	hub.Subscribe(sub)
	ro, _ := previewSub()
	ro.ReadOnly = true
	hub.Subscribe(ro)

	refuse := func(name string, s *Subscriber, payload []byte) {
		t.Helper()
		if hub.PathClick(s, payload) {
			t.Errorf("%s: PathClick started a resolve", name)
		}
	}
	refuse("read-only", ro, []byte(png))
	refuse("nil subscriber", nil, []byte(png))
	refuse("empty", sub, nil)
	refuse("over the cap", sub, bytes.Repeat([]byte("a"), PathClickMaxBytes+1))
	refuse("invalid UTF-8", sub, []byte("/tmp/\xff.png"))
	refuse("newline", sub, []byte("/tmp/a\n.png"))
	refuse("escape", sub, []byte("/tmp/\x1b[0m.png"))
	refuse("C1 control", sub, []byte("/tmp/\u0085.png"))

	hub.pathClickBusy.Store(true)
	refuse("a click resolving", sub, []byte(png))
	hub.pathClickBusy.Store(false)

	for name, flag := range map[string]*bool{"preview streaming": &hub.previewBusy, "freezing": &hub.freezing} {
		hub.mu.Lock()
		*flag = true
		hub.mu.Unlock()
		refuse(name, sub, []byte(png))
		hub.mu.Lock()
		*flag = false
		hub.mu.Unlock()
	}
	if hub.pathClickBusy.Load() {
		t.Error("a refused click left the busy flag set")
	}
	pcWaitNone(t, ch)

	// A valid payload at the cap's edge still starts.
	if got := pcClick(t, hub, ch, sub, png); got != pcPreview {
		t.Errorf("valid click: outcome %q, want %q", got, pcPreview)
	}
	readPreview(t, sub, 0, 5*time.Second)
	pcIdle(t, hub)

	hub.mu.Lock()
	hub.closed = true
	hub.mu.Unlock()
	refuse("hub ended", sub, []byte(png))
	pcWaitNone(t, ch)
}

func TestPathClick_NotPrintedEdges(t *testing.T) {
	png := pcWrite(t, t.TempDir(), "a.png", pcPNG)
	cases := map[string]func(h *Hub){
		"title only": func(h *Hub) { feedHubs("\x1b]0;"+png+"\x07"+"% \r\n", 4096, h) },
		"wiped by RIS": func(h *Hub) {
			feedHubs(png+"\r\n"+fillerLines(40)+"\x1bc"+"% \r\n", 4096, h)
		},
		"trimmed from history": func(h *Hub) {
			h.emuMu.Lock()
			h.history = NewHistory(10, HistoryMaxBytes)
			h.emuMu.Unlock()
			feedHubs(png+"\r\n"+fillerLines(100), 4096, h)
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			hub, ch := pcHub(t, 200, 24)
			sub, _ := previewSub()
			hub.Subscribe(sub)
			setup(hub)
			if got := pcClick(t, hub, ch, sub, png); got != pcNotPrinted {
				t.Errorf("outcome %q, want %q", got, pcNotPrinted)
			}
			if n := len(queued(sub)); n != 0 {
				t.Errorf("%d frames sent", n)
			}
		})
	}
}

// pcLongPNG makes a PNG at an absolute path of exactly total characters whose
// last element is leaf, under a fresh directory.
func pcLongPNG(t *testing.T, total int, leaf string) string {
	t.Helper()
	base, err := os.MkdirTemp("", "p")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(base) })
	pad := total - len(base) - len("/") - len("/") - len(leaf)
	if pad < 20 {
		t.Fatalf("temp dir %q too long for a %d character path", base, total)
	}
	return pcWrite(t, filepath.Join(base, strings.Repeat("e", pad)), leaf, pcPNG)
}

func TestPathClick_WrapJoin(t *testing.T) {
	long := pcLongPNG(t, 130, "a.png")
	hub, ch := pcHub(t, 80, 24)
	sub, _ := previewSub()
	hub.Subscribe(sub)
	feedHubs("% ls\r\n"+long+"\r\n% ", 4096, hub)
	hub.emuMu.Lock()
	inHistory := hub.history.Len()
	hub.emuMu.Unlock()
	if inHistory != 0 {
		t.Fatalf("fixture: %d lines already in history", inHistory)
	}
	pcPreviewed(t, hub, ch, sub, long) // on screen

	feedHubs(fillerLines(40), 4096, hub)
	hub.emuMu.Lock()
	inHistory = hub.history.Len()
	hub.emuMu.Unlock()
	if inHistory == 0 {
		t.Fatal("fixture: nothing scrolled into history")
	}
	pcPreviewed(t, hub, ch, sub, long) // after it scrolled off
}

// pcRowsEqual lists the indexes of the stripped history rows equal to s.
func pcRowsEqual(h *Hub, s string) (idx []int, wrapped []bool) {
	h.emuMu.Lock()
	defer h.emuMu.Unlock()
	lines, wr, _ := h.history.Snapshot()
	for i, l := range lines {
		if strings.TrimRight(ansi.Strip(l), " ") == s {
			idx, wrapped = append(idx, i), append(wrapped, wr[i])
		}
	}
	return
}

func TestPathClick_WrapFallback(t *testing.T) {
	const cols = 80
	// first and fallback say which pass finds text on hub's current snapshot.
	passes := func(t *testing.T, hub *Hub, text string) (first, fallback bool) {
		t.Helper()
		snap, ok := hub.pathSnapshot()
		if !ok {
			t.Fatal("no snapshot")
		}
		_, first = snap.firstPass(text)
		_, fallback = snap.fallbackPass(text)
		return
	}
	newHub := func(t *testing.T) (*Hub, chan string, *Subscriber) {
		hub, ch := pcHub(t, cols, 24)
		sub, _ := previewSub()
		hub.Subscribe(sub)
		return hub, ch, sub
	}

	t.Run("hard row above identical wrapped row", func(t *testing.T) {
		p := pcLongPNG(t, 100, "a.png")
		h1 := p[:cols]

		// Both rows on screen: the hard row took the mark, so the first pass
		// over-joins and the fallback serves the click.
		hub, ch, sub := newHub(t)
		feedHubs(h1+"\r\n"+p+"\r\n", 4096, hub)
		if first, fallback := passes(t, hub, p); first || !fallback {
			t.Errorf("on screen: first pass %v, fallback %v, want false/true", first, fallback)
		}
		pcPreviewed(t, hub, ch, sub, p)

		// Both scrolled into history: the hard row carries the mark, the
		// wrapped row does not.
		feedHubs(fillerLines(40), 4096, hub)
		idx, wrapped := pcRowsEqual(hub, h1)
		if len(idx) != 2 || !wrapped[0] || wrapped[1] {
			t.Errorf("fixture: history rows %v wrapped %v, want two identical rows, the first wrapped only", idx, wrapped)
		}
		if first, fallback := passes(t, hub, p); first || !fallback {
			t.Errorf("in history: first pass %v, fallback %v, want false/true", first, fallback)
		}
		pcPreviewed(t, hub, ch, sub, p)

		// The hard row the newest history line, the wrapped row on screen row 0.
		hub, ch, sub = newHub(t)
		feedHubs(h1+"\r\n"+p+"\r\n", 4096, hub)
		held := func() bool {
			hub.emuMu.Lock()
			defer hub.emuMu.Unlock()
			lines, _, _ := hub.history.Snapshot()
			return len(lines) > 0 && strings.TrimRight(ansi.Strip(lines[len(lines)-1]), " ") == h1 &&
				strings.TrimRight(ansi.Strip(screenLine(hub.liveEmu, 0).Render()), " ") == h1
		}
		for i := 0; i < 80 && !held(); i++ {
			feedHubs("f\r\n", 4096, hub)
		}
		if !held() {
			t.Fatal("fixture: never reached the hard row in history above the wrapped row on screen")
		}
		pcPreviewed(t, hub, ch, sub, p)
	})

	t.Run("tail of a marked wrap", func(t *testing.T) {
		p := pcLongPNG(t, 100, "a.png")
		dir, tail := t.TempDir(), p[cols:]
		pcWrite(t, filepath.Join(dir, filepath.Dir(tail)), filepath.Base(tail), pcPNG) // the tail names a PNG here
		hub, ch, sub := newHub(t)
		feedHubs(osc7(dir)+p+"\r\n", 4096, hub)
		if first, fallback := passes(t, hub, tail); first || fallback {
			t.Errorf("first pass %v, fallback %v, want false/false", first, fallback)
		}
		if got := pcClick(t, hub, ch, sub, tail); got != pcNotPrinted {
			t.Errorf("outcome %q, want %q", got, pcNotPrinted)
		}
		if n := len(queued(sub)); n != 0 {
			t.Errorf("%d frames sent", n)
		}
	})

	t.Run("hard break between short rows", func(t *testing.T) {
		dir := t.TempDir()
		pcWrite(t, filepath.Join(dir, "docs"), "diagram.png", pcPNG)
		hub, ch, sub := newHub(t)
		feedHubs(osc7(dir)+"docs/dia\r\ngram.png\r\n", 4096, hub)
		if got := pcClick(t, hub, ch, sub, "docs/diagram.png"); got != pcNotPrinted {
			t.Errorf("outcome %q, want %q", got, pcNotPrinted)
		}
	})

	t.Run("pub key beside its private key", func(t *testing.T) {
		pub := pcLongPNG(t, 100, "id_rsa.pub")
		priv := strings.TrimSuffix(pub, ".pub")
		if err := os.WriteFile(priv, pcPNG, 0o644); err != nil {
			t.Fatal(err)
		}
		hub, ch, sub := newHub(t)
		feedHubs(pub[:cols]+"\r\n"+pub+"\r\n", 4096, hub)
		if got := pcClick(t, hub, ch, sub, priv); got != pcNotPrinted {
			t.Errorf("without .pub: outcome %q, want %q", got, pcNotPrinted)
		}
		if n := len(queued(sub)); n != 0 {
			t.Errorf("%d frames sent", n)
		}
		pcPreviewed(t, hub, ch, sub, pub) // the printed one still opens
	})
}

func TestPathClick_StripsLineCol(t *testing.T) {
	png := pcWrite(t, t.TempDir(), "a.png", pcPNG)
	hub, ch := pcHub(t, 200, 24)
	sub, _ := previewSub()
	hub.Subscribe(sub)
	feedHubs(png+"\r\n", 4096, hub)
	hd := pcPreviewed(t, hub, ch, sub, png+":12:3")
	if hd.Key != pcKey(t, png) {
		t.Errorf("previewed key %s, want %s", hd.Key, pcKey(t, png))
	}
}

// lockedBuf is a log sink the resolve goroutine and the test can share.
type lockedBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuf) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

func TestPathClick_NeverLogsPath(t *testing.T) {
	png := pcWrite(t, filepath.Join(t.TempDir(), "UNIQDIR-q7"), "UNIQFILE-x9z.png", pcPNG)
	hub, ch := pcHub(t, 200, 24)
	sub, _ := previewSub()
	hub.Subscribe(sub)
	feedHubs(png+"\r\n", 4096, hub)

	var buf lockedBuf
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })
	pcPreviewed(t, hub, ch, sub, png)
	out := buf.String()
	if n := strings.Count(out, "viewer-diag: path click"); n != 1 {
		t.Errorf("%d path click lines, want 1:\n%s", n, out)
	}
	if !strings.Contains(out, "outcome=preview") {
		t.Errorf("no outcome code in the log:\n%s", out)
	}
	for _, leak := range []string{"UNIQFILE", "UNIQDIR", filepath.Dir(filepath.Dir(png))} {
		if strings.Contains(out, leak) {
			t.Errorf("the log carries %q:\n%s", leak, out)
		}
	}
}
