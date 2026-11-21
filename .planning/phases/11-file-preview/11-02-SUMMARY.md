---
phase: 11-file-preview
plan: 02
subsystem: relay
status: complete
tags: [relay, preview, wire-format, backpressure, hub]
requires: []
provides:
  - "MsgPreview (0x21) wire format: header / chunk / end frames, builders and parsers"
  - "Hub.SendPreview: paced per-target sender, owner-only focus, one preview per hub"
  - "Golden fixture frontend/src/lib/__tests__/fixtures/preview-frames.json for the 11-05 parser"
affects: [11-04, 11-05, 11-07, 11-10]
tech-stack:
  added: []
  patterns:
    - "Paced per-subscriber sender: queue under h.mu only while len(sub.Msgs) < PreviewWindow, never sendLocked/broadcast"
    - "Go test pins a frontend fixture behind an -update-* flag (replay fixture precedent)"
key-files:
  created:
    - internal/relay/preview.go
    - internal/relay/preview_test.go
    - frontend/src/lib/__tests__/fixtures/preview-frames.json
  modified:
    - internal/relay/protocol.go
    - internal/relay/hub.go
decisions:
  - "Preview state (previewID, previewBusy) lives on Hub under mu (hub.go, Rule 3 deviation)"
  - "Hub end and stall both send a best-effort aborted end frame; a viewer that left gets nothing more"
  - "Header JSON uses json.Marshal (HTML-escaped: < becomes \\u003c); the fixture pins those exact bytes"
metrics:
  duration: "~45 min"
  completed: 2026-11-21
actuals:
  tokens: 19000
  tasks: 3
  commits: 2
plan_head_before: a4a81f1ecd1b1ae8877a857c630db53a28fa742e
plan_head_after: 4c9c9709c768bd5d9da9a7e7d37c0a2b1675c39f
---

# Phase 11 Plan 02: Relay preview wire format and paced Hub.SendPreview Summary

File previews now travel on the session's own hub as MsgPreview (0x21) frames. A paced sender delivers them to each viewer connected at that moment and only lets a preview use 16 of a viewer's 256 queue slots. In the test, a 20,000,000-byte preview to two throttled viewers dropped nobody while terminal output kept flowing.

## Wire layout as built (normative for 11-04 and 11-05)

| Frame | Bytes |
|-------|-------|
| header | `[0x21][0x01]` + JSON `{"id":<uint32>,"name":"<base name>","mime":"<PreviewMIMEs>","size":<bytes>,"key":"<32 lowercase hex>","focus":<bool>}` |
| chunk | `[0x21][0x02][id u32 big-endian][payload, at most 16384 bytes]` |
| end | `[0x21][0x03][id u32 big-endian][status u8]`: 0 ok, 1 aborted, 2 too large |

- The id comes from a per-hub counter that starts at 1. Frames are never written to the scrollback ring, so offsets and resume don't change.
- The header JSON comes from `json.Marshal`, which HTML-escapes the name (`<` becomes `<`, and `JSON.parse` reads it back unchanged).
- Largest frames: a chunk is 16390 bytes. The worst-case header (255 × `<` and the other worst values) is about 1.7 KB. Both are under 32768 and `ViewerReadLimit`.

## Constants

- `PreviewMaxBytes = 20_000_000`
- `PreviewChunk = 16 << 10`
- `PreviewWindow = 16`
- `PreviewMIMEs = image/png, image/jpeg, image/gif, image/webp, image/svg+xml, application/pdf, text/markdown`
- `PreviewOK = 0`, `PreviewAborted = 1`, `PreviewTooLarge = 2`
- `previewStallTimeout` is a package var, 60 s (tests shorten it)

## API

```go
type Preview struct { Name, MIME, Key string; Data []byte }
type PreviewSend struct { Viewers int; Done <-chan struct{} }
func (h *Hub) SendPreview(p Preview) (PreviewSend, error)
type PreviewHeader struct { ID uint32; Name, MIME string; Size int; Key string; Focus bool }
func MakePreviewHeader(PreviewHeader) []byte
func MakePreviewChunk(id uint32, data []byte) []byte
func MakePreviewEnd(id uint32, status byte) []byte
func ParsePreviewHeader(frame []byte) (PreviewHeader, error)
func ParsePreviewChunk(frame []byte) (id uint32, data []byte, err error)
func ParsePreviewEnd(frame []byte) (id uint32, status byte, err error)
```

- **Errors:**
  - `ErrPreviewInvalid` comes back before any frame is sent. Triggers: data over the cap, a MIME outside the set, a key that isn't 32 lowercase hex, a name that is empty, longer than 255 bytes, invalid UTF-8, or contains a C0/C1 control character.
  - `ErrPreviewBusy` comes back until the previous preview's Done closes.
  - `ErrHubEnded` comes back once the session has ended.
- **Targets:** taken as a snapshot under `h.mu`. A target is subscribed, not replaying and not revoked; read-only viewers count, later joiners never do. `focus = (sub == h.owner)`, so with no owner nobody gets focus. With no targets, SendPreview returns `Viewers: 0` and a Done that is already closed.
- **Sender:** one goroutine per target.
  - For each frame it takes `h.mu` and stops if the viewer left or is revoked (status `left`) or the hub is closed (status `ended`, best-effort aborted end).
  - It queues only while `len(sub.Msgs) < PreviewWindow`, using a non-blocking select. Otherwise it unlocks and sleeps, backing off from 2 ms to 50 ms.
  - If nothing moves for `previewStallTimeout`, it sends a best-effort aborted end (status `aborted`) and never kicks.
  - It takes only `mu` (no gridMu, emuMu or inMu) and logs after unlocking. It never calls `sendLocked` or `broadcast`.
  - The busy flag clears before Done closes.
- **Handoff:** previews aren't part of `HubState`. A preview in flight ends when viewers get 1012 and unsubscribe.

## Log lines (11-10 greps these)

```
viewer-diag: preview start session=<DiagID> id=<n> viewers=<n> bytes=<n> mime=<mime>
viewer-diag: preview end session=<DiagID> conn=<n> id=<n> status=ok|aborted|left|ended sent=<payload bytes> elapsed=<d>
```
Neither line includes the file name or any file bytes.

## Fixture

`frontend/src/lib/__tests__/fixtures/preview-frames.json` contains `maxBytes` (20000000), `chunkMax` (16384) and these cases:
- a header with focus true (id 1, `screenshot.png`)
- a header with focus false (id 0xfffffffe, `notes <v2> é.md`, text/markdown)
- a full chunk (16384 bytes) and a short chunk (id 0x01020304, 5 bytes)
- an end frame for each status

Each case has `frame` as base64 plus its decoded fields. `TestPreviewFrames_Fixture` compares the file byte for byte and rewrites it only with `-update-preview-fixture`.

## Tests (internal/relay/preview_test.go, for 11-10's TESTING.md rows)

- TestPreviewFrames_RoundTrip, TestPreviewFrames_ParseRejects, TestPreviewFrames_UnderReadLimits, TestPreviewFrames_Fixture
- TestHubSendPreview_TwoViewersOwnerFocus, TestHubSendPreview_NearCapNoKick, TestHubSendPreview_SkipsReplayingAndLateJoiners, TestHubSendPreview_NoOwnerNobodyFocuses, TestHubSendPreview_Busy, TestHubSendPreview_ViewerLeavesMidStream, TestHubSendPreview_HubEnds, TestHubSendPreview_StallAborts, TestHubSendPreview_Invalid, TestHubSendPreview_NoViewers

**Near-cap test runtime under -race:** 2.1 s of streaming (the test took 2.3 s in total). The setup was a 20,000,000-byte preview to two viewers (one read-only), each draining one frame per millisecond, while the terminal wrote a TICK line every 5 ms. Neither viewer was kicked, every byte arrived in order, and each viewer received at least 20 terminal frames during its stream.

**Mutation check:** I replaced the window test with `len(sub.Msgs) < cap(sub.Msgs)`. NearCapNoKick failed (both viewers kicked) and StallAborts failed (66 frames queued). I reverted the change before committing.

## Gate (Task 3)

- `go build ./...` and `go vet ./...` passed.
- One `go test -race -short -count=1 ./...` run, saved to `/tmp/p11-02-gate-go.log`. Every package was `ok` (relay took 268.6 s) except `internal/webserver`, where one test failed: `TestShare_DeadViewerReapedBacklogDelivered` (`input_queue_test.go:175: input idle at the reap, want the backlog still queued behind the blocked PTY`).
  - **That failure was already there before this plan:** the coordinator saw it fail 4 of 10 runs on the gsd/v1.2 base a4a81f1e without this plan's changes. It is a Phase 10 input-FIFO timing test, and a separate debug session owns it. This plan doesn't touch the webserver package, the input queue or the pumps.
  - No `--- SKIP` lines in the gate log, and no DATA RACE or panic lines.
- `tests/check-brand.sh` and `tests/check-traceability-paths.sh` passed.
- `internal/webserver/csp_mw.go` and `internal/daemon/remote_ws_proxy.go` are unchanged since a4a81f1e.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Hub fields added in hub.go (not in files_modified)**
- **Found during:** Task 1
- **Issue:** the per-hub preview id counter and the one-in-flight busy flag need state on `Hub`, and hub.go isn't in the plan's file list.
- **Fix:** added `previewID uint32` and `previewBusy bool` to `Hub`, both guarded by `mu` (4 lines). No logic in hub.go changed.
- **Files modified:** internal/relay/hub.go
- **Commit:** 73dccb3e

**2. Order of work in Task 1:** I wrote the implementation before the Task 1 tests. The plan is type `execute`, so no RED gate applies, and the Task 2 mutation check shows the tests catch a sender with no window.

## Commits

- `73dccb3e` feat(11-02): preview frames and Hub.SendPreview
- `4c9c9709` test(11-02): paced preview sender edge rules

## Threat Flags

None. There is no new endpoint or file access: preview.go imports no os, filepath or fs package (checked by a source gate), and the logs leave out file names.

## Known Stubs

None.

## Self-Check: PASSED

- FOUND: internal/relay/preview.go, internal/relay/preview_test.go, frontend/src/lib/__tests__/fixtures/preview-frames.json
- FOUND: 73dccb3e, 4c9c9709 (ancestors of HEAD)
