package relay

// DEBT-01: vt gives the alternate screen its own 10,000-line scrollback
// (NewScreen), and no exported API reaches it: SetScrollbackSize, Scrollback
// and ClearScrollback all use the main screen. Nothing reads it either (the
// hub drains the main scrollback and renders the alt screen's buffer), yet a
// full-width TUI scrolling in the alt screen fills it, about 10,000 x cols x
// 112 B per session until the app exits.
//
// So the hub disables it once, at construction, with the exported
// Screen.SetScrollback(nil) on the alt Screen, which it reaches through the
// unexported scrs field. Reflection rather than a vt fork: one field of a
// pinned module, type-checked to be exactly [2]vt.Screen before the cast. vt
// nil-checks the scrollback everywhere it pushes, and RIS and Resize do not
// recreate it. TestVTLayout_AltScreenScrollback pins the vt version and this
// layout, so any vt bump forces a re-review of this file.

import (
	"reflect"
	"unsafe"

	xvt "github.com/charmbracelet/x/vt"
)

// altScreenOf returns emu's alternate Screen, or nil if vt's layout changed.
func altScreenOf(emu *xvt.Emulator) *xvt.Screen {
	f := reflect.ValueOf(emu).Elem().FieldByName("scrs")
	if !f.IsValid() || f.Type() != reflect.TypeOf([2]xvt.Screen{}) {
		return nil
	}
	return (*xvt.Screen)(unsafe.Pointer(f.Index(1).UnsafeAddr()))
}

// disableAltScreenScrollback turns off emu's alt-screen scrollback and reports
// whether it could.
func disableAltScreenScrollback(emu *xvt.Emulator) bool {
	alt := altScreenOf(emu)
	if alt == nil {
		return false
	}
	alt.SetScrollback(nil)
	return true
}
