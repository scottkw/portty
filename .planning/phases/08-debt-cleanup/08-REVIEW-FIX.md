---
phase: 08-debt-cleanup
fixed_at: 2026-11-19T19:12:39Z
review_path: .planning/phases/08-debt-cleanup/08-REVIEW.md
iteration: 1
findings_in_scope: 13
fixed: 10
skipped: 3
status: partial
---

# Phase 8: Code Review Fix Report

**Fixed at:** 2026-11-19T19:12:39Z
**Source review:** .planning/phases/08-debt-cleanup/08-REVIEW.md
**Iteration:** 1
**Base:** 2caeea0b (`docs(08): code review`)

**Summary:**
- Findings in scope: 13 (all: 5 warnings, 8 info)
- Fixed: 10 (WR-01..WR-05, IN-01..IN-04, IN-08)
- Deferred: 3 (IN-05 with a todo, IN-06, IN-07)

The reviewer's probes (scratchpad `zz_probe_test.go`) became committed regression tests. Each one failed on the code before its fix: P1, the DCS variant and P2 for WR-02; P3a and the CUD variant for WR-03; P3b and P6 for IN-01; P7 for IN-02; P4 for IN-03. The exception is WR-04: its test needed the new `waitForDaemon` signature, so it could not compile before the fix, and the old code waited the full 15 s.

## Fixed Issues

### WR-02: The OSC 8 tracker stores and re-emits an unvalidated URI

**Files modified:** `internal/relay/charstate.go`, `internal/relay/hub.go`, `internal/relay/replay_fidelity_test.go`
**Commit:** f173f158
**Applied fix:** Added `safeLinkField`, which rejects C0/C1 controls (including UTF-8-encoded C1), invalid UTF-8 and anything over 2,048 bytes. It is applied in three places:
- the tracker (`charState.setLink`, called from `osc8`);
- the alt-exit rebuild's resync from the probe cell;
- `fixRenderedLinks`, which replaces an unsafe link with a close so its cells render unlinked.

Tests: `TestOSC8_UnsafeLinkDropped` covers P1, the DCS variant, C1 in params, and P2 (1 MiB, then `clear`, where the replay was 2 MB before the fix). It also checks that a safe link is kept. `TestFixRenderedLinks_DropsUnsafe` is the second test.

### WR-01: A large paste stays in xterm's helper textarea

**Files modified:** `frontend/src/lib/pasteGuard.ts` (new), `frontend/src/lib/__tests__/pasteGuard.test.ts` (new), `frontend/src/components/TerminalPanel.tsx`, `web/assets/terminal.js`
**Commit:** daefc750
**Applied fix:** `guardPasteInsert(term.textarea)` runs right after `term.open` and adds a `paste` listener that calls `preventDefault()`. Its remover runs first in the mount effect's cleanup. The browser web-share viewer got the same one-liner, because its vendored xterm is also 6.0.0.

Checked against xterm 6.0.0:
- `Clipboard.ts` `handlePasteEvent` calls `stopPropagation()` (not `stopImmediatePropagation`). It reads `clipboardData` synchronously, then `paste()` does `bracketTextForPaste` and `triggerDataEvent`.
- `CoreBrowserTerminal.ts:342-344` registers that listener on the textarea at `open`, so it runs before ours.
- Bracketed paste is unchanged, and so is the HARD-02 32767-byte splitting, which happens downstream in `sendInput`.

Test (real xterm in jsdom, with the WebKit default insertion modelled):
- Without the guard, a 5 MB paste stays in the textarea.
- With it, the textarea is empty and `onData` fires once with the paste.
- Bracketed paste is kept, and the remover detaches the listener.
- A source gate pins the wiring in TerminalPanel.

### WR-04: The 15 s wait has no early exit when the spawned daemon dies

**Files modified:** `internal/daemon/process.go`, `internal/daemon/process_unix.go`, `internal/daemon/process_test.go`
**Commit:** 516d8d4a
**Applied fix:** `startDetachedDaemon` no longer calls `Release()`. It runs `go cmd.Wait()`, which also reaps the child so it does not become a zombie, and returns an `exited` channel. In `waitForDaemon`, once that channel closes:
1. It does one final health and relay-port check.
2. It fails at once, unless another process holds `daemon.lock`.

That exception is a small, deliberate extension of the requested "one final health check, then fail fast". In that case the spawn lost the lock to a daemon that is still starting, either a concurrent EnsureDaemon's or the B-07/B-08 login race, so that daemon keeps the rest of the 15 s. Without it, the DEBT-06 slow-login case would regress whenever a second spawn races the first.

Quit All, Retry, the poller respawn and every CLI command all go through `EnsureDaemon`, so they all fail fast now.

Tests:
- `TestWaitForDaemon_DiedAtSpawnFailsFast` does a real spawn of `false`. It fails in about 20 ms.
- `TestWaitForDaemon_LostLockWaitsForHolder` checks that a spawn which lost the lock waits for the holder, which binds after 1 s.
- `TestWaitForDaemon_SlowStart` still passes (4 s bind, nil `exited`).

### WR-03 and IN-01: The soft-wrap detector records non-wraps; the hash multiset steals and inflates

**Files modified:** `internal/relay/hub.go`, `internal/relay/wraps.go`, `internal/relay/softwrap_test.go`
**Commit:** c7dd4a4d (one commit: both findings change the same wrap-recording code)
**Applied fix:**
- **WR-03:** every cursor-motion CSI (`A`-`I`, `L`, `M`, `Z`, `` ` ``, `a`, `d`-`f`, `r`, `s`) and ESC `8`/`D`/`M` gets a registered handler that forgets the last move. Registered handlers run before vt's own handler, so the command's own move never completes a wrap pair. C0 moves cannot be marked; of those, only BS reaches column 1 or 2 from the last column, and only on a 3- or 4-column grid.
- **IN-01:** wraps are now keyed by absolute line number, with the row hash kept as a check. The line number is the lines drained so far, plus the lines scrolled during this write, plus the screen row. The effects:
  - An identical hard row cannot take another row's wrap (P3b).
  - A redraw records its row once (P6).
  - A row moved without scrolling (IL, DL, RI, a partial scroll region) loses its wrap. A wrap can now be missed but never moved onto the wrong line.
- The misleading "never a false one" comment and joinLocked's identical-row shortcut note are gone.

Tests:
- `TestSoftWrap_CursorMovesAreNotWraps` covers P3a, CUD, IND+CUP, HPA, CUB and LF+CR. It also checks that autowrap after the same traffic is still recorded.
- `TestSoftWrap_IdenticalHardRowAboveWrap` covers P3b.
- `TestSoftWrap_RedrawCountsOnce` covers P6.
- `TestWrapSet` is re-pinned to line keys. A redraw of one line counting once is the intended change, not a weakening.
- Every existing soft-wrap, fixture and rebuild-fuzz test passes unchanged, and the fixtures did not need regenerating.

**Status:** fixed: requires human verification (logic change in the wrap detector)

### WR-05: TESTING.md M-101 still requires the "N of M KB sent" wording

**Files modified:** `TESTING.md`
**Commit:** e490881f
**Applied fix:** The step 1 pass rule now passes on a notice in either wording, with M = ceil(P/1024). The office file must be an exact prefix that never grows (nothing resent), and the tab must take typing again. The sub-run uses the same rule. The N-of-M rule moved to an optional browser-viewer variant. "_Last run_" records the V-05 disposition and that WR-01 has not been re-run on the VMs.

### IN-02: `History.EndsWrapped` survives an erase of screen row 0

**Files modified:** `internal/relay/hub.go`, `internal/relay/history.go`, `internal/relay/history_test.go`
**Commit:** bf6605e1
**Applied fix:** The hub's `J` handler (ED 0 from home, ED 1) and a new `K` handler (EL 1, EL 2, and EL 0 from column 0, all on row 0) call `row0ErasedLocked`. It drops the wrap of the newest line still in vt's scrollback (same read), or else clears the newest history line's flag (`History.ClearEndWrap`). It does nothing on the alt screen. ED 2 keeps the join, because it pushes row 0 into history right after the line.
**Test:** `TestHistory_EndWrapClearedWhenRow0Erased` covers P7 and four more erases, each fed in one read and in two. It also checks three cases that must keep the join.
**Status:** fixed: requires human verification (logic change)

### IN-03: The GL scan misses a RIS after an ESC-ended OSC, and an SO in a cut-off UTF-8 tail

**Files modified:** `internal/relay/charstate.go`, `internal/relay/replay_fidelity_test.go`
**Commit:** b237b77a
**Applied fix:**
- `x == 'c' && c.esc && (state == scanEsc || state == scanStrEsc)`.
- An SO or SI no longer requires `need == 0`: xterm.js ends a cut-off UTF-8 character at a non-continuation byte and runs it.
**Test:** `TestCharState_Scan` gains P4 (one read and across two), the DCS case, an ST-is-no-RIS guard and two cut-off UTF-8 SO cases.

### IN-04: `altScreenOf` checks the field type but not which element is the alt screen

**Files modified:** `internal/relay/vtalt.go`
**Commit:** a917d618
**Applied fix:** Element 0 must now hold the non-nil scrollback that `emu.Scrollback()` returns. Otherwise `altScreenOf` returns nil, and the hub logs the layout change and falls back to the alt-exit rebuild. The existing vtalt tests cover the positive path. No test builds a swapped layout, since that needs a fake vt type.

### IN-08: No fuzz target for the new byte-level parsers

**Files modified:** `internal/relay/fuzz_test.go`
**Commit:** 03d1bd0a
**Applied fix:** `FuzzCharState` has 13 fixed seeds (P1, the DCS variant, P4, a cut-off UTF-8 SO, an over-cap URI, an 8-bit OSC, an alt-screen visit and others). It asserts that:
- the scan state does not depend on how the reads are split;
- the replay suffix holds no C0 except ESC/BEL/SO/SI, no C1 and only valid UTF-8;
- every OSC 8 the replay or `fixRenderedLinks` emits has safe fields.

The gate runs only the seeds. A 25 s fuzz run (about 80,000 execs) found nothing. It does not compare GL against a full xterm.js reference model; that model would need to match xterm.js exactly at the edges to avoid false failures.

## Skipped Issues

### IN-05: A paste into a slow PTY reader blocks the read loop and the heartbeat drops the viewer

**File:** `internal/webserver/server.go:896-904`
**Reason:** deferred, with a todo: `.planning/todos/pending/2026-11-19-paste-heartbeat-drop.md` (commit 4bc7d452). There is no small fix, because three separate liveness checks drop the viewer:
1. the server heartbeat, whose pong is read only inside the blocked `Read`;
2. the daemon proxy's upstream heartbeat, whose `Ping` cannot be written while a data frame holds coder/websocket's write lock under TCP backpressure;
3. the browser watchdog's MSG_PING probe, which sits unread behind the paste.

A heartbeat-only tweak covers only the first. The fix that covers all three is a bounded per-connection input queue drained by a writer goroutine, so the pump keeps reading. That changes the input path's lifetime and the meaning of DEBT-05's ack (a pong would mean "read by the server", not "handed to the PTY"), and it needs an M-101 VM run.
**Original issue:** coder/websocket handles the pong only inside `Read`; while `WriteInput` blocks on a full PTY input queue the `Ping` times out after 15 s and `CloseNow` drops a healthy viewer mid-paste.

### IN-06: The paste notice can fire for a paste that fully arrived

**File:** `frontend/src/lib/relayClient.ts:369-375`, `:594-614`
**Reason:** deferred, no change. The review says no fix is required. The notice is conservative (it never claims "complete" falsely) and its copy says "may". The suggested refinement, a blocking pong with a timeout, touches the same ack path as IN-05 and should be designed with it.
**Original issue:** a dropped pong (full `sub.Msgs`) or a slow-consumer kick can show "Paste may have been cut short" for a complete paste.

### IN-07: Snapshot host keying does not cover the paths that replace tabs

**File:** `frontend/src/App.tsx:944`
**Reason:** deferred. The cost is bounded memory, not correctness:
- Orphaned entries are capped by `PEER_SNAPSHOT_MAX` and `PEER_SNAPSHOT_MAX_BYTES`.
- A stale entry stays self-consistent.
- `retryInit` only runs after a daemon error.

The `retryInit` drop is a few lines but needs an App-harness test. The equal-session-id-across-hosts half needs `webSessionTabId` keyed by host, which is a tab-identity change outside a review-fix iteration.
**Original issue:** `retryInit` replaces every tab without `dropPeerSnapshot`, and peer tab ids are keyed by session id alone.

## Verification

All gates ran in the isolated worktree (`.claude/worktrees/agent-addbc65f98ff21748`, branch `worktree-agent-addbc65f98ff21748`), with `GOTOOLCHAIN=go1.26.11`. Frontend dependencies were installed there with `pnpm install --frozen-lockfile --offline`. The results are reproducible from the same commit in the main checkout.

- `go build ./... && go vet ./...`: clean.
- `go test -race -short ./...`: all 12 packages ok (relay 74.5 s, daemon 24.7 s).
- Non-short `go test ./internal/relay` after WR-03/IN-01 and IN-02: ok. The rebuild fuzz is still at 0 mismatches.
- `pnpm vitest run`: 91 files, 1280 tests passed. `pnpm exec tsc --noEmit`: clean.
- `bash tests/check-traceability-paths.sh`: 150 paths OK. `bash tests/check-brand.sh`: clean.

Unchanged: the 32 KiB limits (`MaxInputPayload`, `encodeInputFrames`, `ViewerReadLimit`), share tokens, the CSP and the WR-07 tailnet guard. No file under `internal/webserver/` changed.

Not VM-verified: WR-01 (a 20 MB Cmd+V should leave WebContent idle within a few seconds, with an empty textarea value read through AX) and WR-04 at a real login.

---

_Fixed: 2026-11-19T19:12:39Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
