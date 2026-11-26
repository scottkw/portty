---
phase: 19-unattended-desktop-viewing
plan: 09
subsystem: desktop-sharing
tags: [state-machine, watchdog, power-assertions, wake, lapse-notification, crash-loop-breaker, perm-probe]
status: complete

requires:
  - phase: 19-03
    provides: Manager, per-viewer tracks, fan-out, requestKeyframe
  - phase: 19-05
    provides: screencap SessionLocked, DisplayAsleep, OnACPower, Hold*Awake, DeclareRemoteUserActivity, NudgePointer, DisplaySource.HardwareEncoder
provides:
  - "Manager.Status() derives capture_paused > needs_permission > display_asleep > awaiting_approval > locked > streaming > idle; Locked is an attribute; every change is pushed to each viewer by a state loop (1 s with viewers, 10 s idle)"
  - "Manager.Locked(): 250 ms wall-clock cache, true on a read error (fail closed for Phase 20)"
  - "Prober / DefaultProber: `<exe> daemon perm-probe` through a runner (1 s cache, 3 s timeout, strict JSON, refuses under testing.Testing()); readApprovalStore via plutil"
  - "Stall watchdog (nudge, still-screen recheck, restart, classification) and the once-per-lapse notification with NoteGUISeen / ClaimLapse sender seam"
  - "desktop-state.json (0600, atomic, strict): approvalSeenAt, captureActive, abnormalExits; ApproachingLapse from day 27; breaker, ResetBreaker, NoteTestCapture"
  - "Viewer display + system assertions, DeclareUserActivity on every join, AC-only system assertion, restart after wake or -3815 once the display reads awake"
affects: [19-08, 19-11, 19-12, 19-14, 19-16, 20]

actuals:
  tokens: 27500
  tasks: 3
  commits: 6
plan_head_before: f8f4953b09919cdec0c652e1ac65d926a3beee79
plan_head_after: 8e86ce63d21bf8f21c76ee13801b962cf5b47379

tech-stack:
  added: []
  patterns:
    - "State loop advances every detector one step per tick; all elapsed-time checks compare Deps.Now().Round(0); timers only set cadence"
    - "One leaf mutex (smu) for derived-state flags; every transition is a method holding it, which is what makes concurrent detectors produce one LapseSeq"
    - "Fake outside world (fakes_test.go: world, fakeSource, sources) so watchdog, power and lapse tests run on a fake wall clock against the real viewer and pion path"

key-files:
  created:
    - internal/desktop/state.go
    - internal/desktop/state_test.go
    - internal/desktop/watchdog.go
    - internal/desktop/watchdog_test.go
    - internal/desktop/power.go
    - internal/desktop/power_test.go
    - internal/desktop/runtime_state.go
    - internal/desktop/permissions.go
    - internal/desktop/fakes_test.go
  modified:
    - internal/desktop/manager.go
    - internal/desktop/export_test.go
    - internal/desktop/fanout_test.go

key-decisions:
  - "The watchdog is a tick-driven state machine in the state loop, not a goroutine per source: one place to serialise (tickMu), no timers to fake, and TestWatchdog_* can step a fake clock"
  - "A viewer that joins while Screen Recording is denied or the display has no display stays connected and is told the state; the state loop retries the start (10 s) instead of closing the socket"
  - "A lapse ends only on a frame after the lapse began; during an awaiting retry cycle the restarted stream's own first frame is ignored (only a frame after a nudge counts), so SCK emitting a frame at stream start cannot flap awaiting_approval and re-notify"
  - "Without Accessibility restarts are rationed to one per 60 s: each restart yields a frame, which would otherwise reset the stall clock and restart a still screen every ~6 s"

requirements-completed: [DPRM-02, DPRM-03, DPRM-04]

coverage:
  - id: E1
    description: "A viewer reads state streaming/locked/display_asleep/streaming within 1.5 s of the fake lock and display changes, and Status() matches; precedence holds over all 32 flag combinations; Locked() reads once inside 250 ms and returns true on a read error"
    requirement: "DPRM-02"
    verification:
      - kind: integration
        ref: "internal/desktop/state_test.go#TestState_PushTracer"
        status: pass
      - kind: unit
        ref: "internal/desktop/state_test.go#TestState_Precedence"
        status: pass
      - kind: unit
        ref: "internal/desktop/state_test.go#TestLocked_CacheAndFailClosed"
        status: pass
    human_judgment: false
  - id: E2
    description: "perm-probe output is strict (unknown field, missing key, trailing data, empty are errors), cached 1 s, a failed spawn is an error; the idle probe runs at most every 5 minutes, SR false is needs_permission, a probe error keeps the previous answer"
    requirement: "DPRM-01"
    verification:
      - kind: unit
        ref: "internal/desktop/state_test.go#TestProber_StrictAndCached"
        status: pass
      - kind: integration
        ref: "internal/desktop/state_test.go#TestIdleProbe_NeedsPermission"
        status: pass
    human_judgment: false
  - id: E3
    description: "Stall watchdog: 5 s silence nudges; a frame within 1 s is a still screen and the next nudge waits 30 s; no frame restarts the stream with a keyframe first and recovers; -3801 gives needs_permission, -3815 gives display_asleep; awaiting_approval only after a restart plus a second nudge with no frame, one LapseSeq, one notification (exact id, title, body) across three 60 s retries and a recovery; never without Accessibility; never nudges without a viewer"
    requirement: "DPRM-03"
    verification:
      - kind: integration
        ref: "internal/desktop/watchdog_test.go#TestWatchdog_StillScreenNudges"
        status: pass
      - kind: integration
        ref: "internal/desktop/watchdog_test.go#TestWatchdog_FrozenRestartRecovers"
        status: pass
      - kind: integration
        ref: "internal/desktop/watchdog_test.go#TestWatchdog_RestartPermissionDenied"
        status: pass
      - kind: integration
        ref: "internal/desktop/watchdog_test.go#TestWatchdog_RestartNoDisplay"
        status: pass
      - kind: integration
        ref: "internal/desktop/watchdog_test.go#TestWatchdog_AwaitingApprovalOnce"
        status: pass
      - kind: integration
        ref: "internal/desktop/watchdog_test.go#TestWatchdog_NoAXNeverAwaiting"
        status: pass
      - kind: integration
        ref: "internal/desktop/watchdog_test.go#TestWatchdog_NoNudgeWithoutViewer"
        status: pass
    human_judgment: false
  - id: E4
    description: "One sender per lapse: the GUI's claim stops the daemon; a silent GUI lets the daemon post at 15 s (not before); no GUI posts at once; a barrier race between the daemon's claim and the GUI's gives exactly one sender over 15 rounds; stale, future and zero seq claims fail; two simultaneous detectors give one transition and one notification"
    requirement: "DPRM-03"
    verification:
      - kind: integration
        ref: "internal/desktop/watchdog_test.go#TestLapseSender_GUIClaims"
        status: pass
      - kind: integration
        ref: "internal/desktop/watchdog_test.go#TestLapseSender_GUISilentDaemonPosts"
        status: pass
      - kind: integration
        ref: "internal/desktop/watchdog_test.go#TestLapseSender_NoGUIDaemonPostsAtOnce"
        status: pass
      - kind: integration
        ref: "internal/desktop/watchdog_test.go#TestLapseSender_ClaimRace"
        status: pass
      - kind: integration
        ref: "internal/desktop/watchdog_test.go#TestWatchdog_ConcurrentDetectorsOneTransition"
        status: pass
    human_judgment: false
  - id: E5
    description: "Approval record: set at a first capture start, kept under 30 days, ApproachingLapse true on day 27 and 29, reset at a capture start on day 30; a readable store overrides, a store error changes nothing. Breaker: three abnormal exits in an hour pause capture (viewer told capture_paused, no source), old entries dropped, ResetBreaker resumes, clean Close clears captureActive. State file: 0600, atomic, symlink, loose mode, unknown schema and unknown field are errors that never rewrite the file; NoteTestCapture clears exits and applies the approval rule"
    requirement: "DPRM-03"
    verification:
      - kind: integration
        ref: "internal/desktop/watchdog_test.go#TestApprovalRecord"
        status: pass
      - kind: integration
        ref: "internal/desktop/watchdog_test.go#TestBreaker"
        status: pass
      - kind: unit
        ref: "internal/desktop/watchdog_test.go#TestRuntimeState_File"
        status: pass
      - kind: unit
        ref: "internal/desktop/watchdog_test.go#TestNoteTestCapture"
        status: pass
    human_judgment: false
  - id: E6
    description: "Power: viewer display+system assertions on first join, DeclareActivity on every join, released on last leave and on Close; AC assertion taken at Start on AC, released and re-taken on the 30 s recheck, nothing held on battery without a viewer; wake with a dark display waits (no restart, 30 s cap) and restarts once awake with a keyframe first; ErrNoDisplay shows display_asleep at once and a joiner gets no media; failed holds, lock read and probe are logged once and a Status.Reason while the picture keeps flowing; a software encoder is a diagnostic; a 2 hour clock jump is acted on at the next tick"
    requirement: "DPRM-04"
    verification:
      - kind: integration
        ref: "internal/desktop/power_test.go#TestPower_ViewerAssertions"
        status: pass
      - kind: integration
        ref: "internal/desktop/power_test.go#TestPower_ACOnly"
        status: pass
      - kind: integration
        ref: "internal/desktop/power_test.go#TestPower_WakeRestartsStream"
        status: pass
      - kind: integration
        ref: "internal/desktop/power_test.go#TestPower_NoDisplayStopThenRestart"
        status: pass
      - kind: integration
        ref: "internal/desktop/power_test.go#TestPower_HoldFailureContinues"
        status: pass
      - kind: integration
        ref: "internal/desktop/power_test.go#TestPower_WallClockOnly"
        status: pass
    human_judgment: false
  - id: E7
    description: "Real behaviour was NOT run: the real perm-probe child (the `daemon perm-probe` subcommand does not exist yet, 19-11 adds it), real IOPM assertions in pmset, the real nudge returning the pointer, the replayd-restart classification (V5), whether SCK emits a frame after a restart on a still screen, the real approval store read, kIOPMUserActiveRemote lighting a display (A12), the 30-day prompt (M-160) and the AC/battery behaviour on the office Mac"
    requirement: "DPRM-03"
    verification: []
    human_judgment: true
    rationale: "Host safety forbids real capture, TCC and power calls on the host; they belong to the VM rows (19-12) and the office-Mac M-rows (19-16)"

duration: 3h
completed: 2026-11-26
---

# Phase 19 Plan 09: Desktop state, stall watchdog and power Summary

**The desktop link now knows and pushes its real state (with the lock as an attribute and a fail-closed `Locked()`), tells a still screen from a frozen stream with a nudge-and-restart watchdog, notifies once per approval lapse through a claim-based GUI-or-daemon sender, holds the display and system awake for viewers and on AC, restarts after wake, and pauses itself after three crashes in an hour.**

## Accomplishments

- **State (DPRM-02):** `pickState` is one precedence function; `deriveState` reads flags kept under one leaf mutex. A state loop (cadence 1 s with viewers, 10 s idle, re-paced on every join and leave) reads the display and the lock, runs the idle probe, and pushes a state message only when `{state, viewers, locked}` changed. Failed reads are a `Status.Reason` and one log line each, never a stop.
- **Permissions (research C1):** `Prober` spawns `os.Executable() daemon perm-probe` (1 s cache, 3 s timeout, singleflight by mutex), decodes exactly `{"sr","ax","pe"}`, refuses under `testing.Testing()`. The idle probe is at most every 5 minutes with no viewer and never in-process. `readApprovalStore` parses `plutil -p` for this bundle's `LastAlerted`; any error is "signal unavailable".
- **Watchdog (DPRM-03):** nudge (only with a viewer and a fresh probe saying AX) at 5 s; a frame within 1 s is a still screen (recheck 30 s); no frame restarts the stream (new source, viewers re-gated to its keyframe) and classifies by the start error or, after a second nudge with no frame in 2 s, awaiting_approval. Awaiting approval is reachable only through restart plus nudge and never without Accessibility (logged once).
- **Lapse sender:** `enterAwaiting` is the single transition (takes the next `LapseSeq` once under the mutex). `ClaimLapse(seq, claimant)` is atomic; with a GUI seen in the last 12 s the daemon waits 15 s of wall clock for the GUI's claim, otherwise it claims and posts at once through `Deps.Notify` (outside any lock). Recovery and the approaching-lapse warning post nothing.
- **Runtime state:** `desktop-state.json` (0600, temp plus rename, `O_EXCL` temp, strict decode, unknown schema or loose mode is an error that disables persistence and never rewrites). Approval rule at every capture start; `ApproachingLapse` from day 27 to day 30 from the store when readable, else the own record. A `captureActive` left true at `NewManager` is one abnormal exit; three in an hour open the breaker (`capture_paused`, no source, viewers still told); `ResetBreaker` and `NoteTestCapture` clear it.
- **Power (DPRM-04):** viewer assertions on the 0 to 1 transition under `m.mu`, `DeclareActivity` on every join before the source starts, release on 1 to 0 and Close; AC system assertion at `Start` and re-read every 30 s; wake (wall-clock) or `-3815` restarts once `DisplayAsleep` reads false (30 s cap, then the watchdog is the fallback); the fan-out drops units while the display is dark so a joiner sees the state, not a stale cached frame.

## Task Commits

1. **Task 1 (tracer): derived state, lock read, prober, push** - `55849ae3` (feat)
2. **Task 2 RED: watchdog, lapse, approval, breaker and state-file tests** - `a8f34f91` (test; package did not build)
3. **Task 2 GREEN: watchdog, lapse sender, runtime state, breaker** - `d3ffda0e` (feat)
4. **Task 3 RED: power, wake and display tests** - `fe33a045` (test; assertions failed)
5. **Task 3 GREEN: assertions, AC, wake, display asleep** - `4aa72565` (feat)
6. **Fix: ration restarts of a still screen without Accessibility** - `8e86ce63` (fix)

## TDD Gate Compliance

Both TDD tasks have a `test(19-09)` commit before their `feat(19-09)` commit. Task 2's RED fails at build time (undefined `runtimeState`, `enterAwaiting`, `ClaimLapse`, ...). Task 3's RED builds and fails at assertions (no assertions taken, no restart after wake, no state for ErrNoDisplay, no hold-failure reasons); `TestPower_WallClockOnly` was green at RED because its second half asserts an absence. One honest note: the software-encoder diagnostic (`noteStartSuccess`) was written with Task 2's start-path rework, so its assertion inside `TestPower_HoldFailureContinues` was already satisfiable at the Task 3 RED; the hold-failure half of that test failed there.

## Deviations from Plan

**1. [Rule 1 - Bug] Restart storm on a still screen without Accessibility**
- **Found during:** Task 3 review of the AX-off path.
- **Issue:** with AX off the watchdog restarted after every 5 s of silence; a restart yields a frame that reset the stall clock, so a still screen restarted about every 6 s.
- **Fix:** one restart per `awaitingRetry` (60 s) on that path; `TestWatchdog_NoAXNeverAwaiting` now asserts a frame-then-30 s-of-silence does not restart.
- **Commit:** `8e86ce63`

**2. [Rule 2 - Missing critical behaviour] A joiner that cannot be served stays connected**
- The 19-01 join closed the socket "capture unavailable" on any start error. With `-3801`, `-3815` or a paused breaker the viewer would never see why. The viewer now joins, is told the state, and the state loop retries every 10 s (`retryStartEvery`, a constant the plan does not list). Other start errors still fail the join and release the assertions.

**3. [Plan interpretation] Watchdog as a tick-driven state machine**
- The plan says "one goroutine per running source, started by the fan-out". The machine runs inside the state loop (serialised by `tickMu`), one step per tick, so every check is a wall-clock comparison and tests step a fake clock. Behaviour and timings are the plan's.

**4. [Plan interpretation] A lapse ends on a frame after it began, not on any frame**
- The restarted stream may emit a frame on its own (open question below). During an awaiting retry cycle that frame is ignored: only a frame after a nudge ends the lapse, so awaiting cannot flap and re-notify. Outside a retry cycle, any frame after the lapse began ends it.

**5. [Rule 3 - Test infrastructure] Files outside the plan's list**
- `export_test.go` gets quiet package-level defaults for the lock, display, AC and holds (tests never read the host's lock or display, which would make existing loopback tests flaky); `fanout_test.go` records the viewer's `state` and `locked`; new `fakes_test.go` holds the fake world and source. All test-only.

**6. [Plan note] `go test ./internal/daemon/ -run 'TestDesktopAPI_|TestShareLifetime_'` matched no tests**
- 19-08 (daemon wiring) runs in parallel and its tests are not in this base, so that step proved nothing here. `go build ./internal/...` passes with the Manager changes. Re-run it after the merge.

**7. [Instruction conflict] Commit trailer**
- The shared instructions say `Claude Opus 5.5`; the dispatch said to use exactly `Claude Sonnet 5.5`. I used the dispatch's (the accurate model).

**Total deviations:** 7 (1 bug, 1 missing behaviour, 2 interpretation, 1 test infrastructure, 2 notes). None changes a contract 19-08, 19-11 or 19-14 builds on.

## Open Questions (for the VM rows and 19-16)

- **Does SCK emit a frame after a restart on a still screen?** If yes (likely), a still screen reads "recovered" at the first restart, which is correct (the stream is alive). It also means awaiting_approval is only reached when replayd gives nothing after a restart and a nudge. The `retry` rule above is what keeps the lapse from flapping if it does emit one. V5 and M-160 decide whether the classification catches the real prompt.
- **`perm-probe` subcommand:** `Prober` spawns `daemon perm-probe`, which 19-11 must add to `cmd_daemon.go`; until then every probe is an error (a `probe` reason, state unchanged).
- **`NoteTestCapture` and a live Manager:** it writes the file; the Manager picks up the approval date at its next refresh (5 minutes) and the daemon must also call `ResetBreaker` for the in-memory paused flag (19-11).

## Known Stubs

None. `Deps.Notify == nil` logs "lapse notification not wired" (19-11 wires the daemon's sender; 19-14 adds the GUI side that calls `NoteGUISeen` and `ClaimLapse`). Nothing added to `.planning/WINDOWS.md`.

## Threat Flags

None beyond the plan's register. T-19-31 (breaker, `TestBreaker`), T-19-32 (strict probe decode, 0600 regular state file, `TestProber_StrictAndCached`, `TestRuntimeState_File`), T-19-33 (capture and assertions only with a viewer, plus the user-approved AC assertion), T-19-34 (nudge only with a viewer, a stall and Accessibility; `Nudge` returns the pointer itself in 19-05) are implemented. The new `plutil` spawn and the `perm-probe` spawn run this executable or `/usr/bin/plutil` with fixed arguments.

## Verification

- `go vet`, `gofmt -l`, `golangci-lint run --new-from-rev=89d29266 ./internal/desktop/...`: clean.
- `go test -race -count=1 ./internal/desktop/ ./internal/screencap/`: pass. The watchdog, lapse, approval, breaker, state-file and power tests pass `-race -count=3`.
- `internal/desktop` statement coverage 88.5 % (target 80 %). 16 watchdog-group tests, 6 power tests, 3 tracer-task tests.
- `bash tests/check-brand.sh`: clean. TESTING.md not touched (19-16 owns the consolidated update); every new test file is under `internal/desktop/`.
- No real capture, TCC, IOPM, launchd or tailnet call was made on the host; every native call is a fake.

## Self-Check: PASSED

Created files exist (state.go, watchdog.go, power.go, runtime_state.go, permissions.go and their tests, fakes_test.go) and the six commits `55849ae3`, `a8f34f91`, `d3ffda0e`, `fe33a045`, `4aa72565`, `8e86ce63` are ancestors of HEAD.
