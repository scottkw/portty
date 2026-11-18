---
created: 2026-11-18T04:00:00Z
title: UI polish, four small upgrades on the current design
area: frontend
severity: minor
files:
  - frontend/src/style.css
  - frontend/src/components/Sidebar.tsx
  - frontend/src/components/TabBar.tsx
---

## Problem

The user finds the app plain. Sketch 001 (`.planning/sketches/001-app-polish/`) offered three looks. The user chose to keep the current native design and add four upgrades (2026-11-17, "do the four").

## Solution

1. Sidebar icons in the system accent color, not gray (see variant A).
2. Each peer's status dot turns amber while one of its tabs is reconnecting, and returns to green once reconnected (ties into the 05-04 reconnect state).
3. Remote tabs show a globe icon and a dimmed host name (`shell 1 · kens-macbook-air-1574`), truncated cleanly.
4. One stable color per Mac (derived from the peer ID or hostname, chosen from a small macOS-friendly palette with a light and a dark value), shown only as a small dot on that Mac's sidebar rows and tabs (see variant C).

Constraints: CSS tokens only, light and dark, no new settings. Check in the Tart VM with axdump plus a careful visual pass, and add vitest coverage for the color derivation and the amber dot state. Do this after Phase 5, because 05-12 and 05-13 touch TabBar and TerminalPanel.

## Resolution

Done 2026-11-18 (commits cfaa70be, 737eb1f7, a5104d08, d7e76093, 1c32da40).

1. Sidebar row icons (`.sidebar__row-icon`) use `--color-accent` (#0063CC light, #4DA3FF dark) and turn white on the selected pill. Group header icons stay secondary. The accent is the app's fixed token, the same one the selection fill uses, so it does not follow a custom accent chosen in System Settings.
2. `TerminalPanel` reports its reconnecting-banner state through `onReconnectingChange` (true when the banner shows, false when it hides or the panel unmounts). `WebShareSessionView` passes it through, App keeps the one reconnecting peer host, and the Sidebar dot gets `.sidebar__dot--warn` (new token `--color-status-warn`, #FF9F0A in both schemes) with the accessible name "Reconnecting". No new state machine. The banner text carries the state in words. The VM check found that the daemon's discovery lists only peers that answer its probe, so a blocked Mac's row disappeared. `peerView` now keeps the reconnecting host listed as unreachable until it reconnects.
3. Peer tabs show an aria-hidden `GlobeAltIcon` and "session · host", with the host in `--color-text-secondary` (`peerTabParts` in tabs.ts splits the existing `<host> › <session>` name). The tooltip and the close button's name use the same label. The ellipsis trims the host first, and the globe and the dot keep their size.
4. `peerColorIndex` (FNV-1a of the lower-cased full hostname, mod 5) picks one of `--color-peer-0..4`: teal #30B0C7/#40C8E0, indigo #5856D6/#5E5CE6, purple #AF52DE/#BF5AF2, pink #FF2D55/#FF375F, brown #A2845E/#AC8E68 (light/dark). The palette has no green, orange or grey, so a Mac dot never looks like a status dot. It shows as a 6px aria-hidden `PeerDot` at the end of the peer's sidebar session rows and of its tabs, with a white ring on the selected row. This Mac gets no colour, since the colour comes from the peer hostname.

Tests: sidebarPeers.test.ts, Sidebar.test.tsx, TabBar.test.tsx, TerminalPanel.reconnect.test.tsx and App.open-remote.test.tsx, all extended (no new files). VM check: TESTING.md M-91.
