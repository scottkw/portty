---
phase: 08-debt-cleanup
plan: 06
fixed_at: 2026-11-19T18:45:00Z
review_path: .planning/phases/08-debt-cleanup/08-06-REVIEW.md
iteration: 1
findings_in_scope: 12
fixed: 10
skipped: 2
status: partial
---

# Phase 8 Plan 06 (DEBT-08 upgrade guard): Code Review Fix Report

**Fixed at:** 2026-11-19
**Source review:** .planning/phases/08-debt-cleanup/08-06-REVIEW.md
**Iteration:** 1
**Base:** 5a28a9d1 (gsd/phase-07-hardening), fast-forwarded in the worktree before any change

**Summary:**
- Findings in scope: 12 (4 warnings, 8 info, as asked)
- Fixed: 10 (WR-01 to WR-04; IN-01, IN-02, IN-04, IN-05, IN-06, IN-08)
- Deferred: 2 (IN-03, IN-07)

The reviewer's probe (`probe_restart_test.go`) is now two committed tests:
probe 1 is the "grew" case of `TestAnswerDaemonRestart_ApprovesShownCountOnly`,
and probe 2 is `TestAnswerDaemonRestart_NoPendingPrompt`. Every new test was
run against the code before its fix and failed (or did not compile, for new
seams), then passed after it.

## Fixed Issues

### WR-01: "Restart Now" approves any number of sessions

**Files modified:** `app.go`, `app_test.go`
**Commit:** b936b812
**Applied fix:** `restartApproved` is now the count the dialog showed (0 means none). An approval covers that count or fewer. An approved unknown count (-1) covers only a count that is still unknown, and an approved known count does not cover an unknown one. Any other count re-raises the prompt with the new number. `AnswerDaemonRestart` then returns `errRestartCountChanged` ("The number of running sessions changed. Check it and choose again."), so the dialog stays open with the new count. Test: `TestAnswerDaemonRestart_ApprovesShownCountOnly` (6 cases, including the probe's 2 to 5).
**Status:** fixed: requires human verification (consent logic)

### WR-02: count-then-shutdown race

**Files modified:** `internal/daemon/process.go`, `internal/daemon/process_test.go`, `app.go`, `app_test.go`
**Commit:** f04b89c8
**Applied fix:**
- `EnsureDaemon` counts again right before `/shutdown`. If the count grew, or changed between known and unknown (`countGrew`), it goes back to the policy, and a no keeps the daemon.
- The `CreateSession` binding waits for startup (`awaitStartup`, fails with `errStartupTimeout`). It also waits while a restart question is open (Restart Now runs under it) and takes `daemonMu`, so it never runs beside the poller's or Retry's EnsureDaemon.
- `AnswerDaemonRestart` also waits for startup.
- The few-ms window left between the recount and an old daemon's unconditional `/shutdown` is a `shortcut:` comment. It points at v1.2's session handoff across an upgrade (REQUIREMENTS.md DEBT-08) and a conditional `/shutdown?ifSessions=<n>`.

Tests: `TestEnsureDaemon_StaleRestartConsent` (5 new cases on successive `/sessions` answers) and `TestCreateSession_WaitsOutRestarts`.
**Status:** fixed: requires human verification (race narrowing)

### WR-03: prompt unseen in a hidden window

**Files modified:** `app.go`, `app_update.go`, `app_test.go`
**Commit:** 67a08bfb
**Applied fix:** whenever `restartConsent` raises or re-raises the prompt, it calls the existing `showWindow` path (the one the update offer uses: `runtime.WindowShow` plus `setDockVisible(true)`), after it releases `restartMu`. Nothing blocks this in a start-minimized login launch: the window already exists (StartHidden), and `domReady` never hides it again. So no notification fallback was needed. `showWindow` now also calls `runtime.WindowUnminimise`, because Wails' `WindowShow` only does `makeKeyAndOrderFront`. Like `emitEvent`, it skips when the context has no Wails frontend (unit tests used to hit `log.Fatal` there). Test of the seam: `TestRestartConsent_ShowsWindowWhenAsking`. The native effect is M-104 step 5, which has not been run (see below).
**Status:** fixed

### WR-04: `--restart-daemon` placement

**Files modified:** `cmd_cli.go`, `main.go`, `cmd_cli_test.go`
**Commit:** a8b8c9a7
**Applied fix:**
- `takeLeadingFlag` takes the flag only from the run of flags right after the command name. Every CLI flag is a bool, so no flag can take it as a value. `daemon status` counts as the command.
- After the first argument the flag is a placement error (exit 2), never consent. So `rename <id> --restart-daemon` neither restarts the daemon nor renames the session to `--restart-daemon`. After `--` it is an ordinary value.
- `portty --restart-daemon <command> …` becomes `portty <command> --restart-daemon …` and never opens the GUI. A bare `portty --restart-daemon` exits 2 with "needs a command".
- The refusal and the usage text say where the flag goes ("Re-run as 'portty <command> --restart-daemon …'").
- The possible `args[1:]` panic is gone, because the command is never stripped.

Test: `TestRestartDaemonFlagPlacement`. A scratch build also checked the daemon-free paths (rc 2 with the right messages).
**Status:** fixed

### IN-01: AnswerDaemonRestart with no pending prompt

**Files modified:** `app.go`, `app_test.go`
**Commit:** 65aeee98
**Applied fix:** under `daemonMu` and `restartMu`, Restart Now with `restartPrompt == 0` returns `errNoRestartPending` and runs no EnsureDaemon. So it cannot override a Not Now after the fact or raise a new prompt. Test: `TestAnswerDaemonRestart_NoPendingPrompt` (probe 2).
**Status:** fixed

### IN-02: dialog after a failed Restart Now

**Files modified:** `app.go`, `app_test.go`, `frontend/src/components/__tests__/App.daemon-restart.test.tsx`
**Commit:** d4226655
**Applied fix:** when Restart Now fails and `/health` no longer answers (the old daemon was told to stop, so its sessions ended), the prompt is cleared. Go stores the error in `daemonErr` and emits `daemon:restart-needed 0`, which closes the dialog through the existing handler. It then emits `daemon:error` with "Restarting Portty's background service failed: <cause>", so the banner and Retry take over. When the old daemon still answers, the prompt stays as before, and a second Restart Now re-checks the count through WR-01. No frontend source change was needed. Tests: `TestAnswerDaemonRestart_FailedAfterStopRaisesDaemonError` and a vitest case. Note: the banner shows only while no tabs are open (`daemonError && tabs.length === 0`, existing behaviour).
**Status:** fixed

### IN-04: CLI after a GUI Not Now; health, help and version

**Files modified:** `cmd_cli.go`, `main.go`, `cmd_cli_test.go`, `dispatch_test.go`
**Commit:** 09296261
**Applied fix:** `health`, `version` and any `-h`/`-help`/`--help` skip `ensureCLIDaemon` (`cliNeedsDaemon`), so they never refuse and never start a daemon. There was no `version` command: `portty version` fell through to "unknown command" after it had already run EnsureDaemon. It now prints the build version (two lines plus a usage line). `--keep-daemon` uses the same placement rule. It makes the policy say no without a refusal and prints `portty: using the running daemon, version <A> (this is <B>)`. Combined with `--restart-daemon` it exits 2. The refusal names both flags.

**Choice: `--keep-daemon` over documentation.** Documentation is fewer lines, but it cannot tell anyone how to proceed. Every daemon-using CLI command on that Mac refuses until the old daemon's sessions end, so the only advice possible is "end your sessions or wait", which is the problem IN-04 reports. Given WR-04's `takeLeadingFlag`, the flag costs about 10 lines: one more call, one policy branch and the main.go route. Tests: `TestCLINeedsDaemon` and the `--keep-daemon` case in `TestEnsureCLIDaemon_RestartConsent`.
**Status:** fixed

### IN-06: transient `/health` failure read as a mismatch

**Files modified:** `internal/daemon/client.go`, `internal/daemon/process.go`, `internal/daemon/process_test.go`
**Commit:** 46242f48
**Applied fix:** `GetDaemonVersion` returns `(string, error)`. `EnsureDaemon` retries the read once. If the version is still unknown, it logs "daemon answered but its version could not be read (…); keeping it" and returns nil: no prompt, no `/shutdown`. `daemonReady` treats an unreadable version as not ready (the same result as before). Test: `TestEnsureDaemon_UnreadableVersionIsNotAMismatch` (3 cases).
**Status:** fixed

### IN-05: recovery adoption skipped the version check

**Files modified:** `app.go`, `app_respawn_test.go`, `internal/daemon/process.go`
**Commit:** a8b4c150
**Applied fix:** in the poller's recovery branch, a daemon from another build, or one whose version cannot be read, is adopted only through `a.ensure()` under the `daemonMu` already held. So the version check and the restart policy (prompt or keep) run as they do at startup. A failed ensure keeps the error, and the next tick tries again. A daemon from this build is adopted directly, as before, so the existing recovery tests are unchanged. `staleVersion` was renamed `StaleVersion` (exported) with an identical body, and the CR-01 readiness check is untouched. Test: `TestRespawnDaemon_RecoveryChecksVersion`.
**Status:** fixed

### IN-08: guard wiring tests

**Files modified:** `app.go`, `cmd_attach.go`, `app_test.go`, `cmd_attach_test.go`, `frontend/src/components/__tests__/App.daemon-restart.test.tsx`
**Commit:** 64b6b650
**Applied fix:** a package-level `ensureDaemon = daemon.EnsureDaemon` seam, used by `App.ensure()` and by the attach reconnect. The attach policy is now the named `keepMismatchedDaemon`.
- `TestEnsureLocalDaemon_KeepsMismatchedDaemon`: the reconnect's policy keeps the daemon for 2, 1 and -1 sessions.
- `TestEnsure_DefaultSeamPassesRestartConsent`: with no `ensureDaemonFunc`, the policy raises the prompt and says no.
- A vitest case: a rejected Restart Now keeps the dialog with the new count, the error line and enabled buttons.
**Status:** fixed

## Skipped Issues

### IN-03: API compatibility check after Not Now

**File:** `internal/daemon/process.go`
**Reason:** deferred to the v1.2 session handoff across an upgrade (REQUIREMENTS.md DEBT-08). Today it is compatible: the reviewer found the same routes, JSON tags and relay frames as v1.0, the only release. An `apiVersion` on `/health`, plus a "cannot run beside the old service" dialog branch, belongs with the handoff protocol that v1.2 designs. The first release that changes a route or payload must decide this.
**Original issue:** Not Now keeps a stale daemon with nothing checking API compatibility.

### IN-07: Quit All waiting behind an approved restart

**File:** `app.go`
**Reason:** not trivial. Stopping the restart between `/shutdown` and the spawn needs a hook inside EnsureDaemon (a ctx or a `shouldSpawn` callback) on every caller. Both actions already have consent, nothing ends wrongly, and the cost is a window that lingers for up to about 30 s on "Restarting…".
**Original issue:** Quit All can wait up to about 30 s behind an approved restart, then stop the daemon it just spawned.

## Other changes

- wailsjs stubs: no binding was added, removed or changed in signature (`CreateSession`, `GetDaemonRestartPrompt` and `AnswerDaemonRestart` keep theirs), so `frontend/src/wailsjs/go/main/App.{js,d.ts}` need no edit.
- TESTING.md (commit 1fed8e72): Section 2 note (no new test files; Go 137, vitest 93, total 238). Section 4 has 6 DEBT-08 rows. Section 5 M-104 has the new refusal text and steps 4 (CLI flags) and 5 (start minimized), both marked not yet run. `check-traceability-paths.sh`: "OK: 152 traceability paths exist". `check-brand.sh`: "OK: brand gate clean".
- Kept unchanged: the CR-01 readiness check (now `StaleVersion`), the WR-07 lock wait, the 32 KiB limits, share tokens, the CSP and the tailnet guard.

## Verification

All gates ran in the isolated worktree `.claude/worktrees/agent-a651f65b9a85a14e1` at 1fed8e72, with `GOTOOLCHAIN=go1.26.11`. They ran once, and no rerun was needed:

- `go build ./...` OK, `go vet ./...` OK
- `go test -race -short ./...`: rc 0; ok for portty, internal/attach, capability, daemon, pty, relay, release, statusbar, tailnet, updater, wakewatch and webserver
- `pnpm vitest run`: Test Files 93 passed (93), Tests 1288 passed (1288)
- `pnpm exec tsc --noEmit`: rc 0

No VM run: WR-03 uses the same native show path that the update offer and `domReady` already use, and the seam is unit-tested. The window actually appearing in a start-minimized launch is M-104 step 5, which has not been run.

---

_Fixed: 2026-11-19_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
