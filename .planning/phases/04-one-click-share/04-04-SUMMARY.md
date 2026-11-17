---
phase: 04-one-click-share
plan: 04
subsystem: sharing
tags: [go, daemon, cli, tailscale, websocket, wails, capability]
status: complete

requires:
  - phase: 04-one-click-share
    provides: "04-03: same-account peer routes (GET /api/sessions/meta, POST /api/peer/sessions/{id}/cap), relay.CloseReasonShareStopped"
provides:
  - "POST /peers/connect on the daemon socket: host resolution from tailscaled, meta and cap fetch through the same-account peer routes, RemoteCapStore.Put, typed error codes"
  - "tailnet.MetaStatus (MetaOK / MetaForbidden / MetaUnreachable), tailnet.FetchPeerCap, forbidden peers dropped from FetchAllPeerSessionsMeta"
  - "Remote WebSocket proxy forwards the peer's close code and reason"
  - "portty attach <host>:<session>, bare <host>, local names; stopped-sharing exit; UI-SPEC error copy"
  - "App.ConnectPeerSession(host, sessionID) -> PeerConnectResult (never throws) and its hand-maintained stub"
affects: [04-05 GUI share popover, 04-06 GUI peer wiring of ConnectPeerSession, removal of the old remote-cap stubs, M-80 two-Mac UAT]

actuals:
  tokens: 33600
  tasks: 2
  commits: 2
plan_head_before: c0c8048b4e4789e42a89f816702d7578f40811c3
plan_head_after: f76f08d0ce7308538987a04c3f0536187b8dfb59

tech-stack:
  added: []
  patterns:
    - "Peer seams on API (discoverPeers, probePeers, peerBaseURL) copied from package defaults in NewAPI; tests call SetPeerSeamsForTest. No test reaches tailscaled"
    - "internal/daemon/export_test.go: PutRemoteCapForTest replaces the removed deposit route in the proxy tests"
    - "Connect failures are typed (*daemon.PeerConnectError, mirroring *ShareError); the CLI maps codes to copy, the App binding returns errorCode"

key-files:
  created:
    - internal/daemon/peer_connect.go
    - internal/daemon/peer_connect_test.go
    - internal/daemon/export_test.go
  modified:
    - internal/tailnet/sessions.go
    - internal/tailnet/tailnet_test.go
    - internal/daemon/api.go
    - internal/daemon/types.go
    - internal/daemon/client.go
    - internal/daemon/remote_ws_proxy.go
    - internal/daemon/remote_ws_proxy_test.go
    - internal/daemon/remote_files.go
    - internal/daemon/remote_caps.go
    - internal/daemon/share_removed_test.go
    - cmd_attach.go
    - cmd_attach_test.go
    - cmd_share.go
    - cmd_remote.go
    - cmd_remote_test.go
    - cmd_cli.go
    - cmd_cli_test.go
    - app.go
    - app_test.go
    - frontend/src/wailsjs/go/main/App.d.ts
    - frontend/src/wailsjs/go/main/App.js
    - TESTING.md
  deleted:
    - internal/daemon/open_remote_session_url_test.go
    - internal/daemon/client_remote_files.go
    - app_remote_files_test.go

key-decisions:
  - "FetchPeerSessionsMetaWithClient returns (sessions, MetaStatus, error) so POST /peers/connect can tell a refused port (not sharing) from other failures"
  - "An ambiguous ID prefix on the peer gives no_such_session (the plan's order), not ambiguous; only a name clash gives ambiguous"
  - "Error bodies carry the matched peer's MagicDNS short name in Detail; the CLI prefers it over what was typed (the unknown-host line uses the typed name)"
  - "The several-sessions hint keeps <session> literal: Run 'portty attach office-air:<session>' to attach."
  - "app_remote_files_test.go and client_remote_files.go were deleted whole: everything in them belonged to the removed bindings"

patterns-established:
  - "CLI argument resolution for attach: <host>:<session> is always remote; a bare argument is a local session (exact name, exact ID, unique ID prefix), then a host"

requirements-completed: []

coverage:
  - id: D1
    description: "POST /peers/connect resolves the host (DNS label, HostName, case-insensitive), fetches meta and cap through the same-account routes, stores the cap and returns {sessionId, name, host, url} or a typed error"
    requirement: SHAR-06
    verification:
      - kind: integration
        ref: "internal/daemon/peer_connect_test.go#TestPeerConnect_HostMatching"
        status: pass
      - kind: integration
        ref: "internal/daemon/peer_connect_test.go#TestPeerConnect_StoresCap"
        status: pass
      - kind: integration
        ref: "internal/daemon/peer_connect_test.go#TestPeerConnect_NotSharing"
        status: pass
      - kind: integration
        ref: "internal/daemon/peer_connect_test.go#TestPeerConnect_ClientDecodesError"
        status: pass
    human_judgment: false
  - id: D2
    description: "Peers on a different Tailscale account are refused (forbidden) and dropped from the GUI list and portty list"
    requirement: SHAR-01
    verification:
      - kind: integration
        ref: "internal/daemon/peer_connect_test.go#TestPeerConnect_Forbidden"
        status: pass
      - kind: unit
        ref: "internal/tailnet/tailnet_test.go#TestMetaStatus_TriState"
        status: pass
      - kind: unit
        ref: "internal/tailnet/tailnet_test.go#TestFetchAllPeerSessionsMeta_DropsForbiddenPeers"
        status: pass
      - kind: unit
        ref: "cmd_cli_test.go#TestRemoteListGroups"
        status: pass
    human_judgment: false
  - id: D3
    description: "The proxy forwards the host's close (1000 share stopped / session ended); portty attach prints 'Portty: <host> stopped sharing <session>.' and exits 0 on share stopped"
    requirement: SHAR-04
    verification:
      - kind: integration
        ref: "internal/daemon/remote_ws_proxy_test.go#TestRemoteWSProxy_ForwardsClose"
        status: pass
      - kind: unit
        ref: "cmd_attach_test.go#TestRemoteAttachEnd"
        status: pass
    human_judgment: false
  - id: D4
    description: "Bare-host listing and every attach error print their UI-SPEC copy with the portty prefix; ConnectPeerSession never throws"
    requirement: SHAR-06
    verification:
      - kind: unit
        ref: "cmd_attach_test.go#TestAttach_MultipleSharedListing"
        status: pass
      - kind: unit
        ref: "cmd_attach_test.go#TestPeerConnectMessage"
        status: pass
      - kind: integration
        ref: "app_test.go#TestConnectPeerSession"
        status: pass
    human_judgment: false
  - id: D5
    description: "portty attach office-air:build from Warp between the two real Macs, with the status bar, and the stopped-sharing exit when the host turns Share off"
    requirement: SHAR-06
    verification:
      - kind: manual
        ref: "deferred two-Mac UAT (M-80, added in 04-06)"
        status: pending
    human_judgment: true

duration: ~45min
completed: 2026-11-17
---

# Phase 4 Plan 04: Viewing-side peer connect and CLI attach Summary

**`portty attach <host>:<session>` now works from any terminal: the local daemon finds the Mac in Tailscale's own peer list, gets the shared session and its capability through the same-account routes from 04-03, and the CLI attaches through the daemon's relay proxy. When the host turns Share off, the CLI prints `Portty: <host> stopped sharing <session>.` and exits 0. The GUI gets the same path as `ConnectPeerSession`.**

## POST /peers/connect contract

Request `{"host": "...", "session": "..."}` (8 KiB limit; `session` may be empty). Owner-only daemon socket.

| Status | `error` | When |
|--------|---------|------|
| 200 | — | `{sessionId, name, host, url}`; host is the MagicDNS short name, url is `<base>/sessions/<id>?cap=<token>`; the cap is stored in RemoteCapStore |
| 400 | (plain text) | Bad body or empty host |
| 503 | `tailscale_off` | Peer discovery failed |
| 404 | `unknown_host` | No online peer matches; `hosts` = probed peers' short names |
| 403 | `forbidden` | Meta 403 with the X-Portty marker, or cap 403 |
| 409 | `not_sharing` | Meta 200 with an empty list, or the port refused the connection |
| 502 | `unreachable` | Any other meta, cap or store failure |
| 409 | `multiple` | Empty session and several shared; `sessions` lists them |
| 409 | `ambiguous` | Several sessions with that exact name; `sessions` lists them |
| 404 | `no_such_session` | No name, exact ID or unique ID prefix match, or the cap route said 404; `sessions` = what is shared |

Every error body carries `detail` = the matched host's short name when there is one. The base URL is always `https://<peer DNSName without the trailing dot>:7444`. A URL from the peer's payload is never used. The cap never reaches a log line or an error (redactCapTokenFromError).

**Seams:** `API.discoverPeers` (tailnet.DiscoverPeers), `API.probePeers` (tailnetCache over tailnet.DiscoverAndProbe, also used by GET /tailnet/peers), `API.peerBaseURL` (defaultPeerBaseURL). Tests replace them with `SetPeerSeamsForTest(discover, probe, baseURL)` (nil keeps the current one) and the HTTP client with `SetRemoteFilesClientForTest`.

## Attach argument resolution order

1. No argument: `usage: portty attach <session> | <host>:<session> | <host>`.
2. `<host>:<session>`: always remote (POST /peers/connect with both fields).
3. Bare argument: local `resolveSession` (exact name, exact ID, unique ID prefix). Found: attach locally. Name clash: the clash error.
4. Not found locally: POST /peers/connect with an empty session. `unknown_host` gives the local `no session named` error. `multiple` prints the list and exits 0. One shared session attaches.
5. A remote attach dials `ws://127.0.0.1:<relay>/api/relay/remote/<sessionId>/ws` (no Origin, no cap) and keeps the banner, raw mode, status bar and connection-state watcher.

Every path checks for a terminal on stdin first.

## CLI copy as implemented

| Case | Output |
|------|--------|
| Several shared (stdout, exit 0) | `<host> is sharing <n> sessions:` / `  <name>  <id>` rows (tabwriter) / `Run 'portty attach <host>:<session>' to attach.` |
| Host stops sharing (stdout, exit 0) | `Portty: <host> stopped sharing <session>.` after the terminal is restored and the bar removed |
| tailscale_off | `portty: Tailscale isn't connected. Remote access and sharing are off until it is.` |
| unknown_host | `portty: no Mac named "<host>" is running Portty on your tailnet. Found: <h1>, <h2>.` / none: `portty: no other Macs running Portty were found. Check that Tailscale is connected on both Macs.` |
| not_sharing (or no_such_session with an empty list) | `portty: <host> isn't sharing any sessions. Turn on Share for a session on <host> first.` |
| no_such_session | `portty: <host> has no shared session named "<session>". Shared there: <s1>, <s2>.` |
| ambiguous | `portty: more than one session is named "<x>". Use an ID prefix instead: <id1>, <id2>.` (8-character prefixes) |
| forbidden | `portty: <host> didn't allow access. Sessions are shared only with Macs signed in to the same Tailscale account.` |
| unreachable / other codes | `portty: couldn't connect to <host>. Check that it's awake and signed in to your Tailscale account.` |
| No local session, not a host | `portty: no session named "<x>". Run 'portty list' to see your sessions.` |
| Non-terminal stdin | `portty: attach needs an interactive terminal.` |

No `attach: ` prefix remains in cmd_attach.go.

## Task Commits

1. **Task 1: host:session attach through the daemon, stopped-sharing exit** - `09ec3383` (feat)
2. **Task 2: bare host, error copy, list fix, ConnectPeerSession, old remote-cap routes removed** - `f76f08d0` (feat)

## Decisions Made

See `key-decisions` in the frontmatter. The one with the most weight: `FetchPeerSessionsMetaWithClient` returns a third value (the transport error) so the connect route can map ECONNREFUSED to `not_sharing`. The plan's two-value signature couldn't do that.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Proxy tests deposited caps through the route this plan removes**
- **Found during:** Task 2
- **Issue:** remote_ws_proxy_test.go put its caps in with POST /api/remote-files/caps
- **Fix:** Added internal/daemon/export_test.go (`PutRemoteCapForTest`) and a `depositCap` helper
- **Files modified:** internal/daemon/export_test.go, internal/daemon/remote_ws_proxy_test.go
- **Commit:** f76f08d0

**2. [Rule 1 - Bug] New tailnet tests could hang with no client timeout**
- **Found during:** Task 1 verification. One non-short run left tailnet.test stuck for over 9 minutes in the new tests (likely a reused port on the refused case while other packages ran in parallel).
- **Fix:** TestMetaStatus_TriState and TestFetchPeerCap now run under a 10 s context; ran 3 times clean
- **Files modified:** internal/tailnet/tailnet_test.go
- **Commit:** 09ec3383

**3. [Signature] FetchPeerSessionsMetaWithClient returns (sessions, MetaStatus, error)** instead of the plan's two values (see Decisions). FetchPeerSessionsMeta keeps two values.

**4. [Scope] Whole files deleted:** app_remote_files_test.go and client_remote_files.go held only the removed binding and client methods, so both were deleted outright rather than emptied.

## Known Stubs

None that block this plan. On purpose, the frontend still imports the RegisterRemoteCap / OpenRemoteSessionURL / GetRemoteSessions stubs from App.d.ts and App.js until 04-06 wires ConnectPeerSession. Their Go bindings are gone, so the GUI's old cached-cap peer open path fails until then. TESTING.md M-04 still describes that path, and the FIX-03 vitest row for App.open-remote.test.tsx stays until 04-06 changes that file.

## Threat Flags

None. POST /peers/connect is on the owner-only socket and was already in the plan's threat model (T-04-16..T-04-19).

## Verification

- `go build ./... && go vet ./... && go test -race -short ./...`: pass
- `go vet -tags playwrightfixture ./cmd/playwright-fixture/` and `go vet -tags vmpreview ./...`: pass
- `pnpm -C frontend exec tsc --noEmit` and `vitest run` (93 files, 1204 tests): pass
- `bash tests/check-traceability-paths.sh` (114 paths) and `bash tests/check-brand.sh`: pass
- Built binary `--help` shows both new attach lines
- `pgrep -f '\.test daemon'` prints nothing after the runs

VM checks were not needed: this plan has no GUI changes, and every connect path is covered with fakes. The vmpreview WhoIs fake noted in 04-03 is still open for 04-06's GUI checks.

## Self-Check: PASSED

- FOUND: internal/daemon/peer_connect.go, internal/daemon/peer_connect_test.go, internal/daemon/export_test.go
- FOUND commits: 09ec3383, f76f08d0 (both ancestors of HEAD)
- GONE as intended: internal/daemon/open_remote_session_url_test.go, internal/daemon/client_remote_files.go, app_remote_files_test.go
