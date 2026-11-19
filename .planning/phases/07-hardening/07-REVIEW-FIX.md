---
phase: 07-hardening
fixed_at: 2026-11-19T00:00:00Z
review_path: .planning/phases/07-hardening/07-REVIEW.md
iteration: 1
findings_in_scope: 12
fixed: 7
skipped: 5
status: partial
---

# Phase 7: Code Review Fix Report

**Fixed at:** 2026-11-19
**Source review:** .planning/phases/07-hardening/07-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 12 (CR-01, WR-01..WR-04, IN-01..IN-07)
- Fixed: 7 (CR-01, WR-01, WR-02, WR-03, WR-04 (comment only), IN-04, IN-07)
- Deferred: 5 (IN-01, IN-02, IN-03, IN-05, IN-06)

Verification ran in the agent's isolated worktree (`.claude/worktrees/agent-a23168798cf6c72b1`, branch `worktree-agent-a23168798cf6c72b1`, fast-forwarded to `gsd/phase-07-hardening` at b3c869a9 first), not in the main checkout.

## Fixed Issues

### CR-01: a query the regex misses wedges the session's hub and the styled tail

**Files modified:** `internal/relay/hub.go`, `internal/daemon/engine.go`, `internal/relay/emu_nohang_test.go` (new), `internal/daemon/engine_test.go`, `TESTING.md`
**Commit:** 322ac362
**Applied fix:** I added `relay.NewQuietEmulator`, which closes vt's reply pipe writer when it builds the emulator. Every reply then returns `ErrClosedPipe` at once. vt ignores that error at every one of its pipe writes (checked in the source), so `emu.Write` can no longer block, whatever bytes it gets. All three production emulators come from it: the hub's live emulator (`newLiveEmulator`, which is also used by the resize and alt-exit rebuilds) and the engine's styled-tail emulator. A grep found no other production `xvt.NewEmulator`; the remaining calls are in tests that feed known content.

This differs from the suggested drain goroutine on purpose. It closes the pipe writer, as asked, but does it once at construction instead of at teardown. So there is no reader goroutine to leak when an emulator is replaced, and nothing ever calls `emu.Read` or `emu.Close`. The #100 race (a concurrent `Read`/`Close` on `e.closed`, removed in 1376705e) cannot come back. The doc comments on `liveEmuQueryStripPattern`, `StripTerminalQueries`, the `emuMu` field and `recordFrameLocked` now say that the strip keeps replies out of the shared shell during a replay, and the closed pipe prevents hangs. The live feed still runs the strip, which keeps `?2048` out of `h.modes`.

**Tests:**
- `TestHub_UnstrippedQueryNeverWedges` feeds `\x9bc`, a DA1 split across two PTY reads, `ESC[?2004;2048h` and a seeded 2 MiB random blob through a running hub. It then checks that the next output is broadcast and that Join and a host resize return within 5 s. All four subtests failed (wedged) before the fix.
- `TestGetSessionStyledTailLines_UnstrippedQueryNeverHangs` covers the styled-tail path. With only the engine change reverted, all three subtests fail.

### WR-03: a resize in the alt screen loses the main screen and bypasses HARD-01

**Files modified:** `internal/relay/hub.go`, `internal/relay/altscreen_heap_test.go`, `TESTING.md`
**Commit:** 741317b7
**Applied fix:** When the screen is in the alt buffer, `resizeLiveEmulator` now does four things:
- writes `ESC[?1049l` into the old emulator and clears the `altExited` flag that write raises;
- pushes the main rows to history, the same way a main-screen resize does;
- builds the new emulator;
- writes `altScreenEnterSeq` into it.

The TUI's redraw therefore stays out of history, and the app's own exit still fires the HARD-01 rebuild. `TestAltScreenResize_KeepsMainScreenAndExitRebuild` failed before the fix.

### WR-01: peer-tab snapshot restore drops hidden modes and can restore a torn state

**Files modified:** `frontend/src/lib/peerSnapshots.ts`, `frontend/src/components/TerminalPanel.tsx`, `frontend/src/lib/__tests__/peerSnapshots.test.ts`, the `TerminalPanel.tabSwitch`, `.scale`, `.reconnect` and `.guestClose` tests (mock `parser` stubs only), `TESTING.md`
**Commit:** 6c37c737
**Applied fix:** `watchPeerTerm(term)` uses xterm's public parser hooks to track the state that `SerializeAddon` 0.14 leaves out and xterm's public API does not expose: cursor visibility `?25`, the SGR mouse encodings `?1006`/`?1016`, the alt buffer, and DECSTBM per buffer. It is cleared on RIS and DECSTR. The snapshot is `serialize()` plus the re-emitted modes. `serialize()` already covers mouse tracking and bracketed paste, and the tests confirm both survive.

The watch also follows the output bytes with a code-point state machine (ESC, CSI, OSC, DCS/SOS/PM/APC, C1, UTF-8). If the last read ended mid-sequence, or a non-full scroll region is live, no snapshot is stored, and the returning tab dials without a resume and gets the full replay.

**Tests:** in real xterm.js, a restored tab has SGR encoding, a hidden cursor, mouse tracking and bracketed paste back. Seven torn tails each veto the snapshot, as does a scroll region. TerminalPanel stores nothing after a `$ \x1b[3` frame.

### WR-02: the alt-exit rebuild loses emulator state

**Files modified:** `internal/relay/hub.go`, `internal/relay/altscreen_heap_test.go`, `TESTING.md`
**Commit:** 57ffaae0
**Applied fix:**
- **Region and saved cursor:** new handlers track the main screen's DECSTBM region (CSI r) and DECSC saved cursor (ESC 7, CSI s without DECLRMM, `?1048h`, and the `?1049` save). They are cleared on RIS and on a resize rebuild.
- **Pen and charset:** the rebuild prints one probe cell into the old emulator, which is about to be discarded, and reads the current pen and DEC line-drawing G0 from that cell.
- **Re-seeding:** it re-applies all four after the rendered seed.
- **Cut-off reads:** it carries over the bytes the read cut off (`incompleteTail`: an unfinished escape sequence or UTF-8 character after the last ground-state point), so the next read completes them.

`TestAltScreenExit_RebuildMatchesReference` reproduces the reviewer's probe cases: mid-CSI, mid-UTF-8, inside an OSC, red pen, shell region, DECSC, and DEC line drawing. It checks that the rebuilt emulator matches one fed every byte. `TestIncompleteTail` covers the scanner.

A `shortcut:` comment records what is still lost: tab stops, G1-G3/SO, DECSC's saved pen, and a pending wrap after a region or saved-cursor restore.
**Status:** fixed: requires human verification (logic change in the state-rebuild path)

### WR-04: the HARD-01 bound is per session and scales with width

**Files modified:** `internal/relay/hub.go`
**Commit:** 9eeb38bb
**Applied fix:** comment only, as instructed. The `shortcut:` comment in `newLiveEmulator` now gives the cost per session and by width: 10,000 × cols × about 112-120 B. That is about 128 MiB at 100 columns and about 270 MiB at the 220-column fallback, or 0.4-0.8 GiB for three overnight `less +F` sessions. It also says when and how to bound it.

### IN-04: the replay strip lets some answers through

**Files modified:** `internal/relay/hub.go`, `internal/relay/querystrip_test.go`, `TESTING.md`
**Commit:** 028adbe6
**Applied fix:** the OSC 10/11/12 alternative now takes `(?:;\?)+`, so it matches `OSC 10;?;?`. XTWINOPS now covers `13/14/16/18/19/21 t`, with 21 t being the title report. I added 6 strip subtests and 2 keep subtests (title pop `23;0 t` and an OSC 10 colour set).

Not done: dropping the 0x01 bytes from the raw replay. That changes replay byte offsets and was not in the requested scope.

### IN-07: testmain ignores errors and skips cleanup

**Files modified:** `internal/daemon/testmain_test.go`
**Commit:** b32e735d
**Applied fix:** the Setenv errors now panic. The temp HOME is removed by a defer in `runInTempHome`, which returns the exit code to `os.Exit`. A comment notes that `XDG_CONFIG_HOME` only matters on Linux.

The fix is partial by nature: a panic inside a test's own goroutine still kills the process before any defer runs, so the comment says so.

## Skipped Issues

### IN-01: launch respawn grace guards an unreproduced blip

**File:** `app.go:310-349`
**Reason:** deferred. This is a necessity finding, and whether to keep or remove the grace is the user's scope decision. The `now`-before-`Health()` skew is harmless, as the review itself notes.
**Original issue:** the grace has never fired outside unit tests, and its time is judged from before `Health()`.

### IN-02: peerSnapshots caps entries, not bytes

**File:** `frontend/src/lib/peerSnapshots.ts:18-36`
**Reason:** deferred. Bounding by bytes and evicting on tab close needs a byte budget and a close hook from App.tsx's tab model, which is more than a trivial change. WR-01 already stores fewer snapshots, because torn or region-holding states store none.
**Original issue:** eight full serializations can hold hundreds of MB, and a closed tab's content stays.

### IN-03: deferred putPeerSnapshot races a quick remount

**File:** `frontend/src/components/TerminalPanel.tsx:559-565`
**Reason:** deferred. A pending-promise handshake is a design change. The race only costs a full replay plus a stale but consistent entry, never wrong content.
**Original issue:** a remount before the write callback finds no snapshot.

### IN-05: splitting a paste removes atomicity on a socket drop

**File:** `frontend/src/lib/relayClient.ts:528-533`
**Reason:** deferred. The fix is a UX notice in the reconnect banner, not a trivial change, and the 32 KiB input limit must stay.
**Original issue:** a drop mid-paste delivers a prefix with no indication.

### IN-06: the alt-exit seed writes width modes after the text

**File:** `internal/relay/hub.go` (renderScreenLocked)
**Reason:** deferred. Reordering `?2027` changes the shared rendered replay that Join sends to every viewer, which needs its own fidelity test. WR-02's reference test passes for the cases covered. Only emoji/ZWJ rows under `?2027` are affected.
**Original issue:** a wide-grapheme row can wrap and scroll the seed by one line.

## Gate

The gate ran in the isolated worktree:
- `go build ./... && go vet ./...`: clean
- `go test -race -short ./...`: all 12 packages ok on the first run, no failures and no reruns (relay 85.8 s, daemon 27.1 s, root 31.6 s)
- `pnpm vitest run`: 90 files, 1232 tests passed
- `pnpm exec tsc --noEmit`: clean
- `bash tests/check-traceability-paths.sh`: OK, 147 paths
- `bash tests/check-brand.sh`: clean

---

_Fixed: 2026-11-19_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
