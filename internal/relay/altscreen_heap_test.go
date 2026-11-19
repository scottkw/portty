package relay

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"runtime"
	"strings"
	"testing"
	"unsafe"

	uv "github.com/charmbracelet/ultraviolet"
)

// altChunks returns n full-width TUI lines tagged ALTX-<from..>, each exactly
// cols non-blank characters plus CRLF, in PTY-sized chunks of at most 32 KiB.
func altChunks(cols, from, n int) []string {
	var out []string
	var b strings.Builder
	for i := from; i < from+n; i++ {
		tag := fmt.Sprintf("ALTX-%06d", i)
		b.WriteString(tag + strings.Repeat("a", cols-len(tag)) + "\r\n")
		if b.Len() > 32<<10-(cols+2) {
			out = append(out, b.String())
			b.Reset()
		}
	}
	if b.Len() > 0 {
		out = append(out, b.String())
	}
	return out
}

func heapAlloc() int64 {
	var m runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m)
	return int64(m.HeapAlloc)
}

// TestHistory_HeapBoundAfterAltScreenScroll: a TUI scrolling 40,000 full-width
// lines in the alternate screen grows the hub's heap by less than one fixed
// 32 MiB bound at 80 and at 220 columns, while still in the alt screen and
// after it exits: the alt screen keeps no scrollback (DEBT-01, vtalt.go).
func TestHistory_HeapBoundAfterAltScreenScroll(t *testing.T) {
	const bound = 32 << 20
	for _, cols := range []int{80, 220} {
		t.Run(fmt.Sprintf("cols=%d", cols), func(t *testing.T) {
			hub := NewHub("heap-alt", nil, nil, DefaultScrollbackBytes, nil)
			resizeHub(t, hub, cols, 24)
			feed := func(chunks ...string) {
				hub.emuMu.Lock()
				defer hub.emuMu.Unlock()
				for _, c := range chunks {
					hub.recordFrameLocked(MakeOutputFrame([]byte(c)), []byte(c))
				}
			}

			var main strings.Builder
			for i := 1; i <= 200; i++ {
				fmt.Fprintf(&main, "LINE-%06d %s\r\n", i, strings.Repeat("x", 16))
			}
			feed(main.String())

			m0 := heapAlloc()
			feed("\x1b[?1049h")
			feed(altChunks(cols, 0, 20000)...)
			m1 := heapAlloc()
			feed(altChunks(cols, 20000, 20000)...)
			m2 := heapAlloc()
			hub.emuMu.Lock()
			onScreen := hub.liveEmu.String()
			hub.emuMu.Unlock()
			feed("\x1b[?1049l")
			m3 := heapAlloc()

			burst1, burst2, after := m1-m0, m2-m0, m3-m0
			t.Logf("cols=%d heap growth: burst 1 %.1f MiB, burst 2 %.1f MiB, after exit %.1f MiB (bound %.0f MiB)",
				cols, mib(burst1), mib(burst2), mib(after), mib(bound))
			for _, g := range []struct {
				when string
				n    int64
			}{{"after burst 1", burst1}, {"after burst 2", burst2}, {"after the exit", after}} {
				if g.n >= bound {
					t.Errorf("heap grew %.1f MiB %s, want under %.0f MiB", mib(g.n), g.when, mib(bound))
				}
			}
			if !strings.Contains(onScreen, "ALTX-039999") {
				t.Error("fixture invalid: the alt screen does not show the last alt line")
			}

			hub.emuMu.Lock()
			defer hub.emuMu.Unlock()
			if hub.liveEmu.IsAltScreen() {
				t.Error("emulator still in the alt screen after ESC[?1049l")
			}
			if n := hub.history.Len(); n < 170 {
				t.Errorf("history holds %d lines, want the main lines that scrolled off (about 177)", n)
			}
		})
	}
}

// TestAltScreenScrollback_ControlWithoutBound keeps the soak honest: without
// disableAltScreenScrollback, the same fixture fills vt's alt scrollback.
func TestAltScreenScrollback_ControlWithoutBound(t *testing.T) {
	const cols, n = 80, 3000
	emu := NewQuietEmulator(cols, 24)
	_, _ = emu.WriteString("\x1b[?1049h")
	m0 := heapAlloc()
	for _, c := range altChunks(cols, 0, n) {
		_, _ = emu.WriteString(c)
	}
	grew := heapAlloc() - m0
	want := int64(n) * cols * int64(unsafe.Sizeof(uv.Cell{})) / 2
	t.Logf("unbounded alt scrollback: %.1f MiB for %d lines", mib(grew), n)
	if grew < want {
		t.Errorf("heap grew %.1f MiB, want at least %.1f MiB: the fixture does not fill the alt scrollback", mib(grew), mib(want))
	}
	runtime.KeepAlive(emu)
}

// TestAltScreen_JoinWhileInAltKeepsMainHistory: a viewer who joins while the
// app is still in the alt screen gets the pre-app main history once, in order,
// with no alt line, then the alt screen.
func TestAltScreen_JoinWhileInAltKeepsMainHistory(t *testing.T) {
	hub, w := historyHub(t)
	emitLines(t, hub, w, 1, 200)
	emit(t, hub, w, "\x1b[?1049h")
	for _, c := range altChunks(80, 0, 5000) {
		emit(t, hub, w, c)
	}

	replay := fullReplay(t, hub)
	i := bytes.Index(replay, []byte(altScreenEnterSeq))
	if i < 0 {
		t.Fatalf("replay has no %q prefix while the app is in the alt screen", altScreenEnterSeq)
	}
	if !bytes.Contains(replay[i:], []byte("ALTX-004999")) {
		t.Error("replay's alt screen lacks the last alt line")
	}
	hist := replay[:i]
	if bytes.Contains(hist, []byte("ALTX-")) {
		t.Error("replay history carries alternate-screen lines")
	}
	tags := lineTags(hist)
	if len(tags) != 177 {
		t.Fatalf("replay history has %d LINE tags, want 177 (1..177 once each)", len(tags))
	}
	for k, n := range tags {
		if n != k+1 {
			t.Fatalf("history LINE tag %d is %d, want %d (missing, repeated or out of order)", k, n, k+1)
		}
	}
}

func mib(b int64) float64 { return float64(b) / (1 << 20) }

// TestAltScreenExit_KeepsMainScreenAndHistory: the alt-exit rebuild keeps the
// pre-app main history once, in order, with no alt line, and the main screen
// and cursor exactly as the host shows them.
func TestAltScreenExit_KeepsMainScreenAndHistory(t *testing.T) {
	hub, w := historyHub(t)
	emitLines(t, hub, w, 1, 200)
	emit(t, hub, w, "$ vim\r\n")
	cup := cursorCUP(t, hub)
	hub.emuMu.Lock()
	screen := hub.liveEmu.String()
	hub.emuMu.Unlock()

	emit(t, hub, w, "\x1b[?1049h")
	for _, c := range altChunks(80, 0, 20000) {
		emit(t, hub, w, c)
	}
	emit(t, hub, w, "\x1b[?1049l")

	replay := fullReplay(t, hub)
	tags := lineTags(replay)
	if len(tags) != 200 {
		t.Fatalf("replay has %d LINE tags, want 200 (1..200 once each)", len(tags))
	}
	for i, n := range tags {
		if n != i+1 {
			t.Fatalf("replay LINE tag %d is %d, want %d (missing, repeated or out of order)", i, n, i+1)
		}
	}
	if bytes.Contains(replay, []byte("ALTX-")) {
		t.Error("replay carries alternate-screen lines")
	}
	if got := cursorCUP(t, hub); got != cup {
		t.Errorf("cursor %q after the app exited, want %q as before it started", got, cup)
	}
	hub.emuMu.Lock()
	after := hub.liveEmu.String()
	hub.emuMu.Unlock()
	if after != screen || !strings.Contains(after, "LINE-000200") || !strings.Contains(after, "$ vim") {
		t.Errorf("screen after the app exited:\n%s\nwant as before it started:\n%s", after, screen)
	}
}

// TestAltScreenExit_RebuildMatchesReference: after the alt-exit rebuild, later
// output lands as it does in an emulator fed every byte, including a read cut
// mid-escape or mid-UTF-8, a pen, a scroll region, a saved cursor and DEC line
// drawing (WR-02). Each step is one PTY read; tab stops are a documented loss.
func TestAltScreenExit_RebuildMatchesReference(t *testing.T) {
	const app = "\x1b[?1049hALT\x1b[?1049l"
	const app47 = "\x1b[?1047hALT\x1b[?1047l"
	cases := []struct {
		name  string
		steps []string
	}{
		{"read ends mid-CSI after the exit", []string{"$ vim\r\n" + app + "$ \x1b[3", "2mgreen"}},
		{"read ends mid-UTF-8 after the exit", []string{"$ vim\r\n" + app + "$ \xe2\x94", "\x80ok"}},
		{"read ends inside an OSC after the exit", []string{"$ vim\r\n" + app + "\x1b]0;ti", "tle\x07$ ok"}},
		{"pen left red", []string{"$ \x1b[31m", app, "after"}},
		{"scroll region kept by the shell", []string{"\x1b[1;4r$ vim\r\n", app, "\r\nA\r\nB\r\nC\r\nD\r\nE\r\n"}},
		{"saved cursor", []string{"ab\x1b7\r\n\r\n", app, "\x1b8Z"}},
		{"DEC line drawing in G0", []string{"\x1b(0", app, "qqq"}},
		// WR-05: the pen/charset probe must land whatever tail the read left.
		{"pen red, read ends on ESC", []string{"\x1b[1;31m$ vim\r\n" + app + "$ \x1b", "[mafter"}},
		{"pen red, read ends on ESC P", []string{"\x1b[1;31m$ vim\r\n" + app + "$ \x1bP", "zz\x1b\\after"}},
		{"pen red, read ends on ESC P +", []string{"\x1b[1;31m$ vim\r\n" + app + "$ \x1bP+", "zz\x1b\\after"}},
		{"pen red, read ends on C1 DCS", []string{"\x1b[1;31m$ vim\r\n" + app + "$ \x90", "zz\x1b\\after"}},
		{"pen red, read ends inside APC", []string{"\x1b[1;31m$ vim\r\n" + app + "$ \x1b_x", "y\x1b\\after"}},
		{"pen red, read ends on C1 CSI", []string{"\x1b[1;31m$ vim\r\n" + app + "$ \x9b3", "2mafter"}},
		{"line drawing, read ends mid-UTF-8", []string{"\x1b(0\x1b[1;31m$ vim\r\n" + app + "$ \xe2\x94", "\x80qqq"}},
		// WR-06: saved cursor pen, origin mode, pending wrap, ?1048. ?1047
		// saves no cursor, so the shell's own DECSC is what ESC 8 restores.
		{"saved cursor keeps its pen", []string{"\x1b[32mab\x1b7\x1b[m\r\n", app47, "\x1b8Z"}},
		{"origin mode with a region", []string{"\x1b[2;5r\x1b[?6h\x1b[1;1Hab\x1b7\r\n\r\n", app47, "\x1b8Z\r\nY"}},
		{"pending wrap at the read's end", []string{app + strings.Repeat("a", 40), "b"}},
		{"pending wrap left by the app", []string{"\x1b[?1049h\x1b[1;40Hx\x1b[?1049l", "b"}},
		{"pending wrap, read ends mid-UTF-8", []string{app + strings.Repeat("a", 40) + "\xe2\x94", "\x80b"}},
		{"last column without a wrap, read ends mid-UTF-8", []string{app + strings.Repeat("a", 39) + "\xc3", "\xa9b"}},
		{"?1048 reset not replayed as a restore", []string{"\x1b[?1048h\x1b[?1048lab\r\ncd", app47, "Z"}},
		// IN-10: a C0 inside a cut-off ESC/CSI already ran on the old screen.
		// (vt also ORs an executed C0 into the CSI's command, so LF turns m
		// into o; xterm.js does not, and the rebuild cannot copy it.)
		{"read ends on ESC LF", []string{"$ " + app + "\x1b\n", "[mok"}},
		{"read ends mid-CSI after a BS", []string{"$ " + app + "\x1b[3\b", "1mok"}},
		{"read ends mid-CSI after a CR", []string{"$ " + app + "\x1b[3\r", "1mok"}},
		{"read ends on C1 CSI after a BS", []string{"$ " + app + "\x9b3\b", "1mok"}},
	}
	// WR-09: vt takes a CAN as a byte of an open UTF-8 character, so check
	// every cut shape (bytes read of the character's length), on column 1
	// without a wrap and in pending wrap.
	for _, ch := range []string{"\xc3\xa9", "\xe2\x94\x80", "\xe6\x97\xa5", "\xf0\x9f\x98\x80"} {
		for cut := 1; cut < len(ch); cut++ {
			for where, pre := range map[string]string{"column 1": "a" + app, "pending wrap": app + strings.Repeat("a", 40)} {
				cases = append(cases, struct {
					name  string
					steps []string
				}{fmt.Sprintf("%s, read ends %d of %d bytes into %q", where, cut, len(ch), ch), []string{pre + ch[:cut], ch[cut:] + "b"}})
			}
		}
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hub := NewHub("rebuild-ref", nil, nil, DefaultScrollbackBytes, nil)
			resizeHub(t, hub, 40, 6)
			ref := NewQuietEmulator(40, 6)
			hub.emuMu.Lock()
			defer hub.emuMu.Unlock()
			hub.ensureLiveEmulatorLocked()
			first := hub.liveEmu
			for _, s := range c.steps {
				hub.recordFrameLocked(MakeOutputFrame([]byte(s)), []byte(s))
				ref.Write([]byte(s)) //nolint:errcheck
			}
			if hub.liveEmu == first {
				t.Fatal("fixture invalid: the app's exit did not rebuild the emulator")
			}
			got, want := hub.liveEmu.Render(), ref.Render()
			if got != want || hub.liveEmu.CursorPosition() != ref.CursorPosition() {
				t.Errorf("rebuilt %q cursor %v, want %q cursor %v", got, hub.liveEmu.CursorPosition(), want, ref.CursorPosition())
			}
		})
	}
}

func TestIncompleteTail(t *testing.T) {
	for in, want := range map[string]string{
		"plain":                    "",
		"$ \x1b[3":                 "\x1b[3",
		"x\xe2\x94":                "\xe2\x94",
		"\x1b]0;t\x07ok":           "",
		"\x1b]0;t\x1b\\ok":         "",
		"a\x1b]0;title":            "\x1b]0;title",
		"a\x1bPq#0~":               "\x1bPq#0~",
		"a\x9b6":                   "\x9b6",
		"a\x1b(":                   "\x1b(",
		"\x1b[3\x18ok":             "",
		"\x1b]0;a\x1b[1":           "\x1b]0;a\x1b[1", // no ground point since the OSC: replay both
		"\x1b[?1049l\x1b[1;32mok ": "",
	} {
		if got := string(incompleteTail([]byte(in))); got != want {
			t.Errorf("incompleteTail(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestAltScreenResize_KeepsMainScreenAndExitRebuild: a host resize while an
// app is in the alternate screen keeps the main screen's lines (in history),
// keeps the app's redraw out of history, and the app's later exit still
// leaves the alt screen and rebuilds the emulator (WR-03, HARD-01).
func TestAltScreenResize_KeepsMainScreenAndExitRebuild(t *testing.T) {
	hub, w := historyHub(t)
	emitLines(t, hub, w, 1, 10)
	emit(t, hub, w, "$ less\r\n")
	emit(t, hub, w, "\x1b[?1049h")
	for _, c := range altChunks(80, 0, 100) {
		emit(t, hub, w, c)
	}

	resizeHub(t, hub, 100, 30)
	hub.emuMu.Lock()
	inAlt := hub.liveEmu.IsAltScreen()
	hub.emuMu.Unlock()
	if !inAlt {
		t.Fatal("emulator left the alt screen on a resize while the app is still up")
	}
	for _, c := range altChunks(80, 100, 100) { // the app's SIGWINCH redraw, scrolling
		emit(t, hub, w, c)
	}
	hub.emuMu.Lock()
	before := hub.liveEmu
	hub.emuMu.Unlock()
	emit(t, hub, w, "\x1b[?1049l$ ")

	hub.emuMu.Lock()
	rebuilt, stillAlt := hub.liveEmu != before, hub.liveEmu.IsAltScreen()
	hub.emuMu.Unlock()
	if stillAlt {
		t.Fatal("emulator still in the alt screen after the app's ESC[?1049l")
	}
	if !rebuilt {
		t.Error("the app's exit did not rebuild the emulator (HARD-01)")
	}
	replay := fullReplay(t, hub)
	if bytes.Contains(replay, []byte("ALTX-")) {
		t.Error("replay carries alternate-screen lines")
	}
	if got := lineTags(replay); len(got) != 10 || got[0] != 1 || got[9] != 10 {
		t.Errorf("replay LINE tags %v, want 1..10 (the main screen behind the app)", got)
	}
	if !bytes.Contains(replay, []byte("$ less")) {
		t.Error("replay lost the main-screen line \"$ less\"")
	}
}

// rebuildFuzzStream returns one random shell-output stream around an app's
// alt-screen visit, cut into random reads. It avoids the rebuild's
// documented losses: tab stops, SO/SI and G1-G3, OSC 8 links, REP.
func rebuildFuzzStream(r *rand.Rand, cols, rows int) []string {
	main := func(n int) string {
		var b strings.Builder
		for range n {
			switch r.IntN(20) {
			case 0, 1, 2:
				b.WriteString(strings.Repeat(string(rune('a'+r.IntN(26))), 1+r.IntN(cols+cols/2)))
			case 3, 4:
				b.WriteString("\r\n")
			case 5:
				fmt.Fprintf(&b, "\x1b[%dm", []int{0, 1, 4, 7, 31, 32, 33, 34, 42}[r.IntN(9)])
			case 6:
				fmt.Fprintf(&b, "\x1b[1;3%dm", r.IntN(8))
			case 7:
				b.WriteString([]string{"\x1b7", "\x1b8", "\x1b[s", "\x1b[u"}[r.IntN(4)])
			case 8:
				if r.IntN(3) == 0 {
					b.WriteString("\x1b[r")
				} else {
					top := 1 + r.IntN(rows-1)
					fmt.Fprintf(&b, "\x1b[%d;%dr", top, top+1+r.IntN(rows-top))
				}
			case 9:
				b.WriteString([]string{"\x1b[?6h", "\x1b[?6l"}[r.IntN(2)])
			case 10:
				fmt.Fprintf(&b, "\x1b[%d;%dH", 1+r.IntN(rows), 1+r.IntN(cols))
			case 11:
				b.WriteString([]string{"\x1b(0", "\x1b(B"}[r.IntN(2)])
			case 12:
				b.WriteString([]string{"qqx", "\xe2\x94\x80", "\xc3\xa9", "\xe2\x9d\xaf", "\xe6\x97\xa5", "\xf0\x9f\x98\x80"}[r.IntN(6)]) // WR-09: 2-, 3- and 4-byte, wide
			case 13:
				b.WriteString([]string{"\x1b[?25l", "\x1b[?25h", "\x1b[?7l", "\x1b[?7h"}[r.IntN(4)])
			case 14:
				b.WriteString([]string{"\x1b]0;title\x07", "\x1bPzz\x1b\\", "\x1b[K", "\x1b[2J"}[r.IntN(4)])
			default:
				b.WriteString("$ ")
			}
		}
		return b.String()
	}
	apps := [][2]string{{"\x1b[?1049h", "\x1b[?1049l"}, {"\x1b[?1047h", "\x1b[?1047l"}, {"\x1b[?1048h\x1b[?1047h", "\x1b[?1047l\x1b[?1048l"}}
	app := apps[r.IntN(len(apps))]
	s := main(30+r.IntN(40)) + app[0] + main(10+r.IntN(20)) + app[1] + main(20+r.IntN(40))
	var reads []string
	for len(s) > 0 {
		n := min(len(s), 1+r.IntN(24))
		reads = append(reads, s[:n])
		s = s[n:]
	}
	return reads
}

// TestAltScreenExit_RebuildFuzz: random streams around an alt-screen visit,
// cut into random reads, leave the rebuilt emulator matching one fed every
// byte, read for read (WR-05, WR-06). Fixed seeds keep it deterministic.
func TestAltScreenExit_RebuildFuzz(t *testing.T) {
	const cols, rows, streams = 20, 6, 600
	r := rand.New(rand.NewPCG(7, 2026))
	mismatches := 0
	for i := range streams {
		reads := rebuildFuzzStream(r, cols, rows)
		hub := NewHub("rebuild-fuzz", nil, nil, DefaultScrollbackBytes, nil)
		resizeHub(t, hub, cols, rows)
		ref := NewQuietEmulator(cols, rows)
		hub.emuMu.Lock()
		hub.ensureLiveEmulatorLocked()
		for j, s := range reads {
			hub.recordFrameLocked(MakeOutputFrame([]byte(s)), []byte(s))
			ref.Write([]byte(s)) //nolint:errcheck
			got, want := hub.liveEmu.Render(), ref.Render()
			if gp, wp := hub.liveEmu.CursorPosition(), ref.CursorPosition(); got != want || gp != wp {
				if mismatches < 5 {
					t.Errorf("stream %d read %d: rebuilt %q cursor %v, want %q cursor %v\nreads so far: %q",
						i, j, got, gp, want, wp, reads[:j+1])
				}
				mismatches++
				break
			}
		}
		hub.emuMu.Unlock()
	}
	if mismatches > 0 {
		t.Errorf("%d of %d streams diverged from the reference", mismatches, streams)
	}
}
