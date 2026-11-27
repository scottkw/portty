package desktop

// export_relock_test.go: test-only access to the re-lock rule's poll, so a test
// on a fake clock does not wait for the lease loop's one-second tick.

// RelockPollForTest runs one pass of the re-lock rule at the fake clock's now.
func (m *Manager) RelockPollForTest() { m.relockPoll(m.d.Now()) }
