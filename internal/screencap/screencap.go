// Package screencap is the capture interface of the always-available desktop
// link: Source turns a display (Phase 20: a window) into H.264 access units.
// This file declares the whole contract; the cgo bridges to ScreenCaptureKit
// and VideoToolbox arrive in plan 19-05, and every real bridge refuses to run
// under testing.Testing() so no test ever captures a screen or touches TCC.
// SyntheticSource (synthetic.go) is the pure-Go implementation tests use.
package screencap

import (
	"context"
	"errors"
	"time"
)

// SurfaceKind says what a Surface names.
type SurfaceKind int

const (
	// SurfaceDisplay is a whole display.
	SurfaceDisplay SurfaceKind = iota
	// SurfaceWindow is a single window. Phase 20 implements it; Phase 19 only
	// ever selects the main display.
	SurfaceWindow
)

// Surface names what is captured. ID 0 with SurfaceDisplay is the main display.
type Surface struct {
	Kind SurfaceKind
	ID   uint32
}

// Config is the capture and encode configuration of one Start.
type Config struct {
	Surface          Surface
	MaxLongEdge      int           // pixel cap on the longer edge of the encoded picture
	FPS              int           // target frame rate
	BitrateBps       int           // average encoder bitrate
	KeyframeInterval time.Duration // ceiling between keyframes
}

// Defaults for DefaultConfig, taken from the spike's measured values.
const (
	DefaultMaxLongEdge      = 2560
	DefaultFPS              = 30
	DefaultBitrateBps       = 8_000_000
	DefaultKeyframeInterval = 4 * time.Second
)

// DefaultConfig captures the main display with the default constants.
func DefaultConfig() Config {
	return Config{
		Surface:          Surface{Kind: SurfaceDisplay},
		MaxLongEdge:      DefaultMaxLongEdge,
		FPS:              DefaultFPS,
		BitrateBps:       DefaultBitrateBps,
		KeyframeInterval: DefaultKeyframeInterval,
	}
}

// AccessUnit is one encoded picture. Data is Annex-B (start-code delimited
// NAL units); every keyframe carries SPS and PPS ahead of the IDR slice.
// Width and Height are the encoded dimensions of this unit, so a viewer can
// follow a Rebuild or a surface change without a side channel.
type AccessUnit struct {
	Data          []byte
	Keyframe      bool
	Width, Height int
	Duration      time.Duration
}

// Permissions is what the capture permission probe reports (19-09 fills it):
// Screen Recording, Accessibility, and a pointer-event (nudge) capability.
type Permissions struct{ SR, AX, PE bool }

// Errors a Source reports.
var (
	// ErrPermissionDenied is ScreenCaptureKit error -3801 (no Screen Recording grant).
	ErrPermissionDenied = errors.New("screencap: screen recording permission denied")
	// ErrNoDisplay is ScreenCaptureKit error -3815 (no display to capture, e.g. asleep).
	ErrNoDisplay = errors.New("screencap: no display")
	// ErrRefusedInTest is returned by every real bridge under testing.Testing().
	ErrRefusedInTest = errors.New("screencap: real capture refused in tests")
	// ErrSurfaceUnsupported is returned for a surface the source cannot capture.
	ErrSurfaceUnsupported = errors.New("screencap: surface unsupported")
	// ErrNotRunning is returned by a call that needs a started source.
	ErrNotRunning = errors.New("screencap: not running")
)

// Source captures a surface and emits encoded access units.
//
//   - Frames is closed by Stop.
//   - ForceKeyframe is coalesced by the caller (the desktop Manager), not by
//     the source: each call asks for one more keyframe.
//   - Rebuild re-creates the encoder at the new dimensions; the next access
//     unit is a keyframe carrying SPS and PPS.
//   - SelectSurface with a SurfaceWindow returns ErrSurfaceUnsupported in
//     Phase 19 (Phase 20 implements it).
type Source interface {
	Start(ctx context.Context, cfg Config) error
	Stop() error
	Frames() <-chan AccessUnit
	Errors() <-chan error
	SelectSurface(s Surface) error
	ForceKeyframe()
	Rebuild(width, height int) error
}
