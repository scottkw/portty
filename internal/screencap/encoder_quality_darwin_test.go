package screencap

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// The rate run: noisy frames (so the encoder can use whatever rate it is given)
// at 30 fps for 4 s, 8 Mbps for the first 2 s and 1 Mbps after a live change.
const (
	rateFPS      = 30
	rateFrames   = 4 * rateFPS
	rateChangeAt = 2 * rateFPS
	rateHigh     = 8_000_000
	rateLow      = 1_000_000
)

var (
	rateOnce  sync.Once
	rateUnits []AccessUnit
	rateErr   error
)

// rateRun runs the synthetic rate encode once and shares the result.
func rateRun(t *testing.T) []AccessUnit {
	t.Helper()
	rateOnce.Do(func() {
		rateUnits, rateErr = encodeSyntheticRateForTest(rateFrames, 1280, 720, rateFPS, rateChangeAt, rateHigh, rateLow, -1, time.Second/rateFPS)
	})
	if errors.Is(rateErr, errNoEncoder) {
		t.Skipf("no H.264 encoder on this machine: %v", rateErr)
	}
	if rateErr != nil {
		t.Fatal(rateErr)
	}
	return rateUnits
}

func sumBytes(units []AccessUnit) (n int) {
	for _, u := range units {
		n += len(u.Data)
	}
	return n
}

// A live bitrate change on the running session lowers the produced rate: the
// last second at 1 Mbps carries at most half the bytes per second of the first
// two seconds at 8 Mbps (A-apply; RESEARCH assumption A7).
func TestEncoderRate_LiveChangeLowersRate(t *testing.T) {
	units := rateRun(t)
	if len(units) != rateFrames {
		t.Fatalf("got %d units for %d frames; per-frame accounting needs one unit per frame", len(units), rateFrames)
	}
	before := float64(sumBytes(units[:rateChangeAt])) / 2  // bytes per second over the first 2 s
	after := float64(sumBytes(units[rateFrames-rateFPS:])) // bytes in the last second
	t.Logf("bytes_per_s before=%.0f (%.2f Mbps) after=%.0f (%.2f Mbps) ratio=%.3f", before, before*8/1e6, after, after*8/1e6, after/before)
	if after > before/2 {
		t.Fatalf("after the change to %d bps the rate is %.0f B/s, want at most half of %.0f B/s", rateLow, after, before)
	}
}

// The change is a property set on the live session, not a new session: no unit
// after the first is a keyframe (a new session would start with one).
func TestEncoderRate_SessionIsNotRecreated(t *testing.T) {
	units := rateRun(t)
	if !units[0].Keyframe {
		t.Fatal("the first unit is not a keyframe")
	}
	for i, u := range units[1:] {
		if u.Keyframe {
			t.Fatalf("unit %d is an unrequested keyframe: the session was recreated", i+1)
		}
	}
}

// A resolution step rebuilds the encoder; the rebuilt session keeps the live rate
// (its first unit is the keyframe a rebuild promises, and the rate stays low).
func TestEncoderRate_RebuildKeepsRate(t *testing.T) {
	const rebuildAt = rateChangeAt + rateFPS/2
	units, err := encodeSyntheticRateForTest(rateFrames, 1280, 720, rateFPS, rateChangeAt, rateHigh, rateLow, rebuildAt, time.Second/rateFPS)
	if errors.Is(err, errNoEncoder) {
		t.Skipf("no H.264 encoder on this machine: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	if len(units) < rateFrames {
		t.Fatalf("got %d units for %d frames", len(units), rateFrames)
	}
	after := float64(sumBytes(units[len(units)-rateFPS:]))
	before := float64(sumBytes(units[:rateChangeAt])) / 2
	t.Logf("bytes_per_s before=%.0f after_rebuild=%.0f ratio=%.3f", before, after, after/before)
	if after > before/2 {
		t.Fatalf("a rebuilt session runs at %.0f B/s, want at most half of %.0f B/s: it did not keep the live rate", after, before)
	}
}

// SetBitrate and SetFPS on a stopped source say so; SetFPS touches the live
// stream, so even a source that believes it runs refuses it inside a test.
func TestEncoderRate_StoppedAndTestRefusals(t *testing.T) {
	d := &DisplaySource{}
	if err := d.SetBitrate(rateLow); !errors.Is(err, ErrNotRunning) {
		t.Errorf("SetBitrate on a stopped source = %v, want ErrNotRunning", err)
	}
	if err := d.SetFPS(15); !errors.Is(err, ErrNotRunning) {
		t.Errorf("SetFPS on a stopped source = %v, want ErrNotRunning", err)
	}
	d.running = true
	if err := d.SetFPS(15); !errors.Is(err, ErrRefusedInTest) {
		t.Errorf("SetFPS under a test = %v, want ErrRefusedInTest", err)
	}
	d.running = false
}
