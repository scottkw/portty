---
phase: 20-remote-control
fixed_at: 2026-11-27T04:40:00Z
review_path: .planning/phases/20-remote-control/20-REVIEW.md
iteration: 1
findings_in_scope: 10
fixed: 10
skipped: 0
deferred: 9
status: all_fixed
---

# Phase 20: Code Review Fix Report

**Fixed at:** 2026-11-27
**Source review:** .planning/phases/20-remote-control/20-REVIEW.md, plus T-20-21 (20-SECURITY.md) and UI issue 1 (20-UI-REVIEW.md)
**Iteration:** 1
**Scope (orchestrator-selected):** H-01, H-02, M-01, M-02 with T-20-21 (option (a), narrow), M-03, M-04, M-06, M-07, UI-1. M-05 and every low and info finding stay as recorded follow-ups.

**Summary:**
- Findings in scope: 10 (H-01, H-02, M-01, M-02, T-20-21, M-03, M-04, M-06, M-07, UI-1)
- Fixed: 10
- Deferred (out of scope by decision): 9 (M-05, L-01 to L-06, I-01, I-02)

Each fix is one commit, and its regression test is in the same commit. Each test was written first and seen to fail before the fix, except the M-02 cases: those changed an existing table test, which failed on the five new "drops" rows before the fix. No new test files were added. Every new test is a case in an existing file, so TESTING.md Section 2 needs no new paths. The orchestrator may still add Section 4 rows for the new test names listed below.

## Disposition

| ID | Severity | Disposition | Commit(s) | Regression test |
|---|---|---|---|---|
| H-01 | high | fixed | a8c3ceb0 | TestInput_CloseReleasesHeldAfterStartContextCancelled (internal/desktop/input_test.go) |
| H-02 | high | fixed: requires human verification (viewer side only) | 19ae0c10 | two `keys` cases in frontend/src/lib/__tests__/desktopInput.test.ts |
| M-01 | medium | fixed: requires human verification | 1dc75276 | TestSurface_SwitchAfterStartFailsResyncs, TestSurface_FallbackSwitchFailedRestarts (internal/desktop/surface_test.go) |
| M-02 | medium | fixed (option (a), narrow) | e9dfa052, docs c24f489c | TestWindow_SameAppPopupAllowed, 5 new rows (internal/desktop/window_confine_test.go) |
| T-20-21 | high (security) | CLOSED | e9dfa052, docs c24f489c | same as M-02 |
| M-03 | medium | fixed: requires human verification | 87a585e7 | TestSurface_ResizeWaitsForHeldButton (internal/desktop/surface_test.go) |
| M-04 | medium | fixed: requires human verification | efcc60d2 | TestControl_OffDisplayMovesAreNotMisses (internal/desktop/control_test.go) |
| M-05 | medium | deferred | none | Recorded follow-up (window-share key gate vs non-activating panels). It needs AX focused-UI-element work plus a VM row; out of scope by orchestrator decision. User decision 2026-11-27: accepted as a known limit for Phase 20 (own-account viewers only); must be re-reviewed and fixed or re-accepted before Phase 22 admits guests |
| M-06 | medium | fixed | 7970be7f | TestPasteboard_ReadComparesChangeCount (internal/screencap/pasteboard_darwin_test.go, source check) |
| M-07 | medium | fixed | 47317b21 | TestObjC_EntryPointsDrainAutoreleasePool (internal/screencap/screencap_darwin_test.go, source check) |
| UI-1 | warning (UI review top issue 1) | fixed | 299d842f | two `bar behaviour` cases in frontend/src/components/__tests__/DesktopControlBar.test.tsx |
| L-01 | low | deferred | none | Recorded follow-up (win.mu held across control pushes) |
| L-02 | low | deferred | none | Recorded follow-up (held Cmd chords do not auto-repeat) |
| L-03 | low | deferred | none | Recorded follow-up (slow select and clipboard handlers block the signalling loop) |
| L-04 | low | deferred | none | Recorded follow-up (releases bypass the per-viewer queue cap) |
| L-05 | low | deferred | none | Recorded follow-up (logout chords unguarded) |
| L-06 | low | deferred | none | Recorded follow-up (gConfig race during a switch) |
| I-01 | info | deferred | none | Recorded follow-up (clipResult ID slice) |
| I-02 | info | deferred | none | Recorded follow-up (composed text on kVK 0) |

## Fixed Issues

### H-01: Daemon shutdown cancels the input goroutine before Close posts the releases

**Files modified:** `internal/desktop/manager.go`, `internal/desktop/input_test.go`, `internal/desktop/loopback_test.go`
**Commit:** a8c3ceb0
**Applied fix:** The input goroutine's context is now `context.WithCancel(context.WithoutCancel(ctx))`, so only `Close` ends it, after the final flush. The test harness gained a `startCtx` field. The new test cancels the Start context while a key and a button are held, then calls `Close`, and checks that both ups are posted. Before the fix it failed with "input releases not posted before close".

### H-02: A key pressed before Cmd and released while Cmd is down is never released

**Files modified:** `frontend/src/lib/desktopInput.ts`, `frontend/src/lib/__tests__/desktopInput.test.ts`
**Commit:** 19ae0c10
**Applied fix:** When a `MetaLeft` or `MetaRight` keydown or keyup is seen, the viewer first sends a key-up for every forwarded non-modifier code and removes it from `forwarded`. This happens even if that Cmd down was itself never forwarded. Tests: the sequence S down, Shift down, Cmd down, (no S up), Cmd up, Shift up sends S up before Cmd down and ends with nothing held. A lone Cmd keyup also releases a held KeyA. The optional daemon-side equivalent (release non-modifiers on a Command key-up in `handleKey`) was not added. It is a candidate follow-up for viewers older than this fix.

### M-01: A failed surface switch leaves the mapping on a surface the viewer is not seeing

**Files modified:** `internal/desktop/manager.go`, `internal/desktop/surface.go`, `internal/desktop/surface_test.go`
**Commit:** 1dc75276
**Applied fix:**
- (a) `startSource` snapshots the surface and its info before the start. If the pick that arrived during the start then fails to switch, it records the surface the stream really started on and bumps the epoch. After `mu` is released it lets go of held input and broadcasts the surface message.
- (b) `applySurface` with a fallback reason whose switch fails now calls `restartSource()`, which starts on the main display just recorded, and keeps the fallback reason in the broadcast.
- The `switchSource` fake gained a start gate and a fail-new-sources knob.

### M-02 and T-20-21: The same-app carve-out lets clicks through to windows the capture does not show

**Files modified:** `internal/desktop/window.go`, `internal/desktop/window_confine_test.go`; docs `20-04-SUMMARY.md`, `20-VM-CHECKS.md`, `20-SECURITY.md`
**Commits:** e9dfa052 (code), c24f489c (docs)
**Applied fix:**
- Code: `sameAppPopup(top, pid)` is now `top.PID == pid && top.Layer >= 101` (`popUpMenuLayer`, kCGPopUpMenuWindowLevel). The contained-layer-0 branch and the unused frame parameter are gone.
- New table rows: layer 3 floating panel, layer 8, layer 100, layer -1 and a contained layer-0 window all drop as `covered`. Layer 102 posts.
- Docs: the "Post-VM fix (X1)" section and both X1 notes now say sheets and panels are NOT carved out.
- 20-SECURITY.md: T-20-21 is CLOSED with the new evidence. The frontmatter now reads threats_closed 52, threats_open 0, verdict SECURED, and the unregistered-flag mapping is updated.
- Still open: M-186 (pop-up click on a real Mac) is the owner-run confirmation for the menu case.

### M-03: Resizing the shared window releases held input mid-drag

**Files modified:** `internal/desktop/surface.go`, `internal/desktop/surface_test.go`
**Commit:** 87a585e7
**Applied fix:** `surfaceTick` defers a size change while any viewer holds a mouse button (new `anyButtonHeld`). The first poll after the button is up applies it, with the epoch bump and release as before. Moves and minimizes still only update the frame. The test resizes during a held press, checks that there is no epoch bump, that the drag continues, and that the viewer's own MouseUp posts, and then sees the deferred resize applied at epoch 3.

### M-04: The control-effect check falsely trips needs_accessibility off-display

**Files modified:** `internal/desktop/control.go`, `internal/desktop/manager.go`, `internal/desktop/control_test.go`, `internal/screencap/input_darwin.go`, `internal/screencap/input_darwin.m`
**Commit:** efcc60d2
**Applied fix:**
- New `Deps.PointOnDisplay`, defaulting to a new `screencap.PointOnDisplay` (CGGetDisplaysWithPoint). Like the other host bridges, it refuses under `testing.Testing()`.
- `checkMoveEffect` skips a move whose posted point lies on no active display and counts it as `effect_offscreen`. A read error falls back to the old check, so the trip still works when the answer is unknown.
- The test uses a fake that says "off display", with a stuck pointer: 4 moves give 0 misses and control stays ok.

### M-06: The pasteboard concealed check is check-then-read

**Files modified:** `internal/screencap/pasteboard_darwin.m`, `internal/screencap/pasteboard_darwin_test.go`
**Commit:** 7970be7f
**Applied fix:**
- The read is factored into `pcPbReadOnce`. It reads `changeCount` before `[pb types]` and compares it after `stringForType:`. On a change it discards the string.
- `pc_pb_read` retries once. A second change returns the new `kPCPbChangedErr` (-90304), which surfaces as a failed copy (the treat-as-busy-and-retry behaviour the orchestrator asked for).
- The test is a source check, because no test may read the host pasteboard.

### M-07: No @autoreleasepool in cgo ObjC entry points

**Files modified:** `internal/screencap/{input,media,pasteboard,sys,window}_darwin.m`, `internal/screencap/screencap_darwin_test.go`
**Commit:** 47317b21
**Applied fix:** 22 entry points now wrap their whole body in `@autoreleasepool { … }`, with the existing @try/@catch inside:
- input: 7, all but the constant `pc_event_marker`;
- pasteboard: 3;
- window: 5;
- media: `pc_stream_start`, `pc_stream_stop`, `pc_stream_select`, `pc_list_surfaces`, `pc_still_capture`, `pc_main_display_id`;
- sys: `pc_pointer_nudge` (the one Phase 20 touched, via pcMoveTo).

`git diff -w` shows only the pool lines. Media and sys entry points that Phase 20 did not touch were left alone. A source-check test pins the list.

### UI-1: Clicks pass through the hidden control bar onto the Mac

**Files modified:** `frontend/src/components/DesktopControlBar.tsx`, `frontend/src/components/__tests__/DesktopControlBar.test.tsx`
**Commit:** 299d842f
**Applied fix:** `useReveal` now treats the bar's own bounding box as reveal zone, plus the 8 px it is shifted up while hidden. That applies to pointermove. A document capture-phase pointerdown inside that box while the bar is hidden calls preventDefault and stopPropagation, so the stage and the Mac never see the press, and it reveals the bar instead. Once the bar is shown it takes its own clicks again. No CSS change was needed. The tests mock the bar's rect to be wider than the middle half and taller than the 16 px zone.

## Verification

All gates ran in the orchestrator-provided isolated worktree (branch `worktree-agent-a151d50dfaf3eacc8`), not in the main checkout. Nothing was run against the host's capture, input, AX, window list or pasteboard. The new native function refuses under tests, as the others do.

- `GOTOOLCHAIN=go1.26.11 go vet ./...`: clean.
- `gofmt -l`: clean.
- `golangci-lint run --new-from-rev=d792ca07 ./...`: 7 errcheck findings, all in files this fix run did not touch (internal/daemon/client.go, process.go, restart_pin_test.go, internal/tailnet/sessions.go). There are none in changed files.
- `go test -race -short -count=1 ./internal/desktop/ ./internal/screencap/ ./internal/daemon/`: all three ok.
- `pnpm --dir frontend exec vitest run`: 133 files, 2363 tests passed. `tsc --noEmit`: clean.
- `bash tests/check-brand.sh`: rc 0.

---

_Fixed: 2026-11-27_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
