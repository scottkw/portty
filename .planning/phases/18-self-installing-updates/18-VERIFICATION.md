---
phase: 18-self-installing-updates
verified: 2026-11-26T10:45:00Z
status: human_needed
score: 14/15 must-haves verified
covered_files: [".github/workflows/release.yml", ".planning/phases/18-self-installing-updates/18-01-PLAN.md", ".planning/phases/18-self-installing-updates/18-01-SUMMARY.md", ".planning/phases/18-self-installing-updates/18-02-PLAN.md", ".planning/phases/18-self-installing-updates/18-02-SUMMARY.md", ".planning/phases/18-self-installing-updates/18-03-PLAN.md", ".planning/phases/18-self-installing-updates/18-03-SUMMARY.md", ".planning/phases/18-self-installing-updates/18-04-PLAN.md", ".planning/phases/18-self-installing-updates/18-04-SUMMARY.md", ".planning/phases/18-self-installing-updates/18-05-PLAN.md", ".planning/phases/18-self-installing-updates/18-05-SUMMARY.md", ".planning/phases/18-self-installing-updates/18-06-PLAN.md", ".planning/phases/18-self-installing-updates/18-06-SUMMARY.md", ".planning/phases/18-self-installing-updates/18-07-PLAN.md", ".planning/phases/18-self-installing-updates/18-07-SUMMARY.md", ".planning/phases/18-self-installing-updates/18-08-PLAN.md", ".planning/phases/18-self-installing-updates/18-08-SUMMARY.md", ".planning/phases/18-self-installing-updates/18-09-PLAN.md", ".planning/phases/18-self-installing-updates/18-09-SUMMARY.md", ".planning/phases/18-self-installing-updates/18-10-PLAN.md", ".planning/phases/18-self-installing-updates/18-10-SUMMARY.md", "TESTING.md", "app.go", "app_install.go", "app_install_test.go", "app_mover.go", "app_mover_test.go", "app_update.go", "cmd_install_update.go", "cmd_install_update_test.go", "frontend/src/App.tsx", "frontend/src/components/UpdateBanner.tsx", "internal/daemon/share_vmpreview.go", "internal/updater/appinstall.go", "internal/updater/appinstall_test.go", "internal/updater/install.go", "internal/updater/installer.go", "internal/updater/installer_test.go", "internal/updater/mover.go", "internal/updater/mover_test.go", "internal/updater/outcome.go", "internal/updater/outcome_test.go", "internal/updater/selfinstall_test.go", "internal/updater/service.go", "internal/updater/update.go", "main.go", "trash_darwin.go", "trash_darwin.m", "updates_vmpreview.go"]
covered_digest: "v4:sha256:5c940b7adf4e0ee0959a9e046e6f95dd9890d12036774795afffffa12856d137"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: human_needed
  previous_score: 14/15
  gaps_closed: []
  gaps_remaining: []
  regressions: []
advisory:
  - finding: "RC-WR04-stall: both sessions' output stalled ~17 min (07:11:25-07:28:24) after a keep-daemon rollback"
    category: other
    reason: "Resolved after the first verification as a guest clock step (timed moved the guest clock forward 17 min 03 s at 07:10:02; evidence/RC-WR04-clockstep.log, .planning/debug/resolved/rc-wr04-stall.md). Not a Phase 18 gap. A same-version and keep-daemon soak remains a sensible follow-up under the rock-solid priority."
    evidence_status: "resolved as a VM clock step; soak not run"
  - finding: "update-result.json with outcome opened (and, by timing, updated) stays on disk until the next GUI start"
    category: other
    reason: "The installer writes it after the GUI it launched already ran consumeInstallResult. TakeResult consumes it at the next start; opened/updated return before ShowResult, the notification or the skip record. Cosmetic."
    evidence_status: "code read app_install.go:353-394"
  - finding: "IN-08 (left open by the review fix): any failed result writes the skip record, including transient causes (open, quit-timeout, swap), so an unattended Mac stays on the old version until a newer release or a manual check"
    category: other
    reason: "Not a Phase 18 success criterion; relevant to the rock-solid priority. Candidate follow-up."
    evidence_status: "code read outcome.go / service.go; 18-REVIEW.md IN-08"
  - finding: "F6 finding: a SIGKILLed installer leaves the verified DMG in the update cache until the next Download's Clean"
    category: other
    reason: "Recorded as deferred in 18-VM-CHECKS.md F6; the install itself recovered on the next launch. Not part of a successful install."
    evidence_status: "F6 row, evidence/F6.state cache_dmgs=1"
  - finding: "The late-failure notice (quick 261126-dfn) is posted with osascript, so the banner carries the Script Editor icon and a click opens Script Editor"
    category: other
    reason: "Chosen deliberately: a VM probe showed UNUserNotificationCenter is refused or only prompts for an installer that LaunchServices did not start. The text and the Portty title still say what happened. Cosmetic."
    evidence_status: "install.go:198-207; 261126-dfn-SUMMARY.md probe; end-to-end VM row QT-WR02 is PENDING (VM busy with the soak)"
  - finding: "A mover keepDaemon result is put back after showEarlierResult, so the installed copy's next start may show the same failure a second time"
    category: other
    reason: "Intentional trade (WR-04 keepDaemon must survive); a repeated sentence, never a lost keepDaemon."
    evidence_status: "app_mover.go:99-101; 261126-dfn-SUMMARY.md deviation 1"
  - finding: "The sweep deletes (not Trashes) a stale staging copy only when its version is readable and strictly newer than the real Portty.app beside it; the one reused daemon client in the health poll has no unit test"
    category: other
    reason: "Both from quick 261126-e29. The sweep rule is covered by TestSweep_FailedNewCopyRemoved and TestSweep_StagingDirs and fails toward the Trash on every unprovable case; the reused client can only report the old version, which delays health and never fakes it (cmd_install_update.go:60-65)."
    evidence_status: "code read app_install.go:259-339; tests pass"
coincidental_reliance_items:
  - truth: "An update counts as done only when the new GUI is up and its daemon answers at the new version, steadily for HealthStable"
    reason: undeclared-precondition
    harden: "ParseGUIPids requires ps argv[0] to equal EvalSymlinks(target)/Contents/MacOS/Portty byte for byte (installer.go:365). Nothing guarantees LaunchServices spells argv[0] that way (case on a case-insensitive volume, /System/Volumes/Data, a symlinked home). The VM rows show the common /Applications case holds; compare with os.SameFile or proc_pidpath instead (18-REVIEW.md IN-07)."
human_verification:
  - test: "M-169 part 1 and part 2 on the office Mac: part 1 opens the notarized, browser-downloaded release DMG over an installed v1.2.x (Replace); part 2, at the next release, clicks Update in the banner with two ticking sessions and a share link open from the home Mac over the tailnet"
    expected: "Sessions keep shell pids and history; the share link opens from the home Mac over the real tailnet before and after; DMG ejected and trashed (part 1) or deleted from the cache (part 2); old app in the Trash; no Gatekeeper or App Management dialog on the relaunch"
    why_human: "ROADMAP SC1's office-Mac half. Needs two real signed and notarized releases, the office Mac's Gatekeeper, App Management and tailnet. The VM uses ad-hoc builds, a loopback share link and has Gatekeeper assessments disabled."
  - test: "Real Gatekeeper enforcement on a quarantined, notarized release DMG (the half of R4-f that row R4-f-gatekeeper left BLOCKED), and the relaunch after an update"
    expected: "Only the system's first-launch confirmation for the downloaded app appears; no refusal, no Open Anyway, and no Gatekeeper dialog when the installer relaunches the installed copy"
    why_human: "spctl --status in portty-test reads 'assessments disabled'; R3's spctl answers carry override=security disabled, so no VM row is an enforcement result. Covered on a real Mac by M-169 part 1."
  - test: "M-171: after an update, select the old Portty.app in the Trash in Finder, record whether File > Put Back is offered and where it restores; drag the old app out and open it"
    expected: "The dragged-out copy opens as the older version; Put Back's state recorded either way"
    why_human: "Finder UI on a real Mac; the VM had no Automation grant, and P3 showed the installer could not write the Trash's .DS_Store, so Put Back may be greyed out"
  - test: "M-170: on a real Mac with a Developer ID release, reset the Downloads consent, download the release DMG in a browser, open Portty from the volume; repeat with Don't Allow"
    expected: "Portty installed and running from /Applications before the Downloads prompt appears; Allow trashes the DMG within seconds; Don't Allow keeps the DMG and Portty stays installed; a later first install of a newer release does not ask again"
    why_human: "A real TCC prompt for a Developer ID app; the VM shows it only for ad-hoc rig builds (R4-smoke, R4-a, R4-smoke-deny-a2)"
  - test: "Decision and VM row (UPD-09 residual, observation 2 / RC-WR02, now mitigated in code by quick 261126-dfn): run row QT-WR02, a first install from a DMG with no installed copy that fails inside the installer with no GUI left (reverify, swap, quit-timeout of the DMG GUI, or open/health), and confirm the osascript notice 'Portty couldn't install version X.' appears and that re-opening the DMG's app alerts the earlier failure once before it moves again"
    expected: "A notification banner (Script Editor icon, title Portty) and the Finder reveal on the late failure; an alert on the next open of the DMG's app. Owner accepts the Script Editor icon, or schedules a native notification"
    why_human: "The code and unit tests now cover the path (installer.go:420-439, app_mover.go:84-110, TestInstaller_MoverFailureLeavesSomethingToActOn, TestRunMover_ShowsEarlierResultOnce), but the end-to-end VM row QT-WR02 is PENDING (VM busy with the soak) and the banner is a user-visible product choice"
---

# Phase 18: Self-Installing Updates Verification Report

**Phase Goal:** Clicking Update installs the new Portty by itself. The verified DMG replaces `/Applications/Portty.app`, Portty relaunches, the DMG is cleaned up, and terminal sessions survive through the v1.2 takeover. A version that will not start is rolled back. A first launch from a mounted DMG moves Portty into place by itself.
**Verified:** 2026-11-26T10:45:00Z
**Status:** human_needed
**Re-verification:** Yes. The earlier report (2026-11-26T09:55:00Z, human_needed, 14/15) went stale because covered source changed: quick 261126-dfn (late mover failure notice, mover shows an earlier failed result), quick 261126-wsg (a webserver test fix, no Phase 18 file), and quick 261126-e29 (commits 331c71fc, 32bf4ab3, a40e8ae8: quit-timeout relaunch, keepDaemon carried into every result, sweep removes Portty's own failed new copy, one reused daemon client, StageFrom/Busy/IsDir de-duplication). The previous report had no `gaps:` section, so every truth was re-checked against the current code, not only a regression glance; truths touched by the changes got a full read.

## What changed since the last report, and what it did to the truths

| Change | Where (current code) | Effect on truths |
| --- | --- | --- |
| Unprovable quit-timeout pid: no signal, failed `quit-timeout` result written, target relaunched (`finishOpen`) | `installer.go:530-536`; SIGKILL-lands-but-survives branch still returns without a relaunch (`:539-543`); mover quit-timeout unchanged but now reveals and notifies (`:525-529`) | Strengthens truths 3, 8, 11. `TestInstaller_QuitTimeoutKillsOnlyOldGUI` now asserts the relaunch with the failed result already on disk, and no signal to an unlisted pid. |
| `Job.KeepDaemon` seeds `Result.KeepDaemon` in `RunInstaller`; `showEarlierResult` returns and puts back a keepDaemon | `installer.go:68-70,407`; `app_mover.go:84-110,141` | Strengthens truth 9 (a mover's retry no longer drops the daemon-keep). `TestInstaller_MoverCarriesKeepDaemon` passes. |
| Sweep removes the whole staging dir only for a copy strictly newer than the real installed `Portty.app` beside it; everything unprovable still goes to the Trash | `app_install.go:259-339` | Truth 10 holds, safer fallback direction kept. `TestSweep_FailedNewCopyRemoved`, `TestSweep_StagingDirs`, `TestSweep_TrashRefusedKeepsCopyAside` pass. |
| One `daemon.NewDaemonClient` for the health poll | `cmd_install_update.go:60-65` | Truth 7 unchanged: a pooled connection can only report an old version, which delays health and never fakes it. |
| `StageFrom`, `updater.Busy`, `updater.IsDir` shared; `installerStartFailText` and a duplicate release-notes line removed | `appinstall.go`, `mover.go`, `service.go`, `app_update.go` | Refactor only; truths 1, 4, 5 re-checked below. App-size failure now maps to `ErrCopy` on the update path ("couldn't copy the new version"), still a Reveal fallback with its own sentence. |
| Late mover failure with no GUI: installer posts `Portty couldn't install version X.` via osascript (argv, not script text) after the Finder reveal; mover mode alerts an earlier failed or rolled-back result once before staging | `installer.go:420-439`, `install.go:198-219`, `app_mover.go:84-119` | Closes most of the previous human item 5 in code (SC4 warning). Row QT-WR02 is PENDING in the VM, so the item stays, reworded. |

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
| --- | --- | --- | --- |
| 1 | SC1, VM half: with two ticking sessions and a share link, Update on N installs N+1 once the DMG passes SHA-256, Team ID, signing identifier and notarization; `/Applications/Portty.app` replaced; relaunch as N+1; shells keep PID and history; share link works; DMG ejected and deleted (UPD-05, UPD-06) | VERIFIED | Code re-read: `RunInstaller` quit loop, `VerifyApp` re-verify, `Swap`, `relaunch` with retry, health loop, old bundle to the Trash after health, `os.Remove(j.DMG)` (`installer.go:522-667`); `Service.Download` -> `install` -> `StartInstaller` unchanged. Unit: `TestSelfInstall_UpdateHappyPath`, `TestSwap_ExchangesBundles`, `TestInstaller_TrashesOldAfterHealthOnly` pass on the current tree. VM rows R1, RC-R1, R3 (run on the build before the e29 refactor; the refactor touched staging helpers only, covered by the passing unit flow and by the dfn/e29 `go test -race ./...` runs in their summaries; no VM re-run of R1 on the final tree, see Behavioral Spot-Checks). |
| 2 | SC1, office-Mac half: the same, once on the office Mac | ? HUMAN NEEDED | M-169 "not yet run"; TESTING.md records that SC1's office-Mac half stays unmet until part 2. Expected human_needed, not a gap. |
| 3 | SC2: when N+1 fails to start or never reports healthy, Portty puts N back and runs it by itself, sessions intact, and says the update did not take (UPD-07) | VERIFIED | `rollback` (`installer.go:501-520`): stopGUIs, swap back, remove failed new copy, `KeepDaemon = true`, `finishOpen(RolledBack)` writes the result before the relaunch; health needs GUI plus daemon at `v`+new steadily; GUI exit counts after two misses. `TestInstaller_HealthDeadlineRollsBack`, `GUIExitRollsBackEarly`, `ResultWrittenBeforeOpen`, `RollbackKeepDaemonWithoutResultFile`, `OpenPersistentFailureRollsBack` run and pass. VM rows R2-A..D, RC-R2A, RC-WR04 from the earlier report still stand; the later code change touched only the quit-timeout and sweep branches. |
| 4 | SC3: opening Portty from a mounted, quarantined DMG moves it to `/Applications` (or `~/Applications`), relaunches from there, ejects the DMG, trashes the downloaded DMG; two copies never run (UPD-08, amended Downloads prompt) | VERIFIED | `runMover` (`app_mover.go:118-197`): now calls `showEarlierResult` first, then ExistingCopy / Replace question / open-installed for same or newer / `MoveTarget` falls to `~/Applications`, Preflight, `StageFrom`, `start(job)`; installer mover kinds, `finishTrash` writes the result first, bounded Trash, run dir last. `TestInstaller_MoverResultBeforeTrashRunDirAfter`, `MoverTrashBounded`, `MoverTrashFailureKeepsOutcome`, `MoverCarriesKeepDaemon`, `MoveQuitTimeoutTouchesNothing`, `TestRunMover_ShowsEarlierResultOnce` pass. VM rows R4-a..g, RC-CR01 from the earlier report. Gatekeeper-enforced half and the Developer-ID Downloads prompt remain human. |
| 5 | SC4: every install failure (target not writable, App Management refusal, mount or copy failure, disk full) falls back to Reveal-in-Finder with a clear message; no relaunch stops at a Gatekeeper dialog (UPD-09) | VERIFIED | Update path: per-cause sentences, `revealFallback`; `TestService_InstallFailuresReveal`, `TestSelfInstall_StageFailureReveals`, `TestSelfInstall_StartFailureReveals`, `TestSelfInstall_RevealFailureAvailable` pass. Mover path: DMG-GUI alerts for preflight/verify/space/copy/staging failures; failures inside the installer with no GUI left now also post a notice and reveal (`revealLeft`, `installer.go:423-439`), including the mover quit-timeout, and a re-opened DMG alerts the earlier result once (`showEarlierResult`). The earlier WARNING (a silent first-install failure inside the installer) is closed in code; its end-to-end VM row QT-WR02 is PENDING, kept as a human item. |
| 6 | The installer runs from a whole-bundle ditto copy of the running Portty.app (18-10), the same version as the GUI that wrote the job, checked before its quarantine is removed | VERIFIED | `app_install.go` `startInstaller`/`copyBundle` unchanged by the later commits apart from `isRealDir` -> `updater.IsDir`; `TestStartInstaller_*` in the passing main-package run; P2d-bundle and R3 VM rows stand. |
| 7 | An update counts as done only when a GUI is up and its daemon answers at the new version steadily for HealthStable | VERIFIED (coincidental-reliance) | `installer.go:620-649`; `TestInstaller_HealthNeedsGUIAndSteadyDaemon` passes; the reused daemon client (`cmd_install_update.go:60-65`) cannot fake a new version. Holds only because LaunchServices' argv[0] equals the resolved target path exactly (`installer.go:365`; see coincidental_reliance_items). |
| 8 | No update or rollback ever signals the daemon, the installer, or another account's GUI | VERIFIED | `ParseGUIPids` (`installer.go:354-374`) takes only argv exactly the bundle executable (plus a launch-services flag) and uid == `os.Getuid()`; the unprovable-pid quit-timeout now sends no signal at all (`:530-536`). `TestInstaller_NeverSignalsDaemon`, `TestParseGUIPids`, `TestInstaller_QuitTimeoutKillsOnlyOldGUI` pass. |
| 9 | After a rollback, N comes up against whichever daemon holds the sessions with no restart question, also when the result file could not be written | VERIFIED | `app.go` keepDaemonOnce -> `EnsureOptions{Restart: false, NoHandoff: true}`; env path `KeepDaemonEnv` through `open --env` (`installer.go:456-460`, `app_install.go:356-360`); now also carried across a mover retry (`Job.KeepDaemon`). `TestInstaller_RollbackKeepDaemonWithoutResultFile`, `TestConsumeInstallResult_KeepDaemonPolicyOnce`, `TestConsumeInstallResult_KeepDaemonFromLaunchEnv`, `TestInstaller_MoverCarriesKeepDaemon` pass. |
| 10 | The old app goes to the Trash only after health; a refused Trash keeps it aside; the startup sweep cleans interrupted installs | VERIFIED | `installer.go:650-659`; `sweepDirs` (`app_install.go:259-339`): old bundle -> Trash, own newer unproven-in-place copy -> removed only with both versions readable and strictly newer, every unprovable case -> Trash. `TestInstaller_TrashesOldAfterHealthOnly`, `TrashFailureKeepsOldAside`, `TestSweep_*` (incl. `FailedNewCopyRemoved`) pass. Finder Put Back is human (M-171). |
| 11 | A failed or rolled-back version is not offered again by automatic checks, also after restart; an unreadable record suppresses offers | VERIFIED | `outcome.go` ShowResult/skip record unchanged; `TestService_CheckSkipsFailedVersion`, `TestService_CheckUnreadableSkipSuppressesOffer` pass. |
| 12 | The updater accepts both `portty-vX.Y.Z-darwin-universal.dmg` and `portty-vX.Y.Z-darwin-arm64.dmg`, preferring universal | VERIFIED | `update.go` DMGAssetNames; `TestDMGAssetNames`, `TestLatestAcceptsArm64Name`, `TestLatestPrefersFirstName` pass. |
| 13 | Downloads prompt (option A, A2): the result is written before the DMG Trash, the Trash is bounded, the run folder is removed only after it, and Don't Allow keeps the DMG while the install counts | VERIFIED | `finishTrash` (`installer.go:565-589`); the three `TestInstaller_Mover*` tests pass; VM R4-smoke, R4-smoke-deny-a2, R4-a. |
| 14 | F6 backstop (non-inferable): SIGKILL of the installer after the swap and before health leaves a launchable Portty with sessions intact, and later launches sweep the leftovers | VERIFIED | Directly observed in the VM (F6 PASS, `evidence/F6-killed.state`, `evidence/F6.state`); explicit evidence, not presence. The later sweep change only alters how a stale own copy is disposed of (deleted if strictly newer, Trashed otherwise), which does not touch the F6 recovery. |
| 15 | TESTING.md carries the Phase 18 manifest, UPD-05..UPD-09 traceability rows (paths only) and Category BI M-165..M-171 | VERIFIED | `bash tests/check-traceability-paths.sh`: `OK: 237 traceability paths exist` (was 236; e29 added rows). |

**Score:** 14/15 truths verified (0 present, behavior-unverified). The one not verified is SC1's office-Mac half, human_needed by design.

### Required Artifacts

| Artifact | Expected | Status | Details |
| --- | --- | --- | --- |
| `internal/updater/installer.go` | Job/result files, RunInstaller (swap, relaunch, health, rollback, mover kinds, Trash, late-failure notice) | VERIFIED | 667 lines, substantive, read in full for RunInstaller; wired from `cmd_install_update.go` |
| `internal/updater/appinstall.go` | Preflight, CheckSpace, Mount/Detach, CopyApp, VerifyApp, CheckStagedApp, Stage, StageFrom, Swap | VERIFIED | used by `service.go` install and `app_mover.go` (`StageFrom`) |
| `internal/updater/mover.go`, `install.go` | DMG-run detection, installed version, move target; Notify, ResultText, SafeRunner | VERIFIED | `Notify`/`ResultText` used by `installer.go` and `app_install.go`/`app_mover.go` |
| `internal/updater/service.go`, `outcome.go`, `update.go` | install hand-off, reveal fallback texts, result display and skip, asset names | VERIFIED | `app_update.go` sets Bundle and StartInstaller |
| `app_install.go`, `app_mover.go`, `cmd_install_update.go`, `main.go` | GUI hand-off, sweep, result consumption, mover mode, `install-update` dispatch | VERIFIED | read in full for the changed functions; `main.go` dispatch and `app.go` startup wiring unchanged |
| `trash_darwin.go/.m` | NSFileManager trashItemAtURL bridge | VERIFIED | `trashItem` used by installer env and sweep |
| `frontend/src/components/UpdateBanner.tsx`, `frontend/src/App.tsx` | Update / Installing / Restarting / warning states | VERIFIED | vitest `UpdateBanner` + `App.update`: 3 files, 37 tests passed |
| `evidence/` (18-07, 18-08, 18-09) | rig state captures | VERIFIED | folder present with the cited `ROW.state`/`ROW.log` captures |

### Key Link Verification

| From | To | Via | Status | Details |
| --- | --- | --- | --- | --- |
| UpdateBanner Update | `App.DownloadUpdate` | Wails binding | WIRED | `app_update.go` |
| `Service.Download` | `Service.install` | `s.Bundle != "" && s.StartInstaller != nil` | WIRED | `service.go` |
| `Service.install` | `App.startInstaller` | `StartInstaller` field | WIRED | `app_update.go` |
| `startInstaller` | `install-update` | spawn of the whole-bundle copy, started marker | WIRED | `app_install.go:88-150`, `main.go` dispatch, `cmd_install_update.go:31-49` |
| `runInstallUpdate` | `RunInstaller` | real env (Swap, openApp, realGUIPids, trashItem, one daemon client) | WIRED | `cmd_install_update.go:60-82` |
| `RunInstaller` result | `consumeInstallResult` | `update-result.json` / `PORTTY_KEEP_DAEMON_ONCE` | WIRED | `app_install.go:353-394`, `app.go` startup |
| `consumeInstallResult` | `ensure` | `keepDaemonOnce` | WIRED | `app.go` |
| `OnStartup` | `startMover` -> `runMover` -> `showEarlierResult` -> `startInstaller` | DMG-run detection | WIRED | `app_mover.go:39-54,118-197` |
| `revealLeft` | `Reveal` + `Notify` | installer env `Run` (osascript with the body as argv) | WIRED | `installer.go:423-439`, `install.go:203-207` |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| --- | --- | --- | --- | --- |
| UpdateBanner | `UpdateStatus` (state, latest, error) | `Service.set` -> `update:status` event / `GetUpdateStatus` | Yes (state machine driven by real download/verify/install) | FLOWING |
| Rolled-back banner + notification | `Result` | installer `WriteResult` -> `TakeResult` -> `ResultText` | Yes (RC-R2A banner text captured earlier; `ResultText` is now the single sentence source for installer and GUI) | FLOWING |
| Late-failure notice | `body` | `ResultText(r)` on the installer's own result | Yes | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| --- | --- | --- | --- |
| Updater and main package tests (current tree) | `GOTOOLCHAIN=go1.26.11 go test -count=1 ./internal/updater/ .` | `ok internal/updater 1.978s`, `ok portty 22.201s` | PASS |
| Named installer, sweep, mover, flow tests | `go test -count=1 -v -run 'TestInstaller_(...)\|TestSelfInstall_UpdateHappyPath\|TestSwap_ExchangesBundles\|TestParseGUIPids\|TestService_Check\|TestDMGAssetNames\|TestLatest...\|TestSweep' ./internal/updater/ .` | 37 named tests PASS, none skipped, none failed | PASS |
| Vet, both tag sets | `go vet ./internal/updater/ . && go vet -tags vmpreview . ./internal/daemon/` | clean | PASS |
| Brand gate | `bash tests/check-brand.sh` | `OK: brand gate clean` | PASS |
| Traceability paths | `bash tests/check-traceability-paths.sh` | `OK: 237 traceability paths exist` | PASS |
| Banner vitest | `npx vitest run UpdateBanner App.update` (in `frontend/`) | 3 files, 37 tests passed | PASS |
| Debt-marker scan | `grep -n "TBD\|FIXME\|XXX"` over the phase's non-test source | none | PASS |
| End-to-end VM re-run of R1/R2/R4 on the final tree | not run | the rig needs the Tart VM, which is out of scope here (host-safety rule); the VM rows predate commits 331c71fc and 32bf4ab3 | SKIPPED, see note |

Note: the VM rows (R1, R2, R4, F) were captured on builds before quick 261126-dfn and 261126-e29. The later changes are confined to the quit-timeout branch, the sweep, the late-mover-failure notice, `Job.KeepDaemon`, and de-duplicated helpers; each has a regression test that passed here. The next VM pass is a sensible regression run but is not a gap in code evidence.

### Probe Execution

Step 7c: no `scripts/*/tests/probe-*.sh` declared by the phase. The phase's probes (P1-P8, P2d-bundle) are VM rows run through `rig/p18-rig.sh` with state captures; they cannot run on the host by design. Not re-run here.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| --- | --- | --- | --- | --- |
| UPD-05 | 18-01, 18-03, 18-07, 18-08, 18-10 | Replace in /Applications after the verify chain, relaunch, sessions survive | SATISFIED (VM); office Mac human | Truths 1, 6-9; R1, RC-R1, R3; M-169 open |
| UPD-06 | 18-01, 18-05 | DMG ejected and deleted after a successful install | SATISFIED | Stage detaches on every path; `os.Remove(j.DMG)` after health (`installer.go:663`); R1/RC-R1 `cache_dmgs=0`, `attached_images=0` |
| UPD-07 | 18-03 | Rolls back by itself if the new version fails to start | SATISFIED | Truth 3; R2-A..D, RC-R2A, RC-WR04 |
| UPD-08 (amended 2026-11-25) | 18-06, 18-09 | Mover to /Applications or ~/Applications, relaunch, eject, DMG to the Trash; one-time Downloads prompt accepted, Don't Allow keeps the DMG and the install succeeds | SATISFIED (VM); Developer-ID prompt human | Truths 4, 13; R4-a..g, R4-smoke, R4-smoke-deny-a2; M-170 open |
| UPD-09 | 18-02, 18-04, 18-07 | Any install failure falls back to Reveal-in-Finder with a clear message; no Gatekeeper dialog on relaunch | SATISFIED for the enumerated causes; the mover first-install residual is now closed in code (notice + earlier-result alert), VM row QT-WR02 pending; Gatekeeper enforcement human | Truth 5; F1-F5, F7; RC-WR02; quick 261126-dfn |

No orphaned requirements: REQUIREMENTS.md maps exactly UPD-05..UPD-09 to Phase 18, and every one is claimed by a plan.

### Decision Coverage

`check.decision-coverage-verify` was skipped in the first report ("No trackable decisions in CONTEXT.md"). Key decisions re-checked against the current code: bundle-copy installer (truth 6), old app to the Trash after health (truth 10), Reveal fallback with no admin prompt (truth 5), Replace question for an older copy and silent move with none (truth 4), Downloads prompt option A with the A2 refinement (truth 13), relaunch retry (`OpenAttempts`, `installer.go:378-393`), asset names (truth 12), detection via hdiutil info. All honored. The e29 sweep rule (delete only a provably newer own copy) is consistent with the 18-CONTEXT rule that the user's old app is never deleted, only Trashed.

### Test Quality Audit

| Test File | Linked Req | Active | Skipped | Circular | Assertion Level | Verdict |
| --- | --- | --- | --- | --- | --- | --- |
| internal/updater/installer_test.go | UPD-05/07/08/09 | yes | 0 | no | Behavioral (multi-step installer runs with fakes; asserts swaps, signals, opens with the result on disk, notice text, keepDaemon) | OK |
| internal/updater/selfinstall_test.go | UPD-05/06/07/09 | yes | 0 | no | Behavioral; `SafeRunner` refuses `osascript` in a test binary | OK |
| internal/updater/appinstall_test.go | UPD-09 | yes | 1 conditional (`root can write anywhere`) | no | Value | OK |
| internal/updater/mover_test.go, app_mover_test.go | UPD-08/09 | yes | 0 | no | Behavioral; `hdiutilInfoP6` fixture captured in the VM; `TestRunMover_ShowsEarlierResultOnce` | OK |
| app_install_test.go, outcome_test.go, cmd_install_update_test.go | UPD-05/06/07 | yes | 0 | no | Value/Behavioral; `TestSweep_FailedNewCopyRemoved` isolates HOME | OK |
| internal/updater/real_darwin_test.go | v1.0 verify chain | env-gated | 2 (env unset) | no | n/a | Pre-existing opt-in; not the only proof of any UPD-05..09 truth |

Disabled tests on requirements: 0 blocking. Circular patterns: 0. Insufficient assertions: 0. The e29 summary records the (b), (c), (d) tests as written first and failing before the fix.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| --- | --- | --- | --- | --- |
| (phase source files) | - | TBD/FIXME/XXX | none found | - |
| app_mover.go | 159-161 | `shortcut:` unreadable installed version recorded as 0.0.0 after the user agreed to replace | Info | documented, user-consented |
| internal/updater/mover.go | ~71 | `shortcut:` translocated mount-from shape untested (P5 saw no translocation) | Info | guard path only |
| internal/updater/installer.go | 539-543 | the SIGKILL-lands-but-survives branch returns failed with no relaunch (old GUI is by definition still running) | Info | correct: nothing to relaunch while the old GUI lives |
| internal/updater/installer.go | 365 | exact argv[0] string match for GUI detection | Info | coincidental-reliance advisory (IN-07) |

The earlier Warning on a mover quit-timeout that returned with no reveal is gone: that path now calls `revealLeft` (`installer.go:525-529`). Its staging dir is still left for the startup sweep, which is the intended recovery.

### Human Verification Required

### 1. M-169: the office Mac, real releases
**Test:** Part 1 at the first self-installing release (mover Replace over v1.2.x from the browser-downloaded, notarized DMG); part 2 at the release after that (click Update in the banner) with two ticking sessions and a share link open from the home Mac over the tailnet.
**Expected:** Shell pids and history kept; share link opens from the home Mac before and after; DMG ejected and trashed or deleted; old app in the Trash; no Gatekeeper or App Management dialog on the relaunch.
**Why human:** SC1's office-Mac half; real releases, real Gatekeeper, App Management and tailnet.

### 2. Real Gatekeeper enforcement
**Test:** A quarantined, notarized release DMG opened on a real Mac (R4-f-gatekeeper BLOCKED in the VM), and the relaunch after an update.
**Expected:** Only the first-launch confirmation; no refusal, no Open Anyway, no dialog on the installer's relaunch.
**Why human:** The VM guest has assessments disabled.

### 3. M-171: Finder Put Back
**Test:** After an update, select the old Portty.app in the Trash; record Put Back; drag it out and open it.
**Expected:** The dragged-out copy opens as the older version.
**Why human:** Finder UI; Put Back data may be missing (P3 `.DS_Store` Code=513).

### 4. M-170: the Downloads prompt with a Developer ID release
**Test:** Reset consent, browser-download the release DMG, open Portty from the volume; repeat with Don't Allow.
**Expected:** Installed and running before the prompt; Allow trashes the DMG; Don't Allow keeps it and the install stands; no second prompt for a later release.
**Why human:** Real TCC prompt for a Developer ID app.

### 5. Row QT-WR02 and a product call on the late-failure notice
**Test:** In the VM with no installed Portty, open a DMG whose installer fails late with no GUI left (reverify, swap, DMG-GUI quit-timeout, or open/health). Then open the DMG's app again.
**Expected:** A "Portty couldn't install version X." banner and the Finder reveal on the failure; an alert for the earlier failure on the next open, once, before it moves again. Owner accepts that the banner carries the Script Editor icon (osascript), or schedules a native notification.
**Why human:** Code and unit tests now cover the path, but the end-to-end row is PENDING (the VM was busy with the soak) and the banner style is a product choice. Previously this item was an open decision on silent failures; the silence itself is fixed in code.

### Gaps Summary

No blocking gaps. Every roadmap success criterion has code that implements it on the current tree, passing behavioral unit tests (updater and main packages, race-free run of the named tests, vet on both tag sets, brand and traceability gates, frontend banner tests), and VM rows backed by rig captures from before the last three quick tasks. The quick tasks 261126-dfn, 261126-wsg and 261126-e29 introduced no regression: every truth was re-read against the current code, and the changed branches (unprovable quit-timeout relaunch, keepDaemon carried into every result, staging sweep, late mover failure notice, earlier-result alert) each have a passing regression test.

The phase is not `passed` because SC1's office-Mac half (M-169), real Gatekeeper enforcement, Finder Put Back (M-171) and the Developer-ID Downloads prompt (M-170) need a real Mac, and because the VM row for the UPD-09 residual fix (QT-WR02) has not been run. A VM regression pass of R1/R2/R4 on the final build is also advisable before release, but it is not required to close a code gap.

---

_Verified: 2026-11-26T10:45:00Z_
_Verifier: Claude (gsd-verifier)_
