package main

import (
	"strings"
	"testing"
)

// Phase 122-03 Task 1 — Wails binding for RegisterRemoteCap.
//
// The DaemonClient.RegisterRemoteCap helper itself is exercised by the
// internal/daemon tests; here we only verify the Wails-binding shell returns
// the daemon-not-connected error when a.client is nil.
//
// These tests are hermetic — no real remote webserver is contacted; the
// nil-client case is the safest deterministic surface to pin.

func TestApp_RegisterRemoteCap_NilClientReturnsError(t *testing.T) {
	app := &App{} // a.client is nil
	err := app.RegisterRemoteCap("sess-1", "https://hub-a.tailnet.ts.net:9443", "cap-token")
	if err == nil {
		t.Fatal("RegisterRemoteCap returned nil error with nil daemon client; want error")
	}
	if !strings.Contains(err.Error(), "daemon not connected") {
		t.Errorf("RegisterRemoteCap error = %q; want substring 'daemon not connected'", err.Error())
	}
}

// Smoke-test: the method exists on *App with the expected signatures. A
// compile-time check is technically enough, but spelling it out as a test
// makes intent explicit and catches signature drift during refactors.
//
// Method expression signature includes the receiver, so we check
// `func(*App, ...) (...)`.
func TestApp_RegisterRemoteCap_SignatureShape(t *testing.T) {
	var _ func(*App, string, string, string) error = (*App).RegisterRemoteCap
}
