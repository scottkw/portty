---
phase: 23-audio-adaptive-quality-unlock-and-release
plan: 08
subsystem: ui
tags: [react, desktop-viewer, iphone, quality, audio, tab-bar]
requires:
  - phase: 23-audio-adaptive-quality-unlock-and-release
    provides: "23-02 quality message and golden lines; 23-06 DesktopSoundButton, phone/compact props; 23-07 Unlock panel (phone placement)"
  - phase: 21-portty-app-and-iphone-viewers
    provides: the phone bottom bar and key bar (21-10), the Desktop tab and tab chrome (21-08)
provides:
  - desktopClient quality event { slow }
  - the Slow connection chip (DESK-07) in the Phase 19 chip slot
  - Sound as the iPhone bottom bar's last item; Show Keyboard greyed while Locked; empty bar slot while the Unlock field has focus
  - DesktopView onSound prop; DesktopTab onPlaying; the speaker mark on the app tab entry
affects: [23-10 VM rows, 23-12 TESTING.md, 23-14 real-iPhone rows]
plan_head_before: 7b97c952645caad5fc6e1b780ec528f819e7b19b
plan_head_after: d29801b125b839b26e3dfc2a72ee1e5452e52779
actuals:
  tokens: 30000
  tasks: 3
  commits: 3
tech-stack:
  added: []
  patterns:
    - "playing = sound on AND streaming AND not host-paused: derived in DesktopView, so every disconnect path clears the tab mark without extra wiring"
    - "the tab indicator is a per-tab set in App, not part of the active-tab-only toolbar label"
key-files:
  created:
    - frontend/src/components/__tests__/DesktopView.quality.test.tsx
    - frontend/src/components/__tests__/DesktopPhoneBars.sound.test.tsx
  modified:
    - frontend/src/lib/desktopClient.ts
    - frontend/src/components/DesktopView.tsx
    - frontend/src/components/DesktopPhoneBars.tsx
    - frontend/src/components/DesktopPhoneLayer.tsx
    - frontend/src/components/DesktopTab.tsx
    - frontend/src/components/TabBar.tsx
    - frontend/src/App.tsx
    - frontend/src/style.css
    - frontend/src/components/__tests__/TabBar.test.tsx
    - frontend/src/components/__tests__/DesktopTab.test.tsx
    - frontend/src/components/__tests__/DesktopView.sound.test.tsx
    - frontend/src/components/__tests__/DesktopPhoneBars.test.tsx
    - frontend/src/lib/__tests__/desktopClient.test.ts
key-decisions:
  - "The playing flag does not ride the toolbar label payload: that label is reported only for the active tab (null otherwise), but a background Desktop tab making noise is exactly when the tab entry needs the speaker. DesktopTab gets its own onPlaying callback and App keeps a Set of playing tab ids"
  - "The slow flag is also cleared when the connection is lost (reconnecting or unreachable), like the unlock message: it describes one connection"
  - "The Unlock field's focus is tracked with document focusin and focusout in DesktopView (the panel is a sibling of the stage and uncontrolled), and also reset when the panel goes away, because a focused element removed from the DOM does not fire focusout in every engine"
requirements-completed: [DESK-07, DCTL-07, UNLK-01]
duration: 40min
completed: 2026-11-27
status: complete
---

# Phase 23 Plan 08: Phone bar Sound item, tab playing indicator, quality chip Summary

A viewer the daemon sets below its floor now reads "Slow connection. The picture may freeze." (and nobody else does), the iPhone bottom bar ends in a 44 px Unmute/Mute item, and a Desktop tab in the Portty app shows a speaker while its sound is on.

## Precondition record

The four SUMMARY guards (21-08, 21-10, 23-02, 23-07) passed at base. `grep -c '"name":"quality-slow-true"'` on the golden file was 1 (quality-slow-false is line 22). `git log --oneline 1c55b159..HEAD -- DesktopView.tsx DesktopPhoneBars.tsx TabBar.tsx DesktopTab.tsx` at base, newest first: 79d3be8e, 76901ea5, 6bc11b3f, 4004dae8, f82c4c87, 98c47df9, 307277be, 64cdb045, d23310b1, 0af1d672, 7ce37551, ad8ca062, 15a44f6e, 48f23917, 951908f4, b41055f0, bb55eae6, c292ab30, 6ca45021, e9e53a29, d01693f9, 2e9f88c4, 3c22c9f7, 0b4978a1, f97d700e.

## What was built

- **desktopClient.ts**: `quality` event `{ slow }` from the signalling message; only `slow === true` is slow, anything else is false.
- **DesktopView.tsx**: `SLOW_CHIP` (`SignalIcon`, `Slow connection. The picture may freeze.`) sits in the chip chain below Reconnecting, Locked, a window problem, Control needs Accessibility and Someone is using this Mac, and above the other guest chips and Streaming N viewers. It shows in the existing `role="status"` live region, raises no notice, and is cleared by a lost connection. New optional `onSound(on)` prop: `playing = soundOn && status streaming && not host-paused`, reported on change and as `false` at unmount. The phone layer receives the Sound props when audio was negotiated, and `slotHidden` while a document-level focusin shows the `.desktop-unlock__input` has focus.
- **DesktopPhoneBars.tsx**: optional `sound` prop renders `DesktopSoundButton` (phone form, icon only) as the bottom bar's last item; the key bar never has it; a phone with no other items shows Sound alone. Show Keyboard is `aria-disabled` and inert only for `Locked` (live while Reconnecting). New `hidden` prop renders neither bar, so the 44 px slot is empty.
- **DesktopPhoneLayer.tsx**: passes `sound` and `hidden` through; the visual-viewport `fieldSelector` is now the exported `PHONE_FIELDS = '.desktop-phone__input, .desktop-unlock__input'`, so the Unlock panel rides above the iOS keyboard.
- **DesktopTab.tsx / App.tsx / TabBar.tsx**: `DesktopTab` `onPlaying` (via DesktopView `onSound`), App's `playingTabs` Set, TabBar `playingTabIds`. A playing Desktop tab shows `SpeakerWaveIcon` (`.tab__sound`, aria-hidden) between the name and the peer dot, and its accessible name is `<label>, playing sound`.
- **style.css**: `.desktop-sound--phone` is `flex: 0 0 44px` with a 20 px icon (matching the key bar icons); `.tab__sound` is 16 px with 4 px margins on the label's 4 px gap (8 px each side).

## Verification

- `tsc --noEmit` clean after every task. Full `vitest run` before this commit: 154 files, 2805 tests passed. `bash tests/check-brand.sh` clean before each commit.
- Acceptance greps: `Slow connection. The picture may freeze.` in DesktopView 1; `SignalIcon` in DesktopView at least 1; `DesktopSoundButton` in DesktopPhoneBars at least 1; `DesktopPhoneBars.sound.test.tsx` has 14 cases (gate 5); `, playing sound` in TabBar 1; `onSound` in DesktopTab 2.
- Quality tests read the golden `quality-slow-true` and `quality-slow-false` lines from `internal/desktop/testdata/signal-v1.jsonl` (at the client seam and for the view).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] The tab indicator cannot ride the label payload**
- **Found during:** Task 3
- **Issue:** the plan says DesktopTab carries `playing` in its `onLabel` payload. `DesktopTab` reports a label only while its tab is active (null otherwise), and App shows only the active tab's label, so an inactive Desktop tab that is playing would never show its speaker, which is the case the indicator exists for.
- **Fix:** a separate `onPlaying` callback on `DesktopTab`, a `playingTabs` Set in App, and a `playingTabIds` prop on TabBar. The label type and `handleDesktopLabel` are untouched.
- **Files modified:** DesktopTab.tsx, App.tsx, TabBar.tsx
- **Commit:** d29801b1

**2. [Rule 3 - Blocking] Two earlier tests pinned behavior this plan changes**
- `DesktopPhoneBars.test.tsx` (21-10) expected Show Keyboard live for Locked; now it is greyed for Locked and live for Reconnecting. `DesktopView.sound.test.tsx` (23-06) expected no Sound item at all on a phone; it now expects none in the page bar and one in the phone bottom bar. Both outside the plan's file list. Commits 8dfc6be4 and d29801b1.

### Plan structure

- The DesktopView `onSound` prop and its effects landed in the Task 2 commit (8dfc6be4) because DesktopView.tsx was already staged for the phone changes and interactive hunk staging is unavailable. Task 3's commit holds the DesktopTab, App, TabBar side and the tests.
- `desktopClient.test.ts` got one new test (golden quality lines) in Task 1's commit; the plan lists it only in the verify command.

No architectural changes.

## Tests for 23-12 (TESTING.md not edited)

New: `frontend/src/components/__tests__/DesktopView.quality.test.tsx` (8 cases), `frontend/src/components/__tests__/DesktopPhoneBars.sound.test.tsx` (14 cases). Traceability rows (DESK-07, DCTL-07, UNLK-01): both new files.
Edited: `frontend/src/components/__tests__/TabBar.test.tsx` (4 cases), `frontend/src/components/__tests__/DesktopTab.test.tsx` (5 cases, the playing feed incl. the hidden pause), `frontend/src/components/__tests__/DesktopView.sound.test.tsx` (5 `onSound` cases, one assertion changed), `frontend/src/components/__tests__/DesktopPhoneBars.test.tsx` (Locked case), `frontend/src/lib/__tests__/desktopClient.test.ts` (golden quality lines).

## Deferred (live checks, not run: no live browser, VM or device on this host)

Rows for 23-10, 23-12 and 23-14:
1. Real iPhone Safari and Home Screen: the Sound item fires from a tap (click, not pointerdown) and unmutes with audible output; the 44 px item and the 20 px icon next to the other bottom bar items at 375 pt and 320 pt widths, light and dark.
2. Real iPhone: focusing Unlock empties the slot and the panel sits directly above the keyboard in portrait and landscape; blurring brings the bottom bar back with no stuck viewport (the heal path for `.desktop-unlock__input`); Show Keyboard greys when the Mac locks and returns when it unlocks.
3. Real shaped link (23-10 / a two-Mac row): the Slow connection chip appears about 3 s after the viewer drops below the floor, clears about 5 s above it, never flaps, and appears for that viewer only. Chip wrapping at 288 px (long text) in the app tab and the browser.
4. Portty.app: the speaker on a background Desktop tab while sound is on; it clears on Mute, when the window is hidden about 2 minutes and the tab disconnects, and on Close; the tab's VoiceOver name reads `..., playing sound`; the 8 px gaps next to a long tab name (ellipsis still trims the name first).

## Known Stubs

None.

## Threat Flags

None. T-23-22 holds: the viewer only reads a boolean the daemon sends to that viewer, the chip names no one, and no package was added (T-23-SC).

## Self-Check: PASSED

- FOUND: DesktopView.quality.test.tsx, DesktopPhoneBars.sound.test.tsx
- FOUND commits (ancestors of HEAD): f28a967a, 8dfc6be4, d29801b1
