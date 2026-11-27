package main

/*
#cgo darwin CFLAGS: -x objective-c
#cgo darwin LDFLAGS: -framework Cocoa

// Implemented in window_occlusion_darwin.m.
void installWindowOcclusionObserver(void);
*/
import "C"

import (
	"sync"
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

// occlusionQueue and occlusionPump hand each change from the Cocoa callback to one goroutine that
// emits in order. Emitting inside the callback killed the real app with SIGSEGV at launch (21-11 VM
// row D1): the callback runs on the main thread inside a cgo call, and Wails' EventsEmit from there
// crashed in system code. Wails' own native callbacks (menus) also run user code off the main thread.
// shortcut: a 16-deep buffer, a full one would block the main thread; a window does not change
// occlusion that fast, upgrade to a coalescing latest-value slot if it ever does.
var (
	occlusionQueue = make(chan bool, 16)
	occlusionPump  sync.Once
)

func emitWindowOcclusion(hidden bool) {
	occlusionPump.Do(func() {
		go func() {
			for h := range occlusionQueue {
				windowOccludedEmit(h)
			}
		}()
	})
	occlusionQueue <- hidden
}

// porttyWindowOcclusionChanged is called on the main thread by the observer in window_occlusion_darwin.m.
//
//export porttyWindowOcclusionChanged
func porttyWindowOcclusionChanged(hidden C.int) {
	emitWindowOcclusion(hidden != 0)
}
