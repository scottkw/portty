---
phase: 15-v1-2-hardening
plan: 01
subsystem: attach-cli
tags: [go, attach, statusbar, resize, grid-owner]
requires:
  - phase: 10-input
    provides: grid-owner model and attach SIGWINCH claim (INPT-06)
provides:
  - "portty attach reports and claims rows-1 while its status bar is sized"
  - "attach.BarRows, Bar.ReservedRows, initialResizeFrame"
affects: [15-09 TESTING.md M-153]
status: complete
actuals:
  tokens: 3200
  tasks: 2
  commits: 2
plan_head_before: 3dcc612b2f1d58c5e356fe2cfbc5e924aa910e7a
plan_head_after: fb0c1f2dd88c30e1abbc15230a0ecf5c5d5a706a
commits: 2
tech-stack:
  added: []
  patterns: ["reserve func() int read per signal and per connection"]
key-files:
  created: []
  modified:
    - internal/attach/attach.go
    - internal/attach/attach_unix.go
    - internal/attach/attach_test.go
    - internal/statusbar/bar.go
    - internal/statusbar/bar_test.go
    - cmd_attach.go
    - cmd_attach_resize.go
    - cmd_attach_test.go
    - internal/relay/hub_grid_owner_test.go
key-decisions:
  - "watchResize takes a reserve func so the subtraction is tested through the real SIGWINCH loop (deviation from RESEARCH Pitfall 11, as the plan states)"
  - "ReservedRows is 1 only once Bar.Start stored a size; nil bar reports 0"
requirements-completed: []
---

# Phase 15 Plan 01: Attach bar rows-1 Summary

`portty attach` now sends rows-1 in the first resize frame of every connection and in every SIGWINCH claim while its status bar is sized, and full rows otherwise, so full-screen apps stay off the bar row.

## Accomplishments

- `attach.BarRows(rows, reserve)` subtracts only when `reserve > 0 && rows > reserve`, so a 0- or 1-row terminal never wraps the uint16 frame.
- `(*Bar).ReservedRows()` is nil-safe and returns 1 only after `Start` sized the terminal.
- `onConnect` uses `initialResizeFrame(cols, rows, bar.ReservedRows())` and passes `bar.ReservedRows` to `watchResize`; the `--status-top` origin-mode limit is recorded in a comment at that call site. No escape constant added to bar.go.
- Tests: `TestBarRows`, `TestWatchResize_ClaimsBarRows`, `TestBar_ReservedRows`, `TestInitialResizeFrame`, and `TestHub_GridAttachBarRows` (GUI 120x40 vs CLI 80x23 under the grid-owner model).

## Task Commits

1. Task 1 (tracer): `9665c805` feat(15-01) BarRows, ReservedRows, SIGWINCH loop, initial frame
2. Task 2: `fb0c1f2d` test(15-01) bar, initial-frame and hub grid-owner pins

## Verification

Logs in `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.OLAJgn5MoS` (task 1) and `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.9cweiZM5bj` (task 2). go vet, go build, and `-race` runs of internal/attach, internal/statusbar, the root package (`-short`) and the relay grid tests all passed. The full Go and vitest suites run in 15-09.

## Deviations from Plan

None - plan executed exactly as written. Tests and implementation for Task 1 were committed together (one commit per task), not as separate RED and GREEN commits; the plan is `type: execute`, not `type: tdd`.

## Known Stubs

None.

## Threat Flags

None.

## Self-Check: PASSED

Both commits are ancestors of HEAD; all nine files_modified paths changed in the plan range.
