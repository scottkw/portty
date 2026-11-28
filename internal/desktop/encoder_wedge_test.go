package desktop_test

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/desktop"
	"github.com/scottkw/portty/internal/screencap"
)

// TestEncoderWedge_ResetsOwnServiceAndRecovers drives U-load's wedge through the
// real start path: with a viewer waiting, every capture start fails with
// ErrEncoderStart (VideoToolbox refusing new encoders). The third consecutive
// failure resets this process's encoder service exactly once, with a log line
// and a reason in Status; the next start streams and the reason clears.
func TestEncoderWedge_ResetsOwnServiceAndRecovers(t *testing.T) {
	var resets atomic.Int32
	r := newRig(t, func(_ *tracerEnv, d *desktop.Deps) {
		d.ResetEncoder = func() ([]int, error) {
			resets.Add(1)
			return []int{4242}, nil
		}
	})
	logs := captureLogs(t)
	r.join()
	r.settle(6 * time.Second)
	r.untilNudges(1)
	r.src.setStartErr(fmt.Errorf("%w (code -12902)", screencap.ErrEncoderStart))

	// "capture start failed" is logged after the recovery policy has seen the
	// start, so counting it means the start is fully handled.
	failed := func(n int) {
		t.Helper()
		eventually(t, 3*time.Second, fmt.Sprintf("start failure %d", n), func() bool { return logs.count("desktop: capture start failed") == n })
	}
	r.settle(1100 * time.Millisecond) // the nudge got no frame: restart
	failed(1)
	if !strings.Contains(r.env.mgr.Status().Reason, "encoder") {
		t.Fatalf("Status.Reason %q after an encoder start failure, want it to name the encoder", r.env.mgr.Status().Reason)
	}
	r.settle(10 * time.Second) // the state loop's start retry
	failed(2)
	if got := resets.Load(); got != 0 {
		t.Fatalf("%d encoder service resets after 2 failures, want none before the third", got)
	}
	r.settle(10 * time.Second)
	failed(3)
	if got := resets.Load(); got != 1 {
		t.Fatalf("%d encoder service resets after 3 failures, want 1", got)
	}
	if got := logs.count("ended this process's encoder service"); got != 1 {
		t.Fatalf("%d reset log lines, want 1", got)
	}

	// The fresh service works: the next retry starts and streams.
	r.src.setStartErr(nil)
	n := r.src.count()
	r.settle(10 * time.Second)
	eventually(t, 3*time.Second, "a start after the reset", func() bool { return r.src.count() > n })
	before := r.env.mgr.FramesSeenForTest()
	eventually(t, 3*time.Second, "frames after the reset", func() bool {
		r.src.last().emit(true) // only a started source emits
		return r.env.mgr.FramesSeenForTest() > before
	})
	if reason := r.env.mgr.Status().Reason; strings.Contains(reason, "encoder") {
		t.Fatalf("Status.Reason %q after a good start, want the encoder reason cleared", reason)
	}
	if got := resets.Load(); got != 1 {
		t.Fatalf("%d resets in all, want 1", got)
	}
}

// A reset that finds no service of this process, or fails, says so in its own
// line and claims nothing. Unset, Deps.ResetEncoder is the real
// screencap.ResetEncoderService, which refuses in a test binary: it must be
// wired (a nil func here would panic the state loop) and its error logged.
func TestEncoderWedge_ResetOutcomesAreLogged(t *testing.T) {
	for _, c := range []struct {
		name  string
		reset func() ([]int, error)
		line  string
	}{
		{"nothing found", func() ([]int, error) { return nil, nil }, "no encoder service of this process was found"},
		{"default refuses in tests", nil, "resetting its service failed"},
	} {
		t.Run(c.name, func(t *testing.T) {
			r := newRig(t, func(_ *tracerEnv, d *desktop.Deps) { d.ResetEncoder = c.reset })
			logs := captureLogs(t)
			r.join()
			r.settle(6 * time.Second)
			r.untilNudges(1)
			r.src.setStartErr(fmt.Errorf("%w (code -12902)", screencap.ErrEncoderStart))
			r.settle(1100 * time.Millisecond)
			r.settle(10 * time.Second)
			r.settle(10 * time.Second)
			eventually(t, 3*time.Second, c.line, func() bool { return logs.count(c.line) == 1 })
			if got := logs.count("ended this process's encoder service"); got != 0 {
				t.Fatalf("%d 'ended' lines, want 0", got)
			}
		})
	}
}
