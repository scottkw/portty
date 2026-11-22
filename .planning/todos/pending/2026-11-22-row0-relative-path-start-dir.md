---
created: 2026-11-22
area: relay
due: later
---
A relative path printed on screen row 0 of a never-scrolled session resolves against the shell's START directory, not the directory it was printed in (finding F-1 of the 12-09 re-run, C-04R in .planning/phases/12-file-path-click/12-VM-CHECKS.md). User decision 2026-11-22: "Todos, not blockers".

Repro (fresh session, a path click needs the zsh shim):
1. `portty new` a session; the first prompt is absolute row 0.
2. `cd ~/p12/proj` (its OSC 7 report arrives while the cursor is on row 1, so the cwd log is {row 0: start dir, row 1: proj}).
3. `clear; ls -1d docs/*` (nothing was ever pushed to history, so absolute rows do not move: the first output row is still absolute row 0).
4. Cmd-click `docs/big.dat` printed on row 0: `path click … outcome=missing` (it looked under the start directory). The same text on row 1 gives `outcome=too-large`; the same with `docs/diagram.png`.

Effect: rows 1 and below resolve correctly, and an absolute path is unaffected. A session whose screen has scrolled does not show it (the first run's p1 had scrolled). Workaround: a leading blank line (`clear; echo; ls …`) keeps the path off row 0.

Where to look: the directory log in `internal/relay` (cwd log lookup by absolute row, `internal/relay/cwd.go`) and how `Hub.PathClick` picks the entry for the clicked row; decide what directory applies to a row that was printed before the first report on the row below it. Add a test in `internal/relay/pathclick_test.go` for a report that arrives on row 1 after output on row 0.
