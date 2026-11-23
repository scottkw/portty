---
phase: 12-file-path-click
fixed_at: 2026-11-22T00:00:00Z
review_path: .planning/phases/12-file-path-click/12-REVIEW.md
iteration: 1
findings_in_scope: 1
fixed: 1
skipped: 0
status: all_fixed
---

# Phase 12: Code Review Fix Report

**Fixed at:** 2026-11-22
**Source review:** .planning/phases/12-file-path-click/12-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 1 (WR-02, as directed by the caller)
- Fixed: 1
- Skipped: 0

Edits and commits were made in the main checkout on `gsd/v1.2` (the caller asked for the main tree, so no isolated worktree was created). Verification ran in that same checkout: `go vet ./internal/relay/` clean, `go test -race ./internal/relay/` passed (132s).

## Fixed Issues

### WR-02: `Hub.PathClick` does not check that the requesting subscriber is still joined and not revoked

**Files modified:** `internal/relay/pathclick.go`, `internal/relay/pathclick_test.go`
**Commit:** 30ac9abb
**Applied fix:** Inside the existing `h.mu` block, `PathClick` now looks up `h.subscribers[sub]` and refuses unless the subscriber is joined and `!sub.revoked`, matching `pathNotice`. The doc comment lists the new refusal. New test `TestPathClick_DepartedOrRevokedRefused` covers a departed and a revoked subscriber: `PathClick` returns false, no resolve starts, no frame is queued for the clicker or another viewer, and no log line is written. Confirmed the test fails without the fix and passes with it.

Status note: this is an authorization-condition fix, so a human may want to confirm the intent. The change is small and mirrors the existing `pathNotice` check.

## Not fixed (out of scope)

### WR-01: relative click resolves against the newest print of that text

**Status:** deferred, user decision. The "last occurrence" behaviour is a recorded decision in 12-CONTEXT. No change made.

### IN-01 to IN-07

**Status:** not in scope (fix scope was WR-02 only). No change made.

---

_Fixed: 2026-11-22_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
