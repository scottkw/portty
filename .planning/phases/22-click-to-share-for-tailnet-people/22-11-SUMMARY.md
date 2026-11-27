---
phase: 22-click-to-share-for-tailnet-people
plan: 11
subsystem: frontend
tags: [react, wails, click-to-share, admit, banners, focus, security]
status: complete
requires:
  - phase: 22-09
    provides: AdmitDesktopGuest and DenyDesktopGuest bindings (never throw), desktopshare:* events, Quit GUI Only ends the share
  - phase: 22-10
    provides: useDesktopShare store, ShareDesktopSheet mounted in App, .admit* CSS
provides:
  - AdmitSheet (ADMIT_GUARD_MS = 600), the topmost Admit prompt
  - useDialogFocus initialFocus 'dialog'
  - ShareBanner (secure-input and stopped variants)
  - QuitConfirmModal sharing prop and sentence
affects: [22-12, 22-13]
key-files:
  created:
    - frontend/src/components/AdmitSheet.tsx
    - frontend/src/components/ShareBanner.tsx
    - frontend/src/components/__tests__/AdmitSheet.test.tsx
    - frontend/src/components/__tests__/ShareBanner.test.tsx
  modified:
    - frontend/src/lib/useDialogFocus.ts
    - frontend/src/lib/desktopShare.ts
    - frontend/src/App.tsx
    - frontend/src/components/QuitConfirmModal.tsx
    - frontend/src/components/__tests__/QuitConfirmModal.test.tsx
    - frontend/src/lib/__tests__/useDialogFocus.test.tsx
decisions:
  - "The guard is derived (unlocked key !== `${requestId}:${run}`), not a flag set in an effect, so the render that shows a new person is already disabled; no window where an old click lands on the new name."
  - "A successful answer keeps that person's buttons off (answered id) until the state line moves the sheet on; no_request counts as success (gone), any other error code or a thrown binding shows the one alert and restarts the guard."
  - "Oldest pending is chosen by since then id (the Watching list's order) rather than trusting array order."
  - "The stopped banner's dismiss is share.closeSheet (it clears stoppedNotice); the sheet cannot be open under a click on the banner, so no extra store verb was added."
actuals:
  tokens: 9400
  tasks: 3
  commits: 3
plan_head_before: 346909678cc6b46d277128c5d4b321bc6383d72c
plan_head_after: 204586ba9a5a37bd8a090cb3b45fb20d7f35a782
---

# Phase 22 Plan 11: Admit sheet, sharer banners and the quit sentence Summary

Admit prompt that names a guest by login name and can only be answered by a deliberate click (dialog focus, inert Esc, no default button, 600 ms guard per person), plus the Secure Event Input and Share stopped banners and the quit warning.

## What was built

- **useDialogFocus `'dialog'` mode.** Focuses the dialog element (tabindex -1). The element itself counts as outside for Tab handling, so Tab goes to the first button (Deny) and Shift+Tab wraps to the last (Admit) instead of leaking out of the modal. Existing modes and default unchanged.
- **AdmitSheet.** Renders null with nothing pending; otherwise the oldest request. Title, 17/600 name in `bdi`, Login name and Device rows (device has `title`), the two paragraphs, queue line, `role="alert"` error, Deny then Admit. Deny carries `data-dialog-cancel` for Tab order only; neither button has `data-dialog-primary` or the primary class. `onCancel` is a no-op. Names are sanitized again (Cc and Cf dropped). Display name empty, invisible-only, or equal to the login makes the login the bold line and omits the Login name row. Mounted in App as the last element, web mode excluded.
- **ShareBanner.** secure-input: warning, `ShieldExclamationIcon`, no action or dismiss. stopped: info, `InformationCircleIcon`, Share Desktop… and a dismiss. Strings from shareDesktopCopy.ts. App shows secure-input while `secureInput && anyController`, stopped while `stoppedNotice`, and both triggers were added to the banner-stack gating expression (the INVARIANT comment's reason).
- **QuitConfirmModal.** `sharing` prop (App passes `share.state.active`) renders `QUIT_WARNING` as a Caption paragraph directly under the session count.

## Precondition record

Both gates passed: `21-08-SUMMARY.md` and `22-10-SUMMARY.md` exist. `git log --oneline 07ad7d21..HEAD -- App.tsx useDialogFocus.ts QuitConfirmModal.tsx` listed, all read before editing: c8fefe80 and b40367f3 (22-10 Share Desktop sheet and Settings Row), 7b60fe5f (quick-261127-fb9 Phase 21 tidy), 307277be (21 WR-05), 2e9f88c4, 3c22c9f7, 0b4978a1 (21-08 Desktop tab wiring).

## Tasks and commits

| Task | Commit | Notes |
|------|--------|-------|
| 1 tracer: knock then deliberate admit | be0c0c5e | useDialogFocus dialog mode, AdmitSheet, App mount, tracer test; tsc, vitest and brand gate clean before expanding |
| 2 stray keys, guard per person, queue, name shapes | 18575629 | 17 more AdmitSheet cases (18 total), 2 useDialogFocus cases |
| 3 banners and quit sentence | 204586ba | ShareBanner, App wiring, QuitConfirmModal, store fix, 7 + 2 tests |

Full frontend suite: 147 files, 2680 tests pass. `tsc --noEmit` and `tests/check-brand.sh` clean.

Mutation check: dropping the guard from `disabled` fails 4 AdmitSheet cases.

## Deviations from Plan

**1. [Rule 1 - Bug] useDesktopShare kept `stoppedNotice` true after a new share started.**
- **Found during:** Task 3 (the banner must clear "when a new share starts").
- **Issue:** The notice was cleared only by openSheet and closeSheet. A new active state line left it true, so a later inactive line (without a `stopped` event) would have resurrected the old banner.
- **Fix:** the `desktopshare:state` handler clears it when `active` is true. Two lines in `frontend/src/lib/desktopShare.ts` (not in the plan's file list). Covered by the "new active share clears it" test.
- **Commit:** 204586ba

Otherwise none. The error line uses `.viewer-row__error` rather than `.admit__paragraph` because `.admit__paragraph` is declared later in style.css and would override the error colour; no CSS was edited.

## Auth gates

None.

## Known Stubs

None.

## Threat Flags

None. T-22-36 (stray key or click-through) and T-22-37 (look-alike names) mitigated as planned: dialog focus, inert Esc, no default button, guard per person, aria-disabled during answers; login always shown and wrapped, bdi, sanitized again. T-22-SC: no packages added.

## For 22-13 (TESTING.md, not edited here)

New test files for Section 2 and the Section 4 traceability map (DSHR-04, DSHR-06, DCTL-04):
- `frontend/src/components/__tests__/AdmitSheet.test.tsx` (DSHR-04)
- `frontend/src/components/__tests__/ShareBanner.test.tsx` (DSHR-06, DCTL-04)

Extended existing files: `QuitConfirmModal.test.tsx`, `frontend/src/lib/__tests__/useDialogFocus.test.tsx`.

Manual-row candidates (Section 5, for 22-12):
- M: with a real terminal underneath, a stray Return, Space or Esc during a knock answers nobody, and a click within about 600 ms of the sheet appearing does nothing.
- M: a made-up right-to-left and look-alike display name renders correctly on the real Admit sheet, login name visible.
- M: the Admit sheet sits above the Share Desktop and Settings sheets and the Portty window comes forward on a knock.
- M: Secure Event Input banner appears with a password field focused while a guest holds control, and clears after.
- M: Share stopped banner after killing the daemon during a share.

## Self-Check: PASSED

Files exist (AdmitSheet.tsx, ShareBanner.tsx, both test files); commits be0c0c5e, 18575629, 204586ba are ancestors of HEAD. STATE.md and ROADMAP.md untouched.
