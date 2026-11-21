---
created: 2026-11-19
title: TestExitEvent_ListSessions_ExitCodePopulatedForStopped flakes under the full -race suite
area: testing
resolves_phase: null
audit_acknowledged:
  milestone: v1.1
  at: 2026-11-19
---

## Problem
`internal/daemon/engine_exit_test.go` `TestExitEvent_ListSessions_ExitCodePopulatedForStopped` failed once in `go test -race -short -count=3 ./internal/daemon/` (2026-11-19, HEAD 187a7ffb). It passed 200 of 200 runs alone. The file hasn't changed since Phase 3 (cef671ea), so the flake predates v1.1. The failure line was not captured.

## Suspected cause
The test uses `newCatEngine()` with a spy backend and forces `pty.StateStopped` via SetState. Under load, the real cat process or the session's wait loop probably races the manual state change (e.g. it overwrites the state or the cached exit code).

## Solution sketch
Reproduce with `-count=20` across the whole package under load, capture the message, then make the test hermetic (no live process) or wait for the wait loop to settle before SetState.
