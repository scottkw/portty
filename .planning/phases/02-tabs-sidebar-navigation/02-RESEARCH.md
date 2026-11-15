# Phase 2: Tabs + Sidebar Navigation - Research

**Researched:** 2026-11-15
**Domain:** Wails v2 macOS window chrome, React app-shell restructure (Hub removal), xterm.js live light/dark, protocol/feature deletion (chat)
**Confidence:** MEDIUM-HIGH. Codebase findings were read from source this session (HIGH). Native macOS behavior under Wails, specifically the title-bar click-through and vibrancy look on macOS 27, is the weak spot (LOW to MEDIUM), so the first plan opens with a spike.

## Summary

Most of this phase is deletion and rewiring, not new technology. All the hard pieces already exist in the repo. Wails v2.10.2 exposes every window option the UI-SPEC needs (`mac.TitleBarHiddenInset()`, `WebviewIsTransparent`, `WindowIsTranslucent`, an unset `Appearance`). `TerminalPanel` already applies a changed `theme` prop live, including the safe hidden-panel deferral (POL-04). Remote-peer polling and the peer data shape (`RemotePeerSessions{Hostname, Reachable, Sessions}`) already carry everything the peer groups need. `InlineSessionName` and the TabBar context menu supply the rename and menu behavior. The real work is (1) restructuring `App.tsx` around a session list that is always polled instead of the Hub tab, (2) deleting chat end to end (Go relay/daemon/webserver plus frontend, including presence/typing/alias/self frames, which exist only for chat), (3) collapsing 138 themes and a manual UI-theme toggle into one `prefers-color-scheme` hook, and (4) a token-based CSS rewrite of a 5,722-line `style.css`.

Three findings change the plan's shape. **(a)** Closing a tab currently **kills** the session (`handleCloseTab` calls `ToggleWebServing(id,false)` and then `KillSession`). The detach-only decision needs a new close path plus a separate End Session path. **(b)** `TitleBarHiddenInset()` turns on a real `NSToolbar` (`UseToolbar: true`). On macOS 26+ an `NSGlassContainerView` inside `NSToolbarView` has been reported to swallow clicks meant for content under the toolbar. That is exactly where the tabs, "+", Share and the sidebar toggle live. This machine runs macOS 27.2.1, so Plan 1 must spike this before any toolbar UI is built on it, with `mac.TitleBarHidden()` (no NSToolbar) as the ready fallback. **(c)** The "legacy `theme` key in settings.json" from CONTEXT does not exist. The terminal and UI themes live in **localStorage** (`agenthub:terminalTheme`, `agenthub:uiTheme`). The only settings.json key this phase removes is `stayOnHubAfterCreate`.

**Primary recommendation:** Do the work in serial waves (App.tsx, style.css and TESTING.md are shared hot files): native chrome spike, then Go chat removal, then frontend chat removal, then the THEM-01 hook and tokens, then the nav restructure (sidebar, tabs, Share toolbar, Hub/Help/Welcome/StatusBar deletion), then the Settings sheet, dialogs and menu, then a CSS sweep with a token source gate and TESTING.md reconciliation. Prefer deleting over adapting. Do not add npm or Go dependencies. Six npm dependencies become removable.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### Window & Chrome (LOOK-01)
- Hidden-inset title bar (Wails `mac.TitleBarHiddenInset()`): traffic lights sit in the sidebar's top strip, session tabs live in a unified toolbar to the right, as in the mockup.
- Native vibrancy behind the sidebar only (Wails translucent window + transparent webview where the sidebar is). The terminal and toolbar stay opaque (readability, WebGL renderer).
- System font (`-apple-system`) and SF Mono for terminals. Every color is a CSS variable with one light and one dark set, accent = macOS blue. No hardcoded hex left in components or style.css (fixes UI-REVIEW 01's banner and inline-color findings).
- Icons: keep Heroicons outline (already a dependency), sized and weighted to look like SF Symbols.

#### Sidebar & Tabs (NAV-01..03, TERM-03)
- Sidebar: "This Mac" group with every local session, then one group per peer machine listing that peer's shared sessions, then a Tailscale status footer (dot + "connected · <host>" / "not connected"). No Home, Hub, Help, Settings or custom-group items.
- Clicking a local session focuses its tab or opens one; clicking a peer session attaches to it in a tab (replaces the Hub remote cards, so GUI remote attach never regresses).
- Closing a tab only detaches: the shell keeps running in the daemon and stays in the sidebar. Right-click "End Session…" (with a confirm) kills it.
- Rename: double-click the name in the sidebar row or the tab for inline edit, plus a right-click "Rename" item. The new name shows in both places.
- No tabs open: a quiet empty state with a "New Session" button (⌘T). The Welcome tab is removed. App launches straight into sidebar + tabs (no Hub anywhere).
- When Tailscale is off, the peer groups show one muted line ("Peers appear when Tailscale is connected") and Share is disabled with the NET-01 reason visible on screen (not tooltip-only; UI-REVIEW 01 fix #2/#3: stable accessible name, reason via `aria-describedby`).

#### Settings, Help, Share & Status Bar
- Settings opens as a sheet over the main window (⌘, in the app menu), styled like the mockup's Settings window with General / Sharing / Security panes in macOS grouped-form style. (Wails v2 is single-window, so a sheet, not a second window.) The theme pickers go away; Appearance shows "Automatic — follows System Settings".
- Help: the in-app Help tab and help search are removed. Help ▸ Portty Help opens the docs site in the browser (portty.app docs are written in Phase 3).
- Share in Phase 2: a toolbar Share button opens the existing Share dialog, restyled to the new look, with the Funnel risk panel still reachable (ROADMAP SC5). The one-switch popover from the mockup is Phase 4.
- The bottom status bar is removed. Share moves to the toolbar; Tailscale status moves to the sidebar footer.

#### Light/Dark, Chat & Cuts (THEM-01, CHAT-01)
- Terminals: one built-in dark ANSI palette and one built-in light palette, tuned to the new chrome. Open terminals and the app switch live on `prefers-color-scheme` change, no restart. The web share page follows the browser the same way with the same palettes.
- The 138-theme library, theme picker and its settings key are removed. A legacy `theme` key in settings.json is ignored on load and dropped on the next save (Phase 1 pattern).
- Chat is removed everywhere: desktop and web chat UI, daemon/webserver chat API and store, relay chat frames, FuzzSanitizeChatContent, chat CORS helpers (FilesCORS naming). Old clients' chat frames are dropped without crashing anything, mirroring how Phase 1 retired the 0x35/0x36 inject frames.
- Custom session groups (POL-05: create/rename/drag-drop groups) are cut; the sidebar groups by machine only.
- Kept as-is (user did not select them for cutting): plugin toggles, OSC 9;4 progress glyphs, sixel.

### Claude's Discretion
- Exact palette values (must pass 4.5:1 text contrast in both modes; selected sidebar row uses a darker blue fill with white text, as in the mockup).
- How the Share dialog, Funnel risk panel and web-share session view are relocated out of `components/Hub/` (move, not delete).
- Plan order and granularity, as long as every plan leaves build and tests green.

### Deferred Ideas (OUT OF SCOPE)
- One-switch Share popover (on/off, web link, `portty attach` line) from the mockup — Phase 4.
- Follow the user's chosen macOS accent color live (needs a native bridge) — not planned.
- Remaining cut candidates not chosen now: plugin toggles, OSC 9;4 glyphs, sixel, key regen + CT notice, extra CLI commands, launchd install, tray menu, join codes/QR — revisit in later phases.

**Also locked:** `02-UI-SPEC.md` (approved 2026-11-15) is the visual and interaction contract. Its token tables, terminal palettes, component contracts, copy and menu bar list are binding. This research does not repeat them. The planner should cite UI-SPEC sections directly.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| NAV-01 | Sidebar lists local sessions; click focuses or opens a tab | The "Session list source" pattern (always-on 3 s `ListSessions` poll replaces the Hub-gated poll). `handleOpenSessionTab` (App.tsx) already does find-or-open |
| NAV-02 | Sidebar lists tailnet peers' shared sessions grouped by machine; click attaches in a tab | `GetRemoteSessionsWithMeta` returns `[]RemotePeerSessions{Hostname, Reachable, Sessions}` (app.go:61-65). `handleOpenRemoteSession` + `RemoteJoinCodeModal` + `openWebSessionTab` already attach in-app. The poll must drop its `activeId !== HUB_TAB.id` gate |
| NAV-03 | Hub grid gone; app opens straight into tabs + sidebar | Hub blast-radius inventory (below). Delete `HUB_TAB`/`WELCOME_TAB`/`HELP_TAB`/`SETTINGS_TAB` and `HubPanel` and its tree |
| TERM-03 | Multiple session tabs; rename and close | Close currently kills (App.tsx:776-787), so a detach-only close plus End Session confirm is needed. Rename goes through `RenameSession` RPC → `engine.tabNames` (engine.go:446-454). Derive tab labels from the session list so both places update together |
| CHAT-01 | Chat removed everywhere; old chat frames dropped without error | Full Go and frontend inventory (below). Retire 0x30-0x34 and 0x37 the same way 0x35/0x36 were retired (no case, no default arm). Extend the existing inject tests into retired-frame tests |
| THEM-01 | App and terminals follow system light/dark live; theme library, picker and key removed; web share page follows the browser | `TerminalPanel` already applies `theme` prop changes live (TerminalPanel.tsx:815-833). Add a `useColorScheme()` hook plus `terminalPalettes.ts`. Theme state lives in localStorage, not settings.json |
| LOOK-01 | Native macOS look: system font, accent, source-list sidebar, unified title/toolbar, native controls, no hardcoded colors | Wails mac options (verified in module source), drag-region mechanics, CSS token migration (`style.css` has 300 hex literals and 584 `var(--hub-*)` refs), plus a new source-gate test |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- **Stack:** keep Go/Wails/React/tsnet. This is a strip-down, not a rewrite. Add no new UI libraries (UI-SPEC Registry Safety).
- **Security:** keep the capability-gated share model, CSP and Funnel consent gates intact through the refactor (Funnel is removed in Phase 4, not here).
- **Chesterton's Fence:** read each piece before removing it, especially daemon/relay/capability code shared between kept and removed features. This matters concretely here: `Subscriber.Origin` drives the PTY resize arbiter and must stay. See Pitfall 4.
- **Silent fallbacks:** no `or {}` style masking. The rename RPC failure must restore the old name, not silently keep the new one (the current code keeps it).
- **Package manager:** Node uses `pnpm`. Go uses `go mod`.
- **Testing:** 80%+ coverage in critical components. Integration tests for routes.
- **TESTING.md standing rule:** every added, renamed or removed test file updates Section 2 (manifest counts), Section 4 (traceability rows, repo-relative `.go/.ts/.tsx/.sh` path only), and Section 5 (M-NN manual items for native-only behavior). Run `bash tests/check-traceability-paths.sh` before committing.
- **Ground-truth commands:** `go build ./... && go vet ./...`, `go test -race -short ./...`, `cd frontend && pnpm exec tsc --noEmit && pnpm exec vitest run`. LSP is stale. Never plain `wails build`. Use `./build.sh --platform macos` or `-tags wailsassets`.
- **GSD workflow:** edits happen inside `/gsd-execute-phase`.
- **Simplicity bias (user memory):** default to cutting. Prefer deletion and reuse over new abstractions.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Window chrome (title bar, vibrancy, appearance) | Native (Wails `options.Mac` in `main.go`) | Browser (CSS transparency + drag regions) | NSWindow/NSVisualEffectView are created natively. CSS only decides where they show through |
| App menu (⌘T, ⌘W, ⌘,, ⌃⌘S, Help) | Native (Wails `menu` in `main.go`) | Browser (EventsOn handlers in App.tsx) | NSMenu key equivalents. Callbacks emit Wails events the React shell handles |
| Local session list, rename, kill | Daemon (session engine) | Browser (poll + optimistic UI) | The daemon owns names (`tabNames`) and lifecycle. The GUI is a view |
| Tab open/close (detach) | Browser (App.tsx tab state) | — | Closing a tab only unmounts the TerminalPanel WS. The daemon is not touched |
| Peer discovery | Daemon (`tailnet.FetchAllPeerSessionsMeta`) | Browser (30 s poll, grouping) | Already implemented. Only the polling gate changes |
| Light/dark | Browser (`prefers-color-scheme` + CSS tokens + xterm `options.theme`) | Native (window `Appearance` unset → follows system) | WKWebView reports the window's effective appearance to CSS |
| Chat removal | Daemon/relay/webserver (Go) | Browser | Wire frames, routes and stores are in Go. UI is in React |
| Web share page theme | Browser (same React bundle under `/app/`) | — | `handleTerminalPage` redirects to the SPA. No server-side theming |

## Standard Stack

No new packages. Everything below is already in the repo.

### Core (existing, verified in repo)
| Library | Version | Purpose | Notes |
|---------|---------|---------|-------|
| `github.com/wailsapp/wails/v2` | v2.10.2 `[VERIFIED: go.mod:17 "github.com/wailsapp/wails/v2 v2.10.2"]` | Window, menu, runtime events | Installed CLI is v2.16.0 (`wails version`). The latest module is v2.17.1 (`go list -m -versions`). Do not bump in this phase unless the spike needs it (see Open Questions) |
| `@xterm/xterm` | 6.0.0 `[VERIFIED: node_modules/@xterm/xterm/package.json]` | Terminal | Measures cells with `OffscreenCanvas.measureText` when available (read in lib/xterm.js) |
| `@xterm/addon-webgl` | 0.19.0 `[VERIFIED: node_modules]` | GPU renderer | `allowTransparency` stays off (UI-SPEC) |
| `@heroicons/react` | 2.2.x | Icons | `24/outline` only (UI-SPEC) |
| React 19 + Vite 8 + Vitest 4 + jsdom 29 | from package.json | Shell + tests | — |

### To remove (become dead in this phase)
| Package | Only consumers (verified with grep this session) | Removed with |
|---------|-----------------------------------|--------------|
| `xterm-theme` | `App.tsx`, `themes.ts`, `vite-env.d.ts`, `xterm-theme.d.ts`, `App.test.tsx` | THEM-01 |
| `react-markdown`, `rehype-sanitize`, `remark-gfm` | `HelpContent.tsx`, `Hub/ChatMessage.tsx` (+tests) | Help tab + chat |
| `react-textarea-autosize` | `Hub/ChatPanel.tsx` | chat |
| `@tanstack/react-virtual` | `Hub/ChatPanel.tsx` | chat |

Removal: `cd frontend && pnpm remove xterm-theme react-markdown rehype-sanitize remark-gfm react-textarea-autosize @tanstack/react-virtual`. Then confirm `pnpm install --frozen-lockfile` passes and the lockfile keeps `lockfileVersion: '9.0'`. Phase 1 recorded that local pnpm 12 must stay removal-only (STATE.md 01-02 decision).

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `mac.TitleBarHiddenInset()` (NSToolbar) | `mac.TitleBarHidden()` (no NSToolbar) | Avoids the macOS 26+ toolbar click-swallowing risk. The traffic lights then sit at the standard ~28px title-bar height instead of centered in the 52px strip. This is the fallback if the spike fails |
| Upgrading Wails to v2.17.1 | Stay on v2.10.2 | v2.11+ includes a macOS 26 WebView crash fix (PR #4806, incremental rendering) `[CITED: github.com/wailsapp/wails/pull/4806 via search]`. Phase 1 UAT passed on v2.10.2, so only bump if the spike or UAT shows crashes |
| Custom ObjC to set `NSVisualEffectMaterialSidebar` | Wails default effect view | Wails never sets a material (see Pattern 1). Accept the default look unless the visual check fails. The repo already has a cgo ObjC file (`tray_objc_darwin.m`) if a fix is needed |

## Package Legitimacy Audit

This phase installs **no** external packages (npm or Go). It only removes six npm packages that are already installed.

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| (none installed) | — | — | — | — | — | — |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
 macOS appearance change ──► WKWebView prefers-color-scheme ──► useColorScheme() ──┬─► :root tokens (CSS, automatic)
                                                                                   └─► palette ──► <TerminalPanel theme> ──► term.options.theme (active)
                                                                                                                         └─► pendingThemeRef (hidden, applied on activate)

 NSMenu (⌘T ⌘W ⌘, ⌃⌘S Help) ──► Go callback ──► runtime.EventsEmit("menu:…") ──► App.tsx EventsOn ──► open dialog / close tab / Settings sheet / toggle sidebar
                                             └─► Help ▸ Portty Help: runtime.BrowserOpenURL(helpURL)

 Daemon ──ListSessions (3 s, desktop only)──► sessions[] ──► Sidebar "This Mac" rows ──click──► find-or-open local tab ──► TerminalPanel (loopback relay WS)
        ──GetRemoteSessionsWithMeta (30 s, desktop && !tailscaleOff)──► peers[] ──► peer groups ──click──► cap cached? ─yes─► OpenRemoteSessionURL ─► web-session tab (peer WS)
                                                                                                              └─no──► RemoteJoinCodeModal ─► ExchangeJoinCodeAtURL + RegisterRemoteCap ─► same

 Tab × / ⌘W ──► removeTabState (UI only: no ToggleWebServing, no KillSession) ──► next tab right, else left, else empty state
 End Session… ──► confirm dialog ──► ToggleWebServing(false) if shared ──► KillSession ──► Go exit watcher emits session:exit ──► tab + row removed
 Rename (sidebar/tab) ──► optimistic sessions[] update ──► RenameSession ──fail──► restore old name
```

### Recommended file layout (end of phase)
```
frontend/src/
├── App.tsx                    # shell: sidebar | main(toolbar, banners, terminal/empty state); dialogs; settings sheet
├── components/
│   ├── Sidebar.tsx            # rewritten: This Mac + peer groups + footer (existing file is groups/nav — not reusable)
│   ├── TabBar.tsx             # reshaped: pills, +, Share button / "On <host>" chip; chevron ▾ removed
│   ├── ContextMenu.tsx        # extracted from TabBar's existing role="menu" (2 consumers: tab, sidebar row)
│   ├── InlineSessionName.tsx  # moved from Hub/ (reused by sidebar row + tab)
│   ├── SettingsSheet.tsx      # SettingsTab content re-hosted in a sheet (or keep SettingsTab.tsx name, restyle)
│   ├── EndSessionDialog.tsx   # new tiny confirm (or a generic ConfirmDialog if it also replaces the disable-warning confirm)
│   ├── RemoteSessionView.tsx  # = WebShareSessionView minus chat (move from Hub/; renaming optional)
│   └── Share/                 # SessionShareModal, FunnelRiskPanel, ShareSegmentedControl (+ SessionShare/* may stay)
├── lib/
│   ├── terminalPalettes.ts    # LIGHT / DARK ITheme from UI-SPEC table
│   └── useColorScheme.ts      # matchMedia hook → 'light' | 'dark'
└── style.css                  # :root light tokens + @media(prefers-color-scheme: dark) tokens; no other hex
```
Everything under `components/Hub/` that is not moved is deleted. `Hub/` should not exist at phase end.

### Pattern 1: Wails macOS window options (verified in module source)

`[VERIFIED: ~/go/pkg/mod/github.com/wailsapp/wails/v2@v2.10.2/pkg/options/mac/mac.go:18-22]`:
```
type Options struct {
	TitleBar             *TitleBar
	Appearance           AppearanceType
	WebviewIsTransparent bool
	WindowIsTranslucent  bool
```
`[VERIFIED: .../pkg/options/mac/titlebar.go:43-49]` `func TitleBarHiddenInset() *TitleBar` sets `UseToolbar: true` and `HideToolbarSeparator: true`. `TitleBarHidden()` (lines 30-36) is identical except `UseToolbar: false` and `HideToolbarSeparator: false`.

What the native side does `[VERIFIED: .../internal/frontend/desktop/darwin/WailsContext.m:139-188, 277-279, 337-345]`:
- `useToolbar` → `[[NSToolbar alloc] initWithIdentifier:@"wails.toolbar"]` attached to the window. This is the source of the risk in Pitfall 1.
- `windowIsTranslucent` → one `NSVisualEffectView` covering the **whole** content view, `NSVisualEffectBlendingModeBehindWindow`, `setState:NSVisualEffectStateActive`. **No material is set**, and the state is forced Active, so the vibrancy does not dim when the window is inactive.
- `webviewIsTransparent` → WKWebView `drawsBackground = NO`. Any CSS surface left transparent shows the vibrancy, so html/body/#root and the sidebar must be transparent, and every other surface must paint an opaque token color.
- `appearance` is only applied when non-nil (empty string → follows system).
- `BackgroundColour` → `NSWindow setBackgroundColor`. If nil, Wails defaults it to white (options.go:168-172), so set alpha 0 explicitly.

```go
// main.go (shape; values from UI-SPEC "Window chrome contract")
BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0},
Mac: &mac.Options{
    TitleBar:             mac.TitleBarHiddenInset(), // fallback: mac.TitleBarHidden() — see Pitfall 1
    WebviewIsTransparent: true,
    WindowIsTranslucent:  true,
    // Appearance unset → follows system
},
```
Import: `"github.com/wailsapp/wails/v2/pkg/options/mac"`.

### Pattern 2: Drag regions (`--wails-draggable`)

`[VERIFIED: .../internal/frontend/runtime/desktop/main.js:91-111, 124-146]`. On every `mousedown` Wails reads `getComputedStyle(e.target).getPropertyValue('--wails-draggable')`. If it equals `drag`, with the primary button and `e.detail === 1`, it calls `preventDefault()` and starts a native window drag. Three consequences follow.
1. **CSS custom properties inherit.** Setting `--wails-draggable: drag` on the toolbar or sidebar strip makes every child button draggable too, and their clicks are swallowed by `preventDefault`. Every interactive descendant must set `--wails-draggable: no-drag` (for example `.toolbar :is(button,[role=tab],input,a) { --wails-draggable: no-drag }`).
2. **Double-click does nothing** (`e.detail !== 1` → no drag, and Wails does not zoom). Double-click-to-zoom on the CSS strip will not work. That is acceptable. Do not hand-roll it.
3. The property name and value are configurable (`options.App.CSSDragProperty/CSSDragValue`). Keep the defaults.

### Pattern 3: App menu with Settings… (custom first submenu)

The `menu.AppMenu()` role is fixed native code `[VERIFIED: .../darwin/WailsMenu.m:64-87]`: optional About (only if `options.Mac.About` is set), Hide ⌘H, Hide Others, Show All, Quit ⌘Q. Go cannot insert "Settings…" into it. On macOS the **first** top-level submenu is always the application menu, so replace the role with a custom first submenu:

```go
m := menu.NewMenu()
app := m.AddSubmenu("AgentHub") // title is shown as the app name by macOS; Phase 3 renames
app.AddText("Settings…", keys.CmdOrCtrl(","), emit("menu:settings"))
app.AddSeparator()
app.AddText("Hide AgentHub", keys.CmdOrCtrl("h"), func(_ *menu.CallbackData) { runtime.Hide(appCtx) })
app.AddSeparator()
app.AddText("Quit AgentHub", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) { runtime.Quit(appCtx) })
file := m.AddSubmenu("File")
file.AddText("New Session", keys.CmdOrCtrl("t"), emit("menu:new-session"))
file.AddSeparator()
file.AddText("Close Tab", keys.CmdOrCtrl("w"), emit("menu:close-tab"))
m.Append(menu.EditMenu())
view := m.AddSubmenu("View")
view.AddText("Hide Sidebar", keys.Combo("s", keys.CmdOrCtrlKey, keys.ControlKey), emit("menu:toggle-sidebar"))
m.Append(menu.WindowMenu())
help := m.AddSubmenu("Help")
help.AddText("Portty Help", nil, func(_ *menu.CallbackData) { runtime.BrowserOpenURL(appCtx, helpURL) })
help.AddText("Check for Updates", nil, checkForUpdatesCallback)
help.AddText("AgentHub on GitHub", nil, openGitHubCallback) // kept until Phase 3 (UI-SPEC)
```
- `runtime.Quit` → `Frontend.Quit()` → runs `OnBeforeClose` first `[VERIFIED: .../darwin/frontend.go:326-336]`. `app.beforeClose` emits `app:quit-requested` and returns true (app.go:218-230), so the Quit-confirm modal flow is preserved.
- `runtime.Hide` → `[NSApplication hide:]` `[VERIFIED: WailsContext.m:398-400]`. "Hide Others" and "Show All" have no Go API. Drop them (minor native-parity loss). Alternatively set `options.Mac.About` and keep the role, but then Settings… cannot sit in the app menu.
- `keys.Combo(key, modifier1, modifier2, rest...)` and the modifiers `CmdOrCtrlKey`, `ControlKey` are `[VERIFIED: .../pkg/menu/keys/keys.go:13-21, 97-103]`.
- **Today's File items are dead.** `fileMenu.AddText("New Session", keys.CmdOrCtrl("n"), nil)` and `AddText("Close Tab", keys.CmdOrCtrl("w"), nil)` have nil callbacks `[VERIFIED: main.go:104-106]`, and no frontend handler for ⌘N/⌘W exists (grep for `metaKey` finds only FindBar, TerminalPanel ⌘F/zoom and openLink). This phase wires them for the first time.
- The pattern for Go → React events already exists: tray emits `tray:focus-session` and App.tsx `EventsOn('tray:focus-session', …)`.
- The menu label "Hide Sidebar" is static in Wails v2 unless the app rebuilds the menu (`runtime.MenuSetApplicationMenu` + `MenuUpdateApplicationMenu`). Simplest: label it "Toggle Sidebar", or rebuild the menu when the state changes. The planner picks one and records it as a UI-SPEC deviation if the label differs.
- **About:** today there is no About item (`options.Mac.About` is unset). UI-SPEC lists "About". Options: a custom item calling `runtime.MessageDialog(InfoDialog)`, or leave it to Phase 3 (BRND-01 owns the About box). See Open Questions.

### Pattern 4: Live light/dark (THEM-01)

`TerminalPanel` already does the hard part `[VERIFIED: frontend/src/components/TerminalPanel.tsx:815-833]`. When active it sets `term.options.theme`, then `clearTextureAtlas()` and `fitTerminal()`. When hidden it stashes the theme in `pendingThemeRef` and applies it on activation (POL-04: atlas work on a `display:none` panel corrupts the WebGL atlas). So THEM-01 only needs a hook in App that feeds the same prop to every TerminalPanel (local tabs, peer tabs, web mode):

```ts
// lib/useColorScheme.ts
export function useColorScheme(): 'light' | 'dark' {
  const q = '(prefers-color-scheme: dark)'
  const [dark, setDark] = useState(() => window.matchMedia(q).matches)
  useEffect(() => {
    const mql = window.matchMedia(q)
    const on = (e: MediaQueryListEvent) => setDark(e.matches)
    mql.addEventListener('change', on)
    return () => mql.removeEventListener('change', on)
  }, [])
  return dark ? 'dark' : 'light'
}
// App: const theme = useColorScheme() === 'dark' ? DARK_PALETTE : LIGHT_PALETTE  (module constants → stable identity)
```
- Palette objects must be module-level constants. A new object each render would re-run the theme effect and `clearTextureAtlas` on every App render.
- Delete the `uiTheme` state, `data-ui-theme` attribute, `document.documentElement.style.colorScheme` writes (App.tsx:266-281), `THEME_STORAGE_KEY`/`DEFAULT_THEME_NAME`/`UI_THEME_STORAGE_KEY` (App.tsx:71-73), and the `index.html` inline `style="color-scheme: dark"`. Use `:root { color-scheme: light dark; }` in CSS (UI-SPEC).
- Replace the TerminalPanel inline `backgroundColor: theme.background ?? '#1a1b26'` (TerminalPanel.tsx:962) with a class using `var(--color-terminal-bg)`. UI-SPEC requires the token to equal the palette background.
- **jsdom has no `matchMedia`.** Add a polyfill to `src/test-setup.ts` (returns `matches:false` with `addEventListener`/`removeEventListener`). Otherwise every App render test crashes.
- Web share page: the same React bundle is served under `/app/`. `handleTerminalPage` 302-redirects `/sessions/{id}?cap=` to `/app/?session=…&cap=…` `[VERIFIED: internal/webserver/server.go:1240-1248]`, so the hook covers it with no extra work. `web/terminal.html` + `terminal.js` (hardcoded dark theme) are **unreachable** legacy kept only for Go asset/vendor tests. Leave them (out of scope) or delete them with their tests. Planner's call.
- `web/join.html`/`dashboard.html` use hardcoded Tokyo-Night CSS (`web/assets/join.css` 24 hex, `dashboard.css` 16 hex). They are part of the browser join flow. Recommend giving them the same two-block token treatment for consistency (small). See Open Questions.

### Pattern 5: One session list, always polled (NAV-01)

Today `hubSessions` is seeded at init, appended in `createTab`, filtered on `session:exit`, and **polled only while the Hub tab is active** (App.tsx `useEffect` keyed on `activeId === HUB_TAB.id`) or while the Share modal is open. Sessions created outside the GUI (CLI `new`) would never appear in the sidebar. Recommended:
- One effect: in desktop mode, `ListSessions()` every 3 s, unconditionally. This replaces both the Hub poll and the "CR-02 Share modal open" poll. The cost is one Unix-socket RPC per 3 s.
- Rename it `sessions`. It feeds sidebar rows, the "Shared" icon (`s.webEnabled`), the TabBar funnel icons, and the Share modal live-sync effect (already keyed on `hubSessions`).
- **Tab labels derive from `sessions`** (`sessions.find(s => s.id === tab.sessionId)?.name ?? tab.name`), so rename shows in both places in the same render with one source of truth. Peer tabs use `<host> › <name>` stored on the Tab at open time. Today `openWebSessionTab` hardcodes `name: 'Session'`, so pass host and name in.
- Remote peers: keep the 30 s `GetRemoteSessionsWithMeta` poll and drop `if (activeId !== HUB_TAB.id) return`. Keep the `tailscaleOff → setRemotePeers([])` branch. Use `remotePeers` directly for grouping, **not** `adaptAllRemoteSessions` (it filters out `!reachable`, but the sidebar must show "Can't reach this Mac").

### Pattern 6: Detach vs End Session (TERM-03)

Current code `[VERIFIED: frontend/src/App.tsx:776-787]`:
```
  const handleCloseTab = useCallback(async (id: string) => {
    // Disable web serving for this session before closing.
    if (webEnabled[id]) {
      try { await ToggleWebServing(id, false) } catch (_) { /* ignore */ }
    }
    try {
      await KillSession(id)
```
Required:
- **Close tab (×, ⌘W, menu)** = UI only. Remove the tab, pick the right neighbor, else the left, else null (the current `removeTabState` picks `idx-1`, so change that). **Do not** delete `webEnabled[id]` (the session is still shared). Better: drop the local `webEnabled` map and read `s.webEnabled` from `sessions`. Unmounting TerminalPanel closes its loopback WS. The relay host-authority arbiter then freezes the PTY grid at the last local size (hub.go `Origin == "local"` logic). That is existing behavior and harmless.
- **End Session…** = confirm dialog, then the old close body (`ToggleWebServing(false)` if shared, then `KillSession`). The Go exit watcher then reports the removed session (`TestCheckExits_ReportsRemovedWithoutStoppedSighting`, Phase 1), which removes the tab and row. Remove locally too for instant feedback.
- `KillSession` in the daemon removes the backend, manager, registry, tab name and workdir (engine.go:457-479). After chat removal the ChatStore delete block (lines 467-475) goes too.
- Launch keeps the Phase 1 behavior: every running session restores as a tab (UI-SPEC Interaction Contract).

### Pattern 7: Retiring chat wire frames (CHAT-01)

Frame constants `[VERIFIED: internal/relay/protocol.go:84-89]`:
```
	MsgChat     byte = 0x30 // server → client: deliver chat message (JSON ChatMessage)     [Phase 154 dispatch]
	MsgChatSend byte = 0x31 // client → server: send chat message (JSON content)            [Phase 154 dispatch]
	MsgPresence byte = 0x32 // server → client: full presence roster (JSON PresencePayload) [Phase 152]
	MsgTyping   byte = 0x33 // bidirectional: typing-start/stop (JSON TypingPayload)        [Phase 152]
	MsgAliasSet byte = 0x34 // client → server: set/update alias (JSON AliasPayload)        [Phase 152]
	MsgSelf     byte = 0x37 // server → client: self-identity hint on connect (JSON SelfPayload) [Phase 161]
```
Presence, typing, alias and self exist only for chat. Their only frontend consumers are `relayClient.ts` and `Hub/Chat*`/`MentionPopover`/`useChatUnreadListeners` (grep this session). Viewer count is the separate `MsgMeta` (0x20, `NotifyViewerCount`) and **stays**.

Mirror Phase 1: delete the constants, payload types and the `case MsgTyping/MsgAliasSet/MsgChatSend` arms in **both** read pumps (`internal/relay/server.go:323-365` and `internal/webserver/server.go:1472-1504`). Keep the "no default arm" property. Replace the constant block with a comment listing 0x30-0x37 as RETIRED, never to be reused. The frontend `parseServerFrame` already returns `{type:'unknown'}` for unrecognized bytes. Keep that, so an old AgentHub peer still sending 0x32/0x37 to a new client is ignored.

Proof test: extend the existing inject tests rather than adding files (no TESTING.md churn). In `internal/relay/server_inject_test.go` and `internal/webserver/inject_test.go`, send each of `0x30…0x37` with a JSON payload and assert zero PTY writes. Then send a `MsgInput` frame and assert it **does** reach the PTY (connection survived). The test helper currently wires `hub.SetChatAppendFn` (server_inject_test.go:46-60), so drop it.

### Anti-Patterns to Avoid
- **Keeping a "Hub" in disguise:** don't port SessionCardGrid/MiniPreview/HubFilterBar into the sidebar. The sidebar is a plain list.
- **Two sources of truth for names:** storing `name` on local Tab objects *and* in `sessions` is what makes rename drift. Derive the label instead.
- **Per-render palette objects:** these re-trigger `clearTextureAtlas` (Pattern 4).
- **Inheriting drag onto controls:** see Pattern 2.
- **Generic abstractions:** one `ContextMenu` (two real consumers) is fine. A generic "Dialog framework" is not needed beyond one shared sheet CSS class plus focus-trap logic. Check whether existing modals already trap focus and reuse that.
- **Leaving `components/Hub/` with moved files:** move the four survivors out, then delete the directory.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Live theme switch for hidden WebGL terminals | New theme pipeline | Existing `TerminalPanel` theme effect + `pendingThemeRef` | Already handles the zero-size atlas corruption (POL-04) |
| Native-looking selects/scrollbars | Custom dropdowns/scrollbars | `<select>` with `appearance:auto`, and delete the `::-webkit-scrollbar` rules | WKWebView draws native controls that follow `color-scheme` (UI-SPEC) |
| Window dragging / zoom | JS drag math | `--wails-draggable: drag` | Native drag via `WailsInvoke("drag")` |
| Peer attach | New attach flow | `handleOpenRemoteSession` + `RemoteJoinCodeModal` + `openWebSessionTab` | Capability handshake already correct (WR-01 SID protection) |
| Inline rename | New input logic | `InlineSessionName` (move from Hub/) | Enter/Esc/blur/empty-revert already tested (`InlineSessionName.test.tsx`) |
| App-wide dark detection in Go | Native appearance observer | CSS `prefers-color-scheme` | WKWebView follows the window's effective appearance |
| Opening URLs | `exec open` | `runtime.BrowserOpenURL` (Go) / `BrowserOpenURL` (wailsjs runtime) | Already used (main.go:122) |

**Key insight:** most of this phase's apparent new features (live theming, peer attach, rename, Share) already exist inside the Hub tree. The job is to lift them out and delete the rest. Every line kept from the Hub tree must earn its place.

## Hub Removal Blast Radius (App.tsx, verified line references)

| Item in App.tsx | Depends on Hub? | Disposition |
|-----------------|-----------------|-------------|
| `WELCOME_TAB`, `SETTINGS_TAB`, `HUB_TAB`, `HELP_TAB` consts (~:80-85) and `initialTabs` | yes | Delete. Initial tabs `[]`, `activeId` null until restore |
| `handleOpenSettings`, `handleOpenHelp`, `handleOpenHub`, `handleHome` | yes | Delete. Settings becomes `showSettings` sheet state. Help becomes the menu URL |
| Hub sessions poll (`activeId !== HUB_TAB.id`) and CR-02 modal poll | yes | Replace with one always-on poll (Pattern 5) |
| Remote peers poll gated on `HUB_TAB` | yes | Drop the gate (keep `tailscaleOff`) |
| `capAcquiredRef`, `capCancelledRef`, `'hub-modal'` intent in `handleModalExchange`/`RemoteJoinCodeModal` | HubPanel only | Delete the `hub-modal` branch. Keep `'open-session'`. Update `RemoteJoinCodeModal` intent type and its tests |
| `groupDefs`/`activeGroupId`/`groupCounts`/`globalGroupCounts` + handlers, `lib/hubGroups.ts`, `lib/hubGroupCounts.ts` | POL-05 | Delete (+ `hubGroups.test.ts`, `hubGroupCounts.test.ts`) |
| `stayOnHubAfterCreateRef` + `GetStayOnHubAfterCreate` | yes | Delete. Always activate a new tab. Go side: delete `GetStayOnHubAfterCreate/Set` (app.go:531-551), daemon routes (api.go:162-163, 832-845), engine field (engine.go:43,84,125,157,710-725), `engine_stayonhub_test.go`, `api_stayonhub_test.go`. Add the key to `settings_removed_test.go` / migration test (`stayOnHubAfterCreate` dropped on save) |
| `<HubPanel …/>` render branch | yes | Delete |
| `<WelcomeTab/>`, `<HelpTab/>` | — | Delete files + tests + `content/help/*.md` + `HelpSearch`, `HelpSectionNav`, `HelpContent`, `SettingsJumpBar`, `SettingsSearch` |
| `<StatusBar>` in each terminal wrapper + `openShareModalForActiveSession` | — | Delete StatusBar (+ `StatusBar.shareSession.test.tsx`). Toolbar Share button calls `openShareModalForActiveSession` (keep; derive from `sessions`) |
| `TerminalChatHost` wrapper | chat | Render `TerminalPanel` directly |
| Web-session render branch (`__websession__`) + `WebShareSessionView` | — | Keep; strip chat; move file out of Hub/ |
| `SessionShareModal` single instance at App level (`shareModalSession`) | already lifted (Phase 168-05) | Keep; move file; `onOpenHelp` → `BrowserOpenURL(helpURL)` or remove the link (Help tab gone) |
| Inline-styled `daemonError` block (hex colors) | — | Rewrite as a banner with tokens |
| `createTab` estimate subtracts `statusBarHeight = 32` | StatusBar | Remove the subtraction |
| `GetSessionStyledTailLines` (HubPanel MiniPreview only) | yes | Delete Go binding + daemon route if Hub-only. `GetSessionTailLines` already has no frontend consumer. Check the CLI before deleting the daemon route |

Survivors to move out of `components/Hub/` (Claude's discretion: target `components/Share/` and `components/`): `SessionShareModal.tsx`, `FunnelRiskPanel.tsx`, `ShareSegmentedControl.tsx`, `InlineSessionName.tsx` (+ test), `WebShareSessionView.tsx` (+ tests, chat stripped). Everything else in `components/Hub/` is deleted: Chat*, MentionPopover, TerminalChatHost, useChatUnreadListeners, HubPanel, HubModal, HubInteractiveModal, HubEmptyState, HubFilterBar, SessionCard, SessionCardGrid, MiniPreview, chatToggleOverlap.test.

## CHAT-01 Full Inventory

### Go (delete unless marked edit)
| File | Action |
|------|--------|
| `internal/daemon/chat.go`, `chat_routes.go`, `alias_store.go` (+ `chat_test.go`, `chat_routes_test.go`, `alias_store_test.go`) | Delete |
| `internal/daemon/engine.go` | Edit: drop `chatStores`, `chatsBaseDir`, `aliasStore`, `ChatsBaseDirForTest`, `ChatStoreFor`, `Aliases()`, alias reconstruction in `ConfigDirForTest`, `hub.SetChatAppendFn` wiring in CreateSession, ChatStore delete in KillSession |
| `internal/daemon/api.go` | Edit: drop `SetIdentityProviders` call (:293-305), `wrapRelayWithChat` (:311 returns `withRemote`), `setChatProviders` (:507-567), alias providers |
| `internal/daemon/engine_test.go`, `api_test.go`, `engine_migration_test.go`, `client.go` | Edit out chat references (103/11/4/11 hits) |
| `internal/relay/protocol.go` | Edit: delete 0x30-0x34, 0x37 constants, Presence/Typing/Alias/Self/ChatSend payloads, `ChatMessage`, `ChatSchemaVersion`, `ValidateAlias`, Make*Frame for them. Keep 0x01-0x12 and MsgMeta 0x20. Add RETIRED comment |
| `internal/relay/hub.go` | Edit: drop presence/typing rosters, `chatAppendFn`, `SetChatAppendFn`, `HandleChatSend`, `BroadcastChat`, `BroadcastPresence`, `RegisterPresence`, `UpdateAlias`, `UpdateTyping`, `NotifyPresence`. `Unsubscribe` returns nothing (signature change: update callers). Subscriber: drop `TailnetID`, `PersonKey`, `Alias`, `AliasSetFn`. **Keep `Origin`** |
| `internal/relay/server.go` | Edit: drop identity providers (:21-23, :47-67), alias/self emission (:217-265), typing/alias/chat-send arms, and the CORS helpers `isAllowedFilesOrigin`, `withCORS`, `handleFilesPreflight`, `FilesCORS`, `FilesPreflight` (chat-only consumers, verified with `git grep`). **Keep `loopbackOriginPatterns`** (WS origin check) |
| `internal/relay/sanitize.go` (+ `sanitize_test.go`) | Delete (`SanitizeChatContent` only) |
| `internal/relay/fuzz_test.go` | Edit: remove `FuzzSanitizeChatContent`. `FuzzParseFrame`, `FuzzInputAbsorberFilter` stay (STRP-04 relay coverage kept) |
| `internal/relay/hub_chatsend_test.go`, `hub_presence_test.go`, `protocol_chat_test.go`, `protocol_presence_test.go`, `server_chatsend_test.go`, `server_identity_test.go`, `server_cors_test.go`, `hub_subscribe_race_test.go` (all its tests are presence/identity) | Delete |
| `internal/relay/server_inject_test.go` | Edit: retired-frame test (Pattern 7) |
| `internal/relay/server_test.go`, `hub_test.go`, `scrollback_altscreen_test.go` | Edit stray refs (8/1/1 hits) |
| `internal/webserver/chat.go` (+ `chat_test.go`, `server_chatsend_test.go`, `identity_test.go`) | Delete |
| `internal/webserver/server.go` | Edit: drop `chatHistoryProvider/chatExportProvider/aliasGet/aliasSet` fields and setters, `/api/chat/{id}/history|export` routes (:892-900), WhoIs identity block (:1377-1408; `lc.WhoIs` is used only for this), `RegisterPresence`/`NotifyPresence`, self frame (:1419-1426), typing/alias/chat arms. Keep `NotifyViewerCount`. The "two-phase subscribe" comment block becomes moot; simplify to a single `hub.Subscribe` |
| `internal/webserver/inject_test.go` | Edit: retired-frame test |
| `internal/webserver/fuzz_test.go`, `server_test.go`, `session_ended_test.go` | Edit stray refs. `FuzzWebServerRoutes`/`FuzzRequireCapability` stay. Consider a negative route test (`/api/chat/x/history` → 404), mirroring `files_removed_test.go` |
| `cmd/playwright-fixture/main.go` | Edit: drop ChatStore seeding (:142-175) |
| `app.go` | Edit 12 refs (comments/bindings) |

### Frontend (delete unless marked)
`components/Hub/ChatPanel.tsx`, `ChatMessage.tsx`, `ChatBadge.tsx`, `ChatDaySeparator.tsx`, `MentionPopover.tsx`, `TerminalChatHost.tsx`, `useChatUnreadListeners.ts` + all their tests + `chatToggleOverlap.test.ts`. `lib/relayClient.ts` (edit: drop MSG_CHAT…MSG_SELF, PresenceEntry, ChatMessage, encode*Frame for chat, the `presence/typing/chat/self` union members; keep output/resize/input/ping). `relayClient.test.ts` (edit). `WebShareSessionView.tsx` (edit: drop ChatPanel/ChatBadge/ChatBubbleLeftRightIcon). `content/help/chat.md`. `e2e/chat-parity.spec.ts` (delete). `e2e/web-plugin-hot-swap.spec.ts`, `web-vendor-parity.spec.ts`, `web-share-scope.spec.ts` (edit comments/assertions). `TerminalPanel.scale.test.tsx`, `App.open-remote.test.tsx` (edit). style.css: `.chat-*`, `.mention-popover*` rules. localStorage `agenthub.chatPanelWidth` becomes orphaned (harmless).

## Runtime State Inventory

This phase deletes features and renames UI surfaces, so it counts as a refactor.

| Category | Items Found | Action Required |
|----------|-------------|-----------------|
| Stored data | `~/.config/agenthub/chats/*.jsonl` (per-session chat logs, `chatsDir()` in internal/daemon/chat.go:52), `~/.config/agenthub/aliases.json` (alias_store.go:47). localStorage: `agenthub:terminalTheme`, `agenthub:uiTheme`, `agenthub:hubGroups:v1`, `agenthub.chatPanelWidth` | **Leave in place** (inert after code removal; Phase 3 moves the config dir to `portty`, which orphans the whole old dir anyway). Optional one-liner: `localStorage.removeItem` for the 4 keys on desktop boot. No data migration |
| Live service config | None. The daemon serves chat routes only from code. Peers' `/api/sessions/meta` is unchanged | None. Verified via route list in server.go:829-1010 |
| OS-registered state | The tray icon is a template image (`setTemplate:YES`, tray_objc_darwin.m:120,155) and already adapts to light/dark. A running **daemon** process from the old build keeps serving chat until restarted | The daemon persists across GUI restarts (TERM-02). UAT must restart the daemon (`agenthub daemon` stop or reboot) before verifying CHAT-01. Add this to the manual checklist step |
| Secrets/env vars | None | None |
| Build artifacts | `frontend/dist/` (contains the old bundle incl. `agenthub-title-logo.png` reference), embedded via `-tags wailsassets` | Rebuild via `./build.sh`. Wails CLI v2.16.0 vs module v2.10.2 mismatch: build warns but works (Phase 1 built fine) |

`settings.json`: the `stayOnHubAfterCreate` key is ignored on load once the struct field is removed and dropped on the next save. `[VERIFIED: internal/daemon/engine.go:81-88]` shows `daemonSettings` fields `StartMinimized`, `StayOnHubAfterCreate`, `ShellWebShareWarned`, `ShellWebShareWarningEnabled`, `Plugins`, `SchemaVersion`. There is **no theme field**. The CONTEXT "legacy theme key in settings.json" is a no-op. Theme state is in localStorage `[VERIFIED: frontend/src/App.tsx:71-73 "const THEME_STORAGE_KEY = 'agenthub:terminalTheme'" … "const UI_THEME_STORAGE_KEY = 'agenthub:uiTheme'"]`. A plain save only rewrites when something triggers a save. Phase 1's `TestSettingsMigration_LegacyAgentHubV42` pattern asserts the drop.

## Common Pitfalls

### Pitfall 1: NSToolbar swallows clicks on macOS 26+ (HIGH impact, unverified here)
**What goes wrong:** With `TitleBarHiddenInset()` Wails attaches an `NSToolbar` (`UseToolbar: true`). Developers report that from macOS 26 an `NSGlassContainerView` inside `NSToolbarView` returns itself from `hitTest:`, so clicks never reach views under the toolbar region. The tab pills, "+", Share and the sidebar toggle live exactly there. `[CITED: developer.apple.com/forums/thread/788928, still present on 26.0.1, no Apple fix in thread]` This machine is on macOS 27.2.1 (`sw_vers`), and there is no evidence either way for 27.
**How to avoid:** Plan 1 is a spike. Apply the window options, put a test button in the top 52px and in the sidebar strip, build with `./build.sh --platform macos`, and click them. If clicks fail, switch to `mac.TitleBarHidden()` (same flags minus the toolbar). The traffic lights then sit at the standard position (about y=6-20). Keep the 52px strip and 80px reserve as in the UI-SPEC, record the vertical-centering deviation, and test again.
**Warning signs:** Buttons in the toolbar highlight nothing. Clicks start window drags instead.

### Pitfall 2: Whole-window opaque or whole-window vibrant
**What goes wrong:** With a transparent webview, any ancestor painting a background hides the vibrancy. Any opaque surface that forgets its token shows vibrancy. The `index.html` splash (`background: #1a1b26`, `style="color-scheme: dark"`) also flashes dark in Light mode.
**How to avoid:** `html, body, #root, .app { background: transparent }` in desktop mode (`html.is-desktop`, set in main.tsx from `detectMode()`). Sidebar `--color-sidebar: transparent` there. Toolbar, main, terminal, sheets and banners paint tokens explicitly. Rewrite the splash to token colors (or remove it) and drop the AgentHub logo (Phase 3 brand).

### Pitfall 3: Closing a tab kills the shell (current behavior)
See Pattern 6. Any test asserting `KillSession` on close (`TabBar.test.tsx`, `App.test.tsx`, `App.shellExit.test.tsx`) must be rewritten to assert the **opposite**. Add a positive test: closing a tab makes no `KillSession` or `ToggleWebServing` call, and the sidebar row stays.

### Pitfall 4: Deleting `Subscriber.Origin` with the identity fields
`Origin` sits next to the Phase 152 identity fields but is read by the host-authority resize arbiter (`hub.go:271, 299, 329, 340`: `if s.Origin == "web"`, `if sub.Origin != "local"`). Delete only `TailnetID`, `PersonKey`, `Alias`, `AliasSetFn` (Chesterton's Fence).

### Pitfall 5: Hand-maintained Wails bindings
Phase 1 recorded that `frontend/src/wailsjs/go/main/App.d.ts`, `App.js` and both `models.ts` copies are hand-edited. Removing Go bindings (`GetStayOnHubAfterCreate`, `SetStayOnHubAfterCreate`, `GetSessionStyledTailLines`) or adding them (`OpenTailscale` for the banner action, if done as a binding) needs a matching hand edit. Otherwise `tsc --noEmit` passes while runtime calls a missing binding. Also stub any new runtime calls (`BrowserOpenURL`) in `src/wailsjs/runtime/runtime.js` (the vitest alias target in vite.config.ts).

### Pitfall 6: Rename keeps the new name on RPC failure
`handleRenameTab` (App.tsx:789-798) logs the RPC error and **still** applies the name. UI-SPEC requires a revert. Also `engine.RenameSession` does no validation (engine.go:446-454). The UI trims and rejects empty or unchanged names, React renders the text safely, and no length cap exists. Keep it that way; don't add server validation now.

### Pitfall 7: Style source-gate tests break wholesale
`themeTokens.test.ts`, `style.contrast.test.ts`, `style.hub.test.ts`, `style.hub.modal.test.ts`, `style.redesign.test.ts`, `style.settings.test.ts`, `HelpTab.test.tsx`, `WelcomeTab.test.tsx`, `FindBar.themeMatrix/visual.test.tsx` and others read `style.css` or `themes.ts` source and pin old selectors and hex values. Replace the obsolete ones with **one** new gate (for example `src/__tests__/style.tokens.test.ts`) that asserts: (a) no hex/`rgb(`/named colors outside the `:root` and `@media (prefers-color-scheme: dark)` token blocks, (b) both blocks declare the same token names, (c) no `--hub-` declaration carries a literal color, (d) no `data-ui-theme`, (e) no `@font-face`. Pair it with a TSX gate (no `#hex` / `rgb(` in `style={{…}}` in `src/components/**` + `App.tsx`). Today's offenders: `SettingsTab.tsx:498 '#9ece6a'`, `SessionShare/shared.tsx:160-170`, `TerminalPanel.tsx:962`, App.tsx daemon-error block. This test *is* the LOOK-01 "no hardcoded colors" proof.

### Pitfall 8: Flaky `SessionShareModal.test.tsx:813-821`
It waits one `tick()` for async `cachedShare` state (Phase 1 VERIFICATION). Convert it to `await vi.waitFor(...)` when the file moves (UI-SPEC).

### Pitfall 9: Terminal font string
"SF Mono" by family name is not exposed to web content on macOS. The supported route is `ui-monospace` (Safari 13.1+). `[CITED: caniuse.com/extended-system-fonts; bugzil.la/1342741 via search]` xterm 6 measures cells with `OffscreenCanvas` `ctx.font` and the WebGL renderer rasterizes with canvas fonts, so measurement and drawing stay consistent even if canvas resolved `ui-monospace` differently. The DOM renderer uses CSS fonts. Use `'ui-monospace, Menlo, monospace'` (the UI-SPEC string with `"SF Mono"` in it is harmless but dead). **Verify early** in the built app's Web Inspector: compare `new OffscreenCanvas(1,1).getContext('2d')` with `font='13px ui-monospace'` against `'13px Menlo'` using `measureText('W').width`. Different widths mean ui-monospace resolved to SF Mono in canvas. Also check visually for glyph overlap with WebGL on and off. `[ASSUMED: canvas behavior in WKWebView]`
Also: `DEFAULT_FONT_SIZE = 14` (App.tsx:70) changes to 13 (UI-SPEC). Delete the `@font-face` JetBrains Mono / Plus Jakarta Sans rules and the woff2 files + `LICENSES.md`.

### Pitfall 10: Window key state for the inactive selection color
Wails forces the effect view Active, so native dimming won't happen. For the selected-row inactive color, toggle a class (for example `html.is-inactive`) on `window` `blur`/`focus`. App.tsx already listens to `window` `focus` for the Tailscale recheck. `[ASSUMED: WKWebView fires window blur/focus on NSWindow key changes]` Verify in the spike.

### Pitfall 11: Web guest scope
In web mode the sidebar must stay unrendered (Phase 159 WEBCHAT-03: guests must not reach desktop nav or the open `/api/sessions/meta` enumeration). Keep `mode !== 'web'` gates on Sidebar, Settings sheet, menus, End Session and rename (`webMode` prop). `e2e/web-share-scope.spec.ts` guards this, so keep it green.

### Pitfall 12: The daemon outlives the GUI
CHAT-01 UAT against a still-running old daemon will show chat routes alive. Restart the daemon after installing the new build.

## Code Examples

### Read-pump shape after retirement (both pumps)
```go
// Source: pattern of internal/relay/server.go:307-322 after removing chat arms
switch msgType {
case MsgInput:
    // unchanged (ReadOnly-gated, absorber)
case MsgResize2:
    // unchanged
case MsgPing:
    // keep-alive
// 0x30–0x37 are RETIRED (chat, presence, typing, alias, inject, self).
// No case and no default arm: old clients' frames are dropped.
}
```

### Sidebar peer-group state derivation (from UI-SPEC table)
```ts
// remotePeers: RemotePeerSessions[] (hostname, reachable, sessions)
type PeerView =
  | { kind: 'off' } | { kind: 'loading' } | { kind: 'none' }
  | { kind: 'groups'; groups: { host: string; reachable: boolean; sessions: RemoteSession[] }[] }
function peerView(tailscaleOff: boolean, loaded: boolean, peers: RemotePeerSessions[]): PeerView {
  if (tailscaleOff) return { kind: 'off' }
  if (!loaded) return { kind: 'loading' }
  const groups = peers
    .filter((p) => !p.reachable || p.sessions.length > 0)          // reachable+empty → hidden
    .map((p) => ({ host: p.hostname.split('.')[0], reachable: p.reachable, sessions: p.sessions }))
    .sort((a, b) => a.host.localeCompare(b.host))
  return groups.length ? { kind: 'groups', groups } : { kind: 'none' }
}
```
`remoteHasLoadedRef` (App.tsx) already exists for the `loaded` flag. Turn it into state so the sidebar re-renders when the first poll lands.

### Menu event wiring (frontend)
```ts
useEffect(() => {
  if (mode === 'web') return
  const offs = [
    EventsOn('menu:new-session', () => setShowNewSessionModal(true)),
    EventsOn('menu:close-tab', () => activeIdRef.current && closeTab(activeIdRef.current)), // no-op with no tabs
    EventsOn('menu:settings', () => setShowSettings(true)),
    EventsOn('menu:toggle-sidebar', () => setSidebarHidden((h) => !h)),
  ]
  return () => offs.forEach((off) => off())
}, [])
```
Use refs for `activeId` and other values read inside these handlers to avoid stale closures (App.tsx already uses `removeTabStateRef` for the same reason).

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| 138 xterm themes + dark/light toggle in localStorage | One palette per mode via `prefers-color-scheme` | This phase | Delete `themes.ts`, `xterm-theme`, picker UI, toggle |
| "SF Mono" by name | `ui-monospace` generic family | Safari 13.1 | Font string ordering |
| Tokyo-Night `--hub-*` dark tokens + `[data-ui-theme="light"]` overrides | Two-block `:root` / `@media (prefers-color-scheme: dark)` tokens | This phase | Rename `--hub-*` refs or alias them during migration. No `--hub-*` hex at phase end |

**Deprecated/outdated in repo:**
- `web/terminal.html` + `web/assets/terminal.js`: retired viewer since Phase 159 (redirect to `/app/`). Still embedded and tested. Not part of THEM-01.
- `ChatPanel isReadOnly`/inject: already removed (Phase 1).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The macOS 26 NSToolbar click-swallowing affects WKWebView content under a Wails `TitleBarHiddenInset` toolbar on macOS 27 | Pitfall 1 | If it applies and no spike is done, all toolbar controls are dead. If it doesn't apply, the spike costs ~30 min |
| A2 | WKWebView `prefers-color-scheme` updates live when the system appearance changes (window `Appearance` unset) | Pattern 4 | Live switching would need a native appearance observer bridge (deferred idea) |
| A3 | Canvas `ctx.font` accepts `ui-monospace` in WKWebView, or falls back consistently | Pitfall 9 | Glyph overlap in the DOM renderer. Fix by using `Menlo` |
| A4 | `window` blur/focus fire on NSWindow key changes in WKWebView | Pitfall 10 | The inactive selection color won't switch. Cosmetic |
| A5 | Wails's default (material-less) NSVisualEffectView looks acceptable as a sidebar | Pattern 1 | Needs ~15 lines of ObjC to set `NSVisualEffectMaterialSidebar` |
| A6 | `helpURL` should be `https://portty.app` (docs come in Phase 3) | Pattern 3 | The link may 404 until Phase 3. User may prefer the GitHub README |
| A7 | A 3 s always-on `ListSessions` poll is acceptable load | Pattern 5 | Negligible. Local Unix socket |

## Open Questions

1. **Spike outcome: HiddenInset vs Hidden.** We know the risk report is real for macOS 26. Nobody has tested 27 with Wails. Recommendation: Plan 1 Task 1 = spike plus a `checkpoint:human-verify` (the native click test cannot run headless). Record the result in the plan summary and the UI-SPEC deviation table.
2. **About item.** UI-SPEC lists "About". Today there is none. Recommend deferring the About box to Phase 3 (BRND-01) and noting it as a UI-SPEC deviation, or adding a minimal `runtime.MessageDialog`.
3. **"Hide Sidebar" / "Show Sidebar" label swap.** Wails v2 menu labels are static unless the app menu is rebuilt. Recommend "Toggle Sidebar", or rebuild the menu via `runtime.MenuUpdateApplicationMenu`. Planner decides.
4. **Help URL target before Phase 3 docs exist.** Recommend a single Go constant, `https://portty.app`.
5. **"Open Tailscale" banner action** needs `open -a Tailscale` with a URL fallback. This is a new Go binding with fixed argv (no user input). Alternative: just `BrowserOpenURL('https://tailscale.com/download/mac')`, which is simpler. UI-SPEC locks the `open -a` behavior, so implement as specced unless the planner flags it.
6. **join/dashboard pages light/dark.** Not strictly "the web share page". Cheap to tokenize. Recommend including them.
7. **Wails bump to v2.17.1.** Only if the spike or UAT hits macOS-26-class WebView crashes.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| macOS | Native chrome checks | ✓ | 27.2.1 (`sw_vers`) | — |
| Go | Build/test | ✓ | go1.27.4 | — |
| Wails CLI | `./build.sh` | ✓ | v2.16.0 (module v2.10.2) | — |
| Node / pnpm | Frontend | ✓ | v24.22.1 / 12.6.0 | — |
| Playwright browsers | e2e (chromium + webkit projects in playwright.config) | ✓ | chromium-1255, webkit-2336 | — |
| Tailscale (live tailnet + second Mac) | NAV-02 manual check | not probed | — | Manual checklist item (M-NN) |

**Missing dependencies with no fallback:** none for automated work. Live peer attach and the native look need human or second-machine verification.

## Validation Architecture

Skipped: `workflow.nyquist_validation` is `false` in `.planning/config.json`. Test strategy for the planner follows anyway (the orchestrator asked for it).

**Commands:** `go build ./... && go vet ./...` · `go test -race -short ./...` · `cd frontend && pnpm exec tsc --noEmit && pnpm exec vitest run` · `cd frontend && pnpm exec playwright test` (e2e) · `bash tests/check-traceability-paths.sh`.

**Vitest files to delete:** all `components/Hub/*.test.*` except `InlineSessionName.test.tsx` and the two `WebShareSessionView` tests (move them), `App.hub.test.tsx`, `App.createTab.stayOnHub.test.tsx`, `App.createTab.hubSessionsSeed.test.tsx` (or rewrite as a sessions-seed test), `SessionCard.share.test.tsx`, `StatusBar.shareSession.test.tsx`, `WelcomeTab*.test.tsx`, `Help*.test.tsx` (5 files), `SettingsTab.appearance-theme.test.tsx`, `SettingsTab.stay-on-hub-toggle.test.tsx`, `lib/hubGroups.test.ts`, `lib/hubGroupCounts.test.ts`, `__tests__/themeTokens.test.ts`, `style.hub*.test.ts`, `style.redesign.test.ts` (replaced by the token gate).

**Vitest files to rewrite:** `Sidebar.test.tsx`, `TabBar.test.tsx` (close = detach, no chevron, Share button / chip, `aria-disabled` + `aria-describedby`), `App.test.tsx`, `App.nav.test.tsx`, `App.tailscale.test.tsx`, `App.shellWebShare.test.tsx`, `App.open-remote.test.tsx` (peer row instead of Hub card), `App.dead-modal.test.tsx`, `App.saver.test.tsx`, `SettingsTab*.test.tsx` (sheet), `SessionShareModal*.test.tsx` (move + `vi.waitFor`), `RemoteJoinCodeModal.test.tsx` (×2 files, drop `hub-modal`), `ShellWebShareBanner.test.tsx`, `relayClient.test.ts`, `TerminalPanel.scale.test.tsx`, `FindBar.themeMatrix/visual`, `TerminalPanel.search.test.tsx` (theme refs), `style.contrast.test.ts`, `style.settings.test.ts`.

**New tests:** `lib/useColorScheme` + palette swap (TerminalPanel receives the new theme on a `change` event), `style.tokens.test.ts` (+ TSX inline-color gate), sidebar peer-state table, close-is-detach, End Session confirm → KillSession, rename revert on failure + both-places update, menu events → actions.

**Go:** delete listed chat/presence/stayOnHub tests, extend the inject tests into retired-frame tests, add a `/api/chat/*` → 404 negative test, and extend the settings migration test for `stayOnHubAfterCreate`. Playwright: delete `chat-parity.spec.ts`. Keep `web-share-scope.spec.ts` green. Optionally add a webkit-project check that the web page swaps the palette under `page.emulateMedia({ colorScheme: 'light' })`.

**TESTING.md:** Section 2 counts and a Phase 2 note. Section 4: remove rows for deleted files (the check script fails on dangling paths) and add rows for NAV/TERM-03/CHAT-01/THEM-01/LOOK-01 test files. Section 5: delete Categories H (Help), K (presence), L, M, Q, R, S (chat), rewrite B (Hub nav → sidebar peer attach), C (theme → live light/dark), and add M-NN items for vibrancy + inactive window, live appearance switch (WebGL and DOM renderers, tray glyphs), toolbar click-through (spike result), peer attach from the sidebar on a live tailnet, and the daemon restart before the CHAT-01 check. Section 3 only if `build.yml` matrix names change (they shouldn't).

## Security Domain

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no change | Capability tokens unchanged |
| V3 Session Management | no change | — |
| V4 Access Control | yes | Capability gating on `/sessions/{id}/ws` and `/api/*` unchanged. Web-guest scope: no sidebar or desktop nav in web mode (Pitfall 11). Closing a tab no longer revokes sharing (by design: Share state is explicit, and End Session still disables web serving before kill) |
| V5 Input Validation | yes | Rename input trimmed and rendered as React text. Retired frames dropped with no parsing. `open -a Tailscale` binding uses fixed argv only |
| V6 Cryptography | no | — |
| V14 Configuration | yes | CSP on `/app/` unchanged. No new inline scripts. Index.html inline `<style>` block already existed |

| Threat | STRIDE | Mitigation |
|--------|--------|-----------|
| Old client sends chat/presence frames after removal | Tampering/DoS | No case, no default arm, so frames are dropped. Tests prove zero PTY writes and the connection survives (Pattern 7) |
| Removing chat routes leaves a dangling CORS surface | Info disclosure | Delete `FilesCORS`/`FilesPreflight`/`isAllowedFilesOrigin` with their only consumers. WS origin check (`loopbackOriginPatterns`) untouched |
| Attack surface reduction | — | Removes WhoIs-on-connect, the alias persistence file and the chat JSONL write path from the remote-reachable webserver |
| User assumes closing a tab stops sharing | Info disclosure (user-intent) | Sidebar "Shared" icon + toolbar "Shared" state stay visible. End Session copy warns about attached viewers (UI-SPEC) |

## Sources

### Primary (HIGH confidence, read this session)
- Wails v2.10.2 module source: `pkg/options/mac/{mac.go,titlebar.go,appearance.go}`, `pkg/options/options.go`, `pkg/menu/keys/keys.go`, `internal/frontend/desktop/darwin/{WailsContext.m,WailsMenu.m,frontend.go,window.go}`, `internal/frontend/runtime/desktop/main.js`
- Repo: `main.go`, `app.go`, `frontend/src/App.tsx`, `TerminalPanel.tsx`, `TabBar.tsx`, `Sidebar.tsx`, `InlineSessionName.tsx`, `relayClient.ts`, `remoteAdapter.ts`, `internal/relay/{protocol,hub,server,sanitize}.go`, `internal/webserver/server.go`, `internal/daemon/{engine,api}.go`, `TESTING.md`, `tests/check-traceability-paths.sh`, `frontend/index.html`, `vite.config.ts`, `src/test-setup.ts`, `node_modules/@xterm/xterm` 6.0.0
- Planning: 02-CONTEXT.md, 02-UI-SPEC.md, REQUIREMENTS.md, STATE.md, ROADMAP.md, 01-VERIFICATION.md

### Secondary (MEDIUM)
- [Apple Developer Forums 788928: buttons behind NSToolbarView not clickable on macOS 26](https://developer.apple.com/forums/thread/788928)
- [Wails PR #4806: macOS 26 WebView crash fix](https://github.com/wailsapp/wails/pull/4806) (via search summary)
- [caniuse: ui-monospace](https://caniuse.com/extended-system-fonts); [Mozilla bug 1342741 (SF Mono not exposed to content)](https://bugzil.la/1342741)
- [Wails options reference](https://wails.io/docs/reference/options) (returned 403 to fetch; option semantics confirmed in module source instead)

### Tertiary (LOW)
- [Electron issue 39885 (hidden titlebar click regions)](https://github.com/electron/electron/issues/39885): analogous behavior, not Wails

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH. No new deps. Versions read from go.mod and node_modules
- Architecture / blast radius / chat inventory: HIGH. Read from source with line references
- Native chrome behavior on macOS 27: LOW to MEDIUM. Option semantics verified in source, runtime behavior needs the spike
- Pitfalls: HIGH for code pitfalls (3-8, 11, 12), MEDIUM/LOW for native ones (1, 9, 10)

**Research date:** 2026-11-15
**Valid until:** 2026-12-15 (codebase-specific; re-check if Wails is bumped)
