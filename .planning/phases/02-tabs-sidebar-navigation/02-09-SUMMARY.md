---
phase: 02-tabs-sidebar-navigation
plan: 09
subsystem: frontend-dialogs
status: complete
tags: [look-01, dialogs, sheets, focus-trap, share, funnel, testing]
requires: ["02-08"]
provides:
  - "useDialogFocus(ref, {onCancel, initialFocus, active}): initial focus, Tab/Shift+Tab wrap, Esc, focus return; only the topmost aria-modal dialog handles keys"
  - "One sheet style (.dialog-scrim / .dialog / .dialog__title/__body/__buttons / .button--primary|secondary|destructive) on tokens, under the 52px toolbar"
  - "New Session: Cancel / Create Session, inline create error, dialog stays open on failure"
  - "Regenerate / disable-warning / End Session, Quit, Join (Cancel / Join Session) and Share in the sheet style with no header ×"
  - "Share: 560px, title Share “<name>”, footer Done; NSSegmentedControl-style segments; Funnel risk panel on warning banner tokens; logic unchanged"
affects: ["02-10", "02-11", "02-12"]
actuals:
  tokens: 27400
  tasks: 2
  commits: 2
commits: 2
plan_head_before: dd9ced9366096fc31b247ab5c8b6a1f16adf25f2
plan_head_after: 5327709f5e53ac57a8524a70de85988ee1405619
tech-stack:
  added: []
  patterns:
    - "Dialog components put ref on the aria-modal element and mark buttons data-dialog-primary / data-dialog-cancel; useDialogFocus picks the initial target from those"
    - "The scrim has no click handler (modal); Esc is a window keydown listener gated on 'am I the last [aria-modal=true] in the DOM'"
    - "Async confirm: the dialog awaits onConfirm and shows a rejection inline; the parent closes only after success"
key-files:
  created:
    - frontend/src/lib/useDialogFocus.ts
    - frontend/src/lib/__tests__/useDialogFocus.test.tsx
  modified:
    - frontend/src/components/NewSessionModal.tsx
    - frontend/src/components/RegenerateKeyModal.tsx
    - frontend/src/components/QuitConfirmModal.tsx
    - frontend/src/components/RemoteJoinCodeModal.tsx
    - frontend/src/components/Share/SessionShareModal.tsx
    - frontend/src/components/Share/FunnelRiskPanel.tsx (button classes only)
    - frontend/src/components/SessionShare/shared.tsx
    - frontend/src/App.tsx (createTab rethrows; New Session onConfirm awaits it)
    - frontend/src/style.css
    - TESTING.md
    - "tests: NewSessionModal, RegenerateKeyModal, QuitConfirmModal, RemoteJoinCodeModal (both), SessionShareModal, style.hub.modal, style.hub, style.settings, style.tokens"
key-decisions:
  - "Quit keeps its labels and order (Keep Running, Quit GUI Only, Quit Everything); Quit Everything is the primary and Keep Running keeps initial focus, as before, because both other choices end the GUI."
  - "The Share dialog keeps its entering/open/exiting phase machine and now drives the shared .dialog motion; onAnimationEnd ignores animations that bubble up from children."
  - "useDialogFocus listens on window (not document) so the existing window-dispatched Esc tests keep working, and acts only when its dialog is the last aria-modal element in the DOM. That keeps a confirm opened over the 02-11 Settings sheet from closing both."
  - "Orphan CSS deleted (no consumer anywhere in src or e2e): the QR modal, the Phase 124 write opt-in / locked-row rules, the daemon button --kill/--web/--browse modifiers, and the Hub overlay / close-button rules that only the Share dialog used."
  - "LinkConfirmPopover stays a click-point popover; only its CSS moved to the light/dark tokens (orchestrator note: every modal must follow Light/Dark)."
requirements-completed: [LOOK-01]
---

# Phase 2 Plan 09: Dialogs and Sheets Summary

**Every dialog is now a light/dark macOS-style sheet under the toolbar, with no header ×, the buttons bottom-right and the primary on the accent. Focus is trapped by one `useDialogFocus` hook (Esc cancels, a scrim click does nothing, focus returns to the invoker). The Share dialog closes with a footer Done, and its Funnel consent gate is untouched.**

## Performance

- Started: 2026-11-15T15:48Z (local)
- Completed: 2026-11-15T16:10Z (about 25 min)
- Tasks: 2 of 2
- Files: 22 changed (+885 / -1068)

## Task Commits

1. **Task 1 (tracer): New Session sheet, focus hook, shared dialog CSS, inline create error**: `791d44c0`
2. **Task 2: Regenerate/End Session/disable-warning, Quit, Join and Share in the sheet style; TESTING.md**: `5327709f`

Tracer gate (Task 1): auto mode is off and human_verify_mode is end-of-phase. I re-ran Task 1's `<verify>` (tsc, the two targeted files, then the full suite: 92 files, 1237 tests). It passed before Task 2 started.

## What changed

- **`useDialogFocus`**: initial focus follows `first-input` (else primary, else Cancel), `primary` or `cancel`. Tab and Shift+Tab wrap and skip `tabindex=-1`, so the Share segments' roving tabindex still works. Tab pressed from outside the dialog moves focus back in. Esc calls the latest `onCancel` without re-running the effect, and focus returns to the invoker on close or unmount.
- **Sheet CSS** (tokens only): 480px by default and 560px for Share, `max-width: calc(100% - 48px)`, `max-height: calc(100vh - 104px)` with the body scrolling, radius 12, `--shadow-popover`, 24px padding. Opening uses a 150ms ease-out fade with `translateY(-8px)`, closing a 100ms fade, and both become `animation: none` under reduced motion.
- **New Session**: the dismiss button reads "Cancel" and Create Session gets initial focus. `createTab` now rethrows after logging. The dialog awaits it and on failure shows "Couldn't create the session: <detail>. Check that the folder exists, then try again." in `--color-error-text`. It closes only after a successful create.
- **RegenerateKeyModal** (Regenerate Signing Key, the disable-warning confirm, End Session): Cancel gets initial focus, and the confirm button is primary, or destructive when `destructive` is set. The inline onConfirm error is kept, and callers' props are unchanged.
- **Join**: the buttons are "Cancel" and "Join Session" ("Joining…"), and focus starts in the code input. The body copy and error mapping are unchanged.
- **Share**: title `Share “<name>”` is the `aria-labelledby` target, the "Close share modal" × became a footer "Done", and Done gets initial focus. These are unchanged: the share toggle, the segmented control's behaviour, link cards, the LAN password, the shell warning, Disconnect all viewers, the Funnel risk panel with its consent gate and hold-to-confirm, and the close-on-disconnect effect.
- **Segmented control**: 28px on `--color-segment-bg` with a 2px inset. The selected segment uses `--color-segment-selected` with `--shadow-tab-active` at 13/600. Main and sub labels now sit on one line, because two lines do not fit in 28px. The danger ring uses `--color-error-text`.
- **Funnel risk panel**: warning banner bg/fg, a 16px ExclamationTriangleIcon in a flex row, 16px padding, a native `<select>`, and dialog-style Keep local only / Enable internet share buttons.
- **`SessionShare/shared.tsx`**: the inline hex colours and styles became `.share-field__label` / `.share-field__value` (`--color-field`, ui-monospace 12px).
- **Also on tokens**: `.daemon-panel__btn` (the Copy/Open/QR buttons in the Share tabs), the `.session-share-panel__label/__url/__error` text and the link confirm popover.

## Verification

- `pnpm exec tsc --noEmit`: clean
- `pnpm exec vitest run`: 92 files, 1239 tests passed
- `pnpm exec playwright test --project=chromium`: 7 passed, 9 skipped (fixture-gated, same as before)
- `go build ./... && go vet ./... && go test -race -short ./...`: exit 0 (no Go changes)
- `bash tests/check-traceability-paths.sh`: OK, 87 paths
- `wails build -platform darwin/arm64 -tags wailsassets`: exit 0, built `build/bin/agenthub.app`. The generated wailsjs tree was restored afterwards.
- Acceptance greps: Task 1 gave 1 / 0 / 1 / 6 (needed ≥1). Task 2 gave × total 0, "Close share modal" 0, `>Done<` 1, `useDialogFocus(` total 4, hex in shared.tsx 0, `**M-64**` 1.
- T-02-32: `FunnelRiskPanel.test.tsx` and every SessionShareModal Funnel / consent / hold-to-confirm case passed with no edits. The only edits to `SessionShareModal.test.tsx` are the old header-strip and 520px-width pins plus the new sheet-shell block.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Style pins on deleted rules**
- `style.hub.modal.test.ts` pinned `.hub-modal-overlay` (z-index 200, keyframes, reduced motion) and `.hub-modal__close svg`. I retargeted these to the `.dialog-scrim` and dialog motion contract.
- `style.hub.test.ts` pinned the `.hub-modal__close:focus-visible` ring. It now pins `.button:focus-visible` on `--color-focus-ring`. A bare `#565f89` presence pin was dropped: the only rules holding that hex were the deleted orphans and the now-tokenized link popover.
- `style.settings.test.ts` "h3 divider" matched `border-top: 1px solid #292e42` from the orphan write-opt-in rule, not the h3 rule. It now reads the h3 rule itself, which uses `var(--hub-border)`.
- `style.tokens.test.ts`: the S-07 motion gate now checks `.dialog`. The dialog and Share content selectors joined the no-hex gate.
- Commit: 5327709f

**2. [Rule 2 - Critical] Orchestrator note: every modal follows Light/Dark**
- The Share tab buttons (`.daemon-panel__btn`), the link label/url/error text and the link confirm popover still had Tokyo-night hex, which renders badly in Light mode. They are on tokens now, and the no-hex gate covers them.
- Commit: 5327709f

**3. [Rule 1 - Bug] The Share phase machine could react to a child's animation**
- `onAnimationEnd` on the dialog also caught animationend events bubbling up from descendants, which could close the dialog early. It now ignores events whose target is not the dialog. A test covers this.
- Commit: 5327709f

**4. [Plan wording] Quit initial focus**
- The UI-SPEC rule is "primary, or Cancel for destructive confirms". Quit's other two choices both end the GUI, so it keeps its existing Cancel-first ("Keep Running") focus. Its last button, Quit Everything, is the primary, per the plan.

**5. [Scope] Close fade only on Share**
- The 100ms close fade plays on Share, which owns its close through the existing phase machine. The other dialogs are unmounted by their parent at once and close without a fade. Adding delayed unmount to each parent was more machinery than the cosmetic gain. This is logged in deferred-items.md.

**6. [Behaviour] Esc while the shell-warning banner is showing in Share**
- Before, the dialog's React `stopPropagation` swallowed Esc, so only the dialog closed. Now both window listeners run: the banner's cancel fires (`onShellWebShareCancel`, "keep sharing off") and the dialog closes. Closing the dialog without confirming leaves sharing off either way.

## Known Stubs

None.

## Threat Flags

None. No new network, auth or file surface. T-02-32: the Funnel logic was not touched and its tests are unchanged and green. T-02-33: the scrim has no click handler, the destructive confirms focus Cancel first, and Esc cancels.

## Needs VM check (native GUI, built app at build/bin/agenthub.app)

Run M-64 from TESTING.md in Light, then Dark. Quit any installed AgentHub and stop its daemon first.
1. New Session (⌘T), End Session… (tab menu), Regenerate Signing Key and the disable-warning confirm (Settings tab), and Quit (⌘Q with sessions running). Each dialog attaches under the toolbar, centred, with window-coloured surface, thin border, 12px corners and shadow over the scrim. **None stays dark in Light mode** (this is the 02-08 VM finding for End Session and New Session).
2. None of them has a header ×. Buttons sit bottom-right, the primary on the accent fill, and End Session's confirm is red.
3. Initial focus: Create Session (New Session) and Cancel / Keep Links / Keep Running on the confirms. Tab and Shift+Tab never leave the dialog, Esc cancels, a scrim click does nothing, and focus returns to the invoker.
4. Opening fades in with a slight downward slide. With Reduce motion on, there is no animation.
5. The link confirm popover (⌘-click an OSC 8 link whose label differs from its target) uses light colours in Light mode.
6. Join Remote Session and Share cannot be opened in the VM with Tailscale off. Join's restyle and focus are covered in `RemoteJoinCodeModal.test.tsx`. Share's title, Done footer, focus, scrim, Esc and close fade are covered in `SessionShareModal.test.tsx`, and its Funnel panel logic in `FunnelRiskPanel.test.tsx` and the SessionShareModal Funnel cases. If a Tailscale-connected Mac is available: Share is 560px, titled Share “name”, Done closes it, the segmented control reads like NSSegmentedControl, and the Funnel risk panel shows on the yellow warning colours with a triangle icon.

Human-check result: not run here. It is deferred to the orchestrator's VM pass, per the sequential_execution instructions.

## Self-Check: PASSED

- FOUND: frontend/src/lib/useDialogFocus.ts, frontend/src/lib/__tests__/useDialogFocus.test.tsx
- FOUND commits on HEAD: 791d44c0, 5327709f
