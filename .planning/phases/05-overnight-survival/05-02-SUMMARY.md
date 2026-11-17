---
phase: 05-overnight-survival
plan: 02
subsystem: relay
status: complete
tags: [relay, websocket, liveness, heartbeat, shar-07]
requires:
  - "05-01: Hub.Join, the replay/offset meta sequence"
provides:
  - "relay.MsgPong (0x04) and relay.PongFrame: servers answer every MsgPing"
  - "relay.Heartbeat(ctx, conn, every, timeout) bool, relay.PingEvery (30s), relay.PingTimeout (15s)"
  - "Share server reaps a viewer that stops answering WebSocket pings"
  - "Viewing daemon's proxy drops a silent upstream with no close frame (client sees -1, reconnects)"
  - "(*daemon.API).SetHeartbeatForTest; unexported WebServer.heartbeatEvery / heartbeatTimeout"
affects: [05-04, 05-06, 05-07]
tech-stack:
  added: []
  patterns:
    - "Heartbeat starts only after a goroutine is reading the conn (coder/websocket answers pings inside Read)"
    - "Pong queued with a non-blocking select on sub.Msgs; a full queue drops it"
    - "Per-instance interval overrides (zero means relay defaults), no package globals"
key-files:
  created:
    - internal/relay/heartbeat.go
    - internal/relay/heartbeat_test.go
    - internal/webserver/liveness_test.go
  modified:
    - internal/relay/protocol.go
    - internal/relay/protocol_test.go
    - internal/relay/server.go
    - internal/relay/server_test.go
    - internal/webserver/server.go
    - internal/daemon/remote_ws_proxy.go
    - internal/daemon/api.go
    - internal/daemon/remote_ws_proxy_test.go
decisions:
  - "Heartbeat returns false (no reap log) when a ping fails because the handler's ctx ended, so a normal handler exit is not logged as a heartbeat timeout"
  - "RED commits carry declaration-only scaffolds (MsgPong, PongFrame, interval fields, SetHeartbeatForTest) so the RED tests compile and fail on their assertions, not on build"
metrics:
  duration: 12min
  completed: 2026-11-17
  tasks: 3
  files: 11
estimate:
  tokens: 50000
  tasks: 3
actuals:
  tokens: 4700
  tasks: 3
  commits: 6
plan_head_before: f9adbbe47a26007134481b221af7eaaa402dc495
plan_head_after: 8dc18ade2a74c3f51a1f7fea3f11bba00c728352
---

# Phase 5 Plan 02: Connection Liveness Summary

Dead connections are now visible. The share server and the local relay answer the clients' app-level `MsgPing` (0x12) with a new `MsgPong` (0x04). The share server pings each viewer at the WebSocket level every 30 s and closes one that does not answer within 15 s. The viewing daemon's proxy does the same toward its upstream peer. When the upstream goes silent, the proxy closes both sides with no close frame, so the GUI tab or `portty attach` sees an abnormal close and reconnects.

## What was built

- **Wire (`protocol.go`):** `MsgPong byte = 0x04` and the shared `PongFrame = []byte{MsgPong}`. Old clients have no case for 0x04 and drop it.
- **`relay.Heartbeat` (`heartbeat.go`):** a ticker loop calling `c.Ping` with a `timeout`-bounded context. On a failed ping it calls `CloseNow` and returns true. It returns false when ctx is done, including when a ping fails because ctx ended. The doc comment states the two coder/websocket rules: start it only after a reader is running, and never put a deadline on the read context.
- **Share server (`handleWSSRelay`):** the `MsgPing` arm sends `PongFrame` on `sub.Msgs` without blocking. A heartbeat goroutine starts right after the read pump. On reap it logs `viewer-diag: heartbeat timeout session=<DiagID> conn=<n>`. The existing deferred `Unsubscribe` and viewer-count notify then remove the viewer.
- **Proxy (`handleRemoteSessionWS`):** a heartbeat on `upstream` starts after both `copyWS` goroutines (the peer-side copy is its reader). On reap it logs `viewer-diag: proxy heartbeat timeout session=<id8> host=<host>`, never the URL. `clientConn` (loopback) is not pinged.
- **Local relay (`handleSession`):** the same non-blocking pong. It has no heartbeat (Flagged decisions; the guard grep for `Heartbeat(` in `relay/server.go` is 0).

## Verification

- Task 1: build, vet, 4 targeted PASS lines, `go test -race ./internal/webserver` ok.
- Task 2: vet, 3 targeted PASS lines (`ForwardsClose` both subtests still pass); all `RemoteWSProxy|RemoteSessionWS` tests pass under -race.
- Task 3 / plan gate: `go build ./... && go vet ./... && go vet -tags playwrightfixture ./cmd/playwright-fixture/ && go vet -tags vmpreview ./...` exit 0. `go test -race -short -skip '^(TestAPIAppearance_GetPatch|TestAPITerminalFont_GetPatch)$' ./...` exit 0.
- Acceptance greps: `MsgPong *byte = 0x04` (1), `func Heartbeat(` (1), `relay.Heartbeat(` in webserver/server.go (1) and in daemon/remote_ws_proxy.go (1), `relay.PongFrame` (1), `PongFrame` in relay/server.go (1), `SetHeartbeatForTest(` (1), `Keep-alive — no-op` (0), `Heartbeat(` in relay/server.go (0).

## TDD Gate Compliance

| Task | RED commit | GREEN commit | RED cause (validated RED_EVIDENCE_OK) |
|------|-----------|--------------|-----------|
| 1 | 818a785f | 15341531 | TestWSS_PingGetsPong: no pong within 3 s; TestWSS_ReapsSilentViewer: "silent viewer was never reaped" |
| 2 | 754f8a96 | ad990e02 | TestRemoteWSProxy_SilentUpstreamClosesClient: "proxy never dropped the silent upstream within 2s" |
| 3 | 0e00cb0e | 8dc18ade | TestRelay_PingGetsPong: no pong within 5 s |

Semantic assessment: each target test compiled, ran, and failed on its planned assertion. These tests were green at their RED commits, as expected:
- `TestWSS_KeepsReadingViewer` and `TestRemoteWSProxy_LiveUpstreamStaysOpen` are guards: a reading peer must survive. With no heartbeat yet, they pass trivially.
- `TestMsgPongWireValue` pins a constant that the RED scaffold declared.
- `TestHeartbeat_*` pin `Heartbeat`, which Task 1 built, as the plan orders.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Declaration-only scaffolds in RED commits**
- **Found during:** Tasks 1 and 2
- **Issue:** The RED tests reference `MsgPong`, `PongFrame`, `WebServer.heartbeatEvery/heartbeatTimeout` and `API.SetHeartbeatForTest`. Without those, the package does not build, which tdd.md rejects as `build_failure` RED.
- **Fix:** The RED commits declare these symbols with no behavior. The GREEN commits add the behavior.
- **Commits:** 818a785f, 754f8a96

**2. [Rule 1 - Bug] Heartbeat does not report a reap when its own context ended**
- **Found during:** Task 1
- **Issue:** If the handler is exiting while a ping is in flight, the ping fails because the context was cancelled. That would log a false "heartbeat timeout".
- **Fix:** `Heartbeat` checks `ctx.Err()` after a failed ping and returns false.
- **Commit:** 15341531

## Known Stubs

None.

## Notes for later plans

- The client watchdogs (05-04, 05-06) can arm on the first `MsgPong`. The frontend should pin the literal `0x04`.
- There is a small window where a connection closes for another reason just before a heartbeat tick. The ping then fails and logs one spurious `heartbeat timeout` diag line. It does no harm; the conn is already ending.
- TESTING.md rows for `heartbeat_test.go` and `liveness_test.go` are deferred to 05-07, as the plan states.

## Self-Check: PASSED

- FOUND: internal/relay/heartbeat.go, internal/relay/heartbeat_test.go, internal/webserver/liveness_test.go
- FOUND commits: 818a785f, 15341531, 754f8a96, ad990e02, 0e00cb0e, 8dc18ade
