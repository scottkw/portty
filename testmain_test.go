package main

import (
	"os"
	"testing"
)

// TestMain points HOME at a temporary directory so nothing in this package's
// tests reads or writes the user's real ~/Library/Application Support/portty
// (config, capability key, daemon socket), and clears the session id a
// Portty shell would pass to `portty open`.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "portty-test-home-")
	if err != nil {
		panic(err)
	}
	os.Setenv("HOME", home)
	os.Setenv("XDG_CONFIG_HOME", home)
	os.Unsetenv("PORTTY_SESSION_ID") // tests may run inside a Portty session
	code := m.Run()
	os.RemoveAll(home)
	os.Exit(code)
}
