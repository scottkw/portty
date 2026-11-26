//go:build darwin

package main

import "testing"

// TestHasAppBundleID_FalseWhenUnbundled locks in the bundle-id detection
// primitive that the Go wrapper's log-and-swallow branch keys off. A `go
// test` binary has no .app wrapper, so [[NSBundle mainBundle]
// bundleIdentifier] is nil — this reproduces exactly the `wails dev`
// condition that crashed the GUI process in the Phase 167 M-41 regression.
func TestHasAppBundleID_FalseWhenUnbundled(t *testing.T) {
	if hasAppBundleID() {
		t.Fatal("hasAppBundleID() = true in a go-test binary; expected false (no app-bundle identifier for an unbundled process)")
	}
}

// TestSendNotification_NoBundleReturnsCleanly is a callable/smoke guard on
// the Go wrapper's early-return: calling sendNotification in this unbundled
// test process must return (not panic/abort) instead of reaching the
// crash-prone native UNUserNotificationCenter path.
//
// This proves the wrapper is safe to call in an unbundled process. It is NOT
// a substitute for the live M-41 delivery test — real UNUserNotificationCenter
// delivery + tray-hidden behavior can only be proven on a signed production
// build, not under `go test`, because the main dispatch queue is never pumped
// in a test binary.
func TestSendNotification_NoBundleReturnsCleanly(t *testing.T) {
	sendNotification("portty.test", "Portty", "test")
}

// TestNotificationUsesMainQueue_FalseWithoutApplication pins the V1 fix's
// decision: with no NSApplication running (the launchd daemon, a go-test
// binary) notification work must not hop to the main queue, which nothing
// services. The GUI answers true because the Wails app runs NSApplication; that
// side is shown on a real build, not here. Nothing is posted.
func TestNotificationUsesMainQueue_FalseWithoutApplication(t *testing.T) {
	if notificationUsesMainQueue() {
		t.Fatal("notificationUsesMainQueue() = true with no NSApplication running; the daemon's notifications would be queued on a main queue nobody services (VM row V1)")
	}
}
