package main

/*
#cgo darwin CFLAGS: -x objective-c
#cgo darwin LDFLAGS: -framework Cocoa

// Implemented in window_occlusion_darwin.m.
void installWindowOcclusionObserver(void);
*/
import "C"

import (
	"testing"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// windowOccludedEvent carries one boolean: true when the main window is fully covered or hidden.
// App.tsx re-dispatches it as the window event portty:window-hidden, which the Desktop tab folds into
// its hidden-window pause (about 2 minutes hidden stops the stream so the office display can sleep).
const windowOccludedEvent = "window:occluded"

// windowOccludedEmit is a seam so a test can record the event without a Wails runtime.
var windowOccludedEmit = func(hidden bool) {
	if appCtx != nil {
		runtime.EventsEmit(appCtx, windowOccludedEvent, hidden)
	}
}

// startWindowOcclusionObserver begins reporting the main window's occlusion. Call it once the window
// exists (startup). It does nothing under go test: no window, no notification centre traffic.
func startWindowOcclusionObserver() {
	if testing.Testing() {
		return
	}
	C.installWindowOcclusionObserver()
}

func emitWindowOcclusion(hidden bool) { windowOccludedEmit(hidden) }

// porttyWindowOcclusionChanged is called on the main thread by the observer in window_occlusion_darwin.m.
//
//export porttyWindowOcclusionChanged
func porttyWindowOcclusionChanged(hidden C.int) {
	emitWindowOcclusion(hidden != 0)
}
