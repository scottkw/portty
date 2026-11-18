---
phase: 05-overnight-survival
plan: 06
subsystem: cli
status: complete
tags: [cli, attach, reconnect, resume, liveness, shar-07, verf-01]
requires:
  - "05-01: ?resume= on the proxy route, Meta{replay} / Meta{offset} replay bracket, frame-byte offsets"
  - "05-02: MsgPong 0x04 answers to MsgPing; the proxy drops a silent upstream with no close frame"
provides:
  - "attach.Run(ctx, first, RunOptions) error: the remote CLI reconnect loop"
  - "attach.RunOptions, attach.Timing / DefaultTiming (15 s ping, 40 s dead, 1-30 s backoff, 30 s healthy reset), attach.Verdict"
  - "relay.IsTerminalClose(err): 1000 + share stopped / session ended"
  - "cmd_attach.go remoteTarget{url, reauth}, peerVerdict(err), CLI copy 'Portty: <session> on <host> ended.'"
  - "attach_viewers_test.go helpers: runCLI / loopCLI, newViewingDaemon, checkSameLines, captureDiag, lineTags"
affects: [05-07]
tech-stack:
  added: []
  patterns:
    - "One stdin reader per run; chunks read while disconnected are stamped stale and dropped, and the gap drains stdin too, so only the detach key acts"
    - "Per-connection frame reader owns the offset; the gap reads it only after that reader exited (channel happens-before, no lock)"
    - "Watchdog armed by the first MsgPong on each connection, silence measured on the wall clock (a Mac's sleep counts)"
key-files:
  created:
    - internal/attach/loop.go
    - internal/attach/loop_test.go
  modified:
    - cmd_attach.go
    - cmd_attach_test.go
    - internal/relay/close_reasons.go
    - internal/relay/close_reasons_test.go
    - internal/daemon/attach_viewers_test.go
decisions:
  - "Silence is measured with UnixNano (wall clock), per RESEARCH Pitfall 9: Go's darwin monotonic clock stops during sleep, so a wall-clock delta lets the first tick after wake drop a dead socket at once. Cost: a healthy socket may be redialed once after a long sleep, which is a cheap delta replay"
  - "Each redial of the loopback proxy is bounded at 10 s (dialTimeout), the same bound the proxy puts on its upstream dial, so a hung local daemon cannot stall the gap"
  - "A Stdout write failure (terminal gone) ends Run instead of reconnecting (outputError)"
  - "A failed input write does not decide the outcome alone: the loop closes the socket and lets the reader's error decide, so a host's final close that raced the write still ends the attach"
metrics:
  duration: 18min
  completed: 2026-11-17
  tasks: 3
  files: 7
estimate:
  tokens: 95000
  tasks: 3
actuals:
  tokens: 13500
  tasks: 3
  commits: 5
plan_head_before: d4a9c1aab47120b8086954174295ee04b7742379
plan_head_after: 5d9bc86647f9ebbe1ba50ad275cf7e88a2498f01
---

# Phase 5 Plan 06: CLI Reconnect Loop Summary

`portty attach <host>:<session>` now reconnects by itself after a drop, a silent socket or a restart of this Mac's daemon. The new `attach.Run` keeps raw mode and the status bar, shows `[reconnecting]`, backs off 1 to 30 s with ±20 % jitter, re-runs `GetRelayPort` + `ConnectPeer` before every redial, and redials the local proxy with `?resume=<offset>`. Keystrokes typed while disconnected are thrown away, and Ctrl-\ works in every state. The five-second no-output watcher behind UAT gap 1 is gone; the loop is the only source of `[reconnecting]`.

## What was built

- **`internal/attach/loop.go`:**
  - `Run(ctx, first, RunOptions)`. One goroutine reads stdin for the whole run. Each connection gets a frame reader. `MsgOutput` goes to Stdout and, once this socket's offset meta has arrived, adds `len(payload)+1` to the offset. `MsgMeta` updates the viewer count, the replay mode and the offset; a meta that fails to parse makes the offset untrusted, so the next redial asks for a full replay. `MsgPong` arms the watchdog.
  - Clear rule: with `ClearOnFull`, `CSI 2 J + CSI H` is written before the first output of a full replay (or output before any replay meta, from an older host), but only when output was already rendered. The first connection never clears; runAttach already did.
  - A `MsgPing` every `PingEvery`. Once a pong was seen on this socket, `DeadAfter` of silence makes the loop `CloseNow` the socket and reconnect.
  - Ending: detach or ctx done closes 1000 "detach" and returns nil. A close that `relay.IsTerminalClose` accepts is returned as is. Anything else calls `OnState("reconnecting")` and enters the gap. In the gap the loop waits out the backoff, runs Reauth (`VerdictShareStopped` returns a 1000 "share stopped" CloseError; `VerdictRetry` waits again) and dials. On success it calls `OnState("")`. It retries forever. The backoff resets only after a connection stayed open for `HealthyReset`.
- **`relay.IsTerminalClose`:** true only for 1000 with `share stopped` or `session ended`. It mirrors the frontend's `isTerminalClose`.
- **`cmd_attach.go`:**
  - `runAttach` takes `remote *remoteTarget` (nil means local). Remote attaches run `attach.Run`; `OnConnect` sends the initial resize and starts the SIGWINCH watcher for each connection. Local attaches keep `AttachSession` with a nil onFrame.
  - `attachRemote` builds the target. `url` uses the latest relay port. `reauth` calls `GetRelayPort` (an error means retry), then `ConnectPeer(host, sessionID)` mapped by `peerVerdict`: not_sharing, no_such_session and forbidden mean share stopped; anything else means retry. The new port is adopted only on OK.
  - `remoteAttachEnd` now also recognises `session ended` and prints `Portty: <session> on <host> ended.`. Both final ends restore the terminal, stop the bar, print the message and exit 0.

## Verification

- Task 1: build, vet, `--- PASS: TestCLIAttach_ReconnectsWithDelta` (also `-count=5`), every `TestCLIAttach_` test, `./internal/attach ./internal/relay`, and the root attach tests all pass under -race. Tracer gate: the automated verify was re-run end to end and passed (human_verify_mode end-of-phase).
- Task 2: vet; 9 `TestBackoffDelay|TestRun_` PASS lines (at least 7 required); `--- PASS: TestCLIAttach_IdleNoReconnecting`.
- Task 3 / plan gate: `go build ./... && go vet ./... && go vet -tags playwrightfixture ./cmd/playwright-fixture/ && go vet -tags vmpreview ./...` exit 0. 3 harness PASS lines and 2 root PASS lines. `go test -race -short -skip '^(TestAPIAppearance_GetPatch|TestAPITerminalFont_GetPatch)$' ./...` exit 0. One earlier full run hit a load flake in an unchanged test; see Deferred Issues.
- Acceptance greps: `func Run(` 1, `attach.Run(` in cmd_attach.go 1, `func IsTerminalClose(` 1, `SetConnectionState("reconnecting")` in cmd_attach.go 0, `5*time.Second` in cmd_attach.go 0, `func TestRun_InputWhileDisconnectedNeverSent` 1, `func TestCLIAttach_IdleNoReconnecting` 1, `^var DefaultTiming` 1, `func peerVerdict(` 1, `func TestCLIAttach_ViewingDaemonRestart` 1, `Reconnect (05-06, SHAR-07)` 1, ` ended\.` in cmd_attach.go 1. The `test(05-06)` commit edfd0f44 is older than the `feat(05-06)` commit 6e1c735a.
- Mutation check on the Task 2 tests. Each mutation was applied to loop.go alone and then reverted:
  - Arming the watchdog without a pong failed `TestRun_NoPongNeverDropped`.
  - Removing the healthy reset failed `TestRun_HealthyResetRestartsBackoff`.
  - Counting the payload without its type byte failed `TestRun_ResumeAndClearOnFull`.
  - Clearing before the first connection's output also failed `TestRun_ResumeAndClearOnFull`.

## TDD Gate Compliance

| Task | RED commit | GREEN commit | RED cause |
|------|-----------|--------------|-----------|
| 1 | edfd0f44 | 6e1c735a | `attach.Run` scaffold returned "not implemented"; the CLI never saw MARKER-1 (runtime failure, not a build failure) |
| 2 | 047e8957 | none needed | Tests only. Task 1 had already built the loop they pin, as the plan orders ("fix whatever the tests expose"). The mutation check above shows each test fails when its rule is broken |
| 3 | a198b76d | 5d9bc866 | `TestRemoteAttachEnd/session_ended`: `("", false)`, wanted `("Portty: build on office-air ended.", true)` |

`TestPeerVerdict`, `TestIsTerminalClose`, `TestRun_FinalCloseStops` and the three Task 3 harness tests were green at their RED commit, because Task 1 built `peerVerdict`, `IsTerminalClose` and the final-close path.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Safety] Stale-stamped stdin chunks, in addition to draining in the gap**
- **Found during:** Task 1 design
- **Issue:** a chunk read during the gap could be handed to the next connection if the redial finished before the gap loop received it. That would send gap keystrokes to the shell (T-05-21).
- **Fix:** the stdin reader stamps each chunk with whether a connection was open when it was read. `serve` drops stale chunks except for the detach key.
- **Commit:** 6e1c735a

**2. [Discretion] Extra loop tests beyond the plan's list**
- `TestRun_NoPongNeverDropped` (an older host that never pongs is never redialed) and `TestRun_NoOffsetMetaMeansFullReplay` (the planner's edge assumption) pin rules the plan states in prose.

**3. [Discretion] `newAttachScene` stores `shareBase`**
- The restart test needs the cap's base URL to fill the second daemon's store. `newViewingDaemon` was extracted as the plan asked.

None of these change the plan's interfaces.

## Deferred Issues

- `TestCLIAttach_LongHistoryJoiners/gui_tab_through_proxy` failed once in a full `-race -short ./...` run. The scene's 15 s context expired while history was typed under heavy host load (two VMs running). It passes alone in about 2.5 s per subtest, and the next daemon-package run and full-suite run both passed. 05-06 does not change that test. Logged in `deferred-items.md`.
- `gofmt -l` lists `internal/relay/scrollback_altscreen_test.go` (from 05-01). Logged, not fixed.

## Known Stubs

None. `AttachSession` and `WsOutputPump` keep their `onFrame` parameter, which every caller now passes as nil. The plan leaves its removal as a mechanical follow-up.

## Notes for later plans

- 05-07: add TESTING.md rows for `internal/attach/loop_test.go`. Note the new cases in `cmd_attach_test.go` (`TestPeerVerdict`, the rewritten `TestRemoteAttachEnd`), `internal/relay/close_reasons_test.go` (`TestIsTerminalClose`) and `internal/daemon/attach_viewers_test.go`: ReconnectsWithDelta, IdleNoReconnecting, ShareOffWhileConnected, ReauthShareStoppedEnds and ViewingDaemonRestart.
- When the host is unreachable, the proxy still accepts the loopback socket and then closes it with 1013 "remote unreachable". The CLI counts that as a short-lived connection, so the backoff keeps growing. The bar flips to "" for a moment before going back to `[reconnecting]`. The bar redraws once a second, so the flip is rarely visible. Worth watching in the manual two-Mac checks.
- After a long sleep the wall-clock watchdog may drop a socket that is still healthy on the first tick after wake. The cost is one delta replay.

## Self-Check: PASSED

- FOUND: internal/attach/loop.go, internal/attach/loop_test.go
- FOUND commits: edfd0f44, 6e1c735a, 047e8957, a198b76d, 5d9bc866
