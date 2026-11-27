---
phase: 21-portty-app-and-iphone-viewers
plan: 06
subsystem: daemon
tags: [go, websocket, tailnet, desktop, proxy]
requires:
  - phase: 21-01
    provides: tailnet.FetchPeerDesktopCap, ErrPeerDesktopOff, POST /api/peer/desktop/cap
provides:
  - DesktopCapStore (in-memory peer desktop tokens keyed by MagicDNS name)
  - POST /peers/desktop and GET /peers/desktop/link on the daemon socket
  - GET /api/relay/remote-desktop/{host}/ws loopback signalling proxy
  - DaemonClient.ConnectPeerDesktop and PeerDesktopLink
affects: [21-05, 21-07, 21-13]
tech-stack:
  added: []
  patterns:
    - "desktop twin of the terminal proxy: lookup before upgrade, Origin injection, request-context copy loops"
    - "refusal-to-close-code mapping with fixed literal reasons"
key-files:
  created:
    - internal/daemon/desktop_caps.go
    - internal/daemon/desktop_caps_test.go
    - internal/daemon/peer_desktop.go
    - internal/daemon/peer_desktop_test.go
    - internal/daemon/remote_desktop_ws_proxy.go
    - internal/daemon/remote_desktop_ws_proxy_test.go
  modified:
    - internal/daemon/api.go
    - internal/daemon/client.go
    - internal/daemon/relay_remote_files.go
key-decisions:
  - "connectPeerDesktop is one helper behind both POST /peers/desktop and the proxy's single refresh, so the refresh path cannot drift from the connect path; its error (*peerDesktopFail) carries only a stable code"
  - "A refresh that fails with desktop_off or forbidden closes 4410 or 4403; any other refresh failure is 1013"
  - "Only a 401 carrying X-Portty-Desktop: token refreshes; a 403 without the X-Portty marker is 1013, not 4403"
status: complete
actuals:
  tokens: 12500
  tasks: 3
  commits: 3
plan_head_before: 610960d23c27b9a59813f48ae267692b6c297396
plan_head_after: 3fbed4ef38a9ada978b769cd1711a8e883a8322c
metrics:
  duration: ~1h
  completed: 2026-11-27
---

# Phase 21 Plan 06: Home-daemon desktop token store and loopback proxy Summary

The viewing daemon now holds a peer's standing desktop token in memory and exposes a loopback WebSocket proxy, so the Wails web view can reach a peer's /desktop/ws signalling without ever seeing the token.

## What was built

- **DesktopCapStore** (`desktop_caps.go`): memory-only map keyed by lower-cased MagicDNS name (trailing dot removed). `Put` rejects an empty host or token and any base URL that is not https with a host. A source-grep test keeps file-writing calls out of the file.
- **POST /peers/desktop** (`peer_desktop.go`): matches the host against tailscaled's peer list, builds the base URL from the peer's DNS name, fetches the token through the verified peer client, stores it, and answers `{host, name}` with no token. Outcomes: 403 forbidden, 404 or refused share port gives 409 `desktop_off` (and deletes the stored token), anything else 502 `unreachable`, discover error 503 `tailscale_off`, no match 404 `unknown_host` with the probed short names, empty host 400.
- **GET /peers/desktop/link?host=** (socket only): returns `{url}` = base URL + `/desktop/?cap=<escaped token>` from the store, no network call, 404 when nothing is held. Not on the relay mux (tested).
- **Freeze gate**: POST /peers/desktop is exempt beside /peers/connect.
- **Proxy** (`remote_desktop_ws_proxy.go`): lookup before upgrade (JSON 404 on a miss), `relay.LoopbackOriginPatterns`, Origin set to the base URL, dial `wss://<peer>/desktop/ws?cap=<token>` with only the stored token (a cap in the inbound URL is ignored), copy loops and heartbeat on the request context, peer close frames forwarded. Refusals map to fixed closes: 401 token gives one refresh and redial then 4401 `token`; 410 gives 4410 `off`; 403 with the X-Portty marker gives 4403 `forbidden`; anything else 1013 `remote unreachable`. Neither leg sets a read limit (32768 default).
- **DaemonClient**: `ConnectPeerDesktop(host)` (returns `*PeerConnectError`) and `PeerDesktopLink(host)`.

## Verification

- `go vet`, `gofmt -l` clean; brand gate clean.
- `go test -race -count=3` of the new tests and `go test -race -count=1 ./internal/daemon/` (full package) pass.
- golangci-lint `--new-from-rev=060281bd`: nothing from this plan's code. Remaining reports are in code this plan did not write (`client.go` lines 319, 337, 351; `process.go`; `restart_pin_test.go`).
- The tracer runs against the real 21-01 cap route and a real always-available share server (Phase 19 test env, synthetic source): POST /peers/desktop then a loopback socket with a `wails://wails` Origin receives the host's hello.

## New test files (for 21-13: TESTING.md Sections 2 and 4)

- `internal/daemon/desktop_caps_test.go` (TestDesktopCapStore_*)
- `internal/daemon/peer_desktop_test.go` (TestPeerDesktop_*, includes the real-share-server tracer)
- `internal/daemon/remote_desktop_ws_proxy_test.go` (TestRemoteDesktopWS_*; the long-lived case sleeps 11 s past the dial timeout and is skipped under `-short`)

Requirement covered: DVWR-01 (home-daemon side). No manual M-NN item: nothing here needs a GUI, live PTY or remote peer beyond the VM rows other plans own.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Task 1 committed the complete `peer_desktop.go`**
- **Issue:** the plan scopes Task 1 to the happy path and Task 3 to the error switch and link handler, but the error mapping, refresh helper and link handler are a few lines each and share one helper with the proxy's refresh.
- **Fix:** wrote `peer_desktop.go` whole in Task 1, with the link handler left unregistered. Task 3's tests were still written first and failed to compile (no client methods, no link route) before the registration, freeze exemption and client methods landed. The error-switch tests passed immediately rather than going red first.
- **Commits:** e9cc6911, 3fbed4ef

**2. [Rule 1 - Lint] errcheck on deferred closes**
- Wrapped `CloseNow` and `Close` calls in the new files as `_ = x.CloseNow()` so lint reports nothing new.

### Other

- New tests live in package `daemon` (the plan's files list gives no package); `captureDiagLog` is in package `daemon_test`, so `desktopLogBuf` is a small local equivalent.

## Authentication Gates

None.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register. T-21-15 (a local process opening the proxy) is accepted as planned; T-21-16 through T-21-20 are mitigated and asserted by tests: no token in logs or close reasons (`TestRemoteDesktopWS_NeverLogsTheToken`), no disk write (`TestDesktopCapStore_NoDiskWriteAPIs`), link route absent from the relay mux (`TestPeerDesktop_LinkRoute`), exactly one refresh per dial (`TestRemoteDesktopWS_TokenRefusedTwiceCloses4401`).

## Commits

- e9cc6911 feat(21-06): loopback desktop proxy and POST /peers/desktop tracer
- a99c7b3b feat(21-06): desktop proxy refusal mapping, one token refresh, redaction
- 3fbed4ef feat(21-06): peer desktop link route, freeze exemption and client methods

## Self-Check: PASSED

Files created exist, and the three task commits are ancestors of HEAD.
