---
resolves_phase: 7
created: 2026-11-17T20:32:19Z
title: Split pastes over 32 KiB into multiple input frames
area: frontend
severity: minor
files:
  - frontend/src/lib/relayClient.ts
audit_acknowledged:
  milestone: v1.0
  at: 2026-11-18
---

## Problem

Phase 4 review WR-04 and 04-10 planner note: a paste over 32 KiB goes out as one input frame (relayClient.ts encodeInputFrame) and is dropped by every input reader (local relay, proxy client side, share server), which keep the 32 KiB limit as the DoS guard. The CLI's 32 KiB stdin buffer plus the 1-byte header makes a 32769-byte frame, the same off-by-one. Pre-existing code.

## Solution

Split large input in the frontend (and cap the CLI read at 32767 bytes) so the input-side limit stays. Fix the remote_ws_proxy.go:66-67 comment that calls this harmless.
