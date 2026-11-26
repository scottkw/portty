---
phase: 19
fixed_at: 2026-11-26T23:10:00Z
review_path: .planning/phases/19-unattended-desktop-viewing/19-REVIEW.md
iteration: 1
findings_in_scope: 13
fixed: 10
skipped: 3
status: partial
---

# Phase 19: Code Review Fix Report

**Fixed at:** 2026-11-26T23:10:00Z
**Source review:** .planning/phases/19-unattended-desktop-viewing/19-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 13 (scope `all`: CR-01, WR-01..WR-07, IN-01..IN-05), plus the three 19-UI-REVIEW items the orchestrator added
- Fixed: 10 review findings and the 3 UI items
- Skipped (deferred): 3 (IN-03, IN-04, IN-05)

Every fix is its own commit. A regression test was written first and seen failing (RED) before the fix, except where noted.

## Fixed Issues

### CR-01: Regenerating the signing key does not revoke a viewer that is mid-handshake

**Files modified:** `internal/webserver/desktop_routes.go`, `internal/daemon/desktop_api.go`, `internal/desktop/manager.go`, `internal/desktop/loopback_test.go`, `internal/webserver/desktop_routes_test.go`, `internal/webserver/desktop_csp_e2e_test.go`
**Commit:** 779d5924
**Applied fix:** `DesktopBackend.ServeDesktopViewer` takes an `admit func() bool`. The route builds it as "the token still verifies under the current signing key" (`ws.currentSigningKey` is a leaf lock). `join` calls it before capture starts, and again in the same `vmu` hold as the registration. `CloseAll` also takes `vmu`, so a revocation either finds the viewer or the check sees the new key. A refused viewer is closed with 4401. A nil admit admits nobody. I did not add a per-Manager revocation counter: a key re-check needs no state, and a counter would reset when the Manager is rebuilt. Test: `TestDesktopWS_RegeneratedKeyRefusesHandshakingViewer` (RED: the viewer was answered after regeneration).

### WR-01: `Manager.mu` is held across blocking native capture starts

**Files modified:** `internal/desktop/manager.go`, `internal/desktop/runtime_state.go`, `internal/desktop/watchdog.go`, `internal/desktop/close_breaker_test.go`, `internal/desktop/fakes_test.go`
**Commit:** a0279fd3
**Applied fix:**
- A start is reserved under `mu` (a `starting` flag), runs `src.Start` unlocked, then publishes under `mu`. A start that finishes after Close, or with no viewers left, stops its source and records nothing.
- `restartSource` skips a restart while a start is in flight.
- A viewer is now registered before its start.
- `closed` is an `atomic.Bool`. `Close` clears `captureActive` before it takes `mu`.
- `noteStartSuccess` reads `closed` inside the state file's `rtMu`, which serialises it against Close's clear, so a late start never writes the flag back.

Tests:
- `TestClose_ClearsCaptureActiveDuringBlockedStart` (RED: the flag stayed true while a restart was blocked).
- `TestJoin_NotBlockedByAnotherViewersStart` (RED: the second viewer was never answered).

**Residual:** the first viewer's own answer still waits for its own start (consequence 3 in the review). Moving the start to a goroutine would make the existing tests nondeterministic, so it was left.

### WR-02: RTP timestamps lag one access unit behind capture time

**Files modified:** `internal/desktop/manager.go`, `internal/desktop/export_test.go`, `internal/desktop/fanout_test.go`
**Commit:** 5a0ec826
**Applied fix:** a new `packetize` helper calls `SkipSamples(ticks)` and then `Packetize(data, 0)`. The ticks are integer math, `s*rate + frac*rate/1e9`, and wrap mod 2^32. Test: `TestFanout_RTPTimestampFollowsCaptureTime` (RED against the old formula: the 5 s step landed on the next unit, and the 14 h step saturated).

### WR-03: `RefreshPermissions` is never called

**Files modified:** `internal/daemon/desktop_api.go`, `internal/daemon/desktop_permissions_test.go`
**Commit:** 36d55f9a
**Applied fix:** after a successful read, `handleDesktopPermissions` calls `mgr.RefreshPermissions` on the running Manager, outside `a.mu`. Test: `TestDesktopPermissions_PollRefreshesState` (RED: the state stayed idle after a not-granted poll).

### WR-04: launchd liveness depends on one regex

**Files modified:** `internal/daemon/launchd.go`, `internal/daemon/launchd_test.go`, `cmd_daemon_test.go`
**Commits:** 202b8425, db033df0
**Applied fix:** `jobLoaded` returns pid 0 only when launchd prints `state = not running`. Any other output with no parsable pid returns `pidUnknown` (-1), and every caller treats that as running:
- `EnsureAgent` defers the reload.
- `uninstallAgent` refuses with `ErrAgentRunning`.
- `LegacyCleanup` leaves the job alone.

Test: `TestLaunchd_UnparsedPidCountsAsRunning` (RED: bootout ran three times). The follow-up commit db033df0 fixes the root package's CLI fake, which printed `pid = 0`, a line launchd never prints.

**Residual:** the race window between `print` and `bootout` is not addressed.

### WR-05: The offer-wait loop reads without a deadline once the deadline has passed

**Files modified:** `internal/desktop/manager.go`
**Commit:** df7164c3
**Applied fix:** the loop returns when `time.Until(offerDeadline) <= 0`. There is no automated test: the deadline is a 30 s wall-clock constant and has no seam.

### WR-06: `fanOut` does not handle a closed Errors channel

**Files modified:** `internal/desktop/manager.go`, `internal/desktop/fakes_test.go`, `internal/desktop/fanout_test.go`
**Commit:** babc963f
**Applied fix:** `case err, ok := <-errs`. A closed channel is set to nil, and the frames case ends the loop as before. Test: `TestFanout_ClosedErrorsChannelIsQuiet` (RED: 31,809 "capture error" lines in 100 ms).

### WR-07: `lapseTried` is never reset when `LapseSeq` restarts

**Files modified:** `app.go`, `app_lapse_test.go`
**Commit:** e3377af2
**Applied fix:** the guard resets to 0 (by CAS) when the status leaves `awaiting_approval` or shows a lower `LapseSeq`. The daemon's `ClaimLapse` is still the one-sender gate. Test: `TestRefreshTray_ClaimsAfterLapseSeqRestarts` (RED: claims were [3], want [3 1 1]).

### IN-01: Doc comment promises a power-hold retry that is not implemented

**Files modified:** `internal/desktop/manager.go`, `internal/desktop/power.go`, `internal/desktop/power_test.go`
**Commit:** cf9c60ab
**Applied fix:** `takeViewerHolds` now runs on every join. It is idempotent, because `hold()` keeps an assertion that is already held. Test: `TestPower_FailedHoldRetriedOnNextJoin` (RED).

### IN-02: `LoadConfig` does not reject trailing data

**Files modified:** `internal/desktop/config.go`, `internal/desktop/config_test.go`
**Commit:** ed9848f4
**Applied fix:** after decoding, `LoadConfig` checks for EOF, as `readRuntimeState` does. Test: a `trailing data` case in `TestDesktopConfig_Unsafe` (RED).

### UI (19-UI-REVIEW): viewer buttons inside the live region

**Files modified:** `frontend/src/components/DesktopView.tsx`, `frontend/src/components/__tests__/DesktopView.test.tsx`, `frontend/src/style.css`
**Commit:** 08d2ee85
**Applied fix:** the `role="status"` region now holds only the chip and card text. It stays mounted, so state changes are still announced. Tap to Start and Reload Page are siblings of the region inside the same `.desktop-card` box. A `.desktop-status__text` column keeps the layout. Test: a new case asserts that both buttons sit outside the region and that the region persists (RED).

### UI (19-UI-REVIEW): Test Capture result laid out side by side

**Files modified:** `frontend/src/style.css`
**Commit:** 01bd357f
**Applied fix:** `.desktop-setup__result` is now `flex-direction: column` with an 8px gap. There is no test, since jsdom does not compute layout.

### UI (19-UI-REVIEW): `capture_paused` copy missing from the contract

**Files modified:** `.planning/phases/19-unattended-desktop-viewing/19-UI-SPEC.md`
**Commit:** 7ac943e8
**Applied fix:** the existing strings are kept, as the orchestrator decided: "Desktop paused" and its viewer body, the Settings status "Paused after repeated errors. Run Test Capture to resume.", and the tray line "Desktop: Paused after repeated errors". They are added as a dated amendment to:
- the Surface 1 state table
- the precedence line
- the Surface 5 states table
- the Copywriting Contract

## Skipped Issues

### IN-03: The debug hook ships in the release viewer

**File:** `frontend/src/lib/desktopClient.ts:452-484`
**Reason:** deferred. `TestDesktopCSP_E2E` and the Phase 19 VM rig (`rig/p19-rig.sh` safari-stats) read `#debug` and `window.__desktopStats` on release builds, so a DEV-only gate would break them. The hook holds counts only. The listener leaks once per `start()`, which runs once per page.
**Original issue:** test-only behaviour in the production bundle; the `securitypolicyviolation` listener is never removed.

### IN-04: Production code branches on `testing.Testing()`

**File:** `internal/desktop/peer.go:41`, `internal/desktop/permissions.go:69,135`, `internal/daemon/launchd.go:54,127`, `app.go`
**Reason:** deferred. The review marks it optional, and moving the rails into TestMain seams is a refactor across packages, not a trivial fix.
**Original issue:** links `testing` into the shipped binary.

### IN-05: A persistently failing encoder logs once per frame

**File:** `internal/screencap/media_darwin.m:435`, `internal/desktop/manager.go`
**Reason:** deferred. Rate limiting, or a restart after N failures, is new behaviour, not a trivial fix. The stall watchdog limits the burst to seconds.
**Original issue:** up to about 30 Warn lines per second until the watchdog restarts the stream.

## Verification

All gates ran in the agent's isolated git worktree (branch `worktree-agent-a6d5975f81544d142`), not in the main checkout. `frontend/dist` is gitignored and was missing from the fresh worktree, which made `TestNoFakeWhoIsInRelease` fail at baseline, so `pnpm exec vite build` was run first.

- `GOTOOLCHAIN=go1.26.11 go vet ./...`: clean
- `GOTOOLCHAIN=go1.26.11 go test -race -short ./...`: all packages pass (see the final run in the hand-off report)
- `GOTOOLCHAIN=go1.26.11 golangci-lint run --new-from-rev=89d29266 ./...`: 0 issues
- `pnpm install --frozen-lockfile && pnpm exec tsc --noEmit && pnpm exec vitest run`: tsc clean, 126 files and 1961 tests pass
- `bash tests/check-brand.sh`: OK. `bash tests/check-traceability-paths.sh`: OK, 272 paths
- No test files were added (all new tests are in files already in TESTING.md), so TESTING.md is unchanged.

Nothing ran the app, the daemon, launchctl, screen capture, TCC or notifications on the host, and nothing contacted a tailnet address.

---

_Fixed: 2026-11-26T23:10:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
