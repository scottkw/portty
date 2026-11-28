package desktop

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/screencap"
)

// encFail is what DisplaySource.Start returns when VideoToolbox refuses the
// encoder (U-load: code -12902 on every join while the service was wedged).
var encFail = fmt.Errorf("%w (code -12902)", screencap.ErrEncoderStart)

// step is one capture start fed to the policy: its error and the clock offset.
type step struct {
	err  error
	at   time.Duration
	want bool // reset the encoder service now
}

func runSteps(t *testing.T, steps []step) {
	t.Helper()
	base := time.Unix(1_700_000_000, 0)
	var r encoderRecovery
	for i, s := range steps {
		if got := r.note(s.err, base.Add(s.at)); got != s.want {
			t.Fatalf("start %d (err %v at +%v): reset %v, want %v", i+1, s.err, s.at, got, s.want)
		}
	}
}

// Never on the first failure: the service is reset at the third consecutive
// encoder start failure, once.
func TestEncoderRecovery_ResetsOnThirdConsecutiveFailure(t *testing.T) {
	runSteps(t, []step{
		{encFail, 0, false},
		{encFail, 5 * time.Second, false},
		{encFail, 10 * time.Second, true},
	})
}

// A start that worked proves the service works: the count starts over.
func TestEncoderRecovery_SuccessClearsTheCount(t *testing.T) {
	runSteps(t, []step{
		{encFail, 0, false},
		{encFail, time.Second, false},
		{nil, 2 * time.Second, false},
		{encFail, 3 * time.Second, false},
		{encFail, 4 * time.Second, false},
		{encFail, 5 * time.Second, true},
	})
}

// Failures that never reached the encoder (no Screen Recording grant, no
// display, a capture error) neither count nor break the run of encoder failures.
func TestEncoderRecovery_OnlyEncoderFailuresCount(t *testing.T) {
	other := errors.New("screencap: starting capture failed (code -3805)")
	runSteps(t, []step{
		{screencap.ErrPermissionDenied, 0, false},
		{screencap.ErrNoDisplay, time.Second, false},
		{other, 2 * time.Second, false},
		{encFail, 3 * time.Second, false},
		{screencap.ErrNoDisplay, 4 * time.Second, false},
		{encFail, 5 * time.Second, false},
		{other, 6 * time.Second, false},
		{encFail, 7 * time.Second, true},
	})
}

// After a reset that did not help, the next one needs three more failures and
// a minute; a failure after the minute resets at once.
func TestEncoderRecovery_BacksOffBetweenResets(t *testing.T) {
	runSteps(t, []step{
		{encFail, 0, false},
		{encFail, time.Second, false},
		{encFail, 2 * time.Second, true},
		{encFail, 3 * time.Second, false},
		{encFail, 4 * time.Second, false},
		{encFail, 5 * time.Second, false},  // three since the reset, but inside the minute
		{encFail, 61 * time.Second, false}, // a minute after the reset is 62 s
		{encFail, 62 * time.Second, true},
	})
	// Fewer than three failures since the reset never reset, however late.
	runSteps(t, []step{
		{encFail, 0, false},
		{encFail, time.Second, false},
		{encFail, 2 * time.Second, true},
		{encFail, time.Hour, false},
		{encFail, 2 * time.Hour, false},
		{encFail, 3 * time.Hour, true},
	})
}

// A wall clock stepped back (a VM guest's NTP step) must not hold resets off
// until it catches up.
func TestEncoderRecovery_ClockStepBackDoesNotBlock(t *testing.T) {
	runSteps(t, []step{
		{encFail, time.Hour, false},
		{encFail, time.Hour + time.Second, false},
		{encFail, time.Hour + 2*time.Second, true},
		{encFail, 0, false},
		{encFail, time.Second, false},
		{encFail, 2 * time.Second, true},
	})
}
