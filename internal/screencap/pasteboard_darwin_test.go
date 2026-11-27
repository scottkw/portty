package screencap

import (
	"errors"
	"testing"
)

// TestPasteboard_RefusesUnderTest: no test reads or writes the host's clipboard
// or its privacy setting.
func TestPasteboard_RefusesUnderTest(t *testing.T) {
	p := NewPasteboard()
	_, _, readErr := p.ReadText(1 << 20)
	writeErr := p.WriteText("x")
	_, behaviorErr := p.Behavior()
	for name, err := range map[string]error{"ReadText": readErr, "WriteText": writeErr, "Behavior": behaviorErr} {
		if !errors.Is(err, ErrRefusedInTest) {
			t.Errorf("%s: %v, want ErrRefusedInTest", name, err)
		}
	}
}
