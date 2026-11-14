# Requirements: Portty

**Defined:** 2026-11-14
**Core Value:** A terminal session started on one Mac can be attached to from another Mac (or a browser) over the tailnet, and it survives any idle gap, from minutes to overnight, with history intact.

## v1 Requirements

### Branding

- [ ] **BRND-01**: The built app is named Portty: bundle name, bundle ID, window title, menu bar, About box, tray tooltip
- [ ] **BRND-02**: The app icon, tray icons (including progress and error states), and title logos come from `portty-brand/`
- [ ] **BRND-03**: CLI binary is `portty` (e.g. `portty attach`, `portty daemon`), and the Go module path, config/data directories, and socket names use "portty" instead of "agenthub"
- [ ] **BRND-04**: README, help content, and in-app text describe Portty as a terminal with remote access. No AgentHub or agent-CLI wording remains in user-facing text.
- [ ] **BRND-05**: No "AgentHub"/"agenthub" reference remains anywhere in the repo (code, identifiers, comments, tests, CI, scripts, docs, config). A case-insensitive search over tracked files, excluding `.planning/` history and git history, finds zero hits.
- [ ] **BRND-06**: The app's website is https://portty.app. About box, Help, README, and update/release links point to it or to github.com/scottkw/portty, never to AgentHub URLs.

### Terminal

- [ ] **TERM-01**: User can open a new local terminal session, which starts their login shell in a chosen (or last-used) directory
- [ ] **TERM-02**: Sessions keep running in the daemon when the GUI window is closed or the app restarts, and reopen with scrollback intact
- [ ] **TERM-03**: User can run multiple sessions as tabs, and rename and close them
- [ ] **TERM-04**: Themes and xterm.js plugins (GPU render, search, links, sixel, OSC 52, OSC 9;4 progress) keep working, controllable from Settings

### Navigation

- [ ] **NAV-01**: A sidebar lists the user's local sessions. Clicking one focuses or opens it in a tab.
- [ ] **NAV-02**: The sidebar lists sessions shared by other tailnet peers, grouped by machine. Clicking one attaches to it in a tab.
- [ ] **NAV-03**: The Hub grid view is gone, and the app opens straight into tabs + sidebar

### Remote & Sharing

- [ ] **SHAR-01**: One "Share" action on a session makes it visible to the user's tailnet peers (shows in their Portty sidebar) AND produces a tailnet-only web link. No separate toggles.
- [ ] **SHAR-02**: User can choose read-only or read/write for a share. The capability-gated access model from AgentHub is kept.
- [ ] **SHAR-03**: Opening the web link in a browser on the tailnet gives a working interactive terminal for the session
- [ ] **SHAR-04**: User can stop sharing a session, which disconnects remote viewers and removes it from peers' sidebars
- [ ] **SHAR-05**: Tailscale Funnel public sharing stays available as a separate, risk-gated step (read-only and consent-gated read/write)
- [ ] **SHAR-06**: From any terminal (e.g. Warp), `portty attach <host>:<session>` attaches to a remote shared session with the tmux-style status bar
- [ ] **SHAR-07**: A remote attach left idle for 12+ hours stays connected or reconnects automatically. Detaching and reattaching restores scrollback.

### Chat & Notifications

- [ ] **CHAT-01**: The per-session human chat works across desktop and web surfaces, with the `@session` agent-inject bridge removed
- [ ] **NOTF-01**: Native macOS notifications and the auto-update check keep working under the Portty name and release feed

### Strip-down

- [ ] **STRP-01**: All agent-CLI code is removed: CLI detection and PATH discovery, agent picker, agent badges, per-agent args and theme broadcast, Antigravity, and agent working/waiting status detection
- [ ] **STRP-02**: The file browser/editor (local and remote, GUI and web) is removed, including its daemon API and capability
- [ ] **STRP-03**: Windows and Linux code paths, packaging, and CI jobs are removed. Build, test, and lint pass on macOS.
- [ ] **STRP-04**: Tests for removed features are deleted, tests for kept features still pass, and the security fuzz harness still covers relay and webserver

### Verification

- [ ] **VERF-01**: Overnight end-to-end test passes. On the office Mac, start a session running Claude Code and share it. On the home Mac, attach from the Portty GUI, leave it idle overnight, and the next morning continue typing with scrollback intact. The same session also opens from a browser.

## v2 Requirements

### Distribution

- **DIST-01**: Signed and notarized Portty DMG published as a GitHub release
- **DIST-02**: Auto-update pulls from the Portty releases feed

### Convenience

- **CONV-01**: Office Mac stays awake while a session is shared (built-in keep-awake toggle)
- **CONV-02**: Auto-share new sessions to own tailnet devices without clicking Share

## Out of Scope

| Feature | Reason |
|---------|--------|
| AI agent CLI management | Commoditized. Portty is a plain terminal. |
| Hub session grid | Replaced by tabs + sidebar |
| File browser/editor | Not needed for remote terminal control |
| Windows / Linux | Both target machines are Macs. Cuts maintenance. |
| Claude Code Remote Control | User needs a full terminal, not just the Claude session |
| Warp "tickler" | Warp's idle timeout is server-side and not configurable, and fake activity would type into the live terminal |
| New relay/cloud services | Tailnet (plus opt-in Funnel) only |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| BRND-01 | Phase 3 | Pending |
| BRND-02 | Phase 3 | Pending |
| BRND-03 | Phase 3 | Pending |
| BRND-04 | Phase 3 | Pending |
| BRND-05 | Phase 3 | Pending |
| BRND-06 | Phase 3 | Pending |
| TERM-01 | Phase 1 | Pending |
| TERM-02 | Phase 1 | Pending |
| TERM-03 | Phase 2 | Pending |
| TERM-04 | Phase 1 | Pending |
| NAV-01 | Phase 2 | Pending |
| NAV-02 | Phase 2 | Pending |
| NAV-03 | Phase 2 | Pending |
| SHAR-01 | Phase 4 | Pending |
| SHAR-02 | Phase 4 | Pending |
| SHAR-03 | Phase 4 | Pending |
| SHAR-04 | Phase 4 | Pending |
| SHAR-05 | Phase 4 | Pending |
| SHAR-06 | Phase 4 | Pending |
| SHAR-07 | Phase 5 | Pending |
| CHAT-01 | Phase 1 | Pending |
| NOTF-01 | Phase 3 | Pending |
| STRP-01 | Phase 1 | Pending |
| STRP-02 | Phase 1 | Pending |
| STRP-03 | Phase 1 | Pending |
| STRP-04 | Phase 1 | Pending |
| VERF-01 | Phase 5 | Pending |

**Coverage:**
- v1 requirements: 27 total
- Mapped to phases: 27
- Unmapped: 0 ✓

---
*Requirements defined: 2026-11-14*
*Last updated: 2026-11-14 after roadmap creation (traceability mapped)*
