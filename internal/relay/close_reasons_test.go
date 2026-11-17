package relay

import "testing"

// The literals are a wire contract with the frontend and older installs.
func TestCloseReason_Literals(t *testing.T) {
	if CloseReasonShareStopped != "share stopped" {
		t.Fatalf("CloseReasonShareStopped = %q", CloseReasonShareStopped)
	}
	if CloseReasonSessionEnded != "session ended" {
		t.Fatalf("CloseReasonSessionEnded = %q", CloseReasonSessionEnded)
	}
}
