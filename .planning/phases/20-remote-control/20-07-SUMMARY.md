---
phase: 20-remote-control
plan: 07
subsystem: frontend
tags: [control-bar, picker, send-keys, lock-screen, swap, fullscreen, keyboard-lock, context-menu, desktop-viewer]
requires:
  - phase: 20
    plan: 03
    provides: "surfaces / select / select_failed messages and the surface message (real main display id)"
  - phase: 20
    plan: 06
    provides: "desktopInput (sendChord, setSwap, releaseAll, setPaused), DesktopView control state, notice slot, onReleaseChord"
provides:
  - "DesktopControlBar: top-center auto-hiding toolbar with picker, Send Keys, Swap, View Only/Control, Full Screen; barAvailability table"
  - "DesktopSendKeysMenu: standalone Send Keys menu that owns its Lock This Mac confirm (Phase 21 places it unchanged)"
  - "ContextMenu extension: checked items, hints, label and caption rows, className, Home/End, late-arriving lists"
  - "desktopClient: requestSurfaces, selectSurface, surfaces and selectFailed events"
affects: [20-08, 20-11, 21, 22]
tech-stack:
  added: []
  patterns:
    - "menus and dialogs portal into document.fullscreenElement when there is one, or they are invisible in full screen"
    - "greyed = aria-disabled with the reason as title and as linked visually-hidden text"
key-files:
  created:
    - frontend/src/components/DesktopControlBar.tsx
    - frontend/src/components/DesktopSendKeysMenu.tsx
    - frontend/src/components/__tests__/DesktopControlBar.test.tsx
    - frontend/src/components/__tests__/DesktopSendKeysMenu.test.tsx
  modified:
    - frontend/src/components/ContextMenu.tsx
    - frontend/src/components/DesktopView.tsx
    - frontend/src/lib/desktopClient.ts
    - frontend/src/style.css
    - frontend/src/components/__tests__/ContextMenu.test.tsx
    - frontend/src/components/__tests__/DesktopView.test.tsx
    - frontend/src/lib/__tests__/desktopClient.test.ts
key-decisions:
  - "DesktopView owns View Only/Control, Swap, Full Screen and the switch-in-flight state; the bar is presentational. The controlMode and menuOpen props stay and still win when they change"
  - "A pick of the surface already shown does nothing (no epoch bump, no Switching flicker)"
  - "The surface notice is pushed before releaseAll('surface') so that, when something was held, Held keys were released is the notice that stays"
requirements-completed: [DCTL-03, DESK-05, DESK-06, DCTL-05]
status: complete
duration: ~2h (interrupted once by an API rate limit)
completed: 2026-11-26
estimate:
  tokens: 90000
  raw_tokens: 90000
  tasks: 3
  confidence: low
actuals:
  tokens: 31000
  tasks: 3
  commits: 3
plan_head_before: 1af57e1f104fe64b042317269d473b8c39730fe5
plan_head_after: e31f38250b5a96945466baf219aca59c86f0bf47
---

# Phase 20 Plan 07: Control bar - picker, Send Keys, Lock Screen confirm, Swap, View Only/Control, Full Screen Summary

The viewer now has one slim, auto-hiding top-center bar: a display and window picker with honest Switching and failure feedback, a standalone Send Keys menu whose Lock Screen needs a Lock This Mac confirm, the Swap toggle, a View Only/Control radiogroup and Full Screen with Keyboard Lock, all following the UI-SPEC availability table.

## Reconcile gate (Task 1 precondition)

`git log --oneline d792ca07..HEAD -- frontend/src/style.css frontend/src/components/RegenerateKeyModal.tsx frontend/src/components/ContextMenu.tsx` lists, all already in this base: `a9b4e9b8` and `fc87909f` (20-06 control-layer CSS), `01bd357f`, `08d2ee85`, `95d6306e` (Phase 19: Test Capture thumbnail, viewer card buttons, 19-13 Desktop group styles and Regenerate copy). I read the current RegenerateKeyModal (its default copy is 19-13's) and passed every Lock Screen string explicitly (title, body, confirmLabel, cancelLabel, actingLabel, titleId, destructive false) so default changes cannot leak in. The 20-07 CSS is one block appended after 20-06's; no earlier rule was touched. ContextMenu.tsx had no Phase 19 changes in range.

## What was built

- **Task 1 (tracer, 60e1e93c):** ContextMenu takes `checked` (role menuitemradio, 16 px check column kept for every item), `hint`, `title`, `{kind: 'label' | 'caption'}` rows (not focusable, skipped by arrows, captions linked by `aria-describedby`), `className`, Home/End, and focuses the first item when a list that started empty gets one. `DesktopSendKeysMenu` (Cmd+Tab, Cmd+Space, Lock Screen… with hint Ctrl+Cmd+Q) calls `sendChord`; through a real `createDesktopInput` the burst is MetaLeft down m 8, Tab down m 8, Tab up m 8, MetaLeft up m 0, and the swap does not apply. The tracer gate (automated-only verify) passed before expanding.
- **Task 2 (26f58bf2):** `requestSurfaces` (5 s timeout, rejects on `list_failed`/`forbidden` and on a dropped connection), `selectSurface`, `surfaces` and `selectFailed` events with typed fields only (names as strings, ids and sizes as integers; checked against the golden `surfaces`, `surfaces-error`, `select-*`, `select-failed` lines). Picker menu with Displays/Windows labels, greyed Loading…, Main Display checked with `1470 x 956` right-aligned, `App — Title` windows, `No windows to show`, the list-failure item and the window-share caption. DesktopView: Switching… until the matching surface message, revert plus a role=alert notice after 5 s or on `select_failed`, `Showing …` and the two fallback notices for every surface message, phones included.
- **Task 3 (e31f3825):** `barAvailability`, reveal/hide (5 s first frame, 3 s on lock/access/surface change, top-center zone, focus-within, open menu, 1.5 s hide), 880 px density in CSS, Swap toggle with check mark, View Only/Control `.share-segbar` with the release-chord tooltip, Full Screen with Keyboard Lock and both notices, the Lock This Mac confirm (Don't Lock focused first, Esc does the same, only the confirm sends Ctrl+Cmd+Q, never in a window share), chips moved below the 44 px bar row (`desktop-view--bar`), the release chord focuses the Control segment.

## Verification

- `pnpm --dir frontend exec tsc --noEmit -p tsconfig.json`: clean.
- `pnpm --dir frontend test`: 132 files, 2247 tests pass (includes the token source gate).
- `bash tests/check-brand.sh`: clean. Acceptance greps all pass (`export function DesktopSendKeysMenu` 1, `checked?: boolean` 1, `role="toolbar"` 1, caption text 1, `selectSurface` 2 in desktopClient, `dangerouslySetInnerHTML` 0, Remote unlock 1, Don't Lock 1, `barAvailability` 1, `max-width: 879px` 1).
- Host safety: only vitest in jsdom ran; no viewer, capture, injection or tailnet access.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing critical] Menus and dialog would be invisible in full screen**
- **Found during:** Task 3 design
- **Issue:** ContextMenu and the Lock confirm portal to `document.body`; while an element is full screen only that element and its descendants are drawn, so every bar menu and the confirm would vanish in full screen, and the confirm is the only guard in front of Lock Screen.
- **Fix:** both portal into `document.fullscreenElement ?? document.body`; tested with a stubbed fullscreen element. Not verifiable in jsdom beyond that.
- **Files:** ContextMenu.tsx, DesktopSendKeysMenu.tsx. **Commit:** e31f3825

**2. [Rule 1 - Bug] Lock confirm stayed mounted after Don't Lock / Esc / confirm when the host did not unmount it**
- **Found during:** Task 3 tests (RED). **Fix:** the component closes its own confirm state before calling `onClose`. **Commit:** e31f3825

**3. [Rule 2 - Missing critical] "Typing stays on this device" never showed on a mouse press on the bar**
- **Issue:** the press blurs the hidden input before any effect runs, so the 20-06 `menuOpen` check saw "not captured". Read at `pointerdown` capture on the bar overlay instead (also feeds "return focus to the stage").
- **Commit:** e31f3825

**4. [Rule 3 - Blocking] No `node_modules` in the worktree**
- `pnpm --dir frontend install --frozen-lockfile --offline` (lockfile only, no new package); `node_modules` is gitignored.

### Other adjustments (not bugs)

- Commit trailer names `Claude Sonnet 5.5` (the model that wrote it), not the plan's Opus.
- `ControlMode` now lives in DesktopControlBar.tsx; DesktopView re-exports it, so the 20-06 import path still works.
- A trigger press while its menu is open closes it and does not reopen it (press tracked in a ref).
- The bar does not render the Clipboard button: 20-08 inserts it after Send Keys. `barAvailability` already has the `clipboard` row (greyed while locked/reconnecting; no right-based hiding yet, since `rights.clipboard` is 20-08's).
- `control` in `barAvailability` gains an optional `controlOnly` flag (Control needs Accessibility greys only the Control segment).
- Task 2 and 3 tests were written alongside their implementation; the client tests (Task 2) were run RED first and committed with their implementation.
- `gsd_run windows append` was not run (helper unavailable in this worktree shell); no stubs to record.

## Cross-plan notes for the orchestrator (Phase 21 merge)

- DesktopView.tsx edits are additive: new state (`surf`, `switching`, `mode`, `swapOn`, `canSurface`, `fullscreen`, `barMenuOpen`), refs (`rootRef`, `controlSegRef`, `clientRef`), a `fullscreenchange` effect and the `showBar` block at the end of the JSX. The 20-06 `controlMode`/`menuOpen` props still work. desktopClient.ts additions are new events/methods plus `RECONNECT.surfacesTimeoutMs`.
- 20-08 should add its Clipboard button inside the Send Keys group in DesktopControlBar.tsx (`groups.push(<BarButton key="sendKeys" .../>)`) and use `onMenuOpenChange` for its popovers.
- A stray empty file `placeholder` was written outside the repo under /private/tmp by mistake; it is not in the worktree.

## TESTING.md rows for plan 20-11 (not edited here)

- Section 2, frontend group: new `frontend/src/components/__tests__/DesktopControlBar.test.tsx` (38 cases) and `frontend/src/components/__tests__/DesktopSendKeysMenu.test.tsx` (15). Extended: `ContextMenu.test.tsx`, `DesktopView.test.tsx`, `frontend/src/lib/__tests__/desktopClient.test.ts`.
- Section 4 traceability (path column file only): DCTL-03 `frontend/src/components/__tests__/DesktopSendKeysMenu.test.tsx`; DCTL-03 `frontend/src/components/__tests__/DesktopControlBar.test.tsx`; DESK-05 `frontend/src/components/__tests__/DesktopControlBar.test.tsx`; DESK-05 `frontend/src/lib/__tests__/desktopClient.test.ts`; DESK-06 `frontend/src/components/__tests__/DesktopView.test.tsx`; DCTL-05 `frontend/src/components/__tests__/DesktopView.test.tsx`.
- Section 5 manual (new M-NN, VM browser): (a) bar, menus and chips read correctly in light and dark appearance; (b) Chrome grants Keyboard Lock in full screen, a 2 s Esc hold leaves it, and the bar menus and the Lock This Mac dialog are visible inside full screen; (c) Safari and Firefox full screen shows the "browser keeps Cmd shortcuts" notice; (d) Send Keys Cmd+Tab and Cmd+Space reach the Mac and leave no modifier held; (e) the picker lists the real displays and windows, a switch shows Switching… then Showing …, and a window that closes shows the fallback notice.

## Threat Flags

None. Window titles render as React text only (T-20-35, grep criterion plus an injection-string test); Lock Screen sits behind the confirm and is greyed in window shares (T-20-36, T-20-37); the release chord focuses the Control segment and the hidden bar stays reachable by Tab (T-20-38); no package added (T-20-SC).

## Self-Check: PASSED

Created files exist (DesktopControlBar.tsx, DesktopSendKeysMenu.tsx and both new test files); commits 60e1e93c, 26f58bf2 and e31f3825 are on the branch; the full frontend suite, tsc and `tests/check-brand.sh` pass.
