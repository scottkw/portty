# GSD Debug Knowledge Base

Resolved debug sessions. Used by `gsd-debugger` to surface known-pattern hypotheses at the start of new investigations.

---

## selection-offset-copy — drag selects rows above the pointer on a scaled pane; Cmd-C copies nothing under a mouse TUI
- **Date:** 2026-11-21
- **Error patterns:** selection offset, highlight above pointer, wrong row selected, mouse report wrong row, copy does nothing, Cmd-C empty clipboard, Option-drag no selection, Claude Code fullscreen, CSS transform scale, xterm MouseService, lone viewer, stale grid
- **Root cause(s):** TerminalPanel.recomputeScale draws a pane that follows a larger session grid with transform: scale(s < 1), and xterm 6's MouseService maps the pointer with a post-transform rect over an unscaled cell, so selections and TUI mouse reports land at row*s (a lone viewer hits this until it types or resizes); Claude Code fullscreen captures the mouse, so xterm has no selection for Cmd-C, and Option-drag could not force one because macOptionClickForcesSelection was unset
- **Fix:** compensatePointerScale (frontend/src/lib/terminalScale.ts) wraps the terminal's one MouseService (getCoords, getMouseReportCoords) so the pointer is mapped into the element's unscaled box first (unscalePointer); macOptionClickForcesSelection: true in TerminalPanel's Terminal options
- **Files changed:** frontend/src/lib/terminalScale.ts, frontend/src/components/TerminalPanel.tsx, frontend/src/lib/terminalScale.test.ts, frontend/e2e/selection-offset.spec.ts, TESTING.md
- **Why not caught:** no gate existed for this class. vitest mocks xterm (no MouseService, no layout) and the scale tests assert only the transform string; no e2e drove a real pointer on a scaled pane; the scaling VM checks looked at rendering, not input. Two premises were also wrong in triage: "one viewer means no scale" (a lone viewer keeps another viewer's grid) and "Option-drag forces a selection" (only with macOptionClickForcesSelection).
- **Recurrence guard:** frontend/e2e/selection-offset.spec.ts (scaled and unscaled drags, the TUI mouse-report row, Option-drag + copy under mouse tracking); frontend/src/lib/terminalScale.test.ts "unscalePointer" and "compensatePointerScale"; a console.warn if an xterm upgrade drops `_core._mouseService`; TESTING.md M-136. Pattern: any CSS transform on term.element (or an ancestor) breaks xterm's pointer math; check pointer-to-cell mapping whenever a pane is scaled.
---


## rc-wr04-stall — "sessions stalled ~17 min" in a VM row was a guest wall-clock step
- **Date:** 2026-11-26
- **Error patterns:** session stalled, output stopped, ticks gapless, same pids, resumed when viewer joined, daemon.log gap, VM row, guest clock, timed, NTP, settimeofday
- **Root cause(s):** the Tart guest's clock fell ~17 min behind its RTC (VM not running for a while) and timed stepped it forward mid-row (guest 07:10:02 -> 07:27:05); tick arithmetic on guest wall-clock timestamps read the step as a stall
- **Fix:** none in code; evidence in .planning/phases/18-self-installing-updates/evidence/RC-WR04-clockstep.log
- **Files changed:** none (docs only)
- **Why not caught:** the rig derives elapsed time from guest `date`; nothing checks for clock steps
- **Recurrence guard:** before treating a VM-row time gap as a product stall, run `/usr/bin/log show --predicate 'process == "timed" AND eventMessage CONTAINS "Received time"'` over the window and look for a guest log line-count hole
---

## rtp-seq-restart-freeze — every desktop source restart froze existing viewers until the new random RTP sequence passed the old one
- **Date:** 2026-11-27
- **Error patterns:** frozen picture after stall recovered, frozen after wake, viewer receives 0 units after restart, RTP sequence, NewRandomSequencer, SRTP replay, rollover counter, restartSource, packetizer per run
- **Root cause(s):** fanOut built a new rtp.Packetizer with rtp.NewRandomSequencer() and a new timestamp base per capture run (audioOut likewise built a new audio clock), while each viewer's track, SSRC and SRTP context outlive restartSource, so SRTP dropped every packet behind the last accepted sequence
- **Fix:** one video packetizer (Manager.vpkt) and one audio clock (Manager.aclk) live on the Manager for its life; Manager.vlast stamps a new run's first video unit with the real gap
- **Files changed:** internal/desktop/manager.go, internal/desktop/audio.go, internal/desktop/fanout_test.go, internal/desktop/loopback_test.go, TESTING.md
- **Why not caught:** no test restarted the source with a viewer kept; the VM rows looked at the restart, not at an existing viewer's decoding after it
- **Recurrence guard:** TestFanout_SourceRestartKeepsRTPSequence and TestFanout_SourceRestartStampsTheGap (pion loopback); TESTING.md M-221 (VM, Safari and Chrome PASS 2026-11-27). Pattern: anything stamped into RTP (sequence, timestamp, SSRC) belongs to the viewer's track, not to a capture run
---

## dtls-closed-zombie-viewer — a viewer whose media closed kept counting until its socket dropped; silent guests took 5-7 s to end
- **Date:** 2026-11-28
- **Error patterns:** zombie viewer, viewer count stuck, DTLS close, PeerConnectionStateClosed, capture keeps running, guest stop slow, CloseNow waits, read lock
- **Root cause(s):** serveAdmitted's OnConnectionStateChange handled Connected and Failed only, so pion's Closed (after a DTLS close notice) never ended the viewer; closeSocket's 2 s ceiling called coder/websocket CloseNow, which waits for the read lock held by the blocked read loop (~5 s)
- **Fix:** Failed and Closed both end the session through cutViewer(1001); the read loop reads on a child context cancelled at the ceiling (viewer.drop)
- **Files changed:** internal/desktop/manager.go, internal/desktop/revoke.go, internal/desktop/goroutine_leak_test.go, internal/desktop/revoke_test.go
- **Recurrence guard:** TestFanout_JoinLeaveCyclesLeaveNoGoroutines/media_closed_socket_open, TestGuestStop_ThreeSilentPeersDoNotHoldItUp; VM row Z-01
---

## unlk01-late-success-retype — a slow Mac authorization made the unlock retype into an unlocked desktop and skip the re-lock
- **Date:** 2026-11-28
- **Error patterns:** unlock retype, Trying once more, late success, unlock_failed but Mac unlocked, relock_armed false, loginPressed twice, Screensaver authorization succeeded late, loaded VM
- **Root cause(s):** a 3 s check window shorter than a loaded Mac's authorization plus lock-flag clear; no fresh lock read before each key; a final failure verdict so a later unlock never armed the re-lock
- **Fix:** 10 s check window; lockedForKey before the wake key, each stroke and Return; a 30 s late watch that turns a late unlock into the normal success path (audit, armRelock)
- **Files changed:** internal/desktop/unlock.go, internal/desktop/lease.go, internal/desktop/unlock_test.go, internal/desktop/export_unlock_test.go
- **Recurrence guard:** TestUnlock_LateSuccessIsOneTyping, TestUnlock_UnlockedBeforeTheRetypeTypesNothing, TestUnlock_SuccessAfterTheVerdictCountsAsSuccess and siblings; VM rows U-03r, U-04r, U-10r
---
