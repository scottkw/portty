---
phase: 21-portty-app-and-iphone-viewers
plan: 05
subsystem: ui
tags: [typescript, webrtc, desktop-tab, vitest, local-network]

requires:
  - phase: 19
    provides: createDesktopClient with injectable transport, check, beforeRedial, createPeerConnection and clock
provides:
  - createPeerDesktopOptions (deferred first transport, refresh before redial, check, Local Network watcher wiring)
  - connectVerdict, watchLocalNetwork, createHiddenPause, desktopTabLabel, LABEL_TOOLTIP
  - DESKTOP_TAB_CARDS and LOCAL_NETWORK_CARD (UI-SPEC copy)
affects: [21-07, 21-08, 21-12, 21-13]

tech-stack:
  added: []
  patterns:
    - "Lazy transport wrapper: the WebSocket is opened only once the loopback url is known"
    - "Watcher uses addEventListener only, never the client's property handlers"

key-files:
  created:
    - frontend/src/lib/peerDesktop.ts
    - frontend/src/lib/__tests__/peerDesktop.test.ts
  modified: []

key-decisions:
  - "An empty url from a redial's connect keeps the last good url, so the proxy's own refusal drives classification instead of a silent 1006 loop"
  - "desktopTabLabel treats captured as controlling only while connected (streaming with a picture); not connected always wins"
  - "A Mac that does not allow control shows View only when it is not locked"
  - "createHiddenPause also starts its timer when the document is already hidden at creation"

requirements-completed: [DVWR-01]

status: complete
duration: ~35min
completed: 2026-11-26
actuals:
  tokens: 7000
  tasks: 3
  commits: 5
plan_head_before: 6662f19644b8914f539b0952f62f526d6d7e3711
plan_head_after: 525db247c54b56a49e8ab800fff28c33aef5fa77
---

# Phase 21 Plan 05: Desktop tab connection logic Summary

**Pure, tested module that connects the Phase 19 desktop client to a peer through the daemon's loopback proxy, refreshes the daemon-held key before every redial, heals a blocked Local Network, pauses after 2 minutes hidden, and holds the app's label and card copy.**

## Accomplishments

- Tracer: the real `createDesktopClient` streams over the loopback url that `connect(host)` returned, with one transport and no `cap=` or `token=` in any url.
- Refresh before redial, handshake classification (`connectVerdict`), and a Local Network watcher that arms on the applied answer, reports blocked after 10 s of ICE `new` (or `new` straight to `failed`) and closes the transport with 4000 so the client rebuilds.
- Hidden-window pause (119 s no pause, 120 s pauses once, resume on next visible), `desktopTabLabel`, `LABEL_TOOLTIP`, and the UI-SPEC card tables word for word.

## Task Commits

1. Task 1 (tracer): `12af3218` feat - tracer for the Desktop tab streaming over the loopback url
2. Task 2 RED: `b1610b71` test - failing tests for redial refresh, classification and the watcher
3. Task 2 GREEN: `2dc0881a` feat - refresh before redial, classify handshakes, heal Local Network
4. Task 3 RED: `7fc62f12` test - failing tests for hidden pause, label and card copy
5. Task 3 GREEN: `525db247` feat - hidden-window pause, toolbar label and card copy

## Verification

- `tsc --noEmit`: clean. `peerDesktop.test.ts`: 40 tests pass. Full frontend suite: 127 files, 2001 tests pass. `bash tests/check-brand.sh`: clean.
- All acceptance greps hold (single occurrence of the pinned strings, no wailsjs or react import, no `onconnectionstatechange =`).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] pnpm refused to run in the worktree**
- **Found during:** Task 1 verify
- **Issue:** `pnpm --dir frontend exec` tries a dependency check and fails on the symlinked `frontend/node_modules` ("workspace hoist directory is not a real directory").
- **Fix:** ran the same binaries directly (`frontend/node_modules/.bin/tsc` and `vitest`), same checks and same config. No package was installed and no lockfile changed.
- **Files modified:** none

**2. [Plan wording] Test-case count**
- The acceptance grep counts top-level `it(` lines (12, at least 8); `it.each` expands to 40 running tests.

None other: the plan executed as written. A `check` throw is mapped to `retry` inside `check` itself (as the behavior list says), in addition to the client's own catch.

## Known Stubs

None.

## Threat Flags

None. The module adds no network endpoint; it only opens the loopback url the binding returns. T-21-12, T-21-13 and T-21-14 are covered by the tracer assertion, the 119 s and 120 s tests and the watcher tests.

## Notes for later plans

- 21-08 wires `connect` to the `ConnectPeerDesktop` binding (21-07), passes `options` to `createDesktopClient`, calls `createHiddenPause` with `client.stop` and `client.start` (or `wake('external')`), maps `onLocalNetwork` to the Local Network card, and renders `desktopTabLabel` and the card tables. `forceRebuild` and `dispose` are returned for the Reconnect button and tab close.
- A redial now makes two `connect` calls after an empty-url failure (the client's `check`, then `beforeRedial`); both are cheap binding calls.
- New test file for 21-13 to list in TESTING.md Sections 2 and 4 (DVWR-01): `frontend/src/lib/__tests__/peerDesktop.test.ts`. TESTING.md was not edited here.
- The `frontend/node_modules` symlink in this worktree is untracked and was never staged.

## Self-Check: PASSED

- FOUND: frontend/src/lib/peerDesktop.ts
- FOUND: frontend/src/lib/__tests__/peerDesktop.test.ts
- FOUND commits: 12af3218, b1610b71, 2dc0881a, 7fc62f12, 525db247
