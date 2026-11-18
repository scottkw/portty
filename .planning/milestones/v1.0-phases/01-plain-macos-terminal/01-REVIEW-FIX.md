---
phase: 01-plain-macos-terminal
fixed_at: 2026-11-15T01:36:18Z
review_path: .planning/phases/01-plain-macos-terminal/01-REVIEW.md
iteration: 3
findings_in_scope: 1
fixed: 1
skipped: 0
status: all_fixed
---

# Phase 1: Code Review Fix Report

**Fixed at:** 2026-11-15T01:36:18Z
**Source review:** .planning/phases/01-plain-macos-terminal/01-REVIEW.md (re-review after iteration 2)
**Iteration:** 3 (iteration-2 report backed up as 01-REVIEW-FIX.iter3.md)

**Summary:**
- Findings in scope: 1 (WR-01; fix_scope `critical_warning`)
- Fixed: 1
- Skipped: 0
- WR-02 is a necessity finding and is out of scope, so I did not touch it.

**Where verification ran:** in the main checkout `/Users/ken/dev/portty`, on branch `gsd/phase-01-plain-macos-terminal`. No worktree was used, because the orchestrator's root-pin guard requires the main checkout. The results can be reproduced from the main checkout.

**Gates after the fix (HEAD eda1a433):**
- `go build ./... && go vet ./...` passed.
- `go test -race -short -count=1 ./...` passed in every package.
- `pnpm exec tsc --noEmit` passed.
- `pnpm exec vitest run` passed: 117 files, 1824 tests.
- `bash tests/check-traceability-paths.sh` passed: 120 paths OK.

## Fixed Issues

### WR-01: An open remote Hub modal flips to "Local" when the remote list empties

**Files modified:** `frontend/src/components/Hub/HubPanel.tsx`, `frontend/src/components/Hub/HubPanel.test.tsx`
**Commit:** eda1a433
**Status:** fixed
**Applied fix:**
- The modal render now passes `remote={modalState.isRemote}`, the provenance recorded at open in `handleCardClick` / `handleCapAcquired`. It no longer recomputes `remoteIdSet.has(modalState.session.id)` on every render. The IIFE that wrapped that computation is gone.
- `remote` was the only local/remote input the open modal received. It drives:
  - HubModal's header, through `const isLocal = !remote`;
  - TerminalPanel's `isGuestRef`, which gates the guest-only session-ended banner and the guest scale path.
- All three now stay remote when `remotePeers` empties.
- Test: the HubModal mock in `HubPanel.test.tsx` now exposes the `remote` prop as `data-remote`. The iteration-2 WR-04 test (open a remote modal, then re-render with `remoteSessions: []`) now asserts `data-remote="true"` both before and after the list empties. I confirmed it fails against the previous code: it got `'false'`.
- The test checks the routing prop rather than the rendered globe icon or banner. Both are pure functions of `remote`, and the existing `HubModal.test.tsx` contract test pins that mapping (`const isLocal = !remote`). HubModal and TerminalPanel are mocked in this file, because xterm needs canvas, which jsdom does not have.

## Not fixed (out of scope)

### WR-02 (necessity): init/retryInit `KillSession` of stopped sessions duplicates the Go watcher

Left untouched as instructed: necessity findings are excluded under `critical_warning`, and removing code is a scope decision for the human.

---

_Fixed: 2026-11-15T01:36:18Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 3_
