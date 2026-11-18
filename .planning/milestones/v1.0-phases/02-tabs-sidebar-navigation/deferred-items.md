# Phase 02 Deferred Items

Out-of-scope discoveries logged during plan execution (not fixed).

## From 02-04

- **Flaky test under full-suite load:** `frontend/src/components/__tests__/SessionShareModal.test.tsx` › "SM-02: .hub-share-modal__tabpanel is nested inside .hub-share-modal__body" failed once in a full `vitest run`, then passed alone and in three full runs in a row. It waits with a single `await tick()` before querying the tabpanel, which races under load. SessionShareModal was not touched by 02-04. Plan 02-08 or 02-12 (share dialog restyle) could make the test wait for the element instead.

**Resolved in 02-05:** the SM-02 flake above. SM-02, SM-03 and SM-05 now poll the async `cachedShare` seed with `vi.waitFor` (commit 4e587aae).
  status: acknowledged

## From 02-05

- `lib/remoteAdapter.ts` `adaptAllRemoteSessions` and `lib/remoteSession.ts` `findRemoteSession` (listed as orphans in 02-04-SUMMARY) were not in 02-05's delete list and are still there. Only their own tests use them.
- `frontend/src/assets/agenthub-title-logo.png` has no frontend importer but `scripts/verify-icons.sh` (SC5) checks that it exists, so it was kept. Delete it together with that check when Phase 3 rebrands the icons.
- `frontend/src/components/Hub/` now holds only the chat components and `WebShareSessionView` (plan 02-07). The Share dialog's classes are still named `hub-share-modal*` / `hub-modal-overlay` / `hub-modal__close`. A rename could go with the 02-12 restyle.
  status: acknowledged

## From 02-09

- `ShareSegmentedControl.tsx` still prefixes "⚠" onto the Full-access sub label (the SM-07 colorblind cue, pinned by `ShareSegmentedControl.test.tsx`). UI-SPEC "Removed UI" bans the glyph at phase end; plan 02-10 owns the remaining ⚠ glyphs.
- The Share dialog's "Share the session" and "Enable internet sharing" rows still use the old `.settings-panel__toggle-*` checkbox look. The UI-SPEC switch lands with the Settings sheet (02-11).
- `LinkConfirmPopover` is a click-point popover, not a sheet. It now uses the light/dark tokens but keeps its own Esc and Cancel-first focus, with no Tab trap (not in 02-09's dialog list).
- Only the Share dialog plays the 100ms close fade (it owns its close through the phase machine). The other dialogs are unmounted by their parent at once, so they close without a fade.

**Resolved in 02-10:** the "⚠" glyph in `ShareSegmentedControl.tsx` is now an `ExclamationTriangleIcon` (SM-07 cue kept: `is-danger` ring plus icon), and the duplicate "⚠" in the Funnel write-gate heading (`InternetFullAccessTab.tsx`, which already had the icon) is gone. No "⚠" remains in frontend source.
  status: acknowledged

## From 02-10

- `ShellWebShareBanner` (inside the Share dialog) still uses the old `.webgl-recovery-banner` / `--shell-warning` / `__dismiss` rules, which keep their TokyoNight hex. 02-10 kept those rules because that component still uses them. 02-12's no-hex sweep of `style.css` has to retoken or migrate them.
- `.banner-stack` still caps at `calc(3 * 53px)`, sized for the old 53px banners. The new banners are about 33px, so the cap now fits about four banners before it scrolls. Harmless; tidy it if the stack ever overflows.

**Resolved in 02-11:** the Share dialog's "Share the session" and "Enable internet sharing" rows now use the shared `Switch` (classes `share-toggle-row*`); the Funnel toggle still only opens the risk panel, and its committed-state "On/Off/N/A/Confirm…" text is unchanged.
  status: acknowledged

## From 02-11

- The plugin disclosure sub-options (Search defaults, Link click behavior) stay native checkboxes rather than switches; UI-SPEC lists them as native controls. The class names `settings-panel__details` and `settings-panel__description--italic` were kept (retokened) because PluginsSection tests pin them.
- ⌘, opens Settings even while another dialog (New Session, Share) is open. Both render; `useDialogFocus` gives keys to whichever is later in the DOM. Not reproduced or ruled out in the VM yet (see 02-11-SUMMARY "Needs VM check").
  status: acknowledged
