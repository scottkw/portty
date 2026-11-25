package main

// bridge_darwin.go: the cgo side of the media path. Every C prototype and the
// exported callback live here; the ObjC lives in media_darwin.m and
// sys_darwin.m. Frames stay inside ObjC from source to encoder
// (zero copy) and cross into Go once per access unit.
//
// ARC is on for the spike's ObjC (-fobjc-arc), unlike the repo's non-ARC .m
// files such as appearance_darwin.m.

/*
#include <stdlib.h>
#cgo darwin CFLAGS: -x objective-c -fobjc-arc -mmacosx-version-min=14.0
#cgo darwin LDFLAGS: -framework Foundation -framework CoreMedia -framework CoreVideo -framework VideoToolbox -framework ScreenCaptureKit -framework CoreGraphics -framework ImageIO -framework ApplicationServices -framework IOKit -framework ServiceManagement

// Implemented in media_darwin.m. Every int result is 0 or an OSStatus.
int spk_encoder_start(int width, int height, int fps, int bitrate);
void spk_encoder_force_keyframe(void);
const char *spk_encoder_info(void);
int spk_synthetic_start(int width, int height, int fps);
void spk_source_stop(void);

// Capture, also in media_darwin.m. The const char * results are JSON that stay
// valid until the next call of the same function; Go copies them at once.
int spk_sck_start(int widthCap, int fps, int withAudio);
void spk_request_refresh(void);
const char *spk_sck_stats(void);
double spk_dump_png(const char *path);
const char *spk_png_error(void);
const char *spk_encoder_list(void);
const char *spk_displays(void);

// Implemented in sys_darwin.m.
int spk_sr_preflight(void);
int spk_sr_request(void);
int spk_ax_trusted(int prompt);
int spk_post_event_preflight(void);
int spk_post_event_request(void);
int spk_mouse_nudge(void);
int spk_post_keycodes(const int *codes, int n);
const char *spk_session_dict(void);
int spk_user_activity(void);
int spk_display_awake(int on);
const char *spk_smapp(const char *verb, const char *plistName);
*/
import "C"

import (
	"fmt"
	"sync/atomic"
	"unsafe"
)

// sckWidthCap is the long-edge cap for `serve --source sck`: the plan's 2560 px.
const sckWidthCap = 2560

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
	case "sck":
		// The capture size comes from the display, so spk_sck_start sizes the
		// encoder itself; width and height here are only the synthetic default.
		if rc := C.spk_sck_start(C.int(sckWidthCap), C.int(fps), 0); rc != 0 {
			return fmt.Errorf("spk_sck_start: code %d (%s)", int(rc), sckStats())
		}
		return nil
	default:
		return fmt.Errorf("unknown source %q (known: synthetic, sck)", name)
	}
}

func stopSource() { C.spk_source_stop() }

// forceKeyframe answers a PLI, FIR or viewer join. With a capture stream it
// re-sends the cached picture as an IDR (a still screen produces no frames);
// otherwise the next frame becomes one.
func forceKeyframe() { C.spk_request_refresh() }

// encoderInfo returns the pinned `encoder` JSON object.
func encoderInfo() string { return C.GoString(C.spk_encoder_info()) }

// Each of these returns a pinned JSON value (see the plan's Pinned JSON keys).

func sckStats() string { return C.GoString(C.spk_sck_stats()) }

func encoderList() string { return C.GoString(C.spk_encoder_list()) }

func displayList() string { return C.GoString(C.spk_displays()) }

func srPreflight() bool { return C.spk_sr_preflight() != 0 }

func srRequest() bool { return C.spk_sr_request() != 0 }

// dumpPNG writes the cached frame and returns its mean luma 0-255.
func dumpPNG(path string) (float64, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	luma := float64(C.spk_dump_png(cPath))
	if luma < 0 {
		return 0, fmt.Errorf("%s (code %d)", C.GoString(C.spk_png_error()), int(luma))
	}
	return luma, nil
}

// The next four return 1 or 0, and -1 when the call itself could not be made.

func axTrusted(prompt bool) int {
	p := 0
	if prompt {
		p = 1
	}
	return int(C.spk_ax_trusted(C.int(p)))
}

func postEventPreflight() int { return int(C.spk_post_event_preflight()) }

func postEventRequest() int { return int(C.spk_post_event_request()) }

func mouseNudge() int { return int(C.spk_mouse_nudge()) }

// postKeycodes posts key down and up per code; 0 on success.
func postKeycodes(codes []int) int {
	if len(codes) == 0 {
		return 0
	}
	cs := make([]C.int, len(codes))
	for i, c := range codes {
		cs[i] = C.int(c)
	}
	return int(C.spk_post_keycodes(&cs[0], C.int(len(cs))))
}

func sessionDict() string { return C.GoString(C.spk_session_dict()) }

// smapp runs register, unregister or status on an SMAppService agent plist and
// returns its JSON ({"status": ..., "error": ...}).
func smapp(verb, plist string) string {
	cv, cp := C.CString(verb), C.CString(plist)
	defer C.free(unsafe.Pointer(cv))
	defer C.free(unsafe.Pointer(cp))
	return C.GoString(C.spk_smapp(cv, cp))
}
