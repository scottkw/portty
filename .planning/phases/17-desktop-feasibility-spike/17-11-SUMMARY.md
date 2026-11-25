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
  - "R01-host, R18-host, R20-host PARTIAL. Session 1: Screen Recording grant not given within 120 s, run never started. Session 2 (grant added by the user by hand): the spike crashed at about ready+66 s (SIGTRAP on the main thread in spk_app_run)"
  - "R20-host real-silicon numbers for 20 s of motion: hardware AVC encoder, 2560 x 1664, 29.10 Complete fps, encode latency mean 12.69 ms (p95 median 20.66), own CPU 29.6 percent of one core, SSIM 0.999024 (luma 0.999313)"
  - "R01-host: tailscale ping from VM home read DERP before and after; the viewer connected over ICE-lite but no viewer report exists (server crashed). R18-host not reached"
  - "Host revoked and proven clean: tccutil resets rc 0, post-revoke preflight all false, status NotFound, pgrep rc 1, launchctl Could not find service, both guard mutations shown"
  - "Matrix fix: R15-R17 VM results moved from the Routing table back into Rows"
affects: [17-12, DVWR-03, DPRM-04, DESK-01, DESK-07, FEAS-01]

actuals:
  tokens: 13896
  tasks: 3
  commits: 3
plan_head_before: 91408414c2c078a5d4c06432ac82f4619ea35673
plan_head_after: a87c496d
# commits measured: git rev-list --count 91408414..a87c496d = 3 (bbafab37, 43011602, a87c496d); session 2 adds a87c496d and its docs commit

tech-stack:
  added: []
  patterns: ["approval detection by polling the non-prompting `probe --preflight-only` launched with open -n -W, because tccd logs no AUTHREQ_RESULT authValue=2 for a grant toggled later in System Settings", "host run and revoke chained in one script so the host window closes as soon as the run ends"]

key-files:
  created:
    - .planning/phases/17-desktop-feasibility-spike/evidence/host-run-notes.txt
    - .planning/phases/17-desktop-feasibility-spike/evidence/r14-iphone-reports.json
    - .planning/phases/17-desktop-feasibility-spike/evidence/r14-frame-iphone-*.jpg (7 frames of VM office's screen)
    - .planning/phases/17-desktop-feasibility-spike/evidence/r18-r20-host-timeline.jsonl (session 2 host-run timeline, no address or token)
  modified:
    - spike/desktop/viewer.html
    - .planning/phases/17-desktop-feasibility-spike/17-SPIKE-MATRIX.md

key-decisions:
  - "R14 GO with the finding that iOS (Safari and Home Screen) starts video only from a user gesture; the Phase 21 DVWR-03 iPhone viewer must start playback from a tap"
  - "Screen Recording not approved within 120 s: per the orchestrator, no new checkpoint; R01-host, R18-host, R20-host and the R01, R18, R20 base rows close PARTIAL with basis user-skip (user-not-approved), follow-up re-run the host sequence in a new user-approved session"
  - "Session 2 crash: rows stay PARTIAL with basis ran; no re-run and no code change in this plan (a rebuild would void the user's manual grant, and a new host run needs a new go); grant revoked right after"

duration: 30min
completed: 2026-11-25
---

# Phase 17 Plan 11: iPhone row and host run Summary

**R14 GO on iPhone (iOS 27.2.1) in both ICE modes once a tap starts playback. The host run on this MacBook Air did not start in session 1 (no grant within 120 s); in session 2, with the grant the user added by hand, the spike crashed at about ready+66 s, after 20 s of motion: R20-host has real hardware-encoder numbers for that window (29.10 fps, 12.69 ms mean latency, 29.6 percent of one core, SSIM 0.999), R01-host pinged over DERP both times, R18-host was not reached. All three stay PARTIAL, and the host was revoked with no spike TCC rows left.**

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

## Task Commits

1. Task 1: host-run plan, `322c2357`
2. Task 2: user checkpoint (go, DND on, iPhone done, iOS 27.2.1), no commit
3. Task 3: R14, host attempt, revoke, matrix, `bbafab37`
4. Task 3 session 2: host run with the manual grant (crash), revoke, matrix and notes, `a87c496d`

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

- Session 2: the spike crashed on real hardware at about ready+66 s, in the AppKit run loop on the main thread. The same command line ran 180 s three times in VM office. Candidates: the private-window-picker dialog (the user's click on it, or the dialog closing), or the real display's 2560 x 1664 size and motion. Neither is shown. Next time, set GOTRACEBACK=crash or use a native crash reporter so a symbolicated report exists.
- The post-revoke probe (AXIsProcessTrusted) writes a denied Accessibility row after `tccutil reset`. The revoke order should reset after the last probe, or the probe should skip the AX check.

## Known Stubs

None.

## Next Phase Readiness

- 17-12 has R14 GO and R01, R18 and R20 PARTIAL (user-skip). SC1's real-Mac part (the live desktop of a real Mac with no STUN, TURN or cloud server) has not been shown. FEAS-01 and FEAS-02 are not marked complete here.
- To close the host rows: find the session 2 crash (in VM office first, with the dialog clicked at different times), then re-run the 17-11 host sequence in a new user-approved session. Adding the app by hand in System Settings works where `--request` cannot prompt on this Mac. Running from VM home to this Mac also needs a direct tailnet path, or SC1's clause stays unshown.

## Self-Check: PASSED
- FOUND: evidence/host-run-notes.txt, evidence/r14-iphone-reports.json, 7 r14-frame jpgs, 17-SPIKE-MATRIX.md rows R01-host, R14-safari, R14-homescreen, R18-host, R20-host
- FOUND: commits 322c2357, bbafab37 in HEAD's history
- Task 3 verify script: VERIFY-OK (GORUN=no, TS=no)
- Session 2: FOUND evidence/r18-r20-host-timeline.jsonl; FOUND commit a87c496d; verify script VERIFY-OK (GORUN=yes, TS=no); leak scan (100.64/10, 32-hex, ts.net, home path, host names) clean on notes, timeline and matrix
