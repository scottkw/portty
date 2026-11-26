---
phase: quick-261126-wsg
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/webserver/whois_gate_test.go
autonomous: true
requirements: []
---

# Quick 261126-wsg: fix flaky TestSameUserGate_WebSocket

CI (run on af2bfaba, passed on re-run) failed at whois_gate_test.go:175/207,
"subscribers after refusals = 1, want 0".

## Task 1: synchronize the "same" subtest on the hub subscriber count

- In the "same" subtest, after the 101, poll `hub.SubscriberCount()` until it is 1,
  close the socket, and poll until it is 0. Fail on a 5s timeout instead of
  continuing silently.
- Replace the post-subtest wait loop and the measured `base` with a constant 0.
- Fix the test, not the product: the server writes the 101 before `hub.Join`,
  which is expected (Accept must run first).

## Verify

- `GOTOOLCHAIN=go1.26.11 go test -race -count=200 -run TestSameUserGate_WebSocket ./internal/webserver/`
- Temporary 5ms `beforeSubscribeHookForTest` delay reproduces the failure on the
  old test and passes on the new one (not committed).
