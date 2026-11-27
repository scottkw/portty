package desktop

// Test-only seams of window.go, in their own file so the window tests do not
// share export_test.go with the other plans.

import "github.com/scottkw/portty/internal/screencap"

// SetSurfaceForTest records sel and info as the shared surface without
// switching the source or bumping the epoch (no select round trip needed).
func (m *Manager) SetSurfaceForTest(sel screencap.Surface, info screencap.SurfaceInfo) {
	m.setSurface(sel, info)
}

// WindowRaisingForTest reports whether a raise is running.
func (m *Manager) WindowRaisingForTest() bool {
	m.win.mu.Lock()
	defer m.win.mu.Unlock()
	return m.win.raising
}
