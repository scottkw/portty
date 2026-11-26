---
phase: quick-261126-dfn
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/updater/install.go
  - internal/updater/installer.go
  - internal/updater/installer_test.go
  - internal/updater/selfinstall_test.go
  - app_install.go
  - app_mover.go
  - app_mover_test.go
  - TESTING.md
  - .planning/phases/18-self-installing-updates/18-VM-CHECKS.md
autonomous: true
requirements:
  - UPD-09
---

<objective>
Close the Phase 18 observation 2 / RC-WR02 residual, as the user decided on 2026-11-26. A mover
first install that fails late in the installer (reverify, swap, the DMG GUI's quit-timeout, or
open/health with no GUI left) now tells the user why:

1. The installer posts a native notification, "Portty couldn't install version X.", when a mover
   job ends failed and no GUI of the target runs to read the result. The Finder reveal stays.
2. Mover mode reads an earlier failed or rolled-back result before it stages or writes a job. It
   shows the result once in an alert and logs it without paths, then runs the normal flow.
</objective>

<context>
Sources: 18-VM-CHECKS.md row RC-WR02, 18-VERIFICATION.md human item 5, 18-UAT.md, 18-REVIEW-FIX.md WR-02.

**How the installer posts.** The installer runs straight from the run-*/Portty.app copy. It is not
started by LaunchServices and has no main run loop. A probe in VM portty-test (macOS 27.2) copied
the installer's shape: a bundle with its own identifier, started directly in the user's GUI session.
- Posting through `UNUserNotificationCenter`, the path `sendNotification` uses:
  - Bundle not registered with LaunchServices: refused at once with "Notifications are not allowed
    for this application".
  - Bundle registered, but its identifier never authorized (a true first install): the system shows
    only the "allow notifications" prompt, never the message. The prompt is removed when the
    process exits.
  - On top of that, `sendNotification` dispatches to the main queue, and the installer never drains it.
- `osascript display notification` from the same context was delivered and shown as a banner at
  once, with no prompt.

So the installer posts with `/usr/bin/osascript`, through `env.Run`. The body is passed as an
argv item, never pasted into the script. The cost: the banner shows the Script Editor icon, with
the title "Portty".
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: installer notice on a late mover failure</name>
  <files>internal/updater/install.go, internal/updater/installer.go, internal/updater/installer_test.go, internal/updater/selfinstall_test.go</files>
  <behavior>
    - Each row of TestInstaller_MoverFailureLeavesSomethingToActOn makes exactly one osascript call. The call carries "Portty couldn't install version 9.9.9." as its argv item, with no path in it. The one reveal is still there.
    - A new row: a first install whose DMG GUI never quits (quit-timeout) also reveals the DMG and posts the notice.
    - TestInstaller_MoveFirstInstallUnhealthy (the new GUI is running) posts nothing.
    - TestSafeRunner_RefusesUnderTest: SafeRunner refuses /usr/bin/osascript in a test binary.
  </behavior>
  <action>
    - Add `ResultText(r) (string, bool)`, the user-facing sentence for a failed or rolled-back result, and `Notify(ctx, run, body)` (osascript with argv) next to Reveal.
    - Add osascript to SafeRunner's refused list.
    - revealLeft posts `ResultText(r)` after the reveal; a failed post is only logged.
    - The mover's quit-timeout path calls revealLeft.
  </action>
  <verify>GOTOOLCHAIN=go1.26.11 go test -race -count=1 ./internal/updater/</verify>
  <done>The tests above fail before the change and pass after it. No test binary runs osascript.</done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: mover mode shows an earlier failed result once</name>
  <files>app_mover.go, app_install.go, app_mover_test.go, TESTING.md</files>
  <behavior>
    - TestRunMover_ShowsEarlierResultOnce:
      - A failed result for this same version (9.9.9, no old version) is shown as an alert titled "Portty couldn't install version 9.9.9.". The alert comes before any copy (ditto) or job.
      - The result file is consumed, and the move job is still started.
      - update.log holds "mover: earlier result failed" and no "/".
    - Rolled-back row: the sentence is shown. The result has keepDaemon, so it is put back for the installed copy's next start (WR-04).
    - An updated result shows nothing.
  </behavior>
  <action>
    - consumeInstallResult uses updater.ResultText.
    - runMover first calls showEarlierResult: TakeResult, log outcome, versions and reason (never the dmg path), put back a keepDaemon result, then show the alert if ResultText has a sentence and the new version parses.
    - Add the test name to TESTING.md's UPD-08 app_mover_test.go row.
  </action>
  <verify>GOTOOLCHAIN=go1.26.11 go test -race -count=1 . -run 'Mover|ConsumeInstallResult'</verify>
  <done>The tests above fail before the change and pass after it. A test binary never shows a real alert, because messageDialogFunc is the seam.</done>
</task>

<task type="auto">
  <name>Task 3: QT-WR02 row and gates</name>
  <files>.planning/phases/18-self-installing-updates/18-VM-CHECKS.md</files>
  <action>
    - Check portty-test for the debug soak (`portty list`, ps). The row needs a first install, so no
      Portty may be in /Applications. If the soak's Portty is installed or running, write the row
      as PENDING and do not touch it. Otherwise repeat RC-WR02's method on a rig build of this
      branch: watch the NotificationCenter log and axdump for the notice, then re-open the DMG's
      app and look for the earlier-failure alert before the move.
    - Run every gate from the task brief.
  </action>
  <verify>gofmt, go vet (both), golangci-lint --new-from-rev=c714c8ef, go test -race ./internal/updater/ ., check-brand.sh, check-traceability-paths.sh</verify>
  <done>QT-WR02 row present (verdict or PENDING with reason); every gate is clean.</done>
</task>

</tasks>
