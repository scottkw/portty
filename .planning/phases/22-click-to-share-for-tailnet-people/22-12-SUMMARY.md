---
phase: 22-click-to-share-for-tailnet-people
plan: 12
subsystem: vm-proof
tags: [vm, tart, click-to-share, guest-link, admit, tray, secure-event-input, notifications, audit, accessibility]
requires:
  - phase: 20-09
    provides: p20-rig.sh (probe route, textedit-new, safari helpers, dlog)
  - phase: 21-12
    provides: keystate helper and the guest VM state
  - phase: 22-11
    provides: Admit sheet, banners, quit sentence (the last UI the rows drive)
provides:
  - "22-VM-CHECKS.md: criteria written before any row ran, results for G-01..G-11, SC1 and AR-21-01 UNVERIFIED, Appendix B PARTIAL, User checkpoint section, notes and the owner-run list for 22-13"
  - "rig/p22-rig.sh, rig/axp22.swift and rig/rows/g01..g11.sh: the repeatable VM rig"
affects: [22-13]
key-files:
  created:
    - .planning/phases/22-click-to-share-for-tailnet-people/rig/p22-rig.sh
    - .planning/phases/22-click-to-share-for-tailnet-people/rig/axp22.swift
    - .planning/phases/22-click-to-share-for-tailnet-people/rig/rows/ (common.sh, g01..g11 incl. g04b)
    - .planning/phases/22-click-to-share-for-tailnet-people/22-VM-CHECKS.md
    - .planning/phases/22-click-to-share-for-tailnet-people/evidence/ (G-01..G-11, G-08 menu-bar crops, X-same-node-windows)
  modified: []
key-decisions:
  - "The Task 2 checkpoint was not returned: the orchestrator answered it on the user's behalf (permissions handled in the VMs, skip SC1, defer the five real-Mac items); the answers are recorded in the User checkpoint section as the orchestrator's decision"
  - "A second guest on a different tailnet node is the office VM's own Safari: two windows of one device cannot hold two steady viewers (evidence X-1)"
  - "G-04 is PARTIAL, not PASS and not FAIL: the criterion held in two of three measurements and one leave came 6.9 s after the share_stop under heavy host load; reported as an open item for DSHR-05"
  - "G-11 is PARTIAL: keys were dropped while a panel held focus, but by the Phase 20 'raising' check, so the new window_key_focus counter never rose"
requirements-completed: []
status: complete
commits: 4
plan_head_before: dfa1a208f1afe438b0bed480841613ac7eb9503f
plan_head_after: 61ec8869eab2281bb8b8361e3d2087a6266b8b0a
duration: about 3 h of VM time
completed: 2026-11-27
actuals:
  tokens: 34600
  tasks: 3
  commits: 4
---

# Phase 22 Plan 12: VM proof of click-to-share on two real macOS machines Summary

**The whole guest path ran between the two Tart VMs on macOS 27.2: a share started from the Portty GUI, a guest in Safari waiting at Admit, the Admit sheet answered through Accessibility, a moving View only picture, Allow control then a click and typing that reached TextEdit, Disconnect releasing a held key in 0.14 s, a physical stand-in click beating a controlling guest, the Secure Event Input banner in 1.4 s, notifications delivered while the display was captured, the tray line and icon variant, the GUI dying ending the share in 0.5 s, and an audit file at mode 0600 with every event and no secret. Two rows are PARTIAL and the second-account proof is UNVERIFIED.**

## Rows (what 22-13 reads)

| Row | Verdict |
|-----|---------|
| G-01 tracer (guest waits, Admit, view, Stop Sharing from the tray) | PASS |
| G-02 grant control, then click and typing post | PASS |
| G-03 revoke while the guest holds a key | PASS (key released 0.14 s after the press) |
| G-04 Stop with two guests | **PARTIAL** (two timed runs: leaves within 1.5 s; the tray run had a 6.9 s straggler under host load) |
| G-05 physical input wins | PASS |
| G-06 Secure Event Input banner | PASS (seen 1.38 s, gone 1.56 s) |
| G-07 notifications while captured | PASS (admit, join and leave all delivered) |
| G-08 tray line and icon variant | PASS (light menu bar only) |
| G-09 GUI quit ends the share | PASS (`gui_gone`, 0.51 s) |
| G-10 audit file | PASS (0600, all events, no secrets) |
| G-11 window-share key focus (M-05) | **PARTIAL** (`window_key_focus` path not reached on 27.2) |

SC1 (DSHR-03 live two-account proof): **UNVERIFIED** ("skip SC1", no second Tailscale user). Never a pass.

AR-21-01 (own-account peer filter, live): **UNVERIFIED** (needs the same second account).

Appendix B (real-Mac physical input): **PARTIAL** by reference to `evidence/appendix-b-real-mac.txt`; the warp check and the real-hardware Secure Keyboard Entry check are deferred to an owner-run real-Mac item.

No row is a FAIL. DSHR-03 and DSHR-05 are the two requirements to watch: DSHR-03's live proof is unverified, and the G-04 straggler is an open item for DSHR-05 (see below).

## Owner-run M-NN candidates for 22-13

1. SC1: second Tailscale user opens the guest link, waits at Admit, watches view-only, gets the own-devices refusal on a terminal link from the same device.
2. AR-21-01: a shared-in node from another account is not offered as a Desktop row or terminal peer and a typed connect is refused as unknown host.
3. Tray icon variant beside the other tray icons in a light and a dark menu bar (deferred; VM showed light only).
4. Admit sheet arriving while typing in a real terminal: stray Return, Space or Esc answers nobody; a click within about 600 ms does nothing (deferred).
5. Right-to-left or look-alike display name on the real Admit sheet (deferred).
6. "Someone is using this Mac" for a controlling guest while the owner types at the Mac with real hardware (deferred).
7. Warp check and the real-hardware Secure Keyboard Entry check (deferred; completes Appendix B).
8. M-05 with a real non-activating panel (Alfred or Raycast style): keys dropped as `window_key_focus`, nothing types into the panel (G-11 PARTIAL).
9. Stop with two real guests, both leave within 2 s on a quiet machine (G-04 PARTIAL).
10. Admit sheet above the Share Desktop and Settings sheets and the Portty window forward on a knock; Share stopped banner after killing the daemon during a share; Quit GUI Only while sharing (from 22-09 and 22-11; not driven here).
11. A restrictive tailnet access rule blocks the guest and the page shows "Can't reach this Mac": NOT RUN (no restrictive policy on the test tailnet).

## What was done

- **Task 1 (tracer, G-01):** build 22.12.0 of HEAD (`wailsassets vmdesktop`, Developer ID) built from a scratch `git archive` on the host (compile only), installed in the office, criteria for every row written and committed first, then G-01 run end to end with nothing but AX presses on the office and WebDriver on the guest.
- **Task 2 (checkpoint, not returned):** the orchestrator's decision stands in for the user's answers. The daemon already held Screen Recording and Accessibility, so nothing needed granting. Recorded in the "User checkpoint" section of `22-VM-CHECKS.md` as "orchestrator decision on the user's behalf (2026-11-27)".
- **Task 3:** G-02..G-11 and the unverified rows. Each row has a script in `rig/rows/` and a scrubbed evidence file.

## Findings worth a look (none is fixed here)

- **G-04 straggler (DSHR-05).** Stop pressed in the tray with two guests: `share_stop` and the office guest's leave at 19:37:48, the guest VM's leave at 19:37:55. Two timed repeats through the same DELETE route left within 0.2 s and 1.5 s, and 0.2 s and 1.0 s. The host's load average read 36 to 55 during the session, so load is the likely cause, but it is not shown. Needs a quiet-machine re-measure.
- **G-11: `window_key_focus` unreachable with Spotlight or the Fonts panel on 27.2.** Both take the focused application, so the Phase 20 frontmost check drops the keys first (counter `raising`, the guest sees the "could not bring the window to the front" notice). The safety outcome holds (nothing posted, nothing typed); the 22-05 path is covered by unit tests only.
- **Two windows of one device fight** (X-1): a second Safari window on the same link made the daemon log a join and a leave every 1 to 3 s with one viewer at a time (the page closes its own connection; reason `viewer_closed`) and produced coalesced `10 viewers joined` and `16 viewers left` notifications. Cause not investigated.
- **A long-lived viewer's picture stalled once** (`status streaming`, `bytesReceived` frozen, no Reconnecting; a page reload fixed it). Seen in an earlier G-07 attempt, not in the final runs.
- **Relaunching the GUI after it was killed** needed `open -n`: `open` answered -600 because LaunchServices holds the surviving daemon as the running Portty app.
- **A node already admitted in a share is re-admitted without a knock**, as designed; G-07 starts a fresh share for that reason.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Row runs were repeated after rig bugs, not product bugs.** The stand-in click tool needed `click X Y` (the first G-05 run never clicked); bash 3.2 mis-parses escaped quotes inside `$( )` inside `"..."` (the window `select` JSON in G-11); `Allow control` is a toggle and was decided by page text instead of the daemon's answer; a sheet left open hides the banner from the AX tree (G-06); a static screen sends no new frames (G-07's liveness check now changes the screen first); `axp22` read the daemon's pid after the GUI was relaunched. Each is fixed in the rig and named in the row it affected.
- **Files modified:** rig scripts only. **Commits:** `a89de9a7`, `61ec8869`.

**2. [Rule 2 - Missing critical functionality in the rig] Names were leaking through two paths.** The p19 notification helper does not scrub login names (they are e-mail shaped), and Safari's page text and the AX tree carry a real display name. The rig's `scrub` now also masks e-mail-shaped strings and the VMs' short names, and an optional sed file (kept outside the repo) masks a real person's display name. The one early evidence file that held a login was overwritten by a re-run; every committed evidence file was grepped for the name, the login, `cap=`, tailnet addresses and `.ts.net` names: none.

### Process

**3. [Process] The Task 2 checkpoint was not returned.** The orchestrator's policy for this run answered it on the user's behalf; the answers are recorded as such.

**4. [Process] Commit trailer.** The harness prescribes `Claude Sonnet 5.5`, which wrote these commits; the plan's `Claude Opus 5.5` was not used (the same call Phase 21's plans made).

**5. [Process] Two evidence artifacts the plan did not list:** `evidence/G-08-*.png` (three 160x26 crops of the VM's own menu bar) and `evidence/X-same-node-windows.txt`.

## Auth gates

None.

## Known Stubs

None. No product code changed; only `.planning` files.

## Threat Flags

None new. T-22-38: every evidence file was scrubbed on capture and grepped before each commit; the guest link only ever travelled over one ssh pipe into a 0600 file on the VM and was removed at the end. T-22-39: nothing ran on the host except compiling and `ssh`/`tart ip` to the VMs; no Portty, capture, tap or injection on the host, no host tailscaled or 100.x contact, nothing touched in `~/Downloads` or `/Applications` on the host. T-22-SC: no package added.

## VM end state

- portty-test (office) and portty-test-2 (guest) both running; neither stopped or restarted.
- Office: Phase 22 build 22.12.0 installed (`vmdesktop` probe route present), GUI and daemon running, no share active, TextEdit and Terminal not running, no Safari, the link file removed, always-available as set before.
- Guest: Safari session closed, safaridriver stopped, link file removed; its Portty (Phase 21 build) untouched.

## Self-Check: PASSED

Created files (`p22-rig.sh`, `axp22.swift`, `rows/*.sh`, `22-VM-CHECKS.md`, the evidence files named in the rows) and the four task commits (`015e8590`, `a89de9a7`, `c4eda353`, `61ec8869`) verified present on the branch. `bash tests/check-brand.sh` clean; the plan's automated checks pass (10 `G-` rows, G-01 carries a verdict, G-02..G-10 carry results, the SC1 row carries UNVERIFIED, `appendix-b-real-mac.txt` referenced).
