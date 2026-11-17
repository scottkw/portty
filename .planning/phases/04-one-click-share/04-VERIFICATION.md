---
phase: 04-one-click-share
verified: 2026-11-17T22:40:00Z
status: human_needed
score: 14/16 must-haves verified (1 present but behavior-unverified, 1 needs the live two-Mac repro)
covered_files: [".github/workflows/build.yml", ".planning/phases/04-one-click-share/04-01-PLAN.md", ".planning/phases/04-one-click-share/04-01-SUMMARY.md", ".planning/phases/04-one-click-share/04-02-PLAN.md", ".planning/phases/04-one-click-share/04-02-SUMMARY.md", ".planning/phases/04-one-click-share/04-03-PLAN.md", ".planning/phases/04-one-click-share/04-03-SUMMARY.md", ".planning/phases/04-one-click-share/04-04-PLAN.md", ".planning/phases/04-one-click-share/04-04-SUMMARY.md", ".planning/phases/04-one-click-share/04-05-PLAN.md", ".planning/phases/04-one-click-share/04-05-SUMMARY.md", ".planning/phases/04-one-click-share/04-06-PLAN.md", ".planning/phases/04-one-click-share/04-06-SUMMARY.md", ".planning/phases/04-one-click-share/04-07-PLAN.md", ".planning/phases/04-one-click-share/04-07-SUMMARY.md", ".planning/phases/04-one-click-share/04-08-PLAN.md", ".planning/phases/04-one-click-share/04-08-SUMMARY.md", ".planning/phases/04-one-click-share/04-09-PLAN.md", ".planning/phases/04-one-click-share/04-09-SUMMARY.md", ".planning/phases/04-one-click-share/04-10-PLAN.md", ".planning/phases/04-one-click-share/04-10-SUMMARY.md", "TESTING.md", "app.go", "cmd_attach.go", "cmd_share.go", "frontend/src/App.tsx", "frontend/src/components/LinkNoLongerWorks.tsx", "frontend/src/components/SessionEndedBanner.tsx", "frontend/src/components/SharePopover.tsx", "frontend/src/components/Sidebar.tsx", "frontend/src/components/TabBar.tsx", "frontend/src/components/TerminalPanel.tsx", "frontend/src/components/__tests__/App.tailscale.test.tsx", "frontend/src/components/__tests__/App.test.tsx", "frontend/src/lib/closeReasons.ts", "frontend/src/lib/shareCopy.ts", "go.mod", "internal/attach/attach.go", "internal/daemon/api.go", "internal/daemon/attach_viewers_test.go", "internal/daemon/daemon_log.go", "internal/daemon/daemon_log_test.go", "internal/daemon/engine.go", "internal/daemon/peer_connect.go", "internal/daemon/process.go", "internal/daemon/process_unix.go", "internal/daemon/remote_ws_proxy.go", "internal/daemon/remote_ws_proxy_test.go", "internal/daemon/share.go", "internal/daemon/share_test.go", "internal/relay/close_reasons.go", "internal/relay/hub.go", "internal/relay/hub_test.go", "internal/relay/scrollback.go", "internal/tailnet/sessions.go", "internal/webserver/revoke_test.go", "internal/webserver/server.go", "internal/webserver/whois.go", "tests/check-share-removal.sh", "web/dashboard.html", "web/link-invalid.html"]
covered_digest: "v4:sha256:069e0ac1cc8bd02afcfed4672aca663db19b6a5c7da462a5b4e0609ff9446117"
behavior_unverified: 1
overrides_applied: 0
re_verification:
  previous_status: human_needed
  previous_score: 5/5 roadmap success criteria (automatable evidence); 6/6 requirements
  gaps_closed:
    - "UAT gap 2: browser stop banner names the host by its tailnet short name (04-09)"
    - "UAT gap 3: with Tailscale off, Share can be turned off from the popover switch; 04-UI-SPEC.md gives one answer (04-09, plus review fix WR-05)"
    - "UAT gap 4: a viewer joining through another Mac's daemon proxy gets a catch-up history larger than 32 KiB (04-10)"
  gaps_remaining:
    - "UAT gap 1 (CLI attach blanking the other viewers) is diagnosed, not closed: root cause not pinned; live repro M-83 with daemon.log pending"
  regressions: []
advisory:
  - finding: "WR-03: ViewerReadLimit (1 MiB) is an estimate for the wrapped-ring RenderSnapshot preamble; a very large truecolor screen could exceed it and reproduce the gap-4 drop"
    category: other
    reason: "Deferred by user decision 2026-11-17; todo .planning/todos/pending/2026-11-17-chunk-catch-up-replay.md. Resolved by chunking the preamble into frames of 32 KiB or less"
    evidence_status: "none provided (review reasoning only; no failing test)"
  - finding: "WR-04: viewer input over 32 KiB (a large paste in a peer GUI tab or browser; a full 32 KiB read in the CLI StdinPump) closes the connection with 1009"
    category: other
    reason: "Pre-existing. Deferred by user decision; todo 2026-11-17-split-large-pastes.md. The new comment in remote_ws_proxy.go (input 'limited to 32 KiB anyway') understates this"
    evidence_status: "none provided"
  - finding: "WR-02: the catch-up replay query strip misses XTWINOPS reports, XTGETTCAP/DECRQSS, DA3, OSC 4 and OSC 52 queries, so H-replies is only partly fixed"
    category: security
    reason: "Deferred by user decision; todo 2026-11-17-strip-more-terminal-queries-from-replay.md. OSC 52 read is the sharpest case (viewer clipboard typed into the host shell if the joining terminal permits clipboard reads)"
    evidence_status: "none provided"
  - finding: "TestRevoke_InputAfterRevokeNeverReachesPTY flaked once under the full suite during 04-07 (input after share-off reached the PTY)"
    category: other
    reason: "Recorded open in deferred-items.md with no todo. Passed 5/5 here with -race -count=5. Worth a todo because SHAR-04 implies input stops at once"
    evidence_status: "single unreproduced failure"
behavior_unverified_items:
  - truth: "daemon.log lives in the 0700 config dir, mode 0600, keeps the previous run as daemon.log.1 and is capped at 10 MiB during a run (04-08 truth, as changed by review fix WR-01)"
    test: "Restart the daemon on a real Mac (quit Portty, kill the daemon or reboot, relaunch). Then ls -l ~/Library/Application\\ Support/portty/daemon.log*"
    expected: "daemon.log is -rw------- and starts just after the `daemon: listening on` line; daemon.log.1 holds the previous run plus that line; viewer-diag lines with microsecond timestamps appear in daemon.log after a share or attach"
    why_human: "startDaemonLog's os.SameFile gate and the dup2 onto the real fds 1 and 2 have no test (REVIEW-FIX says a test would need to redirect the test process's own stdio). TestDaemonLog_RollsOverPastCap covers the rollover on a stand-in descriptor only, and nothing runs startDetachedDaemon end to end"
human_verification:
  - test: "M-83 live repro of UAT gap 1: office Mac shares a shell that has run `seq 1 20000`; home Mac opens it in the GUI and a browser; run `tailscale ping <office>`; from Warp run `portty attach <office>:\"<session>\"`, watch for 15 s, type, detach with Ctrl-\\; repeat attach/detach three times"
    expected: "GUI tab and browser never blank or freeze; every command shows on every viewer; bar shows 4 viewers. If anything blanks, collect `grep viewer-diag daemon.log` (and daemon.log.1) from both Macs for 30 s either side and read them for a small `grid` line (H-grid), `slow` lines, a CLI `first input` right after `join` (H-replies regression), slow `proxy open` dial or `proxy end` ordering (H-net), or `message too big` (gap 4 regression)"
    why_human: "Root cause not pinned in-process. The UAT event was intermittent (first attach only), so one clean run is weak evidence; the diagnostic log is the real deliverable. Needs two Macs, a real tailnet path and Warp's terminal replies"
  - test: "daemon.log rotation on a real daemon restart (see behavior_unverified_items)"
    expected: "daemon.log 0600 starting after `daemon: listening on`; previous run in daemon.log.1"
    why_human: "The fd redirection path runs only in a real detached daemon"
  - test: "M-80 / M-79 banner text: turn Share off on the office Mac with a home GUI tab, a browser and `portty attach` open"
    expected: "Browser banner reads `<tailnet-short-name> stopped sharing this session.` (e.g. `kens-macbook-air-1574`), the same name as the GUI banner and the CLI line `Portty: <name> stopped sharing <session>.`; never `Kens-MacBook-Air.local`"
    why_human: "Needs a real share server whose certificate domain is the MagicDNS name. Covered in-process by TestShare_WebInfoHostIsTailnetName with a fake domain"
  - test: "M-80 long-session attach: on a shared session that has run `seq 1 20000` (history over 32 KiB), open it from the home Mac sidebar and from `portty attach office-air:<session>`"
    expected: "Both show the history ending in 20000 and keep receiving live output; the CLI shows the status bar and detaches cleanly"
    why_human: "Real tailnet and real webview. TestCLIAttach_LongHistoryJoiners covers it in-process (I reproduced its RED failure at 3290a355: `message too big: read limited at 32769 bytes`)"
  - test: "M-82 with real Tailscale off: for a shared session click the toolbar Shared button, turn the switch off, then click the button again; for an unshared session click Share"
    expected: "Shared button opens the popover showing the Tailscale-off reason with the switch on and enabled; switching off stops sharing; the same button click then closes the popover, after which it is dimmed and does nothing; an unshared session's Share button is dimmed and opens nothing"
    why_human: "Behavior changed since the UAT pass (04-09 and WR-05). Mounted jsdom tests cover the states; the real Tailscale toggle and WKWebView focus are not covered"
  - test: "M-80 step 6 (skipped in UAT, no second account): a Mac signed in to a different Tailscale account"
    expected: "No group for the office Mac; `portty attach office-air` prints the didn't-allow-access line"
    why_human: "Needs a second Tailscale account. The 403 path is covered with fakes (TestWhoIs_MetaRefused, TestPeerConnect_Forbidden)"
---

# Phase 4: One-Click Share Verification Report

**Phase Goal:** Sharing is one on/off switch per session: shared means tailnet peers and a tailnet web link at once, always full control, with a clean stop and CLI attach from any terminal. Funnel and read-only links are gone.
**Verified:** 2026-11-17
**Status:** human_needed
**Re-verification:** Yes, after the two-Mac UAT (3/3 passed, 4 gaps logged) and gap-closure plans 04-07..04-10 plus review fixes CR-01, WR-01, WR-05.

The five roadmap success criteria still hold, and the UAT passed all three live tests. UAT gaps 2, 3 and 4 are closed, and each closure has a test. For gap 4, I ran the new test at its RED commit in an extracted copy and confirmed it failed there. Gap 1 is diagnosed but not closed. The harness found one real disturbance (H-replies), which is now fixed, but it never reproduced the UAT signature, and the code and SUMMARY both say so plainly. The live M-83 repro, which reads the new daemon.log, is still pending. Nothing I found is a blocker.

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC1: one Share step lists the session in the other Mac's sidebar and gives a tailnet-only link; no tabs, modes or separate toggles | VERIFIED (regression) | share.go `shareOn` / `unshareLocked` intact (`Perms: "read,write"` at share.go:275). The UAT test 1 two-Mac flow passed live. Removal gate clean |
| 2 | SC2: the link opens a working interactive terminal with full control | VERIFIED (regression) | Unchanged write gate in server.go. UAT test 1: bidirectional typing across browser, GUIs and CLI |
| 3 | SC3: Share off disconnects every remote viewer and removes the session from peers' sidebars | VERIFIED (regression) | `RevokeWebViewers(relay.CloseReasonShareStopped)` still in share.go. UAT test 1 step 5 passed. The proxy still forwards the close (`TestRemoteWSProxy_DiagLog` checks `first=peer code=1000 reason="share stopped"`) |
| 4 | SC4: no Funnel, public-link, risk-panel or read-only code, UI, settings or tests | VERIFIED (regression) | `bash tests/check-share-removal.sh` gives `OK: share removal gate clean` (run by me) |
| 5 | SC5: `portty attach <host>:<session>` from any terminal attaches with the status bar | VERIFIED (regression) | UAT test 1 passed it from Warp. `runAttach` still hands its conn to `attach.AttachSession` (cmd_attach.go:322). No read happens before the new `SetReadLimit` |
| 6 | Gap 1: a CLI attach does not disturb other viewers of the same share (live, real tailnet) | ? UNCERTAIN, backstop (needs human) | In-process `TestCLIAttach_OtherViewersKeepOutput` and the 5 hypothesis subtests pass (run by me, uncached, -race). The harness uses the real share server, the real proxy route `/api/relay/remote/{id}/ws` and the real `attach.AttachSession`. The plan marks this truth `verification: backstop`. The UAT event was intermittent and its root cause is not pinned, so presence plus a green harness does not prove it. Routed to M-83 |
| 7 | Gap 1 diagnosis is explicit: each hypothesis has a verdict and one line says whether the root cause is pinned | VERIFIED | attach_viewers_test.go header: H-grid and H-net "not reproducible in-process", H-replies "Reproduced and fixed", H-resize and H-stuck "Refuted", plus `Root cause: not pinned — ...` pointing to 04-08 and M-83. The 04-07 SUMMARY's key-decisions say the same. No fix is claimed for the UAT event |
| 8 | H-replies fixed: the catch-up replay carries no reply-eliciting queries, so a joining terminal types nothing into the shared shell | VERIFIED | hub.go `ReconnectPreamble` runs `liveEmuQueryStripPattern.ReplaceAll(...)` on both raw paths. The engine.go mirror gained the same two alternatives. `TestReconnectPreamble_StripsTerminalQueries` and `TestCLIAttach_Hypotheses/replayed_query_replies` PASS. The list is incomplete (WR-02, advisory) |
| 9 | Host and viewing-side `viewer-diag:` lines (join/leave/grid/slow/revoke/first input/ignored resize; proxy open/dial failed/end), with no token, link, `cap=` or input bytes, and quoted client names | VERIFIED | hub.go, server.go and remote_ws_proxy.go diffs read. Names use `%q`, input is logged as counts, and the proxy redacts `u.RawQuery` and then the token. `TestHub_DiagLog`, `TestRemoteWSProxy_DiagLog` and the log assertions in `TestCLIAttach_OtherViewersKeepOutput` (including no cap) PASS. The 04-08 truth said "every web resize request it ignores". Since CR-01, only the first is logged per connection and the total goes into `resizes=` on the input-total line. That trades nothing needed for diagnosis and removes a disk-fill path |
| 10 | daemon.log in the 0700 config dir, 0600, previous run in daemon.log.1, size-capped (04-08 + WR-01) | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | Wiring is present. process_unix.go sets the child's Stdout and Stderr to `openDaemonLog(daemonConfigDir())` (append, 0600). process.go:66 calls `startDaemonLog` after the socket bind. daemon_log.go rotates and dup2s onto fds 1 and 2 past 10 MiB. Unit tests (`TestOpenDaemonLog_AppendsAndIs0600`, `TestDaemonLog_RollsOverPastCap`, `TestDaemonLog_GridChangeLandsInFile`) pass, but the `os.SameFile` gate and the dup2 onto the real fds are not exercised. The REVIEW-FIX itself lists this as untested |
| 11 | Gap 2: the browser stop banner names the host by its tailnet short name, like the GUI and CLI | VERIFIED | share.go `host := magicDNSLabel(h.Domain)` in the session resolver. `h.Domain` = `status.CertDomains[0]` (webserver/tailscale.go:47). Data flow: resolver -> `/api/sessions/{id}/info` -> App.tsx:760-769 `setWebHost` -> `hostLabel` (App.tsx:1050) -> SessionEndedBanner.tsx:71. `TestShare_WebInfoHostIsTailnetName` and `TestMagicDNSLabel` PASS |
| 12 | Gap 3: with Tailscale off, a shared session's Shared button opens the popover and the switch turns Share off; an unshared session stays blocked; UI-SPEC has one answer | VERIFIED | App.tsx:916 `shareDisabled={tailscaleOff && !activeShared && !sharePopoverOpen}`. SharePopover.tsx:266 `blocked = tailscaleOff && !shared && !busy` keeps Share-on blocked. The daemon still answers 503 tailscale_off (share.go:137-138). UI-SPEC lines 139-140, 271 and 347 agree. The mounted cases in App.tailscale.test.tsx PASS (44/44 with SharePopover.test.tsx) |
| 13 | Gap 4: a GUI tab and the real CLI joining through the proxy after about 120 KiB of history get every history line and live output | VERIFIED | `TestCLIAttach_LongHistoryJoiners` (both subtests) PASS at HEAD. **Independent RED check:** I ran `git archive 3290a355` into the scratchpad, and there both subtests FAIL with `proxy end ... err=failed to read: websocket: message too big: read limited at 32769 bytes`. The fix is what turns the test green |
| 14 | One named limit `relay.ViewerReadLimit` (4 x 256 KiB) on host-to-viewer readers only; viewer-input readers keep the 32 KiB default; no literal sizes | VERIFIED | `grep SetReadLimit` finds only remote_ws_proxy.go:118 (upstream, right after Dial and before copyWS) and attach.go:41 (before the pumps), plus a test-only `-1` on the stand-in webview conn. The share server, local relay and proxy client side have no `SetReadLimit` |
| 15 | CR-01: a link holder cannot fill the host disk with resize frames | VERIFIED | server.go logs `web resize ignored` only when `resizes == 1`. `TestResizeFlood_LogsOncePerConnection` PASS (50 frames give one line plus `resizes=50`). The daemon.log cap (truth 10) is the second layer |
| 16 | WR-05: after Share off with Tailscale off, the toolbar button can still close its open popover, and then cannot reopen it | VERIFIED | Same App.tsx:916 expression. The mounted sequence in App.tailscale.test.tsx PASS |

**Score:** 14/16 truths verified (1 present but behavior-unverified, truth 10; 1 backstop needing the live repro, truth 6).

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| internal/daemon/attach_viewers_test.go | Harness, hypothesis variants, verdict header, long-history variant | VERIFIED | 549 lines. Real host daemon and viewing daemon, real proxy, real `attach.AttachSession`. Not a stub |
| internal/daemon/daemon_log.go | openDaemonLog (append 0600), startDaemonLog, capped writer | VERIFIED (wired; live path behavior-unverified) | Called from process_unix.go and process.go |
| internal/daemon/daemon_log_test.go | Append, rollover, grid line lands in file | VERIFIED | 3 tests PASS. Uses t.TempDir() |
| internal/daemon/peer_connect.go | `magicDNSLabel` shared by peers and share server | VERIFIED | Used by `peerShortName` and share.go |
| frontend/src/components/__tests__/App.tailscale.test.tsx | Mounted Tailscale-off cases | VERIFIED | `mountApp` cases at lines 169, 203 |
| internal/relay/scrollback.go | `ViewerReadLimit = 4 * DefaultScrollbackBytes` | VERIFIED | Bound documented as estimated (WR-03) |
| internal/daemon/remote_ws_proxy.go | `upstream.SetReadLimit(relay.ViewerReadLimit)` | VERIFIED | Line 118 |
| internal/attach/attach.go | `conn.SetReadLimit(relay.ViewerReadLimit)` | VERIFIED | Line 41 |

The gsd-tools artifact check passed all artifacts in 04-07..04-10.

### Key Link Verification

The gsd-tools key-link check returned "Target not referenced in source" for every link in 04-07..04-10. The links are same-package calls, HTTP routes, or the process-wide `log` writer, so there is no import path for it to find. I checked each link by reading the code:

| From | To | Via | Status |
|------|----|-----|--------|
| attach_viewers_test.go | remote_ws_proxy.go | dials `viewRelay + "/api/relay/remote/" + sid + "/ws"` on `viewAPI.RelayHandler()` | WIRED |
| attach_viewers_test.go | attach/attach.go | `attach.AttachSession(...)`, `attach.MakeClientResizeFrame` | WIRED |
| attach_viewers_test.go | webserver/server.go | browser dials `wss://<link host>/sessions/<sid>/ws?cap=` with the link Origin | WIRED |
| process_unix.go | daemon_log.go | `openDaemonLog(daemonConfigDir())` -> `cmd.Stdout/Stderr` | WIRED |
| relay/hub.go | daemon_log.go | `log.Printf` -> `log.SetOutput(d)` in `startDaemonLog` (or the inherited stderr = daemon.log) | WIRED |
| webserver/server.go | relay/hub.go | `sub.Remote` set before `hub.Subscribe`; read pump logs with `sub.ID` | WIRED |
| share.go | peer_connect.go | `magicDNSLabel(h.Domain)` in the session resolver | WIRED |
| App.tsx | TabBar.tsx | `shared={activeShared}`, `shareDisabled={tailscaleOff && !activeShared && !sharePopoverOpen}` | WIRED (expression extended by WR-05) |
| App.tsx | SharePopover.tsx | `tailscaleOff` prop; `blocked = tailscaleOff && !shared && !busy` | WIRED |
| remote_ws_proxy.go | scrollback.go | `upstream.SetReadLimit(relay.ViewerReadLimit)` after Dial, before copyWS | WIRED |
| attach.go | scrollback.go | `SetReadLimit` first line of `AttachSession` | WIRED |
| cmd_attach.go | attach.go | `attach.AttachSession(ctx, conn, ...)` at line 322, no earlier read | WIRED |

### Data-Flow Trace (Level 4)

| Artifact | Data | Source | Real data | Status |
|----------|------|--------|-----------|--------|
| Browser stop banner host | `webHost` | `/api/sessions/{id}/info` `hostname` <- resolver <- `magicDNSLabel(CertDomains[0])` | yes | FLOWING |
| Toolbar Share button state | `activeShared` | `sessions` poll `webEnabled` | yes | FLOWING |
| Proxied catch-up replay | preamble message | `Hub.ReconnectPreamble` (query-stripped) -> share server -> proxy upstream (1 MiB limit) -> client | yes | FLOWING (test-proven at about 120 KiB) |

### Behavioral Spot-Checks

All of these were run by me in this session.

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Gap tests, daemon | `go test -race -count=1 ./internal/daemon -run '^(TestCLIAttach_OtherViewersKeepOutput\|TestCLIAttach_Hypotheses\|TestCLIAttach_LongHistoryJoiners\|TestOpenDaemonLog_AppendsAndIs0600\|TestDaemonLog_RollsOverPastCap\|TestDaemonLog_GridChangeLandsInFile\|TestRemoteWSProxy_DiagLog\|TestMagicDNSLabel\|TestShare_WebInfoHostIsTailnetName)$'` | 9/9 PASS (subtests included) | PASS |
| Gap tests, relay | `go test -race -count=1 ./internal/relay -run '^(TestReconnectPreamble_StripsTerminalQueries\|TestHub_DiagLog)$'` | 2/2 PASS | PASS |
| CR-01 | `go test -race -count=1 ./internal/webserver -run '^TestResizeFlood_LogsOncePerConnection$'` | PASS | PASS |
| Gap 4 RED proof | `git archive 3290a355` into the scratchpad; `go test ./internal/daemon -run '^TestCLIAttach_LongHistoryJoiners$'` | both subtests FAIL: `message too big: read limited at 32769 bytes` | PASS (RED confirmed) |
| Revoke flake re-run | `go test -race -count=5 ./internal/webserver -run '^TestRevoke_InputAfterRevokeNeverReachesPTY$'` | ok | PASS |
| Frontend gap 3 / WR-05 | `pnpm exec vitest run App.tailscale.test.tsx SharePopover.test.tsx` | 2 files, 44 tests PASS | PASS |
| Removal gate | `bash tests/check-share-removal.sh` | `OK: share removal gate clean` | PASS |
| Traceability | `bash tests/check-traceability-paths.sh` | `OK: 111 traceability paths exist` | PASS |
| Commits | `verify.commits` on 13 gap and fix hashes | all_valid | PASS |

Orchestrator evidence I did not re-run, to avoid a second full suite: `go build ./...` OK, `go test -race -short -skip '^(TestAPIAppearance_GetPatch|TestAPITerminalFont_GetPatch)$' ./...` exit 0, vitest 82 files / 1047 tests. The two skipped tests are a known pre-existing isolation bug that reads the real settings.json (todo 2026-11-17-isolate-daemon-settings-tests.md). The failure is in the test, and it does not affect the phase goal.

### Probe Execution

Step 7c: not applicable. No `scripts/*/tests/probe-*.sh` is declared. The phase gates are the tests/*.sh scripts above.

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| SHAR-01 | 04-02/03/05/06, 04-07..04-10 | One on/off state; peers plus link | SATISFIED | Truths 1, 12, 13 |
| SHAR-02 | 04-01, 04-02 | Always full control; no read-only link | SATISFIED | `Perms: "read,write"` only; truth 4 |
| SHAR-03 | 04-02/03/06, 04-07..04-09 | Link gives a working interactive terminal | SATISFIED | Truths 2, 11 |
| SHAR-04 | 04-02/03/04/06, 04-09 | Stop disconnects viewers and removes from sidebars | SATISFIED | Truths 3, 11, 12, 16 |
| SHAR-05 | 04-01/05/06 | Funnel removed | SATISFIED | Truth 4 |
| SHAR-06 | 04-04, 04-07, 04-08, 04-10 | `portty attach <host>:<session>` with status bar | SATISFIED (gap 1 live check pending) | Truths 5, 7-10, 13, 14 |

There are no orphaned requirements. REQUIREMENTS.md maps exactly SHAR-01..06 to Phase 4, and every plan's `requirements:` field stays within that set.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| gap diff (all .go/.ts/.tsx added lines) | - | TBD/FIXME/XXX/TODO/HACK | none found | - |
| internal/daemon/remote_ws_proxy.go | 66-67 | Comment says viewer input is limited to 32 KiB "anyway", which treats a 1009 on a large paste as harmless (WR-04) | Info | Misleading comment. Behavior is pre-existing. Deferred with a todo |
| internal/relay/scrollback.go | 15-19 | ViewerReadLimit bound is estimated, not enforced (WR-03) | Warning (advisory) | A very large styled screen after the ring wraps could reproduce the gap-4 drop. Deferred with a todo |
| internal/relay/hub.go | 122-133 | Query strip list incomplete (WR-02) | Warning (advisory) | Some TUI startup queries (and OSC 52 read) still replay. Deferred with a todo |
| internal/daemon/peer_connect.go | 45-47 | Stale `peerShortName` doc comment above `magicDNSLabel` (IN-01) | Info | godoc only |
| internal/daemon/remote_ws_proxy.go | 159-170 | `copyEnd.side` blames the read side for a write failure (IN-03) | Info | Could point M-83's diagnosis at the wrong Mac on a write error. Keep in mind when reading `proxy end first=` lines |
| internal/daemon/process.go / service.go | 23 | Microsecond timestamps only on the RunDaemon path (IN-04) | Info | A service-manager-launched daemon logs at second resolution |
| internal/relay/hub.go | 322-323 | Grid line `by=` is always `local` (IN-02) | Info | Names the caller, not the subscriber that set the minimum |
| internal/relay/scrollback_altscreen_test.go | - | not gofmt-clean (pre-existing) | Info | Out of scope |

### Advisory (New Scope, Unevidenced)

| # | Finding | Category | Why Advisory |
|---|---------|----------|--------------|
| 1 | WR-03 ViewerReadLimit bound estimated | other | Review reasoning, no failing test. User deferred it with a todo |
| 2 | WR-04 input over 32 KiB drops the viewer | other | Pre-existing. User deferred it with a todo |
| 3 | WR-02 incomplete query strip | security | No failing test. User deferred it with a todo |
| 4 | TestRevoke_InputAfterRevokeNeverReachesPTY single flake | other | Not reproduced (5/5 here). In deferred-items.md but has **no todo**. Consider filing one |

### Human Verification Required

1. **M-83 live repro of UAT gap 1.** Run CLI attach/detach beside a home GUI tab and a browser on a session with more than 32 KiB of history, and run `tailscale ping` first. Expected: no viewer blanks. If one does, collect the `viewer-diag` lines from both Macs. The root cause is not pinned, and the event was intermittent, so a clean run is weak evidence. The log is what turns a recurrence into a diagnosis.
2. **daemon.log on a real restart.** Expected: `daemon.log` is 0600 and starts after `daemon: listening on`, and the previous run is in `daemon.log.1`. The fd-redirection path is untested.
3. **M-80 / M-79 banner text.** Expected: the browser banner uses the tailnet short name (e.g. `kens-macbook-air-1574`), the same as the GUI and CLI, never `.local`.
4. **M-80 long-session attach.** Expected: after `seq 1 20000`, both the GUI tab and `portty attach` show history ending in 20000 and keep live output.
5. **M-82 with real Tailscale off.** Expected: the Shared button opens the popover, and the switch turns Share off. After that the button closes the popover once, then is dimmed. An unshared session's Share button stays dimmed.
6. **M-80 step 6 (skipped in UAT).** Expected: a Mac on another Tailscale account sees nothing, and attach prints the access-refused line.

### Gaps Summary

There are no blocking gaps. Gaps 2, 3 and 4 from the UAT are closed with substantive, wired code and passing tests. Gap 4 also has an independent RED-to-GREEN confirmation. The review fixes CR-01, WR-01 and WR-05 are in the code and tested, except for WR-01's live fd path (truth 10).

Gap 1 is the open item. 04-07 did what an honest diagnosis can do in-process. It built the real attach path, refuted two hypotheses, fixed one real disturbance (H-replies), and stated that the UAT root cause is not pinned. 04-08 added the logging needed to pin it on a recurrence. Whether the phase is done with gap 1 is a human decision after M-83. Until then the phase is `human_needed`, not `passed`.

The deferred review warnings (WR-02, WR-03, WR-04) are real but outside the success criteria, and each has a todo. WR-03 deserves attention before Phase 5's overnight test. A large styled screen on a long-lived session could make a reattach hit the same "message too big" drop, and that would look like a Phase 5 failure.

---

_Verified: 2026-11-17_
_Verifier: Claude (gsd-verifier)_
