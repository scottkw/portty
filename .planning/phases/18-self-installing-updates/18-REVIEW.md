---
phase: 18-self-installing-updates
reviewed: 2026-11-26T06:37:15Z
depth: standard
files_reviewed: 20
files_reviewed_list:
  - app_install.go
  - app_mover.go
  - app_update.go
  - app.go
  - cmd_install_update.go
  - main.go
  - trash_darwin.go
  - trash_darwin.m
  - updates_vmpreview.go
  - internal/daemon/share_vmpreview.go
  - internal/updater/appinstall.go
  - internal/updater/install.go
  - internal/updater/installer.go
  - internal/updater/mover.go
  - internal/updater/outcome.go
  - internal/updater/service.go
  - internal/updater/update.go
  - frontend/src/App.tsx
  - frontend/src/components/UpdateBanner.tsx
  - .planning/phases/18-self-installing-updates/rig/p18-rig.sh
findings:
  critical: 1
  warning: 5
  info: 10
  total: 16
  necessity: 1
status: issues_found
---

# Phase 18: Code Review Report

**Reviewed:** 2026-11-26T06:37:15Z
**Depth:** standard (per-file, with cross-file tracing where safety depends on it)
**Files Reviewed:** 20
**Status:** issues_found

## Summary

I reviewed the Phase 18 self-install code: the GUI hand-off (`app_install.go`, `app_mover.go`, `app_update.go`, the startup parts of `app.go`), the detached installer (`cmd_install_update.go`, `internal/updater/installer.go`), staging and verification (`appinstall.go`, `install.go`), the mover detection, the result and skip records, the Trash bridge, the vmpreview-only files, the banner, and the VM rig. In `app.go` I covered only `keepDaemonOnce`, `consumeInstallResult`, the mover startup and `ensure`. For `app.go` I also traced into `internal/daemon/process.go`, to see how `keepDaemonOnce` reaches `ensureDaemon` and how the daemon is spawned.

Things that hold up:
- The installer never signals the daemon. `ParseGUIPids` takes only argv that is exactly the bundle executable, optionally followed by a launch-services flag. The daemon (`exe daemon`, `process_unix.go:26`), the takeover and the installer all carry a subcommand, so none of them is ever listed.
- `Swap` (RENAME_SWAP) keeps a complete bundle at the target at every instant.
- Job and result files are read with O_NOFOLLOW and checked for owner, mode and size.
- `copyBundle` checks the copy before it clears quarantine.
- The DMG Trash is bounded, and the result is written before it.
- Every vmpreview file carries `//go:build vmpreview`, and `release.yml` refuses a binary that contains the feed address.

The main defect is CR-01. The mover refuses every DMG in Portty's own update cache. That is exactly the DMG the Reveal fallback shows the user, so the documented manual path fails after the user has already said "Replace". Other findings:
- A ps match that ignores which account owns the process (WR-01).
- Mover failures after the GUI quits leave no GUI, an ejected volume, a hidden staging copy and no message (WR-02).
- A home-path leak into `update.log` (WR-03).
- A rollback that depends on a result-file write it never checks (WR-04).
- A Trash name no code ever creates (WR-05, necessity).

`go vet ./internal/updater/ .` is clean under the pinned toolchain (go1.26.11).

## Critical Issues

### CR-01: The mover refuses the DMG that the Reveal fallback hands the user (also renamed or foreign-owned DMGs)

**File:** `internal/updater/installer.go:215-217`, `internal/updater/installer.go:254-265`, `app_mover.go:110-111`, `app_mover.go:186-188`
**Issue:** `checkMoveJob` turns the job away unless `imageFileOK(j.ImagePath)` holds. `imageFileOK` rejects:
- any path containing `/app.portty.Portty/`
- any name not matching `portty-v*-darwin-*.dmg`
- any file another account owns

It applies to both `KindMove` and `KindOpenInstalled`. Every self-install failure ends in `revealFallback` (`service.go:356-374`) or `ShowResult` (`outcome.go:43-58`). Both reveal the DMG in `~/Library/Caches/app.portty.Portty/updates`, and the banner says "The download is shown in Finder". The user opens that DMG and launches Portty from the volume. The mover asks "Replace Portty X in Applications?", the user clicks Replace, and Portty copies and checks the staged app (`app_mover.go:154-184`). Then `startInstaller` spawns the installer, and `ReadJob` refuses the job before the started marker. The GUI waits out `installerStartWait` and gets `no-marker`. The user sees "Portty couldn't move itself to the Applications folder. Portty couldn't install the update." When the installed copy is the same or newer, even the plain "open the installed copy" path fails the same way.

The design meant only to skip the Trash. `finishTrash` (`installer.go:510-513`) already re-checks `imageFileOK` and keeps the DMG gracefully. But the job-level check blocks the whole move. A renamed DMG ("Portty.dmg"), or one in `/Users/Shared` downloaded by another account, fails the same way. The staged copy is checked, then deleted, after the user already agreed to replace.
**Fix:** Make the image path optional for the move. Keep the hard checks only for the Trash. In `checkMoveJob`, drop the `imageFileOK` refusal, or accept an empty `ImagePath`. In `runMover`, set `job.ImagePath` only when the DMG qualifies:
```go
// checkMoveJob: an image path that may not be trashed is not a refusal.
if j.ImagePath != "" && (!filepath.IsAbs(j.ImagePath) || filepath.Clean(j.ImagePath) != j.ImagePath) {
    return jobErr("image path")
}
// finishTrash already skips the Trash when !imageFileOK(j.ImagePath).
```
Add a row test: a `KindMove` job whose `ImagePath` is in the update cache is accepted, swaps, and keeps the DMG.

## Warnings

### WR-01: GUI matching ignores the process owner, so another account's GUI can pass the health check and draws signals

**File:** `cmd_install_update.go:98-106`, `internal/updater/installer.go:341-359`, `internal/updater/installer.go:431-445`, `internal/updater/installer.go:561-576`
**Issue:** `realGUIPids` runs `ps -axww -o pid=,args=` across all users. `ParseGUIPids` matches on argv alone. On a Mac where a second account runs `/Applications/Portty.app` (fast user switching), that account's GUI pid is listed. This has three effects:
1. **The health check passes falsely.** `gui` is true while any listed pid exists, so `noGUI` never counts up. Suppose this user's new GUI exits after its takeover committed, so the daemon is at the new version. That is fault mode R2-B. The installer still records `updated` and never rolls back.
2. `stopGUIs` sends SIGTERM, then SIGKILL, to the other account's process. That fails with EPERM, so no harm is done, but the installer waits out `StopWait + KillWait` and logs "GUI still running".
3. The swap replaces the bundle under the other account's running GUI and daemon. The old bundle then goes into this user's Trash.

"Must never signal the wrong process" is only safe here by accident: EPERM protects it.
**Fix:** Filter by uid in the ps call and in the parser:
```go
out, err := exec.CommandContext(ctx, "/bin/ps", "-axww", "-U", strconv.Itoa(os.Getuid()), "-o", "pid=,args=").Output()
```
Or add a `uid=` column and have `ParseGUIPids` take the wanted uid. Add a parser test with a foreign-uid line.

### WR-02: Mover failures after the GUI quits are silent and leave no Portty running, an ejected volume and a hidden staged copy

**File:** `internal/updater/installer.go:486-495`, `internal/updater/installer.go:402-406`, `internal/updater/installer.go:528-545`, `app_install.go:343-347`
**Issue:** For a mover job, the installer ejects the volume first (`Detach`, lines 489-495), before `VerifyApp` and `Swap`. If the first-install reverify or swap fails, `finishOpen` finds no target and opens nothing (lines 404-406). Afterwards:
- no Portty GUI is running
- the DMG volume is gone
- `/Applications/.Portty-update-*/Portty.app` stays behind, quarantine already cleared. Only a running GUI's sweep removes it, and there is no installed GUI to run one.
- the result has `OldVersion: ""`, so the next start rejects it in `consumeInstallResult` (lines 343-347, "versions malformed") and the user is never told.

`KindOpenInstalled` with a failed `open` (lines 529-531) also leaves no GUI and an ejected volume. If the user later starts the installed copy by hand, they get "Portty couldn't install version X" plus a skip record (`ShowResult` -> `skipVersion`) for a version that was never being installed.
**Fix:** In the mover kinds:
- Detach only after the swap succeeded, or for open-installed only after the open succeeded. If the eject cannot be moved, at least `os.RemoveAll` the staging dir on a first-install failure, since it holds only Portty's copy.
- When nothing can be relaunched, run `open -R` on the DMG image path (or reattach it) so the user has something to act on.
- Make `consumeInstallResult` accept an empty `OldVersion` for mover results (use a distinct message), and skip `ShowResult`/`skipVersion` for `KindOpenInstalled` failures.

### WR-03: The mover writes home paths into update.log

**File:** `app_mover.go:99`, `app_install.go:85`, `app_install.go:92-101`, `app_install.go:112-114`
**Issue:** `startInstaller` returns `fmt.Errorf("installer not started: %s: %w", step, err)`, which wraps raw errors:
- `os.MkdirAll` / `os.MkdirTemp` (`*PathError` with `~/Library/Application Support/portty/update`)
- `WriteJob` (open of `run-*/job.json`)
- `spawnInstaller` (`fork/exec /Users/<name>/.../Portty`)
- `runningBundle` (EvalSymlinks error)

`Service.install` deliberately logs only "installer start failed" (`service.go:344`). `runMover`'s `fail` instead runs `updateLogf("mover: %v", err)`, which writes the full chain into `update.log`. The code knows the rule, since line 148 says "the error names a path; the log must not". The VM rows assert `update.log` holds no `/Users`. `bundleForUpdates` and `detectMover` also `log.Printf` raw EvalSymlinks errors (`app_install.go:57`, `app_mover.go:61`), but those reach only stderr.
**Fix:** Log the step, not the wrapped error. One option is to return a typed error that carries the step and keep the cause out of `Error()`:
```go
type installerStartErr struct{ step string; cause error }
func (e *installerStartErr) Error() string { return "installer not started: " + e.step }
func (e *installerStartErr) Unwrap() error { return e.cause }
```
Or in `runMover`, `updateLogf("mover: %s", updater.InstallFailureReason(err))` plus the step.

### WR-04: The rollback's keep-daemon guarantee depends on a result write it never checks

**File:** `internal/updater/installer.go:392-399`, `internal/updater/installer.go:461-462`, `app_install.go:331-333`, `internal/daemon/process.go:270-290`
**Issue:** After a rollback where the new daemon already took the sessions (R2-B), the only thing that stops the relaunched old GUI from running a normal ensure is `KeepDaemon` in the result file. A normal ensure means: StaleVersion, a takeover from a newer daemon (refused by the downgrade wall), then the DEBT-08 "Restart Now" question, and approving it ends every session. `finish` only logs "result not written" when `WriteResult` fails, for example on a full disk (likely during an update) or a config dir that is not writable. Then `finishOpen` relaunches the old GUI anyway. For the remote-Mac use case, one click on the resulting question ends the sessions the rollback was meant to keep.
**Fix:** Treat a failed result write in `rollback` as something that needs a second channel:
- retry `WriteResult` a few times
- if it still fails, pass the flag through the launch instead, e.g. `open --env PORTTY_KEEP_DAEMON_ONCE=1 <app>` (supported by `/usr/bin/open` on macOS 13+), read in `startup` next to `consumeInstallResult`
- at minimum, log the failure in a line the user's GUI surfaces

Add a test where `WriteResult` fails during a rollback.

### WR-05: Phantom requirement: the sweep trashes `Portty.staged`, a name nothing ever creates

**File:** `app_install.go:241`, `app_install.go:264`
**Category:** necessity
**Issue:** `sweepDirs` moves both `Portty.app` and `Portty.staged` out of every stale `.Portty-update-*` folder. No code in the repo creates `Portty.staged`: `CopyApp` stages as `Portty.app` (`appinstall.go:180`), and `Swap` exchanges `Portty.app` names. `git log -S'Portty.staged' -- '*.go'` finds only the sweep commit and its test (`a84ffb3d`, `c3a1c744`), and no released build ever wrote it. The branch guards a state that cannot occur, and it reads as if a second staging layout existed.
**Fix:** Remove `"Portty.staged"` from the loop and its test fixture (`app_install_test.go:240`), or name the layout that produces it.

## Info

### IN-01: Mover comment says quarantine is cleared after the checks, but code clears it before `VerifyApp(staged)`

**File:** `app_mover.go:87-88`, `app_mover.go:175-184`
**Issue:** The doc says "The staged copy passes the update's checks before its quarantine is cleared." The loop runs `ClearQuarantine`, then `VerifyApp`, then `CheckStagedApp`. The source on the read-only volume was verified first, and a failed check removes the staging dir, so nothing unverified survives. Still, the comment and the UPD-09 order disagree with the code.
**Fix:** Run `VerifyApp(staged)` before `ClearQuarantine(staged)` (codesign does not need the flag cleared), then `CheckStagedApp`. Or correct the comment.

### IN-02: The update job's `DMG` path is not tied to the update cache

**File:** `internal/updater/installer.go:185-187`, `internal/updater/installer.go:599`
**Issue:** `ReadJob` checks only the base name, absoluteness and cleanliness of `j.DMG`. The installer then calls `os.Remove(j.DMG)`, so a same-account job can delete any `portty-v*-darwin-*.dmg` anywhere. `ShowResult` already requires `filepath.Dir(r.DMG) == s.Dir`.
**Fix:** Pass the cache dir into the installer and require `filepath.Dir(j.DMG) == cacheDir` in `ReadJob`.

### IN-03: `moverRoots` silently uses an empty home

**File:** `app_mover.go:26-29`
**Issue:** `home, _ = os.UserHomeDir()` turns a failure into `""`. `ExistingCopy` and `MoveTarget` then build relative paths such as `Applications/Portty.app`, resolved against the GUI's cwd (`/`). It fails closed later (`Preflight` or `checkMoveJob` refuse relative paths), but with a misleading message. That conflicts with the "no silent fallbacks" rule.
**Fix:** Return the error and call `fail` with a step name when there is no home.

### IN-04: A rollback deletes the bundle that the kept daemon runs from

**File:** `internal/updater/installer.go:451-460`
**Issue:** After R2-B, the new daemon is kept (`KeepDaemon`), but its bundle is swapped into the staging dir and removed with `os.RemoveAll`. The process keeps running from the unlinked vnode. Its `os.Executable()` still returns the target path, which now holds the old binary. So `refreshCLILink` (`internal/daemon/clilink.go:15-21`) points `portty` in new sessions at the older CLI, which hits the downgrade wall without `--keep-daemon`.
**Fix:** Document the behavior. Or move the failed copy to the Trash instead of deleting it, so it stays inspectable, and consider keeping it until the kept daemon exits.

### IN-05: The rollback continues after it could not stop the new GUI

**File:** `internal/updater/installer.go:440-453`
**Issue:** `stopGUIs` only logs "GUI still running" when SIGKILL did not land. `rollback` then swaps and deletes the bundle under that GUI. Because `open` without `-n` activates an already-running app with the same bundle ID, `finishOpen` may bring forward the failed new GUI instead of launching the old one, while the result says `rolled-back`.
**Fix:** Have `stopGUIs` report failure, and record `failed` with reason `gui-stuck` (without RemoveAll) when the new GUI survives.

### IN-06: The old-bundle Trash on success is unbounded and runs before the result is written

**File:** `internal/updater/installer.go:589-602`
**Issue:** The DMG Trash is bounded by `trashWait` because `trashItemAtURL` can block on a system prompt. The old-bundle `env.Trash(j.Staged)` is synchronous, and `finish(OutcomeUpdated)` runs only after it. If it blocks (a TCC or App Management prompt, or a volume without a usable Trash), there is no result, the DMG is not deleted, and the run folder stays until the 1 h sweep.
**Fix:** Write the result before the Trash, and reuse the bounded goroutine pattern from `finishTrash`.

### IN-07: The health check depends on the exact argv[0] string

**File:** `internal/updater/installer.go:389`, `internal/updater/installer.go:349-355`, `app_install.go:37-45`
**Issue:** `exe` comes from `EvalSymlinks(os.Executable())`, and a GUI counts only when ps argv equals that string exactly. If LaunchServices ever launches with a different spelling (case on a case-insensitive volume, `/System/Volumes/Data/...`, a home reached through a symlink), every update would roll back with `health`. The VM rows show the common case works.
**Fix:** Compare `os.SameFile` on the argv[0] path (or the pid's `proc_pidpath`) with the target executable, instead of comparing strings.

### IN-08: A transient install failure suppresses that version from automatic offers

**File:** `internal/updater/outcome.go:41`, `internal/updater/service.go:153-162`
**Issue:** `ShowResult` writes the skip record for every `failed` result, including reasons that have nothing to do with the version: `quit-timeout`, `open` (six -600s), `swap`, `swap-back`. After the next restart, automatic checks settle on "current", and an unattended Mac stays on the old version until the next release, unless someone checks by hand.
**Fix:** Record the skip only for version-caused reasons (`health`, `gui-exited`, `reverify`). Leave the others re-offerable.

### IN-09: vmpreview: the release canary lives in one file, and the loopback gate trusts any local account

**File:** `internal/daemon/share_vmpreview.go:55-59`, `updates_vmpreview.go:41`, `.github/workflows/release.yml:105-110`
**Issue:** `release.yml` catches the vmpreview tag only through the `127.0.0.1:8788` string in `updates_vmpreview.go` (package main). The daemon-package vmpreview files have no canary of their own: the same-account WhoIs bypass, the fake Tailscale and the handoff/compat fakes. If that main-package file is ever renamed or removed, the bypass could ship undetected. Inside a vmpreview build, `vmLoopbackWhoIs` treats every 127.0.0.1 caller, from any local uid, as this Mac's user. The link token still applies. VM-only, so acceptable, but worth stating.
**Fix:** Add a unique canary string to `share_vmpreview.go` (or a shared `vmpreview` constant referenced from both packages), and have `release.yml` grep for it too.

### IN-10: The rig interpolates VERSION into remote shell strings, and `deploy-n` kills the daemon

**File:** `.planning/phases/18-self-installing-updates/rig/p18-rig.sh:93`, `:103-105`, `:77`
**Issue:** `cmd_dmg` and `cmd_feed` put `$ver` unquoted inside strings run by the VM's shell, so a malformed VERSION runs as shell. `pkill -x Portty` also matches the daemon (`Portty daemon`). That is intended for a wipe, but it contradicts the "never the daemon" wording used for `quit-gui`. VM-only operator tool.
**Fix:** Validate `ver` against `^[0-9]+\.[0-9]+\.[0-9]+$` at the top of each command. Add a comment that `deploy-n` deliberately ends the daemon.

---

_Reviewed: 2026-11-26T06:37:15Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
