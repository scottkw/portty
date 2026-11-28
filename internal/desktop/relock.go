package desktop

// relock.go: after a Portty unlock the Mac locks itself again once the last
// own-account viewer has left and none came back within relockGrace, unless the
// person at the Mac used it in the meantime (UNLK-01). An unlocked office Mac
// must never be left open after the user disconnects from home.
//
// Fail secure: physical input counts only from a healthy listen-only tap, and
// any unhealthy or unavailable reading while the rule is armed makes the
// decision lock. The tap is kept running while armed (lease.go syncTap).
//
// A posted chord is not a locked Mac (RL-01r: under load with a busy app in
// front macOS dropped it). The rule stays armed until a lock read says locked,
// and only that read writes relocked; a Mac still unlocked gets the chord again
// relockConfirm apart, and after relockTries unconfirmed posts relock_failed is
// logged, counted and audited and the chord is posted every relockBackoff.
//
// The rule is armed only by a Portty unlock, never by an unlock at the Mac's
// keyboard, and it survives a daemon restart through desktop-state.json. It is
// driven by the lease loop's one-second poll, so the 30 s is "about": the lock
// comes at the first poll at or after it.
//
// Lock order: rl.mu may be held while taking vmu (the own-account count), never
// the other way round.

import (
	"log/slog"
	"sync"
	"time"

	"github.com/scottkw/portty/internal/screencap"
)

// relockGrace is how long the Mac waits, with no own-account viewer connected,
// before it locks again.
const relockGrace = 30 * time.Second

// A posted chord has relockConfirm to show as a locked Mac before it is posted
// again; after relockTries such posts the re-lock is reported failed and the
// chord is posted every relockBackoff while the rule stays armed.
const (
	relockConfirm = 5 * time.Second
	relockTries   = 3
	relockBackoff = 30 * time.Second
)

// relockState is the re-lock rule's bookkeeping. pollMu serialises polls (the
// lease loop and tests), so one decision is taken at a time; mu guards the rest
// and is a leaf apart from the vmu read noted above.
type relockState struct {
	pollMu sync.Mutex

	mu      sync.Mutex
	armed   bool
	since   time.Time // untagged input after this counts as someone at the Mac: the unlock, or the daemon's start after a restart
	graceAt time.Time // decide at the first poll at or after this (the grace's end, then each re-post); zero while an own-account viewer is connected
	posts   int       // chords posted since the grace ended that the Mac has not been read locked after
	seen    bool      // a healthy tap showed untagged input after since
	unknown bool      // some reading while armed was not healthy
	who     Identity  // the last own-account viewer, for the relocked and relock_failed audit lines
	failing bool      // the last chord post failed (logged once per streak)
	resume  bool      // desktop-state.json said armed when this process started
}

// ownViewers counts the connected own-account viewers whose join worked; one
// whose capture start is still in flight, or failed, is not a viewer yet.
func (m *Manager) ownViewers() int {
	m.vmu.Lock()
	defer m.vmu.Unlock()
	n := 0
	for _, v := range m.viewers {
		if v.Mode == ModeOwn && v.counted {
			n++
		}
	}
	return n
}

// relockArmed reports whether the re-lock rule is armed.
func (m *Manager) relockArmed() bool {
	m.rl.mu.Lock()
	defer m.rl.mu.Unlock()
	return m.rl.armed
}

// armRelock starts the rule after a Portty unlock at time at. With no
// own-account viewer left the grace starts at once.
func (m *Manager) armRelock(at time.Time) {
	rl := &m.rl
	rl.mu.Lock()
	rl.armed, rl.since, rl.seen, rl.unknown, rl.failing, rl.posts = true, at, false, false, false, 0
	rl.graceAt = time.Time{}
	if m.ownViewers() == 0 {
		rl.graceAt = at.Add(relockGrace)
	}
	rl.mu.Unlock()
	_, _ = m.persist(func(rs *runtimeState) { rs.RelockArmed, rs.RelockSince = true, at })
}

// relockViewer is told when an own-account viewer's join worked or it left. A
// return cancels the grace; the last one leaving starts it. A join that failed
// was never counted, so its leave neither restarts nor ends the grace (U-load).
func (m *Manager) relockViewer(joined bool, v *viewer) {
	if v.Mode != ModeOwn {
		return
	}
	m.vmu.Lock()
	was := v.counted
	v.counted = joined
	m.vmu.Unlock()
	if !joined && !was {
		return
	}
	now := m.d.Now()
	rl := &m.rl
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.who = v.Identity
	if !rl.armed {
		return
	}
	switch own := m.ownViewers(); {
	case own > 0:
		rl.graceAt, rl.posts = time.Time{}, 0
	case rl.graceAt.IsZero():
		rl.graceAt = now.Add(relockGrace)
	}
}

// disarmRelock ends the rule and clears the persisted flag.
func (m *Manager) disarmRelock() {
	rl := &m.rl
	rl.mu.Lock()
	was := rl.armed
	rl.armed, rl.graceAt, rl.seen, rl.unknown, rl.failing, rl.posts = false, time.Time{}, false, false, false, 0
	rl.mu.Unlock()
	if was {
		_, _ = m.persist(func(rs *runtimeState) { rs.RelockArmed, rs.RelockSince = false, time.Time{} })
	}
}

// resumeRelock runs at NewManager: a persisted arm with the Mac still unlocked
// starts the grace again, with the unlock time unknown (only input after the
// start can disarm it). A Mac found locked, or that cannot be read as unlocked
// for certain, is handled the safe way: locked clears the flag, unreadable keeps
// the rule.
func (m *Manager) resumeRelock() {
	rl := &m.rl
	if !rl.resume {
		return
	}
	if locked, err := m.d.Unlock.SessionLocked(); err == nil && locked {
		_, _ = m.persist(func(rs *runtimeState) { rs.RelockArmed, rs.RelockSince = false, time.Time{} })
		return
	}
	now := m.d.Now()
	rl.mu.Lock()
	rl.armed, rl.since, rl.graceAt = true, now, now.Add(relockGrace)
	rl.mu.Unlock()
	slog.Info("desktop: re-lock was armed before this start; it is armed again")
}

// relockEndIfLocked ends the rule on a Mac that reads locked; a failed read is
// not locked. The relocked line is written here, after the read, and only when
// the rule's own chord was posted: never on the strength of a post alone.
func (m *Manager) relockEndIfLocked() bool {
	if locked, err := m.d.Unlock.SessionLocked(); err != nil || !locked {
		return false
	}
	rl := &m.rl
	rl.mu.Lock()
	posted, who := rl.posts > 0, rl.who
	rl.mu.Unlock()
	if posted {
		m.audit(AuditRelocked, who, "")
		m.countInput("relocked")
		slog.Info("desktop: the Mac was locked again after the last own-account viewer left")
	}
	m.disarmRelock()
	return true
}

// relockPoll is one pass of the rule, run each second by the lease loop after
// the tap's own poll: end it when the Mac reads locked (confirming a posted
// chord), sample the physical-input source, and decide once the grace is over
// or a posted chord's confirm window has passed.
func (m *Manager) relockPoll(now time.Time) {
	rl := &m.rl
	rl.pollMu.Lock()
	defer rl.pollMu.Unlock()

	rl.mu.Lock()
	armed, since := rl.armed, rl.since
	rl.mu.Unlock()
	if !armed {
		return
	}
	// Locked by the rule's chord, or by any other means (a screen saver, the lid,
	// someone at the keyboard): the rule has nothing left to do.
	if m.relockEndIfLocked() {
		return
	}
	r := m.d.Physical()

	rl.mu.Lock()
	switch {
	case !r.Healthy:
		rl.unknown = true
	case r.Seen && now.Add(-r.Idle).After(since):
		rl.seen = true
	}
	due := rl.armed && !rl.graceAt.IsZero() && !now.Before(rl.graceAt) && m.ownViewers() == 0
	rl.mu.Unlock()
	if due {
		m.relockDecide(now)
	}
}

// relockDecide ends the grace, and runs again each time a posted chord was not
// confirmed: a Mac found locked, or one a healthy tap showed someone using, is
// left alone and the rule ends; anything else gets the Lock Screen chord and
// stays armed until a later pass reads it locked. The caller holds pollMu.
func (m *Manager) relockDecide(now time.Time) {
	rl := &m.rl
	rl.mu.Lock()
	if !rl.armed {
		rl.mu.Unlock()
		return
	}
	seen, unknown, who, posts := rl.seen, rl.unknown, rl.who, rl.posts
	rl.mu.Unlock()

	if m.relockEndIfLocked() {
		return
	}
	if seen && !unknown {
		m.disarmRelock()
		return
	}
	if unknown {
		slog.Warn("desktop: re-locking the Mac without a physical-input signal")
	}
	m.releaseHeldAll("relock")
	if err := m.postLockChord(); err != nil {
		m.countInput("relock_failed")
		rl.mu.Lock()
		first := !rl.failing
		rl.failing = true
		rl.mu.Unlock()
		if first {
			slog.Warn("desktop: the re-lock keys could not be posted; trying again each second")
		}
		return // stays armed; the next poll tries again
	}
	m.in.mu.Lock()
	m.in.lockUntil = now.Add(optimisticLock)
	m.in.mu.Unlock()
	if posts == relockTries {
		slog.Warn("desktop: the Mac did not lock after the re-lock keys; still trying", "posts", posts, "every", relockBackoff)
		m.countInput("relock_failed")
		m.audit(AuditRelockFailed, who, "")
	}
	wait := relockConfirm
	if posts >= relockTries {
		wait = relockBackoff
	}
	rl.mu.Lock()
	if !rl.graceAt.IsZero() { // zero: an own-account viewer came back meanwhile
		rl.posts++
		rl.graceAt = now.Add(wait)
	}
	rl.failing = false
	rl.mu.Unlock()
}

// postLockChord posts Lock Screen (Ctrl+Cmd+Q) through the input bridge, which
// tags every event as Portty's. Every key is released even when one post fails;
// the first error is returned.
func (m *Manager) postLockChord() error {
	ctrl, _ := screencap.KeyCode("ControlLeft")
	cmd, _ := screencap.KeyCode("MetaLeft")
	q, _ := screencap.KeyCode("KeyQ")
	both := uint64(screencap.FlagControl | screencap.FlagCommand)
	var first error
	for _, k := range []struct {
		vk    uint16
		down  bool
		flags uint64
	}{
		{ctrl, true, screencap.FlagControl},
		{cmd, true, both},
		{q, true, both},
		{q, false, both},
		{cmd, false, screencap.FlagControl},
		{ctrl, false, 0},
	} {
		if err := m.d.Input.PostKey(k.vk, k.down, k.flags, false); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// audit writes one own-account line to the local audit log, when the host wired
// one. It never blocks and never fails (audit.go).
func (m *Manager) audit(event string, id Identity, reason string) {
	if m.d.Audit != nil {
		m.d.Audit(event, id, reason)
	}
}
