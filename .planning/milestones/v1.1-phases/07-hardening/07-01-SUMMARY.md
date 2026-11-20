---
phase: 07-hardening
plan: 01
subsystem: relay
status: complete
requirements-completed: [HARD-01, HARD-03, HARD-07]
tags: [relay, daemon, vt, query-strip, heap, flaky-test]
requires: []
provides:
  - "relay.StripTerminalQueries: one query strip shared by the hub (live feed, full and delta replays) and the daemon engine"
  - "Live emulator rebuild on alt-screen exit (rebuildAfterAltExitLocked), freeing vt's alt scrollback"
  - "Deterministic TestHub_TwoClientsFanOut / TestHub_InputFanOut"
affects: [internal/relay/hub.go, internal/daemon/engine.go]
tech-stack:
  added: []
  patterns:
    - "vt Callbacks.AltScreen only sets a Hub flag (it runs inside emu.Write); recordFrameLocked acts after the write"
    - "Heap soak tests read HeapAlloc after runtime.GC() and derive bounds from library constants"
key-files:
  created:
    - internal/relay/querystrip_test.go
    - internal/relay/altscreen_heap_test.go
  modified:
    - internal/relay/hub.go
    - internal/relay/server_test.go
    - internal/daemon/engine.go
    - internal/daemon/engine_test.go
decisions:
  - "HARD-03: the daemon engine calls relay.StripTerminalQueries (daemon imports relay, never the reverse); the engine's private copy is deleted, so no parity test is needed"
  - "HARD-01: on alt-screen exit the live emulator is rebuilt and re-seeded with the old main screen (renderScreenLocked), not pushed into history like a resize, because no redraw follows an alt exit"
  - "HARD-01: growth while still in the alt screen stays capped only by vt's 10,000-line DefaultScrollbackSize; documented in a shortcut comment, no vt fork"
  - "HARD-07: the flake is a test race (write before the server's Join), not a hub race; the tests wait for SubscriberCount() >= 2"
metrics:
  duration: "~35 min"
  completed: 2026-11-18
estimate:
  tokens: 95000
  tasks: 3
actuals:
  tokens: 6900
  tasks: 3
  commits: 5
plan_head_before: f54a223a42507aad8d13cb177f1f24092fafac9e
plan_head_after: fb5f346d221f2df4f7c3557b1a1918dee7b5909d
---

# Phase 7 Plan 01: Query strip, alt-screen heap, fan-out flake Summary

The hub and the daemon engine now share one query strip (`relay.StripTerminalQueries`), which adds CSI 14/16/18 t, DA3, XTGETTCAP, DECRQSS, OSC 4 palette queries and OSC 52 clipboard reads. The live emulator is rebuilt when an app leaves the alt screen, so vt's alt scrollback (93.7 MiB at the cap) drops to 0.1 MiB, and history, screen and cursor are kept. TestHub_TwoClientsFanOut no longer flakes, because it waits for both server-side Joins before writing.

## Tasks

| Task | Name | Commits | Files |
|------|------|---------|-------|
| 1 | Shared query strip with one test per added sequence (HARD-03) | fe149959 (RED), 7da59cb9 (GREEN) | hub.go, querystrip_test.go, engine.go, engine_test.go |
| 2 | Alt-screen exit rebuild with a heap soak test (HARD-01) | 7323136d (RED), 49235eb3 (GREEN) | hub.go, altscreen_heap_test.go |
| 3 | Reproduce and close the fan-out flake (HARD-07) | fb5f346d | server_test.go |

## Tests added or edited (07-05 copies these into TESTING.md)

- **internal/relay/querystrip_test.go** (new)
  - `TestStripTerminalQueries`: 36 subtests. 15 strip the added sequences (CSI 14 t, 14;2 t, 16 t, 18 t, DA3 x2, XTGETTCAP x2, DECRQSS x2, OSC 4 x3, OSC 52 x2). 13 are regression strips (DA1, DA2, DSR, DECXCPR, DECRQM ANSI and DEC, ?2048h/l, XTVERSION, kitty, OSC 10/11/12). 8 keep cases leave the input unchanged (CSI 8;24;80 t, CSI 22;0 t, an OSC 4 set, a sixel DCS, SGR, ?2004h, ?1049h, plain text).
  - `TestJoin_StripsAddedQueries`: a full raw replay and a delta replay both drop every added sequence.
- **internal/relay/altscreen_heap_test.go** (new)
  - `TestHistory_HeapBoundAfterAltScreenScroll`: the alt-screen heap soak test (HARD-01).
  - `TestAltScreenExit_KeepsMainScreenAndHistory`: after the app exits, history holds 1..200 exactly once, the replay has no alt lines, and the cursor and screen text are unchanged.
- **internal/relay/server_test.go** (edited): the new helper `waitTwoSubscribers` is used by `TestHub_TwoClientsFanOut` and `TestHub_InputFanOut`.
- **internal/daemon/engine_test.go** (edited): `TestGetSessionStyledTailLines_AllQueriesNoHang` adds 8 anchors that cover the added sequences.

## Measured heap growth (80x24 grid, 80-column alt lines)

| Point | Without -race | With -race | Bound |
|-------|---------------|------------|-------|
| Burst 1 (20,000 alt lines) | 93.7 MiB | 102.9 MiB | ceiling 101.4 MiB (113.7 MiB under -race), floor 42.7 MiB |
| Burst 2 (another 20,000) | 0.0 MiB | 0.0 MiB | < burst 1 / 4 |
| After `ESC[?1049l` | 0.1 MiB (93.7 MiB before the fix) | 0.1 MiB | < 32 MiB |

## Fan-out flake reproduction (HARD-07)

- **Before the fix:**
  - Alone: 50/50 pass with `-race -count=50`, and 50/50 with `-cpu 1`.
  - Under load: `go test -race -short -count=2` over the non-daemon `./internal/...` packages ran in the background (the daemon package was left out, per the executor rule). One block of 200 failed twice and the other block passed. The failing block was not identified because the output was merged before counting.
  - Failing payload: `clientB: got payload "\x01hello world", want "hello world"`. That is exactly the planning-time analysis.
- **Root cause:** `dialWS` returns at the WebSocket handshake, and the server's `hub.Join` runs afterwards. When the PTY write is recorded before B's Join, B gets the bytes exactly once, inside its full raw replay. That copy carries the ring's 0x01 frame header. Bytes were never duplicated or lost, so there is no hub race.
- **After the fix:**
  - Alone: 50/50 pass with `-race -count=50`, 50/50 with `-cpu 1`, and 50/50 for TestHub_InputFanOut.
  - Under the same background load: 200/200 pass in each block.

## Verification output

```
go test -race -short -count=1 ./internal/relay/            ok  github.com/scottkw/portty/internal/relay  41.573s
go test -short -count=1 ./internal/relay/                  ok  github.com/scottkw/portty/internal/relay  7.116s
go test -race -short -count=1 -run 'TestGetSessionStyledTailLines' ./internal/daemon/   ok  ...daemon  1.049s
TestStripTerminalQueries subtests passing: 36 (>= 25)
grep -c queryStripPattern internal/daemon/engine.go           0
grep -c relay.StripTerminalQueries internal/daemon/engine.go  1
grep -c 'AltScreen:' internal/relay/hub.go                    1
grep -c 'shortcut:' internal/relay/hub.go                     2
grep -c 'SubscriberCount() >= 2' internal/relay/server_test.go 1
go test -race -count=50 -run 'TestHub_TwoClientsFanOut$' ./internal/relay/          ok  1.770s
go test -race -count=50 -cpu 1 -run 'TestHub_TwoClientsFanOut$' ./internal/relay/   ok  1.237s
```

Mutation check: a rebuild without the re-seed makes `TestAltScreenExit_KeepsMainScreenAndHistory` fail with "replay has 178 LINE tags, want 200".

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Test calibration] The in-alt heap ceiling needed a -race allowance**
- **Found during:** Task 2 GREEN (the full relay run under -race)
- **Issue:** Under -race, burst 1 grew 102.9 MiB, above the plan's 101.4 MiB ceiling. Without -race it grew 93.7 MiB. A scratch test showed the size of each cloned 80-cell line is the same with and without -race (9,498 vs 9,499 B). -race disables Go's tiny allocator, so each cell's 1-byte content string takes its own 16-byte block (+9.2 MiB at the cap).
- **Fix:** Without -race the ceiling is the plan's formula unchanged: `DefaultScrollbackSize x 80 x Sizeof(uv.Cell) + 16 MiB`. Only in a -race build (detected through `debug.ReadBuildInfo`'s `-race=true` setting) the test adds `DefaultScrollbackSize x 80 x 16 B`. The floor, plateau and post-exit assertions are unchanged.
- **Files modified:** internal/relay/altscreen_heap_test.go
- **Commit:** 49235eb3

**2. [Rule 1 - Bug] TestHub_InputFanOut had the same race**
- **Found during:** the Task 3 audit
- **Issue:** It dials two clients and compares client B's live payload exactly. It only waits on client A (the input round-trip), so B's Join can come after the write.
- **Fix:** The same wait, through the shared `waitTwoSubscribers` helper.
- **Commit:** fb5f346d

### Other notes
- The fan-out comment is on the helper `waitTwoSubscribers`, not inline, because two tests use it.
- `TestGetSessionStyledTailLines_AllQueriesNoHang` already passed before GREEN, because vt answers none of the added sequences. It is a guard and was never RED.
- The plan-ledger file was not written under `.git/worktrees/...`, because the worktree sandbox refused that git path form. `plan_head_before` is the startup base f54a223a, which was confirmed after the ff-only merge.

## Known Stubs

None.

## Threat Flags

None. The change stays inside the plan's threat model: T-07-01..04 are mitigated as planned, and T-07-05 is accepted (RE2 regexp).

## Self-Check: PASSED

- FOUND: internal/relay/querystrip_test.go, internal/relay/altscreen_heap_test.go
- FOUND commits: fe149959, 7da59cb9, 7323136d, 49235eb3, fb5f346d (all ancestors of HEAD)
