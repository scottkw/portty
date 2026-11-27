package desktop

// unlock.go: remote unlock (UNLK-01). An own-account viewer sends a macOS login
// password as one binary message on the input data channel; the daemon wakes
// the screen, types the password through the native bridge, checks the lock,
// retries once and reports. The password lives in one daemon-owned byte buffer
// for the attempt and is zeroed in a defer. It is never a Go string, never
// logged, never put in an error, a message or a counter name. Guests never get
// the capability.
//
// The lock gate (gates.go inputLocked) is not loosened: the typer calls the
// bridge directly and never goes through handleKey or pressAllowed.
//
// The throttle counts submits, not macOS checks: a failed submit counts once
// although its retry means the Mac checked the password twice, so the pause
// arrives after at most 6 checks. The count and the pause live in daemon memory
// and a daemon restart resets them (macOS's own policy is the backstop).

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/scottkw/portty/internal/screencap"
)

// unlockMsgType is byte 0 of an unlock request on the input channel.
const unlockMsgType byte = 0x01

const (
	maxUnlockBytes = 1024 // the most password bytes (after the type byte) one request carries

	unlockSettle = time.Second            // wake key to first typed key: the password field appears
	unlockKeyGap = 60 * time.Millisecond  // between typed strokes
	unlockCheck  = 3 * time.Second        // how long after Return the lock is polled
	unlockPoll   = 250 * time.Millisecond // between lock reads while polling

	unlockSpacing     = 5 * time.Second  // no submit this soon after an attempt that typed
	unlockPause       = 15 * time.Minute // how long unlock stays paused after the failures
	unlockMaxFailures = 3                // consecutive failed submits that start the pause
)

// unlockWakeKey is the one key posted to reveal the password field; it must
// type no character. kVK_Shift is the starting value: the VM row U-01 (23-10)
// decides the final one.
const unlockWakeKey uint16 = 0x38

// redacted is what every formatter of a secret prints.
const redacted = "[redacted]"

// secretBytes holds the password. Every way to print it, marshal it or hand it
// to slog yields the marker, so a stray %v or log field cannot leak it; the
// only things it offers are a byte view for the layout map and a zero.
type secretBytes struct{ b []byte }

func (s secretBytes) String() string   { return redacted }
func (s secretBytes) GoString() string { return redacted }

// Format covers every fmt verb, including %x and %q, which would otherwise
// reach the bytes.
func (s secretBytes) Format(f fmt.State, _ rune) { _, _ = f.Write([]byte(redacted)) }

func (s secretBytes) MarshalJSON() ([]byte, error) { return []byte(`"` + redacted + `"`), nil }
func (s secretBytes) LogValue() slog.Value         { return slog.StringValue(redacted) }

// bytes is the view the layout map reads.
func (s secretBytes) bytes() []byte { return s.b }

// zero overwrites the buffer.
func (s secretBytes) zero() { clear(s.b) }

// KeyStroke is one typed character: the virtual key and its modifier flags.
type KeyStroke = screencap.KeyStroke

// ErrUnmappable means a character of the password has no key on the Mac's
// current layout; nothing is typed.
var ErrUnmappable = screencap.ErrUnmappable

// UnlockBridge is the narrow native seam the unlock sequence types through.
// The default is screencap.NewUnlockBridge(); every posting method of it
// refuses under tests, so tests inject a fake.
type UnlockBridge interface {
	// MapRunes maps UTF-8 bytes to one stroke per character through the current
	// layout, or returns ErrUnmappable before any stroke.
	MapRunes(b []byte) ([]KeyStroke, error)
	CapsLockOn() (bool, error)
	// Wake declares user activity and posts one key that types no character.
	Wake(vk uint16) error
	Type(KeyStroke) error
	Return() error
	// SessionLocked is the uncached lock read.
	SessionLocked() (bool, error)
}

// States and reasons of the {type: unlock} message (the wire contract in
// 23-01-PLAN).
const (
	typeUnlock = "unlock"

	unlockIdle        = "idle"
	unlockUnlocking   = "unlocking"
	unlockRetrying    = "retrying"
	unlockUnlocked    = "unlocked"
	unlockFailed      = "failed"
	unlockRateLimited = "rate_limited"
	unlockPaused      = "paused"
	unlockRefused     = "refused"     // declined before typing: Reason says why
	unlockUnavailable = "unavailable" // cannot type at all: Reason says why
	unlockErrorSt     = "error"

	reasonCapsLock     = "caps_lock"
	reasonUnmappable   = "unmappable"
	reasonUnlockAccess = "accessibility"
)

// msgUnlock tells an unlock-capable viewer where unlock is. Failures is the
// consecutive failed-submit count; WaitMS the remaining wait of rate_limited and
// paused; ByPortty is true on unlocked when the daemon typed the password (as
// opposed to someone at the Mac).
type msgUnlock struct {
	Type     string `json:"type"`
	State    string `json:"state"`
	Failures int    `json:"failures"`
	WaitMS   int    `json:"wait_ms,omitempty"`
	Reason   string `json:"reason,omitempty"`
	ByPortty bool   `json:"by_portty,omitempty"`
}

// UnlockStats is the unlock state as the host sees it: counts and states only.
type UnlockStats struct {
	State       string `json:"state"` // idle, unlocking, retrying or paused
	Failures    int    `json:"failures"`
	RelockArmed bool   `json:"relock_armed"`
}

// unlockState is the Manager's unlock bookkeeping behind one leaf mutex (never
// held across a native call, a sleep or a send).
type unlockState struct {
	mu         sync.Mutex
	running    bool   // an attempt is in flight
	state      string // unlocking or retrying while running
	lastEnd    time.Time
	failures   int
	pauseUntil time.Time
}

// unlockResult is how an attempt ended.
type unlockResult int

const (
	ulNone     unlockResult = iota // refused or ended before typing; the submitter was told
	ulUnlocked                     // the Mac reports unlocked
	ulFailed                       // still locked after the retry
	ulError                        // the bridge failed, or the attempt panicked
)

// ulRun is what one attempt holds that must be wiped and released when it ends.
type ulRun struct {
	strokes []KeyStroke
	slot    bool // this attempt holds the single attempt flag
	typed   bool // typing began
}

// expireLocked ends a pause whose time has passed and starts the count over;
// u.mu is held.
func (u *unlockState) expireLocked(now time.Time) {
	if !u.pauseUntil.IsZero() && !now.Before(u.pauseUntil) {
		u.pauseUntil, u.failures = time.Time{}, 0
	}
}

// viewLocked is the state a viewer is told now and its wait; u.mu is held.
func (u *unlockState) viewLocked(now time.Time) (state string, waitMS int) {
	u.expireLocked(now)
	switch {
	case u.running:
		return u.state, 0
	case !u.pauseUntil.IsZero():
		return unlockPaused, int(u.pauseUntil.Sub(now).Milliseconds())
	}
	return unlockIdle, 0
}

// UnlockStats reports the unlock state.
func (m *Manager) UnlockStats() UnlockStats {
	u := &m.ul
	u.mu.Lock()
	st, _ := u.viewLocked(m.d.Now())
	failures := u.failures
	u.mu.Unlock()
	return UnlockStats{State: st, Failures: failures, RelockArmed: m.relockArmed()}
}

// msgLocked builds a message of this state with the current failure count; u.mu
// is held.
func (u *unlockState) msgLocked(state, reason string, waitMS int) msgUnlock {
	return msgUnlock{Type: typeUnlock, State: state, Failures: u.failures, Reason: reason, WaitMS: waitMS}
}

// handleUnlockBytes takes one unlock request off the input channel. data[0] is
// the type byte; the rest is the password. The bytes are copied into one
// daemon-owned buffer and the inbound slice is zeroed at once; nothing builds a
// string from them. It runs inline on the SCTP loop, so it only validates and
// hands off.
func (m *Manager) handleUnlockBytes(v *viewer, data []byte) {
	defer clear(data)
	switch {
	case !v.unlockCap:
		// A guest, whatever it is granted, is not heard.
		m.countInput("unlock_forbidden")
		return
	case len(data) < 2 || len(data) > maxUnlockBytes+1 || data[0] != unlockMsgType:
		m.countInput("malformed")
		return
	}
	secret := secretBytes{b: bytes.Clone(data[1:])}
	go m.runUnlock(v, secret)
}

// runUnlock is one attempt's goroutine. The deferred wipes run on a panic too:
// the buffer, then the strokes derived from it.
func (m *Manager) runUnlock(v *viewer, secret secretBytes) {
	defer recoverLog()
	defer secret.zero()
	var r ulRun
	defer func() { clear(r.strokes) }()
	res := ulError // what a panic leaves behind
	defer func() { m.unlockEnd(v, &r, res) }()
	res = m.attemptUnlock(v, secret, &r)
}

// attemptUnlock is the sequence: a fresh lock read, the throttle and the single
// attempt slot, the refusals that cost nothing (Accessibility, Caps Lock, a
// character with no key), release of held input, the wake key, a settle, the
// strokes, Return, the lock poll, and one retype if the Mac is still locked.
func (m *Manager) attemptUnlock(v *viewer, secret secretBytes, r *ulRun) unlockResult {
	b := m.d.Unlock
	locked, err := b.SessionLocked()
	if err != nil {
		return m.unlockTell(v, unlockErrorSt, "", 0, "unlock_error")
	}
	if !locked {
		return m.unlockTell(v, unlockIdle, "", 0, "unlock_not_locked")
	}

	u := &m.ul
	now := m.d.Now()
	u.mu.Lock()
	u.expireLocked(now)
	switch {
	case !u.pauseUntil.IsZero():
		wait := int(u.pauseUntil.Sub(now).Milliseconds())
		msg := u.msgLocked(unlockPaused, "", wait)
		u.mu.Unlock()
		m.countInput("unlock_paused")
		m.sendUnlock(v, msg)
		return ulNone
	case u.running || now.Before(u.lastEnd.Add(unlockSpacing)):
		wait := unlockSpacing // the earliest a new try can start is spacing after this one ends
		if !u.running {
			wait = u.lastEnd.Add(unlockSpacing).Sub(now)
		}
		msg := u.msgLocked(unlockRateLimited, "", int(wait.Milliseconds()))
		u.mu.Unlock()
		m.countInput("unlock_rate_limited")
		m.sendUnlock(v, msg)
		return ulNone
	}
	u.running, u.state = true, unlockUnlocking
	r.slot = true
	u.mu.Unlock()

	if in, _ := m.ControlState(); in == ControlNeedsAccessibility {
		return m.unlockTell(v, unlockUnavailable, reasonUnlockAccess, 0, "unlock_refused")
	}
	caps, err := b.CapsLockOn()
	switch {
	case err != nil:
		return m.unlockTell(v, unlockErrorSt, "", 0, "unlock_error")
	case caps:
		return m.unlockTell(v, unlockRefused, reasonCapsLock, 0, "unlock_refused")
	}
	strokes, err := b.MapRunes(secret.bytes())
	r.strokes = strokes
	switch {
	case err != nil && isUnmappable(err):
		return m.unlockTell(v, unlockRefused, reasonUnmappable, 0, "unlock_refused")
	case err != nil || len(strokes) == 0:
		return m.unlockTell(v, unlockErrorSt, "", 0, "unlock_error")
	}

	m.unlockProgress(unlockUnlocking)
	m.releaseHeldAll("unlock")
	if err := b.Wake(unlockWakeKey); err != nil {
		return ulError
	}
	m.unlockSleep(unlockSettle)
	if !m.typeStrokes(r, strokes) {
		return ulError
	}
	if !m.lockedAfterReturn() {
		return ulUnlocked
	}
	// Still locked: one retype from the same strokes, never a second.
	m.unlockProgress(unlockRetrying)
	if !m.typeStrokes(r, strokes) {
		return ulError
	}
	if !m.lockedAfterReturn() {
		return ulUnlocked
	}
	return ulFailed
}

func isUnmappable(err error) bool { return errors.Is(err, ErrUnmappable) }

// unlockProgress records the running state and tells every unlock-capable viewer.
func (m *Manager) unlockProgress(state string) {
	u := &m.ul
	u.mu.Lock()
	u.state = state
	msg := u.msgLocked(state, "", 0)
	u.mu.Unlock()
	m.sendUnlock(nil, msg)
}

// typeStrokes types each stroke with unlockKeyGap between them and presses
// Return. false means the bridge failed or the Manager closed; typing stopped
// and Return was not pressed, so the Mac never saw a whole password.
func (m *Manager) typeStrokes(r *ulRun, strokes []KeyStroke) bool {
	b := m.d.Unlock
	r.typed = true
	for i, s := range strokes {
		if m.closed.Load() {
			return false
		}
		if i > 0 {
			m.unlockSleep(unlockKeyGap)
		}
		if err := b.Type(s); err != nil {
			return false
		}
	}
	return b.Return() == nil
}

// lockedAfterReturn polls a fresh lock read for unlockCheck and reports whether
// the Mac is still locked. A failed read counts as still locked (fail closed).
func (m *Manager) lockedAfterReturn() bool {
	start := m.d.Now()
	for {
		if l, err := m.d.Unlock.SessionLocked(); err == nil && !l {
			return false
		}
		if m.d.Now().Sub(start) >= unlockCheck {
			return true
		}
		m.unlockSleep(unlockPoll)
	}
}

func (m *Manager) unlockSleep(d time.Duration) {
	if m.d.UnlockSleep != nil {
		m.d.UnlockSleep(d)
		return
	}
	time.Sleep(d)
}

// unlockTell answers a refusal to the submitter alone and counts it. Nothing
// was typed, so it does not count as a failed submit and does not start the
// spacing window.
func (m *Manager) unlockTell(v *viewer, state, reason string, waitMS int, counter string) unlockResult {
	m.countInput(counter)
	u := &m.ul
	u.mu.Lock()
	msg := u.msgLocked(state, reason, waitMS)
	u.mu.Unlock()
	m.sendUnlock(v, msg)
	return ulNone
}

// unlockEnd releases the attempt slot and reports how the attempt ended to
// every unlock-capable viewer: the spacing window starts when typing began, a
// failed submit counts once, the third in a row starts the pause, a success
// resets the count.
func (m *Manager) unlockEnd(v *viewer, r *ulRun, res unlockResult) {
	u := &m.ul
	now := m.d.Now()
	u.mu.Lock()
	if r.slot {
		u.running, u.state = false, ""
		if r.typed {
			u.lastEnd = now
		}
	}
	var msg msgUnlock
	switch res {
	case ulUnlocked:
		u.failures = 0
		msg = u.msgLocked(unlockUnlocked, "", 0)
		msg.ByPortty = true
	case ulFailed:
		u.failures++
		if u.failures >= unlockMaxFailures {
			u.pauseUntil = now.Add(unlockPause)
			msg = u.msgLocked(unlockPaused, "", int(unlockPause.Milliseconds()))
		} else {
			msg = u.msgLocked(unlockFailed, "", 0)
		}
	case ulError:
		msg = u.msgLocked(unlockErrorSt, "", 0)
	default:
		u.mu.Unlock()
		return
	}
	u.mu.Unlock()
	// One audit line for every attempt that typed; nothing about the password goes
	// in it. A success arms the re-lock rule before the viewer is told.
	switch {
	case res == ulUnlocked:
		m.audit(AuditUnlocked, v.Identity, "")
		m.armRelock(now)
	case res == ulFailed:
		m.audit(AuditUnlockFailed, v.Identity, ReasonStillLocked)
	case res == ulError && r.typed:
		m.audit(AuditUnlockFailed, v.Identity, ReasonUnlockError)
	}
	m.countInput("unlock_" + msg.State)
	slog.Info("desktop: unlock finished", "viewer", shortID(v.ID), "outcome", msg.State)
	m.sendUnlock(nil, msg)
}

// sendUnlock writes msg to one viewer (only) or to every unlock-capable viewer
// whose answer is out. shortcut: writes are sequential like broadcast's, so a
// stalled viewer delays the rest by up to writeTimeout; per-viewer queues if the
// VM rows ever show it.
func (m *Manager) sendUnlock(only *viewer, msg msgUnlock) {
	var conns []*websocket.Conn
	if only != nil {
		if only.ws != nil {
			conns = append(conns, only.ws)
		}
	} else {
		m.vmu.Lock()
		for _, vw := range m.viewers {
			if vw.unlockCap && vw.leaseReady.Load() {
				conns = append(conns, vw.ws)
			}
		}
		m.vmu.Unlock()
	}
	for _, c := range conns {
		_ = send(context.Background(), c, msg)
	}
}

// sendUnlockJoin tells a viewer that just joined where unlock stands, once,
// while the Mac is locked.
func (m *Manager) sendUnlockJoin(ctx context.Context, v *viewer) {
	if !v.unlockCap || !m.Locked() {
		return
	}
	u := &m.ul
	u.mu.Lock()
	state, wait := u.viewLocked(m.d.Now())
	msg := u.msgLocked(state, "", wait)
	u.mu.Unlock()
	_ = send(ctx, v.ws, msg)
}
