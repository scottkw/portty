---
phase: 17-desktop-feasibility-spike
plan: 05
subsystem: desktop-spike
tags: [launchagent, launchd, tcc, accessibility, screencapturekit, ssh, kickstart, tart, feas-01, feas-02, r04, r05, r06, r07, r08, r09]

requires:
  - phase: 17-desktop-feasibility-spike
    provides: "17-04 helper-shape rows (R02, R03, R07-A, R08-A, R09-A), the running guest portty-test with ~/ptest/Spike.app and the routing table"
provides:
  - "R04 GO: the legacy LaunchAgent daemon captures Complete frames with no GUI process under one tccd identity, app.portty.spike, with one Bundle ID key serving Screen Recording and Accessibility"
  - "R04-none and R04-interactive reference numbers: 28.57 against 28.60 Complete fps, no launchd throttle visible in the guest"
  - "R05 GO: a Setsid child of the GUI app captures with no GUI process and shares the Bundle ID grant"
  - "R06 NO-GO (judgment call): an SSH nohup respawn is attributed to sshd-keygen-wrapper, not to the daemon; with that wrapper's Screen Recording off the same command gets SCError -3801"
  - "R07-B GO: launchctl kickstart -k over SSH restarts the LaunchAgent daemon in the Aqua session with the daemon's own grant and no new prompt"
  - "R08-B GO, R08-C GO, R08-D PARTIAL, R09-B GO (3 Settings entries, same count as the helper)"
affects: [17-06, 17-07, 17-12]

actuals:
  tokens: 86800
  tasks: 2
  commits: 2
plan_head_before: f48ce04eee319bb6196bfd6ea7f5b1ff2e64d144
plan_head_after: caaa0afb0ac7dac225bf4ba3ee356f41a04deac7

tech-stack:
  added: []
  patterns: ["tccd checks the AUTHREQ_SUBJECT, not the process that ran: launchd jobs and Setsid children of the bundle's Mach-O resolve to Bundle ID app.portty.spike, an SSH-started process resolves to the responsible sshd-keygen-wrapper", "a legacy LaunchAgent's Login Items row is named after the signing developer, not the app", "Login Items toggle off boots the legacy job out at once and on starts it again with no password"]

key-files:
  created:
    - .planning/phases/17-desktop-feasibility-spike/evidence/r04-daemon.jsonl
    - .planning/phases/17-desktop-feasibility-spike/evidence/r04-attribution.txt
    - .planning/phases/17-desktop-feasibility-spike/evidence/r04-processtype.txt
    - .planning/phases/17-desktop-feasibility-spike/evidence/r09-B-settings.txt
    - .planning/phases/17-desktop-feasibility-spike/evidence/r05-spawn.jsonl
    - .planning/phases/17-desktop-feasibility-spike/evidence/r06-ssh.jsonl
    - .planning/phases/17-desktop-feasibility-spike/evidence/r06-ssh-nogrant.jsonl
    - .planning/phases/17-desktop-feasibility-spike/evidence/r07-B-kickstart.jsonl
    - .planning/phases/17-desktop-feasibility-spike/evidence/r05-r07-attribution.txt
  modified:
    - .planning/phases/17-desktop-feasibility-spike/17-SPIKE-MATRIX.md

key-decisions:
  - "R06 is recorded NO-GO although the first run captured 36 of 36 records: tccd's subject was sshd-keygen-wrapper and the no-grant control failed, so the daemon's own grant is not what worked"
  - "R08-D is PARTIAL because the Accessibility control could not be run without losing the guest's SSH tooling; the base row R08 is therefore PARTIAL with a follow-up"
  - "R09-B counts 3 entries (Screen Recording, Accessibility, Login Items), the same as R09-A, so the tiebreaker cannot separate the shapes on count"

patterns-established:
  - "Revocability is tested live for the daemon too: toggle off, read the next records, toggle on, read again, no restart"
  - "A control run that removes a suspected borrowed grant is added when a path succeeds under an identity other than the one under test"

requirements-completed: [FEAS-01, FEAS-02]

coverage:
  - id: D1
    description: "R04 daemon as legacy LaunchAgent with the GUI closed, R08-B, R09-B and the ProcessType reference rows, with tccd, launchctl, procinfo and sfltool evidence"
    requirement: "FEAS-02"
    verification:
      - kind: integration
        ref: "Task 1 verify block (jq count of label B records, AUTHREQ_ATTRIBUTION and procinfo greps, Interactive in r04-processtype.txt, matrix R04, R04-interactive, R08-B and R09-B rows, user-path grep)"
        status: pass
    human_judgment: false
  - id: D2
    description: "R05, R06, R07-B, R08-C and R08-D for the GUI-spawned, SSH-spawned and SSH-kickstarted daemon, guest left clean"
    requirement: "FEAS-02"
    verification:
      - kind: integration
        ref: "Task 2 verify block (record counts, three procinfo sections, matrix rows R05, R06, R07-B, R08-C, R08-D, R07, R08, user-path grep) and guest end-state check (no service, no spike process, LaunchAgents directory empty)"
        status: pass
    human_judgment: true
    rationale: "The structural checks pass; the R06 NO-GO against its literal GO wording and the R08-D PARTIAL are readings of the matrix criteria that the FEAS-02 reviewer should confirm"

duration: 31min
completed: 2026-11-25
status: complete
---

# Phase 17 Plan 05: the in-daemon shape in the guest Summary

**The plain Spike.app Mach-O run as a legacy LaunchAgent captures Complete frames with no GUI process under one tccd identity (Bundle ID app.portty.spike, shared by Screen Recording and Accessibility) and survives an SSH `launchctl kickstart -k`, while an SSH `nohup` respawn is attributed to `sshd-keygen-wrapper` and only captures when that wrapper holds the grant (R04, R05, R07-B, R08-B, R08-C, R09-B GO; R06 NO-GO; R08-D PARTIAL on macOS 27.2).**

## Performance

- **Duration:** 31 min
- **Started:** 2026-11-25T03:13:30Z (first guest command 03:14:55)
- **Completed:** 2026-11-25T03:44:00Z
- **Tasks:** 2
- **Files:** 9 created, 1 modified

## Accomplishments

- **R04.** `launchctl bootstrap gui/501` of a plist shaped like Portty's own (no ProcessType) worked from the SSH shell with no refusal and no sudo. The first record raised the Screen Recording prompt ("Spike" would like to record this computer's screen and audio, name Spike) and a banner "Software from DEVELOPER can run in the background"; no Accessibility dialog appeared, the request just created the Accessibility row. After the toggle the same pid captured with no restart; after bootout and bootstrap pid 3402 had every preflight true and 21 of 21 early records Complete. All 84 records have ppid 1 and managername Aqua. tccd names one accessing identity, `app.portty.spike`, for ScreenCapture, Accessibility, PostEvent and ListenEvent; the Notifying and BUNDLE_ATTRIBUTION lines show the bare Mach-O resolved to Spike.app; both TCC rows are keyed to Bundle ID `app.portty.spike`. The helper needed two keys; the daemon needs one.
- **ProcessType.** One 30 s `vmprobe` per setting with the stock video screen saver running: launchd reports spawn type daemon (3) without the key and interactive (4) with it, and throughput is the same (28.57 and 28.60 Complete fps, p95 encode latency 33.2 and 35.7 ms). No throttle shows in the guest; real hardware is not excluded.
- **R08-B and R09-B.** The daemon's CGEvent nudge follows the live Accessibility toggle (off for 4 records, on again, no restart). Settings gains one Spike row in Screen Recording, one Spike row in Accessibility (same key) and one Login Items row named after the signing developer, not Spike; 3 entries, all revocable live. Turning the Login Items toggle off makes launchd stop and remove the job at once, on starts it again with no password.
- **R05 and R08-C.** A Setsid child started with `open -n Spike.app --args spawn --quit-after` runs in its own session with the registrar gone, with no GUI process; 36 of 36 records Complete, nudge 37 of 37, tccd subject `app.portty.spike`.
- **R06 and R08-D.** Started from the SSH shell the process is in the SSH audit session (100347, is_remote) with responsible path `/usr/libexec/sshd-keygen-wrapper`, and tccd's subject is that wrapper. It captured 36 of 36 records only because the guest's sshd-keygen-wrapper holds Screen Recording and Accessibility. With the wrapper's Screen Recording toggled off for 43 s, the same command got SCError -3801 in 5 of 5 records while the Spike row stayed on; the toggle was restored.
- **R07-B.** From a Background SSH shell, `launchctl kickstart -k` replaced pid 4603 with 4622 in the Aqua session (audit session 100002), 23 records, 24 to 26 Complete frames, no prompt, tccd subject `app.portty.spike`. R07 base is GO (R07-A and R07-B).

## Task Commits

1. **Task 1: R04, R08-B, R09-B and the ProcessType rows** - `d42f0b6b` (feat)
2. **Task 2: R05, R06, R07-B, R08-C, R08-D** - `caaa0afb` (feat)

**Plan metadata:** committed with this summary (docs).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] R09-B Login Items status text was misreported**
- **Found during:** Task 2 (a later screenshot showed the daemon row as Running in background)
- **Issue:** Task 1 wrote that the row read "Not running in background" while the daemon ran; later screenshots show the text is inconsistent with the process state (running right after first load, not running after the bootout/bootstrap cycle, running again later).
- **Fix:** r09-B-settings.txt and the R09-B matrix cell now say the text did not follow the process state.
- **Files modified:** 17-SPIKE-MATRIX.md, evidence/r09-B-settings.txt
- **Commit:** caaa0afb

**2. [Rule 2 - Missing critical functionality] Added an SSH no-grant control run (evidence/r06-ssh-nogrant.jsonl)**
- **Found during:** Task 2, path D (tccd's subject was sshd-keygen-wrapper, an identity this guest already granted)
- **Issue:** Without a control, a GO for R06 would have recorded a grant that is not the daemon's as if it were.
- **Fix:** turned Screen Recording off for sshd-keygen-wrapper in the guest (03:39:41), ran the same command for 20 s, turned it back on (03:40:24, checked with axdump and a working screenshot). Accessibility was not turned off because the guest's SSH click and axdump tools depend on it; that is why R08-D is PARTIAL.
- **Files modified:** evidence/r06-ssh-nogrant.jsonl (new), evidence/r05-r07-attribution.txt, 17-SPIKE-MATRIX.md
- **Commit:** caaa0afb

### Notes that are not deviations

- `tccutil reset` cannot name the helper (not a bundle id): the Accessibility row `spike-helper` from 17-04 stayed in the guest and the Login Items `Spike` row from the helper's old record stayed too; both are marked as leftovers in r09-B-settings.txt and are not counted for the daemon.
- A first path C run at 03:29:33 was discarded after one record: `killall ScreenSaverEngine` at the end of the ProcessType runs left the guest session locked (CGSSessionScreenIsLocked true). The session was unlocked, the file removed and the run repeated at 03:30:22. The ProcessType runs themselves were taken before the lock.
- The last record of each 3 minute path C and D run (cut by `--duration`) has no frames and a launchctl timeout; counted as a duration artifact, "36 of 36 full records" in the rows.
- One nudge record at 03:21:35 reads false because the executor's own click moved the pointer in that second.
- A `sleep 170` exceeded the tool time limit and ran in the background; no guest state was affected.

## Authentication Gates

None. The System Settings password sheet is a guest GUI step the agent drove itself (guest login, public per CONTEXT).

## Issues Encountered

- **R06 versus its literal GO wording.** The matrix says GO when capture works under a named identity; the first run did. I recorded NO-GO because the identity is the SSH daemon wrapper, not the daemon, and the control run failed without it. Reviewers can reverse this; DAEMON-RESPAWN is true either way because R07-B is GO.
- **R08-D cannot be completed in this guest.** The Accessibility half of the control needs sshd-keygen-wrapper's Accessibility off, which also removes the means to restore it from the SSH rig. A guest not driven over SSH is needed.
- **The Login Items row name.** A legacy LaunchAgent is listed under the developer's name, not the app's. Phase 19 onboarding copy for the daemon shape must say so, and a signed bundle association (SMAppService) is what gives the app name instead.
- **The daemon shape needs one TCC key where the helper needs two** (R02/R08-A), but it has the same Settings count (3), so the FEAS-02 tiebreaker at row 8 does not separate them on count.

## Known Stubs

None.

## Threat Flags

None new. T-17-32 held: the LaunchAgent was booted out and its plist removed at the end, `launchctl print` reports no such service, `pgrep -f Spike.app` prints nothing and the guest LaunchAgents directory is empty. T-17-33 held: tccd lines are msgID-joined to probe pids, user paths read `/Users/USER/`, the session user name and UUID are `REDACTED` in the JSONL, the developer name is `DEVELOPER`, and the verify greps for user paths and tailnet addresses pass. The temporary toggle of sshd-keygen-wrapper's Screen Recording in the guest was restored and checked.

## Next Phase Readiness

Ready for 17-06. The guest `portty-test` is still running (never stopped, not on the tailnet), no spike process or service exists, and `/tmp/spike-evidence/agent-A.json` and `agent-B.json` remain in the guest (labels A-kickstart and B-kickstart). Grants in place: Screen Recording and Accessibility for Bundle ID `app.portty.spike` (daemon) and Accessibility for the helper path (leftover); sshd-keygen-wrapper Screen Recording is on again. System Settings is open on Screen and System Audio Recording. 17-06 reads for the FEAS-02 rule: R04 GO, R02 GO, R03 GO, R09-B GO, R07-B GO (DAEMON-RESPAWN true), R07-A GO; DAEMON-LOCK-OK waits for R17.

## Self-Check: PASSED

- Files: all 9 evidence files and 17-SPIKE-MATRIX.md exist under the phase directory.
- Commits: d42f0b6b and caaa0afb are ancestors of HEAD; `git rev-list --count` from the recorded base is 2.
- Both task verify blocks and the acceptance criteria pass (the jq check prints true for r05-spawn.jsonl; ProcessType was absent from the guest plist after Task 1 and the plist was removed at the end; no service or spike process in the guest; `tart list` shows `portty-test` running).
