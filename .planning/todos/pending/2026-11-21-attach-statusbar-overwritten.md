---
created: 2026-11-21
area: cli
due: after Phase 12
---
`portty attach` status bar gets overwritten by full-screen apps (seen in v1.1.0: Claude Code's spinner line "Sautéing…" drawn over the bar, giving `sSautéing…kens-m6cbook-air-1574 | Ctrl-\ to detach | 3 viewers`).

Cause: the bar reserves the last row with DECSTBM rows 1..rows-1 (`internal/statusbar/bar.go:116`), but attach tells the remote PTY the full height: `cmd_attach.go:397-398` and `watchResize` send `rows`, not `rows-1`. The app draws on the bar's row; the bar only repaints once a second.

Fix: send `rows-1` (bar shown) in the initial resize frame and on every SIGWINCH; with `--status-top` the same. Check how the PTY grid is chosen when several viewers attach (grid owner / host grid logic) — the CLI's reduced height must actually win or the app still overdraws. Add a test that the resize frame carries rows-1 when the bar is on. Also consider: no way to turn the bar off (no flag).
