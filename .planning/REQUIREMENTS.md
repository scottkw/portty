# Requirements: Portty

**Defined:** 2026-11-14
**Core Value:** A terminal session started on one Mac can be attached to from another Mac (or a browser) over the tailnet, and it survives any idle gap, from minutes to overnight, with history intact.

## v1 Requirements

### Branding

- [ ] **BRND-01**: The built app is named Portty: bundle name, bundle ID, window title, menu bar, About box, tray tooltip
- [ ] **BRND-02**: The app icon, tray icons (including progress and error states), and title logos come from `portty-brand/`
- [x] **BRND-03**: CLI binary is `portty` (e.g. `portty attach`, `portty daemon`), and the Go module path, config/data directories, and socket names use "portty" instead of "agenthub"
- [x] **BRND-04**: README, help content, and in-app text describe Portty as a terminal with remote access. No AgentHub or agent-CLI wording remains in user-facing text.
- [x] **BRND-05**: No "AgentHub"/"agenthub" reference remains anywhere in the repo (code, identifiers, comments, tests, CI, scripts, docs, config). A case-insensitive search over tracked files, excluding `.planning/` history and git history, finds zero hits.
- [x] **BRND-06**: The app's website is https://portty.app. About box, Help, README, and update/release links point to it or to github.com/scottkw/portty, never to AgentHub URLs.

### Terminal

- [x] **TERM-01**: User can open a new local terminal session, which always starts macOS zsh as a login shell (`/bin/zsh -l`, no shell choice) in a chosen (or last-used) directory
- [x] **TERM-02**: Sessions keep running in the daemon when the GUI window is closed or the app restarts, and reopen with scrollback intact
- [ ] **TERM-03**: User can run multiple sessions as tabs, and rename and close them
- [x] **TERM-04**: xterm.js plugins (GPU render, search, links, sixel, OSC 52, OSC 9;4 progress) keep working, controllable from Settings
- [ ] **THEM-01**: The app and every terminal use one standard macOS light or dark appearance that follows the system setting live by default; Settings ▸ Appearance offers Automatic / Light / Dark to override it (user decision 2026-11-15, later the same day). The 138 terminal themes, the theme picker and its settings key are removed (old settings files still load). The web share page follows the browser's light/dark preference the same way (user decision 2026-11-15)
- [ ] **LOOK-01**: The desktop GUI is redesigned to look and behave like a native macOS app: system font, system accent color, a source-list style sidebar, a unified title bar and toolbar, native-style controls and icons, and no leftover custom dark chrome or hardcoded colors (user decision 2026-11-15)

- [x] **TERM-05**: When a session's shell exits, for any reason, its tab closes immediately. There is no status indicator, exit countdown, toast or banner, and this works for shells that exit hours after they started
- [x] **NET-01**: Portty checks for a working Tailscale connection (LocalAPI). When there isn't one, it shows a banner, "Tailscale isn't connected. Remote access and sharing are off until it is.", and disables the share and remote controls. It rechecks automatically and never installs or configures Tailscale

### Navigation

- [ ] **NAV-01**: A sidebar lists the user's local sessions. Clicking one focuses or opens it in a tab.
- [ ] **NAV-02**: The sidebar lists sessions shared by other tailnet peers, grouped by machine. Clicking one attaches to it in a tab.
- [ ] **NAV-03**: The Hub grid view is gone, and the app opens straight into tabs + sidebar

### Remote & Sharing

- [x] **SHAR-01**: Sharing is one on/off state per session. Shared means BOTH: visible to the user's tailnet peers (in their Portty sidebar) AND reachable through a tailnet-only web link. No share tabs, modes, or separate toggles.
- [x] **SHAR-02**: A share always grants full control (read/write). The read-only link type and its UI are removed; links stay token-gated and tailnet-only (user decision 2026-11-15)
- [x] **SHAR-03**: Opening the web link in a browser on the tailnet gives a working interactive terminal for the session
- [x] **SHAR-04**: User can stop sharing a session, which disconnects remote viewers and removes it from peers' sidebars
- [x] **SHAR-05**: Tailscale Funnel public sharing is removed: risk panel, consent gates, hold-to-confirm, public links, Funnel config and their tests (user decision 2026-11-15)
- [x] **SHAR-06**: From any terminal (e.g. Warp), `portty attach <host>:<session>` attaches to a remote shared session with the tmux-style status bar
- [ ] **SHAR-07**: A remote attach left idle for 12+ hours stays connected or reconnects automatically. Detaching and reattaching restores scrollback.

### Chat removal & Notifications

- [ ] **CHAT-01**: Session chat is removed everywhere: desktop and web chat UI, the daemon/webserver chat API and store, the relay chat frames, and their tests. Old clients' chat frames are dropped without error (user decision 2026-11-15; Phase 1 removed only the `@session` inject bridge)
- [x] **NOTF-01**: Native macOS notifications appear under the Portty name

### Updates & Distribution

- [ ] **UPD-01**: Portty checks the latest GitHub release of scottkw/portty (public Releases API, no login) shortly after launch and then periodically, and shows an in-app notice plus a native notification when a newer version than the running one is published
- [ ] **UPD-02**: The user can force an immediate check from a "Check for Updates…" menu item. It reports "Portty is up to date", "Version X is available", or a readable error
- [ ] **UPD-03**: From the update notice, one click downloads the release DMG, verifies its SHA-256 digest and Developer ID Team signature, and reveals it in Finder. This matches the GIFsmith flow (port of `~/dev/gifsmith/internal/update`)
- [ ] **UPD-04**: No published release, a 404, drafts, prereleases, network failures, and rate limits are all handled quietly: no false "update available", no repeated error popups, and periodic checks keep running
- [x] **DIST-03**: No Homebrew distribution. `packaging/homebrew` and every Homebrew reference (docs, scripts, release workflow) are removed. Releases ship only as GitHub release DMGs, linked from https://portty.app

### Strip-down

- [x] **STRP-01**: All agent-CLI code is removed: CLI detection and PATH discovery, agent picker, agent badges, per-agent args and theme broadcast, Antigravity, and agent working/waiting status detection
- [x] **STRP-02**: The file browser/editor (local and remote, GUI and web) is removed, including its daemon API and capability
- [x] **STRP-03**: Windows and Linux code paths, packaging, and CI jobs are removed. Build, test, and lint pass on macOS.
- [x] **STRP-04**: Tests for removed features are deleted, tests for kept features still pass, and the security fuzz harness still covers relay and webserver

### Verification

- [ ] **VERF-01**: Overnight end-to-end test passes. On the office Mac, start a session running Claude Code and share it. On the home Mac, attach from the Portty GUI, leave it idle overnight, and the next morning continue typing with scrollback intact. The same session also opens from a browser.

## v2 Requirements

### Distribution

- **DIST-01**: Signed and notarized Portty DMG published as a GitHub release

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
| Homebrew distribution | Releases ship only as GitHub release DMGs linked from portty.app (user decision 2026-11-14) |
| Warp "tickler" | Warp's idle timeout is server-side and not configurable, and fake activity would type into the live terminal |
| New relay/cloud services | Tailnet (plus opt-in Funnel) only |
| Terminal theme library (138 themes) | Replaced by system light/dark (THEM-01, 2026-11-15) |
| Funnel public sharing | Tailnet-only is enough; removed (SHAR-05, 2026-11-15) |
| Read-only share links | A share is always full control (SHAR-02, 2026-11-15) |
| Session chat | Not needed; removed in Phase 2 (user decision 2026-11-15) |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| BRND-01 | Phase 3 | Pending |
| BRND-02 | Phase 3 | Pending |
| BRND-03 | Phase 3 | Complete |
| BRND-04 | Phase 3 | Complete |
| BRND-05 | Phase 3 | Complete |
| BRND-06 | Phase 3 | Complete |
| TERM-01 | Phase 1 | Complete |
| TERM-02 | Phase 1 | Complete |
| TERM-03 | Phase 2 | Pending |
| TERM-04 | Phase 1 | Complete |
| TERM-05 | Phase 1 | Complete |
| THEM-01 | Phase 2 | Pending |
| LOOK-01 | Phase 2 | Pending |
| NET-01 | Phase 1 | Complete |
| NAV-01 | Phase 2 | Pending |
| NAV-02 | Phase 2 | Pending |
| NAV-03 | Phase 2 | Pending |
| SHAR-01 | Phase 4 | Complete |
| SHAR-02 | Phase 4 | Complete |
| SHAR-03 | Phase 4 | Complete |
| SHAR-04 | Phase 4 | Complete |
| SHAR-05 | Phase 4 | Complete |
| SHAR-06 | Phase 4 | Complete |
| SHAR-07 | Phase 5 | Pending |
| CHAT-01 | Phase 2 | Pending |
| NOTF-01 | Phase 3 | Complete |
| DIST-03 | Phase 3 | Complete |
| UPD-01 | Phase 6 | Pending |
| UPD-02 | Phase 6 | Pending |
| UPD-03 | Phase 6 | Pending |
| UPD-04 | Phase 6 | Pending |
| STRP-01 | Phase 1 | Complete |
| STRP-02 | Phase 1 | Complete |
| STRP-03 | Phase 1 | Complete |
| STRP-04 | Phase 1 | Complete |
| VERF-01 | Phase 5 | Pending |

**Coverage:**
- v1 requirements: 35 total
- Mapped to phases: 35
- Unmapped: 0 ✓

---
*Requirements defined: 2026-11-14*
*Last updated: 2026-11-15 after CHAT-01 changed to chat removal (Phase 2)*
