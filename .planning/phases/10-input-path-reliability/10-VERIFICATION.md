---
phase: 10-input-path-reliability
verified: 2026-11-23T04:08:14Z
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
  - "frontend/src/components/TerminalPanel.tsx"
  - "frontend/src/components/WebShareSessionView.tsx"
  - "frontend/src/lib/relayClient.ts"
  - "frontend/src/lib/tabs.ts"
  - "internal/attach/attach.go"
  - "internal/attach/attach_unix.go"
  - "internal/daemon/engine_exit_test.go"
  - "internal/daemon/handoff.go"
  - "internal/daemon/remote_ws_proxy.go"
  - "internal/relay/handoff_state.go"
  - "internal/relay/hub.go"
  - "internal/relay/input.go"
  - "internal/relay/oscabsorb.go"
  - "internal/relay/pong.go"
  - "internal/relay/protocol.go"
  - "internal/relay/server.go"
  - "internal/webserver/csp_mw.go"
  - "internal/webserver/server.go"
covered_digest: "v4:sha256:eff75a11a0d2f2f591bbe90fe9919167806da9a65a66fb058cd957d8f8de6e1c"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_verified: 2026-11-21T08:02:23Z
  previous_status: passed (after the 2026-11-21 addendum)
  reason: "stale fingerprint: Phases 11-14 changed covered files"
  head: 52d960a5
accepted:
  - item: "SC3 drop mechanism: pf `block drop` of the share port on utun4 instead of a literal `tailscale down`"
    accepted_by: user
    date: 2026-11-21
    record: "previous 10-VERIFICATION.md addendum (\"Accept both\"); 10-VM-CHECKS.md Deviation (SC3)"
  - item: "SC3 surface: the role=\"status\" SessionEndedBanner in the React browser viewer is the \"status bar\""
    accepted_by: user
    date: 2026-11-21
    record: "previous 10-VERIFICATION.md addendum (\"Accept both\")"
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
**Verified:** 2026-11-23T04:08:14Z at HEAD 52d960a5 (branch gsd/v1.2)
**Status:** passed
**Re-verification:** Yes. The previous report (2026-11-21, passed 6/6 after the addendum) went stale because Phases 11-14 changed covered files. This pass re-checks every SC and must_have against the current code.

## What changed since the last verification (4d9b78da..52d960a5)

About 45 later commits touch covered files: Phase 11 preview frames and tabs, Phase 12 OSC 7 cwd and path clicks, the 7a37bfb4 selection fix, Phase 13 port forwarding, and Phase 14 same-account gating, the key bar, pinch zoom and the lone-Esc change. The Phase 10 core is **byte-for-byte unchanged**: `internal/relay/input.go`, `internal/relay/pong.go`, `frontend/src/lib/tabs.ts` and `internal/daemon/engine_exit_test.go` have no diff. The grid owner functions in hub.go (`ClaimGrid`, `NoteTyping`, `sizeClient`, `applyGridLocked`) have no diff. The relayClient.ts diff (+108) touches no paste, ping, pong or claim line.

Intentional supersessions, judged against the intended current behaviour:

- **Lone Esc (14-02, 659aee3a):** `InputAbsorber.Filter` now forwards a trailing bare ESC at once instead of holding it. `IsUserInput` still treats a lone ESC as not typing, so an Esc keypress does not claim the grid. That is consistent with INPT-06's conservative classifier. `TestIsUserInput`, `FuzzIsUserInput` and the new `TestRelay_LoneEscReachesPTY` pass.
- **Pinch (14-11, bc25e326):** a font change still claims when shown, but a pinch claims once at its end (`TerminalPanel.tsx:1248`, `!pinchingRef.current`). That extends SC6's "zoom claims" to phones and does not weaken it.
- **Same-account gate (14-01/14-03):** the WebSocket, `/sessions/{id}` and discovery probe are now limited to the user's own Tailscale account. That adds a check in front of the read pump. The pump itself (OpenInput, owed pongs, claims) is unchanged.
- **CSP (11-06, 11-07, 14-05):** `csp_mw.go` gained `blob:` images, `worker-src 'self'` and `manifest-src 'self'` for preview and the Home Screen manifest. Each was reviewed in its own phase. SC3's "no CSP change" referred to the paste notice. The notice still needs no CSP change, and no product `innerHTML` was added (the only new `innerHTML` lines are in test files).

## Goal Achievement

| # | Truth | Status | Evidence at HEAD |
|---|-------|--------|------------------|
| 1 | SC1 / INPT-01: a long paste keeps every viewer connected through a drain of several heartbeat timeouts. The file matches, and a really dead viewer is still dropped | ✓ VERIFIED | Both read pumps use the hub FIFO: `relay/server.go:167,171,195` and `webserver/server.go:1025,1029,1062` (`OpenInput`, `defer in.Close()`, `in.Write`). There is no production `WriteInput` caller. input.go is unchanged. Ran: `TestHubInput_*`, `TestRelay_PasteBacklogKeepsPonging`, `TestShare_PasteBacklogKeepsViewer`, `TestShare_DeadViewerReapedBacklogDelivered`, `TestRemoteWSProxy_PasteBackpressureStaysOpen` (x5) all pass. VM evidence R-01a-c, R-05 and R-03 (10-VM-CHECKS.md) stands, because the code it exercised is unchanged |
| 2 | SC2 / INPT-02: a fully arrived paste shows no notice even with a full outgoing queue at the ping, and a real drop shows it | ✓ VERIFIED | `pongs.Owe()` on MsgPing in both pumps (`relay/server.go:217`, `webserver/server.go:1105`). `WritePump` in both (`:223`, `:1123`). `relayClient.ts:461-462` clears `paste` at `pongsReceived >= ackPing`. Ran: `TestWritePump_*`, `TestPongOwed`, and relayClient.test.ts "the ack pong arriving after a burst of output frames still clears the paste: no notice" pass. R-02 VM evidence stands (pong.go is unchanged and so are the paste lines in relayClient) |
| 3 | SC3 / INPT-03: a browser viewer whose socket drops mid-paste shows the same notice, with no innerHTML and no CSP change for it | ✓ VERIFIED (deviations accepted by the user 2026-11-21) | Ran: TerminalPanel.reconnect.test.tsx "browser view (wsURL) … Paste may have been cut short (98 KB) in role=status" and "Paste interrupted: 31 of 98 KB sent in role=status" pass (17/17). R-04 in real Safari stands. Accepted: pf drop in place of `tailscale down`, and the role="status" banner as the "status bar" (see `accepted` in the frontmatter) |
| 4 | SC4 / INPT-04: after Retry, no snapshot of a removed peer tab is left. Two Macs with one session id give two tabs, each rejoining with its own delta | ✓ VERIFIED | tabs.ts is unchanged (`webSessionTabId:50`, `peerTabsRemoved:65`). `App.tsx:1093-1096`: Retry drops removed peer tabs' snapshots, and `:625` does the same on close. Ran: tabs.test.ts, App.open-remote "two peers with the same session id (INPT-04)", TerminalPanel.tabSwitch "resume=104 … resume=204", App.daemon-recovered and Sidebar.test all pass |
| 5 | SC5 / INPT-05: the exit-code test passes under repeated full-package -race runs, and the full -race suite passes | ✓ VERIFIED | engine_exit_test.go is unchanged (`testutil.WaitFor`, no `SetState` forcing). Ran here: `-race -count=5 -run TestExitEvent_ListSessions_ExitCodePopulatedForStopped` gave 5/5 PASS. The full-package `-race -count=20` run approved by the user (2026-11-21, `ok … 1954.929s`, 0 DATA RACE) is still the record. It was not re-run, because this re-verification is limited to targeted tests while other verifiers run. The test and engine.go it exercises have not changed since |
| 6 | SC6 / INPT-06: the viewer you type on, zoom or resize sets the size, in a peer tab, a browser and `portty attach` | ✓ VERIFIED | `NoteTyping` on `IsUserInput` and `ClaimGrid` on the claim bit in both pumps (`relay/server.go:188-202`, `webserver/server.go:1055-1092`). attach SIGWINCH sends `MakeClientClaimFrame` (`attach_unix.go:38`). `TerminalPanel` window-resize claim is focus- and visibility-gated (`:1212`, WR-03), and so is the font-change claim (`:1248`, pinch-aware). Per-tab peer font size: `App.tsx:1346-1347`. Ran: 36 matched relay tests (grid owner, claim, `TestParseResize`, `TestIsUserInput`, `TestHub_HandoffRejoinKeepsGrid`), `TestReplayFidelity_ResumeAtResizeIsDelta`, internal/attach (incl. `TestWatchResize_SendsClaim`), `TestCLIAttach_TypingTakesGrid` (x5), TerminalPanel.scale (focused claims, unfocused only reports) and TerminalPanel.pinch all pass. VM R-06a-f and WR3-01 stand (grid logic is unchanged) |

**Score:** 6/6

## Targeted test runs (this pass, GOTOOLCHAIN=go1.26.11, node v24.22.1)

| Command | Result |
|---------|--------|
| `go build ./... && go vet ./internal/{relay,webserver,attach,daemon}/` | rc 0 |
| `go test -race -run 'TestHubInput_\|TestRelay_PasteBacklogKeepsPonging\|TestWritePump_\|TestPongOwed\|TestHub_.*Grid\|TestParseResize\|TestIsUserInput\|Claim' ./internal/relay/` (36 tests) | ok |
| `go test -race -run TestReplayFidelity_ResumeAtResizeIsDelta ./internal/relay/` | PASS |
| `go test -race -run 'TestShare_PasteBacklogKeepsViewer\|TestShare_DeadViewerReapedBacklogDelivered\|TestRelay_LoneEscReachesPTY' ./internal/webserver/` | ok |
| `go test -race ./internal/attach/` | ok |
| `go test -race -count=5 -run 'TestExitEvent_ListSessions_ExitCodePopulatedForStopped\|TestRemoteWSProxy_PasteBackpressureStaysOpen\|TestRemoteWSProxy_SessionEndedMidPaste\|TestHandoff_TakeoverHoldsInputUntilOldExits\|TestCLIAttach_TypingTakesGrid' ./internal/daemon/` | 25/25 PASS, ok |
| vitest relayClient, tabs, Sidebar, App.open-remote, App.daemon-recovered, WebShareSessionView, TerminalPanel.{pinch,tabSwitch,scale} | 9 files, 325 passed |
| vitest TerminalPanel.reconnect | 17 passed |
| `bash tests/check-traceability-paths.sh` | OK: 228 traceability paths exist |

No full suites were run, and nothing touched the host's tailscaled, a 100.x address, the host GUI or the Tart VMs.

## Requirements Coverage

| Requirement | Status | Evidence |
|-------------|--------|----------|
| INPT-01 | ✓ SATISFIED | SC1 |
| INPT-02 | ✓ SATISFIED | SC2 |
| INPT-03 | ✓ SATISFIED (accepted deviations) | SC3 |
| INPT-04 | ✓ SATISFIED | SC4 |
| INPT-05 | ✓ SATISFIED | SC5 |
| INPT-06 | ✓ SATISFIED | SC6 |

## Accepted, not gaps

These are carried forward from the earlier VERIFICATION.md, 10-SECURITY.md and 10-VM-CHECKS.md:

- SC3 pf drop and role="status" surface. The user accepted both on 2026-11-21.
- RF-01's 300 KB payload.
- AR-10-01..05.
- The CONTEXT `shortcut:` limits.
- The R-05/RF-03 FAIL rows kept above their PASS re-runs (Phase 9 convention).
- The RF-03 informational follow-ups ("stopped sharing" after a remount of an ended tab, a brief unreachable sidebar).

## Gaps Summary

There are no gaps. The Phase 10 input path (FIFO, owed pongs, paste ack), the host-keyed peer tabs, the hermetic exit test and the grid owner model are all unchanged at HEAD, and their targeted tests pass. The later phases built on that code without regressing it. The lone-Esc forward (14-02), pinch claim (14-11), same-account gate (14-01/03) and CSP additions (11, 14) are intentional supersessions and are consistent with the Phase 10 must_haves. The VM rows were not re-run, because the code they exercised has not changed.

---

_Re-verified: 2026-11-23T04:08:14Z_
_Verifier: Claude (gsd-verifier)_
