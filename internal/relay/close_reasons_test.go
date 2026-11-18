package relay

import (
	"errors"
	"fmt"
	"testing"

	"github.com/coder/websocket"
)

// The literals are a wire contract with the frontend and older installs.
func TestCloseReason_Literals(t *testing.T) {
	if CloseReasonShareStopped != "share stopped" {
		t.Fatalf("CloseReasonShareStopped = %q", CloseReasonShareStopped)
	}
	if CloseReasonSessionEnded != "session ended" {
		t.Fatalf("CloseReasonSessionEnded = %q", CloseReasonSessionEnded)
	}
}

// Only the two owner-initiated reasons on 1000 end a reconnecting client.
func TestIsTerminalClose(t *testing.T) {
	ce := func(code websocket.StatusCode, reason string) error {
		return websocket.CloseError{Code: code, Reason: reason}
	}
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"share stopped", ce(websocket.StatusNormalClosure, CloseReasonShareStopped), true},
		{"session ended", ce(websocket.StatusNormalClosure, CloseReasonSessionEnded), true},
		{"wrapped", fmt.Errorf("read: %w", ce(websocket.StatusNormalClosure, CloseReasonShareStopped)), true},
		{"detach", ce(websocket.StatusNormalClosure, "detach"), false},
		{"abnormal", ce(websocket.StatusAbnormalClosure, ""), false},
		{"too slow", ce(websocket.StatusPolicyViolation, "too slow"), false},
		{"plain", errors.New("EOF"), false},
		{"nil", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsTerminalClose(tc.err); got != tc.want {
				t.Fatalf("IsTerminalClose = %v; want %v", got, tc.want)
			}
		})
	}
}
