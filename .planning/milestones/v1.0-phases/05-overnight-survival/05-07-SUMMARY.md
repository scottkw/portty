---
phase: 05-overnight-survival
plan: 07
subsystem: testing
status: complete
tags: [testing, regression-suite, traceability, manual-checklist, shar-07, verf-01]
requires:
  - "05-01..05-06: the Phase 5 test files and their SUMMARY notes for 05-07"
provides:
  - "TESTING.md Section 2: counts Go 118, vitest 83, Playwright 7, build-script 1, total 209, with a Phase 5 note"
  - "TESTING.md Section 4: 19 SHAR-07 rows and 2 VERF-01 rows"
  - "TESTING.md Section 5 Category AT: M-84..M-87 plus user-facing caveats; M-79 and M-83 reworded"
  - ".planning/todos/completed/2026-11-17-chunk-catch-up-replay.md (resolved by 05-01)"
  - ".planning/todos/pending/2026-11-17-alt-screen-scrollback-growth.md (RESEARCH Pitfall 3)"
affects: [phase-05-verification]
tech-stack:
  added: []
  patterns:
    - "VERF-01 rows are labelled accelerated stand-ins; the gate decision stays with the user"
key-files:
  created:
    - .planning/todos/pending/2026-11-17-alt-screen-scrollback-growth.md
    - .planning/todos/completed/2026-11-17-chunk-catch-up-replay.md
  modified:
    - TESTING.md
    - tests/check-share-removal.sh
decisions:
  - "The share-removal gate's /join route pattern now skips an underscore, so the required Section 4 path internal/relay/join_test.go is not read as the removed code-based-access route"
  - "The user's rock-solid requirement (glitches, network/AP switches, sleep/wake) and the two Tart VMs were folded into M-85..M-87 as steps, not as new requirements"
metrics:
  duration: 8min
  completed: 2026-11-17
  tasks: 2
  files: 5
estimate:
  tokens: 45000
  tasks: 2
actuals:
  tokens: 4900
  tasks: 2
  commits: 2
plan_head_before: 4935002e96ab4a0f4d0fc4b04c01a6cb69d0bff6
plan_head_after: b17029ba3be5ce38970e27b077a961f7749ea1cb
---

# Phase 5 Plan 07: Regression Suite Close-Out Summary

TESTING.md now covers Phase 5. The counts match `git ls-files` (Go 118, vitest 83, total 209, the same as the plan expected). A Phase 5 note lists the 7 new test files and the 17 files edited in place. Section 4 has 19 SHAR-07 rows and 2 VERF-01 rows. The VERF-01 rows are marked as accelerated stand-ins with the gate decision deferred. New Category AT holds M-84..M-87 and the caveats a user can trip over. The chunk-replay todo is closed and the alt-screen scrollback growth is recorded as a todo.

## What was built

- **Section 2:** updated counts and a `Portty Phase 5 (overnight survival, SHAR-07/VERF-01)` note: the new files, the files edited in place (with the new test names), one sentence for each production change, and the Section 4/5 changes.
- **Section 4:**
  - SHAR-07 rows for every new Phase 5 test file and every file whose cases were rewritten for reconnect. That is the plan's 19 files.
  - VERF-01 rows for `attach_viewers_test.go` (ReconnectsWithDelta scrollback diff, ViewingDaemonRestart, IdleNoReconnecting) and `history_test.go` (history after the ring wraps).
  - The SHAR-06 `hub_test.go` row now uses the 05-01 rename `TestFullReplay_StripsTerminalQueries`.
  - The SHAR-04 guestClose row was left as it is: it never said "any other close".
- **Section 5, Category AT:**
  - **M-84:** overnight two-Mac test, VERF-01 as written. The office Mac stays awake through a foreground `caffeinate -dims` stopped with Ctrl-C, with no settings changes. Checks in the morning: typing works in the GUI and the CLI, yesterday's scrollback is there, a browser opens the session, and the daemon.log `replay mode=` and `heartbeat timeout` lines are read.
  - **M-85:** two-Mac lid sleep for 10+ minutes, Wi-Fi off for 20 s, and a switch to another network or access point. Includes the 05-06 note that the bar may flip from [reconnecting] to connected for a moment while the home daemon cannot reach the host.
  - **M-86:** VM browser freeze with `kill -STOP` / `-CONT`, plus a two-VM variant (portty-test as office, portty-test-2 as home).
  - **M-87:** `tart suspend` clock jump, with a kill -STOP fallback.
  - **Caveats paragraph:** zsh `clear` empties the history; history is memory-only; Share turned off while a browser sleeps leaves "Connection lost — reconnecting…" up indefinitely; both Macs need the same build; the office Mac is kept awake by hand (CONV-01, v2); a watchdog wait of up to about 45 s after wake.
  - M-84 and M-85 are marked deferred-capable.
- **M-79** now says a stopped share is final and a network drop reconnects. **M-83** now says an idle bar never shows `[reconnecting]` (UAT gap 1, fixed in 05-06).
- **Todos:** `gsd-tools todo complete` moved the chunk todo to completed, and a Resolution section was added (ReplayChunk 16 KiB, ViewerReadLimit kept for live 32 KiB frames). The new alt-screen todo has `area: relay` and `files: internal/relay/hub.go`, and asks for a heap soak test with the fix.

## Verification

- Task 1 gate:
  - `check-traceability-paths.sh` (119 paths), `check-brand.sh`, `check-share-removal.sh`, `go build`, `go vet` and `go test -race -short -skip '...' ./...` all passed (all packages ok, no FAIL lines). tsc was clean.
  - vitest: the first full run had 1 failed file out of 83. Its name was not captured, so it is unconfirmed whether it was one of the known load flakes in deferred-items.md. This plan changed no frontend code, and the rerun passed 83/83 files (1119 tests).
  - 19 SHAR-07 rows (at least 15 required) and 2 VERF-01 rows (at least 2 required).
  - Tracer gate: the automated verify was re-run in full after the share-removal fix and passed (human_verify_mode end-of-phase), so Task 2 went ahead.
- Task 2 verify: all checks passed (`VERIFY_OK`). `Category AT — Overnight Survival` 1, 4 M-84..M-87 items, `caffeinate -dims` present, `known no-heartbeat indicator` 0.
- Guards:
  - `git diff --name-only fb268a0a -- .planning/REQUIREMENTS.md` printed nothing.
  - The ROADMAP numbered-criterion diff count is 0.
  - `git grep -E 'pmset|caffeinate' -- '*.go' '*.ts' '*.tsx'` printed nothing.
- VERF-01 and SHAR-07 were not marked complete (orchestrator instruction).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] The share-removal gate flagged the required join_test.go row**
- **Found during:** Task 1 gate
- **Issue:** `tests/check-share-removal.sh` scans TESTING.md Sections 4-6 for the removed `/join` route with `/j[o]in([^a-z]|$)`. The path `internal/relay/join_test.go`, which the plan requires in Section 4, matched on `/join_`.
- **Fix:** the pattern is now `/j[o]in([^a-z_]|$)`, with a one-line comment. Still caught: `GET /join` and `/join/abc`. No longer caught: `relay/join_test.go` (checked with grep).
- **Files modified:** tests/check-share-removal.sh (not in the plan's file list)
- **Commit:** eab95b3a

**2. [Scope] Extra wording from the orchestrator folded into the existing M-items**
- M-85 gained the Wi-Fi network / access-point switch and the 05-06 bar-flip note.
- M-86 and M-87 gained the two-VM variant (portty-test / portty-test-2 on the tailnet).
- The caveats gained the watchdog wake delay (todo `network-resilience-matrix`, left pending).
- No new requirement was added.

## Known Stubs

None.

## Notes for verification

- Claude runs M-86 and M-87 in the Tart VMs (Safari through dev-browser). The user picks M-84 or M-85 as the VERF-01 gate, or defers both.
- These pending todos were left alone, as instructed: jumbled-screen-on-reattach, duplicate-remote-tab-blank-window, guest-scrollbar-at-pane-edge, network-resilience-matrix, pin-go-toolchain-for-tailscale.

## Self-Check: PASSED

- FOUND: TESTING.md, tests/check-share-removal.sh, .planning/todos/completed/2026-11-17-chunk-catch-up-replay.md, .planning/todos/pending/2026-11-17-alt-screen-scrollback-growth.md
- MISSING (expected): .planning/todos/pending/2026-11-17-chunk-catch-up-replay.md
- FOUND commits: eab95b3a, b17029ba
