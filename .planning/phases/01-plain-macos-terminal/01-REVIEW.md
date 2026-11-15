---
phase: 01-plain-macos-terminal
reviewed: 2026-11-15T00:59:19Z
depth: standard
files_reviewed: 199
files_reviewed_list:
  - .github/workflows/build.yml
  - .github/workflows/distribute.yml
  - .github/workflows/release.yml
  - .gitignore
  - app.go
  - app_exit_test.go
  - app_save_terminal_test.go
  - app_test.go
  - build.sh
  - cmd/playwright-fixture/main.go
  - cmd_attach.go
  - cmd_attach_test.go
  - cmd_cli.go
  - cmd_cli_test.go
  - cmd_remote.go
  - cmd_remote_test.go
  - frontend/e2e/chat-parity.spec.ts
  - frontend/e2e/fixture-env.ts
  - frontend/e2e/global-setup.ts
  - frontend/e2e/web-csp.spec.ts
  - frontend/e2e/web-share-scope.spec.ts
  - frontend/package.json
  - frontend/src/App.tsx
  - frontend/src/__tests__/themeTokens.test.ts
  - frontend/src/components/Hub/ChatMessage.test.tsx
  - frontend/src/components/Hub/ChatMessage.tsx
  - frontend/src/components/Hub/ChatPanel.test.tsx
  - frontend/src/components/Hub/ChatPanel.tsx
  - frontend/src/components/Hub/HubEmptyState.test.tsx
  - frontend/src/components/Hub/HubEmptyState.tsx
  - frontend/src/components/Hub/HubFilterBar.test.tsx
  - frontend/src/components/Hub/HubFilterBar.tsx
  - frontend/src/components/Hub/HubInteractiveModal.test.tsx
  - frontend/src/components/Hub/HubModal.test.tsx
  - frontend/src/components/Hub/HubModal.tsx
  - frontend/src/components/Hub/HubPanel.test.tsx
  - frontend/src/components/Hub/HubPanel.tsx
  - frontend/src/components/Hub/MentionPopover.test.tsx
  - frontend/src/components/Hub/MentionPopover.tsx
  - frontend/src/components/Hub/SessionCard.test.tsx
  - frontend/src/components/Hub/SessionCard.tsx
  - frontend/src/components/Hub/SessionCardGrid.test.tsx
  - frontend/src/components/Hub/SessionCardGrid.tsx
  - frontend/src/components/Hub/SessionShareModal.tsx
  - frontend/src/components/Hub/TerminalChatHost.tsx
  - frontend/src/components/Hub/useChatUnreadListeners.test.tsx
  - frontend/src/components/NewSessionModal.tsx
  - frontend/src/components/QuitConfirmModal.tsx
  - frontend/src/components/RemoteJoinCodeModal.test.tsx
  - frontend/src/components/RemoteJoinCodeModal.tsx
  - frontend/src/components/SessionShare/TailnetTab.tsx
  - frontend/src/components/SettingsJumpBar.tsx
  - frontend/src/components/SettingsSearch.tsx
  - frontend/src/components/SettingsTab.tsx
  - frontend/src/components/ShellWebShareBanner.tsx
  - frontend/src/components/Sidebar.tsx
  - frontend/src/components/StatusBar.tsx
  - frontend/src/components/TabBar.tsx
  - frontend/src/components/TailscaleBanner.tsx
  - frontend/src/components/WelcomeTab.tsx
  - frontend/src/components/__tests__/App.createTab.stayOnHub.test.tsx
  - frontend/src/components/__tests__/App.hub.test.tsx
  - frontend/src/components/__tests__/App.open-remote.test.tsx
  - frontend/src/components/__tests__/App.shellExit.test.tsx
  - frontend/src/components/__tests__/App.shellWebShare.test.tsx
  - frontend/src/components/__tests__/App.tailscale.test.tsx
  - frontend/src/components/__tests__/App.test.tsx
  - frontend/src/components/__tests__/NewSessionModal.test.tsx
  - frontend/src/components/__tests__/QuitConfirmModal.test.tsx
  - frontend/src/components/__tests__/SessionCard.share.test.tsx
  - frontend/src/components/__tests__/SessionShareModal.disconnect.test.tsx
  - frontend/src/components/__tests__/SessionShareModal.test.tsx
  - frontend/src/components/__tests__/SettingsTab.appearance-theme.test.tsx
  - frontend/src/components/__tests__/SettingsTab.hyperlinked-index.test.tsx
  - frontend/src/components/__tests__/SettingsTab.persistence.test.tsx
  - frontend/src/components/__tests__/SettingsTab.shell-warn-toggle.test.tsx
  - frontend/src/components/__tests__/SettingsTab.stay-on-hub-toggle.test.tsx
  - frontend/src/components/__tests__/SettingsTab.tailscale-status.test.tsx
  - frontend/src/components/__tests__/SettingsTab.test.tsx
  - frontend/src/components/__tests__/ShellWebShareBanner.test.tsx
  - frontend/src/components/__tests__/Sidebar.test.tsx
  - frontend/src/components/__tests__/StatusBar.shareSession.test.tsx
  - frontend/src/components/__tests__/TabBar.test.tsx
  - frontend/src/components/__tests__/TailnetTab.test.tsx
  - frontend/src/components/__tests__/TailscaleBanner.test.tsx
  - frontend/src/components/__tests__/TerminalPanel.scale.test.tsx
  - frontend/src/components/__tests__/WelcomeTab.install.test.tsx
  - frontend/src/components/__tests__/WelcomeTab.test.tsx
  - frontend/src/components/__tests__/style.contrast.test.ts
  - frontend/src/components/__tests__/style.hub.modal.test.ts
  - frontend/src/components/__tests__/style.hub.test.ts
  - frontend/src/components/__tests__/style.redesign.test.ts
  - frontend/src/components/__tests__/style.settings.test.ts
  - frontend/src/lib/__tests__/remoteAdapter.test.ts
  - frontend/src/lib/__tests__/remoteSession.test.ts
  - frontend/src/lib/hubGroupCounts.test.ts
  - frontend/src/lib/hubGroupCounts.ts
  - frontend/src/lib/relayClient.test.ts
  - frontend/src/lib/relayClient.ts
  - frontend/src/lib/remoteAdapter.test.ts
  - frontend/src/lib/remoteAdapter.ts
  - frontend/src/lib/remoteSession.ts
  - frontend/src/lib/tailscale.ts
  - frontend/src/lib/vtColor.ts
  - frontend/src/lib/webMode.ts
  - frontend/src/style.css
  - frontend/src/wailsjs/go/main/App.d.ts
  - frontend/src/wailsjs/go/main/App.js
  - frontend/src/wailsjs/go/models.ts
  - frontend/src/wailsjs/wailsjs/go/models.ts
  - go.mod
  - go.sum
  - internal/capability/capability.go
  - internal/capability/capability_test.go
  - internal/daemon/alias_store_test.go
  - internal/daemon/api.go
  - internal/daemon/api_shell_warn_test.go
  - internal/daemon/api_stayonhub_test.go
  - internal/daemon/api_test.go
  - internal/daemon/chat.go
  - internal/daemon/chat_routes.go
  - internal/daemon/chat_routes_test.go
  - internal/daemon/chat_test.go
  - internal/daemon/client.go
  - internal/daemon/client_test.go
  - internal/daemon/engine.go
  - internal/daemon/engine_exit_test.go
  - internal/daemon/engine_migration_test.go
  - internal/daemon/engine_natural_exit_test.go
  - internal/daemon/engine_plugins_test.go
  - internal/daemon/engine_settings_test.go
  - internal/daemon/engine_shell_warn_test.go
  - internal/daemon/engine_stayonhub_test.go
  - internal/daemon/engine_test.go
  - internal/daemon/files_removed_test.go
  - internal/daemon/funnel_test.go
  - internal/daemon/image_config_test.go
  - internal/daemon/open_remote_session_url_test.go
  - internal/daemon/path.go
  - internal/daemon/path_test.go
  - internal/daemon/plugin_settings.go
  - internal/daemon/process.go
  - internal/daemon/process_test.go
  - internal/daemon/relay_remote_files.go
  - internal/daemon/remote_caps.go
  - internal/daemon/remote_files.go
  - internal/daemon/remote_ws_proxy_test.go
  - internal/daemon/search_config_test.go
  - internal/daemon/settings_removed_test.go
  - internal/daemon/socket.go
  - internal/daemon/socket_test.go
  - internal/daemon/types.go
  - internal/daemon/web_links_config_test.go
  - internal/pty/native.go
  - internal/pty/reap_other.go
  - internal/pty/session.go
  - internal/relay/fuzz_test.go
  - internal/relay/hub.go
  - internal/relay/hub_chatsend_test.go
  - internal/relay/protocol.go
  - internal/relay/protocol_chat_test.go
  - internal/relay/sanitize.go
  - internal/relay/sanitize_test.go
  - internal/relay/server.go
  - internal/relay/server_chatsend_test.go
  - internal/relay/server_cors_test.go
  - internal/relay/server_identity_test.go
  - internal/relay/server_inject_test.go
  - internal/relay/server_test.go
  - internal/statusbar/bar.go
  - internal/statusbar/bar_test.go
  - internal/tailnet/sessions.go
  - internal/tailnet/tailnet_test.go
  - internal/webserver/capability_mw.go
  - internal/webserver/capability_test.go
  - internal/webserver/capability_test_helpers.go
  - internal/webserver/chat.go
  - internal/webserver/files_removed_test.go
  - internal/webserver/funnel_test.go
  - internal/webserver/fuzz_test.go
  - internal/webserver/inject_test.go
  - internal/webserver/rwgate_test.go
  - internal/webserver/server.go
  - internal/webserver/server_chatsend_test.go
  - internal/webserver/server_test.go
  - internal/webserver/sessions_meta_embed_test.go
  - internal/webserver/sessions_meta_test.go
  - internal/webserver/tailscale.go
  - internal/webserver/tailscale_test.go
  - main.go
  - notification_darwin.go
  - notification_darwin_test.go
  - tests/build-script.test.sh
  - tests/check-traceability-paths.sh
  - tests/fixtures/settings_agenthub_v4.2.json
  - tools.go
  - tray.go
  - tray_objc_darwin.m
  - tray_test.go
findings:
  critical: 1
  warning: 8
  info: 6
  total: 15
  necessity: 0
status: issues_found
---

# Phase 1: Code Review Report

**Reviewed:** 2026-11-15T00:59:19Z
**Depth:** standard
**Files Reviewed:** 199 (86 non-test source files read in full or by diff; test files skimmed for what they assert)
**Status:** issues_found

## Summary

Scope: the diff `aaedd8c2..HEAD` (Phase 1: plain macOS terminal). I confirmed the current state with `go build ./...` (clean), `go vet ./...` (clean), `go mod tidy -diff` (clean), `go test -short ./...` (all packages pass), `go test -race -short -run 'Exit|Health|Tailscale|Retry' .` (passes), `tsc --noEmit` on the frontend (clean), and `bash tests/check-traceability-paths.sh` (120 paths OK). No removed symbol is still referenced. The bindings match `app.go`, apart from one older orphan noted below.

The removals are mostly clean. The capability, relay and webserver security surfaces did not regress:
- Minting is limited to `read` / `read,write`.
- Legacy `files.*` tokens still read as read-only through the whole-token `HasPerm(write)`.
- The retired 0x35/0x36 frames have no read-pump arm.
- `requireCapability` still binds `claims.SID` to the path id.
- The `/assets` invalid-UTF-8 404 is correct.

Legacy settings migration (unknown keys ignored, dropped on the next save) behaves as CONTEXT specifies.

The defects are in the two new runtime mechanisms:

1. **Tailscale state can go stale (CR-01).** The Go poller emits only on change. The frontend also writes health state from a second, independent path (the focus/visibility recheck and the init call). One transient "not connected" read from that path sticks with no expiry. The banner stays up and Share stays disabled after Tailscale recovers, which breaks NET-01's "clear the banner when the connection is back".
2. **The exit watcher only covers part of the lifecycle (WR-01..WR-04).** It reports only sessions it catches in `State == "stopped"`. It reports each one exactly once, and the frontend must then perform the removal. Sessions that disappear without a "stopped" sighting leave dead tabs. So does a failed removal, or a fast-exiting shell. An open Hub modal is never told.

Known/accepted items (public-write gate state unread, T-01-42 LAN fallback) are not re-reported. IN-04 is only a note that a new UI path now contradicts its own copy.

## Critical Issues

### CR-01: Tailscale health can stick at "not connected" after recovery. The frontend writes state outside the change-only poller.

**File:** `app.go:1021-1025`, `app.go:1270-1279`, `frontend/src/App.tsx:436-445`, `frontend/src/App.tsx:963-980`
**Issue:** `checkTailscaleHealth` emits `tailscale:health` only when the result differs from `a.lastHealth`, which only the poller goroutine updates. Two other paths set the frontend state directly, through `GetTailscaleStatus()` → `setTailscaleHealth`:
- the focus/visibility `recheck` (App.tsx:966-970);
- the `init` / `retryInit` `Promise.all` (App.tsx:439, 1205).

`GetTailscaleStatus` (app.go:1021) runs a fresh check that never touches `lastHealth`. Concrete sequence:
1. The poller's `lastHealth` is `{connected:true, ...}`.
2. The user wakes the Mac. The window `focus` / `visibilitychange` fires while tailscaled is still re-establishing. LocalAPI reports BackendState `Starting`, or errors, so `checkHealth` fails closed and returns `{connected:false}`. The frontend sets it: the banner shows, Share is disabled, an open Share dialog is force-closed (App.tsx:958-960), and remote peers are cleared (App.tsx:930-933).
3. Tailscale reaches `Running` again before the next 10 s tick. The poller computes `h == *a.lastHealth` (still connected, same IP/domain/AcceptDNS) and emits nothing.
4. The UI stays "not connected" until another focus event or a real state change happens. Nothing tells the user it is stale.

A late-resolving `GetTailscaleStatus` from step 2 can also overwrite a newer `tailscale:health` event. Responses are not ordered against events.
**Fix:** Keep one source of truth. Route every check through the poller's state and emit the result:
```go
// app.go
type App struct { /* ... */ healthMu sync.Mutex; lastHealth *webserver.TailscaleHealth }

func (a *App) checkTailscaleHealth(ctx context.Context) webserver.TailscaleHealth {
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	h := a.tailscaleCheck(checkCtx)
	cancel()
	a.healthMu.Lock()
	changed := a.lastHealth == nil || *a.lastHealth != h
	a.lastHealth = &h
	a.healthMu.Unlock()
	if changed {
		a.emitEvent("tailscale:health", h)
	}
	return h
}

func (a *App) GetTailscaleStatus() webserver.TailscaleHealth {
	return a.checkTailscaleHealth(context.Background())
}
```
Once the poller knows about every value the frontend has seen, it will emit the recovery. A simpler alternative is to have the poller emit every tick, unconditionally. Add a test: poller reports connected, `GetTailscaleStatus` returns not-connected, the next poll returns connected, and an event must be emitted.

## Warnings

### WR-01: The exit watcher never reports sessions removed without a "stopped" sighting, which leaves dead tabs.

**File:** `app.go:329-353`
**Issue:** `checkExits` emits `session:exit` only for `s.State == "stopped"`. Several paths remove a session from the daemon registry without ever exposing that state to a 500 ms poll:
- `agenthub kill <id>`, and any other socket client calling `DELETE /sessions/{id}`: `NativePTYBackend.Kill` sets `StateStopped` and calls `registry.Remove` in the same call (`internal/pty/native.go:110-113`).
- A daemon crash or restart: the whole list is replaced.

The id simply disappears from `ListSessions`. The pruning loop (app.go:348-352) only deletes from `exitEmitted` and emits nothing. The GUI tab stays open as a dead terminal, and `hubSessions` / `webEnabled` keep stale entries. CONTEXT Amendment 2 says "when a session's shell exits, for any reason ... its tab closes immediately and the session is removed".
**Fix:** Track every id the watcher has seen. Treat "previously seen, now missing" as an exit too:
```go
for id := range a.seen {
	if !listed[id] {
		if !a.exitEmitted[id] { a.emitEvent("session:exit", map[string]any{"sessionId": id}) }
		delete(a.seen, id); delete(a.exitEmitted, id)
	}
}
```
On the frontend, `handleCloseTab`'s `KillSession` "not found" error is already tolerated.

### WR-02: `session:exit` fires once and leaves the removal to the frontend, so a failed `KillSession` leaks the session for the life of the GUI.

**File:** `app.go:341-345`, `frontend/src/App.tsx:744-752`
**Issue:** The watcher sets `exitEmitted[id] = true` before anything consumes the event. The daemon-side removal depends entirely on the frontend calling `KillSession`. `handleCloseTab` logs the `KillSession` failure (`console.warn`) and removes the tab anyway. The event is also lost if the webview is not subscribed when it fires. If the kill fails (socket timeout, daemon busy), the stopped session stays in the daemon. It still appears in `agenthub list`, the tray menu and the Hub. Its chat store and web/funnel grants are only reclaimed by the 10 s `onExit` cleanup. It is never re-emitted, because `exitEmitted` keeps it as long as the daemon lists it. CLAUDE.md's "Silent Fallbacks" rule applies: the failure becomes silent residue.
**Fix:** Make removal the watcher's job, idempotent on the Go side, and let the event only drive the UI:
```go
if s.State == "stopped" {
	if err := a.client.KillSession(s.ID); err != nil { continue } // retry next tick
	if !a.exitEmitted[s.ID] { a.exitEmitted[s.ID] = true; a.emitEvent("session:exit", ...) }
}
```
Alternatively, do not mark `exitEmitted` until the id disappears from the list, and re-emit while it is still listed as stopped.

### WR-03: A shell that exits before its tab is added leaves a permanent dead tab.

**File:** `frontend/src/App.tsx:664-671`, `frontend/src/App.tsx:607-616`
**Issue:** `createTab` adds the tab only after `await CreateSession(...)` resolves. Take a shell that exits within about one watcher tick: an `exit` in `.zprofile`/`.zshrc`, or a broken login configuration. The watcher can emit `session:exit` before the RPC response reaches JS. `handleCloseTab` then runs `KillSession` and filters a tab that does not exist yet. After that, `createTab` runs `setTabs([...prev, tab])` and `setHubSessions([...prev, {...state:'running'}])`. The result is a tab and Hub entry for a session the daemon has already removed. `exitEmitted` suppresses any re-report. The window is narrow, but the failure is permanent for the GUI session.
**Fix:** Record exited ids in a ref, for example `exitedIdsRef.current.add(sessionId)` in the `session:exit` handler. In `createTab`, check it after `CreateSession` resolves and skip adding the tab. With WR-01's "seen then missing" logic, the next tick would report the id again anyway.

### WR-04: The Hub modal stays open on an exited or removed session with no indication.

**File:** `frontend/src/components/Hub/HubPanel.tsx:234`, `frontend/src/components/Hub/HubPanel.tsx:507-527`, `frontend/src/components/Hub/HubModal.tsx` (status header removed)
**Issue:** `modalState.session` is a snapshot. Nothing closes the modal when the session exits. The `session:exit` handler only calls `handleCloseTab`, and HubPanel never reconciles `modalState` against `sessions`. Before this phase, the modal header showed "Exited"/"Done" via `STATUS_CONFIG`. That was removed under Amendment 2, and no auto-close replaced it. The host path in `TerminalPanel` (`onClose` at TerminalPanel.tsx:360) sets `sessionEnded` only for guests. The user is left with a frozen terminal that still accepts keystrokes, which go nowhere.
**Fix:** In HubPanel, close the modal when its session leaves the local list:
```tsx
useEffect(() => {
  if (modalState && !remoteIdSet.has(modalState.session.id) &&
      !sessions.some((s) => s.id === modalState.session.id)) setModalState(null)
}, [sessions, modalState, remoteIdSet])
```
Also remove the session from `hubSessions` in App's `session:exit` handler so the Hub reflects the exit immediately.

### WR-05: `RetryDaemon` starts a new health poller on every call and writes `a.client` while background goroutines read it.

**File:** `app.go:192-206`, `app.go:1283-1298`
**Issue:** `startExitWatcher` has a `sync.Once` guard. `startHealthPoller` has none. `RetryDaemon` can be reached after a successful start: `retryInit` runs whenever `init` / `retryInit` hit their `catch` and call `setDaemonError`, for example when a `Promise.all` RPC rejects. Each successful `RetryDaemon` then starts another poller goroutine. Two pollers read and write `a.lastHealth` with no synchronisation, which is a data race under `-race`, and they emit duplicate events. `RetryDaemon` also assigns `a.client` while the exit watcher (`checkExits` reads `a.client`), the tray poller and the health poller may be running. That is another unsynchronised write.
**Fix:** Guard the health poller with a `sync.Once`, as the exit watcher is. Start it in `startup` regardless of daemon state: it does not use the daemon. Store `client` behind an `atomic.Pointer[daemon.DaemonClient]` or a mutex.

### WR-06: `GetSessionStatus` reports "running" for unknown sessions, and nothing in production consumes it.

**File:** `internal/daemon/engine.go:494-503`, `internal/daemon/api.go:773-777`, `internal/daemon/client.go:88-89`
**Issue:** `GetSessionStatus` falls back to `"running"` when the id is not found. `GET /sessions/{id}/status` therefore returns `200 {"status":"running"}` for sessions that never existed or were just removed. That is a silent fallback (CLAUDE.md), and it is exactly wrong now that exit detection matters. The 01-10 summary says this route stays for "CLI, web list, peers". In fact `DaemonClient.GetSessionStatus` has no non-test caller, and the webserver resolver uses `s.Status` from `ListSessions`.
**Fix:** Return a not-found signal and a 404:
```go
func (e *SessionEngine) GetSessionStatus(id string) (string, bool) { ...; return "", false }
// handler: if !ok { http.Error(w, "session not found", http.StatusNotFound); return }
```
Or delete the route and client method if nothing needs them (that decision is the user's).

### WR-07: `agenthub new` resolves relative paths in the daemon's cwd and silently drops extra or legacy arguments.

**File:** `cmd_cli.go:63-86`
**Issue:**
1. The positional `workDir` is sent verbatim. The daemon runs `cmd.Dir = req.WorkDir` (`internal/pty/native.go:40`) with its own cwd (`/` under launchd). So `agenthub new .` or `agenthub new myproj` starts in the wrong directory, or fails.
2. Only `positionals[0]` is used. The removed forms that users and scripts still type are silently misread:
   - `agenthub new claude ~/proj` becomes workDir `claude`, and `~/proj` is ignored.
   - `agenthub new shell ~/proj` becomes workDir `shell`.
3. Every `CreateSession` error is printed as `daemon unreachable: ...`, including chdir failures, which misdirects the user.
**Fix:**
```go
if len(positionals) > 1 { return fmt.Errorf("usage: agenthub new [<path>] (got %d arguments)", len(positionals)) }
if workDir != "" { if abs, err := filepath.Abs(workDir); err == nil { workDir = abs } }
// ...
fmt.Fprintf(os.Stderr, "agenthub new: %v\n", err)
```

### WR-08: The rewritten CI jobs have no `timeout-minutes`.

**File:** `.github/workflows/build.yml:6-17`, `.github/workflows/release.yml:9-40` (and `build-macos`)
**Issue:** Each job is now a single macOS runner that runs `go test -race ./...` (including the root package with PTY and process tests), the frontend tests and a Wails build. These jobs have no job-level `timeout-minutes`, so a hung PTY test holds a macOS runner (10x billing) for the 6 h default. `e2e.yml` already sets `timeout-minutes: 20`.
**Fix:** Add `timeout-minutes: 30` to `build`, and to `validate` and `build-macos` in `release.yml`.

## Info

### IN-01: Stale comments that now contradict the code

**File:** `internal/daemon/engine.go:358-366`, `internal/pty/cleanup.go:37`, `frontend/src/components/SettingsTab.tsx:53-55`, `tray_objc_darwin.m:245-252`, `frontend/src/wailsjs/go/main/App.js:69`
**Issue:**
- engine.go says "On Linux/Windows ReapNaturalExit is a no-op: the platform exit detector already cached the code". No exit detector exists on any platform now, and `reap_other.go` is untagged.
- cleanup.go mentions "a racing exit detector (Linux)".
- The SettingsTab doc comment, newly written in this phase, says "shell and tailscale path overrides". Both were removed.
- tray_objc describes notifications for "your session needs input".
- App.js has an empty `// Auto-install Tailscale bound method` header.

**Fix:** Update or delete these comments.

### IN-02: `QuitGUIOnly` counts active sessions by `Status != "stopped"`, a value `Status` never takes

**File:** `app.go:247-252`
**Issue:** `Status` is now `running|exited|errored`. The check is always true for non-empty status, so exited sessions are counted as "active". Impact is small because the exit watcher removes them within about 500 ms.
**Fix:** Use `s.State != "stopped"`.

### IN-03: The Linux Playwright fixture has no exit detection at all

**File:** `internal/pty/reap_other.go:1`, `internal/pty/native.go` (removed `startExitDetector`)
**Issue:** The `exit_linux.go` Wait4 detector was deleted, and `reap_other.go` lost its build tag so the ubuntu e2e fixture still compiles. On Linux, PTY master EOF does not surface. `hub.Done()` therefore never fires, and fixture sessions never reach "stopped". No current e2e test depends on this, but a future exit-path e2e test would hang.
**Fix:** Document this limitation in `reap_other.go`, or add a `//go:build darwin` note plus a fixture-side guard.

### IN-04: "Start Web Server" with Tailscale off starts LAN mode while the banner says sharing is off (T-01-42-adjacent)

**File:** `app.go:731-744`, `frontend/src/components/SettingsTab.tsx:301-321`
**Issue:** When `GetTailscaleStatus` is not connected, `StartWebServer` silently falls back to `local` mode, which exposes the server on the LAN. Meanwhile the persistent banner reads "Remote access and sharing are off". 01-12 deleted `LocalNetworkBanner`, so the only remaining indicator of LAN exposure is inside Settings. This is a known Phase 4 item. It is noted here only because the UI copy and the actual state now disagree on this path.
**Fix:** Defer to Phase 4. At minimum, disable "Start Web Server" with `TAILSCALE_OFF_REASON` while `tailscaleOff`.

### IN-05: The traceability parser drops the second of two adjacent path cells

**File:** `tests/check-traceability-paths.sh:22-23`
**Issue:** `grep -oE '\| [^|]+\.(go|ts|tsx|sh) \|'` consumes the closing pipe. In a row `| a.go | b.go |`, the second cell never matches. No current row has this shape (verified: 0 matches), but a future row could hide a missing path.
**Fix:** Split on `|` first, for example `awk -F'|' '{for(i=2;i<NF;i++) print $i}'`, then filter cells that match `\.(go|ts|tsx|sh)$`.

### IN-06: `GetSessionQRCode` binding has no Go method

**File:** `frontend/src/wailsjs/go/main/App.d.ts:51`, `frontend/src/wailsjs/go/main/App.js:33`
**Issue:** This binding was already orphaned at `aaedd8c2`, and this phase hand-edited both files. Calling it rejects at runtime. It has no callers today.
**Fix:** Remove both lines.

---

_Reviewed: 2026-11-15T00:59:19Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
