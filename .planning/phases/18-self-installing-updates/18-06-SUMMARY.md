---
phase: 18-self-installing-updates
plan: 06
subsystem: updater
tags: [macos, mover, dmg, hdiutil, tcc, installer, tart-vm, rig]
status: stopped

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
  - "internal/updater/installer.go: KindMove, KindOpenInstalled, OutcomeOpened, checkMoveJob, imageFileOK, moveParents/volumesRoot seams, the move and open-installed branches of RunInstaller (one trashImage guard)"
  - "internal/updater/service.go: InstallFailureReason"
  - "app.go: moverMode, detectMoverFunc, moverStartFunc, the startup branch and the domReady early return"
  - "app_mover.go: startMover, detectMover, statfsOrigin, runMover, moverStdoutRunner, moverRoots, openAppPathFunc"
  - "18-VM-CHECKS.md rows P4, P5, P6, R4-smoke (FAIL), R4-smoke-deny, P4-dialog"
affects: [18-07, 18-09]

tech-stack:
  added: []
  patterns:
    - "hdiutil output read through a stdout-only runner, written to a 0600 temp file and converted with plutil (no plist library)"
    - "Mover mode decides before anything that could pin the volume, then reuses the update installer with two new job kinds"

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
  modified:
    - internal/updater/installer.go
    - internal/updater/installer_test.go
    - internal/updater/service.go
    - cmd_install_update.go
    - app.go
    - .planning/phases/18-self-installing-updates/18-VM-CHECKS.md

key-decisions:
  - "P5 CONFIRMED: a DMG launch runs from /Volumes, not translocated, so detection reads the path; the statfs branch stays as an unexercised guard"
  - "P4: opening the volume's app starts a second GUI beside a running installed one, so the Replace path stops the installed GUI's pids before the swap"
  - "An unreadable installed version is written to the job as 0.0.0 once the user answers Replace, so the never-downgrade rule still holds for every readable version"
  - "The DMG GUI is never signalled: a mover job whose DMG GUI outlives QuitWait ends failed quit-timeout with nothing changed"
  - "A failed first install (no older copy) leaves the verified new copy installed and the DMG in place; it never relaunches a target that does not exist"

requirements-completed: []

estimate:
  tokens: 90000
actuals:
  tokens: 25100
  tasks: 3
  commits: 8
plan_head_before: 4f85e69934abec4192c9fe5f0dc8ce3b946f4d50
plan_head_after: c1df01da4b3875fba9d1d5efdbb3d0049a3408d3

duration: about 75 min
completed: 2026-11-26
---

# Phase 18 Plan 06: First launch from a DMG moves Portty into place (STOPPED at R4-smoke)

## STOP 2 (re-execution after decision option A, 2026-11-26): the run-folder order kills the prompt

**What I did:** I applied option A test-first. RED is `c24700c5` and GREEN is `135c5e20`. On the move and open-installed kinds, the installer now writes its final result, calls `env.RemoveRunDir`, and only then trashes the DMG. The relaunch still comes first, and the Trash runs in a goroutine bounded by `trashWait` (60 s). The unit tests pass, including `-race`. Then I ran R4-smoke on a 1.8.1 rig build of `135c5e20` (row R4-smoke-rundir-first).

**What happened:** The move worked again: 1.8.1 ran from `/Applications`, with one GUI and its daemon there, the volume ejected, and no staging dir. The result `updated` was written at 04:43:57. But **no Downloads prompt appeared**. Removing the run folder deletes the installer's own executable, because the installer runs from `run-*/Portty.app`. tccd then logged `proc_pidpath_audittoken() failed from PID[73924]: (#2) No such file or directory` and refused the access without asking, with 0 `AUTHREQ_PROMPTING` lines. The Trash returned "you don't have permission" at once, the installer logged `dmg kept` and exited, and the DMG stayed in `~/Downloads`. With this order, a DMG in Downloads is never trashed and the user is never asked. The decision expected a prompt followed by Allow and the Trash, so its two parts cannot both hold. Under RULE 0 I did not run the Allow or Don't Allow runs, and I did not change the order on my own.

**Options:**

| Option | Change | Pros | Cons |
| --- | --- | --- | --- |
| A2 (recommended) | Keep "result written before the Trash" and the 60 s bound, but remove the run folder **after** the Trash returns or the bound expires. This moves one call in `finishTrash`, and the tests change to "result before Trash, run dir after". | The prompt works, as in the first R4-smoke, where tccd resolved the run-* binary. An unanswered prompt still leaves the outcome recorded, and the bound means the run dir lives at most about 60 s after the result. If the process dies first, the GUI's startup sweep removes run-* dirs older than its age floor. | It amends the decision's "run folder removed before the Trash". The run dir exists while the prompt is up, but nothing reads it then. |
| B2 | Keep the order as committed and accept that a Downloads DMG is never trashed. Make that explicit in code (skip the Trash for a DMG under `~/Downloads`) and amend UPD-08. | No prompt ever. | It relies on, or replaces, a TCC failure mode. UPD-08's "moves the downloaded DMG to the Trash" would almost never happen. This is old option B again. |
| C2 | Have the relaunched GUI (`/Applications/Portty.app`, a stable path) trash the DMG named in the result file. | The prompt is attributed to the installed app. | A new result field, a GUI-side copy of the image guard, and the trash moves out of the installer. Much more change than A2. |

**Recommendation: A2.** It keeps everything the decision asked for, except where the run folder is removed, and the evidence says that one point cannot hold. Once you choose, a continuation needs to: move one line (A2), update the ordering test, rebuild, and run R4-smoke (Allow) and the Don't Allow re-check under the decision's PASS rule.

**Commits in this re-execution:** `c24700c5` test(18-06) RED, `135c5e20` fix(18-06) GREEN, plus the docs commit that carries this section, the R4-smoke-rundir-first row and its evidence.

**RED evidence (#3770):** `go test -v -run TestInstaller_Mover ./internal/updater/` exited 1. TestInstaller_MoverResultAndRunDirBeforeTrash failed in all 3 subtests ("result file at the image Trash ... present false", "run folder still there"), and TestInstaller_MoverTrashBounded failed after blocking 5 s. TestInstaller_MoverTrashFailureKeepsOutcome passed: it guards the existing Don't Allow behaviour. The classifier returned RED_EVIDENCE_OK. Semantic assessment: the target compiled (with inert stubs for the new field and var) and failed on the planned ordering assertions.

**Verification of the code (fixed tree, `135c5e20`):** gofmt is clean, `go vet ./internal/updater/ .` passes, golangci-lint `--new-from-rev=c714c8ef` reports 0 issues, `GOTOOLCHAIN=go1.26.11 go test -race -count=1 ./internal/updater/ .` passes, and check-brand is clean.

**VM end state (STOP 2):** `portty-test` is running. `/Applications/Portty.app` is the 1.8.1 rig build of `135c5e20`, running (GUI 74447 and daemon 74463), with no sessions. `~/Downloads` holds `portty-v1.8.1-darwin-universal.dmg` (the refused Trash left it). No Portty volume is attached, and the Trash has no Portty items. `~/p18` holds the rig DMG, verify-mode, the run scripts, wins.js and axpress. The Downloads consent has no Portty entry (it was reset, and no prompt was ever answered). fakegh is not running. I did not restore the 1.8.0 rig or clean `~/p18`, so the continuation can re-run from here. `portty-test-2` was not touched, and neither VM was stopped.

**For later plans (18-09):** the first install from a DMG in Downloads shows the system prompt "“Portty” would like to access files in your Downloads folder." once (A5), so TESTING.md's manual checklist needs an item for it. Its exact expected behaviour depends on the option chosen above.

---

## STOP 1 (first execution)

**Opening Portty from a mounted DMG now installs it: mover mode starts before any daemon, tray or window work, stages a checked copy, and hands it to the installer. The installer ejects the volume, puts the copy in /Applications, relaunches it and trashes the DMG. In the VM the move itself worked end to end in under 9 s. But trashing a DMG that sits in `~/Downloads` raises the macOS prompt "“Portty” would like to access files in your Downloads folder.", and the installer waits on it. That breaks two R4-smoke PASS conditions (no dialog, DMG in the Trash within 60 s), so the plan stopped for a decision.**

## STOP: decision needed

**What happened (R4-smoke, `evidence/R4-smoke.state`, `evidence/R4-smoke-extra.log`):** the volume's GUI started no daemon and asked nothing. By T0+8.7 s, 1.8.1 ran from `/Applications`, with exactly one GUI and its daemon there. The volume was ejected, and no staging or run dir was left. At T0+11.6 s the installer's `trashItem` of `~/Downloads/portty-v1.8.1-darwin-universal.dmg` raised the system Downloads-folder prompt. tccd logged `AUTHREQ_PROMPTING ... kTCCServiceSystemPolicyDownloadsFolder, subject=Sub:{app.portty.Portty}`, with the installer's run-* copy as the requesting binary. The installer blocked until the prompt was answered, so it wrote no result and kept its run dir for 4 min 10 s. Then I pressed Allow, and the DMG went to the Trash. With Don't Allow (row R4-smoke-deny), the refusal is non-fatal: `installer: dmg kept: ...` then `installer: updated`, and the DMG stays in Downloads.

**Why this needs you:** CONTEXT says a first install moves "without asking" and trashes the download. On macOS, any Portty process that moves a file out of `~/Downloads` needs the Downloads-folder consent once. A Developer ID build keeps the answer for its bundle ID and Team, so a real user would see the prompt once per Mac, on the very first install. An ad-hoc rig build is asked again for every build. This is research Assumption A5. The two CONTEXT decisions conflict, and neither the plan nor CONTEXT says which one gives way.

**Options:**

| Option | What changes | Pros | Cons |
| --- | --- | --- | --- |
| A (recommended) | Keep trashing the DMG and accept a one-time Downloads prompt on a first install. Make the installer write its result and remove its run dir before the Trash call, so a prompt that is never answered does not leave the installer hanging with nothing recorded. R4-smoke PASS is then judged with "the only dialog is the one-time system Downloads prompt, answered Allow". | Keeps UPD-08 as written. Refusal already works (R4-smoke-deny). Small code change. | A first-time user sees one system prompt, and Portty asks for Downloads access, which may look odd for a terminal app. |
| B | Do not trash a DMG that is inside a privacy-protected folder (Downloads, Desktop, Documents). Eject only, and leave the DMG. | No prompt ever. | Amends UPD-08 ("moves the downloaded DMG to the Trash"), and most DMGs live in Downloads, so the trash step would almost never run. |
| C | Ask Finder to trash it through Apple Events. | Finder already has Downloads access. | It is still a prompt (Automation: "Portty wants to control Finder"), and it needs an usage-description key and a new entitlement path. Strictly worse than A. |

**Recommendation: A.** Before applying it I will not change the code. It needs your answer, because A means a privacy prompt on first install, and B means amending UPD-08.

## Result

| Row | Verdict | One line |
| --- | --- | --- |
| P5 | CONFIRMED | A quarantined v1.2.0 DMG opened Finder-style runs from `/Volumes/Portty/...`, not translocated (0 AppTranslocation mounts, `lsof` txt on `/Volumes`). The first-launch dialog needed an AXPress; synthetic clicks do not reach it |
| P6 | OBSERVED | Plain `hdiutil detach` succeeds on the first try after the volume's GUI and daemon exit: 10 676 ms on the first detach of a fresh download (one blocking call), then 118 ms with a Finder window open, and 57 ms without one. `-force` was never needed. The fixture was captured here |
| P4 | OBSERVED | Opening the volume's app while the installed copy runs starts a second GUI, from `/Volumes`. A plain `open /Applications/Portty.app` after the old pid is gone started exactly one GUI in 3 of 3 runs |
| R4-smoke | **FAIL** | The move works (above). The Downloads privacy prompt appears, and the installer blocks until it is answered |
| R4-smoke-deny | OBSERVED | Don't Allow: the DMG is kept, the outcome is `updated`, and the run dir is removed |
| P4-dialog | OBSERVED | The Replace question (`Replace Portty 1.8.0 in Applications?`, `This copy is version 1.8.1.`) shows within 1 s, frontmost at layer 8. Cancel opens the installed 1.8.0 and leaves the DMG mounted |

## Task Commits

1. Task 1 RED: `eb902e4d` test(18-06): add failing tests for disk-image launch detection
2. Task 1 GREEN: `6a3fe29c` feat(18-06): detect a disk-image launch and record probes P4-P6
3. Task 2 RED: `151077c4` test(18-06): add failing tests for the installer's move and open-installed kinds
4. Task 2 GREEN: `7866fea3` feat(18-06): installer move and open-installed kinds
5. Task 3 RED: `9d4accfc` test(18-06): add failing tests for mover mode at GUI startup
6. Task 3 GREEN: `4188de71` feat(18-06): mover mode at GUI startup with the Replace question
7. Fix: `4ae1bc50` fix(18-06): keep the mover installer tests' result file in a temp dir
8. Task 3 VM rows: `c1df01da` docs(18-06): R4-smoke FAIL on the Downloads privacy prompt, deny and dialog rows

## TDD Gate Compliance

- Task 1: RED `eb902e4d` (`undefined: ParseHdiutilInfo`), then GREEN `6a3fe29c`.
- Task 2: RED `151077c4` (`undefined: moveParents`), then GREEN `7866fea3`. A mutation check that disabled the detach guard made TestInstaller_MoveDetachFailureNeverTrashesImage fail, as it should.
- Task 3: RED `9d4accfc` (`unknown field detectMoverFunc`), then GREEN `4188de71`.
- REFACTOR: none.

## Verification

- Task 1 verify: passed in full. gofmt and vet are clean, 4 or more tests are listed, the race tests pass, and there are 3 probe rows.
- Task 2 verify: passed in full. gofmt and vet are clean, 11 move-kind tests are listed, and the race tests pass. `grep -c KindOpenInstalled internal/updater/installer.go` = 5. The 18-03 and 18-05 installer tests and TestSelfInstall_UpdateHappyPath still pass.
- Task 3 verify: every check passes except the R4-smoke PASS grep, which is 0 (the row is FAIL):
  - gofmt is clean, `go vet ./internal/updater/ .` passes, and golangci-lint `--new-from-rev=c714c8ef` reports 0 issues.
  - 8 TestMover/TestRunMover tests are listed, and they pass under `-race` with TestDomReady, TestStartDaemon, TestConsumeInstallResult, TestSweep and TestEnsure.
  - `moverMode` is in app.go, and `Replace Portty` is in app_mover.go.
  - check-brand is clean. check-traceability-paths passes (228 paths).
  - `go vet -tags vmpreview .` passes.
- Full `GOTOOLCHAIN=go1.26.11 go test -race -count=1 ./internal/updater/ .` passes.
- Every Go command ran with `GOTOOLCHAIN=go1.26.11`.

## Deviations from Plan

**1. [Rule 1 - Bug] Test fixture wrote update-result.json into the package folder.** A mover job has no cache DMG, so rbFix's result path (beside `job.DMG`) resolved to `.`. rbFix now has its own `resDir`, and the stray file was deleted. Commit `4ae1bc50`.

**2. [Test design] The normal-path startup test does not run the whole `startup`.** The full startup would start the tray, the sweep (which lists the host's `/Applications`) and the Tailscale health poller (which would contact the host's tailscaled). Instead, the branch is a separate `startMover()` call at the top of startup. TestMoverStartup_NoRunKeepsNormalPath checks that `startMover` declines, that startupDone stays open, and that `startDaemon` then reaches the ensure seam. The mover-mode test does run the real `startup`.

**3. [Placement] The move-kind installer tests are in installer_test.go, as the plan says.** No extra test file was added. `mover_test.go` and `app_mover_test.go` are new test files that the plan lists. TESTING.md was not touched, because 18-09 owns it (same as 18-10).

**4. [Rule 2] `finishOpen` skips the relaunch when the target does not exist.** This applies to a first install that fails before its swap (reverify or swap). Without the skip, the installer would spend six open retries on a path that does not exist.

**5. [Tooling] AXPress helper.** Synthetic CGEvent clicks do not reach the system's first-launch dialog or the Downloads privacy prompt, so I built a 15-line Swift `axpress` on the host (in scratch) and copied it to the VM's `~/p18`. It was removed with `~/p18` afterwards.

**6. [Process] Plan commit ledger.** The worktree guard refuses writes under `.git/worktrees`, as it did in 18-10. `plan_head_before` is the dispatch base `4f85e699`, and `commits: 8` is `git log 4f85e699..HEAD` before this SUMMARY's own commit.

## Host commands run (all of them)

- `pnpm install --frozen-lockfile` and `pnpm run build` in `frontend/` (both outputs are gitignored)
- gofmt, `go vet` (default and `-tags vmpreview`), golangci-lint, `go test`, check-brand, check-traceability-paths
- `p18-rig.sh build 1.8.1` and `build 1.8.0` into the session scratch dir. Each runs wails build, jq, cp, and the restore of wails.json and frontend/src/wailsjs; the tree was clean afterwards.
- `shasum -a 256 -c` of the existing verified v1.2.0 scratch download (no new download)
- `swiftc` of the axpress helper in scratch
- `p18-rig.sh dmg`, `deploy-n` and `state`, plus tar and cat streams over ssh into the VM

Nothing on the host was mounted, opened or copied from `~/Downloads` or `/Applications`. The host's tailscaled and 100.x addresses were never contacted, and there was no host GUI automation. All DMG opens, clicks and axdumps happened in `portty-test`. `portty-test-2` was not touched, and neither VM was stopped.

## VM end state

- `portty-test` is running. `/Applications/Portty.app` is the 1.8.0 ad-hoc rig build of `4188de71`, launched (one GUI, its daemon), with no sessions.
- No Portty volume is attached. `~/Downloads` holds only `.localized`, and the Trash holds no Portty items.
- `~/p18` is removed (along with verify-mode, wins.js and axpress), and fakegh is not running.
- The Downloads consent for `app.portty.Portty` was reset with `tccutil`. No App Management grant was made.

## Known Stubs

None. One shortcut is marked in the code: `DetectDMGRun`'s translocation branch adds `Portty.app` to a folder-shaped origin. P5 showed no translocation, so that mount-from shape is untested.

## Threat Flags

None beyond the register:
- T-18-25: `imageFileOK` is called from ReadJob and again just before the Trash, and only after a successful Detach.
- T-18-26: the mover runs the update's checks before the quarantine is cleared.
- T-18-27: startup branches before consumeInstallResult, the tray and the daemon.
- T-18-28: same or newer opens the installed copy.
- T-18-29: the installer stops only the installed copy's GUI pids and never the DMG GUI.
- T-18-30: the fixture's account name was replaced by `USER`.

New surface the register did not name: the first install asks for Downloads-folder access (above).

## Self-Check: PASSED

- Files found: internal/updater/mover.go, internal/updater/mover_test.go, app_mover.go, app_mover_test.go, evidence/R4-smoke.state, evidence/R4-smoke-deny.state, evidence/R4-smoke-extra.log
- Commits eb902e4d, 6a3fe29c, 151077c4, 7866fea3, 9d4accfc, 4188de71, 4ae1bc50 and c1df01da are ancestors of HEAD
