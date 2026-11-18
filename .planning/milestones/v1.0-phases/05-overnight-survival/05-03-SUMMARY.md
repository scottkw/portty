---
phase: 05-overnight-survival
plan: 03
subsystem: relay
status: complete
tags: [relay, scrollback, history, replay, shar-07, verf-01]
requires:
  - "05-01: Hub.Join full/delta replay, chunked to ReplayChunk"
provides:
  - "relay.History (NewHistory, Push, Clear, Len, Render), relay.HistoryMaxLines (10,000), relay.HistoryMaxBytes (4 MiB)"
  - "(*Hub).newLiveEmulator: the only emulator constructor (scrollback max, DEC mode callbacks, ESC[3J and RIS hooks)"
  - "Wrapped full replay: history CRLF + all Height() rows CRLF-joined + CUP + DEC mode restores (alt: ESC[?1049h before the rows)"
  - "Hub.history and Hub.modes survive emulator rebuilds; resize pushes visible main-screen rows into history"
affects: [05-04, 05-05, 05-06, 05-07]
tech-stack:
  added: []
  patterns:
    - "Drain emulator scrollback after every write: Push each line's Render(), clear(ls[:cap(ls)]), then Clear()"
    - "Emulator hooks (callbacks, CSI J, ESC c) registered in one constructor and returning false so defaults still run"
key-files:
  created:
    - internal/relay/history.go
    - internal/relay/history_test.go
  modified:
    - internal/relay/hub.go
decisions:
  - "Join prepends history only when the emulator screen is non-empty; a blank main screen with a wrapped ring still falls back to the raw tail, as before"
  - "The RIS hook also clears the emulator's own scrollback, because vt's fullReset leaves it in place and the post-write drain would otherwise refill history with pre-reset lines"
  - "After RIS the default reset reports every mode at its default, so restores then emit those defaults (harmless: they set the viewer to defaults)"
  - "Resize skips the visible-row push when the main screen is blank, so the first host attach does not add an empty line"
metrics:
  duration: 6min
  completed: 2026-11-17
  tasks: 2
  files: 3
estimate:
  tokens: 70000
  tasks: 2
actuals:
  tokens: 5800
  tasks: 2
  commits: 4
plan_head_before: fd374a2d8b08f19bc3dfff6b89ac76b7da6b97fa
plan_head_after: c92cc05e067c4ccf2388a484d278f5e41197ca19
---

# Phase 5 Plan 03: Hub-Owned Scrollback History Summary

Once the 256 KiB raw ring has wrapped, a full replay now carries up to the last 10,000 lines that scrolled off the host screen, with their colors. After that come every screen row, joined with CRLF, a CUP to the host cursor, and the app's DEC private modes. History lives in the hub, in memory only. A resize rebuild keeps it, and `clear` (ESC[3J) or RIS empties it.

## What was built

- **`internal/relay/history.go`:** `History` is a ring of rendered lines. A head index plus compaction at half keeps Push amortized O(1) and the backing array bounded. It drops oldest lines while over either the line cap or the byte cap; a single line over the byte cap empties the ring. `Render` joins with `\r\n` and adds no trailing CRLF. It has no mutex; the hub only touches it under `emuMu`.
- **`newLiveEmulator`:** this replaces all three `xvt.NewEmulator` sites. It sets the scrollback max to `HistoryMaxLines`, and the old 50-line constant is gone. It also tracks DEC modes into `h.modes`, clears history on `ESC[3J` (CSI J param 3), and clears history, modes and emulator scrollback on RIS.
- **Drain (`drainScrollbackLocked`):** runs after every emulator write in `recordFrameLocked`. It zeroes the full capacity of the internal slice before `Clear()`.
- **Replay format (`renderScreenLocked`):** an optional `ESC[?1049h`, then `Render()` with LF replaced by CRLF (all rows, trailing blanks kept), `ESC[y+1;x+1H`, and `ESC[?n h/l` for each tracked DEC mode in ascending order. Modes 47, 1047, 1049 and 2048 are skipped. In Join's wrapped branch, `history.Render() + "\r\n"` goes in front when history is non-empty. The intact-ring raw branch is untouched (#109).
- **Resize (`resizeLiveEmulator`):** for a main-screen emulator it drains first, then pushes the rendered rows from 0 through the cursor row, then rebuilds (the M-51 fix is kept).

## Verification

- Task 1 gate: `go vet ./internal/relay && go test -race -count=1 ./internal/relay` ok, with 4 targeted PASS lines.
- Task 2 / plan gate: `go build ./... && go vet ./... && go vet -tags vmpreview ./...` exit 0, 6 targeted PASS lines, and `go test -race -short -skip '^(TestAPIAppearance_GetPatch|TestAPITerminalFont_GetPatch)$' ./...` ok in all 11 packages.
- Mutation check: with `clear(ls[:cap(ls)])` removed, `TestHistory_HeapBoundAfterBurst` fails with 116 MiB of growth (bound 32 MiB). With the line restored it passes.
- Acceptance greps: `func NewHistory(` 1, `newLiveEmulator(` 1, `xvt.NewEmulator(` 1, `liveEmulatorScrollbackLines` 0, `clear(ls[:cap(` 1, `RegisterCsiHandler('J'` 1, `os.(WriteFile|Create|OpenFile)` 0, and non-05-01/02 commits on scrollback.go, protocol.go and server.go 0. go.mod and go.sum are unchanged.
- The existing #109, M-51 and BUG-04 tests in `scrollback_altscreen_test.go` pass unchanged. None of them pinned the old trimmed bare-LF screen byte for byte.

## TDD Gate Compliance

| Task | RED commit | GREEN commit | RED cause (RED_EVIDENCE_OK, semantic check done) |
|------|-----------|--------------|-----------|
| 1 | 2082e202 | 9cf509e3 | WrappedRingCarriesHistory: 23 LINE tags (screen only); WrappedRingFormat: bare LF at byte 28; History_Caps: empty scaffold. IntactRingUnchanged was green as a guard |
| 2 | 78673247 | c92cc05e | ED3Clears/RISClears: 10,000 LINE tags remained; SurvivesResize: last tag 11977, not 12000; ModeRestores/AltScreenOrder: no restores. HeapBoundAfterBurst was green as a guard (the zeroing landed in Task 1, per the plan) |

The Task 1 RED commit had a declaration-only `History` scaffold, so the RED tests compiled and failed on their assertions.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] RIS left pre-reset lines in the emulator scrollback**
- **Found during:** Task 2
- **Issue:** vt's `fullReset` resets both screens but not the scrollback. Lines scrolled earlier in the same write as `ESC c` would be drained into history after the hook had cleared it.
- **Fix:** the RIS hook also calls `emu.ClearScrollback()`. The drain still zeroes the backing array on the next write.
- **Files modified:** internal/relay/hub.go
- **Commit:** c92cc05e

Otherwise the plan was executed as written. `scrollback_altscreen_test.go` needed no change: its assertions use `bytes.Contains`, and `RenderSnapshot` stays screen-only, so the M-51 "no pre-resize content on screen" guard still holds.

## Known risks (not fixed, out of scope)

- vt's ED2 (`ESC[2J`) pushes the non-empty screen rows into scrollback before clearing (`ClearWithScrollback`), so a main-screen app that repaints with a bare `ESC[2J` adds a copy of its screen to history each time. zsh `clear` sends `ESC[3J` right after, which empties it.
- A single 32 KiB read with more than 10,000 newlines (for example `yes ''`) hits vt's O(n) `slices.Delete` in `Scrollback.Push` for each line past 10,000. That is slow but correct (RESEARCH accepted it).
- Alt-screen scrollback inside vt still grows without bound (Pitfall 3). This was deferred to 05-07 by the plan.

## Notes for later plans

- 05-07: add `internal/relay/history_test.go` rows to TESTING.md (Suite Manifest and traceability for SHAR-07 / VERF-01). Document that `clear` wipes the replayed history.
- Clients writing `ESC[2J ESC[H` before a full replay (CLI, 05-04/05) get history scrolled into their own scrollback, followed by the full screen. The replay never ends with a CRLF, so the viewport lines up with the host's.

## Self-Check: PASSED
