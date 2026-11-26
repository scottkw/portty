---
phase: quick-261126-wsg
plan: 01
subsystem: webserver
status: complete
tags: [test-flake, websocket, same-user-gate]
requires: []
provides:
  - "TestSameUserGate_WebSocket waits for the accepted viewer to join and leave the hub"
affects: [internal/webserver]
tech-stack:
  added: []
  patterns: ["poll the observable count to the expected value, fail loudly on timeout"]
key-files:
  created:
    - .planning/quick/261126-wsg-flaky-same-user-gate-websocket-test/261126-wsg-PLAN.md
  modified:
    - internal/webserver/whois_gate_test.go
decisions:
  - "Test fix only. handleWSSRelay writes the 101 in websocket.Accept and only then calls hub.Join, so a client dial can return before the subscriber is counted. That order is expected, not a product race."
---

# Quick 261126-wsg Summary

**Cause.** `handleWSSRelay` answers 101 in `websocket.Accept` and then calls
`hub.Join`. The "same" subtest's dial could return, the subtest end and close the
socket, and the old wait loop read `SubscriberCount() == 0` before the server
goroutine joined. The late join then raised the count to 1 for a short time
(until the server noticed the closed socket), and the refusal and order-row
checks (`want 0`) failed.

**Fix.** The "same" subtest now waits for the count to reach 1, calls
`conn.CloseNow()`, and waits for 0, with a 5s timeout that fails the test.
`base` is the constant 0.

**Evidence.**
- With a temporary 5ms delay in `beforeSubscribeHookForTest`, the old test failed
  4 of 20 runs (`subscribers after order rows = 1, want 0`); the new test passed
  20/20 at delays of 2, 5, 10 and 20ms.
- `GOTOOLCHAIN=go1.26.11 go test -race -count=200 -run TestSameUserGate_WebSocket ./internal/webserver/`: ok (8.2s).
- `go test -race ./internal/webserver/...`: ok.

TESTING.md is unchanged: no test file was added, renamed or removed.

Commit: d6120016
