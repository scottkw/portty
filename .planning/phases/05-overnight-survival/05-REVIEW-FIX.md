---
phase: 05-overnight-survival
fixed_at: 2026-11-18T08:36:04Z
review_path: .planning/phases/05-overnight-survival/05-REVIEW.md
iteration: 1
findings_in_scope: 8
fixed: 8
skipped: 0
status: all_fixed
---

# Phase 5: Code Review Fix Report

**Fixed at:** 2026-11-18T08:36:04Z
**Source review:** .planning/phases/05-overnight-survival/05-REVIEW.md
**Iteration:** 1

**Scope:** the orchestrator picked these: all six Warnings (WR-01..WR-06), plus IN-08 and IN-01. The other Info items (IN-02..IN-07) were left alone on purpose and are still open in 05-REVIEW.md.

**Summary:**
- Findings in scope: 8
- Fixed: 8. WR-01, WR-02, WR-03, WR-04 and IN-01 change behaviour or logic, so they are marked "fixed: requires human verification".
- Skipped: 0
- No change needed: 0

Every fix got a test that failed before the fix. For WR-01, WR-04, IN-08 (under -race) and WR-05 the test first failed because the new symbol did not exist yet, and failed on behaviour once the stub compiled. For IN-08 it was a -race DATA RACE. For WR-02, WR-03, WR-06 and IN-01 the assertions failed.

## Fixed Issues

### WR-01: A large full replay to a remote viewer gets it kicked as "too slow"

**Status:** fixed: requires human verification
**Files modified:** `internal/relay/hub.go`, `internal/relay/protocol.go`, `internal/relay/server.go`, `internal/webserver/server.go`, `internal/relay/join_test.go`
**Commit:** 069e172c
**Applied fix:**
- `Join` marks the subscriber `replaying` before `Subscribe` publishes it.
- One shared `sendLocked` helper now serves `broadcast`, `broadcastResize` and `BroadcastMeta`. While a subscriber is replaying, it adds frames to a per-viewer pending list instead of the 256-frame channel.
- The pending list is capped at `ReplayPendingMax` bytes, which equals `HistoryMaxBytes` (4 MiB). A viewer whose link keeps up with the output holds less than its own replay, so going over the cap means the output outran the link. In that case the existing slow kick still fires.
- Both WS handlers call `Hub.TakePending` in a loop after the offset meta. The loop ends replay mode only once nothing is held, so the flush itself cannot fill the channel.
- The resume offset is still correct and each frame still arrives exactly once: held frames follow the offset meta and are counted by the client like live frames. Subscribe and capture still happen under `emuMu`.

### WR-02: The resume offset is discarded before any replay byte arrives

**Status:** fixed: requires human verification
**Files modified:** `frontend/src/lib/relayClient.ts`, `internal/attach/loop.go`, `frontend/src/lib/relayClient.test.ts`, `internal/attach/loop_test.go`
**Commit:** dcdd2606
**Applied fix:**
- Neither `dial()` (JS) nor the top of `readFrames` (Go) invalidates the offset any more.
- The offset is invalidated when the new socket's replay starts. That is its replay meta, or, for an older host that sends no meta, its first output frame.
- So a socket that the proxy accepts and then refuses with 1013, or one that dies after only the grid frame, keeps `resume=`.
- This applies to relayClient.ts (GUI peer tabs and the browser) and to the CLI loop.

### WR-03: Clients report "open" and accept input when connected only to the local proxy

**Status:** fixed: requires human verification
**Files modified:** `frontend/src/lib/relayClient.ts`, `internal/attach/loop.go`, `frontend/src/lib/relayClient.test.ts`, `internal/attach/loop_test.go`, `frontend/src/components/__tests__/TerminalPanel.reconnect.test.tsx`
**Commit:** 6c7114ae
**Applied fix:** "Open" now means the host's first frame has arrived. The host sends its grid or replay meta at once on join.
- **relayClient.ts, reconnect mode:**
  - It sets the state to `open`, calls `onOpen` and records `openedAt` (used for the healthy reset) when the host's first frame arrives, not at `onopen`.
  - `sendInput` drops input in every other state, so the client enforces T-05-14 itself, not only the UI.
  - The host path (no reconnect, local relay direct) still opens at `onopen`. There is no proxy gap on that path, and its `onOpen` sends the PTY size.
- **CLI loop:**
  - The frame reader sets `connected` before it renders the first frame.
  - `serve` clears `[reconnecting]` at that first frame, and only after a reconnect, not on the first connection.
  - `reconnect()` no longer emits `""` when the dial returns.
- **Test fakes:** the `open` helpers in relayClient.test.ts and TerminalPanel.reconnect.test.tsx now deliver the host's grid frame. The CLI test `TestRun_InputWhileDisconnectedNeverSent` now waits for the host's first frame before typing. Both changes match what a real host does.
- The input-dropped-never-queued decision (T-05-14) is unchanged.

### WR-04: "Quit All" can be undone by the new respawn poller

**Status:** fixed: requires human verification
**Files modified:** `app.go`, `app_respawn_test.go`, `tray_test.go`
**Commit:** 6b70f7ae
**Applied fix:**
- `App.quitting` is now an `atomic.Bool`.
- `QuitAll` sets it before anything else, then holds `daemonMu` around `ShutdownDaemon`. A respawn that is already running finishes first.
- The Wails `shutdown` hook also sets it. The other quit paths (the menu's Quit Portty and the tray's Quit Portty) only open the confirm modal, which ends in QuitAll or QuitGUIOnly.
- `respawnDaemonIfDown` checks the flag on entry, and again after `TryLock`. The second check covers a QuitAll that runs while the `Health()` call is in flight.
- `beforeClose` reads the flag atomically. It used to be a plain bool read across goroutines.
- The source contract string in `tray_test.go` is updated to `a.quitting.Store(true)`.

### WR-05: No cross-process lock around daemon spawn

**Status:** fixed
**Files modified:** `internal/daemon/process.go`, `internal/daemon/process_unix.go`, `internal/daemon/process_test.go`
**Commit:** 3b938b69
**Applied fix:**
- `runDaemonCore` takes `lockDaemon(<socket dir>/daemon.lock, 2s)` before `CleanupStaleSocket` or the bind. That is a non-blocking `flock(LOCK_EX)`, retried every 50 ms until the deadline.
- It keeps the file open for the life of the process with `defer lock.Close()`. Go opens files close-on-exec, so PTY shells never inherit the lock.
- A daemon that cannot get the lock logs "already running" and returns without touching the socket.
- The 2 s retry keeps EnsureDaemon's version-mismatch restart working. The old daemon calls `os.Exit` 50 ms after its shutdown reply, and the kernel then releases the lock. EnsureDaemon sleeps 200 ms before it spawns, and its 3 s readiness poll covers the new daemon's wait.
- The launchd service path also goes through `runDaemonCore`, so it is covered.

### WR-06: The browser redial path has no timeout on the probe or the WebSocket handshake

**Status:** fixed
**Files modified:** `frontend/src/components/WebShareSessionView.tsx`, `frontend/src/lib/relayClient.ts`, `frontend/src/components/__tests__/WebShareSessionView.test.tsx`, `frontend/src/lib/relayClient.test.ts`
**Commit:** e1197401
**Applied fix:**
- `probeShare` passes `signal: AbortSignal.timeout(10_000)`. An abort rejects into the existing `{ kind: 'retry' }` handler.
- In reconnect mode, each socket arms a connect timer, `RECONNECT.connectMs`, set to 15 s. That is longer than the daemon proxy's 10 s upstream dial, so on the GUI path the proxy's 1013 arrives first.
- If the host's first frame has not arrived when the timer fires, the client:
  - abandons the socket: it bumps the generation, nulls the handlers and calls `close()`;
  - then runs `scheduleRetry()`, which is the normal backoff.
- A timeout therefore never reaches the terminal stopped state.
- The timer is cleared at the first frame, at `onclose`, and in `teardown` (the user's `close()` and `stop()`).
- One existing test window was shortened. "late onclose from an abandoned socket" now advances less than `connectMs`, because the redialed socket in that test never opens.

### IN-08: Data race on `a.daemonErr` between `RetryDaemon` and `GetDaemonError`

**Status:** fixed
**Files modified:** `app.go`, `app_respawn_test.go`, `app_test.go`
**Commit:** 5bb8dc18
**Applied fix:**
- `daemonErr` is now an `atomic.Pointer[error]`.
- I chose an atomic rather than `daemonMu` so that `GetDaemonError`, which runs during init, never blocks behind a running EnsureDaemon.
- `TestDaemonError_RetryAndReadConcurrently` reproduced the race under -race before the fix.

### IN-01: After a failed startup, a background respawn that works never recovers the UI

**Status:** fixed: requires human verification
**Files modified:** `app.go`, `app_respawn_test.go`, `frontend/src/App.tsx`, `frontend/src/components/__tests__/App.daemon-recovered.test.tsx` (new)
**Commit:** 32c5cbb1
**Applied fix:**
- When the third respawn raises `daemon:error`, it now also records that error, so `GetDaemonError` agrees with the banner.
- On a healthy poll with an error showing and no respawn streak in progress (a streak ends after a minute of healthy daemon), the poller:
  - clears `daemonErr`;
  - applies the stored Appearance, which startup skipped;
  - emits `daemon:recovered`.
- Requiring the streak to have ended stops the "keeps stopping" banner from vanishing 5 s after a third respawn.
- App.tsx subscribes to `daemon:recovered` (except in web mode) and runs the existing `retryInit`. That clears the banner, calls RetryDaemon (a no-op on a healthy daemon) and reloads the relay port, sessions and tabs.
- **Trade-off:** after a startup failure, the UI recovers about a minute after the respawn, not within 5 s.

## Skipped Issues

None.

## Verification

Each fix was edited, tested and committed in the isolated worktree `.claude/worktrees/rf-05-6081-*` on branch `gsd-reviewfix/05-6081`. Frontend dependencies were installed there with `pnpm install --frozen-lockfile --offline`. The full gate also ran in that worktree, after the last code commit; the only later commit was the TESTING.md doc. The branch was then fast-forwarded onto `gsd/phase-05-overnight-survival` and the worktree was removed. The main checkout now holds exactly the tree that was tested. I re-ran `tests/check-traceability-paths.sh` in the main checkout after the merge.

Gate results, all with `GOTOOLCHAIN=go1.26.11`:

| Check | Result |
|-------|--------|
| `go build ./... && go vet ./...` | Pass |
| `go test -race -short -count=1 -skip '^(TestAPIAppearance_GetPatch\|TestAPITerminalFont_GetPatch)$' ./...` | Pass: every package ok, exit 0, no reruns needed |
| `go test -race -count=1 ./internal/daemon -run 'TestCLIAttach\|TestRemote'` (without -short, for WR-03's CLI change) | Pass |
| `pnpm exec tsc --noEmit` | Pass |
| `pnpm exec vitest run` | Pass: 87 files, 1166 tests, no flake reruns needed |
| `bash tests/check-traceability-paths.sh` | Pass: 131 paths |

TESTING.md is updated in commit ac7ccc2c:
- vitest count 86 to 87, total 218;
- Section 4 SHAR-07 rows extended;
- new rows for `App.daemon-recovered.test.tsx` and `internal/daemon/process_test.go`;
- no new Section 5 item, because the orchestrator runs the VM regression.

Not staged: `.gitignore`, `.planning/state.json`, `.ignore`, `agenthub`. REQUIREMENTS.md and the ROADMAP were not touched. The token-gated share route, `requireCapability`, the grant re-check before the replay, and the CSP are all unchanged.

---

_Fixed: 2026-11-18T08:36:04Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
