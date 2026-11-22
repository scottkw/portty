---
created: 2026-11-22
area: frontend
due: later
---
After the pointer leaves the terminal and comes back to the SAME cell, and the text under that cell changed meanwhile, the first Cmd-click sends no path click (finding F-2 of the 12-09 re-run, C-04R in .planning/phases/12-file-path-click/12-VM-CHECKS.md). User decision 2026-11-22: "Todos, not blockers".

Repro (build C, office session p1 viewed in the office local tab and a home peer tab):
1. A path (`docs/diagram.png`) is printed and the pointer rests on it, or is warped (CGEvent, no intermediate cells) out of the terminal and back to the same cell.
2. Meanwhile the other viewer types new output so the text under that cell changes.
3. Cmd-click at the same cell: 0 `path click` lines in the office log. Seen 11 of 11 times in the home peer tab (A1..A6, C1..C3, E1, F1) and 1 of 1 in the office local tab.
4. A click at any other cell of the same text, then the same cell again, works every time (6 of 6). With the pointer resting and not leaving, a changed-text same-cell click works (office 1 of 1, peer 1 of 1).

Likely cause (not proven): xterm Linkifier's same-cell memory (`_lastBufferCell`, `_activeLine`) after a mouse leave. 12-08's `PathLinksHandle.invalidate()` runs only on capability reports and terminal resizes (`frontend/src/lib/pathLinks.ts`), so a leave followed by a return to the same cell is not covered. A real pointer passes over other cells on its way back, which resets it, so the practical effect is small.

Fix idea: also call `invalidate()` on `mouseleave`, or on a buffer change under the hovered cell; add a case to `frontend/src/lib/__tests__/pathLinks.linkifier.test.ts` with the real xterm Linkifier (leave, change the row, return to the same cell, Cmd-click).
