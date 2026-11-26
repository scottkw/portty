---
phase: 19-unattended-desktop-viewing
plan: 07
subsystem: vm-proof
tags: [launchd, launchagent, kickstart, keepalive, handoff, rehome, vm, tart, macos-27]
status: complete

requires:
  - phase: 19-02
    provides: launchd-first daemon start, EnsureAgent, kickstart, LegacyCleanup, re-home
  - phase: 19-17
    provides: GUI launchd-first start, fallback repair state, re-home kickstart and legacy cleanup at start
provides:
  - "rig/p19-rig.sh: VM rig (build, deploy, start-gui, sessions, state, quit-gui, quit-all, legacy-plist, clean)"
  - "19-VM-CHECKS.md rows L1, V4, V2, K1, C1, LG1, V3, S1 all PASS on macOS 27.2, with 28 evidence captures"
affects: [19-08, 19-12, 19-13, 19-15, 19-16]

actuals:
  tokens: 23400
  tasks: 3
  commits: 3
plan_head_before: f8f4953b09919cdec0c652e1ac65d926a3beee79
plan_head_after: 4bedefd8bff604639ea90e82fe6837879518b4a5

tech-stack:
  added: []
  patterns:
    - "VM rig modelled on p18-rig.sh: host builds and tars, every launchctl/kill/rm/ /Applications write inside the VM through the ssh helper; state capture beginning `p19-state row=ROW`, scrubbed by sed on the host before it is written"
    - "GUI stderr captured with `open --stdout/--stderr FILE` so the GUI's own log lines are evidence"

key-files:
  created:
    - .planning/phases/19-unattended-desktop-viewing/rig/p19-rig.sh
    - .planning/phases/19-unattended-desktop-viewing/19-VM-CHECKS.md
    - .planning/phases/19-unattended-desktop-viewing/evidence/ (28 *.state captures)
  modified: []

key-decisions:
  - "No product code was changed: no row failed, so the plan's local-fix clause did not apply; the findings below are reported instead"
  - "V4 is decided from XPC_SERVICE_NAME plus the launchctl pid because 19-02 writes no log line naming the ManagedByLaunchd branch"
  - "Builds use the vmpreview tag (as p18-rig.sh) so the guest never joins or queries a real tailnet for these rows"

requirements-completed: [DSHR-01]
---

# Phase 19 Plan 07: VM proof of the launchd start model Summary

The launchd-primary start model of 19-02 and 19-17 behaves as designed on real macOS 27.2: all eight rows (L1, V2, V3, V4, K1, C1, LG1, S1) PASS in the portty-test VM, with no CONTRADICTED verdict, so later plans can build on it.

## What was built

- **Task 1 (tracer, `604c85a5`):** `p19-rig.sh` plus rows L1 and V4. A first GUI start in a clean VM wrote the plist (label `app.portty.Portty.daemon`, `KeepAlive {SuccessfulExit false}`, `AbandonProcessGroup`, `RunAtLoad`), launchctl showed the job running with the daemon's pid, ppid 1, `/health launchd:true`; XPC_SERVICE_NAME equalled the label.
- **Task 2 (`8f55eeec`):** V2 (kickstart semantics, disabled agent falls back and terminals start, enable plus GUI start re-homes the same shells), K1 (SIGKILL restarted in about 1 s with no GUI running, Quit All stays down, an unreadable `capability.key` exits 1 and is retried then recovers 4 s after the fix), C1 (CLI over ssh: launchd start with no sshd ancestor; with the agent removed the repair sentence goes to stderr and the fallback still serves), LG1 (legacy job never booted out while it holds the lock, gone at the next start), S1 (`portty daemon stop` exits 0 and stays down).
- **Task 3 (`4bedefd8`):** V3. 19.0.1 to 19.0.2 over a launchd-managed daemon with two ticking shells: takeover then re-home inside 4 s, shell pids 23247 and 23298 unchanged, ticks 22 to 122 and 16 to 116, no gap above 2 s (1 s epoch granularity), `/health launchd:true` on 19.0.2, the old job exited 0 and was not restarted.

Assumptions A3 (plain kickstart), A4 (XPC_SERVICE_NAME) and A5 (shells survive a re-home out of a managed job) are confirmed; the real `launchctl print` and `print-disabled` formats are recorded in 19-VM-CHECKS.md under "Findings".

## Findings for the orchestrator (none blocks Phase 19)

1. **A launchd-started daemon writes no log.** The plist has no StandardOutPath/StandardErrorPath and only the detached spawner opens `daemon.log`, so the daemon's stderr (relay port, handoff lines, "re-home not done: ...") goes nowhere under launchd; the fallback daemon logged its lines to `daemon.log.1` in the same VM at the same time. Given "rock-solid first, nobody can get in to fix the remote Mac", this is the one finding worth acting on. A fix is local to `process.go` (open `daemon.log` and dup it onto fds 1 and 2 when stderr is not already that file; `startDaemonLog` then rotates as it does for the fallback) or to the plist (StandardErrorPath), which changes the golden plist tests. I did not make it: no row failed, and the plan limits in-plan fixes to failures.
2. **ppid 1 does not prove launchd.** A CLI-spawned detached fallback daemon also has ppid 1 (C1). The sure signals are `/health launchd:true`, the launchctl pid and XPC_SERVICE_NAME.
3. **A loaded legacy `portty-daemon` job revives a daemon after Quit All** until two GUI starts have booted it out (`KeepAlive true` in the old kardianos plist). It is bounded and the shells survive every re-home, so it is recorded, not a failure.
4. **Not checked (out of this plan's scope):** the Login Items URL pane on macOS 27.2 and the Settings needs-repair row (19-13 and 19-15 F1).

## Deviations from Plan

### Auto-fixed Issues

None - no product code changed.

### Plan-shape notes (not defects)

- `deploy` takes `--keep-daemon` (V3 deploys N+1 over N without Quit All), and the rig gained `start-gui`, `quit-all` and `legacy-plist` subcommands beyond the plan's list; row captures use extra names (for example `V2-disabled`, `LG1-quitall`) besides the required `ROW-a` and `ROW-b`.
- Evidence home paths are shown as `~` (orchestrator rule) rather than `/Users/USER`.
- `frontend/dist` and `frontend/node_modules` did not exist in this worktree, so `pnpm install --frozen-lockfile` and `pnpm run build` were run once before `wails build` (both are gitignored output; no package was added or changed).
- Commit trailer is `Claude Sonnet 5.5` per the orchestrator, not the Opus line in the shared prompt.
- The VM-side `launchctl enable` in `clean` leaves explicit `=> enabled` entries in `print-disabled`; harmless.

## Rig safety review (acceptance criterion)

Host-side commands in `p19-rig.sh` are limited to `tart list`, `wails build`, `codesign` of the build output copy, `tar`, `jq`, `sed` and writing evidence files. Every `launchctl`, `kill`, `rm`, `chmod`, `open` and `/Applications` write sits inside a `vm`/`vmsh` string or heredoc. Every VM-touching subcommand starts with `require_vm`, which refuses unless `tart list` shows portty-test running. The VMs were never stopped; guest tailscale was Running before and after.

## VM end state

portty-test has Portty 19.0.2 in /Applications running under launchd (job pid 23729, `/health launchd:true`), no sessions, no legacy plist, `capability.key` mode 600, agent enabled. portty-test-2 was not used.

## Known Stubs

None.

## Threat Flags

None. Evidence was grepped for 100.x addresses, ts.net names and `cap=` (none), and `tests/check-brand.sh` is clean.

## Self-Check: PASSED

- Files exist: rig/p19-rig.sh (executable, `bash -n` clean), 19-VM-CHECKS.md, evidence/ with L1, V2-a/b, K1-a/b, C1-a/b, LG1-a/b, S1-a/b, V3-a/b.
- Commits `604c85a5`, `8f55eeec`, `4bedefd8` are ancestors of HEAD.
