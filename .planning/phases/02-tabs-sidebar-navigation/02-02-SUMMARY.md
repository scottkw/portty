---
phase: 02-tabs-sidebar-navigation
plan: 02
subsystem: relay
tags: [go, websocket, relay, webserver, daemon, chat-removal, fuzz]
status: complete

requires:
  - phase: 01-strip-down
    provides: 0x35/0x36 inject-frame retirement pattern (01-09), files_removed_test.go removed-route harness (01-03), STRP-04 fuzz harness
  - phase: 02-tabs-sidebar-navigation
    provides: 02-01 window chrome (no Go overlap)
provides:
  - Both WebSocket read pumps handle only MsgInput (RO-gated, absorber-filtered), MsgResize2 and MsgPing; 0x30-0x37 have no case and no default arm
  - No server frame in 0x30-0x37 (no presence, self or chat) on either path; webserver connect is a single hub.Subscribe with no WhoIs
  - No chat store, alias store, chat routes, chat providers, presence roster, chat sanitizer or chat-only CORS helpers in daemon/relay/webserver
  - TestRetiredFrames_* (relay + web) and TestChatRemoved_* (daemon relay loopback + webserver) regression tests
  - Playwright fixture without chat seeding; chat-parity.spec.ts gone
affects: [02-05 Hub tree deletion, 02-07 frontend chat UI removal and its manual daemon-restart item]

actuals:
  tokens: 84500
  tasks: 3
  commits: 3
plan_head_before: 8819ce49394a92dc2abeedd1fa71851082449360
plan_head_after: a303ed1eb99caf88d29028eae1d82851cecc909b

tech-stack:
  added: []
  patterns:
    - "Retired wire bytes: no case and no default arm in the read pump; tests send raw byte literals so constants can be deleted"
    - "WebSocket test reads run in one goroutine with a background context (coder/websocket closes the conn when a Read context expires)"
    - "Removed-route tests use a live session so a surviving route would answer 200, not an incidental 404"

key-files:
  created:
    - internal/daemon/chat_removed_test.go
    - internal/webserver/chat_removed_test.go
  modified:
    - internal/relay/server.go
    - internal/relay/hub.go
    - internal/relay/protocol.go
    - internal/relay/server_inject_test.go
    - internal/relay/fuzz_test.go
    - internal/relay/server_test.go
    - internal/webserver/server.go
    - internal/webserver/inject_test.go
    - internal/webserver/server_test.go
    - internal/webserver/session_ended_test.go
    - internal/webserver/fuzz_test.go
    - internal/daemon/engine.go
    - internal/daemon/api.go
    - internal/daemon/engine_test.go
    - cmd/playwright-fixture/main.go
    - TESTING.md

key-decisions:
  - "hub_subscribe_race_test.go deleted: all three tests (TestBroadcastDeliversBeforeIdentity, TestRegisterPresenceRefCounts, TestUnsubscribeEmptyPersonKeyIsClean) covered the WhoIs two-phase presence registration, which no longer exists"
  - "Playwright fixture's /__test__/hub-status admin endpoint deleted whole: its only consumer was chat-parity.spec.ts"
  - "Hub.Unsubscribe now returns nothing; Subscriber keeps only Origin of the identity fields (resize arbiter and RemoteViewerCount read it)"
  - "Webserver fuzz seed api/chat/s1/history kept with a comment: the removed route must 404, not 500"

patterns-established:
  - "Wire-frame retirement: one RETIRED comment in protocol.go and above each pump switch; test with raw bytes, doubled, RO and RW, plus a no-server-frames check"

requirements-completed: [CHAT-01]

duration: ~30min
completed: 2026-11-15
---

# Phase 2 Plan 02: CHAT-01 Go Removal Summary

**Chat, presence, typing, alias and self frames (0x30-0x37) are retired on both WebSocket read pumps, and the daemon chat/alias stores, the webserver chat routes and WhoIs identity, the relay presence hub, the sanitizer and the chat-only CORS helpers are deleted. Old clients' frames are dropped and their sockets keep working.**

## Performance

- **Duration:** about 30 min
- **Completed:** 2026-11-15
- **Tasks:** 3
- **Files:** 39 changed (+435 / -6978)

## Accomplishments

- Both read pumps now handle only MsgInput, MsgResize2 and MsgPing. The MsgInput arm and its `sub.ReadOnly` gate are unchanged byte for byte. There is no default arm, and `loopbackOriginPatterns` and the webserver capability and origin checks are untouched.
- The webserver connect path is a single `hub.Subscribe` followed by `NotifyViewerCount`. The `lc.WhoIs` lookup, presence registration and self frame are gone. `Origin: "web"` and `sub.Origin = "local"` stay.
- The daemon serves no `/api/chat/*` on the relay loopback, and the webserver serves none even with a valid read,write cap. `chats/*.jsonl` and `aliases.json` stay on disk untouched: nothing reads or deletes them now (KillSession no longer deletes the JSONL either).
- `protocol.go` now has 0x01-0x12, MsgMeta 0x20 and one RETIRED comment for 0x30-0x37.

## Task Commits

1. **Task 1: retire 0x30-0x37 on both WebSocket paths (tracer)**: `dbd7fcf4` (feat). Tracer gate: `<verify>` re-run end to end and passed (5+ `--- PASS: TestRetiredFrames_` lines).
2. **Task 2: remove daemon/webserver chat routes, chat store, alias store; trim fixture**: `1dca8ecc` (feat)
3. **Task 3: drop chat/presence/identity from relay protocol and hub; TESTING.md**: `a303ed1e` (refactor)

## Deleted Files

Daemon: `internal/daemon/chat.go`, `chat_routes.go`, `alias_store.go`, `chat_test.go`, `chat_routes_test.go`, `alias_store_test.go`.
Webserver: `internal/webserver/chat.go`, `chat_test.go`, `server_chatsend_test.go`, `identity_test.go`.
Relay: `internal/relay/sanitize.go`, `sanitize_test.go`, `hub_chatsend_test.go`, `hub_presence_test.go`, `protocol_chat_test.go`, `protocol_presence_test.go`, `server_chatsend_test.go`, `server_identity_test.go`, `server_cors_test.go`, `hub_subscribe_race_test.go`.
Frontend: `frontend/e2e/chat-parity.spec.ts`.

**hub_subscribe_race_test.go (flagged assumption):** I read it first. All three tests cover presence or identity: delivery before WhoIs identity resolves, `RegisterPresence` ref-counting and `Unsubscribe` with an empty PersonKey. None covers a non-chat subscribe race, so the file was deleted as RESEARCH predicted.

## Tests Added

- `internal/relay/server_inject_test.go`:
  - `TestRetiredFrames_RW_RelayPath`: every byte 0x30-0x37 sent twice, then one MsgInput, gives exactly 1 PTY write.
  - `TestRetiredFrames_RO_RelayPath`: a read-only client gives 0 writes, and its socket is still subscribed (it receives the viewerCount=2 MsgMeta when a second viewer joins).
  - `TestRetiredFrames_NoServerFrames_RelayPath`: the server never sends a frame in 0x30-0x37.
  - `TestRetiredFrames_Concurrent_RelayPath`: this covers the backstop concurrency truth. Two connections send retired frames at once with 0 writes, then one MsgInput from each gives 2 writes.
- `internal/webserver/inject_test.go`:
  - `TestRetiredFrames_WebPath`: a table over the `read`, `read,files.read` and `read,write` caps gives 0/0/1 writes.
  - `TestRetiredFrames_NoServerFrames_WebPath`: the webserver never sends a frame in 0x30-0x37.
- `internal/daemon/chat_removed_test.go` `TestChatRemoved_RelayLoopbackMux`: GET and OPTIONS on history and export return 404/405 for a **live** session, and `GET /sessions` still returns 200.
- `internal/webserver/chat_removed_test.go` `TestChatRemoved_WebServerMux`: GET and OPTIONS return 404/405 with a valid read,write cap. At HEAD the GET returned 503, so this test fails on the old code.

## Verification Results

- `go build ./... && go vet ./...`: clean
- `go test -race -short ./...`: all packages ok
- `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags playwrightfixture ./cmd/playwright-fixture` and `go build ./internal/...` (linux): ok
- `cd frontend && pnpm exec tsc --noEmit && pnpm exec vitest run`: 117 files, 1832 tests passed
- `pnpm exec playwright test --project=chromium` (headless): 5 passed, 9 skipped (all the skips are already in the specs: `test.skip` / `describe.skip` in progress, web-links-live-toggle and web-plugin-hot-swap)
- `bash tests/check-traceability-paths.sh`: OK, 108 paths
- Symbol greps for Task 2 and Task 3 (`ChatStore`, `AliasStore`, `/api/chat/`, `MsgChat`, `PersonKey`, `FilesCORS`, etc., non-test Go): no match
- Fuzz, 10 s each: `FuzzParseFrame` PASS, `FuzzInputAbsorberFilter` PASS, `FuzzWebServerRoutes` PASS, `FuzzRequireCapability` PASS. No new crasher files under testdata/fuzz.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Chat-send and identity test files deleted in Task 1 instead of Tasks 2/3**
- **Found during:** Task 1
- **Issue:** Once the read pumps lost their chat/alias/typing arms and the self frame, these files no longer compiled or failed: `internal/relay/server_chatsend_test.go`, `internal/relay/server_identity_test.go`, `internal/webserver/server_chatsend_test.go` and `internal/webserver/identity_test.go`. They asserted the behavior Task 1 retires, and `server_chatsend_test.go` used a helper that Task 1 removed from `server_inject_test.go`. Task 1's verify runs the package tests.
- **Fix:** `git rm` them in the Task 1 commit. Every test in them covered chat send, alias, typing, presence or self frames. I moved the counting-writer webserver setup they shared into `inject_test.go` as `setupInjectWebServer`, without the chat-append wiring.
- **Commit:** dbd7fcf4

**2. [Rule 1 - Stale] Stale WhoIs comment in webserver startTailscale**
- **Found during:** Task 1 (the acceptance grep required 0 `WhoIs` hits)
- **Fix:** I dropped the sentence describing the removed handleWSSRelay `lc` local.
- **Commit:** dbd7fcf4

**3. Task 2's final symbol grep only went clean after Task 3**
- The leftover hits were relay-side comments and fields: the `AliasStore` mentions in `SetIdentityProviders` and the hub's `chatAppendFn` comment. Both were Task 3 scope, so I kept the plan's task split. The grep is clean at plan end.

**4. Playwright fixture `/__test__/hub-status` deleted whole** (the plan said to drop only `chatAppendFnWired` and keep the rest if other specs read it). No other spec reads it.

## Known Stubs

None created. As planned, the desktop chat panel is still rendered until 02-07, and it does nothing now (history 404s, sends are dropped).

## Threat Flags

None. This plan only removes surface: the WhoIs-on-connect lookup, the alias persistence and the cap-gated chat routes. The capability check, the WS origin allowlists and the RO input gate are unchanged.

## Next Phase Readiness

- 02-05 and 02-07 can delete the frontend Hub and chat UI. The server side no longer answers chat at all.
- A daemon from the old build keeps serving chat until it restarts (RESEARCH Pitfall 12). That is covered by 02-07's manual item.

## Self-Check: PASSED

- Created files exist: internal/daemon/chat_removed_test.go, internal/webserver/chat_removed_test.go
- Commits on HEAD: dbd7fcf4, 1dca8ecc, a303ed1e
