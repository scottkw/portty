---
created: 2026-11-21
area: frontend
due: after Phase 12 gap closure
---
Text selection lands several rows above the mouse pointer, in both the desktop app and the web share view (user report, running v1.1.0). Copying the highlighted text doesn't reach the clipboard either.

Lead (unproven): `TerminalPanel.tsx` `recomputeScale` sets `term.element.style.transform = scale(s)` on any panel following a server grid. Guests always do; per INPT-06, so does a local tab that lost the grid to another viewer. xterm 6 maps the mouse with `getBoundingClientRect` (scaled) and unscaled cell size, so with s < 1 the row comes out too small, and the gap grows further down the screen. That fits "several rows above". The same wrong row also goes into the mouse reports sent to TUIs (Claude Code), which do their own selection.

Copy: the native Edit menu exists (`main.go:191`). Check whether the drag went to the TUI as mouse reports (mouse tracking on, so xterm made no selection to copy; on macOS Option-drag forces an xterm selection), or whether the selection is cleared by output or redraw. Also check whether the clipboard addon (OSC 52) is on for this tab.

Repro first: scaled panel (two viewers at different sizes), drag near the bottom, compare the selected row with the pointer row. Plain shell vs. Claude Code. Desktop and Safari.

## Resolution
Fixed at 7a37bfb4 (selection follows the pointer on a scaled pane; Option-drag selects under a mouse TUI) and filed here at 52d960a5.
Evidence: TESTING.md M-136 (PASS 2026-11-21, build from 7a37bfb4), frontend/e2e/selection-offset.spec.ts and frontend/src/lib/terminalScale.test.ts; debug record .planning/debug/resolved/selection-offset-copy.md.
Triage recorded by Phase 16 (16-03): no code change, nothing carried to v1.3.
