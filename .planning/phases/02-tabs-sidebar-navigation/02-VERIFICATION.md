---
phase: 02-tabs-sidebar-navigation
verified: 2026-11-16T01:25:33Z
status: human_needed
score: 7/8 must-haves verified
covered_files: [".planning/phases/02-tabs-sidebar-navigation/02-01-PLAN.md",".planning/phases/02-tabs-sidebar-navigation/02-01-SUMMARY.md",".planning/phases/02-tabs-sidebar-navigation/02-02-PLAN.md",".planning/phases/02-tabs-sidebar-navigation/02-02-SUMMARY.md",".planning/phases/02-tabs-sidebar-navigation/02-03-PLAN.md",".planning/phases/02-tabs-sidebar-navigation/02-03-SUMMARY.md",".planning/phases/02-tabs-sidebar-navigation/02-04-PLAN.md",".planning/phases/02-tabs-sidebar-navigation/02-04-SUMMARY.md",".planning/phases/02-tabs-sidebar-navigation/02-05-PLAN.md",".planning/phases/02-tabs-sidebar-navigation/02-05-SUMMARY.md",".planning/phases/02-tabs-sidebar-navigation/02-06-PLAN.md",".planning/phases/02-tabs-sidebar-navigation/02-06-SUMMARY.md",".planning/phases/02-tabs-sidebar-navigation/02-07-PLAN.md",".planning/phases/02-tabs-sidebar-navigation/02-07-SUMMARY.md",".planning/phases/02-tabs-sidebar-navigation/02-08-PLAN.md",".planning/phases/02-tabs-sidebar-navigation/02-08-SUMMARY.md",".planning/phases/02-tabs-sidebar-navigation/02-09-PLAN.md",".planning/phases/02-tabs-sidebar-navigation/02-09-SUMMARY.md",".planning/phases/02-tabs-sidebar-navigation/02-10-PLAN.md",".planning/phases/02-tabs-sidebar-navigation/02-10-SUMMARY.md",".planning/phases/02-tabs-sidebar-navigation/02-11-PLAN.md",".planning/phases/02-tabs-sidebar-navigation/02-11-SUMMARY.md",".planning/phases/02-tabs-sidebar-navigation/02-12-PLAN.md",".planning/phases/02-tabs-sidebar-navigation/02-12-SUMMARY.md",".planning/phases/02-tabs-sidebar-navigation/02-13-PLAN.md",".planning/phases/02-tabs-sidebar-navigation/02-13-SUMMARY.md","TESTING.md","app.go","app_test.go","appearance_darwin.go","appearance_darwin.m","cmd/playwright-fixture/main.go","frontend/e2e/web-color-scheme.spec.ts","frontend/e2e/web-plugin-hot-swap.spec.ts","frontend/e2e/web-share-scope.spec.ts","frontend/index.html","frontend/package.json","frontend/pnpm-lock.yaml","frontend/src/App.tsx","frontend/src/__tests__/App.saver.test.tsx","frontend/src/__tests__/style.tokens.test.ts","frontend/src/components/ContextMenu.tsx","frontend/src/components/EmptyState.tsx","frontend/src/components/FindBar/__tests__/FindBar.themeMatrix.test.tsx","frontend/src/components/FindBar/__tests__/FindBar.visual.test.tsx","frontend/src/components/InlineSessionName.tsx","frontend/src/components/NewSessionModal.tsx","frontend/src/components/PluginToggleBanner.tsx","frontend/src/components/PluginsSection.tsx","frontend/src/components/QuitConfirmModal.tsx","frontend/src/components/RegenerateKeyModal.tsx","frontend/src/components/RemoteJoinCodeModal.test.tsx","frontend/src/components/RemoteJoinCodeModal.tsx","frontend/src/components/SessionEndedBanner.tsx","frontend/src/components/SessionShare/InternetFullAccessTab.tsx","frontend/src/components/SessionShare/shared.tsx","frontend/src/components/SettingsSheet.tsx","frontend/src/components/Share/FunnelRiskPanel.tsx","frontend/src/components/Share/SessionShareModal.tsx","frontend/src/components/Share/ShareSegmentedControl.tsx","frontend/src/components/Sidebar.tsx","frontend/src/components/Switch.tsx","frontend/src/components/TabBar.tsx","frontend/src/components/TailscaleBanner.tsx","frontend/src/components/TerminalPanel.tsx","frontend/src/components/UpdateBanner.tsx","frontend/src/components/WebGLRecoveryBanner.tsx","frontend/src/components/WebShareSessionView.tsx","frontend/src/components/__tests__/App.createTab.sessionsSeed.test.tsx","frontend/src/components/__tests__/App.dead-modal.test.tsx","frontend/src/components/__tests__/App.nav.test.tsx","frontend/src/components/__tests__/App.open-remote.test.tsx","frontend/src/components/__tests__/App.shellExit.test.tsx","frontend/src/components/__tests__/App.shellWebShare.test.tsx","frontend/src/components/__tests__/App.tailscale.test.tsx","frontend/src/components/__tests__/App.test.tsx","frontend/src/components/__tests__/ContextMenu.test.tsx","frontend/src/components/__tests__/EmptyState.test.tsx","frontend/src/components/__tests__/FunnelRiskPanel.test.tsx","frontend/src/components/__tests__/InlineSessionName.test.tsx","frontend/src/components/__tests__/NewSessionModal.test.tsx","frontend/src/components/__tests__/PluginToggleBanner.test.tsx","frontend/src/components/__tests__/PluginsSection.disclosure.render.test.tsx","frontend/src/components/__tests__/PluginsSection.test.tsx","frontend/src/components/__tests__/QuitConfirmModal.test.tsx","frontend/src/components/__tests__/RegenerateKeyModal.test.tsx","frontend/src/components/__tests__/RemoteJoinCodeModal.test.tsx","frontend/src/components/__tests__/SessionEndedBanner.test.tsx","frontend/src/components/__tests__/SessionShareModal.disconnect.test.tsx","frontend/src/components/__tests__/SessionShareModal.test.tsx","frontend/src/components/__tests__/SettingsSheet.persistence.test.tsx","frontend/src/components/__tests__/SettingsSheet.shell-warn-toggle.test.tsx","frontend/src/components/__tests__/SettingsSheet.start-minimized.test.tsx","frontend/src/components/__tests__/SettingsSheet.tailscale-status.test.tsx","frontend/src/components/__tests__/SettingsSheet.test.tsx","frontend/src/components/__tests__/SettingsSheet.web-link-ux.test.tsx","frontend/src/components/__tests__/ShareSegmentedControl.test.tsx","frontend/src/components/__tests__/Sidebar.test.tsx","frontend/src/components/__tests__/Switch.test.tsx","frontend/src/components/__tests__/TabBar.test.tsx","frontend/src/components/__tests__/TailscaleBanner.test.tsx","frontend/src/components/__tests__/TerminalPanel.scale.test.tsx","frontend/src/components/__tests__/TerminalPanel.test.tsx","frontend/src/components/__tests__/UpdateBanner.test.tsx","frontend/src/components/__tests__/WebGLRecoveryBanner.test.tsx","frontend/src/components/__tests__/WebShareSessionView.plugin-config.test.tsx","frontend/src/components/__tests__/WebShareSessionView.test.tsx","frontend/src/components/__tests__/style.contrast.test.ts","frontend/src/components/__tests__/style.hub.modal.test.ts","frontend/src/components/__tests__/style.redesign.test.ts","frontend/src/components/__tests__/style.settings.test.ts","frontend/src/lib/__tests__/sidebarPeers.test.ts","frontend/src/lib/__tests__/tabs.test.ts","frontend/src/lib/__tests__/terminalPalettes.test.ts","frontend/src/lib/__tests__/useColorScheme.test.tsx","frontend/src/lib/__tests__/useDialogFocus.test.tsx","frontend/src/lib/helpUrl.ts","frontend/src/lib/relayClient.test.ts","frontend/src/lib/relayClient.ts","frontend/src/lib/sidebarPeers.ts","frontend/src/lib/tabs.ts","frontend/src/lib/terminalPalettes.ts","frontend/src/lib/useColorScheme.ts","frontend/src/lib/useDialogFocus.ts","frontend/src/main.tsx","frontend/src/style.css","frontend/src/test-setup.ts","frontend/src/vite-env.d.ts","frontend/src/wailsjs/go/main/App.d.ts","frontend/src/wailsjs/go/main/App.js","frontend/src/wailsjs/go/models.ts","internal/daemon/api.go","internal/daemon/appearance_settings_test.go","internal/daemon/chat_removed_test.go","internal/daemon/client.go","internal/daemon/engine.go","internal/daemon/engine_migration_test.go","internal/daemon/engine_test.go","internal/daemon/plugin_settings.go","internal/daemon/settings_removed_test.go","internal/relay/fuzz_test.go","internal/relay/hub.go","internal/relay/protocol.go","internal/relay/server.go","internal/relay/server_inject_test.go","internal/relay/server_test.go","internal/webserver/assets_test.go","internal/webserver/chat_removed_test.go","internal/webserver/fuzz_test.go","internal/webserver/inject_test.go","internal/webserver/server.go","internal/webserver/server_test.go","internal/webserver/session_ended_test.go","main.go","menu_test.go","web/assets/dashboard.css","web/assets/join.css"]
covered_digest: "v4:sha256:6b9970f64fce517cb07fac804c61f8c0f0c17c91dc27b3f3780e75e79bfab923"
behavior_unverified: 0
overrides_applied: 0
human_verification:
  - test: "Second Mac on the tailnet (M-03, M-04, M-05, M-13): share a session on the other Mac, then click its row under that Mac's group in the sidebar. Do the first open with a join code, then close the tab and reopen it (cached capability). Also check the peer states: the other Mac asleep, sharing nothing, and Tailscale off. Then open two peer sessions"
    expected: "Peer groups are labelled with the short hostname and sorted A to Z. The first click opens the Join Remote Session dialog, and a successful join opens a '<host> › <session>' tab that you can type in. The reopen skips the dialog. An asleep Mac shows \"Can't reach this Mac…\", a Mac with nothing shared is hidden, and with Tailscale off you see \"Peers appear when Tailscale is connected.\" The two peer tabs stay isolated and show an 'On <host>' chip instead of Share"
    why_human: "Needs a second Mac and a live tailnet. Unit tests mock GetRemoteSessionsWithMeta, ExchangeJoinCodeAtURL, RegisterRemoteCap and OpenRemoteSessionURL. The VM has no peer"
  - test: "With Tailscale connected (M-01, M-57): click Share in the toolbar on a local session tab. Open the Funnel/Internet segment and go through the risk panel's consent gate. Then disconnect and reconnect Tailscale"
    expected: "A 560px sheet titled 'Share “<name>”' opens under the toolbar with a Done footer and no header ×. The Funnel risk panel and its consent gate can be reached. When Tailscale disconnects, the dialog closes, Share dims (aria-disabled), the NET-01 banner appears with Open Tailscale, and Settings ▸ Sharing shows the reason inline. Reconnecting clears all of this without a restart"
    why_human: "The VM had no Tailscale, so the toolbar Share button was always aria-disabled and the dialog could not open natively. The wiring is proven by TabBar.test.tsx and SessionShareModal tests only"
  - test: "With a session web-shared over the tailnet, set the desktop Settings ▸ Appearance to Dark, then open the share link in a browser set to Light (and the reverse)"
    expected: "The web share page follows the browser's light/dark setting and ignores the desktop override, and it switches live when the browser setting changes"
    why_human: "Web serving needs Tailscale. The Playwright web-color-scheme spec proves the page follows emulated prefers-color-scheme, but nothing has exercised the override's independence end to end"
  - test: "VoiceOver (M-63, M-66 step 4): with Tailscale off, focus the toolbar Share button and Settings ▸ Sharing ▸ Start Server. Move through the sidebar rows and the tabs. Open a sheet"
    expected: "Share is read as 'Share, dimmed/unavailable', followed by the NET-01 sentence (aria-describedby=ts-off-reason). Start Server is read with its inline reason. The active row is announced as current page. Sheets announce their title and keep focus inside"
    why_human: "Screen reader output cannot be checked with grep or jsdom"
  - test: "5-second host glance on real Apple Silicon hardware (not the VM), in Light and in Dark: look at the sidebar, then click another app so Portty is not key. Look at a single open tab and a very long session name"
    expected: "The sidebar shows native vibrancy (desktop tint shows through) and its text is readable. The selected row turns gray when the window is not key. A single tab is a left-aligned pill no wider than 220px. Long names truncate with an ellipsis and never overlap the × or the next pill"
    why_human: "VM vibrancy is reduced and was only seen in Dark. These are visual, native-only checks (02-04 and 02-08 backstop truths)"
---

# Phase 2: Tabs + Sidebar Navigation Verification Report

**Phase Goal:** Users navigate everything from tabs plus a sidebar. The Hub grid is gone, and the sidebar lists local sessions and tailnet peers' sessions, each opening in a tab. In scope too: the macOS-native redesign (LOOK-01), system light/dark with an Automatic/Light/Dark override (THEM-01, amended 2026-11-15) and chat removal (CHAT-01).
**Verified:** 2026-11-16T01:25:33Z
**Status:** human_needed
**Re-verification:** No. This is the initial verification.

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria; PLAN must-haves merged under each)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | The app launches straight into tabs plus a sidebar, with no Hub grid view or Hub entry anywhere (NAV-03) | ✓ VERIFIED | `frontend/src/components/Hub/` is gone. `git grep` finds no Hub, Welcome or HelpTab code in `frontend/src`. `App.tsx:1074-1116` renders `<Sidebar>` + `<TabBar>`. `Tab['type']` has no hub/welcome/help. The menu (`main.go:139-169`) and the tray (`tray_common.go:41`) have no Hub item. Stay-on-Hub is removed end to end (`settings_removed_test.go`). VM: the app launches into the sidebar and tabs |
| 2 | The sidebar lists every local session; clicking one focuses its tab if open, or opens it in a new tab (NAV-01) | ✓ VERIFIED | Always-on 3 s `ListSessions` poll (`App.tsx:743-758`) → `sessions={sessions}` → `Sidebar.tsx:83-127` rows keyed by id. `handleOpenSessionTab` (`App.tsx:901-915`) focuses or appends. `Sidebar.test.tsx` and `App.nav.test.tsx` pass (run by me). VM: CLI-created sessions appeared within 3 s |
| 3 | The sidebar lists sessions shared by other machines, grouped by machine; clicking one attaches in a tab (NAV-02) | ? UNCERTAIN | The code is present and wired. The peer poll runs every 30 s, gated only on Tailscale (`App.tsx:779-806`). `peerView()` groups and sorts by short hostname (`sidebarPeers.ts`). A row click goes to `handleOpenRemoteSession` → cached-cap `OpenRemoteSessionURL` or join-code `ExchangeJoinCodeAtURL` + `RegisterRemoteCap` → a `'<host> › <session>'` tab (`App.tsx:939-1006`). `sidebarPeers.test.ts` and `App.open-remote.test.tsx` pass, but they use mocks. A real cross-Mac attach has not been observed (needs a second Mac): see Human Verification 1 |
| 4 | User can run several sessions as tabs, rename a session (the name shows in tab and sidebar) and close it (TERM-03) | ✓ VERIFIED | `closeTab` = UI-only `removeTabState` with no RPC (`App.tsx:628-644`). `menu:close-tab` uses `activeIdRef`. The End Session confirm → `ToggleWebServing(false)` if shared → `KillSession` (`App.tsx:649-657, 1342-1368`). Tab labels come from the session list (`tabLabel`), so a rename updates both places in one render (`App.tsx:662-671, 1089`). `TabBar`, `tabs`, `App.test` pass. VM: detach-only close, End Session confirm, rename in the tab, the sidebar and the daemon, and ⌘W down to the empty state |
| 5 | The Share dialog (with the Funnel risk panel) is reachable from any session tab | ✓ VERIFIED | Toolbar Share (`TabBar.tsx:316-322`) → `openShareModalForActiveSession` → the single App-level `<SessionShareModal>` (`App.tsx:685-688, 1327-1340`), which renders `FunnelRiskPanel` (`Share/SessionShareModal.tsx`). `TabBar.test.tsx:510` asserts that the click calls onShare. The native open with Tailscale on is still to be confirmed (Human Verification 2) |
| 6 | Chat is gone from the desktop app, the web share page, the daemon/webserver API and the relay protocol; old chat frames crash nothing (CHAT-01) | ✓ VERIFIED | No chat store, routes or UI remain; only historical comments are left (IN-03 deferred). The read pumps have only Input/Resize2/Ping arms (`relay/server.go:153-171`, `webserver/server.go:1327-1340`). I ran `TestRetiredFrames_{RW,RO,NoServerFrames,Concurrent}_RelayPath`, `TestRetiredFrames_{WebPath,NoServerFrames_WebPath}` and `TestChatRemoved_{RelayLoopbackMux,WebServerMux}`, and all pass. `relayClient.ts` parses 0x30-0x37 as unknown (test passes). Five chat-only npm dependencies are removed |
| 7 | macOS Light/Dark switches the app and every open terminal live; no theme picker (Automatic/Light/Dark override per the amendment); the web share page follows the browser (THEM-01) | ✓ VERIFIED | `useColorScheme(override)` (`lib/useColorScheme.ts`) → `terminalTheme` → `theme={terminalTheme}` on every `TerminalPanel` and `WebShareSessionView` (`App.tsx:221, 1222, 1307`). Web mode forces `'system'`. `themes.ts`, `xterm-theme` and the bundled fonts are gone. The override path is wired: SettingsSheet radiogroup → `SetAppearance` → daemon `PATCH /settings/appearance` (validated, 400 on a bad value) → `setNativeAppearance` → `NSApp.appearance` (`appearance_darwin.m`). Startup applies the stored value (`app.go:252`), and so does Retry (`app.go:288`). Go tests `TestAppearanceSettings_*` and `TestSetAppearance_*` pass. Playwright `web-color-scheme.spec.ts` passes (2 tests, run by me). VM: live light/dark, all three override modes, and no flash on relaunch |
| 8 | Native macOS look: system font and accent, source-list sidebar, unified title bar/toolbar, native-style controls, no custom dark chrome or hardcoded colours (LOOK-01) | ✓ VERIFIED | `macWindowOptions()`: `TitleBarHiddenInset`, translucent window, transparent webview (`main.go:79-105`, `TestMacWindowOptions` passes). The `style.tokens.test.ts` source gate passes (no literal colour outside the two token blocks, no inline TSX colours, no `::-webkit-scrollbar`, a global focus ring, no `@font-face`). Every dialog is a sheet via `useDialogFocus`. `Switch` is `role=switch`. The banners share one style. The join/dashboard pages are light/dark (`TestAssets_WebPagesFollowColorScheme` passes). VM: title-bar clicks and drag, sheets in Light and Dark, the Settings panes, vibrancy visible in Dark. The host vibrancy glance is still open (Human Verification 5) |

**Score:** 7/8 truths verified (0 present-but-behavior-unverified; 1 uncertain, which needs a second Mac)

PLAN backstop truths: the 02-02 concurrency truth is verified by `TestRetiredFrames_Concurrent_RelayPath`, which passes. The 02-03, 02-12 and 02-13 native truths are verified by the orchestrator's VM checks of live light/dark, the override modes and no relaunch flash. The 02-04 (vibrancy, inactive gray) and 02-08 (single pill ≤220px, truncation) backstops go to Human Verification 5. The CSS (`.tab { min-width: 88px; max-width: 220px }`, ellipsis rules) is present.

### Prohibitions (judgment tier)

| Prohibition | Disposition | Evidence |
|-------------|-------------|----------|
| 02-02: MUST NOT relax WebSocket input gating | Holds | `if !sub.ReadOnly` is kept on both pumps. The diff touches no origin or capability wiring (`GET /sessions/{id}/ws` is still capability-gated). `TestRetiredFrames_RO_RelayPath` passes |
| 02-03 / 02-13: MUST NOT bring back the theme library or picker | Holds | `themes.ts`, `xterm-theme` and the theme key are gone. The only control is Automatic/Light/Dark. Web forces `'system'` |
| 02-04 / 02-07: MUST NOT render the sidebar or navigation in web mode | Holds | `mode !== 'web' && …` gates on Sidebar and SettingsSheet. `web-share-scope.spec.ts` passes (3 tests, run by me) |
| 02-04: MUST NOT open a peer without the capability handshake | Holds | Peer rows only reach `OpenRemoteSessionURL` or `ExchangeJoinCodeAtURL` + `RegisterRemoteCap`. No `?cap=` URL is built by hand |
| 02-08: MUST NOT kill or stop sharing when a tab closes | Holds | `closeTab` → `removeTabState` only. Kill happens only in the confirmed `endSession` |

### Required Artifacts

| Artifact | Status | Details |
|----------|--------|---------|
| `main.go` (macWindowOptions, appMenu, helpURL) | ✓ VERIFIED | Menu tests pass |
| `menu_test.go` | ✓ VERIFIED | 5 tests pass |
| `frontend/src/components/Sidebar.tsx` | ✓ VERIFIED | 185 lines, wired from App with live props |
| `frontend/src/lib/sidebarPeers.ts`, `tabs.ts`, `useColorScheme.ts`, `terminalPalettes.ts`, `useDialogFocus.ts`, `helpUrl.ts` | ✓ VERIFIED | Imported and used. Tests pass |
| `frontend/src/components/TabBar.tsx`, `ContextMenu.tsx`, `EmptyState.tsx`, `SettingsSheet.tsx`, `Switch.tsx`, `TailscaleBanner.tsx` | ✓ VERIFIED | Rendered by App. Tests pass |
| `frontend/src/components/Share/*` (moved from Hub) | ✓ VERIFIED | Single App-level instance |
| `frontend/src/components/WebShareSessionView.tsx` | ✓ VERIFIED | Used for web-guest and peer tabs |
| `appearance_darwin.go` / `.m`, `app.go` Get/SetAppearance, OpenTailscale | ✓ VERIFIED | Bound in hand-edited `App.d.ts`/`App.js` |
| `internal/relay/protocol.go` (RETIRED), `server_inject_test.go`, `webserver/inject_test.go`, `*/chat_removed_test.go`, `appearance_settings_test.go`, `settings_removed_test.go`, `assets_test.go` | ✓ VERIFIED | Present. Named tests pass |
| `frontend/src/__tests__/style.tokens.test.ts`, `frontend/e2e/web-color-scheme.spec.ts` | ✓ VERIFIED | Pass |

### Key Link Verification

| From | To | Status |
|------|----|--------|
| main.go menu (new-session, settings, close-tab, toggle-sidebar) | App.tsx `EventsOn(...)` (`App.tsx:525-534`) | WIRED |
| ListSessions 3 s poll | Sidebar This Mac rows | WIRED |
| Sidebar peer row | handleOpenRemoteSession → OpenRemoteSessionURL / RemoteJoinCodeModal → openWebSessionTab | WIRED |
| useColorScheme(override) | terminalTheme → every TerminalPanel / WebShareSessionView | WIRED |
| SettingsSheet Appearance | SetAppearance → daemon PATCH → setAppAppearance; onAppearanceChange → App state | WIRED |
| app.go startup / RetryDaemon | applyStoredAppearance | WIRED |
| TabBar Share | SessionShareModal (FunnelRiskPanel) | WIRED |
| TabBar Share (Tailscale off) | `#ts-off-reason` in TailscaleBanner via aria-describedby | WIRED |
| End Session menu items | RegenerateKeyModal confirm → ToggleWebServing(false) → KillSession | WIRED |
| TailscaleBanner Open Tailscale | app.go OpenTailscale (`/usr/bin/open -a Tailscale`, URL fallback) | WIRED |
| relay/webserver read pumps | hub.WriteInput (only MsgInput, RO-gated) | WIRED |

### Data-Flow Trace (Level 4)

| Artifact | Data | Source | Real Data | Status |
|----------|------|--------|-----------|--------|
| Sidebar This Mac | `sessions` | `ListSessions()` daemon RPC every 3 s, minus exited ids | Yes | ✓ FLOWING |
| Sidebar peer groups | `peerView(...)` | `GetRemoteSessionsWithMeta()` every 30 s while Tailscale is on | Yes (in mocks only; the live peer path is human-verified) | ✓ FLOWING (code) |
| Sidebar footer | `tailscaleHealth` | `GetTailscaleStatus` + `tailscale:health` events | Yes | ✓ FLOWING |
| TabBar labels | `tabLabel(t, sessions)` | session list | Yes | ✓ FLOWING |
| Terminal palette | `terminalTheme` | matchMedia + `GetAppearance()` | Yes | ✓ FLOWING |

### Behavioral Spot-Checks (run by the verifier)

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| CHAT-01 retired frames and removed routes; appearance API; menu; window options | `go test -count=1 -run 'TestChatRemoved\|TestRetiredFrames\|TestAppearance\|TestSetAppearance\|TestMacWindowOptions\|TestAppMenu\|…' . ./internal/daemon ./internal/relay ./internal/webserver` | 4 packages ok. 20 named tests PASS | ✓ PASS |
| Sidebar, tabs, App nav, settings, relay client, token gate, color scheme | `vitest run` on 11 named files | 11 files, 315 tests pass | ✓ PASS |
| Web share page follows the browser; guest sees no sidebar | `playwright test --project=chromium e2e/web-color-scheme.spec.ts e2e/web-share-scope.spec.ts` | 5 passed | ✓ PASS |
| TESTING.md traceability | `bash tests/check-traceability-paths.sh` | OK: 92 paths | ✓ PASS |

The orchestrator also reported a full regression gate: `go test -race -short` ok in every package and vitest 91 files / 1160 tests. I did not repeat the full suite.

### Probe Execution

Step 7c: SKIPPED. No phase-declared probes and no `scripts/*/tests/probe-*.sh`.

### Requirements Coverage

| Requirement | Source Plans | Status | Evidence |
|-------------|--------------|--------|----------|
| NAV-01 | 02-04, 02-12 | ✓ SATISFIED | Truth 2 |
| NAV-02 | 02-04, 02-12 | ? NEEDS HUMAN | Truth 3: code wired and unit-tested, cross-Mac attach not observed |
| NAV-03 | 02-04, 02-05, 02-06, 02-11, 02-12 | ✓ SATISFIED | Truth 1 |
| TERM-03 | 02-08, 02-12 | ✓ SATISFIED | Truth 4 |
| CHAT-01 | 02-02, 02-07, 02-12 | ✓ SATISFIED | Truth 6 |
| THEM-01 | 02-03, 02-11, 02-12, 02-13 | ✓ SATISFIED | Truth 7 (as amended 2026-11-15) |
| LOOK-01 | 02-01, 02-03, 02-08..02-12 | ✓ SATISFIED | Truth 8 |

All 7 phase requirement IDs appear in the PLAN frontmatter. REQUIREMENTS.md maps exactly these 7 to Phase 2, so there are no orphaned requirements. TESTING.md Section 4 has rows for every ID (NAV-01 3, NAV-02 4, NAV-03 9, TERM-03 6, CHAT-01 8, THEM-01 9, LOOK-01 12).

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (131 phase-changed files) | — | TBD / FIXME / XXX | none found | — |
| `frontend/src/components/Sidebar.tsx` | 11, 135 | `PEER_PLACEHOLDER` | ℹ️ Info | UI-SPEC copy constant for the empty peer states, not a stub |
| `internal/relay/hub.go:164,182`, `internal/daemon/engine.go:327`, `internal/webserver/server.go:1166-1173` | — | Stale ChatPanel / Hub-card / "chat-capable SPA" comments | ℹ️ Info | Comment-only. Review IN-03, deferred |
| `main.go:141,148,169`, `tray_common.go:41` | — | "AgentHub" in menu and tray labels | ℹ️ Info | Planned. The 02-01 truth names "Hide AgentHub" explicitly, and the rename is Phase 3 (BRND-01) |

The code review disposition shows all 8 warnings fixed. The deferred Info items (IN-01, 02, 03, 04, 06, 08, 12, 14) each carry a reason. None of them blocks a success criterion.

Other notes (Info):
- LOOK-01 says "system accent colour". The accent is a fixed macOS-blue token, as decided in CONTEXT. Following the user's chosen accent live is deferred and not planned.
- IN-04: chat history (`chats/*.jsonl`) and `aliases.json` are left on disk for upgraded users. This is deliberate (removing them deletes data), and the user still has to decide.

### Human Verification Required

### 1. Peer groups and attach (second Mac: M-03, M-04, M-05, M-13)

**Test:** Share a session on another Mac on the tailnet, then click its row under that Mac's group. Try a first open (join code), a reopen (cached capability), the peer asleep, the peer sharing nothing, Tailscale off, and two peer sessions open at once.
**Expected:** A short-hostname group sorted A to Z. The join opens a working `'<host> › <session>'` tab, and the reopen skips the dialog. The state lines match the UI-SPEC. Tabs are isolated and show the 'On <host>' chip.
**Why human:** Needs a second Mac and a live tailnet. Unit tests mock every RPC.

### 2. Share dialog with Tailscale connected (M-01, M-57)

**Test:** Click toolbar Share on a local tab, go through the Funnel risk panel consent gate, then toggle Tailscale off and on.
**Expected:** A 560px sheet with a Done footer and the risk panel reachable. Going off closes the dialog, dims Share, shows the banner with Open Tailscale, and shows the Settings reason inline. Going back on clears all of this live.
**Why human:** The VM had no Tailscale, so Share was always disabled.

### 3. Web share page ignores the desktop override

**Test:** Desktop Appearance on Dark, share link opened in a browser set to Light (and the reverse).
**Expected:** The page follows the browser, live.
**Why human:** Web serving needs Tailscale. Playwright only proves the browser-follow half.

### 4. VoiceOver (M-63, M-66 step 4)

**Test:** With Tailscale off, focus the toolbar Share and Start Server. Walk the sidebar rows and tabs, and open a sheet.
**Expected:** Dimmed and unavailable, with the NET-01 reason read out. The active row is announced as current. A sheet announces its title and traps focus.
**Why human:** Screen reader output.

### 5. 5-second host glance (real hardware, Light and Dark)

**Test:** Look at the sidebar, then make another app frontmost. Look at a single tab and a very long name.
**Expected:** Readable native vibrancy. The selected row turns gray when the window is not key. A single pill is ≤220px and left-aligned. Ellipsis truncation that does not overlap anything.
**Why human:** VM vibrancy is reduced and was only seen in Dark. These checks are visual.

### Gaps Summary

No blocking gaps. Seven of eight success criteria have code, wiring and test evidence, and most of them also have native VM observation. The eighth, peer-session attach (SC3 / NAV-02), is implemented, wired and unit-tested against mocks, but it can only be confirmed with a second Mac on the tailnet. The remaining items are environment-bound (second Mac, Tailscale, VoiceOver, real-hardware vibrancy). They are not code defects, so the status is `human_needed` rather than `gaps_found`.

---

_Verified: 2026-11-16T01:25:33Z_
_Verifier: Claude (gsd-verifier)_
