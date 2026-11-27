package screencap

// keytable.go: the closed table from a browser's KeyboardEvent.code to a macOS
// virtual key code (kVK), and the text chunker for composed text. Pure Go, so
// it is tested everywhere; the table is checked against the SDK's Events.h by
// TestKeytable_MatchesEventsH, because a wrong kVK types the wrong key on every
// layout. A code with no entry is dropped, never guessed (Fn/Globe and the media
// keys cannot be sent from a browser). "Help" has no entry of its own: Insert
// is the key that lands on kVK_Help, and a vk must appear once.

import (
	"unicode"
	"unicode/utf16"
)

// CGEventFlags modifier masks, for the explicit flags of every posted key.
const (
	FlagAlphaShift = 0x10000
	FlagShift      = 0x20000
	FlagControl    = 0x40000
	FlagOption     = 0x80000
	FlagCommand    = 0x100000
)

type keyEntry struct {
	name string // the Events.h constant, kVK_...
	vk   uint16
}

var keyTable = map[string]keyEntry{
	"KeyA": {"kVK_ANSI_A", 0x00}, "KeyB": {"kVK_ANSI_B", 0x0B}, "KeyC": {"kVK_ANSI_C", 0x08},
	"KeyD": {"kVK_ANSI_D", 0x02}, "KeyE": {"kVK_ANSI_E", 0x0E}, "KeyF": {"kVK_ANSI_F", 0x03},
	"KeyG": {"kVK_ANSI_G", 0x05}, "KeyH": {"kVK_ANSI_H", 0x04}, "KeyI": {"kVK_ANSI_I", 0x22},
	"KeyJ": {"kVK_ANSI_J", 0x26}, "KeyK": {"kVK_ANSI_K", 0x28}, "KeyL": {"kVK_ANSI_L", 0x25},
	"KeyM": {"kVK_ANSI_M", 0x2E}, "KeyN": {"kVK_ANSI_N", 0x2D}, "KeyO": {"kVK_ANSI_O", 0x1F},
	"KeyP": {"kVK_ANSI_P", 0x23}, "KeyQ": {"kVK_ANSI_Q", 0x0C}, "KeyR": {"kVK_ANSI_R", 0x0F},
	"KeyS": {"kVK_ANSI_S", 0x01}, "KeyT": {"kVK_ANSI_T", 0x11}, "KeyU": {"kVK_ANSI_U", 0x20},
	"KeyV": {"kVK_ANSI_V", 0x09}, "KeyW": {"kVK_ANSI_W", 0x0D}, "KeyX": {"kVK_ANSI_X", 0x07},
	"KeyY": {"kVK_ANSI_Y", 0x10}, "KeyZ": {"kVK_ANSI_Z", 0x06},

	"Digit0": {"kVK_ANSI_0", 0x1D}, "Digit1": {"kVK_ANSI_1", 0x12}, "Digit2": {"kVK_ANSI_2", 0x13},
	"Digit3": {"kVK_ANSI_3", 0x14}, "Digit4": {"kVK_ANSI_4", 0x15}, "Digit5": {"kVK_ANSI_5", 0x17},
	"Digit6": {"kVK_ANSI_6", 0x16}, "Digit7": {"kVK_ANSI_7", 0x1A}, "Digit8": {"kVK_ANSI_8", 0x1C},
	"Digit9": {"kVK_ANSI_9", 0x19},

	"Minus": {"kVK_ANSI_Minus", 0x1B}, "Equal": {"kVK_ANSI_Equal", 0x18},
	"BracketLeft": {"kVK_ANSI_LeftBracket", 0x21}, "BracketRight": {"kVK_ANSI_RightBracket", 0x1E},
	"Backslash": {"kVK_ANSI_Backslash", 0x2A}, "Semicolon": {"kVK_ANSI_Semicolon", 0x29},
	"Quote": {"kVK_ANSI_Quote", 0x27}, "Backquote": {"kVK_ANSI_Grave", 0x32},
	"Comma": {"kVK_ANSI_Comma", 0x2B}, "Period": {"kVK_ANSI_Period", 0x2F}, "Slash": {"kVK_ANSI_Slash", 0x2C},

	"Enter": {"kVK_Return", 0x24}, "Tab": {"kVK_Tab", 0x30}, "Space": {"kVK_Space", 0x31},
	"Backspace": {"kVK_Delete", 0x33}, "Escape": {"kVK_Escape", 0x35}, "Delete": {"kVK_ForwardDelete", 0x75},
	"Insert": {"kVK_Help", 0x72},
	"Home":   {"kVK_Home", 0x73}, "End": {"kVK_End", 0x77}, "PageUp": {"kVK_PageUp", 0x74}, "PageDown": {"kVK_PageDown", 0x79},
	"ArrowLeft": {"kVK_LeftArrow", 0x7B}, "ArrowRight": {"kVK_RightArrow", 0x7C},
	"ArrowDown": {"kVK_DownArrow", 0x7D}, "ArrowUp": {"kVK_UpArrow", 0x7E},

	"F1": {"kVK_F1", 0x7A}, "F2": {"kVK_F2", 0x78}, "F3": {"kVK_F3", 0x63}, "F4": {"kVK_F4", 0x76},
	"F5": {"kVK_F5", 0x60}, "F6": {"kVK_F6", 0x61}, "F7": {"kVK_F7", 0x62}, "F8": {"kVK_F8", 0x64},
	"F9": {"kVK_F9", 0x65}, "F10": {"kVK_F10", 0x6D}, "F11": {"kVK_F11", 0x67}, "F12": {"kVK_F12", 0x6F},
	"F13": {"kVK_F13", 0x69}, "F14": {"kVK_F14", 0x6B}, "F15": {"kVK_F15", 0x71}, "F16": {"kVK_F16", 0x6A},
	"F17": {"kVK_F17", 0x40}, "F18": {"kVK_F18", 0x4F}, "F19": {"kVK_F19", 0x50}, "F20": {"kVK_F20", 0x5A},

	"Numpad0": {"kVK_ANSI_Keypad0", 0x52}, "Numpad1": {"kVK_ANSI_Keypad1", 0x53},
	"Numpad2": {"kVK_ANSI_Keypad2", 0x54}, "Numpad3": {"kVK_ANSI_Keypad3", 0x55},
	"Numpad4": {"kVK_ANSI_Keypad4", 0x56}, "Numpad5": {"kVK_ANSI_Keypad5", 0x57},
	"Numpad6": {"kVK_ANSI_Keypad6", 0x58}, "Numpad7": {"kVK_ANSI_Keypad7", 0x59},
	"Numpad8": {"kVK_ANSI_Keypad8", 0x5B}, "Numpad9": {"kVK_ANSI_Keypad9", 0x5C},
	"NumpadEnter": {"kVK_ANSI_KeypadEnter", 0x4C}, "NumpadDecimal": {"kVK_ANSI_KeypadDecimal", 0x41},
	"NumpadMultiply": {"kVK_ANSI_KeypadMultiply", 0x43}, "NumpadAdd": {"kVK_ANSI_KeypadPlus", 0x45},
	"NumpadSubtract": {"kVK_ANSI_KeypadMinus", 0x4E}, "NumpadDivide": {"kVK_ANSI_KeypadDivide", 0x4B},
	"NumpadEqual": {"kVK_ANSI_KeypadEquals", 0x51}, "NumLock": {"kVK_ANSI_KeypadClear", 0x47},

	"ShiftLeft": {"kVK_Shift", 0x38}, "ShiftRight": {"kVK_RightShift", 0x3C},
	"ControlLeft": {"kVK_Control", 0x3B}, "ControlRight": {"kVK_RightControl", 0x3E},
	"AltLeft": {"kVK_Option", 0x3A}, "AltRight": {"kVK_RightOption", 0x3D},
	"MetaLeft": {"kVK_Command", 0x37}, "MetaRight": {"kVK_RightCommand", 0x36},
	"CapsLock": {"kVK_CapsLock", 0x39},

	"IntlBackslash": {"kVK_ISO_Section", 0x0A}, "IntlYen": {"kVK_JIS_Yen", 0x5D}, "IntlRo": {"kVK_JIS_Underscore", 0x5E},
	"Lang1": {"kVK_JIS_Kana", 0x68}, "Lang2": {"kVK_JIS_Eisu", 0x66},
}

// modifierFlags maps each modifier code to its CGEventFlags bit.
var modifierFlags = map[string]uint64{
	"ShiftLeft": FlagShift, "ShiftRight": FlagShift,
	"ControlLeft": FlagControl, "ControlRight": FlagControl,
	"AltLeft": FlagOption, "AltRight": FlagOption,
	"MetaLeft": FlagCommand, "MetaRight": FlagCommand,
	"CapsLock": FlagAlphaShift,
}

// KeyCode returns the kVK for a W3C KeyboardEvent.code; ok is false for a code
// with no entry.
func KeyCode(code string) (vk uint16, ok bool) {
	e, ok := keyTable[code]
	return e.vk, ok
}

// KeyName returns the kVK constant name of a code ("" for no entry).
func KeyName(code string) string { return keyTable[code].name }

// IsModifierCode reports whether code is a modifier key (including Caps Lock).
func IsModifierCode(code string) bool { _, ok := modifierFlags[code]; return ok }

// ModifierFlag returns the CGEventFlags bit of a modifier code, 0 for any other.
func ModifierFlag(code string) uint64 { return modifierFlags[code] }

// attaches reports whether r belongs with the rune before it: a joiner, a
// variation selector or a combining mark.
func attaches(r rune) bool {
	return r == 0x200D || r == 0xFE0F || unicode.Is(unicode.M, r)
}

// UTF16Chunks splits s into pieces of at most limit UTF-16 units (limit is at least
// 2, so a surrogate pair fits). A cut is made only between characters: never
// inside a surrogate pair, and a ZWJ sequence, variation selector or combining
// mark stays with what it modifies unless that run alone exceeds max.
func UTF16Chunks(s string, limit int) [][]uint16 {
	limit = max(limit, 2)
	// Group runes into clusters that must not be cut.
	var clusters [][]uint16
	joined := false // the previous rune was a ZWJ: this one joins it
	for _, r := range s {
		u := utf16.Encode([]rune{r})
		if n := len(clusters); n > 0 && (joined || attaches(r)) {
			clusters[n-1] = append(clusters[n-1], u...)
		} else {
			clusters = append(clusters, u)
		}
		joined = r == 0x200D
	}
	var out [][]uint16
	var cur []uint16
	flush := func() {
		if len(cur) > 0 {
			out = append(out, cur)
			cur = nil
		}
	}
	for _, c := range clusters {
		if len(cur)+len(c) > limit {
			flush()
		}
		for len(c) > limit { // one run longer than a chunk: cut it between code points
			cut := limit
			if utf16.IsSurrogate(rune(c[cut-1])) && c[cut-1] < 0xDC00 {
				cut-- // do not end on a high surrogate
			}
			out = append(out, c[:cut:cut])
			c = c[cut:]
		}
		cur = append(cur, c...)
	}
	flush()
	return out
}
