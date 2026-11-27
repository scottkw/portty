package desktop

// state.go: the desktop link's derived state (DPRM-02). Native reads feed a
// state loop; deriveState applies one precedence; every change is pushed to
// each viewer as a state message. Locked is an attribute that coexists with
// the viewer count, and Locked() is the synchronous, fail-closed read Phase 20's
// input gate uses.

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/scottkw/portty/internal/wakewatch"
)

const (
	lockCacheTTL     = 250 * time.Millisecond
	stateTickViewers = time.Second
	stateTickIdle    = 10 * time.Second
	idleProbeEvery   = 5 * time.Minute
)

// stateFlags is everything the precedence looks at besides the viewer count.
type stateFlags struct {
	paused, perm, asleep, awaiting, locked bool
}

// pickState is the precedence: capture_paused > needs_permission >
// display_asleep > awaiting_approval > locked > streaming > idle.
func pickState(f stateFlags, viewers int) State {
	switch {
	case f.paused:
		return StateCapturePaused
	case f.perm:
		return StateNeedsPermission
	case f.asleep:
		return StateDisplayAsleep
	case f.awaiting:
		return StateAwaitingApproval
	case f.locked:
		return StateLocked
	case viewers > 0:
		return StateStreaming
	}
	return StateIdle
}

// deriveState builds the Status from the flags the loop and the watchdog keep.
func (m *Manager) deriveState() Status {
	n := m.Viewers()
	m.smu.Lock()
	defer m.smu.Unlock()
	f := m.flagsLocked()
	return Status{
		State:            pickState(f, n),
		Viewers:          n,
		Locked:           f.locked,
		ApproachingLapse: m.approaching,
		LapseSeq:         m.lapseSeq,
		LapseClaimedSeq:  m.lapseClaimed,
		Reason:           m.reasonLocked(),
	}
}

func (m *Manager) flagsLocked() stateFlags {
	return stateFlags{paused: m.paused, perm: m.permDenied, asleep: m.asleep || m.noDisplay, awaiting: m.awaiting, locked: m.locked}
}

// noteReason records why a read or action failed (shown in Status.Reason) and
// logs it once until the same key recovers. A failure never stops capture.
func (m *Manager) noteReason(key string, err error) {
	m.smu.Lock()
	_, seen := m.reasons[key]
	m.reasons[key] = key + ": " + err.Error()
	m.smu.Unlock()
	if !seen {
		slog.Warn("desktop: "+key+" failed", "err", err)
	}
}

// noteDiagnostic records a reason that is not a failure (a software encoder).
func (m *Manager) noteDiagnostic(key, text string) {
	m.smu.Lock()
	m.reasons[key] = text
	m.smu.Unlock()
}

func (m *Manager) clearReason(key string) {
	m.smu.Lock()
	delete(m.reasons, key)
	m.smu.Unlock()
}

func (m *Manager) reasonLocked() string {
	if len(m.reasons) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m.reasons))
	for k := range m.reasons {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = m.reasons[k]
	}
	return strings.Join(parts, "; ")
}

// Locked reports whether the session screen is locked, cached for lockCacheTTL
// by wall clock. A failed read returns true: Phase 20's input gate must fail
// closed, never let input through on a Mac it cannot read.
func (m *Manager) Locked() bool {
	now := m.d.Now().Round(0)
	m.lmu.Lock()
	defer m.lmu.Unlock()
	if age := now.Sub(m.lockAt); m.lockKnown && age >= 0 && age < lockCacheTTL {
		return m.lockVal
	}
	v, err := m.d.Lock()
	if err != nil {
		m.noteReason("lock", err)
		v = true
	} else {
		m.clearReason("lock")
	}
	m.lockAt, m.lockVal, m.lockKnown = now, v, true
	return v
}

// noteFrame is the fan-out's mark that an access unit arrived: the watchdog's
// stall clock, and proof that a stream shown as needing permission is flowing.
func (m *Manager) noteFrame() {
	now := m.d.Now().Round(0)
	m.smu.Lock()
	m.lastFrameAt = now
	m.frames++
	m.permDenied = false
	m.smu.Unlock()
}

// kickLoop makes the state loop re-pace itself and tick now (a viewer joined or
// left, so its cadence and the state it pushes may both have changed).
func (m *Manager) kickLoop() {
	select {
	case m.kick <- struct{}{}:
	default:
	}
}

// stateLoop ticks every stateTickViewers while viewers exist and stateTickIdle
// otherwise. The timers only set the cadence: every decision compares wall clock.
func (m *Manager) stateLoop(ctx context.Context, wake <-chan wakewatch.Reason, done chan<- struct{}) {
	defer close(done)
	defer recoverLog()
	for {
		every := m.tickIdle
		if m.Viewers() > 0 {
			every = m.tickViewers
		}
		t := time.NewTimer(every)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case r, ok := <-wake:
			t.Stop()
			if !ok {
				wake = nil
			} else {
				m.noteWake(r)
			}
		case <-m.kick:
			t.Stop()
		case <-t.C:
		}
		m.tick(ctx)
	}
}

// tick is one pass of the loop: read the display and the lock, probe
// permissions when idle, and push the state if it changed.
func (m *Manager) tick(ctx context.Context) {
	m.tickMu.Lock()
	defer m.tickMu.Unlock()
	now := m.d.Now().Round(0)
	if a, err := m.d.DisplayAsleep(); err != nil {
		m.noteReason("display", err)
	} else {
		m.clearReason("display")
		m.smu.Lock()
		m.asleep = a
		m.smu.Unlock()
	}
	locked := m.Locked()
	m.smu.Lock()
	m.locked = locked
	m.smu.Unlock()

	m.refreshApproval(ctx, now, false)
	m.refreshAC(now, false)
	m.processWake(now)
	if m.Viewers() == 0 {
		m.idleProbe(ctx, now, false)
		m.wd.reset()
	} else {
		m.ensureSource(now)
		m.watchdogStep(ctx, now)
	}
	m.lapseDueCheck(now)
	m.pushIfChanged(ctx)
}

// idleProbe reads the permissions through the fresh child at most every
// idleProbeEvery while nobody watches (force skips the cadence). It never runs
// in-process (research C1). SR false is needs_permission; a probe error keeps
// the previous answer.
func (m *Manager) idleProbe(ctx context.Context, now time.Time, force bool) {
	m.smu.Lock()
	if age := now.Sub(m.lastProbeAt); !force && !m.lastProbeAt.IsZero() && age >= 0 && age < idleProbeEvery {
		m.smu.Unlock()
		return
	}
	m.lastProbeAt = now
	m.smu.Unlock()

	p, err := m.d.Probe(ctx)
	if err != nil {
		m.noteReason("probe", err)
		return
	}
	m.clearReason("probe")
	m.smu.Lock()
	m.permDenied = !p.SR
	m.smu.Unlock()
}

// RefreshPermissions probes now (the onboarding endpoint's poll) and pushes the
// resulting state. The Prober's one-second cache bounds the spawns.
func (m *Manager) RefreshPermissions(ctx context.Context) {
	m.idleProbe(ctx, m.d.Now().Round(0), true)
	m.pushIfChanged(ctx)
}

// stateMsg is the state message for st, with the audio failure the Manager holds.
func (m *Manager) stateMsg(st Status) msgState {
	msg := msgState{Type: typeState, State: st.State, Viewers: st.Viewers, Locked: st.Locked}
	if m.audioFail.Load() {
		msg.Audio = "failed"
	}
	return msg
}

// pushIfChanged broadcasts the state message when it differs from the last one.
func (m *Manager) pushIfChanged(ctx context.Context) {
	msg := m.stateMsg(m.deriveState())
	m.smu.Lock()
	changed := !m.pushedKnown || m.pushed != msg
	if changed {
		m.pushed, m.pushedKnown = msg, true
	}
	m.smu.Unlock()
	if changed {
		m.broadcast(ctx, msg)
	}
}
