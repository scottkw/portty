---
phase: 19-unattended-desktop-viewing
plan: 17
subsystem: gui-daemon-start
tags: [launchd, launchagent, fallback, repair-state, rehome, login-items, go]
status: complete

requires:
  - phase: 19-02
    provides: EnsureAgent, Kickstart, AgentLoaded, LegacyCleanup, typed agent errors, EnsureOptions.OnFallback, fallback spawn
provides:
  - "GUI start is launchd-first: EnsureAgent for an installed Portty.app before the ensure; Retry Connection and the respawn poller go through the same ensure"
  - "GetDaemonRepair binding (login_items, move_to_applications, open_app, rehome_pending, '') and OpenLoginItems binding"
  - "re-home kickstart after the ensure with bounded tray-tick retry; legacy agent cleanup at every GUI start"
affects: [19-07, 19-13, 19-16]

actuals:
  tokens: 15000
  tasks: 2
  commits: 3
plan_head_before: 542c37c015c5a91ded4db4b3024652f9f9a2d682
plan_head_after: a964d23beb6437b83992f3eb69614e8f64ed90e1

tech-stack:
  added: []
  patterns:
    - "package-var seams for ensureAgentFunc, kickstartFunc, agentLoadedFunc, legacyCleanupFunc, executablePathFunc, openSystemURL; the test fake drives daemon.SetLaunchctlForTest and SetFallbackSpawnForTest"

key-files:
  created:
    - app_launchd_test.go
  modified:
    - app.go
    - frontend/src/wailsjs/go/main/App.d.ts
    - frontend/src/wailsjs/go/main/App.js

key-decisions:
  - "An EnsureAgent error is recorded as a repair kind immediately, not only through OnFallback: an already-running daemon makes the ensure start nothing and fall back to nothing, which would otherwise hide the repair"
  - "A repair kind is cleared when launchd runs the daemon: after a start that went through the kickstart (daemon was down, no OnFallback), or when /health reports launchd true at GUI start or on a re-home tick"
  - "installedExecutable resolves both the executable and HOME through EvalSymlinks (macOS temp dirs and /var are symlinks); an unresolvable path is compared as given"
  - "Each due re-home tick consumes an attempt; a disabled agent stops the retry and records login_items, an unloaded agent stops it silently, an unreadable /health skips without consuming"

requirements-completed: [DSHR-01]
---

# Phase 19 Plan 17: GUI launchd-first start with desktop-only repair Summary

A GUI start from an installed Portty.app now installs or refreshes the LaunchAgent, lets the ensure kickstart it first, and falls back to the detached spawn when the agent is unusable, so terminals always start. The fallback is never silent: it records a repair kind that only the desktop shows.

## What was built

- **Task 1 (tracer, `2d1fe267`):** `startDaemon` calls `prepareAgent` before the ensure: `installedExecutable` (only `/Applications/Portty.app/...` or `~/Applications/Portty.app/...` qualifies; the reason names the failed rule and never a path) gates `ensureAgentFunc`; any other executable logs `daemon: launchd agent not written: <reason>`. The ensure passes `OnFallback`, which stores `repairKindFor(err)` and logs `daemon: started without launchd`. `GetDaemonRepair` and `OpenLoginItems` bindings (the latter opens only the fixed `x-apple.systempreferences:com.apple.LoginItems-Settings.extension` URL with `/usr/bin/open`, refuses under a test binary) plus the two Wails JS/TS bindings. The tracer test `TestStartDaemon_DisabledAgentFallsBackAndMarksRepair` drives the real `daemon.EnsureDaemon` against a fake launchctl and a fallback-spawn recorder: no daemon error, one fallback, `login_items`, zero kickstarts.
- **Task 2 (`14bb25ab` RED, `a964d23b` GREEN):** `afterEnsure` reads `/health` once: launchd true clears any repair kind; launchd false triggers a re-home kickstart (agent loaded and enabled) and arms the tray-tick retry (gaps 30 s, 1 m, 2 m, 4 m, then every 10 m, 8 attempts per GUI run, stops once health reports launchd true). `LegacyCleanup(ctx, health.PID)` runs after every successful start; a false done or an error is logged and never fails startup. `respawnDaemonIfDown` still only calls the ensure. A disk image or App Translocation launch sets `diskImageLaunch`, reported as `move_to_applications`. `GetDaemonRepair` precedence: disk image, recorded kind, `rehome_pending` (retry armed), empty.

## TDD record (Task 2; Task 1 is a tracer)

- **RED (`14bb25ab`):** the new tests failed on assertions against compile stubs (empty `rehomeIfUnmanaged`, no startDaemon wiring): `TestApp_RehomeRetryBackoff` (0 kicks at start, want 1), `TestApp_RehomeStops` (3 subtests), `TestApp_LegacyCleanupAtStart` (no cleanup pids), `TestDaemonRepair_Kinds` (rehome_pending and disk-image precedence), `TestStartDaemon_DiskImageMoveToApplications` (login_items, want move_to_applications). Semantic check: each fails on the planned assertion, not on a fixture or build fault.
- Guards already green at RED because Task 1 implemented them: `TestInstalledExecutable`, `TestOpenLoginItems_FixedURL`, `TestRespawnDaemonIfDown_KickstartFirstFallback`, `TestRetryDaemon_UsesLaunchdFirst` (regression guards, not RED evidence).
- **GREEN:** `a964d23b`. **REFACTOR:** none. The `gsd_run check tdd-red-evidence` classifier was not run; the above is self-assessed.

## Verification

- `go vet .`, `gofmt -l app.go app_launchd_test.go` clean, `golangci-lint run --new-from-rev=89d29266 .` 0 issues.
- `go test -race -count=1 .` passes except `TestNoFakeWhoIsInRelease` (pre-existing in this worktree: `frontend/dist` is not built; same failure recorded in 19-02). The eight plan-listed tests all exist and pass.
- `pnpm exec tsc --noEmit` was NOT run: `frontend/node_modules` does not exist in this worktree. The two additions to `App.d.ts` are plain `export function` declarations in the existing style.
- No real launchctl, LaunchAgent, daemon or GUI ran on the host: every test sets HOME to a temp dir and goes through the launchctl and fallback-spawn seams, which refuse under `testing.Testing()`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing critical] An unusable agent is recorded even when no fallback fires**
- **Found during:** Task 1 design
- **Issue:** the plan records the repair kind only from `OnFallback`. With a fallback daemon already running (an earlier GUI run), the ensure finds a healthy daemon, starts nothing and never calls `OnFallback`, so a still-disabled agent would show no repair.
- **Fix:** `prepareAgent` also stores `repairKindFor(err)` when `EnsureAgent` returns an error; the re-home path stores `login_items` when it finds the agent disabled. Cleared when launchd runs the daemon (see key-decisions).
- **Files modified:** app.go
- **Commit:** 2d1fe267, a964d23b

### Plan-shape notes (not defects)

- The plan says "clears after a kickstart start (no OnFallback call)". The ensure cannot see whether a kickstart ran, so `ensure()` reads health first and clears the kind only when the daemon was down and no fallback fired; a restart over a live daemon is covered by the `/health` launchd-true clear in `afterEnsure` and the re-home tick.
- `TestStartDaemon_DiskImageMoveToApplications` makes the agent unusable by reporting it disabled rather than "not loaded". The daemon package runs the test binary's own path, and a not-loaded agent with a HOME outside `os.TempDir()` yields `ErrAgentUnsafePath` for an unrelated reason, so the test would pass without the flag. With "disabled" the recorded kind is `login_items`, and the test proves the disk-image flag outranks it.
- `OpenLoginItems`'s open seam is a package var (`openSystemURL`) rather than the `a.openURLFunc` field, because that field defaults to the in-app browser opener, not `/usr/bin/open`.
- Commit trailer: the orchestrator note asked for "Claude Sonnet 5.5"; the shared prompt's Opus line was overridden, as in 19-02.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register: T-19-51 (fallback daemon surfaced, re-home retried) and T-19-52 (only installed bundles write the agent, `TestInstalledExecutable`) are covered by tests.

## Unverified until 19-07 (VM)

- That the Login Items URL opens the right pane on macOS 13+ (row V2(d)).
- Real `launchctl kickstart` re-home of an unmanaged daemon, and that one kickstart at startup plus the tick retries land the sessions under launchd.

## Self-Check: PASSED

- Files exist: app_launchd_test.go, the four modified files.
- Commits `2d1fe267`, `14bb25ab`, `a964d23b` are ancestors of HEAD.
