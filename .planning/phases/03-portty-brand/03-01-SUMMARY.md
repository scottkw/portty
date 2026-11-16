---
phase: 03-portty-brand
plan: 01
subsystem: cli-daemon-identity
status: complete
tags: [brand, go-module, config-dir, cli, launchagent]
requires: []
provides:
  - "Go module github.com/scottkw/portty"
  - "Config, settings, signing key and socket under os.UserConfigDir()/portty"
  - "unknownCommandMessage(cmd) in cmd_cli.go"
  - "kardianos service portty-daemon / Portty Daemon"
affects: [03-02, 03-03, 03-04, 03-05, 03-06]
tech-stack:
  added: []
  patterns: ["Old-name comments reworded, not blanket-renamed"]
key-files:
  created: []
  modified:
    - go.mod
    - app.go
    - main.go
    - cmd_cli.go
    - cmd_daemon.go
    - cmd_attach.go
    - internal/daemon/socket.go
    - internal/daemon/engine.go
    - internal/daemon/service.go
    - dispatch_test.go
    - internal/daemon/socket_test.go
    - internal/daemon/service_test.go
    - cmd_cli_test.go
    - internal/daemon/engine_migration_test.go
    - tests/fixtures/settings_legacy_v4.json (renamed)
    - .gitignore
    - TESTING.md
key-decisions:
  - "Module path is github.com/scottkw/portty; all 57 importing .go files rewritten by one sed, zero new gofmt diffs"
  - "Portty stores settings, capability.key and daemon.sock in os.UserConfigDir()/portty (0700), fresh start, no migration from the old directory"
  - "LaunchAgent label is portty-daemon so it sits beside any old upstream agent"
requirements-completed: [BRND-03, BRND-05]
duration: 7 min
completed: 2026-11-16
commits: 3
plan_head_before: f3de5d89039d772cb3cdf600f0e4aabd4525b793
plan_head_after: c8de6dce8549b3907abae4639c54af5612c4a6fe
actuals:
  tokens: 18300
  tasks: 3
  commits: 3
coverage:
  - deliverable: "Module renamed to github.com/scottkw/portty; tree builds, vets and passes race tests"
    human_judgment: false
    verification:
      - kind: command
        ref: "go build ./... && go vet ./... && go test -race -short ./...; git grep 'github.com/scottkw/agent[h]ub' -- '*.go' go.mod exits 1"
        status: pass
  - deliverable: "Three path builders use os.UserConfigDir()/portty with 0700"
    human_judgment: false
    verification:
      - kind: test
        ref: "internal/daemon/socket_test.go#TestSocketPathDefault"
        status: pass
  - deliverable: "CLI usage, error prefixes and unknown-command message say portty"
    human_judgment: false
    verification:
      - kind: test
        ref: "dispatch_test.go#TestDispatchHelp, dispatch_test.go#TestRunCLI_UnknownCommand, cmd_cli_test.go#TestCmdNew_*, cmd_cli_test.go#TestCmdWeb_MissingSubcommand"
        status: pass
      - kind: command
        ref: "go build -o build/bin/portty-cli-check . && build/bin/portty-cli-check --help | head -1 == 'Usage: portty [command] [flags]'"
        status: pass
  - deliverable: "LaunchAgent config portty-daemon / Portty Daemon / new description"
    human_judgment: false
    verification:
      - kind: test
        ref: "internal/daemon/service_test.go#TestNewServiceConfig_Fields"
        status: pass
  - deliverable: "Legacy settings test runs on renamed fixture; plan-scope code has no old name"
    human_judgment: false
    verification:
      - kind: test
        ref: "internal/daemon/engine_migration_test.go#TestSettingsMigration_LegacyV4Keys"
        status: pass
      - kind: command
        ref: "Task 3 scoped git grep for the old name exits 1"
        status: pass
---

# Phase 3 Plan 01: Module rename, portty config dir, CLI text and LaunchAgent Summary

The Go module is now github.com/scottkw/portty (one sed over 57 files), the GUI, daemon and CLI all keep their data in ~/Library/Application Support/portty, every CLI string says `portty`, and `portty daemon install` registers a `portty-daemon` LaunchAgent.

## Performance

- Start: 2026-11-16T15:50:12Z, end: 2026-11-16T15:57:34Z (7 min)
- Tasks: 3/3, files changed: 71 (60 code/test via sed and edits, plus fixture rename, .gitignore, TESTING.md)

## Accomplishments

- **Task 1 (tracer), cef671ea:** `go mod edit -module github.com/scottkw/portty`, then BSD sed over the 57 tracked .go files that imported the old prefix. `gofmt -l *.go cmd internal web` lists only the HEAD baseline (tray.go, internal/daemon/client_test.go, internal/relay/scrollback_altscreen_test.go), so the rename added zero gofmt diffs. configDir(), daemonConfigDir() and DefaultSocketPath() changed only their leaf to `portty`; os.UserConfigDir(), the TempDir fallback and `os.MkdirAll(dir, 0700)` are unchanged. Doc comments now say os.UserConfigDir()/portty instead of the wrong `~/.config/...`. usage() header/trailer say portty; new `unknownCommandMessage(cmd)` is what runCLI prints; the EnsureDaemon error prefix is `portty: `. TestRunCLI_UnknownCommand now asserts the real builder's exact output. Tracer gate: verify re-run end-to-end, passed (4 PASS lines, built binary prints `Usage: portty [command] [flags]`).
- **Task 2, 721d987c:** every usage line and error prefix in cmd_cli.go (8 `usage: portty` lines), cmd_daemon.go and cmd_attach.go reads portty. Service config: Name `portty-daemon`, DisplayName `Portty Daemon`, Description `Keeps Portty terminal sessions running in the background.`. `.gitignore` ignores `/portty` instead of the four old binary names. TESTING.md TERM-01 row shows `portty new [dir]`; two BRND-03 rows added.
- **Task 3, c8de6dce:** fixture `git mv`'d to tests/fixtures/settings_legacy_v4.json (tailscale path now `/usr/local/bin/tailscale`), test renamed to TestSettingsMigration_LegacyV4Keys with identical assertions, one-line comment rewrites in engine.go, api.go, api_test.go, engine_test.go, plugin_settings.go, staticapp.go, relay/server.go, relay/oscabsorb_relay_test.go, app_exit_test.go, playwright-fixture/main.go. TESTING.md: 03-01 Section 2 note, NAV-03/STRP-02 rows renamed, two more BRND-03 rows.

## Gate results

| Gate | Result |
|------|--------|
| Task 1 automated verify | PASS (4/4 named PASS lines, `--help` line 1 exact) |
| Task 2 automated verify | PASS (TestNewServiceConfig_Fields, 7/7 TestCmdWeb/TestCmdNew PASS lines, scoped grep exit 1, `/portty` line, traceability OK) |
| Task 3 automated verify | PASS (TestSettingsMigration_LegacyV4Keys, fixture tracked, plan-scope grep exit 1, traceability OK: 96 paths) |
| All acceptance criteria (Tasks 1-3) | PASS |
| `go build ./... && go vet ./... && go test -race -short ./...` | PASS |
| gofmt beyond HEAD baseline | none |
| `git grep 'github.com/scottkw/agent[h]ub' -- '*.go' go.mod` | exit 1 (no hits) |
| `pnpm -C frontend exec tsc --noEmit` / `vitest run` | PASS / 1173 passed |
| `bash tests/check-traceability-paths.sh` | OK: 96 traceability paths exist |

## Deviations from Plan

**1. [Rule 3 - Blocking] cmd_remote_test.go comment fixed in Task 2 instead of Task 3**
- Task 2's verify greps `cmd_*.go`, which includes cmd_remote_test.go line 82; leaving it for Task 3 would fail Task 2's gate. Applied the plan's exact Task 3 wording in commit 721d987c.

**2. [Rule 1 - Bug] Stale test name in a live TESTING.md row**
- The STRP-04 row (settings_removed_test.go) also named the old migration test. Updated it to TestSettingsMigration_LegacyV4Keys in c8de6dce. Historical Section 2 notes that name the old test were left as history.

**3. [Process] Pre-existing `.gitignore` edit kept out of the commit**
- The working tree already had an uncommitted user edit (`graft/` to `/graft/`). Only this plan's hunk was staged (git apply --cached); the user's hunk stays uncommitted.

**Total deviations:** 2 auto-fixed (1 blocking, 1 bug) plus one process note. **Impact:** none on scope.

## Issues Encountered

- `TestExitEvent_ListSessions_ExitCodePopulatedForStopped` (internal/daemon/engine_exit_test.go) failed once under the full race suite (the cat session reported an exit code before the "running" check), then passed 5/5 alone and on every full re-run. Load-dependent flake, unrelated to the rename. Not fixed (out of scope).
- A stale, untracked `agenthub` Mach-O binary (Nov 15) sits at the repo root. It was hidden by the old `/agenthub` ignore line, which the plan replaced with `/portty`, so `git status` now shows it. Left untouched; the user can delete it.
- Observation: the daemon test suite starts a real daemon on os.UserConfigDir()/<leaf>/daemon.sock. It used to touch the upstream app's directory; it now touches ~/Library/Application Support/portty instead.

## Known Stubs

None.

## Next Phase Readiness

Ready for 03-02. Remaining old-name text lives in files owned by later plans (internal/daemon/process.go and client.go, frontend, docs, build).

## Self-Check: PASSED

- FOUND: go.mod, internal/daemon/socket.go, internal/daemon/engine.go, app.go, cmd_cli.go, internal/daemon/service.go, tests/fixtures/settings_legacy_v4.json
- FOUND commits: cef671ea, 721d987c, c8de6dce
