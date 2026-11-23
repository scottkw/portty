# Phase 14 deferred items

## From 14-01 (found, not fixed: outside the plan's changes)

- **RESOLVED (fixed at `9b3e6b2a`, noted by the user at the 14-14 checkpoint, 2026-11-22): the spec now reads the copy event's own clipboardData.** The 14-12 gate's Playwright run had 0 failures.
- **Firefox Playwright, `selection-offset.spec.ts:179` ("mouse tracking on (Claude Code fullscreen), scaled pane ... copy carries it").** Fails 3 of 3 on a repeat run: `copied(page)` returns `""`, expected `"line-59 abcdefg"` (clipboard readback in Firefox). The spec mocks the WebSocket and does not touch a gated route; the three other Firefox selection-offset tests and every Firefox gated-route spec pass. Firefox 148 (`firefox-1511`) was first installed in this environment during 14-01, so this spec may never have run on Firefox here before. A baseline run at the pre-plan commit was attempted in a scratch worktree and could not be set up (global-setup runs `pnpm install`, which refuses a symlinked node_modules), so "pre-existing" is not proven by a baseline run.
- **WebKit `selection-offset.spec.ts:152` first test, one run** (scale 0.709 vs 1). Did not recur: 0 failures in 3 repeats of the spec on WebKit and Firefox, and the second full run passed it. Treat as a flake.
- **Homebrew node 26.13.1 hangs `playwright install` at the zip-extract step** (after the first file, 0 percent CPU, reproduced twice). Node v24.22.1 from nvm (`PATH=$HOME/.nvm/versions/node/v24.22.1/bin:$PATH`) installs cleanly. Later plans that install a browser need the same workaround.

## From 14-08 (found, not fixed: outside the plan's changes)

- **`frontend/src/lib/__tests__/pathLinks.linkifier.test.ts` "after a reconnect, another cell of the same row offers the link" failed once in the full vitest run** (`term.onRender fired: expected 2 to be greater than 2`, `renderOnce` waits a fixed `sleep(100)` for a real xterm render). It passed alone (9 of 9) and the file imports nothing 14-08 touched (no phoneQuery, banner, ErrorBoundary or PdfPreview). Treat as a timing flake under full-suite load; the full suite was run once per plan rule so no repeat was made.
  - **Filed 2026-11-22 (14-14 checkpoint, "Todo to fix later"):** `.planning/todos/pending/2026-11-22-linkifier-reconnect-flake.md`. It failed again in the 14-12 gate and passed alone.
