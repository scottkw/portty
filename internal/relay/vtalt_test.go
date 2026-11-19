package relay

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	xvt "github.com/charmbracelet/x/vt"
)

// pinnedVT is the vt version vtalt.go was reviewed against (DEBT-01).
const pinnedVT = "v0.0.0-20260615092313-b57e5e6d29bb"

// TestVTLayout_AltScreenScrollback pins the vt version and the Emulator
// layout disableAltScreenScrollback relies on, and checks that the disabled
// alt scrollback stays disabled through RIS and Resize while the main one is
// untouched.
func TestVTLayout_AltScreenScrollback(t *testing.T) {
	mod, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	found := "(not found)"
	for _, l := range strings.Split(string(mod), "\n") {
		if f := strings.Fields(l); len(f) >= 2 && f[0] == "github.com/charmbracelet/x/vt" {
			found = f[1]
		}
	}
	if found != pinnedVT {
		t.Fatalf("vt changed from %s to %s: re-review internal/relay/vtalt.go against the new vt source "+
			"(is the alt Screen still scrs[1]; are DeleteLine, ClearWithScrollback and the ED 3 handler still nil-safe; "+
			"do Screen.Reset, Resize or fullReset recreate the scrollback), then update the pinned version here", pinnedVT, found)
	}

	f, ok := reflect.TypeOf(xvt.Emulator{}).FieldByName("scrs")
	if !ok || f.Type != reflect.TypeOf([2]xvt.Screen{}) {
		t.Fatalf("vt Emulator field scrs is %v (present %v), want [2]vt.Screen: re-review internal/relay/vtalt.go", f.Type, ok)
	}

	emu := NewQuietEmulator(80, 24)
	alt := altScreenOf(emu)
	if alt == nil || alt.Scrollback() == nil {
		t.Fatal("fresh emulator: alt screen or its scrollback not found")
	}
	main := emu.Scrollback()
	if !disableAltScreenScrollback(emu) {
		t.Fatal("disableAltScreenScrollback returned false")
	}
	if altScreenOf(emu).Scrollback() != nil {
		t.Error("alt scrollback still set after disableAltScreenScrollback")
	}
	if emu.Scrollback() == nil || emu.Scrollback() != main {
		t.Error("main scrollback changed by disableAltScreenScrollback")
	}
	_, _ = emu.WriteString("\x1bc")
	if altScreenOf(emu).Scrollback() != nil {
		t.Error("RIS re-created the alt scrollback")
	}
	emu.Resize(100, 30)
	if altScreenOf(emu).Scrollback() != nil {
		t.Error("Resize re-created the alt scrollback")
	}
}

// TestDisableAltScreenScrollback_KeepsMainHistory: with the alt scrollback
// off, the main screen still keeps the lines that scrolled off it, and no alt
// line reaches it.
func TestDisableAltScreenScrollback_KeepsMainHistory(t *testing.T) {
	emu := NewQuietEmulator(80, 24)
	if !disableAltScreenScrollback(emu) {
		t.Fatal("disableAltScreenScrollback returned false")
	}
	emu.SetScrollbackSize(HistoryMaxLines)
	var b strings.Builder
	for i := 1; i <= 50; i++ {
		fmt.Fprintf(&b, "LINE-%06d\r\n", i)
	}
	_, _ = emu.WriteString(b.String())
	before := emu.ScrollbackLen() // 27: LINE-1..27 scrolled off
	if before == 0 {
		t.Fatal("fixture invalid: no main line scrolled off")
	}
	_, _ = emu.WriteString("\x1b[?1049h")
	for _, c := range altChunks(80, 0, 500) {
		_, _ = emu.WriteString(c)
	}
	_, _ = emu.WriteString("\x1b[?1049l")
	if n := emu.ScrollbackLen(); n != before {
		t.Errorf("main scrollback holds %d lines after the alt visit, want %d as before it", n, before)
	}
	for _, l := range emu.Scrollback().Lines() {
		if strings.Contains(l.String(), "ALTX-") {
			t.Fatalf("main scrollback holds an alt line: %q", l.String())
		}
	}
}
