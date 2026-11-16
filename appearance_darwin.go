package main

/*
#cgo darwin CFLAGS: -x objective-c
#cgo darwin LDFLAGS: -framework Cocoa

// Implemented in appearance_darwin.m.
void setAppAppearance(int mode);
void setSidebarMaterial(void);
*/
import "C"

// setNativeAppearance sets NSApp.appearance for an Appearance value
// (system|light|dark); anything else follows the system.
func setNativeAppearance(mode string) {
	switch mode {
	case "light":
		C.setAppAppearance(1)
	case "dark":
		C.setAppAppearance(2)
	default:
		C.setAppAppearance(0)
	}
}

// applySidebarMaterial gives the translucent window's effect view the native
// Sidebar material.
func applySidebarMaterial() {
	C.setSidebarMaterial()
}
