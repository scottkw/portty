package screencap

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
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

// On a stopped source SelectSurface only records the surface (no native call);
// a window and any display ID are accepted now, an unknown kind is not.
func TestDisplaySource_SelectSurface(t *testing.T) {
	src := NewDisplaySource()
	for _, s := range []Surface{{Kind: SurfaceDisplay}, {Kind: SurfaceWindow, ID: 7}, {Kind: SurfaceDisplay, ID: 3}} {
		if err := src.SelectSurface(s); err != nil {
			t.Errorf("SelectSurface(%+v) = %v, want it recorded", s, err)
		}
	}
	if err := src.SelectSurface(Surface{Kind: SurfaceKind(9)}); !errors.Is(err, ErrSurfaceUnsupported) {
		t.Errorf("unknown kind = %v, want ErrSurfaceUnsupported", err)
	}
}

// ListSurfaces never asks ScreenCaptureKit for shareable content under go test.
func TestListSurfaces_RefusesUnderTest(t *testing.T) {
	if list, err := ListSurfaces(context.Background()); !errors.Is(err, ErrRefusedInTest) || list != nil {
		t.Fatalf("ListSurfaces under test = %v, %v, want ErrRefusedInTest", list, err)
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

// Every bridge that holds the Mac awake, posts an event or reads TCC refuses
// inside a test binary; the three pure reads are safe and just must not fail.
func TestSysBridges_RefuseUnderTest(t *testing.T) {
	if _, err := HoldDisplayAwake("test"); !errors.Is(err, ErrRefusedInTest) {
		t.Errorf("HoldDisplayAwake = %v, want ErrRefusedInTest", err)
	}
	if _, err := HoldSystemAwake("test"); !errors.Is(err, ErrRefusedInTest) {
		t.Errorf("HoldSystemAwake = %v, want ErrRefusedInTest", err)
	}
	if err := DeclareRemoteUserActivity("test"); !errors.Is(err, ErrRefusedInTest) {
		t.Errorf("DeclareRemoteUserActivity = %v, want ErrRefusedInTest", err)
	}
	if moved, err := NudgePointer(); !errors.Is(err, ErrRefusedInTest) || moved {
		t.Errorf("NudgePointer = %v, %v; want false, ErrRefusedInTest", moved, err)
	}
	if _, err := Preflight(); !errors.Is(err, ErrRefusedInTest) {
		t.Errorf("Preflight = %v, want ErrRefusedInTest", err)
	}

	locked, err := SessionLocked()
	if err != nil {
		t.Errorf("SessionLocked: %v", err)
	}
	asleep, err := DisplayAsleep()
	if err != nil {
		t.Errorf("DisplayAsleep: %v", err)
	}
	ac, err := OnACPower()
	if err != nil {
		t.Errorf("OnACPower: %v", err)
	}
	t.Logf("reads (not asserted): locked=%v displayAsleep=%v onAC=%v", locked, asleep, ac)
}

func TestNudgeDirection(t *testing.T) {
	const minX, maxX = 0.0, 1470.0
	for name, c := range map[string]struct{ x, want float64 }{
		"right edge":    {maxX - 0.5, -1},
		"left edge":     {minX, 1},
		"mid screen":    {735, 1},
		"just inside":   {maxX - 2.5, 1},
		"within a step": {maxX - 1, -1},
	} {
		if got := nudgeDelta(c.x, minX, maxX); got != c.want {
			t.Errorf("%s: nudgeDelta(%v) = %v, want %v", name, c.x, got, c.want)
		}
	}
	if got := nudgeDelta(0.5, 0, 1); got != 1 {
		t.Errorf("a display narrower than the step still gets +1, got %v", got)
	}
}

// The live permission reads exist in exactly one file, because a long-running
// process caches the first answer (research C1); no prompting variant exists.
func TestPreflightSymbols_OnlyInPreflightFile(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for ; ; root = filepath.Dir(root) {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
		if root == filepath.Dir(root) {
			t.Fatal("go.mod not found above the test directory")
		}
	}
	const home = "internal/screencap/preflight_darwin.m"
	reads := []string{"CGPreflightScreenCaptureAccess", "AXIsProcessTrusted", "CGPreflightPostEventAccess"}
	banned := []string{"CGRequestScreenCaptureAccess", "CGRequestPostEventAccess", "kAXTrustedCheckOptionPrompt"}
	sourceExt := map[string]bool{".go": true, ".m": true, ".h": true, ".c": true, ".swift": true, ".ts": true, ".tsx": true, ".js": true, ".sh": true, ".md": true, ".yml": true, ".yaml": true}
	skipDir := map[string]bool{".git": true, "node_modules": true, "spike": true, ".claude": true, ".planning": true}

	found := map[string][]string{} // symbol -> files
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if skipDir[d.Name()] || rel == "frontend/dist" {
				return filepath.SkipDir
			}
			return nil
		}
		if !sourceExt[filepath.Ext(path)] || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, sym := range append(slices.Clone(reads), banned...) {
			if bytes.Contains(data, []byte(sym)) {
				found[sym] = append(found[sym], rel)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, sym := range reads {
		if !slices.Equal(found[sym], []string{home}) {
			t.Errorf("%s found in %v, want only %s", sym, found[sym], home)
		}
	}
	for _, sym := range banned {
		if len(found[sym]) > 0 {
			t.Errorf("prompting API %s found in %v", sym, found[sym])
		}
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
