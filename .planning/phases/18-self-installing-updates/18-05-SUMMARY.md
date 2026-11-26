---
phase: 18-self-installing-updates
plan: 05
subsystem: updater
tags: [macos, installer, trash, nsfilemanager, cgo, startup-sweep, tart-vm, rig]
status: complete

requires:
  - phase: 18-03
    provides: RunInstaller with rollback, the rb test fixture, the R2 rig pollers, R2-C's leftover staging copy
  - phase: 18-10
    provides: the whole-bundle installer copy under update/run-*/Portty.app
provides:
  - "trash_darwin.m / trash_darwin.go: porttyTrashItem (NSFileManager trashItemAtURL, Foundation only) and trashItem, which refuses in a test binary"
  - "installer.go: healthy branch trashes j.Staged (the old bundle) only inside a sibling .Portty-update- dir, then os.Remove of the empty dir; a refused Trash logs 'old copy kept aside'"
  - "cmd_install_update.go: Trash is trashItem in the real env"
  - "app_install.go: sweepAge, trashItemFunc, sweepParents, sweepUpdateLeftovers, sweepDirs, isRealDir"
  - "app.go: startup runs go sweepUpdateLeftovers(ctx, time.Now()) after startDaemon"
  - "rig/p18-rig.sh state: trash_portty count and Trash listing"
  - "18-VM-CHECKS.md rows R1-trash PASS, P3 CONFIRMED, S1 PASS"
affects: [18-06, 18-07, 18-09]

tech-stack:
  added: []
  patterns:
    - "Every possible copy of the user's app goes to the Trash, never RemoveAll; RemoveAll only for Portty's own run-* dirs under updateWorkDir()"
    - "Startup cleanup runs off the startup path, after the daemon is up, with a 1 hour age floor so it never touches a live install"

key-files:
  created:
    - trash_darwin.go
    - trash_darwin.m
    - .planning/phases/18-self-installing-updates/evidence/R1-trash.state
    - .planning/phases/18-self-installing-updates/evidence/R1-trash-before.state
    - .planning/phases/18-self-installing-updates/evidence/R1-trash-timeline.log
    - .planning/phases/18-self-installing-updates/evidence/P3-tcc.log
    - .planning/phases/18-self-installing-updates/evidence/S1.state
    - .planning/phases/18-self-installing-updates/evidence/S1-before.state
    - .planning/phases/18-self-installing-updates/evidence/S1-launch.log
  modified:
    - internal/updater/installer.go
    - internal/updater/installer_test.go
    - cmd_install_update.go
    - app_install.go
    - app.go
    - app_install_test.go
    - .planning/phases/18-self-installing-updates/rig/p18-rig.sh
    - .planning/phases/18-self-installing-updates/18-VM-CHECKS.md

key-decisions:
  - "The installer guards Trash twice: the staged path's parent must start with StagingPrefix and sit beside the target. A path outside that is never trashed, even on a healthy update"
  - "The sweep's inner sweepDirs takes the parents, work and cache dirs as parameters, so its tests run on t.TempDir() only. sweepUpdateLeftovers passes the real dirs"
  - "The sweep writes its summary line to update.log only when it did something. A refused Trash logs every launch (T-18-23)"
  - "The installer's 'kept aside' log carries macOS's error description, which names the item, not its path"

requirements-completed: [UPD-06, UPD-05]

estimate:
  tokens: 60000
actuals:
  tokens: 14700
  tasks: 2
  commits: 5
plan_head_before: a9382cf430a8eaa8fe77f623499ee164c5635ca0
plan_head_after: d36e1dd3b2b1d071d6fb766ecdaaecfcf254d9c5

duration: about 15 min (01:57 to 02:12 UTC)
completed: 2026-11-26
---

# Phase 18 Plan 05: Trash the old app, sweep interrupted installs Summary

**After a healthy update, the installer moves the old Portty.app to the user's Trash. It uses NSFileManager trashItemAtURL from its own non-AppKit process, with no prompt. Every normal GUI start then sweeps up after interrupted installs: stale staging copies go to the Trash (never deleted), stale installer run folders are removed, and stale mounts are detached. All three VM rows pass.**

## Result

| Row | Verdict | One line |
| --- | --- | --- |
| R1-trash | PASS | 1.8.0 to 1.8.1 with two live sessions. The Trash then holds `Portty.app 1.8.0`, with `staging_dirs=0`, `run_dirs=0` and `cache_dmgs=0`. Session ids and shell pids are unchanged, and the ticker is gapless (TICK 0..73). No new window appeared in 28 all-app window samples |
| P3 | CONFIRMED | A4 holds. tccd shows no AppBundles request, and the installer's only SystemPolicy line is a sandboxd AllFiles preflight with no prompt. Side finding: the Trash's `.DS_Store` write was refused, so Finder's Put Back may be unavailable (not tested) |
| S1 | PASS | Within 1.05 s of the launch: the stale staging copy was in the Trash and its dir was gone, the fresh copy was untouched, `run-stale1` was gone with nothing from it in the Trash, and `mnt-s1` was detached. `update.log`: `update: sweep trashed 1, kept aside 0, run dirs removed 1, mounts detached 1` |

All rows ran on ad-hoc rig builds of `a84ffb3d` (verify-mode adhoc, un-granted). S1 counts as pre-final evidence for the 18-07 closure unless it is re-run.

## Task Commits

1. Task 1 RED: `bf7eb5af` test(18-05): add failing installer Trash and trashItem refusal tests
2. Task 1 GREEN: `5fd57c47` feat(18-05): installer trashes the old Portty.app once the new one is healthy
3. Task 2 RED: `c3a1c744` test(18-05): add failing startup sweep tests
4. Task 2 GREEN: `a84ffb3d` feat(18-05): startup sweep of interrupted-install leftovers
5. Task 2 rows: `d36e1dd3` docs(18-05): VM rows R1-trash and S1 PASS, P3 CONFIRMED on ad-hoc rig builds

## TDD Gate Compliance

- Task 1: RED `bf7eb5af` failed at runtime (`Trash calls = [], want exactly one`; `logs = ["installer: updated "]`), and `go vet .` failed with `undefined: trashItem`. GREEN `5fd57c47`: both installer tests and TestTrashItem_RefusesUnderTest pass. TestSelfInstall_UpdateHappyPath passes unedited.
- Task 2: RED `c3a1c744` failed to compile (`undefined: trashItemFunc`). GREEN `a84ffb3d`: all 6 sweep tests pass.
- REFACTOR: none needed.

## Verification

All Go commands were run with `GOTOOLCHAIN=go1.26.11`.

- Task 1 verify block passed in full:
  - gofmt is clean and vet passes
  - 2 of 2 trash tests are listed
  - `go test -race ./internal/updater/` passes, and so does `. -run 'TestTrashItem|TestInstallUpdate'`
  - `trashItemAtURL` appears 2 times in trash_darwin.m, `testing.Testing()` once in trash_darwin.go, and `Trash:` once in cmd_install_update.go
- Task 2 verify block passed in full:
  - gofmt is clean and vet passes
  - golangci-lint `--new-from-rev=c714c8ef` reports 0 issues
  - 6 TestSweep tests are listed, including TestSweep_StaleRunDirWithBundleCopy
  - The race run of sweep, consume, ensure, startDaemon and domReady tests passes
  - `sweepUpdateLeftovers` appears once in app.go
  - The doc greps find 2 PASS rows (R1-trash, S1) and 1 P3 verdict row
- Plan verification: `go test -race -count=1 ./internal/updater/ .` passes (20.7 s for the main package).
- tests/check-brand.sh is clean. The new rows and evidence contain no `/Users/`, 100.x address or ts.net name.
- **RemoveAll review (acceptance):** app_install.go has two RemoveAll calls.
  - Line 82 is startInstaller's existing cleanup of its own fresh run dir.
  - Line 280 is the sweep. It runs only on `run-*` entries of `work` (updateWorkDir()), each confirmed by Lstat as a real directory older than sweepAge.
  - Nothing in sweepParents() (the folders that hold the user's Portty.app) is ever passed to RemoveAll. Those staging dirs only see trashItemFunc on `Portty.app` and `Portty.staged`, then a plain os.Remove that fails while anything is still inside.

## Deviations from Plan

**1. [Rule 2 - Evidence] The rig `state` capture prints the Trash.** p18-rig.sh is not in files_modified. I added a `trash_portty` count and a Trash listing (name, CFBundleShortVersionString, ctime) so the R1-trash and S1 captures prove the Trash contents themselves, as the W1 rule asks (each PASS row cites a ROW.state). The change is read-only in the VM, and later rows (18-06, 18-07) get it too.

**2. [Interpretation] Sweep seam.** The plan allows "fake parents injected through a parameter or seam". sweepDirs takes the parents, work and cache dirs as parameters, so the tests touch only t.TempDir() (UPD-06 prohibition). trashItemFunc and updateRunner are swapped and restored with t.Cleanup. sweepParents is tested through the existing runningBundleFunc seam.

**3. [Process] Fresh rig scratch dir.** The shared scratchpad's `p18-rig/n180` already held an older build, and the rig refuses to overwrite one. The 18-05 builds went to `scratchpad/p1805/rig` instead, and fakegh was rebuilt there.

**4. [Process] Frontend built in the worktree** (`pnpm install --frozen-lockfile`, `pnpm run build`; both gitignored), as wails build needs it.

## Findings for later plans (not fixed here)

- **Put Back (P3):** the installer could not write `~/.Trash/.DS_Store`, which is where Finder keeps Put Back data. The bundle is in the Trash and can be dragged out, but Finder's "Put Back" may not offer it. Testing that would need Finder automation in the VM (an Automation grant), so it was left alone. Candidate for 18-07's manual checklist.
- **hdiutil deprecation (S1):** on macOS 27.2, `hdiutil attach ... -mountpoint` prints a deprecation warning pointing to `diskutil image attach`. It still works. updater.Mount uses this exact command. Logged here for a future plan; nothing is changed.
- Trash entry naming: a second `Portty.app` in the Trash is named `Portty HH-MM-SS-mmm.app` by macOS. Rows that count Trash entries should glob `Portty*.app`, as the rig now does.

## Host commands run

- gofmt, go vet, golangci-lint, go test
- tests/check-brand.sh
- `pnpm install` and `pnpm run build` in frontend/
- `p18-rig.sh build 1.8.0` and `build 1.8.1` into the scratchpad. The wails.json and wailsjs restores left the tree clean.
- plutil, `--version` and a byte grep on the scratch builds
- `p18-rig.sh` deploy-n, dmg, feed, state, quit-gui and clean (every effect inside the VM over vm.sh)
- tar and cat streams of my helper scripts (r2.sh, cli.exp, vtick.vim from 18-03, plus s1.sh and p3.sh) into the VM's `~/p18`
- git add and commit on explicit paths

Nothing on the host was mounted, opened or copied from `~/Downloads` or `/Applications`. No host tailscaled or 100.x address was contacted, and there was no host GUI automation (clicks and axdump only inside the VM). portty-test was never stopped, and portty-test-2 was not touched.

## VM end state

- `/Applications/Portty.app` is the ad-hoc 1.8.1 rig build of `a84ffb3d`, running: GUI 55442 and daemon 53362 at `v1.8.1`, no sessions (tickA and vimB were killed after S1).
- No `.Portty-update-*` dir (fresh1 was removed after the S1 capture), no `run-*` dir, no mount from the cache.
- fakegh is stopped and `~/p18` is removed. No fault file and no uchg flag.
- The VM user's Trash holds `Portty.app` (1.8.0, from R1-trash) and `Portty 02-07-51-507.app` (1.8.1, from S1). I left them there as evidence. Empty the Trash before any row that counts Trash entries from zero.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register:
- T-18-21: trashing is limited to Portty.app or Portty.staged inside a .Portty-update- dir. The installer requires that dir to sit beside the target. The sweep requires it to be older than an hour and in the fixed parent list. RemoveAll is only used for run- dirs under updateWorkDir(), and symlinks are skipped. Tests cover all of this.
- T-18-22: the 1 hour floor holds, and the sweep runs after startDaemon in a goroutine.
- T-18-23: a refused Trash is logged on every launch.
- T-18-24: no AppleEvent is used, and P3 recorded no prompt.

TESTING.md was not touched. No new test file was added: the tests went into the existing installer_test.go and app_install_test.go. 18-09 owns the Phase 18 manifest and traceability rows.

## Self-Check: PASSED

- Files found: trash_darwin.go, trash_darwin.m, internal/updater/installer.go, internal/updater/installer_test.go, cmd_install_update.go, app_install.go, app.go, app_install_test.go, rig/p18-rig.sh, 18-VM-CHECKS.md, evidence/R1-trash.state, evidence/R1-trash-before.state, evidence/R1-trash-timeline.log, evidence/P3-tcc.log, evidence/S1.state, evidence/S1-before.state, evidence/S1-launch.log
- Commits bf7eb5af, 5fd57c47, c3a1c744, a84ffb3d and d36e1dd3 are ancestors of HEAD
