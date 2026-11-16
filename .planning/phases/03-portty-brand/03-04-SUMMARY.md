---
phase: 03-portty-brand
plan: 04
subsystem: app-identity-menu-tray
status: complete
tags: [brand, bundle-id, about-panel, menu, tray, notification]
requires:
  - "Brand assets plan (03-03): final icons in the bundle and the two-buffer tray code in tray_objc_darwin.m"
provides:
  - "wails build writes build/bin/Portty.app (Contents/MacOS/Portty), bundle ID app.portty.Portty"
  - "Native AppKit About panel: showAboutPanel() (about_darwin.m), showNativeAboutPanel() (about_darwin.go), menuShowAbout seam (main.go)"
  - "App menu Portty: About Portty, Settings…, Hide Portty, Quit Portty; Help = Portty Help / Check for Updates (GitHub item removed)"
  - "tray_common.go: Portty tooltip, Open Portty / Quit Portty, quitNotificationID, quitNotificationTitle, quitToMenuBarBody(n)"
  - "TESTING.md: BRND-01/NOTF-01 rows, LOOK-01 row rewritten, M-41 rewritten, M-72, M-73 (Category AN)"
affects: [03-05, 03-06]
tech-stack:
  added: []
  patterns: ["cgo + separate .m pair with dispatch_async to the main queue (same as appearance_darwin)", "Pure copy function for notification text so it can be tested without a Wails runtime"]
key-files:
  created:
    - about_darwin.go
    - about_darwin.m
  modified:
    - wails.json
    - build/darwin/Info.plist
    - build/darwin/Info.dev.plist
    - main.go
    - menu_test.go
    - tray_common.go
    - tray_common_test.go
    - tray_objc_darwin.m
    - app.go
    - notification_darwin_test.go
    - TESTING.md
decisions:
  - "wails.json name and outputfilename are both Portty, which gives Portty.app (CONTEXT Research follow-ups win over the UI-SPEC lowercase name)"
  - "The bundle ID app.portty.Portty is the same in Info.plist and Info.dev.plist; macOS notification permission and WebKit storage start fresh"
  - "About panel is a sibling cgo pair about_darwin.go/.m, not part of tray_objc_darwin.m; it sets only credits and a blank build version, and the icon, name, version and copyright come from the bundle"
  - "The GitHub Help item and openGitHubCallback are removed, not renamed"
metrics:
  duration: "~25 min"
  completed: 2026-11-16
  tasks: 2
  files: 13
estimate:
  tokens: 70000
actuals:
  tokens: 10200
  tasks: 2
  commits: 2
plan_head_before: e71674f63cf65a7cbed2646ef1fd7e7433354798
plan_head_after: c6967566ccea3f3976d493bf545022d82deb7434
---

# Phase 3 Plan 04: Portty App Identity Summary

`wails build` now writes build/bin/Portty.app with bundle ID app.portty.Portty. The window and app menu say Portty, and About Portty opens the native AppKit panel through a tested `menuShowAbout` seam. The tray tooltip, tray menu and quit-to-menu-bar notification say Portty, and their 0/1/n copy comes from one pure function that the tests check.

## What was built

**Task 1 (tracer), c6196289**
- wails.json: `name` and `outputfilename` are Portty. companyName and productName are Portty, copyright is `Copyright © 2026 Ken Scott. MIT License.`, comments is `A macOS terminal with remote access over Tailscale.` productVersion stays 2.2.0.
- Info.plist and Info.dev.plist: CFBundleDisplayName is Portty and CFBundleIdentifier is app.portty.Portty in both files. Nothing else changed.
- about_darwin.m (new): `showAboutPanel(void)` uses dispatch_async to the main queue. It builds a centered credits string in the small system font: the description in secondaryLabelColor, then `portty.app` with NSLinkAttributeName set to https://portty.app. It then calls `activateIgnoringOtherApps` and `orderFrontStandardAboutPanelWithOptions` with the credits and an empty NSAboutPanelOptionVersion. The file is non-ARC, and every allocated object is released.
- about_darwin.go (new): cgo wrapper `showNativeAboutPanel()`.
- main.go: header comment updated; the AugmentServicePath comment now names only the Tailscale CLI; Title is Portty; new `menuShowAbout` seam; app submenu is Portty with About Portty, a separator, Settings… ⌘,, a separator, Hide Portty ⌘H, a separator, Quit Portty ⌘Q. The GitHub help item and `openGitHubCallback` are deleted.
- menu_test.go: the structure, accelerator and event tests now use the Portty menu, and the new TestAppMenu_AboutOpensNativePanel checks that a click makes one About call, emits no event, opens no URL, and that the item has no accelerator.

**Task 2, c6967566**
- tray_common.go: the tooltip reads `Portty — no sessions / 1 session / %d sessions`. The menu reads `Open Portty` … `Quit Portty`. New constants `quitNotificationID = "portty.quit-gui-only"` and `quitNotificationTitle = "Portty"`, and new `quitToMenuBarBody(n)` returning 0/1/n copy.
- app.go QuitGUIOnly: calls `a.sendNotificationFunc(quitNotificationID, quitNotificationTitle, quitToMenuBarBody(count))`. The old body code (and its "sessions active" text) is gone.
- tray_objc_darwin.m: PorttyMenuDelegate, PorttyNotificationDelegate and the (PorttyTray) category; menu titles `Open Portty` / `Quit Portty`; tooltip `Portty`; every NSLog prefix is `Portty: `; two comments updated. No behaviour change.
- tray_common_test.go: tests updated for the Portty strings, and new TestQuitToMenuBarBody covers 0/1/3, rejects "1 sessions" and "0 sessions", and checks the ID and title constants. notification_darwin_test.go now calls `sendNotification("portty.test", "Portty", "test")`.
- TESTING.md: added the Section 2 note for 03-04 (counts unchanged at 208), rewrote the LOOK-01 menu_test.go row, added 3 BRND-01/NOTF-01 rows, rewrote M-41 (Portty build, 0/1/n copy, no step to stop another app), and added Category AN with M-72 (About panel) and M-73 (signed-build identity).

## Gate results

| Gate | Result |
|------|--------|
| `go build ./... && go vet ./... && go test -race -short ./...` | PASS at both task boundaries (no flake hit) |
| Menu tests `-run 'TestAppMenu_\|TestMacWindowOptions'` | 6 `--- PASS: TestAppMenu_*` (needs 6) + TestMacWindowOptions |
| Tray/notification tests | 7 PASS (TestTrayTooltip, 3 x TestBuildMenuItems*, TestQuitToMenuBarBody, TestSendNotification_NoBundleReturnsCleanly, TestHasAppBundleID_FalseWhenUnbundled) |
| `wails build -platform darwin/arm64 -tags wailsassets` | PASS: `build/bin/Portty.app/Contents/MacOS/Portty` |
| `git checkout -- frontend/src/wailsjs/wailsjs/` | Done; `git status --porcelain -- frontend/src/wailsjs` is empty |
| Old-name git grep (`*.go *.m wails.json build/darwin/*`, excluding the four 03-05 webserver files) | exit 1 (no hits) |
| `bash tests/check-traceability-paths.sh` | OK: 102 traceability paths exist |
| Task 1 + Task 2 acceptance greps | All match the expected counts |

Values in the built Info.plist (`plutil -extract … raw build/bin/Portty.app/Contents/Info.plist`):

| Key | Value |
|-----|-------|
| CFBundleIdentifier | app.portty.Portty |
| CFBundleName | Portty |
| CFBundleDisplayName | Portty |
| CFBundleExecutable | Portty |
| CFBundleGetInfoString | A macOS terminal with remote access over Tailscale. |
| NSHumanReadableCopyright | Copyright © 2026 Ken Scott. MIT License. |
| CFBundleShortVersionString | 2.2.0 |

Tracer feedback gate: the Task 1 `<verify>` is fully automated and passed end to end. The only change between that run and the commit was a comment edit in about_darwin.m, after which `go build ./...` passed again. Then expansion to Task 2 started.

Not run: frontend `tsc` and `vitest`, because this plan touched no frontend files.

## Deviations from Plan

**1. [Rule 1 - Acceptance] about_darwin.m comment reworded**
- **Found during:** Task 1 acceptance check
- **Issue:** a comment that named `NSAboutPanelOptionVersion` made `grep -c` print 2, but the acceptance check requires exactly 1
- **Fix:** the comment now says "Empty build version". The code is unchanged.
- **Commit:** c6196289

No other deviations. The plan was executed as written.

## Orchestrator reminders

- The Tart VM deploy helper (~/.local/share/portty-vm-tools) must copy **build/bin/Portty.app** from now on, not `agenthub.app`. The old build/bin/agenthub.app was removed by `rm -rf build/bin` before the build (build/bin is untracked output).
- M-72 (About panel, Light/Dark, link rendering) and M-73 (signed build, notification permission under app.portty.Portty) are end-of-phase backstops. M-72 must record whether `portty.app` shows as a clickable link or as plain text.
- BRND-05 stays open until the 03-06 zero-hit gate, and no requirements were marked complete here. The four webserver files plan 03-05 owns still contain the old name, by design.

## Known Stubs

None.

## Self-Check: PASSED

- FOUND: about_darwin.go, about_darwin.m, build/bin/Portty.app/Contents/MacOS/Portty
- FOUND: c6196289, c6967566 (ancestors of HEAD)
