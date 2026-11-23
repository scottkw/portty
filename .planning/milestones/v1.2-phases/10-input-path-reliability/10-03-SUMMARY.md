---
phase: 10-input-path-reliability
plan: 03
subsystem: relay input path, share server, daemon handoff, relay client
status: complete
tags: [input, paste, heartbeat, pong, handoff, reliability]
requires: []
provides:
  - "relay hub input FIFO (OpenInput / InputConn / WaitInputIdle / ErrInputClosed)"
  - "relay.PongOwed and relay.WritePump shared by both servers"
  - "oldHandoff.finish waits for queued input inside restartDrain"
affects: [10-06 (claim/typing logic goes into the same MsgInput case), 10-07 (VM greps the diag lines below, TESTING.md rows)]
tech-stack:
  added: []
  patterns: ["hub-owned FIFO with per-connection quota, writer outlives the socket", "out-of-band owed-pong counter written by the write pump"]
key-files:
  created:
    - internal/relay/input.go
    - internal/relay/input_test.go
    - internal/relay/pong.go
    - internal/relay/pong_test.go
    - internal/relay/server_input_queue_test.go
    - internal/webserver/input_queue_test.go
  modified:
    - internal/relay/hub.go
    - internal/relay/server.go
    - internal/webserver/server.go
    - internal/daemon/handoff.go
    - internal/daemon/handoff_test.go
    - internal/daemon/remote_ws_proxy_test.go
    - frontend/src/lib/relayClient.ts
    - frontend/src/lib/relayClient.test.ts
    - frontend/src/components/__tests__/TerminalPanel.reconnect.test.tsx
decisions:
  - "Owed-pong counter (not a blocking send with timeout): a blocked read pump is the bug, and a timeout still drops the pong for a slow viewer"
  - "Daemon proxy unchanged: TestRemoteWSProxy_PasteBackpressureStaysOpen passed with no proxy change (Research Open Question 4)"
  - "TestShare_PasteBacklogKeepsViewer uses a 500 ms heartbeat timeout (not 100 ms) and holds 3x it after the whole paste is queued: under -race the 8 MiB burst itself can delay a pong past 100 ms, which is not the bug"
metrics:
  duration: "~45 min (includes ~10 min waiting for another executor's daemon test run to release the host PTY pool)"
  completed: 2026-11-20
estimate:
  tokens: 140000
  tasks: 3
actuals:
  tokens: 13000
  tasks: 3
  commits: 6
plan_head_before: d2998ad283a19c954c0f3120d5df43fe620dc2dc
plan_head_after: e8b3331a401628ff6d8d24baf28b33685daf7871
---

# Phase 10 Plan 03: Input Path Reliability (queue, pongs, handoff) Summary

Both read pumps now hand input to one hub-owned FIFO (1,024 frames / 32 MiB per connection) drained by a writer goroutine that outlives the socket. A pump never blocks on a slow PTY, so pings keep flowing. Pongs are written out of band by a shared `WritePump` ahead of queued output. The old daemon waits for queued input inside `restartDrain` before it exits on a handoff.

## What was built

- **Input FIFO (INPT-01)** `internal/relay/input.go`: each pump's filtered `MsgInput` goes to `InputConn.Write`. Write blocks only while that connection is at quota, never takes the handler context, and returns `ErrInputClosed` once the writer is dead (a PTY write error or hub shutdown). `Close` on pump exit keeps the connection's queued frames for delivery. Order is arrival order across all of a session's viewers, so a Ctrl-C typed after a big backlog waits behind it (documented in the file header). `inMu` is a leaf lock and is released around the PTY write. Pops are O(1): the slice resets when empty and compacts once the head passes half its length. The writer and its done-watcher start on the first Write only.
- **Pongs (INPT-02)** `internal/relay/pong.go`: `PongOwed` holds an atomic count plus a one-slot kick. `WritePump` checks the kick without blocking at the top of each loop, then selects on `sub.Msgs`, the kick, ctx, `hub.Done()` (close 1000 session ended, BUG-02 comment kept) and readDone. The two identical write loops in `relay/server.go` and `webserver/server.go` now both call it. Both read pumps answer `MsgPing` with `pongs.Owe()`. The non-blocking `sub.Msgs <- PongFrame` sends and their T-05-06 comments are gone.
- **Local relay pump** gets the same queue (Research Pitfall 1).
- **Handoff** (`oldHandoff.finish`): one `restartDrain` deadline now covers the 1012 handshakes and then `waitHubsInputIdle(m, deadline)` over every hub. `restartDrain` and `maxFrozen` are unchanged. A `shortcut:` comment records the residual: an upgrade during a slow multi-MB paste can lose the bytes still queued in the old daemon.
- **Client**: in `relayClient.ts` only comments changed (onPasteInterrupted, the `paste` field, the `pong` case, sendInput's ping and `checkPaste`). They now say a pong means "read by the server and queued for the PTY in order; queued bytes still reach the shell after a drop", and that the notice covers bytes sent after the last pong. No logic or copy change.
- **Browser notice (INPT-03, jsdom half)**: two new cases mount `WebShareSessionView` (wsURL, the live browser viewer) and assert the notices sit in a `[role="status"]` element.

## Exported API (signatures)

```go
// internal/relay/input.go
const InputMaxFrames = 1024       // per connection, queued plus in flight
const InputMaxBytes  = 32 << 20   // per connection
var ErrInputClosed = errors.New("relay: input writer stopped")
type InputConn struct{ /* unexported */ }
func (h *Hub) OpenInput(sub *Subscriber) *InputConn   // sub only tags diag lines; may be nil
func (c *InputConn) Write(b []byte) error            // caller must not reuse b
func (c *InputConn) Close()
func (h *Hub) WaitInputIdle(timeout time.Duration) (idle bool, queued int) // queued = bytes queued + in flight

// internal/relay/pong.go
type PongOwed struct{ /* atomic.Int32 + chan struct{} (cap 1) */ }
func NewPongOwed() *PongOwed
func (p *PongOwed) Owe()
func (p *PongOwed) Kick() <-chan struct{}
func (p *PongOwed) Take() int
func WritePump(ctx context.Context, conn *websocket.Conn, hub *Hub, sub *Subscriber, pongs *PongOwed, readDone <-chan struct{})

// internal/daemon/handoff.go (unexported)
func waitHubsInputIdle(m *relay.HubManager, deadline time.Time) map[string]int
```

`Hub.WriteInput` stays as the direct blocking writer. No production caller remains (engine_test and TestHubWriteInput still use it).

## New log lines (verbatim formats, counts only, never payloads)

```
viewer-diag: input backlog session=%s conn=%d frames=%d bytes=%d     // pump closed with frames still queued
viewer-diag: input backlog drained session=%s conn=%d                // that connection's last queued frame was written
viewer-diag: input writer stopped session=%s queued=%d err=%v        // queued = bytes dropped; logged once per hub that ever had input, at write error or hub shutdown
handoff: input still queued at exit session=%s bytes=%d              // session = relay.DiagID (first 8 chars)
```

Session fields use `relay.DiagID`. Note that `input writer stopped ... queued=0 err=<nil>` appears at every normal session end once the hub has received input. Read it as a loss only when queued > 0 or err is set.

## Tests added or extended (for 10-07's TESTING.md)

| File | Tests |
|------|-------|
| internal/relay/input_test.go (new) | TestHubInput_FIFOAcrossConnections, TestHubInput_QuotaBlocksOnlyThatConn, TestHubInput_WriterDeathUnblocks (subtests "write error", "hub shutdown"), TestHubInput_CloseKeepsBacklog |
| internal/relay/pong_test.go (new) | TestPongOwed, TestWritePump_PongBeforeQueuedOutput, TestWritePump_TwoPingsTwoPongs |
| internal/relay/server_input_queue_test.go (new) | TestRelay_PasteBacklogKeepsPonging |
| internal/webserver/input_queue_test.go (new) | TestShare_PasteBacklogKeepsViewer, TestShare_DeadViewerReapedBacklogDelivered |
| internal/daemon/handoff_test.go (extended) | TestHandoff_FinishWaitsForInput |
| internal/daemon/remote_ws_proxy_test.go (extended) | TestRemoteWSProxy_PasteBackpressureStaysOpen (+ fixture newSlowSinkPeer) |
| frontend/src/lib/relayClient.test.ts (extended) | "the ack pong arriving after a burst of output frames still clears the paste: no notice" (+ describe header comment) |
| frontend/src/components/__tests__/TerminalPanel.reconnect.test.tsx (extended) | "browser view (wsURL): a drop with nothing buffered and no ack pong shows "Paste may have been cut short (98 KB)" in role=status", "browser view (wsURL): a drop with 40,000 bytes buffered shows "Paste interrupted: 31 of 98 KB sent" in role=status" |

**Proxy:** TestRemoteWSProxy_PasteBackpressureStaysOpen passed on its first run with no proxy change. `internal/daemon/remote_ws_proxy.go` is not touched (`git diff d027e86d` on it is empty).

## TDD Gate Compliance

| Task | RED commit | Target | RED evidence | GREEN commit |
|------|-----------|--------|--------------|--------------|
| 1 | 6d8ecb87 | TestShare_PasteBacklogKeepsViewer | RED_EVIDENCE_OK: `input_queue_test.go:113: viewer dropped while the paste is queued (heartbeat timeout)`. Semantic: the target ran and failed on the liveness assertion because the share pump was still blocked in WriteInput. | c054b350 |
| 2 | 07b6a889 | TestRelay_PasteBacklogKeepsPonging | RED_EVIDENCE_OK: `no pong while the paste is queued: context deadline exceeded`. TestWritePump_PongBeforeQueuedOutput failed on ordering (0x01 first). | db43eb85 |
| 3 | 05d8c8c7 | TestHandoff_FinishWaitsForInput | RED_EVIDENCE_OK: `lost map[], want stuck-session: 300 bytes`. The proxy test, the wsURL notice cases and the relayClient burst case passed at once (the behavior already existed; they pin it). | e8b3331a |

The RED commits ship compile-only stubs (input.go, pong.go, a `waitHubsInputIdle` returning nil) that keep today's behavior. That way each RED commit builds and fails on an assertion, not on missing symbols (the tdd.md `build_failure` rule). Each GREEN commit replaces the stub.

## Gate results (run once each)

- `go build ./... && go vet ./...`: clean
- `go test -race -short -count=1 ./...`: exit 0, 0 `--- FAIL` / `^FAIL`, 0 DATA RACE, 13 packages ok. Log: /private/tmp/claude-501/-Users-ken-dev-portty/66f0384b-8902-40e9-9d84-68c47b57f8f8/scratchpad/p10-03-gate-go.log
- `pnpm vitest run`: 95 files / 1307 tests passed. Log: /private/tmp/claude-501/-Users-ken-dev-portty/66f0384b-8902-40e9-9d84-68c47b57f8f8/scratchpad/p10-03-vitest.log
- `pnpm exec tsc --noEmit`: exit 0. Log: /private/tmp/claude-501/-Users-ken-dev-portty/66f0384b-8902-40e9-9d84-68c47b57f8f8/scratchpad/p10-03-tsc.log
- check-brand and check-traceability-paths: pass
- Guards: MaxInputPayload 32767 unchanged. terminal.js, csp_mw.go, capability_mw.go, tailnet/live.go and remote_ws_proxy.go have no diff against d027e86d. SessionEndedBanner has no innerHTML. No `hub.WriteInput(` or pong-through-`sub.Msgs` remains in either pump. Both servers call `WritePump(`.

## Deviations from Plan

1. **[Rule 1 - test bug] TestShare_PasteBacklogKeepsViewer timing.** With the plan's 100 ms heartbeat timeout, the GREEN code once lost the viewer mid-burst under -race (213 of 256 frames read). The server was not blocked: it was busy unmasking and filtering megabytes, so the pong landed after 100 ms. The test now uses a 500 ms timeout, waits until the whole 8 MiB is queued (`WaitInputIdle(0)` queued == payload), then holds 3x the timeout watching the client socket. RED still fails: the stuck pump closes the socket within one timeout. It passed 6/6 in a loop. TestShare_DeadViewerReapedBacklogDelivered keeps 100 ms.
2. **[Rule 1 - test bug] Cross-test log bleed.** Late `input total` lines from the previous test landed in the captured log, so TestShare_DeadViewerReapedBacklogDelivered read the wrong frame count (1 in 5 runs). It now matches lines by its own session `DiagID`.
3. **Gate logs in the scratchpad, not /tmp.** The environment says to use the session scratchpad; the paths are listed above.
4. **Tracer gate:** Task 1 is `type="tracer"`. Its `<verify>` was re-run end to end after GREEN and passed (plus loops: relay FIFO 20x, share tests 6x) before Task 2 started.

No proxy queue was added. No architectural changes.

## Environment note (not this plan's code)

The first targeted run of the handoff tests failed with `open pty: device not configured`. Another executor's `go test -race -count=20 ./internal/daemon/` (pid 81668, the INPT-05 reproduction) held about 1,059 `/dev/ptmx` fds, which exhausts macOS's 511-PTY limit. I waited for it to exit and re-ran: everything passed. If that leak happens on every iteration, the 10-01 executor may want to look at it.

## Known Stubs

None. (The RED-commit stubs are fully replaced at HEAD.)

## Threat Flags

None. No new endpoint, auth path or trust boundary. The FIFO sits behind the existing read pumps and the 32 KiB frame guard. T-10-05/06/07/09/11 are mitigated as the register says. T-10-08 is accepted, with its `shortcut:` comment in handoff.go.

## Self-Check: PASSED

- Created files exist: input.go, input_test.go, pong.go, pong_test.go, server_input_queue_test.go, input_queue_test.go.
- Commits 6d8ecb87, c054b350, 07b6a889, db43eb85, 05d8c8c7 and e8b3331a are ancestors of HEAD.
