---
created: 2026-11-17T23:55:00Z
title: Network-resilience test matrix and wake-triggered reconnect
area: relay
severity: major
files:
  - frontend/src/lib/relayClient.ts
  - cmd_attach.go
  - internal/daemon/remote_ws_proxy.go
completed: 2026-11-18
status: completed
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

## Resolution

Fixed in 05-10, 05-13 and 05-16, measured in 05-14 and 05-16; closed by 05-15.

1. Wake/network trigger: a pure-Go detector `internal/wakewatch` (a wall-clock tick gap or a changed interface address list) instead of an NSWorkspace observer, user approved. The CLI loop uses it in 05-10 (`6ef013cf`, `934db371`, `020d16a5`, `4aa0042a`): a wake skips a pending backoff or probes an open socket. 05-13 (`67149bc5`, `17021753`, `9fd0a2fd`, `8af5c8b0`) emits it to the GUI as `system:wake`, re-dispatched as `portty:wake` for RelayClient; browser views treat a late watchdog tick as a wake. Tests: `internal/wakewatch/wakewatch_test.go`, `internal/attach/loop_test.go` (`TestRun_Wake*`), `app_wake_test.go`, `frontend/src/lib/relayClient.test.ts`, `frontend/src/components/__tests__/App.remote-poll.test.tsx`.
2. Matrix: 05-14 (`764e92ef`, `69735aab`) ran R1-R9 on two Tart VMs over the real tailnet; R6 (viewing-Mac daemon restart) failed and was fixed in 05-16 (`bf628725`, `c2c99648`, `1e0c33c3`, re-run `e0ad1234`): the GUI tray poller and the `portty attach` re-auth start a dead local daemon. Final result 9 PASS / 0 FAIL in `.planning/phases/05-overnight-survival/05-VM-MATRIX.md`; long output during the gap is also pinned by `TestCLIAttach_LongOutputDuringGap` (05-10, `9846f32a`). The matrix is repeatable as TESTING.md M-90.
3. Left to the user: a real Wi-Fi access-point switch and lid sleep on two physical Macs (TESTING.md M-85; R3 covered an interface flap and address changes only), the overnight run (M-84), and the browser view (M-86).
