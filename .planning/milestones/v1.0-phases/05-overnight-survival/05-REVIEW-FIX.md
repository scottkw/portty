---
phase: 05-overnight-survival
fixed_at: 2026-11-18T09:05:44Z
review_path: .planning/phases/05-overnight-survival/05-REVIEW.md
iteration: 3
findings_in_scope: 5
fixed: 5
skipped: 0
status: all_fixed
---

# Phase 5: Code Review Fix Report (iteration 3)

**Fixed at:** 2026-11-18T09:05:44Z
**Source review:** .planning/phases/05-overnight-survival/05-REVIEW.md (iteration 2)
**Iteration:** 3

**Scope:** every finding in the iteration-2 review: WR-01, WR-02, WR-03, IN-01 and IN-02. (The iteration-1 report, 8/8 fixed, is in git history at a8b72318.)

**Summary:**
- Findings in scope: 5
- Fixed: 5. WR-01, WR-02 and WR-03 change logic, so they are marked "fixed: requires human verification".
- Skipped: 0

Each fix got a test that failed before the fix:
- WR-01, WR-02, IN-01 and IN-02 failed on their assertions.
- WR-03: `TestSocketLost` first failed to compile (no `socketLost` yet). Against a stub, both new tests then failed on behaviour.

## Fixed Issues

### WR-01: Input opened at the grid frame, before the replay

**Status:** fixed: requires human verification
**Files modified:** `frontend/src/lib/relayClient.ts`, `internal/attach/loop.go`, `frontend/src/lib/relayClient.test.ts`, `frontend/src/components/__tests__/TerminalPanel.reconnect.test.tsx`, `internal/attach/loop_test.go`
**Commit:** 65f53775
**Applied fix:**
- **relayClient (GUI peer tabs and browser), reconnect mode:**
  - It enters `open` and calls `onOpen` when the socket's offset meta arrives, which is the end of the replay.
  - A host that sends no replay meta opens at its first output.
  - The connect timer still clears at the host's first frame, so a slow replay is never killed.
- **Watchdog (needed by the change above):** the liveness check now runs from the host's first frame, not only from `open`. Without this, a socket that went half-open mid-replay would hang forever, because the connect timer is already cleared and the state is not yet `open`. `openedAt` is reset on close and abandon, so the watchdog leaves a dead socket alone.
- **TerminalPanel:** no code change. Its stdin follows the state, so it stays disabled until `open`. The banner also stays up until then.
- **CLI loop:** `connected` is set, and `[reconnecting]` cleared, at the offset meta, or at an older host's first output. Keystrokes read before that are still marked stale and dropped, never queued (T-05-14).
- **WR-02 resume offset:** unchanged. `markOpen` touches no offset state. The refused-socket and grid-only-drop tests still keep `resume=`.
- **Test fixtures:** `open` in the relayClient and TerminalPanel tests now means "replay done" (grid, replay meta, offset meta). The new `joined` helper is the grid-only step. `TestRun_NotConnectedUntilFirstFrame` is renamed `TestRun_NotConnectedUntilReplayEnds` and now also types during the replay.
- **Residual (accepted by the review):** keystrokes typed between the offset meta and the end of the host's short pending flush still queue on the host.

### WR-02: daemon:recovered wiped the open tabs

**Status:** fixed: requires human verification
**Files modified:** `frontend/src/App.tsx`, `frontend/src/components/__tests__/App.daemon-recovered.test.tsx`
**Commit:** 02ab3d77
**Applied fix:**
- A `tabsRef` mirrors `tabs`, in the same pattern as `activeIdRef`.
- With no tabs open, `daemon:recovered` runs `retryInit`, as before.
- With tabs open, it only clears `daemonError` and refreshes `relayPort` (when the port is above 0). Tabs and the active tab are untouched.
- The new test raises `daemon:error` on a healthy app with two tabs and selects the second tab. When `daemon:recovered` fires, the daemon reports a different session. The test checks that both tabs and the selection stay, RetryDaemon is not called, and the relay port is fetched again.

### WR-03: The daemon lock turned a socket unlink into a permanent outage

**Status:** fixed: requires human verification
**Files modified:** `internal/daemon/process.go`, `internal/daemon/process_test.go`
**Commit:** fa477a39
**Applied fix:**
- **No client-side unlink:** both `CleanupStaleSocket` calls in `EnsureDaemon` are gone (the version-mismatch and stale-relay paths). The only remaining callers run in the daemon: `runDaemonCore` under the lock, and `API.Start`.
- **Version-mismatch restart:** unchanged. EnsureDaemon calls `ShutdownDaemon`, sleeps 200 ms, then spawns. The new daemon waits up to 2 s for the lock, then cleans up the stale file and binds.
- **Self-check:** after the bind, the daemon stats the socket path. Every 5 s, `socketLost` checks the path against that stat with `os.SameFile`.
  - If the path is gone (`ErrNotExist`) or names another file, the daemon logs "socket … was removed or replaced; exiting to free the lock".
  - It closes its listener with `SetUnlinkOnClose(false)`, because the path may now be another daemon's socket, so neither `Close` nor `API.Stop` unlinks it.
  - It then runs the normal shutdown, and `defer lock.Close()` releases the flock.
  - Other stat errors are not counted, so a transient error cannot end every session.
  - There is no watch when the socket is not a file (a Windows pipe).
- **Tests:**
  - `TestEnsureDaemon_NeverUnlinksSocket`: a fake daemon on a Unix socket stops listening, leaving its file, just before EnsureDaemon's old cleanup point. The test covers both paths. Before the fix the file was removed.
  - `TestSocketLost`: kept, removed and rebound.
- **Trade-off:** an orphaned daemon ends its sessions when it exits. No client could reach them through the socket anyway. The review's other option, re-listening on the path, would keep them, but that is a larger change. I followed the requested exit behaviour.

### IN-01: RetryDaemon ignored quitting

**Status:** fixed
**Files modified:** `app.go`, `app_respawn_test.go`
**Commit:** d1ca6c25
**Applied fix:** `RetryDaemon` checks `a.quitting` after taking `daemonMu`, the same as the poller. If it is set, it unlocks and returns `errors.New("quitting")` without running EnsureDaemon. The new test is `TestRetryDaemon_QuittingSkipsEnsure`.

### IN-02: AbortSignal.timeout threw on older browsers

**Status:** fixed
**Files modified:** `frontend/src/components/WebShareSessionView.tsx`, `frontend/src/components/__tests__/WebShareSessionView.test.tsx`
**Commit:** 42d34d3a
**Applied fix:**
- A module-level `timeoutSignal(ms)` uses `AbortSignal.timeout` when it exists. Otherwise it uses an `AbortController` aborted by a `setTimeout`.
- `probeShare` uses `timeoutSignal`, so the 10 s timeout still applies on old browsers. The review's one-line `undefined` fallback would have dropped it.
- The new test deletes `AbortSignal.timeout`, then checks that fetch is still called and that the probe resolves `retry` exactly at 10 s.

## Skipped Issues

None.

## Verification

**Where it ran:** each fix was edited, tested and committed in the isolated worktree `.claude/worktrees/rf-05-27183-*` on the temp branch `gsd-reviewfix/05-27183`. Frontend dependencies were installed there with `pnpm install --frozen-lockfile --offline`.
- The full gate ran in that worktree after the last code commit. The only later commit was the TESTING.md doc (26cdfd3e).
- The tip of `gsd/phase-05-overnight-survival` had not moved (still 96e8c6d8), so the branch was fast-forwarded with no rebase.
- The worktree, temp branch and recovery sentinel were then removed.
- `tests/check-traceability-paths.sh` was re-run in the main checkout after the merge.

The main checkout holds exactly the tree that was tested.

Gate results, all with `GOTOOLCHAIN=go1.26.11`:

| Check | Result |
|-------|--------|
| `go build ./... && go vet ./...` | Pass |
| `go test -race -short -count=1 -skip '^(TestAPIAppearance_GetPatch\|TestAPITerminalFont_GetPatch)$' ./...` | Pass on the second full run (exit 0, 12 packages ok). See the note below. |
| `go test -race -count=1 ./internal/daemon -run 'TestCLIAttach\|TestRemote'` (without -short, because the CLI loop changed) | Pass |
| `pnpm exec tsc --noEmit` | Pass |
| `pnpm exec vitest run` | 87 files, 1171 tests. 1 failure: the known SharePopover Copy flake. That file passed 29/29 on rerun. |
| `bash tests/check-traceability-paths.sh` | Pass: 131 paths (worktree and main checkout) |

**Flake note:** the first full Go run failed `TestRevoke_SubscribeRaceClosed` in `internal/webserver`. Its TLS WebSocket dial hit its context deadline under the full-suite `-race` load. That test is not on the known-flake list. Nothing in this iteration touches `internal/webserver` or the relay join path. It then passed when its package ran alone, passed 5 more times with `-count=5`, and passed in a second full-suite run.

**TESTING.md** (commit 26cdfd3e):
- Counts are unchanged, because there are no new test files.
- An iteration-3 note was added.
- The Section 4 SHAR-07 rows were updated for loop_test.go (the test rename plus the new older-host test), relayClient.test.ts, TerminalPanel.reconnect.test.tsx, WebShareSessionView.test.tsx, app_respawn_test.go, App.daemon-recovered.test.tsx and process_test.go.

**Not touched:**
- Not staged: `.gitignore`, `.planning/state.json`, `.ignore`, `agenthub`.
- No other `.planning` file was changed.
- The token-gated share route, `requireCapability`, the grant re-check before the replay, and the CSP are unchanged.

---

_Fixed: 2026-11-18T09:05:44Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 3_
