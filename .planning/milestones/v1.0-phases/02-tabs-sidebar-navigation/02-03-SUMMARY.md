---
phase: 02-tabs-sidebar-navigation
plan: 03
subsystem: ui
tags: [react, css, xterm, light-dark, prefers-color-scheme, tokens, fonts]

requires:
  - phase: 02-01
    provides: html.is-desktop scope, transparent webview over vibrancy, window Appearance left unset
provides:
  - useColorScheme() matchMedia hook (frontend/src/lib/useColorScheme.ts)
  - LIGHT_PALETTE / DARK_PALETTE module constants (frontend/src/lib/terminalPalettes.ts)
  - UI-SPEC --color-*/--shadow-*/--opacity-* tokens, Light on :root and Dark in @media (prefers-color-scheme: dark)
  - html.is-desktop { --color-sidebar: transparent }
  - --hub-* colour tokens reduced to var(--color-*) aliases
  - style.tokens.test.ts token source gate (extended in 02-12)
affects: [02-04 sidebar, 02-08 tab bar, 02-11 settings sheet, 02-12 token sweep]

actuals:
  tokens: 31500
  tasks: 3
  commits: 3
plan_head_before: 8e99a80988917d6f952ac5c92dd4de0921d016d8
plan_head_after: 7a77accf87d44b7d22521d3363ee2e51da2efdd7

tech-stack:
  added: []
  removed: [xterm-theme]
  patterns:
    - "Appearance comes only from prefers-color-scheme: CSS tokens via the media block, terminals via useColorScheme()"
    - "Palettes are module constants, so the TerminalPanel theme effect (clearTextureAtlas) runs only on a real scheme change"
    - "Legacy --hub-* tokens are aliases; later plans rename references and delete them"

key-files:
  created:
    - frontend/src/lib/useColorScheme.ts
    - frontend/src/lib/terminalPalettes.ts
    - frontend/src/lib/__tests__/useColorScheme.test.tsx
    - frontend/src/lib/__tests__/terminalPalettes.test.ts
    - frontend/src/__tests__/style.tokens.test.ts
    - frontend/e2e/web-color-scheme.spec.ts
  modified:
    - frontend/src/App.tsx
    - frontend/src/components/SettingsTab.tsx
    - frontend/src/components/SettingsSearch.tsx
    - frontend/src/components/TerminalPanel.tsx
    - frontend/src/components/Hub/WebShareSessionView.tsx
    - frontend/src/style.css
    - frontend/src/test-setup.ts
    - frontend/src/vite-env.d.ts
    - frontend/index.html
    - frontend/package.json
    - frontend/pnpm-lock.yaml
    - TESTING.md
  deleted:
    - frontend/src/themes.ts
    - frontend/src/xterm-theme.d.ts
    - frontend/src/assets/fonts/ (5 woff2 + LICENSES.md)
    - frontend/src/__tests__/themeTokens.test.ts
    - frontend/src/components/__tests__/SettingsTab.appearance-theme.test.tsx

key-decisions:
  - "--hub-* colour tokens alias --color-* by role (full mapping in SUMMARY); unused --hub-teal, --hub-orange, --hub-empty-icon-color dropped"
  - "themeTokens.test.ts's RDS-02 migrated-surface hex scan and reduced-motion checks were carried into style.tokens.test.ts, not lost"
  - "Existing .terminal-session-container class carries background: var(--color-terminal-bg); no new class"
  - "Web share view (WebShareSessionView) also defaults to 13px, matching the desktop terminal"
  - "SettingsSearch's 'Terminal Theme' entry removed with the control it pointed at"

requirements-completed: [THEM-01]

coverage:
  - id: D1
    description: "useColorScheme follows matchMedia changes, rapid changes keep the last value, listener removed on unmount"
    requirement: THEM-01
    verification:
      - kind: unit
        ref: "frontend/src/lib/__tests__/useColorScheme.test.tsx"
        status: pass
    human_judgment: false
  - id: D2
    description: "Both palettes match UI-SPEC exactly (21 keys each) and are stable module constants"
    requirement: THEM-01
    verification:
      - kind: unit
        ref: "frontend/src/lib/__tests__/terminalPalettes.test.ts"
        status: pass
    human_judgment: false
  - id: D3
    description: "Web share page terminal and --color-window follow the browser scheme live"
    requirement: THEM-01
    verification:
      - kind: e2e
        ref: "frontend/e2e/web-color-scheme.spec.ts (chromium, 2 passed)"
        status: pass
    human_judgment: false
  - id: D4
    description: "Two token blocks with the same names and UI-SPEC values, no data-ui-theme, no --hub-* literal colours, no @font-face"
    requirement: LOOK-01
    verification:
      - kind: unit
        ref: "frontend/src/__tests__/style.tokens.test.ts"
        status: pass
    human_judgment: false
  - id: D5
    description: "No theme picker: Settings shows Appearance 'Automatic'"
    requirement: THEM-01
    verification:
      - kind: unit
        ref: "frontend/src/components/__tests__/SettingsTab.test.tsx"
        status: pass
    human_judgment: false
  - id: D6
    description: "Live NSAppearance switch in the native app (chrome, WebGL and DOM terminals, hidden tab, Settings, Safari, tray, OSC 9;4)"
    requirement: THEM-01
    verification:
      - kind: manual_procedural
        ref: "TESTING.md M-60"
        status: unknown
    human_judgment: true
    rationale: "WKWebView appearance propagation and WebGL atlas repaint only happen in the built app; orchestrator runs it in the Tart VM"

duration: 16min
completed: 2026-11-15
status: complete
---

# Phase 2 Plan 03: Live Light/Dark and System Fonts Summary

**The terminal and the whole UI now follow macOS Light/Dark live. useColorScheme() feeds one of two UI-SPEC palettes to every terminal. style.css declares the UI-SPEC tokens in a Light `:root` block and a Dark `prefers-color-scheme` block, and every `--hub-*` colour is an alias. The 138-theme library, the picker and the bundled fonts are gone, and terminals use ui-monospace (SF Mono) at 13px.**

## Performance

- **Duration:** about 16 min
- **Started:** 2026-11-15T19:55:44Z
- **Completed:** 2026-11-15T20:11:00Z
- **Tasks:** 3 of 3
- **Files changed:** 40 (762 insertions, 1380 deletions)

## Accomplishments

- `useColorScheme()` reads `matchMedia('(prefers-color-scheme: dark)')`, follows `change` events, re-syncs once on subscribe and removes the handler on unmount. App derives `terminalTheme = useColorScheme() === 'dark' ? DARK_PALETTE : LIGHT_PALETTE`. Every existing `theme={terminalTheme}` site, including web mode, is unchanged. TerminalPanel's theme effect (with `pendingThemeRef` for hidden tabs) was not touched.
- Removed from App: the theme library, `ALLOWED_THEMES`, the storage keys, `uiTheme`, the `data-ui-theme` and `style.colorScheme` writes. Stale `agenthub:terminalTheme` / `agenthub:uiTheme` localStorage keys are never read.
- Settings ▸ Appearance shows one static row: "Appearance", "Follows your Mac's Light or Dark setting in System Settings.", "Automatic".
- style.css: 41 UI-SPEC tokens in both blocks, `color-scheme: light dark`, `html.is-desktop { --color-sidebar: transparent }`. The `[data-ui-theme="light"]` block, the theme-toggle and theme-select CSS, and the 5 `@font-face` rules are deleted. `.terminal-session-container` paints `var(--color-terminal-bg)`.
- index.html: no inline `color-scheme: dark`; the splash is a blank `Canvas`-coloured cover (follows color-scheme), and the logo is removed. `#splash-static` stays because App hides it on mount.
- `xterm-theme` was removed with `pnpm remove`. The lockfile stays `lockfileVersion: '9.0'`, and `pnpm install --frozen-lockfile` passes.

## Task Commits

1. **Task 1: terminals follow macOS light/dark live via useColorScheme** - `d18b3770` (feat)
2. **Task 2: two-block light/dark colour tokens with --hub-* aliases** - `2ce521fb` (feat)
3. **Task 3: SF Mono 13 terminals on the token background; theme library and bundled fonts dropped; TESTING.md** - `7a77accf` (feat)

## --hub-* → --color-* mapping

| --hub-* | Alias |
|---------|-------|
| --hub-bg | var(--color-window) |
| --hub-surface | var(--color-toolbar) |
| --hub-surface-elevated | var(--color-popover) |
| --hub-border | var(--color-control-border) |
| --hub-border-hover | var(--color-switch-off) |
| --hub-text-primary | var(--color-text) |
| --hub-text-secondary | var(--color-text-secondary) |
| --hub-text-muted | var(--color-text-secondary) |
| --hub-text-placeholder | var(--color-text-secondary) |
| --hub-text-dim | var(--color-text-secondary) |
| --hub-accent | var(--color-accent) |
| --hub-accent-hover | var(--color-accent) |
| --hub-destructive | var(--color-error-text) |
| --hub-success | var(--color-success-text) |
| --hub-warning | var(--color-banner-warning-fg) |
| --hub-group-header-bg | var(--color-toolbar) |
| --hub-scrollbar | var(--color-switch-off) |
| --hub-scrollbar-hover | var(--color-text-secondary) |
| --hub-preview-bg | var(--color-terminal-bg) |
| --hub-preview-text | var(--color-text-secondary) |
| --hub-preview-border | var(--color-separator) |
| --hub-sidebar-bg | var(--color-sidebar) |
| --hub-sidebar-item-active-bg | var(--color-selection-inactive) |
| --hub-sidebar-item-hover-bg | var(--color-control-hover) |
| --hub-drag-over-border | var(--color-accent) |
| --hub-drag-over-bg | var(--color-control-hover) |
| --hub-internet-badge-bg | var(--color-chip) |
| --hub-internet-badge-text | var(--color-accent) |
| --hub-fullaccess-badge-bg | var(--color-chip) |
| --hub-fullaccess-badge-text | var(--color-error-text) |
| --hub-search-highlight-bg | var(--color-selection-inactive) |
| --hub-share-seg-active-bg | var(--color-segment-selected) |
| --hub-font-ui | -apple-system, BlinkMacSystemFont, system-ui, sans-serif |
| --hub-font-mono | ui-monospace, "SF Mono", Menlo, monospace |
| --hub-teal, --hub-orange, --hub-empty-icon-color | deleted (zero references) |

Radii, type-scale and sidebar-width `--hub-*` tokens keep their values.

## Source-gate tests changed (per file)

- `style.hub.test.ts`: the two dark-hex pins now check `--hub-bg`, `--hub-accent` and `--hub-destructive` aliases. The whole light-block describe is deleted (it only pinned the removed block).
- `style.redesign.test.ts`: the `@font-face` describe is inverted (no `@font-face`, no `./assets/fonts/`). The light-block font tests became system-font value checks. The comp-hex palette and accent and semantic tests now check the aliases, and the violet rejection is kept.
- `style.contrast.test.ts`: the three old comp-hex contrast pairs became six UI-SPEC `--color-text-secondary` pairs (sidebar, toolbar and window, both modes). The `#565f89` absence checks are unchanged.
- `style.settings.test.ts`: unchanged (passes; its `#292e42` pin is a non-token rule that 02-11/02-12 rework).
- `HelpTab.test.tsx`: the light-block search-highlight test now checks that `--hub-search-highlight-bg` aliases a `--color-*` token.
- `App.test.tsx`: THM-02/03 became THEM-01 palette assertions. TERM-04 keeps only the no-`NotifyThemeChange` case. `DEFAULT_FONT_SIZE = 13`.
- `SettingsTab.test.tsx`: the THM-01 picker tests and the THM-04 allowlist tests are replaced by a no-theme-props check and a DOM case ("Automatic" renders, no Interface/Terminal Theme control).
- `TerminalPanel.test.tsx` (Rule 3): the inline-background assertions now check the token class, no `#1a1b26`, and the system mono font.
- FindBar theme-matrix/visual: comments only, assertions unchanged.

## RESEARCH A3 font measurement (record only)

I ran a throwaway Playwright script from the scratch dir: `OffscreenCanvas` `measureText` at 13px.

| Engine | ui-monospace W / i | "SF Mono" W / i | Menlo W / i | Full stack W / i |
|--------|------|------|------|------|
| WebKit (Playwright build 2336, already cached, no download) | 8.036 / 8.036 | 12.27 / 3.61 (proportional fallback) | 7.827 / 7.827 | 8.036 / 8.036 |
| Chromium | 12.27 / 3.61 (proportional fallback) | 12.27 / 3.61 | 7.827 / 7.827 | 7.827 / 7.827 |

In WebKit canvas, `ui-monospace` resolves to a monospace face distinct from Menlo, i.e. SF Mono. "SF Mono" by family name does not resolve, as RESEARCH Pitfall 9 predicted. Chromium falls through the stack to Menlo. This is Playwright's WebKit build, not the app's WKWebView, so M-60 still checks for glyph overlap.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] TerminalPanel.test.tsx pinned the inline dark background**
- **Found during:** Task 3
- **Fix:** retargeted it to `background: var(--color-terminal-bg)` on `.terminal-session-container`, no `#1a1b26`, and the system font
- **Commit:** 7a77accf

**2. [Rule 1 - Bug] e2e token read failed on the minified CSS**
- **Found during:** Task 2
- **Issue:** the production minifier writes `--color-window: #fff`, so a literal `#FFFFFF` comparison never matched
- **Fix:** the spec expands 3-digit hex before comparing
- **Commit:** 2ce521fb

**3. [Rule 2 - Coverage] themeTokens.test.ts deletion would have dropped the RDS-02 hex scan and S-07 motion gates**
- **Fix:** carried them into style.tokens.test.ts; they still pass
- **Commit:** 2ce521fb

**4. [Rule 1] Dead references to the removed controls**
- SettingsSearch's "Terminal Theme" search entry was removed. WebShareSessionView's hardcoded `fontSize={14}` is now 13, so the web page matches the UI-SPEC default.
- **Commits:** d18b3770, 7a77accf

### Not done

- `@testing-library/react` is not installed, so the hook test uses a 10-line createRoot probe instead of `renderHook`. No dependency was added.
- `HubPanel.tsx` keeps its own `DEFAULT_FONT_SIZE = 14`. Plans 02-05/02-07 delete it.
- I did not install a Playwright WebKit browser. The A3 measurement used the WebKit build already in the cache.

## Verification

- `go build ./... && go vet ./...`: clean. `go test -race -short ./...`: all ok.
- `pnpm exec tsc --noEmit`: clean. `pnpm test`: 118 files passed.
- `pnpm exec playwright test --project=chromium`: 7 passed, 9 skipped (the pre-existing documented `test.skip` in progress and web-links), including web-color-scheme ×2 and web-csp.
- `pnpm install --frozen-lockfile` passes. The lockfile header is `lockfileVersion: '9.0'`, and package.json has no `xterm-theme`.
- `bash tests/check-traceability-paths.sh`: OK (110 paths).
- `wails build -platform darwin/arm64 -tags wailsassets`: built `build/bin/agenthub.app`. The wailsjs tree was restored afterwards.
- All acceptance greps match their targets. Task 1's grep sum is 0. In style.css, data-ui-theme, `--hub-*` literal colours and `@font-face` all count 0. M-60 appears once, and there are 5 THEM-01 rows.

## Needs VM check

Built app: `build/bin/agenthub.app` (quit any installed AgentHub and stop its daemon first). TESTING.md M-60:

1. Open two terminal tabs, one with WebGL on and one with WebGL turned off in Settings. Run `ls -la /usr/bin` in each, leave one tab hidden, and open Settings.
2. Toggle System Settings ▸ Appearance Light → Dark → Light. Each toggle should repaint the chrome, both terminals and Settings within a second, with no restart and no garbled or overlapping glyphs. Selecting the hidden tab should show the new palette at once. No dark surface should remain in Light mode. If the webview does not switch live, RESEARCH A2 has failed: raise a blocker, because a native appearance observer is out of scope here.
3. Settings ▸ Appearance should read "Automatic" with the follows-your-Mac description.
4. The web share link in Safari should follow a macOS appearance toggle.
5. The tray glyph should stay legible in both modes, and an OSC 9;4 run (`printf '\e]9;4;1;50\a'`) should be legible in both.
6. Splash: launch in Light mode and confirm there is no dark flash before the UI mounts.
7. Optional A3 in-app check: in Web Inspector, compare `measureText('W').width` for `13px ui-monospace` against `13px Menlo`.

Human-check result: pending (VM).

## Known Stubs

None. The static "Automatic" text is the UI-SPEC contract, not a stub.

## Self-Check: PASSED

- Created files exist: useColorScheme.ts, terminalPalettes.ts, both lib tests, style.tokens.test.ts, web-color-scheme.spec.ts.
- Commits d18b3770, 2ce521fb and 7a77accf are ancestors of HEAD.
