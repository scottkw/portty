---
phase: 21-portty-app-and-iphone-viewers
plan: 08
subsystem: desktop-tab
tags: [desktop-tab, desktop-view, local-network, hidden-window-pause, toolbar-label, occlusion]
requires:
  - phase: 21-02
    provides: HIDDEN_VISIBILITY (covered window stays visible to the web view), RELEASE_CHORD
  - phase: 21-04
    provides: Tab.type desktop, desktopTabId, DesktopRowState, sidebar Desktop row and desktopStates prop
  - phase: 21-05
    provides: createPeerDesktopOptions, createHiddenPause, desktopTabLabel, card tables
  - phase: 21-07
    provides: ConnectPeerDesktop, OpenPeerDesktopInBrowser, OpenLocalNetworkSettings bindings
provides:
  - DesktopTab (the app's Desktop tab) mounting the shared DesktopView over the loopback connection
  - Optional DesktopView embedding props (clientOptions, embedded, cards, hostCard, paused, onClient, onStatus, onDesktopState, onPicture, onCaptureChange, onModeChange, controlRef, hideCaptureNotices) and DesktopControlHandle
  - App open-or-focus, always-mounted desktop branch, desktopStates, per-tab toolbar labels
  - TabBar desktopLabel chip, .toolbar-chip--desktop, .desktop-tab styles
  - Native window occlusion event window:occluded bridged to portty:window-hidden
affects: [21-09, 21-12, 21-13]
tech-stack:
  added: []
  patterns: [optional-prop embedding with page-preserving defaults, hidden-not-display-none panel, hidden-window pause with a native occlusion fallback]
key-files:
  created:
    - frontend/src/components/DesktopTab.tsx
    - frontend/src/components/__tests__/DesktopTab.test.tsx
    - frontend/src/components/__tests__/App.desktop-tab.test.tsx
    - window_occlusion_darwin.go
    - window_occlusion_darwin.m
    - window_occlusion_darwin_test.go
  modified:
    - frontend/src/components/DesktopView.tsx
    - frontend/src/components/__tests__/DesktopView.test.tsx
    - frontend/src/components/TabBar.tsx
    - frontend/src/components/__tests__/TabBar.test.tsx
    - frontend/src/App.tsx
    - frontend/src/lib/peerDesktop.ts
    - frontend/src/style.css
    - app.go
key-decisions:
  - "Occlusion fallback written although 21-02's HIDDEN_VISIBILITY reads hidden for closed and menubar: the orchestrator directed it because occluded=visible (a covered window never fires visibilitychange); the plan's own gate text only covered closed or menubar"
  - "Resume after the hidden-window pause calls the options' beforeRedial (which runs ConnectPeerDesktop and refreshes the url) before client.start(), because createPeerDesktopOptions only refreshes before redials, never before a restart"
  - "Embedded, html:has(.desktop-view) background rule is scoped to non-embedded views so a mounted Desktop tab does not paint the whole Portty window black behind other tabs"
  - "onModeChange added to DesktopView (not in the plan's prop list): the bar's View Only choice is internal state and the label's View only line needs it; onControlChange's payload was left alone because Phase 20 tests assert it exactly"
  - "The muted-row colour rule skips the selected pill so on-accent text stays readable"
status: complete
actuals:
  tokens: 60000
  tasks: 3
  commits: 5
plan_head_before: 1d89a729781a6cab7ef505b38586936757ed0809
plan_head_after: 2e9f88c4384939fbec728090146ae2a4150db38e
---

# Phase 21 Plan 08: Desktop tab Summary

**A Desktop row click opens one Desktop tab per peer that streams the peer's desktop through the daemon's loopback proxy with the same DesktopView the browser page uses, shows the app's own cards (Local Network, Refused, Can't reach, off), stays connected behind other tabs, pauses after about 2 minutes of a hidden or covered window, feeds the sidebar's muted line, and shows a fixed-width keyboard-ownership label in the tab bar.**

## Precondition record

Phase 20 SUMMARYs 20-06, 20-07 and 20-08 exist; 21-VM-CHECKS.md carries `HIDDEN_VISIBILITY: closed=hidden menubar=hidden minimized=hidden occluded=visible`. Changes read before editing (`git log 060281bd..HEAD` on DesktopView.tsx, desktopClient.ts, style.css, App.tsx): fc87909f, c344b5bf, a9b4e9b8 (20-06 stage, input, ring, notice slot), 60e1e93c, 26f58bf2, e31f3825 (20-07 bar), dd7a6347, 9d0bff43, db356d0b (20-08 clipboard, chips), 6ae4ca65, f97d700e (20-09 splash fix).

## What was built

- **DesktopView** gained only optional props; with none passed the existing 135 DesktopView tests pass untouched, and the title, splash removal and `UserRefused` page are unchanged. Embedded it skips title, meta and splash, adds `desktop-view--embedded`, maps other-account to the Refused card, and hides Full Screen.
- **DesktopTab** builds the 21-05 options once per mount, disposes on a deferred tick (StrictMode-safe), shows the Local Network host card with its two actions, runs Reconnect (refresh url, then `wake('external')` while the client runs or `start()` after a final status), bridges `portty:wake`, runs the 2-minute hidden pause, and feeds the sidebar and the label. It never passes `onReconnectingChange`, so the amber dot is untouched.
- **App** opens or focuses one tab per peer, never mounts `TerminalPanel` for it (asserted), renders each desktop panel always-mounted with `desktop-tab--hidden`, `inert` and `aria-hidden` when not active, keeps `desktopStates` in memory pruned on every poll, and shows only the active tab's label.
- **Native**: `window_occlusion_darwin.go/.m` emit `window:occluded` (bool) for the Wails window; App re-dispatches it as `portty:window-hidden`; DesktopTab folds it into the pause's hidden signal. The observer refuses to install under `testing.Testing()`.

## Tests added

- `frontend/src/components/__tests__/DesktopTab.test.tsx` (17 cases)
- `frontend/src/components/__tests__/App.desktop-tab.test.tsx` (6 cases, loopback streaming walk with fake socket and peer connection)
- additions to `DesktopView.test.tsx` and `TabBar.test.tsx`
- `window_occlusion_darwin_test.go`

Verification run: `pnpm --dir frontend exec tsc --noEmit` clean; `pnpm --dir frontend test` 2404 passed (135 files); `bash tests/check-brand.sh` clean; `go vet .` clean. `go test -race -count=1 .` passes except `TestNoFakeWhoIsInRelease`, which fails only because `frontend/dist` is not built in this worktree (`pattern all:frontend/dist: no matches`); unrelated to this plan.

## TDD Gate Compliance

Tasks 2 and 3 ran RED then GREEN.

- **Task 2 RED** (`8265fe4b`): 19 target assertions failed on the planned behaviours (cards, hostCard, paused, onPicture, Local Network flag, Reconnect, wake, pause, state feed, sidebar muted line). Semantic assessment: every failure was an assertion on missing behaviour in code that compiled and ran under vitest; none was a load, import or fixture fault. GREEN `3c22c9f7`.
- **Task 3 RED** (`22303692`): 10 target assertions failed (TabBar label, onCaptureChange, controlRef, hideCaptureNotices, onModeChange, embedded Full Screen, label feed). Same assessment. GREEN `2e9f88c4`. One test (Controlling label) needed its fixture fixed in GREEN: the jsdom video had no size, so the click fell in the zero-size letterbox; the fixture now sizes the video as DesktopView.test does.
- The RED evidence classifier (`gsd_run check tdd-red-evidence`) was not run; RED was read from the vitest TAP output by hand.

## Deviations from Plan

**1. [Rule 3 - Blocking] Occlusion files written despite the plan's literal gate.** The plan writes `window_occlusion_darwin.*` only if HIDDEN_VISIBILITY reads visible for closed or menubar. It reads hidden for both, but the orchestrator directed the files as required because occluded reads visible (T-21-26 mitigation). Written and wired; plan files_modified lists them.
- **Files:** window_occlusion_darwin.go, window_occlusion_darwin.m, app.go (one `startWindowOcclusionObserver()` call in startup)
- **Commit:** 3c22c9f7

**2. [Rule 3 - Blocking] `html:has(.desktop-view)` made an embedded view paint html and body black.** Scoped the Phase 19 rule to `.desktop-view:not(.desktop-view--embedded)` so other tabs and the translucent window are unaffected. The /desktop/ page selector still matches.
- **File:** frontend/src/style.css
- **Commit:** 0b4978a1

**3. [Rule 3 - Blocking] Resume needs a fresh url.** `createPeerDesktopOptions` refreshes only before redials, so a plain `client.start()` after the pause would dial a possibly stale loopback port. DesktopTab calls `options.beforeRedial()` (which runs ConnectPeerDesktop) first; also exported `realClock` from peerDesktop.ts for the pause.
- **Files:** DesktopTab.tsx, frontend/src/lib/peerDesktop.ts
- **Commit:** 3c22c9f7

**4. [Rule 2 - Missing functionality] `onModeChange` prop.** The label's View only line needs the bar's chosen mode, which DesktopView kept internal; `onControlChange` was not extended because Phase 20 tests assert its exact payload.
- **Commit:** 2e9f88c4

**5. Muted-row rule excludes the selected pill** (`:not([aria-current="page"])`) so the selected row keeps on-accent text.

## Assumptions on Phase 20 as built

All matched without adaptation: `releaseAll(reason)` exists on the input controller, `onControlChange({available, captured})` and `controlMode` exist, the two capture notices are pushed from `onCapture` and `typingStays` (both now gated by `hideCaptureNotices`), the rights event gives `control`, and the control bar's Full Screen item is driven by `fullscreenAvailable` (hidden when embedded).

## For 21-09 and later plans

- **Release chord in the label:** `desktopTabLabel` and `LABEL_TOOLTIP` (21-05) still spell `Ctrl+Option+Cmd+Return`, as the plan and UI-SPEC say. CONTEXT line 63 moved the app's chord to `Ctrl+Option+Cmd+Shift+Return`; 21-09 applies it. Note the longer line 2 (about 37 characters at 12 px) may not fit the fixed 232 px chip: `.toolbar-chip__line2` is `nowrap` and the chip is `overflow: visible`, so check it in the VM row before accepting.
- **DesktopTab** takes `controlRef`; App does not pass one yet (21-09 owns the handle consumer).
- **DVWR-01 left Pending in REQUIREMENTS.md:** no VM or live-tailnet check ran here (hard constraints), so the requirement stays open for 21-12.
- TESTING.md, the traceability map and M-NN rows are 21-13's; the new test files (including the Go one) need Section 2 and 4 entries there.

## Known Stubs

None.

## Threat Flags

None. The tab handles the host, the loopback URL and error codes only (T-21-25); cards render fixed copy by enum and host names as React text (T-21-27); other-account maps to the Refused card (T-21-28); the terminal loop excludes `desktop` and the App test asserts no `TerminalPanel` for a desktop id (T-21-29); the hidden-window pause and occlusion fallback cover T-21-26 (office assertion measured by 21-12).

## Self-Check: PASSED

Created files and all five commits (0b4978a1, 8265fe4b, 3c22c9f7, 22303692, 2e9f88c4) verified present.
