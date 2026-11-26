---
phase: 17-desktop-feasibility-spike
plan: 04
subsystem: desktop-spike
tags: [smappservice, tcc, launchd, accessibility, screencapturekit, tart, feas-01, feas-02, r02, r03, r07, r08, r09]

requires:
  - phase: 17-desktop-feasibility-spike
    provides: "17-03 R00 GO (frames and a hardware H.264 session work in the guest), the routing table and the running guest portty-test with ~/ptest/Spike.app"
provides:
  - "R02 GO: the SMAppService helper registers from ~/ptest with no approval step, runs in Aqua and captures Complete frames; Screen Recording is keyed to the parent bundle app.portty.spike, Accessibility to the helper path"
  - "R08-A GO: a CGEvent posted by the helper takes effect and follows the live Accessibility grant; attributed to the helper path"
  - "R09-A GO: the helper shape creates three Settings entries (Screen Recording, Accessibility, Login Items), each with a toggle that revokes and restores live"
  - "R03 GO: the helper keeps capturing with no GUI process, 24 of 24 records Complete"
  - "R07-A GO: launchctl kickstart -k over SSH replaces the helper pid, in the Aqua session, same attribution, no new prompt"
affects: [17-05, 17-06, 17-07, 17-12]

actuals:
  tokens: 53900
  tasks: 2
  commits: 2
plan_head_before: 9ab5f7c2229369e24ce341344cd00feb350e0724
plan_head_after: d91b3fb6cb42bd55775a5787c512d8c9b28f72ad

tech-stack:
  added: []
  patterns: ["tccd responsible attribution for a bundle helper is in the Notifying for access lines (responsiblePID, responsiblePath), not in a responsible= field of AUTHREQ_ATTRIBUTION", "the TCC row key is read from TCCDEvent Modify lines (identifier_type Bundle ID or Path)", "a running process sees a Screen Recording or PostEvent toggle on its next call; AXIsProcessTrusted, CGPreflightPostEventAccess and CGPreflightScreenCaptureAccess keep their first answer for the life of the process"]

key-files:
  created:
    - .planning/phases/17-desktop-feasibility-spike/evidence/r02-helper.jsonl
    - .planning/phases/17-desktop-feasibility-spike/evidence/r02-attribution.txt
    - .planning/phases/17-desktop-feasibility-spike/evidence/r09-A-settings.txt
    - .planning/phases/17-desktop-feasibility-spike/evidence/r03-helper-guiclosed.jsonl
    - .planning/phases/17-desktop-feasibility-spike/evidence/r07-A-kickstart.jsonl
    - .planning/phases/17-desktop-feasibility-spike/evidence/r07-A-attribution.txt
  modified:
    - .planning/phases/17-desktop-feasibility-spike/17-SPIKE-MATRIX.md

key-decisions:
  - "R02 is GO although the helper holds two TCC identities: Screen Recording resolves to the parent bundle app.portty.spike, Accessibility and PostEvent to the helper binary path; the split is recorded in the rows, not hidden"
  - "R09-A counts 3 entries (Screen Recording, Accessibility, Login Items) for the FEAS-02 tiebreaker"

patterns-established:
  - "Revocability is tested live: toggle off, read the next probe records, toggle on, read again, without restarting the process"

requirements-completed: [FEAS-01, FEAS-02]

coverage:
  - id: D1
    description: "R02 helper attribution, registration, prompt text and name, frames and procinfo/launchctl/sfltool corroboration, with evidence"
    requirement: "FEAS-02"
    verification:
      - kind: integration
        ref: "Task 1 verify block (jq on r02-helper.jsonl, AUTHREQ_ATTRIBUTION, kTCCServiceScreenCapture and procinfo greps, matrix R02 row, user-path grep)"
        status: pass
    human_judgment: false
  - id: D2
    description: "R08-A CGEvent effect and Accessibility attribution; R09-A Settings entries with count and revocability"
    requirement: "FEAS-02"
    verification:
      - kind: integration
        ref: "Task 1 verify block (matrix R08-A and R09-A rows, r09-A-settings.txt present)"
        status: pass
    human_judgment: true
    rationale: "The structural checks pass; whether two TCC keys for one helper count as 'one identity' for the R02 verdict is a reading of the decision rule that the verifier should confirm"
  - id: D3
    description: "R03 GUI-closed capture and R07-A SSH kickstart restart, guest left clean"
    requirement: "FEAS-02"
    verification:
      - kind: integration
        ref: "Task 2 verify block (record counts, procinfo section, matrix R03 and R07-A rows) and guest end-state check (no service, no spike process)"
        status: pass
    human_judgment: false

duration: 17min
completed: 2026-11-25
status: complete
---

# Phase 17 Plan 04: the helper shape in the guest Summary

**An SMAppService helper inside Spike.app registers from ~/ptest with no approval, runs in the Aqua session under launchd, captures Complete frames with the GUI closed, survives an SSH `launchctl kickstart -k`, and holds two TCC identities (Screen Recording on the parent bundle, Accessibility on its own path) with three Settings entries (R02, R03, R07-A, R08-A, R09-A all GO on macOS 27.2).**

## Performance

- **Duration:** 17 min
- **Started:** 2026-11-25T02:56:00Z (guest 02:56)
- **Completed:** 2026-11-25T03:13:00Z
- **Tasks:** 2
- **Files:** 6 created, 1 modified

## Accomplishments

- **Registration needs no approval from a dev path (RESEARCH A2).** After a clean reset, `register` from `~/ptest` returned `Enabled` at once. No `/Applications` copy and no RequiresApproval step were needed. A banner read "Spike can run in the background". `launchctl print gui/501/app.portty.spike.helper` showed a pid, `managed_by com.apple.xpc.ServiceManagement` and `parent bundle identifier app.portty.spike`.
- **Prompt and name.** The helper's first record raised "Spike would like to record this computer's screen and audio" (Open System Settings, Deny). The name is the bundle name Spike, not "Portty Spike" and not "spike-helper". No Accessibility prompt and no bypass-picker prompt appeared (the second prompt seen in 17-03 did not recur).
- **Who tccd names.** The accessing identity is `app.portty.spike.helper` (binary under `Contents/Helpers/spike-helper`) for ScreenCapture through `replayd` and for PostEvent through WindowServer. The Notifying lines name `responsiblePID` equal to the helper pid and `responsiblePath` `.../Spike.app` for ScreenCapture, the helper binary path for PostEvent. The ScreenCapture TCC row is keyed to Bundle ID `app.portty.spike` and the Accessibility row to the helper path, so Settings lists "Spike" and "spike-helper" separately. `procinfo` names the helper itself as responsible pid and path.
- **Grants apply live.** A Screen Recording toggle made the already running helper return 1 display and 24 Complete frames in its next record; a restart was not needed. Toggling Accessibility off and on made the one-point pointer nudge fail for 2 records and recover, again without a restart. The cached preflight booleans stay at their first answer until the process restarts (false after a mid-run grant); the posts and the capture follow the live grant.
- **R09-A.** Three entries: Screen and System Audio Recording "Spike", Accessibility "spike-helper", Login Items Background App Activity "Spike". All three toggles revoke and restore live. Turning the Login Items toggle off kills the helper immediately (status RequiresApproval) and on starts it again with no password. The Login Items row has no separate helper entry.
- **R03.** After unregister and register, the registrar exited and only `spike-helper` ran. 24 of 24 records after that were Complete (24 to 26 frames per 3 s, 0 Blank, no prompt).
- **R07-A.** From an SSH shell (managername Background) `launchctl kickstart -k gui/501/app.portty.spike.helper` exited 0 and replaced pid 2643 with 2736. All 19 records report managername Aqua, ppid 1, the same audit session 100002 as the GUI login, 23 to 25 Complete frames, and tccd answered authValue 2 with no new prompt.

## Task Commits

1. **Task 1: R02, R08-A, R09-A** - `bdcf9815` (feat)
2. **Task 2: R03 and R07-A** - `d91b3fb6` (feat)

**Plan metadata:** committed with this summary (docs).

## Deviations from Plan

None - plan executed exactly as written.

Notes that are not deviations:
- The first `tccutil reset` loop passed each "service bundle" pair as one word under zsh and printed "Failed to reset"; it was re-run with separate arguments and the reset was confirmed with `probe --preflight-only` (all three false) before registering.
- A first Accessibility click raised an unlock sheet; the typed password keys went to the Settings sidebar once and navigated it to Notifications. Nothing was changed there. The Accessibility toggle was then turned on, off and on again; the off period is visible in the records and the evidence says so.
- The registrar (`open -n ... --args status`, SMAppService read only, no TCC call) ran for about 3 s once while helper pid 2441 was alive (the re-enabled status check). No probe run overlapped another probe run.
- The `r02-helper.jsonl` evidence holds three pids (1774 before any grant, 2156 after unregister and register, 2441 after the Login Items off and on) because the revocation tests restarted the helper.
- Session user name and UUID in the committed JSONL are `REDACTED`; the developer name in the sfltool excerpt is `DEVELOPER`.

## Authentication Gates

None. The System Settings password sheet is a guest GUI step the agent drove itself (guest login admin, public per CONTEXT).

## Issues Encountered

- **Two TCC identities for one helper.** Screen Recording attaches to the parent bundle (so a rebuild of the same bundle id keeps it, as in R10), Accessibility attaches to the helper path. Phase 19 must request and document both, and the helper path is part of the Accessibility key (a moved bundle changes the path). 17-06 and 17-12 should read R02 with this in mind when they compare identity counts with the daemon shape (R09-B).
- **Whether "one identity as responsible" holds** for the R02 verdict is the executor's reading: ScreenCapture has one responsible path (Spike.app), the Accessibility split is recorded under R08-A.
- **Cached preflight answers.** A Phase 19 helper must not infer a revoked or newly granted state from `CGPreflightScreenCaptureAccess`, `AXIsProcessTrusted` or `CGPreflightPostEventAccess` in a long-running process; zero frames or SCError -3801 and a failed post are the live signals.

## Known Stubs

None.

## Threat Flags

None. T-17-12 held: the helper was unregistered at the end (`launchctl print` reports no such service, `pgrep -f Spike.app` prints nothing). T-17-31 held: only msgID-joined lines for helper pids are kept, user paths read `/Users/USER/`, session identifiers and the developer name are redacted, and the verify greps for user paths and tailnet addresses pass.

## Next Phase Readiness

Ready for 17-05. The guest `portty-test` is still running (never stopped, not on the tailnet), `~/ptest/Spike.app` (CFBundleVersion 20261125025233) holds Screen Recording for `app.portty.spike` and Accessibility for the helper path `.../Contents/Helpers/spike-helper`; the main Mach-O `Contents/MacOS/spike` has no Accessibility grant, so the daemon shape in 17-05 will raise its own Accessibility prompt. No spike process or SMAppService registration remains; `/tmp/spike-evidence/agent-A.json` is left in the guest (label A-kickstart). Settings is open on the Login Items pane.

## Self-Check: PASSED

- Files: r02-helper.jsonl, r02-attribution.txt, r09-A-settings.txt, r03-helper-guiclosed.jsonl, r07-A-kickstart.jsonl, r07-A-attribution.txt and 17-SPIKE-MATRIX.md all exist.
- Commits: bdcf9815 and d91b3fb6 are ancestors of HEAD; `git rev-list --count` from the recorded base is 2.
- Both task verify blocks and every acceptance criterion pass; the guest end state was checked (no service, no spike process, `tart list` shows `portty-test` running).
