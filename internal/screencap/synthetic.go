package screencap

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Fixture NAL units. The bytes need not decode: tests check NAL types and the
// SPS profile fields. The SPS header is 0x67 (type 7), profile_idc 0x42
// (baseline), constraint flags 0xe0 and level_idc 0x1f, which is the
// profile-level-id 42e01f the peer negotiates.
var (
	startCode  = []byte{0, 0, 0, 1}
	fixtureSPS = []byte{0x67, 0x42, 0xe0, 0x1f, 0xda, 0x05, 0x00, 0x7d, 0xb0, 0x11}
	fixturePPS = []byte{0x68, 0xce, 0x3c, 0x80}
	fixtureIDR = []byte{0x65, 0x88, 0x84, 0x00, 0x33, 0xff}
	fixtureP   = []byte{0x41, 0x9a, 0x24, 0x6c, 0x41, 0x4f}
)

// SyntheticSource is a pure-Go Source that emits fixture Annex-B access units
// on a ticker. It exists so tests and the loopback tracer exercise the whole
// pipeline with no ScreenCaptureKit, VideoToolbox, TCC or cgo.
type SyntheticSource struct {
	mu      sync.Mutex
	w, h    int
	frames  chan AccessUnit
	errs    chan error
	stop    chan struct{}
	done    chan struct{}
	started bool
	forceKF bool
	forced  int
}

// NewSyntheticSource returns a source that will emit width x height units.
func NewSyntheticSource(width, height int) *SyntheticSource {
	return &SyntheticSource{w: width, h: height}
}

// Start emits a keyframe first, then one access unit per tick at cfg.FPS.
func (s *SyntheticSource) Start(ctx context.Context, cfg Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return errors.New("screencap: synthetic source already started")
	}
	if cfg.Surface.Kind != SurfaceDisplay {
		return ErrSurfaceUnsupported
	}
	fps := cfg.FPS
	if fps <= 0 {
		fps = DefaultFPS
	}
	s.started = true
	s.forceKF = true // the first unit is always a keyframe
	s.frames = make(chan AccessUnit, 4)
	s.errs = make(chan error, 1)
	s.stop = make(chan struct{})
	s.done = make(chan struct{})
	go s.run(ctx, time.Second/time.Duration(fps), s.frames, s.stop, s.done)
	return nil
}

func (s *SyntheticSource) run(ctx context.Context, every time.Duration, frames chan<- AccessUnit, stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	defer close(frames)
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		au := s.next(every)
		select {
		case frames <- au:
		case <-stop:
			return
		case <-ctx.Done():
			return
		}
		select {
		case <-t.C:
		case <-stop:
			return
		case <-ctx.Done():
			return
		}
	}
}

// next builds the next access unit, consuming a pending keyframe request.
func (s *SyntheticSource) next(d time.Duration) AccessUnit {
	s.mu.Lock()
	defer s.mu.Unlock()
	au := AccessUnit{Width: s.w, Height: s.h, Duration: d}
	if s.forceKF {
		s.forceKF = false
		au.Keyframe = true
		au.Data = nal(fixtureSPS, fixturePPS, fixtureIDR)
		return au
	}
	au.Data = nal(fixtureP)
	return au
}

func nal(units ...[]byte) []byte {
	var b []byte
	for _, u := range units {
		b = append(b, startCode...)
		b = append(b, u...)
	}
	return b
}

// Stop ends emission and closes Frames. It is idempotent.
func (s *SyntheticSource) Stop() error {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return nil
	}
	stop, done := s.stop, s.done
	s.stop = nil
	s.mu.Unlock()
	if stop != nil {
		close(stop)
	}
	<-done
	return nil
}

// Frames returns the access-unit channel, closed by Stop. It is nil before Start.
func (s *SyntheticSource) Frames() <-chan AccessUnit {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.frames
}

// Errors returns the error channel; the synthetic source never reports one.
func (s *SyntheticSource) Errors() <-chan error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.errs
}

// SelectSurface accepts the main display and refuses windows (Phase 20).
func (s *SyntheticSource) SelectSurface(surf Surface) error {
	if surf.Kind != SurfaceDisplay {
		return ErrSurfaceUnsupported
	}
	return nil
}

// ForceKeyframe makes the next access unit a keyframe and counts the request.
func (s *SyntheticSource) ForceKeyframe() {
	s.mu.Lock()
	s.forceKF = true
	s.forced++
	s.mu.Unlock()
}

// Rebuild sets the size and forces a keyframe, as a real encoder re-creation does.
func (s *SyntheticSource) Rebuild(width, height int) error {
	if width <= 0 || height <= 0 {
		return errors.New("screencap: rebuild needs a positive size")
	}
	s.mu.Lock()
	s.w, s.h = width, height
	s.forceKF = true
	s.mu.Unlock()
	return nil
}

// KeyframesForced reports how many ForceKeyframe calls the source has seen.
func (s *SyntheticSource) KeyframesForced() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.forced
}
