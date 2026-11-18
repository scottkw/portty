---
created: 2026-11-18T10:00:00Z
title: Switching remote tabs disconnects the inactive one and costs a full replay
area: frontend
severity: minor
files:
  - frontend/src/App.tsx
  - frontend/src/components/TerminalPanel.tsx
audit_acknowledged:
  milestone: v1.0
  at: 2026-11-18
---

## Problem
Review IN-04, seen on VMs (post-review regression): opening a second remote tab closes the first tab's connection; switching back rejoins with `resume=false` (full replay) instead of delta. Works, but slower and redraws everything.

## Solution
Either keep inactive peer panels mounted (hidden) or keep the per-tab resume offset across unmount so the rejoin is a delta. Weigh memory/socket cost of keeping N sockets open.
