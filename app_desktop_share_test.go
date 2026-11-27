package main

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/daemon"
)

type trayShareCall struct {
	active bool
	title  string
}

// shareHarness records everything the share loop does to the tray and to React.
type shareHarness struct {
	mu        sync.Mutex
	tray      []trayShareCall
	events    []string
	refreshed chan struct{}
}

func newShareHarness(a *App) *shareHarness {
	h := &shareHarness{refreshed: make(chan struct{}, 16)}
	a.setTrayShareFunc = func(active bool, title string) {
		h.mu.Lock()
		h.tray = append(h.tray, trayShareCall{active, title})
		h.mu.Unlock()
	}
	a.emitEventFunc = func(name string, _ any) {
		h.mu.Lock()
		h.events = append(h.events, name)
		h.mu.Unlock()
	}
	a.refreshTrayStateFunc = func() { h.refreshed <- struct{}{} }
	a.sendNotificationFunc = func(string, string, string) {}
	a.showWindowFunc = func() {}
	a.appActiveFunc = func() bool { return true }
	return h
}

func (h *shareHarness) lastTray() trayShareCall {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.tray) == 0 {
		return trayShareCall{title: "<none>"}
	}
	return h.tray[len(h.tray)-1]
}

func (h *shareHarness) count(name string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, e := range h.events {
		if e == name {
			n++
		}
	}
	return n
}

func viewers(n int) []daemon.DesktopShareViewer {
	out := make([]daemon.DesktopShareViewer, n)
	for i := range out {
		out[i] = daemon.DesktopShareViewer{ID: string(rune('a' + i)), Mode: "guest"}
	}
	return out
}

func TestTrayShare_Line(t *testing.T) {
	for n, want := range map[int]string{
		0: "Desktop shared with nobody yet",
		1: "Desktop shared with 1 person",
		3: "Desktop shared with 3 people",
	} {
		if got := trayShareLine(n); got != want {
			t.Errorf("trayShareLine(%d) = %q, want %q", n, got, want)
		}
	}
}

// TestDesktopShareApp_TracerTray: an active share with nobody connected shows
// "nobody yet", Stop Sharing from the tray reaches the daemon client, and the
// stream ending under an active share resets the tray and says why.
func TestDesktopShareApp_TracerTray(t *testing.T) {
	a := &App{}
	h := newShareHarness(a)
	release := make(chan struct{})
	a.streamShareFunc = func(ctx context.Context, fn func(daemon.DesktopShareLine)) error {
		fn(daemon.DesktopShareLine{Type: "state", Active: true})
		select {
		case <-release:
			return errors.New("daemon went away")
		case <-ctx.Done():
			return nil
		}
	}
	stops := make(chan struct{}, 1)
	a.stopShareFunc = func() error { stops <- struct{}{}; return nil }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.startDesktopShareEvents(ctx)

	waitFor(t, func() bool { return h.count("desktopshare:state") == 1 })
	if got := h.lastTray(); !got.active || got.title != "Desktop shared with nobody yet" {
		t.Fatalf("tray = %+v, want active nobody yet", got)
	}

	a.stopSharingFromTray()
	select {
	case <-stops:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop Sharing did not reach StopDesktopShare")
	}

	close(release)
	waitFor(t, func() bool { return h.count("desktopshare:stopped") == 1 })
	if got := h.lastTray(); got.active || got.title != "" {
		t.Errorf("tray after stream end = %+v, want reset", got)
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not reached")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestDesktopShareApp_StateDrivesCountAndIconRefresh(t *testing.T) {
	a := &App{}
	h := newShareHarness(a)

	a.applyDesktopShareState(daemon.DesktopShareLine{Type: "state", Active: true, Viewers: viewers(2), Pending: []daemon.DesktopSharePending{{ID: "r1"}}})
	if got := h.lastTray(); !got.active || got.title != "Desktop shared with 2 people" {
		t.Errorf("tray = %+v, pending requests must not count", got)
	}
	if n := a.trayViewers.Load(); n != 2 {
		t.Errorf("trayViewers = %d, want 2", n)
	}
	<-h.refreshed

	// Same count again: no tray refresh (state lines arrive up to 10 a second).
	a.applyDesktopShareState(daemon.DesktopShareLine{Type: "state", Active: true, Viewers: viewers(2)})
	select {
	case <-h.refreshed:
		t.Error("refreshed again with an unchanged viewer count")
	case <-time.After(50 * time.Millisecond):
	}

	// Own devices only, no click-to-share: no line, but the icon count stays.
	a.applyDesktopShareState(daemon.DesktopShareLine{Type: "state", Active: false, Viewers: viewers(1)})
	if got := h.lastTray(); got.active || got.title != "" {
		t.Errorf("tray = %+v, want no share line without an active share", got)
	}
	if n := a.trayViewers.Load(); n != 1 {
		t.Errorf("trayViewers = %d, want 1", n)
	}
}

func TestDesktopShareApp_ForwardsEventsAndRedials(t *testing.T) {
	shareBackoffMin, shareBackoffMax = time.Millisecond, 2*time.Millisecond
	t.Cleanup(func() { shareBackoffMin, shareBackoffMax = time.Second, 30*time.Second })

	a := &App{}
	h := newShareHarness(a)
	var mu sync.Mutex
	dials := 0
	a.streamShareFunc = func(ctx context.Context, fn func(daemon.DesktopShareLine)) error {
		mu.Lock()
		dials++
		n := dials
		mu.Unlock()
		if n == 1 {
			return &daemon.ShareError{Code: "gui_busy"}
		}
		fn(daemon.DesktopShareLine{Type: "heartbeat"})
		fn(daemon.DesktopShareLine{Type: "event", Event: "join"})
		<-ctx.Done()
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.startDesktopShareEvents(ctx)
	a.startDesktopShareEvents(ctx) // once

	waitFor(t, func() bool { return h.count("desktopshare:event") == 1 })
	mu.Lock()
	defer mu.Unlock()
	if dials != 2 {
		t.Errorf("dials = %d, want 2 (one refusal, one redial; second start is a no-op)", dials)
	}
	if h.count("desktopshare:stopped") != 0 {
		t.Error("stopped emitted although no share was active")
	}
}

func TestDesktopShareApp_TrayShowSheet(t *testing.T) {
	shown := 0
	a := &App{}
	h := newShareHarness(a)
	a.showWindowFunc = func() { shown++ }
	a.shareDesktopFromTray()
	if shown != 1 || h.count("desktopshare:open-sheet") != 1 {
		t.Errorf("shown=%d open-sheet=%d, want 1 and 1", shown, h.count("desktopshare:open-sheet"))
	}
}

type sentNote struct{ id, title, body string }

// noteHarness records the notifications, window shows and events the share
// loop causes, with a fake clock and a fake "Portty is frontmost" answer.
type noteHarness struct {
	mu     sync.Mutex
	notes  []sentNote
	shown  int
	events []string
	now    time.Time
	active bool
}

func newNoteHarness(a *App) *noteHarness {
	h := &noteHarness{now: time.Unix(1_000_000, 0)}
	a.sendNotificationFunc = func(id, title, body string) {
		h.mu.Lock()
		h.notes = append(h.notes, sentNote{id, title, body})
		h.mu.Unlock()
	}
	a.showWindowFunc = func() { h.mu.Lock(); h.shown++; h.mu.Unlock() }
	a.emitEventFunc = func(name string, _ any) { h.mu.Lock(); h.events = append(h.events, name); h.mu.Unlock() }
	a.shareNowFunc = func() time.Time { h.mu.Lock(); defer h.mu.Unlock(); return h.now }
	a.appActiveFunc = func() bool { h.mu.Lock(); defer h.mu.Unlock(); return h.active }
	a.setTrayShareFunc = func(bool, string) {}
	a.refreshTrayStateFunc = func() {}
	return h
}

func (h *noteHarness) advance(d time.Duration) { h.mu.Lock(); h.now = h.now.Add(d); h.mu.Unlock() }
func (h *noteHarness) setActive(v bool)        { h.mu.Lock(); h.active = v; h.mu.Unlock() }

func (h *noteHarness) last() sentNote {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.notes) == 0 {
		return sentNote{body: "<none>"}
	}
	return h.notes[len(h.notes)-1]
}

func (h *noteHarness) noteCount() int { h.mu.Lock(); defer h.mu.Unlock(); return len(h.notes) }
func (h *noteHarness) shows() int     { h.mu.Lock(); defer h.mu.Unlock(); return h.shown }
func (h *noteHarness) eventCount(name string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, e := range h.events {
		if e == name {
			n++
		}
	}
	return n
}

func eventLine(event, mode, login, node string) daemon.DesktopShareLine {
	return daemon.DesktopShareLine{Type: "event", Event: event, Viewer: &daemon.DesktopShareViewer{ID: "v", Mode: mode, LoginName: login, NodeName: node}}
}

func TestShareNotification_AdmitBringsWindowForwardAndNotifiesOnlyWhenHidden(t *testing.T) {
	a := &App{}
	h := newNoteHarness(a)

	h.setActive(false)
	a.handleShareLine(eventLine("admit_request", "guest", "ann@example.com", ""))
	if h.shows() != 1 || h.eventCount("desktopshare:event") != 1 {
		t.Errorf("shows=%d events=%d, want the window shown and the event forwarded", h.shows(), h.eventCount("desktopshare:event"))
	}
	if got := h.last(); got != (sentNote{"portty.desktop-admit", "Portty", "ann@example.com wants to see your desktop."}) {
		t.Errorf("admit notification = %+v", got)
	}

	h.setActive(true)
	a.handleShareLine(eventLine("admit_request", "guest", "bob@example.com", ""))
	if h.shows() != 2 || h.noteCount() != 1 {
		t.Errorf("shows=%d notes=%d, want the window shown again and no second notification while Portty is frontmost", h.shows(), h.noteCount())
	}
}

func TestShareNotification_JoinAndLeaveSentences(t *testing.T) {
	a := &App{}
	h := newNoteHarness(a)
	for _, c := range []struct {
		line daemon.DesktopShareLine
		want sentNote
	}{
		{eventLine("join", "guest", "ann@example.com", "annmac"), sentNote{"portty.desktop-join", "Portty", "ann@example.com joined your desktop."}},
		{eventLine("join", "own", "ken@example.com", "ken-iphone"), sentNote{"portty.desktop-join", "Portty", "ken-iphone joined your desktop."}},
		{eventLine("leave", "guest", "ann@example.com", "annmac"), sentNote{"portty.desktop-leave", "Portty", "ann@example.com left your desktop."}},
		{eventLine("leave", "own", "ken@example.com", "ken-iphone"), sentNote{"portty.desktop-leave", "Portty", "ken-iphone left your desktop."}},
	} {
		h.advance(time.Minute) // apart: never coalesced
		a.handleShareLine(c.line)
		if got := h.last(); got != c.want {
			t.Errorf("%s/%s notification = %+v, want %+v", c.line.Event, c.line.Viewer.Mode, got, c.want)
		}
	}
	if h.shows() != 0 {
		t.Errorf("join and leave showed the window %d times, want 0", h.shows())
	}
	// Every line still reaches React, own-account ones included.
	if n := h.eventCount("desktopshare:event"); n != 4 {
		t.Errorf("desktopshare:event = %d, want 4", n)
	}
}

func TestShareNotification_CoalescesWithinFiveSeconds(t *testing.T) {
	a := &App{}
	h := newNoteHarness(a)
	a.handleShareLine(eventLine("join", "guest", "a@x", ""))
	h.advance(2 * time.Second)
	a.handleShareLine(eventLine("join", "guest", "b@x", ""))
	h.advance(2 * time.Second)
	a.handleShareLine(eventLine("join", "guest", "c@x", ""))
	if got := h.last(); got.id != "portty.desktop-join" || got.body != "3 viewers joined your desktop." {
		t.Errorf("third join = %+v, want the coalesced sentence under the same identifier", got)
	}
	// A leave in the same burst is its own kind.
	a.handleShareLine(eventLine("leave", "guest", "a@x", ""))
	if got := h.last(); got.body != "a@x left your desktop." {
		t.Errorf("leave = %+v, want its own single sentence", got)
	}
	// Quiet for more than 5 s: a fresh single sentence.
	h.advance(6 * time.Second)
	a.handleShareLine(eventLine("join", "guest", "d@x", ""))
	if got := h.last(); got.body != "d@x joined your desktop." {
		t.Errorf("join after a quiet gap = %+v, want a single sentence", got)
	}
}

func TestShareNotification_NameIsCappedAtEightyWithEllipsis(t *testing.T) {
	a := &App{}
	h := newNoteHarness(a)
	long := strings.Repeat("a", 200)
	a.handleShareLine(eventLine("join", "guest", long, ""))
	want := strings.Repeat("a", 80) + "… joined your desktop."
	if got := h.last().body; got != want {
		t.Errorf("long name body = %q, want %q", got, want)
	}
	// Format characters in a name never reach a banner (bidi override, zero-width).
	h.advance(time.Minute)
	a.handleShareLine(eventLine("join", "guest", "ann\u202e@example.com\u200b", ""))
	if got := h.last().body; got != "ann@example.com joined your desktop." {
		t.Errorf("sanitized body = %q", got)
	}
}

func TestShareNotification_ClickRouting(t *testing.T) {
	for _, id := range []string{"portty.desktop-admit", "portty.desktop-join", "portty.desktop-leave"} {
		a := &App{}
		h := newNoteHarness(a)
		a.applyDesktopShareState(daemon.DesktopShareLine{Type: "state", Active: true})
		a.handleNotificationClick(id)
		if h.shows() != 1 || h.eventCount("desktopshare:open-sheet") != 1 {
			t.Errorf("%s with an active share and nothing pending: shows=%d open-sheet=%d, want 1 and 1", id, h.shows(), h.eventCount("desktopshare:open-sheet"))
		}

		a.applyDesktopShareState(daemon.DesktopShareLine{Type: "state", Active: true, Pending: []daemon.DesktopSharePending{{ID: "r1"}}})
		a.handleNotificationClick(id)
		if h.shows() != 2 || h.eventCount("desktopshare:open-sheet") != 1 {
			t.Errorf("%s with a request pending: shows=%d open-sheet=%d, want 2 and 1 (the Admit sheet is already up)", id, h.shows(), h.eventCount("desktopshare:open-sheet"))
		}

		a.applyDesktopShareState(daemon.DesktopShareLine{Type: "state", Active: false})
		a.handleNotificationClick(id)
		if h.shows() != 3 || h.eventCount("desktopshare:open-sheet") != 1 {
			t.Errorf("%s with no share: shows=%d open-sheet=%d, want 3 and 1", id, h.shows(), h.eventCount("desktopshare:open-sheet"))
		}
	}
}

func TestShareNotification_NoActionButtonsOrLinkInBody(t *testing.T) {
	a := &App{}
	h := newNoteHarness(a)
	v := eventLine("join", "guest", "ann@example.com", "")
	a.handleShareLine(v)
	if got := h.last(); strings.Contains(got.body, "http") || strings.Contains(got.body, "cap=") {
		t.Errorf("notification body carries a link: %+v", got)
	}
}

func TestDesktopShareQuit_StopsAnActiveShareFirst(t *testing.T) {
	a := &App{}
	newNoteHarness(a)
	stops := 0
	a.stopShareFunc = func() error { stops++; return nil }
	a.applyDesktopShareState(daemon.DesktopShareLine{Type: "state", Active: true})
	a.stopShareForQuit()
	if stops != 1 {
		t.Errorf("stops = %d, want 1", stops)
	}
}

func TestDesktopShareQuit_NoShareNoStop(t *testing.T) {
	a := &App{}
	newNoteHarness(a)
	stops := 0
	a.stopShareFunc = func() error { stops++; return nil }
	a.stopShareForQuit()
	// Own-account viewers alone are not a click-to-share.
	a.applyDesktopShareState(daemon.DesktopShareLine{Type: "state", Active: false, Viewers: viewers(1)})
	a.stopShareForQuit()
	if stops != 0 {
		t.Errorf("stops = %d, want 0 without an active share", stops)
	}
}

func TestDesktopShareQuit_SlowStopDoesNotBlockHiding(t *testing.T) {
	old := quitShareStopWait
	quitShareStopWait = 20 * time.Millisecond
	t.Cleanup(func() { quitShareStopWait = old })

	a := &App{}
	newNoteHarness(a)
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	a.stopShareFunc = func() error { <-release; return nil }
	a.applyDesktopShareState(daemon.DesktopShareLine{Type: "state", Active: true})
	done := make(chan struct{})
	go func() { a.stopShareForQuit(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("stopShareForQuit did not give up on a slow stop")
	}
}

// shareRoute is what one fake daemon route answers.
type shareRoute struct {
	status int
	body   string
}

// fakeShareDaemon serves the click-to-share routes on a temp daemon socket, so
// the bindings run against real HTTP with no daemon, tailnet or capture. hits
// records "METHOD path" for every request.
func fakeShareDaemon(t *testing.T, routes map[string]shareRoute) (*App, *[]string) {
	t.Helper()
	sock := respawnHome(t)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	hits := []string{}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		mu.Lock()
		hits = append(hits, key)
		mu.Unlock()
		rt, ok := routes[key]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if rt.status != 0 {
			w.WriteHeader(rt.status)
		}
		_, _ = w.Write([]byte(rt.body))
	})}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return &App{client: daemon.NewDaemonClient(sock)}, &hits
}

const shareTestURL = "https://mac.example.ts.net/d/?cap=SECRETCAP"

func TestDesktopShareBinding_NilClientNeverThrows(t *testing.T) {
	a := &App{}
	if got := a.StartDesktopShare(); got.ErrorCode != "unavailable" || got.URL != "" {
		t.Errorf("StartDesktopShare = %+v", got)
	}
	if got := a.GetDesktopShare(); got.ErrorCode != "unavailable" || got.Active {
		t.Errorf("GetDesktopShare = %+v", got)
	}
	for name, got := range map[string]DesktopShareActionResult{
		"stop":       a.StopDesktopShare(),
		"admit":      a.AdmitDesktopGuest("r1"),
		"deny":       a.DenyDesktopGuest("r1"),
		"rights":     a.SetDesktopViewerRights("v1", true, false),
		"disconnect": a.DisconnectDesktopViewer("v1"),
	} {
		if got.ErrorCode != "unavailable" {
			t.Errorf("%s = %+v, want unavailable", name, got)
		}
	}
}

func TestDesktopShareBinding_StartAndGet(t *testing.T) {
	var logs bytes.Buffer
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	a, _ := fakeShareDaemon(t, map[string]shareRoute{
		"POST /desktop/share": {body: `{"url":"` + shareTestURL + `","firstShare":true}`},
		"GET /desktop/share":  {body: `{"type":"state","active":true,"viewers":[],"pending":[],"url":"` + shareTestURL + `"}`},
	})
	if got := a.StartDesktopShare(); got != (DesktopShareResult{URL: shareTestURL, FirstShare: true}) {
		t.Errorf("StartDesktopShare = %+v", got)
	}
	if got := a.GetDesktopShare(); got != (DesktopShareInfo{Active: true, URL: shareTestURL}) {
		t.Errorf("GetDesktopShare = %+v", got)
	}
	if strings.Contains(logs.String(), "cap=") {
		t.Errorf("a binding logged the link: %q", logs.String())
	}
}

func TestDesktopShareBinding_StartRefusalsPassThrough(t *testing.T) {
	for _, code := range []string{"gui_required", "tailscale_off", "https_off", "tailscale_userspace", "start_failed"} {
		a, _ := fakeShareDaemon(t, map[string]shareRoute{
			"POST /desktop/share": {status: http.StatusConflict, body: `{"error":"` + code + `","detail":"why"}`},
		})
		got := a.StartDesktopShare()
		if got.ErrorCode != code || got.Detail != "why" || got.URL != "" {
			t.Errorf("code %s: StartDesktopShare = %+v", code, got)
		}
	}
}

func TestDesktopShareBinding_TransportErrors(t *testing.T) {
	// A daemon that is not listening: a transport error, not a daemon code.
	a := &App{client: daemon.NewDaemonClient(respawnHome(t))}
	if got := a.StartDesktopShare(); got.ErrorCode != "start_failed" || got.Detail == "" {
		t.Errorf("StartDesktopShare = %+v, want start_failed with the local error", got)
	}
	if got := a.StopDesktopShare(); got.ErrorCode != "stop_failed" {
		t.Errorf("StopDesktopShare = %+v, want stop_failed", got)
	}
	if got := a.GetDesktopShare(); got.ErrorCode != "unavailable" {
		t.Errorf("GetDesktopShare = %+v, want unavailable", got)
	}
	if got := a.AdmitDesktopGuest("r1"); got.ErrorCode != "unavailable" {
		t.Errorf("AdmitDesktopGuest = %+v, want unavailable", got)
	}
}

func TestDesktopShareBinding_ActionsPassIDAndMapRefusals(t *testing.T) {
	a, hits := fakeShareDaemon(t, map[string]shareRoute{
		"DELETE /desktop/share":                 {status: http.StatusNoContent},
		"POST /desktop/share/requests/r1/admit": {status: http.StatusNoContent},
		"POST /desktop/share/requests/r2/deny":  {status: http.StatusNotFound, body: `{"error":"no_request"}`},
		"PUT /desktop/share/viewers/v1/rights":  {status: http.StatusConflict, body: `{"error":"own_viewer"}`},
		"DELETE /desktop/share/viewers/v2":      {status: http.StatusNotFound, body: `{"error":"no_viewer"}`},
	})
	if got := a.StopDesktopShare(); got != (DesktopShareActionResult{}) {
		t.Errorf("StopDesktopShare = %+v", got)
	}
	if got := a.AdmitDesktopGuest("r1"); got != (DesktopShareActionResult{}) {
		t.Errorf("AdmitDesktopGuest = %+v", got)
	}
	if got := a.DenyDesktopGuest("r2"); got.ErrorCode != "no_request" {
		t.Errorf("DenyDesktopGuest = %+v", got)
	}
	if got := a.SetDesktopViewerRights("v1", true, true); got.ErrorCode != "own_viewer" {
		t.Errorf("SetDesktopViewerRights = %+v", got)
	}
	if got := a.DisconnectDesktopViewer("v2"); got.ErrorCode != "no_viewer" {
		t.Errorf("DisconnectDesktopViewer = %+v", got)
	}
	if len(*hits) != 5 {
		t.Errorf("hits = %v, want 5 daemon calls", *hits)
	}
}
