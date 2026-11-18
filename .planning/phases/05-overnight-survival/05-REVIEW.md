---
phase: 05-overnight-survival
reviewed: 2026-11-18T08:09:53Z
depth: standard
files_reviewed: 33
files_reviewed_list:
  - app.go
  - build.sh
  - cmd_attach.go
  - toolchain_guard.go
  - go.mod
  - internal/attach/loop.go
  - internal/daemon/api.go
  - internal/daemon/remote_ws_proxy.go
  - internal/daemon/process.go
  - internal/relay/close_reasons.go
  - internal/relay/heartbeat.go
  - internal/relay/history.go
  - internal/relay/hub.go
  - internal/relay/protocol.go
  - internal/relay/scrollback.go
  - internal/relay/server.go
  - internal/wakewatch/wakewatch.go
  - internal/webserver/server.go
  - frontend/src/App.tsx
  - frontend/src/main.tsx
  - frontend/src/components/ErrorBoundary.tsx
  - frontend/src/components/SessionEndedBanner.tsx
  - frontend/src/components/TabBar.tsx
  - frontend/src/components/TerminalPanel.tsx
  - frontend/src/components/WebShareSessionView.tsx
  - frontend/src/lib/closeReasons.ts
  - frontend/src/lib/guestTerminal.ts
  - frontend/src/lib/relayClient.ts
  - frontend/src/lib/tabs.ts
  - tests/build-script.test.sh
  - tests/check-share-removal.sh
  - internal/daemon/socket.go
  - internal/daemon/process_unix.go
findings:
  critical: 0
  warning: 6
  info: 8
  total: 14
  necessity: 1
status: issues_found
---

# Phase 5: Code Review Report

**Reviewed:** 2026-11-18T08:09:53Z
**Depth:** standard, plus cross-file tracing of the respawn, reconnect, replay and proxy paths
**Files Reviewed:** 33 (the 31 in scope, plus `internal/daemon/socket.go` and `process_unix.go`, read to answer the respawn-race question)
**Status:** issues_found

## Summary

I reviewed the Phase 5 source changes: the base is `3df657f2^` and the head is `40f49b92`. The core reconnect design holds up:

- **Exactly-once replay is correct.** The live drain loop records and broadcasts under `emuMu`, and `Join` subscribes and captures under that same lock.
- **Offset units match end to end.** The ring counts frame bytes. JS adds `payload.length + 1` and Go adds `len(payload) + 1`. The proxy forwards frames verbatim.
- **Lock order is consistent.** It is always `emuMu` then `hub.mu`, and I found no inversion.
- **`?resume=` cannot bypass capability checks.** It is parsed as a decimal uint64 after `requireCapability`, and re-encoded by the proxy.

Toolchain and test checks, run with `GOTOOLCHAIN=go1.26.11`:

- `go vet` is clean on relay, attach, wakewatch, daemon and the root package.
- `go test -race` passes for relay, attach and wakewatch.
- The respawn and reauth tests (`TestRespawn|TestRemoteReauth`) pass under `-race`.

No Critical issues were found. Six Warnings remain, and each comes with a concrete failure scenario:

- **WR-01: a viewer can be kicked over and over in the core overnight case.** A full replay to a remote viewer can get it kicked as "too slow" before the replay finishes. If the session is still busy, this repeats on every redial.
- **WR-02 and WR-03: the proxy accepts the loopback socket before it dials the peer.** Because of this, clients:
  - throw away a resume offset that is still valid;
  - report "open" and enable stdin while no peer connection exists yet.
- **WR-04: "Quit All" can leave a daemon running.** The new respawn poller can bring the daemon back during quit.
- **WR-05: a rare race can orphan a daemon.** `EnsureDaemon` has no cross-process lock, and the dial-then-remove step in `CleanupStaleSocket` is not atomic. A simultaneous GUI and CLI respawn can, rarely, leave an orphaned daemon behind.
- **WR-06: the browser's redial path has no timeouts.**

### Answers to the six questions

**1. Simultaneous respawn race (05-16). See WR-05.**

- **Can two daemons start?** Yes. `EnsureDaemon` (`process.go:81`) has no lock, so the GUI poller and `portty attach` can each call `startDetachedDaemon`.
- **Who wins the socket?** The socket bind decides it in almost every case. The second daemon's `api.Start` either:
  - runs `CleanupStaleSocket`, connects to the first daemon and exits with "already running"; or
  - gets `EADDRINUSE` from `net.Listen`.

  Either way it exits before it serves anything. Each daemon's relay port is a separate `127.0.0.1:0` listener, so the ports never collide. The loser's port closes when the loser exits.
- **Residual hole:** `CleanupStaleSocket` connects, gets `ECONNREFUSED`, then runs `os.Remove(path)` (`socket.go:42-50`). That pair of steps is not atomic. If one daemon stalls between the two steps while the other removes the stale file and binds, the stalled daemon unlinks the live socket. The first daemon then keeps running, listening on a socket file that no longer exists.
- **Effect:** that daemon is orphaned forever with its own relay port. A client is affected only if it got the orphan's relay port during the window between bind and unlink, and that heals on the next redial. Local sessions are split only if one was created in that same window.
- **Likelihood:** very low. It needs millisecond-scale preemption at one exact point, during the only moment two spawns overlap: GUI plus CLI respawning together, or with a launchd KeepAlive agent installed.
- **Worst case:** a leaked daemon process and port. In the most unlikely case, a shell started inside that window becomes unreachable.

**2. Respawn backoff and streak logic in app.go.**

- **Verified OK:**
  - `a.client` is read without a lock but is set in `NewApp` and never reassigned.
  - The streak fields are touched only under `daemonMu`.
  - There is no deadlock. `RetryDaemon` waits for startup before it takes the lock, and the poller checks `startupDone` without blocking, then uses `TryLock`.
  - The backoff math gives 10, 20, 40, 80 and 120 s.
  - There is no respawn during startup: `startDaemon` runs synchronously before `startTrayPoller`.
- **Not OK:**
  - The poller can respawn during "Quit All" (WR-04).
  - It also overrides a deliberate `portty daemon stop` (IN-02).
  - After a failed startup, a respawn that works in the background never recovers the UI (IN-01).

**3. relayPort refresh in App.tsx (`App.tsx:613-614`).**

Verified OK:

- **No stale closure.** It uses a functional updater.
- **No out-of-order stale port.** A request to a dead daemon errors, and `GetRelayPort` maps that to 0, which is ignored.
- **No remount or disconnect.** The `TerminalPanel` mount effect depends only on `[sessionId, wsURL]`. Local panels are keyed by `tab.sessionId`, and the peer view by `sid:cap`. The port reaches `RelayClient` only at construction.

Edge case: the refresh never moves the port off `null`, so after a failed startup it never takes effect (IN-01).

**4. Reconnect loop.**

- **Verified OK:**
  - The offset arithmetic is consistent between `relayClient.ts:277` and `loop.go:298-299`, and agrees with `Scrollback.Append`.
  - There is no duplicate-output window in steady state or on `abandon()`/`broken()`. A frame that arrives on an abandoned socket is neither counted nor rendered, so the next delta resends it.
  - Unmount cleanup (`close()` then `teardown()`) clears the watchdog, ping, retry timer and listeners. The banner timer is also cleared.
  - Wake handling is rate-limited and cannot start a second redial while one is running.
  - The CLI's stale-keystroke discard is correct. The single stdin reader and the `stale` flag are sound.
- **Not OK:**
  - The resume offset is dropped too early (WR-02).
  - "Open" means "connected to the loopback proxy", not "connected to the host". Keystrokes typed while the proxy is still dialing the peer (up to 10 s) are buffered by the proxy and delivered later, or dropped (WR-03).
  - The browser path has no timeout on the probe or the WebSocket handshake (WR-06).

**5. Hub Join, replay and history.**

- **Verified OK:**
  - Locking is correct, and `Run` and `Join` give exactly-once delivery.
  - Geometry gating is right. A delta is sent only when `resume > geomEnd`. The raw full replay is used only when the ring is intact and no genuine resize has happened; otherwise the rendered history plus the screen goes out.
  - History is bounded at 10,000 lines and 4 MiB, and `Push` compacts in amortized O(1).
- **Emulator memory: measured, no issue.** I suspected a large per-write spike from the emulator's `SetScrollbackSize(10000)` buffer. A scratch program pushed 10,000 lines at 80 and 220 columns, and the heap grew only 1.3 MiB.
- **Real concern:** the subscriber queue cannot drain while a large replay is being written (WR-01). Each join's replay buffers are held until the write finishes (IN-03).

**6. Security.**

Verified OK:

- **The share WebSocket route is still gated.** `GET /sessions/{id}/ws` remains `requireAllowedOrigin(requireCapability(...))` (`webserver/server.go:493-494`).
- **Revocation is re-checked before replay.** The grant re-check runs after `Join` and before any replay byte is written (`:830-833`), so a revoked viewer gets no history.
- **`?resume=` is safe.** Only `ParseResume` reads it. The proxy re-encodes it from the parsed number (`remote_ws_proxy.go:94-96`). It selects only which replay is sent, never whether access is allowed.
- **Close reasons leak nothing.** All of them are fixed literals ("share stopped", "session ended", "too slow", "remote unreachable", "bad base url", "detach"). The proxy forwards the peer's reason, and banners never render wire text.
- **Logs stay clean.** The proxy logs redact the query and the cap.
- **The CSP is untouched.** `csp_mw.go` has no Phase 5 diff. `ErrorBoundary` logs only the error type.

One privacy nuance about the T-05-10 claim is in IN-05.

## Warnings

### WR-01: A large full replay to a remote viewer gets it kicked as "too slow" while output continues, and every redial repeats it

**File:** `internal/relay/hub.go:839-866`, `internal/webserver/server.go:795,803-805,817,850-854`, `internal/relay/server.go:89,101,123-127`
**Issue:**
- `Join` subscribes the viewer before the caller writes the replay. Live frames therefore queue in `sub.Msgs` (capacity 256) for the whole replay write.
- The same goroutine that drains `Msgs` is the one writing the replay, so the queue only grows.
- After an overnight gap the ring (256 KiB) has wrapped past the viewer's offset. The viewer then gets the full rendered replay, which is up to 4 MiB of history plus the screen.

The scenario:

1. A DERP-relayed link runs at about 300 KB/s, so a 4 MiB replay takes about 13 s to write.
2. The session keeps producing output, for example Claude Code repainting at about 30 renders/s, or a build log. That is more than 256 frames in 13 s.
3. `broadcast` calls `kickSlow` and the viewer is closed with 1008 "too slow" in the middle of the replay.
4. The offset meta never arrived, so both clients' offsets are invalid: JS cleared it at `relayClient.ts:207` and Go at `loop.go:272`.
5. The next redial is another full replay and is kicked the same way. The viewer cannot get back in until output pauses for the length of one replay.

This is the core overnight scenario. Its likelihood depends on link speed and history size. R8 tested a large gap but not output during the replay.
**Fix:** Do not count frames against a subscriber that is still being replayed. Options:
- Let `Join` mark the subscriber `replaying`. While it is set, `broadcast` appends to a pending list capped in bytes, for example `2*DefaultScrollbackBytes`, instead of the 256-frame channel. The writer flushes that list after the offset meta. Kick only if the byte cap is exceeded.
- Or write the replay from a separate goroutine while the write pump keeps draining `Msgs` into a local buffer capped in bytes.

```go
// sketch: in broadcast, under h.mu
if sub.replaying {
    if sub.pendingBytes+len(frame) > replayPendingCap { h.kickSlow(sub); continue }
    sub.pending = append(sub.pending, frame); sub.pendingBytes += len(frame)
    continue
}
```

### WR-02: The resume offset is discarded before any replay byte arrives, so a socket that the proxy accepts and then refuses turns the next reconnect into a full reset

**File:** `frontend/src/lib/relayClient.ts:206-208,428-430`, `internal/attach/loop.go:272`, `internal/daemon/remote_ws_proxy.go:73,116-125`
**Issue:**
- `RelayClient.dial()` sets `offsetValid = false` before the new socket even exists. The CLI's `readFrames` clears it as soon as a socket is up.
- The daemon proxy accepts the loopback socket (`:73`) before it dials the peer (`:116`). When the upstream dial fails, it closes the socket with 1013 "remote unreachable".

Any redial where the re-auth says OK but the peer WebSocket dial then fails therefore loses a resume offset that was still exactly valid. For example:
- the network flaps between `ConnectPeer` and the dial;
- a cold DERP path takes longer than the proxy's 10 s dial timeout;
- in the browser, the handshake fails after `probeShare` succeeds.

On the next successful connect:
- the GUI and browser receive `replay: full`, run `onReset` (RIS, which wipes the 10,000-line xterm buffer) and redraw;
- the CLI clears the screen and repaints.

Nothing is duplicated, but the "gap-only replay" promise is broken, a full replay is sent over the tailnet, and it feeds WR-01.

Invalidating the offset is needed only once this socket has rendered replay output. After that, the old offset is stale and replaying from it would duplicate output.
**Fix:** Keep the old offset until the first output frame or replay meta on the new socket:

```ts
// dial(): do NOT touch offsetValid; set this.replayPending = true
// handleFrame 'output' / handleMeta 'replay': if (this.replayPending) { this.offsetValid = false; this.replayPending = false }
```

In `loop.go`, move `l.offsetValid = false` out of the top of `readFrames` and into the first `MsgOutput` frame, or the first `meta.Replay`, of that connection. Separately, consider having the proxy dial upstream before `websocket.Accept`, so a refusal becomes a failed handshake (HTTP 502) instead of an opened-then-closed socket. That change also fixes WR-03.

### WR-03: Clients report "open" and accept input when connected only to the local proxy; keystrokes are buffered for up to 10 s, then delivered late or dropped

**File:** `frontend/src/lib/relayClient.ts:226-229`, `frontend/src/components/TerminalPanel.tsx:440-441`, `internal/attach/loop.go:182,354-358`, `internal/daemon/remote_ws_proxy.go:73-131`
**Issue:**
- For a peer tab and `portty attach`, `onopen`, or `websocket.Dial` returning, happens when the loopback proxy accepts. That is before the proxy has dialed the peer.
- The GUI then sets state `open`. That hides the reconnecting banner and sets `disableStdin = false`. The CLI clears `[reconnecting]` and marks stdin chunks as not stale.
- Keystrokes typed now go into the loopback socket. The proxy starts reading the client side (`copyWS` at `:139`) only after the upstream dial, which can take up to 10 s on a cold or DERP path.

The keystrokes are then either delivered late into the live shell, or dropped when the dial fails (1013).

This undercuts the Phase 5 decision that input stays disabled and keystrokes are never queued into a live shell (T-05-14). The user is typing at the pre-drop screen, because the delta has not arrived yet. The GUI banner also flaps: hidden while the proxy dials, shown again after the 1013.
**Fix:** Treat the first server frame as "open", not the socket upgrade. The host always sends the grid frame or the replay meta at once on join.

```ts
// ws.onopen: start pings only; ws.onmessage: if (this.state !== 'open') { this.setState('open'); this.callbacks.onOpen?.() }
```

In `loop.go`, set `l.connected.Store(true)` and `l.state("")` from `readFrames` on the first frame instead of in `serve` and `reconnect`. Dialing upstream before accepting in the proxy (see WR-02) also closes most of the window.

### WR-04: "Quit All" can be undone by the new respawn poller, leaving a fresh daemon running after the app exits

**File:** `app.go:428-437`, `app.go:290-320`, `app.go:1126-1141`
**Issue:**
1. `QuitAll` calls `ShutdownDaemon()`. The daemon exits 50 ms after replying (`api.go:281-289`).
2. Only then does `QuitAll` set `a.quitting` and call `runtime.Quit`.
3. `respawnDaemonIfDown` never checks for quitting, and the tray poller's ctx is not cancelled before the process exits.

If the 5 s tick lands between the daemon exiting and the GUI process ending:
- `Health()` fails;
- `respawnAttempts` is 0, so there is no backoff wait;
- `EnsureDaemon` calls `startDetachedDaemon`, which starts a daemon with `Setsid` that outlives the GUI.

Result: the user chose "Quit All (end every session)", and an empty daemon is left running in the background. The odds per quit are roughly the quit duration divided by 5 s, so a few percent. The bad state persists. The respawned daemon has no shares (they are memory-only), so this is not a remote-exposure problem. A running `portty attach` will also respawn on its next redial; that is arguably intended, but worth stating.
**Fix:** Make the quit intent visible to the poller and serialize it with respawn:

```go
type App struct { /* ... */ quitting atomic.Bool }
func (a *App) QuitAll() {
    a.quitting.Store(true)          // before the daemon goes away
    a.daemonMu.Lock()               // no respawn in flight
    _ = a.client.ShutdownDaemon()
    a.daemonMu.Unlock()
    runtime.Quit(a.ctx)
}
// respawnDaemonIfDown: if a.quitting.Load() { return }  (first line)
```

`beforeClose` must then read `a.quitting.Load()`. Today the plain `bool` is also a latent data race between the binding goroutine and the Wails close callback.

### WR-05: No cross-process lock around daemon spawn; the non-atomic dial-then-remove in `CleanupStaleSocket` can orphan a live daemon during a simultaneous GUI and CLI respawn

**File:** `internal/daemon/process.go:81-138`, `internal/daemon/socket.go:41-55`, `internal/daemon/process.go:36,61`, `internal/daemon/api.go:208-222`. Callers: `app.go:320`, `cmd_attach.go:168`.
**Issue:** Phase 05-16 added a second process that respawns the daemon: `remoteReauth` calls `ensureLocalDaemon`. Each process spawns independently. After a `kill -9`, the stale socket file is still present. An interleaving that orphans daemon A:

1. B's `CleanupStaleSocket` connects to the stale file and gets `ECONNREFUSED`.
2. B is preempted before its `os.Remove`.
3. A removes the stale file, bootstraps, starts its relay and `net.Listen`s, which creates a new socket file.
4. B resumes and `os.Remove`s A's live socket.
5. B binds the path and serves.

A keeps running forever, listening on a socket file that no longer exists, with its relay port open. Nothing ever signals it to stop, because `SIGTERM` comes only from launchd or a manual kill. A smaller window also exists between `bind` and `listen` inside `net.Listen`, where a connect attempt gets `ECONNREFUSED`.

- **Impact:** a leaked process and port. A viewer that read A's relay port during the window heals on its next redial. A shell created through A in that window becomes unreachable.
- **Likelihood:** very low (millisecond-scale preemption at a single point). It applies only while two spawns overlap: GUI poller plus CLI, or either one plus a launchd KeepAlive agent. VM sub-run R6(c) left exactly one daemon, as expected for the common interleavings.

**Fix:** Have the daemon take an exclusive `flock` on `daemon.lock` in the config dir, before it touches the socket, and hold it for life. A second daemon then exits immediately and never runs `CleanupStaleSocket`:

```go
f, err := os.OpenFile(filepath.Join(daemonConfigDir(), "daemon.lock"), os.O_CREATE|os.O_RDWR, 0600)
if err != nil { return }
if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
    fmt.Fprintln(os.Stderr, "daemon: already running"); return
}
// keep f open for the process lifetime; then CleanupStaleSocket is safe (only the lock holder unlinks)
```

### WR-06: Browser redial path has no timeout on the pre-redial probe or on the WebSocket handshake, so a hang can only end when the browser gives up

**File:** `frontend/src/components/WebShareSessionView.tsx:96-98`, `frontend/src/lib/relayClient.ts:214,323-331,336-350,407-418,315`
**Issue:** `probeShare` calls `fetch` with no timeout or `AbortSignal`, and `new WebSocket(url)` has no connect timeout. Two hang cases:

- **The probe hangs.** Say the host Mac is asleep (R5), or the browser reuses a half-open pooled connection to the host. Then `redial()` stays inside `await beforeReconnect()` with `redialing = true` and `retryTimer = null`.
  - `onWake` and `onSystemWake` act only when `retryTimer !== null`, so 'online', visibility changes and wake events do nothing.
  - The liveness watchdog (`checkLiveness`) runs only in state `open`.
  - The browser view is therefore stuck in "reconnecting" until the browser's own network timeout, which can be minutes, even after the host is back.
- **The handshake hangs.** The same applies to a socket stuck in CONNECTING (SYN retries to a host that is asleep).

The GUI path is bounded: the daemon client has a 30 s timeout, `/peers/connect` 20 s, and the proxy dial 10 s. The CLI is bounded too: `dialTimeout` is 10 s. So this affects only the browser surface, which may be the user's only way in.
**Fix:**

```ts
fetch(url, { cache: 'no-store', signal: AbortSignal.timeout(10_000) })
```

In `dial()`, arm a connect timer, for example 10 s. If `onopen` has not fired by then, do what `abandon()` does: bump `socketGen`, null the handlers, call `close()` and `scheduleRetry()`.

## Info

### IN-01: After a failed startup, a background respawn that works never recovers the UI

**File:** `frontend/src/App.tsx:322-325,613-614,1138`, `app.go:290-333`
**Issue:**
- **The backend recovers.** When startup's `EnsureDaemon` fails, the poller keeps respawning, so the daemon may come up a few seconds later.
- **The frontend does not.** `init` returned early and `relayPort` stays `null`, because the poll never moves it off `null`, so local panels never render. `a.daemonErr` and the frontend `daemonError` stay set, and the "Unable to connect" banner remains until the user clicks Retry.
- **Same problem after a mid-session error.** A respawn after the attempt-3 `daemon:error` that later succeeds never clears the banner, which shows whenever `tabs.length === 0`.

**Fix:** Emit a `daemon:recovered` event, or clear `daemonErr`, when `respawnDaemonIfDown` succeeds. Have App.tsx clear `daemonError` and re-run the init or retry sequence on that event.

### IN-02: The respawn poller overrides a deliberate stop of the daemon

**File:** `cmd_daemon.go:47-52`, `app.go:301-320`
**Issue:**
- **Deliberate stops are reversed.** While the GUI is open, `portty daemon stop` (launchd service stop), or killing the daemon on purpose, is undone within 5 to 10 s. A non-launchd daemon is started in its place.
- **Double spawns with launchd.** With a KeepAlive agent installed, launchd and the poller can both respawn after a crash, which widens the WR-05 window.

**Fix:** Have the `stop` and `uninstall` subcommands leave a marker, such as a `daemon.stopped` file or a daemon RPC, that the poller honors. Skip the poller's respawn when the launchd service is installed.

### IN-03: Replay buffers and the missing heartbeat during the replay write

**File:** `internal/relay/hub.go:805-815,858`, `internal/webserver/server.go:850-931`, `internal/daemon/remote_ws_proxy.go:144-152`
**Issue:**
- **Replay buffers.** Each join builds `history.Render()` (up to 4 MiB) and then a chunked copy, and holds both until the replay write finishes.
- **No heartbeat until the replay is written.** The heartbeat starts only after the replay. A half-open viewer therefore holds that memory and a handler goroutine until TCP gives up, or until 256 frames trigger the kick.
- **Many joins at once.** Any holder of a read cap can open many such sockets.
- **Pings go unanswered on the host during replay.** The host's read pump, which answers WebSocket pings, also starts after the replay. If a replay through the proxy takes more than about 45 s, the proxy's upstream `Heartbeat` (30 s ping, 15 s timeout) declares the host dead and drops a healthy connection. That needs a link slower than about 90 KB/s.

**Fix:** Start the read pump and heartbeat before writing the replay. Consider capping concurrent full replays per hub.

### IN-04: A peer tab's resume offset is discarded on every tab switch

**File:** `frontend/src/App.tsx:1063-1118`, `frontend/src/components/TerminalPanel.tsx:461-464`
**Issue:**
- **Only the active tab is mounted.** Only the active `__websession__` tab renders a `WebShareSessionView`, so switching to another tab unmounts the panel and closes its `RelayClient`.
- **Each return is a full replay.** Switching back is a fresh connection without `?resume=`, so it gets a full replay (up to 4 MiB over the tailnet) and a redraw.
- **Scope.** This was the architecture before Phase 5, but it limits the "GUI keeps its buffer" goal to tabs that stay in front.

**Fix:** Keep remote panels mounted but hidden, the way local tabs are (`display: none`), or keep each tab's last offset in App state and pass it to the next `RelayClient`.

### IN-05: ESC[3J clears the hub history, but raw and delta replays still send the cleared bytes

**File:** `internal/relay/hub.go:479-486,847-850,864-866`
**Issue:**
- **What `clear` erases.** It sends `ESC[3J` and empties `h.history`.
- **What it does not.** The raw ring is not touched. While the ring is intact (no wrap, no genuine resize), a full replay is the raw ring, so a viewer that joins later receives the pre-clear output on the wire, then the clear sequence. The same is true for a delta whose range spans the clear.
- **Who sees it.** xterm and most terminals honor ESC[3J, so the final screen is clean. However, the bytes cross the network and pass through the CLI viewer's local terminal and any logging it does.
- **Risk.** This partly contradicts T-05-10 ("a cleared transcript is never replayed"). The risk is low because every viewer already has read access.

**Fix:** Record the ring offset of the last ESC[3J or RIS. Treat any raw or delta replay that would start before that offset as the rendered path, or start the raw replay from that offset.

### IN-06: `RenderSnapshot` and `EnsureLiveEmulator` are dead in production

**File:** `internal/relay/hub.go:640-644,672-680`
**Issue:** After `ReconnectPreamble` was removed in 05-01, a grep finds no non-test caller of either method. `Join` uses `ensureLiveEmulatorLocked` and `renderScreenLocked` directly. Their doc comments still say callers "MUST call EnsureLiveEmulator before RenderSnapshot", which describes a contract nothing uses.

**Fix:** Remove both, or mark them as test helpers, and move the tests to the locked variants or to `Join`.

### IN-07: Impossible-state guard in `respawnDaemonIfDown`: the `startupDone` check

**File:** `app.go:294-300`
**Category:** necessity
**Issue:**
- **The guard cannot fire in production.** It skips while startup's `EnsureDaemon` is still running. But `startup` calls `startDaemon`, which closes `startupDone` on every path, synchronously at `app.go:234` and before `startTrayPoller` at `app.go:240`. `startup` is the only caller of `startTrayPoller`.
- **Only tests reach it.** `TestRespawnDaemon_BeforeStartupSkipsEnsure` builds an `App` by hand to exercise it.
- **Why it might stay.** It is cheap protection if startup is ever reordered, so removing it is a judgment call.

**Fix:** Either keep it and say so in the comment ("defends a future reorder; unreachable today"), or remove it together with its test.

### IN-08: Data race on `a.daemonErr` between `RetryDaemon` and `GetDaemonError`

**File:** `app.go:368-372`, `app.go:339-346`
**Issue:**
- **Unsynchronized write and read.** `RetryDaemon` writes `a.daemonErr` after releasing `daemonMu`. `GetDaemonError` reads it with no lock.
- **The startup channel does not help.** The `startupDone` happens-before edge covers only startup's own write.
- **When they overlap.** Both are binding goroutines, so a Retry click concurrent with an `init` (for example, React StrictMode running init twice in dev) races.
- **Impact.** Low, but `-race` would flag it in an end-to-end test.

**Fix:** Write and read `daemonErr` under `daemonMu`, or store it in an `atomic.Pointer[error]` or `atomic.Value`.

---

_Reviewed: 2026-11-18T08:09:53Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard (plus cross-file concurrency and reconnect tracing)_
