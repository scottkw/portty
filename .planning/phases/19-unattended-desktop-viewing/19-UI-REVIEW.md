# Phase 19 — UI Review

**Audited:** 2026-11-26
**Baseline:** 19-UI-SPEC.md (approved 2026-11-25) with 19-CONTEXT.md decisions
**Screenshots:** not captured (code-only audit by instruction)
**Interaction captures:** off (workflow.ui_interaction_capture is false)

---

## Pillar Scores

| Pillar | Score | Key Finding |
|--------|-------|-------------|
| 1. Copywriting | 3/4 | Contract copy matches word for word, but the `capture_paused` state ships copy the contract never defined ("Desktop paused", "Paused after repeated errors. Run Test Capture to resume.") |
| 2. Visuals | 3/4 | Test Capture result lays the thumbnail and caption side by side; contract stacks them. Open Login Items button sits under the Settings footer until the pane scrolls (documented cosmetic defect, 19-15) |
| 3. Color | 3/4 | Tokens used correctly, `--color-viewer-bg` in both blocks, no hex outside the token blocks; the 60/30/10 split and light/dark legibility cannot be checked without screenshots |
| 4. Typography | 3/4 | Only 11/12/13/17 sizes and 400/600 weights in the phase surfaces, but the viewer chip uses a 16px line height where the Body role specifies 18px |
| 5. Spacing | 3/4 | Mostly on the 4px grid; `.desktop-card__icon` uses 8px icon-to-heading gap where the contract specifies 12px; Test Capture result row layout drives the spacing mismatch above |
| 6. Experience Design | 3/4 | Every Surface 5 state and all three distinct errors are implemented with the specified actions; the viewer's `role="status"` region wraps the interactive Tap to Start and Reload Page buttons, so assistive tech re-reads them on every state change |

**Overall: 18/24**

No BLOCKER found. Every pillar has at least one WARNING-level finding. The overall score is limited by what could not be verified (rendered look, sharpness, both appearances), not by a found defect.

---

## Top 3 Priority Fixes

1. **Unsanctioned copy for the `capture_paused` state** — the heading "Desktop paused", its body, the Settings status text "Paused after repeated errors. Run Test Capture to resume." and the tray line "Desktop: Paused after repeated errors" are not in the Copywriting Contract, the state table or the interaction contract. The 19-06 summary calls the card "planner-added", so it was a known addition that never reached the contract. User impact: the copy has no approved wording, and it is the only state that tells the owner to act at the Mac from the viewer. Fix: either add the state and its four strings to 19-UI-SPEC.md (Surface 1 state table, Surface 5 states table, Copywriting Contract, tray column) and have the owner approve them, or reword the strings to an approved state (for example "Needs permission" plus the Test Capture pointer). Do not leave it as silent drift.

2. **Viewer status region wraps interactive controls** — `frontend/src/components/DesktopView.tsx` renders the card, including the Tap to Start and Reload Page `<button>`s, inside `<div role="status" aria-live="polite" className="desktop-status">`. Each state change makes a screen reader announce the heading, body and button label as a status update, and focus on the auto-focused Tap to Start button sits inside a live region. User impact: on the iPhone path this is the one control the user must press. Fix: keep `role="status"` on a text-only element (heading and body or chip text) and render the card's buttons as a sibling outside that region, or drop `aria-live` from the wrapper and give the card `role="group"` with `aria-labelledby` pointing at its heading.

3. **Test Capture result is laid out side by side, not stacked** — `frontend/src/components/DesktopSetupSheet.tsx` puts the 256 x 160 thumbnail and the caption in `.settings-row.desktop-setup__result`, which is a flex row (`frontend/src/style.css` `.settings-row`, gap 12px). The contract shows the caption under the thumbnail (Surface 2 layout and Surface 5 Test Capture table: "thumbnail ..., then Caption"). With the 560px sheet this leaves roughly 212px for the caption, so it wraps to several lines beside the image. Fix: add a column modifier, for example `.desktop-setup__result { flex-direction: column; align-items: flex-start; }`, and give the caption the 8px top gap the contract's `settings-row__note` pattern uses.

---

## Detailed Findings

### Pillar 1: Copywriting (3/4)

Verified word for word against the contract:
- Viewer cards (`DesktopView.tsx` CARDS): Connecting, Needs permission, Waiting for approval at the Mac, Display asleep, Tap to start, Can't reach this Mac, This desktop link no longer works, Desktop sharing is off on this Mac. Chip text: Locked, Reconnecting…, Streaming to N viewer(s).
- Settings Desktop group (`SettingsSheet.tsx`): switch description (off and on), CT disclosure line, userspace sentence, Getting a link…, Copy Link / Copied!, Permissions rows, Background service row and its five descriptions, footnote, the three inline error sentences, the Security pane Session keys sentence.
- Setup sheet (`DesktopSetupSheet.tsx`): title, intro, both how-to lines (Accessibility uses the contract's "Device Control and Data Access" wording), Test Capture states, failure line, both notes, Done.
- Static pages `web/desktop-off.html` and `web/desktop-link-invalid.html` match the contract; `web/service-retry.html` carries the reworded route-neutral body.
- Tray copy in `tray_common.go` and the lapse notification text in `app.go` and `internal/desktop/watchdog.go` match. `RegenerateKeyModal.tsx` carries the added sentence.

Deviation:
- `DesktopView.tsx` line 70 (`capture_paused` card: "Desktop paused" / "Portty stopped showing this Mac's screen after repeated errors. On the Mac, open Portty, go to Settings, then Sharing, and run Test Capture.") and `SettingsSheet.tsx` line 141 (status "Paused after repeated errors. Run Test Capture to resume.", tray "Desktop: Paused after repeated errors", `tray_common.go` line 89) are not in 19-UI-SPEC.md. The contract's Status texts and Tray lines lists have no such entries.

### Pillar 2: Visuals (3/4)

- Focal point and hierarchy are clear on the viewer: one card or one chip at a time, icon plus heading plus body, scrim only where a frame exists. Status is never color alone (every dot has words beside it; every viewer state has an icon and words).
- Test Capture result layout deviates (see Top Fix 3): `.desktop-setup__result` is a `.settings-row` flex row, so thumbnail and caption sit side by side.
- The 19-15 summary records an open cosmetic defect: once the Background service row shows, the Open Login Items button sits under the Settings footer until the Sharing pane is scrolled. The button is a repair action, so it should be reachable without scrolling tricks. Fix: let `.settings-sheet` body scroll with the footer pinned, or move the row above the footnote.
- Icon-only controls: none in this surface. Decorative icons carry `aria-hidden="true"`.
- Rendered appearance was not checked (WebKit screenshots are blank in the VMs; the owner checks M-rows cover it). Score is capped on that basis.

### Pillar 3: Color (3/4)

- Tokens: `--color-viewer-bg: #000000` is declared in both the light and the dark token blocks (`frontend/src/style.css` lines ~2154 and ~2208). A grep for hex colour values outside the two token blocks finds only comments.
- Accent discipline: `.button--primary` is used for Done (setup sheet) and Tap to Start (viewer). Copy Link, Test Capture, Open System Settings, Set Up Permissions, Review Permissions and Reload Page are `.button--secondary`. Consistent with the contract's accent list.
- Status dots: `ts-status__dot--ok/--warn/--error` map to `--color-status-on`, `--color-status-warn`, `--color-status-off`. Granted uses `--color-success-text`, inline errors `--color-error-text`.
- Viewer text sits on `--color-popover` surfaces only; no text is drawn on the video or on `--color-viewer-bg`.
- Not verifiable in code: the 60/30/10 proportion as rendered, the look of chips and cards over the letterbox in light and dark (the contract marks this as a backstop), and the 1:1 sharpness judgement. These remain owner checks.

### Pillar 4: Typography (3/4)

- Phase surfaces use 11px (section label), 12px (captions, status text, footnotes), 13px (row labels, buttons, chip text, card body) and 17px (viewer card headings). Weights are 400 and 600 only. This matches the contract's four-size scale.
- Deviation: `.desktop-chip` sets `font-size: 13px; line-height: 16px`. The contract's Body role (used for the chip text) specifies 18px line height. Fix: change the chip's line-height to 18px and let the 32px min-height hold the box.
- Other off-scale sizes in `style.css` (14px, 16px, 20px, 24px) were introduced by earlier phases (commits 954ca500 and 48f3b06a), not Phase 19; they are out of this audit's scope.
- The contract's known 1px-step FLAG (11/12/13) remains non-blocking and unchanged.

### Pillar 5: Spacing (3/4)

- Phase values are on the 4px grid: row padding 12px 16px, gaps 8px and 12px, sheet padding 24px, card padding 24px, card side margin `calc(100% - 48px)`, chip padding 8px 12px, 44px viewer tap targets, 256 x 160 thumbnail, 560px setup sheet, 320px card max.
- Deviation: `.desktop-card__icon` has `margin-bottom: 8px`; the contract's spacing table assigns 12px to the card icon-to-heading gap.
- Deviation: the Test Capture result row layout (see Top Fix 3) changes the intended vertical rhythm of the result block.
- Copy Link `min-width: 76px` is a fixed number, not measured against the rendered "Copied!" width. Confirm on screen when the owner check runs.

### Pillar 6: Experience Design (3/4)

Strengths, verified in code:
- Viewer state coverage: Connecting, Streaming chip (3 s on first frame and on count change), Locked chip, Needs permission, Waiting for approval, Display asleep, Reconnecting chip over a frozen frame, Tap to Start (with scrim tap), Can't reach this Mac after 30 s with Reload Page (`desktopClient.ts` `unreachableAfterMs: 30000`), and the three distinct error pages and cards. Other-account state reuses `UserRefused`.
- Reconnect policy: capped backoff `[1000, 2000, 4000, 8000, 10000]` ms, wake on visibility, pageshow and online.
- Settings: switch uses `aria-disabled` (not `disabled`) so focus stays; blocked switch has `aria-describedby` to the visible reason; turning off acts at once with no confirm; Copy Link has the 2 s Copied! state; the setup sheet polls every 2 s and on window focus with no timeout; Test Capture is blocked only on a known "Not granted"; Esc and Done close and return focus.
- Failed reads are shown as Checking…, never as a default "Not granted" (the `errorCode` handling in `SettingsSheet.tsx` and `DesktopSetupSheet.tsx`).

Gaps:
- `DesktopView.tsx` places interactive buttons inside `role="status" aria-live="polite"` (see Top Fix 2).
- The `capture_paused` state is a real error state that the contract never defined (see Pillar 1). Its recovery path (Test Capture, in the setup sheet) is reachable only by going to the Mac, which the card says, so the flow is complete but its copy is unreviewed.
- Permissions row (Settings Row 4) refreshes only when the setup sheet closes, not on a timer. The contract's interaction row 8 accepts this ("Row 4 reflects the grants" after Done), so this is noted, not counted.
- Interaction-driven behaviour (Tap to Start on iOS, reconnect on sleep and wake, multi-viewer chip) is covered by the unit tests and the 19-VM-CHECKS rows recorded in the phase summaries, not by this audit. The audit does not claim it.

---

## Registry Safety

Registry audit: skipped. `frontend/components.json` is absent and 19-UI-SPEC.md lists no third-party registries (`shadcn_initialized: false`, Registry Safety: none).

---

## Files Audited

Frontend (phase 19 touched, per `git log --grep="19-"`):
- frontend/src/components/DesktopView.tsx
- frontend/src/components/DesktopSetupSheet.tsx
- frontend/src/components/SettingsSheet.tsx (Desktop group, Security copy)
- frontend/src/components/RegenerateKeyModal.tsx (default body copy)
- frontend/src/lib/desktopClient.ts
- frontend/src/lib/webMode.ts
- frontend/src/main.tsx
- frontend/src/style.css (desktop, ts-status, dialog--setup, settings-row buttons and switch rules, token blocks)
- frontend/index.html (viewport meta)

Static pages and Go strings that carry UI copy:
- web/desktop-off.html
- web/desktop-link-invalid.html
- web/service-retry.html
- tray_common.go (Desktop tray lines)
- app.go (lapse notification call)
- internal/desktop/watchdog.go (lapse body text)

Planning inputs:
- .planning/phases/19-unattended-desktop-viewing/19-UI-SPEC.md
- .planning/phases/19-unattended-desktop-viewing/19-CONTEXT.md
- .planning/phases/19-unattended-desktop-viewing/19-01 through 19-17 SUMMARY.md (searched all 17 for deviations, known stubs, UI notes and open items; 19-06, 19-11, 19-13 and 19-15 read in the relevant sections)
- .planning/phases/19-unattended-desktop-viewing/deferred-items.md
