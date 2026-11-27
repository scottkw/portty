package main

import (
	"context"
	"errors"
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
	a := &App{showWindowFunc: func() { shown++ }}
	h := newShareHarness(a)
	a.shareDesktopFromTray()
	if shown != 1 || h.count("desktopshare:open-sheet") != 1 {
		t.Errorf("shown=%d open-sheet=%d, want 1 and 1", shown, h.count("desktopshare:open-sheet"))
	}
}
