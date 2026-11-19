package relay

import (
	"bytes"
	"fmt"
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
