package desktop

// encoder_recovery.go: recovery from a wedged VideoToolbox encoder service
// (U-load). Under extreme load the encoder service launchd runs for this
// process can end up refusing every new encoder; every join then fails until
// that service is gone, and killing it by hand fixed joins on the same daemon.
// The daemon itself is never restarted (it holds the terminal sessions): after
// encoderResetAfter consecutive encoder start failures it ends its own service
// instance (Deps.ResetEncoder) and the next start gets a fresh one.

import (
	"errors"
	"log/slog"
	"time"

	"github.com/scottkw/portty/internal/screencap"
)

const (
	encoderResetAfter = 3           // consecutive encoder start failures before a reset; never the first
	encoderResetEvery = time.Minute // at least this long between resets
)

// encoderRecovery is the reset policy, guarded by Manager.smu.
type encoderRecovery struct {
	fails     int       // encoder start failures since the last good start or reset
	lastReset time.Time // zero until the first reset
}

// note records one capture start and reports whether to reset the encoder
// service now. A good start clears the count; a start that failed short of the
// encoder (permission, display, capture) neither counts nor clears.
func (r *encoderRecovery) note(err error, now time.Time) bool {
	if err == nil {
		r.fails = 0
		return false
	}
	if !errors.Is(err, screencap.ErrEncoderStart) {
		return false
	}
	r.fails++
	if r.fails < encoderResetAfter {
		return false
	}
	// A clock stepped back reads as elapsed, so it never holds resets off.
	if since := now.Sub(r.lastReset); !r.lastReset.IsZero() && since >= 0 && since < encoderResetEvery {
		return false
	}
	r.fails, r.lastReset = 0, now
	return true
}

// encoderOutcome feeds a capture start's result to the policy and resets the
// encoder service when it says so. startSource calls it before taking mu: no
// source runs and no other start can begin (m.starting), so nothing of this
// process is using the service. The failure shows in Status.Reason until a
// start works.
func (m *Manager) encoderOutcome(err error, now time.Time) {
	m.smu.Lock()
	reset := m.enc.note(err, now)
	m.smu.Unlock()
	switch {
	case err == nil:
		m.clearReason("encoder")
	case errors.Is(err, screencap.ErrEncoderStart):
		m.noteReason("encoder", err)
	}
	if !reset {
		return
	}
	pids, rerr := m.d.ResetEncoder()
	switch {
	case len(pids) > 0:
		slog.Warn("desktop: the encoder keeps failing to start; ended this process's encoder service so macOS starts a fresh one", "pids", pids, "err", rerr)
	case rerr != nil:
		slog.Warn("desktop: the encoder keeps failing to start; resetting its service failed", "err", rerr)
	default:
		slog.Warn("desktop: the encoder keeps failing to start; no encoder service of this process was found to reset")
	}
}
