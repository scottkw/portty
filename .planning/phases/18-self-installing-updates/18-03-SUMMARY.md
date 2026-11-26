---
phase: 18-self-installing-updates
plan: 03
subsystem: updater
tags: [macos, installer, rollback, launchservices, daemon-handoff, tart-vm, rig]
status: complete

requires:
  - phase: 18-01
    provides: RunInstaller, Job/Result files, TakeResult, the installer env seams
  - phase: 18-08
    provides: the VM rig, R1-lite timings, row A9 (open -600 right after the old GUI exits)
  - phase: 18-10
    provides: the whole-bundle installer copy (P2d-bundle CONFIRMED), updateLogf
provides:
  - "installer.go: rollback (reasons health, gui-exited, open; swap-back), GUI-alive health, guarded quit-timeout SIGKILL, relaunch() with OpenAttempts/OpenRetryWait/OpenTimeout, ParseGUIPids"
  - "cmd_install_update.go: realGUIPids (ps) and Signal: syscall.Kill in the real env"
  - "outcome.go: Service.ShowResult, rolledBackText/failedText, skip-version record (skipVersion, skippedVersion) for 18-02"
  - "app_install.go/app.go: consumeInstallResult before the daemon ensure, keepDaemonOnce for exactly one ensure, update-result notification, vmFaultHook stages"
  - "updates_vmpreview.go: VM-only fault hooks from ~/p18/fault"
  - "18-VM-CHECKS.md rows R2-A, R2-B, R2-C, R2-D and A9-fix PASS"
affects: [18-02, 18-04, 18-05, 18-06, 18-07, 18-09]

tech-stack:
  added: []
  patterns:
    - "Every relaunch goes through one retrying helper; the result file is on disk before any relaunch of the old app"
    - "Signals only to pids ps proves are the installed bundle's GUI (no args or a launch-services arg, the same rule as main.launchMode)"

key-files:
  created:
    - internal/updater/outcome.go
    - internal/updater/outcome_test.go
    - internal/updater/installer_test.go
    - app_install_test.go
    - .planning/phases/18-self-installing-updates/evidence/R2-A.state (+ R2-A-before.state, R2-A-timeline.log; same set for R2-B, R2-C, R2-D)
    - .planning/phases/18-self-installing-updates/evidence/A9-fix.state
    - .planning/phases/18-self-installing-updates/evidence/A9-fix-run5.log
  modified:
    - internal/updater/installer.go
    - internal/updater/selfinstall_test.go
    - cmd_install_update.go
    - app.go
    - app_install.go
    - updates_vmpreview.go
    - .planning/phases/18-self-installing-updates/18-VM-CHECKS.md

key-decisions:
  - "ParseGUIPids keeps only args == exe or exe plus a launch-services argument (-psn_, -NS, -Apple), mirroring main.launchMode, so CLI clients (portty attach from the bundle) are never signalled"
  - "The result file is written before every relaunch of the old app (reverify, swap, rollback, swap-back); the forward open of N+1 sees no result file, because N+1 would consume it"
  - "relaunch() ignores the overall ctx deadline (each try still has OpenTimeout): skipping a relaunch would leave no GUI"
  - "A refused swap (R2-C) is not a rollback: its staged N+1 copy is left for 18-05's sweepUpdateLeftovers"
  - "skippedVersion with an empty Dir is an error, not an empty answer (no cache folder means nothing can be known)"

requirements-completed: [UPD-07, UPD-05]

estimate:
  tokens: 98000
actuals:
  tokens: 24000
  tasks: 3
  commits: 6
plan_head_before: 3bcd457084d8df0ee2a57cf9f15aeba0ff6a4e65
plan_head_after: 0c771324928c4dcba8a1794b339ae9eab990ad1b

duration: about 35 min of work, plus a rate-limit pause before the rows were written up
completed: 2026-11-26
---

# Phase 18 Plan 03: Rollback, relaunch retry and the user-facing result Summary

**A broken update now undoes itself. When N+1 never becomes healthy, its window dies, or it never opens, the installer stops only N+1's GUI, swaps N back, removes the failed copy and relaunches N. N keeps whichever daemon holds the sessions, asks nothing, and shows the rolled-back banner plus a notification. Every relaunch retries the `open` that LaunchServices refuses right after the old GUI exits. All five VM rows pass.**

## Result

| Row | Verdict | One line |
| --- | --- | --- |
| R2-A | PASS | `exit-before-daemon v1.8.1`: rolled back (gui-exited) 3.62 s after the click (W=99.6 s); same daemon pid at v1.8.0, same shells, one GUI; banner text shown, notification delivered, DMG kept, skip-version 1.8.1 |
| R2-B | PASS | `exit-after-daemon v1.8.1`: the takeover committed to the 1.8.1 daemon; rolled back (gui-exited) 3.40 s after the click; 1.8.0 runs against the 1.8.1 daemon with the same shells; 0 DEBT-08 dialog matches in 25 axdumps |
| R2-C | PASS | `uchg` on the bundle: `failed swap`; the installer relaunched 1.8.0 at +1.47 s; daemon and sessions untouched; failed-text banner |
| R2-D | PASS | `hang-before-quit v1.8.0`: the old GUI was SIGKILLed after QuitWait (+21.76 s) while the daemon was untouched; then a normal update to 1.8.1 with the sessions moved |
| A9-fix | PASS | 6 of 6 plain updates relaunched by the installer itself; run 5 hit `-600 procNotFound` for the dead old pid, and the retry 1 s later opened 1.8.1 |

W for R2-A and R2-B = 2 s (R1-lite click to new GUI) + 0.163 s + 0.417 s (P2d-bundle copy and spawn-to-marker) + 60 s + 5 s + 2 s + 30 s = 99.6 s. All rows ran on ad-hoc rig builds (verify-mode adhoc, un-granted). The installer ran from 18-10's whole-bundle copy in every row, and `run_dirs=0` afterwards.

**Finding for 18-02 (expected, not a failure):** after a rollback or a failed install, the banner shows the result for about 10 s. Then the update poller's first check offers the same version again (`Portty 1.8.1 is available.` with `Update`). This was seen in R2-A, R2-B and R2-C. 18-02's Check must read `skippedVersion()` (the record is written: `skip-version` = `1.8.1` in the cache).

## Task Commits

1. Task 1 RED: `dfb969c5` test(18-03): add failing installer rollback, relaunch-retry and ParseGUIPids tests
2. Task 1 GREEN: `50640539` feat(18-03): installer rollback, GUI-alive health, guarded quit-timeout kill and relaunch retry
3. Task 2 RED: `a2170a04` test(18-03): add failing ShowResult, skip-record and consumeInstallResult tests
4. Task 2 GREEN: `483176f2` feat(18-03): relaunched N keeps the session daemon and tells the user the update did not take
5. Task 3 code: `b968a6ef` feat(18-03): VM-only fault hooks for the rollback rows
6. Task 3 rows: `0c771324` docs(18-03): VM rows R2-A..R2-D and A9-fix PASS on ad-hoc rig builds

## TDD Gate Compliance

- Task 1: RED `dfb969c5` failed to compile (`undefined: OpenRetryWait` from `go vet`). GREEN `50640539`: all 10 new installer tests pass, and so does TestSelfInstall_UpdateHappyPath.
- Task 2: RED `a2170a04` failed to compile (`ShowResult undefined`, `consumeInstallResult undefined`). GREEN `483176f2`: 5 updater tests and 3 app tests pass.
- REFACTOR: none needed.

## Verification

All Go commands were run with `GOTOOLCHAIN=go1.26.11`.

- Task 1 verify block passed in full:
  - gofmt and vet are clean
  - 9 of 9 listed tests exist
  - `OpenAttempts` is used outside its declaration
  - `go test -race` passes for `./internal/updater/` and for `. -run TestInstallUpdate`
  - The acceptance greps pass: OutcomeRolledBack 4, `GUIPids:` 1, `func ParseGUIPids` 1
- Task 2 verify block passed in full:
  - gofmt and vet are clean
  - golangci-lint `--new-from-rev=c714c8ef`: 0 issues
  - 5 TestShowResult/TestSkippedVersion tests and 3 TestConsumeInstallResult tests are listed
  - The race tests pass, and so do the existing ensure, restart-consent, respawn and startup tests
- Task 3 verify block passed in full:
  - `go vet -tags vmpreview` is clean
  - `p18/fault` is present in updates_vmpreview.go
  - 4 R2 PASS rows; the R2-A and R2-B rows carry W and click-to-rollback
  - The A9-fix row cites its capture, and the capture has its header and installed_version=1.8.1, gui_pid_count=1, run_dirs=0
  - No 100.x address in the rows or captures; check-brand is clean
- Plan verification: `go test -race -count=1 ./internal/updater/ .` passes.
- The new rows and evidence hold no home path or tailnet DNS name.
- Release-path check: only updates_vmpreview.go (vmpreview tag) reads the fault file. The other hit for `"fault")` is an unrelated vmwhois test mode.

## Deviations from Plan

**1. [Rule 2 - Security, T-18-14] ParseGUIPids is stricter than the action's wording.** The action says to keep exe plus any first word except `daemon` and `install-update`. That would also list `Portty attach X` run from the bundle; the rig's expect driver runs exactly that, and so can a user. A rollback would then SIGTERM CLI clients. The parser now keeps only no-argument launches or a launch-services first argument (the exact rule `main.launchMode` uses to start the GUI). This matches the behavior spec ("the executable plus launch-services arguments"). TestParseGUIPids also asserts that an `attach` line is dropped.

**2. [Rule 1 - Test changed by design] TestSelfInstall_HealthMissWritesFailed now expects `rolled-back`.** 18-03 turns a health miss into a rollback, as the old code comment `18-03 turns this into a rollback` said it would. Only the assertion line was edited (the plan's read_first says not to edit selfinstall_test.go; siRelease and TestSelfInstall_UpdateHappyPath are untouched and pass).

**3. [Interpretation] "Result written before each relaunching Open"** means every Open that relaunches the old app (reverify, swap, rollback, swap-back). The forward Open of N+1 after the swap writes nothing first: N+1's startup would consume a provisional result. TestInstaller_ResultWrittenBeforeOpen asserts both halves.

**4. [Rule 2] Validation of versions and records.**
- consumeInstallResult sets keepDaemon first, then ignores a rolled-back or failed result whose versions don't parse (logs `update: result versions malformed`), so no untrusted text reaches the banner or notification.
- skipVersion only writes a valid version.
- skippedVersion returns an error for an empty Dir.

**5. [Process] Pollers instead of the 2 s `state` loop.** The VM-side process poller sampled the same fields every 0.4 s, with a 1 s window lister and a ~1 s axdump grep. `state ROW` (and `ROW-before`) were still captured for every row. The transitions are committed as `evidence/ROW-timeline.log`.

**6. [Process] My own capture mistakes.**
- R2-A attempt 1: the row script started the pollers with an empty row name, and the next start deleted their files. That attempt had the same outcome but is not the recorded run.
- My closing `portty list` (the 1.8.0 CLI without `--keep-daemon`) asked the 1.8.1 daemon for a takeover in R2-B. The downgrade wall refused it (daemon.log 00:17:48), and the sessions were unaffected. Both are noted in the rows.

**7. [Process] Frontend built in the worktree** (`pnpm install --frozen-lockfile`, `pnpm run build`; both gitignored). TestNoFakeWhoIsInRelease needs `frontend/dist`.

**8. [Process] Plan commit ledger.** It was written to the per-worktree git dir this time. `commits: 6` is measured with `git rev-list --count 3bcd4570..HEAD` before this SUMMARY's own commit.

## Assumptions kept (from the plan)

- O2: the deadline still rolls back with the DEBT-08 question pending. An N+1 that passes health and fails later is not rolled back.
- keepDaemon covers only the first ensure after a rollback.

## Host commands run

- gofmt, go vet (default and `-tags vmpreview`), golangci-lint, go test
- `pnpm install` and `pnpm run build` in frontend/
- `p18-rig.sh build 1.8.0` and `build 1.8.1` into the scratchpad. The wails.json and wailsjs restores left the tree clean.
- `plutil` and `--version` on the scratch builds
- `p18-rig.sh` deploy-n, dmg, feed, state and clean (every effect inside the VM over vm.sh)
- tar and cat streams of the rig app, my helper scripts, vtick.vim and a patched cli.exp into the VM's `~/p18`
- Copying the VM's axdump binary to the scratchpad to run `strings` on it
- git add and commit on explicit paths

Nothing on the host was mounted, opened or copied from `~/Downloads` or `/Applications`. No host tailscaled or 100.x address was contacted, and there was no host GUI automation (clicks and axdump only inside the VM).

## VM end state

- portty-test was never stopped (portty-test-2 was not touched).
- `/Applications/Portty.app` is the ad-hoc 1.8.1 rig build, launched (1 GUI, daemon v1.8.1, no sessions).
- No fault file (`~/p18` was removed by `clean`), no uchg flag (`ls -ldO` shows `-`), and fakegh is stopped.
- A Finder `updates` window from the reveals may still be open.

## Known Stubs

- `Service.skippedVersion` has no production caller yet. 18-02's Check is its planned consumer. Until then the failed version is offered again on the next check (see the finding above). This is intentional and tracked by 18-02's plan.

## Threat Flags

None beyond the plan's register.
- T-18-14: signals go only to ParseGUIPids pids, tested with TestInstaller_NeverSignalsDaemon.
- T-18-15: the result is on disk before relaunch, a failed relaunch is rewritten to failed, and a swap-back failure still opens the target.
- T-18-16: TakeResult checks are unchanged.
- T-18-17: the hook exists only in the vmpreview file.
- T-18-18: banner plus notification.
- T-18-36: accepted.

TESTING.md was not touched; 18-09 owns the Phase 18 test manifest and traceability rows (installer_test.go, outcome_test.go and app_install_test.go are in its list).

## Self-Check: PASSED

- Files found: internal/updater/installer.go, installer_test.go, outcome.go, outcome_test.go, cmd_install_update.go, app.go, app_install.go, app_install_test.go, updates_vmpreview.go, 18-VM-CHECKS.md, evidence/R2-{A,B,C,D}.state, evidence/R2-{A,B,C,D}-before.state, evidence/R2-{A,B,C,D}-timeline.log, evidence/A9-fix.state, evidence/A9-fix-run5.log
- Commits dfb969c5, 50640539, a2170a04, 483176f2, b968a6ef and 0c771324 are ancestors of HEAD
