---
phase: 19-unattended-desktop-viewing
plan: 05
subsystem: desktop-sharing
tags: [screencapturekit, videotoolbox, h264, cgo, objc, iokit, tcc, power-assertion]

requires:
  - phase: 19-01
    provides: screencap.Source interface, AccessUnit, Permissions, the sentinel errors
provides:
  - screencap.DisplaySource (implements Source) for the main display, with HardwareEncoder and FramesSeen
  - screencap.CaptureStill (JPEG, quality 0.8, 15 s ceiling)
  - VideoToolbox H.264 encoder with forced keyframe from the cached frame and rebuild
  - SessionLocked, DisplayAsleep, OnACPower, HoldDisplayAwake, HoldSystemAwake, PowerAssertion.Release, DeclareRemoteUserActivity, NudgePointer
  - Preflight, confined to preflight_darwin.m
affects: [19-03, 19-09, 19-10, 19-12, 19-16]

actuals:
  tokens: 15900
  tasks: 3
  commits: 5
plan_head_before: 542c37c015c5a91ded4db4b3024652f9f9a2d682
plan_head_after: 95092a59

tech-stack:
  added: []
  patterns:
    - "ObjC bridge pattern: every entry point and callback in @try/@catch returning a code, Go callbacks recover, handles via runtime/cgo.Handle"
    - "Every real bridge refuses with ErrRefusedInTest under testing.Testing(); encoder runs on synthetic frames only"
    - "Live permission reads in one file, enforced by a repo-walk test"

key-files:
  created:
    - internal/screencap/frames.go
    - internal/screencap/frames_test.go
    - internal/screencap/media_darwin.m
    - internal/screencap/media_darwin.go
    - internal/screencap/sys_darwin.m
    - internal/screencap/sys_darwin.go
    - internal/screencap/preflight_darwin.m
    - internal/screencap/preflight_darwin.go
    - internal/screencap/screencap_darwin_test.go
  modified: []

key-decisions:
  - "One capture stream per process: the ObjC stream state is file-level, DisplaySource.mu serialises Start, Stop, Rebuild and ForceKeyframe"
  - "Full Frames queue drops the newest delta and lets a new keyframe evict the oldest unit (a channel cannot drop the oldest delta and keep order); commented as a shortcut"
  - "Rebuild rebuilds the encoder first and then resizes the capture, so no new-size frame meets the old session"
  - "AccessUnit.Duration is the wall time since the previous unit (frames arrive only on change), first unit 1/30 s"
  - "No idle keepalive re-encode: a still screen yields no frames and 19-09's watchdog is designed around that"
  - "DeclareRemoteUserActivity releases its returned assertion at once, as the plan says; the held display assertion is the persistent guard"

requirements-completed: [DESK-01, DPRM-04, DPRM-02, DPRM-01]

coverage:
  - id: E1
    description: "60 synthetic NV12 frames at 1280x800 go through the real VideoToolbox encoder and return as Annex-B units: first is SPS+PPS+IDR at 1280x800, a forced keyframe re-encodes the cached frame, Rebuild(640,400) gives a keyframe with SPS+PPS at 640x400, later units stay 640x400"
    requirement: "DESK-01"
    verification:
      - kind: integration
        ref: "internal/screencap/screencap_darwin_test.go#TestEncoder_SyntheticFramesTracer"
        status: pass
    human_judgment: false
  - id: E2
    description: "captureSize gives 2560x1664 for a 1470x956-point display at scale 2, caps at 2560 with even sides, never upscales; avccToAnnexB round-trips, rejects truncated and zero lengths, emits parameter sets only on keyframes"
    requirement: "DESK-01"
    verification:
      - kind: unit
        ref: "internal/screencap/frames_test.go#TestCaptureSize"
        status: pass
      - kind: unit
        ref: "internal/screencap/frames_test.go#TestAvccToAnnexB"
        status: pass
    human_judgment: false
  - id: E3
    description: "Screen capture and the JPEG still refuse with ErrRefusedInTest under a test binary and open no channel; only the main display can be selected; -3801 and -3815 map to ErrPermissionDenied and ErrNoDisplay"
    requirement: "DPRM-01"
    verification:
      - kind: unit
        ref: "internal/screencap/screencap_darwin_test.go#TestDisplaySource_RefusesUnderTest"
        status: pass
      - kind: unit
        ref: "internal/screencap/screencap_darwin_test.go#TestCaptureStill_RefusesUnderTest"
        status: pass
      - kind: unit
        ref: "internal/screencap/screencap_darwin_test.go#TestDisplaySource_SelectSurface"
        status: pass
      - kind: unit
        ref: "internal/screencap/screencap_darwin_test.go#TestMapSCKError"
        status: pass
    human_judgment: false
  - id: E4
    description: "Assertions, remote activity, the nudge and the preflight refuse under test; the lock, display and AC reads return without error; the nudge direction rule picks away from the right edge"
    requirement: "DPRM-04"
    verification:
      - kind: unit
        ref: "internal/screencap/screencap_darwin_test.go#TestSysBridges_RefuseUnderTest"
        status: pass
      - kind: unit
        ref: "internal/screencap/screencap_darwin_test.go#TestNudgeDirection"
        status: pass
    human_judgment: false
  - id: E5
    description: "The three non-prompting permission reads appear only in preflight_darwin.m and no prompting variant exists anywhere in the repo"
    requirement: "DPRM-01"
    verification:
      - kind: unit
        ref: "internal/screencap/screencap_darwin_test.go#TestPreflightSymbols_OnlyInPreflightFile"
        status: pass
    human_judgment: false
  - id: E6
    description: "Real ScreenCaptureKit capture at Retina size into the encoder, the real -3801/-3815 codes, the JPEG still content, the hardware readback under capture, the lock and display reads on a real session, power assertions visible in pmset, the pointer nudge returning, and kIOPMUserActiveRemote lighting a display (A12) were NOT run: host safety forbids it"
    requirement: "DESK-01"
    verification: []
    human_judgment: true
    rationale: "Real capture, TCC and display-power behaviour run only in the Tart VMs (19-12) and on the office Mac (19-16 M-row); this plan proves compile, link, refusal in tests and the synthetic encoder path only"

duration: 40min
completed: 2026-11-26
status: complete
---

# Phase 19 Plan 05: Native Capture Bridges Summary

**ScreenCaptureKit main-display capture sized for Retina into a VideoToolbox Constrained Baseline encoder (forced keyframe from the cached frame, rebuild, hardware readback), a JPEG still, IOKit/CoreGraphics lock, display, power and returning-nudge bridges, and a single-file permission preflight, all refusing under `go test`.**

## Performance

- **Duration:** about 40 min
- **Tasks:** 3 (a tracer, two TDD tasks)
- **Files:** 9 created in `internal/screencap`, none modified
- **Commits:** 5 (2 feat, 2 RED test, 1 tracer feat)

## Accomplishments

- **Tracer (seam 9, real encoder):** synthetic NV12 frames go through a real `VTCompressionSession` and return to Go as Annex-B units. On this Mac the readback was `hardware`. 15 consecutive `-race` runs passed, and it ran as PASS, not SKIP.
- **Encoder:** H.264 Constrained Baseline AutoLevel, realtime, no reordering, 8 Mbps average, `MaxKeyFrameIntervalDuration` 4 s, hardware preferred and read back after every session creation and rebuild (`UsingHardwareAcceleratedVideoEncoder`). `DisplaySource.HardwareEncoder()` reports it; a software session is logged as `screencap: encoder using software`.
- **Capture:** main display via `CGMainDisplayID`, points times `pointPixelScale`, cap 2560 long edge with even sides (same math in ObjC and `captureSize`), 30 fps, queue depth 5, cursor shown, NV12, only `Complete` frames encoded; `FrameSeen` callback per Complete frame (`DisplaySource.FramesSeen()`). `didStopWithError` and every start failure pass their code to the Go error path.
- **Typed errors:** -3801 is `ErrPermissionDenied`, -3815 is `ErrNoDisplay`, -12908 is the internal `errNoEncoder`, anything else names its code.
- **Crash containment (T-19-18):** `@try/@catch` in every ObjC entry point and callback (26 sites across the three `.m` files), nil-checked dictionary literals via `pcDict1`, `recover` in every exported Go callback.
- **Still:** `CaptureStill` via `SCScreenshotManager` and ImageIO JPEG at quality 0.8, 15 s ceiling, buffer freed by the Go side.
- **System bridges:** lock (`CGSSessionScreenIsLocked`, NULL session is an error), display asleep, AC power, display and system assertions, `kIOPMUserActiveRemote`, and `NudgePointer` through a private event source that moves one point away from the right edge and returns, reporting whether both reads matched.
- **Refusal (T-19-19):** capture, still, assertions, activity, nudge and preflight return `ErrRefusedInTest` under `testing.Testing()`.
- **Preflight isolation (T-19-20):** `preflight_darwin.m` is the only file with the three non-prompting reads; `TestPreflightSymbols_OnlyInPreflightFile` walks the repo and also forbids `CGRequestScreenCaptureAccess`, `CGRequestPostEventAccess` and the Accessibility prompt option anywhere.

## Task Commits

1. **Task 1 (tracer): synthetic frames through the real encoder** - `2b46242c` (feat)
2. **Task 2 RED: DisplaySource refusal, surface, error mapping** - `7a638c9a` (test)
3. **Task 2 GREEN: capture, typed errors, JPEG still** - `befdcb2f` (feat)
4. **Task 3 RED: system bridges, nudge direction, preflight confinement** - `f06246c0` (test)
5. **Task 3 GREEN: system bridges and isolated preflight** - `95092a59` (feat)

## TDD Gate Compliance

Tasks 2 and 3 carry `tdd="true"`. Each has a `test(19-05)` commit before its `feat(19-05)` commit. A package that does not compile is not a valid RED, so each RED commit holds the tests plus a compiling stub that returns zero values; the target tests then failed on their own assertions:

- Task 2 RED: `TestDisplaySource_RefusesUnderTest`, `TestCaptureStill_RefusesUnderTest`, `TestDisplaySource_SelectSurface`, `TestMapSCKError` failed ("Start under test = nil, want ErrRefusedInTest", "-3801 = nil, want ErrPermissionDenied", ...). The RED stub is replaced in the GREEN commit.
- Task 3 RED: `TestSysBridges_RefuseUnderTest`, `TestNudgeDirection`, `TestPreflightSymbols_OnlyInPreflightFile` failed ("HoldDisplayAwake = nil, want ErrRefusedInTest", "nudgeDelta(735) = 0, want 1", "CGPreflightScreenCaptureAccess found in [], want only internal/screencap/preflight_darwin.m").

I did not run the GSD `tdd-red-evidence` classifier, since this is a task-level flag in an `execute` plan, not a `type: tdd` plan. Task 1 is a tracer, so its tests were written with the code and passed at first run; mutation checks were not done.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Design] "Drop the oldest delta" is not expressible on a channel without reordering**
- **Found during:** Task 2
- **Issue:** The plan says to drop the oldest delta when the Frames channel is full and never a keyframe. Popping an old keyframe and re-queueing it puts it behind later deltas.
- **Fix:** A full queue drops the new delta, and a new keyframe evicts the oldest unit. Order is kept and the newest keyframe is never dropped. Commented as a `shortcut:` in `DisplaySource.emit`; `TestDisplaySource_EmitWhenFull` pins it.
- **Files:** `internal/screencap/media_darwin.go`
- **Commit:** `befdcb2f`

**2. [Rule 3 - Plan text] `encodeSyntheticForTest` cannot return the hardware readback through its fixed signature**
- **Issue:** The test must log which encoder it used, but the plan's signature returns only units and an error.
- **Fix:** Signature kept; the readback lands in the package-level `syntheticHardware` atomic, read by the test. It is test-only and unexported.
- **Commit:** `2b46242c`

**3. [Rule 3 - Tooling] Task 3 verify command not run as written**
- The plan's golangci-lint line (`--new-from-rev=89d29266`) ran clean (0 issues) on `./internal/screencap/...`. `bash tests/check-brand.sh` passed.

**Total deviations:** 2 auto-fixed (1 design, 1 plan-signature). **Impact:** none on the contract 19-03, 19-09 and 19-10 build on.

## Issues Encountered

- None blocking. The Task 1 precondition (Phase 18.1 landed) was met: `LSMinimumSystemVersion` is 14.0 and `build.sh` carries `-mmacosx-version-min=14.0`.
- `go test` on the host builds without a deployment-target flag, so the host build links against the current SDK. The real macOS 14 floor comes from `build.sh`; the SCK and VideoToolbox APIs used all exist on 14.0 (`SCScreenshotManager` is 14.0+). Not built here with the 14.0 flag.

## Known Stubs

None. Not stubs, but not exercised: `pc_stream_start`, `pc_stream_stop`, `pc_encoder_rebuild` against a live stream, `pc_still_capture`, `DisplaySource.Start/Stop/Rebuild/ForceKeyframe/HardwareEncoder` and the system reads against real power and session state. They compile, link and are covered by the refusal tests only, because host safety forbids running them. The VM plan 19-12 and the 19-16 M-rows own the real runs.

## Hand-off notes

- **19-09:** `HardwareEncoder() (hardware, known bool)` is read after Start and after every Rebuild; show `hardware == false && known` as Status reason "software encoder". `FramesSeen()` counts Complete frames since Start (an extra, not in the plan's artifact list). `Errors()` delivers mapped errors but drops them if nobody reads, so drain it. `ForceKeyframe()` errors also arrive on `Errors()`. Use `HoldSystemAwake("Portty always-available desktop on AC power")` and `HoldDisplayAwake`/`HoldSystemAwake("Portty desktop viewer connected")` as the plan names them.
- **19-12 (VM):** check that `updateConfiguration` after `Rebuild` actually produces a frame on a still screen; if SCK emits none, the viewer waits until the screen changes after a rebuild (the encoder is rebuilt first and holds `forceNext`). The first IDR after Rebuild depends on that frame. Also verify the 420v pixel format is accepted, that `DeclareRemoteUserActivity` plus a held assertion behaves (A12), and the nudge returns on a real display.
- **19-16 (TESTING.md):** the symbol-confinement test walks `.go .m .h .c .swift .ts .tsx .js .sh .md .yml .yaml` files outside `.git`, `node_modules`, `spike`, `.claude`, `.planning`, `frontend/dist` and skips `*_test.go`. TESTING.md and any docs must not name `CGPreflightScreenCaptureAccess`, `AXIsProcessTrusted`, `CGPreflightPostEventAccess` or the prompting variants. Add `internal/screencap/*_test.go` rows to Section 4 for DESK-01, DPRM-01, DPRM-02 and DPRM-04. I did not touch TESTING.md.
- Capture shares the process-wide ObjC stream state: only one `DisplaySource` may run at a time.

## Threat Flags

None beyond the plan's register. T-19-18 (exceptions and nil literals), T-19-19 (tests capturing the host) and T-19-20 (cached permission answers, prompting APIs) are implemented and tested as above. T-19-SC: no package added, system frameworks only.

## Verification

- `GOTOOLCHAIN=go1.26.11 go test -race -count=1 ./internal/screencap/` passes; `TestEncoder_SyntheticFramesTracer` PASS (not SKIP), hardware readback on this Mac. Also `-count=15` on the tracer and `-count=3` with `./internal/desktop/`.
- `go vet`, `gofmt -l` clean; `go build ./...` ok; `golangci-lint --new-from-rev=89d29266 ./internal/screencap/...` 0 issues; `bash tests/check-brand.sh` OK.
- Acceptance greps: `kVTProfileLevel_H264_ConstrainedBaseline_AutoLevel` 1, `@try` 8 in `media_darwin.m`, `framework ScreenCaptureKit` 1, `UsingHardwareAcceleratedVideoEncoder` 1, `mmacosx-version-min` 0 in `media_darwin.go`, `pointPixelScale` 2, `showsCursor = YES` 1, `testing.Testing()` 3, `kIOPMUserActiveRemote` 1, `CGSSessionScreenIsLocked` 1, `CGPreflightScreenCaptureAccess` 1 in `preflight_darwin.m`.
- Host safety: no capture, SCK, TCC, assertion, nudge or permission read ran on the host; only the VideoToolbox encoder on synthetic frames and the three read-only state reads (lock, display, AC) did.

## Self-Check: PASSED

- Files exist: frames.go, frames_test.go, media_darwin.go, media_darwin.m, sys_darwin.go, sys_darwin.m, preflight_darwin.go, preflight_darwin.m, screencap_darwin_test.go.
- Commits `2b46242c`, `7a638c9a`, `befdcb2f`, `f06246c0`, `95092a59` are ancestors of HEAD.
