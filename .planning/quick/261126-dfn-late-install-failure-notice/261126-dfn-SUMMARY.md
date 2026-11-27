---
phase: quick-261126-dfn
plan: 01
subsystem: updater
status: complete
tags: [self-install, mover, notification, UPD-09, WR-02]
requires: [18-REVIEW-FIX WR-02 (revealLeft, consumeInstallResult mover rules)]
provides:
  - "Installer-posted notice for a late mover failure with no GUI left"
  - "Mover mode alerts an earlier failed or rolled-back result once, before staging"
affects: [internal/updater, app_mover.go, app_install.go]
tech-stack:
  added: []
  patterns: ["osascript display notification with the body as argv, through the installer's env.Run"]
key-files:
  created:
    - .planning/quick/261126-dfn-late-install-failure-notice/261126-dfn-PLAN.md
  modified:
    - internal/updater/install.go
    - internal/updater/installer.go
    - internal/updater/installer_test.go
    - internal/updater/selfinstall_test.go
    - app_mover.go
    - app_install.go
    - app_mover_test.go
    - TESTING.md
    - .planning/phases/18-self-installing-updates/18-VM-CHECKS.md
decisions:
  - "The installer posts with osascript, not UNUserNotificationCenter. A VM probe showed the UN path is refused for an unregistered bundle, and shows only a permission prompt for a never-authorized one (a first install)."
  - "Mover mode puts a keepDaemon result back after showing it, so the installed copy's next start still keeps the daemon (WR-04). That copy may then show the failure a second time."
metrics:
  completed: 2026-11-26
actuals:
  tokens: 4800
  tasks: 3
  commits: 6
plan_head_before: 57e84b9a35d173ae4434bd8ad62e339b643bff6e
plan_head_after: 23d18c0eab8554c94aaf46d1dc1c3c392ff97d52
---

# Quick 261126-dfn: Late Install Failure Notice Summary

Closes the RC-WR02 / observation 2 residual. When a mover install fails late with no Portty GUI
left, the installer now posts "Portty couldn't install version X." through osascript. Re-opening
the DMG's app shows the earlier failure in an alert before it moves again.

## What changed

- `updater.Notify` runs `/usr/bin/osascript` with `display notification (item 1 of argv) with title "Portty"`. The body is passed as argv, so it never becomes script text.
- `updater.ResultText` is now the one source of the failed and rolled-back sentences. The installer and `consumeInstallResult` both use it.
- `RunInstaller`:
  - `revealLeft` posts `ResultText(r)` right after the Finder reveal. It still runs only for mover jobs with no GUI of the target. A failed post is only logged ("installer: failure notice not posted"). The reveal itself is unchanged.
  - The mover's quit-timeout path now calls `revealLeft` too. It was the one listed late-failure case that showed nothing.
- `SafeRunner` refuses `/usr/bin/osascript` in a test binary, so no test can post a real notification.
- `runMover` first calls `showEarlierResult`, before it stages anything or writes a job:
  - It takes `update-result.json` and logs the outcome, the versions and the reason, never the DMG path.
  - A failed or rolled-back result is shown as an alert through `a.alert`. A result for the same or an older version is shown too.
  - A `keepDaemon` result is put back. Then the normal Replace/move flow runs.

## How the installer posts (requirement 1)

The installer runs straight from the run-* copy. LaunchServices does not start it, and it has no
main run loop: Wails locks the main goroutine, but nothing drains the main dispatch queue that
`sendNotification` posts to. Probe in portty-test (macOS 27.2): a scratch bundle started directly
in the GUI session, the way the installer is.

- `UNUserNotificationCenter`, bundle unregistered: `Notifications are not allowed for this application` at once.
- `UNUserNotificationCenter`, bundle registered but its identifier never authorized (the first-install case): usernoted showed only the `com.apple.notificationcenter.askpermissions` prompt, never the message. It removed the prompt when the process exited.
- `osascript display notification` from the same context: delivered and presented as a banner (`app:"com.apple.ScriptEditor2"`), with no prompt.

osascript is the reliable option. The cost: the banner carries the Script Editor icon, and clicking
it opens Script Editor. The text and the "Portty" title still say what happened.

## Verification

- RED, then GREEN, for both tasks: `3fb7562c` then `275a226c`; `440d7733` then `3d9f7be0`.
- `gofmt -l *.go internal/`: clean
- `go vet ./internal/updater/ .`: clean
- `go vet -tags vmpreview .`: clean
- `golangci-lint run --new-from-rev=c714c8ef ./...`: 0 issues
- `go test -race -count=1 ./internal/updater/ .`: ok (updater 3.8 s, main 24.0 s)
- `bash tests/check-brand.sh`: clean
- `bash tests/check-traceability-paths.sh`: OK (236 paths)
- QT-WR02: PASS (2026-11-26 16:43-16:47 UTC, portty-test). It ran on a 1.8.1 rig build of `65925382`, executable sha256 `778bd91d75024767...`, ad-hoc, `verify-mode` `adhoc`. The earlier PENDING was wrong about a soak: the copy in `/Applications` was the RC-WR02 end state, and the soak never ran. That copy was backed up, removed, and restored afterwards (same sha256 `f6a9cb6c...`, GUI and daemon running, no sessions, config files put back, Downloads consent reset, `~/p18` removed). The method is RC-WR02's: a first install from `~/Downloads` with no installed copy. `verify-mode` flips to `off` once the installer exists, so the installer's re-verify fails.
  - **Notice:** `installer: failed reverify` at 16:43:47. At 16:43:48.186 usernoted logged `Presenting <NotificationRecord app:"com.apple.ScriptEditor2" ... alertStyle:"banner"> as banner`. Notification Center then listed `Script Editor, Portty, Portty couldn't install version 1.8.1.` (axdump and screenshot). There was no `failure notice not posted` line. The Finder reveal of the DMG in `Downloads` is unchanged.
  - **Caveat:** the VM already had a Focus (Do Not Disturb) on, so NotificationCenter muted the on-screen banner (`muted by DND suppression: delay`) and put it straight into the history. The Focus was left as it was.
  - **Re-open:** opening the DMG's app again (T0 16:45:47.8) showed the alert `Portty couldn't install version 1.8.1.` with one `OK` button by 16:45:50.2. At that point nothing had been staged: 0 staging dirs, 0 run dirs, no installer, no `/Applications/Portty.app`. update.log held `mover: earlier result failed  to 1.8.1 reverify`. After OK the move ran: `installer: updated` at 16:46:22, then the Downloads prompt, answered Allow, then the DMG went to the Trash. The installed copy showed no second alert.
  - **Clock:** no clock step in the row window (timed's last NTP receipt, at 16:40:55, was in step).
  - Evidence: `evidence/QT-WR02.log`, `evidence/QT-WR02-before.state`, `evidence/QT-WR02.state`, `evidence/QT-WR02-alert.png`, `evidence/QT-WR02-nc-history.png`, `evidence/QT-WR02-reveal.png`.

## Commits

| Hash | Message |
| --- | --- |
| 27c18fcb | docs(quick-261126-dfn): plan the late install failure notice |
| 3fb7562c | test(quick-261126-dfn): installer posts a notice on a late mover failure |
| 275a226c | feat(quick-261126-dfn): installer posts a notice on a late mover failure |
| 440d7733 | test(quick-261126-dfn): mover mode shows an earlier failed result once |
| 3d9f7be0 | feat(quick-261126-dfn): mover mode shows an earlier failed result once |
| 23d18c0e | docs(quick-261126-dfn): QT-WR02 row pending, VM busy with the debug soak |

## Deviations from Plan

**1. [Rule 2 - Correctness] A keepDaemon result is put back in mover mode**
- **Found during:** Task 2
- **Issue:** Consuming a failed result that has `keepDaemon` would lose the WR-04 guarantee. Example: a mover replace whose rollback relaunch failed, then the user re-opens the DMG and picks Cancel. The installed copy would then start with a normal ensure and could raise the restart question that ends the sessions.
- **Fix:** `showEarlierResult` writes such a result back after showing it. That copy may show the failure a second time, which is less harmful than losing the sessions.
- **Commit:** 3d9f7be0

**2. [Rule 1 - Gap] The mover quit-timeout now reveals and notifies**
- **Found during:** Task 1
- **Issue:** It was a listed late-failure case that showed nothing.
- **Fix:** It now goes through `revealLeft`, so it gets the same check for a running GUI.
- **Commit:** 275a226c

## Not done / known limits

- After a quit-timeout, a first install's staging copy stays, as it did before (the DMG's GUI is still alive). Out of scope.
- The alert in mover mode has a title only, no extra message.
- A notice is not posted when `GUIPids` itself fails, as with the reveal.

## Self-Check: PASSED

Every listed file exists. All 6 commits are ancestors of HEAD.
