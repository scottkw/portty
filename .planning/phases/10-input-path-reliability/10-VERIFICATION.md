---
phase: 10-input-path-reliability
verified: 2026-11-23T19:30:00Z
status: passed
score: 6/6 must-haves verified
covered_files:
  - ".planning/phases/10-input-path-reliability/10-01-PLAN.md"
  - ".planning/phases/10-input-path-reliability/10-01-SUMMARY.md"
  - ".planning/phases/10-input-path-reliability/10-02-PLAN.md"
  - ".planning/phases/10-input-path-reliability/10-02-SUMMARY.md"
  - ".planning/phases/10-input-path-reliability/10-03-PLAN.md"
  - ".planning/phases/10-input-path-reliability/10-03-SUMMARY.md"
  - ".planning/phases/10-input-path-reliability/10-04-PLAN.md"
  - ".planning/phases/10-input-path-reliability/10-04-SUMMARY.md"
  - ".planning/phases/10-input-path-reliability/10-05-PLAN.md"
  - ".planning/phases/10-input-path-reliability/10-05-SUMMARY.md"
  - ".planning/phases/10-input-path-reliability/10-06-PLAN.md"
  - ".planning/phases/10-input-path-reliability/10-06-SUMMARY.md"
  - ".planning/phases/10-input-path-reliability/10-07-PLAN.md"
  - ".planning/phases/10-input-path-reliability/10-07-SUMMARY.md"
  - "TESTING.md"
  - "cmd_attach.go"
  - "frontend/src/App.tsx"
  - "frontend/src/components/Sidebar.tsx"
  - "frontend/src/components/TabBar.tsx"
  - "frontend/src/components/TerminalPanel.tsx"
  - "frontend/src/components/WebShareSessionView.tsx"
  - "frontend/src/lib/closeReasons.ts"
  - "frontend/src/lib/relayClient.ts"
  - "frontend/src/lib/tabs.ts"
  - "internal/attach/attach.go"
  - "internal/attach/attach_test.go"
  - "internal/attach/attach_unix.go"
  - "internal/daemon/engine_exit_test.go"
  - "internal/daemon/handoff.go"
  - "internal/daemon/remote_ws_proxy.go"
  - "internal/relay/handoff_state.go"
  - "internal/relay/hub.go"
  - "internal/relay/hub_grid_owner_test.go"
  - "internal/relay/input.go"
  - "internal/relay/oscabsorb.go"
  - "internal/relay/pong.go"
  - "internal/relay/protocol.go"
  - "internal/relay/server.go"
  - "internal/webserver/csp_mw.go"
  - "internal/webserver/input_queue_test.go"
  - "internal/webserver/server.go"
covered_digest: "v4:sha256:7e79e32771485ade4b7570d10877ce8a65d0a89b7d1b38b51a1baec7fbafa1af"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_verified: 2026-11-23T04:08:14Z
  previous_status: passed
  reason: "stale fingerprint: Phase 15 (v1.2 Hardening) changed covered files"
  head: c818e1e1
  prior_head: 52d960a5
accepted:
  - item: "SC3 drop mechanism: pf `block drop` of the share port on utun4 instead of a literal `tailscale down`"
    accepted_by: user
    date: 2026-11-21
    record: "earlier 10-VERIFICATION.md addendum (\"Accept both\"); 10-VM-CHECKS.md Deviation (SC3)"
  - item: "SC3 surface: the role=\"status\" SessionEndedBanner in the React browser viewer is the \"status bar\""
    accepted_by: user
    date: 2026-11-21
    record: "earlier 10-VERIFICATION.md addendum (\"Accept both\")"
  - item: "RF-01 payload of 300 KB instead of a one-frame paste (test-method choice, not tied to an SC)"
    accepted_by: user
    record: "10-VM-CHECKS.md"
  - item: "AR-10-01..AR-10-05 low-severity threats accepted at plan time"
    record: "10-SECURITY.md"
  - item: "CONTEXT accepted limits marked `shortcut:` (relaunched GUI shows the current grid, attach wider grid, paste over quota, upgrade during a slow paste, IN-01 frozen-claim loss, IN-08/WR-03 unfocused resize only reports)"
    record: "10-CONTEXT.md, 10-REVIEW.md"
---

# Phase 10: Input Path Reliability Re-Verification Report

**Phase Goal:** Pasting into a shared session is trustworthy everywhere. A long paste never gets a healthy viewer dropped, the "paste may have been cut short" notice shows exactly when a paste may really have been cut short (in the app and in the browser), and peer tabs never mix up two Macs.
**Verified:** 2026-11-23T19:30:00Z at HEAD c818e1e1 (branch gsd/v1.2)
**Status:** passed
**Re-verification:** Yes. The previous report (2026-11-23, passed 6/6, HEAD 52d960a5) went stale because Phase 15 changed covered files. Every SC and requirement was re-checked against the code at HEAD, with Phase 15's diff read line by line.

## What Phase 15 changed in covered files (52d960a5..HEAD)

`git diff 52d960a5..HEAD` over the covered set touches 17 files (309 insertions, 38 deletions). Read in full:

| Change | Files | Effect on Phase 10 truths |
|--------|-------|---------------------------|
| 15-02 test hardening of `TestShare_PasteBacklogKeepsViewer`: counts client-received pings through a `dialWebServerWSPing` hook, 5 s ceiling instead of a 500 ms timeout held for 3x | `internal/webserver/input_queue_test.go`, `capability_helpers_test.go` | Test only. `relay.Heartbeat` (heartbeat.go), `input.go`, `pong.go`, `relay/server.go`, `webserver/server.go` have **no diff**, so the product heartbeat is unchanged. See SC1 below for why the test still catches the INPT-01 bug |
| 15-05 ended peer tab: `Tab.sessionEnded`, `onSessionEnded`, `peerConnectVerdict(code, sawEnded)` | `App.tsx`, `TabBar.tsx`, `TerminalPanel.tsx`, `WebShareSessionView.tsx`, `closeReasons.ts` | `markPeerSessionEnded` matches on `peerSnapshotHost(t) === host && t.sessionId === sessionId`, so it stays host-aware. `tabs.ts` (`webSessionTabId`, `peerTabsRemoved`) has no diff. The Retry snapshot-drop at `App.tsx:1104-1109` is intact. No effect on the paste or ping path |
| 15-01 attach bar rows: SIGWINCH claim and first resize frame report `rows - reserve` (`BarRows`, never wraps) | `cmd_attach.go`, `attach.go`, `attach_unix.go`, `attach_test.go`, `hub_grid_owner_test.go` | Still a claim frame (`MakeClientClaimFrame`) on SIGWINCH, so SC6 "portty attach" holds. Only the height is one row less while the bar is shown. `TestHub_GridAttachBarRows` pins that a claiming or typing CLI's reduced height wins |
| 15-03, 15-08 handoff: socket mode tightened after COMMIT, HELLO-OK pid checked against the kernel's peer pid | `handoff.go` | Hardening in front of the takeover. Input hold (`TestHandoff_TakeoverHoldsInputUntilOldExits`) passes |
| 15-04 `History.Skip` and ED 3 handling | `history.go`, `hub.go` | Keeps absolute row numbering monotonic after a clear. `TestReplayFidelity_*` (delta resume) and the grid-owner tests pass |
| Doc/comment | `manager.go`, `TESTING.md` | None |

No Phase 15 change touches `input.go`, `pong.go`, `heartbeat.go`, `tabs.ts`, `engine_exit_test.go`, the paste and ping lines of `relayClient.ts`, or the grid-owner functions.

## Goal Achievement

| # | Truth | Status | Evidence at HEAD |
|---|-------|--------|------------------|
| 1 | SC1 / INPT-01: a long paste keeps every viewer connected through a drain of several heartbeat timeouts, and a really dead viewer is still dropped | ✓ VERIFIED | Both read pumps queue input through the hub FIFO: `relay/server.go:167` and `webserver/server.go:1025` (`hub.OpenInput(sub)`). There is no production `.WriteInput(` caller (grep over non-test Go files is empty). Ran: `TestHubInput_*`, `TestRelay_PasteBacklogKeepsPonging` (relay, -race), `TestShare_PasteBacklogKeepsViewer` (x3, -race, 1.3-1.5 s each), `TestShare_DeadViewerReapedBacklogDelivered`, `TestRemoteWSProxy_PasteBackpressureStaysOpen` (x3). All pass. **Phase 15's test edit does not weaken the assertion.** `relay.Heartbeat` pings synchronously (`c.Ping(pctx)` blocks until the pong, heartbeat.go:33) so the client's ping hook reaching `start+8` means at least 7 completed round trips while the paste sat behind the PTY. If the read pump blocked on the PTY write (the bug), no pong would be read, pings would stop at one, and the viewer would be reaped at the 5 s ceiling, so the test would fail on `RemoteViewerCount() != 1` or on the 30 s wait. The viewer-count check runs every poll. Product timeouts are unchanged. The dead-viewer test keeps its 100 ms heartbeat and still reaps |
| 2 | SC2 / INPT-02: a fully arrived paste shows no notice even with a full outgoing queue at the ping, and a real drop shows it | ✓ VERIFIED | `pongs.Owe()` on MsgPing in both pumps (`relay/server.go:217`, `webserver/server.go:1105`), `relay.WritePump` in both (`:223`, `:1123`). `relayClient.ts:461-462` clears `paste` at `pongsReceived >= ackPing`, with `ackPing = pingsSent` at `:677`. relayClient.ts has no Phase 15 diff. Ran: `TestWritePump_*`, `TestPongOwed` and the relayClient vitest file (in the 250-test vitest run) pass |
| 3 | SC3 / INPT-03: a browser share viewer whose socket drops mid-paste shows the same notice, with no innerHTML and no CSP change | ✓ VERIFIED (deviations accepted by the user 2026-11-21) | Ran: `TerminalPanel.reconnect.test.tsx` (browser view, `wsURL`, "Paste may have been cut short" in role=status) and `TerminalPanel.guestClose.test.tsx` pass. `csp_mw.go` has no Phase 15 diff. TerminalPanel's Phase 15 edit only adds the `onSessionEnded` callback on the session-ended close reason and does not touch the paste notice. R-04 (real Safari) stands, because the notice code path is unchanged |
| 4 | SC4 / INPT-04: after Retry, no snapshot of a removed peer tab is left. Two Macs with one session id give two tabs, each rejoining with its own delta | ✓ VERIFIED | `tabs.ts` is unchanged (`webSessionTabId` :50, `peerTabsRemoved` :65). `App.tsx:1104-1109`: `peerTabsRemoved(tabsRef.current, restoredTabs)` then `dropPeerSnapshot(host, t.sessionId)` for each. `webSessionTabId(sessionId, host)` at `App.tsx:901` and `:991`. Phase 15's `markPeerSessionEnded` keys on `peerSnapshotHost(t)` plus session id, so two Macs with one id are not mixed. Ran: `App.open-remote.test.tsx -t INPT-04` (4 passed), plus the whole file, `tabs.test.ts`, `Sidebar.test.tsx`, `App.daemon-recovered.test.tsx`, `TerminalPanel.tabSwitch.test.tsx` (resume deltas) pass |
| 5 | SC5 / INPT-05: the exit-code test is deterministic under repeated -race runs, and the full -race suite passes | ✓ VERIFIED | `engine_exit_test.go` has no diff. Ran: `-race -count=3 -run TestExitEvent_ListSessions_ExitCodePopulatedForStopped` plus the other daemon targets, all pass. The orchestrator's full `go test -race -short -count=1 ./...` gave 14/14 ok on HEAD. The `-count=20` full-package run approved by the user on 2026-11-21 (`ok … 1954.929s`, 0 DATA RACE) is still the record, and the test file is unchanged since |
| 6 | SC6 / INPT-06: the viewer you type on, zoom or resize sets the size, in a peer tab, a browser and `portty attach` | ✓ VERIFIED | Typing and claim paths in both pumps are unchanged (`relay/server.go`, `webserver/server.go` have no diff). attach SIGWINCH still sends `MakeClientClaimFrame` (`attach_unix.go:41`), now with `BarRows(rows, reserve())`; the first resize frame goes through `initialResizeFrame`. `BarRows` returns `rows` unchanged when `rows <= reserve`, so a 0- or 1-row terminal never wraps the uint16. `TerminalPanel.tsx` claim code (window resize, font change, pinch) has only the `onSessionEnded` addition. Ran: grid-owner, claim, `TestParseResize`, `TestIsUserInput`, `TestHub_HandoffRejoinKeepsGrid`, `TestHub_GridAttachBarRows` (relay, -race); `internal/attach` (-race, includes `TestWatchResize_*`, `TestBarRows`); `TestInitialResizeFrame` (root package); `TestCLIAttach_TypingTakesGrid` (daemon, x3); TerminalPanel.scale and TerminalPanel.pinch vitest. All pass |

**Score:** 6/6

## Targeted test runs (this pass, GOTOOLCHAIN=go1.26.11, `-count=1` unless noted)

| Command | Result |
|---------|--------|
| `go build ./... && go vet ./internal/{relay,webserver,attach,daemon}/` | rc 0 |
| `go test -race -run 'TestHubInput_\|TestRelay_PasteBacklogKeepsPonging\|TestWritePump_\|TestPongOwed\|TestHub_.*Grid\|TestParseResize\|TestIsUserInput\|Claim\|TestReplayFidelity\|TestHub_HandoffRejoinKeepsGrid\|TestCwd_\|TestPathClick' ./internal/relay/` | ok |
| `go test -race -count=3 -run 'TestShare_PasteBacklogKeepsViewer\|TestShare_DeadViewerReapedBacklogDelivered\|TestRelay_LoneEscReachesPTY' ./internal/webserver/` | ok |
| `go test -race -count=3 -run TestShare_PasteBacklogKeepsViewer -v ./internal/webserver/` | 3/3 PASS (1.28, 1.42, 1.51 s) |
| `go test -race ./internal/attach/` | ok |
| `go test -run 'TestInitialResizeFrame\|TestBarRows' . ./internal/attach/` | ok |
| `go test -race -count=3 -run 'TestExitEvent_ListSessions_ExitCodePopulatedForStopped\|TestRemoteWSProxy_PasteBackpressureStaysOpen\|TestRemoteWSProxy_SessionEndedMidPaste\|TestHandoff_TakeoverHoldsInputUntilOldExits\|TestCLIAttach_TypingTakesGrid\|TestHandoff_RefusesPIDMismatch\|TestHandoff_TightensSocketMode\|TestHandoff_HappyPath' ./internal/daemon/` | ok |
| vitest tabs, closeReasons, Sidebar, App.open-remote, App.daemon-recovered, WebShareSessionView, TerminalPanel.{pinch,tabSwitch,scale,reconnect,guestClose} | 11 files, 250 passed |
| vitest `src/lib/relayClient.test.ts` (path is `src/lib/`, not `__tests__/`; run separately) | 131 passed |
| vitest `App.open-remote.test.tsx -t INPT-04` | 4 passed |
| Orchestrator, not re-run here: `go test -race -short -count=1 ./...` and full vitest | 14/14 ok; 122 files, 1826 tests |

No full suites were run, and nothing touched the host's tailscaled, a 100.x address, the host GUI or the Tart VMs. No source was edited.

## Requirements Coverage

Requirement IDs from the Phase 10 PLAN frontmatter and the ROADMAP Phase 10 section: INPT-01 through INPT-06. All six are mapped to Phase 10 in REQUIREMENTS.md, and none is orphaned.

| Requirement | Status | Evidence |
|-------------|--------|----------|
| INPT-01 | ✓ SATISFIED | SC1 |
| INPT-02 | ✓ SATISFIED | SC2 |
| INPT-03 | ✓ SATISFIED (accepted deviations) | SC3 |
| INPT-04 | ✓ SATISFIED | SC4 |
| INPT-05 | ✓ SATISFIED | SC5 |
| INPT-06 | ✓ SATISFIED | SC6 |

## Human verification history (carried forward)

The VM runs recorded in `10-VM-CHECKS.md` and `TESTING.md` (R-01a-c, R-02, R-03, R-04 in real Safari, R-05, R-06a-f, WR3-01, and the Phase 9 V-02 re-run) stay as verified. Phase 15 did not change the behaviour those runs checked: the FIFO, the owed pongs, the paste-ack clearing, the notice, the host-keyed tabs and the grid-owner model have no diff. The one Phase 15 change near SC6 on the CLI (`rows - 1` under the attach bar) is a height adjustment on the same claim frames, and Phase 15's own M-153 run (v1.2.0-vm16, local and remote attach: `tput lines` = rows-1, grid log `why=input`/`why=claim`) shows the typing and resize claims still take the grid. Phase 15's M-154 run (ended peer tab) covers the peer-tab edit on the real tailnet.

No new human verification items.

## Accepted, not gaps

Carried forward from the earlier reports, 10-SECURITY.md and 10-VM-CHECKS.md:

- SC3 pf drop and role="status" surface (user, 2026-11-21).
- RF-01's 300 KB payload.
- AR-10-01..05.
- The CONTEXT `shortcut:` limits.
- The R-05/RF-03 FAIL rows kept above their PASS re-runs (Phase 9 convention).
- The RF-03 informational follow-ups.
- The `portty attach` passive-watch case noted in TESTING.md M-153 (a CLI that only watches a taller GUI-owned grid shows that grid) is the existing grid-owner behaviour, accepted in 10-CONTEXT.

## Anti-Patterns Found

Scanned the Phase 15 diff for the covered files. No `TBD`, `FIXME` or `XXX` marker, no stub, no empty handler. The new comments reference todo records by id. No `innerHTML` was added to product code.

## Gaps Summary

There are no gaps. Phase 15's edits to covered files are a test-only ping-counting change, a peer-tab ended-state flag, an attach bar-row adjustment, handoff hardening and history row bookkeeping. None of them touches the Phase 10 core (input FIFO, owed pongs, paste ack, host-keyed tab ids and `peerTabsRemoved`, the exit test, the grid-owner model), and the targeted tests for each truth pass at HEAD.

---

_Re-verified: 2026-11-23T19:30:00Z_
_Verifier: Claude (gsd-verifier)_
