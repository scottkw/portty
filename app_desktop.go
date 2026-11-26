package main

import (
	"encoding/base64"
	"fmt"
)

// app_desktop.go: the GUI side of the desktop permission setup (19-11). The
// grants belong to the daemon's bundle, so the daemon reads them and takes the
// test capture; these bindings only ask it and open two System Settings panes.

// The only two System Settings panes the setup sheet may open. Full Disk Access
// is never one of them: Portty does not ask for it (DPRM-01).
const (
	paneScreenRecording = "x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture"
	paneAccessibility   = "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility"
)

// DesktopPermissionsResult is GetDesktopPermissions' answer. ErrorCode is ""
// when SR and AX are the OS's live answer and "unavailable" when they are not
// known (daemon not reachable, or its probe failed): never a guessed false.
type DesktopPermissionsResult struct {
	SR        bool   `json:"sr"`
	AX        bool   `json:"ax"`
	ErrorCode string `json:"errorCode"`
}

// TestCaptureResult is RunDesktopTestCapture's answer: a JPEG data URL, or an
// ErrorCode of needs_permission, timeout, capture_failed or unavailable.
type TestCaptureResult struct {
	ImageDataURL string `json:"imageDataURL"`
	ErrorCode    string `json:"errorCode"`
}

// GetDesktopPermissions reads the live Screen Recording and Accessibility
// grants through the daemon's fresh-process probe. Never throws.
func (a *App) GetDesktopPermissions() DesktopPermissionsResult {
	if a.client == nil {
		return DesktopPermissionsResult{ErrorCode: "unavailable"}
	}
	sr, ax, err := a.client.DesktopPermissions()
	if err != nil {
		return DesktopPermissionsResult{ErrorCode: "unavailable"}
	}
	return DesktopPermissionsResult{SR: sr, AX: ax}
}

// RunDesktopTestCapture has the daemon take one still of the main display.
// Never throws; the picture is passed on and not kept.
func (a *App) RunDesktopTestCapture() TestCaptureResult {
	if a.client == nil {
		return TestCaptureResult{ErrorCode: "unavailable"}
	}
	jpeg, code, err := a.client.DesktopTestCapture()
	switch {
	case err != nil:
		return TestCaptureResult{ErrorCode: "unavailable"}
	case code != "":
		return TestCaptureResult{ErrorCode: code}
	}
	return TestCaptureResult{ImageDataURL: "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(jpeg)}
}

// OpenDesktopPermissionPane opens the System Settings pane for kind, "screen"
// or "accessibility". Anything else is refused without opening anything.
func (a *App) OpenDesktopPermissionPane(kind string) error {
	switch kind {
	case "screen":
		return openSystemURL(paneScreenRecording)
	case "accessibility":
		return openSystemURL(paneAccessibility)
	}
	return fmt.Errorf("unknown permission pane %q", kind)
}
