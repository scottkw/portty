---
resolves_phase: 7
created: 2026-11-18T10:00:00Z
title: GUI logs a respawn attempt at launch while the daemon is healthy
area: daemon
severity: minor
files:
  - app.go
audit_acknowledged:
  milestone: v1.0
  at: 2026-11-18
---

## Problem
Iteration-3 VM spot check (05-VM-MATRIX.md): every captured GUI launch logged `portty: daemon unreachable, respawn attempt 1: ok` immediately, with a healthy daemon. The attempt counts toward the 3-attempt `daemon:error` streak for the first minute, so two real crashes soon after launch would raise the error one attempt early.

## Solution
Find why the first poller tick sees the daemon unreachable (tick before startup completes? health check timing?) and skip respawn until the first healthy observation. Add a seam test.
