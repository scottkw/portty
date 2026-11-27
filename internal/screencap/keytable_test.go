package screencap

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestKeytable_KnownCodes(t *testing.T) {
	for _, c := range []struct {
		code string
		vk   uint16
	}{
		{"KeyA", 0x00}, {"Backspace", 0x33}, {"Delete", 0x75}, {"IntlBackslash", 0x0A},
		{"Lang1", 0x68}, {"Enter", 0x24}, {"F20", 0x5A}, {"NumLock", 0x47}, {"Insert", 0x72},
	} {
		if vk, ok := KeyCode(c.code); !ok || vk != c.vk {
			t.Errorf("KeyCode(%q) = %#x, %v; want %#x, true", c.code, vk, ok, c.vk)
		}
	}
	for _, code := range []string{"Fn", "MediaPlayPause", "", "keya", "Help"} {
		if vk, ok := KeyCode(code); ok || vk != 0 {
			t.Errorf("KeyCode(%q) = %#x, %v; want 0, false", code, vk, ok)
		}
	}
}

func TestKeytable_UniqueAndNamed(t *testing.T) {
	seen := map[uint16]string{}
	for code, e := range keyTable {
		if !strings.HasPrefix(e.name, "kVK_") || KeyName(code) != e.name {
			t.Errorf("%s: name %q, want a kVK_ name", code, e.name)
		}
		if prev, dup := seen[e.vk]; dup {
			t.Errorf("vk %#x is both %s and %s", e.vk, prev, code)
		}
		seen[e.vk] = code
	}
}

// TestKeytable_MatchesEventsH checks every entry against the macOS SDK header:
// a wrong kVK types the wrong key on every layout.
func TestKeytable_MatchesEventsH(t *testing.T) {
	out, err := exec.Command("xcrun", "--sdk", "macosx", "--show-sdk-path").Output()
	if err != nil {
		t.Skipf("xcrun unavailable: %v", err)
	}
	hdr := filepath.Join(strings.TrimSpace(string(out)), "System/Library/Frameworks/Carbon.framework/Frameworks/HIToolbox.framework/Headers/Events.h")
	b, err := os.ReadFile(hdr)
	if err != nil {
		t.Skipf("Events.h not readable: %v", err)
	}
	want := map[string]uint16{}
	for _, m := range regexp.MustCompile(`(kVK_\w+)\s*=\s*0x([0-9A-Fa-f]+)`).FindAllStringSubmatch(string(b), -1) {
		n, _ := strconv.ParseUint(m[2], 16, 16)
		want[m[1]] = uint16(n)
	}
	if len(want) < 100 {
		t.Fatalf("parsed only %d kVK constants from the header", len(want))
	}
	for code, e := range keyTable {
		got, ok := want[e.name]
		if !ok {
			t.Errorf("%s: %s is not in Events.h", code, e.name)
		} else if got != e.vk {
			t.Errorf("%s: %s = %#x in the table, %#x in Events.h", code, e.name, e.vk, got)
		}
	}
}

func TestKeytable_Modifiers(t *testing.T) {
	want := map[string]uint64{
		"ShiftLeft": FlagShift, "ShiftRight": FlagShift, "ControlLeft": FlagControl, "ControlRight": FlagControl,
		"AltLeft": FlagOption, "AltRight": FlagOption, "MetaLeft": FlagCommand, "MetaRight": FlagCommand,
		"CapsLock": FlagAlphaShift,
	}
	for code, flag := range want {
		if !IsModifierCode(code) || ModifierFlag(code) != flag {
			t.Errorf("%s: IsModifierCode %v, ModifierFlag %#x, want true, %#x", code, IsModifierCode(code), ModifierFlag(code), flag)
		}
	}
	if IsModifierCode("KeyA") || ModifierFlag("KeyA") != 0 {
		t.Error("KeyA is not a modifier")
	}
}

func TestUTF16Chunks(t *testing.T) {
	if got := UTF16Chunks("", 20); len(got) != 0 {
		t.Errorf("empty string gave %v", got)
	}
	lens := func(cs [][]uint16) []int {
		var out []int
		for _, c := range cs {
			out = append(out, len(c))
		}
		return out
	}
	for _, c := range []struct {
		name, s string
		want    []int
	}{
		{"ascii", strings.Repeat("a", 45), []int{20, 20, 5}},
		{"pair fits", strings.Repeat("a", 15) + "\U0001F600b", []int{18}},
		{"pair would straddle", strings.Repeat("a", 19) + "\U0001F600", []int{19, 2}},
		{"zwj sequence", "\U0001F469\u200d\U0001F4BB", []int{5}},
		{"zwj sequence moves whole", strings.Repeat("a", 17) + "\U0001F469\u200d\U0001F4BB", []int{17, 5}},
		{"combining mark", "é", []int{2}},
		{"mark moves with base", strings.Repeat("a", 19) + "é", []int{19, 2}},
		{"variation selector", strings.Repeat("a", 19) + "❤️", []int{19, 2}},
		{"one run longer than a chunk", strings.Repeat("\U0001F600\u200d", 9) + "x", nil},
	} {
		got := UTF16Chunks(c.s, 20)
		if c.want != nil && !slices.Equal(lens(got), c.want) {
			t.Errorf("%s: chunk lengths %v, want %v", c.name, lens(got), c.want)
		}
		var all []uint16
		for _, ch := range got {
			if len(ch) > 20 {
				t.Errorf("%s: a chunk of %d units", c.name, len(ch))
			}
			if last := ch[len(ch)-1]; last >= 0xD800 && last < 0xDC00 {
				t.Errorf("%s: a chunk ends on a lone high surrogate", c.name)
			}
			all = append(all, ch...)
		}
		if !slices.Equal(all, utf16.Encode([]rune(c.s))) {
			t.Errorf("%s: chunks do not concatenate back to the input", c.name)
		}
	}
}
