package screencap

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
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

func TestDisplaySource_RefusesUnderTest(t *testing.T) {
	src := NewDisplaySource()
	if err := src.Start(context.Background(), DefaultConfig()); !errors.Is(err, ErrRefusedInTest) {
		t.Fatalf("Start under test = %v, want ErrRefusedInTest", err)
	}
	if src.Frames() != nil || src.Errors() != nil {
		t.Error("a refused Start must open no channel")
	}
	if err := src.Stop(); !errors.Is(err, ErrNotRunning) {
		t.Errorf("Stop on an unstarted source = %v, want ErrNotRunning", err)
	}
}

func TestCaptureStill_RefusesUnderTest(t *testing.T) {
	if b, err := CaptureStill(context.Background(), 1280); !errors.Is(err, ErrRefusedInTest) || b != nil {
		t.Fatalf("CaptureStill under test = %d bytes, %v; want ErrRefusedInTest", len(b), err)
	}
}

func TestDisplaySource_SelectSurface(t *testing.T) {
	src := NewDisplaySource()
	if err := src.SelectSurface(Surface{Kind: SurfaceDisplay}); err != nil {
		t.Errorf("main display: %v", err)
	}
	if err := src.SelectSurface(Surface{Kind: SurfaceWindow, ID: 7}); !errors.Is(err, ErrSurfaceUnsupported) {
		t.Errorf("window = %v, want ErrSurfaceUnsupported", err)
	}
	if err := src.SelectSurface(Surface{Kind: SurfaceDisplay, ID: 3}); !errors.Is(err, ErrSurfaceUnsupported) {
		t.Errorf("another display = %v, want ErrSurfaceUnsupported", err)
	}
}

// A full queue drops a delta and never a keyframe, and keeps order.
func TestDisplaySource_EmitWhenFull(t *testing.T) {
	d := &DisplaySource{frames: make(chan AccessUnit, 2)}
	unit := func(b byte, key bool) AccessUnit { return AccessUnit{Data: []byte{b}, Keyframe: key} }
	drain := func() (got []byte) {
		for len(d.frames) > 0 {
			got = append(got, (<-d.frames).Data[0])
		}
		return got
	}

	d.emit(unit(1, true))
	d.emit(unit(2, false))
	d.emit(unit(3, false)) // full: the new delta is dropped
	if got := drain(); !bytes.Equal(got, []byte{1, 2}) {
		t.Errorf("delta into a full queue: queue = %v, want [1 2]", got)
	}

	d.emit(unit(4, false))
	d.emit(unit(5, false))
	d.emit(unit(6, true)) // full: a keyframe evicts the oldest unit
	if got := drain(); !bytes.Equal(got, []byte{5, 6}) {
		t.Errorf("keyframe into a full queue: queue = %v, want [5 6]", got)
	}
}

func TestMapSCKError(t *testing.T) {
	if err := mapSCKError(-3801, "denied"); !errors.Is(err, ErrPermissionDenied) {
		t.Errorf("-3801 = %v, want ErrPermissionDenied", err)
	}
	if err := mapSCKError(-3815, "no display"); !errors.Is(err, ErrNoDisplay) {
		t.Errorf("-3815 = %v, want ErrNoDisplay", err)
	}
	err := mapSCKError(-3805, "stream stopped")
	if err == nil || errors.Is(err, ErrPermissionDenied) || errors.Is(err, ErrNoDisplay) || !strings.Contains(err.Error(), "-3805") {
		t.Errorf("other code = %v, want a non-sentinel error naming -3805", err)
	}
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
