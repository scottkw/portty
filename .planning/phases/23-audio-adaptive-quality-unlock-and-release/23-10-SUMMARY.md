---
phase: 23-audio-adaptive-quality-unlock-and-release
plan: 10
subsystem: desktop
tags: [unlock, lock-screen, loginwindow, vm-rig, wake-key, relock, password-manager]
requires:
  - phase: 23-09
    provides: p23-rig.sh, the stats route, 23-VM-CHECKS.md and the VM permissions
provides:
  - "U-01..U-12 and P-01 results on a real macOS 27.2 lock screen, with evidence"
  - "rig subcommands: lock, unlock-admin, lockstate, wake, lw-log, u01-tight, unlock-try, unlock-seq, panel, sentinel-hits; rig/capsset.swift; a vmdesktop probe op, wake"
affects: [23-12, 23-14, 23-15]
tech-stack:
  added: []
  patterns:
    - "loginwindow's keyPressed log line used as the reveal oracle, with a diagnostic letter and a control"
    - "wrong-password sentinels made per run outside the repo, redacted from all rig output, counted (never printed) by sentinel-hits"
key-files:
  created:
    - .planning/phases/23-audio-adaptive-quality-unlock-and-release/rig/js/unlockpanel.js
    - .planning/phases/23-audio-adaptive-quality-unlock-and-release/rig/u01tight.sh
    - .planning/phases/23-audio-adaptive-quality-unlock-and-release/rig/capsset.swift
  modified:
    - internal/desktop/probe_vmdesktop.go
    - .planning/phases/23-audio-adaptive-quality-unlock-and-release/rig/p23-rig.sh
    - .planning/phases/23-audio-adaptive-quality-unlock-and-release/23-VM-CHECKS.md
key-decisions:
  - "unlockWakeKey stays kVK_Shift: no candidate beat another; unlock.go and its test did not change"
  - "The orchestrator released the stop after U-01 FAIL because U-02 passed; U-01 stays RESULT=FAIL and goes to 23-15"
  - "Findings are recorded, not fixed: late-detected success is retyped (F-late-success), P-01 Chrome save prompt"
duration: 5h
completed: 2026-11-28
status: complete
estimate_tokens: 90000
actuals:
  tokens: 90000
  tasks: 3
  commits: 5
plan_head_before: d4849d39b641cd6b7bebca197d1fe631a99d2a0d
plan_head_after: a0e6ff57972a1ca5955112f4bbdbda88e020b215
---

# Phase 23 Plan 10: Remote unlock on a real lock screen Summary

**Remote unlock works on a real macOS lock screen (10 of 10 clean unlocks, retry never needed in U-03), its refusals, throttle, pause, guest boundary, input block and re-lock all hold, and nothing leaks into any log. Two real findings: U-01 FAIL (no wake key reveals the password field; the first typed character does it) and P-01 FAIL (Chrome offers to save the Mac password); plus a race (F-late-success) in which a slow macOS authorization is retyped and reported as failure, leaving the Mac unlocked with no re-lock armed.**

## Row results

| Row | Result | One line |
|-----|--------|----------|
| U-01 | FAIL | Shift, F15, Left Arrow, awake and display-dark: none reaches loginwindow's reveal handler (also Escape, Tab, Right, Up). `unlockWakeKey` stays Shift |
| U-02 | PASS | First-try unlock from the home VM: `Unlocking…`, `Mac unlocked.`, one `unlocked` audit line |
| U-03 | PASS | **10 of 10 unlocked, 0 needed the retry** (one `loginPressed` and one success per cycle) |
| U-04 | PASS | Wrong sentinel: failed state, `failures` 1, exactly two typed tries |
| U-05 | PASS | Second submit 0.1 s later refused before typing, `Try again in 5 s`, not counted |
| U-06 | PASS | Failures 2 then 3: warning line, then `Unlock is paused after 3 failed tries. Try again in 15 min...`; six macOS checks in total, no macOS delay; admin unlock worked after the pause ended |
| U-07 | PARTIAL | Emoji refused before typing, no key event. Caps Lock could not be set at the lock screen in the VM (synthetic key and an IOHID helper both fail), so its refusal was not exercised |
| U-08 | PASS | Guest page on the locked Mac: `Locked. Ask the person at the Mac to unlock it.`, no panel, no password input, no match in Safari's AX tree; view only and with control; stats unchanged |
| U-09 | PASS | Click and keys never reach loginwindow; own-account notice shows (not more than once per 10 s); on a fresh page the first click and keys showed no notice |
| U-10 | PASS | Unlocked at 25 s, locked at 35 s after the viewer left (locked at about 31-33 s), `relocked` audit line 31 s after the leave |
| U-11 | PASS | An untagged stand-in click inside the window: re-lock disarmed at 31.5 s, still unlocked at 42 s, no `relocked` line |
| U-12 | PASS | 5 sentinels: 0 hits in daemon.log, rotated logs, desktop-audit.jsonl and the unified log; audit lines are mode own with no length or password key |
| P-01 | FAIL | Safari: no save prompt. Chrome: a `Save password?` bubble appears after the panel is submitted |

## Chosen wake key

**kVK_Shift (0x38), unchanged.** `unlock.go` and `unlock_test.go` did not change; `TestUnlock_` passes under `-race`. loginwindow's `keyPressed:` handler is reached by a letter and by none of the non-character keys tested, so the premise "a key reveals the field and types nothing" does not hold on macOS 27.2. The wake step still lights a dark display, and the first typed character is itself what reveals the field (U-02, and all 10 cycles of U-03). Counts: U-03 **10 successes of 10, 0 retries**. Retries seen elsewhere: 1 (U-10's unlock, which is the late-detection case below).

## Findings (details and evidence in 23-VM-CHECKS.md, Notes items 1 to 8)

1. **F-late-success (UNLK-01, security-relevant), `evidence/F-late-success.txt`.** macOS authorization can outlast the daemon's 3 s check window (2.3 s in P-01's Chrome run with the office VM at load average 15; the usual is 0.35 s). The daemon reads still locked, retypes the password (in that run into the still-visible lock UI, a real wrong try at macOS; in U-10's run the retype started the same second the Mac read unlocked, so it went to the live desktop), reports `unlock_failed`, and **does not arm the re-lock**. With the viewer gone the Mac stayed unlocked for the whole minute watched. Suggested fix, not done: poll longer than 3 s and re-read the lock state right before the retype. This is not the U-08 or U-10 FAIL the stop rule names (both PASS), so the run continued.
2. **P-01 FAIL (Chrome)**: the password field is `type=password` outside a form with `autocomplete=off`, which Chrome ignores. A front-end change is needed (not done).
3. **U-01 FAIL**: see above.
4. **Goroutines** (build 23.10.1 with the shared tailscaled client): **28 right after the deploy, 63 to 67 through the ten U-03 cycles with a viewer, 32 to 37 with no viewer, 33 at the end.** The 23.10.0 build grew about 15 a minute with nobody connected (32 to 456 in about 80 minutes). The leak fix holds.
5. Lock shortcut: Ctrl+Cmd+Q does not lock while the Portty window is frontmost on the office (the rig now puts Finder in front first).

## Decisions made on the user's behalf

None needed. No permission prompt or system dialog appeared in either VM and nothing was clicked for the user (permissions re-read `{"sr":true,"ax":true}` after both redeploys). Chrome's `Save password?` bubble was a finding, closed by quitting Chrome and removing its throwaway profile.

## Deviations from Plan

1. **[Rule 3] Probe `wake` op** added to `probe_vmdesktop.go` (vmdesktop only, not in the plan's file list), because the existing probe `key` op is stopped by the lock gate.
2. **[Stop, then released]** The plan stopped after U-01 FAIL. The orchestrator released the stop because U-02 passed (U-01 stays FAIL, go to 23-15) and Tasks 2 and 3 ran on build 23.10.1 (HEAD `7e47f2b1`, merged into this worktree) on both VMs.
3. **[Rule 1] Rig fixes:** zsh's `log` builtin shadows `/usr/bin/log`; ssh splits arguments with spaces; audit evidence prints only event, mode, reason and key names (the raw lines name a person); Finder in front before the lock key.
4. **[Order]** U-07 and Task 3's rows ran before U-04..U-06 so the daemon's 15 minute pause would not block them; the pause was then waited out with the VM running (no VM stop, no daemon restart).
5. **[Plan text]** U-11's stand-in click came 6 s after the viewer left (the plan says 10 s); U-08 used the home VM's Safari as the guest (the VMs share one Safari session, so own viewer and guest were run one after the other).
6. **[Incident, recovered]** A control that left a stray letter in the lock field made one admin unlock a counted wrong try (Attempt 1 failed, Attempt 2 succeeded). During P-01 the F-late-success retype was a second real wrong try at macOS. Wrong sentinel submits that were typed: three (U-04, U-06 twice), which macOS checked six times; U-05's fourth sentinel and the emoji one were refused before typing. With the two incidents that is eight wrong checks across the run, each reset by a later success; no macOS lock-screen delay ever appeared.

## Owner-run and later items

- 23-15 go/no-go: U-01 FAIL (premise), F-late-success (retype and no re-lock), P-01 Chrome FAIL. Candidate fixes are named above; none was made.
- 23-12 manifest: no new Go test file (the probe op is VM-only). New manual items: Caps Lock refusal at a real lock screen (M-NN, real Mac), the iPhone Safari half of P-01, and the office-Mac unlock row (23-14).
- Not run on a real Mac: the U-07 Caps Lock half and the office half of the unlock sequence.

## Known Stubs

None.

## Threat Flags

| Flag | File | Description |
|------|------|-------------|
| threat_flag: late-success retype | internal/desktop/unlock.go | A retype after a slow success can type the password into an unlocked session and leaves the re-lock unarmed (observed, not changed here) |

## VM state at the end

portty-test: build 23.10.1, unlocked, `relock_armed` false, `failures` 0, no viewer, no guest share (`share-status` active false), no dummynet pipe, pf disabled, Caps Lock flag off, Finder in front. portty-test-2: build 23.10.1 in /Applications (GUI not running), Safari session closed, Chrome quit and its throwaway profile removed, `dnctl list` empty. Neither VM was stopped or restarted; both stayed on the tailnet.

## Self-Check: PASSED

Present: `rig/js/unlockpanel.js`, `rig/u01tight.sh`, `rig/capsset.swift`, every evidence file named in the 23-VM-CHECKS.md rows; commits `647b78bb`, `004f54d2`, `466481c1`, `a0e6ff57` are ancestors of HEAD. `bash tests/check-brand.sh` clean.
