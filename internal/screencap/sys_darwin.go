package screencap

// sys_darwin.go: the cgo side of sys_darwin.m. The three reads (lock, display
// asleep, AC power) are harmless and run anywhere; everything that holds the
// Mac awake or posts an event refuses under testing.Testing(), so no test ever
// changes the host's power state or moves its pointer.

/*
#cgo darwin LDFLAGS: -framework IOKit -framework CoreGraphics -framework ApplicationServices -framework CoreFoundation

#include <stdint.h>
#include <stdlib.h>

int pc_session_locked(int *locked);
int pc_display_asleep(int *asleep);
int pc_on_ac_power(int *ac);
int pc_hold_display_awake(const char *name, uint32_t *id);
int pc_hold_system_awake(const char *name, uint32_t *id);
int pc_release_assertion(uint32_t id);
int pc_declare_remote_activity(const char *name);
int pc_pointer_state(double *x, double *minX, double *maxX);
int pc_pointer_nudge(double dx, int *ok);
*/
import "C"

import (
	"fmt"
	"testing"
	"unsafe"
)

func sysErr(what string, rc C.int) error {
	return fmt.Errorf("screencap: %s failed (code %#x)", what, uint32(rc))
}

// readFlag runs one of the C reads that fill an int flag.
func readFlag(what string, read func(*C.int) C.int) (bool, error) {
	var v C.int
	if rc := read(&v); rc != 0 {
		return false, sysErr(what, rc)
	}
	return v != 0, nil
}

// SessionLocked reports whether the login session's screen is locked. A Mac
// with no window server session is an error, not "unlocked".
func SessionLocked() (bool, error) {
	return readFlag("reading the session lock state", func(v *C.int) C.int { return C.pc_session_locked(v) })
}

// DisplayAsleep reports whether the main display is asleep.
func DisplayAsleep() (bool, error) {
	return readFlag("reading the display state", func(v *C.int) C.int { return C.pc_display_asleep(v) })
}

// OnACPower reports whether the Mac is powered from the wall.
func OnACPower() (bool, error) {
	return readFlag("reading the power source", func(v *C.int) C.int { return C.pc_on_ac_power(v) })
}

// PowerAssertion is a held IOPM assertion; Release gives it back.
type PowerAssertion struct{ id uint32 }

func hold(name string, take func(*C.char, *C.uint32_t) C.int) (PowerAssertion, error) {
	if testing.Testing() {
		return PowerAssertion{}, ErrRefusedInTest
	}
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	var id C.uint32_t
	if rc := take(cname, &id); rc != 0 {
		return PowerAssertion{}, sysErr("taking a power assertion", rc)
	}
	return PowerAssertion{id: uint32(id)}, nil
}

// HoldDisplayAwake keeps the display from idling off until Release. It does
// not light a display that is already dark (see DeclareRemoteUserActivity).
// name shows in `pmset -g assertions`.
func HoldDisplayAwake(name string) (PowerAssertion, error) {
	return hold(name, func(n *C.char, id *C.uint32_t) C.int { return C.pc_hold_display_awake(n, id) })
}

// HoldSystemAwake keeps the system from idle sleep until Release; the display
// may still dim. name shows in `pmset -g assertions`.
func HoldSystemAwake(name string) (PowerAssertion, error) {
	return hold(name, func(n *C.char, id *C.uint32_t) C.int { return C.pc_hold_system_awake(n, id) })
}

// Release gives the assertion back. The zero PowerAssertion is a no-op.
func (p PowerAssertion) Release() error {
	if p.id == 0 {
		return nil
	}
	if rc := C.pc_release_assertion(C.uint32_t(p.id)); rc != 0 {
		return sysErr("releasing a power assertion", rc)
	}
	return nil
}

// DeclareRemoteUserActivity tells the system a remote user is active, which
// lights a sleeping display and postpones its sleep up to the Energy Saver
// timer. The held display assertion, not this call, keeps it on.
func DeclareRemoteUserActivity(name string) error {
	if testing.Testing() {
		return ErrRefusedInTest
	}
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	if rc := C.pc_declare_remote_activity(cname); rc != 0 {
		return sysErr("declaring remote user activity", rc)
	}
	return nil
}

// nudgeDelta picks the direction of a one-point pointer nudge: right, unless
// that would leave the display (x is within a step of the right edge), then left.
func nudgeDelta(x, minX, maxX float64) float64 {
	if x+1 >= maxX && x-1 >= minX {
		return -1
	}
	return 1
}

// NudgePointer moves the pointer one point away from the nearest edge and back,
// and reports whether it moved and came back to where it started. false with no
// error means the move was blocked (no permission to post events) or the return
// did not land. Used only by the stall watchdog, only while a viewer is waiting.
func NudgePointer() (bool, error) {
	if testing.Testing() {
		return false, ErrRefusedInTest
	}
	var x, minX, maxX C.double
	if rc := C.pc_pointer_state(&x, &minX, &maxX); rc != 0 {
		return false, sysErr("reading the pointer position", rc)
	}
	var ok C.int
	if rc := C.pc_pointer_nudge(C.double(nudgeDelta(float64(x), float64(minX), float64(maxX))), &ok); rc != 0 {
		return false, sysErr("nudging the pointer", rc)
	}
	return ok != 0, nil
}
