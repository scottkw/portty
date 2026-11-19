---
phase: 07-hardening
verified: 2026-11-19T04:55:50Z
status: passed
score: 5/5 must-haves verified
covered_files:
  - ".planning/phases/07-hardening/07-01-PLAN.md"
  - ".planning/phases/07-hardening/07-01-SUMMARY.md"
  - ".planning/phases/07-hardening/07-02-PLAN.md"
  - ".planning/phases/07-hardening/07-02-SUMMARY.md"
  - ".planning/phases/07-hardening/07-03-PLAN.md"
  - ".planning/phases/07-hardening/07-03-SUMMARY.md"
  - ".planning/phases/07-hardening/07-04-PLAN.md"
  - ".planning/phases/07-hardening/07-04-SUMMARY.md"
  - ".planning/phases/07-hardening/07-05-PLAN.md"
  - ".planning/phases/07-hardening/07-05-SUMMARY.md"
  - "README.md"
  - "TESTING.md"
  - "app.go"
  - "app_respawn_test.go"
  - "frontend/src/components/TerminalPanel.tsx"
  - "frontend/src/components/__tests__/TerminalPanel.contextLoss.test.tsx"
  - "frontend/src/components/__tests__/TerminalPanel.guestClose.test.tsx"
  - "frontend/src/components/__tests__/TerminalPanel.hot-swap.test.tsx"
  - "frontend/src/components/__tests__/TerminalPanel.reconnect.test.tsx"
  - "frontend/src/components/__tests__/TerminalPanel.scale.test.tsx"
  - "frontend/src/components/__tests__/TerminalPanel.tabSwitch.test.tsx"
  - "frontend/src/components/__tests__/TerminalPanel.test.tsx"
  - "frontend/src/lib/__tests__/fixtures/ris-replay.json"
  - "frontend/src/lib/__tests__/fixtures/wrapped-replay.json"
  - "frontend/src/lib/__tests__/peerSnapshots.test.ts"
  - "frontend/src/lib/__tests__/replayFidelity.test.ts"
  - "frontend/src/lib/peerSnapshots.ts"
  - "frontend/src/lib/relayClient.test.ts"
  - "frontend/src/lib/relayClient.ts"
  - "internal/attach/attach.go"
  - "internal/attach/attach_test.go"
  - "internal/attach/loop.go"
  - "internal/attach/loop_test.go"
  - "internal/daemon/api_test.go"
  - "internal/daemon/engine.go"
  - "internal/daemon/engine_test.go"
  - "internal/daemon/peer_client.go"
  - "internal/daemon/remote_ws_proxy.go"
  - "internal/daemon/remote_ws_proxy_test.go"
  - "internal/daemon/share.go"
  - "internal/daemon/testmain_test.go"
  - "internal/relay/altscreen_heap_test.go"
  - "internal/relay/emu_nohang_test.go"
  - "internal/relay/history_test.go"
  - "internal/relay/hub.go"
  - "internal/relay/input_limit_test.go"
  - "internal/relay/protocol.go"
  - "internal/relay/querystrip_test.go"
  - "internal/relay/replay_fidelity_test.go"
  - "internal/relay/server_test.go"
  - "internal/tailnet/live.go"
  - "internal/tailnet/sessions.go"
  - "internal/tailnet/tailnet.go"
  - "internal/tailnet/tailnet_test.go"
  - "internal/webserver/input_limit_test.go"
  - "internal/webserver/server.go"
  - "internal/webserver/tailscale.go"
  - "internal/webserver/tailscale_test.go"
  - "internal/webserver/whois.go"
covered_digest: "v4:sha256:76014e438a773f1a767c251586c9ecc3248cbb9d5c8a06eb99a9851d29cb65a5"
behavior_unverified: 0
overrides_applied: 0
coincidental_reliance_items:
  - truth: "SC3: Launching the GUI with a healthy daemon logs no respawn attempt and leaves the 3-attempt daemon-error streak at zero"
    reason: undeclared-precondition
    harden: "The 10 s launch grace holds only if the launch-time health failure (root cause unknown, HARD-04) clears within 10 s of a successful startup. Nothing in the code establishes that. When the new 'not answering right after launch (<err>)' line shows up in a GUI stderr capture, use its error text to find and fix the cause, then drop or narrow the grace."
decision_coverage:
  honored: 0
  total: 0
  not_honored: []
---

# Phase 7: Hardening Verification Report

**Phase goal:** The v1.0 reliability and test-hygiene debt is closed. The daemon's memory stays bounded under full-screen apps. The shared shell receives exactly the bytes users type or paste, and nothing a joining terminal answers. Launch and tab switches do no wasted recovery work. The full test suite passes on a developer Mac with nothing skipped.
**Verified:** 2026-11-19T04:55:50Z, branch `gsd/phase-07-hardening`, HEAD `4f17c305`
**Status:** passed
**Re-verification:** No. This is the first verification. No earlier 07-VERIFICATION.md existed.

## Method

I worked from the five ROADMAP success criteria and HARD-01..08 back to the code. I did not take any SUMMARY claim on trust:
- I read the diff from `f54a223a` (the last planning commit before execution) to HEAD for every source file.
- I ran the full gate once.
- I ran targeted tests from test binaries compiled once with `go test -race -c`, so the firewall alert fires once per package and not once per run.
- I checked the real Portty config dir before and after the run.

I could not re-run the Tart VM checks myself (SSH to the office VM failed host-key verification from this shell). The VM rows below rely on the evidence lines in 07-VM-CHECKS.md, which the executor recorded with log lines, sha256 sums and pids.

## Goal Achievement

### Success criteria (roadmap contract)

| # | Success criterion | Status | Evidence |
|---|---|---|---|
| 1 | Full-screen app scrolling 20,000+ alt-screen lines keeps the daemon heap under a fixed bound, proven by a heap soak test; a viewer who joins afterwards still gets the pre-app main-screen history | ✓ VERIFIED | **Code:** `hub.go` `recordFrameLocked` acts on the `AltScreen(false)` callback flag with `rebuildAfterAltExitLocked`. That replaces the live emulator, which frees vt's alt scrollback, and re-seeds the main screen. `h.history` is Hub-owned and survives. While the app stays in the alt screen, vt's 10,000-line `DefaultScrollbackSize` caps the heap; the limit is documented as a `shortcut:` in `newLiveEmulator`. **Test (my run, -race):** `TestHistory_HeapBoundAfterAltScreenScroll`: "burst 1 102.9 MiB, burst 2 0.0 MiB, after exit 0.1 MiB; in-alt ceiling 113.7 MiB", PASS (bound, plateau, release). `TestAltScreenExit_KeepsMainScreenAndHistory`: PASS (LINE 1..200 once, in order, no ALTX line, same cursor and screen). **VM:** H-05 shows identical 128 MiB peaks on two 30,000-line runs at 100 columns and 6 MiB after exit against a 5 MiB baseline. R-02 shows a fresh viewer after `less` gets all 40 main-screen lines with the cursor on the prompt. See the H-05 judgement below. |
| 2 | The shell receives exactly what users send: a paste over 32 KiB (local tab, peer tab, browser view, `portty attach`) arrives complete and in order; one frame over 32 KiB is still dropped; a joining viewer answers none of CSI 14/16/18 t, XTGETTCAP, DECRQSS, DA3, OSC 4 ?, OSC 52 ?; hub and engine strip the same set, one test per sequence | ✓ VERIFIED | **Paste:** the only GUI input path is `TerminalPanel` `term.onData` → `RelayClient.sendInput` → `encodeInputFrames`, which encodes once and slices into frames of at most 32767 payload bytes. The local tab, peer tab and browser view (`WebShareSessionView` mounts `TerminalPanel`) all use it. The CLI caps each stdin read at `relay.MaxInputPayload` in `attach/loop.go:156` and `attach/attach.go:72`. Tests: `TestRun_LargePasteArrivesWhole` and `TestStdinPump_LargePasteArrivesWhole` (100,000 bytes, in order) PASS, plus the vitest `encodeInputFrames (HARD-02)` in the full run. VM: H-01 and R-05 (GUI peer tab, 100,000 bytes, sha256 equal, 4 frames, no 1009) and H-02 (`portty attach`, sha256 equal). **Guard:** no `SetReadLimit` was added or changed on any input reader. `TestServer_InputFrameOverLimitDropped`, `TestRemoteSessionWS_InputFrameOverLimitClosed` and `TestShare_InputFrameOverLimitDropped` PASS: a 32769-byte frame closes the socket and never reaches the PTY. Their AtLimit siblings PASS. **Strip:** one function, `relay.StripTerminalQueries`, is used by the hub live feed (`hub.go:562`), the ring replays (`stripRingQueries`, `hub.go:1192`) and the daemon engine (`engine.go:514`). The engine has no pattern of its own, so parity is structural. `TestStripTerminalQueries` (49 subtests, one per added sequence and variant), `TestJoin_StripsAddedQueries` (full and delta) and `TestJoin_StripsQuerySplitAcrossReads` PASS. |
| 3 | Healthy-daemon launch logs no respawn attempt, streak stays zero; switching back to a remote tab rejoins with a delta from its last offset, with no full redraw and no duplicated or missing output | ✓ VERIFIED (coincidental-reliance, advisory) | **Launch:** `app.go:343` returns before `respawnAttempts++` while `!seenHealthy && launchedAt` is set and less than 10 s old. `launchedAt` is set only on a successful `startDaemon`, so a failed startup gets no grace and Phase 5 R6 is kept. Tests: `TestRespawnDaemon_LaunchGraceSkips` (unhealthy at t0 and t0+5 s: 0 ensures, 0 attempts, no event), `_LaunchGraceEnds`, `_HealthySeenEndsGrace`, `_FailedStartupNoGrace` and `TestStartDaemon_SetsLaunchedAtOnSuccess`, plus all 9 earlier respawn tests, PASS. VM H-04: three GUI relaunches against daemon pid 14757 logged 0 respawn lines and the same pid throughout; the positive control (`kill -9` of the daemon) logged `respawn attempt 1: ok`. **Tab switch:** `TerminalPanel` (peer tabs only, `remote && !wsURL`) reads `resumePoint()` before `close()`, serializes inside a `term.write('', cb)` flush, restores on remount and dials `resume: snap.offset`. Tests: the `TerminalPanel peer tab switch (HARD-05)` suite (6 cases), `peerSnapshots.test.ts` (including snapshot plus delta equal to the live terminal in real xterm.js) and `relayClient.test.ts` `resume option`, all in the full vitest run. VM H-03 and R-04: the returns log `mode=delta ... resume=true` three times each, and `seqcheck` gives `dups=0`, no jump (291 and 54 ticks). |
| 4 | On a dev Mac with non-default settings, `go test -race -short ./...` and Vitest pass with no `-skip` in TESTING.md or any gate; daemon tests never touch the real config dir; TestHub_TwoClientsFanOut passes in repeated full -race runs | ✓ VERIFIED | **Non-default settings present:** the real `settings.json` holds `"appearance":"dark","terminalFontSize":16`. **Full gate:** passed, see the gate output below. **No -skip:** `grep -e -skip` over TESTING.md, `.github/workflows`, `scripts`, `tests` and `frontend/package.json` finds only prose saying the skip is gone, plus an unrelated Playwright `describe.skip` note. `build.yml` and `release.yml` run `go test -race -short ./...` bare. **Isolation:** `testmain_test.go` moves HOME and XDG_CONFIG_HOME to `/tmp/pdh-*` before any test. `testDaemon` uses `t.Setenv("HOME", t.TempDir())`. `TestTestMain_IsolatesConfigDir`, `TestTestDaemon_IgnoresSettingsFile`, `TestAPIAppearance_GetPatch` and `TestAPITerminalFont_GetPatch` PASS. Behavioural check: after the full suite, every file in `~/Library/Application Support/portty/` keeps its earlier mtime (settings.json Nov 17 11:58, daemon.log Nov 18 10:34; the suite ran around 22:47-22:50), and no `daemon.sock` was created. **Fan-out:** the test waits for `SubscriberCount() >= 2` before writing (a test race, not a hub race). It passed in the full run, in 50 runs under -race, and in 8 concurrent copies × 100 runs × `-cpu=1,4` (1,600 runs each of `TestHub_TwoClientsFanOut` and `TestHub_InputFanOut`), with 0 failures. Earlier full runs (07-05 twice, fix iterations 1-3) also passed. |
| 5 | A Known issues entry in the user docs gives the cause of stray prompt lines after narrowing (zsh redrawing a wrapped multi-line prompt per resize) and the remedies (`clear`/Ctrl-L, single-line prompt via `p10k configure`) | ✓ VERIFIED | `README.md:28-33` `## Known Issues`. It states the cause: each resize makes zsh redraw the prompt, and narrowing wraps it so the redraw starts one line low. It gives `clear` or Ctrl-L, and `p10k configure` one-line style or `~/.p10k.zsh`. The brand gate is clean. |

**Score:** 5/5 criteria verified (0 present but behavior-unverified).

### Plan-level must-haves (merged)

All plan truths in 07-01..07-04 hold; the evidence is in the table above.

The 07-05 truths hold except for one detail of the H-05 VM truth. The truth says the in-alt peak stays "under the ceiling derived from vt's 10,000-line cap at the session width". In the plan body that ceiling is 10,000 × C × 112 B + 16 MiB. My judgement is below.

### H-05 judgement (SC1)

The orchestrator's disposition is **sound**. I concur that H-05 is a ceiling calibration miss, not a product defect.

- **Indeterminate, not a clear fail.** gctrace truncates to whole MiB. A peak read as 128 and a baseline read as 5 put the true difference anywhere from 122.01 to 123.99 MiB. The ceiling of 122.81 MiB lies inside that range. The idle in-alt reading (127 - 5) spans 121.01 to 122.99. The FAIL verdict is "as read". The instrument cannot settle a 0.19 MiB margin either way.
- **The criterion's substance holds.**
  - The bound is fixed: both 30,000-line runs peak at the same 128 MiB, set by vt's 10,000-line cap.
  - Growth plateaus: run 2 is not higher.
  - The memory is released: 6 MiB after exit, 1 MiB over baseline, and run 2 does not raise it.
  - SC1 does not name the 16 MiB headroom. That headroom was a plan-time constant chosen before anyone measured at 100 columns. vt's per-line overhead grows with width, which the fixed headroom ignores.
- **The automated proof uses a precise instrument.** `TestHistory_HeapBoundAfterAltScreenScroll` reads `HeapAlloc` after `runtime.GC()`. It passed in my run and in every earlier gate.
- **Honest caveat: the in-alt bound is the same as before the phase.** The todo measured 126.6 MiB with the alt scrollback held until a resize. What Phase 7 adds is the release on exit (memory stuck until a resize becomes memory freed at once) and a test that pins the bound and the plateau. CONTEXT allowed exactly this when vt offers no alt cap, and the residual is documented in `hub.go` `newLiveEmulator`: about 10,000 × cols × ~120 B per session while a TUI sits in the alt screen, about 128 MiB at 100 columns and about 270 MiB at 220. The phase did not remove that residual, but it does not contradict SC1's "fixed bound".

I did not add an override, because an agent cannot record a human acceptance. If you want the deviation recorded formally, add this to the frontmatter:

```yaml
overrides:
  - must_have: "In the Tart VMs: the daemon's live-heap peak stays under the ceiling derived from vt's 10,000-line cap at the session width"
    reason: "H-05 measured 123 MiB as read against 122.81 MiB. That is within gctrace's 1 MiB truncation (true value 122.01-123.99 MiB). Plateau and release passed, and the precise automated soak test passes. The 16 MiB headroom was a plan-time constant."
    accepted_by: "<name>"
    accepted_at: "<ISO timestamp>"
```

### Deferred review items (IN-01/02/03/05/06/08)

None of these blocks a criterion.

| ID | Issue | Effect on a criterion | Verdict |
|---|---|---|---|
| IN-01 | The launch grace guards a blip nobody has reproduced | None. SC3 holds. The grace is bounded at 10 s, a failed startup gets none, and R6 is unchanged (tests pass). Whether to keep the grace is a scope question. See the coincidental-reliance advisory. | Acceptable |
| IN-02 | `peerSnapshots` caps entries (8), not bytes | Frontend memory only. SC1 is about daemon heap. A closed tab's snapshot stays until evicted. | Acceptable; worth a byte budget later given the rock-solid priority |
| IN-03 | A quick remount can beat the deferred `putPeerSnapshot` | In that window of a few milliseconds the rejoin is a full replay, not a delta. The content is still correct, with nothing duplicated or missing. A rare degradation of SC3's "no full redraw", not a correctness failure. | Acceptable |
| IN-05 | A socket drop mid-paste delivers a prefix | Only under a network failure while the frames are on the wire. SC2 describes normal delivery, which is proven. A prefix leaves any cut line without its newline, so nothing executes that a full paste would not have run. | Acceptable; a banner notice is a reasonable follow-up |
| IN-06 | The alt-exit seed writes `?2027` after the text | Cosmetic: emoji or ZWJ rows under mode 2027 may shift by one line in the rebuilt screen. History is unaffected. | Acceptable |
| IN-08 | The rebuild loses SO-shifted line drawing and an open OSC 8 link | Cosmetic, rare. Listed in the `shortcut:` comment. | Acceptable |

### HARD-04 root cause and SC3

SC3 is met as written. With a healthy daemon:
- the healthy branch never reaches `respawnAttempts++`;
- an unhealthy result within 10 s of a successful startup, before the first healthy tick, returns before the increment;
- the VM run shows 0 respawn lines over three launches.

The root cause is still unknown, and the blip did not appear in any Phase 7 VM launch, so the grace path has unit-test evidence only. The grace relies on an undeclared precondition: the blip clears within 10 s. That is recorded as an advisory `coincidental_reliance_items` entry, which changes neither the score nor the status. Every respawn line now carries the health error, and the grace logs `not answering right after launch (<err>)`, so the next time it happens the cause can be read from a stderr capture.

### WR-07 tailnet guard (HARD-06 intent)

Confirmed. `tailnet.LiveAllowed()` is `!testing.Testing() || PORTTY_LIVE_TAILNET_TESTS=1`. It guards every production path to tailscaled or a tailnet peer:
- `DiscoverPeers` and `ProbePeer` (`tailnet.go`)
- `FetchPeerSessionsMeta` (`sessions.go`)
- `CheckHealth` (`webserver/tailscale.go`)
- the WebServer's default WhoIs (`noLiveWhoIs`) and `GetCertificate` (`server.go`)
- `defaultWarmCert` (`daemon/share.go`)
- `API.peerClient` (`daemon/peer_client.go`), which feeds `/peers/connect` and the remote WS proxy

The other `local.Client` uses are behind those guards. No gate, workflow or script sets `PORTTY_LIVE_TAILNET_TESTS`, and it is not set in this shell. `TestLiveTailnetGuard` and `TestLiveTailnetGuard_Webserver` PASS. In a real binary `testing.Testing()` is false, so production behaviour is unchanged.

### 32 KiB limits, share tokens, CSP

Unchanged.
- **32 KiB limits:** no added or removed `SetReadLimit` call in the phase diff. Every input reader still uses coder/websocket's 32768-byte default. Only the CLI stdin buffer changed, from `32*1024` to `relay.MaxInputPayload` (32767), so each read fits in one frame.
- **CSP and share tokens:** `internal/webserver/csp_mw.go` and `internal/capability/` have zero diff lines. The changes to `webserver/server.go` are only the WR-07 test guards (WhoIs and GetCertificate swapped out when `!LiveAllowed()`).

## Required Artifacts

| Artifact | Status | Details |
|---|---|---|
| 07-01 artifacts (hub.go, querystrip_test.go, altscreen_heap_test.go, engine.go, server_test.go) | ✓ VERIFIED | `verify.artifacts` 5/5, `verify.key-links` 3/3 |
| 07-02 artifacts (peerSnapshots.ts, relayClient.ts, TerminalPanel.tsx and tests) | ✓ VERIFIED | 5/5, links 3/3 |
| 07-03 artifacts (app.go, app_respawn_test.go, testmain_test.go, api_test.go, README.md) | ✓ VERIFIED | 5/5, links 3/3 |
| 07-04 artifacts (protocol.go, loop.go, attach.go, relayClient.ts, remote_ws_proxy.go, input_limit tests) | ✓ VERIFIED | 7/7, links 3/3 |
| 07-05 artifacts (TESTING.md, 07-VM-CHECKS.md) | ✓ VERIFIED | 2/2, links 2/2. TESTING.md has HARD-01..07 Section 4 rows and M-94..M-97 in Category AV. |

## Key Link Verification

| From | To | Via | Status |
|---|---|---|---|
| `TerminalPanel` onData | relay / share server | `RelayClient.sendInput` → `encodeInputFrames` (≤ 32767 B payload) | WIRED |
| `attach` loop / StdinPump | relay | read buffer `relay.MaxInputPayload` | WIRED |
| hub live feed, ring replays, engine styled tail | `relay.StripTerminalQueries` | direct calls (`hub.go:562`, `:1192`; `engine.go:514`) | WIRED |
| vt `AltScreen(false)` callback | `rebuildAfterAltExitLocked` | `h.altExited` flag checked after `emu.Write` in `recordFrameLocked` | WIRED |
| `TerminalPanel` unmount / remount | `peerSnapshots` + `RelayClient{resume}` | `resumePoint()` → `putPeerSnapshot`; `takePeerSnapshot` → `resume: snap.offset` | WIRED |
| `startDaemon` success | `respawnDaemonIfDown` grace | `a.launchedAt` | WIRED |
| daemon test binary | temp HOME | `TestMain` → `runInTempHome` | WIRED |

## Behavioral Spot-Checks (targeted, -race, compiled once per package)

| Behavior | Command | Result | Status |
|---|---|---|---|
| Alt-screen heap bound, plateau, release | `relay.test -test.run TestHistory_HeapBoundAfterAltScreenScroll` | burst 1 102.9 MiB, burst 2 0.0, after exit 0.1; ceiling 113.7 | ✓ PASS |
| Main history kept after alt exit | `TestAltScreenExit_KeepsMainScreenAndHistory` | PASS | ✓ PASS |
| Query strip, one subtest per sequence | `TestStripTerminalQueries`, `TestJoin_StripsAddedQueries`, `TestJoin_StripsQuerySplitAcrossReads` | 49 strip/keep subtests PASS | ✓ PASS |
| Input guard at and over 32768 B | relay `TestServer_InputFrame{AtLimitReachesPTY,OverLimitDropped}`, daemon `TestRemoteSessionWS_InputFrame{AtLimitForwarded,OverLimitClosed}`, webserver `TestShare_InputFrame{AtLimitReachesPTY,OverLimitDropped}` | all PASS | ✓ PASS |
| CLI large paste | attach `LargePasteArrivesWhole` (2 tests) | PASS | ✓ PASS |
| Launch grace / R6 | root `TestRespawnDaemon_.*`, `TestStartDaemon_SetsLaunchedAtOnSuccess` | 14 PASS | ✓ PASS |
| Daemon isolation | `TestTestMain_IsolatesConfigDir`, `TestTestDaemon_IgnoresSettingsFile`, `TestAPIAppearance_GetPatch`, `TestAPITerminalFont_GetPatch` | PASS | ✓ PASS |
| Fan-out determinism | `relay.test -test.count=50`, then 8 concurrent × `-test.count=100 -test.cpu=1,4` for `TestHub_TwoClientsFanOut\|TestHub_InputFanOut` | 9 × PASS, 0 FAIL | ✓ PASS |
| WR-07 guard | tailnet `TestLiveTailnetGuard`, webserver `TestLiveTailnetGuard_Webserver` | PASS | ✓ PASS |
| WR-08 replay cursor after RIS | `TestRenderScreen_RISKeepsCursor` | PASS | ✓ PASS |

## Full gate (run once, `GOTOOLCHAIN=go1.26.11`, no reruns needed)

```
== build+vet
BUILD_VET_OK
== gotest   (go test -race -short ./..., no -skip)
ok  	github.com/scottkw/portty	8.157s
ok  	github.com/scottkw/portty/internal/attach	5.714s
ok  	github.com/scottkw/portty/internal/capability	1.026s
ok  	github.com/scottkw/portty/internal/daemon	20.905s
ok  	github.com/scottkw/portty/internal/pty	2.640s
ok  	github.com/scottkw/portty/internal/relay	55.362s
ok  	github.com/scottkw/portty/internal/release	8.586s
ok  	github.com/scottkw/portty/internal/statusbar	5.527s
ok  	github.com/scottkw/portty/internal/tailnet	6.228s
ok  	github.com/scottkw/portty/internal/updater	1.555s
ok  	github.com/scottkw/portty/internal/wakewatch	1.017s
ok  	github.com/scottkw/portty/internal/webserver	13.551s
GOTEST_EXIT=0
== vitest   (cd frontend && pnpm vitest run)
 Test Files  90 passed (90)
      Tests  1236 passed (1236)
== brand
OK: brand gate clean
== trace
OK: 147 traceability paths exist
```

None of the known flakes (SharePopover Copy, initialGrid, TestServer_LoopbackOrigin127Accepted, TestCLIAttach_LongHistoryJoiners, TestRevoke_SubscribeRaceClosed) failed, so nothing was rerun. The vitest exit code was not captured (zsh has no `PIPESTATUS`), but the summary reports 90/90 files and 1236/1236 tests with no failure line.

## Tart VM evidence (07-VM-CHECKS.md, executor-recorded)

| Check | Result | Criterion |
|---|---|---|
| H-01 | PASS | SC2: 100,000-byte Cmd+V into a GUI peer tab, sha256 equal, 4 frames, no 1009 |
| H-02 | PASS | SC2: 100,000 bytes through `portty attach`, sha256 equal |
| H-03 | PASS | SC3: three delta rejoins, 291 ticks, dups=0 |
| H-04 | PASS | SC3: three healthy launches, 0 respawn lines, same daemon pid; the positive control respawns |
| H-05 | FAIL as read (plan ceiling) / plateau and release PASS | SC1: see the H-05 judgement |
| R-01..R-05 | PASS | Post-review regression: CR-01 no wedge, WR-02/05/06/09 main screen and cursor after `less`, WR-08 cursor after `reset`, HARD-05 delta plus SGR mouse after a switch, HARD-02 paste |

## Requirements Coverage

| Requirement | Source plan | Status | Evidence |
|---|---|---|---|
| HARD-01 | 07-01, 07-05 | ✓ SATISFIED | Alt-exit rebuild; `TestHistory_HeapBoundAfterAltScreenScroll` and `TestAltScreenExit_*` PASS; VM plateau and release (H-05), R-02 |
| HARD-02 | 07-04, 07-05 | ✓ SATISFIED | `encodeInputFrames`, CLI read cap, 3 over-limit guard tests, 2 CLI paste tests, vitest; VM H-01, H-02, R-05 |
| HARD-03 | 07-01 | ✓ SATISFIED | A single shared `StripTerminalQueries` in the hub and engine; one subtest per sequence; full and delta join tests |
| HARD-04 | 07-03, 07-05 | ✓ SATISFIED | Bounded launch grace; 5 new respawn tests; VM H-04 (root cause still unknown, advisory) |
| HARD-05 | 07-02, 07-05 | ✓ SATISFIED | Snapshot plus resume offset; tabSwitch, peerSnapshots and relayClient vitest; VM H-03, R-04 |
| HARD-06 | 07-03 | ✓ SATISFIED | TestMain temp HOME, `testDaemon` temp HOME, settings tests pass with non-default host settings, no `-skip` anywhere, real config dir untouched by the full run, WR-07 tailnet guard |
| HARD-07 | 07-01 | ✓ SATISFIED | `waitTwoSubscribers`; 1,600+ stressed -race runs plus the full suite, 0 failures |
| HARD-08 | 07-03 | ✓ SATISFIED | README Known Issues entry with cause and remedies |

No requirements are orphaned: REQUIREMENTS.md maps exactly HARD-01..08 to Phase 7, and the plans claim all eight.

## Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|---|---|---|---|---|
| (all phase-modified source files) | | TBD / FIXME / XXX | none found | |
| (all phase-modified source files) | | TODO / HACK / PLACEHOLDER | none found | |
| `internal/tailnet/tailnet_test.go:450`, `internal/webserver/tailscale_test.go:22` | | `t.Skip` added | ℹ️ Info | Skips only when `PORTTY_LIVE_TAILNET_TESTS=1` is set deliberately; never in a gate |
| `internal/relay/hub.go` | 595, 680, 696, 772, 795, 1110 | `shortcut:` comments | ℹ️ Info | Documented limits (alt-screen cap per session and width, rebuild losses, UTF-8 assumption, vt C0 quirk); none hides a criterion failure |

Decision coverage: the gate was skipped because 07-CONTEXT.md has no trackable decisions (a table, not a `<decisions>` list of IDs). It has no status impact.

## Human Verification Required

None for this phase's criteria. All five criteria have automated tests plus Tart VM evidence over a real (DERP-relayed) tailnet.

These optional real-hardware confirmations are not required by any criterion and do not change the status. They are what the two physical MacBook Airs could add beyond the VMs:
1. **HARD-04 on the real office Mac:** a cold GUI launch at login or after a reboot, where the GUI spawns its own daemon. Check the GUI stderr for a `not answering right after launch (<err>)` line; its text is the missing root-cause evidence. The VMs only covered warm relaunches against a running daemon.
2. **HARD-02 and HARD-05 with the WebGL renderer on and a direct tailnet path:** the VMs ran with WebGL off (office GPU crash loop) and over DERP only. Paste and snapshot restore do not depend on the renderer, so the risk is low.
3. **HARD-01 residual on the 8 GB Mac:** a full-width TUI left in the alt screen in a wide window holds about 10,000 × cols × 120 B until it exits. This is a known, documented limit (WR-04), not a regression.

## Gaps Summary

No gaps. Every success criterion and every HARD requirement is backed by code that exists, is wired, and is exercised by tests that pass in this run, with VM evidence for the GUI and tailnet behaviours. Two judgement calls are recorded rather than turned into gaps:
- **H-05:** a plan-time ceiling missed within instrument resolution, with plateau and release proven. The disposition is sound.
- **HARD-04:** root cause unknown. SC3 holds through a bounded grace, flagged as an advisory coincidental reliance.

---

_Verified: 2026-11-19T04:55:50Z_
_Verifier: Claude (gsd-verifier)_
