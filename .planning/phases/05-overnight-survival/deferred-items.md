# Phase 5 Deferred Items

## From 05-04

- **Flaky `SharePopover: Copy > Copy writes the full link and reads Copied! for 2 s`** (`frontend/src/components/__tests__/SharePopover.test.tsx`). It passes when run alone and fails in roughly half of full `vitest run` passes. It also failed in 2 of 3 full runs at the plan base (7d5a9abd), so 05-04 did not cause it. One base run also failed `initialGrid.test.ts > measureInitialGrid > returns null under jsdom`, another load flake. Likely cause: `vi.useFakeTimers({ shouldAdvanceTime: true })` lets real time pass under parallel-worker load, so the 1999 ms check sees the 2 s timer already fired. Out of scope for 05-04; not fixed.
