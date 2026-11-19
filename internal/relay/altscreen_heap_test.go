package relay

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"
	"testing"
	"unsafe"

	uv "github.com/charmbracelet/ultraviolet"
	xvt "github.com/charmbracelet/x/vt"
)

// altChunks returns n full-width TUI lines tagged ALTX-<from..>, each exactly
// 80 non-blank characters plus CRLF, in PTY-sized chunks of at most 32 KiB.
func altChunks(from, n int) []string {
	var out []string
	var b strings.Builder
	for i := from; i < from+n; i++ {
		tag := fmt.Sprintf("ALTX-%06d", i)
		b.WriteString(tag + strings.Repeat("a", 80-len(tag)) + "\r\n")
		if b.Len() > 32<<10-82 {
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

// TestHistory_HeapBoundAfterAltScreenScroll: a TUI scrolling 20,000+ lines in
// the alternate screen holds at most vt's 10,000-line alt scrollback (which no
// vt API can shrink), plateaus there, and frees it once the app exits (HARD-01).
func TestHistory_HeapBoundAfterAltScreenScroll(t *testing.T) {
	const cols = 80
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
	feed(altChunks(0, 20000)...)
	m1 := heapAlloc()
	feed(altChunks(20000, 20000)...)
	m2 := heapAlloc()
	feed("\x1b[?1049l")
	m3 := heapAlloc()

	capBytes := int64(xvt.DefaultScrollbackSize) * cols * int64(unsafe.Sizeof(uv.Cell{}))
	ceiling := capBytes + 16<<20
	if raceBuild() {
		// -race disables the tiny allocator, so each cell's 1-byte content
		// string takes its own 16-byte block (measured +9.2 MiB at the cap).
		ceiling += int64(xvt.DefaultScrollbackSize) * cols * 16
	}
	burst1, burst2, after := m1-m0, m2-m1, m3-m0
	t.Logf("heap growth: burst 1 %.1f MiB, burst 2 %.1f MiB, after exit %.1f MiB; in-alt ceiling %.1f MiB",
		mib(burst1), mib(burst2), mib(after), mib(ceiling))

	if burst1 > ceiling {
		t.Errorf("in-alt growth %.1f MiB exceeds the vt-derived ceiling %.1f MiB", mib(burst1), mib(ceiling))
	}
	if burst1 < capBytes/2 {
		t.Errorf("in-alt growth %.1f MiB is below half the cap's cell bytes (%.1f MiB): the fixture did not fill the alt scrollback",
			mib(burst1), mib(capBytes/2))
	}
	if burst2 >= burst1/4 {
		t.Errorf("second burst grew the heap %.1f MiB, want under a quarter of the first (%.1f MiB): no plateau",
			mib(burst2), mib(burst1/4))
	}
	if after >= 32<<20 {
		t.Errorf("heap grew %.1f MiB once the app left the alt screen, want under 32 MiB", mib(after))
	}

	hub.emuMu.Lock()
	defer hub.emuMu.Unlock()
	if hub.liveEmu.IsAltScreen() {
		t.Error("emulator still in the alt screen after ESC[?1049l")
	}
	if n := hub.history.Len(); n < 170 {
		t.Errorf("history holds %d lines, want the main lines that scrolled off (about 177)", n)
	}
}

func mib(b int64) float64 { return float64(b) / (1 << 20) }

func raceBuild() bool {
	bi, _ := debug.ReadBuildInfo()
	return bi != nil && slices.Contains(bi.Settings, debug.BuildSetting{Key: "-race", Value: "true"})
}

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
	for _, c := range altChunks(0, 20000) {
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
	for _, c := range altChunks(0, 100) {
		emit(t, hub, w, c)
	}

	resizeHub(t, hub, 100, 30)
	hub.emuMu.Lock()
	inAlt := hub.liveEmu.IsAltScreen()
	hub.emuMu.Unlock()
	if !inAlt {
		t.Fatal("emulator left the alt screen on a resize while the app is still up")
	}
	for _, c := range altChunks(100, 100) { // the app's SIGWINCH redraw, scrolling
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
