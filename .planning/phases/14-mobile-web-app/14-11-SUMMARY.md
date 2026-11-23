---
phase: 14-mobile-web-app
plan: 11
subsystem: frontend
tags: [react, xterm, pinch-zoom, visualviewport, safe-area, playwright, webkit, vitest]
requires:
  - phase: 14-08
    provides: phoneQuery (PHONE_QUERY, usePhone)
  - phase: 14-10
    provides: KeyBar, the `iphone` Playwright project, the 52px-class phone bar
provides:
  - attachPinchZoom (two-touch pinch to a 6..32 px font, no page zoom)
  - useVisualViewportInset (root sized to visualViewport, keyboardUp flag, stuck-viewport heal)
  - one-claim pinch gate in TerminalPanel
  - in-memory browser-guest font in WebShareSessionView
  - phone chrome CSS (single-tab strip hidden, 4px 8px padding, safe areas, 16px controls, touch-action)
affects: [14-12, 14-13, 14-14]
status: complete
actuals:
  tokens: 11100
  tasks: 3
  commits: 3
plan_head_before: 36293958d09fe04cbdcaa68caef81a7a025a126e
plan_head_after: 48f3b06aa8e936977c5b1217484a7d6fce7d1f1e
tech-stack:
  added: []
  patterns:
    - "a gesture module shaped like attachTouchScroll: header doc block, constants, returned cleanup"
    - "the web root takes --vv-height/--vv-top CSS variables with fallbacks, so desktop rules are untouched when they are unset"
key-files:
  created:
    - frontend/src/lib/pinchZoom.ts
    - frontend/src/lib/__tests__/pinchZoom.test.ts
    - frontend/src/lib/visualViewportInset.ts
    - frontend/src/lib/__tests__/visualViewportInset.test.ts
    - frontend/src/components/__tests__/TerminalPanel.pinch.test.tsx
    - frontend/src/components/__tests__/WebShareSessionView.font.test.tsx
    - frontend/src/components/__tests__/style.mobile.test.ts
  modified:
    - frontend/src/components/TerminalPanel.tsx
    - frontend/src/components/WebShareSessionView.tsx
    - frontend/src/components/KeyBar.tsx
    - frontend/src/components/TabBar.tsx
    - frontend/src/App.tsx
    - frontend/src/style.css
    - frontend/src/components/__tests__/KeyBar.test.tsx
    - frontend/src/components/__tests__/TabBar.test.tsx
    - frontend/src/components/__tests__/WebShareSessionView.keybar.test.tsx
    - frontend/src/components/__tests__/TerminalPanel.touchscroll.test.tsx
    - frontend/e2e/mobile-iphone.spec.ts
key-decisions:
  - "The pinch's setSize converts to a delta against the size the pinch last asked for (pinchSizeRef), not the rendered fontSize, which lags between renders"
  - "The browser guest's font fallback uses a functional state update plus a ref for the base size, because TerminalPanel's Shift+= handler holds the first closure and a pinch fires faster than React renders"
  - "The stuck-viewport heal hides the root, forces a reflow, and restores display in the next frame (not a same-tick toggle)"
requirements-completed: []  # MOB-01 also needs the iOS Simulator run (14-13) and the real-phone check (14-14); requirements.mark-complete not run, same as 14-10
---

# Phase 14 Plan 11: Pinch font, keyboard inset and phone chrome Summary

A phone in the browser share view now pinches to change the terminal font (whole pixels, 6..32, one grid claim 250 ms after the last finger lifts), keeps the key bar and cursor line above the on-screen keyboard by sizing the app root to visualViewport, and drops the lone-tab strip and tightens padding and safe areas, with the desktop view untouched.

Logs: `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.iGdm5zju8M` (Task 1 vitest and tsc), `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.7ptfTuoOnI` (Task 2 vitest and tsc), `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.xD1iToYItb` (Task 3 targeted vitest and the iphone-project run), `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.24hzvonKpr` (Task 3 full vitest, tsc, share-removal, full Playwright).

## Accomplishments

- **pinchZoom.** `attachPinchZoom(container, { getSize, setSize, onEnd })`. The first two-touch move records `d0` and `size0`; live size is `clamp(round(size0 * d / d0), 6, 32)`; `setSize` only on a change; every two-touch `touchmove` is `preventDefault`'ed on a `{ passive: false }` listener; `gesturestart` is prevented on document; `onEnd` fires once 250 ms after the last finger lifts (a new pinch inside the window restarts it). One-finger events are untouched. The cleanup removes every listener and any pending end.
- **One claim.** TerminalPanel attaches it beside `attachTouchScroll`. `setSize` sets `pinchingRef` and calls `onFontSizeChange(px - last)`; the font effect lays out and reports but skips `claimFit()` while `pinchingRef` is set; `onEnd` clears it and claims once.
- **Web font in memory.** WebShareSessionView keeps `pinchFontSize` state: effective size is `fontSize ?? pinchFontSize ?? hostFont?.size ?? DEFAULT`, and its guest `onFontSizeChange` clamps `effective + delta` to 6..32. A remount returns to the host's font. Nothing is stored.
- **Keyboard inset.** `useVisualViewportInset(enabled)` writes `--vv-height` and `--vv-top` on `.app` inside requestAnimationFrame on visualViewport `resize` and `scroll`, and returns `keyboardUp` (`innerHeight - vv.height > 100`). App enables it for `mode === 'web' && usePhone()` and passes `keyboardUp` through WebShareSessionView to KeyBar's `keybar--kb-up` class. The heal runs about 140 ms after the helper textarea blurs with none focused, when the viewport is still more than 100 px under the largest innerHeight seen.
- **Phone chrome.** TabBar adds `tab-bar--single` (web mode, one tab). Under `(pointer: coarse) and (hover: none)`: strip hidden, strip sides `max(12px, env(safe-area-inset-*))`, terminal padding `4px 8px`, root top `env(safe-area-inset-top)`, `html { touch-action: pan-x pan-y }`, `.keybar--kb-up { padding-bottom: 0 }`. Under `(pointer: coarse)`: `input, textarea, select, .xterm-helper-textarea { font-size: 16px }`.

## Claim counts (TerminalPanel.pinch.test.tsx)

| Scenario | Claims |
|----------|--------|
| five size changes (14 to 18) while pinching | 0 (font effect still lays out and reports) |
| then `onEnd` | exactly 1, at 900 / (18 x 0.7) columns, the final fit |
| a font change after the pinch ended | 1 (the gate is cleared) |
| a pinch that changed nothing, then `onEnd` | 1, no font change |

Deltas sent for the five changes were `[1, 1, 1, 1, 1]`, so the parent lands exactly on 18 even though no render happens between them.

## Did the helper textarea need `!important`?

No. xterm's CSS sets no font-size on `.xterm-helper-textarea` and writes none inline; the plain rule gave `16px` in the iPhone 15 WebKit emulation.

## Selection fix (compensatePointerScale)

Untouched. It patches xterm's one `_mouseService` once per Terminal after `term.open`; the pinch only changes `term.options.fontSize` and goes through the existing `relayout()` and `recomputeScale()`, so the scale transform it compensates for is recomputed from the new cell metrics as before. TerminalPanel.scale and the terminalScale tests pass in the full suite.

## Test results

- Full vitest: 122 files, 1808 tests passed, 0 failed. tsc: clean. `tests/check-share-removal.sh`: exit 0.
- Full Playwright (all projects, once): 52 passed, 31 skipped, 0 failed. Of those, the `iphone` project alone: 11 passed (the 7 key bar specs plus the 4 new chrome specs: strip hidden with one tab, padding `4px 8px`, helper textarea `16px`, key bar below the terminal area).
- New unit tests: pinchZoom (8), TerminalPanel.pinch (4), WebShareSessionView.font (3), visualViewportInset (8), style.mobile (8), TabBar single-tab (3), plus a KeyBar and a keybar-view case for `keybar--kb-up`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Existing wiring test pinned a 600-character window**
- **Found during:** Task 1
- **Issue:** `TerminalPanel.touchscroll.test.tsx` Test 3 looks 600 characters after `attachTouchScroll(` for the `}, [sessionId])` closer; the touch effect now also attaches the pinch handler and is longer.
- **Fix:** widened the window to 1500 characters; the assertion (that effect keys off `[sessionId]`) is unchanged.
- **Files modified:** `frontend/src/components/__tests__/TerminalPanel.touchscroll.test.tsx`
- **Commit:** bc25e326

**2. [Rule 1 - Bug avoided] Pinch deltas against a stale rendered size**
- **Found during:** Task 1 design
- **Issue:** several `setSize` calls land between React renders; a delta against the rendered `fontSize` would add up wrong (13 to 14 to 15 sent as +1 then +2).
- **Fix:** `pinchSizeRef` holds the size the pinch last asked for and `getSize` returns it while pinching; the guest font update is a functional update.
- **Commit:** bc25e326

Test file note: `visualViewportInset.test.ts` uses `React.createElement` (the plan names a `.ts` file, which cannot hold JSX).

## Follow-up fix: desktop Shift+=/- stay no-ops

The guest `onFontSizeChange` fallback briefly made Shift+= and Shift+- change the font in the desktop browser share view, against SC1 ("the desktop browser view is unchanged"). Fixed after the plan's tasks: WebShareSessionView now uses the in-memory fallback only when `usePhone()` is true; on a desktop browser it is the no-op it was. Pinch on a phone is unaffected. `WebShareSessionView.font.test.tsx` pins both (desktop: the delta calls leave `fontSize` unchanged, so TerminalPanel's font effect never runs and there is no claim; phone: the deltas apply, clamp and reset on remount). Touched vitest files (WebShareSessionView, TerminalPanel.pinch: 57 tests) and tsc pass; log `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.LYul9SLFzd`. The full suites were not re-run.

## Not verified here (backstop, 14-13 and 14-14)

- A real pinch and the on-screen keyboard cannot be produced by WebKit emulation; both are unit-tested only. The iOS 26 Simulator run (14-13) and a real phone (14-14) cover: the key bar and cursor line above the keyboard in portrait and landscape, the keyboard inset and safe areas in a Home Screen web app, the stuck-viewport heal ([ASSUMED] per A3), and that `touch-action` plus the non-passive handler blocks page zoom on iOS 26.
- The 31 skipped Playwright tests are skips already in the suite (conditional per project); not investigated.
- TESTING.md was not updated: the plan assigns it to 14-12 and 14-14.

## Known Stubs

None.

## Threat Flags

None. No new endpoint, auth path or file access; T-14-26 (one claim) is covered by TerminalPanel.pinch.test.tsx and T-14-27 by the viewport hook tests and the iPhone e2e (iOS behaviour deferred to 14-13).

## Self-Check: PASSED

Files found: pinchZoom.ts, visualViewportInset.ts, TerminalPanel.pinch.test.tsx, style.mobile.test.ts, WebShareSessionView.font.test.tsx. Commits found on HEAD: bc25e326, ac2bb0b3, 48f3b06a.
