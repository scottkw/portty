//go:build vmdesktop

// VM-only simulated lapse for the Tart VM rows (19-12, row V1). A real 30-day
// approval lapse cannot be produced in a VM, so this drives the production
// transition into awaiting_approval and the production once-per-lapse
// notification (enterAwaiting, the claim, the daemon's sender). Never in a
// release build: the vmdesktop tag must not appear in .github, build.sh or
// scripts (tests/check-brand.sh enforces this in CI).

package desktop

import "math"

// SimulateLapse enters awaiting_approval exactly as the stall watchdog does at
// the end of a failed restart-and-nudge, and parks the watchdog where it would be
// after that (wdAwaiting, next retry 60 s out). Live frames cannot end it: the
// "a frame after the lapse began" count is set out of reach, so only
// ClearSimulatedLapse (or the retry cycle a minute on) ends it. A second call
// while the lapse is open changes nothing, as with the real transition.
func (m *Manager) SimulateLapse() {
	m.tickMu.Lock()
	defer m.tickMu.Unlock()
	now := m.d.Now().Round(0)
	m.enterAwaiting("simulated")
	m.noteAwaitSeq(math.MaxUint64)
	m.wd.phase, m.wd.retry, m.wd.at, m.wd.nextCheck = wdAwaiting, false, now, now.Add(awaitingRetry)
}

// ClearSimulatedLapse ends the lapse through the watchdog's own recovery.
func (m *Manager) ClearSimulatedLapse() {
	m.tickMu.Lock()
	defer m.tickMu.Unlock()
	m.stallRecovered()
}
