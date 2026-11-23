---
phase: 09-upgrades-keep-sessions
plan: 05
subsystem: daemon-lifecycle
status: verification-open
status-note: "All code and targeted tests are green. The plan's single full-suite run (go test -race -short ./internal/daemon/ .) reported FAIL for internal/daemon (root ok). The failing test's name was lost (output piped through tail). Every process/EnsureDaemon/takeover test passes together in one run. The suite was not re-run, per the at-most-once rule. One re-run with saved output is needed to name the failure."
tags: [upgrade, handoff, takeover, status-pipe, debt-08, cli-routing]
requirements-completed: [UPGR-01, UPGR-03]
dependency_graph:
  requires:
    - "09-01 EnsureDaemon(ctx, socketPath, EnsureOptions{Restart, NoHandoff}), RestartQuestion.MoveFailed, HealthResponse.Handoff"
    - "09-04 runTakeover(ctx, takeoverDeps) int, moveFailedReason, serveDaemon, HandoffVersion, the timeouts block"
  provides:
    - "portty daemon --takeover (RunTakeover) with the status pipe on fd 3"
    - "startDetachedTakeover(exe) (pid int, outcome <-chan int, err error)"
    - "EnsureDaemon handoff branch with the DEBT-08 fallback, decided only from the outcome code"
  affects: [09-06 TESTING.md and the two-binary integration test, 09-07 VM rows]
tech-stack:
  added: []
  patterns:
    - "one-byte status pipe on an ExtraFiles fd; the child unsets the env var and sets close-on-exec before anything else"
    - "EnsureDaemon's process seams are passed in (ensureHooks), not package vars, so the 12 s tests run in parallel"
    - "helper-process tests through a TestMain env switch (the spawn's args are fixed: daemon --takeover)"
key-files:
  created: []
  modified:
    - internal/daemon/process.go
    - internal/daemon/process_unix.go
    - internal/daemon/process_test.go
    - internal/daemon/testmain_test.go
    - cmd_daemon.go
    - cmd_daemon_test.go
decisions:
  - "startDetachedTakeover also returns the pid (the 'still waiting' line names it); signature (pid int, outcome <-chan int, err error)"
  - "An unreported exit 0 yields 1: only a written 0 means committed"
  - "RunTakeover checks /health first; with no daemon answering it exits 1 at once, before the cert warm-up (no tailnet call, fast failure)"
  - "After any non-zero outcome, this build answering /health (version only, not relay) counts as success, so the DEBT-08 question can never offer to restart the daemon that now owns the sessions"
  - "Busy (3) waits up to readyWait for this build, then the same version check, then the busy reason"
  - "Seams via an ensureHooks struct (spawnTakeover, readyWait, noteAfter, stderr) passed to ensureDaemon; EnsureDaemon passes the production values"
metrics:
  duration: ~45m
  completed: 2026-11-19
estimate:
  tokens: 85000
  tasks: 2
actuals:
  tokens: 8450
  tasks: 2
  commits: 4
plan_head_before: f1c2d00a0f2e00ba97661331bf361a2939ed3054
plan_head_after: f69680b16d8f23aee3c18896a6803d8b5799cb80
---

# Phase 9 Plan 05: The takeover process and the EnsureDaemon handoff Summary

A new build's EnsureDaemon now starts `portty daemon --takeover` when an old daemon with live sessions speaks handoff version 1. It learns the result from a one-byte status pipe. On 0 (committed) it asks nothing. On a failure it asks the DEBT-08 question with the couldn't-move reason. It never decides on a clock, and a Quit All cancel never kills the takeover. The built binary routes the internal flag and fails cleanly when there is no daemon to take over.

## Final signatures

```go
// internal/daemon/process.go
const takeoverStatusEnv = "PORTTY_TAKEOVER_STATUS_FD"
func RunTakeover()                               // `portty daemon --takeover`
func takeoverStatus() *os.File                   // nil when run by hand
func reportTakeover(status *os.File, code int)   // nil-safe, write error ignored
func EnsureDaemon(ctx context.Context, socketPath string, opts EnsureOptions) error // unchanged
type ensureHooks struct {
	spawnTakeover func(exe string) (pid int, outcome <-chan int, err error)
	readyWait     time.Duration // daemonReadyWait
	noteAfter     time.Duration // readyTimeout + committedWatchdog + 5 s
	stderr        io.Writer
}
func ensureDaemon(ctx context.Context, socketPath string, opts EnsureOptions, h ensureHooks) error
func spawnTakeover(exe string) (int, <-chan int, error) // refuses from a test binary
func handOff(ctx context.Context, client *DaemonClient, h ensureHooks) (done bool, reason string, err error)

// internal/daemon/process_unix.go
func startDetachedTakeover(exe string) (pid int, outcome <-chan int, err error)

// cmd_daemon.go
var runTakeoverFunc = daemon.RunTakeover // case "--takeover"
```

## Status-pipe protocol as built

- Spawn: `exe daemon --takeover`, Setsid, stdout and stderr to daemon.log (as for startDetachedDaemon). The write end of `os.Pipe()` goes in `cmd.ExtraFiles[0]` (fd 3). The environment gets `PORTTY_TAKEOVER_STATUS_FD=3`. The parent closes its write end after Start.
- Child (RunTakeover), first statement: read the variable, `os.Unsetenv` it, reject fds below 3, `syscall.CloseOnExec(fd)`. Then:
  - No daemon answers `/health` on the socket: write 1, close, exit 1 (stderr: `daemon: no running daemon to take over (…)`, then `daemon: takeover failed: the new version exited`).
  - Otherwise runTakeover. 0: write byte 0, close, run serveDaemon (the process keeps running as the daemon). Non-zero: write that byte, close, exit with the code.
  - Without the variable (run by hand) nothing is written.
- Parent goroutine: reads one byte. A byte is the outcome, delivered at once (the committed child keeps running). EOF with no byte: wait for the exit and yield the exit code. A signal death (-1) or an unreported 0 yields 1. The child is always reaped with cmd.Wait. There is no kill func.

## EnsureDaemon branch as built

In the stale-version path, before the DEBT-08 policy. It runs when live sessions != 0 (-1 also tries), `!opts.NoHandoff`, and `health.Handoff == HandoffVersion`:
- Spawn error (including `os.Executable` failure and the test-binary refusal): outcome 1.
- Wait for the outcome or ctx. ctx cancelled: return `ctx.Err()` at once and leave the takeover running.
- 0: `waitForDaemon(ctx, client, readyWait, nil, "")`. Ready: nil, no question. Not ready: an error (`EnsureDaemon: the sessions moved to the new daemon, but it is not ready: …`), never the question.
- 3: the same wait. Ready: nil. Otherwise fall through.
- Any non-zero: `/health` reports this build, so nil and no question. Otherwise `EnsureDaemon: the handoff failed (outcome N: reason)` and `MoveFailed = moveFailedReason(code)` into the unchanged DEBT-08 flow (the recount question carries it too).
- Zero sessions: today's plain restart. Handoff absent (v1.1) or another version: no takeover, plain question.

"Still waiting" line, exact text (stderr, printed once at noteAfter, decides nothing):
`EnsureDaemon: still waiting for the handoff (pid N)`

## Tests added or extended (for 09-06 / TESTING.md)

| File | Tests |
|------|-------|
| internal/daemon/process_test.go | TestStartDetachedTakeover_Outcomes (report0-stay → 0 while alive, report5-exit5 → 5, exit7 → 7, sigkill → 1), TestTakeoverStatus_FromEnv, TestEnsureDaemon_HandsOff (T-D8), TestEnsureDaemon_HandoffFailsToDebt08 (codes 4, 5, 6, 1 and a spawn error × keep/restart), TestEnsureDaemon_BusyThenReady, TestEnsureDaemon_HandoffFailedButNowCurrent, TestEnsureDaemon_SlowCommitNoQuestion (12 s, parallel), TestEnsureDaemon_NoWallClockDecision (12 s, parallel), TestEnsureDaemon_V11GetsPlainDebt08 (handoff 0 and 2), TestEnsureDaemon_ZeroSessionsPlainRestart, TestEnsureDaemon_NoHandoffForKeep, TestEnsureDaemon_CancelNeverKillsTakeover (a real `sleep` process stands in for the takeover and is still alive after the cancel), TestEnsureDaemon_TakeoverRefusedFromTestBinary |
| internal/daemon/testmain_test.go | TestMain runs the fake takeover (helperTakeover) when PORTTY_TEST_TAKEOVER_HELPER is set |
| cmd_daemon_test.go | TestCmdDaemon_TakeoverRoutes |

## Verification

- Task 1 verify: `go build ./... && go vet ./internal/daemon/ .` clean. `go test -race -count=1 -run 'TestStartDetachedTakeover_|TestTakeoverStatus_|TestEnsureDaemon_HandsOff|TestHelperTakeover' ./internal/daemon/` → `ok internal/daemon 1.190s`. `go test -race -count=1 -run 'TestCmdDaemon_' .` → `ok portty 1.057s`. All three grep gates ≥ 1.
- Real launch check (temp HOME `/tmp/p9t-…`, built binary):
  - `portty daemon --takeover` → rc=1 within the 5 s alarm. stderr: `daemon: no running daemon to take over (do request: Get "http://daemon/health": dial unix /tmp/p9t-N9k1/Library/Application Support/portty/daemon.sock: connect: no such file or directory)` and `daemon: takeover failed: the new version exited`. No panic.
  - `portty daemon bogus` → rc=1, `unknown daemon subcommand "bogus"; usage: portty daemon <install|uninstall|start|stop|status|run>`, no `--takeover`.
  - `portty --help`: 0 mentions of `--takeover`. Usage goes to stderr, so the plan's stdout-only grep passed without testing anything. It was re-checked with `2>&1`: 0 of 35 lines.
  - `portty version` → `dev`.
  - No daemon.sock in the temp HOME (the config dir was empty).
  - The no-argument (GUI) launch was not run on the host. It is left to the 09-07 VM row.
- Task 2 targeted: `go test -race -count=1 -run 'TestEnsureDaemon_|TestStartDetachedTakeover_|TestTakeoverStatus_' ./internal/daemon/` → `ok 14.821s` (the two 12 s tests ran side by side). After the test fix: `-run 'TestEnsureDaemon_|TestStartDaemon_|TestWaitForDaemon_|TestStartDetachedTakeover_|TestTakeoverStatus_|TestLockDaemon_|TestSocketLost|TestRunDaemon|TestGetDaemonVersion|TestTestMain|TestTestDaemon'` → `ok 25.479s`.
- **Plan suite (run once): `go test -race -short -count=1 ./internal/daemon/ .` → `FAIL internal/daemon 62.618s`, `ok portty 8.616s`.** The failing test's name was lost because the output went through `tail -15`, which showed only viewer-diag log lines. A `--- FAIL` line printed before the last log lines (22:33:03–04) means the failure was in a sequential test that finished before the parallel phase, so it was not one of the two 12 s tests. Every test in the process/EnsureDaemon/takeover group passes in one run. The suite was not re-run, per the at-most-once rule. **Open: one re-run of `./internal/daemon/` with saved output is needed to name the failure and decide whether it is a flake or a regression.**
- `go build ./... && go vet ./...` clean, gofmt clean. csp_mw.go, capability_mw.go and tailnet/live.go are unchanged against b4d63644. `const MaxInputPayload = 32767` is unchanged. Task 2 grep gates all ≥ 1.

## Deviations from Plan

**1. [Design] Seams passed in, not package vars (`ensureHooks`)**
- The plan names a `spawnTakeover` package seam and a shortened `daemonReadyWait` var. The two 12 s tests must run in parallel. Each needs its own fake takeover, and SlowCommit needs its own stderr and note delay. Swapping globals between parallel tests is a data race. `ensureDaemon(ctx, path, opts, ensureHooks)` takes them; `EnsureDaemon` passes the production values. daemonReadyWait stays a const.

**2. [Design] startDetachedTakeover returns the pid**
- The "still waiting (pid N)" line needs it. Signature: `(pid int, outcome <-chan int, err error)`.

**3. [Rule 2] RunTakeover's /health pre-check**
- runTakeover warms the tailnet certificate before dialing (09-04 W4), which can take up to 5 + 10 s and call tailscaled. With no daemon, there is nothing to take over. RunTakeover now exits 1 at once, which also keeps the launch check off the network.

**4. [Rule 1] Unreported exit 0 maps to 1**
- Only a written 0 means committed. A child that exits 0 without reporting did not commit.

**5. [Rule 1] "Now current" checks the version only**
- The plan says to "read /health once more; this build's version means success". daemonReady also needs the relay port. If this build answered but its relay was not up, falling into DEBT-08 could offer to restart the daemon that owns the sessions. The check is `GetDaemonVersion` plus `!StaleVersion`.

**6. [Test infra] TestMain env switch instead of `-test.run=TestHelperTakeover`**
- The spawn's arguments are fixed (`daemon --takeover`). A test binary given those positional arguments would run every test. TestMain checks `PORTTY_TEST_TAKEOVER_HELPER` first. testmain_test.go is not in files_modified (3 lines).

**7. [Test] `slowBuild` leaves BuildVersion = "v2" (`shortcut:` comment)**
- A restore in one parallel test's cleanup would race the other's reads. Parallel tests run after every sequential test, and these two are the package's only ones.

**8. [Rule 1, own test] Only the live helper is signalled in cleanup**
- The first version sent SIGKILL to every helper pid in cleanup, including reaped ones. Fixed in f69680b1.

**9. [Process] Suite output lost**
- See Verification. The single suite run failed and its failing test is unnamed.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register. T-09-27 is covered by CancelNeverKillsTakeover (a real process survives the cancel), T-09-28 by TakeoverStatus_FromEnv, T-09-29 by SlowCommitNoQuestion and NoWallClockDecision, and T-09-30 by TakeoverRefusedFromTestBinary.

## Notes for later plans

- 09-06: the integration test should check that a committed takeover's shells lack `PORTTY_TAKEOVER_STATUS_FD` and fd 3 (for example `env` and `ls /dev/fd` in a session). The unit test proves the child-side calls, not a real shell.
- 09-07: run the no-argument GUI launch in the VM (it was skipped on the host).
- The takeover uses `DefaultSocketPath()`, not EnsureDaemon's socketPath argument. Every production caller passes DefaultSocketPath, the same assumption startDetachedDaemon makes.

## Self-Check: PASSED

- FOUND: internal/daemon/process.go, internal/daemon/process_unix.go, internal/daemon/process_test.go, internal/daemon/testmain_test.go, cmd_daemon.go, cmd_daemon_test.go
- FOUND (ancestors of HEAD): 04dcc3b6, fab0f715, 7ca7cd7c, f69680b1
