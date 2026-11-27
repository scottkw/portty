---
phase: 22-click-to-share-for-tailnet-people
plan: 03
subsystem: desktop-input
tags: [cgo, event-tap, arbiter, control-lease, physical-wins]
requires:
  - phase: 20-remote-control
    provides: screencap.EventMarker and the C pc_event_marker written to every posted event
provides:
  - screencap listen-only input tap (StartInputTap, StopInputTap, InputTapState, PhysicalState, SecureInputOn)
  - desktop arbiter (physical wins, 2 s control lease) with injected clock and physical source
affects: [22-05 press-gate integration, 22-11 Secure Event Input banner, 22-12 VM rows]
tech-stack:
  added: []
  patterns: [listen-only CGEventTap on its own run-loop thread, C atomic timestamp read through cgo, fail-closed physical knowledge]
key-files:
  created:
    - internal/screencap/inputtap_darwin.m
    - internal/screencap/inputtap_darwin.go
    - internal/screencap/inputtap_darwin_test.go
    - internal/desktop/arbiter.go
    - internal/desktop/arbiter_test.go
  modified: []
key-decisions:
  - "Tap health is decided at read time: running, CGEventTapIsEnabled true now, no dead flag, no disable event in the last 1 s; anything else is physical_unknown and drops remote input"
  - "A tap disabled by user input is recreated by the next InputTapState poll, and a tap that cannot be recreated is retried on every poll while one is wanted"
  - "Physical state (for viewers) is not set by an unhealthy tap: remote input is dropped with physical_unknown but the 'someone is using this Mac' chip is only for real untagged input"
requirements-completed: [DCTL-04]
duration: 40min
completed: 2026-11-27
status: complete
actuals:
  tokens: 14000
  tasks: 2
  commits: 3
plan_head_before: 51d8f9f94040cf981d221cca58305f5e907d63cb
plan_head_after: 0566ca50b4973d3e9ebb3e52851d8f4de8d87382
---

# Phase 22 Plan 03: Input tap and arbiter Summary

Listen-only session event tap that timestamps only untagged (non-Portty) input, a Secure Event Input probe, and a pure arbiter with physical-wins plus a 2 s control lease, tested with a fake clock and fake physical source.

## What was built

- `inputtap_darwin.m`: dedicated pthread with its own CFRunLoop holding a `kCGEventTapOptionListenOnly` session tap (mask: key down/up, flagsChanged, moved, left/right/other down/up/dragged, scroll). The callback re-enables on timeout, records a dead flag on disable-by-user-input, and otherwise stores `mach_absolute_time` in a C atomic when `kCGEventSourceUserData` differs from `pc_event_marker()`. It never reads key codes, coordinates or content, and never logs. Creation fails (and tears down) when the tap is NULL or `CGEventTapIsEnabled` is false.
- `inputtap_darwin.go`: `StartInputTap` (refuses with `ErrInputTapInTests` before any C call under `testing.Testing()`), `StopInputTap`, `InputTapState`, `PhysicalState` (Healthy / Seen / Idle on a monotonic clock), `SecureInputOn`; cgo links Carbon.
- `arbiter.go`: `decide` runs the physical rule first (unhealthy tap: `physical_unknown`; untagged input under 1 s: `physical`; both return the holder for release and clear the lease; a healthy tap that saw nothing allows), then the lease (free or expired takes and returns the old holder; holder renews to now + 2 s; others get `lease_held`). `forget`, `state` (holder plus Physical held for max(last untagged + 1 s, start + 2 s)), mutex-guarded. `physicalHold`, `leaseHold`, `physicalDwell` are constants.

## Precondition record

20-01 and 20-02 SUMMARY present; `EventMarker` present in `input_darwin.go`. `git log --oneline 07ad7d21..HEAD -- internal/screencap/`: f93c44dc, 47317b21, 7970be7f, efcc60d2, 78ea4fc8, ca7b4e74, 65c8bda4, 40781a4d.

## Task commits

1. Task 1 (tracer): `71130f97` feat - tap, probe, physical rule, 4 tests. The tracer gate was re-run end to end (vet, gofmt, list, race x3, screencap suite) and passed before expanding.
2. Task 2 RED: `81a23418` test - failing lease/forget/dwell tests (compile failure on missing `state` and `forget`).
3. Task 2 GREEN: `0566ca50` feat - lease, forget, state.

## Verification

- `go vet`, `gofmt -l`, `golangci-lint --new-from-rev=07ad7d21` (0 issues), `go test -race -count=5 -run TestArbiter_` and full `-race -count=1` of `./internal/desktop/` and `./internal/screencap/` all pass. 9 `TestArbiter_` tests plus `TestInputTap_RefusesInTests`.
- Acceptance greps: ListenOnly 1, CGEventTapIsEnabled 3, `kCGEventTapOptionDefault` 0, HID idle symbols 0, `testing.Testing()` 3, `"physical_unknown"` 1, `TestArbiter_DisabledTapDrops` present, `leaseHold = 2 * time.Second`, `"lease_held"` 1, `forget` present. `tests/check-brand.sh` clean before every commit.
- No tap was started on the host: nothing in the tests calls the C start (the Go wrapper refuses first), and `PhysicalState` under tests only reads idle atomics.

## New test files (for TESTING.md, owned by 22-13)

- `internal/screencap/inputtap_darwin_test.go`
- `internal/desktop/arbiter_test.go`

## Deviations from Plan

None - plan executed as written. Minor interpretation: `newArbiter` keeps the `now` parameter from the plan's artifact list but `decide` and `state` take `now` explicitly, so the stored clock is not read by this plan; 22-05 can pass `time.Now` and call with it. The tap thread polls its run loop in 1 s slices as well as `CFRunLoopStop`, because a stop issued before the loop runs would otherwise be lost.

## Issues for later plans

- 22-05: the tap must be started only while a controller exists and stopped after the last one; `decide` is not yet in the press gate. `StartInputTap` fails without Accessibility, which the caller must treat as physical_unknown (the arbiter already does, since `PhysicalState` is then unhealthy).
- 22-05/22-11: `state` does not set `Physical` for an unhealthy tap; if viewers should be told control is paused for that reason, it needs its own signal.
- Compile scope: `desktop` imports darwin-only `screencap.PhysicalState`, same as it already does for the input bridge.

## VM-row candidates for 22-12 (real tap needs a real Mac or the Tart VMs)

1. Tagged posts never count: Portty-posted events leave `PhysicalState` Seen false / Idle unchanged.
2. An untagged stand-in (another process posting without the marker) counts: Seen true, arbiter drops with `physical`.
3. Disabled tap without Accessibility: `StartInputTap` errors, `PhysicalState` Healthy false, remote input drops with `physical_unknown`; grant Accessibility, restart, healthy again.
4. Secure Event Input on and off: `SecureInputOn` flips, tap gets no key events while on (RESEARCH Finding 3), pointer detection still works.
5. Tap disable by timeout (re-enable) and disable by user input (recreate on next poll) behave as designed.
6. Owner real-Mac item (already planned at the 22-12 checkpoint): the warp check and the Secure Keyboard Entry case from Appendix B.

## Known Stubs

None.

## Threat Flags

None. The only new surface is the tap itself, covered by T-22-09 and T-22-10 in the plan.

## Self-Check: PASSED

All five files exist and commits 71130f97, 81a23418 and 0566ca50 are on the branch.
