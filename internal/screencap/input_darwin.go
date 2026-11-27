package screencap

// input_darwin.go: the cgo side of input_darwin.m. Reads (cursor, display
// bounds) are harmless and run anywhere; every function that posts an event
// refuses under testing.Testing(), so no test ever moves the host's pointer or
// types into it.

/*
#include <stdint.h>

int64_t pc_event_marker(void);
int pc_input_mouse(int kind, double x, double y, int button, int clickState, int dx, int dy, uint64_t flags);
int pc_input_cursor(double *x, double *y);
int pc_display_bounds(uint32_t id, double *x, double *y, double *w, double *h);
*/
import "C"

import (
	"fmt"
	"testing"
)

// EventMarker is written to kCGEventSourceUserData of every posted event
// (ASCII "Portty"), so injected input can be told from real input.
const EventMarker int64 = 0x506F72747479

// MaxTextUnits is the most UTF-16 units one PostText call posts.
const MaxTextUnits = 20

// MouseKind is what a posted pointer event does.
type MouseKind int

// The pointer event kinds; the values are the kind codes of pc_input_mouse.
const (
	MouseMoved MouseKind = iota
	MouseDown
	MouseUp
	MouseDragged
)

// InputBridge posts remote input as the user at the Mac. The zero value works.
type InputBridge struct{}

// NewInputBridge returns the bridge to the real event system.
func NewInputBridge() *InputBridge { return &InputBridge{} }

func inputErr(what string, rc C.int) error {
	return fmt.Errorf("screencap: %s failed (code %d)", what, int32(rc))
}

// eventMarkerC is the marker the C side writes, for the test that keeps both
// copies equal.
func eventMarkerC() int64 { return int64(C.pc_event_marker()) }

// PostMouse posts one pointer event at the global point (x, y) in points.
// button is 0 left, 1 right, 2 other; clickState counts clicks on down and up;
// dx and dy are the movement since the previous event; flags are the explicit
// CGEventFlags modifier mask.
func (*InputBridge) PostMouse(kind MouseKind, x, y float64, button, clickState, dx, dy int, flags uint64) error {
	if testing.Testing() {
		return ErrRefusedInTest
	}
	if rc := C.pc_input_mouse(C.int(kind), C.double(x), C.double(y), C.int(button), C.int(clickState), C.int(dx), C.int(dy), C.uint64_t(flags)); rc != 0 {
		return inputErr("posting a pointer event", rc)
	}
	return nil
}

// CursorLocation reads the pointer's global location in points.
func (*InputBridge) CursorLocation() (x, y float64, err error) {
	var cx, cy C.double
	if rc := C.pc_input_cursor(&cx, &cy); rc != 0 {
		return 0, 0, inputErr("reading the pointer position", rc)
	}
	return float64(cx), float64(cy), nil
}

// DisplayBounds returns a display's bounds in global points (top-left origin,
// negative for displays left of or above the main one). id 0 is the main display.
func DisplayBounds(id uint32) (x, y, w, h float64, err error) {
	var cx, cy, cw, ch C.double
	if rc := C.pc_display_bounds(C.uint32_t(id), &cx, &cy, &cw, &ch); rc != 0 {
		return 0, 0, 0, 0, inputErr("reading the display bounds", rc)
	}
	return float64(cx), float64(cy), float64(cw), float64(ch), nil
}
