---
phase: quick-261127-bb7
plan: 01
subsystem: desktop-sharing
tags: [ponytail-review, input, encoder, frontend]
status: complete
commits: 3
plan_head_before: cb12fb488fa559a542c2da5647acc64912789f2c
plan_head_after: b41055f005b2ae8be581990569fe37b1dff1ff84
actuals:
  tokens: 30000
  tasks: 3
  commits: 3
key-files:
  modified:
    - internal/desktop/input.go
    - internal/desktop/manager.go
    - internal/desktop/surface.go
    - internal/desktop/input_test.go
    - internal/desktop/export_input_test.go
    - internal/screencap/media_darwin.m
    - internal/screencap/input_darwin.go
    - internal/screencap/window_darwin.go
    - internal/screencap/pasteboard_darwin.go
    - internal/screencap/screencap_darwin_test.go
    - frontend/src/lib/desktopInput.ts
    - frontend/src/lib/desktopClient.ts
    - frontend/src/components/DesktopView.tsx
    - frontend/src/lib/__tests__/desktopInput.test.ts
requirements: [quick-261127-bb7]
---

# Quick 261127-bb7: Accepted Phase 20 ponytail-review fixes

Nine accepted review items applied in three atomic commits: five correctness fixes, one regression test, three duplicate-helper removals.

## Commits

| Task | Commit | Items |
|------|--------|-------|
| 1 internal/desktop | d47bd7eb | closed input channel cleared (identity-checked, ReleaseViewer still runs); Close nils `m.in.done`; `releaseAllViewers` uses `flushInput`; new `TestInput_RepeatAfterReleaseNotHeld` |
| 2 internal/screencap | f93c44dc | `rebuildWidth:height:` restores the old-size session on failure and returns the first error; one `bridgeErr` replaces `inputErr`/`windowErr`/`pasteboardErr` |
| 3 frontend | b41055f0 | held buttons reconciled against `ev.buttons` (chorded release); bar-menu state cleared when the bar unmounts; one exported `utf8Len` |

## Verification

- `go vet ./internal/desktop/ ./internal/screencap/`: clean
- `go test -race -count=1 ./internal/desktop/`: ok (no flake re-run needed)
- `go test -count=1 ./internal/screencap/`: ok; `go build ./...`: ok
- `pnpm exec tsc --noEmit`: clean; `vitest run desktop Desktop`: 12 files, 510 tests passed (509 + 1 new)
- `bash tests/check-brand.sh`: clean before each commit
- RED checked before each fix: the extended `TestInputTracer_KeyHeldReleasedOnClose` (1 open channel), the source check (0 `createSessionLocked` calls found) and the chorded-release vitest case (run against the HEAD `desktopInput.ts`) each failed without the change.

## Test coverage notes

- Item 5 (flush after Close) has no automated test: the only observable is `flushInput` not waiting 1 s, which would need a timing assertion.
- Item 1 (encoder rebuild) is covered by a source check only (`TestEncoder_RebuildFailureRestoresOldSize`); VideoToolbox failure cannot be forced.
- No TESTING.md change: no test file was added.

## Deviations from Plan

**1. [Rule 1 - Bug] Source check anchored on the method definition**
- **Found during:** Task 2
- **Issue:** `- (OSStatus)rebuildWidth:` also matches the `@interface` declaration at media_darwin.m L125, so the planned slice found no `createSessionLocked` calls. The plan's `awk` verify range has the same anchor (it starts at the declaration and still counts at least 2 only by running on into later code).
- **Fix:** The test anchors on `- (OSStatus)rebuildWidth:(int)w height:(int)h {`.
- **Files modified:** internal/screencap/screencap_darwin_test.go
- **Commit:** f93c44dc

Otherwise: plan executed as written. The existing bar-edge move test was given its realistic `{ button: -1, buttons: 1 }` as the plan instructed; no other test needed a change.

## Known Stubs

None.

## Threat Flags

None.

## Self-Check: PASSED

Commits d47bd7eb, f93c44dc and b41055f0 are on `gsd/phase-17-desktop-feasibility-spike`; modified files verified present; none of `.gitignore`, `.planning/state.json`, `.ignore`, `agenthub`, `.gitkeep` or `.claude/` were staged.
