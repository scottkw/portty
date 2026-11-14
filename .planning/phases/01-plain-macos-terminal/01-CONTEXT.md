# Phase 1: Plain macOS Terminal - Context

**Gathered:** 2026-11-14
**Status:** Ready for planning

<domain>
## Phase Boundary

Turn the forked AgentHub codebase into a plain macOS terminal. New sessions run the user's login shell. All agent-CLI code, the file browser/editor, and Windows/Linux code/packaging/CI are removed. Every kept feature (daemon persistence, tabs, themes/plugins, chat, web share, Funnel, Tailscale remote attach, notifications, tray, auto-update) keeps working. The Hub UI, branding/paths rename, and Share redesign are NOT in this phase (Phases 2, 3, 4).

</domain>

<decisions>
## Implementation Decisions

### Session status & notifications
- Status model shrinks to running / exited / errored. Delete the agent output heuristics (`internal/status` pattern sets for claude/agy, the working/waiting/idle classification).
- Replace awaiting-input notifications with a generic trigger: when a session's PTY output contains a terminal bell (BEL `\x07` outside OSC strings) or an OSC 9 notification (`ESC ] 9 ; <text>`, excluding OSC 9;4 progress) AND the app window is hidden, fire one native macOS notification (de-duplicated or rate-limited per session). Keep the existing Settings → Behavior toggle (default OFF).
- Keep shell-exit behavior: clean exit gives a 5s countdown, toast, and Keep Open. Non-zero exit stays open with the error state.
- Keep OSC 9;4 progress (tab underline + tray quartile glyph) unchanged.

### New session & CLI
- New sessions start `$SHELL` as a login shell. No shell picker and no arguments field.
- Starting directory defaults to the last-used directory, picked with the native folder picker (existing behavior).
- CLI `new [dir]` starts a shell. Drop the agent argument and the `--` arg passthrough. Keep the other CLI commands (list, attach, kill, daemon, etc.).
- Keep the "web-sharing a raw shell" warning as a one-time warning, re-enableable in Settings. It must not fire on every share.

### Removal boundaries
- Remove the agent/CLI field from the session API, daemon state, and persisted session data. Old persisted sessions must still load, with the unknown field ignored (no crash).
- CI: delete the Windows and Linux build/release jobs and the winget distribute job. Keep the Playwright browser e2e on its ubuntu runner (it tests web pages, not a Linux app). macOS build/test jobs stay.
- Delete `packaging/winget`. Leave `packaging/homebrew` untouched (v2 distribution).
- Do NOT rename paths, socket, port, module path, or binary in this phase (Phase 3). While testing Portty builds in Phases 1-2, quit any installed AgentHub, because they share config paths, socket, and port 7443.
- Chat: remove the `@session` pinned target and the chat→PTY inject bridge, including its server-side endpoint. Keep `@alias` mentions, presence, typing, export, and unread badges.
- Remove: CLI detection/PATH discovery for agents (`internal/pty/detect.go` known CLIs), agent picker, agent badges/icons, per-agent args persistence, per-agent theme broadcast (OpenCode SIGUSR2), Antigravity, agent status UI (filter counts and dots that mean working/waiting).
- Remove the file browser/editor everywhere: `internal/files`, its daemon/webserver routes, the files capability, `FileBrowser*`/`Editor` components, remote-browse UI (`RemoteBrowseDNSWarning`), and web-share file panels.
- Remove Windows/Linux sources: `tray_{linux,windows}.go`, `notification_{linux,windows}.go`, `*_windows.go`/`*_linux.go` in internal packages (e.g. `internal/pty/win32input.go`, `notify_theme_windows.go`), and Windows/Linux-only dependencies (go-winio, godbus, etc.) if they become unused (`go mod tidy`).

### Claude's Discretion
- Order and granularity of removal plans, as long as every plan leaves the build and tests green.
- Whether the Hub keeps rendering in Phase 1 with agent bits stripped (expected: yes, minimal edits, since Phase 2 deletes it), or which Hub pieces are simply simplified.
- Exact BEL/OSC 9 rate-limit window.

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `GetShellPath()` (Go App binding) plus the single "Shell" row in `frontend/src/components/NewSessionModal.tsx`, which already supports bare `'shell'` sessions (SHELL-10).
- Native notification plumbing: `notification_darwin.go` (UNUserNotificationCenter), edge-triggered/de-duplicated logic from v4.2 #110. Re-point it from status transitions to BEL/OSC 9 events.
- OSC parsing regexes in `internal/daemon/engine.go` (~line 655-705) already distinguish BEL-terminated and ST-terminated OSC sequences, so BEL-inside-OSC must not count as a bell.

### Established Patterns
- Go/Wails app: `app.go` (1742 lines) binds Go to the frontend. The daemon is in `internal/daemon` (`engine.go` 1448 lines, `api.go` 2090 lines, `client.go`). The web server is `internal/webserver`, the relay `internal/relay`, the capability model `internal/capability`.
- Platform splits use `_darwin.go` / `_linux.go` / `_windows.go` file suffixes plus build tags. Delete non-darwin files and fix any shared code that referenced them.
- The test suite is large: Go tests, Vitest (`frontend/src/**/__tests__`, `*.test.tsx`), Playwright (`frontend/playwright.config.ts`, `playwright-fixture`), and the `.security-review/` fuzz harness for relay and webserver. Tests for removed features are deleted along with them.

### Integration Points
- Agent-specific Go files: `app.go`, `internal/pty/detect.go`, `internal/status/detector.go`, `internal/daemon/{engine,api,client}.go`, `internal/daemon/notify_theme_{unix,windows}.go`.
- Frontend: `App.tsx` (passes `detectedShells`, agent lists), `NewSessionModal.tsx`, `components/Hub/*` (agent badges, status filter `HubFilterBar`, `MentionPopover` @session), `ShellWebShareBanner.tsx`, `SessionShare/`.
- CI: `.github/workflows/{build,release,distribute,e2e}.yml`.

</code_context>

<specifics>
## Specific Ideas

- The user runs Claude Code inside Portty shells. The BEL/OSC 9 notification exists so "Claude needs input" still reaches them without agent-specific code.
- Chesterton's Fence (project CLAUDE.md): read why each piece exists before deleting, especially daemon/relay/capability code shared between kept and removed features.

</specifics>

<deferred>
## Deferred Ideas

- Path/socket/port/module/binary rename → Phase 3 (BRND-03).
- Hub removal and sidebar → Phase 2.
- Homebrew packaging → v2 (DIST).

</deferred>

<amendment date="2026-11-14">
## Amendment: Fixed zsh, simple Tailscale check

The user decided this mid-execution, after plans 01-01..01-05 had landed. It supersedes the earlier "$SHELL as a login shell" and "Settings shell path" behavior, and it becomes new plans after 01-10.

### Shell: always macOS zsh
- Every session runs exactly `/bin/zsh -l`. No shell choice anywhere: `$SHELL` is ignored and there is no Settings shell path. The resolver from 01-05 shrinks to a constant. Remove `GetShellPath` as a user-visible concept, unless a test seam still needs it.
- New Session dialog: only the working-directory picker remains. Remove the read-only shell line and its helper text.
- Settings → Paths: delete the whole section (shell row and tailscale row), including the stale-path filter and the `cliPaths` override plumbing. Old settings files with `shellPath`/`cliPaths` keys must still load; the unknown keys are ignored and dropped on the next save.

### Tailscale: detect, don't manage
- Portty only checks for a working Tailscale connection through the existing LocalAPI (`tailscale.com/client/local`). Managing Tailscale is the user's job.
- Remove the Tailscale install flow (`brew install --cask tailscale-app`), the custom tailscale binary path override, the `macsys` daemon and permission probes, and the multi-state health UI beyond connected/not connected.
- Keep LocalAPI use for peer discovery, HTTPS certs and Funnel. Those features need it.
- When not connected (LocalAPI unreachable, or backend state not Running/logged in): show a persistent in-app banner, "Tailscale isn't connected. Remote access and sharing are off until it is.", and disable the Share and remote/peer controls with the same reason. Local terminals work normally. Re-check automatically (on launch, periodically, and on window focus), and clear the banner when the connection is back.
</amendment>

<amendment date="2026-11-14" n="2">
## Amendment 2: No status or exit UI, no bell notifications

The user decided this after plan 01-06 landed. It supersedes the "Session status & notifications" decisions above and the UI-SPEC status/exit sections.

- **No status indicators.** Remove the running/exited/errored tab dot, the Hub card status labels/icons, the status part of the exit toast, and any UI that shows session status. 01-06's derived status may stay in the daemon only where something still needs it (for example to know a session ended). It is not shown to the user.
- **No exit handling UI.** Remove the 5 s auto-close countdown, the "Keep Open" button, the exit toast/banner and the error-state tab. New behavior: when a session's shell exits, for any reason or exit code, its tab closes immediately and the session is removed. Exit detection must still be reliable for long-lived shells (the >300 s exit-window fix stays in scope, because tabs must close for shells that exit after hours).
- **No bell/OSC 9 notifications.** Drop the planned BEL/OSC 9 → native notification feature and the "awaiting input" notification toggle plus its settings key handling (old settings keys load and are ignored). Keep the native notification plumbing (`notification_darwin.go`) only if something else still uses it. Phase 6 update notifications will use it, so keep a minimal path for that.
- **Kept:** OSC 9;4 progress (tab underline + tray quartile glyph). It is terminal progress, not session status.
- Implemented by the re-planned plan 01-10 and the new follow-up plans, together with Amendment 1.
</amendment>
