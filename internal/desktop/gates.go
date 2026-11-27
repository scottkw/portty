package desktop

// gates.go: the one gate every press passes. Releases of what a viewer holds
// never come here: a dropped release strands a key or button on an unattended
// Mac, so only presses, moves, wheel and text are gated (a rock-solid rule).

// positional reports whether the kind carries a surface epoch.
func (msg inputMsg) positional() bool {
	return msg.T == inMove || msg.T == inRMove || msg.T == inBtn || msg.T == inWheel
}

// inputLocked reports whether input must be refused because the screen is
// locked.
func (m *Manager) inputLocked() bool {
	return m.Locked()
}

// pressAllowed decides whether v's press, move, wheel or text msg may post and
// names the drop reason when not: no_right, locked, stale_epoch.
func (m *Manager) pressAllowed(v *viewer, msg inputMsg) (ok bool, reason string) {
	switch {
	case v.Rights&RightsControl == 0:
		return false, "no_right"
	case m.inputLocked():
		return false, "locked"
	case msg.positional() && msg.E != m.currentEpoch():
		return false, "stale_epoch"
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
