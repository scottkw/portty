package daemon

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/pty"
	"github.com/scottkw/portty/internal/testutil"
)

// TestExitEvent_ExitWatcher_TransitionsToStopped verifies that when a session's
// process exits naturally (hub.Done() fires), the exit watcher goroutine transitions
// the session state to StateStopped and calls the onExit callback with the session ID
// and exit code.
//
// Uses the spy backend: Read returns EOF immediately, so hub.Done() fires quickly.
func TestExitEvent_ExitWatcher_TransitionsToStopped(t *testing.T) {
	spy := &spyBackend{}
	e := newCatEngine()
	e.backend = spy

	var (
		mu           sync.Mutex
		callbackID   string
		callbackCode int
		called       = make(chan struct{})
	)

	onExit := func(id string, code int) {
		mu.Lock()
		callbackID = id
		callbackCode = code
		mu.Unlock()
		close(called)
	}

	id, err := e.CreateSession(context.Background(), "exit-test", "", 80, 24, onExit)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// hub.Done() fires when the hub's Run goroutine returns EOF from the spy session.
	// spy session Read returns error immediately (nil pty), so this is fast.
	select {
	case <-called:
	case <-time.After(3 * time.Second):
		t.Fatal("onExit callback not called within 3s — exit watcher goroutine may not have fired")
	}

	// Verify callback received correct session ID.
	mu.Lock()
	gotID := callbackID
	gotCode := callbackCode
	mu.Unlock()

	if gotID != id {
		t.Errorf("onExit sessionID: got %q, want %q", gotID, id)
	}
	// spy cmd is nil, so WaitForExit returns 0.
	if gotCode != 0 {
		t.Errorf("onExit exitCode: got %d, want 0 (nil cmd)", gotCode)
	}

	// Verify session state was transitioned to StateStopped.
	sess, ok := e.registry.Get(id)
	if !ok {
		t.Fatal("session not found in registry after exit")
	}
	// Give a brief moment for the goroutine to complete SetState after onExit returns.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if sess.State == pty.StateStopped {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if sess.State != pty.StateStopped {
		t.Errorf("session state after exit: got %v, want StateStopped", sess.State)
	}
}

// TestExitEvent_ExitWatcher_NilOnExitNoIsPanic verifies that passing nil for onExit
// does not cause a panic when the exit watcher fires.
func TestExitEvent_ExitWatcher_NilOnExitNoIsPanic(t *testing.T) {
	spy := &spyBackend{}
	e := newCatEngine()
	e.backend = spy

	id, err := e.CreateSession(context.Background(), "nil-exit-test", "", 80, 24, nil)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Wait for hub to shut down — if it panics, the test will fail.
	hub, ok := e.manager.Get(id)
	if !ok {
		t.Fatal("hub not found")
	}
	select {
	case <-hub.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("hub.Done() not closed within 3s")
	}
	// Give exit watcher a moment to complete without panicking.
	time.Sleep(50 * time.Millisecond)
}

// TestExitEvent_ListSessions_RunningHasNoExitCode verifies that ListSessions
// leaves ExitCode and Duration nil for a running session. It uses the real cat
// backend: /bin/cat on a real PTY stays running until cleanup kills it, so no
// exit watcher can flip the state under the assertion.
func TestExitEvent_ListSessions_RunningHasNoExitCode(t *testing.T) {
	e := newCatEngine()
	id, err := e.CreateSession(context.Background(), "running-session", "", 80, 24, nil)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	t.Cleanup(func() { _ = e.KillSession(id) })

	info := findSessionByID(e.ListSessions(), id)
	if info.ID == "" {
		t.Fatal("session not found in ListSessions")
	}
	if info.State != "running" {
		t.Fatalf("State: got %q, want running", info.State)
	}
	if info.ExitCode != nil {
		t.Errorf("ExitCode for running session: want nil, got %d", *info.ExitCode)
	}
	if info.Duration != nil {
		t.Errorf("Duration for running session: want nil, got %d", *info.Duration)
	}
}

// TestExitEvent_ListSessions_ExitCodePopulatedForStopped verifies that ListSessions
// populates ExitCode and Duration for a session the exit watcher stopped.
//
// The spy session has no PTY, so the hub's first Read fails and the exit watcher
// sets StateStopped by itself within microseconds. The test waits for that
// instead of forcing SetState: the old version asserted "running" first and then
// set the state manually, both racing the watcher (INPT-05).
func TestExitEvent_ListSessions_ExitCodePopulatedForStopped(t *testing.T) {
	e := newCatEngine()
	e.backend = &spyBackend{}

	id, err := e.CreateSession(context.Background(), "stopped-session", "", 80, 24, nil)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	sess, ok := e.registry.Get(id)
	if !ok {
		t.Fatal("session not found in registry")
	}
	testutil.WaitFor(t, 3*time.Second, func() bool { return sess.GetState() == pty.StateStopped },
		"exit watcher never stopped the spy session")

	info := findSessionByID(e.ListSessions(), id)
	if info.ID == "" {
		t.Fatal("session not found in ListSessions")
	}
	if info.ExitCode == nil {
		t.Fatal("ExitCode for stopped session: want non-nil *int, got nil")
	}
	// The spy session has no cmd, so the cached exit code normalizes to 0.
	if *info.ExitCode != 0 {
		t.Errorf("ExitCode value for stopped spy session: got %d, want 0", *info.ExitCode)
	}
	if info.Duration == nil {
		t.Error("Duration for stopped session: want non-nil *int, got nil")
	}
}
