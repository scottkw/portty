---
phase: 14-mobile-web-app
plan: 13
subsystem: verification
tags: [ios-simulator, xcuitest, headless, safari, home-screen, key-bar, pinch, keyboard-inset, reload, playwright-fixture]
requires:
  - phase: 14
    provides: plans 14-01..14-11 (fixture admin routes, key bar, pinch, keyboard inset, Reload, PDF Done layer) and 14-07's rig notes
provides:
  - 14-SIM-CHECKS.md rows S-00..S-08 with element frames in points, fixture readings and screenshot hashes
  - the Home Screen answer: the launch URL keeps session and cap, and the icon dies with the share
  - notes for 14-14 (key bar sits 85 to 117 pt above the keyboard, pinch-in tab-overview, blank fixture PDF)
affects: [14-14]
status: complete
actuals:
  tokens: 4000
  tasks: 2
  commits: 2
plan_head_before: 4fb792bc861cad093fb1b92f3ff388a2ec2a7e8b
plan_head_after: a32e828515c9d826185fe94adc5245fb3238f146
tech-stack:
  added: []
  patterns:
    - "headless simulator rig: simctl boot, per-run CA via simctl keychain, throwaway xcodegen XCUITest project in a mktemp dir, fixture admin routes as observations"
key-files:
  created:
    - .planning/phases/14-mobile-web-app/14-13-SUMMARY.md
  modified:
    - .planning/phases/14-mobile-web-app/14-SIM-CHECKS.md
key-decisions:
  - "S-02 is written FAIL on its literal 8 pt criterion (bar 117 pt above the keyboard), with the cause stated: Safari's own iOS 26 form-assist bar and URL pill sit between; 14-14 judges whether it matters"
  - "S-03 is PASS on the terminal-element pinch; the whole-web-view pinch-in that opened Safari's tab overview is recorded as a real-phone item, not hidden"
requirements-completed: []  # MOB-01..MOB-03 also need the real-phone check at 14-14, same as 14-10 and 14-11; requirements.mark-complete not run
---

# Phase 14 Plan 13: Headless iOS Simulator rows S-00..S-08 Summary

A throwaway iPhone 17 on the iOS 26.5 runtime, booted headless and driven by a throwaway XCUITest project against the loopback Playwright fixture, answered the moved iPhone checks: seven rows PASS and S-02 (keyboard inset) FAIL on its literal criterion. The decisive one, S-04, passed: the Home Screen icon's launch URL keeps `session` and `cap` (fixture info-hits 8 to 9), and the icon dies with the share (S-07).

## Rig

- Host macOS 27.2.1, Xcode 27.2 (27B52), runtime iOS 26.5 (23F77), device type iPhone 17 (402 x 874 pt), device `portty-p14` created, booted with `simctl boot` and `bootstatus -b`, trusted the fixture CA with `simctl keychain add-root-cert`, deleted at the end. UDID `BF6D8B7A-146D-44A3-BCE0-4462CCF4AF0B` (gone).
- Fixture built as `frontend/e2e/global-setup.ts` does (`GOTOOLCHAIN=go1.26.11`, tags `playwrightfixture,wailsassets`), loopback only.
- Runner: xcodegen project (empty SwiftUI host plus UI-test bundle, ad-hoc signing, Swift 5 language mode) built on the first `build-for-testing`; each row one or more methods run with `test-without-building`.
- Standalone web app process: `UIKitApplication:com.apple.webapp`, bundle identifier `com.apple.webapp`. XCUITest queried it directly, so no row used indirect observations.

## Row results

| Row | Result | One line of evidence |
|-----|--------|----------------------|
| S-00 | PASS | Xcode 27.2, iOS 26.5, iPhone 17, headless boot, `pgrep -x Simulator` empty throughout, CA trusted, runner built first try |
| S-01 | PASS | eight keys all at least 51 x 44 pt in one row; Esc gives `1b`; Ctrl then `c` gives `03` and Ctrl returns to off; `a--`, `"`, `b` plus two spaces give `612d2d`, `22`, `622020` |
| S-02 | FAIL | bar bottom 117 pt above the keyboard (68 pt in landscape), not within 8 pt; input element above the bar; Esc 51 x 44 before and after focusing (no zoom). Cause: Safari's form-assist bar and URL pill |
| S-03 | PASS | pinch out 2.0: one resize (3 to 4), cols 48 to 20, Esc width 51 both; pinch in on the terminal element 0.7: one resize (4 to 5), cols 20 to 28. Whole-web-view pinch-in opened Safari's tab overview |
| S-04 | PASS | Add sheet name "Portty", Open as Web App on; icon in the App Library only; launch gave info-hits 8 to 9, key bar, no address bar: query kept |
| S-05 | PASS | keys' bottom 38 pt above the window bottom, terminal top at 62 pt, identical in light and dark; armed Ctrl blue fill with white label in both |
| S-06 | PASS | Done at (354,62 49x44) hittable below the status bar; Done returned to the preview pane then the session tab without a relaunch; Safari path opens a new tab with a back link |
| S-07 | PASS | stopped banner with Reload (57 x 44); Reload gives Link No Longer Works; icon relaunch gives Link No Longer Works (info-hits stayed 10) |
| S-08 | PASS | share-on new link shows the key bar; `/__test__/end` gives "Session ended" banner with Reload (57 x 44) |

## Screenshot directory (not committed, 14-14 deletes it)

`/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/p14-shots.TJxqBvzGDp` (30 PNGs, names and sha256 in 14-SIM-CHECKS.md `## Screenshots`).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] First Safari launch landed on the start page**
- **Found during:** Task 1 (S-01)
- **Issue:** `system.open` raced Safari's first launch, so the link never loaded and the key bar was not found.
- **Fix:** one `simctl openurl` nudge on the throwaway device; every later run opened the link directly.
- **Files modified:** none (rig only)

**2. [Rule 3 - Blocking] Ctrl is a Switch, and a fresh method must `activate()` the app**
- **Found during:** Task 1 and Task 2
- **Fix:** query Ctrl as a switch (`value` 0 or 1); `activate()` at the start of methods that do not open a link.

**3. [Plan variance] S-03 pinch-in**
- **Issue:** `pinch(withScale: 0.5, velocity: -1.0)` on the whole web view opened Safari's tab overview twice and 0.8 changed nothing, so the plan's pinch-in could not be read as written.
- **Fix:** pinch-in 0.7 on the terminal element ("tab panel") gave the one-resize result; the tab-overview behaviour is recorded in S-03 and the notes for 14-14.

**4. [Plan variance] S-02 criterion**
- **Issue:** the 8 pt "directly above the keyboard" criterion fails because of Safari's own chrome, not hidden content. Written FAIL as measured, with the cause; 14-14 decides whether it needs a fix.

## Auth gates

None.

## Known Stubs

None. (The fixture serves no terminal output and its PDF is blank; those are fixture properties, recorded in the SIM-CHECKS notes, not product stubs.)

## Threat Flags

None. T-14-34 (headless, no host input, `pgrep -x Simulator` empty at start, boot and end), T-14-35 (CA only on the throwaway device, which is deleted), T-14-36 (temp directory deleted, only 14-SIM-CHECKS.md staged, no link in the evidence) and T-14-41 (S-06: Done visible and returns without a relaunch) are mitigated as planned.

## Not verified

- Real suspension and resume (WebKit bug 308073), cellular or Wi-Fi roaming, the Tailscale app's VPN, MOB-03's reconnect, and the real-hardware M-145 look: all stay for 14-14.
- Terminal text and PDF marks: the fixture has no terminal output and a blank one-page PDF, so no screenshot shows them; the two pinch-in screenshots are byte-identical and the grid readings are the evidence.
- WebKit build number (the fixture does not log the user agent).
- Whether the real phone puts the icon on the Home Screen page (simulator: App Library only) and whether a real pinch-in on the terminal opens the tab overview.
- Whether the 85 to 117 pt gap above the keyboard (Safari's form-assist bar) matters on the real phone.

## Self-Check: PASSED

- 14-SIM-CHECKS.md exists with rows S-00..S-08 (every row PASS or FAIL), `## Screenshots` and `## Cleanup`, no link pattern; plan verify greps pass.
- Commits `ba4a5567` and `a32e8285` are on `gsd/v1.2` (`git rev-list --count 4fb792bc..HEAD` was 2 before this summary).
- `simctl list devices` shows no `portty-p14`; `pgrep -x Simulator` empty; `git status --porcelain -- cmd/playwright-fixture/dist frontend/src/wailsjs` empty.
