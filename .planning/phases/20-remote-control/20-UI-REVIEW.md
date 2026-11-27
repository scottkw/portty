# Phase 20 — UI Review

**Audited:** 2026-11-26
**Baseline:** 20-UI-SPEC.md (approved), plus 20-06/07/08/10 SUMMARY deviations and 20-VM-CHECKS.md
**Screenshots:** not captured (code and spec audit only, as instructed; nothing was launched on the host)
**Interaction captures:** off (not requested; every interaction finding below comes from reading the code and from the VM-CHECKS evidence)
**Checks run:** `pnpm --dir frontend test` 133 files / 2359 tests pass; `tsc --noEmit` clean

---

## Pillar Scores

| Pillar | Score | Key Finding |
|--------|-------|-------------|
| 1. Copywriting | 3/4 | Every contract string matches word for word. Two small leaks: an unspecified "Shared screen" fallback label, and inconsistent main-display wording in the switch-failed notice |
| 2. Visuals | 3/4 | Hierarchy, icons and grouping follow Surface B. The reason an item is greyed shows only in a hover tooltip, and the one notice slot drops "Showing <name>" |
| 3. Color | 3/4 | Token-only (no literal colours in the Phase 20 CSS) and accent stays on the reserved list. The colourblind rule is broken for greyed items: on screen they show only a dimmed look, with no visible words |
| 4. Typography | 4/4 | Only 11/12/13px and weights 400/600, as declared. No 17px, no monospace |
| 5. Spacing | 2/4 | Six measured values differ from the contract (divider, menu label padding, check gap, popover gaps, popover edge inset) |
| 6. Experience Design | 3/4 | State coverage is thorough and tested. But a hidden bar lets clicks fall through to the Mac outside the reveal zone, and a reconnect keeps focus on the stage (Interaction row 25 says it should not) |

**Overall: 18/24**

---

## Top 3 Priority Fixes

1. **WARNING: clicks on a hidden bar outside the reveal zone go to the Mac.** `.desktop-bar--hidden` has `pointer-events: none` (style.css:2766). The reveal zone is only the top 16px of the middle 50% of the width (DesktopControlBar.tsx:177-178), but the bar runs from y=8 to y=44 and, with full labels, is wider than half of a 1280px window. So a click where the user remembers "Full Screen" or the picker to be (y≈26, or outside the middle half) never reveals the bar. Instead it lands on the stage and is forwarded to the Mac. 20-VM-CHECKS "Notes on the rig" saw exactly this: "a WebDriver click on a hidden control lands on the stage and posts to the Mac". Because the project ranks reliability first, a stray click on an unattended Mac is the worst case. **Fix:** count the hidden bar's own bounding box (plus the reserved 44px row) as part of the reveal zone in `onMove`. Alternatively, have the stage drop a press that starts inside the reserved bar row while the bar is hidden. The contract allows this, since it already says presses in non-picture regions are not forwarded.
2. **WARNING: the reason an item is greyed is not visible on screen** (colourblind and words rule). `BarButton` puts the reason only in `title` plus a visually hidden span (DesktopControlBar.tsx:234-236, 508-512). Send Keys menu items do the same through `title` (DesktopSendKeysMenu.tsx:603). A sighted user sees only the `--opacity-disabled` dim, and the tooltip needs a hover delay. Screen readers are covered, but the project rule "every state in words" is not met for sighted users. The words are there for Locked and Reconnecting, because the state chip names them. They are missing for "Turn on Control first" (View Only) and for "Control needs Accessibility" on Send Keys. **Fix:** when Send Keys is greyed because of View Only, show the reason as a caption row inside the menu, or let the click show a notice ("Turn on Control first"). Today a click on a greyed item does nothing at all (DesktopControlBar.tsx:237-239), so it fails silently.
3. **WARNING: spacing drift from the contract** (6 values; details under Pillar 5). The quickest fix is to update style.css to the contract values: divider height 20px, menu label padding `8px 8px 4px`, check-column gap 4px, popover gaps 16px, popover side inset 24px. If any value was changed on purpose, amend 20-UI-SPEC instead.

Further findings (all WARNING): the held-keys notice overwrites "Showing <name>" (Pillar 2); a nested `role="alert"` inside the `role="status"` live region (Pillar 6); focus is kept on the stage after a reconnect, against row 25 (Pillar 6); the copy issues listed under Pillar 1. No BLOCKERs: no defect stops a user task, and all six blocking VM rows passed.

---

## Detailed Findings

### Pillar 1: Copywriting (3/4)

Verified word for word against the Copywriting Contract: bar labels, greyed tooltips (`barAvailability`, DesktopControlBar.tsx:66-86), the Control tooltip, the picker texts (Loading…, No windows to show, the list-failed text, the Windows caption), the Send Keys items with the Ctrl+Cmd+Q hint and the window-share caption, the Clipboard caption, every copy and paste notice (DesktopView.tsx:160-181), the capture, swap, surface, fallback and held-key notices, both Full Screen notices, the Lock This Mac dialog (Don't Lock / Lock Screen, not Cancel), both popovers, and the three state chips (DesktopView.tsx:136-145). The host strings also match: Showing / `Window: …` (SettingsSheet.tsx:332), the Row 3 link text, and the Row 4 "Control needs Accessibility. Choose Set Up Permissions…" text. There are no generic labels (Submit, OK, Cancel).

- WARNING: DesktopControlBar.tsx:347 falls back to the label **"Shared screen"** before the first surface message arrives. The contract does not list this string. It should either be added to the spec or replaced with "Main Display" or "Switching…".
- WARNING: DesktopView.tsx:308 builds `Couldn't switch to ${sw.name}. Still showing ${sw.previous}.`, where `previous` is the raw surface name ("Main Display"). The success notice, by contrast, says "Showing **the main display**" (DesktopView.tsx:462). The same surface gets two different names. Map "Main Display" to "the main display" in both places, or in neither.
- Accepted deviation (20-10): the Setup sheet's Accessibility how-to keeps the VM-verified pane name "Device Control and Data Access (called Accessibility on earlier macOS)" (DesktopSetupSheet.tsx:124) and uses only the contract's new ending. This was the correct call, but 20-UI-SPEC Surface E should be updated so the contract and the code agree.

### Pillar 2: Visuals (3/4)

What works: a top-center floating bar with four groups and 1px dividers; 16px heroicons that match the Layout table; the Swap toggle shows `CheckIcon` in place of the arrows when on; Full Screen changes both its icon and its name; every icon is `aria-hidden` and sits next to a text or `aria-label` name, so compact mode keeps its names. Chips always pair an icon with words.

- WARNING: the greyed-reason visibility gap (Top fix 2).
- WARNING: there is one notice slot, and a surface switch that released held keys pushes "Showing <name>" first and then "Held keys were released" (DesktopView.tsx:447-466), so the switch confirmation is replaced at once. Interaction row 12 asks for both. The picker label does change, so the information survives, but the confirmation for every viewer (including phones, which have no picker label) is lost. Either queue the held-keys notice after the surface notice's dwell, or combine them ("Showing Display 2. Held keys were released.").
- WARNING: the hidden-bar fall-through (Top fix 1) is also a visual problem, because nothing on screen shows the user where the bar's hit area is.

### Pillar 3: Color (3/4)

- Token gate: the Phase 20 CSS block (style.css:2698-2969) uses only `var(--…)` tokens. No hex or rgb values appear there, and the source-gate test passes.
- Accent (`--color-selection-fill`) is used only on the capture ring (style.css:2706), the `.context-menu__item:focus` row (existing), and `.button--primary` on Lock Screen, Send to Mac and Done. That is the reserved list exactly. The Swap "on" state uses `--color-segment-selected` plus a border plus a check (style.css:2809), not accent, as specified. The dot uses `--color-on-accent` with a `--color-viewer-bg` border, as specified.
- No text is drawn on the video: chips, bar, menus and popovers all sit on `--color-popover`.
- WARNING (colourblind rule): greyed state is shown visually only by `opacity: var(--opacity-disabled)` (style.css:2797-2800, 2967). Dimming is a luminance cue, so it does not depend on hue. But the contract says "The reason is real text, never only a tooltip on hover", and on screen the reason is only a tooltip (see Top fix 2).
- Settings Row 4 keeps the words Granted / Needs permission next to the dot (SettingsSheet.tsx:384), so it passes.

### Pillar 4: Typography (4/4)

Sizes in use: 11px (menu labels, style.css:2881), 12px (captions, popover text, the error), and 13px (bar items, popover title, textarea). Weights in use: 400 and 600 only. The selected segment uses the existing `.share-seg.is-active` 600 style. Key chords are written as plain text with no monospace; "⌘V" appears only in the paste notice the contract fixes. A minor note, not a deviation: the menu labels add `text-transform: uppercase; letter-spacing: 0.04em` (style.css:2883-2884). The spec's mock shows "DISPLAYS" in capitals but does not mention letter-spacing.

### Pillar 5: Spacing (2/4)

Contract values compared with style.css and the components:

| Element | Contract | Implemented | File |
|---|---|---|---|
| Bar group divider | 20px tall | **16px** | style.css:2825 |
| Menu group label padding | `8px 8px 4px` | **`4px 12px 2px`** (2px is off the 4px grid) | style.css:2880 |
| Check column gap | 16px + **4px** gap | `.desktop-menu .context-menu__item { gap: 8px }` | style.css:2856-2858 |
| Popover text → textarea | 16px | **12px** | style.css:2941 |
| Popover button row above | 16px | **12px** | style.css:2964 |
| Popover side margin | 24px (`max-width: calc(100% - 48px)`) | width is clamped to −48px, but the left edge is clamped with `INSET = 12` | DesktopClipboardPopover.tsx:5, 39 |
| Menu caption padding | `4px 8px` | `4px 12px` | style.css:2889 |

Correct: bar 36px tall with 4px padding, 4px gap, 8px radius, `max-width: calc(100% - 48px)`; items 28px tall, `0 12px` (compact `0 8px` below 880px); chips at 52px on bar viewers; the notice offset follows the measured chip height (20-08 fix); popover 320px wide with 16px padding and 10px radius; textarea 96 to 160px. No other arbitrary values were found. Each deviation is only 2 to 12px, but the contract set these numbers explicitly and seven of them are not met.

### Pillar 6: Experience Design (3/4)

Strong coverage: picker loading, empty, failed and switching states with a 5s timeout and a failure notice; every clipboard outcome, including slow (>400ms, persistent) and busy-greying so transfers never overlap; the Safari gesture rule (ClipboardItem with a promise, DesktopView.tsx:646-667); the Copy and Paste popover fallbacks with a 1 MB inline error and `aria-disabled` Send; a Lock confirm with the safe button focused first, a portal that also works in fullscreen (20-07 fix), and the window-share chord never sent even on confirm (DesktopSendKeysMenu.tsx:653); the release chord moves focus to the Control segment; `aria-disabled` rather than `disabled` throughout; capabilities hide items (`canSurface`, `canClipboard`, `fullscreenAvailable`). VM rows E1, C2, W2, D1, P2 and H1 (all blocking) passed.

- WARNING: the hidden-bar fall-through to the Mac (Top fix 1).
- WARNING: a click on a greyed bar button or menu item is silently ignored (DesktopControlBar.tsx:237; ContextMenu.tsx `if (item.disabled) return`). There is no notice, so a sighted user gets no feedback.
- WARNING: Interaction row 25 ("after the picture returns the page is in Control, keyboard off") conflicts with the Control paused row. 20-06 chose to keep focus (`KEEP_FOCUS` includes reconnect). It is documented, but the spec was never reconciled. After a long reconnect, the keyboard is still captured without a fresh notice. Decide the rule and amend the spec or the code.
- WARNING: a failure notice nests `role="alert"` inside the `role="status" aria-live="polite"` slot (DesktopView.tsx:827-831). Some screen readers announce it twice or only politely. Render alert notices outside the status region.
- Note: 20-VM-CHECKS says the Clipboard trigger was "closed again by the click event of a WebDriver element click". The `pressedWhileOpen` handling (DesktopControlBar.tsx:349-358) depends on a mousedown arriving before the click. A synthetic or assistive-technology click with no mousedown works, but a click that ContextMenu's outside-mousedown sees first may toggle the menu shut. This is worth one real-browser check (M-row), but it is not scored as a defect.

---

## Files Audited

- frontend/src/components/DesktopView.tsx
- frontend/src/components/DesktopControlBar.tsx
- frontend/src/components/DesktopSendKeysMenu.tsx
- frontend/src/components/DesktopClipboardPopover.tsx
- frontend/src/components/ContextMenu.tsx
- frontend/src/components/SettingsSheet.tsx (Desktop group)
- frontend/src/components/DesktopSetupSheet.tsx
- frontend/src/lib/desktopClient.ts, frontend/src/lib/desktopInput.ts (behaviour referenced through the view)
- frontend/src/style.css (lines 2690-2969, plus `.share-seg`)
- .planning/phases/20-remote-control/20-UI-SPEC.md, 20-06/07/08/10-SUMMARY.md, 20-VM-CHECKS.md
