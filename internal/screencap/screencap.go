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

// SurfaceInfo describes one thing that can be shared, as ListSurfaces reports
// it. Sizes and positions are in points. A window's Name is "App — Title" (or
// just the app), a display's "Main Display" or "Display N"; names are for
// viewers only and never logged.
type SurfaceInfo struct {
	Kind           SurfaceKind
	ID             uint32 // CGDirectDisplayID or CGWindowID
	PID            int32  // owning process of a window, 0 for a display
	Name           string
	App, Title     string // windows only
	X, Y, W, H     float64
	Main, OnScreen bool // Main: the main display. OnScreen: a window that is not minimized or off screen
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
	Kind          UnitKind // how the unit came to be; the zero value is a fresh capture
}

// UnitKind says why an access unit exists. Only UnitFresh comes from a new
// capture; the others re-encode the cached picture, so the stall watchdog and the
// still machine must not mistake them for the screen changing.
type UnitKind int

const (
	// UnitFresh is a captured frame (the zero value, so a Source that never sets it is right).
	UnitFresh UnitKind = iota
	// UnitForced is the cached picture re-encoded as a keyframe for a joiner or a PLI.
	UnitForced
	// UnitRefine is the cached picture re-encoded sharply after the screen went still.
	UnitRefine
	// UnitIdle is the cached picture re-encoded as a small non-key frame to keep a still stream alive.
	UnitIdle
)

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
//   - SelectSurface on a running source switches it live to a display or a
//     window (the encoder is rebuilt at the new size and the next unit is a
//     keyframe); on a stopped one it records the surface for the next Start.
type Source interface {
	Start(ctx context.Context, cfg Config) error
	Stop() error
	Frames() <-chan AccessUnit
	Errors() <-chan error
	SelectSurface(s Surface) error
	ForceKeyframe()
	Rebuild(width, height int) error
}

// QualitySource is the optional live-quality half of a Source: the adaptive
// controller (desktop quality.go) changes the running encoder's target bitrate
// and the capture's frame rate through it, never by starting a new session.
// A Source that does not implement it is simply never adapted. Both return
// ErrNotRunning on a stopped source.
type QualitySource interface {
	// SetBitrate changes the running encoder's average bitrate in bits per second.
	SetBitrate(bps int) error
	// SetFPS changes the running capture's frame rate.
	SetFPS(fps int) error
}

// StillSource is the optional still-screen half of a Source (DESK-07): when the
// screen stops changing the desktop controller asks it to refine the cached
// picture once and then to refresh it quietly. Both return ErrNotRunning on a
// stopped source and do nothing while no picture is cached.
type StillSource interface {
	// Refine re-encodes the cached picture as a keyframe at bps (UnitRefine), then
	// puts the previous target back for the next unit.
	Refine(bps int) error
	// IdleRefresh re-encodes the cached picture as a small non-key frame (UnitIdle).
	IdleRefresh() error
}
