---
phase: 09-upgrades-keep-sessions
plan: 04
subsystem: daemon-handoff
status: complete
tags: [upgrade, handoff, scm_rights, daemon, fault-injection, 1012]
requirements-completed: [UPGR-01, UPGR-02, UPGR-03]
requirements-note: "Daemon level. 09-05 adds the process around runTakeover (portty daemon --takeover, the status pipe) and the EnsureDaemon branch; until then nothing starts a takeover."
dependency_graph:
  requires:
    - "09-01 HealthResponse.Handoff, frozenRetryBudget, the 503 + Retry-After client wait"
    - "09-02 fdpass Send/Recv/PeerUID/PeerPID, NativePTYBackend.Adopt, pty.WatchExit, Session.HandoffFiles, WebServer.Listener/StartOn"
    - "09-03 Hub.Freeze/Thaw/Export/Digest/Start, NewHubFromState, HubManager.AddStaged, RestartViewers, Subscriber.Restart"
  provides:
    - "HandoffVersion = 1 and /health handoff: 1"
    - "POST /handoff on the unix socket mux (old side)"
    - "runTakeover(ctx, takeoverDeps) int with the outcome-code table and moveFailedReason"
    - "serveDaemon(ctx, api, engine, socketPath): runDaemonCore's serve tail with the SIGTERM guard"
    - "the freeze gate (503 Retry-After: 1 on writes)"
    - "handoffFault seam and the vmpreview hook PORTTY_VMPREVIEW_HANDOFF_FAULT"
    - "browser share viewers close with 1012 on RestartViewers"
  affects: [09-05 RunTakeover and EnsureDaemon, 09-06 TESTING.md and integration test, 09-07 VM rows]
tech-stack:
  added: []
  patterns:
    - "framed gob protocol on a hijacked unix conn; fd frames are a fixed 9-byte header sent with SCM_RIGHTS, never with a payload"
    - "the 101 response read byte by byte, so nothing sits in a buffer ahead of an fd frame"
    - "fault seam with a die sentinel (errHandoffDie): plain error = fail/roll back, errHandoffDie = die"
    - "in tests a recorded exit is followed by what the dead process would no longer hold (listener copies, fds), reached only where the exit seam returns"
key-files:
  created:
    - internal/daemon/handoff.go
    - internal/daemon/handoff_test.go
    - internal/daemon/handoff_fault_test.go
    - internal/daemon/handoff_vmpreview.go
  modified:
    - internal/daemon/api.go
    - internal/daemon/api_test.go
    - internal/daemon/engine.go
    - internal/daemon/share.go
    - internal/daemon/process.go
    - internal/webserver/server.go
decisions:
  - "A failed COMMITTED write polls the old pid for up to oldGoneWait (500 ms) before deciding, not one kill(pid,0): a write fails almost only because the old daemon is dying, and exiting while it is still a zombie would lose every session"
  - "The daemon log rotation and the socketLost watch start in serveDaemon, after runTakeover returns 0, not inside runTakeover before COMMITTED"
  - "Old-side death vs failure at a step: errHandoffDie dies (exit seam, conn closed, no thaw); any other fault error rolls back"
  - "A connection failure before COMMIT first reads any pending ABORT (50 ms); an ABORT always wins over the old-pid-gone rule (threat I1)"
  - "At COMMITTED the old daemon closes the share server with ws.Stop (Listener() must not be closed directly) and its lock copy before the exit seam"
  - "Config.APIVersion = APIVersion is set in defaultNewShareServer, so a normal share start reports it too"
metrics:
  duration: ~75m
  completed: 2026-11-19
estimate:
  tokens: 140000
  tasks: 2
actuals:
  tokens: 25900
  tasks: 2
  commits: 4
plan_head_before: 59abb8084423c547c220f2b2adcfa03ead4c2403
plan_head_after: 714a183faf05d26ea9eec722e948eab54e26261b
---

# Phase 9 Plan 04: In-process two-phase handoff Summary

A takeover daemon now connects to the running daemon's `/handoff`, receives every live session (PTY master and slave, ring with its end offset, history, emulator seed, name, workDir, createdAt, pid), the share records with their tokens, the socket, relay and share listeners and the daemon lock over SCM_RIGHTS, imports them into staged hubs, and only after READY and COMMIT starts reading. The old daemon closes its listener copies without unlinking, sends every viewer 1012 and exits without ending a session. A fault at any step on either side leaves the old daemon serving, and no fault lets two processes read one PTY.

## Contract for 09-05

```go
const HandoffVersion = 1

type takeoverDeps struct {
	socketPath string
	api        *API   // fresh NewAPI(NewSessionEngine()); serves nothing yet
	build      string // BuildVersion
}
func runTakeover(ctx context.Context, d takeoverDeps) int
func moveFailedReason(code int) string
func serveDaemon(ctx context.Context, api *API, engine *SessionEngine, socketPath string)
```

- runTakeover takes no lock and binds nothing. On 0, `d.api` already serves the received unix and relay listeners (and the share server if records came) and holds the lock in `api.lock`. The caller must then run `serveDaemon(ctx, api, api.engine, socketPath)`, which starts the daemon log rotation and the socketLost watch and carries the SIGTERM guard. On any other code everything received was closed and the process should exit with that code.
- Call `AugmentServicePath()` before `NewSessionEngine()`, as runDaemonCore does.
- The takeover logs through the standard `log` package (lines start `handoff:`), so its pre-COMMIT lines go to whatever stderr the spawner gave it (daemon.log).

### Outcome codes and reasons

| Code | Meaning | moveFailedReason |
|------|---------|------------------|
| 0 | committed | (none) |
| 1 | anything else (dial failure, a 404, a new-side fault, an undeliverable COMMITTED with the old daemon alive, the watchdog) | the new version exited |
| 3 | busy (another handoff in progress) | another update is already moving them |
| 4 | refused (uid, protocol version, older release, unknown peer pid) | the running version refused |
| 5 | import failed (bad frame, cap exceeded, validation, adopt, "share certificate unavailable") | the new version could not load them |
| 6 | timed out, aborted by the old daemon, or the old daemon went away alive before COMMIT | the handoff timed out |

## Wire format as built

- Request: `POST /handoff HTTP/1.1` with `Upgrade: portty-handoff/1` on the daemon socket. The handler checks the header, hijacks, requires a `*net.UnixConn` with nothing buffered, and writes `101 Switching Protocols`. The takeover reads the 101 one byte at a time.
- Frame: 1 type byte, 4-byte big-endian length, gob payload (COMMIT and COMMITTED have none). Types: 1 HELLO, 2 HELLO-OK, 3 REFUSE, 4 GLOBAL, 5 SESSION, 6 READY, 7 ABORT, 8 COMMIT, 9 COMMITTED, 10 fds.
- fd frame: 9 bytes (type 10, index uint32, count uint32) in one `fdpass.Send`; read with `fdpass.Recv` for the exact count. Index 0 is GLOBAL's, index i+1 is session i's.
- Messages in order:
  - `hsHello{Version, Build, PID}`, answered by `hsHelloOK{PID}` or `hsRefuse{Code, Reason}`
  - `hsGlobal{Sessions, ShareListener, Shares map[id]{GrantID, Token}}` with fds [lock, API unix listener, relay TCP listener, share TCP listener if ShareListener]
  - per session `hsSession{ID, Name, WorkDir, CLI, CreatedAt, Pid, Hub relay.HubState}` with fds [master, slave]
  - `hsReady{Digests map[id][32]byte}` or `hsAbort{Reason}`, then COMMIT, then COMMITTED
- Caps: any frame 64 MiB; SESSION `relay.DefaultScrollbackBytes + relay.HistoryMaxBytes + 1 MiB`; at most 256 sessions; name, workDir and CLI at most 4 KiB each. A share record for a session that did not arrive is dropped.

## Timeouts block (handoff.go)

| Var | Value | Where |
|-----|-------|-------|
| freezeTimeout | 2 s | relay Hub.Freeze's own bound (hubs freeze concurrently); in the sum only |
| stateSendTimeout | 5 s | old: one write deadline for GLOBAL plus every SESSION |
| readyTimeout | 5 s | old: READY after the state was sent |
| committedWatchdog | 5 s | new: COMMIT to COMMITTED, else the exit seam |
| takeoverStepTimeout | 10 s | new: read deadline per frame before COMMIT; old: HELLO wait |
| handoffDialTimeout | 2 s | new: dial |
| certWarmTimeout | 10 s | new: warm before HELLO |
| restartDrain | 1 s | old: wait for the viewers' 1012 close handshakes |
| oldGoneWait | 500 ms | new: kill(pid,0) poll before deciding the old daemon is gone |

freezeTimeout + stateSendTimeout + readyTimeout + committedWatchdog = 17 s, below frozenRetryBudget (20 s). TestHandoff_TimeoutsFitClientRetry pins it.

## Step names as implemented

- old: `after-hello` (after HELLO-OK), `after-freeze`, `mid-state-send` (after the first session when there are two or more), `after-state-sent`, `after-ready-before-commit`, `after-commit-before-committed`. Plus the trace point `abort-written` (between the ABORT write and the Thaw), whose error is ignored.
- new: `after-hello`, `after-global`, `after-state`, `before-ready`, `after-commit-before-serve`, `after-serve-before-committed`, `committed-write-fails`.
- Semantics of a non-nil error from `handoffFault`: `errHandoffDie` (or wrapping it) dies at that step; any other error fails there (old side rolls back with ABORT; new side sends ABORT and returns). After COMMIT any error on the new side is a death. `committed-write-fails` turns the COMMITTED write into that error.

## vmpreview fault syntax

`PORTTY_VMPREVIEW_HANDOFF_FAULT=<side>:<step>:<action>` in a `-tags vmpreview` build. side is `old` or `new`; action is `exit` (logs `handoff-fault: exiting at …`, then `os.Exit(1)`) or `pause<N>` (logs `handoff-fault: paused at <side>:<step> pid=<pid>` and sleeps N seconds). An old-side fault needs the old daemon itself to be a vmpreview build started with the variable. The hook never returns an error, so it never triggers the in-process fail path.

## Tests added (for 09-06 / TESTING.md)

| File | Tests |
|------|-------|
| internal/daemon/handoff_test.go (new) | TestHandoff_HappyPath (T-D1), TestHandoff_RestartDrainBounded, TestHandoff_CertWarmBeforeHello (warm first; warm fails with shares → 5), TestHandoff_RouteOnlyOnUnixSocket, TestHandoff_RefusesOtherUID, TestHandoff_RefusesOlderRelease, TestHandoff_NoSecretsInLog |
| internal/daemon/handoff_fault_test.go (new) | TestHandoffFault_EveryStep (T-D2, 10 steps), TestHandoffFault_NoThawUntilTakeoverExits (B1, 2 steps), TestHandoffFault_CommittedWriteFailsOldAlive (alive / gone), TestHandoffFault_OldDiesAfterStateSent (T-D3), TestHandoffFault_OldDiesMidState (gone / alive), TestHandoffFault_OldDiesAfterCommit (W2), TestHandoffFault_NewHangsAfterCommit, TestHandoffFault_AbortBeforeThaw (I1), TestHandoff_OneAtATime (T-D4), TestHandoff_FreezeGate (T-D5), TestHandoff_SIGTERMAfterCommitKeepsSocket (T-D6, after / before COMMIT), TestHandoff_LockHeldThroughout (T-D7), TestHandoff_TimeoutsFitClientRetry (W3), TestHandoffFault_SeamNilInRelease |
| internal/daemon/api_test.go (extended) | TestAPIHealth now expects `"handoff":1` |

The harness checks `lockHeld` at every fault-seam step of every test and fails in cleanup if the lock was ever free. Every test kills each shell it started (`kill -9` on the process group) and uses a temp HOME, a `/tmp/dtestN_ho.sock` socket and a temp lock.

### Fault cases (EveryStep)

`old:after-hello`, `old:after-freeze`, `old:after-ready-before-commit`, `new:after-hello`, `new:after-global`, `new:after-state`, `new:before-ready`, `new:after-commit-before-serve`, `new:after-serve-before-committed`, `new:committed-write-fails`. For each: non-zero code; the count of PTY, listening-socket and lock descriptors back to its baseline; every session running; POST /sessions accepted; the old ring and the connected viewer contiguous (for the two post-serve deaths, contiguous once united with what the takeover's ring read); a resume from an offset taken before the attempt contiguous.

## Mutation notes

Task 2's tests were written after the implementation (see deviation 6), so each key rule was checked by a temporary edit, reverted with a per-file checkout:

| Mutation | Test that failed |
|----------|------------------|
| Old side resumes on a COMMITTED read error without waiting for the takeover's exit (removed `<-peerGone`) | NoThawUntilTakeoverExits: "old daemon read on (ring 40 -> 52) before the takeover was gone" |
| rollback thaws before writing ABORT | AbortBeforeThaw: "old ring grew from 40 to 108 before ABORT was written" |
| lost() skips the pending-ABORT read | AbortBeforeThaw: "runTakeover = 0, want 6" |
| freeze gate disabled | FreezeGate: POST 201 while frozen, /shutdown reached its handler, CreateSession returned during the freeze |
| SIGTERM guard removed | SIGTERMAfterCommitKeepsSocket: socket unlinked; shutdown ran before the rollback |
| one-at-a-time check removed | OneAtATime: second takeover 6, want 3 (plus a panic on the shared done channel) |
| old-dies save removed (always 6 on a lost conn) | OldDiesAfterStateSent and OldDiesMidState: 6, want 0 |
| failed COMMITTED write never exits | CommittedWriteFailsOldAlive and NoThawUntilTakeoverExits: 0, want 1 |
| watchdog does not exit | NewHangsAfterCommit: the exit seam never ran |
| no restart drain wait | RestartDrainBounded (exit 149 µs after COMMITTED) and HappyPath (close frames 150 to 219 µs after the exit) |
| abort cleanup leaves adopted session files open | EveryStep/new:before-ready: descriptor count 7 after, 5 before |

## Verification

- `go build ./... && go vet ./internal/daemon/ ./internal/webserver/`: clean. `go vet -tags vmpreview ./internal/daemon/`: clean.
- Task 1 verify: `go test -race -count=1 -run 'TestHandoff_|TestAPIHealth|TestShare|TestRunDaemon' ./internal/daemon/` → `ok internal/daemon 4.052s`. Every grep gate passed.
- Plan suite (run once): `go test -race -short -count=1 ./internal/daemon/ ./internal/webserver/` → `ok internal/daemon 49.559s`, `ok internal/webserver 12.787s`. No flake reruns needed.
- `bash tests/check-brand.sh` → `OK: brand gate clean`. `const MaxInputPayload = 32767` unchanged. csp_mw.go, capability_mw.go and tailnet/live.go unchanged against b4d63644.
- TESTING.md is not edited here (the plan gives it to 09-06).

## Deviations from Plan

### Auto-fixed / adjusted

**1. [Rule 1 - Correctness] A failed COMMITTED write polls for the old daemon (up to 500 ms) instead of one check**
- **Found during:** Task 2 design
- **Issue:** The plan says a takeover whose COMMITTED write fails while the old pid is alive exits at once (test: within 50 ms). A write to the old daemon's socket fails almost only because that process is exiting, and kill(pid, 0) succeeds on an exiting or zombie process. Exiting at once there would leave nobody holding the PTYs, ending every shell: the rock-solid case this plan exists for.
- **Fix:** The same `oldGone()` poll as the pre-COMMIT rule. A truly alive old daemon is still frozen and waiting for this process to exit, so the extra wait (at most 500 ms) never creates two readers. TestHandoffFault_CommittedWriteFailsOldAlive bounds the exit at `oldGoneWait + 150 ms`.
- **Files:** internal/daemon/handoff.go, internal/daemon/handoff_fault_test.go. **Commit:** c1aaa78b, 60acc214

**2. [Scope] Log rotation and the socketLost watch start in serveDaemon**
- **Issue:** The plan lists them inside runTakeover before COMMITTED. They are process-level and are already part of runDaemonCore's serve tail, which this plan extracts for 09-05 to reuse.
- **Fix:** runTakeover returns 0 right after COMMITTED; 09-05 calls serveDaemon next. The difference is a few milliseconds, and neither affects which process reads a PTY.

**3. [Rule 1 - Correctness] A pending ABORT is read before a connection failure is acted on**
- **Found during:** Task 2 (AbortBeforeThaw)
- **Issue:** After a READY timeout the old daemon writes ABORT and closes. The takeover's READY write then fails with EPIPE while the ABORT sits unread. Without a read, an old pid reported gone would make it proceed as committed on a stale snapshot (threat I1).
- **Fix:** `lost()` first tries to read a frame for 50 ms; an ABORT returns 6.
- **Commit:** c1aaa78b

**4. [Test] The share server answers 405, not 404, for POST /handoff**
- Its `GET /` catch-all matches every path, so a POST gets 405. RouteOnlyOnUnixSocket accepts 404 or 405 and checks that no handoff started. The relay port answers 404.

**5. [Design] Fault semantics and one extra trace point**
- The plan's "behave as failing or dying" needed a way to choose: `errHandoffDie` dies, any other error fails. The old side also calls the seam at `abort-written` (error ignored) so AbortBeforeThaw can see the ring between the ABORT write and the Thaw.

**6. [Process] TDD order**
- Task 1: RED commit f819e4c3 (tests; the package did not build without handoff.go), GREEN c1aaa78b. The implementation for both tasks went into c1aaa78b, so Task 2's tests (60acc214) passed when first run. Each Task 2 rule got the mutation check listed above instead of a RED run. The Task 2 GREEN commit (714a183f) adds only the vmpreview hook.

**7. [Rule 2] The old daemon refuses to hand over without a lock or its listeners**
- GLOBAL needs `api.lock`, a unix listener and a TCP relay listener; without them the handoff rolls back with ABORT. runDaemonCore now sets `api.lock`.

**8. [Design] The old daemon closes its lock copy at COMMITTED, before the exit seam**
- Harmless in production (the exit closes it anyway). In process it makes T-D7's final `lockHeld` check prove the takeover's copy holds the lock.

**9. [Scope] Config.APIVersion in defaultNewShareServer**
- The plan sets it for the share server built at import. Both paths build through the same factory, so a normal share start reports `api` on /info as well, which CONTEXT asked for. The vmpreview factory (share_vmpreview.go) is unchanged.

## Known Stubs

None.

## Threat Flags

None beyond the plan's threat register. T-09-15..T-09-25 are mitigated as planned and pinned by the tests named above. T-09-26 (a SIGSTOPped takeover after COMMIT keeps the old daemon frozen) is accepted as planned.

## Notes for later plans

- 09-05: a takeover refused with 4 for "older release" happens when a GUI A is relaunched after an upgrade to B. Show the DEBT-08 question then; Not Now keeps B.
- 09-05: the takeover must not pass a kill func to startDaemon (09-01 note); after COMMIT it owns the sessions.
- 09-06/09-07: killing the old daemon at `old:after-freeze` ends the shells (no fd has left it yet). The old-dies VM row should pause at `old:after-state-sent`.
- In process, a recorded exit is followed by cleanup the real exit would do (listener copies closed, fds closed). That code is unreachable in production because the exit seams never return there.

## Self-Check: PASSED

- FOUND: internal/daemon/handoff.go, internal/daemon/handoff_test.go, internal/daemon/handoff_fault_test.go, internal/daemon/handoff_vmpreview.go
- FOUND (ancestors of HEAD): f819e4c3, c1aaa78b, 60acc214, 714a183f
