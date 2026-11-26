---
phase: 18-self-installing-updates
plan: 02
subsystem: updater
status: complete
tags: [updater, self-install, UPD-09, UPD-07, UPD-05, gatekeeper, assets]
requires:
  - "18-01: Stage, VerifyApp, CopyApp, revealFallback, Service.install"
  - "18-03: ShowResult, skipVersion, skippedVersion (outcome.go)"
  - "18-08: P7 (app identifier) and P8 (staging layout) rows in 18-VM-CHECKS.md"
  - "18-10: copyBundle runs VerifyApp on the whole-bundle installer copy"
provides:
  - "Preflight, CheckSpace, AppSize, CheckStagedApp, SmokeTimeout, AppIdentifier (appinstall.go)"
  - "Sentinels ErrNoWrite, ErrAppManagement, ErrNoSpace, ErrLocation, ErrSafety, ErrMount, ErrCopy, ErrDetach (each wraps ErrInstall)"
  - "installFailureText and its table; revealFallback that does not reveal a vanished DMG"
  - "ManualCheck(ctx); Check honours skippedVersion on automatic checks"
  - "DMGAssetNames: universal, then portty-vX.Y.Z-darwin-arm64.dmg"
affects: [18-05, 18-06, 18-07, 18-09, phase-23]
tech-stack:
  added: []
  patterns:
    - "Install failure causes are sentinel errors wrapped with the step name; one table maps cause to banner text"
    - "A context value (ManualCheck) marks a user-requested check without changing Check's signature"
key-files:
  created:
    - internal/updater/appinstall_test.go
  modified:
    - internal/updater/appinstall.go
    - internal/updater/service.go
    - internal/updater/install.go
    - internal/updater/update.go
    - internal/updater/service_test.go
    - internal/updater/update_test.go
    - internal/updater/install_test.go
    - app_update.go
    - app_update_test.go
decisions:
  - "AppIdentifier = app.portty.Portty (P7 CONFIRMED); P8 CONFIRMED, so staging keeps .Portty-update-*/Portty.app"
  - "A manual check (ManualCheck) skips both keep-revealed cases, so Help > Check for Updates… offers a failed version for a retry even from the failure banner"
  - "The keep-revealed case compares versions, not whole releases, and replaces ShowResult's partial s.rel with the full release"
  - "LSMinimumSystemVersion: only plutil's 'No value at that key path' counts as a missing key; any other plutil error fails the safety check"
  - "A statfs failure in CheckSpace is a plain install failure, not the disk-space text"
  - "hasLine ignores whitespace around each line (encoding edge of UPD-09); exact-line matching is unchanged"
metrics:
  duration: "~16 min"
  completed: 2026-11-25
actuals:
  tokens: 14300
  tasks: 2
  commits: 2
plan_head_before: a9382cf430a8eaa8fe77f623499ee164c5635ca0
plan_head_after: 12c05a4b5f70bbde252921114ca35c4ddcf7d26a
requirements: [UPD-09, UPD-05, UPD-07]
---

# Phase 18 Plan 02: Install hardening, failed-version skip and the arm64 asset name Summary

Every install failure now reveals the DMG with its own sentence and never prompts for a password. The staged app has to pass the same trust chain a manual install would face (Team, identifier `app.portty.Portty`, notarized exec assessment, exact version, minimum macOS, and a 10 s `--version` smoke run) before it is swapped in or launched. An automatic check no longer offers a version whose install failed. The updater also accepts the Apple Silicon-only DMG name the user confirmed.

## Confirmed asset name (for 18-09 and Phase 23)

**`portty-vX.Y.Z-darwin-arm64.dmg`**, confirmed by the user on **2026-11-25** (user answer "portty-vX.Y.Z-darwin-arm64.dmg (Recommended)", 18-CONTEXT [Assets]). `DMGAssetNames(v)` returns `[portty-vX.Y.Z-darwin-universal.dmg, portty-vX.Y.Z-darwin-arm64.dmg]`, and the universal name wins when both are published. Phase 23 must sign the arm64 DMG with `--identifier portty-vX.Y.Z-darwin-arm64`, because Verify requires the identifier to equal the asset name without `.dmg`. This is a one-way door: installed builds look for exactly these two names.

## 18-03 VM finding: fixed

In R2-A, R2-B and R2-C, the update poller's first Check came about 10 s after a rollback or failed install. It offered the failed version again and replaced the rolled-back banner. The fix in Check has three parts:
- An automatic check that sees the same version as a revealed status keeps that status. Versions are compared, not whole releases, because after a relaunch ShowResult's `s.rel` only knows the version and the DMG name. So the failure banner (revealed with an Error) survives the check.
- An automatic check that sees the version `skippedVersion()` returns never offers it. If prev already shows that version (revealed or available), prev is kept. Otherwise the state goes to `current` with an empty Latest, so the banner, the tray item and the notification all stay quiet. This also holds after a restart and when the DMG is gone. A newer version is offered as usual.
- While the skip record is unreadable, no automatic check offers any version. Each such check logs `update: skip record unreadable, automatic offer suppressed: ERR`, with no path. `Help > Check for Updates…` (now `Check(updater.ManualCheck(...))`) still offers the latest version.

## Tasks

| Task | Name | Commit | Files |
| ---- | ---- | ------ | ----- |
| 1 | Preflight, disk space, full app trust chain, one text per failure, failed-version rule in Check | 59e088fb | appinstall.go, service.go, install.go (hasLine), appinstall_test.go, service_test.go, app_update.go, app_update_test.go |
| 2 | DMGAssetNames in Checker.parse and Clean | 12c05a4b | update.go, install.go, update_test.go, install_test.go |

## Failure texts (installFailureText)

The banner text is the sentence below plus " The download is shown in Finder." when the DMG was revealed, or " Try again later." when the DMG was gone at fallback time (state available, no Reveal):

| Cause | Sentence |
|---|---|
| ErrNoWrite | Portty doesn't have permission to replace itself from this account. |
| ErrAppManagement | macOS didn't allow Portty to replace itself. |
| ErrMount | Portty couldn't open the download. |
| ErrCopy | Portty couldn't copy the new version. |
| ErrNoSpace | There isn't enough free disk space to install the update. |
| ErrSafety | The new version didn't pass the safety check, so it wasn't installed. |
| ErrLocation | Portty can't update itself from where it's running. Move Portty to the Applications folder first. |
| ErrDetach | Portty couldn't eject the download. |
| installer start | Portty couldn't start the installer. |
| anything else | Portty couldn't install the update. |

## Verification

All Go commands ran with `GOTOOLCHAIN=go1.26.11`.

- `gofmt -l internal/updater app_update.go`: empty
- `go vet ./internal/updater/ .`: clean
- `golangci-lint run --new-from-rev=c714c8ef ./...`: 0 issues
- `go test -list` for the Task 1 sets: 6 and 4. For the Task 2 set: 4.
- `go test -race -count=1 ./internal/updater/`: ok. TestSelfInstall_UpdateHappyPath passes, and selfinstall_test.go was not edited.
- `go test -race -count=1 . -run 'TestManualUpdateCheck|TestUpdatePoller|TestTrayUpdate'`: ok. Also with `TestConsumeInstallResult|Install` added: ok.
- `administrator privileges` count in appinstall.go, installer.go and service.go: 0
- `bash tests/check-brand.sh`: OK
- The non-comment `darwin-arm64.dmg` line count in update.go is 1.
- Mutation checks: with the skip case disabled, TestService_CheckSkipsFailedVersion and TestService_CheckUnreadableSkipSuppressesOffer fail. With the identifier check disabled, TestVerifyApp fails. Both mutations were reverted.
- The full `go test -race .` has one failure, TestNoFakeWhoIsInRelease. It needs a built `frontend/dist` (`pattern all:frontend/dist: no matching files found`), which this worktree does not have. This is an environment gap and is not caused by this plan.
- No test runs hdiutil, open or ditto, or touches host files. Every new test uses fake runners and `t.TempDir()`. No VM, tailnet or host GUI was used.

## TDD note

The tests were written before the implementation. The RED run failed to compile because the symbols were not defined yet. Under tdd.md #3770 that is INVALID_RED, so the targeted mutation checks above stand in as evidence that the new tests catch regressions. There is no separate `test(18-02)` commit, because the orchestrator asked for one commit per task.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] app_update_test.go holdAt fake answered the app's checks like a DMG**
- **Found during:** Task 1 verify (`TestManualUpdateCheck_WhileUpdating/relaunching`, `TestTrayUpdate_OfferUpdate/relaunching`)
- **Issue:** the harness's runner answered every `codesign -dvv` with the DMG identifier and had no plutil, sw_vers or `--version` answers. The new staged-app checks therefore (correctly) stopped the install at the safety check, before it reached relaunching.
- **Fix:** added cases for the app's `-dvv` (`Identifier=app.portty.Portty`), plutil version and minimum OS, sw_vers and the smoke run. The file is outside files_modified and is not one of 18-05's files.
- **Commit:** 59e088fb

**2. [Rule 1 - Plan contradiction] TestLatestNotInstallable and TestClean could not pass unchanged**
- **Found during:** Task 2
- **Issue:** the acceptance criterion "TestLatestNotInstallable and TestClean pass unchanged" conflicts with the user-confirmed [Assets] decision. TestLatestNotInstallable's "missing" case used the arm64 name as an unknown asset, and TestClean expected an arm64 DMG to be left in the cache.
- **Fix:** the "missing" case now uses `portty-v1.1.0-darwin-x86_64.dmg`, and a case for an older version's arm64 asset was added. TestClean now expects the arm64 DMG and its `.part` to be removed, while an x86_64 file and folders with either name stay.
- **Commit:** 12c05a4b

**3. [Rule 2 - Correctness] Manual checks skip the keep-revealed case; the partial s.rel is replaced**
- **Issue:** if the keep-revealed case only compared versions, `Help > Check for Updates…` from a failure banner would have kept "revealed", and checkForUpdatesManually would then report "Portty is up to date." That contradicts the must-have that a manual check still offers the failed version.
- **Fix:** both keep cases apply only to automatic checks. When an automatic check keeps the revealed status, it also replaces ShowResult's partial `s.rel` (version and DMG name only) with the full release, so Update can still download if the kept DMG has since been deleted.
- **Commit:** 59e088fb

**4. [Rule 2] hasLine trims each line**
- Small change in install.go (Task 1 scope) so that "a check value that differs only by surrounding whitespace still passes" for VerifyApp. A line that merely contains the value still fails.

**5. [Rule 2] Test seam `mkdirTemp`**
- `var mkdirTemp = os.MkdirTemp` lets TestCopyApp return EACCES and EPERM. A unit test cannot produce those errors for real.

## Deferred / for later plans

- **18-09 (TESTING.md):** new test file `internal/updater/appinstall_test.go`, with TestPreflight, TestCheckSpace, TestFreeBytesAndAppSize, TestVerifyApp, TestCheckStagedApp, TestCopyApp, TestInstallFailureText, TestService_InstallFailuresReveal, TestService_SafetyFailureNeverLaunches, TestService_FallbackWithoutDMG and TestService_PreflightBeforeStaging. New tests in service_test.go: TestService_CheckKeepsRevealedSameVersion, TestService_CheckSkipsFailedVersion, TestService_ManualCheckOffersSkipped and TestService_CheckUnreadableSkipSuppressesOffer. TestService_ErrorTextsHaveNoURLOrPath now covers all nine install failure modes. New tests in update_test.go: TestDMGAssetNames, TestLatestAcceptsArm64Name, TestLatestPrefersFirstName and TestLatestInvalidPreferredAsset. The TestLatestNotInstallable "missing" wording in Section 4 should say an unknown (x86_64) name. Record the confirmed arm64 name and its date.
- **18-07 (VM failure rows):** this plan adds the new spctl exec, plutil, sw_vers and `Portty --version` calls on the staged app. A real Developer ID run in the VM should confirm that `spctl -a -t exec -vv` on the staged copy (quarantine stripped) still answers `accepted` / `source=Notarized Developer ID`.

## Known Stubs

None.

## Self-Check: PASSED

- FOUND: internal/updater/appinstall_test.go, internal/updater/appinstall.go (`func Preflight(`, `func CheckStagedApp`), internal/updater/update.go (`func DMGAssetNames(`)
- FOUND: 59e088fb, 12c05a4b (ancestors of HEAD)
- `grep -c 'Preflight(' internal/updater/service.go` = 1, `grep -c 'skippedVersion()' internal/updater/service.go` = 1, `grep -c 'ManualCheck(' app_update.go` = 1, `grep -c DMGAssetNames internal/updater/install.go` = 2
