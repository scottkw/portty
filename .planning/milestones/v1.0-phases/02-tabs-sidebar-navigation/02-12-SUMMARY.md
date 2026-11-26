---
phase: 02-tabs-sidebar-navigation
plan: 12
subsystem: frontend-style, web-pages, testing
status: complete
tags: [look-01, them-01, tokens, focus-ring, scrollbars, testing-md, phase-gate]
requires:
  - 02-03 two-block colour tokens and style.tokens.test.ts
  - 02-04..02-11 rules written on tokens
provides:
  - style.tokens.test.ts as the LOOK-01 proof, driven by TOKEN_BLOCK_SELECTORS (02-13 can register a fallback block)
  - token-only style.css with one global focus ring and no ::-webkit-scrollbar rules
  - light/dark join.css and dashboard.css guarded by TestAssets_WebPagesFollowColorScheme
  - TESTING.md reconciled for plans 02-01..02-12 (M-67 added)
affects:
  - 02-13 adds its data-appearance block to TOKEN_BLOCK_SELECTORS, appends M-68 and re-runs the phase gate
tech-stack:
  added: []
  patterns:
    - "Literal colours live only in the blocks listed in TOKEN_BLOCK_SELECTORS; the gate blanks those blocks and scans every declaration value"
    - "Global `:focus-visible { outline: 3px solid var(--color-focus-ring); outline-offset: 1px }` plus `:focus:not(:focus-visible) { outline: none }`; component rules may only restate the same ring (e.g. -3px offset)"
    - "Scrollbars are hidden with standard `scrollbar-width: none`, never ::-webkit-scrollbar"
key-files:
  created: []
  modified:
    - frontend/src/__tests__/style.tokens.test.ts
    - frontend/src/style.css
    - frontend/src/components/__tests__/style.contrast.test.ts
    - frontend/src/components/__tests__/style.hub.modal.test.ts
    - frontend/src/components/__tests__/style.redesign.test.ts
    - frontend/src/components/__tests__/style.settings.test.ts
    - web/assets/join.css
    - web/assets/dashboard.css
    - internal/webserver/assets_test.go
    - TESTING.md
  deleted:
    - frontend/src/components/__tests__/style.hub.test.ts
decisions:
  - "The four remaining ::-webkit-scrollbar rules were hide rules, not custom-styled scrollbars (02-11 already removed those). They became `scrollbar-width: none` (html/body, .xterm-viewport, .tab-list) so the gap fix and hidden tab/terminal scrollbars keep working"
  - "ShellWebShareBanner kept its class names (tests pin them); its rules moved onto the warning banner pair, with --color-destructive for the red edge and Share button"
  - "Find bar moved onto the popover tokens; active toggles use --color-chip fill with --color-accent glyph"
  - "Filled-button hover on the web pages uses `filter: brightness()` instead of a second literal colour"
  - "Per-component focus rings that already matched the global ring were kept; only the off-token ones (#7aa2f7, --hub-accent, 2px) were deleted"
  - "Dead rules deleted after git grep showed no user: the daemon-panel family except __btn, .hub, .sr-only"
metrics:
  duration: "about 20 min"
  completed: 2026-11-15
  tasks: 3
  files: 11
estimate:
  tokens: 110000
  tasks: 3
actuals:
  tokens: 22500
  tasks: 3
  commits: 3
plan_head_before: bb123eca89e2f258fe18a1120bb76d04d2d6e919
plan_head_after: c0d738d6a6f5c818ae8f592e37f8129927fff0a0
---

# Phase 2 Plan 12: No Hardcoded Colours, Web Pages Light/Dark, Phase Gate Summary

style.css now has no literal colour outside its two token blocks, which `style.tokens.test.ts` proves. The custom scrollbar rules are gone and one 3px `--color-focus-ring` ring covers every control. The browser join and dashboard pages follow light/dark with the same tokens. TESTING.md matches the final Phase 2 suite, and every phase gate passes.

## What was built

- **Gate first (Task 1, tracer):** `style.tokens.test.ts` now reads its token blocks from `TOKEN_BLOCK_SELECTORS`. Each entry is a nested header path plus the UI-SPEC column (light or dark) its values must match. Checks (a) and (b) loop over the entries, and (g) blanks them out. New checks:
  - (g) No hex, `rgb()`/`rgba()`/`hsl()`/`hsla()` or named colour in any declaration value outside the token blocks. `transparent`, `currentColor`, `inherit`, `Canvas` and `CanvasText` are allowed.
  - (h) No `::-webkit-scrollbar` and no `--hub-scrollbar`.
  - (i) The global `:focus-visible` ring and the `:focus:not(:focus-visible)` reset exist, and no `:focus-visible` rule draws any other outline.
  - (j) No `style={{ … }}` in `App.tsx` or `components/**/*.tsx` (tests excluded) has a hex or `rgb()`/`hsl()` colour.
- **Failing count before the sweep:** 3 of 15 tests failed. (g) listed 56 literal colours: body, the dead daemon panel, the ShellWebShareBanner rules, the find bar and one rgba. (h) found 4 `::-webkit-scrollbar` rules. (i) found no global ring and 7 off-token focus rings. After the sweep all 15 pass. The TSX gate scans 12 inline style objects across 60+ files, and none has a colour (02-10 and 02-11 had already removed them).
- **Sweep:** body is on `--color-window`/`--color-text` (the desktop vibrancy override `html.is-desktop body { transparent }` is untouched). The find bar and the ShellWebShareBanner (`.webgl-recovery-banner*`) are on tokens. The dead daemon-panel family, `.hub` and `.sr-only` are deleted. The global ring sits after the reset. `outline: none` was removed from `.tab__rename-input`, `.find-bar__input` and the Funnel expiry `select`, so keyboard focus shows the ring there too.
- **Old gates (per file):**
  - `style.hub.test.ts`: **deleted.** It pinned old hex values (`#3b82f6`, `#22c55e`), removed Hub card/briefing/filter selectors, and alias and focus checks that (c) and (i) now cover. Its two live absence guards moved into `style.tokens.test.ts` under "Removed UI stays removed": CHAT-01 (no chat CSS) and TEST-03 (no `data-agent` or agent chip).
  - `style.hub.modal.test.ts`: **retargeted.** The dialog scrim and dialog animation checks stay. The removed-briefing block is dropped.
  - `style.contrast.test.ts`: **retargeted.** Only the WCAG AA `--color-text-secondary` pairs (both modes) stay. All the `#565f89` and removed-selector pins are dropped.
  - `style.redesign.test.ts`: **retargeted.** Font, CDN, alias, radius, type-scale and New Session folder-display checks stay. Dropped: the "≥ 8 font-family uses" count (deleting `.hub` took it to 7; the system font is inherited from body), and the `#7C8CFF`, agent-badge, status-bar and New Session shell-line/agent pins.
  - `style.settings.test.ts`: **retargeted.** The sheet, row and Switch checks stay. Its `--hub-scrollbar` and no-hex checks are dropped because (h) and (g) now cover them.
  - The SM-02 Share dialog scroll-region test lives in `SessionShareModal.test.tsx` and was not touched.
- **Web pages (Task 2):** `join.css` and `dashboard.css` start with `:root { color-scheme: light dark; … }` (Light) and `@media (prefers-color-scheme: dark) { :root { … } }`, using the same UI-SPEC names and values as the app. Every rule reads `var(--color-*)`. Fonts are `-apple-system, BlinkMacSystemFont, system-ui, sans-serif`, with `ui-monospace, "SF Mono", Menlo, monospace` for codes. Every selector and layout value was kept. dashboard.css is 2704 bytes (TestAssets_FirstPartyCSS needs at least 1 KB). Both stay first-party external stylesheets, so the CSP is unchanged. `TestAssets_WebPagesFollowColorScheme` reads both files from `web.WebFS`. It requires `color-scheme: light dark`, the dark media query and exactly two `:root` blocks, and no hex, `rgb(` or `hsl(` outside them.
- **TESTING.md (Task 3):**
  - Section 2: counts are 107 / 91 / 7 / 1 = 206, and the Guards cells are reworded. A "Portty Phase 2 plan 02-12 (final Phase 2 reconciliation)" note lists every Phase 2 test file deleted, renamed or added, plus the current five fuzz targets.
  - Section 4: rows that pointed at the deleted `style.hub.test.ts` (RDS-02, RDS-04, TEST-03, CHAT-01) now point at `style.tokens.test.ts`. The LOOK-01 row describes (g)-(j), and a THEM-01 row was added for `assets_test.go`.
  - Section 5: the retired M-02 and M-06 stubs are deleted. M-13 is rewritten as the two-session isolation check from sidebar rows (no card, no chat). Removed-surface wording is fixed in M-07, M-42, M-43, M-57 (Settings ▸ Sharing, inline reason, renamed test file), M-59 (⌘W now closes the tab), M-61 and M-62. M-67 is added.

## Deviations from Plan

### Auto-fixed / scope adjustments

1. **[Rule 1 - Bug] The four `::-webkit-scrollbar` rules were hide rules, not custom scrollbars.** They prevent a 16px gap with "Always show scrollbars", and they hide the tab-strip and xterm-viewport scrollbars. Deleting them outright would have brought back visible scrollbars and the gap. They became the standard `scrollbar-width: none`, which `.xterm-viewport` and `.tab-list` already declared. This meets (h) and keeps the behaviour. WebKit has supported `scrollbar-width` since Safari 18.2. On an older system WebKit the gap could return (VM check below). Commit e1c7c0d7.
2. **[Rule 3 - Blocking] TESTING.md Section 4 rows were retargeted in the Task 1 commit, not Task 3.** Deleting `style.hub.test.ts` would otherwise fail `tests/check-traceability-paths.sh`, which the standing rule requires before every commit. Commit e1c7c0d7.
3. **[Rule 1 - Bug] The first (i) draft flagged the `:focus:not(:focus-visible)` reset itself.** The selector filter now ignores `:not(:focus-visible)`. This was fixed before the commit.
4. **Additional stale Section 5 items** beyond the 15 lines the plan counted (M-42 cited the deleted chat-parity spec, M-43 a plugin "theme" setting that no longer exists, M-57 the old Settings → Web Server tooltip, M-59 "⌘W does nothing yet"). Each was a one-line fix.
5. **M-67 also lists this plan's native-only app checks** (focus ring, find bar, shell warning banner, overlay scrollbars) beside the join/dashboard Light/Dark check. The plan reserves only M-67, and M-68 belongs to 02-13.

## Gate results (all on macOS, HEAD c0d738d6 for TESTING.md, e1c7c0d7/3a9d4534 for code)

| Gate | Result |
|------|--------|
| `go build ./... && go vet ./...` | pass |
| `go test -race -short ./...` | pass (exit 0) |
| Linux cross-compile: `-tags playwrightfixture ./cmd/playwright-fixture` and `./internal/...` | pass |
| `pnpm -C frontend install --frozen-lockfile` | pass (lockfile up to date) |
| `pnpm -C frontend exec tsc --noEmit` | pass |
| `pnpm -C frontend test` (vitest) | 91 files / 1142 tests pass (was 92 / 1176; −1 file, −34 obsolete assertions) |
| `pnpm -C frontend exec playwright test --project=chromium` | 7 passed, 9 skipped (same as 02-11) |
| `bash tests/build-script.test.sh` | 37 passed, 0 failed |
| `bash tests/check-traceability-paths.sh` | OK, 91 paths |
| `bash scripts/grep-gate.sh` | PASS, all action refs SHA-pinned |
| Section 5 removed-surface grep | 0 (baseline 15 at plan HEAD 07ac2bf5; 7 at this plan's start) |
| Removed-file `git ls-files` check | empty |
| Requirement rows NAV-01 / NAV-02 / NAV-03 / TERM-03 / CHAT-01 / THEM-01 / LOOK-01 | 2 / 4 / 9 / 6 / 8 / 6 / 12 |
| Fuzz 10 s each: FuzzParseFrame, FuzzInputAbsorberFilter, FuzzRequireCapability, FuzzWebServerRoutes, FuzzVerify | all ok, no new crasher files |
| `wails build -platform darwin/arm64 -tags wailsassets` | built `build/bin/agenthub.app`; wailsjs tree restored, `git status -- frontend/src/wailsjs` empty |
| Task 1 acceptance: `::-webkit-scrollbar` / `--hub-scrollbar` / `var(--color-focus-ring)` / `style={{` in gate | 0 / 0 / 14 / present |
| Task 2 acceptance: `prefers-color-scheme: dark` total, `color-scheme: light dark` in join.css, `TestAssets_FirstPartyCSS` | 2 / 1 / PASS |

golangci-lint was not run as a gate (per plan). The only Go file touched is a test file, and it is gofmt-clean.

## End-of-phase human-check list (for /gsd-verify-work)

Browser checks use the dev-browser skill. Native checks need the built app with the old AgentHub daemon stopped (shared socket and port 7443). Peer attach needs a second Mac on the tailnet. Run every item in Light, then Dark.

- **Category A, M-01:** the Share dialog as a 560px sheet under the toolbar, with no header ×, Done/Esc to close, the native-looking segmented control, and the Funnel risk panel on the warning colours.
- **Category B, M-03 / M-04 / M-05:** sidebar peer attach covers the first open with a join code, the cached-cap reopen, and the peer states (asleep, not sharing, Tailscale off).
- **Category C, M-07 / M-54 / M-55 / M-56:** the terminal repaints on a Light/Dark switch, the New Session dialog truncates long paths, a tab closes on shell exit, and sessions survive hiding and relaunching the GUI.
- **M-13:** two peer sessions open in two isolated tabs.
- **M-57:** Tailscale off and on (banner, dimmed Share, Settings ▸ Sharing reason).
- **M-58:** hidden-inset window chrome, top-bar clicks, window drag, sidebar vibrancy (on a real host).
- **M-59:** menu bar accelerators (⌘T, ⌘,, Help, ⌘H, ⌘Q confirm, ⌘W closes a tab, ⌃⌘S).
- **M-60:** live appearance switch across the chrome, both terminals, Settings, the web share page and the tray glyph.
- **M-61:** sidebar navigation (session tabs only, CLI-created row within 3 s, hide/show, gray when not key, footer).
- **M-62:** nothing chat-related in the app or the web share page; the chat history route returns 404.
- **M-63:** tabs, ⌘W detach, End Session confirm, rename in tab and row, 15-tab overflow, toolbar Share states.
- **M-64:** every dialog in the sheet style (focus start, Tab trap, Esc, no scrim close, reduced motion).
- **M-65:** every banner in the shared style (order, icons, Open Tailscale).
- **M-66:** the Settings sheet (panes, switches, Start Server reason with VoiceOver, Security confirms).
- **M-67:** the browser join and dashboard pages in Light/Dark, plus this plan's app checks (below).

## Needs VM check

Built app: `build/bin/agenthub.app` (code from 3a9d4534; TESTING.md-only commit after it). In Light, then Dark:

1. **Focus ring:** Tab through toolbar buttons, the Share button, sidebar rows, tab labels and × buttons, a dialog's buttons and fields, Settings pane tabs, switches and fields, the find bar's buttons and input, the Funnel panel's Disable and hold buttons, and the expiry select. Each shows the same soft blue 3px ring. Sidebar rows and tab labels draw it inside their edge. A mouse click shows no ring, except in text fields. Context menu items still show the blue selection fill rather than a ring.
2. **Tab rename field:** double-click a tab name. The field now shows the focus ring. Check that the ring is not clipped badly by the tab pill. If it looks wrong, restoring `outline: none` on `.tab__rename-input` is a one-line change and the gate still passes.
3. **Find bar (⌘F):** popover background, thin border and soft shadow. Typed text and the "n of m" count are readable. Active Aa / .* / word toggles show a grey fill and blue glyph. "No results" is red. The input shows the focus ring plus the blue underline.
4. **Shell web-share warning:** with "Ask before sharing a session" on, turn on sharing for a shell session in the Share dialog. The warning sits on the yellow warning colours with a red left edge, the text is readable, Cancel is outlined, the red Share button has white text, and × dims on hover.
5. **Scrollbars:** the tab strip (15 tabs) and the terminal show no scrollbar. Set System Settings ▸ Appearance ▸ Show scroll bars to "Always": there is no ~16px gap at the window's right edge (this was the old `html::-webkit-scrollbar` rule's job; it now relies on `scrollbar-width: none`). Long Settings panes and the Share dialog show macOS overlay scrollbars that follow the appearance.
6. **Body background:** in the desktop app nothing changes, because the root stays transparent for vibrancy. In a browser web-share guest page, the background around the terminal is white in Light and #1E1E1E in Dark (it was always Tokyo Night navy before).
7. **Browser pages (dev-browser or Safari):** `/dashboard` and `/join?code=…`, including the expired and invalid states, follow the browser's light/dark preference with no reload. The Join button is blue with white text, Go Back is a grey chip, and the code field and placeholder are readable.

## Known Stubs

None added. The interim "Automatic" Appearance row from 02-11 is still in place for 02-13.

## Threat Flags

None. T-02-37: join.css and dashboard.css stay first-party external files, with no inline style and no remote font; `web-csp.spec.ts` passed in the Playwright gate. T-02-38: the gate failed first (3 tests, 56 literal colours) before the sweep made it pass. T-02-39: the traceability script passes, and the Section 5 grep is 0.

## Self-Check: PASSED

- FOUND: frontend/src/__tests__/style.tokens.test.ts, web/assets/join.css, web/assets/dashboard.css, internal/webserver/assets_test.go, TESTING.md
- MISSING (intended): frontend/src/components/__tests__/style.hub.test.ts
- FOUND commits: e1c7c0d7, 3a9d4534, c0d738d6
