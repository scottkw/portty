package relay

import "testing"

// fullReplay Joins a throwaway subscriber with no resume, unsubscribes it and
// returns the concatenated replay chunk payloads: what a fresh viewer replays.
func fullReplay(t *testing.T, hub *Hub) []byte {
	t.Helper()
	sub := &Subscriber{Msgs: make(chan []byte, 256), CloseSlow: func() {}}
	r := hub.Join(sub, 0, false)
	hub.Unsubscribe(sub)
	if r.Mode != ReplayFull {
		t.Fatalf("Join without resume: mode %q, want %q", r.Mode, ReplayFull)
	}
	var out []byte
	for _, c := range r.Chunks {
		out = append(out, c[1:]...)
	}
	return out
}
