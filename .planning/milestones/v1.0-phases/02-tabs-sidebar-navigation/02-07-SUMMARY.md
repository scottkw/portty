---
phase: 02-tabs-sidebar-navigation
plan: 07
subsystem: frontend-chat-removal
status: complete
tags: [chat-01, chat-removal, relay-client, web-share, dependencies, css, testing]
requires: ["02-06"]
provides:
  - "Web share page and desktop tabs render only the terminal: no chat button, badge, panel or mention popover"
  - "Frontend relay codec limited to output/resize/input/resize2/ping; retired 0x30-0x37 bytes parse as unknown"
  - "WebShareSessionView at frontend/src/components/WebShareSessionView.tsx; frontend/src/components/Hub is gone"
  - "Five chat-only npm dependencies removed (lockfileVersion 9.0, frozen install passes)"
affects: ["02-08", "02-12"]
actuals:
  tokens: 75300
  tasks: 2
  commits: 2
commits: 2
plan_head_before: 43a96453d8367b07f6972a669a4116596f8ec7b0
plan_head_after: bba55a9cc369d74ff27ee7b9d1a7f852d2015086
tech-stack:
  added: []
  removed: [react-markdown, rehype-sanitize, remark-gfm, react-textarea-autosize, "@tanstack/react-virtual"]
  patterns:
    - "Retired wire bytes are documented in one RETIRED comment and fall to parseServerFrame's default unknown arm; a test loops every retired byte"
    - "When removed CSS was a design token's last consumer, the token and its consumption pin go with it"
key-files:
  created: []
  moved:
    - frontend/src/components/Hub/WebShareSessionView.tsx -> frontend/src/components/WebShareSessionView.tsx
    - frontend/src/components/Hub/WebShareSessionView.test.tsx -> frontend/src/components/__tests__/WebShareSessionView.test.tsx
    - frontend/src/components/Hub/__tests__/WebShareSessionView.plugin-config.test.tsx -> frontend/src/components/__tests__/WebShareSessionView.plugin-config.test.tsx
  deleted:
    - frontend/src/components/Hub/ChatPanel.tsx (+ test)
    - frontend/src/components/Hub/ChatMessage.tsx (+ test)
    - frontend/src/components/Hub/ChatBadge.tsx (+ test)
    - frontend/src/components/Hub/ChatDaySeparator.tsx (+ test)
    - frontend/src/components/Hub/MentionPopover.tsx (+ test)
    - frontend/src/components/Hub/TerminalChatHost.tsx (+ test)
    - frontend/src/components/Hub/useChatUnreadListeners.ts (+ test)
    - frontend/src/components/Hub/chatToggleOverlap.test.ts
  modified:
    - frontend/src/lib/relayClient.ts
    - frontend/src/lib/relayClient.test.ts
    - frontend/src/App.tsx
    - frontend/src/style.css
    - frontend/package.json
    - frontend/pnpm-lock.yaml
    - frontend/e2e/web-share-scope.spec.ts
    - frontend/e2e/web-plugin-hot-swap.spec.ts
    - frontend/src/components/__tests__/App.test.tsx
    - frontend/src/components/__tests__/App.open-remote.test.tsx
    - frontend/src/components/__tests__/TerminalPanel.scale.test.tsx
    - frontend/src/components/__tests__/style.hub.test.ts
    - frontend/src/components/__tests__/style.hub.modal.test.ts
    - frontend/src/components/__tests__/style.redesign.test.ts
    - TESTING.md
key-decisions:
  - "Local terminal tabs render TerminalPanel directly inside .terminal-wrapper, the pre-Phase-158 layout. TerminalPanel's root already carries flex:1; min-height:0; position:relative, so the .terminal-chat-host wrapper rule went with the host."
  - "WebShareSessionView keeps its hub-modal__body / hub-modal__body--interactive root classes (layout and App.open-remote source gates rely on them). The chat-only position:relative on .hub-modal__body--interactive was removed."
  - "--hub-radius-pill and --hub-font-size-heading were deleted: their only consumers were chat rules, and style.redesign.test.ts pinned that each was consumed."
  - "M-31 step 5 (Phase 157 scale parity through the /app/ redirect) is not chat; it was carried into M-62 step 3 instead of being lost with Category R."
  - "web-vendor-parity.spec.ts was left unchanged: its only chat match is the requirement ID WEBCHAT-01, with no wording that describes chat as present."
requirements-completed: [CHAT-01]
coverage:
  - id: D1
    description: "Web share guest sees only the terminal, no chat control, no sidebar"
    requirement: CHAT-01
    verification:
      - kind: e2e
        ref: "frontend/e2e/web-share-scope.spec.ts (3 tests, chromium)"
        status: pass
      - kind: unit
        ref: "frontend/src/components/__tests__/WebShareSessionView.test.tsx#renders no chat control"
        status: pass
  - id: D2
    description: "Retired 0x30-0x37 frames parse as unknown; output keeps flowing"
    requirement: CHAT-01
    verification:
      - kind: unit
        ref: "frontend/src/lib/relayClient.test.ts#retired chat/presence bytes (CHAT-01) + RelayClient drops retired frames"
        status: pass
  - id: D3
    description: "No chat CSS, no chat components, no chat-only dependencies, components/Hub gone"
    requirement: CHAT-01
    verification:
      - kind: unit
        ref: "frontend/src/components/__tests__/style.hub.test.ts#No chat CSS remains (CHAT-01)"
        status: pass
      - kind: other
        ref: "git ls-files frontend/src/components/Hub (empty); git grep of the five packages (no match); pnpm install --frozen-lockfile"
        status: pass
duration: ~25min
completed: 2026-11-15
---

# Phase 2 Plan 07: Chat UI Removal Summary

The chat UI is gone from the desktop and the web share page. The frontend relay codec now handles only terminal frames and drops retired 0x30-0x37 bytes as unknown. WebShareSessionView moved out of components/Hub, which no longer exists, and five chat-only npm dependencies were removed.

## What changed

**Relay client (`frontend/src/lib/relayClient.ts`).** Removed: the chat, chat-send, presence, typing, alias-set and self constants; the `PresenceEntry` and `ChatMessage` types; four `ServerFrame` members; three encoders; four parse cases; four callbacks and their dispatch; `sendChat` and `sendAliasSet`. One comment now marks 0x30-0x37 as RETIRED. `ServerFrame` is now `output | resize | unknown`.

**WebShareSessionView.** Moved to `frontend/src/components/WebShareSessionView.tsx`, and its two tests moved to `frontend/src/components/__tests__/`. It renders only TerminalPanel, for both the native web guest (direct `wsURL`) and in-app peer tabs (daemon proxy). The plugin-config self-fetch and SSE hot-swap are unchanged.

**App.tsx.** Imports WebShareSessionView from its new path. Local tabs render `<TerminalPanel>` directly, with the same 10 props TerminalChatHost forwarded.

**Deleted (16 files):** ChatPanel, ChatMessage, ChatBadge, ChatDaySeparator, MentionPopover, TerminalChatHost and useChatUnreadListeners, each with its test, plus chatToggleOverlap.test.ts. `frontend/src/components/Hub/` no longer exists.

**style.css.** Removed every `.chat-*`, `.mention-popover*`, `.hub-modal__chat*` and `.terminal-chat-host` rule, the chat reduced-motion block, and two tokens that only chat used. That is about 690 lines, and grep now counts 0 matches.

**Dependencies:** `pnpm remove react-markdown rehype-sanitize remark-gfm react-textarea-autosize @tanstack/react-virtual` removed 108 packages and added none. The lockfile is still `lockfileVersion: '9.0'`, and `pnpm install --frozen-lockfile` passes.

**TESTING.md:**
- Counts: vitest 97 → 89, total 212 → 204.
- Section 4: 22 chat rows removed. The FIX-01, FIX-03 and PARITY-01 rows now point to the moved files. WEBCHAT-03 was reworded, and four CHAT-01 rows were added.
- Section 5: Categories K, L, M, Q, R and S were deleted (M-18..M-24 and M-29..M-33). Category AF with M-62 was added.
- `tests/check-traceability-paths.sh`: OK, 85 paths.

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | 69b193c1 | Web share guest sees only the terminal; relay client ignores retired chat frames; WebShareSessionView moved; chat components deleted |
| 2 | bba55a9c | Chat CSS and chat-only dependencies removed; TESTING.md reconciled |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Chat component deletion and the TerminalChatHost unwrap moved from Task 2 into Task 1**
- **Found during:** Task 1 verification
- **Issue:** Task 1's `tsc --noEmit` gate cannot pass while ChatPanel, MentionPopover, ChatMessage and useChatUnreadListeners still import `ChatMessage` / `PresenceEntry` / `onChat` / `sendChat` from the stripped relay client.
- **Fix:** Task 1 also `git rm`'d the chat components, rendered TerminalPanel directly in App.tsx, and fixed the chat mocks and comments in `TerminalPanel.scale.test.tsx` and `App.open-remote.test.tsx`. Task 2 kept the CSS, dependency, e2e-comment and TESTING.md work.
- **Commit:** 69b193c1

**2. [Rule 1 - Bug] Two design tokens orphaned by the CSS removal**
- **Found during:** Task 2 full vitest run
- **Issue:** `style.redesign.test.ts` asserts that `--hub-radius-pill` and `--hub-font-size-heading` are each consumed by some rule. Their only consumers were `.chat-badge` and `.chat-panel__empty-heading`.
- **Fix:** Deleted both token declarations and both consumption pins, and updated the token comments.
- **Files modified:** frontend/src/style.css, frontend/src/components/__tests__/style.redesign.test.ts
- **Commit:** bba55a9c

**3. [Rule 1 - Bug] style.hub.test.ts pinned chat CSS (WEBCHAT-06)**
- **Issue:** The suite asserted `.chat-msg__alias` / `__tailnet-id` / `__header` truncation rules. This file was not in the plan's file list.
- **Fix:** Replaced those pins with a guard that style.css has no chat rule, and recorded it as a CHAT-01 row in Section 4. Also reworded one stale comment in `style.hub.modal.test.ts`.
- **Commit:** bba55a9c

### Other notes
- The plan's baseline said components/Hub had 39 tracked files. At HEAD 43a96453 it had 18; 02-05 had already removed the rest. The gate (`git ls-files` prints nothing) passes.
- web-vendor-parity.spec.ts is unchanged (see key-decisions).

## Verification

- `go build ./... && go vet ./...`: clean. `go test -race -short ./...`: exit 0.
- `pnpm -C frontend install --frozen-lockfile`: passes. Lockfile head is `lockfileVersion: '9.0'`.
- `pnpm exec tsc --noEmit`: clean. `pnpm test`: 89 files, 1201 tests passed.
- `pnpm exec playwright test --project=chromium`: 7 passed, 9 skipped (the same documented `test.skip`s as earlier plans), including all 3 web-share-scope tests.
- `bash tests/check-traceability-paths.sh`: OK, 85 paths.
- Every acceptance grep in the plan returns its target value (relay symbols 0, chat-toggle in the spec 0, chat CSS 0, the five packages 0, M-62 1, Categories K/L/M/Q/R/S 0, `git grep` of the chat components exits 1).

## Needs VM check

Built `build/bin/agenthub.app` with `./build.sh --platform macos`, then restored the generated wailsjs tree. Stop any older daemon in the VM before checking:
1. Open a local session tab. There is no chat button or badge at the bottom-right of the terminal. The terminal fills the area above the status bar, the status bar stays fully visible, and resizing the window refits the terminal (TerminalPanel now sits directly in `.terminal-wrapper`).
2. Open a tailnet peer's session from the sidebar (an in-app `__websession__` tab). It shows the terminal at full height with no chat button.
3. Open a web share link in a browser. Only the terminal shows: no chat button, badge, panel or sidebar.

## Known Stubs

None.

## Self-Check: PASSED

- FOUND: frontend/src/components/WebShareSessionView.tsx, frontend/src/components/__tests__/WebShareSessionView.test.tsx, frontend/src/components/__tests__/WebShareSessionView.plugin-config.test.tsx
- MISSING (intended): frontend/src/components/Hub
- FOUND commits: 69b193c1, bba55a9c (both ancestors of HEAD)
