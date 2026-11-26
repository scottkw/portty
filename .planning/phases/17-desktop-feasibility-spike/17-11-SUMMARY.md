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
  - "Session 3 (after the nil-font fix bf95b9a6): the full 180 s host run completed on this MacBook Air, no crash, no 'chart text skipped' line"
  - "R20-host GO (R20 GO): hardware AVC encoder, 2560 x 1664, full 60 s motion at 29.15 Complete fps, encode latency mean 12.85 ms (p95 median 20.94), own CPU 27.0 percent of one core, RSS flat at about 93 MB, SSIM 0.999327 (luma 0.999406), no thermal warning"
  - "R18-host NO-GO (R18 NO-GO): SCK stream ends with -3815 0.7 s after display sleep and does not come back after the wake; on battery the Mac went into system idle sleep 5.5 s after the display, suspending the server and the spike's monotonic-clock wake timer"
  - "R01-host PARTIAL: VM home Safari decoded 1853 frames (0 dropped) of the real Mac's display through the motion window, but tailscale ping read DERP before and after, so the no-relay clause of SC1 is still not shown"
  - "Sessions 1 and 2 (history): session 1 grant not given within 120 s; session 2 crashed at about ready+66 s (nil font in drawText:, fixed in bf95b9a6)"
  - "Host revoked and proven clean: tccutil resets rc 0, post-revoke preflight all false, status NotFound, pgrep rc 1, launchctl Could not find service, both guard mutations shown"
  - "Matrix fix: R15-R17 VM results moved from the Routing table back into Rows"
affects: [17-12, DVWR-03, DPRM-04, DESK-01, DESK-07, FEAS-01]

actuals:
  tokens: 13896
  tasks: 3
  commits: 7
plan_head_before: 91408414c2c078a5d4c06432ac82f4619ea35673
plan_head_after: f8b10d2e
# commits measured: git rev-list --count 91408414..f8b10d2e = 7 (322c2357, bbafab37, 43011602, a87c496d, 4b4fc9f5, bf95b9a6, f8b10d2e); session 3's docs commit follows

tech-stack:
  added: []
  patterns: ["approval detection by polling the non-prompting `probe --preflight-only` launched with open -n -W, because tccd logs no AUTHREQ_RESULT authValue=2 for a grant toggled later in System Settings", "host run and revoke chained in one script so the host window closes as soon as the run ends"]

key-files:
  created:
    - .planning/phases/17-desktop-feasibility-spike/evidence/host-run-notes.txt
    - .planning/phases/17-desktop-feasibility-spike/evidence/r14-iphone-reports.json
    - .planning/phases/17-desktop-feasibility-spike/evidence/r14-frame-iphone-*.jpg (7 frames of VM office's screen)
    - .planning/phases/17-desktop-feasibility-spike/evidence/r18-r20-host-timeline.jsonl (session 2 host-run timeline, no address or token)
    - .planning/phases/17-desktop-feasibility-spike/evidence/r18-r20-host-timeline-session3.jsonl (session 3 host-run timeline, full run)
    - .planning/phases/17-desktop-feasibility-spike/evidence/r01-host-report-safari-lite.json (session 3 viewer report, addresses null)
  modified:
    - spike/desktop/viewer.html
    - .planning/phases/17-desktop-feasibility-spike/17-SPIKE-MATRIX.md

key-decisions:
  - "R14 GO with the finding that iOS (Safari and Home Screen) starts video only from a user gesture; the Phase 21 DVWR-03 iPhone viewer must start playback from a tap"
  - "Screen Recording not approved within 120 s: per the orchestrator, no new checkpoint; R01-host, R18-host, R20-host and the R01, R18, R20 base rows close PARTIAL with basis user-skip (user-not-approved), follow-up re-run the host sequence in a new user-approved session"
  - "Session 2 crash: rows stay PARTIAL with basis ran; no re-run and no code change in this plan (a rebuild would void the user's manual grant, and a new host run needs a new go); grant revoked right after"
  - "Session 3: R20 GO and R18 NO-GO (the wake does not bring frames back without a stream restart, as R18-vm); R01 stays PARTIAL on the DERP path. Phase 19 must hold a system-sleep and display-awake assertion while a viewer watches (on battery a display-off idle-slept this Mac in 5.5 s) or restart the stream on -3815, and must not time across sleep with a monotonic timer"

duration: 30min
completed: 2026-11-25
---

# Phase 17 Plan 11: iPhone row and host run Summary

**R14 GO on iPhone (iOS 27.2.1) in both ICE modes once a tap starts playback. On this MacBook Air the third host session ran the full 180 s schedule after the nil-font fix: R20 GO (hardware AVC, 60 s of motion at 29.15 fps, 12.85 ms mean encode latency, 27.0 percent of one core, SSIM 0.9993), R18 NO-GO (the stream ends with -3815 at display sleep and does not return after the wake; on battery the Mac also idle-slept 5.5 s after the display), R01 still PARTIAL (VM home Safari decoded 1853 frames from the real Mac, but over DERP). The host was revoked with no spike TCC rows left.**

## Performance

- Duration: about 30 min (continuation from Task 3), host window 16:17:25 to 16:20:03 UTC (2.6 min of the 6.5 min budget)
- Tasks: 3 (Task 1 earlier, commit 322c2357; Task 2 the user checkpoint; Task 3 here)
- Files: 5 text files plus 7 frames

## Accomplishments

- R14: 12 reports pulled from VM office into `evidence/r14-iphone-reports.json` (no raw addresses or tokens; the server had redacted them), with 7 viewer frames. Before the viewer change iOS decoded frames but never showed a picture. After it, with a tap: safari lite 295 frames, safari full 266 (first frame 2223 ms), Home Screen lite 307 and 324 (2815 to 4471 ms), Home Screen full 202 (1189 ms). Codec video/H264 42e01f, 2560 x 1440.
- Host run: baseline taken (74.85 percent idle, the Tart VMs 57.8 and 4.6 percent CPU, no thermal warning). The `--request` launch made tccd take its notify path ("Service kTCCServiceScreenCapture does not allow prompting; returning denied"). 14 preflight polls over 122 s stayed false, so the measured run did not start. No viewer was opened toward the Mac, no ping was taken, no frame was captured, nothing was typed, and the screen was never slept or locked by the spike.
- Revoke: all three `tccutil reset ... app.portty.spike` exited 0. Post-revoke preflight read sr, ax and pe false. `spike status` read NotFound. The pgrep guard (host) gave sentinel rc 0 then rc 1 after kill, and the launchctl guard (VM office) gave loaded rc 0 then Could not find service after bootout. Final pgrep rc 1, and both spike labels report Could not find service.

## Session 2 (re-run with the user's manual grant, 2026-11-25)

- Host window 16:58:15 to 17:01:33 UTC (3.3 min), plus one Accessibility reset at 17:03:57. Default command line (a), no --request, no key-input flag, fresh token, Spike.app not rebuilt.
- The spike process exited between 16:59:34.4 (last timeline line) and 16:59:37 UTC (crash dump written): SIGTRAP, "signal arrived during cgo execution", goroutine 1 in `_Cfunc_spk_app_run` (bridge_darwin.go:225, called from hostrun.go:703). The native frame is unknown: Go printed the dump and exited, so no .ips report exists, and `log show` returned nothing without extra rights.
- The ready+15 s frame showed the test chart plus the macOS "Spike is requesting to bypass the system private window picker" dialog (Allow, Open System Settings), the same second dialog R00 saw in the VM. The frame was read and deleted, not committed.
- R20-host: see provides. Static window 4.76 fps (still chart), latency mean 20.94 ms, own CPU 2.35 percent. WindowServer 35.0 percent in the motion top sample. No thermal warning.
- R01-host: ping before and after both read DERP (37 to 74 ms). The server logged the viewer connected over ICE-lite at ready+9.3 s. The page reports at the end of its 95 s, and the server was gone by then, so the viewer side is not recorded.
- R18-host: the sleep was due at ready+120 s, and the spike died first. The run never slept, woke or locked the display.
- Revoke: three `tccutil reset ... app.portty.spike` rc 0, post-revoke preflight all false, status NotFound, pgrep guard rc 0 then 1, no launchd labels. A read-only look at the system TCC database found no ScreenCapture row (the user's manual entry is gone), but the post-revoke probe had written a denied Accessibility row (auth_value 0, system set) at 17:01:32. A second `tccutil reset Accessibility app.portty.spike` (rc 0) cleared it: 0 rows left.

## Session 3 (re-run after the nil-font fix, 2026-11-25)

- Host window 17:51:13 to 17:54:44 UTC (3.5 min). Same default command line (a), fresh token, Spike.app not rebuilt after the orchestrator's 17:49:34 preflight (sr true, ax false, pe false).
- The run completed: spike exit at 17:54:42 UTC (last timeline line 17:54:41.86), 36 perf lines, no crash dump, no "spike: chart text skipped" line in hostrun.err. The end came 14 s late by the wall clock because the spike's timers use the monotonic clock, which stopped during 18 s of system sleep.
- The ready+15 s frame showed only the test chart, the capture indicator and the pointer: no bypass-picker dialog this time and no notification. Read, then deleted with static.h264.
- R20-host: see provides. Static window 4.53 fps (still chart), latency mean 14.31 ms, own CPU 2.13 percent; top in motion: WindowServer 41.3, spike 18.1. No thermal warning.
- R18-host: displaysleepnow at ready+120.1 s, -3815 0.70 s later, locked and asleep at the next poll; system idle sleep 17:53:33 to 17:53:51 on battery; woken by keyboard or trackpad (pmset log) and unlocked 1.3 s later, not by the spike; the spike's wake call ran 10 s after that; 0 Complete frames for the remaining 55 s. The R18 viewer joined after the unlock and its report was lost to the shutdown race.
- R01-host: pings DERP before (38 to 58 ms) and after (39 to 52 ms); viewer report: connected 1770 ms, first frame 2372 ms, 1853 frames decoded, 0 dropped, 2560 x 1664, 42e01f, RTT median 45 ms.
- Revoke 17:54:42 to 17:54:44: three tccutil resets rc 0, post-revoke preflight all false, the probe's denied Accessibility row found and reset again at once (0 rows), status NotFound, pgrep guard rc 0 then 1, no launchd labels.

## Task Commits

1. Task 1: host-run plan, `322c2357`
2. Task 2: user checkpoint (go, DND on, iPhone done, iOS 27.2.1), no commit
3. Task 3: R14, host attempt, revoke, matrix, `bbafab37`
4. Task 3 session 2: host run with the manual grant (crash), revoke, matrix and notes, `a87c496d` (docs `4b4fc9f5`)
5. Crash diagnosis and spike fix (nil font in drawText:), `bf95b9a6`
6. Task 3 session 3: full host run, revoke, matrix and notes, `f8b10d2e`

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

- Session 2: the spike crashed on real hardware at about ready+66 s, in the AppKit run loop on the main thread. Resolved: a transient nil font reached an unchecked dictionary literal in drawText: (evidence/host-run-crash-diagnosis.txt); fixed in bf95b9a6, and session 3 ran the full schedule with no "chart text skipped" line, so the nil did not recur this time.
- Session 3: on battery, a display sleep took this Mac to system idle sleep within 6 s even though another process held a PreventUserIdleSystemSleep assertion (why it did not hold is not established). Anything that must keep serving across a display sleep needs a stronger assertion and its own test on battery.
- The post-revoke probe (AXIsProcessTrusted) writes a denied Accessibility row after `tccutil reset`. The revoke order should reset after the last probe, or the probe should skip the AX check.

## Known Stubs

None.

## Next Phase Readiness

- 17-12 has R14 GO, R20 GO, R18 NO-GO (fallback DPRM-04 limited to keep-awake-while-viewing) and R01 PARTIAL. The real Mac's desktop was shown live over the tailnet, but over DERP, so SC1's "no STUN, TURN or cloud server" clause is still not shown. FEAS-01 and FEAS-02 are not marked complete here.
- To close R01: a run with a direct tailnet path between the viewer and this Mac (both pings direct). Adding the app by hand in System Settings works where `--request` cannot prompt on this Mac.

## Self-Check: PASSED
- Session 3: FOUND evidence/r18-r20-host-timeline-session3.jsonl, evidence/r01-host-report-safari-lite.json; FOUND commit f8b10d2e; verify script VERIFY-OK (GORUN=yes, TS=no); leak scan clean on the new evidence, notes and matrix
- FOUND: evidence/host-run-notes.txt, evidence/r14-iphone-reports.json, 7 r14-frame jpgs, 17-SPIKE-MATRIX.md rows R01-host, R14-safari, R14-homescreen, R18-host, R20-host
- FOUND: commits 322c2357, bbafab37 in HEAD's history
- Task 3 verify script: VERIFY-OK (GORUN=no, TS=no)
- Session 2: FOUND evidence/r18-r20-host-timeline.jsonl; FOUND commit a87c496d; verify script VERIFY-OK (GORUN=yes, TS=no); leak scan (100.64/10, 32-hex, ts.net, home path, host names) clean on notes, timeline and matrix
