package screencap

// window_darwin.go: the cgo side of window_darwin.m, what the window gate of a
// window share reads and does: the live Accessibility focus, a window's frame,
// whether it is minimized, a raise and the topmost window at a point (id, owner,
// layer and frame). Every
// method refuses under testing.Testing(), so no test reads or changes
// Accessibility state on the host.

/*
#cgo darwin LDFLAGS: -framework AppKit -framework ApplicationServices
#include <stdint.h>
#include <stdlib.h>

int pc_ax_focus(int *pid, double *x, double *y, double *w, double *h, int *ok);
int pc_ax_key_focus(int *pid, double *x, double *y, double *w, double *h, int *ok);
int pc_window_info(uint32_t id, int *pid, double *x, double *y, double *w, double *h, int *onScreen);
int pc_window_minimized(int pid, double x, double y, double w, double h, const char *title, int *minimized);
int pc_window_raise(int pid, double x, double y, double w, double h, const char *title);
int pc_window_topmost(double x, double y, uint32_t *id, int *pid, int *layer, double *fx, double *fy, double *fw, double *fh);
*/
import "C"

import (
	"testing"
	"unsafe"
)

// WindowBridge reads and raises windows. The zero value works.
type WindowBridge struct{}

// NewWindowBridge returns the bridge to the real window system.
func NewWindowBridge() *WindowBridge { return &WindowBridge{} }

// RungName names a raise rung for the log: 1 Accessibility, 2 NSWorkspace, 3 activate.
func RungName(rung int) string {
	switch rung {
	case 1:
		return "ax"
	case 2:
		return "workspace"
	case 3:
		return "activate"
	}
	return "none"
}

// Focus reads, live, the focused application's pid and its focused window's
// frame in global points. ok is false when no window has focus. It never asks
// NSWorkspace: its frontmost application lags a real change.
func (*WindowBridge) Focus() (pid int32, x, y, w, h float64, ok bool, err error) {
	if testing.Testing() {
		return 0, 0, 0, 0, 0, false, ErrRefusedInTest
	}
	var cp, cok C.int
	var cx, cy, cw, ch C.double
	if rc := C.pc_ax_focus(&cp, &cx, &cy, &cw, &ch, &cok); rc != 0 {
		return 0, 0, 0, 0, 0, false, bridgeErr("reading the focused window", rc)
	}
	return int32(cp), float64(cx), float64(cy), float64(cw), float64(ch), cok != 0, nil
}

// KeyFocus reads, live, the owner pid of the element that holds keyboard focus
// (the system-wide focused UI element) and the frame of that element's window,
// in global points. It differs from Focus when a non-activating panel
// (Spotlight, Alfred, Raycast) holds key focus while the focused application is
// unchanged. ok is false when there is no element or window, which a caller
// treats as no.
func (*WindowBridge) KeyFocus() (pid int32, x, y, w, h float64, ok bool, err error) {
	if testing.Testing() {
		return 0, 0, 0, 0, 0, false, ErrRefusedInTest
	}
	var cp, cok C.int
	var cx, cy, cw, ch C.double
	if rc := C.pc_ax_key_focus(&cp, &cx, &cy, &cw, &ch, &cok); rc != 0 {
		return 0, 0, 0, 0, 0, false, bridgeErr("reading the key focus", rc)
	}
	return int32(cp), float64(cx), float64(cy), float64(cw), float64(ch), cok != 0, nil
}

// WindowInfo reads a window's owner pid and frame in global points and whether
// it is on screen. A window that no longer exists is an error.
func (*WindowBridge) WindowInfo(id uint32) (pid int32, x, y, w, h float64, onScreen bool, err error) {
	if testing.Testing() {
		return 0, 0, 0, 0, 0, false, ErrRefusedInTest
	}
	var cp, on C.int
	var cx, cy, cw, ch C.double
	if rc := C.pc_window_info(C.uint32_t(id), &cp, &cx, &cy, &cw, &ch, &on); rc != 0 {
		return 0, 0, 0, 0, 0, false, bridgeErr("reading a window", rc)
	}
	return int32(cp), float64(cx), float64(cy), float64(cw), float64(ch), on != 0, nil
}

// Minimized reports whether the window of app pid with this frame (within 2
// points; the title breaks a tie) is minimized.
func (*WindowBridge) Minimized(pid int32, x, y, w, h float64, title string) (bool, error) {
	if testing.Testing() {
		return false, ErrRefusedInTest
	}
	ct := C.CString(title)
	defer C.free(unsafe.Pointer(ct))
	var min C.int
	if rc := C.pc_window_minimized(C.int(pid), C.double(x), C.double(y), C.double(w), C.double(h), ct, &min); rc != 0 {
		return false, bridgeErr("reading the minimized state", rc)
	}
	return min != 0, nil
}

// Raise brings the window to the front: Accessibility, then NSWorkspace, then
// NSRunningApplication, reading focus back after each. It returns the rung
// (1..3) that confirmed focus, and an error when none did. It blocks for up to
// about a second, so callers run it off the input path.
func (*WindowBridge) Raise(pid int32, x, y, w, h float64, title string) (rung int, err error) {
	if testing.Testing() {
		return 0, ErrRefusedInTest
	}
	ct := C.CString(title)
	defer C.free(unsafe.Pointer(ct))
	rc := C.pc_window_raise(C.int(pid), C.double(x), C.double(y), C.double(w), C.double(h), ct)
	if rc <= 0 {
		return 0, bridgeErr("raising a window", rc)
	}
	return int(rc), nil
}

// TopWindow is the topmost on-screen window at a point: its ID, owner pid,
// window layer (0 is a normal window; menus and pop-ups sit above) and frame.
type TopWindow struct {
	ID         uint32
	PID, Layer int32
	X, Y, W, H float64
}

// TopmostAt returns the topmost on-screen window at a global point (ID 0 when
// none is there).
func (*WindowBridge) TopmostAt(x, y float64) (TopWindow, error) {
	if testing.Testing() {
		return TopWindow{}, ErrRefusedInTest
	}
	var id C.uint32_t
	var pid, layer C.int
	var fx, fy, fw, fh C.double
	if rc := C.pc_window_topmost(C.double(x), C.double(y), &id, &pid, &layer, &fx, &fy, &fw, &fh); rc != 0 {
		return TopWindow{}, bridgeErr("finding the topmost window", rc)
	}
	return TopWindow{uint32(id), int32(pid), int32(layer), float64(fx), float64(fy), float64(fw), float64(fh)}, nil
}
