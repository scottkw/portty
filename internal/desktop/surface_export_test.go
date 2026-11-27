package desktop

// Test-only seams of surface.go, in their own file so the surface tests do not
// share export_test.go with the other plans.

// RestartSourceForTest is the stall-recovery path: stop the source and start a
// new one for the viewers that remain.
func (m *Manager) RestartSourceForTest() error { return m.restartSource() }

// SetViewerRightsForTest changes a connected viewer's rights (a Phase 22 guest
// has fewer than the own-account viewer every loopback viewer gets).
func (m *Manager) SetViewerRightsForTest(id ViewerID, r Rights) {
	m.vmu.Lock()
	defer m.vmu.Unlock()
	if v := m.viewers[id]; v != nil {
		v.Rights = r
		v.setRightsBits(r)
	}
}
