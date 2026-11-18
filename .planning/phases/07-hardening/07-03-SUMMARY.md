---
phase: 07-hardening
plan: 03
subsystem: daemon-lifecycle, test-hygiene, docs
status: complete
tags: [respawn, launch-grace, test-isolation, testmain, readme]
requires: []
provides:
  - "bounded launch grace in respawnDaemonIfDown (HARD-04)"
  - "health error in every respawn log line"
  - "internal/daemon test binary isolated from the real config dir (HARD-06)"
  - "README Known Issues entry for multi-line prompt resize artifacts (HARD-08)"
affects: [app.go, internal/daemon tests, README.md]
tech-stack:
  added: []
  patterns: ["package TestMain moving HOME to a short /tmp dir", "per-test t.Setenv HOME before NewSessionEngine"]
key-files:
  created:
    - internal/daemon/testmain_test.go
  modified:
    - app.go
    - app_respawn_test.go
    - internal/daemon/api_test.go
    - README.md
decisions:
  - "HARD-04 gate is bounded: skip respawn only before the first healthy tick and within 10 s of a successful startup; failed startup gets no grace (keeps Phase 5 R6)"
  - "HARD-06: testDaemon isolates HOME instead of resetting settings fields one by one; the reset block is removed"
metrics:
  duration: "~25 min"
  completed: 2026-11-18
actuals:
  tokens: 2800
  tasks: 3
  commits: 5
plan_head_before: f54a223a
plan_head_after: 923b089b
---

# Phase 7 Plan 03: Launch respawn gate, daemon test isolation, README Known Issues Summary

A healthy launch can no longer spend a respawn attempt (10 s launch grace before the first healthy tick, with the health error now logged). The internal/daemon test binary runs entirely under a temp HOME. README documents the stray-prompt-lines-on-narrowing issue.

## Tasks

| Task | Name | Commits |
|------|------|---------|
| 1 | Launch gate in respawnDaemonIfDown, health error in the log (HARD-04) | 84ac366c (test), f3e3a48b (fix) |
| 2 | Daemon test binary isolation, testDaemon clean settings (HARD-06) | e49764e8 (test), 49e513c7 (fix) |
| 3 | README Known Issues entry (HARD-08) | 923b089b |

## Tests added / edited (for 07-05 to copy into TESTING.md)

- `app_respawn_test.go` (root package, edited):
  - `TestRespawnDaemon_LaunchGraceSkips`: calls at t0 and t0+5s on a dead socket run no ensure, attempts stay 0, no daemon:error
  - `TestRespawnDaemon_LaunchGraceEnds`: a call at t0+10s runs ensure once
  - `TestRespawnDaemon_HealthySeenEndsGrace`: healthy at t0, daemon stopped, ensure runs at t0+5s (R6)
  - `TestRespawnDaemon_FailedStartupNoGrace`: zero launchedAt + daemonErr, ensure runs at once
  - `TestStartDaemon_SetsLaunchedAtOnSuccess` (success/failure subtests)
- `internal/daemon/testmain_test.go` (new):
  - `TestMain`: HOME and XDG_CONFIG_HOME point at `os.MkdirTemp("/tmp", "pdh-")` for the whole binary (covers the external `daemon_test` files too)
  - `TestTestMain_IsolatesConfigDir`: daemonConfigDir() and DefaultSocketPath() lie under the temp HOME, not the real one
  - `TestTestDaemon_IgnoresSettingsFile`: a settings.json with dark/Menlo/16 in the TestMain HOME does not reach testDaemon (appearance "system", font {"" 13})
- `internal/daemon/api_test.go` (edited): `testDaemon` sets HOME/XDG_CONFIG_HOME to t.TempDir() before `NewSessionEngine()`. `TestAPIAppearance_GetPatch` and `TestAPITerminalFont_GetPatch` now pass without skips.

## Verification

- `go build ./... && go vet . ./internal/daemon/`: clean
- `go test -race -short -count=1 . ./internal/daemon/`: `ok github.com/scottkw/portty 32.031s`, `ok github.com/scottkw/portty/internal/daemon 18.337s`; `grep -c 'already running at'` on the output prints 0
- Task 1 greps: `respawnLaunchGrace` 3, `seenHealthy` 4, `daemon unreachable (%v), respawn attempt` 1
- Task 2: `t.Setenv("HOME"` in api_test.go 1; the four named tests pass under -race
- Task 3: Known Issues at line 28, License at line 35; `bash tests/check-brand.sh` prints `OK: brand gate clean`
- `-skip` check: `git grep` outside .planning finds no `-skip` for the two settings tests

## HARD-04 root cause

Not found. Code reading confirms the plan's analysis: `NewApp` and `startup` resolve the same `DefaultSocketPath()`, `startDaemon` runs synchronously and closes `startupDone` before `startTrayPoller` starts, and nothing uses `a.client` synchronously before the poller's first `Health()`. `EnsureDaemon` checks health on its own fresh client, so the App's long-lived client has made no request before that first tick. The respawn line now prints the health error (`portty: daemon unreachable (<err>), respawn attempt N: <result>`), and the one-time launch line prints it too (`portty: daemon not answering right after launch (<err>); waiting up to 10s before any respawn`). The 07-05 VM launch capture will show the actual error. The gate makes the blip harmless whatever the cause.

## Deviations from Plan

None in code. Process notes:
- The worktree was created from main (c2f4255d), not the phase branch. Fast-forwarded with `git merge --ff-only gsd/phase-07-hardening` to f54a223a before starting. The coordinator later sent the same instruction.
- Task 1 split the planned "LaunchGrace" behaviour into two tests (`LaunchGraceSkips`, `LaunchGraceEnds`), which gives five new tests rather than four. Both match the plan's `TestRespawnDaemon_LaunchGrace` prefix.

## Known Stubs

None.

## Self-Check: PASSED

- FOUND: internal/daemon/testmain_test.go, app.go, app_respawn_test.go, internal/daemon/api_test.go, README.md
- FOUND commits: 84ac366c, f3e3a48b, e49764e8, 49e513c7, 923b089b (all ancestors of HEAD)
