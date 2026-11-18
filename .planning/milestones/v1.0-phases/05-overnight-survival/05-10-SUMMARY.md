---
phase: 05-overnight-survival
plan: 10
subsystem: cli
status: complete
gap_closure: true
tags: [cli, attach, reconnect, wake, network-change, shar-07, verf-01]
requires:
  - "05-06: attach.Run, Timing/DefaultTiming, stale-stamped stdin, wall-clock watchdog armed by the first pong"
  - "05-08: full replay of a wrapped ring is rendered history + screen"
provides:
  - "internal/wakewatch: type Reason; Woke (\"wake\"), NetworkChanged (\"network\"); Watch(ctx) <-chan Reason (buffered 1, never blocks, no queueing, closed when ctx ends)"
  - "attach.RunOptions.Wake; attach.Timing.ProbeGrace and WakeMinGap (DefaultTiming 5 s each)"
  - "cmd_attach.go: remote attaches pass wakewatch.Watch(ctx)"
  - "attach_viewers_test.go: TestCLIAttach_LongOutputDuringGap, newAttachSceneTimeout, screenOf"
affects: [05-13, 05-15]
tech-stack:
  added: []
  patterns:
    - "Sleep detected as a wall-clock jump (time.Now().Round(0)) between ticks of a 2 s ticker, with 3 s slack; works whether or not the monotonic clock pauses in sleep"
    - "Network change detected as a change in the sorted non-loopback, non-link-local interface address list"
    - "One wake action per WakeMinGap, shared by the open state (probe) and the gap (skip backoff)"
key-files:
  created:
    - internal/wakewatch/wakewatch.go
    - internal/wakewatch/wakewatch_test.go
  modified:
    - internal/attach/loop.go
    - internal/attach/loop_test.go
    - cmd_attach.go
    - internal/daemon/attach_viewers_test.go
    - .planning/phases/05-overnight-survival/05-CONTEXT.md
decisions:
  - "Pure-Go wake detector (wall-clock tick gap + interface addresses) instead of an NSWorkspace observer: the CLI has no AppKit run loop, the GUI can share it, no cgo, testable with an injected clock. Supersedes the CONTEXT 'no NSWorkspace wake hook' decision (user approved)"
  - "A wake-triggered redial does not count as an attempt: the attempt counter is 0 after it, so a quick drop afterwards waits the first backoff step"
  - "A wake probe drops only a connection that has seen a pong and got no frame since the probe; a host that never pongs is never dropped"
  - "The first successful address lookup only learns the key; offline to online (empty key to non-empty) still counts as a change"
metrics:
  duration: 16min
  completed: 2026-11-17
  tasks: 3
  files: 7
estimate:
  tokens: 75000
  tasks: 3
actuals:
  tokens: 7800
  tasks: 3
  commits: 6
plan_head_before: 77ab0d1824cea53813f7a8923c4bf09e4e738989
plan_head_after: 9846f32a10fb706d6b6264fda808f6420eaba17c
---

# Phase 5 Plan 10: Wake and Network-Change Reconnect Summary

`portty attach` to a remote session now reacts within about 2 s when the Mac wakes from sleep or its network changes. Before this, it waited up to 15 s for a ping tick to notice a dead socket, or up to 30 s for a pending backoff. The new `internal/wakewatch` package spots sleep as a jump in the wall clock between 2 s ticks, and spots network or access-point switches as a change in the Mac's routable addresses. `attach.Run` responds in two ways. During a backoff wait it dials at once and restarts the schedule. On an open socket it sends a probe ping, and if a ponging host stays silent for 5 s it drops the socket and redials at once. A new harness test covers the "long output during the gap" row of the resilience matrix: the ring wraps, the CLI gets a full replay, and its screen matches the host's with no line duplicated.

## What was built

- **`internal/wakewatch/wakewatch.go`** (API for 05-13):
  - `Watch(ctx)` starts a 2 s ticker. On each tick it compares the wall-clock gap since the previous tick (`time.Now().Round(0)`) against interval + 3 s, and compares the sorted `net.InterfaceAddrs()` key against the last one. Loopback, fe80::/10 and 169.254/16 are left out of the key. A gap reports `Woke` and a changed key reports `NetworkChanged`. If both happen on one tick, only `Woke` is sent, and the new key is still stored. A failed lookup keeps the old key.
  - The channel is buffered 1. A send never blocks, an event that arrives while one is pending is dropped, and the channel closes when ctx ends (the ticker stops through `context.AfterFunc`).
  - The decision logic is a pure `config.step`, so tests inject the clock, the address source and the ticks.
- **`internal/attach/loop.go`:**
  - `RunOptions.Wake` (nil means no wake events), plus `Timing.ProbeGrace` and `Timing.WakeMinGap` (5 s each in DefaultTiming).
  - Gap: `backoff()` waits on the timer, the Wake channel and ctx. A wake let through by `wakeAllowed()` ends the wait, and the attempt counter becomes 0 after that dial.
  - Open state: on a wake that `wakeAllowed()` lets through, `serve` writes a MsgPing and arms a `ProbeGrace` timer. When the timer fires, a connection that has ponged and got no frame since the probe is dropped through `broken(errSilent)`. `wakeRedial` then makes the next `redial` skip its backoff and restart the counter.
  - Unchanged: the stdin path (stale chunks dropped, detach key works in every state) and the 05-06 wall-clock watchdog.
- **`cmd_attach.go`:** the remote branch passes `Wake: wakewatch.Watch(ctx)`. Local attaches pass nothing.
- **`05-CONTEXT.md`:** the dated amendment (quoted below).
- **`internal/daemon/attach_viewers_test.go`:** `TestCLIAttach_LongOutputDuringGap` plus the small `newAttachSceneTimeout` helper (90 s context for this test).

## Verification

- Task 1: `go vet ./internal/wakewatch ./internal/attach .`, then `go test -race ./internal/wakewatch` (4 tests) and 2 `--- PASS: TestRun_Wake` lines. `./internal/attach` passed in full, and `wakewatch.Watch(` appears once in cmd_attach.go. Tracer gate (end-of-phase, automated verify): re-run end to end and passed. I also ran the real detector by hand. A throwaway binary running `wakewatch.Watch` was SIGSTOPped for 8 s and then SIGCONTed. It printed `wake` 0.4 s after the resume (22:19:58 resume, `22:19:58.424 wake`). The binary was then deleted.
- Task 2: vet passed. 6 `--- PASS: TestRun_Wake` lines (at least 5 required), and `go test -race -count=3 ./internal/attach` passed.
- Mutation checks (each applied alone, then reverted):
  - Removing `l.wakeRedial = true` failed `TestRun_WakeProbesOpenSocket` and `TestRun_WakeStaleInputNeverSent`.
  - Dropping the `ponged.Load()` condition failed `TestRun_WakeNoPongHostKept`.
  - Removing the history prepend in `Hub.joinLocked` failed `TestCLIAttach_LongOutputDuringGap` (LINE-0001..: cli 0 copies, host 2).
- Task 3 / plan gate: `--- PASS: TestCLIAttach_LongOutputDuringGap`, every `TestCLIAttach_` test passed, and `go build ./...` and `go vet ./...` passed. The full suite (`go test -race -short -skip '^(TestAPIAppearance_GetPatch|TestAPITerminalFont_GetPatch)$' ./...`) exited 0 with 12 ok packages. The new test also passed `-count=4` while the relay and webserver suites ran at the same time.

## TDD Gate Compliance

| Task | RED commit | GREEN commit | RED cause (RED_EVIDENCE_OK, semantic check done) |
|------|-----------|--------------|-----------------------------------------------|
| 1 | 6ef013cf | 934db371 | Scaffold compiled and failed on assertions. `TestStep`: `reason = ""; want "wake"`. `TestRun_WakeSkipsBackoff`: `redial 898.988958ms after the wake; want within 200ms`, and the next backoff was 1.89 s (the second step). `TestRun_WakeMinGap` failed the same way |
| 2 | 020d16a5 | 4aa0042a | `TestRun_WakeProbesOpenSocket/{wake,network}`: `no probe ping after the wake`. HealthySocketKept and NoPongHostKept: timed out waiting for the probe ping. StaleInputNeverSent: never went reconnecting |
| 3 | 9846f32a | none needed | Test-only task. The production code it pins (05-01/05-08 full replay, 05-06 ClearOnFull) already behaved correctly, so the test passed when written. The hub mutation above shows that it fails when the replay drops history |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug, test] Pacing in the long-gap test let echo land mid-row**
- **Found during:** Task 3, first full-suite run
- **Issue:** under load, `GAP-0061-` showed up once in the CLI's post-clear output against twice on the host. The pacing waited only for the tag to reach the ring twice. So the next line could be typed while cat's write was still blocked on a full tty output queue. Its echo then landed mid-row, and a row wrap in the rendered history split the tag. The screen comparison still passed, so this was a test artifact, not a replay bug.
- **Fix:** the next line now waits until the ring (SOH headers removed) holds both whole copies of the line, ending in `\r\n`.
- **Files modified:** internal/daemon/attach_viewers_test.go
- **Commit:** 9846f32a

**2. [Discretion] Extra wakewatch coverage**
- `TestAddrKey_IgnoresLoopbackAndLinkLocal` and `TestWatch_RealTickerStops` (the production Watch closes its channel on cancel), plus a "first good lookup only learns" step case (an initial lookup failure must not report a change later).

**3. [Discretion] No wakewatch_test.go change in Task 2**
- The plan listed it under Task 2. The NetworkChanged-equals-Woke case belongs to the loop, so it sits in `TestRun_WakeProbesOpenSocket` as a table case. wakewatch already covers detecting `NetworkChanged`.

None of these change the planned interfaces.

## Concurrent commit inside this plan's range

`b01f6107 docs(todo): faster remote session discovery` (Ken Scott, 22:19) landed on the branch during this run. It is not part of this plan. The measured `commits: 6` includes it: 5 commits are this plan's (6ef013cf, 934db371, 020d16a5, 4aa0042a, 9846f32a), plus this SUMMARY's docs commit.

## User confirmation needed

05-CONTEXT.md had decided "no NSWorkspace wake hook (backoff + liveness suffice)". This plan replaces that with a wake/network-change trigger: `internal/wakewatch`, which watches for a wall-clock tick gap and interface-address changes. It is pure Go and shared by the CLI and the GUI (05-13). The reason is the user's 2026-11-17 requirement to survive network glitches, network and AP switches, and sleep/wake without a manual reattach.

**Confirmation was already given** (2026-11-17, in the orchestrator conversation): "Ok, then let's go with your plan for the wake detector." The user also asked whether it lives in the main executable and liked "everything in one executable". `internal/wakewatch` is pure Go compiled into the single `portty` binary: no helper process and no cgo.

Amendment line added to 05-CONTEXT.md:

> Amended 2026-11-17 (gap closure, user requirement): 'no NSWorkspace wake hook' is superseded. The user requires recovery from sleep/wake and network or access-point switches without waiting out the watchdog. A wake/network-change trigger is added: internal/wakewatch (05-10, CLI) and a Wails event to the GUI and browser views (05-13). Mechanism: a pure-Go wall-clock tick-gap and interface-address detector rather than NSWorkspace, so the CLI gets it too.

**How to revert:** remove the `Wake: wakewatch.Watch(ctx)` line in `cmd_attach.go` and the 05-13 GUI wiring. With a nil `Wake`, `attach.Run` behaves exactly as in 05-06. The 05-04/05-06 watchdog and backoff stay in place.

## Known Stubs

None.

## Threat Flags

None. No new network endpoint, auth path or file access. Wake signals come from the local clock and interface list and only speed up redials that already go through Reauth. T-05-31 is pinned by `TestRun_WakeStaleInputNeverSent`, and T-05-32 by `TestRun_WakeMinGap` plus the buffered-1 channel.

## Notes for later plans

- **05-13 (GUI):** consume `wakewatch.Watch(ctx)` in the Go app and emit a Wails event. The frontend's `onWake` already probes an open socket and skips a pending backoff. `Reason` values are `"wake"` and `"network"`.
- **05-15 (TESTING.md), so the counts do not drift:**
  - New file: `internal/wakewatch/wakewatch_test.go` (TestAddrKey_IgnoresLoopbackAndLinkLocal, TestStep, TestWatch_OneEventPerJumpNoQueueClosesOnCancel, TestWatch_RealTickerStops).
  - New tests in `internal/attach/loop_test.go`: TestRun_WakeSkipsBackoff, TestRun_WakeMinGap, TestRun_WakeProbesOpenSocket, TestRun_WakeHealthySocketKept, TestRun_WakeNoPongHostKept, TestRun_WakeStaleInputNeverSent.
  - New test in `internal/daemon/attach_viewers_test.go`: TestCLIAttach_LongOutputDuringGap.
  - All cover SHAR-07 / VERF-01. TESTING.md was not touched in this plan.
- Real sleep/wake and an AP switch on two Macs are still manual matrix rows (todo `2026-11-17-network-resilience-matrix`, M-84..M-87).
- Expected side effects: IPv6 temporary-address rotation and Tailscale toggling its utun address both count as `NetworkChanged`. That costs one probe on an open socket (a pong keeps it) or one skipped backoff wait, at most once per 5 s.

## Self-Check: PASSED

- FOUND: internal/wakewatch/wakewatch.go, internal/wakewatch/wakewatch_test.go, internal/attach/loop.go, cmd_attach.go, internal/daemon/attach_viewers_test.go
- FOUND commits: 6ef013cf, 934db371, 020d16a5, 4aa0042a, 9846f32a
- FOUND: amendment line in 05-CONTEXT.md
