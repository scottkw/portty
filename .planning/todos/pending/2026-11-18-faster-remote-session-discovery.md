---
created: 2026-11-18T03:00:00Z
title: Newly shared sessions take up to 30 s to appear on other Macs
area: frontend
severity: minor
files:
  - frontend/src/App.tsx
---

## Problem

User report (2026-11-17): after sharing a session, it takes a while to show up in other clients' sidebars. Cause: App.tsx polls `GetRemoteSessionsWithMeta` (fan-out to every tailnet peer) every 30 s (`setInterval(() => void refresh(), 30_000)`, ~App.tsx:637), with no refresh on window focus. Worst case is 30 s.

## Solution

- Poll every ~5 s while the window is visible/focused; back off to 30 s when hidden (document.visibilityState / window blur).
- Refresh immediately on window focus / visibilitychange (the same trigger the Tailscale recheck already uses nearby) and on wake (05-13's wake event).
- Keep fan-out cost in mind: one LocalAPI status + one HTTPS GET per online peer per poll; skip a tick if the previous refresh is still in flight.
- Later option: push (host notifies peers on share/unshare) if polling is still too slow.
- Test: fake timers — visible → 5 s cadence, hidden → 30 s, focus → immediate refresh, no overlapping refreshes.
