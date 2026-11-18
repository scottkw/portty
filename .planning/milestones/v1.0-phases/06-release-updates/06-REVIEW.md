---
phase: 06-release-updates
iteration: 2
reviewed: 2026-11-18T19:48:51Z
depth: deep
files_reviewed: 13
files_reviewed_list:
  - .github/workflows/release.yml
  - app_update.go
  - app_update_test.go
  - frontend/src/App.tsx
  - frontend/src/components/__tests__/App.update.test.tsx
  - internal/updater/install.go
  - internal/updater/install_test.go
  - internal/updater/real_darwin_test.go
  - internal/updater/release_workflow_test.go
  - internal/updater/service.go
  - internal/updater/service_test.go
  - internal/updater/update.go
  - TESTING.md
findings:
  critical: 0
  warning: 0
  info: 1
  total: 1
  necessity: 0
status: issues_found
---

# Phase 6: Code Review Report (iteration 2)

**Reviewed:** 2026-11-18T19:48:51Z
**Depth:** deep
**Files Reviewed:** 13
**Status:** issues_found (one Info item; no blockers or warnings)

## Summary

Scope: `git diff b2bee9b7..HEAD -- . ':!.planning'`. That covers the fix commits b0d55bb6..9772d702 for the iteration-1 findings WR-01..WR-04, IN-01..IN-04 and IN-06, plus the code they call (`Checker.Latest`/`parse` in update.go, `Clean`/`Download` in install.go, the banner in UpdateBanner.tsx). IN-05 was skipped by the fixer. It still needs confirmation in the Tart VM and is not re-raised here.

Gates I re-ran for this review (GOTOOLCHAIN=go1.26.11):
- `go vet ./internal/updater/ .`: clean.
- `go test -race -short -count=1 ./internal/updater/ .`, with the two known skips: pass.
- `pnpm vitest run` on UpdateBanner.test.tsx and App.update.test.tsx: 23/23 pass.
- `tests/check-brand.sh`: OK.
- `tests/check-traceability-paths.sh`: OK (138 paths).

Per-finding verdicts:

- **WR-01: resolved.**
  - **Signing.** release.yml signs with `--identifier "portty-${VERSION}-darwin-universal"`. `VERSION` is `GITHUB_REF_NAME`, for example `v1.2.3`, the same variable that builds `DMG_NAME`. That matches `DMGAssetName("1.2.3")` = `portty-v1.2.3-darwin-universal.dmg` with `.dmg` removed, and `TestReleaseWorkflow_DMGName` asserts the match.
  - **Stapling.** A DMG ticket is stored in its own slot of the signature superblob, and the CodeDirectory (where the identifier lives) is not re-signed. So the identifier survives notarization and stapling. Even if it did not, the release.yml "Verify DMG like the in-app updater" step runs after stapling and checks the exact `Identifier=` line. A DMG without the identifier would fail the release rather than ship.
  - **Client check.** The client matches a whole line from `codesign -dvv` combined output, after stripping `\r`. The test cases cover a suffix (`...universalx`), a missing line, an older Portty identifier and a GIFsmith identifier.
  - **Older releases.** DMGs published before this change carry the default identifier. That is the file name at signing time, which was already `DMG_NAME`, so they still pass.
  - **Rollback.** The asset name is built from the validated tag, never taken from the JSON. A DMG signed for another version or another product cannot pass without the Team signing key.
- **WR-02: resolved for the accidental case.** The brand gate runs in `validate`. The binary grep runs between the build and tar steps. It handles a missing binary (`test -f`) and grep exit codes (`|| true`: an error leaves `N` empty and fails the step). A tagged commit that edits release.yml could remove both, but that is beyond what this finding asked for.
- **WR-03: resolved.**
  - `t.Setenv("HOME", t.TempDir())` runs before the service is built, so `updateCacheDir()` and every `notifiedVersionPath()` call resolve under the temp dir.
  - No main-package test uses `t.Parallel`, so the `Setenv` cannot panic.
  - No other test touches the notified file.
- **WR-04: resolved.**
  - A newer release that is not installable restores `prev` (state, `Latest` and `Error` intact; `s.rel` untouched) and returns a check error. `autoCheck` then returns false and `last` is not advanced, so the next hourly tick retries quietly.
  - A same or older version without a DMG still falls through to `current`, so a failed upload of the running version does not error forever.
  - ETag reuse on the not-ready body is harmless: a 304 parses to the same result, and an asset upload changes the ETag.
- **IN-01: resolved.** A negative gap counts as due, and once the check completes `last` resets to the corrected clock.
- **IN-02: resolved.** Verify and Reveal run on a fresh 2-minute context that has nothing to do with the download's deadline.
- **IN-03: resolved.** The 30 s budget starts after `checkMu` is acquired, and both callers dropped their own pre-lock timeouts. The check HTTP client's own 30 s timeout still bounds each request.
- **IN-04: partially resolved.** See IN-01 below.
- **IN-06: resolved.** The event handler is subscribed before the snapshot call, and the snapshot is dropped once any event has been applied. Go emits every state change under `s.mu`, in order, so the last event delivered is always at least as new as the snapshot. StrictMode double-mounting gets a separate `gotEvent` per effect.

## Narrative Findings (AI reviewer)

## Info

### IN-01: IN-04 fix only covers the window until the next Check; after that the tray, alert and banner Download delete and re-fetch the verified DMG again

**File:** `internal/updater/service.go:122-125` (with `service.go:163-166`, `service.go:190`, `app_update.go:176-178`)
**Issue:** `revealKept` runs only while the state is `revealed`. Any `Check` from `revealed` that finds the same version moves the state back to `available` (`case newer > 0` sets `Status{State: "available"}`). That includes:
- the automatic check, at most 24 h later;
- any manual Help > Check for Updates…;
- every relaunch, which starts at idle.

From `available`, `Download` runs `Clean(s.Dir)` and deletes the kept, verified DMG, which may still be mounted or open in Finder. It then downloads and verifies it again. That is the original IN-04 behavior. The manual-check path hits it right away: Check makes the state `available`, `offerDownload` follows, and choosing Download deletes the file and re-fetches. The fix and its tests (`TestService_DownloadAfterRevealRevealsAgain`, the `revealed` subtest) only cover the window between a reveal and the next check. Nothing is unsafe (the new file is fully re-verified), so this stays Info.
**Fix:** Keep the kept file across checks for the same version. Two options:
- In `Check`, when `newer > 0`, `prev.State == "revealed"` and `rel.Version == s.rel.Version`, return `s.set(prev)`, not `available`. U-05's "file removed" flow is still covered, because `revealKept` falls back to a fresh download when the file is gone.
- Or, in `Download` from `available`, try `revealKept()` first when `Dir/<asset name>` exists and that exact name was verified earlier in this process. Record the name on a successful verify.

---

_Reviewed: 2026-11-18T19:48:51Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
_Iteration: 2_
