---
phase: 09-upgrades-keep-sessions
verified: 2026-11-22T22:15:00Z
status: passed
score: 5/5 must-haves verified
re_verification: true
previous_verified: 2026-11-20T22:30:16Z
covered_files: [".planning/phases/09-upgrades-keep-sessions/09-01-PLAN.md", ".planning/phases/09-upgrades-keep-sessions/09-01-SUMMARY.md", ".planning/phases/09-upgrades-keep-sessions/09-02-PLAN.md", ".planning/phases/09-upgrades-keep-sessions/09-02-SUMMARY.md", ".planning/phases/09-upgrades-keep-sessions/09-03-PLAN.md", ".planning/phases/09-upgrades-keep-sessions/09-03-SUMMARY.md", ".planning/phases/09-upgrades-keep-sessions/09-04-PLAN.md", ".planning/phases/09-upgrades-keep-sessions/09-04-SUMMARY.md", ".planning/phases/09-upgrades-keep-sessions/09-05-PLAN.md", ".planning/phases/09-upgrades-keep-sessions/09-05-SUMMARY.md", ".planning/phases/09-upgrades-keep-sessions/09-06-PLAN.md", ".planning/phases/09-upgrades-keep-sessions/09-06-SUMMARY.md", ".planning/phases/09-upgrades-keep-sessions/09-07-FIX.md", ".planning/phases/09-upgrades-keep-sessions/09-07-FIX2.md", ".planning/phases/09-upgrades-keep-sessions/09-07-PLAN.md", ".planning/phases/09-upgrades-keep-sessions/09-07-SUMMARY.md", ".planning/phases/09-upgrades-keep-sessions/09-REVIEW-FIX.md", ".planning/phases/09-upgrades-keep-sessions/09-SECURITY.md", ".planning/phases/09-upgrades-keep-sessions/09-VM-CHECKS.md", "TESTING.md", "app.go", "cmd_attach.go", "cmd_cli.go", "cmd_daemon.go", "frontend/src/App.tsx", "frontend/src/components/TerminalPanel.tsx", "frontend/src/lib/hostReconnect.ts", "internal/attach/loop.go", "internal/daemon/api.go", "internal/daemon/client.go", "internal/daemon/compat_vmpreview.go", "internal/daemon/engine.go", "internal/daemon/handoff.go", "internal/daemon/handoff_vmpreview.go", "internal/daemon/process.go", "internal/daemon/process_unix.go", "internal/daemon/share.go", "internal/daemon/types.go", "internal/fdpass/fdpass.go", "internal/pty/adopt_darwin.go", "internal/pty/exitwatch_darwin.go", "internal/pty/native.go", "internal/pty/session.go", "internal/relay/close_reasons.go", "internal/relay/handoff_state.go", "internal/relay/hub.go", "internal/relay/manager.go", "internal/relay/server.go", "internal/webserver/server.go", "terminate_darwin.go", "terminate_darwin.m"]
covered_digest: "v4:sha256:e28fa75bc4db2d51faf0a09eb4a772ea7d1c87dd06bb1a69dd762aeb2f0a72de"
behavior_unverified: 0
overrides_applied: 0
---

# Phase 9: Upgrades Keep Sessions — Verification Report

**Phase Goal:** Upgrading Portty never costs a session. The new daemon takes over the running shells and their history, every attached viewer carries on by itself, and if the takeover fails the old daemon keeps serving. Nobody has to be at the remote Mac.
**Verified:** 2026-11-22, at HEAD `52d960a5` (branch gsd/v1.2), before the v1.2 close.
**Status:** passed, 5/5
**Re-verification:** Yes. The 2026-11-20 report passed. The GSD tool marked it stale because Phases 10–14 changed files it covered: hub.go, handoff.go, handoff_state.go, relay/server.go, webserver/server.go, client.go, types.go, api.go, app.go, App.tsx, TerminalPanel.tsx and TESTING.md. 09-SECURITY.md was also added after that report.

## What was re-checked against the current code

**Targeted tests run now.** I ran only the named tests, because other verifiers were running at the same time. I used `GOTOOLCHAIN=go1.26.11`, and node v24.22.1 with `NO_COLOR=1` for vitest. All passed:

| Package / files | Selection | Result |
|---|---|---|
| `./internal/daemon/` | `-run 'TestHandoff\|TestTakeover\|TestEnsureDaemon\|TestStartDetachedTakeover\|TestTakeoverStatus\|TestStartDaemon_Cancel\|TestSessionInfo_ReportsAPIVersion\|Health\|DaemonAPI'`. This covers HappyPath, every `TestHandoffFault_*` (EveryStep, NoThawUntilTakeoverExits, CommittedWriteFailsOldAlive, OldDies*, NewHangsAfterCommit, WatchdogAfterCommittedWritten, TakeoverGoneAtFinish, AbortBeforeThaw, SeamNilInRelease), FreezeGate, OneAtATime, SIGTERMAfterCommitKeepsSocket, TakeoverHoldsInputUntilOldExits, `TestTakeover_FinalizeDeniesRelayPort`, and every `TestEnsureDaemon_*` (HandsOff, HandoffFailsToDebt08, V11GetsPlainDebt08, SlowCommitNoQuestion, CancelNeverKillsTakeover, CancelUnblocksHungDaemon …) | ok (38 s) |
| `./internal/relay/` | `-run 'Handoff\|Freeze\|Thaw\|Restart\|AddStaged'`: 26 tests, including RoundTripCorpus, `TestHub_HandoffRejoinKeepsGrid`, `TestHub_HandoffFrozenClaimAppliedOnThaw`, CwdRoundTrip, FitsSessionFrameCap and `TestRestartViewers_Close1012` | ok |
| `./internal/fdpass/`, `./internal/pty/` | whole packages (small: fdpass, adopt, exit watch, native) | ok |
| `.` (root), `./internal/attach/` | `-run 'TestEnsure_DefaultSeamPassesRestartConsent\|TestQuitAll\|TestRestartConsent_MoveFailed\|TestOnTerminate_QuitsGUIOnly\|TestDaemonUpdateNeeded\|TestEnsureCLIDaemon\|TestTakeover\|TestRunAttach\|Reconnect'` | ok |
| vitest | `hostReconnect.test.ts`, `TerminalPanel.reconnect.test.tsx`, `App.daemon-restart.test.tsx`, `App.update-needed.test.tsx`, `QuitConfirmModal.test.tsx` | 5 files, 61 tests passed |
| build / vet | `go build ./...`; `go vet ./internal/daemon/ ./internal/relay/ .` | rc 0 |
| traceability | `bash tests/check-traceability-paths.sh` | `OK: 228 traceability paths exist` |

**Code wiring confirmed at HEAD:**
- The handoff branch in EnsureDaemon (process.go:275, `health.Handoff == HandoffVersion`, skipped for a v1.1 daemon and for `NoHandoff`).
- `/health` with `API: APIVersion, Handoff: HandoffVersion` (api.go:384).
- `MinDaemonAPI = 1` (client.go:97).
- The CLI couldn't-move line and the update-needed check (cmd_cli.go:133, 160).
- `signal.Reset(SIGTERM)` and `onTerminate` (app.go:277, 661).
- `checkDaemonAPI` and the `daemon:update-needed` event (app.go:368–381).
- `cancelEnsure()` in QuitAll (app.go:723).
- App.tsx: the quit request answers Not Now while the restart dialog is open (App.tsx:510), the Quit modal renders only when `restartPrompt === 0` (App.tsx:1496), and the update-needed banner is at App.tsx:1201.
- The 1012 restart close in both the relay and the share server (relay/server.go:101, webserver/server.go:961).

## Supersessions by later phases (judged against the intended current behaviour)

| Phase 9 item | Later change | Judgement |
|---|---|---|
| SC2: the `50903bee` 3 s settle window in hub.go (`applyGridLocked`/`endSettle`) and its tests `TestHub_HandoffSettleKeepsGrid` / `SkipsFrozen` | Phase 10-04 (`b3836aa9`) dropped the settle window when INPT-06's grid owner replaced min-among-local. A rejoining viewer no longer bumps `geomEnd`, because only a claim takes the grid | Still satisfied. The tests are now `TestHub_HandoffRejoinKeepsGrid` and `TestHub_HandoffFrozenClaimAppliedOnThaw` (both pass). 10-VM-CHECKS **V-02r PASS ×2** re-ran Phase 9 V-02 without the settle window: all 5 viewers back in about 1.0–1.6 s, 5/5 `mode=delta`, 0 duplicate or missing ticks. **RF-02 PASS** after the Phase 10 review fixes repeats it with a typist on s3. The only loss was keystrokes typed while `portty attach` was disconnected (about 0.95 s), which the row allows when reported. `9cda1853` (a delta resume exactly at `geomEnd`) closed the related R-05 full-replay case |
| SC1/SC2: input during the handoff | Phase 10 added the hub input FIFO. The handoff waits for queued input (`e8b3331a`), and the takeover writes no input until the old daemon exits (`1857e271`, WR-02) | Strengthens UPGR-01/02. `TestHandoff_TakeoverHoldsInputUntilOldExits` and `TestHandoff_FinishWaitsForInput` pass |
| SC4: `APIVersion = 2` | Bumped to 3 (preview, Phase 11), 4 (path click, Phase 12) and 5 (localhost forwards, Phase 13). `MinDaemonAPI` stays 1 | The intended design: the API version is the compatibility channel Phase 9 built. "Update needed" still triggers only below `MinDaemonAPI`, and newer features check `PreviewAPI`/`PathClickAPI` per feature. An older compatible daemon kept with Not Now still works |
| SC1: handoff state | Phase 12-03 carries the cwd log across handoffs, and Phase 13-02 hands over or ends localhost forwards. Phase 13 CR-01 (`32a2b551`) makes a takeover daemon deny the relay port (`TestTakeover_FinalizeDeniesRelayPort`) | Extends the handoff. The Phase 9 contract holds (`TestHandoff_HappyPath`, `TestHandoffState_RoundTripCorpus`, `CwdRoundTrip`, `CwdOldShape`) |
| SC1: "the share link still works with the same token and still refuses a wrong one" | Phase 14-01/14-03 added a same-Tailscale-account gate (`requireSameUser`/WhoIs) on the share server routes, the discovery probe, `/app/` and the WebSocket. The token check is unchanged | An intended tightening (a user decision in Phase 14). After an upgrade the share listener and records still move (`TestHandoff_HappyPath`). Access now needs the same token **and** the same account. The Phase 9 VM rows ran before the gate. Phase 14's own verification covers the gate |
| SC2: the browser viewer's reconnect path | Phase 14 reworked webserver/server.go (gates, `/app/`, the retry link) and the Phase 14 selection and lone-Esc fixes changed TerminalPanel.tsx | No change to the 1012 close or to the resume. `TestRestartViewers_Close1012`, hostReconnect.test.ts and TerminalPanel.reconnect.test.tsx pass at HEAD |

## Accepted limits and earlier decisions (carried forward; not gaps)

- **IN-04:** stopped sessions are dropped on handoff (09-CONTEXT.md:37; `shortcut:` on `handoffEntries`). No live shell is lost.
- **09-SECURITY.md** (status verified) accepted risks, all low and accepted at plan time:
  - AR-09-01 / T-09-04: the copy names versions only;
  - AR-09-02 / T-09-22: the COMMIT-to-COMMITTED window;
  - AR-09-03 / T-09-26: a takeover stopped by SIGSTOP after COMMIT, where the old side waits on `peerGone` with no timer.
- **09-VM-CHECKS.md:**
  - The V-02 FAIL at `bbb34439` was kept as history and fixed. V-05b's first FAIL (54.76 s) was fixed and passed.
  - V-03b/V-03d were re-run after CR-01 at `dc48674b`.
  - The V-03a first attempt was discarded (a check viewer used the B binary), and the row was re-run.
- **10-VM-CHECKS RF-02:** `portty attach` drops stdin typed during its roughly 1 s reconnect. The row allows this when reported. Nothing was reordered or doubled.

## Status rationale

All five success criteria still hold at HEAD.
- **Code:** the handoff core, rollback, thaw only after the takeover exits, the 1012 close plus delta resume, the API version with update-needed, and the SIGTERM, Quit All and one-modal quit paths are all present and wired.
- **Tests:** the targeted Phase 9 tests pass on the current code.
- **VM evidence:** the VM-only parts have PASS rows. The SC2 grid behaviour that later phases changed was re-proven in the VMs on the new design (10-VM-CHECKS V-02r ×2 and RF-02).

No human verification is required. A fresh VM re-run of V-01 under the Phase 14 account gate would only confirm the gate, and Phase 14's verification owns the gate.

---

_Re-verified: 2026-11-22_
_Verifier: Claude (gsd-verifier)_

## Original verification (2026-11-20, HEAD `dc827079`), kept for its evidence

_The line numbers, test names and the settle window below describe the code at that time. See the supersessions table above for what changed._

**Phase Goal:** Upgrading Portty never costs a session. The new daemon takes over the running shells and their history, every attached viewer carries on by itself, and if the takeover fails the old daemon keeps serving. Nobody has to be at the remote Mac.
**Verified:** 2026-11-20T22:30:16Z, at HEAD `dc827079`. The latest product commit is `50903bee`; everything after it is docs.
**Status:** passed
**Re-verification:** No. This is the first verification.

### How this was verified

- I read the production code for each criterion: handoff.go (the old side and the takeover), process.go (EnsureDaemon, handOff, serveDaemon's SIGTERM guard), api.go (the /handoff route, the freeze gate, /health), client.go and cmd_cli.go (API version and update needed), app.go (onTerminate, cancelEnsure, QuitAll, checkDaemonAPI), App.tsx (one modal, the Update needed banner), engine.go handoffEntries (the IN-04 shortcut), and the 50903bee settle-window diff in hub.go and handoff_state.go.
- I checked that the tests named for each criterion exist (grep for `func Test` / `it(`). Per the caller's instruction I ran no test binary.
- The full gate output for `50903bee` is in the scratchpad file `fix2-test.txt`. `go test -race -short ./...` passed all 13 packages; the file's mtime is 16:05, which matches the commit time of 16:05:06. Vitest last ran at `e7c6ab82`: 95 files and 1304 tests passed (`rf-gate-vitest.txt`). `git diff e7c6ab82 HEAD -- frontend` is empty, so that run still covers HEAD.
- I ran `GOTOOLCHAIN=go1.26.11 go build ./... && go vet ./...` myself at HEAD: rc 0. I also ran `bash tests/check-traceability-paths.sh`: `OK: 163 traceability paths exist`.
- I read 09-VM-CHECKS.md in full and set the commit each row ran at against the product diff since that commit.

### Goal Achievement

### Observable Truths (ROADMAP success criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC1 / UPGR-01: on the office VM, with two ticking sessions (one in vim) and a share link, installing B over A and launching it ends no session and asks nothing. `/health` reports B, the shell PIDs are kept, the tickers have no gap, a fresh viewer gets the pre-upgrade history, and the share link works with the same token and refuses a wrong one | ✓ VERIFIED | **Code:** the two-phase protocol in handoff.go (HELLO → GLOBAL+fds → SESSION+fds → READY digests → COMMIT → COMMITTED). The share records and share listener move in `global()`/`finalize()`. The old daemon exits through `finish()` with no session teardown. The EnsureDaemon branch (process.go:275) tries a handoff before the DEBT-08 question and returns without asking when the handoff commits. `/health` reports `api:2, handoff:1` (api.go:371). **Tests:** `TestHandoff_HappyPath` (sessions, rings, history, share records and listeners move; viewers resume with a delta), `TestEnsureDaemon_HandsOff`, `TestEnsureDaemon_SlowCommitNoQuestion`, the fdpass/adopt/round-trip/fuzz tests. **VM:** V-01 PASS at `bbb34439` (no dialog, same shell pids, 0 duplicates and no jump in TICK-s1 or the vim ticker, 300 HIST lines and `ESC[?1049h` in fresh viewers, share 302/200 with the same token and 401/401 with a changed one). V-02 PASS ×2 at `6ec55a2a`, which is the `50903bee` product code, repeats the upgrade with a shared s1 (same shells, `/health` B, gapless tickers, history and alt screen in fresh viewers, Safari on the same share link resuming with a delta). TestHandoffIT `upgrade` PASS at `bbb34439` |
| 2 | SC2 / UPGR-02: viewers attached during the upgrade (office local tab, home peer tab, browser share link, `portty attach` over the tailnet) reconnect on their own within seconds and resume with a delta, with no repeated or missing tick | ✓ VERIFIED | **Code:** `RestartViewers` sends 1012 (relay/server.go:98, webserver/server.go:874). hostReconnect.ts redials a host tab while the daemon still lists the session. The CLI local attach goes through `attach.Run`. The `50903bee` settle window keeps an imported hub's grid as a floor in the D-02 minimum for 3 s (hub.go `applyGridLocked`, `Start`, `endSettle`), so local viewers that rejoin one by one never bump `geomEnd`. The lock order is gridMu, then mu, the same as Freeze and Thaw. Shutdown stops the timer, and a frozen or closed hub is skipped. **Tests:** `TestHub_HandoffSettleKeepsGrid`, `TestHub_HandoffSettleSkipsFrozen`, `TestHandoffState_RoundTripCorpus` (resume deltas equal a reference hub), `TestRestartViewers_Close1012`, hostReconnect.test.ts (6), TerminalPanel.reconnect, attach loop tests. **VM:** V-02 FAIL at `bbb34439` (4 of 5 replays full, the home attach repeated ticks 0..77), fixed in `50903bee`, then V-02 PASS ×2 at `6ec55a2a`. In both runs all 5 viewers were back in 1.06–1.57 s, 5 of 5 resumes were `mode=delta`, the new daemon logged 0 grid lines, and every capture had 0 duplicates and no jump. Run 1 used the attach-first order that failed before; run 2 used the GUI-first order |
| 3 | SC3 / UPGR-03: a handoff that fails at any step loses nothing. Fault injection at each step in tests, plus a VM run that kills the new daemon mid-handoff, shows the old daemon serving every session and viewer, the GUI/CLI saying the sessions could not be moved, and the DEBT-08 choice offered. A v1.1 daemon gets the same DEBT-08 choice | ✓ VERIFIED | **Code:** `rollback` writes ABORT before any Thaw. After COMMIT the old daemon thaws only once the takeover pid has exited (`<-peerGone`, kqueue watch from HELLO). CR-01: the watchdog/COMMITTED atomic state machine (handoff.go:1216-1289), so a takeover never exits after a delivered COMMITTED. The old-dies rule is in `lost()`. `handOff` maps the outcome code to `moveFailedReason`. The CLI prints the couldn't-move line (cmd_cli.go:130), and the GUI dialog carries `MoveFailed`. A v1.1 daemon has no `handoff` in /health, so the handoff branch is skipped (process.go:275). **Tests:** `TestHandoffFault_EveryStep` (10 steps on both sides, with descriptor counts and contiguous ticks checked), `NoThawUntilTakeoverExits`, `CommittedWriteFailsOldAlive`, `OldDiesAfterStateSent`, `OldDiesMidState` (die and fail), `OldDiesAfterCommit`, `NewHangsAfterCommit`, `WatchdogAfterCommittedWritten`, `TakeoverGoneAtFinish`, `AbortBeforeThaw`, `TestEnsureDaemon_HandoffFailsToDebt08`, `V11GetsPlainDebt08`, `TestEnsureCLIDaemon_MoveFailedCopy`, the vitest "says the sessions couldn't be moved". **VM:** V-03a (GUI dialog after a kill -9 before READY), V-03b (CLI, a kill -9 after serve; the old daemon stayed frozen until the kill), V-03c (v1.1, plain DEBT-08 and Not Now keeps it) and V-03d (old daemon killed, B proceeds) all PASS at `8d5bcbb5`. TestHandoffIT `fault` (the takeover exits at new:before-ready, A keeps serving, couldn't-move plus the DEBT-08 refusal, rc 1) and `from-v1.1` PASS at `bbb34439` |
| 4 | SC4 / UPGR-04: `/health` reports an API version. A GUI or CLI facing a daemon it cannot talk to shows a clear "update needed" state (an app banner; a one-line CLI message and a non-zero exit). A compatible older daemon kept with Not Now keeps working | ✓ VERIFIED | **Code:** `APIVersion = 2` (types.go:62) in /health and on share /info. `DaemonAPI` reads an absent field as 1. `MinDaemonAPI = 1`, so a v1.1 daemon stays usable. The CLI check comes after ensure (cmd_cli.go:157), and the GUI check is `checkDaemonAPI` plus the `daemon:update-needed` event, which drives the App.tsx banner. **Tests:** `TestEnsureCLIDaemon_UpdateNeeded`, App.update-needed.test.tsx (3), api_test/client_test, `TestSessionInfo_ReportsAPIVersion`. **VM:** V-04 PASS at `8d5bcbb5` (the banner, plus a one-line CLI message with rc 1 under `PORTTY_VMPREVIEW_MIN_DAEMON_API=3`). V-03c PASS (v1.1 kept with Not Now; `--keep-daemon` works) |
| 5 | SC5 / UPGR-05: `kill -TERM` quits the GUI and leaves the daemon and sessions running. Quit All during an approved restart cancels it and quits within a few seconds. Cmd-Q while the restart dialog is open never draws a second dialog (a vitest plus a VM click check) | ✓ VERIFIED | **Code:** `signal.Reset(SIGTERM)` plus its own Notify, with `onTerminate` quitting only the GUI (app.go:276, 660). `QuitAll` calls `cancelEnsure()` before it takes daemonMu. The ensure client is ctx-aware (`WithContext(ctx)`, process.go:260). App.tsx answers Not Now on `app:quit-requested` while the restart dialog is open and renders the Quit modal only when `restartPrompt === 0`. The terminate_darwin.m hook lets a logout or restart go through. **Tests:** `TestOnTerminate_QuitsGUIOnly`, `TestQuitAll_CancelsRunningEnsure`, `TestQuitAll_HungDaemon`, `TestEnsureDaemon_CancelUnblocksHungDaemon`, `CancelNeverKillsTakeover`, the vitest "a quit request while the restart dialog is open answers Not Now and shows only the Quit dialog" and the hung-ListSessions test. **VM:** V-05b FAIL (54.76 s) at `8d5bcbb5`, fixed, then PASS at `72214c58` and again at `bbb34439` (Quit dialog in 2.3 s, GUI gone 3.03 s after Quit Everything, the only dialog shown). V-05a PASS at `8d5bcbb5`, and the same SIGTERM evidence again in V-01 at `bbb34439` (gone in 0.11 s, daemon alive). V-05c (exactly one dialog) and V-05d (real macOS restart not cancelled) PASS at `72214c58` |

**Score:** 5/5 truths verified (0 present, behavior-unverified)

### Is the evidence commit adequate? (the caller's question)

**Yes for the commits the caller named.** The only product change after `bbb34439` is `50903bee`: 47 lines in hub.go and 1 line in handoff_state.go, all in the relay grid arbiter, live only on hubs built by `NewHubFromState` once `Start` runs.

- **V-02 at `6ec55a2a`** runs exactly that code and is the row it changes. It also re-proves most of SC1 at HEAD: the same shells, `/health` B, gapless shell and vim tickers, history and the alt screen in fresh viewers, and the share link still serving Safari with a delta resume after the upgrade.
- **TestHandoffIT at `bbb34439`.** The `fault` subtest uses staged hubs that never `Start`, so the settle timer is never armed and that path is byte-for-byte unchanged. `from-v1.1` makes no takeover at all. In `upgrade`, the settle cap can only keep a grid from growing for 3 s. The same behaviour runs in V-02 at HEAD.
- **V-01 at `bbb34439`.** Covered at HEAD by V-02's upgrade, as above. Two parts of V-01 were not repeated at `6ec55a2a`, and neither is touched by the diff: the GUI-launch path (app.go) and the post-upgrade wrong-token 401 (webserver auth).
- **V-05b at `bbb34439`.** The quit path (app.go, App.tsx, client.go) is outside the diff.

**A wider staleness the caller did not ask about.** These rows predate review iterations 1–2, which changed the handoff core (CR-01 watchdog, WR-01 drain gate, WR-02/WR-03 digest and frame cap, WR-05 fd validation) and the Quit dialog's session read (WR-04):

- V-03a, V-03b, V-03d, V-04 and V-05a ran at `8d5bcbb5`;
- V-05c and V-05d ran at `72214c58`.

I judge this adequate but not ideal:

- **SC3's VM requirement holds after the review fixes.** TestHandoffIT `fault` at `bbb34439` is the new daemon dying mid-handoff with real processes in the VM: the old daemon keeps serving, the couldn't-move line prints and the DEBT-08 refusal follows.
- **The paths CR-01 rewrote are covered in-process at HEAD.** These are the post-serve kill (V-03b) and old-dies (V-03d) paths. `EveryStep` (new:after-serve-before-committed), `NoThawUntilTakeoverExits`, `WatchdogAfterCommittedWritten`, `TakeoverGoneAtFinish` and `OldDiesAfterStateSent` all passed in the `50903bee` gate.
- **Only one-modal Quit behaviour is VM-proven after WR-04.** V-05b at `bbb34439` showed the Quit dialog as the only dialog over an acting restart dialog. The vitest covers it at HEAD.

If you want VM proof of V-03b/V-03d on post-CR-01 code, re-run those two rows. They are optional; nothing blocks on them.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/daemon/handoff.go` | /handoff protocol, both sides, fault seam | ✓ VERIFIED | 1332 lines, substantive. The route is registered on the unix mux only (api.go:199; `TestHandoff_RouteOnlyOnUnixSocket`) |
| `internal/daemon/process.go` | `RunTakeover`, EnsureDaemon handoff branch, SIGTERM guard | ✓ VERIFIED | `handOff` decides from the outcome code only. `serveDaemon` waits up to `maxFrozen()` for a handoff's outcome on SIGTERM |
| `cmd_daemon.go` | `portty daemon --takeover` | ✓ VERIFIED | `runTakeoverFunc = daemon.RunTakeover` |
| `internal/fdpass/fdpass.go` | SCM_RIGHTS, peer uid/pid | ✓ VERIFIED | used by handoff.go `peerUID`/`peerPID`/`sendFDs`/`recvFDs` |
| `internal/pty/adopt_darwin.go`, `exitwatch_darwin.go` | adopt passed PTYs, kqueue exit | ✓ VERIFIED | `engine.importSession`, `watchPeerExit = pty.WatchExit` |
| `internal/relay/handoff_state.go`, `hub.go` | Freeze/Thaw, Export/Import, staged Start, settle window | ✓ VERIFIED | wired through `oldHandoff.sendSession` / `takeover` import / `commit` → `hub.Start()` |
| `internal/daemon/types.go`, `client.go`, `cmd_cli.go`, `app.go`, `App.tsx` | API version, update needed | ✓ VERIFIED | see SC4 |
| `app.go`, `terminate_darwin.m`, `App.tsx` | SIGTERM, Quit All cancel, one modal, logout | ✓ VERIFIED | see SC5 |
| `frontend/src/lib/hostReconnect.ts`, `internal/attach/loop.go`, `cmd_attach.go` | viewer reconnect | ✓ VERIFIED | used by TerminalPanel.tsx and `runAttach` |
| `internal/daemon/handoff_vmpreview.go`, `compat_vmpreview.go` | fault and min-API hooks, VM builds only | ✓ VERIFIED | `//go:build vmpreview`. No workflow or build.sh uses the tag. `TestHandoffFault_SeamNilInRelease` |

### Key Link Verification

| From | To | Via | Status |
|------|----|-----|--------|
| EnsureDaemon (GUI and CLI) | takeover process | `handOff` → `spawnTakeover` → `startDetachedTakeover` (status pipe fd 3) | WIRED |
| takeover | old daemon `/handoff` | `dial` POST with Upgrade, 101 read byte by byte | WIRED |
| takeover commit | relay hubs | `s.hub.Start()` + `watchSessionExit` | WIRED |
| old `finish` | viewers | `RestartViewers()` → 1012 → client redial | WIRED |
| failed handoff | DEBT-08 question | `moveFailed` → `RestartQuestion.MoveFailed` → CLI line / GUI copy | WIRED |
| `/health` api | CLI/GUI update needed | `DaemonAPI(h) < MinDaemonAPI` → error / `daemon:update-needed` event → banner | WIRED |
| QuitAll | running ensure | `cancelEnsure()` → ctx → `client.WithContext` | WIRED |

### Data-Flow Trace (Level 4)

| Artifact | Data | Source | Real | Status |
|----------|------|--------|------|--------|
| Update needed banner | `updateNeeded` | `GetDaemonUpdateNeeded()` / event, from `checkDaemonAPI`, which reads the live /health | Yes | ✓ FLOWING |
| Restart dialog copy | `MoveFailed`, count | `RestartQuestion` from `handOff` outcome and `liveSessions` | Yes | ✓ FLOWING |
| Takeover hubs | ring, history, seed, grid | SESSION frame `HubState` from `Export()` on the frozen old hub | Yes | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Whole module compiles and vets at HEAD | `GOTOOLCHAIN=go1.26.11 go build ./... && go vet ./...` | rc 0 | ✓ PASS |
| Traceability paths exist | `bash tests/check-traceability-paths.sh` | `OK: 163 traceability paths exist` | ✓ PASS |
| Go suite at `50903bee` | scratchpad `fix2-test.txt` (orchestrator gate; I did not re-run it, by instruction) | 13/13 packages `ok` | ✓ PASS (recorded) |
| Vitest | scratchpad `rf-gate-vitest.txt` at `e7c6ab82`; no frontend change since | 95 files, 1304 tests pass | ✓ PASS (recorded) |

### Probe Execution

None declared. The phase's runnable checks are the VM rows and TestHandoffIT, recorded in 09-VM-CHECKS.md.

### Requirements Coverage

| Requirement | Source Plans | Status | Evidence |
|-------------|-------------|--------|----------|
| UPGR-01 | 09-02, 09-03, 09-04, 09-05, 09-06, 09-07 | ✓ SATISFIED | SC1 |
| UPGR-02 | 09-03, 09-04, 09-06, 09-07, 09-07-FIX2 | ✓ SATISFIED | SC2 |
| UPGR-03 | 09-01 (copy), 09-04, 09-05, 09-06, 09-07 | ✓ SATISFIED | SC3 |
| UPGR-04 | 09-01, 09-02, 09-06, 09-07 | ✓ SATISFIED | SC4 |
| UPGR-05 | 09-01, 09-06, 09-07, 09-07-FIX | ✓ SATISFIED | SC5 |

No orphaned requirements: REQUIREMENTS.md maps exactly UPGR-01..05 to Phase 9. The checkboxes there are still `[ ]`; per the caller, phase.complete or requirements.mark-complete sets them, not this verifier.

**Accepted limit (not a gap):** IN-04. Stopped sessions are dropped on handoff. This is recorded in 09-CONTEXT.md:37, with a `shortcut:` comment on `handoffEntries` (engine.go:317). No shell is lost.

**TESTING.md (standing convention):**
- Every phase test file is in Section 2 and has a Section 4 UPGR row: handoff_test, handoff_fault_test, handoff_it_test, handoff_state_test, handoff_fuzz_test, fdpass_test, adopt_test, listener_test, native_test, server_test, process_test, cmd_daemon_test, cmd_cli_test, cmd_attach_test, loop_test, api_test, client_test, app_test, hostReconnect.test.ts, TerminalPanel.reconnect, App.update-needed, App.daemon-restart.
- Section 5 has M-105..M-115, and the traceability check passes.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (all phase files scanned) | — | TBD/FIXME/XXX/TODO/HACK/PLACEHOLDER | — | none found |
| TESTING.md | 1345, 1351, 1381, 1399 | stale `_Last run:_` lines | ⚠️ Warning (docs only) | Four lines miss the later runs (details below). The convention does not require last-run lines, so this does not block. A future reader of M-106 will not see that V-02 once failed and why |
| internal/daemon/handoff_fault_test.go | 65-69 | `EveryStep` has no plain-fail case for `old:after-state-sent` | ℹ️ Info | That step's rollback goes through the same `o.fault` → `rollback` path as the other old-side steps. `OldDiesAfterStateSent` covers the die case, and `OldDiesMidState` covers both die and fail for mid-state |

The four stale `_Last run:_` lines:
- M-105 still says `8d5bcbb5`. V-01 was re-run at `bbb34439`.
- M-106 still says `8d5bcbb5` PASS. It does not record the V-02 FAIL at `bbb34439` or the PASS ×2 at `6ec55a2a`.
- M-111 says `72214c58`. V-05b was re-run at `bbb34439`.
- M-114 says `a13f4928`. TestHandoffIT was re-run at `e7c6ab82` and `bbb34439`.

### Human Verification Required

None. Every VM-only part of SC1–SC5 has a recorded PASS row in 09-VM-CHECKS.md. The settle fix and the review fixes have the coverage judged adequate above.

### Gaps Summary

There are no gaps that block the goal. The code delivers all five criteria:
- a two-phase fd-passing handoff that never lets two processes read a PTY;
- rollback on every pre-COMMIT failure, with thaw after COMMIT only once the takeover has exited;
- 1012 plus delta resume for all four viewer kinds, with the settle window closing the V-02 race;
- an API version with update-needed in both the CLI and the GUI;
- SIGTERM, Quit All cancellation and one-modal quit.

Tests exist for each one, and the last full gate at `50903bee` is green. The VM evidence covers every VM-only part. The newest VM rows (`6ec55a2a`) run the final product code for the one criterion it changes.

Two non-blocking follow-ups:
1. Bring the four `_Last run:_` lines in TESTING.md (M-105, M-106, M-111, M-114) up to date.
2. Optionally, re-run V-03b and V-03d in the VMs on post-CR-01 code. In-process tests already cover these paths at HEAD.

---

_Verified: 2026-11-20T22:30:16Z_
_Verifier: Claude (gsd-verifier)_

### Addendum (2026-11-20, after this report; docs-only changes)

The covered docs changed after the verifier ran, with no product code change (latest product commit is still `50903bee`):
- 09-VM-CHECKS.md: V-03b PASS twice and V-03d PASS at `dc48674b`, which includes the CR-01 fix. This answers the report's optional follow-up 2: the post-CR-01 fault paths now have VM evidence. The TestHandoffIT re-runs at `e7c6ab82` and `bbb34439` (both PASS) are recorded in a table.
- TESTING.md: the M-105, M-106, M-107, M-111, M-114 and M-115 "Last run" lines now cite these re-runs (optional follow-up 1).

The verdict is unchanged: passed, 5/5. The digest above was recomputed over the same covered_files.

