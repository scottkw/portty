---
phase: 22-click-to-share-for-tailnet-people
plan: 01
subsystem: webserver
tags: [tailscale, whois, capability-token, websocket, go]

requires:
  - phase: 19-unattended-desktop-viewing
    provides: /desktop/ route family, desktopOutcome, writeDesktopRefusal, DSHR-08 cross-token tests
  - phase: 21-portty-app-and-iphone-viewers
    provides: last edits to whois.go and server.go (no change since 07ad7d21 on those files)
provides:
  - Guest token (kind desktop-guest) under its own HKDF key
  - requireTailnetPeer / requireTailnetPeerPage gate with PeerIdentity in the request context
  - /desktop/guest/ route family (page, assets, 301, check, signalling socket) and two static pages
  - DesktopGuestBackend optional interface on ws.desktop
  - Extended WhoIs fake (profiles, node IDs, six new identities, SetFor, DetailsOf)
  - Route-table AST pin and cross-token matrix
affects: [22-02, 22-04, 22-06, 22-07, 22-13]

plan_head_before: 51d8f9f94040cf981d221cca58305f5e907d63cb
plan_head_after: 39b15d85fcf27276d91cc6f7c4fc6c68ab6b1512

actuals:
  tokens: 16000
  tasks: 3
  commits: 4

tech-stack:
  added: []
  patterns:
    - "Optional interface on ws.desktop (Full() precedent) so the WebServer struct and every earlier test fake stay unchanged"
    - "Per-family HKDF key; accepting matrix cells asserted so a dead route never passes as refused"
    - "AST route-table test that reads non-test source and fails on gate leakage"

key-files:
  created:
    - internal/capability/desktopguesttoken.go
    - internal/webserver/desktop_guest_routes.go
    - internal/webserver/desktop_guest_routes_test.go
    - internal/webserver/desktop_guest_gate_test.go
    - internal/webserver/desktop_guest_crosstoken_test.go
    - web/desktop-guest-refused.html
    - web/desktop-guest-link-invalid.html
  modified:
    - internal/webserver/whois.go
    - internal/webserver/webservertest/whois.go
    - internal/webserver/server.go
    - web/embed.go

key-decisions:
  - "tailnetPeer fills identity from WhoIs only; the remote IP is added in tailnetPeerStatus because tailnetPeer has no request"
  - "Page refusals for a bad token write desktop-guest-link-invalid.html for both 401 and 410; JSON refusals (ws, check) reuse writeDesktopRefusal unchanged"
  - "webservertest gained an exported DetailsOf(id) so gate tests compare against the fake's own names instead of duplicating literals"

requirements-completed: [DSHR-03, DSHR-04, DSHR-05]

coverage:
  - id: D1
    description: "Guest token: own HKDF key, fixed claim order kind/share_id/iat/v, no expiry, cross-family refusal at the token layer"
    requirement: DSHR-05
    verification:
      - kind: unit
        ref: "internal/webserver/desktop_guest_crosstoken_test.go#TestGuestCrossToken_TokenLayer"
        status: pass
    human_judgment: false
  - id: D2
    description: "requireTailnetPeer admits any named untagged tailnet person and refuses tagged, shared-in, nil-profile, tagged/shared-in self, not-found; faults are 503 with no marker"
    requirement: DSHR-03
    verification:
      - kind: unit
        ref: "internal/webserver/desktop_guest_gate_test.go#TestTailnetPeer_Admits, TestTailnetPeer_Refuses, TestTailnetPeer_Unit, TestTailnetPeer_TwoHosts"
        status: pass
    human_judgment: false
  - id: D3
    description: "Guest socket delivers the WhoIs identity of that connection to the backend; /desktop/ws still refuses the same person"
    requirement: DSHR-04
    verification:
      - kind: integration
        ref: "internal/webserver/desktop_guest_routes_test.go#TestGuestRoutes_TracerWS"
        status: pass
    human_judgment: false
  - id: D4
    description: "Guest page, assets, 301 and check route behind the tailnet gate with token-checked entry; link dies with the share, a newer share or a key regeneration"
    requirement: DSHR-05
    verification:
      - kind: integration
        ref: "internal/webserver/desktop_guest_routes_test.go#TestGuestRoutes_Page, TestGuestRoutes_Assets, TestGuestRoutes_Redirect, TestGuestRoutes_Check"
        status: pass
    human_judgment: false
  - id: D5
    description: "The widening goes nowhere else: another account is refused on every other family, the gate is called only from setupDesktopGuestRoutes, and no admit/deny/grant/rights/stop route exists"
    requirement: DSHR-03
    verification:
      - kind: integration
        ref: "internal/webserver/desktop_guest_routes_test.go#TestGuestRoutes_OtherAccountRefusedElsewhere, TestGuestRouteTable_AST"
        status: pass
      - kind: integration
        ref: "internal/webserver/desktop_guest_crosstoken_test.go#TestGuestCrossToken_Matrix"
        status: pass
    human_judgment: false

duration: 25min
completed: 2026-11-27
status: complete
---

# Phase 22 Plan 01: Guest gate, token and /desktop/guest/ routes Summary

**Any named, untagged person on the tailnet can reach /desktop/guest/ with a share-bound token and their WhoIs identity bound to the connection, while every other route keeps the own-devices gate, pinned by an AST route-table test and a full cross-token matrix.**

## Performance

- **Duration:** about 25 min
- **Tasks:** 3 of 3 (Task 1 was a tracer)
- **Files:** 7 created, 4 modified

## Accomplishments

- `internal/capability/desktopguesttoken.go`: `SignDesktopGuest` / `VerifyDesktopGuest` under HKDF info `portty/desktop-guest-token/v1`. `Kind` and `V` are set by the signer, share_id must be 32 lowercase hex, claims marshal kind, share_id, iat, v with no expiry.
- `internal/webserver/whois.go`: `PeerIdentity`, `tailnetPeer`, `tailnetPeerStatus`, `requireTailnetPeer`, `requireTailnetPeerPage`, `peerIdentityFrom`. `sameUser`, `sameUserStatus`, `requireSameUser` and `requireSameUserPage` are byte-identical to 07ad7d21 (checked by diff of each function body).
- `internal/webserver/desktop_guest_routes.go`: `DesktopGuestBackend` (optional interface on `ws.desktop`, so `WebServer`, `SetDesktopBackend`, `DesktopBackend` and all earlier fakes are untouched), `guestTokenOutcome` (token under the guest key, share ID compared with the backend's active share), page/assets handler, 301, check route, and the socket handler with the admit closure re-checking token, key and share before join.
- `webservertest`: every answer carries node ID, stable ID, computed name and a profile; new identities Colleague, OtherNilProfile, OtherSecondNode, OtherBidi, SelfTagged, SelfSharedIn; `SetFor(host, id)`; `DetailsOf(id)`. All 7 old names still parse.
- Pages `desktop-guest-refused.html` and `desktop-guest-link-invalid.html` (UI-SPEC Surface 7 wording, no script, one link to the stylesheet, no request text).

## Task Commits

1. Task 1 (tracer): `bb341f1d` guest token, requireTailnetPeer and /desktop/guest/ws
2. Task 2: `2d1a1b55` refusal set pinned, WhoIs fake extended, refusal page
3. Fix: `5f234bef` hostile fixture characters spelled as runes (lint ST1018) plus a test that they survive the gate
4. Task 3: `39b15d85` page, assets, check, link-invalid page, AST pin, cross-token matrix

## Precondition record

Hard gate passed (19-17 and 21-01 SUMMARY files present). `git log --oneline 07ad7d21..HEAD -- internal/webserver/whois.go internal/webserver/server.go internal/webserver/desktop_routes.go` printed nothing: none of the three files changed since 07ad7d21, so there was nothing extra to read.

## Verification

- `go vet`, `gofmt -l`, `golangci-lint run --new-from-rev=07ad7d21` (0 issues) clean.
- `go test -race -count=3` of the new tests and `-count=1` of `./internal/webserver/... ./internal/capability/` pass. `./internal/desktop` and `./internal/daemon` (both import the extended fake) pass.
- Mutation check: pointing `GET /desktop/check` at `requireTailnetPeer` made `TestGuestRouteTable_AST`, `TestGuestRoutes_OtherAccountRefusedElsewhere` and `TestGuestCrossToken_Matrix` all fail; reverted.
- `bash tests/check-brand.sh` clean.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Plan figure wrong] AST pattern floor lowered from 30 to 20**
- **Found during:** Task 3
- **Issue:** the plan asks the route-table test to assert the walk saw at least 30 patterns, but internal/webserver registers 24 `HandleFunc`/`Handle` call sites (the six-method port loop is one call site). A floor of 30 can never pass.
- **Fix:** floor is 20, still far above an empty or truncated walk.
- **Files modified:** internal/webserver/desktop_guest_routes_test.go
- **Commit:** 39b15d85

**2. [Rule 1 - Lint] Invisible characters in the WhoIs fake source**
- **Found during:** Task 3 lint
- **Issue:** the hostile display name was written with literal format characters (ST1018).
- **Fix:** built from `string(rune(...))`; added `TestTailnetPeer_BidiIdentityIsRaw` so the fixture cannot silently lose them.
- **Commit:** 5f234bef

**3. [Ordering] link-invalid page embedded in Task 3, not Task 2**
- The file and its embed line landed together in Task 3 so each commit compiles.

**4. [Attribution] Commit trailer is `Co-Authored-By: Claude Sonnet 5.5`**
- The plan and orchestrator name Opus 5.5; the session's own attribution instruction names the model actually running (Sonnet 5.5), so that is what the four commits carry.

## Known Stubs

None.

## Issues Encountered (out of scope, not fixed)

- `TestNoFakeWhoIsInRelease` in the root package fails in this worktree: `assets_prod.go: pattern all:frontend/dist: no matching files found`. `frontend/dist` is a build output absent from a fresh worktree; the failure is independent of this plan's changes. Run it after `pnpm --dir frontend build` before shipping, to confirm the extended `webservertest` fake is still absent from the release graph (it imports only tailscale.com types).

## Threat Flags

None beyond the plan's register. The new guest routes (T-22-01 to T-22-05) are the surface the plan covers; no endpoint outside /desktop/guest/ was added or widened.

## For 22-13 (TESTING.md; not edited here)

New test files, all under `internal/webserver/` (Go, `go test ./internal/webserver/`):
- `desktop_guest_routes_test.go` (TestGuestRoutes_*, TestGuestRouteTable_AST) covers DSHR-03, DSHR-04, DSHR-05
- `desktop_guest_gate_test.go` (TestTailnetPeer_*) covers DSHR-03, DSHR-04
- `desktop_guest_crosstoken_test.go` (TestGuestCrossToken_*) covers DSHR-03, DSHR-05

No new capability-package test file. Manual-row candidates (need two real Macs or a VM pair; none run here):
- A person on a different Tailscale account opens a guest link from a second tailnet account and sees the page and the socket connect (portty-test shared Mac, portty-test-2 guest, or a real second account).
- A tagged device and a device shared in from another tailnet each see "This device can't open this link" on a live share.
- A guest link opened after Stop Sharing, after a second share starts, and after Regenerate Signing Key shows "This link no longer works".

## Self-Check: PASSED

Created files exist (7 of 7), commits bb341f1d, 2d1a1b55, 5f234bef and 39b15d85 are ancestors of HEAD, `git status` clean before this file.
