package main

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/scottkw/portty/internal/daemon"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Phase 21-07: the Wails side of a peer's always-available desktop. The viewing
// daemon holds the peer's desktop token in memory; the web view only ever sees a
// token-free loopback URL, and the browser link is opened from Go.

// paneLocalNetwork is System Settings > Privacy & Security > Local Network.
// ASSUMPTION: the pane's anchor name is recorded from the known Privacy_* family
// and is confirmed on a real macOS in the 21-11 VM row.
const paneLocalNetwork = "x-apple.systempreferences:com.apple.preference.security?Privacy_LocalNetwork"

// PeerDesktopResult is what ConnectPeerDesktop returns. It never throws and has
// no token field: URL is the loopback proxy URL (ws://127.0.0.1:<relay port>/...),
// filled whenever the daemon's relay port is known, even when ErrorCode is set.
// ErrorCode is the daemon's POST /peers/desktop code, or "unreachable".
type PeerDesktopResult struct {
	Host      string `json:"host"`
	URL       string `json:"url"`
	ErrorCode string `json:"errorCode"`
}

// desktopHostKey is the form the daemon keys a peer's held token by: lower-case,
// no trailing dot.
func desktopHostKey(host string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
}

// remoteDesktopWsURL is the loopback proxy URL for host's desktop. It carries no
// token: the daemon adds the held one when it dials the peer.
func remoteDesktopWsURL(port int, host string) string {
	return fmt.Sprintf("ws://127.0.0.1:%d/api/relay/remote-desktop/%s/ws", port, url.PathEscape(desktopHostKey(host)))
}

// peerDesktopCode maps a ConnectPeerDesktop error to its binding error code.
func peerDesktopCode(err error) string {
	var pe *daemon.PeerConnectError
	if errors.As(err, &pe) {
		return pe.Code
	}
	return daemon.PeerErrUnreachable
}

// ConnectPeerDesktop has the daemon fetch host's desktop token into its memory
// and returns the loopback URL the Desktop tab points at. It never throws.
func (a *App) ConnectPeerDesktop(host string) PeerDesktopResult {
	if a.client == nil {
		return PeerDesktopResult{ErrorCode: daemon.PeerErrUnreachable}
	}
	key := desktopHostKey(host)
	res := PeerDesktopResult{Host: key}
	resp, err := a.client.ConnectPeerDesktop(host)
	if err != nil {
		res.ErrorCode = peerDesktopCode(err)
	} else if resp.Host != "" {
		key = desktopHostKey(resp.Host)
		res.Host = key
	}
	if port, perr := a.GetRelayPortChecked(); perr == nil {
		res.URL = remoteDesktopWsURL(port, key)
	} else if res.ErrorCode == "" {
		res.ErrorCode = daemon.PeerErrUnreachable
	}
	return res
}

// openPeerDesktopLink passes link to the browser seam. The link carries the
// token, so it goes nowhere else: not returned, not logged.
func (a *App) openPeerDesktopLink(link string) {
	openURL := a.openURLFunc
	if openURL == nil {
		openURL = func(u string) {
			if a.ctx != nil {
				runtime.BrowserOpenURL(a.ctx, u)
			}
		}
	}
	openURL(link)
}

// OpenPeerDesktopInBrowser opens host's desktop link in the default browser from
// the token the daemon holds, fetching it first when none is held. It returns ""
// or an error code and never hands the link to the web view. Call it only from a
// user's click.
func (a *App) OpenPeerDesktopInBrowser(host string) string {
	if a.client == nil {
		return daemon.PeerErrUnreachable
	}
	key := desktopHostKey(host)
	link, err := a.client.PeerDesktopLink(key)
	if err != nil {
		// Nothing held yet (a 404), or the daemon is gone: one connect, one retry.
		resp, cerr := a.client.ConnectPeerDesktop(host)
		if cerr != nil {
			return peerDesktopCode(cerr)
		}
		if resp.Host != "" {
			key = desktopHostKey(resp.Host)
		}
		if link, err = a.client.PeerDesktopLink(key); err != nil {
			return daemon.PeerErrUnreachable
		}
	}
	a.openPeerDesktopLink(link)
	return ""
}

// OpenLocalNetworkSettings opens System Settings at Privacy & Security, Local
// Network (the Desktop tab's permission card).
func (a *App) OpenLocalNetworkSettings() error {
	return openSystemURL(paneLocalNetwork)
}
