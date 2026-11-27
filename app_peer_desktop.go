package main

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/scottkw/portty/internal/daemon"
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
