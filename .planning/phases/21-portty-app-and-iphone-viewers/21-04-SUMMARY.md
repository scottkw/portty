---
phase: 21-portty-app-and-iphone-viewers
plan: 04
subsystem: ui
tags: [react, sidebar, tabbar, desktop-viewer, vitest]

requires:
  - phase: 19
    provides: the /desktop/ viewer whose state words the muted lines reuse
provides:
  - RemotePeerSessions.desktop flag, DesktopRowState and DESKTOP_ROW_LINES, desktopConnectHost
  - Sidebar Desktop row (first in a reachable peer's list), muted state line, aria-current marking
  - desktopTabId and the 'desktop' Tab type with its tab-bar chrome
affects: [21-07, 21-08, 21-13]

status: complete
commits: 3
plan_head_before: 6662f19644b8914f539b0952f62f526d6d7e3711
plan_head_after: ab4c61679fcf63874cca8368158b08556360f900
actuals:
  tokens: 9000
  tasks: 2
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Desktop tab id is 'desktop:' + connect host (MagicDNS name, else hostname): one tab per peer"
    - "Row renders only when App passes onOpenDesktop, so no dead affordance exists before 21-08"

key-files:
  created: []
  modified:
    - frontend/src/lib/remoteSession.ts
    - frontend/src/lib/sidebarPeers.ts
    - frontend/src/lib/tabs.ts
    - frontend/src/components/Sidebar.tsx
    - frontend/src/components/TabBar.tsx
    - frontend/src/components/__tests__/Sidebar.test.tsx
    - frontend/src/components/__tests__/TabBar.test.tsx
    - frontend/src/lib/__tests__/sidebarPeers.test.ts
    - frontend/src/lib/__tests__/tabs.test.ts

key-decisions:
  - "Desktop row hidden unless reachable, peer.desktop === true and onOpenDesktop is passed"
  - "Muted rows stay working buttons; the line, not the colour, carries the state"

requirements-completed: [DVWR-01]
---

# Phase 21 Plan 04: Sidebar Desktop row and desktop tab chrome Summary

Frontend-only entry point for DVWR-01: a reachable peer that announced its desktop gets a first-in-group Desktop row (with a muted state line and aria-current marking), and the tab bar renders a 'desktop' tab as "Desktop · host" with the desktop icon.

## Accomplishments

- `RemotePeerSessions.desktop?: boolean`, `DesktopRowState`, `DESKTOP_ROW_LINES` (five UI-SPEC lines), `desktopConnectHost`; the synthetic reconnecting peer carries desktop false.
- Sidebar: `onOpenDesktop` and `desktopStates` props; Desktop row first in the nested list; "Nothing shared" only without sessions and without a Desktop row; `data-muted`, `aria-describedby` and a `.sidebar__muted` line directly under a muted row; `aria-current="page"` only for the active desktop tab of the same connect host.
- tabs.ts: `desktopTabId`; `peerTabParts` splits desktop tab names; `peerTabsRemoved` unchanged and still web-session only (tested).
- TabBar: `'desktop'` in the type union, ComputerDesktopIcon in place of the globe; label, tooltip, Close name, Close-only menu and no-rename follow the existing peer path.
- Verification: `tsc --noEmit` clean, full vitest 126 files / 1982 tests pass, `bash tests/check-brand.sh` clean.

## Task Commits

1. Task 1 (tracer): `39a79612` feat - data field, row, tab id, tab chrome
2. Task 2 RED: `8f23ee8f` test - failing tests (6 failed as expected)
3. Task 2 GREEN: `ab4c6167` feat - muted line, data-muted, aria-describedby, aria-current

## Deviations from Plan

None to the code. Environment notes:

- The worktree had no `node_modules`; a symlink to the main checkout's `frontend/node_modules` was created (untracked, not committed). `pnpm exec` tries to reinstall through that symlink and fails ("workspace hoist directory is not a real directory"), so tsc and vitest were run via `./node_modules/.bin/` directly instead of `pnpm --dir frontend exec`.
- The commit trailer names Claude Sonnet 5.5 (the actual model), per the dispatch instruction, rather than the plan's "Claude Opus 5.5".

## Known Gaps (not stubs)

- Host-only ellipsis in the tab: the existing `.tab__name` rule ellipsises the whole "Desktop · host" label (CSS comment says the host trims first only because the host is last). No style.css edit was allowed in this plan, so no dedicated host-only rule was added; the sidebar Desktop row has a fixed "Desktop" name and the host is shown by the group header, which already ellipsises. If a distinct host-only truncation is wanted, it belongs with 21-08's style.css work.
- The row is inert until 21-08 passes `onOpenDesktop` and `desktopStates` from App, and until 21-07 fills `desktop` from the meta header (by design, per the plan).

## Tests added (for 21-13's TESTING.md update; TESTING.md not edited here)

No new test files; cases were added to existing files already in the suite: `frontend/src/components/__tests__/Sidebar.test.tsx`, `frontend/src/components/__tests__/TabBar.test.tsx`, `frontend/src/lib/__tests__/sidebarPeers.test.ts`, `frontend/src/lib/__tests__/tabs.test.ts`.

## Threat Flags

None. Host names render as React text (T-21-10); the row requires reachable plus desktop true, with every negative case tested (T-21-11); no package added.

## Self-Check: PASSED

All nine modified files exist; commits 39a79612, 8f23ee8f and ab4c6167 are ancestors of HEAD.
