---
phase: 06-release-updates
fixed_at: 2026-11-18T19:44:22Z
review_path: .planning/phases/06-release-updates/06-REVIEW.md
iteration: 1
findings_in_scope: 10
fixed: 9
skipped: 1
status: partial
---

# Phase 6: Code Review Fix Report

**Fixed at:** 2026-11-18T19:44:22Z
**Source review:** .planning/phases/06-release-updates/06-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 10 (WR-01..WR-04, IN-01..IN-06)
- Fixed: 9
- Skipped: 1 (IN-05)

Fixes were edited and committed in the main checkout, on branch `gsd/phase-06-release-updates`, as the caller asked. No worktree was used. All gates ran in the main checkout, so the results can be reproduced from this tree.

## Fixed Issues

### WR-01: Verify does not bind the DMG to the release version or product

**Files modified:** `internal/updater/install.go`, `internal/updater/service.go`, `.github/workflows/release.yml`, `internal/updater/install_test.go`, `internal/updater/service_test.go`, `internal/updater/real_darwin_test.go`, `internal/updater/release_workflow_test.go`, `app_update_test.go`, `TESTING.md`, `.planning/phases/06-release-updates/06-VM-CHECKS.md`
**Commits:** b0d55bb6 (fix), 9650b4f6 (note in the VM-check record)
**Status:** fixed (security-relevant; a human should review it)
**Applied fix:**
- release.yml signs the DMG with `--identifier "portty-${VERSION}-darwin-universal"`. Its "Verify DMG like the in-app updater" step now also requires the exact line `Identifier=portty-${VERSION}-darwin-universal`.
- `Verify(ctx, run, path, teamID, assetName, now)` requires `Identifier=<assetName without .dmg>` from `codesign -dvv`. `Service.Download` passes `rel.DMG.Name`. The order is codesign --strict, then Team, then identifier (new failure step `identifier`), then spctl, then quarantine.
- New TestVerifyFailures cases for: no identifier line, an older Portty identifier, the GIFsmith identifier, and an identifier with extra trailing characters.
- `TestRealDMG` now reads the DMG's real identifier and checks two things:
  - the DMG passes all real checks under its own identifier (codesign, Team and spctl still pass);
  - the same DMG is refused at step `identifier` under a Portty asset name.
- I ran `TestRealDMG` on the host with GIFsmith v1.0.1's DMG (sha256 4119ccd1…faa1, `Identifier=Gifsmith-1.0.1-arm64`):
  - accepted under its own name;
  - "as portty-v0.0.1-darwin-universal.dmg: download verification failed: identifier";
  - byte-flipped copy: codesign; unsigned: codesign; ad-hoc: team.
- I also checked with an ad-hoc signature that an explicit `--identifier` is sealed into a DMG's signature and survives a rename. The host Developer ID was not used.

**Changed expectation:** the M-92 U-05 reveal used GIFsmith's DMG served under Portty's asset name. Verify now refuses that DMG at the identifier step. So the reveal path can only be re-checked with a notarized Portty release DMG, which first exists at M-93. This is noted in TESTING.md (M-92 step 2) and 06-VM-CHECKS.md.

### WR-02: The vmpreview gate is not enforced by the release pipeline

**Files modified:** `.github/workflows/release.yml`, `internal/updater/release_workflow_test.go`, `TESTING.md`
**Commit:** 24a93350
**Applied fix:**
- The `validate` job runs `bash tests/check-brand.sh`.
- A new build-macos step, "Release binary carries no VM-only fakes", runs between "Build macOS app" and the tar step. It fails if `build/bin/Portty.app/Contents/MacOS/Portty` is missing or if `grep -ac '127.0.0.1:8788'` on it is not 0.
- I checked it locally: a `-tags vmpreview` build gives 1 match and a plain build gives 0. The snippet fails on the vmpreview binary and on a missing file.
- The step text avoids the literal tag name, so check-brand.sh stays clean.
- New test: `TestReleaseWorkflow_VMOnlyFakesGated`.

### WR-03: Tests delete and overwrite the real user's `update_notified` file

**Files modified:** `app_update_test.go`, `TESTING.md`
**Commit:** 0fad9fed
**Applied fix:**
- `newUpdateHarness` runs `t.Setenv("HOME", t.TempDir())`. On darwin, `os.UserConfigDir` and `os.UserCacheDir` read `$HOME`, so `update_notified` and the updates folder land in a temp dir.
- The isolation also covers a second leak the review did not name: the harness's `Download`/`Clean` used the real `~/Library/Caches/app.portty.Portty/updates`.
- `resetNotified`, which deleted the real file, is removed.
- New test: `TestUpdateHarness_IsolatesUserFiles`.

### WR-04: A newer release whose DMG is not yet installable counts as "up to date" for 24 h

**Files modified:** `internal/updater/service.go`, `internal/updater/service_test.go`, `app_update_test.go`, `TESTING.md`, `.planning/phases/06-release-updates/06-VM-CHECKS.md`
**Commit:** 07c056bd
**Status:** fixed: requires human verification (state-handling logic)
**Applied fix:**
- If `Latest` returns `ErrNotInstallable` for a version newer than the running one, `Service.Check` now:
  - restores the previous status;
  - returns `&Error{Kind: "check", Text: "Version X isn't ready to download yet. Try again later."}`.
- So `autoCheck` stays quiet and retries on the next hourly tick, and the manual check shows "Couldn't check for updates." with that text.
- A release that is the same version or older and has no DMG still counts as "current". Otherwise a failed upload of the running version would error forever.
- Tests:
  - `TestService_CheckNewerNotInstallable` (no DMG, uploading, no digest);
  - a new "same version, no DMG" case in `TestService_CheckQuietOutcomes`;
  - `TestUpdatePoller_NotReadyRetriedHourly`;
  - `TestManualUpdateCheck_NotReady`.

**Changed expectation (U-06 noasset):**
- Automatic checks still show no banner, notification or alert. They now retry on the next tick instead of waiting 24 h.
- The manual check now shows "Couldn't check for updates." / "Version 9.9.9 isn't ready to download yet. Try again later." where it used to show "Portty is up to date.".
- TESTING.md M-92 U-06 is updated. 06-VM-CHECKS.md has a post-review note; the recorded PASS predates the change.
- ROADMAP SC4 (nothing published / 404 means "up to date") is unchanged and still holds.

### IN-01: A backwards wall-clock jump stops automatic checks until the clock catches up

**Files modified:** `app_update.go`, `app_update_test.go`, `TESTING.md`
**Commit:** b1dcc0d2
**Status:** fixed: requires human verification (condition change)
**Applied fix:** `runUpdateLoop` now skips a tick only when `0 <= now-last < updateEvery`. A negative gap counts as due.
**Test:** `TestUpdatePoller_ClockSetBack`. The clock is set back 30 days, the next tick checks, and the daily cadence then resumes.

### IN-02: Verify inherits what is left of the download's 10-minute deadline

**Files modified:** `internal/updater/service.go`, `internal/updater/install_test.go`, `internal/updater/service_test.go`, `TESTING.md`
**Commit:** b8e8130a
**Applied fix:** Verify and Reveal run on a fresh `context.WithTimeout(context.Background(), 2*time.Minute)`.
**Test:** `fakeRun` records how much time is left on each call's deadline. `TestService_DownloadReveals` asserts every verify and reveal call has at most 2 minutes left. The same commit also gofmts the WR-01/WR-04 test tables.

### IN-03: A manual check queued behind an automatic check shares its 30 s budget

**Files modified:** `internal/updater/service.go`, `internal/updater/service_test.go`, `app_update.go`, `TESTING.md`
**Commit:** d1dd9fbf
**Applied fix:**
- `Service.Check` applies `checkTimeout` (30 s, a package variable) after it takes `checkMu`.
- `autoCheck` and `checkForUpdatesManually` no longer create their own timeouts before taking the lock. The manual check passes `context.Background()` and autoCheck passes the loop's context.

**Test:** `TestService_CheckQueuedGetsOwnBudget`. It sets the budget to 300 ms. The first check runs out its budget, and the check queued behind it still succeeds on its own budget.

### IN-04: The tray item in the revealed state re-downloads instead of re-revealing

**Files modified:** `internal/updater/service.go`, `internal/updater/service_test.go`, `app_update_test.go`, `TESTING.md`
**Commit:** 2fcc7dfc
**Status:** fixed: requires human verification (state handling)
**Applied fix:**
- From `revealed`, `Service.Download` calls the new `revealKept`:
  - `Dir/<asset name>` must be a regular file (Lstat);
  - `open -R` runs on a 10 s context, under `s.mu`;
  - the status is left unchanged and no event is emitted.
- If the file is gone or the reveal fails, Download fetches the DMG afresh as before.
- U-05's "file removed, then Check for Updates…" flow is unaffected: the check sets the state back to available.
- Tests:
  - `TestService_DownloadAfterRevealRevealsAgain` (one `open -R`, no new asset request; after removal, a fresh download);
  - a `revealed` subtest in `TestTrayUpdate_OfferUpdate`.

### IN-06: The banner's mount snapshot can overwrite a newer event

**Files modified:** `frontend/src/App.tsx`, `frontend/src/components/__tests__/App.update.test.tsx`, `TESTING.md`
**Commit:** 9772d702
**Applied fix:** App subscribes to `update:status` first and sets `gotEvent` in the handler. The `GetUpdateStatus()` reply is applied only if no event has arrived yet.
**Test:** a new vitest. GetUpdateStatus is held, a downloading event fires, then the reply resolves to available, and the banner stays on "Downloading… 25%". I confirmed this test fails without the App.tsx change.

## Skipped Issues

### IN-05: Clicking the update notification has no handler

**File:** `tray_objc_darwin.m:84-95`
**Reason:** needs VM confirmation; tracked for verification
**Original issue:** `PorttyNotificationDelegate` does not implement `userNotificationCenter:didReceiveNotificationResponse:withCompletionHandler:`. So clicking "Version X is available." while the window is hidden probably activates the app without showing the window or banner. This needs to be confirmed in the Tart VM.

## Verification (main checkout, GOTOOLCHAIN=go1.26.11)

- `go build ./... && go vet ./... && go vet -tags vmpreview .`: pass
- `go test -race -short -skip '^(TestAPIAppearance_GetPatch|TestAPITerminalFont_GetPatch)$' ./...`: pass (exit 0; no flake reruns needed)
- `cd frontend && pnpm vitest run`: 88 files, 1199 tests pass
- `bash tests/check-brand.sh`: OK
- `bash tests/check-traceability-paths.sh`: OK (138 paths)
- Host real-tools run: `UPDATE_REAL_DMG=<GIFsmith v1.0.1 DMG> UPDATE_REAL_BAD_DMG=<unsigned>:<adhoc> go test -run '^TestRealDMG$' ./internal/updater` passes, with GIFsmith refused at `identifier` under a Portty asset name.
- No Wails build ran, so `frontend/src/wailsjs` is untouched. No new test files were added; the Section 4 rows in TESTING.md were updated for the new test functions.

---

_Fixed: 2026-11-18T19:44:22Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_

## Iteration 2

| ID | Status | Commit | Note |
|---|---|---|---|
| IN-01 (iter 2) | fixed | 6f9411cc | Check keeps `revealed` when it finds the same release (`rel == s.rel`), so Download re-shows the kept DMG. A relaunch still re-downloads (no in-memory release); acceptable, fully re-verified. Test extended in `TestService_DownloadAfterRevealRevealsAgain`; fails without the fix. |
