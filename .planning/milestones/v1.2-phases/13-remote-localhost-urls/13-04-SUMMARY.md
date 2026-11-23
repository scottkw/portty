---
phase: 13-remote-localhost-urls
plan: 04
subsystem: daemon-client-and-app-binding
tags: [go, wails-binding, daemon-api, versioning, typed-errors]
requires:
  - phase: 13-02
    provides: POST /peers/ports, PortOpenRequest/Response, PortErr* codes
  - phase: 13-03
    provides: host grant and proxy routes (used by the end-to-end binding test through a stub peer)
provides:
  - APIVersion 5 and PortProxyAPI = 5 (MinDaemonAPI stays 1)
  - DaemonClient.OpenRemotePort and *PortOpenError
  - RemotePortResult and App.OpenRemotePort (never throws)
affects: [13-05 click branch, consent sheet and notices (TypeScript stub of this signature), 13-06, 13-07]
status: complete
actuals:
  tokens: 9000
  tasks: 2
  commits: 2
plan_head_before: 91b5590bb65167579a1d1d6e9be8eebdc8e89918
plan_head_after: 8c8f9c6f8190c1aa8672552443fbf96060c191b2
tech-stack:
  added: []
  patterns:
    - "binding order mirrors ConnectPeerSession: nil client, health, version gate (no route call below PortProxyAPI), route, typed error, else unreachable"
    - "client maps JSON error body to a code; 404/405 without one means an older mux (daemon_outdated); anything else is a plain error the binding reports as unreachable"
key-files:
  created: []
  modified:
    - internal/daemon/types.go
    - internal/daemon/client.go
    - internal/daemon/client_test.go
    - internal/daemon/api_test.go
    - app.go
    - app_test.go
key-decisions:
  - "No new decisions beyond the plan's flagged ones; MinDaemonAPI left at 1 so an API 4 daemon shows no Update needed banner (only this click says daemon_outdated)"
requirements-completed: []  # URL-01 and URL-02 need 13-05 (click branch) to be delivered end to end; the plan forbids requirements.mark-complete
---

# Phase 13 Plan 04: App binding for remote localhost URLs Summary

`App.OpenRemotePort(sessionID, rawURL, consent)` asks the local daemon to open a loopback forward for a clicked remote `localhost` URL and returns only status, url, host, port and errorCode; every failure is a stable code, and an API 4 daemon is reported as `daemon_outdated` without its route ever being called.

## Accomplishments

- `types.go`: `APIVersion` 4 to 5 (doc comment extended), new `PortProxyAPI = 5` next to `PathClickAPI`.
- `client.go`: `PortOpenError{Code}` and `DaemonClient.OpenRemotePort`, modelled on `ConnectPeer`: 200 decodes `PortOpenResponse`; 400+ with a JSON `error` gives `*PortOpenError{Code}`; 404/405 without one gives `daemon_outdated`; anything else is a plain error.
- `app.go`: `RemotePortResult` and `App.OpenRemotePort` placed after `ConnectPeerSession`; order is nil client (unreachable), `HealthInfo` failure (unreachable), `DaemonAPI(h) < PortProxyAPI` (daemon_outdated, no route call), route, `*PortOpenError` code, else unreachable.
- Old-host 405 (13-03's deferred item): the viewing daemon already maps a grant 404/405 without the marker to `host_outdated` (13-02, `requestPortGrant`). This plan's client and binding pass that code through unchanged (covered by the `host_outdated` row of `TestOpenRemotePortClient` and `TestOpenRemotePort_Codes`). The 404/405 mapping added here is for the local daemon's own mux lacking `/peers/ports`.

## Tests added or edited

- `app_test.go`: `TestOpenRemotePort` (stub peer; consent first with port and host and no URL, then open with `http://localhost:<N>/x?y=1`, then a GET of `http://127.0.0.1:<N>/x?y=1` returns `remote page` with the host seeing the port token and target), `TestOpenRemotePort_DaemonOutdated` (API 4 stub: handler called 0 times; API 5 with no route: daemon_outdated), `_Codes` (six codes unchanged, Status and URL empty; plain 500 is unreachable), `_NoSecrets` (marshalled consent and open results contain no `TOKEN`/`PTOKEN`), `_NoDaemon`; helpers `freeLoopbackPortForTest`, `newRemotePortFixture`, `stubDaemonApp`, `healthMux`.
- `internal/daemon/client_test.go`: `TestOpenRemotePortClient` (six JSON codes, plain 404 and 405, plain 500 not a `*PortOpenError`, 200 decode and request body); `TestHealthInfo` now pins API 5, PortProxyAPI 5, PathClickAPI 4, PreviewAPI 3 and MinDaemonAPI 1.
- `internal/daemon/api_test.go`: `TestAPIHealth` pins `"api":5` and `"handoff":1`.
- Stub unix sockets use `os.MkdirTemp("", "pp")`.

## Verification

- Log directories (private `mktemp -d`): `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.hkNUW0fFBJ` (Task 1 tracer), `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.y7r3zG7TwL` (Task 2 targeted), `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.k7kkl8cObg` (full suite).
- Tracer gate (Task 1): verify block passed end to end before Task 2 started.
- `GOTOOLCHAIN=go1.26.11 go build ./... && go vet ./...` clean; `gofmt -l` clean on touched files.
- `go test -race -short -count=1 ./...`: rc 0, zero `--- FAIL`/`FAIL` lines, every package `ok` (root 13.9s, daemon 85.2s, relay 122.9s, webserver 16.5s).
- Mutation check: disabling the `DaemonAPI(h) < PortProxyAPI` gate fails `TestOpenRemotePort_DaemonOutdated`; reverted, tree clean afterwards.
- All Task 2 greps pass, including `MinDaemonAPI = 1`.
- TDD note: the Task 2 tests passed on first run because Task 1's tracer already contained the client and binding; the mutation check above is the evidence that the new tests bite.

## Commits

- 7215d88e feat(13-04): app binding opens a remote localhost page, asking consent first
- 8c8f9c6f test(13-04): every binding failure is named, nothing secret leaves it, version pins move to 5

## Deviations from Plan

None - plan executed exactly as written. Per the dispatch, `state.record-session` was skipped, no worktree was used, and commits carry the dispatch's `Co-Authored-By` trailer (plain `git commit`).

## Known Stubs

None.

## Threat Flags

None. No new endpoint, auth path or file access; `RemotePortResult` carries no credential (T-13-25), a version mismatch is `daemon_outdated` (T-13-24), the binding never throws (T-13-26).

## Not verified

- No GUI or browser-driven check (13-05 owns the frontend call, 13-07 the VM checks).
- `frontend/src/wailsjs` untouched, so the generated Wails binding for `OpenRemotePort` does not exist yet; 13-05 writes its stub.

## Self-Check: PASSED

Files found: internal/daemon/types.go, client.go, client_test.go, api_test.go, app.go, app_test.go. Commits 7215d88e and 8c8f9c6f are ancestors of HEAD.
