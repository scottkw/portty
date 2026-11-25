---
phase: 01-plain-macos-terminal
fixed_at: 2026-11-15T01:28:56Z
review_path: .planning/phases/01-plain-macos-terminal/01-REVIEW.md
iteration: 2
findings_in_scope: 5
fixed: 5
skipped: 0
status: all_fixed
---

# Phase 1: Code Review Fix Report

**Fixed at:** 2026-11-15T01:28:56Z
**Source review:** .planning/phases/01-plain-macos-terminal/01-REVIEW.md (re-review after iteration 1)
**Iteration:** 2 (iteration-1 report backed up as 01-REVIEW-FIX.iter2.md)

**Summary:**
- Findings in scope: 5 (WR-01..WR-05; fix_scope `critical_warning`)
- Fixed: 5
- Skipped: 0
- WR-06 is a necessity finding and is out of scope. I left it alone; see the note below. IN-01..IN-07 were not attempted.

**Where verification ran:** in the main checkout `the repo root`, on branch `gsd/phase-01-plain-macos-terminal`. No worktree was used, because the orchestrator's root-pin guard requires the main checkout. The results can be reproduced from the main checkout.

**Gates after the last fix (HEAD e47fe583):**
- `go build ./... && go vet ./...` passed.
- `go test -race -short -count=1 ./...` passed in every package.
- `pnpm exec tsc --noEmit` passed.
- `pnpm exec vitest run` passed: 117 files, 1824 tests.
- `bash tests/check-traceability-paths.sh` passed: 120 paths OK.

**Testing note:** no test in the repo mounts `App.tsx`. Every App test inspects the source through `App.tsx?raw`. The regression tests for WR-01..WR-03 follow that convention and assert the guard's presence and its order relative to `await Promise.all(` / `CreateSession`. The reviewer asked for a behavioral event-vs-reply test for WR-01. That would need a full App mount harness with mocks for every Wails binding, and I judged it too large for this fix. The WR-04 tests are behavioral: they render HubPanel.

## Fixed Issues

### WR-01: init/retryInit can overwrite a newer `tailscale:health` event

**Files modified:** `frontend/src/App.tsx`, `frontend/src/components/__tests__/App.tailscale.test.tsx`
**Commit:** 3b5c76c9
**Status:** fixed: requires human verification (ordering logic)
**Applied fix:**
- Added `healthEventSeqRef`, which the `tailscale:health` handler increments.
- `init` and `retryInit` capture the sequence before `Promise.all` and apply `setTailscaleHealth(health)` only if it is unchanged.
- Skipping the reply loses nothing. When the Go side's check produced a change, it emitted that change as an event, and the event already reached the handler.
- Tests: both restore paths capture the sequence before `await Promise.all(`, have the guarded apply and have no unguarded `setTailscaleHealth(health)`, and the handler increments the sequence.

### WR-02: init/retryInit restore a tab for a session that exited while `Promise.all` was pending

**Files modified:** `frontend/src/App.tsx`, `frontend/src/components/__tests__/App.shellExit.test.tsx`
**Commit:** 730e35fc
**Status:** fixed: requires human verification (race fix)
**Applied fix:** Both restore sets now filter `s.state !== 'stopped' && !exitedIdsRef.current.has(s.id)`, so neither tabs nor `hubSessions` get a session whose single `session:exit` was already handled. The existing per-path test (`describe.each` over init/retryInit) now asserts the new filter.

### WR-03: Every `session:exit` runs a redundant `KillSession` / `ToggleWebServing` and logs a warn

**Files modified:** `frontend/src/App.tsx`, `frontend/src/components/__tests__/App.shellExit.test.tsx`
**Commit:** 8f440756
**Status:** fixed
**Applied fix:**
- Split `handleCloseTab` in two. The new `removeTabState(id)` does UI work only: it clears `webEnabled`, filters `tabs` with adjacent-tab activation, and clears `fontSizes`.
- `handleCloseTab` still runs `ToggleWebServing` when web serving is on, then `KillSession` with its warn, then `removeTabState(id)`.
- The `session:exit` handler now calls `removeTabStateRef.current` (renamed from `handleCloseTabRef`) and makes no daemon calls.
- The only remaining `[App] KillSession failed:` warn is a genuine failure of a user-initiated kill.
- Tests:
  - The exit handler contains no `handleCloseTab`, `KillSession` or `ToggleWebServing`.
  - `removeTabState` is UI-only and keeps the focus shift.
  - `handleCloseTab` still kills unconditionally before the UI removal, with no early return.
- Behavior change: `removeTabState` now deletes the `webEnabled` entry unconditionally. Before, it did so only when the entry was true. The result is the same; it may cost an extra render.

### WR-04: Hub modal auto-close also closes remote modals on a transient remote-list empty

**Files modified:** `frontend/src/components/Hub/HubPanel.tsx`, `frontend/src/components/Hub/HubPanel.test.tsx`
**Commit:** 34b8411a
**Status:** fixed
**Applied fix:**
- `HubModalState` gained `isRemote`, recorded at open time in both `handleCardClick` and `handleCapAcquired`. `remoteIdSet` was added to the deps of `handleCapAcquired`.
- The auto-close effect now acts only on local modals whose id has left `sessions`.
- New behavioral test: open a remote modal (cap cached), re-render with `remoteSessions: []`, and the modal stays open. I confirmed this test fails against the iteration-1 effect. The iteration-1 local-close test still passes.
- The MODAL-06 source test was updated for the new `setModalState({ session, sourceRect: rect, isRemote })` call.
- Not changed (pre-existing, outside this finding): the modal's render-time routing still reads `remoteIdSet.has(modalState.session.id)`. While the remote list is empty, an open remote modal's routing prop flips to local. Switching it to `modalState.isRemote` would be a one-line follow-up if wanted.

### WR-05: `sign-macos` and `publish` have no timeouts, and `notarytool --wait` is unbounded

**Files modified:** `.github/workflows/release.yml`
**Commit:** e47fe583
**Status:** fixed
**Applied fix:**
- `sign-macos` now has `timeout-minutes: 90`, and `xcrun notarytool submit ... --wait --timeout 60m`.
- `publish` now has `timeout-minutes: 15`.
- The YAML parses. All 4 release jobs and the build job are now bounded.

## Not fixed (out of scope)

### WR-06 (necessity): frontend `KillSession` of stopped sessions in init/retryInit duplicates the Go watcher

**File:** `frontend/src/App.tsx` (the `allSessions.filter((s) => s.state === 'stopped').forEach(KillSession…)` blocks in init and retryInit)
**Reason:** necessity findings are excluded under `critical_warning`. Deleting these blocks is also not the smallest way to fix WR-02 or WR-03:
- WR-02 is fixed by the `exitedIdsRef` filter.
- WR-03's third, failing kill is gone because the exit path is now UI-only.

The remaining effect is at most one `KillSession of stopped session failed` warn at launch, when the frontend kill loses a race with the watcher. Removing the blocks means also changing the `App.shellExit.test.tsx` assertion that requires them. That is a scope decision for the human.

---

_Fixed: 2026-11-15T01:28:56Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 2_
