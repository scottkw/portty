package screencap

import (
	"errors"
	"testing"
)

// TestInputBridge_RefusesUnderTest: no test ever posts an event on the host.
func TestInputBridge_RefusesUnderTest(t *testing.T) {
	b := NewInputBridge()
	for name, err := range map[string]error{
		"mouse":  b.PostMouse(MouseDown, 10, 10, 0, 1, 0, 0, 0),
		"key":    b.PostKey(0, true, 0, false),
		"text":   b.PostText([]uint16{'a'}),
		"scroll": b.PostScroll(0, 10, 0),
	} {
		if !errors.Is(err, ErrRefusedInTest) {
			t.Errorf("%s: %v, want ErrRefusedInTest", name, err)
		}
	}
	if err := b.PostText(make([]uint16, MaxTextUnits+1)); err == nil || errors.Is(err, ErrRefusedInTest) {
		t.Errorf("a %d-unit text: %v, want a length error", MaxTextUnits+1, err)
	}
	if err := b.PostText(nil); err == nil {
		t.Error("empty text was accepted")
	}
}

// TestInputBridge_ReadsAreHarmless: the cursor and display reads run on a Mac
// with a window server and post nothing.
func TestInputBridge_ReadsAreHarmless(t *testing.T) {
	if _, _, err := NewInputBridge().CursorLocation(); err != nil {
		t.Skipf("no window server: %v", err)
	}
	if _, _, w, h, err := DisplayBounds(0); err != nil || w <= 0 || h <= 0 {
		t.Errorf("DisplayBounds(0) = %v x %v, %v", w, h, err)
	}
}

func TestEventMarker_MatchesC(t *testing.T) {
	if got := eventMarkerC(); got != EventMarker {
		t.Errorf("C marker %#x, Go EventMarker %#x", got, EventMarker)
	}
}
