---
phase: 05-overnight-survival
verified: 2026-11-18T12:10:00Z
status: human_needed
score: 3/5 roadmap success criteria verified (2 need human verification; 0 failed)
covered_files: [".planning/phases/05-overnight-survival/05-01-PLAN.md", ".planning/phases/05-overnight-survival/05-01-SUMMARY.md", ".planning/phases/05-overnight-survival/05-02-PLAN.md", ".planning/phases/05-overnight-survival/05-02-SUMMARY.md", ".planning/phases/05-overnight-survival/05-03-PLAN.md", ".planning/phases/05-overnight-survival/05-03-SUMMARY.md", ".planning/phases/05-overnight-survival/05-04-PLAN.md", ".planning/phases/05-overnight-survival/05-04-SUMMARY.md", ".planning/phases/05-overnight-survival/05-05-PLAN.md", ".planning/phases/05-overnight-survival/05-05-SUMMARY.md", ".planning/phases/05-overnight-survival/05-06-PLAN.md", ".planning/phases/05-overnight-survival/05-06-SUMMARY.md", ".planning/phases/05-overnight-survival/05-07-PLAN.md", ".planning/phases/05-overnight-survival/05-07-SUMMARY.md", ".planning/phases/05-overnight-survival/05-08-PLAN.md", ".planning/phases/05-overnight-survival/05-08-SUMMARY.md", ".planning/phases/05-overnight-survival/05-09-PLAN.md", ".planning/phases/05-overnight-survival/05-09-SUMMARY.md", ".planning/phases/05-overnight-survival/05-10-PLAN.md", ".planning/phases/05-overnight-survival/05-10-SUMMARY.md", ".planning/phases/05-overnight-survival/05-11-PLAN.md", ".planning/phases/05-overnight-survival/05-11-SUMMARY.md", ".planning/phases/05-overnight-survival/05-12-PLAN.md", ".planning/phases/05-overnight-survival/05-12-SUMMARY.md", ".planning/phases/05-overnight-survival/05-13-PLAN.md", ".planning/phases/05-overnight-survival/05-13-SUMMARY.md", ".planning/phases/05-overnight-survival/05-14-PLAN.md", ".planning/phases/05-overnight-survival/05-14-SUMMARY.md", ".planning/phases/05-overnight-survival/05-15-PLAN.md", ".planning/phases/05-overnight-survival/05-15-SUMMARY.md", ".planning/phases/05-overnight-survival/05-16-PLAN.md", ".planning/phases/05-overnight-survival/05-16-SUMMARY.md", ".planning/phases/05-overnight-survival/05-VM-MATRIX.md", "TESTING.md", "app.go", "cmd_attach.go", "frontend/src/App.tsx", "frontend/src/components/TerminalPanel.tsx", "frontend/src/components/WebShareSessionView.tsx", "frontend/src/lib/relayClient.ts", "internal/attach/loop.go", "internal/daemon/process.go", "internal/daemon/remote_ws_proxy.go", "internal/relay/heartbeat.go", "internal/relay/history.go", "internal/relay/hub.go", "internal/relay/protocol.go", "internal/relay/server.go", "internal/wakewatch/wakewatch.go", "internal/webserver/server.go"]
covered_digest: "v4:sha256:d18f54e816c26e6719e48059641b1769ff35c3981622931413b59afed4c575d1"
behavior_unverified: 0
overrides_applied: 0
decision_coverage:
  honored: 0
  total: 0
  not_honored: []
human_verification:
  - test: "VERF-01 gate decision (user). Choose what gates VERF-01: M-84 (the real overnight two-Mac run, VERF-01 as written), M-85 (the accelerated two-Mac sleep/wake and Wi-Fi run), the automated and VM evidence in this report, or defer again. 05-CONTEXT.md deferred this decision to verification time."
    expected: "A recorded decision. VERF-01 stays Pending in REQUIREMENTS.md until the chosen gate passes or the user accepts the accelerated evidence."
    why_human: "The user owns the gate. 05-CONTEXT.md says do not rewrite VERF-01 or the ROADMAP criteria and leave the gate to the user."
  - test: "M-84 overnight two-Mac test (TESTING.md Section 5, Category AT). Office Mac on a foreground `caffeinate -dims`, Claude Code in a shared Portty session; the home Mac attaches from the GUI sidebar and with `portty attach`; both sit idle overnight and the home Mac may sleep. Next morning, without reattaching, type into both, check the previous day's scrollback, open the same session from a browser on the tailnet, then do the long-session reattach (step 5) and read the daemon.log viewer-diag lines (step 6)."
    expected: "Typing reaches Claude Code from the GUI tab and the CLI with no manual reattach. The previous day's scrollback is intact in both, with nothing duplicated. The browser view opens and works. Reconnects log mode=delta unless the 256 KiB ring wrapped."
    why_human: "Needs two physical Macs, a real 12+ hour idle gap with lid sleep, the real tailnet path and the native GUI. This is ROADMAP SC4 and the literal 12+ hour part of SC1 and SC2. The VMs ran 3-minute suspends, not a 12-hour idle."
  - test: "M-85 two-Mac short sleep/wake and network changes. Close the home Mac's lid for 10+ minutes and open it. Turn Wi-Fi off for 20 s and back on. Switch to another Wi-Fi network or access point (or a phone hotspot) and back. Do this with a GUI tab, `portty attach` and a browser all attached."
    expected: "All three viewers reconnect on their own after each step. History is intact with nothing duplicated. Typing into Claude Code works from all three."
    why_human: "A real laptop lid sleep and a real access-point switch can't be reproduced in Tart VMs. VM row R3 covered an interface flap and address changes, and R4 a tart suspend, but not a Wi-Fi AP switch or lid sleep."
  - test: "M-86 browser view (Claude-run: per project CLAUDE.md, run it with dev-browser rather than asking the user). In the two Tart VMs, open the shared session's tailnet link in a browser and do the following. (a) kill -STOP the office daemon for 60 s, then kill -CONT it. (b) Do the same to the home side. (c) Reload the page (detach and reattach) after more than 256 KiB of output. (d) Do the M-87 browser leg (tart suspend of the browser's VM, then resume)."
    expected: "The page shows 'Connection lost — reconnecting…' during the freeze and ignores typing. It reconnects on its own when the freeze ends, nothing is duplicated, and typing works. A reload restores the scrollback (up to 10,000 lines) laid out like the host."
    why_human: "No real-browser run of the Phase 5 reconnect path exists. 05-14 and its regressions recorded 'Browser views: not run'. The browser's own code paths (direct wss with ?cap=, the /info probe before redial, the timer-gap wake, and the visibility-gated watchdog) are tested only in jsdom. This covers ROADMAP SC5 and the browser leg of SC3."
  - test: "Harvested from 05-07-PLAN.md Task 2 <human-check>: at Phase 5 verification, Claude runs M-86 and M-87 in the Tart VMs (Safari via dev-browser); the user decides whether M-84 or M-85 gates VERF-01 and runs it, or defers it."
    expected: "At Phase 5 verification: Claude runs M-86 and M-87 in the Tart VM (Safari via dev-browser). The user decides whether M-84 (real overnight) or M-85 (sleep/wake) gates VERF-01 and runs the chosen two-Mac item, or defers it. Pass: every expected result in the item holds and daemon.log shows delta replays on reconnects."
    why_human: "Same as the gate, M-84/M-85 and M-86 items above. M-87's GUI and CLI legs are covered by VM rows R4/R5 (real tart suspend of each role). Its browser leg is not."
  - test: "Harvested from 05-09-PLAN.md <human-check> (Claude-run in the VMs): double-clicking a remote session opens one tab; Cmd+W keeps the sidebar and tab bar; reopening works."
    expected: "Claude runs this in the Tart VMs (no human). Pass: after a double-click the home VM shows one tab for the office session; after Cmd+W the sidebar and tab bar remain and reopening works. Evidence: three screenshots described in 05-09-SUMMARY.md."
    why_human: "Already recorded: 05-VM-MATRIX.md V-09 PASS (read from the AX tree because WebKit screenshots capture blank in the VMs). Nothing is left to run. Listed because harvested items are always carried."
  - test: "Harvested from 05-12-PLAN.md <human-check> (Claude-run in the VMs): guest reattach after more than 300 KiB of Claude-like output matches the office rows; typing round-trips; scrollbar at the pane edge."
    expected: "Claude runs this in the Tart VMs (no human). Pass: after more than 300 KiB of Claude-like output, the home VM's guest tab shows the same rows as the office window on first open and on reopen, typing round-trips, and the guest scrollbar is at the pane edge. Evidence: screenshots described in 05-12-SUMMARY.md."
    why_human: "Already recorded: 05-VM-MATRIX.md V-12 PASS (AX-tree rows diffed empty against the office window). Nothing is left to run."
  - test: "Harvested from 05-13-PLAN.md <human-check> (Claude-run in the VMs): after the home VM's Portty is suspended or stopped for 3 minutes, the GUI tab reconnects and accepts typing within about 10 s, with nothing duplicated."
    expected: "Claude runs this in the Tart VMs (no human). Pass: after a 3-minute suspend (or stop) of the home VM's Portty, the GUI tab reconnects and accepts typing within about 10 s of resume, with nothing duplicated. Evidence: method, timings from daemon.log and screenshots in 05-13-SUMMARY.md."
    why_human: "Already recorded: V-13 PASS (1.5 s), plus R4 PASS with a real tart suspend (9.4 / 11.8 s on 05-14; 49 / 54 s on the post-review rerun, where the tailnet itself took about 40 s to recover). Nothing is left to run."
  - test: "Harvested from 05-14-PLAN.md Tasks 1-2 <human-check> (Claude-run in the VMs): resilience matrix R1-R8 on the real tailnet."
    expected: "Claude runs this in the Tart VMs (no human). Pass: R2, R4, R5, R6 and R8 recover with typing working and nothing duplicated; R7 ends with the final message; R3 recovers (or is DEFERRED:M-85 only if the VM cannot take its interface down); every row's Result is PASS or DEFERRED:M-NN and none reads FAIL."
    why_human: "Already recorded: R1-R9 PASS (R6 after the 05-16 fix). The post-review regression (11/11 PASS) and the iteration-3 spot check (6/6 PASS) also passed. Nothing is left to run."
---

# Phase 5: Overnight Survival Verification Report

**Phase Goal:** A session attached from another Mac or a browser survives an overnight idle gap and picks up where it left off, proving the core value end to end.
**Verified:** 2026-11-18T12:10:00Z
**Status:** human_needed
**Re-verification:** No. This is the initial verification; no earlier VERIFICATION.md exists.

## Verdict in one paragraph

The reconnect and scrollback machinery is real, wired and tested. I checked it in the code, not in the summaries:
- **GUI tab and browser:** RelayClient has a reconnect loop with backoff, a resume offset, a pong-armed watchdog, wake handling and open-after-replay.
- **CLI:** `attach.Run` re-authenticates before each redial and handles wake signals.
- **Servers:** `Hub.Join` delivers a delta or full replay exactly once, with a 10,000-line in-memory history, and the share server and proxy have WebSocket heartbeats.
- **Viewing daemon:** it respawns after a kill.

Every fault class an idle night produces was reproduced on the real app over the real tailnet in two VMs and recovered with no manual reattach: drop, half-open, network change, viewing sleep, host sleep, daemon kills on both sides, and output past the ring during the gap.

Three things are not proven yet, and all three need a person or a decision:
- The literal 12+ hour run on two physical Macs (M-84, ROADMAP SC4).
- A real browser on the Phase 5 reconnect path (M-86, SC5). It has never been run.
- The user's VERF-01 gate decision, which was deferred.

No criterion failed.

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | A session attached from the home Mac's Portty GUI and left idle 12+ hours stays connected, or reconnects on its own after a drop, and accepts typing without a manual reattach | ✓ VERIFIED (accelerated) | **Code:** `relayClient.ts` reconnects forever with 1-30 s backoff. It redials with `?resume=<offset>`, and its watchdog sends a probe after 40 s of silence and drops the socket 5 s later. It handles `portty:wake`. It opens input only at the offset meta (`markOpen`, l.428), so typing is accepted only once the replay is written. `TerminalPanel.tsx` l.401-448 passes `reconnect` for guests and gates stdin on state. `App.tsx` l.856 `reconnectPeerTab` runs GetRelayPort + ConnectPeerSession before each redial, without remounting. `app.go` l.1271 forwards wakewatch to `system:wake`. l.290 `respawnDaemonIfDown` brings back a dead viewing daemon. The proxy heartbeat is at `remote_ws_proxy.go` l.145-150. **Behavior:** VM rows R1-R9 PASS on the real app over the real tailnet (DERP). Post-review R2/R4/R5/R6/R8/N1/N4 PASS. Iteration-3 T1/T2/T4/R6 PASS. In every row the GUI tab came back with no click, typing landed once, and no tick repeated. vitest 1171/1171 includes relayClient.test.ts and TerminalPanel.reconnect.test.tsx. **Not exercised:** a literal 12-hour idle. I found no duration-dependent mechanism: tokens have no TTL, the share server's ReadHeaderTimeout/IdleTimeout do not apply to hijacked WebSockets, history is bounded, and the backoff is capped and never gives up. The 12-hour run itself is M-84 (human). |
| 2 | `portty attach` behaves the same way: after 12+ idle hours it is still attached or has reconnected on its own | ✓ VERIFIED (accelerated) | **Code:** `cmd_attach.go` l.368 calls `attach.Run` with `Reauth: remoteReauth` (GetRelayPort, EnsureDaemon when unreachable, ConnectPeer) and `Wake: wakewatch.Watch(ctx)`. `loop.go` sends a MsgPing every 15 s and treats 40 s of silence as dead. It backs off forever and resumes from the offset. Stale keystrokes are dropped, and `connected` is set only at the offset meta. **Behavior:** I ran `TestCLIAttach_ReconnectsWithDelta`, `_IdleNoReconnecting` (no false `[reconnecting]` when idle), `_ViewingDaemonRestart` and `_ReauthShareStoppedEnds` by name: all PASS. VM rows R1-R9 CLI column PASS, plus post-review R6b and T1b/T2a/T3. Same 12-hour caveat as #1. |
| 3 | Detaching and reattaching from the GUI, the CLI or a browser restores the session's scrollback | ✓ VERIFIED (GUI, CLI; browser leg shares the code, real-browser run in M-86) | **Code:** `hub.go` l.870 `joinLocked` returns a delta from the ring, or a full replay: the history (up to 10,000 lines / 4 MiB, `history.go`) plus the screen when the ring has wrapped or the grid changed. Both servers write grid, meta, chunks, offset meta, then the pending frames (`relay/server.go` l.100-136, `webserver/server.go` l.816-863). The proxy forwards only a decimal `resume` (`remote_ws_proxy.go` l.94). **Behavior:** `TestReplayFidelity_*` (9 tests), `TestHistory_*` (caps, ED3/RIS clear, survives resize, heap bound), `TestJoin_ExactlyOnceUnderConcurrentOutput`: all PASS in my run. V-12 PASS: after a 390 KB Claude-like session, all 29 rows of the home tab were identical to the office window on first open and on reopen. R8 / post-review R8 PASS: LONG 1..4000 each once after a full replay. N1 PASS: about 12,000 lines restored after an 80 s gap. **Browser:** the same RelayClient and the same share-server endpoint the proxy uses. `replayFidelity.test.ts` checks real xterm.js against the Go replay. No real browser has reattached on the Phase 5 build. That leg is in the M-86 item. |
| 4 | Overnight test: Claude Code in a shared shell on the office Mac, home Mac attached from the GUI; after both sit idle overnight the user keeps typing next morning and the previous day's scrollback is intact | ? UNCERTAIN (human) | This is the literal VERF-01 run (TESTING.md M-84). It has not been done. The user deferred the gate decision (05-CONTEXT.md, "Verification gate"). The accelerated stand-ins (#1-#3 and the VM matrix) all pass, but whether they stand in for the overnight run is the user's call. |
| 5 | The next morning the same session also opens from a browser on the tailnet and works | ? UNCERTAIN (human) | No real-browser evidence on the Phase 5 build. 05-14-SUMMARY l.152 and 05-VM-MATRIX l.46 say "Browser views were not run". The browser-only paths are tested only in jsdom: `WebShareSessionView.test.tsx` (probeShare /info: 2xx redials, 401/403 stop as stopped-sharing, 404 stops as ended; the `timeoutSignal` fallback) and relayClient.test.ts (timer-gap wake). Covered by the M-86 and M-84 step 4 items. |

**Score:** 3/5 truths verified (0 present-but-behavior-unverified, 2 uncertain and routed to a human, 0 failed).

#### Supporting plan must-haves (05-01..05-16)

I checked the plan-level truths against the code and the test or VM evidence. They add detail to the five criteria above and don't change the score:
- 05-01 replay contract: Join, 16 KiB `ReplayChunk`, exactly-once, grant re-check before the replay.
- 05-02 heartbeats: `PingEvery` 30 s, `PingTimeout` 15 s, MsgPong.
- 05-03 history: 10k lines / 4 MiB, CRLF rows, ED3/RIS clear.
- 05-04 to 05-06 client loops.
- 05-08 geometry-aware Join.
- 05-09 duplicate-tab guard (V-09 PASS).
- 05-10 / 05-13 wake: wakewatch, `system:wake`, timer-gap wake, debounce.
- 05-11 toolchain pin: the tests ran under go1.26.11.
- 05-12 in-stream reset and scrollbar (V-12 PASS).
- 05-14 matrix.
- 05-16 respawn: `respawnDaemonIfDown`, and `ensureLocalDaemon` in the CLI re-auth.

Security constraint (project CLAUDE.md): `isGrantActive` / `IsSessionEnabled` still run after the subscribe and before any replay byte (`webserver/server.go` l.829). The cap never reaches the GUI tab URL, which is a loopback proxy path.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `frontend/src/lib/relayClient.ts` | Reconnect loop, watchdog, wake, resume, open-after-replay | ✓ VERIFIED | 545 lines, substantive. Used by TerminalPanel for every guest. |
| `frontend/src/components/TerminalPanel.tsx` | Guest reconnect wiring, banner after 3 s, stdin gating | ✓ VERIFIED | `reconnect: isGuest ? {beforeReconnect}`; `SessionEndedBanner kind="reconnecting"` |
| `frontend/src/App.tsx` | Peer re-auth before redial, wake forwarding, additive daemon:recovered | ✓ VERIFIED | `reconnectPeerTab`, `EventsOn('system:wake')`, `tabsRef` guard (it3 WR-02) |
| `frontend/src/components/WebShareSessionView.tsx` | Browser /info probe, timeoutSignal fallback | ✓ VERIFIED (jsdom only) | `beforeReconnect={useProxy ? beforeReconnect : probeShare}` |
| `internal/attach/loop.go` + `cmd_attach.go` | CLI reconnect loop | ✓ VERIFIED | `attach.Run` with Reauth and Wake. Local attach keeps a single connection. |
| `internal/relay/hub.go`, `history.go`, `protocol.go` | Join / resume / history | ✓ VERIFIED | |
| `internal/relay/heartbeat.go`, `server.go`; `internal/webserver/server.go`; `internal/daemon/remote_ws_proxy.go` | Heartbeats, pong, replay framing, resume forwarding | ✓ VERIFIED | |
| `internal/wakewatch/wakewatch.go`, `app.go` | Wake detector; GUI respawn poller | ✓ VERIFIED | |
| `internal/daemon/process.go` | Lock, no client-side unlink, socketLost self-check (it3 WR-03) | ✓ VERIFIED | T2 VM PASS |
| `TESTING.md` | Section 2 counts, Section 4 SHAR-07/VERF-01 rows, M-84..M-90 | ✓ VERIFIED | `tests/check-traceability-paths.sh`: OK, 131 paths |
| `05-VM-MATRIX.md` | Durable VM evidence | ✓ VERIFIED | V-09/12/13, R1-R9, post-review 11/11, it3 6/6, all PASS. No cap values seen. |

### Key Link Verification

| From | To | Via | Status |
|------|----|-----|--------|
| TerminalPanel (guest) | RelayClient reconnect mode | `reconnect: isGuest ? {...}` l.446 | WIRED |
| App peer tab | GetRelayPort + ConnectPeerSession | `beforeReconnect={peerHost ? () => reconnectPeerTab(...)}` l.1127 | WIRED |
| WebShareSessionView (browser) | `/api/sessions/{id}/info?cap=` | `probeShare` passed as beforeReconnect l.179 | WIRED |
| Go wakewatch | GUI RelayClient | `startWakeWatcher` → `system:wake` → `portty:wake` → `onSystemWake` | WIRED |
| GUI poller | EnsureDaemon | `respawnDaemonIfDown` l.1147/1154 | WIRED |
| `portty attach` remote | attach.Run, re-auth, wakewatch | `cmd_attach.go` l.368-378 | WIRED |
| Viewing proxy | peer share server, with resume | decimal-only `resume` forwarded (l.94); upstream heartbeat (l.149) | WIRED |
| Share server / local relay | Hub.Join, TakePending | `server.go` l.101/129, `webserver/server.go` l.817/856 | WIRED |

### Data-Flow Trace (Level 4)

| Artifact | Data | Source | Real data | Status |
|----------|------|--------|-----------|--------|
| Guest xterm (GUI/browser) | Replay and live output | `Hub.Join`: ring delta, or `history.Render()` + `renderScreenLocked()`; then Msgs | Yes (PTY stream). VM rows diffed against the office window | ✓ FLOWING |
| CLI stdout | Same | Same, through the proxy | Yes (R8: LONG 1..4000 once each) | ✓ FLOWING |
| RelayClient `offset` | Resume position | offset meta + (len+1) per output frame | Yes (VM: `mode=delta resume=true` on every recoverable row) | ✓ FLOWING |

### Behavioral Spot-Checks (run by me, GOTOOLCHAIN=go1.26.11)

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Go build and vet | `go build ./... && go vet ./...` | exit 0 | ✓ PASS |
| Go suite | `go test -race -short -count=1 -skip '^(TestAPIAppearance_GetPatch\|TestAPITerminalFont_GetPatch)$' ./...` | exit 0, 12 packages ok, first run, no flakes | ✓ PASS |
| Frontend types | `pnpm exec tsc --noEmit` | exit 0 | ✓ PASS |
| Frontend tests | `pnpm exec vitest run` | 87 files, 1171 tests passed, first run (no SharePopover/initialGrid flake this time) | ✓ PASS |
| CLI reconnect harness | `go test -race -v ./internal/daemon -run 'TestCLIAttach_(ReconnectsWithDelta\|IdleNoReconnecting\|ViewingDaemonRestart\|ReauthShareStoppedEnds)$'` | 4/4 PASS | ✓ PASS |
| Long output during the gap | `TestCLIAttach_LongOutputDuringGap` | 1 FAIL in 36 runs (the targeted run right after the full suite); 35 PASS, including 12 under heavy parallel load and 8 in the same five-test combination. The failure message was not kept (my output filter dropped it). See Anti-Patterns. | ⚠️ PASS with one unexplained failure |
| Replay fidelity, history, exactly-once | `go test -race -v ./internal/relay -run 'ReplayFidelity\|TestHub_Join\|ExactlyOnce\|TestHistory'` | 14/14 PASS | ✓ PASS |
| Traceability | `bash tests/check-traceability-paths.sh` | OK: 131 paths | ✓ PASS |

### Probe Execution

Step 7c: no `scripts/*/tests/probe-*.sh` exists and no plan declares a probe. Not applicable.

### Requirements Coverage

| Requirement | Source Plans | Description | Status | Evidence |
|-------------|--------------|-------------|--------|----------|
| SHAR-07 | 05-01..05-16 | A remote attach left idle for 12+ hours stays connected or reconnects automatically. Detaching and reattaching restores scrollback. | ✓ SATISFIED (accelerated evidence); literal 12 h run in M-84 | Truths #1-#3. Code wired on all three surfaces. Go and vitest suites green. VM rows R1-R9 + post-review 11/11 + it3 6/6 PASS on the real app over the real tailnet. TESTING.md Section 4 has SHAR-07 rows for every new test file. Browser reconnect is jsdom-only (M-86). |
| VERF-01 | 05-03, 05-06..05-08, 05-10, 05-12, 05-14..05-16 | Overnight end-to-end test passes: Claude Code shared from the office Mac, the home GUI attaches, idle overnight, typing continues next morning with scrollback intact; the same session opens from a browser. | ? NEEDS HUMAN | The overnight run (M-84) has not happened and the gate decision is deferred to the user (05-CONTEXT.md). Accelerated stand-ins: `TestCLIAttach_ReconnectsWithDelta`, `_ViewingDaemonRestart`, `_LongOutputDuringGap`, `TestHistory_*`, `TestReplayFidelity_*`, VM rows R4/R5 (real suspend of each role). REQUIREMENTS.md and ROADMAP wording unchanged (no Phase 5 commit touches REQUIREMENTS.md). |

No orphaned requirements: REQUIREMENTS.md maps only SHAR-07 and VERF-01 to Phase 5, and both are claimed by plans.

### Decision Coverage

No trackable decisions in CONTEXT.md. The gate tool skipped because CONTEXT.md has no D-IDs. A manual spot check found every key decision honored:
- **History:** about 10,000 lines, memory only (`HistoryMaxLines = 10_000`; nothing written to disk in history.go/hub.go).
- **Backoff:** 1 s to 30 s with 20 % jitter, retrying forever.
- **Close handling:** `isTerminalClose` stops on "share stopped" / "session ended"; every other close retries.
- **Daemon restarted:** peers/connect re-runs before each redial.
- **Typing while disconnected:** dropped, never queued.
- **Banner:** shows after 3 s (`bannerDelayMs: 3_000`).
- **Wake hook:** the amended decision uses wakewatch, not NSWorkspace.
- **VERF-01:** not rewritten.

### Test Quality Audit

| Test file | Linked req | Skipped | Circular | Assertion level | Verdict |
|-----------|-----------|---------|----------|-----------------|---------|
| internal/daemon/attach_viewers_test.go | SHAR-07, VERF-01 | 0 | No | Behavioral (real CLI loop, real proxy, in-process host; per-tag line counts and screen compared with the host viewer) | OK (one unexplained intermittent failure in LongOutputDuringGap) |
| internal/relay/replay_fidelity_test.go | SHAR-07, VERF-01 | 0 | Partial: the host reference and guest are both the Go vt emulator. It does include a negative control: `DetectsPre0503Staircase` shows it catches the old bug. | Behavioral | OK |
| frontend/src/lib/__tests__/replayFidelity.test.ts | SHAR-07, VERF-01 | 0 | No: real xterm.js checked against the Go-generated fixture, an independent emulator | Value | OK |
| internal/relay/join_test.go, history_test.go, heartbeat_test.go | SHAR-07 | 0 | No | Behavioral | OK |
| internal/attach/loop_test.go, internal/wakewatch/wakewatch_test.go, app_wake_test.go, app_respawn_test.go | SHAR-07 | 0 | No | Behavioral | OK |
| relayClient.test.ts, TerminalPanel.reconnect.test.tsx, WebShareSessionView.test.tsx | SHAR-07 | 0 | No | Behavioral (fake WebSocket, fake timers) | OK |

Disabled tests on requirements: 0. Circular patterns: 0 blocking. Insufficient assertions: 0.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| internal/daemon/attach_viewers_test.go | 867 | `TestCLIAttach_LongOutputDuringGap` failed once in 36 runs, with no load beyond the just-finished suite. The failed run took 5.30 s, close to a full normal run (4.9-7.5 s), so it may have failed at the content assertions (screen compare, checkSameLines) rather than at one of the 2 s `waitUntil` deadlines. It is not on the known-flake list. | ⚠️ Warning | It guards the same behavior that VM rows R8, N1 and T1 passed end to end, so this is not a blocker. Rerun it in a loop with full output (`go test -race -count=50 -v ./internal/daemon -run 'TestCLIAttach_LongOutputDuringGap$'`) and keep the failure text, then add it to the flake list if it times out, or investigate if it is a content mismatch. |
| internal/relay/close_reasons_test.go, internal/relay/scrollback_altscreen_test.go | — | `gofmt -l` lists them | ℹ️ Info | Formatting only, test files. |
| (whole phase) | — | No TBD/FIXME/XXX, TODO/HACK or placeholder markers in the phase's source files | — | Debt-marker gate passes. |

Known, intentionally open (from 05-REVIEW.md iteration 1, outside the fix scope):
- **IN-02:** the poller overrides a deliberate `daemon stop`.
- **IN-03:** there is no heartbeat or read pump during the replay write.
- **IN-04:** a peer tab's offset is dropped on tab switch. The post-review run saw an inactive web-session tab disconnect and rejoin with `mode=full resume=false`. Scrollback still comes back through the full replay.
- **IN-05:** ESC[3J and raw replays.
- **IN-06:** dead `RenderSnapshot` / `EnsureLiveEmulator`.
- **IN-07:** the `startupDone` guard.

None of these fails a success criterion. The iteration-3 residual (keystrokes between the offset meta and the end of the short pending flush queue on the host) was accepted by the review.

Observation, not a failure: both GUI launches with stderr captured on the iteration-3 spot check logged a spurious `respawn attempt 1: ok` against a healthy daemon. That attempt counts toward the 3-attempt streak for the first minute. Uninvestigated.

### Human Verification Required

#### 1. VERF-01 gate decision (user)
**Test:** Decide whether M-84, M-85 or the accelerated evidence here gates VERF-01, or defer again.
**Expected:** A recorded decision. VERF-01 stays Pending until the chosen gate passes or the evidence is accepted.
**Why human:** The user owns this gate (05-CONTEXT.md).

#### 2. M-84 overnight two-Mac test (SC4; the 12 h part of SC1/SC2)
**Test:** TESTING.md M-84 steps 1-6: office `caffeinate -dims`, Claude Code shared, the home GUI tab and `portty attach` left idle overnight with the home Mac allowed to sleep. Next morning: type, check scrollback, open in a browser, do the long-session reattach, read viewer-diag.
**Expected:** Typing works with no reattach. The previous day's scrollback is intact with nothing duplicated. The browser works. Reconnects log `mode=delta` unless the ring wrapped.
**Why human:** Two physical Macs, a real 12+ hour gap with lid sleep, the native GUI.

#### 3. M-85 lid sleep and real Wi-Fi access-point switch
**Test:** 10+ minutes of lid sleep. Wi-Fi off for 20 s and back on. Switch AP or network and back. GUI tab, CLI and browser all attached.
**Expected:** All three recover on their own, history intact with nothing duplicated, typing works.
**Why human:** VMs can't do a lid sleep or an AP switch. R3/R4 covered an interface flap, address changes and tart suspend only.

#### 4. M-86 browser view (Claude-run with dev-browser, per project CLAUDE.md)
**Test:** In the two Tart VMs, attach a browser over the tailnet link. Do the kill -STOP/-CONT freeze on the office side, then the home side. Reload after more than 256 KiB of output. Do the M-87 browser leg (tart suspend and resume).
**Expected:** The banner shows during the freeze and typing is ignored. It recovers on its own with nothing duplicated and typing works. A reload restores the scrollback laid out like the host.
**Why human:** No real browser has run the Phase 5 reconnect path. SC5 and the browser leg of SC3 rest on shared code and jsdom tests only. Note: WebKit can't be screen-captured in these VMs, so read the browser's DOM through dev-browser instead.

#### 5-9. Harvested plan human-checks
Carried in frontmatter:
- 05-07 Task 2: M-86/M-87 plus the gate. Open, and covered by items 1-4.
- 05-09: V-09. Already recorded PASS.
- 05-12: V-12. Already recorded PASS.
- 05-13: V-13. Already recorded PASS.
- 05-14: R1-R8. Already recorded PASS.

### Gaps Summary

There are no blocking gaps. All five ROADMAP criteria have their code present, substantive and wired on every surface. SC1-SC3 have behavioral evidence: passing named tests, plus real-app runs on two VMs over the real tailnet covering every fault class an idle night produces.

What stops a `passed` verdict is evidence the automation can't produce:
- The real overnight two-Mac run (M-84) and the user's deferred VERF-01 gate decision.
- A lid-sleep and access-point switch on real Macs (M-85).
- A real-browser run of the Phase 5 reconnect and reattach path (M-86), which no plan executed.

One warning should be looked at before shipping: `TestCLIAttach_LongOutputDuringGap` failed once in 36 runs, and the failure text was not captured.

---

_Verified: 2026-11-18T12:10:00Z_
_Verifier: Claude (gsd-verifier)_
