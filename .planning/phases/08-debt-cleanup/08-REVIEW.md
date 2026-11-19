---
phase: 08-debt-cleanup
reviewed: 2026-11-19T19:26:19Z
depth: standard
iteration: 2
files_reviewed: 17
files_reviewed_list:
  - TESTING.md
  - frontend/src/components/TerminalPanel.tsx
  - frontend/src/lib/__tests__/pasteGuard.test.ts
  - frontend/src/lib/pasteGuard.ts
  - internal/daemon/process.go
  - internal/daemon/process_test.go
  - internal/daemon/process_unix.go
  - internal/relay/charstate.go
  - internal/relay/fuzz_test.go
  - internal/relay/history.go
  - internal/relay/history_test.go
  - internal/relay/hub.go
  - internal/relay/replay_fidelity_test.go
  - internal/relay/softwrap_test.go
  - internal/relay/vtalt.go
  - internal/relay/wraps.go
  - web/assets/terminal.js
findings:
  critical: 0
  warning: 2
  info: 4
  total: 6
  necessity: 0
status: issues_found
---

# Phase 8: Code Review Report (iteration 2)

**Reviewed:** 2026-11-19T19:26:19Z
**Depth:** standard, plus cross-file reads of the pinned vt `b57e5e6d29bb` (`screen.go` `ClearWithScrollback`/`DeleteLine`/`InsertLine`/`ScrollUp`, `scrollback.go` `Push`, `handlers.go` ED/EL and the handler registry, `cc.go` `index`/`reverseIndex`, `emulator.go` `Scrollback`/`ScrollbackLen`) and xterm.js 6.0.0 (`Clipboard.ts`, `CoreBrowserTerminal.ts` `_initGlobal`, `EscapeSequenceParser.ts` transition table and OSC/DCS end handling)
**Files Reviewed:** 17
**Status:** issues_found

## Summary

Scope: `git diff 2caeea0b..HEAD -- . ':!.planning'` (HEAD 5e48b6f2), the fixes for 08-REVIEW iteration 1 as listed in 08-REVIEW-FIX.md. I read every production change in full, along with the code it connects to. Probes ran as one overlay test file in one relay test binary (scratchpad `zz_probe2_test.go`, `-run TestProbe2_`). They are cited as Q1..Q6. I ran no full suites and no daemon test binary. The WR-04 gap below is proven from the code.

Overall, most fixes are correct:
- WR-01, WR-02, WR-03, WR-05, IN-04 and IN-08 hold up.
- IN-01 fixes P3b and P6, but keying wraps by absolute line number brings in a regression. vt's ED 2 (`ClearWithScrollback`) pushes only non-empty rows, so after an ED 2 (zsh's Ctrl+L) the drain indices no longer match the line numbers. That causes missed joins and, when rows are identical, false joins (WR-06, probes Q1/Q2). Before this iteration the hash multiset handled ED 2 correctly.
- WR-04's lost-lock exception is checked only once. So the version-mismatch restart that WR-04 listed (an old daemon slow to stop) still waits the full 15 s (WR-07).
- IN-02 and IN-03 are fixed for the cases named, but each misses some siblings (IN-10, IN-11).

**Unchanged surfaces confirmed:** `git diff --stat 2caeea0b..HEAD` is empty for `internal/webserver/` (CSP, share-token and capability checks, the WR-07 tailnet guard), `internal/relay/protocol.go` (`MaxInputPayload = 32767`), `internal/relay/scrollback.go` (`ViewerReadLimit`), `internal/relay/server.go`, `internal/daemon/remote_ws_proxy.go`, `internal/attach/`, `main.go`, `app.go`, `go.mod`/`go.sum` and `frontend/package.json`. `relayClient.ts` (`encodeInputFrames`) is untouched. The only `web/` change is the 3-line paste listener in `terminal.js`, which is an external script and so does not touch the CSP. The 32 KiB limits, share tokens, CSP and the WR-07 guard are unchanged.

## Status of iteration-1 findings

| ID | Title (short) | Status | Notes |
|----|---------------|--------|-------|
| WR-01 | Large paste stays in xterm's helper textarea | **Fixed** (not VM-verified) | Checked against xterm 6.0.0. `_initGlobal` (`CoreBrowserTerminal.ts:342-344`) registers xterm's paste listener on the textarea during `open()`, so the guard added after `term.open` runs second. `handlePasteEvent` never reads `defaultPrevented` and calls only `stopPropagation`, so `preventDefault` changes nothing in xterm. IME is unaffected: composition events are not `paste`, and `CompositionHelper` reads `textarea.value`, which now stays clean. Drag-and-drop is a `drop` event, not `paste`. Middle-click paste (Linux-only path, `:361`/`:530`) and the context-menu paste (`rightClickHandler` moves the textarea under the mouse, and the paste still targets it) both reach xterm's handler first. `term.paste()` callers send no DOM event. A paste that targets `this.element` instead hits a non-editable element, so it has no default insertion. While reconnecting, `disableStdin` drops the paste, the same as before. `web/assets/terminal.js` gets the same correct guard. VM evidence is still missing; see IN-12. |
| WR-02 | OSC 8 tracker re-emits an unvalidated URI | **Fixed** | `safeLinkField` runs on all three emit paths: tracker `setLink`, the alt-exit probe resync, and `fixRenderedLinks`. `pendingWrapLocked` and `screenBodyLocked` go through `fixRenderedLinks`. vt link fields can never hold BEL or ESC, so `renderedLinkRE` always matches a vt-rendered link and an unsafe link cannot slip past the regex. Over-rejection check: spaces (0x20) pass, percent-encoding is ASCII and passes, non-ASCII/IDN hostnames pass (only U+007F-U+009F and below U+0020 are rejected), and `id=`/`key=value:key=value` params pass. The one real over-rejection is the 2048-byte cap, a little under VTE's 2083-byte URI limit. A longer link loses its link only in replays, not in the live view. Acceptable. |
| WR-03 | Soft-wrap detector records LF/IND/CUD + CHA as a wrap | **Fixed** | Every vt cursor-motion CSI and ESC 8/D/M gets a `notWrap` handler that runs before vt's (reverse registration order, `handlers.go`). Probe Q6: C1 IND (0x84) followed by a print records no wrap. The only unmarked second move from column W-1 to column 1 or 2 is a print, which is a real autowrap, or BS on a 3-4 column grid, as the comment says. ESC E is not registered in vt at all (pre-existing vt gap, not a wrap issue). |
| WR-04 | 15 s wait has no early exit when the spawned daemon dies | **Partially fixed**, see **WR-07** | `go cmd.Wait()` reaps the child on every path. In the GUI the goroutine lives as long as the daemon and is not a leak. In the CLI the process exits and the child is re-parented. No other code in the repo reaps children (`git grep` finds no `Wait4`/`SIGCHLD`/`signal.Ignore`), so `Wait` cannot get ECHILD early. `lockHeld` uses `flock(LOCK_EX|LOCK_NB)` on a new open file description. That reports a lock held by another process (or another fd in this one) as `EWOULDBLOCK` and never breaks it. When the lock is free it takes it for microseconds and closes the file, and a starting daemon's 2 s retry loop absorbs that. Exit racing a successful bind is covered: `ready()` is checked again after the exit. The gap is that the lock check runs once. |
| WR-05 | TESTING.md M-101 requires unreachable wording | **Fixed** | The step 1 and sub-run PASS rules now accept either wording, with M = ceil(P/1024). That matches `Math.ceil(totalBytes / 1024)` in `TerminalPanel.tsx:480`. The N-of-M rule moved to an optional variant, and "_Last run_" records the disposition. |
| IN-01 | Hash multiset false joins / redraw inflation | **Fixed for P3b and P6; regressions** in **WR-06** and **IN-09** | Line keys plus the hash check fix both probes. Resize clears the set and does not advance `wrapBase`, which is consistent because the new emulator restarts at the same base with an empty set. RIS and ED 3 clear the set. The alt-exit seed scrolls nothing and records nothing (`trackWraps` is false). History trimming does not touch the keys, and a `uint64` line counter cannot overflow. ED 2 and row-moving operations break the claim "a wrap is missed, never moved onto another row". |
| IN-02 | `EndsWrapped` survives an erase of row 0 | **Fixed for the listed erases**; gaps in **IN-10** | ED 0 from home, ED 1, and EL 0/1/2 on row 0 clear the end-wrap. The unflushed case drops the newest emu-scrollback line's key. ED 2 correctly keeps the join (row 0 is non-empty and is pushed right after the line). The alt screen is a no-op. RI at top, IL on row 0, SD, ECH and DCH still leave the join (Q4). |
| IN-03 | GL scan misses RIS after ESC-ended OSC; SO in cut-off UTF-8 | **Fixed for RIS and the UTF-8 case**; sibling gap in **IN-11** | `scanStrEsc` now counts for `ESC c`. Dropping `need == 0` matches xterm.js, which ends the character at a non-continuation byte, and matches vtScan, which resets `need` there. An SO or SI right after a string-ending ESC is still missed (Q5). |
| IN-04 | `altScreenOf` checks type, not which element is main | **Fixed** | `emu.Scrollback()` returns `scrs[0].Scrollback()` (`emulator.go:451`). In the current layout the check compares element 0 with itself and passes. With swapped roles, element 0 would hold the alt screen's own non-nil `NewScrollback` (`screen.go:26`) and the check would fail. `altScreenOf` runs before `SetScrollbackSize`, and the default scrollback is already non-nil then, so the `sb == nil` branch cannot trip by accident. |
| IN-05 | Paste into a slow PTY blocks the read loop; heartbeat drops viewer | **Deferred (todo filed)**, reasoning sound | `.planning/todos/pending/2026-11-19-paste-heartbeat-drop.md` exists. The three liveness checks named are real, so a heartbeat-only tweak would be partial. One correction for the todo: the writer-goroutine design does not have to change DEBT-05's ack. If the read loop puts the ping's pong on the same queue, behind the input it read, the writer can send it, and a pong still means "handed to the PTY". |
| IN-06 | Paste notice can fire for a complete paste | **Deferred**, acceptable | The review said no fix was required. |
| IN-07 | Snapshot host keying misses tab-replacing paths | **Deferred**, acceptable | The cost is memory, bounded by `PEER_SNAPSHOT_MAX` and `PEER_SNAPSHOT_MAX_BYTES`. |
| IN-08 | No fuzz target for the byte-level parsers | **Fixed** | `FuzzCharState` has 13 seeds and checks split-independence, the suffix control set, and safe emitted links. As the fix report says, it has no xterm.js reference model, so it would not have caught IN-11. |

## Warnings

### WR-06: Line-keyed wraps misalign after ED 2: vt pushes only non-empty rows, so wraps are missed and identical rows get false joins (regression from the IN-01 fix)

**File:** `internal/relay/hub.go:993-1001` (`drainScrollbackLocked` keys line `i` as `wrapBase+i`), `:1009-1011` (`absLine`), `:930-941` (the `J` handler does nothing for ED 2); vt `screen.go:97-107` (`ClearWithScrollback` skips rows `isLineEmpty` reports as empty)

**Issue:** The new keying assumes that scrollback index `i` holds the screen row whose absolute line number was `wrapBase+i`. vt's ED 2 breaks that assumption: `ClearWithScrollback` pushes only rows with a cell whose content is neither empty nor a space. Each empty row above a wrapped row shifts every later drain index down by one, so each pushed row takes the key of a different row. `take` also deletes on a hash mismatch, so the real wrap is consumed and lost.

- **Q1 (missed join):** `"$ a\r\n\r\n" + 30 digits + "\r\n$ "` at 20×6, then `ESC[H ESC[2J`. History flags: `[false false false …]`. Without the ED 2 (control): `["$ a" "" "0123…9"(true) "0123456789" …]`. The digits line has lost its join.
- **Q2 (false join):** `"\r\n" + 60×"-" + "\r\n$ "`, then `ESC[H ESC[2J` (in one read or two). History: `d(false) d(true) d(true) "$"`. `Render()` gives `--------------------\r\n----------------------------------------$`. The third, hard row is joined to the prompt, and the first row's real join is lost. The control (no ED 2) gives the correct `d(true) d(true) d(false)`. The pre-iteration hash multiset also got this right.

zsh's Ctrl+L (`clear-screen` sends `ESC[H ESC[2J` with no `ESC[3J`) triggers this whenever an empty row sits above a wrapped line on the screen. That is common. The effect is the same class as WR-03: lines merge or split after an xterm.js reflow, and copy loses or gains a newline. The new comment in `wraps.go` ("A wrap is missed, never moved onto another row") is wrong for this path.

**Fix:** re-key the screen rows in the hub's `J` handler before vt's ED 2 runs. The handler already runs first:

```go
// hub.go, in the 'J' handler, before `return false`:
if n == 2 && !emu.IsAltScreen() {
	h.rekeyForClearLocked(emu)
}

// rekeyForClearLocked: vt's ClearWithScrollback pushes only non-empty rows,
// so row y lands at scrollback index sbLen+idx(y); move each wrap to that line.
func (h *Hub) rekeyForClearLocked(emu *xvt.Emulator) {
	if len(h.wraps.n) == 0 {
		return // also keeps the writes below off a nil map
	}
	base := h.absLine(emu, 0)
	type mv struct{ line, key uint64 }
	var moves []mv
	idx := 0
	for y := range emu.Height() {
		k, had := h.wraps.n[base+uint64(y)]
		delete(h.wraps.n, base+uint64(y))
		if rowEmptyForVT(screenLine(emu, y)) { // new helper: vt's isLineEmpty rule (every cell "" or " ")
			continue
		}
		if had {
			moves = append(moves, mv{base + uint64(idx), k})
		}
		idx++
	}
	for _, m := range moves {
		h.wraps.n[m.line] = m.key // ring slots stay; eviction tolerates stale lines
	}
}
```

Pin it with Q1 and Q2 in `softwrap_test.go`. The expected flags are the control runs above.

### WR-07: WR-04's lost-lock exception is checked once, so a lock holder that never binds (a version-mismatched daemon still shutting down) still costs the full 15 s

**File:** `internal/daemon/process.go:190-200` (`exited = nil // handled once`, then `lockHeld` once); `:121-126` (version-mismatch restart: `ShutdownDaemon`, `Sleep(200ms)`, spawn); `process.go:42` (the spawned daemon waits only 2 s for the lock)

**Issue:** Once `exited` fires, `waitForDaemon` calls `lockHeld` exactly once. If the lock is held at that moment, the loop falls back to polling only `ready()` until the deadline and never looks at the lock again. WR-04 named this case explicitly: "`lockDaemon` failing after its 2 s when a version-mismatched old daemon is slow to stop".
1. `EnsureDaemon` tells the old daemon to shut down and spawns after 200 ms.
2. The old daemon's `api.Stop()` plus `engine.Manager().Shutdown()` (killing every PTY) take more than about 2.2 s, so the new daemon gives up on the lock and exits.
3. `lockHeld` is true, because the old daemon is still exiting.
4. The old daemon exits and frees the lock. No daemon is starting, but the GUI startup, Retry, Quit All and every CLI command still wait out the rest of the 15 s.

A holder that crashes before binding behaves the same way. This is no worse than before iteration 1, but the fix report's claim that "Quit All, Retry, the poller respawn and every CLI command all go through `EnsureDaemon`, so they all fail fast now" does not hold for this path.

**Fix:** keep checking the lock after the exit, and fail once it is free and nothing answers. For the version-mismatch path, also wait for the old daemon's lock to free before spawning:

```go
exitedSeen := false
for time.Now().Before(deadline) {
	if ready() {
		return nil
	}
	select {
	case <-exited:
		exited, exitedSeen = nil, true
	case <-time.After(50 * time.Millisecond):
	}
	if exitedSeen && !lockHeld(lockPath) && !ready() {
		return fmt.Errorf("EnsureDaemon: daemon exited during startup (see daemon.log)")
	}
}
// EnsureDaemon, version-mismatch branch: replace the fixed 200 ms sleep with
// "poll lockHeld(lockPath) every 50 ms for up to ~5 s" so the spawn starts
// after the old daemon let go and its 2 s lock wait is not wasted.
```

To test it: hold the lock with `lockDaemon`, pass a closed `exited` and a refusing socket, and release the lock after 200 ms. The call must fail in well under 1 s. Today it waits for the whole `wait`.

## Info

### IN-09: Rows moved without scrolling still move a wrap onto an identical row; the "never moved onto another row" comment is wrong

**File:** `internal/relay/wraps.go:13-18` (the claim); `internal/relay/hub.go:1009-1011`; vt `screen.go:321-337` (`InsertLine`), `:359-367` (`DeleteLine` pushes the region's top row to scrollback even when the region's top is above 0)

**Issue:** The hash check blocks only rows with different content. When rows shift without a matching scrollback change, the key of a wrapped row passes to whichever row now sits on its line. If that row is byte-identical, it is a false join.
- **Q3:** at 20×6, `d CRLF d d CRLF` (the P3b layout), then `ESC[H ESC M` (RI at top) or `ESC[H ESC[L` (IL). History: `"" d(true) d(false) d`. The hard row took the wrap and the real wrap was missed. The correct result is `"" d(false) d(true) d`. Pagers on the main screen do this (git's default `LESS=FRX` runs less without the alt screen, and scrolling back uses RI).
- **Q3b:** a DECSTBM region whose top is above row 0: vt pushes the region's top row into scrollback, so every row outside the region renumbers. A hard row identical to a wrapped row above the region took that wrap (`["xxxx…"(true) "" "xxxx…"(false) "xxxxx"]`). The hash multiset made the same mistake. For rows that are not identical this is now a missed join.
- A single read that scrolls more than vt's 10,000-line scrollback (only a flood of empty lines fits in 32 KiB) saturates `ScrollbackLen`, which shifts the keys of rows still on screen in the same way.

Same impact as WR-03 (reflow and copy), with rarer triggers.

**Fix:** correct the comment to say "missed, or moved onto a byte-identical row". If it matters, shift keys in the existing `L`, `M`, `S`, `T` and ESC `M` handlers for rows inside the region, and in the region-top case of `DeleteLine`.

### IN-10: IN-02's end-wrap clearing misses RI at top, IL on row 0, SD, ECH and DCH

**File:** `internal/relay/hub.go:930-948` (only `J` and `K` call `row0ErasedLocked`)

**Issue:** **Q4:** after the P7 setup (a 30-x line wrapped across the history/screen boundary), each of `ESC[H ESC M $ prompt`, `ESC[H ESC[L $ prompt`, `ESC[T ESC[H $ prompt`, `ESC[H ESC[20X $ prompt` and `ESC[H ESC[20P $ prompt` leaves `EndsWrapped() == true` with row 0 = `$ prompt`. The Join replay then joins the old line to the new prompt with no CRLF: the original IN-02 symptom by another route. RI at top is the pager scroll-back case from IN-09.

**Fix:** call `h.row0ErasedLocked(emu)` from the already-registered handlers in these cases:
- ESC `M` when the cursor is on row 0 and the region's top is 0;
- CSI `L` when the cursor is on row 0;
- CSI `T` when the region's top is 0;
- CSI `X` and CSI `P` when the cursor is at (0, 0).

Add these to `TestHistory_EndWrapClearedWhenRow0Erased`.

### IN-11: The GL scan still misses an SO or SI right after a string-ending ESC

**File:** `internal/relay/charstate.go:102-104`

**Issue:** **Q5:** `ESC]0;t ESC SO abc` and `ESC P $q ESC SO` leave `gl1 == false`. In xterm.js an ESC inside an OSC or DCS ends the string and moves to ESCAPE (`EscapeSequenceParser.ts:593/604`, `:757/782`), and C0 controls execute in ESCAPE (`:113`), so the viewer runs the SO. The tracker sees state `scanStrEsc`, which is not in its list. The reverse case (`SO … ESC]0;t ESC SI`) leaves `gl1 == true` where xterm.js has G0. This is the sibling of the IN-03 RIS fix, and it is rare because it needs a malformed `ESC SO`.

**Fix:** add `|| c.vt.state == scanStrEsc` to the SO/SI case, and add the Q5 strings to `TestCharState_Scan` and the `FuzzCharState` seeds.

### IN-12: WR-01 has no VM pass condition in TESTING.md, and M-101 states the unverified outcome as fact

**File:** `TESTING.md:1252` (M-101 step 1: "Since 08-REVIEW WR-01 the GUI's WebContent also returns to idle within a few seconds of the paste")

**Issue:** The vitest models WebKit's default insertion and cannot show that WKWebView honours `preventDefault` on the native Edit-menu paste, or that the caret-blink re-shaping stops. The fix report says WR-01 is not VM-verified. Standing rule 2 in CLAUDE.md says a behaviour that cannot be automated (native GUI) needs an M-item check, but M-101's PASS rule does not include one. The sentence reads as an observed result.

**Fix:** add to M-101 step 1's PASS rule, or as a new M-103: "after a 20 MB Cmd+V, WebContent CPU is idle within 5 s, and an AX read of `xterm-helper-textarea`'s value is empty". Reword the sentence as the expected outcome until a run records it.

---

_Reviewed: 2026-11-19T19:26:19Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
_Iteration: 2_
