---
phase: 12-file-path-click
plan: 03
subsystem: relay
tags: [go, osc7, working-directory, relay-hub, handoff, gob]

requires:
  - phase: 12-file-path-click
    provides: "12-02 zsh ZDOTDIR shim that emits OSC 7 from Portty zsh sessions"
provides:
  - "internal/relay/cwd.go: parseOSC7, sameHost, CwdRecord and the bounded row-keyed cwd log (add, at, clear, records, load, checkCwdRecords)"
  - "History.Pushed, History.FirstRow, History.Snapshot: the absolute row scheme 12-04 resolves clicks against"
  - "OSC 7 handler in newLiveEmulator (beside OSC 8), Hub-owned cwds log, RIS clears it"
  - "HubState.Cwds and HubState.HistoryPushed: the log crosses an upgrade handoff, additive gob fields"
affects: [12-04 click resolver, 12-05 review, 12-07 TESTING.md consolidation]

actuals:
  tokens: 7800
  tasks: 3
  commits: 3
plan_head_before: 23fd05ee6342148ec65a3e04a6e880ddf2f36d9c
plan_head_after: 3d8348a17e2855508d8583c43b33b77c217c11af

tech-stack:
  added: []
  patterns:
    - "Absolute output rows: History.Pushed() + emulator scrollback + cursor row at report time; history line i is FirstRow()+i, screen row y is Pushed()+y after the drain"
    - "Hub-owned state under emuMu that survives emulator rebuilds (cwds beside cs, modes, wraps)"
    - "Additive gob fields with zero-value meaning 'older daemon'"

key-files:
  created:
    - internal/relay/cwd.go
    - internal/relay/cwd_test.go
  modified:
    - internal/relay/hub.go
    - internal/relay/history.go
    - internal/relay/handoff_state.go
    - internal/relay/handoff_state_test.go

key-decisions:
  - "OSC 7 is parsed by our own handler on the emulator write (data includes the leading 7;), not vt's WorkingDirectory callback, which drops payloads with a second ';' and reports after the write"
  - "The log prunes against the history as of the report's frame, so records older than the retained history can linger for one frame; lookups are unaffected and the caps bound the log"
  - "The 2048-byte cap applies to the URL after 7;, the 1024-byte cap to the decoded path"

patterns-established:
  - "Test helper cwdOf resolves a marker to its absolute row through History.Snapshot and the screen, never a hard-coded row"

requirements-completed: [PATH-01]

coverage:
  - id: D1
    description: "A shell's OSC 7 is recorded against the row it was printed at and resolves after the line scrolls into history, through resize, ESC[3J, trimming, alt screen and RIS"
    requirement: PATH-01
    verification:
      - kind: unit
        ref: "internal/relay/cwd_test.go#TestCwd_HubRecordsAtPrintRow"
        status: pass
      - kind: unit
        ref: "internal/relay/cwd_test.go#TestCwd_SameReadOrdering"
        status: pass
      - kind: unit
        ref: "internal/relay/cwd_test.go#TestCwd_SurvivesResizeAndClear"
        status: pass
      - kind: unit
        ref: "internal/relay/cwd_test.go#TestCwd_HistoryTrimKeepsFloor"
        status: pass
      - kind: unit
        ref: "internal/relay/cwd_test.go#TestCwd_AltScreenIgnored"
        status: pass
      - kind: unit
        ref: "internal/relay/cwd_test.go#TestCwd_RISClears"
        status: pass
    human_judgment: false
  - id: D2
    description: "Malformed, foreign-host, oversized and control-character reports are ignored; the log is bounded and ordered"
    requirement: PATH-01
    verification:
      - kind: unit
        ref: "internal/relay/cwd_test.go#TestParseOSC7"
        status: pass
      - kind: unit
        ref: "internal/relay/cwd_test.go#TestCwdLog_Rules"
        status: pass
      - kind: unit
        ref: "internal/relay/cwd_test.go#TestCwdLog_Caps"
        status: pass
    human_judgment: false
  - id: D3
    description: "The log and row counter survive an upgrade handoff; older states import empty; malformed state is refused"
    requirement: PATH-01
    verification:
      - kind: unit
        ref: "internal/relay/handoff_state_test.go#TestHandoffState_CwdRoundTrip"
        status: pass
      - kind: unit
        ref: "internal/relay/handoff_state_test.go#TestHandoffState_CwdOldShape"
        status: pass
      - kind: unit
        ref: "internal/relay/handoff_state_test.go#TestHandoffState_CwdValidates"
        status: pass
    human_judgment: false

duration: 19min
completed: 2026-11-21
status: complete
---

# Phase 12 Plan 03: Hub OSC 7 working-directory log Summary

**The relay hub parses OSC 7 inside the live emulator write and keeps a bounded log of directory changes keyed to absolute output rows, so each retained line resolves to the directory in effect when it was printed, and the log crosses upgrade handoffs.**

## Performance

- **Duration:** about 19 min (most of it the single full suite run, 385 s for the relay package)
- **Tasks:** 3
- **Files modified:** 6 (2 new)

## Accomplishments

- OSC 7 handler registered beside OSC 8 in `newLiveEmulator`; it runs under emuMu in PTY byte order, returns false so vt's default still runs, and ignores reports while the alt screen is active.
- Row scheme: `History.Pushed() + emu.ScrollbackLen() + cursor Y` at the report. History line i has row `FirstRow()+i`; screen row y has row `Pushed()+y` after the drain. Verified through scrolling, resize (the resize pushes the visible rows into history under the same numbers), ESC[3J (vt blanks the screen, keeps the counter and the log), trimming and alt-screen exit. RIS clears the log.
- `History` gained a `pushed` counter (never reset by Clear), `Pushed`, `FirstRow` and `Snapshot` (cloned lines, wrap flags and first row, for 12-04 to search outside emuMu).
- Handoff: `HubState.Cwds []CwdRecord` and `HubState.HistoryPushed uint64`, additive gob fields. Older state (both zero) imports with an empty log and the counter at the line count. `validate` refuses relative, unclean, control-character or over-1024-byte directories, non-increasing rows, a row above `HistoryPushed + maxHandoffGrid`, over 1024 records or 64 KiB, and a counter below the line count.

## Row scheme, caps and parse rules (for 12-04, 12-05, 12-07)

- **Caps:** 1024 records, 64 KiB of directory bytes (oldest dropped first; `HandoffStateMaxBytes` unchanged); URL after `7;` at most 2048 bytes; decoded path at most 1024 bytes.
- **Log rules:** same directory twice makes one record; a report at or below an earlier record's row replaces every record at or after that row; lookup returns the newest record at or before the row; pruning keeps the newest record at or before the history's first row as the floor.
- **Parse rules:** `file:` scheme, host empty, `localhost` or this Mac (case-insensitive, trailing dot and `.local` tolerated), absolute path, no control character (`%00`, `%0A`, `%7F` decoded and rejected), `filepath.Clean`ed. A raw unencoded `%` makes `url.Parse` fail and the report is ignored: emitters that do not percent-encode are unsupported (the Portty shim always encodes). The host check is not a security boundary.
- **Test names for 12-07 TESTING.md rows (all in `internal/relay/`):** `cwd_test.go`: `TestCwd_HubRecordsAtPrintRow`, `TestParseOSC7`, `TestSameHost`, `TestCwdLog_Rules`, `TestCwdLog_Caps`, `TestCwd_SameReadOrdering`, `TestCwd_SurvivesResizeAndClear`, `TestCwd_HistoryTrimKeepsFloor`, `TestCwd_AltScreenIgnored`, `TestCwd_RISClears`; `handoff_state_test.go`: `TestHandoffState_CwdRoundTrip`, `TestHandoffState_CwdOldShape`, `TestHandoffState_CwdValidates`.

## Gate results

- `go build ./...`, `go vet ./...`, `bash tests/check-brand.sh`, `bash tests/check-traceability-paths.sh`: pass.
- One full `go test -race -short -count=1 -coverprofile=/tmp/p12-03-cover.out ./...` (log `/tmp/p12-03-gate-go.log`): every package passes except `internal/daemon`, whose two failures are environmental and not caused by this plan (see Deviations). `internal/relay` passes (94.0% of statements).
- **`internal/relay/cwd.go` statement coverage: 100.0%** (from `/tmp/p12-03-cover.out`, per block, deduplicated; figure in `/tmp/p12-03-cover.txt`); the bar was 80%.
- `--- SKIP` lines: none (the gate runs without `-v`, which prints none; `ownHostForms` skips only when `os.Hostname` fails, which it did not).

## Deviations from Plan

### Auto-fixed Issues

None needed in production code: no case in the Task 2 matrix exposed a bug in Task 1's cwd.go or handler. Test-expectation corrections made while writing the tables (not production fixes):

- `TestCwdLog_Rules`: a report whose firstRow is at or past every record makes the new record itself the floor, so the log holds just it (the first draft expected the previous record to stay).
- `TestCwd_SurvivesResizeAndClear`: a resize moves the visible rows into history (the screen starts empty), and vt's ESC[3J blanks the screen as well as history, so the test checks resolution after resize, then that output after ESC[3J still resolves to the directory in effect and that a new report lands on the right row.
- `TestCwd_HistoryTrimKeepsFloor`: a report prunes against the history as of its own frame, so the test adds one more report in a later frame before checking the pruned log.

### Out-of-scope finding (deferred, RULE 0 reported)

**`internal/daemon` TestCLILink_SurvivesZshLoginPathHelper and TestCLILink_SurvivesZshLoginPathHelperWithShim fail at the gate.** `command -v portty` returns `/usr/local/bin/portty`: the host now has an installed Portty (`/usr/local/bin/portty` is a symlink into `/Applications/Portty.app`, created 17:11 today), and macOS `path_helper` puts `/usr/local/bin` before the test's temp bin dir. 12-03 touches no `internal/daemon` file. Logged in `.planning/phases/12-file-path-click/deferred-items.md`; the plan's gate therefore reports 2 FAIL lines that this plan neither caused nor fixed.

**Total deviations:** 0 production fixes; 1 out-of-scope environmental finding. **Impact:** none on this plan's deliverables; the gate is green for everything this plan touches.

## Known Stubs

None.

## Threat Flags

None. T-12-11 through T-12-14 are mitigated as planned (parser checks and caps, validate on import, memory-only log, no directory in any log line); no new endpoint or file access.

## Next Phase Readiness

12-04 can read `History.Snapshot()` plus the screen under emuMu and resolve a clicked relative path with `cwds.at(row)`; with no OSC 7 the log is empty and only absolute paths resolve.

## Self-Check: PASSED

- internal/relay/cwd.go, cwd_test.go: present. Commits 0fb15f8e, f4ff7601, 3d8348a1 are ancestors of HEAD.
