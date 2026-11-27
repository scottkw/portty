package screencap

// unlock_darwin.go: the cgo side of unlock_darwin.m, the narrow native bridge
// remote unlock types through. The layout map and the Caps Lock read only read
// and run anywhere; wake, type and return post events, so each refuses under
// testing.Testing() and no test ever wakes the host or types into it. Nothing
// here logs, wraps or formats a buffer, a stroke or a character: errors carry a
// fixed phrase and a code.

/*
#cgo darwin LDFLAGS: -framework Carbon -framework CoreGraphics -framework CoreFoundation

#include <stdint.h>

// Same layout as KeyStroke below.
typedef struct {
	uint16_t vk;
	uint64_t flags;
} pc_stroke;

int pc_unlock_map(const uint8_t *utf8, int n, pc_stroke *out, int *count);
int pc_unlock_translate(uint16_t vk, uint64_t flags, uint32_t *cp);
int pc_unlock_caps_lock(int *on);
int pc_unlock_wake(uint16_t vk);
int pc_unlock_key(uint16_t vk, uint64_t flags);
*/
import "C"

import (
	"errors"
	"fmt"
	"testing"
	"unsafe"
)

// ErrUnmappable means a character has no key on the current keyboard layout (or
// the bytes are not valid UTF-8, or not printable). Nothing was typed.
var ErrUnmappable = errors.New("screencap: a character has no key on this layout")

// kVK_Return, the key that submits the lock screen's password field.
const vkReturn = 0x24

// KeyStroke is one key press that types one character: the virtual key and the
// CGEventFlags modifier mask (Shift, Option) the layout needs for it.
type KeyStroke struct {
	VK    uint16
	Flags uint64
}

// UnlockBridge maps a password to key strokes and types them at the lock screen.
// The zero value works.
type UnlockBridge struct{}

// NewUnlockBridge returns the bridge to the real event system.
func NewUnlockBridge() *UnlockBridge { return &UnlockBridge{} }

func unlockErr(what string, rc C.int) error {
	return fmt.Errorf("screencap: %s failed (code %d)", what, int32(rc))
}

// MapRunes maps the UTF-8 bytes in b to one stroke per character through the
// current keyboard layout, or returns ErrUnmappable before any stroke when a
// character has no key. b is only read; the native scratch decode buffer is
// zeroed before the call returns.
func (*UnlockBridge) MapRunes(b []byte) ([]KeyStroke, error) {
	if len(b) == 0 {
		return nil, ErrUnmappable
	}
	out := make([]KeyStroke, len(b)) // at most one character per byte
	var n C.int
	rc := C.pc_unlock_map((*C.uint8_t)(unsafe.Pointer(&b[0])), C.int(len(b)), (*C.pc_stroke)(unsafe.Pointer(&out[0])), &n)
	switch {
	case rc == unlockUnmappableRC:
		clear(out)
		return nil, ErrUnmappable
	case rc != 0:
		clear(out)
		return nil, unlockErr("reading the keyboard layout", rc)
	}
	return out[:n:n], nil
}

// unlockUnmappableRC is pc_unlock_map's code for a character with no key.
const unlockUnmappableRC = 1

// unlockTranslate runs one stroke back through UCKeyTranslate and returns the
// character it types: the round-trip check of MapRunes.
func unlockTranslate(s KeyStroke) (rune, error) {
	var cp C.uint32_t
	if rc := C.pc_unlock_translate(C.uint16_t(s.VK), C.uint64_t(s.Flags), &cp); rc != 0 {
		return 0, unlockErr("translating a key", rc)
	}
	return rune(cp), nil
}

// CapsLockOn reads whether Caps Lock is on at the Mac. It posts nothing.
func (*UnlockBridge) CapsLockOn() (bool, error) {
	var on C.int
	if rc := C.pc_unlock_caps_lock(&on); rc != 0 {
		return false, unlockErr("reading Caps Lock", rc)
	}
	return on != 0, nil
}

// SessionLocked is the uncached lock read (sys_darwin.go).
func (*UnlockBridge) SessionLocked() (bool, error) { return SessionLocked() }

// Wake declares remote user activity (it lights a dark display) and then posts
// one press of vk, a key that types no character.
func (*UnlockBridge) Wake(vk uint16) error {
	if testing.Testing() {
		return ErrRefusedInTest
	}
	if rc := C.pc_unlock_wake(C.uint16_t(vk)); rc != 0 {
		return unlockErr("waking the screen", rc)
	}
	return nil
}

// Type presses and releases one stroke's key, 60 ms apart, at the HID tap.
func (*UnlockBridge) Type(s KeyStroke) error {
	if testing.Testing() {
		return ErrRefusedInTest
	}
	if rc := C.pc_unlock_key(C.uint16_t(s.VK), C.uint64_t(s.Flags)); rc != 0 {
		return unlockErr("typing a key", rc)
	}
	return nil
}

// Return presses and releases the Return key.
func (*UnlockBridge) Return() error {
	if testing.Testing() {
		return ErrRefusedInTest
	}
	if rc := C.pc_unlock_key(vkReturn, 0); rc != 0 {
		return unlockErr("pressing Return", rc)
	}
	return nil
}
