---
phase: 02-tabs-sidebar-navigation
plan: 11
subsystem: frontend-settings
status: complete
tags: [settings, sheet, switch, look-01, them-01, nav-03, accessibility]
requires:
  - 02-09 useDialogFocus and the .dialog sheet shell
  - 02-10 shared banner style (no inline colours in App.tsx)
provides:
  - SettingsSheet (General / Sharing / Security) opened by menu:settings
  - Switch, the one toggle control (Settings and the Share dialog)
  - Inline, aria-described disabled reason for Start Server
affects:
  - 02-13 replaces the static Appearance "Automatic" row with Automatic / Light / Dark
  - 02-12 no-hex sweep has less to do (settings rules are gone or on tokens)
tech-stack:
  added: []
  patterns:
    - "Grouped-form rows: .settings-group > .settings-row (label + Caption description left, control right)"
    - "Blocked action: aria-disabled + aria-describedby pointing at the on-screen reason, click returns early"
    - "Sheet content mounts only while open, so values reload on every open"
key-files:
  created:
    - frontend/src/components/Switch.tsx
    - frontend/src/components/__tests__/Switch.test.tsx
  renamed:
    - frontend/src/components/SettingsTab.tsx -> frontend/src/components/SettingsSheet.tsx
    - frontend/src/components/__tests__/SettingsTab.{,persistence.,shell-warn-toggle.,start-minimized.,tailscale-status.,web-link-ux.}test.tsx -> SettingsSheet.*
  modified:
    - frontend/src/App.tsx
    - frontend/src/components/PluginsSection.tsx
    - frontend/src/components/TabBar.tsx
    - frontend/src/components/Share/SessionShareModal.tsx
    - frontend/src/style.css
    - TESTING.md
  deleted:
    - frontend/src/components/SettingsJumpBar.tsx
    - frontend/src/components/SettingsSearch.tsx
    - frontend/src/components/__tests__/SettingsTab.hyperlinked-index.test.tsx
decisions:
  - "Inactive Settings panes are kept in the DOM with the hidden attribute (ARIA tabpanel pattern); sheet content mounts only while open so values load fresh each time"
  - "Confirms (Stop asking, Regenerate) render as siblings after the sheet so they are the topmost aria-modal for Esc and Tab"
  - "Start Server error copy is built from the start failure; a stop failure reads \"Couldn't stop the web server: <detail>.\"; a blocked reason outranks a stale start error"
  - "Tailscale row reads \"Connected as <short host>\" using the sidebar's shortHost, IP fallback"
  - "Save Plugins is a secondary push button that stays enabled while it reads \"Saved!\" (the old green --saved fill and the dimmed-while-saved state are gone)"
  - "The Share dialog's two toggles moved to Switch here (deferred-items assigned them to 02-11), so the old checkbox toggle CSS could be deleted"
  - "LAN password row uses a Copy button instead of a click-anywhere div with a title tooltip (keyboard reachable, same handler)"
metrics:
  duration: "about 35 min"
  completed: 2026-11-15
  tasks: 3
  files: 33
estimate:
  tokens: 105000
  tasks: 3
actuals:
  tokens: 64000
  tasks: 3
  commits: 4
plan_head_before: 6a233917b2a56e3a9af6e38aae1460c24537de89
plan_head_after: b407b8d4029d62a05f419acf8a15e086bc1cfaeb
---

# Phase 2 Plan 11: Settings Sheet Summary

Settings is now one macOS-style sheet (⌘,) with General / Sharing / Security panes in grouped-form style. Every toggle is a shared `Switch`. A blocked Start Server is `aria-disabled` and described by its reason, which shows on screen. The Settings tab type, jump bar, search and old settings CSS are gone.

## What was built

- **SettingsSheet.tsx** (renamed from SettingsTab.tsx): `.dialog--settings` shell (720×560, max `calc(100% - 48px)`). It has a toolbar-coloured header with the pane name centred, pane tabs (`role="tablist"` "Settings sections"; Cog6Tooth / Signal / LockClosed icons; ←/→ move between them) and a content area that scrolls on its own, so the header and Done stay visible on a 720pt screen. The 52px footer holds a primary Done button, which gets focus on open. Esc closes the sheet through `useDialogFocus`.
- **General:** a static "Automatic" Appearance row (interim, as amended; 02-13 turns it into a control), the "Start hidden in the menu bar" switch, Terminal Plugins rows with switches (copy unchanged), the existing disclosures (native select, 24px fields) and a secondary "Save Plugins" button.
- **Sharing:** a Tailscale row (dot plus "Connected as <host>" or "Not connected", with TAILSCALE_OFF_REASON under it) and the "Ask before sharing a session" switch. Turning that switch off opens "Stop asking before sharing?" (Keep Asking / Stop Asking). The Web server row's description (`#web-server-reason`) always shows the current state: Running on / Stopped / Tailscale off / CT first, or the start-failure copy in `--color-error-text`. Below it are the Port row (72px, right-aligned), the URL row (Open / Copy / QR), the LAN rows and the footnote.
- **Security:** Certificate Transparency ("I Understand", then "Acknowledged" in `--color-success-text`) and Session keys ("Regenerate…" opens the confirm with a destructive "Invalidate All Links" and Cancel focused).
- **Switch.tsx:** `<button role="switch" aria-checked>`, 38×22px, 18px knob, on = `--color-selection-fill`, off = `--color-switch-off`, 120ms slide (none under reduced motion). `saving` dims it and blocks clicks.
- **App.tsx:** `showSettings` state. `menu:settings` sets it. One `<SettingsSheet>` renders, desktop only. The `'settings'` tab type is gone from TabBar.
- **Share dialog:** the two toggle rows use `Switch`. The Funnel consent flow is unchanged: the toggle still only opens the risk panel, and only the CTA commits.

## Deviations from Plan

### Auto-fixed / scope adjustments

1. **[Rule 3 - Blocking] Jump bar, search and the hyperlinked-index test were deleted in Task 1, not Task 3.** The General pane rewrite removed the anchors they index, so the hyperlinked-index test could not stay green through Task 1. Task 2's acceptance line ("ls-files lists only SettingsTab.hyperlinked-index.test.tsx") therefore listed nothing. The final state matches Task 3's criteria. Commit 5ff85868.
2. **[Rule 3 - Blocking] The Share dialog toggles were migrated to Switch** (SessionShareModal.tsx and its test, not in the plan's file list). Deleting `settings-panel__toggle-row` CSS (a Task 3 criterion) would otherwise have left them unstyled, and deferred-items.md assigned this to 02-11. Commit f38d4da9.
3. **[Rule 1 - Bug] A stale start error hid the blocked reason.** Once Start became blocked (for example, Tailscale dropped), the click returned before it could clear the error. The blocked reason now wins. Commit b407b8d4.
4. **Retired tokens:** `--hub-scrollbar*` (UI-SPEC "Inputs and native controls") and `--hub-radius-lg` (its only consumer was the deleted `.settings-panel`). `style.redesign.test.ts` now asserts that `--hub-radius-lg` is gone.
5. **Test retargets (recorded per plan):** `style.settings.test.ts` was retargeted, not deleted. It now pins the sheet, row and Switch rules and their absence of hex. The old `.settings-tab`, `.settings-panel__body h3` and description pins were dropped with those rules. `style.tokens.test.ts` now covers `.settings-`, `.switch`, `.share-toggle-row` and `.ts-status` in its no-hex scan.
6. **Copy:** the URL-row and CT buttons lost their decorative heroicons and inline sizing styles. They are plain secondary push buttons, as the UI-SPEC row table specifies.

## Known Stubs

- `SettingsSheet.tsx` Appearance row: the static "Automatic" text is intentional and interim (CONTEXT amended 2026-11-15). Plan 02-13 replaces it with the Automatic / Light / Dark control and its settings.json key.

## Threat Flags

None. No new endpoints or trust-boundary changes. T-02-34: the blocked Start click returns early (tested) and the daemon's CT and mode checks are unchanged. T-02-35: turning the warning off still needs "Stop Asking". T-02-36: Regenerate opens with Keep Links focused and a destructive confirm.

## Verification

- `go build ./... && go vet ./...` and `go test -race -short ./...`: green
- `pnpm exec tsc --noEmit`, vitest 92 files / 1172 tests: green
- `pnpm exec playwright test --project=chromium`: 7 passed, 9 skipped (as before)
- `bash tests/check-traceability-paths.sh`: OK (91 paths)
- `wails build -platform darwin/arm64 -tags wailsassets`: built `build/bin/agenthub.app` from HEAD b407b8d4. The generated wailsjs tree was restored afterwards.

## Needs VM check

Built app: `/Users/ken/dev/portty/build/bin/agenthub.app` (M-66 in TESTING.md). In Light, then Dark:

1. ⌘, opens one sheet under the toolbar, 720×560. On the 720pt-tall VM screen it fits (max height `100vh - 104px` = 616), and only the pane content scrolls, with the header and Done always visible. No Settings tab appears.
2. Pane tabs: the selected tab has a grey fill and blue icon and label. Mouse and ←/→ (after focusing a tab) switch panes, and the centred title follows.
3. Switches look native (blue on, grey off, white knob with shadow) and slide. "Start hidden in the menu bar" persists across relaunch.
4. Sharing with Tailscale off: the Web server row reads the NET-01 sentence. Start Server is dimmed but focusable, and clicking it does nothing. VoiceOver reads it as dimmed or unavailable and then the reason. With CT unacknowledged and Tailscale on, the description reads "Acknowledge Certificate Transparency in Security first."
5. "Ask before sharing a session" off shows the "Stop asking before sharing?" confirm over the sheet. Esc closes only the confirm.
6. Security: "I Understand" turns into green "Acknowledged". "Regenerate…" opens the confirm with Keep Links focused and a red Invalidate All Links.
7. Done and Esc both close the sheet. A live System Settings appearance switch repaints it.
8. Share dialog (toolbar Share): the two rows show the new switch with the On/Off text beside it. The Funnel switch still only opens the risk panel.
9. Edge case: press ⌘, while the New Session dialog is open. Check whether both dialogs stack sensibly (see deferred-items "From 02-11").

## Self-Check: PASSED

- FOUND: frontend/src/components/SettingsSheet.tsx, frontend/src/components/Switch.tsx, frontend/src/components/__tests__/Switch.test.tsx
- MISSING (intended): frontend/src/components/SettingsTab.tsx, SettingsJumpBar.tsx, SettingsSearch.tsx
- FOUND commits: 5ff85868, 5a0e2ddb, f38d4da9, b407b8d4
