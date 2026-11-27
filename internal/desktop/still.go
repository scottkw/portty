package desktop

// still.go: sharp when still (DESK-07). While the screen moves the encoder runs at
// the network's target, which is soft on small text. About 300 ms after the last
// real change the cached picture is re-encoded once as a keyframe at a higher rate,
// then a small non-key frame goes out every 2 s so a browser never decides a quiet
// stream has stalled. A real change ends the stillness at once. Nothing on screen
// tells the viewer: the picture just sharpens.

import "time"

const (
	stillAfter       = 300 * time.Millisecond // no fresh capture this long: refine once
	idleRefreshEvery = 2 * time.Second        // while still: one small refresh per this
	refineFactor     = 4                      // the refine keyframe may use this multiple of the target
)

type stillAction int

const (
	stillNone stillAction = iota
	stillRefine
	stillIdleRefresh
)

// refineBps is the bitrate for the refine keyframe: a multiple of the current
// target, never above the ceiling, so a weak link's burst stays proportionate (T-23-07).
func refineBps(target int) int { return min(ceilingBps, refineFactor*target) }

// stillMachine has two states, moving and still. It is pure: the caller feeds it
// the clock and says when a fresh capture arrived.
type stillMachine struct {
	lastFresh time.Time // the last unit from a new capture; zero before the first
	lastAct   time.Time // the last refine or idle refresh
	still     bool
}

// fresh notes a unit from a new capture: the screen moved, so it is moving again.
func (s *stillMachine) fresh(now time.Time) {
	s.lastFresh, s.still = now, false
}

// step says what to do now. Nothing happens before the first capture, while the
// display is asleep or while no viewer's media path is up: a refine then would be
// a burst nobody can use, so the machine waits and acts on the first tick after.
func (s *stillMachine) step(now time.Time, ready, asleep bool) stillAction {
	if s.lastFresh.IsZero() || asleep || !ready {
		return stillNone
	}
	switch {
	case !s.still && now.Sub(s.lastFresh) >= stillAfter:
		s.still, s.lastAct = true, now
		return stillRefine
	case s.still && now.Sub(s.lastAct) >= idleRefreshEvery:
		s.lastAct = now
		return stillIdleRefresh
	}
	return stillNone
}
