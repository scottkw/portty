---
phase: 17-desktop-feasibility-spike
plan: 10
subsystem: desktop-spike
tags: [host-run, screencapture, display-sleep, wake, lock, ssim, encoder, tart, feas-01, r18, r20]

requires:
  - phase: 17-desktop-feasibility-spike
    provides: "17-08 guest state: both VMs running and on the tailnet, signed Spike.app in VM office with its grants, the daemon-shape pion serve, viewer.html, build.sh"
provides:
  - "`spike host-run`: serve --source sck behind a full-screen test chart, with a pinned-event timeline, perf samples every 5 s (fps, per-window encode latency, own CPU via proc_pid_rusage and mach_timebase_info, RSS), scheduled display sleep, wake and typing, frame dump and a 3 s H.264 segment for SSIM"
  - "R18-vm NO-GO (reference row): pmset displaysleepnow ends the SCK stream with SCError -3815 within 0.3 s and the stream never comes back after the wake; the lock, the typed unlock and the wake call all worked"
  - "R20-vm GO (reference row): the guest's paravirtualized AVC encoder reads hardware true; motion window 28.70 fps, encode latency mean 5.38 ms, own CPU 12.4 percent; SSIM 0.9968 on a still IDR"
  - "Two tested host-run command lines for 17-11 (default with no key-input flag, and the typing additions) and the rehearsal step list, in the session scratchpad and below"
affects: [17-11, 17-12, DPRM-04, DESK-01, DESK-07]

actuals:
  tokens: 16720
  tasks: 2
  commits: 3
plan_head_before: e7151bfa7d5bba99e55ee5c553bf1bc4ba4d27af
plan_head_after: 54c4784a4c81f29d61870711e4321627cd01aa40

tech-stack:
  added: []
  patterns: ["a run schedule whose zero is the moment the capture is running, so offsets match between VM rehearsal and the real Mac", "serve gains optional control, peerState and tap hooks instead of host-run copying the server", "per-window encode latency read from the latency ring by a cursor (spk_sck_lat_since), so static and motion windows get their own figures"]

key-files:
  created:
    - spike/desktop/hostrun.go
    - spike/desktop/hostrun_darwin.m
    - spike/desktop/hostrun_test.go
    - .planning/phases/17-desktop-feasibility-spike/evidence/r18-vm-timeline.jsonl
    - .planning/phases/17-desktop-feasibility-spike/evidence/r20-vm.txt
  modified:
    - spike/desktop/bridge_darwin.go
    - spike/desktop/main.go
    - spike/desktop/server.go
    - spike/desktop/media_darwin.m
    - .planning/phases/17-desktop-feasibility-spike/17-SPIKE-MATRIX.md

key-decisions:
  - "R18-vm is NO-GO against R18's own criterion (wake brings frames back): the stream ends at display sleep and is not restarted; this matches 17-06's -3815 finding and maps to R18's pre-written consequence (DPRM-04 limited to keep-awake-while-viewing)"
  - "No restart-after-wake flag was added: the plan's R18 consequence already covers a dead stream, and a restart option would change what the real-Mac run measures"
  - "host-run exits before serving when Screen Recording is not granted, naming the fix, instead of failing later inside SCK"
  - "Typing needs --allow-input and a keycode in --allow-keycodes, for the scheduled case too; parse refuses it with exit 2 (checked on the host, parse only)"

patterns-established:
  - "A rehearsal that reproduces the host sequence also reproduces its gaps: the guest does not lock on display sleep, so the lock step was added to the rehearsal as a labelled extra"

requirements-completed: []  # FEAS-01 stays open: only 17-12 closes it (and FEAS-02)

coverage:
  - id: D1
    description: "host-run builds: vet, gofmt, golangci-lint 0 issues, build.sh, usage lists host-run, binary links AppKit, release tests and check-brand pass, four acceptance greps"
    requirement: "FEAS-01"
    verification:
      - kind: other
        ref: "Task 1 verify block and acceptance criteria (go vet, gofmt -l empty, golangci-lint 0 issues, build.sh signed, host-run in usage, otool AppKit, TestSER03_NoAutoSavePatterns, TestPRG_OffPath_NoProgressLogic, tests/check-brand.sh)"
        status: pass
      - kind: unit
        ref: "spike/desktop/hostrun_test.go: key gate, typing refused without allow-list, start flags hide token path and address, timeline lines carry pinned keys, frame class"
        status: pass
    human_judgment: false
  - id: D2
    description: "R18-vm and R20-vm: the host sequence rehearsed in VM office with VM home Safari as viewer; timeline with 40 perf lines, sleep, wake and a key line; r20-vm.txt with the SSIM line; both rows state the launch-path deviation; nothing leaks"
    requirement: "FEAS-01"
    verification:
      - kind: other
        ref: "Task 2 verify block: perf count >= 20, sleep and wake events, wall/ms/event on every line, event values pinned, key line present, SSIM line, R18-vm and R20-vm results, launch-path text in both rows, no 100.x address or token in the evidence"
        status: pass
    human_judgment: false
  - id: D3
    description: "The default host command and the typing additions ran in VM office as written (0 key lines and 36 perf lines; one key line with codes [7])"
    requirement: "FEAS-01"
    verification:
      - kind: other
        ref: "run 3 timelines read in the guest copy: key lines 0 and 1, perf lines 36 and 6; spike process absent and screenLock off afterwards"
        status: pass
    human_judgment: false
  - id: D4
    description: "Frames of the host's real desktop never reach evidence; the chart covers the display; notification banners may sit above it"
    verification: []
    human_judgment: true
    rationale: "Only a person looking at raw-1.png on the real Mac can confirm the chart alone is captured; in the VM it was (the screenshots show the chart over the menu bar), but banners and other high-level windows were not provoked. 17-11 should read every frame before anything is committed."

duration: about 35 min
completed: 2026-11-25
status: complete
---

# Phase 17 Plan 10: host-run and its VM rehearsal Summary

**`spike host-run` serves the live display behind a full-screen test chart with a pinned-event timeline, and its rehearsal in VM office shows display sleep ending the SCK stream with -3815 (no frames after the wake), a guest lock plus typed unlock that works, and a still-picture SSIM of 0.9968 from a hardware-flagged paravirtualized encoder at 28.7 fps under motion.**

## Performance

- **Duration:** about 35 min
- **Started:** 2026-11-25T05:47:27Z
- **Completed:** 2026-11-25T06:20:00Z (approximate)
- **Tasks:** 2 of 2
- **Files:** 5 created, 5 modified (10 in the diff against the plan start)

## Accomplishments

- **host-run.** One subcommand: `serve --source sck` behind hooks (`control`, `peerState`, `tap` on `serveOpts`), a borderless window at `NSScreenSaverWindowLevel` over the primary display with 9 to 20 pt text, a one-pixel grid, a gray ramp and a 60 Hz motion region, and AppKit on the main thread (the main goroutine is locked to it in `init` when argv[1] is host-run). The timeline (`host-run-timeline.jsonl`, O_EXCL) carries `wall`, `ms`, `event` on every line with exactly the 13 pinned event values. Perf lines every 5 s carry fps, per-window encode latency (n, mean, p95, max), own CPU percent (proc_pid_rusage ticks converted with mach_timebase_info), RSS, and motion, asleep and locked flags. Lock and display state are polled every second, frame-status runs every 250 ms (50 ms while a wake awaits its first frame).
- **Key input is gated three ways.** Off unless `--allow-input`; only keycodes in `--allow-keycodes` (empty by default); `--type-at` is refused at parse time (exit 2) unless every code is allowed. A `key` line carries `codes`, `source`, `rc` and no character field. The start event omits the token and the evidence path and redacts the bind address (`100.x.x.1`).
- **R18-vm (NO-GO, reference row).** `pmset displaysleepnow` at ready+120 s; the stream ended at most 0.3 s later with SCError -3815 and read `running` false to the end; the wake call (IOReturn 0) at ready+140 s woke the display within the next 1 s poll; no `first_frame_after_wake` line because the stream never restarted. A viewer joining at 123 s during the sleep got 19 frames in 45 s, all the stale cached chart (the idle keepalive keeps re-sending the last picture), not the lock screen.
- **Lock and typed unlock in the guest.** The first run showed that display sleep plus wake did not lock the guest (screenLock immediate was set); the committed run added Ctrl+Cmd+Q at about ready+150 s, giving a `lock` event at 150.1 s, and host-run's scheduled typing of the public VM password as keycodes at ready+160 s cleared it by the next poll.
- **R20-vm (GO, reference row).** Encoder `paravirtualized:...ave.avc`, hardware flag true (the plan expected software), captured 2560 x 1440. Motion window (60 s, 12 samples): 28.70 Complete fps (27.60 to 28.80), encode latency mean 5.38 ms, per-sample p95 median 14.18 ms, max 30.38 ms, own CPU 12.43 percent (top: WindowServer 15.0, spike 12.2, VTEncoderXPCServ 2.3, replayd 1.1, system 87.6 percent idle). Static window: 0.60 fps (a still screen gives Idle frames, then nothing), own CPU about 1 percent. SSIM of a decoded forced IDR against the raw frame: All 0.996833, luma 0.999135, PSNR 45.55 dB. Safari report: 1521 frames decoded, 20 dropped, 4.46 ms decode per frame, jitter buffer 0.307 s per frame averaged over the whole session.

## Task Commits

1. **Task 1: host-run, chart, timeline, scheduled sleep, wake and allow-listed typing** - `d594ce26`
2. **Task 1 fix: chart layout (panels overlapped the grid and ramp)** - `fdddb351`
3. **Task 2: R18-vm and R20-vm, timeline and r20-vm.txt** - `54c4784a`

## Tested host-run command lines for 17-11

Placeholders: `<IP>` the Mac's own tailnet address from `tailscale ip -4`, `<T>` a fresh 32-hex token, `<scratch>` a scratch directory outside `~/Downloads` and `/Applications`. Launch with `open -n --stdout <scratch>/hostrun.out --stderr <scratch>/hostrun.err spike/desktop/build/Spike.app --args host-run <flags>`.

- **(a) Default** (ran in VM office as written: 36 perf lines, 0 key lines, exit at ready+180 s; no `--allow-input`, `--allow-keycodes`, `--type-at` or `--type-keycodes`):
  `host-run --bind <IP> --token <T> --evidence-dir <scratch>/hostrun --pattern --dump-at 15s --motion-at 45s --motion-for 60s --sleep-at 120s --wake-after 20s --duration 180s`
  Add `--request` on the first start so the Screen Recording prompt names the spike app. Without a grant host-run exits non-zero before serving.
- **(b) Only when R15 is routed to host-run:** `--dump-at 15s,150s`.
- **(c) Only when R16 is routed to host-run and the user replies type-ok** (ran in VM office on a 25 s run: one key line, codes [7]): `--allow-input --allow-keycodes 7 --type-at 160s --type-keycodes 7`. With `--duration 180s` the typing lands 20 s before the end.

Rehearsal step list (offsets from the moment the capture is running, which came 1.8 s after `open` in every run): chart settled and capture running 1.8 s; viewer joins about 5 s after open; dump 1 at +15 s; motion on +45 s, off +105 s; `pmset displaysleepnow` +120 s (stream lost within 0.3 s, display reads asleep at the next poll); wake +140 s (display reads awake at the next poll; no frames return); guest-only lock +150 s; guest-only typed unlock +160 s; end +180 s for (a). No flag needed changing during the rehearsal.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing critical] Per-window encode latency needed a reader in media_darwin.m**
- **Found during:** Task 1, designing the perf sampler
- **Issue:** `spk_sck_stats` reports mean, p95 and max over the whole 4096-sample ring (about 136 s at 30 fps), so the static and motion windows could not be told apart, and the plan's "from spk_sck_stats deltas" had no delta to read.
- **Fix:** `spk_sck_lat_since(since)` returns n, mean, p95, max and a `next` cursor for the samples after `since`; host-run calls it once per perf sample.
- **Files modified:** `spike/desktop/media_darwin.m`, `spike/desktop/bridge_darwin.go` (neither is in the plan's file list)
- **Commit:** `d594ce26`

**2. [Rule 1 - Bug] Chart panels overlapped the grid and the gray ramp**
- **Found during:** Task 2, VM smoke run screenshot
- **Issue:** the text panels were 330 pt tall for 250 pt of text, so the grid started inside the black panel and the ramp ran into the motion region; long lines spilled out of their panels.
- **Fix:** panel height 250, text clipped to its panel, ramp 400 wide, title under the grid.
- **Files modified:** `spike/desktop/hostrun_darwin.m`
- **Commit:** `fdddb351`

**3. [Rule 2 - Missing critical] host-run checks the Screen Recording grant before serving**
- **Found during:** Task 1
- **Issue:** the plan's `--request` flag only raises the prompt; without a grant the server would start and fail inside SCK with -3801 after the chart was up.
- **Fix:** after the optional request, `sr_preflight` is recorded once and host-run exits non-zero naming the fix when the grant is missing. This matters for the user-gated run, where the first start will hit it.
- **Files modified:** `spike/desktop/hostrun.go`
- **Commit:** `d594ce26`

### Plan adjustments

- **Encoder info sits in the first perf line, not the start event.** The encoder is created inside `runServe`, after `start` is written; the first perf line is written the moment the capture is running and carries `encoder`. r20-vm.txt says so.
- **Start event hides more than the token.** The plan said "flags without the token"; the bind address is redacted (`100.x.x.1`) and the evidence path is omitted, because the real run's scratch path holds the account name and the evidence gate forbids tailnet addresses.
- **Two rehearsal runs for Task 2.** Display sleep plus wake did not lock the guest even with `screenLock immediate`, so the first run had no lock or unlock. The committed run adds one Ctrl+Cmd+Q at about ready+150 s, labelled in the R18-vm row. The first run (and its screenshots) stay in the session scratchpad only.
- **Encoder not software.** The plan expected "software in the guest"; the guest's paravirtualized AVC encoder reads hardware true. R20-vm says so.
- **Extra runs.** The default command (a) and the typing additions (c) each ran once more in VM office, so the two command lines handed to 17-11 are the ones that were tested; the numbers in r20-vm.txt come only from the longer rehearsal.
- **New files outside the plan's list.** `spike/desktop/hostrun_test.go` (key gate, typing refusal, start-flag redaction, timeline keys, frame class) and the `control`, `peerState`, `tap` hooks in `server.go`.
- **TESTING.md not updated.** The standing rule asks for new test files in the Suite Manifest. `hostrun_test.go` is in the nested spike module, which the root suite never runs and 17-12 removes or archives; I did not add a manifest row for a throwaway test. If 17-12 keeps the spike's tests, it should add one.

**Total deviations:** 3 auto-fixed (2 Rule 2, 1 Rule 1), 6 plan adjustments. **Impact:** none on the plan's answers; the adjustments make the evidence cleaner or the run safer.

## Authentication Gates

None. Both guests were Running and Online on the tailnet (checked inside the guests); no tailnet skip was recorded; no re-key checkpoint was raised. The Spike.app grants survived the rebuild and swap (same signing identity), another R10 data point.

## Issues Encountered

- `pmset displaysleepnow` in the guest did not lock the session (see adjustments). The real Mac's immediate lock delay is expected to lock at the sleep (RESEARCH, verified there), which the host run will show.
- The static window yields almost no Complete frames after the first 15 s (Idle, then nothing), so R20's latency figure comes from the motion window only and the static window is scored by SSIM.
- `top`'s first sample in each file reads 0.0 for every process (it is a since-boot sample); the committed numbers use the third sample.

## Known Stubs

None.

## Threat Flags

None. The surface this plan adds is the one in its threat model: data-channel `wake`, `motion:*` and `key:<code>` messages (T-17-22, gated by `--allow-input` and the keycode list), captured frames (T-17-23, chart over the display, none committed), and the guest lock (T-17-38, restored to off; `sysadminctl -screenLock status` reads off and `pgrep -f Spike.app` prints nothing).

## Guest end state

Both VMs are running and on the tailnet (not stopped). VM office: screen lock off, no spike, top or helper process, `/tmp/spike-evidence` run directories and logs removed (older evidence from earlier plans left as it was), `~/ptest/Spike.app` is the new build with its grants. VM home: no spike process; Safari still holds tabs and history for the finished runs (the token in them is dead). The token and the office address were deleted from the session scratchpad.

## Next Phase Readiness

Ready for 17-11. It needs: the command lines above (also in the scratchpad file `17-10-for-17-11.txt`), the notes that notification banners can sit over the chart (turn Do Not Disturb on, read raw-1.png before committing anything), that the stream will most likely end at display sleep on the real Mac as in the VM and the user unlocks by hand, and that `first_frame_after_wake` may never appear (the missing line is the R18-host result, not a failure). FEAS-01 and FEAS-02 stay open: only 17-12 closes them. 17-12 still has the misplaced R15 to R17 rows (deferred-items.md) and the TESTING.md question for `hostrun_test.go`.

## Self-Check: PASSED

- Files exist: hostrun.go, hostrun_darwin.m, hostrun_test.go, r18-vm-timeline.jsonl, r20-vm.txt.
- Commits `d594ce26`, `fdddb351` and `54c4784a` are on the branch.
