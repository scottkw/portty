---
phase: 21-portty-app-and-iphone-viewers
plan: 10
subsystem: frontend-iphone-viewer
tags: [iphone, touch, trackpad, soft-keyboard, zoom, desktop-viewer, typescript, vitest]
requires:
  - phase: 21-03
    provides: attachTrackpad intents, desktopCoords math
  - phase: 21-08
    provides: DesktopView embedding props (kept page-preserving)
  - phase: 20-06
    provides: desktopInput encoders and constants, DesktopClient sendInput / currentEpoch / rights, input, surface and cursor events
  - phase: 20-07
    provides: DesktopSendKeysMenu, ContextMenu checked items and labels
provides:
  - "createPhoneInput: rmove, click, doubleClick, press/release, wheel, keyTap, text, chord, heartbeat while a button is held, release on blur/pagehide/hidden"
  - "DesktopPhoneLayer: stage touch layer, local CSS zoom with pan-follow, ripple, hidden soft-keyboard field, armed modifiers"
  - "DesktopPhoneBars: one slot swapping the bottom bar (Desktop controls) and the key bar (Desktop keys)"
  - "useVisualViewportInset(enabled, { rootSelector, fieldSelector })"
  - "DesktopView phone branch (.desktop-view--phone) and the phone CSS block"
affects: [21-12, 21-13]
tech-stack:
  added: []
  patterns:
    - "injected-deps input sender with the Phase 20 frozen encoders (no new wire shapes)"
    - "latest-handler ref so native field listeners never go stale"
    - "slot portaled into the view root so the stage and the bars are siblings"
key-files:
  created:
    - frontend/src/lib/desktopPhoneInput.ts
    - frontend/src/components/DesktopPhoneLayer.tsx
    - frontend/src/components/DesktopPhoneBars.tsx
    - frontend/src/lib/__tests__/desktopPhoneInput.test.ts
    - frontend/src/components/__tests__/DesktopPhoneLayer.test.tsx
    - frontend/src/components/__tests__/DesktopPhoneBars.test.tsx
  modified:
    - frontend/src/components/DesktopView.tsx
    - frontend/src/components/__tests__/DesktopView.test.tsx
    - frontend/src/components/ContextMenu.tsx
    - frontend/src/components/DesktopSendKeysMenu.tsx
    - frontend/src/lib/visualViewportInset.ts
    - frontend/src/lib/__tests__/visualViewportInset.test.ts
    - frontend/src/style.css
key-decisions:
  - "Armed modifiers go out as a Phase 20 chord (modifier downs, key, ups in reverse), the same shape Send Keys already proves, so the daemon sees real modifier key events and not only a mask"
  - "Held state is only mouse buttons; keys are always complete taps or chords, so the heartbeat runs only while a button is down"
  - "A tap with no cursor report for the current epoch asks for one (zero rmove) and waits up to 500 ms, then is dropped; a drag start with no report is dropped as a whole (its moves send nothing)"
  - "Pinch applies incrementally to the current zoom (scale relative to the zoom when the pinch began), centre fixed"
requirements-completed: [DVWR-03]
status: complete
duration: ~75min
completed: 2026-11-27
commits: 4
plan_head_before: cb12fb488fa559a542c2da5647acc64912789f2c
plan_head_after: bb55eae66ba2d4f45d7a42608b2816d5d509a0aa
actuals:
  tokens: 36000
  tasks: 3
  commits: 4
---

# Phase 21 Plan 10: iPhone viewer (touch layer, bars, soft keyboard) Summary

On a phone the same /desktop/ page gains trackpad mode, tap, double-tap, long-press right-click, tap-and-drag, two-finger scroll, local 1x to 5x pinch zoom, a soft keyboard with a desktop key bar and Phase 20's Send Keys and surface picker, all speaking Phase 20's frozen input protocol; every other device renders exactly Phase 20's page.

## What was built

- `desktopPhoneInput.ts`: `createPhoneInput` over Phase 20's encoders. Nothing goes out while the epoch is null. `click` and `doubleClick` carry the real tap times (the Mac's double-click count comes from viewer timestamps). `press` starts a 1 s heartbeat that stops on `release`. Window blur, pagehide and a hidden page send `release` and clear held state. `text` goes through `splitText` at 1 KiB. `release` with no epoch falls back to the daemon release.
- `DesktopPhoneLayer.tsx`: `attachTrackpad` on the stage, intents mapped through `normalizedDelta` / `scrollDelta` / `zoomAround` / `panToKeepPointer`. The pointer lives on the Mac: drags are `rmove`, clicks use the latest cursor report for the current epoch, one zero `rmove` asks for it when the input channel opens and after a surface change. Pinch changes only the video's CSS transform (empty at 1x, `will-change` only during the gesture), clamps 1x to 5x, follows the cursor report to keep the pointer 32 px inside, and resets on a portrait/landscape flip or two two-finger taps. 24 px ripple on long press and drag start (150 ms). Hidden 16 px field (`Type on the Mac`) with a zero-width sentinel for Backspace, native `beforeinput` / `keydown` / `input` / composition listeners, Return as Enter, one-shot Ctrl/Opt/Cmd that carry the first key or character and disarm, keyboard down disarms and releases.
- `DesktopPhoneBars.tsx`: one `.keybar` slot. Keyboard down: `Desktop controls` (surface name with an upward picker, Send Keys, Show Keyboard, Fit to Screen only above 1x with a zoom aria-label). Keyboard up: `Desktop keys` (Esc, Tab, Ctrl, Opt, Cmd, four arrows, Send Keys). Phase 14 firing rule everywhere except Show Keyboard (its click). Items that send are `aria-disabled` while Locked or Reconnecting; Show Keyboard and Fit stay live; items the hello never grants are not rendered.
- `visualViewportInset.ts`: optional root and field selectors, terminal defaults unchanged (all existing hook tests untouched and passing).
- `DesktopView.tsx`: `usePhone()` branch renders the layer inside the stage, adds `desktop-view--phone`, and keeps rights and client in state for the layer. Phase 20's controller is still never created on a phone.
- `style.css`: `.desktop-view--phone` (flex column, follows `--vv-height` / `--vv-top`), stage `touch-action: none` and clipping, slot, bar, keys (`min-width: 36px`), hidden field, ripple. Tokens only.

## Tests added

- `frontend/src/lib/__tests__/desktopPhoneInput.test.ts` (16 cases)
- `frontend/src/components/__tests__/DesktopPhoneLayer.test.tsx` (41 cases)
- `frontend/src/components/__tests__/DesktopPhoneBars.test.tsx` (18 cases)
- additions to `visualViewportInset.test.ts` (2) and a wording update in `DesktopView.test.tsx`

## Verification

`tsc --noEmit` clean. Full frontend suite: 138 files, 2485 tests passed (includes the token source gate and the Phase 14 key bar tests). `bash tests/check-brand.sh` clean. All acceptance greps hold (`<DesktopPhoneLayer` 1, `createPhoneInput` 1, userAgent/wakeLock 0, `pagehide` listener 1, `HEARTBEAT_MS` used, `videoTransform` used, `rootSelector`, `desktop-phone` x6, `min-width: 36px`, `Type on the Mac` 1, `Desktop keys` 1).

## TDD Gate Compliance

Plan type is `execute` with `tdd="true"` tasks; the `check tdd-red-evidence` classifier was not run, RED was read from vitest output by hand.

- Task 1 was the tracer (test written with the code, passes end to end; re-run before expansion).
- Task 2: RED commit `9271e7f7` (15 of 16 lib tests failing with "not a function" TypeErrors because the sender methods did not exist yet, 17 of 24 layer tests failing on assertions about missing behaviour); GREEN `400eeee8`.
- Task 3: **not RED-first.** I wrote DesktopPhoneBars and the layer's keyboard code before their tests, then wrote the tests and mutation-checked the firing rule (removing `detail === 0` and the pointerdown `preventDefault` fails the firing-rule test; restored). The visualViewportInset option tests were added after the hook change. All behaviors in the plan's `<behavior>` list have a test.

## Deviations from Plan

**1. [Rule 3 - Blocking] `ContextMenu` and `DesktopSendKeysMenu` gained an optional `above` prop.** The menus had to open upward from a bar at the bottom of the screen. ContextMenu only flips when it would overflow `window.innerHeight`, which is the layout viewport, so with the keyboard up a menu would open downward behind it. `above` is additive and defaults off. Files: `ContextMenu.tsx`, `DesktopSendKeysMenu.tsx` (not in the plan's `files_modified`). Test: menus anchored at the bar's top land at top - menu height. Commit bb55eae6.

**2. [Rule 1/2 - Superseded Phase 20 assertions] Three Phase 20 tests in `DesktopView.test.tsx`** asserted the phone has no `[role="toolbar"]` (Phase 20 Surface F). UI-SPEC Surface 4 supersedes that for the phone, so they now assert the Phase 20 floating `.desktop-bar` is absent. Same commit.

**3. [Judgment] `createPhoneInput` deps trimmed.** The plan's interface lists `setTimeout` / `clearTimeout`; the sender needs only `setInterval` / `clearInterval` (no other timer), so the others are not declared. The cursor-wait and ripple timers live in the layer.

**4. [Judgment] `DesktopPhoneBars` takes an extra optional `surface: {kind, id}`** besides the plan's `surfaceName`, so the picker can check the current surface and the Send Keys menu can grey itself in a window share without matching on a name.

**5. [Judgment] Armed modifiers are sent as a chord** (modifier key downs, the key, ups in reverse) rather than only a mask on the key message. The daemon contract for a mask without modifier key events is not in the golden file, while Send Keys already sends chords this way. Shifted punctuation and non-ASCII under an armed modifier fall back to plain text (marked `shortcut:` in code).

**6. [Rule 3 - Blocking] pnpm cannot run in the worktree** (no `node_modules`; `pnpm --dir frontend exec ...` would reinstall). Used a temporary `frontend/node_modules` symlink to the main checkout and ran `tsc` / `vitest` directly, as 21-03 did. The symlink was never staged and is removed at the end.

**7. [Process] Commit trailer.** The plan constraint names `Claude Opus 5.5`; the session's attribution instruction names `Claude Sonnet 5.5`, which is the model that actually wrote these commits, so all four commits carry the Sonnet trailer.

## Assumptions on Phase 20 as built

All matched without adaptation: encoders and constants exported from `desktopInput.ts`, `client.sendInput(msg, droppable)`, `currentEpoch()`, the `rights`, `input`, `surface` and `cursor` events, `requestSurfaces` / `selectSurface`, and DesktopView creating no input controller on a phone. The cursor report arriving after a zero `rmove` is a 20-02 must-have and is covered here only against the mocked client; the VM row (21-13) must confirm it on a real daemon.

## Known Stubs

None.

## Threat Flags

None. No network, auth or file surface added. T-21-35 mitigated (release on blur, hidden, pagehide, channel close, keyboard down, lock, surface change; heartbeat only while a button is held; each trigger tested). T-21-36 mitigated (pinch and Fit never call the sender; stage `touch-action: none`, non-passive stage listeners and the gesturestart guard from 21-03; no listener on bars). T-21-37 mitigated (surface names are React text). T-21-38 mitigated (nothing in the phone path logs text or keys). T-21-SC: no package added.

## For 21-13 (TESTING.md, not edited here)

New test files for Section 2 / Section 4 (DVWR-03):
- `frontend/src/lib/__tests__/desktopPhoneInput.test.ts`
- `frontend/src/components/__tests__/DesktopPhoneLayer.test.tsx`
- `frontend/src/components/__tests__/DesktopPhoneBars.test.tsx`

Manual real-iPhone rows (not automatable): keyboard opens from Show Keyboard and the slot sits on it (safe-area, landscape), Backspace through the sentinel, Return, predictive text, key bar fits at 360 to 430 pt with no scrolling, armed-modifier look, pinch to 5x and pan-follow, two-finger scroll sign (A10), long press shows no callout, Tap to start and resume after sleep, Send Keys and picker upward menus, `release` reaching the Mac when the phone sleeps with a button held, the Mac's cursor report arriving after a zero `rmove`.

## Commits

- e9e53a29 feat(21-10): one-finger drag on a phone sends Phase 20 relative moves
- 9271e7f7 test(21-10): add failing tests for phone clicks, drag, scroll, zoom, cursor request and release
- 400eeee8 feat(21-10): phone clicks, drag, right-click, scroll, local zoom, cursor request and release
- bb55eae6 feat(21-10): phone bottom bar, key bar, soft keyboard, armed modifiers and viewport inset

## Self-Check: PASSED

All six created files exist; commits e9e53a29, 9271e7f7, 400eeee8 and bb55eae6 are ancestors of HEAD.
