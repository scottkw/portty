---
phase: 20-remote-control
plan: 03
subsystem: desktop
tags: [surface, screencapturekit, updateContentFilter, epoch, fallback, wire-protocol]
requires:
  - phase: 20-01
    provides: "bumpEpoch/currentEpoch, takeHeldLocked, release{flush}, Deps.DisplayBounds, frozen signal-v1.jsonl surface lines"
  - phase: 19
    provides: "Manager, DisplaySource, pc_encoder_rebuild, nativeMu, one stream per process"
provides:
  - "surfaces and select over /desktop/ws with RightsSurface, busy, not_found and switch_failed answers"
  - "live switch of the running capture (encoder rebuild, updateConfiguration, updateContentFilter, still-frame fallback, rollback)"
  - "epoch bump before every switch, held input released without ending any viewer's input life"
  - "fallback to the main display (window_closed, display_gone), resize re-select, restart keeps the surface, reset when the last viewer leaves"
  - "screencap.ListSurfaces, BuildSurfaces, SurfaceInfo, MainDisplayID"
affects: [20-04, 20-07, 20-09, 22]
tech-stack:
  added: []
  patterns:
    - "surface state behind its own leaf mutex; the main display is stored as the zero value and read fresh"
    - "a switch slot (switching flag) shared by picks and fallbacks; the poll never holds it while listing"
key-files:
  created:
    - internal/desktop/surface.go
    - internal/desktop/surface_test.go
    - internal/desktop/surface_export_test.go
    - internal/screencap/surfaces.go
    - internal/screencap/surfaces_test.go
  modified:
    - internal/desktop/manager.go
    - internal/desktop/protocol_test.go
    - internal/screencap/screencap.go
    - internal/screencap/media_darwin.go
    - internal/screencap/media_darwin.m
    - internal/screencap/screencap_darwin_test.go
key-decisions:
  - "A switch lets go of held input with a new releaseHeldAll, not releaseAllViewers, because releaseAllViewers ends each viewer's input life (closed flag) and viewers must keep sending after a switch"
  - "The surfaces list needs RightsSurface too (window titles are not for a viewer that may not pick)"
  - "A fallback records the main display even if the native switch fails, so a restart starts on a surface that exists"
  - "Frames are held back from the encoder rebuild until the new filter is in place (gSwitching), so a scaled picture of the old surface is never encoded"
requirements-completed: [DESK-05, DESK-06]
status: complete
duration: ~1h30m
completed: 2026-11-26
estimate:
  tokens: 95000
  raw_tokens: 95000
  tasks: 3
  confidence: low
actuals:
  tokens: 21700
  tasks: 3
  commits: 3
plan_head_before: 6662f19644b8914f539b0952f62f526d6d7e3711
plan_head_after: 129e73760a30bd18176d70ff0071b91e060aabf4
---

# Phase 20 Plan 03: Surface list, live switch and fallback Summary

A viewer can list the Mac's displays and windows, pick one, and the running capture switches to it with no new offer: the epoch is bumped first, held input is released, every viewer gets the new surface and size, and a vanished window or display falls back to the main display without a black stream.

## What was built

- **Task 1 (tracer, 7f980dbb):** `surface.go` with `handleSurfaces`, `handleSelect`, `applySurface`, `target()` (moved from manager.go), `surfaceMsg`, `CurrentSurface`. `manager.go` gets `Deps.ListSurfaces` and `Deps.MainDisplayID`, `inbound.Kind`/`ID`, the post-offer dispatch and the join's surface message. `screencap.SurfaceInfo`, `screencap.MainDisplayID`. `TestSurfaceTracer_SelectWindowBumpsEpoch` runs it over loopback WSS and pion with two viewers.
- **Task 2 (408f03fa):** `BuildSurfaces` (naming, filter, sort), `decodeSurfaces`, `ListSurfaces` (refuses under `testing.Testing()`), `pcResolveSurface` (replaces `pcMainDisplay`; a window by ID via `initWithDesktopIndependentWindow`, requested with `onScreenWindowsOnly:NO`), `pc_stream_start` with kind and id, `pc_stream_select`, `pc_list_surfaces`, `DisplaySource.SelectSurface` live or recorded.
- **Task 3 (129e7376):** `surfaceLoop` (1 s poll only while a window or non-main display is shared), fallback with `ReasonWindowClosed` / `ReasonDisplayGone`, resize re-select, move and minimize update only the recorded frame, `startSource` uses the current surface, `leave` of the last viewer resets it and bumps the epoch, `Close` waits for the loop.

## Phase 19 reconcile gate

`git log --oneline d792ca07..HEAD -- manager.go media_darwin.m media_darwin.go screencap.go` listed 20-01's two commits and the Phase 19 fixes (c95728b9 process-wide native serialisation `nativeMu`, cf9c60ab, babc963f, df7164c3, 5a0ec826, a0279fd3 start outside the lifecycle lock, 779d5924, 20e38428). All were read before editing: the new native calls take `nativeMu` after `d.mu` (same order as `Rebuild`), and `startSource` still starts outside `m.mu`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `releaseAllViewers("surface")` would end every viewer's input**
- **Found during:** Task 1 design (read of input.go)
- **Issue:** `releaseAllViewers` calls `endViewerInput`, which sets `closed` and discards the viewer's queue for good. After a switch viewers must keep sending (the plan's own tracer sends moves with epochs 2 and 3).
- **Fix:** `releaseHeldAll` in surface.go snapshots `takeHeldLocked` for every viewer and queues the releases, between two `flushInput` waits (the first lets an item that passed the epoch gate just before the bump record what it holds). `TestSurface_SwitchReleasesHeld` proves releases post and the holder can still move.
- **Files modified:** internal/desktop/surface.go
- **Commit:** 7f980dbb

**2. [Rule 2 - Missing critical] List needs RightsSurface**
- **Issue:** the plan gates only `select`; a viewer without the right would still receive every window title.
- **Fix:** `surfaces` from such a viewer answers `{type:surfaces, error:"forbidden"}`. Tested in `TestSurface_SelectForbiddenAndBusy`.
- **Commit:** 7f980dbb

**3. [Rule 2 - Missing critical] Failed or raced switches must not leave the stream dead**
- **Issue:** (a) after a failed `updateConfiguration`/`updateContentFilter` the encoder was already at the new size, so every frame would be dropped; (b) a select that runs while a start is in flight only recorded its choice, leaving the source on the old surface; (c) a fallback whose native switch fails left a restart pointing at a surface that no longer exists.
- **Fix:** (a) `pc_stream_select` rebuilds back to the old size and re-applies the old filter on failure, and holds frames (`gSwitching`) from the rebuild until the new filter is in; (b) `startSource` re-selects when the recorded surface differs from the one it started with; (c) a fallback (non-empty reason) records the main display regardless.
- **Commits:** 408f03fa, 129e7376

**4. [Rule 3 - Blocking] Real main display ID needed for the surface message**
- **Issue:** viewers match the current surface to the list by ID (20-07 checks the current item), so the join message must carry the main display's real ID, not 0. Nothing in the tree returned it.
- **Fix:** `screencap.MainDisplayID` (`CGMainDisplayID`, no capture) and `Deps.MainDisplayID`. Not in the plan's artifact list.
- **Commit:** 7f980dbb

**5. [Plan inconsistency] Stale versus no_target on a window share**
- **Issue:** the plan says an e 1 move on a window share is "dropped as stale", but `gate()` in input.go checks `!t.ok` (no_target) before the epoch. input.go belongs to 20-02, so I left the order alone.
- **Fix:** the tracer test asserts both moves on the window share are dropped as `no_target`, and tests the stale epoch on display 2 instead (e 2 after the switch to epoch 3 counts `stale_epoch`, e 3 posts at (2430, 540)). 20-04 may reorder the gate when it adds the window gate.

### Other adjustments (not bugs)

- Test-only `internal/desktop/surface_export_test.go` (new file, not in the plan's list) holds `SetViewerRightsForTest` and `RestartSourceForTest`, so the shared export_test.go and input_test.go were not touched.
- The surface and size messages are not ordered: the size message comes from the fan-out when the first new-size frame arrives, which can precede the surface message. Tests wait for each separately. Either order is safe for input (the epoch is the guard).
- Selecting the surface already shared runs a full switch (epoch bump, encoder rebuild). Not short-circuited; small, harmless, and a viewer's "Switching..." state always gets its surface message.
- The poll lists without holding the switch slot and takes it only to apply, so a viewer's pick is not answered `busy` just because a poll was listing.
- `flushInput` repeats the tail of `releaseAllViewers` (input.go is 20-02's); dedupe in a later cleanup.
- A select for an off-screen (minimized) window that is in the fresh list is allowed; only the picker hides it.
- `gsd_run windows append` was not run (helper unavailable in this worktree shell); the only known gap is under Known Stubs.

## Cross-plan hazards for the orchestrator

- surface.go calls `takeHeldLocked`, `m.in.rels`, `release{flush}`, `wakeInput`, `flushWait`, `bumpEpoch` from input.go. If 20-02 renames or reshapes them (for example to add held keys), the merge needs `releaseHeldAll` updated; keeping `takeHeldLocked(*viewerInput) []release` makes it work for keys too.
- `target()` now lives in surface.go; 20-02/20-04 must not re-add it to manager.go or input.go.
- manager.go: 20-03 is the only plan editing it in wave 2; later plans (20-05 clipboard messages) add to `inbound` and the same dispatch switch.

## Known Stubs

None in this plan's code. Window targets intentionally return no geometry (`target()` fails closed for windows) until 20-04 adds the window gate and frame mapping; releases still post. Surface-list ObjC and the live switch have never run on a real display (host safety): they compile, lint clean and are refused under tests. `windows append` ledger entry for this intentional gap was not written (see above).

## Verification

- `go vet`, `gofmt -l`, `golangci-lint --new-from-rev=d792ca07` (0 issues), `bash tests/check-brand.sh` pass.
- `go test -race -count=3 -run 'TestSurfaceTracer_|TestSurface_|TestProtocolGolden_' ./internal/desktop/` passes (11 `TestSurface_` tests plus the tracer and the golden decode test).
- `go test -race -short -count=1 ./internal/desktop/ ./internal/screencap/ ./internal/daemon/` passes.
- Host safety: no capture, no `SCShareableContent` request, no event posted. `ListSurfaces` and the running-switch branch refuse under `testing.Testing()`; tests use a test-local switching source and list.
- Flaky tests: none seen in the full runs. One race of my own was fixed during Task 1 (two moves sent back to back are coalesced by the input queue, so the test sends the second after the first is counted). The unidentified 20-01 failure did not reproduce in five full-package runs.

## TESTING.md rows for plan 20-11 to add (not edited here)

Section 2 (Suite Manifest, new files): `internal/desktop/surface_test.go` (loopback surface tests, desktop group), `internal/desktop/surface_export_test.go` (test seams, no tests), `internal/screencap/surfaces_test.go` (pure BuildSurfaces and decodeSurfaces tests, screencap group). `internal/screencap/screencap_darwin_test.go` and `internal/desktop/protocol_test.go` gain tests.

Section 4 (traceability, path column file only):

| Requirement | Path |
|-------------|------|
| DESK-05 | internal/desktop/surface_test.go |
| DESK-05 | internal/screencap/surfaces_test.go |
| DESK-05 | internal/desktop/protocol_test.go |
| DESK-06 | internal/desktop/surface_test.go |
| DESK-06 | internal/screencap/screencap_darwin_test.go |

Section 5 (manual, VM rows; 20-09 owns the runs):

- M-NN: switch 2560x1440 display to a 1280x800 window and back, decoded and rendered in Chrome, Safari, the Portty Wails web view and iPhone Safari with no renegotiation (research A11).
- M-NN: after `updateContentFilter` onto a still window or display, a frame arrives within 1 s; record whether the log line "screencap: switch used a still frame" fired (research A12).
- M-NN: the window list on a real Mac shows on-screen normal windows including Portty's own, none of the helper or menu-bar windows, and a minimized window disappears from the picker but does not drop the share to the main display.
- M-NN: share a window, close it: the viewer shows the fallback notice and the stream never goes black; share the second display, unplug it: the same with display_gone.

## Self-Check: PASSED

Created files exist (surface.go, surface_test.go, surface_export_test.go, surfaces.go, surfaces_test.go); commits 7f980dbb, 408f03fa and 129e7376 are on the branch.
