package desktop

// export_unlock_test.go: test-only access to the unlock password holder, kept
// apart from the other export files.

// SecretForTest wraps b in the type the daemon holds a password in, so the
// external tests can print and marshal it every way and see only the marker.
func SecretForTest(b []byte) any { return secretBytes{b: b} }

// SecretPtrForTest is SecretForTest as a pointer.
func SecretPtrForTest(b []byte) any { return &secretBytes{b: b} }
