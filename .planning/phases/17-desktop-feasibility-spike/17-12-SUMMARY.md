---
phase: 17-desktop-feasibility-spike
plan: 12
subsystem: desktop-spike
tags: [feas-01, feas-02, capture-host, in-daemon, closure, sc4, cleanup, testing-md, archive]
status: complete

requires:
  - phase: 17-desktop-feasibility-spike
    provides: "17-01 decision rule (committed before any evidence); 17-02..17-11 row evidence; 17-11 session 3 host run and host revoke"
provides:
  - "FEAS-02 DECIDED: capture host in-daemon, decided by rule row 7 (DAEMON-RESPAWN true via R07-B); Pipeline shape: daemon, so no shape flip (evidence/feas02-rerun.txt: rerun: not needed)"
  - "Every matrix row closed: 24 base rows (19 GO, 2 NO-GO R06 and R18, 3 PARTIAL R01 and R08 user-accepted, R19 context-unanswerable); 30 sub-rows (25 GO, 2 NO-GO, 3 PARTIAL)"
  - "Closure checkpoint held (evidence/closure-decisions.txt): R01-vm, R01-host, R08-D user-accepted; R16-B re-run GO (9 of 10 daemon-typed unlocks; failure cause in loginwindow's log)"
  - "## Requirement changes in the matrix (sole source) and the Required user edits below"
  - "TESTING.md: Phase 17 Section 2 note and Category BH, M-157..M-164"
  - "SC4 holds (evidence/sc4-final.txt): 14 ok / 0 failed packages, brand clean, 228 traceability paths, 0 spike packages, nothing outside spike/, .planning/, TESTING.md changed since the phase base"
  - "VMs cleaned with guard mutations shown, both still running; host clean (status NotFound, pgrep rc 1, launchctl Could not find service)"
  - "spike/desktop archived in place (README.md, Status: ARCHIVED)"
affects: [19, 21, 23, DESK-02, DSHR-01, DPRM-01, DPRM-03, DPRM-04, DVWR-03, UNLK-01]

actuals:
  tokens: 65400
  tasks: 3
  commits: 4
plan_head_before: 18d14593f2f02869780abc75c753a7a317ff7be1
plan_head_after: 4cccd1d9
# commits measured: git rev-list --count 18d14593..4cccd1d9 = 4 (0aa87000, 08e6d3fe, ba40bd05, 4cccd1d9); the docs commit carrying this file follows
# tokens: chars/4 over git diff 18d14593..4cccd1d9 (261667 chars)

tech-stack:
  added: []
  patterns: ["residue gates proven by guard mutations inside the VMs (sentinel process, throwaway LaunchAgent, placeholder directories) before they are trusted on a clean rig"]

key-files:
  created:
    - spike/desktop/README.md
    - .planning/phases/17-desktop-feasibility-spike/evidence/sc4-final.txt
    - .planning/phases/17-desktop-feasibility-spike/evidence/cleanup-final.txt
    - .planning/phases/17-desktop-feasibility-spike/evidence/closure-decisions.txt
    - .planning/phases/17-desktop-feasibility-spike/evidence/feas02-rerun.txt
    - .planning/phases/17-desktop-feasibility-spike/evidence/r16-B-rerun.txt
    - .planning/phases/17-desktop-feasibility-spike/evidence/r16-B-rerun-try3.jsonl
  modified:
    - .planning/phases/17-desktop-feasibility-spike/17-SPIKE-MATRIX.md
    - TESTING.md

key-decisions:
  - "FEAS-02: capture host is in-daemon (rule row 7). Capture, encode and WebRTC serve run inside the existing daemon on its legacy LaunchAgent; one Mach-O, one TCC identity, no helper bundle"
  - "The CLI respawns the daemon only through launchd (launchctl kickstart, R07-B GO); a daemon started from an SSH shell is attributed to sshd-keygen-wrapper (R06 NO-GO)"
  - "UNLK-01 is feasible (R15, R16, R17 GO): ship, not cut; type after a wake key, check CGSSessionScreenIsLocked, retry once"
  - "Host tccutil proof for 17-12 accepted from 17-11 session 3's revoke (user reply: accept), because the permission system refused the host tccutil reset in 17-12"

duration: 43min
completed: 2026-11-25
---

# Phase 17 Plan 12: Phase closure, FEAS-02 decision, SC4 and cleanup Summary

**FEAS-02 is decided: the capture host is in-daemon (rule row 7, the same as 17-06's provisional pick, so no shape flip). Every matrix row is closed with evidence (24 base rows: 19 GO, 2 NO-GO, 3 PARTIAL by user acceptance or context), and each NO-GO or PARTIAL maps to the requirement it changes. TESTING.md has Category BH (M-157..M-164). SC4 holds: no spike code reaches the shipping build and the suite is unchanged. The VMs are clean and still running, the host is clean, and spike/desktop is archived in place.**

## Performance

- Duration: about 43 min (12:01 to 12:44 local, 2026-11-25), including the closure checkpoint and the R16-B re-run
- Tasks: 3 of 3
- Files: 9 changed (443 insertions, 40 deletions)

## Accomplishments

- Task 1: final evaluation of the pre-written rule. DAEMON-CORE YES (R04), HELPER-CORE YES (R02, R03), DAEMON-LISTED true (R09-B), DAEMON-RESPAWN true (R07-B), DAEMON-LOCK-OK true, so row 7 holds: in-daemon. Closure checkpoint held; R16-B re-run GO (9 of 10). `## Requirement changes` written with its `Sole source:` line.
- Task 2: TESTING.md Section 2 note (no new test files) and Section 5 Category BH, M-157..M-164; every matrix `follow-up:` marker replaced with its M-NN; traceability check passes (228 paths).
- Task 3: VM office and VM home cleaned (registrations, LaunchAgent, TCC rows, app copies, Chrome copy, scratch); residue gates shown to fail on left-behind state inside the VMs; host read-only checks clean; SC4 gates pass; spike/desktop/README.md archive notice; matrix header records SC4 and the archive.

## Task Commits

1. Task 1: FEAS-02 decision, rows closed, requirement changes: `0aa87000`
2. Task 2: TESTING.md Phase 17 note and Category BH: `08e6d3fe`
3. Task 3: VM cleanup, guards, SC4, archive: `ba40bd05`
4. Deviation fix: host tccutil line accepted from 17-11 session 3: `4cccd1d9`

## Verification

- Task 1 verify: pass (re-run after the fix).
- Task 2 verify: pass (re-run after the fix).
- Task 3 verify: every check passes except the literal `grep -qE '^host tccutil: ok$' evidence/cleanup-final.txt` (see Deviations). Run with only that line removed, the verify exits 0, including the live host checks (`spike status` reads NotFound, `pgrep -f 'build/Spike.app'` rc 1, `launchctl print` of app.portty.spike.daemon and app.portty.spike.helper rc 113 "Could not find service"), the VM checks over SSH and `tart list` showing both portty-test VMs running.

## Required user edits

Apply each before the named phase is planned. Source: the matrix `## Requirement changes` tables.

- .planning/REQUIREMENTS.md and .planning/ROADMAP.md (Phase 19 success criteria), DESK-02: changed. The "no STUN/TURN/cloud" clause was shown only on a DERP path between the test rigs; add a Phase 19 success criterion that the office Mac and the home Mac reach each other on a direct tailnet path (tailscale ping reads direct) with no STUN, TURN or cloud server. The WebRTC premise holds; no Plan B. Apply before Phase 19 is planned.
- .planning/REQUIREMENTS.md, DSHR-01 and DPRM-01 (R06 NO-GO): changed. The daemon is never started from an SSH shell; the CLI respawns it with `launchctl kickstart` through launchd. Apply before Phase 19 is planned.
- .planning/REQUIREMENTS.md, DSHR-01 (R08-D PARTIAL, user-accepted): changed. Accessibility for the daemon is claimed only for launchd and GUI start paths; an SSH-started daemon is unsupported. Apply before Phase 19 is planned.
- .planning/REQUIREMENTS.md and .planning/ROADMAP.md (Phase 19 success criteria), DPRM-04 (R18-host NO-GO): changed. The daemon holds display-awake and system-sleep assertions while a viewer is connected, and restarts a stream that ends on display sleep (SCError -3815) after the wake; waking a sleeping Mac from a viewer connection is not promised; lid-closed sleep stays excepted. Apply before Phase 19 is planned.
- R18-vm (reference row): none.
- .planning/REQUIREMENTS.md, DPRM-03 (R19 PARTIAL, context-unanswerable): changed. Lapse detection cannot rest on zero frames; detect by the approval store's LastAlerted plus 30 days where Full Disk Access allows, otherwise by a pointer nudge plus frame check; the real 30-day prompt is confirmed on the office Mac (M-160). Apply before Phase 19 is planned.
- .planning/REQUIREMENTS.md (requirement text and the Traceability status "gated by FEAS-01: ship or cut"), UNLK-01 (R15, R16, R17 GO): feasible, ship, not cut; unlock types after a wake key, checks CGSSessionScreenIsLocked and retries once; the password is never stored or logged. Apply before Phase 23 is planned.
- .planning/REQUIREMENTS.md, DVWR-03 (R14 GO): changed. The iPhone viewer (Safari and Home Screen) starts video from a tap. Apply before Phase 21 is planned.

Also, now that FEAS-02 is DECIDED: file Apple's persistent-content-capture entitlement request (`com.apple.developer.persistent-content-capture`). The capture host is the daemon inside the app bundle, so the request is for the app's own bundle ID, app.portty.Portty (no separate helper bundle ID). Use case: an unattended remote-desktop server on the user's own Mac, reached over the user's tailnet, with nobody present to answer the monthly Screen Recording re-approval prompt; Developer ID notarized distribution. Team, process and reported wait times are in .planning/todos/pending/2026-11-24-persistent-content-capture-entitlement.md (Claude can draft the request text on request).

## VMs

Both Tart VMs (portty-test and portty-test-2) are still running. Stopping them for hours removes them from the tailnet; bringing them back needs a re-key and a Tailnet Lock signature.

## Decisions Made

See `key-decisions` in the frontmatter. The full decision, term evidence, table walk and Phase 19 consequences are in 17-SPIKE-MATRIX.md `## FEAS-02 decision`.

## Deviations from Plan

### Accepted by the user

**1. [Verify literal] Host tccutil reset not re-run in 17-12**
- **Found during:** Task 3
- **Issue:** the plan's host step runs an idempotent `tccutil reset` for ScreenCapture, Accessibility and PostEvent and records `host tccutil: ok`. The permission system refused the host tccutil call, so it was not run (and not retried).
- **Resolution:** user reply "accept" (2026-11-25): 17-11 session 3's host revoke stands as the proof (evidence/host-run-notes.txt, "Revoke, session 3", 17:54:42 to 17:54:44: three resets rc 0; post-revoke preflight sr=false ax=false pe=false; the extra Accessibility reset rc 0; 0 portty or Spike rows in the system TCC database). evidence/cleanup-final.txt now reads `host tccutil: accepted from 17-11 session 3, not re-run in 17-12 (user reply: accept, 2026-11-25) ...`, not a bare `host tccutil: ok`.
- **Effect on verify:** Task 3's verify fails only on `grep -qE '^host tccutil: ok$'`; every other check, including the live host status, pgrep and launchctl reads, passes.
- **Files modified:** .planning/phases/17-desktop-feasibility-spike/evidence/cleanup-final.txt
- **Commit:** 4cccd1d9

### Plan vs orchestrator on REQUIREMENTS.md

The plan says this phase does not edit REQUIREMENTS.md or ROADMAP.md for requirement changes. That still holds: the requirement changes above are left to the user. The only edits to those files are the standard closure state updates the orchestrator asked for (FEAS-01 and FEAS-02 marked complete, Phase 17 progress 12/12). `requirements.mark-complete` reported both IDs `not_found` because it only advances a traceability row from `Pending` or `Gaps Found`, and these rows read `In Progress`; the two checkboxes and two rows were set to `[x]` and `Complete` by a direct edit instead.

## Known Stubs

None (docs, evidence and an archive README only).

## Next Phase Readiness

- Phase 19 planning was waiting on FEAS-02; that blocker is cleared. Apply the Required user edits for Phase 19 first.
- Open office-Mac follow-ups the decision rests on: M-163 (FA-1, in-daemon attribution and Settings entries on the office Mac) and M-164 (office Mac Tailscale mode).

## Self-Check: PASSED

All four commits are ancestors of HEAD; the README and the five evidence files exist; no address, tailnet domain, token or home path in this file.
