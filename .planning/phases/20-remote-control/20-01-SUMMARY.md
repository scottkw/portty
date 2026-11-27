---
phase: 20-remote-control
plan: 01
subsystem: desktop
tags: [input, cgevent, webrtc-datachannel, wire-protocol, keytable]
requires:
  - phase: 19
    provides: "Manager, DataChannelHook, SCTP 8192, viewer record, Locked() cache, sys_darwin bridge style"
provides:
  - "screencap.InputBridge: tagged private-source pointer, key, text and scroll posting, display bounds and cursor reads"
  - "screencap.KeyCode closed W3C-code to kVK table (checked against the SDK header) and UTF16Chunks"
  - "desktop input path: input channel accept, parseInput, handleInput single entry point, one input goroutine, ReleaseViewer"
  - "frozen v1 wire contract: testdata/input-v1.jsonl and signal-v1.jsonl"
  - "RightsControl/Clipboard/Surface/OwnAccount and the hello caps {video:1,input:1} plus rights map"
affects: [20-02, 20-03, 20-04, 20-05, 20-06, 21, 22]
tech-stack:
  added: []
  patterns:
    - "viewer input state under one Manager.in.mu; releases queued to the same goroutine that posts, so they order after any in-flight press"
    - "a later move replaces a pending move only while nothing is queued behind it"
key-files:
  created:
    - internal/screencap/input_darwin.m
    - internal/screencap/input_darwin.go
    - internal/screencap/input_darwin_test.go
    - internal/screencap/keytable.go
    - internal/screencap/keytable_test.go
    - internal/desktop/input.go
    - internal/desktop/input_test.go
    - internal/desktop/protocol_test.go
    - internal/desktop/testdata/input-v1.jsonl
    - internal/desktop/testdata/signal-v1.jsonl
  modified:
    - internal/desktop/manager.go
    - internal/desktop/viewer.go
    - internal/desktop/fanout_test.go
    - internal/desktop/export_test.go
    - TESTING.md
key-decisions:
  - "Releases bypass every gate and a full queue; a viewer's input life ends (closed flag) on leave, channel close and Close, which discards its queue and queues one release per held button"
  - "An input channel must also be ordered and fully reliable, or it is closed (an unordered channel could reorder down and up)"
  - "Help has no key table entry: Insert already maps to kVK_Help and every vk appears once"
requirements-completed: [DCTL-01, DCTL-02]
status: complete
duration: ~1h
completed: 2026-11-26
estimate:
  tokens: 90000
  raw_tokens: 90000
  tasks: 3
  confidence: low
actuals:
  tokens: 20500
  tasks: 3
  commits: 3
plan_head_before: 787f62d10db7ea5a3923ef4e2e6af1118053382c
plan_head_after: 2ba7e6dd3da1b3cc666e998e01b20cbe2693f3c5
---

# Phase 20 Plan 01: Input path, key table and frozen v1 wire contract Summary

A viewer's click now travels from its `input` data channel through parse, epoch and lock gates, main-display mapping and a tagged private CGEventSource to the Mac, with the key table, text chunker and golden wire files that Phases 20 to 22 build on.

## What was built

- **Task 1 (tracer, c5ccd717):** `screencap.InputBridge` (`PostMouse`, `CursorLocation`, `DisplayBounds`, `EventMarker`) over one process-lifetime private source, marker in `kCGEventSourceUserData`, HID-tap posting, no cursor warp, `@try/@catch` on every entry. In `desktop`: `acceptDataChannel` (label `input`, viewer declared caps input 1, one per viewer), `parseInput` (4096 cap, closed kind set, per-kind validation, clamping), non-blocking `enqueueInput` hand-off, one `inputLoop` goroutine (the only caller of the bridge), `handleInput` as the single entry point, `ReleaseViewer`, `releaseAllViewers`, `InputCounters`, `currentEpoch`/`bumpEpoch`, `Manager.target()`. `leave` and `Close` release held buttons; Close drains them (up to 1 s) before anything else closes. `TestInputTracer_ClickReachesPoster` runs the whole path over loopback WSS and pion with a display at origin (-1470, 0) and lands the click at (-735, 239).
- **Task 2 (74b30200):** `keytable.go` (closed table, `KeyName`, modifier flags, `UTF16Chunks`), native `pc_input_key`, `pc_input_text` (1..20 units, flags 0), `pc_input_scroll` (pixel, continuous), and `PostKey`/`PostText`/`PostScroll`, all refusing under `testing.Testing()`. `TestKeytable_MatchesEventsH` runs against the real SDK header and passes (not skipped).
- **Task 3 (2ba7e6dd):** `RightsControl|Clipboard|Surface|OwnAccount`; server hello caps exactly `{video:1,input:1}` (an unknown offered cap is not echoed) plus `rights {control,clipboard,surface}`; viewers on the same-user route are `RightsOwnAccount`; the two golden JSONL files and `protocol_test.go`.

## Handoff order (cross-phase check)

`internal/daemon/handoff.go` `closeDesktopForHandoff` now calls only `mgr.Close()` (it no longer calls `CloseAll` first; its comment explains the 1012 and captureActive ordering). The order still holds: `Close` runs `releaseAllViewers("closing")` and waits for the input goroutine to post the queued releases before it cancels the state loop or closes any viewer, and a release already queued by `leave` is not posted twice (the held set is cleared on the first release). `go test ./internal/daemon -run 'Desktop|Handoff'` passes.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Plan contradicts itself on Insert and Help**
- **Found during:** Task 2
- **Issue:** The plan lists both Insert and Help as kVK_Help (0x72) and also requires every vk in the table to be unique.
- **Fix:** Only `Insert` maps to kVK_Help; `Help` has no entry (a physical Mac Help key is not a browser code a viewer sends in practice). Documented in `keytable.go`.
- **Files modified:** internal/screencap/keytable.go, keytable_test.go
- **Commit:** 74b30200

**2. [Rule 2 - Missing critical] Input channel must be ordered and reliable**
- **Found during:** Task 1
- **Issue:** The truth says "reliable ordered data channel", but nothing checked it; an unordered channel could deliver a button-up before its down.
- **Fix:** `acceptDataChannel` also requires `Ordered()` and no `MaxRetransmits`/`MaxPacketLifeTime`; a binary message is counted as malformed.
- **Files modified:** internal/desktop/input.go
- **Commit:** c5ccd717

**3. [Rule 3 - Blocking] Lint "unused" on bumpEpoch**
- **Found during:** Task 1 verify
- **Issue:** `bumpEpoch` is called only by 20-03, so golangci-lint failed the plan's own verify step.
- **Fix:** `BumpEpochForTest` in export_test.go plus `TestInput_EpochBumpDropsOldCoordinates`.
- **Files modified:** internal/desktop/export_test.go, input_test.go
- **Commit:** c5ccd717

### Other adjustments (not bugs)

- `UTF16Chunks` groups runes into clusters (base plus joiners, variation selectors, marks) and packs clusters, instead of cutting then moving the cut back. The behaviour list is met, including a run longer than a chunk (cut between code points, never mid-pair).
- `PostText` checks length before the test refusal so a 21-unit slice is a length error, not ErrRefusedInTest.
- `TestHello_RightsAndInputCap` lives in fanout_test.go (package `desktop_test`, where the loopback harness is); `protocol_test.go` is package `desktop` because it calls the unexported `parseInput`.
- `joinCaps` was split out of the `join` helper in fanout_test.go so input tests declare caps without a second copy of the handshake.
- `TESTING.md` Section 4 got four DCTL-01/DCTL-02 rows (standing rule). The Section 2 file counts and the Phase 20 note were left for the phase's closing plan, to avoid conflicting edits across parallel plans.
- `gsd_run windows append` was not run (the helper is not available in this worktree shell); the one known gap is listed under Known Stubs.

## Known Stubs

`handleInput` parses `rmove`, `wheel`, `key` and `text` but posts nothing for them yet: they are counted as `unhandled`. This is intentional; 20-02 implements them (the bridge calls and the key table already exist). `release` and `hb` are handled.

## Verification

- `go vet`, `gofmt -l`, `golangci-lint --new-from-rev=d792ca07` (0 issues), `bash tests/check-brand.sh`, `bash tests/check-traceability-paths.sh` pass.
- `go test -race -count=3 -run 'TestInput|TestFanout_'` and `go test -race -count=1 ./internal/desktop/ ./internal/screencap/` pass; the input tests were also run with `-count=12`.
- Host safety: no event was posted. The only native calls under test are the harmless cursor and display-bounds reads; every posting call returns ErrRefusedInTest and the desktop tests use the fake bridge.
- One full-package run of `./internal/desktop/ ./internal/screencap/` failed once (output truncated, test not identified) and passed in the 10 reruns that followed, with and without the screencap package alongside. Not reproduced; flagged for the orchestrator in case a Phase 19 timing test is flaky under load.

## Self-Check: PASSED

Created files all exist; commits c5ccd717, 74b30200 and 2ba7e6dd are on the branch.
