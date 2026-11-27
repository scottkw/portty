---
phase: 22-click-to-share-for-tailnet-people
plan: 09
subsystem: gui
tags: [wails, notifications, tray, click-to-share]
requires:
  - phase: 22-06
    provides: DaemonClient click-to-share methods and ShareError codes
  - phase: 22-08
    provides: events loop, desktopshare:event/state/stopped forwarding
provides:
  - seven never-throwing Wails sharer bindings with JS declarations
  - Admit window-forward and join/leave notifications with 5 s coalescing
  - notification click routing for portty.desktop-admit/join/leave
  - Quit GUI Only ends an active click-to-share
affects: [22-10, 22-11, 22-12, 22-13]
tech-stack:
  added: []
  patterns: [notifyCoalescer keyed by notification ID, appActiveFunc/shareNowFunc test seams]
key-files:
  created: []
  modified:
    - app_desktop_share.go
    - app_desktop_share_test.go
    - app_update.go
    - app.go
    - tray.go
    - tray_objc_darwin.m
    - frontend/src/wailsjs/go/main/App.d.ts
    - frontend/src/wailsjs/go/main/App.js
key-decisions:
  - "Quit GUI Only ends the share before hiding, with a 3 s bounded wait; a timeout is logged and treated as fine because the stop carries on in the daemon"
  - "Own-account join/leave notify too (device name), as CONTEXT says; no filtering of mode own"
  - "A coalesced burst keeps the notification identifier so the newer banner replaces the older; window is 5 s since the previous same-kind event"
status: complete
commits: 2
plan_head_before: 109bf60121b95dfc4f140f09b83a69f4a0aaf397
plan_head_after: 7a30b98f
actuals:
  tokens: 14000
  tasks: 2
  commits: 2
---

# Phase 22 Plan 09: GUI sharer actions and attention signals Summary

Seven typed Wails bindings for the share sheets, an Admit knock that always brings Portty forward (banner only when Portty is not the active visible app), coalesced join/leave banners, click routing, and Quit GUI Only ending the share first.

## Precondition and upstream log

Gate passed: 21-07, 21-09 and 22-08 SUMMARY files all exist. `git log --oneline 07ad7d21..HEAD -- app.go app_update.go App.d.ts App.js` listed, and was read before editing:

- 5c333752 feat(22-08): GUI liveness stream and the tray share slot
- 0298897d test(21-09): Cmd+V paste flow and ReadHomeClipboard, RED
- 3c22c9f7 feat(21-08): app states, Local Network and Refused cards, hidden tab and window, wake, sidebar feed
- 0a95e689 feat(20-10): Settings Showing row, Row 4 control state, consent copy that says control
- 42f6aef1 feat(21-07): desktop flag in the peer poll, Open in Browser and Local Network settings
- 0d33e1ef feat(21-07): ConnectPeerDesktop binding returning a token-free loopback URL

No textual conflict with the bindings or QuitGUIOnly (it hides the window and Dock icon, keeps GUI and tray alive, as assumed).

## What was built

Task 1 (03f9a9e7):
- `StartDesktopShare`, `GetDesktopShare`, `StopDesktopShare`, `AdmitDesktopGuest`, `DenyDesktopGuest`, `SetDesktopViewerRights`, `DisconnectDesktopViewer` in `app_desktop_share.go` with `DesktopShareResult`, `DesktopShareInfo`, `DesktopShareActionResult`. Nil client is `unavailable`; a daemon `*ShareError` keeps its code and detail; other errors are `start_failed` (start), `stop_failed` (stop), `unavailable` (the rest) with the local error text. The URL is returned only by Start and Get and is never logged.
- Declarations and Call shims in `App.d.ts` and `App.js`.

Task 2 (7a30b98f):
- `handleShareLine` routes stream lines; every event line is still forwarded to React (own included), then `announceShareEvent` runs.
- `admit_request`: `showWindow()`, plus notification `portty.desktop-admit` "<login> wants to see your desktop." only when `appActiveAndVisible()` is false.
- `notifyCoalescer`: join and leave sentences, 2 or more of a kind within 5 s become "N viewers joined/left your desktop." under the same identifier. Names: login (guest) or device (own), re-sanitized with `desktop.SanitizeName`, capped at 80 runes plus an end ellipsis.
- `handleNotificationClick` routes the three IDs to `shareNotificationClicked` (window forward; `desktopshare:open-sheet` when a share is active and nothing is pending). The update ID is unchanged.
- `QuitGUIOnly` calls `stopShareForQuit` before hiding (3 s bounded wait, timeout logged and tolerated).
- `isAppActiveAndVisible` C helper in `tray_objc_darwin.m` (main-queue read of `NSApp.isActive` and a visible, non-minimized key or main window) behind the `appActiveFunc` Go seam.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] tray.go touched though not in files_modified**
- **Found during:** Task 2
- **Issue:** cgo needs the `isAppActiveAndVisible` declaration and a Go wrapper in the file that holds the cgo preamble (`tray.go`).
- **Fix:** added the declaration and a 3-line wrapper.
- **Files modified:** tray.go
- **Commit:** 7a30b98f

**2. [Rule 1 - Test hygiene] Existing share harness given no-op notification/window seams**
- **Found during:** Task 2
- **Issue:** `TestDesktopShareApp_ForwardsEventsAndRedials` sends a `join` event with no `sendNotificationFunc`; joins now notify.
- **Fix:** `newShareHarness` sets no-op `sendNotificationFunc`, `showWindowFunc`, `appActiveFunc` instead of adding nil guards in production code.

**3. QuitGUIOnly itself is not unit-tested end to end.** It needs a Wails context (`runtime.WindowHide`) and `setDockVisible` (real AppKit). The quit logic is `stopShareForQuit`, tested directly (active share stops, none does not, a slow stop does not block); `QuitGUIOnly` calls it as its first action after the ctx check. Ordering "before hiding" is therefore verified by reading, and belongs in a manual row.

## Verification

- `go vet .`, gofmt, `golangci-lint run --new-from-rev=07ad7d21 .`: clean.
- `go test -race -short -count=3` of `TestDesktopShareBinding_`, `TestShareNotification_`, `TestDesktopShareQuit_`, `TestNotificationClick`: pass. `go test -race -short -count=1 .`: pass.
- `tsc --noEmit -p tsconfig.json` clean (run as `frontend/node_modules/.bin/tsc` with a temporary, uncommitted symlink to the main checkout's node_modules, because the fresh worktree has none; `pnpm exec` refused the symlink). `tests/check-brand.sh` clean.
- A local gitignored `frontend/dist/index.html` stub was used for tests; not committed.

## For 22-13 (TESTING.md)

New tests, all in `/app_desktop_share_test.go` (already listed in the suite group):
- `TestDesktopShareBinding_NilClientNeverThrows`, `_StartAndGet`, `_StartRefusalsPassThrough`, `_TransportErrors`, `_ActionsPassIDAndMapRefusals`
- `TestShareNotification_AdmitBringsWindowForwardAndNotifiesOnlyWhenHidden`, `_JoinAndLeaveSentences`, `_CoalescesWithinFiveSeconds`, `_NameIsCappedAtEightyWithEllipsis`, `_ClickRouting`, `_NoActionButtonsOrLinkInBody`
- `TestDesktopShareQuit_StopsAnActiveShareFirst`, `_NoShareNoStop`, `_SlowStopDoesNotBlockHiding`

Manual-row candidates (VM only, never the host):
- Admit request while Portty is hidden: window comes forward and a banner appears; while frontmost: no banner (backstop for the UI-SPEC "captured display may silence banners" row).
- Clicking a join/leave/admit banner brings Portty forward and opens the Share Desktop sheet only when a share is active and nothing is pending.
- Quit GUI Only while sharing: share ends (guest page shows ended), tray indicator clears, window hides.
- Real `isAppActiveAndVisible` answer with Portty frontmost vs. another app frontmost vs. minimized window.

## Known Stubs

None.

## Threat Flags

None. Mitigations T-22-31 (sanitized, capped names), T-22-32 (URL only in two bindings, never logged or emitted; binding test) and T-22-33 (Quit GUI Only ends the share) are implemented.

## Self-Check: PASSED

Files and both commits (03f9a9e7, 7a30b98f) verified present before this summary was committed.
