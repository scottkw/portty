package relay

// Phase 5 (SHAR-07, VERF-01): after the 256 KiB raw ring wraps, a full replay
// carries the hub's rendered-line history, then the exact host screen and
// cursor, in a byte format xterm.js (convertEol off) lays out like the host.

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
)

var lineTagRE = regexp.MustCompile(`LINE-(\d{6})`)

// historyHub starts a hub fixed at 80x24 by a local viewer, before any output.
func historyHub(t *testing.T) (*Hub, *io.PipeWriter) {
	t.Helper()
	hub, w := runJoinHub(t, DefaultScrollbackBytes)
	resizeHub(t, hub, 80, 24)
	return hub, w
}

// resizeHub drives a host-authority resize the way a local window does.
func resizeHub(t *testing.T, hub *Hub, cols, rows int) {
	t.Helper()
	sub := &Subscriber{Msgs: make(chan []byte, 256), CloseSlow: func() {}, Origin: "local"}
	hub.Subscribe(sub)
	if err := hub.ResizeClient(sub, cols, rows); err != nil {
		t.Fatalf("ResizeClient: %v", err)
	}
	hub.Unsubscribe(sub)
}

// emitLines writes LINE-<from>..LINE-<to> (about 30 bytes each) in PTY-sized chunks.
func emitLines(t *testing.T, hub *Hub, w io.Writer, from, to int) {
	t.Helper()
	var b strings.Builder
	for i := from; i <= to; i++ {
		fmt.Fprintf(&b, "LINE-%06d %s\r\n", i, strings.Repeat("x", 16))
		if b.Len() > 16<<10 {
			emit(t, hub, w, b.String())
			b.Reset()
		}
	}
	if b.Len() > 0 {
		emit(t, hub, w, b.String())
	}
}

// lineTags returns the LINE numbers in replay order.
func lineTags(p []byte) []int {
	var out []int
	for _, m := range lineTagRE.FindAllSubmatch(p, -1) {
		n, _ := strconv.Atoi(string(m[1]))
		out = append(out, n)
	}
	return out
}

func cursorCUP(t *testing.T, hub *Hub) string {
	t.Helper()
	hub.emuMu.Lock()
	defer hub.emuMu.Unlock()
	pos := hub.liveEmu.CursorPosition()
	return fmt.Sprintf("\x1b[%d;%dH", pos.Y+1, pos.X+1)
}

func TestFullReplay_WrappedRingCarriesHistory(t *testing.T) {
	hub, w := historyHub(t)
	emitLines(t, hub, w, 1, 12000)
	if !hub.scrollback.Truncated() {
		t.Fatal("fixture invalid: ring did not wrap")
	}
	tags := lineTags(fullReplay(t, hub))
	if len(tags) < HistoryMaxLines {
		t.Fatalf("full replay carries %d LINE tags, want at least %d", len(tags), HistoryMaxLines)
	}
	for i := 1; i < len(tags); i++ {
		if tags[i] != tags[i-1]+1 {
			t.Fatalf("LINE tags not consecutive at %d: %d then %d", i, tags[i-1], tags[i])
		}
	}
	if last := tags[len(tags)-1]; last != 12000 {
		t.Errorf("last LINE tag %d, want 12000", last)
	}
}

func TestFullReplay_WrappedRingFormat(t *testing.T) {
	hub, w := historyHub(t)
	emitLines(t, hub, w, 1, 12000)
	p := fullReplay(t, hub)

	for i, b := range p {
		if b == '\n' && (i == 0 || p[i-1] != '\r') {
			t.Fatalf("bare LF at byte %d (staircase in xterm.js): %q", i, p[max(0, i-20):i+1])
		}
	}
	cup := cursorCUP(t, hub)
	if !bytes.HasSuffix(p, []byte(cup)) {
		t.Fatalf("replay does not end with the host cursor %q: tail %q", cup, p[max(0, len(p)-40):])
	}
	parts := strings.Split(strings.TrimSuffix(string(p), cup), "\r\n")
	if len(parts) < 25 {
		t.Fatalf("replay has %d rows, want history plus 24 screen rows", len(parts))
	}
	screen := parts[len(parts)-24:]
	if !strings.Contains(screen[0], "LINE-011978") {
		t.Errorf("first screen row %q, want LINE-011978", screen[0])
	}
	if !strings.Contains(parts[len(parts)-25], "LINE-011977") {
		t.Errorf("last history line %q, want LINE-011977", parts[len(parts)-25])
	}
	if screen[23] != "" {
		t.Errorf("trailing blank screen row not kept: last row %q", screen[23])
	}
	if cup != "\x1b[24;1H" {
		t.Errorf("cursor %q, want row 24 col 1", cup)
	}
}

func TestHistory_Caps(t *testing.T) {
	h := NewHistory(3, 1<<20)
	for _, l := range []string{"a", "b", "c", "d"} {
		h.Push(l, false)
	}
	if h.Len() != 3 || h.Render() != "b\r\nc\r\nd" {
		t.Errorf("line cap: Len %d Render %q, want 3 %q", h.Len(), h.Render(), "b\r\nc\r\nd")
	}

	h = NewHistory(100, 10)
	for _, l := range []string{"aaaa", "bbbb", "cccc"} {
		h.Push(l, false)
	}
	if h.Render() != "bbbb\r\ncccc" {
		t.Errorf("byte cap: Render %q, want %q", h.Render(), "bbbb\r\ncccc")
	}
	h.Push("0123456789A", false) // one line over the byte cap empties the ring
	if h.Len() != 0 || h.Render() != "" {
		t.Errorf("oversized line: Len %d Render %q, want empty", h.Len(), h.Render())
	}

	h = NewHistory(5, 1<<20)
	for i := range 1000 {
		h.Push(strconv.Itoa(i), false)
	}
	if h.Render() != "995\r\n996\r\n997\r\n998\r\n999" {
		t.Errorf("after many pushes: Render %q", h.Render())
	}
	h.Clear()
	if h.Len() != 0 || h.Render() != "" {
		t.Errorf("Clear: Len %d Render %q", h.Len(), h.Render())
	}
}

// TestHistory_WrappedRender: a wrapped line joins a non-empty next line with
// no CRLF, keeps its CRLF before an empty one, and the flag survives the
// compaction and goes with Clear (DEBT-02).
func TestHistory_WrappedRender(t *testing.T) {
	h := NewHistory(4, 1<<20)
	h.Push("drop", true) // evicted below
	h.Push("ab", true)
	h.Push("cd", false)
	h.Push("ef", true)
	h.Push("", false)
	if got, want := h.Render(), "abcd\r\nef\r\n"; got != want {
		t.Errorf("Render %q, want %q", got, want)
	}
	if h.EndsWrapped() {
		t.Error("EndsWrapped after an unwrapped line")
	}
	for range 10 { // compaction keeps flags with their lines
		h.Push("gh", true)
	}
	if got, want := h.Render(), "ghghghgh"; got != want || !h.EndsWrapped() {
		t.Errorf("after compaction: Render %q EndsWrapped %v, want %q true", got, h.EndsWrapped(), want)
	}
	h.Clear()
	h.Push("x", false)
	if h.EndsWrapped() || h.Render() != "x" {
		t.Errorf("after Clear: Render %q EndsWrapped %v", h.Render(), h.EndsWrapped())
	}
}

// TestHistory_EndWrapClearedWhenRow0Erased (08-REVIEW IN-02 P7): a line that
// wrapped from history onto screen row 0 no longer continues once row 0's
// start is erased without a scroll (ED 0 from home, ED 1, EL 0 from column 0,
// EL 1, EL 2), whether the line drained in an earlier read or is still in
// vt's scrollback in the same read. ED 2 pushes row 0 into history right
// after the line, and an erase that keeps row 0's start leaves it, so both
// keep the join.
func TestHistory_EndWrapClearedWhenRow0Erased(t *testing.T) {
	pre := "\x1bca\r\nb\r\nc\r\nd\r\n" + strings.Repeat("x", 30) + "\r\n\r\n\r\n\r\n"
	for _, c := range []struct {
		name, erase string
		wrapped     bool
	}{
		{"P7 ED 0 from home", "\x1b[H\x1b[J$ prompt", false},
		{"ED 1", "\x1b[3;1H\x1b[1J\x1b[H$ prompt", false},
		{"EL 2 on row 0", "\x1b[H\x1b[2K$ prompt", false},
		{"EL 0 from column 1", "\x1b[H\x1b[K$ prompt", false},
		{"EL 1 on row 0", "\x1b[1;3H\x1b[1K\x1b[H$ prompt", false},
		{"ED 0 from row 0 column 5 keeps it", "\x1b[1;5H\x1b[J", true},
		{"EL on row 1 keeps it", "\x1b[2;1H\x1b[2K", true},
		{"ED 2 keeps it", "\x1b[H\x1b[2J", true},
	} {
		for _, oneRead := range []bool{false, true} {
			h := NewHub("in02", nil, nil, DefaultScrollbackBytes, nil)
			resizeHub(t, h, 20, 5)
			if oneRead {
				probeFeed(h, pre+c.erase)
			} else {
				probeFeed(h, pre)
				probeFeed(h, c.erase)
			}
			h.emuMu.Lock()
			hist := h.history.lines[h.history.head:]
			idx := slices.Index(hist, strings.Repeat("x", 20))
			got := idx >= 0 && h.history.wrapped[h.history.head+idx]
			h.emuMu.Unlock()
			if idx < 0 || got != c.wrapped {
				t.Errorf("%s (one read %v): the 20-x line in history %v, wrapped %v, want %v (history %q)", c.name, oneRead, idx >= 0, got, c.wrapped, hist)
			}
		}
	}
}

// TestFullReplay_IntactRingUnchanged is the #109 guard: before the ring wraps
// the full replay is the query-stripped raw ring, byte for byte.
func TestFullReplay_IntactRingUnchanged(t *testing.T) {
	hub, w := historyHub(t)
	emit(t, hub, w, "\x1b[1mBold\x1b[0m line\r\n\x1b[6n")
	emitLines(t, hub, w, 1, 200)
	if hub.scrollback.Truncated() {
		t.Fatal("fixture invalid: ring wrapped")
	}
	want := liveEmuQueryStripPattern.ReplaceAll(hub.ScrollbackSnapshot(), nil)
	if got := fullReplay(t, hub); !bytes.Equal(got, want) {
		t.Errorf("intact-ring full replay changed (#109): got %d bytes, want %d", len(got), len(want))
	}
}

// zshClear is what zsh `clear` emits; Claude Code's /clear sends the same ED3.
const zshClear = "\x1b[H\x1b[2J\x1b[3J"

func TestHistory_ED3Clears(t *testing.T) {
	hub, w := historyHub(t)
	emitLines(t, hub, w, 1, 12000)
	emit(t, hub, w, zshClear+"% ")
	if tags := lineTags(fullReplay(t, hub)); len(tags) != 0 {
		t.Errorf("ESC[3J left %d LINE tags in the replay (first %d)", len(tags), tags[0])
	}

	// The J handler must survive the resize rebuild (Pitfall 12).
	hub, w = historyHub(t)
	emitLines(t, hub, w, 1, 12000)
	resizeHub(t, hub, 100, 30)
	emitLines(t, hub, w, 12001, 12100)
	emit(t, hub, w, zshClear+"% ")
	if tags := lineTags(fullReplay(t, hub)); len(tags) != 0 {
		t.Errorf("ESC[3J after a resize rebuild left %d LINE tags (first %d)", len(tags), tags[0])
	}
}

func TestHistory_RISClears(t *testing.T) {
	hub, w := historyHub(t)
	emitLines(t, hub, w, 1, 12000)
	emit(t, hub, w, "\x1bc% ")
	if tags := lineTags(fullReplay(t, hub)); len(tags) != 0 {
		t.Errorf("RIS left %d LINE tags in the replay (first %d)", len(tags), tags[0])
	}
}

func TestHistory_SurvivesResize(t *testing.T) {
	hub, w := historyHub(t)
	emitLines(t, hub, w, 1, 12000)
	resizeHub(t, hub, 100, 30)
	emit(t, hub, w, "% ")
	tags := lineTags(fullReplay(t, hub))
	if len(tags) < HistoryMaxLines-30 {
		t.Fatalf("after a resize the replay carries %d LINE tags, want about %d", len(tags), HistoryMaxLines)
	}
	for i := 1; i < len(tags); i++ {
		if tags[i] != tags[i-1]+1 {
			t.Fatalf("LINE tags not consecutive at %d: %d then %d", i, tags[i-1], tags[i])
		}
	}
	// LINE-011978..LINE-012000 were on screen at resize time, never scrolled.
	if last := tags[len(tags)-1]; last != 12000 {
		t.Errorf("rows visible at resize time missing from history: last LINE tag %d, want 12000", last)
	}
}

var (
	cupRE      = regexp.MustCompile(`\x1b\[\d+;\d+H`)
	restoresRE = regexp.MustCompile(`(?:\x1b\[\?\d+[hl])*$`)
)

// replayTail splits the end of a rendered replay at its last CUP: any mode
// restores just before it (only ?7 belongs there, IN-06) and whatever follows
// it (the pending-wrap re-print, if any).
func replayTail(t *testing.T, p []byte) (restores, after string) {
	t.Helper()
	locs := cupRE.FindAllIndex(p, -1)
	if len(locs) == 0 {
		t.Fatalf("no CUP in replay tail %q", p[max(0, len(p)-60):])
	}
	last := locs[len(locs)-1]
	return string(restoresRE.Find(p[:last[0]])), string(p[last[1]:])
}

// leadingRestores is the run of DEC mode restores a rendered replay starts with.
var leadingRestores = regexp.MustCompile(`^(?:\x1b\[\?\d+[hl])*`)

func TestFullReplay_ModeRestores(t *testing.T) {
	hub, w := historyHub(t)
	emit(t, hub, w, "\x1b[?2004h\x1b[?1h\x1b[?25l\x1b[?1049h\x1b[?1049l\x1b[?2048h")
	emitLines(t, hub, w, 1, 12000)
	// Restores first, before the history (IN-06; xterm.js homes on ?6, WR-08),
	// ascending, no 2048/1047/1049/47; then nothing between the text and the CUP.
	want := "\x1b[?1h\x1b[?25l\x1b[?2004h"
	check := func(when string) {
		t.Helper()
		p := fullReplay(t, hub)
		if got := string(leadingRestores.Find(p)); got != want {
			t.Errorf("%s: leading restores %q, want %q", when, got, want)
		}
		if got, after := replayTail(t, p); got != "" || after != "" {
			t.Errorf("%s: %q before the CUP and %q after it, want neither", when, got, after)
		}
	}
	check("wrapped ring")

	resizeHub(t, hub, 100, 30)
	emit(t, hub, w, "% ")
	check("after a resize rebuild")
}

// TestFullReplay_ModesBeforeHistory (IN-06): after a RIS (every vt default,
// DECOM included, lands in h.modes) the Join replay starts with the restores,
// ?7 excepted, then the history; a vt guest gets the host's history tail,
// screen and cursor.
func TestFullReplay_ModesBeforeHistory(t *testing.T) {
	hub, w := historyHub(t)
	ref := fidEmu(80, 24)
	both(t, hub, w, ref, "\x1bc")
	var b strings.Builder
	for i := 1; i <= 12000; i++ {
		fmt.Fprintf(&b, "LINE-%06d %s\r\n", i, strings.Repeat("x", 16))
		if b.Len() > 16<<10 || i == 12000 {
			both(t, hub, w, ref, b.String())
			b.Reset()
		}
	}
	both(t, hub, w, ref, "\x1b[?2004h% ")
	if !hub.scrollback.Truncated() {
		t.Fatal("fixture invalid: ring did not wrap")
	}
	guest, r := joinGuest(t, hub, 80, 24)
	p := payloads(r)

	lead := string(leadingRestores.Find(p))
	if !strings.Contains(lead, "\x1b[?2004h") || !strings.Contains(lead, "\x1b[?6l") || strings.Contains(lead, "\x1b[?7") {
		t.Errorf("leading restores %q: want ?2004h and ?6l, no ?7", lead)
	}
	if first := bytes.Index(p, []byte("LINE-")); first != len(lead) {
		t.Errorf("first history line at byte %d, want right after the restores (%d)", first, len(lead))
	}
	for _, m := range regexp.MustCompile(`\x1b\[\?(\d+)[hl]`).FindAllSubmatchIndex(p[len(lead):], -1) {
		if n := string(p[len(lead)+m[2] : len(lead)+m[3]]); n != "7" {
			t.Errorf("restore ?%s after the history started", n)
		}
	}
	requireSameGrid(t, "screen", screenRows(ref), screenRows(guest))
	requireSameCursor(t, ref, guest)
	refSB, guestSB := scrollbackRows(ref), scrollbackRows(guest)
	if len(guestSB) < 200 {
		t.Fatalf("guest scrollback %d rows, want at least 200", len(guestSB))
	}
	requireSameGrid(t, "history tail", refSB[len(refSB)-200:], guestSB[len(guestSB)-200:])
}

func TestFullReplay_AltScreenOrder(t *testing.T) {
	hub, w := historyHub(t)
	emitLines(t, hub, w, 1, 12000)
	emit(t, hub, w, "\x1b[?2004h\x1b[?1049hALT-ROW-1\r\nALT-ROW-2")
	p := string(fullReplay(t, hub))

	// Restores, then the history, then ?1049h, the alt rows and the CUP (IN-06).
	if lead := "\x1b[?2004h"; !strings.HasPrefix(p, lead+"LINE-") {
		t.Fatalf("alt replay should start with the restores %q then the history: %q", lead, p[:min(40, len(p))])
	}
	i := strings.Index(p, altScreenEnterSeq)
	if i < 0 {
		t.Fatalf("no %q in alt-screen replay", altScreenEnterSeq)
	}
	hist := p[:i]
	if !strings.HasSuffix(hist, "\r\n") || !strings.Contains(hist[max(0, len(hist)-40):], "LINE-011977") {
		t.Errorf("replay before ESC[?1049h should end with the last history line and CRLF: %q", hist[max(0, len(hist)-40):])
	}
	wantTail := "\x1b[2;10H" // the CUP right after the alt rows
	if !strings.HasSuffix(p, "ALT-ROW-2"+strings.Repeat("\r\n", 22)+wantTail) {
		t.Fatalf("alt replay should end with the alt rows then the CUP %q: tail %q", wantTail, p[max(0, len(p)-40):])
	}
	rows := strings.Split(strings.TrimSuffix(p[i+len(altScreenEnterSeq):], wantTail), "\r\n")
	if len(rows) != 24 || rows[0] != "ALT-ROW-1" || rows[1] != "ALT-ROW-2" {
		t.Errorf("alt rows: %d rows, first %q", len(rows), rows[:min(2, len(rows))])
	}
}

// TestHistory_HeapBoundAfterBurst guards RESEARCH Pitfall 2: one emulator write
// of 9,000 lines must not leave the scrolled uv.Lines reachable.
func TestHistory_HeapBoundAfterBurst(t *testing.T) {
	hub := NewHub("heap-burst", nil, nil, DefaultScrollbackBytes, nil)
	var b strings.Builder
	for i := range 9000 {
		fmt.Fprintf(&b, "H%06d %s\r\n", i, strings.Repeat("z", 99))
	}
	data := []byte(b.String())
	frame := MakeOutputFrame(data)

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	hub.emuMu.Lock()
	hub.recordFrameLocked(frame, data)
	n := hub.history.Len()
	hub.emuMu.Unlock()
	runtime.GC()
	runtime.ReadMemStats(&after)

	if n < 8900 {
		t.Fatalf("history holds %d lines after the burst, want about 9000 (drain did not run)", n)
	}
	if growth := int64(after.HeapAlloc) - int64(before.HeapAlloc); growth > 32<<20 {
		t.Errorf("heap grew %d MiB after a 9,000-line burst, want under 32 MiB", growth>>20)
	}
	runtime.KeepAlive(hub)
	runtime.KeepAlive(frame)
}
