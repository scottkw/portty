---
created: 2026-11-20
source: PTY leak debug session (.planning/debug/pty-fd-leak.md), "other problems found"
area: daemon / pty
---

## Problem
1. Dismissing a naturally exited (created, not adopted) session still sends SIGHUP to its old process group (`internal/pty/cleanup.go`), although the shell was already reaped. If macOS reused that pgid, an unrelated process group gets SIGHUP. Adopted sessions are guarded (`killAdopted` checks `exited`); created ones are not. Low odds, but it can hit an innocent process on a long-running Mac.
2. `Manager().Shutdown()` leaves exit watchers blocked in `cmd.Wait` on live shells; a later KillSession races them (seen in test cleanup; harmless in production because the process exits).
3. `closeSessionFiles` closes `HandoffFiles()` (dup + slave) and would miss go-pty's original master on a created session; today it is only called for adopted sessions.

## Solution
1. Skip the group signal once the session's wait has returned (mirror `killAdopted`'s `exited` guard); test with a reaped session.
2/3. Document or guard; fix only if a caller appears.
