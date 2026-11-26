---
phase: 18-self-installing-updates
plan: 09
subsystem: updater
tags: [macos, tart-vm, rig, mover, dmg, testing-md, closure]
status: complete

requires:
  - phase: 18-06
    provides: mover mode, the move and open-installed installer kinds, refinement A2
  - phase: 18-07
    provides: the final builds of e03f4ef1 (1.8.0, 1.8.1) and the update, rollback and failure rows
provides:
  - "18-VM-CHECKS.md rows R4-a to R4-g PASS on e03f4ef1, plus R4-e-sandbox OBSERVED and R4-f-gatekeeper BLOCKED"
  - "evidence/R4-a.state .. R4-g.state with -before.state and .log for each row (and the R4-e-sandbox pair)"
  - "p18-rig.sh state: attached_images leaves out the system PKI image, which is now counted as system_images"
  - "TESTING.md: Go 182 / total 315, the Phase 18 note, 24 UPD-05..UPD-09 rows, Category BI M-165..M-171, the M-92 note and M-93 step 3"
  - "The Phase 18 closure record with the human_needed items"
affects: [18.1-04, 19-16, phase-18-verification]

tech-stack:
  added: []
  patterns:
    - "A VM-only precondition that SIP refuses (an unwritable /Applications) is forced on the one process under test with an access() interposer passed through open --env, so LaunchServices still does the launch"

key-files:
  created:
    - .planning/phases/18-self-installing-updates/evidence/R4-a.state (+ R4-a-before.state, R4-a.log; the same set for R4-b to R4-g)
    - .planning/phases/18-self-installing-updates/evidence/R4-e-sandbox.state (+ R4-e-sandbox.log)
  modified:
    - .planning/phases/18-self-installing-updates/18-VM-CHECKS.md
    - .planning/phases/18-self-installing-updates/rig/p18-rig.sh
    - TESTING.md

key-decisions:
  - "The rig's attached_images leaves out the always-attached system PKI trust-store image (now printed as system_images), so the plan's attached_images=0 means no other disk image is attached"
  - "R4-e forces the unwritable /Applications with an access() interposer for the opened app only, because SIP refuses chmod, chflags and ACLs on /Applications in the guest"
  - "R4-f is PASS on its mover conditions; its enforced Gatekeeper half is a separate BLOCKED row (assessments disabled in the guest, the VM's security setting left unchanged), routed to M-169 part 1"
  - "build-script stays 1 in TESTING.md: tests/verify-bundle.test.sh belongs to Phase 18.1, and 18.1-04 records it"
  - "Two carry-over checks were added as any-Mac items (M-170 Downloads prompt, M-171 Put Back), so M-169 stays the only office-Mac item"

requirements-completed: [UPD-08]

estimate:
  tokens: 60000
actuals:
  tokens: 24600
  tasks: 2
  commits: 2
plan_head_before: 7e6555eb8e6a7ceb5ae6d751dce424659ab3886d
plan_head_after: 6262e168c442cd1e378c2813f72588e3fa8e69b1

duration: about 30 min (05:58 to 06:28 UTC)
completed: 2026-11-26
---

# Phase 18 Plan 09: Final mover matrix, TESTING.md and closure Summary

**All seven first-launch cases pass in the VM on 18-07's final build (`e03f4ef1`).** A first install from `~/Downloads` lands in `/Applications`. An older copy is replaced only after the Replace question, and its sessions keep their shells. Cancel opens the installed copy. A same-version DMG just opens what is installed. An unwritable `/Applications` sends the copy to `~/Applications`. A quarantined DMG installs a copy with no quarantine flag. Every case ends with one Portty and one menu-bar item. TESTING.md now records Phase 18's eight new test files, 24 UPD-05..UPD-09 traceability rows and Category BI. M-169, the one office-Mac check, stays open as human_needed together with ROADMAP SC1's office-Mac half.

## Result (Task 1)

| Row | Verdict | Key evidence |
| --- | --- | --- |
| R4-a | PASS | No installed copy. From T0, the installer is running at +2.6 s, the volume is detached and 1.8.1 is in `/Applications` at +4.1 s, and one GUI and its daemon run from `/Applications` at +5.5 s. `updated` was written before the one-time Downloads prompt, and after Allow the DMG was in the Trash within 0.5 s. No staging dir, 0 quarantine lines. Capture: 1.8.1, 1 GUI, attached_images=0, staging_dirs=0 |
| R4-b | PASS | 1.8.0 running with two sessions. The alert `Replace Portty 1.8.0 in Applications?` / `This copy is version 1.8.1.` appeared at +2.2 s above every window. After Replace, new GUI at +2.6 s, handoff 28 ms. **Same shells 35580 and 35585, vim 35608**, fresh viewer `TICK 0..72` with 0 gaps. Old 1.8.0 and the DMG in the Trash. Capture: 1.8.1, 1 GUI, attached_images=0 |
| R4-c | PASS | Cancel opened the running 1.8.0 (same GUI pid, sessions untouched). The DMG stayed mounted and in `~/Downloads`, with nothing staged and no update.log. Capture: 1.8.0, 1 GUI, attached_images=1 |
| R4-d | PASS | 1.8.1 installed, not running. The installer opened it at +2.3 s. Version, mtimes, inode and executable sha256 were unchanged. The volume was detached and the DMG was in the Trash. Capture: 1.8.1, 1 GUI, attached_images=0 |
| R4-e | PASS | `/Applications` reported unwritable to the opened app (interposer; one EACCES logged, from the DMG GUI's check). Installed at `~/Applications/Portty.app`, and GUI and daemon run from there. Capture: 1 GUI, attached_images=0, staging_dirs=0 |
| R4-e-sandbox | OBSERVED | First mechanism (sandbox-exec). The move to `~/Applications` worked, but the inherited profile stopped the installer from running `/bin/ps`, so it logged `failed health` after 60 s. This is a rig artifact; it was superseded |
| R4-f | PASS | Quarantined DMG (`0081;...;Safari;`), not translocated. The system's first-launch confirmation (Open / Show Disk Image / Cancel) appeared, then the mover ran. The installed copy has 0 quarantine lines and its relaunch showed no dialog. Capture: 1.8.1, 1 GUI, attached_images=0 |
| R4-f-gatekeeper | BLOCKED | Gatekeeper assessments are disabled in portty-test. Open Anyway and a real assessment were not produced, and the VM's security setting was left unchanged. The real path is M-169 part 1 |
| R4-g | PASS | Second run from the same DMG while 1.8.1 ran. Result `opened` at +3.7 s. GUI 30157 and daemon 30191 kept their pids, the Info.plist mtime was unchanged, and the DMG was in the Trash. Capture: 1.8.1, 1 GUI, attached_images=0 |

Only R4-a raised a Downloads prompt: its Allow gave this ad-hoc build the consent, so the later rows trash the DMG with no prompt.

## TESTING.md (Task 2)

- **Section 2:** Go **182** (+8), vitest 122, Playwright 10, build-script 1, total **315**, with the Phase 18 note as the first note. It lists the eight new files, the files edited in place, the arm64 asset name (`portty-vX.Y.Z-darwin-arm64.dmg`, user-confirmed 2026-11-25) and where 18-10's, 18-03's and 18-05's tests live. build-script stays 1 because `tests/verify-bundle.test.sh` (Phase 18.1, already merged here) is recorded by 18.1-04, whose plan sets it to 2.
- **Section 4:** 24 rows for UPD-05..UPD-09. They cover the eight new files, update_test.go (asset names), install_test.go (`TestClean`), service_test.go, app_update_test.go, UpdateBanner.test.tsx and App.update.test.tsx. 18-10's `TestStartInstaller_CopiesRunningBundle`, `TestStartInstaller_FailureCleansUp` and `TestCopyBundle_ChecksBeforeUnquarantine` are in the UPD-05 cmd_install_update_test.go row. The UPD-01, UPD-03 and BRND-06 UpdateBanner rows and the UPD-03 App.update row now end with "Portty 18-04: the button is Update; Go installs and reveals only on failure". `tests/check-traceability-paths.sh`: OK, 236 paths.
- **Section 5, Category BI** (after BH): a rig paragraph, M-165 to M-169 and two carry-over items.
  - The rig paragraph covers the two builds from one commit, the vmpreview tag with `$HOME/p18/verify-mode` and `$HOME/p18/fault`, fakegh, portty-test and the rig script path. It also records the VM limits: Gatekeeper assessments disabled, SIP on `/Applications`, and the `hdiutil -mountpoint` deprecation warning on macOS 27.2.
  - M-165 update path, M-166 rollback, M-167 first launch from a DMG, M-168 install failures (with the Residual risk line) and M-169 office Mac. M-169 has part 1 and part 2, the PASS conditions and the closure sentence verbatim.
  - M-170: the one-time Downloads prompt on a real Mac with a Developer ID release (Allow, Don't Allow, no second ask).
  - M-171: Finder's Put Back for the trashed old app. It is unchecked: no Automation grant was made, and the installer cannot write the Trash's `.DS_Store`.
  - M-170 and M-171 can run on any real Mac, so M-169 stays the only office-Mac item.
- **Category AU:** M-93 step 3 now says Update installs and relaunches (reveal only on failure, M-168; an older release without self-install still reveals). A Phase 18 note was added under M-92.

## Phase closure

For the Phase 18 verification, there are two **human_needed** items. Neither is a gap or a failed check.

| Item | Owner | Trigger | What closes it |
| --- | --- | --- | --- |
| **M-169** (TESTING.md Category BI) | the owner, on the office Mac | Part 1 at the first self-installing release; part 2 at the release after it | Part 1: the notarized, browser-downloaded release DMG opened over an installed v1.2.x with live sessions and a share link (mover Replace path, real Gatekeeper and App Management). Part 2: Update clicked in the banner with two ticking sessions and the share link open |
| **ROADMAP SC1, office-Mac half** (Update clicked in the banner on the office Mac) | the owner | The release after the first self-installing release | Satisfied only by M-169 part 2. **M-169 part 1 (the mover Replace path) is not counted as that banner-Update proof** |

Phase 18 closes with M-169 human_needed: ROADMAP SC1's office-Mac half (Update clicked in the banner on the office Mac) stays unmet until part 2 runs; part 1 (the mover Replace path) is not counted as that proof.

On the VM, the four success criteria have all been shown on one build, `e03f4ef1`. SC1's VM half is R1 and R3. SC2 is R2-A and R2-B, with R2-C and R2-D on `b968a6ef`. SC3 is R4-a to R4-g (this plan). SC4 is F1 to F7.

These are open, not blocking, any-Mac items:

- **M-170 (Downloads prompt).** User decision 2026-11-25: the prompt is accepted. The installer writes its result and relaunches before the Trash call, and removes its run folder after it (A2). Don't Allow keeps the DMG and the install succeeds.
- **M-171 (Put Back).** Unchecked: no Automation grant was made, and `.DS_Store` cannot be written in the Trash.

These are not stops:

- **R4-f-gatekeeper BLOCKED:** Gatekeeper assessments are disabled in portty-test. The real behaviour is in M-169 part 1.
- **The `hdiutil attach -mountpoint` deprecation warning** on macOS 27.2: the command still works, and the finding stays deferred.

## Task Commits

1. Task 1: `6c457af7` docs(18-09): final VM mover matrix R4-a to R4-g PASS on e03f4ef1
2. Task 2: `6262e168` docs(18-09): TESTING.md Phase 18 manifest, UPD-05..UPD-09 traceability, Category BI
3. This SUMMARY: the final docs commit

## Verification

- **Task 1 verify block:** passed in full.
  - 7 R4 PASS rows, each citing its own capture.
  - Each capture has its header, `gui_pid_count=1`, the expected installed version and attached-image count, and `staging_dirs=0` for R4-a and R4-e.
  - No 100.x address in the rows or any capture.
- **Task 2 verify block:** passed in full.
  - `check-traceability-paths.sh` OK, Category BI present, M-169 and the closure sentence present.
  - 24 UPD-05..UPD-09 rows and 5 of M-165..M-169.
  - `git ls-files` Go count 182, matching the note's `Go **182**`. `check-brand.sh` clean.
- **Plan verification on the final tree:**
  - `pnpm --dir frontend test`: 122 files, 1835 tests passed.
  - `GOTOOLCHAIN=go1.26.11 go test -race -short ./...`: GO_TEST_RESULT. The frontend was built in the worktree first (`pnpm install --frozen-lockfile`, `pnpm run build`; both gitignored), because `TestNoFakeWhoIsInRelease` needs `frontend/dist`.
- **Leak scan of the new rows, every new capture and log, and the TESTING.md diff:** no 100.x address, no `ts.net`, no `/Users/` path, no `cap=` token, and no owner name. The VM home appears as `~`.
- All Go commands used `GOTOOLCHAIN=go1.26.11`.

## Deviations from Plan

**1. [Rule 1 - Rig bug] `attached_images` counted the system's PKI trust-store image.** macOS keeps `/System/Library/AssetsV2/.../SECUREPKITRUSTSTORE...Cryptex.dmg` attached, so every earlier capture said `attached_images=1` with no Portty image attached. The plan's `attached_images=0` could never hold. `state` now leaves `/System/Library/AssetsV2/` images out of `attached_images` and prints them as `system_images`. The `--- attached images` listing still shows everything. Earlier captures are unchanged, and the header bullet in 18-VM-CHECKS.md says how to read them.

**2. [Rule 3 - Blocking] R4-e mechanism.** SIP refuses the plan's `sudo chmod 755 /Applications`, and an ACL as well (`Operation not permitted`). There was nothing to restore afterwards, because nothing was changed.
- The first try (sandbox-exec, row R4-e-sandbox) moved the copy correctly. But the installer inherited the profile and could not exec `/bin/ps`, so its health check timed out. Cause checked: `execvp() of '/bin/ps' failed: Operation not permitted` inside the profile, while the health request works.
- The recorded R4-e opens the app through LaunchServices with `open --env DYLD_INSERT_LIBRARIES=` and a small interposer (scratch only, not committed). The interposer makes `access()`/`faccessat()` with `W_OK` on `/Applications` return EACCES, the answer `chmod 755` gives a non-root user.
- It works because the rig build is ad-hoc with no hardened runtime. It logged exactly one denial: the DMG GUI's MoveTarget check.

**3. [Rule 3] R4-f.** With Gatekeeper assessments disabled, a quarantined ad-hoc launch shows the system's first-launch confirmation, not a refusal with Open Anyway. I pressed Open and recorded what appeared and that the launch was not translocated. The mover conditions pass. The enforced half is the separate R4-f-gatekeeper BLOCKED row, and the VM's security setting was left unchanged (orchestrator instruction).

**4. [Rule 3] Rig helpers.**
- `axpress` now searches every process with the bundle id, because the installed GUI and the DMG GUI share `app.portty.Portty`. The 18-06 version took the first process, which in R4-b would have been the installed one.
- A new `axextras` helper counts menu-bar items per process. `axdump` does not show the extras bar.
- Both helpers and the VM-side row script (`r9.sh`) live in the session scratchpad (`p1809/`), not in the repo.

**5. [Process] Release asset name.** The rig DMG was copied into `~/Downloads` as `portty-v1.8.1-darwin-arm64.dmg`, the user-confirmed release name. The mover reads the image path from `hdiutil info` and does not depend on the name.

**6. [Process] R4-e-before.state** was written again by the R4-e re-run, from the same empty starting state as the sandbox attempt. The R4-e-sandbox row says so.

**7. [Process] Two carry-over items added as M-170 and M-171** (orchestrator instruction). They can run on any real Mac, so the plan's "exactly one office-Mac item" still holds.

## Open findings for the orchestrator (no STOP)

1. **R4-g leaves `update-result.json` (outcome `opened`) on disk until the next GUI start.** The installed GUI was already running and was only brought forward, so nothing consumed the file. `consumeInstallResult` only logs an `opened` result, so the user sees nothing.
2. **The Replace alert came up at window layer 3 with Finder still the active app.** It was drawn above every window, as the screenshot shows. P4-dialog had seen layer 8 with Portty frontmost. The difference is probably that the installed GUI was running here. On a real Mac a click would still be needed to make Portty active before the keyboard reaches the alert.
3. **The volume eject took 6.6 to 7 s** in R4-e and R4-f, measured from the installer starting to the volume being gone. That matches P6's slow first detach. No deadline was close.

## Host commands run

- `swiftc` (the `axpress` and `axextras` helpers) and `clang` (the interposer dylib), all in the session scratchpad.
- `p18-rig.sh dmg`, `deploy-n`, `state` and `clean`, with tar and cat streams into the VM's `~/p18`.
- `pnpm install --frozen-lockfile`, `pnpm run build` and `pnpm --dir frontend test`.
- `go vet` and `go test` with `GOTOOLCHAIN=go1.26.11`, `tests/check-traceability-paths.sh` and `tests/check-brand.sh`.
- No build was redone: the 18-07 builds in `p1807/rig/` were reused, and their executable sha256 prefixes were checked against 18-07's record.

Nothing on the host was mounted, opened or copied from `~/Downloads` or `/Applications`. Every DMG open, `hdiutil`, `chmod` attempt, Trash check and `tccutil` call ran inside portty-test. No host tailscaled or 100.x address was contacted, and there was no host GUI automation.

## VM end state

- portty-test was never stopped. portty-test-2 was not touched.
- `/Applications/Portty.app` is the final 1.8.1 rig build of `e03f4ef1` (from R4-f), running with one GUI, daemon `v1.8.1` and no sessions.
- `~/Applications` is empty. Trash and `~/Downloads` hold no Portty items, and no Portty volume or image is attached.
- `/Applications` is still `drwxrwxr-x root:admin`, unchanged. No App Management grant was made.
- The Downloads consent was reset service-wide. `~/p18` (helpers, interposer, DMG) is removed and fakegh is stopped.
- Finder's `updates` window is still open.

## Known Stubs

None.

## Threat Flags

None. The interposer and helpers are rig-only scratch files and were never committed. T-18-38 holds (every DMG, hdiutil, chmod and Trash action ran in the VM). T-18-39 holds (leak scan clean). T-18-40 holds (the closure sentence is in TESTING.md M-169 and in this record).
