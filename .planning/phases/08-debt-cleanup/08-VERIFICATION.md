---
phase: 08-debt-cleanup
verified: 2026-11-19T23:59:00Z
status: human_needed
score: 5/5 must-haves verified
covered_files: [".planning/phases/07-hardening/07-SECURITY.md",".planning/phases/08-debt-cleanup/08-01-PLAN.md",".planning/phases/08-debt-cleanup/08-01-SUMMARY.md",".planning/phases/08-debt-cleanup/08-02-PLAN.md",".planning/phases/08-debt-cleanup/08-02-SUMMARY.md",".planning/phases/08-debt-cleanup/08-03-PLAN.md",".planning/phases/08-debt-cleanup/08-03-SUMMARY.md",".planning/phases/08-debt-cleanup/08-04-PLAN.md",".planning/phases/08-debt-cleanup/08-04-SUMMARY.md",".planning/phases/08-debt-cleanup/08-05-PLAN.md",".planning/phases/08-debt-cleanup/08-05-SUMMARY.md",".planning/phases/08-debt-cleanup/08-VM-CHECKS.md","TESTING.md","frontend/src/App.tsx","frontend/src/components/SessionEndedBanner.tsx","frontend/src/components/TerminalPanel.tsx","frontend/src/components/WebShareSessionView.tsx","frontend/src/components/__tests__/App.open-remote.test.tsx","frontend/src/components/__tests__/TerminalPanel.reconnect.test.tsx","frontend/src/components/__tests__/TerminalPanel.scale.test.tsx","frontend/src/components/__tests__/TerminalPanel.tabSwitch.test.tsx","frontend/src/lib/__tests__/fixtures/charset-link-replay.json","frontend/src/lib/__tests__/fixtures/ris-replay.json","frontend/src/lib/__tests__/fixtures/softwrap-replay.json","frontend/src/lib/__tests__/fixtures/wrapped-replay.json","frontend/src/lib/__tests__/pasteGuard.test.ts","frontend/src/lib/__tests__/peerSnapshots.test.ts","frontend/src/lib/__tests__/replayFidelity.test.ts","frontend/src/lib/pasteGuard.ts","frontend/src/lib/peerSnapshots.ts","frontend/src/lib/relayClient.test.ts","frontend/src/lib/relayClient.ts","internal/daemon/attach_viewers_test.go","internal/daemon/process.go","internal/daemon/process_test.go","internal/daemon/process_unix.go","internal/relay/altscreen_heap_test.go","internal/relay/charstate.go","internal/relay/emu_nohang_test.go","internal/relay/fuzz_test.go","internal/relay/history.go","internal/relay/history_test.go","internal/relay/hub.go","internal/relay/replay_fidelity_test.go","internal/relay/softwrap_test.go","internal/relay/vtalt.go","internal/relay/vtalt_test.go","internal/relay/wraps.go","web/assets/terminal.js"]
covered_digest: "v4:sha256:05f79bc8897e234eb5f40d7e2cc412e65e20bd638cf4f5d3dce5bc586a1d8ceb"
behavior_unverified: 0
overrides_applied: 0
human_verification:
  - test: "V-05 / M-101 disposition (harvested from 08-05-PLAN Task 3 human-check). Claude runs these in the two Tart VMs over SSH, including the tailnet drop inside the office VM; no human is needed."
    expected: "V-04: delta on every return including the quick one, full after close and reopen; V-05: the paste-interrupted notice with N < M and the office file an exact prefix of the payload, stable after the reconnect; the 100 KB sub-run shows a paste notice (never silence) and its office file is a prefix too; V-06: no respawn or launch-blip line"
    why_human: "Native tab clicks, a Cmd+V paste, a real tailnet outage and a real boot cannot run in CI. Recorded result: V-04 PASS, V-06 PASS, V-05 FAIL against this expected text (the notice read 'Paste may have been cut short (M KB)', never 'N of M KB sent'), and the 100 KB sub-run was never run. The orchestrator's disposition (DEBT-05 met; the plan rule was stricter) is an agent decision, not the user's. This verifier agrees on the merits (see SC4), but the user should accept the disposition or ask for the 100 KB sub-run."
  - test: "V-01..V-03 (harvested from 08-05-PLAN Task 2 human-check). Claude runs these in the two Tart VMs over SSH; no human is needed (user rule: drive GUI checks in the VM). The evidence lines in 08-VM-CHECKS.md are what the verifier reads."
    expected: "V-01: in-alt peak within 32 MiB of baseline at 100 and 220 columns, post-exit within 20 MB; V-02: the long command is one line, the exact-width line is separate; V-03: post 1..60 intact, cursor on the prompt"
    why_human: "A real daemon under gctrace, a WebKit xterm.js tab and a real tailnet between two Macs cannot run in CI. Recorded result: V-01 PASS (5 MiB at C=100, 0 MiB at C=220), V-02 PASS, V-03 PASS. Acknowledgement only."
  - test: "L-01..L-05 cold launches (harvested from 08-04-PLAN Task 1 human-check). Claude runs the reboots and launches in the Tart VM over SSH; no human is needed (user rule: drive GUI checks in the VM)."
    expected: "Five rows with the stderr counts, daemon pids and any launch-blip error text; an outcome section."
    why_human: "A first GUI launch after a real macOS boot, with launchd-spawned processes, cannot run in CI. Recorded result: L-01..L-05 PASS; B-01/B-02/B-07/B-08 reproduced the blip at the macOS reopen-at-login; root cause found and fixed; L-06..L-09, V-06 and P-04 PASS on the fixed builds. Acknowledgement only."
---

# Phase 8: Debt Cleanup Verification Report

**Phase Goal:** The tech debt recorded in the v1.1 audit is closed, so v1.1 ships with no known deferred items in its own code.
**Verified:** 2026-11-19 (branch `gsd/phase-07-hardening`, HEAD `b812419b`)
**Status:** human_needed (all 5 success criteria and all 7 requirements verified; three harvested VM human-checks need the user's acknowledgement, one of which carries a recorded FAIL against the plan's stricter wording)
**Re-verification:** No, initial verification

Phase 8 code range: `2e27e73c..b812419b` (2e27e73c added the Phase 8 roadmap entry and is the Phase 7 ship commit that 07-SECURITY.md is pinned to). 37 non-planning files changed.

## Goal Achievement

### Observable Truths (ROADMAP success criteria)

| # | Success criterion | Status | Evidence |
|---|-------------------|--------|----------|
| 1 | A full-screen app scrolling 20,000+ full-width lines in the alt screen keeps per-session heap growth under a small fixed bound that does not grow with width (soak at 80 and 220 columns, VM check). Alt screen and main history stay correct for joining viewers. | ✓ VERIFIED | `internal/relay/vtalt.go` sets vt's alt Screen scrollback to nil once per emulator. `newLiveEmulator` (`hub.go:851`) is the only constructor, so first build, resize rebuild and alt-exit rebuild all get it. If it fails, the daemon logs once and falls back to the HARD-01 exit rebuild. vt source (pinned `b57e5e6d29bb`) checked: `Screen.ClearWithScrollback`, `DeleteLine` and the ED 3 handler nil-check the scrollback. `Emulator.SetScrollbackSize`/`ClearScrollback`/`Scrollback` touch only `scrs[0]`. RIS (`scrs[1].Reset()`) and Resize do not recreate it. **Test run:** `TestHistory_HeapBoundAfterAltScreenScroll` 40,000 lines: growth 0.1 MiB at cols=80 and 0.2 MiB at cols=220, against a 32 MiB bound, in alt and after exit. `TestAltScreenScrollback_ControlWithoutBound` (30.6 MiB for 3,000 lines without the fix) shows the fixture is honest. `TestAltScreen_JoinWhileInAltKeepsMainHistory`, `TestAltScreenExit_KeepsMainScreenAndHistory`, `TestVTLayout_AltScreenScrollback` and `TestDisableAltScreenScrollback_KeepsMainHistory` all PASS. **VM:** V-01 PASS (real daemon under gctrace, 30,000 lines: +5 MiB at C=100, +0 MiB at C=220; before the fix it was +123 MiB). |
| 2 | Replays are faithful: a soft-wrapped long line replays as one wrapped line; rendered replays and the alt-exit rebuild restore modes before text, keep SO/G1 line drawing and an open OSC 8 link; rebuild fuzz stays at 0 mismatches. | ✓ VERIFIED (limit recorded) | **Soft wrap:** `wraps.go` (bounded hash multiset), the `history.go` per-line `wrapped` flag, and `History.Render` joining without a CRLF. 15 `TestSoftWrap_*` tests PASS, including `HistoryReplay`, `ExactWidthHardLineStays`, `BlankContinuationKeepsRows`, `SurvivesAltExitRebuild`, `SurvivesResizePush` and `XtermFixture` (real xterm.js marks the row `isWrapped` via `softwrap-replay.json`). VM V-02 PASS: the 182-character command came back as one line, and the exact-width line stayed separate. **Modes first:** `TestRenderScreen_ModesBeforeText`, `TestFullReplay_ModesBeforeHistory` and `TestRenderScreen_RISKeepsCursor` PASS. VM V-03 PASS (post 1..60 intact, cursor on the prompt). **SO/G1 and OSC 8:** `charstate.go` tracks designations (ESC handlers), the open OSC 8 link (OSC handler) and GL (byte scan, because vt's C0 handlers are unexported). Every replay body ends with the tracked state. The rebuild probes G0 and the link and resyncs. `TestRenderedReplay_KeepsCharsetAndLink` (vt guest), the `charset-link-replay` vitest (real xterm.js: `lqqk` after SO prints `┌──┐`), `TestCharState_Scan`, `FuzzCharState` seeds, `TestOSC8_UnsafeLinkDropped` and `TestAltScreenExit_RebuildMatchesReference` (9 IN-08 subtests) all PASS. **Fuzz:** `TestAltScreenExit_RebuildFuzz`, 600 streams with SO/SI, G0/G1 designations and OSC 8, PASS (0 mismatches; the test fails on any). **Limit, judged acceptable:** see "DEBT-03 judgment" below. |
| 3 | The peer-tab snapshot store is bounded by bytes, frees a closed tab's entry, is keyed by host plus session, never leaks a terminal, and a quick switch back still gets a delta rejoin. | ✓ VERIFIED | `peerSnapshots.ts`: `PEER_SNAPSHOT_MAX_BYTES = 8 MiB` plus the 8-entry cap, with oldest-first eviction and an over-budget snapshot never stored. `peerSnapshotKey(host, sessionId)` uses a NUL separator. `dropPeerSnapshot` bumps a per-key generation, so a late flush cannot store again. `App.tsx:531-537` `removeTabState` (close button, Cmd+W, End Session, session:exit) calls `dropPeerSnapshot(peerSnapshotHost(tab), …)`, and `App.tsx:1157` passes the same helper's host to the panel. `TerminalPanel.tsx:613` disposes after `PEER_FLUSH_TIMEOUT_MS` (1 s) if xterm never runs the flush. `:515` awaits a pending save for up to 250 ms before dialling. Tests PASS (205 in the targeted vitest run): peerSnapshots byte budget and key/generation suites; tabSwitch "a flush callback that never fires: the terminal is disposed…", "a remount while the save is pending waits for it… dials resume=104", "a drop before the unmount: the late flush stores nothing"; App.open-remote "closing a MagicDNS peer's tab drops its entry and keeps another host's entry with the same id", the hostname-only case and Cmd+W. **VM:** V-04 PASS (deltas on every return, quick switch `mode=delta bytes=0 resume=true` 1.05 s later, full replay after close and reopen). |
| 4 | A paste cut short by a dropped socket tells the user so. No frame is sent twice, and none is silently lost. | ✓ VERIFIED (V-05 disposition accepted on the merits; user ack requested) | `relayClient.ts` `sendInput` records each split paste and sends an ack `MSG_PING` after it. A pong at or past `ackPing` clears the record. `checkPaste` runs on every socket death that `close()` did not cause and calls `onPasteInterrupted` with either whole frames sent (bytes still in `bufferedAmount`) or `null` (unconfirmed). Nothing is ever queued or resent. `sendInput` drops input unless the state is `open`. `SessionEndedBanner` shows "Paste interrupted: N of M KB sent" or "Paste may have been cut short (M KB)". relayClient "partial paste on a socket drop (DEBT-05)" PASS, including "a drop mid-paste: one notice, and after the redial the new socket carries no input frame" and "a user close() mid-paste: no notice". TerminalPanel.reconnect PASS for both banner copies. **VM:** V-05, 5 MB and 20 MB pastes cut by `tailscale down`: the notice appeared both times with M correct, the office files were exact prefixes (98,301 and 262,136 bytes) that never grew after the reconnect, and the tab took typing again. **Judgment:** SC4 and DEBT-05 ask that the user is told, that nothing is sent twice, and that nothing is silently lost. All three are shown. The plan's extra "N of M" wording cannot appear on the GUI paths, because they hand the paste to a loopback socket, so `bufferedAmount` is 0 when the tailnet side dies. "May have been cut short" is the honest copy there. The 100 KB sub-run was not run; it takes the same `unsent === 0` branch. |
| 5 | The HARD-04 launch blip has a root-cause fix (or a documented VM non-reproduction with diagnostics). The Phase 7 SUMMARYs carry `requirements_completed`, and Phase 7 has a SECURITY.md. | ✓ VERIFIED | **Root cause found and fixed:** 08-VM-CHECKS B-01/B-02/B-07/B-08 reproduced the blip at the macOS reopen-at-login. The spawned daemon took up to 10 s to bind against a 3 s `EnsureDaemon` wait. Startup failed, which skips the launch grace, and the poller respawned a second daemon at once (`daemon: already running`). The fix is `daemonReadyWait = 15 * time.Second` (`internal/daemon/process.go`). The grace and both app.go log lines are unchanged, and `app.go` has no diff since 2e27e73c. `TestWaitForDaemon_SlowStart` PASS (failed before the fix). Review-fix hardening: `TestWaitForDaemon_DiedAtSpawnFailsFast`, `_LostLockWaitsForHolder`, `_LockReleasedWithoutBindFailsFast`, `TestStartDaemon_WaitsForOldDaemonLock`, `_LateLockReleaseKeepsSpawnWindow` and `_MismatchedDaemonAnsweringIsNotReady` (CR-01) all PASS. **VM:** L-06..L-09 PASS (L-09 hit the slow path, a 4 s bind, with no respawn), V-06 PASS, P-03 PASS (fail-fast in 2.2 s), P-04 PASS, and U-01/U-02 PASS (the CR-01 version-mismatch handover leaves no daemon gap). **Bookkeeping:** `gsd-tools summary-extract --fields requirements_completed` returns HARD IDs for all five 07 SUMMARYs (07-01 HARD-01/03/07, 07-02 HARD-05, 07-03 HARD-04/06/08, 07-04 HARD-02, 07-05 all eight). `07-SECURITY.md` exists with `threats_open: 0` and a 17-threat register (T-07-01..T-07-17), each checked against code pinned at 2e27e73c. |

**Score:** 5/5 truths verified (0 present-but-behavior-unverified)

### DEBT-03 judgment: SO/G1 rows drawn before a viewer joins

vt ignores SO/SI (`TestVT_IgnoresShiftOut` pins this: `ESC)0 SO q` prints `q`). So rows that were drawn through G1 before a viewer joins are stored in vt's buffer as ASCII, and they replay as ASCII in a rendered replay. A raw ring replay and the live stream carry the real bytes and are unaffected.

**Verdict: SC2 is met, and the past-row limit is acceptable.** The reasons:
- The requirement says "keep a G1 line-drawing charset shifted in with SO". The original issue (07-REVIEW-FIX IN-08: "with GL = G1 the rebuild designates G0 instead") is about charset state carried across the replay or rebuild, and that is what was fixed and tested in vt and in real xterm.js.
- Past-row content is a property of vt's buffer. Restoring it would need a vt fork or a shadow cell model, which is out of proportion.
- Practical exposure is small. Portty spawns shells with `TERM=xterm-256color`, whose terminfo has `smacs=\E(0` (G0 designation, not SO). vt renders G0 line drawing correctly, so ncurses boxes in past rows replay correctly. Only programs that emit SO directly are affected, for example under a forced `TERM=vt100` or `screen` over ssh. tmux and screen running inside Portty translate ACS to the outer terminal's G0.
- **Gap in documentation (follow-up 4):** the limit is not written down. `charstate.go`'s `shortcut:` lists G2/G3/LS2/LS3/GR and the DECRC link drift, but not "rows already drawn through SO/G1 replay as ASCII in a rendered replay".

### DEBT-01 judgment: reflect/unsafe with a version pin

This is acceptable. It reads one unexported field (`Emulator.scrs`) of a pinned module and checks it in three layers:
1. The runtime type check `[2]vt.Screen`, plus an identity check that element 0 holds the scrollback the exported `Scrollback()` returns, so swapped roles can never disable main history (IN-04).
2. A graceful fallback: a one-time log, after which the HARD-01 exit rebuild frees the alt scrollback.
3. `TestVTLayout_AltScreenScrollback` fails on any change of the vt version in go.mod, with a message listing what to re-review.

The write itself goes through vt's exported `Screen.SetScrollback(nil)`, which vt documents as "Pass nil to disable scrollback". `go vet` is clean. The residual risk is a vt bump done without running tests, and the pin catches that in CI.

### Requirements Coverage

| Requirement | Source plan | Description (short) | Status | Evidence |
|-------------|-------------|---------------------|--------|----------|
| DEBT-01 | 08-01, 08-05 | Alt-screen heap bound per session, independent of width | ✓ SATISFIED | SC1 row: `vtalt.go`, the heap soak at 80/220 (0.1/0.2 MiB), V-01 |
| DEBT-02 | 08-01, 08-05 | A soft-wrapped line comes back as one wrapped line, in history replay and after a rebuild | ✓ SATISFIED | SC2 row: `wraps.go`/`history.go`, 15 `TestSoftWrap_*` incl. the alt-exit and resize rebuilds, the xterm.js fixture, V-02. Accepted shortcuts (wraps.go, IN-13): identical rows can trade a wrap mark, and a redraw can defeat the row-0 erase clear. Both affect only reflow and copy; layout at the host grid is unchanged (`TestSoftWrap_IdenticalRowsTradeWrap`, `_RedrawKeepsHostLayout`, `TestHistory_RedrawDefeatsRow0EraseInOneRead` pin this) |
| DEBT-03 | 08-02, 08-05 | Modes before text (IN-06); keep G1 via SO and an open OSC 8 link (IN-08) | ✓ SATISFIED | SC2 row and the DEBT-03 judgment above; fuzz at 0 of 600; V-03 |
| DEBT-04 | 08-03, 08-05 | Snapshot store bounded by bytes, dropped on close, host+session key, no xterm leak, quick-switch delta | ✓ SATISFIED | SC3 row; V-04. Residual IN-07 (retryInit does not drop; tab ids keyed by session id alone) is deferred: bounded memory, not correctness. It has **no todo** (follow-up 3) |
| DEBT-05 | 08-03, 08-05 | User told of a cut paste; never silently lost, never sent twice | ✓ SATISFIED | SC4 row; V-05 (disposition), P-01/P-02 (the WR-01 paste-textarea hang fix) |
| DEBT-06 | 08-04, 08-05 | Root-cause fix of the launch blip, or a documented non-reproduction | ✓ SATISFIED | SC5 row: reproduced (B-rows), root cause, fix `c27b69d3`, seam test, L-06..L-09/V-06/P-04 |
| DEBT-07 | 08-04, 08-05 | 07 SUMMARY `requirements_completed` plus 07-SECURITY.md | ✓ SATISFIED | SC5 row: summary-extract output, `threats_open: 0` |

No orphaned requirements: REQUIREMENTS.md maps DEBT-01..07 to Phase 8, and every one is claimed by a plan.

### Required Artifacts

| Artifact | Status | Details |
|----------|--------|---------|
| `internal/relay/vtalt.go` | ✓ VERIFIED | `altScreenOf`, `disableAltScreenScrollback`; wired at `hub.go:851` |
| `internal/relay/vtalt_test.go` | ✓ VERIFIED | `pinnedVT` equals the go.mod version; layout and RIS/Resize checks PASS |
| `internal/relay/wraps.go`, `history.go` | ✓ VERIFIED | `wrapSetMax`, `wrapped bool`, `EndsWrapped`; used by the drain, resize and render paths |
| `internal/relay/charstate.go` | ✓ VERIFIED | Tracker, `fixRenderedLinks`, `safeLinkField` (2083-byte cap, C0/C1/invalid UTF-8 rejected) |
| `internal/relay/softwrap_test.go`, `altscreen_heap_test.go`, `replay_fidelity_test.go`, `history_test.go`, `fuzz_test.go` | ✓ VERIFIED | All named tests PASS in the targeted run |
| `frontend/src/lib/peerSnapshots.ts` | ✓ VERIFIED | Byte budget, key, generation, pending save, flush timeout; wired in App.tsx and TerminalPanel.tsx |
| `frontend/src/lib/relayClient.ts`, `SessionEndedBanner.tsx` | ✓ VERIFIED | `onPasteInterrupted` reaches `setPasteNotice` (`TerminalPanel.tsx:478`), which renders the banner (`:1149-1153`) |
| `frontend/src/lib/pasteGuard.ts` (+ `web/assets/terminal.js` one-liner) | ✓ VERIFIED | WR-01; P-01: a 20 MB paste, WebContent idle, AX read 0.05 s |
| `internal/daemon/process.go` | ✓ VERIFIED | `daemonReadyWait` 15 s, `startDaemon` lock wait, `staleVersion` in `daemonReady` (CR-01) |
| `.planning/phases/08-debt-cleanup/08-VM-CHECKS.md` | ✓ VERIFIED | L/B, V-01..V-06, P-01..P-04, U-01..U-02 rows; no capability tokens |
| `.planning/phases/07-hardening/07-SECURITY.md` | ✓ VERIFIED | `threats_open: 0` |
| `TESTING.md` | ✓ VERIFIED | Section 2 counts match `git ls-files` (Go 137, vitest 91); Section 4 DEBT-01..06 rows; Section 5 M-96..M-103 |

### Key Link Verification

| From | To | Via | Status |
|------|----|-----|--------|
| `hub.go` `newLiveEmulator` | `vtalt.go` | `disableAltScreenScrollback(emu)` on every build | ✓ WIRED |
| `hub.go` | `wraps.go`/`history.go` | `CursorPosition` callback records wraps; drain/resize push the flag; `History.Render` joins | ✓ WIRED |
| `hub.go` | `charstate.go` | ESC/OSC 8 handlers, `h.cs.scan(clean)` (`hub.go:575`), suffix appended to the body, `fixRenderedLinks` on rendered rows | ✓ WIRED |
| `App.tsx` `removeTabState` | `peerSnapshots.ts` | `dropPeerSnapshot(peerSnapshotHost(tab), tab.sessionId)` | ✓ WIRED |
| `App.tsx` | `TerminalPanel.tsx` | `snapshotHost={peerSnapshotHost(activeWebTab)}` through WebShareSessionView | ✓ WIRED |
| `relayClient.ts` | `TerminalPanel.tsx` | `onPasteInterrupted` sets `pasteNotice`, which renders `SessionEndedBanner kind="paste-interrupted"` | ✓ WIRED |
| `EnsureDaemon` | `startDaemon`/`waitForDaemon` | `daemonReadyWait`; `daemonReady` rejects a stale version | ✓ WIRED |

### Unchanged security surfaces (2e27e73c..HEAD)

`git diff --stat 2e27e73c..HEAD` is **empty** for all of these:
- `internal/webserver/` (CSP middleware `csp_mw.go`, share-token/capability checks, the 32768-byte input read limit)
- `internal/relay/protocol.go` (`MaxInputPayload = 32767`), `internal/relay/server.go`, `internal/relay/scrollback.go`
- `internal/daemon/remote_ws_proxy.go` (`ViewerReadLimit`), `internal/daemon/api.go`, `internal/daemon/peer_client.go` (the WR-07 `tailnet.LiveAllowed()` guard)
- `internal/capability/`, `internal/tailnet/` (`live.go`, the WR-07 guard), `app.go`
- `web/terminal.html`, `web/dashboard.html`, `go.mod`/`go.sum`

The relayClient diff has no line touching `MAX_INPUT_PAYLOAD = 32767` or `encodeInputFrames`. The only `web/` change is the 3-line WR-01 paste listener in `web/assets/terminal.js` (external JS, so the CSP is unaffected). **The 32 KiB limits, share tokens, CSP and WR-07 tailnet guard are unchanged.**

### Full gate (run once, `GOTOOLCHAIN=go1.26.11`, HEAD b812419b, no reruns needed)

```
go version go1.26.11 darwin/arm64
BUILD_OK            (go build ./...)
VET_OK              (go vet ./...)
go test -race -short ./...   (no -skip)
ok  	github.com/scottkw/portty	8.207s
?   	github.com/scottkw/portty/cmd/fakegh	[no test files]
ok  	github.com/scottkw/portty/internal/attach	6.073s
ok  	github.com/scottkw/portty/internal/capability	(cached)
ok  	github.com/scottkw/portty/internal/daemon	29.274s
ok  	github.com/scottkw/portty/internal/pty	(cached)
ok  	github.com/scottkw/portty/internal/relay	76.034s
ok  	github.com/scottkw/portty/internal/release	8.354s
ok  	github.com/scottkw/portty/internal/statusbar	(cached)
ok  	github.com/scottkw/portty/internal/tailnet	(cached)
?   	github.com/scottkw/portty/internal/testutil	[no test files]
ok  	github.com/scottkw/portty/internal/updater	(cached)
ok  	github.com/scottkw/portty/internal/wakewatch	(cached)
ok  	github.com/scottkw/portty/internal/webserver	12.590s
?   	github.com/scottkw/portty/web	[no test files]
GOTEST_EXIT=0
cd frontend && pnpm vitest run:  Test Files 91 passed (91) · Tests 1280 passed (1280)
bash tests/check-brand.sh:              OK: brand gate clean
bash tests/check-traceability-paths.sh: OK: 150 traceability paths exist
```

The cached packages have no Phase 8 changes, and a cached result is a prior pass on identical inputs.

### Behavioral Spot-Checks (targeted, `-race -short -count=1 -v`)

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| DEBT-01 heap bound at 80/220 | `go test ./internal/relay -run TestHistory_HeapBoundAfterAltScreenScroll` | cols=80: 0.1/0.1/0.1 MiB; cols=220: 0.2/0.2/0.1 MiB (bound 32) | ✓ PASS |
| DEBT-01 control | `-run TestAltScreenScrollback_ControlWithoutBound` | 30.6 MiB for 3,000 lines without the fix | ✓ PASS |
| DEBT-02/03 replay and rebuild | `-run 'TestSoftWrap_|TestRenderScreen_ModesBeforeText|TestFullReplay_ModesBeforeHistory|TestRenderedReplay_KeepsCharsetAndLink|TestAltScreenExit_Rebuild…|TestCharState_Scan|TestVT_IgnoresShiftOut…'` | 32 top-level tests PASS, `ok … relay 36.1s` | ✓ PASS |
| DEBT-06/CR-01 daemon start | `go test ./internal/daemon -run 'TestWaitForDaemon_|TestStartDaemon_|TestEnsureDaemon_NeverUnlinksSocket'` | 8 PASS, `ok … daemon 10.7s` | ✓ PASS |
| DEBT-02..05 frontend | `pnpm vitest run` on peerSnapshots, tabSwitch, App.open-remote, relayClient, TerminalPanel.reconnect, replayFidelity, pasteGuard | 7 files, 205 tests PASS | ✓ PASS |

### Probe Execution

No `scripts/*/tests/probe-*.sh` exist, and no Phase 8 plan declares a probe. Step 7c: not applicable.

### Anti-Patterns Found

| File | Pattern | Severity | Impact |
|------|---------|----------|--------|
| Phase 8 diff (37 files) | `TBD`/`FIXME`/`XXX` added | none found | No debt-marker blocker |
| Phase 8 diff | `TODO`/`HACK`/`PLACEHOLDER` added | none found | n/a |
| `hub.go`, `wraps.go`, `charstate.go` | 8 new `shortcut:` comments | ℹ️ Info | Documented, accepted limits (wrap trade, IN-13, charset G2/G3, DECRC link drift, and others) |
| `web/assets/terminal.js:1082-1085` (pre-existing, not changed by Phase 8 apart from WR-01) | A whole paste goes in one `makeInputFrame` | ⚠️ Warning (out of Phase 8 scope) | Follow-up 2 |

### Human Verification Required

These are the harvested `<human-check>` blocks from 08-04 and 08-05. Claude ran each one in the Tart VMs, as the user's rule requires, and the results are in 08-VM-CHECKS.md. They are listed so the user can acknowledge them. Only item 1 needs a real decision.

#### 1. V-05 / M-101 disposition (08-05 Task 3)

**Test:** Claude runs these in the two Tart VMs over SSH, including the tailnet drop inside the office VM; no human is needed.
**Expected:** V-04: delta on every return including the quick one, full after close and reopen; V-05: the paste-interrupted notice with N < M and the office file an exact prefix of the payload, stable after the reconnect; the 100 KB sub-run shows a paste notice (never silence) and its office file is a prefix too; V-06: no respawn or launch-blip line
**Why human:** V-05 is recorded as FAIL against this text (the notice read "may have been cut short (M KB)"), and the 100 KB sub-run was never run. The orchestrator dispositioned this as "DEBT-05 met". This verifier agrees on the merits (SC4), but an agent disposition is not the user's acceptance. **Decide:** accept the disposition (TESTING.md M-101 already passes on either wording), or ask for the 100 KB sub-run.

#### 2. V-01..V-03 (08-05 Task 2)

**Test:** Claude runs these in the two Tart VMs over SSH; no human is needed (user rule: drive GUI checks in the VM). The evidence lines in 08-VM-CHECKS.md are what the verifier reads.
**Expected:** V-01: in-alt peak within 32 MiB of baseline at 100 and 220 columns, post-exit within 20 MB; V-02: the long command is one line, the exact-width line is separate; V-03: post 1..60 intact, cursor on the prompt
**Why human:** Real daemon, WebKit tab and tailnet. Recorded: all PASS.

#### 3. L-01..L-05 cold launches (08-04 Task 1)

**Test:** Claude runs the reboots and launches in the Tart VM over SSH; no human is needed (user rule: drive GUI checks in the VM).
**Expected:** Five rows with the stderr counts, daemon pids and any launch-blip error text; an outcome section.
**Why human:** A real macOS boot. Recorded: L-01..L-05 PASS. The blip reproduced at the reopen-at-login (B-rows), was fixed, and L-06..L-09 PASS.

## Follow-ups

### 1. Top priority for unattended reliability: a version-mismatch restart ends every running session

**Found in:** U-01/U-02. **Predates Phase 8:** `handleShutdown` dates from `3d7fd1a2` (2026-04-02). **Does not block a Phase 8 criterion.**

What happens: when a GUI or CLI of a different `BuildVersion` finds a daemon running, `EnsureDaemon` logs `version mismatch … restarting` and calls `ShutdownDaemon`. `internal/daemon/api.go:281-290` `handleShutdown` replies 204 and calls `os.Exit(0)` 50 ms later. It does not even go through `engine.Manager().Shutdown()`. Every PTY closes, nothing migrates, and the new daemon's `/sessions` is `[]`. U-01 confirmed this in 8 of 8 runs, and U-02 in 3 of 3 with the CLI alone.

Why it matters here:
- **Every app upgrade while a daemon is running loses all sessions.** The daemon outlives the GUI, so that is the normal upgrade case. The updater only reveals the DMG and installs nothing on its own, so the trigger is user-initiated, but there is no warning.
- **An upgrade done from home, through a Portty session on the office Mac, ends the session it was done from**, plus every other session there (for example a long-running Claude Code), and nobody is at the office Mac to recover them.
- **A stray `portty` CLI of another version** (an old binary on PATH, a second install) triggers the same silent restart from a script or a shell.

Options, smallest first:
- (a) If sessions are live, do not restart silently. The GUI shows "N sessions running; restarting the daemon ends them" with Restart / Later. The CLI refuses with that message unless given `--restart-daemon`, and otherwise keeps talking to the old daemon where the API allows.
- (b) Keep the old daemon until it has no sessions, and run the new one only once it is idle.
- (c) Real handoff: pass the PTY master fds and ring/history state to the new daemon over the unix socket (`SCM_RIGHTS`). This is the only option that keeps sessions across an upgrade.

Recommend (a) now as a guardrail, and (c) as the planned fix. Capture this as a todo or a v1.2 requirement.

### 2. Browser share viewer does not split large pastes (pre-existing, outside Phase 8)

`web/assets/terminal.js:1082-1085` sends `term.onData` text as a single `makeInputFrame`. A paste over 32,767 bytes from a read-write share link in a plain browser would exceed the share server's 32,768-byte read limit and close the socket with 1009 (the behavior `TestShare_InputFrameOverLimitDropped` pins), losing the paste with no DEBT-05 notice. Phase 7's HARD-02 "browser view" meant the GUI's `WebShareSessionView`, which does split. This comes from code reading and was not run. Fix: port `encodeInputFrames` into terminal.js and add a test.

### 3. IN-06 and IN-07 have no todo; only IN-05 does

The brief says the deferred IN-05/06/07 have a todo. Only IN-05 does: `.planning/todos/pending/2026-11-19-paste-heartbeat-drop.md`, whose step 2 touches the ack that IN-06 depends on. IN-06 (the notice can fire for a paste that fully arrived) and IN-07 (`retryInit` does not call `dropPeerSnapshot`; peer tab ids are keyed by session id alone, so equal ids on two hosts collide as tabs) are recorded only in 08-REVIEW-FIX.md "Skipped Issues". Both are bounded and conservative, but under the phase goal ("no known deferred items") they should each have a todo.

### 4. Document the SO/G1 past-row limit

Add to `charstate.go`'s `shortcut:` (and optionally to a TESTING.md note) that rows already drawn through SO/G1 replay as ASCII in a rendered replay, because vt ignores SO/SI (`TestVT_IgnoresShiftOut`). The raw ring and live paths are unaffected, and G0 line drawing (xterm-256color's `smacs`) is unaffected.

### 5. Smaller items

- **The "N of M KB sent" wording is effectively unreachable on GUI paths**, because they go over loopback to the local daemon. It is covered by unit tests only. Keep it (harmless), or simplify to the one honest copy when IN-05/IN-06 are redesigned.
- **The 100 KB V-05 sub-run** was not run (see human item 1).
- **No iteration-4 code review** covers the iteration-3 fixes (CR-01 `abc0b032`, IN-15 `37522bcb`, IN-14 `69c4cb58`). Each has a regression test that failed before its fix, and CR-01 was VM-checked by U-01/U-02, so the risk is low.
- **IN-13 and the wraps.go identical-row trade** are accepted `shortcut:`s (reflow and copy only), pinned by tests.
- **DEBT-01:** any vt bump must re-review `vtalt.go`. The pin test enforces this.
- `TestCLIAttach_ViewingDaemonRestart` and the reconnect test were de-flaked in `c6db0dfb`/`af5a865b`. The daemon package passed in this gate on the first run.

### Gaps Summary

No gaps block the phase. All five ROADMAP success criteria and DEBT-01..07 have code, passing tests and VM evidence. The status is `human_needed` because the harvested VM human-checks need the user's acknowledgement. One of them (V-05) has a recorded FAIL against the plan's stricter wording, which this verifier judges does not affect SC4, and its 100 KB sub-run was never run. Follow-up 1 is not a Phase 8 gap, but it is the most important reliability item found: an upgrade ends every running session, silently.

---

_Verified: 2026-11-19_
_Verifier: Claude (gsd-verifier)_
