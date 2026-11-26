package main

import (
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/scottkw/portty/internal/daemon"
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

// The Settings ▸ Sharing Desktop group (19-13). Like the share bindings these
// never throw: a failure is an ErrorCode (and a detail for the inline error).

// DesktopStatusResult is GetDesktopStatus' answer: the daemon's view of the
// always-available desktop. ErrorCode is "unavailable" when the daemon cannot
// be asked; the other fields are then zero and mean nothing.
type DesktopStatusResult struct {
	AlwaysAvailable  bool   `json:"alwaysAvailable"`
	State            string `json:"state"`
	Viewers          int    `json:"viewers"`
	Locked           bool   `json:"locked"`
	ApproachingLapse bool   `json:"approachingLapse"`
	Tailscale        string `json:"tailscale"`
	Launchd          bool   `json:"launchd"`
	Reason           string `json:"reason"`
	ErrorCode        string `json:"errorCode"`
}

// DesktopResult is SetDesktopAlwaysAvailable's answer. FirstShare is true when
// this call recorded the Certificate Transparency acknowledgement.
type DesktopResult struct {
	AlwaysAvailable bool   `json:"alwaysAvailable"`
	FirstShare      bool   `json:"firstShare"`
	ErrorCode       string `json:"errorCode"`
	ErrorDetail     string `json:"errorDetail"`
}

// DesktopLinkResult is GetDesktopLink's answer: the link, or an ErrorCode.
type DesktopLinkResult struct {
	URL       string `json:"url"`
	ErrorCode string `json:"errorCode"`
}

// GetDesktopStatus reads the daemon's desktop state.
func (a *App) GetDesktopStatus() DesktopStatusResult {
	if a.client == nil {
		return DesktopStatusResult{ErrorCode: "unavailable"}
	}
	s, err := a.client.DesktopStatus()
	if err != nil {
		return DesktopStatusResult{ErrorCode: "unavailable"}
	}
	return DesktopStatusResult{
		AlwaysAvailable: s.AlwaysAvailable, State: s.State, Viewers: s.Viewers, Locked: s.Locked,
		ApproachingLapse: s.ApproachingLapse, Tailscale: s.Tailscale, Launchd: s.Launchd, Reason: s.Reason,
	}
}

// SetDesktopAlwaysAvailable turns the always-available desktop on or off. A
// refusal keeps the daemon's code (tailscale_off, https_off, tailscale_userspace,
// port_in_use, start_failed) and detail; an unreachable daemon is "unavailable".
func (a *App) SetDesktopAlwaysAvailable(on bool) DesktopResult {
	if a.client == nil {
		return DesktopResult{ErrorCode: "unavailable", ErrorDetail: "daemon not connected"}
	}
	resp, err := a.client.SetDesktopAlwaysAvailable(on)
	if err == nil {
		return DesktopResult{AlwaysAvailable: resp.AlwaysAvailable, FirstShare: resp.FirstShare}
	}
	var se *daemon.ShareError
	if errors.As(err, &se) {
		return DesktopResult{ErrorCode: se.Code, ErrorDetail: se.Detail}
	}
	return DesktopResult{ErrorCode: "unavailable", ErrorDetail: err.Error()}
}

// GetDesktopLink returns the always-available link. The UI fetches it only on
// Copy Link, writes it to the clipboard and never shows or keeps it.
func (a *App) GetDesktopLink() DesktopLinkResult {
	if a.client == nil {
		return DesktopLinkResult{ErrorCode: "unavailable"}
	}
	url, err := a.client.DesktopLink()
	if err != nil {
		return DesktopLinkResult{ErrorCode: "unavailable"}
	}
	return DesktopLinkResult{URL: url}
}
