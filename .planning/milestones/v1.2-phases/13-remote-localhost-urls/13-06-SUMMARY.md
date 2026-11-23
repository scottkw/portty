---
phase: 13-remote-localhost-urls
plan: 06
subsystem: daemon-and-webserver-revocation
tags: [go, websocket, sse, revoke, reverse-proxy, security-review, threat-model]
requires:
  - phase: 13-02
    provides: the viewing forward, RevokeSession and the session stream's terminal-close hook
  - phase: 13-03
    provides: the host grant and proxy routes, the revoke-bound request context, stopCh
  - phase: 13-04
    provides: App.OpenRemotePort and the version gate
  - phase: 13-05
    provides: the viewer click branch, consent sheet and notices
provides:
  - two-hop WebSocket and SSE tests, stop-sharing, session-kill and re-share tests across two daemons
  - host revoke tests (ClearGrants, Stop, key regeneration, client gone, goroutine baseline)
  - 13-SECURITY-REVIEW.md parts 1 (contract, check order, threat register T-13-01..T-13-35)
affects: [13-07 VM checks, TESTING.md and the review's VM evidence and verdict]
status: complete
actuals:
  tokens: 14800
  tasks: 3
  commits: 3
plan_head_before: 6e5f9942f240dc70d9907ec57bd297b323272344
plan_head_after: 39402801a16b0c29a17ed3ebaaade8b3be5402dc
tech-stack:
  added: []
  patterns:
    - "one rig (newE2E) builds the host daemon, a dev server with a WebSocket echo and an SSE stream, and a viewing daemon holding the share's capability; an optional second shared session keeps the share server running so only the grant revoke can end the streams"
    - "stream-end timing measured from the trigger, with no request sent to the forward meanwhile"
    - "goroutine baseline taken after a warm-up request with idle connections closed, then compared exactly"
key-files:
  created:
    - internal/webserver/portproxy_revoke_test.go
    - .planning/phases/13-remote-localhost-urls/13-SECURITY-REVIEW.md
  modified:
    - internal/daemon/portforward_e2e_test.go
key-decisions:
  - "The re-share step uses a real POST /peers/connect against the host's real share server (through the peer seams), not remoteCaps.Put, so the connect-time revoke is exercised"
  - "NoConnectStoppedPage and one StopSharing variant share a second session so the share server survives the share-off; the old capability then reaches a live server and is refused (`refused`), instead of the old base URL being dead (`unreachable`)"
requirements-completed: []  # the plan forbids requirements.mark-complete; 13-07 owns the end-to-end verification
---

# Phase 13 Plan 06: Stop-sharing revocation proofs and the security review Summary

Stopping the share, killing the session, regenerating the key or stopping the server now provably ends every proxied WebSocket and SSE stream on both Macs within milliseconds, closes the viewing forward with no request sent to it, and makes a re-share ask consent again; the phase's threat model is written down in 13-SECURITY-REVIEW.md.

## Accomplishments

- **Two-hop streams:** a coder/websocket echo (default same-origin check) and an SSE stream work through viewing forward, host share server and dev server. The client dials with Origin `http://127.0.0.1:<N>`, which the host rewrites to match Host `localhost:<N>`, and the dev server accepts.
- **Stop sharing across two daemons:** share-off (last share), share-off with another session still shared, and a killed session each end the WebSocket, the SSE body and the peer tab's session stream (1000 `share stopped`, `session ended` for the kill), and the forward stops listening, all with no request sent to it. The viewing daemon logs `outcome=revoked`.
- **Re-share asks again:** after share-off and a real `POST /peers/connect` with the new capability, a click with no consent answers `consent`.
- **No signal window:** with no session stream open, a connect carrying the new capability closes the forward at once; with no connect, the click is refused (`refused`) and opens nothing, the first request through the forward gets 502 and the stopped page, the dev server receives nothing, and the forward then closes.
- **Host revoke:** ClearGrants, Stop and a key regeneration (SetSigningKey, ClearGrants, a new grant) each end both stream kinds within 2 s; an old port token gets the identical 403 with `X-Portty` next to a capability that is good under the new key; a client that cancels a long poll cancels the dev server's request.
- **13-SECURITY-REVIEW.md:** every section of the layout, the four CONTEXT refinements, the contract table, the check order on both Macs as implemented, 15 numbered reasons it holds, honest scope, ten accepted risks each with its source, the unchanged list, version behaviour, viewer and consent behaviour, T-13-01..T-13-35 with pinned-by filled (T-13-SC noted once), and the test index.

## Tests added

- `internal/daemon/portforward_e2e_test.go`: `TestPortForwardE2E_WebSocketAndSSE`; `TestPortForwardE2E_StopSharing` (subtests `ShareOff`, `ShareOffKeepShareServer`, `SessionKilled`); `TestPortForwardE2E_ReshareWithoutStream` (subtests `PeerConnectAsksConsent`, `NoConnectStoppedPage`).
- `internal/webserver/portproxy_revoke_test.go` (new): `TestPortProxyRevoke_ClearGrants` (with goroutine baseline), `_Stop`, `_KeyRegeneration`, `_ClientGone`.

## Measured end times (from the trigger)

From `go test -v` of `TestPortForwardE2E_StopSharing`, one run under `-race`:

| Trigger | Session stream | WebSocket | SSE | Forward closed |
|---------|----------------|-----------|-----|----------------|
| share-off, last share | 3.4 ms | 3.0 ms | 1.5 ms | 4.6 ms |
| share-off, another session still shared | 2.2 ms | 2.8 ms | 2.2 ms | 4.0 ms |
| session killed | 2.1 ms | 1.6 ms | 2.2 ms | 3.6 ms |

Host-only (`TestPortProxyRevoke_*`): WebSocket and SSE ended within about 0.4 to 1.5 ms of ClearGrants, Stop and the key regeneration. The bound in the tests is 2 s. The goroutine count returned to its pre-stream baseline (6 in this package) within 5 s; the test compares exactly, not with a tolerance.

## Fixes the tests forced

None. No file other than the two test files and the review changed; `internal/webserver/portproxy.go`, `server.go` and `internal/daemon/portforward.go` are unmodified. The revoke-bound context and the forward's close-on-revoke from 13-02 and 13-03 behaved as designed.

## Verification

- Log directories (private `mktemp -d`): `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.0P20SWKuJe` (Task 1 development and mutations), `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.gKFueMOOkG` (Task 1 verify block), `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.My33Qj7MUc` (Task 3, full suite).
- Tracer gate (Task 1): the verify block was re-run after the first green and passed before Task 2 started. It also failed once for a missing `/api/relay/remote/` literal in the file (the dial helper lives in portforward_test.go); a comment naming the route was added and the block passed.
- `GOTOOLCHAIN=go1.26.11 go build ./... && go vet ./...` clean; `gofmt -l` clean.
- `go test -race -short -count=1 ./...`: rc 0, zero `--- FAIL` or `FAIL` lines, every package ok (root 13.9s, daemon 85.9s, relay 123.2s, webserver 16.5s, pty 8.1s).
- E2E `-count=5` and `PortProxyRevoke` `-count=10`: ok, no flake.
- Review gate: every heading, T-13-01 to T-13-35 and T-13-SC, the six keyword strings, the six named tests, and zero matches for the link, token and tailnet URL patterns; every `Test...` name the review cites exists as a function (checked by grep). `git diff --quiet 9574fd49 -- internal/webserver/csp_mw.go frontend/index.html` clean; capability_test.go, whois_test.go, pathclick_test.go and web_links_test.go also identical to 9574fd49.
- Mutation checks (each reverted, `git diff` on portproxy.go and remote_ws_proxy.go empty afterwards):
  - no Origin rewrite: `TestPortForwardE2E_WebSocketAndSSE` fails with a 403 on the upgrade.
  - no `RevokeSession` on the terminal close: all three `StopSharing` subtests fail (forward still listens after 2 s). The `ReshareWithoutStream` subtests rightly still pass, since no stream is involved.
  - dropping the `revokeSignal` case from the proxied-request context: `ClearGrants` and `KeyRegeneration` fail (WebSocket still open after 2 s).
  - dropping the `stopCh` case: `Stop` fails.
  - verifying the port token under a fixed old key: `KeyRegeneration` fails on the refusal row.
  - using `context.Background()` for the proxied request: `ClientGone` fails.
  - one leaked goroutine per request: `ClearGrants` fails on the baseline (8 against 6).

## Commits

- fd477f92 test(13-06): stopping the share ends proxied WebSockets and streams and closes the forward with no request
- 4be0313f test(13-06): every host revoke path ends proxied streams and leaves no goroutine behind
- 39402801 docs(13-06): security review parts 1

## Deviations from Plan

### Auto-fixed Issues

None for Rules 1-3.

### Process deviations

**1. TDD order (Task 2).** The behaviours already held (13-03 wrote the revoke-bound context), so the new tests passed on first run: there was no observed RED step. The mutation checks above stand in as evidence that each test can fail, and each one named a different test.

**2. Two extra subtests beyond the plan's list.** `ShareOffKeepShareServer` was added to `StopSharing` so the streams end through the grant revoke alone, with the share server still running (without it, `Stop` also closes non-hijacked connections and masks a missing revoke). `PeerConnectAsksConsent` uses a keeper session for the same reason. The plan's `remoteCaps.Put` fallback was not needed: `POST /peers/connect` through the peer seams works against the real loopback share server.

**3. Session kill reason.** The plan expects `session ended` for the kill; that is what the session stream carried. For share-off it is `share stopped`. Each subtest asserts its own reason exactly.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register (T-13-32 to T-13-35): no new endpoint, auth path or file access; the plan adds tests and a document.

## Not verified

- No real tailnet, certificate or tailscaled WhoIs: the share server is a loopback server with a self-signed certificate and a same-user fake, as in 13-03.
- Both Macs share one machine's loopback in the E2E tests, so the host's dial goes through `SetPortDialerForTest`; the production `[::1]` then `127.0.0.1` dialer is covered by the webserver tests (`TestPortProxy_DialOrder`) and `TestPortProxyRevoke_*`, which use it against real loopback listeners.
- No browser, GUI or VM checks (13-07). The review has no VM evidence or verdict yet; that section is 13-07's. The review's claims about `lsof`, a real Vite or `python3 -m http.server`, and L-06 are forward references to 13-07.
- A real browser's behaviour for `localhost` resolving to `[::1]` first is cited from RESEARCH, not re-tested here.
- `TESTING.md` is not updated (the plan's file list excludes it; 13-07 owns it, including `tests/check-traceability-paths.sh`). The new test file `portproxy_revoke_test.go` and the extended `portforward_e2e_test.go` need their Section 2 and Section 4 rows there.
- The review's accepted risk on per-request WhoIs (a 503 on asset-heavy pages) and "no in-flight cap" are recorded from RESEARCH and not measured.
- `golangci-lint` was not run.

## Self-Check: PASSED

Files present: internal/daemon/portforward_e2e_test.go, internal/webserver/portproxy_revoke_test.go, .planning/phases/13-remote-localhost-urls/13-SECURITY-REVIEW.md; commits fd477f92, 4be0313f, 39402801 are ancestors of HEAD.
