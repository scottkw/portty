---
phase: 19-unattended-desktop-viewing
plan: 14
subsystem: desktop-sharing
tags: [tray, lapse-notification, one-sender, usernotifications, launchd, vm]
status: complete

requires:
  - phase: 19-08
    provides: GET /desktop status and DesktopStatusResponse
  - phase: 19-09
    provides: Manager.NoteGUISeen, ClaimLapse, the daemon's deferring lapse sender
  - phase: 19-11
    provides: the daemon's notifier wiring (wireDesktopNotifier)
  - phase: 19-12
    provides: the vmdesktop lapse route and VM rig, and the V1 FAIL this plan fixes
provides:
  - "One dimmed tray line (Desktop: ...) driven by the daemon status, in Go (desktopTrayLine) and ObjC (disabled NSMenuItem); none while always-available is off"
  - "The GUI half of the one-sender lapse rule: gui=1 presence on the 5 s tray poll, POST /desktop/lapse-claim, post only after winning"
  - "Native notifications from a process with no app run loop (the launchd daemon) now deliver: private-queue hop instead of the main queue; VM row V1b PASS"
affects: [19-15, 19-16]

actuals:
  tokens: 31000
  tasks: 2
  commits: 5
plan_head_before: c8515fa59bd7ba5521a15e30abf218385b3cff42
plan_head_after: c0a580abb559cc0acd3938ae4efb3e90a1dea943

tech-stack:
  added: []
  patterns:
    - "Presence by polling: the existing 5 s tray poll carries gui=1; the daemon reads 12 s of silence as no GUI; the atomic Manager claim decides the sender"
    - "runNotificationBlock: main queue only while [NSApp isRunning], otherwise a private serial queue"

key-files:
  created:
    - app_lapse_test.go
    - internal/daemon/desktop_lapse_test.go
    - internal/desktop/lapse_hook.go
    - .planning/phases/19-unattended-desktop-viewing/evidence/V1b.run
    - .planning/phases/19-unattended-desktop-viewing/evidence/V1b.gui
    - .planning/phases/19-unattended-desktop-viewing/evidence/V1b.tagcheck
  modified:
    - tray_common.go
    - tray_common_test.go
    - tray.go
    - tray_objc_darwin.m
    - app.go
    - internal/daemon/desktop_api.go
    - internal/daemon/client.go
    - internal/desktop/export_test.go
    - notification_darwin.go
    - notification_darwin_test.go
    - .planning/phases/19-unattended-desktop-viewing/19-VM-CHECKS.md
    - .planning/phases/19-unattended-desktop-viewing/rig/p19-rig.sh
    - .planning/phases/19-unattended-desktop-viewing/deferred-items.md

key-decisions:
  - "Tailscale off, userspace and https_off all read Waiting for Tailscale in the tray: the plan names off and userspace; https_off also leaves the desktop unreachable, and showing Ready there would be wrong"
  - "lapseTried is an atomic.Int64 with a compare-and-swap, not a plain int: refreshTrayState runs on the poller and on the SetTrayProgress RPC goroutine"
  - "EnterAwaitingForTest moved from export_test.go to a normal file (lapse_hook.go) so daemon tests can drive a real Manager into a lapse; nothing outside the process can call it"
  - "V1 fix is the orchestrator's option 1 (no main-queue hop without an app run loop): decided by [NSApp isRunning], so the GUI path is byte-for-byte the same hop"
  - "p19-rig.sh build gained --skip-frontend (wails -s) so a VM build needs no pnpm in a fresh worktree"

requirements-completed: [DPRM-02, DPRM-03]

coverage:
  - id: T1
    description: "With always-available on the tray menu carries one inert Desktop line between the sessions and Quit with the exact UI-SPEC wording (singular and plural, locked with viewers, paused, Tailscale wins); with it off the menu is the old four items"
    requirement: "DPRM-02"
    verification:
      - kind: unit
        ref: "tray_common_test.go#TestBuildMenuItems_DesktopLine"
        status: pass
      - kind: unit
        ref: "tray_common_test.go#TestDesktopTrayLine"
        status: pass
    human_judgment: true
    rationale: "The disabled NSMenuItem rendering (dimmed, unclickable) is only visible in the real menu bar; no host GUI automation is allowed and the VM check was not part of this plan's rows (19-16 M-row)"
  - id: T2
    description: "One sender per lapse: gui=1 presence, exact atomic claim, the GUI posts only after winning, never on recovery, a daemon-won lapse posts nothing from the GUI, a claimed lapse is not claimed again"
    requirement: "DPRM-03"
    verification:
      - kind: integration
        ref: "internal/daemon/desktop_lapse_test.go#TestDesktopLapse_PresenceAndClaim"
        status: pass
      - kind: integration
        ref: "internal/daemon/desktop_lapse_test.go#TestDesktopLapse_ClaimIsAtomicAndExact"
        status: pass
      - kind: integration
        ref: "internal/daemon/desktop_lapse_test.go#TestDesktopLapse_GUIRunningOneNotification"
        status: pass
      - kind: integration
        ref: "internal/daemon/desktop_lapse_test.go#TestDesktopLapse_GUIQuitDaemonPosts"
        status: pass
      - kind: unit
        ref: "app_lapse_test.go#TestRefreshTray_ClaimsAndPostsOnce"
        status: pass
      - kind: unit
        ref: "app_lapse_test.go#TestRefreshTray_SkipsClaimedLapse"
        status: pass
      - kind: vm-row
        ref: ".planning/phases/19-unattended-desktop-viewing/19-VM-CHECKS.md#V1b"
        status: pass
    human_judgment: false
  - id: T3
    description: "With the GUI quit the launchd daemon's lapse notification is delivered (record in the user notification database, delivery accepted), a repeat lapse replaces it, and with the GUI running the GUI claims and posts instead of the daemon"
    requirement: "DPRM-03"
    verification:
      - kind: vm-row
        ref: ".planning/phases/19-unattended-desktop-viewing/19-VM-CHECKS.md#V1b"
        status: pass
      - kind: unit
        ref: "notification_darwin_test.go#TestNotificationUsesMainQueue_FalseWithoutApplication"
        status: pass
    human_judgment: true
    rationale: "Banner on screen was not observed (VM screenshots are blank); delivery is shown by the usernoted record and hasError 0. The office-Mac M-row repeats it on the real Mac"

duration: about 1h 45m
completed: 2026-11-26
---

# Phase 19 Plan 14: Tray desktop line, one-sender lapse notification, daemon notification fix Summary

The tray names the desktop's state in one dimmed line, the running GUI claims and posts the lapse notification (otherwise the daemon does), and a headless launchd daemon can now actually deliver it: V1's "never delivered" was a main-queue hop nothing serviced, fixed and re-proved in the VM as row V1b.

## What was built

- **Task 1 (tracer, `b39fb079`):** `MenuItemInfo`, `BuildMenuItems(sessions, desktopLine)`, `desktopTrayLine` (the 19-UI-SPEC Tray column in Go; "" while off); `trayDesktopTitle` and `setTrayDesktopTitle` in `tray_objc_darwin.m` with a disabled, action-less NSMenuItem plus a separator before Quit; `setTrayDesktopLine` in `tray.go`; `refreshTrayState` reads the status and sets or clears the line.
- **Task 2 (TDD; RED `6cfe9643`, GREEN `e568d858`):** `GET /desktop?gui=1` calls `Manager.NoteGUISeen`; the status carries `lapseClaimedSeq`; `POST /desktop/lapse-claim` (strict body) answers `{"claimed": bool}`; `DesktopStatusAsGUI` and `ClaimDesktopLapse` on the client; `refreshTrayState` polls as the GUI and `claimAndPostLapse` claims each new awaiting_approval lapse once, posting only on a win, never on recovery or the approaching-lapse warning.
- **Orchestrator-added V1 fix (`8bc4acf7`, evidence `c0a580ab`):** `sendNotification` queued its work with `dispatch_async` on the main queue, which the headless daemon never services. `runNotificationBlock` keeps the main-queue hop while `[NSApp isRunning]` (the GUI) and otherwise uses a private serial queue. A host unit test pins the decision without posting anything.

## V1 verdict (named as the plan requires): V1b PASS, the daemon path is verified

Root cause confirmed by reading the path first: the Go log line is written before the cgo call, the Objective-C log line is inside the queued block, so V1's evidence (Go line present, Objective-C line absent, no usernoted record) is exactly "the block never ran". A scratch program with Cocoa and no NSApplication printed `mainQueueBlockRan=0 privateQueueBlockRan=1`. After the fix, in portty-test with the GUI quit (build 19.1.2, vmdesktop tag, Developer ID): `lapse claimed lapse=1 by=daemon`, then `(main queue: 0)` and `notification delivery accepted`, the notification database went 2 to 3 with `portty.desktop-approval-lapse`, and a second lapse replaced it (count stayed 3). Control with the GUI running: the daemon saw `gui=true`, the GUI claimed 3 s later (`by=gui`, `main queue: 1`), the daemon never posted. 19-16 therefore lists the daemon path as verified, not a known gap. The VM was left on the untagged 19.1.3 build, always-available off, GUI quit, launchd daemon running, route answering 404.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Daemon tests could not drive a lapse**
- **Found during:** Task 2
- **Issue:** the plan's daemon-level tests need a real Manager in awaiting_approval, but `EnterAwaitingForTest` lived in `internal/desktop/export_test.go`, invisible to another package's tests, and the daemon's Manager runs on the real clock so the 12 s and 15 s windows cannot be waited out.
- **Fix:** moved `EnterAwaitingForTest` to `internal/desktop/lapse_hook.go` (not reachable from outside the process). `TestDesktopLapse_GUIQuitDaemonPosts` therefore proves "no gui=1 poll means the daemon posts at the transition" rather than "a GUI that stopped polling 20 s earlier"; the 12 s presence window and the 15 s claim wait stay pinned by 19-09's fake-clock tests `TestLapseSender_GUISilentDaemonPosts` and `TestLapseSender_GUIClaims`. The names the plan lists all exist.
- **Files modified:** internal/desktop/lapse_hook.go, internal/desktop/export_test.go
- **Commit:** `6cfe9643`

**2. [Rule 2 - Missing critical] `lapseTried` is atomic**
- **Found during:** Task 2
- **Issue:** the plan says `a.lapseTried int`, but `refreshTrayState` also runs from the `SetTrayProgress` RPC goroutine, so a plain int is a data race and two refreshes could both claim.
- **Fix:** `atomic.Int64` with a compare-and-swap before the claim. **Commit:** `e568d858`

**3. [Plan text] Tailscale `https_off`**
- The plan lists off and userspace for "Waiting for Tailscale"; `https_off` is the third refusal verdict the daemon reports and also leaves the desktop unreachable, so it reads the same. `TestDesktopTrayLine` pins all three. **Commit:** `b39fb079`

**4. [Rule 3 - Rig] `build --skip-frontend`**
- A fresh worktree has no `frontend/dist` or `node_modules`; `wails build` would run `pnpm install`. The rig gained `--skip-frontend` (wails `-s`) and the existing build artifacts were copied into the ignored `frontend/dist`. **Commit:** `c0a580ab`

**Total deviations:** 4 (1 blocking test seam, 1 race fix, 2 plan-shape notes). **Impact:** none on behaviour; the V1 fix itself is the orchestrator's added task, not a deviation.

## Authentication gates

None.

## Issues Encountered

- **Pre-existing, not from this plan:** besides the three host-flaky loopback media tests 19-12 logged, `TestDesktopAPI_OffClosesViewers` and `TestDesktopAPI_RegenerateKey` also failed with `no keyframe within 10s` on this host (the failing set varies run to run). Logged in `deferred-items.md`; every `TestDesktopLapse_*` and the other `TestDesktopAPI_*` pass, `internal/desktop` passes with `-race`.
- The tray line's visual rendering (dimmed, unclickable) was not observed: no host GUI automation is allowed and the plan has no VM row for it. It is a candidate manual M-row for 19-16.
- Banner on the guest screen not observed for V1b (VM screenshots are blank); the usernoted record, `hasError: 0` and the delivery-accepted log stand in.
- TESTING.md was not touched (19-16 owns the consolidated update); the new test files `app_lapse_test.go` and `internal/daemon/desktop_lapse_test.go` need traceability rows there.

## Known Stubs

None.

## Threat Flags

None new. T-19-46 mitigated (atomic claim, daemon fallback, fixed identifier replaces, shown in V1b.run); T-19-47 holds (fixed strings only; evidence scrubbed of tailnet addresses, names, tokens and home paths). The new `POST /desktop/lapse-claim` is on the same 0600 same-user socket as the rest of the daemon API and takes a strict one-field body.

## Self-Check: PASSED

- Files exist: tray_common.go, tray_common_test.go, tray.go, tray_objc_darwin.m, app.go, app_lapse_test.go, internal/daemon/desktop_lapse_test.go, internal/daemon/desktop_api.go, internal/daemon/client.go, internal/desktop/lapse_hook.go, notification_darwin.go, evidence/V1b.run, V1b.gui, V1b.tagcheck, 19-VM-CHECKS.md (row V1b).
- Commits `b39fb079`, `6cfe9643`, `e568d858`, `8bc4acf7`, `c0a580ab` are ancestors of HEAD; `commits: 5` measured with `git rev-list --count` from the ledger base.
- Acceptance criteria: `trayDesktopTitle` 5 (>=3), `Desktop: Waiting for approval at the Mac` 1, `MenuItemInfo` 2, `/desktop/lapse-claim` 2, `ClaimDesktopLapse(` 1, `SetDesktopNotifier(sendNotification)` in cmd_daemon.go 1; `-list` counts 2 and 5 meet the minimums.
- Verification ran clean: gofmt, `go vet`, golangci-lint (0 issues), `go test -race` for the tray, refresh, notification and `TestDesktopLapse_*` tests, `go test ./internal/desktop/`, `bash tests/check-brand.sh`.
