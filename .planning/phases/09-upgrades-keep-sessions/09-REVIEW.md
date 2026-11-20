---
phase: 09-upgrades-keep-sessions
reviewed: 2026-11-20T06:16:00Z
depth: standard
files_reviewed: 36
files_reviewed_list:
  - internal/daemon/handoff.go
  - internal/daemon/process.go
  - internal/daemon/process_unix.go
  - internal/daemon/api.go
  - internal/daemon/client.go
  - internal/daemon/engine.go
  - internal/daemon/share.go
  - internal/daemon/types.go
  - internal/daemon/handoff_vmpreview.go
  - internal/daemon/compat_vmpreview.go
  - internal/fdpass/fdpass.go
  - internal/pty/adopt_darwin.go
  - internal/pty/exitwatch_darwin.go
  - internal/pty/cleanup.go
  - internal/pty/session.go
  - internal/pty/native.go
  - internal/pty/reap_other.go
  - internal/relay/handoff_state.go
  - internal/relay/hub.go
  - internal/relay/manager.go
  - internal/relay/server.go
  - internal/relay/close_reasons.go
  - internal/relay/history.go
  - internal/webserver/server.go
  - internal/attach/loop.go
  - app.go
  - cmd_attach.go
  - cmd_cli.go
  - cmd_daemon.go
  - terminate_darwin.go
  - terminate_darwin.m
  - frontend/src/App.tsx
  - frontend/src/components/QuitConfirmModal.tsx
  - frontend/src/components/TerminalPanel.tsx
  - frontend/src/lib/hostReconnect.ts
  - frontend/src/wailsjs/go/main/App.d.ts
findings:
  critical: 1
  warning: 5
  info: 6
  total: 12
  necessity: 0
status: issues_found
---

# Phase 9: Code Review Report

**Reviewed:** 2026-11-20T06:16:00Z
**Depth:** standard (the handoff core was traced across files)
**Files Reviewed:** 36
**Status:** issues_found

## Summary

Scope: `git diff 3be899dd..HEAD -- . ':!.planning'` at HEAD f36f2631. I read every production file in the diff, and the connected code each one calls. The test files were read only for their helpers. They were not reviewed as findings targets.

The handoff mostly holds up under adversarial tracing:
- ABORT is written before any Thaw.
- The old daemon resumes after COMMIT only on the kernel-attested pid watch.
- Every fd is close-on-exec, and `fdpass.Recv` closes every fd on every error path.
- The freeze/Run/unfreeze races in `hub.go` are handled. I traced the stale-deadline and timed-out-freeze interleavings.
- The status pipe and the exit-code decision are correct.

Two probes ran as overlay files in the scratchpad, without a full suite and without touching the host's Portty:
1. **kqueue on a zombie.** `EVFILT_PROC` registration on an exited, unreaped child returns `ESRCH`. A shell that exits while the old daemon's hub is frozen therefore stays a zombie of the old daemon. Its pid cannot be reused before the takeover registers, and `WatchExit` reports it gone at once. Pitfall 9 is safe at adopt time.
2. **SESSION frame size.** I exported one 400x120 hub with truecolor output, a full history and the alt screen. Its gob frame is 7,742,898 bytes, against the 5,505,024-byte `maxSessionFrame` cap (WR-02).

The one blocker is a check-then-act race in the takeover's COMMITTED watchdog. It can make both daemons exit after a successful COMMITTED, which ends every shell (CR-01). The rest are robustness and fidelity defects. Most of them fall back to the DEBT-08 question rather than lose sessions.

**Unchanged and confirmed:**
- **CSP:** no diff line touches it.
- **Share tokens and signing key:** grant ids and tokens move verbatim (`hsShare`), and the takeover reloads the signing key from disk (`BootstrapCapabilityState`).
- **32 KiB limits:** the 32 KiB `Run` read buffer and the relay read limits are unchanged.
- **WR-07 guard:** `assertTailnetBindIP` is unchanged and still called from `StartOn`. Its premise no longer fully holds on the handoff path, though (WR-05).

## Critical Issues

### CR-01: The COMMITTED watchdog can kill the takeover after COMMITTED was delivered, so both daemons exit and every session dies

**File:** `internal/daemon/handoff.go:1131-1171` (new side), `internal/daemon/handoff.go:562-569` and `698-725` (old side)

**Issue:** `commit(true)` arms `time.AfterFunc(committedWatchdog, func(){ t.die(1); t.c.Close() })`. In production, `t.die` is `os.Exit(1)`, run on the timer goroutine. The main goroutine then calls `writeFrame(COMMITTED)`, `wd.Stop()`, and only then checks `t.dead.Load()`. Nothing makes "COMMITTED was written" and "the watchdog exits" mutually exclusive:
- If the timer fires while the write is landing, or between the write and `wd.Stop()`, `Stop` returns false. `dead` may still be false when the main goroutine reads it, and the timer goroutine calls `os.Exit(1)` regardless.
- The COMMITTED bytes are already in the old daemon's receive buffer. The old daemon's goroutine reads them, `finish()` runs, and it calls `handoffExit(0)`.
- The old side's `select` (562-573) picks `got` over `peerGone` whenever COMMITTED arrives first, which is the usual order.

**Failure scenario:**
1. An 8 GB MacBook Air under memory pressure stalls the takeover for about 5 s between receiving COMMIT and writing COMMITTED. The work in between is `serveOn`, `StartOn`, N×`hub.Start` and `watchSessionExit`.
2. The write completes at about 4.999 s, and the watchdog fires at 5.000 s.
3. The takeover exits with code 1. The old daemon reads COMMITTED and exits 0.
4. Both processes held the only copies of every PTY master, so every shell gets SIGHUP. Every session on the unattended Mac is gone. This is the one outcome the phase exists to prevent.

The window is narrow but the loss is total, and the code's own invariant ("a takeover that cannot confirm COMMITTED ... exits") is violated: this one had confirmed it.

**Fix:** make the watchdog and the write race on one atomic state, and add a last-chance check on the old side:
```go
// takeover
const (cPending int32 = iota; cWriting; cDead)
var st atomic.Int32
wd = time.AfterFunc(committedWatchdog, func() {
	if st.CompareAndSwap(cPending, cDead) { // the write never started
		t.die(1)
		return
	}
	_ = t.c.Close() // the write is in flight: unblock it; the writer decides via oldGone
})
...
if !st.CompareAndSwap(cPending, cWriting) { // the watchdog won
	t.cleanup()
	return takeoverFailed
}
err = writeFrame(t.c, frameCommitted, nil)
wd.Stop()
// err != nil -> the existing oldGone()/die path; err == nil -> committed, never die.

// old side, finish(): before closing a.lock and exiting
select {
case <-peerGone: // the takeover died after COMMITTED: do not abandon the shells
	o.resumeAfterCommit() // keep the listener copies open until this check
	return
default:
}
```
This needs `peerGone` passed into `finish`, and `closeListenerCopies` moved after the check, or made reversible. Add a fault test where the watchdog fires right after a successful COMMITTED write: the takeover must stay up, or else the old daemon must resume.

## Warnings

### WR-01: The freeze gate is a check, not a barrier, so in-flight writes land after the export

**File:** `internal/daemon/api.go:118-127` (`freezeGate`), `internal/daemon/handoff.go:479-480`

**Issue:** `freezeGate` reads `a.frozen.Load()` once at the start of a request. A mutating request that passed the check just before `a.frozen.Store(true)` keeps running during the freeze, the export and the COMMIT. Nothing waits for it, and `handoffEntries()` snapshots the sessions right after the Store.

**Failure scenarios:**
- **New session lost.** A CLI or second client sends `POST /sessions`, and it passes the gate. `engine.CreateSession` registers the session after the snapshot, so the session is never sent. The client gets 201 with the new id. On COMMITTED the old daemon's `finish()` calls `os.Exit(0)`. It was the only process holding that shell's master, so the shell gets SIGHUP and the id the client holds does not exist on the takeover.
- **Revoked links verify again.** A `RegenerateSigningKey` (the D-16 panic button) passes the gate and writes the new key to disk after the takeover's `finalize()` has run `BootstrapCapabilityState`. The takeover keeps serving with the old key, so links the user just revoked verify again. That is fail-open.
- **Share or settings change dropped.** A share toggle or settings PATCH that lands after `global()` or `loadSettingsFromDisk` is silently lost.

**Fix:** turn the gate into a drain barrier. Mutating handlers hold a read lock for their whole duration, and `serveHandoff` takes the write lock, bounded by a timeout, before it sets `frozen` and snapshots:
```go
type API struct{ ...; writeGate sync.RWMutex }
// freezeGate, mutating branch:
if !a.writeGate.TryRLock() { w.Header().Set("Retry-After", "1"); http.Error(w, "...", 503); return }
defer a.writeGate.RUnlock()
next.ServeHTTP(w, r)
// serveHandoff, before FREEZE:
if !lockWithin(&a.writeGate, time.Second) { o.rollback("freeze", "writes did not drain"); return }
a.frozen.Store(true)
defer-on-resume: a.writeGate.Unlock()
```

### WR-02: The SESSION frame cap is smaller than what Export produces, so one heavy session aborts the whole handoff

**File:** `internal/daemon/handoff.go:162`, `825`; `internal/relay/handoff_state.go:102-145`

**Issue:** `maxSessionFrame = DefaultScrollbackBytes + HistoryMaxBytes + 1 MiB` (5,505,024 bytes). It budgets only 1 MiB for everything that is neither ring nor history text. `HubState.Seed` is the rendered screen with an SGR per style change, and inside a full-screen app it carries both screens. Gob framing of up to 10,000 history strings and the wrap set also count against that 1 MiB. The sender never checks this cap (`writeFrame` checks only `maxFrame`, 64 MiB). The receiver rejects the frame with a `*frameError`, and `fail(takeoverImport)` aborts the handoff for every session.

**Measured** (overlay probe `TestProbeSessionFrameCap`): one 400x120 hub with truecolor output, a full styled history and the alt screen gives ring=262,144, history=4,186,619, seed=3,292,065 and gob=7,742,898. That is over the cap by 2.2 MB. At about 34 bytes per cell, a 250x70 truecolor TUI with a full history already exceeds it.

**Failure scenario:** the user runs btop, or a truecolor Neovim theme, in a large window. Every upgrade then reports "the new version could not load them", and the only offered path, Restart Now, ends all sessions.

**Fix:** size the cap from the real bound. Allow 2 × `maxHandoffGrid`² cells times the worst-case bytes per cell, or simply use `maxFrame`, since the peer is a same-uid process already trusted with the PTYs. Also add a sender-side size check that logs which session is too big. Add the probe above as a regression test.

### WR-03: The READY digest compares two builds' renderers, so the first release that changes rendering can never hand off

**File:** `internal/relay/handoff_state.go:398-412` (`Digest`), `internal/daemon/handoff.go:649-659`

**Issue:** `Digest` hashes `renderScreenLocked()`: the vt emulator's screen body plus `modeRestoresLocked` and `cs.suffix`. The old side computes it with the old build's emulator and render code. The new side computes it with the new build's code. `HandoffVersion` promises that "two independently installed builds speak it". Yet any later change to `screenBodyLocked`, `modeRestoresLocked`, the SGR serialisation or the `charmbracelet/x/vt` version (go.mod already changed in this phase) gives different bytes for the same seed.

**Failure scenario:** v1.3 bumps vt, which renders an attribute order differently. Every v1.2 to v1.3 upgrade then rolls back with "digest mismatch", forever. Users see the restart question on every upgrade, and the tests pass because both sides are built from one tree.

**Fix:** derive the digest from build-independent data. Hash the canonical decoded `HubState` fields on both sides: ring end and bytes, history lines and wrap flags, Seed, Tail, modes, geometry and region. The takeover hashes what it decoded, and the old side hashes what it sent. Keep the render comparison as a log-only diagnostic. Add a test that runs the digest with a deliberately different render function on one side.

### WR-04: Host-tab reconnect and the Quit dialog treat a failed daemon call as "no sessions"

**File:** `frontend/src/lib/hostReconnect.ts:17-28`, `frontend/src/App.tsx:482-490`, `app.go:841-848`, `app.go:910-918`

**Issue:** the Go bindings never reject. `App.ListSessions` returns `[]` on any error, and `App.GetRelayPort` returns `0`. The FIX doc itself relies on this ("the Go binding turns an error into []"). As a result:
- In `hostReconnectVerdict`, the `catch { return { kind: 'retry' } }` branch is dead. A failed read becomes `sessions=[]`, so `!s` returns `{ kind: 'stop', reason: 'ended' }`.
- In the Quit handler, `.catch(() => open(null))` is dead. A fast error resolves to `[]`, and the dialog says "No active sessions."

**Failure scenarios:**
1. A host tab gets 1012 from the old daemon. The re-auth's `ListSessions` fails on a pooled keep-alive connection the exiting old daemon closes mid-request, or hits the 30 s client timeout on a busy daemon. The verdict is stop, so the relay client tears down. Host tabs show no end banner (`TerminalPanel.tsx:1148` is guest-only), so the tab stays frozen and dead while the session runs on.
2. Cmd-Q while `ListSessions` fails fast shows "No active sessions." The user picks Quit Everything, believing nothing runs, and the running sessions are ended.

**Fix:** add error-returning bindings, for example `ListSessionsChecked() ([]SessionInfo, error)` and `GetRelayPortChecked() (int, error)`, so that Wails rejects the promise. Use them in `hostReconnectVerdict` and in the quit handler. Keep the old bindings for the callers that want `[]`, and update the hand-maintained `wailsjs` stubs.

### WR-05: Received listeners and the lock are not validated, so the WR-07 bind check no longer covers the handoff path

**File:** `internal/daemon/handoff.go:945-977` (`takeGlobalFiles`), `internal/webserver/server.go:353-357` (`StartOn` calls `assertTailnetBindIP`, `server.go:317-337`)

**Issue:** `takeGlobalFiles` checks only `Addr().Network()`. It never checks that:
- the API listener is bound to `socketPath`;
- the relay listener is on loopback (the relay serves unauthenticated, `Origin="local"`, full-write shells);
- the share listener is on `t.health.IP:tailnet.DefaultProbePort`;
- `files[0]` is `daemon.lock` (fstat it and compare the inode with the lock path).

`StartOn` still calls `assertTailnetBindIP`, but that checks `ws.config.BindIP`, not `raw.Addr()`. Before this phase the server bound `config.BindIP` itself, so the two were equal. Now the listener comes from another process, and the guard checks the wrong thing.

**Failure scenario:** a buggy older build, or any process answering the socket, sends a relay listener bound to `0.0.0.0`. The takeover serves the unauthenticated local relay to the LAN, with shell write access and no token. A share listener on a non-tailnet interface would likewise expose the share server outside the tailnet without a warning.

**Fix:** in `takeGlobalFiles`, reject the transfer unless each address matches what is expected:
```go
if ua := t.ln.Addr().(*net.UnixAddr); ua.Name != d.socketPath { return errors.New("API listener is not the daemon socket") }
if ta := t.relayLn.Addr().(*net.TCPAddr); !ta.IP.IsLoopback() { return errors.New("relay listener is not loopback") }
// share: ta.IP.Equal(net.ParseIP(t.health.IP)) && ta.Port == tailnet.DefaultProbePort
```
In `StartOn`, run the WR-07 check against `raw.Addr()` as well as against the config.

## Info

### IN-01: HELLO and READY are read with a 64 MiB cap before the peer is authenticated

**File:** `internal/daemon/handoff.go:433`, `533`

**Issue:** `serveHandoff` allocates up to `maxFrame` (64 MiB) for HELLO before the `peerUID` check at line 442. READY has the same cap. Any local connector can make the daemon allocate 64 MiB per connection, and refused (busy) handoffs pay it too.

**Fix:** check `peerUID` before reading HELLO. Cap HELLO at 4 KiB and READY at about `maxHandoffSessions*64` bytes.

### IN-02: The adopted pid is validated only as `> 0`, and pid 1 turns `kill(-pid)` into a broadcast

**File:** `internal/daemon/handoff.go:987`, `internal/pty/cleanup.go` (`killAdopted`, `signalGroup(-s.pid, ...)`)

**Issue:** a SESSION frame with `Pid == 1`, or a pid that is not the PTY's session leader, is accepted. `killAdopted` sends SIGHUP and then SIGKILL to `-pid`. For pid 1, that is `kill(-1, SIGKILL)`, which reaches every process the user owns.

**Fix:** reject `m.Pid <= 1`. Verify `unix.Getpgid(m.Pid) == m.Pid`, or that `tcgetsid` on the received master equals `m.Pid`, before adopting.

### IN-03: A failed adopt leaks its exit-watch thread and leaves a stale backend registry entry

**File:** `internal/pty/adopt_darwin.go:31-38`, `internal/daemon/engine.go` (`importSession` error path after `nb.Adopt`)

**Issue:**
- `WatchExit` is armed before `pollableDup`. On a dup error, the kqueue goroutine, which holds a blocked OS thread, lives until the shell exits.
- When `NewHubFromState` or `AddStaged` fails, the session stays in `b.registry`, because `Adopt` added it there.

The takeover exits after an import failure, so this is bounded. It would matter if import ever became partial.

**Fix:** arm the watch after the dup succeeds, or return a cancel. On the error path, remove the session from `b.registry`.

### IN-04: A handoff silently drops stopped sessions

**File:** `internal/daemon/engine.go` (`handoffEntries`, `GetState() == pty.StateStopped` → skip)

**Issue:** exited sessions, and the cards showing their exit code, duration and final output, vanish from the list after an upgrade. Neither 09-CONTEXT nor the summaries record this as a decision.

**Fix:** either record it as an accepted loss in CONTEXT, or move a stopped session's hub state without fds (a `Stopped` flag in `hsSession`).

### IN-05: Quit All can report success against the exiting old daemon and leave the takeover running

**File:** `internal/daemon/client.go` (`ShutdownDaemon`: `if err != nil { return nil }`), `app.go` (`QuitAll`)

**Issue:**
1. During the dual-accept window, a `/shutdown` retry can go out on a pooled keep-alive connection to the old daemon, which still answers while frozen.
2. The old daemon then exits after COMMITTED, the transport error reads as "daemon exited", and Quit All quits.
3. The takeover keeps every session. Quit Everything did not end them, which is contrary to the 09-CONTEXT rule that Quit All "waits for the handoff to finish, then stops the daemon normally".

**Fix:** after a 503 has been seen, treat a transport error as "retry on a fresh connection". Set `req.Close = true` on retries, or use a transport without keep-alive for `/shutdown`, until a 204 or the budget runs out.

### IN-06: "Restart Now" after a failed move runs the whole handoff again before restarting

**File:** `app.go:480` → `internal/daemon/process.go:281-288`

**Issue:**
1. `AnswerDaemonRestart(true)` calls `ensure()`, and `ensureDaemon` tries `handOff` again whenever `n != 0 && Handoff == 1`.
2. Every session is frozen again for up to `maxFrozen()` (17 s), and the dialog sits at "Restarting…" for that long before the restart the user chose.
3. If the second attempt succeeds, the sessions move without the user being told why the question went away.

**Fix:** pass `EnsureOptions{NoHandoff: true}` on the approved-restart path, or document the retry as intended and show it in the dialog.

---

_Reviewed: 2026-11-20T06:16:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
