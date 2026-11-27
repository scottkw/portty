# Phase 22 — UI Review

**Audited:** 2026-11-27
**Baseline:** `22-UI-SPEC.md` with `22-CONTEXT.md` decisions. Phase 23 changes to shared strings (the "and sound" clauses in `SETTINGS_ROW_DESCRIPTION`, `SHARE_NOT_SHARING_DESCRIPTION`, `SHARE_FOOTNOTE`, `ADMIT_PARAGRAPH_1`, the locked chip, the Sound item, quality and unlock) are out of scope and not counted as deviations.
**Screenshots:** not captured (code-only audit by project rule; no Portty, dev server, browser or screen capture on the host). The tray icon PNG was opened as a file only.
**Interaction captures:** off (`workflow.ui_interaction_capture` is false). Interaction findings are code-derived, plus the VM observations recorded in `22-VM-CHECKS.md` (G-01 to G-11).

---

## Pillar Scores

| Pillar | Score | Key Finding |
|--------|-------|-------------|
| 1. Copywriting | 3/4 | Every string lives in `lib/shareDesktopCopy.ts` and matches the copy contract. The grant caption drops the bold name the spec asks for, and one string (`Unknown viewer`) is not in the contract. |
| 2. Visuals | 3/4 | Sheet, Admit sheet, viewer rows, badges and the tray glyph follow the spec structure. The Secure Event Input banner is hidden behind the Share Desktop sheet, the place where the sharer grants control (seen in G-06). |
| 3. Color | 3/4 | Tokens only, no hex in the Phase 22 components or CSS. Accent is limited to the switches, Start Sharing and Done. Light and dark rendering never observed. |
| 4. Typography | 3/4 | Only 12, 13 and 17px, weights 400 and 600, as declared. The `<bdi>` name in the grant caption is not bold, against the spec's Line 4 rule. |
| 5. Spacing | 3/4 | Values sit on the 4px scale and the declared dimensions match (badge 24px, Admit buttons 96px, identity block 16px). Two new 12px gaps go beyond the "12px only to match existing surfaces" exception. |
| 6. Experience Design | 3/4 | Admit guard, inert Esc, dialog focus, no-confirm actions and grant hold logic are correct. The sheet renders an empty body while checking, Getting a link… is not announced, the Status row's live region is created with its text, and all 15 UAT items are still pending. |

**Overall: 18/24**

---

## Top 3 Priority Fixes

1. **Secure Event Input banner is invisible while the Share Desktop sheet is open** (WARNING) — the sharer turns on Allow control inside the sheet, which is exactly when the "someone may type at the same time as you" warning matters, but `.dialog-scrim` covers `.banner-stack` and the modal makes the rest of the window inert (G-06 notes: the first run "saw nothing: the tree showed only the sheet, not the banner behind it"). — Render the same `ShareBanner kind="secure-input"` inside the sheet body (above the Watching list) when `secureInput && anyController`, or show a one-line Caption warning in the Watching group. Keep the window banner for when the sheet is closed.
2. **Empty sheet body during `checking`, and the `starting` text is not announced** (WARNING) — `ShareDesktopSheet.tsx:256-302` renders nothing in the body while `phase === 'checking'` (two IPC calls), so the sheet opens as a title over two disabled buttons; `Getting a link…` (`:292`) is a plain `<p>` with no live region, so VoiceOver users get no feedback after the one-click start. — Show `SHARE_STARTING` for both `checking` and `starting`, and wrap the body's state line in a persistent `role="status"` container that exists from first render.
3. **Status row live region is mounted together with its text** (WARNING) — `ShareDesktopSheet.tsx:269-275` only renders the row when `status` is set, and the `aria-live` span is created with the text already in it, so screen readers usually do not announce "Needs permission" or "Waiting for Tailscale" when it first appears. The spec makes the row a polite live region so the sharer "finds out first". — Keep an always-mounted `aria-live="polite"` element in the Link group and change only its text; render the visible row as now.

Further WARNINGs (fix recommended):

4. Grant caption name not bold (`ViewerList.tsx:168`). Spec Line 4: "`<name>` is the bold name, in `<bdi>`". Wrap in `<strong>` or give `.viewer-row__caption bdi` `font-weight: 600`.
5. Double announcement on join (`ViewerList.tsx:99` and `:176`). The whole `.settings-group` is `aria-live="polite"`, so a new row's full contents (name, badge, both switch labels, caption) are read, then the hidden status says "<name> joined". Badge flips on Allow control are also read as a list change. The spec asks for both regions, but in practice the list region is noisy. Consider `aria-relevant="additions removals"` on the list, or rely on the status line alone (a spec change, flag to the user).
6. Admit inline error reuses `.viewer-row__error` (`AdmitSheet.tsx:118`) instead of the dialog's own `.dialog__error`, which the Share sheet uses (`ShareDesktopSheet.tsx:299`). Same colour, but two classes for one role in two dialogs drift apart over time.
7. `.admit:focus { outline: none; }` (`style.css:3422`). Focus on the dialog is required by the spec, but nothing shows where focus is until the first Tab. Acceptable under the spec; note for a native-look pass (macOS sheets also show no ring, so this is consistent with the native look).

---

## Detailed Findings

### Pillar 1: Copywriting (3/4)

Matches, verified against the Copywriting Contract and Surfaces 1 to 7:
- `lib/shareDesktopCopy.ts:7-137` holds every sharer and guest string; no component spells one inline (grep of the components found only `copy.*` and imported constants).
- Sheet: Share Desktop, Link description, Copy Link / Copied!, Watching, the empty line, View only / Controlling / Own account, Allow control, Allow clipboard, Disconnect, Your own device, Stop Sharing, Done, Not Now, Start Sharing, the stopped notice and the four error forms (`:17-56`) match the spec word for word (Phase 23 sound clauses excepted).
- Status row texts are the Phase 19 words verbatim (`:59-64`).
- Admit: title, Login name, Device, paragraph 2, queue line with singular and plural, Deny, Admit (`:68-78`).
- Tray, notifications, banners (`:81-97`), guest cards and chips (`:100-131`) and the two static pages (`web/desktop-guest-link-invalid.html`, `web/desktop-guest-refused.html`) match Surface 6 and 7.
- Quit warning sentence (`QuitConfirmModal.tsx:53`) and the Security-pane sentence (`:13-14`) match.
- README line 26 describes the feature with the shipped labels (Share Desktop…, Stop Sharing, Can't reach this Mac) and the tcp/udp 7444 access-rules note.

Defects:
- `ViewerList.tsx:168`: the grant caption name is not bold (see Pillar 4).
- `shareDesktopCopy.ts:38`: `UNNAMED_VIEWER = 'Unknown viewer'` is not in the contract. It only shows when the daemon sends no display, login or device name, which the Names rule says cannot happen for a guest (login always present). Harmless fallback; add it to the spec or drop it.
- `AdmitSheet.tsx:102`: when a display name is shown but the login name is empty, no login line renders. The spec says the login name is always visible. Not reachable if the daemon always sends a login, but the UI does not enforce the rule.

### Pillar 2: Visuals (3/4)

Structure matches:
- Share sheet is `.dialog.dialog--setup.dialog--share` (560px), Link row and Status row in one `.settings-group`, Watching header, list, footnote, errors, footer with Stop Sharing at the left edge (`ShareDesktopSheet.tsx:249-341`, `style.css:3280-3282`).
- Viewer row is four lines (name and badge, identity, controls, caption), Disconnect pushed to the right (`ViewerList.tsx:114-171`, `style.css:3288-3342`). Own-account rows have no switches and no caption.
- Badges pair an icon with words (`ViewerList.tsx:112-120`); `CursorArrowRaysIcon` for Controlling as specified.
- Admit sheet has the identity block with 17/600 name, label and value rows, two paragraphs, queue line, and two equal non-primary buttons (`AdmitSheet.tsx:98-139`).
- Tray glyph `assets/tray_icon_sharing@2x.png`: the Portty glyph with a filled dot in the lower right, a silhouette change as specified. G-08 compared crops of the status item and passed.
- Static pages reuse the `link-invalid.html` structure and `dashboard.css`.

Defects:
- Secure Event Input banner occluded by the sheet (Top Fix 1). Confirmed by G-06's first run.
- The `checking` phase renders an empty body (Top Fix 2): the first frame of the "one click" flow is a titled dialog with no content.
- Light and dark appearance, wrapping of long logins and the 560px sheet with several rows were never observed on screen (code-only audit; UAT pending).

### Pillar 3: Color (3/4)

- No hex or `rgb(` in the Phase 22 components or in `style.css:3258-3430`. Every colour is an existing token: `--color-group`, `--color-group-border`, `--color-chip`, `--color-control-border`, `--color-text`, `--color-text-secondary`, `--color-error-text`, `--opacity-disabled`.
- Accent use (2 elements, both in the reserved list): `.button--primary` on Start Sharing and Done (`ShareDesktopSheet.tsx:315, 331`), and the `Switch` track when on. Admit and Deny are both `.button--secondary` with no `data-dialog-primary` (`AdmitSheet.tsx:122-138`), as required.
- Destructive colour not used for Stop Sharing, Deny or Disconnect, as specified.
- Status dots always have words beside them (`ShareDesktopSheet.tsx:273-275`, `SettingsSheet.tsx:199-201`).
- Score held at 3: light/dark and the 60/30/10 balance in the real window are unverified, as in Phase 21.

### Pillar 4: Typography (3/4)

Distribution in the Phase 22 CSS block (`style.css:3258-3430`):
- Sizes: 12px (watching header, identity line, caption, error, badge, fact label, Admit paragraphs), 13px (viewer name, fact value), 17px (Admit name). Three sizes, as declared.
- Weights: 600 (header, viewer name, badge, Admit name), 400 elsewhere. Two weights, as declared.
- Line heights 16, 18, 22 match the Caption, Body and Display roles.
- Login wraps with `overflow-wrap: anywhere`; device ends in an ellipsis with the full text in `title` (`style.css:3321-3330`, `ViewerList.tsx:127`).

Defect:
- `ViewerList.tsx:168`: the name inside the grant caption renders at 400. The spec's Line 4 calls for the bold name.

### Pillar 5: Spacing (3/4)

- Row rhythm: `.viewer-row` gap 8px between lines (`style.css:3288-3292`), switch pair gap 16px (`:3331-3333`), switch label gap 8px (`:3334-3338`), badge padding `0 8px`, gap 4px, min-height 24px, radius 6px (`:3356-3377`). All match the Component dimensions table.
- Admit identity block padding 16px, rows 8px apart, radius 8px (`:3381-3389`); buttons `min-width: 96px` (`:3425-3427`).
- Copy Link keeps `min-width: 76px` from the Phase 19 rule (`:947`), so "Copied!" does not shift it.
- Watching header margin `0 0 4px`, group offset 16px: on the scale.

Defects (minor):
- `style.css:3293-3298`: `.viewer-row__head` uses `gap: 12px`. The spec allows 12px only to match `.button`, `.settings-row` and the Phase 19 chip; a name-to-badge gap is a new use. Use 8px.
- `style.css:3397-3401`: `.admit__fact` `gap: 12px`, same issue. The fixed `min-width: 80px` label column is fine (multiple of 4) but is not in the dimensions table.

### Pillar 6: Experience Design (3/4)

Strong points, verified in code:
- Admit click guard: 600 ms per person and restarted after a failed answer (`AdmitSheet.tsx:40-60`); buttons `aria-disabled` and dimmed during the guard and while an answer is in flight; an answered id stays disabled until the state moves on (`:44, 60, 75`).
- Admit focus on the dialog (`tabIndex={-1}`, `initialFocus: 'dialog'`) and Esc inert (`:49, 94`). Render order: `AdmitSheet` is last in `App.tsx:1647`, so it is topmost. G-01 drove Admit through the AX tree and passed.
- One-click start after the CT acknowledgement, first-time path with Start Sharing as the consent (`ShareDesktopSheet.tsx:168-197`); share-stopped and tray-stop transitions drop to Not sharing without restarting (`:110-128`).
- Errors map to the spec's slots: Tailscale off and userspace disable Start Sharing with `aria-describedby` the reason; HTTPS off and other failures are inline `role="alert"` (`:153-158, 294-299, 333-334`).
- The link lives in a ref and is never rendered (`:75, 162`).
- Viewer actions act at once with no confirm; per-row busy state, per-row errors, `no_viewer` treated as success, and a requested grant held only while the daemon still reports the old value (`ViewerList.tsx:78-108`). G-02 and G-03 passed.
- Accessible names: each row is a named `role="group"`, switches are labelled by label plus name and described by the caption, Disconnect has `aria-label="Disconnect <name>"` (`:114, 139-140, 159`).
- Settings Row 6 has all three states including `aria-disabled` with the reason when Tailscale is off (`SettingsSheet.tsx:189-232`). Quit confirm names the share (`QuitConfirmModal.tsx:53`).

Gaps that hold the score at 3:
- Secure Event Input banner hidden behind the sheet (Top Fix 1).
- Empty `checking` body and unannounced `starting` text (Top Fix 2).
- Status row live region created with its text (Top Fix 3).
- Double announcements from the list-wide live region (Fix 5). Also `announcement` is never cleared (`ViewerList.tsx:72`), so an identical message twice in a row is not re-announced.
- `ShareDesktopSheet.tsx:151` and `:222` await `StartDesktopShare` / `StopDesktopShare` without a `.catch`. A rejected Wails call leaves the sheet stuck in `starting` (both buttons disabled, no Esc path blocked but no message) or `stopping`. Other calls in the same file catch. Wrap both and map the rejection to the existing error strings.
- `22-UAT.md` is `status: testing` with all 15 items `[pending]`. G-04 (one stop took 6.9 s to drop a guest) and G-11 are PARTIAL in `22-VM-CHECKS.md`. No visual or interaction check has been done by a person.

---

## Files Audited

- `.planning/phases/22-click-to-share-for-tailnet-people/22-UI-SPEC.md`
- `.planning/phases/22-click-to-share-for-tailnet-people/22-VM-CHECKS.md` (G-01 to G-11), `22-UAT.md` (status)
- `.planning/phases/21-portty-app-and-iphone-viewers/21-UI-REVIEW.md` (format model)
- `frontend/src/lib/shareDesktopCopy.ts`
- `frontend/src/components/ShareDesktopSheet.tsx`
- `frontend/src/components/AdmitSheet.tsx`
- `frontend/src/components/ViewerList.tsx`
- `frontend/src/components/ShareBanner.tsx`
- `frontend/src/components/Switch.tsx`
- `frontend/src/components/SettingsRow.tsx`
- `frontend/src/components/SettingsSheet.tsx` (Row 6, lines 189-232)
- `frontend/src/components/QuitConfirmModal.tsx` (share line)
- `frontend/src/components/DesktopView.tsx` (guest card table, lines 141-164, 620-629)
- `frontend/src/App.tsx` (banner, sheet and Admit wiring)
- `frontend/src/lib/desktopShare.ts` (sheet and notice state), `frontend/src/lib/useDialogFocus.ts` (focus modes)
- `frontend/src/style.css` (lines 947, 3258-3430)
- `web/desktop-guest-link-invalid.html`, `web/desktop-guest-refused.html`
- `assets/tray_icon_sharing@2x.png` (opened as a file)
- `README.md` (line 26)

Not audited in this pass: the full guest-mode render path of `DesktopView.tsx`, `DesktopControlBar.tsx`, `DesktopSendKeysMenu.tsx`, `DesktopPhoneLayer.tsx`, `DesktopPhoneBars.tsx` beyond their Phase 22 guest gating, and the native tray menu code (`tray_objc_darwin.m`), which G-08 covered in the VM. The `git diff 5bdeb6eb~1 HEAD -- frontend/src/style.css` range also contains Phases 19 to 21 CSS; only the Phase 22 block was scored.

Registry audit: not applicable (no shadcn, no `components.json`).
