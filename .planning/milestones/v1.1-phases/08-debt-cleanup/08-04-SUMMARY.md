---
phase: 08-debt-cleanup
plan: 04
subsystem: daemon-lifecycle, vm-verification, phase-7-bookkeeping
status: complete
requirements-completed: [DEBT-06, DEBT-07]
tags: [DEBT-06, DEBT-07, launch-blip, ensuredaemon, cold-launch, tart-vm, security-audit]
requires: []
provides:
  - "08-VM-CHECKS.md: cold-launch rows L-01..L-05, at-login rows B-01..B-08, X-01, fix re-runs L-06..L-09, DEBT-06 outcome"
  - "Launch-blip root cause fixed: EnsureDaemon waits 15 s (was 3 s) for a spawned daemon"
  - "requirements-completed in 07-01..07-05 SUMMARY frontmatter"
  - "07-SECURITY.md: 20 threats, threats_open: 0, audited at 2e27e73c"
affects: [internal/daemon/process.go, internal/daemon/process_test.go, .planning/phases/07-hardening]
tech-stack:
  added: []
  patterns: ["VM-only wrapper app in loginwindow's relaunch list to capture stderr of an at-login launch"]
key-files:
  created:
    - .planning/phases/08-debt-cleanup/08-VM-CHECKS.md
    - .planning/phases/07-hardening/07-SECURITY.md
  modified:
    - internal/daemon/process.go
    - internal/daemon/process_test.go
    - .planning/phases/07-hardening/07-01-SUMMARY.md
    - .planning/phases/07-hardening/07-02-SUMMARY.md
    - .planning/phases/07-hardening/07-03-SUMMARY.md
    - .planning/phases/07-hardening/07-04-SUMMARY.md
    - .planning/phases/07-hardening/07-05-SUMMARY.md
decisions:
  - "DEBT-06 root cause: EnsureDaemon's 3 s readiness wait is shorter than a daemon's start at login (10 s measured), so startup fails, the launch grace does not apply and the poller respawns a duplicate daemon; fixed by a 15 s wait, grace kept"
  - "07-SECURITY: the four plan-time low-severity accepts (T-07-05/06/09/12) are logged as accepted at plan time, not newly accepted by the audit"
metrics:
  duration: "~58 min"
  completed: 2026-11-19
estimate:
  tokens: 90000
  tasks: 3
actuals:
  tokens: 8700
  tasks: 3
  commits: 6
plan_head_before: 292daeea39c089bbdb9938f4576138403c47cd61
plan_head_after: 0458e37d
---

# Phase 8 Plan 04: Launch blip captured and fixed at its root, Phase 7 bookkeeping and security audit Summary

The HARD-04 launch blip was reproduced and its error text captured. It appears when macOS reopens Portty at login. At that point the spawned daemon needs up to 10 s to bind, EnsureDaemon gave it only 3 s, and the failed startup skipped the launch grace, so the poller spawned a second daemon. EnsureDaemon now waits 15 s. A seam test failed before the fix and passes after, and in the VM a login launch with a 4 s bind no longer respawns. Phase 7's SUMMARYs now carry `requirements-completed`. 07-SECURITY.md verifies all 20 Phase 7 threats closed against 2e27e73c (`threats_open: 0`).

## Tasks

| Task | Name | Commits |
|------|------|---------|
| 1 | Cold launches after VM reboots, rows L-01..L-05 plus the at-login reproduction (DEBT-06) | 53d8799d |
| 2 | Root-cause fix with a seam test, rebuilt app, VM re-run L-06..L-09 (DEBT-06) | 27c90bb1 (refactor), 498d1c77 (test, RED), c27b69d3 (fix, GREEN), 8fb58fb2 (docs) |
| 3 | requirements-completed in 07-01..05 SUMMARYs, 07-SECURITY.md (DEBT-07) | 0458e37d |

## Cold-launch results (home VM portty-test-2, 08-VM-CHECKS.md)

Plan rows (first `open --stderr` launch after each reboot, deployed build bcb837dd):

| Row | Result | Condition | Key evidence |
|-----|--------|-----------|--------------|
| L-01 | PASS | no daemon (GUI spawns it) | daemon 873, ppid = GUI 867; capture 0 lines. macOS had reopened Portty at login first; that instance was killed and noted (B-01) |
| L-02 | PASS | CLI-started daemon aged 3:17 | daemon 632 throughout; capture 0 lines; log not rotated |
| L-03 | PASS | no daemon, launched 39 s after boot | daemon 610, ppid = GUI 603; capture 0 lines |
| L-04 | PASS | CLI-started daemon aged 8:14 | daemon 635 throughout; capture 0 lines |
| L-05 | PASS | no daemon, launched 28 s after boot | daemon 556, ppid = GUI 539; capture is only AppKit's hotkey line |

At-login rows (macOS reopens Portty, so its stderr is /dev/null):

- B-01, B-02 and B-07: FAIL. A duplicate daemon (`daemon: already running (lock: …)` in daemon.log.1) and 354 or 191 bytes written to the GUI's /dev/null stderr.
- B-07 measured 10 s from daemon spawn to socket bind.
- B-08 used a VM-only wrapper app in loginwindow's relaunch list, which gives the launch the reopen's timing and a stderr file. It captured, verbatim:
  `portty: daemon unreachable (do request: Get "http://daemon/health": dial unix /Users/admin/Library/Application Support/portty/daemon.sock: connect: connection refused), respawn attempt 1: ok`
- B-03..B-06 (LaunchAgent and wrapper variants) and X-01 (CPU-load control) PASS.

Fix re-runs (fixed build c27b69d3):

| Row | Result | Key evidence |
|-----|--------|--------------|
| L-06 | PASS | macOS reopen: GUI stderr 0 bytes, 0 `already running`, bind in 0 s |
| L-07 | PASS | macOS reopen: 163 bytes (the AppKit line only), 0 `already running` |
| L-08 | PASS | wrapper: capture holds only the AppKit line, `respawn=0 blip=0` |
| L-09 | PASS | macOS reopen with a **4 s bind** (the slow path): 163 bytes, one daemon, no `already running` |

## DEBT-06 outcome

**Root cause found and fixed.**

- `EnsureDaemon` (internal/daemon/process.go) gave a spawned daemon a fixed 3 s to answer `/health` and `/relay-port`.
- At login the daemon takes longer. Startup's EnsureDaemon then failed: `daemonErr` was set, `daemon:error` emitted, `launchedAt` left zero.
- A failed startup gets no launch grace (HARD-04 by design). So the poller's immediate first check dialled the previous run's stale socket (`connection refused`) and spent respawn attempt 1.
- That attempt spawned a duplicate daemon, which the WR-05 lock turned away with `already running`.
- Fix: `daemonReadyWait = 15 * time.Second`.
- Unchanged: `respawnLaunchGrace` (10 s) and both app.go diagnostic lines (guard greps: 3 and 1).
- On the real Mac, stderr reaches a file only with `open --stderr <file> /Applications/Portty.app`. A reopened or Dock-launched Portty writes stderr to /dev/null. The fallback trace is `already running` in daemon.log(.1).

## DEBT-07 outcome

- `requirements-completed` was added as one frontmatter line to each of 07-01..07-05. `git diff --numstat 2e27e73c` shows `1 0` for each file. summary-extract returns:
  - 07-01: HARD-01,HARD-03,HARD-07
  - 07-02: HARD-05
  - 07-03: HARD-04,HARD-06,HARD-08
  - 07-04: HARD-02
  - 07-05: HARD-01..HARD-08
- Security audit (self-audit following gsd-security-auditor.md; no Agent tool in this run). ASVS L1, block on high. Every file was read from a `git archive 2e27e73c` extract.
  - 20 threats (T-07-01..T-07-19, T-07-SC).
  - 16 mitigations verified in code and tests.
  - 4 low-severity risks were accepted in the plans' own threat models (T-07-05, 06, 09, 12) and logged as such.
  - `threats_open: 0`, status verified. No high threat open. The CSP has no diff across Phase 7, and the tailnet/webserver/share diffs only add the WR-07 test-binary guard.

## Tests added (for 08-05 to copy into TESTING.md)

- `internal/daemon/process_test.go` (edited): `TestWaitForDaemon_SlowStart`. A stale socket file refuses connections, then a fake daemon binds after 4 s; `waitForDaemon(client, daemonReadyWait)` must succeed. Covers DEBT-06.

## Verification

- `go build ./...` and `go vet . ./internal/daemon/`: clean.
- RED: `TestWaitForDaemon_SlowStart` failed with `EnsureDaemon: daemon did not become ready within 3s`.
- GREEN, run once: `go test -race -short -count=1 . ./internal/daemon/` gave `ok github.com/scottkw/portty 8.222s`, `ok …/internal/daemon 26.624s`.
- Task 1 verify (L-01..L-05 rows, outcome section, no `cap=` token) passed and was re-run as the tracer gate before Task 2. Task 2 verify (guard greps, root cause text, L-06..L-08 PASS, wailsjs clean) and Task 3 verify (summary-extract, `threats_open: 0`, `2e27e73c`, 24 `T-07-` mentions) passed.
- Build at c27b69d3: `codesign -dv` reports `Signature=adhoc` and `TeamIdentifier=not set`. sha256 `14db8623…7e1bfc`. frontend/src/wailsjs restored and clean.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] macOS reopened Portty at login, so the plan's L launches could not reach the blip**
- **Found during:** Task 1, boot 1
- **Issue:** loginwindow's relaunch list still held Portty, because the GUI was killed rather than quit. The reopened instance has stderr on /dev/null, yet it showed the blip's trace on every boot.
- **Fix:** kept the plan's five L rows as specified, removing Portty from the relaunch list before each reboot so `open --stderr` was the first launch. Added rows B-01..B-08 and X-01 to chase the at-login launch: LaunchAgents first, then a VM-only wrapper app in the relaunch list, which captured the line (B-08). Every VM change was backed up and restored (`cmp` clean). The plan named re-runs L-06..L-08; L-09 is extra.
- **Files modified:** .planning/phases/08-debt-cleanup/08-VM-CHECKS.md (VM-side changes only otherwise)

**2. [Rule 3 - Blocking] Wails build in a fresh worktree needs frontend/dist**
- Same as 07-05. A placeholder `frontend/dist/index.html` was created (gitignored) and the frontend build replaced it. Nothing was committed.

**3. Process notes**
- The worktree started at c2f4255d. Fast-forwarded with `git merge --ff-only gsd/phase-07-hardening` to 292daeea, as instructed.
- The extraction of `waitForDaemon` was committed as its own `refactor(08-04)` before the RED test, so the RED fails on the timeout rather than on a missing function.
- The VM's settings.json stayed WebGL-off through Task 2's re-runs and was restored afterwards, not at the end of Task 1.
- The home VM now runs the fixed build. The office VM still runs bcb837dd; both are version `dev`.

## Known Stubs

None.

## Threat Flags

None. The change only lengthens an existing local wait; there is no new endpoint or input. 08-VM-CHECKS.md was checked and holds no `cap=` value, share URL or auth key.

## Self-Check: PASSED

- FOUND: .planning/phases/08-debt-cleanup/08-VM-CHECKS.md, .planning/phases/07-hardening/07-SECURITY.md, internal/daemon/process.go, internal/daemon/process_test.go
- FOUND commits: 53d8799d, 27c90bb1, 498d1c77, c27b69d3, 8fb58fb2, 0458e37d (all ancestors of HEAD)
