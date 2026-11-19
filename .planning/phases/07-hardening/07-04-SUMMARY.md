---
phase: 07-hardening
plan: 04
subsystem: relay/input
status: complete
requirements-completed: [HARD-02]
tags: [HARD-02, relay, attach, relayClient, paste, dos-guard]
requires: [07-02]
provides:
  - "relay.MaxInputPayload = 32767 (Go) and MAX_INPUT_PAYLOAD = 32767 (frontend)"
  - "encodeInputFrames: sender-side split of large pastes, used by every GUI and browser terminal"
  - "CLI stdin reads (remote loop and local StdinPump) capped so every frame fits the 32768-byte read limit"
  - "over-limit guard tests on the relay server, the proxy client socket and the host share server"
affects:
  - "07-05 (TESTING.md rows, VM paste check)"
tech-stack:
  added: []
  patterns:
    - "split on the sending side; every server keeps coder/websocket's default 32768-byte read limit as the DoS guard"
key-files:
  created:
    - internal/relay/input_limit_test.go
    - internal/webserver/input_limit_test.go
  modified:
    - internal/relay/protocol.go
    - internal/attach/loop.go
    - internal/attach/loop_test.go
    - internal/attach/attach.go
    - internal/attach/attach_test.go
    - internal/daemon/remote_ws_proxy.go
    - internal/daemon/remote_ws_proxy_test.go
    - frontend/src/lib/relayClient.ts
    - frontend/src/lib/relayClient.test.ts
    - frontend/src/components/__tests__/TerminalPanel.scale.test.tsx
    - frontend/src/components/__tests__/TerminalPanel.guestClose.test.tsx
    - frontend/src/components/__tests__/TerminalPanel.contextLoss.test.tsx
decisions:
  - "HARD-02: large input is split on the sending side into frames of at most 32767 payload bytes; no server read limit was raised"
  - "HARD-02: byte-level splitting is correct; a multibyte character cut across two frames reaches the PTY reassembled because frames stay in order"
metrics:
  duration: ~20m
  completed: 2026-11-18
actuals:
  tokens: 5900
  tasks: 3
  commits: 6
plan_head_before: c5031b67d1cda09a6693f8dc921dd71cccab4bd9
plan_head_after: 87c12b92febfe8d177296a3db78a6d0b81f7f96e
---

# Phase 7 Plan 04: Split large pastes Summary

A paste over 32 KiB now goes out as several input frames of at most 32768 bytes each, from the GUI (local tab, peer tab, browser view) and from `portty attach` (local and remote). Every input reader keeps coder/websocket's 32768-byte default read limit as its DoS guard, and tests on all three readers prove a 32769-byte frame still closes the socket with 1009 and never reaches the PTY or the peer.

## What was built

- `internal/relay/protocol.go`: `const MaxInputPayload = 32767`, with a doc comment: the 32768 read limit minus the type byte, and senders split longer input.
- `internal/attach/loop.go` `readStdin` and `internal/attach/attach.go` `StdinPump`: the read buffer is `relay.MaxInputPayload` instead of `32*1024`. Before this, a full 32 KiB read plus the type byte made a 32769-byte frame, and the relay closed the socket.
- `frontend/src/lib/relayClient.ts`: `MAX_INPUT_PAYLOAD = 32767`. `encodeInputFrames(text)` encodes once with TextEncoder and slices the bytes into MSG_INPUT frames (empty text gives `[]`). It replaces `encodeInputFrame`. `sendInput` keeps both of its gates (reconnect-mode `state !== 'open'` and `readyState`) and sends each frame in order. The 07-02 code (`resume`, `withResume`, `resumePoint`) is unchanged.
- `internal/daemon/remote_ws_proxy.go`: the comment saying the client-side limit is harmless is replaced. The new comment says the limit is a DoS guard, that a larger frame closes the viewer socket, and that senders split input to `relay.MaxInputPayload`. No code change.

## Tests (for 07-05 / TESTING.md)

**New: `internal/relay/input_limit_test.go`**
- `TestServer_InputFrameAtLimitReachesPTY`: a 32768-byte input frame arrives on the PTY pipe complete
- `TestServer_InputFrameOverLimitDropped`: a 32769-byte frame never reaches the PTY within 500 ms and the server closes the socket (observed: `StatusMessageTooBig`, "read limited at 32769 bytes")

**New: `internal/webserver/input_limit_test.go`**
- `TestShare_InputFrameAtLimitReachesPTY`: over `dialShared`, a frame with a `relay.MaxInputPayload`-byte payload reaches the PTY complete
- `TestShare_InputFrameOverLimitDropped`: a 32769-byte frame never reaches the PTY within 500 ms and the socket closes with status 1009 (asserted)

**Extended: `internal/daemon/remote_ws_proxy_test.go`** (plus the helper `echoProxyConn`)
- `TestRemoteSessionWS_InputFrameAtLimitForwarded`: a 32768-byte frame comes back from the echo peer unchanged
- `TestRemoteSessionWS_InputFrameOverLimitClosed`: a 32769-byte frame closes the viewer socket with 1009 from the proxy (asserted), and no echo arrives

**Extended: `internal/attach/loop_test.go`** (plus the helper `pasteBytes`)
- `TestRun_LargePasteArrivesWhole`: a 100,000-byte numbered paste through the reconnecting loop reaches the fake relay's first connection whole and in order, over a single connection

**Extended: `internal/attach/attach_test.go`**
- `TestStdinPump_LargePasteArrivesWhole`: a 100,000-byte paste through `AttachSession` (local attach) reaches the fake relay whole and in order

**Extended: `frontend/src/lib/relayClient.test.ts`**
- `encodeInputFrames (HARD-02)` (replaces the `encodeInputFrame` describe):
  - short input is one frame: MSG_INPUT then the UTF-8 bytes
  - empty input is no frame
  - 100,000 ASCII characters split into 4 frames under the 32768-byte read limit, in order
  - a multibyte character straddling a frame boundary still concatenates byte-for-byte (20,000 '€', 60,000 bytes)
- `RelayClient host path sends a large paste as ordered frames (HARD-02)`:
  - sendInput of 100,000 characters on an open socket sends 4 MSG_INPUT frames in order
- `RelayClient reconnect mode (SHAR-07)`:
  - a large paste is sent as ordered frames once open, and nothing while reconnecting (HARD-02)

## Verification

- `go build ./...` ok; `go vet` on relay, attach, daemon and webserver ok.
- `go test -race -short -count=1 ./internal/relay/ ./internal/attach/ ./internal/daemon/ ./internal/webserver/`: all four `ok` (relay 42.7s, attach 6.1s, daemon 18.0s, webserver 12.4s).
- `go test -race -short -count=1 -run 'Attach|StdinPump' .` (the root cmd_attach tests that call AttachSession): ok.
- Frontend: `pnpm vitest run`: 90 files, 1218 tests passed, exit 0. `pnpm exec tsc --noEmit`: clean.
- `git diff c5031b67 -- internal | grep -c SetReadLimit`: 0.
- The plan's grep checks: `relay.MaxInputPayload` appears in loop.go and attach.go, `32*1024` appears in neither, the old "limits to 32 KiB anyway" text is gone, the proxy comment names MaxInputPayload, and relayClient.ts has MAX_INPUT_PAYLOAD 4 times and encodeInputFrames 2 times.
- RED evidence: before the fix, the loop test failed with "first connection got 0 bytes; want the 100000-byte paste in order" and the StdinPump test with "timed out after 2s waiting for the whole paste". The four guard tests passed before and after, because they pin limits that already exist.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] TerminalPanel.scale.test.tsx's RelayClient mock had no resumePoint**
- **Found during:** Task 2 (full Vitest run)
- **Issue:** 07-02 made a remote panel's cleanup call `client.resumePoint()`. It fixed the guestClose mock but not the scale mock. All 1218 tests passed, but Vitest reported 13 unhandled `TypeError: client.resumePoint is not a function` errors and exited non-zero, so the plan's `pnpm vitest run` check could not pass. The base commit had the same problem.
- **Fix:** added `resumePoint() { return null }` to the mock, the same line 07-02 added to guestClose.
- **Commit:** d9f8b5f9

**2. [Rule 1 - Cleanup] Stale `encodeInputFrame` keys in three TerminalPanel test mocks**
- The guestClose, scale and contextLoss module mocks still stubbed the removed export. They were renamed to `encodeInputFrames: () => []`. TerminalPanel imports neither, so behaviour did not change. Commit d9f8b5f9.

**3. Test detail: cross-realm Uint8Array comparison**
- jsdom's TextEncoder returns a Uint8Array from another realm, so `toEqual` on two byte-identical arrays failed. The byte comparisons use `Array.from` on both sides. The assertion is not weakened.

**4. Test detail: the relay guard tests pin the wire number**
- `internal/relay/input_limit_test.go` uses a local `wireLimit = 32768` rather than `MaxInputPayload`. A future raise of the constant would then still be tested against the real read limit. The share-server tests use `relay.MaxInputPayload`, as the plan says.

## Threat Flags

None. No new endpoints and no read limit changed.

## Known Stubs

None.

## Self-Check: PASSED

- FOUND: internal/relay/input_limit_test.go, internal/webserver/input_limit_test.go
- FOUND commits: d4050193, 78f7964d, 83e95924, d9f8b5f9, a4d29a7d, 87c12b92 (all ancestors of HEAD)
