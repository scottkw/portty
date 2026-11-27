---
phase: 21-portty-app-and-iphone-viewers
plan: 01
subsystem: webserver, tailnet, daemon
tags: [desktop, peer-discovery, same-user, capability, tailnet]
requires:
  - phase: 19-desktop-share-link
    provides: DesktopBackend, DesktopKey token separation, /desktop/check
provides:
  - X-Portty-Desktop: always discovery header on GET /api/sessions/meta
  - POST /api/peer/desktop/cap same-user route returning the standing desktop token
  - tailnet.PeerMeta, FetchPeerMeta, FetchPeerMetaWithClient, PeerSessionMetaGroup.Desktop
  - tailnet.FetchPeerDesktopCap and ErrPeerDesktopOff
affects: [21-home-side-peer-desktop-plans, 21-13]
tech-stack:
  added: []
  patterns:
    - optional interface on ws.desktop (DesktopToken) like the existing Full() check
    - struct-returning fetch variant with thin wrappers that keep old signatures
key-files:
  created:
    - internal/daemon/desktop_peer_token.go
    - internal/daemon/desktop_peer_token_test.go
    - internal/tailnet/desktop_meta_test.go
    - internal/webserver/desktop_peer_test.go
  modified:
    - internal/tailnet/sessions.go
    - internal/webserver/server.go
    - internal/webserver/whois.go
key-decisions:
  - "One header name serves both uses: X-Portty-Desktop is the /desktop/ refusal reason (token, off) and the meta discovery marker (always); readers branch on the exact value."
  - "DesktopToken takes the token out of desktopLinkLocked's link, so claims are signed in one place and desktop_api.go (owned by Phase 20) is untouched."
patterns-established:
  - "Peer cap routes: requireSameUser, JSON content type, 8 KiB body cap, no-store, nothing logged."
requirements-completed: [DVWR-01]
status: complete
actuals:
  tokens: 28000
  tasks: 3
  commits: 3
plan_head_before: 6662f19644b8914f539b0952f62f526d6d7e3711
plan_head_after: dae621e6c063e0f27eb251f714479c309bbe2fa3
duration: one session
completed: 2026-11-26
---

# Phase 21 Plan 01: Peer desktop discovery and cap route Summary

The office Mac now announces an always-available desktop on the existing meta poll, hands the standing token to a same-account peer daemon with one POST, and the tailnet package reads both.

## What was built

- **Discovery header** (`internal/webserver/server.go`): `handleSessionsMeta` sets `X-Portty-Desktop: always` only while `ws.desktop` exists and `DesktopGrant` reports on. The body is still the bare array. The header is written after `requireSameUser`, so another account never sees it.
- **Cap route** (`internal/webserver/whois.go`, `server.go`): `POST /api/peer/desktop/cap` behind `requireSameUser`. Same checks as `handlePeerCap` (JSON media type, 8 KiB body cap), then an optional `DesktopToken() (string, bool)` on the backend; none, nil or off is 404. Response is `{"cap": token}` with `application/json` and `Cache-Control: no-store`. Nothing is logged.
- **Token provider** (`internal/daemon/desktop_peer_token.go`): `desktopAdapter.DesktopToken` reads `desktopLinkLocked` under the API read lock and returns the link's `cap` query value, so it is byte-identical to Copy Link and dies with always-available off or a key regeneration.
- **Tailnet** (`internal/tailnet/sessions.go`): `DesktopHeader`/`DesktopAlways`, `PeerMeta`, `FetchPeerMeta`, `FetchPeerMetaWithClient`, `PeerSessionMetaGroup.Desktop` (json `desktop`), `ErrPeerDesktopOff`, `FetchPeerDesktopCap`. `doFetchSessionsMeta` now returns `(PeerMeta, error)` with the forbidden branch still first. `FetchPeerSessionsMeta`, `FetchPeerSessionsMetaWithClient`, `FetchAllPeerSessionsMeta` and the two-value `fetchAllPeerSessionsMeta` seam keep their signatures; `tailnet_test.go` is untouched.

## Tasks and commits

| Task | Name | Commit |
|------|------|--------|
| 1 | Tracer: meta header and tailnet reader | 29b9632b |
| 2 | POST /api/peer/desktop/cap and DesktopToken (RED seen: 405s and a compile error, then GREEN) | 118561e9 |
| 3 | tailnet.FetchPeerDesktopCap (RED seen: compile error, then GREEN) | dae621e6 |

## Verification

- `go test -race -count=1 ./internal/tailnet/ ./internal/webserver/ ./internal/daemon/` passes; `-count=3` on the new `TestDesktopMeta_`, `TestPeerDesktopCap_`, `TestDesktopPeerToken_` tests passes.
- `go vet` and `gofmt -l` clean; `bash tests/check-brand.sh` clean.
- golangci-lint with `--new-from-rev=060281bd` reports 6 errcheck findings, all in files this plan did not touch (`internal/daemon/client.go`, `process.go`, `restart_pin_test.go`); none in this plan's files. Left alone as out of scope.
- No test contacts a live tailnet: WhoIs is the in-package fake, servers are httptest or the self-signed test server.

## New test files (for 21-13's TESTING.md rows)

Section 2 (suite manifest) and Section 4 (traceability, DVWR-01), repo-relative paths:

- `internal/webserver/desktop_peer_test.go` (TestDesktopMeta_*, TestPeerDesktopCap_*)
- `internal/daemon/desktop_peer_token_test.go` (TestDesktopPeerToken_*)
- `internal/tailnet/desktop_meta_test.go` (TestDesktopMeta_*, TestFetchPeerDesktopCap_*)

No manual M-NN item is needed for this plan. TESTING.md was not edited, per the dispatch instructions.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Plan claim wrong] GET on the cap route is 404, not 405**
- **Found during:** Task 2 (RED run)
- **Issue:** The plan says a GET "answers 405 from the mux pattern". The catch-all `GET /` handler matches first, so a GET to the route is 404 (the existing `POST /api/peer/sessions/{id}/cap` behaves the same).
- **Fix:** The test asserts the GET is refused with 404 or 405 and never reaches the handler. No production change.
- **Files modified:** internal/webserver/desktop_peer_test.go
- **Commit:** 118561e9

**2. [Minor] Empty-cap error text** is "peer desktop cap: empty capability" (the plan did not fix the text; it follows the "peer desktop cap:" prefix of the other errors).

Otherwise the plan was executed as written. `internal/daemon/desktop_api.go` and `internal/tailnet/tailnet_test.go` are not touched.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register: T-21-01 (same-user, POST plus JSON, no-store, not logged), T-21-02 (the returned token is refused on /api/sessions and /sessions/{id}/ws, tested), T-21-04 (header written only after requireSameUser; forbidden branch read first on the client, tested) are all covered by tests.

## Notes for the orchestrator

- `commits: 3` counts the task commits; the SUMMARY commit follows `plan_head_after`.
- STATE.md and ROADMAP.md were not modified.

## Self-Check: PASSED

Created files exist (desktop_peer_token.go, desktop_peer_token_test.go, desktop_meta_test.go, desktop_peer_test.go); commits 29b9632b, 118561e9, dae621e6 are ancestors of HEAD.
