# Phase 02 — UI Review

**Audited:** 2026-11-15
**Baseline:** 02-UI-SPEC.md (approved, including the Deviations table and the 2026-11-15 Appearance amendment)
**Screenshots:** captured by the orchestrator in a macOS 27.2 Tart VM at 1280x720 (native Wails app, Playwright not applicable). Primary evidence: Z1, Z2, AP1, ST1, ST2, G3, T1, T4.
**Interaction captures:** off (workflow.ui_interaction_capture is false). Interaction findings are code-derived or taken from the orchestrator's static screenshots.

Out of scope (noted, not scored): "AgentHub" in the menu bar (Phase 3 rename); one-switch Share popover (Phase 4).

---

## Pillar Scores

| Pillar | Score | Key Finding |
|--------|-------|-------------|
| 1. Copywriting | 3/4 | Contract copy lands verbatim; Sharing pane says "Not connected" while the web server reports "Running", contradicting the banner's "sharing are off" |
| 2. Visuals | 3/4 | Layout matches the mockup; terminal text sits 8px from the chrome, and the Dark sidebar renders as a flat mid-grey slab |
| 3. Color | 3/4 | Token system is clean (zero hex outside the two declaration blocks); 109 `--hub-*` alias references remain and Dark-sidebar secondary text contrast is unverified on vibrancy |
| 4. Typography | 3/4 | New chrome uses only 11/12/13/17 and 400/600; legacy 14px, 12.5px and 700 survive in rename input and Share/Funnel rules |
| 5. Spacing | 2/4 | Terminal padding is 8px, not the contracted 12px/16px, on the largest surface; 13 off-scale 2/6/10px values remain in restyled Share dialog rules |
| 6. Experience Design | 3/4 | States, focus, aria-disabled Share, and destructive confirm all correct; Settings sheet shows a focus ring on Done at open and the Share dialog spacing was not fully rewritten |

**Overall: 17/24**

---

## Top 3 Priority Fixes

1. **Terminal padding is 8px instead of `12px 16px`** (`style.css:28`, `.terminal-session-container { padding: 8px; }`). Visible in Z1/Z2/T4: the `~ %` prompt starts at x=296 against a content edge of 288. The terminal is the declared focal point, and text jammed against the toolbar and banner reads as unfinished next to the 16px insets everywhere else. Fix: `padding: var(--space-md) var(--space-lg)` (12px 16px). `fitTerminal()` already reads the content box, so no JS change. WARNING
2. **Sharing pane contradicts the banner** (ST2, `SettingsSheet.tsx` Sharing pane). With Tailscale off, row 1 says "Not connected" with `TAILSCALE_OFF_REASON` ("Remote access and sharing are off until it is."), and row 2 directly below says "Running on https://192.168.64.108:7443" in LAN mode. Users cannot tell whether their shell is exposed. Fix: when Tailscale is off and LAN mode is running, make the Web server description explicit, e.g. "Running on your local network at <url>. Tailscale sharing is off.", or scope the banner text so it doesn't claim all sharing is off. WARNING
3. **Legacy type and spacing survive in restyled components.** `style.css:1820` `.inline-session-name { font-size: 14px }` (contract: Body 13; rename input visibly jumps 1px larger than the row it replaces). `style.css:1811-1812` `--hub-font-size-base: 14px; --hub-font-size-sm: 12.5px` still feed 15 rules in the Share/Funnel dialog. `style.css:2246` `font-weight: 700`. Off-scale spacing at `style.css:1199, 1419, 1918, 2051, 2054(10px), 2230, 2346, 2355`. Fix: point `--hub-font-size-base` at 13px and `--hub-font-size-sm` at 12px, drop the 700 to 600, and snap the 2/6/10px values to 4/8/12. WARNING

---

## Detailed Findings

### Pillar 1: Copywriting (3/4)

Landed (verified in screenshots or source): "No Open Sessions" / "Start a new shell, or open a session from the sidebar." / "New Session" + "⌘T" (T1); "This Mac", "No sessions", "Other Macs", "Peers appear when Tailscale is connected." (Z1, ST1); "Tailscale not connected" footer; banner `TAILSCALE_OFF_REASON` + "Open Tailscale" (Z1); "End “admin”?" with curly quotes, the contracted body, Cancel / End Session (G3); context menu "Rename" · separator · "End Session…" with U+2026 (T4); `aria-label={`Close ${tab.name}`}` (`TabBar.tsx:273`); "New Session (⌘T)" tooltip (`TabBar.tsx:307`); "Copied!" / "Saved!" are the only exclamations.

- WARNING: Sharing pane contradiction (Fix 2 above, ST2).
- WARNING: Appearance description (AP1) reads "Automatic follows your Mac's Light or Dark setting in System Settings." and wraps to two lines beside the 260px segmented control, ending on the orphan "Settings.". Shorten to "Automatic matches your Mac's appearance." or let the control drop below the label.
- Minor: tab close has `title="Close Tab"` (`TabBar.tsx:272`) while its name is "Close <name>"; harmless but inconsistent with the menu item.
- Minor: "Unicode 11 widths" row adds an italic "Applies to new sessions you create." (ST1). Italic isn't in the type contract; use Caption roman.

### Pillar 2: Visuals (3/4)

- Focal point is right: active terminal dominates, selected sidebar row (accent fill) and active pill (neutral, 600) agree (Z1, Z2, T4).
- Traffic lights sit in the 52px strip with the 80px reserve; no painted dots. Sidebar toggle glyph present with `aria-label="Hide Sidebar"` (`Sidebar.tsx:75`).
- WARNING: terminal inset 8px (Fix 1).
- WARNING: in Dark (Z2, G3, AP1) the sidebar renders as a uniform mid-grey (~#4A4A4C to #555) noticeably lighter than the toolbar and terminal, not the `#262628` declared fallback. This is `html.is-desktop { --color-sidebar: transparent }` (`style.css:1771`) showing vibrancy over a light wallpaper in the VM. It may be correct native behavior, but it inverts the hierarchy (sidebar brighter than content) and needs checking on a real Mac with a dark wallpaper before ship (the UI-SPEC backstop item for vibrancy).
- Minor: empty-state body wraps to leave "sidebar." alone on line 2 (T1). Max-width 320px is per contract; consider 340px or `text-wrap: balance`.
- Minor: a single tab is ~220px wide and left-aligned (Z1), matching the zero-one-many backstop.

### Pillar 3: Color (3/4)

- `style.css` has zero hex / `rgb()` values outside the Light `:root` block (1681-1720) and Dark `@media` block (1726-1764). Every token in the spec table is declared with the spec values. Phase 1's #1 fix (banner hex) landed: Z1/Z2 banners use the warning tokens in both modes.
- Accent use matches the reserved list in screenshots: selected row, Done/New Session primary, switch on (ST2), selected pane tab, highlighted menu item (T4), focus ring, "Search defaults" disclosure link. Active tab is neutral. Destructive red is only on End Session (G3).
- WARNING: 109 `--hub-*` references remain, aliased to new tokens (`style.css:1778+`). This is the spec's permitted path, but aliases like `--hub-text-muted`, `--hub-text-dim`, `--hub-text-placeholder` all collapsing to `--color-text-secondary` hide whether each site meant secondary text. Retire them as files are touched.
- WARNING: on the Dark vibrant sidebar (Z2), "Peers appear when Tailscale is connected." and "Tailscale not connected" in `#A1A1A6` sit on ~#555; that pair is roughly 3:1, below the 5.9:1 the contract computed against the opaque `#262628`. Contract contrast was measured on the fallback, not on the vibrancy actually shown.
- Inline styles: none carry color. `TabBar.tsx:283` (progress scaleX), `ContextMenu.tsx:87` (position), and `FindBar.tsx:233-269` (font/size, Phase 1 component, not touched) only.

### Pillar 4: Typography (3/4)

Distribution in `style.css`: 13px x28, 12px x25, 11px x6, 17px x1, 14px x1, plus `var(--hub-font-size-sm)` (12.5px) x8 and `var(--hub-font-size-base)` (14px) x7. Weights: 600 x23, 400 x6, 700 x1, `var(--hub-font-weight-emphasis)` (600) x2.

- New chrome (sidebar, tabs, banners, empty state, Settings, dialogs) holds to the 4 sizes / 2 weights. Screenshots confirm: 11/600 group headers and pane labels, 13 rows, 17/600 empty heading, 13/600 dialog title.
- WARNING: `.inline-session-name` 14px (`style.css:1820`) violates "Body 13" for the rename input.
- WARNING: `--hub-font-size-base: 14px` / `--hub-font-size-sm: 12.5px` (`style.css:1811-1812`) are off-contract and drive Share/Funnel text (e.g. `style.css:1901, 1954, 2244-2246`). 12.5px is a fractional size that renders blurry-adjacent in WKWebView.
- WARNING: `font-weight: 700` at `style.css:2246` (Funnel write-gate heading) is a third weight.

### Pillar 5: Spacing (2/4)

Distribution of px values in padding/margin/gap: 8px x66, 4px x30, 12px x26, 16px x23, 24px x4; off-scale: 2px x5, 6px x6, 10px x1, 1px x1.

- WARNING (main surface): `.terminal-session-container { padding: 8px }` (`style.css:28`) vs contracted `12px 16px`. This is the largest area in the app and visible in every main-window screenshot.
- WARNING: Share dialog was declared "restyled" with padding rewritten, but `style.css:1419` (`2px 8px`), `1918` (`2px 6px`), `2051` (`gap: 6px`), `2054` (`10px 12px`), `2230`, `2346`, `2355` (`margin-top: 6px`) remain. The spec exempts only the segmented control's 2px inset (`1523`, `1971` are plausibly that) and untouched components; `share-*` rules are new in this phase.
- `1199` `gap: 2px` (WebGL recovery banner shell body) is in a banner the spec puts under the one-banner style.
- Settings sheet (ST1/ST2) is on-scale: 24px content padding, 16px group gap, 12/16 row padding, 52px footer.

### Pillar 6: Experience Design (3/4)

- Phase 1 fixes landed: #2 Web server reason shown inline as the row description (ST2, `SettingsSheet.tsx`); #3 disabled Share keeps the name "Share" with `aria-disabled="true"` and `aria-describedby="ts-off-reason"` and no handler (`TabBar.tsx:317-319`).
- Empty, loading, error states declared in UI Considerations are present: "No sessions", "Peers appear…", the empty state, "Checking Tailscale…". Inactive-window grey row: `html.is-inactive .sidebar__row[aria-current="page"]` (`style.css:468`).
- Destructive confirm focuses Cancel (G3, visible focus treatment on Cancel) per contract.
- Context menu opens at the pointer with the first item highlighted (T4).
- WARNING: the Settings sheet opens with a 3px focus ring on "Done" (ST1, AP1). The contract says initial focus goes to the primary button, but a ring on open, after a ⌘, or menu click, reads as an error state on a sheet whose first real control is Appearance. Put initial focus on the first pane tab or the Appearance group, or keep focus on Done but don't show `:focus-visible` for programmatic focus.
- WARNING: Sharing pane state contradiction (Fix 2) is an experience defect as well as copy: the user can't answer "is my shell exposed?" from the pane.
- WARNING: the context menu (T4) covers the row it was invoked from and the group header "Other Macs". Native NSMenu opens to the right of the pointer; this opens left-overlapping. Low impact.
- Not verified (no interaction capture): Hide/Show Sidebar inset, tab overflow scroll buttons, Esc/focus-return on menus and sheets, live Light/Dark repaint of open terminals. Code exists for each; unit tests cover several per SUMMARYs.

Registry audit: not applicable (no `components.json`, UI-SPEC lists no registries).

---

## Files Audited

- `frontend/src/style.css` (2436 lines; token blocks 1676-1800)
- `frontend/src/App.tsx`
- `frontend/src/components/TabBar.tsx`, `Sidebar.tsx`, `SettingsSheet.tsx`, `ContextMenu.tsx`, `Switch.tsx`, `PluginsSection.tsx`, `ShellWebShareBanner.tsx`, `FindBar/FindBar.tsx`, `SessionShare/*`
- Screenshots: Z1, Z2, AP1, ST1, ST2, G3, T1, T4 (scratchpad)
- `.planning/phases/02-tabs-sidebar-navigation/02-UI-SPEC.md`, `01-plain-macos-terminal/01-UI-REVIEW.md`
