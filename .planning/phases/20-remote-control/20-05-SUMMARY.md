---
phase: 20-remote-control
plan: 05
subsystem: desktop
tags: [clipboard, nspasteboard, privacy, chunking, wire-protocol]
requires:
  - phase: 20-03
    provides: "inbound struct and post-offer dispatch switch in manager.go, RightsClipboard in viewer.go"
  - phase: 20-04
    provides: "manager.go as left after the window gate (re-read before editing)"
provides:
  - "clip_get / clip_put over /desktop/ws: Copy from Mac and Paste to Mac, only on an explicit viewer action"
  - "screencap.Pasteboard bridge: types-first read, accessBehavior, plain-string write, refused under go test"
  - "desktop.PasteboardBridge seam and Deps.Pasteboard"
affects: [20-08, 20-09, 20-10, 22]
tech-stack:
  added: []
  patterns:
    - "one pasteboard read in flight per process on its own goroutine, 2 s wait, result discarded after a timeout"
    - "chunk cost measured as the JSON-escaped size (conservative table), so every frame stays under 30000 bytes"
key-files:
  created:
    - internal/desktop/clipboard.go
    - internal/desktop/clipboard_test.go
    - internal/desktop/clipboard_export_test.go
    - internal/screencap/pasteboard_darwin.m
    - internal/screencap/pasteboard_darwin.go
    - internal/screencap/pasteboard_darwin_test.go
  modified:
    - internal/desktop/manager.go
    - internal/desktop/protocol_test.go
key-decisions:
  - "A transfer ID (xid) is 1 to 64 bytes; anything else answers failed with an empty xid. The ID is echoed on every frame, so an unbounded one would break the frame cap"
  - "Chunk cost uses a conservative JSON escape table (control characters 6, quote/backslash/newline/CR/tab 2, < > & and U+2028/2029 6) plus the exact worst-case frame overhead, rather than marshalling each candidate"
  - "The copy handler runs in the viewer's read loop (serial per viewer), so a viewer cannot have two copies at once; the cross-viewer guard is the process-wide readPending flag"
requirements-completed: [DCTL-06]
status: complete
duration: ~1h15m
completed: 2026-11-26
estimate:
  tokens: 75000
  raw_tokens: 75000
  tasks: 2
  confidence: low
actuals:
  tokens: 8300
  tasks: 2
  commits: 3
plan_head_before: 36b965ceaf33f4b5c89dbed36d82f70fde850f4d
plan_head_after: 6c11d3b8cc7e838b33497772140a0a2d7a9634ee
---

# Phase 20 Plan 05: Clipboard Copy and Paste Summary

A viewer's explicit Copy from Mac reads the Mac's pasteboard (types first, so concealed and transient items answer `concealed` with no content read) and streams it as chunks under the 32768-byte socket limit; Paste to Mac reassembles strictly ordered chunks and sets one plain string, never typing into an app.

## What was built

- **Task 1 (tracer, 65c8bda4):** `pasteboard_darwin.m/.go` (`pc_pb_read`, `pc_pb_write`, `pc_pb_behavior`; accessBehavior read by key so an older SDK still compiles; 1 MiB check before any copy; `@try/@catch` on all three; `Pasteboard.ReadText/WriteText/Behavior` all return `ErrRefusedInTest` under `testing.Testing()`). `clipboard.go` get side: rights, `Locked()`, per-viewer busy, process-wide `readPending`, 2 s wait, chunked `clip_data` then `clip_result`. `manager.go`: `Deps.Pasteboard` (default `screencap.NewPasteboard()`), `Manager.clip`, inbound `Xid/Seq/Total/Len/Text`, dispatch, `clip.drop` in `leave`. `TestClipboardTracer_CopyFromMac` over loopback WSS and pion.
- **Task 2 RED (ca7b4e74):** eight tests plus the clip golden decode and the pasteboard refusal test; the put-dependent ones failed against the stub.
- **Task 2 GREEN (6c11d3b8):** `handleClipPut` and `clipState.accept` (first chunk fixes xid, total, len; seq must be next; chunk at most 16384; running length never over len; final length equals len and valid UTF-8; one `WriteText`; any failure discards).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing critical] Transfer ID bound**
- **Issue:** the plan caps text and frame size but the viewer-chosen `xid` is echoed on every frame; a long one would push a chunk past 30000 bytes and a `clip_result` past what a viewer expects.
- **Fix:** `validXid` (1 to 64 bytes); the frame overhead in `chunkText` is computed for a 64-byte worst-case escaped ID and the largest numbers. A bad ID answers `failed` with an empty xid. Tested in `TestClipboard_CopyOutcomes`.
- **Commit:** 65c8bda4

**2. [Rule 2 - Missing critical] Defence in Go for a bridge that did not check the size**
- A `ReadOK` text over 1 MiB answers `too_large` in `handleClipGet` even if the bridge missed it (`TestClipboard_CopyOutcomes`, `t2`).
- **Commit:** 65c8bda4

### Other adjustments (not bugs)

- `pasteboard_darwin.m` has a third error code `kPCPbWriteErr = -90303` (`setString:forType:` returned NO); the plan listed two. A nil NSString still uses `kPCPbNilErr`.
- New test-only `internal/desktop/clipboard_export_test.go` (`ChunkTextForTest`, `ClipPutsForTest`), not in the plan's file list, in the same style as 20-03's `surface_export_test.go`.
- JSON decoding replaces invalid UTF-8 and lone surrogates with U+FFFD, so an invalid-UTF-8 chunk is never seen as invalid on the Go side: the raw-bytes test (`\xff`, len 1) is caught because the decoded text is 3 bytes over `len`. The `utf8.Valid` check on the assembled text stays as a backstop.
- The 2 s wait is a constant, so `TestClipboard_WaitingApproval` takes about 2 s of real time.
- The copy handler runs on the viewer's read loop, so that viewer's pings wait up to 2 s behind a blocked read. Acceptable; the daemon itself never hangs.
- `gsd_run windows append` not run (helper unavailable in this worktree shell); no stubs to record.

## Known Stubs

None. The real pasteboard calls (types-first read, `accessBehavior`, whether a launchd daemon is prompted for paste privacy) have never run: they compile, lint clean and are refused under tests (host safety). Plan 20-09 owns the VM measurement (research A10, R14).

## Threat Flags

None beyond the plan's register (T-20-25 to T-20-30 are all mitigated as written).

## Verification

- `go vet`, `gofmt -l`, `golangci-lint --new-from-rev=d792ca07` (0 issues), `bash tests/check-brand.sh` pass.
- `go test -race -count=3 -run 'TestClipboard_|TestClipboardTracer_|TestProtocolGolden_' ./internal/desktop/` passes; `go test -race -count=1 ./internal/desktop/ ./internal/screencap/` passes (59 s).
- Acceptance greps: ConcealedType 1, TransientType 1, changeCount 0, `testing.Testing()` 4, `handleClipPut` 1, `clipFrameMax` 3, text in a slog call 0, 8 desktop clipboard/golden tests listed.
- Host safety: no real pasteboard read or write, no capture, no input. Every test uses `fakePasteboard`.
- Flaky tests: none seen (`TestFanout_ThreeViewersEachGetKeyframe` passed in the full runs).

## TESTING.md rows for plan 20-11 to add (not edited here)

Section 2 (Suite Manifest, new files): `internal/desktop/clipboard_test.go` (loopback clipboard tests, desktop group), `internal/desktop/clipboard_export_test.go` (test seams, no tests), `internal/screencap/pasteboard_darwin_test.go` (refusal test, screencap group). `internal/desktop/protocol_test.go` gains `TestProtocolGolden_ClipDecodes`.

Section 4 (traceability, path column file only):

| Requirement | Path |
|-------------|------|
| DCTL-06 | internal/desktop/clipboard_test.go |
| DCTL-06 | internal/desktop/protocol_test.go |
| DCTL-06 | internal/screencap/pasteboard_darwin_test.go |

Section 5 (manual, VM rows; 20-09 owns the runs):

- M-NN: on macOS 27.2 a launchd daemon reads `NSPasteboard`; record whether the paste-privacy alert appears and whether "Paste from Other Apps" answers it (research A10, R14); with the alert up and unanswered, Copy from Mac shows waiting_approval after 2 s and the daemon keeps serving.
- M-NN: copy a password-manager item (ConcealedType) and a plain text on the Mac, Copy from Mac returns only the plain one; a concealed item returns the concealed notice and no content.
- M-NN: Paste to Mac sets the Mac's clipboard and nothing is typed into the focused app; Cmd+V on the Mac then pastes the text.

## Self-Check: PASSED

Created files exist (clipboard.go, clipboard_test.go, clipboard_export_test.go, pasteboard_darwin.m, pasteboard_darwin.go, pasteboard_darwin_test.go); commits 65c8bda4, ca7b4e74 and 6c11d3b8 are on the branch.
