# Phase 14 deferred items

## From 14-01 (found, not fixed: outside the plan's changes)

- **Firefox Playwright, `selection-offset.spec.ts:179` ("mouse tracking on (Claude Code fullscreen), scaled pane ... copy carries it").** Fails 3 of 3 on a repeat run: `copied(page)` returns `""`, expected `"line-59 abcdefg"` (clipboard readback in Firefox). The spec mocks the WebSocket and does not touch a gated route; the three other Firefox selection-offset tests and every Firefox gated-route spec pass. Firefox 148 (`firefox-1511`) was first installed in this environment during 14-01, so this spec may never have run on Firefox here before. A baseline run at the pre-plan commit was attempted in a scratch worktree and could not be set up (global-setup runs `pnpm install`, which refuses a symlinked node_modules), so "pre-existing" is not proven by a baseline run.
- **WebKit `selection-offset.spec.ts:152` first test, one run** (scale 0.709 vs 1). Did not recur: 0 failures in 3 repeats of the spec on WebKit and Firefox, and the second full run passed it. Treat as a flake.
- **Homebrew node 26.13.1 hangs `playwright install` at the zip-extract step** (after the first file, 0 percent CPU, reproduced twice). Node v24.22.1 from nvm (`PATH=$HOME/.nvm/versions/node/v24.22.1/bin:$PATH`) installs cleanly. Later plans that install a browser need the same workaround.
