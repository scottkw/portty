# Phase 01 — UI Review

**Audited:** 2026-11-15
**Baseline:** 01-UI-SPEC.md as amended by 01-CONTEXT.md Amendments 1-3 (Amendment 1 removed the shell line and Settings → Paths; Amendment 2 removed all status, exit and bell UI; Amendment 3 gated Start Web Server). Chat removal and system light/dark are deferred to Phase 2 and are not scored as gaps.
**Screenshots:** not captured (no dev server; code-only audit)
**Interaction captures:** off (workflow.ui_interaction_capture is false)

---

## Pillar Scores

| Pillar | Score | Key Finding |
|--------|-------|-------------|
| 1. Copywriting | 3/4 | Contract copy is exact, and the NET-01 sentence comes from one constant. The disabled Share button's accessible name is replaced by the reason sentence |
| 2. Visuals | 3/4 | New Session modal has the intended focal point. The Tailscale banner uses a text glyph `⚠` instead of the Heroicons convention |
| 3. Color | 2/4 | The new banner adds hardcoded dark hexes, including amber `#f59e0b`, outside the token system, and has no light-theme override. This breaks the "no new colors" rule |
| 4. Typography | 3/4 | 13px body is correct. The banner icon uses 14px, which is not in the declared scale |
| 5. Spacing | 3/4 | Banner padding `12px 16px` and a 3px border-left are new rules that use values off the 4/8/16/24 scale |
| 6. Experience Design | 3/4 | Tailscale gating is solid (auto-recheck, the Share dialog closes on disconnect, Start is gated). The disabled-control reason is only in `title` on a disabled button, so keyboard users and some hover cases never see it |

**Overall: 17/24**

---

## Top 3 Priority Fixes

1. **The Tailscale banner sits outside the token system** (`style.css:2068-2088`: `#16161e`, `#292e42`, `#f59e0b`, `#a9b1d6`, and no `[data-ui-theme="light"]` override). In light theme you get a dark strip at the top of a light app. Fix: use `var(--hub-surface)`, `var(--hub-border)` (or its equivalent), `var(--hub-text-muted)` and an existing warning token. If no warning token exists, use `--hub-destructive` or declare one in the spec. Do not add a raw amber hex. WARNING
2. **The reason a control is disabled is only in a tooltip** (`StatusBar.tsx:54-56`, `SessionCard.tsx:501-503`, `SettingsTab.tsx:538-546`). A disabled `<button>` cannot be focused and often shows no tooltip in WKWebView, so the reason is hard to find. The persistent banner covers this in practice, but Start Web Server in Settings has no visible inline reason. Fix: render `TAILSCALE_OFF_REASON` as `settings-panel__description` text under the Start button when `tailscaleOff`, or use `aria-disabled` plus `aria-describedby` instead of `disabled`. WARNING
3. **The accessible name of the disabled Share button changes** (`StatusBar.tsx:56`: `aria-label={shareDisabledReason}`). A screen reader then announces only the sentence and loses "Share", and when enabled the button has no aria-label override at all. Fix: keep the name stable (`aria-label="Share session"`) and put the reason in `aria-describedby` or `title`. The same pattern appears in `SessionCard.tsx:502`. WARNING

---

## Detailed Findings

### Pillar 1: Copywriting (3/4)
- PASS: The New Session modal copy matches exactly: "New Session", "Working Directory", "Home directory (default)", "Browse…"/"Browsing…", "Close", "Create Session"/"Creating…" (`NewSessionModal.tsx`). The shell line was removed as Amendment 1 requires.
- PASS: The NET-01 sentence is one constant (`lib/tailscale.ts:3`) shared by the banner, StatusBar, SessionCard and Settings (`SettingsTab.tsx:488,543`). It matches CONTEXT word for word.
- PASS: "Keep sharing off" (`ShellWebShareBanner.tsx:125`) and the Read-Only Link description (`TailnetTab.tsx:37`) match the contract.
- PASS: No agent, file-browser, Linux/Windows, Keep Open or Inject strings remain in non-test TSX. The only matches are in test files that assert the strings are absent.
- WARNING: `StatusBar.tsx:56` and `SessionCard.tsx:502` replace the button's name with the reason sentence (see Fix 3).
- MINOR: Settings has "Tailscale Status: Not connected" with the full NET-01 sentence directly below it. That repeats the banner. It is acceptable but redundant.

### Pillar 2: Visuals (3/4)
- PASS: The modal hierarchy is the accent "Create Session" button first, then the accent directory path. Nothing else competes.
- WARNING: `TailscaleBanner.tsx:11` uses the Unicode `⚠` glyph. The spec's icon convention is `@heroicons/react` 24/outline (`ExclamationTriangleIcon`). Glyph rendering varies by font, and the WebGL and update banners use Heroicons.
- PASS: The banner is placed first in `.banner-stack` (`App.tsx:1287`), uses `role="status"`, and correctly has no dismiss button.
- MINOR: The close button in the modal header uses `&times;` text, not `XMarkIcon` 20/solid as the spec's icon convention says. This is pre-existing.

### Pillar 3: Color (2/4)
- BLOCKER-adjacent WARNING: The spec rule is "add no new colors". The `.tailscale-banner` rules (`style.css:2068-2088`) are new CSS with 4 hardcoded hexes. `#f59e0b` amber appears in neither the spec's color table nor the status palette (it occurs 4 times in style.css). There is no light override. Light theme is a declared theme today, even though THEM-01 rewrites theming in Phase 2.
- MINOR: `SettingsTab.tsx:497` uses an inline `style={{ color: '#9ece6a' }}` on the CT check. This is pre-existing and was not touched in this phase.
- PASS: Accent stays on its reserved elements in the modal. Status color usage is gone under Amendment 2.

### Pillar 4: Typography (3/4)
- PASS: The banner message is 13px/400, which matches the body role.
- WARNING: `.tailscale-banner__icon` uses `font-size: 14px` (`style.css:2081`). That size is not in the declared 12/13/16 set. It goes away if you switch to a Heroicon sized 16px.
- PASS: The modal uses only the existing 16/13/12 sizes and weights 400/600.

### Pillar 5: Spacing (3/4)
- WARNING: The banner uses `padding: 12px 16px` and `border-left: 3px`. The spec says new rules use the 4/8/16/24/32 scale and must not copy the 12px exceptions. Use `8px 16px` or `16px`, and a 4px rule.
- PASS: The banner `gap: 8px` is on scale. The modal kept its inherited exceptions untouched, as the spec instructs.
- NOTE: `.banner-stack` caps height at `3 * 53px`. The banner's height is about 44px, so it fits, but the cap assumes a fixed 53px banner.

### Pillar 6: Experience Design (3/4)
- PASS: Tailscale is re-checked on focus and visibility and resumes polling after reconnect (`App.tsx:947-973`). An open Share dialog closes on disconnect (`App.tsx:977`). Remote peers are cleared while off. Start Web Server is gated (`SettingsTab.tsx:538`, Amendment 3).
- PASS: The Settings status shows a dot plus the text "Connected"/"Not connected", so it is colorblind-safe.
- PASS: The New Session Browse button has a disabled loading state, and Create shows "Creating…" and disables. The modal mounts conditionally (`App.tsx:1621`), so `creating` resets each time it opens.
- WARNING: The disabled-reason discoverability issue (Fix 2).
- WARNING: When Tailscale is off, remote session cards and join-code entry points disappear (`setRemotePeers([])`) instead of showing as disabled with the reason. CONTEXT says "disable the Share and remote/peer controls with the same reason". Hiding them is a defensible reading, and the banner explains it, but it differs from the literal contract.
- MINOR: The `<label className="new-session-modal__section-label">` in `NewSessionModal.tsx` has no `htmlFor` or associated control. It is pre-existing.
- MINOR: Errors from remote polling are silently swallowed (`App.tsx:961`). This is pre-existing.

Registry audit: not applicable (no shadcn, no third-party registries).

---

## Files Audited
- frontend/src/components/TailscaleBanner.tsx
- frontend/src/lib/tailscale.ts
- frontend/src/components/NewSessionModal.tsx
- frontend/src/components/StatusBar.tsx
- frontend/src/components/Hub/SessionCard.tsx (share button)
- frontend/src/components/SettingsTab.tsx (Web Server / Tailscale status)
- frontend/src/components/ShellWebShareBanner.tsx
- frontend/src/components/SessionShare/TailnetTab.tsx
- frontend/src/App.tsx (banner stack, NET-01 effects, modal mount)
- frontend/src/style.css (tailscale-banner, folder-display, banner-stack)
