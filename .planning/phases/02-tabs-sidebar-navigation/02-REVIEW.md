---
phase: 02-tabs-sidebar-navigation
reviewed: 2026-11-16T00:30:40Z
depth: standard
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
  warning: 6
  info: 10
  total: 16
  necessity: 1
status: issues_found
---

# Phase 2: Code Review Report

**Reviewed:** 2026-11-16T00:30:40Z
**Depth:** standard
**Files Reviewed:** 129 (56 non-test source files read in full or by diff; test files consulted for coverage)
**Status:** issues_found

## Summary

I reviewed the Phase 2 diff (`c2f4255d..HEAD`): the cgo/ObjC Appearance bridge, the `startupDone` gate between `startup` and `domReady`, removal of the chat and presence relay frames, the `appearance` settings API, the new sidebar, tab bar, context menu and dialog focus code, and the restyled Share, Settings and confirm dialogs. Ground truth passes: `go build ./... && go vet ./...` and `pnpm exec tsc --noEmit` both exit 0.

These areas checked out:
- **Retired frames 0x30-0x37:** both read pumps (`internal/relay/server.go:153`, `internal/webserver/server.go:1327`) have no case and no default arm, so these frames are dropped before any PTY write. `parseServerFrame` maps them to `unknown`, so a peer still running older code cannot crash the client. Fuzz and inject tests cover both pumps.
- **Appearance enum:** validated in three places, `App.SetAppearance` (`app.go:545`), `SessionEngine.SetAppearance` (`engine.go:643`) and `handleSetAppearance`, which returns 400. Stored values are normalized on load, so a missing or garbage value reads as `system`, including on the fresh-install path.
- **ObjC bridge:** `dispatch_async` to the main queue, with `mode` captured by value. FIFO order on the main queue means `WindowShow` from `domReady` runs after the Appearance is set. Startup does no synchronous main-thread work (the tray and `EventsEmit` both hop async), so I found no deadlock as long as the daemon I/O returns (see WR-05).
- **End Session:** the client-side `ToggleWebServing(false)` is defense in depth. `handleDeleteSession` already runs `runSessionExitCleanup` (DisableSession, ClearGrants, ref-counted Funnel teardown).
- **`OpenTailscale`:** fixed argv and a fixed URL. No frontend value reaches `exec`.
- **Capability, CSP and Funnel consent:** not weakened. The web-mode sidebar and tab chrome stay hidden. `FunnelRiskPanel` and the write gate changed styling only. The webserver sets `Origin: "web"` again, so host-authority resize still works.

The main problems are lifecycle and race issues around the new flows: Esc bypasses the disabled Cancel during an in-flight End Session, the tray can no longer reopen detached sessions, the stored Appearance is lost on the Retry path, concurrent Appearance writes are unordered, the `startupDone` wait has no bound, and the session poll can resurrect an exited session. Nothing reaches BLOCKER level.

## Narrative Findings (AI reviewer)

## Warnings

### WR-01: Esc cancels a confirm while its action is in flight, bypassing the disabled Cancel button

**File:** `frontend/src/components/RegenerateKeyModal.tsx:53-76` (with `frontend/src/lib/useDialogFocus.ts:57-61`)
**Issue:** While `acting` is true, both buttons are disabled so the user cannot cancel. But `useDialogFocus` still maps Escape to `onCancel`. For End Session (`App.tsx:1352-1356`), Esc during "Ending…" closes the dialog as if cancelled, yet `endSession` and `KillSession` keep running and the shell is killed anyway. The UI tells the user they cancelled when they did not. Esc also triggers the reset effect (`isOpen` → false, which clears `error` and `acting`). If `onConfirm` then rejects, `handleConfirm`'s catch calls `setError(msg)` on the closed modal, and the next End Session or Regenerate dialog opens showing that stale error. The same applies to the Regenerate Signing Key and "Stop asking" confirms in `SettingsSheet`.
**Fix:** Ignore Esc while acting, and clear the error on open rather than only on close:
```tsx
useDialogFocus(dialogRef, {
  onCancel: () => { if (!actingRef.current) onCancel() },
  initialFocus: 'cancel',
  active: isOpen,
})
useEffect(() => { if (isOpen) { setError(null); setActing(false) } }, [isOpen])
```
(keep an `actingRef` in sync with `acting`, since the hook reads the latest `onCancel` through a ref).

### WR-02: Tray "focus session" does nothing for a detached session (regression from detach-only close)

**File:** `frontend/src/App.tsx:501-509`
**Issue:** `onTraySession` (`tray.go:73-88`) lists every daemon session and emits `tray:focus-session`. The handler only activates an existing tab (`prev.find(...)`) and otherwise does nothing. TERM-03 makes closing a tab detach only, so a closed tab's session keeps running and stays in the tray menu. Clicking it shows the window but opens nothing. Before this phase a running session nearly always had a tab, so this is a new dead path. CLI-created sessions hit it too.
**Fix:** Find the tab or open one, inside the functional updater so the `[]`-deps listener never reads stale state:
```tsx
const cancelTrayFocus = EventsOn('tray:focus-session', (sessionId: string) => {
  if (exitedIdsRef.current.has(sessionId)) return
  setTabs((prev) => prev.some((t) => t.id === sessionId)
    ? prev
    : [...prev, { id: sessionId, name: 'shell', sessionId }])
  setActiveId(sessionId)
})
```
(the label comes from `tabLabel(tab, sessions)`, so the placeholder name is never shown once the poll lands).

### WR-03: The stored Appearance is never applied after "Retry Connection"

**File:** `app.go:218-225`, `frontend/src/App.tsx:1004-1054`
**Issue:** `startup` calls `applyStoredAppearance()` only when the first `EnsureDaemon` succeeds (`app.go:179-181`). If the first attempt fails and the user clicks Retry Connection, `RetryDaemon` brings the daemon up but never applies the stored override, and `retryInit` never calls `GetAppearance`. A user with Dark (or Light) stored gets the system appearance in the chrome, the terminals and the sidebar vibrancy until the next launch, while Settings shows Dark selected. That contradicts THEM-01 ("applies before the window first paints… applies live").
**Fix:** In `RetryDaemon`, after a successful `EnsureDaemon`, call `a.applyStoredAppearance()`. In `retryInit`, add the same `GetAppearance().then(...)` hydration that `init` does (lines 419-421).

### WR-04: Changing the Appearance quickly can leave the stored, native and on-screen values out of sync

**File:** `frontend/src/components/SettingsSheet.tsx:265-286`; `app.go:542-557`
**Issue:** `selectAppearance` applies the change optimistically and fires `SetAppearance` with no in-flight guard. Arrow-key navigation (`handleAppearanceKeyDown`) makes repeated calls easy. Each Wails binding call runs in its own goroutine (`wails/v2 darwin/frontend.go:388`), so two PATCHes can reach the daemon out of order, and their `setNativeAppearance` hops can land out of order too. Pressing Right twice (Automatic → Light → Dark) can leave Light persisted and applied natively while the control shows Dark. A failed earlier call also rolls back to its own `prev`, overwriting a newer successful choice.
**Fix:** Serialize the calls. Ignore input while a save is in flight (as `Switch`'s `saving` does), or keep a monotonically increasing request id and apply rollback or `onAppearanceChange` only for the latest request. On the Go side, guard `SetAppearance` with a mutex so the persist and apply pair is atomic per call.

### WR-05: `domReady` waits on `startupDone` with no bound, and the daemon client has no timeout

**File:** `app.go:144-146`, `app.go:175-184`; `internal/daemon/client.go:32-35`
**Issue:** `domReady` runs on the single Wails message-processor goroutine (`frontend.go:152-154, 361-365`) and now blocks until `startup` closes `startupDone`. Before closing it, `startup` runs `EnsureDaemon` and `GetAppearance`, both over an `http.Client` with no `Timeout`. A wedged daemon (socket accepting, handler stuck on `e.mu`) blocks `startup` forever. The window then never shows, every later frontend message (binding calls, events) queues behind `domReady`, and once the 100-slot `messageBuffer` fills, the exported `processMessage` blocks the AppKit main thread. `close(a.startupDone)` is also not deferred, so any future early return or panic recovery in `startup` would reproduce the hang. The old `domReady` had a similar hang through `GetStartMinimized`. The new gate widens it to the whole message loop and adds an RPC to the critical path.
**Fix:** Bound the wait and make the close unconditional:
```go
// startup
defer func() { if a.startupDone != nil { close(a.startupDone) } }() // or a sync.Once
// domReady
select {
case <-a.startupDone:
case <-time.After(5 * time.Second): // show anyway; Appearance lands when startup finishes
}
```
Also give `DaemonClient.http` a request timeout (for example 5 s, with streaming endpoints exempt). Add a test for the domReady/startup ordering; `app_test.go` has none.

### WR-06: The session poll can bring back an exited or ended session, and clicking it opens a tab that never closes

**File:** `frontend/src/App.tsx:741-755`, `frontend/src/App.tsx:898-911`, `frontend/src/App.tsx:648-655`
**Issue:** The 3 s poll does `setSessions(list)` unfiltered. A `ListSessions` call already in flight when `session:exit` fires (or when `endSession` removes the row) can resolve afterwards with the dead session still listed, so the sidebar shows it again for up to one poll interval. `handleOpenSessionTab` does not check `exitedIdsRef`, so clicking that row opens a terminal tab for a dead session. `session:exit` is reported only once, so nothing ever closes that tab. `exitedIdsRef` exists for exactly this race (`createTab` and the restore paths use it), but the poll and the sidebar open path skip it.
**Fix:** Filter the poll result and guard the open path:
```tsx
if (!cancelled) setSessions(list.filter((s) => !exitedIdsRef.current.has(s.id)))
// handleOpenSessionTab
if (exitedIdsRef.current.has(sessionId)) return
```
and add the killed id to `exitedIdsRef` in `endSession` after `KillSession` resolves.

## Info

### IN-01: A failed rename reverts silently, and the poll can flicker the name

**File:** `frontend/src/App.tsx:660-669`
**Issue:** A `RenameSession` failure only does `console.warn` and puts the old name back with no visible reason. A poll that started before the RPC landed can also briefly show the old name over the optimistic one. With two quick renames where the first fails after the second succeeds, the revert sets the name back to the original while the daemon holds the second name, until the next poll.
**Fix:** Show the failure in `saveBanner` (it already exists for this kind of message). Revert only if the current name still equals the attempted one.

### IN-02: Menu shortcuts act behind open dialogs

**File:** `frontend/src/App.tsx:524-533`
**Issue:** ⌘W closes (detaches) the active tab while Settings, Share or the End Session confirm is open. ⌘T stacks New Session over Settings. A native sheet would block these.
**Fix:** In the listeners, return early when `document.querySelector('[aria-modal="true"]')` matches (or track a `modalOpen` ref).

### IN-03: Dead Hub-era code and stale comments left behind

**File:** `app.go:428-447`, `internal/daemon/api.go:157-158, 680-720`, `internal/daemon/engine.go:458-560`, `internal/daemon/client.go:88-110`, `frontend/src/wailsjs/go/main/App.d.ts:53-55`, `internal/relay/hub.go:88-90, 164-165`, `internal/webserver/server.go:1166-1173`, `frontend/src/components/RemoteJoinCodeModal.tsx:114`
**Issue:** The only callers of `GetSessionTailLines` and the styled-tail route, engine renderer and client method were the Hub previews, so with the Hub gone they are unreachable. `App.d.ts:55` keeps an orphan comment for the removed `GetSessionStyledTailLines`. The comments in `hub.go` still mention ChatPanel and the Hub-card preview, `server.go` still says "chat-capable SPA", and the join dialog says the session opens "in your browser" although it now opens in an app tab.
**Fix:** Remove the dead tail paths, or record why they stay, and update the comments and copy.

### IN-04: Chat history files and aliases.json are orphaned on disk for upgraded users

**File:** `internal/daemon/engine.go` (former `chatsDir()` / `NewAliasStore(cfgDir)`)
**Issue:** CHAT-01 removed the chat store and alias store, but nothing deletes the existing `<configDir>/chats/*.jsonl` or `aliases.json`. Those files can hold chat content and tailnet identities.
**Fix:** Add a one-time best-effort `os.RemoveAll(filepath.Join(cfgDir, "chats"))` and remove `aliases.json` at daemon start, or document that they are retained.

### IN-05: `OpenTailscale` finds `open` through PATH

**File:** `app.go:1053`
**Issue:** `exec.Command("open", ...)` resolves through `PATH`. When the app is started from a shell with a modified PATH, a different `open` would run.
**Fix:** Use `exec.Command("/usr/bin/open", "-a", name)`.

### IN-06: Impossible-state guard: `@available(macOS 10.14, *)`

**File:** `appearance_darwin.m:18-19`
**Category:** necessity
**Issue:** The guarded state (running on macOS older than 10.14) cannot occur. The module targets Go 1.26 (`go.mod:3`), whose binaries require a much newer macOS, and the product targets Apple Silicon MacBook Airs only (macOS 11+). The comment's premise ("deployment target is 10.13") describes Wails' own cgo flags, not this package. This rests partly on judgment about the toolchain minimum, so it is Info.
**Fix:** Keep it if it only silences `-Wunguarded-availability`, but correct the comment. Otherwise drop the branch.

### IN-07: Unused dependency and unhandled promise rejections

**File:** `frontend/src/App.tsx:623`, `frontend/src/components/SettingsSheet.tsx:225-230`, `frontend/src/components/NewSessionModal.tsx:25-36`
**Issue:** `createTab`'s dependency list includes `webServerRunning`, which it never reads. `handleCopyURL` and `handleBrowse` have no catch, so a failing `ClipboardSetText` or `OpenDirectoryDialog` produces an unhandled rejection and no user feedback.
**Fix:** Drop the dependency, and add catch handlers that set the existing error state.

### IN-08: Settings shows "Automatic" when the Appearance load fails, and Automatic cannot be re-saved

**File:** `frontend/src/components/SettingsSheet.tsx:190-195, 265-266`
**Issue:** If `GetAppearance` rejects, the control renders "Automatic" even though Dark may be stored, and `selectAppearance` returns early on `next === appearance`, so the user cannot re-save Automatic to clear a stored override.
**Fix:** When loading fails, show an error and render no selection, or drop the early return on load failure.

### IN-09: The danger cue on the Full-access segment is visual only

**File:** `frontend/src/components/Share/ShareSegmentedControl.tsx:76, 108-110`
**Issue:** The old "⚠" prefix was part of the text label. The replacement `ExclamationTriangleIcon` is `aria-hidden`, so screen readers lose the danger signal on the full command-execution tier (the CSS ring is visual too).
**Fix:** Add visually hidden text ("Warning:") or an `aria-describedby` on the danger segment.

### IN-10: Inherited fresh-install plugin settings bug (already logged)

**File:** `internal/daemon/engine.go` `loadSettingsFromDisk`
**Issue:** As already noted elsewhere, plugin settings stay zero-valued when settings.json is missing. Cited for completeness only. The new `appearance` key is not affected, because `GetAppearance` normalizes `""` to `system`.
**Fix:** None in this phase. The post-phase fix is already tracked.

---

_Reviewed: 2026-11-16T00:30:40Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
