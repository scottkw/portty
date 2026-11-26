---
phase: 19-unattended-desktop-viewing
plan: 12
subsystem: vm-proof
tags: [screencapturekit, videotoolbox, pion, safari, chrome, csp, tcc, pmset, notifications, launchd, vm, tart, macos-27]
status: complete

requires:
  - phase: 19-05
    provides: real capture, encoder, power assertions (untested on real hardware until here)
  - phase: 19-06
    provides: the viewer and its #debug counters (window.__desktopStats, page title)
  - phase: 19-07
    provides: rig/p19-rig.sh and 19-VM-CHECKS.md, the launchd start model
  - phase: 19-09
    provides: the Prober (fresh perm-probe child), the watchdog and the lapse sender chain
  - phase: 19-11
    provides: permissions and test-capture endpoints, the pane URL constants
provides:
  - "19-VM-CHECKS.md rows M1, V7, B1, P1, V6, V9 PASS, V1 FAIL, V8 not run, with evidence in evidence/"
  - "rig/p19-rig.sh: vm2, enable-desktop, safari-stats, safari-close, chrome-stats, assertions, lsof-desktop, pcap, perm-poll, tcc-log, notif-list"
  - "VM-only Manager.SimulateLapse and ClearSimulatedLapse, POST and DELETE /desktop/debug/lapse behind the vmdesktop build tag, guarded by tests/check-brand.sh"
affects: [19-13, 19-14, 19-15, 19-16]

actuals:
  tokens: 19000
  tasks: 3
  commits: 4
plan_head_before: 4467056541995c17d2e32dbe93fe90400b55abe6
plan_head_after: bdadc79758374ddd22af148cc75547d963cbe93a

tech-stack:
  added: []
  patterns:
    - "Rig output is scrubbed on the host by one sed (home path to ~, tailnet addresses and names and cap= tokens removed) before anything is kept; the share link goes office to home through one ssh pipe into a mode 600 file in each guest"
    - "Build-tagged production seam: an untagged empty hook slice (extraDesktopRoutes) plus a tagged file that appends to it, so the release build compiles without the tag and cannot carry the route"

key-files:
  created:
    - internal/desktop/lapse_vmdesktop.go
    - internal/desktop/lapse_vmdesktop_test.go
    - internal/daemon/desktop_vmdesktop.go
    - .planning/phases/19-unattended-desktop-viewing/evidence/ (M1, V7, B1, P1, V6, V9, V1 captures)
  modified:
    - internal/daemon/desktop_api.go
    - tests/check-brand.sh
    - .planning/phases/19-unattended-desktop-viewing/rig/p19-rig.sh
    - .planning/phases/19-unattended-desktop-viewing/19-VM-CHECKS.md

key-decisions:
  - "extraDesktopRoutes is declared in the untagged desktop_api.go (one var, one range loop): a var declared in the tagged file would break the release build that ranges over it"
  - "SimulateLapse sets the lapse's frame count out of reach and parks the watchdog in its awaiting phase, so live frames cannot end a simulated lapse; without that a still-screen frame cleared it within seconds (the test fails when that line is removed). The watchdog's own 60 s retry still ends it"
  - "The V1 notification failure is recorded as a known gap, not worked around, per the orchestrator's rule; the fix options are in 19-VM-CHECKS.md findings"
  - "V1 used a second Developer ID build (19.1.1, vmdesktop tag) deployed over 19.1.0 with the daemon handed off, then the plain 19.1.0 build deployed again, so no tagged build stays installed"

requirements-completed: [DVWR-02, DESK-01, DESK-02, DPRM-01, DPRM-04]
requirements-partial: [DPRM-03]

coverage:
  - id: H1
    description: "Real ScreenCaptureKit capture, VideoToolbox encoding and pion in the office VM reach Safari in the home VM over the tailnet with the GUI quit: 2560 x 1440 frames (the display's native size), state streaming, viewers 1, 0 CSP violations"
    requirement: "DESK-01"
    verification:
      - kind: vm-row
        ref: ".planning/phases/19-unattended-desktop-viewing/19-VM-CHECKS.md#M1"
        status: pass
    human_judgment: false
  - id: H2
    description: "Chrome 158 and Safari both decode under the share server's real header CSP, 0 violations, Chrome's Local Network prompt answered Allow (and decoding the same with it unanswered)"
    requirement: "DVWR-02"
    verification:
      - kind: vm-row
        ref: ".planning/phases/19-unattended-desktop-viewing/19-VM-CHECKS.md#V7"
        status: pass
    human_judgment: false
  - id: H3
    description: "The desktop's UDP 7444 and TCP 7444 sockets are bound only to the tailnet address; no Portty packet on UDP 3478, 19302 or 5353 during a viewer session"
    requirement: "DESK-02"
    verification:
      - kind: vm-row
        ref: ".planning/phases/19-unattended-desktop-viewing/19-VM-CHECKS.md#B1"
        status: pass
    human_judgment: false
  - id: H4
    description: "The AC assertion is held with always-available on and no viewer, both viewer assertions while a viewer watches, and the viewer pair is released after it leaves"
    requirement: "DPRM-04"
    verification:
      - kind: vm-row
        ref: ".planning/phases/19-unattended-desktop-viewing/19-VM-CHECKS.md#P1"
        status: pass
    human_judgment: false
  - id: H5
    description: "The fresh perm-probe child under the launchd daemon reads the bundle's live Screen Recording grant both ways within 2 s of the toggle, tccd names app.portty.Portty, no prompt or dialog in 62 s of polling; both System Settings URL constants open the right panes"
    requirement: "DPRM-01"
    verification:
      - kind: vm-row
        ref: ".planning/phases/19-unattended-desktop-viewing/19-VM-CHECKS.md#V6"
        status: pass
      - kind: vm-row
        ref: ".planning/phases/19-unattended-desktop-viewing/19-VM-CHECKS.md#V9"
        status: pass
    human_judgment: false
  - id: H6
    description: "The simulated lapse takes the production transition (one LapseSeq, one daemon claim, the viewer's Waiting for approval card) and the route never exists without the vmdesktop tag; the daemon's own native notification with the GUI quit is NOT delivered"
    requirement: "DPRM-03"
    verification:
      - kind: unit
        ref: "internal/desktop/lapse_vmdesktop_test.go#TestSimulateLapse_OneNotificationHeldUntilCleared"
        status: pass
      - kind: vm-row
        ref: ".planning/phases/19-unattended-desktop-viewing/19-VM-CHECKS.md#V1"
        status: fail
    human_judgment: true
    rationale: "The daemon notification half failed in the VM (known gap for 19-16); the owner decides which fix to take"

duration: about 2h 20m
completed: 2026-11-26
---

# Phase 19 Plan 12: Real media path, permissions, power and notification rows in the VMs Summary

The real capture, encode, WebRTC and browser path works end to end on macOS 27.2 (Safari and Chrome in portty-test-2 decode 2560 x 1440 frames from portty-test over the tailnet with the GUI quit, 0 CSP violations, binds tailnet-only, no STUN or mDNS traffic, power assertions follow viewers, the fresh permission probe tracks the live grant both ways), but the daemon cannot post the lapse notification when no GUI runs (row V1 FAIL, a known gap with fix options).

## What was built

- **Task 1 (tracer, `38cdcb45`):** the rig gains `vm2`, `enable-desktop`, `safari-stats`, `safari-close`, `chrome-stats`, `assertions`, `lsof-desktop`, `pcap`. A Developer ID build 19.1.0 (no VM tag, real tailnet) went to portty-test; the daemon's Test Capture raised the Screen Recording prompt, the pane listed Portty, the toggle took the admin password, and the fresh probe read `sr:true` straight away (Later on Quit and Reopen). The second real capture raised the 27.2 picker-bypass dialog (Allow). Accessibility was granted in its pane. Always-available on, GUI quit, and Safari in portty-test-2 (safaridriver) decoded live frames at 2560 x 1440. Row M1 PASS.
- **Task 2 (`b2487a11`):** V7 (Chrome 158 installed in the home guest only, with its signature and Team ID verified; the Local Network prompt appeared, decoding worked before and after Allow), B1 (lsof and a pktap-attributed tcpdump), P1 (pmset assertions with and without a viewer). All PASS.
- **Task 3 (`13a33509`, `bdadc797`):** the vmdesktop tag files, the check-brand pattern (verified to fail when the tag is placed in `scripts/`), then V6 (two timed toggle cycles plus 62 s quiet polling, tccd summary), V9 (axdump titles), V1 (build 19.1.1, handoff, lapse, viewer card, database, daemon log) and the V8 line. V6 and V9 PASS, V1 FAIL.

## V1 verdict for 19-16 (named as the plan requires)

**V1 = FAIL.** What worked: the simulated lapse enters awaiting_approval through the watchdog's own transition, the viewer shows `Waiting for approval at the Mac`, one claim `desktop: lapse claimed lapse=2 by=daemon`, a second POST adds nothing, DELETE clears it. What did not: no notification reached Notification Center. The daemon logs `notification: dispatching native notification ...` and then nothing: the Objective-C `sendNotification` queues its work with `dispatch_async` on the main dispatch queue, which the headless launchd daemon never services, so no `Portty: dispatching native notification` log line, no usernoted record and no banner appear. The GUI process delivers (earlier records exist for the same bundle). **Known gap: with the GUI quit and nobody at the Mac, the user is not told that approval lapsed.** Options and a recommendation are in `19-VM-CHECKS.md` (findings): drop the main-queue hop when no NSApplication runs (recommended, a few lines in `tray_objc_darwin.m`), run a run loop for notifications in the daemon, or hand the notification to a GUI-session helper. This is a decision for the owner and 19-16; no product code was changed for it here.

## Deviations from Plan

### Auto-fixed Issues

None to product behaviour. Plan-shape notes:

**1. [Rule 3 - Plan text] `extraDesktopRoutes` lives in the untagged file**
- **Found during:** Task 3.
- **Issue:** the plan says to declare it "in this file" (the tagged file) if no hook exists; a var declared only under the tag would make the release build fail at `registerDesktopRoutes`.
- **Fix:** declared next to `registerDesktopRoutes` in `internal/daemon/desktop_api.go` with the one range call, as the plan's `files_modified` already listed that file. Verified: the untagged `go list` does not include `desktop_vmdesktop.go`, `go build ./...` and `go vet -tags vmdesktop` pass.
- **Commit:** `13a33509`

**2. [Rule 1 - Design] `SimulateLapse` needed a hold**
- **Issue:** the watchdog ends an open lapse on the next frame after it began, so a bare `enterAwaiting` was cleared within seconds on a still screen (frames arrive every few seconds there).
- **Fix:** after `enterAwaiting`, `noteAwaitSeq(MaxUint64)` and park `wd` in `wdAwaiting`; `ClearSimulatedLapse` calls `stallRecovered` under `tickMu`. A small tagged test holds the lapse through 40 ticks with frames emitted, and fails when the hold is removed (checked by mutation). The lapse still ends about a minute on through the watchdog's own retry cycle (recorded in the findings).
- **Commit:** `13a33509`

**3. [Plan text] Two verify details**
- The daemon logs no capture size, so M1 compares `frameWidth` with the display's native pixel width (2560, `screencapture`). The log line the plan quotes as "lapse 1 claimed by daemon" is `lapse claimed lapse=N by=daemon`.
- The `unrun-verify` ledger `.planning/WINDOWS.md` does not exist, so nothing was appended there (the ledger is optional).

**Total deviations:** 3 plan-shape or design notes, 0 product fixes.

## Authentication gates

None. The guest admin password was typed for System Settings' own authorization prompts (and `safaridriver --enable`) as the orchestrator allowed.

## Issues Encountered

- **Pre-existing, not from this plan:** three loopback media tests in `internal/daemon` fail on this host with and without my edits (`TestDesktopAPI_OnLinkStreamTracer` and `TestHandoffDesktop_RollbackKeepsDesktop` "no keyframe within 10s", `TestDesktopAPI_ViewerCapRefusedBeforeUpgrade` "viewer 0 stopped receiving video"). Verified by running them with my `desktop_api.go` change and the new tagged file removed. Logged in `deferred-items.md`; not fixed (scope boundary). `internal/desktop` passes, including `-race` for the new tagged test.
- An exploratory `strings` call in the office guest made macOS note a developer-tools request (the guests have no Xcode tools); no dialog appeared and nothing else ran. No python3 or other xcode-select-triggering tool was used afterwards.
- The first Chrome launch took about 21 s to show a page (cold start in the VM); later launches took 2 s.

## VM end state

Both guests are running on the tailnet (BackendState Running). portty-test: Portty 19.1.0 (Developer ID, plain, no tag) in /Applications, GUI quit, running under launchd, always-available turned off, Screen Recording and Accessibility granted to app.portty.Portty, no tagged build installed, share link and thumbnail removed from ~/p19. portty-test-2: Chrome 158 added in /Applications (left installed), Safari session closed, link file and Chrome profile removed. Nothing was stopped; no `tailscale down`; the host never contacted a tailnet address or the share link.

## Known Stubs

None.

## Threat Flags

None new. T-19-42 is mitigated and shown: the tag guard fails when the tag is placed in `scripts/`, the plain build's binary has no route text and answers 404, and the tagged build was removed after V1 (`evidence/V1.tagcheck`). T-19-43: evidence was grepped for 100.x addresses, ts.net names, `cap=` and home paths (none); screenshots were read in the session and not committed.

## Self-Check: PASSED

- Files exist: lapse_vmdesktop.go, lapse_vmdesktop_test.go, desktop_vmdesktop.go, p19-rig.sh (`bash -n` clean), 19-VM-CHECKS.md with rows M1, V7, B1, P1, V6, V9, V1 and the V8 line, evidence/ captures.
- Commits `38cdcb45`, `b2487a11`, `13a33509`, `bdadc797` are ancestors of HEAD.
- Verification commands from the plan ran clean: `go vet -tags vmdesktop`, `go build ./...`, untagged package excludes the tagged file, `bash tests/check-brand.sh`, row verdict greps, evidence forbidden-pattern grep.
