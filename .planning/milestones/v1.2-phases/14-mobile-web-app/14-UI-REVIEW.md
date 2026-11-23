# Phase 14 — UI Review

**Audited:** 2026-11-22
**Baseline:** 14-UI-SPEC.md (status: draft, checker sign-off boxes unticked, approval "pending")
**Screenshots:** not captured (retroactive audit by instruction: no host GUI, no app start). Visual claims rest on recorded evidence in 14-SIM-CHECKS.md and 14-VM-CHECKS.md plus code reading.
**Interaction captures:** off (not run; interaction findings are code-derived or taken from the recorded simulator and real-iPhone rows)

---

## Pillar Scores

| Pillar | Score | Key Finding |
|--------|-------|-------------|
| 1. Copywriting | 4/4 | Refusal, retry, Reload and error strings match the contract word for word on both the static pages and the app |
| 2. Visuals | 3/4 | Eight keys measured at least 51x44 pt in one row, but the key bar floats 85-117 pt above the keys (accepted iOS limit) and pressed feedback is unevidenced |
| 3. Color | 3/4 | Tokens only and accent confined to armed Ctrl and the retry link, but manifest `theme_color` is dark-only and light-mode contrast was judged from screenshots, not measured |
| 4. Typography | 4/4 | Four sizes (12, 13, 16, 17) and two weights (400, 600) as declared; the 16px control rule is scoped to `pointer: coarse` |
| 5. Spacing | 3/4 | Key bar, safe-area and 4px/8px phone padding match the contract; the post-keyboard gap and a 32 pt portrait settle drift were measured |
| 6. Experience Design | 3/4 | State coverage and Reload escalation are strong; pinch has no gesture-free alternative, and pinch-in and iOS version are unreported on the real phone |

**Overall: 20/24**

---

## Top 3 Priority Fixes

1. **Pressed-state feedback on keys is unproven on iOS** (WARNING). `onPointerDown` calls `preventDefault()` (KeyBar.tsx:34) and the pressed fill relies on CSS `:active` (style.css:2420). iOS Safari often does not apply `:active` without a touch listener, and `preventDefault` on pointerdown can suppress it. No SIM or phone row records a pressed fill, so a user may tap Esc and see nothing. Fix: add a `keybar__key--down` class set in pointerdown and cleared on pointerup/cancel/leave, styled like `:active`. Then record one screenshot in the next phone check.
2. **The pinch-to-zoom and tap paths have no real-phone close-out** (WARNING). PHONE-1 says "Pinch: not reported" and "iOS version: not reported". S-03 pinch-in in the rig opened Safari's tab overview, and the two pinch-in screenshots are byte-identical. The spec admits there is no gesture-free alternative. Fix: run M-147 once on the phone and record iOS version plus a pinch-in/out result. Longer term, add a size control in Settings, since a user with a stuck 6px font has no recovery short of reload.
3. **Light-mode Home Screen launch uses the dark manifest colors** (WARNING). `manifest.json` sets `background_color` and `theme_color` to `#1C1C1E` (a [default] in the spec), so a light-appearance iPhone can show a dark launch flash before the light terminal paints. The terminal itself follows the system scheme. Fix: verify the launch on a light-mode phone, and if it flashes, keep the dark default but add a note in the spec, or drop the manifest `theme_color` and rely on the media-query meta tags.

Further items (lower priority):
4. `KeyBar.tsx` keeps the `|` glyph without `aria-hidden` (the `aria-label="Vertical bar"` overrides it, so VoiceOver is fine). The spec asked for `aria-hidden` on the glyph. Cosmetic divergence.
5. 14-UI-SPEC.md still shows Status: draft, checker sign-offs unchecked and approval pending, though the phase shipped. Close the contract so the baseline is auditable.
6. No key repeat on arrows (spec [default]). Scrolling a long command history or moving the cursor across a line takes one tap per step. Consider repeat on hold in a later phase.

---

## Detailed Findings

### Pillar 1: Copywriting (4/4)
- `web/user-refused.html:12-13` and `UserRefused.tsx:12-13` carry the exact heading "This link only works on your own devices" and body "Open it on a device signed in to the same Tailscale account as the Mac that shared it." Same words on both surfaces, as required.
- `web/service-retry.html:12-14` matches the contract: "Can't check this device right now", the body, and "Try again" as a real `<a class="button" href="">`. No script, no auto-refresh, no host or account names.
- `ErrorBoundary.tsx:46-48` uses "This view hit an error. Reload to try again." in web mode and keeps the old copy otherwise. `SessionEndedBanner.tsx:131-132` renders a fixed "Reload" label.
- `docs/launcher-app.md` keeps the mandatory Messages/iCloud warning (line 31), "Delete the old icon first" (line 43) and the wrong-account explanation (line 45).
- No generic "Submit/OK/Click here" strings were introduced. Minor: the static retry page is the only place a "Try again" appears; the in-app reconnect banner says "Reload", which is consistent with the contract.

### Pillar 2: Visuals (3/4)
- Focal point is right: the terminal takes the space; the key bar is toolbar-colored quiet chrome; the tab strip is hidden while only the session tab exists (`.tab-bar--single`, style.css:2468-2470).
- Icon-only keys all carry `aria-label` (KeyBar.tsx:35), and the toolbar has `role="toolbar"` and an `aria-label`.
- S-01: eight keys measured 51-52 x 44 pt, one row, y=728, which beats the 44 pt minimum.
- S-02/S-05: the bar's bottom edge sat 117 pt above the keyboard on the first pass and 85 pt after, because iOS's own form-assist bar and the Safari URL pill sit between. The literal 8 pt criterion failed. The user accepted it (2026-11-22, confirmed on the real phone), so it is not a defect in the code, but the key bar is not "directly above the keyboard" as SC1 words it. It is still an honest WARNING-level visual divergence.
- Armed Ctrl has fill, a dot (style.css:2434-2442) and `aria-pressed`, so the state is not color-only. S-05 confirms it reads in light and dark.
- Pressed state: only `:active` (style.css:2420). Not evidenced on iOS (see fix 1).
- S-03 evidence is thin: the fixture draws no terminal text, and the two pinch-in screenshots are byte-identical, so the font change is shown by grid readings only.

### Pillar 3: Color (3/4)
- `style.css` keybar block uses only tokens (`--color-toolbar`, `--color-button`, `--color-control-border`, `--color-selection-inactive`, `--color-selection-fill`, `--color-on-accent`, `--color-separator`, `--color-focus-ring`). No hardcoded hex in the key bar, banner or static pages (`dashboard.css` takes tokens too).
- Accent (`--color-selection-fill`) appears on the armed Ctrl and `a.button` only, matching the reserved list. 60/30/10 holds: terminal/window dominant, toolbar and pills secondary.
- Gaps: `manifest.json:5-6` is dark-only for launch (spec [default]); no light-mode launch result is recorded. Contrast of key text on `--color-button` and on-accent text is reused from shipped pairs but was judged from screenshots, not measured.

### Pillar 4: Typography (4/4)
- Sizes in new code: 12 (banner Reload, inherited), 13 (key labels, style.css:2406), 17 (`|` glyph, style.css:2424), 16 (`pointer: coarse` form controls, style.css:2480-2487). Weights: 600 for keys and Reload, 400 for the pipe. This is the contract exactly.
- The 16px rule uses `@media (pointer: coarse)` only, so the desktop UI is unchanged, and S-02 confirms no focus zoom (Esc 51x44 before and after).
- Static pages keep the `dashboard.css` scale as specified. Minor: `a.button` is `1rem`/600 in line with the existing `button` rule.

### Pillar 5: Spacing (3/4)
- Key bar: `height: 44px` plus 4px padding (content-box, style.css:2379-2391), 2px pill inset, side and bottom `env(safe-area-inset-*)`. S-05 measured keys 38 pt above the window bottom, so the home-indicator band is filled.
- Phone terminal padding `4px 8px` (style.css:2475), tab strip side padding `max(12px, env(safe-area-inset-*))`, root top pad `env(safe-area-inset-top)`: all as specified. All multiples of 4 apart from the declared exceptions.
- `a.button` uses `padding: 12px 20px` and `min-height: 44px` per contract.
- Deviations: the S-02 portrait pass settled 32 pt lower on the second portrait pass (117 vs 85 pt), so settled position was not identical across rotation. `overflow-x: auto` on `.keybar` is the safety net below 352 pt and was not exercised.

### Pillar 6: Experience Design (3/4)
- State coverage is good. Reload appears on stopped, ended, unreachable, reconnect-stuck (after two timeouts) and ErrorBoundary. S-07 and S-08 confirm Reload at 57x44 pt after share-off and session end. S-07 also shows the dead icon landing on "Link No Longer Works".
- Key bar is hidden for read-only caps and for preview tabs (`WebShareSessionView.tsx:134` `showKeyBar`), and Ctrl is one-shot with no timeout. ErrorBoundary and the refusal screen replace the view instead of dead-ending silently.
- Real-phone evidence: PHONE-1 and PHONE-2 PASS (page load under 5 s, standalone launch keeps the query, 10-minute lock, Wi-Fi toggle and app switch all resumed). But resume time, iOS version and pinch are "not reported".
- Gaps: no gesture-free font-size control (spec admits it), no key repeat, no pressed-feedback evidence, a failed pinch-in in the rig that was never replayed on a phone. A user who pinches to 6 px has no in-UI way back to the host font other than reloading.
- No destructive actions in this phase, so no confirmation needed. Registry audit skipped (no shadcn, no third-party registries).

---

## Files Audited
- frontend/src/components/KeyBar.tsx, UserRefused.tsx, ErrorBoundary.tsx, SessionEndedBanner.tsx, PdfPreview.tsx (header and Done layer), TabBar.tsx (`tab-bar--single`), WebShareSessionView.tsx (`showKeyBar`)
- frontend/src/style.css (banner, keybar and phone blocks), web/user-refused.html, web/service-retry.html, web/assets/dashboard.css, frontend/public/manifest.json, docs/launcher-app.md
- Evidence: 14-UI-SPEC.md, 14-SIM-CHECKS.md, 14-VM-CHECKS.md
