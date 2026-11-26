---
phase: 18-self-installing-updates
fixed_at: 2026-11-26T06:54:47Z
review_path: .planning/phases/18-self-installing-updates/18-REVIEW.md
iteration: 1
findings_in_scope: 6
fixed: 6
skipped: 0
status: all_fixed
---

# Phase 18: Code Review Fix Report

**Fixed at:** 2026-11-26T06:54:47Z
**Source review:** .planning/phases/18-self-installing-updates/18-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 6 (CR-01, WR-01..WR-05; the Info items were left alone)
- Fixed: 6
- Skipped: 0

## Fixed Issues

### CR-01: The mover refuses the DMG that the Reveal fallback hands the user

**Files modified:** `internal/updater/installer.go`, `internal/updater/installer_test.go`
**Commit:** 5326b5b6
**Applied fix:** `checkMoveJob` no longer refuses a job whose image path fails `imageFileOK`. It now only asks for a clean absolute path, or no path at all. `finishTrash` still re-checks `imageFileOK` before it trashes anything, so a DMG in the update cache, a renamed DMG or another account's DMG is now moved from and then kept. The Trash step keeps the same safety as before.
**Tests:** In `TestJob_MoveKinds`, the cache, renamed, directory, symlink, missing and empty image-path rows now expect the job to be accepted. A new row refuses an image path that is not clean. `TestInstaller_MoveImageGuard/<kind> image in the update cache` covers both kinds: `ReadJob` accepts the job, it swaps (move), and the DMG is kept.

### WR-01: GUI matching ignores the process owner

**Files modified:** `internal/updater/installer.go`, `internal/updater/installer_test.go`, `cmd_install_update.go`, `TESTING.md`
**Commit:** 16831a98
**Applied fix:** `realGUIPids` now runs `ps -axww -o pid=,uid=,args=`. `ParseGUIPids(psOut, exe, uid)` keeps only lines whose uid equals `os.Getuid()`. The health check, `stopGUIs` and the quit-timeout SIGKILL all get their pids from `GUIPids`, so all of them now see only this user's GUIs. In TESTING.md, the description of `TestParseGUIPids` in the UPD-05 row was updated (no path change).
**Tests:** `TestParseGUIPids` now uses uid-column fixtures. Lines from other accounts (uid 502 and 0), a line with a non-numeric uid and a line with no uid column are all dropped. Asking for uid 502 returns that account's GUIs.

### WR-02: Mover failures after the GUI quits are silent

**Files modified:** `internal/updater/installer.go`, `internal/updater/installer_test.go`, `app_install.go`, `app_install_test.go`
**Commit:** 502654d8
**Status:** fixed: requires human verification (this changes failure-path logic)
**Applied fix:**
- **Something is opened or shown:** a new `revealLeft` step runs only for mover jobs, and only when `GUIPids` lists no GUI of the target. It reveals the installed copy in Finder (`open -R`), or, when there is no installed copy, the DMG image file. It runs in four places:
  - a first install that failed reverify or swap
  - a first install that never opened, or that ended `failed` for another reason with no GUI left
  - a replace whose relaunch of the old copy failed
  - an open-installed whose open failed

  An existing installed copy is still relaunched through `finishOpen`, as before.
- **Staging copy:** when a first install fails before its swap, the staging folder is removed with `os.RemoveAll`. It holds only Portty's own checked, quarantine-cleared copy, there is no user app beside it, and no installed GUI exists to sweep it. A replace that fails (an installed copy exists) still leaves its staging folder to the relaunched copy's startup sweep, which moves it to the Trash after `sweepAge` (1 h) and never deletes it. I kept that behaviour on purpose: it matches the update path.
- **Result the next start can show:** `consumeInstallResult` now accepts a `failed` result whose `OldVersion` is empty (only the mover's first install writes that). It also only logs, and does not show, a result about a version that is not newer than the running build. That covers an open-installed whose open failed, and a copy the user installed by hand since. The keep-daemon handling runs first and is unchanged.
- **Eject order:** not changed. The volume is still ejected before verify and swap, which is the order the VM rows R4-* ran. Instead of reattaching the volume, the DMG file is revealed.

**Tests:**
- `TestInstaller_MoverFailureLeavesSomethingToActOn` has four rows: first-install reverify, first-install swap, first install never opens, and open-installed never opens. Each row checks the result file, exactly one reveal of the right path, and that no staging folder is left.
- `TestInstaller_MoveFirstInstallUnhealthy` now also checks that nothing is revealed while the new GUI is running.
- `TestConsumeInstallResult_MoverResults` covers the empty old version and the stale results.

### WR-03: The mover writes home paths into update.log

**Files modified:** `app_install.go`, `app_mover_test.go`
**Commits:** 8da5bb61, c137d0c9 (test placeholder path changed so `tests/check-brand.sh` passes)
**Applied fix:** `startInstaller` now returns an `installerStartErr`. Its `Error()` is only `installer not started: <step>`, and `Unwrap()` returns the cause. Logging the error in `runMover` (`updateLogf("mover: %v", err)`) therefore writes only the step name, and `InstallFailureReason` still finds the cause's sentence (for example ErrSafety from copy-check). Any other caller that logs the error gets the same result.
**Tests:** `TestRunMover_InstallerStartFailureLogsNoPath` drives the real `startInstaller` with two failing steps. The bundle step fails with a `*PathError` that names a fake home path; the copy-check step fails with ErrSafety. The test asserts that update.log contains the step and no `/`, and that the alert still starts with the right sentence.

### WR-04: The rollback's keep-daemon guarantee depends on an unchecked result write

**Files modified:** `internal/updater/installer.go`, `internal/updater/installer_test.go`, `internal/updater/selfinstall_test.go`, `cmd_install_update.go`, `app_mover.go`, `app_install.go`, `app_install_test.go`
**Commit:** 275d38de
**Status:** fixed: requires human verification (one new external behaviour: `open --env`)
**Applied fix:** The installer now notes whether the last result write worked. If a rollback sets `KeepDaemon` and the result was not written, `finishOpen` relaunches the old GUI with `PORTTY_KEEP_DAEMON_ONCE=1` passed through `open --env` and logs that. `consumeInstallResult` reads the variable first, sets `keepDaemonOnce`, logs it, and unsets it at once so a daemon this GUI spawns does not inherit it.
- To carry this, `InstallerEnv.Open` and `openApp` gained an optional `env ...string`. `openAppPathFunc` keeps its old two-argument type.
- **Why this option:** the write fails because of a full disk or a config folder that cannot be written. A retry or a second marker file would fail the same way, while the launch environment needs no disk. Startup detection ("daemon newer than this GUI") would also change the DEBT-08 restart offer after a manual downgrade, which is a product decision, so I did not use it.
- **Safety:** the variable can only make the first ensure keep the running daemon and ask nothing. That is the safe direction even if someone else sets it.

**Tests:**
- `TestInstaller_RollbackKeepDaemonWithoutResultFile`, case "result not written": the result folder cannot be created. The new app opens with no extra environment, and the old app is relaunched with the variable and a log line.
- Same test, case "result written": no open ever carries the variable.
- `TestConsumeInstallResult_KeepDaemonFromLaunchEnv`: the first ensure uses the keep-daemon policy, and the variable is gone afterwards.

### WR-05: The sweep trashes `Portty.staged`, a name nothing creates

**Files modified:** `app_install.go`, `app_install_test.go`
**Commit:** 9d4c125d
**Applied fix:** Removed `Portty.staged` from the sweep loop and from its `TestSweep_StagingDirs` fixture. Chesterton's Fence: that name was the staging layout for the case where probe P8 came out CONTRADICTED (18-02-PLAN.md). 18-VM-CHECKS.md records P8 as CONFIRMED, `CopyApp` always stages as `Portty.app`, and no code creates the other name.

## Verification

All gates ran in the agent's isolated git worktree (branch `worktree-agent-ae3cb02c2286cab95`), not in the main checkout. That worktree had `frontend/dist` built with `pnpm install --frozen-lockfile && pnpm run build`. Toolchain: `GOTOOLCHAIN=go1.26.11`.

- `gofmt -l *.go internal/`: clean
- `go vet ./internal/updater/ .`: clean
- `go vet -tags vmpreview . ./internal/daemon/`: clean
- `golangci-lint run --new-from-rev=c714c8ef ./...`: 0 issues
- `go test -race -count=1 ./internal/updater/ . ./internal/daemon/`: all ok (updater 3.0 s, main 23.5 s, daemon 103.2 s)
- `bash tests/check-brand.sh`: clean, after c137d0c9
- `bash tests/check-traceability-paths.sh`: OK (236 paths)

No new test files, so TESTING.md needed no manifest or traceability rows. Only the description of `TestParseGUIPids` changed.

## VM re-check recommended

None of this was run in the VM. These paths change real-system behaviour that unit tests only fake:
- **WR-04:** `open --env PORTTY_KEEP_DAEMON_ONCE=1 /Applications/Portty.app` must reach the GUI's environment through LaunchServices. Check it once on the VM, for example with a rig fault that makes the result folder unwritable during an R2-B rollback.
- **WR-01:** `ps -o pid=,uid=,args=` on the VM's macOS. The column format matched on the host, and a health-check row (R1) would confirm it.
- **WR-02:** the Finder reveal after a failed mover install, for example a first install with the staged copy's codesign broken.

## Post-review VM re-check

These rows ran in VM `portty-test` on new rig builds of `83df1e1f` (the post-fix tree): N = 1.8.0 and N+1 = 1.8.1, ad-hoc, `verify-mode` `adhoc`, un-granted. The full rows, the method and the end state are in 18-VM-CHECKS.md. The evidence is in `evidence/RC-*`.

| Row | Covers | Verdict | Key evidence |
| --- | --- | --- | --- |
| RC-R1 | regression of the update path; WR-01 on macOS 27.2 | PASS | 1.8.1 installed, 1 GUI, daemon `v1.8.1`, same shell pids, old app in the Trash, run_dirs=0, no dialog. `installer: updated` means the uid-filtered `ps` parse found the new GUI and the health check passed (click-to-healthy 3.45 s). |
| RC-R2A | regression of the rollback | PASS | Rolled back at +2.76 s, well inside W = 100.45 s. Daemon still `v1.8.0`, same shells, warning banner, DMG kept. |
| RC-CR01 | CR-01 | PASS | The cache DMG opened by hand got `Replace Portty 1.8.0 in Applications?`. Replace swapped the app in (`installer: updated`), then `dmg kept: not a Portty download`. No "couldn't move itself" alert. |
| RC-WR04 | WR-04 | PASS | update.log has `result not written`, then `keepDaemon passed as PORTTY_KEEP_DAEMON_ONCE`. `ps -E` shows the variable in N's launch environment, and N logged `keep daemon once: set in the launch environment`. No restart dialog in any axdump sample. Sessions kept their shells under the 1.8.1 daemon. The blocker directory was removed afterwards. |
| RC-WR04-stall | (seen during RC-WR04) | OBSERVED | Both sessions' output stalled for about 17 min (about 07:11:25 to 07:28:24) and resumed when a CLI viewer joined. Same pids, no gaps. Cause not established, and it did not recur over a further 11 min. Recommended: a soak test with only a GUI viewer attached. |
| RC-WR02 | WR-02 | PASS | The installer's re-verify failed after the DMG GUI quit. Finder `Downloads` came to the front with the DMG selected, nothing was installed, and no `.Portty-update-*` was left. The next manual launch (a copy installed by hand) logged `result not shown: not newer than this version`, as designed. |

---

_Fixed: 2026-11-26T06:54:47Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
