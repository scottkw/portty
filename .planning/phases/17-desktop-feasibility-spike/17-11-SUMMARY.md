---
phase: 17-desktop-feasibility-spike
plan: 11
subsystem: desktop-spike
tags: [iphone, ios, webrtc, host-run, tcc, revoke, feas-01, r01, r14, r18, r20]
status: complete

requires:
  - phase: 17-desktop-feasibility-spike
    provides: "17-10 tested host-run command line (default, no key-input flag), signed Spike.app, VM office daemon-shape pipeline, viewer.html"
provides:
  - "R14 GO: iPhone Safari and the Home Screen web app (iOS 27.2.1) connect to VM office over the tailnet and decode H.264 42e01f in both ICE-lite and full ICE; iOS needs a user tap to start playback"
  - "viewer.html: explicit play() on track, a Tap to play button and a playback diag line"
  - "R01-host, R18-host, R20-host PARTIAL (basis: user-skip, user-not-approved): the Screen Recording grant was not given within the 120 s window, so the measured host run never started"
  - "Host revoked and proven clean: tccutil resets rc 0, post-revoke preflight all false, status NotFound, pgrep rc 1, launchctl Could not find service, both guard mutations shown"
  - "Matrix fix: R15-R17 VM results moved from the Routing table back into Rows"
affects: [17-12, DVWR-03, DPRM-04, DESK-01, DESK-07, FEAS-01]

actuals:
  tokens: 13896
  tasks: 3
  commits: 2
plan_head_before: 91408414c2c078a5d4c06432ac82f4619ea35673
plan_head_after: bbafab377831f57d6e00da2f2c42e37995871fc6

tech-stack:
  added: []
  patterns: ["approval detection by polling the non-prompting `probe --preflight-only` launched with open -n -W, because tccd logs no AUTHREQ_RESULT authValue=2 for a grant toggled later in System Settings", "host run and revoke chained in one script so the host window closes as soon as the run ends"]

key-files:
  created:
    - .planning/phases/17-desktop-feasibility-spike/evidence/host-run-notes.txt
    - .planning/phases/17-desktop-feasibility-spike/evidence/r14-iphone-reports.json
    - .planning/phases/17-desktop-feasibility-spike/evidence/r14-frame-iphone-*.jpg (7 frames of VM office's screen)
  modified:
    - spike/desktop/viewer.html
    - .planning/phases/17-desktop-feasibility-spike/17-SPIKE-MATRIX.md

key-decisions:
  - "R14 GO with the finding that iOS (Safari and Home Screen) starts video only from a user gesture; the Phase 21 DVWR-03 iPhone viewer must start playback from a tap"
  - "Screen Recording not approved within 120 s: per the orchestrator, no new checkpoint; R01-host, R18-host, R20-host and the R01, R18, R20 base rows close PARTIAL with basis user-skip (user-not-approved), follow-up re-run the host sequence in a new user-approved session"

duration: 30min
completed: 2026-11-25
---

# Phase 17 Plan 11: iPhone row and host run Summary

**R14 GO on iPhone (iOS 27.2.1) in both ICE modes once a tap starts playback; the host run on this MacBook Air never started because the Screen Recording grant was not given within 120 s, so R01-host, R18-host and R20-host are PARTIAL, and the host was revoked and proven clean.**

## Performance

- Duration: about 30 min (continuation from Task 3), host window 16:17:25 to 16:20:03 UTC (2.6 min of the 6.5 min budget)
- Tasks: 3 (Task 1 earlier, commit 322c2357; Task 2 the user checkpoint; Task 3 here)
- Files: 5 text files plus 7 frames

## Accomplishments

- R14: 12 reports pulled from VM office into `evidence/r14-iphone-reports.json` (no raw addresses or tokens; the server had redacted them), with 7 viewer frames. Before the viewer change iOS decoded frames but never showed a picture. After it, with a tap: safari lite 295 frames, safari full 266 (first frame 2223 ms), Home Screen lite 307 and 324 (2815 to 4471 ms), Home Screen full 202 (1189 ms). Codec video/H264 42e01f, 2560 x 1440.
- Host run: baseline taken (74.85 percent idle, the Tart VMs 57.8 and 4.6 percent CPU, no thermal warning). The `--request` launch made tccd take its notify path ("Service kTCCServiceScreenCapture does not allow prompting; returning denied"). 14 preflight polls over 122 s stayed false, so the measured run did not start. No viewer was opened toward the Mac, no ping was taken, no frame was captured, nothing was typed, and the screen was never slept or locked by the spike.
- Revoke: all three `tccutil reset ... app.portty.spike` exited 0. Post-revoke preflight read sr, ax and pe false. `spike status` read NotFound. The pgrep guard (host) gave sentinel rc 0 then rc 1 after kill, and the launchctl guard (VM office) gave loaded rc 0 then Could not find service after bootout. Final pgrep rc 1, and both spike labels report Could not find service.

## Task Commits

1. Task 1: host-run plan, `322c2357`
2. Task 2: user checkpoint (go, DND on, iPhone done, iOS 27.2.1), no commit
3. Task 3: R14, host attempt, revoke, matrix, `bbafab37`

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Matrix: R15-R17 results sat in the Routing table**
- Found during: Task 3 (the verify's base-row check passed only because of the misplaced lines)
- Issue: 17-06 overwrote the Routing lines for R15, R16 and R17 with their full result rows, while the Rows section still read PENDING for them.
- Fix: put back the original Routing lines (R15, R16 and R17 run in VM; text from commit caaa0afb) and moved the nine result rows into Rows in place of the PENDING lines. No result text was changed.
- Files: 17-SPIKE-MATRIX.md. Commit: bbafab37

**2. [Orchestrator instruction] No checkpoint when approval timed out**
- The plan returns a human-action checkpoint if approval does not arrive within 120 s. The orchestrator told me instead to record the rows as user-not-approved and continue cleanup, and I did that.

**3. [Rule 3 - Blocking] How approval was detected**
- I polled the non-prompting `probe --preflight-only` every 8 s through `open -n -W`, because tccd logs no AUTHREQ_RESULT authValue=2 when a grant is toggled later in System Settings. The tccd log was still captured and is quoted, filtered, in the notes.

**4. [Orchestrator instruction] viewer.html change committed with the evidence**
- The orchestrator made the iOS playback fix during the checkpoint, and it is committed as spike code.

## Issues Encountered

- On this Mac, tccd did not show a Screen Recording dialog for the `--request` launch. It notified and returned denied, while the same launch in VM office (R00) raised a dialog. The log cannot say whether anything reached the screen. Do Not Disturb was on, and the user may have been at a locked screen. The user's observation is needed.

## Known Stubs

None.

## Next Phase Readiness

- 17-12 has R14 GO and R01, R18 and R20 PARTIAL (user-skip). SC1's real-Mac part (the live desktop of a real Mac with no STUN, TURN or cloud server) has not been shown. FEAS-01 and FEAS-02 are not marked complete here.
- To close the host rows, re-run the 17-11 host sequence in a new user-approved session. Have the user turn on Screen Recording for Portty Spike in System Settings while the Mac is unlocked and they are watching, before the measured launch.

## Self-Check: PASSED
- FOUND: evidence/host-run-notes.txt, evidence/r14-iphone-reports.json, 7 r14-frame jpgs, 17-SPIKE-MATRIX.md rows R01-host, R14-safari, R14-homescreen, R18-host, R20-host
- FOUND: commits 322c2357, bbafab37 in HEAD's history
- Task 3 verify script: VERIFY-OK (GORUN=no, TS=no)
