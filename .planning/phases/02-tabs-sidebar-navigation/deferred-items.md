# Phase 02 Deferred Items

Out-of-scope discoveries logged during plan execution (not fixed).

## From 02-04

- **Flaky test under full-suite load:** `frontend/src/components/__tests__/SessionShareModal.test.tsx` › "SM-02: .hub-share-modal__tabpanel is nested inside .hub-share-modal__body" failed once in a full `vitest run`, then passed alone and in three full runs in a row. It waits with a single `await tick()` before querying the tabpanel, which races under load. SessionShareModal was not touched by 02-04. Plan 02-08 or 02-12 (share dialog restyle) could make the test wait for the element instead.

**Resolved in 02-05:** the SM-02 flake above. SM-02, SM-03 and SM-05 now poll the async `cachedShare` seed with `vi.waitFor` (commit 4e587aae).

## From 02-05

- `lib/remoteAdapter.ts` `adaptAllRemoteSessions` and `lib/remoteSession.ts` `findRemoteSession` (listed as orphans in 02-04-SUMMARY) were not in 02-05's delete list and are still there. Only their own tests use them.
- `frontend/src/assets/agenthub-title-logo.png` has no frontend importer but `scripts/verify-icons.sh` (SC5) checks that it exists, so it was kept. Delete it together with that check when Phase 3 rebrands the icons.
- `frontend/src/components/Hub/` now holds only the chat components and `WebShareSessionView` (plan 02-07). The Share dialog's classes are still named `hub-share-modal*` / `hub-modal-overlay` / `hub-modal__close`. A rename could go with the 02-12 restyle.
