---
created: 2026-11-17T23:30:00Z
title: Opening a remote session twice makes duplicate tabs; closing one blanks the window
area: frontend
severity: major
files:
  - frontend/src/App.tsx
  - frontend/src/lib/tabs.ts
  - frontend/src/main.tsx
  - frontend/src/components/WebShareSessionView.tsx
---

## Problem

User report (2026-11-17): after a network change, clicking the remote session in the sidebar opened it in two tabs; closing one blanked the whole window and the app had to be force-quit. User confirmed the sidebar vanished too (entire React root gone, not just the terminal pane) — consistent with an uncaught render/effect error unmounting the root.

Read-only diagnosis (not yet reproduced):
- Duplicate tabs (high confidence): `handleOpenRemoteSession` (App.tsx:781-804) checks the click-time `tabs` snapshot, then awaits a slow `ConnectPeerSession` (:791). A second click passes the same check; `openWebSessionTab` (:715) also checks a stale snapshot and both append `__websession__<sid>` at :721.
- Blank window (medium-high): `removeTabState` (:508-511) removes both copies by id; `nextActiveTabId` (lib/tabs.ts:12-14) returns the other copy, i.e. the removed id. At App.tsx:1005-1009 the missing `activeWebTab` falls back to `window.location.origin` (`wails://wails`), so `new WebSocket("wails://…")` (relayClient.ts:191) throws a SyntaxError. No ErrorBoundary exists, so React unmounts the root.

## Solution

1. `openWebSessionTab`: dedupe inside the functional `setTabs(prev => …)` update.
2. In-flight guard (`useRef<Set<string>>`) in `handleOpenRemoteSession`.
3. `nextActiveTabId` never returns the closing id.
4. App.tsx:1005: render nothing when `activeWebTab` is missing outside web mode.
5. Top-level ErrorBoundary in main.tsx.

Add regression tests (double-click race, close with duplicate ids). Do after Phase 5 plan 05-05 lands (it edits App.tsx and WebShareSessionView.tsx).
