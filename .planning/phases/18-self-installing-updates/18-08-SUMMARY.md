---
phase: 18-self-installing-updates
plan: 08
subsystem: updater
tags: [macos, tart-vm, amfi, launchservices, rig, vmpreview, probes]
status: complete

requires:
  - phase: 18-01
    provides: the self-install tracer (Stage, hand-off, installer copy, relaunch) this plan proves on real macOS
provides:
  - "updates_vmpreview.go: VM-only verify-mode runner (adhoc fakes codesign and spctl only, fails closed)"
  - "rig/p18-rig.sh: build, deploy-n, dmg, feed, state ROW, quit-gui, clean (all destructive steps inside the VM)"
  - "18-VM-CHECKS.md rows R1-lite, P1, P7, P2a, P2d, P8 and A9 with rig-written captures under evidence/"
affects: [18-02, 18-03, 18-05, 18-07, 18-09]

actuals:
  tokens: 8000
  tasks: 2
  commits: 2
plan_head_before: 32b69fc3041362c95fa2d7b7e1647c25c55a3110
plan_head_after: 7c512e66d971c746a7d9e3d67d3762ad0305652e

key-decisions:
  - "A GUI for the rig's state capture is a Portty.app executable started with no argument, stricter than the plan's 'first argument is not daemon or install-update': the expect driver's `portty attach` and every CLI call would otherwise count as GUIs and break gui_pid_count=1"
  - "P2d CONTRADICTED is reported, not worked around (RULE 0): the installer-copy design (A1) does not run for Developer ID builds"
  - "The relaunch race found by the repeats is recorded as row A9 and left to 18-03, not patched in 18-01's installer.go from this plan"

requirements-completed: []

duration: one session
completed: 2026-11-25
---

# Phase 18 Plan 08: VM rig, tracer proof and macOS probes Summary

**The tracer works on real macOS with ad-hoc builds (one click, same sessions, gapless ticker), but the probes found two blockers for the shipped design: macOS kills the lone Developer ID installer copy at exec (P2d CONTRADICTED), and the installer's `open` straight after the swap fails in half the runs (A9).**

## Result

| Row | Verdict | One line |
| --- | --- | --- |
| R1-lite | PASS (un-granted) | 1.8.0 to 1.8.1 on one click: one GUI, daemon v1.8.1, same session ids and shell pids, ticker `TICK 0..270` with no gap, DMG gone, no dialog; click to new GUI about 2 s, to daemon v1.8.1 about 3 s, to `installer: updated` 6 s |
| P1 | CONFIRMED | no dialog with the quarantine cleared (and none with it set, via the Stage mount flags) |
| P7 | CONFIRMED | `Identifier=app.portty.Portty`, `TeamIdentifier=S2K7P43927` |
| P2a | OBSERVED | ad-hoc installer copy ran and swapped in 6 of 6 runs, no App Management refusal or notification |
| P2d | CONTRADICTED | identity=found, un-granted: AMFI killed the installer copy at exec (`Code=-420`, `load code signature error 4`) |
| P8 | CONFIRMED | staging into `/Applications/.Portty-update-N/Portty.app` never refused |
| A9 | OBSERVED | plain `open` after the old GUI exits failed with `procNotFound` (-600) in 3 of 6 runs |

## STOP: what the orchestrator must decide before 18-02/18-03 build on this

1. **P2d CONTRADICTED (A1 is false for Developer ID builds).** The installer copy is a lone copy of the executable. The Developer ID signature binds `Info.plist`, so outside its bundle the copy fails validation and AMFI refuses to run it. Ad-hoc builds only pass because they have no Team ID, which is why R1-lite and every ad-hoc run worked. By hand: the lone v1.2.0 executable also dies (exit 137), while a `ditto` copy of the whole bundle in another folder runs. Options (details in the P2d row): (a) run the installer from the staged new bundle, (b) copy the whole bundle next to the job and run its executable, (c) embed `Info.plist` in the Mach-O (not tried). `NSUpdateSecurityPolicy` does not address this kill. Not applied.
2. **A9: relaunch race.** `RunInstaller` calls `Open` once; right after the old GUI's pid is gone, LaunchServices still holds it and `open` fails with `-600 procNotFound`. 3 of 6 ad-hoc runs ended with 1.8.1 installed, no GUI and the old daemon still running. A manual `open` seconds later works. Suggested for 18-03: retry `Open` a few times about a second apart before reporting `open`, and treat a still-failing open as a rollback case.
3. **Still untested with a Team ID:** the swap itself and App Management on a Developer ID swap (P2d never got past exec). The no-dialog evidence for a real release therefore still rests on this unresolved row. R1-lite's un-granted ad-hoc PASS shows no App Management refusal for an ad-hoc swap, nothing more.

## Accomplishments

- Task 1: `updates_vmpreview.go` wraps `updateRunner` (reads `$HOME/p18/verify-mode` every call; exactly `adhoc` fakes only `codesign --verify`, `codesign -dvv` and `spctl`; anything else or no file is the real runner; logs `vmpreview: verify-mode adhoc` once). The rig script, two builds (1.8.0, 1.8.1) and R1-lite.
- Task 2: P1, P7, P2a, P2d, P8 with the real published v1.2.0 DMG (downloaded into a host scratch dir, checksum verified, copied into the VM), plus the A9 finding from repeating the ad-hoc update.
- The rig's `build` guard held: wails.json and frontend/src/wailsjs were restored after every build (verified with `git diff --quiet`).

## Task Commits

1. Task 1: `83400447` feat(18-08) VM rig and R1-lite
2. Task 2: `7c512e66` docs(18-08) probes P1, P7, P2a, P2d, P8 and A9

## Deviations from Plan

**1. [Rule 3 - Blocking] Built the frontend in the worktree**
- The worktree has no `frontend/node_modules` or `frontend/dist`, and `wails build` failed at binding generation (`pattern all:frontend/dist: no matching files found`). Ran `pnpm install --frozen-lockfile` and `pnpm run build` in `frontend/` (both ignored by git). No tracked file changed.

**2. [Rule 1 - Bug in the plan's definition] GUI pid filter**
- The plan defines a GUI as "first argument is not `daemon` or `install-update`". The expect driver's `portty attach` (and any CLI call) would match. The rig counts only Portty.app executables started with no argument. Row R1-lite still reads `gui_pid_count=1`.

**3. [Process] Co-Authored-By line**
- Commits end with `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>` (the attribution this session's harness set), not the Opus line in the dispatch prompt.

**4. [Process] No Go unit test for the verify bypass**
- `vmFakeVerify` is a small branch; its proof is R1-lite (real macOS, the bypass drove the whole flow) and the guard gates (single file, brand gate). A new `_test.go` would need a TESTING.md row, which 18-09 owns.

**5. [Process] Rig guards**
- `grep -c '127.0.0.1:8788' updates_vmpreview.go` is now 4, not 3 (the header comment names the address on purpose, so the gate's `-ge 3` holds).

## Issues Encountered (VM noise, not Portty behaviour)

- A stack of unrelated macOS banners (`App Background Activity: "p22b"/"p22c"`, test binaries from earlier phases) covered the banner's click spot; the first click hit one and opened System Settings > Login Items. Cleared by hand; the recorded run started from a clean window list.
- Running `strings` in the VM raised the `Install Command Line Developer Tools` dialog; killed before the run. Avoid CLT shims (`strings`, `git`, `python3`) in the VM.
- A `Finder | updates` window (opened by P2d's reveal fallback) stays open in the VM; later rows should compare window lists against that baseline.
- The Wails webview renders blank in VM screenshots (known); the GUI was read with `axdump`.

## Host commands run (all of them)

`go vet`/`gofmt` on the repo; `pnpm install --frozen-lockfile` and `pnpm run build` in `frontend/`; `p18-rig.sh build` x3 (`wails build`, `jq`, `cp -R`, `git checkout` restore); `go build ./cmd/fakegh` (scratch dir); `plutil` and `codesign -dvv/--verify` on the scratch builds; `security find-identity -v -p codesigning` (read-only); one `codesign --deep --force --options runtime --timestamp --sign IDENTITY` on the scratch copy of the 1.1.90 build (no prompt); `gh release download v1.2.0` into a scratch dir and `shasum -a 256 -c`; `tar` and `cat` streams over ssh into the VM; `git add/commit` on explicit paths. Nothing was mounted, opened or copied from `~/Downloads` or `/Applications` on the host, and no host tailscaled or 100.x address was contacted. Host builds, downloads and scratch files are under the session scratchpad.

## VM end state

`portty-test` left running (never stopped), `~/p18` removed, fakegh stopped (nothing on 8788), no image attached except the system PKI trust-store asset, `/Applications/Portty.app` is an ad-hoc rig build (1.8.1), launched. No App Management grant was made at any point.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register. `update-result.json` in the VM config dir holds the cache DMG path by design (0600); no captured file does.

## Self-Check: PASSED

- Files found: updates_vmpreview.go, .planning/phases/18-self-installing-updates/rig/p18-rig.sh, 18-VM-CHECKS.md, evidence/R1-lite.state, R1-lite-before.state, P2a.state, P2d.state, P2d-before.state, P2d-amfi.log, A9-open-procNotFound.log
- Commits 83400447 and 7c512e66 are ancestors of HEAD
- `bash tests/check-brand.sh` clean; `go vet -tags vmpreview .` clean; wails.json and frontend/src/wailsjs unchanged
