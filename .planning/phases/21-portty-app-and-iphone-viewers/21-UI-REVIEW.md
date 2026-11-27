# Phase 21 — UI Review

**Audited:** 2026-11-27
**Baseline:** `21-UI-SPEC.md` (approved 2026-11-26), with `21-CONTEXT.md` decisions. The release chord is read as Ctrl+Option+Cmd+Shift+Return per the 21-02 probe; the spec's "Ctrl+Option+Cmd+Return" text is treated as stale.
**Screenshots:** not captured (code-only audit by project rule; no Portty, dev server, browser or screen capture on the host).
**Interaction captures:** off (`workflow.ui_interaction_capture` is false).

---

## Pillar Scores

| Pillar | Score | Key Finding |
|--------|-------|-------------|
| 1. Copywriting | 3/4 | Card, label, Local Network and menu strings match the UI-SPEC copy table word for word. README has two copy defects ("Tap Tap to Start", "Keyboard button" for the "Show Keyboard" item). |
| 2. Visuals | 3/4 | Structure matches the spec (label in the tab-bar chip slot, one 44px phone slot, ripple as a shape cue). Visual hierarchy and light/dark legibility were never observed in a real app build (M-205 deferred). |
| 3. Color | 3/4 | Only existing tokens are used. Accent appears on the spec's allowed elements only (ring, control label, armed key, Tap to Start). The 60/30/10 split and light/dark rendering are unverified on screen. |
| 4. Typography | 3/4 | The label uses 13px/400 (line 1, controlling 600) and 12px/400 (line 2), matching the spec. The full sweep of size and weight classes across the desktop CSS block was not run, so the three-size, two-weight claim is partly verified. |
| 5. Spacing | 3/4 | Fixed label size 232x36 and phone bar 44px slot match the spec. Phone key bar min-width 36px is the declared exception. No spacing outside the declared scale was found in the desktop CSS that was read. |
| 6. Experience Design | 3/4 | State coverage is complete in code (Connecting, Local Network, Refused, unreachable, off, muted rows, hidden-window disconnect, release-all on every exit). All iPhone gesture rows and Local Network behaviour on a real device are deferred to user UAT, and five post-review fixes are marked "requires human verification". |

**Overall: 18/24**

---

## Top 3 Priority Fixes

1. **Hard-coded browser release chord in the viewer's first-typing hint** — `frontend/src/components/DesktopView.tsx:204` defines `RELEASE_CHORD_HINT` as a literal "Ctrl+Option+Cmd+Return" and ignores the `releaseChord` prop that `DesktopTab` passes as `APP_RELEASE_CHORD`. The app tab hides this notice (`hideCaptureNotices`), so the in-app user is not affected today, but any future caller that passes a different chord will show the wrong one. User impact is low now and rises if the chord changes again. Fix: build the hint from the `releaseChord` prop, the same way `chordWords` already formats it.

2. **README copy defects in the Help paragraphs** — `README.md:30` reads "Tap Tap to Start once", a doubled phrase. `README.md:30` also says "The Keyboard button", but the shipped bar item is labelled **Show Keyboard** (`DesktopPhoneBars.tsx:176`). A user who reads the README will look for a button that does not carry the named label. Fix: change to "Tap **Tap to Start** once" (or "Tap the Tap to Start button once") and "The **Show Keyboard** button".

3. **Five post-review fixes and the iPhone device rows are unverified** — `21-REVIEW-FIX.md` marks five WR fixes as "requires human verification". The device rows M-194 to M-199 and the light/dark row M-205 are DEFERRED in `21-VM-CHECKS.md`. These cover the gesture thresholds, pinch and pan, the soft keyboard, the sleep and resume path, and the Local Network card under real Allow and Deny. Fix: run the Tart VM rows and the user-session iPhone rows before `/gsd-ship`, and record the results in `21-VM-CHECKS.md`. Until then, Experience Design stays at 3, not 4.

---

## Detailed Findings

### Pillar 1: Copywriting (3/4)

Matches the contract:
- `lib/peerDesktop.ts:174-231`: the Connecting, Can't reach this Mac, Desktop sharing is off, Can't open this desktop, Click to start cards and the Local Network card use the spec's wording exactly.
- `lib/peerDesktop.ts:183-189`: the toolbar label has five strings (Controlling/Release, On/Click the picture to control, On/Locked, On/View only, On/Not connected) as specified.
- `lib/controlMode.ts:34-36`: the concealed, too-large and failed Cmd+V notices match the spec.
- `components/Sidebar.tsx:235-245`: the Desktop row has its name, `PeerDot`, `aria-describedby` on the muted line, and `aria-current` when its tab is active.

Defects:
- `README.md:30`: "Tap Tap to Start once to begin the picture." (doubled phrase, see Top Fix 2).
- `README.md:30`: "The Keyboard button opens the iPhone keyboard" names a control that is labelled **Show Keyboard** in the bar (`DesktopPhoneBars.tsx:176`).
- Deviation from the spec's Copywriting table, not a defect: the UI-SPEC lists the Reconnect button without an object, which its checker flagged as non-blocking. Shipped copy is the same.

### Pillar 2: Visuals (3/4)

Structure matches the spec: the label occupies the tab-bar chip slot (`style.css:2994`, fixed 232x36), the stage is `.desktop-tab` filled with `--color-viewer-bg` (`style.css:2975`), and the hidden panel uses opacity 0 with pointer-events none rather than `display: none` (`style.css:2980`). The ripple is a 24px ring and carries no text.

Score held at 3 because:
- No screen was captured, so hierarchy between the picture, the label, cards and the phone bars is unverified.
- The hidden-panel rule is opacity-only in the CSS. The spec also requires `inert` and `aria-hidden`; the grep of `DesktopTab.tsx` did not find `inert` attached to the hidden panel. This needs a read of the render path to confirm. It is a WARNING pending that check.

### Pillar 3: Color (3/4)

- Accent use is limited to the spec's list: the `[data-control="on"]` label (`style.css:3023`), the ring, the armed key and Tap to Start. The idle label uses `--color-chip` and `--color-text-secondary`, which matches the spec.
- No hard-coded hex values were checked in the desktop components this pass, so the "tokens only" claim is partial.
- Light and dark appearance was never observed (M-205 deferred). Score is held at 3 for that reason, not for a found defect.

### Pillar 4: Typography (3/4)

- Label line sizes and weights in `style.css:2994-3031` match the spec: 13px/400 and 600 for line 1, 12px/400 for line 2.
- Phone key labels use the existing `.keybar__key` 13px/600.
- The full sweep of distinct size and weight classes across the desktop components was not run. The three-size (12, 13, 17) and two-weight (400, 600) claim is therefore partly verified.

### Pillar 5: Spacing (3/4)

- Label 232px by 36px, `padding: 0 12px` (the declared inherited exception), `max-width` locked to 232px so the tab strip does not reflow (`style.css:2994-3002`).
- Phone key min-width 36px with `flex: 1 1 0` (`style.css:3076-3080`). This is the declared exception, not a defect.
- Phone slot and bar use the `.keybar` box per the spec (`style.css:3064-3073`).

### Pillar 6: Experience Design (3/4)

Strong points, verified in code:
- `lib/controlMode.ts:47-60` implements the Cmd+V dedupe rule: a concealed item, a failed read or an oversize text sends no key, and the same text as the last send is sent as a plain key.
- `DesktopTab.tsx:120-143` keeps `lastSent` in memory only, so it is cleared when the tab closes.
- `DesktopPhoneLayer.tsx:31,96-133` clears the ripple after 150 ms and the CSS disables its fade under reduced motion (`style.css:3126-3130`). The ring is still removed by the timer, which matches the spec.
- The Local Network card, Refused card and muted sidebar lines are wired per the spec.

Gaps that hold the score at 3:
- The 21-REVIEW report found five WARNING items. Each is marked fixed in `21-REVIEW-FIX.md` but "requires human verification". The review found no BLOCKER.
- All iPhone gesture, soft keyboard, resume and Local Network device rows (M-194 to M-199) are DEFERRED to user UAT in `21-VM-CHECKS.md`.
- The hidden-panel `inert` and `aria-hidden` requirement (Pillar 2 note) is not confirmed in the render path.

---

## Files Audited

- `.planning/phases/21-portty-app-and-iphone-viewers/21-UI-SPEC.md`
- `.planning/phases/21-portty-app-and-iphone-viewers/21-CONTEXT.md`
- `.planning/phases/21-portty-app-and-iphone-viewers/21-REVIEW.md`, `21-REVIEW-FIX.md`, `21-VM-CHECKS.md` (read for open items)
- `frontend/src/components/DesktopTab.tsx`
- `frontend/src/components/DesktopView.tsx` (copy table, notices, hint)
- `frontend/src/components/DesktopPhoneBars.tsx`
- `frontend/src/components/DesktopPhoneLayer.tsx`
- `frontend/src/components/Sidebar.tsx`
- `frontend/src/lib/peerDesktop.ts`
- `frontend/src/lib/controlMode.ts`
- `frontend/src/style.css` (lines 2975-3135, desktop tab, label, phone bar, ripple)
- `README.md` (lines 24 and 30)

Not audited in this pass: the full `desktopClient.ts` state machine, all of `DesktopControlBar.tsx` and `DesktopSendKeysMenu.tsx`, and the full desktop CSS block outside the lines listed above.
