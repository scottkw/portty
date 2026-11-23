---
phase: 11-file-preview
plan: 03
subsystem: daemon
tags: [go, pty, env, path, symlink, zsh, path_helper]
status: complete

requires: []
provides:
  - "PORTTY_SESSION_ID=<session id> in every new session's env (required mergeEnv entry; an inherited value never leaks)"
  - "<configDir>/bin/portty symlink to this build, refreshed atomically on every CreateSession, prepended to the session PATH"
affects: [11-08 (CLI reads PORTTY_SESSION_ID; `portty open` on the session PATH), 11-10 (TESTING.md rows)]

actuals:
  tokens: 4200     # chars/4 over the realized diff (16936 chars)
  tasks: 3
  commits: 2       # measured: git rev-list --count a4a81f1e..HEAD at SUMMARY write (the SUMMARY commit follows)
plan_head_before: a4a81f1ecd1b1ae8877a857c630db53a28fa742e
plan_head_after: ddfefba0068ae4e9e68f42951a653714b3e2063a

tech-stack:
  added: []
  patterns:
    - "Package-level seam (cliExecutable) for the link target, restored per test"
    - "Atomic symlink replace: symlink under a random temp name, os.Rename over the link"

key-files:
  created:
    - internal/daemon/clilink.go
    - internal/daemon/clilink_test.go
  modified:
    - internal/pty/native.go
    - internal/pty/native_test.go
    - internal/daemon/engine.go

key-decisions:
  - "Session id generated before spawn and passed as a required env entry (overrides base and extras)"
  - "PATH entry is a per-user <configDir>/bin/portty symlink, not …/Portty.app/Contents/MacOS (capital P binary, case-sensitive volumes, completion)"
  - "A link failure logs `cli link: <reason>` and the session starts without the PATH entry; never blocks a shell"

requirements-completed: [PREV-01, PREV-05]

coverage:
  - id: D1
    description: "Every new session carries its own PORTTY_SESSION_ID, never an inherited one"
    requirement: PREV-01
    verification:
      - kind: unit
        ref: "internal/pty/native_test.go#TestSpawnPTY_EnvSessionID"
        status: pass
      - kind: integration
        ref: "internal/daemon/clilink_test.go#TestCLILink_SessionEnv"
        status: pass
  - id: D2
    description: "New session PATH starts with <configDir>/bin, whose portty links to this build; link atomic, self-healing, self-loop guarded; failure logged and non-blocking"
    requirement: PREV-05
    verification:
      - kind: integration
        ref: "internal/daemon/clilink_test.go#TestCLILink_SessionEnv, TestCLILink_RelinksWhenTargetChanges, TestCLILink_ReplacesStaleEntry, TestCLILink_ConcurrentRefresh, TestCLILink_RefusesTargetInsideBin, TestCLILink_FailureStillSpawns"
        status: pass
  - id: D3
    description: "A real zsh login shell keeps the bin dir on PATH after /etc/zprofile's path_helper; `command -v portty` resolves there"
    requirement: PREV-05
    verification:
      - kind: integration
        ref: "internal/daemon/clilink_test.go#TestCLILink_SurvivesZshLoginPathHelper"
        status: pass

duration: ~40min
completed: 2026-11-21
---

# Phase 11 Plan 03: Session id and `portty` on the session PATH Summary

**Every new session gets `PORTTY_SESSION_ID=<its id>` (generated before spawn, required over any inherited value) and `PATH=<configDir>/bin:<daemon PATH>`, where `<configDir>/bin/portty` is an atomically refreshed symlink to this build's resolved executable.**

## What was built

- **Env entry** (`internal/pty/native.go`): `generateID()` now runs before `cmd.Env` is built; `"PORTTY_SESSION_ID="+id` is a required `mergeEnv` value beside TERM and COLORTERM, so a `PORTTY_SESSION_ID` the daemon inherited (daemon started from inside a Portty shell) never reaches a new session. The Session keeps the same id. The id-generation error path now runs before the child context exists, so it only closes the pty and its dup.
- **Bin dir and link** (`internal/daemon/clilink.go`):
  - `binDir = <configDir>/bin` (configDir is `~/Library/Application Support/portty`), created 0700.
  - `cliExecutable` seam, default `filepath.EvalSymlinks(os.Executable())`.
  - `refreshCLILink(binDir)`: no-op if `bin/portty` already resolves to the target; otherwise symlink under `.portty-<16 hex>` and `os.Rename` over `bin/portty` (temp removed if the rename fails). Refuses a target equal to or inside the resolved bin dir (self-loop guard).
  - `(*SessionEngine).cliPathEnv()`: returns `PATH=<binDir>:<daemon PATH>` or nil on failure.
- **Wiring** (`internal/daemon/engine.go`): `CreateSession` passes `Env: e.cliPathEnv()` in `pty.CreateRequest`; nothing else changed. Existing sessions are untouched.
- **Log line:** on any link failure the daemon logs `cli link: <reason>` (e.g. `cli link: refusing target … inside … (it would link to itself)`, or the mkdir/symlink/rename error) and the session starts without the PATH entry.

## Documented limits (not fixed)

- Sessions created before this build, including shells moved over by a Phase 9 handoff, have no `PORTTY_SESSION_ID` (11-08's CLI says so in one line).
- A `tmux`/`screen` server started in session A and attached from session B carries A's id.
- An rc file that assigns PATH without `$PATH` drops the entry (RESEARCH Pitfall 11); the 11-08 menu item covers other terminals.
- After zsh's login `path_helper` the bin dir sits after the system dirs (no system `portty` exists, so no shadowing).
- `shortcut:` a crash between Symlink and Rename leaves a harmless `.portty-*` link in the bin dir (never named `portty`).

## Tests (for 11-10's TESTING.md rows)

| File | Tests |
|------|-------|
| `internal/pty/native_test.go` | `TestSpawnPTY_EnvSessionID` (own id with `PORTTY_SESSION_ID=stale` inherited; two sessions differ) |
| `internal/daemon/clilink_test.go` | `TestCLILink_SessionEnv`, `TestCLILink_RelinksWhenTargetChanges`, `TestCLILink_ReplacesStaleEntry` (dangling symlink, regular file), `TestCLILink_ConcurrentRefresh` (8 concurrent, create and replace rounds, no temp names left), `TestCLILink_RefusesTargetInsideBin`, `TestCLILink_FailureStillSpawns` (bin dir 0500), `TestCLILink_SurvivesZshLoginPathHelper` (real `/bin/zsh -l`, empty ZDOTDIR, temp HOME, bin path containing a space) |

All tests use temp config dirs/HOME; nothing touched the host's real config dir, PATH links or `/usr/local/bin`.

## Gate

- `go build ./...` and `go vet ./...`: pass.
- One full `go test -race -short -count=1 ./...` (`/tmp/p11-03-gate-go.log`): every package passed, including `internal/daemon` (89s) and `internal/pty` (8s), **except one pre-existing Phase 10 failure**:
  - `TestShare_DeadViewerReapedBacklogDelivered` (`internal/webserver/input_queue_test.go:175`, "input idle at the reap, want the backlog still queued behind the blocked PTY"). A parallel executor's full suite was loading the host at the time. A coordinator-approved `-count=10` rerun failed 2/10 here (`/tmp/p11-03-rerun.log`); the same `-count=10` run on the `gsd/v1.2` base `a4a81f1e` (before any 11-03 change) fails 4/10 with the same assertion, so it is not 11-03's. 11-03 touches nothing in `internal/webserver`, and that test uses an `io.Pipe` PTY. **Owned by a separate debug session.**
- `--- SKIP` lines in the gate: none.
- `tests/check-brand.sh` and `tests/check-traceability-paths.sh`: pass.

## Commits

- `5e513422` feat(11-03): session id and portty on the session PATH
- `ddfefba0` test(11-03): atomic CLI link, failure path and path_helper survival

## Deviations from Plan

1. **[TDD note] Task 2 tests were green on first run.** The refresh hardening (random temp names, rename over, temp cleanup, self-loop guard, failure log) went into Task 1's `clilink.go`, so Task 2 committed tests only. To prove they discriminate, I mutated the code twice (removed the self-loop guard; broke the rename): 1 and 5 failures respectively; the file was restored each time.
2. **Gate passed with one pre-existing failure** (above), accepted by the coordinator with base-commit evidence.

## Threat model

T-11-08 (link tampering): bin dir 0700 inside the 0700 config dir, atomic rewrite from `EvalSymlinks(os.Executable())` on every CreateSession, self-loop refused. T-11-09 (id spoofing): required env entry overrides inherited values (tested with `stale`). T-11-10 (DoS): link failure logs and spawns anyway (tested). No new surface beyond the plan's threat model.

## Self-Check: PASSED

- FOUND: internal/daemon/clilink.go, internal/daemon/clilink_test.go
- FOUND: 5e513422, ddfefba0 (ancestors of HEAD)
