# Phase 2: Tabs + Sidebar Navigation - Context

**Gathered:** 2026-11-15
**Status:** Ready for planning

<domain>
## Phase Boundary

Replace the Hub grid with a macOS-native main window: a source-list sidebar (local sessions plus one group per tailnet peer machine) and sessions as tabs in a unified toolbar. Restyle the whole desktop GUI to look like a native macOS app (LOOK-01), switch the app, every terminal and the web share page to system light/dark (THEM-01), and remove session chat everywhere (CHAT-01). Requirements: NAV-01, NAV-02, NAV-03, TERM-03, CHAT-01, THEM-01, LOOK-01.

Not in this phase: the one-switch Share popover and Funnel/read-only removal (Phase 4), the rename to Portty and help/doc text (Phase 3).

</domain>

<decisions>
## Implementation Decisions

### Window & Chrome (LOOK-01)
- Hidden-inset title bar (Wails `mac.TitleBarHiddenInset()`): traffic lights sit in the sidebar's top strip, session tabs live in a unified toolbar to the right, as in the mockup.
- Native vibrancy behind the sidebar only (Wails translucent window + transparent webview where the sidebar is). The terminal and toolbar stay opaque (readability, WebGL renderer).
- System font (`-apple-system`) and SF Mono for terminals. Every color is a CSS variable with one light and one dark set, accent = macOS blue. No hardcoded hex left in components or style.css (fixes UI-REVIEW 01's banner and inline-color findings).
- Icons: keep Heroicons outline (already a dependency), sized and weighted to look like SF Symbols.

### Sidebar & Tabs (NAV-01..03, TERM-03)
- Sidebar: "This Mac" group with every local session, then one group per peer machine listing that peer's shared sessions, then a Tailscale status footer (dot + "connected · <host>" / "not connected"). No Home, Hub, Help, Settings or custom-group items.
- Clicking a local session focuses its tab or opens one; clicking a peer session attaches to it in a tab (replaces the Hub remote cards, so GUI remote attach never regresses).
- Closing a tab only detaches: the shell keeps running in the daemon and stays in the sidebar. Right-click "End Session…" (with a confirm) kills it.
- Rename: double-click the name in the sidebar row or the tab for inline edit, plus a right-click "Rename" item. The new name shows in both places.
- No tabs open: a quiet empty state with a "New Session" button (⌘T). The Welcome tab is removed. App launches straight into sidebar + tabs (no Hub anywhere).
- When Tailscale is off, the peer groups show one muted line ("Peers appear when Tailscale is connected") and Share is disabled with the NET-01 reason visible on screen (not tooltip-only; UI-REVIEW 01 fix #2/#3: stable accessible name, reason via `aria-describedby`).

### Settings, Help, Share & Status Bar
- Settings opens as a sheet over the main window (⌘, in the app menu), styled like the mockup's Settings window with General / Sharing / Security panes in macOS grouped-form style. (Wails v2 is single-window, so a sheet, not a second window.) The theme pickers go away. Appearance is a three-way control: Automatic (default, follows System Settings) / Light / Dark. **Amended 2026-11-15 by user: "The light/dark mode should follow macOS, but the user should have the ability to override it with an option in Settings."** The override sets the native app appearance (so CSS `prefers-color-scheme`, the xterm palette hook and sidebar vibrancy all follow it), is stored in settings.json (new `appearance` key: system|light|dark) so it applies before the window first paints, and applies live. The web share page keeps following the browser.
- Help: the in-app Help tab and help search are removed. Help ▸ Portty Help opens the docs site in the browser (portty.app docs are written in Phase 3).
- Share in Phase 2: a toolbar Share button opens the existing Share dialog, restyled to the new look, with the Funnel risk panel still reachable (ROADMAP SC5). The one-switch popover from the mockup is Phase 4.
- The bottom status bar is removed. Share moves to the toolbar; Tailscale status moves to the sidebar footer.

### Light/Dark, Chat & Cuts (THEM-01, CHAT-01)
- Terminals: one built-in dark ANSI palette and one built-in light palette, tuned to the new chrome. Open terminals and the app switch live on `prefers-color-scheme` change, no restart. The web share page follows the browser the same way with the same palettes.
- The 138-theme library, theme picker and its settings key are removed. A legacy `theme` key in settings.json is ignored on load and dropped on the next save (Phase 1 pattern).
- Chat is removed everywhere: desktop and web chat UI, daemon/webserver chat API and store, relay chat frames, FuzzSanitizeChatContent, chat CORS helpers (FilesCORS naming). Old clients' chat frames are dropped without crashing anything, mirroring how Phase 1 retired the 0x35/0x36 inject frames.
- Custom session groups (POL-05: create/rename/drag-drop groups) are cut; the sidebar groups by machine only.
- Kept as-is (user did not select them for cutting): plugin toggles, OSC 9;4 progress glyphs, sixel.

### Claude's Discretion
- Exact palette values (must pass 4.5:1 text contrast in both modes; selected sidebar row uses a darker blue fill with white text, as in the mockup).
- How the Share dialog, Funnel risk panel and web-share session view are relocated out of `components/Hub/` (move, not delete).
- Plan order and granularity, as long as every plan leaves build and tests green.

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `frontend/src/components/Sidebar.tsx` (310 lines) and `TabBar.tsx` (331 lines): current nav (Home/Hub/Settings/Help + POL-05 groups) and tab strip; reshape rather than rewrite where it helps.
- `components/Hub/SessionShareModal.tsx`, `FunnelRiskPanel.tsx`, `ShareSegmentedControl.tsx`, `components/SessionShare/*`, `WebShareSessionView.tsx`: share UI and web-share view that must survive Hub removal.
- `components/Hub/InlineSessionName.tsx`: existing inline rename, reusable for sidebar/tab rename.
- `frontend/src/lib/tailscale.ts` `TAILSCALE_OFF_REASON` and `TailscaleBanner.tsx`: NET-01 sentence and banner from Phase 1.
- Remote peer polling and adapters (`AdaptedRemoteSessionInfo`, `RemoteSession` in `frontend/src/lib/`) feed the new peer groups.

### Established Patterns
- `App.tsx` (1,662 lines) owns tabs, remote-peer polling (currently only while the Hub tab is active — must run whenever the sidebar shows peers and Tailscale is connected), Tailscale health, and modals.
- `style.css` (5,722 lines) holds most styling with Tokyo-Night-style hex values; theme tokens are partly CSS variables (`--hub-*`).
- Wails window options in `main.go` set only a dark `BackgroundColour`; no `options.Mac` block yet.
- Legacy settings keys are ignored by `json.Unmarshal` and dropped on save (Phase 1 01-10/01-11).
- Retired relay frames: read pumps have no case and no default arm, so old frames are dropped (Phase 1 01-09).

### Integration Points
- `main.go` (window options: title bar, translucency, appearance), `App.tsx` (layout root), `style.css` (tokens), Wails app menu (⌘T, ⌘,, Help menu).
- Daemon/webserver/relay chat routes and store (`internal/daemon/chat.go` `ChatStore`, relay chat frames) for CHAT-01.
- Web share page entry (web mode in `App.tsx`) for browser light/dark.
- TESTING.md must be updated for every added/removed test file (standing rule).

</code_context>

<specifics>
## Specific Ideas

- Visual baseline: the approved mockup https://claude.ai/artifact/E5PxYpFTubbmZTifxfjYsz (user: "The redesign looks great"). Artboards: Main window dark/Tailscale on, Main window light/Tailscale off, Settings dark. Planner/UI-SPEC should follow its layout: 248px sidebar, 52px toolbar, tabs as rounded pills in the toolbar, Share button at the toolbar's right edge, terminal below, Tailscale footer in the sidebar.
- Peer tabs are labeled "<host> › <session>"; a remote session's toolbar shows an "On <host>" chip instead of Share.
- Phase 1 open items landing here: flaky `SessionShareModal.test.tsx:813-821` (use `vi.waitFor`); UI-REVIEW 01 findings (banner hex colors, ⚠ glyph, off-scale spacing, disabled-reason tooltips).

</specifics>

<deferred>
## Deferred Ideas

- One-switch Share popover (on/off, web link, `portty attach` line) from the mockup — Phase 4.
- Follow the user's chosen macOS accent color live (needs a native bridge) — not planned.
- Remaining cut candidates not chosen now: plugin toggles, OSC 9;4 glyphs, sixel, key regen + CT notice, extra CLI commands, launchd install, tray menu, join codes/QR — revisit in later phases.

</deferred>
