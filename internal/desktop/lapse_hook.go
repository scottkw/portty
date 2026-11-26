package desktop

// EnterAwaitingForTest is the transition every detector (watchdog, probe) goes
// through; who names the detector in the log. It lives outside export_test.go so
// the daemon package's tests can drive a real Manager into a lapse (19-14); it
// is reachable only from code in this process.
func (m *Manager) EnterAwaitingForTest(who string) { m.enterAwaiting(who) }
