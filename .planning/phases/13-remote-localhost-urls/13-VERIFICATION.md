---
phase: 13-remote-localhost-urls
verified: 2026-11-22T15:10:00Z
status: passed
score: 4/4 roadmap success criteria verified (43/43 plan must-have truths verified)
covered_files: [".planning/phases/13-remote-localhost-urls/13-01-PLAN.md", ".planning/phases/13-remote-localhost-urls/13-01-SUMMARY.md", ".planning/phases/13-remote-localhost-urls/13-02-PLAN.md", ".planning/phases/13-remote-localhost-urls/13-02-SUMMARY.md", ".planning/phases/13-remote-localhost-urls/13-03-PLAN.md", ".planning/phases/13-remote-localhost-urls/13-03-SUMMARY.md", ".planning/phases/13-remote-localhost-urls/13-04-PLAN.md", ".planning/phases/13-remote-localhost-urls/13-04-SUMMARY.md", ".planning/phases/13-remote-localhost-urls/13-05-PLAN.md", ".planning/phases/13-remote-localhost-urls/13-05-SUMMARY.md", ".planning/phases/13-remote-localhost-urls/13-06-PLAN.md", ".planning/phases/13-remote-localhost-urls/13-06-SUMMARY.md", ".planning/phases/13-remote-localhost-urls/13-07-PLAN.md", ".planning/phases/13-remote-localhost-urls/13-07-SUMMARY.md", "TESTING.md", "app.go", "app_test.go", "frontend/src/components/SessionEndedBanner.tsx", "frontend/src/components/TerminalPanel.tsx", "frontend/src/components/__tests__/SessionEndedBanner.test.tsx", "frontend/src/components/__tests__/TerminalPanel.remotePort.test.tsx", "frontend/src/lib/__tests__/fixtures/localhost-url.json", "frontend/src/lib/__tests__/localhostUrl.test.ts", "frontend/src/lib/localhostUrl.ts", "frontend/src/wailsjs/go/main/App.d.ts", "frontend/src/wailsjs/go/main/App.js", "internal/capability/porttoken.go", "internal/capability/porttoken_test.go", "internal/daemon/api.go", "internal/daemon/api_test.go", "internal/daemon/client.go", "internal/daemon/client_test.go", "internal/daemon/handoff.go", "internal/daemon/handoff_test.go", "internal/daemon/peer_client.go", "internal/daemon/peer_connect.go", "internal/daemon/portforward.go", "internal/daemon/portforward_api.go", "internal/daemon/portforward_e2e_test.go", "internal/daemon/portforward_test.go", "internal/daemon/remote_ws_proxy.go", "internal/daemon/share.go", "internal/daemon/types.go", "internal/relay/hub.go", "internal/relay/pathclick.go", "internal/relay/portprinted.go", "internal/relay/portprinted_test.go", "internal/webserver/capability_mw.go", "internal/webserver/portproxy.go", "internal/webserver/portproxy_revoke_test.go", "internal/webserver/portproxy_test.go", "internal/webserver/server.go"]
covered_digest: "v4:sha256:4fdac2eb80c49207be0d906b9f9bfd5b7dd536674228e4d5facf91a3f8344295"
behavior_unverified: 0
overrides_applied: 0
---

# Phase 13: Remote Localhost URLs Verification Report

**Phase Goal:** A user viewing a remote session can click a `localhost` URL printed on the remote Mac and see that page in their normal browser, proxied over the tailnet, with no way to reach a port they didn't approve.
**Verified:** 2026-11-22
**Status:** passed
**Re-verification:** No, initial verification

Method: the code was read (not the SUMMARYs), the gsd-tools artifact and key-link checks were run on all seven plans, the Phase 13 tests were run by name in this session, the full Go and vitest suites were each run once, and the VM evidence in 13-VM-CHECKS.md was checked against the code it exercises.

## Goal Achievement

### Roadmap Success Criteria

| # | Success criterion | Status | Evidence |
|---|-------------------|--------|----------|
| SC1 | A click on `http://localhost:8000/` or `http://127.0.0.1:8000/` in a home peer tab opens the office page in the home default browser through Portty's proxy over the real tailnet, with paths, query strings and assets; no in-app browser (URL-01) | VERIFIED | Code: TerminalPanel.tsx:959 sends a classified loopback URL from a desktop peer tab to `OpenRemotePort`, then `openLink(res.url)` only on `status === 'open'` (TerminalPanel.tsx:399). app.go `OpenRemotePort` calls `POST /peers/ports` (portforward_api.go:204), which gets a grant and binds a forward. The forward's `Rewrite` (portforward.go:331) carries the target in `X-Portty-Target`; the host proxy (portproxy.go:148-208) rebuilds path and RawQuery byte-exact and dials only `::1`, then `127.0.0.1` (portproxy.go:280). Tests passing now: `TestPortForwardE2E_HostDevServer`, `_WebSocketAndSSE`, `TestPortProxy_PathAndHeaders`, `TestOpenRemotePort`. VM: L-02 (index, `app.js` and `logo.png` 200 at the office server; `sub/page.html?q=a%20b` byte-exact; Safari's WebKit process is the forward's only client), L-05 (WebSocket, SSE, hot reload, a `::1`-only server). |
| SC2 | The first click asks for consent in a macOS-style sheet naming the remote Mac and port; declining opens nothing; approval covers only that Mac and port; the listener binds only to loopback (URL-02) | VERIFIED | Code: `RegenerateKeyModal` with the fixed copy and `Don't Allow` / `Allow` (TerminalPanel.tsx:1367-1384; `initialFocus: 'cancel'`). The daemon answers `consent` and binds nothing without a live approval (portforward_api.go:270). Approvals are keyed `(host, port)` and live only while the session holds the same capability fingerprint (portforward.go:74-88). `bindLoopback` opens only `127.0.0.1:P` and `[::1]:P` (portforward.go:228). Tests: `TestPortForward_ConsentFirst`, `TestPortOpen_Approvals`, `_ApprovalLiveCheck`, `_NoApprovalAfterPortInUse`, and the vitest "consent sheet (URL-02)" suite. VM: L-03 (exact title and body, Return and Esc bind nothing, 8000 needed no second sheet while 8001 asked, `lsof` lists only `127.0.0.1:8001` and `[::1]:8001`). |
| SC3 | The proxy refuses whatever wasn't asked for, before the remote Mac connects anywhere; loopback-only; no port scanning; stopping sharing stops the page; the security review records the threat model (URL-02) | VERIFIED | Code: both host routes sit behind `requireSameUser` (server.go:575-578). Grant order: content type, body, write capability, deny list, `Hub.PortPrinted`, then mint (portproxy.go:79-128). It never dials. Proxy order: port, capability, `VerifyPort` (session, port and grant must all match), deny list, target bytes, `ParseRequestURI`, grant re-check, then the dial (portproxy.go:148-207). Every refusal is `refuseOtherUser`. Port tokens are signed under an HKDF-derived key with `Kind: "port"`, so they never verify as session capabilities and the reverse is also true (porttoken.go). Revocation runs through `revokeSignal`/`stopCh` on the host and `RevokeSession` on the viewer (remote_ws_proxy.go terminal close, peer_connect.go capability change). Tests: `TestPortGrant_Refusals` and `TestPortProxy_Refusals` (tripwire counts 0; another-user rows included), `TestPortGrant_IdenticalOpenOrClosed`, `TestPortToken_*`, `FuzzVerifyPort`, `TestHasURLPort_Table`, `TestPortProxyRevoke_*`, `TestPortForwardE2E_StopSharing`, `_ReshareWithoutStream`, `TestTakeover_FinalizeDeniesRelayPort`. VM: L-04 (identical 403s with `X-Portty: 1` and one normalised hash, tripwire 0, LAN path refused) and L-06 (`outcome=revoked` 50 ms after unshare with no request; consent asked again after a re-share; the no-stream window proxied nothing). 13-SECURITY-REVIEW.md records the contract and threat register with the verdict "approved". 13-SECURITY.md has `threats_open: 0`. |
| SC4 | Local URLs, and non-localhost URLs in remote tabs, work as today (confirm popover, then default browser); existing link tests still pass (URL-03) | VERIFIED | Code: the new branch runs only when `desktopPeerRef.current` (remote and not a browser share view) and `classifyLoopbackUrl` accepts the URI. Otherwise the existing risk check, popover and `openLink` run unchanged (TerminalPanel.tsx:959-978). `internal/webserver/web_links_test.go`, `csp_mw.go` and `frontend/index.html` have no diff since `9574fd49`. Tests run now: all 9 link-related vitest files pass (172 tests), including the unchanged `TerminalPanel.web-links`, `openLink`, `App.webLink`, `WebShareSessionView.peerLinks`, `LinkConfirmPopover` and `urlSafety` suites. VM: L-08 (a local-tab localhost URL opens directly, the punycode popover appears in both tab kinds, `https://example.com/` opens, `https://localhost` is not proxied). |

**Score:** 4/4 success criteria verified (0 present-but-behavior-unverified).

### Plan must-have truths (merged)

Every plan truth maps onto the SCs above. All are VERIFIED by passing named tests or recorded VM rows:

| Plan | Truths | Status | Key evidence |
|------|--------|--------|--------------|
| 13-01 | 5 | VERIFIED | `TestHasURLPort_Table`, `_Banners`, `TestPortPrinted_*` (incl. `_Busy`, `_History`, `_SoftWrap`, `_ANSIPort`, `_AltScreen`); `TestPortToken_*`, `FuzzVerifyPort`; `internal/relay/pathclick_test.go` has no diff and the full relay package passes |
| 13-02 | 9 | VERIFIED | `TestPortForward_*` (21 tests), `TestPortOpen_*` (5), `TestForwardHostAllowed`, `TestClassifyLoopbackURL_Fixture`; `POST /peers/ports` is registered on the socket mux only (`TestPortForward_NotOnRelayHandler`); Stop and handoff call `CloseAll` (api.go:350, handoff.go:406) |
| 13-03 | 6 | VERIFIED | `TestPortForwardE2E_HostDevServer`, `TestPortGrant_*`, `TestPortProxy_*`; the routes are outside `cspHeaders`/`porttyMarker` (server.go:565-578); `verifyCapability` delegates to `verifyCapabilityToken` and the existing capability tests pass |
| 13-04 | 4 | VERIFIED | `TestOpenRemotePort`, `_DaemonOutdated`, `_Codes`, `_NoSecrets`, `_NoDaemon`, `TestOpenRemotePortClient`, `TestHealthInfo`, `TestAPIHealth`; `APIVersion = 5`, `PortProxyAPI = 5`, `PathClickAPI` 4 |
| 13-05 | 5 | VERIFIED | `TerminalPanel.remotePort.test.tsx` (routing, consent, notices, URL-03 suite), `localhostUrl.test.ts` (shared fixture), `SessionEndedBanner.test.tsx` (five port kinds) |
| 13-06 | 5 | VERIFIED | `TestPortForwardE2E_WebSocketAndSSE`, `_StopSharing`, `_ReshareWithoutStream`, `TestPortProxyRevoke_ClearGrants`, `_Stop`, `_KeyRegeneration`, `_ClientGone`; 13-SECURITY-REVIEW.md has the threat register and the tests that pin it |
| 13-07 | 9 | VERIFIED | 13-VM-CHECKS.md L-00..L-08 all PASS; TESTING.md Sections 2, 4 and 5 (Category BD, M-137..M-144) are present; `check-traceability-paths.sh` reports 207 paths OK; the backstop truth (UI-1/UI-2 only in the user's recorded words) is met by the "User decisions" section of 13-VM-CHECKS.md, which quotes "Accept automated-only" and "Check on my Macs" |

Behavior-dependent truths (revocation, cancellation and cleanup) each have a passing behavioural test run in this session, so none is left PRESENT_BEHAVIOR_UNVERIFIED.

### Required Artifacts

All 25 declared artifacts across 13-01..13-07 pass `verify.artifacts` (exists, substantive, contains the declared symbol). They are also wired: `portprinted.go` is called from portproxy.go:111, `porttoken.go` from portproxy.go:116 and :161, `portforward*.go` from api.go:214 and the binding, `localhostUrl.ts` from TerminalPanel.tsx:959, and `OpenRemotePort` from app.go → client.go → `/peers/ports`.

### Key Link Verification

All 25 key links across the seven plans report `verified: true` from `verify.key-links`. They were confirmed by reading the code: the registration at api.go:214, `remoteCaps.Get` per request (portforward.go:339), `RevokeSession` in remote_ws_proxy.go (after `relay.IsTerminalClose`) and peer_connect.go (before `remoteCaps.Put`), `SetDeniedPorts(a.relayPort)` in share.go:295, and `revokeSignal`/`stopCh` in portproxy.go:191-206.

### Data-Flow Trace (Level 4)

| Artifact | Data | Source | Real data | Status |
|----------|------|--------|-----------|--------|
| Consent sheet title | `portConsent.host/port` | `RemotePortResult` from the daemon (`magicDNSLabel(fqdn)`, classified port) | Yes | FLOWING |
| Opened URL | `res.url` | `openedURL(port, rest)` in portforward_api.go | Yes | FLOWING |
| Forward → host | `X-Portty-Cap`, `X-Portty-Port-Token`, `X-Portty-Target` | `remoteCaps.Get(sid)` per request, the grant token, `pr.In.URL.RequestURI()` | Yes | FLOWING |
| Host → dev server | request line and Host | the validated target, `localhost:<P>`, `dialLoopback` | Yes (L-02 server log) | FLOWING |

### Behavioral Spot-Checks (run in this session)

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Build and vet | `GOTOOLCHAIN=go1.26.11 go build ./... && go vet ./internal/... .`; `gofmt -l internal app.go` | clean | PASS |
| Port token and printed check | `go test -race -short ./internal/capability ./internal/relay` | ok (relay 201 s, full package) | PASS |
| Host routes and revoke | `go test -race -short ./internal/webserver -run 'Port\|Capability\|...'`; `-run Port -v` lists 13 port tests | ok | PASS |
| Viewing forward, e2e, takeover deny | `go test -race -short ./internal/daemon -run 'PortForward\|ClassifyLoopback\|Takeover_FinalizeDeniesRelayPort\|...'`; `-run 'TestPortOpen_\|TestForwardHostAllowed'` | ok (24 + 6 named tests PASS) | PASS |
| App binding | `go test -race -short . -run OpenRemotePort` | 5 tests PASS | PASS |
| Link and consent UI | `npx vitest run` on the 9 link, consent and banner files | 172/172 | PASS |
| Full vitest (once) | `npx vitest run` | 110 files / 1674 tests | PASS |
| Full Go (once) | `go test -race -short -count=1 ./...` | rc 0: 14 packages `ok`, no FAIL, DATA RACE or panic line (includes `internal/pty` at HEAD, after the review fixes) | PASS |
| Types | `npx tsc --noEmit` | clean | PASS |
| Traceability | `bash tests/check-traceability-paths.sh` | `OK: 207 traceability paths exist` | PASS |

### Probe Execution

Step 7c: SKIPPED. No phase-declared or conventional `scripts/*/tests/probe-*.sh` probes. The VM rows in 13-VM-CHECKS.md are the runtime evidence.

### Review fixes after the VM run: do they need VM re-evidence?

CR-01 and WR-01..WR-04 (32a2b551..82811b55) landed after the B13/HEAD builds the VM rows used. Judgment: **the unit and in-process e2e tests are enough. No VM re-run is needed.** Each fix leaves every positive path the L-rows exercised unchanged, or covers it with an in-process test against the real share server:

- **CR-01** (relay port denied on a takeover daemon) only adds a refusal. L-04's relay-port probe was refused anyway by the printed check, so the VM could not have seen the bug or the fix. `TestTakeover_FinalizeDeniesRelayPort` drives the real `finalize` and the real share server and passes.
- **WR-01** (target byte filter) refuses only bytes `<= 0x20` or `>= 0x7f`. Browsers percent-encode these, and Go's `RequestURI()` re-escapes the path. The L-02 target `/sub/page.html?q=a%20b` is pure printable ASCII. The new `TestPortProxy_Refusals` rows pass.
- **WR-02** (Origin check on non-navigation requests) refuses only non-loopback Origins and `null`. Every L-02/L-05/L-06 request came from a page on `localhost:<P>`, so its Origin was loopback or absent. `TestPortForward_CrossSite` proves that loopback Origins on any port, `[::1]` and a cross-origin navigation still reach the stub.
- **WR-03** (re-grant once on a marked 403) changes the path behind L-06's no-stream window. `TestPortForwardE2E_ReshareWithoutStream/NoConnectStoppedPage` runs that exact sequence through the real share server and passes: old capability, host 403, re-grant refused, stopped page, forward closed, dev server untouched. `TestPortForward_TransientRefusalKeepsForward` pins the new single-miss branch. The fixer flagged this as "requires human verification". A real WhoIs miss cannot be produced on demand on the VMs either, so a VM re-run would add nothing the e2e test lacks.
- **WR-04** (rebuild the forward when the host address changed) keeps the unchanged-address path the VMs used. `TestPortForward_ReclickRebuildsOnNewAddress` covers both branches.

### Requirements Coverage

| Requirement | Source plans | Description | Status | Evidence |
|-------------|--------------|-------------|--------|----------|
| URL-01 | 13-02, 13-03, 13-04, 13-05, 13-06, 13-07 | Clicking `http://localhost:PORT` / `127.0.0.1:PORT` in a remote session opens the remote page in the normal browser, proxied over the tailnet, WebSocket upgrades included so hot reload works | SATISFIED | SC1 evidence; L-02, L-05 (WebSocket, SSE and a WebSocket-driven reload `load:2` within 1.2 s); e2e WebSocket/SSE tests |
| URL-02 | 13-01..13-07 | Token-gated, tailnet-only, consent per port, loopback-only bind, no port scanning | SATISFIED | SC2 and SC3 evidence; L-03, L-04, L-06, L-07 |
| URL-03 | 13-05, 13-07 | Local URLs keep today's behaviour: confirm popover, then default browser | SATISFIED | SC4 evidence; L-08; the unchanged link suites pass |

All three IDs declared in PLAN frontmatter appear in REQUIREMENTS.md (lines 41-43, traceability rows 90-92 mapped to Phase 13). REQUIREMENTS.md maps no other ID to Phase 13, so none is orphaned. The REQUIREMENTS.md checkboxes still read Pending. Updating them is the orchestrator's phase-completion step, not a verification gap.

### Accepted user decisions (not gaps, not open items)

Recorded in the user's own words in 13-VM-CHECKS.md "User decisions" (2026-11-22):

- **UI-1** (a different tailnet user is refused): "Accept automated-only". Pinned by the "another user" rows in `TestPortGrant_Refusals` and `TestPortProxy_Refusals`, both run and passing here. TESTING.md M-139.
- **UI-2** (a real Vite or webpack HMR project): "Check on my Macs", TESTING.md M-140. URL-01's hot-reload clause is already shown by L-05's WebSocket-driven reload. Real Vite is extra assurance the user owns in the manual regression checklist.
- **G-1** (`internal/pty` `TestKillClean_NoOrphans` flake in the 13-07 gate): accepted as the known Phase 12 flake. `internal/pty` has no diff since `9574fd49`, and it passed in this session's full run.

### Anti-Patterns Found

No `TBD`, `FIXME`, `XXX`, `TODO`, `HACK` or `PLACEHOLDER` markers in any of the 35 implementation, test and fixture files the phase changed. No stubs: every handler reaches a real store, dial or proxy.

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| internal/daemon/portforward.go | 414 | WR-02 refuses `Origin: null` and non-loopback Origins on non-navigation requests | Info | A dev app that talks to its own server from a sandboxed iframe (Origin `null`), or an OAuth `form_post` callback in a browser that sends no `Sec-Fetch-Mode`, gets the forward's 403. This is the secure choice. If it bites, M-140 on the user's Macs will show it. |
| .planning/phases/13-remote-localhost-urls/13-SECURITY-REVIEW.md | 35, 75 | The narrative still describes only the `Sec-Fetch-Site` check; the WR-02 Origin check is recorded in 13-SECURITY.md (T-13-07) but not in the review text | Info | Documentation lag only. The code and 13-SECURITY.md agree. |
| internal/daemon/portforward.go | 305-320 | WR-03 race: a stop, re-share and peer reconnect while a re-grant is in flight lets the forward continue under the new share without asking again | Info | Recorded as an accepted residual in 13-SECURITY.md line 107. Same user, same Mac, same port, and it needs a sub-second race. |

### Human Verification Required

None. Every success criterion has code evidence, passing named tests and a PASS VM row. The remaining manual items (M-139, M-140) are user-accepted, as recorded above.

### Gaps Summary

No gaps. The phase goal is met in the code. A printed loopback URL in a desktop peer tab goes through the classifier, the binding, the viewing daemon's consent-gated loopback-only forward, and the host's same-user, write-capability, port-token-gated proxy to the host's own loopback. Every refusal is identical and decided before any dial. The anti-scan printed check and the per-(Mac, port) consent are enforced. Forwards end with the share, without a request. Local and non-localhost link behaviour is unchanged. The five review fixes after the VM run are covered by unit and in-process e2e tests and need no VM re-evidence.

---

_Verified: 2026-11-22_
_Verifier: Claude (gsd-verifier)_
