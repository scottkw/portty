package main

import (
	"strings"
	"testing"
)

// newTestApp returns an App with lastTrayQuartile pre-stored to the given
// initial value. Centralizes the atomic.Int32 initialization so individual
// tests don't have to know the field's storage type.
func newTestApp(initialQuartile int32, refreshTrayStateFunc func()) *App {
	a := &App{
		trayInit:             true,
		refreshTrayStateFunc: refreshTrayStateFunc,
	}
	a.lastTrayQuartile.Store(initialQuartile)
	return a
}

// TestApp_SetTrayProgress_Idempotent verifies that calling SetTrayProgress
// with the same quartile twice invokes the tray update only once.
//
// Phase 98 PRG-03 / Pitfall #3 (transition guard) — idempotency check.
// Uses the refreshTrayStateFunc injection (Phase 98 / PROJECT.md
// "Function injection" pattern) to avoid cgo/D-Bus/Win32 calls in tests.
func TestApp_SetTrayProgress_Idempotent(t *testing.T) {
	calls := 0
	a := newTestApp(-1, func() { calls++ })

	// First call — quartile differs from -1, should invoke the refresh.
	if err := a.SetTrayProgress(2); err != nil {
		t.Fatalf("first SetTrayProgress(2) returned unexpected error: %v", err)
	}
	if got := a.lastTrayQuartile.Load(); got != 2 {
		t.Errorf("after first call: want lastTrayQuartile=2, got %d", got)
	}
	if calls != 1 {
		t.Errorf("after first call: want refreshTrayState called 1 time, got %d", calls)
	}

	// Second call — same quartile; must be a no-op (idempotency check).
	if err := a.SetTrayProgress(2); err != nil {
		t.Fatalf("second SetTrayProgress(2) returned unexpected error: %v", err)
	}
	if got := a.lastTrayQuartile.Load(); got != 2 {
		t.Errorf("after second call: want lastTrayQuartile=2, got %d", got)
	}
	if calls != 1 {
		t.Errorf("after second call: refreshTrayState should NOT have been called again; got calls=%d", calls)
	}
}

// TestApp_SetTrayProgress_BoundsCheck verifies that quartile values outside
// [0,4] return a non-nil error and do not mutate state.
//
// Phase 98 PRG-03 / T-98-02 (Tampering — bounds check mitigates frontend bug
// passing arbitrary quartile). Mirrors the error-wrapping convention of
// SaveTerminalSession (Phase 97 SER-01).
func TestApp_SetTrayProgress_BoundsCheck(t *testing.T) {
	a := newTestApp(-1, func() {})

	// Negative out-of-range.
	err := a.SetTrayProgress(-1)
	if err == nil {
		t.Fatal("SetTrayProgress(-1): expected non-nil error, got nil")
	}
	if !strings.Contains(err.Error(), "out of range") {
		t.Errorf("SetTrayProgress(-1): error should contain 'out of range', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "-1") {
		t.Errorf("SetTrayProgress(-1): error should contain '-1', got %q", err.Error())
	}
	if got := a.lastTrayQuartile.Load(); got != -1 {
		t.Errorf("SetTrayProgress(-1): state mutated — want lastTrayQuartile=-1, got %d", got)
	}

	// Positive out-of-range.
	err = a.SetTrayProgress(5)
	if err == nil {
		t.Fatal("SetTrayProgress(5): expected non-nil error, got nil")
	}
	if !strings.Contains(err.Error(), "5") {
		t.Errorf("SetTrayProgress(5): error should contain '5', got %q", err.Error())
	}
	if got := a.lastTrayQuartile.Load(); got != -1 {
		t.Errorf("SetTrayProgress(5): state mutated — want lastTrayQuartile=-1, got %d", got)
	}
}

// TestApp_SetTrayProgress_ErrorPrecedence verifies that trayIconBytesForState
// returns the error pair when connected=false, regardless of lastTrayQuartile,
// and the matching 1x and @2x pair otherwise.
//
// Phase 98 PRG-03 / T-98-08 (error-state precedence, Pitfall #8) — progress
// glyph must not mask daemon-disconnect state. Tests the helper directly to
// avoid any platform-tray API invocation.
func TestApp_SetTrayProgress_ErrorPrecedence(t *testing.T) {
	// Apply quartile 3 (75% progress glyph).
	a := newTestApp(3, func() {})

	check := func(desc string, connected bool, want1, want2 []byte) {
		t.Helper()
		x1, x2 := a.trayIconBytesForState(connected)
		if &x1[0] != &want1[0] {
			t.Errorf("trayIconBytesForState(%s): wrong 1x slice", desc)
		}
		if &x2[0] != &want2[0] {
			t.Errorf("trayIconBytesForState(%s): wrong @2x slice", desc)
		}
	}

	// When disconnected, must return the error pair regardless of quartile.
	check("connected=false, quartile=3", false, trayIconErrorBytes, trayIconErrorBytes2x)
	// When connected with quartile 3, must return the progress-75 pair.
	check("connected=true, quartile=3", true, trayIconProgress75Bytes, trayIconProgress75Bytes2x)
	// Reset to quartile 0 (no active progress) — connected path returns the base pair.
	a.lastTrayQuartile.Store(0)
	check("connected=true, quartile=0", true, trayIconBytes, trayIconBytes2x)
}
