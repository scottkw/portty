---
phase: 01-plain-macos-terminal
plan: 09
subsystem: relay / webserver / daemon chat / frontend chat / security fuzz harness
tags: [strip-down, chat-inject-removal, wire-protocol, fuzzing, security]
status: complete
requires:
  - phase: 01-08
    provides: Go side no longer carries session agent identity
provides:
  - "0x35 (inject) and 0x36 (inject NAK) are RETIRED wire bytes; neither read pump (relay loopback, webserver) has a case for them, so an old client's 0x35 is dropped without touching the PTY"
  - "No HandleInject, InjectPayload, InjectErrorPayload, MakeInjectErrorFrame, InjectErrorReason, SanitizePTYText or ChatMessage.SessionInject; hub.SetChatAppendFn wiring kept (chat send needs it)"
  - "Chat on desktop and web: every send goes through sendChat; no @session target, inject button, inject indicator or inject CSS; mention popover lists only participants and renders nothing when none match"
  - "Legacy chat history with \"sessionInject\":true still loads and exports with no marker"
  - "Security fuzz harness: FuzzParseFrame, FuzzInputAbsorberFilter, FuzzSanitizeChatContent (relay), FuzzRequireCapability, FuzzWebServerRoutes (webserver), plus existing capability.FuzzVerify"
  - "/assets/<invalid UTF-8> returns 404 instead of 500"
affects: [01-10, later Phase 1 plans]
actuals:
  tokens: 45600
  tasks: 3
  commits: 3
plan_head_before: 6ac257be7b3aeac4b88f06b83cd2e6262f32806c
plan_head_after: a5dbcda00d6f4cfd761099e7285a4b1937f93e49
tech-stack:
  added: []
  removed: ["relay HandleInject / InjectErrorReason / ErrReadOnly / ErrInjectNotRecorded / ErrInjectTooLarge / MaxInjectTextBytes", "relay MsgSessionInject / MsgInjectError / InjectPayload / InjectErrorPayload / MakeInjectErrorFrame / ChatMessage.SessionInject", "relay SanitizePTYText", "frontend MSG_SESSION_INJECT / MSG_INJECT_ERROR / encodeSessionInjectFrame / sendSessionInject / onInjectError", "ChatPanel press-and-hold inject gesture, isReadOnly state and its /info fetch", "MentionPopover Agent section / @session row", "ChatMessage inject indicator and its CSS"]
  patterns: ["Retired wire bytes are documented next to the live constants and never reused", "Security tests for a removed frame send the raw byte and prove the read pump drops it (zero PTY bytes, socket open, next frame handled)", "Fuzz targets live next to the code as fuzz_test.go; seed corpora run in the normal suite; crashers that pin a fixed bug are committed under testdata/fuzz"]
key-files:
  created:
    - internal/relay/fuzz_test.go
    - internal/webserver/fuzz_test.go
    - internal/webserver/testdata/fuzz/FuzzWebServerRoutes/fbdee7071c7665ef
  modified:
    - internal/relay/server.go
    - internal/webserver/server.go
    - internal/relay/hub.go
    - internal/relay/protocol.go
    - internal/relay/sanitize.go
    - internal/daemon/chat.go
    - internal/daemon/engine.go
    - cmd/playwright-fixture/main.go
    - internal/relay/server_inject_test.go
    - internal/webserver/inject_test.go
    - internal/daemon/chat_test.go
    - frontend/src/lib/relayClient.ts
    - frontend/src/components/Hub/ChatPanel.tsx
    - frontend/src/components/Hub/MentionPopover.tsx
    - frontend/src/components/Hub/ChatMessage.tsx
    - frontend/src/style.css
    - frontend/src/content/help/chat.md
    - frontend/e2e/chat-parity.spec.ts
    - TESTING.md
key-decisions:
  - "0x35/0x36 marked RETIRED in protocol.go and relayClient.ts; no default arm added to the read pumps, so unknown frames stay silently ignored"
  - "ChatPanel isReadOnly state and its /api/sessions/{id}/info fetch deleted: their only reader was the removed inject gesture (noUnusedLocals would fail tsc otherwise); server-side gating is unchanged"
  - "The composer routes Arrow/Enter to the mention popover only when it is visible (at least one participant matches); otherwise Enter sends chat"
  - "sanitize_test.go kept with a direct TestSanitizeChatContent so the SEC-02 traceability row still points at a real file"
  - "FuzzSanitizeChatContent compares rune counts, not bytes: invalid UTF-8 bytes become U+FFFD (3 bytes), which is safe"
  - "Fuzz webserver gets a bare, never-served loopback listener so BaseURL is set; cspHeaders fails closed with 500 without one"
patterns-established:
  - "Retired protocol bytes: comment beside the remaining constants, raw-byte regression test on every read-pump path"
requirements-completed: [CHAT-01, STRP-04]
coverage:
  - id: D1
    description: "A 0x35 frame from RO or RW clients on the relay path writes zero PTY bytes, the socket stays open, and a following chat send is persisted and broadcast"
    requirement: CHAT-01
    verification:
      - kind: integration
        ref: "internal/relay/server_inject_test.go#TestInjectFrame_RO_RelayPath_NoPTYWrite, TestInjectFrame_RW_RelayPath_NoPTYWrite"
        status: pass
  - id: D2
    description: "Same 0x35 proof over the webserver path for RO read, legacy RO read,files.read and RW caps"
    requirement: CHAT-01
    verification:
      - kind: integration
        ref: "internal/webserver/inject_test.go#TestInjectFrame_WebPath_NoPTYWrite"
        status: pass
  - id: D3
    description: "Legacy history line with sessionInject:true loads and exports with no inject marker"
    requirement: CHAT-01
    verification:
      - kind: unit
        ref: "internal/daemon/chat_test.go#TestChatStore_LegacySessionInjectLine"
        status: pass
  - id: D4
    description: "@session text is sent as plain chat; button always reads Send; popover lists only participants and renders nothing for no match"
    requirement: CHAT-01
    verification:
      - kind: unit
        ref: "frontend/src/components/Hub/ChatPanel.test.tsx, frontend/src/components/Hub/MentionPopover.test.tsx"
        status: pass
      - kind: e2e
        ref: "pnpm -C frontend exec playwright test e2e/chat-parity.spec.ts --project=chromium (8 passed)"
        status: pass
      - kind: manual_procedural
        ref: "TESTING.md M-23 / M-24: desktop + browser chat incl. '@session echo hi' and @alias; nothing typed into terminal"
        status: unknown
  - id: D5
    description: "Security fuzz harness: five new targets plus FuzzVerify each run clean for 10 s"
    requirement: STRP-04
    verification:
      - kind: other
        ref: "go test -run='^$' -fuzz='^<Target>$' -fuzztime=10s for each of the six targets"
        status: pass
metrics:
  duration: 20 min
  completed: 2026-11-14
---

# Phase 1 Plan 09: Chat inject bridge removal and security fuzz harness Summary

The chat-to-PTY inject bridge is gone end to end. 0x35/0x36 are retired wire bytes, and tests on both WebSocket paths prove a raw 0x35 from RO or RW caps writes zero PTY bytes. The desktop and web chat UIs no longer have an @session target or inject UI. Five new Go fuzz targets cover relay frame parsing, the input absorber, chat sanitization and webserver capability/route handling, and one of them found and pinned a real 500.

## What was done

**Task 1 (5d56c21e): server side.** Deleted the `case MsgSessionInject` blocks in `internal/relay/server.go` and `internal/webserver/server.go`, and all the hub inject machinery. Also removed the inject wire types, `ChatMessage.SessionInject`, `SanitizePTYText`, the export marker and the fixture's inject seed. `hub.SetChatAppendFn` and its `engine.go` wiring are kept, with comments rewritten to describe chat-send persistence. The inject tests were retargeted rather than deleted, and a legacy-history test was added.

**Task 2 (21a02122): frontend.** relayClient lost its inject frame code. In ChatPanel every send goes through `sendChat`, and the button always reads "Send". MentionPopover lists only participants, starting at index 0, and renders nothing when none match. ChatMessage lost its inject indicator, and all inject CSS was deleted. The help text and the chat-parity e2e spec were trimmed to match.

**Task 3 (a5dbcda0): fuzz harness.** Added the fuzz targets and fixed the 500 that the route fuzzer found. TESTING.md now covers the harness and the retargeted rows, and the counts are Go 122, total 255.

## Fuzz targets and 10 s results

| Target | Package | Result (10 s) |
|--------|---------|---------------|
| FuzzParseFrame | internal/relay | PASS (~386k execs) |
| FuzzInputAbsorberFilter | internal/relay | PASS (~420k execs) |
| FuzzSanitizeChatContent | internal/relay | PASS (~268k execs) |
| FuzzRequireCapability | internal/webserver | PASS (~230k execs) |
| FuzzWebServerRoutes | internal/webserver | Initial run found a 500 at `/assets/\xad`. Fixed, then PASS (~131k execs) |
| FuzzVerify (existing) | internal/capability | PASS (~423k execs) |

## Verification

- `go build ./... && go vet ./... && go test -race -short ./...`: all packages ok
- `go test -race -run 'Inject|ChatSend|Legacy|Export' -v` (relay, webserver, daemon): PASS, including RO and RW cases on both paths
- Linux cross-compile of the playwright fixture and `./internal/...`: ok
- `pnpm -C frontend exec tsc --noEmit`: ok. `pnpm -C frontend test`: 125 files / 2003 tests pass
- Playwright chromium: chat-parity 8/8; full suite 13 passed, 9 skipped
- `bash tests/check-traceability-paths.sh`: OK (122 paths). `bash tests/build-script.test.sh`: 37/37
- `wails build -platform darwin/arm64 -tags wailsassets`: ok; `frontend/src/wailsjs/wailsjs/` restored
- Acceptance greps: non-test Go inject symbols 0; read-pump inject cases 0; `RETIRED` in protocol.go 1; `SetChatAppendFn` in engine.go 1; frontend inject symbols 0; `@session` in MentionPopover.tsx and chat.md 0; inject CSS classes 0; ROCHAT-02/SC-4 e2e tests 0

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `/assets/<invalid UTF-8>` returned 500**
- **Found during:** Task 3 (FuzzWebServerRoutes, first 10 s run)
- **Issue:** `fs.Sub` rejects non-UTF-8 names with `fs.ErrInvalid`, and `http.FileServerFS` maps any non-NotExist error to 500. Any client could trigger it with `GET /assets/%AD`. There was no information leak.
- **Fix:** `assetsNoStore` (shared by both `/assets/` routes) returns 404 for a non-UTF-8 path. The crasher is committed as a regression seed.
- **Files modified:** internal/webserver/server.go, internal/webserver/testdata/fuzz/FuzzWebServerRoutes/fbdee7071c7665ef
- **Commit:** a5dbcda0

**2. [Rule 3 - Blocking] ChatPanel `isReadOnly` state and `/info` fetch removed**
- **Found during:** Task 2
- **Issue:** The removed inject gesture was the only reader of `isReadOnly`. With `noUnusedLocals`, tsc would fail.
- **Fix:** Deleted the state and the `/api/sessions/{id}/info` fetch that existed only to set it. The server still gates PTY input, and RO guests can still chat and set an alias, as before.
- **Commit:** 21a02122

**3. [Rule 1 - Bug] Composer keyboard math with zero matches**
- **Found during:** Task 2
- **Issue:** Once @session was gone, `totalOptions` could be 0. ArrowUp/ArrowDown would then compute `% 0` (NaN), and Enter would be swallowed by an invisible popover.
- **Fix:** Arrow/Enter go to the popover only when at least one participant matches; otherwise Enter sends chat. Covered by a new ChatPanel test.
- **Commit:** 21a02122

**4. [Plan refinement] FuzzSanitizeChatContent length invariant**
- The plan said "output is never longer than input". That fails at byte level because invalid UTF-8 becomes U+FFFD (3 bytes), which is safe. The target asserts on rune count instead.

**5. [Plan refinement] Fuzz webserver listener**
- `cspHeaders` fails closed with 500 when `BaseURL()` is empty (listener unset). Every CSP-wrapped route fuzzed as a false 500. The fuzz server gets a bare loopback listener that is never served, so `BaseURL` is set. This is not a TLS server.

**6. [Scope] Extra test files touched for the removed API**
- `internal/relay/protocol_chat_test.go`, `internal/relay/server_chatsend_test.go`, `internal/webserver/server_chatsend_test.go` and `frontend/src/components/__tests__/TerminalPanel.scale.test.tsx` referenced removed symbols (SessionInject, MsgInjectError, inject mocks) and were updated.
- `frontend/src/components/Hub/TerminalChatHost.tsx` got a one-word comment fix ("Send/Inject" became "Send").
- `internal/relay/sanitize_test.go`'s only test (`TestSanitizePTYText`) was replaced by `TestSanitizeChatContent`.

### Mistake caught during execution

While deleting `InjectErrorReason` from hub.go, I first cut everything after it, assuming it was the end of the file. `go build` failed immediately. I restored the tail from `git show HEAD:internal/relay/hub.go` and reviewed the diff before committing. The committed hub.go diff contains only the intended removals.

## Known Stubs

None.

## Threat Flags

None. The assets fix narrows behavior (500 → 404) on an existing public route and adds no new surface.

## Notes for later plans

- Per the scope note, nothing here depends on a user-chosen shell, Settings → Paths, multi-state Tailscale health, session status display, exit UI or bell notifications.
- A pre-existing gofmt drift remains in `internal/daemon/client_test.go` and `internal/relay/scrollback_altscreen_test.go`. It is out of scope and left untouched.
- Manual items M-23 / M-24 (desktop and browser chat with `@session echo hi`) are left for end-of-phase UAT with dev-browser.

## Self-Check: PASSED

- All key-files.created exist on disk; commits 5d56c21e, 21a02122 and a5dbcda0 are ancestors of HEAD (checked after writing).
