---
phase: 22-click-to-share-for-tailnet-people
plan: 07
subsystem: desktop-guest-page
tags: [guest, admission, rights, lease, copy, desktopClient, DesktopView]
requires:
  - phase: 22-04
    provides: guest signalling wire (hello, admission, rights, close codes 4430 to 4433)
  - phase: 22-05
    provides: live rights message, lease message, signal-guest-v1.jsonl lease lines
  - phase: 20-remote-control
    provides: DesktopView stage and input controller, DesktopControlBar, DesktopSendKeysMenu, notice slot
  - phase: 21-portty-app-and-iphone-viewers
    provides: DesktopView optional props, clientOptions seam
provides:
  - "lib/shareDesktopCopy.ts: every Phase 22 UI string (sharer and guest) and the counted forms"
  - "desktopClient guest mode: guest option, defaultGuestMode, defaultGuestWsUrl, fetchGuestCheck, admission, lease and live rights events, guest_final status with GuestCloseReason"
  - "DesktopView guest prop: guest cards and chips, live grants, 2 s dwell, bar for a clipboard-only guest"
  - "DesktopControlBar and DesktopSendKeysMenu guest restrictions"
affects: [22-10, 22-11, 22-13]
key-files:
  created:
    - frontend/src/lib/shareDesktopCopy.ts
    - frontend/src/lib/__tests__/desktopClient.guest.test.ts
    - frontend/src/components/__tests__/DesktopView.guest.test.tsx
    - frontend/src/components/__tests__/DesktopControlBar.guest.test.tsx
  modified:
    - frontend/src/lib/desktopClient.ts
    - frontend/src/components/DesktopView.tsx
    - frontend/src/components/DesktopControlBar.tsx
    - frontend/src/components/DesktopSendKeysMenu.tsx
key-decisions:
  - "A guest's connection ends on a fixed reason through one new ClientStatus kind, guest_final with a GuestCloseReason, so the five final cards (denied, admit_timeout, revoked, share_ended, token_invalid) never retry and never touch the own-account token_rejected path"
  - "The page tells close reasons apart by close code only (SignalTransport.onClose hands up the code, not the reason string); the golden check asserts each code against the reason the Go file names"
  - "In guest mode the hello handler only records the hello (rights event, input capability) and the offer starts on admission admitted, through one beginOffer shared with the own-account path"
  - "Waiting at Admit cancels the 30 s unreachable timer; admitted arms it again, so Can't reach this Mac needs a socket that was up and no picture for about 30 s"
  - "Bar visibility is canControl or canClipboard (identical for own-account, where the clipboard right is only taken with control); BarState gained optional canControl and guest so barAvailability stays one pure table"
requirements-completed: [DSHR-03, DSHR-04, DSHR-05, DCTL-04]
status: complete
estimate:
  tokens: 100000
  tasks: 3
actuals:
  tokens: 45000
  tasks: 3
  commits: 3
plan_head_before: a0b538578378149cdc272671cbb04c9135463abe
plan_head_after: 7ce3755162caf2798cc48c2afe9397feaf0e5c0b
completed: 2026-11-27
---

# Phase 22 Plan 07: Guest page (browser half) Summary

The Phase 19/20 DesktopView and desktopClient now run in guest mode: wait at Admit, watch view-only, control by live grant, in the UI-SPEC's words, with every guest card and chip, and the single Phase 22 copy file.

## What was built

- **`shareDesktopCopy.ts`.** Every string of the 22-UI-SPEC Copywriting Contract and Surface 6, verbatim, as constants, with functions for the counted and named forms: `sharedWithPeople`, `traySharedLine`, `morePeopleWaiting`, `viewersJoined`, `viewersLeft`, `guestWaitingHeading`, `guestConnectingBody`, `controllingChip`, `grantCaption`, plus the notification, banner, error and static-page strings other plans will import.
- **`desktopClient.ts`.** `guest?: boolean` option (default `defaultGuestMode(location)`, true on a `/desktop/guest/` path); `defaultGuestWsUrl` (`wss://<host>/desktop/guest/ws?cap=`); `fetchGuestCheck` (`/desktop/guest/check`, 401, 410 and 403 are all "link gone"); events `admission`, `lease` and live `rights`; status `guest_final` with `GuestCloseReason`. The offer moved into `beginOffer`, called from the hello for an own-account viewer and from `admission admitted` for a guest, input channel still opened before `createOffer`. A lease name is cleaned again on the page (no Cc or Cf characters, whitespace collapsed, 128 characters) and kept only for state `other`.
- **`DesktopView.tsx`.** `guest?: boolean` prop (path default when absent; when absent the client options are passed through untouched). Cards: Waiting, Not admitted (denied and timeout, Reload Page on the timeout), You were disconnected, Sharing has ended (scrim over the frozen frame), This link no longer works, Connecting…, Can't reach this Mac (Reload Page). Chips by the spec's precedence; Streaming to N viewers never shows on a guest page. Live grants: control off calls `setControl(false)` (releases everything, drops the dot), hides the ring and shows View only; clipboard changes push the two 3 s notices. "Someone is using this Mac" is held for 2 s with `Date.now()` and a timer. The stage of a view-only guest has `tabindex -1`.
- **Bar.** `barAvailability` hides the picker for a guest and, without the control right, Send Keys, Swap, the segment and Full Screen; the Clipboard menu stays by its grant. A clipboard-only guest gets a bar holding Clipboard alone. `DesktopSendKeysMenu` takes `guest` and drops Lock Screen… and its separator. Own-account pages are unchanged (every existing test passes untouched).

## Task commits

1. Task 1 (tracer): `15a44f6e` - waits at Admit, then View only. Tracer gate (tsc plus the four vitest files) passed before expanding.
2. Task 2: `ad8ca062` - every card and chip, live grants, lease, close reasons, golden check.
3. Task 3: `7ce37551` - the guest bar restrictions.

## Precondition record

All six gate summaries present (20-06, 20-07, 20-08, 21-08, 21-10, 22-05). `git log --oneline 07ad7d21..HEAD -- DesktopView.tsx desktopClient.ts DesktopControlBar.tsx DesktopSendKeysMenu.tsx` listed and read before editing: `48f23917`, `951908f4` (merges), `b41055f0` (quick-261127-bb7 release chorded buttons, bar menu reset on unmount), `bb55eae6` and `e9e53a29` (21-10 phone layer), `c292ab30`, `6ca45021` (21-09), `9f4428be`, `2e9f88c4`, `3c22c9f7`, `0b4978a1` (21-08), `299d842f` (20 UI-1), `f97d700e` (20-09), `db356d0b`, `9d0bff43`, `dd7a6347` (20-08), `e31f3825`, `26f58bf2`, `60e1e93c` (20-07). The assumptions table held: bar, send-keys menu, clipboard menu and the one notice slot exist as 20-07 and 20-08 describe.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `defaultGuestMode` tolerates a window with no `pathname`**
- **Found during:** Task 1 verification.
- **Issue:** the existing `desktopClient.test.ts` injects a fake `win.location` with `host` and `hash` but no `pathname`, so `loc.pathname.startsWith` threw in 68 existing tests. A real Location always has a pathname.
- **Fix:** `typeof loc.pathname === 'string' && ...` with a comment, rather than editing the existing test files (the plan says an existing test failing means own-account behaviour changed).
- **Commit:** `15a44f6e`.

### Interpretations

- **Close reasons by code only.** The plan says "by code and reason string". `SignalTransport.onClose` carries only the code, and the codes are one to one with the reasons, so the page maps by code; the golden check compares each code with the reason string in `signal-guest-v1.jsonl`.
- **4410 in guest mode is Sharing has ended.** The own-account `desktop_off` card tells the reader to open Portty's Settings, which a guest does not have. Not in the golden file or the UI-SPEC; one table entry in `GUEST_CLOSE`.
- **Name fallback.** `<name> is controlling` falls back to "Someone is controlling" if the lease message carries an empty name (UI-SPEC is silent; the server sends a name).
- **Window and Accessibility chips** (`Couldn't bring the window…`, `Control needs Accessibility…`) keep their Phase 20 place after Locked and before the guest mode chips; the UI-SPEC precedence list does not mention them.
- **Not done, flagged:** a close of 1000 `replaced` (another tab of the same device was admitted) and 4429 `full` are not final on the page; they take the ordinary reconnect path (Reconnecting…). With two tabs of one device open they will take turns replacing each other at the 1 to 10 s backoff. The UI-SPEC has no card for either; a final "opened in another tab" card is a small follow-up if wanted. `main.tsx` needed no change (the path already selects the viewer shell and `DesktopView` defaults guest mode from it).
- **Tooling.** The worktree had no `node_modules`; `pnpm exec` installed them from the frozen lockfile (no package added, no `package.json` or lockfile change).

## Verification

- `pnpm --dir frontend exec tsc --noEmit -p tsconfig.json` clean after each task.
- Task verify sets: `desktopClient.guest`, `DesktopView.guest` (22 cases), `DesktopControlBar.guest`, plus the existing `desktopClient`, `DesktopView`, `DesktopControlBar`, `DesktopSendKeysMenu` tests: 352 tests pass. Full frontend suite: 142 files, 2584 tests pass.
- `bash tests/check-brand.sh` clean before each commit. Nothing ran Portty, capture, input injection, the host tailscaled or any 100.x address; vitest and jsdom only.
- Acceptance greps met: `/desktop/guest/ws` (2), `Someone Wants to Watch Your Desktop` (1), `admission` (9), `signal-guest-v1.jsonl` (3), `admit_timeout` (2), `'lease'` (2), `guest` in both bar files.
- Not exercised: a real browser against a real share server (the guest rows are 22-12 VM work); layout of the cards at 375 pt (the cards reuse the Phase 19 card classes, no CSS was touched).

## New test files for 22-13 (TESTING.md traceability; DSHR-03, DSHR-04, DSHR-05, DCTL-04)

- `frontend/src/lib/__tests__/desktopClient.guest.test.ts` (golden check of `internal/desktop/testdata/signal-guest-v1.jsonl`)
- `frontend/src/components/__tests__/DesktopView.guest.test.tsx`
- `frontend/src/components/__tests__/DesktopControlBar.guest.test.tsx`
- Manual checklist candidate (M-NN): in a real browser on a second tailnet account, the guest page wait, admit, grant control on and off, grant clipboard, and Stop Sharing over the frozen frame; a 375 pt phone width card check.

## Known Stubs

None.

## Threat Flags

None. The page renders one name (the lease holder's display name, cleaned and set as text, T-22-26), picks states by close code (T-22-28, golden-checked), and the page restrictions are usability only (T-22-27, enforced by 22-05). No package was added (T-22-SC).

## Self-Check: PASSED

All created files exist and commits 15a44f6e, ad8ca062 and 7ce37551 are on the branch.
