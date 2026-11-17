---
phase: 04-one-click-share
fixed_at: 2026-11-17T20:35:00Z
review_path: .planning/phases/04-one-click-share/04-REVIEW.md
iteration: 1
findings_in_scope: 3
fixed: 3
skipped: 0
status: all_fixed
---

# Phase 4: Code Review Fix Report (gap closure 04-07..04-10)

**Fixed at:** 2026-11-17
**Source review:** .planning/phases/04-one-click-share/04-REVIEW.md
**Iteration:** 1
**Scope:** CR-01, WR-01, WR-05 only (user decision 2026-11-17)

**Summary:**
- Findings in scope: 3
- Fixed: 3
- Skipped: 0
- Deferred by user decision: 8 (WR-02, WR-03, WR-04, IN-01..IN-05)

## Fixed Issues

### CR-01: A read-only guest can fill the host's disk via one unthrottled log line per resize frame

**Files modified:** `internal/webserver/server.go`, `internal/webserver/revoke_test.go`
**Commit:** d7b16cb4
**Applied fix:** The share server's read pump now logs `viewer-diag: web resize ignored` only for the first resize frame on each connection. The end-of-connection `viewer-diag: input total` line now carries `resizes=<n>` and is also written when a connection sent only resize frames. New test `TestResizeFlood_LogsOncePerConnection` sends 50 MsgResize2 frames and checks for exactly one ignored-resize line and `frames=0 bytes=0 resizes=50`.

### WR-01: daemon.log is never rotated during a run, and every spawn attempt rotates away the previous run's log

**Files modified:** `internal/daemon/daemon_log.go`, `internal/daemon/daemon_log_test.go`, `internal/daemon/process.go`, `internal/daemon/process_unix.go`, `TESTING.md`
**Commit:** 1f380649
**Applied fix:**
- (b) The spawning parent (`openDaemonLog`) now only appends to daemon.log, so a duplicate or retried spawn adds its error line to the existing log and no longer rotates it away. Rotation moved into the daemon: `startDaemonLog` runs in `runDaemonCore` after `api.Start` has bound the socket, so only a daemon that is really starting rotates. The file so far (the previous run plus this run's startup lines) becomes daemon.log.1.
- (a) `startDaemonLog` installs a size-capped writer (10 MiB) on the `log` package. Past the cap, the writer renames daemon.log to daemon.log.1, opens a fresh 0600 file and dup2s it onto fds 1 and 2, so direct stderr writes and Go panics follow the rotation. Rotation only happens when the daemon's stderr is daemon.log (`os.SameFile`); a terminal or service-manager stderr is left alone.
- ErrorLog: not changed. The share server's `http.Server` has no `ErrorLog`, so its TLS handshake errors go through the default logger, which is now the capped writer, so their volume is bounded. Filtering them out would take a filter writer, and discarding `ErrorLog` outright would also hide handler panics that net/http recovers. Left as is.
- Tests: `TestOpenDaemonLog_RotatesAndIs0600` was replaced by `TestOpenDaemonLog_AppendsAndIs0600`: repeated spawns append, never create daemon.log.1, and the file is 0600. New `TestDaemonLog_RollsOverPastCap`: rollover at the cap, file contents of .1 and the new file, mode 0600, and a direct write on the descriptor lands in the new file. TESTING.md SHAR-06 row updated. No test files added, so the counts are unchanged.

### WR-05: After turning Share off with Tailscale off, the toolbar button can no longer close its own open popover

**Files modified:** `frontend/src/App.tsx`, `frontend/src/components/TabBar.tsx` (comment only), `frontend/src/components/__tests__/App.tailscale.test.tsx`, `frontend/src/components/__tests__/App.test.tsx`, `TESTING.md`
**Commit:** 5adb01e9
**Applied fix:** `shareDisabled={tailscaleOff && !activeShared && !sharePopoverOpen}`. Share-on is still impossible with Tailscale off: the popover's own `blocked = tailscaleOff && !shared && !busy` keeps the switch aria-disabled. The mounted test now also covers this sequence: Tailscale off, shared, open the popover, switch off. The switch becomes aria-disabled, the button (still enabled) closes the popover, then turns aria-disabled and cannot reopen it. `SetSessionShared` is called only once. The existing unshared case still checks that the button is aria-disabled and opens nothing. The test mock's share state now follows `SetSessionShared`. Before, it always returned `shared: true`, which re-checked the switch on the refetch after Share off. The source-inspection assertions in both App test files were updated, and so were the TESTING.md NET-01 and SHAR-04 rows. I confirmed the new mounted assertions fail against the pre-fix App.tsx.

## Deferred by user decision (not attempted)

- **WR-02:** The replay query strip misses several common reply-eliciting queries.
- **WR-03:** ViewerReadLimit's bound is estimated, not enforced.
- **WR-04:** Viewer input over 32 KiB still drops the session.
- **IN-01:** Stale doc comment above magicDNSLabel.
- **IN-02:** The grid line's `by=` is always "local".
- **IN-03:** copyEnd.side blames the read side for a write failure.
- **IN-04:** Microsecond timestamps only on the RunDaemon entry point.
- **IN-05:** The close error is extracted twice in the proxy end path.

## Verification

These checks ran in the isolated review-fix worktree (`.claude/worktrees/rf-04-*`, branch `gsd-reviewfix/04-*`). Frontend dependencies there came from `pnpm install --offline --frozen-lockfile`. The commits are then fast-forwarded onto `gsd/phase-04-one-click-share`, so the same tree can be checked from the main checkout.

- `go build ./... && go vet ./...`: clean.
- `go test -race -skip '^(TestAPIAppearance_GetPatch|TestAPITerminalFont_GetPatch)$' ./internal/webserver/ ./internal/daemon/`: ok.
- `cd frontend && NO_COLOR=1 pnpm exec vitest run`: 82 files, 1047 tests passed.
- `cd frontend && pnpm exec tsc --noEmit`: clean.
- `bash tests/check-traceability-paths.sh`: OK, 111 paths.

Not tested automatically: `startDaemonLog`'s `os.SameFile` gate and the dup2 onto the real fds 1 and 2. A test would have to redirect the test process's own stdout and stderr. The rollover mechanics are covered by `TestDaemonLog_RollsOverPastCap` on a stand-in descriptor. Live check: after the next daemon restart, daemon.log should start just after the `daemon: listening on` line, with the previous run in daemon.log.1.

---

_Fixed: 2026-11-17_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
