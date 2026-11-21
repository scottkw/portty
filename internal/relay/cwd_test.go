package relay

// Phase 12 (PATH-01): the hub records OSC 7 working directories against
// absolute output rows (cwd.go). Hub-level cases feed bytes through feedHubs,
// which records frames the way Run does.

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// cwdHub is a hub at cols x rows with no reader: tests feed it with feedHubs.
func cwdHub(t *testing.T, cols, rows int) *Hub {
	t.Helper()
	hub := NewHub("cwd", strings.NewReader(""), io.Discard, DefaultScrollbackBytes, nil)
	resizeHub(t, hub, cols, rows)
	return hub
}

// osc7 is the OSC 7 report for dir, BEL-terminated.
func osc7(dir string) string { return "\x1b]7;file://" + dir + "\x07" }

// cwdRowOf returns the absolute row of the newest line (history, then screen)
// whose text contains marker, or false. Rows come only from the history
// snapshot and the screen, never hard-coded. Caller holds emuMu.
func cwdRowOfLocked(h *Hub, marker string) (uint64, bool) {
	for y := h.liveEmu.Height() - 1; y >= 0; y-- {
		if strings.Contains(ansi.Strip(screenLine(h.liveEmu, y).Render()), marker) {
			return h.history.Pushed() + uint64(h.liveEmu.ScrollbackLen()) + uint64(y), true
		}
	}
	lines, _, first := h.history.Snapshot()
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.Contains(ansi.Strip(lines[i]), marker) {
			return first + uint64(i), true
		}
	}
	return 0, false
}

// cwdOf returns the directory the log gives the line carrying marker; the
// second result is false when the marker is nowhere or has no directory.
func cwdOf(t *testing.T, h *Hub, marker string) (string, bool) {
	t.Helper()
	h.emuMu.Lock()
	defer h.emuMu.Unlock()
	row, ok := cwdRowOfLocked(h, marker)
	if !ok {
		t.Fatalf("marker %q is neither on screen nor in history", marker)
	}
	return h.cwds.at(row)
}

func wantCwd(t *testing.T, h *Hub, marker, want string) {
	t.Helper()
	if got, ok := cwdOf(t, h, marker); !ok || got != want {
		t.Errorf("%s resolves to %q (found %v), want %q", marker, got, ok, want)
	}
}

func wantNoCwd(t *testing.T, h *Hub, marker string) {
	t.Helper()
	if got, ok := cwdOf(t, h, marker); ok {
		t.Errorf("%s resolves to %q, want nothing", marker, got)
	}
}

func fillerLines(n int) string {
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, "filler %d\r\n", i)
	}
	return b.String()
}

func TestCwd_HubRecordsAtPrintRow(t *testing.T) {
	host, err := os.Hostname()
	if err != nil {
		t.Skipf("no hostname: %v", err)
	}
	h := cwdHub(t, 80, 24)
	feedHubs(osc7("/a")+"MARK-A\r\n"+osc7("/b%20c")+"MARK-B\r\n"+fillerLines(40)+
		osc7("/d")+"MARK-D\r\n"+osc7(host+"/e")+"MARK-E\r\n", 4096, h)

	if h.history.Len() == 0 {
		t.Fatal("fixture: nothing scrolled into history")
	}
	wantCwd(t, h, "MARK-A", "/a")
	wantCwd(t, h, "MARK-B", "/b c")
	wantCwd(t, h, "MARK-D", "/d")
	wantCwd(t, h, "MARK-E", "/e")

	bare := cwdHub(t, 80, 24)
	feedHubs("MARK-X\r\n"+fillerLines(40), 4096, bare)
	wantNoCwd(t, bare, "MARK-X")
}
