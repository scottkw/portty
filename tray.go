//go:build darwin

package main

/*
#cgo darwin CFLAGS: -x objective-c
#cgo darwin LDFLAGS: -framework Cocoa

#include <stdlib.h>

// Function declarations — implementations live in tray_objc.m (compiled once
// by cgo to avoid duplicate ObjC class symbols during go test linking).
void initStatusItem(const void *d1, int n1, const void *d2, int n2);
void removeStatusItem(void);
void updateTrayIcon(const void *d1, int n1, const void *d2, int n2);
void updateTrayTooltip(const char *tooltip);
void setTraySessionData(const char **names, const char **ids, int count);
void setTrayUpdateItem(const char *title);
void setTrayDesktopTitle(const char *title);
void setTrayShareState(int active, const char *title);
void setDockVisible(int visible);
*/
import "C"

import (
	_ "embed"
	"unsafe"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed assets/tray_icon.png
var trayIconBytes []byte

//go:embed assets/tray_icon@2x.png
var trayIconBytes2x []byte

//go:embed assets/tray_icon_error.png
var trayIconErrorBytes []byte

//go:embed assets/tray_icon_error@2x.png
var trayIconErrorBytes2x []byte

//go:embed assets/tray_icon_progress_25.png
var trayIconProgress25Bytes []byte

//go:embed assets/tray_icon_progress_25@2x.png
var trayIconProgress25Bytes2x []byte

//go:embed assets/tray_icon_progress_50.png
var trayIconProgress50Bytes []byte

//go:embed assets/tray_icon_progress_50@2x.png
var trayIconProgress50Bytes2x []byte

//go:embed assets/tray_icon_progress_75.png
var trayIconProgress75Bytes []byte

//go:embed assets/tray_icon_progress_75@2x.png
var trayIconProgress75Bytes2x []byte

//go:embed assets/tray_icon_progress_100.png
var trayIconProgress100Bytes []byte

//go:embed assets/tray_icon_progress_100@2x.png
var trayIconProgress100Bytes2x []byte

// trayCallbackApp is the global App reference for cgo callbacks.
// Set before initTray returns. Only accessed from the main goroutine.
var trayCallbackApp *App

//export onTrayShow
func onTrayShow() {
	if trayCallbackApp != nil && trayCallbackApp.ctx != nil {
		runtime.WindowShow(trayCallbackApp.ctx)
		trayCallbackApp.setDockVisible(true)
	}
}

//export onTrayQuit
func onTrayQuit() {
	// Capture the pointer before launching the goroutine to avoid a data race
	// between the goroutine reading trayCallbackApp and tests restoring it.
	app := trayCallbackApp
	go func() {
		if app != nil && app.ctx != nil {
			runtime.WindowShow(app.ctx) // D-08: auto-show if hidden
			app.setDockVisible(true)
			runtime.EventsEmit(app.ctx, "app:quit-requested", nil)
		}
	}()
}

//export onTraySession
func onTraySession(idx C.int) {
	if trayCallbackApp == nil || trayCallbackApp.ctx == nil {
		return
	}
	// Hand off to goroutine — cgo callback thread is not safe for all Wails calls.
	go func() {
		sessions := trayCallbackApp.ListSessions()
		i := int(idx)
		if i >= 0 && i < len(sessions) {
			runtime.WindowShow(trayCallbackApp.ctx)
			trayCallbackApp.setDockVisible(true)
			runtime.EventsEmit(trayCallbackApp.ctx, "tray:focus-session", sessions[i].ID)
		}
	}()
}

//export onTrayUpdate
func onTrayUpdate() {
	app := trayCallbackApp // captured before the goroutine, like onTrayQuit
	if app != nil {
		go app.offerUpdate() // the alert blocks; never on the main thread
	}
}

//export onTrayShareDesktop
func onTrayShareDesktop() {
	app := trayCallbackApp // captured before the goroutine, like onTrayQuit
	if app != nil {
		go app.shareDesktopFromTray() // the menu callback must return at once
	}
}

//export onTrayStopSharing
func onTrayStopSharing() {
	app := trayCallbackApp // captured before the goroutine, like onTrayQuit
	if app != nil {
		go app.stopSharingFromTray()
	}
}

//export onNotificationClick
func onNotificationClick(identifier *C.char) {
	id := C.GoString(identifier) // copy now: the C string is only valid during this callback
	app := trayCallbackApp       // captured before the goroutine, like onTrayQuit
	if app != nil {
		go app.handleNotificationClick(id) // the native callback must return at once
	}
}

// setTrayUpdateItem sets the tray menu's update item title; "" removes the item.
func setTrayUpdateItem(title string) {
	ct := C.CString(title)
	C.setTrayUpdateItem(ct)
	C.free(unsafe.Pointer(ct))
}

// setTrayShareState sets the click-to-share slot of the tray menu: active shows
// title and Stop Sharing, inactive shows Share Desktop….
func setTrayShareState(active bool, title string) {
	ct := C.CString(title)
	a := C.int(0)
	if active {
		a = 1
	}
	C.setTrayShareState(a, ct)
	C.free(unsafe.Pointer(ct))
}

// setTrayDesktopLine sets the tray menu's dimmed desktop line; "" removes it.
func (a *App) setTrayDesktopLine(line string) {
	cl := C.CString(line)
	C.setTrayDesktopTitle(cl)
	C.free(unsafe.Pointer(cl))
}

// setDockVisible shows or hides the macOS Dock icon by toggling the application
// activation policy. Call with true when showing the window, false when hiding.
func (a *App) setDockVisible(visible bool) {
	if visible {
		C.setDockVisible(1)
	} else {
		C.setDockVisible(0)
	}
}

// trayIconBytesForState returns the 1x (18 px) and @2x (36 px) tray icon
// byte slices for the given connection state and current progress quartile
// (Phase 98 PRG-03).
//
// Error precedence (Pitfall #8): when connected=false, always returns
// the error pair regardless of a.lastTrayQuartile to ensure daemon-
// disconnect is not masked by a progress glyph.
func (a *App) trayIconBytesForState(connected bool) (x1, x2 []byte) {
	if !connected {
		return trayIconErrorBytes, trayIconErrorBytes2x
	}
	switch a.lastTrayQuartile.Load() {
	case 1:
		return trayIconProgress25Bytes, trayIconProgress25Bytes2x
	case 2:
		return trayIconProgress50Bytes, trayIconProgress50Bytes2x
	case 3:
		return trayIconProgress75Bytes, trayIconProgress75Bytes2x
	case 4:
		return trayIconProgress100Bytes, trayIconProgress100Bytes2x
	default:
		return trayIconBytes, trayIconBytes2x
	}
}

// updateTray updates the tray icon, tooltip, and session list based on current state.
func (a *App) updateTray(sessions []SessionInfo, connected bool) {
	// Update icon based on connectivity and progress quartile.
	x1, x2 := a.trayIconBytesForState(connected)
	C.updateTrayIcon(unsafe.Pointer(&x1[0]), C.int(len(x1)), unsafe.Pointer(&x2[0]), C.int(len(x2)))

	// Update tooltip with session count.
	tip := trayTooltip(len(sessions))
	ctip := C.CString(tip)
	C.updateTrayTooltip(ctip)
	C.free(unsafe.Pointer(ctip))

	// Update session names for menu delegate.
	n := len(sessions)
	if n == 0 {
		C.setTraySessionData(nil, nil, 0)
		return
	}
	cNames := make([]*C.char, n)
	cIDs := make([]*C.char, n)
	for i, s := range sessions {
		cNames[i] = C.CString(s.Name)
		cIDs[i] = C.CString(s.ID)
	}
	C.setTraySessionData(&cNames[0], &cIDs[0], C.int(n))
	for i := range cNames {
		C.free(unsafe.Pointer(cNames[i]))
		C.free(unsafe.Pointer(cIDs[i]))
	}
}

// initTray creates a macOS system tray icon with a dynamic menu delegate.
func (a *App) initTray() {
	trayCallbackApp = a
	C.initStatusItem(unsafe.Pointer(&trayIconBytes[0]), C.int(len(trayIconBytes)),
		unsafe.Pointer(&trayIconBytes2x[0]), C.int(len(trayIconBytes2x)))
}

// cleanupTray removes the status bar item.
func (a *App) cleanupTray() {
	C.removeStatusItem()
}
