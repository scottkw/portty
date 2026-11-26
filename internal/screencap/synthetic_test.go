package screencap

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"
)

// nalTypes lists the NAL unit types of an Annex-B buffer.
func nalTypes(b []byte) []int {
	var out []int
	for i := 0; i+4 < len(b); i++ {
		if bytes.Equal(b[i:i+4], startCode) {
			out = append(out, int(b[i+4]&0x1f))
		}
	}
	return out
}

func next(t *testing.T, s *SyntheticSource) AccessUnit {
	t.Helper()
	select {
	case au, ok := <-s.Frames():
		if !ok {
			t.Fatal("Frames closed")
		}
		return au
	case <-time.After(2 * time.Second):
		t.Fatal("no access unit within 2s")
		return AccessUnit{}
	}
}

// TestSynthetic_RebuildChangesSize: Rebuild changes the size of every later
// access unit, and the first one at the new size is a keyframe with SPS+PPS.
func TestSynthetic_RebuildChangesSize(t *testing.T) {
	s := NewSyntheticSource(1920, 1200)
	if err := s.Start(context.Background(), Config{FPS: 120}); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Stop() }()
	if au := next(t, s); au.Width != 1920 || au.Height != 1200 || !au.Keyframe {
		t.Fatalf("first unit %dx%d keyframe=%v, want a 1920x1200 keyframe", au.Width, au.Height, au.Keyframe)
	}

	if err := s.Rebuild(1280, 800); err != nil {
		t.Fatal(err)
	}
	// Units already queued or being built keep the old size; the first at the
	// new size is the keyframe.
	var au AccessUnit
	for range 20 {
		if au = next(t, s); au.Width == 1280 {
			break
		}
	}
	if au.Width != 1280 || au.Height != 800 {
		t.Fatalf("no unit at the new size, last %dx%d", au.Width, au.Height)
	}
	if got := nalTypes(au.Data); !au.Keyframe || len(got) != 3 || got[0] != 7 || got[1] != 8 || got[2] != 5 {
		t.Errorf("first unit at the new size: keyframe=%v NAL types %v, want a keyframe with 7, 8, 5", au.Keyframe, got)
	}
	for range 3 {
		if au = next(t, s); au.Width != 1280 || au.Height != 800 || au.Keyframe {
			t.Errorf("later unit %dx%d keyframe=%v, want a 1280x800 delta", au.Width, au.Height, au.Keyframe)
		}
	}

	if err := s.Rebuild(0, 800); err == nil {
		t.Error("Rebuild with a zero width succeeded")
	}
}

func TestSynthetic_SelectSurfaceRefusesWindows(t *testing.T) {
	s := NewSyntheticSource(1280, 800)
	if err := s.SelectSurface(Surface{Kind: SurfaceWindow, ID: 7}); !errors.Is(err, ErrSurfaceUnsupported) {
		t.Errorf("SelectSurface(window) = %v, want ErrSurfaceUnsupported", err)
	}
	if err := s.SelectSurface(Surface{Kind: SurfaceDisplay}); err != nil {
		t.Errorf("SelectSurface(display) = %v, want nil", err)
	}
}
