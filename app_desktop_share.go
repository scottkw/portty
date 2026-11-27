package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/scottkw/portty/internal/daemon"
)

// app_desktop_share.go: the GUI side of the click-to-share (Phase 22). The GUI
// holds the daemon's events stream for its whole life: the daemon reads the
// stream ending as "no GUI, so no share" (DSHR-06), and every state line drives
// the tray slot and icon and reaches React as desktopshare:state.

// Backoff of the events loop. Vars so tests shorten them.
var (
	shareBackoffMin = time.Second
	shareBackoffMax = 30 * time.Second
)

// desktopShareView is the share as the GUI last heard it; it is also the
// desktopshare:state payload (slices never nil, so React sees [] not null).
type desktopShareView struct {
	Active        bool                         `json:"active"`
	Viewers       []daemon.DesktopShareViewer  `json:"viewers"`
	Pending       []daemon.DesktopSharePending `json:"pending"`
	SecureInput   bool                         `json:"secureInput"`
	AnyController bool                         `json:"anyController"`
}

// desktopShareStopped is the desktopshare:stopped payload. Reason restart means
// the daemon (or the stream) went away under an active share (UI-SPEC Surface 5).
type desktopShareStopped struct {
	Reason string `json:"reason"`
}

// trayShareLine is the shared-with line of the tray menu; n counts connected
// viewers, never pending requests.
func trayShareLine(n int) string {
	switch {
	case n <= 0:
		return "Desktop shared with nobody yet"
	case n == 1:
		return "Desktop shared with 1 person"
	default:
		return fmt.Sprintf("Desktop shared with %d people", n)
	}
}

// startDesktopShareEvents starts the events loop once; it ends with ctx.
func (a *App) startDesktopShareEvents(ctx context.Context) {
	if a.client == nil && a.streamShareFunc == nil {
		return // startup failed before a client existed (tests only: NewApp always sets one)
	}
	a.shareLoopOnce.Do(func() { go a.runDesktopShareEvents(ctx) })
}

func (a *App) streamShare(ctx context.Context, fn func(daemon.DesktopShareLine)) error {
	if a.streamShareFunc != nil {
		return a.streamShareFunc(ctx, fn)
	}
	return a.client.StreamDesktopShareEvents(ctx, fn)
}

// runDesktopShareEvents redials the stream with a 1 s to 30 s backoff that a
// state line resets. Every end of the stream clears an active share's view.
func (a *App) runDesktopShareEvents(ctx context.Context) {
	backoff := shareBackoffMin
	for ctx.Err() == nil {
		err := a.streamShare(ctx, func(l daemon.DesktopShareLine) {
			switch l.Type {
			case "state":
				backoff = shareBackoffMin
				a.applyDesktopShareState(l)
			case "event":
				a.emitEvent("desktopshare:event", l)
			}
		})
		a.clearDesktopShare()
		if ctx.Err() != nil {
			return
		}
		log.Printf("desktop share: events stream ended: %v", err)
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return
		}
		backoff = min(backoff*2, shareBackoffMax)
	}
}

// applyDesktopShareState stores a state line, updates the tray and forwards it to React.
func (a *App) applyDesktopShareState(l daemon.DesktopShareLine) {
	v := desktopShareView{
		Active: l.Active, Viewers: l.Viewers, Pending: l.Pending,
		SecureInput: l.SecureInput, AnyController: l.AnyController,
	}
	if v.Viewers == nil {
		v.Viewers = []daemon.DesktopShareViewer{}
	}
	if v.Pending == nil {
		v.Pending = []daemon.DesktopSharePending{}
	}
	a.shareMu.Lock()
	a.shareView = v
	a.shareMu.Unlock()
	title := ""
	if v.Active {
		title = trayShareLine(len(v.Viewers))
	}
	a.setTrayShare(v.Active, title)
	a.setTrayViewers(len(v.Viewers))
	a.emitEvent("desktopshare:state", v)
}

// clearDesktopShare runs when the stream ends. A share that was active is over
// (the daemon ends it with the stream), so the tray resets and React is told why.
func (a *App) clearDesktopShare() {
	a.shareMu.Lock()
	was := a.shareView
	a.shareView = desktopShareView{}
	a.shareMu.Unlock()
	a.setTrayShare(false, "")
	a.setTrayViewers(0)
	if was.Active {
		a.emitEvent("desktopshare:stopped", desktopShareStopped{Reason: "restart"})
	}
}

func (a *App) setTrayShare(active bool, title string) {
	if a.setTrayShareFunc != nil {
		a.setTrayShareFunc(active, title)
		return
	}
	setTrayShareState(active, title)
}

// setTrayViewers stores the connected-viewer count the icon reads and, only when
// it changed (state lines come up to 10 a second), asks for a tray refresh off
// the stream's goroutine.
func (a *App) setTrayViewers(n int) {
	if a.trayViewers.Swap(int32(n)) == int32(n) {
		return
	}
	go func() {
		if a.refreshTrayStateFunc != nil {
			a.refreshTrayStateFunc()
		} else {
			a.refreshTrayState()
		}
	}()
}

// shareDesktopFromTray brings Portty forward through the Open Portty path and
// asks React to open the Share Desktop sheet.
func (a *App) shareDesktopFromTray() {
	a.showWindow()
	a.emitEvent("desktopshare:open-sheet", nil)
}

// The sheets' bindings. Like the Settings bindings they never throw: a failure is
// an ErrorCode, and Detail is only ever the daemon's wording of its own refusal or
// this process's own transport error, never text from a remote peer.

// DesktopShareResult is StartDesktopShare's answer. The link is returned here and
// by GetDesktopShare only, for the sheet's Copy Link; it is never logged, emitted
// in an event or put in a notification.
type DesktopShareResult struct {
	URL        string `json:"url"`
	FirstShare bool   `json:"firstShare"`
	ErrorCode  string `json:"errorCode"`
	Detail     string `json:"detail"`
}

// DesktopShareInfo is GetDesktopShare's answer: whether a share is active and its link.
type DesktopShareInfo struct {
	Active    bool   `json:"active"`
	URL       string `json:"url"`
	ErrorCode string `json:"errorCode"`
	Detail    string `json:"detail"`
}

// DesktopShareActionResult answers every binding that returns nothing but success.
type DesktopShareActionResult struct {
	ErrorCode string `json:"errorCode"`
	Detail    string `json:"detail"`
}

// shareFailure turns a client error into the binding's code and detail: the
// daemon's own refusal keeps its code, anything else gets fallback.
func shareFailure(err error, fallback string) (code, detail string) {
	var se *daemon.ShareError
	if errors.As(err, &se) {
		return se.Code, se.Detail
	}
	return fallback, err.Error()
}

// shareAction runs one daemon call that returns only an error.
func (a *App) shareAction(fallback string, call func(*daemon.DaemonClient) error) DesktopShareActionResult {
	if a.client == nil {
		return DesktopShareActionResult{ErrorCode: "unavailable", Detail: "daemon not connected"}
	}
	if err := call(a.client); err != nil {
		code, detail := shareFailure(err, fallback)
		return DesktopShareActionResult{ErrorCode: code, Detail: detail}
	}
	return DesktopShareActionResult{}
}

// StartDesktopShare starts the click-to-share, or returns the running one's link.
func (a *App) StartDesktopShare() DesktopShareResult {
	if a.client == nil {
		return DesktopShareResult{ErrorCode: "unavailable", Detail: "daemon not connected"}
	}
	s, err := a.client.StartDesktopShare()
	if err != nil {
		code, detail := shareFailure(err, "start_failed")
		return DesktopShareResult{ErrorCode: code, Detail: detail}
	}
	return DesktopShareResult{URL: s.URL, FirstShare: s.FirstShare}
}

// GetDesktopShare reads whether a share is active and, while one is, its link.
func (a *App) GetDesktopShare() DesktopShareInfo {
	if a.client == nil {
		return DesktopShareInfo{ErrorCode: "unavailable", Detail: "daemon not connected"}
	}
	s, err := a.client.DesktopShare()
	if err != nil {
		code, detail := shareFailure(err, "unavailable")
		return DesktopShareInfo{ErrorCode: code, Detail: detail}
	}
	return DesktopShareInfo{Active: s.Active, URL: s.URL}
}

// StopDesktopShare ends the guest share.
func (a *App) StopDesktopShare() DesktopShareActionResult {
	return a.shareAction("stop_failed", func(c *daemon.DaemonClient) error { return c.StopDesktopShare() })
}

// AdmitDesktopGuest lets the waiting guest in.
func (a *App) AdmitDesktopGuest(id string) DesktopShareActionResult {
	return a.shareAction("unavailable", func(c *daemon.DaemonClient) error { return c.AdmitDesktopGuest(id) })
}

// DenyDesktopGuest refuses the waiting guest, and the person for the rest of the share.
func (a *App) DenyDesktopGuest(id string) DesktopShareActionResult {
	return a.shareAction("unavailable", func(c *daemon.DaemonClient) error { return c.DenyDesktopGuest(id) })
}

// SetDesktopViewerRights sets a guest's control and clipboard rights.
func (a *App) SetDesktopViewerRights(id string, control, clipboard bool) DesktopShareActionResult {
	return a.shareAction("unavailable", func(c *daemon.DaemonClient) error {
		return c.SetDesktopViewerRights(id, control, clipboard)
	})
}

// DisconnectDesktopViewer cuts one viewer at once.
func (a *App) DisconnectDesktopViewer(id string) DesktopShareActionResult {
	return a.shareAction("unavailable", func(c *daemon.DaemonClient) error { return c.DisconnectDesktopViewer(id) })
}

// stopSharingFromTray ends the guest share at once, with no confirm. A failure
// is logged: the tray never raises a dialog.
func (a *App) stopSharingFromTray() {
	var err error
	switch {
	case a.stopShareFunc != nil:
		err = a.stopShareFunc()
	case a.client != nil:
		err = a.client.StopDesktopShare()
	}
	if err != nil {
		log.Printf("desktop share: stop from tray failed: %v", err)
	}
}
