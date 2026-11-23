---
phase: 09-upgrades-keep-sessions
plan: 03
subsystem: relay
tags: [relay, hub, handoff, vt, gob, websocket, 1012]
status: complete

requires:
  - phase: v1.1 08-debt-cleanup
    provides: the alt-exit rebuild (probes, wrap keying, charstate, vtalt) and its rebuild fuzz
provides:
  - Hub.Freeze / Thaw (read loop stops at a read deadline without closing Done)
  - HubState export/import (Export, NewHubFromState with validation, staged Start, Digest)
  - seedLocked / applySeedLocked split, plus a handoff seed that carries the main screen behind an alt-screen app
  - HubManager.AddStaged
  - Hub.RestartViewers with a done channel; Subscriber.Restart; CloseReasonServiceRestart (1012)
affects: [09-04 daemon handoff, 09-05/09-06 TESTING.md, internal/webserver share viewer Restart]

actuals:
  tokens: 17650
  tasks: 3
  commits: 6
plan_head_before: 3be899ddad41246dacf8dba1186f88563a11e71e
plan_head_after: 4472dabe2cf7890c7869840b64375781f0e88172

tech-stack:
  added: []
  patterns:
    - "Freeze through SetReadDeadline: Run tells a freeze from EOF by os.ErrDeadlineExceeded plus a freezing flag under mu"
    - "Rebuild = applySeedLocked(seedLocked(tail)); Export reseeds its own hub from the export (rollback = import path)"
    - "State the seed's own writes change (modes, region, link, last move) is restored after applying a seed, on both sides"

key-files:
  created:
    - internal/relay/handoff_state.go
    - internal/relay/handoff_state_test.go
    - internal/relay/handoff_fuzz_test.go
  modified:
    - internal/relay/hub.go
    - internal/relay/manager.go
    - internal/relay/history.go
    - internal/relay/close_reasons.go
    - internal/relay/server.go
    - internal/relay/server_test.go

key-decisions:
  - "The alt part of the handoff seed enters the alt screen with ?1047h, not ?1049h: vt's ?1049h saves the cursor and would overwrite the main saved cursor the main seed restored (caught by the 1047-app corpus case)"
  - "The export tail is scanned from the de-headered ring when the GL scan's parser is mid-sequence, not taken from the last frame only, so a sequence that spans reads completes on the new side"
  - "Digest also hashes the ring bytes (the plan listed end, length, history count, render); the plan's own DigestDetectsDifference test needs it"
  - "Ring.Max is accepted in 1..DefaultScrollbackBytes (tests use 64 KiB rings); larger is refused"
  - "ResizeClient holds a new gridMu so no resize lands between a freeze's last read and the export; Run retries a stale deadline error after a freeze that gave up, so it never shuts a live session down"

patterns-established:
  - "probeCursorLocked: the alt-exit probes extracted unchanged and reused for the alt screen"
  - "probeRegionLocked: reads the active DECSTBM through origin-mode CUP clamping"

requirements-completed: [UPGR-01, UPGR-02]

coverage:
  - id: D1
    description: "Freeze/Thaw lose no byte, never close Done, fail at once without deadlines, report ErrHubEnded on a racing exit, apply frozen resizes on Thaw"
    requirement: UPGR-01
    verification:
      - kind: unit
        ref: "internal/relay/handoff_state_test.go#TestHub_FreezeThaw, TestHub_FreezeWithoutDeadline, TestHub_FreezeWhileEnding, TestHub_FrozenResizeAppliedOnThaw"
        status: pass
  - id: D2
    description: "HubState round-trips through gob into a staged hub: every resume delta, the late-joiner render and continuations equal a reference hub, in and out of the alt screen"
    requirement: UPGR-02
    verification:
      - kind: unit
        ref: "internal/relay/handoff_state_test.go#TestHandoffState_RoundTripPlain, TestHandoffState_RoundTripCorpus, TestHandoffState_ExportReseedsSelf, TestHandoffState_DigestDetectsDifference, TestHandoffState_ImportValidates, TestHandoffState_StagedReadsNothing, TestHubManager_AddStaged"
        status: pass
      - kind: unit
        ref: "internal/relay/handoff_fuzz_test.go#TestHandoffRoundTrip_Streams, FuzzHandoffRoundTrip"
        status: pass
  - id: D3
    description: "RestartViewers closes viewers with 1012 service restart (retryable) and its channel waits for the close handshake"
    requirement: UPGR-02
    verification:
      - kind: integration
        ref: "internal/relay/server_test.go#TestRestartViewers_Close1012, TestRestartViewers_NilRestartFallsBack, TestRestartViewers_NoSubscribers"
        status: pass

duration: ~45min
completed: 2026-11-19
---

# Phase 9 Plan 03: Relay hub state that moves byte for byte — Summary

**A hub can be frozen at a read boundary, exported to gob-able plain data (ring with its end offset, history, modes, wraps, charset state, grid, an emulator seed that carries the main screen behind a full-screen app) and rebuilt as a staged hub whose deltas, renders and continuations equal a reference hub's; viewers can be told 1012 service restart.**

## Exported API (for 09-04)

```go
func (h *Hub) Freeze() error                 // nil: Run stopped, Done open; error at once without deadlines; ErrHubEnded; 2 s bound
func (h *Hub) Thaw()                         // resumes Run, applies resizes held back while frozen
var ErrHubEnded error
func (h *Hub) Export() (HubState, error)     // requires a frozen hub; reseeds the old emulator from the export
type HubState struct { SessionID; Ring RingState; History HistoryState; Modes map[int]bool; GeomEnd uint64;
    MainRegion [2]int; Wraps WrapState; LastMove [2]uv.Position; CS CharsetState;
    PtyCols, PtyRows, EmuCols, EmuRows int; HasEmu bool; Seed, Tail []byte }
func NewHubFromState(st HubState, reader io.Reader, writer io.Writer, resizeFn func(cols, rows int) error) (*Hub, error)
func (h *Hub) Start()                        // runs Run once
func (h *Hub) Digest() [32]byte              // ring end, ring bytes, history count, rendered screen
func (m *HubManager) AddStaged(id string, h *Hub) error
func (h *Hub) RestartViewers() <-chan struct{} // closed once every Restart/CloseSlow call returned
Subscriber.Restart func()                    // relay WS handler: close 1012 "service restart"
const CloseReasonServiceRestart = "service restart"
```

09-04 notes: Freeze needs a reader with SetReadDeadline (09-02's pty.Session). The browser share viewer's `Subscriber.Restart` is still to be set in internal/webserver/server.go (09-04, wave 2); until then it falls back to CloseSlow (policy violation "too slow").

## Accomplishments

- **Freeze/Thaw (Task 1):** `Run` no longer defers Shutdown. On `os.ErrDeadlineExceeded` with `freezing` set it closes `frozen` and returns, and every other error still shuts down. Freeze fails at once without deadline support or on `os.ErrNoDeadline`, returns ErrHubEnded when Done closes first, and gives up after 2 s, restarting Run if it had stopped by then. ResizeClient records sizes while frozen and Thaw re-runs the arbiter (`applyGridLocked`).
- **Seed split (Task 1):** `rebuildAfterAltExitLocked` is now `applySeedLocked(seedLocked(tail), tail, w, h)`. The probe block moved unchanged into `probeCursorLocked`, with every comment and `shortcut:` kept. `altscreen_heap_test.go` and vtalt are unchanged, and the rebuild fuzz still reports 0 mismatches.
- **HubState (Task 1):** Export copies every slice. NewHubFromState validates the ring (max, length, end, truncation consistency), geomEnd, the history caps and flag count, the wrap ring, the grid and emulator sizes (1..1000), MainRegion, charset designations, the parser state and the link fields before building anything.
- **Alt-screen seed (Task 2):** the alt screen is rendered and probed first (cursor, pen, link, saved cursor, region, pending wrap). Then `?1049l` on the old emulator exposes the main screen to seedLocked. The seed is the main seed, then `ESC(B` if G0 was designated, the mode restores, `?1047h`, a pen and link reset, the alt rows, region, saved cursor and pending wrap, then the alt pen and charset suffix.
- **Self-reseed (Task 2):** Export applies its own seed to the old hub (Pitfall 7) and restores the app's modes, region, link and last move. The old hub then equals the imported one, digest included, so a rollback resumes on the exported state.
- **1012 (Task 3):** RestartViewers collects subscribers under mu, unlocks, then calls each Restart (or CloseSlow) in its own goroutine and returns a channel closed when every call has returned. A hub with no viewers gets a channel that is already closed.

## Tests added

- `internal/relay/handoff_state_test.go` (new): TestHub_FreezeThaw, TestHub_FreezeWithoutDeadline, TestHub_FreezeWhileEnding, TestHub_FrozenResizeAppliedOnThaw, TestHandoffState_RoundTripPlain, TestHandoffState_ImportValidates (13 invalid fields), TestHandoffState_StagedReadsNothing, TestHubManager_AddStaged, TestHandoffState_RoundTripCorpus (13 cases), TestHandoffState_ExportReseedsSelf (same 13), TestHandoffState_DigestDetectsDifference
- `internal/relay/handoff_fuzz_test.go` (new): TestHandoffRoundTrip_Streams (300 streams, 60 under -short; checks the imported hub and the reseeded old hub against an emulator fed every byte), FuzzHandoffRoundTrip (8 seeds from rebuildFuzzStream; skips the documented seed losses)
- `internal/relay/server_test.go` (extended): TestRestartViewers_Close1012, TestRestartViewers_NilRestartFallsBack, TestRestartViewers_NoSubscribers

Mutation checks (temporary edits, reverted): removing the alt region probe, the saved-cursor pending path, the last-column pending path, the pen and link reset after `?1047h`, or the `ESC(B` each fails a named corpus case. Swapping `?1047h` for `?1049h` fails 6 of 300 sweep streams, and dropping the alt body fails 53.

## Corpus cases that needed attention

All divergences came from the alt-screen seed while it was being built. None needed a new loss on the `shortcut:` list.
- The alt cursor starts as a copy of the main cursor in vt (pen and link). Without `ESC[m` and an OSC 8 reset after `?1047h`, alt rows printed in the main pen or link.
- G0 is global in vt. The main seed's suffix designated line drawing before the alt rows, so `ESC(B` goes first.
- `?1049h` saves the cursor in vt and clobbered the main saved cursor of a `?1047` app, so the seed uses `?1047h`.
- Entering the alt screen clears vt's pending wrap, so the `?1047` trick cannot set it there. The seed re-prints the cell at the last column, or at the saved cursor through DECRC, which keeps the flag.

## Task Commits

1. Task 1 RED `3641d1fc` test(09-03): freeze/thaw, HubState round trip, import validation, staged start
2. Task 1 GREEN `88270140` feat(09-03): freeze/thaw, seed split, HubState export/import with staged start
3. Task 2 RED `c82bf067` test(09-03): round-trip corpus in and out of the alt screen, self-reseed, digest
4. Task 2 GREEN `d90f34fc` feat(09-03): alt-screen handoff seed, self-reseed on export, digest over ring bytes
5. Task 3 RED `9ef14dab` test(09-03): 1012 restart close for viewers, handoff stream sweep and fuzz target
6. Task 3 GREEN `4472dabe` feat(09-03): RestartViewers closes viewers with 1012 service restart

## Verification

- `go build ./... && go vet ./internal/relay/`: clean
- `go test -race -short -count=1 ./internal/relay/`: `ok github.com/scottkw/portty/internal/relay 88.982s` (the plan's one package run)
- Task 1 subset (`TestHub_Freeze|TestHub_Frozen|TestHandoffState_|TestAltScreenExit_|TestFullReplay_|TestReplayFidelity_|TestSoftWrap_`): ok 35.2s
- Task 2 subset (`TestHandoffState_|TestAltScreenExit_|TestVTLayout_|TestHistory_|TestFullReplay_`): ok 65.0s
- `git diff --quiet b4d63644 -- internal/relay/altscreen_heap_test.go internal/relay/vtalt.go internal/relay/vtalt_test.go internal/relay/protocol.go`: unchanged; `const MaxInputPayload = 32767` unchanged
- The grep gates for Freeze, seedLocked/applySeedLocked, HubState, RoundTripCorpus, 1049h, CloseReasonServiceRestart, StatusServiceRestart and FuzzHandoffRoundTrip all pass

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Export tail from the ring, not the last frame**
- **Found during:** Task 1 design
- **Issue:** `incompleteTail` of the last frame assumes ground at the frame's start. A sequence that spans reads (an OSC 8 URL, common in the 1..24-byte fuzz reads) would be dropped, and its remainder would print as text on the new side.
- **Fix:** when the GL scan's parser state is not ground, scan the ring with its frame headers removed. A `shortcut:` notes the remaining limit: a sequence opened before the ring's oldest byte.
- **Files:** internal/relay/handoff_state.go
- **Commit:** 88270140

**2. [Rule 1 - Bug] `?1047h` instead of `?1049h` in the alt seed**
- **Found during:** Task 2 (1047-app corpus case, sweep)
- **Issue:** vt's `?1049h` saves the cursor, which overwrote the main saved cursor.
- **Fix:** enter with `?1047h`, plus a pen and link reset.
- **Commit:** d90f34fc

**3. [Rule 1 - Bug] Digest hashes the ring bytes**
- **Found during:** Task 2
- **Issue:** the plan's Digest (end, length, history count, render) cannot detect a changed ring byte, which its own TestHandoffState_DigestDetectsDifference requires.
- **Fix:** hash the ring bytes as well.
- **Commit:** d90f34fc

**4. [Rule 2 - Correctness] gridMu and the stale-deadline retry**
- **Issue:** a resize racing the freeze could change the PTY grid after the last read but before the export. A deadline error delivered just after a freeze gave up would have shut a live session down.
- **Fix:** gridMu serialises ResizeClient with freeze start and end, and Run retries a deadline error once freezing is off.
- **Commit:** 88270140

**5. [Rule 2 - Correctness] Mode restores on a blank screen**
- **Issue:** seedLocked restores modes only with a screen body, so a blank screen (after `clear` with `?7l`) would lose its modes on import.
- **Fix:** the handoff seed prefixes the mode restores when the main body is empty. The alt part always restores them. The rebuild itself is unchanged.
- **Commit:** d90f34fc

**6. [Scope] applySeedLocked takes cols, rows**
- **Issue:** an import has no old emulator to take the size from.
- **Fix:** the key-link call is `applySeedLocked(seedLocked(tail), tail, w, h)`.

**7. [Scope] Ring.Max accepted in 1..DefaultScrollbackBytes**
- **Issue:** the plan allowed DefaultScrollbackBytes or "the hub's configured size". Hubs have no configured size other than the default, and the replay-fidelity corpus uses 64 KiB rings.
- **Fix:** accept any size up to the default.

## Known Stubs

None.

## Threat Flags

None beyond the plan's threat model (T-09-11..T-09-14 mitigated as planned; HubState is memory-only, and the History comment records it).

## Self-Check: PASSED

- FOUND: internal/relay/handoff_state.go, internal/relay/handoff_state_test.go, internal/relay/handoff_fuzz_test.go
- FOUND (ancestors of HEAD): 3641d1fc, 88270140, c82bf067, d90f34fc, 9ef14dab, 4472dabe
