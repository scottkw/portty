package desktop

import "context"

// export_lease_test.go: test-only access to the lease loop's passes, so a test
// with a fake clock does not wait for the one-second tick.

// PushLeaseForTest runs one lease pass: every controlling viewer whose state
// changed is told.
func (m *Manager) PushLeaseForTest(ctx context.Context) { m.pushLease(ctx) }

// PollTapForTest runs one poll of the tap's life cycle and Secure Event Input.
func (m *Manager) PollTapForTest() { m.pollTap() }
