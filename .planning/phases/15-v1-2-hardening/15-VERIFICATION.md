---
phase: 15-v1-2-hardening
verified: 2026-11-23T17:00:00Z
status: human_needed
score: 8/8 roadmap success criteria verified in code and tests (5 backstop clauses pending real-process runs)
covered_files:
  - ".planning/phases/15-v1-2-hardening/15-01-PLAN.md"
  - ".planning/phases/15-v1-2-hardening/15-01-SUMMARY.md"
  - ".planning/phases/15-v1-2-hardening/15-02-PLAN.md"
  - ".planning/phases/15-v1-2-hardening/15-02-SUMMARY.md"
  - ".planning/phases/15-v1-2-hardening/15-03-PLAN.md"
  - ".planning/phases/15-v1-2-hardening/15-03-SUMMARY.md"
  - ".planning/phases/15-v1-2-hardening/15-04-PLAN.md"
  - ".planning/phases/15-v1-2-hardening/15-04-SUMMARY.md"
  - ".planning/phases/15-v1-2-hardening/15-05-PLAN.md"
  - ".planning/phases/15-v1-2-hardening/15-05-SUMMARY.md"
  - ".planning/phases/15-v1-2-hardening/15-06-PLAN.md"
  - ".planning/phases/15-v1-2-hardening/15-06-SUMMARY.md"
  - ".planning/phases/15-v1-2-hardening/15-07-PLAN.md"
  - ".planning/phases/15-v1-2-hardening/15-07-SUMMARY.md"
  - ".planning/phases/15-v1-2-hardening/15-08-PLAN.md"
  - ".planning/phases/15-v1-2-hardening/15-08-SUMMARY.md"
  - ".planning/phases/15-v1-2-hardening/15-09-PLAN.md"
  - ".planning/phases/15-v1-2-hardening/15-09-SUMMARY.md"
  - "TESTING.md"
  - "app_update.go"
  - "app_update_test.go"
  - "cmd_attach.go"
  - "cmd_attach_resize.go"
  - "cmd_attach_test.go"
  - "frontend/src/App.tsx"
  - "frontend/src/components/TabBar.tsx"
  - "frontend/src/components/TerminalPanel.tsx"
  - "frontend/src/components/WebShareSessionView.tsx"
  - "frontend/src/components/__tests__/App.open-remote.test.tsx"
  - "frontend/src/components/__tests__/App.update.test.tsx"
  - "frontend/src/components/__tests__/TerminalPanel.guestClose.test.tsx"
  - "frontend/src/lib/__tests__/closeReasons.test.ts"
  - "frontend/src/lib/__tests__/pathLinks.linkifier.test.ts"
  - "frontend/src/lib/closeReasons.ts"
  - "frontend/src/lib/pathLinks.ts"
  - "internal/attach/attach.go"
  - "internal/attach/attach_test.go"
  - "internal/attach/attach_unix.go"
  - "internal/daemon/engine.go"
  - "internal/daemon/handoff.go"
  - "internal/daemon/handoff_test.go"
  - "internal/daemon/ipc.go"
  - "internal/daemon/socket.go"
  - "internal/daemon/socket_test.go"
  - "internal/fdpass/fdpass.go"
  - "internal/fdpass/fdpass_test.go"
  - "internal/pty/adopt_test.go"
  - "internal/pty/cleanup.go"
  - "internal/pty/native_test.go"
  - "internal/pty/reap_other.go"
  - "internal/pty/session.go"
  - "internal/relay/cwd_test.go"
  - "internal/relay/history.go"
  - "internal/relay/hub.go"
  - "internal/relay/hub_grid_owner_test.go"
  - "internal/relay/manager.go"
  - "internal/relay/pathclick_test.go"
  - "internal/statusbar/bar.go"
  - "internal/statusbar/bar_test.go"
  - "internal/tailnet/sessions.go"
  - "internal/tailnet/tailnet_test.go"
  - "internal/webserver/capability_helpers_test.go"
  - "internal/webserver/input_queue_test.go"
  - "tray.go"
  - "tray_objc_darwin.m"
covered_digest: "v4:sha256:eaf864e6be0c4615b2fd0ad8deeb98d27d65717674169ce785143b0fd93653cc"
behavior_unverified: 0
overrides_applied: 0
human_verification:
  - test: "M-152: click the 'Portty X is available' notification on a notarized build in Tart VM portty-test; then dismiss the banner, quit, relaunch, click the notification left in Notification Center"
    expected: "Window comes forward with 'Portty X is available.' and Download; no download starts until Download is pressed; the post-relaunch click also works (covers review IN-05, delegate registered after launch)"
    why_human: "Real UNUserNotificationCenter delegate on a notarized bundle; ad-hoc builds cannot post notifications. Backstop truth (15-06)."
  - test: "M-153: attach to a session from a real terminal, run vim then Claude Code, resize, open the same session in a taller GUI tab; record the passive-watch case"
    expected: "Nothing draws on the bar row once the CLI has typed or resized; `tput lines` = terminal rows - 1 after typing in the CLI; --status-top overdraw is the accepted limit"
    why_human: "Real terminal emulator and full-screen apps."
  - test: "M-154: over the two-VM tailnet, end a shared session, switch away and back to the ended peer tab; sample the sidebar for 40 s after the last share ends; then pf-drop the host"
    expected: "Ended tab reads 'Session ended', never 'stopped sharing'; host row reads 'Nothing shared', never 'Can't reach this Mac'; with pf dropping, the can't-reach line shows"
    why_human: "Needs a real TCP RST from a closed share port versus a timeout over the real tailnet (the code relies on errors.Is(err, syscall.ECONNREFUSED) from the system dialer)."
  - test: "M-155: TestHandoffIT (upgrade, fault, from-v1.1) in portty-test, plus the pre-phase daemon upgraded twice (socket 755, then 600, then 600; no 'does not match the socket peer' in daemon.log)"
    expected: "All pass; B's HELLO-OK pid check accepts a genuine old daemon across real processes, including on a second upgrade where the listener was created by another process; a killed takeover leaves A serving; socket tightened to 0600"
    why_human: "Two real daemons from two builds must not run on the host. A false refusal would turn every upgrade into the DEBT-08 question, so this is a ship blocker per TESTING.md. The dialing-side LOCAL_PEERPID result rests on one research probe (15-RESEARCH Item D); no automated test pins it."
---

# Phase 15: v1.2 Hardening Verification Report

**Phase Goal:** Close the bugs and test flakes found during v1.2 so the milestone ships clean (source: pending todos; dompurify-3418 and v12-release-checks out of scope).
**Verified:** 2026-11-23
**Status:** human_needed
**Re-verification:** No, initial verification

All eight roadmap success criteria are implemented, wired, and covered by tests that I ran myself. Nothing failed. The status is `human_needed` only because the plans themselves declare four live checks (TESTING.md M-152..M-155) that cannot run on this host, and those cover real-process or real-OS behaviour that the in-process tests do not.

## Goal Achievement

### Observable Truths (ROADMAP success criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `portty attach` with its bar shown sends rows-1 initially and on every resize; claiming or typing viewer's reduced height wins (amended scope); a test pins it | VERIFIED | `attach.BarRows` (internal/attach/attach.go:178) subtracts only when `reserve > 0 && rows > reserve`. `initialResizeFrame` (cmd_attach.go:463) is used in `onConnect`, and `bar.ReservedRows` (nil-safe, bar.go:130) is passed to `watchResize`. `attach_unix.go:41` applies `BarRows` before `MakeClientClaimFrame`. Pinned by `TestBarRows`, `TestWatchResize_ClaimsBarRows`, `TestInitialResizeFrame`, `TestBar_ReservedRows`, `TestHub_GridAttachBarRows` (CLI report leaves 120x40; CLI claim or typing gives 80x23; GUI claim takes it back). All pass under `-race`. The passive-watch case keeps today's behaviour per the user's 2026-11-23 decision; M-153 observes it. |
| 2 | Clicking an already-ended peer tab shows "session ended", not "stopped sharing" | VERIFIED (live sidebar and tailnet clause pending M-154) | `peerConnectVerdict(errorCode, sawEnded)` (closeReasons.ts:29) maps `no_such_session` with `sawEnded` to `ended`. `markPeerSessionEnded` sets `Tab.sessionEnded`, keyed by host plus session (App.tsx:1053). `reconnectPeerTab` passes the flag (App.tsx:1030, 1363). `not_sharing` and `forbidden` stay share-stopped. Vitest passes (closeReasons.test.ts, App.open-remote.test.tsx, TerminalPanel.guestClose). The sidebar half is fixed in 15-07: `MetaRefused` via `errors.Is(err, syscall.ECONNREFUSED)` is reported Reachable with no sessions (sessions.go:84, 236); timeout stays unreachable and no IP retry follows a refusal. `TestMetaStatus_TriState` and `TestFetchAllPeerSessionsMeta_RefusedIsReachableEmpty` pass. The client uses a plain `http.Transport` (system dialer), so a real ECONNREFUSED is plausible. |
| 3 | Dismissing a naturally exited session never signals a stale pgid; the other todo items are resolved | VERIFIED | `Session.reaped` is set after every `cmd.Wait` (reap_other.go:37, cleanup.go:72). `killSession` returns before `signalGroup` when reaped (cleanup.go:40). `ReapNaturalExit` is called from engine.go:298. Items 2 and 3 are doc comments on `HubManager.Shutdown` and `closeSessionFiles`, with no caller today. `TestKillSession_ReapedSkipsGroupSignal` and the kill tests pass. |
| 4 | Phase 9 socket-hardening items (O-1, O-2, O-6) done or recorded as accepted | VERIFIED (real-process clause pending M-155) | O-2: `listenDaemonSocket` does Listen then `os.Chmod(0600)`, closes the listener and fails the start on error (ipc.go). `tightenSocketMode` runs after takeover COMMIT and only logs on failure (handoff.go:1338). The config dir is tightened to 0700 (socket.go). O-6: `takeoverPeerPID(t.c)` is compared with HELLO-OK, and a mismatch or lookup error returns `takeoverRefused`, so the old daemon keeps serving. O-1: `rightsFromOOB` walks control messages one at a time and `Recv` closes every fd it returned. `TestListenDaemonSocket_Mode0600`, `TestHandoff_TightensSocketMode`, `TestHandoff_RefusesPIDMismatch` (mismatch and lookup-error cases, old daemon verified serving), `TestRightsFromOOB`, `TestRecv_TruncatedClosesFds` all pass. |
| 5 | Clicking "Portty X is available" brings Portty forward | VERIFIED in code (live click pending M-152) | `didReceiveNotificationResponse` on `PorttyNotificationDelegate` calls the cgo export `onNotificationClick` for the default action only, and always calls `completionHandler()` (tray_objc_darwin.m). `tray.go` converts the C string, then runs `go app.handleNotificationClick(id)`. The handler acts only on `portty.update-available`: it calls `showWindow` and emits `update:show`, and never downloads. App.tsx `EventsOn('update:show')` clears `dismissedUpdate`. The delegate is registered at launch from `initStatusItem`. `go build ./...` (cgo plus .m) succeeds. Go tests (`TestNotificationClick_*`) and App.update.test.tsx pass, including no download. |
| 6 | F-1: relative path on row 0 after `cd proj; clear` resolves against proj. F-2: first Cmd-click after leave and return to the same changed cell sends the path click | VERIFIED | F-1: `History.Skip(emu.ScrollbackLen())` in the hub's ED 3 handler (hub.go:1305) keeps rows monotonic. Mutation check: with that line commented out, `TestPathClick_RelativeAfterClear`, `TestCwd_ClearKeepsRowsMonotonic` and `TestCwd_ScrollbackEraseKeepsRows` all FAIL, so the tests have teeth. I restored the file. F-2: the `mouseleave` handler now calls `invalidate()` (pathLinks.ts:152). Mutation check: with it reverted, the real-xterm-Linkifier F-2 test FAILS, then passes after I restored it. `Hub.PathClick` and cwd.go are untouched. |
| 7 | Flaky tests are deterministic without loosening product behaviour; full Go and vitest suites pass once, clean | VERIFIED | `TestKillClean_NoOrphans` and `TestShare_PasteBacklogKeepsViewer` passed 10 of 10 under `-race` in my run. The linkifier test passed 10 of 10 in each of 5 consecutive vitest runs. Its waits are `vi.waitFor`; the one remaining `sleep(20)` sits in a negative assertion. No non-test file in internal/webserver changed, so the heartbeat values are untouched. `go vet ./...` and `gofmt -l` on the phase's Go files are clean, `tsc --noEmit` is clean, and the orchestrator's full `-race -short` Go run (14 of 14 ok) and 15-09's clean full vitest run (122 files, 1825 tests, no frontend change since) are accepted as supplied. |
| 8 | Each fixed todo moves to completed; TESTING.md updated per the standing rule | VERIFIED | Nine todos are in `.planning/todos/completed/`, each with a `## Resolution` (the pre-existing selection-offset todo has none). Only `dompurify-3418` and `v12-release-checks` remain in pending. TESTING.md has a Phase 15 Section 2 note and Category BF with M-152..M-155, each with rig, PASS criteria, "Last run: not yet run" and source. `bash tests/check-traceability-paths.sh` gives OK: 228 paths. |

**Score:** 8/8 criteria verified in code and tests; 0 behavior-unverified; 0 failed.

### Backstop truths (non-inferable, abstained pending real runs)

| Truth | Status | Where it is routed |
|-------|--------|--------------------|
| Live notification click on a notarized VM build (15-06) | insufficient_spec, human | M-152 |
| Pre-phase socket reads 0600 after two real-process upgrades (15-03) | insufficient_spec, human | M-155 |
| Pid check accepts a genuine old daemon across real processes, including a listener created by another process (15-08) | insufficient_spec, human | M-155 (ship blocker) |

### Required Artifacts

`verify.artifacts` for all nine plans passes: 1 plan has 5/5, 02 has 4/4, 03 has 4/4, 04 has 5/5, 05 has 3/3, 06 has 4/4, 07 has 2/2, 08 has 4/4 and 09 has 2/2. I read the implementations behind each and none is a stub.

### Key Link Verification

Plans 02 to 08 pass 15/15 under `verify.key-links`. The three tool-reported failures are pattern-syntax artefacts, not broken links:
- 15-01 `cmd_attach_resize.go` to attach_unix.go: the actual call is `attach.WatchResize(ctx, conn, reserve)`, present in cmd_attach_resize.go.
- 15-01 attach_unix.go to attach.go: the `BarRows(` pattern is not valid RE2. The call is present at attach_unix.go:41.
- 15-09 TESTING.md to the traceability script: the pattern is script output, not source text. I ran the script and got OK: 228.

### Data-Flow Trace (Level 4)

| Artifact | Data | Source | Real data | Status |
|----------|------|--------|-----------|--------|
| attach frames | rows - reserve | `term.GetSize` and `Bar.rows` (set by `Bar.Start`) | Yes | FLOWING |
| Tab.sessionEnded | ended flag | the guest's real 1000/"session ended" close | Yes | FLOWING |
| sidebar Reachable | status | real transport error classification | Yes | FLOWING |

### Behavioral Spot-Checks (single named tests, race on)

| Behavior | Result |
|----------|--------|
| attach, bar and hub grid-owner tests (SC1) | PASS |
| `TestKillSession`, `TestKillClean`, `TestKillAdopted` (SC3) | PASS |
| `TestShare_PasteBacklogKeepsViewer`, `TestShare_DeadViewerReapedBacklogDelivered` (3x), `TestKillClean_NoOrphans` (10x) | PASS |
| socket mode, tighten, pid mismatch, fdpass (SC4) | PASS |
| `TestNotificationClick_*` (SC5) | PASS |
| path click, cwd and history tests in internal/relay (SC6) | PASS |
| tailnet meta tri-state (SC2) | PASS |
| vitest: closeReasons, linkifier, App.update, TerminalPanel.pathClick | 60 of 60 PASS |

Mutation checks on F-1 and F-2 are reported under SC6. Both working-tree files were restored with `git checkout`; `git status` shows no source changes.

### Probe Execution

SKIPPED (no declared probe scripts).

### Requirements Coverage

No requirement IDs are declared: all nine plans have `requirements: []`, and REQUIREMENTS.md maps nothing to Phase 15. No requirement is orphaned.

### Anti-Patterns Found

No TBD, FIXME, XXX, TODO or HACK markers were added in non-test source files. No stubs found.

| File | Severity | Note |
|------|----------|------|
| 15-SECURITY.md, Trust Boundaries row 2 | Info | Describes the control socket as "PeerUID-checked". 9a277a74 established that it has no peer-UID check (only the handoff socket does). The doc is stale. The mitigation holds (0600 plus a 0700 parent), but the register text should be corrected. |
| repo root `agenthub` (27 MB untracked binary) | Info | Not phase output, untracked, outside any commit. Consider deleting or ignoring it before ship. |
| internal/daemon/ipc.go bind-to-chmod window | Info | Fenced only by the 0700 parent. A custom socket path in a looser directory gets no fence. Accepted in 15-REVIEW-DISPOSITION (WR-01 iter 3). |
| Open review infos IN-02, IN-03, IN-05, IN-06 | Info | IN-02 (duplicate `killSession` branches), IN-03 (ED 3 `Skip` counts main-screen scrollback during alt screen) and IN-06 are cosmetic or narrow; IN-05 (cold-launch click) is folded into M-152. None blocks the goal. |

### Human Verification Required

1. **M-152, update notification click.** On a notarized build in `portty-test`: click the notification, dismiss and relaunch, then click again. Expected: window forward, available banner, no download. Why human: the real UserNotifications delegate cannot be tested on an ad-hoc build.
2. **M-153, attach bar with real full-screen apps.** Run vim and Claude Code under `portty attach`, resize, and add a taller GUI tab. Expected: nothing paints the bar row after the CLI types or resizes. Why human: real terminal and apps. Also record the passive-watch case.
3. **M-154, ended peer tab and sidebar over the real tailnet.** Expected: "Session ended", then "Nothing shared", never "Can't reach this Mac" after a host share ends. Why human: needs a real TCP reset versus a timeout.
4. **M-155, real two-build upgrade chain (ship blocker).** TestHandoffIT upgrade, fault and from-v1.1, plus the pre-phase daemon upgraded twice. Why human: real daemons from two builds must not run on the host. Residual risk: if the dialing side's `LOCAL_PEERPID` reports the listener creator rather than the accepting process on a second upgrade, every upgrade would hit the DEBT-08 prompt. The research probe says it does not, but no automated test pins it.

### Gaps Summary

No gaps. Every roadmap success criterion is delivered in code that I read and in tests that I ran. Two of those tests I confirmed fail when the fix is removed. The phase is code-complete; ship waits on M-152..M-155 in the Tart VMs, with M-155 as the one the phase itself marks as a ship blocker.

---

_Verified: 2026-11-23_
_Verifier: Claude (gsd-verifier)_
