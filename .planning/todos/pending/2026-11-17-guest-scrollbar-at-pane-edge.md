---
created: 2026-11-17T23:00:00Z
title: Put the remote (guest) session scrollbar at the pane edge, not mid-screen
area: frontend
severity: minor
files:
  - frontend/src/components/TerminalPanel.tsx
  - frontend/src/lib/terminalScale.ts
---

## Problem

Reported by the user with a screenshot: on a remote (peer) tab, the xterm scrollbar sits in the middle of the terminal area instead of at the right edge. A guest renders at the host's fixed grid (cols × rows), and `computeGuestScale` (terminalScale.ts:12) caps the scale at 1. When the pane is wider than the host grid, the xterm element is narrower than the pane, so its viewport scrollbar ends up wherever the grid ends.

## Solution

Keep the host grid (a guest must not resize the host PTY). Options:
- Stretch the xterm viewport/scroll container to the full pane width while the text grid stays at host cols, so the scrollbar is at the pane edge, or
- Render a pane-level scrollbar for guests and hide xterm's own.

Verify in the Tart VM (portty-test) with a host grid narrower than the pane. Do this after Phase 5 plan 05-04 lands, because that plan also edits TerminalPanel.tsx.
