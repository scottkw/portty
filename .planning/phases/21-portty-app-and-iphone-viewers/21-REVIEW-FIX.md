---
phase: 21-portty-app-and-iphone-viewers
fixed_at: 2026-11-27T10:35:00Z
review_path: .planning/phases/21-portty-app-and-iphone-viewers/21-REVIEW.md
iteration: 1
findings_in_scope: 5
fixed: 5
skipped: 0
status: all_fixed
---

# Phase 21: Code Review Fix Report

**Fixed at:** 2026-11-27
**Source review:** .planning/phases/21-portty-app-and-iphone-viewers/21-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 5 (WR-01..WR-05; no Critical findings; IN-01..IN-03 out of scope for `critical_warning`)
- Fixed: 5
- Skipped: 0

Each fix was test-first (the new or changed test failed before the change). The fixer ran in the existing agent worktree, so it did not create a second one (no recovery sentinel, no cleanup tail).

## Fixed Issues

### WR-01: A refused peer share port is classified as the terminal "Desktop sharing is off" state

**Files modified:** `internal/daemon/peer_desktop.go`, `internal/daemon/peer_desktop_test.go`, `TESTING.md`
**Commit:** 9ef402e6
**Status:** fixed: requires human verification (logic change)
**Applied fix:** `connectPeerDesktop` now treats only `tailnet.ErrPeerDesktopOff` (the peer's 404) as `desktop_off` (409, held token deleted). `ECONNREFUSED` falls through to the generic unreachable case (502, token kept), which the viewer retries. The test `TestPeerDesktop_RefusedSharePortIsDesktopOff` was inverted and renamed `TestPeerDesktop_RefusedSharePortIsRetryable` (asserts 502 `unreachable` and the held token kept); the rename is mirrored in the TESTING.md DVWR-01 traceability row.
**Not done (reviewer's optional second half):** making a click on an existing Desktop sidebar row reconnect a final tab, or adding a Reconnect action to the `desktop_off` card. With the refused case now retryable, `desktop_off` is only a real 404, and a click-to-retest is a UI-SPEC behavior change that needs its own decision. Closing and reopening the tab still recovers a genuine `desktop_off`.

### WR-02: Cmd+V in control mode could push clipboard text to the Mac after control ended

**Files modified:** `frontend/src/components/DesktopTab.tsx`, `frontend/src/components/__tests__/DesktopTab.test.tsx`
**Commit:** 0af1d672
**Status:** fixed: requires human verification (logic change)
**Applied fix:** `paste()` returns after the notice check when `controlling.current` is false, before `pasteToMac`. The existing post-send check (before replaying the key) stays, because control can also end during the send. The existing "leaving control mode while the paste is in flight" test now also asserts `pasteToMac` is not called.

### WR-03: The bar's Paste to Mac bypassed the concealed-item check inside the app

**Files modified:** `frontend/src/components/DesktopView.tsx`, `frontend/src/components/DesktopTab.tsx`, `frontend/src/components/__tests__/DesktopView.test.tsx`, `frontend/src/components/__tests__/DesktopTab.test.tsx`
**Commit:** d23310b1
**Status:** fixed: requires human verification (logic change)
**Applied fix:** `DesktopView` takes an optional `readClipboard?: () => Promise<HomeClipboard>`. When present, the bar's `pasteToMac()` reads through it (never `navigator.clipboard`) and maps `ok` to `sendToMac`, `empty` and `too_large` to the existing notices, `concealed` to a new notice using the shared words from `controlMode`, and anything else (including a rejected call) to the failed notice. `DesktopTab` passes a module-level `readHome()` wrapper over `ReadHomeClipboard`, which `paste()` now also uses, so there is one home-clipboard read in the app. The browser page passes nothing and keeps `navigator.clipboard`. Tests cover the bridge in `DesktopView` and the wiring end-to-end in `DesktopTab` (a concealed item is never sent, and the browser clipboard is not consulted).

### WR-04: `rights` after the first hello were ignored

**Files modified:** `frontend/src/components/DesktopView.tsx`, `frontend/src/components/__tests__/DesktopView.test.tsx`
**Commit:** 64cdb045
**Status:** fixed: requires human verification (logic change)
**Applied fix:** In the `rights` handler, when the viewer is not a guest, not a phone and the input controller already exists, every later event now sets `canSurface` and `canClipboard` from the grant. A lost control right calls `setControl(false)` and `setCanControl(false)`; a regrant calls `setCanControl(true)`, and the existing `active` effect turns the controller back on. The first hello still goes through the creation branch unchanged, and the guest branch (which already handled live changes) is untouched. A new test drops clipboard, then control, then regrants, checking the Clipboard button, the `onControlChange` availability and that clicks on the picture stop and resume sending.

### WR-05: Sidebar Desktop row state outlived its tab and was erased by a single failed probe

**Files modified:** `frontend/src/App.tsx`, `frontend/src/components/DesktopTab.tsx`, `frontend/src/components/__tests__/App.desktop-tab.test.tsx`
**Commit:** 307277be
**Status:** fixed: requires human verification (logic change)
**Applied fix:** `DesktopTab` now has an unmount cleanup that sends `onDesktopState(connectHost, null)` and `onLabel(null)`, which covers every close path and clears `desktopLabels[tabId]` (the entry becomes `null`, which reads as no label). The poll in `App.tsx` now prunes a peer's muted line only when that peer is `reachable` and `desktop !== true`, instead of whenever it was not currently `desktop === true`. Two new App tests cover close-clears-line and a failed probe keeping "Locked".
**Residual:** `Reachable` is also true for a refused share port (`MetaRefused`), which reports `desktop: false`, so a refused probe can still prune the line. The app layer cannot tell that apart from a real "off" answer without a new field on the binding, so it was left alone.

## Verification

Run in the isolated agent worktree (not the main checkout), with `frontend/node_modules` temporarily symlinked to the main checkout's and removed afterwards (never staged). Numbers are therefore reproducible from the main checkout once these commits are merged.

- `tsc --noEmit`: clean.
- Frontend `vitest run`: 142 files, 2594 tests, all passing.
- `go test ./...` (GOTOOLCHAIN=go1.26.11): all packages pass except the root package's `TestNoFakeWhoIsInRelease`, the known failure in worktrees without `frontend/dist` (not caused by these changes).
- `bash tests/check-brand.sh` clean before every commit; `bash tests/check-traceability-paths.sh` OK (313 paths) after the TESTING.md rename. No new test files were added.

---

_Fixed: 2026-11-27_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
