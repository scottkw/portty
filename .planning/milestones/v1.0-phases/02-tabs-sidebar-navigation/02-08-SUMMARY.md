---
phase: 02-tabs-sidebar-navigation
plan: 08
subsystem: frontend-toolbar-tabs
status: complete
tags: [term-03, look-01, net-01, toolbar, tabs, context-menu, rename, end-session, share, testing]
requires: ["02-07"]
provides:
  - "Closing a tab (x, Cmd+W, Close Tab item) only detaches: no session RPC; right neighbour, else left, else the empty state"
  - "End Session... confirm (destructive, Cancel focused) stops web serving if shared, then KillSession"
  - "In-place rename from the tab or the sidebar row; tab labels derive from the session list; reverts on RPC failure"
  - "One NSMenu-style ContextMenu for tabs and sidebar rows (right-click, Shift+F10, arrows, Enter, Esc, focus return)"
  - "52px unified toolbar: pill tabs in a tablist, New Session, Share / Shared / aria-disabled, 'On <host>' chip, chevron scroll buttons"
  - "No bottom status bar; TailscaleBanner sentence carries id=ts-off-reason"
affects: ["02-09", "02-10", "02-11", "02-12"]
actuals:
  tokens: 47700
  tasks: 3
  commits: 4
commits: 4
plan_head_before: b2510868d9e3a23fda3bca278f7cd4bc15676592
plan_head_after: 681891a5e518023db7d08f647f94e1c1ffa32c20
tech-stack:
  added: []
  patterns:
    - "Context menus are portaled to <body> so no drag region, overflow clip or containing block of the invoker applies"
    - "Single source of truth for names: TabBar gets tabs mapped through tabLabel(t, sessions); rename edits only the session list"
    - "Share state lives on SessionInfo.webEnabled; setSessionWebEnabled writes a toggle into the list before the next poll"
    - "Disabled-with-reason control = a separate branch with aria-disabled + aria-describedby and no handler"
key-files:
  created:
    - frontend/src/lib/tabs.ts
    - frontend/src/lib/__tests__/tabs.test.ts
    - frontend/src/components/ContextMenu.tsx
    - frontend/src/components/__tests__/ContextMenu.test.tsx
    - frontend/src/components/EmptyState.tsx
    - frontend/src/components/__tests__/EmptyState.test.tsx
  modified:
    - frontend/src/App.tsx
    - frontend/src/components/TabBar.tsx
    - frontend/src/components/Sidebar.tsx
    - frontend/src/components/InlineSessionName.tsx
    - frontend/src/components/RegenerateKeyModal.tsx
    - frontend/src/components/TailscaleBanner.tsx
    - frontend/src/components/Share/SessionShareModal.tsx (comment only)
    - frontend/src/style.css
    - frontend/e2e/web-share-scope.spec.ts
    - TESTING.md
    - "tests: TabBar, Sidebar, InlineSessionName, RegenerateKeyModal, TailscaleBanner, App, App.shellExit, App.tailscale, App.open-remote, App.saver, style.redesign, style.contrast, style.tokens"
  deleted:
    - frontend/src/components/StatusBar.tsx
    - frontend/src/components/__tests__/StatusBar.shareSession.test.tsx
key-decisions:
  - "The webEnabled map was removed. Its readers (close path, StatusBar) are gone; share state is read from sessions, and the Share dialog's onShareEnabledChange plus the shell-warning confirm write into sessions via setSessionWebEnabled for instant feedback."
  - "removeTabState uses a functional setActiveId((cur) => nextActiveTabId(prev, id, cur)) with [] deps, so it never reads a stale activeId."
  - "endSession throws KillSession errors so the confirm dialog shows them inline; a ToggleWebServing(false) failure is logged and the kill still runs. It also drops the session from the list at once (the exit watcher and the 3 s poll agree)."
  - "Save Terminal As... shows only while the serialize plugin is on: App passes onRequestSave only when pluginConfig.serialize is true (UI-SPEC), replacing the old always-show-then-banner behaviour."
  - "The disabled Share button has no click or key handler at all, which satisfies 'click and Enter do nothing' without early-return code."
  - "The empty state is gated on relayPort !== null (set in the same batch as the restored tabs) so launch never flashes 'No Open Sessions'."
  - "isMenuKey lives in ContextMenu.tsx; the Settings tab (until 02-11) gets only Close Tab, like a peer tab."
requirements-completed: [TERM-03, LOOK-01]
---

# Phase 2 Plan 08: Toolbar, Tabs, Detach and End Session Summary

**Closing a tab now only detaches (the shell stays in the sidebar), a confirmed End Session... kills it, rename works in place on the tab and the sidebar row from one session list, and Share moved from the deleted status bar into a 52px pill-tab toolbar. While Tailscale is off, Share uses aria-disabled with aria-describedby pointing at the banner sentence, so the reason is visible on screen.**

## Performance

- Started: 2026-11-15T21:20:25Z
- Completed: 2026-11-15T21:45:23Z (about 25 min)
- Tasks: 3 of 3
- Files: 31 changed (+1591 / -1084)

## Task Commits

1. **Task 1 (tracer): close = detach, pill toolbar, empty state, Cmd+W**: `08c4d227`
2. **Task 2: End Session confirm, in-place rename, shared context menu**: `fbf37cc0`
3. **Task 3: toolbar Share with accessible NET-01 state, status bar deleted, TESTING.md**: `681891a5`

The measured count is 4 (`b2510868..681891a5`). One of the four, `2a09cc55` ("docs(02): add 02-13 appearance override"), landed on the branch from outside this plan between Task 1 and Task 2. This plan made three commits.

Tracer gate (Task 1): auto mode is off and human_verify_mode is end-of-phase, so I re-ran Task 1's automated `<verify>` (tsc, the five targeted vitest files, then the full suite). It passed before Task 2 started.

## Verification

- `pnpm exec tsc --noEmit`: clean
- `pnpm test`: 91 files, 1224 tests passed
- `pnpm exec playwright test --project=chromium`: 7 passed, 9 skipped. The skips are fixture-gated specs and were already skipped before this plan. `web-share-scope.spec.ts` passes for owner and viewer caps.
- `go build ./... && go vet ./... && go test -race -short ./...`: exit 0
- `bash tests/check-traceability-paths.sh`: OK, 86 paths
- `wails build -platform darwin/arm64 -tags wailsassets`: exit 0, built `build/bin/agenthub.app`. The generated wailsjs tree was restored afterwards.
- Acceptance greps for all three tasks return the expected counts. Task 3 checks: `aria-describedby="ts-off-reason"` 1, `id="ts-off-reason"` 1, `'⚠'` 0, `&#8249;|&#8250;` 0, `**M-63**` 1, TERM-03 rows 6.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] endSession added in Task 2, not kept unused in Task 1**
- Issue: tsconfig has `noUnusedLocals`, so keeping the old kill body as an uncalled `endSession` in Task 1 would fail tsc.
- Fix: Task 1 removed the kill-on-close handler, and Task 2 added `endSession` together with its caller.
- Commit: 08c4d227 / fbf37cc0

**2. [Rule 1 - Bug] Scroll buttons never appeared when tabs were added**
- Issue: the ResizeObserver watches the strip's own box. Adding a tab changes `scrollWidth` but not the box, so overflow was never detected. This would break the "many tabs scroll" truth.
- Fix: `useEffect(checkScroll, [tabs.length])` in TabBar.
- Commit: 08c4d227

**3. [Rule 1 - Bug] Rename could commit twice, or commit after Esc**
- Issue: unmounting a focused input can fire blur after Enter or Esc. InlineSessionName's blur handler then committed again, or committed the edited value after a cancel.
- Fix: a one-shot `doneRef` guard per edit, which a test covers.
- Commit: fbf37cc0

**4. [Rule 1 - Bug] Closing a peer tab called KillSession on a `__websession__` id**
- Issue: the old close handler ran KillSession on every tab type. For peer tabs that call always failed.
- Fix: the detach-only closeTab covers this case too.
- Commit: 08c4d227

**5. [Rule 3 - Blocking] Tests and specs outside the plan's file list**
- `style.redesign.test.ts`, `style.contrast.test.ts` and `style.tokens.test.ts` pinned `.tab-status-bar*` rules. Those fences were retired, and the new toolbar, menu and empty-state selectors joined the no-hex gate.
- `App.open-remote.test.tsx` needed the new required Sidebar props.
- `App.saver.test.tsx` pinned `onRequestSave={handleRequestSave}`, which is now gated on the serialize plugin.
- `e2e/web-share-scope.spec.ts` now finds the rename input by `input[aria-label="Session name"]`.
- Commits: fbf37cc0, 681891a5

**6. [Rule 1 - Cleanup] Orphaned tokens**
- `--hub-internet-badge-text` and `--hub-fullaccess-badge-text` lost their last consumers when the tab icons moved to `--color-text-secondary` and `--color-error-text` per UI-SPEC. The two `-bg` siblings already had no consumers. All four were removed, following the 02-07 precedent.
- Commit: 681891a5

Plan-level choice: TabBar gets labels by App mapping `tabs` through `tabLabel` instead of through a new `labels` prop. The result is the same single source of truth without adding a prop.

## Known Stubs

None.

## Threat Flags

None. No new network, auth or file surface. T-02-26..29 are mitigated as planned:
- closeTab makes no RPC, asserted by the App.test region check.
- End Session stops sharing before the kill, its copy warns attached viewers, and Cancel has focus first.
- Rename input is trimmed, and empty or unchanged values make no RPC.
- The disabled Share button has no handler, and the dialog still closes when Tailscale disconnects.

## Needs VM check (native GUI, built app at build/bin/agenthub.app)

Run M-63 from TESTING.md in Light and Dark. Quit any installed AgentHub and stop its daemon first.
1. Three tabs: close the middle one with its ×. The sidebar row stays and the right neighbour activates. Press ⌘W down to zero tabs: you get the empty state, and "No Open Sessions", its New Session button and ⌘T are all visible on the 720pt screen. ⌘W with no tabs does nothing. Reopening from the sidebar keeps the history.
2. End Session... on a web-shared session (tab menu and sidebar row menu). The title reads End “name”?, the extra "Anyone attached..." line shows, and Cancel has focus. Confirming removes the tab and the row, and the link stops working.
3. Rename by double-clicking the tab name, by double-clicking the sidebar row name, and from the Rename item. Each new name shows in both places at once. Esc cancels and an empty value reverts. The input stays at the same height.
4. With 15 tabs, the pills shrink to 88px and the chevron scroll buttons appear. New Session and Share never scroll away. A single tab is one left-aligned pill no wider than 220px. A long name truncates with an ellipsis and never overlaps the close button.
5. Share opens the dialog, and the Funnel risk panel is reachable. With sharing on, the button reads "Shared" on the accent fill. A peer tab shows the "On host" chip. With Tailscale off, the button dims and a click does nothing, while the banner sentence stays visible. VoiceOver on the focused button reads "Share, dimmed" plus the banner sentence.
6. With the sidebar hidden, the first tab and the Show Sidebar button clear the traffic lights (80px inset). Dragging the toolbar's empty space moves the window, while tabs, ×, +, Share and the context menu still take clicks.
7. The right-click menu looks like NSMenu: it highlights in accent on hover and on arrow keys, it flips at the window edges, and Esc returns focus to the tab.

Human-check result: not run here. It is deferred to the orchestrator's VM pass, per the sequential_execution instructions.

## Self-Check: PASSED

- FOUND: frontend/src/lib/tabs.ts, frontend/src/components/ContextMenu.tsx, frontend/src/components/EmptyState.tsx, and their three test files
- GONE: frontend/src/components/StatusBar.tsx, frontend/src/components/__tests__/StatusBar.shareSession.test.tsx
- FOUND commits on HEAD: 08c4d227, fbf37cc0, 681891a5
