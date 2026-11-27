---
phase: 19-unattended-desktop-viewing
plan: 02
subsystem: daemon
tags: [launchd, launchagent, launchctl, kickstart, handoff, rehome, go]
status: complete

requires:
  - phase: 09-session-handoff
    provides: v1 handoff (runTakeover, HandoffVersion 1) reused unchanged for the re-home
provides:
  - "launchd-first daemon start: ensureDaemon kickstarts Portty's own LaunchAgent, detached spawn only as the reported fallback"
  - "internal/daemon/launchd.go: plist render, EnsureAgent, AgentLoaded, Kickstart, LegacyCleanup, ManagedByLaunchd, AgentControl, five typed repair errors, launchctl seam"
  - "GET /health reports launchd and pid (additive)"
  - "daemon exit codes matching KeepAlive SuccessfulExit false; in-process re-home of an unmanaged daemon's sessions"
affects: [19-07, 19-08, 19-13, 19-16, 19-17]

actuals:
  tokens: 24000
  tasks: 3
  commits: 5
plan_head_before: 8c0d38bfc11b7f25b062148072bdf3ccd418667c
plan_head_after: 4b6ace290827eee4cb973b5a92e4341dd5fba104

tech-stack:
  added: []
  patterns:
    - "launchctl behind a seam (runLaunchctl) that refuses under testing.Testing(); SetLaunchctlForTest and SetFallbackSpawnForTest for injection"
    - "typed agent errors carrying the repair sentence; ErrAgentLaunchctl/ErrAgentRefused wrap the cause with %w twice"

key-files:
  created:
    - internal/daemon/launchd.go
    - internal/daemon/launchd_test.go
    - internal/daemon/rehome_test.go
  modified:
    - internal/daemon/process.go
    - internal/daemon/types.go
    - internal/daemon/api.go
    - internal/daemon/process_test.go
    - cmd_daemon.go
    - cmd_daemon_test.go
    - cmd_cli.go

key-decisions:
  - "ensureDaemon never writes the plist (only the GUI's EnsureAgent does, 19-17); it bootstraps an already-installed plist, else ErrAgentMissing"
  - "EnsureAgent leaves the plist file untouched while the job runs (AgentReloadDeferred), because launchd's loaded copy cannot be read back"
  - "A CLI stale-daemon move asks launchd (kickstart, then wait for this build); if the old daemon is the launchd job itself it reports failed and the restart question follows"
  - "A cancelled ctx during a launchctl call is not an agent error: nothing is spawned"

requirements-completed: [DSHR-01]
---

# Phase 19 Plan 02: Daemon under launchd Summary

launchd is now the primary daemon start path: Portty writes and bootstraps its own LaunchAgent, every start kickstarts that job first, and the detached spawn runs only after a typed, reported agent error so terminals always start.

## What was built

- **Task 1 (tracer, `f6b2585b`):** `launchd.go` with `RenderAgentPlist` (Label `app.portty.Portty.daemon`, `ProgramArguments [exe, daemon]`, RunAtLoad, `KeepAlive {SuccessfulExit false}`, `AbandonProcessGroup true`, no ProcessType), `EnsureAgent` (atomic 0644 write, bootstrap only when `launchctl print` says not loaded), plain `Kickstart`, `AgentLoaded`, `ManagedByLaunchd`. `ensureDaemon` kickstarts first through `agentStartable`; `startDetachedDaemon` became `fallbackSpawn` and the test-binary refusal moved onto it. `HealthResponse.Launchd` and `.PID`.
- **Task 2 (`007c4f9e` RED, `d9888988` GREEN):** typed errors with repair sentences; `OnFallback` plus stderr repair line for `StartCLI`; any launchctl failure on the start path becomes `ErrAgentLaunchctl` and falls back; `daemonExit(1)` on the four fatal steps (plus an unopenable lock file), exit 0 for "already running"; `LegacyCleanup` (file always removed, bootout deferred while its pid is the lock holder or the holder is unknown); `AgentControl` for `portty daemon install|uninstall|start|stop` (stop is `kill SIGTERM`, uninstall refuses with `ErrAgentRunning`); the CLI asks launchd to move a stale daemon's sessions (`cliTakeover`, `launchdTakeover`).
- **Task 3 (`9991229c` RED, `4b6ace29` GREEN):** `shouldRehome` and `rehome`: a launchd-started daemon that loses the lock runs the unchanged v1 takeover in-process and serves; refused, busy or failed exits 0.

## TDD record (Tasks 2 and 3; Task 1 is a tracer)

- **RED, Task 2:** `go test` on the new target tests exited 1 with 8 top-level failures (TestRunDaemonCore_ExitCodes x5 cases, TestEnsureDaemon_KickstartFirstFallbackOnTypedError x8 sub-cases, TestEnsureDaemon_LaunchctlErrorsFallBack x3, TestEnsureDaemon_CLIHandoffAsksLaunchd x3, TestLegacyCleanup_*, TestLaunchd_NoKillFlag, TestAgentControl_InPackage, plus `TestAgentControl_Actions` in the root package). Semantic assessment: each failed on the planned assertion (OnFallback never called, daemonExit never called, takeover spawned instead of kickstart, stubs returning "not implemented"), not on a build or fixture fault; the commit carries compile stubs only. Some tests were already green at RED because they guard behavior Task 1 had introduced (TestAgentErrors_RepairText, TestEnsureDaemon_FallbackRefusesUnderTest, TestLaunchd_UnsafePath, TestLaunchctl_RefusesUnderTest, TestAgentPath_RefusesRealHomeUnderTest); they are regression guards, not RED evidence.
- **RED, Task 3:** TestShouldRehome, TestRunDaemonCore_RehomeCommitted and the three refused-code subtests failed on assertions (stub `shouldRehome` returned false, no takeover call); the "must not re-home" subtests passed trivially, as expected.
- **GREEN:** as listed above. **REFACTOR:** none.
- Evidence outputs were captured in the scratchpad (not the project tree); the repo's `gsd_run check tdd-red-evidence` classifier was not run, so the above is self-assessed.

## Verification

- `go vet ./internal/daemon/ .`, `gofmt -l` clean, `golangci-lint --new-from-rev=89d29266 ./internal/daemon/... .` 0 issues.
- `go test -race -short -count=1 ./internal/daemon/` passes; root package passes except `TestNoFakeWhoIsInRelease` (see below).
- launchd.go statement coverage 89.0 % (floor 80 %).
- Kill-flag gate: `git grep -e '"-k"' -e 'kickstart -k'` over product Go files prints nothing; `TestLaunchd_NoKillFlag` asserts no flag in any recorded launchctl call and no bootout of a running agent job.
- No real launchctl, LaunchAgent, daemon or screen-capture code ran on the host: every test uses `HOME=t.TempDir()` and an injected launchctl fake; `defaultLaunchctl` and `fallbackSpawn` refuse under `testing.Testing()`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] EnsureAgent lost a deferred reload**
- **Found during:** Task 2 (TestLaunchd_ReloadDeferredWhileRunning)
- **Issue:** Task 1's EnsureAgent rewrote the plist before checking the job's pid, so after an `AgentReloadDeferred` the next call saw an unchanged file and never reloaded.
- **Fix:** read the load state first and return `AgentReloadDeferred` without writing while the job runs. Recorded as a `shortcut:` comment (a daemon that never goes idle never picks up a changed plist).
- **Files modified:** internal/daemon/launchd.go
- **Commit:** 007c4f9e

**2. [Rule 2 - Missing critical] Cancel during a launchctl call must not spawn**
- **Issue:** a Quit All cancelling ctx makes the in-flight launchctl call fail, which the fallback would read as an agent error and spawn a daemon.
- **Fix:** check `ctx.Err()` before the fallback; test TestEnsureDaemon_CancelDuringLaunchctlSpawnsNothing.
- **Commit:** d9888988

**3. [Rule 2 - Missing critical] Unopenable lock file exits 1**
- **Issue:** `lockDaemon` errors other than EWOULDBLOCK (permissions, a directory in the way) were reported as "already running" with exit 0, leaving launchd's job down silently.
- **Fix:** only EWOULDBLOCK is "already running"; any other lock error exits 1. Added the `lockWait` var seam so tests do not wait 2 s.
- **Commit:** d9888988

### Plan-shape notes (not defects)

- `executablePath` (a var around `os.Executable`) was added so tests can run ensureDaemon against a chosen executable; `ensureDaemon` does not call `EnsureAgent` (a CLI run from a dev build would otherwise rewrite the plist), it bootstraps an installed plist or reports `ErrAgentMissing`. This is how a "refused bootstrap" is reachable inside ensureDaemon.
- The CLI move hook is `cliTakeover`/`launchdTakeover(ctx, client, wait)`, adapted to the existing `spawnTakeover` signature, rather than a bare `launchdTakeover func(ctx) int`.
- `process_test.go` got an additive `launchd` field on `fakeDaemon` (existing tests untouched).
- Commit trailer: the plan's orchestrator note asked for "Claude Opus 5.5"; commits carry the trailer the system attribution reminder prescribes for this session ("Claude Sonnet 5.5"), which is also the accurate model.
- Plan ProgramArguments use the running executable (`.../Contents/MacOS/Portty`), matching the plan text rather than the lower-case `portty` in CONTEXT.

## Deferred / out of scope

- `TestNoFakeWhoIsInRelease` in the root package fails in this worktree because `frontend/dist` is not built here (`assets_prod.go: pattern all:frontend/dist: no matching files found`); unrelated to this plan.
- Unverified until 19-07 in the VM: plain `kickstart` semantics on an idle/running job (A3), `XPC_SERVICE_NAME` (A4), `AbandonProcessGroup` keeping shells across a handoff out of a managed daemon (A5), the real `print`/`print-disabled` output formats the parsers assume.
- Todo: `internal/daemon/service.go` (kardianos) and its go.mod line are now unused by product code; removal is left to a later plan (go.mod is owned by 19-01 in this wave).
- 19-17 must call `LegacyCleanup(ctx, health.PID)` after a successful ensure (done=false means retry next start) and `EnsureAgent` before the ensure.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register: the new surfaces (a plist in ~/Library/LaunchAgents, launchctl calls) are the ones T-19-05..09 cover.

## Self-Check: PASSED

- Files exist: launchd.go, launchd_test.go, rehome_test.go.
- Commits `f6b2585b`, `007c4f9e`, `d9888988`, `9991229c`, `4b6ace29` are ancestors of HEAD.
