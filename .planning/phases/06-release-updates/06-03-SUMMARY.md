---
phase: 06-release-updates
plan: 03
subsystem: frontend
status: complete
tags: [updates, banner, wails-bindings, vitest]
requires: []
provides:
  - "UpdateBanner({ status, onDownload, onDismiss, className }) with available, downloading, verifying, revealed and error states"
  - "UpdateStatus interface plus GetUpdateStatus and DownloadUpdate in the hand-maintained JS stubs"
  - "App follows GetUpdateStatus on mount and update:status events; showUpdateBanner in the banner-stack INVARIANT gate"
affects: [06-02 (Go must implement the exact contract), 06-04 (TESTING.md rows)]
tech-stack:
  added: []
  patterns: ["Go owns update state; the UI renders Status events and ignores binding results"]
key-files:
  created:
    - frontend/src/components/__tests__/App.update.test.tsx
  modified:
    - frontend/src/components/UpdateBanner.tsx
    - frontend/src/components/__tests__/UpdateBanner.test.tsx
    - frontend/src/components/__tests__/App.test.tsx
    - frontend/src/App.tsx
    - frontend/src/wailsjs/go/main/App.d.ts
    - frontend/src/wailsjs/go/main/App.js
    - frontend/src/style.css
decisions:
  - "A downloading event clears dismissedUpdate, so verifying and revealed for a dismissed version also stay visible after a download starts (the plan's formula only covered downloading itself)"
  - "The error state uses ExclamationTriangleIcon, matching the other warning banners"
metrics:
  duration: "about 20 min"
  completed: 2026-11-18
actuals:
  tokens: 7300
  tasks: 2
  commits: 4
plan_head_before: 39f4341c9749340004aaade1ffb6d3fd187c9829
plan_head_after: 5650abc78e3d4d59906d3d9e52e9deeb69598abc
---

# Phase 6 Plan 03: Update Banner States Summary

The update banner now follows Go's `update:status` and `GetUpdateStatus`. Download calls `DownloadUpdate`. The banner then shows a percent and a progress bar, "Verifying…", "verified and shown in Finder", or Go's error text with Try Again. It never opens a URL.

## What was built

- **Hand stubs** (`App.d.ts`, `App.js`): `UpdateStatus` with a string-literal `state` union, plus `GetUpdateStatus` and `DownloadUpdate`. `GetLastUpdateInfo` and `CheckForUpdates` are removed.
- **UpdateBanner**: the state table from the plan. Every state keeps role alert, aria-live polite, the XMarkIcon Dismiss and className. The error state uses `banner--warning`. The `BrowserOpenURL` import is gone, and the component re-exports `UpdateStatus`.
- **App.tsx**:
  - `updateStatus` and `dismissedUpdate` state.
  - The mount effect returns early in web mode. Otherwise it reads `GetUpdateStatus` and subscribes to `EventsOn('update:status', apply)`. `apply` ignores null and `checking`, and a downloading status clears the dismissal.
  - `showUpdateBanner` replaces `update` in the INVARIANT gate. The UI-SPEC order is unchanged.
  - `handleDownloadUpdate` ignores the result of `DownloadUpdate()`.
- **style.css**: `.banner__progress` has a 120px width, `flex-shrink: 0` and `accent-color: var(--color-accent)`. It uses no literal colours.

## Tasks and commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 RED | 1762e1d4 | test: App update flow (available, Download, progress, verifying, revealed, none) |
| 1 GREEN | 95e32615 | feat: stubs, banner states, App wiring; UpdateBanner.test.tsx ported to new props |
| 2 RED | 80c9920e | test: state cases, error/Try Again, dismissal, checking, web mode, App.test.tsx pins |
| 2 GREEN | 5650abc7 | feat: error state with Try Again, `.banner__progress` |

Tracer gate: Task 1's `<verify>` re-ran green before Task 2 started.

## Verification

- `tsc --noEmit`: clean.
- The 4 named files (UpdateBanner, App.update, App, style.redesign) passed.
- Full vitest: 88/88 files, 1198 tests. On the first run, the known SharePopover Copy flake failed once. It passed alone, and then the full suite passed.
- `banner__progress` is in style.css. App.tsx and the stubs contain no old update names (0 matches).

## Deviations from Plan

1. **[Rule 3 - Blocking] UpdateBanner.test.tsx ported in Task 1.** Task 1's gate runs `tsc`, which failed on the old `update` prop in this test. The plan lists the test only under Task 2. Task 1 ported the file to the new props and dropped the release-URL cases. Task 2 then rewrote it with the full state coverage.
2. **[Rule 1 - Correctness] Dismissal cleared on a download, not just bypassed while downloading.** The plan's formula `latest !== dismissedUpdate || state === 'downloading'` would hide the banner again at verifying and revealed for a dismissed version. The must-have says a dismissal holds "until a newer version arrives or a download starts", so `apply` sets `dismissedUpdate` to null on a downloading status. Covered in App.update.test.tsx (dismiss 9.9.10, downloading, then verifying stays visible).
3. **Task 2 RED was partial.** The checking, dismissal and web-mode rules already passed after Task 1's App wiring, because the plan put them in the App effect. Only the error and Try Again cases failed before Task 2's GREEN commit.

## Known Stubs

None. The bindings call `main.App.GetUpdateStatus` and `main.App.DownloadUpdate`, which 06-02 implements in Go. Until 06-02 merges, the real app gets a Wails error from GetUpdateStatus. The App catches it and shows no banner.

## Self-Check: PASSED

- FOUND: frontend/src/components/__tests__/App.update.test.tsx
- FOUND: frontend/src/components/UpdateBanner.tsx
- FOUND commits: 1762e1d4, 95e32615, 80c9920e, 5650abc7
