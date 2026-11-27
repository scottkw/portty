---
phase: 22-click-to-share-for-tailnet-people
plan: 02
subsystem: desktop
tags: [audit, jsonl, sanitizer, identity, tailnet]
requires:
  - phase: 19-unattended-desktop-viewing
    provides: internal/desktop package
provides:
  - "Audit appender (OpenAudit, Record, Close) writing desktop-audit.jsonl, 0600, rotating at 5 MiB"
  - "Identity, NewIdentity, SanitizeName (names sanitized once, at the source)"
  - "Event, mode and reason constants for the persisted v1 audit format"
affects: [22-06, 22-10, 22-11, 22-13]
tech-stack:
  added: []
  patterns:
    - "Mutex-guarded single-Write JSON Lines appender with injected clock and log seams"
key-files:
  created:
    - internal/desktop/audit.go
    - internal/desktop/audit_test.go
    - internal/desktop/names.go
    - internal/desktop/names_test.go
  modified: []
key-decisions:
  - "Audit cap is 5 MiB with one .1 file (daemon.log's 10 MiB is not copied)"
  - "Failure log carries the operation and underlying errno only, never the record or path"
  - "SanitizeName turns whitespace (including newline and tab) into one space, but drops other Cc and Cf characters outright"
requirements-completed: [DSHR-07, DSHR-04]
status: complete
estimate:
  tokens: 45000
  tasks: 2
actuals:
  tokens: 14000
  tasks: 2
  commits: 2
plan_head_before: 51d8f9f94040cf981d221cca58305f5e907d63cb
plan_head_after: 588285e95d25b52efed1d011dcf3f95937d433e3
completed: 2026-11-27
---

# Phase 22 Plan 02: Audit appender and name sanitizer Summary

JSON Lines audit appender (0600, O_APPEND, one Write per line, 5 MiB rotation, never blocks) plus the SanitizeName/Identity pair, so every guest name is cleaned once at the source.

## What was built

- `internal/desktop/audit.go`: `Audit`, `OpenAudit`, `Record`, `Close`, `AuditRecord` (fixed v1 key order, `reason` omitted when empty), the twelve `Audit*` event constants, `AuditModeGuest`/`AuditModeOwn`, the ten `Reason*` constants, `maxAuditBytes = 5 << 20`.
- `internal/desktop/names.go`: `Identity`, `NewIdentity` (sanitizes login, display and node names), `SanitizeName`, `maxNameRunes = 128`.
- Rotation renames to `.1` (replacing an older one) and reopens 0600; size is seeded from Stat at open. A failed marshal, write, rotate or reopen drops the event and logs at most once a minute through slog (op and errno only).
- Tests use only `t.TempDir()`; injected `now` and `logf` fields (unexported) pin the timestamp and count log calls.

## Task commits

1. Task 1 (tracer): `98b547a2` - one guest event becomes one 0600 JSON line; tracer verify re-run end-to-end (vet, gofmt, `-race -count=3`) passed before Task 2.
2. Task 2: `588285e9` - sanitizer, rotation, failure handling, concurrency, constants.

## Verification

- `go vet`, `gofmt -l`, `golangci-lint run --new-from-rev=07ad7d21 ./internal/desktop/...` (0 issues), `go test -race -count=3 -run 'TestAudit_|TestNames_'` (13 tests listed, all pass), `go test -race -count=1 ./internal/desktop/` all pass.
- `bash tests/check-brand.sh` clean before both commits.

## New test files for 22-13 (TESTING.md traceability)

- `internal/desktop/audit_test.go` (DSHR-07)
- `internal/desktop/names_test.go` (DSHR-04 / T-22-06)

## Deviations from Plan

None - plan executed exactly as written. Note: Task 1 kept `SanitizeName` to trim only, as the plan allowed; Task 2 completed it.

## Known Stubs

None.

## Threat Flags

None. The file is local, 0600, and `AuditRecord` has no field able to hold a token, input, clipboard text, SDP or frames (T-22-07). T-22-06 is mitigated by SanitizeName; the "login name beside display name" half lands in 22-10 and 22-11.

## UAT candidates (real Mac only)

None for this plan; it is pure Go and fully covered by unit tests. Not exercised: behavior on a real full disk and the 0500-directory case on non-APFS volumes (the rotate-failure test relies on POSIX directory permissions).

## Self-Check: PASSED

audit.go, audit_test.go, names.go, names_test.go exist; commits `98b547a2` and `588285e9` are ancestors of HEAD.
