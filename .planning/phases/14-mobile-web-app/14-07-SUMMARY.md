---
phase: 14-mobile-web-app
plan: 07
subsystem: test-rig
tags: [ios-simulator, home-screen, xcuitest, spike, mob-02]
requires:
  - phase: 14
    plan: 05
    provides: manifest, icons and head tags the Add to Home Screen sheet reads
  - phase: 14
    plan: 06
    provides: fixture CA PEM, info-hits, share-off and share-on routes
provides:
  - "H-00 and H-01 in 14-SIM-CHECKS.md: the Home Screen launch kept session and cap on iOS 26.5"
  - "Spike decision: continue (user, 2026-11-22)"
affects: [14-09, 14-13, 14-14]
status: complete
actuals:
  tokens: 2400
  tasks: 3
  commits: 2
plan_head_before: c19dfc2070fd760c65d46e7298e9afd6ea51b2ed
plan_head_after: b03003a8d60d075b7c81d992d60d7e88648be10d
tech-stack:
  added: []
  patterns:
    - "headless simctl boot plus a throwaway xcodegen XCUITest runner in a temp dir; host screenshots taken on request through a sync-file watcher"
key-files:
  created:
    - .planning/phases/14-mobile-web-app/14-SIM-CHECKS.md
  modified: []
key-decisions:
  - "Spike decision: continue. The icon keeps the token it was added with; re-adding after a re-share is accepted and the token-less start_url idea stays deferred"
requirements-completed: []  # MOB-02 stays open: the finished-UI run is 14-13 and the real-phone look is 14-14, so mark-complete was not run
---

# Phase 14 Plan 07: Early headless Home Screen spike Summary

On an iOS 26.5 simulator, launching the Portty icon added from the share link opened the share view with `session` and `cap` kept, and the user chose to continue the phase on the locked design.

## Accomplishments

- **Rig.** A throwaway iPhone 17 on the iOS 26.5 runtime, started with `simctl boot` (no Simulator.app, no host input), the loopback Playwright fixture over TLS with its CA trusted only on that device, and an XCUITest runner generated with xcodegen in a temp directory (Xcode 27.2). The runner drove Safari and SpringBoard, read the fixture's info-hits and posted to its admin routes.
- **H-00 PASS, H-01 PASS** in `14-SIM-CHECKS.md`. After Add to Home Screen (name field "Portty", Open as Web App on), Safari was terminated and info-hits read 9. Tapping the icon gave 10 within 10 s, and the screenshot showed the share view with no Safari address bar. The standalone process is `com.apple.webapp`.
- **Extra check (asked for by the caller).** With share off, the icon showed Link No Longer Works. After share-on issued a new token, the same icon still showed it. The icon keeps the token it was added with.
- **Decision.** The user answered "continue" and accepted re-adding the icon after a re-share. Recorded under `## Spike decision`, with notes for 14-09, 14-13 and 14-14.
- **Cleanup.** Device deleted, fixture stopped, temp directory removed, `pgrep -x Simulator` empty throughout. Screenshots kept in `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/p14-spike-shots.DL4VYuu3Bn` (not committed; 14-14 deletes them).

## Task commits

| Task | Commit | What |
|------|--------|------|
| 1 | 6c61c92c | `14-SIM-CHECKS.md` with header, H-00, H-01, screenshot list, spike cleanup |
| 2 | none | checkpoint:decision, user answered "continue" |
| 3 | b03003a8 | `## Spike decision` and notes for later plans |

## Deviations from Plan

None to the plan's scope. Notes:

- The rig needed one build fix (Swift 5 language mode in the throwaway project). The other iterations were UI element lookups (iOS 26 Safari keeps Share in the More menu, Add to Home Screen is a cell behind View More, the new icon is in the App Library), not rig failures, so the plan's three-fix limit was not reached.
- One run reused stale sync files, so its screenshots were not taken; that run was repeated on a freshly erased device with Safari terminated before the icon tap. The recorded H-01 numbers come from that clean run.
- The share-off and re-share steps are beyond the plan's rows; they were requested by the caller and are recorded as an extra check, not as H-01.

## Auth gates

None.

## Known Stubs

None.

## Threat Flags

None. T-14-39 (headless only, CA trusted only on a deleted throwaway device, no host defaults changed, CA key never written) and T-14-40 (H-01 answered before 14-09, 14-10 and 14-11) are mitigated as planned.

## Not verified

- The simulator is not a phone: no real suspension, cellular, roaming or Tailscale VPN. MOB-03 and WebKit bug 308073 stay real-phone items.
- Where the icon lands on a real phone (Home Screen page or App Library only) is open; on a fresh simulator it went only to the App Library.
- The share view was judged only by the Session tab and the absence of the Safari address bar and the error page, over an empty fixture terminal. The finished UI is 14-13.
- The 07 and 08 screenshots are byte-identical, so the second share-off step adds the info-hits reading (unchanged at 10) but no new visual.
- TESTING.md and traceability were not touched: this plan adds no test file to the repo, and the plan assigns TESTING.md to 14-12 and 14-14.

## Self-Check: PASSED

`14-SIM-CHECKS.md` exists with H-00, H-01 and `## Spike decision`; commits `6c61c92c` and `b03003a8` are ancestors of HEAD; the throwaway device is gone.
