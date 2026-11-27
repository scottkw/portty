---
phase: 23-audio-adaptive-quality-unlock-and-release
plan: 07
subsystem: ui
tags: [react, unlock, password, desktop-viewer, webrtc-datachannel, privacy]
requires:
  - phase: 23-audio-adaptive-quality-unlock-and-release
    provides: "23-04 daemon unlock protocol (binary 0x01 on the input channel, caps unlock 1 on the own-account hello, unlock state messages, golden lines); 23-06 caps event"
  - phase: 20-remote-control
    provides: the notice slot, the Lock This Mac confirm, the paused input while locked
  - phase: 22-click-to-share-for-tailnet-people
    provides: the guest page (DesktopView guest prop) and its Locked chip
provides:
  - unlockSecret.ts (encodeUnlock, zeroBytes, UNLOCK_MSG_TYPE)
  - desktopClient unlock event and sendUnlock(bytes) on the input channel
  - DesktopUnlockPanel with the eleven UI-SPEC states
  - DesktopView wiring, success and blocked-input notices, guest Locked chip
  - the new Lock This Mac sentence
affects: [23-08 iPhone bar slot and keyboard rules, 23-10 VM rows U-01, 23-12 TESTING.md, 23-14 real-iPhone rows]
plan_head_before: b6bcc1f0542d2b4632a7adb16721f8885bfc204d
plan_head_after: 79d3be8e3746d1ebbb768c98ce01fdb71a07d97d
actuals:
  tokens: 17000
  tasks: 3
  commits: 3
tech-stack:
  added: []
  patterns:
    - "uncontrolled password input: the text lives in the DOM value only; React holds a boolean for empty or not"
    - "the viewer holds no counters: failures, waits and the pause are read from the daemon's last message and counted down locally for display only"
key-files:
  created:
    - frontend/src/lib/unlockSecret.ts
    - frontend/src/components/DesktopUnlockPanel.tsx
    - frontend/src/components/__tests__/DesktopUnlockPanel.test.tsx
    - frontend/src/lib/__tests__/unlockSecret.test.ts
    - frontend/src/components/__tests__/DesktopView.locked.test.tsx
  modified:
    - frontend/src/lib/desktopClient.ts
    - frontend/src/components/DesktopView.tsx
    - frontend/src/components/DesktopSendKeysMenu.tsx
    - frontend/src/lib/shareDesktopCopy.ts
    - frontend/src/style.css
    - frontend/src/lib/__tests__/desktopClient.test.ts
    - frontend/src/components/__tests__/DesktopSendKeysMenu.test.tsx
key-decisions:
  - "The panel has no onSuccess prop: success is read in DesktopView from the unlock event and the state flip, because the panel is already unmounted when a late by_portty message arrives"
  - "An 'unlocked' unlock message is never stored; the unlocking line stays until the state flips, so a stale unlocked cannot greet the next lock episode"
  - "No answer from the daemon within 8 s of a submit reads as Could not run (state 11), so the panel cannot sit on Unlocking forever"
  - "Reconnecting keeps the panel (inert) by remembering the last Locked value across the desk reset a reconnect causes"
requirements-completed: [UNLK-01]
duration: 55min
completed: 2026-11-27
status: complete
---

# Phase 23 Plan 07: Browser unlock panel and the rest of the Locked experience Summary

An own-account viewer on a locked Mac gets an Unlock This Mac panel in the browser and the app tab: the password goes out once as one binary message on the input data channel, the field is empty before the send returns, the byte array is zeroed after it, and every outcome the daemon reports has words and a next step.

## Precondition record

All five SUMMARY guards (20-07, 21-10, 22-07, 23-04, 23-06) passed at base; `grep -c unlock frontend/src/lib/desktopClient.ts` was 3 and the golden file has 11 `unlock-*` lines (the plan said at least 10).

`git log --oneline 1c55b159..HEAD -- DesktopView.tsx desktopClient.ts DesktopSendKeysMenu.tsx` at base: 6bc11b3f, 4004dae8, f82c4c87, 98c47df9, 64cdb045, d23310b1, 7ce37551, ad8ca062, 15a44f6e, 48f23917, 951908f4, b41055f0, bb55eae6, c292ab30, 6ca45021, e9e53a29, 2e9f88c4, 3c22c9f7, 0b4978a1, f97d700e.

## What was built

- **unlockSecret.ts**: `encodeUnlock` writes the 0x01 type byte and the UTF-8 text into one array (the encoder's intermediate buffer is zeroed); `zeroBytes` fills with zeros.
- **desktopClient.ts**: `unlock` event `{ state, failures, waitMs, reason, byPortty }` parsed from the signalling `unlock` message (only the ten enum states, bounded integers, and the three reasons are taken); `sendUnlock(bytes)` sends on the open input channel and returns false otherwise. It never touches the signalling socket.
- **DesktopUnlockPanel.tsx**: `role="group"` named by its heading; uncontrolled `type="password"` input (no form, no name, autocomplete off, `data-1p-ignore`, `data-lpignore`, `data-bwignore`, maxlength 256, enterkeyhint go, no show control). Submit (button, or Return outside an IME composition) reads the ref, encodes, empties the field, sends, zeroes. Eleven states from the daemon's message: Waiting, Typing, Unlocking, Trying once more, Wrong password (alert, plus the pause warning from the second failure), Rate limited (Try again in N s, `aria-live="off"` ticking number and one hidden polite line), Paused (in N min, in 1 min, in under 1 min), Refused (Caps Lock, unmappable), Inert (Reconnecting, the Accessibility line), Could not run. Field emptied on visibilitychange hidden, pagehide, unmount and Esc, not on blur. Focused only after a wrong password, never on appear.
- **DesktopView.tsx**: panel rendered as a sibling of the stage (never inside the key-listening element) only when `caps.unlock` and the Mac is Locked, under no card, not for a guest. After the state flips to unlocked: stage takes focus with the Phase 20 capture notice suppressed once, then `Mac unlocked. It will lock again after you disconnect.` (5 s) if a by_portty message came (before the flip, or within 1.5 s after), else plain `Mac unlocked` (3 s). A dropped connection is not an unlock. Guest Locked chip `Locked. Ask the person at the Mac to unlock it.`; a printable key or a press on the picture while Locked (Control on) sends nothing and shows the blocked-input notice once per lock episode, then at most every 10 s, in the own-account or guest form.
- **DesktopSendKeysMenu.tsx**: the Lock This Mac body is now `The Mac will lock. To unlock it from here you will need the Mac's password.`
- **style.css**: `.desktop-unlock*` with the Phase 20 popover metrics (320 px, 16 px padding, 10 px radius), 17/600 heading, 12/400 status lines with tabular numerals, 28 px pointer controls, 44 px and 16 px text on the phone, and a compact landscape row without the heading under 480 px of height. Existing tokens only.

## Verification

- `pnpm exec tsc --noEmit`: clean. Targeted vitest (panel, unlockSecret, desktopClient, DesktopView, DesktopView.sound, DesktopView.locked, DesktopSendKeysMenu, DesktopControlBar, guest): all pass. Full `vitest run` after the last change: 2768 tests, the one failure (Phase 22 guest test pinning the old guest Locked chip) was fixed in Task 3 and its file passes.
- `DesktopUnlockPanel.test.tsx` has 31 cases (gate was 14). Sentinel test: no `console.log/info/warn/error/debug` call and no page text or storage holds the typed sentinel after a submit.
- `pnpm --dir frontend build` succeeds. `GOTOOLCHAIN=go1.26.11 go test -tags=e2e -count=1 -run TestDesktopCSP_E2E -v ./internal/webserver/` printed `--- PASS: TestDesktopCSP_E2E` (not skipped). That test uses the synthetic loopback source and the Go tests' refusal of real capture and input; nothing touched the host's capture, input or tailnet.
- `bash tests/check-brand.sh` clean before each commit.
- Acceptance greps: `type="password"` count 1; no `<form` or `useState(...password)`; `sendUnlock` in desktopClient 2; the wrong-password sentence 1; the pause sentence 1; `.desktop-unlock` in style.css 21; `Remote unlock isn` gone from the menu; the new Lock sentence 1; `Ask the person at the Mac to unlock it.` in DesktopView 1.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Phase 22 guest test pinned the old guest Locked chip**
- **Found during:** Task 3 (full vitest run)
- **Issue:** `DesktopView.guest.test.tsx` expects `copy.GUEST_CHIP_LOCKED`, which was `Locked`; the new contract changes the guest chip.
- **Fix:** `GUEST_CHIP_LOCKED` in `shareDesktopCopy.ts` now carries the new sentence. DesktopView keeps the literal too, because the plan's acceptance grep counts the sentence in DesktopView.tsx.
- **Files modified:** frontend/src/lib/shareDesktopCopy.ts
- **Commit:** 79d3be8e

**2. [Rule 2 - Missing critical] No answer from the daemon would leave Unlocking… forever**
- **Found during:** Task 2
- **Issue:** the plan has the viewer wait for the daemon's messages only; a submit the daemon never answers (a build without unlock, a dead channel after the send) would leave the field inert.
- **Fix:** 8 s with no unlock message after a local submit shows Could not run. This is a display timeout, not a counter.
- **Commit:** 015435d1

### Plan interface differences

- `DesktopUnlockPanel` has no `onSuccess(byPortty)` prop (see key-decisions); DesktopView handles success.
- The DesktopView flow for success, reconnect-inert and the lock-episode reset (Task 2 behaviour) landed in the Task 1 commit, because the render gate and the flip effect live in the same component edit. Task 2's commit holds the panel states, CSS and tests.
- Hint for 23-08: the phone panel is rendered by the same gate with `phone` set and sits at `bottom: calc(44px + 8px + safe-area)`; the one-slot rule (empty bar slot while the field is focused), the Show Keyboard greying and the `useVisualViewportInset` selector are 23-08's and were not touched.

## Deferred (live checks, not run here)

No live browser, VM or device was used. These stay as rows for 23-10, 23-12 and 23-14:
- No browser save prompt or keychain suggestion for the Mac password in Chrome, Safari and iPhone Safari (opt-out attributes are best effort).
- The panel in light and dark appearance in a real browser and in the real Portty.app tab (look, 3 px focus ring, wrapping to at most 3 lines at 288 px).
- The panel above the iOS keyboard in portrait and landscape (23-08 owns the layout).
- The full unlock sequence on a real lock screen against the daemon (success, wrong password, retry, cooldown, pause), including that the stage takes focus without the capture notice in a real browser and in WKWebView.
- `TestDesktopCSP_E2E` only proves the page boots under the CSP against an unlocked synthetic source; it does not render the Unlock panel (the synthetic Mac is never locked), so CSP with the panel on screen is judged from the build and the VM rows.

## Test files for 23-12 (TESTING.md not edited)

New: `frontend/src/components/__tests__/DesktopUnlockPanel.test.tsx`, `frontend/src/lib/__tests__/unlockSecret.test.ts`, `frontend/src/components/__tests__/DesktopView.locked.test.tsx`.
Edited: `frontend/src/lib/__tests__/desktopClient.test.ts` (unlock describe, golden lines), `frontend/src/components/__tests__/DesktopSendKeysMenu.test.tsx` (new body). Traceability rows (UNLK-01): the three new files plus the two edited ones.

## Known Stubs

None.

## Threat Flags

None. T-23-19 (password kept or logged), T-23-20 (browser offers to save) and T-23-21 (panel shown to a guest) are mitigated as the plan's register says; T-23-20's residual is the browser rows above. No package added.

## Self-Check: PASSED

Files found: unlockSecret.ts, DesktopUnlockPanel.tsx, the three new test files. Commits found as ancestors of HEAD: 76901ea5, 015435d1, 79d3be8e.
