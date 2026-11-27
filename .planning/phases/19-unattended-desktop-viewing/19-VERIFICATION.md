---
phase: 19-unattended-desktop-viewing
verified: 2026-11-26T23:59:00Z
status: human_needed
score: 5/5 must-haves verified
covered_files:
  - ".planning/phases/19-unattended-desktop-viewing/19-01-PLAN.md"
  - ".planning/phases/19-unattended-desktop-viewing/19-01-SUMMARY.md"
  - ".planning/phases/19-unattended-desktop-viewing/19-02-PLAN.md"
  - ".planning/phases/19-unattended-desktop-viewing/19-02-SUMMARY.md"
  - ".planning/phases/19-unattended-desktop-viewing/19-03-PLAN.md"
  - ".planning/phases/19-unattended-desktop-viewing/19-03-SUMMARY.md"
  - ".planning/phases/19-unattended-desktop-viewing/19-04-PLAN.md"
  - ".planning/phases/19-unattended-desktop-viewing/19-04-SUMMARY.md"
  - ".planning/phases/19-unattended-desktop-viewing/19-05-PLAN.md"
  - ".planning/phases/19-unattended-desktop-viewing/19-05-SUMMARY.md"
  - ".planning/phases/19-unattended-desktop-viewing/19-06-PLAN.md"
  - ".planning/phases/19-unattended-desktop-viewing/19-06-SUMMARY.md"
  - ".planning/phases/19-unattended-desktop-viewing/19-07-PLAN.md"
  - ".planning/phases/19-unattended-desktop-viewing/19-07-SUMMARY.md"
  - ".planning/phases/19-unattended-desktop-viewing/19-08-PLAN.md"
  - ".planning/phases/19-unattended-desktop-viewing/19-08-SUMMARY.md"
  - ".planning/phases/19-unattended-desktop-viewing/19-09-PLAN.md"
  - ".planning/phases/19-unattended-desktop-viewing/19-09-SUMMARY.md"
  - ".planning/phases/19-unattended-desktop-viewing/19-10-PLAN.md"
  - ".planning/phases/19-unattended-desktop-viewing/19-10-SUMMARY.md"
  - ".planning/phases/19-unattended-desktop-viewing/19-11-PLAN.md"
  - ".planning/phases/19-unattended-desktop-viewing/19-11-SUMMARY.md"
  - ".planning/phases/19-unattended-desktop-viewing/19-12-PLAN.md"
  - ".planning/phases/19-unattended-desktop-viewing/19-12-SUMMARY.md"
  - ".planning/phases/19-unattended-desktop-viewing/19-13-PLAN.md"
  - ".planning/phases/19-unattended-desktop-viewing/19-13-SUMMARY.md"
  - ".planning/phases/19-unattended-desktop-viewing/19-14-PLAN.md"
  - ".planning/phases/19-unattended-desktop-viewing/19-14-SUMMARY.md"
  - ".planning/phases/19-unattended-desktop-viewing/19-15-PLAN.md"
  - ".planning/phases/19-unattended-desktop-viewing/19-15-SUMMARY.md"
  - ".planning/phases/19-unattended-desktop-viewing/19-16-PLAN.md"
  - ".planning/phases/19-unattended-desktop-viewing/19-16-SUMMARY.md"
  - ".planning/phases/19-unattended-desktop-viewing/19-17-PLAN.md"
  - ".planning/phases/19-unattended-desktop-viewing/19-17-SUMMARY.md"
  - "TESTING.md"
  - "app.go"
  - "frontend/src/components/DesktopSetupSheet.tsx"
  - "frontend/src/components/DesktopView.tsx"
  - "frontend/src/lib/desktopClient.ts"
  - "internal/capability/desktoptoken.go"
  - "internal/daemon/desktop_api.go"
  - "internal/daemon/handoff.go"
  - "internal/daemon/launchd.go"
  - "internal/daemon/share.go"
  - "internal/desktop/bind.go"
  - "internal/desktop/manager.go"
  - "internal/desktop/peer.go"
  - "internal/desktop/permissions.go"
  - "internal/desktop/power.go"
  - "internal/desktop/state.go"
  - "internal/desktop/watchdog.go"
  - "internal/screencap/media_darwin.m"
  - "internal/screencap/sys_darwin.m"
  - "internal/webserver/desktop_routes.go"
  - "tray_common.go"
covered_digest: "v4:sha256:ccf77230ddf47ef5f4d7c8a7590af9b474a4e06931e10e7e35cddcd338279173"
behavior_unverified: 0
overrides_applied: 0
human_verification:
  - test: "Daemon-posted lapse notification with the GUI closed, on a notarized build on the office Mac (TESTING.md M-176)"
    expected: "One real notification banner 'Portty desktop needs approval at the Mac', none on recovery"
    why_human: "VM rows V1b and F13 pass on a Developer ID build only; a notarized build and a real banner need the physical Mac"
  - test: "Direct tailnet path between the office and home Macs, and first-frame time on it (M-157, M-173 step 3)"
    expected: "tailscale ping reads direct; no STUN, TURN or cloud packet; first frame within about 2 s"
    why_human: "The VMs reach each other over a DERP relay (F2 first frame 1.1 to 3.6 s); ROADMAP SC4 names the two real Macs"
  - test: "Battery display-sleep test with a viewer connected, and the AC system-sleep hold released on battery (DPRM-04)"
    expected: "Display stays awake while a viewer is attached; stream restarts by itself after a wake; Mac sleeps normally on battery with no viewer"
    why_human: "The VMs have no battery; ROADMAP SC2 says 'tested on battery'"
  - test: "Retina sharpness at the office display's native size (M-161)"
    expected: "Picture is sharp and fitted to the window"
    why_human: "Sharpness is a visual judgement; the VMs only prove 2560 x 1440 frames decode"
  - test: "iPhone Home Screen link across a daemon restart and across an upgrade"
    expected: "The saved link opens the live desktop after both, with tap-to-start"
    why_human: "Link text survives both in F4/F5 and the manifest has no start_url, but the phone itself is not in the VMs"
  - test: "Real 30-day Screen Recording re-approval prompt and capture behaviour while it waits (M-160)"
    expected: "Waiting for approval at the Mac shows on host and viewer and a notification is sent"
    why_human: "The lapse is simulated in the VMs (vmdesktop build tag); the real prompt needs the office Mac"
  - test: "Office Mac display sleep policy, and what a viewer sees across display sleep and wake there (M-159)"
    expected: "display_asleep shown, then automatic restart after the wake"
    why_human: "F7 covers the VM half only"
  - test: "FA-1 attribution and a fresh probe on the office Mac (M-163)"
    expected: "In-daemon capture host listed under Portty in Screen Recording; grant read live"
    why_human: "Needs the corporate Mac's TCC database"
  - test: "Office Mac Tailscale mode and the Settings refusal for userspace networking (M-164)"
    expected: "Network-extension mode accepted; userspace mode refused with a clear state"
    why_human: "Depends on the physical Mac's Tailscale install"
  - test: "Look and feel of the Desktop group, setup sheet, tray line, 256 x 160 capture box and viewer in real windows (M-179), including the setup sheet's how-to wording with a permission turned off"
    expected: "Matches 19-UI-SPEC and the macOS-native feel"
    why_human: "WebKit screenshots are blank in the VMs; no one has seen these surfaces rendered, and the how-to wording for a missing permission never displayed on 27.2"
  - test: "Repeat the F5 upgrade with a viewer attached on the office Mac and read the shell tick gap"
    expected: "Every shell keeps its PID and history; tick gap at most 3 s"
    why_human: "Gap was 3 s in the recorded run and 4 s in two earlier attempts in the VM, cause unproven; PIDs and history were intact every time"
---

# Phase 19: Unattended Desktop Viewing Verification Report

**Phase Goal:** Turn on always-available desktop on the office Mac once, then open its live desktop from home in Chrome or Safari any time, with nobody at the Mac and the GUI closed. Permissions are guided, the desktop's state is always visible, and the display stays awake while someone is watching. Viewers reconnect by themselves, desktop media never leaves the tailnet, and terminal sessions and their upgrade handoff are untouched.
**Verified:** 2026-11-26
**Status:** human_needed
**Re-verification:** No, initial verification

All 17 plans have delivered substantive, wired code. The phase goal is achieved in the codebase and proven on real macOS 27.2 in two Tart VMs. What remains are owner-run items that only the two physical MacBook Airs can show. They are listed as human verification, not gaps.

## Goal Achievement

### Observable Truths (ROADMAP success criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Guided Screen Recording and Accessibility grants with a test capture; own devices open the live desktop from a link in Chrome and Safari, fitted, with the GUI quit and nobody at the Mac (DPRM-01, DSHR-01, DESK-01, DVWR-02) | VERIFIED | Code: `DesktopSetupSheet.tsx`, `perm-probe` child process (19-11), `handleDesktopTestCapture`, `DesktopView.tsx` mounted by `detectMode()` returning `desktopview` for `/desktop/` (`main.tsx`), object-fit contain letterbox. VM rows F1 (Settings switch, Copy Link, two Open System Settings panes), S1 (Safari, GUI quit, 2560 x 1440 frames, `cspViolations` 0), S2 (Chrome 158 same link), 19-12 rows. Retina sharpness itself is a human item |
| 2 | Host and viewers see the real state; lapse shows "waiting for approval at the Mac" plus a native notification; display and system-sleep assertions while a viewer is connected, stream restarts after display sleep (DPRM-02, DPRM-03, DPRM-04) | VERIFIED (battery clause human) | `state.go` precedence enum, `watchdog.go` nudge-then-frame-check (no zero-frame inference), `permissions.go` approval-store read, `power.go` plus `sys_darwin.m` (`PreventUserIdleDisplaySleep`, `PreventUserIdleSystemSleep`, `IOPMAssertionDeclareUserActivity`), `processWake`, GUI one-sender rule (`claimAndPostLapse`, WR-07 reset), `tray_common.go` state line. VM rows F7 (display_asleep then streaming 2 s after wake), F8 (locked, live lock-screen frames), F10, F11 (no false lapse on a still screen over 62 s), F13 (exactly one notification per lapse, sender follows the GUI), V1b. F9 PARTIAL is a macOS fact: a revoked grant is noticed at the next capture start |
| 3 | Several viewers at once, join gets a picture right away; reconnect by itself after drop, sleep/wake or upgrade showing "Reconnecting"; Home Screen links survive restarts and upgrades (DESK-03, DESK-04, DSHR-02) | VERIFIED (iPhone clause human) | `desktopClient.ts` capped backoff, `visibilitychange`/`pageshow`/`online` wake handling, `reconnecting` status; `manager.go` fan-out with one shared encoder and IDR on join; `desktop.json` own file with grantID and iat; manifest without `start_url`. VM rows F2 (3 viewers, `viewers: 3`), F3 (pf block 20 s, same page back to streaming), F4 and F4b (SIGKILL and Quit All, same link text), F5, F5b and S5 (upgrade keeps shells), S4 |
| 4 | Media stays on the tailnet (no STUN/TURN/cloud, no non-tailnet bind, tests assert both); direct path on the two real Macs; desktop link never opens a terminal and the reverse (DESK-02, DSHR-08) | VERIFIED (direct-path clause human) | `bind.go` `checkBind` refuses unspecified, IPv6 and non-100.64/10; `peer.go` empty ICEServers, ICE-lite, UDP4, mDNS off, IP filter. Tests pass: `TestCheckBind_Boundaries`, `TestPeer_NoStunTurnMdnsTraffic`, `TestPeer_APIHasNoICEServersAndNoMDNS`, `TestPeer_AnswerHasOneHostCandidate`. DSHR-08: `TestCrossToken_Matrix` covers three token kinds against nine route families, both directions, plus the capability slot; HKDF-separated `VerifyDesktop`. VM row S1 `lsof`: UDP and TCP 7444 on the tailnet address only, nothing on `*:` |
| 5 | Terminal sessions never regress: upgrade with the desktop on hands every session over with PIDs and history intact; the share server is never taken from terminal viewers by the desktop toggle, and the reverse (DSHR-02, DESK-03) | VERIFIED | `serverNeededLocked()` is used by `stopServerIfIdleLocked` (share.go) and the handoff listener pass (handoff.go); HandoffVersion unchanged. Tests pass: `TestHandoffDesktop_*` (7 tests incl. rollback, no old-side listener, desktop off), `share_lifetime_test.go`. VM rows F5, F5b, S5 (shell PIDs unchanged, ticks continuous, terminal buffer unbroken across the upgrade) and F6 (each toggle leaves the other running) |

**Score:** 5/5 truths verified, 0 behavior-unverified (every cancellation, cleanup and ordering invariant has a named passing test or a VM row; see the spot-checks).

### Required Artifacts

`verify.artifacts` on plans 01 to 06, 08 to 11, 13, 14, 16 and 17 returned all artifacts present and substantive. The one reported miss, `internal/desktop/peer.go` pattern `SetLite(true)` (19-01), is a literal-text mismatch: `peer.go:66` calls `se.SetLite(sum.Lite)` with `Lite: true` at line 62, asserted by `TestPeer_*`. Plans 07, 12 and 15 are VM plans with no code artifacts; their output is `19-VM-CHECKS.md` and `evidence/`, which exist.

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/desktop/manager.go` | Fan-out, state loop, viewers, restart | VERIFIED | 979 lines, wired from `internal/daemon/desktop_api.go` |
| `internal/desktop/{bind,peer,power,watchdog,permissions,state}.go` | Hardened media, assertions, lapse, state | VERIFIED | Read; real logic, no stubs |
| `internal/screencap/{media,sys,preflight}_darwin.{go,m}` | ScreenCaptureKit, VideoToolbox, IOKit, CGSession | VERIFIED | Real bridges; cgo tests pass on this Mac |
| `internal/daemon/launchd.go`, `process.go` | LaunchAgent, kickstart-only start, fallback | VERIFIED | No `kickstart -k` in product code; fallback per the 2026-11-25 amendment (V2 d, C1, F1) |
| `internal/webserver/desktop_routes.go` | Route gates, refusal pages, admit re-check | VERIFIED | Origin, same-user, token order as decided |
| `internal/capability/desktoptoken.go` | HKDF-separated desktop token | VERIFIED | |
| `frontend/src/components/DesktopView.tsx`, `lib/desktopClient.ts` | Viewer | VERIFIED | 528-line client with reconnect |
| `frontend/src/components/DesktopSetupSheet.tsx` and Settings Desktop group | Guided permissions | VERIFIED | vitest 126 files, 1961 tests pass |
| `build/darwin/Info.plist` | LSMinimumSystemVersion 14.0 | VERIFIED | Both Info.plist and Info.dev.plist |

### Key Link Verification

| From | To | Status | Details |
|------|----|--------|---------|
| share server `/desktop/*` | `DesktopBackend` (Manager owner) | WIRED | `SetDesktopBackend`, `ServeDesktopViewer` with `admit` |
| `handlePutDesktop` | `desktop.json`, share server lifetime, Manager | WIRED | `serverNeededLocked` |
| handoff | share listener with zero terminal shares | WIRED | handoff.go `needed` branch; `TestHandoffDesktop_NoListenerFromOldSide` |
| GUI `startDaemon` | `daemon.EnsureAgent`, kickstart, re-home | WIRED | app.go `ensureAgentFunc`, `kickstartFunc`, `requestRehome` |
| daemon state | tray line, Settings group, notification | WIRED | `desktopTrayLine`, `DesktopStatusAsGUI`, `claimAndPostLapse` |
| `main.tsx` | `DesktopView` | WIRED | `detectMode()` |

### Data-Flow Trace (Level 4)

| Artifact | Data | Source | Real data | Status |
|----------|------|--------|-----------|--------|
| `DesktopView` frames | RTP video | pion track fed by VideoToolbox access units from ScreenCaptureKit | Yes, VM rows S1 and S2: 2560 x 1440 frames decoded in Safari and Chrome | FLOWING |
| Settings state line, tray line | `GET /desktop` status | `Manager.Status()` | Yes, F1, F7, F8 | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Full Go suite | `GOTOOLCHAIN=go1.26.11 go test -race -short ./...` | every package ok, including daemon, desktop, webserver, screencap, capability | PASS |
| Frontend suite | `npx vitest run` | 126 files, 1961 tests passed | PASS |
| Review-fix invariants (one run of the named tests, `-count=1 -race`) | `TestClose_ClearsCaptureActiveDuringBlockedStart`, `TestJoin_NotBlockedByAnotherViewersStart`, `TestFanout_RTPTimestampFollowsCaptureTime`, `TestCheckBind_Boundaries` | PASS | PASS |
| Handoff with desktop on | `TestHandoffDesktop_*` (7), `TestLaunchd_UnparsedPidCountsAsRunning`, `TestDesktopPermissions_PollRefreshesState` | PASS | PASS |
| DSHR-08 | `TestCrossToken_Matrix` (inside the webserver package run) | ok | PASS |
| Traceability | `bash tests/check-traceability-paths.sh` | OK: 272 paths exist | PASS |

### Probe Execution

SKIPPED: no `probe-*.sh` is declared by any Phase 19 plan; real-OS evidence is the VM rows in `19-VM-CHECKS.md`.

### Requirements Coverage

All 12 IDs appear in plan frontmatter (19-16 lists all 12) and in REQUIREMENTS.md mapped to Phase 19. No orphaned IDs: REQUIREMENTS.md maps no other ID to Phase 19, and DESK-05 to DESK-07 belong to Phases 20 and 23.

| Requirement | Source Plans | Status | Evidence |
|-------------|--------------|--------|----------|
| DPRM-01 | 05, 11, 12, 13, 15, 16 | SATISFIED | Setup sheet, perm-probe, test capture; F1, F9, V6 |
| DPRM-02 | 05, 09, 13, 14, 15 | SATISFIED | State enum, tray line, viewer chip; F1, F7, F8, F10, F11 |
| DPRM-03 | 09, 12, 13, 14, 15 | SATISFIED (real prompt human) | Three-signal detection, one-sender notification; F13, V1b |
| DPRM-04 | 05, 09, 12, 15 | SATISFIED (battery human) | IOPM assertions, wake restart; F7 |
| DESK-01 | 03, 05, 06, 12 | SATISFIED (Retina human) | 2560 px cap, fitted view; S1, S2 |
| DESK-02 | 03, 10, 12, 15 | SATISFIED (direct path human) | Bind and no-STUN tests, `lsof` in S1 |
| DESK-03 | 06, 08, 10, 15 | SATISFIED | Client reconnect; F3, F4, F4b, F5, S4 |
| DESK-04 | 01, 03, 15 | SATISFIED | Fan-out, IDR on join; F2, S2 |
| DSHR-01 | 02, 07, 08, 13, 15, 17 | SATISFIED as amended | launchd primary, detached fallback with needs-repair; V2, C1, F1 |
| DSHR-02 | 04, 08, 10, 15 | SATISFIED (iPhone human) | Persisted grant, same link after restart and upgrade; F4, F5, S5 |
| DSHR-08 | 01, 04 | SATISFIED | `TestCrossToken_Matrix` |
| DVWR-02 | 01, 04, 06, 12 | SATISFIED | Chrome and Safari in S1, S2, F2 |

REQUIREMENTS.md still shows these 12 as `Pending` and unchecked; flipping them is a tracking update for the orchestrator after the human items are accepted, not a code gap.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| phase files | n/a | `TBD`, `FIXME`, `XXX` markers | none found | No unreferenced debt markers in modified implementation files |
| viewer, `testing.Testing()` rails, encoder log | n/a | Review items IN-03, IN-04, IN-05 deferred with reasons in `19-REVIEW-DISPOSITION.md` | Info | Debug hook counts only, test rails, one log line per frame on a persistently failing encoder (bounded by the stall watchdog) |
| `internal/desktop/manager.go` | offer wait | WR-05 deadline fix has no automated test | Info | Wall-clock constant with no seam; noted in the fix report |
| `internal/desktop/manager.go` | join | WR-01 residual: the first viewer's own answer waits for its own capture start | Info | Documented; other viewers are not blocked (`TestJoin_NotBlockedByAnotherViewersStart`) |
| `internal/daemon/launchd.go` | n/a | WR-04 residual: print-to-bootout race window | Info | Documented residual, unparsed pid now counts as running |

Review state: CR-01 (signing-key regeneration did not cut a mid-handshake viewer) and WR-01 to WR-07 are fixed with regression tests, and the post-review smoke (S1 to S5, all PASS on a build with every fix) re-proved the live path. `19-SECURITY.md`: `threats_open: 0`. `19-UI-REVIEW.md`: all pillars 3/4, with its three items fixed (01bd357f, 08d2ee85, 7ac943e8).

### Notes on partial VM rows (not gaps)

- F2 PARTIAL: all three viewers decode and read `viewers: 3`; the only unmet part is the 2 s first-frame target, which the VMs cannot meet over a DERP relay (1.1 to 3.6 s). It belongs to the direct-path check below.
- F9 PARTIAL: macOS 27.2 keeps an already-authorised capture alive until it restarts, so a revoke shows `needs_permission` at the next capture start, not while streaming. This is platform behaviour and is recorded in the VM findings.
- F5 tick gap: the plan clause was 3 s; the recorded run was 3 s and two earlier attempts 4 s, with PIDs and history intact in all. Listed as a human item.
- Defect found by 19-15 (a clean daemon stop with a viewer attached counted as a crash and paused capture after three in an hour) was fixed in 20e38428 with a red-first test, and re-proved by F4b, F5b and S4.

### Human Verification Required

Eleven owner-run items on the physical Macs, listed in the frontmatter and recorded in `19-VM-CHECKS.md` (Closure) and TESTING.md Category BJ (M-173 to M-179, plus M-157, M-159, M-160, M-161, M-163, M-164). They are the direct tailnet path, battery behaviour, Retina sharpness, iPhone Home Screen link, the real 30-day approval prompt, the notarized-build notification banner, office display sleep policy, FA-1 attribution, Tailscale mode, the look in real windows, and the upgrade tick gap on the office Mac.

### Gaps Summary

No gaps. Every roadmap success criterion is backed by code that exists, is substantive and is wired, by passing automated tests (Go with race detector, vitest) and by real-macOS VM evidence. The status is `human_needed` solely because items that need the physical office and home MacBook Airs remain, as ROADMAP success criteria 2 and 4 name battery and the two real Macs explicitly.

---

_Verified: 2026-11-26_
_Verifier: Claude (gsd-verifier)_
