---
phase: 18-self-installing-updates
plan: 04
subsystem: update-ui
tags: [react, go, wails, banner, native-alerts, tray]
status: complete

requires:
  - phase: 18-01
    provides: "Service states installing and relaunching, reveal fallback with revealed-with-Error"
provides:
  - "UpdateBanner: Update button, Installing/Restarting messages, revealed-with-error warning variant"
  - "UpdateStatus type declared in UpdateBanner.tsx with state widened by installing and relaunching"
  - "Native alerts (Update/Later, busy copy) and tray rule that hides a failed install"
affects: [18-07, 18-09]

actuals:
  tokens: 14000
  tasks: 2
  commits: 2
plan_head_before: 32b69fc3041362c95fa2d7b7e1647c25c55a3110
plan_head_after: 07077293d23767ad2d64b3efbe3ec36346f27ad3

tech-stack:
  added: []
  patterns:
    - "Frontend widens the generated bindings type locally (Omit + union) instead of editing the generated stub"
    - "Go tests hold the real Service mid-install by wrapping its Run seam and StartInstaller (tmp dirs only)"

key-files:
  created: []
  modified:
    - frontend/src/components/UpdateBanner.tsx
    - frontend/src/App.tsx
    - frontend/src/components/__tests__/UpdateBanner.test.tsx
    - frontend/src/components/__tests__/App.update.test.tsx
    - app_update.go
    - app_update_test.go

key-decisions:
  - "UpdateStatus lives in UpdateBanner.tsx; the Wails bindings stub is untouched (generated header)"
  - "A relaunching event also clears a banner dismissal (plan only required installing), so the banner stays visible in both busy states"
  - "No new binding: DownloadUpdate keeps its name"

requirements-completed: [UPD-09, UPD-05]

duration: 25min
completed: 2026-11-25
---

# Phase 18 Plan 04: Update banner and alert copy for self-install Summary

The banner, the Help-menu alert and the tray now describe a self-installing update: Update/Later buttons, "Installing Portty X…" and "Restarting Portty X…", and a failed install shown as a warning with Go's sentence.

## What changed

- **Banner:** `warn` covers an error on `available` or `revealed`; the action button (Update, or Try Again when warn) renders only for `available`; `installing` and `relaunching` render info text with no action button and the dismiss button kept. The component still never opens a URL or file.
- **App.tsx:** `showUpdateBanner` includes `installing` and `relaunching`; those events (and `downloading`) clear an earlier dismissal.
- **Go:** `offerDownload` offers Update/Later (default Update). The manual check's busy branch (new `busyState` helper covering downloading, verifying, installing, relaunching) says "Portty is updating to version X." / "It will restart by itself when the update is installed." `offerUpdate` shows the window for the two new states. `onUpdateStatus` sets the tray item to "" for a revealed status with an Error. `DownloadUpdate` doc comment updated.
- **Tests:** frontend tests cover Update label, both new messages, revealed-with-error warning with no action, un-dismiss on installing/relaunching. Go tests: `holdAt` helper drives the real Service into downloading/installing/relaunching with fakes confined to temp dirs; `TestManualUpdateCheck_WhileUpdating`, `_WhileVerifying`, new tray-offer subtests, and `TestTrayUpdate_HiddenAfterFailedInstall`.

## Verification

- `tsc --noEmit` clean; vitest UpdateBanner, App.update, App.update-needed, App.test: all pass.
- `go vet .` clean, gofmt clean, `go test -race` on TestManualUpdateCheck|TestTrayUpdate|TestDownloadUpdate|TestUpdatePoller|TestNotificationClick|TestUpdateHarness passes. `bash tests/check-brand.sh` clean.
- No commit or working change touches `frontend/src/wailsjs/go`.
- No host mount, open or installer: the Go hold-helper's fake `Run` only makes directories under `t.TempDir()`.

## Deviations from Plan

None to code scope. Execution notes:

- **[Rule 3 - Blocking] Frontend tooling in a fresh worktree:** the worktree had no `frontend/node_modules`, and `pnpm exec` tried to reinstall. I symlinked the main checkout's node_modules (untracked, removed before the final commit) and ran `tsc`/`vitest` binaries directly. No package installed.
- **Extra test beyond plan:** `TestManualUpdateCheck_WhileVerifying` (verifying is in the busy copy list).
- `TestManualUpdateCheck_AvailableDownload` renamed `_AvailableUpdate`, `_WhileDownloading` renamed `_WhileUpdating`.

## Deferred Issues

- `TestNoFakeWhoIsInRelease` fails in this worktree only because `frontend/dist` is not built (`pattern all:frontend/dist: no matching files`). Pre-existing, unrelated to this plan.

## Known Stubs

None.

## Threat Flags

None. T-18-19 and T-18-20 hold: text rendered by React as text; the banner calls only onDownload/onDismiss and the never-opens-a-URL test passes.

## Self-Check: PASSED

Commits 9b1b1c90 and 07077293 exist on the branch; all six modified files exist.
