package desktop

// Test-only seams of surface.go, in their own file so the surface tests do not
// share export_test.go with the other plans.

// SetViewerRightsForTest changes a connected viewer's rights (a Phase 22 guest
// has fewer than the own-account viewer every loopback viewer gets).
func (m *Manager) SetViewerRightsForTest(id ViewerID, r Rights) {
	m.vmu.Lock()
	defer m.vmu.Unlock()
	if v := m.viewers[id]; v != nil {
		v.Rights = r
	}
}
