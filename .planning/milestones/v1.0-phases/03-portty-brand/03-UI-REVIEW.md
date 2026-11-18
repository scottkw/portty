# Phase 03 — UI Review

**Audited:** 2026-11-16
**Baseline:** 03-UI-SPEC.md (approved), inheriting 02-UI-SPEC.md tokens
**Screenshots:** not captured by Playwright (native Wails app). Used provided Tart VM screenshots (1280x720 pt): sbL1, sbD1, v3tray, v3about, v3aboutD, v3prog3, v3set, v3set2, trayzoom50, trayzoomErr
**Interaction captures:** off (workflow.ui_interaction_capture is false)

Evidence caveat: `sbL1.png` and `sbD1.png` (3:06 PM) show **"AgentHub"** as the menu bar app name and the old tray glyph. They predate the rebrand build, so they show the sidebar layout only, not the brand. `v3about.png` shows no About panel (main window only); the About panel is verified from `v3aboutD.png` (Dark) only. Light About is unverified.

---

## Pillar Scores

| Pillar | Score | Key Finding |
|--------|-------|-------------|
| 1. Copywriting | 2/4 | Rename is clean (zero `agenthub` hits), but Settings still shows stale "v3.2/v3.3" copy and AgentHub-era "running CLI" wording |
| 2. Visuals | 3/4 | About panel, tray glyph and menu match contract; empty state shows duplicate New Session entry points |
| 3. Color | 4/4 | Brand blue absent from CSS/web; accent used only on listed elements |
| 4. Typography | 3/4 | Scale inherited; About description wraps to an orphaned "Tailscale." |
| 5. Spacing | 3/4 | No new values; inherited sheet spacing fine; About panel crowded at bottom |
| 6. Experience Design | 2/4 | Every new session opens with an inverted "%" artifact; four New Session affordances on empty screen; Settings needs "Save Plugins" inside a sheet that also has "Done" |

**Overall: 17/24**

---

## Top 3 Priority Fixes

1. **Inverted "%" line on every new session** (WARNING, near-BLOCKER for first impression) — `sbD1`, `v3about`, `v3prog3`: the first thing a user sees in a fresh shell is a reverse-video `%` above the prompt. It reads as a rendering bug in a terminal app. — Investigate zsh PROMPT_SP vs. initial PTY size: the PTY likely starts at a column count different from the xterm.js grid, or the first resize arrives after zsh prints its prompt. Set the PTY size before spawning the shell, or start with `PROMPT_EOL_MARK=''` only as a stopgap.
2. **Stale/off-brand Settings copy** (WARNING) — `PluginsSection.tsx:315` "Default OFF in v3.2 — flips ON in v3.3 after field validation." refers to AgentHub milestones that mean nothing in Portty. Lines 312/314 say "the running CLI" (agent-CLI framing; in Portty it is the shell or a program). Line 301 "Cmd-click (macOS) or Ctrl-click" is cross-platform copy in a macOS-only app. — Delete the v3.2 line; say "a program in the terminal"; say "Cmd-click".
3. **Duplicate New Session affordances on empty This Mac** (WARNING) — `sbL1`: sidebar header "+", sidebar "New Session ⌘T" row, center "New Session" button, toolbar "+". Two identically labeled buttons in one view dilutes the empty state's single primary CTA. — Hide the sidebar "New Session" row when the center empty state is visible (or drop it, keeping the header "+").

Further warnings (not in top 3, still fix):
4. **"Save Plugins" button inside a sheet with "Done"** (`v3set2`, `PluginsSection.tsx:332`) — two commit models in one sheet; a toggle flipped and then "Done" may silently drop the change. macOS settings apply immediately. Auto-save on toggle and remove the button.
5. **About description wraps** (`v3aboutD`) — "A macOS terminal with remote access over / Tailscale." leaves a one-word orphan. Contract-compliant, but either insert a deliberate line break ("A macOS terminal / with remote access over Tailscale.") or accept.
6. **Light-mode About panel and tray tinting unverified** — no screenshot evidence; keep the TESTING.md Section 5 items open.

---

## Detailed Findings

### Pillar 1: Copywriting (2/4)
Passes:
- `git grep -il agenthub -- ':!.planning'` returns nothing.
- Menu bar "Portty", tray menu "Open Portty / shell 1 / Quit Portty" (`v3tray`) match contract exactly.
- About: "Portty", "Version 0.1.0" (single line, no parenthetical), description, `portty.app` link, "Copyright © 2026 Ken Scott. MIT License." (`v3aboutD`).
- Web: `<h1>Portty</h1>` on all pages, dashboard tagline "Terminal sessions shared over your tailnet." (`web/dashboard.html:12`), join error body matches contract (`web/join.html:57`).
Failures:
- WARNING `frontend/src/components/PluginsSection.tsx:315` stale version copy, visible in `v3set2`.
- WARNING `PluginsSection.tsx:312,314` "running CLI" — banned-word spirit (agent framing) though not literally on the banned list.
- WARNING `PluginsSection.tsx:301` "(macOS) or Ctrl-click" in a macOS-only product.
- Info: "Save terminal as text" helper says "Right-click a tab" — fine. Comments in `App.tsx:235`, `TerminalPanel.tsx`, `aggregateProgress.ts` reference v3.2/v3.3 and "Claude's Discretion"; not user-facing.

### Pillar 2: Visuals (3/4)
- About panel is the native AppKit panel with the bundle icon, as contracted. No HTML fallback.
- Tray glyph (`trayzoom50`) is a template image with a partial ring on the P bowl for 50%; error state (`trayzoomErr`) swaps the cursor bar for "!". Crisp at zoom; distinguishable states.
- No logo/wordmark in window (rule met).
- WARNING: empty state (`sbL1`) has four New Session entry points, two with identical "New Session ⌘T" labels; focal point competes.
- Minor: the About link shows a focus box on first open (`v3aboutD`) — AppKit default when the link is first responder; acceptable.

### Pillar 3: Color (4/4)
- `git grep 2457F5|6F93FF -- frontend web` returns nothing; brand colors live only in rasters.
- Accent appears on: selected sidebar row, primary Done/New Session buttons, switch tracks, selected Settings tab, disclosure links, progress underline (`v3prog3`) — all on Phase 2's reserved list.
- Warning banner uses the existing warning token, not brand blue.
- `style.css` contains 71 hex literals; per Phase 2 these must sit inside the two token blocks (LOOK-01 gate). Not re-verified line by line here; rely on the existing gate test.

### Pillar 4: Typography (3/4)
- No new sizes/weights; Bricolage not loaded. Sidebar section labels 11/600, rows 13/400, empty-state heading 17/600 consistent with scale.
- About uses AppKit system fonts per contract; the 11pt description wraps with an orphaned final word (`v3aboutD`).

### Pillar 5: Spacing (3/4)
- No new spacing values introduced. Settings sheet rows, group insets and footer are consistent (`v3set`, `v3set2`).
- About panel: the credits block sits tight against the copyright line (AppKit layout; limited control). Sidebar empty state "Peers appear when Tailscale is connected." indent aligns with section labels.

### Pillar 6: Experience Design (2/4)
- WARNING: inverted `%` artifact on every new session (known; cause uninvestigated). Visible in Light and Dark.
- WARNING: dual commit model in Settings ("Save Plugins" + "Done").
- WARNING: duplicate New Session CTAs.
- Passes: Share button disabled when Tailscale is down, with a clear banner and "Open Tailscale" action; tray menu lists live sessions; progress underline renders on the correct tab; update check silent-on-failure contract implemented per summaries (not visually verifiable).
- Registry audit: skipped (no shadcn).

---

## Files Audited
- .planning/phases/03-portty-brand/03-UI-SPEC.md
- frontend/src/components/PluginsSection.tsx
- frontend/src/components/Sidebar.tsx
- frontend/src/App.tsx, TerminalPanel.tsx, LinkConfirmPopover.tsx, lib/aggregateProgress.ts (comment strings)
- frontend/src/style.css (hex count)
- web/dashboard.html, web/join.html
- VM screenshots listed above

> Orchestrator note (2026-11-16): the Light About panel was observed in the VM; the correct screenshot is scratchpad/v3ax.png (v3about.png was a capture taken before the panel opened). Fixes 1–3 and the Settings Save/Done warning are scheduled in the next quick task alongside the terminal-font setting.
