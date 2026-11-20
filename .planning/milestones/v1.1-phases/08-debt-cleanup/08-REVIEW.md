---
phase: 08-debt-cleanup
reviewed: 2026-11-19T20:29:42Z
depth: standard
iteration: 3
files_reviewed: 10
files_reviewed_list:
  - TESTING.md
  - internal/daemon/process.go
  - internal/daemon/process_test.go
  - internal/relay/charstate.go
  - internal/relay/fuzz_test.go
  - internal/relay/history_test.go
  - internal/relay/hub.go
  - internal/relay/replay_fidelity_test.go
  - internal/relay/softwrap_test.go
  - internal/relay/wraps.go
findings:
  critical: 1
  warning: 0
  info: 4
  total: 5
  necessity: 0
status: issues_found
---

# Phase 8: Code Review Report (iteration 3)

**Reviewed:** 2026-11-19T20:29:42Z
**Depth:** standard, plus cross-file reads: `app.go` (startup, `RetryDaemon`, `respawnDaemonIfDown`, `QuitAll`), `internal/daemon/api.go` `handleShutdown`, `client.go` `ShutdownDaemon`/`GetRelayPort`, `process_unix.go` `lockHeld`/`lockDaemon`, the replay boundary in `hub.go`, `History.Render`, and the pinned vt `b57e5e6d29bb` (`handlers.go` registration and the RI/IL/DL/SD/ECH/DCH handlers, `cc.go` `reverseIndex`, `screen.go` `InsertLine`/`ScrollDown`/`Reset`)
**Files Reviewed:** 10
**Status:** issues_found

## Summary

Scope: `git diff a7d5822d..HEAD -- . ':!.planning'` (HEAD d0a6f510), the iteration-2 fixes listed in 08-REVIEW-FIX.md "Iteration 2". I read every production change in full, along with the code it connects to. Probes ran as overlay test files from the scratchpad, one relay binary and one daemon binary, with targeted `-run` filters. They are cited as R1, R2 (relay) and D1 (daemon). I ran no full suites. These committed tests passed in the same runs: `TestSoftWrap_ED2KeepsWraps`, `TestSoftWrap_IdenticalRowsTradeWrap`, `TestSoftWrap_RedrawKeepsHostLayout`, `TestSoftWrap_CursorMovesAreNotWraps`, `TestHistory_EndWrapClearedWhenRow0Erased`, `TestCharState_Scan`, `TestStartDaemon_WaitsForOldDaemonLock` (3 runs) and `TestWaitForDaemon_LockReleasedWithoutBindFailsFast` (3 runs).

Results:
- WR-06/IN-09, IN-10, IN-11, IN-12 and the WR-02 cap change are correct.
- WR-07 brings in a regression on the exact path it was meant to fix (CR-01). In the version-mismatch restart, the lock-wait loop's "socket answers" shortcut accepts the old daemon, which still answers for the 50 ms before its `os.Exit`. So `EnsureDaemon` returns success, nothing is spawned, and no daemon is left (probe D1). This happens on every upgrade while a daemon is running. The new test does not catch it because its old daemon never answers.
- Four Info items: R1 (a redraw's extra count defeats the IN-02 clear), R2 (an IN-11 sibling), a deadline edge in `startDaemon`, and a TESTING.md command nit.

**Unchanged surfaces confirmed:** `git diff --stat a7d5822d..HEAD` is empty for `internal/webserver/` (CSP, share-token and capability checks, the WR-07 tailnet guard), `internal/relay/protocol.go` (`MaxInputPayload = 32767`), `internal/relay/scrollback.go`, `internal/relay/server.go`, `internal/daemon/remote_ws_proxy.go`, `internal/daemon/api.go`, `internal/attach/`, `main.go`, `app.go`, `go.mod`/`go.sum`, `frontend/` and `web/`. The 32 KiB limits, share tokens, CSP and the WR-07 tailnet guard are unchanged. The `maxLinkField` raise to 2083 still rejects every C0/C1/DEL byte and invalid UTF-8 (`safeLinkField` is otherwise unchanged), so the only effect is that links of 2049-2083 bytes now survive a replay.

## Status of iteration-2 findings

| ID | Title (short) | Status | Notes |
|----|---------------|--------|-------|
| WR-06 / IN-09 | Line-keyed wraps misalign after ED 2 and row moves | **Fixed** (accepted trade documented) | `wraps.go` matches the pre-iteration-1 multiset (2caeea0b) byte for byte apart from comments. All four call sites (`CursorPosition` add, drain `take`, resize `take`, `screenRowWrappedLocked` `has`) use `rowKey` only. `absLine` and `wrapBase` are gone and nothing references them. **Q1/Q2 (Ctrl+L):** `TestSoftWrap_ED2KeepsWraps` passes in one read and in two. A hash follows its row through vt's empty-row-skipping `ClearWithScrollback`. **WR-03 motion rule kept:** the `notWrap` loop and the `CursorPosition` predicate are unchanged, and `TestSoftWrap_CursorMovesAreNotWraps` passes. **Host-grid layout:** a mark only ever applies where `lineFull` holds at all three sites. `History.Render` (`history.go:87`), the history-to-screen boundary (`hub.go:1495-1497`, which needs row 0 non-empty) and `screenRowWrappedLocked` (`next != ""`) all keep the CRLF before an empty row. So a traded mark joins a full row to a non-empty row, which autowrap puts on the same rows at the host width. No path changes the host-grid layout. Only reflow and copy see a traded mark, as the `shortcut:` says. **IN-02/IN-10 interplay:** `row0ErasedLocked` now takes one count of the newest emu-scrollback line's hash. That is correct for a single mark, but a redraw's extra count defeats it; see IN-13. |
| WR-07 | Lost-lock exception checked once | **Fixed in `waitForDaemon`; regression in `startDaemon`**, see **CR-01** | `waitForDaemon` now re-checks `lockHeld` on every poll after `exited` and fails once the lock is free and nothing answers. `TestWaitForDaemon_LockReleasedWithoutBindFailsFast` passes in about 0.21 s. **Held-lock detection:** `lockHeld` = `lockDaemon(path, 0)`, which is `flock(LOCK_EX\|LOCK_NB)` on a new open file description. `EWOULDBLOCK` means held, any other error means "not held", and a successful try is released at once by `f.Close()`. It never breaks a held lock. A daemon starting at that instant retries for 2 s (`process.go:42`). **Races:** two GUIs or CLIs that both see the lock free both spawn. The daemons' own `lockDaemon` lets only one bind. The loser exits after 2 s, and its caller's `waitForDaemon` sees the lock held and waits for the winner's socket. A caller that sees the lock held never spawns and returns once the holder answers. Probes only take the lock for microseconds, so they cannot starve a starting daemon. **Main thread:** `EnsureDaemon` runs only from the `OnStartup` goroutine (`domReady` is still bounded by `startupWait` = 5 s), from the `RetryDaemon` binding, from the poller goroutine and from the CLI. `QuitAll` (a binding, so on a goroutine) does not call it and only waits out a running respawn on `daemonMu`, the same as before. The 15 s total is shared between the lock wait and the spawn wait. No UI path blocks the main thread. **The broken path:** the version-mismatch branch dropped its 200 ms sleep, and the lock-wait loop returns `nil` as soon as `daemonReady` passes, which the dying old daemon satisfies (CR-01). |
| IN-10 | End-wrap clearing missed RI/IL/SD/ECH/DCH | **Fixed** | vt's `RegisterEscHandler`/`RegisterCsiHandler` append (`handlers.go:86-91`), so the second ESC `M` and CSI `L` handlers sit beside `notWrap` and replace nothing. They run before vt's, and both return false. `regionAtTop` (`mainRegion[0] <= 1`, 1-based, `{}` = full screen) matches vt's preconditions: `reverseIndex` scrolls only at `scroll.Min.Y`, `InsertLine` only inside the region, and `ScrollDown` homes to the region's top. vt resets the region only in RIS (`esc.go:25`, `Screen.Reset`), which the hub mirrors, so `mainRegion` cannot go stale. **Over-clearing:** none of practical weight. The only cases that clear without vt changing row 0 are `CSI 0 L`/`CSI 0 T` (vt treats an explicit 0 as a no-op, while xterm.js treats it as 1) and IL/RI with DECLRMM margins excluding column 0. Both are vanishingly rare, and an extra clear only loses a reflow join. The nine new history cases pass in one read and in two. |
| IN-11 | GL scan missed SO/SI after a string-ending ESC | **Fixed** | `scanStrEsc` is now in the SO/SI state list. vtScan moves `scanStrEsc` to `scanEsc` and re-steps the byte, which stays in `scanEsc` for C0 (`hub.go:676-682`, `:662`). That matches xterm.js (ESC ends the string, and C0 runs in ESCAPE). The reverse (`SO … ESC]0;t ESC SI`) now gives `gl1 == false` (R2). A follow-on `c` is still missed; see IN-14. |
| IN-12 | WR-01 lacks a VM pass condition; M-101 stated it as fact | **Fixed** | M-103 has a measurable PASS rule (CPU sampling, sha256/`cmp`, an AX read of the textarea, regression pastes), and its _Last run_ matches 08-VM-CHECKS P-01/P-02 (present at HEAD via 9dd3fde5). M-101 step 1 now defers to M-103, and its _Last run_ cites the re-run. `tests/check-traceability-paths.sh`: 150 paths OK. One wording nit: IN-16. |
| WR-02 follow-up | Link field cap raised to 2083 | **Correct** | The check is `len(s) > maxLinkField`, so 2083 is inclusive, as `TestFixRenderedLinks_DropsUnsafe` pins. The 4 MB OSC buffer bound still applies before it. No security impact. |

## Critical Issues

### CR-01: WR-07's lock-wait accepts the dying old daemon, so the version-mismatch restart returns success without spawning anything (regression)

**File:** `internal/daemon/process.go:125-131` (the mismatch branch now goes straight from `ShutdownDaemon` to `startDaemon`), `:168-177` (the `for lockHeld(...) { if daemonReady(client) { return nil } … }` loop); `internal/daemon/api.go:281-290` (`handleShutdown`: replies 204, then `os.Exit(0)` 50 ms later); `internal/daemon/process_test.go` `TestStartDaemon_WaitsForOldDaemonLock` (its old daemon is a refusing socket)

**Issue:** `ShutdownDaemon` returns once the 204 arrives. The old daemon keeps serving and keeps `daemon.lock` until its `os.Exit` 50 ms later. `startDaemon` runs within microseconds: `lockHeld` is true, so it calls `daemonReady`, and `/health` plus `/relay-port` from the old daemon both succeed. `startDaemon` returns `nil` without calling `spawn`. 50 ms later the old daemon is gone and nothing replaces it. The 200 ms sleep this replaced used to cover that window.

**D1 (probe, 3 of 3 runs):** an in-process stand-in for the old daemon holds the lock and answers `/health`, `/relay-port` and `/shutdown` like `handleShutdown`. On `/shutdown` it closes the server and releases the lock after 50 ms. After `client.ShutdownDaemon()`, `startDaemon(..., spawn)` returned `err=<nil> spawned=false` in 0.13-0.6 ms. 200 ms later `Health` failed and `lockHeld` was false.

Effect on every upgrade while a daemon is running (the daemon outlives the GUI, so this is the normal upgrade case):
- **GUI:** startup "succeeds", so `launchedAt` is set and `applyStoredAppearance` reads from a daemon that is exiting. The frontend's `GetDaemonError` returns "" and its first calls fail. `respawnDaemonIfDown` ignores failures for `respawnLaunchGrace` (10 s), so the app runs with no daemon and no banner for 10-15 s until respawn attempt 1. That attempt also uses up a slot in the respawn streak.
- **CLI:** the first `portty <cmd>` after an upgrade fails with a connection error, because `EnsureDaemon` returned nil.

The fix report says this path is fixed and lists it as "not VM-run on a real version-mismatch restart". The unit test cannot catch it because its old daemon never answers.

**Fix:** a daemon that is answering only counts if it is the version this binary wants. Check that in the readiness predicate (harmless after the spawn too, since by then the old daemon has let go of the lock), and pin it with a test whose old daemon keeps answering until it releases the lock:

```go
// daemonReady reports whether the daemon at client is fully ready (health,
// relay port) and, in a release build, is this build's version: a
// version-mismatched daemon told to stop still answers until it exits.
func daemonReady(client *DaemonClient) bool {
	var h HealthResponse
	if err := client.doJSON(http.MethodGet, "/health", nil, &h); err != nil {
		return false
	}
	if BuildVersion != "" && BuildVersion != "dev" && h.Version != BuildVersion {
		return false
	}
	port, err := client.GetRelayPort()
	return err == nil && port > 0
}
```

Test sketch (D1 turned into a regression test, `BuildVersion` set for the test): an old daemon reporting version `"old"` holds the lock, answers until 50 ms after `/shutdown`, then releases the lock. The fake spawn takes the lock and serves version `BuildVersion`. Assert that `spawn` was called and that `startDaemon` returns nil with the new daemon answering.

## Info

### IN-13: A redraw's extra count defeats the IN-02/IN-10 clear while the wrapped row is still in vt's scrollback

**File:** `internal/relay/hub.go:1031-1036` (`row0ErasedLocked`, `case emu.ScrollbackLen() > 0`: one `take` of the newest line's hash), `:1021` (the drain `take`s again)

**Issue:** under the multiset, the `ScrollbackLen() > 0` branch removes one count of the newest emu-scrollback line's hash. A line editor's redraw of a wrapped row adds a second count (the accepted P6 trade). So the erase takes one count, the drain in the same read takes the other, and the line stays joined to the new row 0.

**R1:** at 20×5 the reads are: `ESC c a CRLF b CRLF c CRLF d CRLF` + 30 x's, then `CR ESC[A` + 30 x's (a redraw), then `CRLF×4 ESC[H ESC[2K $ prompt` in one read. Result: history `x20 (true)`, `EndsWrapped() == true`, row 0 `$ prompt`. The control without the redraw gives `false`. The two-read variant is correct, because `ClearEndWrap` acts on the history flag.

The erase can also consume the mark of a different identical wrapped row. The impact is reflow and copy only. At the host grid a full row joined to a non-empty row keeps its layout. It belongs to the documented trade's class, but the `shortcut:` comment does not cover it.

**Fix:** if it matters, mark the line by position, not by hash. In this branch, set `h.unwrapNewest = emu.ScrollbackLen()` (the 1-based index in the current emu scrollback). In `drainScrollbackLocked`, for that index, still `take` (so no count lingers) but push `wrapped = false`, then reset the marker. Otherwise, add a sentence to the `wraps.go` shortcut noting that the IN-02 clear is best effort under a redraw.

### IN-14: `ESC SO c` (also right after a string-ending ESC) leaves GL at G1 after the RIS

**File:** `internal/relay/charstate.go:104-110`

**Issue:** `c.esc = x == 0x1b` resets on the SO, so a following `c` is not counted as RIS. In xterm.js and in vt, a C0 control runs in ESCAPE and leaves the parser there, so `c` still dispatches RIS and GL goes back to G0. **R2:** `"\x1b]0;t\x1b\x0ec"` and `"\x1b\x0ec"` both leave `gl1 == true`. The ground form predates this iteration. IN-11 newly exposes the string form, which used to be correct by accident because the SO was ignored. The visible impact is near nil: after RIS the designations are cleared, so the suffix's SO selects an ASCII G1. It only shows if the host later designates G1 as line drawing while staying in G0. It needs malformed output.

**Fix:** keep the ESC flag across C0 controls that run in ESCAPE (CAN and SUB abort it):

```go
c.esc = x == 0x1b || (c.esc && x < 0x20 && x != 0x18 && x != 0x1a)
```

Add both R2 strings to `TestCharState_Scan` and to the `FuzzCharState` seeds.

### IN-15: A lock freed at the deadline spawns with a zero wait, and the timeout message reports the remaining budget, not the total

**File:** `internal/daemon/process.go:170-183`, `:225`

**Issue:** the deadline is checked only while the lock is held. If the lock frees in the last poll, `spawn` runs and `waitForDaemon(…, time.Until(deadline) ≤ 0, …)` returns at once with "did not become ready within 0s". The daemon then starts anyway, the GUI shows an error that the poller later clears, and the CLI fails. Separately, `wait.Round(time.Second)` in `waitForDaemon` is now the time left after the lock wait (for example "within 12s" on a 15 s budget). That misleads anyone reading daemon.log or the banner. Both are narrow and cosmetic.

**Fix:** after a lock wait, give the spawn a floor (for example `max(time.Until(deadline), 3*time.Second)`, which goes past 15 s only in that edge case), or skip the spawn and return the lock-held error once the deadline has passed. Report the total budget (`daemonReadyWait`) in the message.

### IN-16: M-103 step 1 hard-codes a 20,000,000-byte receiver for the 5 MB repeat

**File:** `TESTING.md` M-103 step 1

**Issue:** step 1 gives `head -c 20000000 > /tmp/<file>` and then says "Repeat with 5 MB in a home local tab". Followed literally, the 5 MB run's `head` never finishes, and the "file complete" PASS condition cannot be judged. The VM run used `head -c 5000000` (08-VM-CHECKS P-01).

**Fix:** write the receiver as `head -c P > /tmp/<file>` with P the payload size, or spell out `head -c 5000000` for the repeat.

---

_Reviewed: 2026-11-19T20:29:42Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
_Iteration: 3_
