---
phase: 19-unattended-desktop-viewing
plan: 15
subsystem: vm-proof
tags: [vm, tart, safari, chrome, pf, launchd, handoff, screencapturekit, pmset, tcc, notifications, macos-27]
status: complete

requires:
  - phase: 19-07
    provides: rig/p19-rig.sh and the launchd start model rows
  - phase: 19-12
    provides: the real media path rows, viewer debug counters, the vmdesktop lapse route
  - phase: 19-13
    provides: the Desktop group in Settings
  - phase: 19-14
    provides: the tray line, the one-sender lapse rule, the daemon notification fix (V1b)
provides:
  - "19-VM-CHECKS.md rows F1 to F13 and a Closure section: every Phase 19 VM row with its verdict, the accepted DSHR-01 amendment with its evidence, eleven human_needed owner-run items for 19-16"
  - "rig/p19-rig.sh additions: safari-wins, safari-both, safari-join, safari-page, chrome-open/titles/close, pf-block/unblock, lock/unlock, pbpaste-check, tray-items (tray.swift), tick-gaps, rig/readterm.js, rig/scroll.swift"
  - "An open product defect with its evidence: a clean daemon stop with a viewer attached is counted as a crash by the capture breaker"
affects: [19-16]

actuals:
  tokens: 46000
  tasks: 3
  commits: 3
plan_head_before: 6cdb9e0e2ce8028a8aa94e905a0be2415035a33b
plan_head_after: d9fc5770ed4f9f4ff07ff97cb818217abbdf91eb

tech-stack:
  added: []
  patterns:
    - "Read a canvas-rendered terminal share page by finding the xterm instance through the React fiber tree (readterm.js), so ticks and history are checked, not assumed"
    - "Time a viewer's join and reconnects from the page, with the guest epoch on every line, so a state change is attributed to the right second"

key-files:
  created:
    - .planning/phases/19-unattended-desktop-viewing/rig/readterm.js
    - .planning/phases/19-unattended-desktop-viewing/rig/tray.swift
    - .planning/phases/19-unattended-desktop-viewing/rig/scroll.swift
    - .planning/phases/19-unattended-desktop-viewing/evidence/ (F1 to F13 captures)
  modified:
    - .planning/phases/19-unattended-desktop-viewing/rig/p19-rig.sh
    - .planning/phases/19-unattended-desktop-viewing/19-VM-CHECKS.md

key-decisions:
  - "No product code was changed. The one defect found (clean stops counted as crashes) needs a code fix, a test and a re-run of F4 and F5 on the fixed build; it is recorded with the evidence and a recommended fix, not patched mid-matrix"
  - "F5 is recorded PASS on its recorded run with the tick-gap caveat stated in the row and the closure, not rounded to a clean pass: 3 s on the last run, 4 s on two earlier ones, cause not proven"
  - "F2 and F9 are PARTIAL, naming what the VMs cannot show (the 2 s join target over DERP; a revoke noticed while a capture is live)"

requirements-completed: [DSHR-01, DSHR-02, DESK-02, DESK-03, DPRM-02, DPRM-03, DPRM-04]
requirements-partial: [DESK-04, DPRM-01, DVWR-02]

coverage:
  - id: I1
    description: "Upgrade with the desktop on keeps both shells (same pids, TICK history unbroken, terminal link alive), the desktop viewer returns twice by itself on the unchanged link, the launchd job takes the sessions back within 90 s, permissions stay granted, only the tailnet address is bound"
    requirement: "DSHR-01"
    verification:
      - kind: vm-row
        ref: ".planning/phases/19-unattended-desktop-viewing/19-VM-CHECKS.md#F5"
        status: pass
    human_judgment: false
  - id: I2
    description: "The Settings Desktop group, setup sheet buttons, tray line and the Background service row behave on macOS 27.2 as the UI contract says, through the GUI"
    requirement: "DSHR-01"
    verification:
      - kind: vm-row
        ref: ".planning/phases/19-unattended-desktop-viewing/19-VM-CHECKS.md#F1"
        status: pass
    human_judgment: true
    rationale: "Structure and text were read through Accessibility; how it looks is an owner check (WebKit screenshots are blank in the VMs)"
  - id: I3
    description: "Three viewers at once each decode and read viewers 3; join times 1.1 to 3.6 s over DERP against a 2 s target"
    requirement: "DESK-04"
    verification:
      - kind: vm-row
        ref: ".planning/phases/19-unattended-desktop-viewing/19-VM-CHECKS.md#F2"
        status: partial
    human_judgment: true
    rationale: "The direct-path timing is the owner's M-157 row"
  - id: I4
    description: "Network loss (pf, 20 s) recovers by itself 12 s after the unblock; a SIGKILLed daemon is restarted by launchd and the viewer returns on the same link; Quit All and a GUI start serve the same link text"
    requirement: "DESK-03"
    verification:
      - kind: vm-row
        ref: ".planning/phases/19-unattended-desktop-viewing/19-VM-CHECKS.md#F3"
        status: pass
      - kind: vm-row
        ref: ".planning/phases/19-unattended-desktop-viewing/19-VM-CHECKS.md#F4"
        status: pass
    human_judgment: false
  - id: I5
    description: "Share-server lifetime both ways, the off card and the static off page after a reload"
    requirement: "DSHR-02"
    verification:
      - kind: vm-row
        ref: ".planning/phases/19-unattended-desktop-viewing/19-VM-CHECKS.md#F6"
        status: pass
      - kind: vm-row
        ref: ".planning/phases/19-unattended-desktop-viewing/19-VM-CHECKS.md#F12"
        status: pass
    human_judgment: false
  - id: I6
    description: "Display sleep and wake, lock, replayd restart and a still screen keep or restore the stream; a revoked Screen Recording grant is noticed at the next capture start, not while a capture is live"
    requirement: "DPRM-02"
    verification:
      - kind: vm-row
        ref: ".planning/phases/19-unattended-desktop-viewing/19-VM-CHECKS.md#F7"
        status: pass
      - kind: vm-row
        ref: ".planning/phases/19-unattended-desktop-viewing/19-VM-CHECKS.md#F9"
        status: partial
    human_judgment: false
  - id: I7
    description: "On a vmdesktop build exactly one notification per lapse, claimed by the GUI while it runs and by the daemon with the GUI quit"
    requirement: "DPRM-03"
    verification:
      - kind: vm-row
        ref: ".planning/phases/19-unattended-desktop-viewing/19-VM-CHECKS.md#F13"
        status: pass
    human_judgment: false
  - id: I8
    description: "DEFECT NOT FIXED: a clean daemon stop (upgrade handoff, Quit All) with a desktop viewer attached is counted as an abnormal capture exit; three in an hour pause capture until Test Capture is run at the Mac"
    requirement: "DPRM-02"
    verification: []
    human_judgment: true
    rationale: "Needs a decision and a code fix with a test; evidence in F4-breaker.txt and the F4 row"

duration: about 4h 30m
completed: 2026-11-26
---

# Phase 19 Plan 15: The final VM matrix on one pair of builds Summary

**Every Phase 19 VM row now has a verdict on the final code: the upgrade with the desktop on, the Settings flow, three viewers, network loss, a killed daemon, share-server lifetime, display sleep, lock, replayd, a still screen and the lapse sender all hold on macOS 27.2, with two PARTIAL rows that name what the VMs cannot show, and one product defect found (a clean daemon stop with a viewer attached counts as a crash and can pause the desktop) that is recorded with its fix but not made.**

## What was built

- **Task 1 (tracer, `4a7d9f36`):** rig additions (safari-wins/both, readterm.js, chrome, pf, lock, pbpaste-check, tray-items, tick-gaps) and row F5 on three build pairs (19.2.0 to 19.2.1, 19.2.1 to 19.2.2, 19.2.2 to 19.2.3). The recorded run keeps both shells on the same pids with TICK 1 to 1235 unbroken in the terminal viewer, the terminal link attached throughout, the desktop viewer back twice by itself on the same link, the launchd job re-homed within 37 s, permissions kept, UDP and TCP 7444 on the tailnet address only.
- **Task 2 (`5ff41bcd`):** F1 (Getting a link, Copy Link and the clipboard, both setup-sheet panes, the tray line off, Ready and Streaming, the disabled-agent fallback with the Background service row and Open Login Items, enable and re-home), F2, F3, F4, F6, F12.
- **Task 3 (`d9fc5770`):** F7, F8, F9, F10, F11, F13 and the Closure section.
- Builds: this tree, Developer ID, a fresh `frontend/dist` (the one on disk predated 19-13), versions 19.2.0 to 19.2.5 (the plan's 19.0.5 and 19.0.6 are below the 19.1.3 the VM held). The VM ended on the untagged 19.2.5.

## Verdicts

PASS: F1, F3, F4 (after a breaker reset), F5 (tick-gap caveat), F6, F7, F8, F10, F11, F12, F13. PARTIAL: F2 (first frame 1.1 to 3.6 s over DERP against a 2 s target; all three viewers decode, viewers 3), F9 (macOS keeps a running capture alive after a revoke; Needs permission shows at the next capture start and clears itself within 5 s of the grant returning). No FAIL.

## Deviations from Plan

### Auto-fixed Issues

None to product behaviour. Plan-shape and process notes:

**1. [Rule 3 - Blocking] A fresh frontend/dist was built from this tree**
- **Issue:** `frontend/dist` and `node_modules` do not exist in a worktree, and the dist in the main checkout predates the 19-13 Settings group and the 00ce7a25 sheet copy, so `--skip-frontend` would have tested old UI.
- **Fix:** symlinked the main checkout's `node_modules`, ran `tsc` and `vite build` directly (`pnpm run build` refuses a symlinked workspace), built with `--skip-frontend`, removed the symlink afterwards. No package was added or installed.

**2. [Plan text] Version numbers and build pairs**
- 19.2.x instead of 19.0.5 and 19.0.6 (the VM held 19.1.3), and F5 was run on three pairs, not one, to attribute the tick gap (see below). F13's tagged build was 19.2.4 and the untagged 19.2.5 replaced it.

**3. [Rule 4 avoided] No product fix for the crash-breaker defect**
- It needs a code change in `internal/desktop` and `internal/daemon/handoff.go`, a test that blocks a viewer's close handshake, and a re-run of F4 and F5 on the fixed build, which is more than the plan's small-fix allowance covers mid-matrix. Recorded with evidence and a recommended fix instead (see Issues).

**4. [Process] A share link appeared in my transcript once**
- A debug dump of the Share popover printed the terminal share link of one session (including its capability token) and a guest's tailnet address, and a `tailscale status` read printed the guests' tailnet addresses. None of it reached a file or a commit (evidence was grepped for 100.x addresses, ts.net names, `cap=` and home paths: clean) and the helper scripts scrub afterwards. The token belongs to a throwaway test VM session whose Share was turned off in F6; flagging it because the rule is that links never appear in the transcript.

**Total deviations:** 4 (1 blocking build step, 1 plan-shape, 1 deliberate non-fix, 1 process note).

## Issues Encountered

- **Crash-breaker defect (needs a decision).** Upgrades with a viewer attached and Quit All with a viewer attached left `captureActive` true, so the next daemon counted an abnormal exit; after two of those and one real `kill -9` the breaker opened (`capture_paused`, `Paused after repeated errors`) and F4 attempt 1 got no frames for 70 s; it reopened later the same way. A Test Capture from the setup sheet clears it. Likely cause: `Manager.Close` closes viewers first (a close handshake can take seconds over DERP) and only then clears the flag, while the handoff and Quit All paths can exit sooner. Fix: clear the flag at the start of `Manager.Close` and `closeDesktopForHandoff`, plus a test. Evidence: `evidence/F4-breaker.txt`, `F4-b1.*`, the F4 row and the 19-VM-CHECKS.md findings.
- **Shell tick gap at the first handoff (3 to 4 s).** Every ticking loop pauses once when the new build's daemon first starts, including tty-less orphan loops, so it is not the daemon's PTY handling as far as measured; a builtin monitor saw 0.55 s at most, an exec monitor 0.31 s, a CPU burn did not reproduce it. Cause unproven; pids, ticks and history intact. Owner repeat on the office Mac (closure item 11).
- The Settings dialog is shorter than the Sharing pane once the Background service row shows, so the Open Login Items button sits under the footer until the pane is scrolled (cosmetic).
- Old tick loops from earlier plans (`&!` outlives its session) were still running in portty-test; they were stopped at the end.
- The pre-existing `~/ptest/Portty` in portty-test-2 (13 connections to the share server) stayed running and did not interfere.

## VM end state

Both guests running on the tailnet. portty-test: untagged Developer ID 19.2.5, launchd daemon running, GUI quit, always-available off, no tagged build installed (404 on the debug route, no route text in the binary), Screen Recording and Accessibility granted. portty-test-2: Safari and Chrome closed, pf anchor empty and pf disabled, link files removed from both guests. Nothing was stopped; no `tailscale down`; the host contacted no tailnet address, link or guest browser.

## Known Stubs

None.

## Threat Flags

None new. T-19-48: nothing ran on the host except `wails build`, `codesign` and `swiftc` (compiling only); all capture, TCC, launchctl, pf and browser work ran in the guests. T-19-49: links were compared inside the VMs, evidence greps for 100.x, ts.net, `cap=` and home paths came back clean, VM machine names were replaced in the one capture that carried them.

## Self-Check: PASSED

- Files exist: `19-VM-CHECKS.md` (rows F1 to F13, `## Closure`), `rig/p19-rig.sh` (`bash -n` clean), `rig/readterm.js`, `rig/tray.swift`, `rig/scroll.swift`, the F1 to F13 captures in `evidence/`.
- Commits `4a7d9f36`, `5ff41bcd`, `d9fc5770` are ancestors of HEAD.
- Plan verification ran clean: row verdict greps for F1 to F13, no FAIL in F2, F3, F4, F6, F5 states present, `human_needed` appears 12 times (at least 7), evidence has no 100.x address, ts.net name or `cap=`, `bash tests/check-brand.sh` OK.
