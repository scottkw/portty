---
phase: 10-input-path-reliability
verified: 2026-11-21T08:02:23Z
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
  - "internal/webserver/server.go"
covered_digest: "v4:sha256:0ab638f1464c60f08a4ee01b16a8eae9585b8be2f587cf0f710b7c4a9ea5ecf0"
behavior_unverified: 0
overrides_applied: 0
human_verification:
  - test: "SC3 drop mechanism: read the 'Deviation (SC3)' paragraph in the 10-VM-CHECKS.md header and the R-00, R-03 and R-04 rows. SC3 says the browser notice is checked 'with `tailscale down` on the office VM during a paste'. The drop was a pf `block drop` of TCP 7444 (the office share listener) on utun4 in the anchor com.apple/portty-drop, which kept tailscaled up so the ephemeral node would not be deleted. Accept it, or ask for a literal `tailscale down` run of R-04."
    expected: "The user accepts the pf drop as equivalent for SC3 (R-00 shows a held TLS connection and new requests both stall with no RST, then recover on lift), or asks for a `tailscale down` re-run of R-04 using the plan's fallback procedure."
    why_human: "Only the user can accept a deviation from the roadmap's literal wording. The docs record it as 'For the user to accept' and nothing records that acceptance."
  - test: "SC3 notice surface: SC3 says the browser viewer shows the notice 'in its status bar'. The browser viewer is the React SPA (WebShareSessionView -> TerminalPanel). It has no status bar. The notice renders as a SessionEndedBanner inside a role=\"status\" element (10-03-PLAN must_have; the CONTEXT 'Decided after research' entry, INPT-03 target). R-04 read 'banner banner--info session-ended-banner: Paste may have been cut short (4883 KB)' from [role=status] in real Safari, with no CSP violation and no error."
    expected: "The user accepts the role=\"status\" banner as SC3's 'status bar'. This was an orchestrator decision recorded in the plans; the user has not yet accepted it."
    why_human: "It reinterprets roadmap wording, so it needs the product owner's decision."
  - test: "Recommended re-check, SC6 window-resize claim after WR-03 (fb90a8fa, after the R-06 VM rows at 35aaa035): in the VMs at HEAD, repeat R-06a's 'home window resize' step (System Events window size on the focused home GUI peer tab) and R-06b's WebDriver window-rect step on Safari. Then also resize an inactive home window."
    expected: "A resize of the key/focused window logs one `grid ... by=web conn=N why=claim` line and `tput cols` changes. A resize of an unfocused window only reports (no grid line), which is the documented WR-03/IN-08 shortcut."
    why_human: "WR-03 put a `document.visibilityState === 'visible' && document.hasFocus()` gate on the window-resize claim after the VM rows ran. Its vitest stubs hasFocus. No VM row ran the window-resize claim after the change, and focus in WKWebView and Safari under automation can only be seen on the real apps. RF-01..RF-03 did not touch resize."
  - test: "Optional re-run, SC5 at the final tree: `GOTOOLCHAIN=go1.26.11 go test -race -count=20 -timeout 60m ./internal/daemon/` (about 31 min; it fires a firewall alert, so it needs the user's go-ahead)."
    expected: "ok, 0 FAIL, 0 DATA RACE."
    why_human: "The passing -count=20 run (p10/p10-gate-daemon20-rerun2.log, 22:55 local) ran on the daemon tree of 3985dda6. Since then internal/daemon/handoff.go (WR-02) and remote_ws_proxy.go (RF-03) changed, and two tests were added: TestHandoff_TakeoverHoldsInputUntilOldExits and TestRemoteWSProxy_SessionEndedMidPaste. The named INPT-05 test and engine.go have not changed, so SC5's named-test claim still stands. The verifier may not run test binaries, so the repeat-run stability of the new tests is unproven."
  - test: "(harvested from 10-07-PLAN Task 2 human-check) Claude deploys the builds, proves the pf drop on the office VM, and runs the paste, dead-viewer, completed-paste and V-02 rows in the two Tart VMs over SSH, driving the GUI and Safari inside the VMs; no human is needed (user rule: drive GUI checks in the VM)."
    expected: "R-00, R-01a-c, R-05, R-02 and V-02r PASS with evidence."
    why_human: "A native Cmd+V paste into WKWebView and Safari, a real tailnet between two Macs, a live PTY behind a slow reader and a real two-build upgrade cannot run in CI. (Recorded: all PASS in 10-VM-CHECKS.md; R-05 FAIL then PASS after 9cda1853.)"
  - test: "(harvested from 10-07-PLAN Task 3 human-check) Claude runs the size hand-over rows (peer tab, Safari, attach, idle, relaunch, alternating typing) and the two scripted network drops (a pf block of the office share port; `tailscale down` only if R-00 chose the fallback) in the Tart VMs over SSH, then restores both VMs; no human is needed."
    expected: "R-06a-f, R-03 and R-04 PASS with evidence; VMs restored; TESTING.md Section 5 filled."
    why_human: "Native key events and window resizes in WKWebView and Safari, a real terminal's SIGWINCH, and a real network outage between two Macs cannot run in CI. (Recorded: all PASS in 10-VM-CHECKS.md second pass at 35aaa035.)"
---

# Phase 10: Input Path Reliability Verification Report

**Phase Goal:** Pasting into a shared session is trustworthy everywhere. A long paste never gets a healthy viewer dropped, the "paste may have been cut short" notice shows exactly when a paste may really have been cut short (in the app and in the browser), and peer tabs never mix up two Macs.
**Verified:** 2026-11-21T08:02:23Z (HEAD be60964c; product code equals 15632e48 / d69789c3, since only docs changed after that)
**Status:** human_needed
**Re-verification:** No (initial verification)

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria; PLAN must-haves merged, none reduce scope)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC1 / INPT-01: a multi-MB paste into a slow non-echoing reader keeps every viewer kind connected through a drain of several heartbeat timeouts, the file matches byte for byte, and a viewer whose network really dies is still dropped. Automated with a blocking fake hub, then M-101 on the VMs | ✓ VERIFIED | Code: `internal/relay/input.go` is one hub-owned FIFO with per-conn quotas (1,024 frames / 32 MiB), a 64 MiB hub cap with a 64 KiB per-conn allowance, and one writer goroutine that outlives the socket (`Close` keeps the backlog). Both read pumps call `hub.OpenInput(sub)` and `in.Write`: `internal/webserver/server.go:935` and `internal/relay/server.go:161`. `Hub.WriteInput`, which bypassed the FIFO, was removed (IN-03); `grep` finds no production caller. Tests: `TestShare_PasteBacklogKeepsViewer`, `TestShare_DeadViewerReapedBacklogDelivered`, `TestRelay_PasteBacklogKeepsPonging`, `TestHubInput_*` (9), `TestRemoteWSProxy_PasteBackpressureStaysOpen`. VM: R-01a/b/c PASS (4,000,000 B, drains of 116-118 s = 7.8x the 15 s timeout, sha256 and `cmp` equal, no leave/heartbeat timeout, no banner). R-05 PASS after 9cda1853 (reaped 43 s after STOP with `input backlog … bytes=4066747`, the backlog drained to 6,000,000 B `cmp`-equal, `mode=delta` on reconnect). R-03 PASS (M-101 re-run) |
| 2 | SC2 / INPT-02: a fully arrived paste shows no notice even when the outgoing queue is full as the ping arrives; a real drop still shows it | ✓ VERIFIED | Code: `internal/relay/pong.go` `PongOwed` (count, not flag). `WritePump` writes owed pongs before `sub.Msgs` (non-blocking pre-check, then a select). Both pumps call `pongs.Owe()` on `MsgPing`. `relayClient.ts:386-390` clears `paste` when `pongsReceived >= ackPing`. Tests: `TestWritePump_PongBeforeQueuedOutput` (full `sub.Msgs`), `TestWritePump_TwoPingsTwoPongs`, `TestPongOwed`, relayClient "the ack pong arriving after a burst of output frames still clears the paste: no notice". VM: R-02 PASS (2 x 5,000,000 B in an echoing `cat`, then unshare: only the "stopped sharing" banner, 0 `Paste` hits in AX across the run). R-03 and R-04 show the notice on a real drop. pong.go and relayClient.ts have not changed since the VM pass |
| 3 | SC3 / INPT-03: a browser share viewer whose socket drops mid-paste shows the same notice as the app "in its status bar", no innerHTML, no CSP change; jsdom harness plus `tailscale down` on the office VM | ? UNCERTAIN (needs user acceptance of two deviations) | The substance is delivered. `/sessions/{id}` redirects to `/app/` (`webserver/server.go:787-795`), so the live browser viewer is WebShareSessionView -> TerminalPanel -> RelayClient. terminal.js is unreachable for viewing (CONTEXT "Decided after research"). It was left untouched (no phase commit touches `web/`). vitest `browser view (wsURL): … in role=status` (2 cases) mounts WebShareSessionView and asserts the text inside `[role="status"]`. `git diff d2998ad2..HEAD` adds no innerHTML and does not change `csp_mw.go` or `web/`. VM R-04 PASS in real Safari: `[role=status]` reads "Paste may have been cut short (4883 KB)", CSP violations `[]`, errors `[]`, office file an exact prefix. **Not literal:** (a) the drop was a pf `block drop` of port 7444, not `tailscale down` (the header deviation is "For the user to accept", and no acceptance is recorded); (b) the surface is a role="status" banner, not a "status bar" (an orchestrator decision in the plans). Both are routed to Human Verification and are not gaps |
| 4 | SC4 / INPT-04: after a daemon error and Retry, no snapshot of a removed peer tab is left; two Macs that hand out the same session id open as two tabs, each rejoining with its own delta | ✓ VERIFIED | Code: `lib/tabs.ts` `webSessionTabId(id, host)` gives `__websession__host/id`, and `peerTabsRemoved`. `App.tsx:810` and `:893` key tabs by `peerSnapshotHost`/connectHost. `retryInit` (`App.tsx:995-1000`) drops snapshots and font sizes of the peer tabs it removes. Sidebar highlights rows by host. Tests: `tabs.test.ts` (webSessionTabId, peerTabsRemoved, hostlessWebSessionId), `App.open-remote.test.tsx` "two peers with the same session id (INPT-04)", `TerminalPanel.tabSwitch.test.tsx` "two hosts with one session id each rejoin with their own delta (resume=104 / resume=204)", `App.daemon-recovered.test.tsx` Retry drops the snapshot (`takePeerSnapshot` undefined), `Sidebar.test.tsx`. Automated only, as SC4 allows. vitest 95 files / 1338 passed at 01:06 (it2-gate-vitest.txt). Frontend is unchanged since 1d9cf77d (01:03), so that run covers HEAD's frontend |
| 5 | SC5 / INPT-05: `TestExitEvent_ListSessions_ExitCodePopulatedForStopped` passes in full-package `go test -race -count=20 ./internal/daemon/` runs, and the full `-race` suite passes with nothing skipped | ✓ VERIFIED | Code: `engine_exit_test.go:144-172` no longer calls `SetState`. It waits for the exit watcher with `testutil.WaitFor(3s)`, and the running case uses a real cat PTY. The flake reproduced 3/20 before the change (10-01). `p10/p10-gate-daemon20-rerun2.log`: `ok … internal/daemon 1853.588s` (rc=0) on the 3985dda6 daemon tree. The named test failed in none of the three -count=20 runs (runs 1-2 failed only in other tests' timing, fixed test-only). Full suite: `rf03-test-race-short.txt` (01:42, after d69789c3, product == HEAD) has every package ok. Skips under -short are the pre-existing env- or -short-gated tests (8 in the 10-07 -v log, including TestHandoffIT and TestRealDMG), which matches CONTEXT's locked gate `go test -race -short ./...` with no -skip. Caveat: -count=20 has not been re-run since the WR-02/RF-03 daemon changes. The named test and engine.go have not changed. An optional re-run is listed under Human Verification |
| 6 | SC6 / INPT-06: the viewer you type on sets the size. Cmd+ in a home peer tab reflows the office shell (`tput cols` changes), so does resizing the home window; typing in the office local tab takes it back; the same for a browser viewer and `portty attach` | ✓ VERIFIED | Code: claim bit `relay.ResizeClaim` (byte 5 of MsgResize2, `protocol.go:55-71`). `Hub.ClaimGrid`, `NoteTyping` and `sizeClient` (`hub.go:543-640`) implement the owner model: reports never move a known grid, a local bootstrap applies only on a never-sized grid, the clamp is 20..500 x 5..200, read-only, revoked and departed viewers are ignored (IN-09), and the owner fast path is lock-free. Both pumps call `NoteTyping` for `IsUserInput` (`oscabsorb.go:209`) and route claims to `ClaimGrid`. Attach: `attach_unix.go` sends `MakeClientClaimFrame` on SIGWINCH. Frontend: `TerminalPanel.claimFit` fires on a debounced window resize (focus- and visibility-gated since WR-03) and on a font change while shown. Per-tab peer font size: `App.tsx:1230-1231` passes `fontSizes[activeWebTab.id]` and `handleFontSizeChange(activeWebTab.id, …)` to WebShareSessionView, replacing the old `() => {}`. The Phase 9 settle window was removed and its test rewritten (`TestHub_HandoffRejoinKeepsGrid`). Tests: `hub_grid_owner_test.go` (18), `claim_test.go` (7), `TestParseResize`, `TestIsUserInput` and `FuzzIsUserInput`, `TestWatchResize_SendsClaim`, `TestCLIAttach_TypingTakesGrid`, `TestReplayFidelity_ResumeAtResizeIsDelta`, TerminalPanel.scale (incl. WR-03 unfocused only reports), App per-tab font size. VM (35aaa035): R-06a (Cmd+Shift+= gives 100x30 -> 80x27 -> 76x26 `why=claim`, winch COLS 80/76; window resize gives 60x24 `why=claim`; office typing gives 85x29 `why=input`), R-06b Safari (window rect, View>Zoom In, typing), R-06c home and office `portty attach` (SIGWINCH claim, typing), R-06d idle gives 0 lines in 62 s with vim queries, R-06e relaunch accepted limit, R-06f 10 alternations give exactly 10 grid lines. V-02r: 5/5 delta x 2 runs, 0 grid lines. Caveat: the window-resize claim was gated by WR-03 after these rows. A recommended VM re-check is in Human Verification |

**Score:** 5/6 truths verified (0 present, behavior-unverified). SC3 awaits the user's acceptance of two recorded deviations. Its behavior is proven in jsdom and in real Safari.

### Required Artifacts (gsd-tools verify.artifacts, all 7 plans)

| Plan | Artifacts | Key links | Status |
|------|-----------|-----------|--------|
| 10-01 | 1/1 | 1/1 | ✓ VERIFIED |
| 10-02 | 5/5 | 2/2 | ✓ VERIFIED |
| 10-03 | 5/5 | 3/3 | ✓ VERIFIED |
| 10-04 | 5/5 | 3/3 | ✓ VERIFIED |
| 10-05 | 4/4 | 2/2 | ✓ VERIFIED |
| 10-06 | 5/5 | 3/3 | ✓ VERIFIED |
| 10-07 | 2/2 | 3/3 | ✓ VERIFIED |

All exit 0. Manual wiring checks confirmed the important links: OpenInput in both pumps; PongOwed and WritePump in both pumps; ClaimGrid and NoteTyping from both pumps; the attach SIGWINCH claim; App per-tab fontSize into WebShareSessionView into TerminalPanel; `retryInit` into `peerTabsRemoved` into `dropPeerSnapshot`.

### Key Link Verification

| From | To | Via | Status |
|------|----|-----|--------|
| webserver/server.go read pump | relay/input.go FIFO | `hub.OpenInput(sub)`, `in.Write(filtered)`, `defer in.Close()` | WIRED |
| relay/server.go local pump | relay/input.go FIFO | same | WIRED |
| both pumps (MsgPing) | relay/pong.go WritePump | `pongs.Owe()` → pong written before `sub.Msgs` | WIRED |
| both pumps (MsgResize2 claim / typing) | hub.ClaimGrid / NoteTyping | `ParseResize` claim bit; `IsUserInput(filtered)` | WIRED |
| attach_unix.go SIGWINCH | relay claim | `MakeClientClaimFrame` | WIRED |
| TerminalPanel window resize / font change | relayClient.sendClaim | `claimFit()` | WIRED |
| App.tsx peer tab | WebShareSessionView.onFontSizeChange | `handleFontSizeChange(activeWebTab.id, delta)` | WIRED (desktop; the browser uses browser zoom, which fires a resize claim) |
| App.retryInit | peerSnapshots.dropPeerSnapshot | `peerTabsRemoved(prev, restored)` | WIRED |
| daemon handoff (WR-02) | Hub.HoldInput / ReleaseInput | takeover holds imported hubs until the old daemon's exit | WIRED |

### Data-Flow Trace (Level 4)

| Artifact | Data | Source | Real data | Status |
|----------|------|--------|-----------|--------|
| SessionEndedBanner paste notice | `pasteTotalKB` / sent | RelayClient `paste` record (frame sizes, `bufferedAmount`) at close | yes | ✓ FLOWING (R-03, R-04 readings) |
| Peer tab font size | `fontSizes[tab.id]` | `handleFontSizeChange` on Cmd+/Cmd− | yes | ✓ FLOWING (R-06a claim lines) |
| Grid broadcast (0x02) | `wantCols/wantRows` | owner's reported or claimed size | yes | ✓ FLOWING (R-06 `grid … why=` lines, `stty size`) |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Build and vet at HEAD | `GOTOOLCHAIN=go1.26.11 go build ./... && go vet ./...` | rc 0 | ✓ PASS |
| Traceability paths | `bash tests/check-traceability-paths.sh` | `OK: 174 traceability paths exist` | ✓ PASS |
| Full Go suite (latest, product == HEAD) | scratchpad `rf03-test-race-short.txt` (not re-run; firewall rule) | every package `ok` | ✓ PASS (recorded) |
| SC5 daemon -count=20 | scratchpad `p10/p10-gate-daemon20-rerun2.log` | `ok … 1853.588s`, rc=0 (daemon tree 3985dda6) | ✓ PASS (recorded; predates WR-02/RF-03) |
| vitest (frontend == HEAD) | scratchpad `it2-gate-vitest.txt` | 95 files, 1338 tests passed | ✓ PASS (recorded) |
| RF-03 regression test red on old code | scratchpad `rf03-new-test-oldcode.txt` / `rf03-new-test-fixed.txt` | FAIL with the VM's exact line, then ok | ✓ PASS (recorded) |

### Probe Execution

Step 7c: no `scripts/*/tests/probe-*.sh` are declared by the phase. Evidence comes from the VM rows and the gate logs above.

### VM Evidence vs Later Product Commits

| Evidence | Built at | What later product commits it covers | Not re-covered on VM |
|----------|----------|--------------------------------------|----------------------|
| R-00..R-06f, R-02, R-03, R-04, V-02r (second pass) | 35aaa035 | the core of all six SCs | every review fix (e3e2eb9b..4df30b42) and the RF-03 fix |
| RF-01 (CR-01 and CR-02 revoke drops queued web input; IN-09 is also in this build) | 9dd9eeb8 | CR-01, CR-02, WR-01/WR-05 indirectly (a revoke under a backlog), IN-09 build | RF-01 deviation: 300 KB payload instead of a one-frame paste, "For the user to accept" (not tied to an SC) |
| RF-02 (V-02 again with a typist across the upgrade) | 9dd9eeb8 | WR-02 takeover input hold (0 reorder or dup; gap 102..110 only while the CLI was disconnected); V-02 re-confirmed after the review fixes | an old-daemon backlog racing the takeover (unit test `TestHandoff_TakeoverHoldsInputUntilOldExits` only) |
| RF-03 re-run (2a-2d) | 15632e48 (product == HEAD) | WR-04 plus the RF-03 proxy fix plus the local attach ending; also re-shows at HEAD that a peer tab stays connected through 20-28 s of slow paste through the proxy | R-01a-c, R-05 and R-03/R-04 drops were not re-run after the RF-03 proxy change (the change only acts when the client-to-peer write errors, bounded by a 2 s grace) |
| none | — | WR-03 focus gate on the window-resize claim (fb90a8fa) | SC6 "resizing the home window" and the Safari window-rect/zoom claims (recommended re-check in Human Verification) |

Changes since 35aaa035 that don't affect the SC1-SC3 rows: pong.go and relayClient.ts have not changed. The per-conn quotas have not changed. The hub cap (64 MiB) is above every VM payload. IN-04/05/06 are frontend keying fixes covered by vitest and exercised by the RF peer-tab runs.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| INPT-01 | 10-03, 10-07 | Long paste never drops a healthy viewer; bounded input queue | ✓ SATISFIED | SC1 row |
| INPT-02 | 10-03, 10-07 | Notice never appears for a paste that fully arrived | ✓ SATISFIED | SC2 row |
| INPT-03 | 10-03, 10-07 | Browser viewer shows the same notice on a mid-paste drop | ? NEEDS HUMAN (acceptance) | SC3 row: behavior proven; literal SC wording deviations pending user acceptance |
| INPT-04 | 10-02 | Peer snapshots keyed by host plus session incl. Retry | ✓ SATISFIED | SC4 row |
| INPT-05 | 10-01, 10-07 | Exit-code test deterministic under full -race suite | ✓ SATISFIED | SC5 row |
| INPT-06 | 10-04, 10-05, 10-06, 10-07 | The viewer you type on, zoom or resize sets the size | ✓ SATISFIED | SC6 row |

No orphaned requirements: REQUIREMENTS.md maps exactly INPT-01..06 to Phase 10, and every one is claimed by a plan. REQUIREMENTS.md still shows them "Pending". This verifier did not run requirements.mark-complete, as instructed.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| 19 product files changed in the phase | — | TBD/FIXME/XXX | none found | — |
| same | — | added TODO/HACK/PLACEHOLDER | none found | — |
| input.go, hub.go, TerminalPanel.tsx, handoff.go, attach, oscabsorb.go | — | `shortcut:` comments | ℹ️ Info | The accepted limits from CONTEXT (relaunched GUI shows the current grid, attach wider grid, paste over quota, upgrade during a slow paste), IN-01 frozen-claim loss and IN-08 focus-gate gaps. All documented as intended |
| 10-07-PLAN Task 3 `<automated>` | — | "no FAIL row anywhere in 10-VM-CHECKS.md" | ℹ️ Info | That check is now false by design: the R-05 and RF-03 FAIL rows are kept above their PASS re-runs (the Phase 9 convention). Not a goal gap |
| frontend TerminalPanel.scale.test.tsx | 224-271 | source-text assertions (IN-07, accepted) | ℹ️ Info | Behavioral tests sit alongside them |

### Human Verification Required

#### 1. SC3 drop mechanism (pf instead of `tailscale down`)

**Test:** Read the "Deviation (SC3)" paragraph in the 10-VM-CHECKS.md header and rows R-00, R-03 and R-04.
**Expected:** You accept the pf `block drop` of the share port as SC3's network drop, or you ask for an R-04 re-run with a literal `tailscale down` using the plan's fallback procedure.
**Why human:** It is a deviation from the roadmap's wording, and the docs record it as "For the user to accept". No acceptance is recorded.

#### 2. SC3 "status bar" means the role="status" banner

**Test:** Confirm that the React browser viewer's `role="status"` SessionEndedBanner ("Paste may have been cut short (N KB)") satisfies SC3's "in its status bar".
**Expected:** Accepted. This was an orchestrator decision recorded in 10-03-PLAN and the CONTEXT "Decided after research". terminal.js, which had a status bar, is dead code by decision.
**Why human:** It reinterprets roadmap wording.

#### 3. Recommended: SC6 window-resize claim after WR-03

**Test:** In the VMs at HEAD, repeat R-06a's home-window resize and R-06b's Safari window-rect resize. Then also resize an inactive window.
**Expected:** A focused resize logs one `why=claim` grid line and `tput cols` changes. An unfocused resize only reports, which is the documented WR-03/IN-08 shortcut.
**Why human:** The focus gate came after the R-06 rows. Its unit test stubs `hasFocus`. Real WKWebView and Safari focus can only be seen in the VMs.

#### 4. Optional: SC5 -count=20 at the final tree

**Test:** `GOTOOLCHAIN=go1.26.11 go test -race -count=20 -timeout 60m ./internal/daemon/`
**Expected:** ok, 0 FAIL, 0 DATA RACE.
**Why human:** The firewall alert needs the user's go-ahead. The passing run predates the WR-02 and RF-03 daemon changes and their two new tests. The named INPT-05 test and the code it exercises have not changed.

#### 5-6. Harvested 10-07 human-checks (recorded verdicts)

Both 10-07 `<human-check>` blocks (Task 2: R-00, R-01a-c, R-05, R-02, V-02r; Task 3: R-06a-f, R-03, R-04) were run by Claude in the Tart VMs. 10-VM-CHECKS.md records all of them as PASS (R-05 failed first, then passed after 9cda1853). They are listed in the frontmatter as harvested, with their `<expected>` text unchanged.

### Gaps Summary

No blocking gaps. The code delivers every criterion:

- The hub-owned input FIFO and out-of-band owed pongs are wired into both read pumps.
- The notice semantics hold: a pong means "read and queued in order", and the ack clears the paste.
- The browser notice lives in the React viewer (terminal.js is left dead by decision).
- Peer tabs are keyed by host plus session, with Retry cleanup and per-host deltas.
- The INPT-05 test is hermetic and passed -count=20.
- The grid owner model is in place: claim bit, typing claims via the auto-reply classifier, the attach SIGWINCH claim, per-tab peer zoom, clamp, and read-only, revoked and departed viewers ignored.

Every criterion has tests, and the VM rows cover the VM-only parts. Status is human_needed for these reasons:

- SC3 depends on two deviations from its literal wording that the user has not accepted (pf instead of `tailscale down`; role="status" banner instead of "status bar").
- Two post-VM changes have only unit coverage: the WR-03 resize focus gate, and the daemon -count=20 after WR-02/RF-03. A re-check is recommended for both.

Informational follow-ups already captured: the RF-03 re-run's "ended peer tab shows 'stopped sharing' after a remount" and "brief unreachable sidebar" (todo be60964c); the RF-01 payload deviation (300 KB instead of one frame), noted for the user to accept, not tied to an SC.

---

_Verified: 2026-11-21T08:02:23Z_
_Verifier: Claude (gsd-verifier)_

## Addendum (2026-11-21, human items resolved; docs-only changes after this report)

- **SC3 drop mechanism and "status bar" wording:** accepted by the user on 2026-11-21 ("Accept both"). The pf `block drop` of the share port stands in for `tailscale down`, and the `role="status"` banner is the accepted surface. SC3 is VERIFIED, so the score is 6/6.
- **Item 3, WR-03 focus gate:** VM row WR3-01 PASS (10-VM-CHECKS.md, "WR-03 focus gate re-check"). A focused peer tab or Safari resize claims, while unfocused or background resizes (home, Safari, office) don't, and typing after focusing claims.
- **Item 4, SC5 re-run at current product code:** `go test -race -count=20 -timeout 60m ./internal/daemon/` gave `ok ... 1954.929s` with 0 DATA RACE and 0 `device not configured` (scratchpad p10-sc5-final.log). The user approved the run on 2026-11-21.
- RF-01's 300 KB payload is a test-method choice, not tied to a success criterion. It is recorded in 10-VM-CHECKS.md.

The digest above was recomputed over the same covered_files after the 10-VM-CHECKS.md append.
