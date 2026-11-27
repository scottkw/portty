---
phase: 20-remote-control
plan: 04
subsystem: desktop
tags: [window-share, accessibility, raise, topmost, confinement, cgevent]
requires:
  - phase: 20-02
    provides: "pressAllowed gate, setControlWindow, held sets and release triggers"
  - phase: 20-03
    provides: "surface state, target() in surface.go, SurfaceInfo.PID, applySurface"
provides:
  - "window gate: a live Accessibility focus read per press; raise off the post path, triggering press dropped"
  - "confinement: outside and covered presses dropped, moves clamped, releases clamped and never dropped, window chords dropped"
  - "control window states raise_failed and minimized pushed to every input channel"
  - "screencap.WindowBridge (AX focus, window info, minimized, raise ladder, topmost) and desktop.WindowBridge seam"
affects: [20-05, 20-07, 20-09, 22]
tech-stack:
  added: []
  patterns:
    - "window gate state behind a leaf mutex; no bridge call under it"
    - "a raise runs on its own goroutine, one at a time; a generation counter disowns a late result after a surface change"
key-files:
  created:
    - internal/desktop/window.go
    - internal/desktop/window_test.go
    - internal/desktop/window_confine_test.go
    - internal/desktop/window_export_test.go
    - internal/screencap/window_darwin.m
    - internal/screencap/window_darwin.go
    - internal/screencap/window_darwin_test.go
  modified:
    - internal/desktop/gates.go
    - internal/desktop/input.go
    - internal/desktop/surface.go
    - internal/desktop/manager.go
    - internal/desktop/surface_test.go
key-decisions:
  - "gate() reordered for window shares: the window gate (and so the epoch check) runs before no_target, and the target is taken after it. The press that reads the first frame must not be dropped as no_target. Answers 20-03's open question: yes, reordered, for windows only; display shares keep no_target first"
  - "An off-screen press raises by the frame cached from the last on-screen read, not the off-screen CG bounds, which are not reliable for a minimized window"
  - "A press is mapped with the cached frame and dropped as outside when that point is not in the fresh frame; when it is inside, the point posted is mapped on the fresh frame"
  - "A scroll is gated like a press: pointer must be inside the fresh frame and the shared window topmost at the pointer (the plan listed only presses; a scroll lands at the pointer, the prohibition names scrolls)"
  - "Release posts (button up from the viewer, and every release trigger through postRelease) clamp to the cached frame; the viewer's up first re-reads the frame"
patterns-established:
  - "window drop reasons: raising, outside, covered, window_chord, window_unavailable (plain counter names)"
requirements-completed: [DCTL-05, DESK-05]
status: complete
duration: ~2h
completed: 2026-11-26
estimate:
  tokens: 85000
  raw_tokens: 85000
  tasks: 3
  confidence: low
actuals:
  tokens: 26000
  tasks: 3
  commits: 3
plan_head_before: 1af57e1f104fe64b042317269d473b8c39730fe5
plan_head_after: 9f5809a6ed04ae29c9adb42138608f2e9d3a00d7
---

# Phase 20 Plan 04: Window share gate, raise and confinement Summary

A viewer's input to a window share now reaches only that window: presses post only while a live Accessibility read says the shared window is focused, a background window is raised by the first press (that press dropped), and clicks outside or under another window, and the system chords, never reach another app.

## What was built

- **Task 1 (tracer, bd7b4248):** `window.go` with the `WindowBridge` seam, `windowAllows`, `startRaise` (own goroutine, one at a time), the frame cache; `gate()` reorder; `target()` for windows returns the cached frame; `Deps.Window`, `Manager.win`; `setSurface` resets window state when the surface changes. `TestWindowTracer_RaiseThenPost` runs the whole path over loopback (select window, press dropped and Raise called once with the window's pid, frame and title, next press posts MouseDown at (740, 500), the up posts there).
- **Task 2 (40781a4d):** `window_darwin.m/.go`: `pc_ax_focus` (system-wide focused app then focused/main window frame), `pc_window_info` (CGWindowList by ID), `pc_window_minimized`, `pc_window_raise` (rung 1 AX frontmost/unminimize/AXRaise/AXMain, rung 2 NSWorkspace openApplicationAtURL, rung 3 NSRunningApplication, each followed by up to 3 focus read-backs 50 ms apart that must match pid and frame), `pc_window_topmost`, global 0.25 s AX messaging timeout, also set per application element. No private `_AXUIElementGetWindow`, no NSWorkspace frontmost read, no preflight symbols. `RungName` for the log. Every method refuses under `testing.Testing()`. `NewManager` defaults `Deps.Window` to the real bridge.
- **Task 3 (9f5809a6):** outside and covered drops, wheel confinement, clamped moves and releases (`clampToWindow`, `refreshWindow`), `windowChord` in gates.go, `raise_failed` (failed raise, or not finished in 300 ms; a success clears it), `minimized` (pushed by the 200 ms `windowLoop` while a window is shared and an input channel is open, cleared by an on-screen read), loop started and awaited by Start/Close.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] The plan's gate order would drop the press that reads the first frame**
- **Found during:** Task 1 design
- **Issue:** `gate()` checked `!t.ok` (no_target) first, and a window has no frame until a press reads it, so the first press would never reach the window gate or start a raise.
- **Fix:** for window shares `gate()` runs `allowed` first and takes the target after it. Consequence: on a window share a stale epoch now counts `stale_epoch` before anything else; 20-03's tracer assertions for window moves changed accordingly (stale e 1 counts `stale_epoch`, current e 2 counts `window_unavailable` because that world has no window bridge).
- **Files modified:** internal/desktop/input.go, internal/desktop/surface_test.go
- **Commit:** bd7b4248

**2. [Rule 2 - Missing critical] Scrolls had no confinement**
- **Issue:** the plan's covered/outside checks name presses; a scroll posts at the pointer, which could sit over a covering window, and the DCTL-05 prohibition names scrolls.
- **Fix:** a wheel needs the pointer (read through the input bridge) inside the fresh frame and the shared window topmost there. Test `TestWindow_WheelConfined`.
- **Commit:** 9f5809a6

**3. [Rule 2 - Missing critical] Release triggers other than the viewer's up could land outside a shrunk window**
- **Fix:** `postRelease` clamps a button release to the cached frame too (dead-man, lock, surface switch).
- **Commit:** 9f5809a6

### Other adjustments (not bugs)

- `windowAllows(msg)` has no viewer parameter (unused); `windowChord(v, msg)` does. It counts both the viewer's held modifier keys and the message's modifier mask, so a lost Command key-down cannot let a chord through.
- New test-only files beyond the plan's list: `window_export_test.go` (seams `SetSurfaceForTest`, `WindowRaisingForTest`) and `window_confine_test.go` (Task 3 tests).
- After Task 2 the nil check on `Deps.Window` is unreachable through `NewManager` (it defaults to the real bridge, which refuses under tests, giving the same `window_unavailable`); kept for hand-built Managers.
- The plan's trailer named another model; the commits carry "Claude Sonnet 5.5", the model that wrote them.
- The per-plan commit ledger file was not written (the git wrapper refused the compound command); `plan_head_before` is the base the orchestrator gave (1af57e1f) and `git rev-list --count` over it gives 3.

## Known limitations (decisions for the user)

- **A pop-up of the shared app counts as covering.** `TopmostAt` returns a window ID only, so a menu, popover or sheet window of the shared app on top at the click point is dropped as `covered`. Safe (fail closed), but it can block clicking inside such pop-ups. A fix is to return the owner pid and accept same-app windows; not done, since the plan specifies the id-only comparison.
- Every press in a window share costs one CGWindowList read plus one AX focus read before the rate limit is spent; a flooding viewer burns CPU on its own queue only (round-robin keeps others served).
- The raise ladder, minimized detection and topmost list have never run against a real window server (host safety): they compile and are refused under tests. Research A1/A2 (raise from the launchd daemon on macOS 27.2, minimized window returns) stay backstop for plan 20-09 in the VMs, as does whether `CGWindowListCopyWindowInfo(IncludingWindow)` returns a minimized window.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register. The new native surface (AX reads and writes into other apps) is the plan's own T-20-20, T-20-23 and T-20-24.

## TESTING.md rows for plan 20-11 to apply (not edited here)

Section 2 (Suite Manifest): desktop group gains `internal/desktop/window_test.go`, `internal/desktop/window_confine_test.go`, `internal/desktop/window_export_test.go` (seams, no tests); screencap group gains `internal/screencap/window_darwin_test.go`.

Section 4 (traceability, path column file only):

| Requirement | Path |
|---|---|
| DCTL-05 (window gate, raise, confinement, chords) | internal/desktop/window_test.go |
| DCTL-05 (outside, covered, clamp, minimized, raise_failed, refresh) | internal/desktop/window_confine_test.go |
| DCTL-05 (native bridge refuses under test) | internal/screencap/window_darwin_test.go |
| DESK-05 (window target frame mapping) | internal/desktop/window_test.go |

Section 5 (manual, VM rows for 20-09):

- M-NN: from the launchd daemon on macOS 27.2, a click on a shared background window brings it to the front (record which rung: ax, workspace or activate) and the next click lands in it; a minimized shared window is restored by a click.
- M-NN: with a floating panel or notification banner over the shared window, clicks at that point do nothing in either window; Cmd+Tab, Cmd+Space and Ctrl+Cmd+Q from the viewer do nothing while a window is shared.
- M-NN: a shared app that hangs does not freeze input to other viewers' display shares (0.25 s AX timeout), and the viewer shows the raise-failed chip.

## Verification

- `go vet`, `gofmt -l`, `golangci-lint --new-from-rev=d792ca07` (0 issues), `bash tests/check-brand.sh` pass.
- `go test -race -count=3 -run 'TestWindow_|TestWindowTracer_'` passes (11 `TestWindow_` tests plus the tracer); `go test -race -count=1 ./internal/desktop/ ./internal/screencap/` passes; `internal/desktop` coverage 90.3 %.
- Acceptance greps: `_AXUIElementGetWindow` 0, `frontmostApplication` 0, `AXUIElementSetMessagingTimeout` 3, `kAXFocusedApplicationAttribute` 1, `"covered"` 1, `"window_chord"` 1, `WindowMinimized|WindowRaiseFailed` 5.
- Host safety: no AX read or write, raise, window-list read or event ran on the host; every `WindowBridge` method refuses under tests and all tests use the fake.

## Flaky test (named)

None seen in the three full-package runs (`TestFanout_ThreeViewersEachGetKeyframe` did not fail).

## Post-VM fix (X1)

VM row X1 showed a click on a pop-up menu item of the shared app dropped as `covered`. Fixed in 78ea4fc8 (test d0efd82e): `TopmostAt` now returns a `screencap.TopWindow` (ID, owner pid, layer, frame) read from the same CGWindowList entry, and `windowAllows` lets a press or scroll through when the topmost window is the shared one, or is owned by the shared window's pid and sits at a pop-up menu layer (101 and up: menus, pop-up menus, their popovers). Narrowed in e9dfa052 after the security audit (T-20-21): sheets and panels are NOT carved out. The shared app's sheets (layer 0), floating panels (layer 3), modal panels (layer 8), utility windows, negative layers, and its other document windows still drop as `covered`, as do all other apps' windows, because a single-window capture most likely does not show them. Any read error still fails closed. No private API. The earlier "A pop-up of the shared app counts as covering" limitation above is resolved for menus and pop-ups only; a Save sheet or Fonts panel in a window share is still unclickable (switch to a display share). The `topmost` VM probe op also reports pid and layer. Needs a VM recheck (M-NN candidate for 20-11).

## Self-Check: PASSED

Created files exist; commits bd7b4248, 40781a4d and 9f5809a6 are on the branch.
