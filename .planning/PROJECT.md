# Portty

## What This Is

Portty is a macOS terminal emulator with built-in remote access over Tailscale. It is a stripped-down fork of AgentHub: it keeps the persistent daemon-backed terminal sessions, Tailscale integration, web/desktop sharing, and notifications, and removes everything specific to AI agent CLIs. Its first user is the author, who runs a terminal (often with Claude Code inside) on an office MacBook Air and controls it from a personal MacBook Air at home.

## Core Value

A terminal session started on one Mac can be attached to from another Mac (or a browser) over the tailnet, and it survives any idle gap, from minutes to overnight, with history intact.

## Requirements

### Validated

<!-- Inherited from AgentHub (forked at 624cbf87, v4.2) and kept in Portty. Validated in AgentHub; must keep working after the strip-down. -->

- ✓ Persistent PTY sessions owned by a background daemon. They survive GUI close/restart. (existing)
- ✓ Wails desktop GUI with tabbed xterm.js terminals and a native macOS tray (NSStatusBar). (existing)
- ✓ Tailscale integration: tailnet peer discovery, browser-trusted TLS over Tailscale HTTPS. (existing)
- ✓ Remote attach to sessions on other tailnet machines, from the GUI and from the CLI (`attach host:id`, WSS relay, tmux-style status bar). (existing)
- ✓ Per-session web share with read-only and read/write capability links, multiple simultaneous clients, and PTY resize arbitration. (existing)
- ✓ Vendored xterm.js plugin suite (GPU render, search, links, sixel, OSC 52, OSC 9;4). (existing)
- ✓ Native notifications and auto-update checks. (existing)
- ✓ Remove all agent-CLI-specific code (CLI picker, badges, per-agent args/themes, agent status, chat→PTY bridge) — Phase 1
- ✓ New sessions start macOS zsh as a login shell (`/bin/zsh -l`) — Phase 1
- ✓ File browser/editor removed (local and remote) — Phase 1
- ✓ Windows and Linux code paths, packaging, and CI removed. macOS only — Phase 1
- ✓ Tailscale-off state: LocalAPI health check, banner, share/remote/web-server controls disabled — Phase 1 (NET-01)

### Active

- [ ] Rebrand AgentHub to Portty everywhere: binary names, Go module path, app/bundle ID, UI strings, docs, comments, tests, CI, and icons/logos from `portty-brand/`. Zero "agenthub" references left in tracked files.
- [ ] Website https://portty.app and repo github.com/scottkw/portty are the canonical links. The repo is private during development and becomes public before the first release
- [ ] Releases ship only as GitHub release DMGs (linked from portty.app). Portty checks them on launch, periodically, and on demand ("Check for Updates…"), notifies the user, and downloads, verifies and reveals the DMG. Same flow as GIFsmith/TransomWM
- [ ] Remove the Hub session grid and replace it with tabs plus a sidebar that lists local sessions and remote tailnet peers' sessions. Clicking one opens it in a tab.
- [ ] Replace the 138 terminal themes with system light/dark for the app and terminals
- [ ] Redesign the GUI to look and behave like a native macOS app (LOOK-01, Phase 2)
- [ ] Remove session chat entirely (desktop, web, daemon API, relay frames)
- [ ] Sharing is one on/off state: shared = tailnet peers (sidebar) AND a tailnet web link, always full control. Funnel and read-only links are removed.
- [ ] CLI keeps `attach host:id` (renamed `portty attach`) for use from Warp or any terminal
- [ ] Overnight test passes: start a shell running Claude Code on the office Mac, attach from the home Mac, leave it idle overnight, reattach the next morning with scrollback intact. Attaching from a browser works too.

### Out of Scope

- AI agent CLI management (detection, picker, agent status, agent bridge): commoditized; Portty is a plain terminal
- Hub session grid: replaced by a simpler tabs + sidebar model
- File browser/editor: not needed for remote terminal control
- Session chat: not needed (user decision 2026-11-15; removed in Phase 2)
- Windows and Linux: both target machines are Macs; removing them cuts maintenance a lot
- Claude Code Remote Control as the solution: needs a full terminal, not just the Claude session
- Warp session-sharing "tickler": Warp's idle timeout is server-side and not configurable (warpdotdev/warp#11972), and fake activity would type into the live terminal
- Signed/notarized DMG release: not required for v1 "done". Can follow later, reusing AgentHub's pipeline.
- Homebrew distribution: releases ship only as GitHub release DMGs linked from portty.app (user decision 2026-11-14)

## Context

- **Origin:** Warp session sharing drops after an idle period, and the user can't reconnect. Warp offers no setting to change that. Portty replaces Warp sharing for office→home control.
- **Codebase:** Forked from AgentHub (`~/dev/agenthub`, github.com/scottkw/agenthub) with full git history. Go 1.26 + Wails v2 + React/Vite frontend, `tailscale.com` tsnet, coder/websocket, go-pty. Internal packages: attach, capability, daemon, files, pty, relay, release, status, statusbar, tailnet, testutil, updater, webserver.
- **Agent-specific hotspots (first pass):** `app.go`, `internal/pty/detect.go`, `internal/status/detector.go`, `internal/daemon/{engine,api,client}.go`, `internal/daemon/notify_theme_*.go`, frontend agent picker and badges.
- **Removal candidates:** `internal/files` (file browser), Hub frontend components, `tray_{linux,windows}.go`, `notification_{linux,windows}.go`, `packaging/`, Windows pty input.
- **Brand:** `portty-brand/` (from Claude Design) mirrors AgentHub's asset layout: `build/appicon.png`, `build/AppIcon.iconset/`, `assets/tray_icon*.png` (progress quartiles plus error), `docs/portty-title-logo{,-dark}.png`.
- **Existing quality bar:** AgentHub has a large Go test suite, Vitest, Playwright, and a security-review fuzz harness. Removed features take their tests with them. Kept features keep theirs.
- **Accounts:** office Mac is signed into a corporate Claude account, home Mac into personal Max. Portty is account-agnostic because it just runs a shell.

## Constraints

- **License**: MIT. Keep the root `LICENSE` (MIT, "Copyright (c) 2026 Ken Scott"). README and package metadata (e.g. `frontend/package.json` `license`) state MIT — the user requires the code to be MIT licensed
- **Platform**: macOS only (Apple Silicon MacBook Airs). Both target machines are Macs.
- **Tech stack**: Keep AgentHub's stack (Go/Wails/React/tsnet). This is a strip-down, not a rewrite.
- **Network**: Remote access only over the user's tailnet (plus opt-in Funnel). No new relay servers or cloud dependencies.
- **Security**: Keep token-gated, tailnet-only share links and the CSP intact through the refactor. Remote write access to a shell is powerful.
- **Deletion discipline**: Chesterton's Fence. Read each piece and know why it exists before removing it, especially in daemon, relay, and capability code shared between kept and removed features.

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Fork AgentHub instead of building new or using tmux+SSH | Reuses proven daemon, relay, and Tailscale code. The user wants a GUI app plus web share, not just SSH. | — Pending |
| Fork with full git history | Lets us trace why code exists before deleting it, and cherry-pick AgentHub fixes later | — Pending |
| macOS only | Both machines are MacBook Airs. Big code and CI reduction. | ✓ Good — Phase 1 removed Windows/Linux code and CI |
| Tabs + sidebar instead of Hub grid | Simpler navigation for a plain terminal | — Pending |
| One Share action = tailnet peers + web link | User wants sharing to automatically cover both desktop and browser | — Pending |
| Keep CLI `attach` | Fallback from Warp or any terminal; code already exists | — Pending |
| Keep themes/plugins, notifications/updates; drop file browser | User's explicit choice | — Pending |
| Only macOS light/dark, following the system (drops the theme library) | Simpler, consistent look across app and terminals (user decision 2026-11-15) | — Pending |
| Radical simplicity: one share state, full control, tailnet only; no Funnel, no read-only links | User wants Portty MUCH simpler than AgentHub (2026-11-15) | — Pending |
| Remove session chat (reverses the earlier "keep chat") | Not needed for remote terminal control (user decision 2026-11-15) | — Pending |
| Website portty.app; private repo scottkw/portty | User decision 2026-11-14 | — Pending |
| No Homebrew. GitHub release DMGs plus a GIFsmith-style update check, download and verify | Same distribution model as GIFsmith/TransomWM | — Pending |
| macOS-native GUI redesign, folded into Phase 2 | macOS-only app no longer needs cross-platform chrome; redo the UI once alongside tabs + sidebar (user decision 2026-11-15) | — Pending |
| Make scottkw/portty public before the first release | The anonymous Releases API returns 404 for private repos, so update checks would never see a release | — Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check: still the right priority?
3. Audit Out of Scope: reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-11-15 after Phase 1*
