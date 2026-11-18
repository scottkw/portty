---
created: 2026-11-17T23:59:00Z
title: Cap the vt library's alt-screen scrollback
area: relay
severity: minor
files:
  - internal/relay/hub.go
---

## Problem

charmbracelet/x/vt gives every screen a 10,000-line scrollback, and `SetScrollbackSize` only reaches the main screen. A TUI that scrolls inside the alternate screen keeps growing the daemon heap (measured 126.6 MiB for 20,000 lines) until a resize rebuilds the emulator. Pre-existing; 05-03 bounded and drains the main-screen scrollback only (05-RESEARCH.md Pitfall 3). Phase 5 deferred the heap soak test (05-CONTEXT.md).

## Solution

Either rebuild the live emulator when the app leaves the alternate screen (`AltScreen(false)`), after draining main-screen history into `Hub.history`, or get an upstream fix that applies the scrollback size to both screens. Add a heap soak test with the fix (alt-screen scrolling of 20,000+ lines must stay under a fixed heap bound, like `TestHistory_HeapBoundAfterBurst`).
