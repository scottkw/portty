---
phase: 21-portty-app-and-iphone-viewers
plan: 03
subsystem: frontend-iphone-viewer
tags: [touch, gestures, trackpad, zoom, typescript, vitest]
requires: []
provides:
  - "createTrackpad / attachTrackpad: pure touch-to-intent recognizer (move, tap, doubleTap, longPress, dragStart/Move/End, scroll, pinch, pinchEnd, resetZoom)"
  - "desktopCoords: normalizedDelta, zoomAround, panToKeepPointer, videoTransform, scrollDelta, pointerToStage, SCROLL_SIGN"
affects: [21-10]
tech-stack:
  added: []
  patterns:
    - "injectable clock (now/setTimeout/clearTimeout) so every timer is testable with a fake clock"
    - "mock-container handler capture, plain event objects (pinchZoom.test.ts pattern)"
key-files:
  created:
    - frontend/src/lib/touchTrackpad.ts
    - frontend/src/lib/desktopCoords.ts
    - frontend/src/lib/__tests__/touchTrackpad.test.ts
    - frontend/src/lib/__tests__/desktopCoords.test.ts
  modified: []
key-decisions:
  - "Single tap is emitted only after the 300 ms double-tap window (documented in the module header) so tap-then-drag never leaks a click"
  - "SCROLL_SIGN = -1 is the only place the scroll sign appears (natural scrolling); device row A10 may flip it"
  - "desktopCoords imports only the VideoRect type from DesktopView (import type), so the letterbox math stays the single videoContentRect implementation without a runtime React dependency"
requirements-completed: [DVWR-03]
status: complete
duration: ~40min
completed: 2026-11-26
commits: 5
plan_head_before: 6662f19644b8914f539b0952f62f526d6d7e3711
plan_head_after: d2162e4a3896e9cbe6ca86a8dc55ccce248c6e79
actuals:
  tokens: 14000
  tasks: 3
  commits: 5
---

# Phase 21 Plan 03: Trackpad gesture engine and zoom math Summary

Pure, fake-clock-tested iPhone trackpad recognizer plus the finger-pixel to normalized-delta, zoom, pan-follow and scroll math that 21-10 will wire to Phase 20's input messages.

## What was built

- `touchTrackpad.ts`: `TRACKPAD` thresholds, `accelGain` (1.0 below 200 px/s, linear to 2.5 at 1500 px/s), `createTrackpad` state machine, `attachTrackpad` (non-passive touchstart/move/end/cancel plus a gesturestart guard on the container only; disposer removes all listeners and clears timers).
- One finger: move (8 px threshold, accelerated), tap (up within 250 ms, at most 8 px, emitted after the 300 ms window), doubleTap (second touch within 300 ms, 24 px; carries both tap times), longPress (500 ms timer, never `contextmenu`), tap-then-drag (dragStart, dragMove, dragEnd; the first tap is consumed, no click).
- Two fingers: pinch-or-scroll chosen once per gesture (distance change versus centroid travel, 12 px), scroll deltas, pinch (scale relative to gesture start, plus centre) with pinchEnd, two two-finger taps within 300 ms emit resetZoom, a third finger cancels with no intent.
- `desktopCoords.ts`: `normalizedDelta` (gain * px / (zoom * picture width at 1x)), `clampZoom` (1x to 5x), `zoomAround` (pinch centre fixed, picture always covers the stage), `panToKeepPointer` (32 px margin, no-op at 1x), `videoTransform` (empty string at 1x), `scrollDelta` and `SCROLL_SIGN`, `pointerToStage`.

## Verification

- `tsc --noEmit` clean; touchTrackpad and desktopCoords tests: 40 passed; full frontend suite: 128 files, 2001 tests passed; `bash tests/check-brand.sh`: clean. All acceptance greps hold.
- TDD: Task 2 RED had 8 of 16 tests failing on assertions (no tap/doubleTap/longPress/dragStart yet). Task 3 RED had 20 failing tests: 10 two-finger assertion failures and 10 desktopCoords tests failing with "x is not a function" because the exports did not exist yet (valid RED, but a TypeError rather than an assertion for those). Plan type is `execute` with `tdd="true"` tasks, so the `check tdd-red-evidence` classifier was not run.
- The tracer gate (Task 1) was re-verified end to end through the real listener path before expansion.

## Deviations from Plan

### Auto-fixed / judgment calls

**1. [Rule 3 - Blocking] pnpm cannot run in the worktree.** `pnpm --dir frontend exec ...` tries to reinstall and fails because `frontend/node_modules` is a symlink to the main checkout. Ran the same tools directly (`frontend/node_modules/.bin/tsc` and `vitest`); same commands, same results. The symlink was never committed and was removed at the end.

**2. [Rule 2 - Safety] cancel() and a lifted-or-added finger release a held drag and close an open pinch.** The plan says cancel() in a pending state emits nothing (kept). For a drag in progress it still emits dragEnd (otherwise the Mac's button stays down on an unattended machine), and for a decided pinch it emits pinchEnd so the caller can drop will-change. A third finger during a pinch also emits pinchEnd (not "no intent" strictly; pinchEnd is a local-only close). A second finger landing during a drag emits dragEnd first.

**3. [Judgment] `import type { VideoRect }` instead of importing the `videoContentRect` value.** The plan's key_link mentions importing `videoContentRect`, but the module is also constrained to "no React". A value import would pull DesktopView's React and icon imports into this pure module. A type-only import satisfies the `from '../components/DesktopView'` acceptance grep and keeps one implementation; nothing is copied. `pointerToStage` takes the rect the caller computed with `videoContentRect`.

**4. [Judgment] Ambiguities resolved:** the tap's `at` is the lift time; a second touch farther than 24 px from the first tap still becomes a drag if it moves (position only matters for double-tap); a second touch farther away that is a tap yields two separate taps; a held second touch (500 ms) flushes the first tap then fires longPress; when distance change and centroid travel cross 12 px in the same event the larger wins; the 300 ms double-tap window is measured from the first lift to the second touch (and the boundary is inclusive), both for one finger and for the two-finger reset.

## Known Stubs

None.

## Threat Flags

None. No network, auth or file surface added (pure modules). T-21-08 mitigated (pinch-or-scroll chosen once; pinch emits only local intents; thresholds and boundaries pinned by tests). T-21-09 mitigated at this layer (non-passive container listeners plus gesturestart guard); real-iPhone rows in 21-13 decide the rest.

## For 21-13 (TESTING.md, not edited here)

New test files for Section 2 / Section 4 (DVWR-03):
- `frontend/src/lib/__tests__/touchTrackpad.test.ts`
- `frontend/src/lib/__tests__/desktopCoords.test.ts`

Manual device rows still needed (not automatable): scroll sign (A10), long press shows no iOS callout (touch handlers call preventDefault), Safari page zoom stays off in the stage, edge swipes, the 300 ms single-tap feel.

## Commits

- 66a06bbe feat(21-03): one-finger drag becomes accelerated, zoom-scaled move intents
- 47b2844f test(21-03): add failing tests for tap, double-tap, long-press and tap-then-drag
- 24542b3c feat(21-03): recognize tap, double-tap, long-press and tap-then-drag
- 567d083c test(21-03): add failing tests for two-finger scroll vs pinch, zoom, pan-follow and reset
- d2162e4a feat(21-03): two-finger scroll vs pinch, zoom, pan-follow and reset

## Self-Check: PASSED

Files exist (touchTrackpad.ts, desktopCoords.ts, both test files); all five commits are ancestors of HEAD.
