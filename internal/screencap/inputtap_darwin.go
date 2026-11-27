package screencap

// inputtap_darwin.go: the cgo side of inputtap_darwin.m. The tap tells the
// person at the Mac from Portty's own posts under the existing Accessibility
// grant. StartInputTap refuses under testing.Testing(), so no test ever
// installs an event tap on the host.

/*
#cgo darwin LDFLAGS: -framework Carbon -framework CoreGraphics -framework CoreFoundation

#include <stdint.h>

int pc_tap_start(void);
void pc_tap_stop(void);
void pc_tap_state(int rebuild, int *running, int *enabled);
uint64_t pc_tap_now_ns(void);
uint64_t pc_tap_last_untagged_ns(void);
uint64_t pc_tap_last_disabled_ns(void);
int pc_secure_input_on(void);
*/
import "C"

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

// ErrInputTapInTests is returned by StartInputTap under testing.Testing().
var ErrInputTapInTests = errors.New("screencap: input tap refused in tests")

// tapSettle is how long after a disable event the tap is still not trusted.
const tapSettle = time.Second

// StartInputTap starts the listen-only tap (a no-op while one runs). It fails
// when the tap cannot be created or comes up disabled, which is what a missing
// Accessibility grant looks like.
func StartInputTap() error {
	if testing.Testing() {
		return ErrInputTapInTests
	}
	if rc := C.pc_tap_start(); rc != 0 {
		return fmt.Errorf("screencap: starting the input tap failed (code %d)", int32(rc))
	}
	return nil
}

// StopInputTap stops the tap and forgets what it saw.
func StopInputTap() { C.pc_tap_stop() }

// InputTapState reports whether a tap is running and whether the system has it
// enabled right now. A tap disabled by user input is recreated here; the
// manager's 1 s poll is the one caller that should do that.
func InputTapState() (running, enabled bool) { return tapState(1) }

func tapState(rebuild C.int) (running, enabled bool) {
	var r, e C.int
	C.pc_tap_state(rebuild, &r, &e)
	return r != 0, e != 0
}

// PhysicalReading is what the tap knows about the person at the Mac.
type PhysicalReading struct {
	// Healthy is true only while the tap runs, is enabled now, and has seen no
	// disable event for tapSettle; without it "no input seen" proves nothing.
	Healthy bool
	// Seen is true once any untagged event was stored since the tap started.
	Seen bool
	// Idle is the time since the last untagged event (0 when not Seen).
	Idle time.Duration
}

// PhysicalState reads the tap. It is cheap and safe from any goroutine: it
// never rebuilds a dead tap (it runs on every input event, under the arbiter's
// lock), so a dead tap stays unhealthy until the next poll recreates it.
func PhysicalState() PhysicalReading {
	running, enabled := tapState(0)
	last := uint64(C.pc_tap_last_untagged_ns())
	disabled := uint64(C.pc_tap_last_disabled_ns())
	now := uint64(C.pc_tap_now_ns())
	r := PhysicalReading{
		Healthy: running && enabled && (disabled == 0 || time.Duration(now-disabled) >= tapSettle),
	}
	if last != 0 {
		r.Seen = true
		if now > last {
			r.Idle = time.Duration(now - last)
		}
	}
	return r
}

// SecureInputOn reports whether Secure Event Input is on. While it is, the tap
// gets no key events, so the keyboard half of physical-wins is blind.
func SecureInputOn() bool { return C.pc_secure_input_on() != 0 }
