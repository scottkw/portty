---
phase: 05-overnight-survival
iteration: 2
reviewed: 2026-11-18T08:46:31Z
depth: standard
files_reviewed: 13
files_reviewed_list:
  - app.go
  - frontend/src/App.tsx
  - frontend/src/components/WebShareSessionView.tsx
  - frontend/src/lib/relayClient.ts
  - internal/attach/loop.go
  - internal/daemon/process.go
  - internal/daemon/process_unix.go
  - internal/relay/hub.go
  - internal/relay/protocol.go
  - internal/relay/server.go
  - internal/webserver/server.go
  - internal/relay/join_test.go
  - internal/attach/loop_test.go
findings:
  critical: 0
  warning: 3
  info: 2
  total: 5
  necessity: 0
status: issues_found
---

# Phase 5: Code Review Report (iteration 2, re-review of the fix commits)

**Reviewed:** 2026-11-18T08:46:31Z
**Depth:** standard, plus cross-file tracing of the replay/flush, open/input, quit/respawn and daemon-lock paths
**Files Reviewed:** 13 source files changed by `069e172c..ac7ccc2c` (`git diff 54415d9f ac7ccc2c -- . ':!.planning' ':!TESTING.md'`). I read the surrounding code too: `TerminalPanel.tsx`, `cmd_attach.go`, `internal/daemon/socket.go`, `api.go`, `service.go` and `cmd_daemon.go`. Changed test files were read only to check what they prove.
**Status:** issues_found

## Summary

Most of the eight fixes are correct, and I found no Critical issue. Three Warnings remain. Each one is either a gap in a fix or a new problem a fix introduced:

- **WR-01.** The WR-03 fix opens input at the host's grid frame, which comes before the replay. The host does not read input until the replay and the pending flush have been written. So keystrokes typed during a long full replay are still queued and then delivered late into the live shell. The WR-03 failure mode has moved from the proxy-dial window to the overnight full-replay window.
- **WR-02.** The IN-01 fix runs `retryInit` automatically. Until now `retryInit` was reachable only from a banner that renders only when there are no tabs. It replaces the whole tab list, so it can now drop open peer tabs and switch the active tab.
- **WR-03.** The WR-05 lock turns an orphaned daemon into a permanent outage. `EnsureDaemon` still unlinks the socket without holding the lock, so the original dial-then-remove race is still there. An orphan now holds the lock forever, so no replacement daemon can ever start.

Gate re-run with `GOTOOLCHAIN=go1.26.11`:

- `go vet` is clean on relay, attach, daemon and the root package.
- `go test -race -count=1 ./internal/relay ./internal/attach`: ok.
- `go test -race -short -run 'TestLockDaemon|TestRespawn|TestDaemonError|TestQuit|TestBeforeClose' ./internal/daemon .`: ok.
- `vitest run` on relayClient, WebShareSessionView, App.daemon-recovered and TerminalPanel.reconnect: 4 files, 100 tests passed.

### Answers to the focus questions

**WR-01 pending buffer: correct.**

- **Exactly-once.** `joinLocked` sets `replaying` before `Subscribe` publishes the subscriber, under `emuMu` (`hub.go:873-874`). `Run` records and broadcasts under the same `emuMu`, so each frame goes to exactly one place: the replay, `pending`, or `Msgs`.
- **No reordering.** `TakePending` ends replay mode only when it finds `pending` empty, under `hub.mu`, in the same critical section that `sendLocked` uses (`hub.go:262-270`). Every pending frame therefore comes before every `Msgs` frame, and all of them come after the offset meta. The only other writers to `Msgs` are the read-pump pongs, and the read pump starts after the flush.
- **Offsets.** `End` is captured under `emuMu`. The pending frames are exactly the frames after `End`. Clients count them like live frames, because they arrive after the offset meta.
- **Cap.** The 4 MiB cap is checked per frame, and going over it kicks the viewer.
- **Cleanup.** A viewer that disconnects mid-replay returns, `Unsubscribe` drops it, and `pending` is garbage-collected with it.
- **Lock order.** `TakePending` takes only `hub.mu`. `sendLocked` runs under `hub.mu`, sometimes inside `emuMu`. That keeps the order emuMu then hub.mu, with no inversion.
- **Flush loop.** It ends once a batch drains faster than output arrives. If output arrives faster than the link can carry it, the cap kicks instead.

**WR-02/WR-03 "open on first host frame".**

- **A frame always arrives on join.** `Hub.Cols()`/`Rows()` fall back to 220×50 and are never 0, so both handlers always write the grid frame first (`relay/server.go:108-112`, `webserver/server.go:833-837`). After it come the replay meta and the offset meta. A host that sends nothing for a while is therefore never mistaken for a dead one.
- **Old hosts.** The grid frame has existed since VIEW-03 (Phase 157). Output that arrives before any replay meta invalidates the offset in both clients (`relayClient.ts:301-305`, `loop.go:310-312`). Old hosts are therefore handled.
- **Dropped, not queued (client side).** `sendInput` returns unless the state is `open` (`relayClient.ts:484`). The CLI marks chunks stale until the first frame arrives (`loop.go:292-296`).
- **Queued on the host side.** The window between the grid frame and the end of the replay still queues input, on the host. See WR-01.
- **Surfaces agree.** GUI, browser and CLI all use "first frame". The CLI has no connect timer, but that is fine: the local proxy's 10 s upstream dial, plus its upstream heartbeat, bounds it.

**WR-04 quitting flag: no deadlock.**

- `respawnDaemonIfDown` only `TryLock`s, so it never waits on `QuitAll`.
- `RetryDaemon` `Lock`s, but it never runs while holding anything `QuitAll` needs.
- `QuitAll` is never re-entered while `daemonMu` is held.
- The second `quitting` check after `TryLock` is needed. It covers a `QuitAll` that takes and releases the lock while the poller's `Health()` call is in flight.
- One small gap remains: `RetryDaemon` ignores `quitting` (IN-01 below).

**WR-05 flock.**

- **Version-mismatch restart works.** The old daemon `os.Exit`s 50 ms after replying (`api.go:281-289`). `EnsureDaemon` sleeps 200 ms before it spawns, so the lock is already free and the new daemon takes it at once. The new daemon's wait of up to 2 s fits inside `EnsureDaemon`'s 3 s readiness poll.
- **Stale lock file.** It is harmless. `O_CREATE` reopens it, `flock` is released when the process dies, and nothing depends on the file being absent.
- **launchd.** `portty daemon` (the launchd plist) goes through `RunDaemon`, then `runDaemonCore`, so it is covered. A launchd instance that loses the lock exits after 2 s. Before this fix it also exited, at `CleanupStaleSocket`. With KeepAlive=false there is no restart loop.
- **The lock does not leak to shells.** It is opened close-on-exec, so PTY children never inherit it.
- **Residual.** The lock covers daemon-against-daemon, but not client-side unlinks (WR-03 below).

**WR-06 timers.**

- **Cleared on close.** `teardown` clears the connect timer, and both `close()` and `stop()` call `teardown`. The timer is also cleared at `onclose` and at the first frame.
- **When it stops.** The connect timer stops at the first frame (the grid frame), not at the end of the replay. So a slow but healthy full replay can never be killed by it.
- **Probe abort.** The `probeShare` abort rejects into `{ kind: 'retry' }`.
- **Browser support.** See IN-02 below.

**IN-01 / IN-08.**

- IN-08 is resolved: `atomic.Pointer[error]`, and the `-race` test passes.
- IN-01 is resolved as designed, but the automatic `retryInit` introduces WR-02.

## Original findings: resolution status

| ID | Status | Notes |
|----|--------|-------|
| WR-01 | **Resolved** | Pending buffer verified: exactly-once delivery, offsets, cap, cleanup and lock order (see above). |
| WR-02 | **Resolved** | Both clients keep the offset until the new socket's replay meta, or its first output if the host sent no meta. |
| WR-03 | **Partially resolved** | The proxy-dial window is closed. Input is still queued on the host during the replay window (new WR-01). |
| WR-04 | **Resolved** | No deadlock. Small residual in IN-01 (Retry ignores `quitting`). |
| WR-05 | **Resolved for daemon-against-daemon** | Residual: client-side unlink plus orphan lock-holder (new WR-03). |
| WR-06 | **Resolved** | Timers are cleared everywhere, and the connect timer stops at the first frame. |
| IN-01 | **Resolved, with a regression** | The automatic recovery replaces the tab list (new WR-02). |
| IN-08 | **Resolved** | |
| IN-02 to IN-07 | **Still open** (outside the iteration-1 fix scope, not counted below) | IN-02: the poller overrides a deliberate `daemon stop`. IN-03: no heartbeat or read pump during the replay write; the pending flush now extends that window slightly. IN-04: a peer tab's offset is dropped on tab switch. IN-05: ESC[3J and raw replays. IN-06: dead `RenderSnapshot`/`EnsureLiveEmulator`. IN-07: the `startupDone` guard. |

## Narrative Findings (AI reviewer)

## Warnings

### WR-01: Input opens at the grid frame, before the replay, but the host reads no input until the replay and the pending flush are written, so keystrokes typed during a long full replay are queued and delivered late

**File:** `frontend/src/lib/relayClient.ts:264-268`, `frontend/src/components/TerminalPanel.tsx:441`, `internal/attach/loop.go:292-296`, `internal/relay/server.go:120-151`, `internal/webserver/server.go:847-869`
**Issue:**
- **The client opens early.** In reconnect mode, `RelayClient` sets `open` and calls `onOpen` on the first frame. `TerminalPanel` then sets `disableStdin = false`. The CLI sets `connected` on its first frame. On both host handlers that first frame is always the grid resize frame, which is written before the replay.
- **The host does not read yet.** Each host handler starts its read pump, the only `conn.Read` caller, only after it has written the whole replay and then the `TakePending` flush loop (`relay/server.go:151`, `webserver/server.go:869`). Until then, input frames sit in the host's socket buffer. On the peer path they first pass through the daemon proxy's `copyWS`.

The scenario is the core overnight case:

1. The ring has wrapped, so the viewer gets the full rendered replay, which is up to 4 MiB of history plus the screen.
2. On a DERP link at about 300 KB/s, that takes about 14 s.
3. For those 14 s the GUI peer tab, the browser and the CLI all accept typing, while the screen is still mid-repaint.
4. Every keystroke is queued. It reaches the PTY after the replay and the flush, written all at once into whatever state the live app has reached by then.

For example, a user types `y⏎` at a Claude Code confirmation prompt that the half-drawn screen still shows. That is the failure WR-03 described: keystrokes queued into a live shell and typed at a stale screen, against T-05-14. It has moved from the 10 s proxy-dial window into the replay window. Delta replays are small, so short gaps are barely affected.

**Fix:** Open at the end of the replay, not at the first frame:
- If the socket sends a replay meta, enter `open`, call `onOpen` and enable input or clear stale on that socket's offset meta.
- For an older host, which sends no replay meta, keep "first output frame, or first frame".
- Keep clearing the connect timer at the first frame, so a slow replay is never killed.

```ts
// handleMeta: after `this.offsetValid = true`
if (this.reconnect && this.state !== 'open') { this.setState('open'); this.callbacks.onOpen?.() }
// onmessage first frame: _clearConnect() and openedAt only; defer setState('open') unless no replay meta follows (old host: open on first output)
```

In `loop.go`, store `connected` and close `first` when `meta.Offset` arrives, or on output that comes before any replay meta. The pending flush still delays host reads slightly, but that window is short when the link keeps up.

### WR-02: The automatic `daemon:recovered` calls `retryInit`, which replaces the whole tab list, so open peer tabs are dropped and the active tab switches

**File:** `frontend/src/App.tsx:924-929` (new), `frontend/src/App.tsx:910-918`, `app.go:313-319`
**Issue:**
- **Before the fix.** `retryInit` ran only from the "Retry Connection" button. That banner renders only when `daemonError && tabs.length === 0` (`App.tsx:1144`), so its `setTabs(restoredTabs)` and `setActiveId(restoredTabs[0].id)` always ran on an empty tab list.
- **After the fix.** The IN-01 fix calls `retryInit` from the `daemon:recovered` event, whatever tabs are open. `setTabs(restoredTabs)` replaces the list with local sessions only. That drops every `web-session` (peer) tab and unmounts its `RelayClient`. It also moves focus to the first local tab.

A concrete sequence:

1. The daemon dies three times within the respawn streak. The third respawn succeeds, but `daemonErr` is set to "keeps stopping" and `daemon:error` fires. The banner stays hidden because a peer tab is open, and that peer tab reconnects through the new daemon.
2. The user's local shells died with the daemon, so they open a new local session in the respawned daemon.
3. About a minute after the last respawn, the poller sees a healthy daemon with `respawnAttempts == 0` and `daemonErr != nil`, and emits `daemon:recovered`.
4. `retryInit` gets `sessions = [the new session]` and calls `setTabs([that one tab])`. The remote tab vanishes mid-use, and focus jumps.

The startup-failure path has the same hazard if the user opened a peer or local tab before recovery fires.

**Fix:** Make recovery additive, not a reset. Two options:
- Give the `daemon:recovered` handler its own path. It would clear `daemonError`, refresh `relayPort` and the sessions list, and merge restored sessions into the existing tabs (add missing local ones, keep all others), without touching `activeId` when a tab is active.
- Or call `retryInit` only when `tabs.length === 0`, using a ref, and otherwise just clear `daemonError` and refresh `relayPort`.

```ts
return EventsOn('daemon:recovered', () => {
  if (tabsRef.current.length === 0) { void retryInit(); return }
  setDaemonError(null)
  GetRelayPort().then((p) => { if (p > 0) setRelayPort(p) }).catch(() => {})
})
```

### WR-03: The daemon lock turns any unlink of the live socket into a permanent outage, and `EnsureDaemon` still unlinks without the lock

**File:** `internal/daemon/process.go:39-45` (lock), `internal/daemon/process.go:102` and `:118` (unlocked `CleanupStaleSocket` in `EnsureDaemon`), `internal/daemon/socket.go:41-50`
**Issue:**
- **The fix's assumption.** The fix says only the lock holder unlinks the socket. But `EnsureDaemon` runs in the GUI and CLI processes, which never hold the lock, and it still calls `CleanupStaleSocket` on the version-mismatch path (`:102`) and on the stale-relay path (`:118`). That function is the same non-atomic sequence from the original WR-05: dial, get `ECONNREFUSED`, then `os.Remove`.
- **The interleaving.**
  1. Process P is in the version-mismatch path. The old daemon has exited, so P's dial gets `ECONNREFUSED`.
  2. P stalls before its `os.Remove`.
  3. Daemon A, spawned by the GUI poller, the CLI or launchd, takes the lock, removes the stale file and binds.
  4. P resumes and unlinks A's live socket.
- **Before the fix,** A leaked, but the next spawned daemon bound a fresh socket and service recovered.
- **Now,** A holds `daemon.lock` for life. Every replacement daemon waits 2 s, prints "already running" and exits. Every `EnsureDaemon` fails with "did not become ready within 3s". The GUI poller then cycles through its respawn backoff indefinitely and shows "keeps stopping".

The daemon stays unreachable until someone finds and kills A by hand. For this project ("users can't get in to fix the remote Mac"), that is the worst outcome.

The race is as unlikely as the original WR-05: preemption at one point, while two spawns overlap after an upgrade. But the same permanent lockout follows from any other removal of `daemon.sock` while the daemon runs, such as a user deleting the socket file to "reset" a stuck connection. Before this fix, that was self-healing.

**Fix:**
1. Remove both `CleanupStaleSocket` calls from `EnsureDaemon`. The spawned daemon already runs it under the lock (`process.go:45`, `api.go:212`), so they add no protection, only the race.
2. Make an orphan heal itself. The daemon records `os.Stat(socketPath)` after `api.Start` and checks `os.SameFile` periodically, for example every 10 s. If the path is gone or replaced, it shuts down, which releases the lock, or re-listens.

```go
// process.go, after api.Start
bound, _ := os.Stat(socketPath)
go func() {
    t := time.NewTicker(10 * time.Second); defer t.Stop()
    for { select {
    case <-ctx.Done(): return
    case <-t.C:
        if cur, err := os.Stat(socketPath); err != nil || !os.SameFile(bound, cur) {
            fmt.Fprintln(os.Stderr, "daemon: socket path lost, exiting"); stop() // cancel ctx -> releases lock
            return
        }
    } }
}()
```

## Info

### IN-01: `RetryDaemon` ignores `quitting`, so a Retry that lands after Quit All's shutdown can respawn a daemon that outlives the app

**File:** `app.go:368-385`, `app.go:439-451`
**Issue:**
- **The gap.** WR-04 made the poller respect `quitting`, but `RetryDaemon`, the other post-startup `EnsureDaemon` caller, does not check it.
- **The sequence.** A Retry RPC can be queued just before Quit All, either from a click or from the new `daemon:recovered` → `retryInit` path. `QuitAll` then takes `daemonMu`, shuts the daemon down and unlocks. The queued `RetryDaemon` takes the lock next, finds `Health()` failing and spawns a fresh `Setsid` daemon before `runtime.Quit` finishes.
- **Likelihood.** The window is milliseconds wide, so this is unlikely.

**Fix:** After taking the lock in `RetryDaemon`, add `if a.quitting.Load() { a.daemonMu.Unlock(); return errors.New("quitting") }`.

### IN-02: `AbortSignal.timeout` throws synchronously on browsers without it, which turns every pre-redial probe into "retry" forever

**File:** `frontend/src/components/WebShareSessionView.tsx:101`, `frontend/src/lib/relayClient.ts:388-392`
**Issue:**
- **The failure.** `AbortSignal.timeout` arrived in Safari 16, Chrome 103 and Firefox 100. On an older browser, the argument expression throws a `TypeError` before `fetch` is called. That throw happens inside `beforeReconnect()`. `redial`'s `try/catch` maps it to `{ kind: 'retry' }`, so every redial retries forever, and the browser view never reconnects after its first drop.
- **Before the fix,** that same browser reconnected fine.
- **Scope.** The target Macs run current Safari, so this matters only for an older device used as the browser surface, for example iOS 15.

**Fix:** Feature-detect, falling back to an `AbortController` and `setTimeout`:

```ts
const signal = typeof AbortSignal.timeout === 'function' ? AbortSignal.timeout(10_000) : undefined
```

---

_Reviewed: 2026-11-18T08:46:31Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard (iteration 2, fix-diff scope, with cross-file tracing)_
