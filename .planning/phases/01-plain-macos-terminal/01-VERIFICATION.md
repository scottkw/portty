---
phase: 01-plain-macos-terminal
verified: 2026-11-15T03:55:00Z
status: passed
score: 5/7 must-haves verified
covered_files: [".github/workflows/build.yml",".github/workflows/distribute.yml",".github/workflows/release.yml",".gitignore",".planning/phases/01-plain-macos-terminal/01-01-PLAN.md",".planning/phases/01-plain-macos-terminal/01-01-SUMMARY.md",".planning/phases/01-plain-macos-terminal/01-02-PLAN.md",".planning/phases/01-plain-macos-terminal/01-02-SUMMARY.md",".planning/phases/01-plain-macos-terminal/01-03-PLAN.md",".planning/phases/01-plain-macos-terminal/01-03-SUMMARY.md",".planning/phases/01-plain-macos-terminal/01-04-PLAN.md",".planning/phases/01-plain-macos-terminal/01-04-SUMMARY.md",".planning/phases/01-plain-macos-terminal/01-05-PLAN.md",".planning/phases/01-plain-macos-terminal/01-05-SUMMARY.md",".planning/phases/01-plain-macos-terminal/01-06-PLAN.md",".planning/phases/01-plain-macos-terminal/01-06-SUMMARY.md",".planning/phases/01-plain-macos-terminal/01-07-PLAN.md",".planning/phases/01-plain-macos-terminal/01-07-SUMMARY.md",".planning/phases/01-plain-macos-terminal/01-08-PLAN.md",".planning/phases/01-plain-macos-terminal/01-08-SUMMARY.md",".planning/phases/01-plain-macos-terminal/01-09-PLAN.md",".planning/phases/01-plain-macos-terminal/01-09-SUMMARY.md",".planning/phases/01-plain-macos-terminal/01-10-PLAN.md",".planning/phases/01-plain-macos-terminal/01-10-SUMMARY.md",".planning/phases/01-plain-macos-terminal/01-11-PLAN.md",".planning/phases/01-plain-macos-terminal/01-11-SUMMARY.md",".planning/phases/01-plain-macos-terminal/01-12-PLAN.md",".planning/phases/01-plain-macos-terminal/01-12-SUMMARY.md","README.md","TESTING.md","app.go","app_exit_test.go","app_save_terminal_test.go","app_test.go","build.sh","cmd/playwright-fixture/main.go","cmd_attach.go","cmd_attach_test.go","cmd_cli.go","cmd_cli_test.go","cmd_remote.go","cmd_remote_test.go","frontend/e2e/chat-parity.spec.ts","frontend/e2e/fixture-env.ts","frontend/e2e/global-setup.ts","frontend/e2e/web-csp.spec.ts","frontend/e2e/web-share-scope.spec.ts","frontend/package.json","frontend/pnpm-lock.yaml","frontend/src/App.tsx","frontend/src/__tests__/themeTokens.test.ts","frontend/src/components/Hub/ChatMessage.test.tsx","frontend/src/components/Hub/ChatMessage.tsx","frontend/src/components/Hub/ChatPanel.test.tsx","frontend/src/components/Hub/ChatPanel.tsx","frontend/src/components/Hub/HubEmptyState.test.tsx","frontend/src/components/Hub/HubEmptyState.tsx","frontend/src/components/Hub/HubFilterBar.test.tsx","frontend/src/components/Hub/HubFilterBar.tsx","frontend/src/components/Hub/HubInteractiveModal.test.tsx","frontend/src/components/Hub/HubModal.test.tsx","frontend/src/components/Hub/HubModal.tsx","frontend/src/components/Hub/HubPanel.test.tsx","frontend/src/components/Hub/HubPanel.tsx","frontend/src/components/Hub/MentionPopover.test.tsx","frontend/src/components/Hub/MentionPopover.tsx","frontend/src/components/Hub/SessionCard.test.tsx","frontend/src/components/Hub/SessionCard.tsx","frontend/src/components/Hub/SessionCardGrid.test.tsx","frontend/src/components/Hub/SessionCardGrid.tsx","frontend/src/components/Hub/SessionShareModal.tsx","frontend/src/components/Hub/TerminalChatHost.tsx","frontend/src/components/Hub/useChatUnreadListeners.test.tsx","frontend/src/components/NewSessionModal.tsx","frontend/src/components/QuitConfirmModal.tsx","frontend/src/components/RemoteJoinCodeModal.test.tsx","frontend/src/components/RemoteJoinCodeModal.tsx","frontend/src/components/SessionShare/TailnetTab.tsx","frontend/src/components/SettingsJumpBar.tsx","frontend/src/components/SettingsSearch.tsx","frontend/src/components/SettingsTab.tsx","frontend/src/components/ShellWebShareBanner.tsx","frontend/src/components/Sidebar.tsx","frontend/src/components/StatusBar.tsx","frontend/src/components/TabBar.tsx","frontend/src/components/TailscaleBanner.tsx","frontend/src/components/WelcomeTab.tsx","frontend/src/components/__tests__/App.createTab.stayOnHub.test.tsx","frontend/src/components/__tests__/App.hub.test.tsx","frontend/src/components/__tests__/App.open-remote.test.tsx","frontend/src/components/__tests__/App.shellExit.test.tsx","frontend/src/components/__tests__/App.shellWebShare.test.tsx","frontend/src/components/__tests__/App.tailscale.test.tsx","frontend/src/components/__tests__/App.test.tsx","frontend/src/components/__tests__/NewSessionModal.test.tsx","frontend/src/components/__tests__/QuitConfirmModal.test.tsx","frontend/src/components/__tests__/SessionCard.share.test.tsx","frontend/src/components/__tests__/SessionShareModal.disconnect.test.tsx","frontend/src/components/__tests__/SessionShareModal.test.tsx","frontend/src/components/__tests__/SettingsTab.appearance-theme.test.tsx","frontend/src/components/__tests__/SettingsTab.hyperlinked-index.test.tsx","frontend/src/components/__tests__/SettingsTab.persistence.test.tsx","frontend/src/components/__tests__/SettingsTab.shell-warn-toggle.test.tsx","frontend/src/components/__tests__/SettingsTab.stay-on-hub-toggle.test.tsx","frontend/src/components/__tests__/SettingsTab.tailscale-status.test.tsx","frontend/src/components/__tests__/SettingsTab.test.tsx","frontend/src/components/__tests__/ShellWebShareBanner.test.tsx","frontend/src/components/__tests__/Sidebar.test.tsx","frontend/src/components/__tests__/StatusBar.shareSession.test.tsx","frontend/src/components/__tests__/TabBar.test.tsx","frontend/src/components/__tests__/TailnetTab.test.tsx","frontend/src/components/__tests__/TailscaleBanner.test.tsx","frontend/src/components/__tests__/TerminalPanel.scale.test.tsx","frontend/src/components/__tests__/WelcomeTab.install.test.tsx","frontend/src/components/__tests__/WelcomeTab.test.tsx","frontend/src/components/__tests__/style.contrast.test.ts","frontend/src/components/__tests__/style.hub.modal.test.ts","frontend/src/components/__tests__/style.hub.test.ts","frontend/src/components/__tests__/style.redesign.test.ts","frontend/src/components/__tests__/style.settings.test.ts","frontend/src/content/help/chat.md","frontend/src/content/help/faq.md","frontend/src/content/help/getting-started.md","frontend/src/lib/__tests__/remoteAdapter.test.ts","frontend/src/lib/__tests__/remoteSession.test.ts","frontend/src/lib/hubGroupCounts.test.ts","frontend/src/lib/hubGroupCounts.ts","frontend/src/lib/relayClient.test.ts","frontend/src/lib/relayClient.ts","frontend/src/lib/remoteAdapter.test.ts","frontend/src/lib/remoteAdapter.ts","frontend/src/lib/remoteSession.ts","frontend/src/lib/tailscale.ts","frontend/src/lib/vtColor.ts","frontend/src/lib/webMode.ts","frontend/src/style.css","frontend/src/wailsjs/go/main/App.d.ts","frontend/src/wailsjs/go/main/App.js","frontend/src/wailsjs/go/models.ts","frontend/src/wailsjs/wailsjs/go/models.ts","go.mod","go.sum","internal/capability/capability.go","internal/capability/capability_test.go","internal/daemon/alias_store_test.go","internal/daemon/api.go","internal/daemon/api_shell_warn_test.go","internal/daemon/api_stayonhub_test.go","internal/daemon/api_test.go","internal/daemon/chat.go","internal/daemon/chat_routes.go","internal/daemon/chat_routes_test.go","internal/daemon/chat_test.go","internal/daemon/client.go","internal/daemon/client_test.go","internal/daemon/engine.go","internal/daemon/engine_exit_test.go","internal/daemon/engine_migration_test.go","internal/daemon/engine_natural_exit_test.go","internal/daemon/engine_plugins_test.go","internal/daemon/engine_settings_test.go","internal/daemon/engine_shell_warn_test.go","internal/daemon/engine_stayonhub_test.go","internal/daemon/engine_test.go","internal/daemon/files_removed_test.go","internal/daemon/funnel_test.go","internal/daemon/image_config_test.go","internal/daemon/open_remote_session_url_test.go","internal/daemon/path.go","internal/daemon/path_test.go","internal/daemon/plugin_settings.go","internal/daemon/process.go","internal/daemon/process_test.go","internal/daemon/relay_remote_files.go","internal/daemon/remote_caps.go","internal/daemon/remote_files.go","internal/daemon/remote_ws_proxy_test.go","internal/daemon/search_config_test.go","internal/daemon/settings_removed_test.go","internal/daemon/socket.go","internal/daemon/socket_test.go","internal/daemon/types.go","internal/daemon/web_links_config_test.go","internal/pty/native.go","internal/pty/reap_other.go","internal/pty/session.go","internal/relay/fuzz_test.go","internal/relay/hub.go","internal/relay/hub_chatsend_test.go","internal/relay/protocol.go","internal/relay/protocol_chat_test.go","internal/relay/sanitize.go","internal/relay/sanitize_test.go","internal/relay/server.go","internal/relay/server_chatsend_test.go","internal/relay/server_cors_test.go","internal/relay/server_identity_test.go","internal/relay/server_inject_test.go","internal/relay/server_test.go","internal/statusbar/bar.go","internal/statusbar/bar_test.go","internal/tailnet/sessions.go","internal/tailnet/tailnet_test.go","internal/webserver/capability_mw.go","internal/webserver/capability_test.go","internal/webserver/capability_test_helpers.go","internal/webserver/chat.go","internal/webserver/files_removed_test.go","internal/webserver/funnel_test.go","internal/webserver/fuzz_test.go","internal/webserver/inject_test.go","internal/webserver/rwgate_test.go","internal/webserver/server.go","internal/webserver/server_chatsend_test.go","internal/webserver/server_test.go","internal/webserver/sessions_meta_embed_test.go","internal/webserver/sessions_meta_test.go","internal/webserver/tailscale.go","internal/webserver/tailscale_test.go","internal/webserver/testdata/fuzz/FuzzWebServerRoutes/fbdee7071c7665ef","main.go","notification_darwin.go","notification_darwin_test.go","tests/build-script.test.sh","tests/check-traceability-paths.sh","tests/fixtures/settings_agenthub_v4.2.json","tools.go","tray.go","tray_objc_darwin.m","tray_test.go"]
covered_digest: "v4:sha256:685a12891246a4cc67beb7ede27319a6c0deac347678999a653485c91d37ccce"
behavior_unverified: 2
overrides_applied: 0
behavior_unverified_items:
  - truth: "SC6 / TERM-05: When a shell exits, its tab closes immediately, with no status dot, exit countdown, toast or banner, including for shells that exit hours later"
    test: "In the built app open three sessions; run `exit`, `exit 3`, and `sleep 400; exit 0` (wait ~7 min). Also exit a shell from another terminal via `agenthub kill <id>`."
    expected: "Each tab closes within ~1 s of the exit, including the >5 min one; no toast, countdown, Keep Open, error tab or status dot appears; `agenthub list` no longer lists the session."
    why_human: "The Go exit watcher is behaviorally tested (app_exit_test.go), but the React half (session:exit -> removeTabStateRef -> tab removed) is covered only by source-inspection tests (App.shellExit.test.tsx reads App.tsx?raw). No test mounts the app and observes the tab disappear."
  - truth: "SC7 / NET-01: With Tailscale disconnected a banner says remote access and sharing are off and Share/remote controls are disabled; reconnecting clears the banner without a restart"
    test: "Disconnect Tailscale from its menu bar app, wait <=10 s or refocus the window; then reconnect and refocus."
    expected: "Banner reads exactly \"Tailscale isn't connected. Remote access and sharing are off until it is.\" with no dismiss button; footer and Hub-card Share buttons disabled with that tooltip; open Share dialog closes; no peer cards/hints/join-code entry points. On reconnect the banner clears, Share re-enables and peers return, with no restart."
    why_human: "CheckHealth decision logic and the 10 s emit-on-change poller are behaviorally tested in Go, and StatusBar/SessionCard disabled states are DOM-tested, but the App-level transition (tailscale:health event -> tailscaleOff -> banner mount/unmount, peers cleared/restored, focus recheck) is covered only by source inspection (App.tailscale.test.tsx)."
human_verification:
  - test: "M-55 / M-50 (SC6, TERM-05) - tab closes on every shell exit at any age (see behavior_unverified_items[0])"
    expected: "Tab closes immediately for exit 0, non-zero and killed shells, including one that lived >5 minutes; no exit/status UI anywhere; quit dialog lists names only; Settings has no auto-close toggle"
    why_human: "Needs the native webview, a live daemon and real wall-clock time past the old 300 s window"
  - test: "M-57 + M-40 (SC7, NET-01) - live Tailscale disconnect/reconnect (see behavior_unverified_items[1]); also confirm Settings -> Web Server shows Connected/Not connected only, and Funnel toggle follows web-server mode"
    expected: "Banner/disabled controls while off; everything restored on reconnect without restart; Portty never offers to install Tailscale"
    why_human: "Needs a real Tailscale client to toggle, a second tailnet device and native focus events"
  - test: "M-56 (SC2, TERM-02) - open two sessions, run `sleep 20; exit` in one and print output in the other, Quit GUI only, wait 30 s, reopen from tray; then fully quit the GUI and relaunch from Finder"
    expected: "Each time the running session is restored as a tab with its scrollback visible in xterm; the session that exited while hidden is gone"
    why_human: "Daemon persistence + scrollback tail are proven by TestExitWatcher_GUIRelaunchKeepsRunningSessions, but GUI tab restore and xterm scrollback rehydration in the real webview are not exercised by any test"
  - test: "M-54 (SC1, TERM-01) - New Session modal: only Working Directory picker; long path truncates with ellipsis in a 460 px modal; Create Session yields a shell answering `[[ -o login ]] && echo login; echo $ZSH_VERSION` with `login` and a zsh version even after `chsh -s /bin/bash`; last-used directory is preselected on next open"
    expected: "Login zsh in the chosen/last-used/home directory; no shell line, no Settings -> Paths section, no agent picker anywhere"
    why_human: "Visual layout (truncation is a backstop truth) and the real login-shell environment in the bundled app"
  - test: "SC3 / TERM-04 - in Settings change the terminal theme and toggle each plugin (GPU render/WebGL, search, web links, image/sixel, clipboard/OSC 52, OSC 9;4 progress) with terminals open"
    expected: "Open terminals re-theme live; each plugin toggle takes effect (find bar, clickable links, sixel image renders, OSC 52 copy, progress underline + tray quartile glyph)"
    why_human: "Visual rendering in the native webview; unit tests cover state plumbing only"
  - test: "M-23 (SC4, CHAT-01 desktop) - in a live session send `@session echo hi` via Enter and via Send"
    expected: "Both appear as ordinary chat messages; mention popover lists only participants (no Agent section); nothing is typed into the terminal"
    why_human: "Live desktop chat UI; the zero-PTY-bytes guarantee is unit-tested (server_inject_test.go, inject_test.go) but the user-visible flow is not"
  - test: "M-24 (SC4, CHAT-01 web) - web-share a session, open the link in a browser (UAT will drive this with the dev-browser skill), chat from both desktop and browser incl. `@session echo hi` and an @alias mention"
    expected: "Both sides see every message; no inject marker; nothing reaches the terminal; presence, typing, unread badge and export work"
    why_human: "Cross-surface live chat parity; Playwright chat-parity.spec.ts exists but was not re-run in this verification (needs the fixture server and browsers)"
  - test: "M-16 - one-time web-share warning: first web-share of any session shows the warning with \"Keep sharing off\" (focused on mount, Esc works); later shares do not; re-arming in Settings brings it back"
    expected: "Warning fires exactly once per arm, for every session type"
    why_human: "Live share flow in the native app"
  - test: "M-41 - native notification path: with a Developer-ID-signed build, Settings -> Behavior shows only 'Start minimized to system tray'; choose Quit GUI only with a session open"
    expected: "Exactly one native notification '... is still running in the background. N session(s) active.'; tray icon stays"
    why_human: "UNUserNotificationCenter delivery needs a signed bundle and a live desktop session"
  - test: "Review-fix logic/state changes flagged by the fixer (iteration 1: CR-01 Tailscale health recovery after a not-connected recheck, WR-01 exit reported for sessions removed without a stopped sighting e.g. `agenthub kill` or daemon restart, WR-02 watcher removes stopped sessions itself, WR-03 shell that exits before its tab is added leaves no dead tab, WR-05 RetryDaemon after a failed startup no longer spawns duplicate pollers; iteration 2: WR-01 init does not overwrite a newer tailscale:health event, WR-02 init does not restore a tab for a session that exited while Promise.all was pending, WR-03 session:exit path is UI-only with no redundant KillSession warn)"
    expected: "Kill a session from the CLI -> its tab closes; restart the daemon -> stale tabs close; start the app with the daemon down, click Retry, then confirm a single health/exit poller (no duplicate events); toggle Tailscale during app launch -> banner reflects the final state; `exit` immediately after Create Session -> no tab is left behind; no 'KillSession failed' warn in the console on ordinary exits"
    why_human: "Race/ordering fixes; Go halves have regression tests but the timing windows in the live app cannot be proven from code"
---

# Phase 1: Plain macOS Terminal Verification Report

**Phase Goal:** The app is a plain macOS terminal: new sessions run macOS zsh (`/bin/zsh -l`), no agent-CLI, file-browser, or Windows/Linux code remains, and every kept feature still works
**Verified:** 2026-11-15T03:55:00Z
**Status:** human_needed
**Re-verification:** No - initial verification

> **MVP-mode note.** ROADMAP marks this phase `Mode: mvp`, but the goal is not in User Story form (`gsd query user-story.validate` returns `valid: false`). Rather than refuse, this report uses standard goal-backward verification against the 7 roadmap success criteria (which are already phrased as user-observable outcomes). If MVP-mode User Flow Coverage is required, run `/gsd-mvp-phase 1` to rewrite the goal, or drop `Mode: mvp` for Phase 1.

## Goal Achievement

### Observable Truths (Roadmap Success Criteria - the contract)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | New session -> pick dir (default last-used) -> `/bin/zsh -l`; no shell choice, no Settings -> Paths; no agent picker/badges/status or file browser in desktop or web share | VERIFIED | `internal/daemon/engine.go:220,245,262-288` fixed `sessionShell="/bin/zsh"`, argv `["-l"]`, caller args never reach spawn (`types.go:24-25` ignores legacy `cli`/`args`). Behavioral: `TestCreateSession_LoginShellArgv`, `TestCreateSession_IgnoresShellEnvAndLegacySetting`, real-PTY `TestCreateSession_LoginShell` (SHELL=/bin/bash still yields zsh login). `NewSessionModal.tsx` has only Working Directory + `agenthub:lastWorkDir` localStorage default. SettingsTab sections: Behavior, Session Behavior, Appearance, Web Server, Security, Plugins (no Paths). No `GetShellPath`/`GetCLIPaths`/`DetectCLIs`/`ListShells` anywhere; no agent names, badges or status UI in `frontend/src` (grep); `internal/files`, `internal/status`, FileBrowser/Editor/CodeMirror all gone. CLI `cmd_cli.go:64-99` `new [<path>]` only. |
| 2 | Quit/relaunch (or close/reopen window) keeps every running session with scrollback | VERIFIED | Daemon owns sessions; `TestExitWatcher_GUIRelaunchKeepsRunningSessions` (app_exit_test.go:177) runs a real shell, cancels the first GUI's watcher, attaches a second App to the same daemon, asserts state `running`, scrollback marker present and zero exit events. Frontend restore (`App.tsx:441-460`, `retryInit`) filters only `stopped` sessions. Live GUI rehydration routed to human (M-56). |
| 3 | Theme change and xterm plugin toggles (GPU, search, links, sixel, OSC 52, OSC 9;4) apply to open terminals | VERIFIED | `PluginsSection.tsx` keys `webgl, search, webLinks, image, clipboard, progress, unicode11`; OpenCode SIGUSR2 broadcast gone (no `notify_theme_*`). `TabBar.tsx:271-279` progress underline fed by `App.tsx:1356 tabProgress={tabProgress}`; `app.go:982 SetTrayProgress` quartile glyph. Tests pass: `App.test.tsx` "TERM-04: terminal theme change re-themes open terminals", `TerminalPanel.test.tsx` THM-03, PluginsSection suites, e2e `progress.spec.ts`/`web-plugin-hot-swap.spec.ts` present. Visual check routed to human. |
| 4 | Chat works desktop <-> browser; nothing typed in chat (incl. `@session`) reaches the terminal | VERIFIED | `relay/protocol.go:80-82` 0x35/0x36 RETIRED; both read pumps (`relay/server.go:352`, `webserver/server.go:1501`) have no case for 0x35. Behavioral: `TestInjectFrame_RO_RelayPath_NoPTYWrite`, `TestInjectFrame_RW_RelayPath_NoPTYWrite`, `TestInjectFrame_ChatSendAndStrayFrame_NoPTYWrite`, `TestInjectFrame_WebPath_NoPTYWrite`, `TestChatStore_LegacySessionInjectLine`. `ChatPanel.tsx:615-634` every send via `sendChat`; no inject route on daemon. Cross-surface live parity routed to human (M-23/M-24). |
| 5 | macOS Go build, Go tests, Vitest, lint pass; no Win/Linux sources/packaging/CI; no files API/capability; fuzz harness covers relay + webserver | VERIFIED | Run by verifier: `go build ./... && go vet ./...` OK; `go test -race -short ./...` all packages ok; `tsc --noEmit` OK; `vitest run` 117 files / 1824 tests passed; traceability check 120 paths OK; Linux fixture cross-compile OK; six fuzz targets (FuzzParseFrame, FuzzInputAbsorberFilter, FuzzSanitizeChatContent, FuzzRequireCapability, FuzzWebServerRoutes, FuzzVerify) each clean for 5 s. Lint gate = `go vet` + `tsc` (RESEARCH); golangci-lint reports 68 issues vs 72 pre-existing baseline (not a gate, no growth). No tracked `*_windows.go`/`*_linux.go`/`win32input*`/`*_nonwindows.go`; `packaging/winget`, `build/linux`, `build/windows`, `scripts/install.sh` gone; build.yml single macOS matrix row; release.yml has no Windows/Linux build jobs; distribute.yml has no winget job. Capabilities minted only as `read` / `read,write` (`api.go:1262-1263`); `files_removed_test.go` (daemon + webserver) assert 404/405 for `/api/files/*` and `/browse`. |
| 6 | Shell exit closes its tab immediately, no status dot/countdown/toast/banner, incl. hours later | PRESENT_BEHAVIOR_UNVERIFIED | Go half behaviorally VERIFIED: `app.go:301-347` one watcher, 500 ms, no deadline, removes stopped sessions and emits `session:exit` once; tests `TestCheckExits_EmitsOnceWhenShellExits`, `_LongLivedSessionStillReported`, `_StoppedAtFirstSight`, `_RemovesStoppedSession`, `_ReportsRemovedWithoutStoppedSighting`. UI absence VERIFIED by grep (no ExitToast, countdown, Keep Open, hubStatus, tab dot, attention). React half (`App.tsx:617-627` -> `removeTabStateRef`) is tested only by source inspection. See Human Verification. |
| 7 | Tailscale disconnected -> banner, Share/remote disabled; reconnect clears without restart; never installs/configures Tailscale | PRESENT_BEHAVIOR_UNVERIFIED | `internal/webserver/tailscale.go:31-38,71` LocalAPI only, connected iff `BackendState=="Running"`; no `brew`, `LookPath`, `exec`, `macsys` in production code. `app.go:1264-1290` 10 s poller emits `tailscale:health` on change (`TestCheckTailscaleHealth_EmitsFirstResultAndChanges`, `TestCheckHealth_*`). `App.tsx:133 tailscaleOff`, `:1287 <TailscaleBanner/>` (no dismiss), `:948-949` peers cleared + poll skipped, `:977` Share dialog closed, `:992-993` focus/visibility recheck, `:1412,:1586 shareDisabledReason` -> StatusBar/SessionCard `disabled` (DOM-tested). App-level transition is source-inspected only. See Human Verification. |

**Score:** 5/7 truths verified (2 present, behavior-unverified)

### Superseded plan must-haves (not counted; superseded by CONTEXT amendments)

These PLAN frontmatter items were written before the two `<amendment>` blocks in 01-CONTEXT.md and were deliberately replaced by plans 01-10..01-12. Their absence in code is the intended outcome, confirmed below.

| Plan | Superseded must-have | Superseded by | Current code |
|------|---------------------|---------------|--------------|
| 01-03 | "tailscale override, shellPath and notify toggle survive" legacy load | Amendment 1/2, 01-11 | Keys load and are dropped on save (`TestSettingsMigration_LegacyAgentHubV42`) |
| 01-04 | Read-only Shell line, "Default login shell" pending/fallback text, helper "Change it in Settings -> Paths" | Amendment 1, 01-11 | Modal shows only Working Directory (verify.artifacts miss on this pattern is expected) |
| 01-05 | `resolveDefaultShellPath` ($SHELL / settings), Paths table shell+tailscale rows, tailscale binary override | Amendment 1, 01-11/01-12 | Fixed `/bin/zsh`; no Paths section; LocalAPI-only Tailscale |
| 01-06 | Running/Exited/Error tab dots, Hub status labels, exit toast + Keep Open, `hubStatus.ts`, `session:status` event | Amendment 2, 01-10 | All removed; daemon `Status` field kept internally only |

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `tests/check-traceability-paths.sh` | Portable check, fails on zero paths | VERIFIED | `pipefail`; ran: 120 paths OK |
| `.github/workflows/build.yml` | Single macOS job | VERIFIED | `macos-latest`, runs Go race tests, build-script tests, frontend tests, traceability |
| `internal/pty/reap_other.go` | Untagged ReapNaturalExit (darwin + linux fixture) | VERIFIED | Present; only `_other.go` left, intentionally |
| `frontend/src/components/TabBar.tsx` | No file-browser tab type | VERIFIED | |
| `frontend/src/components/SessionShare/TailnetTab.tsx` | UI-SPEC link copy | VERIFIED | |
| `internal/daemon/files_removed_test.go`, `internal/webserver/files_removed_test.go` | Negative route tests | VERIFIED | Pass in suite |
| `tests/fixtures/settings_agenthub_v4.2.json` | Legacy settings fixture | VERIFIED | Carries filesWrite/cliPaths/notifyOnWaiting/shellPath/autoCloseSession |
| `internal/daemon/engine.go` | Fixed `/bin/zsh -l` spawn + test seam | VERIFIED | `sessionShell`, `SetSpawnCommandForTest` |
| `frontend/src/wailsjs/go/main/App.d.ts` | `CreateSession(name, workDir, cols, rows)` | VERIFIED | |
| `frontend/src/components/ShellWebShareBanner.tsx` | "Keep sharing off" | VERIFIED | |
| `frontend/src/components/Hub/SessionShareModal.tsx` | enabled && !warned gate | VERIFIED | |
| `internal/webserver/server.go` | resolver `ok bool`, 404 on unknown | VERIFIED | |
| `internal/tailnet/sessions.go` | Peer meta without cli_type | VERIFIED | |
| `internal/relay/fuzz_test.go`, `internal/webserver/fuzz_test.go` | Fuzz targets | VERIFIED | Ran clean |
| `internal/relay/protocol.go` | RETIRED note | VERIFIED | |
| `app.go` `startExitWatcher` | App-wide watcher | VERIFIED | |
| `app_exit_test.go` | `TestCheckExits_*` | VERIFIED | 6 tests, pass |
| `internal/daemon/settings_removed_test.go` | Removed settings routes 404/405 | VERIFIED | auto-close, shell-path, cli-paths, notify-on-waiting |
| `notification_darwin.go` | `func sendNotification` minimal path | VERIFIED | Wired via `app.go:117 sendNotificationFunc` and used by QuitGUIOnly (`app.go:254`) |
| `internal/webserver/tailscale.go` | `func CheckHealth` LocalAPI only | VERIFIED | |
| `frontend/src/components/TailscaleBanner.tsx`, `frontend/src/lib/tailscale.ts` | Banner + single sentence constant | VERIFIED | |
| 01-04/01-05/01-06 artifacts | (see Superseded table) | SUPERSEDED | verify.artifacts misses are expected |

### Key Link Verification

| From | To | Via | Status |
|------|----|-----|--------|
| `engine.go` | `reap_other.go` | `ReapNaturalExit()` | WIRED (tool) |
| build.yml | traceability script | CI step | WIRED (tool) |
| App.tsx | WebShareSessionView / `open-session` intent | web-session, ExchangeJoinCodeAtURL | WIRED (tool) |
| chat_routes.go | relay `FilesCORS`/`FilesPreflight` | chat CORS | WIRED (tool) |
| relay_remote_files.go | remote_caps.go | remote WS proxy | WIRED (tool) |
| webserver read pump | capability `HasPerm(...,"write")` | readonly derivation | WIRED (tool) |
| NewSessionModal -> App.tsx -> app.go -> api.go -> engine.go | CreateSession chain | onConfirm(workDir) | WIRED (tool) |
| SessionShareModal -> ShellWebShareBanner | warning gate | WIRED (tool) |
| api.go -> webserver | `SetSessionResolver` found flag | WIRED (tool) |
| engine.go -> relay hub | `SetChatAppendFn` | WIRED (tool) |
| ChatPanel -> relayClient | `sendChat` only | WIRED (tool) |
| app.go `checkExits` -> App.tsx | `session:exit` | WIRED (manual: emit `app.go:344`, handler `App.tsx:617`) |
| App.tsx init/retryInit -> daemon | `KillSession` of stopped | WIRED (manual: `App.tsx:451`) |
| App.tsx progress -> TabBar | `tabProgress` | WIRED (manual: `App.tsx:1356`, `TabBar.tsx:277`) |
| engine.go `spawnCommand` -> pty backend | `CreateRequest{CLI, Args}` | WIRED (manual: `engine.go:262,283`) |
| NewSessionModal -> CreateSession | `onConfirm(workDir)` only | WIRED (tool) |
| app.go QuitGUIOnly -> `sendNotification` | `sendNotificationFunc` | WIRED (manual: `app.go:117,254`) |
| tailscale.go CheckHealth -> App.tsx | `tailscale:health` | WIRED (manual: `app.go:1272`, `App.tsx:545`) |
| App.tsx `tailscaleOff` -> StatusBar/HubPanel -> SessionCard | `shareDisabledReason` | WIRED (manual: `App.tsx:1412,1586`, `HubPanel.tsx:462`, `SessionCard.tsx:501`) |
| App.tsx `tailscaleOff` -> remote-peer poll | `setRemotePeers([])` | WIRED (manual: `App.tsx:949`) |
| 01-05/01-06 links (`GetShellPath`, `session:status`, `hubStatus`) | - | SUPERSEDED | Removed by Amendments 1/2 |

The 01-10/01-11/01-12 key links failed in `verify.key-links` only because their `from:` fields contain a symbol name after the path; each was checked by hand above.

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| TailscaleBanner / Share buttons | `tailscaleHealth` -> `tailscaleOff` | `GetTailscaleStatus()` at init + `tailscale:health` events from `checkTailscaleHealth` -> `webserver.CheckHealth` -> LocalAPI `Status()` | Yes | FLOWING |
| Tabs (exit) | `session:exit` sessionId | `checkExits` -> `client.ListSessions()` daemon state | Yes | FLOWING |
| Restored tabs | `allSessions` | `ListSessions()` -> daemon registry | Yes | FLOWING |
| Tab progress underline | `tabProgress` | OSC 9;4 parsed in TerminalPanel -> `handleProgressChange` | Yes | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Go build + vet | `go build ./... && go vet ./...` | OK | PASS |
| Go tests (race) | `go test -race -short ./...` | all 11 test packages ok (exit 0) | PASS |
| TS typecheck | `pnpm exec tsc --noEmit` | exit 0 | PASS |
| Vitest | `pnpm exec vitest run` | 117 files, 1824 tests passed | PASS |
| Traceability | `bash tests/check-traceability-paths.sh` | `OK: 120 traceability paths exist` | PASS |
| Linux e2e fixture cross-compile | `GOOS=linux GOARCH=amd64 go build -tags playwrightfixture ./cmd/playwright-fixture ./internal/...` | no errors | PASS |
| Fuzz targets (6) | `go test -run '^$' -fuzz ^<T>$ -fuzztime 5s` | all PASS | PASS |
| golangci-lint (advisory) | `golangci-lint run ./...` | 68 issues (50 errcheck, 16 staticcheck, 2 unused) vs 72 baseline | INFO |
| Summary commit hashes | `git cat-file -e` on 33 cited hashes | 33/33 exist | PASS |
| Playwright chromium suite | not run (needs fixture server + browsers) | - | SKIP (human/UAT) |
| `wails build` | not run (writes build/bin) | - | SKIP (claimed in 01-12-SUMMARY) |

### Probe Execution

Step 7c: SKIPPED (no `scripts/*/tests/probe-*.sh` declared or present for this phase).

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|----------------|-------------|--------|----------|
| STRP-01 | 01-04..01-08, 01-10, 01-11 | All agent-CLI code removed | SATISFIED | No detect.go/shells.go/internal/status/notify_theme; no cli/cli_type on wire (decoders tolerant); no agent UI; grep clean of claude/codex/gemini/antigravity/opencode in production code |
| STRP-02 | 01-02, 01-03 | File browser/editor removed incl. daemon API and capability | SATISFIED | `internal/files` gone; negative route tests; perms minted `read`/`read,write` only; no FileBrowser/Editor/CodeMirror |
| STRP-03 | 01-01 | Win/Linux code, packaging, CI removed; build/test/lint pass on macOS | SATISFIED | See SC5 |
| STRP-04 | all plans | Removed-feature tests deleted, kept tests pass, fuzz harness covers relay + webserver | SATISFIED | Suites green; 5 new relay/webserver fuzz targets + FuzzVerify run clean |
| TERM-01 | 01-04, 01-05, 01-11 | New session always `/bin/zsh -l` in chosen/last-used dir | SATISFIED | See SC1 |
| TERM-02 | 01-10 | Sessions survive GUI close/restart with scrollback | SATISFIED (live GUI: human M-56) | See SC2 |
| TERM-04 | 01-04 | Themes + plugins keep working from Settings | SATISFIED (visual: human) | See SC3 |
| TERM-05 | 01-10 | Shell exit closes tab immediately at any age, no exit UI | SATISFIED in code; live behavior NEEDS HUMAN | See SC6 |
| NET-01 | 01-12 | LocalAPI check, banner, disabled share/remote, auto recheck, never installs | SATISFIED in code; live behavior NEEDS HUMAN | See SC7 |
| CHAT-01 | 01-09 | Chat across desktop/web, `@session` inject removed | SATISFIED (live parity: human M-23/M-24) | See SC4 |

All 10 phase requirement IDs appear in at least one plan's `requirements:` field. REQUIREMENTS.md maps no other IDs to Phase 1, so there are no orphaned requirements.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (all 206 changed files) | - | TBD/FIXME/XXX | none found | Debt-marker gate clean |
| `app.go` | 725-740 | `StartWebServer` falls back to LAN (`local`) mode while the NET-01 banner says sharing is off (review IN-04) | Warning | The Share buttons are disabled, so no new session can be shared, but the Settings "Start Web Server" control is not disabled while Tailscale is off. Reviewer deferred to Phase 4, but no Phase 4 success criterion names it. **Human decision:** disable the button while `tailscaleOff` now, or accept it as a Phase 4 item |
| `frontend/src/components/TerminalPanel.tsx` | 965-969 | Guest-only `SessionEndedBanner` still shows when a *remote* viewer's session ends | Info / decision | Plan 01-10 scoped TERM-05 to local sessions, and review WR-01 deliberately kept this banner. SC6's wording ("its tab closes ... no banner") does not exclude remote tabs. Confirm remote-tab exit behavior is Phase 4/5 scope |
| `web/assets/terminal.js` | 58 | Dead `meta.cli_type` branch (webserver no longer sends it) | Info | Harmless leftover |
| `app.go` | 244 | `QuitGUIOnly` counts by `Status != "stopped"` (review IN-02) | Info | The quit-to-tray notice can over-count for at most about 500 ms |
| `internal/pty/*_test.go`, `internal/pty/cleanup.go`, `engine_natural_exit_test.go` | 1 | `//go:build !windows` tags; `capability/keystore_test.go:35` `GOOS != "windows"` | Info | Leftover tags on darwin-compiled files, not Windows sources |
| `frontend/src/content/help/chat.md`, `web/join.html:57` | - | "agent" wording in user-facing copy | Info (deferred) | Phase 3 BRND-04/SC4 covers removing agent-CLI wording |
| Review IN-01, IN-03, IN-05..IN-07 | - | Stale comments, Linux fixture lacks exit detection, traceability parser edge, orphaned `GetSessionQRCode` binding, `daemonErr` race | Info | Out of fix scope; carried in 01-REVIEW.md |
| Review WR-02 (necessity) | `App.tsx:450-452` | Frontend `KillSession` of stopped sessions duplicates the Go watcher | Info | Necessity question for the human; harmless |

### Human Verification Required

UAT for browser-checkable items (M-24 web chat, web-share view) will use the dev-browser skill.

1. **M-55 / M-50: tab closes on every shell exit (SC6, TERM-05).** In the built app, exit three shells: `exit`, `exit 3`, and `sleep 400; exit 0`. Also kill one with `agenthub kill`. Expected: every tab closes within about 1 s and no exit or status UI appears. Why human: the React tab removal is covered only by source-inspection tests.
2. **M-57 + M-40: live Tailscale disconnect and reconnect (SC7, NET-01).** Expected: while disconnected, the exact banner shows, Share is disabled with the same tooltip, the dialog closes and peers are hidden. Reconnecting restores everything without a restart. Why human: needs a real Tailscale client and native focus events.
3. **M-56: GUI hide/relaunch persistence (SC2, TERM-02).** Expected: running sessions come back with scrollback, and a session that exited while hidden is gone. Why human: tab restore and xterm rehydration in the real webview.
4. **M-54: New Session modal and login zsh (SC1, TERM-01).** Expected: only the directory picker shows and long paths truncate. The shell is a zsh login shell even with `chsh -s /bin/bash`, and the last-used directory is preselected. Why human: visual layout plus the real login environment.
5. **Theme and plugin live toggles (SC3, TERM-04).** Expected: each toggle takes visible effect in open terminals. Why human: visual rendering.
6. **M-23: desktop chat never reaches the PTY (SC4).** Why human: live chat UI.
7. **M-24: web-share chat parity (SC4), via dev-browser.** Why human: cross-surface live flow. The Playwright `chat-parity.spec.ts` was not re-run here.
8. **M-16: one-time web-share warning.** Why human: live share flow.
9. **M-41: native notification on "Quit GUI only" (signed build).** Why human: needs a signed bundle.
10. **Review-fix race/ordering fixes.** Iteration 1: CR-01, WR-01, WR-02, WR-03, WR-05. Iteration 2: WR-01, WR-02, WR-03. Exercise: CLI kill, daemon restart, Retry after the daemon is down, toggling Tailscale during launch, and `exit` right after Create Session. Why human: timing windows in the live app.

### Gaps Summary

No blocking gaps. Every artifact the active plans require exists, is substantive and is wired. All four automated gates (Go build/vet/race tests, tsc, Vitest, traceability) pass when run by the verifier, the six fuzz targets run clean, and no Windows/Linux sources, packaging or CI jobs remain. The files API and capability are gone and covered by negative route tests. Every session spawns a fixed `/bin/zsh -l`, which real-PTY tests prove. The chat-to-PTY inject path is retired, and tests on both WebSocket paths prove it writes zero bytes to the PTY.

Two success criteria (SC6 tab-close-on-exit, SC7 Tailscale banner transitions) are implemented and wired, and their Go halves are behaviorally tested. Their React halves are verified only by source inspection, so they stay PRESENT_BEHAVIOR_UNVERIFIED until the live UAT items M-55/M-50 and M-57/M-40 pass.

Two items need a human decision, not a fix-plan:
1. The Settings "Start Web Server" control can still start LAN mode while Tailscale is off (review IN-04).
2. The guest-only "session ended" banner on remote tabs, against SC6's literal wording.

The phase is also tagged `Mode: mvp` with a non-User-Story goal (see note at top).

---

_Verified: 2026-11-15T03:55:00Z_
_Verifier: Claude (gsd-verifier)_
