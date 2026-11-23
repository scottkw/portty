# Phase 12 — UI Review

**Audited:** 2026-11-22
**Baseline:** 12-UI-SPEC.md (approved design contract)
**Screenshots:** not captured (instructed: no host GUI, no app start; audit is code plus recorded VM evidence in 12-VM-CHECKS.md)
**Interaction captures:** off (not run; interaction findings are code-derived or from 12-VM-CHECKS.md)

Note: the frontend is plain CSS (no Tailwind), so pillar greps were done on `style.css` tokens and inline strings.

---

## Pillar Scores

| Pillar | Score | Key Finding |
|--------|-------|-------------|
| 1. Copywriting | 4/4 | Both notice strings match the contract character for character, fixed text, no wire text rendered |
| 2. Visuals | 3/4 | Cmd-held underline never observed in the desktop web view (deferred to M-131); notice overlays terminal row 0 |
| 3. Color | 4/4 | Only existing banner-info tokens; light/dark RGB values verified in Safari; no hardcoded colors added |
| 4. Typography | 4/4 | No new sizes or weights; notice is the existing 12px/16px banner |
| 5. Spacing | 4/4 | No new spacing; reuses `.banner` (8px 16px, 8px gap) and `.session-ended-banner` placement |
| 6. Experience Design | 3/4 | Known first-click defect (F-2), row-0 path resolves wrong directory (F-1), Claude Code TUI unchecked, silent on latency |

**Overall: 22/24**

---

## Top 3 Priority Fixes

1. **First Cmd-click fails after the pointer returns to the same cell whose text changed (F-2)** — the user's most common flow (print path, Cmd-click) can need a second try, and with silent failure the user can't tell why — fix in `frontend/src/lib/pathLinks.ts`: reset the Linkifier hover state when buffer content changes under a resting pointer, not only on resize/capability change. Todo already filed: `.planning/todos/pending/2026-11-22-first-cmd-click-same-cell-after-leave.md`.
2. **Desktop Cmd-held underline and dark repaint are unverified in the Wails web view (M-131)** — the core PATH-01 affordance has been proven only in Safari with synthetic DOM events. If WKWebView does not deliver window key events, users get no underline until they move the mouse, and nothing says paths are clickable — run M-131 on the user's Macs before ship; if it fails, take the contract's accepted fallback (underline on plain hover, modifier-gated click). The decision "no fallback" was made without evidence.
3. **Relative path on row 0 of a never-scrolled session resolves against the start directory (F-1)** — Cmd-click previews the wrong file or silently does nothing, which looks like a dead link — fix in the relay (todo `2026-11-22-row0-relative-path-start-dir.md`); also check the Claude Code TUI case (not installed in either VM, so mouse-reporting behavior inside the user's actual daily program is unproven, M-128).

---

## Detailed Findings

### Pillar 1: Copywriting (4/4)
- `SessionEndedBanner.tsx:92-95`: "Can't preview this file type. Portty previews images, PDFs and Markdown files." and "Too large to preview. The limit is 20 MB." match the UI-SPEC copy contract exactly (straight apostrophe, sentence case, no "Error:"). VM rows C-04R read both strings exactly from AX and Safari DOM.
- No filename or host text is rendered. Dismiss keeps `aria-label="Dismiss"`.
- WARNING (minor, no score impact): the too-large notice gives the limit but not the alternative (the user cannot open it any other way in-app); acceptable per contract.
- WARNING (minor): the component is still named `SessionEndedBanner` and carries `reason`/`host` props for six unrelated kinds; the nested ternary at lines 85-104 is now 7 branches deep. Maintenance cost, not a user-facing defect.

### Pillar 2: Visuals (3/4)
- Underline logic in `pathLinks.ts:117-131` flips `link.decorations.underline/pointerCursor` on `modDown`; correct by reading. Only the Safari cursor mechanism was observed (C-06R, synthetic events). C-06 for the desktop app was recorded as FAIL then re-labeled PASS by user decision "Check on my Macs" (12-VM-CHECKS.md line ~123). That is not visual evidence. BLOCKER-adjacent: scored as a warning because the Safari half works and the decision was the user's.
- The notice is `position:absolute; top:0; z-index:10` (`style.css:1244`) and covers terminal row 0 for 5s. By contract, but it hides exactly the rows where a user just printed a path and clicked it, and the user may want to click a second path on that row. Hover/click through the banner is blocked.
- Folders and bare names without extension are not underlined (`pathLinks.ts` `EXTENSION`), which deviates from the spec line "Folders and non-files are still underlined" for extension-less bare relative tokens. Absolute folders are underlined and silent on click. Inconsistent but harmless.
- No tooltip on paths (by contract); the affordance is discoverable only if the user already knows Cmd-click.

### Pillar 3: Color (4/4)
- Notice uses `--color-banner-info-bg/fg` (`style.css:2010-2011` light, `2066-2067` dark). Measured in Safari (C-06R): light bg rgb(232,241,252) on text rgb(11,79,156); dark rgb(31,42,58) on rgb(156,203,255). Both exceed 4.5:1 by calculation (about 7:1 and 8:1). Live switch repainted in 0.9s with no reload.
- Underline inherits the cell foreground; no accent used. Icon plus words carry the meaning (colorblind rule honored).
- No hex or rgb literals introduced by Phase 12 code in the audited files.
- Not verified: the desktop app's dark repaint (M-131).

### Pillar 4: Typography (4/4)
- No new font size or weight. `.banner` is 12px/16px, weight 400; matches the contract caption row. Terminal text untouched, so no reflow.

### Pillar 5: Spacing (4/4)
- Reuses `.banner` padding `8px 16px`, gap 8px, 16px icon, 24px dismiss (`style.css:1255-1325`). All multiples of 4. No arbitrary values added. Notice overlays rather than pushing layout, so no resize is triggered.

### Pillar 6: Experience Design (3/4)
State coverage is strong and verified in VMs:
- Missing, folder, read-only, busy, reconnecting: silent as decided (C-04R, C-05R PASS). Notice only in the clicking pane (`TerminalPanel.tsx:1288`, `isActive && !otherBannerRef.current`); other banners win; timer cleared when pane inactive (line 356) and on unmount (357). Replacement restarts timer (350-355).
- Read-only and old-host viewers get no links (`enabled()` gate, `onPathClickCapability`).
- Modifier-gated click, plain click selects, URL provider wins (`URLISH` overlap skip). Mouse-reporting vim passes (C-07).

Defects and gaps:
- WARNING F-2: first Cmd-click fails when the pointer re-enters the same cell after its text changed. Accepted as a todo, but it breaks the primary flow intermittently.
- WARNING F-1: row-0 relative paths resolve against the start directory (host-side).
- WARNING: `invalidate()` writes private xterm fields `_lastBufferCell` and `_activeLine` via `_core.linkifier` (`pathLinks.ts:150-160`). Guarded and pinned by a test, but an xterm upgrade can silently disable the post-resize/capability refresh with no user-visible error. Chesterton: it exists because the Linkifier caches; keep, but treat xterm bumps as a risk.
- WARNING: no keyboard path (mouse-only, accepted by contract). Screen-reader/keyboard users have `portty open` only.
- WARNING: auto-dismissing 5s `role=status` notice (WCAG 2.2.1 timing): the dismiss button exists but the message disappears before slower readers finish; acceptable for non-critical info.
- WARNING: a slow tailnet click gives zero feedback (accepted latency); combined with silent-miss, users cannot distinguish "slow", "missing" and "broken" (F-2).
- Claude Code TUI untested (not installed in VMs).

---

## Registry Safety
Skipped: no shadcn, no `components.json`, no third-party registries.

## Files Audited
- /Users/ken/dev/portty/.planning/phases/12-file-path-click/12-UI-SPEC.md
- /Users/ken/dev/portty/.planning/phases/12-file-path-click/12-VM-CHECKS.md
- /Users/ken/dev/portty/frontend/src/lib/pathLinks.ts
- /Users/ken/dev/portty/frontend/src/components/SessionEndedBanner.tsx
- /Users/ken/dev/portty/frontend/src/components/TerminalPanel.tsx (path link registration, notice state, render slot)
- /Users/ken/dev/portty/frontend/src/style.css (`.banner`, `.session-ended-banner`, banner-info tokens)
- /Users/ken/dev/portty/frontend/src/App.tsx and WebShareSessionView.tsx (grepped for path-click wiring only; no UI changes of note)
- Not read in full: 12-CONTEXT.md and 12-0*-SUMMARY.md (UI-SPEC and VM evidence were sufficient for the contract comparison)
