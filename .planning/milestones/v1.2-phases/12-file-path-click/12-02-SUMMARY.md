---
phase: 12-file-path-click
plan: 02
subsystem: daemon
tags: [go, zsh, osc7, zdotdir, shell-integration, pty]

requires:
  - phase: 11-file-preview
    provides: "clilink.go cliPathEnv/refreshCLILink pattern and CreateSession env plumbing"
provides:
  - "internal/daemon/zshshim.zsh: embedded ZDOTDIR shim (.zshenv) that reports the cwd with OSC 7"
  - "internal/daemon/zshshim.go: refreshZshShim (atomic, verified write) and SessionEngine.zshEnv"
  - "CreateSession adds ZDOTDIR and PORTTY_ZSH_ZDOTDIR only for the real session shell"
affects: [12-03 hub OSC 7 parser, 12-07 TESTING.md consolidation]

actuals:
  tokens: 7000
  tasks: 3
  commits: 3
plan_head_before: c634351937ca8e12b8ca7297c461987deda732bc
plan_head_after: 3f2175da5903d50f345422f422be3ab5b571d789

tech-stack:
  added: []
  patterns:
    - "ZDOTDIR shim: restore the user's ZDOTDIR first, source their .zshenv in an always block, then add hooks"
    - "Set the env only after the written file reads back equal; on any failure set nothing and log"

key-files:
  created:
    - internal/daemon/zshshim.zsh
    - internal/daemon/zshshim.go
    - internal/daemon/zshshim_test.go
  modified:
    - internal/daemon/engine.go
    - internal/daemon/clilink_test.go

key-decisions:
  - "The shim script is the RESEARCH Pattern 1 script byte-for-byte; no case of the matrix needed a change to it"
  - "zshEnv returns nil (not an error) on any failure: the session then starts with the user's own startup files, the safe direction"
  - "Writer reads the file back after the rename and compares to the embedded bytes before ZDOTDIR is ever set"

requirements-completed: [PATH-01]

coverage:
  - id: D1
    description: "A new Portty zsh session reports its cwd with OSC 7 at startup, after cd and at each prompt, with no user setup"
    requirement: PATH-01
    verification:
      - kind: integration
        ref: "internal/daemon/zshshim_test.go#TestZshShim_SessionReportsCwd"
        status: pass
    human_judgment: false
  - id: D2
    description: "User startup files behave exactly as without Portty across custom ZDOTDIR, ZDOTDIR in ~/.zshenv, broken ~/.zshenv, subshells, redirects, rc reset, nested and non-interactive shells, stale variable, clean startup"
    requirement: PATH-01
    verification:
      - kind: integration
        ref: "internal/daemon/zshshim_test.go#TestZshShim_CustomZDOTDIR"
        status: pass
      - kind: integration
        ref: "internal/daemon/zshshim_test.go#TestZshShim_BrokenUserZshenv"
        status: pass
      - kind: integration
        ref: "internal/daemon/zshshim_test.go#TestZshShim_UserFilesUntouched"
        status: pass
    human_judgment: false
  - id: D3
    description: "Shim write is atomic, idempotent and fail-safe; non-shell spawns untouched; portty still resolves after path_helper"
    requirement: PATH-01
    verification:
      - kind: unit
        ref: "internal/daemon/zshshim_test.go#TestZshShim_WriterAtomic"
        status: pass
      - kind: integration
        ref: "internal/daemon/zshshim_test.go#TestZshShim_FailureStillSpawns"
        status: pass
      - kind: integration
        ref: "internal/daemon/clilink_test.go#TestCLILink_SurvivesZshLoginPathHelperWithShim"
        status: pass
    human_judgment: false

duration: about 2h across two executor runs (the first died on an API timeout after Task 2)
completed: 2026-11-21
status: complete
---

# Phase 12 Plan 02: zsh ZDOTDIR shim for OSC 7 Summary

**Every new Portty zsh session now reports its working directory with OSC 7 through an embedded `.zshenv` shim the daemon writes atomically and verifies before pointing ZDOTDIR at it, leaving the user's own startup files untouched and in their normal order.**

## Accomplishments

- A real `/bin/zsh -l` session started by `CreateSession` emits `\x1b]7;file:///usr\x07` after `cd /usr`, once at startup and at every prompt; inside the shell `$ZDOTDIR` is empty and `PORTTY_ZSH_ZDOTDIR` is unset.
- The shim restores the user's ZDOTDIR first, so `.zprofile`, `.zshrc`, `.zlogin` and `/etc/zshrc`'s HISTFILE resolve as without Portty.
- Failure never leaves a shell without its startup files: any write/verify error, or an empty or relative config dir, sets neither env entry and logs one `zsh shim:` line.

## Shim and env

- **Shim path:** `<configDir>/zsh/.zshenv` (dir 0700, file 0600), rewritten from the embedded script at each `CreateSession` only when it differs.
- **Env entries** (only when `cliPath == sessionShell`): `ZDOTDIR=<configDir>/zsh` and `PORTTY_ZSH_ZDOTDIR=<daemon's own ZDOTDIR, or empty>` (always present, so a stale inherited value is overridden by mergeEnv's extras).
- **Log line:** `zsh shim: <reason>` (relative config dir, or the write/verify error).
- **OSC 7 form:** `ESC ] 7 ; file:///<percent-encoded path> BEL`, empty host.

## Deviations from Plan

None - plan executed as written. The shim script equals RESEARCH Pattern 1 byte-for-byte (diffed); no matrix case required a change to it.

Additions beyond the named tests, each cheap and aimed at the coverage bar: `TestZshShim_WriterFailures` (read-only dir; a directory named `.zshenv`; both leave no temporary) and `TestZshShim_EnvEntries` (the two entries, with and without a daemon ZDOTDIR).

## Documented limits

- Nested shells, ssh, and bash or fish run inside a session emit no OSC 7 of their own; the outer zsh's last reported cwd stays.
- A user rc file that empties `precmd_functions` disables tracking; relative clicks then do nothing (the locked fallback).
- Sessions started before an upgrade keep the ZDOTDIR they were spawned with (fixed at exec); harmless, the restore runs first.

## Test names (for 12-07's TESTING.md rows)

- `internal/daemon/zshshim_test.go` (PATH-01, needs a Section 4 traceability row): `TestZshShim_SessionReportsCwd`, `_CustomZDOTDIR`, `_ZDOTDIRSetInUserZshenv`, `_BrokenUserZshenv`, `_SubshellAndRedirect`, `_RcResetsChpwd`, `_NestedAndNonInteractive`, `_StaleInheritedVar`, `_CleanStartupNoErrors`, `_UserFilesUntouched`, `_WriterAtomic`, `_WriterFailures`, `_FailureStillSpawns`, `_RefusesRelativeConfigDir`, `_EnvEntries`, `_NonShellSpawnUntouched`.
- `internal/daemon/clilink_test.go`: `TestCLILink_SurvivesZshLoginPathHelperWithShim` (sibling of the unchanged `TestCLILink_SurvivesZshLoginPathHelper`).
- TESTING.md was not edited, per the plan. `bash tests/check-traceability-paths.sh` passes.

## Gate results

- `go build ./... && go vet ./...` clean; one full `go test -race -short -count=1 -coverprofile=/tmp/p12-02-cover.out ./...` passed with 0 FAIL lines (log `/tmp/p12-02-gate-go.log`; daemon package 83.0% overall, 88s).
- **zshshim.go statement coverage: 90.0%** (`/tmp/p12-02-cover.txt`; the 80% bar is met).
- **`--- SKIP` lines in the gate log: 0.** No ZshShim test skipped on this Mac.
- `check-brand.sh` and `check-traceability-paths.sh` exit 0; no stray `internal/daemon/zsh` directory.

## Commits

- 08955d35 feat(12-02): zsh sessions report their working directory with OSC 7 (ZDOTDIR shim)
- 37d384a6 test(12-02): zsh shim startup-file matrix under the real zsh
- 3f2175da test(12-02): atomic shim writer, failure path, path_helper with the shim

## Known Stubs

None.

## Threat Flags

None beyond the plan's register (T-12-04 to T-12-09 are each covered by a named test above).

## Issues Encountered

The first executor died on an API timeout after Task 2 with Task 3's tests uncommitted in the tree. The continuation read that diff, found it complete against the plan, and ran the gate unchanged.

## Self-Check: PASSED
