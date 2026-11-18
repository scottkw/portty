---
phase: 05-overnight-survival
plan: 08
subsystem: relay
status: complete
gap_closure: true
tags: [relay, replay, reattach, geometry, shar-07, verf-01]
requires:
  - "05-01: Hub.Join full/delta replay, chunked to ReplayChunk"
  - "05-03: wrapped full replay (history CRLF + screen rows CRLF + CUP + DEC mode restores)"
provides:
  - "internal/relay/replay_fidelity_test.go: Claude-Code-like stream generator, host-vs-guest vt grid comparison (screenRows, scrollbackRows, diffGrid, requireSameGrid), negative control for the pre-05-03 format"
  - "Hub.geomEnd: ring end offset at the last genuine host resize; Join sends no delta or raw replay that reaches back past it"
  - "renderScreenLocked: never restores 2026; re-prints the last-column cell when the host cursor is in pending wrap"
  - "frontend/src/lib/__tests__/fixtures/wrapped-replay.json (for 05-12's xterm.js cross-check), kept in sync by TestReplayFidelity_XtermFixture"
  - "TestWSS_InputAfterWrappedReplay: typing reaches the PTY after a wrapped full replay over the share server"
affects: [05-12, 05-14, 05-15]
tech-stack:
  added: []
  patterns:
    - "Fidelity test: reference vt fed the raw stream vs guest vt fed only the replay; compare Render() rows, CursorPosition, scrollback"
    - "Negative control kept in the suite so a weakened comparison turns red"
    - "Generated fixture guarded by a Go test with an -update flag"
key-files:
  created:
    - internal/relay/replay_fidelity_test.go
    - frontend/src/lib/__tests__/fixtures/wrapped-replay.json
  modified:
    - internal/relay/hub.go
    - internal/webserver/resume_test.go
decisions:
  - "A resume offset exactly at geomEnd gets a full replay (plan said >=): with no output between the offset and the resize, the offset may predate the resize, and a full replay is always safe"
  - "Mode 2026 (synchronized output) is never restored in a rendered replay: the replay is a complete screen, so a join mid-update must not leave the viewer holding its rendering"
  - "Pending wrap is inferred (vt keeps atPhantom unexported): cursor at the last column, autowrap on (mode 7 unset or set), last cell a non-blank width-1 glyph; the replay re-prints that cell after the CUP"
  - "TESTING.md is not edited here: 05-15 owns one gap-closure note and the Section 4 rows for 05-08..05-14 (its plan names replay_fidelity_test.go and TestWSS_InputAfterWrappedReplay)"
metrics:
  duration: 12min
  completed: 2026-11-18
  tasks: 3
  files: 4
estimate:
  tokens: 75000
  tasks: 3
actuals:
  tokens: 32800
  tasks: 3
  commits: 4
plan_head_before: 56dc0b791084ca9a1f5c8fcf5410349028449b7d
plan_head_after: 26787936ac24bb1f09be3bc053145c70a0162650
---

# Phase 5 Plan 08: Reattach Replay Fidelity Summary

A guest vt emulator fed only the full replay of a wrapped, Claude-Code-like session now provably matches the host. Rows (text and style), cursor and all 1,110 scrollback lines are equal, and so is the next live Ink redraw. Join no longer sends a delta or a raw replay across a genuine host resize, which was the second cause of the jumble. Joins in the middle of a synchronized update, or at a pending wrap, also leave the guest identical to the host.

## Red evidence (required by the plan)

**The Task 1 tests passed at HEAD (e895ac98/56dc0b79): 05-03 had already fixed the staircase for the same-width case.** That pass counts as evidence only because the comparison is shown to catch the bug:

- **Negative control** `TestReplayFidelity_DetectsPre0503Staircase`: for the same session, it feeds the pre-05-03 format to a fresh guest. That format is the emulator's `Render()` as is, with bare-LF rows, no CUP and no restores. First differing row: **row 0**.
  - reference: `" the screen so a viewer that reattaches sees the same"`
  - pre-05-03: `"                      \x1b[38;2;80;200;120m⏺\x1b[m Read(internal/pkg/file158.go)"`

  This is the user's staircase: each row starts where the previous one ended. (The first run, before the cursor was parked in the input box, differed at row 0 too: `" viewer that reattaches sees the same transcript the"` vs `"                    \x1b[38;2;80;200;120m⏺\x1b[m Read(internal/pkg/file160.go)"`.)
- **Mutation checks on hub.go** (not committed):
  - Replacing the CRLF join with `Render()` as is fails WrappedClaudeSession and LiveRedrawAfterReplay at row 0 with the same strings.
  - Dropping the CUP fails WrappedClaudeSession on `cursor: host (13,26), guest (0,29)`. The generator parks the cursor in the input box, as Claude Code does, so the text alone leaves it in the wrong place.

**Task 2 RED (commit 64b0a85d)**, at HEAD with the Task 1 helpers:
- `ResumeAcrossResizeIsFull`: `resume from before a host resize: mode "delta", want "full"`.
- `IntactRingAcrossResizeRendered`: the full replay carried raw pre-resize Ink bytes (`ESC[1A`). Fed into an 80-col guest, that raw replay differs from the hub screen at row 0: host `"\x1b[1m> turn 4: please refactor the session module\x1b[m"`, guest `"完成 ✅ 🚀 turn 2 测试通过"`. The cursor was host {11 9} and guest (11,26). That is the jumble across a width change.
- `JoinMidSyncFrame`: the replay restored `ESC[?2026h`, so the viewer was left in synchronized output.
- `JoinAtPendingWrap`: after live `X\r\n`, row 0 differs. The guest overwrote the last column instead of wrapping.
- `FirstResizeKeepsRaw` was green as the #109 guard.

## What was built

- **Generator** (`claudeLikeTurn`, `claudeLikeFrame`, `claudeLikeRedraw`, `claudeStream`): deterministic output with no time or randomness. It covers a bold prompt echo, a truecolor `⏺` tool line, a dim `⎿` result line, a paragraph 1.5x cols long that soft-wraps, a CJK and emoji line, and a background-colored `ESC[K` line (styled trailing blanks). It also draws a box input frame exactly cols wide, a status line with a CHA, and bracketed paste on once. Each update is wrapped in `ESC[?2026h`/`ESC[?25l` and erases the previous frame Ink-style (`ESC[2K ESC[1A` × n, `ESC[G`). The cursor is parked at the end of the input text. The hub is fed about 200 KiB in 16 KiB chunks cut at arbitrary byte positions (mid-sequence and mid-rune), and the reference gets the same chunks.
- **hub.go**:
  - `geomEnd` (emuMu): `ResizeClient` passes `genuine = oldCols > 0 && oldRows > 0`, and `resizeLiveEmulator` records `scrollback.End()` after its nil check.
  - `joinLocked`: a delta needs `geomEnd == 0 || resume > geomEnd`. A full replay uses the rendered path when the ring is truncated or `geomEnd > 0`; the raw tail is still the fallback when the emulator has nothing to render.
  - `renderScreenLocked`: skips 2026 in the restores, and re-prints the pending-wrap cell (`uv.Line{*c}.Render()`) after the CUP.
  - The Join doc comment gains a geometry paragraph.
- **Pending wrap, what was used**: vt tracks the state as the unexported `Emulator.atPhantom`, and nothing in the public API exposes it. The hub infers it from `CursorPosition().X == Width()-1`, `CellAt` (a width-1, non-blank glyph) and `h.modes[7]` (autowrap). This is marked with a `shortcut:` comment: a cursor *moved* onto a filled last column, rather than printed there, would also be replayed as pending wrap.
- **Fixture**: `wrapped-replay.json` (109 KB) holds cols, rows, replay (base64, 73,135 bytes), screen, cursor, historyTail (20), live (base64), screenAfter and cursorAfter. I reviewed it: it contains only generator text (turn N, file0NN.go, the fixed paragraph, 完成 ✅ 🚀). Without the flag the test fails with `fixture stale: run go test ./internal/relay -run TestReplayFidelity_XtermFixture -update-replay-fixture`. That was checked: the run before the file existed failed with it.
- **Share-server input**: `TestWSS_InputAfterWrappedReplay` writes about 300 KiB in 15 KiB chunks. It asserts the full replay has no SOH byte, which proves the wrapped, rendered path. It then sends `typed-after-replay\r` and reads exactly those bytes from the PTY-input pipe within 2 s. This passed with no production change: input after a wrapped replay works.

## Verification

- Task 1 gate: vet ok, 3/3 PASS, `go test -race ./internal/relay` ok.
- Task 2 gate: vet ok, 8 `TestReplayFidelity_` PASS. The whole relay package passes, including TestFullReplay_IntactRingUnchanged, TestHistory_SurvivesResize, the #109/M-51 guards and TestJoin_*. `./internal/daemon -run '^TestCLIAttach_'` ok.
- Task 3 / plan gate: the fixture checks pass, and the fixture and input tests pass. `go build ./...` and `go vet ./...` are ok. In the full short suite all packages passed except one timing flake under load: `TestServer_LoopbackOrigin127Accepted` (a 5 s dial deadline in internal/relay). It passed alone (0.02 s), and `go test -race -short ./internal/relay` then passed in full.
- Plan verification: 9 `TestReplayFidelity_` PASS lines, including the negative control. `bash tests/check-traceability-paths.sh` reports 119 paths, all present.

## TDD Gate Compliance

| Task | RED commit | GREEN commit | RED cause |
|------|-----------|--------------|-----------|
| 1 (tracer) | 88a12486 | none needed | Passed at HEAD. The negative control and the two mutations show the comparison detects the staircase. The tracer gate re-ran verify after the commit: pass |
| 2 | 64b0a85d | 7c682489 | delta across a resize; raw ESC[1A in the replay; 2026 restored; pending wrap lost (quoted above) |
| 3 | 26787936 (tests + fixture) | n/a | The fixture test failed while the fixture was missing. The input test passed with no production change |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] The delta boundary had to be strict (`resume > geomEnd`)**
- **Found during:** Task 2 GREEN
- **Issue:** the plan's `resume >= geomEnd` still sent a delta when the offset was taken just before a resize with no output in between (offset == geomEnd). The plan's own ResumeAcrossResizeIsFull scenario is exactly that.
- **Fix:** `h.geomEnd == 0 || resume > h.geomEnd`. A full replay is always safe, and zero keeps the "no resize" meaning, so the fresh-session delta from 0 still works.
- **Files modified:** internal/relay/hub.go
- **Commit:** 7c682489

**2. [Rule 2 - Correctness] JoinMidSyncFrame also asserts the state right after the replay**
- **Found during:** Task 2 RED
- **Issue:** the plan's end-state check passes even when the replay restores `ESC[?2026h`, because the live second half closes it. A viewer whose host stalls mid-update would then hold its rendering.
- **Fix:** the test also checks that synchronized output is off right after the replay; hub.go skips 2026 in the restores.
- **Commit:** 64b0a85d, 7c682489

**3. Fixture directory created:** `frontend/src/lib/__tests__/fixtures/` did not exist before.

## Known Stubs

None.

## Threat Flags

None. No new endpoints or trust-boundary changes. The rendered path carries no terminal queries, and both raw paths still use liveEmuQueryStripPattern; TestJoin_DeltaStripsTerminalQueries and the #109 strip test stay green (T-05-27). The fixture is synthetic (T-05-28).

## Notes for later plans

- 05-12: the fixture's `replay` is the concatenated replay payloads, written after the grid frame into a fresh 100x30 terminal. Compare `screen`/`cursor` first, then write `live` and compare `screenAfter`/`cursorAfter`. The wide glyphs to watch are 完成, 测试通过, ✅, 🚀, ⏺, ⎿, ✻ and ✳. If xterm.js disagrees with vt on a width, regenerate nothing until the owning side is known.
- 05-15: add `internal/relay/replay_fidelity_test.go` (SHAR-07 and VERF-01 rows) and `TestWSS_InputAfterWrappedReplay` (extend the resume_test.go row) to TESTING.md. Go count +1 from this plan.
- Manual (todo item 4) still needs the two-Mac reattach after a long Claude Code session, with both Macs on this build (the replay is built by the host daemon).

## Self-Check: PASSED

- FOUND: internal/relay/replay_fidelity_test.go, frontend/src/lib/__tests__/fixtures/wrapped-replay.json, internal/relay/hub.go, internal/webserver/resume_test.go
- FOUND commits: 88a12486, 64b0a85d, 7c682489, 26787936
