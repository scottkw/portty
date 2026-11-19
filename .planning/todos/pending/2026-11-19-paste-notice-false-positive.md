---
created: 2026-11-19T22:00:00Z
title: Stop the paste notice firing for a paste that fully arrived
area: relay
severity: minor
source: .planning/phases/08-debt-cleanup/08-REVIEW.md IN-06 (deferred in 08-REVIEW-FIX.md)
files:
  - frontend/src/lib/relayClient.ts
  - internal/webserver/server.go
---

## Problem

The server answers a ping with a non-blocking send and drops the pong when `sub.Msgs` is full (`internal/webserver/server.go`, ping case). A large paste into a shell that echoes fills that queue, so the DEBT-05 ack (`relayClient.ts`, `ackPing`) waits for the next 15 s heartbeat pong. A socket loss in that window, or the hub's "too slow" kick triggered by the paste's own echo, shows "Paste may have been cut short" for a complete paste.

The notice is conservative: it never claims "complete" falsely, and its copy says "may".

## Why not fixed in 08-REVIEW-FIX

The review says no fix is required. The suggested refinement, a blocking pong with a timeout, touches the same ack path as IN-05 (`2026-11-19-paste-heartbeat-drop.md`), whose input-queue redesign changes what the pong means. It should be designed with IN-05.

## Solution

1. Design together with IN-05. In `internal/webserver/server.go`, send the pong blocking with a short timeout, or count the pong as owed and resend it on the next drain.
2. Re-check `relayClient.ts` `ackPing` handling and its tests against the new pong semantics.
3. Test: a fake hub whose `sub.Msgs` is full when the ping arrives; the client must still clear the pending paste without a heartbeat-length wait.
