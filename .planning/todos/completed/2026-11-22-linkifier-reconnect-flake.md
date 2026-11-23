---
created: 2026-11-22
area: frontend
source: Phase 14 plan 14-14 checkpoint (flake first seen in 14-08, again in the 14-12 gate)
due: later
---
# Linkifier reconnect test flakes under full-suite load

`frontend/src/lib/__tests__/pathLinks.linkifier.test.ts`, "after a reconnect, another cell of the same row offers the link", failed once in the 14-08 full vitest run and once in the 14-12 gate (`term.onRender fired: expected 2 to be greater than 2`). It passes alone (9 of 9) and the file imports nothing Phase 14 touched.

Cause: `renderOnce` waits a fixed `sleep(100)` for a real xterm render, which is too short when the whole suite loads the machine.

What to do: replace the fixed `sleep(100)` with a wait on the render itself (resolve on `term.onRender`, or poll with a timeout until the render count increases), then run the file in a loop and a full vitest run to confirm it no longer flakes. Test-only change; no product code.

## Resolution

Resolved in Phase 15 plan 15-04 (commit 21099b07).
The linkifier tests wait on events (`vi.waitFor`) instead of wall-clock sleeps; 20 of 20 alone and clean in the 15-09 full vitest run.
