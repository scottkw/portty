# Phase 15 — UI Review

**Audited:** 2026-11-23
**Baseline:** 15-UI-SPEC.md (preservation contract, status: approved; approval line still reads "pending")
**Screenshots:** not captured (static code audit plus recorded VM evidence, per audit scope; no dev server started)
**Interaction captures:** off (`workflow.ui_interaction_capture` is false)

---

## Pillar Scores

| Pillar | Score | Key Finding |
|--------|-------|-------------|
| 1. Copywriting | 3/4 | Every contract-pinned string matches the code exactly; the contract's notification copy is wrong and "Nothing shared" is shipped but not pinned |
| 2. Visuals | 3/4 | No layout change in the touched surfaces; the ended-tab banner renders the info variant where the contract says warning |
| 3. Color | 3/4 | Touched components use only tokens; the contract's banner-variant row does not match the shipped variant |
| 4. Typography | 4/4 | 12px/16px and weights 400/600 match the contract on banner and sidebar muted lines |
| 5. Spacing | 4/4 | `.banner` padding 8px 16px, gap 8px, 16x16 icon match; no geometry diff in the phase |
| 6. Experience Design | 3/4 | Ended-tab, sidebar and update-banner flows match the contract and were VM-verified; a reachable host with nothing shared shows a "connected" dot next to "Nothing shared" |

**Overall: 20/24**

No BLOCKER was found. Every WARNING below is either a contract-document error or a minor state-signal issue. None breaks a user task. The phase's UI scope was preservation-only, so a high score here reflects that the surfaces were left alone and the copy is exact, not that the phase added polish.

---

## Top 3 Priority Fixes

1. **Contract notification copy does not match the shipped notification** — the UI-SPEC row says the macOS notification reads `Portty X is available`, but `app_update.go:137` posts title `Portty` and body `Version X is available.` — a future test or VM check written from the contract would fail or pass for the wrong reason — fix the contract row to the shipped title and body (do not change the code, the shipped text is the existing one).
2. **Contract banner variant for the ended-tab banner is wrong** — UI-SPEC Color table lists `SessionEndedBanner` under "Banner warning" (`--color-banner-warning-*`), but `SessionEndedBanner.tsx:80` builds `banner banner--info session-ended-banner`, and the `style.css` banner comment says "info: ... remote 'Session ended'" — the shipped info variant is the intended design, so the contract should say info — update the Color table row and the Interaction Contract note to `--color-banner-info-*`.
3. **Reachable host with nothing shared shows a "connected" state** — after the 15-07 change, `Sidebar.tsx:201-208` gives a reachable host the `sidebar__dot--on` class and the label "Reachable", with `Nothing shared` underneath (`Sidebar.tsx:218`). A user reads a green dot next to "no shares" as "connected and sharing" — the fix within scope is to keep the dot and aria label but pin `Nothing shared` in the contract, or (if the owner wants a visible difference) a muted dot for the reachable-empty case; this is a design call, so record it as a decision rather than a silent change.

---

## Detailed Findings

### Pillar 1: Copywriting (3/4)

Exact-match check of every contract string against the shipped source:

| Contract string | Shipped location | Match |
|---|---|---|
| `Session ended — the owner stopped this session.` | `SessionEndedBanner.tsx:129` (fallback branch) | Exact (em dash present) |
| `<host> stopped sharing this session.` / `This session is no longer shared.` | `SessionEndedBanner.tsx:128` | Exact |
| `Can't reach this Mac. Check that it's awake and on your tailnet.` | `Sidebar.tsx:17` (`PEER_UNREACHABLE`) | Exact |
| `Portty {latest} is available.` | `UpdateBanner.tsx:30` | Exact |
| `Download` / `Try Again` | `UpdateBanner.tsx:45` | Exact |

Findings:
- **WARNING:** The contract's "Update notification (macOS)" row says the text is `Portty X is available`. The shipped post (`app_update.go:137`) is title `Portty`, body `Version X is available.`. The contract row is inaccurate; the code is the existing text and should not change.
- **WARNING:** `Nothing shared` (`Sidebar.tsx:18`, `PEER_NOTHING_SHARED`) is now the user-visible line for the phase's sidebar fix (interaction row 2), but the Copywriting Contract does not pin it. The contract requires tests to pin exact strings, and this one has no contract row. It is pinned by `Sidebar.test.tsx:281-285`, so the gap is in the document only.
- No new copy was added in the phase. `git diff 3dcc612b HEAD` on `Sidebar.tsx`, `UpdateBanner.tsx`, `SessionEndedBanner.tsx` and `style.css` is empty.

### Pillar 2: Visuals (3/4)

- Focal point and hierarchy are unchanged: the banner stack still sits above the terminal row, and the sidebar peer group keeps its nesting (`.sidebar__rows--nested` 32px left padding).
- The update banner's `available` state is reachable from a notification click by clearing `dismissedUpdate` (`App.tsx:878`). `UpdateBanner` renders the info variant with `Download` (`UpdateBanner.tsx:40-47`). The `Downloading…` progress element only renders for `state === 'downloading'` (`UpdateBanner.tsx:24,36-38`), so no progress appears before Download is pressed. This matches interaction row 3.
- **WARNING:** The ended-tab banner's variant does not match the contract (see Pillar 3). The shipped info variant is consistent with the banner comment in `style.css`, so the contract is the document at fault.
- Icon-only dismiss buttons carry `aria-label="Dismiss"` (`SessionEndedBanner`, `UpdateBanner.tsx:47`). Acceptable.

### Pillar 3: Color (3/4)

- Grep for hardcoded colors in the touched components (`UpdateBanner.tsx`, `SessionEndedBanner.tsx`, `Sidebar.tsx`) returns none. Hex values appear only in the `--color-*` token definitions in `style.css` (lines 2074-2133), which is the intended place.
- Banner colors come from `--color-banner-info-*` (`style.css:1271-1273`) and `--color-banner-warning-*` (`style.css:1267-1269`), both with dark-mode values (lines 2130-2133).
- **WARNING:** UI-SPEC Color table row "Banner warning ... `SessionEndedBanner` (existing variant, unchanged)" is wrong. `SessionEndedBanner.tsx:80` uses `banner--info`. Also, the Interaction Contract and the UI Considerations table do not mention the variant at all, so the document is internally inconsistent with the shipped banner.
- Accent usage: the phase adds none (the contract's accent rule is satisfied by absence of change). The 60/30/10 split is unchanged because no touched surface changed its token use.

### Pillar 4: Typography (4/4)

- `.banner`: `font-size: 12px; line-height: 16px` (`style.css:1256-1266`). Message uses inherited weight 400. `.banner__action`: `font-weight: 600` (`style.css:1284-1292`). Two weights, as the contract requires.
- `.sidebar__muted`: `font-size: 12px; line-height: 16px`, inherits weight (`style.css:513-517`). Matches "unchanged from style.css".
- No new font roles or weights in the touched files; the phase diff touches no `style.css` lines.

### Pillar 5: Spacing (4/4)

- `.banner`: `padding: 8px 16px; gap: 8px` (`style.css:1256-1261`). `.banner__icon`: 16x16 (`style.css:1275-1278`). Both match the contract's declared scale (sm 8px, md 16px).
- `.sidebar__muted`: `padding: 4px 8px`, nested to `32px` left (`style.css:513-517,541-543`). These values are not in the contract's spacing table, but the contract instructs the executor not to edit them, and the phase diff does not touch them. Not a finding against the phase.
- The Tailwind spacing-class grep in the audit method does not apply: this codebase uses hand-written CSS in `style.css`, so the class count was not meaningful. The CSS values above are the evidence.

### Pillar 6: Experience Design (3/4)

State coverage per interaction contract row:

1. **Ended peer tab (row 1):** `peerConnectVerdict` (`frontend/src/lib/closeReasons.ts:32-37`) maps `no_such_session` and `not_sharing` to `stop: ended` only when `sawEnded` is true, and to `share-stopped` otherwise. The banner kind comes from the guest-socket close reason (`TerminalPanel.tsx:668-669,1416`), so a redial verdict does not switch an ended tab to "stopped sharing". The VM run (M-154, 2026-11-23, build v1.2.0-vm16) recorded `Session ended — the owner stopped this session.` after switch-back in 2 of 2 runs.
2. **Sidebar reachable host (row 2):** A refused share port now yields `MetaRefused`, which is reachable with an empty list (`internal/tailnet/sessions.go`, per 15-07). `Sidebar.tsx:216` renders `PEER_UNREACHABLE` only when `!g.reachable`, so a reachable host gets no can't-reach line. VM run recorded `Nothing shared` then drop-out, 0 can't-reach in 2 of 2 runs, and the pf-drop case showing the can't-reach line 16 s after the drop.
3. **Update notification click (row 3):** `update:show` clears `dismissedUpdate` (`App.tsx:878`) and does not start a download. VM run (M-152, vmpreview build, not notarized; the 2026-11-23 run was signed with Developer ID locally) recorded the window coming forward, `Portty 9.9.9 is available.` with `Download`, and no progress until Download was pressed.

Findings:
- **WARNING:** Reachable host with zero shares renders `sidebar__dot--on` and the label "Reachable" (`Sidebar.tsx:201,208`) together with "Nothing shared". That signals "connected and sharing" for a host that has nothing shared. This predates the phase (quick 261116-c9f introduced "Nothing shared"), but 15-07 made the state reachable more often, so the signal now appears in the exact case this phase targets. Decision needed: keep the green dot (current) or use a neutral dot for the reachable-empty case.
- **Note (no action):** The VM evidence cites build 7ca43451 and 1c8d3a09, while the audited tree is HEAD 73cc8d42. The UI-touching diff since the phase's first commit (`3dcc612b`) is limited to `TabBar.tsx`, `TerminalPanel.tsx` and `pathLinks.ts`, plus App and test files. The banner and sidebar copy and CSS are unchanged, so the M-152 and M-154 evidence still covers the shipped copy. Rerun the M-152 and M-154 checks if any later commit touches `closeReasons.ts`, `App.tsx` ended-tab wiring, or `Sidebar.tsx`.
- Empty, loading and error states for the touched surfaces were pre-existing and are unchanged. The phase added no destructive action and no new confirmation.
- Keyboard and focus: `Download`, `Try Again` and `Dismiss` remain native `<button type="button">` elements with the existing `:focus-visible` treatment (`style.css:1294-1330`), as the contract requires.

---

## Registry Safety

Not applicable. `components.json` is absent and the contract's Registry table lists none. The registry audit was skipped per the `<registry_audit>` rule.

---

## Files Audited

- `/Users/ken/dev/portty/.planning/phases/15-v1-2-hardening/15-UI-SPEC.md`
- `/Users/ken/dev/portty/.planning/phases/15-v1-2-hardening/15-CONTEXT.md`
- `/Users/ken/dev/portty/.planning/phases/15-v1-2-hardening/15-01-SUMMARY.md`, `15-04-SUMMARY.md`, `15-06-SUMMARY.md`, `15-07-SUMMARY.md`
- `/Users/ken/dev/portty/TESTING.md` (Category BF, M-152 to M-155)
- `/Users/ken/dev/portty/frontend/src/components/UpdateBanner.tsx`
- `/Users/ken/dev/portty/frontend/src/components/SessionEndedBanner.tsx`
- `/Users/ken/dev/portty/frontend/src/components/Sidebar.tsx`
- `/Users/ken/dev/portty/frontend/src/components/TerminalPanel.tsx` (ended-banner wiring, lines 379, 668-670, 1413-1420)
- `/Users/ken/dev/portty/frontend/src/App.tsx` (`update:show` at 878, `markPeerSessionEnded` at 1050-1056)
- `/Users/ken/dev/portty/frontend/src/lib/closeReasons.ts` (`peerConnectVerdict`)
- `/Users/ken/dev/portty/frontend/src/style.css` (banner 1253-1340, sidebar muted 513-545, token blocks 2074-2133)
- `/Users/ken/dev/portty/app_update.go` (notification text, line 137)
- `/Users/ken/dev/portty/frontend/src/components/__tests__/Sidebar.test.tsx` (pin for "Nothing shared", 281-285)
