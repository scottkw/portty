---
phase: 20-remote-control
plan: 02
subsystem: desktop
tags: [input, keys, gates, dead-man, control-state, accessibility, cgevent]
requires:
  - phase: 20-01
    provides: "InputBridge posting, key table, handleInput entry point, input goroutine, v1 wire contract"
  - phase: 19
    provides: "Manager.Locked(), Deps.Nudge (NudgePointer), fake clock harness"
provides:
  - "keys, text, wheel, relative moves, drags and click counts posted through the bridge"
  - "held sets (buttons and keys) with every release trigger including the dead-man"
  - "pressAllowed: the one gate for presses (rights, lock incl. optimistic, epoch, control state, rate)"
  - "control state measured by effect, {t:control} and {t:cursor} messages, Manager.ControlState()"
  - "tagged watchdog nudge events"
affects: [20-03, 20-04, 20-05, 20-06, 21, 22]
tech-stack:
  added: []
  patterns:
    - "gates apply to presses only; releases of held input always post"
    - "periodic checks (dead-man, lock, recovery nudge) run on the input goroutine, the only bridge caller"
    - "elapsed-time decisions read Deps.Now; timers only set cadence"
key-files:
  created:
    - internal/desktop/gates.go
    - internal/desktop/input_map.go
    - internal/desktop/input_map_test.go
    - internal/desktop/control.go
    - internal/desktop/control_test.go
    - internal/desktop/control_golden_test.go
    - internal/desktop/export_input_test.go
  modified:
    - internal/desktop/input.go
    - internal/desktop/input_test.go
    - internal/screencap/sys_darwin.m
key-decisions:
  - "Lock and dead-man releases keep the viewer's input life (no closed flag): only leave, channel close and Close end it, so a viewer that blurs, idles or sees a lock can input again"
  - "The release message releases everything held but does not end the viewer's input life (the plan text said ReleaseViewer, which would disable input after every window blur)"
  - "Drop counters keep their 20-01 plain names (locked, stale_epoch, rate, no_right, no_accessibility), not a drop. prefix: 20-01 tests and 20-03 already use the plain names"
  - "Printable keys never fall back to the typed character (flagged planner decision kept); k is carried and validated, unused"
requirements-completed: [DCTL-01, DCTL-02, DCTL-03]
status: complete
duration: ~1h30m
completed: 2026-11-26
estimate:
  tokens: 95000
  raw_tokens: 95000
  tasks: 3
  confidence: low
actuals:
  tokens: 15700
  tasks: 3
  commits: 3
plan_head_before: 6662f19644b8914f539b0952f62f526d6d7e3711
plan_head_after: 1ef6a75d25ab35d59de4b54d4aa9ddda7fee4270
---

# Phase 20 Plan 02: Keys, gates, release triggers and control state Summary

A viewer's keys, text, wheel and relative moves now reach the Mac through one gate that only presses pass, nothing it holds can stay held, and "control needs Accessibility" is a measured, self-recovering state pushed to viewers and exposed to the host GUI.

## What was built

- **Task 1 (tracer, 88f5ffab):** `InputBridge` gained `PostKey/PostText/PostScroll`. `key` maps through `screencap.KeyCode` (no entry: dropped `unknown_code`), flags are the viewer's held modifiers plus alpha-shift when caps; a repeat of a key not held is dropped (`not_held`) so a repeat can never press what a release let go of. `text` posts `UTF16Chunks(.., 20)`. Held keys are released non-modifiers first, modifiers last, then buttons (`takeHeldLocked`). `gates.go` was created here with rights, lock and epoch.
- **Task 2 (30f659d8):** `pressAllowed` order: `no_right`, `locked` (Locked() or the optimistic lock), `stale_epoch` (positional kinds only), `rate` (token bucket 400/s, burst 200, spent last so only otherwise-allowed presses pay). `input_map.go`: `mapPoint` (the old `inputTarget.point`, moved), `nextClickCount`, `scrollDelta`, `integrateRelative` and the constants. Click state from viewer timestamps (a missing timestamp or a backwards one starts again at 1), drag while a button is held, mouse flags from held modifiers, wheel at the current cursor, rmove from the viewer's last point (or the real cursor), modifier reconcile before any key down, optimistic lock on Ctrl+Cmd+Q (3 s), input ticker every 250 ms (release-all on lock, dead-man at 3 s by Deps.Now), one `desktop: input summary` log per viewer at leave (kinds and reasons only, 8-char ID prefix).
- **Task 3 (1ef6a75d):** `control.go`: state behind a leaf mutex, `checkMoveEffect` 60 ms after a plain move (skipped when a newer pointer post exists), 3 misses set `needs_accessibility`, a hit clears it; recovery by `Deps.Nudge` every 5 s on the input tick while a channel is open; `{t:control,input,window}` on channel open and on change; `ControlState()`; `{t:cursor,x,y,e}` after rmove, at most every 33 ms; `pcMoveTo` in `sys_darwin.m` now sets `kCGEventSourceUserData` to `pc_event_marker()`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Plan's release triggers would end a viewer's input permanently**
- **Found during:** Task 2 design
- **Issue:** The action says `releaseAllViewers("locked")`, `ReleaseViewer(id, "deadman")` and that `release` runs `ReleaseViewer(v.ID, "viewer")`. `ReleaseViewer` sets the viewer's `closed` flag and discards its queue, so after one lock, one idle gap or one window blur that viewer could never input again until it reconnected.
- **Fix:** those three triggers call `releaseHeld` (releases posted, input life kept). `ReleaseViewer`/`releaseAllViewers` remain for leave, channel close and Close.
- **Files modified:** internal/desktop/input.go
- **Commits:** 30f659d8

**2. [Rule 3 - Blocking] Counter names**
- **Issue:** the plan says drops increment `drop.<reason>`; 20-01's tests (and 20-03's, in the same wave) read plain `stale_epoch`/`locked`.
- **Fix:** plain names kept. No test or consumer depends on a `drop.` prefix.
- **Commit:** 30f659d8

**3. [Rule 3 - Blocking] Test access without editing export_test.go**
- **Issue:** 20-03 owns export_test.go/manager.go/protocol_test.go; the gate and control tests need an unregistered viewer and the control setters (the `unused` lint also needs a caller for `setControlWindow`).
- **Fix:** new `export_input_test.go` (package desktop, test-only) with `NewInputViewerForTest`, `HandleInputForTest`, `SetControlInputForTest`, `SetControlWindowForTest`; golden server-line test in `control_golden_test.go` (needs the unexported message types, and `readGolden` from protocol_test.go).
- **Commits:** 30f659d8, 1ef6a75d

### Other adjustments (not bugs)

- Heartbeats are counted (`hb`) so a test can know one arrived before it advances the fake clock.
- The fake bridge in input_test.go now models the pointer (a posted move updates the cursor unless `stuck`), so tests that post moves do not trip the effect check.
- `wheel` is gated by `pressAllowed` (epoch, rights, rate) but needs no display geometry, so it is not dropped for `no_target`.
- Effect checks and the cursor report carry one `shortcut:` note: the last cursor position of a burst is reported with the next rmove (no trailing report).

## Known Stubs

None. `setControlWindow` has no production caller yet by design: 20-04 calls it (window raise/minimize state); here it is covered by `TestControl_WindowStatePush`.

## TESTING.md rows for plan 20-11 to apply (not edited here)

Section 2 (Suite Manifest, internal/desktop group): `gates` and `control` have no own test file; new files are `internal/desktop/input_map_test.go`, `internal/desktop/control_test.go`, `internal/desktop/control_golden_test.go`, `internal/desktop/export_input_test.go` (helper, no tests). Section 4 (traceability), repo-relative paths only:

| Requirement | Path |
|---|---|
| DCTL-02 (keys, text, held sets, release triggers, gates, rate) | internal/desktop/input_test.go |
| DCTL-02 (click count, wheel, relative move, mapping) | internal/desktop/input_map_test.go |
| DCTL-03 (control needs Accessibility, recovery, control and cursor messages) | internal/desktop/control_test.go |
| DCTL-03 (golden server lines) | internal/desktop/control_golden_test.go |

Manual checklist candidate (Section 5): M-NN "Real Accessibility revoke and re-grant on the Mac in the VM: viewer shows Control needs Accessibility after three clicks that do nothing, and clears within 5 s of the re-grant" (cannot be automated: native permission). Also the observation that a non-US viewer layout is expected to need a layout fallback decision (printable keys go by `code`).

## Verification

- `go vet`, `gofmt -l`, `golangci-lint --new-from-rev=d792ca07` (0 issues), `bash tests/check-brand.sh` pass.
- `go test -race -count=3 -run 'TestControl_|TestInput_|TestInputMap_|TestInputTracer_'` and `go test -race -count=1 ./internal/desktop/ ./internal/screencap/` pass. `internal/desktop` coverage 88.5 %.
- Acceptance greps: `kCGEventSourceUserData` 1, preflight symbols in control.go 0, code/text/coordinate in slog calls 0; 8 `TestControl_` tests, 14 `TestInput_`/`TestInputMap_` tests.
- Host safety: no event was posted or captured on the host; the real bridge and nudge refuse under tests and every posting test uses the fake. The `sys_darwin.m` change compiles (cgo vet) but was not exercised: the real nudge only runs on the Mac in the VM.

## Flaky test (named)

`TestFanout_ThreeViewersEachGetKeyframe` failed once in a `-race -coverprofile` full-package run (`fanout_test.go:193: source started 2 times, want 1`) and passed on 10 isolated reruns and 2 further full runs. It involves no input channel, so it is a Phase 19 join/leave timing test that is sensitive to load; probably the same unidentified failure 20-01 saw. Not reproduced.

## Self-Check: PASSED

Created files exist; commits 88f5ffab, 30f659d8 and 1ef6a75d are on the branch.
