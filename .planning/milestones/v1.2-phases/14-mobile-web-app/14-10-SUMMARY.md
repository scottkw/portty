---
phase: 14-mobile-web-app
plan: 10
subsystem: frontend
tags: [react, xterm, key-bar, playwright, webkit, vitest]
requires:
  - phase: 14-01
    provides: fixture /__test__/input routes (the PTY input stub)
  - phase: 14-02
    provides: lone-Esc flush, so a bare ESC arrives without a following byte
  - phase: 14-08
    provides: phoneQuery (PHONE_QUERY, usePhone)
provides:
  - ctrlMap (ctrlByte, arrowSeq, ESC/TAB/PIPE)
  - KeyBar component (eight keys, one-shot Ctrl)
  - TerminalPanel keyInputRef and inputTransform seams
  - the `iphone` Playwright project (iPhone 15, WebKit) running mobile-*.spec.ts
affects: [14-11, 14-12, 14-13, 14-14]
status: complete
actuals:
  tokens: 8900
  tasks: 3
  commits: 3
plan_head_before: 626b2d72770a707f725161be8fd9d93327f7183b
plan_head_after: ab5e7cde94e0cb4847ab5624e9e6f73778a3dac1
tech-stack:
  added: []
  patterns:
    - "the key bar sends through term.input(seq, true), so bar bytes take the same onData to sendInput path as typing"
    - "an armed Ctrl lives in a ref read by an inputTransform inside onData, plus state for the key's look"
key-files:
  created:
    - frontend/src/lib/ctrlMap.ts
    - frontend/src/lib/__tests__/ctrlMap.test.ts
    - frontend/src/components/KeyBar.tsx
    - frontend/src/components/__tests__/KeyBar.test.tsx
    - frontend/src/components/__tests__/WebShareSessionView.keybar.test.tsx
    - frontend/src/components/__tests__/style.keybar.test.ts
    - frontend/e2e/mobile-iphone.spec.ts
  modified:
    - frontend/src/components/TerminalPanel.tsx
    - frontend/src/components/WebShareSessionView.tsx
    - frontend/src/App.tsx
    - frontend/src/style.css
    - frontend/playwright.config.ts
    - frontend/e2e/web-share-scope.spec.ts
key-decisions:
  - "KeyBar disarms Ctrl before calling send, so the input transform never maps a bar key's bytes a second time"
  - "canWrite is undefined when the info call fails or perms is missing, so the bar shows (the server still drops the input)"
requirements-completed: []  # MOB-01 also needs the iOS Simulator run (14-13) and the real-phone check (14-14); requirements.mark-complete not run, same as 14-03, 14-04 and 14-08
---

# Phase 14 Plan 10: Phone key bar and one-shot Ctrl Summary

A phone in the browser share view now gets one row of eight keys (Esc, Tab, Ctrl, four arrows, `|`) whose bytes reach the shell through the normal typing path, with a one-shot Ctrl that also works on the iOS keyboard; proven in WebKit's iPhone 15 emulation against the fixture's PTY stub.

Logs: `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.pxgKsYvXd4` (Task 1), `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.GQ6zmE1I8E` (Task 2), `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.gq45mcftGR` (Task 3: full vitest, tsc, share-removal, full Playwright).

## Accomplishments

- **ctrlMap.** `ctrlByte(ch)` (A-Z and a-z to 0x01-0x1A; `@` and space to 0x00; `[` `\` `]` `^` `_` to 0x1b-0x1f; null otherwise, including multi-character strings), `arrowSeq(dir, applicationCursor, ctrl)` (`ESC [ x`, `ESC O x` in application mode, `ESC [ 1 ; 5 x` with Ctrl in either mode), and the ESC, TAB, PIPE constants.
- **KeyBar.** `role="toolbar"`, `aria-label="Terminal keys"`, horizontal; real `<button type="button">` elements; heroicons arrows at 20px with `aria-hidden`; accessible names Esc, Tab, Ctrl (with `aria-pressed`), Left/Up/Down/Right arrow, Vertical bar. Fires on `pointerdown` with `preventDefault`, and on `click` only when `detail === 0`. No repeat.
- **TerminalPanel seams.** `keyInputRef` is filled in the mount effect (`send` = `term.input(seq, true)`, `applicationCursor` = `term.modes.applicationCursorKeysMode`) and cleared on dispose. `inputTransform` is read through a ref inside the existing `term.onData` handler, so the mount effect's deps did not change.
- **WebShareSessionView.** Ctrl state (ref for the transform, state for the key). The bar renders after TerminalPanel only when not remote, `usePhone()`, `isActive`, and `canWrite !== false`. Ctrl releases when `isActive` goes false and on unmount. App reads `perms` from the bootstrap `/info` response and passes `canWrite`.
- **CSS.** `.keybar` per UI-SPEC, tokens only; armed Ctrl is the accent fill with an 8px on-accent dot; backstop hides the bar when the phone query does not match.
- **Playwright.** `iphone` project (`devices['iPhone 15']`, `testMatch /mobile-.*\.spec\.ts$/`) and `testIgnore` for the same pattern on chromium, firefox and webkit.

## Did tap() fire pointerdown in WebKit emulation?

Yes. Playwright's `tap()` on the iPhone 15 project dispatched `pointerdown` (the click that follows has `detail` 1 and is ignored), so each tap sent exactly once. No synthetic `pointerdown` fallback was needed; the spec says so in a comment.

## Captured bytes (iPhone 15 WebKit emulation, exact match on /__test__/input)

| Action | Hex |
|--------|-----|
| Esc | `1b` |
| Tab | `09` |
| `\|` (Vertical bar) | `7c` |
| Up | `1b5b41` |
| Ctrl, then Up | `1b5b313b3541` (Ctrl's `aria-pressed` returns to false) |
| Ctrl, then typing `c` on the keyboard | `03` (Ctrl releases) |

The eight buttons share one `y`, are each at least 44 x 44 and end inside the 393 pt viewport.

## Test results

- Task 1: `tsc` clean; iphone project 1 passed.
- Task 2: ctrlMap, KeyBar, WebShareSessionView suites 6 files, 69 tests passed; iphone project 6 passed.
- Task 3: full vitest (run once) 117 files, 1772 tests passed; `tsc --noEmit` clean; `tests/check-share-removal.sh` exit 0; full Playwright (run once) 48 passed, 31 skipped, 0 failed. The 7 iphone tests ran only in the iphone project; the desktop-absence assertion in web-share-scope passed in chromium, firefox and webkit.
- All plan verify greps passed.

## Deviations from Plan

### Auto-fixed Issues

**1. [Environment] Verify greps need uncolored vitest output and nvm node first on PATH**
- Ran vitest with `NO_COLOR=1` and Node v24.22.1 first on PATH (caller's rule). No plan change.

**2. [Rule 1 - Bug] Key bar height was 48px, not the spec's 52px**
- **Found during:** Task 3 (writing the style gate against UI-SPEC "Component dimensions").
- **Issue:** the first CSS drew padding-bottom as the safe-area inset alone, so the bar was 4 + 44 + 0 tall (plus the 1px border), not 4 + 44 + 4.
- **Fix:** `padding-bottom: calc(4px + env(safe-area-inset-bottom))`. 14-11's `keybar--kb-up` class (padding-bottom 0 per UI-SPEC) is not part of this plan.
- **Files modified:** frontend/src/style.css, frontend/src/components/__tests__/style.keybar.test.ts
- **Commit:** ab5e7cde

**3. [Scope] canWrite and the App wiring landed in Task 2 as planned, but the prop was typed in the view during Task 2, not Task 1**
- Task 1's view treated the bar as allowed with no `canWrite` prop. No behaviour difference at the Task 1 commit.

### Deferred

None.

## Not verified

- **The armed Ctrl in light and dark on a real WebKit** (the plan's `backstop` truth). The style gate pins the tokens, and the iPhone project ran with the default light scheme only; nobody looked at the armed key in either scheme. This belongs to the iOS Simulator run (14-13) and the real-phone check (14-14).
- **Keyboard docking** (the bar riding above the iOS keyboard) is 14-11; **Ctrl then C stopping a running command** on a real shell is 14-13/14-14. Here Ctrl then `c` is proven to reach the PTY stub as `03`.
- **Application cursor mode in a real terminal** is pinned in vitest (KeyBar with `applicationCursor` true sends `ESC O A`); the e2e does not enter application mode.
- **Sub-352 pt scrolling.** Only the 393 pt iPhone 15 viewport was measured.

## Known Stubs

None.

## Threat Flags

None. T-14-10 (no bar without write; `term.input` is a no-op under `disableStdin`; the server still discards input), T-14-24 (closed byte table, one-shot Ctrl) and T-14-25 (pointerdown versus click detail, React phone check plus CSS backstop) are mitigated as planned and covered by the tests above.

## Self-Check: PASSED

Files found: ctrlMap.ts, KeyBar.tsx, mobile-iphone.spec.ts, style.keybar.test.ts, KeyBar.test.tsx, WebShareSessionView.keybar.test.tsx, ctrlMap.test.ts, 14-10-SUMMARY.md. Commits found: 02813e3c, d1fa8a99, ab5e7cde.
