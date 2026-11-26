package desktop

// watchdog.go: the stall watchdog and the approval lapse (DPRM-03). A still
// screen and a dead stream both deliver no frames (R19), so silence alone says
// nothing. While a viewer waits and no access unit has arrived for stallAfter,
// the watchdog nudges the pointer (Accessibility only): a frame within a second
// is a still screen and the next check waits stillRecheck; no frame restarts the
// stream and classifies it: a start error -3801 is needs_permission, -3815 is
// display_asleep, a stream that started but yields nothing after a second nudge
// is awaiting_approval. Awaiting approval is never derived from zero frames or
// a frame count alone, and never without Accessibility.
//
// It is a state machine advanced by the state loop (one step per tick), so every
// elapsed-time check is a wall-clock comparison against Deps.Now.

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

const (
	stallAfter       = 5 * time.Second
	stillRecheck     = 30 * time.Second
	nudgeFrameWait   = time.Second
	restartFrameWait = 2 * time.Second
	awaitingRetry    = 60 * time.Second
	retryStartEvery  = 10 * time.Second

	lapseNotificationID = "portty.desktop-approval-lapse"
	lapseTitle          = "Portty"
	lapseBody           = "Portty desktop needs approval at the Mac."

	// guiAttachWindow is how recent the GUI's last NoteGUISeen must be for the
	// GUI to count as running; with one, the daemon waits guiClaimWait for the
	// GUI's claim before it posts the lapse itself.
	guiAttachWindow = 12 * time.Second
	guiClaimWait    = 15 * time.Second
)

type wdPhase int

const (
	wdIdle      wdPhase = iota
	wdNudged            // nudged; waiting for a frame
	wdRestarted         // stream restarted; waiting for a frame
	wdNudged2           // nudged after the restart; no frame means awaiting approval
	wdAwaiting          // classified; retrying the restart every awaitingRetry
)

// watchdog is the state machine's memory, guarded by Manager.tickMu.
type watchdog struct {
	phase     wdPhase
	at        time.Time // when the phase began
	seq       uint64    // the frame counter when the phase began
	nextCheck time.Time // no stall check before this
	retry     bool      // the restart in flight is an awaiting retry
	noAX      bool      // the stall was handled without a nudge (no Accessibility)
	noAXLog   bool
}

// reset returns to idle (no viewer, or capture not running), keeping the
// once-only log flag.
func (w *watchdog) reset() { *w = watchdog{noAXLog: w.noAXLog} }

// watchdogStep advances the machine one step.
func (m *Manager) watchdogStep(ctx context.Context, now time.Time) {
	m.smu.Lock()
	blocked := m.paused || m.permDenied || m.asleep || m.noDisplay || !m.wakeAt.IsZero()
	last, frames, awaiting, awaitSeq := m.lastFrameAt, m.frames, m.awaiting, m.awaitSeq
	m.smu.Unlock()
	m.mu.Lock()
	running := m.src != nil
	m.mu.Unlock()
	w := &m.wd
	if !running || blocked || m.Viewers() == 0 {
		w.reset()
		return
	}

	// Any frame after the lapse began ends it, except during a retry cycle, where
	// the restarted stream may emit a frame on its own: only a frame after a nudge counts.
	inRetry := w.retry && (w.phase == wdRestarted || w.phase == wdNudged2)
	if awaiting && !inRetry && frames > awaitSeq {
		m.stallRecovered()
		return
	}

	switch w.phase {
	case wdIdle:
		if now.Before(w.nextCheck) || now.Sub(last) < stallAfter {
			return
		}
		if m.axOn(ctx) {
			w.seq = frames // before the nudge: a frame it causes must count
			m.nudge()
			w.phase, w.at = wdNudged, now
			return
		}
		// Without Accessibility a still screen cannot be told from a frozen stream,
		// so restarting is rationed: one restart per awaitingRetry, not per stall.
		w.noAX = true
		m.restartForStall(now)
	case wdNudged:
		if frames > w.seq {
			slog.Debug("desktop: still screen")
			w.phase, w.nextCheck = wdIdle, now.Add(stillRecheck)
			return
		}
		if now.Sub(w.at) >= nudgeFrameWait {
			m.restartForStall(now)
		}
	case wdRestarted:
		// A restart that was a retry of an open lapse ignores the frame the new
		// stream may emit on its own: only a frame after a nudge ends the lapse.
		if frames > w.seq && !w.retry {
			m.stallRecovered()
			return
		}
		if now.Sub(w.at) < restartFrameWait {
			return
		}
		if !m.axOn(ctx) {
			w.phase, w.nextCheck = wdIdle, now.Add(awaitingRetry)
			return
		}
		w.seq = frames
		m.nudge()
		w.phase, w.at = wdNudged2, now
	case wdNudged2:
		if frames > w.seq {
			m.stallRecovered()
			return
		}
		if now.Sub(w.at) >= restartFrameWait {
			m.enterAwaiting("watchdog")
			m.noteAwaitSeq(frames)
			w.phase, w.retry, w.at, w.nextCheck = wdAwaiting, false, now, now.Add(awaitingRetry)
		}
	case wdAwaiting:
		if !awaiting {
			w.phase = wdIdle
			return
		}
		if !now.Before(w.nextCheck) {
			w.retry = true
			m.restartForStall(now)
		}
	}
}

// restartForStall restarts the stream. A failed start was classified by
// startSourceLocked (needs_permission, display_asleep); the machine goes idle
// and waits out the retry delay.
func (m *Manager) restartForStall(now time.Time) {
	w := &m.wd
	w.seq = m.frameCount()
	if err := m.restartSource(); err != nil {
		w.phase, w.retry = wdIdle, false
		w.nextCheck = now.Add(awaitingRetry)
		return
	}
	w.phase, w.at = wdRestarted, now
}

func (m *Manager) frameCount() uint64 {
	m.smu.Lock()
	defer m.smu.Unlock()
	return m.frames
}

// stallRecovered ends the episode: frames are back (and any open lapse is over).
func (m *Manager) stallRecovered() {
	w := &m.wd
	w.phase, w.retry, w.nextCheck = wdIdle, false, time.Time{}
	if w.noAX {
		w.noAX, w.nextCheck = false, m.d.Now().Round(0).Add(awaitingRetry)
	}
	m.smu.Lock()
	m.awaiting = false
	m.lapseDue = time.Time{}
	m.smu.Unlock()
	slog.Info("desktop: stall recovered")
}

// axOn reads Accessibility through the fresh child. Without it the pointer is
// never nudged and awaiting approval is never classified (logged once).
func (m *Manager) axOn(ctx context.Context) bool {
	p, err := m.d.Probe(ctx)
	if err != nil {
		m.noteReason("probe", err)
	} else {
		m.clearReason("probe")
		if p.AX {
			return true
		}
	}
	if !m.wd.noAXLog {
		m.wd.noAXLog = true
		slog.Warn("desktop: Accessibility is off: a still screen cannot be told from a frozen stream, so awaiting approval is never classified")
	}
	return false
}

// nudge moves the pointer one point and back. Only with a viewer waiting.
func (m *Manager) nudge() {
	if m.Viewers() == 0 {
		return
	}
	ok, err := m.d.Nudge()
	switch {
	case err != nil:
		m.noteReason("nudge", err)
	case !ok:
		m.noteReason("nudge", errors.New("the pointer did not move and return"))
	default:
		m.clearReason("nudge")
	}
}

// enterAwaiting is the one transition into awaiting_approval, whoever decides
// it: under the state mutex it records the state and takes the next LapseSeq
// once. Repeated calls while still awaiting change nothing. who names the
// detector in the log.
func (m *Manager) enterAwaiting(who string) {
	now := m.d.Now().Round(0)
	m.smu.Lock()
	if m.awaiting {
		m.smu.Unlock()
		return
	}
	m.awaiting = true
	m.awaitSeq = m.frames
	m.lapseSeq++
	seq := m.lapseSeq
	guiUp := !m.guiSeenAt.IsZero() && now.Sub(m.guiSeenAt) >= 0 && now.Sub(m.guiSeenAt) < guiAttachWindow
	if guiUp {
		m.lapseDue = now.Add(guiClaimWait)
	}
	m.smu.Unlock()
	slog.Warn("desktop: waiting for approval at the Mac", "lapse", seq, "detector", who, "gui", guiUp)
	if !guiUp {
		m.postLapse(seq)
	}
	m.pushIfChanged(context.Background())
}

// noteAwaitSeq restarts the "a frame ends the lapse" count after a retry cycle.
func (m *Manager) noteAwaitSeq(frames uint64) {
	m.smu.Lock()
	m.awaitSeq = frames
	m.smu.Unlock()
}

// NoteGUISeen records that the GUI is running (its tray poll calls it, 19-14).
func (m *Manager) NoteGUISeen() {
	now := m.d.Now().Round(0)
	m.smu.Lock()
	m.guiSeenAt = now
	m.smu.Unlock()
}

// ClaimLapse claims the right to notify lapse seq for claimant. It succeeds
// once, only for the current LapseSeq; atomic under the state mutex, so the GUI
// and the daemon never both post.
func (m *Manager) ClaimLapse(seq int, claimant string) bool {
	m.smu.Lock()
	if seq <= 0 || seq != m.lapseSeq || m.lapseClaimed == seq {
		m.smu.Unlock()
		return false
	}
	m.lapseClaimed = seq
	m.smu.Unlock()
	slog.Info("desktop: lapse claimed", "lapse", seq, "by", claimant)
	return true
}

// postLapse is the daemon's sender: claim, then notify outside any lock.
func (m *Manager) postLapse(seq int) {
	if !m.ClaimLapse(seq, "daemon") {
		return
	}
	if m.d.Notify == nil {
		slog.Warn("desktop: lapse notification not wired")
		return
	}
	m.d.Notify(lapseNotificationID, lapseTitle, lapseBody)
}

// lapseDueCheck posts a lapse the GUI had time to claim and did not (it died or
// quit), but only while the lapse is still open: a recovery posts nothing.
func (m *Manager) lapseDueCheck(now time.Time) {
	m.smu.Lock()
	seq, due := m.lapseSeq, !m.lapseDue.IsZero() && !now.Before(m.lapseDue)
	if due {
		m.lapseDue = time.Time{}
		due = m.awaiting && m.lapseClaimed != seq
	}
	m.smu.Unlock()
	if due {
		m.postLapse(seq)
	}
}

// ensureSource retries a capture start that failed, or restarts a source that
// ended or reported no display, while a viewer waits (permission granted since,
// display back). A paused breaker or a sleeping display waits.
func (m *Manager) ensureSource(now time.Time) {
	m.mu.Lock()
	dead := false // the source ended by itself (its frames channel closed)
	if m.src != nil {
		select {
		case <-m.fanDone:
			dead = true
		default:
		}
	}
	gone := m.src == nil || dead
	closed := m.closed
	m.mu.Unlock()
	m.smu.Lock()
	need := (gone || m.noDisplay) && !closed
	ready := !m.paused && !m.asleep && !now.Before(m.nextStartAt)
	m.smu.Unlock()
	if need && ready {
		_ = m.restartSource() // a failure was classified and delays the next try
	}
}
