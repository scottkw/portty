package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/scottkw/portty/internal/daemon"
	"github.com/scottkw/portty/internal/desktop"
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
			if l.Type == "state" {
				backoff = shareBackoffMin
			}
			a.handleShareLine(l)
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

// handleShareLine routes one line of the events stream. Every event line reaches
// React unfiltered (own-account joins and leaves included) and then drives the
// window and the notifications.
func (a *App) handleShareLine(l daemon.DesktopShareLine) {
	switch l.Type {
	case "state":
		a.applyDesktopShareState(l)
	case "event":
		a.emitEvent("desktopshare:event", l)
		a.announceShareEvent(l)
	}
}

// Notification identifiers. A same-kind burst reuses its identifier, so the
// newer banner replaces the older one.
const (
	desktopAdmitNotificationID = "portty.desktop-admit"
	desktopJoinNotificationID  = "portty.desktop-join"
	desktopLeaveNotificationID = "portty.desktop-leave"
)

const (
	notifyCoalesceWindow = 5 * time.Second
	notifyNameRunes      = 80
)

// notifyCoalescer counts same-kind notifications that arrive less than
// notifyCoalesceWindow after the previous one of that kind.
type notifyCoalescer struct {
	mu   sync.Mutex
	runs map[string]coalesceRun
}

type coalesceRun struct {
	last time.Time
	n    int
}

// count records one more notification of kind at now and returns how many are in
// the current burst: 1 for a fresh one, 2 or more when it follows within the window.
func (c *notifyCoalescer) count(kind string, now time.Time) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.runs == nil {
		c.runs = map[string]coalesceRun{}
	}
	r := c.runs[kind]
	if r.n == 0 || now.Sub(r.last) >= notifyCoalesceWindow {
		r.n = 0
	}
	r.n++
	r.last = now
	c.runs[kind] = r
	return r.n
}

// shareViewerName is the name a banner shows: the login name for a guest, the
// device name for an own-account device (short, unique, hard to imitate). The
// daemon already sanitized it; this sanitizes again and caps it at 80 characters.
func shareViewerName(v *daemon.DesktopShareViewer) string {
	if v == nil {
		return "Someone"
	}
	candidates := []string{v.LoginName, v.DisplayName, v.NodeName}
	if v.Mode == "own" {
		candidates = []string{v.NodeName, v.DisplayName, v.LoginName}
	}
	for _, c := range candidates {
		if name := desktop.SanitizeName(c); name != "" {
			if r := []rune(name); len(r) > notifyNameRunes {
				return string(r[:notifyNameRunes]) + "…"
			}
			return name
		}
	}
	return "Someone"
}

func (a *App) shareNow() time.Time {
	if a.shareNowFunc != nil {
		return a.shareNowFunc()
	}
	return time.Now()
}

func (a *App) appActiveAndVisible() bool {
	if a.appActiveFunc != nil {
		return a.appActiveFunc()
	}
	return isAppActiveAndVisible()
}

// announceShareEvent brings the window forward for a knock and sends the secondary
// notifications. Banners are never the only signal (they may be silenced while the
// display is captured): the Admit sheet, tray icon and tray line carry the load,
// and a notification has no buttons, so it can never admit or deny anyone.
func (a *App) announceShareEvent(l daemon.DesktopShareLine) {
	switch l.Event {
	case "admit_request":
		a.showWindow()
		if !a.appActiveAndVisible() {
			a.sendNotificationFunc(desktopAdmitNotificationID, "Portty", shareViewerName(l.Viewer)+" wants to see your desktop.")
		}
	case "join":
		a.notifyViewerChange(desktopJoinNotificationID, l.Viewer, "joined")
	case "leave":
		a.notifyViewerChange(desktopLeaveNotificationID, l.Viewer, "left")
	}
}

func (a *App) notifyViewerChange(id string, v *daemon.DesktopShareViewer, verb string) {
	body := shareViewerName(v) + " " + verb + " your desktop."
	if n := a.shareNotes.count(id, a.shareNow()); n > 1 {
		body = fmt.Sprintf("%d viewers %s your desktop.", n, verb)
	}
	a.sendNotificationFunc(id, "Portty", body)
}

// shareNotificationClicked runs for a click on any of the three desktop
// notifications: Portty comes forward and, when a share is active and nobody is
// waiting (the Admit sheet is already up then), the Share Desktop sheet opens.
func (a *App) shareNotificationClicked() {
	a.showWindow()
	a.shareMu.Lock()
	v := a.shareView
	a.shareMu.Unlock()
	if v.Active && len(v.Pending) == 0 {
		a.emitEvent("desktopshare:open-sheet", nil)
	}
}

// quitShareStopWait bounds how long Quit GUI Only waits for the daemon to end the
// share: with a silent guest peer a stop can take about 2 s, so a timeout is
// expected now and then and is fine, because the stop carries on in the daemon.
var quitShareStopWait = 3 * time.Second

// stopShareForQuit ends an active click-to-share before the window hides.
func (a *App) stopShareForQuit() {
	a.shareMu.Lock()
	active := a.shareView.Active
	a.shareMu.Unlock()
	if !active {
		return
	}
	done := make(chan error, 1)
	go func() { done <- a.stopShare() }()
	select {
	case err := <-done:
		if err != nil {
			log.Printf("desktop share: stop on quit failed: %v", err)
		}
	case <-time.After(quitShareStopWait):
		log.Printf("desktop share: stop on quit still running after %v; hiding anyway", quitShareStopWait)
	}
}

// stopShare asks the daemon to end the guest share.
func (a *App) stopShare() error {
	switch {
	case a.stopShareFunc != nil:
		return a.stopShareFunc()
	case a.client != nil:
		return a.client.StopDesktopShare()
	}
	return nil
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
	if err := a.stopShare(); err != nil {
		log.Printf("desktop share: stop from tray failed: %v", err)
	}
}
