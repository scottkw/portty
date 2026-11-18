---
phase: 05-overnight-survival
plan: 05
subsystem: frontend
status: complete
tags: [frontend, reconnect, re-auth, shar-07]
requires:
  - "05-04: RelayClient reconnect mode, ReconnectVerdict, TerminalPanel beforeReconnect prop"
provides:
  - "peerConnectVerdict(errorCode) and shareInfoVerdict(status) in closeReasons.ts"
  - "Tab.connectHost (the MagicDNS name or hostname ConnectPeerSession was called with)"
  - "App reconnectPeerTab(host, sessionId): GetRelayPort + ConnectPeerSession, returns ReconnectVerdict"
  - "WebShareSessionView beforeReconnect prop (remote pass-through) and browser /info?cap= probe"
affects: [05-07]
tech-stack:
  added: []
  patterns:
    - "The pre-redial hook returns only a verdict and port. It never touches tab state, so the view keeps its key and xterm buffer"
key-files:
  created: []
  modified:
    - frontend/src/lib/closeReasons.ts
    - frontend/src/App.tsx
    - frontend/src/components/TabBar.tsx
    - frontend/src/components/WebShareSessionView.tsx
    - frontend/src/components/__tests__/App.open-remote.test.tsx
    - frontend/src/components/__tests__/WebShareSessionView.test.tsx
    - frontend/src/lib/__tests__/closeReasons.test.ts
    - .planning/phases/05-overnight-survival/deferred-items.md
decisions:
  - "no_such_session maps to share-stopped, not ended: mid-gap the daemon cannot tell Share-off from session-ended, and the stopped-sharing copy is true in both cases"
  - "handleOpenRemoteSession now calls ConnectPeerSession(connectHost, ...) so the initial connect and the redial use the same host value; the source-inspection test was updated to match"
  - "The Tab type lives in TabBar.tsx, so connectHost was added there (the plan said 'the Tab type' without naming the file)"
metrics:
  duration: 18min
  completed: 2026-11-17
  tasks: 2
  files: 8
estimate:
  tokens: 50000
  tasks: 2
actuals:
  tokens: 5150
  tasks: 2
  commits: 4
plan_head_before: f085f06b824b75d4dcaea0d6ad96b45eab0272ee
plan_head_after: 021cf2407ff95a978141e791fbe5fc02e966850a
---

# Phase 5 Plan 05: Pre-Redial Re-Auth Summary

Before each redial, both guest surfaces now check that they still have access. A GUI peer tab re-runs `GetRelayPort()` and then `ConnectPeerSession(connectHost, id)`. That re-checks the same-account WhoIs gate and Share, refills the daemon's cap store, and picks up a new relay port after the home Mac's daemon restarts. The browser share view probes `GET /api/sessions/{id}/info?cap=`. Each answer becomes a RelayClient verdict: redial, retry later, or stop with the existing banner.

## What was built

- **`closeReasons.ts`:**
  - `peerConnectVerdict(errorCode)`: `not_sharing`, `no_such_session` and `forbidden` return `stop share-stopped`. Every other code returns `retry`.
  - `shareInfoVerdict(status)`: 2xx returns `ok`, 401/403 return `stop share-stopped`, 404 returns `stop ended`, and anything else (including 0) returns `retry`.
  - Both use a type-only import of `ReconnectVerdict` from `relayClient`, so no runtime import cycle.
- **`App.tsx`:**
  - `handleOpenRemoteSession` computes `connectHost = session.peerDNSName || peerHost` and stores it on the peer tab.
  - New `reconnectPeerTab` callback with an empty dependency list. It calls neither `openWebSessionTab` nor `setTabs`, so the view's `${sessionId}:${cap}` key never changes and the tab is never remounted.
  - The `__websession__` branch passes `beforeReconnect` only when the tab has a `peerHost`. If `connectHost` is missing it falls back to `peerHost`.
  - When the loop stops, the existing `onShareStopped` path still drops the sidebar row.
- **`TabBar.tsx`:** new field `Tab.connectHost?`.
- **`WebShareSessionView.tsx`:**
  - New `beforeReconnect` prop.
  - Proxy (remote) tabs pass the App hook through to TerminalPanel unchanged.
  - The browser path passes its own `probeShare` instead. It fetches with `cache: 'no-store'` and `encodeURIComponent` on the id and cap, and a rejected fetch counts as `retry`.
  - The cap still never reaches TerminalPanel props (`grep -c capToken TerminalPanel.tsx` = 0). The GUI hook returns only a port.

## Verification

- Task 1 verify (tracer): tsc clean; `App.open-remote.test.tsx` gave `Test Files  1 passed` (26 tests). Tracer gate: the automated verify was re-run end-to-end and passed (human_verify_mode end-of-phase), then Task 2 continued.
- Task 2 verify: tsc clean; the three-file run gave `Test Files  3 passed` (62 tests).
- Full suite: `vitest run --maxWorkers=4` passed 83/83 files (1119 tests). Two default-parallel runs each failed only the two known load flakes, `SharePopover Copy` and `initialGrid > returns null under jsdom`. The host load average was about 330 at the time. Both pass alone, and 05-05 does not touch either file (see Deferred Issues).
- Acceptance greps: `export function peerConnectVerdict` 1, `connectHost` in App.tsx ≥1, `beforeReconnect` in App.tsx and WebShareSessionView.tsx ≥1, `export function shareInfoVerdict` 1, `/info?cap=` in WebShareSessionView.tsx 1, `capToken` in TerminalPanel.tsx 0. Both `test(05-05)` commits are older than their `feat(05-05)` commits.
- All verify greps need `NO_COLOR=1` (vitest emits ANSI color even when piped).

## TDD Gate Compliance

| Task | RED commit | GREEN commit | RED cause |
|------|-----------|--------------|-----------|
| 1 | ee8b2093 | 0dc0bffc | 9 hook cases failed: no `beforeReconnect` from App, so the stub's button set no verdict and no second connect happened. The local-tab guard passed at RED as expected |
| 2 | 08e518b7 | 021cf240 | `shareInfoVerdict is not a function`; `beforeReconnect` undefined on the browser path. The remote pass-through and `peerConnectVerdict` table cases were already green, because Task 1 built them |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] The MagicDNS reconnect test passed vacuously at RED**
- **Found during:** Task 1 RED
- **Issue:** `toHaveBeenLastCalledWith(dnsName, id)` also matched the initial open call.
- **Fix:** it now also asserts `toHaveBeenCalledTimes(2)`.
- **Commit:** ee8b2093

**2. [Rule 1 - Bug] The stub mount counter leaked across describes**
- **Found during:** Task 1 GREEN
- **Issue:** earlier suites' mounts showed up as 8 remounts.
- **Fix:** the counters are reset in `beforeEach`.
- **Commit:** 0dc0bffc

**3. [Discretion] The initial connect also uses `connectHost`**
- To avoid two copies of `session.peerDNSName || peerHost`, the existing call now passes `connectHost`.
- The source assertion in `App.open-remote.test.tsx` was updated to match. It still guards that no code-based access returns.
- **Commit:** 0dc0bffc

**4. [Scope] `TabBar.tsx` was not in the plan's file list**
- The `Tab` interface is defined there, so this is where `connectHost` goes.

## Deferred Issues

- Load flakes: `SharePopover Copy` (known) and `initialGrid` (known from the 05-04 base runs). Logged in `deferred-items.md`.

## Related out-of-scope todos (not fixed, not made worse)

- `2026-11-17-duplicate-remote-tab-blank-window.md`: its line numbers in App.tsx have moved by about 15 lines.
  - The new hook only reads tabs. Each tab still holds the same connectHost and cap, so duplicate tabs would just run two independent hooks.
  - If `activeWebTab` is missing (the todo's blank-window path), `peerHost` is undefined and App passes no hook. The view then falls into the browser branch with `window.location.origin`. The WebSocket already throws there before any redial, so the probe adds no new failure.
  - That todo's fix 4 (render nothing when `activeWebTab` is missing outside web mode) would also cover this.
- `2026-11-17-guest-scrollbar-at-pane-edge.md`: unrelated to this plan.

## Known Stubs

None. TerminalPanel's `{kind:'ok'}` default remains only for a guest panel that gets no hook. After this plan, no App path produces that.

## Notes for later plans

- 05-07 (TESTING.md):
  - Add the new cases to the existing rows for `App.open-remote.test.tsx`, `WebShareSessionView.test.tsx` and `closeReasons.test.ts` (SHAR-07).
  - Add the manual item for the known limit: Share turned off while the browser slept, with no other session shared. The share server's port is closed, the probe gets a network error, and the page shows "Connection lost — reconnecting…" indefinitely.
- App's `relayPort` state is not updated by the hook. A later remount dials the stale port once, fails, and heals through the same hook (planned).

## Self-Check: PASSED

- FOUND: frontend/src/lib/closeReasons.ts, frontend/src/App.tsx, frontend/src/components/WebShareSessionView.tsx, frontend/src/components/TabBar.tsx, the three test files
- FOUND commits: ee8b2093, 0dc0bffc, 08e518b7, 021cf240
