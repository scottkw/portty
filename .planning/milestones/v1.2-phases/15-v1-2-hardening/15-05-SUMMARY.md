---
phase: 15-v1-2-hardening
plan: 05
subsystem: frontend peer tabs
tags: [react, vitest, peer-tab, close-reasons, session-ended]

requires:
  - phase: 14
    provides: peerConnectVerdict, beforeReconnect re-auth for peer tabs (SHAR-07)
provides:
  - "peerConnectVerdict(errorCode, sawEnded = false): no_such_session maps to ended for a tab that saw the session-ended close"
  - "Tab.sessionEnded flag set from the guest's session-ended close, keyed by host plus session"
  - "onSessionEnded prop on TerminalPanel and WebShareSessionView"
affects: [15-06 (edits App.tsx after this plan), 15-09 TESTING.md traceability]

actuals:
  tokens: 4200
  tasks: 2
  commits: 2
plan_head_before: e726652c495ed781fc1584b698c6e94bd2b7fe3c
plan_head_after: a190453df687b08e5ab50f3251975bc2ab3a264d
commits: 2

tech-stack:
  added: []
  patterns:
    - "Per-tab state that must survive the panel remount lives on the Tab in App state, not in the panel"

key-files:
  created: []
  modified:
    - frontend/src/lib/closeReasons.ts
    - frontend/src/components/TabBar.tsx
    - frontend/src/components/TerminalPanel.tsx
    - frontend/src/components/WebShareSessionView.tsx
    - frontend/src/App.tsx
    - frontend/src/components/__tests__/App.open-remote.test.tsx
    - frontend/src/lib/__tests__/closeReasons.test.ts
    - frontend/src/components/__tests__/TerminalPanel.guestClose.test.tsx

key-decisions:
  - "Frontend verdict mapping only, no wire change: the daemon still answers no_such_session, the tab's own history decides ended vs share-stopped"
  - "not_sharing and forbidden stay share-stopped even after an ended close, so the daemon's explicit codes are never overridden"

requirements-completed: []

coverage:
  - id: D1
    description: "An ended peer tab clicked again keeps the ended state: no_such_session after the session-ended close gives the ended verdict (ROADMAP Phase 15 SC2, part 1)"
    verification:
      - kind: unit
        ref: "frontend/src/components/__tests__/App.open-remote.test.tsx#after the session-ended close, no_such_session stops with the ended verdict"
        status: pass
      - kind: unit
        ref: "frontend/src/lib/__tests__/closeReasons.test.ts#no_such_session after the session-ended close stops with ended"
        status: pass
    human_judgment: false
  - id: D2
    description: "not_sharing and forbidden stay share-stopped after an ended close; a tab without the ended close keeps the old mapping"
    verification:
      - kind: unit
        ref: "frontend/src/components/__tests__/App.open-remote.test.tsx#after the session-ended close, not_sharing still stops with share-stopped"
        status: pass
      - kind: unit
        ref: "frontend/src/components/__tests__/App.open-remote.test.tsx#errorCode %s stops with the stopped-sharing verdict"
        status: pass
    human_judgment: false
  - id: D3
    description: "The panel calls onSessionEnded only for the session-ended close, with the UI-SPEC copy unchanged"
    verification:
      - kind: unit
        ref: "frontend/src/components/__tests__/TerminalPanel.guestClose.test.tsx#'session ended' keeps the session-ended banner"
        status: pass
    human_judgment: false

duration: 8min
completed: 2026-11-23
status: complete
---

# Phase 15 Plan 05: Ended peer tab keeps "Session ended" Summary

**A peer tab that already saw the session-ended close now remembers it on the Tab (`sessionEnded`), and `peerConnectVerdict` maps a later `no_such_session` to `ended` instead of `share-stopped`, so clicking the tab again shows "Session ended" rather than "stopped sharing".**

## Performance

- **Duration:** 8 min
- **Started:** 2026-11-23T14:31:00Z
- **Completed:** 2026-11-23T14:39:00Z
- **Tasks:** 2
- **Files modified:** 8

## Accomplishments

- The guest panel's `onClose` calls the new `onSessionEnded` for `CLOSE_REASON_SESSION_ENDED`; `WebShareSessionView` passes it through; App's `markPeerSessionEnded(host, sessionId)` sets `Tab.sessionEnded` on the tab matching `peerSnapshotHost(t)` plus `sessionId` (the INPT-04 match `dropPeerSession` uses), so another Mac's tab with the same session id is untouched.
- `reconnectPeerTab(host, sessionId, sawEnded)` feeds the flag into `peerConnectVerdict(result.errorCode, sawEnded)`.
- No copy, CSS or layout change (`style.css`, `SessionEndedBanner.tsx`, `Sidebar.tsx` untouched).
- Part 2 of the todo (the sidebar "Can't reach this Mac" flash) stays in 15-07.

## Task Commits

1. Task 1 (tracer): `aff658fc` fix(15-05): an ended peer tab clicked again keeps the session-ended state
2. Task 2: `a190453d` test(15-05): pin the ended verdict table and the panel onSessionEnded callback

## TDD evidence (RED before GREEN)

- Unfixed code: the new `no_such_session` ended-verdict case in App.open-remote.test.tsx FAILED (received share-stopped). The `not_sharing` case passes on unfixed code by design: it is the guard that the daemon's explicit code is not overridden.
- After the fix: App.open-remote 45 of 45 pass; Task 2 files 82 of 82 pass (closeReasons, TerminalPanel.guestClose, SessionEndedBanner); `tsc --noEmit` clean.
- Task 2 is pin-only tests over Task 1's code, so it has no separate RED.
- Verification logs: Task 1 in `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.Zkvbnu8eQ9` (RED run in `.../tmp.SH84Q6hddq`), Task 2 in `.../tmp.f6VXygJ8QR`.

## Deviations from Plan

None - plan executed exactly as written. (Task 2's `onSessionEnded` spy is a describe-scoped `vi.fn()` shared by both render sites; `vi.clearAllMocks` in `afterEach` resets it per case.)

## Known Stubs

None.

## Threat Flags

None. T-15-12 (accept): the flag only picks between two fixed banners on a tab that already received the server's own session-ended close.

## Self-Check: PASSED

- All eight modified files exist; commits `aff658fc` and `a190453d` are ancestors of HEAD.
- Full vitest and Go suites are deliberately deferred to 15-09 per the plan.
