---
phase: 13-remote-localhost-urls
verified: 2026-11-23T19:30:00Z
status: passed
score: 4/4 roadmap success criteria verified (43/43 plan must-have truths verified)
covered_files: [".planning/phases/13-remote-localhost-urls/13-01-PLAN.md", ".planning/phases/13-remote-localhost-urls/13-01-SUMMARY.md", ".planning/phases/13-remote-localhost-urls/13-02-PLAN.md", ".planning/phases/13-remote-localhost-urls/13-02-SUMMARY.md", ".planning/phases/13-remote-localhost-urls/13-03-PLAN.md", ".planning/phases/13-remote-localhost-urls/13-03-SUMMARY.md", ".planning/phases/13-remote-localhost-urls/13-04-PLAN.md", ".planning/phases/13-remote-localhost-urls/13-04-SUMMARY.md", ".planning/phases/13-remote-localhost-urls/13-05-PLAN.md", ".planning/phases/13-remote-localhost-urls/13-05-SUMMARY.md", ".planning/phases/13-remote-localhost-urls/13-06-PLAN.md", ".planning/phases/13-remote-localhost-urls/13-06-SUMMARY.md", ".planning/phases/13-remote-localhost-urls/13-07-PLAN.md", ".planning/phases/13-remote-localhost-urls/13-07-SUMMARY.md", "TESTING.md", "app.go", "app_test.go", "frontend/src/components/SessionEndedBanner.tsx", "frontend/src/components/TerminalPanel.tsx", "frontend/src/components/__tests__/SessionEndedBanner.test.tsx", "frontend/src/components/__tests__/TerminalPanel.remotePort.test.tsx", "frontend/src/lib/__tests__/fixtures/localhost-url.json", "frontend/src/lib/__tests__/localhostUrl.test.ts", "frontend/src/lib/localhostUrl.ts", "frontend/src/wailsjs/go/main/App.d.ts", "frontend/src/wailsjs/go/main/App.js", "internal/capability/porttoken.go", "internal/capability/porttoken_test.go", "internal/daemon/api.go", "internal/daemon/api_test.go", "internal/daemon/client.go", "internal/daemon/client_test.go", "internal/daemon/handoff.go", "internal/daemon/handoff_test.go", "internal/daemon/ipc.go", "internal/daemon/peer_client.go", "internal/daemon/peer_connect.go", "internal/daemon/portforward.go", "internal/daemon/portforward_api.go", "internal/daemon/portforward_e2e_test.go", "internal/daemon/portforward_test.go", "internal/daemon/remote_ws_proxy.go", "internal/daemon/share.go", "internal/daemon/types.go", "internal/relay/history.go", "internal/relay/hub.go", "internal/relay/pathclick.go", "internal/relay/portprinted.go", "internal/relay/portprinted_test.go", "internal/tailnet/sessions.go", "internal/webserver/capability_mw.go", "internal/webserver/portproxy.go", "internal/webserver/portproxy_revoke_test.go", "internal/webserver/portproxy_test.go", "internal/webserver/server.go"]
covered_digest: "v4:sha256:8ec448e0f22a1a9de8490c287a1c5010803bcc89b82b95c87628d7376774b97e"
re_verification: true
previous_status: passed
previous_digest: "v4:sha256:26d6292015dbb313ddf90f0eeeac42129ca86fc3d9b5c9d70c280b84f9c55641"
behavior_unverified: 0
overrides_applied: 0
---

# Phase 13: Remote Localhost URLs Verification Report

**Phase Goal:** A user viewing a remote session can click a `localhost` URL printed on the remote Mac and see that page in their normal browser, proxied over the tailnet, with no way to reach a port they didn't approve.
**Verified:** 2026-11-23
**Status:** passed
**Re-verification:** Yes. The previous pass (at 52d960a5, status passed) went stale because Phase 15 "v1.2 Hardening" edited files the report covers. This pass re-checks the goal and every success criterion against HEAD (c818e1e1, branch gsd/v1.2).

Method: `git diff 52d960a5 HEAD` over every covered file; each changed hunk read in full; the callers and dependents of the changed code traced; the Phase 13 tests run by name only (`-count=1`, no full suites: other verifiers ran in parallel; the orchestrator supplied full Go `-race` 14/14 and full vitest 122 files / 1826 tests on HEAD). The earlier VM evidence (13-VM-CHECKS.md L-00..L-08) is carried forward as verified, because Phase 15 changed none of the behaviour those runs checked (see below).

## Changes since the previous verification (Phase 15)

Of the 48 files Phase 15 changed outside `.planning`, only five are in the Phase 13 covered set: `TESTING.md`, `frontend/src/components/TerminalPanel.tsx`, `internal/daemon/handoff.go`, `internal/daemon/handoff_test.go`, `internal/relay/hub.go`. Four more are dependencies of Phase 13 code and were read for effect: `internal/relay/history.go`, `internal/tailnet/sessions.go`, `internal/daemon/ipc.go`, `frontend/src/App.tsx` (plus `pathLinks.ts`, `closeReasons.ts`). They are now in the fingerprint.

| File | Phase 15 change | Effect on Phase 13 |
|------|-----------------|--------------------|
| `internal/relay/hub.go` + `history.go` | On ED 3 the hub calls `h.history.Skip(emu.ScrollbackLen())`, which only adds to `History.pushed` (row numbering stays monotonic after `clear`) | None on the anti-scan check. `Hub.PortPrinted` reads the same `pathSnapshot`; `Skip` changes no retained text. `TestPortPrinted_*` (incl. `_History`, `_AltScreen`, `_SoftWrap`, `_Busy`), `TestPathClick_*` and `TestCwd_*` pass at HEAD. |
| `frontend/src/components/TerminalPanel.tsx` | New optional `onSessionEnded` prop and ref; `onClose` calls it only when `reason === CLOSE_REASON_SESSION_ENDED` | None. The peer-tab loopback branch is unchanged (TerminalPanel.tsx:1027-1032: `desktopPeerRef.current ? classifyLoopbackUrl(uri) : null`, then `runRemotePortRef`; every other URL falls through to the risk check, popover and `openLink`). `desktopPeerRef` (:253, :458) and `OpenRemotePort` (:444) are untouched. |
| `frontend/src/App.tsx`, `closeReasons.ts`, `WebShareSessionView.tsx` | Ended-tab wiring: `markPeerSessionEnded`, `sawEnded` passed to `peerConnectVerdict`; `update:show` listener re-shows the update banner | None. They touch the peer tab's reconnect verdict and the update banner only. The port notices, `OpenRemotePort`, and the consent sheet are not read or written. `App.open-remote`, `closeReasons` and `TerminalPanel.guestClose` suites pass. |
| `frontend/src/lib/pathLinks.ts` | `mouseleave` now also runs `invalidate()` | None. This is the Cmd-click file-path provider (Phase 12), separate from the `WebLinksAddon` handler Phase 13 hooks. `pathLinks` suites (43 tests) pass. |
| `internal/tailnet/sessions.go` | New `MetaRefused` status for ECONNREFUSED on the meta probe; the sidebar treats it as reachable with nothing shared | None on the grant path. The one daemon consumer, `peer_connect.go:168` (unchanged), still maps ECONNREFUSED to `not_sharing` because `MetaRefused` still returns the transport error and `status != MetaOK`. The `RevokeSession`-before-`remoteCaps.Put` ordering is intact. `./internal/tailnet` and `PeerConnect` tests pass. |
| `internal/daemon/handoff.go`, `ipc.go`, `socket.go` | Takeover trusts the kernel's LOCAL_PEERPID over HELLO-OK's pid (a mismatch or lookup failure refuses); daemon socket chmod 0600; parent dir 0700 | None on the forward or grant. The `POST /peers/ports` route stays on the socket mux only; 0600 keeps the same-user app and CLI working. The pid check could in principle refuse a real upgrade (the path L-01 used), but Phase 15's M-155 ran `TestHandoffIT` and a real pre-phase daemon upgraded twice in the VM and recorded that the pid check accepted a genuine old daemon with no mismatch in `daemon.log`. `TestHandoff_*` pass. |
| `TESTING.md` | Phase 15 rows and Category BF added | No removed lines. Category BD (M-137..M-144) and the Phase 13 Section 2/4 rows are intact. `check-traceability-paths.sh`: `OK: 228 traceability paths exist`. |

Every other file changed in the Phase 13 production path (`portforward*.go`, `portproxy.go`, `porttoken.go`, `portprinted.go`, `peer_connect.go`, `remote_ws_proxy.go`, `share.go`, `server.go`, `api.go`, `client.go`, `types.go`, `app.go`, `localhostUrl.ts`, `SessionEndedBanner.tsx`) has no diff since the previous verification, so every file:line anchor the earlier report gave still holds.

## Goal Achievement

### Roadmap Success Criteria

| # | Success criterion | Status | Evidence |
|---|-------------------|--------|----------|
| SC1 | A click on `http://localhost:8000/` or `http://127.0.0.1:8000/` in a home peer tab opens the office page in the home default browser through Portty's proxy over the real tailnet, with paths, query strings and assets; no in-app browser (URL-01) | VERIFIED | The TerminalPanel loopback branch routes a classified URL from a desktop peer tab to `OpenRemotePort`; `openLink(res.url)` only on `status === 'open'`. The viewing forward carries the target in `X-Portty-Target`; the host proxy rebuilds path and RawQuery byte-exact and dials only `::1` then `127.0.0.1`. Tests run at HEAD: `TestPortForwardE2E_HostDevServer`, `_WebSocketAndSSE`, `TestPortProxy_PathAndHeaders`, `TestOpenRemotePort`. VM: L-02 and L-05 (13-VM-CHECKS.md, carried forward). |
| SC2 | The first click asks for consent in a macOS-style sheet naming the remote Mac and port; declining opens nothing; approval covers only that Mac and port; the listener binds only to loopback (URL-02) | VERIFIED | Consent sheet and per-(host, port) approval code unchanged. `TestPortForward_ConsentFirst`, `TestPortOpen_*`, `_NoApprovalAfterPortInUse` and the vitest consent-sheet suite pass at HEAD. VM: L-03, L-07. |
| SC3 | The proxy refuses whatever wasn't asked for, before the remote Mac connects anywhere; loopback-only; no port scanning; stopping sharing stops the page; the security review records the threat model (URL-02) | VERIFIED | Host grant and proxy order unchanged (same-user, write capability, port token, deny list, printed check, then dial). The printed check (`Hub.PortPrinted`) still passes its full table after Phase 15's hub change. Webserver `Port`, `SameUser`, `Capability` tests pass (incl. `TestPortGrant_Refusals`, `TestPortProxy_Refusals`, `TestPortProxyRevoke_*`); daemon `TestPortForwardE2E_StopSharing`, `_ReshareWithoutStream`, `TestTakeover_FinalizeDeniesRelayPort` pass. 13-SECURITY.md: `threats_open: 0`. VM: L-04, L-06. |
| SC4 | Local URLs, and non-localhost URLs in remote tabs, work as today (confirm popover, then default browser); existing link tests still pass (URL-03) | VERIFIED | The non-peer and non-loopback path is unchanged. 12 vitest files / 270 tests pass at HEAD: `App.webLink`, `LinkConfirmPopover`, `SessionEndedBanner`, `TerminalPanel.remotePort`, `TerminalPanel.web-links`, `WebShareSessionView.peerLinks`, `localhostUrl`, `openLink`, `urlSafety`, plus the Phase 15-touched `App.open-remote`, `closeReasons`, `TerminalPanel.guestClose`. VM: L-08. |

**Score:** 4/4 success criteria verified (0 present-but-behavior-unverified).

### Plan must-have truths (merged)

Re-checked against HEAD by `verify.artifacts` and `verify.key-links` on all seven plans (25/25 artifacts and 25/25 key links verified; per plan: 4/4 and 3/3, 4/4 and 6/6, 3/3 and 4/4, 4/4 and 3/3, 4/4 and 3/3, 3/3 and 3/3, 3/3 and 3/3), and by the named tests below. Unchanged from the previous report:

| Plan | Truths | Status | Key evidence |
|------|--------|--------|--------------|
| 13-01 | 5 | VERIFIED | `internal/relay` `PortPrinted`/`HasURLPort`/`PathClick`/`Cwd_` tests pass; `internal/capability` `PortToken`/`VerifyPort`/`Capability` pass |
| 13-02 | 9 | VERIFIED | `TestPortForward_*`, `TestPortOpen_*`, `TestForwardHostAllowed`, `TestClassifyLoopbackURL_Fixture` (33 PASS, 0 FAIL in one daemon run) |
| 13-03 | 6 | VERIFIED | `TestPortGrant_*`, `TestPortProxy_*`, `TestPortForwardE2E_HostDevServer`; routes still sit behind `requireSameUser` only |
| 13-04 | 4 | VERIFIED | `TestOpenRemotePort*` (root package), `TestOpenRemotePortClient`, `TestHealthInfo`, `TestAPIHealth` |
| 13-05 | 5 | VERIFIED | `TerminalPanel.remotePort`, `localhostUrl`, `SessionEndedBanner` suites |
| 13-06 | 5 | VERIFIED | `TestPortForwardE2E_WebSocketAndSSE`, `_StopSharing`, `_ReshareWithoutStream`, `TestPortProxyRevoke_*` |
| 13-07 | 9 | VERIFIED | 13-VM-CHECKS.md L-00..L-08 all PASS; TESTING.md Sections 2, 4, 5 present (Category BD, M-137..M-144); traceability script OK |

Behavior-dependent truths (revocation, cancellation, cleanup) each have a passing behavioural test run in this session, so none is PRESENT_BEHAVIOR_UNVERIFIED.

### Required Artifacts

All 25 declared artifacts across 13-01..13-07 pass `verify.artifacts` at HEAD and remain wired (`portprinted.go` from `portproxy.go`, `porttoken.go` from `portproxy.go`, `portforward*.go` from `api.go`, `localhostUrl.ts` from `TerminalPanel.tsx:1028`, `OpenRemotePort` from `app.go` through `client.go` to `/peers/ports`).

### Key Link Verification

All 25 key links across the seven plans report `verified: true` from `verify.key-links` at HEAD. The Phase 15 hunks touch none of them.

### Data-Flow Trace (Level 4)

Unchanged and not touched by Phase 15: consent title data from the daemon's `RemotePortResult`, the opened URL from `openedURL(...)`, the forward's `X-Portty-*` headers per request from `remoteCaps.Get(sid)`, the host dial from the validated target. The only changed data (the hub's `History.pushed` counter) feeds row numbering, not the printed-URL text the check searches. Status: FLOWING.

### Behavioral Spot-Checks (HEAD, targeted, `-count=1`)

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Build, vet, format | `GOTOOLCHAIN=go1.26.11 go build ./...`; `go vet` on capability, relay, webserver, daemon, tailnet and root; `gofmt -l internal app.go` | clean | PASS |
| Port token and printed check | `go test -race -short -count=1 ./internal/capability -run 'PortToken\|VerifyPort\|Capability'`; `./internal/relay -run 'PortPrinted\|HasURLPort\|PathClick\|Cwd_'` | ok, ok | PASS |
| Host routes and revoke | `go test -race -short -count=1 ./internal/webserver -run 'Port\|SameUser\|Capability'` | ok | PASS |
| Viewing forward, e2e, takeover deny, API, handoff and peer connect | `go test -race -short -count=1 ./internal/daemon -run 'PortForward\|PortOpen\|ForwardHostAllowed\|ClassifyLoopback\|Takeover_FinalizeDeniesRelayPort\|HealthInfo\|APIHealth\|OpenRemotePortClient'` (-v: 33 PASS, 0 FAIL); wider run adding `Handoff_Refuses\|Handoff_Tightens\|PeerConnect` | ok | PASS |
| App binding | `go test -race -short -count=1 . -run OpenRemotePort` | ok | PASS |
| Tailnet probe (MetaRefused) | `go test -race -short -count=1 ./internal/tailnet` | ok | PASS |
| Link, consent and Phase 15-touched UI | `npx vitest run` on the 12 files listed under SC4 (node, `NO_COLOR=1`) | 12 files / 270 tests PASS | PASS |
| Path-link provider (Phase 15 mouseleave) | `npx vitest run src/lib/__tests__/pathLinks` | 2 files / 43 tests PASS | PASS |
| Types | `npx tsc --noEmit` | clean | PASS |
| Traceability | `bash tests/check-traceability-paths.sh` | `OK: 228 traceability paths exist` | PASS |

Not re-run (by instruction): the full Go and vitest suites. The orchestrator reports them green on HEAD (Go `-race` 14/14, vitest 122/122 files, 1826 tests).

### Probe Execution

Step 7c: SKIPPED. No phase-declared or conventional `scripts/*/tests/probe-*.sh` probes. The VM rows in 13-VM-CHECKS.md are the runtime evidence.

### Human verification history (carried forward as verified)

The recorded VM runs (13-VM-CHECKS.md L-00..L-08) and the user's decisions stand. The behaviours they checked (printed-URL click, consent, forward, refusals, revoke, local-link unchanged) run through code Phase 15 did not change. The one Phase 15 change on a path L-01 exercised, the handoff pid check, was itself exercised on real processes by Phase 15's M-155, which passed. No Phase 13 VM row needs re-running.

- **UI-1** (a different tailnet user is refused): "Accept automated-only", pinned by the "another user" rows in `TestPortGrant_Refusals` and `TestPortProxy_Refusals` (run and passing here); TESTING.md M-139.
- **UI-2** (a real Vite or webpack HMR project): "Check on my Macs", TESTING.md M-140; L-05 already shows a WebSocket-driven reload.
- **G-1** (`internal/pty` `TestKillClean_NoOrphans` flake): accepted as the known Phase 12 flake; the orchestrator's full `-race` run passed on HEAD.

### Requirements Coverage

| Requirement | Source plans | Description | Status | Evidence |
|-------------|--------------|-------------|--------|----------|
| URL-01 | 13-02, 13-03, 13-04, 13-05, 13-06, 13-07 | Clicking `http://localhost:PORT` / `127.0.0.1:PORT` in a remote session opens the remote page in the normal browser, proxied over the tailnet, WebSocket upgrades included | SATISFIED | SC1; L-02, L-05; e2e WebSocket/SSE tests pass at HEAD |
| URL-02 | 13-01..13-07 | Token-gated, tailnet-only, consent per port, loopback-only bind, no port scanning | SATISFIED | SC2, SC3; L-03, L-04, L-06, L-07; 13-SECURITY.md `threats_open: 0` |
| URL-03 | 13-05, 13-07 | Local URLs keep today's behaviour: confirm popover, then default browser | SATISFIED | SC4; L-08; the link suites pass at HEAD |

PLAN frontmatter declares URL-01, URL-02, URL-03 (and `[URL-02]` for 13-01); all three appear in REQUIREMENTS.md (lines 41-43, now checked `[x]`; traceability rows 90-92 mapped to Phase 13, status Complete). REQUIREMENTS.md maps no other ID to Phase 13, so nothing is orphaned.

### Anti-Patterns Found

No `TBD`, `FIXME`, `XXX`, `TODO`, `HACK` or `PLACEHOLDER` marker in any line Phase 15 added to the covered Phase 13 files (`hub.go`, `TerminalPanel.tsx`, `handoff.go` scanned). No stubs introduced.

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| internal/daemon/portforward.go | 414 | WR-02 refuses `Origin: null` and non-loopback Origins on non-navigation requests | Info | Unchanged; the secure choice, recorded in the earlier report. M-140 on the user's Macs would show it if it bites. |
| .planning/phases/13-remote-localhost-urls/13-SECURITY-REVIEW.md | 35, 75 | Narrative describes only the `Sec-Fetch-Site` check; the WR-02 Origin check is in 13-SECURITY.md (T-13-07) | Info | Documentation lag only. |
| internal/daemon/portforward.go | 305-320 | WR-03 race: a stop, re-share and peer reconnect while a re-grant is in flight | Info | Accepted residual in 13-SECURITY.md line 107. |

### Human Verification Required

None. Every success criterion has code evidence, passing named tests at HEAD and a PASS VM row, and Phase 15 changed nothing those VM rows checked.

### Gaps Summary

No gaps at HEAD. A printed loopback URL in a desktop peer tab still goes through the classifier, the viewing daemon's consent-gated loopback-only forward, and the host's same-user, write-capability, port-token-gated proxy to the host's own loopback. Every refusal is identical and decided before any dial, the printed-URL check still passes its table after Phase 15's hub row-numbering change, and forwards still end with the share. Phase 15's ended-tab wiring, update banner, path-link mouseleave, `MetaRefused` probe status, socket permissions and kernel-confirmed handoff pid did not alter any Phase 13 truth.

---

_Verified: 2026-11-23 (re-verification at c818e1e1, after Phase 15)_
_Verifier: Claude (gsd-verifier)_
