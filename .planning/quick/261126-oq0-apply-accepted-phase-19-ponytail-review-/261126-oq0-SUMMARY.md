---
quick_id: 261126-oq0
type: quick
status: complete
completed: 2026-11-26
commits: 3
plan_head_before: 71225590
plan_head_after: c95728b9
key-files:
  modified:
    - internal/daemon/api.go
    - internal/daemon/desktop_api.go
    - internal/daemon/handoff.go
    - internal/daemon/launchd.go
    - internal/daemon/handoff_fault_test.go
    - internal/daemon/desktop_api_test.go
    - internal/daemon/handoff_desktop_test.go
    - internal/desktop/watchdog.go
    - internal/desktop/runtime_state.go
    - internal/desktop/watchdog_test.go
    - internal/screencap/media_darwin.go
    - internal/screencap/media_darwin.m
---

# Quick 261126-oq0: Apply accepted Phase 19 ponytail-review fixes Summary

Every SAFE item and real bug from the Phase 19 ponytail-review is applied. None was skipped. The regression tests go into existing test files.

## Commits

| Task | Commit | Message |
|------|--------|---------|
| 1 | e84cc8c0 | fix(quick-261126-oq0): daemon handoff safety for the always-available desktop |
| 2 | 45a387a0 | fix(quick-261126-oq0): keep an approval lapse open when the retry probe fails |
| 3 | c95728b9 | fix(quick-261126-oq0): serialise native capture process-wide; plug Test Capture leak |

## Items

### Task 1: daemon (e84cc8c0)

1. **freezeGate exempts PUT /desktop**: APPLIED. The comment now says PUT share and PUT /desktop take the gate themselves after their slow work. I checked that desktopOn and desktopOff both call `enterWrite()` and return `errShareFrozen`, which handlePutDesktop turns into the 503. Added `{PUT, "/desktop", false}` to `TestFreezeGate_SlowRoutesSkipBarrier`. POST /desktop/test-capture is still gated, as the plan says.
2. **desktopTick warms the cert outside the gate**: APPLIED. The tick now reads `running := a.webServer != nil` under the same RLock. When nothing is running it calls `warmShareCert` before `enterWrite` and then sets `warmCert` to a no-op, the same as desktopOn. If the warm-up fails it logs `daemon: desktop not started: <code>` and returns, so the log line the old in-gate path printed is kept. New test `TestDesktopAPI_TickWarmsCertOutsideWriteGate` (desktop_api_test.go): its fake warmCert records `!writeGate.TryLock()`. I confirmed the test FAILS when the fix is disabled ("write gate held true") and passes with it.
3. **Desktop-only finalize never fails the takeover**: APPLIED. A `newServer` or `CheckListener` error in the desktop-only case now logs `handoff: share server unusable (...); always-available server starts after the old daemon exits`, calls `dropShares()` and returns nil. The terminal-share case returns the error unchanged. New test `TestHandoffDesktop_UnusableServerStillCommits` (handoff_desktop_test.go) has two subtests: newServer returns an error, and the share server is built for a different port so CheckListener refuses the received listener. Each checks that the takeover commits, the old daemon exits, the log line appears, and the session PID survives and is alive. Both subtests FAIL with the fix disabled and pass with it.
4. **CLI install resolves symlinks**: APPLIED. `AgentControl("install")` runs `filepath.EvalSymlinks(exe)` and keeps the original path if that fails, the same as the GUI's `installedExecutable`. No test: the path ends in launchctl.

### Task 2: desktop (45a387a0)

1. **Watchdog lapse retry when the probe fails**: APPLIED exactly as the plan wrote it. New test `TestWatchdog_RetryProbeFailKeepsLapse` (watchdog_test.go). I ran it before the fix and it FAILED: state was `streaming` while LapseSeq stayed 1, the bug as described. After the fix it passes, including with `-count=5`.
2. **writeRuntimeState fsync**: APPLIED. `f.Sync()` now runs after the write and before Close, and the first error wins. No new test, as planned.

### Task 3: screencap (c95728b9)

1. **Process-wide nativeMu**: APPLIED. A package-level `nativeMu` is always taken after `d.mu`. It is held around `pc_stream_start` (together with the `noteEncoder` readback right after it, which reads the same global encoder), `pc_stream_stop`, `pc_encoder_force_keyframe` and `pc_encoder_rebuild` (plus its `noteEncoder`). The DisplaySource and ObjC header comments are updated. Deadlock check: no ObjC callback takes `d.mu` or `nativeMu`. Callbacks take only `emitMu`, and nothing holds `emitMu` while it calls into native code. Manager.mu is untouched, so WR-01 still holds.
2. **pc_still_capture late-completion leak**: APPLIED. A `__block atomic_int st` decides which side releases the image: the handler does `exchange(st,1)==2` and releases, and on timeout the caller does `exchange(st,2)==1` and releases. Added `#import <stdatomic.h>`. The normal path is unchanged.
3. **Dead FramesSeen removed**: APPLIED. Before deleting, I grepped the whole repo (Go, ObjC, headers, TS, all build tags) for `FramesSeen` and `goScreencapFrameSeen`. The only hits were the definitions themselves; the Manager's `FramesSeenForTest` is a separate function and stays. Removed: `DisplaySource.FramesSeen()`, the `seen` field, the `sink.seen` member and its two setters, the `goScreencapFrameSeen` export, and its ObjC extern and per-frame call.

No automated test can reach the native race or the screenshot timeout without Screen Recording (the real bridge refuses under `testing.Testing()`).

## Test results

- `go test -race -count=1 ./internal/daemon/`: ok (118.9 s)
- `go test -race -count=1 ./internal/desktop/`: ok; new watchdog test `-count=5` ok
- `go build ./...`, `go vet ./internal/screencap/`, `go vet -tags vmdesktop ./...`: clean (only the existing `ld: duplicate -lobjc` warning)
- `go test -race -count=1 ./internal/screencap/ ./internal/desktop/`: ok
- `go test -race -short ./...` (frontend/dist present): no failures
- `bash tests/check-brand.sh`: OK; `bash tests/check-traceability-paths.sh`: OK (272 paths)
- All builds and tests used `GOTOOLCHAIN=go1.26.11`.

## Deviations from Plan

- Task 1.2: on a warm-up failure the tick logs before it returns (the plan said return). The old code reached the same log line through bringUpDesktopLocked, so this keeps the diagnostic.
- Task 3.1: `noteEncoder` (pc_encoder_hardware) also runs under nativeMu in Start and Rebuild, because it reads the same per-process encoder global.

## TESTING.md

No new test files, only new tests inside existing files, so Sections 2 and 4 need no change.

## Self-Check: PASSED

The three commits (e84cc8c0, 45a387a0, c95728b9) are on HEAD of gsd/phase-17-desktop-feasibility-spike. All files listed above exist.
