package relay

// 05-08 (SHAR-07, VERF-01): a viewer that joins after the raw ring has wrapped
// must end up with the host's screen, not a staircased one (todo
// 2026-11-17-jumbled-screen-on-reattach). Both sides here are vt emulators: a
// reference fed the whole raw PTY stream (the host) and a guest fed only the
// replay (the viewer).

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	xvt "github.com/charmbracelet/x/vt"
)

const (
	fidCols = 100
	fidRows = 30
	// dynLines is the newline count of claudeLikeFrame: Ink erases this many lines.
	dynLines = 6
)

// eraseLines is Ink's ansi-escapes eraseLines: ESC[2K and ESC[1A per line of
// the previous frame, then ESC[G.
func eraseLines(n int) string {
	var b strings.Builder
	for i := range n {
		b.WriteString("\x1b[2K")
		if i < n-1 {
			b.WriteString("\x1b[1A")
		}
	}
	if n > 0 {
		b.WriteString("\x1b[G")
	}
	return b.String()
}

// claudeLikeFrame is Claude Code's dynamic area: spinner, a box-drawn input
// frame exactly cols wide, a status line with a CHA to a status column. It ends
// with CRLF, as Ink's log-update output does after the PTY's onlcr, then parks
// the terminal cursor at the end of the input text (Ink's cursor positioning
// for the prompt), so the host cursor is not where the text alone leaves it.
func claudeLikeFrame(i, cols int) string {
	inner := fmt.Sprintf(" > draft %d", i)
	return fmt.Sprintf("\x1b[38;2;215;119;87m✻\x1b[39m Thinking… (%ds)\r\n", i%60) +
		"╭" + strings.Repeat("─", cols-2) + "╮\r\n" +
		"│" + inner + strings.Repeat(" ", cols-2-len(inner)) + "│\r\n" +
		"╰" + strings.Repeat("─", cols-2) + "╯\r\n" +
		fmt.Sprintf("  \x1b[2m? for shortcuts\x1b[22m\x1b[%dGturn %d", cols-12, i) + "\r\n" +
		fmt.Sprintf("\x1b[3A\x1b[%dG", len(inner)+2)
}

// unpark moves the cursor from the input text back below the frame, where
// Ink's eraseLines expects it.
const unpark = "\x1b[3B\r"

// claudeLikeTurn is one deterministic Claude-Code-shaped turn: erase the
// previous dynamic frame, print static transcript lines (they scroll up, as
// Ink's <Static> output does), then draw the new frame, all inside one
// synchronized-output update with the cursor hidden.
func claudeLikeTurn(i, cols int) string {
	var b strings.Builder
	b.WriteString("\x1b[?2026h\x1b[?25l")
	if i > 0 {
		b.WriteString(unpark + eraseLines(dynLines))
	}
	fmt.Fprintf(&b, "\x1b[1m> turn %d: please refactor the session module\x1b[22m\r\n", i)
	fmt.Fprintf(&b, "\x1b[38;2;80;200;120m⏺\x1b[39m Read(internal/pkg/file%03d.go)\r\n", i)
	fmt.Fprintf(&b, "  \x1b[2m⎿  Read %d lines (ctrl+r to expand)\x1b[22m\r\n", 40+i%17)
	words := strings.Fields("the hub keeps a rendered history of every line that scrolled off the screen so a viewer that reattaches sees the same transcript")
	var para []string
	for n := 0; len(strings.Join(para, " ")) < cols*3/2; n++ {
		para = append(para, words[(n+i)%len(words)])
	}
	b.WriteString(strings.Join(para, " ") + "\r\n") // longer than cols: soft-wraps
	fmt.Fprintf(&b, "完成 ✅ 🚀 turn %d 测试通过\r\n", i)
	fmt.Fprintf(&b, "\x1b[48;2;30;60;30m+ added line %d\x1b[K\x1b[49m\r\n", i) // BCE: styled trailing blanks
	b.WriteString(claudeLikeFrame(i, cols))
	b.WriteString("\x1b[?25h\x1b[?2026l")
	return b.String()
}

// claudeLikeRedraw is the next live update without new transcript lines: Ink
// erases and redraws the dynamic frame, then the spinner ticks in place with
// CR + ESC[2K (two rows above the parked cursor) and the cursor returns to the
// input text.
func claudeLikeRedraw(i, cols int) string {
	return "\x1b[?2026h\x1b[?25l" + unpark + eraseLines(dynLines) + claudeLikeFrame(i, cols) +
		fmt.Sprintf("\x1b[2A\r\x1b[2K\x1b[38;2;215;119;87m✳\x1b[39m Thinking… (%ds)\x1b[2B\x1b[%dG", i%60+1, len(fmt.Sprintf(" > draft %d", i))+2) +
		"\x1b[?25h\x1b[?2026l"
}

// claudeStream returns about size bytes of Claude-Code-like output, starting
// with bracketed paste on, as Claude Code sets it once at startup.
func claudeStream(size, cols int) string {
	var b strings.Builder
	b.WriteString("\x1b[?2004h")
	for i := 0; b.Len() < size; i++ {
		b.WriteString(claudeLikeTurn(i, cols))
	}
	return b.String()
}

// fidEmu is a reference or guest emulator with the hub's scrollback size.
func fidEmu(cols, rows int) *xvt.Emulator {
	e := xvt.NewEmulator(cols, rows)
	e.SetScrollbackSize(HistoryMaxLines)
	return e
}

// feedChunks writes s to the hub in PTY-sized chunks of at most 16 KiB, cut at
// byte positions (mid-sequence, mid-rune), and the same chunks to ref.
func feedChunks(t *testing.T, hub *Hub, w io.Writer, ref *xvt.Emulator, s string) {
	t.Helper()
	for len(s) > 0 {
		n := min(len(s), 16<<10)
		both(t, hub, w, ref, s[:n])
		s = s[n:]
	}
}

// both emits s to the hub as one PTY chunk and writes it to ref.
func both(t *testing.T, hub *Hub, w io.Writer, ref *xvt.Emulator, s string) {
	t.Helper()
	emit(t, hub, w, s)
	_, _ = ref.WriteString(s)
}

// feedReplay writes each replay chunk payload to the guest, as a client does.
func feedReplay(guest *xvt.Emulator, r Replay) {
	for _, c := range r.Chunks {
		_, _ = guest.Write(c[1:])
	}
}

// screenRows is the emulator's screen, one styled string per row.
func screenRows(e *xvt.Emulator) []string {
	rows := strings.Split(e.Render(), "\n")
	for len(rows) < e.Height() {
		rows = append(rows, "")
	}
	return rows
}

// scrollbackRows is the emulator's scrollback, oldest first, one styled string per line.
func scrollbackRows(e *xvt.Emulator) []string {
	var out []string
	for _, l := range e.Scrollback().Lines() {
		out = append(out, l.Render())
	}
	return out
}

// diffGrid reports the first row where got differs from want.
func diffGrid(want, got []string) (row int, wantRow, gotRow string, same bool) {
	for i := range max(len(want), len(got)) {
		var w, g string
		if i < len(want) {
			w = want[i]
		}
		if i < len(got) {
			g = got[i]
		}
		if w != g {
			return i, w, g, false
		}
	}
	return -1, "", "", true
}

// requireSameGrid fails on the first differing row, quoting both strings.
func requireSameGrid(t *testing.T, what string, want, got []string) {
	t.Helper()
	if row, w, g, same := diffGrid(want, got); !same {
		t.Fatalf("%s row %d differs:\n host  %q\n guest %q", what, row, w, g)
	}
}

func requireSameCursor(t *testing.T, ref, guest *xvt.Emulator) {
	t.Helper()
	if r, g := ref.CursorPosition(), guest.CursorPosition(); r != g {
		t.Fatalf("cursor: host %v, guest %v", r, g)
	}
}

// wrappedClaudeSession runs Test 1's scenario: a 64 KiB ring, a 100x30 host
// grid set before any output, about 200 KiB of Claude-like turns. It returns
// the hub, its PTY writer, the reference emulator and the next turn number.
func wrappedClaudeSession(t *testing.T) (*Hub, io.Writer, *xvt.Emulator, int) {
	t.Helper()
	hub, w := runJoinHub(t, 64<<10)
	resizeHub(t, hub, fidCols, fidRows)
	ref := fidEmu(fidCols, fidRows)
	s := claudeStream(200<<10, fidCols)
	feedChunks(t, hub, w, ref, s)
	if !hub.scrollback.Truncated() {
		t.Fatal("fixture invalid: ring did not wrap")
	}
	return hub, w, ref, strings.Count(s, "\x1b[?2026h")
}

// joinGuest Joins with no resume and feeds the full replay to a fresh guest.
func joinGuest(t *testing.T, hub *Hub, cols, rows int) (*xvt.Emulator, Replay) {
	t.Helper()
	r := join(t, hub, 0, false)
	if r.Mode != ReplayFull {
		t.Fatalf("Join without resume: mode %q, want %q", r.Mode, ReplayFull)
	}
	guest := fidEmu(cols, rows)
	feedReplay(guest, r)
	return guest, r
}

func TestReplayFidelity_WrappedClaudeSession(t *testing.T) {
	hub, _, ref, _ := wrappedClaudeSession(t)
	guest, _ := joinGuest(t, hub, fidCols, fidRows)

	requireSameGrid(t, "screen", screenRows(ref), screenRows(guest))
	requireSameCursor(t, ref, guest)

	refSB, guestSB := scrollbackRows(ref), scrollbackRows(guest)
	if len(guestSB) < len(refSB)/2 {
		t.Fatalf("guest scrollback has %d lines, want at least half of the host's %d", len(guestSB), len(refSB))
	}
	if len(guestSB) > len(refSB) {
		t.Fatalf("guest scrollback has %d lines, more than the host's %d", len(guestSB), len(refSB))
	}
	requireSameGrid(t, "scrollback", refSB[len(refSB)-len(guestSB):], guestSB)
}

func TestReplayFidelity_LiveRedrawAfterReplay(t *testing.T) {
	hub, _, ref, next := wrappedClaudeSession(t)
	guest, _ := joinGuest(t, hub, fidCols, fidRows)

	live := claudeLikeRedraw(next, fidCols)
	_, _ = ref.WriteString(live)
	_, _ = guest.WriteString(live)
	requireSameGrid(t, "screen after the live redraw", screenRows(ref), screenRows(guest))
	requireSameCursor(t, ref, guest)
}

// pre0503Replay is the wrapped replay as the old RenderSnapshot sent it: the
// emulator's Render() as is, rows joined with a bare LF, no CUP, no restores.
func pre0503Replay(hub *Hub) string {
	hub.emuMu.Lock()
	defer hub.emuMu.Unlock()
	return hub.liveEmu.Render()
}

// TestReplayFidelity_DetectsPre0503Staircase is the negative control: the grid
// comparison must flag the replay format the user's staircase came from, or a
// pass of the tests above proves nothing.
func TestReplayFidelity_DetectsPre0503Staircase(t *testing.T) {
	hub, _, ref, _ := wrappedClaudeSession(t)
	guest := fidEmu(fidCols, fidRows)
	_, _ = guest.WriteString(pre0503Replay(hub))

	row, w, g, same := diffGrid(screenRows(ref), screenRows(guest))
	if same {
		t.Fatal("the pre-05-03 bare-LF replay compares equal to the host: the comparison cannot see the staircase")
	}
	t.Logf("pre-05-03 replay differs at row %d:\n host    %q\n pre0503 %q", row, w, g)
}

// TestReplayFidelity_ResumeAcrossResizeIsFull: ring bytes after an offset taken
// before a genuine host resize are laid out for the old width, so that resume
// gets a full replay; an offset taken after the resize still gets a delta.
func TestReplayFidelity_ResumeAcrossResizeIsFull(t *testing.T) {
	hub, w := runJoinHub(t, DefaultScrollbackBytes)
	resizeHub(t, hub, fidCols, fidRows)
	emit(t, hub, w, claudeLikeTurn(0, fidCols))
	before := join(t, hub, 0, false).End
	resizeHub(t, hub, 80, fidRows)
	emit(t, hub, w, claudeLikeTurn(1, 80))
	if r := join(t, hub, before, true); r.Mode != ReplayFull {
		t.Fatalf("resume from before a host resize: mode %q, want %q", r.Mode, ReplayFull)
	}
	after := join(t, hub, 0, false).End
	emit(t, hub, w, claudeLikeRedraw(2, 80))
	if r := join(t, hub, after, true); r.Mode != ReplayDelta {
		t.Fatalf("resume from after the resize: mode %q, want %q", r.Mode, ReplayDelta)
	}
}

// hubScreen returns the hub's live emulator screen and cursor.
func hubScreen(hub *Hub) ([]string, xvtPos) {
	hub.emuMu.Lock()
	defer hub.emuMu.Unlock()
	p := hub.liveEmu.CursorPosition()
	return screenRows(hub.liveEmu), xvtPos{p.X, p.Y}
}

type xvtPos struct{ X, Y int }

// TestReplayFidelity_IntactRingAcrossResizeRendered: the ring is intact but
// holds bytes from before a genuine resize, so the full replay is the rendered
// history + screen, not raw bytes laid out for 100 columns.
func TestReplayFidelity_IntactRingAcrossResizeRendered(t *testing.T) {
	hub, w := runJoinHub(t, DefaultScrollbackBytes)
	resizeHub(t, hub, fidCols, fidRows)
	for i := range 4 {
		emit(t, hub, w, claudeLikeTurn(i, fidCols))
	}
	resizeHub(t, hub, 80, fidRows)
	emit(t, hub, w, claudeLikeTurn(4, 80))
	emit(t, hub, w, claudeLikeRedraw(5, 80))
	if hub.scrollback.Truncated() {
		t.Fatal("fixture invalid: ring wrapped")
	}

	guest, r := joinGuest(t, hub, 80, fidRows)
	if p := payloads(r); bytes.Contains(p, []byte("\x1b[1A")) {
		t.Fatal("full replay carries raw Ink redraw bytes (ESC[1A) from before the resize; want the rendered screen")
	}
	want, cur := hubScreen(hub)
	requireSameGrid(t, "screen", want, screenRows(guest))
	if g := guest.CursorPosition(); g.X != cur.X || g.Y != cur.Y {
		t.Fatalf("cursor: host %v, guest %v", cur, g)
	}
}

// TestReplayFidelity_FirstResizeKeepsRaw: output written while the hub grid is
// still unknown, then the first local resize, is not a geometry change (the
// PTY was spawned at the GUI's grid), so the #109 raw replay holds.
func TestReplayFidelity_FirstResizeKeepsRaw(t *testing.T) {
	hub, w := runJoinHub(t, DefaultScrollbackBytes)
	emit(t, hub, w, claudeLikeTurn(0, fidCols))
	resizeHub(t, hub, fidCols, fidRows)
	emit(t, hub, w, claudeLikeTurn(1, fidCols)+"\x1b[6n")
	want := liveEmuQueryStripPattern.ReplaceAll(hub.ScrollbackSnapshot(), nil)
	if got := fullReplay(t, hub); !bytes.Equal(got, want) {
		t.Fatalf("full replay after the first resize is not the raw ring (#109): got %d bytes, want %d", len(got), len(want))
	}
}

// syncGuest is a guest that records whether synchronized output (2026) is on.
func syncGuest(cols, rows int) (*xvt.Emulator, *bool) {
	g := fidEmu(cols, rows)
	on := new(bool)
	set := func(v bool) func(ansi.Mode) {
		return func(m ansi.Mode) {
			if m == ansi.ModeSynchronizedOutput {
				*on = v
			}
		}
	}
	g.SetCallbacks(xvt.Callbacks{EnableMode: set(true), DisableMode: set(false)})
	return g, on
}

// TestReplayFidelity_JoinMidSyncFrame: a viewer joins inside a synchronized
// update. The replay is a complete rendered screen, so it must not leave the
// viewer's terminal holding its rendering for an end marker it never started;
// after the rest of the frame arrives live, the grids match.
func TestReplayFidelity_JoinMidSyncFrame(t *testing.T) {
	hub, w, ref, next := wrappedClaudeSession(t)
	frame := claudeLikeRedraw(next, fidCols)
	cut := strings.Index(frame, "╰") // inside the frame, after ESC[?2026h
	both(t, hub, w, ref, frame[:cut])

	guest, sync := syncGuest(fidCols, fidRows)
	feedReplay(guest, join(t, hub, 0, false))
	if *sync {
		t.Fatal("replay left synchronized output on: the viewer would hold its rendering for a frame it never saw start")
	}
	_, _ = ref.WriteString(frame[cut:])
	_, _ = guest.WriteString(frame[cut:])
	requireSameGrid(t, "screen", screenRows(ref), screenRows(guest))
	requireSameCursor(t, ref, guest)
	if *sync {
		t.Fatal("guest ended with synchronized output on")
	}
}

// TestReplayFidelity_JoinAtPendingWrap: the host's last output fills a row
// exactly, so its cursor sits in the pending-wrap state; the next printable
// character must wrap on the guest as it does on the host.
func TestReplayFidelity_JoinAtPendingWrap(t *testing.T) {
	hub, w, ref, _ := wrappedClaudeSession(t)
	both(t, hub, w, ref, unpark+strings.Repeat("P", fidCols))

	guest, _ := joinGuest(t, hub, fidCols, fidRows)
	_, _ = ref.WriteString("X\r\n")
	_, _ = guest.WriteString("X\r\n")
	requireSameGrid(t, "screen", screenRows(ref), screenRows(guest))
	requireSameCursor(t, ref, guest)
}

var updateReplayFixture = flag.Bool("update-replay-fixture", false, "rewrite the frontend wrapped-replay fixture")

const replayFixturePath = "../../frontend/src/lib/__tests__/fixtures/wrapped-replay.json"

type fixtureCursor struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// replayFixture is what the frontend feeds xterm.js to check it against the
// same replay the Go guest emulator renders (05-12). Text and cursor only.
type replayFixture struct {
	Cols        int           `json:"cols"`
	Rows        int           `json:"rows"`
	Replay      string        `json:"replay"` // base64 replay payload bytes, written after the grid frame
	Screen      []string      `json:"screen"`
	Cursor      fixtureCursor `json:"cursor"`
	HistoryTail []string      `json:"historyTail"`
	Live        string        `json:"live"` // base64 next live bytes
	ScreenAfter []string      `json:"screenAfter"`
	CursorAfter fixtureCursor `json:"cursorAfter"`
}

func plainRows(rows []string) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = strings.TrimRight(ansi.Strip(r), " ")
	}
	return out
}

func emuCursor(e *xvt.Emulator) fixtureCursor {
	p := e.CursorPosition()
	return fixtureCursor{p.X, p.Y}
}

// TestReplayFidelity_XtermFixture writes (with -update-replay-fixture) or
// checks the frontend fixture. Its content is synthetic: claudeStream output.
func TestReplayFidelity_XtermFixture(t *testing.T) {
	hub, _, ref, next := wrappedClaudeSession(t)
	r := join(t, hub, 0, false)
	f := replayFixture{
		Cols:   fidCols,
		Rows:   fidRows,
		Replay: base64.StdEncoding.EncodeToString(payloads(r)),
		Screen: plainRows(screenRows(ref)),
		Cursor: emuCursor(ref),
	}
	sb := plainRows(scrollbackRows(ref))
	f.HistoryTail = sb[max(0, len(sb)-20):]
	live := claudeLikeRedraw(next, fidCols)
	_, _ = ref.WriteString(live)
	f.Live = base64.StdEncoding.EncodeToString([]byte(live))
	f.ScreenAfter = plainRows(screenRows(ref))
	f.CursorAfter = emuCursor(ref)

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(f); err != nil {
		t.Fatal(err)
	}
	if *updateReplayFixture {
		if err := os.WriteFile(replayFixturePath, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(replayFixturePath)
	if err != nil || !bytes.Equal(got, buf.Bytes()) {
		t.Fatalf("fixture stale: run go test ./internal/relay -run TestReplayFidelity_XtermFixture -update-replay-fixture (read err: %v)", err)
	}
}

const risFixturePath = "../../frontend/src/lib/__tests__/fixtures/ris-replay.json"

type risCase struct {
	Name        string        `json:"name"`
	Cols        int           `json:"cols"`
	Rows        int           `json:"rows"`
	Replay      string        `json:"replay"` // base64 rendered replay, written after the viewer's reset
	Screen      []string      `json:"screen"` // plain rows; checked only with historyTail
	Cursor      fixtureCursor `json:"cursor"`
	HistoryTail []string      `json:"historyTail"` // the last history rows above the viewport; empty for screen-only cases
	Live        string        `json:"live"`        // base64 next live bytes
	CursorAfter fixtureCursor `json:"cursorAfter"`
}

var restoreRE = regexp.MustCompile(`\x1b\[\?(\d+)[hl]`)

// TestRenderScreen_ModesBeforeText (IN-06): every DEC mode restore except
// autowrap comes before the first screen byte; ?7 comes after the last row,
// followed only by the CUP (and a pending-wrap re-print), and no ?6 follows
// the CUP (xterm.js homes on it, WR-08).
func TestRenderScreen_ModesBeforeText(t *testing.T) {
	hub := NewHub("modes-before-text", nil, nil, DefaultScrollbackBytes, nil)
	resizeHub(t, hub, 40, 6)
	in := "\x1bc\x1b[?2004h\x1b[?25l\x1b[?7lhello\r\nworld"
	hub.emuMu.Lock()
	hub.recordFrameLocked(MakeOutputFrame([]byte(in)), []byte(in))
	out := string(hub.renderScreenLocked())
	hub.emuMu.Unlock()

	text := strings.Index(out, "hello")
	if text < 0 || !strings.Contains(out, "world") {
		t.Fatalf("no screen text in %q", out)
	}
	for _, m := range restoreRE.FindAllStringSubmatchIndex(out, -1) {
		if out[m[2]:m[3]] != "7" && m[0] > text {
			t.Errorf("restore %q after the first screen byte: %q", out[m[0]:m[1]], out)
		}
	}
	if !strings.Contains(out[:text], "\x1b[?2004h") || !strings.Contains(out[:text], "\x1b[?25l") {
		t.Errorf("app modes not restored before the text: %q", out[:text])
	}
	i7 := strings.Index(out, "\x1b[?7l")
	if i7 < strings.LastIndex(out, "\r\n") || i7 < strings.Index(out, "world") {
		t.Fatalf("?7l not after the last screen row: %q", out)
	}
	if rest := out[i7+len("\x1b[?7l"):]; !regexp.MustCompile(`^\x1b\[\d+;\d+H$`).MatchString(rest) {
		t.Errorf("after ?7l want the CUP only (autowrap off: no pending wrap), got %q", rest)
	}
	if cup := cupRE.FindAllStringIndex(out, -1); len(cup) == 0 || strings.Contains(out[cup[len(cup)-1][0]:], "\x1b[?6") {
		t.Errorf("?6 after the CUP: %q", out)
	}
}

// TestRenderScreen_RISKeepsCursor: a RIS fills h.modes with every vt default,
// 6 (DECOM) included. xterm.js homes its cursor on ?6h and ?6l, so a rendered
// replay must restore modes before its CUP (WR-08). The vt guest here does not
// home on DECOM, so the frontend checks the same bytes in xterm.js through the
// fixture this writes (with -update-replay-fixture) or checks.
func TestRenderScreen_RISKeepsCursor(t *testing.T) {
	var cases []risCase
	var sixty strings.Builder
	for i := range 60 {
		fmt.Fprintf(&sixty, "hist %02d\r\n", i)
	}
	for _, c := range []struct {
		name, out, live string
		history         bool // a rendered Join replay (history, then the screen), not the screen alone
	}{
		{"reset, then a prompt", "\x1bcline1\r\nline2\r\n$ ls", " -l", false},
		{"reset, then a full row (pending wrap)", "\x1bc$ " + strings.Repeat("x", 38), "Y", false},
		{"app sets and resets DECOM", "\x1b[?6h\x1b[?6l$ ls", " -l", false},
		{"RIS, then 60 lines, rendered Join replay with history", "\x1bc" + sixty.String() + "$ ls", " -l", true},
	} {
		ring := DefaultScrollbackBytes
		if c.history {
			ring = 256 // the ring wraps, so Join renders history + screen
		}
		hub := NewHub("ris-render", nil, nil, ring, nil)
		resizeHub(t, hub, 40, 6)
		ref := fidEmu(40, 6)
		hub.emuMu.Lock()
		hub.ensureLiveEmulatorLocked()
		hub.recordFrameLocked(MakeOutputFrame([]byte(c.out)), []byte(c.out))
		out := hub.renderScreenLocked()
		hub.emuMu.Unlock()
		if c.history {
			out = fullReplay(t, hub)
			if !hub.scrollback.Truncated() || !bytes.Contains(out, []byte("hist 00")) {
				t.Fatalf("%s: fixture invalid: not the rendered history path: %q", c.name, out)
			}
		}
		_, _ = ref.WriteString(c.out)
		host := emuCursor(ref)
		screen := plainRows(screenRows(ref))
		tail := []string{}
		if c.history {
			sb := plainRows(scrollbackRows(ref))
			tail = sb[max(0, len(sb)-20):]
		}

		cup := bytes.LastIndex(out, fmt.Appendf(nil, "\x1b[%d;%dH", host.Y+1, host.X+1))
		if m := bytes.LastIndex(out, []byte("\x1b[?6")); cup < 0 || m > cup {
			t.Errorf("%s: DECOM restored after the cursor CUP (homes xterm.js): %q", c.name, out)
		}
		guest := fidEmu(40, 6)
		_, _ = guest.Write(out)
		requireSameCursor(t, ref, guest)
		if c.history {
			requireSameGrid(t, c.name+" screen", screenRows(ref), screenRows(guest))
			gsb := plainRows(scrollbackRows(guest))
			requireSameGrid(t, c.name+" history tail", tail, gsb[max(0, len(gsb)-len(tail)):])
		}
		_, _ = ref.WriteString(c.live)
		_, _ = guest.WriteString(c.live)
		requireSameCursor(t, ref, guest)
		cases = append(cases, risCase{
			Name: c.name, Cols: 40, Rows: 6,
			Replay: base64.StdEncoding.EncodeToString(out), Screen: screen, Cursor: host, HistoryTail: tail,
			Live: base64.StdEncoding.EncodeToString([]byte(c.live)), CursorAfter: emuCursor(ref),
		})
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(cases); err != nil {
		t.Fatal(err)
	}
	if *updateReplayFixture {
		if err := os.WriteFile(risFixturePath, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(risFixturePath)
	if err != nil || !bytes.Equal(got, buf.Bytes()) {
		t.Fatalf("fixture stale: run go test ./internal/relay -run TestRenderScreen_RISKeepsCursor -update-replay-fixture (read err: %v)", err)
	}
}

// TestFixRenderedLinks: vt renders OSC 8 with params and URI swapped
// (charstate.go); the fix swaps them back and closes a link vt left open at
// the end of the row.
func TestFixRenderedLinks(t *testing.T) {
	for in, want := range map[string]string{
		"\x1b]8;https://example.invalid/x;id=7\x07LINK\x1b]8;;\x07": "\x1b]8;id=7;https://example.invalid/x\x07LINK\x1b]8;;\x07",
		"\x1b]8;https://x.invalid/a;\x07A\x1b]8;;\x07 b":           "\x1b]8;;https://x.invalid/a\x07A\x1b]8;;\x07 b",
		"\x1b]8;;\x07":                    "\x1b]8;;\x07",
		"plain \x1b[1mtext\x1b[m":         "plain \x1b[1mtext\x1b[m",
		"\x1b]8;https://x.invalid/a;\x07A": "\x1b]8;;https://x.invalid/a\x07A\x1b]8;;\x07", // no params: vt leaves it open at the row end
		"\x1b]8;u1;p=1\x07a\x1b]8;;\x07 \x1b]8;u2;p=2\x07b\x1b]8;;\x07": "\x1b]8;p=1;u1\x07a\x1b]8;;\x07 \x1b]8;p=2;u2\x07b\x1b]8;;\x07",
	} {
		if got := fixRenderedLinks(in); got != want {
			t.Errorf("fixRenderedLinks(%q)\n got  %q\n want %q", in, got, want)
		}
	}
}

// TestCharState_Scan: GL follows SO and SI where vt executes C0 controls, in
// byte order and across reads, never inside an OSC string, and a RIS resets it.
func TestCharState_Scan(t *testing.T) {
	for _, c := range []struct {
		name  string
		reads []string
		gl1   bool
	}{
		{"SO", []string{"a\x0eb"}, true},
		{"SO then SI", []string{"\x0e", "x\x0f"}, false},
		{"SO inside an OSC title", []string{"\x1b]0;t\x0e\x07q"}, false},
		{"SO inside an OSC cut across reads", []string{"\x1b]0;ti", "t\x0ele\x07q"}, false},
		{"SO in a CSI", []string{"\x1b[1\x0em"}, true},
		{"RIS after SO in one read", []string{"\x0eab\x1bcq"}, false},
		{"CAN aborts an ESC sequence", []string{"\x1b]0;t\x18\x0e"}, true},
		{"SO after UTF-8 with C1-like continuation bytes", []string{"\xe2\x9d\x9d\x0e"}, true},
	} {
		var cs charState
		for _, r := range c.reads {
			cs.scan([]byte(r))
		}
		if cs.gl1 != c.gl1 {
			t.Errorf("%s: GL is G1 = %v, want %v", c.name, cs.gl1, c.gl1)
		}
	}
}

// charsetJoin forces the rendered path (a 256-byte ring), feeds each of pre as
// one PTY read to the hub and a reference, joins a vt guest, then writes live
// to both and requires the same screen (links included) and cursor.
func charsetJoin(t *testing.T, pre []string, live string) (hub *Hub, ref *xvt.Emulator, replay []byte) {
	t.Helper()
	hub, w := runJoinHub(t, 256)
	resizeHub(t, hub, 40, 6)
	ref = fidEmu(40, 6)
	both(t, hub, w, ref, strings.Repeat("filler line\r\n", 30))
	for _, p := range pre {
		both(t, hub, w, ref, p)
	}
	guest, r := joinGuest(t, hub, 40, 6)
	requireSameGrid(t, "screen", screenRows(ref), screenRows(guest))
	requireSameGrid(t, "scrollback", scrollbackRows(ref), scrollbackRows(guest))
	_, _ = ref.WriteString(live)
	_, _ = guest.WriteString(live)
	requireSameGrid(t, "screen after the live bytes", screenRows(ref), screenRows(guest))
	requireSameCursor(t, ref, guest)
	return hub, ref, payloads(r)
}

const (
	testLinkOpen = "\x1b]8;id=7;https://example.invalid/x\x1b\\"
	testLinkEnd  = "\x1b]8;;\x1b\\"
)

var charsetLinkCases = []struct {
	name string
	pre  []string
	live string
	want string // a plain-text substring of the screen after the live bytes
}{
	{"G1 line drawing shifted in with SO", []string{"\x1b)0\x0elqqk"}, "qqx", "┌──┐──│"},
	{"SO then SI", []string{"\x1b)0\x0e\x0f"}, "q", "q"},
	{"G0 line drawing", []string{"\x1b(0"}, "q", "─"},
	{"RIS after SO in one read", []string{"\x1b)0\x0e\x1bcafter"}, "q", "afterq"},
	{"SO inside an OSC title", []string{"\x1b]0;t\x0e\x07"}, "q", "q"},
	{"SO inside an OSC cut across reads", []string{"\x1b]0;ti", "t\x0e\x07"}, "q", "q"},
	{"open OSC 8 link", []string{testLinkOpen + "LINK"}, "MORE" + testLinkEnd, "LINKMORE"},
}

// TestRenderedReplay_KeepsCharsetAndLink (IN-08): a viewer joining through a
// rendered replay prints the host's next bytes in the host's charset and
// inside its open link, and replayed links reach it params then URI.
func TestRenderedReplay_KeepsCharsetAndLink(t *testing.T) {
	for _, c := range charsetLinkCases {
		t.Run(c.name, func(t *testing.T) {
			_, ref, _ := charsetJoin(t, c.pre, c.live)
			if s := strings.Join(plainRows(screenRows(ref)), "\n"); !strings.Contains(s, c.want) {
				t.Fatalf("fixture invalid: reference screen lacks %q:\n%s", c.want, s)
			}
		})
	}
	t.Run("linked words in history and on screen", func(t *testing.T) {
		linked := testLinkOpen + "word" + testLinkEnd
		_, _, replay := charsetJoin(t, []string{linked + " old\r\n" + strings.Repeat("more\r\n", 8) + linked + " new"}, "!")
		if n := bytes.Count(replay, []byte("\x1b]8;id=7;https://example.invalid/x")); n != 2 {
			t.Errorf("replay carries %d params-then-URI opens, want 2 (history and screen): %q", n, replay)
		}
		if bytes.Contains(replay, []byte("\x1b]8;https://example.invalid/x;id=7")) {
			t.Errorf("replay carries the swapped URI-first form: %q", replay)
		}
	})
	t.Run("hub emulator keeps the charset across a genuine resize", func(t *testing.T) {
		hub, w := runJoinHub(t, DefaultScrollbackBytes)
		resizeHub(t, hub, 40, 6)
		emit(t, hub, w, "\x1b)0\x0e")
		resizeHub(t, hub, 30, 6) // rebuilds the emulator, which starts in ASCII
		emit(t, hub, w, "\x1b[H\x1b[2Jqqq")
		if got, _ := hubScreen(hub); plainRows(got)[0] != "───" {
			t.Fatalf("hub row 0 %q after the resize, want %q (the host is still in SO/G1)", plainRows(got)[0], "───")
		}
	})
}

const charsetLinkFixturePath = "../../frontend/src/lib/__tests__/fixtures/charset-link-replay.json"

type charsetLinkCase struct {
	Name     string   `json:"name"`
	Cols     int      `json:"cols"`
	Rows     int      `json:"rows"`
	Replay   string   `json:"replay"` // base64 rendered replay, written after the viewer's reset
	Live     string   `json:"live"`   // base64 next live bytes
	Screen   []string `json:"screen"` // plain rows after the live bytes
	Payloads []string `json:"osc8"`   // OSC 8 payloads (after "8;") in the order the viewer parses them
}

var osc8RE = regexp.MustCompile(`\x1b\]8;([^\x07\x1b]*)(?:\x07|\x1b\\)`)

// TestRenderedReplay_CharsetLinkFixture writes (with -update-replay-fixture)
// or checks charset-link-replay.json, which the frontend feeds real xterm.js.
func TestRenderedReplay_CharsetLinkFixture(t *testing.T) {
	var cases []charsetLinkCase
	for _, i := range []int{0, 2, 6} { // G1+SO, G0, open link
		c := charsetLinkCases[i]
		_, ref, replay := charsetJoin(t, c.pre, c.live)
		var p []string
		for _, m := range osc8RE.FindAllSubmatch(append(slices.Clone(replay), c.live...), -1) {
			if s := string(m[1]); s != ";" && s != "id=7;https://example.invalid/x" {
				t.Fatalf("%s: unexpected OSC 8 payload %q (want params then URI)", c.name, s)
			}
			p = append(p, string(m[1]))
		}
		cases = append(cases, charsetLinkCase{
			Name: c.name, Cols: 40, Rows: 6,
			Replay: base64.StdEncoding.EncodeToString(replay), Live: base64.StdEncoding.EncodeToString([]byte(c.live)),
			Screen: plainRows(screenRows(ref)), Payloads: append([]string{}, p...),
		})
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(cases); err != nil {
		t.Fatal(err)
	}
	if *updateReplayFixture {
		if err := os.WriteFile(charsetLinkFixturePath, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(charsetLinkFixturePath)
	if err != nil || !bytes.Equal(got, buf.Bytes()) {
		t.Fatalf("fixture stale: run go test ./internal/relay -run TestRenderedReplay_CharsetLinkFixture -update-replay-fixture (read err: %v)", err)
	}
}
