package relay

// DEBT-02: a soft-wrapped long line must come back as one wrapped line in a
// rendered replay (no CRLF between its rows), from history and from the
// screen, while exact-width hard lines, blank continuations, autowrap off and
// the alt screen keep their CRLFs. Every case also feeds the replay to a vt
// guest and requires the reference's grid, scrollback and cursor.

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
	xvt "github.com/charmbracelet/x/vt"
)

// wrapText is a 100-character command: "$ " plus it wraps at 80 columns after
// wrapText[:78].
var wrapText = strings.Repeat("0123456789", 10)

const wrapCmd = "$ "

func TestWrapSet(t *testing.T) {
	var s wrapSet
	s.add(7)
	s.add(7)
	if !s.has(7) || !s.has(7) {
		t.Fatal("has after add: false")
	}
	if a, b, c := s.take(7), s.take(7), s.take(7); !a || !b || c {
		t.Errorf("take x3 after add x2 = %v %v %v, want true true false", a, b, c)
	}
	for k := range uint64(wrapSetMax + 1) {
		s.add(k + 100)
	}
	if s.has(100) {
		t.Error("oldest key kept after wrapSetMax+1 adds")
	}
	if !s.has(101) || !s.has(wrapSetMax+100) {
		t.Error("recent keys evicted")
	}
	s.clear()
	if s.has(101) || len(s.n) != 0 {
		t.Error("clear left keys")
	}
}

func TestLineFull(t *testing.T) {
	row := func(s string, n int) uv.Line {
		l := uv.NewLine(80)
		for i := range n {
			l[i] = uv.Cell{Content: s, Width: 1}
		}
		return l
	}
	if !lineFull(row("a", 80), 80) {
		t.Error("80 cells at width 80: not full")
	}
	if lineFull(row("a", 79), 80) {
		t.Error("79 cells at width 80: full")
	}
	wide := row("a", 78)
	wide[78] = uv.Cell{Content: "日", Width: 2}
	wide[79] = uv.Cell{}
	if !lineFull(wide, 80) {
		t.Error("wide cell at x=78: not full")
	}
	if lineFull(row(" ", 80), 80) || lineFull(uv.NewLine(80), 80) {
		t.Error("blank row: full")
	}
}

// wrapHub is an 80x24 hub with a 256-byte ring, so every join takes the
// rendered path, plus a reference emulator to feed the same bytes.
func wrapHub(t *testing.T) (*Hub, io.Writer, *xvt.Emulator) {
	t.Helper()
	hub, w := runJoinHub(t, 256)
	resizeHub(t, hub, 80, 24)
	return hub, w, fidEmu(80, 24)
}

// wrapLines feeds LINE-<from..to> to the hub and ref.
func wrapLines(t *testing.T, hub *Hub, w io.Writer, ref *xvt.Emulator, from, to int) {
	t.Helper()
	var b strings.Builder
	for i := from; i <= to; i++ {
		fmt.Fprintf(&b, "LINE-%06d %s\r\n", i, strings.Repeat("x", 16))
	}
	both(t, hub, w, ref, b.String())
}

// wrapReplay joins, requires the rendered path and a guest at cols x 24 equal
// to ref, and returns the replay payload.
func wrapReplay(t *testing.T, hub *Hub, ref *xvt.Emulator, cols int) []byte {
	t.Helper()
	if !hub.scrollback.Truncated() {
		t.Fatal("fixture invalid: ring did not wrap")
	}
	guest, r := joinGuest(t, hub, cols, 24)
	requireSameGrid(t, "screen", screenRows(ref), screenRows(guest))
	requireSameGrid(t, "scrollback", scrollbackRows(ref), scrollbackRows(guest))
	requireSameCursor(t, ref, guest)
	return payloads(r)
}

var (
	wrapJoined = []byte(wrapCmd + wrapText)                               // the two rows, no CRLF
	wrapSplit  = []byte(wrapCmd + wrapText[:78] + "\r\n" + wrapText[78:]) // two hard lines
)

func TestSoftWrap_HistoryReplay(t *testing.T) {
	hub, w, ref := wrapHub(t)
	both(t, hub, w, ref, wrapCmd+wrapText+"\r\n")
	wrapLines(t, hub, w, ref, 1, 40)
	replay := wrapReplay(t, hub, ref, 80)
	if !bytes.Contains(replay, wrapJoined) {
		t.Errorf("history replay splits the wrapped line: %q", replay)
	}
}

func TestSoftWrap_ScreenReplay(t *testing.T) {
	hub, w, ref := wrapHub(t)
	wrapLines(t, hub, w, ref, 1, 10)
	both(t, hub, w, ref, wrapCmd+wrapText+"\r\n$ ")
	replay := wrapReplay(t, hub, ref, 80)
	if !bytes.Contains(replay, wrapJoined) {
		t.Errorf("screen replay splits the wrapped line: %q", replay)
	}
}

func TestSoftWrap_ExactWidthHardLineStays(t *testing.T) {
	hub, w, ref := wrapHub(t)
	both(t, hub, w, ref, strings.Repeat("=", 80)+"\r\nafter\r\n")
	wrapLines(t, hub, w, ref, 1, 40)
	replay := wrapReplay(t, hub, ref, 80)
	if !bytes.Contains(replay, []byte(strings.Repeat("=", 80)+"\r\nafter")) {
		t.Errorf("exact-width line joined to the next: %q", replay)
	}
}

func TestSoftWrap_BlankContinuationKeepsRows(t *testing.T) {
	for _, scroll := range []bool{false, true} {
		name := "on screen"
		if scroll {
			name = "in history"
		}
		t.Run(name, func(t *testing.T) {
			hub, w, ref := wrapHub(t)
			wrapLines(t, hub, w, ref, 1, 10)
			both(t, hub, w, ref, strings.Repeat("x", 80)+" \r\nnext\r\n")
			if scroll {
				wrapLines(t, hub, w, ref, 11, 50)
			}
			replay := wrapReplay(t, hub, ref, 80)
			if !bytes.Contains(replay, []byte(strings.Repeat("x", 80)+"\r\n\r\nnext")) {
				t.Errorf("blank continuation lost its CRLF: %q", replay)
			}
		})
	}
}

func TestSoftWrap_AutowrapOffNoJoin(t *testing.T) {
	hub, w, ref := wrapHub(t)
	wrapLines(t, hub, w, ref, 1, 10)
	both(t, hub, w, ref, wrapCmd+wrapText+"\r\n\x1b[?7l")
	wrapLines(t, hub, w, ref, 11, 50)
	replay := wrapReplay(t, hub, ref, 80)
	if bytes.Contains(replay, wrapJoined) || !bytes.Contains(replay, wrapSplit) {
		t.Errorf("autowrap off, rows joined: %q", replay)
	}
}

func TestSoftWrap_AltScreenNoJoin(t *testing.T) {
	hub, w, ref := wrapHub(t)
	wrapLines(t, hub, w, ref, 1, 10)
	both(t, hub, w, ref, "\x1b[?1049h"+wrapCmd+wrapText)
	replay := wrapReplay(t, hub, ref, 80)
	if bytes.Contains(replay, wrapJoined) || !bytes.Contains(replay, wrapSplit) {
		t.Errorf("alt screen rows joined: %q", replay)
	}
}

func TestSoftWrap_RISClears(t *testing.T) {
	hub, w, ref := wrapHub(t)
	both(t, hub, w, ref, wrapCmd+wrapText+"\r\n")
	n := func() int {
		hub.emuMu.Lock()
		defer hub.emuMu.Unlock()
		return len(hub.wraps.n)
	}
	if n() == 0 {
		t.Fatal("fixture invalid: no wrap recorded")
	}
	both(t, hub, w, ref, "\x1bc")
	if k := n(); k != 0 {
		t.Errorf("wrap set holds %d keys after RIS, want 0", k)
	}
}

// TestSoftWrap_SurvivesAltExitRebuild: a wrapped line on screen during an app's
// alt-screen visit, scrolled into history after the exit rebuild, still
// replays joined: the wrap set is Hub-owned and the seed re-renders the rows.
func TestSoftWrap_SurvivesAltExitRebuild(t *testing.T) {
	hub, w, ref := wrapHub(t)
	wrapLines(t, hub, w, ref, 1, 10)
	both(t, hub, w, ref, wrapCmd+wrapText+"\r\n$ ")
	hub.emuMu.Lock()
	before := hub.liveEmu
	hub.emuMu.Unlock()
	both(t, hub, w, ref, "\x1b[?1049h")
	for _, c := range altChunks(80, 0, 500) {
		both(t, hub, w, ref, c)
	}
	both(t, hub, w, ref, "\x1b[?1049l")
	hub.emuMu.Lock()
	rebuilt := hub.liveEmu != before
	hub.emuMu.Unlock()
	if !rebuilt {
		t.Fatal("fixture invalid: the app's exit did not rebuild the emulator")
	}
	wrapLines(t, hub, w, ref, 11, 50)
	replay := wrapReplay(t, hub, ref, 80)
	if !bytes.Contains(replay, wrapJoined) {
		t.Errorf("wrap lost across the alt-exit rebuild: %q", replay)
	}
}

// TestSoftWrap_SurvivesResizePush: a wrapped line on screen when a genuine host
// resize pushes the rows into history replays joined, so a viewer at the new,
// wider grid shows it on one row as xterm.js on the host does after reflow.
// The line is 92 columns: it wraps at 80 and fits at 100.
func TestSoftWrap_SurvivesResizePush(t *testing.T) {
	hub, w, ref := wrapHub(t)
	wrapLines(t, hub, w, ref, 1, 10)
	both(t, hub, w, ref, wrapCmd+wrapText[:90]+"\r\n$ ")
	resizeHub(t, hub, 100, 24)
	emit(t, hub, w, "$ ") // the shell's SIGWINCH redraw
	if !hub.scrollback.Truncated() {
		t.Fatal("fixture invalid: ring did not wrap")
	}
	guest, r := joinGuest(t, hub, 100, 24)
	replay := payloads(r)
	if !bytes.Contains(replay, []byte(wrapCmd+wrapText[:90])) {
		t.Errorf("wrap lost across the resize push: %q", replay)
	}
	rows := plainRows(append(scrollbackRows(guest), screenRows(guest)...))
	if !slices.Contains(rows, wrapCmd+wrapText[:90]) || slices.Contains(rows, wrapCmd+wrapText[:78]) {
		t.Errorf("100-column guest does not show the line on one row: %q", rows)
	}
}

const softwrapFixturePath = "../../frontend/src/lib/__tests__/fixtures/softwrap-replay.json"

// softwrapFixture is what the frontend feeds real xterm.js to check that a
// replayed soft wrap is marked isWrapped and an exact-width line is not.
// Row indexes are into the whole buffer (scrollback, then screen).
type softwrapFixture struct {
	Cols           int    `json:"cols"`
	Rows           int    `json:"rows"`
	Replay         string `json:"replay"` // base64 rendered replay, written after the viewer's reset
	LongRow        int    `json:"longRow"`
	LongText       string `json:"longText"`
	EqRow          int    `json:"eqRow"`
	ScreenLongRow  int    `json:"screenLongRow"`
	ScreenLongText string `json:"screenLongText"`
}

// TestSoftWrap_XtermFixture writes (with -update-replay-fixture) or checks the
// softwrap-replay fixture: a wrapped line and an exact-width line in history,
// and a wrapped line on screen.
func TestSoftWrap_XtermFixture(t *testing.T) {
	screenText := wrapCmd + strings.Repeat("abcdefghij", 10)
	hub, w, ref := wrapHub(t)
	both(t, hub, w, ref, wrapCmd+wrapText+"\r\n"+strings.Repeat("=", 80)+"\r\nafter\r\n")
	wrapLines(t, hub, w, ref, 1, 30)
	both(t, hub, w, ref, screenText+"\r\n$ ")
	replay := wrapReplay(t, hub, ref, 80)
	if !bytes.Contains(replay, wrapJoined) || !bytes.Contains(replay, []byte(screenText)) {
		t.Fatalf("fixture invalid: wrapped lines not joined: %q", replay)
	}

	all := plainRows(append(scrollbackRows(ref), screenRows(ref)...))
	sbLen := len(scrollbackRows(ref))
	f := softwrapFixture{
		Cols: 80, Rows: 24, Replay: base64.StdEncoding.EncodeToString(replay),
		LongRow: slices.Index(all, wrapCmd+wrapText[:78]), LongText: wrapCmd + wrapText,
		EqRow:         slices.Index(all, strings.Repeat("=", 80)),
		ScreenLongRow: slices.Index(all, screenText[:80]), ScreenLongText: screenText,
	}
	if f.LongRow < 0 || f.EqRow < 0 || f.LongRow >= sbLen || f.EqRow >= sbLen || f.ScreenLongRow < sbLen {
		t.Fatalf("fixture invalid: rows long %d eq %d screen %d, scrollback %d", f.LongRow, f.EqRow, f.ScreenLongRow, sbLen)
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(f); err != nil {
		t.Fatal(err)
	}
	if *updateReplayFixture {
		if err := os.WriteFile(softwrapFixturePath, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(softwrapFixturePath)
	if err != nil || !bytes.Equal(got, buf.Bytes()) {
		t.Fatalf("fixture stale: run go test ./internal/relay -run TestSoftWrap_XtermFixture -update-replay-fixture (read err: %v)", err)
	}
}
