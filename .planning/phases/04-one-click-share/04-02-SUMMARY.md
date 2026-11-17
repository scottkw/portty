---
phase: 04-one-click-share
plan: 02
subsystem: sharing
tags: [go, daemon, webserver, capability, tailscale, cli, wails]
status: complete

requires:
  - phase: 04-one-click-share
    provides: "04-01: one read,write capability per share; single TLS listener; no Funnel, join codes, QR or view-only links"
provides:
  - "Daemon share registry: GET/PUT /sessions/{id}/share, one in-memory record per session, share server runs exactly while a record exists"
  - "portty share <session> / portty unshare <session> with resolveSession (exact name, exact ID, unique ID prefix)"
  - "App bindings GetSessionShare(id) / SetSessionShared(id, shared) returning ShareResult; stubs in frontend/src/wailsjs/go/main/App.{d.ts,js}"
  - "webserver.ErrPortInUse: a taken share port is an error, never a random port"
  - "vmpreview build tag: fake connected Tailscale + self-signed loopback share server on 127.0.0.1:7444 for the Tart VM"
  - "Brand gate check that the vmpreview tag never reaches .github, build.sh or scripts (tracked or untracked)"
affects: [04-03 same-account peers and reason-carrying revoke, 04-04 remote attach, 04-05 GUI share popover, 04-06 removal gate]

actuals:
  tokens: 46951
  tasks: 2
  commits: 2
plan_head_before: c0359e0a2adcafd9b2597a76c332fa9710e3855b
plan_head_after: a781a66b2c95756dadf54ca546d9aad2a96853bf

tech-stack:
  added: []
  patterns:
    - "Share seams (shareHealth, newShareServer, warmCert) are package defaults copied into each API by NewAPI; tests call API.SetShareSeamsForTest, vmpreview overrides the defaults in init()"
    - "Daemon share errors travel as {error, detail} JSON and come back from DaemonClient.SetSessionShared as *daemon.ShareError; App bindings turn them into ShareResult.errorCode instead of throwing"
    - "a.mu is never held across the Tailscale health check, certificate warm-up or server Start; the winner of a start race installs its server, the loser stops its own"

key-files:
  created:
    - internal/daemon/share.go
    - internal/daemon/share_test.go
    - internal/daemon/share_vmpreview.go
    - cmd_share.go
    - cmd_share_test.go
    - tailscale_vmpreview.go
  modified:
    - internal/daemon/api.go
    - internal/daemon/types.go
    - internal/daemon/client.go
    - internal/daemon/process.go
    - internal/webserver/server.go
    - app.go
    - cmd_cli.go
    - main.go
    - tests/check-brand.sh
    - frontend/src/wailsjs/go/main/App.d.ts
    - frontend/src/wailsjs/go/main/App.js
    - TESTING.md
  deleted:
    - internal/webserver/auth.go
    - internal/webserver/auth_test.go
    - internal/webserver/localip.go
    - internal/webserver/localip_test.go
    - internal/daemon/api_disconnect_test.go

key-decisions:
  - "The share server factory seam takes the HubManager: func(TailscaleHealth, *relay.HubManager) (*WebServer, error), because NewWebServer needs the manager and the plan's one-argument signature could not build a server"
  - "ErrPortInUse is returned only for EADDRINUSE on a non-zero port; other listen failures stay plain errors (start_failed)"
  - "GetSessionShare errors (daemon unreachable, nil client) use errorCode unavailable; SetSessionShared uses start_failed / stop_failed as planned"
  - "The old share-off disconnect test (web viewers closed, owner's local subscriber kept) was ported to TestShare_OffDisconnectsWebViewersOnly rather than dropped with api_disconnect_test.go"
  - "Key regeneration re-mints every share under a.mu (disconnect web viewers, clear grants, new grant); a share that cannot be re-minted is turned off"

patterns-established:
  - "Share route contract (below) is the single state every entry point flips"

requirements-completed: []

coverage:
  - id: D1
    description: "PUT /sessions/{id}/share {shared:true} mints one read,write capability, starts the share server once and returns a working link; the same call while shared returns the same link"
    requirement: SHAR-01
    verification:
      - kind: integration
        ref: "internal/daemon/share_test.go#TestShare_OnReturnsWorkingLink"
        status: pass
      - kind: integration
        ref: "internal/daemon/share_test.go#TestShare_Idempotent"
        status: pass
    human_judgment: false
  - id: D2
    description: "The link opens an interactive full-control terminal: GET redirects to /app/, a WebSocket with the link's cap and Origin echoes typed input; claims perms are exactly read,write"
    requirement: SHAR-03
    verification:
      - kind: integration
        ref: "internal/daemon/share_test.go#TestShare_OnReturnsWorkingLink"
        status: pass
    human_judgment: false
  - id: D3
    description: "Share-off drops the record, clears grants, disconnects web viewers only, stops the server when nothing is shared; re-share mints a new token and the old one fails; exit and kill do the same; key regeneration re-mints links"
    requirement: SHAR-04
    verification:
      - kind: integration
        ref: "internal/daemon/share_test.go#TestShare_OffStopsServerAndNewLinkDiffers"
        status: pass
      - kind: integration
        ref: "internal/daemon/share_test.go#TestShare_OffDisconnectsWebViewersOnly"
        status: pass
      - kind: integration
        ref: "internal/daemon/share_test.go#TestShare_KillDropsRecordAndStopsServer"
        status: pass
      - kind: integration
        ref: "internal/daemon/share_test.go#TestShare_ExitCleanupDropsRecord"
        status: pass
      - kind: integration
        ref: "internal/daemon/share_test.go#TestShare_KeyRegenRemintsLinks"
        status: pass
    human_judgment: false
  - id: D4
    description: "Tailscale off gives 503 tailscale_off, HTTPS certificates off gives 409 https_off with no server built, unknown session 404 not_found, taken port ErrPortInUse"
    requirement: SHAR-01
    verification:
      - kind: unit
        ref: "internal/daemon/share_test.go#TestShare_TailscaleOff"
        status: pass
      - kind: unit
        ref: "internal/daemon/share_test.go#TestShare_HTTPSOff"
        status: pass
      - kind: unit
        ref: "internal/daemon/share_test.go#TestShare_UnknownSession"
        status: pass
      - kind: unit
        ref: "internal/webserver/server_test.go#TestStart_TakenPortIsErrPortInUse"
        status: pass
    human_judgment: false
  - id: D5
    description: "portty share / unshare print the UI-SPEC copy (CT line on the first share), resolve sessions by name, ID or unique prefix; serve, unserve and web are gone from help and dispatch"
    requirement: SHAR-01
    verification:
      - kind: unit
        ref: "cmd_share_test.go#TestResolveSession"
        status: pass
      - kind: integration
        ref: "cmd_share_test.go#TestCmdShare_PrintsLinkAndCTOnFirstShare"
        status: pass
      - kind: integration
        ref: "cmd_share_test.go#TestCmdShare_Errors"
        status: pass
      - kind: integration
        ref: "cmd_share_test.go#TestCmdUnshare"
        status: pass
      - kind: other
        ref: "built binary --help has `share <session>` and no serve/unserve/web line"
        status: pass
    human_judgment: false
  - id: D6
    description: "No server at boot; LAN fallback, local password, upgrade watcher and the seven old lifecycle routes are gone"
    requirement: SHAR-01
    verification:
      - kind: unit
        ref: "internal/daemon/share_test.go#TestShare_NoServerAtBoot"
        status: pass
      - kind: unit
        ref: "internal/daemon/share_removed_test.go#TestShareRemoved_DaemonRoutes"
        status: pass
      - kind: other
        ref: "Task 2 git grep gates (LAN/password/auto-start/Config.Mode; old routes) print nothing; grep -c CheckHealth internal/daemon/process.go = 0"
        status: pass
    human_judgment: false
  - id: D7
    description: "GUI bindings GetSessionShare / SetSessionShared return ShareResult with errorCode, never a Go error"
    requirement: SHAR-01
    verification:
      - kind: integration
        ref: "app_test.go#TestSessionShareBindings"
        status: pass
      - kind: unit
        ref: "app_test.go#TestSessionShareBindings_NilClient"
        status: pass
    human_judgment: false
  - id: D8
    description: "vmpreview build compiles and vets; the brand gate fails (exit 1, names the file) when an untracked file under scripts/ sets the tag, and is clean otherwise"
    requirement: SHAR-01
    verification:
      - kind: other
        ref: "go vet -tags vmpreview ./... && go build -tags vmpreview; probe file scripts/vm-tag-probe.txt -> check-brand.sh exit 1 naming it"
        status: pass
    human_judgment: false
  - id: D9
    description: "M-78: in the Tart VM a vmpreview build shares from the CLI on 127.0.0.1:7444, Safari opens a working terminal, unshare stops it and frees the port"
    verification: []
    human_judgment: true
    rationale: "Needs the built app in the VM and a real Safari on the fixed port; not run by this executor (end-of-phase human verify)"

duration: 20min
completed: 2026-11-17
---

# Phase 4 Plan 02: One Share Switch Summary

**The daemon now owns one in-memory share record per session behind `PUT /sessions/{id}/share`: share-on starts the tailnet share server once and returns a full-control link, share-off revokes, disconnects and stops the server when nothing is shared, and `portty share` / `portty unshare` and two new App bindings flip that same state. No server starts at boot and the password-gated LAN fallback is gone.**

## Performance

- Duration: about 20 min
- Tasks: 2 of 2
- Files: 39 changed (+1398 / -2284), 6 created, 5 deleted

## Share route contract

Owner-only daemon socket.

| Request | Result |
|---------|--------|
| `GET /sessions/{id}/share` | 200 `{shared:false}` or `{shared:true, url}` |
| `PUT /sessions/{id}/share {"shared":true}` | 200 `{shared:true, url, firstShare?}`; repeated while shared returns the same url |
| `PUT ... {"shared":false}` | 200 `{shared:false}` (also when not shared) |
| bad body | 400 plain text |

Error body `{error, detail}`:

| Code | Status | When | Detail |
|------|--------|------|--------|
| `not_found` | 404 | session missing or stopped | `no running session <id>` |
| `tailscale_off` | 503 | not Connected or no IP | `Tailscale isn't connected` |
| `https_off` | 409 | no HasCerts or no Domain (no server built) | `HTTPS certificates are off for your tailnet` |
| `port_in_use` | 409 | 7444 taken (ErrPortInUse) | `port 7444 is in use` |
| `start_failed` | 500 | cert warm-up, server build/start, signing, race | the error text (`couldn't get an HTTPS certificate: …`) |

The link is `https://<fqdn>:7444/sessions/<id>?cap=<token>` (token query-escaped). `firstShare` is true when `<configDir>/ct_disclosed` did not exist; the daemon writes it ("1", 0600) after that share. `SessionInfo.webEnabled` is true exactly when a record exists.

App: `ShareResult {shared, url, firstShare, errorCode, errorDetail}`. Daemon codes pass through; an unreachable daemon gives `start_failed` / `stop_failed` from SetSessionShared and `unavailable` from GetSessionShare.

## Seams

- `defaultShareHealth` (webserver.CheckHealth), `defaultNewShareServer` (Config{BindIP: h.IP, Port: 7444, FQDN: h.Domain}), `defaultWarmCert` (local.Client.CertPair) in internal/daemon/share.go, copied into each API by NewAPI.
- `(*API).SetShareSeamsForTest(health, newServer, warmCert)` for tests.
- `defaultTailscaleCheck` in app.go seeds `App.tailscaleCheckFunc` (nil in normal builds).

## vmpreview build (Tart VM only)

`wails build -platform darwin/arm64 -tags "wailsassets vmpreview"` (afterwards `git checkout -- frontend/src/wailsjs/wailsjs/`). internal/daemon/share_vmpreview.go and tailscale_vmpreview.go fake a connected Tailscale (IP and domain 127.0.0.1) and serve shares on 127.0.0.1:7444 with a self-signed certificate. tests/check-brand.sh, which build.yml already runs, fails if the tag appears in .github, build.sh or scripts (tracked or untracked).

VM human-check (M-78): not run by this executor. It is left for the end-of-phase verification.

## Task Commits

1. **Task 1: one share switch, portty share / unshare** - `0fbcc2ea` (feat)
2. **Task 2: no boot server, LAN path and old routes removed, GUI bindings, consistency, vmpreview** - `a781a66b` (feat)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Share server factory seam signature**
- **Found during:** Task 1
- **Issue:** The plan's `newShareServer func(TailscaleHealth) (*WebServer, error)` cannot build a server: `webserver.NewWebServer` needs the session HubManager.
- **Fix:** The seam is `func(TailscaleHealth, *relay.HubManager) (*WebServer, error)`; the daemon passes `engine.Manager()`.
- **Files modified:** internal/daemon/share.go, share_vmpreview.go, tests
- **Commit:** 0fbcc2ea

**2. [Rule 1 - Bug] ErrPortInUse only for a taken port**
- **Found during:** Task 1
- **Issue:** Reporting every listen failure as "port in use" would mislabel other bind errors (for example a bind IP that is not up).
- **Fix:** `errors.Is(err, syscall.EADDRINUSE)` on a non-zero port gives ErrPortInUse; anything else stays a listen error (start_failed). The random-port fallback is gone either way.
- **Files modified:** internal/webserver/server.go
- **Commit:** 0fbcc2ea

**3. [Rule 2 - Missing coverage] Share-off disconnect test ported, not dropped**
- **Found during:** Task 2
- **Issue:** Deleting api_disconnect_test.go and the per-session serve tests would lose the M-49 check (web viewers closed on stop, the owner's local subscriber kept). share_test.go did not cover it.
- **Fix:** Added `TestShare_OffDisconnectsWebViewersOnly`; `TestShare_OnReturnsWorkingLink` now also asserts the claims are exactly read,write (the deleted `TestIssueCapabilities_ExactPerms` check).
- **Commit:** a781a66b

**4. [Rule 3 - Blocking] dispatch_test.go and other test fallout**
- dispatch_test.go's usage list expected serve, unserve and web; it now expects share and unshare (Task 1).
- `Config.Mode` was set in internal/webserver/capability_test_helpers.go and four webserver test files besides the fixture; the field was dropped there too, and the now-unused `insecureClient` LAN test helper was removed (Task 2).
- `TestClientWebServerStatus` was replaced by `TestClientGetSessionShare_NotShared`; the unused api_test.go helpers `configureCapabilityStateForTest`, `probeGrant` and `newLoopbackTLSListener` were removed with their only callers. `SetWebServerForTest` had no callers left and was removed.

**5. [Additions] GetSessionShare error code and TESTING.md category**
- GetSessionShare is not turning sharing on or off, so its unreachable-daemon errors use `unavailable` instead of start_failed / stop_failed. The 04-05 UI picks copy by code and treats any code other than https_off with the general sentence, so nothing else changes.
- M-78 went into a new Section 5 Category AQ (Share Switch) because Category AP covers SHAR-05 cleanup.

## Issues Encountered

None. Every gate passed on its first full run: Go build and vet (plain, playwrightfixture, vmpreview), `go test -race -short ./...`, tsc, vitest (93 files, 1204 tests), traceability, the brand gate both ways, and the Task 2 grep gates. After the tests, `pgrep -f '\.test daemon'` printed nothing.

## Known Stubs

None. The old GUI share dialog still calls the removed bindings through the hand-maintained stubs. It stays broken at runtime until 04-05, which was accepted in RESEARCH Open Question 4. Its stubs were left in place on purpose.

## Next Phase Readiness

- 04-03 can expose the record's token to same-account peers and replace `hub.DisconnectWebViewers()` in `unshareLocked` / `remintSharesLocked` with a reason-carrying revoke.
- 04-05 builds the popover on `GetSessionShare` / `SetSessionShared`. `ShareResult.errorCode` values are listed in the App.d.ts comment.

## Self-Check: PASSED

- FOUND: internal/daemon/share.go, internal/daemon/share_test.go, internal/daemon/share_vmpreview.go, cmd_share.go, cmd_share_test.go, tailscale_vmpreview.go
- FOUND: 0fbcc2ea, a781a66b (ancestors of HEAD)
- ABSENT as intended: internal/webserver/auth.go, internal/webserver/localip.go, internal/daemon/api_disconnect_test.go
