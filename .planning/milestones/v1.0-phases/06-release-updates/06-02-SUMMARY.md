---
phase: 06-release-updates
plan: 02
subsystem: updater
status: complete
tags: [updater, poller, notifications, tray, native-alert, wails-bindings]
requires:
  - "06-01: internal/updater Checker, NewCheckClient, NewDownloadClient, Download, Verify, Reveal, Clean, ExecRunner"
  - "06-03: frontend UpdateStatus contract, GetUpdateStatus/DownloadUpdate stubs, update:status subscription"
provides:
  - "updater.Service (Check, Download, Status), updater.Status, updater.Error{Kind, Text}"
  - "App bindings GetUpdateStatus() and DownloadUpdate() returning updater.Status; update:status events"
  - "VM seam vars updateAPI, updateAllow, updateFirstDelay, updateTick, updateEvery (app_update.go)"
  - "Help → Check for Updates… native alerts; tray Update Available… item"
affects: [06-04]
tech-stack:
  added: []
  removed: ["github.com/creativeprojects/go-selfupdate and its module tree (gitea SDK, go-github, yaml.v3, oauth2 and others)"]
  patterns: ["channel-driven timer and clock seams for the update loop", "Emit under the service mutex, non-blocking sinks"]
key-files:
  created:
    - internal/updater/service.go
    - internal/updater/service_test.go
    - app_update.go
    - app_update_test.go
  modified:
    - app.go
    - main.go
    - menu_test.go
    - tray.go
    - tray_objc_darwin.m
    - go.mod
    - go.sum
  deleted:
    - internal/updater/updater.go
    - internal/updater/updater_test.go
decisions:
  - "Service.busy covers only downloading and verifying; Check is serialized by its own mutex, so a second Check waits behind the first and then runs (a cheap 304)"
  - "Download refuses Dir == \"\" before any request, instead of relying on MkdirAll(\"\") to fail"
  - "Download and verify failures log step names only (no err text that could carry a URL or path); Verify's error names its step and holds no path"
  - "The tray update item is released after addItem (MRC); the existing items' leak was left alone"
metrics:
  duration: "~10 min"
  completed: 2026-11-18
actuals:
  tokens: 23500
  tasks: 3
  commits: 6
plan_head_before: 12b755a2ba9e776def0683c21935a6a9145d95f1
plan_head_after: e3146f62e6b361206be78c257f739bbc51db7cfd
---

# Phase 6 Plan 02: Update Wiring (Poller, Notifications, Download, Alerts, Tray) Summary

06-01's updater core is now wired into the app. A release build checks GitHub 10 s after launch. After that it checks once per 24 h of wall-clock time, looking on an hourly tick, and a failed check is retried on the next tick. Every state change reaches the banner as `update:status`. The first automatic check that finds a new version sends one native notification for it. DownloadUpdate downloads the DMG, verifies it and reveals it in Finder. Help → Check for Updates… always answers with a native alert. The tray shows "Update Available…" while a newer version is known. go-selfupdate is gone.

## Tasks

| Task | Name | Commits |
|------|------|---------|
| 1 (tracer) | Service.Check, poller, update:status, notify-once; old updater removed | 1e14fe00 (RED), a3ff5196 (GREEN) |
| 2 | Service.Download and the DownloadUpdate binding | b37727e6 (RED), fe358a56 (GREEN) |
| 3 | Help alerts and the tray item | b56d4bcb (RED), e3146f62 (GREEN) |

Tracer gate: Task 1's `<verify>` passed end to end (GATE-OK) before Task 2 started.

## What was built

- **internal/updater/service.go**: a port of GIFsmith's UpdateService with no Wails code.
  - `Status` uses the exact json tags from the 06-03 contract: state, current, latest, done, total, error.
  - `Error{Kind, Text}` has the kinds check, download, verify and reveal. Its text is the plan's copy.
  - `Check` is serialized. A failure restores the previous status. Not-installable, not-found and same-version results give current.
  - `Download` runs on its own 10-minute deadline. It cleans old files, downloads with progress, verifies against Team S2K7P43927 and reveals in Finder. Every failure returns to available with `Status.Error` set. A file that failed its digest or signature check is deleted. A verified file stays when the reveal fails.
- **app_update.go**:
  - The contract vars.
  - `newUpdateService`, wired in `NewApp`.
  - `startUpdatePoller` (gated on `updater.ParseVersion(Version)`) and `runUpdateLoop` (wall clock via `Round(0)`).
  - `autoCheck` with a 30 s timeout.
  - `onUpdateStatus`, which emits the event and sets the tray title.
  - `notifyUpdateOnce` and `rememberNotified`, which write `configDir()/update_notified` with mode 0600.
  - `updateCacheDir`, the bindings `GetUpdateStatus` and `DownloadUpdate`, `checkForUpdatesManually`, `offerUpdate`, `offerDownload`, and the `alert`/`dialog`/`showWindow`/`setTrayUpdate` helpers behind the App seams.
- **app.go**: the lastUpdate fields, `startUpdatePoller`/`runUpdateCheck`, `GetLastUpdateInfo`, `CheckForUpdates` and `updateSlug` are replaced by `updates *updater.Service` and the four seams. `startup` still calls `a.startUpdatePoller(ctx)`.
- **main.go**: the Help item is now "Check for Updates…". The callback runs `go appInstance.checkForUpdatesManually()`.
- **tray.go / tray_objc_darwin.m**: `setTrayUpdateItem` copies the title, then dispatch_async to the main queue, and retains/releases it under MRC. `menuWillOpen` adds the item after "Open Portty". `updateClicked:` calls the exported `onTrayUpdate`, which runs `offerUpdate` on a goroutine.
- **Modules**: `go mod tidy` removed go-selfupdate, and Masterminds/semver/v3 is no longer listed in go.mod. No module was added.

## Contract check against 06-03

- The bindings `main.App.GetUpdateStatus` and `main.App.DownloadUpdate` match `frontend/src/wailsjs/go/main/App.js`.
- The Status JSON keys match the `UpdateStatus` interface in `App.d.ts`. No field uses omitempty, so every key is always present, as the TS type requires.
- The event name is `update:status`, as in App.tsx. The old `GetLastUpdateInfo` and `CheckForUpdates` methods no longer exist on App.
- The log lines match the contract: `update: <v> available`, `update: up to date`, `update: no published release`, `update: release v<v> not installable: <err>`, `update: check failed: <err>`, `update: notified <v>`. They go to stderr through `log`, with its timestamp prefix.

## Gate results

- Task 1 verify: GATE-OK. The old files are gone. go.mod has no creativeprojects line and does not list semver/v3. build and vet are clean. The TestService_Check* and TestUpdatePoller_* runs, 4 tests each, passed under -race. app.go no longer emits update:available.
- Task 2 verify: GATE-OK. The TestService_Download* tests (4 at the top level), TestService_ErrorTextsHaveNoURLOrPath and TestDownloadUpdate_BindingRunsDownload passed under -race.
- Task 3 verify: all checks passed. TestManualUpdateCheck_* (6 tests), TestTrayUpdate_* (2) and TestAppMenu_* passed. The label and the tray strings are present, and no old binding is left.
- Full short suite: `go test -race -short -skip '^(TestAPIAppearance_GetPatch|TestAPITerminalFont_GetPatch)$' ./...` passed in every package on the first run. No flake reruns were needed.
- Extra: `go build -tags vmpreview .` compiles. gofmt is clean.

## Deviations from Plan

1. **[Rule 2 - Correctness] Explicit `Dir == ""` guard in Download.** The plan says an empty Dir should make Download fail with the download error. I check for it before the clean and before any request, so nothing is ever written relative to the working directory. A test covers it ("no Dir" subtest: kind download, no asset request).
2. **[Rule 1 - Correctness] Tray menu item released after addItem.** tray_objc_darwin.m uses MRC, and the menu retains its items, so the new update item gets a `release`. The existing items in menuWillOpen still leak one item per menu open. That predates this plan and was left alone.
3. **Extra test cases inside the planned files.** TestService_CheckQuietOutcomes also covers the same-version case. TestService_DownloadFailures also covers retry-after-failure and the empty Dir. TestUpdatePoller_FailureRetriedHourlyAndQuiet runs for both 500 and 403.

## Deferred / notes for 06-04

- TESTING.md rows for internal/updater/service_test.go and app_update_test.go, plus any manual M-NN item for the real alert and tray, belong to 06-04 under the plan's toolchain note. This plan did not edit TESTING.md.
- 06-04's vmpreview overrides need TestUpdateDefaults_ReleaseBuild excluded from that build (for example with a `!vmpreview` build tag). Otherwise it will fail when the vars are overridden.
- Wails will generate a `updater.Status` model into wailsjs on the next `wails build`. The hand stubs from 06-03 already cover it. Restore frontend/src/wailsjs afterwards, as usual.

## Known Stubs

None.

## Threat Flags

None. All new surface (the automatic GitHub check, the two bindings, update_notified, the updates cache, the alerts and the tray item) is covered by T-06-08..T-06-13.

## Self-Check: PASSED

- FOUND: internal/updater/service.go, internal/updater/service_test.go, app_update.go, app_update_test.go
- GONE: internal/updater/updater.go, internal/updater/updater_test.go
- FOUND commits (ancestors of HEAD): 1e14fe00, a3ff5196, b37727e6, fe358a56, b56d4bcb, e3146f62
