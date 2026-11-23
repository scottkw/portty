---
phase: 14-mobile-web-app
fixed_at: 2026-11-22T21:20:00Z
review_path: .planning/phases/14-mobile-web-app/14-REVIEW.md
iteration: 1
findings_in_scope: 4
fixed: 4
skipped: 0
status: all_fixed
---

# Phase 14: Code Review Fix Report

**Fixed at:** 2026-11-22
**Source review:** .planning/phases/14-mobile-web-app/14-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 4 (WR-01, WR-02, WR-03, plus IN-02 as requested)
- Fixed: 4
- Skipped: 0

Edits and commits were made in the main checkout (no worktree), on `gsd/v1.2`. Verification ran in the main checkout, so it is reproducible from this tree.

## Fixed Issues

### WR-01: Stale `maxInner` makes the viewport heal fire in landscape (and IN-02)

**Files modified:** `frontend/src/lib/visualViewportInset.ts`, `frontend/src/lib/__tests__/visualViewportInset.test.ts`
**Commit:** f35a0498
**Applied fix:** Removed `maxInner`; `heal` now compares against the live `window.innerHeight`, the same baseline `keyboardUp` uses. New rotation test (850 to 390, blur) asserts `.app` display is never written. Both new tests failed before the fix.
IN-02: the heal's restore frame id is tracked and cancelled in cleanup. Cleanup also resets `display` in that case, so cancelling the restore cannot leave `.app` hidden. A test pins both.
Status: fixed: requires human verification (logic change; the iOS standalone heal is still [ASSUMED] until a real phone shows it).

### WR-02: An armed Ctrl is consumed by terminal replies

**Files modified:** `frontend/src/components/WebShareSessionView.tsx`, `frontend/src/components/__tests__/WebShareSessionView.keybar.test.tsx`
**Commit:** a573b752
**Applied fix:** `inputTransform` returns ESC-prefixed multi-byte payloads unchanged and leaves Ctrl armed. Test arms Ctrl, feeds a DA1 reply, a DSR reply and a focus report (each passes through, Ctrl still pressed), then `c` gives 0x03. The test failed before the fix.
Known limit (commented in code): a typed arrow or Alt key arriving while Ctrl is armed also no longer releases it.
Status: fixed: requires human verification (logic change).

### WR-03: Page zoom disabled across the whole phone page

**Files modified:** `frontend/src/style.css`, `frontend/src/lib/pinchZoom.ts`, `frontend/src/lib/__tests__/pinchZoom.test.ts`, `frontend/src/components/__tests__/style.mobile.test.ts`, `frontend/e2e/mobile-iphone.spec.ts`
**Commits:** b39b9d8c, 8f111eea
**Applied fix:** Deleted the phone `html { touch-action: pan-x pan-y }` rule. It was also redundant for the terminal: the base `.terminal-session-container` rule already declares `touch-action: pan-y`, which excludes pinch (confirmed by a computed-style probe under the iphone project), so no new terminal rule was added. `gesturestart` prevention moved from `document` to the container, with its removal in cleanup. Pinch-on-terminal still sets the font: the pinchZoom and TerminalPanel.pinch tests pass unchanged, and the touchmove path is untouched.
Tests: pinchZoom unit tests assert gesturestart is prevented on the container and NOT on the document, and that cleanup removes the container listener. `style.mobile.test.ts` previously pinned the html rule and now pins its absence plus the container `pan-y`. A new iphone Playwright test asserts computed `touch-action`: container `pan-y`, `html` and `.app` `auto`.
Status: fixed.

## Skipped Issues

None.

## Out of Scope (left alone, as instructed)

IN-01, IN-03, IN-04, IN-05, IN-06 and IN-07 (the necessity finding) were not touched. The PDF layer got no explicit `pinch-zoom` touch-action: with the html restriction gone it defaults to `auto`, so none is needed.

## Verification

- Touched vitest files: visualViewportInset, WebShareSessionView.keybar, pinchZoom, TerminalPanel.pinch, style.mobile: all pass.
- Full vitest (`NO_COLOR=1`): 122 files, 1813 tests, all pass. An earlier full run found `style.mobile.test.ts` pinning the removed html rule; fixed in 8f111eea.
- `npx tsc --noEmit`: exit 0.
- Playwright `--project=iphone e2e/mobile-iphone.spec.ts` with nvm node v24.22.1 first on PATH: 12 of 12 pass. The new touch-action test first failed, on the container expectation, which showed the base `pan-y`.
- No new test file, so TESTING.md and `check-traceability-paths.sh` did not need changes. Nothing touched the host tailscaled, a 100.x address or host GUI.

## UI review fixes

Source: `14-UI-REVIEW.md` (items 1, 3 and 4). Each fix has its own commit, test first.

| UI-REVIEW item | Commit | Change |
|----------------|--------|--------|
| 1. Key pressed state | dee38c70 | `KeyBar.tsx` sets `keybar__key--down` on pointerdown and clears it on pointerup, pointercancel and pointerleave. `style.css` gives it the `:active` fill (`--color-selection-inactive`); `:active` stays. preventDefault, focus and the click `detail===0` path are unchanged. |
| 3. Light-mode launch | 85c683d7 | Dropped `theme_color` from `manifest.json`; the two media-query `theme-color` tags decide the status bar. `background_color` kept dark, so a light-mode splash can still flash dark (noted in 14-UI-SPEC.md). `homeScreen.test.ts` and the `web-csp` manifest check now assert it is absent. |
| 4. `\|` glyph | 924ad01e | The glyph sits in a `span aria-hidden="true"`; `aria-label="Vertical bar"` stays the accessible name. |

Not done (need a real phone, not code): items 2 (M-147 pinch and iOS version), 5 (close the spec sign-offs) and 6 (key repeat). The pressed fill on a real iPhone is still unrecorded; capture one screenshot in the next phone check.

Verification (main checkout, no worktree): touched vitest files pass (KeyBar, style.keybar, WebShareSessionView.keybar, style.mobile, homeScreen); full vitest `NO_COLOR=1` 122 files, 1816 tests pass (one run hit a flaky xterm timing test, `pathLinks.linkifier`, which passes alone and on rerun); `npx tsc --noEmit` exit 0; Playwright `--project=iphone`: 12 of 12 pass. The chromium-only manifest assertion in `web-csp.spec.ts` was not run. Nothing touched the host tailscaled, a 100.x address or host GUI.

---

_Fixed: 2026-11-22_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
