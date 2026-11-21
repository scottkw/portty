---
phase: "10"
slug: "input-path-reliability"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-11-21"
---

# Phase 10 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

The register comes from the `<threat_model>` blocks of 10-01..10-07-PLAN.md: T-10-01..T-10-29, plus T-10-SC, which every plan repeats and which is counted once here (30 threats). Each mitigation was checked against the code at `4d9b78da` (Phase 10 complete). `git diff --stat HEAD -- . ':!.planning' ':!.gitignore'` is empty at audit time, so the working tree is the audited code; the untracked `agenthub` binary and `.ignore` are not part of the product. The phase's code diff is `d2998ad2..4d9b78da` (60 files); `d027e86d..d2998ad2` touches only `.planning`. 10-CONTEXT.md, 10-REVIEW.md (both iterations), 10-REVIEW-FIX.md (both iterations and the RF-03 fix), 10-VM-CHECKS.md and 10-VERIFICATION.md were read, and every review fix that a mitigation or a constraint depends on was traced into the code (CR-01, CR-02, WR-01, WR-02, WR-04, WR-05, IN-03, IN-09, RF-03).

Config: security enforcement on, ASVS level 1, block on `high`. The high-severity input and grid threats were also checked at the boundary where each control sits (ASVS L2 depth): the read-only gate sits in both read pumps and again in the hub, the clamp sits at hub intake ahead of every grid change, the revoke drop is keyed by origin and epoch rather than by subscriber set, and the takeover's input hold is set before the takeover serves anything. The audit verified that the declared mitigations exist. It did not scan for new threats. Review-era changes that crossed a trust boundary but are not in the plans' registers are verified under Unregistered Flags. Observations found along the way are listed separately and are non-blocking.

Commands run (no Go or vitest suite was run, by the user's rule on test binaries and firewall prompts):
- `GOTOOLCHAIN=go1.26.11 go build ./... && go vet ./...`: rc 0.
- `git diff --stat d027e86d..HEAD` of `internal/webserver/csp_mw.go`, `capability_mw.go`, `origin_mw.go`, `whois.go`, `tailscale.go`, `selfcert.go`, `internal/tailnet`, `internal/capability`, `frontend/index.html`, `web/` (incl. `web/assets/terminal.js`), `internal/relay/close_reasons.go`, `internal/daemon/share.go`, `internal/daemon/api.go`, `internal/daemon/remote_caps.go`: empty.
- `git diff --stat d027e86d..HEAD` of `go.mod`, `go.sum`, `frontend/package.json`, `frontend/pnpm-lock.yaml`, `.github`, `build.sh`, `scripts`, `tests`, `wails.json`: empty.
- `git grep SetReadLimit` at `d027e86d` and at HEAD (non-test): the same three client-side calls (attach.go:41, loop.go:187, remote_ws_proxy.go:131). No server-side input reader sets a read limit, so coder/websocket's 32,768-byte default is still the frame guard.
- Added lines of the non-test Go diff: 0 matches for `HandleFunc`, `Handle(`, `mux.`, `os.Open`, `ReadFile`, `ReadDir`, `ServeFile`, `FileServer`, `OpenFile`, `os.Create`, `WriteFile`, `exec.Command`. Added lines of the non-test frontend diff: 0 matches for `innerHTML`, `dangerouslySetInnerHTML`, `outerHTML`, `insertAdjacentHTML`, `document.write`, `eval(`, `new Function`, `fetch(`.
- `bash tests/check-brand.sh`: "OK: brand gate clean".
- Greps of every file in the phase directory for `cap=[A-Za-z0-9_-]{6,}`, `tskey-`, `token=[A-Za-z0-9_-]{8,}` and URLs with a query string: 0 matches. Every 40+ character token-like string in 10-VM-CHECKS.md is a sha256 of a binary or payload.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| share-link holder → share read pump | Tailnet viewers holding a signed cap send input, resize/claim and ping frames at any rate, each up to the 32 KiB frame guard | MsgInput, MsgResize2 (+ claim bit), MsgPing |
| local client → relay read pump | The GUI and `portty attach` over loopback (never read-only) | the same frames |
| hub input FIFO → PTY | One writer for every viewer's input into a shell; a backlog can outlive the socket | queued input bytes |
| viewer → session grid | Reports and claims decide the PTY size for every viewer | cols/rows, ownership |
| a viewer's terminal → input stream | Terminals answer queries automatically, with no person involved | auto-replies |
| old daemon → takeover (handoff) | Both processes hold the same PTY masters during the commit tail | ordering of input writes |
| peer daemon → local proxy → GUI / attach | A peer's close code and reason are forwarded to the local viewer | close frames |
| peer Mac → tab list / snapshot store | Session ids from other Macs; snapshots restore terminal contents | tab ids, snapshot keys |
| executor → host / VMs; evidence file → repository | Faults, pf drops, clicks only inside the Tart VMs; evidence must hold no tokens | logs |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation (evidence at 4d9b78da) | Status |
|-----------|----------|-----------|----------|-------------|-----------------------------------|--------|
| T-10-01 | Repudiation | engine_exit_test.go | low | mitigate | The racing test was split into a real-PTY running case (`TestExitEvent_ListSessions_RunningHasNoExitCode`) and a stopped case that waits for the exit watcher instead of forcing `SetState` (engine_exit_test.go). 10-01-SUMMARY records the 3-of-20 repro and 200/200 under `-race`; 10-07-SUMMARY records SC5 `-race -count=20 ./internal/daemon/` run 3 PASS (`ok … 1853.588s`, 0 FAIL, 0 DATA RACE) | closed |
| T-10-02 | Information disclosure | App.tsx peer tab identity | medium | mitigate | `webSessionTabId(sessionId, host)` puts the host in a peer tab's id (tabs.ts:50-52); App keys tabs by it (App.tsx:810, 893). Snapshot keys are `host\0session` (peerSnapshots.ts:39-46). Test `App — two peers with the same session id (INPT-04)` (App.open-remote.test.tsx:286) | closed |
| T-10-03 | Spoofing | daemon RemoteCapStore keyed by session id | low | accept | `internal/daemon/remote_caps.go` unchanged in the phase. See AR-10-01 | closed (accepted at plan time) |
| T-10-04 | Denial of service | peerSnapshots store | low | mitigate | `retryInit` computes `peerTabsRemoved` (tabs.ts:65-67) and drops each one's snapshot (App.tsx:997-1000). The count and byte caps are unchanged (`PEER_SNAPSHOT_MAX = 8`, `PEER_SNAPSHOT_MAX_BYTES = 8 MiB`, peerSnapshots.ts:24, 26; no diff since d027e86d). Test App.daemon-recovered.test.tsx:83 | closed |
| T-10-05 | Denial of service | relay input FIFO | high | mitigate | Per-connection quota `InputMaxFrames = 1024`, `InputMaxBytes = 32 MiB` (input.go:37-38), enforced in the `Write` wait loop (input.go:93-97). Review WR-01/WR-05 added a hub cap, `InputHubMaxBytes = 64 MiB`, with a per-connection `InputConnHubAllowance = 64 KiB` (input.go:45-46, 95). Frames stay at most 32,767 bytes (`MaxInputPayload`, protocol.go:112; no server-side `SetReadLimit`). The queue is dropped on a PTY write error (input.go:271-272) and when the hub ends (done watcher, input.go:212-220, `killInputLocked` input.go:226-239). Read-only input never reaches the queue (webserver/server.go:961). Tests `TestHubInput_QuotaBlocksOnlyThatConn` (input_test.go:80), `TestHubInput_HubCapBlocks` (:128), `TestHubInput_HubCapAllowsTyping` (:156). Residual: O-1 | closed |
| T-10-06 | Denial of service | read pumps on a dead hub | medium | mitigate | `Write`'s wait loop exits on `inDead` and returns `ErrInputClosed` at once (input.go:93, 98-100); `killInputLocked` broadcasts to every waiter (input.go:237). The mechanism changed in review WR-04: the pumps no longer return on `ErrInputClosed` (webserver/server.go:968-972, relay/server.go:184-189); `WritePump` closes with 1000 "session ended" on `hub.Done` (pong.go, the `hub.Done` case), which ends the socket and the read pump. No spin: the pump blocks in `conn.Read` between frames. Tests `TestHubInput_WriterDeathUnblocks` (input_test.go:183), `TestRelay_InputClosedDefersToSessionEnd`, `TestRelay_DeadInputKeepsViewer` (server_input_queue_test.go:81, 116). See O-6 | closed |
| T-10-07 | Tampering | PTY input ordering across a reconnect | medium | mitigate | One queue per hub (`h.inq`, appended in `Write`, input.go:104) and one writer goroutine started once (`startInputLocked`, input.go:206-211). The only PTY write in `internal/relay` is input.go:264; `Hub.WriteInput`, which bypassed the queue, was removed (IN-03). Both pumps feed it (`OpenInput`: webserver/server.go:935, relay/server.go:161). Across an upgrade, the takeover holds its writers until the old daemon exits (WR-02, see UF-10-04). Test `TestHubInput_FIFOAcrossConnections` (input_test.go:53) | closed |
| T-10-08 | Repudiation | pong ack during an upgrade | low | accept | `finish` waits for idle input within the same `restartDrain` deadline (handoff.go:770-786) and `waitHubsInputIdle` logs `input still queued at exit` with byte counts (handoff.go:798-813). The `shortcut:` comment is at handoff.go:781-785. See AR-10-02 | closed (accepted at plan time) |
| T-10-09 | Information disclosure | viewer-diag and handoff logs | medium | mitigate | Every log line added in the phase formats only a DiagID, conn ids, frame/byte counts, sizes, a claim flag, a pid, a fixed close-reason constant (`%q`) or a PTY write error: input.go:125, 238, 274; hub.go:473, 662-663; handoff.go:806, 1350; webserver/server.go:997-998. The existing "input payloads are never logged" rule is kept (webserver/server.go:940) | closed |
| T-10-10 | Denial of service | pong flood | low | accept | Pongs are counted (`PongOwed`, atomic, pong.go) and written by the viewer's own write pump. See AR-10-03 | closed (accepted at plan time) |
| T-10-11 | Tampering | 32 KiB frame guard, CSP, capability, tailnet guard | high | mitigate | `csp_mw.go`, `capability_mw.go`, `origin_mw.go`, `whois.go`, `internal/tailnet` (incl. live.go), `internal/capability`, `frontend/index.html` and `web/assets/terminal.js` have no diff since d027e86d. `MaxInputPayload = 32767` unchanged (protocol.go:108-112) and no server-side reader sets a read limit (both before and after). The webserver diff touches only `handleWSSRelay`'s body; `StartOn`/`assertTailnetBindIP` (server.go:375-384) and the WebSocket `OriginPatterns` (server.go:847) are outside every hunk | closed |
| T-10-12 | Elevation of privilege | Hub.ResizeClient/ClaimGrid, both pumps | high | mitigate | Pumps: the share pump calls `ClaimGrid`/`ResizeClient` only when `!sub.ReadOnly` (webserver/server.go:1000); the relay pump the same (relay/server.go:194). Hub: `sizeClient` returns at once for `sub.ReadOnly` (hub.go:602). `ReadOnly` comes from the signed cap's perms (`HasPerm(claims.Perms, "write")`, webserver/server.go:820-821). Tests `TestHub_GridReadOnlyIgnored` (hub_grid_owner_test.go:233), `TestWebClaim_ReadOnlyCapIgnored` (claim_test.go:120) | closed |
| T-10-13 | Denial of service | resizeLiveEmulator allocation | high | mitigate | `MinGridCols=20`, `MaxGridCols=500`, `MinGridRows=5`, `MaxGridRows=200` (hub.go:517-522). `sizeClient` ignores zero or negative sizes and clamps before taking any lock (hub.go:602-605); it is the only path that sets `sub.Cols/Rows` (hub.go:613), and `wantCols/wantRows` come only from those (hub.go:591). `applyGridLocked` reads only the target (hub.go:647-653). `Hub.Resize` has no production caller. Tests `TestHub_GridClamp` (hub_grid_owner_test.go:247), `TestWebClaim_Clamped` (claim_test.go:170) | closed |
| T-10-14 | Denial of service | claim storm from a write-cap holder | low | accept | A claim rebuilds only on a real size change (`applyGridLocked` returns early when unchanged, hub.go:649). See AR-10-04 | closed (accepted at plan time) |
| T-10-15 | Tampering | grid during a handoff rejoin | high | mitigate | A plain report is stored only, owner or not, unless it is a local viewer on a never-sized, ownerless grid (hub.go:615-622). `Join` never touches the grid; `Unsubscribe` clears the owner and keeps the grid (hub.go:358-360, D-01). The Phase 9 settle window was removed (no `settle` field left in hub.go or handoff_state.go). Test `TestHub_HandoffRejoinKeepsGrid` (handoff_state_test.go:314); 10-VM-CHECKS V-02r PASS | closed |
| T-10-16 | Information disclosure | viewer-diag grid and web resize lines | low | mitigate | Web resize line: DiagID, conn, cols, rows, claim (webserver/server.go:997-998), once per connection. Grid line: sizes, origin, client name (the 64-byte query hint, pre-existing), conn, why (hub.go:662-663). No payloads, caps or URLs | closed |
| T-10-17 | Tampering | TerminalPanel reports | medium | mitigate | `reportFit` and `claimFit` send `measureFit(term)`, never `term.cols/rows` (TerminalPanel.tsx:268-305). Tests TerminalPanel.scale.test.tsx:246, 657 | closed |
| T-10-18 | Denial of service | claim churn from UI chrome | medium | mitigate | `claimFit` has two callers: the window `resize` handler after `CLAIM_DEBOUNCE_MS = 150` (TerminalPanel.tsx:74, 1008-1016), gated by WR-03 on `visibilityState === 'visible' && hasFocus()`, and a font change while the panel is shown (TerminalPanel.tsx:1048). The ResizeObserver only calls `relayout` → `reportFit` (TerminalPanel.tsx:996). Tests TerminalPanel.scale.test.tsx:688, 713, 731, 748 | closed |
| T-10-19 | Information disclosure | share SPA rendering | high | mitigate | No `innerHTML`, `dangerouslySetInnerHTML` or other HTML sink in any added frontend line; the only two mentions in `frontend/src` are comments in files unchanged since d027e86d (LinkConfirmPopover.tsx:14, SessionEndedBanner.tsx:33). CSP (`csp_mw.go`, `frontend/index.html`) and `terminal.js` unchanged | closed |
| T-10-20 | Tampering | grid ownership via auto-replies | high | mitigate | `IsUserInput` (oscabsorb.go:209-276) treats CSI with a private marker or intermediate byte, finals `RnctyIOMm`, X10 mouse, OSC/DCS/APC/PM/SOS strings and unterminated sequences as not typed. It runs on the absorber's output and gates only `NoteTyping` (webserver/server.go:965-967, relay/server.go:182-183). Tests `TestIsUserInput` (oscabsorb_test.go:204), `FuzzIsUserInput` (:227), and the replayed-query scene's `checkUndisturbed` (internal/daemon/attach_viewers_test.go). Residual: O-3 | closed |
| T-10-21 | Elevation of privilege | NoteTyping | high | mitigate | The pumps never call it for read-only viewers: both calls sit inside `if !sub.ReadOnly` (webserver/server.go:961-967, relay/server.go:179-183). `NoteTyping` returns for `sub.ReadOnly` (hub.go:567), and for a viewer that left, is revoked or never reported a size (hub.go:573). The lock-free fast path (hub.go:564-566) changes nothing and is reachable only by the current owner, which can never be read-only (both `setOwnerLocked` callers check `ReadOnly` first, hub.go:567, 602). Tests `TestHub_GridTypingReadOnly` (hub_grid_owner_test.go:468), `TestHub_GridTypingNeedsReport` (:459), `TestWebTyping_NoReportNoClaim` (claim_test.go:163) | closed |
| T-10-22 | Denial of service | typing at line rate | medium | mitigate | The owner's keystrokes take two atomic loads and no lock (`ownerFast`/`ownerStale`, hub.go:141-142, 564-566). `setOwnerLocked` sets `ownerFast` (hub.go:589-593), so a non-owner pays the locked path once per ownership change | closed |
| T-10-23 | Denial of service | attach SIGWINCH storm | low | accept | One claim frame per SIGWINCH (attach_unix.go `watchResize`); the hub clamps and rebuilds only on a size change (hub.go:605, 649). See AR-10-05 | closed (accepted at plan time) |
| T-10-24 | Tampering | host machine (10-07) | high | mitigate | 10-VM-CHECKS.md line 13 and every Cleanup section (lines 48, 87, 121, 144): only the gate, builds, `tar`, `shasum`, `codesign -dv`, `<bin> version` and SSH ran on the host; every kill, `kill -STOP`/`-CONT`, pf block, click and key event ran inside a VM; the host's Portty was not touched; builds are `Signature=adhoc` | closed |
| T-10-25 | Information disclosure | 10-VM-CHECKS.md | high | mitigate | Line 12: links moved in 0600 files over an SSH pipe and were never printed; share checks record HTTP codes only; payloads are random base64. Every Cleanup section lists the deleted link files, with no `*.url` left (lines 43, 82, 117, 140). The cap/token/tskey/query-URL grep over the whole phase directory finds 0 matches | closed |
| T-10-26 | Information disclosure | Safari data in the home VM | medium | mitigate | `safclean.sh` ran after every Safari row: WebDriver session ended, SafariAutomation dir removed, favicon rows deleted, token search (10-VM-CHECKS.md lines 42, 81, 116, 139). One token copy found in Safari's TabSnapshots cache after R-04 was removed and a re-search found 0 (line 81). Residual: O-7 | closed |
| T-10-27 | Denial of service | office VM tailnet node | high | mitigate | Drops were pf `block drop` rules on TCP 7444 in the `com.apple/portty-drop` anchor with `tailscaled` up; `tailscale status` was checked before, during and after each drop (lines 8, 32, 61, 76-77). `tailscale down`, logout and auth keys were never used (lines 8, 86, 120, 143) | closed |
| T-10-28 | Tampering | VM state after faults | low | mitigate | Both `kill -STOP`s (pids 17576, 20442) were followed by `kill -CONT` (lines 36, 67). Each Cleanup restored settings.json (`cmp` clean), installed and launched HEAD, and recorded `/health` (lines 45-46, 84-85, 118-119, 141-142) | closed |
| T-10-29 | Denial of service | office VM share port left blocked | medium | mitigate | Each drop's safety-net flush fired (lines 32, 76-77). Every Cleanup records pf `Status: Disabled` as before the run, the anchor empty, the tokens released and no `drop.sh` or safety-net process left (lines 44, 83) | closed |
| T-10-SC | Tampering | npm/pip/cargo installs (all seven plans) | high | mitigate | `go.mod`, `go.sum`, `frontend/package.json` and `frontend/pnpm-lock.yaml` have no diff since d027e86d. Worktree installs used `pnpm install --frozen-lockfile --offline` (10-02-SUMMARY deviation 3) | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Constraint Checks

| Constraint | Result | Evidence |
|------------|--------|----------|
| Share links stay token-gated and tailnet-only | holds | `capability_mw.go`, `origin_mw.go`, `whois.go`, `internal/capability`, `internal/tailnet` and `selfcert.go`/`tailscale.go` have no diff since d027e86d. Route registration is outside every webserver hunk. The post-subscribe grant re-check is unchanged and still runs before any replay or read pump (webserver/server.go:896-899) |
| CSP unchanged | holds | `frontend/index.html` and `internal/webserver/csp_mw.go` have no diff since d027e86d; no added line in the phase mentions Content-Security-Policy |
| Origin checks unchanged | holds | `requireAllowedOrigin` (origin_mw.go) and the share socket's `OriginPatterns` (server.go:847) are unchanged; `loopbackOriginPatterns` (relay/server.go:238) and the proxy's `LoopbackOriginPatterns(r.Host)` (remote_ws_proxy.go:76) are outside every hunk |
| 32 KiB frame guard unchanged | holds | `MaxInputPayload = 32767` (protocol.go:112); no server-side input reader calls `SetReadLimit` at d027e86d or at HEAD; the proxy's client side keeps the default (remote_ws_proxy.go:71-77) |
| A guest can't do more than its capability allows | holds | Read-only: input discarded before the classifier, `NoteTyping` and the queue (webserver/server.go:961); resize/claim skipped (webserver/server.go:1000) and refused again in the hub (hub.go:567, 602). A ping still gets a pong, as before. Revoked: input dropped (input.go:101-103, 133-136), sizing and typing claims refused (hub.go:573, 609) |
| No new file-read routes | holds | 0 added `HandleFunc`/`Handle(`/`mux.`/file-IO calls in the non-test Go diff; `internal/daemon/api.go` unchanged |

---

## Unregistered Flags

The SUMMARYs raise no `threat_flag` entries: 10-03..10-07 say "None", 10-01 has no section, and 10-02's "Threat surface" maps to T-10-02/03/04. The code review added changes that cross a trust boundary but are in no plan's register. They are logged here as unregistered flags (warnings, not blockers) with the verification the audit ran on each. None is open.

| Flag | Source | Maps to | Verification | Result |
|------|--------|---------|--------------|--------|
| UF-10-01: a revoked share's queued web input reaching the shell | CR-01, CR-02 | none (new); the register had no revoke threat | Both revoke callers clear grants and then call `RevokeWebViewers` (share.go:346-350, 365-367); `ClearGrants` is called nowhere else. `RevokeWebViewers` marks targets `revoked` under `mu` (hub.go:459-467), then `revokeInput` bumps `webEpoch`, marks targets `inputRevoked` and drops every queued web frame (hub.go:472, input.go:141-165). `revoked()` is true for a web conn opened before the last revoke, whether or not its viewer is still subscribed, or whose viewer was a target (input.go:133-136). A pump blocked in `Write` wakes on the revoke's broadcast and its frame is dropped (input.go:93-103). Handler order closes the gap for new viewers: Join (webserver/server.go:883), grant re-check (896-899), then `OpenInput` (935). Local-origin and nil-sub input is never revoked. Tests `TestHubInput_RevokeDropsBacklog`, `…DepartedBacklog`, `…BlockedPumpAfterUnsubscribe` (input_test.go:264, 324, 367); `TestHubInput_CloseKeepsBacklog` (:233) keeps INPT-01 for plain drops. VM: 10-VM-CHECKS RF-01 PASS (main run with the guest tab closed first, and the variant). Residual: O-2 | verified |
| UF-10-02: revoked or departed viewers sizing the grid | IN-09 | T-10-12 | `NoteTyping` and `sizeClient` refuse a viewer that is not subscribed or is `revoked`, under `mu` (hub.go:573, 609). `Unsubscribe` clears ownership (hub.go:358-360). Test `TestHub_GridRevokedOrGoneViewerCannotClaim` (hub_grid_owner_test.go:495) | verified |
| UF-10-03: hub-wide input bound | WR-01, WR-05 | T-10-05 | See T-10-05. Residual: O-1 | verified |
| UF-10-04: input lost or spliced across an upgrade handoff | WR-02 | T-10-07, T-10-08 | The takeover holds every imported hub (`HoldInput`, input.go:171-175) in `commit` before `serveOn`, the share server and `hub.Start` (handoff.go:1274-1276, then 1278-1291). The writer takes no frame while held (input.go:245); `Write` keeps queuing within the quotas, so nothing is dropped; a pump blocks only at its quota, and only until the hold ends. The hold ends on `watchOldExit(oldPID)` (kqueue NOTE_EXIT; ESRCH returns at once, pty/exitwatch_darwin.go:21-45) or after `maxFrozen()` with a log line (handoff.go:1334-1356). Queues are per hub, so no input crosses sessions. The `lost()` path (old daemon already reaped) does not hold. A revoke during the hold still drops held web frames. Test `TestHandoff_TakeoverHoldsInputUntilOldExits` (handoff_test.go:709). VM: 10-VM-CHECKS RF-02 PASS (strictly increasing, no duplicates). Residual: O-2(b) | verified |
| UF-10-05: peer-controlled close reasons forwarded into the local UI | RF-03 | none (new path to an existing forward) | The proxy already forwarded the peer's close code and reason before the phase (d2998ad2 remote_ws_proxy.go). RF-03 only waits up to `peerCloseGrace = 2 s` for the peer copy's end after a failed upstream write and uses it if it is a close frame (remote_ws_proxy.go:155-167, 197). The reason is logged with `%q` (175) and forwarded unchanged (180-183); WebSocket limits it to 123 bytes. Consumers only compare it with the two constants: `isTerminalClose` (closeReasons.ts:13-15), relayClient.ts:337-348, and `attachEnd` (cmd_attach.go:319-334), which prints fixed copy with the locally known host and session names. `SessionEndedBanner` never renders it (SessionEndedBanner.tsx:30-35, 70-76: `console.debug` only). The cap stays server-side and its log redaction is unchanged (remote_ws_proxy.go:110-112). Test `TestRemoteWSProxy_SessionEndedMidPaste` (remote_ws_proxy_test.go:847); VM: RF-03 re-run PASS | verified |
| UF-10-06: `ErrInputClosed` no longer ends the read pump | WR-04 | T-10-06 | See T-10-06 | verified |
| UF-10-07: OS-made window resizes claiming the grid | WR-03, IN-08 | T-10-18 | Focus and visibility gate at TerminalPanel.tsx:1012-1013; residual gaps recorded in the `shortcut:` comment (1004-1007). Test TerminalPanel.scale.test.tsx:713; 10-VM-CHECKS WR-03 re-check PASS | verified |

---

## Observations (outside the register, non-blocking)

These came up while verifying the threats above. None is a missing declared mitigation, and none was given a threat ID.

- **O-1 (residual on T-10-05):** the hub bound scales with the number of connections. Once the hub holds `InputHubMaxBytes`, every connection may still queue up to `InputConnHubAllowance` (64 KiB), a closed connection keeps its frames until they drain, and a reconnect gets a fresh allowance (input.go:39-46, 93-97). Nothing caps the number of share connections. With a PTY that is not reading, a write-cap holder who reconnects repeatedly can grow one hub by about 64 KiB per connection past 64 MiB. Only write-cap holders (read-only input is discarded first, webserver/server.go:961) and loopback clients can do this, and a write-cap holder already has a shell, so it is no escalation. The code comment states the bound. A per-session connection cap or a hard ceiling would close it if the office daemon's memory ever matters.
- **O-2 (residuals on UF-10-01):** (a) The one frame already in the PTY write (at most 32,767 bytes per hub) and what the kernel tty already buffered still reach the shell after a revoke. This is documented (hub.go:468-471) and is the same as before Phase 10. (b) After COMMITTED, the old daemon keeps writing its own queued web backlog for up to `restartDrain` (1 s), while the takeover already serves the API (handoff.go:770-786). A Stop Sharing clicked on the takeover in that window drops the takeover's held queue but cannot reach the old process's queue. The old process exits by the deadline and drops the rest, so the exposure is at most about 1 s of slow-PTY writes during an upgrade.
- **O-3 (residual on T-10-20):** `IsUserInput` counts any non-ESC byte and any ESC + character as typing (oscabsorb.go:212-232). A raw terminal behind `portty attach` that answers ENQ with a non-empty answerback string, or a VT52-mode identify (`ESC / Z`), would therefore claim the grid. xterm.js (GUI and browser) sends neither. Only grid ownership is affected, and only for viewers that may already write. The documented split-reply `shortcut:` (oscabsorb.go:205-208) is similar.
- **O-4 (register text):** T-10-05's declared mitigation (a per-connection quota only) predates review WR-01, which found no hub-wide bound. The hub cap and allowance (WR-01, WR-05) are what make the DoS limit hold, so the register row should cite them in future copies.
- **O-5 (register text):** the register has no threat for "revocation must stop queued input". The phase's own design (the writer outlives the socket) created it and review CR-01/CR-02 caught it (UF-10-01). Future threat models that add buffering behind an auth boundary should register "revocation drains or drops the buffer" up front.
- **O-6 (register text):** T-10-06 says "the pump exits" on `ErrInputClosed`. Since WR-04 the pump ignores it and keeps reading, and the write pump's `hub.Done` branch ends the connection. The threat is still closed, but the row's wording is out of date.
- **O-7 (VM rig, residual on T-10-26):** `safclean.sh` does not clear Safari's `TabSnapshots` cache, where one share-token copy was found after R-04 and removed by hand (10-VM-CHECKS.md line 81). Add it to the script before the next Safari run.

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-10-01 | T-10-03 | `RemoteCapStore` and the proxy route stay keyed by session id alone. A collision needs a hostile peer on the same Tailscale account, and ids are 16 random bytes (Research Pitfall 12). Out of scope for INPT-04. `remote_caps.go` is unchanged | 10-02 plan-time threat model (low severity) | 2026-11-20 |
| AR-10-02 | T-10-08 | An upgrade during a multi-MB slow paste can lose bytes still queued in the old daemon after a pong said they were queued, so no notice shows. `finish` waits within `restartDrain`, logs `input still queued at exit` with byte counts (handoff.go:806), and the `shortcut:` comment is at handoff.go:781-785 (CONTEXT "Accepted limits") | 10-03 plan-time threat model (low severity) | 2026-11-20 |
| AR-10-03 | T-10-10 | Owed pongs are written by the viewer's own write pump. A client that pings without reading blocks only its own pump; it is kicked as slow when `sub.Msgs` fills, and share viewers are also dropped by the WebSocket heartbeat | 10-03 plan-time threat model (low severity) | 2026-11-20 |
| AR-10-04 | T-10-14 | A write-cap holder already has the shell (Research A3). Each claim costs at most one emulator rebuild, only on a real size change (hub.go:649), and clients debounce. A per-connection token bucket can be added if a review asks | 10-04 plan-time threat model (low severity) | 2026-11-20 |
| AR-10-05 | T-10-23 | One claim frame per SIGWINCH; the signals come from the user's own terminal resizes; the hub clamps and rebuilds only on a size change | 10-06 plan-time threat model (low severity) | 2026-11-20 |

All five were accepted in the plans' own threat models. They are low severity (below the `high` block threshold), and this audit did not newly accept any of them.

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-11-21 | 30 | 30 | 0 | gsd-security-auditor (retroactive /gsd-secure-phase), ASVS L1 with the high-severity input, grid and revoke controls checked at the boundary; code pinned at 4d9b78da; 7 unregistered flags verified, 0 open |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-11-21
