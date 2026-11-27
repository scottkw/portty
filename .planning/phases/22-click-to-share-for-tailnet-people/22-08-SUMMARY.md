---
phase: 22-click-to-share-for-tailnet-people
plan: 08
subsystem: gui-tray
tags: [tray, liveness-stream, wails-events, sharing-icon, dshr-06]
requires:
  - phase: 22-06
    provides: StreamDesktopShareEvents, StopDesktopShare, DesktopShareLine
provides:
  - "App.startDesktopShareEvents: the GUI's whole-life events stream with a 1 s to 30 s redial"
  - "Wails events desktopshare:state, desktopshare:event, desktopshare:stopped (reason restart), desktopshare:open-sheet"
  - "Tray slot under Open Portty: Share Desktop... or Desktop shared with N plus Stop Sharing"
  - "Sharing tray icon variant ranked error, sharing, progress, normal"
affects: [22-09, 22-12, 22-13]
key-files:
  created:
    - app_desktop_share.go
    - app_desktop_share_test.go
    - assets/tray_icon_sharing.png
    - assets/tray_icon_sharing@2x.png
  modified:
    - app.go
    - tray.go
    - tray_objc_darwin.m
    - tray_test.go
    - scripts/verify-icons.sh
key-decisions:
  - "The cgo exports onTrayShareDesktop and onTrayStopSharing live in tray.go beside the other //export callbacks (cgo exports belong with the file that imports C); app_desktop_share.go holds the testable methods shareDesktopFromTray and stopSharingFromTray they call"
  - "All event lines are forwarded to React unfiltered, own-account join/leave included; CONTEXT asks for one notification per join and leave for guests and own devices, so 22-09 does the coalescing and needs no mode filter for that rule"
  - "A tray refresh (icon swap) is requested only when the connected-viewer count changes, off the stream goroutine, because state lines arrive up to 10 a second and refreshTrayState does daemon round trips"
  - "The tray line shows only while a click-to-share is active; the icon follows the connected-viewer count alone, so an own device on always-available turns the icon without a line (UI-SPEC Surface 4)"
  - "The sharing glyph replaces the P_ glyph's underscore with a filled dot (diameter about a third of the canvas, one-pixel clearing ring at 18 px, two at 36 px) generated from the normal glyphs by a throwaway program kept in the scratchpad"
requirements-completed: [DSHR-06]
status: complete
commits: 2
plan_head_before: 2e60f6683715e9a7e5c4702396ce252dd291763a
plan_head_after: 61c5e38d7227b221ab12ca372f619ccc7c62e669
actuals:
  tokens: 14000
  tasks: 2
  commits: 2
---

# Phase 22 Plan 08: Tray share indicator Summary

The GUI now holds the daemon's events stream for its whole life and turns it into the tray's share slot, a one-click Stop Sharing and a sharing icon variant while anyone is connected.

## What was built

- **Task 1 (tracer, 5c333752).** `startDesktopShareEvents` runs once from `startup`, after the tray poller. Each state line is stored, sets the tray slot and `trayViewers`, and is emitted as `desktopshare:state`; event lines go out as `desktopshare:event`. When the stream ends (any error, `gui_busy` included) the stored view is cleared and the tray resets; if the share was active, `desktopshare:stopped` with reason `restart` is emitted. Redial backs off 1 s doubling to 30 s, reset by a state line. In `tray_objc_darwin.m` `menuWillOpen` the slot sits directly under Open Portty: Share Desktop... while inactive, otherwise the enabled shared-with line (opens the sheet) and Stop Sharing. Share Desktop... and the line call `showWindow()` (the Open Portty path, with its test seam) and emit `desktopshare:open-sheet`; Stop Sharing calls `DELETE /desktop/share` in a goroutine, errors logged, never a dialog. The Phase 19 desktop line is not moved.
- **Task 2 (61c5e38d).** `trayIconBytesForState` now returns error, then sharing (`trayViewers > 0`), then the quartile pair, then normal. The two PNGs were generated from `tray_icon.png` and `@2x`; `verify-icons.sh` checks both sizes.

## Precondition record

Gate files 20-10, 21-07, 21-09 and 22-06 SUMMARYs all exist. `git log --oneline 07ad7d21..HEAD -- app.go tray.go tray_objc_darwin.m tray_common.go app_desktop.go main.go` listed, and I read the stat of each:

- 3c22c9f7 feat(21-08): window occlusion observer, two lines in app.go startup
- 0a95e689 feat(20-10): Settings Showing row and consent copy (app_desktop.go)
- ec2de385 feat(20-10): ", window only" suffix in the tray line (tray_common.go)
- 42f6aef1 feat(21-07): desktop flag in the peer poll (app.go, app_peer_desktop.go)

None touches the tray menu code, `tray.go` or `tray_objc_darwin.m`, so the plan's assumptions held.

## Deviations from Plan

None needing a rule. Two small departures in placement and extras:

- The two cgo exports are in `tray.go`, not `app_desktop_share.go` (see key decisions).
- Added the change-only refresh guard and `TestDesktopShareApp_StateDrivesCountAndIconRefresh`, `_ForwardsEventsAndRedials`, `_TrayShowSheet` beyond the plan's two named tests.

The plan's tests-first order for Task 2 was followed in source order (tray tests written before the icons and the new branch) but no separate RED commit was made.

## Verification

- `go vet .`, `gofmt -l`, `golangci-lint run --new-from-rev=07ad7d21 .` clean.
- `go test -race -short -count=3 -run 'TestDesktopShareApp_|TestTrayShare_' .` pass; `go test -race -short -count=1 .` pass; `bash scripts/verify-icons.sh` 29 passed; `bash tests/check-brand.sh` clean.
- Environment note: in a fresh worktree `TestNoFakeWhoIsInRelease` fails with `pattern all:frontend/dist: no matching files found` because `frontend/dist` is gitignored and unbuilt. It is not caused by this plan; a stub `frontend/dist/index.html` (gitignored, uncommitted) made the whole package pass.
- Not run, by constraint: the real menu bar, GUI, or a live share. The ObjC compiled under `go vet`; its behavior is the VM and real-Mac check in 22-12.

## For 22-13 (TESTING.md, not edited here)

New test files: `app_desktop_share_test.go` (tray share, stream loop, redial, open-sheet) and additions to `tray_test.go` (`TestTrayIconSharing`, two new rows in `TestTrayIconAsset`). Traceability row for `app_desktop_share_test.go` against DSHR-06.

Manual-row candidates:
- Tray sharing icon is legible beside the other six tray icons in a light and a dark menu bar, at 18 px (VM and real Mac).
- The tray menu shows Share Desktop... with no share; with a live share it shows `Desktop shared with nobody yet`, then the count as a guest is admitted, and Stop Sharing ends it in one click with no confirm (VM, with a real guest).
- Killing the GUI process during a share ends the share and the next GUI launch shows no stale tray state.

## Known Stubs

None.

## Threat Flags

None. No new endpoint or file access; the stream and stop route are 22-06's.

## Self-Check: PASSED

Files found: app_desktop_share.go, app_desktop_share_test.go, assets/tray_icon_sharing.png, assets/tray_icon_sharing@2x.png. Commits 5c333752 and 61c5e38d are ancestors of HEAD.
