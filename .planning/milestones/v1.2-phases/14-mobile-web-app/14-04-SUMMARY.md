---
phase: 14-mobile-web-app
plan: 04
subsystem: frontend
tags: [react, relayClient, same-account-gate, reconnect, pageshow, vitest]
requires:
  - phase: 14-01
    provides: same-account gate on /info and the WebSocket (403 + X-Portty: 1)
  - phase: 14-03
    provides: the page-route refusal and the X-Portty marker the SPA reads
provides:
  - UserRefused component (in-app "This link only works on your own devices" screen)
  - App bootstrap and reconnect-probe branches for 403 + X-Portty: 1
  - shareInfoVerdict(status, porttyMarker) and CLOSE_REASON_OTHER_ACCOUNT
  - RelayClient pageshow wake and onConnectStuck after two connect timeouts
  - TerminalPanel onOtherAccount prop and connectStuck state
affects: [14-08, 14-12, 14-14]
status: complete
actuals:
  tokens: 4050
  tasks: 2
  commits: 2
plan_head_before: ba23ffff0f37cd2d0390aeaf469142044eca5274
plan_head_after: e377fbf9db61eddc4f39a24ca9bf32b5411a45ce
tech-stack:
  added: []
  patterns:
    - "the stop verdict carries a client-only close reason so TerminalPanel can branch without a new wire value"
key-files:
  created:
    - frontend/src/components/UserRefused.tsx
    - frontend/src/components/__tests__/UserRefused.test.tsx
  modified:
    - frontend/src/App.tsx
    - frontend/src/components/__tests__/App.webLink.test.tsx
    - frontend/src/lib/closeReasons.ts
    - frontend/src/lib/__tests__/closeReasons.test.ts
    - frontend/src/lib/relayClient.ts
    - frontend/src/lib/relayClient.test.ts
    - frontend/src/components/WebShareSessionView.tsx
    - frontend/src/components/__tests__/WebShareSessionView.test.tsx
    - frontend/src/components/TerminalPanel.tsx
    - frontend/src/components/__tests__/TerminalPanel.guestClose.test.tsx
key-decisions:
  - "shareInfoVerdict's marker parameter is required (no default) so a forgotten call site fails tsc"
  - "connectStuck is surfaced as a data-connect-stuck attribute on the panel root until 14-08 reads it for the Reload action"
requirements-completed: []  # MOB-05 client half done; MOB-03 needs the real-phone check (14-14). requirements.mark-complete was not run, same as 14-03.
---

# Phase 14 Plan 04: Refusal screen, pageshow wake and stuck signal Summary

A browser on another Tailscale account that loads the SPA now sees "This link only works on your own devices" (at bootstrap and when the reconnect probe is refused), the relay client wakes on `pageshow`, and two consecutive connect timeouts are reported as stuck while reconnecting continues.

Logs: `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.NWRmXkQ0t8` (Task 1) and `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.yl9XRes9Xi` (Task 2).

## Accomplishments

- **Refusal screen.** `UserRefused` (no props, `.link-dead` markup, copy identical to `web/user-refused.html`). `LinkNoLongerWorks` untouched.
- **Bootstrap.** App checks `res.status === 403 && res.headers.get('X-Portty') === '1'` before the 401/403 branch: sets `webOtherUser`, opens no tab. Plain 401/403 stay Link No Longer Works; 503 opens the tab. While `webOtherUser` is set the web session view is not rendered, so a probe-time refusal replaces an already-open view.
- **Probe.** `shareInfoVerdict(status, porttyMarker)`: 403 + marker is `stop / other-account`. `WebShareSessionView.probeShare` passes `res.headers.get('X-Portty') === '1'`.
- **Stop reason.** `ReconnectVerdict` stop reason gains `'other-account'`; RelayClient maps it to the client-only `CLOSE_REASON_OTHER_ACCOUNT = 'other account'` (documented as never sent by a server; `isTerminalClose` unchanged and pinned in a test).
- **pageshow.** Listener added beside `online` / `visibilitychange` and removed in `teardown`. The tick-gap path is the backstop; a test pins an iOS-like suspend with no event at all (probe, abandon, redial).
- **Stuck.** Consecutive `connectMs` timer firings are counted; the second calls `onConnectStuck(true)` once; the first host frame resets the count and calls `onConnectStuck(false)`; teardown resets. Backoff is unchanged.
- **TerminalPanel.** Guest `onClose` with the other-account reason disables stdin, shows no ended banner, calls `onOtherAccountRef.current`. `onConnectStuck` is stored in `connectStuck`.

## New props and callbacks

| Name | Where | Type |
|------|-------|------|
| `onOtherAccount` | `TerminalPanelProps`, `WebShareSessionViewProps` | `() => void` (optional) |
| `onConnectStuck` | `RelayClientCallbacks` | `(stuck: boolean) => void` (optional) |
| `shareInfoVerdict` 2nd param | closeReasons.ts | `porttyMarker: boolean` (required) |
| `CLOSE_REASON_OTHER_ACCOUNT` | closeReasons.ts | `'other account'` (client-only) |
| `ReconnectVerdict` stop reason | relayClient.ts | `'share-stopped' \| 'ended' \| 'other-account'` |
| `data-connect-stuck` | TerminalPanel root div | `'true'` or absent |

## Test results

- Task 1 focused (`App.webLink UserRefused LinkNoLongerWorks`): 3 files, 9 tests passed.
- Task 2 focused (App.webLink, closeReasons, relayClient, WebShareSessionView, TerminalPanel.guestClose): 7 files, 207 tests passed.
- Full vitest (run once): 111 files, 1690 tests passed, 0 failed.
- `tsc --noEmit`: clean. `tests/check-share-removal.sh`: exit 0.
- ESLint is not installed in `frontend/` (`eslint` not found), so it was not run.

## Existing tests edited

- `closeReasons.test.ts`: `shareInfoVerdict` cases now pass the required marker argument (signature change); 403 without the marker still pins share-stopped.
- `App.webLink.test.tsx`: the mocked `WebShareSessionView` gained a button wired to `onOtherAccount` (for the probe-refusal case); `stubInfo` accepts headers.
- `TerminalPanel.guestClose.test.tsx`: the RelayClient mock now captures `onConnectStuck`.
- `relayClient.test.ts`: `mocks()` gained `onConnectStuck`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] `connectStuck` state tripped noUnusedLocals**
- **Found during:** Task 2 (tsc)
- **Issue:** the plan stores `onConnectStuck` as a `connectStuck` state that only 14-08 reads, so tsc failed with TS6133.
- **Fix:** read it as `data-connect-stuck` on the panel root, which also makes it testable now.
- **Files modified:** frontend/src/components/TerminalPanel.tsx (test in TerminalPanel.guestClose.test.tsx)
- **Commit:** e377fbf9

**2. [Environment] Verify greps need uncolored vitest output**
- Plan verify greps `Tests +[0-9]+ passed`, which does not match vitest's ANSI-colored output when logs are redirected. Ran with `NO_COLOR=1`; no plan change needed.

Otherwise the plan ran as written. The task 1 "no web session view while it is set" line was done by gating the view's render on `!webOtherUser`.

## Auth gates

None.

## Known Stubs

None.

## Threat Flags

None. T-14-17, T-14-18 and T-14-19 mitigations are in place and each has a pinning test.

## Not verified

- No real iPhone check (MOB-03's human item stays with 14-14), no browser run against a real share server, no host tailscaled contact.
- The Wails desktop build was not rebuilt; frontend `dist` output was not regenerated here.

## Self-Check: PASSED

- Files: UserRefused.tsx, UserRefused.test.tsx present.
- Commits ec48ac3b and e377fbf9 are ancestors of HEAD.
