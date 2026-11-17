---
phase: 04-one-click-share
plan: 10
subsystem: share
tags: [go, websocket, coder-websocket, attach, remote-proxy, uat-gap]
status: complete

requires:
  - phase: 04-one-click-share
    provides: 04-07 in-process attach harness (newAttachScene, attachCLI, checkUndisturbed) and 04-08 viewer-diag proxy lines
provides:
  - relay.ViewerReadLimit (4 * DefaultScrollbackBytes, 1 MiB), the one named read limit for Go readers of host-to-viewer frames
  - The viewing daemon proxy's upstream conn and the CLI conn (attach.AttachSession) read the catch-up preamble above 32 KiB, so proxied joiners on a long session are no longer dropped (UAT gap 4)
  - TestCLIAttach_LongHistoryJoiners (gui_tab_through_proxy, cli_through_proxy), RED-first
affects: [04 UAT re-run (M-80, M-83), phase 04 verification]

actuals:
  tokens: 5435
  tasks: 2
  commits: 3
plan_head_before: 99a3c48acd4661978a45fed0b69ea0ec43dc1a23
plan_head_after: bc78b242748f7205a2c3681bc1e1ff64ddf1bd61

tech-stack:
  added: []
  patterns:
    - "Go WebSocket readers of host-to-viewer frames call SetReadLimit(relay.ViewerReadLimit); readers of viewer input keep coder/websocket's 32 KiB default"

key-files:
  created: []
  modified:
    - internal/relay/scrollback.go
    - internal/daemon/remote_ws_proxy.go
    - internal/attach/attach.go
    - internal/daemon/attach_viewers_test.go
    - TESTING.md
    - .planning/phases/04-one-click-share/deferred-items.md

key-decisions:
  - "relay.ViewerReadLimit = 4 * DefaultScrollbackBytes (1 MiB), bounded, not disabled. Raw-ring preamble is hard-capped by the 256 KiB ring; the wrapped-ring RenderSnapshot preamble is only estimated (about 20 bytes per styled cell, well under 1 MiB), not enforced"
  - "Set only on the proxy upstream conn and inside attach.AttachSession (so runAttach and the harness share the production line); share server, local relay and proxy clientConn keep 32 KiB as the guest-input DoS guard"
  - "The harness types the long history one line at a time after newAttachScene, waiting for cat's copy of each line: typed in one burst, the tty input queue overflowed and most lines reached the ring only once (~70 KB, below the 96 KiB premise)"

patterns-established:
  - "Harness histories that must land in full are paced line by line against the ring (ScrollbackSnapshot), not typed in one burst"

requirements-completed: [SHAR-01, SHAR-06]

coverage:
  - id: D1
    description: "After about 120 KiB of history, a GUI tab opening through the viewing daemon's proxy gets the whole catch-up replay (every history line and MARKER-1), stays connected and sees live output; existing viewers keep only the 120x40 grid"
    requirement: SHAR-01
    verification:
      - kind: integration
        ref: "internal/daemon/attach_viewers_test.go#TestCLIAttach_LongHistoryJoiners/gui_tab_through_proxy"
        status: pass
    human_judgment: false
  - id: D2
    description: "The real attach.AttachSession joining through the proxy after about 120 KiB of history gets the whole replay, sees live output and detaches cleanly"
    requirement: SHAR-06
    verification:
      - kind: integration
        ref: "internal/daemon/attach_viewers_test.go#TestCLIAttach_LongHistoryJoiners/cli_through_proxy"
        status: pass
    human_judgment: false
  - id: D3
    description: "Live two-Mac check: home GUI tab and portty attach on a session that ran seq 1 20000 show the full history and stay connected, no `message too big` proxy end line (M-80, M-83)"
    requirement: SHAR-01
    human_judgment: true
    rationale: "Needs two Macs and a real tailnet path; deferred to the end-of-phase UAT"

metrics:
  duration: 8 min
  completed: 2026-11-17
---

# Phase 4 Plan 10: Proxy and CLI read limit for long catch-up replays Summary

**The viewing daemon's proxy upstream conn and the CLI attach conn now read host frames up to `relay.ViewerReadLimit` (1 MiB, 4x the 256 KiB ring), so a GUI tab or `portty attach` joining a shared session with more than 32 KiB of history gets the whole replay instead of being dropped after the grid frame (UAT gap 4).**

## Performance

- Duration: about 8 min
- Tasks: 2 of 2
- Files modified: 6

## Accomplishments

- `relay.ViewerReadLimit = 4 * DefaultScrollbackBytes` in `internal/relay/scrollback.go`. Its doc comment states the bound: the raw-ring preamble is hard-capped by the 256 KiB ring; the wrapped-ring preamble (one `RenderSnapshot` of the current screen) is estimated well under 1 MiB, not enforced. It also says which readers keep 32 KiB and why.
- `upstream.SetReadLimit(relay.ViewerReadLimit)` in `handleRemoteSessionWS` right after the successful dial, before the copy goroutines. A comment at the clientConn Accept says it keeps the default.
- `conn.SetReadLimit(relay.ViewerReadLimit)` as the first statement of `attach.AttachSession`, so local and remote `portty attach` (via `runAttach`, unchanged) and the 04-07 harness all get it.
- `TestCLIAttach_LongHistoryJoiners` with `gui_tab_through_proxy` and `cli_through_proxy`, each on its own scene, with a premise check that the ring holds between 96 KiB and 256 KiB.

## Task Commits

1. **Task 1 RED:** `3290a355` test(04-10): add failing long-history joiner variant (UAT gap 4)
2. **Task 1 GREEN:** `380bf145` fix(04-10): read host-to-viewer frames up to relay.ViewerReadLimit (UAT gap 4)
3. **Task 2:** `bc78b242` docs(04-10): record the viewer read-limit fix in TESTING.md and deferred items

## TDD Gate Compliance

- **RED (`3290a355`):** `go test -race -count=1 ./internal/daemon -run '^TestCLIAttach_LongHistoryJoiners$' -v` exit 1. Both subtests failed with `timed out after 2s waiting for late GUI tab catch-up replay (MARKER-1)` and `timed out after 2s waiting for cli catch-up replay (MARKER-1)`, and stderr showed `viewer-diag: proxy end ... first=peer code=-1 ... err=failed to read: websocket: message too big: read limited at 32769 bytes`. Classifier: `RED_EVIDENCE_OK` (target_test_failed, go-test format, 3 failing). Semantic assessment: the target ran and failed on the planned catch-up assertion for the intended reason (the proxy's upstream read limit). Setup and the history-size premise passed.
- **GREEN (`380bf145`):** the variant passes (3 PASS lines); all `TestCLIAttach_`, the attach and relay packages and the root `TestAttachSession_`/`TestWsOutputPump_` tests pass.
- **REFACTOR:** none needed.

## Mutation Checks (not committed)

- (a) Proxy line commented out: both `gui_tab_through_proxy` and `cli_through_proxy` FAIL (catch-up timeout). Restored.
- (b) AttachSession line commented out: only `cli_through_proxy` FAILS; `gui_tab_through_proxy` PASSES (its stand-in conn has no limit). Restored.
- `git status` after both showed only the pre-existing `.gitignore` change, so both production files were back as committed.

## Verification

- Task 1 verify block: exit 0.
- Full gate: `go build`, `go vet` (plain, `playwrightfixture`, `vmpreview`) pass; `go test -race -short -skip '^(TestAPIAppearance_GetPatch|TestAPITerminalFont_GetPatch)$' ./...` passed on rerun (see Issues); `check-traceability-paths.sh` (111 paths), `check-share-removal.sh` and `check-brand.sh` OK.
- Input-side guard: no `SetReadLimit(` under `internal/webserver` or `internal/relay/server.go`; no diff since 6937ec01 under `internal/webserver`, `internal/relay/server.go`, `frontend`, `go.mod`, `go.sum`; no new `_test.go` files. All Task 1 and Task 2 acceptance greps pass.

## Decisions Made

See `key-decisions` in the frontmatter. The plan-checker note about the wrapped-ring bound is in the `ViewerReadLimit` doc comment and above: the raw path is hard-capped, the rendered path is an estimate. No test was added for it.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] History typed line by line instead of as newAttachScene's prelude**
- **Found during:** Task 1 RED
- **Issue:** Passing the 200 history lines to `newAttachScene` as its prelude put only about 70 KB in the ring, below the 96 KiB premise. Diagnosis: all 200 lines arrived via tty echo, but only about 22 got cat's copy; the rest of cat's output never came (tty input queue overflow when typed in a burst), so the plan's "echoed twice, about 125 KB" assumption didn't hold.
- **Fix:** `newLongScene` calls `newAttachScene(t)` (viewers still join first), then types each `HIST-%04d-` line and waits until the ring has two copies of its tag, then types `HIST-END` and waits for the existing viewers to see it. The late joiners still wait for `MARKER-1` (in the preamble before the history; the preamble is one message, so if it arrives at all it arrives whole) and then check every HIST tag. `newAttachScene`, `viewerWait` and the existing tests are unchanged. The subtests take about 2.5 s each.
- **Files modified:** internal/daemon/attach_viewers_test.go
- **Commit:** 3290a355

**Total deviations:** 1 auto-fixed (Rule 3). **Impact:** harness-only. The RED signature and the GREEN result match the plan.

## Issues Encountered

- `TestExitEvent_ListSessions_ExitCodePopulatedForStopped` (internal/daemon/engine_exit_test.go) failed once in the first full-suite run (a running `/bin/cat` session already read as stopped). It passed 20/20 in isolation and the whole suite passed on rerun. 04-10 touched nothing in the engine. Logged to deferred-items.md as out of scope.

## Known Stubs

None.

## Next Phase Readiness

All 10 Phase 4 plans have summaries. Ready for phase verification and the two-Mac UAT (M-80, M-83 on a session that ran `seq 1 20000`).

## Self-Check: PASSED

- FOUND: internal/relay/scrollback.go, internal/daemon/remote_ws_proxy.go, internal/attach/attach.go, internal/daemon/attach_viewers_test.go, TESTING.md
- FOUND: 3290a355, 380bf145, bc78b242
