---
phase: 18-self-installing-updates
plan: 06
subsystem: updater
tags: [macos, mover, dmg, hdiutil, tcc, installer, tart-vm, rig]
status: complete

requires:
  - phase: 18-02
    provides: Preflight, CheckSpace, AppSize, CheckStagedApp, installFailureTexts
  - phase: 18-03
    provides: relaunch() with open retry, rollback, the result file
  - phase: 18-05
    provides: trashItem, the installer's Trash step, the startup sweep
  - phase: 18-10
    provides: startInstaller's whole-bundle installer copy
provides:
  - "internal/updater/mover.go: StdoutRunner, DiskImage, ParseHdiutilInfo, DMGRun, DetectDMGRun, InstalledVersion, ExistingCopy, MoveTarget"
  - "internal/updater/installer.go: KindMove, KindOpenInstalled, OutcomeOpened, checkMoveJob, imageFileOK, moveParents/volumesRoot seams, the move and open-installed branches of RunInstaller, finishTrash (result, then the bounded DMG Trash, then RemoveRunDir), InstallerEnv.RemoveRunDir, trashWait"
  - "internal/updater/service.go: InstallFailureReason"
  - "app.go: moverMode, detectMoverFunc, moverStartFunc, the startup branch and the domReady early return"
  - "app_mover.go: startMover, detectMover, statfsOrigin, runMover, moverStdoutRunner, moverRoots, openAppPathFunc"
  - "cmd_install_update.go: removeRunDir passed to the installer as RemoveRunDir"
  - "18-VM-CHECKS.md rows P4, P5, P6, R4-smoke (PASS), R4-smoke-run1 (FAIL, superseded), R4-smoke-deny, P4-dialog, R4-smoke-rundir-first, R4-smoke-deny-a2"
affects: [18-07, 18-09]

tech-stack:
  added: []
  patterns:
    - "hdiutil output read through a stdout-only runner, written to a 0600 temp file and converted with plutil (no plist library)"
    - "Mover mode decides before anything that could pin the volume, then reuses the update installer with two new job kinds"
    - "A call that can wait on a system prompt runs in a goroutine with a time.After limit, after the outcome is recorded"

key-files:
  created:
    - internal/updater/mover.go
    - internal/updater/mover_test.go
    - app_mover.go
    - app_mover_test.go
    - .planning/phases/18-self-installing-updates/evidence/R4-smoke.state
    - .planning/phases/18-self-installing-updates/evidence/R4-smoke-deny.state
    - .planning/phases/18-self-installing-updates/evidence/R4-smoke-extra.log
    - .planning/phases/18-self-installing-updates/evidence/R4-smoke-before.state
    - .planning/phases/18-self-installing-updates/evidence/R4-smoke-rundir-first.state
    - .planning/phases/18-self-installing-updates/evidence/R4-smoke-rundir-first.log
    - .planning/phases/18-self-installing-updates/evidence/R4-smoke-allow-before.state
    - .planning/phases/18-self-installing-updates/evidence/R4-smoke-allow.state
    - .planning/phases/18-self-installing-updates/evidence/R4-smoke-allow.log
    - .planning/phases/18-self-installing-updates/evidence/R4-smoke-deny-a2.state
    - .planning/phases/18-self-installing-updates/evidence/R4-smoke-deny-a2.log
  modified:
    - internal/updater/installer.go
    - internal/updater/installer_test.go
    - internal/updater/service.go
    - cmd_install_update.go
    - app.go
    - .planning/phases/18-self-installing-updates/18-VM-CHECKS.md
    - .planning/phases/18-self-installing-updates/18-06-PLAN.md
    - .planning/phases/18-self-installing-updates/18-CONTEXT.md

key-decisions:
  - "P5 CONFIRMED: a DMG launch runs from /Volumes, not translocated, so detection reads the path; the statfs branch stays as an unexercised guard"
  - "P4: opening the volume's app starts a second GUI beside a running installed one, so the Replace path stops the installed GUI's pids before the swap"
  - "An unreadable installed version is written to the job as 0.0.0 once the user answers Replace, so the never-downgrade rule still holds for every readable version"
  - "The DMG GUI is never signalled: a mover job whose DMG GUI outlives QuitWait ends failed quit-timeout with nothing changed"
  - "A failed first install (no older copy) leaves the verified new copy installed and the DMG in place; it never relaunches a target that does not exist"
  - "User decision option A: keep trashing the downloaded DMG and accept the one-time system Downloads prompt on a first install from a DMG"
  - "Mover kinds write the final result and relaunch before the DMG Trash; the Trash is capped at 60 s (trashWait), and no answer logs dmg kept"
  - "Refinement A2 (orchestrator, within option A): the run folder is removed after the Trash returns or times out, because tccd refuses without a prompt once the installer's run-* executable is deleted (R4-smoke-rundir-first)"

requirements-completed: [UPD-08, UPD-09]

estimate:
  tokens: 90000
actuals:
  tokens: 26700
  tasks: 3
  commits: 15
plan_head_before: 4f85e69934abec4192c9fe5f0dc8ce3b946f4d50
plan_head_after: 79be910c

duration: about 75 min (first run) plus about 40 min (re-execution)
completed: 2026-11-26
---

# Phase 18 Plan 06: First launch from a DMG moves Portty into place Summary

**Opening Portty from a mounted DMG now installs it.** Mover mode starts before any daemon, tray or window work. It stages a checked copy and hands it to the installer, which ejects the volume, puts the copy in /Applications, relaunches it and moves the DMG to the Trash. A DMG in `~/Downloads` raises macOS's one-time Downloads prompt. Under the user's option A, the installer now records the outcome before that prompt, waits on it for at most 60 s, and removes its run folder only after the Trash returns. VM row R4-smoke passes on that rule.

## Result

| Row | Verdict | One line |
| --- | --- | --- |
| P5 | CONFIRMED | A quarantined v1.2.0 DMG opened Finder-style runs from `/Volumes/Portty/...`, not translocated (0 AppTranslocation mounts, `lsof` txt on `/Volumes`). The first-launch dialog needed an AXPress; synthetic clicks do not reach it |
| P6 | OBSERVED | A plain `hdiutil detach` succeeds on the first try once the volume's GUI and daemon exit: 10 676 ms on the first detach of a fresh download, then 118 ms and 57 ms. `-force` was never needed. The fixture was captured here |
| P4 | OBSERVED | Opening the volume's app while the installed copy runs starts a second GUI from `/Volumes`. A plain `open /Applications/Portty.app` after the old pid is gone started exactly one GUI in 3 of 3 runs |
| R4-smoke-run1 | FAIL (superseded) | `4188de71`: the move worked, but the installer blocked inside the Trash on the Downloads prompt with nothing recorded, and the plan stopped for the decision |
| R4-smoke-deny | OBSERVED | `4188de71`: with Don't Allow, the DMG is kept, the outcome is `updated` and the run dir is removed |
| P4-dialog | OBSERVED | The Replace question shows within 1 s, frontmost at layer 8. Cancel opens the installed 1.8.0 and leaves the DMG mounted |
| R4-smoke-rundir-first | OBSERVED | `135c5e20`, option A as first written (run folder removed before the Trash): tccd logged `proc_pidpath_audittoken() failed` for the installer pid and refused without a prompt, so the DMG stayed in Downloads. This caused STOP 2 |
| **R4-smoke** | **PASS** | `79be910c`: 1.8.1 ran from /Applications with one GUI and its daemon there, the volume was ejected, and no staging dir was left. `updated` was recorded at 04:53:17, before the prompt (`AUTHREQ_PROMPTING` at 04:53:17.77). A capture taken while the prompt was up shows the result already written. The only dialog was the system prompt. After Allow at 04:53:34, the DMG was in the Trash by 04:53:36.9, the installer was gone and run_dirs=0 |
| R4-smoke-deny-a2 | OBSERVED | `79be910c`, Don't Allow: `updated` recorded before the prompt, `dmg kept` logged, the DMG still in Downloads, and the installer and its run folder gone about 2 s after the answer (inside the 60 s limit) |

## The decision and how it was applied

- **STOP 1 (first execution):** CONTEXT asked for a move "without asking" plus a trashed download. On a fresh Mac, any Portty process that moves a file out of `~/Downloads` needs the Downloads consent once. That is research Assumption A5, and it showed up in R4-smoke-run1.
- **User decision 2026-11-25, option A** (18-CONTEXT.md, REQUIREMENTS UPD-08, `4b308d68`):
  - Keep trashing the DMG and accept the one-time prompt.
  - Write the result before the Trash so that an unanswered prompt never leaves the install unrecorded.
  - Don't Allow keeps the DMG.
- **STOP 2 (re-execution):** the decision also said to remove the run folder before the Trash. I did that in `135c5e20`. In the VM it suppressed the prompt entirely, because the run folder holds the installer's own executable and tccd can no longer resolve a process whose executable has been deleted (R4-smoke-rundir-first).
- **Refinement A2** (the orchestrator, within option A, 2026-11-26): the run folder is now removed after the Trash returns or times out (`79be910c`). This is recorded in 18-CONTEXT.md under the Downloads-prompt bullet.
- **The bounded wait, and why this version:** `finishTrash` starts `env.Trash` in a goroutine and waits on it or on `time.After(trashWait)` (60 s). This is the simplest version that is safe:
  - The outcome is already on disk, so an unanswered prompt costs nothing but a lingering process, and the cap ends that process after 60 s. It logs `dmg kept: no answer to the Trash within 1m0s`, removes its run folder and exits, which ends the blocked cgo call.
  - It leaves no lock or marker for anything else to wait on.
  - It does not involve the GUI or a new job state.
  - Only the DMG Trash is wrapped, because only that call can wait on a privacy prompt. The 18-05 old-bundle Trash is unchanged.
  - The timeout path is covered by `TestInstaller_MoverTrashBounded`. It was not run in the VM: both prompt runs were answered within about 16 s.
- **Order on the mover kinds:** relaunch first, then `finish` (result written), then the guarded, bounded DMG Trash, then `RemoveRunDir`. 18-03's rule that a result is written before every rollback relaunch is untouched. The healthy path relaunches before its result, as it always has.

## Task Commits

1. Task 1 RED: `eb902e4d` test(18-06): add failing tests for disk-image launch detection
2. Task 1 GREEN: `6a3fe29c` feat(18-06): detect a disk-image launch and record probes P4-P6
3. Task 2 RED: `151077c4` test(18-06): add failing tests for the installer's move and open-installed kinds
4. Task 2 GREEN: `7866fea3` feat(18-06): installer move and open-installed kinds
5. Task 3 RED: `9d4accfc` test(18-06): add failing tests for mover mode at GUI startup
6. Task 3 GREEN: `4188de71` feat(18-06): mover mode at GUI startup with the Replace question
7. Fix: `4ae1bc50` fix(18-06): keep the mover installer tests' result file in a temp dir
8. Task 3 VM rows: `c1df01da` docs(18-06): R4-smoke FAIL on the Downloads privacy prompt, deny and dialog rows
9. STOP 1 summary: `f6195330`
10. Decision fix RED: `c24700c5` test(18-06): add failing tests for result and run dir before the DMG Trash
11. Decision fix GREEN: `135c5e20` fix(18-06): write the result and remove the run dir before the DMG Trash
12. STOP 2: `1d722d26` docs(18-06): STOP 2, removing the run dir before the Trash suppresses the Downloads prompt
13. A2 RED: `f77365ef` test(18-06): run dir must outlive the DMG Trash call (A2)
14. A2 GREEN: `79be910c` fix(18-06): remove the run dir after the DMG Trash returns (A2)
15. This SUMMARY, the R4-smoke PASS and deny rows, the plan gate and the CONTEXT note (the final docs commit)

## TDD Gate Compliance

- Task 1: RED `eb902e4d` (`undefined: ParseHdiutilInfo`), then GREEN `6a3fe29c`.
- Task 2: RED `151077c4` (`undefined: moveParents`), then GREEN `7866fea3`.
- Task 3: RED `9d4accfc` (`unknown field detectMoverFunc`), then GREEN `4188de71`.
- Decision fix: RED `c24700c5`, then GREEN `135c5e20`.
  - The RED commit adds inert stubs (`RemoveRunDir` field, `trashWait` var) so the tests compile.
  - `TestInstaller_MoverResultAndRunDirBeforeTrash` failed in 3 of 3 subtests ("result file ... present false", "run folder still there"), and `TestInstaller_MoverTrashBounded` blocked 5 s and failed. The classifier returned RED_EVIDENCE_OK.
  - Semantic assessment: the target failed on the planned ordering assertions. `TestInstaller_MoverTrashFailureKeepsOutcome` passed from the start; it guards the existing Don't Allow behaviour.
- A2: RED `f77365ef`, then GREEN `79be910c`.
  - The test was renamed `TestInstaller_MoverResultBeforeTrashRunDirAfter`. It failed in 3 of 3 subtests with "run folder already removed when the image Trash was called". The classifier returned RED_EVIDENCE_OK.
  - Semantic assessment: it failed on the A2 ordering assertion for the intended reason. The bounded test gained a "run folder removed after the wait ran out" check, and that check already passed on `135c5e20`.
- The early RED commits (Tasks 1-3) predate the #3770 format check. They failed on compile errors, as recorded at the time.
- REFACTOR: none.

## Verification

- Every verify block in the plan passes on `79be910c`:
  - gofmt is clean, and `go vet` passes (default and `-tags vmpreview`).
  - golangci-lint `--new-from-rev=c714c8ef` reports 0 issues.
  - The test lists meet their minimums: 4 or more probe tests, 4 or more move-kind tests, 5 or more TestMover tests.
  - The race tests pass.
  - `grep -c KindOpenInstalled internal/updater/installer.go` is at least 2, `moverMode` and `Replace Portty` are present, and the P4-P6 rows have verdicts.
  - `grep -cE '^\| R4-smoke \| PASS \|'` returns 1.
  - check-brand is clean, and check-traceability-paths reports 228 paths.
- `GOTOOLCHAIN=go1.26.11 go test -race -count=1 ./internal/updater/ .` passes.
- The acceptance criteria hold:
  - A test proves mover mode never calls ensure or shows the window, and another proves the normal path still calls ensure.
  - The R4-smoke evidence shows the daemon's executable under /Applications (`evidence/R4-smoke-allow.state`: `83358 /Applications/Portty.app/Contents/MacOS/Portty`).

## Deviations from Plan

**1. [Rule 1 - Bug] Test fixture wrote update-result.json into the package folder.** rbFix now has its own `resDir`. Commit `4ae1bc50`.

**2. [Test design] The normal-path startup test does not run the whole `startup`**, which would start the tray, a sweep of the host's /Applications and the Tailscale health poller. The branch is a separate `startMover()` call instead, and the mover-mode test does run the real `startup`.

**3. [Placement] Move-kind installer tests are in installer_test.go, as the plan says.** TESTING.md is untouched, because 18-09 owns it.

**4. [Rule 2] `finishOpen` skips the relaunch when the target does not exist** (a first install that fails before its swap).

**5. [Tooling] AXPress helper.** Synthetic clicks do not reach the first-launch dialog or the Downloads prompt. A 15-line Swift `axpress` (scratch only) was copied to the VM's `~/p18` and removed with it.

**6. [Process] Plan commit ledger.** The worktree guard refuses writes under `.git/worktrees`. `plan_head_before` is the dispatch base `4f85e699`. `commits: 15` counts the `(18-06)` commits since that base, including the final docs commit. A plain `git rev-list --count 4f85e699..HEAD` also counts the waves merged in between (17 before the final commit).

**7. [Decision-driven] The R4-smoke gate was amended to the user's Downloads-prompt decision (option A).** In 18-06-PLAN.md, the must-have truth and Task 3's PASS text no longer require "no dialog" and "DMG in the Trash within 60 s with no answer". They now require:
- the outcome recorded before the prompt is answered, shown by a capture taken while it is up;
- the system Downloads prompt as the only dialog;
- the DMG in the Trash after Allow, and run_dirs=0;
- a Don't Allow re-check.

The grep gate (`^\| R4-smoke \| PASS \|`) is unchanged. The first-run FAIL row was renamed `R4-smoke-run1` (FAIL, superseded) so that the canonical `R4-smoke` row is the decision-rule run.

**8. [Decision-driven, A2] Run folder removed after the Trash, not before it.** The decision text said "before". R4-smoke-rundir-first showed that order suppresses the prompt and never trashes a DMG in Downloads (STOP 2). The orchestrator chose A2 as a refinement within option A and recorded it in 18-CONTEXT.md.

**9. [Rig] Downloads consent reset service-wide** (`tccutil reset SystemPolicyDownloadsFolder`, VM only) before each prompt run. The bundle-scoped reset fails with -10814 once /Applications/Portty.app is gone. At the end, the bundle-scoped reset was run with the app installed.

## Host commands run (all of them)

- `pnpm install --frozen-lockfile` and `pnpm run build` in `frontend/` (the outputs are gitignored)
- gofmt, `go vet` (default and `-tags vmpreview`), golangci-lint, `go test`, check-brand, check-traceability-paths, `gsd-tools check tdd-red-evidence`
- `p18-rig.sh build 1.8.1` into the session scratch dir: 4188de71 and 1.8.0 in the first run, then 135c5e20 and 79be910c. Each build runs wails build, jq, cp, and the git restore of wails.json and frontend/src/wailsjs. The tree was clean afterwards.
- `shasum -a 256 -c` of the existing verified v1.2.0 scratch download in the first run (no new download)
- `swiftc` of axpress in the first run (the binary was reused in this run)
- `p18-rig.sh dmg`, `state` and `clean`, plus tar and cat streams over ssh into the VM

Nothing on the host was mounted, opened or copied from `~/Downloads` or `/Applications`. The host's tailscaled and 100.x addresses were never contacted, and there was no host GUI automation. All DMG opens, AXPress clicks and AX dumps happened in `portty-test`. `portty-test-2` was not touched, and neither VM was stopped.

## VM end state

- `portty-test` is running. `/Applications/Portty.app` is the 1.8.1 ad-hoc rig build of `79be910c`, launched (one GUI and its daemon), with no sessions.
- `~/Downloads` holds only `.localized`, and the Trash has no Portty items. No Portty volume is attached.
- The Downloads consent for `app.portty.Portty` is reset. No App Management grant was made.
- `~/p18` is removed (rig DMG, verify-mode, scripts, wins.js, axpress), and fakegh is stopped.

## For later plans

- **18-09 (owns TESTING.md):** add a manual-checklist item for the first install from a DMG in `~/Downloads`:
  - the system prompt "“Portty” would like to access files in your Downloads folder." appears once;
  - the result is already recorded while it is up;
  - Allow sends the DMG to the Trash;
  - Don't Allow keeps it, with the outcome still updated;
  - with no answer, the installer exits after 60 s logging `dmg kept`.

  A Developer ID build should be asked once per Mac. An ad-hoc rig build is asked for every build, so reset the consent before each run. The rig needs a service-wide `tccutil` reset when no app is installed.
- **18-09 mover matrix (R4-a..R4-g):** expect the Downloads prompt on every row whose DMG sits in `~/Downloads`, and answer it with AXPress. Synthetic clicks do not reach it.
- **The unanswered-prompt timeout** (installer exits after 60 s) has only a unit test. If 18-09 wants VM evidence, leave the prompt for more than 60 s. It is not yet known whether the prompt stays on screen after the requesting process exits.

## Known Stubs

None. One shortcut is marked in the code: `DetectDMGRun`'s translocation branch adds `Portty.app` to a folder-shaped origin. P5 showed no translocation, so that mount-from shape is untested.

## Threat Flags

None beyond the register:
- T-18-25: `imageFileOK` is called from ReadJob and again just before the Trash, and only after a successful Detach.
- T-18-26: the mover runs the update's checks before the quarantine is cleared.
- T-18-27: startup branches before consumeInstallResult, the tray and the daemon.
- T-18-28: same or newer opens the installed copy.
- T-18-29: the installer stops only the installed copy's GUI pids, never the DMG GUI.
- T-18-30: the fixture's account name was replaced by `USER`.

New surface the register did not name: the first install asks for Downloads-folder access (decision option A). The prompt is attributed to the installer's run-* copy (bundle ID `app.portty.Portty`).

## Self-Check: PASSED

- Files found: internal/updater/mover.go, internal/updater/mover_test.go, app_mover.go, app_mover_test.go, and every evidence file listed in key-files
- Commits eb902e4d, 6a3fe29c, 151077c4, 7866fea3, 9d4accfc, 4188de71, 4ae1bc50, c1df01da, f6195330, c24700c5, 135c5e20, 1d722d26, f77365ef and 79be910c are ancestors of HEAD
