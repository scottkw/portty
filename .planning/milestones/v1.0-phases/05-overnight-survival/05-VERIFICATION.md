---
phase: 05-overnight-survival
verified: 2026-11-18T14:36:07Z
status: human_needed
score: 4/5 roadmap success criteria verified (1 needs human verification, deferred by the user; 0 failed)
covered_files: [".planning/debug/long-output-gap-flake.md",".planning/phases/05-overnight-survival/05-01-PLAN.md",".planning/phases/05-overnight-survival/05-01-SUMMARY.md",".planning/phases/05-overnight-survival/05-02-PLAN.md",".planning/phases/05-overnight-survival/05-02-SUMMARY.md",".planning/phases/05-overnight-survival/05-03-PLAN.md",".planning/phases/05-overnight-survival/05-03-SUMMARY.md",".planning/phases/05-overnight-survival/05-04-PLAN.md",".planning/phases/05-overnight-survival/05-04-SUMMARY.md",".planning/phases/05-overnight-survival/05-05-PLAN.md",".planning/phases/05-overnight-survival/05-05-SUMMARY.md",".planning/phases/05-overnight-survival/05-06-PLAN.md",".planning/phases/05-overnight-survival/05-06-SUMMARY.md",".planning/phases/05-overnight-survival/05-07-PLAN.md",".planning/phases/05-overnight-survival/05-07-SUMMARY.md",".planning/phases/05-overnight-survival/05-08-PLAN.md",".planning/phases/05-overnight-survival/05-08-SUMMARY.md",".planning/phases/05-overnight-survival/05-09-PLAN.md",".planning/phases/05-overnight-survival/05-09-SUMMARY.md",".planning/phases/05-overnight-survival/05-10-PLAN.md",".planning/phases/05-overnight-survival/05-10-SUMMARY.md",".planning/phases/05-overnight-survival/05-11-PLAN.md",".planning/phases/05-overnight-survival/05-11-SUMMARY.md",".planning/phases/05-overnight-survival/05-12-PLAN.md",".planning/phases/05-overnight-survival/05-12-SUMMARY.md",".planning/phases/05-overnight-survival/05-13-PLAN.md",".planning/phases/05-overnight-survival/05-13-SUMMARY.md",".planning/phases/05-overnight-survival/05-14-PLAN.md",".planning/phases/05-overnight-survival/05-14-SUMMARY.md",".planning/phases/05-overnight-survival/05-15-PLAN.md",".planning/phases/05-overnight-survival/05-15-SUMMARY.md",".planning/phases/05-overnight-survival/05-16-PLAN.md",".planning/phases/05-overnight-survival/05-16-SUMMARY.md",".planning/phases/05-overnight-survival/05-VM-MATRIX.md","TESTING.md","app.go","cmd_attach.go","frontend/src/App.tsx","frontend/src/components/SessionEndedBanner.tsx","frontend/src/components/TerminalPanel.tsx","frontend/src/components/WebShareSessionView.tsx","frontend/src/components/__tests__/SessionEndedBanner.test.tsx","frontend/src/components/__tests__/TerminalPanel.reconnect.test.tsx","frontend/src/components/__tests__/WebShareSessionView.test.tsx","frontend/src/lib/relayClient.ts","internal/attach/loop.go","internal/daemon/attach_viewers_test.go","internal/daemon/process.go","internal/daemon/remote_ws_proxy.go","internal/relay/close_reasons_test.go","internal/relay/heartbeat.go","internal/relay/history.go","internal/relay/hub.go","internal/relay/protocol.go","internal/relay/scrollback_altscreen_test.go","internal/relay/server.go","internal/wakewatch/wakewatch.go","internal/webserver/server.go"]
covered_digest: "v4:sha256:5d7564984e3523fdee969632c6fd7b46a84ba7d840315ea108dbe89d9c77e50c"
behavior_unverified: 0
overrides_applied: 0
decision_coverage:
  honored: 0
  total: 0
  not_honored: []
re_verification:
  previous_status: human_needed
  previous_score: 3/5
  gaps_closed: []
  gaps_remaining: []
  regressions: []
  human_items_resolved:
    - "M-86/M-87 browser view on the VMs: B1-B4 and B6 PASS (05-VM-MATRIX.md, build dee1b68b). B5 failed as written; resolved by design in 4a9574d1 (the browser keeps retrying and shows a long-outage banner after 2 min)"
    - "VERF-01 gate decision: the user deferred M-84 and M-85 on 2026-11-18 ('Defer, keep going'; STATE.md Deferred Verification). VERF-01 stays Pending"
  warnings_resolved:
    - "TestCLIAttach_LongOutputDuringGap intermittent failure: diagnosed as a loopback connect stall under load in the test harness (.planning/debug/long-output-gap-flake.md). Reconnect wait raised to 20 s in a5df3f53. No content failures in about 330 runs"
advisory:
  - finding: "One 'local input ... write: broken pipe' failure in TestCLIAttach_LongOutputDuringGap during the flake investigation: the host relay closed the local viewer's socket right after RevokeWebViewers"
    category: other
    reason: "Recorded as an open item in .planning/debug/long-output-gap-flake.md. It happened 1 time in about 330 runs, during concurrent experiments under heavy load with a simulated 3 s stall. It did not recur in 40 runs of the same setup. Rerun under load and capture the viewer-diag timeline to resolve it"
    evidence_status: "No failing test or reproducible command. It did not recur in the 40-run repro, my full suite run or 15 targeted runs"
human_verification:
  - test: "M-84 overnight two-Mac test (TESTING.md Section 5, Category AT). DEFERRED BY THE USER on 2026-11-18. Office Mac on a foreground `caffeinate -dims`, Claude Code in a shared Portty session. The home Mac attaches from the GUI sidebar and with `portty attach`. Both sit idle overnight and the home Mac may sleep. Next morning, without reattaching, type into both, check the previous day's scrollback, open the same session from a browser on the tailnet, then do the long-session reattach (step 5) and read the daemon.log viewer-diag lines (step 6)."
    expected: "Typing reaches Claude Code from the GUI tab and the CLI with no manual reattach. The previous day's scrollback is intact in both, with nothing duplicated. The browser view opens and works. Reconnects log mode=delta unless the 256 KiB ring wrapped."
    why_human: "Needs two physical Macs, a real 12+ hour idle gap with lid sleep, the real tailnet path and the native GUI. This is ROADMAP SC4 and the literal 12+ hour part of SC1, SC2 and SC5. The VMs ran 3-minute suspends, not a 12-hour idle. The user deferred it (STATE.md Deferred Verification); the milestone audit carries it."
  - test: "M-85 two-Mac short sleep/wake and network changes. DEFERRED BY THE USER on 2026-11-18. Close the home Mac's lid for 10+ minutes and open it. Turn Wi-Fi off for 20 s and back on. Switch to another Wi-Fi network or access point (or a phone hotspot) and back. Do this with a GUI tab, `portty attach` and a browser all attached."
    expected: "All three viewers reconnect on their own after each step. History is intact with nothing duplicated. Typing into Claude Code works from all three."
    why_human: "A real laptop lid sleep, a real access-point switch and a real Mac's visibilitychange on wake can't be reproduced in Tart VMs. VM rows R3/R4 and B3 covered an interface flap, address changes and tart suspend, not an AP switch or lid sleep. Deferred by the user."
  - test: "VERF-01 sign-off (user). The user deferred the gate on 2026-11-18. When M-84 (or M-85, if the user accepts it as the gate) is run, record the result and mark VERF-01."
    expected: "VERF-01 stays Pending in REQUIREMENTS.md until the chosen two-Mac check passes or the user accepts the accelerated and VM evidence in this report."
    why_human: "The user owns the gate (05-CONTEXT.md: do not rewrite VERF-01 or the ROADMAP criteria). Its current state is deferred, not decided."
  - test: "Harvested from 05-07-PLAN.md Task 2 <human-check>: at Phase 5 verification, Claude runs M-86 and M-87 in the Tart VMs (Safari via dev-browser); the user decides whether M-84 or M-85 gates VERF-01 and runs it, or defers it."
    expected: "At Phase 5 verification: Claude runs M-86 and M-87 in the Tart VM (Safari via dev-browser). The user decides whether M-84 (real overnight) or M-85 (sleep/wake) gates VERF-01 and runs the chosen two-Mac item, or defers it. Pass: every expected result in the item holds and daemon.log shows delta replays on reconnects."
    why_human: "Claude's part is done: the M-86/M-87 browser leg ran in the VMs (Safari via safaridriver; B1-B4 and B6 PASS, B5 resolved by design in 4a9574d1). The user's part is the deferral recorded on 2026-11-18. M-84 and M-85 stay open as the items above."
  - test: "Harvested from 05-09-PLAN.md <human-check> (Claude-run in the VMs): double-clicking a remote session opens one tab; Cmd+W keeps the sidebar and tab bar; reopening works."
    expected: "Claude runs this in the Tart VMs (no human). Pass: after a double-click the home VM shows one tab for the office session; after Cmd+W the sidebar and tab bar remain and reopening works. Evidence: three screenshots described in 05-09-SUMMARY.md."
    why_human: "Already recorded: 05-VM-MATRIX.md V-09 PASS (read from the AX tree because WebKit screenshots capture blank in the VMs). Nothing is left to run. Listed because harvested items are always carried."
  - test: "Harvested from 05-12-PLAN.md <human-check> (Claude-run in the VMs): guest reattach after more than 300 KiB of Claude-like output matches the office rows; typing round-trips; scrollbar at the pane edge."
    expected: "Claude runs this in the Tart VMs (no human). Pass: after more than 300 KiB of Claude-like output, the home VM's guest tab shows the same rows as the office window on first open and on reopen, typing round-trips, and the guest scrollbar is at the pane edge. Evidence: screenshots described in 05-12-SUMMARY.md."
    why_human: "Already recorded: 05-VM-MATRIX.md V-12 PASS (AX-tree rows diffed empty against the office window). Nothing is left to run."
  - test: "Harvested from 05-13-PLAN.md <human-check> (Claude-run in the VMs): after the home VM's Portty is suspended or stopped for 3 minutes, the GUI tab reconnects and accepts typing within about 10 s, with nothing duplicated."
    expected: "Claude runs this in the Tart VMs (no human). Pass: after a 3-minute suspend (or stop) of the home VM's Portty, the GUI tab reconnects and accepts typing within about 10 s of resume, with nothing duplicated. Evidence: method, timings from daemon.log and screenshots in 05-13-SUMMARY.md."
    why_human: "Already recorded: V-13 PASS (1.5 s), plus R4 PASS with a real tart suspend. Nothing is left to run."
  - test: "Harvested from 05-14-PLAN.md Tasks 1-2 <human-check> (Claude-run in the VMs): resilience matrix R1-R8 on the real tailnet."
    expected: "Claude runs this in the Tart VMs (no human). Pass: R2, R4, R5, R6 and R8 recover with typing working and nothing duplicated; R7 ends with the final message; R3 recovers (or is DEFERRED:M-85 only if the VM cannot take its interface down); every row's Result is PASS or DEFERRED:M-NN and none reads FAIL."
    why_human: "Already recorded: R1-R9 PASS (R6 after the 05-16 fix), post-review regression 11/11 PASS, iteration-3 spot check 6/6 PASS. Nothing is left to run."
audit_acknowledged:
  milestone: v1.0
  at: 2026-11-18
  status: human_needed
---

# Phase 5: Overnight Survival Verification Report

**Phase Goal:** A session attached from another Mac or a browser survives an overnight idle gap and picks up where it left off, proving the core value end to end.
**Verified:** 2026-11-18T14:36:07Z
**Status:** human_needed
**Re-verification:** Yes. The previous report (d4a372b0, human_needed, 3/5, no gaps) went stale when files changed after it. This pass re-checks SC3's browser leg and SC5 against the new browser VM evidence, re-runs the whole code gate, and regression-checks everything else. The host Mac rebooted during the first attempt, so every command below was re-run after the reboot. HEAD is 93ca7ebb; that commit touches only REQUIREMENTS.md.

## Verdict in one paragraph

Four of the five criteria now have behavioral evidence. In the previous pass SC5 and the browser leg of SC3 rested on jsdom tests only. The browser view has since been run in a real Safari over the real tailnet in the two Tart VMs:
- **B1:** first open and typing.
- **B2:** silent half-open.
- **B3:** the browser's VM suspended for 3.5 minutes.
- **B4:** about 400 KB of output during a gap, past the 256 KiB ring, restored as a full replay with every line once.
- **B6:** probes timing out for 3 minutes.

All five passed, with no reload, no click and nothing duplicated. B5 (host daemon killed) failed as written because the page retries forever. It was resolved by design in 4a9574d1: the browser still never ends a session on a network error (the user's top priority), and after 2 minutes of failed probes it says it can't reach the host. I confirmed the code and its tests.

The full code gate passed on go1.26.11 on the first run, with no known flakes hit. The one open warning from the previous pass, `TestCLIAttach_LongOutputDuringGap`, was traced to a loopback connect stall in the test harness, not a product bug.

What remains is the literal overnight run on two physical Macs (M-84, SC4) and the lid-sleep and access-point switch run (M-85). The user deferred both on 2026-11-18, so VERF-01 stays Pending. No criterion failed.

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | A session attached from the home Mac's Portty GUI and left idle 12+ hours stays connected, or reconnects on its own after a drop, and accepts typing without a manual reattach | ✓ VERIFIED (accelerated) | Regression check: none of the GUI reconnect files changed since d4a372b0 except TerminalPanel.tsx. That change is only the banner kind/host props (l.117, l.1065); reconnect wiring (l.450 `beforeReconnectRef`) and stdin gating are unchanged. **Code:** `relayClient.ts` reconnects forever with 1-30 s backoff. It redials with `?resume=`, runs a pong-armed watchdog, handles wake, and opens input only at the offset meta. `App.tsx` `reconnectPeerTab` re-authenticates before each redial. `app.go` forwards wakewatch and runs `respawnDaemonIfDown`. **Behavior:** VM rows R1-R9, post-review 11/11 and it3 6/6 PASS on the real app over the real tailnet. vitest 1176/1176 PASS (TerminalPanel.reconnect.test.tsx, relayClient.test.ts). **Not exercised:** a literal 12-hour idle. I found no duration-dependent mechanism: no token TTL (I grepped webserver and daemon for TTL/expiry; the only hits are the self-signed cert's 365 days and a 30 s peer-discovery cache), bounded history, and a capped backoff that never gives up. The 12-hour run is M-84 (deferred). |
| 2 | `portty attach` behaves the same way: after 12+ idle hours it is still attached or has reconnected on its own | ✓ VERIFIED (accelerated) | Product code unchanged since d4a372b0 (`cmd_attach.go` → `attach.Run` with Reauth and Wake; `loop.go` 15 s ping, 40 s dead, backoff forever, resume). The only change is in the harness: `reconnectWait` is 20 s (`attach_viewers_test.go` l.89). The content assertions (screen compare, checkSameLines, diag regexp) are unchanged. I ran all 9 `TestCLIAttach_*` tests x3: 27/27 PASS. That includes `ReconnectsWithDelta`, `IdleNoReconnecting`, `ViewingDaemonRestart`, `ReauthShareStoppedEnds` and `LongOutputDuringGap`. VM CLI column PASS on every row. Same 12-hour caveat as #1. |
| 3 | Detaching and reattaching from the GUI, the CLI or a browser restores the session's scrollback | ✓ VERIFIED | **GUI/CLI:** unchanged since d4a372b0. `Hub.Join` delta or full replay (`hub.go`), 10k-line history (`history.go`), V-12, R8 and N1 PASS. `TestReplayFidelity_*`, `TestHistory_*` and `TestJoin_ExactlyOnce*` are in the green suite. **Browser (new):** 05-VM-MATRIX.md, Browser view section. B4: about 400 KB / 4000 GAP lines written while the page was cut off. It reconnected with `replay mode=full bytes=395052 resume=true`, and the page's `term.buffer.active` (read in full through WebDriver) held GAP 1..4000 and ticks 1..418, each exactly once. B2, B3 and B6 reattached with a delta, 0 repeated and 0 missing. B1's first open got a full replay. Caveat: no browser page reload was run after more than 256 KiB. B4 took the same server path (full Join after the ring wrapped) and the same client reset path, so I count it, but a reload proper is part of M-84 step 4. |
| 4 | Overnight test: Claude Code in a shared shell on the office Mac, home Mac attached from the GUI; after both sit idle overnight the user keeps typing next morning and the previous day's scrollback is intact | ? UNCERTAIN (human, deferred by user) | This is the literal VERF-01 run (TESTING.md M-84). It has not been done. The user deferred M-84 and M-85 on 2026-11-18 (STATE.md Deferred Verification: "Defer, keep going"). The accelerated stand-ins (#1-#3, #5 and the VM matrices) all pass. No later roadmap phase covers it: Phase 6 is Release Updates. |
| 5 | The next morning the same session also opens from a browser on the tailnet and works | ✓ VERIFIED (accelerated) | Safari 27.2 in the home VM opened the office share link over the tailnet. The office log shows `origin=web from=100.x.x.N`, so the page connects straight to the office share server. B1: live ticks and typing. `B1-WEB-OK` landed once in the office file and once in the page. Reconnect without reload or click: B2 half-open (5.8 s after the block lifted), B3 browser VM suspended 3 min 34 s (6.0 s after its first timer tick; the timer-gap wake fallback), B4 ring overflow (3.0 s), B6 probe timeouts (each `AbortError` at 10.0 s, retried, never final). Typing worked after each with nothing duplicated. B5 is resolved by design: `WebShareSessionView.tsx` l.113-130 counts time from the first failed probe and sets `hostUnreachable` after `LONG_OUTAGE_MS = 120_000`, and any HTTP answer resets it. `shareInfoVerdict` still stops on 401/403/404. TerminalPanel l.1065 renders `kind='unreachable'`. I ran the tests by name and they PASS: "probes refused for 2+ min show the long-outage banner, never stop, and a success restores it" and "a 403 after a long outage still stops" (real WebShareSessionView + RelayClient, fake fetch and timers); SessionEndedBanner `kind 'unreachable'` x2; WebShareSessionView `status 401`. **Not exercised:** the literal next-morning open (M-84 step 4), a real Mac's `visibilitychange` on wake (the WebDriver window stayed visible), and a real-browser re-run of B5 with the new banner. |

**Score:** 4/5 truths verified (0 present-but-behavior-unverified, 1 uncertain and routed to a human (deferred by the user), 0 failed).

#### Supporting plan must-haves (05-01..05-16)

Unchanged since the previous pass. Regression check: none of those production files changed, except TerminalPanel.tsx and WebShareSessionView.tsx (the B5 banner only) and the new `unreachable` kind in SessionEndedBanner.tsx. The security constraint still holds: the banner's host text is `hostLabel`, rendered as React text, not HTML. The cap appears only in the fetch URL, which the code never logs, and the VM run's `fw.js` logged no URL or cap. The grant re-check before the replay (`webserver/server.go`) is untouched.

### Advisory (New Scope, Unevidenced)

| # | Finding | Category | Why Advisory |
|---|---------|----------|--------------|
| 1 | One `local input ... write: broken pipe` failure in `TestCLIAttach_LongOutputDuringGap` (debug doc open item): the host relay closed the local viewer right after RevokeWebViewers | other | New scope with no deterministic evidence. It happened once in about 330 runs, under concurrent experiments, and did not recur in the 40-run repro, my full suite run or 15 targeted runs. |

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `frontend/src/lib/relayClient.ts` | Reconnect loop, watchdog, wake, resume, open-after-replay | ✓ VERIFIED | Unchanged. `redial()` l.449 awaits `beforeReconnect`, then stop, retry or dial with `resume`. |
| `frontend/src/components/TerminalPanel.tsx` | Guest reconnect wiring, banner, stdin gating; B5 prop | ✓ VERIFIED | `hostUnreachable` prop l.117, banner kind l.1065 |
| `frontend/src/components/WebShareSessionView.tsx` | Browser /info probe, timeout fallback, long-outage tracking | ✓ VERIFIED | `probeShare` l.116-130 passed as `beforeReconnect` when not on the proxy path; `hostUnreachable` passed to TerminalPanel l.199 |
| `frontend/src/components/SessionEndedBanner.tsx` | `unreachable` kind, no Dismiss | ✓ VERIFIED | l.76-80, Dismiss suppressed l.82 |
| `frontend/src/App.tsx`, `app.go`, `cmd_attach.go`, `internal/attach/loop.go` | Re-auth, wake, respawn, CLI loop | ✓ VERIFIED | Unchanged since d4a372b0 |
| `internal/relay/{hub,history,protocol,heartbeat,server}.go`, `internal/webserver/server.go`, `internal/daemon/{remote_ws_proxy,process}.go`, `internal/wakewatch/wakewatch.go` | Join, resume, history, heartbeats, proxy, wake | ✓ VERIFIED | Unchanged since d4a372b0 |
| `internal/daemon/attach_viewers_test.go` | CLI harness | ✓ VERIFIED | `waitFor` helper and `reconnectWait` 20 s. That is above attach's 10 s dialTimeout plus one redial, so a stuck redial still fails. |
| `TESTING.md` | Section 4 rows for the B5 tests; caveat updated | ✓ VERIFIED | l.546, l.550, l.1009. `check-traceability-paths.sh` OK, 131 paths |
| `05-VM-MATRIX.md` | Browser view evidence | ✓ VERIFIED | Browser view section: B1-B4, B6 PASS, B5 resolved by design. VM state cleanup recorded (no link or cap left behind). |

### Key Link Verification

| From | To | Via | Status |
|------|----|-----|--------|
| WebShareSessionView (browser) | `/api/sessions/{id}/info?cap=` | `probeShare` → `beforeReconnect` (l.198) → TerminalPanel ref (l.150, l.450) → `RelayClient.redial` | WIRED |
| probeShare failure clock | Long-outage banner | `setHostUnreachable` → `hostUnreachable` prop → `SessionEndedBanner kind='unreachable'` | WIRED (test-exercised end to end with the real components) |
| TerminalPanel (guest) | RelayClient reconnect mode | `reconnect: isGuest ? {...}` l.450 | WIRED |
| App peer tab | GetRelayPort + ConnectPeerSession | `reconnectPeerTab` | WIRED (unchanged) |
| Go wakewatch | GUI RelayClient | `system:wake` → `portty:wake` | WIRED (unchanged) |
| `portty attach` remote | attach.Run, re-auth, wakewatch | `cmd_attach.go` | WIRED (unchanged) |
| Viewing proxy / share server | Hub.Join with resume | decimal `resume`, heartbeats | WIRED (unchanged) |

### Data-Flow Trace (Level 4)

| Artifact | Data | Source | Real data | Status |
|----------|------|--------|-----------|--------|
| Browser xterm | Replay and live output | Office share server `Hub.Join` (delta, or full with history) | Yes. VM B1-B6: the page buffer was read in full and the ticks and GAP lines counted | ✓ FLOWING |
| Banner `hostUnreachable` | Long-outage state | Real `fetch` outcomes in probeShare | Yes. Driven by real probe failures; VM B5 showed real `TypeError` refusals | ✓ FLOWING |
| Guest GUI xterm / CLI stdout | Same | Same, through the viewing proxy | Yes (unchanged evidence) | ✓ FLOWING |

### Behavioral Spot-Checks (run by me after the host reboot, GOTOOLCHAIN=go1.26.11)

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Go build and vet | `go build ./... && go vet ./...` | OK (go1.26.11 darwin/arm64) | ✓ PASS |
| Go suite | `go test -race -short -count=1 -skip '^(TestAPIAppearance_GetPatch\|TestAPITerminalFont_GetPatch)$' ./...` | 12/12 packages ok, no FAIL lines, first run, no known flake hit | ✓ PASS |
| Frontend types | `pnpm exec tsc --noEmit` | exit 0 | ✓ PASS |
| Frontend tests | `pnpm exec vitest run` | 87 files, 1176 tests passed (up 5 from 1171: the B5 tests), first run, no SharePopover or initialGrid flake | ✓ PASS |
| Traceability | `bash tests/check-traceability-paths.sh` | OK: 131 paths, exit 0 | ✓ PASS |
| CLI reconnect harness | `go test -race -count=3 -v ./internal/daemon -run 'TestCLIAttach_'` | 27/27 PASS, including LongOutputDuringGap x3 (5.0-6.1 s) | ✓ PASS |
| B5 long-outage behavior | `vitest run --reporter=verbose` on TerminalPanel.reconnect, SessionEndedBanner, WebShareSessionView | The 2 browser-view B5 tests, 2 `unreachable` banner tests and the `status 401` verdict all PASS (39/39 in the 3 files) | ✓ PASS |
| gofmt | `gofmt -l internal/ cmd_*.go app*.go` | empty (5b1fe4a3 fixed the two relay tests) | ✓ PASS |

### Probe Execution

Step 7c: no `scripts/*/tests/probe-*.sh` exists and no plan declares a probe. Not applicable.

### Requirements Coverage

| Requirement | Source Plans | Description | Status | Evidence |
|-------------|--------------|-------------|--------|----------|
| SHAR-07 | 05-01..05-16 | A remote attach left idle for 12+ hours stays connected or reconnects automatically. Detaching and reattaching restores scrollback. | ✓ SATISFIED (accelerated evidence; literal 12 h run in M-84, deferred) | Truths #1-#3 and #5 on all three surfaces: GUI and CLI from the VM matrices, browser from the B rows. Go and vitest suites green. REQUIREMENTS.md marks it Complete (93ca7ebb, not by this verifier). |
| VERF-01 | 05-03, 05-06..05-08, 05-10, 05-12, 05-14..05-16 | Overnight end-to-end test passes: Claude Code shared from the office Mac, the home GUI attaches, idle overnight, typing continues next morning with scrollback intact; the same session opens from a browser. | ? NEEDS HUMAN (deferred by the user 2026-11-18) | The overnight run (M-84) has not happened. The user deferred it and M-85. Every accelerated stand-in passes, including the browser leg now. REQUIREMENTS.md shows Pending, which is correct. Not edited. |

No orphaned requirements: REQUIREMENTS.md maps only SHAR-07 and VERF-01 to Phase 5, and plans claim both.

### Decision Coverage

The CONTEXT.md has no D-IDs, so the gate tool skipped it. A manual spot check (carried from the previous pass, plus B5) found the decisions honored:
- 10k-line memory-only history.
- 1-30 s backoff that retries forever.
- Only "share stopped" / "session ended" closes are final.
- Typing is dropped while disconnected, never queued.
- The 3 s banner delay.
- wakewatch for the wake hook.
- VERF-01 not rewritten.

B5 is consistent with "retry forever / never falsely end". The new banner adds no stop path.

### Test Quality Audit

| Test file | Linked req | Skipped | Circular | Assertion level | Verdict |
|-----------|-----------|---------|----------|-----------------|---------|
| frontend/src/components/__tests__/TerminalPanel.reconnect.test.tsx (B5 cases) | SHAR-07 | 0 | No. Real WebShareSessionView, TerminalPanel and RelayClient; only fetch, WebSocket and timers are faked | Behavioral: banner text over time, no dial while refused, stdin disabled, recovery and clock reset, 403 still final | OK |
| frontend/src/components/__tests__/SessionEndedBanner.test.tsx | SHAR-07 | 0 | No | Value (exact text, no Dismiss) | OK |
| internal/daemon/attach_viewers_test.go | SHAR-07, VERF-01 | 0 | No | Behavioral. The longer wait only applies to the reconnect step; content checks are unchanged | OK |
| (unchanged files from the previous pass) | | 0 | | | OK |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (changed files since d4a372b0) | — | No TBD/FIXME/XXX, TODO/HACK or placeholder markers | — | Debt-marker gate passes |
| TESTING.md | 1038-1050 | M-86/M-87 have no `_Last run:_` line pointing at the browser VM run. The result lives only in 05-VM-MATRIX.md. M-90's line (l.1074) still says "Browser views were not in this run", which was true for that run. | ℹ️ Info | Bookkeeping only. Add a Last-run line to M-86/M-87 referencing the Browser view section and B5's resolution. |
| frontend/src/components/WebShareSessionView.tsx | 113-130 | The long-outage banner was not re-run in a real browser after 4a9574d1 | ℹ️ Info | The state transition has a passing behavioral test using the real components, and the never-stop behavior it builds on was observed in VM B5. Optional: rerun B5 in the VMs to see the copy in Safari. |

Known, intentionally open (05-REVIEW.md iteration 1, outside the fix scope, unchanged): IN-02 to IN-07. The spurious `respawn attempt 1: ok` observation from the it3 spot check is still uninvestigated. None of these fails a criterion.

### Human Verification Required

#### 1. M-84 overnight two-Mac test (SC4; the literal 12 h part of SC1, SC2 and SC5). Deferred by the user, 2026-11-18
**Test:** TESTING.md M-84 steps 1-6.
**Expected:** Typing works with no reattach. The previous day's scrollback is intact with nothing duplicated. The browser works. Reconnects log `mode=delta` unless the ring wrapped.
**Why human:** Two physical Macs, a real 12+ hour gap with lid sleep, the native GUI.

#### 2. M-85 lid sleep and real Wi-Fi access-point switch. Deferred by the user, 2026-11-18
**Test:** 10+ minutes of lid sleep. Wi-Fi off for 20 s and back on. Switch AP or network and back. GUI tab, CLI and browser all attached.
**Expected:** All three recover on their own, history intact with nothing duplicated, typing works.
**Why human:** VMs can't do a lid sleep, an AP switch or a real `visibilitychange` on wake.

#### 3. VERF-01 sign-off (user)
**Test:** When M-84 (or M-85, if accepted as the gate) runs, record the result and mark VERF-01.
**Expected:** VERF-01 stays Pending until then.
**Why human:** The user owns the gate. It is currently deferred.

#### 4-8. Harvested plan human-checks (carried in frontmatter)
- 05-07 Task 2: Claude's M-86/M-87 part is done (B rows). The user's part is the deferral, so it stays open as items 1-3.
- 05-09 (V-09), 05-12 (V-12), 05-13 (V-13) and 05-14 (R1-R9): already recorded PASS. Nothing to run.

### Gaps Summary

There are no blocking gaps. Four of five ROADMAP criteria are verified with behavioral evidence: named tests plus real-app runs in two VMs over the real tailnet. That now includes a real Safari for the browser legs of SC3 and SC5. SC1, SC2 and SC5 are verified on accelerated evidence (minutes-long faults, not 12 hours), with no duration-dependent mechanism found in the code.

`passed` is blocked only by evidence that needs two physical Macs: M-84 (SC4, VERF-01) and M-85. The user deferred both on 2026-11-18, so VERF-01 stays Pending and the milestone audit carries them.

The previous warning (`LongOutputDuringGap`) is closed by the debug diagnosis. One unexplained one-off (broken pipe) is recorded as advisory.

---

_Verified: 2026-11-18T14:36:07Z_
_Verifier: Claude (gsd-verifier)_
