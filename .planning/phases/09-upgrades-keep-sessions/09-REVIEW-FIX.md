---
phase: 09-upgrades-keep-sessions
fixed_at: 2026-11-20T09:00:00Z
review_path: .planning/phases/09-upgrades-keep-sessions/09-REVIEW.md
iteration: 1
findings_in_scope: 12
fixed: 11
skipped: 1
status: partial
---

# Phase 9: Code Review Fix Report

**Fixed at:** 2026-11-20
**Source review:** .planning/phases/09-upgrades-keep-sessions/09-REVIEW.md
**Iteration:** 1
**Scope:** all (critical, warning and info), as the caller asked

**Summary:**
- Findings in scope: 12
- Fixed: 11 (CR-01, WR-01..WR-05, IN-01, IN-02, IN-03, IN-05, IN-06)
- Skipped: 1 (IN-04, recorded as an accepted limit instead)

Every fix has a regression test that failed before the fix: by assertion where the old code compiled, or by compilation where the test needs the new API (WR-03 `HubState.Digest`, WR-05 `checkGlobalFiles` and `lockPath`). No new test files; TESTING.md Section 2 (note) and the Section 4 UPGR rows name the tests (commit 515ccb98).

## Fixed Issues

### CR-01: The COMMITTED watchdog can kill the takeover after COMMITTED was delivered

**Files modified:** `internal/daemon/handoff.go`, `internal/daemon/handoff_fault_test.go`
**Commit:** 07329682
**Status:** fixed: requires human verification (a concurrency fix)
**Applied fix:**
- The watchdog and the COMMITTED write share one `atomic.Int32` (pending, writing, dead). The watchdog exits only through `CompareAndSwap(pending, dead)`. Once the write has started, the watchdog only closes the conn, so a write still in flight fails and takes the existing `oldGone()` path. A delivered COMMITTED is never followed by an exit. The main goroutine's `CompareAndSwap(pending, writing)` replaces the old `t.dead.Load()` check after the write.
- `oldHandoff.finish(peerGone)` checks the takeover's kernel exit watch first, before it closes the listener copies. If the takeover is already gone, it calls `resumeAfterCommit()` (Thaw, gate lifted) instead of exiting.
- Two trace points were added for the tests: `new:committed-written` and `old:committed-received`.
- Tests:
  - `TestHandoffFault_WatchdogAfterCommittedWritten`: the takeover stalls `committedWatchdog + 300 ms` (about 5.3 s with the real 5 s watchdog) right after writing COMMITTED. Before the fix, the takeover exited 1 and the old daemon exited 0. After the fix, the takeover commits and serves.
  - `TestHandoffFault_TakeoverGoneAtFinish`: the peer watch fires as COMMITTED arrives. Before the fix, the old daemon exited. After the fix, it thaws, keeps its listeners and reads on.

### WR-01: The freeze gate is a check, not a barrier

**Files modified:** `internal/daemon/api.go`, `internal/daemon/handoff.go`, `internal/daemon/handoff_fault_test.go`, `internal/daemon/client.go` (comment)
**Commit:** 6a47eb8c
**Status:** fixed: requires human verification (a concurrency fix)
**Applied fix:**
- Mutating requests hold `API.writeGate` (a `sync.RWMutex`) for reading for their whole run, through `TryRLock`. While frozen, or while a handoff holds the lock, they get 503 with `Retry-After: 1`.
- `serveHandoff` sets `frozen` first. `drainWrites(writeDrainTimeout = 1 s)` then polls `TryLock` until the writes in flight finish, and the handoff rolls back if they don't. It holds the lock until `resume()` (rollback, or resume after COMMIT), or until it exits.
- `maxFrozen` and `TestHandoff_TimeoutsFitClientRetry` include the drain: 18 s, still below the 20 s client retry budget.
- Test: `TestHandoff_FreezeGateDrainsWrites` holds a real `handleCreateSession` and a real `handleRegenerateSigningKey` behind the gate while a handoff starts. It checks three things:
  - new POST /sessions and regenerate-key requests get 503 with `Retry-After: 1`;
  - the hubs don't freeze while the write is in flight;
  - the created session moves to the takeover, and the takeover serves the regenerated key.

  Before the fix, the hubs froze with the write in flight.

### WR-02: The SESSION frame cap is smaller than what Export produces

**Files modified:** `internal/relay/handoff_state.go`, `internal/relay/handoff_state_test.go`, `internal/daemon/handoff.go`
**Commit:** 5a224a0f
**Applied fix:**
- New `relay.HandoffStateMaxBytes` = ring max + history max + 10 B per history line (gob length and wrap flag) + a two-screen 500x200 seed at 64 B a cell + 1 MiB. That is 18,405,024 bytes.
- `maxSessionFrame` now uses it. `writeFrame` checks the SESSION cap on the sender too, and `sendSession` names the session in the error.
- A `shortcut:` comment records the remaining limit: a window much larger than 500x200 with truecolor on every cell can still exceed the cap.
- Test: the reviewer's probe, now `TestHandoffState_FitsSessionFrameCap`. Its 400x120 truecolor hub with a full history and the alt screen gives ring=262,144, history=4,186,619, seed=3,292,065 and gob=7,742,898. The test requires at least 2x margin. It failed against the old 5,505,024-byte cap.

### WR-03: The READY digest compares two builds' renderers

**Files modified:** `internal/relay/handoff_state.go`, `internal/relay/handoff_state_test.go`, `internal/daemon/handoff.go`
**Commit:** 225311a5
**Status:** fixed: requires human verification (a protocol semantic change)
**Applied fix:**
- `HubState.Digest()` hashes every field of the transferred state in a fixed order with map keys sorted: ring end, max, truncated flag and bytes; history caps, lines and wrap flags; modes; geometry offset and region; wraps; last move; charset state; grid and emulator sizes; Seed; Tail.
- The old daemon hashes the state it sent (`oldHandoff.sent`), and the takeover hashes the state it decoded, before import. READY compares the two. `Hub.Digest()` (the render digest) remains for the relay tests only.
- Test: `TestHandoffState_StateDigest` checks four things:
  - the digest is stable across map order and a gob round trip;
  - drawing over the rebuilt hub's screen changes the render digest but not the READY digest;
  - each of 30 single-field mutations changes the digest, so a corrupted transfer is still caught;
  - the field counts of `HubState` and its sub-structs are pinned, so a new field fails the test until it is hashed.
- Note: this changes what READY means within `HandoffVersion` 1. v1.2 is unreleased (the latest tag is v1.1.0), so no shipped build speaks the old meaning. A VM build from before this commit would get a digest mismatch and roll back, which is safe.

### WR-04: Host-tab reconnect and the Quit dialog treat a failed daemon call as "no sessions"

**Files modified:** `app.go`, `app_test.go`, `frontend/src/wailsjs/go/main/App.js`, `frontend/src/wailsjs/go/main/App.d.ts`, `frontend/src/lib/hostReconnect.ts`, `frontend/src/lib/hostReconnect.test.ts`, `frontend/src/App.tsx`, `frontend/src/components/__tests__/App.daemon-restart.test.tsx`, `frontend/src/components/__tests__/TerminalPanel.reconnect.test.tsx`
**Commit:** 3e98ac43
**Applied fix:**
- New bindings `ListSessionsChecked() ([]SessionInfo, error)` and `GetRelayPortChecked() (int, error)`. A port of 0 or less counts as an error. The plain `ListSessions` and `GetRelayPort` now delegate to them and keep returning `[]` and `0` for their other callers.
- `hostReconnectVerdict` reads the checked bindings, so a failed read retries instead of ending the tab.
- The Quit handler uses `ListSessionsChecked`, so a fast failure opens the unknown-count copy.
- The hand-maintained wailsjs stubs list both new methods.
- Tests:
  - vitest: with the default deps, the verdict is `retry` when the plain binding would say `[]`. A rejected session read shows "Portty couldn't check whether any sessions are running." and not "No active sessions.". A good read lists the running sessions. The V-05b hung-read test now targets `ListSessionsChecked`.
  - Go: `TestCheckedBindings_FailedReadIsAnError`.

### WR-05: Received listeners and the lock are not validated

**Files modified:** `internal/daemon/handoff.go`, `internal/daemon/handoff_test.go`, `internal/daemon/process.go`, `internal/webserver/server.go`, `internal/webserver/listener_test.go`, `internal/relay/handoff_state.go` (gofmt whitespace of the WR-03 helper only)
**Commit:** d8c2201e
**Applied fix:**
- `takeoverDeps.lockPath` is new. Production passes `<socket dir>/daemon.lock`.
- `takeGlobalFiles` calls `checkGlobalFiles`, which requires three things:
  - the lock fd is daemon.lock (`os.SameFile` on fstat and stat);
  - the API listener's `UnixAddr.Name` equals the socket path;
  - the relay listener is TCP on loopback.
- The new `WebServer.CheckListener` requires the share listener to be TCP on the configured bind IP, and on the configured port when one is set. In production that is the health IP and 7444. It runs in `finalize()` before COMMIT, so a refusal aborts and the old daemon keeps serving. It also runs in `StartOn`.
- The WR-03/WR-07 `assertTailnetBindIP` guard is unchanged. Because `CheckListener` forces the served address to equal the configured one, the guard now covers the address actually served.
- Tests:
  - `TestHandoff_RefusesForeignFiles` unit-tests these cases: a foreign lock file, another socket, a relay on 0.0.0.0, a relay on the LAN, and a relay that is not TCP. End to end, a takeover given a foreign `lockPath` gets `takeoverImport`, the descriptor count is back at its baseline, and the old daemon serves.
  - `TestWebServer_StartOnRefusesForeignListener` covers another IP, a tailnet IP, another port, and the matching case.

### IN-01: HELLO and READY are read with a 64 MiB cap

**Files modified:** `internal/daemon/handoff.go`, `internal/daemon/handoff_test.go`
**Commit:** 887f3e1b
**Applied fix:** HELLO is capped at 4 KiB (`maxHelloFrame`). READY is capped at `maxHandoffSessions * (maxHandoffText + 128)`, about 1 MiB (`maxReadyFrame`). Test: `TestHandoff_HelloCapped` sends a header claiming a 1 MiB HELLO. The daemon now drops the connection at once (EOF). Before the fix, it waited for the body until the read timed out.

### IN-02: The adopted pid is validated only as > 0

**Files modified:** `internal/daemon/handoff.go`, `internal/daemon/handoff_test.go`, `internal/pty/adopt_darwin.go`, `internal/pty/adopt_test.go`
**Commit:** 105380c1
**Applied fix:** `takeover.invalid` and `Adopt` both reject pid <= 1. Tests: `TestHandoff_RejectsPidOne` and `TestAdopt_RefusesPidOne`. The reviewer's further suggestion, `getpgid(pid) == pid`, was not added: the shells run in their own session, and on macOS `getpgid` across sessions may return EPERM.

### IN-03: A failed adopt leaks its exit-watch thread and a registry entry

**Files modified:** `internal/pty/adopt_darwin.go`, `internal/daemon/engine.go`, `internal/daemon/engine_test.go`
**Commit:** f5b4f2ed
**Applied fix:**
- `Adopt` arms `WatchExit` after `pollableDup` succeeds. Pitfall 8 still holds: a shell that exited before this point is an unreaped zombie of the frozen old daemon, so the kqueue registration returns ESRCH and the shell reads as gone. That is the reviewer's probe result.
- New `NativePTYBackend.Forget(id)`. `importSession` calls it on the error path after `Adopt`.
- Test: `TestImportSession_FailureForgetsAdopted` covers the registry part. Before the fix, the backend kept 1 session. The thread-ordering part has no test, because a dup failure cannot be injected.

### IN-05: Quit All can report success against the exiting old daemon

**Files modified:** `internal/daemon/client.go`, `internal/daemon/client_test.go`
**Commit:** 7b604ae9
**Applied fix:**
- When `/shutdown` ends in a transport error that is not a timeout, `ShutdownDaemon` closes the idle connections and reads `/health` once on a fresh connection, with the same 2 s per-attempt client. If a daemon answers, it sends `/shutdown` to that daemon once more.
- A timeout means a hung daemon, not a dropped connection, so Quit All does not wait on it a second time. The existing 5 s `TestQuitAll_HungDaemon` bound still holds.
- About "the version" in the brief: whichever daemon answers is the one that now owns the socket and the sessions, so it is stopped whatever its version. A version comparison would change no decision, so none was added.
- Test: `TestShutdownDaemon_DroppedThenRechecked`. When something still answers, two /shutdown requests go out; before the fix, only one did. When nothing answers, one goes out and the result is success.

### IN-06: "Restart Now" after a failed move runs the whole handoff again

**Files modified:** `app.go`, `app_test.go`
**Commit:** 471502cb
**Applied fix:**
- `App.ensure()` passes `NoHandoff: true` while a Restart Now approval is pending (`restartApproved != 0`). Startup, Retry and the poller still try the handoff.
- This is safe because a handoff-capable daemon with live sessions always gets a handoff attempt before the question is asked. An open question therefore means the move failed, or the daemon has no handoff support, where the flag changes nothing.
- Test: `TestAnswerDaemonRestart_SkipsSecondHandoff`. It expects NoHandoff to be `[false true]`; before the fix it was `[false false]`.

## Skipped Issues

### IN-04: A handoff silently drops stopped sessions

**File:** `internal/daemon/engine.go` (`handoffEntries`)
**Reason:** it is not cheap. Moving a stopped session's card needs three things the protocol does not have: a SESSION frame without fds (today every session carries exactly 2), a stopped `pty.Session` built without a PTY, and a hub built from state with no reader. It was recorded as an accepted limit instead: a new row in the 09-CONTEXT "Decided autonomously" table, and a `shortcut:` comment on `handoffEntries`. Commit e98efb97. No shell is lost.
**Original issue:** exited sessions, with their exit code, duration and final output, vanish from the list after an upgrade, and this was not recorded as a decision.

## Verification

The gates ran in the agent's isolated worktree (`.claude/worktrees/agent-a54a747d8d8869935`, branch `worktree-agent-a54a747d8d8869935`), at HEAD 515ccb98 plus this report. The frontend needed `pnpm install --frozen-lockfile` there. The results reproduce from the main checkout after a fast-forward.

The full gate ran once, with `GOTOOLCHAIN=go1.26.11`. All of it passed on the first run, so none of the known flakes needed a rerun.
- `go build ./... && go vet ./...`: exit 0, no output.
- `go test -race -short ./...`: exit 0. All 13 packages with tests pass: `portty`, attach, capability, daemon (70.1 s), fdpass, pty, relay (142.7 s), release, statusbar, tailnet, updater, wakewatch and webserver. Three packages have no test files. The fault suite and the fuzz seed corpus pass.
- `pnpm vitest run`: exit 0. 95 test files and 1304 tests pass.
- `pnpm exec tsc --noEmit`: exit 0, no output.
- `bash tests/check-traceability-paths.sh`: "OK: 163 traceability paths exist".
- `bash tests/check-brand.sh`: "OK: brand gate clean".
- `go vet -tags vmpreview ./internal/daemon/ ./internal/relay/`: exit 0.

Not covered here: the two-binary `TestHandoffIT` (env-gated, office Tart VM only) did not run. The share-listener check passes in tests because the test share server uses Port 0, so only the IP is compared; the production port comparison (7444) is exercised only by the webserver unit test.

---

_Fixed: 2026-11-20_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_

## Iteration 2

**Fixed at:** 2026-11-20
**Source review:** 09-REVIEW.md, section "Iteration 2"
**Scope:** all 5 findings (WR-01, WR-02, IN-01, IN-02, IN-03)

**Summary:** 5 in scope, 5 fixed, 0 skipped.

| Finding | Status | Commit(s) |
|---|---|---|
| WR-02 (it2) | fixed (a previous fixer did it; reviewed here and found correct) | 89c6e18f |
| WR-01 (it2) | fixed: requires human verification (a concurrency and route-gating change) | b09971f2, d4dd784c |
| IN-01 (it2) | fixed | 46e8d08a |
| IN-02 (it2) | fixed (no test: see below) | fc46b373 |
| IN-03 (it2) | fixed | fe32d895 |

### WR-02 (it2): READY digest depends on the HubState schema

**Commit:** 89c6e18f (cherry-picked from the stopped fixer's branch, where it was cdf95204's successor d9d4764c)
**Review:** correct.
- `writeSessionFrame` hashes through `io.MultiWriter`, and `readSessionFrame` through `io.TeeReader`.
- `readFrame` uses `io.ReadFull` for exactly 5 + n bytes, so both sides hash the same header and payload bytes and nothing from a later frame.
- The digest is used only on success, so a partial write or read cannot produce a false match.
- ABORT frames still come back as `*abortedError` through the tee.
- Test: `TestHandoff_SessionFrameDigest`. One changed payload byte that still decodes fails the digest; a frame from a later build's struct with an extra field passes.
- A cosmetic leftover: `runTakeover` sets the read deadline twice around `readSessionFrame`.

### WR-01 (it2): slow writes held the write-drain gate

**Files modified:** `internal/daemon/api.go`, `internal/daemon/share.go`, `internal/daemon/client.go`, `internal/daemon/handoff_fault_test.go`, `TESTING.md`
**Commits:** b09971f2, plus the follow-up d4dd784c (the first version warmed the certificate twice; `TestShare_SessionKilledDuringStartIsNotShared` caught it in the gate)
**Applied fix:**
- `freezeGate` no longer applies the barrier to `POST /peers/connect`, which only caches a peer cap that no handoff moves.
- `PUT /sessions/{id}/share` takes the gate itself:
  - Share-off is gated for its whole run.
  - Share-on runs the health check and, when no share server is running, the cert warm-up before it takes the gate. The drain then waits only for the server start and the mint, which take milliseconds.
  - `startShareServer` warms again only if the server stopped between that check and the start.
- New helpers `enterWrite` and `writeFrozen`, shared by the middleware and the share handler.
- `SetSessionShared` now goes through `doRetrying`, as every other write does. A share-on that meets the freeze lands on whichever daemon serves afterwards instead of failing with a bare 503. A 503 without Retry-After (Tailscale off) still returns at once.
- **Why the drain was not lengthened:** once restartDrain is counted (IN-01), the worst-case 503 window is 19 s of the 20 s client budget, which leaves no room for a longer drain. A longer drain also would not cover a cold certificate fetch, which can take up to 20 s. Moving the slow, stateless work out of the gate removes the cause. The one-line reason is in the `freezeGate` comment.
- Tests:
  - `TestHandoff_SlowShareOnDoesNotBlock`: a share-on held in its warm-up for longer than the drain. The handoff commits, and the share lands on the takeover. Before the fix the drain timed out, so the outcome was 6.
  - `TestFreezeGate_SlowRoutesSkipBarrier`: `/peers/connect` and PUT share skip the barrier and serve while frozen; other writes hold the gate and get 503.

### IN-01 (it2): maxFrozen left out restartDrain

**Files modified:** `internal/daemon/handoff.go`, `internal/daemon/client.go` (comment), `internal/daemon/handoff_fault_test.go`, `TESTING.md`
**Commit:** 46e8d08a
**Applied fix:**
- `maxFrozen()` now adds `restartDrain`, giving 19 s. The SIGTERM guard waits 1 s longer as a result.
- `TestHandoff_TimeoutsFitClientRetry` pins three things:
  - `maxFrozen()` equals the sum of all six terms;
  - `maxFrozen()` is less than `frozenRetryBudget`;
  - `maxFrozen()` + 1 s (the gate's Retry-After) is at most `frozenRetryBudget`.
- The client budget stays at 20 s. The margin is exactly one Retry-After wait.

### IN-02 (it2): Start leaked a refused listener

**Files modified:** `internal/webserver/server.go`
**Commit:** fc46b373
**Applied fix:** `Start` closes `raw` when `StartOn` returns an error.
**No regression test:** `CheckListener` cannot refuse a listener that `Start` opened itself. Every IPv6 or zoned `BindIP` fails at `net.Listen` first, and `Start` binds the configured port. A test would need a new seam.

### IN-03 (it2): the second /shutdown got a fresh budget

**Files modified:** `internal/daemon/client.go`, `internal/daemon/client_test.go`, `TESTING.md`
**Commit:** fe32d895
**Applied fix:**
- `doRetrying` takes its deadline as a parameter. `doJSON` and `SetSessionShared` pass now + `frozenRetryBudget`.
- `ShutdownDaemon` computes one deadline and passes it to both posts. The re-post always sends at least one attempt.
- Test: `TestShutdownDaemon_DroppedThenRechecked/the_second_post_shares_the_budget`. The sequence is a 503, a drop, then 503s forever, with a 2.5 s budget. The fix makes 4 posts; a fresh budget would make 5.

### Verification (iteration 2)

All gates ran in this agent's own git worktree, branched from gsd/v1.2 85647018. They did not run in the main checkout.

- `go build ./... && go vet ./...` (GOTOOLCHAIN=go1.26.11): exit 0, no output.
- `go test -race -short ./...`: exit 1, with one failure: `TestShare_SessionKilledDuringStartIsNotShared`. The cause was this fix's double warm-up, not a flake. It was fixed in d4dd784c. Every other package passed, and so did every other daemon test, including the new ones.
- The daemon package alone was then run once more: `go test -race -short ./internal/daemon/` exit 0 (75.5 s). The full suite was not run again, because the follow-up touches only `share.go`.
- `bash tests/check-traceability-paths.sh`: "OK: 163 traceability paths exist".
- `bash tests/check-brand.sh`: "OK: brand gate clean".
- No frontend files changed, so vitest and tsc did not run.
- Not covered: the two-binary `TestHandoffIT` (office Tart VM only) did not run.

_Iteration 2 fixed: 2026-11-20_
_Fixer: Claude (gsd-code-fixer)_
