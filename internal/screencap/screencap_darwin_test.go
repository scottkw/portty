package screencap

import (
	"errors"
	"slices"
	"testing"
)

// nalTypes lists the NAL unit types of an Annex-B access unit.
func nalTypes(data []byte) []int {
	var types []int
	for i := 0; i+3 < len(data); i++ {
		if data[i] == 0 && data[i+1] == 0 && data[i+2] == 1 {
			types = append(types, int(data[i+3]&0x1f))
		}
	}
	return types
}

func hasNALs(data []byte, want ...int) bool {
	got := nalTypes(data)
	for _, w := range want {
		if !slices.Contains(got, w) {
			return false
		}
	}
	return true
}

// TestEncoder_SyntheticFramesTracer pushes synthetic NV12 frames through the
// real VideoToolbox encoder (no capture, no TCC) and checks what comes back to
// Go: Annex-B keyframes with SPS, PPS and IDR, a forced keyframe from the
// cached frame, and a rebuild at a new size (seam 9, real encoder).
func TestEncoder_SyntheticFramesTracer(t *testing.T) {
	const frames, forceAt, rebuildAt = 60, 30, 45
	units, err := encodeSyntheticForTest(frames, 1280, 800, forceAt, rebuildAt, 640, 400)
	if errors.Is(err, errNoEncoder) {
		t.Skipf("no H.264 encoder on this machine: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	switch hw := syntheticHardware.Load(); hw {
	case 1:
		t.Log("encoder readback: hardware")
	case 0:
		t.Log("encoder readback: software (accepted here only; production surfaces it)")
	default:
		t.Log("encoder readback: unknown")
	}
	if len(units) < frames {
		t.Fatalf("got %d access units for %d frames", len(units), frames)
	}

	first := units[0]
	if !first.Keyframe || !hasNALs(first.Data, 7, 8, 5) || first.Width != 1280 || first.Height != 800 {
		t.Fatalf("first unit: keyframe=%v nals=%v size=%dx%d, want keyframe with 7,8,5 at 1280x800",
			first.Keyframe, nalTypes(first.Data), first.Width, first.Height)
	}

	// The forced keyframe lands at or right after frame 30 (unit 30 re-encodes
	// the cached frame 29); the 4 s ceiling cannot fire in 2 s of frames.
	forced := -1
	for i := 1; i < len(units) && units[i].Width == 1280; i++ {
		if units[i].Keyframe {
			forced = i
			break
		}
	}
	if forced < forceAt-1 || forced > forceAt+3 {
		t.Errorf("forced keyframe at unit %d, want about %d", forced, forceAt)
	} else if !hasNALs(units[forced].Data, 7, 8, 5) {
		t.Errorf("forced keyframe NALs %v, want 7,8,5", nalTypes(units[forced].Data))
	}

	rebuilt := -1
	for i, u := range units {
		if u.Width == 640 {
			rebuilt = i
			break
		}
		if u.Width != 1280 || u.Height != 800 {
			t.Fatalf("unit %d is %dx%d before the rebuild", i, u.Width, u.Height)
		}
	}
	if rebuilt < 0 {
		t.Fatal("no unit at 640x400 after Rebuild")
	}
	if u := units[rebuilt]; !u.Keyframe || !hasNALs(u.Data, 7, 8, 5) || u.Height != 400 {
		t.Errorf("first unit after rebuild: keyframe=%v nals=%v size=%dx%d, want keyframe 7,8,5 at 640x400",
			u.Keyframe, nalTypes(u.Data), u.Width, u.Height)
	}
	for i := rebuilt; i < len(units); i++ {
		if units[i].Width != 640 || units[i].Height != 400 {
			t.Errorf("unit %d is %dx%d after the rebuild, want 640x400", i, units[i].Width, units[i].Height)
		}
	}
}
