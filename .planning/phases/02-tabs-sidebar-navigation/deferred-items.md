# Phase 02 Deferred Items

Out-of-scope discoveries logged during plan execution (not fixed).

## From 02-04

- **Flaky test under full-suite load:** `frontend/src/components/__tests__/SessionShareModal.test.tsx` › "SM-02: .hub-share-modal__tabpanel is nested inside .hub-share-modal__body" failed once in a full `vitest run`, then passed alone and in three full runs in a row. It waits with a single `await tick()` before querying the tabpanel, which races under load. SessionShareModal was not touched by 02-04. Plan 02-08 or 02-12 (share dialog restyle) could make the test wait for the element instead.
