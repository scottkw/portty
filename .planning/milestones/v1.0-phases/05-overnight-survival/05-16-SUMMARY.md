---
phase: 05-overnight-survival
plan: 16
subsystem: daemon-lifecycle
status: complete
gap_closure: true
tags: [daemon-respawn, tray-poller, attach-reauth, relay-port, tart-vm, shar-07, verf-01]
requires:
  - phase: 05-14
    provides: "Resilience matrix with R6 FAIL (nothing restarts the viewing Mac's daemon); the two-VM rig"
provides:
  - "app.go respawnDaemonIfDown: the 5 s tray poller starts a dead local daemon, serialized with RetryDaemon by daemonMu, backed off 10 s doubling to 2 min, streak reset after a healthy minute, daemon:error once on the third attempt"
  - "cmd_attach.go remoteReauth plus the ensureLocalDaemon seam: `portty attach` starts a dead local daemon before a redial"
  - "App.tsx NAV-01 session poll refreshes the relay port, so new local tabs dial a respawned daemon"
  - "05-VM-MATRIX.md: R6 PASS on build 1e0c33c3 (GUI alone, CLI alone, both) and the R7 regression note; summary 9 passed, 0 failed"
  - "TESTING.md Section 2 note and SHAR-07 Section 4 rows"
affects: [05-15]
tech-stack:
  added: []
  patterns:
    - "In-process daemon respawn from the app and the CLI (no launchd agent); the per-request socket client is never replaced (WR-05)"
    - "Respawn pacing driven by an injected `now` so tests never sleep"
key-files:
  created:
    - app_respawn_test.go
    - .planning/phases/05-overnight-survival/05-16-SUMMARY.md
  modified:
    - app.go
    - cmd_attach.go
    - cmd_attach_test.go
    - frontend/src/App.tsx
    - frontend/src/components/__tests__/App.nav.test.tsx
    - TESTING.md
    - .planning/phases/05-overnight-survival/05-VM-MATRIX.md
key-decisions:
  - "Respawn runs in-process from the GUI tray poller and the attach re-auth. No launchd agent is installed (orchestrator decision)"
  - "The frontend changed after all: the session poll refreshes relayPort, because new local tabs dialed the dead daemon's port (found in the first R6a VM run)"
  - "relayPort is refreshed only once init has set it, and never to 0, so launch shows no empty-state flash and a daemon outage never unmounts local panels"
requirements-completed: []  # SHAR-07, VERF-01 left to the orchestrator (not marked here, per instruction)
duration: 31min
completed: 2026-11-18
estimate:
  tokens: 70000
  tasks: 2
actuals:
  tokens: 10250
  tasks: 2
  commits: 4
plan_head_before: 7ea0cbcabacbfb579dab28bebbcab39b0d0212e6
plan_head_after: e0ad123471eec4eccfd5c58eaa4543636ccd1bb2
---

# Phase 5 Plan 16: Viewing-Mac Daemon Respawn (R6) Summary

**When the viewing Mac's daemon dies, the GUI's 5 s poller or `portty attach` now starts it again in-process. GUI tabs, the CLI and new local tabs recover on their own in 1.4 to 5 s. R6 passes on the two Tart VMs.**

## Result

- **R6: PASS** on build `1e0c33c3`, after `kill -9` of the home daemon:

  | Sub-run | Who respawned | Daemon up | Viewer back (office join) | Replay |
  |---------|---------------|-----------|---------------------------|--------|
  | (a) GUI tab alone | the GUI app (ppid) | +1 s | 4.3 s (first tick about 5 s) | delta 38 B |
  | (a2) GUI alone, stderr captured | the GUI app: `respawn attempt 1: ok` | +3 s | about 5 s | delta |
  | (b) CLI alone | `portty attach` (ppid) | +1 s | 1.4 s | delta 20 B |
  | (c) GUI tab and CLI together | the CLI won; the GUI poller then found the daemon healthy | +1 s | 1.4 s (CLI), 1.6 s (GUI) | delta 20 B / 20 B |

  - In every sub-run: typing landed once, no tick repeated (GUI scrollback pages and ticks.sh), and exactly one daemon process was left.
  - After (a), the sidebar listed the office again, and a new local session ran `echo R6a-LOCAL-OK`.
- **R7 regression: still correct.**
  - After `kill -9` of the office daemon, the office GUI respawned its own empty daemon.
  - The home tab showed "portty-vm-office stopped sharing this session.", and the CLI printed "Portty: portty-vm-office stopped sharing mx16." and exited 0.
  - Nothing reconnected in the next 52 s.
- **Tests:**
  - Go root package green under `-race`, including 6 `TestRespawnDaemon_*` tests (one with 2 subtests) and 2 `TestRemoteReauth_*` tests.
  - vitest: 86 files, 1160 tests, all passing.
  - `tsc --noEmit` is clean, and `check-traceability-paths.sh` reports OK (124 paths).

## Task Commits

1. `bf628725` test(05-16): RED tests for the GUI and CLI respawn
2. `c2c99648` fix(05-16): respawn from the GUI poller and the attach re-auth
3. `1e0c33c3` fix(05-16): the session poll refreshes the relay port (deviation 1)
4. `e0ad1234` test(05-16): R6 re-run PASS, TESTING.md rows

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] New local tabs dialed the dead daemon's relay port**
- **Found during:** Task 2, the first R6a run (on build `c2c99648`)
- **Issue:** The respawn worked, but a new local tab created a session and then never connected. Its input went nowhere.
  - `relayPort` in App.tsx was set only at init and by Retry.
  - A respawned daemon listens on a new port, so new tabs dialed the dead one.
  - The plan's flagged assumption "Frontend unchanged" was wrong for local tabs. Guest tabs were fine: they fetch the port before each redial.
- **Fix:** The 3 s NAV-01 session poll also calls `GetRelayPort`. It runs `setRelayPort((prev) => (prev !== null && port > 0 ? port : prev))`:
  - init still owns the first set, so launch shows no empty-state flash.
  - A down daemon (port 0) never unmounts local panels.
  - This also covers a daemon that a CLI respawned.
  - The TerminalPanel connect effect does not depend on `relayPort`, so open tabs never reconnect because of it.
- **Test:** a source-pin test in App.nav.test.tsx, in the same style as the existing NAV-01 tests.
- **Files modified:** frontend/src/App.tsx, frontend/src/components/__tests__/App.nav.test.tsx
- **Commit:** `1e0c33c3`. After it, both VMs were redeployed and all three sub-runs were redone.

**2. [Test harness] The healthy-reset test makes a fresh client after stopping the daemon**
- `api.Stop()` closes the listener but leaves the client's keep-alive connection open. A real `kill -9` would close it.
- So `TestRespawnDaemon_HealthyMinuteResetsStreak` gives the app a fresh client for its second phase, with a comment explaining why. Production code is unaffected.

**3. [Rig] New local tab opened from the New Session dialog, not Cmd+T**
- The CGEvent Cmd+T from the click helper opened a tab in the first (pre-fix) run, but did not register in later runs.
- So the local-tab check used the tab bar's New Session dialog (Create Session). The check itself is the same: a new local session after a respawn runs a command.

**4. [Rig] `strings` on the VMs started the Command Line Tools installer**
- The dialog was closed with `kill -9` in both VMs. Nothing was installed.

**Total deviations:** 1 bug fix in the product (frontend relay port), 1 test-harness detail, 2 rig details. **Impact:** the plan's must-have "new local sessions come back with no click" now holds. Without fix 1 it would not.

## How the run worked

- **Builds:**
  - Evidence builds of `c2c99648` and then `1e0c33c3`, each with the local `screenReaderMode: true`. That change was never committed and is reverted.
  - WebGL was off in both VMs' settings.json during the run. Both settings files were backed up to `~/ptest/settings.json.pre16` and restored (sha1 `d2695005...` matches).
- **Session:** office `mx16` (`92a8ef9d...`) with the 2 s ticker, shared. It was recreated by `~/ptest/office-session.sh` after the redeploy.
- **Monitor and kills:**
  - `~/ptest/mon16.sh` on the home VM sampled the daemon pids, the last GUI tick, the GUI banner, the last CLI tick and the status bar about once a second.
  - `~/ptest/kill16.sh` ran each kill and recorded the respawned daemon's ppid and start time. That is how "who respawned" was attributed.
- **GUI stderr:** for the run after (b), the GUI was relaunched with `open --stderr /tmp/gui.err`. That captured the poller's `respawn attempt 1: ok` line in (a2).
- **VM state left:**
  - Both VMs run a clean build of `1e0c33c3` (same code as HEAD `e0ad1234`, only docs differ; binary sha1 `36d93bbe2b7c` on the host and both VMs; `screenReaderMode:!1`). Their original settings are back.
  - The office daemon has no sessions. The expect drivers and monitor are stopped.

## Known Stubs

None.

## Threat Flags

None beyond the plan's threat model.
- T-05-42 is pinned by `TestRespawnDaemon_RateLimitedAndSurfaced` and `TestRespawnDaemon_HealthyMinuteResetsStreak`.
- T-05-44: sub-run (c) left exactly one daemon.
- T-05-45: every kill ran inside a VM over SSH.
- T-05-46: the matrix has no `cap=` token. No share link, cap or auth key was printed or stored.

## Notes for 05-15

- R6 is PASS, so the matrix is now 9 passed, 0 failed, ready for M-90.
- TESTING.md: Go +1 file (app_respawn_test.go). The Go count row is still left for 05-15 to reconcile.
- Known limit, unchanged: `daemon:error` feeds the banner that only shows when no tabs are open. While the daemon is down, the tray icon shows its error state.

## Self-Check: PASSED

- FOUND: app_respawn_test.go, app.go (`respawnDaemonIfDown(time.Now())` in startTrayPoller), cmd_attach.go (`ensureLocalDaemon`, `remoteReauth`)
- FOUND: .planning/phases/05-overnight-survival/05-VM-MATRIX.md (`| R6 | PASS |`, `Rows failed: 0`, other rows byte-identical to 314e61fa, no cap tokens)
- FOUND: TESTING.md (`| SHAR-07 | app_respawn_test.go |`)
- FOUND commits: bf628725, c2c99648, 1e0c33c3, e0ad1234
