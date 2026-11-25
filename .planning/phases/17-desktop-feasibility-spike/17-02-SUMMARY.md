---
phase: 17-desktop-feasibility-spike
plan: 02
subsystem: desktop-spike
tags: [go, cgo, objc, screencapturekit, videotoolbox, smappservice, tcc, codesign, feas-01, feas-02]

requires:
  - phase: 17-desktop-feasibility-spike
    provides: "17-01 module, cgo bridge, startSource seam, encoder info keys, bind guard and token gate"
provides:
  - "ScreenCaptureKit source (serve --source sck) with per-status counts, IOSurface check, encode latency, cached-frame refresh and PNG dump"
  - "vmprobe, the R00 probe, with a host-safe --encoders-only mode"
  - "build/Spike.app: one Mach-O as app and daemon plus a signed copy as the SMAppService helper, signed inside-out with the Developer ID identity of team S2K7P43927"
  - "permission-probe kit: probe, probe-loop, agent, spawn, register, unregister, status, swap, all writing one pinned JSON shape"
affects: [17-03, 17-04, 17-05, 17-06, 17-07, 17-08, 17-09, 17-10, 17-11, 17-12]

actuals:
  tokens: 17534
  tasks: 3
  commits: 3
plan_head_before: 1599a1b77430774327d94b1eea8f4af0bc33c183
plan_head_after: d18c7746dbc8101955ba9a514fe8787e27449fca

tech-stack:
  added: [golang.org/x/sys v0.41.0 as a direct requirement (already in go.sum)]
  patterns: ["one probe function (Probe) behind every launch path so records are comparable", "a step that did not run is omitted, a step that failed holds {error:{domain,code,message}}", "launchd plists stay static; agent --config <json> picks the subcommand so later plans change behaviour without a rebuild"]

key-files:
  created:
    - spike/desktop/sys_darwin.m
    - spike/desktop/probe.go
    - spike/desktop/bundle/Info.plist
    - spike/desktop/bundle/entitlements.plist
    - spike/desktop/bundle/app.portty.spike.helper.plist
    - spike/desktop/bundle/app.portty.spike.daemon.plist
    - spike/desktop/bundle/build.sh
  modified:
    - spike/desktop/media_darwin.m
    - spike/desktop/bridge_darwin.go
    - spike/desktop/main.go
    - spike/desktop/go.mod

key-decisions:
  - "spk_sck_start sizes and (re)creates the VideoToolbox session at the real capture size, so server.go is untouched: serve builds a 16:10 session first and the SCK start replaces it when the display differs"
  - "forceKeyframe now calls spk_request_refresh, which re-sends the cached picture as an IDR while a stream runs and falls back to the plain force otherwise; this replaces the plain force on the RTCP path and on viewer join without editing server.go"
  - "A shared stream in probe-loop --stream starts once and is never restarted: a failed or stopped stream is recorded each interval, because retrying SCShareableContent every interval can raise a prompt per call on macOS 27"
  - "The post-event request's own answer is not a pinned key, so --request calls it before the preflight and the pinned post_event_preflight reports the state it left"
  - "A bare `spike` (no arguments) prints usage and exits 0, because the plan's own verify pipes it into grep under pipefail"

patterns-established:
  - "Host-safe modes are enforced by refusal: probe --no-tcc rejects --request, --ax-move, --png and --preflight-only with exit 2"
  - "cgo string results are JSON parked in a retained NSData and copied by Go at once; one caller at a time"

requirements-completed: [FEAS-01, FEAS-02]

coverage:
  - id: D1
    description: "vmprobe --encoders-only lists the VideoToolbox encoders, reads back hardware use, and writes 60 synthetic frames that ffprobe reports as Constrained Baseline; no SCK or TCC call"
    requirement: "FEAS-01"
    verification:
      - kind: integration
        ref: "spike/desktop/build/spike vmprobe --encoders-only --out spike/desktop/build/enc, then jq and ffprobe (Task 1 verify block)"
        status: pass
    human_judgment: false
  - id: D2
    description: "build.sh produces Spike.app (app.portty.spike, helper app.portty.spike.helper, SMAppService plist inside), signed inside-out with team S2K7P43927, hardened runtime, no deep-signing flag; refuses ad-hoc and any other team; codesign --verify --strict passes"
    requirement: "FEAS-02"
    verification:
      - kind: integration
        ref: "bash spike/desktop/bundle/build.sh, codesign --verify --strict, codesign -dv greps (Task 2 verify block); SPIKE_SIGN_ID=- and a foreign team both exit 2"
        status: pass
    human_judgment: false
  - id: D3
    description: "probe --no-tcc and status run on this host with no TCC call and write every pinned key (label, time, pid, ppid, pgid, sid, executable, macos, managername, session; status reads NotFound); agent, spawn (Setsid, ppid 1) and swap behave"
    requirement: "FEAS-02"
    verification:
      - kind: integration
        ref: "Task 3 verify block (jq on selfcheck.json and the status JSON); manual agent, spawn --quit-after and swap runs on the host"
        status: pass
    human_judgment: false
  - id: D4
    description: "ScreenCaptureKit capture returns Complete frames with an IOSurface, a decodable H.264 stream and a PNG, and SCError codes are recorded verbatim"
    requirement: "FEAS-01"
    verification: []
    human_judgment: true
    rationale: "The capture path compiles, links and is lint-clean but has never run: the plan forbids any SCK or TCC call on this host. R00 in plan 17-03 is its first real run, inside the Tart guest"
  - id: D5
    description: "register, unregister, probe-loop, probe --preflight-only, the AX/CGEvent calls and the --stream deltas record the pinned keys and attribute grants correctly"
    requirement: "FEAS-02"
    verification: []
    human_judgment: true
    rationale: "These paths touch TCC, Login Items or CGEvent and are VM-only by the plan's host-safety rule; only their argument validation and build were exercised here"

duration: 14min
completed: 2026-11-24
status: complete
---

# Phase 17 Plan 02: Capture, R00 Probe, Signed Spike.app and Probe Kit Summary

**ScreenCaptureKit capture with cached-frame refresh, the `vmprobe` R00 probe, and a Developer ID signed `Spike.app` (app, helper and legacy-daemon shapes from one Mach-O) with the probe subcommands every launch path runs, all proven host-safe and none run against TCC.**

## Performance

- **Duration:** 14 min
- **Started:** 2026-11-25T02:32:30Z (about 20:32 local)
- **Completed:** 2026-11-25T02:46:00Z
- **Tasks:** 3
- **Files:** 7 created, 4 modified

## Accomplishments

- `media_darwin.m` gained the ScreenCaptureKit source: main display, 420v with a recorded BGRA retry, long edge capped at 2560, 30 fps, queue depth 5, BT.709. Frames are counted per `SCFrameStatus`, the IOSurface is checked, and encode latency (display time to the VideoToolbox callback, converted with `mach_timebase_info`) feeds a mean, p95 and max over the newest 4096 samples. Every SCError code is kept verbatim in the `error` object.
- Cached-frame refresh: the newest Complete buffer is retained, and `spk_request_refresh` re-encodes it as an IDR on PLI, FIR and viewer join, plus from a timer after 2 s with no Complete frame (RESEARCH Pitfall 3, row R22).
- `vmprobe` runs every step even after a failure, writes `r00-vmprobe.json` regardless, and exits 1 when any step failed. Its `--encoders-only` mode ran here: 28 encoders listed, the H.264 hardware encoder (`com.apple.videotoolbox.videoencoder.ave.avc`) read back as hardware, 60 access units, ffprobe reports Constrained Baseline.
- `Spike.app` is assembled and signed inside-out (helper with `--identifier app.portty.spike.helper`, then the app with the two network entitlements), hardened runtime, no deep-signing flag. `codesign --verify --strict` passes; app is `app.portty.spike`, team S2K7P43927, flags `0x10000(runtime)`. The first signing finished with no Keychain dialog, so the A12 gate never fired.
- One `Probe` function backs `probe`, `probe-loop`, `agent`, `spawn`, `register`, `unregister`, `status` and `swap`. `probe --preflight-only` makes only the three non-prompting calls; `status` reads SMAppService with no TCC call.
- Host findings worth carrying forward: the host session was locked while the self-check ran, and the CGSession dictionary carried `CGSSessionScreenIsLocked: true` with `CGSSessionScreenLockedTime`, which confirms the key's presence while locked for rows R15 and R17. `status` read `NotFound` for the helper plist when run from the build directory.

## Task Commits

1. **Task 1: capture, cached-frame refresh, vmprobe** - `99b4cf26` (feat)
2. **Task 2: Spike.app bundle and inside-out signing** - `b2363091` (feat)
3. **Task 3: permission-probe kit** - `d18c7746` (feat)

**Plan metadata:** committed with this summary (docs).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocker] go.mod promoted golang.org/x/sys to a direct requirement**
- **Found during:** Task 1
- **Issue:** probe.go imports `golang.org/x/sys/unix` directly (Getsid, SysctlUint32, RenamexNp), but go.mod listed it `// indirect`. The build works either way; `go mod tidy` rewrites the line and the module file would otherwise misdescribe itself (the same fix 17-01 made for pion).
- **Fix:** `go mod tidy`; the only change is dropping the `// indirect` marker on the existing v0.41.0 line.
- **Files modified:** spike/desktop/go.mod (not in the plan's `files_modified`, the one path outside it)
- **Commit:** 99b4cf26

**2. [Rule 1 - Bug] The plan's usage check cannot pass if a bare `spike` exits 2**
- **Found during:** Task 3
- **Issue:** the Task 3 verify runs `spike 2>&1 | grep -q -- "$c"` under `set -euo pipefail`, so any non-zero exit from the left side fails the pipeline even when grep matches. 17-01's `run` returned 2 for no arguments.
- **Fix:** no arguments, `help`, `-h` and `--help` print usage and exit 0; an unknown command still exits 2.
- **Files modified:** spike/desktop/main.go
- **Commit:** d18c7746

**3. [Rule 2 - Missing critical] probe --no-tcc refuses every flag that would call TCC**
- **Found during:** Task 3
- **Issue:** the threat model (T-17-07) relies on `--no-tcc` being host-safe, but nothing stopped `probe --no-tcc --request` from running a request API.
- **Fix:** `--no-tcc` combined with `--request`, `--ax-move`, `--png` or `--preflight-only` exits 2 before any call; likewise `--preflight-only` with `--request`, `--ax-move` or `--png`.
- **Files modified:** spike/desktop/probe.go
- **Commit:** d18c7746

**Interface notes (not rule fixes):**
- `serve --source sck` works without editing server.go: `spk_sck_start` recreates the encoder session at the capture size, and `forceKeyframe` now calls `spk_request_refresh`.
- Extra keys beyond the pinned set exist where they cost nothing and help evidence: `capture` also carries `unknown`, `ioSurfaceFrames`, `mainDisplayMatched`, `fallbackReason`, `latestDisplayTime` and `withAudio`; `displays` entries also carry `pointWidth`, `pointHeight` and `main`; each `encoders` entry also carries `codec`; `h264` is `{path, bytes, accessUnits, keyframes}`. Gates read only the pinned names.
- `spk_user_activity` and `spk_display_awake` are built and linked (C prototypes in bridge_darwin.go) but have no Go caller yet; the host-run and sleep/wake plans add their wrappers.

**Total deviations:** 3 auto-fixed (1 blocker, 1 bug, 1 missing critical). **Impact:** none on scope; one extra path (go.mod) in the diff.

## Authentication Gates

None. The Developer ID key signed without a Keychain dialog on the first run, so no checkpoint was needed.

## Issues Encountered

- **SCK has never run.** By the plan's host-safety rule, `vmprobe` without `--encoders-only`, `serve --source sck`, `probe` without `--no-tcc`, `probe-loop`, `register` and `unregister` were built and lint-checked only. Likely first-run surprises in 17-03: SCK in a process with no `NSApplication`, a `-3801` or `-3811` from the guest, and `SCShareableContent` blocking on a permission prompt (the call times out after 15 s and records `spike.sck -1`).
- **Evidence hygiene.** The CGSession dictionary carries the user's long and short name and the session UUID. `build/selfcheck.json` is git-ignored and was not committed; any `session` object copied into tracked evidence must be redacted.

## Known Stubs

- `withAudio` is accepted by `spk_sck_start`, stored and reported as `withAudio` in `capture`, but no audio output is added. Intentional: plan 17-09 wires system-audio capture. Not a goal blocker for this plan.

## Threat Flags

None. `agent --config`, the `/tmp` spawn log and the TCC request calls are covered by T-17-30 and T-17-07; the signing refusals implement T-17-06 and the distinct bundle ids T-17-08.

## Next Phase Readiness

Ready for 17-03 (R00 in the guest). Needs from the user at that point: the VMs booted and re-keyed per the phase's user checkpoint. `spike/desktop/build/Spike.app` is built and signed; copy it into the guest over tar and launch with `open`, never by executing the binary from a shell.

## Self-Check: PASSED

- Files: sys_darwin.m, probe.go, bundle/Info.plist, entitlements.plist, app.portty.spike.helper.plist, app.portty.spike.daemon.plist, build.sh all exist; build/Spike.app and build/app.portty.spike.daemon.plist.template exist (git-ignored).
- Commits: 99b4cf26, b2363091 and d18c7746 are ancestors of HEAD; `git rev-list --count` from the recorded base is 3.
- All three task verify blocks passed; every acceptance grep passed; `go list ./...` at the repo root lists no spike package; the release tests and brand gate pass; `git diff --name-only HEAD -- build/ .github/` is empty.
