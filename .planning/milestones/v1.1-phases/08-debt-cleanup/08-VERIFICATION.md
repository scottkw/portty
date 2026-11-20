---
phase: 08-debt-cleanup
verified: 2026-11-20T00:58:29Z
status: passed
score: 6/6 must-haves verified
covered_files: [".planning/phases/07-hardening/07-SECURITY.md",".planning/phases/08-debt-cleanup/08-01-PLAN.md",".planning/phases/08-debt-cleanup/08-01-SUMMARY.md",".planning/phases/08-debt-cleanup/08-02-PLAN.md",".planning/phases/08-debt-cleanup/08-02-SUMMARY.md",".planning/phases/08-debt-cleanup/08-03-PLAN.md",".planning/phases/08-debt-cleanup/08-03-SUMMARY.md",".planning/phases/08-debt-cleanup/08-04-PLAN.md",".planning/phases/08-debt-cleanup/08-04-SUMMARY.md",".planning/phases/08-debt-cleanup/08-05-PLAN.md",".planning/phases/08-debt-cleanup/08-05-SUMMARY.md",".planning/phases/08-debt-cleanup/08-06-PLAN.md",".planning/phases/08-debt-cleanup/08-06-REVIEW-FIX.md",".planning/phases/08-debt-cleanup/08-06-REVIEW.md",".planning/phases/08-debt-cleanup/08-06-SUMMARY.md",".planning/phases/08-debt-cleanup/08-VM-CHECKS.md","TESTING.md","app.go","app_respawn_test.go","app_test.go","app_update.go","cmd_attach.go","cmd_attach_test.go","cmd_cli.go","cmd_cli_test.go","dispatch_test.go","frontend/src/App.tsx","frontend/src/components/SessionEndedBanner.tsx","frontend/src/components/TerminalPanel.tsx","frontend/src/components/WebShareSessionView.tsx","frontend/src/components/__tests__/App.daemon-restart.test.tsx","frontend/src/components/__tests__/App.open-remote.test.tsx","frontend/src/components/__tests__/TerminalPanel.reconnect.test.tsx","frontend/src/components/__tests__/TerminalPanel.scale.test.tsx","frontend/src/components/__tests__/TerminalPanel.tabSwitch.test.tsx","frontend/src/lib/__tests__/fixtures/charset-link-replay.json","frontend/src/lib/__tests__/fixtures/ris-replay.json","frontend/src/lib/__tests__/fixtures/softwrap-replay.json","frontend/src/lib/__tests__/fixtures/wrapped-replay.json","frontend/src/lib/__tests__/pasteGuard.test.ts","frontend/src/lib/__tests__/peerSnapshots.test.ts","frontend/src/lib/__tests__/replayFidelity.test.ts","frontend/src/lib/pasteGuard.ts","frontend/src/lib/peerSnapshots.ts","frontend/src/lib/relayClient.test.ts","frontend/src/lib/relayClient.ts","frontend/src/lib/webTerminalInput.test.ts","frontend/src/wailsjs/go/main/App.d.ts","frontend/src/wailsjs/go/main/App.js","internal/daemon/attach_viewers_test.go","internal/daemon/client.go","internal/daemon/process.go","internal/daemon/process_test.go","internal/daemon/process_unix.go","internal/relay/altscreen_heap_test.go","internal/relay/charstate.go","internal/relay/emu_nohang_test.go","internal/relay/fuzz_test.go","internal/relay/history.go","internal/relay/history_test.go","internal/relay/hub.go","internal/relay/replay_fidelity_test.go","internal/relay/softwrap_test.go","internal/relay/vtalt.go","internal/relay/vtalt_test.go","internal/relay/wraps.go","main.go","web/assets/terminal.js"]
covered_digest: "v4:sha256:c510eca12b77985daca178840eb493459b125311ffe79c14e363d1fda2ebca23"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: human_needed
  previous_score: 5/5
  gaps_closed:
    - "Human item 1 (V-05 / M-101 disposition): accepted by the user on 2026-11-19"
    - "Human items 2 and 3 (V-01..V-03, L-01..L-05): VM runs Claude drove under the user's VM-testing rule, all PASS in 08-VM-CHECKS.md; no decision was asked of the user"
    - "Initial follow-ups 2-4 (browser paste split, IN-06/IN-07 todos, SO/G1 shortcut): closed in 358550a8, c5a0a7c8 and 4356835a"
    - "Initial follow-up 1 (an upgrade silently ends sessions): became DEBT-08 / SC6 (plan 08-06), verified below"
  gaps_remaining: []
  regressions: []
---

# Phase 8: Debt Cleanup Verification Report

**Phase Goal:** The tech debt recorded in the v1.1 audit is closed, so v1.1 ships with no known deferred items in its own code.
**Verified:** 2026-11-20T00:58:29Z (branch `gsd/phase-07-hardening`, HEAD `5dc843bf`)
**Status:** passed (6/6 success criteria, DEBT-01..08 all satisfied; no open human checks; follow-ups below are bookkeeping and v1.2 items, none blocks a criterion)
**Re-verification:** Yes. This refreshes the 2026-11-19 report, which went stale when plan 08-06 (DEBT-08), its review fixes, the launch fix `dcbe999d` and follow-ups 2-4 changed covered files.

Phase 8 code range: `2e27e73c..5dc843bf`, 52 non-planning files. `git diff --stat 14ad5b6e..HEAD -- . ':!.planning'` is empty, and `git diff --stat dcbe999d..HEAD -- . ':!.planning'` is empty too. So the orchestrator's full gate at 14ad5b6e and the VM re-run G-00, G-04..G-06 (built at `dcbe999d`) both cover exactly HEAD's code.

## Goal Achievement

### Observable Truths (ROADMAP success criteria)

| # | Success criterion | Status | Evidence |
|---|-------------------|--------|----------|
| 1 | A full-screen app scrolling 20,000+ full-width lines in the alt screen keeps per-session heap growth under a small fixed bound that does not grow with width (soak at 80 and 220 columns, VM check). Alt screen and main history stay correct for joining viewers. | ✓ VERIFIED | Unchanged since the initial verification (`vtalt.go`, `hub.go`, `history.go` have no diff since b812419b). `vtalt.go` sets vt's alt Screen scrollback to nil in `newLiveEmulator` (every build, resize and alt-exit rebuild), with a type and identity check, a logged fallback to the HARD-01 exit rebuild, and the `pinnedVT` test on the go.mod version. Soak `TestHistory_HeapBoundAfterAltScreenScroll` (40,000 lines): 0.1 MiB at 80 columns, 0.2 MiB at 220, bound 32 MiB; control `TestAltScreenScrollback_ControlWithoutBound` 30.6 MiB for 3,000 lines. Join tests `TestAltScreen_JoinWhileInAltKeepsMainHistory`, `TestAltScreenExit_KeepsMainScreenAndHistory`, `TestDisableAltScreenScrollback_KeepsMainHistory` PASS. VM V-01 PASS: +5 MiB at C=100, +0 MiB at C=220, against +123 MiB before the fix. |
| 2 | Replays are faithful: a soft-wrapped long line replays as one wrapped line; rendered replays and the alt-exit rebuild restore modes before text, keep SO/G1 line drawing and an open OSC 8 link; rebuild fuzz stays at 0 mismatches. | ✓ VERIFIED (limit recorded) | Unchanged since the initial verification, apart from a comment-only `shortcut:` added to `charstate.go` (4356835a) that now documents the SO/G1 past-row limit. Soft wrap: `wraps.go`, the per-line `wrapped` flag, `History.Render` without CRLF; 15 `TestSoftWrap_*` PASS including the alt-exit and resize rebuilds and the real-xterm.js fixture; VM V-02 PASS. Modes first: `TestRenderScreen_ModesBeforeText`, `TestFullReplay_ModesBeforeHistory`; VM V-03 PASS. SO/G1 and OSC 8: `charstate.go` tracker, `TestRenderedReplay_KeepsCharsetAndLink`, the `charset-link-replay` vitest. Fuzz `TestAltScreenExit_RebuildFuzz`, 600 streams, 0 mismatches. See "DEBT-03 judgment". |
| 3 | The peer-tab snapshot store is bounded by bytes, frees a closed tab's entry, is keyed by host plus session, never leaks a terminal, and a quick switch back still gets a delta rejoin. | ✓ VERIFIED | `peerSnapshots.ts`, `TerminalPanel.tsx` and `WebShareSessionView.tsx` have no diff since b812419b; App.tsx's 08-06 change only adds the restart dialog. 8 MiB budget plus 8-entry cap, `peerSnapshotKey(host, sessionId)`, the per-key generation in `dropPeerSnapshot`, `removeTabState` dropping the entry on every close path, the 1 s flush-timeout dispose, the 250 ms pending-save wait. 205 targeted vitest PASS in the initial run; the full vitest (93 files, 1288 tests) passes at 14ad5b6e. VM V-04 PASS (quick switch `mode=delta bytes=0 resume=true` after 1.05 s). |
| 4 | A paste cut short by a dropped socket tells the user so. No frame is sent twice, and none is silently lost. | ✓ VERIFIED (V-05 accepted by the user) | `relayClient.ts` and `SessionEndedBanner.tsx` unchanged since b812419b. Ack ping after each split paste, `checkPaste` on any socket death not caused by `close()`, `onPasteInterrupted` to the banner, no queue and no resend, input dropped unless `open`. relayClient "partial paste on a socket drop (DEBT-05)" and TerminalPanel.reconnect PASS. VM V-05: the notice appeared on 5 MB and 20 MB pastes cut by `tailscale down`, office files were exact prefixes that never grew. The user accepted the "may have been cut short" wording on the proxy path on 2026-11-19; the 100 KB sub-run is not required. Follow-up 2 (closed in 358550a8) also made the browser share viewer split pastes into 32767-byte frames (`web/assets/terminal.js` `makeInputFrames`); the browser viewer has no paste notice yet (todo `2026-11-19-web-viewer-paste-notice.md`, v1.2 seed). |
| 5 | The HARD-04 launch blip has a root-cause fix (or a documented VM non-reproduction with diagnostics). The Phase 7 SUMMARYs carry `requirements_completed`, and Phase 7 has a SECURITY.md. | ✓ VERIFIED | Root cause reproduced (B-01/B-02/B-07/B-08) and fixed with `daemonReadyWait = 15 * time.Second`. The 08-06 diff to `process.go` touches only `EnsureDaemon` (the consent seam) and renames `staleVersion` to `StaleVersion` with the same body; `daemonReadyWait`, `waitForDaemon`, `startDaemon` and `daemonReady` are unchanged except that `daemonReady` now reads the version as `(string, error)` and treats an unreadable one as not ready (IN-06, the same result as before). **Re-run today:** `TestWaitForDaemon_SlowStart`, `_DiedAtSpawnFailsFast`, `_LostLockWaitsForHolder`, `_LockReleasedWithoutBindFailsFast`, `TestStartDaemon_WaitsForOldDaemonLock`, `_LateLockReleaseKeepsSpawnWindow`, `_MismatchedDaemonAnsweringIsNotReady` all PASS. VM L-06..L-09, V-06, P-03, P-04, U-01/U-02 PASS. Bookkeeping: all five 07 SUMMARYs carry HARD IDs in `requirements_completed`; `07-SECURITY.md` has `threats_open: 0`. |
| 6 | Upgrading Portty never silently ends live sessions. With sessions running, a version-mismatch restart needs consent: a GUI dialog, or the CLI's `--restart-daemon`. Declining keeps the old daemon and its sessions working. Proven by tests and a VM upgrade run. | ✓ VERIFIED (narrow race recorded) | **Seam** (`internal/daemon/process.go:112-160`): `EnsureDaemon(socketPath, RestartPolicy)`. On `StaleVersion`, `liveSessions` counts non-stopped sessions (-1 when `/sessions` fails). A count other than 0 asks the policy; "no" logs `… keeping it` and returns nil without `/shutdown`. The count is read again right before `/shutdown`, and a grown or known/unknown-flipped count (`countGrew`) asks again (WR-02). An unreadable version keeps the daemon (IN-06). Zero sessions restart automatically. **Every caller passes a policy:** `git grep` finds `EnsureDaemon` reached only through `App.ensure()` (GUI startup, the poller, Retry, recovery adoption IN-05) with `restartConsent`, `ensureCLIDaemon` in `main.go:245`, and the attach reconnect (`cmd_attach.go:160`) with `keepMismatchedDaemon`. The only other `ShutdownDaemon` call is Quit All (`app.go:597`), behind the Quit modal. **GUI:** `restartConsent` never blocks; it keeps the daemon, stores the count, emits `daemon:restart-needed` and calls `showWindow` (WR-03). `AnswerDaemonRestart(false)` stops prompts this launch; `(true)` runs one EnsureDaemon under `daemonMu` and approves only the count shown or fewer (WR-01), and refuses with no open question (IN-01). `App.tsx` pulls `GetDaemonRestartPrompt` on mount and subscribes to the event; `RegenerateKeyModal` shows "Update Portty's background service?", "N sessions are running. Restarting ends them." (1 and unknown variants), Not Now focused, Restart Now destructive. Hand-maintained wailsjs stubs carry both bindings. **CLI:** `ensureCLIDaemon` turns a "no" into the refusal (`portty: the running daemon is version X with N live sessions; …`), exit 1; `--restart-daemon` consents, `--keep-daemon` uses the old daemon; both are taken only right after the command (`takeLeadingFlag`, WR-04); `health`, `version` and help skip the daemon (IN-04). **Launch fix** `dcbe999d`: `launchMode` is a pure function; a bare launch is `modeGUI`, and the `os.Args[1]` panic is gone. **Tests re-run today:** 30 top-level tests PASS (list below), including `TestEnsureDaemon_StaleRestartConsent`, `TestAnswerDaemonRestart_ApprovesShownCountOnly`, `TestEnsureCLIDaemon_RestartConsent`, `TestRestartDaemonFlagPlacement` and `TestLaunchMode` ("no args" and "empty" cases); vitest `App.daemon-restart.test.tsx` 5/5 PASS. **VM:** G-01 (Not Now keeps both sessions, typing reaches them, no re-prompt in 25 s), G-02 (Restart Now gives one daemon B), G-03 (CLI refuses rc 1, `--restart-daemon` restarts, zero sessions stay automatic) PASS at 9c2c9c20. After the review fixes, G-04 first FAILED (bare launch panicked), was fixed in `dcbe999d`, and the re-run at `dcbe999d` passed G-00 (bare launch, direct and LaunchServices), G-04 (start minimized and hidden window show the dialog), G-05 (flag placement), G-06 (a grown count asks again). See "SC6 judgment". |

**Score:** 6/6 truths verified (0 present-but-behavior-unverified)

### SC6 judgment: the recount-to-shutdown window and Not Now

- **The few-ms window.** A session created between the second count and `/shutdown` still ends without being asked. It is a documented `shortcut:` in `process.go:144-147`. It cannot be closed from the new binary alone, because the old daemon's `/shutdown` takes no condition, and the old daemon is the one already deployed. Hitting it needs a session created within milliseconds of a restart the user just approved. **Verdict:** SC6 is met. "Never silently" holds for every path a user or viewer can practically hit. The fix belongs with the v1.2 session handoff (a conditional `/shutdown?ifSessions=<n>` or the fd handoff), as the comment says.
- **Not Now keeps an old daemon with no API-compatibility check** (08-06-REVIEW IN-03, deferred). The reviewer checked routes, JSON tags and relay frames against v1.0, the only release, and found them compatible. The first release that changes a route or payload has to decide this. See follow-up 2.
- **The guard runs in the new binary**, which is the one that finds the old daemon. So an upgrade from v1.0 to v1.1 is already protected, even though v1.0 has no guard.

### DEBT-03 judgment: SO/G1 rows drawn before a viewer joins (kept from the initial verification)

vt ignores SO/SI (`TestVT_IgnoresShiftOut`), so rows drawn through G1 before a viewer joins replay as ASCII in a rendered replay. Raw ring replay and the live stream are unaffected. **SC2 is met and the limit is acceptable:** the requirement is about charset state carried across the replay or rebuild (07-REVIEW-FIX IN-08), which is fixed and tested; past-row content would need a vt fork; `TERM=xterm-256color` draws line drawing through G0 (`smacs=\E(0`), which vt renders. The limit is now documented in the `charstate.go` `shortcut:` (4356835a).

### DEBT-01 judgment: reflect/unsafe with a version pin (kept from the initial verification)

Acceptable. It reads one unexported field (`Emulator.scrs`) of a pinned module, with a runtime type and identity check, a graceful logged fallback, and `TestVTLayout_AltScreenScrollback` failing on any vt version change. The write goes through vt's exported `Screen.SetScrollback(nil)`. `go vet` is clean.

### Requirements Coverage

| Requirement | Source plan | Description (short) | Status | Evidence |
|-------------|-------------|---------------------|--------|----------|
| DEBT-01 | 08-01, 08-05 | Alt-screen heap bound per session, independent of width | ✓ SATISFIED | SC1: `vtalt.go`, soak 0.1/0.2 MiB at 80/220, V-01 |
| DEBT-02 | 08-01, 08-05 | Soft-wrapped line comes back as one wrapped line, in replay and after a rebuild | ✓ SATISFIED | SC2: `wraps.go`/`history.go`, 15 `TestSoftWrap_*`, xterm.js fixture, V-02; accepted `shortcut:`s pinned by tests |
| DEBT-03 | 08-02, 08-05 | Modes before text (IN-06); keep G1 via SO and an open OSC 8 link (IN-08) | ✓ SATISFIED | SC2 and the DEBT-03 judgment; fuzz 0 of 600; V-03 |
| DEBT-04 | 08-03, 08-05 | Snapshot store bounded by bytes, dropped on close, host+session key, no xterm leak, quick-switch delta | ✓ SATISFIED | SC3; V-04. Residual IN-07 has a todo (`2026-11-19-peer-snapshot-tab-replace.md`) and is in the v1.2 seed |
| DEBT-05 | 08-03, 08-05 | User told of a cut paste; never silently lost, never sent twice | ✓ SATISFIED | SC4; V-05 accepted by the user; P-01/P-02 |
| DEBT-06 | 08-04, 08-05 | Root-cause fix of the launch blip, or a documented non-reproduction | ✓ SATISFIED | SC5: reproduced, root cause, fix `c27b69d3`, seam tests re-run today, L-06..L-09/V-06/P-04 |
| DEBT-07 | 08-04, 08-05 | 07 SUMMARY `requirements_completed` plus 07-SECURITY.md | ✓ SATISFIED | SC5: summary-extract output, `threats_open: 0` |
| DEBT-08 | 08-06 | Version-mismatch restart never ends live sessions without consent; GUI Restart now / Not now; CLI refuses unless `--restart-daemon`; automatic with no sessions | ✓ SATISFIED | SC6: seam, GUI, CLI, launch fix; 30 Go tests and 5 vitest re-run today; G-00..G-06 PASS (G-04 after `dcbe999d`) |

No orphaned requirements: REQUIREMENTS.md maps DEBT-01..08 to Phase 8, and every one is claimed by a plan (08-06 claims DEBT-08).

### Required Artifacts

| Artifact | Status | Details |
|----------|--------|---------|
| `internal/relay/vtalt.go`, `vtalt_test.go` | ✓ VERIFIED | Unchanged since the initial verification; wired at `hub.go` `newLiveEmulator` |
| `internal/relay/wraps.go`, `history.go`, `charstate.go` | ✓ VERIFIED | Unchanged apart from the `charstate.go` comment |
| `frontend/src/lib/peerSnapshots.ts`, `relayClient.ts`, `pasteGuard.ts`, `SessionEndedBanner.tsx` | ✓ VERIFIED | Unchanged since b812419b |
| `internal/daemon/process.go` | ✓ VERIFIED | `RestartPolicy`, `EnsureDaemon` consent and recount, `liveSessions`, `countGrew`, `StaleVersion`; `daemonReadyWait` 15 s unchanged |
| `internal/daemon/client.go` | ✓ VERIFIED | `GetDaemonVersion` returns `(string, error)` (IN-06) |
| `app.go` | ✓ VERIFIED | `ensure()` seam, `restartConsent`, `GetDaemonRestartPrompt`, `AnswerDaemonRestart`, recovery adoption through ensure; Quit All unchanged |
| `cmd_cli.go`, `main.go` | ✓ VERIFIED | `ensureCLIDaemon`, `takeLeadingFlag`, `cliNeedsDaemon`, `launchMode` |
| `cmd_attach.go` | ✓ VERIFIED | `keepMismatchedDaemon` on the reconnect |
| `frontend/src/App.tsx`, `wailsjs/go/main/App.{js,d.ts}` | ✓ VERIFIED | Dialog wired to both bindings and the event |
| `frontend/src/components/__tests__/App.daemon-restart.test.tsx` | ✓ VERIFIED | 5 tests PASS |
| `web/assets/terminal.js` | ✓ VERIFIED | `makeInputFrames` splits at 32767 (follow-up 2, HARD-02 gap) |
| `.planning/phases/08-debt-cleanup/08-VM-CHECKS.md` | ✓ VERIFIED | L/B, V-01..V-06, P-01..P-04, U-01..U-02, G-00..G-06 |
| `.planning/phases/07-hardening/07-SECURITY.md` | ✓ VERIFIED | `threats_open: 0` |
| `TESTING.md` | ✓ VERIFIED (one stale line) | Section 4 has 10 DEBT-08 rows; Section 5 Category AX / M-104; traceability 152 paths OK; brand clean. M-104 "Last run" is stale (follow-up 1) |

### Key Link Verification

| From | To | Via | Status |
|------|----|-----|--------|
| `hub.go` `newLiveEmulator` | `vtalt.go` | `disableAltScreenScrollback(emu)` on every build | ✓ WIRED |
| `hub.go` | `wraps.go`/`history.go`/`charstate.go` | wrap recording, flag push, `h.cs.scan`, suffix, `fixRenderedLinks` | ✓ WIRED |
| `App.tsx` `removeTabState` | `peerSnapshots.ts` | `dropPeerSnapshot(peerSnapshotHost(tab), tab.sessionId)` | ✓ WIRED |
| `relayClient.ts` | `TerminalPanel.tsx` | `onPasteInterrupted` → `pasteNotice` → `SessionEndedBanner` | ✓ WIRED |
| `EnsureDaemon` | `startDaemon`/`waitForDaemon` | `daemonReadyWait`; `daemonReady` rejects a stale or unreadable version | ✓ WIRED |
| `App.ensure()` | `daemon.EnsureDaemon` | `ensureDaemon(socketPath, a.restartConsent)` (`app.go:303`) | ✓ WIRED |
| `restartConsent` | `App.tsx` dialog | `daemon:restart-needed` event plus `GetDaemonRestartPrompt` pull | ✓ WIRED |
| `App.tsx` dialog | `AnswerDaemonRestart` | wailsjs `Call('main.App.AnswerDaemonRestart', [restart])` | ✓ WIRED |
| `runCLI` | `ensureCLIDaemon` | `main.go:244-247`, exit 1 on refusal | ✓ WIRED |
| attach reconnect | `EnsureDaemon` | `ensureDaemon(socketPath, keepMismatchedDaemon)` (`cmd_attach.go:160`) | ✓ WIRED |
| `main()` | `runGUI`/`runCLI`/`usage` | `launchMode(os.Args[1:])` | ✓ WIRED |

### Unchanged security surfaces (2e27e73c..HEAD)

`git diff --stat 2e27e73c..HEAD` is empty for `internal/webserver/`, `internal/relay/protocol.go`, `server.go`, `scrollback.go`, `internal/daemon/remote_ws_proxy.go`, `api.go`, `peer_client.go`, `internal/capability/`, `internal/tailnet/`, `web/terminal.html`, `web/dashboard.html`, `go.mod`, `go.sum`. `app.go` now changes (08-06), but only daemon lifecycle and consent: no share, token or CSP code. The `web/` changes are the WR-01 paste listener and the 32767-byte split in external `terminal.js`, so the CSP is unaffected. **The 32 KiB limits, share tokens, CSP and WR-07 tailnet guard are unchanged.** The new bindings `GetDaemonRestartPrompt` and `AnswerDaemonRestart` are local Wails bindings with no network surface.

### Gate

**Full gate (orchestrator, once, at 14ad5b6e; HEAD adds only planning docs):** build and vet OK; `go test -race -short ./...` all 12 packages ok; vitest 93 files, 1288 tests; `tsc --noEmit` OK; brand clean; 152 traceability paths. Not re-run here, because each test binary raises a firewall alert for the user.

**Checked here at HEAD:** `git diff --stat 14ad5b6e..HEAD -- . ':!.planning'` empty; `go build ./...` and `go vet . ./internal/daemon/` OK; `bash tests/check-traceability-paths.sh` "OK: 152 traceability paths exist"; `bash tests/check-brand.sh` "OK: brand gate clean".

### Behavioral Spot-Checks (targeted, at HEAD)

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| DEBT-08 consent, CLI, GUI wiring; launch fix; DEBT-06 daemon start | `GOTOOLCHAIN=go1.26.11 go test -race -count=1 -v -run 'TestLaunchMode\|Restart\|Consent\|EnsureDaemon\|StartDaemon\|WaitForDaemon\|EnsureLocalDaemon\|RecoveryChecksVersion\|CLINeedsDaemon\|CreateSession_WaitsOutRestarts\|Ensure_Default' . ./internal/daemon/` | `ok github.com/scottkw/portty 1.6s`, `ok …/internal/daemon 11.0s`; 30 top-level PASS: TestRespawnDaemon_RecoveryChecksVersion, TestStartDaemon_SetsLaunchedAtOnSuccess, TestStartDaemon_AlwaysClosesStartupDone, TestRestartConsent_NotNowKeepsDaemonWithoutAskingAgain, TestAnswerDaemonRestart_RestartNow, TestAnswerDaemonRestart_ApprovesShownCountOnly, TestEnsure_DefaultSeamPassesRestartConsent, TestAnswerDaemonRestart_NoPendingPrompt, TestAnswerDaemonRestart_FailedAfterStopRaisesDaemonError, TestRestartConsent_ShowsWindowWhenAsking, TestCreateSession_WaitsOutRestarts, TestEnsureLocalDaemon_KeepsMismatchedDaemon, TestEnsureCLIDaemon_RestartConsent, TestCLINeedsDaemon, TestRestartDaemonFlagPlacement, TestLaunchMode, TestCLIAttach_ViewingDaemonRestart, TestEnsureDaemon_AlreadyRunning, _Timeout, _VersionMismatchDetected, _NeverUnlinksSocket, TestWaitForDaemon_SlowStart, _DiedAtSpawnFailsFast, _LostLockWaitsForHolder, _LockReleasedWithoutBindFailsFast, TestStartDaemon_WaitsForOldDaemonLock, _LateLockReleaseKeepsSpawnWindow, _MismatchedDaemonAnsweringIsNotReady, TestEnsureDaemon_StaleRestartConsent, TestEnsureDaemon_UnreadableVersionIsNotAMismatch | ✓ PASS |
| DEBT-08 dialog | `pnpm vitest run src/components/__tests__/App.daemon-restart.test.tsx` | 1 file, 5 tests passed | ✓ PASS |

The DEBT-01..05 targeted runs from the initial verification are not repeated: none of their source or test files changed since b812419b (only the `charstate.go` comment), and the 14ad5b6e full gate passed them.

### Probe Execution

No `scripts/*/tests/probe-*.sh` exist, and no Phase 8 plan declares a probe. Not applicable.

### Anti-Patterns Found

| File | Pattern | Severity | Impact |
|------|---------|----------|--------|
| Phase 8 diff (52 files) | `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` added | none found | No debt-marker blocker |
| `process.go:144` | `shortcut:` recount-to-shutdown window | ℹ️ Info | Accepted (SC6 judgment); points at the v1.2 handoff |
| `charstate.go` | `shortcut:` SO/G1 past rows | ℹ️ Info | Accepted (DEBT-03 judgment) |
| `web/assets/terminal.js` | `shortcut:` no paste notice in the browser viewer | ℹ️ Info | Todo `2026-11-19-web-viewer-paste-notice.md`, in the v1.2 seed |
| `hub.go`, `wraps.go` | earlier `shortcut:` comments | ℹ️ Info | Accepted, pinned by tests |

### Human Verification

None open. The three harvested checks from the initial verification are resolved:
1. **V-05 / M-101 (08-05 Task 3):** the user accepted the disposition on 2026-11-19 (see "User decisions").
2. **V-01..V-03 (08-05 Task 2)** and 3. **L-01..L-05 (08-04 Task 1):** Claude ran these in the Tart VMs under the user's rule that Claude drives GUI checks in the VM. All PASS, with evidence in 08-VM-CHECKS.md. They were listed for acknowledgement only.

Plan 08-06 has no `<human-check>` blocks. Its VM runs (G-00..G-06) were driven by Claude under the same rule, and the final results are all PASS.

## User decisions (2026-11-19)

- **V-05:** the user accepted it. DEBT-05/SC4 is met with the "may have been cut short" wording on the proxy path; the 100 KB sub-run is not required.
- **Follow-up 1 of the initial verification (upgrade ends sessions):** "Guard now, keep later". DEBT-08 / SC6 was added to Phase 8 as plan 08-06 (consent before a version-mismatch restart ends live sessions). Keeping sessions alive across upgrades is the first v1.2 phase (`SEED-261119-nus`, item 0).
- **Follow-ups 2-4 of the initial verification:** closed in 358550a8 (browser share viewer paste split, a HARD-02 gap), c5a0a7c8 (todos for IN-06 and IN-07) and 4356835a (charstate shortcut).

## DEBT-08 / SC6 history (added after the initial verification)

- **Plan 08-06:** consent before a version-mismatch restart ends live sessions. One code review (0 critical, 4 warnings, 8 info) and one fix pass followed: 10 fixed, with IN-03 and IN-07 deferred to the v1.2 session handoff.
- **VM:**
  - G-01..G-03 PASS: Not Now keeps the sessions, Restart Now restarts, the CLI refuses without the flag.
  - G-04 first run FAIL: a no-argument launch panicked (`os.Args[1]` read without a length guard, added by WR-04/IN-04). Fixed in dcbe999d with the `launchMode` unit test.
  - Re-run G-00 and G-04..G-06 PASS at dcbe999d: a bare launch works (and Wails binding generation, which runs the binary with no arguments, logs `Done`), the prompt is visible after a minimized start or a hidden GUI, all CLI flag positions behave, and a grown session count makes it ask again.
- **Final gate at 14ad5b6e:** build and vet OK; `go test -race -short ./...` passes all 12 packages; vitest 93 files and 1288 tests pass; tsc OK; brand clean; 152 traceability paths.

## Follow-ups (none blocks a Phase 8 criterion)

1. **TESTING.md M-104 "Last run" is stale.** It still says "build 9c2c9c20: G-01, G-02 and G-03 PASS …; steps 4 and 5 not yet run". Steps 4 and 5 ran at `dcbe999d` (G-05, G-04 PASS), along with the bare-launch check G-00. Update the line, and optionally add the bare launch to M-104 and a Section 4 note for `TestLaunchMode` (`dispatch_test.go` is an existing file, so the standing rule does not require a row).
2. **08-06 deferrals IN-03 and IN-07 have no todo**, and the v1.2 seed does not name them. They are recorded only in 08-06-REVIEW-FIX.md "Skipped Issues". IN-03: Not Now keeps an old daemon with no API-compatibility check (compatible with v1.0 today; the next route or payload change must decide). IN-07: Quit All can wait up to about 30 s behind an approved restart. Under the phase goal ("no known deferred items"), add each to a todo or to the seed's item 0, as was done for the 08-05 IN-06/IN-07.
3. **Known DEBT-08 dialog defect, deferred with a todo:** Cmd-Q while the restart dialog is open draws the Quit dialog underneath it, and Quit GUI Only does nothing (`2026-11-19-quit-dialog-under-restart-dialog.md`, `resolves_phase: null`, found in G-04). It ends no sessions. Schedule it into the v1.2 first phase.
4. **Nine v1.0 todos are still in `todos/pending/` with `resolves_phase: 7`**: `alt-screen-scrollback-growth`, `split-large-pastes`, `spurious-respawn-at-launch`, `tab-switch-full-replay`, `strip-more-terminal-queries-from-replay`, `isolate-daemon-settings-tests`, `daemon-tests-read-host-settings`, `relay-fanout-flake`, `document-multi-line-prompt-resize-artifacts`. HARD-01..08 and DEBT-01/06 closed them, but the files were never moved to `todos/completed/`. Move them (or close them at milestone archive) so `/gsd-audit-milestone` does not count them as open.
5. **Deferred to v1.2 (in the seed or todos):** the session handoff across an upgrade (seed item 0, which also closes the SC6 recount window), the GUI ignoring SIGTERM (U-01), `paste-heartbeat-drop` (IN-05), `paste-notice-false-positive` (IN-06), `peer-snapshot-tab-replace` (IN-07), `web-viewer-paste-notice`. The `exitcode-stopped-flake` todo (`resolves_phase: null`) is a test flake to schedule.

## Gaps Summary

No gaps. All six ROADMAP success criteria and DEBT-01..08 have code, wiring, passing tests (re-run today for SC5/SC6 and the launch fix; the full gate at 14ad5b6e covers the rest, and HEAD's code is identical) and VM evidence. The open items are bookkeeping (a stale TESTING.md line, nine resolved todos still in pending, two 08-06 deferrals without a todo) and v1.2 work the user scheduled. The phase can complete.

---

_Verified: 2026-11-20T00:58:29Z_
_Verifier: Claude (gsd-verifier)_
