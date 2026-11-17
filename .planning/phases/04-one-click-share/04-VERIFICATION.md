---
phase: 04-one-click-share
verified: 2026-11-17T08:45:00Z
status: human_needed
score: 5/5 roadmap success criteria verified (automatable evidence); 6/6 requirements satisfied
covered_files: [".github/workflows/build.yml", ".planning/phases/04-one-click-share/04-01-PLAN.md", ".planning/phases/04-one-click-share/04-01-SUMMARY.md", ".planning/phases/04-one-click-share/04-02-PLAN.md", ".planning/phases/04-one-click-share/04-02-SUMMARY.md", ".planning/phases/04-one-click-share/04-03-PLAN.md", ".planning/phases/04-one-click-share/04-03-SUMMARY.md", ".planning/phases/04-one-click-share/04-04-PLAN.md", ".planning/phases/04-one-click-share/04-04-SUMMARY.md", ".planning/phases/04-one-click-share/04-05-PLAN.md", ".planning/phases/04-one-click-share/04-05-SUMMARY.md", ".planning/phases/04-one-click-share/04-06-PLAN.md", ".planning/phases/04-one-click-share/04-06-SUMMARY.md", "TESTING.md", "app.go", "cmd_attach.go", "cmd_share.go", "frontend/src/App.tsx", "frontend/src/components/LinkNoLongerWorks.tsx", "frontend/src/components/SharePopover.tsx", "frontend/src/components/Sidebar.tsx", "frontend/src/components/TabBar.tsx", "frontend/src/components/TerminalPanel.tsx", "frontend/src/lib/closeReasons.ts", "frontend/src/lib/shareCopy.ts", "go.mod", "internal/daemon/api.go", "internal/daemon/peer_connect.go", "internal/daemon/remote_ws_proxy.go", "internal/daemon/share.go", "internal/relay/close_reasons.go", "internal/relay/hub.go", "internal/tailnet/sessions.go", "internal/webserver/server.go", "internal/webserver/whois.go", "tests/check-share-removal.sh", "web/dashboard.html", "web/link-invalid.html"]
covered_digest: "v4:sha256:1fadc977813ce0fc0cea30ef183f09fe571d784c99b4edbe482a278c90cfa832"
behavior_unverified: 0
overrides_applied: 0
human_verification:
  - test: "M-80 two-Mac flow, part 1: on the office Mac turn Share on for a shell session; on the home Mac (same Tailscale account) wait up to 30 s, click the row under Other Macs"
    expected: "The session is listed; one click opens a `<host> › <session>` tab with no prompt or code and full control. A browser on another tailnet device opens the copied link with full control."
    why_human: "Needs two Macs on a live tailnet. Tests use a fake WhoIs client and a loopback share server."
  - test: "M-80 part 2: from Warp on the home Mac run `portty attach office-air:<session>`, then bare `portty attach office-air` with one and with several sessions shared"
    expected: "Attach shows the banner and the tmux-style status bar; with one share it attaches at once, with several it lists them and exits 0."
    why_human: "Needs a real peer and a TTY. Resolution and error copy are unit-tested (TestPeerConnect_*, TestCmdAttach_Resolution); the status-bar path is shared with local attach."
  - test: "M-80 part 3: with the home GUI tab, a browser and `portty attach` all open, turn Share off on the office Mac"
    expected: "GUI tab keeps scrollback, stops input, shows `office-air stopped sharing this session.` and the row disappears; the browser shows the same banner without reconnecting; the CLI restores the terminal, removes the bar, prints `Portty: office-air stopped sharing <session>.` and exits 0."
    why_human: "The real WebSocket close travels host webserver -> daemon proxy -> client across two machines. Each hop has a unit test (TestShare_OffClosesViewerWithReason, TestRevoke_*, TestRemoteAttachEnd, TerminalPanel.guestClose.test.tsx), but the end-to-end path was not run."
  - test: "M-80 part 4 (WR-05 / WR-06): share again after a stop and click the peer row; if possible, test with two Macs whose HostNames collide"
    expected: "The stopped tab reconnects with a new connection instead of reopening the dead tab; the connect goes to the Mac picked by its MagicDNS name."
    why_human: "The review fix report marks WR-06 as 'fixed: requires human verification'. It is UI state on a live peer."
  - test: "M-80 part 5: a Mac signed in to a different Tailscale account"
    expected: "It shows no group for the office Mac, and `portty attach office-air` prints the didn't-allow-access line."
    why_human: "Needs a second Tailscale account. The 403 path is covered with a fake (TestWhoIs_MetaRefused, TestPeerConnect_Forbidden)."
  - test: "M-77: run `tailscale serve status` on both Macs"
    expected: "`No serve config`. If any entry is left from an older build, run `tailscale serve reset`."
    why_human: "The serve config lives in tailscaled state on the real machines. Portty no longer touches it."
  - test: "M-82: on a Mac with Tailscale, disconnect Tailscale while a session is open, then reconnect"
    expected: "Share on is dimmed with its reason in the toolbar, popover and row menu. Stop Sharing still works for a shared session. Everything works again after reconnecting."
    why_human: "Needs the real Tailscale client. The VM only fakes a connected Tailscale. The states are unit-tested."
---

# Phase 4: One-Click Share Verification Report

**Phase Goal:** Sharing is one on/off switch per session: shared means tailnet peers and a tailnet web link at once, always full control, with a clean stop and CLI attach from any terminal. Funnel and read-only links are gone.
**Verified:** 2026-11-17
**Status:** human_needed
**Re-verification:** No (initial verification)

Every automatable truth checks out, both in the code and in the test runs below. What is left needs two Macs on a live tailnet (M-80), the real serve config (M-77) or a real Tailscale toggle (M-82).

## Goal Achievement

### Observable Truths (ROADMAP success criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | One Share step makes the session visible in the other Mac's sidebar and gives a tailnet-only link to copy. No tabs, modes or separate toggles | VERIFIED (live two-Mac check is human) | `internal/daemon/share.go` `handlePutShare`/`shareOn` mints one `read,write` capability, starts the share server, `EnableSession`+`AddGrant`, returns `https://<fqdn>:7444/sessions/<id>?cap=`. The peer listing `handleSessionsMeta` serves only `webEnabledSessions()` and is wrapped in `requireSameUser`. The GUI, CLI and row menu all call the same `SetSessionShared` (app.go -> client -> PUT). `SharePopover.tsx` has one switch (`SHARE_SWITCH_LABEL`) and is rendered from App.tsx:1126. The old `components/Share/` and `components/SessionShare/` folders are gone. Tests: TestShare_OnReturnsWorkingLink, TestShare_Idempotent, TestWhoIs_MetaSameUser, SharePopover.test.tsx. Human: M-81 PASS in the Tart VM (off/on states, link plus Copy plus warning line, blue "Shared" button, sidebar signal icon, clean turn-off). |
| 2 | The link opens a working interactive terminal with full control | VERIFIED | TestShare_OnReturnsWorkingLink: GET on the link redirects to /app/, and a WebSocket with the link's cap echoes typed input from the PTY (run again uncached, PASS). The write perm comes from the signed capability (`HasPerm(claims.Perms, "write")`, server.go). Human: M-79 PASS. The link returns 302 into the web terminal in the VM. Opening it from a second tailnet device is part of M-80. |
| 3 | Share off disconnects every remote viewer (peer GUI, browser, CLI) and removes the session from peers' sidebars | VERIFIED (live end-to-end is human) | `unshareLocked` clears grants, disables the session, calls `hub.RevokeWebViewers(relay.CloseReasonShareStopped)` and stops the server when nothing is left. Every remote viewer (browser, peer GUI, and the CLI through the daemon proxy) subscribes through `handleWSSRelay` with `Origin: "web"`, and a post-subscribe `isGrantActive` re-check closes the subscribe race. Exit and kill use the same path (api.go:342, handleDeleteSession). Clients: `remote_ws_proxy.go` forwards the upstream CloseError. `cmd_attach.go` `remoteAttachEnd` prints the locked line and returns nil. TerminalPanel branches on `CLOSE_REASON_SHARE_STOPPED` to show the banner and `dropPeerSession`. Tests: TestShare_OffClosesViewerWithReason, TestShare_OffDisconnectsWebViewersOnly, TestShare_OffStopsServerAndNewLinkDiffers, TestRevoke_* (5), TestRemoteAttachEnd, TerminalPanel.guestClose.test.tsx (all PASS). Human: M-79 PASS. After one share is turned off while another stays on, the old link returns 403 "Link No Longer Works". |
| 4 | No Funnel, public-link, risk-panel or read-only code, UI, settings or tests remain | VERIFIED | `bash tests/check-share-removal.sh` gives `OK: share removal gate clean`, and build.yml runs it. My own `git grep -i` for funnel / qrcode / join code / hold-to-confirm / risk panel / InternetReadOnly / --readonly / SetRWGate / isRWGated / shell share warning outside `.planning`, the gate script and the `*_removed_test.go` pins found hits only in TESTING.md Section 2 history notes (lines 40-209). Sections 4-5 are clean. go-qrcode is absent from go.mod and go.sum. The remaining `ReadOnly` hits are the capability write-perm gate, kept on purpose (R-RO-1). Removed routes are pinned to 404/405 (TestShareRemoved_WebServerRoutes, TestShareRemoved_DaemonRoutes). No StartWebServer/LAN fallback/serve/web bindings remain. |
| 5 | `portty attach <host>:<session>` from any terminal attaches to a session shared from another Mac and shows the status bar | VERIFIED (live peer is human) | `cmdAttach` resolves local names first, then `parseRemoteID`, and `attachRemote` calls `client.ConnectPeer` -> POST /peers/connect (`peer_connect.go`: host from tailscaled's peer list, WhoIs-gated meta and cap fetch, `RemoteCapStore.Put`). It then dials `ws://127.0.0.1:<relay>/api/relay/remote/<sid>/ws` through `runAttach`, which builds the same `statusbar.New` as local attach when stdout is a TTY. Tests: TestPeerConnect_* (12), TestRemoteRelayURL, TestCmdAttach_Resolution, TestPeerConnectMessage (PASS, uncached). |

**Score:** 5/5 truths verified (0 present-but-behavior-unverified). Each truth's live cross-machine path is also listed under Human Verification.

### Plan-level must-haves (merged)

All plan truths from 04-01..04-06 trace into the five rows above. Spot checks beyond the table:
- The vmpreview fake is build-tag gated, and the brand gate keeps the tag out of .github, build.sh and scripts (`go vet -tags vmpreview ./...` gives exit 0, check-brand.sh is clean).
- The daemon starts no server at boot (TestShare_NoServerAtBoot). Tailscale off fails with 503 (TestShare_TailscaleOff). HTTPS off fails with 409 (TestShare_HTTPSOff).
- Key regeneration re-mints links (TestShare_KeyRegenRemintsLinks). Concurrent first shares start one server (TestShare_ConcurrentFirstSharesStartOneServer, WR-02). A session killed during start is not shared (TestShare_SessionKilledDuringStartIsNotShared, WR-04).
- Peer cap route: POST-only, JSON-only, 404 when not shared (TestPeerCap_Routes, TestPeerCap_NoProviderIs404). A local tailscaled fault answers 503, not 403 (TestWhoIs_LocalFaultIs503, WR-08).
- Referrer-Policy: no-referrer (TestCSPHeaders_ReferrerPolicyNoReferrer). The Link No Longer Works server page has its own test (link_page_test.go) and so does the in-app block (LinkNoLongerWorks.test.tsx, App.webLink.test.tsx).
- `portty share` / `unshare` copy: TestCmdShare_PrintsLinkAndCTOnFirstShare, TestCmdShare_Errors, TestCmdUnshare.

### Required Artifacts

| Artifact | Status | Details |
|----------|--------|---------|
| internal/webserver/share_removed_test.go, internal/daemon/share_removed_test.go | VERIFIED | `TestShareRemoved_*` present and passing |
| web/dashboard.html | VERIFIED | contains the "To open a session here, use its link." paragraph |
| internal/daemon/share.go, share_test.go, share_vmpreview.go, tailscale_vmpreview.go, cmd_share.go | VERIFIED | substantive, wired (routes registered, App bindings, CLI) |
| internal/webserver/whois.go, internal/relay/close_reasons.go, web/link-invalid.html, internal/webserver/revoke_test.go | VERIFIED | `requireSameUser` wraps meta and peer-cap routes |
| internal/daemon/peer_connect.go, peer_connect_test.go, internal/tailnet/sessions.go | VERIFIED | `MetaForbidden` drops other-account peers |
| frontend/src/components/SharePopover.tsx, __tests__/SharePopover.test.tsx, lib/shareCopy.ts | VERIFIED | The gsd-tools artifact check flagged "Missing pattern: Share this session" in SharePopover.tsx. That is a false positive: the label lives in `shareCopy.ts` as `SHARE_SWITCH_LABEL` and is rendered at SharePopover.tsx:261 |
| tests/check-share-removal.sh, frontend/src/lib/closeReasons.ts, frontend/src/components/LinkNoLongerWorks.tsx | VERIFIED | Present and wired. The gate runs in build.yml |

### Key Link Verification

The gsd-tools key-link check could not parse most `from:` fields, because they hold descriptive text instead of a bare path. I checked every pattern by hand with grep instead:

| From | To | Pattern | Status |
|------|----|---------|--------|
| api.go exit/kill | ClearGrants/DisableSession | now through `unshareLocked` (share.go:290-291), called from api.go:342 and handleDeleteSession | WIRED (refactored; the literal `ws.ClearGrants(sessionID)` no longer appears in api.go) |
| server.go handleWSSRelay | capability HasPerm | `HasPerm(claims.Perms, "write")` | WIRED |
| cmd_share.go | PUT /sessions/{id}/share | `SetSessionShared(` x2 | WIRED |
| app.go | client.SetSessionShared | `func (a *App) SetSessionShared` | WIRED |
| share.go | EnableSession + AddGrant | `AddGrant(` x3 | WIRED |
| api.go handleListSessions | shares map | `WebEnabled` | WIRED |
| share.go share-off | hub.RevokeWebViewers | `RevokeWebViewers(relay.CloseReasonShareStopped)` | WIRED |
| handleWSSRelay | isGrantActive | `isGrantActive(claims.SID, claims.GrantID)` | WIRED |
| share.go | SetShareTokenProvider | present | WIRED |
| meta route | requireSameUser | `requireSameUser(ws.handleSessionsMeta)` | WIRED |
| cmd_attach.go | /api/relay/remote/ | present | WIRED |
| remote_ws_proxy.go | close forward | `errors.As(err, &ce)` (passed by the tool) | WIRED |
| peer_connect.go | FetchPeerCap | passed by the tool | WIRED |
| app.go | ConnectPeerSession | present; Wails stub at App.js:85 | WIRED |
| SharePopover.tsx | SetSessionShared | present; stub at App.js:82 | WIRED |
| Sidebar.tsx | App.tsx onShareToggle | 4 / 1 hits | WIRED |
| TabBar.tsx | popover | `aria-haspopup="dialog"` | WIRED |
| App.tsx peer row | ConnectPeerSession | App.tsx:790 | WIRED |
| TerminalPanel.tsx | closeReasons.ts | `CLOSE_REASON_SHARE_STOPPED` x2 | WIRED |
| build.yml | check-share-removal.sh | present | WIRED |

### Data-Flow Trace (Level 4)

| Artifact | Data | Source | Real data | Status |
|----------|------|--------|-----------|--------|
| SharePopover link field | ShareResult.url | GetSessionShare/SetSessionShared -> daemon share record token -> `shareURL(ws,id,token)` | yes | FLOWING |
| Sidebar Shared icon / toolbar state | SessionInfo.webEnabled | sessions poll -> handleListSessions, true exactly when `a.shares[id]` exists | yes | FLOWING |
| Peer sidebar groups | meta sessions | FetchAllPeerSessionsMeta -> host `/api/sessions/meta` -> `webEnabledSessions()`; 403 groups dropped | yes | FLOWING |
| Peer tab origin/cap | PeerConnectResult.url | POST /peers/connect -> FetchPeerCap -> share token provider | yes | FLOWING |

### Behavioral Spot-Checks (ground-truth commands, run by the verifier)

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Go build, vet, race tests | `go build ./... && go vet ./... && go test -race -short ./...` | exit 0; 11 packages ok (some cached), no failures, no flake re-run needed | PASS |
| Uncached share/revoke/whois/peer tests | `go test -race -count=1 ./internal/daemon -run 'TestShare_\|TestShareRemoved_\|TestPeerConnect_'` and `./internal/webserver -run 'TestRevoke_\|TestShareRemoved_\|TestSameUser\|TestWhoIs_\|TestPeerCap_\|TestStop_'` | 29 PASS (daemon), 14 PASS (webserver) | PASS |
| Uncached CLI tests | `go test -race -count=1 . -run 'TestRemoteAttachEnd\|TestRemoteRelayURL\|TestPeerConnectMessage\|TestCmdAttach_\|TestCmdShare\|TestCmdUnshare\|TestResolve'` | 10 PASS | PASS |
| playwrightfixture vet | `go vet -tags playwrightfixture ./cmd/playwright-fixture/` | exit 0 | PASS |
| vmpreview vet | `go vet -tags vmpreview ./...` | exit 0 | PASS |
| TypeScript | `pnpm -C frontend exec tsc --noEmit` | exit 0 | PASS |
| Vitest | `pnpm -C frontend exec vitest run` | 82 files, 1038 tests passed | PASS |
| Removal gate | `bash tests/check-share-removal.sh` | `OK: share removal gate clean` | PASS |
| Brand gate | `bash tests/check-brand.sh` | `OK: brand gate clean` | PASS |
| Traceability | `bash tests/check-traceability-paths.sh` | `OK: 109 traceability paths exist` | PASS |
| go-qrcode absent | `grep -n qrcode go.mod go.sum` | no match (exit 1) | PASS |
| No leaked test daemons | `pgrep -f '\.test daemon'` | no output | PASS |
| Real app-support dir untouched | mtimes of `~/Library/Application Support/portty/{capability.key,settings.json}` before and after; no new files | unchanged (both mtimes predate the runs), no `ct_disclosed` created | PASS |

### Probe Execution

Step 7c: not applicable. The phase declares no `scripts/*/tests/probe-*.sh`. Its gates are the tests/*.sh scripts run above.

### Requirements Coverage

| Requirement | Source Plan | Status | Evidence |
|-------------|-------------|--------|----------|
| SHAR-01 one on/off state, peers plus link | 04-02, 04-03, 04-05, 04-06 | SATISFIED | Truth 1 |
| SHAR-02 always full control, no read-only link | 04-01, 04-02 | SATISFIED | `mintShareRecord` Perms `read,write` only. Read-only link features removed (truth 4) |
| SHAR-03 link gives a working interactive terminal | 04-02, 04-03, 04-06 | SATISFIED | Truth 2 |
| SHAR-04 stop disconnects viewers and removes from sidebars | 04-02, 04-03, 04-04, 04-06 | SATISFIED | Truth 3 |
| SHAR-05 Funnel removed | 04-01, 04-05, 04-06 | SATISFIED | Truth 4 |
| SHAR-06 `portty attach <host>:<session>` with status bar | 04-04 | SATISFIED | Truth 5 |

No orphaned requirements: REQUIREMENTS.md maps exactly SHAR-01..06 to Phase 4.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (phase diff) | - | TBD/FIXME/XXX | none found | Added lines in the phase diff contain no debt markers and no TODO/HACK/PLACEHOLDER |
| internal/daemon/share.go | 242-246 | WR-03 fix has no test (no seam inside the window) | Info | Correct by lock-order reading. Review-fix marks it "requires human verification" |
| internal/daemon/share.go | 307-312 | key regeneration closes viewers with `share stopped` while the share stays on (IN-03, open) | Info | A viewer would see a "stopped sharing" banner on a key regenerate. Rare and cosmetic |
| cmd_attach.go | ~319 | remote attach ending with `session ended` exits 1 with a raw websocket error (IN-05, open) | Info | Only the host-ended-session path. Not part of a success criterion |
| frontend sidebarPeers.ts / dropPeerSession | - | peer groups keyed by HostName; duplicate HostNames would share a key (left open in REVIEW-FIX) | Info | Edge case. The connect itself picks the right Mac (WR-05) |
| TESTING.md | M-79 step 4 | expects "Link No Longer Works" on reloading the old link, but when the turned-off share was the last one the server stops and the reload gets connection refused | Info | Known behaviour, the server-stop choice CONTEXT left to Claude, and confirmed in the VM by the orchestrator. Consider rewording M-79 step 4 so the 403 page case says "while another session stays shared" |

### Human Verification Required

These come from the orchestrator's VM evidence (M-81 PASS, M-79 main case PASS) and from the items the VM cannot reach:

1. **M-80 two-Mac discovery and codeless open.** Share on the office Mac, then click the row on the home Mac. Expected: listed within 30 s, tab opens with no prompt, full control. The browser link works from another tailnet device.
2. **M-80 CLI attach.** From Warp, run `portty attach office-air:<session>` and bare `portty attach office-air`. Expected: status bar shows; one share attaches at once, several are listed.
3. **M-80 stop-sharing banners.** Turn Share off on the office Mac. Expected: the GUI tab banner shows and the row disappears; the browser banner shows with no reconnect; the CLI prints `Portty: office-air stopped sharing <session>.` and exits 0.
4. **M-80 WR-05 / WR-06.** Re-share after a stop and click the row. Expected: it reconnects with a new connection to the right Mac.
5. **M-80 other account.** Expected: no group appears, and the CLI prints the access-refused line.
6. **M-77.** Expected: `tailscale serve status` prints `No serve config` on both Macs.
7. **M-82.** Toggle real Tailscale. Expected: Share on is dimmed with its reason; Stop Sharing still works.

M-78 (CLI `portty share` in the VM on fixed port 7444) was not cited separately. It is covered by TestCmdShare_* plus the orchestrator's M-79 observation (link 302 while shared, connection refused once nothing is shared, which shows nothing listens on 7444). Optional to re-run.

### Gaps Summary

No gaps. All five roadmap success criteria and all six SHAR requirements have code that is substantive and wired, plus passing behavioral tests. Every ground-truth command passed, and nothing touched the real application-support directory. The phase stays at `human_needed` only because the cross-machine paths (M-80), the real tailscaled serve config (M-77) and a real Tailscale disconnect (M-82) cannot be exercised in this environment.

---

_Verified: 2026-11-17_
_Verifier: Claude (gsd-verifier)_
