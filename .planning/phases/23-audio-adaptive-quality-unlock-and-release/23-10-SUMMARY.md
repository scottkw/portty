---
phase: 23-audio-adaptive-quality-unlock-and-release
plan: 10
subsystem: desktop
tags: [unlock, lock-screen, loginwindow, vm-rig, wake-key]
requires:
  - phase: 23-09
    provides: p23-rig.sh, the stats route, 23-VM-CHECKS.md and the VM permissions
provides:
  - "U-01 evidence: no wake-key candidate reveals the lock screen's password field on macOS 27.2"
  - "U-02 evidence: a real own-account unlock from the home VM on the first try"
  - "rig subcommands for the rest of the U rows (lock, unlock-admin, lockstate, wake, lw-log, u01-tight, unlock-try, panel) and a vmdesktop probe op, wake"
affects: [23-12, 23-15]
tech-stack:
  added: []
  patterns:
    - "loginwindow's keyPressed log line used as the reveal oracle, with a diagnostic letter and a control"
    - "wrong-password sentinels made per run outside the repo, redacted from all rig output"
key-files:
  created:
    - .planning/phases/23-audio-adaptive-quality-unlock-and-release/rig/js/unlockpanel.js
    - .planning/phases/23-audio-adaptive-quality-unlock-and-release/rig/u01tight.sh
  modified:
    - internal/desktop/probe_vmdesktop.go
    - .planning/phases/23-audio-adaptive-quality-unlock-and-release/rig/p23-rig.sh
    - .planning/phases/23-audio-adaptive-quality-unlock-and-release/23-VM-CHECKS.md
key-decisions:
  - "unlockWakeKey stays kVK_Shift: no candidate beat another, and unlock.go and its test did not change"
  - "Stopped at the U-01 FAIL as the plan's constraint requires; U-02 (Task 1's own last step) was still run once to show whether the unlock sequence works without a revealing wake key. It does"
duration: 2h
completed: 2026-11-28
status: partial
estimate_tokens: 90000
actuals:
  tokens: 40000
  tasks: 1
  commits: 2
plan_head_before: d4849d39b641cd6b7bebca197d1fe631a99d2a0d
plan_head_after: 004f54d23941bdb2fdc97a6757ff1fcce08f597a
---

# Phase 23 Plan 10: Remote unlock on a real lock screen Summary

**U-01 FAILED: none of Shift, F15 or Left Arrow (nor Escape, Tab, Right or Up Arrow) reveals the password field at the lock screen. U-02 PASSED anyway: the first typed character is what reaches loginwindow's reveal handler and it still lands in the field. The plan stopped at U-01 as its constraint requires; Tasks 2 and 3 were not run.**

## What was done

- **Probe op `wake`** (`internal/desktop/probe_vmdesktop.go`, vmdesktop tag only, never in a release build): posts one chosen virtual key through the daemon's own Wake step (user-activity declaration, then the key), with no viewer and no lock gate. The existing probe `key` op goes through the lock gate and cannot post at a locked screen, so this op was needed to test candidates through the real bridge (Rule 3, one file outside the plan's list).
- **Build 23.10.0** (HEAD `647b78bb`) built on the host (compile only), deployed to portty-test only. Permissions still held after the redeploy (`{"sr":true,"ax":true}`).
- **Rig** (`p23-rig.sh`, `rig/js/unlockpanel.js`, `rig/u01tight.sh`): `lock`, `unlock-admin`, `lockstate`, `wake`, `lw-log` (loginwindow's lock-screen lines for a time window, scrubbed), `u01-tight`, `unlock-try` (fills the Unlock panel by page script and presses Unlock with a real WebDriver click, reads the panel five times a second and the stats twice a second, then loginwindow and the audit file), `panel`, plus `pw_for` and per-run wrong-password sentinels kept outside the repo and redacted from every output. No wrong sentinel was created or typed in this run.

## Row results

| Row | Result | One line |
|-----|--------|----------|
| U-01 | FAIL | Shift, F15, Left Arrow, awake and display-dark: the candidate logs nothing at loginwindow and the diagnostic letter 0.8 s later still hits the reveal handler. Escape, Tab, Right and Up Arrow the same. Controls show the diagnostic can tell |
| U-02 | PASS | Panel `Unlocking…`, then panel gone and `Mac unlocked.`; stats `unlocking` then unlocked with relock armed; one `loginPressed`, success, no retry; one `unlocked` audit line, mode own, no length field |
| U-03..U-12, P-01 | NOT-RUN | the plan stopped at U-01 |

## Chosen wake key

**kVK_Shift (0x38), unchanged.** `unlock.go` and `unlock_test.go` did not change. `TestUnlock_` passes under `-race -count=1`. No candidate revealed the field, so none was "chosen by evidence"; Shift stays because it is the starting value and no other key did better.

## What the U-01 evidence says

loginwindow logs `keyPressed: View is hidden password field length = 0` when a key reaches its reveal handler and nothing when the already-revealed field takes the key itself. Controls: a letter as the candidate followed by a diagnostic letter gives one line (the first letter only); no candidate gives the line for the diagnostic letter. For all six plan runs the candidate itself logs nothing and the diagnostic letter logs the line, so no candidate reveals the field. In the dark-display runs the daemon's wake does light the display (`Will Wake`, `Did Wake`, `Clearing password field`) and the field is re-shown within 25 ms, yet the diagnostic letter still logs the line.

The premise of the wake key (a key that reveals the field and types nothing) does not hold on macOS 27.2. What the wake step still does is the display wake; the reveal is done by the first typed character, which still lands in the field (Phase 17's R16-B reading, and U-02). The race that R16-B measured (the first character arriving while the field is not ready, 2 of 13 failed there) therefore remains, and the one-retry design is its safety net. U-03's ten cycles would have counted it; they were not run.

## Decisions made on the user's behalf

None. No permission prompt or dialog appeared in either VM, nothing was clicked, no re-authentication was needed (recorded in 23-VM-CHECKS.md "User checkpoint").

## Deviations from Plan

1. **[Rule 3 - blocking] Probe `wake` op added** to `probe_vmdesktop.go` (see above). Not in the plan's files list; vmdesktop-only.
2. **[Stop condition] Plan stopped after Task 1.** U-01 is RESULT=FAIL, so Task 1's verify (`U-01 ... RESULT=PASS`) does not hold and Tasks 2 and 3 were not run. U-02, Task 1's own last step, was run once so the report can say whether the unlock sequence works without a revealing key.
3. **[Rule 1 - rig fix] zsh `log` builtin** in the VM shadows `/usr/bin/log`; the rig uses the full path. ssh splits arguments with spaces, so `lw-log` passes times with an underscore. The audit evidence prints only event, mode, reason and the key names: the lines also name the viewer, which must not reach committed files. The first U-02 run was repeated after that change and after the poll was taught to read the success notice.
4. **[Incident, recovered]** A control that typed a letter left a stray character in the lock field once; the next admin unlock typed `zadmin`-shaped text, was a counted wrong try at macOS (`Attempt #: 1` failed), and the following unlock succeeded (`Attempt #: 2`). No sentinel was involved; no lock-screen delay appeared.

## Owner-run and later items

- For 23-15's go/no-go: U-01 FAIL as a UNLK-01 finding (the wake key premise). The practical risk is the hidden-field race, unmeasured here (U-03).
- U-03..U-12 and P-01 remain to be run (here once the stop is released, or on a real Mac in 23-14). Their criteria are fixed in 23-VM-CHECKS.md.
- Real-Mac items for 23-12 and 23-14: the office-Mac unlock row; iPhone Safari half of P-01.
- New test files for 23-12's manifest: none (no Go test file was added; the probe op is VM-only and has no test).

## Incidental, not a row

Viewer closed at 00:54:26, `relocked` audit line at 00:54:56 (30 s), `relock_armed` false. Goroutines: 1214 at the start of the run (the old 23.9.0 daemon), 32 right after the redeploy, 62 after 3 minutes, 441 at the end of U-02, 456 in the final state read; they grow about 15 a minute with nobody connected (23-09 finding 4). The probe's `wake` op starts no goroutine.

## Known Stubs

None.

## Threat Flags

None. The only new surface is the vmdesktop-tagged probe op (T-23-27: runs only in the VMs; the tag must never reach a release build, which `tests/check-brand.sh` enforces).

## VM state at the end

portty-test: build 23.10.0, unlocked, `relock_armed` false, no viewer, no guest share (`share-status` active false), no dummynet pipe, pf disabled. portty-test-2: Safari session closed, nothing running for this plan. Neither VM was stopped or restarted; both stayed on the tailnet.

## Self-Check: PASSED

Present: `rig/js/unlockpanel.js`, `rig/u01tight.sh`, the nine `evidence/U-0*` files, commits `647b78bb` and `004f54d2` (ancestors of HEAD). `bash tests/check-brand.sh` clean.
