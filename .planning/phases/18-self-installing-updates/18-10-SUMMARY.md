---
phase: 18-self-installing-updates
plan: 10
subsystem: updater
tags: [macos, amfi, codesign, developer-id, installer, tart-vm, rig]
status: complete

requires:
  - phase: 18-01
    provides: startInstaller, spawnInstaller, runInstallUpdate (the hand-off this plan changes)
  - phase: 18-08
    provides: the VM rig, row P2d (the lone-copy kill this plan replaces)
provides:
  - "app_install.go: copyBundle (ditto, VerifyApp on the copy, then ClearQuarantine on the copy), runningBundleFunc/copyBundleFunc seams, installerStartWait (3 s, measured), installerCopyTimeout, updateLogf"
  - "startInstaller spawns RUN/Portty.app/Contents/MacOS/Portty install-update RUN/job.json and logs `update: installer started (copy Nms, start Nms)` or `update: installer not started: STEP`"
  - "rig/p18-rig.sh: state prints run_dirs and lists run folders; new `sign APP_DIR` (scratch only)"
  - "18-VM-CHECKS.md row P2d-bundle CONFIRMED (identity=found, un-granted) with evidence/P2d-bundle.state, P2d-bundle-before.state, P2d-bundle-amfi.log"
affects: [18-03, 18-05, 18-06, 18-09]

tech-stack:
  added: []
  patterns:
    - "Installer runs from a ditto copy of the whole running bundle, checked before its quarantine flag is removed"
    - "GUI-side update outcomes go to update.log (step names and durations only, no paths)"

key-files:
  created:
    - .planning/phases/18-self-installing-updates/evidence/P2d-bundle.state
    - .planning/phases/18-self-installing-updates/evidence/P2d-bundle-before.state
    - .planning/phases/18-self-installing-updates/evidence/P2d-bundle-amfi.log
  modified:
    - app_install.go
    - cmd_install_update_test.go
    - .planning/phases/18-self-installing-updates/rig/p18-rig.sh
    - .planning/phases/18-self-installing-updates/18-VM-CHECKS.md

actuals:
  tokens: 8000
  tasks: 2
  commits: 3
plan_head_before: 0e13d3f2ff7d0bf39c7bdb0f73c565032c7a0abf
plan_head_after: d5e043403d0a9fafea573d122023546d97c687f8

key-decisions:
  - "installerStartWait stays 3 s: measured spawn-to-marker 417 ms on a Developer ID build (under half of the deadline, 7x headroom)"
  - "The failure error from startInstaller wraps the step name; the Service never shows it to the user (it logs a fixed line and uses installerStartFailText)"
  - "The no-marker test proves the kill by checking the 30 s sleep pid is gone (kill 0 gives ESRCH), not via ProcessState, because startInstaller owns the only Wait"

requirements-completed: [UPD-05, UPD-07, UPD-09]

duration: about 25 min
completed: 2026-11-25
---

# Phase 18 Plan 10: Whole-bundle installer copy and signed P2d-bundle Summary

**The installer now runs from a ditto copy of the whole running Portty.app. The copy is checked with VerifyApp before its quarantine flag is removed. On a Developer ID build with no App Management grant, one click swapped in the real v1.2.0 release: no AMFI kill, no dialog, and a spawn-to-marker time of 417 ms (P2d-bundle CONFIRMED).**

## Result

| Row | Verdict | One line |
| --- | --- | --- |
| P2d-bundle | CONFIRMED (identity=found, un-granted) | Installer ran from `run-*/Portty.app/Contents/MacOS/Portty`, swapped in the real v1.2.0, relaunched it by itself and removed its run folder. No AMFI kill line, no App Management request from Portty, no dialog. Capture: installed 1.2.0, 1 GUI, daemon v1.2.0, run_dirs=0 |

Measurements (attempt 1 of 1; A9 was not hit):
- Click to started marker: 3.93 s (download, real verify and staging all happen before the spawn)
- Copy: 163 ms. Spawn to marker: 417 ms (from the GUI's `update.log` line)
- Click to old GUI gone: 4.1 s. Click to new GUI: 4.95 s. Click to daemon v1.2.0: at most 8 s. Click to `installer: updated`: 10.0 s
- Deadline: 417 ms is under 1.5 s, so the 3 s deadline stays. Its doc comment now names the measurement.

Key evidence: the kernel logged `(Sandbox) registered kTCCServiceSystemPolicyAppBundles exception for Portty.app, team-id=S2K7P43927` when the installer copy started (23:46:03.808 UTC). This is the same-Team App Management exception that 18-08 saw for an app in /Applications, now attached to the copy. The AMFI lines in the window are `[non-fatal] unable to accelerate context` only. The one tccd AppBundles line was a preflight for `com.apple.dock`.

## Task Commits

1. Task 1 RED: `93fe387f` test(18-10): add failing tests for whole-bundle installer copy
2. Task 1 GREEN: `d132e572` feat(18-10): start the installer from a whole checked copy of the running Portty.app
3. Task 2: `d5e04340` docs(18-10): P2d-bundle CONFIRMED on a Developer ID build, un-granted

## TDD Gate Compliance

- RED: `93fe387f` adds the three tests. They failed to compile: `undefined: runningBundleFunc` from `go vet .`.
- GREEN: `d132e572`. All three tests pass, and so do the existing app_install.go tests.
- REFACTOR: none needed.

## Verification

- Task 1 verify passed in full:
  - gofmt clean
  - `go vet` passed, both default and `-tags vmpreview`
  - golangci-lint `--new-from-rev=10810056`: 0 issues
  - 3 new tests are listed
  - The grep gates for copyExecutable, portty-installer, installerStartWait and VerifyApp pass
  - selfinstall_test.go is unchanged since 10810056
  - `go test -race -count=1 . ./internal/updater/` passes
  - TestSelfInstall_UpdateHappyPath reports PASS
  - check-brand is clean
- Task 2 verify passed in full:
  - The rig passes `bash -n` and has the run_dirs and cmd_sign changes
  - The CONFIRMED row shows un-granted and identity=found and cites P2d-bundle.state
  - The capture header and all 4 key values are present
  - The AMFI capture is non-empty and has no kill line
  - The row and captures hold no home path, 100.x address or ts.net name
  - The race tests pass again and check-brand is clean
- All Go commands were run with `GOTOOLCHAIN=go1.26.11`.
- No new `_test.go` file was added: the tests went into the existing cmd_install_update_test.go. TESTING.md was not touched (18-09 owns it).

## Deviations from Plan

**1. [Rule 3 - Blocking] Frontend built in the worktree.** `frontend/dist` was missing, which the precondition anticipates. Ran `pnpm install --frozen-lockfile` and `pnpm run build` in `frontend/`. Both outputs are gitignored.

**2. [Rule 1 - Test design] How the no-marker test proves the kill.** The plan asks for the sleep's ProcessState to report a signal. startInstaller calls `proc.Wait()` itself, so the test cannot get a ProcessState. Instead it checks `kill(pid, 0)` returns ESRCH, which means killed and reaped. A 30 s sleep would otherwise still be alive after 200 ms.

**3. [Process] My own capture mistakes, corrected before any result was recorded.**
- My first host-side wait loop grepped `^installer:`, but the installer's lines start with a log timestamp, so the loop never saw the result. The result was already there at 23:46:10; I stopped the stale waiter.
- My first `log show` ran in the VM's zsh, where `log` is a builtin. That capture held only a zsh error, and its "tccd 0" count was meaningless. I recaptured with `/usr/bin/log`. Only the recapture is committed and quoted.

**4. [Process] Plan commit ledger.** The worktree guard refused writes into the `.git/worktrees` folder. `plan_head_before` is the dispatch base `0e13d3f2`, and `commits: 3` is measured with `git rev-list --count 0e13d3f2..HEAD`.

## Host commands run (all of them)

- `pnpm install --frozen-lockfile` and `pnpm run build` in `frontend/`
- gofmt, `go vet`, golangci-lint, `go test`, check-brand
- `p18-rig.sh build 1.1.91 SCRATCH/n-1191`: wails build, jq, cp -R, and the git checkout restore of wails.json and frontend/src/wailsjs (tree clean afterwards)
- plutil and shasum on the scratch build
- `cp -R` to `SCRATCH/n-1191-devid`
- `p18-rig.sh sign SCRATCH/n-1191-devid`: `security find-identity -v -p codesigning` (read-only) and one `codesign --deep --force --verbose --options runtime --timestamp --entitlements build/entitlements.plist --sign HASH`. It printed identity=found and raised no keychain prompt.
- `codesign --verify --deep --strict` and `codesign -dvv` on the signed scratch copy
- `gh release download v1.2.0 --repo scottkw/portty` (DMG and checksums.txt) into a fresh scratch dir, then `shasum -a 256 -c --ignore-missing` (OK)
- The fakegh build, which the rig's `feed` does only if it is missing
- tar and cat streams over ssh into the VM: the app (deploy-n), the DMG under its real name, fakegh, and the poller and window-lister scripts
- git add and commit on explicit paths

Nothing on the host was mounted, opened or copied from `~/Downloads` or `/Applications`. The host's tailscaled and 100.x addresses were never contacted, and there was no host GUI automation. Scratch files are under the session scratchpad (`p18/`), never in the repo.

## VM end state

- `portty-test` was left running and never stopped (portty-test-2 was not touched).
- `/Applications/Portty.app` is the ad-hoc 1.1.91 rig build, launched (1 GUI).
- fakegh is stopped (8788 down) and `~/p18` is removed.
- The only attached image is the system PKI trust-store asset.
- No App Management grant was made. P2d's Finder `updates` window was closed before the run.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register: T-18-41 to T-18-46 are mitigated as planned. update.log lines carry step names and durations only.

## Self-Check: PASSED

- Files found: app_install.go, cmd_install_update_test.go, rig/p18-rig.sh, 18-VM-CHECKS.md, evidence/P2d-bundle.state, evidence/P2d-bundle-before.state, evidence/P2d-bundle-amfi.log
- Commits 93fe387f, d132e572 and d5e04340 are ancestors of HEAD
