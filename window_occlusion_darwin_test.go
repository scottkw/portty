package main

import "testing"

// The observer itself refuses to run under go test; what is tested is the event it feeds.
func TestEmitWindowOcclusion_SendsTheBooleanOnWindowOccluded(t *testing.T) {
	prev := windowOccludedEmit
	t.Cleanup(func() { windowOccludedEmit = prev })
	var got []bool
	windowOccludedEmit = func(hidden bool) { got = append(got, hidden) }

	emitWindowOcclusion(true)
	emitWindowOcclusion(false)

	if len(got) != 2 || !got[0] || got[1] {
		t.Fatalf("emitted %v, want [true false]", got)
	}
	if windowOccludedEvent != "window:occluded" {
		t.Fatalf("event name = %q, App.tsx listens for window:occluded", windowOccludedEvent)
	}
}

func TestStartWindowOcclusionObserver_DoesNothingUnderTest(t *testing.T) {
	// Would register a main-queue observer on a real window if the guard were missing.
	startWindowOcclusionObserver()
}
