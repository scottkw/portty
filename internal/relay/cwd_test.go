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

// cwdRowOfLocked returns the absolute row of the newest line (history, then screen)
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

// ownHostForms returns this Mac's hostname as an OSC 7 emitter may spell it:
// as is, upper-cased, and with `.local` added or removed.
func ownHostForms(t *testing.T) []string {
	t.Helper()
	h, err := os.Hostname()
	if err != nil || h == "" {
		t.Skipf("no hostname: %v", err)
	}
	other := h + ".local"
	if base, ok := strings.CutSuffix(h, ".local"); ok {
		other = base
	}
	return []string{h, strings.ToUpper(h), other, strings.ToUpper(other) + "."}
}

func TestParseOSC7(t *testing.T) {
	good := map[string]string{
		"7;file:///a":                          "/a",
		"7;file:///a%20b%C3%A9%3Bx%25":         "/a bé;x%",
		"7;file://localhost/x":                 "/x",
		"7;file://LOCALHOST./x":                "/x",
		"7;file:///a/../b":                     "/b",
		"7;file:///a/":                         "/a",
		"7;file:///":                           "/",
		"7;file:///" + strings.Repeat("d", 40): "/" + strings.Repeat("d", 40),
	}
	for _, h := range ownHostForms(t) {
		good["7;file://"+h+"/own"] = "/own"
	}
	good["7;file:///a?"+strings.Repeat("q", osc7MaxPayload-len("file:///a?"))] = "/a" // the URL is exactly at the cap
	for in, want := range good {
		if got, ok := parseOSC7([]byte(in)); !ok || got != want {
			t.Errorf("parseOSC7(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	bad := []string{
		"7;file://otherhost/x",
		"7;file://localhost:80/x",
		"7;",
		"7",
		"file:///a", // no "7;" prefix
		"7;file://",
		"7;file:///a?" + strings.Repeat("q", osc7MaxPayload-len("file:///a?")+1), // short path, URL over the cap
		"7;file:///a%zz",
		"7;file:///a%",
		"7;file:///a%00b",
		"7;file:///a%0Ab",
		"7;file:///a%7F",
		"7;file:///a\nb",
		"7;file:relative",
		"7;http:///a",   // wrong scheme
		"7;/a",          // no scheme
		"7;file:" + "x", // opaque
		"7;file:///" + strings.Repeat("d", cwdMaxPathBytes),                  // path over 1024 bytes
		"7;file:///" + strings.Repeat("d", osc7MaxPayload-len("file:///")+1), // URL of 2049 bytes
	}
	for _, in := range bad {
		if got, ok := parseOSC7([]byte(in)); ok {
			t.Errorf("parseOSC7(%q) = %q, want ignored", in, got)
		}
	}
}

func TestSameHost(t *testing.T) {
	for _, c := range []struct {
		host, own string
		want      bool
	}{
		{"mac", "mac", true},
		{"MAC", "mac.local", true},
		{"mac.local", "mac", true},
		{"mac.", "Mac", true},
		{"mac2", "mac", false},
		{"mac", "", false},
		{"", "", false},
	} {
		if got := sameHost(c.host, c.own); got != c.want {
			t.Errorf("sameHost(%q, %q) = %v, want %v", c.host, c.own, got, c.want)
		}
	}
}

// checkLogInvariants: rows strictly increase and bytes matches the directories.
func checkLogInvariants(t *testing.T, l *cwdLog) {
	t.Helper()
	sum := 0
	for i, r := range l.recs {
		sum += len(r.Dir)
		if i > 0 && r.Row <= l.recs[i-1].Row {
			t.Fatalf("rows do not increase at %d: %v", i, l.recs)
		}
	}
	if sum != l.bytes {
		t.Fatalf("bytes = %d, directories hold %d", l.bytes, sum)
	}
}

func TestCwdLog_Rules(t *testing.T) {
	var l cwdLog
	if _, ok := l.at(100); ok {
		t.Fatal("empty log found a directory")
	}

	l.add(5, "/a", 0)
	l.add(7, "/a", 0) // same directory twice: one record
	if len(l.recs) != 1 {
		t.Fatalf("same dir twice: %v", l.recs)
	}
	l.add(9, "/b", 0)
	l.add(9, "/c", 0) // same row: the later report
	if d, _ := l.at(9); d != "/c" || len(l.recs) != 2 {
		t.Fatalf("same row: %v", l.recs)
	}
	l.add(6, "/d", 0) // lower row: replaces records at or after it
	if len(l.recs) != 2 || l.recs[1] != (CwdRecord{6, "/d"}) {
		t.Fatalf("lower row: %v", l.recs)
	}
	l.add(3, "/e", 0)
	if len(l.recs) != 1 || l.recs[0] != (CwdRecord{3, "/e"}) {
		t.Fatalf("lower row below every record: %v", l.recs)
	}
	checkLogInvariants(t, &l)

	l.clear()
	l.add(5, "/a", 0)
	l.add(10, "/b", 0)
	for row, want := range map[uint64]string{5: "/a", 9: "/a", 10: "/b", 1000: "/b"} {
		if d, ok := l.at(row); !ok || d != want {
			t.Errorf("at(%d) = %q, %v; want %q", row, d, ok, want)
		}
	}
	if _, ok := l.at(4); ok {
		t.Error("at(4) found a directory before the first record")
	}

	// A lower report whose directory equals the record before it adds nothing.
	l.add(7, "/a", 0)
	if len(l.recs) != 1 || l.recs[0].Dir != "/a" {
		t.Fatalf("lower row, same dir as before: %v", l.recs)
	}

	// Pruning keeps exactly one floor record at or before firstRow.
	l.clear()
	for i, d := range []string{"/a", "/b", "/c"} {
		l.add(uint64(i+1), d, 0)
	}
	l.add(4, "/d", 3)
	if len(l.recs) != 2 || l.recs[0] != (CwdRecord{3, "/c"}) {
		t.Fatalf("prune to the floor: %v", l.recs)
	}
	l.add(5, "/e", 100) // every record is at or before firstRow: the newest, the new one, is the floor
	if len(l.recs) != 1 || l.recs[0] != (CwdRecord{5, "/e"}) {
		t.Fatalf("prune with all older: %v", l.recs)
	}
	checkLogInvariants(t, &l)
}

func TestCwdLog_Caps(t *testing.T) {
	var l cwdLog
	for i := range cwdLogMaxRecords + 76 {
		l.add(uint64(i+1), fmt.Sprintf("/d%d", i), 0)
	}
	if len(l.recs) != cwdLogMaxRecords {
		t.Fatalf("record cap: %d records, want %d", len(l.recs), cwdLogMaxRecords)
	}
	if d, ok := l.at(1 << 40); !ok || d != fmt.Sprintf("/d%d", cwdLogMaxRecords+75) {
		t.Fatalf("newest record lost: %q", d)
	}
	checkLogInvariants(t, &l)

	l.clear()
	long := "/" + strings.Repeat("x", 999)
	for i := range 100 {
		l.add(uint64(i+1), long+fmt.Sprint(i%10), 0)
	}
	if l.bytes > cwdLogMaxBytes || len(l.recs) < 60 {
		t.Fatalf("byte cap: %d bytes in %d records", l.bytes, len(l.recs))
	}
	checkLogInvariants(t, &l)
}

func TestCwd_SameReadOrdering(t *testing.T) {
	h := cwdHub(t, 80, 24)
	feedHubs(osc7("/p")+"BEFORE-CD\r\n", 4096, h)
	// One PTY read: the command echo, the chpwd report, then ls's output.
	feedHubs("% cd docs && ls\r\n"+osc7("/p/docs")+"MARK-1  MARK-2\r\n% ", 1<<20, h)
	wantCwd(t, h, "BEFORE-CD", "/p")
	wantCwd(t, h, "cd docs", "/p")
	wantCwd(t, h, "MARK-1", "/p/docs")
	wantCwd(t, h, "MARK-2", "/p/docs")

	// chpwd then precmd report the same directory: one record.
	feedHubs(osc7("/p/docs")+"MARK-3\r\n"+osc7("/p/docs"), 4096, h)
	h.emuMu.Lock()
	n := len(h.cwds.recs)
	h.emuMu.Unlock()
	if n != 2 {
		t.Fatalf("a repeated report made %d records, want 2", n)
	}
	wantCwd(t, h, "MARK-3", "/p/docs")
}

func TestCwd_SurvivesResizeAndClear(t *testing.T) {
	h := cwdHub(t, 80, 24)
	feedHubs(osc7("/a")+"MARK-A\r\n"+osc7("/b")+"MARK-B\r\n"+fillerLines(40)+
		osc7("/c")+"MARK-C\r\n"+osc7("/d")+"MARK-D\r\n", 4096, h)
	want := map[string]string{"MARK-A": "/a", "MARK-B": "/b", "MARK-C": "/c", "MARK-D": "/d"}
	check := func(stage string, markers ...string) {
		t.Helper()
		for _, m := range markers {
			if got, ok := cwdOf(t, h, m); !ok || got != want[m] {
				t.Errorf("%s: %s resolves to %q (found %v), want %q", stage, m, got, ok, want[m])
			}
		}
	}
	check("before", "MARK-A", "MARK-B", "MARK-C", "MARK-D")

	// A resize pushes the visible rows into history under the same row numbers.
	resizeHub(t, h, 60, 20)
	check("after resize", "MARK-A", "MARK-B", "MARK-C", "MARK-D")

	feedHubs(osc7("/e")+"MARK-E\r\n", 4096, h)
	want["MARK-E"] = "/e"
	check("report after resize", "MARK-D", "MARK-E")

	// ESC[3J empties history (and vt blanks the screen) but keeps the log and
	// the row numbering: output after it still resolves to the directory in
	// effect, and a new report lands on the right row.
	feedHubs("\x1b[3J"+"MARK-G\r\n", 4096, h)
	want["MARK-G"] = "/e"
	check("after ESC[3J", "MARK-G")
	feedHubs(osc7("/f")+"MARK-F\r\n", 4096, h)
	want["MARK-F"] = "/f"
	check("report after ESC[3J", "MARK-G", "MARK-F")
}

// F-1 (todo 2026-11-22-row0-relative-path-start-dir): zsh `clear` writes ED 2
// then ED 3 in one write. ED 2 pushes the screen into the emulator's
// scrollback and ED 3 drops it before it is drained into history, so those
// lines must still count in History.Pushed or output after the clear reuses
// rows that older directory records cover.
func TestCwd_ClearKeepsRowsMonotonic(t *testing.T) {
	h := cwdHub(t, 80, 24)
	feedHubs(osc7("/start")+"% cd proj\r\n"+osc7("/proj")+"% clear; ls\r\n"+
		"\x1b[H\x1b[2J\x1b[3J"+"docs-x\r\ndocs-y\r\n"+osc7("/proj")+"% ", 4096, h)
	wantCwd(t, h, "docs-x", "/proj")
	wantCwd(t, h, "docs-y", "/proj")
}

// ED 3 in the same write as lines that scrolled into the emulator's scrollback
// drops those lines too.
func TestCwd_ScrollbackEraseKeepsRows(t *testing.T) {
	h := cwdHub(t, 80, 10)
	// The report sits after the filler, on a row the dropped lines pushed
	// past: unfixed, docs-z's row falls back below it and resolves to /start.
	feedHubs(osc7("/start")+"% cd proj\r\n"+fillerLines(12)+osc7("/proj")+
		"\x1b[3J"+"docs-z\r\n"+osc7("/proj"), 4096, h)
	wantCwd(t, h, "docs-z", "/proj")
}

// ED 3 while the alternate screen is active leaves the main screen's scrollback
// in place, so it must not be counted as dropped (15-REVIEW IN-03).
func TestCwd_AltScreenScrollbackEraseKeepsRows(t *testing.T) {
	h := cwdHub(t, 80, 10)
	feedHubs(osc7("/a")+"MARK-A\r\n"+fillerLines(3)+osc7("/b")+"MARK-B\r\n"+fillerLines(12)+
		"\x1b[?1049h"+"\x1b[3J"+"\x1b[?1049l"+"AFTER\r\n", 4096, h)
	wantCwd(t, h, "MARK-A", "/a")
	wantCwd(t, h, "MARK-B", "/b")
	wantCwd(t, h, "AFTER", "/b")
}

func TestCwd_HistoryTrimKeepsFloor(t *testing.T) {
	h := cwdHub(t, 80, 24)
	h.history = NewHistory(50, HistoryMaxBytes)
	tagged := func(tag string, n int) string {
		var b strings.Builder
		for i := range n {
			fmt.Fprintf(&b, "%s-%03d\r\n", tag, i)
		}
		return b.String()
	}
	feedHubs(osc7("/a")+tagged("AAA", 200)+osc7("/b")+tagged("BBB", 200)+osc7("/c")+tagged("CCC", 5), 4096, h)
	// A report prunes against the history as of its own frame, so one more
	// frame trims the log.
	feedHubs(osc7("/d"), 4096, h)

	lines, _, first := h.history.Snapshot()
	if len(lines) != 50 || first == 0 {
		t.Fatalf("fixture: %d lines retained from row %d, want 50 and a trimmed history", len(lines), first)
	}
	h.emuMu.Lock()
	defer h.emuMu.Unlock()
	if d, ok := h.cwds.at(first); !ok || d != "/b" {
		t.Errorf("oldest retained line resolves to %q (found %v), want /b", d, ok)
	}
	if len(h.cwds.recs) != 3 || h.cwds.recs[0].Dir != "/b" {
		t.Errorf("log after trimming: %v, want the /b floor, /c, /d", h.cwds.recs)
	}
	if row, ok := cwdRowOfLocked(h, "CCC-004"); !ok {
		t.Error("CCC-004 not found")
	} else if d, _ := h.cwds.at(row); d != "/c" {
		t.Errorf("CCC-004 resolves to %q, want /c", d)
	}
}

func TestCwd_AltScreenIgnored(t *testing.T) {
	h := cwdHub(t, 80, 24)
	feedHubs(osc7("/a")+"MARK-A\r\n\x1b[?1049h"+osc7("/alt")+"ALT-X\x1b[?1049l"+"MARK-B\r\n", 4096, h)
	wantCwd(t, h, "MARK-A", "/a")
	wantCwd(t, h, "MARK-B", "/a")

	feedHubs(osc7("/b")+"MARK-C\r\n", 4096, h) // reports count again after the exit
	wantCwd(t, h, "MARK-C", "/b")
	h.emuMu.Lock()
	defer h.emuMu.Unlock()
	for _, r := range h.cwds.recs {
		if r.Dir == "/alt" {
			t.Fatalf("an alt-screen report was recorded: %v", h.cwds.recs)
		}
	}
}

func TestCwd_RISClears(t *testing.T) {
	h := cwdHub(t, 80, 24)
	feedHubs(osc7("/a")+"MARK-A\r\n", 4096, h)
	feedHubs("\x1bc"+"MARK-B\r\n", 4096, h)
	wantNoCwd(t, h, "MARK-B")
	feedHubs(osc7("/z")+"MARK-Z\r\n", 4096, h)
	wantCwd(t, h, "MARK-Z", "/z")
	wantNoCwd(t, h, "MARK-B")
}
