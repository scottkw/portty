---
phase: 02-tabs-sidebar-navigation
reviewed: 2026-11-16T01:14:28Z
depth: standard
iteration: 3
files_reviewed: 129
files_reviewed_list:
  - app.go
  - app_test.go
  - appearance_darwin.go
  - appearance_darwin.m
  - cmd/playwright-fixture/main.go
  - frontend/e2e/web-color-scheme.spec.ts
  - frontend/e2e/web-plugin-hot-swap.spec.ts
  - frontend/e2e/web-share-scope.spec.ts
  - frontend/index.html
  - frontend/package.json
  - frontend/src/App.tsx
  - frontend/src/__tests__/App.saver.test.tsx
  - frontend/src/__tests__/style.tokens.test.ts
  - frontend/src/components/ContextMenu.tsx
  - frontend/src/components/EmptyState.tsx
  - frontend/src/components/FindBar/__tests__/FindBar.themeMatrix.test.tsx
  - frontend/src/components/FindBar/__tests__/FindBar.visual.test.tsx
  - frontend/src/components/InlineSessionName.tsx
  - frontend/src/components/NewSessionModal.tsx
  - frontend/src/components/PluginToggleBanner.tsx
  - frontend/src/components/PluginsSection.tsx
  - frontend/src/components/QuitConfirmModal.tsx
  - frontend/src/components/RegenerateKeyModal.tsx
  - frontend/src/components/RemoteJoinCodeModal.test.tsx
  - frontend/src/components/RemoteJoinCodeModal.tsx
  - frontend/src/components/SessionEndedBanner.tsx
  - frontend/src/components/SessionShare/InternetFullAccessTab.tsx
  - frontend/src/components/SessionShare/shared.tsx
  - frontend/src/components/SettingsSheet.tsx
  - frontend/src/components/Share/FunnelRiskPanel.tsx
  - frontend/src/components/Share/SessionShareModal.tsx
  - frontend/src/components/Share/ShareSegmentedControl.tsx
  - frontend/src/components/Sidebar.tsx
  - frontend/src/components/Switch.tsx
  - frontend/src/components/TabBar.tsx
  - frontend/src/components/TailscaleBanner.tsx
  - frontend/src/components/TerminalPanel.tsx
  - frontend/src/components/UpdateBanner.tsx
  - frontend/src/components/WebGLRecoveryBanner.tsx
  - frontend/src/components/WebShareSessionView.tsx
  - frontend/src/components/__tests__/App.createTab.sessionsSeed.test.tsx
  - frontend/src/components/__tests__/App.dead-modal.test.tsx
  - frontend/src/components/__tests__/App.nav.test.tsx
  - frontend/src/components/__tests__/App.open-remote.test.tsx
  - frontend/src/components/__tests__/App.shellExit.test.tsx
  - frontend/src/components/__tests__/App.shellWebShare.test.tsx
  - frontend/src/components/__tests__/App.tailscale.test.tsx
  - frontend/src/components/__tests__/App.test.tsx
  - frontend/src/components/__tests__/ContextMenu.test.tsx
  - frontend/src/components/__tests__/EmptyState.test.tsx
  - frontend/src/components/__tests__/FunnelRiskPanel.test.tsx
  - frontend/src/components/__tests__/InlineSessionName.test.tsx
  - frontend/src/components/__tests__/NewSessionModal.test.tsx
  - frontend/src/components/__tests__/PluginToggleBanner.test.tsx
  - frontend/src/components/__tests__/PluginsSection.disclosure.render.test.tsx
  - frontend/src/components/__tests__/PluginsSection.test.tsx
  - frontend/src/components/__tests__/QuitConfirmModal.test.tsx
  - frontend/src/components/__tests__/RegenerateKeyModal.test.tsx
  - frontend/src/components/__tests__/RemoteJoinCodeModal.test.tsx
  - frontend/src/components/__tests__/SessionEndedBanner.test.tsx
  - frontend/src/components/__tests__/SessionShareModal.disconnect.test.tsx
  - frontend/src/components/__tests__/SessionShareModal.test.tsx
  - frontend/src/components/__tests__/SettingsSheet.persistence.test.tsx
  - frontend/src/components/__tests__/SettingsSheet.shell-warn-toggle.test.tsx
  - frontend/src/components/__tests__/SettingsSheet.start-minimized.test.tsx
  - frontend/src/components/__tests__/SettingsSheet.tailscale-status.test.tsx
  - frontend/src/components/__tests__/SettingsSheet.test.tsx
  - frontend/src/components/__tests__/SettingsSheet.web-link-ux.test.tsx
  - frontend/src/components/__tests__/ShareSegmentedControl.test.tsx
  - frontend/src/components/__tests__/Sidebar.test.tsx
  - frontend/src/components/__tests__/Switch.test.tsx
  - frontend/src/components/__tests__/TabBar.test.tsx
  - frontend/src/components/__tests__/TailscaleBanner.test.tsx
  - frontend/src/components/__tests__/TerminalPanel.scale.test.tsx
  - frontend/src/components/__tests__/TerminalPanel.test.tsx
  - frontend/src/components/__tests__/UpdateBanner.test.tsx
  - frontend/src/components/__tests__/WebGLRecoveryBanner.test.tsx
  - frontend/src/components/__tests__/WebShareSessionView.plugin-config.test.tsx
  - frontend/src/components/__tests__/WebShareSessionView.test.tsx
  - frontend/src/components/__tests__/style.contrast.test.ts
  - frontend/src/components/__tests__/style.hub.modal.test.ts
  - frontend/src/components/__tests__/style.redesign.test.ts
  - frontend/src/components/__tests__/style.settings.test.ts
  - frontend/src/lib/__tests__/sidebarPeers.test.ts
  - frontend/src/lib/__tests__/tabs.test.ts
  - frontend/src/lib/__tests__/terminalPalettes.test.ts
  - frontend/src/lib/__tests__/useColorScheme.test.tsx
  - frontend/src/lib/__tests__/useDialogFocus.test.tsx
  - frontend/src/lib/helpUrl.ts
  - frontend/src/lib/relayClient.test.ts
  - frontend/src/lib/relayClient.ts
  - frontend/src/lib/sidebarPeers.ts
  - frontend/src/lib/tabs.ts
  - frontend/src/lib/terminalPalettes.ts
  - frontend/src/lib/useColorScheme.ts
  - frontend/src/lib/useDialogFocus.ts
  - frontend/src/main.tsx
  - frontend/src/style.css
  - frontend/src/test-setup.ts
  - frontend/src/vite-env.d.ts
  - frontend/src/wailsjs/go/main/App.d.ts
  - frontend/src/wailsjs/go/main/App.js
  - frontend/src/wailsjs/go/models.ts
  - internal/daemon/api.go
  - internal/daemon/appearance_settings_test.go
  - internal/daemon/chat_removed_test.go
  - internal/daemon/client.go
  - internal/daemon/engine.go
  - internal/daemon/engine_migration_test.go
  - internal/daemon/engine_test.go
  - internal/daemon/plugin_settings.go
  - internal/daemon/settings_removed_test.go
  - internal/relay/fuzz_test.go
  - internal/relay/hub.go
  - internal/relay/protocol.go
  - internal/relay/server.go
  - internal/relay/server_inject_test.go
  - internal/relay/server_test.go
  - internal/webserver/assets_test.go
  - internal/webserver/chat_removed_test.go
  - internal/webserver/fuzz_test.go
  - internal/webserver/inject_test.go
  - internal/webserver/server.go
  - internal/webserver/server_test.go
  - internal/webserver/session_ended_test.go
  - main.go
  - menu_test.go
  - web/assets/dashboard.css
  - web/assets/join.css
findings:
  critical: 0
  warning: 1
  info: 9
  total: 10
  necessity: 1
status: issues_found
---

# Phase 2: Code Review Report (iteration 3)

**Reviewed:** 2026-11-16T01:14:28Z
**Depth:** standard
**Files Reviewed:** 129. The scope is the same as iterations 1 and 2. I re-read in full the six files touched by `65c0b5ad..9b3b32ae` (`app.go`, `app_test.go`, `SettingsSheet.tsx`, `NewSessionModal.tsx` and their two test files), plus the code they call: `internal/daemon/process.go` `EnsureDaemon`, `internal/daemon/socket.go`, `tray.go`, the `App.tsx` init, retryInit, quit and poll paths, and the Wails v2.10.2 darwin message loop and dispatcher.
**Status:** issues_found

## Summary

This is the last `--auto` re-review, after the iteration-2 fixes `eb740795` (WR-07, IN-13) and `9b3b32ae` (IN-11). I re-ran the ground truth on `9b3b32ae` and all of it passes:
- `go build ./... && go vet ./...`: exit 0
- `go test -race -short -count=1 ./...`: exit 0, every package ok
- `pnpm exec tsc --noEmit`: exit 0
- `pnpm exec vitest run`: 91 files, 1160 tests pass

WR-07, IN-11 and IN-13 are resolved. `eb740795` introduced one regression, WR-08: `awaitStartup` was added to `ListSessions`, and the Quit confirm path also calls `ListSessions`. So ⌘Q, the tray Quit item and the window close path do nothing visible while startup is slow or wedged, for up to 90 s plus a 30 s RPC timeout. The `ListSessions` and `GetRelayPort` gates protect nothing on the init path, which is already ordered behind `GetDaemonError`, so removing them fixes the regression at no cost. I also found two new Info findings. IN-14: start-minimized is ignored when startup takes over 5 s. IN-15: the 90 s bound does not cover the actual `EnsureDaemon` worst case.

These are the results of the checks the orchestrator asked for:

- **No deadlock.** `awaitStartup` (`app.go:165-175`) only receives from `startupDone` or a timer. The only goroutine that closes `startupDone` is the one running `startDaemon`, and `startDaemon` never calls a gated binding. `applyStoredAppearance` calls `GetAppearance`, which is not gated. `startup` calls `initTray` before `startDaemon`, but that only installs the status item. The tray poller, exit watcher, health poller and update poller start after `startDaemon` returns, so their `ListSessions` calls never wait. `onTraySession` (`tray.go:73-87`) moves its `ListSessions` call onto a goroutine, and the tray menu has no session entries until the first post-startup `refreshTrayState`.
- **`RetryDaemon` cannot hang the UI loop.** Wails dispatches every `C` call on its own goroutine (`wails/v2@v2.10.2/internal/frontend/desktop/darwin/frontend.go:389`). Only `DomReady`, `runtime:ready` and `wails:openInspector` run inline on the message processor (`:360-383`). The JS `Call` has no timeout by default (`runtime/desktop/calls.js:57-61`), so a 90 s wait just resolves late. In the UI, `retryInit` clears `daemonError` before it awaits, so the banner goes away and the main area stays blank with no progress cue while the wait runs. That is a UX gap, not a hang. The real UI-level hang is the quit path (WR-08).
- **`NewApp` creating the client has no network side effects.** `NewDaemonClient` (`internal/daemon/client.go:26-40`) only builds an `http.Transport` with a lazy `DialContext`, so nothing connects at construction. The only side effect is `DefaultSocketPath()`, which runs `os.MkdirAll(<UserConfigDir>/agenthub, 0700)` (`socket.go:21`). `startup` already did that before this change, and `NewApp` is only reached from `runGUI` (`main.go:68`), never from the CLI or `--help` paths. The happens-before argument holds: the field is written before `wails.Run` starts any goroutine.
- **`startDaemon` closes `startupDone` on every return path.** The close is the first statement and is deferred (`app.go:238-240`). Both branches, the `emitEvent("daemon:error")` branch and the `applyStoredAppearance` branch, return normally. `emitEvent` (`app.go:375-383`) checks `ctx.Value("frontend")`. Wails sets that key before it calls `OnStartup` (`internal/app/app_production.go:89`), so the event still reaches the frontend in production. `TestStartDaemon_AlwaysClosesStartupDone` covers both branches.
- **domReady's timeout path never calls the daemon.** `ready=false` skips `GetStartMinimized` (`app.go:187-195`). `TestDomReady_TimeoutShowsWindowWithoutRPC` points the client at a socket that accepts and never answers, and asserts zero accepts. It would catch a regression.
- **Data race (WR-07 part 3).** It is closed for the startup writes: `a.client` (written in `NewApp`), plus `a.daemonErr`, `a.ctx` and `a.trayInit` (written before the close), all have a happens-before edge for any reader that went through `awaitStartup`. One residual remains. `RetryDaemon` writes `a.daemonErr` after `startupDone` with no further synchronization, so the field comment "read only after startupDone" is not the whole story. The frontend never runs `GetDaemonError` and `RetryDaemon` at the same time, because the Retry button only appears after init's `GetDaemonError` has resolved or `daemon:error` has fired, and `startupDone` closes immediately after that emit. So this is not a finding.
- **The `urlCopyError` and folder-picker error clearing is correct.** `handleCopyURL` clears `urlCopyError` before each attempt (`SettingsSheet.tsx:232`), and `handleToggleServer` clears both errors (`:388-389`). Precedence is `serverError ?? urlCopyError` (`:375`), so a copy can never hide a start or stop error. `isServerRunning` only changes in `handleToggleServer` and the mount load (`:154, :394, :400`), so a copy error cannot outlive the running state. The Copy button only renders while the server is running. `handleBrowse` clearing `error` at the start (`NewSessionModal.tsx:26`) also clears a stale create error once the user picks another folder, which is the right behaviour. Both new tests are behavioural. The fix report says both failed against the pre-fix source.

I carried forward the deferred findings IN-01, IN-02, IN-03, IN-04, IN-06, IN-08 and IN-12 with their IDs. All seven still apply, and I updated their line numbers.

## Resolved (verified this iteration)

| ID | Status | Evidence |
|----|--------|----------|
| WR-07 | Resolved; the regression it introduced is WR-08 | (1) The timeout path makes no RPC: `app.go:187-195`, checked by `TestDomReady_TimeoutShowsWindowWithoutRPC`. (2) `init` cannot run before startup: `GetDaemonError` waits (`app.go:258-266`) and `init` awaits it before `Promise.all` (`App.tsx:344-354`). A slow but successful start gives `relayPort > 0` (`TestSlowStartup_InitWaitsForStartup`), and a slow failure reaches the banner (`TestSlowStartup_FailureReachesInit`). (3) The race is closed by creating the client in `NewApp` (`app.go:145`) and reading `daemonErr` only after `awaitStartup`. |
| IN-11 | Resolved | `SettingsSheet.tsx:122-123, 232, 236, 375-377, 389`; `NewSessionModal.tsx:26`. Render tests: `SettingsSheet.web-link-ux.test.tsx:63-90`, `NewSessionModal.test.tsx:135-138`. |
| IN-13 | Resolved | `startupWait`, `startupBindingWait`, `errStartupTimeout` and `awaitStartup` now sit above domReady's doc comment (`app.go:151-179`). The "even on panic" comment is gone. `startDaemon`'s doc says "on every return path" (`app.go:235-236`). |

Resolved in iteration 1 and still resolved: WR-01, WR-02, WR-03, WR-04, WR-05 (its residual was WR-07), WR-06, IN-05, IN-07, IN-09, IN-10. None of the iteration-2 commits touch their code paths, except WR-03's `RetryDaemon`, which still calls `applyStoredAppearance` (`app.go:286`).

## Narrative Findings (AI reviewer)

## Warnings

### WR-08: `awaitStartup` in `ListSessions` blocks the Quit confirm for up to 90 s (120 s with a wedged daemon) during a slow startup

**File:** `app.go:437-438`, `app.go:562-563`, `app.go:305-317`; `frontend/src/App.tsx:537-549`
**Issue:** `eb740795` made `ListSessions` (and `GetRelayPort`) wait on `startupDone` for up to `startupBindingWait` (90 s). Every quit path goes through `ListSessions`:
- ⌘Q goes `applicationShouldTerminate`, then `processMessage("Q")` (`wails .../darwin/AppDelegate.m:27`), then the dispatcher's `Quit()` (`dispatcher.go:69-70`), then `OnBeforeClose` (`frontend.go:326-333`).
- The tray Quit item goes through `onTrayQuit` (`tray.go:59-70`).

Both paths end in `beforeClose`, which returns `true` (cancels the quit) and emits `app:quit-requested` (`app.go:305-317`). The frontend handler shows the confirm only after `ListSessions()` resolves (`App.tsx:540-548`). So while startup is still running, which is past domReady's 5 s bound when the window is already visible, ⌘Q does nothing visible until startup finishes. With a wedged daemon, it waits the full 90 s, and then the "try anyway" RPC adds the 30 s client timeout. During that time the only way out is Force Quit. Repeated ⌘Q presses queue more blocked calls, because the `prev` guard is still `false`. Before this commit, the same path blocked at most 30 s, the client timeout. A slow or wedged startup is exactly when a user is most likely to give up and quit.

The two gates also protect nothing. `init` awaits the gated `GetDaemonError` before it calls `GetRelayPort` and `ListSessions` (`App.tsx:344-354`), and `retryInit` awaits the gated `RetryDaemon` first (`App.tsx:1016-1026`). Those are the only frontend callers of `GetRelayPort`. The other `ListSessions` callers are the session poll, which tolerates `[]` and keeps polling every 3 s (`App.tsx:746-753`), the quit handler, `QuitGUIOnly`, `onTraySession` and `refreshTrayState`. None of them needs to wait for startup.
**Fix:** Remove the gate from `ListSessions` and `GetRelayPort`, and keep it only on `GetDaemonError` and `RetryDaemon`, which order init and retry:
```go
// ListSessions returns a snapshot of all registered sessions.
func (a *App) ListSessions() []SessionInfo {
	if a.client == nil {
		return []SessionInfo{}
	}
	...
}

// GetRelayPort returns the TCP port the daemon's relay HTTP server is listening on.
func (a *App) GetRelayPort() int {
	if a.client == nil {
		return 0
	}
	...
}
```
`TestSlowStartup_InitWaitsForStartup` still passes, because it calls `GetDaemonError` before `GetRelayPort`. Add a test that keeps `startupDone` open and asserts `ListSessions()` returns within ~100 ms. Optionally, also make the quit handler show the confirm at once and fill in the session list when it arrives, so a wedged daemon cannot delay Quit by the 30 s client timeout either.

## Info

### IN-01: A failed rename reverts silently, and the poll can flicker the name (carried forward, deferred)

**File:** `frontend/src/App.tsx:662-671`
**Issue:** Unchanged. A `RenameSession` failure only does `console.warn` and puts the old name back with no visible reason. If two quick renames overlap and the first fails after the second succeeds, the revert sets the name back to the original while the daemon holds the second name, until the next poll.
**Fix:** Show the failure in `saveBanner`. Revert only if the current name still equals the attempted one.

### IN-02: Menu shortcuts act behind open dialogs (carried forward, deferred)

**File:** `frontend/src/App.tsx:525-531`
**Issue:** Unchanged. ⌘W detaches the active tab while Settings, Share or the End Session confirm is open, and ⌘T stacks New Session over Settings.
**Fix:** Return early from the listeners when `document.querySelector('[aria-modal="true"]')` matches.

### IN-03: Dead Hub-era code and stale comments left behind (carried forward, deferred on purpose)

**File:** `app.go:492-511`, `internal/daemon/client.go:91-111`, `internal/daemon/api.go:157-158`, `frontend/src/wailsjs/go/main/App.d.ts:54`, `internal/relay/hub.go:164-165, 182`, `internal/webserver/server.go:1166, 1173`, `frontend/src/components/RemoteJoinCodeModal.tsx:114`
**Issue:** Unchanged; only the `app.go` lines moved. The tail and styled-tail paths (`GetSessionTailLines`, "Used by the Hub mini-preview poller") have had no callers since the Hub was removed. `hub.go` still mentions ChatPanel and the Hub-card preview, `server.go` still says "chat-capable SPA", and the join dialog still says "in your browser".
**Fix:** Remove the dead paths, or record why they stay, and update the comments and copy.

### IN-04: Chat history files and aliases.json are orphaned on disk for upgraded users (carried forward, deferred on purpose)

**File:** `internal/daemon/engine.go` (former `chatsDir()` / `NewAliasStore(cfgDir)`)
**Issue:** Unchanged. Nothing deletes `<configDir>/chats/*.jsonl` or `aliases.json`, and those files can hold chat content and tailnet identities.
**Fix:** Add a one-time best-effort removal at daemon start, or document that the files are kept.

### IN-06: Impossible-state guard: `@available(macOS 10.14, *)` (carried forward, deferred)

**File:** `appearance_darwin.m:19`
**Category:** necessity
**Issue:** Unchanged. A Go 1.26 binary targeting Apple Silicon cannot run on macOS older than 10.14, and the comment's "deployment target is 10.13" premise (`appearance_darwin.m:18`) describes Wails' cgo flags. This rests partly on judgment about the toolchain minimum, so it stays Info.
**Fix:** Keep it if it only silences `-Wunguarded-availability`, but correct the comment. Otherwise drop the branch.

### IN-08: Settings shows "Automatic" when the Appearance load fails, and Automatic cannot be re-saved (carried forward, deferred)

**File:** `frontend/src/components/SettingsSheet.tsx:195-199, 277`
**Issue:** Unchanged; the lines moved +2. The `GetAppearance` catch still marks the control loaded with the default `system`, and `selectAppearance` still returns early on `next === appearance`.
**Fix:** When loading fails, show an error and render no selection, or skip the early return after a failed load.

### IN-12: The WR-02, WR-03 and WR-06 race fixes are covered only by source-text assertions (carried forward, deferred)

**File:** `frontend/src/components/__tests__/App.nav.test.tsx:28-30, 59-60, 69-75`; `frontend/src/components/__tests__/App.test.tsx:64-72, 571-572`
**Issue:** Unchanged. The tests slice `App.tsx` source and look for literal strings. A behaviour-preserving reformat fails them, and a change that keeps the strings but breaks the behaviour passes them. No test renders the poll-in-flight-across-exit race or a tray focus for a detached session. (The IN-11 fix shows the better pattern: it turned its source-text assertion into a render test.)
**Fix:** For WR-06, render `App` with a deferred `ListSessions`, fire `session:exit`, resolve the poll, and assert the sidebar row is absent. For WR-02, emit `tray:focus-session` for an ID that has no tab and assert that a tab panel appears.

### IN-14: Start-minimized is ignored whenever startup takes over 5 s, which is the login-at-cold-boot case it exists for

**File:** `app.go:183-203`
**Issue:** On domReady's timeout path, `ready=false`, so `startMinimized` stays `false` and the window is shown (`app.go:196-203`). The comment documents this as the safe default. But the slow-startup case is most likely at login on a cold boot (`EnsureDaemon` spawning the daemon) and after an upgrade (the version-mismatch restart, `process.go:176-181`). That is exactly when a user who chose "start minimized" expects no window. Iteration 2 had the same visible outcome in most slow cases, because the late RPC usually failed against a daemon that was not yet up. Before WR-05, the preference was always honoured. Nothing hides the window once startup finishes and the preference becomes readable.
**Fix:** Accept and document it in the M-NN manual checklist, or keep a GUI-side copy of the preference (for example in `UserDefaults` or a small file next to the socket, updated by `SetStartMinimized`), so `domReady` can decide without the daemon.

### IN-15: `startupBindingWait` (90 s) does not cover `EnsureDaemon`'s actual worst case, contrary to its comment

**File:** `app.go:155-158`; `internal/daemon/process.go:168-220`
**Issue:** The comment says 90 s "covers EnsureDaemon's worst case with 30 s client timeouts". `EnsureDaemon` makes its own client with the 30 s timeout. A daemon that answers `/health` but hangs on `/relay-port` costs 30 s at `:190`. The spawn follows, and the 3 s poll loop checks its deadline only between iterations, so one `Health` (30 s) plus one `GetRelayPort` (30 s) inside it costs up to 60 s more. That adds up to about 90 s plus spawn time, or more with the `GetDaemonVersion`/`ShutdownDaemon` branch. When startup overruns, `init` shows "daemon startup timed out" even though startup then succeeds a moment later, and nothing clears the banner until the user clicks Retry. The fix report already lists this recovery under "Remaining limit". A late failure does replace the banner text through `daemon:error`.
**Fix:** Correct the comment ("covers the common worst case; past it, Retry recovers"). Alternatively, bound `EnsureDaemon` itself with a context deadline so the two limits agree.

---

_Reviewed: 2026-11-16T01:14:28Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
