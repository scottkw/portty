---
phase: 08-debt-cleanup
plan: 01
subsystem: relay
status: complete
tags: [DEBT-01, DEBT-02, alt-screen, heap, soft-wrap, replay-fidelity]
requires: []
provides:
  - "Alt screen keeps no scrollback (vtalt.go, disableAltScreenScrollback on every newLiveEmulator build); in-alt heap growth 0.1 MiB at 80 and 220 columns (was 93.7 / 268.7 MiB)"
  - "vt version + Emulator.scrs layout pin (TestVTLayout_AltScreenScrollback); runtime fallback logs once and keeps the HARD-01 exit rebuild"
  - "Soft-wrap tracking: vt CursorPosition callback records wrapped-row hashes (wraps.go); History keeps a wrapped flag; rendered replays join wrapped rows with no CRLF in history, on screen and at the boundary"
  - "softwrap-replay.json fixture and a real xterm.js isWrapped check"
affects: [internal/relay/hub.go, internal/relay/history.go, frontend/src/lib/__tests__/fixtures/wrapped-replay.json]
requirements-completed: [DEBT-01, DEBT-02]
tech-stack:
  added: []
  patterns:
    - "Type-checked reflect + unsafe.Pointer(v.UnsafeAddr()) reach into one unexported field of a pinned module, pinned by a go.mod version test"
    - "Detect a terminal event (soft wrap) from emulator cursor callbacks and key it by a row-render hash in a bounded Hub-owned multiset"
key-files:
  created:
    - internal/relay/vtalt.go
    - internal/relay/vtalt_test.go
    - internal/relay/wraps.go
    - internal/relay/softwrap_test.go
    - frontend/src/lib/__tests__/fixtures/softwrap-replay.json
  modified:
    - internal/relay/hub.go
    - internal/relay/history.go
    - internal/relay/history_test.go
    - internal/relay/altscreen_heap_test.go
    - frontend/src/lib/__tests__/replayFidelity.test.ts
    - frontend/src/lib/__tests__/fixtures/wrapped-replay.json
decisions:
  - "DEBT-01: disable the alt Screen's scrollback once at construction (Screen.SetScrollback(nil) via reflect on scrs[1]) instead of SetScrollbackSize(0) (main-only, no-op for 0) or a periodic in-alt rebuild; exit rebuild kept as runtime fallback"
  - "DEBT-02: detect wraps as vt makes them (CursorPosition signature) and key them by FNV-64a of the row render; join only on main screen, autowrap on, full row, key present, non-empty next row"
  - "Autowrap gate also applied when a row drains into history (and in the resize push), so a row that scrolls off after ?7l is pushed unwrapped"
metrics:
  duration: "~20 min (wall clock from plan start)"
  completed: 2026-11-19
estimate:
  tokens: 115000
  tasks: 3
actuals:
  tokens: 63400 # chars/4 over the realized diff incl. regenerated fixtures; code and tests alone about 11,700
  tasks: 3
  commits: 5
plan_head_before: 292daeea
plan_head_after: 8351b9fd
---

# Phase 8 Plan 01: Alt-screen scrollback off and soft wraps kept in replay Summary

The live emulator's alt screen now keeps no scrollback. A TUI that scrolls 40,000 full-width lines in the alt screen grows the hub heap by 0.1 MiB at both 80 and 220 columns, down from 93.7 and 268.7 MiB. Soft-wrapped lines now replay as one wrapped line: from history, from the screen, across the history-to-screen boundary, and after both the alt-exit rebuild and the resize push. A vt guest and real xterm.js (`isWrapped`) both confirm it.

## What changed

- **internal/relay/vtalt.go** (new) has `altScreenOf` and `disableAltScreenScrollback`. Reflect finds `Emulator.scrs` and checks that it is exactly `[2]vt.Screen` before the `unsafe.Pointer(v.UnsafeAddr())` cast. The function then calls the exported `SetScrollback(nil)` on element 1.
- **hub.go newLiveEmulator** calls it on every build: the first build, the resize rebuild and the alt-exit rebuild. If the call fails, a package-level `sync.Once` logs `relay: vt layout changed; ...` and the HARD-01 exit rebuild still frees the alt scrollback when the app exits. The WR-04 `shortcut:` cost comment ("0.4-0.8 GiB") is gone. The rebuildAfterAltExitLocked doc now gives the rebuild's new reason.
- **internal/relay/wraps.go** (new) holds:
  - `wrapSetMax = 4096`;
  - `wrapSet`, a count map plus a FIFO ring;
  - `rowKey`, FNV-64a of `Line.Render()`;
  - `lineFull`.
- **hub.go**:
  - The `CursorPosition` callback records the row above when it sees the soft-wrap signature. It records only while `trackWraps` is set (only around the live feed write) and only on the main screen.
  - `drainScrollbackLocked` and `resizeLiveEmulator` push each row with its wrapped flag. The resize push forces the cursor row unwrapped, then clears the set.
  - `renderScreenLocked` joins screen rows through `screenRowWrappedLocked`.
  - `joinLocked` joins history to the screen when `EndsWrapped()` is true and the screen allows it. The `shortcut:` comment there covers both residuals: a wider `portty attach` reflows joined rows, and a content-identical row can be joined by mistake.
  - RIS and ED 3 clear the set.
- **history.go** keeps a `wrapped []bool` parallel to the lines, through compaction, and `Clear` drops it. The API is now `Push(line string, wrapped bool)`. `Render` joins a wrapped line to a non-empty next line without a CRLF. `EndsWrapped()` is new.

## Measured heap growth (TestHistory_HeapBoundAfterAltScreenScroll)

| Width | In alt, after 20,000 lines | In alt, after 40,000 lines | After exit | Before this plan (in alt) |
|-------|---------------------------|---------------------------|------------|---------------------------|
| 80    | 0.1 MiB | 0.1 MiB | 0.1 MiB | 93.7 MiB |
| 220   | 0.1 MiB | 0.1 MiB | 0.1 MiB | 268.7 MiB |

Under `-race` the figures were 0.1 / 0.1 / 0.1 MiB at 80 columns and 0.2 / 0.1 / 0.0 MiB at 220. The bound is a fixed 32 MiB at both widths. The control test (no disable call) grew 27.8 MiB for 3,000 alt lines at 80 columns, which shows the fixture really fills vt's alt scrollback.

## Tests (for 08-05 / TESTING.md)

- **internal/relay/vtalt_test.go** (new):
  - TestVTLayout_AltScreenScrollback: go.mod vt version pin, `scrs` layout pin, and the alt scrollback stays nil through RIS and Resize while the main one is unchanged.
  - TestDisableAltScreenScrollback_KeepsMainHistory.
- **internal/relay/altscreen_heap_test.go**:
  - TestHistory_HeapBoundAfterAltScreenScroll (rewritten, same name; subtests cols=80 and cols=220).
  - TestAltScreenScrollback_ControlWithoutBound (new).
  - TestAltScreen_JoinWhileInAltKeepsMainHistory (new).
  - `altChunks` now takes a width.
- **internal/relay/softwrap_test.go** (new):
  - TestWrapSet, TestLineFull.
  - TestSoftWrap_HistoryReplay, TestSoftWrap_ScreenReplay.
  - TestSoftWrap_ExactWidthHardLineStays.
  - TestSoftWrap_BlankContinuationKeepsRows (subtests "on screen" and "in history").
  - TestSoftWrap_AutowrapOffNoJoin, TestSoftWrap_AltScreenNoJoin, TestSoftWrap_RISClears.
  - TestSoftWrap_SurvivesAltExitRebuild, TestSoftWrap_SurvivesResizePush.
  - TestSoftWrap_XtermFixture.
- **internal/relay/history_test.go**: TestHistory_WrappedRender (new); TestHistory_Caps now calls `Push(l, false)`.
- **frontend/src/lib/__tests__/replayFidelity.test.ts**: new describe "soft-wrapped lines replay as one wrapped line (softwrap-replay fixture)".

## Rebuild-path fixes

None were needed. Both rebuild-path tests passed on the first run: the Hub-owned set survives the alt-exit seed, and the resize push takes each row's key before it clears the set.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Regenerated wrapped-replay.json**
- **Found during:** Task 2
- **Issue:** After the join change, TestReplayFidelity_XtermFixture reported its fixture stale. The Claude-like stream soft-wraps, so the rendered replay now has 125 fewer CRLFs. The fixture is not in this plan's files_modified. It is in 08-02's, which runs in a later wave, and neither 08-03 nor 08-04 touches it.
- **Fix:** Regenerated it with `-update-replay-fixture`. Only the `replay` field changed, while the screen, cursor and history-tail expectations stayed the same. The existing xterm.js wrapped-replay test passes on the new bytes.
- **Files modified:** frontend/src/lib/__tests__/fixtures/wrapped-replay.json
- **Commit:** 16543e5f

**2. [Test design] TestSoftWrap_SurvivesResizePush uses a 92-column line**
- The plan's 102-column line does not fit on one row at 100 columns, so the test uses `$ ` plus 90 characters instead: it wraps at 80 and fits at 100. The test asserts that the 100-column guest shows the line on one row and never shows the 80-column first row on its own. It has no vt reference grid, because vt does not reflow on resize the way xterm.js does.

**3. [Test design] TestDisableAltScreenScrollback_KeepsMainHistory compares against the pre-alt count**
- The plan said 26 lines scroll off. 50 lines plus CRLF on 24 rows actually scroll 27, so the test asserts that the count after the alt visit equals the count before it, and that the count is non-zero.

**4. Autowrap gate at push time**
- Rule 2 (autowrap on) is also applied when a row drains into history and in the resize push, not only at render time. TestSoftWrap_AutowrapOffNoJoin needs this: its long line drains after `?7l`.

**5. pnpm install --frozen-lockfile --offline in the worktree's frontend/**
- The worktree had no node_modules. The install used only the existing lockfile from the local store and added no package.

## Known Stubs

None.

## Threat Flags

None. The only new surface is in the threat model: T-08-02, the reflect/unsafe reach, which is type-checked and pinned.

## Verification

- `go build ./...`: OK. `go vet ./internal/relay/`: OK.
- The Task 1 verify passed: both widths with and without `-race`, plus the grep checks (disableAltScreenScrollback call present, "0.4-0.8 GiB" gone, vtalt.go present, vt version pinned in vtalt_test.go).
- The Task 2 verify passed: 7/7 TestSoftWrap_ in that set, and the history, full-replay, fidelity, render, alt-exit and alt-resize tests are green.
- The Task 3 verify passed:
  - 10/10 TestSoftWrap_;
  - `pnpm vitest run src/lib/__tests__/replayFidelity.test.ts`: 9 passed;
  - `pnpm exec tsc --noEmit`: OK;
  - the fixture is committed and `git diff --quiet` passes on it;
  - `frontend/src/wailsjs` is clean.
- **The one full run, `go test -race -short -count=1 ./internal/relay/`, ended 149 PASS, 1 SKIP, 1 FAIL.** The failure was TestHub_UnstrippedQueryNeverWedges/2_MiB_random_blob, which hit its 10 s timeout ("the live emulator is wedged").
  - **This is a pre-existing timing failure, not a regression from this plan.** Rerun alone on this branch it passes: 0.80 s plain, 8.48 s under `-race`, and 9.08 s under `-race` with wrap tracking forced off, so tracking adds nothing.
  - The base commit 292daeea (extracted with `git archive` into the scratchpad) fails the same subtest under `-race` 2 out of 2 times alone (10.0 s), while 08-03 and 08-04 run in parallel on the same machine.
  - The full suite was not rerun, per the once-per-plan rule. Logged here, not in deferred-items.md, which parallel plans may also write. It sits at the edge of the 10 s budget under `-race` on a loaded machine.

## Self-Check: PASSED

- FOUND: internal/relay/vtalt.go, internal/relay/vtalt_test.go, internal/relay/wraps.go, internal/relay/softwrap_test.go, frontend/src/lib/__tests__/fixtures/softwrap-replay.json
- FOUND commits: cbed6313, 8eab37a0, a1286014, 16543e5f, 8351b9fd
