package desktop

// power.go: keeping the Mac reachable and the picture alive (DPRM-04). From the
// first viewer join to the last leave the Manager holds a display and a system
// idle-sleep assertion, and declares remote user activity on every join (it
// lights a dark display; ScreenCaptureKit needs it on). While the Manager runs
// on AC power it also holds a system-sleep assertion (the user chose "only
// while plugged in"); on battery nothing is held without a viewer. After a wake
// or a -3815 stop the stream restarts once the display reads awake. Every
// elapsed-time comparison is wall clock (Round(0)): a monotonic timer pauses
// across sleep on this platform.

import (
	"errors"
	"log/slog"
	"time"

	"github.com/scottkw/portty/internal/screencap"
	"github.com/scottkw/portty/internal/wakewatch"
)

const (
	viewerAssertionName = "Portty desktop viewer connected"
	acAssertionName     = "Portty always-available desktop on AC power"
	acRecheck           = 30 * time.Second
	wakeDisplayWait     = 30 * time.Second
)

// declareActivity runs on every join, before the source starts. A failure is a
// reason in Status; the join goes on.
func (m *Manager) declareActivity() {
	if err := m.d.DeclareActivity(viewerAssertionName); err != nil {
		m.noteReason("user activity", err)
		return
	}
	m.clearReason("user activity")
}

// takeViewerHolds takes the viewer's display and system assertions (m.mu held,
// the first viewer joining). One that is already held is not taken again, one
// that failed is tried again on the next join.
func (m *Manager) takeViewerHolds() {
	m.pmu.Lock()
	defer m.pmu.Unlock()
	m.relDisplay = m.hold(m.relDisplay, m.d.HoldDisplay, viewerAssertionName, "display hold")
	m.relSystem = m.hold(m.relSystem, m.d.HoldSystem, viewerAssertionName, "system hold")
}

// hold takes an assertion unless cur already holds one; a failure is recorded
// under key and never stops capture.
func (m *Manager) hold(cur func() error, take func(string) (func() error, error), name, key string) func() error {
	if cur != nil {
		return cur
	}
	rel, err := take(name)
	if err != nil {
		m.noteReason(key, err)
		return nil
	}
	m.clearReason(key)
	return rel
}

// releaseViewerHolds gives the viewer assertions back (the last viewer left).
func (m *Manager) releaseViewerHolds() {
	m.pmu.Lock()
	defer m.pmu.Unlock()
	m.relDisplay = m.release(m.relDisplay)
	m.relSystem = m.release(m.relSystem)
}

// releaseAllHolds is Close: the viewer assertions and the AC one.
func (m *Manager) releaseAllHolds() {
	m.pmu.Lock()
	defer m.pmu.Unlock()
	m.relDisplay = m.release(m.relDisplay)
	m.relSystem = m.release(m.relSystem)
	m.relAC = m.release(m.relAC)
}

func (m *Manager) release(rel func() error) func() error {
	if rel != nil {
		if err := rel(); err != nil {
			m.noteReason("release", err)
		}
	}
	return nil
}

// refreshAC re-reads the power source at most every acRecheck: on AC the
// always-available link holds PreventUserIdleSystemSleep, on battery it
// holds none. A failed read keeps the current state.
func (m *Manager) refreshAC(now time.Time, force bool) {
	m.pmu.Lock()
	defer m.pmu.Unlock()
	if age := now.Sub(m.acAt); !force && !m.acAt.IsZero() && age >= 0 && age < acRecheck {
		return
	}
	m.acAt = now
	onAC, err := m.d.OnAC()
	if err != nil {
		m.noteReason("power source", err)
		return
	}
	m.clearReason("power source")
	switch {
	case onAC && m.relAC == nil:
		m.relAC = m.hold(nil, m.d.HoldSystem, acAssertionName, "ac hold")
	case !onAC && m.relAC != nil:
		m.relAC = m.release(m.relAC)
	}
}

// noteWake is the state loop's reaction to a wall-clock wake: the stream is
// restarted once the display reads awake.
func (m *Manager) noteWake(r wakewatch.Reason) {
	if r != wakewatch.Woke {
		return
	}
	now := m.d.Now().Round(0)
	m.smu.Lock()
	m.wakeAt = now
	m.smu.Unlock()
	slog.Info("desktop: wake, restarting the stream once the display is awake")
}

// processWake restarts the stream for a pending wake once the display reads
// awake, or gives up after wakeDisplayWait of wall clock (the stall watchdog is
// the fallback). It runs after the tick's display read.
func (m *Manager) processWake(now time.Time) {
	m.smu.Lock()
	at, asleep := m.wakeAt, m.asleep
	if age := now.Sub(at); !at.IsZero() && age < 0 {
		at = now // the clock stepped back: count from here
		m.wakeAt = now
	}
	m.smu.Unlock()
	if at.IsZero() {
		return
	}
	switch {
	case m.Viewers() == 0:
	case !asleep:
		_ = m.restartSource() // a failure was classified and delays the next try
	case now.Sub(at) < wakeDisplayWait:
		return
	default:
		slog.Warn("desktop: display still asleep after the wake wait; not restarting")
	}
	m.smu.Lock()
	m.wakeAt = time.Time{}
	m.smu.Unlock()
}

// noteSourceError reacts to an error a running source reported: -3815 means the
// display is gone (shown as display_asleep, restarted once it is awake), -3801
// that Screen Recording was switched off.
func (m *Manager) noteSourceError(err error) {
	m.smu.Lock()
	switch {
	case errors.Is(err, screencap.ErrNoDisplay):
		m.noDisplay = true
	case errors.Is(err, screencap.ErrPermissionDenied):
		m.permDenied = true
	}
	m.smu.Unlock()
}

// isAsleep reports whether the picture must not be sent: the display reads
// asleep or the source said it has no display. A stale cached frame would
// otherwise reach a viewer joining meanwhile.
func (m *Manager) isAsleep() bool {
	m.smu.Lock()
	defer m.smu.Unlock()
	return m.asleep || m.noDisplay
}
