package screencap

// RED skeleton for Task 3: every wrapper is a stub the tests fail against.

// PowerAssertion is a held IOPM assertion (stub).
type PowerAssertion struct{ id uint32 }

// SessionLocked reports whether the login session's screen is locked (stub).
func SessionLocked() (bool, error) { return false, nil }

// DisplayAsleep reports whether the main display is asleep (stub).
func DisplayAsleep() (bool, error) { return false, nil }

// OnACPower reports whether the Mac runs on AC power (stub).
func OnACPower() (bool, error) { return false, nil }

// HoldDisplayAwake holds a display-awake assertion (stub).
func HoldDisplayAwake(name string) (PowerAssertion, error) { return PowerAssertion{}, nil }

// HoldSystemAwake holds a system-awake assertion (stub).
func HoldSystemAwake(name string) (PowerAssertion, error) { return PowerAssertion{}, nil }

// Release releases the assertion (stub).
func (p PowerAssertion) Release() error { return nil }

// DeclareRemoteUserActivity declares remote user activity (stub).
func DeclareRemoteUserActivity(name string) error { return nil }

// NudgePointer moves the pointer one point and back (stub).
func NudgePointer() (bool, error) { return false, nil }

// nudgeDelta picks the nudge direction (stub).
func nudgeDelta(x, minX, maxX float64) float64 { return 0 }
