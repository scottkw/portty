package desktop

import "github.com/scottkw/portty/internal/screencap"

// gates.go: the one gate every press passes. Releases of what a viewer holds
// never come here: a dropped release strands a key or button on an unattended
// Mac, so only presses, moves, wheel and text are gated (a rock-solid rule).

// positional reports whether the kind carries a surface epoch.
func (msg inputMsg) positional() bool {
	return msg.T == inMove || msg.T == inRMove || msg.T == inBtn || msg.T == inWheel
}

// inputLocked reports whether input must be refused because the screen is
// locked: Locked() (a fresh read, fail-closed) or the optimistic lock a Lock
// Screen chord sets, which covers the lag before the poll sees the real state.
func (m *Manager) inputLocked() bool {
	m.in.mu.Lock()
	until := m.in.lockUntil
	m.in.mu.Unlock()
	return m.d.Now().Before(until) || m.Locked()
}

// takeToken spends one of v's rate tokens: inputRate per second, burst inputBurst.
func (m *Manager) takeToken(v *viewer) bool {
	now := m.d.Now()
	m.in.mu.Lock()
	defer m.in.mu.Unlock()
	vi := v.in
	switch {
	case vi.bucketAt.IsZero():
		vi.tokens = inputBurst
	case now.After(vi.bucketAt):
		vi.tokens = min(inputBurst, vi.tokens+now.Sub(vi.bucketAt).Seconds()*inputRate)
	}
	vi.bucketAt = now
	if vi.tokens < 1 {
		return false
	}
	vi.tokens--
	return true
}

// pressAllowed decides whether v's press, move, wheel or text msg may post and
// names the drop reason when not: no_right, locked, stale_epoch, no_accessibility, the window
// gate's reasons (window.go), rate. A token
// is spent only by a press that passed every other check.
func (m *Manager) pressAllowed(v *viewer, msg inputMsg) (ok bool, reason string) {
	switch {
	case v.Rights&RightsControl == 0:
		return false, "no_right"
	case m.inputLocked():
		return false, "locked"
	case msg.positional() && msg.E != m.currentEpoch():
		return false, "stale_epoch"
	case m.controlNeedsAccessibility():
		return false, "no_accessibility"
	}
	if m.windowShared() {
		if m.windowChord(v, msg) {
			return false, "window_chord"
		}
		if ok, why := m.windowAllows(msg); !ok {
			return false, why
		}
	}
	if !m.takeToken(v) {
		return false, "rate"
	}
	return true, ""
}

// windowChord reports whether msg is a key press that would act outside a
// shared window: Cmd+Tab (app switcher), Cmd+Space (Spotlight) and Ctrl+Cmd+Q
// (Lock Screen). The viewer's held modifiers and the mask the key carries both
// count, so a lost Command key-down cannot let the chord through.
func (m *Manager) windowChord(v *viewer, msg inputMsg) bool {
	if msg.T != inKey {
		return false
	}
	m.in.mu.Lock()
	f := modFlags(v.in.keys)
	m.in.mu.Unlock()
	if msg.M&8 != 0 {
		f |= screencap.FlagCommand
	}
	if msg.M&2 != 0 {
		f |= screencap.FlagControl
	}
	cmd := f&screencap.FlagCommand != 0
	switch msg.Code {
	case "Tab", "Space":
		return cmd
	case "KeyQ":
		return cmd && f&screencap.FlagControl != 0
	}
	return false
}

// allowed is pressAllowed that also counts the drop.
func (m *Manager) allowed(v *viewer, msg inputMsg) bool {
	ok, why := m.pressAllowed(v, msg)
	if !ok {
		m.countFor(v, why)
	}
	return ok
}
