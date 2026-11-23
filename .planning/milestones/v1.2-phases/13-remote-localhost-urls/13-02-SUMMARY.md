---
phase: 13-remote-localhost-urls
plan: 02
subsystem: daemon
tags: [go, reverse-proxy, loopback, consent, port-forward, tailnet]
requires:
  - phase: 13-01
    provides: host primitives (not used yet by this plan; it runs against a stub host)
provides:
  - POST /peers/ports on the daemon unix socket (PortOpenRequest/Response, PortErr* codes)
  - PortForwards (approvals, forwards, RevokeSession, CloseAll) and the hop-1 reverse proxy
  - classifyLoopbackURL and the shared fixture localhost-url.json
  - peerTransport extracted from peerHTTPClient
affects: [13-03 host routes, 13-04 binding, 13-05 click branch and consent sheet, 13-06 revoke and security review, 13-07 TESTING.md and VM checks]
status: complete
actuals:
  tokens: 16800
  tasks: 3
  commits: 4
plan_head_before: a42af6bcb4da303bb5446452c397367dea9fa2ca
plan_head_after: ffa7414685c9086449406ad5cad51226615460b6
tech-stack:
  added: []
  patterns:
    - "approval keyed by (Mac, port) holding session id and SHA-256 of the capability; live only while that session still holds that capability"
    - "probe both loopbacks then bind both; approval recorded only after the forward is installed"
    - "forward BaseContext cancelled on close so in-flight requests and upgraded tunnels end with it"
key-files:
  created:
    - internal/daemon/portforward.go
    - internal/daemon/portforward_api.go
    - internal/daemon/portforward_test.go
    - frontend/src/lib/__tests__/fixtures/localhost-url.json
  modified:
    - internal/daemon/peer_client.go
    - internal/daemon/api.go
    - internal/daemon/handoff.go
    - internal/daemon/remote_ws_proxy.go
    - internal/daemon/peer_connect.go
key-decisions:
  - "Hop-1 URL host is the stored base URL's host (hostname:port), not a bare fqdn, so the test seam (stub on 127.0.0.1:port) and production (dial override to the tailnet IP) share one code path"
  - "Stream-close revocation requires end.side == peer: a close frame sent by the local webview cannot revoke forwards"
  - "A request target over 8192 bytes or not starting with / is refused on the forward (414/400) because the host would answer the same 403 as a stopped share and the forward would close for a long URL"
requirements-completed: []  # URL-01 and URL-02 need 13-03 (host routes) and 13-05 (click) to be delivered end to end; the plan forbids requirements.mark-complete
---

# Phase 13 Plan 02: Viewing-side loopback forward Summary

POST /peers/ports turns a clicked remote `localhost` URL into a forward on 127.0.0.1:P and [::1]:P after a host grant and per-(Mac, port) consent, with a hop-1 reverse proxy that carries the request target, capability and port token in headers; forwards and approvals end with the share, with no request needed.

## Accomplishments

- `portforward_api.go`: types and codes of the interface contract, `classifyLoopbackURL`, `openedURL`, `requestPortGrant` (200 token, 403+marker refused, 404/405 without marker host_outdated, everything else unreachable) and `handlePortOpen` in the plan's order (classify, held cap, grant, drop stale approval, another Mac holding P, rebind or probe/consent/bind/install/record).
- `portforward.go`: `PortForwards`, `portForward`, `portFree` (dial-probe both loopbacks, fail closed), `bindLoopback` (tcp4 127.0.0.1 plus tcp6 ::1), `forwardHostAllowed`, `forwardTransport`, the `httputil.ReverseProxy` (Rewrite deletes inbound `X-Portty*`, sets Target/Cap/Port-Token; ModifyResponse maps host 403/502 with marker and deletes `X-Portty*` from responses; ErrorHandler gives the three fixed pages and closes on stopped/ECONNREFUSED), the cross-site check, `RevokeSession`, `CloseAll`.
- Wiring: route on the socket mux only, `Stop()` and `closeListenerCopies` call `CloseAll`, the session stream's peer-side terminal close (`relay.IsTerminalClose`) and a changed capability in `handlePeerConnect` call `RevokeSession`.
- `peerTransport(fqdn, addr)` extracted; `peerHTTPClient` unchanged in behaviour.

## Tests added (internal/daemon/portforward_test.go, package daemon)

TestPortForward_OpenThenGET, _ConsentFirst, _HostRefused, _CrossSite, _HeaderHygiene, _StoppedSharing, _NothingAnswering, _HostGone, _StreamCloseRevokes (share stopped, session ended, service restart), _NewCapRevokes, _StopReshareClickAsksConsent, _CloseAll (Stop, handoff), _NotOnRelayHandler, _NoDiskWriteAPIs, _LogsNoSecrets, _WebSocketUpgrade (added beyond the plan: the plan's goal says upgrades are carried, and nothing else exercised a 101); TestClassifyLoopbackURL_Fixture (28 cases); TestPortOpen_ErrorCodes (9 rows), _PortInUse (wildcard holder, ::1-only holder), _Approvals, _ApprovalLiveCheck, _NoApprovalAfterPortInUse; TestForwardHostAllowed (13 rows). Shared fixture: `frontend/src/lib/__tests__/fixtures/localhost-url.json`, 28 cases (14 matching, 14 not).

IPv6 loopback: this Mac has `::1`, so the `::1`-only holder case ran (not skipped).

## Verification

- Log directories (private `mktemp -d`): `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.xKV8KdBIAF` (Task 1), `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.dM2GBJwc0w` (Task 2), `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.IabgqBVWve` (Task 3 full suite).
- Tracer gate (Task 1): verify block re-run end to end after the first green, passing before any expansion.
- `go build ./...`, `go vet ./...` clean; `gofmt -l` clean.
- `GOTOOLCHAIN=go1.26.11 go test -race -short -count=1 ./...`: rc 0, no `--- FAIL` or `FAIL` line, daemon ok (85.6s), relay ok (121.9s), every package ok.
- New tests re-run `-race -count=8`: ok (no flake). After the last test commit, the new tests plus PeerConnect, RemoteCap and RemoteSession: ok.
- Mutation checks (each reverted, tree clean afterwards): making the approval live check always true fails TestPortOpen_ApprovalLiveCheck; removing the stream-close, new-capability and handoff hooks fails TestPortForward_StreamCloseRevokes, _NewCapRevokes and _CloseAll/handoff. TestPortForward_StopReshareClickAsksConsent still passes with the hooks removed, because the live fingerprint check catches the changed capability (defence in depth, as designed).
- Task 3 greps pass; `git diff --quiet 9574fd49 -- internal/webserver/csp_mw.go frontend/index.html` is clean.

## Commits

- 19019a18 feat(13-02): loopback forward for a clicked remote localhost URL
- 6bbe7234 test(13-02): consent, approvals, port in use, grant codes and the shared classifier fixture
- b1abcabc feat(13-02): forwards end with the share: revoke on terminal close, new capability, Stop and handoff
- ffa74146 test(13-02): a WebSocket upgrade crosses the forward and ends when the forward closes

## Deviations from Plan

**1. [Rule 2 - Missing critical functionality] Request target guard.** The host refuses a target over 8192 bytes with the same 403+marker as a stopped share, so a single long URL would close the forward and drop consent. The forward now answers 414 (over 8192 bytes) or 400 (target not starting with `/`) with a fixed body before proxying. File: internal/daemon/portforward.go. Commit 19019a18.

**2. [Rule 2 - Hardening] Revocation only from the peer side.** The plan says to call `RevokeSession` when `relay.IsTerminalClose(err)` holds. The code also requires `end.side == "peer"`, so a close frame sent by the local webview cannot revoke forwards. File: internal/daemon/remote_ws_proxy.go. Commit b1abcabc.

**3. Hop-1 URL host.** The plan text says `https://<fqdn>`; the code uses the stored base URL's host (`hostname:port`). Production behaviour is the same (the transport dials the tailnet IP whatever the URL names and verifies the certificate against the fqdn); it lets the stub host on 127.0.0.1:port work without a second code path.

**4. Additional test.** TestPortForward_WebSocketUpgrade is not in the plan (see above).

**5. TDD order.** The implementation was written ahead of the Task 2 and Task 3 tests, so there was no separate observed RED step for them; all tests passed on first run except TestPortForward_NothingAnswering, whose expected text was wrong (the apostrophe in `{host}'s` is literal, only the label is escaped); the test was fixed, not the code. The mutation checks above stand in as evidence that the security tests discriminate.

TDD note on Task 1 vs Task 2: the full grant-answer mapping and the approval flow went into Task 1's commit because `handlePortOpen` is one function; Task 2's commit adds the fixture and tests.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register: the one new listener surface (loopback forwards) and the one new socket route are the ones T-13-06 to T-13-14 cover.

## Not verified

- Nothing was run against a real host: 13-03 builds the host routes and proves both daemons together. The stub speaks the contract as written in the plan.
- No browser, VM or GUI check; `lsof` for wildcard binds (13-07 VM row) was not run, only the explicit `tcp4 127.0.0.1` and `tcp6 [::1]` binds in code.
- Real tailnet transport (`peerTransport` path through `forwardTransport` outside tests) is exercised only by the unchanged callers; the `forwardTransport` production branch has no test (it needs a live tailnet).
- A forward keeps the tailnet IP it was opened with; a re-click rebinds session and token but does not refresh the transport's address (Tailscale IPs are stable per node).
- Lint: `golangci-lint run ./internal/daemon/` reports 11 errcheck findings in portforward_test.go (unchecked `Close`/`WriteString` in test helpers), the same class as most of the package's 47 other findings; the production files are clean.
- TESTING.md (Section 2 manifest, Section 4 traceability) is not updated here: this plan's file list does not include it and 13-07 owns TESTING.md per the phase plan.

## Self-Check: PASSED

Files present: portforward.go, portforward_api.go, portforward_test.go, localhost-url.json; commits 19019a18, 6bbe7234, b1abcabc, ffa74146 are ancestors of HEAD.
