---
phase: 15-v1-2-hardening
plan: 03
subsystem: daemon-socket
tags: [go, unix-socket, chmod, handoff, hardening]
requires:
  - phase: 09-handoff
    provides: takeover.commit and the received socket listener (audit item O-2)
provides:
  - "The daemon control socket is 0600 from listen; a failed chmod closes the listener and fails the start"
  - "A takeover tightens the socket it inherited to 0600 after COMMIT, best effort"
affects: [15-08 (edits the same handoff.go and handoff_test.go), 15-09 (TESTING.md M-155 real-process chain)]
status: complete
actuals:
  tokens: 1500
  tasks: 2
  commits: 2
plan_head_before: bf94ae2a5009f7eb85c57e424fb52b9cb68e3423
plan_head_after: 349b37847ac031e28551e5b3f83e055bcb37dffc
commits: 2
tech-stack:
  added: []
  patterns: ["chmod right after listen, failing closed", "post-COMMIT best-effort tighten that only logs"]
key-files:
  created: []
  modified:
    - internal/daemon/ipc.go
    - internal/daemon/socket_test.go
    - internal/daemon/handoff.go
    - internal/daemon/handoff_test.go
key-decisions:
  - "chmod after listen, not a umask change (umask is process-global and races other file creation)"
  - "tightenSocketMode runs after COMMIT and only logs: the sessions have already moved, so failing would cost them"
requirements-completed: []
coverage:
  - id: D1
    description: "A new daemon's control socket is mode 0600 right after it listens, the owner can still connect, and a chmod failure fails the start"
    verification:
      - kind: unit
        ref: "internal/daemon/socket_test.go#TestListenDaemonSocket_Mode0600"
        status: pass
    human_judgment: false
  - id: D2
    description: "A takeover that commits sets an inherited 0755 socket to 0600 and never fails on the chmod"
    verification:
      - kind: unit
        ref: "internal/daemon/handoff_test.go#TestHandoff_TightensSocketMode"
        status: pass
      - kind: unit
        ref: "internal/daemon/handoff_test.go#TestHandoff_ and TestHandoffFault_ suites under -race"
        status: pass
    human_judgment: false
  - id: D3
    description: "A socket created by a pre-phase daemon reads 0600 after a real two-process upgrade and after a second one"
    verification: []
    human_judgment: true
    rationale: "Real daemons from two builds never run on the host; TESTING.md M-155 (recorded by 15-09) runs in the Tart VM portty-test at /gsd-verify-work"
---

# Phase 15 Plan 03: Daemon socket 0600 Summary

The daemon control socket is chmod 0600 right after it listens (a chmod failure closes the listener and fails the start), and a takeover tightens the socket it inherits to 0600 after COMMIT without ever failing the takeover.

## Accomplishments

- `internal/daemon/ipc.go`: `const daemonSocketMode os.FileMode = 0o600`. `listenDaemonSocket` chmods after a successful listen; on error it closes the listener and returns `daemon socket: set mode 600 on <path>: <err>`. Its only caller is `API.Start`.
- `internal/daemon/ipc.go`: `tightenSocketMode(path)` chmods to the same mode and only logs `handoff: could not set the daemon socket to 600: ...` on error.
- `internal/daemon/handoff.go`: `takeover.socketPath`, set in `runTakeover` from `d.socketPath`. `commit` calls `tightenSocketMode(t.socketPath)` right before the final committed log and `return takeoverCommitted`. That one return serves both commit paths (with COMMITTED, and the old-daemon-gone path); the failure paths return earlier and never reach it.
- `TestListenDaemonSocket_Mode0600` (socket_test.go) starts through `testDaemon` (API.Start), requires mode 0600 and `client.Health()` nil.
- `TestHandoff_TightensSocketMode` (handoff_test.go) checks the scene socket is 0600, chmods it to 0755, runs a committing takeover, requires 0600 again.

## Task Commits

1. Task 1 (tracer): `5c25a63a` feat(15-03) 0600 at listen, fails closed
2. Task 2: `349b3784` feat(15-03) takeover tightens the inherited socket after COMMIT

## RED evidence

The plan is `type: execute`, so each task's test and implementation went into one commit. Tests were still written first and run at HEAD (`go test`, `GOTOOLCHAIN=go1.26.11`), each failing on its planned assertion:

- `TestListenDaemonSocket_Mode0600`: `socket mode = 755, want 600` (log `tmp.LiD6eOLujN/t1-red.log`).
- `TestHandoff_TightensSocketMode`: `socket mode after the takeover = 755, want 600`; the takeover committed, so the target executed and failed on the mode, not on setup (log `tmp.FS6PrsVUya/t2-red.log`).

## Verification

Logs are under `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/`: task 1 `tmp.jXEWMQw1Wa`, task 2 `tmp.aChuFmvnFh`.

- go vet ./internal/daemon/: clean, both tasks.
- Task 1 (`-race`): `TestListenDaemonSocket_Mode0600` and `TestCleanupStaleSocket_*` pass, including `_ActiveSocket`; no FAIL lines.
- Task 2 (`-race`): `TestHandoff_TightensSocketMode`, `TestHandoff_HappyPath`, every other `TestHandoff_*` and `TestHandoffFault_*` pass; no FAIL lines.
- All acceptance-criteria greps pass (daemonSocketMode x4 in ipc.go, one `listenDaemonSocket(socketPath)` in api.go, one `func tightenSocketMode(`, one `tightenSocketMode(t.socketPath)`, one of each new test).
- Tracer gate: auto mode is off and the tracer's verify is automated-only, so it was re-run end to end after Task 1 (passing) before expanding.
- The full Go and vitest suites run once, in 15-09. The real-process chain is TESTING.md M-155 (15-09 records it; /gsd-verify-work runs it in the Tart VM). TESTING.md is not touched here.

## Deviations from Plan

None - plan executed exactly as written.

## Known Stubs

None.

## Threat Flags

None. T-15-06 and T-15-09 are mitigated as planned.

## Self-Check: PASSED

Both commits (`5c25a63a`, `349b3784`) are ancestors of HEAD; all four files_modified paths changed in the plan range; no unexpected deletions; unrelated user files (.gitignore, .planning/state.json, .ignore, agenthub/, .gitkeep) were not staged.
