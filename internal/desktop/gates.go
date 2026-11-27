package desktop

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
// names the drop reason when not: no_right, locked, stale_epoch, no_accessibility, rate. A token
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
	case !m.takeToken(v):
		return false, "rate"
	}
	return true, ""
}

// allowed is pressAllowed that also counts the drop.
func (m *Manager) allowed(v *viewer, msg inputMsg) bool {
	ok, why := m.pressAllowed(v, msg)
	if !ok {
		m.countFor(v, why)
	}
	return ok
}
