package main

// bridge_darwin.go: the cgo side of the media path. Every C prototype and the
// exported callback live here; the ObjC lives in media_darwin.m and
// sys_darwin.m. Frames stay inside ObjC from source to encoder
// (zero copy) and cross into Go once per access unit.
//
// ARC is on for the spike's ObjC (-fobjc-arc), unlike the repo's non-ARC .m
// files such as appearance_darwin.m.

/*
#include <stdint.h>
#include <stdlib.h>
#cgo darwin CFLAGS: -x objective-c -fobjc-arc -mmacosx-version-min=14.0
#cgo darwin LDFLAGS: -framework Foundation -framework CoreMedia -framework CoreVideo -framework VideoToolbox -framework ScreenCaptureKit -framework CoreGraphics -framework ImageIO -framework ApplicationServices -framework IOKit -framework ServiceManagement -framework AudioToolbox -framework AppKit

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

// Opus and system audio, also in media_darwin.m (JSON results as above).
const char *spk_opus_sine(int seconds);
const char *spk_audio_stats(void);

// Encode latency of the samples since a previous "next" (media_darwin.m).
const char *spk_sck_lat_since(long since);

// Implemented in hostrun_darwin.m (host-run: test chart, app loop, process accounting).
void spk_chart_show(int show);
void spk_chart_motion(int on);
void spk_app_run(void);
void spk_app_stop(void);
int spk_display_asleep(void);
int spk_rusage_ticks(uint64_t *user, uint64_t *sys, uint64_t *rss);
void spk_mach_timebase(uint32_t *numer, uint32_t *denom);

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

// startSCKAudio starts the main-display capture with system audio on, small and
// slow on the video side: opusprobe measures the audio path, not the picture.
func startSCKAudio() error {
	if rc := C.spk_sck_start(640, 5, 1); rc != 0 {
		return fmt.Errorf("spk_sck_start with audio: code %d (%s)", int(rc), sckStats())
	}
	return nil
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

func opusSine(seconds int) string { return C.GoString(C.spk_opus_sine(C.int(seconds))) }

func audioStats() string { return C.GoString(C.spk_audio_stats()) }

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

// userActivity declares user activity, which wakes a sleeping display; the IOReturn, 0 on success.
func userActivity() int { return int(C.spk_user_activity()) }

// displayAwake holds or releases a no-display-sleep assertion; the IOReturn.
func displayAwake(on bool) int { return int(C.spk_display_awake(cBool(on))) }

// latSince returns the pinned latency-window JSON for the samples after since.
func latSince(since int64) string { return C.GoString(C.spk_sck_lat_since(C.long(since))) }

func chartShow(on bool) { C.spk_chart_show(cBool(on)) }

func chartMotion(on bool) { C.spk_chart_motion(cBool(on)) }

// appRun blocks running NSApplication on the calling thread (the main one) until appStop.
func appRun() { C.spk_app_run() }

func appStop() { C.spk_app_stop() }

func displayAsleep() bool { return C.spk_display_asleep() != 0 }

func cBool(b bool) C.int {
	if b {
		return 1
	}
	return 0
}

// rusageTicks returns this process's user and system CPU time (mach ticks) and resident bytes.
func rusageTicks() (user, sys, rss uint64, err error) {
	var u, s, r C.uint64_t
	if rc := C.spk_rusage_ticks(&u, &s, &r); rc != 0 {
		return 0, 0, 0, fmt.Errorf("proc_pid_rusage failed (code %d)", int(rc))
	}
	return uint64(u), uint64(s), uint64(r), nil
}

// machTimebase returns the mach_timebase_info numerator and denominator.
func machTimebase() (numer, denom uint32) {
	var n, d C.uint32_t
	C.spk_mach_timebase(&n, &d)
	return uint32(n), uint32(d)
}
