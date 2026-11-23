---
phase: 15-v1-2-hardening
plan: 02
subsystem: pty-kill-and-test-flakes
tags: [go, pty, killsession, pgid, flaky-tests, websocket]
requires:
  - phase: 09-handoff
    provides: killAdopted exited guard and the signalGroup seam
provides:
  - "killSession never signals or re-waits a reaped created session (Session.reaped)"
  - "TestKillClean_NoOrphans and TestShare_PasteBacklogKeepsViewer wait on events, not sleeps"
affects: [15-09 single clean full Go run]
status: complete
actuals:
  tokens: 5200
  tasks: 2
  commits: 2
plan_head_before: 8da2ffc2cfc8f092b3939d9987906e205a66caa2
plan_head_after: adcd338c0c1902fc3ccee454ffd876b9d9b84ecf
commits: 2
tech-stack:
  added: []
  patterns: ["atomic.Bool reaped flag mirrors the adopted path's exited channel", "count answered pings (OnPingReceived) instead of holding wall-clock time"]
key-files:
  created: []
  modified:
    - internal/pty/session.go
    - internal/pty/cleanup.go
    - internal/pty/reap_other.go
    - internal/pty/adopt_test.go
    - internal/relay/manager.go
    - internal/daemon/engine.go
    - internal/pty/native_test.go
    - internal/webserver/input_queue_test.go
    - internal/webserver/capability_helpers_test.go
key-decisions:
  - "reaped is set explicitly after cmd.Wait returns; never inferred from ExitCode() != -1 (a signal-killed child reports -1)"
  - "Items 2 and 3 of todo 2026-11-20 are documented at the code (HubManager.Shutdown, closeSessionFiles), not fixed: no caller today"
  - "Per-ping ceiling in the paste test is 5 s; product heartbeat defaults and the dead-viewer test's 100 ms are untouched"
requirements-completed: []
---

# Phase 15 Plan 02: Stale pgid guard and deterministic Go flakes Summary

Dismissing a created session whose shell was already reaped now sends no signal and does not call `cmd.Wait` twice, and the two load-sensitive Go tests wait on events (a marker line, pings answered) instead of fixed sleeps.

## Accomplishments

- `Session.reaped atomic.Bool`: set right after `cmd.Wait()` in `ReapNaturalExit` (created branch) and in `killSession`'s wait goroutine. `killSession` returns early (closes the PTY, cancels the context) when it is set, after the existing nil-cmd check.
- The created path's SIGHUP and SIGKILL go through the `signalGroup` seam; SIGKILL is skipped if the wait goroutine already reaped. `killAdopted` is untouched.
- The `killSession` doc comment records both residual windows (dismiss between child exit and reap signals a zombie group that still holds its pid; and the few-instruction gap between `Wait` returning and `reaped.Store(true)`).
- Doc-only comments: `HubManager.Shutdown` (exit watchers stay `blocked in cmd.Wait`) and `closeSessionFiles` (would miss go-pty's `original master` on a created session). Diff for those two files is comment-only.
- `TestKillClean_NoOrphans`: the script prints `portty-bg-started`; the test waits for it, then after Kill waits (5 s) until signal 0 to the negated pgid fails. No `time.Sleep` left.
- `TestShare_PasteBacklogKeepsViewer`: per-ping ceiling 5 s, pings counted via the new `dialWebServerWSPing` helper's `OnPingReceived` hook (returns true so the pong is sent), and the hold is "8 more pings after the whole paste queued". `joinPasteViewer` gained an `onPing` parameter; the dead-viewer test passes nil and keeps 100 ms.

## Task Commits

1. Task 1 (tracer): `7b6698d5` feat(15-02) reaped guard, signalGroup seam, tests, doc comments
2. Task 2: `adcd338c` test(15-02) event-driven NoOrphans and paste-backlog tests, ping-hook dial helper

## Verification

Logs: `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.zOaSaiv6IS` (task 1; RED run in `tmp.mPSICZ5Kur`) and `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.mphl1c8irP` (task 2). All with `GOTOOLCHAIN=go1.26.11`.

- go vet on pty, relay, daemon, webserver: clean.
- Task 1: new and existing kill tests pass under `-race` (TestKillSession_*, TestKillClean*, TestKillAdopted, TestAdopted_ReapNaturalExit).
- Task 2: `TestKillClean_NoOrphans` 10 of 10 under `-race`; `TestShare_PasteBacklogKeepsViewer` 30 of 30 under `-race` while `internal/relay`'s `-race` run (200 s) ran beside it; `-race -short` of `./internal/pty/...` and `./internal/webserver/...` pass.
- The full Go and vitest suites run once, in 15-09.

## RED evidence (Task 1)

The plan is `type: execute`, so tests and implementation went into one commit. Tests were still written first and run at HEAD: `TestKillSession_RunningSignalsGroup` failed on its planned assertion (`signals sent = [], want SIGHUP first`: the created path bypassed the seam). `TestKillSession_ReapedSkipsGroupSignal` passed at HEAD, vacuously, because the seam recorded nothing from the created path at all; it becomes a real guard only now that the path signals through `signalGroup` (a regression that removes the `reaped` check would make it fail). Recorded here as a known weakness of that test's RED, not hidden.

## Deviations from Plan

None - plan executed exactly as written. One small fix inside Task 2: the `WaitFor` failure message in the paste test originally interpolated `pings.Load()-start` at call time (always 0), so it was reworded to a fixed message.

## Known Stubs

None.

## Threat Flags

None.

## Self-Check: PASSED

Both commits are ancestors of HEAD; all nine files_modified paths changed in the plan range; Task 2's commit lists only the three test files; the manager.go and engine.go diffs are comment-only.
