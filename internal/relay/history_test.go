package relay

// Phase 5 (SHAR-07, VERF-01): after the 256 KiB raw ring wraps, a full replay
// carries the hub's rendered-line history, then the exact host screen and
// cursor, in a byte format xterm.js (convertEol off) lays out like the host.

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
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
		h.Push(l)
	}
	if h.Len() != 3 || h.Render() != "b\r\nc\r\nd" {
		t.Errorf("line cap: Len %d Render %q, want 3 %q", h.Len(), h.Render(), "b\r\nc\r\nd")
	}

	h = NewHistory(100, 10)
	for _, l := range []string{"aaaa", "bbbb", "cccc"} {
		h.Push(l)
	}
	if h.Render() != "bbbb\r\ncccc" {
		t.Errorf("byte cap: Render %q, want %q", h.Render(), "bbbb\r\ncccc")
	}
	h.Push("0123456789A") // one line over the byte cap empties the ring
	if h.Len() != 0 || h.Render() != "" {
		t.Errorf("oversized line: Len %d Render %q, want empty", h.Len(), h.Render())
	}

	h = NewHistory(5, 1<<20)
	for i := range 1000 {
		h.Push(strconv.Itoa(i))
	}
	if h.Render() != "995\r\n996\r\n997\r\n998\r\n999" {
		t.Errorf("after many pushes: Render %q", h.Render())
	}
	h.Clear()
	if h.Len() != 0 || h.Render() != "" {
		t.Errorf("Clear: Len %d Render %q", h.Len(), h.Render())
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
