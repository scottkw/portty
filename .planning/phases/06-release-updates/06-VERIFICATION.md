---
phase: 06-release-updates
verified: 2026-11-18T19:55:51Z
status: human_needed
score: 7/7 must-haves verified
covered_files:
  - ".github/workflows/release.yml"
  - ".planning/phases/06-release-updates/06-01-PLAN.md"
  - ".planning/phases/06-release-updates/06-01-SUMMARY.md"
  - ".planning/phases/06-release-updates/06-02-PLAN.md"
  - ".planning/phases/06-release-updates/06-02-SUMMARY.md"
  - ".planning/phases/06-release-updates/06-03-PLAN.md"
  - ".planning/phases/06-release-updates/06-03-SUMMARY.md"
  - ".planning/phases/06-release-updates/06-04-PLAN.md"
  - ".planning/phases/06-release-updates/06-04-SUMMARY.md"
  - "TESTING.md"
  - "app.go"
  - "app_update.go"
  - "app_update_release_test.go"
  - "app_update_test.go"
  - "cmd/fakegh/main.go"
  - "frontend/src/App.tsx"
  - "frontend/src/components/UpdateBanner.tsx"
  - "frontend/src/components/__tests__/App.test.tsx"
  - "frontend/src/components/__tests__/App.update.test.tsx"
  - "frontend/src/components/__tests__/UpdateBanner.test.tsx"
  - "frontend/src/style.css"
  - "frontend/src/wailsjs/go/main/App.d.ts"
  - "frontend/src/wailsjs/go/main/App.js"
  - "go.mod"
  - "go.sum"
  - "internal/updater/install.go"
  - "internal/updater/install_test.go"
  - "internal/updater/real_darwin_test.go"
  - "internal/updater/release_workflow_test.go"
  - "internal/updater/service.go"
  - "internal/updater/service_test.go"
  - "internal/updater/update.go"
  - "internal/updater/update_test.go"
  - "main.go"
  - "menu_test.go"
  - "tests/check-brand.sh"
  - "tray.go"
  - "tray_objc_darwin.m"
  - "updates_vmpreview.go"
covered_digest: "v4:sha256:4f85e809395e6307d659ee6dfe376ed725ca9f8d5cab2900612d2ce634aa4dcb"
behavior_unverified: 0
overrides_applied: 0
decision_coverage:
  honored: null
  total: 0
  not_honored: []
human_verification:
  - test: "M-93 step 1: push the first v* tag (repo public, release secrets and the release environment in place) and watch release.yml's sign-macos job"
    expected: "Create and sign DMG (codesign --identifier portty-vX.Y.Z-darwin-universal), Notarize and staple DMG, and Verify DMG like the in-app updater (codesign --verify --strict, TeamIdentifier=S2K7P43927, Identifier=portty-vX.Y.Z-darwin-universal, spctl accepted with source=Notarized Developer ID) all pass on the first CI run; the release has the DMG with a sha256 digest"
    why_human: "Needs Apple notarization credentials and GitHub release secrets; the steps have never run in CI. Only TestReleaseWorkflow_* pins their text"
  - test: "M-93 step 2: on the host, run TestRealDMG (UPDATE_REAL_DMG=<release DMG>) and TestReleaseJSON (UPDATE_RELEASE_JSON=<saved releases/latest JSON>, UPDATE_DMG_PATH=<DMG>) against the real release"
    expected: "Both pass and are not skipped: the real codesign and spctl accept the notarized Portty DMG under its own asset name (identifier step included), and the checker offers exactly that DMG and its digest"
    why_human: "No signed and notarized Portty DMG exists yet. Since WR-01 the identifier check refuses GIFsmith's DMG, so the accept path with real tools on current code cannot be run before the first release"
  - test: "M-93 step 3: with an older notarized release installed on the home Mac and a newer one published, launch Portty and wait about 10 s"
    expected: "The update banner appears and a native notification attributed to Portty is delivered once for that version; Help -> Check for Updates... offers the new version; Download verifies and reveals the DMG in Finder (0600, quarantine flag naming Portty), and the DMG opens and installs"
    why_human: "Notification delivery was not observable on the ad-hoc VM build (UNErrorDomain Code=1), and the U-05 reveal evidence predates the WR-01 identifier check. Note: an 'older installed release' needs two published releases (install the first, then publish the second)"
  - test: "IN-05: with Portty's window hidden (quit to menu bar), click the 'Version X is available.' notification"
    expected: "Decide whether this is acceptable. PorttyNotificationDelegate has no didReceiveNotificationResponse handler, so the click probably activates the app without showing the window or the banner"
    why_human: "Clicking a delivered notification needs a notarized build that is allowed to post notifications; product decision whether a handler is needed"
  - test: "U-06 noasset re-run after WR-04 (Tart VM, Claude can run it under M-92): fake GitHub serves a newer 9.9.9 with no DMG asset"
    expected: "Automatic checks stay quiet (no banner, notification or alert) and retry on the next tick; Help -> Check for Updates... says 'Couldn't check for updates.' / 'Version 9.9.9 isn't ready to download yet. Try again later.'"
    why_human: "The recorded U-06 PASS predates WR-04, which changed the manual-check text for this case. Logic is covered by TestService_CheckNewerNotInstallable, TestUpdatePoller_NotReadyRetriedHourly and TestManualUpdateCheck_NotReady; the native alert text needs a GUI session"
---

# Phase 6: Release Updates Verification Report

**Phase Goal:** Installed copies of Portty find out about new GitHub releases on their own or on demand, and the user can download, verify and open an update in a couple of clicks, using the same flow as GIFsmith
**Verified:** 2026-11-18T19:55:51Z
**Status:** human_needed
**Re-verification:** No (initial verification)

## Goal Achievement

The code delivers every success criterion, and every criterion is backed by passing behavioural tests plus the Tart VM run (06-VM-CHECKS.md). What is left can only be checked on the first real release: real notarization in CI, real codesign/spctl acceptance of a Portty DMG, notification delivery, and clicking the notification. These are human items (M-93), not code gaps.

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC1 / UPD-01: with a newer release published (or a fake GitHub), Portty shows an update notice and a native notification shortly after launch and again on its periodic check, without user action | VERIFIED | `app.go:162` `a.updates = a.newUpdateService()`, `app.go:253` `a.startUpdatePoller(ctx)`. `app_update.go` `runUpdateLoop`: first check after `updateFirstDelay` (10 s), then an hourly tick that checks once 24 h of wall-clock time has passed; a failed check retries on the next tick. `autoCheck` -> `notifyUpdateOnce` (`sendNotificationFunc`, remembered in `update_notified`). `onUpdateStatus` emits `update:status` and sets the tray item. `App.tsx:723-741` subscribes and renders `UpdateBanner`. Tests: TestUpdatePoller_FirstCheckAfterDelay, TestUpdatePoller_DailyCadenceAndNoRepeat (asserts 1 notification after a 304, a second only for 9.9.10), TestUpdateDefaults_ReleaseBuild (10 s / 1 h / 24 h, api.github.com), App.update.test.tsx. VM U-01/U-02 PASS (first request 12 s after open, one dispatch per version). Delivery itself is a human item |
| 2 | SC2 / UPD-02: "Check for Updates…" runs a check at once and reports up to date, version X available, or what went wrong | VERIFIED | `main.go:172` Help item "Check for Updates…" -> `go appInstance.checkForUpdatesManually()`. `app_update.go` `checkForUpdatesManually`: alerts "Portty is up to date." / offerDownload "Version X is available." (Download, Later) / "Couldn't check for updates." + readable text (network, rate limit, not ready) / "This build doesn't check for updates." Tests: TestManualUpdateCheck_UpToDate, _AvailableLater, _AvailableDownload, _Error, _DevBuild, _WhileDownloading, _NotReady; TestAppMenu_Structure. VM U-03 PASS |
| 3 | SC3 / UPD-03: Download fetches the release DMG, rejects it on a SHA-256 or Developer ID Team mismatch, and otherwise reveals the verified DMG in Finder | VERIFIED | Banner Download -> `DownloadUpdate` -> `Service.Download`: `Download` (exact size, SHA-256 -> `ErrDigest`, `.part` 0600 renamed only on success) -> `Verify` (`codesign --verify --strict`, `TeamIdentifier=S2K7P43927`, `Identifier=<asset name without .dmg>`, `spctl ... source=Notarized Developer ID`, quarantine) -> `Reveal` (`/usr/bin/open -R`). A failed file is deleted; state returns to available with error text. Tests: TestFlow_CheckDownloadVerifyReveal, TestDownloadDigestMismatch, TestVerifyFailures (codesign, team, identifier x4, spctl, quarantine), TestService_DownloadReveals, TestService_DownloadFailures, TestService_DownloadOneAtATime. VM U-04 PASS (unsigned, ad-hoc, bad digest rejected with real tools, folder empty). The U-05 real reveal PASS predates WR-01; the real-tools accept path on current code waits for the first release (human item) |
| 4 | SC4 / UPD-04: with no release published or a 404, Portty reports "up to date" or stays silent, never a false update or repeated error popups | VERIFIED | `update.go` `Latest`: 404 -> found=false, nil error; drafts, prereleases and non-`vX.Y.Z` tags -> not found; 403/429 -> `ErrRateLimited`. `Service.Check` restores the previous status on any failure (no error state from a check); automatic checks never open an alert (only `checkForUpdatesManually` and `offerUpdate` call `alert`/`dialog`). Tests: TestLatestNotFoundAndErrors, TestLatestNotOffered, TestLatestRateLimited, TestService_CheckQuietOutcomes, TestService_CheckFailures, TestService_CheckNewerNotInstallable, TestUpdatePoller_FailureRetriedHourlyAndQuiet (500 and 403: no available state, no notification, loop keeps running), TestUpdatePoller_NotReadyRetriedHourly. VM U-06 PASS (noasset row re-run listed as a human item after WR-04) |
| 5 | 06-01: release.yml signs the DMG with its asset-name identifier, notarizes and staples it, and runs the client's own checks before upload | VERIFIED | `.github/workflows/release.yml:220-255`: `codesign --identifier "portty-${VERSION}-darwin-universal"`, `notarytool submit --wait --timeout 60m`, `stapler staple`/`validate`, then grep -qx checks for TeamIdentifier, Identifier and source=Notarized Developer ID. `VERSION="${GITHUB_REF_NAME}"` (`v1.2.3`) matches `DMGAssetName` (`portty-v1.2.3-...`). Prerelease flag on any `-` tag. TestReleaseWorkflow_* (4) pass. First real CI run is a human item |
| 6 | 06-03: the banner is driven only by Go's Status (available, downloading with progress, verifying, revealed, error with Try Again); no update UI opens a URL | VERIFIED | `UpdateBanner.tsx` renders all states, no `BrowserOpenURL`; `App.tsx` ignores `checking`, clears dismissal on `downloading`, drops a late mount snapshot (IN-06), web mode never subscribes. UpdateBanner.test.tsx and App.update.test.tsx pass (vitest 88 files / 1199 tests) |
| 7 | 06-02: go-selfupdate and the 1 h throttle are gone; release builds use https://api.github.com and the exact-host allowlist; the VM fake feed cannot reach a release build | VERIFIED | No `creativeprojects` in go.mod/go.sum, no `Masterminds` in go.mod; `internal/updater/updater.go` deleted; no `GetLastUpdateInfo`/`CheckForUpdates` callers (App.test.tsx pins their absence). `updates_vmpreview.go` is `//go:build vmpreview`; tests/check-brand.sh and TestReleaseWorkflow_VMOnlyFakesGated guard the tag and the 127.0.0.1:8788 address; TestGitHubAllow, TestNewCheckClient pass |

**Score:** 7/7 truths verified (0 present, behaviour-unverified)

Behaviour-dependent invariants (one operation at a time, serialized checks, failure restores the previous status, wall-clock cadence and clock set back) each have a passing named test: TestService_DownloadOneAtATime, TestService_CheckSerialized, TestService_CheckQueuedGetsOwnBudget, TestService_CheckFailures, TestUpdatePoller_ClockSetBack.

#### Plan-truth wording superseded by review fixes (informational, not gaps)

- 06-04 truth 4 says a manual check reports "Portty is up to date." for "a release without the DMG". WR-04 (06-REVIEW-FIX.md) changed this on purpose: a newer release without an installable DMG is now an incomplete check ("Version X isn't ready to download yet."), retried hourly and still silent on automatic checks. ROADMAP SC4 and UPD-04 do not list this case and stay satisfied (no false update, no repeated popups). 06-VM-CHECKS.md records the change.
- 06-01 truth 4 (Verify steps) is still true. WR-01 added a signing-identifier step on top of it, and release.yml signs with that identifier to match.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/updater/update.go` | Checker (ETag, 404/draft/prerelease quiet, rate limit), allowlist, asset rules | VERIFIED | 219 lines, used by app_update.go |
| `internal/updater/install.go` | Download, Verify, Reveal, Clean, ExecRunner | VERIFIED | 183 lines, called from service.go |
| `internal/updater/service.go` | State machine, Check, Download | VERIFIED | 234 lines, wired in NewApp |
| `app_update.go` | Poller, notify-once, bindings, manual alerts, tray | VERIFIED | 249 lines; startup, main.go and tray.go call into it |
| `main.go` | Help -> "Check for Updates…" | VERIFIED | line 172 |
| `tray.go`, `tray_objc_darwin.m` | "Update Available…" item | VERIFIED | `setTrayUpdateItem`, `onTrayUpdate` -> `offerUpdate` |
| `frontend/src/components/UpdateBanner.tsx` | Banner states | VERIFIED | rendered at App.tsx:1038 |
| `frontend/src/wailsjs/go/main/App.{d.ts,js}` | GetUpdateStatus, DownloadUpdate stubs | VERIFIED | match the Go bindings and Status JSON tags |
| `.github/workflows/release.yml` | DMG identifier, notarize, staple, self-verify | VERIFIED | lines 220-255 |
| `updates_vmpreview.go`, `cmd/fakegh/main.go` | VM-only seam and fake GitHub | VERIFIED | build-tag gated; vet clean with and without the tag |
| `TESTING.md` | UPD rows, M-92, M-93 | VERIFIED | Section 4 rows 470-482, M-92 at 1108, M-93 at 1123; traceability gate OK |

### Key Link Verification

| From | To | Via | Status |
|------|----|-----|--------|
| `App.startup` | `runUpdateLoop` | `a.startUpdatePoller(ctx)` (release builds only) | WIRED |
| `Service.Emit` | frontend banner | `onUpdateStatus` -> `emitEvent("update:status")` -> `EventsOn` in App.tsx | WIRED |
| `autoCheck` | native notification | `notifyUpdateOnce` -> `sendNotificationFunc` (= `sendNotification`) | WIRED |
| Banner Download / Try Again | `Service.Download` | `DownloadUpdate()` binding | WIRED |
| Help menu | manual check | `checkForUpdatesCallback` -> `checkForUpdatesManually` | WIRED |
| Tray item | offer dialog | `updateClicked:` -> `onTrayUpdate` -> `offerUpdate` | WIRED |
| `Service.Download` | real tools | `Run: updater.ExecRunner` (absolute paths, no shell) | WIRED |
| release.yml DMG identifier | client `Verify` identifier step | `portty-${GITHUB_REF_NAME}-darwin-universal` == `DMGAssetName(v)` minus `.dmg` | WIRED |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| UpdateBanner | `updateStatus` | `GetUpdateStatus()` + `update:status` events <- `Service.st` <- `Checker.Latest` (GitHub REST) | Yes | FLOWING |
| Tray item | title | `onUpdateStatus` from the same Status | Yes | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Build and vet | `go build ./... && go vet ./... && go vet -tags vmpreview .` (GOTOOLCHAIN=go1.26.11) | clean | PASS |
| Go suite | `go test -race -short -count=1 -skip '^(TestAPIAppearance_GetPatch\|TestAPITerminalFont_GetPatch)$' ./...` | all packages ok except internal/relay `TestHub_TwoClientsFanOut` (`got payload "\x01hello world"`) on the first run; passed 3/3 alone and on a full `./internal/relay` rerun. Phase 6 did not touch internal/relay (empty `git diff 39f4341c HEAD -- internal/relay`) | PASS (unlisted flake, pre-existing) |
| SC1/SC2/SC4 app tests | `go test -race -run '^(TestUpdatePoller_FirstCheckAfterDelay\|TestUpdatePoller_DailyCadenceAndNoRepeat\|TestManualUpdateCheck_UpToDate\|TestManualUpdateCheck_AvailableDownload\|TestManualUpdateCheck_Error\|TestUpdatePoller_FailureRetriedHourlyAndQuiet\|TestUpdatePoller_NotReadyRetriedHourly)$' .` | 7 PASS | PASS |
| SC3/SC4 updater tests | `go test -race -run '^(TestService_CheckQuietOutcomes\|TestService_DownloadFailures\|TestService_DownloadReveals\|TestVerifyFailures\|TestDownloadDigestMismatch\|TestLatestNotFoundAndErrors\|TestLatestNotOffered\|TestFlow_CheckDownloadVerifyReveal\|TestReleaseWorkflow_NotarizesAndStaplesDMG)$' ./internal/updater` | 9 PASS | PASS |
| Frontend | `cd frontend && pnpm vitest run` | 88 files, 1199 tests passed (no flake reruns) | PASS |
| Brand gate | `bash tests/check-brand.sh` | OK: brand gate clean | PASS |
| Traceability | `bash tests/check-traceability-paths.sh` | OK: 138 traceability paths exist | PASS |
| Real tools on real DMGs | `TestRealDMG` | SKIP here (no UPDATE_REAL_DMG input on hand); 06-REVIEW-FIX.md records a post-WR-01 host pass with GIFsmith refused at `identifier` | SKIP (human item M-93) |

### Probe Execution

None declared; no `scripts/*/tests/probe-*.sh` exist. Step 7c not applicable.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| UPD-01 | 06-01, 06-02, 06-03, 06-04 | Launch and periodic check of the public Releases API; notice plus native notification | SATISFIED (delivery on a notarized build: human) | Truth 1. Precondition: scottkw/portty must be public, or the API returns 404 and Portty stays quietly "up to date" (M-93 precondition) |
| UPD-02 | 06-02, 06-04 | "Check for Updates…" reports up to date / version X / readable error | SATISFIED | Truth 2 |
| UPD-03 | 06-01, 06-02, 06-03, 06-04 | One click downloads, verifies digest and Team signature, reveals in Finder; GIFsmith port | SATISFIED (real notarized Portty DMG: human) | Truths 3 and 5 |
| UPD-04 | 06-01, 06-02, 06-04 | Quiet on no release, 404, drafts, prereleases, network failures, rate limits; checks keep running | SATISFIED | Truth 4 |

No orphaned requirements: REQUIREMENTS.md maps only UPD-01..UPD-04 to Phase 6, and every plan claims a subset of them.

### Decision Coverage

The gate (`check.decision-coverage-verify`) found no trackable decisions in CONTEXT.md (skipped). Checked by hand, every user decision is honoured: Team ID S2K7P43927 hardcoded; the DMG notarized and stapled in release.yml with the full client verify; 10 s then 24 h cadence with no toggle; native alerts with the agreed copy. The autonomous decisions are honoured too: ETag; 403/429 quiet; throttle file dropped; notify once per version in the config dir; reveal with `open -R` keeping quarantine; the `~/Library/Caches/app.portty.Portty/updates` folder; menu label "Check for Updates…".

### Test Quality Audit

| Test File | Linked Req | Active | Skipped | Circular | Assertion Level | Verdict |
|-----------|-----------|--------|---------|----------|-----------------|---------|
| internal/updater/update_test.go | UPD-01, UPD-04 | all | 0 | No | Value | OK |
| internal/updater/install_test.go | UPD-03 | all | 0 | No | Value (failure step names) | OK |
| internal/updater/service_test.go | UPD-01, UPD-03, UPD-04 | all | 0 | No | Behavioral (state sequences) | OK |
| internal/updater/real_darwin_test.go | UPD-03 | env-gated | skips without inputs | No | Behavioral (real tools) | OK; other tests also cover UPD-03, so the env skip is not the only proof |
| internal/updater/release_workflow_test.go | UPD-03 | all | 0 | No | Value (workflow text) | OK |
| app_update_test.go | UPD-01, UPD-02, UPD-04 | all | 0 | No | Behavioral (request counts, notifications, alert texts) | OK |
| UpdateBanner.test.tsx, App.update.test.tsx | UPD-01, UPD-03 | all | 0 | No | Behavioral | OK |

**Disabled tests on requirements:** 0. **Circular patterns detected:** 0. **Insufficient assertions:** 0.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (phase files) | - | TBD/FIXME/XXX/TODO/placeholder | none found | - |
| tray_objc_darwin.m | ~84-95 | no `didReceiveNotificationResponse` handler (review IN-05) | Info | Clicking the update notification may not show the window; human item |
| app_update_test.go | - | `TestUpdatePoller_DailyCadenceAndNoRepeat` fails under `go test -tags vmpreview .` (06-04 deviation 2) | Info | No gate runs tests with that tag; vet with the tag is clean |

### Human Verification Required

#### 1. First release CI run (M-93 step 1)
**Test:** Push the first `v*` tag with the repo public and the release secrets and environment in place.
**Expected:** "Create and sign DMG" (with identifier), "Notarize and staple DMG" and "Verify DMG like the in-app updater" all pass; the release carries the DMG with a sha256 digest.
**Why human:** Needs Apple and GitHub credentials. These steps have never run in CI.

#### 2. Real tools on the real Portty DMG (M-93 step 2)
**Test:** `UPDATE_REAL_DMG=<dmg> go test -run TestRealDMG ./internal/updater`, and TestReleaseJSON with the saved release JSON.
**Expected:** Both pass, not skipped.
**Why human:** No notarized Portty DMG exists yet. After WR-01, GIFsmith's DMG can no longer stand in for the accept path.

#### 3. End-to-end update on the real Macs (M-93 step 3)
**Test:** With an older release installed and a newer one published, launch Portty, then use Check for Updates… and Download.
**Expected:** Banner after about 10 s; one delivered notification; the alert offers the version; the DMG is verified, revealed in Finder, opens and installs.
**Why human:** Notification delivery was not observable on the ad-hoc build, and the U-05 reveal evidence predates WR-01. This needs two releases (an installed older one and a newer one).

#### 4. Notification click (review IN-05)
**Test:** Hide the window (quit to menu bar), then click the "Version X is available." notification.
**Expected:** Decide whether activating the app without showing the window is acceptable, or add a response handler.
**Why human:** Needs a build allowed to post notifications. It is also a product decision.

#### 5. U-06 noasset re-run after WR-04 (Claude can run it in the Tart VM)
**Test:** The fake GitHub serves newer 9.9.9 with no DMG asset.
**Expected:** Automatic checks stay quiet and retry on the next tick. The manual check says "Couldn't check for updates." / "Version 9.9.9 isn't ready to download yet. Try again later."
**Why human:** The recorded PASS predates the WR-04 text change. The native alert needs a GUI session.

### Gaps Summary

No code gaps. All four ROADMAP success criteria and UPD-01..UPD-04 are implemented, wired end to end (poller -> Service -> update:status -> banner/tray/notification; Help -> native alerts; Download -> digest -> codesign/Team/identifier/spctl -> Finder), and covered by passing tests and the VM run. What remains needs the first real signed and notarized release (M-93) plus the IN-05 decision, so the status is human_needed, not passed.

---

_Verified: 2026-11-18T19:55:51Z_
_Verifier: Claude (gsd-verifier)_
