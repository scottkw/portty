---
phase: 07-hardening
plan: 02
subsystem: frontend/peer-tabs
status: complete
requirements-completed: [HARD-05]
tags: [HARD-05, relay, resume, xterm, serialize, tab-switch]
requires: []
provides:
  - "peer tab switch rejoins with ?resume=<offset> and keeps its content on a delta answer"
  - "RelayClient opts.resume and resumePoint()"
  - "frontend/src/lib/peerSnapshots.ts in-memory store (PEER_SNAPSHOT_MAX = 8)"
affects:
  - "07-04 (edits sendInput in relayClient.ts; wave 2)"
  - "07-05 (TESTING.md rows, VM spot check of the tab switch)"
tech-stack:
  added: []
  patterns:
    - "snapshot + offset as one stream point: offset read before close, serialize inside an empty-write flush callback"
key-files:
  created:
    - frontend/src/lib/peerSnapshots.ts
    - frontend/src/lib/__tests__/peerSnapshots.test.ts
    - frontend/src/components/__tests__/TerminalPanel.tabSwitch.test.tsx
  modified:
    - frontend/src/lib/relayClient.ts
    - frontend/src/lib/relayClient.test.ts
    - frontend/src/components/TerminalPanel.tsx
    - frontend/src/components/__tests__/TerminalPanel.guestClose.test.tsx
    - frontend/src/components/__tests__/TerminalPanel.reconnect.test.tsx
    - frontend/src/components/__tests__/TerminalPanel.test.tsx
decisions:
  - "HARD-05: a hidden peer tab keeps an in-memory xterm snapshot plus its stream offset (no hidden sockets); remount restores it and dials ?resume=<offset>"
  - "HARD-05: RelayClient.resumePoint() is null once the client is stopped, so an ended session leaves no snapshot"
metrics:
  duration: ~12m
  completed: 2026-11-18
actuals:
  tokens: 7100
  tasks: 2
  commits: 4
plan_head_before: f54a223a42507aad8d13cb177f1f24092fafac9e
plan_head_after: b33a66b3479c0f345d303889fd48ad66409684f8
---

# Phase 7 Plan 02: Peer tab delta rejoin Summary

A desktop peer tab now saves its real xterm content (SerializeAddon) plus the exact relay offset when a tab switch unmounts it, restores both on remount and rejoins with `?resume=<offset>`, so the host sends only the missed output. A lost offset falls back to the existing full replay with an in-order RIS.

## What was built

- `frontend/src/lib/peerSnapshots.ts`: module-level `Map` of `{data, cols, rows, offset}` keyed by session id. `takePeerSnapshot` returns and deletes, `putPeerSnapshot` re-inserts as newest and evicts the oldest beyond `PEER_SNAPSHOT_MAX = 8`. In memory only.
- `frontend/src/lib/relayClient.ts`: `opts.resume` (a safe non-negative integer sets offset, offsetValid and hasRendered), a private `withResume()` suffix helper shared by the first dial and redial, and `resumePoint()` (offset when valid and not stopped, else null).
- `frontend/src/components/TerminalPanel.tsx` (mount effect only): `snapSerializer` exists only when `remote && !wsURL`. On mount: take the snapshot, `applyHostGrid(snap.cols, snap.rows)`, write `snap.data`, then construct RelayClient with `resume: snap?.offset`. On cleanup: read `resumePoint()` before `client.close()`. If it is non-null, run `term.write('', cb)`, and inside the callback serialize, put, dispose the serializer, dispose the terminal. Otherwise dispose as before. The Phase 97 plugin serialize arm is untouched. App.tsx and WebShareSessionView.tsx are unchanged (`git diff --stat` is empty).

## Tests (for 07-05 / TESTING.md)

**New: `frontend/src/lib/__tests__/peerSnapshots.test.ts`**
- `peerSnapshots store (HARD-05)`:
  - take returns the entry once, then undefined
  - a second put for the same id replaces the first
  - keeps at most 8: after 9 puts the first id is gone and the other 8 remain
  - a re-put refreshes the position: the oldest untouched entry is evicted instead
- `snapshot + delta equals the live terminal in real xterm.js (HARD-05)`:
  - main screen: scrollback, a wrapped line, SGR, bracketed paste and a mid-line cursor
  - alternate screen: restored in the alt buffer, and leaving it gives the same normal buffer

**New: `frontend/src/components/__tests__/TerminalPanel.tabSwitch.test.tsx`**, `TerminalPanel peer tab switch (HARD-05)`:
- switch away and back restores the snapshot, dials resume=104, and a delta keeps the content
- offset gone on the host: the full replay resets the restored content (RIS after the snapshot)
- an ended session stores nothing: the next mount has no snapshot and no resume
- a host (local) tab stores nothing: a later remote mount has no snapshot and no resume

**Extended: `frontend/src/lib/relayClient.test.ts`**, `RelayClient reconnect mode (SHAR-07) > resume option (HARD-05)`:
- resume: 42 dials resume=42 and resumePoint() is 42 before any frame
- a delta answer: resumePoint() is null until the offset meta, then counts output; no reset
- a full answer resets the restored terminal once
- resume -1 / 1.5 / NaN is ignored: plain URL, resumePoint() null (it.each)

## SerializeAddon fidelity finding

None. With the real `@xterm/addon-serialize` and default `serialize()` options, snapshot plus delta matched the live terminal on every check: all buffer lines, wrapped-line flags, cursor, bracketed paste mode, and a byte-identical re-serialization (attributes included) on the main screen. On the alternate screen, the restored terminal was in the alt buffer with an equal screen and cursor, and after `?1049l` the normal buffer and cursor were equal too. No test was weakened.

## Verification

- `pnpm vitest run` (whole suite): `Test Files 90 passed (90)`, `Tests 1215 passed (1215)`
- `pnpm exec tsc --noEmit`: clean
- Task 1 list (tabSwitch, reconnect, guestClose, WebShareSessionView, relayClient): `Test Files 5 passed (5)`, `Tests 115 passed (115)` (before the Task 2 additions)
- grep checks: resumePoint in relayClient.ts 1+, putPeerSnapshot( and takePeerSnapshot( in TerminalPanel.tsx 1 each, SerializeAddon in peerSnapshots.test.ts 6

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] guestClose test's RelayClient mock lacked resumePoint**
- **Found during:** Task 1
- **Issue:** TerminalPanel.guestClose.test.tsx stubs RelayClient. A remote panel's cleanup now calls `client.resumePoint()`, which threw an unhandled TypeError in 6 cases.
- **Fix:** added `resumePoint() { return null }` to the stub. Production code was not made tolerant of a missing method.
- **Files modified:** frontend/src/components/__tests__/TerminalPanel.guestClose.test.tsx (outside files_modified; no overlap with 07-01/07-03)
- **Commit:** 8828f876

**2. [Rule 3 - Blocking] Snapshot leaked across cases in the reconnect test**
- **Found during:** Task 1
- **Issue:** the store is module-level. A remote 's1' panel unmounted in one TerminalPanel.reconnect.test.tsx case left a snapshot that the next case restored (resume dial, extra RIS). One case failed: "a full replay on the new socket writes RIS once…".
- **Fix:** an afterEach `takePeerSnapshot('s1')` drain, the same as the tabSwitch test's.
- **Files modified:** frontend/src/components/__tests__/TerminalPanel.reconnect.test.tsx
- **Commit:** 8828f876

**3. [Rule 3 - Blocking] SER-01 source-shape test forbade any `new SerializeAddon` in the mount effect**
- **Found during:** Task 1 (full suite)
- **Issue:** TerminalPanel.test.tsx "SerializeAddon construction lives in HOT-SWAP useEffect, NOT mount useEffect" guards the Phase 97 plugin serializer's placement. The plan deliberately adds a separate snapshot serializer to the mount effect.
- **Fix:** the mount-block assertion now checks that the plugin serializer's ref assignment and saver registration are absent (its intent). The hot-swap block must still contain `new SerializeAddon`.
- **Files modified:** frontend/src/components/__tests__/TerminalPanel.test.tsx
- **Commit:** 8828f876

**4. [Rule 1 - Bug, plan-anticipated] Ended session stored a snapshot**
- **Found during:** Task 2 (RED)
- **Fix:** `resumePoint()` returns null when the client is stopped.
- **Commit:** b33a66b3

## Known Stubs

None.

## Threat Flags

None. No new endpoints or server input. `?resume=` uses the existing hub contract (T-07-09). T-07-07 is mitigated by reading the offset before close and serializing inside a flush callback, and is pinned by the fidelity tests. T-07-08 is mitigated by the cap of 8 and take-deletes.

## Commits

- ec203245 test(07-02): add failing peer tab-switch snapshot/resume test
- 8828f876 feat(07-02): peer tab switch rejoins with a delta replay (HARD-05)
- 6bc75306 test(07-02): pin snapshot fidelity, fallback, ended session and resume semantics
- b33a66b3 fix(07-02): an ended peer session keeps no tab-switch snapshot

## Self-Check: PASSED

- FOUND: frontend/src/lib/peerSnapshots.ts, frontend/src/lib/__tests__/peerSnapshots.test.ts, frontend/src/components/__tests__/TerminalPanel.tabSwitch.test.tsx
- FOUND: ec203245, 8828f876, 6bc75306, b33a66b3 (ancestors of HEAD)
