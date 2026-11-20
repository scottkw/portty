package main

/*
#cgo darwin CFLAGS: -x objective-c
#cgo darwin LDFLAGS: -framework Cocoa

// Implemented in terminate_darwin.m.
void installTerminateHook(void);
*/
import "C"

// installTerminateHook lets a macOS logout, restart or shutdown quit the GUI
// without the quit dialog (terminate_darwin.m). Call it once the app delegate
// exists (startup).
func installTerminateHook() {
	C.installTerminateHook()
}

// porttySystemWillPowerOff is called on the main thread by the terminate hook
// right before it lets AppKit terminate the app.
//
//export porttySystemWillPowerOff
func porttySystemWillPowerOff() {
	if appInstance != nil {
		appInstance.onSystemPowerOff()
	}
}
