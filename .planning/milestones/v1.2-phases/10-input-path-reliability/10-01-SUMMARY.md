---
phase: 10-input-path-reliability
plan: 01
subsystem: testing
tags: [go, race, flake, daemon, pty]
status: complete

requires: []
provides:
  - "Hermetic TestExitEvent_ListSessions_RunningHasNoExitCode (real cat PTY) and TestExitEvent_ListSessions_ExitCodePopulatedForStopped (spy session stopped by the exit watcher, waited for with testutil.WaitFor)"
affects: [10-07 (SC5 -count=20 proof, TESTING.md rows)]

actuals:
  tokens: 1100     # chars/4 over the realized diff (4476 chars)
  tasks: 2
  commits: 1       # measured: git rev-list --count 1d72a6e5..HEAD at SUMMARY write (the SUMMARY commit follows)
plan_head_before: 1d72a6e5d1cc1ccfffb0ab73d9af5c2a4eebd786
plan_head_after: 8fa0e01a7d3ebc0c46a64511e26d628a1fb05cf0

tech-stack:
  added: []
  patterns:
    - "Wait for the exit watcher (testutil.WaitFor on GetState) instead of forcing SetState"

key-files:
  created: []
  modified:
    - internal/daemon/engine_exit_test.go

key-decisions:
  - "INPT-05 root cause confirmed by repro: the PTY-less spy session is stopped by the exit watcher before the test's first ListSessions; split into a real-PTY running case and a waited-for stopped case"

requirements-completed: [INPT-05]

coverage:
  - id: D1
    description: "Exit-code ListSessions tests no longer race the exit watcher"
    requirement: INPT-05
    verification:
      - kind: unit
        ref: "go test -race -count=200 -run 'TestExitEvent' ./internal/daemon/ (/tmp/p10-01-stress.log)"
        status: pass

duration: 58min
completed: 2026-11-21
---

# Phase 10 Plan 01: Hermetic exit-code ListSessions test Summary

**The INPT-05 flake was caught failing 3 of 20 times in a full-package `-race -count=20` run, at the "running" assertion. Its cause is that the exit watcher had already stopped the PTY-less spy session. The test is now split into a real-PTY running case and a stopped case that waits for the exit watcher. The split passes 200 of 200 runs under -race, and the whole repo passes one `-race -short` run.**

## Repro (Task 1, base d2998ad2)

`go test -race -count=20 -timeout 60m ./internal/daemon/` → `/tmp/p10-01-repro.log`, `rc=1`, 1414 s.

The target flake reproduced 3 times in 20 passes (log lines 3734, 4529, 4796):

```
--- FAIL: TestExitEvent_ListSessions_ExitCodePopulatedForStopped (0.00s)
    engine_exit_test.go:136: ExitCode for running session: want nil, got 0xc0eb030028
    engine_exit_test.go:139: Duration for running session: want nil, got 0xc0eb030038
```

This confirms Research A1. `spyBackend.Create` returns a session with no PTY, so the hub's first Read fails. `watchSessionExit` then sets `StateStopped`, and the test's first `ListSessions()` sometimes sees a session that is already stopped. The todo suspected the cat process or the wait loop, but those were not involved.

The same run also failed for two unrelated reasons that only show up with `-count>1`. I stopped under RULE 0 (see Deviations):
- `TestWaitForDaemon_SlowStart` failed in 19 of 20 passes, because `slowBuild` never restored `BuildVersion`.
- From about pass 5 on, every test that needs a PTY failed with `open pty: device not configured` (ENXIO). The macOS PTY limit (`kern.tty.ptmx_max` = 511) was used up.

A separate debug session fixed both on gsd/v1.2 (`.planning/debug/pty-fd-leak.md`):
- The PTYs were held by test code only: `testDaemon` and `handoffScene` never killed their sessions. They now do, and a new `TestEngine_PTYFDsReleased` checks it.
- `slowBuild` now restores `BuildVersion`.

## Changes (commit 8fa0e01a)

- `TestExitEvent_ListSessions_RunningHasNoExitCode` (new): uses `newCatEngine()` with a real `/bin/cat` on a real PTY, and `t.Cleanup` kills the session. It asserts State `running` and nil ExitCode and Duration.
- `TestExitEvent_ListSessions_ExitCodePopulatedForStopped` (name kept for SC5): uses the spy backend. `testutil.WaitFor(3s)` waits until the exit watcher sets `pty.StateStopped`, with no manual `SetState`. It then asserts ExitCode is non-nil and equal to 0, and Duration is non-nil.
- It reuses the existing `findSessionByID` helper (engine_test.go). The other two tests in the file are unchanged, because the repro did not implicate them.

## Verification

| Check | Result | Log |
|---|---|---|
| Base repro `-race -count=20 ./internal/daemon/` | FAIL as expected (flake 3/20, plus the two `-count>1` blockers fixed elsewhere) | /tmp/p10-01-repro.log |
| Stress `-race -count=200 -run 'TestExitEvent'` | ok, 0 FAIL, rc=0 | /tmp/p10-01-stress.log |
| `go build ./... && go vet ./...` | OK | /tmp/p10-01-gate-buildvet.log |
| Gate `go test -race -short -count=1 ./...` | 13 packages ok (daemon ok in 80.9 s), 0 FAIL, rc=0 | /tmp/p10-01-gate-go.log |
| check-brand | OK: brand gate clean | /tmp/p10-01-gate-brand.log |
| check-traceability-paths | OK: 164 paths exist | /tmp/p10-01-gate-trace.log |

Skips: the gate log has no `--- SKIP` lines. A run without `-v` doesn't print skips from packages that pass. The only expected skip is `TestHandoffIT`, which is gated by an environment variable.

The literal SC5 proof at the final tree (`go test -race -count=20 ./internal/daemon/`) is 10-07's single run.

**For 10-07's TESTING.md rows (INPT-05):** `internal/daemon/engine_exit_test.go`, which contains `TestExitEvent_ListSessions_RunningHasNoExitCode` and `TestExitEvent_ListSessions_ExitCodePopulatedForStopped`.

## Deviations from Plan

**1. RULE 0 stop, then resumed.** The base repro failed in tests that INPT-05 does not touch (the SlowStart failures and the PTY exhaustion above). I stopped before the stress run, the gate and the commit, and reported. The coordinator had the blockers fixed separately (debug session, gsd/v1.2 at 1d72a6e5). I then committed the unchanged rewrite, rebased it onto 1d72a6e5 with no conflicts, and ran the stress and the gate.

No other deviations. Code changes are exactly as the plan specified.

## Known Stubs

None.

## Self-Check: PASSED

- FOUND: internal/daemon/engine_exit_test.go
- FOUND: 8fa0e01a (ancestor of HEAD)
