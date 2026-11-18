---
phase: 05-overnight-survival
plan: 01
subsystem: relay
status: complete
tags: [relay, websocket, replay, resume, shar-07]
requires: []
provides:
  - "relay.Hub.Join: atomic subscribe + replay capture (delta or full), chunked to ReplayChunk"
  - "relay.Replay{Mode, Chunks, End} with Frames() (Meta{replay}, chunks, Meta{offset}) and Bytes()"
  - "relay.MetaPayload.Replay / Offset, ReplayFull / ReplayDelta, ReplayChunk, ParseResume"
  - "relay.Scrollback.End / SnapshotFrom (frame-byte offsets)"
  - "?resume= on GET /sessions/{id}/ws (share server, local relay); validated pass-through on /api/relay/remote/{sid}/ws"
affects: [05-02, 05-03, 05-04, 05-05, 05-06, 05-07]
tech-stack:
  added: []
  patterns:
    - "Join sequence: grid frame, Meta{replay}, <=16 KiB MsgOutput chunks, Meta{offset}, live"
    - "Client offset = offset meta + len(every later MsgOutput frame, header included)"
    - "Run records and broadcasts each frame under emuMu; Join subscribes and captures under emuMu"
key-files:
  created:
    - internal/relay/join_test.go
    - internal/webserver/resume_test.go
  modified:
    - internal/relay/protocol.go
    - internal/relay/scrollback.go
    - internal/relay/hub.go
    - internal/relay/server.go
    - internal/webserver/server.go
    - internal/daemon/remote_ws_proxy.go
    - internal/relay/scrollback_test.go
    - internal/relay/scrollback_altscreen_test.go
    - internal/relay/hub_test.go
    - internal/relay/server_test.go
    - internal/daemon/remote_ws_proxy_test.go
    - internal/daemon/attach_viewers_test.go
decisions:
  - "Replay.Frames() builds the meta/chunks/meta sequence once so the share server and local relay cannot drift on the wire contract"
  - "Join strips queries and chunks outside emuMu; only the subscribe and the byte capture run under the lock, so the drain loop is not held for a 256 KiB regex pass"
  - "The local relay moved to Join in Task 1 (not Task 3): deleting the old preamble method would not compile otherwise"
metrics:
  duration: 13min
  completed: 2026-11-17
  tasks: 3
  files: 14
estimate:
  tokens: 80000
  tasks: 3
actuals:
  tokens: 15300
  tasks: 3
  commits: 7
plan_head_before: 7885007062da8051a2fd99b7450f40319b3d408d
plan_head_after: 6d17c90958ef1558f984738b5cc8b3b77150967f
---

# Phase 5 Plan 01: Resumable Replay Contract Summary

The relay hub now counts output by frame-byte offset. A viewer that reconnects with `?resume=<offset>` gets only what it missed (a delta) when the 256 KiB ring still covers that offset. Otherwise it gets a full replay marked as such. Either way the replay is bracketed by `Meta{replay}` and `Meta{offset}` frames and sent in chunks of at most 16 KiB. `Hub.Join` subscribes and captures under the same lock `Run` records and broadcasts under, so every frame reaches the viewer exactly once.

## What was built

- **Wire contract (`protocol.go`):** `MetaPayload.Replay *string` (`"full"`/`"delta"`) and `MetaPayload.Offset *uint64`. Also `ReplayChunk = 16 << 10` (a var, so tests can shrink it) and `ParseResume` (decimal uint64 or not-ok).
- **Offsets (`scrollback.go`):** an `end` counter, `End()`, and `SnapshotFrom(off)`, which is ok only for `start <= off <= end`. The `ViewerReadLimit` doc now says why the 1 MiB limit stays: a live 32 KiB frame is still over the default.
- **`Hub.Join` (`hub.go`):** this replaces `ReconnectPreamble`. Delta: the query-stripped ring tail after `resume`. Full: the same choice the old preamble made (intact ring gives the raw stripped ring, a wrapped ring gives the emulator screen, an empty emulator gives the raw tail). `ensureLiveEmulatorLocked` and `renderScreenLocked` are factored out, and `EnsureLiveEmulator` and `RenderSnapshot` wrap them. The long #109 / M-51 why-comment moved onto Join, with a SHAR-07 paragraph added.
- **Exactly-once (`Run`):** `Run` now holds emuMu across `recordFrameLocked` and `broadcast`. Before this change, a frame recorded just before a Join was also queued to the new subscriber (the RED test caught 2 to 8 duplicates per run).
- **Call sites:** the share server (`handleWSSRelay`) and the local relay (`handleSession`) parse `resume`, call `hub.Join`, and write the grid frame and then `replay.Frames()`. On the share server the TOCTOU grant re-check still runs right after Join and before any replay byte. Both log `viewer-diag: replay session= conn= mode= bytes= resume=<bool>`.
- **Proxy:** `/api/relay/remote/{sid}/ws` builds the upstream query with `url.Values`. It always sends `cap`, and sends `resume` only if `ParseResume` accepts it, re-encoded with `strconv.FormatUint`. `redact` still replaces the whole RawQuery.

## Verification

- Plan gate: `go build ./... && go vet ./... && go vet -tags playwrightfixture ./cmd/playwright-fixture/ && go vet -tags vmpreview ./...` exit 0.
- `go test -race -short -skip '^(TestAPIAppearance_GetPatch|TestAPITerminalFont_GetPatch)$' ./...` exit 0. The same suite without `-short` also passes.
- Task verifies: 2 `TestWSS_Resume` passes, 8+ `TestJoin_`/`TestParseResume`/`TestScrollback_SnapshotFrom` passes, and the 3 Task 3 targeted passes. `TestJoin_ExactlyOnceUnderConcurrentOutput` passed `-count=5` under `-race`.
- Guards: `git grep 'ReconnectPreamble('` is empty. CSP, capability and Origin middleware and `web/` are unchanged since the plan base.

## TDD Gate Compliance

| Task | RED commit | GREEN commit | RED cause |
|------|-----------|--------------|-----------|
| 1 | 3df657f2 | bd74dc3d | no replay/offset meta (timeout; output before replay meta) |
| 2 | 3870384f | ce6c66f2 | duplicated LINE frames under concurrent joins |
| 3 | 82e5c4e5 | 6d17c909 | proxy dropped `?resume=` upstream |

TestRelay_ResumeDelta was green at its RED commit because the local relay moved in Task 1 (see deviation 1). The proxy test in the same commit was the real RED.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Local relay and old preamble tests migrated in Task 1, not Tasks 2/3**
- **Found during:** Task 1
- **Issue:** Task 1 deletes the old preamble method, but `internal/relay/server.go` and the relay tests still called it. The relay package would not build, and Task 1's `go vet ./internal/relay` would fail.
- **Fix:** Task 1 switched `relay/server.go` to Join (with resume parsing and the diag line) and moved the `fullReplay` helper and the test migration (`TestReconnectPreamble_*` renamed to `TestFullReplay_*`) into Task 1. Task 3 added TestRelay_ResumeDelta and the proxy change.
- **Commit:** bd74dc3d

**2. [Rule 1 - Bug] TestCLIAttach_LongHistoryJoiners assumed a single-message replay**
- **Found during:** Task 2 (non-short daemon run)
- **Issue:** the late joiners waited for MARKER-1 and then checked history. With chunked replay, MARKER-1 sits in an early chunk, so the check ran before the remaining chunks arrived (it reported 95 to 147 of 200 lines missing).
- **Fix:** wait for HIST-END, the last replayed line, before checking. The UAT gap 4 comment was updated.
- **Files modified:** internal/daemon/attach_viewers_test.go
- **Commit:** 82669909

**3. [Discretion] `Replay.Frames()` / `Replay.Bytes()` helpers**
- The plan spelled out writing the meta, the chunks and the meta inline at each server. One helper keeps the two servers on the same wire sequence.

**4. [Discretion] Join strips and chunks outside emuMu**
- Only the subscribe and the byte capture need the lock for exactly-once. The regex pass over up to 256 KiB runs after it is released, so the drain loop is not stalled.

Comment-only edits: `internal/daemon/attach_viewers_test.go` references to `Hub.ReconnectPreamble` now say `Hub.Join`.

## Notes for later plans

- Clients must take their resume position as `offset meta + sum(len(MsgOutput frame))` for live frames, counting the type byte. This is the unit the ring stores (see `readLiveUntil` in `internal/webserver/resume_test.go`).
- Replay payloads carry the ring's 0x01 frame headers as SOH bytes, the same as the old preamble.
- TESTING.md rows for `join_test.go` and `resume_test.go` are deferred to 05-07, as the plan states.

## Self-Check: PASSED

- FOUND: internal/relay/join_test.go, internal/webserver/resume_test.go
- FOUND commits: 3df657f2, bd74dc3d, 3870384f, ce6c66f2, 82669909, 82e5c4e5, 6d17c909
