---
phase: 16-v1-2-cleanup
plan: 01
subsystem: cli
tags: [go, cli, launch-dispatch, session-resolution]

requires:
  - phase: 15-v1-2-hardening
    provides: M-153 VM run that found both CLI todos
provides:
  - "Portty --version / -v print the version with no GUI and no daemon"
  - "unknown leading flag is a usage error (stderr, exit 2); -psn_*, -NS*, -Apple* still launch the GUI"
  - "portty kill / rename resolve a session by name, ID or unique ID prefix"
affects: [16-04 TESTING.md M-156 launch-services check, v1.2 milestone close]

actuals:
  tokens: 3000
  tasks: 2
  commits: 2
plan_head_before: f4473566cb0aee0dc0feb1ac69f5f6a04ba70f69
plan_head_after: e6ec405049b585500b13dc9e4776f0a316826b21

tech-stack:
  added: []
  patterns:
    - "launchMode returns modeHelp with a non-nil error for a usage error; main prints it plus usage and exits 2"

key-files:
  created: []
  modified:
    - main.go
    - cmd_cli.go
    - dispatch_test.go
    - cmd_cli_test.go

key-decisions:
  - "D-01: unknown leading flags exit 2 with usage; allowlist is exactly -psn_*, -NS*, -Apple* (case-sensitive prefixes)"
  - "Usage error reuses modeHelp plus an error instead of a fourth mode constant"
  - "kill and rename call lookupSession and pass SessionInfo.ID to the daemon; resolveSession errors returned unwrapped"

requirements-completed: []

coverage:
  - id: D1
    description: "Portty --version and -v print the version on stdout and exit 0 without starting the GUI or daemon"
    verification:
      - kind: unit
        ref: "dispatch_test.go#TestLaunchMode/long_version,short_version"
        status: pass
      - kind: other
        ref: "built binary with temp HOME and 10 s alarm: --version and -v print v1.2.0-t16, exit 0"
        status: pass
    human_judgment: false
  - id: D2
    description: "Unknown leading flag prints the error plus usage to stderr and exits 2; never starts the GUI"
    verification:
      - kind: unit
        ref: "dispatch_test.go#TestLaunchMode/unknown_long_flag,unknown_short_flag,bare_double_dash,near_miss_psn"
        status: pass
      - kind: other
        ref: "built binary --bogus: exit 2, empty stdout, stderr has unknown-flag line and Usage line"
        status: pass
    human_judgment: false
  - id: D3
    description: "No args and -psn_*/-NS*/-Apple* launches still route to the GUI (Finder, Dock, open, login items)"
    verification:
      - kind: unit
        ref: "dispatch_test.go#TestLaunchMode/no_args,empty,launch_services_*"
        status: pass
    human_judgment: true
    rationale: "launchMode is tested, but the real Finder/Dock/open/login-item launch on macOS is TESTING.md M-156 in the Tart VM (16-04); it must run before ship"
  - id: D4
    description: "portty kill NAME and portty rename NAME NEWNAME resolve like share/attach; duplicate or unknown names change nothing"
    verification:
      - kind: integration
        ref: "cmd_cli_test.go#TestCmdKill_ByName"
        status: pass
      - kind: integration
        ref: "cmd_cli_test.go#TestCmdRename_ByName"
        status: pass
    human_judgment: false

duration: 15min
completed: 2026-11-23
status: complete
---

# Phase 16 Plan 01: CLI Fixes Summary

**`Portty --version`/`-v` print the version without the GUI, any other unknown leading flag is a usage error (exit 2) with only `-psn_*`/`-NS*`/`-Apple*` still launching the GUI, and `portty kill`/`rename` resolve a session by name, ID or unique prefix via `lookupSession`.**

## Performance

- **Duration:** about 15 min
- **Tasks:** 2 of 2
- **Files modified:** 4 (main.go, cmd_cli.go, dispatch_test.go, cmd_cli_test.go)
- **Commits:** 2 task commits

## Accomplishments

- launchMode maps `--version`/`-v` to `[version]` (modeCLI), which `cliNeedsDaemon` already skips the daemon for. main's modeHelp branch prints `portty: <err>` and usage and exits 2 when an error is set; plain `--help`/`-h` still exits 0.
- The GUI allowlist is exactly the three launch-services prefixes; every other leading flag, including the near miss `-psn` and bare `--`, returns `unknown flag "<arg>"`. File header and doc comment rewritten (the stale "locked in STATE.md" pointer now points at D-01).
- TestLaunchMode is a single 18-row table (compares cliArgs and error text). The stale TestDispatchFlagRouting, which re-implemented the old condition inline and asserted `-v`/`--verbose` launch the GUI, was deleted (Chesterton's Fence: it never called launchMode and asserted the behaviour D-01 reverses; its cases now live in TestLaunchMode).
- cmdKill and cmdRename call `lookupSession(client, args[0])` and send the resolved ID to the daemon. The daemon's rename handler looks up the path value as an ID only, so rename-by-name was broken too (CONTEXT specific idea confirmed by the RED run: 404 on PATCH). Usage lines and errors now say `<session>`.

## Task Commits

1. **Task 1: --version/-v, D-01 usage error, TestLaunchMode table** - `9321f505` (fix)
2. **Task 2: kill/rename resolve by name, ID or prefix** - `e6ec4050` (fix)

## TDD evidence (RED before GREEN)

- Task 1 RED (go test -run TestLaunchMode before the main.go change): the six new rows failed (long_version, short_version, unknown_long_flag, unknown_short_flag, bare_double_dash, near_miss_psn); the other 12 passed. Log: `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.gMqYUQH0oF/red1.log`.
- Task 2 RED (before the cmd_cli.go change): TestCmdKill_ByName failed with `DELETE /sessions/alpha: status 500: kill session: session not found`; TestCmdRename_ByName failed with `PATCH /sessions/oldname/name: status 404: session "oldname" not found`. Log: `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.MjLwSwAyJ3/red2.log`.
- Tests and implementation were committed together per task (one commit each, as the plan's tasks are the commit unit), not as separate test/feat commits.

## Verification results

- Task 1 verify passed (logs `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.HYlaLyJbL7`): go vet clean, 18 TestLaunchMode subtests PASS under -race, TestDispatchHelp and TestRunCLI_UnknownCommand PASS. Built with `-X main.Version=v1.2.0-t16`: `--version` and `-v` printed `v1.2.0-t16`; `--bogus` exit code 2, empty stdout, stderr began `portty: unknown flag "--bogus"` followed by `Usage: portty [command] [flags]`.
- Task 2 verify passed (logs `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.40EwbDWOGj`): TestCmdKill_ByName, TestCmdRename_ByName, TestCmdKill_Success, TestResolveSession PASS under -race; `lookupSession(client, args[0])` appears twice in cmd_cli.go; `kill <id>` count is 0.
- Acceptance greps for Task 1 all met (each of the five main.go patterns at 1, TestDispatchFlagRouting at 0, `launch services psn` at 1).
- The built binary was only ever run with `--version`, `-v` and `--bogus`, with HOME set to a temp dir; never with no arguments or a launch-services argument on the host.
- Only targeted tests were run, as the plan directs; the phase's full Go and vitest run is 16-04.

## Deviations from Plan

None - plan executed exactly as written.

## Authentication Gates

None.

## Known Stubs

None.

## Threat Flags

None. T-16-01 (GUI launch allowlist) is covered by the TestLaunchMode rows including the near miss; the real launch-services check is TESTING.md M-156 (16-04). T-16-02 and T-16-03 are covered by TestCmdKill_ByName/TestCmdRename_ByName (ambiguous name leaves both sessions; only a daemon-listed ID reaches the URL path).

## Notes for later plans

- 16-04 must add the TestLaunchMode and TestCmdKill_ByName/TestCmdRename_ByName files to TESTING.md and the M-156 manual item (real Finder/Dock/`open`/login-item launch in the Tart VM) before ship. The two todos (kill-by-name-fails, version-flag-hangs) are fixed in code; moving them to `todos/completed/` was not part of this plan.
- Unrelated uncommitted files (.gitignore, .planning/state.json, .ignore, agenthub, .gitkeep files) were left unstaged.

## Self-Check: PASSED

- Files modified exist: main.go, cmd_cli.go, dispatch_test.go, cmd_cli_test.go.
- Commits 9321f505 and e6ec4050 are ancestors of HEAD.
