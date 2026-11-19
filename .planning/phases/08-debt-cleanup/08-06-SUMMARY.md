---
phase: 08-debt-cleanup
plan: 06
subsystem: daemon-lifecycle
status: complete
tags: [upgrade, daemon, consent, cli, gui, reliability]
requirements-completed: [DEBT-08]
dependency_graph:
  requires: [08-05, "08-REVIEW-FIX iteration 3 CR-01 (staleVersion / daemonReady)"]
  provides: ["daemon.RestartPolicy seam in EnsureDaemon", "CLI --restart-daemon", "GUI restart consent dialog"]
  affects: [main.go runCLI, cmd_attach.go reconnect, app.go startup/respawn/retry]
tech-stack:
  added: []
  patterns: ["policy callback on EnsureDaemon; a 'no' returns nil with the old daemon serving", "Go keeps state + emits event, frontend pulls on mount (GetDaemonRestartPrompt) to beat the subscribe race"]
key-files:
  created:
    - frontend/src/components/__tests__/App.daemon-restart.test.tsx
  modified:
    - internal/daemon/process.go
    - internal/daemon/process_test.go
    - app.go
    - app_test.go
    - cmd_cli.go
    - cmd_cli_test.go
    - main.go
    - cmd_attach.go
    - frontend/src/App.tsx
    - frontend/src/wailsjs/go/main/App.js
    - frontend/src/wailsjs/go/main/App.d.ts
    - frontend/src/lib/webTerminalInput.test.ts
    - TESTING.md
    - .planning/phases/08-debt-cleanup/08-VM-CHECKS.md
decisions:
  - "EnsureDaemon(socketPath, RestartPolicy): with live sessions (State != stopped) a 'no' keeps the old daemon and returns nil; the policy's own 'no' is the caller's kept signal. Zero sessions restart without asking; a /sessions error asks with -1"
  - "GUI startup never blocks on the dialog: the policy keeps the daemon and records the prompt; Restart Now re-runs EnsureDaemon under daemonMu with a one-call approval, then emits daemon:recovered"
  - "The dialog reuses the house confirm modal (RegenerateKeyModal, already generalized for End Session), Not Now focused and on Esc"
  - "The CLI flag --restart-daemon is accepted anywhere after the command and stripped before dispatch; the attach reconnect path always keeps a mismatched daemon"
  - "app.go QuitAll's ShutdownDaemon (the old app.go:478) is Quit Everything's deliberate stop and is unchanged"
metrics:
  duration: 17m
  completed: 2026-11-19
  tasks: 3
  files: 15
commits: 5
plan_head_before: 93d114ca
plan_head_after: 0e147432c874770de19266be312946546dc03c9c
actuals:
  tokens: 11500
  tasks: 3
  commits: 5
---

# Phase 8 Plan 06: Upgrade guard (DEBT-08) Summary

An app upgrade no longer ends live sessions without asking. EnsureDaemon now takes a consent policy. With live sessions in a version-mismatched daemon, the GUI keeps the old daemon and asks "Update Portty's background service?" (Not Now is the default). The CLI refuses with a clear message unless `--restart-daemon` is given. With no live sessions the restart stays automatic, and the CR-01 `staleVersion` fix is untouched.

## What was built

- **Task 1: consent seam** (`internal/daemon/process.go`). `type RestartPolicy func(daemonVersion string, sessions int) bool`, and `EnsureDaemon(socketPath, allowRestart)`. On a mismatch, `liveSessions` counts sessions that have not stopped (-1 if `/sessions` fails). A count other than 0 asks the policy. A "no" logs `… with N live sessions; keeping it` and returns nil without sending `/shutdown`. `staleVersion`, `daemonReady` and `startDaemon` are unchanged.
- **Task 2: CLI and GUI.**
  - CLI: `ensureCLIDaemon` (cmd_cli.go) turns a "no" into `the running daemon is version X with N live sessions; restarting it to version Y would end them. Re-run with --restart-daemon to restart anyway.`, which main prints as `portty: …` with exit 1. `runCLI` takes `--restart-daemon` out of the arguments. Usage documents it. The attach reconnect (`ensureLocalDaemon`) always keeps a mismatched daemon.
  - GUI: `App.restartConsent` is the policy behind the `ensure()` seam. It keeps the daemon, stores the session count and emits `daemon:restart-needed`. The frontend also pulls `GetDaemonRestartPrompt` after `GetDaemonError`. `AnswerDaemonRestart(false)` clears the prompt and stops further prompts this launch. `AnswerDaemonRestart(true)` approves one EnsureDaemon under `daemonMu` (Quit All wins), then emits `daemon:recovered`. A failed restart keeps the prompt and shows the error in the dialog.
  - Dialog: the house `RegenerateKeyModal` with title "Update Portty's background service?" and body "N sessions are running. Restarting ends them." (1-session and unknown-count variants). It has Not Now (focused, Esc) and Restart Now (destructive fill).
  - The wailsjs stubs are hand-maintained in this repo, so they gain the two bindings by hand. The Wails builds regenerated only `src/wailsjs/wailsjs/*`, and those files were restored with `git checkout`.
- **Task 3:** TESTING.md (Sections 2, 4 and 5, Category AX / M-104) and the G-01..G-03 VM run in 08-VM-CHECKS.md.

## Tests added

- `internal/daemon/process_test.go`: `TestEnsureDaemon_StaleRestartConsent` (5 subtests: keep, restart, only-stopped, none, unknown count). `TestRunDaemon_Exports` and `TestEnsureDaemon_NeverUnlinksSocket` were updated to the new signature.
- `app_test.go`: `TestRestartConsent_NotNowKeepsDaemonWithoutAskingAgain`, `TestAnswerDaemonRestart_RestartNow`.
- `cmd_cli_test.go`: `TestEnsureCLIDaemon_RestartConsent`.
- `frontend/src/components/__tests__/App.daemon-restart.test.tsx` (new, 3 tests).

TDD: RED commits d20cd57f and 0fc52b70 (compile and assertion failures), then GREEN commits 0c0f3d25 and 9c2c9c20.

## VM verification (home VM portty-test-2)

Two ad-hoc signed builds at 9c2c9c20 (`-X main.Version=v1.1.0-vmA` / `vmB`), installed in place:

- **G-01 PASS.** GUI B against daemon A with 2 ticking sessions showed the dialog. Not Now kept daemon 28327 (`v1.1.0-vmA`) and both shells. The tickers kept advancing. Typing into both tabs reached the shells, and no prompt came back in 25 s.
- **G-02 PASS.** After a relaunch, the dialog asked again. Restart Now left exactly one daemon reporting `v1.1.0-vmB` (ppid = GUI B). The old shells were gone, `/sessions` was `[]`, and the window showed No Open Sessions with no respawn, no banner and no `already running`.
- **G-03 PASS.** B's `portty list` gave rc 1 with the exact message, and daemon A and its tickers kept running. `--restart-daemon` gave rc 0 and the daemon became B. With zero sessions the restart stayed automatic (rc 0).
- The VM was restored: settings.json unchanged (`cmp`), and HEAD build `4fe84fcd…` (`dev`) deployed and running.

## Gate (once, at the end)

- `go build ./... && go vet ./...`: clean.
- `go test -race -short ./...`: 11 of 12 packages ok on the first run (root ok, relay 74.8 s). `internal/daemon` FAILED once. The failing test's name was cut off by the output `tail`, so it is not known. One rerun of `internal/daemon` passed (29.7 s); root was `(cached)`, i.e. passed. The known flake in that package is `TestCLIAttach_ViewingDaemonRestart` (08-REVIEW-FIX iteration 3), but that cannot be confirmed for this run.
- `pnpm vitest run`: 93 files, 1286 tests passed. `pnpm exec tsc --noEmit`: clean.
- `bash tests/check-traceability-paths.sh`: 152 paths OK. `bash tests/check-brand.sh`: clean.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `tsc --noEmit` failed on a pre-existing test file**
- **Found during:** Task 2 verify.
- **Issue:** `frontend/src/lib/webTerminalInput.test.ts` (from 358550a8) used vitest globals without importing them, so `pnpm exec tsc --noEmit` (part of the Task 2 verify) failed with TS2582/TS2304.
- **Fix:** added `import { describe, it, expect } from 'vitest'`. No behaviour change.
- **Commit:** 9c2c9c20.

### Plan-shape notes (no behaviour change)

- The plan says keeping the old daemon "returns success with a 'stale daemon kept' signal". The signal is the caller's own policy returning false, and EnsureDaemon returns nil. That avoids a sentinel error that every GUI caller would have to translate back into success.
- The GUI dialog does not block startup. The plan does not say whether it should. A blocking wait would have tripped the frontend's 90 s `startupBindingWait` on an unattended Mac. Keeping the daemon first, then asking, matches "the default is to keep the old daemon".
- `internal/daemon/client.go` (listed in files_modified) needed no change: `ListSessions` already gives the count.
- The plan lists `frontend/src/components/__tests__/` for the dialog test. The test mounts App (house modal reused), so it lives in `App.daemon-restart.test.tsx` rather than a new component file.

## Known Stubs

None.

## Threat Flags

None. No new network surface. `AnswerDaemonRestart` / `GetDaemonRestartPrompt` are local Wails bindings, and the CLI flag only widens consent for the invoking user. The WR-07 tailnet guard, the 32 KiB limits, share tokens and the CSP are unchanged.

## Self-Check: PASSED

- FOUND: internal/daemon/process.go, app.go, cmd_cli.go, main.go, cmd_attach.go, frontend/src/App.tsx, frontend/src/components/__tests__/App.daemon-restart.test.tsx, TESTING.md, 08-VM-CHECKS.md (G-03 present).
- FOUND commits: d20cd57f, 0c0f3d25, 0fc52b70, 9c2c9c20, 0e147432 (all ancestors of HEAD).
