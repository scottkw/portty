---
phase: 18-self-installing-updates
plan: 01
subsystem: updater
tags: [go, macos, renamex_np, hdiutil, ditto, codesign, self-update, tracer]
status: complete

requires:
  - phase: 17
    provides: none (Phase 18 is independent of Phase 17)
provides:
  - "Stage pipeline (mount, VerifyApp, ditto copy, quarantine strip, detach) and Swap via renamex_np RENAME_SWAP"
  - "Job and Result files with strict readers, InstallerEnv seams and RunInstaller"
  - "Hidden `portty install-update JOB` subcommand run from a copy of the binary"
  - "Service states installing and relaunching, Bundle and StartInstaller fields, reveal fallback with a message"
  - "SafeRunner that makes test binaries refuse hdiutil, open and ditto"
affects: [18-02, 18-03, 18-04, 18-05, 18-06, 18-07, 18-08, 18-09]

actuals:
  tokens: 16100
  tasks: 3
  commits: 3
plan_head_before: 8ec50e45b4568af9785b499a2454c384becc073d
plan_head_after: 51f743f922994c89e9f838e07a59f57c1c6dcd2e

tech-stack:
  added: []
  patterns:
    - "Stage everything fallible in the GUI, hand off to a detached copy of the binary through a 0600 job file, report through a result file"
    - "Every effect of the installer behind an InstallerEnv function field so tests run the whole path with fakes and a fake clock"
    - "Test binaries never run real hdiutil, open or ditto: SafeRunner, spawnInstaller and openApp all refuse under testing.Testing()"

key-files:
  created:
    - internal/updater/appinstall.go
    - internal/updater/installer.go
    - internal/updater/selfinstall_test.go
    - cmd_install_update.go
    - cmd_install_update_test.go
    - app_install.go
  modified:
    - internal/updater/install.go
    - internal/updater/service.go
    - app_update.go
    - main.go

key-decisions:
  - "ReadJob and TakeResult share one reader (readOwnFile): O_NOFOLLOW, O_NONBLOCK, regular file, no mode bits beyond 0600, owner wantUID, 1 byte to 64 KiB; errors carry a reason, never a path"
  - "Detach on a folder that is already gone is success; a busy image gets three tries a second apart then one -force; the folder is removed best-effort even when detach fails"
  - "ClearQuarantine fails closed: a failing xattr -lr listing counts as the flag still being there"
  - "TakeResult does not reject unknown JSON fields (ReadJob does): an older GUI after a rollback must still read a result written by a newer installer copy"

patterns-established:
  - "si-prefixed test helpers (siRelease, siFix, siBundle, siJobFix) so later plans in this phase add rows and fields without editing the shared fake"

requirements-completed: [UPD-05, UPD-06, UPD-09]

coverage:
  - id: D1
    description: "Update click stages the verified app beside the installed bundle, hands off, swaps atomically, relaunches, waits for the new daemon version and deletes the DMG (in-process, real swaps on temp dirs)"
    requirement: "UPD-05"
    verification:
      - kind: unit
        ref: "internal/updater/selfinstall_test.go#TestSelfInstall_UpdateHappyPath"
        status: pass
  - id: D2
    description: "Every failure before the installer takes over reveals the DMG with a message, writes the quarantine back and leaves no staging folder or mount; an app that failed its check is never un-quarantined"
    requirement: "UPD-09"
    verification:
      - kind: unit
        ref: "internal/updater/selfinstall_test.go#TestSelfInstall_StageFailureReveals"
        status: pass
  - id: D3
    description: "Repeated clicks and repeated ejects change nothing; a missing DMG or a health miss is recorded correctly"
    requirement: "UPD-06"
    verification:
      - kind: unit
        ref: "internal/updater/selfinstall_test.go#TestSelfInstall_SecondDownloadIsNoOp"
        status: pass
      - kind: unit
        ref: "internal/updater/selfinstall_test.go#TestStage_DetachIdempotent"
        status: pass
  - id: D4
    description: "Job and result files are accepted only when Portty wrote them for a real staging sibling"
    requirement: "UPD-05"
    verification:
      - kind: unit
        ref: "internal/updater/selfinstall_test.go#TestJob_RoundTripAndRejects"
        status: pass
      - kind: unit
        ref: "internal/updater/selfinstall_test.go#TestResult_TakeOnceAndRejects"
        status: pass
  - id: D5
    description: "install-update is hidden from usage, exits 1 on a bad job without a marker, and test binaries refuse to spawn an installer or open an app"
    requirement: "UPD-09"
    verification:
      - kind: unit
        ref: "cmd_install_update_test.go#TestInstallUpdate_BadJobExits1"
        status: pass
      - kind: other
        ref: "built binary: install-update MISSING exits 1 in well under 5 s (checked by hand, not in the Go suite)"
        status: pass

duration: one session
completed: 2026-11-25
---

# Phase 18 Plan 01: Self-installing update tracer Summary

**Clicking Update now runs a full in-process path: stage the verified app beside the installed bundle, hand off to a detached copy of the binary, swap with renamex_np RENAME_SWAP, relaunch with `open`, wait for the new daemon version, delete the DMG; every pre-handoff failure falls back to the old Reveal flow with a message.**

## Accomplishments

- Task 1 (tracer): `Stage`, `Swap`, `Job`/`Result`, `RunInstaller`, hidden `install-update`, `app_install.go` hand-off, `Service.install`; `TestSelfInstall_UpdateHappyPath` proves target holds 9.9.9, the old 1.0.0 bundle is kept aside in its staging folder, the cache DMG and mount folder are gone and the result says updated.
- Task 2: `ClearQuarantine` proves the strip with `xattr -lr`; `Detach` is idempotent; `busy` covers installing and relaunching; five new tests for failure, start failure, reveal failure, second click and detach.
- Task 3: strict `ReadJob` (T-18-02 full list) and `TakeResult`; tests for DMG already gone, health miss, hidden usage, launch mode, bad job, spawn and open refusals, release bundle layout.
- Existing reveal-flow tests (`TestService_DownloadReveals`, `TestFlow_CheckDownloadVerifyReveal`, `TestVerify`, `TestDownloadUpdate_BindingRunsDownload`) pass unchanged.

## Task Commits

1. Task 1 tracer: `46a6bdb1`
2. Task 2 staging seam: `d43d1579`
3. Task 3 installer seam: `51f743f9`

Tracer feedback gate: auto mode off, `human_verify_mode` end-of-phase, verify is automated only, so it was re-run end to end after Task 1 (go vet, gofmt, golangci-lint, race tests, built binary exits 1 on a missing job, release guards, brand gate) and passed before Tasks 2 and 3.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing critical functionality] Reopen the old app when the installer fails after the old GUI already quit at the re-verify step**
- **Found during:** Task 1 (RunInstaller step b)
- **Issue:** The plan reopens the old app only for a swap failure. A re-verify failure also happens after the old GUI exited, which would leave the user with no running Portty.
- **Fix:** `RunInstaller` calls `env.Open(j.Target)` before it reports `reverify`, same as for `swap`. The old bundle is still in place, so this is the same safe relaunch.
- **Files modified:** internal/updater/installer.go
- **Commit:** 46a6bdb1

**2. [Test-design adjustment] siRelease fields are struct fields, and `open -R` is allowed**
- The plan calls `fail` and `failOut` "package-level fields" and says `/usr/bin/open` is an error. They are fields of the `siRelease` struct (so parallel tests do not share them), and `open -R` (the Reveal fallback, which the failure tests must observe) answers ok unless `fail` is `open`; any other `open` through the runner is an error, which still guards that the installer never opens through the runner. `fail: "detach"` models an hdiutil that errors although the volume is gone, so the tests can check that no `mnt-*` folder remains.
- **Files modified:** internal/updater/selfinstall_test.go

No other deviations.

## Issues Encountered / Deferred

- `TestNoFakeWhoIsInRelease` in the root package fails in this worktree because `frontend/dist` is not built here (`pattern all:frontend/dist: no matching files found`). It is unrelated to this plan and was not touched; every test the plan names passes. Logged here instead of fixed.
- `checkForUpdatesManually` and `offerUpdate` in `app_update.go` do not yet know the installing and relaunching states (a manual check during an install would say "up to date"). The plan assigns that wording to 18-04, so it was left alone on purpose.
- TESTING.md Sections 2, 4 and 5 are not updated here: the phase plan assigns them to 18-09. `bash tests/check-traceability-paths.sh` therefore not run (TESTING.md untouched).

## Known Stubs

None. `InstallerEnv.Signal`, `GUIPids` and `Trash` have no caller until 18-03 and 18-05 and are declared on purpose (see the plan's flagged decisions); `OutcomeRolledBack` is accepted by `TakeResult` but produced from 18-03.

## Threat Flags

None beyond the plan's register. The installer binary copy and `update.log` live in the 0700 config dir; no network listener or new GitHub contact was added.

## Not verified here (needs the VM, 18-08)

Real hdiutil attach of a quarantined DMG (A3/P1), whether macOS lets the copied installer binary run (A1/P2d), App Management on the swap (P2/P8), a real relaunch with `open`, and the health timing against a real takeover. Nothing in this plan touched the host's /Applications, ~/Downloads, tailscaled or any Tart VM.

## Self-Check: PASSED

- Files found: internal/updater/appinstall.go, installer.go, selfinstall_test.go, cmd_install_update.go, cmd_install_update_test.go, app_install.go
- Commits 46a6bdb1, d43d1579, 51f743f9 are ancestors of HEAD
