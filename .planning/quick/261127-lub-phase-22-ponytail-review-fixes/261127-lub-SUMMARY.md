---
phase: quick-261127-lub
plan: 01
subsystem: desktop-share
tags: [phase-22, ponytail-review, guest-share, revoke, media-binding, input-tap, audit, frontend]
status: complete
requires: [phase-22 code-review fixes (22-REVIEW-FIX.md)]
provides: [Phase 22 pre-ship ponytail-review fixes]
affects: [internal/desktop, internal/screencap, internal/daemon, frontend guest page, Share Desktop sheet]
key-files:
  modified:
    - internal/desktop/revoke.go
    - internal/desktop/media_binding.go
    - internal/desktop/manager.go
    - internal/screencap/inputtap_darwin.go
    - internal/screencap/inputtap_darwin.m
    - internal/daemon/desktop_share_api.go
    - internal/daemon/client.go
    - frontend/src/lib/desktopClient.ts
    - frontend/src/lib/desktopShare.ts
    - frontend/src/components/DesktopView.tsx
    - frontend/src/components/ShareDesktopSheet.tsx
    - TESTING.md
decisions:
  - "A guest Disconnect deny-lists the person up front, before any cut, so a row already cut another way cannot skip the deny-list"
  - "A media check with no verdict (lookup_failed, no_pair) still cuts but closes 1001 'media failed' (retryable) with no revoked audit line; wrong_user/wrong_node keep 4432 + media_identity_mismatch; no_lookup (nil seam) unchanged"
  - "PhysicalState reads the tap without rebuilding it; only the 1 s poll (InputTapState) rebuilds"
completed: 2026-11-27
commits: 11
plan_head_before: 10d8d7e9efb4567ab4dc5e2d13bba6be983fd0c6
plan_head_after: 1bc20b957875c2f4ae150b8842c284f0335f4b60
actuals:
  tasks: 3
  commits: 11
---

# Quick 261127-lub: Phase 22 ponytail-review fixes Summary

Ghost-row Disconnect now deny-lists the person. A media check that cannot reach a verdict closes retryable with no false revoke. The per-input tap reading no longer rebuilds the tap. The daemon admit/deny routes 404 unknown ids early and take back leave reasons when a cut fails. On the frontend: a guest waiting at Admit pings on wake, phone guests see their control chip, and Esc is ignored while the share sheet is inert. Two lean deletions were also made.

Every behaviour fix was written test-first, and its new test failed before the change. The exceptions are 1.3 (no Go seam), 1.4 (the new test is coverage of existing behaviour, see below) and 1.5 (a comment).

## Applied

| Item | Commit | Change | Test |
|------|--------|--------|------|
| 1.1 | 2f723903 | `CloseViewer` of a guest calls `gs.RevokeUser` up front, unconditionally, then `closeGuestUser`. `cutViewer` no longer takes a `deny` parameter, and its call sites in revoke.go, media_binding.go and manager.go:861 (one-line change) were updated. | `TestGuestRevoke_DisconnectOfAnAlreadyCutRowStillDenyLists`: a silent guest is cut with `CloseViewerWith(replaced)`, then `CloseViewer` runs on that ghost row. The test expects `IsDenied` and a 4430 refusal on reconnect. It failed before the fix with "not deny-listed". |
| 1.2 | 507ee5fc | `lookup_failed` and `no_pair` still cut through `cutViewer` (fail closed), but now close 1001 "media failed" and emit no revoked event. | `TestMediaBinding_LookupErrorFailsClosed` and `TestMediaBinding_NoSelectedPairFailsClosed` now use `wantRetryableCut`: 1001, no revoked event, not deny-listed. Both failed before the fix (4432 plus a revoked event). The 4432 branch is still covered by `_AnotherUserIsCut`, `_PairChangeToAnotherNodeCutsTheGuest` and `_TwoMismatchesCutOnceAndReportOnce`. |
| 1.3 | ac23d19d | `pc_tap_state` takes a `rebuild` flag. `PhysicalState` calls it with 0 (read only). `InputTapState`, used by `pollTap` every 1 s, calls it with 1. | No Go seam: a rebuild needs `g_wanted`, which needs a real tap, and `StartInputTap` refuses under `go test`. This item is covered by the build plus the existing `internal/screencap` and `internal/desktop` tests only. |
| 1.4 | 70878372 | `answerRequest` looks up the pending request once and returns 404 early for an unknown id. That removes the uid=0 deny path and the repeated `Pending()` scans. `setLeaveReason` now reports whether it set the reason, and `markPersonLeaves` returns the IDs it set. If `DenyGuest` or `CloseViewer` fails, `clearLeaveReasons` removes those reasons. | `TestShareAudit_PersonCutLogsEveryDevicesLeave` (subtests disconnect and deny). It runs Other plus OtherSecondNode. The guests never apply the SDP answer, so ICE never connects and the fake WhoIs media check never runs, which gets around what blocked the code-fixer. Every leave line must carry revoked or denied. This test passed before the change too (the existing `markPersonLeaves`), so it is coverage, not RED. A mutation check (making `markPersonLeaves` a no-op) makes it fail. The early-404 path is covered by the existing `TestShareAudit_UnknownRequestWritesNothing`. The clear-on-failure path only happens in a race and has no deterministic test. |
| 1.5 | de1cdff4 | Fixed the stale comment at client.go:54-55: the bound has one exception, `StreamDesktopShareEvents`, which lifts it on a copy of the client. | n/a |
| 2.1 | 598b77d0 | A `waiting` flag is set on admission waiting and cleared in `connect()` and on admitted. `onWake` now pings when `(streaming \|\| waiting) && transport`. | "a wake while waiting at Admit pings; a pong keeps the socket and no answer redials" (desktopClient.guest.test.ts). |
| 2.2 | 93cb8b79 | `mayControl = phoneMode ? phoneRights?.control === true : canControl` is used on the unavailable, someone-using, other-holder and guest control chip lines. The window and Accessibility chips still use `canControl`. | "a phone guest with control sees You have control and the lease chips, not View only" (DesktopView.guest.test.tsx). |
| 2.3 | db4cfcae | `onCancel` ignores Esc while checking or starting. `inert` moved above `useDialogFocus`, which reads the latest callback on each render. | "Esc does nothing while the sheet is checking or getting a link, as Done does" (ShareDesktopSheet.test.tsx). |
| 3.1 | 8db34dad | DesktopView uses `copy.GUEST_CHIP_LOCKED`. The duplicate `LOCKED_GUEST` was deleted (same string). | Existing DesktopView guest tests. |
| 3.2 | e9015bd2 | Deleted `ShareEvent`, `lastEvent` and the `desktopshare:event` subscription. A grep found no reader anywhere in frontend/src. The Go side still emits the event. | tsc plus the existing ShareBanner and ShareDesktopSheet tests. |
| TESTING.md | 1bc20b95 | Section 4 rows for revoke_test.go, media_binding_test.go, desktop_share_audit_test.go, desktopClient.guest.test.ts, DesktopView.guest.test.tsx and ShareDesktopSheet.test.tsx now name the new or changed tests. No new test files. | `check-traceability-paths.sh` OK (340 paths). |

## Skipped

Skipped on purpose, per the plan:
- unused `shareDesktopCopy` exports (a copy-contract mirror of the Go and HTML strings; tests pin them)
- the `fetchGuestCheck` merge
- the two name sanitizers
- the `BarState.guest` double guard (defense in depth)
- `rightsMap` reuse in manager.go
- the `endReason` field
- arbiter unused bits
- heartbeat removal
- the `doShare` merge
- `desktopShareStatus` dead cases
- duplicated state structs
- whois.go near-copies

Skipped by the executor: nothing.

## Deviations from Plan

None. Notes:
- Item 1.4's daemon test was possible after all. The guests never complete ICE, so the fake's single identity is never asked for a media verdict. No fallback to internal/desktop was needed.
- Item 1.1 changes the order inside a guest Disconnect: the deny-list entry is now written before the cut instead of in the middle of it. `closeGuestUser`'s comment already assumed this order ("The caller has deny-listed the user first").

## Verification

- `gofmt -l`: clean. `GOTOOLCHAIN=go1.26.11 go vet ./...`: exit 0.
- `go test -race -count=1 ./internal/desktop ./internal/daemon ./internal/webserver ./internal/screencap .`:
  - daemon, webserver and screencap: ok.
  - internal/desktop: one failure in the full run, `TestQualityShaped_MediumLinkPullsTheTargetDown` (a Phase 23 adaptive-quality test that depends on load; these fixes do not touch it). It passed when re-run alone with `-race -count=2`.
  - root package: `TestNoFakeWhoIsInRelease` fails only because this worktree has no `frontend/dist` (`pattern all:frontend/dist: no matching files found`). As instructed, dist was not built. Every other root test passed.
- `cd frontend && pnpm exec tsc --noEmit`: exit 0. `pnpm exec vitest run`: 154 files, 2821 tests passed.
- `bash tests/check-brand.sh`: clean before every commit. `bash tests/check-traceability-paths.sh`: OK.
- Nothing ran Portty, screen capture or a real input tap, and nothing contacted tailscaled or a 100.x address. The 100.64.x strings in the tests go to fakes only.

## Self-Check: PASSED

All 11 commits are ancestors of HEAD. Every modified file exists.
