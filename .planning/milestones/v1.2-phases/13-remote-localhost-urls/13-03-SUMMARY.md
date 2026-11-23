---
phase: 13-remote-localhost-urls
plan: 03
subsystem: webserver
tags: [go, reverse-proxy, loopback, capability, port-token, anti-scan, tailnet]
requires:
  - phase: 13-01
    provides: Hub.PortPrinted, capability.SignPort/VerifyPort
  - phase: 13-02
    provides: the viewing daemon's forward (POST /peers/ports) that calls these routes
provides:
  - POST /api/peer/sessions/{id}/ports (grant) and per-method /ports/{port} (proxy) on the share server
  - verifyCapabilityToken, SetDeniedPorts, SetPortDialerForTest, stopCh on WebServer
  - relay port denied via wireShareServer
affects: [13-04 API 5 and binding, 13-06 revoke/streams and security review, 13-07 TESTING.md and VM checks]
status: complete
actuals:
  tokens: 12000
  tasks: 3
  commits: 3
plan_head_before: cad1a085486ca16f383c4756cd67d28a221dd886
plan_head_after: 247f6abded92cf65f9af823541ee7f36d13ede14
tech-stack:
  added: []
  patterns:
    - "every credential checked and every refusal identical (refusePort = refuseOtherUser) before the first dial; tripwire listeners prove it"
    - "one shared HTTP/1.1 transport whose dialer ignores the URL host, takes only a validated port, and dials [::1] then 127.0.0.1"
    - "request context cancelled by revokeSignal or the server's stopCh so upgraded tunnels end with the share"
key-files:
  created:
    - internal/webserver/portproxy.go
    - internal/webserver/portproxy_test.go
    - internal/daemon/portforward_e2e_test.go
  modified:
    - internal/webserver/server.go
    - internal/webserver/capability_mw.go
    - internal/daemon/share.go
key-decisions:
  - "One shared transport (created in NewWebServer, idle conns closed in Stop) instead of one per request, so keep-alive and HMR asset bursts do not leak idle sockets; its dialer takes only the port from the address, re-checks the deny list and ignores the host"
  - "Origin is rewritten only from exactly http://127.0.0.1:<P> (plan's refinement), any other Origin reaches the dev server unchanged"
  - "Grant body is read with ReadAll over a 1 KiB MaxBytesReader then json.Unmarshal, so an oversized or trailing-garbage body is refused and allocation is bounded"
requirements-completed: []  # URL-01 and URL-02 need 13-04 (API 5, binding) and 13-05 (click) to be delivered end to end; the plan forbids requirements.mark-complete
---

# Phase 13 Plan 03: Host grant and proxy routes Summary

The host Mac's half of a remote localhost URL: a grant route that mints a port token only for a port the session printed (same-user, write-capability caller), and a proxy route that checks every credential before it dials and then reverse-proxies, WebSocket upgrades included, to the host's own loopback.

## Accomplishments

- `portproxy.go`: `refusePort`, `portDenied` (0, the share server's own port, 7444, `SetDeniedPorts`), `portCapability` (header capability via the new `verifyCapabilityToken` plus a `write` permission), `handlePortGrant`, `handlePortProxy`, `newPortReverseProxy`, `newPortTransport`, `dialLoopback`.
- Check order as in the plan. Grant: same user (existing wrapper), JSON content type, body at most 1 KiB with an integer port in 1..65535, write capability, deny list, `PortPrinted`, mint. Proxy: same user, `{port}` decimal 1..65535 without leading zero, capability, `VerifyPort` with SID, port and grant id all matching, deny list, target (non-empty, at most 8192 bytes, starts with `/`, parses), revoke-bound context, proxy.
- Proxy mechanics: `Rewrite` copies Path, RawPath and RawQuery from the parsed target (byte-exact RequestURI), sets Host `localhost:<P>`, deletes every `X-Portty*`, `X-Forwarded*` and `Forwarded` header, rewrites Origin only from `http://127.0.0.1:<P>`; `ModifyResponse` deletes `X-Portty*` from the dev server; the error handler gives 502 with the marker and `nothing is answering`, logging no address.
- `server.go`: routes registered next to the peer cap route behind `requireSameUser`, per method, outside the CSP and marker wrappers; `deniedPorts`, `stopCh` (closed once by `Stop`), the port transport, `SetDeniedPorts`, `SetPortDialerForTest`.
- `capability_mw.go`: `verifyCapability` now calls `verifyCapabilityToken(token, pathID)`; sequence, statuses and bodies unchanged.
- `share.go`: `wireShareServer` passes `a.relayPort` to `SetDeniedPorts` (the relay accepts Origins naming its own port).

## Tests added

- `internal/daemon/portforward_e2e_test.go`: TestPortForwardE2E_HostDevServer (two daemons: forward, then the real share server, then a loopback dev server; Host `localhost:<N>`, RequestURI `/hello?q=1`, no `X-Portty*`/`X-Forwarded*`; a never-printed port answers 403 refused and no forward is bound).
- `internal/webserver/portproxy_test.go`: TestPortGrant_Granted, _Refusals (18 rows), _IdenticalOpenOrClosed, _LocalFault, _LogsNoSecrets; TestPortProxy_Refusals (18 rows), _PathAndHeaders (path, headers both ways, 5 Origin rows, CSP/cookie/CORS rules, HTTP/1.1 hop), _DialOrder (`::1` only, `127.0.0.1` only, `::1` wins when both answer, nothing answering), _Methods (GET, POST, PUT, PATCH, DELETE, OPTIONS, HEAD). Every refusal row has a tripwire on the requested port (both loopbacks) counting zero; 7444 gets a best-effort tripwire and the share server's own port none (it is the server).

## Verification

- Log directories (private `mktemp -d`): `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.CBWH7MMwtF` (Task 1), `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.4pES6NxO57` (Task 2), `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.YzVnY25VXD` (Task 3, full suite).
- Tracer gate (Task 1): the whole verify block re-run after the first green and passing before any expansion.
- `go build ./...`, `go vet ./...`, `gofmt -l` clean.
- `GOTOOLCHAIN=go1.26.11 go test -race -short -count=1 ./...`: no `--- FAIL` or `FAIL` line, every package ok (webserver 16.0s, daemon 84.8s, relay 122.4s, capability ok).
- New webserver tests `-race -count=8` and the E2E `-count=3`: ok, no flake.
- IPv6 loopback: this Mac has `::1`, so the `::1`-only and `::1`-wins cases ran (not skipped).
- Mutation checks (each reverted, `git diff` on portproxy.go empty afterwards): skipping the printed check fails "port never printed", IdenticalOpenOrClosed and LogsNoSecrets; skipping the deny list fails the three deny rows; skipping the write-permission check fails "read-only capability"; a blanket Origin rewrite, keeping the dev server's `X-Portty*` response headers, dialling 127.0.0.1 before `::1`, dropping the token's port or grant-id comparison, dropping RawPath, and keeping other `X-Forwarded-*` headers each fail one named test.
- Guards: `git diff --quiet 9574fd49 --` on csp_mw.go, frontend/index.html, capability_test.go and whois_test.go is clean; no `Access-Control-Allow-Origin` in non-test webserver code.

## Commits

- bd1483b9 feat(13-03): host grant and proxy routes reach a loopback dev server through the share server
- 5c424ad3 test(13-03): port grant refuses everything not asked for, identically and without dialling
- 247f6abd test(13-03): proxy route refuses before any dial and proxies byte-exact to the host's own loopback

## Deviations from Plan

**1. TDD order (Tasks 2 and 3).** The routes were written in Task 1 as the plan specifies, so the Task 2 and Task 3 tests were written against existing code and all passed on first run: there was no observed RED step. The mutation checks above stand in as evidence that the tests discriminate.

**2. `newPortReverseProxy` signature.** `newPortReverseProxy(port, sid)` became `ws.newPortReverseProxy(port, sid, target *url.URL)` (the handler has already parsed the target) and uses one shared transport created in `NewWebServer` (see key-decisions). Behaviour per the plan: dialer ignores the host, `DisableCompression`, no HTTP/2, `MaxIdleConnsPerHost` 8, `IdleConnTimeout` 30 s. The dialer additionally refuses a port outside 1..65535 or on the deny list (defence in depth). File: internal/webserver/portproxy.go. Commit bd1483b9.

**3. Extra header deletion.** `Rewrite` deletes every `X-Forwarded*` header, not only the three `ReverseProxy` strips itself, so that `X-Forwarded-Port` and similar cannot reach the dev server (the plan's goal says "no X-Forwarded header"). Covered by TestPortProxy_PathAndHeaders.

**4. Test structure.** The tests build their own share server in `newPPEnv` (rather than `testServerWithHub`) so `SetWhoIsClientForTest` and `SetDeniedPorts` run before `Start`, as in production; `SetDeniedPorts` is unsynchronised by design.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register (T-13-15 to T-13-23): the two new routes and the loopback dial are the surfaces it covers.

## Not verified

- No real tailnet, certificate or tailscaled WhoIs (tests use a fake WhoIs and a self-signed loopback server); the production `[::1]`/`127.0.0.1` dialer ran against real loopback listeners in the webserver tests, but the E2E test uses the `SetPortDialerForTest` seam because both Macs share one machine.
- WebSocket and SSE through the host proxy, revoke and `Stop` ending a live proxied stream: the code is in place (revoke-bound context) but 13-06 owns the tests. No test here exercises a 101 on the host route.
- An old host (API 4) answering 405 to the grant: not tested here (it is the absence of the route; 13-04 owns the client side).
- A request where `ws.manager` has no hub for a session whose grant is still active is refused at the grant; not separately tabled.
- Lint: `golangci-lint` not run.
- TESTING.md (Section 2 manifest, Section 4 traceability, per the repo's standing convention) is not updated here: this plan's file list excludes it and 13-07 owns it (its plan references TESTING.md).
- No GUI or VM checks.

## Self-Check: PASSED

Files present: portproxy.go, portproxy_test.go, portforward_e2e_test.go; commits bd1483b9, 5c424ad3, 247f6abd are ancestors of HEAD.
