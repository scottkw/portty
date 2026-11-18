---
phase: 01-plain-macos-terminal
plan: 03
subsystem: daemon / relay / webserver / capability
tags: [strip-down, files-api, capability, security, settings-compat, testing]
status: complete
requires: [01-02]
provides:
  - No files API on any HTTP surface (daemon socket, relay loopback, webserver)
  - Capabilities minted as exactly "read" / "read,write"
  - No browse toggle, home-dir flag or filesWrite setting anywhere in Go
  - Legacy AgentHub settings.json load compatibility test + fixture
  - Negative route tests on all three muxes
affects: [01-04, 01-05, 01-09]
tech-stack:
  added: []
  removed: ["internal/files package", "wailsapp/mimetype as a direct requirement (still // indirect via wails/v2)"]
  patterns: [negative route tests for removed surface, whole-token perm check kept for legacy tokens]
key-files:
  created:
    - internal/daemon/files_removed_test.go
    - internal/webserver/files_removed_test.go
    - tests/fixtures/settings_agenthub_v4.2.json
  modified:
    - internal/daemon/api.go
    - internal/daemon/remote_files.go
    - internal/daemon/relay_remote_files.go
    - internal/daemon/client.go
    - internal/daemon/types.go
    - internal/daemon/engine.go
    - internal/daemon/chat_routes.go
    - internal/daemon/remote_caps.go
    - internal/daemon/plugin_settings.go
    - internal/relay/server.go
    - internal/webserver/server.go
    - internal/webserver/capability_mw.go
    - internal/webserver/chat.go
    - internal/capability/capability.go
    - cmd/playwright-fixture/main.go
    - app.go
    - go.mod
    - TESTING.md
key-decisions:
  - "Kept the CORS helper names (FilesCORS/FilesPreflight/isAllowedFilesOrigin) and their advertised verb/header set unchanged; only comments now say they guard chat history/export. Renaming was optional and would only add churn"
  - "Kept redactCapTokenFromError although nothing calls it now: remote_ws_proxy.go names it as the redaction path if logging is added (Chesterton's Fence)"
  - "Kept isRWGated/SetRWGate per the plan. After deleting originAllowedForWrite, isRWGated has no production reader (tests only). SetRWGate is still called by the funnel-write handlers. Flagged for a later cleanup decision"
  - "Moved the four TestRegisterRemoteCap_* tests into open_remote_session_url_test.go instead of deleting them with remote_files_test.go: they cover the kept cap-deposit route"
  - "Kept TestIssueCapabilities_*RebindsPublicCode, renamed to ClearGrantsRebindsPublicCode: the rebind logic it guards is still in issueCapabilitiesForSession; only the browse-toggle trigger went away"
  - "wailsapp/mimetype stays in go.mod as // indirect (wails/v2 pkg/assetserver imports it); go mod tidy re-adds it if removed"
requirements-completed: [STRP-02, STRP-04]
duration: 21 min
completed: 2026-11-14
actuals:
  tokens: 126000
  tasks: 3
  commits: 3
plan_head_before: 2f99b28176e2517fa876ed2bd2e24247cdd912b2
plan_head_after: c1a5fe3b2f2f100371b35d55d68aef2be4c4f8ef
coverage:
  - deliverable: "No mux serves a files API; remote attach and chat CORS still work"
    human_judgment: false
    verification:
      - {kind: test, ref: "internal/daemon/files_removed_test.go + internal/webserver/files_removed_test.go (404/405 on all three muxes)", status: pass}
      - {kind: test, ref: "TestRemoteSessionWS_*, TestRemoteSessionOpenURL_*, TestRegisterRemoteCap_*, TestChatRoutes_*, TestServer_FilesCORS_ChatRouteShape", status: pass}
      - {kind: command, ref: "GOOS=linux go build -tags playwrightfixture ./cmd/playwright-fixture; go list ./... has no internal/files", status: pass}
  - deliverable: "Caps mint only read / read,write; browse, home-dir and filesWrite state gone; legacy settings load"
    human_judgment: false
    verification:
      - {kind: test, ref: "TestIssueCapabilities_ExactPerms, TestBrowseRoute_Removed, TestSettingsMigration_LegacyAgentHubV42", status: pass}
      - {kind: test, ref: "TestInjectRO_WebPath/legacy_files_read (read,files.read token stays read-only)", status: pass}
  - deliverable: "Dependencies, traceability and e2e match the daemon without a files API"
    human_judgment: false
    verification:
      - {kind: command, ref: "bash tests/check-traceability-paths.sh (OK: 116)", status: pass}
      - {kind: command, ref: "pnpm -C frontend exec playwright test --project=chromium (14 passed, 9 skipped)", status: pass}
---

# Phase 1 Plan 03: Files API and Files Capability Removal Summary

Deleted `internal/files` and every `/api/files/*` route on the daemon socket, relay loopback and webserver muxes. Also removed the files perms, the browse toggle, the home-dir flag and the `filesWrite` setting. Kept the "files"-named code that GUI remote-session attach and chat CORS depend on. New caps are exactly `read` / `read,write`, and a legacy `read,files.read` token is still read-only.

## Performance

- Start 2026-11-14T21:33:25Z, end 2026-11-14T21:54Z (about 21 min).
- Tasks: 3 of 3. 54 files changed (+654 / -9,620).

## Accomplishments

- **Task 1 (tracer):** removed `internal/files` (including its fuzz targets) and 5 removed-feature test files. Also removed the local and remote files routes from api.go, the `filesHandler` argument to `relay.NewServer`, the webserver `filesDispatch` routes, `requireFilesRead`, `requireFilesWrite`, `originAllowedForWrite` and the two perm constants. In the playwright fixture: no sandbox, owner cap `read,write`, no `WRITE_CAP`/`SESSION_CWD`, and the remote peer serves only `/join/exchange`. Added negative tests on all three muxes. The relay CORS test now exercises the chat-route shape. The tracer gate re-ran the full verify on the committed tree and it passed.
- **Task 2:** minting always gives `read` / `read,write`. Removed: the `POST /sessions/{id}/browse` route and its handler, the engine `sessionBrowse` map and browse/home-dir helpers, the `FilesWrite` setting with its load and save lines, `SessionInfo.HomeDir`/`BrowseEnabled`, `IssueCapabilitiesResponse.HomeDir`, `App.SetSessionBrowse` and `App.DownloadFile`. Added an AgentHub v4.2 settings fixture with a load test. Funnel tests no longer take a browse parameter.
- **Task 3:** ran `go mod tidy`, reconciled TESTING.md (Go 124, total 259, STRP-02 rows), and re-ran Playwright chromium.

## "files"-named symbols deliberately kept (per plan `<output>`)

| Symbol | Where | Why kept |
|--------|-------|----------|
| `RemoteCapStore` | internal/daemon/remote_caps.go | Holds the remote peer cap for GUI remote-session open and the remote WS proxy (F1) |
| `POST /api/remote-files/caps`, `handleRegisterRemoteCap` | api.go, remote_files.go | Cap deposit after join-code exchange (`App.RegisterRemoteCap`) (F1) |
| `GET /api/remote-files/caps/{sessionID}/open-url`, `handleRemoteSessionOpenURL` | api.go, remote_files.go | Held-cap reuse when opening a remote session (`App.OpenRemoteSessionURL`) (F1) |
| `newRemoteFilesHTTPClient`, `remoteFilesClient`, `remoteFilesClientForTest`, `SetRemoteFilesClientForTest` | remote_files.go, api.go | Outbound TLS client used by `handleRemoteSessionWS`; the test hook is used by remote_ws_proxy_test.go |
| `redactCapTokenFromError` | remote_files.go | No caller now. remote_ws_proxy.go points to it as the redaction path for future logging |
| `client_remote_files.go` (`RegisterRemoteCap`, `ExchangeJoinCodeAtURL`, `OpenRemoteSessionURL`) | internal/daemon | Client side of remote attach (F1) |
| `wrapRelayWithRemoteFiles` | relay_remote_files.go | Parent relay mux. Mounts `GET /api/relay/remote/{sessionID}/ws` and the `/` fallthrough (F2) |
| `FilesCORS`, `FilesPreflight`, `withCORS`, `isAllowedFilesOrigin`, `handleFilesPreflight` | internal/relay/server.go | CORS for chat history/export, which chat_routes.go mounts on the relay (F3) |
| `sessionWorkDirs`, `GetSessionWorkDir`, `SessionInfo.WorkDir` | engine.go, types.go | The Hub groups cards by work dir (F5) |
| `readonly := !capability.HasPerm(claims.Perms, "write")`, `HasPerm` | webserver/server.go, capability.go | Whole-token check keeps legacy `read,files.read` tokens read-only (F4, T-01-11) |
| `isRWGated` / `SetRWGate`, `requireCapability` | webserver | Kept per plan (see decisions: `isRWGated` now has no production reader) |
| `App.OpenFileDialog`, `App.SaveTerminalSession` | app.go | Settings path picker and terminal export, not file browsing |

## Task Commits

| Task | Commit | Message |
|------|--------|---------|
| 1 | 21c4db43 | feat(01-03): remove files API from daemon, relay and webserver muxes |
| 2 | d1d67892 | feat(01-03): mint only read / read,write; drop browse, home-dir and filesWrite state |
| 3 | c1a5fe3b | chore(01-03): tidy go.mod and reconcile TESTING.md for files API removal |

Deletions in commit 21c4db43 are all intentional: `internal/files/` (10 files) plus the 5 test files in the plan's `files_deleted` list.

## Plan-level Verification

- `go build ./... && go vet ./... && go test -race -short ./...`: all packages pass.
- Linux cross-build of `cmd/playwright-fixture` (playwrightfixture tag) and `./internal/...` passes.
- `tsc --noEmit` passes. Vitest: 127/127 files, 2076 tests on the second run (see Issues).
- `check-traceability-paths.sh` printed OK: 116, and `build-script.test.sh` passed 37/37.
- Playwright chromium: 14 passed, 9 skipped. These are the same 9 environment-conditional skips seen in 01-02.
- `wails build -platform darwin/arm64 -tags wailsassets` succeeded. Afterwards `git checkout -- frontend/src/wailsjs/wailsjs/` was run, and the tree was clean.
- Acceptance greps: no files routes in non-test sources; `PermFiles|requireFiles|SetFilesHandler|filesHandler|originAllowedForWrite` returns 0; no browse/HomeDir/filesWrite in non-test Go; `DownloadFile` count 0 and `OpenFileDialog` count 1; `internal/files/` appears 0 times in TESTING.md and `files_removed_test.go` appears 3 times.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Kept coverage] Preserved the tests for the kept cap-deposit route**
- Found during: Task 1. `remote_files_test.go` (which the plan lists for deletion) also held `TestRegisterRemoteCap_*` for the kept `POST /api/remote-files/caps`. `open_remote_session_url_test.go` relied on its helpers `mockRemoteWebserver`/`proxyTestAPI`.
- Fix: moved the 4 RegisterRemoteCap tests into `open_remote_session_url_test.go` with a small `capTestAPI` helper, which needs no files upstream. The relay fallthrough check from the deleted `relay_remote_files_test.go` is now part of `TestFilesRemoved_RelayLoopbackMux` (`GET /sessions` returns 200). Commit 21c4db43.

**2. [Rule 3 - Blocking] Did the perm-minting change and the PermFiles test edits in Task 1**
- Deleting the perm constants (Task 1) broke the build in the browse branch of `issueCapabilitiesForSession` and in the browse-matrix and funnel tests. So the change to always mint `read` / `read,write` and the `TestIssueCapabilities_ExactPerms` replacement landed in commit 21c4db43. The rest of the browse state (engine, route, types, app) landed in Task 2 as planned.

**3. [Rule 3 - Blocking] Updated call sites outside the task file lists**
- `cmd_attach_test.go`, `internal/relay/server_{test,identity_test,inject_test}.go` (`NewServer` arity), `internal/capability/capability_test.go` (dropped `TestPermFilesRead_Constant`), `internal/webserver/{funnel_test,rwgate_test}.go` (dropped the two `originAllowedForWrite` tests), `internal/webserver/{inject_test,server_chatsend_test}.go` (renamed the `browse_on` case to `legacy_files_read`), `internal/daemon/open_remote_session_url_test.go`, and stale comments in `chat_routes.go`, `remote_caps.go`, `plugin_settings.go`, `webserver/chat.go`.

**4. [Rule 1 - Plan assumption] `wailsapp/mimetype` cannot leave go.mod**
- Found during: Task 3. `go mod tidy` moved it from direct to `// indirect` and did not remove it. `go mod why` shows that `wails/v2/pkg/assetserver` imports it, so module graph pruning requires the entry. Taking it out by hand would leave go.mod untidy. So the plan's check `grep -c 'wailsapp/mimetype' go.mod == 0` cannot pass, and the real outcome is that it is no longer a direct requirement. The kept deps `kardianos/service` and `chromedp` are still present.

**5. [Rule 1 - Dead code] Deleted `TestRequireCapability_UnchangedByPhase118`**
- It only checked that `files.read` stayed out of `requireCapability` and lived in the separate `requireFilesRead` wrapper. Both the perm and the wrapper are gone.

**Total deviations:** 5 (1 coverage preservation, 2 blocking, 2 plan-assumption/dead-code). **Impact:** none on kept behavior.

## Issues Encountered

- One full vitest run reported 1 failing test out of 2076. A re-run piped through grep showed no FAIL line, and the next full run passed 127/127. This plan touched no frontend files, so the failure is the load-sensitive flake noted in 01-02, probably SM-02. Noted, not chased.
- Gofmt drift already exists in `tray.go`, `cmd/playwright-fixture/main.go` (hub-status map), `internal/daemon/{client_test,engine_test}.go` and two relay test files. It was out of scope and left alone.

## Known Stubs

None.

## Threat Flags

None. This plan only removes surface. T-01-10 and T-01-14 are covered by the negative route tests and the `go list` gate. T-01-11 is covered by `TestInjectRO_WebPath/legacy_files_read` and `TestChatSend_ROCanPost_WebPath/legacy_files_read`. T-01-12 is covered by `TestServer_FilesCORS_ChatRouteShape` and the passing `security_regression_test.go`. T-01-13 is covered by `TestRemoteSessionWS_*` and `TestRemoteSessionOpenURL_*`.

## Next Phase Readiness

Ready for 01-04. STRP-02 is now complete on both halves (UI in 01-02, daemon in 01-03). Open follow-ups:
- `isRWGated` is still written by the funnel-write handlers, but only tests read it.
- `FixtureEnv.remotePeerURL` in the e2e setup is still parsed. The fixture still prints `REMOTE_PEER_URL` because the plan said to keep it, even though no spec uses it now.
- 01-09 adds the relay/webserver fuzz targets.

## Self-Check: PASSED

- Created files exist: `internal/daemon/files_removed_test.go`, `internal/webserver/files_removed_test.go`, `tests/fixtures/settings_agenthub_v4.2.json`.
- Commits 21c4db43, d1d67892 and c1a5fe3b are ancestors of HEAD, and the rev-list count from plan_head_before is 3. `git ls-files internal/files` is empty.
