package screencap

// media_darwin.go: the cgo side of the encoder (and capture) in media_darwin.m.
// Every C prototype lives here, the ObjC in media_darwin.m. Frames stay inside
// ObjC from capture to encoder and cross into Go once per access unit.
//
// No minimum-version CFLAG here: Phase 18.1's build.sh sets the macOS 14
// deployment target for the whole build.

/*
#cgo darwin CFLAGS: -x objective-c -fobjc-arc
#cgo darwin LDFLAGS: -framework ScreenCaptureKit -framework VideoToolbox -framework CoreMedia -framework CoreVideo -framework CoreGraphics -framework ImageIO -framework Foundation

#include <stdint.h>
#include <stdlib.h>

int pc_encode_synthetic(uintptr_t handle, int frames, int width, int height, int force, int rebuildAt, int rebuildW, int rebuildH, int *hardware);
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime/cgo"
	"sync/atomic"
	"time"
	"unsafe"
)

// errNoEncoder is VideoToolbox's kVTCouldNotFindVideoEncoderErr: this machine
// has no H.264 encoder at all (a bare CI runner). Tests skip on it only.
var errNoEncoder = errors.New("screencap: no H.264 encoder available")

const osStatusNoEncoder = -12908

// sink receives what the ObjC side produces. One per Start (or per synthetic
// run), reached from the callbacks through a cgo.Handle.
type sink struct {
	au   func(AccessUnit)
	err  func(error)
	seen func()
	last atomic.Int64 // unix nanos of the previous unit, for Duration
}

// lookup returns the sink behind a callback's handle, or nil if it is gone.
func lookup(h uintptr) (s *sink) {
	defer func() {
		if recover() != nil {
			s = nil
		}
	}()
	s, _ = cgo.Handle(h).Value().(*sink)
	return s
}

// statusErr names an OSStatus or ScreenCaptureKit code from the ObjC side.
func statusErr(code int, msg string) error {
	if code == osStatusNoEncoder {
		return errNoEncoder
	}
	return fmt.Errorf("screencap: %s (code %d)", msg, code)
}

//export goScreencapAccessUnit
func goScreencapAccessUnit(h C.uintptr_t, avcc unsafe.Pointer, avccLen C.int, sps unsafe.Pointer, spsLen C.int, pps unsafe.Pointer, ppsLen C.int, keyframe C.int, width, height C.int) {
	// A panic on a framework thread would end the daemon.
	defer func() {
		if r := recover(); r != nil {
			log.Printf("screencap: access unit callback: %v", r)
		}
	}()
	s := lookup(uintptr(h))
	if s == nil {
		return
	}
	key := keyframe != 0
	var params [][]byte
	if key {
		params = [][]byte{C.GoBytes(sps, spsLen), C.GoBytes(pps, ppsLen)}
	}
	data, err := avccToAnnexB(C.GoBytes(avcc, avccLen), params, key)
	if err != nil {
		s.err(err)
		return
	}
	// Variable-rate capture (frames only on change): the RTP clock follows real time.
	now := time.Now().UnixNano()
	dur := time.Second / DefaultFPS
	if prev := s.last.Swap(now); prev != 0 {
		dur = max(time.Duration(now-prev), time.Millisecond)
	}
	s.au(AccessUnit{Data: data, Keyframe: key, Width: int(width), Height: int(height), Duration: dur})
}

//export goScreencapStreamError
func goScreencapStreamError(h C.uintptr_t, code C.int, msg *C.char) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("screencap: error callback: %v", r)
		}
	}()
	if s := lookup(uintptr(h)); s != nil {
		s.err(statusErr(int(code), C.GoString(msg)))
	}
}

// syntheticHardware is the encoder readback of the last encodeSyntheticForTest:
// 1 hardware, 0 software, -1 unknown. Test-only diagnostic.
var syntheticHardware atomic.Int32

// encodeSyntheticForTest runs only the encoder on synthetic NV12 frames: no
// capture, no TCC. A negative force or rebuildAt disables that step.
func encodeSyntheticForTest(frames, width, height int, force, rebuildAt int, rebuildW, rebuildH int) ([]AccessUnit, error) {
	var units []AccessUnit
	var errs []error
	h := cgo.NewHandle(&sink{
		au:   func(au AccessUnit) { units = append(units, au) },
		err:  func(err error) { errs = append(errs, err) },
		seen: func() {},
	})
	defer h.Delete()
	var hw C.int = -1
	rc := C.pc_encode_synthetic(C.uintptr_t(h), C.int(frames), C.int(width), C.int(height), C.int(force), C.int(rebuildAt), C.int(rebuildW), C.int(rebuildH), &hw)
	syntheticHardware.Store(int32(hw))
	if rc != 0 {
		return nil, statusErr(int(rc), "synthetic encode failed")
	}
	// pc_encode_synthetic completes the session before it returns, so every
	// callback has run and the slices are quiescent.
	return units, errors.Join(errs...)
}

// RED skeleton for Task 2: every method is a stub the tests fail against.

// DisplaySource captures the main display (stub).
type DisplaySource struct{}

var _ Source = (*DisplaySource)(nil)

// NewDisplaySource returns a Source for the main display (stub).
func NewDisplaySource() Source { return &DisplaySource{} }

func (d *DisplaySource) Start(ctx context.Context, cfg Config) error { return nil }
func (d *DisplaySource) Stop() error                                 { return nil }
func (d *DisplaySource) Frames() <-chan AccessUnit                   { return make(chan AccessUnit) }
func (d *DisplaySource) Errors() <-chan error                        { return make(chan error) }
func (d *DisplaySource) SelectSurface(s Surface) error               { return errors.New("not implemented") }
func (d *DisplaySource) ForceKeyframe()                              {}
func (d *DisplaySource) Rebuild(width, height int) error             { return nil }

// CaptureStill returns a JPEG of the main display (stub).
func CaptureStill(ctx context.Context, maxLongEdge int) ([]byte, error) { return nil, nil }

// mapSCKError maps a ScreenCaptureKit error code (stub).
func mapSCKError(code int, msg string) error { return nil }
