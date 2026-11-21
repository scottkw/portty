---
phase: 12-file-path-click
plan: 01
subsystem: preview
tags: [go, previewfile, relay, portty-open, refactor]

requires:
  - phase: 11-file-preview
    provides: "portty open file checks (cmd_open.go) and the daemon preview route"
provides:
  - "internal/previewfile: Open, MIME, Key, ErrNotExist/ErrDir/ErrUnsupported/ErrTooLarge, TooLargeError, File"
  - "relay.CleanPreviewName, shared by the daemon preview route and the path click (12-04)"
affects: [12-04 relay path click, 12-07 TESTING.md consolidation]

actuals:
  tokens: 5700
  tasks: 2
  commits: 2
plan_head_before: 00c4f8f7b5212a4aa3e9daebcf1d44c6c4f14efe
plan_head_after: 806f611b42c7778e81d9ec7461e5a8e5fa65390e

tech-stack:
  added: []
  patterns:
    - "Leaf package for host file reads: caller passes the size cap, package imports nothing of this module"

key-files:
  created:
    - internal/previewfile/previewfile.go
    - internal/previewfile/previewfile_test.go
  modified:
    - cmd_open.go
    - cmd_open_test.go
    - internal/daemon/preview.go
    - internal/relay/preview.go

key-decisions:
  - "previewfile.Open takes maxBytes as an argument so it never imports relay (relay imports previewfile in 12-04)"
  - "CleanPreviewName lives in relay, not previewfile: the name is part of the preview wire contract and previewfile stays a pure file reader"
  - "Moved the code unchanged (Chesterton's Fence); the research's early 8 KiB unsupported read was not taken"

requirements-completed: [PATH-02]

coverage:
  - id: D1
    description: "portty open reads through internal/previewfile and prints the same lines as before"
    requirement: PATH-02
    verification:
      - kind: unit
        ref: "cmd_open_test.go#TestCmdOpen_* (go test -race -short -run CmdOpen .)"
        status: pass
    human_judgment: false
  - id: D2
    description: "previewfile.Open applies the CLI check order, type table and symlink-resolved key; imports no module package"
    requirement: PATH-02
    verification:
      - kind: unit
        ref: "internal/previewfile/previewfile_test.go#TestOpen_Order"
        status: pass
      - kind: unit
        ref: "internal/previewfile/previewfile_test.go#TestOpen_Types"
        status: pass
      - kind: unit
        ref: "internal/previewfile/previewfile_test.go#TestOpen_NoModuleImports"
        status: pass
    human_judgment: false
  - id: D3
    description: "CleanPreviewName moved to relay; daemon route uses it; source gate unchanged"
    requirement: PATH-02
    verification:
      - kind: unit
        ref: "internal/daemon/preview_test.go#TestPreview_SourceGate"
        status: pass
      - kind: unit
        ref: "internal/daemon/preview_test.go#TestPreview_NameIsPercentDecodedAndCleaned"
        status: pass
    human_judgment: false

duration: 20min
completed: 2026-11-21
status: complete
---

# Phase 12 Plan 01: previewfile package and CleanPreviewName Summary

**`portty open`'s file checks now live in the leaf package internal/previewfile (stat, non-blocking open, descriptor re-check, bounded read, type sniff, symlink-resolved key), with every CLI line unchanged, and the preview name cleaner moved to relay as CleanPreviewName.**

## Performance

- **Duration:** about 20 min (the single full suite took about 4 min of that)
- **Completed:** 2026-11-21T22:32Z
- **Tasks:** 2
- **Files modified:** 6 (2 created, 4 modified)

## Accomplishments

- One implementation of the `portty open` file checks that the relay hub (12-04) can import without a cycle.
- No CLI behaviour change: all `TestCmdOpen_*` pass; the only test edits are two retargeted call sites.
- `CleanPreviewName` exported from relay, daemon route calls it, `TestPreview_SourceGate` untouched and passing.

## previewfile API (for 12-04)

- `Open(path string, maxBytes int64) (File, error)`; `File{Name, MIME, Key string; Data []byte}` (Name is the base name).
- `MIME(name string, data []byte) string` ("" is unsupported); `Key(path string) (string, error)`; `hasSVGRoot` unexported.
- Sentinels: `ErrNotExist`, `ErrDir`, `ErrUnsupported` (not a regular file, or no supported type), `ErrTooLarge`.
- `TooLargeError{Size int64}` unwraps to `ErrTooLarge` (use `errors.As` for the size).
- Any other stat, open, fstat, read or key failure is returned as the OS error itself, unwrapped.

## CleanPreviewName decision

Moved from `internal/daemon/preview.go` to `internal/relay/preview.go` verbatim (exported, doc comment updated). Relay was chosen over previewfile because the name is part of the preview wire contract (`Preview.validate`), previewfile stays a pure file reader, and the daemon route then imports nothing that opens files. relay/preview.go gained only the `strings` import (it already had `unicode` and `unicode/utf8`); no `os`, `io/fs` or `path/filepath`.

## Test names (for 12-07's TESTING.md rows)

- `internal/previewfile/previewfile_test.go`: `TestOpen_Order`, `TestOpen_TooLargeSize`, `TestOpen_Types`, `TestOpen_KeyFollowsSymlinks`, `TestOpen_NoModuleImports`.
- Retargeted, same names: `TestCmdOpen_TypeDetection` (calls `previewfile.MIME`), `TestCmdOpen_SameFileSameKey` (calls `previewfile.Key`).
- Unchanged and passing: `TestPreview_SourceGate`, `TestPreview_NameIsPercentDecodedAndCleaned`.
- New file needing a Section 4 traceability row: `internal/previewfile/previewfile_test.go` (PATH-02). TESTING.md was not edited, per the plan.

## Gate results

- `go build ./...`, `go vet ./...`, one `go test -race -short -count=1 -coverprofile=/tmp/p12-01-cover.out ./...` all passed (log `/tmp/p12-01-gate-go.log`); `check-brand.sh` and `check-traceability-paths.sh` exit 0.
- **previewfile.go statement coverage: 90.1%** (`/tmp/p12-01-cover.txt`; package figure in the log is also 90.1%).
- **`--- SKIP` lines in the gate: none.**

## Task Commits

1. **Task 1: previewfile package, cmd_open.go mapping, moved tests** - `34bc8162` (refactor)
2. **Task 2: CleanPreviewName in relay, daemon route uses it, full gate** - `806f611b` (refactor)

## Decisions Made

See key-decisions. Task 1 was a pure move of already-tested code, so the new tests passed on first run rather than showing a RED step; `TestCmdOpen_*` was the regression that proved the move.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking, instruction conflict] Commit attribution trailer**
- **Found during:** Task 1 commit
- **Issue:** The plan says to end commits with `Co-Authored-By: Claude Opus 5.5`; the session's attribution instruction specifies `Claude Sonnet 5.5`.
- **Fix:** Used the session attribution line (it takes precedence over the plan text).
- **Files modified:** none (commit messages only)

---

**Total deviations:** 1 (a trailer wording difference). **Impact on plan:** none.

## Issues Encountered

None. The full suite takes about 4 minutes (relay 222 s, daemon 82 s) and ran once, as the plan requires.

## Known Stubs

None.

## Threat Flags

None. previewfile gained no new caller besides the CLI; the one relay caller arrives in 12-04 (T-12-02 stays that plan's gate).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

12-04 can call `previewfile.Open(path, relay.PreviewMaxBytes)` and `relay.CleanPreviewName`. 12-07 needs the TESTING.md Section 2 and 4 rows listed above.

---
*Phase: 12-file-path-click*
*Completed: 2026-11-21*
