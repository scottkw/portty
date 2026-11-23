---
phase: 09-upgrades-keep-sessions
plan: 01
subsystem: daemon-lifecycle
status: complete
tags: [upgrade, daemon, api-version, quit, sigterm, logout, cli, gui]
requirements-completed: [UPGR-04, UPGR-05]
requirements-partial: [UPGR-03]
dependency_graph:
  requires: ["08-06 DEBT-08 consent guard (RestartPolicy, restart dialog)"]
  provides:
    - "GET /health api: 2 and a handoff field (left 0 for 09-04)"
    - "DaemonClient.HealthInfo, DaemonAPI, MinDaemonAPI (var, default 1)"
    - "EnsureDaemon(ctx, socketPath, EnsureOptions{Restart, NoHandoff}); RestartQuestion{DaemonVersion, Sessions, MoveFailed}"
    - "startDetachedDaemon(exe) (exited <-chan struct{}, kill func(), err error)"
    - "bounded 503 + Retry-After wait (frozenRetryBudget = 20 s) on mutating doJSON calls and ShutdownDaemon"
    - "GetDaemonRestartMoveFailed and GetDaemonUpdateNeeded bindings"
  affects: [app.go startup/ensure/QuitAll, cmd_cli.go ensureCLIDaemon, cmd_attach.go ensureLocalDaemon, frontend/src/App.tsx]
tech-stack:
  added: []
  patterns:
    - "Go keeps state + emits an event, frontend pulls on load (GetDaemonUpdateNeeded, like GetDaemonRestartPrompt)"
    - "one cancellable ctx per GUI ensure, stored under ensureMu; QuitAll cancels it before taking daemonMu"
    - "ObjC method swizzle of one Wails delegate method, original IMP kept for every non-system quit"
key-files:
  created:
    - internal/daemon/compat_vmpreview.go
    - terminate_darwin.go
    - terminate_darwin.m
    - frontend/src/components/__tests__/App.update-needed.test.tsx
  modified:
    - internal/daemon/types.go
    - internal/daemon/api.go
    - internal/daemon/api_test.go
    - internal/daemon/client.go
    - internal/daemon/client_test.go
    - internal/daemon/process.go
    - internal/daemon/process_unix.go
    - internal/daemon/process_test.go
    - cmd_cli.go
    - cmd_cli_test.go
    - cmd_attach.go
    - cmd_attach_test.go
    - app.go
    - app_test.go
    - frontend/src/App.tsx
    - frontend/src/components/__tests__/App.daemon-restart.test.tsx
    - frontend/src/wailsjs/go/main/App.js
    - frontend/src/wailsjs/go/main/App.d.ts
decisions:
  - "APIVersion = 2; an absent api reads as 1 (v1.0/v1.1). MinDaemonAPI stays 1, so a v1.1 daemon kept with Not Now or --keep-daemon keeps working"
  - "The 503 wait applies only to a 503 that carries Retry-After (09-04's freeze sends Retry-After: 1). A bare 503 (Tailscale off on share/peer routes) is returned at once"
  - "The GUI's post-ensure API check and the cancellable ctx live in App.ensure()'s default path, so startup, RetryDaemon, AnswerDaemonRestart and the respawn recovery all get both; the ensureDaemonFunc test seam skips both"
  - "QuitAll keeps the literal runtime.Quit(a.ctx) (tray_test.go source-inspects it); quitFunc is a nil-by-default test seam in front of it"
  - "Logout/restart/shutdown: NSTerminateNow when NSWorkspaceWillPowerOffNotification was seen or the quit Apple event's kAEQuitReason is a logout/restart/shutdown code; everything else calls Wails' original applicationShouldTerminate:"
metrics:
  duration: 25m
  completed: 2026-11-19
estimate:
  tokens: 130000
actuals:
  tokens: 19700
  tasks: 3
  commits: 6
plan_head_before: 3be899ddad41246dacf8dba1186f88563a11e71e
plan_head_after: d1e2d88903e34d3235c8363aaefb5da0753aaf84
---

# Phase 9 Plan 1: Lifecycle groundwork Summary

/health now reports API version 2. A CLI or GUI facing a daemon below MinDaemonAPI says "update needed". EnsureDaemon takes a ctx that Quit All cancels, and a fresh child spawned by a cancelled ensure is killed. The client waits out a frozen daemon's 503 for up to 20 s. The restart question can carry a couldn't-move reason, which both front ends show. SIGTERM quits only the GUI. A macOS logout, restart or shutdown is no longer cancelled. Only one modal is ever shown at a time.

## What was built

### Contract for 09-04 and 09-05 (final signatures)

```go
// internal/daemon/types.go
const APIVersion = 2
type HealthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version,omitempty"`
	API     int    `json:"api,omitempty"`     // absent: 1
	Handoff int    `json:"handoff,omitempty"` // left 0 here; 09-04 sets it
}

// internal/daemon/client.go
func (c *DaemonClient) HealthInfo() (HealthResponse, error)
func DaemonAPI(h HealthResponse) int        // h.API, or 1 when 0
var MinDaemonAPI = 1
var frozenRetryBudget = 20 * time.Second    // > 09-04 freezeTimeout + readyTimeout + committedWatchdog (~12 s)

// internal/daemon/process.go
type RestartQuestion struct {
	DaemonVersion string
	Sessions      int    // -1: unknown
	MoveFailed    string // "" unless a handoff was tried and failed (09-05 sets it)
}
type RestartPolicy func(RestartQuestion) bool
type EnsureOptions struct {
	Restart   RestartPolicy
	NoHandoff bool // set by ensureCLIDaemon for --keep-daemon; 09-05's handoff branch reads it
}
func EnsureDaemon(ctx context.Context, socketPath string, opts EnsureOptions) error
func startDaemon(ctx context.Context, client *DaemonClient, wait time.Duration, lockPath string,
	spawn func() (exited <-chan struct{}, kill func(), err error)) error
func waitForDaemon(ctx context.Context, client *DaemonClient, wait time.Duration, exited <-chan struct{}, lockPath string) error

// internal/daemon/process_unix.go
func startDetachedDaemon(exe string) (exited <-chan struct{}, kill func(), err error)
// kill: SIGTERM to the started process; a no-op once exited has closed
```

Cancellation rules as built:
- A cancelled ctx returns `ctx.Err()` before the test-binary refusal and before any spawn.
- startDaemon's lock wait checks ctx on every 50 ms tick, and checks it once more before calling spawn.
- When waitForDaemon fails because ctx was cancelled, startDaemon calls that spawn's `kill` once and returns `ctx.Err()`.
- 09-05's takeover spawn must not pass a kill func that ends a takeover child. Its spawn path should return a nil `kill`, which startDaemon allows.

main.go is unchanged: `ensureCLIDaemon(daemon.EnsureDaemon, …)` compiles with the new signature.

### Bindings (hand-maintained stubs updated in the same commits)
- `GetDaemonRestartMoveFailed(): Promise<boolean>`. True while the open restart question follows a failed handoff.
- `GetDaemonUpdateNeeded(): Promise<string>`. The version of a daemon below MinDaemonAPI, or "". It waits for startup, as GetDaemonError does.
- New event `daemon:update-needed` (payload: the daemon version). `daemon:restart-needed` keeps its number payload.

### Exact copy strings
- CLI update needed (runCLI adds the `portty: ` prefix, then exits 1): `the running daemon (version X, API N) is too old for this portty (version Y needs API M): update needed. Re-run as 'portty <command> --restart-daemon …' to restart it.`
- CLI couldn't-move (stderr, printed before the DEBT-08 refusal, restart or keep line): `portty: couldn't move the running sessions to version Y (reason); they are still running on version X.`
- GUI restart dialog, first sentence when MoveFailed: `Portty couldn't move your sessions to the new version. They're still running on the old version.` The existing count line follows. Title, buttons and default (Not Now) are unchanged.
- GUI banner: title `Update needed`, body `Portty's background service (version X) is too old for this version of the app. Relaunch Portty to update it.` It has no action button.

### Quitting
- SIGTERM: App.startup runs `signal.Reset(syscall.SIGTERM)` (Wails' `signal.Start` ran before OnStartup), then installs Portty's own Notify. `onTerminate` sets quitting and quits through the `quitFunc` seam (`runtime.Quit` in production), and never stops the daemon. SIGINT stays with Wails.
- Logout/restart/shutdown: `terminate_darwin.m` replaces `applicationShouldTerminate:` on the Wails delegate class, and keeps the original IMP for Cmd-Q and the menu. The `//export porttySystemWillPowerOff` callback sets quitting. A `shortcut:` comment names Wails v2.10.2 AppDelegate.m:26-29.
- One modal: `app:quit-requested` while the restart dialog is open answers `AnswerDaemonRestart(false)`, clears the prompt, then runs the quit flow. QuitConfirmModal also requires `restartPrompt === 0` before it renders.

## Tests added or extended (for 09-06 / TESTING.md)

| File | Tests |
|------|-------|
| internal/daemon/api_test.go | TestAPIHealth (extended: `"api":2`, no `handoff`) |
| internal/daemon/client_test.go | TestHealthInfo, TestDaemonClient_Waits503 |
| internal/daemon/process_test.go | TestEnsureDaemon_CancelBeforeSpawn, TestStartDaemon_CancelDuringLockWait, TestStartDaemon_CancelAfterSpawnKillsChild; TestEnsureDaemon_StaleRestartConsent (now asserts MoveFailed is empty); existing startDaemon/waitForDaemon/EnsureDaemon tests moved to the new signatures |
| cmd_cli_test.go | TestEnsureCLIDaemon_UpdateNeeded, TestEnsureCLIDaemon_MoveFailedCopy; TestEnsureCLIDaemon_RestartConsent (new signature) |
| cmd_attach_test.go | swapEnsureDaemon and TestEnsureLocalDaemon_KeepsMismatchedDaemon (new signature) |
| app_test.go | TestQuitAll_CancelsRunningEnsure, TestRestartConsent_MoveFailed, TestOnTerminate_QuitsGUIOnly, TestSystemPowerOff_AllowsQuit, TestDaemonUpdateNeeded; DEBT-08 tests use a `question(n)` helper |
| frontend/src/components/__tests__/App.daemon-restart.test.tsx | one-modal quit, couldn't-move first sentence, count-line-only copy |
| frontend/src/components/__tests__/App.update-needed.test.tsx (new) | banner at load, banner on event, no banner when compatible |

## Verification

- `go build ./... && go vet ./...` clean. `go vet -tags vmpreview ./internal/daemon/` clean.
- `go test -race -short -count=1 ./internal/daemon/ .` (the plan's one full run): `ok internal/daemon 35.741s`, `ok portty 8.589s`.
- `pnpm vitest run`: 94 files, 1294 tests passed. `pnpm exec tsc --noEmit` clean.
- Wails build: `Generating bindings: Done.`, 0 panics, `Built …/build/bin/Portty.app/Contents/MacOS/Portty`. frontend/src/wailsjs restored. `git status --porcelain -- frontend/src/wailsjs` is empty.
- Daemon-free launch check on the final code: `portty version` printed `dev`, and `portty --help` printed the usage.
- CSP middleware, capability middleware and tailnet guard are unchanged against b4d63644. `MaxInputPayload = 32767` is unchanged. main.go is unchanged. `tests/check-brand.sh` passes.
- Every task's targeted `-run` verify block passed. All grep acceptance checks passed.

## Deviations from Plan

### Auto-fixed / adjusted

**1. [Rule 2 - Correctness] Only a 503 that carries Retry-After is waited out**
- **Found during:** Task 2
- **Issue:** The plan said to retry every 503 on mutating calls. The daemon already answers 503 for "Tailscale off" on share and peer-connect routes. Those use their own request code today, but a blind retry would turn that message into a 20 s stall if they ever moved onto doJSON.
- **Fix:** A 503 is retried only when it carries `Retry-After`. An unreadable value still waits the default 1 s, capped at 2 s. 09-04's freeze must send `Retry-After` (its plan already does).
- **Files:** internal/daemon/client.go. **Commit:** 16a03818

**2. [Rule 1 - Edge case] An empty daemon version reads as "unknown" in the Update needed state**
- **Issue:** "" means "compatible" in the binding contract, so a too-old daemon that reports no version would have shown no banner.
- **Fix:** `cmp.Or(h.Version, "unknown")`. **Commit:** d1e2d889

**3. [Rule 2] GetDaemonUpdateNeeded waits for startup**
- Like GetDaemonError: startup's `daemon:update-needed` can fire before React subscribes. **Commit:** d1e2d889

**4. Constraint-driven: no `git clean` in the worktree**
- The plan's verify block runs `git clean -fdq -- frontend/src/wailsjs`. The executor rules forbid `git clean` in a worktree. Instead, the three modified generated files were restored with `git checkout -- <path>`, and the folder the build generated (ignored `frontend/src/wailsjs/wailsjs/go/main/`, created by this build) was removed with `rm -rf` on that path.

**5. Environment**
- Ran `pnpm install --frozen-lockfile --offline` in the worktree so vitest could run. No package was added or changed.
- The Wails log went to the session scratchpad instead of /tmp.

## Known Stubs

None. `HealthResponse.Handoff` is deliberately left 0 (09-04 sets it when the handoff route exists). `EnsureOptions.NoHandoff` and `RestartQuestion.MoveFailed` are wired at every caller and are read or set by 09-05.

## Threat Flags

| Flag | File | Description |
|------|------|-------------|
| threat_flag: process-signal | internal/daemon/process_unix.go | New kill func sends SIGTERM to the child this call spawned (T-09-01). It is only called on a cancelled ctx after waitForDaemon fails, and it is a no-op once the child has been reaped |
| threat_flag: appkit-terminate | terminate_darwin.m | The swizzled applicationShouldTerminate: answers NSTerminateNow for system quits (T-09-03). The AppKit behaviour is unverified on the host; 09-07 must check it in the VM (a restart issued with the GUI open must reboot the VM) |

## Notes for later plans

- `waitForDaemon` and the lock wait check ctx between polls, but `daemonReady`'s client calls are not ctx-aware. A daemon that accepts connections and then hangs could delay a Quit All cancel by up to the client's 30 s timeout per call. With a refusing socket or a responsive daemon, the cancel takes effect within one tick.
- The CLI update-needed message suggests `--restart-daemon`. In a `dev` build StaleVersion is always false, so that flag never restarts the daemon. Release builds are unaffected.

## Self-Check: PASSED

- FOUND: internal/daemon/compat_vmpreview.go, terminate_darwin.go, terminate_darwin.m, frontend/src/components/__tests__/App.update-needed.test.tsx
- FOUND commits: da8856b5, b3549cda, 2341bf6b, 16a03818, e30fd591, d1e2d889
