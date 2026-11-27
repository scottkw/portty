package main

import (
	"context"
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
