package desktop

// Test-only seams of clipboard.go, in their own file so the clipboard tests do
// not share export_test.go with the other plans.

// ChunkTextForTest cuts text the way a Copy from Mac does.
func ChunkTextForTest(s string) []string { return chunkText(s, clipChunkBytes, clipFrameMax) }

// ClipPutsForTest is how many viewers have a paste in progress.
func (m *Manager) ClipPutsForTest() int {
	m.clip.mu.Lock()
	defer m.clip.mu.Unlock()
	return len(m.clip.puts)
}
