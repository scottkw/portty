---
phase: 02-tabs-sidebar-navigation
fixed_at: 2026-11-16T01:18:40Z
review_path: .planning/phases/02-tabs-sidebar-navigation/02-REVIEW.md
iteration: 3
findings_in_scope: 1
fixed: 1
skipped: 0
status: all_fixed
---

# Phase 2: Code Review Fix Report

**Fixed at:** 2026-11-16T01:18:40Z (iteration 3); 2026-11-16T01:09:55Z (iteration 2); 2026-11-16T00:50:05Z (iteration 1)
**Source review:** .planning/phases/02-tabs-sidebar-navigation/02-REVIEW.md
**Iteration:** 3

The frontmatter counts are for iteration 3. Over all three iterations, 14 findings were in scope and all 14 are fixed.

## Iteration 3

**Summary:**
- Findings in scope: 1 (WR-08 from `fix_scope: critical_warning`)
- Fixed: 1
- Skipped: 0
- Also done in the same commit, as the orchestrator asked: IN-15's comment fix (comment-only).
- Not in scope, left as-is: IN-01, IN-02, IN-03, IN-04, IN-06 (necessity), IN-08, IN-12 and IN-14.

### Verification (iteration 3)

Gates ran in the **main checkout** on branch `gsd/phase-02-tabs-sidebar-navigation`. No worktree was used, so these results can be reproduced from the tree as it stands.

- `go build ./... && go vet ./...`: pass
- `go test -race -short -count=1 ./...`: pass (every package ok)
- `pnpm exec tsc --noEmit`: pass
- `pnpm exec vitest run`: 91 files, 1160 tests pass
- The new `TestListSessions_DoesNotWaitForStartup` was also run against the pre-fix `app.go`, and it failed there ("ListSessions waited on startupDone").

TESTING.md was not changed. The new test went into the existing `app_test.go` and covers no new requirement ID.

### Fixed Issues (iteration 3)

#### WR-08: `awaitStartup` in `ListSessions` blocks the Quit confirm for up to 90 s (120 s with a wedged daemon) during a slow startup

**Files modified:** `app.go`, `app_test.go`
**Commit:** b9041abc
**Status:** fixed: requires human verification (the Quit path during a slow startup can only be confirmed in the native GUI)
**Applied fix:**
- Removed `awaitStartup()` from `ListSessions` and `GetRelayPort`. It stays only on `GetDaemonError` and `RetryDaemon`.
- Ordering is unchanged: `init` awaits `GetDaemonError` before `GetRelayPort` and `ListSessions` (`App.tsx:344-352`), and `retryInit` awaits `RetryDaemon` before `GetRelayPort` (`App.tsx:1016-1024`). `TestSlowStartup_InitWaitsForStartup` still passes. Its doc comment now says `GetDaemonError` is what waits.
- New `TestListSessions_DoesNotWaitForStartup`: with `startupDone` left open and an in-process daemon from `testApp`, `ListSessions` must return a non-nil slice within 1 s.
- IN-15 (comment only): the `startupBindingWait` comment now says it bounds `GetDaemonError` and `RetryDaemon`, covers the common `EnsureDaemon` worst case rather than every one, and that Retry recovers when startup overruns it.

**Remaining limit:** the quit handler still shows the confirm only after `ListSessions` resolves, so a wedged daemon can still delay Quit by the 30 s client timeout, the same as before `eb740795`. The review's optional fix (show the confirm at once and fill in the sessions later) was not applied.

## Iteration 2

**Summary:**
- Findings in scope: 3 (WR-07 from `fix_scope: critical_warning`, plus IN-11 and IN-13, which the orchestrator added)
- Fixed: 3
- Skipped: 0
- Not in scope, left as-is: IN-01, IN-02, IN-03, IN-04, IN-06 (necessity), IN-08 and IN-12.

### Verification (iteration 2)

Gates ran in the **main checkout** on branch `gsd/phase-02-tabs-sidebar-navigation`. No worktree was used, so these results can be reproduced from the tree as it stands.

- `go build ./... && go vet ./...`: pass
- `go test -race -short -count=1 ./...`: pass (every package ok), run after the WR-07 change. IN-11 touches only the frontend.
- `pnpm exec tsc --noEmit`: pass
- `pnpm exec vitest run`: 91 files, 1160 tests pass
- `bash tests/check-traceability-paths.sh`: OK, 92 paths
- The new IN-11 render tests were also run against the pre-fix source, and both failed there.

TESTING.md was not changed. Every new test went into an existing file, and none covers a new requirement ID.

### Fixed Issues (iteration 2)

#### WR-07: Once domReady's 5 s bound expires, it still blocks the message loop on the daemon, and the frontend `init` races `startup` with no recovery

**Files modified:** `app.go`, `app_test.go`
**Commit:** eb740795
**Status:** fixed: requires human verification (startup ordering and race fix; the real slow-start path needs a native GUI launch to confirm)
**Applied fix:**
- `domReady` calls `GetStartMinimized` only when `startupDone` closed within `startupWait`. On timeout it shows the window without any RPC.
- New `awaitStartup()` waits on `startupDone`, bounded by `startupBindingWait` (90 s, which covers `EnsureDaemon`'s worst case with 30 s client timeouts). `GetDaemonError` returns "daemon startup timed out" when the bound passes. `GetRelayPort` and `ListSessions` wait and then try anyway. `RetryDaemon` waits too, so a Retry clicked after a timeout never runs a second `EnsureDaemon` next to startup's. All of these are bindings and run on their own goroutines, so the message loop never blocks. `init` already calls `GetDaemonError` before anything else, so a slow but successful start now ends with `relayPort > 0` and tabs restored, and a failed one shows the error banner.
- Data race: the daemon client is now created in `NewApp`, before `wails.Run`, so every Wails goroutine has a happens-before edge on `a.client`. `NewApp` is only called from `main.go`, and tests build `&App{}` directly. `daemonErr` is read only after `startupDone`.
- `startup`'s daemon block moved into `startDaemon(socketPath)`, which closes `startupDone` on every return path. It emits `daemon:error` through the existing `emitEvent` helper, which calls `runtime.EventsEmit` the same way in production.
- New seams, following the existing nil-means-default pattern: `ensureDaemonFunc` (default `daemon.EnsureDaemon`) and `showWindowFunc` (default `runtime.WindowShow` plus the Dock icon).
- Tests (all pass with `-race`):
  - `TestDomReady_TimeoutShowsWindowWithoutRPC` replaces `TestDomReady_BoundedWhenStartupHangs`. The client points at a socket that accepts and never answers. The test checks that `domReady` returns within the bound, shows the window, and makes no connection.
  - `TestSlowStartup_InitWaitsForStartup`: startup outlasts `domReady`'s bound. `GetDaemonError` and `GetRelayPort` block until startup finishes, then return `""` and a port > 0.
  - `TestSlowStartup_FailureReachesInit`: a slow startup that fails gives `GetDaemonError` the error.
  - `TestGetDaemonError_BoundedWhenStartupHangs`: past the binding bound, it returns the timeout error.
  - `TestStartDaemon_AlwaysClosesStartupDone`: on the success and failure paths, `startupDone` is closed, and `daemon:error` is emitted only on failure.

**Remaining limit:** if startup takes longer than 90 s, `init` shows the timeout banner. Retry then waits up to another 90 s for startup to finish before it acts.

#### IN-11: The new IN-07 error messages stay up after the problem is gone

**Files modified:** `frontend/src/components/SettingsSheet.tsx`, `frontend/src/components/NewSessionModal.tsx`, `frontend/src/components/__tests__/SettingsSheet.web-link-ux.test.tsx`, `frontend/src/components/__tests__/NewSessionModal.test.tsx`
**Commit:** 9b3b32ae
**Applied fix:**
- A copy failure now goes to its own `urlCopyError` state. The Web server row shows `serverError ?? urlCopyError`. The next copy attempt clears it, so a successful copy brings back "Running on <url>". Start and stop also clear it. A separate state means a successful copy can never hide a real start or stop error.
- `handleBrowse` calls `setError(null)` at the start of each attempt, as `handleConfirm` already does.
- The old source-text assertion is now a render test: a failed copy shows the error, and the next copy shows "Running on https://example.ts.net" again. The picker test now also checks that the next successful Browse clears the error.

#### IN-13: domReady's doc comment now documents `startupWait`, and the "even on panic" comment is misleading

**Files modified:** `app.go`
**Commit:** eb740795 (these lines were rewritten by the WR-07 change, so the fix is in that commit)
**Applied fix:** `startupWait` and the new declarations now sit above `domReady`'s doc comment. `go doc -u . App.domReady` shows the right text again. The "even on panic" defer comment is gone. The new `startDaemon` doc says it closes `startupDone` "on every return path". No `recover` was added, because a panic on Wails' bare startup goroutine kills the process anyway.

## Iteration 1

**Summary:**
- Findings in scope: 10 (WR-01 to WR-06 from `fix_scope: critical_warning`, plus IN-10, IN-05, IN-09 and IN-07, which the orchestrator added)
- Fixed: 10
- Skipped: 0
- Not in scope, left as-is: IN-01, IN-02, IN-06 (necessity) and IN-08. IN-03 and IN-04 were deferred on purpose because fixing them removes data or behaviour.

### Verification (iteration 1)

Gates ran in the **isolated worktree** (`.claude/worktrees/rf-02-*`, branch `gsd-reviewfix/02-*`). Its `frontend/node_modules` was a symlink to the main checkout's `frontend/node_modules`. `pnpm exec` refuses a symlinked hoist directory, so the binaries were called directly from `frontend/node_modules/.bin`. The commits are fast-forwarded onto `gsd/phase-02-tabs-sidebar-navigation`, so these results can be reproduced from the main checkout.

- `go build ./... && go vet ./...`: pass
- `go test -race -short ./...`: pass (exit 0, 11 packages ok)
- `tsc --noEmit`: pass
- `vitest run`: 91 files, 1160 tests pass
- `bash tests/check-traceability-paths.sh`: OK, 92 paths

One `go test -race -short ./internal/daemon/` run failed once during IN-10. The failure output was cut off by `tail`, so the failing test is unknown. The package then passed 13 runs in a row with `-count=1`, and the new test passes on its own. This looks like an existing load-sensitive flake, but that is not confirmed.

TESTING.md was not changed. No test file was added, renamed or removed, because every new test went into an existing file, and none of the new tests covers a new requirement ID.

### Fixed Issues (iteration 1)

#### WR-01: Esc cancels a confirm while its action is in flight

**Files modified:** `frontend/src/components/RegenerateKeyModal.tsx`, `frontend/src/components/__tests__/RegenerateKeyModal.test.tsx`
**Commit:** e279d501
**Applied fix:** The `onCancel` passed to `useDialogFocus` now does nothing while `acting` is true. The hook reads the latest closure through its ref on every render. The reset effect now clears `error` and `acting` on open as well as on close, so a stale error is never shown. New test: Esc is ignored while `onConfirm` is pending and works again once it resolves. This one fix covers all three users: End Session, Regenerate Signing Key and the disable-warning confirm.

#### WR-02: Tray "focus session" does nothing for a detached session

**Files modified:** `frontend/src/App.tsx`, `frontend/src/components/__tests__/App.nav.test.tsx`
**Commit:** 0525d622
**Status:** fixed: requires human verification (tray behaviour can only be checked in the native GUI)
**Applied fix:** The `tray:focus-session` handler returns early for exited sessions. Otherwise, inside the functional `setTabs` updater, it focuses the open tab or appends `{ id: sessionId, name: 'shell', sessionId }` and activates it. Tabs are still matched by `sessionId`, as before. The tab label comes from `tabLabel` once the session list has it. Source-inspection test added.

#### WR-03: The stored Appearance is never applied after "Retry Connection"

**Files modified:** `app.go`, `app_test.go`, `frontend/src/App.tsx`, `frontend/src/components/__tests__/App.test.tsx`
**Commit:** bba0b62f
**Applied fix:** A successful `RetryDaemon` now calls `a.applyStoredAppearance()`. `retryInit` hydrates `appearance` through `GetAppearance()` after `RetryDaemon`, the same way `init` does. `TestRetryDaemonSuccess_KeepsClientAndPollers` now stores Dark and checks that the retry applies it. The App.test assertion that `GetAppearance()` appears exactly once now expects 2, and a test checks that the retryInit call comes after `RetryDaemon`.

#### WR-04: Changing the Appearance quickly can leave stored, native and on-screen values out of sync

**Files modified:** `app.go`, `frontend/src/components/SettingsSheet.tsx`, `frontend/src/components/__tests__/SettingsSheet.test.tsx`
**Commit:** 1bc6c142
**Status:** fixed: requires human verification (race fix)
**Applied fix:** Frontend: an `appearanceSavingRef` in-flight guard. `selectAppearance` ignores new choices while a save is pending. Arrow keys do nothing during a save, so focus stays on the checked segment. Rollback can therefore never overwrite a newer choice. Go: an `appearanceMu` mutex makes persist-then-apply in `SetAppearance`, and read-then-apply in `applyStoredAppearance`, atomic per call. Native `dispatch_async` hops are therefore queued in the order the values were persisted. New test: a second click or arrow press during a pending save is ignored, and a later click works.

#### WR-05: `domReady` waits on `startupDone` with no bound, and the daemon client has no timeout

**Files modified:** `app.go`, `app_test.go`, `internal/daemon/client.go`
**Commit:** aa239574
**Applied fix:**
- `domReady` now waits on `select { <-startupDone | time.After(startupWait) }`, where `startupWait` is a 5 s package var so tests can shorten it.
- In `startup`, the daemon block runs in a closure that closes `startupDone` with `defer`.
- `DaemonClient.http` now has `Timeout: 30 * time.Second`. I chose 30 s rather than the review's 5 s because the slowest bounded endpoint, `/tailnet/peers`, caps itself at 10 s server-side, and Funnel and webserver start make local calls. The client has no streaming endpoints; only `doJSON` and `ShutdownDaemon` use it.
- New tests: `TestDomReady_WaitsForStartupDone` and `TestDomReady_BoundedWhenStartupHangs`. Both set start-minimized so `WindowShow` is skipped, because tests have no Wails ctx.

**Remaining limit:** `domReady` still calls `GetStartMinimized` on the message loop. With a wedged daemon, that call can block the loop for up to the 30 s client timeout. Before this fix the block was unbounded.

#### WR-06: The session poll can bring back an exited or ended session

**Files modified:** `frontend/src/App.tsx`, `frontend/src/components/__tests__/App.nav.test.tsx`, `frontend/src/components/__tests__/App.test.tsx`
**Commit:** ef12c91b
**Status:** fixed: requires human verification (race fix)
**Applied fix:**
- The poll does `setSessions(list.filter((s) => !exitedIdsRef.current.has(s.id)))`.
- `handleOpenSessionTab` returns early for exited ids.
- `endSession` adds the id to `exitedIdsRef` after `KillSession` resolves.
- The source-inspection tests were updated to match.

#### IN-10: Fresh-install plugin settings are zero-valued (added by orchestrator)

**Files modified:** `internal/daemon/engine.go`, `internal/daemon/engine_migration_test.go`
**Commit:** 7bd1b323
**Applied fix:** The `NewSessionEngine` struct literal now seeds `pluginSettings: defaultPluginSettings()` and `shellWebShareWarningEnabled: &true`. When settings.json exists, `loadSettingsFromDisk` overwrites both, so its defaults-merge path is unchanged. A missing or unparseable file now leaves the defaults in place. New `TestNewSessionEngine_FreshInstallDefaults` sets `HOME` to an empty temp dir and checks that settings.json is absent, the plugin settings equal `defaultPluginSettings()`, WebGL is true, image `storageLimit` is 16 and the warning is enabled.

#### IN-05: `OpenTailscale` finds `open` through PATH (added by orchestrator)

**Files modified:** `app.go`, `app_test.go`
**Commit:** 453c15b5
**Applied fix:** A new `openAppCmd(name)` returns `exec.Command("/usr/bin/open", "-a", name)`, and the default opener uses it. New `TestOpenAppCmd_AbsolutePath` checks `cmd.Path` and `cmd.Args`. The existing seam-based OpenTailscale tests are unchanged and still pass.

#### IN-09: The danger cue on the Full-access segment is visual only (added by orchestrator)

**Files modified:** `frontend/src/components/Share/ShareSegmentedControl.tsx`, `frontend/src/components/__tests__/ShareSegmentedControl.test.tsx`
**Commit:** 903e37cf
**Applied fix:** An enabled danger segment gets `aria-label="{main}, Warning: {sub}"`, which reads "Internet, Warning: Full access". The icon stays `aria-hidden` and the visuals are unchanged. Other segments keep their text-content name. New test checks the label and that only the danger tab has an `aria-label`. No e2e test matches the tab by name.

#### IN-07: Unused dependency and unhandled promise rejections (added by orchestrator)

**Files modified:** `frontend/src/App.tsx`, `frontend/src/components/SettingsSheet.tsx`, `frontend/src/components/NewSessionModal.tsx`, `frontend/src/components/__tests__/NewSessionModal.test.tsx`, `frontend/src/components/__tests__/SettingsSheet.web-link-ux.test.tsx`
**Commit:** 65c0b5ad
**Applied fix:**
- `createTab` deps are now `[tabCounter]`; it never read `webServerRunning`.
- `handleCopyURL` catches a `ClipboardSetText` rejection and sets the existing `serverError` ("Could not copy the URL — …").
- `handleBrowse` catches an `OpenDirectoryDialog` rejection and sets the dialog's existing `error` ("Couldn't open the folder picker: …").
- Tests: a behavioural test for the picker failure and a source-inspection test for the copy catch.

---

_Fixed: 2026-11-16T01:18:40Z (iteration 3), 2026-11-16T01:09:55Z (iteration 2), 2026-11-16T00:50:05Z (iteration 1)_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 3_
