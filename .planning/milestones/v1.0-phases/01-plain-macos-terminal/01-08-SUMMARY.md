---
phase: 01-plain-macos-terminal
plan: 08
subsystem: daemon / webserver / tailnet / CLI list + attach bar
tags: [strip-down, agent-identity-removal, webserver-404, tailnet-peer-compat]
status: complete
requires:
  - phase: 01-07
    provides: Frontend no longer reads SessionInfo.cli / RemoteSession.cliType
provides:
  - "Webserver session resolver signature: func(sessionID string) (name, status, hostname string, ok bool)"
  - GET /api/sessions/{id}/info returns 404 when the resolver reports !ok (empty-cliType sentinel removed)
  - Daemon /sessions, webserver /api/sessions, /api/sessions/meta, /api/sessions/{id}/info and tailnet peer metadata carry no cli / cli_type
  - "`agenthub list` prints HOST, ID, NAME, STATUS; attach banner and status bar have no agent segment"
  - AgentHub peers that still send cli_type decode unchanged (plain encoding/json ignores the extra key)
affects: [01-09, 01-10]
actuals:
  tokens: 13300
  tasks: 2
  commits: 2
plan_head_before: 829e1cadd261fd2df1b1115e67a200a1b566f29a
plan_head_after: 78464480a474ce8880e15549825db822fddd2ebc
tech-stack:
  added: []
  removed: ["daemon/app SessionInfo.CLI", "webserver sessionListItem/sessionMetaItem CLIType", "tailnet PeerSession/ShareableSessionMeta CLIType", "CLIRemoteSession.CLIType", "app RemoteSession.CLIType", "statusbar Options.AgentType", "printAttachBanner cli parameter", "fixture sessionAgent"]
  patterns: ["Resolvers return an explicit found flag instead of encoding 'not found' in a sentinel field value"]
key-files:
  created: []
  modified:
    - internal/webserver/server.go
    - internal/daemon/api.go
    - internal/daemon/types.go
    - internal/daemon/engine.go
    - internal/webserver/capability_test_helpers.go
    - cmd/playwright-fixture/main.go
    - internal/tailnet/sessions.go
    - internal/statusbar/bar.go
    - app.go
    - cmd_cli.go
    - cmd_attach.go
    - cmd_remote.go
    - internal/webserver/server_test.go
    - internal/tailnet/tailnet_test.go
    - TESTING.md
decisions:
  - "Resolvers return (\"\", \"\", \"\", false) for an unknown id; list/meta handlers keep their existing name==\"\" -> id fallback, so their output for unknown ids is unchanged"
  - "pty.Session.CLI / CreateRequest.CLI (the launched binary path, internal to the daemon) stay: they are the spawn path, not a wire-level agent identity"
  - "No new test files: the 404 regression and legacy-peer decode tests live in existing server_test.go and tailnet_test.go, so the Go test file count stays 120"
requirements-completed: [STRP-01, STRP-04]
coverage:
  - id: D1
    description: "Unknown session ids 404 on /api/sessions/{id}/info via the resolver found flag; known ids 200; no cli_type in info/list/meta JSON"
    requirement: STRP-01
    verification:
      - kind: integration
        ref: "internal/webserver/server_test.go#TestSessionInfo_UnknownID404KnownID200"
        status: pass
    human_judgment: false
  - id: D2
    description: "Daemon /sessions JSON has no cli key"
    requirement: STRP-01
    verification:
      - kind: integration
        ref: "internal/daemon/api_test.go#TestAPIListSessionsHostname"
        status: pass
    human_judgment: false
  - id: D3
    description: "AgentHub peers sending cli_type and status waiting still decode"
    requirement: STRP-04
    verification:
      - kind: unit
        ref: "internal/tailnet/tailnet_test.go#TestFetchPeerSessionsMeta_LegacyAgentHubPeer"
        status: pass
      - kind: unit
        ref: "cmd_remote_test.go#TestFetchPeerSessions_Success"
        status: pass
    human_judgment: false
  - id: D4
    description: "agenthub list has no AGENT column; attach banner and status bar have no agent segment"
    requirement: STRP-01
    verification:
      - kind: unit
        ref: "cmd_cli_test.go#TestCmdList_Empty; cmd_attach_test.go#TestPrintAttachBanner; internal/statusbar/bar_test.go#TestBar_FormatContainsRequiredFields"
        status: pass
      - kind: manual_procedural
        ref: "Run `agenthub list` and `agenthub attach` against the built app; open the remote section with an AgentHub peer on the tailnet"
        status: unknown
    human_judgment: true
    rationale: "The live attach status bar and peer discovery need a real terminal, daemon and tailnet peer (plan human-check, deferred to end-of-phase)"
metrics:
  duration: 8m
  completed: 2026-11-14
---

# Phase 1 Plan 08: Go-side agent identity removal Summary

**The daemon, webserver, tailnet peer metadata, CLI list and attach bar no longer carry cli/cli_type, and the webserver resolver now returns an explicit `ok` flag, so unknown sessions still 404 on `/api/sessions/{id}/info`.**

## Performance

- **Duration:** 8 min
- **Tasks:** 2/2
- **Files modified:** 25 (12 source, 12 test, TESTING.md)

## Accomplishments

- New resolver signature: `func(sessionID string) (name, status, hostname string, ok bool)`. The field, `SetSessionResolver`, all three handlers, both daemon `api.go` closures, `capability_test_helpers.go`, the playwright fixture and all 23 test resolvers moved to it.
- `handleSessionInfo` returns 404 on `!found`. This replaces the `name == id && cliType == ""` sentinel, avoiding RESEARCH Pitfall 7 (where every unknown id would have returned 200).
- `SessionInfo.CLI` removed from the daemon and app. `CLIType` removed from webserver list/meta items, tailnet `PeerSession`/`ShareableSessionMeta`/decode struct, `CLIRemoteSession` and app `RemoteSession`.
- `agenthub list` header is `HOST\tID\tNAME\tSTATUS`. `printAttachBanner(w, name, hostname)` and `statusbar.Options` have no agent field.
- Non-test Go grep for `CLIType|cli_type|AgentType` is 0 (it was 23).

## Task Commits

1. **Task 1 (tracer): daemon/webserver/list/attach bar drop agent identity; 404 via found flag.** `f4477a88`
2. **Task 2: peer metadata drops cli_type; legacy peers decode; TESTING.md.** `78464480`

## Verification

- `go build ./... && go vet ./... && go test -race -short ./...`: all packages ok
- `TestSessionInfo_UnknownID404KnownID200`: PASS
- Linux cross-compile of the playwright fixture and `./internal/...`: ok
- `pnpm -C frontend exec tsc --noEmit`: ok. vitest: 125 files / 2011 tests passed
- Playwright chromium: 14 passed, 9 skipped. The 9 skips are the same environment-conditional ones seen since 01-02.
- `bash tests/check-traceability-paths.sh`: OK (120 paths). `bash tests/build-script.test.sh`: 37 passed
- `wails build -platform darwin/arm64 -tags wailsassets`: built. The generated `models.ts` already had no cli/cliType (01-07), so the only diff was a blank line and runtime churn. `frontend/src/wailsjs/wailsjs/` was restored afterwards.
- Tracer gate (interactive, end-of-phase, automated-only verify): re-ran the verify chain before Task 2. It passed.

## Deviations from Plan

None. The plan was executed as written. Small test additions inside the planned files: a one-separator assertion in `TestPrintAttachBanner` and an AGENT-absent assertion in `TestCmdList_Empty`. The "no cli key" assertion went into the existing `TestAPIListSessionsHostname`, not a new test function.

## Known Stubs

None.

## Deferred / Human Check

- Plan human-check, deferred to end-of-phase verification: run `agenthub list` / `agenthub attach` against the built app, and, if an AgentHub peer is on the tailnet, confirm its sessions still list.
- Out of scope, untouched: `gofmt -l` flags `internal/webserver/inject_test.go` and `internal/daemon/client_test.go`. Both were already unformatted before this plan.

## Self-Check: PASSED

- internal/webserver/server.go has `ok bool` in the resolver: FOUND
- internal/tailnet/sessions.go has `ShareableSessionMeta`: FOUND
- Commits f4477a88 and 78464480 are ancestors of HEAD: FOUND
