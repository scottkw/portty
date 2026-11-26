---
phase: 17-desktop-feasibility-spike
plan: 03
subsystem: desktop-spike
tags: [screencapturekit, videotoolbox, tart, tcc, codesign, rename-swap, feas-01, r00, r10]

requires:
  - phase: 17-desktop-feasibility-spike
    provides: "17-02 signed Spike.app, vmprobe, probe kit (probe, swap) and the matrix skeleton from 17-01"
provides:
  - "R00 GO: ScreenCaptureKit Complete frames, IOSurface and a paravirtualized hardware H.264 session work inside the Tart guest on macOS 27.2 (26B123)"
  - "Routing table: every row R01 to R23 assigned (VM, or host-run for R18 and R20) with a reason derived from R00"
  - "R10 GO: the Screen Recording grant and the designated requirement survive a same-identity rebuild and a RENAME_SWAP bundle swap, with no new prompt"
  - "A running guest portty-test holding ~/ptest/Spike.app (swapped build 20261125025233) with a Screen Recording grant for app.portty.spike"
affects: [17-04, 17-05, 17-06, 17-07, 17-08, 17-09, 17-10, 17-11, 17-12]

actuals:
  tokens: 10640
  tasks: 2
  commits: 2
plan_head_before: 1c15b7344d78a811e7bb0c0c78d4fa9fb47bcd39
plan_head_after: 12b63d949955eb27a710496578c0be4195828a19

tech-stack:
  added: []
  patterns: ["open -n for every TCC-attributed launch, probe JSON written to a file because open drops stdout", "tccd lines joined to the probe pid by msgID: the ATTRIBUTION line carries the pid, CTX and RESULT carry only the msgID", "committed probe records have the session user names and UUID replaced with REDACTED"]

key-files:
  created:
    - .planning/phases/17-desktop-feasibility-spike/evidence/r00-rig.txt
    - .planning/phases/17-desktop-feasibility-spike/evidence/r00-vmprobe.json
    - .planning/phases/17-desktop-feasibility-spike/evidence/r00-frame.png
    - .planning/phases/17-desktop-feasibility-spike/evidence/r00-ffprobe.txt
    - .planning/phases/17-desktop-feasibility-spike/evidence/r00-tcc.log
    - .planning/phases/17-desktop-feasibility-spike/evidence/r10-before.json
    - .planning/phases/17-desktop-feasibility-spike/evidence/r10-after.json
    - .planning/phases/17-desktop-feasibility-spike/evidence/r10-codesign.txt
  modified:
    - .planning/phases/17-desktop-feasibility-spike/17-SPIKE-MATRIX.md

key-decisions:
  - "R00 is GO, so the capture-dependent rows stay in the VM; no row is VM-synthetic and the host run keeps only R18 and R20 (plus the user-session parts of R01 and R14)"
  - "R15 and R16 stay in VM as CONTEXT requires; RESEARCH's real-Mac variants for them are not used"
  - "Matrix Evidence cells hold one evidence path each (the primary file); the other files of a row are named in its Procedure cell"

patterns-established:
  - "A frame captured by the probe can show a system dialog; reading the PNG is part of the verdict, not only the luma number"

requirements-completed: [FEAS-01]

coverage:
  - id: D1
    description: "R00 verdict GO with evidence: Complete frames, ioSurface true, hardware H.264 session, ffprobe Constrained Baseline, PNG of the guest desktop, guest macOS version and build, classified SCError for the first refused run"
    requirement: "FEAS-01"
    verification:
      - kind: integration
        ref: "Task 1 verify block (jq on r00-vmprobe.json, PNG and ffprobe files present, matrix R00 row GO, redaction greps)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Routing table assigns R01 to R23 with reasons from R00; R16 and, under GO, R15 are not host-run"
    requirement: "FEAS-01"
    verification:
      - kind: integration
        ref: "Task 1 verify block (23 routed lines, no PENDING, R15 and R16 guards)"
        status: pass
    human_judgment: true
    rationale: "The structural checks pass, but whether each row's rig suits it is a judgment the verifier should read in the Routing table"
  - id: D3
    description: "R10 GO: same-identity rebuild plus RENAME_SWAP leaves capture working with no new prompt and an unchanged designated requirement"
    requirement: "FEAS-01"
    verification:
      - kind: integration
        ref: "Task 2 verify block (jq on r10-before.json and r10-after.json, designated requirement line, matrix R10 row GO)"
        status: pass
    human_judgment: false

duration: 12min
completed: 2026-11-24
status: complete
---

# Phase 17 Plan 03: R00 in the guest and R10 across a swap Summary

**ScreenCaptureKit, IOSurface and a paravirtualized hardware H.264 encoder all work in the Tart guest (R00 GO, macOS 27.2 build 26B123), every later row is routed from that verdict, and the Screen Recording grant plus the designated requirement survive a same-identity rebuild and a RENAME_SWAP bundle swap (R10 GO).**

## Performance

- **Duration:** 12 min
- **Started:** 2026-11-25T02:45:45Z (about 20:45 local)
- **Completed:** 2026-11-25T02:58:00Z
- **Tasks:** 2
- **Files:** 8 created, 1 modified

## Accomplishments

- **R00 GO.** `vmprobe`, started with `open` in the guest (pid 992), got 28 Complete and 104 Idle frames in 5 s, no Blank or Suspended, `ioSurface` true, first frame 383 ms, 2560 x 1440 in `420v`. The hardware H.264 encoder `paravirtualized:com.apple.videotoolbox.videoencoder.ave.avc` read back hardware; the 29 access units read as Constrained Baseline level 50 in ffprobe; the PNG shows the guest Finder desktop (mean luma 235). Encode latency was mean 38.8 ms, p95 87.2 ms.
- **The grant was driven by the executor.** The first run (`--request`, pid 807) raised the prompt "Spike would like to record this computer's screen and audio" (buttons Open System Settings and Deny) and ended with SCError -3801 (declined, no displays) in under a second. The agent clicked through to Screen & System Audio Recording, toggled "Spike", typed the guest admin password as key codes, and the next run captured. TCC attributes the grant to `app.portty.spike` through `replayd`.
- **New macOS 27.2 behaviour: a second prompt.** While already allowed, the measuring run raised "Spike is requesting to bypass the system private window picker and directly access your screen and audio" (Allow or Open System Settings). Frames kept flowing with the dialog pending, and the dialog stayed after the process exited (it is visible in `r00-frame.png`). After Allow it did not return on the next two launches, including after the swap. R19 should characterise this prompt.
- **Guest facts.** `system_profiler SPDisplaysDataType` prints an empty list in the guest, so the display and GPU facts come from the probe (one 2560 x 1440 display, 1280 x 720 points) and ioreg (AppleParavirtGPU). Gatekeeper showed no dialog: a tar copy carries no quarantine attribute, so no notarization is needed to run in the guest.
- **Routing.** All 23 rows are assigned. VM carries R01 to R17, R19, R21, R22 and R23; host-run carries R18 and R20, each with a VM reference sub-row (R18-vm, R20-vm). No row is VM-synthetic; R15 and R16 stay in VM.
- **R10 GO.** Before: 32 Complete frames. After build.sh rebuilt with the same Developer ID identity (new CFBundleVersion) and `spike swap` exchanged the bundles over SSH, the swapped `~/ptest/Spike.app` ran the probe: `sr_preflight` true, 28 Complete frames, no prompt in screenshots over 14 s, tccd answered authValue 2 for `app.portty.spike`, and `codesign -d -r-` printed the identical designated requirement. REL-01's signing rule can rely on a same-identity re-sign keeping the grant, and the Phase 18 swap needs no re-grant under that rule.

## Task Commits

1. **Task 1: R00 in the guest, grant, verdict and Routing** - `a5466a8e` (feat)
2. **Task 2: R10 rebuild, RENAME_SWAP and grant check** - `12b63d94` (feat)

**Plan metadata:** committed with this summary (docs).

## Deviations from Plan

None - plan executed exactly as written.

Notes that are not deviations:
- `tccutil reset ScreenCapture/Accessibility app.portty.spike` answered "No such bundle identifier" (-10814) on the first deploy because the app had never launched; there was no state to reset. The grant run then created the state from zero.
- The guest's zsh shadows `log` with a builtin; tccd lines were read with `/usr/bin/log show --info --debug`. Without `--info --debug` the AUTHREQ lines are not shown.
- `open` discards stdout, so probes write their JSON with `--out`; `vmprobe --out` takes a directory and `probe --out` takes a file path.
- The first Routing edit script matched the heading text quoted in the matrix intro and duplicated a block; it was caught by the verify block (23 PENDING lines), the file was restored from git and the edit was redone line-anchored before any commit.

## Authentication Gates

None. The System Settings password sheet is a guest GUI step the agent drove itself (guest login admin, public per CONTEXT), not a human gate.

## Issues Encountered

- **Prompt text names "Spike", not "Portty Spike".** The Settings entry also reads "Spike" with a generic icon. DPRM-01 onboarding copy and R09 should note the name shown comes from the bundle name.
- **Capture is not blocked by a pending bypass-picker dialog.** Whether this is "monthly re-approval" in a different guise, or a first-use prompt per bundle path, is open until R19.
- **The guest still has a stopped sibling.** `portty-test-2` (VM home) is stopped; 17-07 owns bringing it and the tailnet back. `portty-test` was started by this plan and is left running; it is not on the tailnet at this point (no tailscale commands were run).

## Known Stubs

None.

## Threat Flags

None. T-17-09 held: the tccd log is filtered by msgID to the probe pids, user paths read `/Users/USER/`, the frame is the guest desktop at 1280 px, probe session objects are redacted, and the verify greps for user paths and tailnet addresses pass. T-17-10 held: the VM was only started, never stopped.

## Next Phase Readiness

Ready for 17-04. The guest `portty-test` is running with `~/ptest/Spike.app` (CFBundleVersion 20261125025233) holding the Screen Recording grant for `app.portty.spike`, `~/ptest/click`, `~/ptest/axdump` present, scratch tool copies `vm-office.sh` and `vshot-office.sh` in the session scratchpad (guest IP 192.168.64.108, ephemeral guest IP: re-check `tart ip`), and `/tmp/spike-evidence/` kept. Accessibility is not granted (`ax_trusted` false, `post_event_preflight` false in both R10 records), which 17-04 to 17-06 must request for R08 and R16. Do not stop the VM once it joins the tailnet in 17-07.

## Self-Check: PASSED

- Files: r00-rig.txt, r00-vmprobe.json, r00-frame.png, r00-ffprobe.txt, r00-tcc.log, r10-before.json, r10-after.json, r10-codesign.txt and 17-SPIKE-MATRIX.md all exist.
- Commits: a5466a8e and 12b63d94 are ancestors of HEAD; `git rev-list --count` from the recorded base is 2.
- Both task verify blocks and every acceptance criterion pass; the Routing table has 0 PENDING lines and 23 routed lines; `tart list` shows `portty-test` running; `~/ptest/Spike.new.app` is absent in the guest.
