---
created: 2026-11-17T23:55:00Z
title: Network-resilience test matrix and wake-triggered reconnect
area: relay
severity: major
files:
  - frontend/src/lib/relayClient.ts
  - cmd_attach.go
  - internal/daemon/remote_ws_proxy.go
---

## Problem

User requirement (2026-11-17): Portty must survive network glitches, switching networks, switching Wi-Fi APs, and sleep/wake (either Mac) with no manual reattach. Users often have no other way into the remote Mac.

Phase 5 covers this with liveness (30 s ping / 15 s timeout), retry forever with 1–30 s backoff, resume offsets, and re-auth. Gap: the plan decided "no NSWorkspace wake hook", so after the viewing Mac wakes, reconnect waits for the watchdog (up to ~45 s) unless the webview fires online/visibilitychange. A Wails WKWebView may not fire those on system wake.

## Solution

1. Add a wake/network-change trigger: Go side observes NSWorkspace didWake (and optionally the network-path change) and emits a Wails event; RelayClient and the CLI loop treat it as "reconnect now" (reset backoff, probe immediately). Small, reversible.
2. Test matrix, each scenario must end with typing working and screen intact, no duplicate output:
   - brief drop (seconds) — Tart VM: pause network / `pfctl` block
   - half-open (packets silently dropped) — peer stops reading
   - network switch / AP switch (IP change) — VM: change interface / toggle Wi-Fi
   - viewing Mac sleep/wake — `tart suspend`/resume, `pmset sleepnow`
   - host Mac sleep/wake (host side sleeping is out of scope; wake must recover)
   - home daemon restart, host daemon restart (host restart = session gone, must say so clearly)
   - long output during the gap (ring wrap → full replay)
3. Fold the automatable rows into Phase 5 verification; the two-Mac rows go on the manual checklist (M-84..M-87).
