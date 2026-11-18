# Phase 2: Tabs + Sidebar Navigation - Pattern Map

**Mapped:** 2026-11-15
**Files analyzed:** 24 (groups of new, modified, moved and deleted files)
**Analogs found:** 22 / 24

All analog paths below were checked with `git ls-files` (tracked source, no mirrors).

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `main.go` (window `options.Mac`, transparent `BackgroundColour`, rebuilt `appMenu()`) | config | event-driven (menu -> Wails event) | `main.go` `appMenu()` lines 94-138 + App.tsx `EventsOn('tray:focus-session')` line 606 | exact (self) |
| `internal/relay/protocol.go` (retire 0x30-0x34, 0x37) | model / wire protocol | request-response (frames) | same file lines 76-82 (0x35/0x36 RETIRED comment, Phase 1 01-09) | exact |
| `internal/relay/server.go` read pump (drop typing/alias/chat-send arms, CORS helpers, identity) | controller | streaming | same file lines 307-365 | exact |
| `internal/webserver/server.go` read pump + `/api/chat` routes + WhoIs/self block | controller | streaming + request-response | same file ~1457-1504 (pump), mirrors relay pump | exact |
| `internal/relay/hub.go` (drop presence/chat, keep `Origin`) | service | pub-sub | self | exact |
| `internal/daemon/engine.go`, `api.go` (chat stores, `stayOnHubAfterCreate`) | service / route | CRUD | Phase 1 01-10/01-11 removals in same files | exact |
| `internal/relay/server_inject_test.go` (retired-frame test 0x30-0x37) | test | streaming | same file lines 108-185 | exact (needs new sentinel, see below) |
| `internal/webserver/inject_test.go` (retired-frame test) | test | streaming | `internal/relay/server_inject_test.go` | exact |
| `internal/daemon/settings_removed_test.go` (+ stayonhub, chat route gone) | test | request-response | same file lines 1-46 | exact |
| `internal/webserver/files_removed_test.go` -> new chat-route 404 case (same file or `chat_removed_test.go`) | test | request-response | `internal/webserver/files_removed_test.go` lines 1-34 | exact |
| `internal/daemon/engine_migration_test.go` (`stayOnHubAfterCreate` dropped on save) | test | file-I/O | same file lines 225-290 | exact |
| `frontend/src/lib/useColorScheme.ts` (NEW) | hook | event-driven | none in repo kept (`Hub/useChatUnreadListeners.ts` is deleted); `EventsOn` effect pattern in App.tsx 602-615 | partial — use RESEARCH Pattern 4 |
| `frontend/src/lib/terminalPalettes.ts` (NEW) | utility (constants) | transform | `frontend/src/lib/tailscale.ts` (single exported constant module) | role-match |
| `frontend/src/test-setup.ts` (matchMedia polyfill) | config/test | — | same file, ResizeObserver polyfill lines 12-22 | exact |
| `frontend/src/App.tsx` (session poll, peer poll, detach close, End Session, rename revert, menu events, theme) | component/shell | request-response + polling | self lines 756-975 | exact (self) |
| `frontend/src/components/Sidebar.tsx` (rewrite: This Mac + peer groups + footer) | component | request-response (props) | `TabBar.tsx` (rows + context menu) + RESEARCH `peerView()` | role-match |
| `frontend/src/components/TabBar.tsx` (pills, +, Share / On-host chip, drop chevron) | component | event-driven | self lines 215-331 | exact (self) |
| `frontend/src/components/ContextMenu.tsx` (NEW, extracted) | component | event-driven | `TabBar.tsx` lines 294-328 (`role="menu"`) | exact (extract) |
| `frontend/src/components/InlineSessionName.tsx` (MOVE from `Hub/`) + test | component | event-driven | `components/Hub/InlineSessionName.tsx` | exact (move) |
| `frontend/src/components/EndSessionDialog` (NEW, or reuse) | component (modal) | request-response | `components/RegenerateKeyModal.tsx` (already generalized via copy props) | exact — prefer reuse |
| `frontend/src/components/SettingsSheet.tsx` (re-host SettingsTab) | component (modal) | CRUD | `SettingsTab.tsx` content + `RegenerateKeyModal.tsx` dialog shell | role-match |
| `frontend/src/components/TailscaleBanner.tsx` (icon, `id="ts-off-reason"`, Open Tailscale) | component | — | self (15 lines) | exact (self) |
| `frontend/src/components/Share/*` (MOVE SessionShareModal, FunnelRiskPanel, ShareSegmentedControl), `RemoteSessionView.tsx` (MOVE WebShareSessionView, strip chat) | component | request-response | the files themselves | exact (move) |
| `frontend/src/__tests__/style.tokens.test.ts` (NEW source gate, replaces themeTokens/style.* gates) | test | file-I/O (source read) | `frontend/src/__tests__/themeTokens.test.ts` lines 1-90 | exact |
| `frontend/src/style.css` token blocks | config (styles) | — | current `--hub-*` `:root` / `[data-ui-theme="light"]` pair | role-match |

Deletions (no analog needed, follow Chesterton's Fence notes in RESEARCH "Hub Removal Blast Radius" and "CHAT-01 Full Inventory"): `components/Hub/*` remainder, `WelcomeTab`, `HelpTab` + help components + `content/help/*`, `StatusBar.tsx`, `themes.ts`, `lib/hubGroups*.ts`, `lib/hubGroupCounts*.ts`, `internal/daemon/chat*.go`, `alias_store*.go`, `internal/relay/sanitize*.go`, `internal/webserver/chat*.go`, chat/presence relay tests, `engine_stayonhub_test.go`, `api_stayonhub_test.go`. Every deleted test file updates TESTING.md Sections 2/4.

## Pattern Assignments

### `internal/relay/protocol.go` (wire protocol, retire frames)

**Analog:** same file lines 76-82 — the Phase 1 retirement comment. Extend it to cover 0x30-0x37 and delete the const block (lines 83-90) plus payload types:
```go
// 0x35 and 0x36 are RETIRED (Portty Phase 1, CHAT-01): they were the chat→PTY
// inject frame and its NAK. Never reuse them. The read pumps have no default
// arm, so an old client's 0x35 frame is dropped without writing to the PTY.
```
Target: one comment "0x30-0x37 are RETIRED (chat, presence, typing, alias, inject, self). Never reuse." Keep 0x01-0x12 and `MsgMeta` 0x20.

### `internal/relay/server.go` / `internal/webserver/server.go` read pumps

**Analog:** `internal/relay/server.go` lines 307-322 (the arms that stay):
```go
switch msgType {
case MsgInput:
    if !sub.ReadOnly { // MC-03: discard input for read-only clients
        filtered := absorber.Filter(payload)
        if len(filtered) > 0 {
            _ = hub.WriteInput(filtered)
        }
    }
case MsgResize2:
    ...
    _ = hub.ResizeClient(sub, int(cols), int(rows)) // host-authority arbiter (VIEW-01/VIEW-02)
case MsgPing:
    // Keep-alive — no-op.
}
```
Delete `case MsgTyping` (323-330), `case MsgAliasSet` (331-347), `case MsgChatSend` (348-365). Do NOT add a `default:` arm. Webserver pump at `internal/webserver/server.go:1457-1504` gets the identical treatment. Keep `Subscriber.Origin` (read by the resize arbiter in hub.go).

### `internal/relay/server_inject_test.go` and `internal/webserver/inject_test.go`

**Analog:** `internal/relay/server_inject_test.go`.
- Harness `setupInjectTestServer` (lines 31-70): counting PTY writer via `writerFunc` (lines 17-20). Delete the `hub.SetChatAppendFn(...)` block (lines 47-60).
- Raw retired byte constant pattern (lines 107-110):
```go
const retiredInjectFrameType byte = 0x35
```
Generalize to a slice `[]byte{0x30,0x31,0x32,0x33,0x34,0x35,0x36,0x37}`.
- **Sentinel must change.** Today `assertInjectFrameDropped` (lines 118-150) proves the socket survived by sending `MsgChatSend` and waiting for a `MsgChat` broadcast. That path no longer exists. Per RESEARCH Pattern 7: after the retired frames send a `MsgInput` frame and assert the PTY write count becomes exactly 1 (use a short poll loop instead of `time.Sleep`). Use a read-write (no `readonly=1`) connection for the positive sentinel; the RO test variant should instead assert 0 writes after a `MsgInput` (MC-03) plus that `conn.Write` still succeeds.
- Keep `TestInjectFrame_*` names or rename to `TestRetiredFrames_*`; if renamed, update TESTING.md Section 4.

### `internal/daemon/settings_removed_test.go` (route-gone tests)

**Analog:** same file lines 9-22:
```go
// Portty 01-10 (CONTEXT Amendment 2, T-01-35): ...
func TestSettingsRemoved_AutoCloseSessionRoutes(t *testing.T) {
	h := newRemovedRoutesAPI(t).Handler()
	assertRouteGone(t, h, http.MethodGet, "/settings/auto-close-session")
	assertRouteGone(t, h, http.MethodPatch, "/settings/auto-close-session")
	// kept sibling route still answers 200
```
Helpers `newRemovedRoutesAPI` and `assertRouteGone` live in `internal/daemon/files_removed_test.go:15-35`. Add `TestSettingsRemoved_StayOnHubAfterCreateRoutes` with the two paths from api.go:162-163, and a chat-routes case for any daemon chat routes from `chat_routes.go` before it is deleted (read the route strings first).

### Webserver chat route 404 test

**Analog:** `internal/webserver/files_removed_test.go` lines 12-34: `testServer(t)`, `ws.SetSigningKey(capTestKey)`, `ws.EnableSession(...)`, `issueCapFor(t, ws, id, "read,write")`, loop over `{method,path}` asserting 404 or 405. Paths: `/api/chat/{id}/history`, `/api/chat/{id}/export`. A valid cap proves the route is gone, not the gate.

### `internal/daemon/engine_migration_test.go`

**Analog:** `TestSettingsMigration_LegacyAgentHubV42` lines 230-290. Currently asserts `stayOnHubAfterCreate` is KEPT (lines 249-251, 283-285) and calls `e.GetStayOnHubAfterCreate()`. Invert: remove the getter call, add `"stayOnHubAfterCreate"` to the dropped-keys loop:
```go
for _, k := range []string{"cliPaths", "shellPath", "notifyOnWaiting"} {
    if _, ok := onDisk[k]; ok {
        t.Errorf("saved settings.json still contains %s; the removed key should be dropped on save", k)
    }
}
```
Keep the `shellWebShareWarned` and `plugins` survival checks. Fixture `tests/fixtures/settings_agenthub_v4.2.json` already contains the key. Note RESEARCH finding: there is no settings.json `theme` key; theme lives in localStorage, so no Go test for it.

### `main.go` (window chrome + menu)

**Analog:** current `appMenu()` lines 94-114 and callback style lines 116-130:
```go
func openGitHubCallback(_ *menu.CallbackData) {
	if appCtx != nil {
		runtime.BrowserOpenURL(appCtx, "https://github.com/scottkw/agenthub")
	}
}
```
New callbacks follow the same `appCtx != nil` guard and emit `runtime.EventsEmit(appCtx, "menu:new-session")` etc. The comment at line 96 ("AppMenu MUST be first") is the Chesterton fence: replacing `menu.AppMenu()` with a custom first submenu keeps "first", per RESEARCH Pattern 3. File items at 104-106 currently have nil callbacks (dead) and ⌘N; switch to ⌘T. Window options: RESEARCH Pattern 1 block (`mac.TitleBarHiddenInset()` gated by the spike, fallback `mac.TitleBarHidden()`).

### `frontend/src/App.tsx`

**Frontend menu-event analog** (lines 606-614):
```ts
const cancelTrayFocus = EventsOn('tray:focus-session', (sessionId: string) => {
  setTabs(prev => {
    const tab = prev.find(t => t.sessionId === sessionId)
    if (tab) { setActiveId(tab.id) }
    return prev
  })
})
```
Add `menu:new-session`, `menu:close-tab`, `menu:settings`, `menu:toggle-sidebar` the same way; read `activeId` via a ref (existing `removeTabStateRef` pattern, line 774).

**Session poll analog** (lines 867-883): keep the `cancelled` flag + `setInterval(3000)` shape, delete `if (activeId !== HUB_TAB.id) return`, rename `hubSessions` -> `sessions`, and delete the CR-02 modal poll (893-915) since the always-on poll covers it.

**Peer poll analog** (lines 944-972): keep the `tailscaleOff -> setRemotePeers([])` branch, delete line 952 gate, turn `remoteHasLoadedRef` (line 178) into state so the sidebar "Looking for other Macs…" line clears.

**Close tab** (`removeTabState` 758-773, `handleCloseTab` 776-787): new close = `removeTabState` only, neighbor rule changed from `idx - 1` to right-else-left; stop deleting `webEnabled[id]`. Move the `ToggleWebServing(false)` + `KillSession` body into the End Session confirm's `onConfirm`.

**Rename** (789-798): currently applies the name even on RPC failure. Change to optimistic update of `sessions`, then restore old name in `catch`. Tab labels derive from `sessions`.

### `frontend/src/components/ContextMenu.tsx` (extract) + `TabBar.tsx` / `Sidebar.tsx`

**Analog:** `TabBar.tsx` lines 294-328:
```tsx
<div className="tab__context-menu" role="menu"
  style={{ position: 'fixed', top: contextMenu.y, left: contextMenu.x }}
  onMouseDown={(e) => e.stopPropagation()}>
  {!webMode && (
  <button role="menuitem" className="tab__context-menu__item"
    onClick={() => { startEditById(contextMenu.tabId); setContextMenu(null) }}>
    Rename
  </button>)}
```
Trigger pattern (lines 222-227): `onContextMenu={webMode ? undefined : (e) => { e.preventDefault(); e.stopPropagation(); setContextMenu({ tabId, x: e.clientX, y: e.clientY }) }}`. Extract into `ContextMenu` with `items` prop (2 consumers: tab, sidebar row). Add ↑/↓/Enter/Esc and focus return (UI-SPEC Context menus). Inline `position` style is allowed (not a color). Delete the `tab__chevron` button (lines ~229-240) and the `&#8250;` scroll glyphs (use `ChevronLeftIcon/RightIcon`). Keep every `webMode` gate (Pitfall 11).

### `frontend/src/components/InlineSessionName.tsx` (move)

**Analog:** `components/Hub/InlineSessionName.tsx` lines 27-80. Behavior already matches UI-SPEC (Enter/blur commit, Esc cancel, trim, empty/unchanged revert, `select()` on edit, fires `onRenamed` only — App owns the RPC per CR-02). Changes: `git mv` file + test, add `aria-label="Session name"` (currently `placeholder` only), start edit on double-click and from an external `editing` trigger (menu "Rename"), update doc comment that references HubPanel. TabBar's own duplicate rename input (lines ~205-221 `tab__rename-input`) should be replaced by this component.

### End Session confirm

**Analog:** `components/RegenerateKeyModal.tsx` — already generalized with `title/body/confirmLabel/actingLabel/cancelLabel/titleId` props (lines 3-19), focuses Cancel on open (lines 55-58), Esc closes (45-53), shows `onConfirm` errors inline (66-77). It is reused for the disable-share-warning confirm. Recommend reusing it for End Session (third consumer) with copy from UI-SPEC Copywriting, rather than a new file; the web-shared extra line needs `body` to accept `ReactNode` or a second line prop. Restyle: drop the `&times;` header close (line 96), scrim click must do nothing (line 86 currently calls `onCancel`), destructive button class.

### `frontend/src/components/SettingsSheet.tsx`

**Content analog:** `SettingsTab.tsx` (keep its load/save `*Loaded` gates and PluginsSection). **Shell analog:** `RegenerateKeyModal.tsx` dialog markup (`role="dialog" aria-modal aria-labelledby`, Esc listener). Remove jump bar, search, theme switch/select, stay-on-hub toggle, `#9ece6a` inline color (SettingsTab.tsx:498).

### `frontend/src/components/TailscaleBanner.tsx`

**Analog:** self:
```tsx
<div className="tailscale-banner" role="status">
  <span className="tailscale-banner__icon" aria-hidden="true">{'⚠'}</span>
  <span className="tailscale-banner__message">{TAILSCALE_OFF_REASON}</span>
</div>
```
Add `id="ts-off-reason"`, swap glyph for `ExclamationTriangleIcon`, add "Open Tailscale" action. Keep `TAILSCALE_OFF_REASON` from `lib/tailscale.ts` line 3 as the only copy. Update `components/__tests__/TailscaleBanner.test.tsx`.

### `frontend/src/lib/terminalPalettes.ts` (new)

**Analog:** `frontend/src/lib/tailscale.ts` — a tiny module of exported module-level constants with a one-line provenance comment. Export `LIGHT_PALETTE` and `DARK_PALETTE` typed as xterm `ITheme`, values verbatim from UI-SPEC "Terminal palettes". Module-level constants are required (stable identity; see TerminalPanel theme effect below). Note: user is colorblind (themeTokens.test.ts header) — palette correctness must be asserted in source tests, not by eye.

### `frontend/src/lib/useColorScheme.ts` (new)

**No kept analog.** Use RESEARCH Pattern 4 code verbatim. Consumer contract: `TerminalPanel.tsx` lines 822-833 already applies a changed `theme` prop live and stashes it in `pendingThemeRef` while hidden:
```ts
useEffect(() => {
  if (!termRef.current) return
  if (!isActive) { pendingThemeRef.current = theme; return }
  termRef.current.options.theme = theme
  termRef.current.clearTextureAtlas()
  fitTerminal(termRef.current)
}, [theme, isActive])
```
Do not touch this effect. Replace TerminalPanel.tsx:962 inline `'#1a1b26'` with a token class.

### `frontend/src/test-setup.ts` (matchMedia polyfill)

**Analog:** same file lines 15-22:
```ts
if (typeof globalThis.ResizeObserver === 'undefined') {
  globalThis.ResizeObserver = class ResizeObserver { observe() {} unobserve() {} disconnect() {} }
}
```
Add the same guarded shape for `window.matchMedia` returning `{ matches: false, media, addEventListener() {}, removeEventListener() {}, onchange: null, addListener() {}, removeListener() {}, dispatchEvent: () => false }`. The IntersectionObserver polyfill (lines 28-34) only serves HelpSectionNav; remove it only after confirming no other consumer (Chesterton).

### `frontend/src/__tests__/style.tokens.test.ts` (new LOOK-01 gate)

**Analog:** `frontend/src/__tests__/themeTokens.test.ts` lines 28-90 — reuse its path resolution, `HEX_RE`, `stripBlockComments`, and brace-balanced `extractBlockBody`:
```ts
const __dirname = dirname(fileURLToPath(import.meta.url))
const STYLE_PATH = resolve(__dirname, '..', 'style.css')
const rawCss = readFileSync(STYLE_PATH, 'utf8')
const HEX_RE = /#[0-9a-fA-F]{3}(?:[0-9a-fA-F]{3})?\b/g
```
Its existing parity check (`:root` vs `[data-ui-theme="light"]`) becomes `:root` vs `@media (prefers-color-scheme: dark) { :root }`. Assertions per RESEARCH Pitfall 7 (a)-(e) plus the TSX inline-color gate. Then delete `themeTokens.test.ts` and the obsolete `style.*.test.ts` gates, updating TESTING.md.

## Shared Patterns

### Retired wire frames (no case, no default arm)
**Source:** `internal/relay/protocol.go:76-82`, `internal/relay/server.go:307-365`
**Apply to:** both read pumps, frontend `relayClient.ts` (`parseServerFrame` already returns `{type:'unknown'}` — keep it).

### Removed-route proof (404/405 with a valid credential, plus a kept sibling 200)
**Source:** `internal/daemon/files_removed_test.go:15-42`, `internal/daemon/settings_removed_test.go:11-22`, `internal/webserver/files_removed_test.go:12-34`
**Apply to:** stay-on-hub routes, daemon/webserver chat routes, `GetSessionStyledTailLines` route if deleted.

### Legacy settings key dropped on save
**Source:** `internal/daemon/engine_migration_test.go:230-290` (remove struct field; `json.Unmarshal` ignores the key; next save omits it)
**Apply to:** `stayOnHubAfterCreate`.

### Polling effect shape
**Source:** `App.tsx:867-883` (`cancelled` flag, immediate `refresh()`, `setInterval`, cleanup) — mode `'web'` early return first.
**Apply to:** sessions poll (3 s) and peer poll (30 s).

### Modal shell (dialog a11y, Esc, initial focus, inline error)
**Source:** `components/RegenerateKeyModal.tsx:45-123`; also `QuitConfirmModal.tsx`, `RemoteJoinCodeModal.tsx` (aria-modal at :117)
**Apply to:** End Session, Settings sheet, Share dialog restyle, New Session. Common restyle: remove header `&times;`, scrim click no-op, focus trap (none of the existing modals trap focus beyond initial focus — verify before claiming reuse).

### Web guest gates
**Source:** `TabBar.tsx` `webMode ? undefined : ...` (lines 222-227, 300-325)
**Apply to:** Sidebar (unrendered in web), context menus, rename, End Session, Settings, menu events.

### Hand-edited Wails bindings
**Source:** `frontend/src/wailsjs/go/main/App.d.ts`, `App.js`, both `models.ts`; vitest runtime stub `frontend/src/wailsjs/runtime/runtime.js`
**Apply to:** every Go binding add/remove (RESEARCH Pitfall 5).

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| `frontend/src/lib/useColorScheme.ts` | hook | event-driven | No surviving custom hook or matchMedia use; use RESEARCH Pattern 4 |
| Native window chrome spike (`options.Mac`) | config | — | No `options.Mac` block exists yet; use RESEARCH Pattern 1/2 and Pitfall 1 fallback |

## Metadata

**Analog search scope:** `main.go`, `internal/relay`, `internal/webserver`, `internal/daemon`, `frontend/src/{App.tsx,components,lib,__tests__,test-setup.ts}`
**Files scanned:** ~20
**Pattern extraction date:** 2026-11-15
