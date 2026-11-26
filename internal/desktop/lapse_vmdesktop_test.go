//go:build vmdesktop

package desktop_test

import (
	"testing"
	"time"

	"github.com/scottkw/portty/internal/desktop"
)

// The simulated lapse takes the production transition: one LapseSeq, one daemon
// notification (no GUI attached), no second notification for a second call, live
// frames do not end it, and ClearSimulatedLapse does.
func TestSimulateLapse_OneNotificationHeldUntilCleared(t *testing.T) {
	r := newRig(t)
	r.join()

	r.env.mgr.SimulateLapse()
	r.env.mgr.SimulateLapse()
	if got := r.state(); got != desktop.StateAwaitingApproval {
		t.Fatalf("state %q, want awaiting_approval", got)
	}
	if st := r.env.mgr.Status(); st.LapseSeq != 1 {
		t.Fatalf("LapseSeq %d, want 1 for two calls", st.LapseSeq)
	}
	if got := r.w.notified(); len(got) != 1 {
		t.Fatalf("notifications %v, want exactly one", got)
	}

	// Frames keep arriving through ticks; the lapse is held.
	for i := 0; i < 40; i++ {
		r.src.last().emit(i == 0)
		r.w.clock.Advance(time.Second)
		r.env.mgr.TickForTest(r.ctx)
		time.Sleep(25 * time.Millisecond)
		if got := r.state(); got != desktop.StateAwaitingApproval {
			t.Fatalf("state %q after live frames, want the simulated lapse held", got)
		}
	}

	r.env.mgr.ClearSimulatedLapse()
	if got := r.state(); got == desktop.StateAwaitingApproval {
		t.Fatalf("state %q after ClearSimulatedLapse, want it cleared", got)
	}
}
