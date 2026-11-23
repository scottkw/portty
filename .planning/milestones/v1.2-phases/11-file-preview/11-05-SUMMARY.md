---
phase: 11-file-preview
plan: 05
subsystem: frontend
status: complete
tags: [frontend, preview, relay-client, assembler, store]
requires: [11-02]
provides:
  - "MSG_PREVIEW (0x21) parsing in relayClient.ts, pinned to the Go fixture"
  - "RelayClient onPreview / onPreviewReset callbacks"
  - "previewAssembler.ts: PREVIEW_MAX_BYTES, viewer MIME table, cleanPreviewName, createPreviewAssembler"
  - "previewStore.ts: PreviewStore and PreviewEntry (for App, 11-09)"
  - "TerminalPanel onPreview prop; WebShareSessionView onPreview and isActive props"
affects: [11-09, 11-10]
tech-stack:
  added: []
  patterns:
    - "Pure per-connection assembler fed by RelayClient callbacks, events up through an optional prop"
    - "External store with subscribe/getSnapshot for useSyncExternalStore, with injected URL functions and clock"
key-files:
  created:
    - frontend/src/lib/previewAssembler.ts
    - frontend/src/lib/previewStore.ts
    - frontend/src/lib/__tests__/previewAssembler.test.ts
    - frontend/src/lib/__tests__/previewStore.test.ts
    - frontend/src/components/__tests__/TerminalPanel.preview.test.tsx
  modified:
    - frontend/src/lib/relayClient.ts
    - frontend/src/lib/relayClient.test.ts
    - frontend/src/components/TerminalPanel.tsx
    - frontend/src/components/WebShareSessionView.tsx
    - frontend/src/components/__tests__/WebShareSessionView.test.tsx
decisions:
  - "The assembler's socket-drop method is named discard(), not reset(): TerminalPanel.hot-swap.test.tsx forbids any .reset() call in TerminalPanel.tsx"
  - "Any new preview header fails the open preview as incomplete, even if the new header is then rejected (bad MIME or key)"
  - "While a preview refreshes, PreviewEntry.size and .received describe the incoming copy; name, kind, mime, blob and url describe the copy shown until the swap"
  - "Retained bytes are counted from the Blob sizes in the store; a start (new or refresh) updates arrivedAt"
metrics:
  duration: "~60 min"
  completed: 2026-11-21
actuals:
  tokens: 13200
  tasks: 3
  commits: 3
plan_head_before: 8e07f421a44029934ff9a9fd4e846e54ea2f2885
plan_head_after: ba0e9ce718121b8b44ecc2d15f73066be6d7d9bb
---

# Phase 11 Plan 05: Frontend preview data path Summary

Every viewer's socket now turns MSG_PREVIEW frames into a finished, correctly typed Blob, with limits on what it accepts. Finished previews go into a PreviewStore that sits outside any terminal view and caps itself at 10 entries and 100 MiB. The JS parser is tested against the Go golden fixture. App.tsx is not changed (11-09 builds on this).

## As built (11-09 consumes these)

### relayClient.ts

- `MSG_PREVIEW = 0x21`, `PREVIEW_HEADER = 0x01`, `PREVIEW_CHUNK = 0x02`, `PREVIEW_END = 0x03`, `PREVIEW_CHUNK_MAX = 16384`
- `ServerFrame` gains `{ type: 'preview'; preview: PreviewFrame }` with:
  ```ts
  type PreviewFrame =
    | { kind: 'header'; id: number; name: string; mime: string; size: number; key: string; focus: boolean }
    | { kind: 'chunk'; id: number; payload: Uint8Array }   // a view into the frame (subarray)
    | { kind: 'end'; id: number; status: number }
  ```
- The parser applies the same shape checks as `relay.ParsePreview*`. A chunk must be at least 6 bytes with a payload of at most 16384. An end must be exactly 7 bytes. A header must be JSON with a uint32 id, string name, mime and key, a non-negative safe-integer size and a boolean focus. Anything else, including an unknown sub-kind, parses as `{type:'unknown'}` and is dropped.
- New `RelayClientCallbacks`:
  - `onPreview?(frame: PreviewFrame)` gets every preview frame in order. The stream offset never moves for these frames.
  - `onPreviewReset?()` fires on every socket `onclose` (before any reconnect, and also in non-reconnect mode) and when a dead socket is abandoned.

### previewAssembler.ts

- `PREVIEW_MAX_BYTES = 20_000_000`. A test checks it equals the fixture's `maxBytes`.
- `PreviewKind = 'image' | 'svg' | 'pdf' | 'markdown'`
- `PreviewMime`: image/png, image/jpeg, image/gif, image/webp (kind `image`); image/svg+xml (`svg`); application/pdf (`pdf`); text/markdown (`markdown`). The Blob type comes from this table, never from the header string.
- `cleanPreviewName(name)` removes C0 and C1 controls, U+202A–U+202E and U+2066–U+2069. It keeps at most 255 code points and returns `file` if nothing is left.
- `createPreviewAssembler(emit)` returns `{ feed(frame: PreviewFrame), discard() }` and emits:
  ```ts
  type PreviewEvent =
    | { type: 'start'; id; name /* cleaned */; kind; mime: PreviewMime; size; key; focus }
    | { type: 'progress'; id; received }        // at most one per 100 ms, plus the one that reaches size
    | { type: 'complete'; id; blob: Blob }
    | { type: 'failed'; id; reason: 'tooLarge' | 'incomplete' }
  ```
- Rules:
  - A header with an unknown MIME emits nothing and logs `console.warn`. A key that isn't 32 lowercase hex emits nothing.
  - size > cap: `start`, then `failed tooLarge`, and no bytes are accepted.
  - A new header while one is open: the open one fails `incomplete`.
  - Chunks and ends for another id are ignored.
  - Received bytes > cap: `tooLarge`. Received bytes > declared size: `incomplete`.
  - End status 2: `tooLarge`. Status 1 (or any status other than 0), or a byte count that differs from the size: `incomplete`.
  - `discard()` fails the open preview `incomplete`.
  - Nothing is preallocated. The chunk array is dropped once the Blob is built.

### previewStore.ts

- `PREVIEW_MAX_TABS = 10`, `PREVIEW_MAX_RETAINED = 100 * 1024 * 1024`
- ```ts
  interface PreviewEntry {
    tabId: string        // `__preview__${ownerTabId}/${key}`
    ownerTabId: string; key: string; name: string; kind: PreviewKind; mime: PreviewMime
    size: number; state: 'receiving' | 'rendered' | 'tooLarge' | 'failed'; received: number
    blob?: Blob; url?: string; arrivedAt: number; refreshing: boolean
  }
  ```
- `new PreviewStore({ createURL?, revokeURL?, now? })`. The defaults are `URL.createObjectURL`, `URL.revokeObjectURL` and `Date.now`.
- `subscribe(listener) => unsubscribe` and `getSnapshot()` are arrow properties, so they can be passed straight to `useSyncExternalStore`. A new array is created only when something changes.
- `handle(ownerTabId, event)` returns `{ tabId, isNew } | null`. It returns null when the event refers to nothing held (an unknown owner or id, or an entry that was evicted or closed).
- `setActive(tabId | null)`: eviction never picks this tab.
- `close(tabId)` and `closeAll()` revoke the URLs they drop.
- Behaviour:
  - **Refresh in place:** a start for a rendered (owner, key) keeps the old blob and URL and sets `refreshing: true`, with progress on the entry. `complete` swaps in the new blob and URL in one change and revokes the old URL once. `failed` keeps the old content.
  - A start for an entry that never rendered restarts it.
  - A new 11th entry evicts the oldest non-active entry by `arrivedAt`.
  - After each `complete`, the oldest non-active entries are evicted while the retained Blob bytes exceed 100 MiB.
  - Eviction revokes URLs and forgets any preview in flight for that entry.
  - Session end and owner-tab close never touch entries. App decides when to call `close`.

### Components

- `TerminalPanel` gets `onPreview?: (e: PreviewEvent) => void`, read through a ref. It creates one assembler with each RelayClient, wires `onPreview: preview.feed` and `onPreviewReset: preview.discard`, and calls `preview.discard()` on unmount (after `client.close()`).
- `WebShareSessionView` gets `onPreview?` and `isActive?: boolean` (default `true`, replacing the constant `isActive={true}`) and forwards both to TerminalPanel.

## Tests (for 11-10's TESTING.md rows)

- `frontend/src/lib/relayClient.test.ts`, describe "MSG_PREVIEW parsing against the Go fixture (PREV-01)":
  - pins the type byte and the chunk size
  - decodes each fixture case: header focus=true, header focus=false, full chunk, short chunk, end ok, end aborted, end too large
  - drops malformed frames
  - RelayClient delivery: onPreview order and no offset move; onPreviewReset on close in reconnect and plain modes
- `frontend/src/lib/__tests__/previewAssembler.test.ts`:
  - "previewAssembler happy path (PREV-01)": the cap equals the fixture, Blob size and type, the MIME-to-kind map, discard mid-stream
  - "previewAssembler failure rules (PREV-03, T-11-16)": over-cap header, at-cap header, bytes past the cap or the size, end status 2 and 1, short ok end, a second header, foreign ids, unknown MIME, bad keys, progress throttle, cleanPreviewName, the cleaned name in start
- `frontend/src/lib/__tests__/previewStore.test.ts`, "PreviewStore (PREV-01, T-11-17)": rendered entry and id scheme, refresh swap and single revoke, failed refresh keeps content (incomplete and tooLarge), restart of a never-rendered entry, separate owners, ignored events, the 10-entry limit sparing the active tab, the 100 MiB limit, an evicted in-flight entry, close and closeAll revoke, snapshot stability and subscribe
- `frontend/src/components/__tests__/TerminalPanel.preview.test.tsx`, "TerminalPanel onPreview (PREV-01)": local, peer and browser panels get start/progress/complete; a socket reset fails `incomplete`; unmount fails `incomplete`
- `frontend/src/components/__tests__/WebShareSessionView.test.tsx`, "WebShareSessionView — preview pass-through (PREV-01)"

## Gate (Task 3)

- **Full vitest, run once** (`/tmp/p11-05-gate-vitest.log`): 1392 passed, 2 failed, 98 files.
  - `SharePopover: Copy > Copy writes the full link and reads Copied! for 2 s` is a known flake. It passed on its one allowed single-test rerun (`/tmp/p11-05-gate-vitest-sharepopover-rerun.log`).
  - `TerminalPanel.hot-swap.test.tsx > Scrollback preservation: source never calls term.reset()` was caused by this plan. Fixed in ba0e9ce7 (see Deviations). After the fix, that file plus the assembler and TerminalPanel.preview files all passed (43 tests, `/tmp/p11-05-gate-vitest-fix.log`). Per the run-once rule, the full suite was not run again.
- **tsc --noEmit:** passed (`/tmp/p11-05-gate-tsc.log`, rerun after the fix).
- **go build ./... and go vet ./...:** passed.
- **go test -race -short -count=1 ./..., run once** (`/tmp/p11-05-gate-go.log`): 12 packages ok. The only failure was `internal/webserver` `TestShare_DeadViewerReapedBacklogDelivered` (`input_queue_test.go:175`). That failure was already there before this plan, and the debug session owns it. This plan changes no Go file. No DATA RACE, panic or SKIP lines.
- `tests/check-brand.sh` and `tests/check-traceability-paths.sh` passed.
- No changes since 8e07f421 to any `*.go` file, App.tsx, TabBar.tsx, style.css, the fixture or `frontend/src/wailsjs`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] The assembler's reset() tripped the TerminalPanel source guard**
- **Found during:** Task 3 (full vitest)
- **Issue:** `TerminalPanel.hot-swap.test.tsx` asserts that TerminalPanel.tsx contains no `.reset()` call, which protects the buffer from `term.reset()`. `live?.preview.reset()` matched it.
- **Fix:** renamed the assembler method to `discard()`. The guard test is unchanged.
- **Files modified:** previewAssembler.ts, previewAssembler.test.ts, TerminalPanel.tsx
- **Commit:** ba0e9ce7

**2. TDD order in Task 1:** I wrote the relayClient parser before its tests. The Task 2 failure-rule and WebShareSessionView tests were written first, and I saw them fail (6 assembler tests and 1 view test) before the implementation.

**3. Commit ledger:** the worktree guard refused the write to `.git/worktrees/.../gsd-plan-head-before-11-05`, so the plan base was taken from `/tmp/p11-05-base` (8e07f421). Same value.

## Known Stubs

None.

## Threat Flags

None. No new endpoints or file access. The new code handles bytes that arrive on the existing socket. T-11-16 through T-11-19 are mitigated as planned and have tests.

## Self-Check: PASSED

- FOUND: frontend/src/lib/previewAssembler.ts, frontend/src/lib/previewStore.ts, frontend/src/lib/__tests__/previewAssembler.test.ts, frontend/src/lib/__tests__/previewStore.test.ts, frontend/src/components/__tests__/TerminalPanel.preview.test.tsx
- FOUND: 8f98a701, a882f89e, ba0e9ce7 (ancestors of HEAD)
