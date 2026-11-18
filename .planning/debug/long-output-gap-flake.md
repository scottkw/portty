---
status: awaiting_human_verify
trigger: "TestCLIAttach_LongOutputDuringGap in internal/daemon/attach_viewers_test.go failed once in 36 runs during phase-05 verification; failure text lost; failed run took 5.30 s (normal duration)."
created: 2026-11-18T00:00:00Z
updated: 2026-11-18T05:05:00Z
audit_acknowledged:
  milestone: v1.0
  at: 2026-11-18
  status: awaiting_human_verify
---

## Current Focus

hypothesis: H1 (harness) the reconnect is healthy but slower than 2 s under load (big full replay over TLS + proxy); H2 (product) the reconnect stalls or retries (dial refused, pending-cap kick, redial loop) and only completes late or never
H3 (environment) a loopback TCP connect on the reconnect path (CLI -> viewing proxy, or proxy -> share server) stalls for seconds under heavy load
bug_class: Heisenbug (load-dependent, environment-triggered); SBFL not run (flaky spectrum)

reasoning_checkpoint:
  hypothesis: "The failure is the test's 2 s waitReconnected deadline (viewerWait) expiring because, under heavy parallel test load, a fresh loopback TCP connect on the redial path stalls for seconds (OS level). The product's reconnect itself is correct; its healthy duration is 21-131 ms."
  confirming_evidence:
    - "Reproduced failure text: attach_viewers_test.go:908: timed out after 2s waiting for cli to reconnect (6.30 s run), under load only (0/50 unloaded)"
    - "Same window: 5 other loopback-dial failures across the suite incl. this test's own scene setup taking 30 s to connect to 127.0.0.1; SIGQUIT dump shows a client parked in netFD.connect while the server had accepted"
    - "130 instrumented runs: reconnect 21-131 ms; simulated 3 s stall reproduces the exact failure and the product still delivers one full replay with screen and per-line counts identical to the host"
  falsification_test: "A slow reconnect whose diag timeline shows repeated proxy opens, dial failures, a slow kick, a delta replay, or content-check failures after it completes would refute H3 and point at the product"
  fix_rationale: "The deadline on a step that opens fresh TCP connections is the thing that is wrong for a loaded machine; give waitReconnected (shared by all three reconnect tests) the product's own per-redial bound (attach dialTimeout, 10 s). A stuck reconnect still fails within 10 s; content assertions are untouched."
  blind_spots: "No captured timeline of a real (non-simulated) slow reconnect yet, so which of the two connects stalled is inferred; the original verification failure's text is lost, so it is assumed to be the same deadline (all 3 failures seen in ~280 runs were loopback connect stalls, zero content mismatches). The 30 s 'dial local' setup stall is not fixable by the test and is left alone."
  candidate_causes:
    - "code (test): 2 s deadline on a TCP-connect-bound step"
    - "environment: macOS loopback connect stalls under heavy connection churn (TIME_WAIT or netpoll), seen across unrelated packages"
    - "code (product): redial/backoff/replay slow path - ruled out by 130 instrumented runs and the simulated-stall content checks"
  and_gate: "yes: needs both the environment stall (trigger) and the 2 s test deadline (amplifier); removing either removes this failure"
fix_applied: waitFor(t, what, d, cond) helper; waitReconnected uses reconnectWait = 20 s (one stalled dial = attach dialTimeout 10 s + redial). Uncommitted.
open_side_issue: one "local input ... broken pipe" failure (local viewer socket closed by host relay right after the kick) in the stall experiment; unexplained
next_action: read dbg-longgap/fixdiag.log (fixed test + DBG_STALL=3s + diag dump on failure, 40 runs under load C); if broken pipe recurs, read the dumped viewer-diag timeline for the local conn (leave/slow/session ended)

## Symptoms

expected: test passes every run
actual: 1 failure in 36 runs; duration 5.30 s (not a timeout)
errors: lost
reproduction: go test -race -count=N -run 'TestCLIAttach_LongOutputDuringGap$' ./internal/daemon
started: seen during phase 05 verification

## Eliminated

- hypothesis: content mismatch (lost / duplicated output, wrong resume offset, delta instead of full replay)
  evidence: 0 content-assertion failures in ~330 runs; every slow reconnect (real 10.1 s and simulated 3 s) passed screen compare, checkSameLines and the mode=full diag check
  timestamp: 2026-11-18T04:45Z
- hypothesis: H1 healthy-but-slow full replay (370 KB through TLS + proxy) exceeds 2 s under load
  evidence: replay finishes within ~10-15 ms of "proxy open" even in the slow runs; healthy reconnect 21-131 ms
  timestamp: 2026-11-18T04:45Z
- hypothesis: H2 product redial/backoff/proxy slow path (repeated proxy opens, dial failed, slow kick)
  evidence: slow-run timelines show exactly one proxy open, one join, one replay, no dial-failed or slow lines; the 10 s gap sits before the proxy handler runs, i.e. in the CLI's TCP connect
  timestamp: 2026-11-18T04:45Z
- hypothesis: /tmp/dtestN socket collision caused the broken-pipe one-off
  evidence: the failing process was at dtest5/6 while the concurrent one was past dtest36
  timestamp: 2026-11-18T04:40Z

## Evidence

- timestamp: 2026-11-18T04:10Z
  checked: go test -race -count=50 -v -run 'TestCLIAttach_LongOutputDuringGap$' ./internal/daemon, no extra load (scratchpad/long.log)
  found: 50/50 PASS, 299 s total
  implication: does not reproduce unloaded; the original failure followed the full suite, so load is likely a trigger

- timestamp: 2026-11-18T04:10Z
  checked: code read of the test's assertions and the reconnect path (relay/hub.go Join/joinLocked, webserver handleWSSRelay, daemon remote_ws_proxy, attach/loop.go readFrames)
  found: non-fatal checks are the diag-log regexp (logged by the share server AFTER its TakePending loop), the emulator screen compare, and checkSameLines; fatal ones are the 2 s waitUntil deadlines, the gap-size check and "never cleared"
  implication: a 5.3 s failed run fits either a content mismatch or a late deadline; need the text

- timestamp: 2026-11-18T04:25Z
  checked: prebuilt race binary, -test.count=25 -test.cpu=1,2, while `go test -race -short -count=1 ./...` loops in the background (dbg-longgap/long-load.log)
  found: REPRODUCED on run 15: "attach_viewers_test.go:908: timed out after 2s waiting for cli to reconnect" (6.30 s). Passing runs under the same load took 7.5-8.1 s.
  implication: failure is the 2 s waitReconnected deadline after close(release), not a content assertion. Still open whether the reconnect is merely slow (harness) or stuck / retrying (product). viewer-diag lines go to the captured buffer, so the failure output carries no server-side timeline.

- timestamp: 2026-11-18T04:20Z
  checked: same loaded run, rest of the 50 + the concurrent full-suite iteration 2 (dbg-longgap/load-2.log)
  found: 48 pass / 2 fail. Second failure: "attach_viewers_test.go:871: dial local: ... dial tcp 127.0.0.1:53862: i/o timeout" (30.08 s), i.e. a plain loopback TCP connect in scene setup took 30 s. In the same window the full suite failed TestRun_AbruptCloseRetries (loopback dial i/o timeout, 30 s), TestCLIAttach_ReconnectsWithDelta (dial local, ctx deadline 15 s), TestHeartbeat_ClosesSilentPeer (30 s), TestServer_LoopbackOrigin127Accepted (5 s), and internal/tailnet hung 8+ min. SIGQUIT dump of the hung TestProbePeer_Found: client goroutine parked in net.(*netFD).connect while the httptest server goroutine had already ACCEPTED that connection and sat in readClientHello.
  implication: during heavy parallel load this Mac's loopback TCP connects stall for seconds to indefinitely, with the server side already accepted. No Go code can cause a client connect() to stay pending after the peer accepted; it is the OS network stack (or a network filter extension). Every test that makes fresh loopback connects is exposed.

- timestamp: 2026-11-18T04:20Z
  checked: instrumented binary (reconnect duration measured, 30 s grace, diag dump if > 2 s), 50 runs, load loop was idle by then (stuck in the hung tailnet test)
  found: 50/50 pass; reconnect after release took 21-91 ms every time
  implication: a healthy reconnect with the 360 KB full replay is ~20-40x under the 2 s deadline; a 2 s miss needs a multi-second stall, matching the loopback connect stalls above, not a slow-but-progressing replay

- timestamp: 2026-11-18T04:20Z
  checked: internal/daemon/socket_test.go shortSocketPath
  found: /tmp/dtest<N>_<name> with a per-process counter, so two daemon test processes at once collide ("api.Start: daemon already running at /tmp/dtest13_api.sock" in load-2.log)
  implication: artifact of running my target binary alongside the suite's daemon package; not in the original sequential run. Load loop B excludes internal/daemon.

- timestamp: 2026-11-18T04:28Z
  checked: falsification run, instrumented binary + temporary DBG_STALL=3s sleep in the test's Reauth after release (simulates a 3 s stall before the redial), 3 runs
  found: each run: reconnect 3.04-3.09 s, then ONE share-server join, ONE "proxy open ... dial=10.7ms", ONE "replay ... mode=full bytes=370469 resume=true" 13 ms after the proxy open; the screen compare, checkSameLines and the diag regexp all passed (only the DBG SLOW error fired)
  implication: a multi-second stall on the reconnect path fails the 2 s deadline exactly like the reproduced failure, while the product reconnects with a correct full replay: nothing lost, nothing duplicated, right mode

- timestamp: 2026-11-18T04:30Z
  checked: instrumented binary, 80 runs (load loop B was broken: zsh did not word-split $PKGS, so it only spawned failing go commands)
  found: 80/80 pass, reconnect max 131 ms
  implication: no product-side slow path seen in 130 instrumented runs

- timestamp: 2026-11-18T04:43Z
  checked: instrumented binary, 91 runs under the working load loop C (race suite minus internal/daemon, bash word splitting) (dbg-longgap/long-dbg3.log)
  found: 85 reconnects measured: 83 at 23-124 ms and TWO real slow ones, 10.09 s and 10.08 s. Timeline of both: "DBG release" -> nothing on the viewing proxy for ~10.05 s (no proxy open, no proxy dial failed) -> share-server join, "proxy open dial=23ms/16ms", "replay mode=full bytes=370469 resume=true" within 10 ms. Content checks passed in both (only the DBG SLOW error). Also 4 more scene-setup "dial local ... i/o timeout" 30 s loopback connect stalls.
  implication: H3 CONFIRMED with direct evidence. The CLI's first redial (TCP connect to the loopback proxy) stalled until attach's dialTimeout (10 s); the product then redialed and got a correct full replay. The proxy-side upstream dial was fast. Stalls are "pending until the dialer times out", so a test wait must exceed 10 s plus one redial: 10 s is not enough, 20 s is.

- timestamp: 2026-11-18T04:35Z
  checked: falsification run of the FIX (reconnectWait) with a temporary 3 s stall in Reauth, 3 runs, concurrent with long-dbg3 and load loop C
  found: runs 1-2 PASS (9.1 s, 11.6 s). Run 3 FAIL at 0.33 s: "local input: failed to write msg: ... write tcp ...: write: broken pipe" at the first GAP typeLine, i.e. the host relay closed the LOCAL viewer's socket right after RevokeWebViewers
  implication: a different, unexplained failure (not the reconnect deadline). /tmp/dtestN socket collision ruled out: this process was at dtest5/6 while long-dbg3 was past dtest36. Needs a repro with the diag log dumped on failure.

- timestamp: 2026-11-18T04:55Z
  checked: FIXED test + DBG_STALL=3s + diag dump on failure, 40 runs under load loop C (dbg-longgap/fixdiag.log)
  found: 40/40 PASS; the broken-pipe failure did not recur
  implication: fix holds against a multi-second redial stall under load; the broken pipe stays a one-off (1 in ~330 runs), unexplained

- timestamp: 2026-11-18T04:55Z
  checked: long-dbg3 final (120 runs) and load loop C failures (19 iterations)
  found: long-dbg3: 107 healthy reconnects (<=124 ms), 3 at 10.08-10.09 s (all content intact), 0 stuck; 7 scene-setup "dial local" 30 s connect timeouts. Load loop C: loopback connect failures across attach, relay, webserver and root packages (i/o timeout at the dialer's 30 s / 5 s limit, "connect: connection reset by peer" after 70 s), incl. attach TestRun_NoOffsetMetaMeansFullReplay "timed out after 2s waiting for a third connection" (same 2 s-redial-wait class in another package)
  implication: machine-wide loopback connect stalls under heavy parallel load, independent of Portty's reconnect logic

- timestamp: 2026-11-18T04:56Z
  checked: systemextensionsctl list
  found: three active network flow extensions that see every new TCP flow: Radio Silence (com.radiosilenceapp NetworkExtension), LuLu (objective-see), Proton VPN Split Tunneling transparent proxy; plus Tailscale
  implication: likely mechanism for the stalls (a filter holding new flows for a user-space verdict that lags under thousands of connects); not proven, not needed for the fix

## Resolution

root_cause: "Test-harness deadline + environment (AND-gate): under heavy parallel test load this Mac sometimes leaves a fresh loopback TCP connect pending until the dialer gives up (three active network-flow extensions: Radio Silence, LuLu, Proton split tunnel; mechanism not proven). The CLI's redial to the loopback proxy then waits out attach's 10 s dialTimeout and redials successfully, but waitReconnected allowed only viewerWait (2 s). Not a product bug: every slow reconnect delivered one full replay with screen and per-line counts equal to the host's."
fix: "attach_viewers_test.go: waitFor(t, what, d, cond) helper (waitUntil delegates with viewerWait); waitReconnected uses reconnectWait = 20 s (one stalled dial + redial). Shared by ReconnectsWithDelta, ViewingDaemonRestart, LongOutputDuringGap."
oracle_type: specified (existing screen compare + checkSameLines + diag regexp, unchanged)
verification: |
  - repro before fix: 1/50 under load ("timed out after 2s waiting for cli to reconnect"); 0/50 unloaded
  - real slow reconnects captured: 3 of 110 at 10.08-10.09 s, content intact
  - simulated 3 s stall: old wait fails, fixed test 40/40 PASS under load (plus 2/3 in an earlier concurrent run; 3rd hit the one-off broken pipe)
  - unloaded: all 9 TestCLIAttach_ tests x3 PASS
  - gate: go vet ./... exit 0; go test -race -short -count=1 -skip '^(TestAPIAppearance_GetPatch|TestAPITerminalFont_GetPatch)$' ./internal/... exit 0 (11 ok)
  - commit a5df3f53 (fix), 5b1fe4a3 (style gofmt)
open_items: |
  - one "local input ... write: broken pipe" failure (host relay closed the LOCAL viewer right after RevokeWebViewers), 1 in ~330 runs, during concurrent experiments under load; did not recur in 40 runs of the same config; unexplained
  - scene-setup "dial local ... i/o timeout" (30 s) stalls: 7/120 under heavy load; environment, not addressed
files_changed: [internal/daemon/attach_viewers_test.go]
