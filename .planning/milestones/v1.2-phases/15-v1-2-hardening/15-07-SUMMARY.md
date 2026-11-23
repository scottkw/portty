---
phase: 15-v1-2-hardening
plan: 07
subsystem: tailnet peer discovery
tags: [go, tailnet, sidebar, meta-fetch, econnrefused]
status: complete

requires:
  - phase: 04
    provides: tri-state meta fetch and the daemon peer connect ECONNREFUSED branch
provides:
  - "MetaRefused: a refused share port is reachable with an empty session list in FetchAllPeerSessionsMeta"
affects: [15-09 TESTING.md traceability and M-154]

actuals:
  tokens: 1800
  tasks: 2
  commits: 2
plan_head_before: 23792027dcf8b6309faed8a81bf3fdc64c07bcc1
plan_head_after: 1d7a61e1224e50fa0a6e08547352343f3bbfc1c9
commits: 2

tech-stack:
  added: []
  patterns:
    - "Classify a transport failure with errors.Is(err, syscall.ECONNREFUSED) and still return the error to callers that read it"

key-files:
  created: []
  modified:
    - internal/tailnet/sessions.go
    - internal/tailnet/tailnet_test.go

key-decisions:
  - "A refusal (TCP reset) means the Mac answered and nothing is shared: reachable, zero sessions. Timeout and no route stay unreachable; no time-based grace."
  - "A refusal by DNS name is not retried by Tailscale IP; the IP fallback stays for MetaUnreachable only."
  - "doFetchSessionsMeta still returns the transport error with MetaRefused, so internal/daemon/peer_connect.go is unchanged."

requirements-completed: []
---

# Phase 15 Plan 07: Refused share port reads as "Nothing shared" Summary

A host whose share server stopped with its last share now lists as reachable with no sessions instead of flashing "Can't reach this Mac", via a new `MetaRefused` status classified from `ECONNREFUSED` at the meta fetch.

Sidebar branch: fixed

## What changed

- `internal/tailnet/sessions.go`: new last `MetaStatus` constant `MetaRefused`; `doFetchSessionsMeta` returns `nil, MetaRefused, err` when `errors.Is(err, syscall.ECONNREFUSED)`; `fetchAllPeerSessionsMeta` sets `Reachable: status == MetaOK || status == MetaRefused`; comments on the IP fallback and on the error contract of the two fetch functions.
- `internal/tailnet/tailnet_test.go`: `TestMetaStatus_TriState/refused` now expects `MetaRefused` plus `ECONNREFUSED`; new `timeout` subtest (200 ms client timeout against a server that never answers: `MetaUnreachable`, not refused); new `TestFetchAllPeerSessionsMeta_RefusedIsReachableEmpty` (refused is reachable with a non-nil empty list, asleep stays unreachable, forbidden dropped).

The refusal was reliably distinguishable (a closed local TLS port gives `ECONNREFUSED` through `errors.Is`), so the CONTEXT fallback did not run. The live tailnet confirmation is TESTING.md M-154 (15-09, Tart VMs).

## Tasks

| Task | Name | Commit |
| ---- | ---- | ------ |
| 1 | MetaRefused classified, passed through without IP retry, Reachable in fetchAllPeerSessionsMeta (tracer, verified end to end) | aa923815 |
| 2 | Timeout stays unreachable (pinned); error contract documented | 1d7a61e1 |

## Verification

- Task 1 logs: `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.d31IuKGnOS` (vet clean; refused subtest, RefusedIsReachableEmpty and DropsForbiddenPeers PASS).
- Task 2 logs: `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.QJRjjYtHOC` (vet clean; `timeout` and `refused` subtests PASS; `go test -race -short -count=1 ./internal/tailnet/...` ok).
- `git diff 23792027 -- internal/daemon/peer_connect.go frontend/src/components/Sidebar.tsx` is empty; `syscall.ECONNREFUSED` still present in peer_connect.go.
- Red phase: with the tests written first, the package failed to compile (`undefined: MetaRefused`).

## Deviations from Plan

None - plan executed exactly as written. Task 2's doc-comment updates landed in Task 1's commit because they describe the same classification; Task 2 added the timeout test only (the plan states no behaviour change there).

## Accepted consequence

A Mac that is awake with its Portty daemon down also refuses, so it shows "Nothing shared" until the 30 s probe cache drops it.

## Known Stubs

None.

## Threat Flags

None. T-15-13 mitigated: only `ECONNREFUSED` maps to reachable, pinned by the `timeout` subtest; a refused peer carries no sessions or capability; MetaForbidden peers are still dropped.

## Self-Check: PASSED

- internal/tailnet/sessions.go and tailnet_test.go modified; commits aa923815 and 1d7a61e1 are ancestors of HEAD.
