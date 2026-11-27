package screencap

import (
	"errors"
	"testing"
)

// TestUnlockBridge_MapRoundTrips: every printable ASCII character the host's
// current layout can produce maps to a stroke that translates back to the same
// character through UCKeyTranslate with the stroke's modifier state. A layout
// that cannot produce a character is not a failure (ErrUnmappable is the
// answer for it), but a layout that produces none at all skips the test.
func TestUnlockBridge_MapRoundTrips(t *testing.T) {
	b := NewUnlockBridge()
	mapped := 0
	for c := byte(0x20); c <= 0x7E; c++ {
		strokes, err := b.MapRunes([]byte{c})
		if errors.Is(err, ErrUnmappable) {
			continue
		}
		if err != nil {
			t.Skipf("no keyboard layout to read: %v", err)
		}
		if len(strokes) != 1 {
			t.Fatalf("%q mapped to %d strokes, want 1", c, len(strokes))
		}
		got, err := unlockTranslate(strokes[0])
		if err != nil || got != rune(c) {
			t.Errorf("%q: stroke %+v translates back to %q (%v)", c, strokes[0], got, err)
		}
		mapped++
	}
	if mapped == 0 {
		t.Skip("the host layout produces no printable ASCII")
	}
	t.Logf("%d of 95 printable ASCII characters map on this layout", mapped)
	// Several characters at once map in order, one stroke each.
	if strokes, err := b.MapRunes([]byte("a")); err == nil {
		if two, err := b.MapRunes([]byte("aa")); err != nil || len(two) != 2 || two[0] != strokes[0] || two[1] != strokes[0] {
			t.Errorf("aa mapped to %+v, %v", two, err)
		}
	}
}

// TestUnlockBridge_RefusesUnmappable: a character no key produces (an emoji),
// invalid UTF-8 and an empty buffer are refused with ErrUnmappable before any
// stroke is returned.
func TestUnlockBridge_RefusesUnmappable(t *testing.T) {
	b := NewUnlockBridge()
	if _, err := b.MapRunes([]byte("a")); err != nil && !errors.Is(err, ErrUnmappable) {
		t.Skipf("no keyboard layout to read: %v", err)
	}
	for name, in := range map[string][]byte{
		"emoji":       []byte("pa\U0001F600ss"),
		"bad utf8":    {0xff, 0xfe, 'a'},
		"truncated":   {'a', 0xe2, 0x82},
		"overlong":    {0xc0, 0xaf},
		"surrogate":   {0xed, 0xa0, 0x80},
		"too big":     {0xf4, 0x90, 0x80, 0x80},
		"empty":       {},
		"control":     {'a', 0x07},
		"delete":      {0x7f},
		"private-use": {0xef, 0x9c, 0x80}, // U+F700
	} {
		strokes, err := b.MapRunes(in)
		if !errors.Is(err, ErrUnmappable) || strokes != nil {
			t.Errorf("%s: %+v, %v, want nil and ErrUnmappable", name, strokes, err)
		}
	}
}

// TestUnlockBridge_ReadsAreHarmless: the Caps Lock read posts nothing and the
// lock read agrees with SessionLocked.
func TestUnlockBridge_ReadsAreHarmless(t *testing.T) {
	b := NewUnlockBridge()
	if _, err := b.CapsLockOn(); err != nil {
		t.Errorf("CapsLockOn: %v", err)
	}
	want, werr := SessionLocked()
	got, gerr := b.SessionLocked()
	if want != got || (werr == nil) != (gerr == nil) {
		t.Errorf("bridge says %v, %v; SessionLocked says %v, %v", got, gerr, want, werr)
	}
}

// TestUnlockBridge_RefusesUnderTest: no test ever wakes the Mac or types into it.
func TestUnlockBridge_RefusesUnderTest(t *testing.T) {
	b := NewUnlockBridge()
	for name, err := range map[string]error{
		"wake":   b.Wake(0x38),
		"type":   b.Type(KeyStroke{VK: 0}),
		"return": b.Return(),
	} {
		if !errors.Is(err, ErrRefusedInTest) {
			t.Errorf("%s: %v, want ErrRefusedInTest", name, err)
		}
	}
}
