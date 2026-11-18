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
