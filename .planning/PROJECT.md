# Portty

## What This Is

Portty is a macOS terminal emulator with built-in remote access over Tailscale. It is a stripped-down fork of the author's earlier app: it keeps the persistent daemon-backed terminal sessions, Tailscale integration, web/desktop sharing, and notifications, and removes everything specific to AI agent CLIs. Its first user is the author, who runs a terminal (often with Claude Code inside) on an office MacBook Air and controls it from a personal MacBook Air at home.

## Core Value

A terminal session started on one Mac can be attached to from another Mac (or a browser) over the tailnet, and it survives any idle gap, from minutes to overnight, with history intact.

## Current State

v1.1 Hardening shipped 2026-11-19 (Phases 7-8, 16/16 requirements). See `.planning/MILESTONES.md`.

## Next Milestone: v1.2 (scope in `.planning/seeds/SEED-261119-nus-v1-2-preview-and-remote-urls.md`)

1. Keep sessions alive across a Portty upgrade (PTY handoff to the new daemon), plus the paste/ack/input-queue todos. This is first: it serves the rock-solid priority.
2. File preview tabs via `portty open <file>` (images, PDF, markdown), local or remote.
3. Click a file path printed in the terminal to preview it (OSC 7 working directory).
4. Remote `localhost:PORT` URLs proxied over the tailnet and opened in the normal browser (no embedded browser).
5. A mobile-aware web app served by each host Mac over the tailnet. portty.app/app is a static launcher page only, owned by the website repo.

## Requirements

### Validated

<!-- Inherited from AgentHub (forked at 624cbf87, v4.2) and kept in Portty. Validated in AgentHub; must keep working after the strip-down. -->

- ✓ Persistent PTY sessions owned by a background daemon. They survive GUI close/restart. (existing)
- ✓ Wails desktop GUI with tabbed xterm.js terminals and a native macOS tray (NSStatusBar). (existing)
- ✓ Tailscale integration: tailnet peer discovery, browser-trusted TLS over Tailscale HTTPS. (existing)
- ✓ Remote attach to sessions on other tailnet machines, from the GUI and from the CLI (`attach host:id`, WSS relay, tmux-style status bar). (existing)
- ✓ Per-session web share with read-only and read/write capability links, multiple simultaneous clients, and PTY resize arbitration. (existing)
- ✓ Vendored xterm.js plugin suite (GPU render, search, links, sixel, OSC 52, OSC 9;4). (existing)
- ✓ Native notifications. (existing; the inherited go-selfupdate checker was replaced in v1.0)
- ✓ Remove all agent-CLI-specific code (CLI picker, badges, per-agent args/themes, agent status, chat→PTY bridge) — Phase 1
- ✓ New sessions start macOS zsh as a login shell (`/bin/zsh -l`) — Phase 1
- ✓ File browser/editor removed (local and remote) — Phase 1
- ✓ Windows and Linux code paths, packaging, and CI removed. macOS only — Phase 1
- ✓ Tailscale-off state: LocalAPI health check, banner, share/remote/web-server controls disabled — Phase 1 (NET-01)

- ✓ Rebrand to Portty everywhere (binaries, module path, bundle ID, strings, docs, icons from `portty-brand/`); brand gate in CI — v1.0
- ✓ Tabs plus a sidebar listing local and tailnet peers' sessions replace the Hub grid — v1.0
- ✓ System light/dark only for app and terminals; native macOS look — v1.0
- ✓ Session chat removed (desktop, web, daemon API, relay frames) — v1.0
- ✓ One Share state: tailnet peers' sidebars plus a tailnet web link, full control; Funnel and read-only links removed — v1.0
- ✓ `portty attach host:session` for Warp or any terminal — v1.0
- ✓ Sessions survive network glitches, sleep/wake, daemon death (auto-respawn) and replay history without gaps; verified on two Tart VMs over the real tailnet — v1.0
- ✓ GitHub-release update check on launch, daily and on demand; download, SHA-256 + Team ID + signing identifier + notarization verify, reveal in Finder — v1.0

### Active

- [ ] Overnight two-Mac test (VERF-01, M-84) and lid-sleep / AP-switch test (M-85): deferred by the user at v1.0 close
- [ ] Deferred two-Mac UI checks from Phases 2–4 (peer attach, live share state, VoiceOver, README on GitHub)
- [ ] First real release: make scottkw/portty public, add release secrets, tag `vX.Y.Z`; run M-93 (CI DMG signing/notarize/staple, notarized verify + reveal, delivered notification)
- [ ] Website https://portty.app links to the GitHub release DMGs
- [ ] v1.1 Hardening: the seven fixes listed under Current Milestone

### Out of Scope

- AI agent CLI management (detection, picker, agent status, agent bridge): commoditized; Portty is a plain terminal
- Hub session grid: replaced by a simpler tabs + sidebar model
- File browser/editor: not needed for remote terminal control
- Session chat: not needed (user decision 2026-11-15; removed in Phase 2)
- Windows and Linux: both target machines are Macs; removing them cuts maintenance a lot
- Claude Code Remote Control as the solution: needs a full terminal, not just the Claude session
- Warp session-sharing "tickler": Warp's idle timeout is server-side and not configurable (warpdotdev/warp#11972), and fake activity would type into the live terminal
- Homebrew distribution: releases ship only as GitHub release DMGs linked from portty.app (user decision 2026-11-14)

## Context

- **Origin:** Warp session sharing drops after an idle period, and the user can't reconnect. Warp offers no setting to change that. Portty replaces Warp sharing for office→home control.
- **Codebase:** Forked from AgentHub (`~/dev/agenthub`, github.com/scottkw/agenthub) with full git history. Go 1.26 + Wails v2 + React/Vite frontend, `tailscale.com` tsnet, coder/websocket, go-pty. Internal packages after v1.0: attach, capability, daemon, pty, relay, statusbar, tailnet, testutil, updater, wakewatch, webserver.
- **State after v1.0 (2026-11-18):** six phases shipped; strip-down, tabs + sidebar, rebrand, one-click share, overnight survival, release updates. Daemon single-instance lock + GUI/CLI respawn; viewer replay with offset resume. Release pipeline signs, notarizes and staples the .app and DMG.
- **Known debt:** see `.planning/milestones/v1.0-MILESTONE-AUDIT.md` and STATE.md Deferred Items (two-Mac checks, real-release checks, ~10 todos incl. leftover AgentHub plugin-banner code).
- **Brand:** `portty-brand/` (from Claude Design) mirrors AgentHub's asset layout: `build/appicon.png`, `build/AppIcon.iconset/`, `assets/tray_icon*.png` (progress quartiles plus error), `docs/portty-title-logo{,-dark}.png`.
- **Existing quality bar:** AgentHub has a large Go test suite, Vitest, Playwright, and a security-review fuzz harness. Removed features take their tests with them. Kept features keep theirs.
- **Accounts:** office Mac is signed into a corporate Claude account, home Mac into personal Max. Portty is account-agnostic because it just runs a shell.

## Constraints

- **License**: MIT. Keep the root `LICENSE` (MIT, "Copyright (c) 2026 Ken Scott"). README and package metadata (e.g. `frontend/package.json` `license`) state MIT — the user requires the code to be MIT licensed
- **Platform**: macOS only (Apple Silicon MacBook Airs). Both target machines are Macs.
- **Tech stack**: Keep the inherited stack (Go/Wails/React/tsnet). This is a strip-down, not a rewrite.
- **Network**: Remote access only over the user's tailnet. No new relay servers or cloud dependencies.
- **Security**: Keep token-gated, tailnet-only share links and the CSP intact through the refactor. Remote write access to a shell is powerful.
- **Deletion discipline**: Chesterton's Fence. Read each piece and know why it exists before removing it, especially in daemon, relay, and capability code shared between kept and removed features.

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Fork AgentHub instead of building new or using tmux+SSH | Reuses proven daemon, relay, and Tailscale code. The user wants a GUI app plus web share, not just SSH. | ✓ Good — daemon/relay/tsnet reused; v1.0 shipped in 4 days |
| Fork with full git history | Lets us trace why code exists before deleting it, and cherry-pick AgentHub fixes later | ✓ Good — Chesterton's Fence checks relied on it |
| macOS only | Both machines are MacBook Airs. Big code and CI reduction. | ✓ Good — Phase 1 removed Windows/Linux code and CI |
| Tabs + sidebar instead of Hub grid | Simpler navigation for a plain terminal | ✓ Good — Phase 2 |
| One Share action = tailnet peers + web link | User wants sharing to automatically cover both desktop and browser | ✓ Good — Phase 4 |
| Keep CLI `attach` | Fallback from Warp or any terminal; code already exists | ✓ Good — `portty attach`, reconnect + respawn in Phase 5 |
| Keep themes/plugins, notifications/updates; drop file browser | User's explicit choice | ✓ Good — themes later reduced to light/dark |
| Only macOS light/dark, following the system (drops the theme library) | Simpler, consistent look across app and terminals (user decision 2026-11-15) | ✓ Good — Phase 2 |
| Radical simplicity: one share state, full control, tailnet only; no Funnel, no read-only links | User wants Portty MUCH simpler than AgentHub (2026-11-15) | ✓ Good — Phase 4 |
| Remove session chat (reverses the earlier "keep chat") | Not needed for remote terminal control (user decision 2026-11-15) | ✓ Good — Phase 2 |
| Website portty.app; private repo scottkw/portty | User decision 2026-11-14 | — Pending (site not yet linked) |
| No Homebrew. GitHub release DMGs plus a GIFsmith-style update check, download and verify | Same distribution model as GIFsmith/TransomWM | ✓ Good — Phase 6; adds signing-identifier check |
| macOS-native GUI redesign, folded into Phase 2 | macOS-only app no longer needs cross-platform chrome; redo the UI once alongside tabs + sidebar (user decision 2026-11-15) | ✓ Good — plus 2026-11-18 polish |
| Daemon respawn from GUI poller + CLI, single-instance flock | Remote Mac must recover on its own; no one can log in to fix it | ✓ Good — Phase 5 R6 |
| Updater verifies DMG signing identifier, not just Team ID | Same team signs GIFsmith; blocks cross-app and downgrade swaps | ✓ Good — Phase 6 WR-01 |
| Make scottkw/portty public before the first release | The anonymous Releases API returns 404 for private repos, so update checks would never see a release | — Pending (user action before first release) |
| Mobile UI served by each host Mac; portty.app/app is a static launcher only | Trusting the website's origin would turn a website compromise into shell access on every Mac | — Pending (v1.2 Phase 14) |
| Remote localhost URLs proxied over the tailnet into the normal browser, no embedded browser | Wails v2 has a single web view; the proxy covers the real need (remote dev servers) | — Pending (v1.2 Phase 13) |
| Path click previews only paths the session printed | Keeps "no generic remote file-read API" | — Pending (v1.2 Phase 12) |
| Upgrades hand live sessions to the new daemon; DEBT-08 consent stays as the fallback | Upgrading the unattended office Mac must never cost a session | — Pending (v1.2 Phase 9) |
| Hardening checks fail toward the old daemon (takeover refuses an unconfirmed pid; socket 0600 with no process-wide umask) | A false refusal costs a consent prompt; a leaked umask or wrong pid could cost the remote Mac | ✓ Good — Phase 15; real two-build upgrade chain passed in the VM (M-155) |
| The viewer you type in sets the PTY size, also for `portty attach` (rows-1 under its bar) | One grid-owner rule for every viewer; a passive CLI watching a taller GUI grid keeps today's behaviour | ✓ Good — Phase 15 (user decision 2026-11-23) |
| Unknown leading flags are a usage error (exit 2); only no-args and macOS launch arguments (`-psn_*`, `-NS*`, `-Apple*`) start the GUI | A CLI call over SSH must never start a GUI, and a Finder/Dock/login-item launch on the remote Mac must never be refused | ✓ Good — Phase 16 (user decision 2026-11-23); every launch path passed in the VM (M-156) |
| `portty kill`/`rename` resolve names like share/attach, but need at least 8 characters for an ID prefix | A one-letter abbreviation of a name must never kill an unrelated session | ✓ Good — Phase 16 (code review WR-01) |

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
*Last updated: 2026-11-23 after Phase 16 (v1.2 Cleanup)*
