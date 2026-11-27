package main

import (
	"testing"
	"time"
)

// The observer itself refuses to run under go test; what is tested is the event it feeds.

// recordOcclusion swaps the emit seam for one that reports every call on a channel and restores it
// after the test. The pump goroutine reads the seam, so a test must receive what it emitted before it ends.
func recordOcclusion(t *testing.T) chan bool {
	t.Helper()
	prev := windowOccludedEmit
	t.Cleanup(func() { windowOccludedEmit = prev })
	got := make(chan bool, 16)
	windowOccludedEmit = func(hidden bool) { got <- hidden }
	return got
}

func TestEmitWindowOcclusion_SendsTheBooleanOnWindowOccluded(t *testing.T) {
	got := recordOcclusion(t)

	emitWindowOcclusion(true)
	emitWindowOcclusion(false)

	for i, want := range []bool{true, false} {
		select {
		case v := <-got:
			if v != want {
				t.Fatalf("emit %d = %v, want %v (order must be kept)", i, v, want)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("emit %d never delivered", i)
		}
	}
	if windowOccludedEvent != "window:occluded" {
		t.Fatalf("event name = %q, App.tsx listens for window:occluded", windowOccludedEvent)
	}
}

// 21-11 VM row D1: the real app died with SIGSEGV at launch when the Wails event was emitted from inside
// the Cocoa notification callback (the main thread, in a cgo callback). The callback must hand off and return.
func TestEmitWindowOcclusion_ReturnsBeforeTheEmitFinishes(t *testing.T) {
	prev := windowOccludedEmit
	t.Cleanup(func() { windowOccludedEmit = prev })
	entered, release := make(chan struct{}), make(chan struct{})
	windowOccludedEmit = func(bool) {
		close(entered)
		<-release
	}
	returned := make(chan struct{})
	go func() {
		emitWindowOcclusion(true)
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		close(release) // unblock whichever goroutine holds the emit before failing
		t.Fatal("emitWindowOcclusion blocked on the emit: the Cocoa callback would run Wails code on the main thread")
	}
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("the hand-off never reached the emit")
	}
	close(release)
}

func TestStartWindowOcclusionObserver_DoesNothingUnderTest(t *testing.T) {
	// Would register a main-queue observer on a real window if the guard were missing.
	startWindowOcclusionObserver()
}
