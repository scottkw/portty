package main

// bridge_darwin.go: the cgo side of the media path. Every C prototype and the
// exported callback live here; the ObjC lives in media_darwin.m (and, from the
// next plan, sys_darwin.m). Frames stay inside ObjC from source to encoder
// (zero copy) and cross into Go once per access unit.
//
// ARC is on for the spike's ObjC (-fobjc-arc), unlike the repo's non-ARC .m
// files such as appearance_darwin.m.

/*
#cgo darwin CFLAGS: -x objective-c -fobjc-arc -mmacosx-version-min=14.0
#cgo darwin LDFLAGS: -framework Foundation -framework CoreMedia -framework CoreVideo -framework VideoToolbox

// Implemented in media_darwin.m. Every int result is 0 or an OSStatus.
int spk_encoder_start(int width, int height, int fps, int bitrate);
void spk_encoder_force_keyframe(void);
const char *spk_encoder_info(void);
int spk_synthetic_start(int width, int height, int fps);
void spk_source_stop(void);
*/
import "C"

import (
	"fmt"
	"sync/atomic"
	"unsafe"
)

// accessUnit is one Annex-B H.264 access unit (SPS and PPS first on keyframes).
type accessUnit struct {
	data  []byte
	ptsNs int64
	key   bool
}

// auCh carries access units from the encoder callback to the server's pump.
// A full channel drops the unit and counts it: a live stream prefers a gap to
// unbounded latency, and the count is logged at exit so the gap is visible.
var (
	auCh      = make(chan accessUnit, 64)
	auDropped atomic.Int64
)

//export spkOnAccessUnit
func spkOnAccessUnit(data unsafe.Pointer, length C.int, ptsNs C.longlong, keyframe C.int) {
	au := accessUnit{data: C.GoBytes(data, length), ptsNs: int64(ptsNs), key: keyframe != 0}
	select {
	case auCh <- au:
	default:
		auDropped.Add(1)
	}
}

func startEncoder(width, height, fps, bitrate int) error {
	if rc := C.spk_encoder_start(C.int(width), C.int(height), C.int(fps), C.int(bitrate)); rc != 0 {
		return fmt.Errorf("spk_encoder_start: OSStatus %d (%s)", int(rc), encoderInfo())
	}
	return nil
}

// startSource begins feeding frames from the named source into the encoder.
func startSource(name string, width, height, fps int) error {
	switch name {
	case "synthetic":
		if rc := C.spk_synthetic_start(C.int(width), C.int(height), C.int(fps)); rc != 0 {
			return fmt.Errorf("spk_synthetic_start: OSStatus %d (%s)", int(rc), encoderInfo())
		}
		return nil
	default:
		return fmt.Errorf("unknown source %q (known: synthetic)", name)
	}
}

func stopSource() { C.spk_source_stop() }

func forceKeyframe() { C.spk_encoder_force_keyframe() }

// encoderInfo returns the pinned `encoder` JSON object.
func encoderInfo() string { return C.GoString(C.spk_encoder_info()) }
