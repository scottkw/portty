---
phase: 12-file-path-click
plan: 04
subsystem: relay
tags: [go, relay, path-click, preview, websocket-protocol, security]

requires:
  - phase: 12-file-path-click
    provides: "12-01 internal/previewfile (Open, sentinels) and CleanPreviewName; 12-03 cwd log, History.Pushed/FirstRow/Snapshot, absolute row scheme"
provides:
  - "internal/relay/pathclick.go: Hub.PathClick, the printed-token check with the full-row wrap fallback, cwd-at-print resolution, previewfile.Open delivery, unicast notices"
  - "MsgPathClick 0x13, MakePathClickFrame, PathClickMaxBytes, MetaPayload.PathClick and Notice, NoticePathUnsupported and NoticePathTooLarge"
  - "Preview.FocusSub: focus for the clicker without claiming the grid"
  - "Replay.PathClick capability bit on the replay meta (writable subscribers only)"
  - "frontend/src/lib/__tests__/fixtures/path-click.json: token, detection and frame cases shared with the viewer (12-06)"
  - "TestPathClick_SourceGate beside TestPreview_SourceGate"
affects: [12-05 share server arm and review, 12-06 viewer, 12-07 TESTING.md consolidation]

actuals:
  tokens: 13300
  tasks: 3
  commits: 3
plan_head_before: 88cb12ccec3953ca84febf48b92a157165e8f8e3
plan_head_after: 54779636f6a313c6637efca81703b797dfca50eb

tech-stack:
  added: []
  patterns:
    - "Snapshot under emuMu, search outside it (pathSnap copied rows, strip and scan with the lock released)"
    - "Resolve on its own goroutine behind an atomic one-at-a-time flag; drop, never queue"
    - "Test seam fields (pathClickResolved on Hub, testReadOnly on Server), nil/false in production"

key-files:
  created:
    - internal/relay/pathclick.go
    - internal/relay/pathclick_test.go
    - frontend/src/lib/__tests__/fixtures/path-click.json
  modified:
    - internal/relay/protocol.go
    - internal/relay/protocol_test.go
    - internal/relay/preview.go
    - internal/relay/server.go
    - internal/relay/cwd.go
    - internal/relay/hub.go
    - internal/daemon/preview_test.go

key-decisions:
  - "The capability bit is set in Hub.Join (one place) rather than inside joinLocked's several return paths; same value, same replay meta"
  - "Outcome code `missing` covers every non-notice open failure (missing, folder, unreadable): the log must not distinguish more than the user can see"
  - "The log line's bytes= is the file's byte count (also for too-large), never a name or path"
  - "Absolute clicks are used as is (no path.Clean); relative ones are joined and cleaned with the path package"

requirements-completed: [PATH-02]

coverage:
  - id: D1
    description: "A write-capable viewer's click on a printed absolute path previews it in every viewer, focus only for the clicker, grid untouched"
    requirement: PATH-02
    verification:
      - kind: unit
        ref: "internal/relay/pathclick_test.go#TestPathClick_LocalViewerAbsolute"
        status: pass
      - kind: unit
        ref: "internal/relay/pathclick_test.go#TestPathClick_NotPrintedAbsolute"
        status: pass
    human_judgment: false
  - id: D2
    description: "Printed-token check: whole-token rule, titles, RIS, trimmed history, soft wraps (first pass and full-row fallback), newest occurrence, relative and ~/ resolution"
    requirement: PATH-02
    verification:
      - kind: unit
        ref: "internal/relay/pathclick_test.go#TestPathClick_TokenTable"
        status: pass
      - kind: unit
        ref: "internal/relay/pathclick_test.go#TestPathClick_NotPrintedEdges"
        status: pass
      - kind: unit
        ref: "internal/relay/pathclick_test.go#TestPathClick_WrapJoin"
        status: pass
      - kind: unit
        ref: "internal/relay/pathclick_test.go#TestPathClick_WrapFallback"
        status: pass
      - kind: unit
        ref: "internal/relay/pathclick_test.go#TestPathClick_Relative"
        status: pass
      - kind: unit
        ref: "internal/relay/pathclick_test.go#TestPathClick_NewestOccurrenceWins"
        status: pass
    human_judgment: false
  - id: D3
    description: "Refusals, outcomes and notices: read-only (hub and relay pump), malformed, busy, freezing, ended; missing and folder silent; unsupported and too-large notify the clicker only; no path in logs"
    requirement: PATH-02
    verification:
      - kind: unit
        ref: "internal/relay/pathclick_test.go#TestPathClick_Refusals"
        status: pass
      - kind: unit
        ref: "internal/relay/pathclick_test.go#TestPathClick_RelayPumpReadOnly"
        status: pass
      - kind: unit
        ref: "internal/relay/pathclick_test.go#TestPathClick_Outcomes"
        status: pass
      - kind: unit
        ref: "internal/relay/pathclick_test.go#TestPathClick_NeverLogsPath"
        status: pass
      - kind: unit
        ref: "internal/relay/pathclick_test.go#TestPathClick_ReturnsAtOnce"
        status: pass
    human_judgment: false
  - id: D4
    description: "Capability bit, frame bytes and shared fixture; source gate keeps the click path off the file system"
    requirement: PATH-02
    verification:
      - kind: unit
        ref: "internal/relay/pathclick_test.go#TestPathClick_CapabilityBit"
        status: pass
      - kind: unit
        ref: "internal/relay/protocol_test.go#TestPathClickFrames_Fixture"
        status: pass
      - kind: unit
        ref: "internal/relay/pathclick_test.go#TestPathClick_DetectCasesAccepted"
        status: pass
      - kind: unit
        ref: "internal/daemon/preview_test.go#TestPathClick_SourceGate"
        status: pass
    human_judgment: false
  - id: D5
    description: "Soft-wrap fallback behaves on a real Mac terminal (C-09, 12-07)"
    requirement: PATH-02
    verification: []
    human_judgment: true
    rationale: "wraps.go's heuristic miss is reproduced in unit tests with synthetic bytes; a real shell and real wrapping is left to 12-07's manual checklist"

duration: 16min
completed: 2026-11-21
status: complete
---

# Phase 12 Plan 04: Host half of a path click Summary

**A write-capable viewer's MsgPathClick (0x13 plus the clicked text) now makes the host preview the file in every viewer, focused for the clicker, but only for text the session itself printed as a whole token, resolved against the directory in effect at the row it was printed, through `previewfile.Open`.**

## Performance

- **Duration:** about 16 min, of which the single full suite was about 6.5 min (relay 380 s)
- **Tasks:** 3 (task 1 tracer, tasks 2 and 3 auto, all tdd)
- **Files modified:** 10 (3 created, 7 modified)

## Accomplishments

- Wire: `MsgPathClick byte = 0x13`, `PathClickMaxBytes = 1024`, `MakePathClickFrame`, `MetaPayload.PathClick *bool` (`pathClick`) and `Notice *string` (`notice`), notice codes `path-unsupported` and `path-too-large`. Old hosts drop 0x13 (no case, no default arm in either pump).
- `Hub.PathClick(sub, payload) bool` refuses silently, cheapest first: nil or read-only subscriber; payload empty, over 1024 bytes, not UTF-8 or holding any `unicode.IsControl` character; under `h.mu` a closed, freezing or preview-streaming hub; then an atomic one-resolve-per-hub flag. Only then does it start the resolve goroutine and return true, so the read pump keeps answering pings.
- The resolve copies history rows plus the live screen (with the replay's wrap marks, the history-to-screen join, the live width and the cwd log) under `emuMu`, releases it, then strips and searches. Disk is touched only by `previewfile.Open`.
- `Preview.FocusSub`: when set, a header's focus is `sub == FocusSub`; otherwise the old owner rule. The click never calls `NoteTyping`, so it cannot move the grid.
- Replay meta carries `pathClick:true` for subscribers that can write (set in `Hub.Join`); the offset meta never does.

## Wire values, rule and codes (for 12-05, 12-06, 12-07)

- **Frame:** `0x13` + UTF-8 text. Fixture `frames` (text `docs/diagram.png`): click `13646f63732f6469616772616d2e706e67`; replay meta `{"replay":"full","pathClick":true}` under `0x20`; notice metas `{"notice":"path-unsupported"}` and `{"notice":"path-too-large"}` under `0x20`. Reversibility: costly (a byte and two meta fields shared across versions).
- **Token rule:** alphabet is Unicode letters and digits, `_ . ~ @ % + # -` and `/`. A match at offset i, length n is a token when the character before i is absent or outside the alphabet, and the character after is absent, outside the alphabet, or a `.` followed by absent or an outside character. `=` and `:` are outside, so `--out=/tmp/a.png` and `a/b.png:12:3` match; `/x/.ssh/id_rsa` is not a token of `/x/.ssh/id_rsa.pub`. The host also strips a trailing `:line[:col]` (at most two numeric groups, never reducing to empty).
- **Full-row wrap fallback** (runs only when the first pass, which joins rows by the hub's wrap marks, finds nothing): "full" is `lineFull` at the live width for screen rows and `ansi.StringWidth == live width` for history rows. Row k joins row k+1 when k is full and k+1 is not empty (never from the newest history row above an alt screen). A run may start at any row except one whose row above is full, wrap-marked and has different text. The run stops at the first non-full row or once it holds `len(row k)+len(text)+2` bytes, and the same whole-token rule applies to the run. Newest run wins; its row is the absolute row of the run's first row.
- **Resolution:** `/…` as is; `~/…` under `userHomeDir()` (a `var` in cwd.go); bare `~` and `~user` refused; anything else joined (path package) to the cwd log's newest record at or before the match row (alt-screen matches use the newest record); no record means refused, never seeded from the spawn directory. `..` is not a concern: authorisation is "this text was printed", not "inside a root".
- **Outcome codes and log line:** `viewer-diag: path click session=<DiagID> conn=<sub.ID> outcome=<code> bytes=<n>`, codes `preview`, `not-printed`, `no-cwd`, `missing`, `unsupported`, `too-large`, `send-failed`. Never text, path or name. Pre-goroutine refusals log nothing.
- **Notices:** `queueIfRoom` to the clicker only, under `h.mu`, only if still subscribed, not revoked and the hub open. Never broadcast.
- **Source gate:** `TestPathClick_SourceGate` (internal/daemon/preview_test.go): pathclick.go imports none of os, os/exec, io/fs, path/filepath, syscall, io/ioutil; previewfile.go no os/exec; pathclick.go is the only non-test relay file importing previewfile. `TestPreview_SourceGate` unchanged.

## Documented limits

- Hard-wrapped TUI paths (a program that breaks the line itself) are not joined: rows that are not full are never joined.
- The fallback judges history rows only at the live width: rows drawn before a resize are never full there and rely on the wrap marks alone (it can miss, never over-join).
- Where a full row has no wrap mark both readings count as printed.
- A path printed right below a hard row that took a wrap mark, and starting at its row edge, is still missed.
- Clicks that arrive while another click resolves, while a preview streams, while the hub freezes or after it ended are dropped, never queued; the user clicks again.
- Nested shells without OSC 7 (and shells other than the zsh shim, bash or fish with OSC 7 set up) give relative clicks no directory, so they do nothing.
- A resolve already running when the hub starts freezing still finishes; SendPreview does not check `freezing` (same as Phase 11).

## Test names (for 12-07's TESTING.md rows)

All in `internal/relay/pathclick_test.go` unless noted: `TestPathClick_LocalViewerAbsolute`, `_NotPrintedAbsolute`, `_ReturnsAtOnce`, `_RelayPumpReadOnly`, `_TokenTable`, `_DetectCasesAccepted`, `_Relative`, `_NewestOccurrenceWins`, `_Outcomes`, `_Refusals`, `_NotPrintedEdges`, `_WrapJoin`, `_WrapFallback`, `_StripsLineCol`, `_NeverLogsPath`, `_CapabilityBit`; `internal/relay/protocol_test.go`: `TestPathClickFrames_Fixture`; `internal/daemon/preview_test.go`: `TestPathClick_SourceGate`. New files needing a Section 4 traceability row: `internal/relay/pathclick_test.go` (PATH-02) and the fixture's consumers. TESTING.md was not edited, per the plan.

## Gate results

- `go build ./...`, `go vet ./...`, `bash tests/check-brand.sh`, `bash tests/check-traceability-paths.sh`: pass.
- One full `go test -race -short -count=1 -coverprofile=/tmp/p12-04-cover.out ./...` (log `/tmp/p12-04-gate-go.log`): every package `ok`, zero `--- FAIL` or `^FAIL` lines (the two TestCLILink login-shell tests fixed in ea7d5a38 pass). `internal/relay` 94.5% of statements.
- **`internal/relay/pathclick.go` statement coverage: 93.7%** (per block, deduplicated; `/tmp/p12-04-cover.txt`); the bar was 80%.
- **`--- SKIP` lines in the gate: none** (the gate runs without `-v`, which prints none).

## Task Commits

1. **Task 1: MsgPathClick, FocusSub, Hub.PathClick, relay pump arm** - `a01b36a4` (feat)
2. **Task 2: printed-token check, fallback, resolution, notices, refusals, fixture, source gate** - `ad5d5209` (feat)
3. **Task 3: capability bit on the replay meta, frame fixture** - `54779636` (feat)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking, instruction conflict] Commit attribution trailer**
- **Found during:** Task 1 commit
- **Issue:** The plan says commits end with `Co-Authored-By: Claude Opus 5.5`; the session's attribution instruction specifies `Claude Sonnet 5.5`.
- **Fix:** Used the session attribution line (it takes precedence).
- **Files modified:** none (commit messages only)

**2. [Plan wording] Capability bit set in `Join`, not `joinLocked`**
- The plan says "set in joinLocked". `joinLocked` has four return paths each building a `Replay`; setting it once in `Join` after the call gives the same result with one line. `Replay.Frames()` still emits it. Verified by `TestPathClick_CapabilityBit`.

**3. [Test-only] Two test fixture slips fixed while writing**
- The third WrapFallback placement first fed `P` without a trailing CRLF, so the following filler glued onto the path's last row; fixed in the test, no production change.

**Total deviations:** 1 instruction conflict, 1 wording difference, 1 test slip. **Impact:** none on deliverables. Tasks were written implementation-first rather than strict RED-first (the tracer slice was small and each test was run against the real code; the wrap-fallback tests assert the first pass alone refuses what the fallback accepts, which is the RED evidence for the fallback).

## Issues Encountered

None. The full suite takes about 6.5 minutes and ran once.

## Known Stubs

None.

## Threat Flags

None. The only new surface is the one in the plan's threat model (T-12-15 to T-12-24): read-only refusal at both pump and hub, printed-token check over stripped history and screen only, unicast notices, outcome-only logs, resolve off the pump goroutine, no file-system imports in pathclick.go. T-12-23 and T-12-24 are accepted as planned and belong in 12-05's review.

## Next Phase Readiness

12-05 adds the share server's `MsgPathClick` arm (call `hub.PathClick(sub, payload)` for non-read-only subscribers), the API version bump and the review section. 12-06 reads `path-click.json` (tokens, detect and frames) and offers links only when the replay meta says `pathClick:true`. 12-07 needs the TESTING.md rows above and manual item C-09 (real-shell soft-wrap check).

## Self-Check: PASSED

- Created files present: internal/relay/pathclick.go, internal/relay/pathclick_test.go, frontend/src/lib/__tests__/fixtures/path-click.json.
- Commits a01b36a4, ad5d5209, 54779636 are ancestors of HEAD; `git rev-list --count` from the ledger base gives 3.
