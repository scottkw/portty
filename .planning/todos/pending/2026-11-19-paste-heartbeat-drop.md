---
created: 2026-11-19T21:00:00Z
title: Keep a viewer connected while a long paste drains into a slow program
area: relay
severity: minor
source: .planning/phases/08-debt-cleanup/08-REVIEW.md IN-05 (deferred in 08-REVIEW-FIX.md)
files:
  - internal/webserver/server.go
  - internal/relay/heartbeat.go
  - internal/daemon/remote_ws_proxy.go
  - frontend/src/lib/relayClient.ts
---

## Problem

Pre-existing (not Phase 8). The webserver's read pump calls `hub.WriteInput` synchronously (`internal/webserver/server.go`, MsgInput case). When the PTY input queue is full (a multi-MB paste into a program that reads slowly and does not echo), the pump stops reading the socket, and three separate liveness checks then drop a healthy viewer mid-paste:

1. The server's `relay.Heartbeat`: coder/websocket reads the client's pong only inside `Read`, so the ping times out after `PingTimeout` (15 s) and `CloseNow` drops the viewer.
2. The GUI peer path's daemon proxy (`remote_ws_proxy.go`): its client-to-upstream copy blocks on TCP backpressure while holding coder/websocket's frame write lock, so its own upstream `Ping` cannot even be written and times out.
3. The browser's `relayClient` watchdog: after `deadMs` (40 s) with no output it sends a MSG_PING probe, which sits unread behind the paste, and abandons the socket after `probeGraceMs`.

DEBT-05 then shows "Paste may have been cut short", which is honest but self-inflicted.

## Why not fixed in 08-REVIEW-FIX

A heartbeat-only fix (forgive a ping timeout while the pump is mid-`WriteInput`) covers path 1 only. The fix that covers all three is to keep the read pump reading: a bounded per-connection input queue (for example 1,024 frames, 32 MiB) drained by a writer goroutine that outlives the socket, so nothing already read is lost. That changes the input path's lifetime and DEBT-05's ack meaning (a pong would then mean "read by the server", not "handed to the PTY"), and needs the M-101 VM run to confirm. Too large for a review-fix iteration.

## Solution

1. In `internal/webserver/server.go`, queue filtered input to a writer goroutine (bounded; the pump blocks only when the queue is full) that drains to `hub.WriteInput` in order and exits once the queue is closed and empty.
2. Re-check the DEBT-05 ack (`relayClient.ts`, `ackPing`) against the new meaning and update its comments and tests.
3. Test: a fake hub whose writer blocks for 3 x the heartbeat timeout; the viewer must stay connected (short `heartbeatEvery`/`heartbeatTimeout`), every byte must arrive in order, and a dead peer must still be dropped once the backlog drains.
4. Re-run M-101 in the Tart VMs with a slow, non-echoing reader (for example `stty raw -echo; pv -L 50k > /tmp/f`).
