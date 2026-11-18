---
phase: 04-one-click-share
plan: 03
subsystem: sharing
tags: [go, webserver, relay, daemon, tailscale, whois, capability, websocket]
status: complete

requires:
  - phase: 04-one-click-share
    provides: "04-02: daemon share registry, share server runs only while something is shared, one read,write token per share"
provides:
  - "requireSameUser WhoIs middleware (fail closed) on GET /api/sessions/meta and POST /api/peer/sessions/{id}/cap"
  - "Peer capability route returning the share link's own token to same-account peers"
  - "relay.CloseReasonShareStopped / relay.CloseReasonSessionEnded wire constants; Hub.RevokeWebViewers(reason); Subscriber.Kick"
  - "Post-subscribe grant re-check in handleWSSRelay; per-session revoke signal that ends the plugin-config stream"
  - "Static web/link-invalid.html served by requireCapabilityPage on GET /sessions/{id}; Referrer-Policy: no-referrer on cspHeaders"
affects: [04-04 viewing-side peer cap fetch and close-reason forwarding, 04-05 GUI share popover, 04-06 removal gate and frontend close-reason constants]

actuals:
  tokens: 19593
  tasks: 2
  commits: 2
plan_head_before: 6fdf2f9c520143b63bc63ab6fc469cdae144a761
plan_head_after: 7ef6a325c258265afacb3eb100f3bc084b91eaa3

tech-stack:
  added: []
  patterns:
    - "whoIsClient seam: ws.whois = &ws.lc in production; tests call SetWhoIsClientForTest with a fake (webserver: fakeWhoIs / SameUserWhoIsForTest; daemon: sameUserWhoIs). No test reaches tailscaled; the fuzz server uses a failing fake"
    - "verifyCapability returns (claims, status, plain-text msg); requireCapability writes plain text, requireCapabilityPage writes the embedded HTML page with the same status"
    - "Long-lived streams take ws.revokeSignal(sid) and then re-check isGrantActive; ClearGrants closes and deletes the channel under ws.mu"

key-files:
  created:
    - internal/webserver/whois.go
    - internal/webserver/whois_test.go
    - internal/webserver/revoke_test.go
    - internal/webserver/link_page_test.go
    - internal/relay/close_reasons.go
    - internal/relay/close_reasons_test.go
    - web/link-invalid.html
  modified:
    - internal/webserver/server.go
    - internal/webserver/capability_mw.go
    - internal/webserver/csp_mw.go
    - internal/webserver/plugin_config_stream.go
    - internal/relay/hub.go
    - internal/relay/server.go
    - internal/daemon/share.go
    - web/embed.go
    - TESTING.md

key-decisions:
  - "The peer capability route returns the share record's existing token (one token per share), not a per-peer grant"
  - "GET on the peer capability route falls through to the GET / catch-all and answers 404 (not 405); the contract is only that GET never yields the token"
  - "A same-account meta 200 carries no X-Portty marker; only the 403 refusal does (TestProbeMarker_OtherRoutesHaveNoMarker kept)"
  - "Key regeneration now clears the grant before kicking (was kick-then-clear), matching share-off's order"
  - "No h2.error CSS rule was added; the dead-link page renders correctly with the existing dashboard.css"

patterns-established:
  - "Revoke order: ClearGrants, DisableSession, RevokeWebViewers(relay.CloseReasonShareStopped), then Stop when no share is left"

requirements-completed: []

coverage:
  - id: D1
    description: "Same-account peers get GET /api/sessions/meta and the share's capability from POST /api/peer/sessions/{id}/cap with no code; the cap equals the link token"
    requirement: SHAR-01
    verification:
      - kind: integration
        ref: "internal/webserver/whois_test.go#TestWhoIs_MetaSameUser"
        status: pass
      - kind: integration
        ref: "internal/webserver/whois_test.go#TestPeerCap_Routes"
        status: pass
      - kind: integration
        ref: "internal/daemon/share_test.go#TestShare_PeerCapReturnsLinkToken"
        status: pass
    human_judgment: false
  - id: D2
    description: "Other user, tagged node, shared-in node, unknown address, WhoIs or LocalAPI error all get 403 with X-Portty: 1 and a generic body; no Access-Control-* headers"
    requirement: SHAR-01
    verification:
      - kind: unit
        ref: "internal/webserver/whois_test.go#TestSameUser"
        status: pass
      - kind: integration
        ref: "internal/webserver/whois_test.go#TestWhoIs_MetaRefused"
        status: pass
    human_judgment: false
  - id: D3
    description: "Share-off closes every web viewer with 1000 'share stopped'; subscribe race closed; input after revoke never reaches the PTY; plugin stream ends on revoke without a broadcast"
    requirement: SHAR-04
    verification:
      - kind: integration
        ref: "internal/webserver/revoke_test.go#TestRevoke_ClosesWithShareStopped"
        status: pass
      - kind: integration
        ref: "internal/webserver/revoke_test.go#TestRevoke_SubscribeRaceClosed"
        status: pass
      - kind: integration
        ref: "internal/webserver/revoke_test.go#TestRevoke_PluginStreamEndsWithoutBroadcast"
        status: pass
      - kind: integration
        ref: "internal/daemon/share_test.go#TestShare_OffClosesViewerWithReason"
        status: pass
      - kind: unit
        ref: "internal/relay/hub_test.go#TestRevokeWebViewers"
        status: pass
    human_judgment: false
  - id: D4
    description: "Failed link opens show the static Link No Longer Works page with the same status codes, CSP and Referrer-Policy, and no session identity"
    requirement: SHAR-03
    verification:
      - kind: integration
        ref: "internal/webserver/link_page_test.go#TestLinkPage_FailedOpensShowPage"
        status: pass
      - kind: unit
        ref: "internal/webserver/csp_mw_test.go#TestCSPHeaders_ReferrerPolicyNoReferrer"
        status: pass
    human_judgment: false
  - id: D5
    description: "Same-account rule holds between the user's two real Macs (RESEARCH A2: Node.User equals Self.UserID)"
    requirement: SHAR-01
    verification:
      - kind: manual
        ref: "deferred two-Mac UAT (M-80, added in 04-06)"
        status: pending
    human_judgment: true

duration: ~35min
completed: 2026-11-16
---

# Phase 4 Plan 03: Same-account peer access and reason-carrying revoke Summary

**Peers signed in to the same Tailscale account now get the host's shared sessions and the share's own capability through a WhoIs check that fails closed. Turning Share off disconnects every viewer with close code 1000 and the fixed reason `share stopped`. A dead link now opens a static "Link No Longer Works" page.**

## Peer route contract (host side)

| Route | Gate | Responses |
|-------|------|-----------|
| `GET /api/sessions/meta` | `requireSameUser` | 200 JSON `[{id,name,status,url}]` (no marker); 403 `forbidden` + `X-Portty: 1` for any refusal |
| `POST /api/peer/sessions/{id}/cap` | `requireSameUser` | 200 `{"cap": "<link token>"}` + `Cache-Control: no-store`; 415 when the media type isn't `application/json`; 404 when the session isn't shared or no provider is set; 403 + `X-Portty: 1` on refusal; GET is 404 via the catch-all. Body capped at 8 KiB and ignored |
| `/sessions/{id}`, `/sessions/{id}/ws` | capability only (unchanged) | link works from any tailnet device |

No WhoIs-gated route sends `Access-Control-*` headers. A gate checks this: `git grep 'Access-Control-Allow'` over non-test webserver code is empty.

## sameUser rules as implemented

The check refuses when any of these is true: the response is nil, `who.Node` is nil, or self is nil; `who.Node.IsTagged()`; `who.Node.Sharer != 0`; self is tagged (`Tags` non-nil with `Len() > 0`); `self.AltSharerUserID != 0`; `self.UserID == 0`. Otherwise it allows only when `who.Node.User == self.UserID` (numeric IDs, never login names). The middleware allows 3 s for both LocalAPI calls. A `StatusWithoutPeers` error, a nil `Self`, or any `WhoIs` error (including `ErrPeerNotFound`) also gives 403. Each refusal writes a debug log with only `peer_user` and `self_user`.

## Revoke order

`ClearGrants` → `DisableSession` → `hub.RevokeWebViewers(relay.CloseReasonShareStopped)` → `Stop` when no share is left. Share-off, session exit and kill all use this order through `unshareLocked`. Key regeneration (`remintSharesLocked`) runs `ClearGrants` and then `RevokeWebViewers` before it mints the new grant. Stop comes last because it closes only the listener, and hijacked WebSockets outlive it.

Revocation reaches three places:
- **Viewers subscribed before the revoke:** they are kicked through `Subscriber.Kick` with 1000 and the reason.
- **A viewer that passed the cap check just before the revoke:** it fails the grant/enabled re-check right after `hub.Subscribe` and is closed with `share stopped`.
- **The plugin-config SSE stream:** it ends as soon as `ClearGrants` closes the session's revoke signal. It also re-checks the grant before every send.

## Task Commits

1. **Task 1: same-account peers via WhoIs, peer cap route** — `9ef6e440` (feat)
2. **Task 2: revoke with reason, subscribe race, plugin stream, dead-link page, Referrer-Policy** — `7ef6a325` (feat)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Tests that hit the meta route needed a WhoIs fake**
- **Found during:** Task 1
- **Issue:** Five test files besides the meta tests now reach a WhoIs-gated route: probe_marker_test.go, server_test.go `TestSessionInfo_UnknownID404KnownID200`, the fuzz route corpus, and the two meta tests. Without a fake they would return 403 or call the real tailscaled.
- **Fix:** probe_marker_test.go and server_test.go install `SameUserWhoIsForTest()`. The fuzz server installs a failing fake, so the fuzz corpus never talks to tailscaled.
- **Files modified:** internal/webserver/probe_marker_test.go, internal/webserver/server_test.go, internal/webserver/fuzz_test.go
- **Commit:** 9ef6e440

**2. [Rule 1 - Bug] Key regeneration kicked viewers before clearing the grant**
- **Found during:** Task 2
- **Issue:** `remintSharesLocked` disconnected viewers first and cleared the grant after. In that gap a viewer could reconnect on the old grant.
- **Fix:** It now clears the grant first, then revokes viewers.
- **Files modified:** internal/daemon/share.go
- **Commit:** 7ef6a325

Other notes:
- `requireCapabilityPage` also sets `X-Content-Type-Options: nosniff`. This keeps the header that `http.Error` already sent on the old plain-text path.
- `TestRevoke_PluginStreamDropsFrameAfterRevoke` deletes the grant directly instead of calling `ClearGrants`. That way only the per-send re-check is tested and the revoke signal does not mask it.
- Comments describing the meta route and `assertTailnetBindIP` as "open, network-layer trust" were updated, because they no longer hold.

## Mutation check

With the post-subscribe re-check disabled and `ClearGrants` no longer closing the revoke signal, `TestRevoke_SubscribeRaceClosed` and `TestRevoke_PluginStreamEndsWithoutBroadcast` both fail. The code was restored afterwards.

## Verification

- `go build ./... && go vet ./... && go test -race -short ./...`: green
- `go vet -tags playwrightfixture ./cmd/playwright-fixture/ && go vet -tags vmpreview ./...`: green
- `pnpm -C frontend exec tsc --noEmit && pnpm -C frontend exec vitest run`: 1204 passed
- `bash tests/check-traceability-paths.sh` (112 paths) and `bash tests/check-brand.sh`: OK
- Both fuzz seed corpora pass. `DisconnectWebViewers` is gone from Go. Non-test Go has exactly one `"share stopped"` literal and one `"session ended"` literal, both in close_reasons.go. `"too slow"` is unchanged (2).
- `pgrep -f '\.test daemon'` prints nothing.

## Known Stubs

None.

## Threat Flags

None. The new surface, the peer capability route, is the one the plan's threat model covers (T-04-10, T-04-12).

## Notes for later plans

- The vmpreview share server keeps the real `local.Client` as its WhoIs client. In the Tart VM (no Tailscale), meta and the peer route therefore return 403. 04-04/04-05 may need a vmpreview fake if VM peer discovery matters.
- TESTING.md Section 5 still has an M item with steps for the removed share-dialog viewer kick (from 04-02). Only its source line was updated here; 04-06 owns the wording sweep.

## Self-Check: PASSED
