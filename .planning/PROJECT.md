# Portty

## What This Is

Portty is a macOS terminal emulator with built-in remote access over Tailscale. It is a stripped-down fork of AgentHub: it keeps the persistent daemon-backed terminal sessions, Tailscale integration, web/desktop sharing, chat, themes, and notifications, and removes everything specific to AI agent CLIs. Its first user is the author, who runs a terminal (often with Claude Code inside) on an office MacBook Air and controls it from a personal MacBook Air at home.

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
- ✓ Tailscale Funnel public sharing: risk-gated, auto-expiring, consent-gated write. (existing)
- ✓ Per-session human chat side channel. (existing)
- ✓ 138 terminal themes and the vendored xterm.js plugin suite (GPU render, search, links, sixel, OSC 52, OSC 9;4). (existing)
- ✓ Native notifications and auto-update checks. (existing)

### Active

- [ ] Rebrand AgentHub to Portty everywhere: binary names, Go module path, app/bundle ID, UI strings, docs, comments, tests, CI, and icons/logos from `portty-brand/`. Zero "agenthub" references left in tracked files.
- [ ] Website https://portty.app and repo github.com/scottkw/portty (private) are the canonical links
- [ ] Remove all agent-CLI-specific code: CLI detection/picker, agent badges, per-agent arguments/themes (e.g. the OpenCode SIGUSR2 path), Antigravity, agent status detection, and the `@session` chat→PTY bridge
- [ ] New sessions start the user's login shell (no agent picker)
- [ ] Remove the Hub session grid and replace it with tabs plus a sidebar that lists local sessions and remote tailnet peers' sessions. Clicking one opens it in a tab.
- [ ] Remove the file browser/editor (read and write, local and remote)
- [ ] Remove Windows and Linux code paths, packaging, and CI. macOS only.
- [ ] One "Share" action publishes a session to BOTH tailnet peers (shows in their sidebar) AND a tailnet web link. Funnel public sharing stays a separate risk-gated step.
- [ ] CLI keeps `attach host:id` (renamed `portty attach`) for use from Warp or any terminal
- [ ] Overnight test passes: start a shell running Claude Code on the office Mac, attach from the home Mac, leave it idle overnight, reattach the next morning with scrollback intact. Attaching from a browser works too.

### Out of Scope

- AI agent CLI management (detection, picker, agent status, agent bridge): commoditized; Portty is a plain terminal
- Hub session grid: replaced by a simpler tabs + sidebar model
- File browser/editor: not needed for remote terminal control
- Windows and Linux: both target machines are Macs; removing them cuts maintenance a lot
- Claude Code Remote Control as the solution: needs a full terminal, not just the Claude session
- Warp session-sharing "tickler": Warp's idle timeout is server-side and not configurable (warpdotdev/warp#11972), and fake activity would type into the live terminal
- Signed/notarized DMG release: not required for v1 "done". Can follow later, reusing AgentHub's pipeline.

## Context

- **Origin:** Warp session sharing drops after an idle period, and the user can't reconnect. Warp offers no setting to change that. Portty replaces Warp sharing for office→home control.
- **Codebase:** Forked from AgentHub (`~/dev/agenthub`, github.com/scottkw/agenthub) with full git history. Go 1.26 + Wails v2 + React/Vite frontend, `tailscale.com` tsnet, coder/websocket, go-pty. Internal packages: attach, capability, daemon, files, pty, relay, release, status, statusbar, tailnet, testutil, updater, webserver.
- **Agent-specific hotspots (first pass):** `app.go`, `internal/pty/detect.go`, `internal/status/detector.go`, `internal/daemon/{engine,api,client}.go`, `internal/daemon/notify_theme_*.go`, frontend agent picker and badges.
- **Removal candidates:** `internal/files` (file browser), Hub frontend components, `tray_{linux,windows}.go`, `notification_{linux,windows}.go`, `packaging/`, Windows pty input.
- **Brand:** `portty-brand/` (from Claude Design) mirrors AgentHub's asset layout: `build/appicon.png`, `build/AppIcon.iconset/`, `assets/tray_icon*.png` (progress quartiles plus error), `docs/portty-title-logo{,-dark}.png`.
- **Existing quality bar:** AgentHub has a large Go test suite, Vitest, Playwright, and a security-review fuzz harness. Removed features take their tests with them. Kept features keep theirs.
- **Accounts:** office Mac is signed into a corporate Claude account, home Mac into personal Max. Portty is account-agnostic because it just runs a shell.

## Constraints

- **Platform**: macOS only (Apple Silicon MacBook Airs). Both target machines are Macs.
- **Tech stack**: Keep AgentHub's stack (Go/Wails/React/tsnet). This is a strip-down, not a rewrite.
- **Network**: Remote access only over the user's tailnet (plus opt-in Funnel). No new relay servers or cloud dependencies.
- **Security**: Keep AgentHub's capability-gated share model, CSP, and Funnel consent gates intact through the refactor. Remote write access to a shell is powerful.
- **Deletion discipline**: Chesterton's Fence. Read each piece and know why it exists before removing it, especially in daemon, relay, and capability code shared between kept and removed features.

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Fork AgentHub instead of building new or using tmux+SSH | Reuses proven daemon, relay, and Tailscale code. The user wants a GUI app plus web share, not just SSH. | — Pending |
| Fork with full git history | Lets us trace why code exists before deleting it, and cherry-pick AgentHub fixes later | — Pending |
| macOS only | Both machines are MacBook Airs. Big code and CI reduction. | — Pending |
| Tabs + sidebar instead of Hub grid | Simpler navigation for a plain terminal | — Pending |
| One Share action = tailnet peers + web link | User wants sharing to automatically cover both desktop and browser | — Pending |
| Keep CLI `attach` | Fallback from Warp or any terminal; code already exists | — Pending |
| Keep chat, themes/plugins, notifications/updates; drop file browser | User's explicit choice | — Pending |
| Website portty.app; private repo scottkw/portty | User decision 2026-11-14 | — Pending |

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
*Last updated: 2026-11-14 after initialization*
