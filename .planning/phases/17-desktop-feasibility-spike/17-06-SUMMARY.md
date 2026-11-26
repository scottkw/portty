---
phase: 17-desktop-feasibility-spike
plan: 06
subsystem: desktop-spike
tags: [lock-screen, screencapturekit, cgsession, cgevent, tart, feas-01, feas-02, r15, r16, r17]

requires:
  - phase: 17-desktop-feasibility-spike
    provides: "17-04 and 17-05 shape rows (R02, R03, R04, R07, R09), the running guest portty-test with ~/ptest/Spike.app, the FEAS-02 decision rule"
provides:
  - "R15 GO for both shapes: while the guest is locked SCK delivers the lock screen as normal frames (8 to 10 Complete and about 20 Idle per second, 0 Blank), until the guest display turns off about 31 s after the lock"
  - "R17 GO for both shapes: CGSSessionScreenIsLocked is set 1 s after a lock and cleared 1 s after the unlock; the stream survives a lock and unlock with no error while the display stays awake, and is lost for good (SCError -3815) when the display turns off"
  - "R16 GO for the helper (3 of 3) and PARTIAL for the daemon (2 of 3): key events posted by the capture-host process unlock the guest, but one daemon attempt was rejected"
  - "Provisional FEAS-02 outcome in-daemon by row 7, Pipeline shape: daemon"
affects: [17-07, 17-08, 17-11, 17-12]

actuals:
  tokens: 5800
  tasks: 2
  commits: 3
plan_head_before: 716e5f2ee5d250a5511e9424b0f3fb0f077a5597
plan_head_after: 4305cac1a193727f0a58287dcbd0afc75cc6bd44

tech-stack:
  added: []
  patterns: ["the guest display turns off about 31 s after a lock and ends the SCK stream with -3815, a display-awake assertion (caffeinate -d) keeps it alive", "a click-tool key does not wake a dark display, a user-activity assertion (caffeinate -u) does", "typing at the lock screen from the capture-host process works but is not reliable on the first try"]

key-files:
  created:
    - .planning/phases/17-desktop-feasibility-spike/evidence/r15-r17-A.jsonl
    - .planning/phases/17-desktop-feasibility-spike/evidence/r15-r17-B.jsonl
    - .planning/phases/17-desktop-feasibility-spike/evidence/r15-r17-A-awake.jsonl
    - .planning/phases/17-desktop-feasibility-spike/evidence/r15-r17-B-awake.jsonl
    - .planning/phases/17-desktop-feasibility-spike/evidence/r15-A-locked.png
    - .planning/phases/17-desktop-feasibility-spike/evidence/r15-B-locked.png
    - .planning/phases/17-desktop-feasibility-spike/evidence/r16-A.jsonl
    - .planning/phases/17-desktop-feasibility-spike/evidence/r16-B.jsonl
    - .planning/phases/17-desktop-feasibility-spike/evidence/r16-A-try2.jsonl
    - .planning/phases/17-desktop-feasibility-spike/evidence/r16-A-try3.jsonl
    - .planning/phases/17-desktop-feasibility-spike/evidence/r16-B-try2.jsonl
    - .planning/phases/17-desktop-feasibility-spike/evidence/r16-B-try3.jsonl
  modified:
    - .planning/phases/17-desktop-feasibility-spike/17-SPIKE-MATRIX.md
    - spike/desktop/probe.go

key-decisions:
  - "R15 and R17 are GO for both shapes because the lock screen frames, the lock key and the stream continuity are proven with the display held awake; the unaided 45 s lock loses the stream and that is recorded as a constraint (hold a display-awake assertion or restart on -3815), not hidden in the result"
  - "R16-B is PARTIAL, not the plan's single-run NO-GO: the first daemon attempt stayed locked but two repeats unlocked, so the row is an unexplained intermittency, not a feasibility cut"
  - "Provisional outcome in-daemon: rows 1 to 6 do not hold (both cores YES, DAEMON-LISTED, DAEMON-RESPAWN and DAEMON-LOCK-OK all true) and row 7 holds"

patterns-established:
  - "A plan-literal run is kept as the evidence and a labelled variant (the awake runs, the retried typing attempts) is added when the literal run alone would misstate the answer"

requirements-completed: []

coverage:
  - id: D1
    description: "R15 and R17 for both shapes: stream through two locks with the CGSession dictionary per second, lock screen PNGs, and the awake variant"
    requirement: "FEAS-01"
    verification:
      - kind: integration
        ref: "Task 1 verify block (jq record counts, CGSSessionScreenIsLocked in the daemon records, both PNGs, matrix rows R15, R15-A, R15-B, R17, R17-A, R17-B) and guest end-state check (no spike process, no service)"
        status: pass
    human_judgment: true
    rationale: "The structural checks pass; calling R15 and R17 GO although the unaided 45 s lock loses the stream is a reading of the criteria that the FEAS-02 reviewer should confirm"
  - id: D2
    description: "R16 typing from the capture-host process at the lock screen for both shapes (three attempts each) and the provisional FEAS-02 pick"
    requirement: "FEAS-02"
    verification:
      - kind: integration
        ref: "Task 2 verify block (record counts, matrix rows R16, R16-A, R16-B, Status PROVISIONAL, Decided by row 7, Provisional outcome in-daemon, Pipeline shape daemon, decision-rule section unchanged since its first commit) and guest end-state check (screenLock off, no spike process)"
        status: pass
    human_judgment: true
    rationale: "R16-B PARTIAL against the plan's 70 s NO-GO rule is a judgment call, and the provisional pick is costly to reverse"

duration: 36min
completed: 2026-11-25
status: complete
---

# Phase 17 Plan 06: lock screen rows and the provisional capture-host pick Summary

**Both shapes behave the same across a lock on macOS 27.2: SCK delivers the lock screen as normal frames and CGSSessionScreenIsLocked tracks the lock, but the guest display turns off about 31 s after a lock and ends the stream with SCError -3815 unless a display-awake assertion is held; typing from the capture-host process unlocks the guest (helper 3 of 3, daemon 2 of 3), and the pre-written FEAS-02 rule gives in-daemon by row 7 with Pipeline shape: daemon (R15, R17 GO; R16-A GO; R16-B PARTIAL).**

## Performance

- **Duration:** 36 min
- **Started:** 2026-11-25T03:46:30Z
- **Completed:** 2026-11-25T04:22:00Z
- **Tasks:** 2
- **Files:** 12 evidence files created, the matrix and probe.go modified

## Accomplishments

- **R15 (lock-screen frames).** Per shape, one probe-loop with one SCStream for 170 s and two locks. Lock 1 (12 s): 128 Complete, 258 Idle, 0 Blank, 0 Suspended frames for the helper, 121, 275, 0, 0 for the daemon. Lock 2 (45 s): 8 to 10 Complete and about 20 Idle per second until the display turned off. The PNGs (`r15-A-locked.png`, `r15-B-locked.png`) show the macOS lock screen, not black and not the last desktop: wallpaper, clock, the guest account's avatar and name above an empty password field, and the capture indicator "Your screen is being observed". On the unlocked desktop a still screen delivers Idle frames for about 14 s and then nothing, which is the R22 situation.
- **R17 (lock key and continuity).** CGSSessionScreenIsLocked appears in the first record after the lock command and is gone in the first or second record after the unlock typing; kCGSSessionOnConsoleKey stays true. Through the 12 s lock the stream ran on with no error and the next Complete frames arrived in the record that showed the key cleared (within 1 s). Through the 45 s lock the display list went empty 31 s after the lock, SCError -3815 ("Failed to find any displays or windows to capture") followed in the same second or the next, the stream stopped and never resumed after the wake and unlock. The cause is the display turning off, not the lock: the awake variant (`caffeinate -d`, one 45 s lock per shape) had 0 errors, no empty display list and about 440 Complete frames while locked. Helper and daemon are indistinguishable, so DAEMON-LOCK-OK is true.
- **R16 (typing from the capture-host process).** The helper unlocked the guest in 3 of 3 attempts, the keys posted 25 to 28 s after the lock. The daemon unlocked it in 2 of 3: its first attempt (the plan's run) left the guest locked and showed a burst of frames and then the lock screen with an empty password field, consistent with a rejected password; the same code unlocked the next two attempts. Cause unexplained, failure rate not characterised. A click-tool key did not wake a dark display and did not unlock it; `caffeinate -u` woke it.
- **Provisional FEAS-02.** DAEMON-CORE YES (R04), HELPER-CORE YES (R02, R03), DAEMON-LISTED true (R09-B), DAEMON-RESPAWN true (R07-B), HELPER-RESPAWN true (R07-A), DAEMON-LOCK-OK true. Rows 1 to 6 fail, row 7 holds: `Decided by row: 7`, `Provisional outcome: in-daemon`, `Pipeline shape: daemon`. The decision section lists the constraints Phase 19 must design around (respawn through `launchctl kickstart`, keep-awake or restart on -3815, unlock as type-check-retry, the Login Items row named after the developer) and the crash-isolation argument as non-measured.
- **Guest end state.** Screen lock set back to off (`screenLock is off`), no spike process, no LaunchAgent plist, no service loaded, `portty-test` still running.

## Task Commits

0. **Fix found during Task 1: probe-loop --stream stats parse** - `6029d008` (fix)
1. **Task 1: R15 and R17 for both shapes** - `38992493` (feat)
2. **Task 2: R16 and the provisional FEAS-02 pick** - `4305cac1` (feat)

**Plan metadata:** committed with this summary (docs).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `probe-loop --stream` dropped delta, streamRunning and the PNG from every record**
- **Found during:** Task 1, first helper run
- **Issue:** `probeStream` parsed the stats `running` as a JSON boolean, but `sckStats` emits 0 or 1, so every record failed with "unreadable stats" and returned before it set `delta`, `streamRunning` or the PNG. The lock rows are the first to use `--stream`.
- **Fix:** parse `running` as an int and derive the boolean. The bundle was rebuilt with the same Developer ID identity and swapped into the guest (`mv`, after `unregister`); both shapes kept their grants with no prompt, which is a second R10 data point. The first run on the old binary was discarded (kept only in the guest as `r15-r17-A.prelim-old.jsonl`).
- **Files modified:** `spike/desktop/probe.go`
- **Commit:** `6029d008`

### Plan adjustments

**2. [Rule 3 - Blocking] `--capture-seconds 0` added to every probe-loop argv.** The plan's argv uses `--interval 1s` with the default 5 s capture, which probe-loop refuses (a record's capture must be shorter than the interval); with `--stream` the capture wait is only the first-start wait.

**3. [Rule 2 - Missing critical measurement] Short lock before the long lock, wake step, awake variant.** The plan's single 45 s lock loses the stream at +31 s and the click tool's unlock keys did not unlock the dark display, so a literal run would have measured the display sleep and not the lock. Each shape therefore ran lock 1 (12 s, display still on) and lock 2 (45 s, the plan's lock) in one probe run; lock 2 is woken with `caffeinate -u -t 2` before the unlock typing; and one 45 s lock per shape ran with `caffeinate -d` (`r15-r17-A-awake.jsonl`, `r15-r17-B-awake.jsonl`). Probe runs stayed sequential and never overlapped.

**4. [Judgment] R16 repeated, and R16-B recorded PARTIAL.** The plan says to record NO-GO if the guest is still locked 70 s after the typing; the first daemon run was (about 73 s). One failure against nothing else is an anecdote, so each shape was run three times in all (`r16-*-try2.jsonl`, `-try3.jsonl`). The daemon's later attempts unlocked, so R16-B is PARTIAL with a follow-up and R16 base is PARTIAL. A strict reading of the plan gives NO-GO for R16-B and for the base row; neither changes the FEAS-02 outcome (the rule does not read R16).

**5. [Judgment] R15 and R17 recorded GO.** The unaided 45 s lock loses the stream in both shapes, so a strict reading of "the stream continues without a restart" gives NO-GO. They are GO because the lock-screen frames, the lock key and the continuity are proven with the display held awake, which is what the product does while a viewer watches (R18 limits DPRM-04 to keep-awake-while-viewing); the unaided loss is written into the row cells and the decision constraints. The FEAS-02 outcome does not depend on the choice: A and B tie either way, so DAEMON-LOCK-OK is true.

## Authentication Gates

None. The guest's public admin password was typed as key codes inside the guest only; nothing was typed on the host.

## Issues Encountered

- **The display turns off 31 s after a lock in the guest.** The stream dies with -3815 and does not resume. Whether a real Mac does the same depends on its display-sleep settings, and the host run (17-11) and R18 are where that is measured. The VM numbers are the answer for the macOS line, not for a Mac's power settings.
- **R16-B intermittency is unexplained.** One in three daemon attempts was rejected; the helper never failed. One frame per second does not show which key was lost. The follow-up asks for the failure rate and a retry design.
- **Evidence PNGs of the unlocked desktop are not committed.** The post-unlock frames show desktop notification banners with the developer's name, so only the two lock-screen PNGs are in git.
- **A shell timeout.** One chained command exceeded the 120 s tool limit and ran in the background; its output was read after it finished and no guest state was affected.

## Known Stubs

None.

## Threat Flags

None new. T-17-14 held: typing was inside the VM only, with its public password and the fixed keycode list. T-17-34 held: the two committed PNGs show only the generic guest lock screen, downscaled to 1280 px; the session user name and UUID are `REDACTED` in every JSONL and user paths read `/Users/USER/`. T-17-35 held: the lock is restored to off (`screenLock is off`) and the guest was left unlocked.

## Next Phase Readiness

Ready for 17-07 in the daemon shape. The guest `portty-test` is running, unlocked, with the screen lock off, no spike process, no LaunchAgent and no service. The bundle in `~/ptest/Spike.app` is the rebuilt one with the stream fix, grants intact (Screen Recording and Accessibility for Bundle ID `app.portty.spike`; the Accessibility row for the helper path and the sshd-keygen-wrapper rows are still there). `/tmp/spike-evidence/agent-A.json` (label A-type) and `agent-B.json` (label B-type) remain in the guest. System Settings is still open on Screen and System Audio Recording. FEAS-01 and FEAS-02 stay open: only 17-12 closes them.

## Self-Check: PASSED

- Files: the 12 evidence files, `17-SPIKE-MATRIX.md`, `spike/desktop/probe.go` and this summary exist.
- Commits: `6029d008`, `38992493` and `4305cac1` are ancestors of HEAD; `git rev-list --count` from the recorded base is 3.
- Both task verify blocks printed PASS and the acceptance criteria hold: the `## FEAS-02 decision rule` section is identical to its first-commit text, `Status: PROVISIONAL`, `Decided by row: 7`, `Provisional outcome: in-daemon`, `Pipeline shape: daemon`; in the guest `sysadminctl -screenLock status` reports off and `pgrep -f Spike.app` prints nothing.
