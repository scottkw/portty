# Roadmap: Portty

## Overview

Portty starts as a full AgentHub fork. The work strips it down to a plain macOS terminal that still builds and passes its tests, replaces the Hub grid with tabs plus a sidebar, renames everything to Portty, folds sharing into one action that reaches both tailnet peers and a browser link, and ends by proving the core value: a session attached from home survives an overnight idle gap with history intact. Every phase leaves a buildable, working app.

Ordering rationale: strip before rebrand so the rename (Go module path, strings, paths) touches the smallest possible tree; build navigation before rebrand so help and in-app text are rewritten once, against the final UI; rebrand before sharing and overnight work so those are verified on the final binary names, config paths, and daemon socket.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Plain macOS Terminal** - Remove agent-CLI, file browser, and Windows/Linux code; new sessions run the login shell; kept features still work (completed 2026-11-15)
- [ ] **Phase 2: Tabs + Sidebar Navigation** - Replace the Hub grid with tabs and a sidebar listing local and tailnet-peer sessions
- [ ] **Phase 3: Portty Brand** - Rename app, CLI, module, paths, icons, docs, notifications, and update feed to Portty
- [ ] **Phase 4: One-Click Share** - One Share action publishes to peers' sidebars and a tailnet web link; stop, Funnel, and CLI attach work
- [ ] **Phase 5: Overnight Survival** - Remote attaches survive 12+ hour idle gaps and the overnight end-to-end test passes
- [ ] **Phase 6: Release Updates** - Portty checks GitHub releases, notifies, supports a forced check, and downloads, verifies and reveals the DMG (GIFsmith flow)

## Phase Details

### Phase 1: Plain macOS Terminal
**Goal**: The app is a plain macOS terminal: new sessions run macOS zsh (`/bin/zsh -l`), no agent-CLI, file-browser, or Windows/Linux code remains, and every kept feature still works
**Depends on**: Nothing (first phase)
**Requirements**: STRP-01, STRP-02, STRP-03, STRP-04, TERM-01, TERM-02, TERM-04, TERM-05, NET-01
**Success Criteria** (what must be TRUE):
  1. User opens a new session, picks a directory (defaulting to the last-used one), and gets `/bin/zsh -l` there, with no shell choice anywhere and no Settings → Paths section; no agent picker, agent badges, agent working/waiting status, or file browser/editor appears in the desktop app or the web share view
  2. User quits and relaunches the app (or closes and reopens the window) and every running session is still there with its scrollback
  3. User changes the terminal theme and toggles xterm.js plugins (GPU render, search, links, sixel, OSC 52, OSC 9;4 progress) in Settings, and open terminals pick up the change
  4. ~~Chat parity~~ Superseded 2026-11-15: chat is removed entirely in Phase 2 (CHAT-01). Phase 1 removed the `@session` inject bridge; UAT confirmed browser chat never reaches the PTY
  5. On macOS the Go build, Go tests, Vitest, and lint all pass with no Windows/Linux sources, packaging, or CI jobs left; the daemon no longer serves a files API or files capability; the security fuzz harness still runs against relay and webserver
  6. When a local shell exits, its tab closes immediately, with no status dot, exit countdown, toast or banner, including for shells that exit hours later
  7. With Tailscale disconnected, a banner says remote access and sharing are off, and the Share and remote controls are disabled. Reconnecting clears the banner without a restart. Portty never installs or configures Tailscale
**Plans**: 12/12 plans complete (sequential waves 1-12; shared hot files and TESTING.md force serial order)

Plans:
**Wave 1**
- [x] 01-01-PLAN.md — Windows/Linux strip: Go sources, packaging, CI on macOS, portable traceability check

**Wave 2** *(blocked on Wave 1 completion)*
- [x] 01-02-PLAN.md — File browser/editor UI removed from desktop and web share (incl. CodeMirror, e2e)

**Wave 3** *(blocked on Wave 2 completion)*
- [x] 01-03-PLAN.md — Files API, files capability, browse/home-dir/filesWrite state removed; remote attach + chat CORS kept

**Wave 4** *(blocked on Wave 3 completion)*
- [x] 01-04-PLAN.md — New sessions are login shells end to end (`$SHELL -l`, `new [dir]` CLI, UI-SPEC New Session modal); OpenCode/theme broadcast removed

**Wave 5** *(blocked on Wave 4 completion)*
- [x] 01-05-PLAN.md — One shell resolver; agent-CLI detection and shell discovery removed; Settings → Paths = shell + tailscale

**Wave 6** *(blocked on Wave 5 completion)*
- [x] 01-06-PLAN.md — Status model running/exited/errored (daemon-derived) on tabs, exit toast and Hub; pill group removed

**Wave 7** *(blocked on Wave 6 completion)*
- [x] 01-07-PLAN.md — Agent identity removed from every UI surface; one-time web-share warning for any session ("Keep sharing off")

**Wave 8** *(blocked on Wave 7 completion)*
- [x] 01-08-PLAN.md — Agent identity removed from daemon, webserver, peer and CLI wire; explicit webserver 404 contract

**Wave 9** *(blocked on Wave 8 completion)*
- [x] 01-09-PLAN.md — Chat @session inject bridge removed on both paths; relay/webserver fuzz targets added

**Wave 10** *(blocked on Wave 9 completion)*
- [x] 01-10-PLAN.md — Shell exit closes its tab at any age (one app-wide exit watcher, no 300 s window); exit toast/countdown/Keep Open, auto-close toggle and all status UI (tab dot, Hub status/attention, briefing modal, quit-dialog status) removed

**Wave 11** *(blocked on Wave 10 completion)*
- [x] 01-11-PLAN.md — Every session is `/bin/zsh -l`; New Session shell line, Settings → Paths, shellPath/cliPaths plumbing and the awaiting-input notification toggle removed; legacy settings load and drop the keys

**Wave 12** *(blocked on Wave 11 completion)*
- [x] 01-12-PLAN.md — Tailscale detect-only (LocalAPI connected/not connected), NET-01 banner, Share/remote controls disabled with the same reason, focus recheck, brew install flow removed; final TESTING.md reconciliation and full phase gate
**UI hint**: yes

### Phase 2: Tabs + Sidebar Navigation
**Goal**: Users navigate everything from tabs plus a sidebar: the Hub grid is gone, and the sidebar lists local sessions and tailnet peers' sessions, each opening in a tab
**Depends on**: Phase 1
**Requirements**: NAV-01, NAV-02, NAV-03, TERM-03, CHAT-01, THEM-01, LOOK-01
**Success Criteria** (what must be TRUE):
  1. The app launches straight into tabs plus a sidebar; there is no Hub grid view or Hub entry anywhere
  2. The sidebar lists every local session; clicking one focuses its tab if it is open, or opens it in a new tab
  3. The sidebar lists sessions shared by other machines on the tailnet, grouped by machine; clicking one attaches to it in a tab
  4. User can run several sessions as tabs, rename a session (the new name shows in both its tab and the sidebar), and close it
  5. The Share dialog (including the Funnel risk panel) is still reachable from any session tab now that the Hub is gone
  6. Session chat is gone from the desktop app, the web share page, the daemon/webserver API and the relay protocol; no chat button, badge, or endpoint remains, and old clients that send chat frames do not crash anything
  7. Switching macOS between Light and Dark changes the whole app and every open terminal at once, with no restart; there is no theme picker, and the web share page follows the browser's light/dark setting
  8. The window looks like a native macOS app: system font and accent color, a source-list sidebar, a unified title bar/toolbar, and native-style controls; no custom dark chrome or hardcoded colors remain (LOOK-01)
**Plans**: 13/13 plans executed (sequential waves 1-13; App.tsx, style.css and TESTING.md are shared hot files)

Plans:
**Wave 1**
- [x] 02-01-PLAN.md — Native chrome spike: hidden-inset title bar, vibrancy, drag strip (human click-through check, Hidden fallback); menu bar with ⌘T, ⌘,, ⌘W, ⌃⌘S, Help ▸ Portty Help

**Wave 2** *(blocked on Wave 1 completion)*
- [x] 02-02-PLAN.md — CHAT-01 Go: retire 0x30-0x37 on both WebSocket paths; delete daemon/webserver chat store, routes, aliases, WhoIs identity, relay presence/sanitizer/chat CORS; fixture and fuzz updated

**Wave 3** *(blocked on Wave 2 completion)*
- [x] 02-03-PLAN.md — THEM-01: useColorScheme + two built-in palettes, live switch on desktop and web; two-block light/dark tokens; system fonts; theme library, picker and bundled fonts removed

**Wave 4** *(blocked on Wave 3 completion)*
- [x] 02-04-PLAN.md — Source-list sidebar: This Mac (always-on poll), peer groups with attach in '<host> › <session>' tabs, Tailscale footer, Hide/Show Sidebar; App stops rendering Hub/Welcome/Help

**Wave 5** *(blocked on Wave 4 completion)*
- [x] 02-05-PLAN.md — Share dialog moved to components/Share/; orphaned Hub grid, Welcome and Help deleted (frontend only); SM-02 flake fixed

**Wave 6** *(blocked on Wave 5 completion)*
- [x] 02-06-PLAN.md — Stay-on-Hub preference removed end to end (Settings, App, bindings, daemon); key dropped on save, migration test flipped, removed routes 404

**Wave 7** *(blocked on Wave 6 completion)*
- [x] 02-07-PLAN.md — CHAT-01 UI: chat components, frontend chat frames and chat-only deps removed; WebShareSessionView moved; components/Hub gone

**Wave 8** *(blocked on Wave 7 completion)*
- [x] 02-08-PLAN.md — Unified toolbar: pill tabs, close = detach, End Session… confirm, rename in tab and sidebar, context menu, empty state, toolbar Share with aria-described disabled state; status bar removed

**Wave 9** *(blocked on Wave 8 completion)*
- [x] 02-09-PLAN.md — Every dialog as a macOS sheet with a focus trap (useDialogFocus); Share dialog Done footer (Funnel kept)

**Wave 10** *(blocked on Wave 9 completion)*
- [x] 02-10-PLAN.md — Unified banners in UI-SPEC order; Open Tailscale action (fixed-argv OpenTailscale binding); no inline colours in App

**Wave 11** *(blocked on Wave 10 completion)*
- [x] 02-11-PLAN.md — Settings sheet (General / Sharing / Security), native switches, inline disabled reasons; Settings tab, jump bar and search removed

**Wave 12** *(blocked on Wave 11 completion)*
- [x] 02-12-PLAN.md — CSS sweep onto tokens with the LOOK-01 source gate; join/dashboard light/dark; final TESTING.md reconciliation and full phase gate

**Wave 13** *(blocked on Wave 12 completion)*
- [x] 02-13-PLAN.md — Appearance override: Settings ▸ Appearance Automatic / Light / Dark; native app appearance; settings.json key; startup apply; full gate re-run (added 2026-11-15, user request)
**UI hint**: yes

### Phase 3: Portty Brand
**Goal**: Everything a user sees or types says Portty: app, CLI, paths, icons, docs, notifications, and update feed
**Depends on**: Phase 2
**Requirements**: BRND-01, BRND-02, BRND-03, BRND-04, BRND-05, BRND-06, NOTF-01, DIST-03
**Success Criteria** (what must be TRUE):
  1. The built app is Portty.app with a Portty bundle ID, and the window title, menu bar, About box, and tray tooltip all say Portty
  2. The app icon, tray icons (progress quartiles and error state), and title logos (light and dark) are the `portty-brand/` assets
  3. The CLI is `portty` (`portty daemon`, `portty attach`), the Go module path is portty, and Portty keeps its config, data, and daemon socket in portty-named locations, never reading or writing AgentHub's
  4. The README, Help tab, Settings, and other user-facing text contain no "AgentHub" or agent-CLI wording and describe Portty as a terminal with remote access
  5. Native macOS notifications appear under the Portty name, and the update check queries Portty's release feed and handles "no release published yet" without an error
  6. The root `LICENSE` is still MIT, and README plus `frontend/package.json` declare the MIT license
  7. `packaging/homebrew` is gone, and no docs, scripts or workflows mention Homebrew
  8. A case-insensitive search for "agenthub" over tracked files (excluding `.planning/`) returns zero hits, and every website/repo link points to https://portty.app or github.com/scottkw/portty
**Plans**: 6/6 plans executed (sequential waves 1-6; the module rename must land first, and tray_objc_darwin.m, app.go and TESTING.md are shared hot files)

Plans:
**Wave 1**
- [x] 03-01-PLAN.md — Go module github.com/scottkw/portty, config/socket under ~/Library/Application Support/portty, `portty` CLI text and unknown-command message, portty-daemon service, renamed legacy fixture

**Wave 2** *(blocked on Wave 1 completion)*
- [x] 03-02-PLAN.md — Port 7444, Portty-only `X-Portty: 1` probe marker on GET /api/sessions, update feed scottkw/portty with DMG asset filter (silent when no release)

**Wave 3** *(blocked on Wave 2 completion)*
- [x] 03-03-PLAN.md — Brand assets from portty-brand/: Retina (@2x) template tray icons, app icon, iconset + iconutil icns, title logos; generators and portty-brand/ deleted; sips-based verify-icons.sh

**Wave 4** *(blocked on Wave 3 completion)*
- [x] 03-04-PLAN.md — Portty.app with bundle ID app.portty.Portty, window title and Portty menus, native About Portty panel, tray text and 0/1/n quit notification

**Wave 5** *(blocked on Wave 4 completion)*
- [x] 03-05-PLAN.md — Web pages, local-mode realm/cert, Quit dialog, document title and portty:lastWorkDir key; no source file carries the old name

**Wave 6** *(blocked on Wave 5 completion)*
- [x] 03-06-PLAN.md — Homebrew removed (hdiutil DMG), Portty release/CI names, brand gate script in CI, README/CHANGELOG rewrite, MIT metadata, TESTING.md reconciliation, zero-hit gate and full phase gate
**UI hint**: yes

### Phase 4: One-Click Share
**Goal**: Sharing is one on/off switch per session: shared means tailnet peers and a tailnet web link at once, always full control, with a clean stop and CLI attach from any terminal. Funnel and read-only links are gone
**Depends on**: Phase 3
**Requirements**: SHAR-01, SHAR-02, SHAR-03, SHAR-04, SHAR-05, SHAR-06
**Success Criteria** (what must be TRUE):
  1. User turns Share on for a session. In that one step the session shows up in the other Mac's Portty sidebar and a tailnet-only web link appears, ready to copy. There are no share tabs, modes, or separate peer/web toggles
  2. Opening the link in a browser on another tailnet device gives a working interactive terminal with full control
  3. User turns Share off, every remote viewer (peer GUI, browser, CLI) is disconnected, and the session disappears from peers' sidebars
  4. No Funnel, public-link, risk-panel, or read-only code, UI, settings, or tests remain
  5. From Warp or any terminal, `portty attach <host>:<session>` attaches to a session shared from another Mac and shows the tmux-style status bar
**Plans**: 1/6 plans executed (sequential waves 1-6; api.go, server.go, app.go, App.tsx, the Wails stubs and TESTING.md are shared hot files)

Plans:
**Wave 1**
- [x] 04-01-PLAN.md — Go removal: Funnel and the public-write gate, join codes and QR (go-qrcode out), read-only link features and the one-time share warning; removed routes pinned with 404 tests

**Wave 2** *(blocked on Wave 1 completion)*
- [ ] 04-02-PLAN.md — Share is one daemon switch: GET/PUT /sessions/{id}/share, server starts on first share and stops when none, `portty share`/`unshare`, LAN fallback and boot auto-start removed, GetSessionShare/SetSessionShared bindings, vmpreview build for the VM

**Wave 3** *(blocked on Wave 2 completion)*
- [ ] 04-03-PLAN.md — Same-account WhoIs gate on discovery and a peer-capability route; share off revokes every viewer with `share stopped`; "Link No Longer Works" page; Referrer-Policy

**Wave 4** *(blocked on Wave 3 completion)*
- [ ] 04-04-PLAN.md — POST /peers/connect for GUI and CLI, proxy forwards close reasons, `portty attach <host>[:<session>]` with UI-SPEC copy, `portty list` fixed, ConnectPeerSession binding, old remote-cap routes removed

**Wave 5** *(blocked on Wave 4 completion)*
- [ ] 04-05-PLAN.md — Share popover with one switch, sidebar Share / Stop Sharing, Settings ▸ Sharing shrunk; share dialog, risk panel, tabs, warning banner, web-server settings and stale stubs deleted

**Wave 6** *(blocked on Wave 5 completion)*
- [ ] 04-06-PLAN.md — Codeless peer tabs with stopped-sharing and connect-failed banners, web "Link No Longer Works" and stopped banner, join dialog deleted, SC4 removal gate in CI, docs, full phase gate
**UI hint**: yes

### Phase 5: Overnight Survival
**Goal**: A session attached from another Mac or a browser survives an overnight idle gap and picks up where it left off, proving the core value end to end
**Depends on**: Phase 4
**Requirements**: SHAR-07, VERF-01
**Success Criteria** (what must be TRUE):
  1. A session attached from the home Mac's Portty GUI and left idle for 12+ hours is still connected, or has reconnected on its own after a dropped connection, and accepts typing without a manual reattach
  2. `portty attach` from a terminal behaves the same way: after 12+ idle hours it is still attached or has reconnected on its own
  3. Detaching and reattaching from the GUI, the CLI, or a browser restores the session's scrollback
  4. Overnight test: Claude Code runs in a shared shell on the office Mac and the home Mac attaches from the Portty GUI. After both sit idle overnight, the user keeps typing the next morning and the previous day's scrollback is intact
  5. The next morning the same session also opens from a browser on the tailnet and works
**Plans**: TBD

### Phase 6: Release Updates
**Goal**: Installed copies of Portty find out about new GitHub releases on their own or on demand, and the user can download, verify and open an update in a couple of clicks, using the same flow as GIFsmith
**Depends on**: Phase 3
**Requirements**: UPD-01, UPD-02, UPD-03, UPD-04
**Success Criteria** (what must be TRUE):
  1. With a newer release published (or a fake GitHub API serving one), Portty shows an update notice and a native notification shortly after launch, and again on its periodic check, without the user doing anything
  2. "Check for Updates…" runs a check immediately and tells the user they are up to date, that version X is available, or what went wrong
  3. Clicking Download in the notice fetches the release DMG, rejects it if the SHA-256 digest or Developer ID Team signature doesn't match, and otherwise reveals the verified DMG in Finder
  4. With no release published, or the repo returning 404, Portty reports "up to date" or stays silent, never a false update or repeated error popups
**Plans**: TBD
**UI hint**: yes

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5 → 6

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Plain macOS Terminal | 12/12 | Complete    | 2026-11-15 |
| 2. Tabs + Sidebar Navigation | 13/13 | In Progress | - |
| 3. Portty Brand | 6/6 | In Progress | - |
| 4. One-Click Share | 1/6 | In Progress | - |
| 5. Overnight Survival | 0/TBD | Not started | - |
| 6. Release Updates | 0/TBD | Not started | - |
