---
phase: 14-mobile-web-app
plan: 08
subsystem: frontend
tags: [react, phone, reload, standalone, pdf, vitest]
requires:
  - phase: 14-04
    provides: connectStuck state and the data-connect-stuck attribute on the TerminalPanel root
provides:
  - phoneQuery module (PHONE_QUERY, isPhone, usePhone, isStandalone), reused by 14-10 and 14-11
  - SessionEndedBanner onReload action
  - web-session ErrorBoundary reload copy (webMode)
  - standalone in-app PDF view with Done and Open full PDF
affects: [14-10, 14-11, 14-12, 14-13, 14-14]
status: complete
actuals:
  tokens: 8200
  tasks: 3
  commits: 3
plan_head_before: 77699b3e62296274a5d5b97e094770f3fe509237
plan_head_after: ed20db3312e081526369528c792f34ea8361de8a
tech-stack:
  added: []
  patterns:
    - "one module decides phone and standalone; components never read the user agent or ontouchstart"
key-files:
  created:
    - frontend/src/lib/phoneQuery.ts
    - frontend/src/lib/__tests__/phoneQuery.test.ts
  modified:
    - frontend/src/components/SessionEndedBanner.tsx
    - frontend/src/components/__tests__/SessionEndedBanner.test.tsx
    - frontend/src/components/TerminalPanel.tsx
    - frontend/src/components/__tests__/TerminalPanel.guestClose.test.tsx
    - frontend/src/components/__tests__/TerminalPanel.reconnect.test.tsx
    - frontend/src/components/ErrorBoundary.tsx
    - frontend/src/components/__tests__/ErrorBoundary.test.tsx
    - frontend/src/App.tsx
    - frontend/src/style.css
    - frontend/src/components/PdfPreview.tsx
    - frontend/src/components/__tests__/PreviewRenderers.test.tsx
key-decisions:
  - "The guest end banner is one element for 'ended' and 'share-stopped', so both get Reload from the same onReload"
  - "The unreachable banner always carries Reload on a phone; reconnecting carries it only while connectStuck"
requirements-completed: []  # MOB-03 needs the real-phone check (14-14); requirements.mark-complete not run, same as 14-03 and 14-04
---

# Phase 14 Plan 08: Reload states, web error copy and the Open PDF way back Summary

A phone now has a one-tap Reload on every finished, stuck or broken view, and a Home Screen web app opens a PDF in an in-app layer with a Done control instead of navigating away.

Logs: `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.rzA0lhAGTu` (Task 1), `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.BqldZVi9eV` (Task 2), `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.twL9LEteee` (Task 3 focused) and `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.0UiIHY9Vyp` (Task 3 full suite, tsc, share-removal).

## Accomplishments

- **phoneQuery.** `PHONE_QUERY = '(pointer: coarse) and (hover: none)'`, `isPhone(): boolean`, `usePhone(): boolean` (state plus `change` listener with cleanup, shaped like `useColorScheme`), `isStandalone(): boolean` (`(display-mode: standalone)` or `navigator.standalone === true`).
- **Reload.** `SessionEndedBanner` takes `onReload?: () => void` and renders `<button type="button" className="banner__action">Reload</button>` before Dismiss for kinds `share-stopped`, `ended`, `unreachable`, `reconnecting` only. TerminalPanel passes `location.reload()` only when `usePhone()` is true; for the reconnecting banner only while `connectStuck`. Under `@media (pointer: coarse) and (hover: none)`, `.banner__action` is `min-height: 44px; min-width: 44px; padding: 0 8px`.
- **ErrorBoundary.** New optional `webMode?: boolean`. With it, the fallback reads "This view hit an error. Reload to try again." plus a Reload button when `isPhone()`. Without it the "This tab hit an error. Close it and open the session again." copy is unchanged. App passes `webMode={mode === 'web'}` to the `web-session` boundary only.
- **PDF layer (standalone only).** PdfPreview keeps its pane. When `isStandalone()`, Open PDF sets `inApp` and renders `.pdf-layer`: `position: fixed; inset: 0`, `role="dialog"`, `aria-modal="true"`, `aria-label` the file name, on `--color-window`. Inside, `.pdf-layer__bar` (`--color-toolbar`, 1px `--color-separator` bottom edge, safe-area padding top and sides) holds the file name (13px/600, ellipsis), an "Open full PDF" button (the old `window.open(url, '_blank', 'noopener,noreferrer')`) and a "Done" button, both 44 x 44 minimum in `--color-text`. `.pdf-layer__body` scrolls vertically and holds `<img src={url} alt={name}>` at full width. Focus moves to Done on open; Done and Esc (document keydown) close it and return focus to Open PDF. Outside standalone mode Open PDF still calls `window.open`. No CSP change, no new package.

## Test results

- Task 1 focused (phoneQuery, SessionEndedBanner, TerminalPanel.guestClose): 3 files, 60 tests passed.
- Task 2 focused (TerminalPanel.reconnect, ErrorBoundary, SessionEndedBanner): 3 files, 70 tests passed.
- Task 3 focused (PreviewRenderers): 23 tests passed.
- Full vitest (run once): 113 files, 1732 passed, 1 failed (see Deviations). The failing file passes alone (9 of 9).
- `tsc --noEmit`: clean after every task. `tests/check-share-removal.sh`: exit 0.
- All plan verify greps passed.

## Deviations from Plan

### Auto-fixed Issues

**1. [Environment] Verify greps need uncolored vitest output**
- Same as 14-04: ran vitest with `NO_COLOR=1` so `Tests +[0-9]+ passed` matches. No plan change.

**2. [Scope] Task 1's guest-banner wiring covers 'ended' as well as 'share-stopped'**
- The guest end banner is a single element whose `kind` is chosen from `sessionEnded.shareStopped`, so passing `onReload` there gave both kinds Reload in Task 1. Task 2 added the tests that pin `ended` in the reconnect suite.

### Deferred

- `pathLinks.linkifier.test.ts` "after a reconnect, another cell of the same row offers the link" failed once in the full run (`term.onRender fired: expected 2 to be greater than 2`, a fixed `sleep(100)` wait on a real xterm render). It passes alone, and the file imports nothing this plan touched. Logged in `deferred-items.md`. Not repeated, because the plan allows the full suite once.

Otherwise the plan ran as written.

## Auth gates

None.

## Known Stubs

None.

## Threat Flags

None. T-14-13 (Open PDF stranding) and T-14-42 (Reload is user-initiated only, same token URL, phone and listed states only) mitigations are in place and each has pinning tests. No package was installed (T-14-SC).

## Not verified

- iOS WebKit behavior of the PDF `<img>` (it may draw only page 1) and the Home Screen standalone flow on a real iPhone: those are 14-13 S-06 and 14-14. Done is the way back whatever the image shows; Open full PDF keeps the whole document reachable.
- The 44 pt tap target and the safe-area insets are CSS only: jsdom has no layout, so nothing measured them.
- No browser run, no host tailscaled contact, no Wails rebuild.
- ESLint is not installed in `frontend/`, so it was not run.

## Self-Check: PASSED

- Files: phoneQuery.ts, phoneQuery.test.ts present.
- Commits 6309bc53, 3e3de536 and ed20db33 are ancestors of HEAD; `git rev-list --count` from the ledger base is 3.
