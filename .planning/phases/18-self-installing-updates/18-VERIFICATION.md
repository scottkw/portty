---
phase: 18-self-installing-updates
verified: 2026-11-26T09:55:00Z
status: human_needed
score: 14/15 must-haves verified
covered_files: [".planning/phases/18-self-installing-updates/18-01-PLAN.md", ".planning/phases/18-self-installing-updates/18-01-SUMMARY.md", ".planning/phases/18-self-installing-updates/18-02-PLAN.md", ".planning/phases/18-self-installing-updates/18-02-SUMMARY.md", ".planning/phases/18-self-installing-updates/18-03-PLAN.md", ".planning/phases/18-self-installing-updates/18-03-SUMMARY.md", ".planning/phases/18-self-installing-updates/18-04-PLAN.md", ".planning/phases/18-self-installing-updates/18-04-SUMMARY.md", ".planning/phases/18-self-installing-updates/18-05-PLAN.md", ".planning/phases/18-self-installing-updates/18-05-SUMMARY.md", ".planning/phases/18-self-installing-updates/18-06-PLAN.md", ".planning/phases/18-self-installing-updates/18-06-SUMMARY.md", ".planning/phases/18-self-installing-updates/18-07-PLAN.md", ".planning/phases/18-self-installing-updates/18-07-SUMMARY.md", ".planning/phases/18-self-installing-updates/18-08-PLAN.md", ".planning/phases/18-self-installing-updates/18-08-SUMMARY.md", ".planning/phases/18-self-installing-updates/18-09-PLAN.md", ".planning/phases/18-self-installing-updates/18-09-SUMMARY.md", ".planning/phases/18-self-installing-updates/18-10-PLAN.md", ".planning/phases/18-self-installing-updates/18-10-SUMMARY.md", ".planning/phases/18-self-installing-updates/18-REVIEW-FIX.md", ".planning/phases/18-self-installing-updates/18-REVIEW.md", ".planning/phases/18-self-installing-updates/18-VM-CHECKS.md", "TESTING.md", "app.go", "app_install.go", "app_install_test.go", "app_mover.go", "app_mover_test.go", "app_update.go", "cmd_install_update.go", "cmd_install_update_test.go", "frontend/src/App.tsx", "frontend/src/components/UpdateBanner.tsx", "internal/daemon/share_vmpreview.go", "internal/updater/appinstall.go", "internal/updater/appinstall_test.go", "internal/updater/install.go", "internal/updater/installer.go", "internal/updater/installer_test.go", "internal/updater/mover.go", "internal/updater/mover_test.go", "internal/updater/outcome.go", "internal/updater/outcome_test.go", "internal/updater/selfinstall_test.go", "internal/updater/service.go", "internal/updater/update.go", "main.go", "trash_darwin.go", "trash_darwin.m", "updates_vmpreview.go"]
covered_digest: "v4:sha256:7699ce4f5d7a98cf566b91cd46c0bd4003416ea91870008e2d7438c0530bab63"
behavior_unverified: 0
overrides_applied: 0
advisory:
  - finding: "RC-WR04-stall: both sessions' output stalled ~17 min (07:11:25-07:28:24) after a keep-daemon rollback, same pids, gapless, resumed per session as a viewer joined that session"
    category: other
    reason: "No Phase 18 code runs after 07:07:49 (installer exited); Phase 18 did not change internal/relay or internal/daemon apart from the vmpreview share file; the relay hub never blocks on a slow viewer (kickSlow), and a refused takeover freezes nothing (handoff.go:460). Most likely pre-existing daemon/relay or guest behaviour, exposed in a new state (1.8.0 GUI on a 1.8.1 daemon running from an unlinked bundle). Resolve with the soak test the row proposes: GUI viewer only, same-version pair and keep-daemon pair."
    evidence_status: "observed once, cause not established, not reproduced"
  - finding: "update-result.json with outcome opened (and, by the same timing, updated) stays on disk until the next GUI start"
    category: other
    reason: "The installer writes these after the GUI it launched already ran consumeInstallResult. TakeResult consumes it at the next start, and opened/updated return before ShowResult, the notification or the skip record, so nothing is shown or suppressed. Cosmetic, out of scope."
    evidence_status: "R4-g; code read app_install.go:349-356"
  - finding: "IN-08 (left open by the review fix): any failed result writes the skip record, including transient causes (open, quit-timeout, swap), so an unattended Mac stays on the old version until a newer release or a manual check"
    category: other
    reason: "Not a Phase 18 success criterion; relevant to the rock-solid priority. Candidate follow-up."
    evidence_status: "code read outcome.go / service.go; 18-REVIEW.md IN-08"
  - finding: "F6 finding: a SIGKILLed installer leaves the verified DMG in the update cache until the next Download's Clean"
    category: other
    reason: "Recorded as deferred in 18-VM-CHECKS.md F6; the install itself recovered on the next launch. Not part of a successful install, so UPD-06 is not contradicted."
    evidence_status: "F6 row, evidence/F6.state cache_dmgs=1"
coincidental_reliance_items:
  - truth: "An update counts as done only when the new GUI is up and its daemon answers at the new version, steadily for HealthStable"
    reason: undeclared-precondition
    harden: "ParseGUIPids requires ps argv[0] to equal EvalSymlinks(target)/Contents/MacOS/Portty byte for byte (installer.go:362). Nothing guarantees LaunchServices spells argv[0] that way (case on a case-insensitive volume, /System/Volumes/Data, a symlinked home). The VM rows show the common /Applications case holds; compare with os.SameFile or proc_pidpath instead (18-REVIEW.md IN-07)."
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
  - test: "Decision (UPD-09 residual, observation 2 / RC-WR02): a first install from a DMG with no installed copy that fails inside the installer (reverify, swap, quit-timeout of the DMG GUI, or open/health with no GUI left) gives no message: the user sees only Finder with the DMG (or installed copy) selected, and re-opening the DMG's app never shows the failed result because startMover returns before consumeInstallResult and its installer overwrites update-result.json"
    expected: "Owner either accepts this as a known limitation (all SC4-enumerated causes are caught by the DMG GUI's own pre-checks and alerted, and the remaining causes need tampering or a hung GUI), or schedules a fix, e.g. mover mode reads a failed first-install result and alerts before retrying"
    why_human: "Product decision on the literal 'every install failure ... with a clear message' wording; the behaviour is observed, not uncertain"
---

# Phase 18: Self-Installing Updates Verification Report

**Phase Goal:** Clicking Update installs the new Portty by itself. The verified DMG replaces `/Applications/Portty.app`, Portty relaunches, the DMG is cleaned up, and terminal sessions survive through the v1.2 takeover. A version that will not start is rolled back. A first launch from a mounted DMG moves Portty into place by itself.
**Verified:** 2026-11-26T09:55:00Z
**Status:** human_needed
**Re-verification:** No, initial verification (no earlier 18-VERIFICATION.md)

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
| --- | --- | --- | --- |
| 1 | SC1, VM half: with two ticking sessions and a share link, Update on N installs N+1 once the DMG passes SHA-256, Team ID, signing identifier and notarization; `/Applications/Portty.app` replaced; relaunch as N+1; shells keep PID and history; share link works; DMG ejected and deleted (UPD-05, UPD-06) | VERIFIED | Code: `service.go` Download -> Verify -> `install` (Preflight, Stage, StartInstaller); `appinstall.go` Stage mounts, VerifyApp on the mounted app, CheckSpace, CopyApp, VerifyApp + CheckStagedApp (spctl `source=Notarized Developer ID`, exact version, min macOS, `--version` smoke) on the staged copy, detaches on every path; `installer.go` RunInstaller: Swap (RENAME_SWAP), relaunch with retry, health, old bundle to the Trash, `os.Remove(j.DMG)`. VM: R1 and the post-fix RC-R1 PASS (same shell pids, 300 HIST lines, gapless ticks, loopback share link 302/200/401 before and after, `cache_dmgs=0`, `attached_images=0`, old 1.8.0 in the Trash, `evidence/RC-R1.state` consistent). R3 PASS: Developer-ID 1.1.90 to the real notarized v1.2.0 through the real codesign chain, no AMFI kill, no dialog. Unit: `TestSelfInstall_UpdateHappyPath`, `TestSwap_ExchangesBundles` pass. |
| 2 | SC1, office-Mac half: the same, once on the office Mac | ? HUMAN NEEDED | M-169 "not yet run"; TESTING.md records that SC1's office-Mac half stays unmet until part 2. Expected human_needed, not a gap. |
| 3 | SC2: when N+1 fails to start or never reports healthy, Portty puts N back and runs it by itself, sessions intact, and says the update did not take (UPD-07) | VERIFIED | Code: `rollback` stops only N+1's GUIs, swaps back, removes the failed copy, writes `rolled-back` + KeepDaemon BEFORE relaunch; health needs a GUI plus the daemon at `v`+new for HealthStable within HealthTimeout; GUI exit counts after two misses. `consumeInstallResult` shows the banner text and a notification. VM: R2-A, R2-B, R2-C, R2-D (pre-final), R2-A/R2-B on the final build, RC-R2A PASS (`Portty 1.8.1 didn't start, so Portty put version 1.8.0 back. The download is shown in Finder.`, same shells, no Restart Now). RC-WR04 PASS: result write failing, keepDaemon reached N through `open --env` (`ps -E` shows `PORTTY_KEEP_DAEMON_ONCE=1`), no dialog. Unit: `TestInstaller_HealthDeadlineRollsBack`, `TestInstaller_GUIExitRollsBackEarly`, `TestInstaller_ResultWrittenBeforeOpen`, `TestInstaller_RollbackKeepDaemonWithoutResultFile`, `TestInstaller_OpenPersistentFailureRollsBack` run and pass. |
| 4 | SC3: opening Portty from a mounted, quarantined DMG moves it to `/Applications` (or `~/Applications`), relaunches from there, ejects the DMG, trashes the downloaded DMG; two copies never run (UPD-08, amended Downloads prompt) | VERIFIED | Code: `app.go:306` startMover runs before consumeInstallResult, tray, daemon; `mover.go` DetectDMGRun (read-only image under /Volumes via hdiutil info); `app_mover.go` ExistingCopy / Replace question / open-installed for same or newer / MoveTarget falls to `~/Applications`; installer mover kinds Detach, swap, relaunch, `finishTrash` writes the result first, Trash bounded by `trashWait` (60 s), run dir removed after the Trash returns (A2). VM: R4-a to R4-g PASS on `e03f4ef1`, RC-CR01 PASS post-fix; R4-b sessions moved through the takeover with the same shell pids; R4-f quarantined DMG not translocated, installed copy has 0 quarantine lines; menu-bar item count 1 after every row. The Gatekeeper-enforced half (R4-f-gatekeeper BLOCKED) and the Developer-ID Downloads prompt (M-170) are routed to human verification. |
| 5 | SC4: every install failure (target not writable, App Management refusal, mount or copy failure, disk full) falls back to Reveal-in-Finder with a clear message; no relaunch stops at a Gatekeeper dialog (UPD-09) | VERIFIED (with WARNING) | Update path: each cause has its own sentence (`installFailureTexts`), `revealFallback` re-quarantines and reveals; installer-stage failures write the result before relaunching N, which shows it. VM F1, F2, F3, F4, F5(a/b), F7 PASS. Mover path: preflight, verify, space, copy and staged-check failures alert in the DMG GUI (`TestRunMover_StagingFailureAlerts`). Gatekeeper: the installed copy is the staged copy that passed spctl `Notarized Developer ID` with quarantine cleared, relaunched with plain `open`; no dialog in any VM relaunch (enforcement itself is human, item 2). WARNING: a mover first install that fails inside the installer leaves no message (RC-WR02 "Nothing says why, except update.log"); not one of SC4's enumerated causes, escalated as a decision (human item 5). |
| 6 | The installer runs from a whole-bundle ditto copy of the running Portty.app (18-10), the same version as the GUI that wrote the job, checked before its quarantine is removed | VERIFIED | `app_install.go` startInstaller/copyBundle (ditto, VerifyApp, ClearQuarantine, spawn, 3 s marker wait, run dir removed on any failure); P2d-bundle and R3 on Developer ID: kernel registered the same-Team AppBundles exception, no `Code=-420`. |
| 7 | An update counts as done only when a GUI is up and its daemon answers at the new version steadily for HealthStable | VERIFIED (coincidental-reliance) | `installer.go:606-635`; `TestInstaller_HealthNeedsGUIAndSteadyDaemon`; RC-R1 confirms the uid-filtered `ps -o pid=,uid=,args=` parse on macOS 27.2. Holds only because LaunchServices' argv[0] equals the resolved target path exactly (see coincidental_reliance_items). |
| 8 | No update or rollback ever signals the daemon, the installer, or another account's GUI | VERIFIED | `ParseGUIPids` takes only argv exactly the bundle executable (optionally a launch-services flag) and uid == os.Getuid(); `TestInstaller_NeverSignalsDaemon`, `TestParseGUIPids` pass; R2-D: only the hung old GUI was SIGKILLed, daemon kept its pid until the takeover. |
| 9 | After a rollback, N comes up against whichever daemon holds the sessions with no restart question, also when the result file could not be written | VERIFIED | `app.go:390` keepDaemonOnce -> EnsureOptions{Restart: false, NoHandoff: true}; env path `app_install.go:331-335`; RC-WR04 and R2-B. |
| 10 | The old app goes to the Trash only after health; a refused Trash keeps it aside; the startup sweep cleans interrupted installs | VERIFIED | `installer.go:639-645`, `app_install.go` sweepDirs; `TestInstaller_TrashesOldAfterHealthOnly`, `TestInstaller_TrashFailureKeepsOldAside`, `TestSweep_*`; VM R1-trash, P3, S1, F6. Finder Put Back is human (M-171). |
| 11 | A failed or rolled-back version is not offered again by automatic checks, also after restart; an unreadable record suppresses offers | VERIFIED | `outcome.go` ShowResult/skip record; `TestService_CheckSkipsFailedVersion`, `TestService_CheckUnreadableSkipSuppressesOffer` (in the passing updater run); RC-R2A shows the `skip-version` file. |
| 12 | The updater accepts both `portty-vX.Y.Z-darwin-universal.dmg` and `portty-vX.Y.Z-darwin-arm64.dmg`, preferring universal | VERIFIED | `update.go` DMGAssetNames; `TestDMGAssetNames`, `TestLatestAcceptsArm64Name`, `TestLatestPrefersFirstName`; R4 rows used the arm64 name. |
| 13 | Downloads prompt (option A, A2): the result is written before the DMG Trash, the Trash is bounded, the run folder is removed only after it, and Don't Allow keeps the DMG while the install counts | VERIFIED | `installer.go:551-575`; `TestInstaller_MoverResultBeforeTrashRunDirAfter`, `TestInstaller_MoverTrashBounded`, `TestInstaller_MoverTrashFailureKeepsOutcome`; VM R4-smoke (PASS), R4-smoke-deny-a2, R4-a (during-prompt capture shows result `updated`, run dir 1, DMG still in Downloads). |
| 14 | F6 backstop (non-inferable): SIGKILL of the installer after the swap and before health leaves a launchable Portty with sessions intact, and later launches sweep the leftovers | VERIFIED | Directly observed in the VM (F6 PASS, `evidence/F6-killed.state`, `evidence/F6.state`); this is explicit evidence, not presence plus wiring. |
| 15 | TESTING.md carries the Phase 18 manifest, UPD-05..UPD-09 traceability rows (paths only) and Category BI M-165..M-171 | VERIFIED | TESTING.md line 36 note, rows 551-574, Category BI at 1998-2058; `bash tests/check-traceability-paths.sh`: OK (236 paths). |

**Score:** 14/15 truths verified (0 present, behavior-unverified). The one not verified is SC1's office-Mac half, which is human_needed by design.

### Required Artifacts

| Artifact | Expected | Status | Details |
| --- | --- | --- | --- |
| `internal/updater/installer.go` | Job/result files, RunInstaller (swap, relaunch, health, rollback, mover kinds, Trash) | VERIFIED | 653 lines, substantive; wired from `cmd_install_update.go` |
| `internal/updater/appinstall.go` | Preflight, CheckSpace, Mount/Detach, CopyApp, VerifyApp, CheckStagedApp, Stage, Swap | VERIFIED | used by `service.go` install and `app_mover.go` |
| `internal/updater/mover.go` | DMG-run detection, installed version, move target | VERIFIED | used by `app_mover.go` |
| `internal/updater/service.go`, `outcome.go`, `update.go` | install hand-off, reveal fallback texts, result display and skip, asset names | VERIFIED | `app_update.go` sets Bundle and StartInstaller |
| `app_install.go`, `app_mover.go`, `cmd_install_update.go`, `main.go` | GUI hand-off, sweep, result consumption, mover mode, `install-update` dispatch | VERIFIED | `main.go:246` dispatch; `app.go:306-330` startup wiring |
| `trash_darwin.go/.m` | NSFileManager trashItemAtURL bridge | VERIFIED | used by installer env and sweep |
| `frontend/src/components/UpdateBanner.tsx`, `frontend/src/App.tsx` | Update / Installing / Restarting / warning states | VERIFIED | vitest UpdateBanner.test.tsx + App.update.test.tsx: 34 passed |
| `evidence/` (18-07, 18-08, 18-09 plans) | rig state captures | VERIFIED | gsd verify.artifacts reports "Not a file: path is a directory" for the folder entry; the folder holds every cited `ROW.state`/`ROW.log` (tool false negative) |

gsd `verify.artifacts`: 18-01..06 and 18-10 all passed; 18-07/08/09 only the evidence-folder entry above. `verify.key-links`: all plans 100% verified.

### Key Link Verification

| From | To | Via | Status | Details |
| --- | --- | --- | --- | --- |
| UpdateBanner Update | `App.DownloadUpdate` | Wails binding | WIRED | `app_update.go:170` |
| `Service.Download` | `Service.install` | `s.Bundle != "" && s.StartInstaller != nil` | WIRED | `service.go:264` |
| `Service.install` | `App.startInstaller` | `StartInstaller` field | WIRED | `app_update.go:43` |
| `startInstaller` | `install-update` | spawn of the whole-bundle copy, started marker | WIRED | `app_install.go:122-139`, `main.go:246` |
| `runInstallUpdate` | `RunInstaller` | real env (Swap, openApp, realGUIPids, trashItem, HealthInfo) | WIRED | `cmd_install_update.go:52-74` |
| `RunInstaller` result | `consumeInstallResult` | `update-result.json` / `PORTTY_KEEP_DAEMON_ONCE` | WIRED | `app.go:311`, `app_install.go:328` |
| `consumeInstallResult` | `ensure` | `keepDaemonOnce` | WIRED | `app.go:390` |
| `OnStartup` | `startMover` -> `runMover` -> `startInstaller` | DMG-run detection | WIRED | `app.go:306`, `app_mover.go:52,186` |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| --- | --- | --- | --- | --- |
| UpdateBanner | `UpdateStatus` (state, latest, error) | `Service.set` -> `update:status` event / `GetUpdateStatus` | Yes (state machine driven by real download/verify/install) | FLOWING |
| Rolled-back banner + notification | `Result` | installer `WriteResult` -> `TakeResult` | Yes (RC-R2A banner text captured by axdump) | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| --- | --- | --- | --- |
| Updater and main package tests | `GOTOOLCHAIN=go1.26.11 go test -count=1 ./internal/updater/ .` | `ok internal/updater 1.129s`, `ok portty 19.589s` | PASS |
| Vet, both tag sets | `go vet ./internal/updater/ . && go vet -tags vmpreview . ./internal/daemon/` | clean | PASS |
| Brand gate | `bash tests/check-brand.sh` | `OK: brand gate clean` | PASS |
| Traceability paths | `bash tests/check-traceability-paths.sh` | `OK: 236 traceability paths exist` | PASS |
| Rollback, keep-daemon, mover named tests | `go test -v -run 'TestInstaller_(GUIExitRollsBackEarly|HealthDeadlineRollsBack|RollbackKeepDaemonWithoutResultFile|ResultWrittenBeforeOpen|MoverResultBeforeTrashRunDirAfter|MoverFailureLeavesSomethingToActOn|OpenPersistentFailureRollsBack|NeverSignalsDaemon)|TestSelfInstall_UpdateHappyPath|TestSwap_ExchangesBundles' ./internal/updater/` | 10 PASS, none skipped | PASS |
| Banner vitest | `npx vitest run UpdateBanner.test.tsx App.update.test.tsx` | 2 files, 34 tests passed | PASS |

### Probe Execution

Step 7c: no `scripts/*/tests/probe-*.sh` declared by the phase. The phase's probes (P1-P8, P2d-bundle) are VM rows run through `rig/p18-rig.sh` with state captures; they cannot run on the host by design (host-safety rule). Not re-run here.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| --- | --- | --- | --- | --- |
| UPD-05 | 18-01, 18-03, 18-07, 18-08, 18-10 | Replace in /Applications after the verify chain, relaunch, sessions survive | SATISFIED (VM); office Mac human | Truths 1, 6-9; R1, RC-R1, R3; M-169 open |
| UPD-06 | 18-01, 18-05 | DMG ejected and deleted after a successful install | SATISFIED | Stage detaches on every path; `os.Remove(j.DMG)` after health; R1/RC-R1 `cache_dmgs=0`, `attached_images=0` |
| UPD-07 | 18-03 | Rolls back by itself if the new version fails to start | SATISFIED | Truth 3; R2-A..D, RC-R2A, RC-WR04 |
| UPD-08 (amended 2026-11-25) | 18-06, 18-09 | Mover to /Applications or ~/Applications, relaunch, eject, DMG to the Trash; one-time Downloads prompt accepted, Don't Allow keeps the DMG and the install succeeds | SATISFIED (VM); Developer-ID prompt human | Truths 4, 13; R4-a..g, R4-smoke, R4-smoke-deny-a2; M-170 open |
| UPD-09 | 18-02, 18-04, 18-07 | Any install failure falls back to Reveal-in-Finder with a clear message; no Gatekeeper dialog on relaunch | SATISFIED for the enumerated causes; residual mover path escalated; Gatekeeper enforcement human | Truth 5; F1-F5, F7; RC-WR02 |

No orphaned requirements: REQUIREMENTS.md maps exactly UPD-05..UPD-09 to Phase 18, and every one is claimed by a plan.

### Decision Coverage

`check.decision-coverage-verify`: skipped, "No trackable decisions in CONTEXT.md" (the decisions are prose bullets, not a trackable list). Manual check of the key decisions: bundle-copy installer (18-10, truth 6), old app to the Trash after health (truth 10), Reveal fallback with no admin prompt (truth 5), Replace question for an older copy and silent move with none (truth 4), Downloads prompt option A with the A2 refinement (truth 13), relaunch retry (A9, `OpenAttempts`), asset names (truth 12), detection via hdiutil info (mover.go). All honored.

### Test Quality Audit

| Test File | Linked Req | Active | Skipped | Circular | Assertion Level | Verdict |
| --- | --- | --- | --- | --- | --- | --- |
| internal/updater/installer_test.go | UPD-05/07/08 | yes | 0 | no | Behavioral (multi-step installer runs with fakes, asserts swaps, signals, result files, reveals) | OK |
| internal/updater/selfinstall_test.go | UPD-05/06/07/09 | yes | 0 | no | Behavioral | OK |
| internal/updater/appinstall_test.go | UPD-09 | yes | 1 conditional (`root can write anywhere`) | no | Value | OK |
| internal/updater/mover_test.go, app_mover_test.go | UPD-08 | yes | 0 | no | Behavioral; `hdiutilInfoP6` fixture captured in the VM (external provenance) | OK |
| app_install_test.go, outcome_test.go, cmd_install_update_test.go | UPD-05/06/07 | yes | 0 | no | Value/Behavioral | OK |
| internal/updater/real_darwin_test.go | v1.0 verify chain | env-gated | 2 (UPDATE_REAL_DMG / UPDATE_RELEASE_JSON unset) | no | n/a | Pre-existing opt-in; not the only proof of any UPD-05..09 truth |

Disabled tests on requirements: 0 blocking. Circular patterns: 0. Insufficient assertions: 0.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| --- | --- | --- | --- | --- |
| (all 19 phase source files) | - | TBD/FIXME/XXX | none found | - |
| app_mover.go | 129 | `shortcut:` unreadable installed version recorded as 0.0.0 after the user agreed to replace | Info | documented, user-consented |
| internal/updater/mover.go | 71 | `shortcut:` translocated mount-from shape untested (P5 saw no translocation) | Info | guard path only |
| internal/updater/installer.go | 516-517 | mover quit-timeout returns `failed` with no reveal and leaves the staging dir | Warning | part of the observation-2 residual (DMG GUI hang only) |
| internal/updater/installer.go | 362 | exact argv[0] string match for GUI detection | Info | coincidental-reliance advisory (IN-07) |

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

### 5. Decision: silent mover first-install failures inside the installer
**Test:** Read RC-WR02. Decide whether a first install from a DMG that fails after the DMG GUI quit (reverify, swap, DMG-GUI quit-timeout, open/health with no GUI) may end with only a Finder reveal and no message, with re-opening the DMG's app never showing the failed result.
**Expected:** Accept as a known limitation (override) or schedule a fix.
**Why human:** The behaviour is observed; whether it meets "every install failure ... with a clear message" is a product call.

### Gaps Summary

No blocking gaps. Every roadmap success criterion has code that implements it, passing behavioural unit tests, and VM rows backed by rig state captures, re-run on the post-review build (RC-R1, RC-R2A, RC-CR01, RC-WR04, RC-WR02). The phase is not `passed` because SC1's office-Mac half (M-169), real Gatekeeper enforcement, Finder Put Back (M-171) and the Developer-ID Downloads prompt (M-170) need a real Mac, and because one UPD-09 residual (observation 2) needs an owner decision.

Verdict on the three open observations:
1. **RC-WR04-stall:** resolved after verification as a guest clock step, not a stall: at guest time 07:10:02 `timed` set the clock forward 17 min 03 s to 07:27:05, so the "17 minutes" never passed for any process (`evidence/RC-WR04-clockstep.log`, `.planning/debug/resolved/rc-wr04-stall.md`; 18-VM-CHECKS.md is left as recorded so this report's covered digest stays valid). The original verdict follows. Not a Phase 18 gap; out of scope for this phase's goal (sessions stayed intact, same pids, no lost output). No Phase 18 code was running when it began (the installer exited at 07:07:49; the stall began at about 07:11:25). Phase 18 changed nothing in `internal/relay` or `internal/daemon` except the vmpreview share file. The relay hub never blocks on a slow viewer (`kickSlow`), and a refused takeover freezes nothing (`handoff.go:460`). The 1.8.0 and 1.8.1 builds come from the same commit, so the wire protocol is identical. The per-session resume when a viewer joined points at daemon/relay or guest behaviour that predates the phase. Phase 18 does create the state it appeared in: an older GUI on a newer kept daemon whose bundle was deleted by the rollback (IN-04). Given the reliability priority, track it as a follow-up soak test, as the row proposes: GUI viewer only, on a same-version pair and on the keep-daemon pair.
2. **Failed first install from a DMG, result never shown:** a real but narrow UPD-09 shortfall in the "clear message" half. It is not one of SC4's enumerated causes, so it is a warning, escalated as human item 5, not a blocker. The reveal does happen except on the DMG-GUI quit-timeout path.
3. **`opened` result left on disk:** out of scope and harmless. `updated` results linger the same way because the installer writes them after the new GUI has started. The next start consumes them, and neither outcome reaches ShowResult, the notification or the skip record.

---

_Verified: 2026-11-26T09:55:00Z_
_Verifier: Claude (gsd-verifier)_
