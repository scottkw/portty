---
phase: 01-plain-macos-terminal
verified: 2026-11-15T15:55:00Z
status: passed
score: 6/6 must-haves verified
covered_files: [".github/workflows/build.yml",".github/workflows/distribute.yml",".github/workflows/release.yml",".gitignore",".planning/phases/01-plain-macos-terminal/01-01-PLAN.md",".planning/phases/01-plain-macos-terminal/01-01-SUMMARY.md",".planning/phases/01-plain-macos-terminal/01-02-PLAN.md",".planning/phases/01-plain-macos-terminal/01-02-SUMMARY.md",".planning/phases/01-plain-macos-terminal/01-03-PLAN.md",".planning/phases/01-plain-macos-terminal/01-03-SUMMARY.md",".planning/phases/01-plain-macos-terminal/01-04-PLAN.md",".planning/phases/01-plain-macos-terminal/01-04-SUMMARY.md",".planning/phases/01-plain-macos-terminal/01-05-PLAN.md",".planning/phases/01-plain-macos-terminal/01-05-SUMMARY.md",".planning/phases/01-plain-macos-terminal/01-06-PLAN.md",".planning/phases/01-plain-macos-terminal/01-06-SUMMARY.md",".planning/phases/01-plain-macos-terminal/01-07-PLAN.md",".planning/phases/01-plain-macos-terminal/01-07-SUMMARY.md",".planning/phases/01-plain-macos-terminal/01-08-PLAN.md",".planning/phases/01-plain-macos-terminal/01-08-SUMMARY.md",".planning/phases/01-plain-macos-terminal/01-09-PLAN.md",".planning/phases/01-plain-macos-terminal/01-09-SUMMARY.md",".planning/phases/01-plain-macos-terminal/01-10-PLAN.md",".planning/phases/01-plain-macos-terminal/01-10-SUMMARY.md",".planning/phases/01-plain-macos-terminal/01-11-PLAN.md",".planning/phases/01-plain-macos-terminal/01-11-SUMMARY.md",".planning/phases/01-plain-macos-terminal/01-12-PLAN.md",".planning/phases/01-plain-macos-terminal/01-12-SUMMARY.md","README.md","TESTING.md","app.go","app_exit_test.go","app_save_terminal_test.go","app_test.go","build.sh","cmd/playwright-fixture/main.go","cmd_attach.go","cmd_attach_test.go","cmd_cli.go","cmd_cli_test.go","cmd_remote.go","cmd_remote_test.go","frontend/e2e/chat-parity.spec.ts","frontend/e2e/fixture-env.ts","frontend/e2e/global-setup.ts","frontend/e2e/web-csp.spec.ts","frontend/e2e/web-share-scope.spec.ts","frontend/package.json","frontend/pnpm-lock.yaml","frontend/src/App.tsx","frontend/src/__tests__/themeTokens.test.ts","frontend/src/components/Hub/ChatMessage.test.tsx","frontend/src/components/Hub/ChatMessage.tsx","frontend/src/components/Hub/ChatPanel.test.tsx","frontend/src/components/Hub/ChatPanel.tsx","frontend/src/components/Hub/HubEmptyState.test.tsx","frontend/src/components/Hub/HubEmptyState.tsx","frontend/src/components/Hub/HubFilterBar.test.tsx","frontend/src/components/Hub/HubFilterBar.tsx","frontend/src/components/Hub/HubInteractiveModal.test.tsx","frontend/src/components/Hub/HubModal.test.tsx","frontend/src/components/Hub/HubModal.tsx","frontend/src/components/Hub/HubPanel.test.tsx","frontend/src/components/Hub/HubPanel.tsx","frontend/src/components/Hub/MentionPopover.test.tsx","frontend/src/components/Hub/MentionPopover.tsx","frontend/src/components/Hub/SessionCard.test.tsx","frontend/src/components/Hub/SessionCard.tsx","frontend/src/components/Hub/SessionCardGrid.test.tsx","frontend/src/components/Hub/SessionCardGrid.tsx","frontend/src/components/Hub/SessionShareModal.tsx","frontend/src/components/Hub/TerminalChatHost.tsx","frontend/src/components/Hub/useChatUnreadListeners.test.tsx","frontend/src/components/NewSessionModal.tsx","frontend/src/components/QuitConfirmModal.tsx","frontend/src/components/RemoteJoinCodeModal.test.tsx","frontend/src/components/RemoteJoinCodeModal.tsx","frontend/src/components/SessionShare/TailnetTab.tsx","frontend/src/components/SettingsJumpBar.tsx","frontend/src/components/SettingsSearch.tsx","frontend/src/components/SettingsTab.tsx","frontend/src/components/ShellWebShareBanner.tsx","frontend/src/components/Sidebar.tsx","frontend/src/components/StatusBar.tsx","frontend/src/components/TabBar.tsx","frontend/src/components/TailscaleBanner.tsx","frontend/src/components/WelcomeTab.tsx","frontend/src/components/__tests__/App.createTab.stayOnHub.test.tsx","frontend/src/components/__tests__/App.hub.test.tsx","frontend/src/components/__tests__/App.open-remote.test.tsx","frontend/src/components/__tests__/App.shellExit.test.tsx","frontend/src/components/__tests__/App.shellWebShare.test.tsx","frontend/src/components/__tests__/App.tailscale.test.tsx","frontend/src/components/__tests__/App.test.tsx","frontend/src/components/__tests__/NewSessionModal.test.tsx","frontend/src/components/__tests__/QuitConfirmModal.test.tsx","frontend/src/components/__tests__/SessionCard.share.test.tsx","frontend/src/components/__tests__/SessionShareModal.disconnect.test.tsx","frontend/src/components/__tests__/SessionShareModal.test.tsx","frontend/src/components/__tests__/SettingsTab.appearance-theme.test.tsx","frontend/src/components/__tests__/SettingsTab.hyperlinked-index.test.tsx","frontend/src/components/__tests__/SettingsTab.persistence.test.tsx","frontend/src/components/__tests__/SettingsTab.shell-warn-toggle.test.tsx","frontend/src/components/__tests__/SettingsTab.stay-on-hub-toggle.test.tsx","frontend/src/components/__tests__/SettingsTab.tailscale-status.test.tsx","frontend/src/components/__tests__/SettingsTab.test.tsx","frontend/src/components/__tests__/ShellWebShareBanner.test.tsx","frontend/src/components/__tests__/Sidebar.test.tsx","frontend/src/components/__tests__/StatusBar.shareSession.test.tsx","frontend/src/components/__tests__/TabBar.test.tsx","frontend/src/components/__tests__/TailnetTab.test.tsx","frontend/src/components/__tests__/TailscaleBanner.test.tsx","frontend/src/components/__tests__/TerminalPanel.scale.test.tsx","frontend/src/components/__tests__/WelcomeTab.install.test.tsx","frontend/src/components/__tests__/WelcomeTab.test.tsx","frontend/src/components/__tests__/style.contrast.test.ts","frontend/src/components/__tests__/style.hub.modal.test.ts","frontend/src/components/__tests__/style.hub.test.ts","frontend/src/components/__tests__/style.redesign.test.ts","frontend/src/components/__tests__/style.settings.test.ts","frontend/src/content/help/chat.md","frontend/src/content/help/faq.md","frontend/src/content/help/getting-started.md","frontend/src/lib/__tests__/remoteAdapter.test.ts","frontend/src/lib/__tests__/remoteSession.test.ts","frontend/src/lib/hubGroupCounts.test.ts","frontend/src/lib/hubGroupCounts.ts","frontend/src/lib/relayClient.test.ts","frontend/src/lib/relayClient.ts","frontend/src/lib/remoteAdapter.test.ts","frontend/src/lib/remoteAdapter.ts","frontend/src/lib/remoteSession.ts","frontend/src/lib/tailscale.ts","frontend/src/lib/vtColor.ts","frontend/src/lib/webMode.ts","frontend/src/style.css","frontend/src/wailsjs/go/main/App.d.ts","frontend/src/wailsjs/go/main/App.js","frontend/src/wailsjs/go/models.ts","frontend/src/wailsjs/wailsjs/go/models.ts","go.mod","go.sum","internal/capability/capability.go","internal/capability/capability_test.go","internal/daemon/alias_store_test.go","internal/daemon/api.go","internal/daemon/api_shell_warn_test.go","internal/daemon/api_stayonhub_test.go","internal/daemon/api_test.go","internal/daemon/chat.go","internal/daemon/chat_routes.go","internal/daemon/chat_routes_test.go","internal/daemon/chat_test.go","internal/daemon/client.go","internal/daemon/client_test.go","internal/daemon/engine.go","internal/daemon/engine_exit_test.go","internal/daemon/engine_migration_test.go","internal/daemon/engine_natural_exit_test.go","internal/daemon/engine_plugins_test.go","internal/daemon/engine_settings_test.go","internal/daemon/engine_shell_warn_test.go","internal/daemon/engine_stayonhub_test.go","internal/daemon/engine_test.go","internal/daemon/files_removed_test.go","internal/daemon/funnel_test.go","internal/daemon/image_config_test.go","internal/daemon/open_remote_session_url_test.go","internal/daemon/path.go","internal/daemon/path_test.go","internal/daemon/plugin_settings.go","internal/daemon/process.go","internal/daemon/process_test.go","internal/daemon/relay_remote_files.go","internal/daemon/remote_caps.go","internal/daemon/remote_files.go","internal/daemon/remote_ws_proxy_test.go","internal/daemon/search_config_test.go","internal/daemon/settings_removed_test.go","internal/daemon/socket.go","internal/daemon/socket_test.go","internal/daemon/types.go","internal/daemon/web_links_config_test.go","internal/pty/native.go","internal/pty/reap_other.go","internal/pty/session.go","internal/relay/fuzz_test.go","internal/relay/hub.go","internal/relay/hub_chatsend_test.go","internal/relay/protocol.go","internal/relay/protocol_chat_test.go","internal/relay/sanitize.go","internal/relay/sanitize_test.go","internal/relay/server.go","internal/relay/server_chatsend_test.go","internal/relay/server_cors_test.go","internal/relay/server_identity_test.go","internal/relay/server_inject_test.go","internal/relay/server_test.go","internal/statusbar/bar.go","internal/statusbar/bar_test.go","internal/tailnet/sessions.go","internal/tailnet/tailnet_test.go","internal/webserver/capability_mw.go","internal/webserver/capability_test.go","internal/webserver/capability_test_helpers.go","internal/webserver/chat.go","internal/webserver/files_removed_test.go","internal/webserver/funnel_test.go","internal/webserver/fuzz_test.go","internal/webserver/inject_test.go","internal/webserver/rwgate_test.go","internal/webserver/server.go","internal/webserver/server_chatsend_test.go","internal/webserver/server_test.go","internal/webserver/sessions_meta_embed_test.go","internal/webserver/sessions_meta_test.go","internal/webserver/tailscale.go","internal/webserver/tailscale_test.go","internal/webserver/testdata/fuzz/FuzzWebServerRoutes/fbdee7071c7665ef","main.go","notification_darwin.go","notification_darwin_test.go","tests/build-script.test.sh","tests/check-traceability-paths.sh","tests/fixtures/settings_agenthub_v4.2.json","tools.go","tray.go","tray_objc_darwin.m","tray_test.go"]
covered_digest: "v4:sha256:ef85af72905660c7f5b89fe52c403b6b080c0fbe34e5b479d270347061133121"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: passed
  previous_score: 5/7
  trigger: "Report went stale because commit 452cf7d4 (Start Web Server gated on Tailscale, NET-01, CONTEXT Amendment 3) changed covered source"
  gaps_closed:
    - "SC6 / TERM-05 moved from PRESENT_BEHAVIOR_UNVERIFIED to VERIFIED: UAT check 2 watched a tab close on shell exit in the built app, and the Go exit watcher tests still pass"
    - "SC7 / NET-01 moved from PRESENT_BEHAVIOR_UNVERIFIED to VERIFIED: UAT check 4 did a live Tailscale off/on with the banner, Share disabled and recovery without a restart; Amendment 3's Start Web Server gate is now in the UI and in Go"
    - "Previous decision item 1 (review IN-04, Start Web Server while Tailscale is off) is closed by 452cf7d4"
    - "Previous decision item 2 (remote-tab 'session ended' banner) is resolved by Amendment 3 (SC6 now says 'local shell')"
    - "Previous MVP-mode note is resolved by Amendment 3 (Mode: mvp dropped from ROADMAP)"
  gaps_remaining: []
  regressions: []
deferred:
  - truth: "No web-server listener outside the tailnet. The daemon still auto-starts a password-gated, LAN-bound web server when Tailscale is off at launch (review WR-01, internal/daemon/process.go:114-145). StartWebServer and the daemon still use LAN mode when Tailscale is connected but tailnet HTTPS certs are off (review WR-02, app.go:736-743). The CLI `serve`/`qr` commands can web-enable a session without a Tailscale check"
    addressed_in: "Phase 4"
    evidence: "Phase 4 SC1: 'a tailnet-only web link appears'; SHAR-02: 'links stay token-gated and tailnet-only'; SHAR-01: 'one on/off state per session'. 01-SECURITY.md T-01-42 / AR-07 accepts the LAN fallback for Phase 1 and gives it to the Phase 4 Share redesign. 01-UAT.md note: 'only Start is gated (Amendment 3 scope)'"
  - truth: "Native notification on Quit GUI only shows up (M-41; plan 01-11 truth 'Native notifications still work through one minimal path')"
    addressed_in: "Phase 3 / Phase 6"
    evidence: "Phase 3 NOTF-01: 'Native macOS notifications appear under the Portty name'; Phase 6 UPD-01 uses a native notification. 01-UAT.md: 'M-41 needs a signed build; deferred to Phase 6'. The code path is wired (app.go sendNotificationFunc -> notification_darwin.go sendNotification)"
  - truth: "Agent-CLI wording and the AgentHub Homebrew install line in user-facing copy (WelcomeTab.tsx:38, help/chat.md, web/join.html)"
    addressed_in: "Phase 3"
    evidence: "BRND-04: 'No AgentHub or agent-CLI wording remains in user-facing text'; DIST-03: 'every Homebrew reference ... removed'"
---

# Phase 1: Plain macOS Terminal Verification Report

**Phase Goal:** The app is a plain macOS terminal: new sessions run macOS zsh (`/bin/zsh -l`), no agent-CLI, file-browser, or Windows/Linux code remains, and every kept feature still works
**Verified:** 2026-11-15T15:55:00Z
**Status:** passed
**Re-verification:** Yes. The previous report was stale after commit 452cf7d4 changed covered source (`app.go`, `app_test.go`, `SettingsTab.tsx`, its test, `TESTING.md`). Every other commit since the last report changes `.planning/` docs only (`git diff --stat 32b89a74 HEAD -- . ':!.planning'`).

> The previous frontmatter said `status: passed` while its body said `human_needed`. This run recomputes the status from scratch.

## Goal Achievement

### Observable Truths (ROADMAP success criteria, the contract)

SC4 (chat parity) is struck through in ROADMAP. It moved to Phase 2 (CHAT-01, chat removal), so it is not counted. Plan-frontmatter truths from 01-10, 01-11 and 01-12 restate these SCs in more detail. They are folded into the rows below. Plan truths from 01-03..01-06 that Amendments 1 and 2 replaced are listed under "Superseded" and not counted.

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | New session -> pick dir (default last-used) -> `/bin/zsh -l`. No shell choice, no Settings -> Paths, no agent picker/badges/status or file browser in desktop or web share | VERIFIED | `internal/daemon/engine.go:220` `const sessionShell = "/bin/zsh"`, `:245` argv `["-l"]`. Named tests pass: `TestCreateSession_LoginShellArgv`, `TestCreateSession_IgnoresShellEnvAndLegacySetting`, and real-PTY `TestCreateSession_LoginShell`. `NewSessionModal.tsx` has only `onConfirm(workDir)` plus the `lastWorkDir` default. No Paths entry in SettingsTab, JumpBar or Search (grep). No `GetShellPath`/`GetCLIPaths`/`DetectCLIs`/`ListShells` in production code. `internal/files` and `internal/status` are gone. No ExitToast/hubStatus/FileBrowser/CodeMirror in `frontend/src` production files. Agent names show up only in comments ("Claude's Discretion", CLAUDE.md citations). UAT check 1: zsh 5.9 login shell, no badges |
| 2 | Quit/relaunch (or close/reopen window) keeps every running session with scrollback | VERIFIED | `TestExitWatcher_GUIRelaunchKeepsRunningSessions` passes: a real shell, a second App attaching to the same daemon, state `running`, scrollback marker present. UAT check 3: GUI `kill -9`, then relaunch reattached with `persist-check-123` in scrollback |
| 3 | Theme change and xterm plugin toggles (GPU, search, links, sixel, OSC 52, OSC 9;4) apply to open terminals | VERIFIED | Covered by Vitest suites `TerminalPanel.hot-swap.test.tsx`, `TerminalPanel.search*.test.tsx`, `TerminalPanel.web-links.test.tsx`, `PluginsSection*.test.tsx`, `App.plugin-event.test.tsx` (all green in the full run). OSC 9;4: `tabProgress` -> TabBar underline. UAT check 5: theme switches live, and WebGL off/on keeps rendering. The theme library itself is replaced in Phase 2 (THEM-01). That is a later change, not a gap |
| 5 | macOS Go build, Go tests, Vitest, lint pass. No Win/Linux sources/packaging/CI. No files API/capability. Fuzz harness covers relay + webserver | VERIFIED (one flaky test, see Anti-Patterns) | Run by this verifier: `go build ./... && go vet ./...` OK; `go test -race -short ./...` exit 0 (11 packages ok); `tsc --noEmit` OK; traceability `OK: 120 traceability paths exist`. Vitest: 1825/1826 passed. The one failure, `SessionShareModal.test.tsx` "SM-02 ... tabpanel is nested", passed 1/1 as a file (42/42) and 5/5 when run alone. It waits a single `tick()` for async `cachedShare` state, so it is a load-timing flake in code 452cf7d4 did not touch. No tracked `*_windows.go`/`*_linux.go`/`win32input*`/winget/`build/linux`/`build/windows`/`scripts/install.sh`. The only linux/windows paths are `portty-brand/build/{linux,windows}` icon assets, which are Phase 3 brand inputs and not code. Negative route tests `files_removed_test.go` (daemon and webserver) pass. Fuzz targets are unchanged since the last run, where all 6 ran clean, and their seed corpora run inside `go test` |
| 6 | When a local shell exits, its tab closes immediately with no status dot, countdown, toast or banner, including shells that exit hours later | VERIFIED | The Go watcher (`app.go` `startExitWatcher`/`checkExits`, 500 ms, no deadline) passes its named tests `TestCheckExits_EmitsOnceWhenShellExits`, `_LongLivedSessionStillReported`, `_StoppedAtFirstSight`, `_RemovesStoppedSession` and `_ReportsRemovedWithoutStoppedSighting`. React: `App.tsx:616-625` `session:exit` -> `removeTabStateRef` (the handler does not depend on age). The behavior evidence missing last time is now there: UAT check 2 watched the tab close on exit in the built app. Remote tabs keep their "session ended" banner by design (Amendment 3) |
| 7 | Tailscale disconnected -> banner, Share/remote controls disabled; reconnect clears without restart; never installs/configures Tailscale | VERIFIED | `internal/webserver/tailscale.go` uses LocalAPI only. `App.tsx:133` `tailscaleOff` drives `:1287 <TailscaleBanner/>`, `:948-949` peers cleared and poll skipped, `:977` Share dialog closed, and `:1412,:1586 shareDisabledReason`. Amendment 3 (new in 452cf7d4): `SettingsTab.tsx:303,538-543` disables Start Web Server with `TAILSCALE_OFF_REASON` as its title (Stop stays available), and `app.go:732-735` makes `StartWebServer` refuse with the NET-01 sentence. Named tests pass: `TestStartWebServerRefusedWhileTailscaleOff`, `TestCheckTailscaleHealth_EmitsFirstResultAndChanges`, and `SettingsTab.tailscale-status.test.tsx` 6/6. No `AutoInstallTailscale`/`findBrew`/brew-install code. UAT check 4: live off -> banner + Share disabled + "Not connected", then on -> cleared with no restart |

**Score:** 6/6 truths verified (0 present, behavior-unverified)

### NET-01 / Amendment 3 vs review WR-01 and WR-02: decision and reasoning

The question is whether the daemon's LAN-mode web server (WR-01: auto-started at launch when Tailscale is off; WR-02: used when Tailscale is connected but HTTPS certs are off) means NET-01 or Amendment 3 is unmet. **Decision: neither is unmet. Both findings are deferred to Phase 4, and none of them blocks Phase 1.**

1. **What the contract asks for.** NET-01 and SC7 ask for four things: a LocalAPI check, the banner, *disabled share and remote controls*, and automatic rechecks without installing anything. All four exist and UAT saw them work. Amendment 3 makes one narrow decision: "'Start Web Server' is disabled while Tailscale is off, with the NET-01 banner sentence as its reason". The previous verifier's IN-04 row (the thing Amendment 3 "closes") was exactly that control. 452cf7d4 does this in the UI and adds a Go-side refusal behind it as a second layer. Neither text says the daemon must never bind a non-tailnet listener.
2. **WR-02 is outside NET-01 entirely.** In that state Tailscale *is* connected (BackendState Running, as 01-12's must-have defines it). NET-01 only governs the not-connected state. LAN mode while connected is inherited AgentHub behavior.
3. **What WR-01 actually exposes.** Every session route on the LAN server sits behind both Basic Auth (`server.go:561`, the whole mux) and `requireCapability` (`server.go:870-910`). While `tailscaleOff`, the GUI has no way to mint a capability: Share is disabled, the Share dialog closes, and peer and join-code entry points are hidden. A fresh daemon has no sessions, so none can be web-enabled from before. The remaining reach is the password-gated dashboard/join pages and the CLI `serve`/`qr` commands, which an explicit user action would have to run. That is a real inconsistency with the banner text and with the project constraint "Remote access only over the user's tailnet". It is not a broken share/remote *control* in the app.
4. **There is a specific owner for it.** Phase 4 SC1 says "a tailnet-only web link", and SHAR-02 says "links stay token-gated and tailnet-only". Removing LAN mode is exactly that work. 01-SECURITY.md T-01-42 / AR-07 records the LAN fallback as an accepted risk owned by the Phase 4 Share redesign. The UAT notes record the user seeing "only Start is gated (Amendment 3 scope)" and passing UAT.

Recommendation (optional, not a gap). If you want the banner text to be literally true in Phase 1, review WR-01 option (a) is a small change: drop the local-mode branch of the daemon auto-start and let `upgradeToTailscale` start the server. Otherwise, add one sentence to Amendment 3 and to TESTING.md row 233 / M-57 saying that only the GUI Start path is gated and the daemon LAN fallback stays until Phase 4. As written, "no LAN fallback" reads broader than the `StartWebServer` RPC it describes.

### Superseded plan must-haves (not counted)

| Plan | Superseded must-have | Superseded by | Current code |
|------|---------------------|---------------|--------------|
| 01-03 | tailscale override, shellPath, notify toggle survive legacy load | Amendments 1/2, 01-11 | Keys load and are dropped on save (`TestSettingsMigration_LegacyAgentHubV42`) |
| 01-04 | Read-only Shell line and "Change it in Settings -> Paths" helper | Amendment 1, 01-11 | Modal has the Working Directory only |
| 01-05 | `$SHELL`/settings shell resolver; Paths shell + tailscale rows | Amendment 1, 01-11/01-12 | Fixed `/bin/zsh`; no Paths; LocalAPI-only Tailscale |
| 01-06 | Status dots, Hub status, exit toast + Keep Open, `hubStatus.ts`, `session:status` | Amendment 2, 01-10 | All removed |
| 01-09 | CHAT-01 chat parity (requirements field) | ROADMAP 2026-11-15, CHAT-01 -> Phase 2 | Inject bridge removed (inject-free tests pass); chat itself removed in Phase 2 |

### Required Artifacts

`gsd query verify.artifacts`: 01-10 3/3, 01-11 3/3, 01-12 3/3 passed. The earlier plans' artifacts were verified in the previous run, and no covered source except the five 452cf7d4 files has changed since.

| Artifact | Status | Details |
|----------|--------|---------|
| `app.go` `startExitWatcher`, `StartWebServer` guard | VERIFIED | Watcher plus the new `!h.Connected` refusal (`app.go:732-735`) |
| `app_exit_test.go` `TestCheckExits_*` | VERIFIED | 5 named tests plus the relaunch test pass |
| `internal/daemon/settings_removed_test.go` | VERIFIED | Passes in the suite |
| `internal/daemon/engine.go` `/bin/zsh` | VERIFIED | `sessionShell` const |
| `notification_darwin.go` `sendNotification` | VERIFIED (wired) | Live delivery deferred (M-41) |
| `internal/webserver/tailscale.go` `CheckHealth` | VERIFIED | LocalAPI only |
| `frontend/src/components/TailscaleBanner.tsx`, `frontend/src/lib/tailscale.ts` | VERIFIED | One shared sentence constant |
| `frontend/src/components/SettingsTab.tsx` Start gate | VERIFIED | `tailscaleOff` in `disabled`; title order is running -> none, off -> NET-01, undisclosed -> CT |
| `frontend/src/components/__tests__/SettingsTab.tailscale-status.test.tsx` | VERIFIED | 6/6, including the two Amendment 3 cases |

### Key Link Verification

`verify.key-links` reports 01-10 0/3, 01-11 1/3 and 01-12 0/3 only because the `from:` fields put a symbol name after the path ("Source file not found"). Each was checked by hand:

| From | To | Via | Status |
|------|----|-----|--------|
| `app.go` checkExits | `App.tsx` `session:exit` handler | event -> `removeTabStateRef` (`App.tsx:616-625`) | WIRED |
| `App.tsx` init/retryInit | daemon `KillSession` | stopped sessions not restored | WIRED |
| `App.tsx` handleProgressChange | `TabBar.tsx` | `tabProgress` | WIRED |
| `engine.go` spawnCommand | pty Create | `sessionShell, ["-l"]` (`engine.go:245`) | WIRED |
| `NewSessionModal.tsx` | `app.go` CreateSession | `onConfirm(workDir)` | WIRED (tool) |
| `app.go` QuitGUIOnly | `notification_darwin.go` | `sendNotificationFunc` | WIRED |
| `tailscale.go` CheckHealth | `App.tsx` tailscaleHealth | `tailscale:health` + `GetTailscaleStatus` | WIRED |
| `App.tsx` tailscaleOff | StatusBar / HubPanel -> SessionCard | `shareDisabledReason` (`App.tsx:1412,1586`) | WIRED |
| `App.tsx` tailscaleOff | remote-peer poll | `setRemotePeers([])` (`App.tsx:949`) | WIRED |
| `App.tsx` tailscaleHealth | `SettingsTab` Start gate | `tailscaleHealth={tailscaleHealth}` (`App.tsx:1477`) -> `tailscaleOff` (`SettingsTab.tsx:303`) | WIRED (new) |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| TailscaleBanner / Share buttons / Start Web Server | `tailscaleHealth` -> `tailscaleOff` | `GetTailscaleStatus()` + `tailscale:health` from `checkTailscaleHealth` -> LocalAPI | Yes | FLOWING |
| Tabs (exit) | `session:exit` sessionId | `checkExits` -> daemon `ListSessions()` | Yes | FLOWING |
| Restored tabs | `allSessions` | daemon registry | Yes | FLOWING |
| Tab progress | `tabProgress` | OSC 9;4 parse in TerminalPanel | Yes | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Go build + vet | `go build ./... && go vet ./...` | OK | PASS |
| Go tests (race) | `go test -race -short ./...` | exit 0, 11 packages ok | PASS |
| TS typecheck | `pnpm exec tsc --noEmit` | exit 0 | PASS |
| Vitest (single full run) | `pnpm exec vitest run` | 1825/1826; 1 failure in `SessionShareModal.test.tsx` SM-02 | FLAKY (see below) |
| Flake isolation | `vitest run SessionShareModal.test.tsx`, then `-t 'tabpanel is nested'` x5 | 42/42; 5/5 | PASS |
| Amendment 3 Go guard | `go test -run TestStartWebServerRefusedWhileTailscaleOff .` | PASS | PASS |
| Amendment 3 UI gate | `vitest run SettingsTab.tailscale-status.test.tsx` | 6/6 | PASS |
| Exit watcher + relaunch | `go test -run 'TestCheckExits_\|TestExitWatcher_GUIRelaunch' .` | 6 PASS | PASS |
| Login zsh | `go test -run 'TestCreateSession_LoginShell\|..._IgnoresShellEnv' ./internal/daemon/` | 3 PASS | PASS |
| Traceability | `bash tests/check-traceability-paths.sh` | 120 paths OK | PASS |
| Live app behavior | 01-UAT.md (built with `wails build -platform darwin/arm64 -tags wailsassets`, after 452cf7d4) | 5/5 active checks + M-16 pass | PASS (human, recorded) |

### Probe Execution

Step 7c: SKIPPED (no `scripts/*/tests/probe-*.sh` declared or present for this phase).

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|----------------|-------------|--------|----------|
| STRP-01 | 01-04..01-08, 01-10, 01-11 | All agent-CLI code removed | SATISFIED | SC1. Grep finds agent names only in comments |
| STRP-02 | 01-02, 01-03 | File browser/editor and files API/capability removed | SATISFIED | SC5. Negative route tests pass |
| STRP-03 | 01-01 | Win/Linux code, packaging, CI removed; macOS build/test/lint pass | SATISFIED | SC5 |
| STRP-04 | all plans | Removed-feature tests deleted, kept tests pass, fuzz covers relay + webserver | SATISFIED (one flaky kept test) | SC5 |
| TERM-01 | 01-04, 01-05, 01-11 | Always `/bin/zsh -l` in chosen/last-used dir | SATISFIED | SC1, UAT 1 |
| TERM-02 | 01-10 | Sessions survive GUI close/restart with scrollback | SATISFIED | SC2, UAT 3 |
| TERM-04 | 01-04 | xterm plugins keep working from Settings | SATISFIED | SC3, UAT 5 |
| TERM-05 | 01-10 | Shell exit closes tab immediately at any age, no exit UI | SATISFIED | SC6, UAT 2 |
| NET-01 | 01-12 (+452cf7d4) | LocalAPI check, banner, share/remote controls disabled, recheck, never installs | SATISFIED | SC7, UAT 4. LAN fallback deferred to Phase 4 (see decision above) |

All 9 phase requirement IDs appear in at least one plan's `requirements:` field. REQUIREMENTS.md maps no other ID to Phase 1, so none are orphaned. CHAT-01 is still in 01-09's `requirements:` but now maps to Phase 2. It is re-scoped, not orphaned.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (all 206 covered implementation files) | - | TBD/FIXME/XXX | none | Debt-marker gate clean |
| `frontend/src/components/__tests__/SessionShareModal.test.tsx` | 813-821 | Single `await tick()` before checking DOM that needs async `cachedShare` | Warning | Failed once under full-suite load, passed 6/6 alone. Fix: `await vi.waitFor(...)` or flush with `React.act`. The modal is redesigned in Phase 4 anyway |
| `internal/daemon/process.go` | 114-145 | Daemon LAN-mode auto-start while Tailscale is off (review WR-01) | Warning (deferred, Phase 4) | See decision above |
| `app.go` | 736-743 | LAN fallback when connected without certs (review WR-02) | Warning (deferred, Phase 4) | See decision above |
| `cmd_cli.go` | 272-280 | `serve` web-enables a session with no Tailscale check | Info (deferred, Phase 4) | Needs an explicit CLI action; password- and capability-gated |
| `TESTING.md` / `01-CONTEXT.md` | 233 / Amendment 3 | "no LAN fallback" / "closes IN-04" read broader than the GUI Start path they cover | Info | Clarify the wording, or apply WR-01 option (a) |
| `frontend/src/App.tsx` | 450-452, 1228-1230 | Frontend `KillSession` of stopped sessions duplicates the Go watcher (review WR-03, necessity) | Info | Harmless. Removing it is a human call |
| `frontend/src/components/__tests__/SettingsTab.appearance-theme.test.tsx`, `SettingsTab.shell-warn-toggle.test.tsx` | 31, 50 | Mocks for the removed `GetCLIPaths`/`GetShellPath` | Info | Dead mock entries |
| Review IN-01..IN-10 | - | Redundant `h.Connected &&`, duplicated NET-01 literal in Go, `TestStartWebServerLocalModeFallback` asserts nothing, stale comments, `QuitGUIOnly` Status/State, Linux fixture exit detection, traceability parser edge, orphaned `GetSessionQRCode` binding, `daemonErr` race | Info | Carried in 01-REVIEW.md. None blocks a success criterion |

### Human Verification Required

None outstanding. All live items the previous report listed were run in 01-UAT.md on a build that includes 452cf7d4: checks 1-5 and M-16 passed, and M-24 was superseded by the CHAT-01 re-scope. M-41 (native notification) is deferred to Phase 3/6 because it needs a signed build. The previous report's race/ordering review-fix items have Go regression tests (`TestCheckExits_ReportsRemovedWithoutStoppedSighting`, `TestCheckTailscaleHealth_EmitsFirstResultAndChanges`, and others), and their React side is the same `session:exit` / `tailscale:health` handlers UAT exercised. They are not success criteria.

### Gaps Summary

There are no blocking gaps, and the phase goal is achieved in the codebase:
- Every session spawns a fixed `/bin/zsh -l`, proven by real-PTY tests and UAT.
- No agent-CLI, file-browser, files-API or Windows/Linux code, packaging or CI remains.
- Kept features still work: session persistence across GUI restarts, themes and plugins, tab close on exit at any age, and the NET-01 Tailscale banner and gating. Each was checked by automated tests run in this verification and by the recorded UAT.

Commit 452cf7d4 adds the Amendment 3 gate on Start Web Server in the UI, with a Go refusal behind it. Both have passing tests, and no regressions were found.

Open items, none blocking:
1. **Daemon LAN-mode web server (review WR-01/WR-02, CLI `serve`).** Deferred to Phase 4's tailnet-only share redesign, under accepted risk T-01-42. Optionally fix now with WR-01 option (a), or narrow the "no LAN fallback" wording.
2. **One flaky Vitest case (`SessionShareModal` SM-02).** Worth hardening so CI does not go red at random.

---

_Verified: 2026-11-15T15:55:00Z_
_Verifier: Claude (gsd-verifier)_
