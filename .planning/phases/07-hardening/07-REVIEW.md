---
phase: 07-hardening
reviewed: 2026-11-19T04:15:27Z
depth: deep
iteration: 3
files_reviewed: 17
files_reviewed_list:
  - internal/relay/hub.go
  - internal/relay/altscreen_heap_test.go
  - internal/relay/querystrip_test.go
  - internal/tailnet/live.go
  - internal/tailnet/tailnet.go
  - internal/tailnet/sessions.go
  - internal/tailnet/tailnet_test.go
  - internal/webserver/server.go
  - internal/webserver/tailscale.go
  - internal/webserver/whois.go
  - internal/webserver/tailscale_test.go
  - internal/daemon/peer_client.go
  - internal/daemon/share.go
  - frontend/src/components/TerminalPanel.tsx
  - frontend/src/components/__tests__/TerminalPanel.tabSwitch.test.tsx
  - frontend/src/components/__tests__/TerminalPanel.hot-swap.test.tsx
  - TESTING.md
findings:
  critical: 0
  warning: 2
  info: 2
  total: 4
  necessity: 0
status: issues_found
---

# Phase 7: Code Review Report (iteration 3)

**Reviewed:** 2026-11-19T04:15:27Z
**Depth:** deep
**Files Reviewed:** 17
**Status:** issues_found

## Summary

**Scope.** `git diff f6b68df4..HEAD -- . ':!.planning'`, which covers fix commits d663e738..23bbb9b4. I checked it against the iteration-2 findings and the "Iteration 2" section of 07-REVIEW-FIX.md.

**Connected code I read:**
- the pinned `charmbracelet/x/vt@v0.0.0-20260615092313-b57e5e6d29bb`: `handleGrapheme`/`atPhantom`, `setAltScreenMode`, `Screen.SaveCursor`/`RestoreCursor`, the `Cursor` struct, `resetModes`, `Write`'s grapheme flush, and the ESC 7/8 handlers
- `@xterm/xterm` 6.0.0 `InputHandler.setModePrivate`/`resetModePrivate`
- `peerSnapshots.ts` `groundTracker`
- every production dialer and `local.Client` use

**How I probed.** I built one test binary with `go test -overlay` and reused it, so no source file changed. It ran:
- the fixer's fuzz generator extended with wide characters, combining marks, IND/RI/NEL, ICH/DCH/ECH, IRM, LNM, cursor moves, extended SGR and DECSTR (about 13,000 streams at 20x6, 21x6 and 80x24)
- a greedy minimiser over every divergence
- direct checks of `renderScreenLocked` and `stripRingQueries`

**Verdict.** WR-07, IN-04 and IN-09 are fixed and complete. WR-05's pen and charset probe now lands for every tail. The WR-06 rewrite is correct for every escape-sequence tail and for UTF-8 tails cut one byte short. Two new problems:
- **WR-09.** WR-06's pending-wrap probe assumes vt's CAN aborts a cut-off UTF-8 character. vt instead consumes the CAN as one of the character's bytes. Reads that end two or more bytes short of a character make the rebuilt emulator put the rest of the line on the wrong row, or drop a wide character.
- **WR-08.** Dropping `?1048` from rendered replays was correct but incomplete. The mode loop still runs after the cursor CUP, and `?6l`/`?6h` home xterm.js's cursor. After any RIS (the `reset` command), every rendered replay puts every viewer's cursor at the top left.

## Open findings from iteration 2: status

| ID | Title | Status | Notes |
|---|---|---|---|
| WR-05 | Pen/charset probe swallowed after a DCS introducer or mid-UTF-8 | **Fixed** | Escape tails: CAN returns vt to ground. The fixer's subtests and my checks cover ESC, ESC P, ESC P +, 0x90, APC, 0x9b and ESC LF. UTF-8 tails: the CAN does *not* abort (see WR-09), but the probe `q` is printed and read at whatever position the cursor reaches, so pen and G0 still come out right. I checked bold red and line drawing with tails `c3`, `e2`, `e2 94`, `f0`, `f0 9f`, `f0 9f 98`. The fix report's "CAN returns vt's parser to ground from every state" is false for UTF-8. |
| WR-06 | Saved-cursor block drops pending wrap, saves the wrong pen, ignores DECOM | **Fixed, with a regression (WR-09)** | Checked: (1) `seed`, `pos` and `inferred` are all read before any probe writes to `old`, and `old` is never read after the swap. The probes fire none of the hub's hooks (no `r`/`J`/`c`, no mode callbacks), and a probe-caused scroll lands in `old`'s scrollback, which is never drained. (2) The `CellAt` copies are taken before the next `Write`, so the aliasing fix is right. (3) vt's `setMode(ModeOrigin)` has no cursor side effect, so `?6l … ?6h` makes the CUPs and `ESC 7` absolute, as intended. (4) The `?1047h CUP x ?1047l` switch: vt's `atPhantom` is emulator-wide and `setAltScreenMode` leaves the main cursor alone, so the flag lands at `pos`. The stray `x` is cleared by the next `setAltScreenMode(true)` (`scr.Clear()`), and `h.altExited` and `h.modes[1047]` end where they started. Copying vt's shared flag is deliberate fidelity to vt: an un-rebuilt emulator would carry it too. (5) vt's `Cursor` holds Pen, Link, Position, Style, Steady and Hidden, but no charset, so G0 is not lost through DECSC. The extended fuzz is clean (0 mismatches) for the baseline, IND/RI/NEL, ICH/DCH/ECH, IRM, cursor moves, extended SGR and DECSTR. The pending-wrap detection breaks for UTF-8 tails cut two or more bytes short (WR-09). |
| WR-07 | Go test suites reach the real tailscaled and probe peers | **Fixed** | Details in the WR-07 notes below. |
| IN-04 | Replay strip lets split queries and CSI 11/15/20 t through | **Fixed** | Details in the IN-04 notes below. |
| IN-08 | SO-shifted line drawing and OSC 8 link lost in the rebuild | Deferred | Listed in the `shortcut:` comment. Unchanged. |
| IN-09 | Ground tracker not reset on the in-band reset | **Fixed** | Details in the IN-09 notes below. |

**WR-07 notes.**
- Production grep:
  - every `local.Client`: `tailnet.go:89`, `webserver/tailscale.go:76`, `webserver/server.go` `ws.lc` (WhoIs and `GetCertificate`), and `daemon/share.go:45`
  - every tailnet-peer dialer: `ProbePeer`/`probePeerByIP`, `FetchPeerSessionsMeta` and its IP fallback, and `API.peerClient`, which feeds `/peers/connect` and `remote_ws_proxy.go:118`
  - Each is behind the guard or reached only through one.
  - There is no `tsnet`.
  - The remaining dialers reach only the local daemon or relay: `cmd_attach.go:303` (remote attach goes through the local relay port), `attach/loop.go:417`, `daemon/client.go`, `ipc.go` and `socket.go`.
  - `updater` tests use `httptest`.
- `testing.Testing()` reads a linker flag that only `go test` sets, so it is false in the Wails or `go build` binary.
- `testing` was already linked into production by `daemon/process.go:149`, which uses the same guard, and by `webserver/capability_test_helpers.go`. The release binary gains no package, and since Go 1.13 `testing` registers no flags at init.
- With the guard passing, the production error paths are byte-for-byte unchanged.

**IN-04 notes.**
- `stripRingQueries` returns the same slice, same backing array, when nothing matches, so a query-free replay is byte-identical (checked).
- It drops only the headers inside a match, and a match cannot be empty or end on a header.
- The joined view is what xterm.js parses: a C0 inside ESC/CSI is executed and the sequence continues. So split matches are real queries.
- Cost on a full 256 KiB ring is about 2.3 MiB transient (an `[]int` per byte plus the view) and one linear regexp pass per raw join. That is acceptable.
- The XTWINOPS class `1[134-689]|2[01]` matches exactly 11/13/14/15/16/18/19/20/21.

**IN-09 notes.**
- A single `0x18` resets both `need` and `state` in `groundTracker`.
- xterm.js returns to ground on the `ESC c` from every state (ESC, CSI, OSC, DCS, SOS/PM/APC).
- The tracker is fed in order before the replay bytes, and `suffix()` runs after xterm drains its queue, so the two stay in step.

## Narrative Findings (AI reviewer)

## Warnings

### WR-08: Rendered replays restore DEC modes after the cursor CUP, so `?6l`/`?6h` homes every xterm.js viewer's cursor; any RIS triggers it

**File:** `internal/relay/hub.go:1052-1078`
**Issue:** `renderScreenLocked` writes `CUP pos` and the pending-wrap re-print first, and then the mode loop. xterm.js 6.0.0 moves the cursor on DECOM in both directions: `setModePrivate` case 6 and `resetModePrivate` case 6 each call `this._setCursor(0, 0)` (`InputHandler.ts:1901-1904`, `:2142-2145`). Any replay that carries mode 6 therefore leaves the viewer's cursor at the top left, not at `pos`.

`h.modes[6]` is filled by more than apps that use DECOM. vt's RIS runs `resetModes`, which calls `setMode` for every default mode and fires the hub's `DisableMode` callback for each. After one `ESC c`, `h.modes` holds `6:false`, along with 1, 7, 9, 25, 66, 69, 1000-1006, 1047-1049 and 2004.

The probe fed `ESC c` and then `line1\r\nline2\r\n$ ls`. The rendered replay ends `\x1b[3;5H\x1b[?1l\x1b[?6l\x1b[?7h…`, so the viewer's cursor goes to (1,1). An app's bare `?6h ?6l` gives `…\x1b[1;5H\x1b[?6l` with the same effect.

This is the same class as the `?1048` bug the WR-06 fix removed from this loop: an action replayed after the CUP. The fix left its sibling in place. It predates this diff, but it decides the answer to "can a viewer's cursor be wrong after a replay?": yes.

**Failure scenario:**
1. The user runs `reset`. ncurses `rs1` for xterm-256color is `\Ec…`.
2. Later, the 256 KiB ring wraps or the host window is resized, so Join takes the rendered path.
3. Every web or peer viewer that joins gets the screen with its cursor at the top left.
4. The shell's echo, and any relative redraw such as Claude Code's Ink frames, then draws over the top rows in the viewer, while the host shows it at the prompt. This lasts until something positions the cursor absolutely.

**Fix:** Restore the modes before positioning the cursor. Rendered replays send no DECSTBM, so the viewer's region is the full screen and a CUP under DECOM is still absolute. In vt (the rebuild seed), DECOM has no side effect and the seed's CUP runs before the rebuild sets a region, so the rebuild is unaffected:
```go
b.WriteString(strings.ReplaceAll(h.liveEmu.Render(), "\n", "\r\n"))
for _, m := range slices.Sorted(maps.Keys(h.modes)) { /* unchanged loop body */ }
pos := h.liveEmu.CursorPosition()
fmt.Fprintf(&b, "\x1b[%d;%dH", pos.Y+1, pos.X+1)
b.WriteString(h.pendingWrapLocked(pos)) // after ?7: pendingWrapLocked already requires autowrap on
```
Add a `renderScreenLocked` test: after `ESC c` plus text, the bytes after the last CUP contain no `?6`. Ideally, feed the replay to xterm.js in a vitest and assert `buffer.active.cursorX/Y`.

### WR-09: The pending-wrap probe misreads a read that ends two or more bytes short of a UTF-8 character, because vt takes the CAN as one of the character's bytes

**File:** `internal/relay/hub.go:685-695`
**Issue:** The probe assumes a cut-off UTF-8 tail plus `\x18` gives exactly one replacement cell followed by the printed `x`, so it sets `n = 2` whenever `!utf8.FullRune(tail)`. vt's parser does not abort UTF-8 collection on CAN. It counts the CAN, and then the `x`, as continuation bytes, and decodes when the count is reached. I wrote `"ab" + tail` and then `"\x18x"` into a fresh pinned vt:

| Tail (bytes of the character) | Result | Cells printed |
|---|---|---|
| `c3` (1 of 2), `e2 94` (2 of 3), `f0 9f 98` (3 of 4) | `ab�x` | 2, as assumed |
| `e2` / `e6` (1 of 3), `f0 9f` (2 of 4) | `ab�`, the `x` swallowed | **1** |
| `f0` (1 of 4) | `ab`, nothing printed; the parser still holds 1 byte and swallows the probe CUP's ESC | **0** |

So `pending := after.X == n && …` is wrong for three of the six cut shapes:
- **False pending.** The cursor is on column 1 with no wrap pending and the read ends after `e2`. `after.X` is 2, which equals `n`. The rebuild re-creates a wrap, and the rest of the line lands on the next row.
- **Missed pending.** The wrap is real, but `after.X` is 1, not 2. The block's CUP clears the seed's re-print, so the next character overwrites the last column. A wide character there is dropped.

End to end at 40x6 (minimised; the first read is the exit read, `probeRun` compares against a reference fed every byte):

| Reads | Rebuilt | Reference |
|---|---|---|
| `"a\e[?1049h\e[?1049l\xe2"`, `"\x94\x80b"` | `a` / `─b` on row 2 | `a─b` |
| `"\e[1;31m$ \e[?1049h\e[?1049l\xf0"`, `"\x9f\x98\x80b"` | `$ ` / `😀b` | `$ 😀b` |
| 40×`a` + `"\xf0\x9f"`, `"\x98\x80b"` | 39 `a`s, the emoji lost | 40 `a`s / `😀b` |

The pen and G0 survive (WR-05 holds), but the screen layout diverges for good. That corrupts every later rendered replay and `h.history`.

**Trigger:** the read that carries an alt-screen exit ends inside a 3- or 4-byte character the shell is printing after it. Examples are a starship or powerlevel10k prompt glyph (`❯` is `e2 9d af`, Nerd Font MD icons are 4 bytes), box drawing, CJK or emoji. The fixer's generator only produces 2- and 3-byte characters, and a false positive needs `pos.X == 1`, which is why its fuzz stays at 0.

**Fix:** Complete the cut-off character with exactly as many CANs as bytes are missing. The result is always one invalid sequence, so one U+FFFD of width 1, and the parser is back at ground for the `x` and the CUP. I verified that all seven tails above then give `ab�x` and a working `ESC[1;1H`:
```go
n, fill := 1, 1
if len(tail) > 0 && !utf8.FullRune(tail) {
	// vt counts CAN as a byte of an open UTF-8 sequence: complete it with
	// CANs (one U+FFFD), so the probe x and the CUP below start from ground.
	want := map[bool]int{tail[0]&0xe0 == 0xc0: 2, tail[0]&0xf0 == 0xe0: 3, tail[0]&0xf8 == 0xf0: 4}[true]
	n, fill = 2, want-len(tail)
}
old.Write([]byte(strings.Repeat("\x18", fill) + "x"))
```
Write a plain `switch` for the length; the map is only to keep the sketch short. Add subtests for tails `e2`, `f0` and `f0 9f`, with the cursor on column 1 and in pending wrap. Add 4-byte characters (and a 1-of-3 cut at column 1) to `rebuildFuzzStream`. Correct the WR-05 claim in the fix report.

## Info

### IN-10: The tail replay re-executes C0 controls the old emulator already ran inside the cut-off escape sequence

**File:** `internal/relay/hub.go:752` (with `incompleteTail`, `:580-649`)
**Issue:** A C0 control inside an ESC or CSI sequence is executed at once, and the sequence continues. `incompleteTail` correctly keeps such bytes in the tail, but the tail is then written into the new emulator, so they run a second time. Minimised:
- `"$ \e[?1049h\e[?1049l\e\n"` + `"[mok"`: the rebuilt cursor ends on row 3, the reference on row 2.
- `"…\e[3\b"` + `"1mok"`: the rebuilt cursor ends on column 0, the reference on column 1.

At the region bottom, the extra LF scrolls a line into `h.history`. CR is idempotent. It predates this diff (the WR-02 tail replay) and is rare, because shells seldom put a C0 inside a CSI.
**Fix:** Before the replay, drop C0 bytes other than CAN, SUB and ESC from a tail that starts in ESC or CSI. They have already acted on the old screen, which the seed captured. In OSC and DCS strings vt ignores them, so dropping them there is also safe:
```go
if len(tail) > 0 && (tail[0] == 0x1b || tail[0] == 0x9b) {
	tail = slices.DeleteFunc(slices.Clone(tail), func(c byte) bool { return c < 0x20 && c != 0x18 && c != 0x1a && c != 0x1b })
}
```

### IN-11: The seed cannot re-create vt's zero-width cells (orphan combining marks), and the `shortcut:` list omits them

**File:** `internal/relay/hub.go:678-681` (seed from `renderScreenLocked`)
**Issue:** vt prints an ASCII base character immediately, and turns a combining mark that arrives on its own (after a CUP, at the start of a line, or split from its base by a read) into its own cell. `Render()` emits that cell's mark right after the previous cell. Re-ingesting the render merges the mark into the previous cell or drops it. Examples from the minimiser:
- `"\e[4H́\e[4;2H$" + ?1047 visit`: the reference keeps the mark on row 4, column 1, and the rebuilt screen drops it.
- `"\e(0é"` + exit: the rebuilt screen shows `␊́␊`, the reference `␊␊`.

This was about 40 of 900 mismatches when combining marks were in the generator. The effect is cosmetic: one diacritic on the hub's screen and in rendered replays. macOS NFD filenames (`e` + U+0301) are the realistic source, and vt already mishandles those when a read splits them.
**Fix:** Add "zero-width (combining-mark) cells" to the rebuild's `shortcut:` comment. It is not worth fixing in the seed.

---

_Reviewed: 2026-11-19T04:15:27Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
_Iteration: 3_
