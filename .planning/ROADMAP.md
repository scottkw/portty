# Roadmap: Portty

## Overview

Portty starts as a full AgentHub fork. The work strips it down to a plain macOS terminal that still builds and passes its tests, replaces the Hub grid with tabs plus a sidebar, renames everything to Portty, folds sharing into one action that reaches both tailnet peers and a browser link, and ends by proving the core value: a session attached from home survives an overnight idle gap with history intact. Every phase leaves a buildable, working app.

Ordering rationale: strip before rebrand so the rename (Go module path, strings, paths) touches the smallest possible tree; build navigation before rebrand so help and in-app text are rewritten once, against the final UI; rebrand before sharing and overnight work so those are verified on the final binary names, config paths, and daemon socket.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [ ] **Phase 1: Plain macOS Terminal** - Remove agent-CLI, file browser, and Windows/Linux code; new sessions run the login shell; kept features still work
- [ ] **Phase 2: Tabs + Sidebar Navigation** - Replace the Hub grid with tabs and a sidebar listing local and tailnet-peer sessions
- [ ] **Phase 3: Portty Brand** - Rename app, CLI, module, paths, icons, docs, notifications, and update feed to Portty
- [ ] **Phase 4: One-Click Share** - One Share action publishes to peers' sidebars and a tailnet web link; stop, Funnel, and CLI attach work
- [ ] **Phase 5: Overnight Survival** - Remote attaches survive 12+ hour idle gaps and the overnight end-to-end test passes

## Phase Details

### Phase 1: Plain macOS Terminal
**Goal**: The app is a plain macOS terminal: new sessions run the user's login shell, no agent-CLI, file-browser, or Windows/Linux code remains, and every kept feature still works
**Mode:** mvp
**Depends on**: Nothing (first phase)
**Requirements**: STRP-01, STRP-02, STRP-03, STRP-04, TERM-01, TERM-02, TERM-04, CHAT-01
**Success Criteria** (what must be TRUE):
  1. User opens a new session, picks a directory (defaulting to the last-used one), and gets their login shell there; no agent picker, agent badges, agent working/waiting status, or file browser/editor appears in the desktop app or the web share view
  2. User quits and relaunches the app (or closes and reopens the window) and every running session is still there with its scrollback
  3. User changes the terminal theme and toggles xterm.js plugins (GPU render, search, links, sixel, OSC 52, OSC 9;4 progress) in Settings, and open terminals pick up the change
  4. User chats on a session from the desktop app and from a browser web share, and both sides see the messages; nothing typed in chat (including `@session`) is ever written into the terminal
  5. On macOS the Go build, Go tests, Vitest, and lint all pass with no Windows/Linux sources, packaging, or CI jobs left; the daemon no longer serves a files API or files capability; the security fuzz harness still runs against relay and webserver
**Plans**: 1/10 plans executed (sequential waves 1-10; shared hot files and TESTING.md force serial order)

Plans:
**Wave 1**
- [x] 01-01-PLAN.md — Windows/Linux strip: Go sources, packaging, CI on macOS, portable traceability check

**Wave 2** *(blocked on Wave 1 completion)*
- [ ] 01-02-PLAN.md — File browser/editor UI removed from desktop and web share (incl. CodeMirror, e2e)

**Wave 3** *(blocked on Wave 2 completion)*
- [ ] 01-03-PLAN.md — Files API, files capability, browse/home-dir/filesWrite state removed; remote attach + chat CORS kept

**Wave 4** *(blocked on Wave 3 completion)*
- [ ] 01-04-PLAN.md — New sessions are login shells end to end (`$SHELL -l`, `new [dir]` CLI, UI-SPEC New Session modal); OpenCode/theme broadcast removed

**Wave 5** *(blocked on Wave 4 completion)*
- [ ] 01-05-PLAN.md — One shell resolver; agent-CLI detection and shell discovery removed; Settings → Paths = shell + tailscale

**Wave 6** *(blocked on Wave 5 completion)*
- [ ] 01-06-PLAN.md — Status model running/exited/errored (daemon-derived) on tabs, exit toast and Hub; pill group removed

**Wave 7** *(blocked on Wave 6 completion)*
- [ ] 01-07-PLAN.md — Agent identity removed from every UI surface; one-time web-share warning for any session ("Keep sharing off")

**Wave 8** *(blocked on Wave 7 completion)*
- [ ] 01-08-PLAN.md — Agent identity removed from daemon, webserver, peer and CLI wire; explicit webserver 404 contract

**Wave 9** *(blocked on Wave 8 completion)*
- [ ] 01-09-PLAN.md — Chat @session inject bridge removed on both paths; relay/webserver fuzz targets added

**Wave 10** *(blocked on Wave 9 completion)*
- [ ] 01-10-PLAN.md — Bell/OSC 9 notifications, exit events for long-lived/relaunched sessions, Settings copy, full phase gate
**UI hint**: yes

### Phase 2: Tabs + Sidebar Navigation
**Goal**: Users navigate everything from tabs plus a sidebar: the Hub grid is gone, and the sidebar lists local sessions and tailnet peers' sessions, each opening in a tab
**Mode:** mvp
**Depends on**: Phase 1
**Requirements**: NAV-01, NAV-02, NAV-03, TERM-03
**Success Criteria** (what must be TRUE):
  1. The app launches straight into tabs plus a sidebar; there is no Hub grid view or Hub entry anywhere
  2. The sidebar lists every local session; clicking one focuses its tab if it is open, or opens it in a new tab
  3. The sidebar lists sessions shared by other machines on the tailnet, grouped by machine; clicking one attaches to it in a tab
  4. User can run several sessions as tabs, rename a session (the new name shows in both its tab and the sidebar), and close it
  5. Session chat and the Share dialog (including the Funnel risk panel) are still reachable from any session tab now that the Hub is gone
**Plans**: TBD
**UI hint**: yes

### Phase 3: Portty Brand
**Goal**: Everything a user sees or types says Portty: app, CLI, paths, icons, docs, notifications, and update feed
**Mode:** mvp
**Depends on**: Phase 2
**Requirements**: BRND-01, BRND-02, BRND-03, BRND-04, BRND-05, BRND-06, NOTF-01
**Success Criteria** (what must be TRUE):
  1. The built app is Portty.app with a Portty bundle ID, and the window title, menu bar, About box, and tray tooltip all say Portty
  2. The app icon, tray icons (progress quartiles and error state), and title logos (light and dark) are the `portty-brand/` assets
  3. The CLI is `portty` (`portty daemon`, `portty attach`), the Go module path is portty, and Portty keeps its config, data, and daemon socket in portty-named locations, never reading or writing AgentHub's
  4. The README, Help tab, Settings, and other user-facing text contain no "AgentHub" or agent-CLI wording and describe Portty as a terminal with remote access
  5. Native macOS notifications appear under the Portty name, and the update check queries Portty's release feed and handles "no release published yet" without an error
  6. A case-insensitive search for "agenthub" over tracked files (excluding `.planning/`) returns zero hits, and every website/repo link points to https://portty.app or github.com/scottkw/portty
**Plans**: TBD
**UI hint**: yes

### Phase 4: One-Click Share
**Goal**: One Share action publishes a session to tailnet peers and a tailnet web link at the same time, with capability-gated access, a clean stop, Funnel as a separate step, and CLI attach from any terminal
**Mode:** mvp
**Depends on**: Phase 3
**Requirements**: SHAR-01, SHAR-02, SHAR-03, SHAR-04, SHAR-05, SHAR-06
**Success Criteria** (what must be TRUE):
  1. User clicks Share on a session and picks read-only or read/write. In that one step the session shows up in the other Mac's Portty sidebar and a tailnet-only web link appears, ready to copy, with no separate peer and web toggles
  2. Opening the link in a browser on another tailnet device gives a working interactive terminal; with a read-only share, browser and peer viewers can watch but their keystrokes never reach the shell
  3. User stops sharing, every remote viewer (peer GUI, browser, CLI) is disconnected, and the session disappears from peers' sidebars
  4. Public Funnel sharing is available only as a separate step from the Share dialog, behind the risk gate. It auto-expires, and read/write requires explicit consent
  5. From Warp or any terminal, `portty attach <host>:<session>` attaches to a session shared from another Mac and shows the tmux-style status bar
**Plans**: TBD
**UI hint**: yes

### Phase 5: Overnight Survival
**Goal**: A session attached from another Mac or a browser survives an overnight idle gap and picks up where it left off, proving the core value end to end
**Mode:** mvp
**Depends on**: Phase 4
**Requirements**: SHAR-07, VERF-01
**Success Criteria** (what must be TRUE):
  1. A session attached from the home Mac's Portty GUI and left idle for 12+ hours is still connected, or has reconnected on its own after a dropped connection, and accepts typing without a manual reattach
  2. `portty attach` from a terminal behaves the same way: after 12+ idle hours it is still attached or has reconnected on its own
  3. Detaching and reattaching from the GUI, the CLI, or a browser restores the session's scrollback
  4. Overnight test: Claude Code runs in a shared shell on the office Mac and the home Mac attaches from the Portty GUI. After both sit idle overnight, the user keeps typing the next morning and the previous day's scrollback is intact
  5. The next morning the same session also opens from a browser on the tailnet and works
**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Plain macOS Terminal | 1/10 | In Progress | - |
| 2. Tabs + Sidebar Navigation | 0/TBD | Not started | - |
| 3. Portty Brand | 0/TBD | Not started | - |
| 4. One-Click Share | 0/TBD | Not started | - |
| 5. Overnight Survival | 0/TBD | Not started | - |
