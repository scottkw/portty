---
phase: 01-plain-macos-terminal
fixed_at: 2026-11-15T01:15:07Z
review_path: .planning/phases/01-plain-macos-terminal/01-REVIEW.md
iteration: 1
findings_in_scope: 9
fixed: 9
skipped: 0
status: all_fixed
---

# Phase 1: Code Review Fix Report

**Fixed at:** 2026-11-15T01:15:07Z
**Source review:** .planning/phases/01-plain-macos-terminal/01-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 9 (CR-01, WR-01..WR-08; fix_scope `critical_warning`, so IN-01..IN-06 were not attempted)
- Fixed: 9
- Skipped: 0

**Where verification ran:** in the main checkout `the repo root`, on branch `gsd/phase-01-plain-macos-terminal`. No worktree was used. The orchestrator's root-pin guard requires the git toplevel to be the main checkout and says to commit on this branch, and a hand-rolled worktree would also have no `node_modules` for the frontend gates. All results below can be reproduced from the main checkout.

**Gates after the last fix (HEAD 4a41c8ee):**
- `go build ./... && go vet ./...` passed.
- `go test -race -short -count=1 ./...` passed in every package.
- `pnpm exec tsc --noEmit` passed.
- `pnpm exec vitest run` passed: 117 files, 1820 tests.
- `bash tests/check-traceability-paths.sh` passed: 120 paths OK.

During one intermediate run, `internal/daemon` failed once in `TestExitEvent_ListSessions_ExitCodePopulatedForStopped`: the spy-backend session read as exited at once. That test is outside the WR-06 change. It passed `-count=5` on its own and in every later full run, so it is pre-existing load-flakiness.

## Fixed Issues

### CR-01: Tailscale health can stick at "not connected" after recovery

**Files modified:** `app.go`, `app_test.go`, `frontend/src/App.tsx`, `frontend/src/components/__tests__/App.tailscale.test.tsx`
**Commit:** b436b10e
**Status:** fixed: requires human verification (logic/state fix)
**Applied fix:**
- `GetTailscaleStatus` now goes through `checkTailscaleHealth`. `lastHealth` sits behind a new `healthMu`, and the emit happens under that lock. The poller therefore records every value the frontend has seen and emits the recovery.
- The focus/visibility recheck in `App.tsx` no longer applies the RPC reply. It only triggers the check, and the `tailscale:health` event updates the UI. This means a late reply cannot overwrite a newer event.
- init/retryInit still apply their reply, because they need a value before the first event.
- Regression test: `TestGetTailscaleStatus_SharesPollerState` runs connected, then a not-connected recheck, then a connected poll, and expects 3 emits. The source test now asserts the recheck does not call `setTailscaleHealth`.
- Remaining edge case: at launch only, a poller event that lands while init's `Promise.all` is still pending could be overwritten by init's reply. The window is tens of milliseconds and happens once.

### WR-01: Exit watcher never reports sessions removed without a "stopped" sighting

**Files modified:** `app.go`, `app_exit_test.go`
**Commit:** acbc53b0
**Status:** fixed: requires human verification (logic fix)
**Applied fix:** `checkExits` now tracks every id it has listed (`seenSessions`). An id that was listed before and is now missing emits `session:exit` once. This covers `agenthub kill` and a daemon restart. New test: `TestCheckExits_ReportsRemovedWithoutStoppedSighting`, where a running session is killed between ticks and exactly one exit is emitted.

### WR-02: `session:exit` fires once and leaves removal to the frontend

**Files modified:** `app.go`, `app_exit_test.go`, `frontend/src/App.tsx` (comment only)
**Commit:** 688b3438
**Status:** fixed: requires human verification (logic fix)
**Applied fix:**
- The watcher now removes stopped sessions itself with `KillSession`. A failed kill leaves the session listed, and the next 500 ms tick retries.
- The exit is emitted once, when the id leaves the list, so the event only drives the UI. `exitEmitted` was deleted because `seenSessions` covers it.
- `TestCheckExits_PrunesRemovedSessions` became `TestCheckExits_RemovesStoppedSession`. It asserts the daemon no longer lists the session after its exit is reported and that exactly one event fired.
- Side effect: on an exit event, the frontend's `handleCloseTab` → `KillSession` now gets "not found" and logs a `console.warn`. That error was already tolerated. The exit-handler comment now says so.

### WR-03: A shell that exits before its tab is added leaves a permanent dead tab

**Files modified:** `frontend/src/App.tsx`, `frontend/src/components/__tests__/App.shellExit.test.tsx`
**Commit:** 89675fd8
**Status:** fixed: requires human verification (race fix)
**Applied fix:** The `session:exit` handler records ids in `exitedIdsRef`. `createTab` returns early after `CreateSession` resolves if the id has already exited, so it adds neither a tab nor a Hub entry. A source test asserts that the guard comes after `CreateSession` and before `setTabs`/`setHubSessions`. The set grows by one short id per exit for the life of the GUI, which is negligible.

### WR-04: Hub modal stays open on an exited or removed session

**Files modified:** `frontend/src/components/Hub/HubPanel.tsx`, `frontend/src/components/Hub/HubPanel.test.tsx`, `frontend/src/App.tsx`, `frontend/src/components/__tests__/App.shellExit.test.tsx`
**Commit:** 061e13da
**Status:** fixed
**Applied fix:**
- HubPanel now has an effect that closes the modal when its session is in neither the local `sessions` nor the remote list.
- App's `session:exit` handler removes the session from `hubSessions` straight away.
- Behavioral test (WR-04 in `HubPanel.test.tsx`): open the modal, re-render with no sessions, and the overlay is gone. I confirmed this test fails without the effect.
- This is consistent with Amendment 2: the modal closes and no exit UI is added.

### WR-05: `RetryDaemon` starts a new health poller per call and writes `a.client` concurrently

**Files modified:** `app.go`, `app_test.go`
**Commit:** f1a062ba
**Status:** fixed: requires human verification (startup ordering change)
**Applied fix:** I took a lighter route than the review's `atomic.Pointer`, which would have meant rewriting about 95 `a.client` call sites.
- `startup` now sets `a.client` once, before `EnsureDaemon`, and never reassigns it. The client only dials on use.
- The exit watcher, tray, health and update pollers now start once in `startup` whether or not the daemon is up. While it is down, their calls fail and they retry.
- `RetryDaemon` now only runs `EnsureDaemon` and updates `daemonErr`. It neither reassigns the client nor starts pollers. `exitWatcherOnce` was deleted because nothing calls the watcher twice any more.
- New test: `TestRetryDaemonSuccess_KeepsClientAndPollers` runs a real daemon API at `DefaultSocketPath` under a short temp HOME. It starts the watcher, calls a successful `RetryDaemon`, and asserts the client is the same pointer. It runs under `-race`.
- Behavior change to check: after a failed startup, `a.client` is non-nil, so Wails calls return socket dial errors instead of "daemon not connected". The frontend stops at `GetDaemonError()` in that state, and the tray still shows the error icon through `Health()` failing.

### WR-06: `GetSessionStatus` reports "running" for unknown sessions and is unused

**Files modified:** `internal/daemon/api.go`, `internal/daemon/client.go`, `internal/daemon/engine.go`, `internal/daemon/types.go`, `internal/daemon/api_test.go`, `internal/daemon/client_test.go`, `internal/daemon/engine_test.go`, `internal/daemon/engine_natural_exit_test.go`, `internal/daemon/files_removed_test.go`
**Commit:** f6e5ea18
**Status:** fixed
**Applied fix:**
- Deleted the dead code rather than patching it, as the task allowed: the `GET /sessions/{id}/status` route and handler, `DaemonClient.GetSessionStatus`, `SessionEngine.GetSessionStatus` and `StatusResponse`.
- Nothing in production called them. Status comes from `ListSessions`.
- Removed the 3 tests that only exercised them. The status assertions in `TestDerivedStatus_ExitedErrored` and `TestListSessions_ShellStatusRunning` now go through `ListSessions` only.
- New negative test: `TestSessionStatusRouteRemoved` (404/405). The shared `assertRouteGone` message now says "route removed".

### WR-07: `agenthub new` resolves relative paths in the daemon's cwd and drops extra args

**Files modified:** `cmd_cli.go`, `cmd_cli_test.go`
**Commit:** 52ebd041
**Status:** fixed
**Applied fix:**
- A positional path now goes through `filepath.Abs` in the CLI's own cwd.
- More than one positional argument fails with `usage: agenthub new [<path>] (got N arguments)`. This covers the removed `new claude ~/proj` and `new shell ~/proj` forms.
- Errors print as `agenthub new: <err>` instead of always saying "daemon unreachable". The "Locked stderr" doc line was not backed by any requirement or planning doc, so I updated it and `TestCmdNew_DaemonError`.
- New tests: `TestCmdNew_RelativeDir` and `TestCmdNew_RejectsExtraArgs`.

### WR-08: Rewritten CI jobs have no `timeout-minutes`

**Files modified:** `.github/workflows/build.yml`, `.github/workflows/release.yml`
**Commit:** 4a41c8ee
**Status:** fixed
**Applied fix:**
- Added `timeout-minutes: 30` to `build`, `validate` and `build-macos`, and checked the YAML parses.
- I left `sign-macos` alone. The review did not name it, and its notarization wait can run long.

## Not in scope (fix_scope = critical_warning)

IN-01..IN-06 remain open in REVIEW.md. IN-02 (`QuitGUIOnly` counts `Status != "stopped"`) matters less now, because the watcher removes stopped sessions itself within about 500 ms.

---

_Fixed: 2026-11-15T01:15:07Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
