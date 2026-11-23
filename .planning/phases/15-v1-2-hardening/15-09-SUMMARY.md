---
phase: 15-v1-2-hardening
plan: 09
subsystem: regression-home-and-phase-close
tags: [testing-md, full-suite, todos, manual-checks]
requires:
  - phase: 15-01..15-08
    provides: the tests and fixes TESTING.md records and the full run covers
provides:
  - "TESTING.md Phase 15 Section 2 note and Section 5 Category BF (M-152..M-155)"
  - "One clean full Go (-race) and vitest run on the phase's final tree"
  - "Nine fixed todos moved to .planning/todos/completed/ with Resolution sections"
affects: [/gsd-verify-work (runs M-152..M-155 before /gsd-ship)]
status: complete
actuals:
  tokens: 9500
  tasks: 3
  commits: 2
plan_head_before: b153acaa050ab43a635a84c5410972cf9d29a5a4
plan_head_after: 8659af70f54e36a8cb874c845f42a2a5fbc98087
commits: 2
tech-stack:
  added: []
  patterns: []
key-files:
  created: []
  modified:
    - TESTING.md
    - .planning/todos/completed/ (nine moved todos)
key-decisions:
  - "No new test files in Phase 15, so Section 2 counts stay Go 174, vitest 122, Playwright 10, build-script 1, total 307 and Section 4 is unchanged"
  - "M-154 keeps its sidebar PASS clause because 15-07 reads Sidebar branch: fixed"
requirements-completed: []
---

# Phase 15 Plan 09: Close the phase Summary

TESTING.md now records Phase 15 (counts note plus Category BF with four manual items, all "not yet run"), the full Go `-race` and vitest suites passed once with no re-run, and the nine fixed todos are in completed with their resolutions.

## Sidebar branch and SC2

Quoted from 15-07-SUMMARY.md: `Sidebar branch: fixed`.

ROADMAP SC2 is COVERED: the ended peer tab by 15-05, the sidebar "Nothing shared" instead of "Can't reach this Mac" by 15-07. M-154's sidebar PASS clause stands as written and no split todo exists.

## Task Commits

1. Task 1 (tracer): `5c74ccc7` docs(15-09) TESTING.md Phase 15 note and Category BF. The traceability script printed `OK: 228 traceability paths exist`; every grep in the task's verify is at least 1 (`origin mode` and `does not match the socket peer` 2 each, `passive-watch` 1). The Phase 15 note (line 36) sits above the Phase 14 note (line 38).
2. Task 2: no commit (no repository files). The single clean run is below.
3. Task 3: `8659af70` docs(15-09) nine todos moved with Resolution sections.

## Task 2: the single clean run

Log directory: `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.xyoq6l1tTw` (`t2-status.log`, `t2-go-full.log`, `t2-vitest-full.log`, `t2-vet.log`, `t2-tsc.log`, `t2-pkgs.log`, `t2-gofmt.log`). Each command ran exactly once, in the plan's order, with `GOTOOLCHAIN=go1.26.11`. No test failed and nothing was re-run.

| Command | Exit |
|---------|------|
| `gofmt -l` over the Go files changed since 09c6c07a (`git diff --name-only 09c6c07a -- '*.go'`) | 0, no output |
| `go vet ./...` | 0 |
| `go test -race -short -count=1 -timeout 30m ./...` | 0 |
| `pnpm --dir frontend exec tsc --noEmit` | 0 |
| `pnpm --dir frontend test` | 0 |

- `go list` count of packages with tests: 14. `ok` lines in `t2-go-full.log`: 14 (equal). `--- FAIL` or `^FAIL` lines: 0. Slowest packages: internal/relay 202.4 s, internal/daemon 96.4 s.
- Vitest: `Test Files 122 passed (122)`, `Tests 1825 passed (1825)`, duration 32.34 s. The `SharePopover > Copy writes the full link and reads Copied! for 2 s` flake seen once during 15-04 did not appear in this run.
- Section 2's counts match `git ls-files`: 174 Go test files, 122 vitest files, 10 Playwright specs (recounted before editing).

## Task 3: todos

Moved with `git mv` after the clean run, each with `## Resolution` naming its plan and commits: attach-statusbar-overwritten (15-01), ended-peer-tab-banner (15-05 and 15-07), killsession-stale-pgid (15-02), daemon-socket-hardening (15-03 and 15-08), update-notification-click (15-06), row0-relative-path-start-dir (15-04), first-cmd-click-same-cell-after-leave (15-04), linkifier-reconnect-flake (15-04), paste-backlog-flake (15-02). `2026-11-21-dompurify-3418.md` and `2026-11-21-v12-release-checks.md` remain in pending (2 files there). The task's verify loop printed `VERIFY-OK`.

## Deviations from Plan

None - plan executed exactly as written. The Section 2 note lists the tests from the eight SUMMARYs; every named test function was confirmed to exist in the named file with `git grep` before it went into TESTING.md.

## Still open for /gsd-verify-work (not run here)

M-152 (notification click), M-153 (attach bar with full-screen apps, passive-watch observation, `--status-top` accepted limit), M-154 (ended peer tab and sidebar over the real tailnet) and M-155 (real-process upgrades in `portty-test`: TestHandoffIT `upgrade`, `fault`, `from-v1.1`, plus the P to A to B chain with socket 755, 600, 600 and no `does not match the socket peer` line) are recorded as "_Last run:_ not yet run". M-155 must PASS before /gsd-ship; a FAIL is a ship blocker. None ran on the host.

## Known Stubs

None.

## Threat Flags

None.

## Self-Check: PASSED

- `TESTING.md`: FOUND; commits `5c74ccc7` and `8659af70` are ancestors of HEAD.
- All nine files under `.planning/todos/completed/` FOUND; none left in `.planning/todos/pending/`.
