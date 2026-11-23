---
phase: 15-v1-2-hardening
plan: 04
subsystem: relay, frontend path click
tags: [go, relay, history, cwd, xterm-linkifier, vitest, path-click]

requires:
  - phase: 12
    provides: path click (PATH-01, PATH-02), the cwd log keyed on absolute output rows
provides:
  - "History.Skip(n): lines dropped before they reached history still count in Pushed"
  - "ED 3 handler counts the emulator scrollback it drops, so absolute rows stay monotonic (F-1)"
  - "mouseleave invalidates the xterm Linkifier's same-cell memory (F-2)"
  - "linkifier test file waits on events, not wall-clock sleeps"
affects: [15-09 TESTING.md traceability]

actuals:
  tokens: 2600
  tasks: 3
  commits: 3
plan_head_before: 923540e9307ec1f8372ed027ee35967a9522d0d3
plan_head_after: 21099b0778776041299b38eae38e366d346603f0
commits: 3

tech-stack:
  added: []
  patterns:
    - "Row numbering is fixed at the source (History.Pushed) rather than papering over it in the cwd lookup"

key-files:
  created: []
  modified:
    - internal/relay/history.go
    - internal/relay/hub.go
    - internal/relay/cwd_test.go
    - internal/relay/pathclick_test.go
    - frontend/src/lib/pathLinks.ts
    - frontend/src/lib/__tests__/pathLinks.linkifier.test.ts

key-decisions:
  - "F-1 is fixed in row numbering (History.Skip in the CSI J handler), not in Hub.PathClick or cwdLog.at, which stay unchanged"
  - "The 20 ms no-input window in the mouse-tracking linkifier test stays a sleep: nothing marks 'nothing happened'"

requirements-completed: []

coverage:
  - id: D1
    description: "A relative path printed on row 0 after cd proj; clear previews the file under proj (F-1)"
    verification:
      - kind: unit
        ref: "internal/relay/pathclick_test.go#TestPathClick_RelativeAfterClear"
        status: pass
      - kind: unit
        ref: "internal/relay/cwd_test.go#TestCwd_ClearKeepsRowsMonotonic"
        status: pass
      - kind: unit
        ref: "internal/relay/cwd_test.go#TestCwd_ScrollbackEraseKeepsRows"
        status: pass
    human_judgment: false
  - id: D2
    description: "First Cmd-click after leaving and returning to the same cell with changed text sends the new path (F-2)"
    verification:
      - kind: unit
        ref: "frontend/src/lib/__tests__/pathLinks.linkifier.test.ts#after leaving and returning to the same cell with changed text"
        status: pass
    human_judgment: false
  - id: D3
    description: "pathLinks.linkifier.test.ts is deterministic: 20 of 20 alone and once beside a full vitest run"
    verification:
      - kind: other
        ref: "20 sequential vitest runs of the file, plus one run beside pnpm --dir frontend test"
        status: pass
    human_judgment: false

duration: 14min
completed: 2026-11-23
status: complete
---

# Phase 15 Plan 04: Path click F-1, F-2 and the linkifier flake Summary

**Absolute output rows stay monotonic across ED 3 (History.Skip counts the dropped emulator scrollback), the first Cmd-click after leave-and-return to a changed cell works (mouseleave calls invalidate), and the real-Linkifier tests wait on events instead of sleeps.**

## Performance

- **Duration:** 14 min
- **Started:** 2026-11-23T14:22:00Z
- **Completed:** 2026-11-23T14:37:00Z
- **Tasks:** 3
- **Files modified:** 6

## Accomplishments

- F-1: `clear` writes ED 2 then ED 3 in one write. ED 2 pushes the screen into the emulator's scrollback and ED 3 drops it before it drains into History, so `Pushed()` never counted those lines and output after the clear reused rows older cwd records cover. `History.Skip(emu.ScrollbackLen())` in the 'J' handler (n == 3) keeps rows monotonic. `Hub.PathClick` and `cwdLog.at` are untouched.
- F-2: `onLeave` now sets `pointerInside = false` and calls `invalidate()` (declared after `invalidate`, no re-hover while outside).
- Flake: `renderOnce`, `resizeTo`, `dropAndReturn`, `beforeEach` and the URL case use `vi.waitFor` or `renderOnce`; one 20 ms sleep remains.

## Task Commits

1. Task 1 (tracer): `dc81e940` fix(15-04): rows stay monotonic across ED 3
2. Task 2: `a1143a00` fix(15-04): mouseleave resets the Linkifier's same-cell memory
3. Task 3: `21099b07` test(15-04): linkifier tests wait on events

## TDD evidence (RED before GREEN)

- Go, unfixed code: `TestCwd_ClearKeepsRowsMonotonic` FAIL ("docs-x resolves to /start, want /proj"); `TestPathClick_RelativeAfterClear` FAIL (outcome "missing", want "preview"); `TestCwd_ScrollbackEraseKeepsRows` FAIL after the fixture adjustment below. All pass after the fix.
- Vitest, unfixed pathLinks.ts: the new F-2 case FAILED (`['docs/diagram.png']` received, two entries expected); 10 of 10 pass after.
- Verification logs: Task 1 in `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.psJoWQRsiJ` (`go vet`, scoped tests, `go test -race -short -count=1 ./internal/relay/...` ok in 207 s); Task 2 in `.../tmp.jF7MwK0NkG`; Task 3 loop in `.../tmp.7gLtCplXOR`, load runs in `.../tmp.LV9K7GokIY` and `.../tmp.tIBADkjkH6`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] TestCwd_ScrollbackEraseKeepsRows fixture passed on unfixed code**
- **Found during:** Task 1 (RED run)
- **Issue:** The plan's fixture put the `/proj` report on row 1, before the filler. Unfixed, docs-z's row fell back to row 9, still past that record, so the test proved nothing.
- **Fix:** Moved the `/proj` report after the twelve filler lines (on a row the dropped lines pushed past). Unfixed it now resolves to `/start`; fixed it resolves to `/proj`.
- **Files modified:** internal/relay/cwd_test.go
- **Commit:** dc81e940

**2. [Rule 3 - Blocking] Task 3 verify greps ANSI-coloured vitest output**
- **Found during:** Task 3
- **Issue:** `grep -cE 'Test Files +1 passed'` never matches vitest's coloured log, so the plan's loop check failed even though all 20 runs passed.
- **Fix:** Counted with colour codes stripped (`sed 's/\x1b\[[0-9;]*m//g'`): 20 of 20 passed. Later runs checked the same way.

### Out-of-scope observation

The first full `pnpm --dir frontend test` run beside the linkifier test failed one unrelated test, `SharePopover > Copy writes the full link and reads Copied! for 2 s` (timing under load). It passes alone and the full run passed 122 of 122 on a second attempt. Not caused by this plan; not fixed (scope boundary). Candidate for a later flake pass.

**Total deviations:** 2 auto-fixed. **Impact:** none on product behaviour.

## Known Stubs

None.

## Threat Flags

None. T-15-10 (rows resolve against the directory printed in) is mitigated by `TestPathClick_RelativeAfterClear`; `Hub.PathClick`'s printed-token check and previewfile checks are unchanged.

## Next Phase Readiness

Ready for 15-05. 15-09 should add TESTING.md Section 4 rows for the new test files' coverage (no new files here; the tests live in existing files).

## Self-Check: PASSED

- Commits `dc81e940`, `a1143a00`, `21099b07` are ancestors of HEAD.
- Acceptance greps: `History.Skip` 1, `history.Skip(emu.ScrollbackLen())` 1, `TestPathClick_RelativeAfterClear` 1, `TestCwd_ClearKeepsRowsMonotonic` 1, `pointerInside = false; invalidate()` 1, `await sleep(` 1, `vi.waitFor` 4.
