---
phase: 04-one-click-share
plan: 08
subsystem: relay
tags: [go, logging, diagnostics, websocket, relay, proxy, uat-gap]

requires:
  - phase: 04-one-click-share
    provides: 04-07 in-process CLI attach harness (attach_viewers_test.go) and the H-grid / H-net hypotheses that need live evidence
provides:
  - Detached daemon writes daemon.log (0600, previous run kept as daemon.log.1) in its config dir, with microsecond timestamps
  - viewer-diag lines on the host (grid, join, leave, slow, revoke, first input, input total, web resize ignored)
  - viewer-diag lines on the viewing Mac (proxy open, proxy dial failed, proxy end)
  - TESTING.md M-83 two-Mac repro that says which lines to collect
affects: [04 UAT re-run (M-83), phase 05 reconnect/idle]

actuals:
  tokens: 10889
  tasks: 3
  commits: 6
plan_head_before: 0a6e451d49bee99429b5f609d0fb4c5b68e00743
plan_head_after: 167fa485f5710bfcebeb3b9d50771f4ee48387cc

tech-stack:
  added: []
  patterns:
    - "viewer-diag: lines go through the standard log package, written only on state changes and never under hub.mu"
    - "Connection numbers (Subscriber.ID) tie a viewer's join, input, resize and leave lines together"

key-files:
  created:
    - internal/daemon/daemon_log.go
    - internal/daemon/daemon_log_test.go
  modified:
    - internal/daemon/process_unix.go
    - internal/daemon/process.go
    - internal/relay/hub.go
    - internal/relay/hub_test.go
    - internal/webserver/server.go
    - internal/daemon/remote_ws_proxy.go
    - internal/daemon/remote_ws_proxy_test.go
    - internal/daemon/attach_viewers_test.go
    - TESTING.md
    - .planning/phases/04-one-click-share/deferred-items.md

key-decisions:
  - "Proxy log errors strip the whole upstream query before redacting the raw token; redactCapTokenFromError alone left cap=<redacted> in the dial error"
  - "relay.DiagID (exported, first 8 characters of a session ID) is shared by the hub, the webserver read pump and the proxy"
  - "A log-file failure in startDetachedDaemon prints one portty: daemon log: line and starts the daemon anyway; the kardianos LaunchAgent path is unchanged"

patterns-established:
  - "Diagnostic log lines quote client names with %q and log counts, never payloads, tokens or URLs"

requirements-completed: [SHAR-06, SHAR-01, SHAR-03]

coverage:
  - id: D1
    description: "The detached daemon writes daemon.log (0600, rotated to daemon.log.1 at each start) in its config dir, and a hub grid change lands in it"
    requirement: SHAR-06
    verification:
      - kind: unit
        ref: "internal/daemon/daemon_log_test.go#TestOpenDaemonLog_RotatesAndIs0600"
        status: pass
      - kind: integration
        ref: "internal/daemon/daemon_log_test.go#TestDaemonLog_GridChangeLandsInFile"
        status: pass
    human_judgment: false
  - id: D2
    description: "Host-side viewer timeline: join, leave, one slow line per stalled viewer, revoke, %q-quoted names that cannot forge lines"
    requirement: SHAR-06
    verification:
      - kind: unit
        ref: "internal/relay/hub_test.go#TestHub_DiagLog"
        status: pass
    human_judgment: false
  - id: D3
    description: "Viewing-side proxy lines (open with dial time, dial failed, end with side, code and reason) that never contain the cap or cap="
    requirement: SHAR-06
    verification:
      - kind: integration
        ref: "internal/daemon/remote_ws_proxy_test.go#TestRemoteWSProxy_DiagLog"
        status: pass
    human_judgment: false
  - id: D4
    description: "A whole CLI attach logs a coherent timeline (four joins, the CLI's ignored 80x24 resize and leave under one conn number, proxy open/end) with no cap"
    requirement: SHAR-06
    verification:
      - kind: integration
        ref: "internal/daemon/attach_viewers_test.go#TestCLIAttach_OtherViewersKeepOutput"
        status: pass
    human_judgment: false
  - id: D5
    description: "Live two-Mac CLI attach beside a GUI tab and a browser; if a viewer blanks, daemon.log on both Macs shows which mechanism fired"
    requirement: SHAR-06
    verification: []
    human_judgment: true
    rationale: "Needs two Macs, a real tailnet path and Warp (TESTING.md M-83); no in-process trigger for H-grid or H-net"

duration: 93min
completed: 2026-11-17
status: complete
---

# Phase 4 Plan 08: Daemon Diagnostic Log for UAT Gap 1 Summary

**The detached daemon now writes to a 0600 daemon.log in its config dir instead of /dev/null. The host hub, the share server's read pump and the viewing daemon's proxy write `viewer-diag:` lines at each point where H-grid or H-net would show. The lines carry counts and sizes only, never tokens, links or typed bytes. TESTING.md M-83 says which lines to collect from both Macs.**

## Performance

- **Duration:** 93 min (wall clock, including test runs)
- **Started:** 2026-11-17T17:59:46Z
- **Completed:** 2026-11-17T19:32:46Z
- **Tasks:** 3
- **Files modified:** 12 (2 created, 10 modified)

## Accomplishments

- `openDaemonLog` rotates `daemon.log` to `daemon.log.1`, then opens a fresh 0600 file. `startDetachedDaemon` points the child's stdout and stderr at it. `RunDaemon`, and never `runDaemonCore`, which tests call, sets microsecond log timestamps.
- Hub: `Subscribe` assigns a per-hub connection ID and logs `join`. `Unsubscribe` logs `leave`, but only when the subscriber was present. `ResizeClient` logs `grid` with the old and new size, origin and client on a real grid change. `kickSlow` replaces the three bare `go sub.CloseSlow()` sites and logs `slow` once per subscriber, inside the spawned goroutine. `RevokeWebViewers` logs `revoke`.
- Webserver read pump: `Subscriber.Remote` is the viewer's IP. The pump logs `first input` (delay after join, byte counts), `input total` on exit, and `web resize ignored` with cols and rows.
- Proxy: `proxy open` with the dial duration, `proxy dial failed` with a redacted error, and `proxy end` with which side ended first, the close code (-1 when the connection ended without a close frame), the reason and the duration. Only the upstream hostname is logged.
- TESTING.md: Go 112 and total 202, three new SHAR-06 rows plus an updated 04-07 row, and M-83 in Category AS.

## Task Commits

1. **Task 1: daemon.log plus the grid line (tracer)**: `9402c8ee` (feat). Tracer gate: the automated verify was re-run and passed (end-of-phase mode, automated-only verify), then expansion continued.
2. **Task 2: host-side viewer timeline**
   - `de628d23` (test, RED: `TestHub_DiagLog`)
   - `39e183d6` (feat, GREEN)
3. **Task 3: proxy lines, attach log check, TESTING.md**
   - `398b5fd9` (test, RED: `TestRemoteWSProxy_DiagLog`, plus log assertions in `TestCLIAttach_OtherViewersKeepOutput`)
   - `34cd7265` (feat, GREEN)
   - `167fa485` (docs: TESTING.md, deferred-items.md)

## TDD Gate Compliance

- **RED, Task 2 (`de628d23`):** `gsd-tools check tdd-red-evidence` returned `RED_EVIDENCE_OK` (go-test, `TestHub_DiagLog` failed, exit 1). Semantic check: the test compiled and failed on its first planned assertion, `connection IDs not assigned uniquely: web=0 local=0`. It was not a build or fixture fault. To make the test compile, the RED commit also declares the two diagnostics-only `Subscriber` fields, `ID` and `Remote`. Nothing assigns them yet.
- **RED, Task 3 (`398b5fd9`):** `RED_EVIDENCE_OK` for `TestRemoteWSProxy_DiagLog`. It failed on `no proxy end line`. The extended tracer failed on the same missing proxy line after its host-side assertions (join, resize ignored, leave) had already passed.
- **GREEN:** `39e183d6` and `34cd7265`. Both named tests pass, and the relay, webserver and proxy suites are green.
- **REFACTOR:** none needed.

## Files Created/Modified

- `internal/daemon/daemon_log.go`: `openDaemonLog`
- `internal/daemon/daemon_log_test.go`: rotation and mode test, plus a grid line landing in the file (t.TempDir only)
- `internal/daemon/process_unix.go`: child stdout and stderr go to daemon.log; a failure is reported and does not stop the daemon
- `internal/daemon/process.go`: `log.SetFlags(LstdFlags | Lmicroseconds)` in `RunDaemon`
- `internal/relay/hub.go`: `Subscriber.ID`, `Subscriber.Remote`, `slowLogged`, `Hub.nextConnID`, `DiagID`, `webCountLocked`, `kickSlow`, and the five viewer-diag lines
- `internal/relay/hub_test.go`: `TestHub_DiagLog`
- `internal/webserver/server.go`: Remote IP, join time, the first input / input total / web resize ignored lines
- `internal/daemon/remote_ws_proxy.go`: `copyEnd`, side-tagged `copyWS`, and the open / dial failed / end lines
- `internal/daemon/remote_ws_proxy_test.go`: `TestRemoteWSProxy_DiagLog`
- `internal/daemon/attach_viewers_test.go`: log capture and timeline assertions in the tracer, a `capToken` field on the scene, and a header line pointing at the viewer-diag evidence
- `TESTING.md`: Section 2, Section 4 and M-83
- `.planning/phases/04-one-click-share/deferred-items.md`: settings tests read the real settings.json

## Decisions Made

See key-decisions in the frontmatter.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Security] Proxy error redaction strips the whole query**
- **Found during:** Task 3
- **Issue:** The plan logs `redactCapTokenFromError(dial error, cap)`. coder/websocket's dial error quotes the full URL, so the line still read `...?cap=<redacted>`. That broke the plan's own requirement that no line contains `cap=`. A token that `QueryEscape` changes would also have leaked in its escaped form.
- **Fix:** Errors are logged with `u.RawQuery` replaced by `<redacted>` first, then passed through `redactCapTokenFromError` for the raw token. A mutation run without the query strip made `TestRemoteWSProxy_DiagLog` fail with exactly that leak.
- **Files modified:** internal/daemon/remote_ws_proxy.go
- **Commit:** `34cd7265`

**2. [Rule 3 - Blocking] Exported `relay.DiagID`**
- **Found during:** Task 2
- **Issue:** The webserver and the daemon proxy need the same 8-character session ID as the hub, and the hub's helper was unexported.
- **Fix:** Added one exported `relay.DiagID(sessionID)`, used in all three places.
- **Commit:** `39e183d6`

**Total deviations:** 2 auto-fixed (1 security, 1 blocking). **Impact:** no change in scope. The redaction fix makes T-04-31 hold for real dial errors.

## Issues Encountered

- **Pre-existing, not caused by 04-08:** `TestAPIAppearance_GetPatch` and `TestAPITerminalFont_GetPatch` (internal/daemon) fail on the full `go test -race -short ./...` run. They report `want system, got "dark"` and font size 16. Those values match the user's real `~/Library/Application Support/portty/settings.json`, so the test daemon reads real settings. Both fail the same way on the 04-08 base commit `0a6e451d`, checked in a temporary worktree that was removed afterwards. Logged in deferred-items.md with a fix sketch, not fixed (scope boundary). Every other package is green.
- `TestRevoke_InputAfterRevokeNeverReachesPTY` did not flake in this plan's two full-suite runs.
- The new viewer-diag lines also print during `go test` (the standard logger goes to stderr), which makes verbose test output longer. They contain no tokens.

## Known Stubs

None.

## Threat Flags

None. No new network endpoint, auth path or schema. The one new file on disk (daemon.log) is covered by T-04-34.

## User Setup Required

None. After the next build, the daemon writes `~/Library/Application Support/portty/daemon.log` by itself once it restarts.

## Next Phase Readiness

- Run TESTING.md M-83 on the two Macs at the end-of-phase UAT. If a viewer blanks, attach the `viewer-diag` lines from both Macs.
- The 32 KiB WebSocket read-limit item in deferred-items.md is still open. In M-83 it would show as a joiner's `proxy end` with `code=-1` right after `proxy open` on a long session.
- The CSP, capability gate and Origin allowlist are untouched. go.mod and go.sum are unchanged, nothing under frontend/ changed, and the real config directory gained no files from the tests.

## Self-Check: PASSED

- Key files exist: internal/daemon/daemon_log.go, internal/daemon/daemon_log_test.go.
- All six commits (`9402c8ee`, `de628d23`, `39e183d6`, `398b5fd9`, `34cd7265`, `167fa485`) are ancestors of HEAD.
- Every acceptance criterion in Tasks 1-3 passed. The plan's `<verification>` passed: build, vet and the tagged vets; the named daemon and relay tests; the traceability, share-removal and brand gates; the config dir unchanged; go.mod and go.sum unchanged. One exception: the full race suite has the two pre-existing settings-test failures described above.

---
*Phase: 04-one-click-share*
*Completed: 2026-11-17*
