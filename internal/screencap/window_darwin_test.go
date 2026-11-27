package screencap

import (
	"errors"
	"testing"
)

// TestWindowBridge_RefusesUnderTest: no test reads or changes Accessibility
// state, the window list or the focus on the host.
func TestWindowBridge_RefusesUnderTest(t *testing.T) {
	b := NewWindowBridge()
	_, _, _, _, _, _, focusErr := b.Focus()
	_, _, _, _, _, _, infoErr := b.WindowInfo(1)
	_, minErr := b.Minimized(1, 0, 0, 100, 100, "t")
	_, raiseErr := b.Raise(1, 0, 0, 100, 100, "t")
	_, topErr := b.TopmostAt(10, 10)
	for name, err := range map[string]error{
		"Focus": focusErr, "WindowInfo": infoErr, "Minimized": minErr, "Raise": raiseErr, "TopmostAt": topErr,
	} {
		if !errors.Is(err, ErrRefusedInTest) {
			t.Errorf("%s: %v, want ErrRefusedInTest", name, err)
		}
	}
}

func TestRaiseRungNames(t *testing.T) {
	for rung, want := range map[int]string{1: "ax", 2: "workspace", 3: "activate", 0: "none", 4: "none"} {
		if got := RungName(rung); got != want {
			t.Errorf("RungName(%d) = %q, want %q", rung, got, want)
		}
	}
}
