---
phase: 05-overnight-survival
plan: 14
subsystem: verification
status: complete
gap_closure: true
tags: [resilience-matrix, tart-vm, tailnet, reconnect, shar-07, verf-01]
requires:
  - phase: 05-09
    provides: "Two-VM rig, axdump, 05-VM-MATRIX.md Feature checks table"
  - phase: 05-12
    provides: "screenReaderMode evidence-build trick, rows.sh, click helper with type"
  - phase: 05-13
    provides: "system:wake reconnect in GUI and browser views; V-13 row"
provides:
  - "05-VM-MATRIX.md `## Resilience matrix`: rows R1-R9 with Result, method, GUI/CLI outcome, reconnect seconds, replay mode, evidence; summary line"
  - "deferred-items.md `## From 05-14`: R6 daemon-respawn gap, the ECONNREFUSED answer, R5 host-wake backoff, sidebar delay, R4 proxy notice time"
  - "Rig pieces in the scratchpad: expect-based CLI driver (cli.exp/cli.sh), pf anchor helper (pf.sh), snapshot, monitor, tick checker, GUI scrollback scanner"
affects: [05-15]
tech-stack:
  added: []
  patterns:
    - "Drive `portty attach` without a human through /usr/bin/expect in a 100x30 pty, with a FIFO for input (macOS `script` rejects a FIFO stdin)"
    - "VM faults through the pf anchor com.apple/p14 (always evaluated from /etc/pf.conf), killing states for the peer IP at load time"
    - "Tick gaps checked in the GUI by paging scrollback with Shift+PageUp/PageDown and reading each page through the AX tree"
key-files:
  created:
    - .planning/phases/05-overnight-survival/05-14-SUMMARY.md
  modified:
    - .planning/phases/05-overnight-survival/05-VM-MATRIX.md
    - .planning/phases/05-overnight-survival/deferred-items.md
key-decisions:
  - "R6 (home daemon restart) recorded as FAIL and not fixed: the orchestrator said to measure and record, and the fix needs a choice of respawn point"
  - "The extra ECONNREFUSED row is R9 (its own row, not part of R7): PASS, because no transient condition produced a refusal"
  - "Both VMs restarted with `tart run --suspendable`, so R4 and R5 used real `tart suspend`, not 05-13's kill -STOP fallback"
requirements-completed: []
duration: 58min
completed: 2026-11-18
estimate:
  tokens: 70000
  tasks: 2
actuals:
  tokens: 6000
  tasks: 2
  commits: 2
plan_head_before: 39b0153e886df8f9b2614d743f1802bd7d8cef35
plan_head_after: 69735aab49438fa8155114684c4abec12c8d3a94
---

# Phase 5 Plan 14: Resilience Matrix on Two Tart VMs Summary

**Matrix rows R1 to R8, plus the orchestrator's extra row R9, were run on the real tailnet (DERP-relayed) between two Tart VMs on build `39b0153e`. In each row the home VM watched the office session through both the GUI tab and `portty attach`. 8 rows pass and 1 fails. R6 FAIL: nothing restarts the viewing Mac's daemon, so after it dies both home viewers retry forever. The ECONNREFUSED question is answered with PASS: no transient condition made the host refuse a connection, so no viewer was falsely told sharing had stopped.**

## Result

| Row | Result | One line |
|-----|--------|----------|
| R1 brief 15 s drop | PASS | No reconnect at all. TCP retransmission carried the gap, with nothing duplicated or missing. |
| R2 70 s silent half-open | PASS | Both viewers showed "reconnecting" at 21 to 28 s and came back 1.6 / 2.6 s after the flush (delta replay). |
| R3 interface down/up, address change | PASS | Back 1.5 / 2.0 s after the interface came up. An address change with a working route caused no reconnect at all. |
| R4 viewing VM sleep (real `tart suspend`, 3 min) | PASS | Back 9.4 / 11.8 s after resume (delta replay). |
| R5 host VM sleep (real `tart suspend`, 3 min) | PASS | Back 32 / 46 s after the office resumed, held back by the backoff. Nothing lost or duplicated. |
| R6 home daemon killed | **FAIL** | Never recovered on its own. Recovered 13 s after a daemon was started by hand. |
| R7 host daemon killed | PASS | Final "stopped sharing" in both viewers within 1 to 3 s. CLI exit 0. Nothing reconnected after the office daemon restarted. |
| R8 328 KB printed during a 76 s gap | PASS | Full replay (338 KB). GUI tab reset and redrawn with no duplicate. CLI cleared and redrawn (05-03 design). |
| R9 transient refusal sweep (extra) | PASS | tailscaled restarts on either side, office en0 flap, office GUI crash and relaunch: no refusal, no false stop. |

**Plan gate: fails, as it should.** The Task 2 automated verify fails on R6 (`GATE: R6 not PASS/DEFERRED`, one FAIL row). V-09, V-12 and V-13 still read PASS, no row is DEFERRED, there is no cap token, and the summary line is present. Task 1's verify (R1 PASS, no cap token) passes.

## Performance

- **Duration:** about 58 min (06:05Z to 07:03Z)
- **Tasks:** 2 of 2
- **Files modified:** 2 (05-VM-MATRIX.md, deferred-items.md), plus this summary

## Task Commits

1. **Task 1 (tracer): rig plus R1:** `764e92ef`
   - Tracer gate (end-of-phase mode, automated verify): the verify was re-run after the commit and passed.
2. **Task 2: R2 to R9 plus findings:** `69735aab`

## How the rig worked

- **Build:** HEAD `39b0153e` (same code as `2af365b6`; only docs differ), real Tailscale, deployed to both VMs.
  - Evidence-only local change: `screenReaderMode: true`, never committed (the same trick as 05-12).
  - WebGL was off in both VMs' settings.json during the run.
  - Afterwards both VMs got a clean HEAD build, and their settings files were restored.
- **Session:** office `mx14` (later `mx14b`, after the office reboot). It ran a background 2 s ticker (`TICK n time`), so the shell prompt stayed free for typing.
- **CLI viewer:** macOS `script` refuses a FIFO stdin ("tcgetattr/ioctl: Operation not supported on socket"). With stdin from /dev/null, it sent ^D and ended a session. So `portty attach` ran under `/usr/bin/expect`, which logs output and forwards a FIFO.
- **Measurements:**
  - A 1 s monitor recorded when each tick arrived.
  - `ticks.sh` checked the CLI log for duplicate ticks and jumps.
  - GUI scrollback was paged with Shift+PageUp/PageDown and read through the AX tree.
  - Typing used `echo <ROW>-GUI-OK | tee -a /tmp/mx.txt` (and the CLI equivalent). Each line had to land once in the office file.

## The R6 failure (for the orchestrator)

- **What breaks:** the home Mac's daemon carries both home viewers. The GUI tab goes through the daemon's proxy, and so does `portty attach`. If the daemon dies:
  - nothing restarts it: no launchd agent unless `portty daemon install` was run
  - `App.startDaemon` runs only at app startup
  - `App.RetryDaemon` runs only from the startup-failure Retry button
  - the GUI tab's re-auth and the CLI redial never call `EnsureDaemon`
- **Measured:** both viewers retried for 1 min 39 s and showed no sign of stopping.
- **After a manual respawn:** `portty list` calls `EnsureDaemon`. With the new daemon (new relay port), both viewers reconnected 13 s later with a delta replay, nothing duplicated, and typing worked.
- **Candidate fixes** (a choice, so it is not made here):
  1. The GUI's existing 5 s tray or health poller calls `EnsureDaemon` when health fails.
  2. The guest-tab re-auth and the CLI redial call `EnsureDaemon` before redialing.
  3. A KeepAlive launchd agent is installed by default.
- Option 1 or 2 is the smallest change. Note that a daemon crash on the viewing Mac also ends that Mac's own local sessions. No respawn can save those.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] The CLI driver needed expect, not script**
- **Found during:** Task 1
- **Issue:** The plan's `script -q ... < FIFO` exits 1 on macOS. The first attempt with /dev/null stdin sent ^D to the shell and ended an office session, which had no viewers and no work in it.
- **Fix:** `cli.exp` (expect, pty 100x30, FIFO input, exit code to a file).
- **Files modified:** none in the repo (scratchpad and VM `~/ptest`)

**2. [Rule 3 - Blocking] VM restarts for real suspend**
- **Found during:** Task 2, R4 and R5
- **Issue:** `tart suspend` needs a VM started with `--suspendable`. Neither VM was.
- **Fix:** Both VMs were stopped and booted with `tart run --no-graphics --suspendable`. This rebooted the office, so session mx14 ended. Its home viewers correctly showed the final stop, and the matrix records that. A new shared session mx14b was used from R5 onward.

**3. [Discretion] Extra runs inside rows**
- R1 was run twice (13 s, then a full 15 s with a ping proof).
- R3 added two address-change runs (with and without an IPv4 route).
- R7 also restarted the office daemon to show that nothing reconnects.
- For R9, "Quit GUI Only" turned out to only hide the window, so the GUI was crashed with `kill -9` and relaunched to get a real relaunch with the daemon running.

**4. [Timing] R2 blocked 72 s instead of 70, R8 blocked 76 s**
- A shell `sleep` got a negative argument. Both stayed well past the 45 s heartbeat, so the rows' meaning is unchanged.

**5. [Orchestrator vs plan] R6 not fixed**
- The plan allows a fix with a regression test. The orchestrator said not to change production code and to measure and record, and to report a FAIL honestly. R6 stays FAIL and the gate fails.

**Total deviations:** 2 blocking fixes outside the repo, 2 minor, and 1 instruction conflict resolved toward the orchestrator. **Impact:** the plan's Task 2 gate fails on R6, as intended.

## Known Stubs

None.

## Threat Flags

None. Nothing ran on the host Mac's network or power. Every pf anchor was flushed and its enable token released; both VMs show `Status: Disabled`. No share link or cap value was printed or stored, and the matrix has no `cap=` token. The user's `shell 1` session on the host was not touched.

## Notes for 05-15

- **New matrix rows for TESTING.md:** R1 to R9. R6 is FAIL until fixed.
- **Manual item for real hardware:** a real access-point or Wi-Fi switch stays with M-85. R3 covered an interface flap and address changes only.
- **Browser views:** not run in this matrix. M-86 covers the VM browser freeze.
- **VM state left:**
  - Both VMs run `--suspendable` (started with nohup `tart run`) on a clean HEAD build, with WebGL settings restored.
  - The office daemon was restarted and has no sessions. The home viewers were closed by the redeploy.
- **Observations in deferred-items:** R5 host-wake recovery is bounded by the backoff; the sidebar took 20 to 45 s to show the office after its reboot; R4 took 9 to 12 s, mostly the proxy noticing its dead sockets.

## Self-Check: PASSED

- FOUND: .planning/phases/05-overnight-survival/05-VM-MATRIX.md (R1 to R9 rows, summary line, 0 cap tokens)
- FOUND: .planning/phases/05-overnight-survival/deferred-items.md (`## From 05-14`)
- FOUND commits: 764e92ef, 69735aab
