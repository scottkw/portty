---
created: 2026-11-19T22:00:00Z
title: Tell a browser share viewer when a split paste may have been cut short
area: web
severity: minor
source: .planning/phases/08-debt-cleanup/08-VERIFICATION.md follow-up 2 (v1.2)
files:
  - web/assets/terminal.js
  - frontend/src/lib/relayClient.ts
---

## Problem

The browser share viewer (`web/assets/terminal.js`) now splits a large paste into ordered frames of at most 32767 payload bytes (`makeInputFrames`), so a paste over 32 KiB no longer closes the socket with 1009. But if the socket drops mid-paste, the viewer says nothing: there is no DEBT-05 "Paste may have been cut short" notice like the GUI's `relayClient` gives. The viewer's only status surface is the status bar (`updateStatusBar`), and it does not track MsgPing/pong.

## Why not fixed with the split

The GUI notice depends on ping/pong ack accounting (`relayClient.ts` `paste`, `ackPing`, `pongsReceived`): after a split paste it sends a MSG_PING, and the pong acknowledges every frame before it. terminal.js has neither the ping sender nor pong handling, so the notice is not cheap to add. It also shares the ack path with IN-05/IN-06 (`2026-11-19-paste-heartbeat-drop.md`, `2026-11-19-paste-notice-false-positive.md`), and should be designed with them.

## Solution

1. In terminal.js, after a paste of 2+ frames, send `[MsgPing]` and remember the ping count; clear the pending paste when the matching pong arrives.
2. On `ws.onclose` with a pending paste, show "Paste may have been cut short" in the status bar (textContent only; no innerHTML, CSP unchanged).
3. Test in `frontend/src/lib/webTerminalInput.test.ts` or a jsdom harness with a fake WebSocket.
