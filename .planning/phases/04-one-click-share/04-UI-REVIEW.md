# Phase 04 — UI Review

**Audited:** 2026-11-17
**Baseline:** 04-UI-SPEC.md (with 02-UI-SPEC.md tokens inherited)
**Screenshots:** Playwright not used (native Wails app). Six Tart VM screenshots supplied by the orchestrator (vmpreview build, fake Tailscale host 127.0.0.1): empty state, session open, popover off with the CT line, popover on, popover after unshare, two tabs with one shared.
**Interaction captures:** off (workflow.ui_interaction_capture is false). Interaction findings come from reading the code, apart from what the six VM screenshots show.

---

## Pillar Scores

| Pillar | Score | Key Finding |
|--------|-------|-------------|
| 1. Copywriting | 3/4 | Every fixed string matches the contract word for word. But `<detail>` is a raw `String(err)`, so errors can read "Couldn't start sharing: Error: …." |
| 2. Visuals | 3/4 | The popover reads as an NSPopover. But a shared session reopened before the link loads shows an on switch with nothing under it, and the link field shows only `https://host:port/se…` |
| 3. Color | 4/4 | Accent appears only on the switch track, the "Shared" button and the selected row. No hex values in the new components |
| 4. Typography | 4/4 | Uses only 13/400, 12/400 (mono field 12) and 17/600, all from the inherited scale |
| 5. Spacing | 3/4 | 16 padding, 12 row gap, 8 Copy gap and 320 width all match. The arrow is 16×9 (contract says 16×8), and the 12px link gap is built from 8 + a 4px margin patch |
| 6. Experience Design | 3/4 | Covers starting, stopping, failure, Tailscale-off and copy-failure states. Gaps: a blank body while the link loads, and errors that aren't cleared in some paths |

**Overall: 20/24**

---

## Top 3 Priority Fixes

1. **Shared but URL not yet loaded leaves the popover empty** (WARNING): `SharePopover.tsx` sets `body = url ? (...) : null` in the shared branch, and `useState('')` starts the URL empty. When you reopen the popover on an already-shared session, the switch is on with nothing under it until `GetSessionShare` resolves. If that call fails, it stays empty for good, because `.catch` swallows the error. Fix: show `SHARE_STARTING` (or a "Loading link…" Caption) while `shared && !url`, and show the stop-error style line if `GetSessionShare` rejects.
2. **Raw error text inside the fixed error copy** (WARNING): `toggle()` builds `errorDetail: String(err)`, which yields "Error: …", and the detail often already ends with a period. So the shown text can read "Couldn't start sharing: Error: dial tcp …: connection refused.. Check that…". Fix: strip a leading `Error: ` and any trailing period from the detail inside `shareStartError` / `shareStopError` / `menuShareError` / `menuStopError`.
3. **The link field shows nothing that tells links apart** (WARNING): at 320px with 12px mono, the field shows only scheme + host + port + "se" (p4d). On real hardware the `*.tailXXXX.ts.net` host is longer, so the field shows only the host. The contract accepts horizontal scrolling, but this hides the one part that differs between links. Fix options: scroll the field to the end on mount (`input.scrollLeft = input.scrollWidth`), or leave it and record that you accept it in the backstop row.

---

## Detailed Findings

### Pillar 1: Copywriting (3/4)
- PASS: `frontend/src/lib/shareCopy.ts` matches the contract for the switch label, description, "Getting a link…", the locked warning, Copy/Copied!, the copy-failure line, the CT line, both start errors (general and `https_off`), the stop error and both menu banners. Curly quotes are used in the GUI text.
- PASS: `LinkNoLongerWorks.tsx` and `web/link-invalid.html` carry the exact heading and body copy, and show no host, session or ID.
- PASS: The menu items read "Share" / "Stop Sharing" with no ellipsis (`Sidebar.tsx:241-242`).
- WARNING: `SharePopover.tsx` `toggle()` catch block passes `String(err)` as `<detail>`, so a JS Error prints its "Error: " prefix and may double the period (see Fix 2).
- INFO: In p4c the CT line reads "certificate for 127.0.0.1". That comes from the vmpreview host. The code takes `tailscaleDomain` and hides the line when it is empty, so this is correct on real hardware.

### Pillar 2: Visuals (3/4)
- PASS (p4c/p4d): The popover hangs under the Share button with the arrow centred on it and no title, close button or footer. The off state's visual anchor is the switch row, and the on state's is the link field, as the checker notes ask.
- PASS (p4d/p4f): Sharing shows in three places that don't depend on colour alone: the button label "Shared", the sidebar `SignalIcon` (also visible on the unselected row "shell 2" in p4f) and the switch.
- WARNING: The blank shared-body state (Fix 1).
- WARNING: The truncated link (Fix 3).
- MINOR: In p4d the warning wraps with a one-word last line ("control."). The copy is locked, so this is cosmetic only. Narrowing the padding is not allowed, so accept it.
- MINOR: In both themes the arrow stroke is very faint against the shadow (p4c). Confirm in Dark mode during the M-NN Tart check.

### Pillar 3: Color (4/4)
- `.share-popover` uses `--color-popover`, `--color-control-border` and `--shadow-popover`. The link field uses `--color-field`. Status lines use `--color-text-secondary`, and errors use `--color-error-text` (`style.css:1530-1620`).
- Accent appears in exactly the three places the contract allows: switch track, "Shared" button and selected sidebar row (p4d). The popover, field, Copy button and warning use no accent. Stop Sharing has no destructive style.
- A grep for hex values in `SharePopover.tsx` and `LinkNoLongerWorks.tsx` returns 0 matches.
- Finding, informational: the radius uses `--hub-radius-md` (10px), not the `--radius-lg` named in the contract. The value matches, but the token name differs.

### Pillar 4: Typography (4/4)
- Popover base is 13/18 (Body). `.share-popover__line` is 12/16 (Caption). The link field is 12px mono. `.link-dead__heading` is 17/600/22 (Display), and `.link-dead__body` is 13/18 secondary. No new sizes or weights.
- `web/link-invalid.html` reuses `dashboard.css` for its `h1`/`h2.error`, as the contract requires (legacy CSS untouched).

### Pillar 5: Spacing (3/4)
- PASS: padding 16, switch-row gap 12, row min-height 24, state gap 8, link-to-Copy gap 8, field 24px tall with padding `0 8px` and radius 6, Copy 24px tall with padding `0 12px` and min-width 76 (fits "Copied!"), width 320 clamped to 12px from both window edges (`SharePopover.tsx` `place()`). The code adds the left clamp the contract left unresolved.
- WARNING: The arrow SVG is 16×9 with `top: -9px` (`SharePopover.tsx`, `style.css:1557`). The contract says 16×8 with the base flush with the top edge. The extra pixel covers the border seam, which works, but it breaks the stated dimension. Either record it as an exception or set it to 8.
- MINOR: `.share-popover__link-row { margin-top: 4px }` adds to the container's 8px gap to make the 12px from the contract. This reads as a patch. Use a `margin-top: 12px` row in its own wrapper, or document it, so later state changes don't break the gap.

### Pillar 6: Experience Design (3/4)
- PASS: Starting state: switch on with `saving` (aria-disabled), "Getting a link…" in an `aria-live="polite"` region. Stopping: the body empties at once, and `:empty` hides the container. Start failure: the switch goes back to off, with the error above the description. Stop failure: the link row stays, with the error under the warning. Copy failure: error line plus `select()`. Tailscale off: switch blocked only when not shared, with `aria-describedby` set to the reason line. Turning Share off is never blocked.
- PASS: Esc returns focus to the button. Clicking outside closes the popover. Window blur closes it. Tab/Shift+Tab wrap. Focus moves to the switch on open. The button has `aria-haspopup="dialog"`, `aria-expanded` and `aria-controls` (`TabBar.tsx:292-296`). The menu's disabled flag skips the item on ↑/↓ and ignores click and Enter (`ContextMenu.tsx:54-107`).
- PASS (p4f): After closing, focus returns visibly to the Share button (focus ring).
- WARNING: Blank body or silent failure while the link loads (Fix 1). `GetSessionShare(...).catch(() => {})` hides a real failure.
- WARNING: In the `tailscaleOff` branch, `errorLine(['start'])` and `errorLine(['stop'])` render around the reason line. A stale start error from before Tailscale dropped stays on screen next to "Tailscale isn't connected…", so two messages show at once. Clear `error` when `tailscaleOff` changes.

Registry audit: skipped (no shadcn, no `components.json`, no third-party registries).

---

## Files Audited
- frontend/src/components/SharePopover.tsx
- frontend/src/lib/shareCopy.ts
- frontend/src/components/LinkNoLongerWorks.tsx
- frontend/src/components/TabBar.tsx (Share button attributes)
- frontend/src/components/ContextMenu.tsx, frontend/src/components/Sidebar.tsx (Share / Stop Sharing items)
- frontend/src/style.css (`.share-popover*`, `.link-dead*`, tokens)
- web/link-invalid.html
- VM screenshots p4a–p4f
