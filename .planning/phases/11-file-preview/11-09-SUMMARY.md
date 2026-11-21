---
phase: 11-file-preview
plan: 09
subsystem: frontend
status: complete
tags: [frontend, preview, tabs, focus, accessibility]
requires: [11-05, 11-06, 11-07]
provides:
  - "Tab type 'preview' (ownerTabId, previewKind, owner snapshot) and its TabBar anatomy"
  - "App: one PreviewStore, preview tab insertion, the focus rule, close rules, keep-mounted session view, live region"
  - "PreviewPanel: Receiving / Too Large / Couldn't Receive / Couldn't Show panes and renderer dispatch in an ErrorBoundary"
affects: [11-10]
tech-stack:
  added: []
  patterns:
    - "Preview tabs live in App's tab list; the store is the source of truth for entries, and an effect drops tabs whose entry the store evicted"
    - "The mounted web-session view is the one for the session tab (sessionTabId = the active preview's owner, else the active tab)"
    - "Only the visible preview's pane is mounted"
key-files:
  created:
    - frontend/src/components/PreviewPanel.tsx
    - frontend/src/components/__tests__/App.preview.test.tsx
    - frontend/src/components/__tests__/PreviewPanel.test.tsx
  modified:
    - frontend/src/App.tsx
    - frontend/src/components/TabBar.tsx
    - frontend/src/style.css
    - frontend/src/components/__tests__/TabBar.test.tsx
    - frontend/src/components/__tests__/App.open-remote.test.tsx
    - frontend/src/components/__tests__/App.nav.test.tsx
decisions:
  - "A preview Tab's sessionId is its own tab id (never the owner's), so the existing tabProgress map drives its underline and no owner side effect (font size, progress, save) applies to it"
  - "Every close (×, ⌘W, Close Tab) goes through removeTabState, which calls store.close for a preview and returns to the owner tab (with terminal focus) when that tab is still open"
  - "The tab carries an owner snapshot (name, type, peerHost) so its tooltip and accessible name still name the session after the session's tab closes; while that tab is open the live label is used"
  - "Font-size and find keys needed no change: they are xterm key handlers that only fire while a terminal has focus, and a preview pane never holds terminal focus; there is no app-level zoom or find shortcut"
metrics:
  duration: "~95 min"
  completed: 2026-11-21
actuals:
  tokens: 16000    # chars/4 over the realized diff (63,828 chars, 9 files)
  tasks: 3
  commits: 3
plan_head_before: b8a2516e5e9d8c5497187a5ea610d07dc3c8a736
plan_head_after: 20acccf524b262315ae8f90f48c1eb886c7f74c2
---

# Phase 11 Plan 09: Preview tabs Summary

A `portty open` preview now opens as a native-looking tab right of its session's tab in every viewer (local tab, peer tab, browser viewer). Only the viewer that typed last switches to it, and only from that session. A peer or browser session's view stays mounted and connected behind its preview. PreviewPanel shows every UI-SPEC state with the exact copy.

## As built

### Tab id scheme

- Preview tab id = the store's `__preview__<ownerTabId>/<key>` (never starts with `__websession__`).
- `ownerTabId` = the session's tab id: the session id for a local tab, `webSessionTabId(id, host)` for a peer tab, `__websession__<id>` for the browser viewer's tab.
- `sessionId` = the preview's own tab id. `previewKind` sets the icon. `owner` = `{name, type, peerHost}` of the owner tab when the preview first arrived.

### Arrival and position

- `handlePreview(ownerTabId, ev)` is wired to every local TerminalPanel (`onPreview={(ev) => handlePreview(tab.id, ev)}`) and to the WebShareSessionView (`handlePreview(sessionTabId, ev)`).
- On `start`, `insertPreviewTab` puts the tab after the last tab that is the owner or one of its previews. With neither open, it goes at the end. An id that is already open is left alone (a refresh keeps its tab).
- Store evictions (10 tabs, 100 MiB) remove their tabs through an effect on the store snapshot. `store.setActive(activeId)` runs on every active change, and `store.closeAll()` runs on unmount.

### Focus rule (locked: "Only the viewer you typed in")

A start switches the active tab only if `ev.focus` is true **and** the active tab is the owner tab or one of that owner's previews. In that case it also sets `focusPreview`, and the pane gets `autoFocus`: the renderer focuses its own target, else PreviewPanel focuses its pane (`tabIndex=-1`). `focusPreview` clears as soon as the active tab changes. Every other arrival only adds the tab: the active tab, keyboard focus and open dialogs stay as they were. Every start (new or refresh) writes **Preview opened: <name>** into one `.visually-hidden` `aria-live="polite"` div at the App root.

### Keep-mounted session view

`sessionTabId = activePreview?.ownerTabId ?? activeId`. The `__websession__` branch renders for `sessionTabId`:

- The view is keyed `${sessionTabId}:${wsCapToken}`, and the ErrorBoundary resetKey is `sessionTabId`.
- The wrapper is `display: none` while a preview is active, and the view gets `isActive={!activePreview}`.
- Local TerminalPanels were already all mounted. The local map skips `preview` tabs.
- The Sidebar gets `tabs.find((t) => t.id === sessionTabId)`, so the owner's row stays selected.
- The toolbar shows no Share button and no chip for a preview tab (existing `isLocal` and `web-session` checks). The Share popover closes on the activeId change (existing effect).

### Close rules

- removeTabState, for a preview: `store.close(id)` (revokes the URL), and the tab is removed.
- If the preview was active and its owner tab is open, the owner tab is activated, and an effect focuses its `.xterm-helper-textarea` once that tab is shown. Otherwise `nextActiveTabId` decides.
- Closing the owner's tab and `session:exit` never touch previews.

### PreviewPanel

`PreviewPanel({entry, webMode, autoFocus})`, root `div.preview-panel[role=tabpanel][id=tabpanel-<tabId>][aria-labelledby=tab-<tabId>][tabIndex=-1]`. TabBar tab buttons now carry `id="tab-<id>"`.

| State | Pane |
|-------|------|
| receiving | Nothing until `arrivedAt + 300 ms`. Then the type icon, the name, `<progress aria-label="Receiving <name>">` and **Receiving… 4.2 MB of 18.6 MB** (decimal; whole KB below 1 MB). Not a live region |
| tooLarge | **Too Large to Preview** / **<name> is larger than 20 MB.** (`role=status`) |
| failed | **Couldn't Receive File** / **<name> didn't arrive completely. Run portty open again.** |
| rendered | ImagePreview / PdfPreview / MarkdownPreview, keyed by url, inside `ErrorBoundary label="preview"` whose fallback is **Couldn't Show File** / **<name> may be damaged, or its format isn't supported.** A renderer's `onFail` shows the same pane for that url. A refresh (new url) renders again |
| refreshing | The old content stays (the store keeps `state: 'rendered'`) |

Only the active preview's PreviewPanel is mounted. This is the CONTEXT "render a preview pane only while its tab is visible" decision.

### TabBar preview anatomy

- Leading icon: PhotoIcon (image, svg), DocumentIcon (pdf) or DocumentTextIcon (markdown), 14px, `aria-hidden`.
- Name in `.tab__name`, with no rename.
- The owner's PeerDot for a peer owner.
- Tooltip: `<name> — <session>`, or `<name> — <session> · <host>` for a peer.
- Accessible name: `<name>, preview from <session>`, or `… on <host>` for a peer.
- `.tab__close` with `Close <name>`, shown in web mode too (the session pill keeps none). Desktop context menu: Close Tab only. No menu in web mode.
- Underline from `received/size` while receiving or refreshing.

### Styles

- `.preview-panel` and `.preview-message .empty-state__heading { overflow-wrap: anywhere }`.
- `.preview-receiving__bar`: 200×4, radius 2, track `--color-separator`, fill `--color-selection-fill`.
- `.preview-receiving__caption`: 12/16, tabular numerals, secondary color.
- `.visually-hidden`.
- `.tab__progress { transition: none }` under `prefers-reduced-motion: reduce`.

## App test pins changed (render shape only, nothing weakened)

| File | Was | Now |
|------|-----|-----|
| App.open-remote.test.tsx (4 tests) | `raw.indexOf("activeId.startsWith('__websession__')")` | `raw.indexOf("sessionTabId.startsWith('__websession__')")` |
| App.open-remote.test.tsx (CR-03) | ``key={`${activeId}:${wsCapToken}`}`` | ``key={`${sessionTabId}:${wsCapToken}`}`` (the plan's gate requires the old key to be gone) |
| App.open-remote.test.tsx (3 tests) | `raw.slice(idx, idx + 3500)` | `raw.slice(idx, raw.indexOf('})()}', idx))`: the exact branch, since the branch outgrew 3500 chars. Same assertions, a tighter slice |
| App.nav.test.tsx | `activeTab={tabs.find((t) => t.id === activeId) ?? null}` | `activeTab={tabs.find((t) => t.id === sessionTabId) ?? null}` (not in the plan's file list; the UI-SPEC sidebar rule requires it) |

App.test.tsx needed no change: its WebShareSessionView font pins still match.

## Tests (for 11-10's TESTING.md rows)

- `frontend/src/components/__tests__/App.preview.test.tsx` (18 tests):
  - "App preview tabs: a local session (PREV-01, PREV-02)": focused PNG beside its tab; × returns and revokes.
  - "… position (PREV-01)": grouped in arrival order, never between B and B's previews.
  - "… focus rule (locked: only the viewer you typed in)": a focused arrival moves focus into the pane; focus true from another session stays in the background; focus false never switches; a focused arrival from one of the session's previews switches; a background arrival leaves keyboard focus and an open dialog alone.
  - "… close rules (UI-SPEC "Close behavior")": the active preview's close activates the owner and focuses its terminal; closing the owner tab leaves previews (then `nextActiveTabId`); `session:exit` leaves previews; ⌘W (`menu:close-tab`).
  - "… refresh, limit, live region, unmount": the same key refreshes in place; the underline shows received/size; the 11th preview evicts the oldest non-active one; the polite live region; unmount revokes every URL.
  - "… the session view stays mounted (UI-SPEC "Viewer Connection Requirement")": a desktop peer tab (no unmount, `isActive` false, display none, row selected, no chip or Share, peer tooltip, name and dot, no remount on switching back); the browser viewer bootstrap tab (mounted behind the preview, × on the preview only, back to Session).
- `frontend/src/components/__tests__/PreviewPanel.test.tsx` (14 tests): "PreviewPanel: receiving", "PreviewPanel: message panes", "PreviewPanel: focus" (300 ms delay, MB and KB captions, no partial render, each pane's copy, onFail, ErrorBoundary, refresh keeps content, refresh after a failure, kind dispatch, hostile name as text, renderer target focus, pane focus, no focus without autoFocus).
- `frontend/src/components/__tests__/TabBar.test.tsx`, "TabBar preview tabs (PREV-01, PREV-04)" (9 tests): icons by kind, local and peer tooltip and accessible name, the owner snapshot, no rename, desktop Close Tab only, web-mode × only on previews with no menu, underline, no HTML strings.
- Mutation check (reverted): keying the view by `activeId` and dropping the focus guard failed 4 App.preview tests (both keep-mounted tests, the background-arrival test and the focus-from-another-session test).

## Gate results

- Task 1 verify: PASS. The tracer gate re-ran the Task 1 verify after its commit: PASS (`/tmp/p11-09-t1-tracer.log`).
- Task 2 verify: PASS (4 vitest files, 211 tests, `/tmp/p11-09-t2.log`). All App test files passed too: 283 tests (`/tmp/p11-09-t2-apps.log`).
- Task 3 gate, on the code rebased onto gsd/v1.2 b8a2516e:
  - Copy strings and `.visually-hidden`: present.
  - Full vitest: 104 files, 1510 tests passed (`/tmp/p11-09-gate-vitest.log`). No flake reruns.
  - `tsc --noEmit`: clean (`/tmp/p11-09-gate-tsc.log`).
  - Standard-build lazy-load check (11-07 deviation 4), `vite build --outDir /tmp/p11-09-dist`:
    - The entry chunk `index-cNI0F7By.js` has no `ALLOWED_URI_REGEXP`, no DOMPurify and no marked.
    - DOMPurify is only in `markdownSanitize-DO9YwuFP.js` (29 KB). marked is only in the worker chunk `markdownWorker-DbMf6T8E.js` (50 KB).
    - The small MarkdownPreview component itself sits in the entry chunk (PreviewPanel imports it statically); its parser and sanitizer load lazily.
  - `go build ./...` and `go vet ./...`: clean.
  - `go test -race -short -count=1 ./...`, run once: exit 0, all 13 packages `ok`, 0 FAIL lines, no DATA RACE, panic or SKIP lines. No flake reruns (`/tmp/p11-09-gate-go.log`).
  - check-brand: clean. check-traceability-paths: 175 paths exist.
  - Since b8a2516e, only this plan's 9 files changed. No Go file, renderer, `frontend/src/lib`, TerminalPanel or WebShareSessionView changed. `frontend/src/wailsjs` is untouched.

## Deviations from Plan

1. **[Coordinator] Rebased onto gsd/v1.2 before the gate.**
   - The Markdown worker fix (b0785383, 7c643a0a, b8a2516e) landed mid-plan. The three plan commits were rebased onto it with no conflicts, and the plan base moved to b8a2516e (`/tmp/p11-09-base`).
   - One full vitest run had already finished on the pre-rebase code (103 files, 1500 tests passed, `/tmp/p11-09-premerge-vitest.log`). The gate run above is the one on the final code, so vitest ran twice in full. The Go suite ran once.
   - No 11-09 test renders the real MarkdownPreview (PreviewPanel.test stubs the renderers, and App.preview uses PNGs), so no Worker stub was needed.
2. **[Pin] App.nav.test.tsx updated** (not in files_modified) to follow the sidebar's `sessionTabId`. See the pin table.
3. **[Pin] App.open-remote slices** now end at the branch end instead of a fixed 3500 chars. See the pin table.
4. **Commit ledger:** the plan base is recorded in `/tmp/p11-09-base` rather than inside `.git`, as in 11-04, 11-05 and 11-08.
5. **[TDD note]** Task 3's PreviewPanel tests were written first. 5 of them failed (receiving delay and captions, too large, couldn't receive, hostile name) before the panes were built (`/tmp/p11-09-t3-red.log`).

## Known limits (not stubs)

- If the same file name arrives twice in a row, the live region text doesn't change, so the second arrival may not be announced again.
- A refresh whose file type changed keeps the old tab icon. The key is the path, so this is rare.
- 999,500 to 999,999 bytes shows as "1000 KB" (UI-SPEC's whole-KB rounding).
- Accepted by UI-SPEC: an inactive peer tab with no active preview is not connected and misses previews.

## Known Stubs

None.

## Threat Flags

None. No new endpoint, file access or HTML sink. T-11-33 (names only as React text; hostile-name test; the grep gate finds no HTML strings in App or TabBar), T-11-34 (focus rule tests), T-11-35 (keep-mounted tests) and T-11-36 (limit, close and unmount revoke tests) are mitigated as planned.

## Self-Check: PASSED

- FOUND: PreviewPanel.tsx, App.preview.test.tsx, PreviewPanel.test.tsx
- FOUND: 348db6b9, 4d1b17f7, 20acccf5 (ancestors of HEAD)
