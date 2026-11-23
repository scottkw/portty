---
phase: 15-v1-2-hardening
plan: 06
subsystem: update notification click (native tray, Go app, React banner)
tags: [go, cgo, objective-c, usernotifications, react, wails-events]
status: complete

requires:
  - phase: 15-05
    provides: App.tsx edits earlier in the phase (wave order)
provides:
  - "A click on the Portty X is available notification reaches Go (onNotificationClick), which shows the window and emits update:show"
  - "update:show clears dismissedUpdate so the banner returns in its available state; nothing downloads"
  - "The notification delegate is registered at launch from initStatusItem"
affects: [15-09 TESTING.md M-152 and traceability rows]

actuals:
  tokens: 4200
  tasks: 2
  commits: 2
plan_head_before: cc610442f519e8f3fafabbc81cb55c8a4d47d601
plan_head_after: 955d1bb1322554cb95545c747e451039a205ba0c
commits: 2

tech-stack:
  added: []
  patterns:
    - "Native callback copies the C string, captures trayCallbackApp, and hands work to a goroutine (same rule as onTrayQuit)"
    - "A Wails event with no payload (update:show) asks the UI to undo a local dismissal"

key-files:
  created: []
  modified:
    - tray_objc_darwin.m
    - tray.go
    - app_update.go
    - app_update_test.go
    - frontend/src/App.tsx
    - frontend/src/components/__tests__/App.update.test.tsx

key-decisions:
  - "Click shows window plus the existing banner and never starts a download (consent stays with the Download button); no new copy or layout."
  - "handleNotificationClick is unexported so Wails does not bind it to the frontend; it acts only on the exact updateNotificationID."
  - "registerNotificationDelegate() is also called in initStatusItem's main-queue block so a click on a notification from an earlier run is delivered; it stays idempotent and bundle-guarded."

requirements-completed: []

coverage:
  - id: D1
    description: "handleNotificationClick shows the window and emits update:show for the update notification id, starts no download, and ignores other ids"
    verification:
      - kind: unit
        ref: "app_update_test.go#TestNotificationClick_ShowsWindowAndBanner, TestNotificationClick_IgnoresOtherIDs"
        status: pass
  - id: D2
    description: "update:show brings the dismissed available banner back with Download and no DownloadUpdate call; idle shows nothing; web mode never subscribes"
    verification:
      - kind: unit
        ref: "frontend/src/components/__tests__/App.update.test.tsx#update:show after a dismissal shows the available banner again and downloads nothing"
        status: pass
  - id: D3
    description: "Live click on a notarized build brings Portty forward with the banner and no download"
    verification:
      - kind: manual_procedural
        ref: "TESTING.md M-152 (added by 15-09, Tart VM portty-test)"
        status: unknown
---

# Phase 15 Plan 06: Update notification click Summary

A click on the update notification now runs app code: the ObjC delegate hands the default action to a cgo export, Go shows the window and emits `update:show`, and React un-dismisses the existing banner. No download starts.

## Tasks

| Task | Name | Commit |
| ---- | ---- | ------ |
| 1 | Native default-action handler, launch-time delegate registration, cgo export, Go handler with tests | 58bd89bb |
| 2 | Banner returns on update:show, no download until Download is pressed | 955d1bb1 |

## What changed

- `tray_objc_darwin.m`: `didReceiveNotificationResponse` on `PorttyNotificationDelegate` calls `onNotificationClick` for the default action only, and always calls `completionHandler()`. `initStatusItem` registers the delegate at launch.
- `tray.go`: `//export onNotificationClick` copies the string and runs `go app.handleNotificationClick(id)`.
- `app_update.go`: `updateNotificationID` constant (also used by `notifyUpdateOnce`) and `handleNotificationClick`.
- `app_update_test.go`: `updateHarness.names` records every event; two new tests.
- `App.tsx`: `EventsOn('update:show', () => setDismissedUpdate(null))`, unsubscribed in the same cleanup.

## TDD

- RED: Go tests failed to compile (`handleNotificationClick undefined`) before the implementation. The vitest dismissal test was mutation-checked: replacing the handler body with a no-op fails it (1 failed, 11 passed), and the file was restored.
- RED and GREEN landed in one commit per task (a compile-failing test commit would have left the tree unbuildable).

## Verification

Logs: `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.a8gFVd4lRD` (Go: vet, test -race, build) and `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.rYSEiw1X6P` (vitest, tsc).

- `go vet .`, `go test -race` for `TestNotificationClick_*` plus the two update poller tests, and `go build .` (compiles the ObjC through cgo): pass.
- Region-scoped gate: `handleNotificationClick` body contains no `Download` (0).
- vitest `App.update.test.tsx` + `App.update-needed.test.tsx`: 15 passed. `tsc --noEmit`: clean.
- Acceptance greps: didReceiveNotificationResponse 1, extern onNotificationClick 1, registerNotificationDelegate() 2, completionHandler() 1, //export onNotificationClick 1, handleNotificationClick( 1, updateNotificationID 4, wire literal still in tests.
- Not run here (by plan): the full Go and vitest suites (15-09), and the live click (TESTING.md M-152, notarized build in the Tart VM; the delegate cannot run in an unbundled process).
- The known `SharePopover > Copy writes the full link` flake was not exercised.

## Deviations from Plan

None - plan executed exactly as written.

## Known Stubs

None.

## Threat Flags

None. The new native-to-Go entry point (`onNotificationClick`) is the boundary the plan's threat model (T-15-14, T-15-16) already covers.

## Self-Check: PASSED

- Files modified exist; commits 58bd89bb and 955d1bb1 are on gsd/v1.2.
