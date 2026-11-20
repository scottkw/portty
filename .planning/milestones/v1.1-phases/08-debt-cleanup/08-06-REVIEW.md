---
phase: 08-debt-cleanup
plan: 06
reviewed: 2026-11-20T00:03:28Z
depth: deep
files_reviewed: 9
files_reviewed_list:
  - internal/daemon/process.go
  - app.go
  - cmd_cli.go
  - cmd_attach.go
  - main.go
  - frontend/src/App.tsx
  - frontend/src/wailsjs/go/main/App.js
  - frontend/src/wailsjs/go/main/App.d.ts
  - TESTING.md
findings:
  critical: 0
  warning: 4
  info: 8
  total: 12
  necessity: 0
status: issues_found
---

# Phase 8 Plan 06 (DEBT-08 upgrade guard): Code Review Report

**Reviewed:** 2026-11-20T00:03:28Z
**Depth:** deep (call chains traced across EnsureDaemon, the GUI poller, Retry, Quit All, the attach reconnect and the frontend dialog)
**Files Reviewed:** 9 source files, plus the tests (`process_test.go`, `app_test.go`, `cmd_cli_test.go`, `App.daemon-restart.test.tsx`) read for coverage
**Diff:** `git diff aaaf95ae..187a7ffb -- . ':!.planning'`
**Status:** issues_found

## Summary

The consent seam is in the right place. Every production `EnsureDaemon` caller passes a policy: GUI startup, the poller respawn and RetryDaemon go through `App.ensure()` and `restartConsent`, the CLI through `ensureCLIDaemon`, and the attach reconnect through an always-keep policy. The only other `ShutdownDaemon` caller is Quit All (`app.go:546`), which the Quit modal already consents to. GUI startup does not block: the policy records the prompt, emits, and returns false at once. The lock order (daemonMu, then restartMu) has no inversion. No path ends live sessions without any prompt except the narrow count-then-shutdown race (WR-02). The consent itself is weaker than the dialog says, though. "Restart Now" approves whatever the daemon holds when the restart runs, not the N sessions the dialog showed (WR-01, confirmed by a probe). The prompt can also sit unseen in a hidden window (WR-03).

Answers to the questions asked:

- **Poller respawn:** it calls `ensure` only when `Health` fails. If EnsureDaemon's own `Health` then succeeds against a mismatched daemon, the policy runs (keep, plus a re-prompt unless the user said Not Now). If the daemon is wedged, `startDaemon` waits on the old daemon's lock and never sends `/shutdown`. No unconsented kill.
- **RetryDaemon:** same policy, under `daemonMu`. No kill.
- **Attach reconnect:** `cmd_attach.go:159-161` always keeps a mismatched daemon that has live sessions. A zero-session mismatch still restarts, as before. Untested (IN-08).
- **Quit All:** unchanged. Consent comes from the Quit modal. `AnswerDaemonRestart` checks `quitting` after taking `daemonMu`, so Quit All wins when it locks first. When the restart locks first, Quit All waits it out (IN-07).
- **Count of -1:** the GUI says "Sessions may be running", the CLI says "with live sessions", and attach keeps the daemon. The fail-safe holds.
- **Event vs binding duplicate:** both set the same state (`setRestartPrompt(n)`), so one modal opens. A re-open after Not Now needs `GetDaemonRestartPrompt` to resolve after the click, and it is issued only once `startupDone` closes, a few ms after the emit. No practical defect; IN-01 covers the Go side not checking that a prompt is pending.
- **After Not Now, GUI B against an older daemon:** compatible today against the only release, v1.0. `internal/daemon/api.go:96-125` has the same routes as v1.0 (`git diff v1.0..187a7ffb` adds or removes no `HandleFunc`). No `json:"…"` tag changed in internal/daemon, internal/relay or internal/webserver. The relay protocol only gained the `MaxInputPayload` constant. MsgPing/MsgPong, which the DEBT-05 paste ack relies on, were already served at v1.0 (`internal/relay/server.go:178`), and `?resume=` was already accepted (`hub.go:835` at v1.0). The B frontend splits input into 32767-byte frames, which v1.0's 32768-byte read limit accepts. Nothing enforces this for the next release, though (IN-03).
- **CLI `--restart-daemon`:** no command passes arguments through to a shell, so a shell command cannot collide. But the flag only works after the command: `portty --restart-daemon list` launches the GUI (WR-04), and a flag-shaped value such as a rename target is consumed as consent (WR-04).
- **wailsjs stubs:** they match. `GetDaemonRestartPrompt() int` maps to `(): Promise<number>`, and `AnswerDaemonRestart(restart bool) error` maps to `(restart: boolean): Promise<void>` (an error rejects). Both use `Call('main.App.<Name>', [...])` with the right arity. The Wails-generated copy under `frontend/src/wailsjs/wailsjs/go/main/` still lacks both methods, as it lacks other methods today; App.tsx imports only the hand-maintained `./wailsjs/go/main/App`.
- **32 KiB limits, share tokens, CSP, WR-07:** unchanged. The diff touches nothing under internal/relay or internal/webserver, nor the CSP, `startupDone`, `awaitStartup` or `startupBindingWait`.

Probe (overlay `probe_restart_test.go` in the scratchpad, `go test -run TestProbeDEBT08 .`, nothing else run):
```
probe1: prompt showed 2, restart ran with 5 live sessions, allowed=true
probe2: after Not Now, prompt=0, AnswerDaemonRestart(true) still allowed=true
```

## Warnings

### WR-01: "Restart Now" approves any number of sessions, not the N the dialog showed

**File:** `app.go:311-313`, `app.go:345-347`
**Issue:** `restartConsent` returns true for every call while `restartApproved` is set, whatever `sessions` is now. The dialog's count was read when the prompt was raised, possibly hours earlier: the prompt has no timeout, and Not Now only takes effect when clicked. Sessions created in the meantime are ended without having been part of the consent. Who can create them: another GUI instance (there is no `SingleInstanceLock`, so an old GUI A still running beside GUI B can), an older CLI binary installed outside the bundle, or GUI B itself. The native ⌘T menu accelerator is not blocked by a web modal (`App.tsx:441` opens the New Session sheet regardless of `restartPrompt`). The probe confirms it: the prompt showed 2 and the approved restart ran with 5 live sessions, allowed. This breaks the plan's must-have ("a version-mismatch restart happens only after consent") in substance: the user agreed to end 2 sessions, not 5. A `-1` prompt ("Sessions may be running") followed by a successful count is approved blindly in the same way.
**Failure scenario:** At the office, GUI B shows "2 sessions are running." The user leaves the dialog up and opens a third session from old GUI A, still running. That evening they click Restart Now from screen sharing, and 3 sessions end.
**Fix:** Approve the count (or the session IDs) that was shown, not a boolean. Re-ask when it grew:
```go
// restartApproved int: the count the user agreed to; 0 = no approval
if a.restartApproved != 0 {
    if a.restartApproved < 0 || sessions <= a.restartApproved { // -1 approval only covers an unknown count
        return true
    }
    a.restartPrompt = sessions // grew since the dialog: ask again with the new number
    a.emitEvent("daemon:restart-needed", sessions)
    return false
}
```
In `AnswerDaemonRestart(true)`, set `a.restartApproved = a.restartPrompt`. Return a distinct error ("more sessions are running now") when the restart was not allowed, so the dialog stays open with the new count. Comparing ID sets is stricter. Count comparison is the minimal version.

### WR-02: Count-then-shutdown race ends a session created in between, with no prompt at all

**File:** `internal/daemon/process.go:129-134`
**Issue:** `liveSessions` reads `/sessions`. If it sees 0, the policy is skipped and `ShutdownDaemon` follows without a re-check. A session created between the two calls ends without any consent. The window is small (two local RPCs plus a log line). It is also reachable from the GUI itself: the `CreateSession` binding (`app.go:559`) does not wait for `startupDone`, while startup's `EnsureDaemon` runs on the OnStartup goroutine outside `daemonMu`, so a create issued during startup reaches daemon A while startup's restart is in flight. An old daemon's `/shutdown` takes no condition, so this cannot be made atomic against v1.0 daemons.
**Failure scenario:** CLI A or GUI A runs `portty new` just as GUI B starts. B's ListSessions returns `[]`, A's create lands, and B sends `/shutdown`. The new shell dies, and no prompt was ever shown.
**Fix:** Narrow the window and close the in-process part. (1) Have `CreateSession` call `a.awaitStartup()` and fail with `errStartupTimeout` like RetryDaemon, so the GUI cannot race its own startup restart. (2) Re-list right before `ShutdownDaemon`, and treat a non-zero result as a fresh consent question:
```go
if n := liveSessions(client); n != 0 && !allowRestart(ver, n) { ...keep... }
if n2 := liveSessions(client); n2 > n && !allowRestart(ver, n2) { ...keep... }
_ = client.ShutdownDaemon()
```
(3) For future daemons, add a server-side `POST /shutdown?ifSessions=<n>` that refuses when the live count differs, and use it when the daemon supports it. Note the remaining window in TESTING/threat model T-08-30.

### WR-03: The consent prompt can sit unseen in a hidden window, and the update silently does not take effect

**File:** `app.go:308-320` (`restartConsent`), `app.go:218-241` (`domReady`)
**Issue:** `restartConsent` only emits to the webview. With "start minimized" on (`domReady` skips `WindowShow`, Dock icon hidden), or after Quit GUI Only hid the window, the dialog renders in a window nobody sees. The old daemon keeps running with its old bugs. Every CLI command on that Mac refuses meanwhile (IN-04). The tray and notifications say nothing. Threat T-08-32's mitigation is an "explicit, visible choice", and that does not hold here. The VM checks (G-01/G-02) covered only a visible window.
**Failure scenario:** The office Mac has start-minimized on. The user installs B and launches it, and nothing appears. Later, from home, `portty attach office:…` works through the old daemon, while `portty list` on the office Mac refuses with "the running daemon is version …". No UI ever explained why.
**Fix:** On the first prompt of a launch, surface it natively: show the window and Dock icon (`runtime.WindowShow(a.ctx); a.setDockVisible(true)`, as `beforeClose` does for the quit modal). Or send a notification through `a.sendNotificationFunc` ("Portty was updated. 2 sessions are still running on the old background service. Open Portty to restart it."). Add a tray menu line while `restartPrompt != 0`. Add an M-104 step with start-minimized on.

### WR-04: `--restart-daemon` before the command launches the GUI, and a flag-shaped argument value is consumed as consent

**File:** `main.go:55`, `main.go:191-192`, `cmd_cli.go:76-77`
**Issue:** (a) `main` routes any first argument starting with `-` to `runGUI()`. So `portty --restart-daemon list`, a natural reading of the refusal's "Re-run with --restart-daemon", opens a GUI instance from the terminal instead of running `list`. That GUI instance then shows its own consent dialog, or keeps the daemon. The CLI user gets neither the command nor the restart. The `forceRestart`/`slices.Contains(args, …)` scan in `runCLI` can never see the flag in `args[0]`. If runCLI were ever reached with `args == ["--restart-daemon"]`, `cmdArgs := args[1:]` (`main.go:214`) would panic after the forced restart. (b) The flag is stripped from every position, including argument values: `portty rename <id> --restart-daemon` restarts the daemon (ending its sessions) and then fails with "usage: portty rename <session-id> <name>". The same goes for `portty new --restart-daemon` as a path. No command forwards a shell command, so `attach host:session --restart-daemon` simply restarts the local daemon, which matches the message.
**Failure scenario:** The user runs `portty list` on an upgraded Mac, reads the refusal, and types `portty --restart-daemon list`. A second Portty window opens, and the daemon is neither restarted nor listed.
**Fix:** Make the message say where the flag goes: `Re-run as 'portty <command> --restart-daemon' to restart anyway.` In `main`, send a first argument equal to `--restart-daemon` to `runCLI` with the flag moved after the command, or print usage and exit 2. Strip the flag only from flag positions: stop at `--`, and never strip the argument that fills a positional slot of `rename`/`new`. The simplest version accepts the flag only as the last argument:
```go
forceRestart := len(args) > 1 && args[len(args)-1] == restartDaemonFlag
if forceRestart { args = args[:len(args)-1] }
```
and documents it as `<command> … --restart-daemon` (last).

## Info

### IN-01: `AnswerDaemonRestart(true)` does not check that a prompt is pending, and it overrides Not Now

**File:** `app.go:333-348`
**Issue:** The binding sets `restartApproved` and restarts whether or not `restartPrompt != 0`, and ignores `restartDeclined`. Probe 2 shows that after Not Now (prompt 0, declined), a later `AnswerDaemonRestart(true)` is still allowed to end the sessions. Today only the dialog's confirm calls it, so this is defense in depth. It does matter if the late `GetDaemonRestartPrompt` re-open race ever wins, or if a future caller reuses the binding.
**Fix:** Take `restartMu` first and return `errors.New("no restart pending")` when `a.restartPrompt == 0`, before taking `daemonMu`. Combine this with WR-01's count approval.

### IN-02: After a failed "Restart Now", the dialog keeps saying "N sessions are running" although the old daemon was already told to stop

**File:** `app.go:348-356`, `frontend/src/App.tsx:1321-1324`
**Issue:** On failure, `ShutdownDaemon` has usually already been sent (the error comes from `startDaemon`, after `/shutdown`). The sessions are gone, but `restartPrompt` stays at N and the dialog still says they are running, now with an error line. No `daemon:error` is emitted, and the poller then respawns on its own schedule.
**Fix:** On error, re-read the state: if `a.client.Health()` fails, clear the prompt and emit `daemon:error` with the restart error, so the normal banner and Retry flow take over. Otherwise keep the prompt with a refreshed count. Add a vitest for the rejected `AnswerDaemonRestart(true)` (error shown, buttons enabled).

### IN-03: Not Now keeps a stale daemon with nothing checking API compatibility. Compatible with v1.0 today; the next route or payload change breaks silently

**File:** `internal/daemon/process.go:129-132`, `internal/daemon/process.go:214-218`
**Issue:** `staleVersion`'s own comment says the version restart exists because "a stale daemon from an older release won't have newly added API routes (e.g. start-minimized)". Not Now now keeps such a daemon for the whole launch. Against v1.0 it works (see Summary: identical routes in `api.go:96-125`, no JSON tag changes, ping/pong and resume present at v1.0). The first release that adds a route (say `GET /settings/foo`) or changes a payload gets 404s or decode errors in GUI B after Not Now, with no indication why.
**Fix:** Add an `apiVersion` (integer) to `/health`. EnsureDaemon offers "keep" only when the old daemon's API version is at least the minimum this build needs. Otherwise the dialog should say the update cannot run beside the old service. Alternatively, add a test that pins the route list and JSON field sets, so a change forces a deliberate decision.

### IN-04: The CLI has no "keep" answer, so after a GUI Not Now every CLI command on that Mac refuses, including `health` and `--help`

**File:** `main.go:206-211`, `cmd_cli.go:65-86`
**Issue:** `ensureCLIDaemon` runs before dispatch for every command except `daemon …`. With a mismatched daemon holding sessions, `portty attach <local>`, `portty list`, `portty daemon status`, `portty health` (which never uses the daemon, `cmd_cli.go:251`) and `portty <cmd> --help` all exit 1 until someone ends the sessions. The GUI can keep working against the old daemon; the CLI cannot.
**Fix:** Add `--keep-daemon` (the policy returns false, and the command proceeds against the old daemon with a one-line warning). Skip `ensureCLIDaemon` for `health` and for any `--help`/`-h` argument.

### IN-05: A daemon that recovers after a failed startup is adopted without the mismatch check or prompt

**File:** `app.go:397-409` (`respawnDaemonIfDown`, IN-01 branch)
**Issue:** If startup's EnsureDaemon failed (old daemon wedged, lock wait timed out) and the old daemon later answers, the poller emits `daemon:recovered` without going through `ensure`. GUI B then runs against the old version with no prompt, and the user is never told the update is not in effect. No sessions end, but the guard's "visible choice" is skipped.
**Fix:** In that branch, call `daemon.GetDaemonVersion()` with `staleVersion`. If it is stale, run the policy path: `a.ensure()(…)` under the `daemonMu` already held, or call `restartConsent(ver, n)` directly.

### IN-06: A transient `/health` failure between `Health()` and `GetDaemonVersion()` raises a false "update" prompt for a same-version daemon

**File:** `internal/daemon/process.go:126-128`, `internal/daemon/client.go:49-55`
**Issue:** `GetDaemonVersion` returns `""` on any error, and `staleVersion("")` is true for release builds. The GUI then asks to "Update Portty's background service?" for a daemon that is current. The CLI refuses with "the running daemon is version  with 2 live sessions" (an empty version). This predates 08-06 (it used to restart silently), and 08-06 makes it visible.
**Fix:** Make `GetDaemonVersion` return `(string, error)`. On error, treat the daemon as not stale (`needsSpawn = false`) or retry once. Do not treat it as a mismatch.

### IN-07: Quit All can wait up to about 30 s behind an approved restart, then stop the daemon it just spawned

**File:** `app.go:340-357`, `app.go:539-549`
**Issue:** When "Restart Now" takes `daemonMu` first, Quit All sets `quitting` and blocks on `daemonMu` while `EnsureDaemon` sends `/shutdown`, waits for the lock (up to 15 s) and for readiness (up to 15 s). The app quits only after that, and then shuts down the newly spawned daemon B. Both actions had consent, so nothing ends wrongly, but the window lingers with "Restarting…" and a daemon is spawned only to be stopped.
**Fix:** Optional. In the approved path, check `a.quitting.Load()` again after `ShutdownDaemon` and before spawning. That needs a hook in EnsureDaemon (for example a `ctx` or a `shouldSpawn func() bool`). Or accept it and document it.

### IN-08: Test gaps around the guard's wiring

**File:** `cmd_attach.go:159-161`, `frontend/src/components/__tests__/App.daemon-restart.test.tsx`, `app_test.go:951-1012`
**Issue:** Every test swaps `ensureLocalDaemon` or `ensureDaemonFunc`, so (a) no test pins the attach reconnect's always-keep policy, (b) no test runs `App.ensure()` with `ensureDaemonFunc == nil` to prove the GUI's default seam passes `restartConsent`, and (c) no vitest covers a rejected `AnswerDaemonRestart(true)` (the error line, buttons re-enabled, dialog kept). A future edit that passes `nil` or `func(string,int) bool { return true }` at any of these sites passes the suite.
**Fix:** Expose the attach policy as a named func (`keepMismatchedDaemon`) and assert it returns false for `n = 2` and `n = -1`. Add a Go test that `(&App{}).ensure()` reaches `restartConsent`, for example via a package-level `ensureDaemon = daemon.EnsureDaemon` seam that records the policy it was given. Add the rejected-confirm vitest case.

---

_Reviewed: 2026-11-20T00:03:28Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
