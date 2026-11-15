package main

// TERM-05 / TERM-02: the app-wide exit watcher (startExitWatcher/checkExits)
// reports every stopped session exactly once, with no time window, and leaves
// running sessions alone across a GUI relaunch.

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

type exitEvent struct {
	name      string
	sessionID string
}

// eventRecorder is a fake emitEventFunc. The watcher goroutine may call it, so
// it records behind a mutex.
type eventRecorder struct {
	mu     sync.Mutex
	events []exitEvent
}

func (r *eventRecorder) emit(name string, data any) {
	id := ""
	if m, ok := data.(map[string]any); ok {
		id, _ = m["sessionId"].(string)
	}
	r.mu.Lock()
	r.events = append(r.events, exitEvent{name, id})
	r.mu.Unlock()
}

func (r *eventRecorder) exitsFor(id string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, e := range r.events {
		if e.name == "session:exit" && e.sessionID == id {
			n++
		}
	}
	return n
}

func (r *eventRecorder) total() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.events)
}

// sessionState returns the daemon's State for id, or "" when it is not listed.
func sessionState(t *testing.T, app *App, id string) string {
	t.Helper()
	sessions, err := app.client.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	for _, s := range sessions {
		if s.ID == id {
			return s.State
		}
	}
	return ""
}

// checkUntilExit calls checkExits until id has been reported or 5 s pass.
func checkUntilExit(t *testing.T, app *App, rec *eventRecorder, id string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for rec.exitsFor(id) == 0 {
		if time.Now().After(deadline) {
			t.Fatalf("no session:exit for %s within 5s", id)
		}
		app.checkExits()
		time.Sleep(50 * time.Millisecond)
	}
}

func shApp(t *testing.T, script string) (*App, *eventRecorder, string) {
	t.Helper()
	app := testAppWithSpawn(t, "/bin/sh", []string{"-c", script})
	rec := &eventRecorder{}
	app.emitEventFunc = rec.emit
	id, err := app.CreateSession("exit-test", "", 0, 0)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	return app, rec, id
}

func TestCheckExits_EmitsOnceWhenShellExits(t *testing.T) {
	app, rec, id := shApp(t, "sleep 1; exit 3")
	checkUntilExit(t, app, rec, id)
	for i := 0; i < 5; i++ {
		app.checkExits()
	}
	if n := rec.exitsFor(id); n != 1 {
		t.Fatalf("session:exit count = %d, want exactly 1", n)
	}
	if n := rec.total(); n != 1 {
		t.Fatalf("total events = %d, want 1", n)
	}
}

func TestCheckExits_LongLivedSessionStillReported(t *testing.T) {
	app, rec, id := shApp(t, "sleep 2; exit 0")
	// ~20 checks while the shell runs (well under its 2 s life) emit nothing.
	for i := 0; i < 20; i++ {
		app.checkExits()
		time.Sleep(50 * time.Millisecond)
	}
	if n := rec.total(); n != 0 {
		t.Fatalf("events while running = %d, want 0", n)
	}
	checkUntilExit(t, app, rec, id)
	app.checkExits()
	if n := rec.exitsFor(id); n != 1 {
		t.Fatalf("session:exit count = %d, want exactly 1", n)
	}
}

func TestCheckExits_StoppedAtFirstSight(t *testing.T) {
	app, rec, id := shApp(t, "exit 0")
	deadline := time.Now().Add(5 * time.Second)
	for sessionState(t, app, id) != "stopped" {
		if time.Now().After(deadline) {
			t.Fatal("session never reached stopped")
		}
		time.Sleep(50 * time.Millisecond)
	}
	app.checkExits() // first call ever: no cold-start baseline
	if n := rec.exitsFor(id); n != 1 {
		t.Fatalf("session:exit count after first check = %d, want 1", n)
	}
}

func TestCheckExits_PrunesRemovedSessions(t *testing.T) {
	app, rec, id := shApp(t, "exit 0")
	checkUntilExit(t, app, rec, id)
	if err := app.KillSession(id); err != nil {
		t.Fatalf("KillSession: %v", err)
	}
	if st := sessionState(t, app, id); st != "" {
		t.Fatalf("session still listed after KillSession (state %q)", st)
	}
	app.checkExits()
	if len(app.exitEmitted) != 0 {
		t.Fatalf("exitEmitted = %v, want empty after prune", app.exitEmitted)
	}
}

func TestExitWatcher_GUIRelaunchKeepsRunningSessions(t *testing.T) {
	first, rec1, id := shApp(t, "echo SCROLLBACK_MARK_42; exec sleep 60")

	ctx1, cancel1 := context.WithCancel(context.Background())
	first.startExitWatcher(ctx1)
	time.Sleep(1200 * time.Millisecond) // two or more 500 ms ticks
	cancel1()                           // GUI quits; no KillSession
	if n := rec1.exitsFor(id); n != 0 {
		t.Fatalf("first GUI emitted %d session:exit for a running session", n)
	}

	rec2 := &eventRecorder{}
	second := &App{client: first.client, emitEventFunc: rec2.emit} // relaunched GUI, same daemon
	if st := sessionState(t, second, id); st != "running" {
		t.Fatalf("relaunched GUI sees state %q, want running", st)
	}
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(strings.Join(second.GetSessionTailLines(id, 20), "\n"), "SCROLLBACK_MARK_42") {
		if time.Now().After(deadline) {
			t.Fatalf("scrollback lost: tail = %q", second.GetSessionTailLines(id, 20))
		}
		time.Sleep(100 * time.Millisecond)
	}
	second.checkExits()
	if n := rec2.total(); n != 0 {
		t.Fatalf("relaunched GUI emitted %d events for a running session", n)
	}
}
