---
phase: 07-hardening
reviewed: 2026-11-19T00:06:05Z
depth: deep
files_reviewed: 19
files_reviewed_list:
  - internal/relay/hub.go
  - internal/relay/protocol.go
  - internal/relay/altscreen_heap_test.go
  - internal/relay/querystrip_test.go
  - internal/relay/input_limit_test.go
  - internal/relay/server_test.go
  - internal/daemon/engine.go
  - internal/daemon/engine_test.go
  - internal/daemon/remote_ws_proxy.go
  - internal/daemon/testmain_test.go
  - internal/daemon/api_test.go
  - internal/attach/attach.go
  - internal/attach/loop.go
  - app.go
  - app_respawn_test.go
  - frontend/src/lib/relayClient.ts
  - frontend/src/lib/peerSnapshots.ts
  - frontend/src/components/TerminalPanel.tsx
  - README.md
findings:
  critical: 1
  warning: 4
  info: 7
  total: 12
  necessity: 1
status: issues_found
---

# Phase 7: Code Review Report

**Reviewed:** 2026-11-19T00:06:05Z
**Depth:** deep
**Files Reviewed:** 19
**Status:** issues_found

## Summary

Scope: `git diff f54a223a..HEAD -- . ':!.planning'` on `gsd/phase-07-hardening`. I also read the code it connects to: the vendored `charmbracelet/x/vt` (handlers, mode switching, the response pipe), `@xterm/addon-serialize` 0.14, the relay and webserver input read pumps, `InputAbsorber`, the remote WS proxy, and the `App.tsx` tab mounting.

Constraints I checked that still hold:
- No input reader raises the 32 KiB limit. `relay/server.go` and `webserver/server.go` call no `SetReadLimit` on the inbound conn. The proxy's client side keeps the default, and a frame of exactly 32768 bytes passes (coder/websocket reads limit+1).
- Every sender splits input into frames of at most `MaxInputPayload` + 1. The split preserves byte order: frames go out in one synchronous loop on one socket, and the absorber keeps its state across frames.
- `capability_mw.go`, `csp_mw.go`, the share-token flow and `webserver/server.go` are untouched by the diff.
- `StripTerminalQueries` runs on Go's RE2 engine, so it can't backtrack catastrophically. I found no over-stripping: OSC 52 writes, OSC 4 sets, `CSI 8;r;c t`, `CSI 22;0 t`, sixel DCS, SGR and mode sets all pass through (probe-verified).
- Locking around the alt-exit rebuild is sound. Run, Join, RenderSnapshot and resizeLiveEmulator all serialize on `emuMu`, and the `AltScreen` callback only fires on a real screen transition (`csi_mode.go:39`).

The main problem is the live emulator's anti-hang design. It relies on a regex to strip every reply-producing query before `emu.Write`. vt honours several encodings the regex can't see, and nothing reads vt's unbuffered response pipe. In a probe, every 2 MB random file I tried, and Helvetica.ttc, permanently wedged an emulator fed the way `Hub.Run` feeds it (CR-01). The bug predates Phase 7. HARD-03 rewrote this function and its doc comment still promises the guarantee, so it is reported here.

The other findings are fidelity gaps in the two new state-rebuild paths: the hub's alt-exit rebuild (WR-02, WR-03) and the peer-tab snapshot restore (WR-01). There is also the per-session size of the HARD-01 bound (WR-04). I confirmed each claim with a scratch program against the pinned vt version or by reading the dependency source.

## Narrative Findings (AI reviewer)

## Critical Issues

### CR-01: BLOCKER: a reply-eliciting query the regex misses permanently wedges the session's hub (and the styled-tail call)

**File:** `internal/relay/hub.go:515` (also `internal/relay/hub.go:93-98`, `:494-498`, `:146-173`; `internal/daemon/engine.go:516`)
**Issue:** vt writes its answers to DA1, DA2, DSR 5/6, DECXCPR, DECRQM, OSC 10/11/12 `?` and the `?2048` set into an unbuffered `io.Pipe` (`vt/emulator.go:102`; writes at `handlers.go:695,712,806,809,826`, `csi.go:30`, `csi_mode.go:95`, `osc.go:87-97`). Nothing reads that pipe on the hub path. The only defence is `liveEmuQueryStripPattern`, which matches only 7-bit, contiguous, single-mode forms. I fed these through `StripTerminalQueries` into a vt emulator at the pinned version:

| Input | Result |
|---|---|
| `"\x9bc"`, `"abc\x9b6n"` (8-bit C1 CSI, which vt's parser honours) | HANG |
| `"\x1b"` then `"[c"`, `"hello\x1b["` then `"6n"` (query split across two PTY reads) | HANG |
| `"\x1b[?2004;2048h"` (2048 inside a mode list) | HANG |
| `"\x1b[0:0c"`, `"\x1b[6\x00n"` | HANG |
| Five 2 MB files of `/dev/urandom`, `/System/Library/Fonts/Helvetica.ttc`, in 32 KiB chunks | HANG on every one (16-20 DA1 replies `\x1b[?62;1;6;22c` per file) |

Failure scenario: on the office Mac, the user runs `cat build.zip` (or `curl` without `-o`, or `less -r` on a binary) in a shared session. `Hub.Run` blocks inside `liveEmu.Write` while it holds `emuMu`:
- no more PTY output is drained or broadcast, so the shell blocks once the PTY buffer fills;
- every new viewer blocks forever in `Join`, leaking a goroutine per attempt;
- the host's next resize blocks in `resizeLiveEmulator`. That call runs on the relay read pump, so input (including Ctrl-C) stops reaching the PTY too.

The session can't be recovered from the remote Mac. `GET /sessions/{id}/styled-tail` hangs the same way on any ring holding such bytes, and each call leaks a goroutine holding a ring copy. The doc comments at `hub.go:93-98` and `:494-498` say the strip *is* the anti-hang guarantee, but the guarantee doesn't hold.
**Fix:** Stop relying on pattern coverage. Give every emulator whose replies nobody wants a reader for its response pipe, and close it by the pipe writer (not `emu.Close`, so the #100 `e.closed` race doesn't return):
```go
// newLiveEmulator (and the engine's styled-tail emulator)
emu := xvt.NewEmulator(cols, rows)
go io.Copy(io.Discard, emu) // vt answers queries into an unbuffered pipe; drain it so Write never blocks
// ...where an emulator is replaced (resizeLiveEmulator, rebuildAfterAltExitLocked) or dropped:
if pw, ok := old.InputPipe().(*io.PipeWriter); ok {
	_ = pw.Close() // ends the drain goroutine without touching e.closed
}
```
If a goroutine per emulator is unwanted, register swallowing handlers instead. A handler that returns true stops the default (`handlers.go` handleCsi iterates newest first): `emu.RegisterCsiHandler(c, func(ansi.Params) bool { return true })` for `'c'`, `ansi.Command('>',0,'c')`, `'n'`, `ansi.Command('?',0,'n')`, `ansi.Command(0,'$','p')`, `ansi.Command('?','$','p')`, plus OSC 10/11/12 handlers that swallow `?`. The `?2048` set inside a list still needs the drain. Keep the regex for the replay path, where its job is different. Add regression tests for the 8-bit, split, list and random-binary cases against the hub (`recordFrameLocked` with a timeout).

## Warnings

### WR-01: WARNING: peer-tab snapshot restore (HARD-05) drops mouse encoding, cursor visibility, scroll region and parser state, so delta bytes render as garbage and mouse input is mis-encoded into the remote app

**File:** `frontend/src/components/TerminalPanel.tsx:405-413`, `:559-565`; `frontend/src/lib/relayClient.ts:204-211`
**Issue:** The delta from `offset` assumes the restored terminal is in the exact state the old xterm was in at `offset`. `SerializeAddon.serialize()` (0.14) emits buffer content plus `_serializeModes`, and that covers only `?1 ?66 ?2004 4 ?6 ?45 ?1004 ?7l` and the mouse *tracking* mode. It omits:
- SGR mouse encoding `?1006`;
- cursor visibility `?25`;
- the scroll region (DECSTBM);
- the G0 charset;
- the current SGR pen;
- the parser's in-progress sequence.

The snapshot offset is a PTY read boundary. During streaming output (Claude Code generating, a build log), a read routinely ends inside a CSI or a UTF-8 character. Failure scenarios:
1. A peer tab shows vim, htop or Claude Code with SGR mouse on. After switching away and back, xterm is in `?1000h` without `?1006h`, so clicks and wheel events go to the remote app as legacy `ESC [ M b x y` bytes. vim in normal mode reads them as keystrokes. Remote write access makes this worse than a display bug.
2. The last frame before the switch ended in `\x1b[3`, and the delta starts with `2mtext`. The restored terminal prints a literal `2mtext`. The same happens to a split `─` (`e2 94 | 80`), which becomes a replacement character. I verified the equivalent in vt; xterm.js keeps the same cross-write parser state.
3. A TUI with a DECSTBM region (vim splits, a progress bar reserving the bottom line) scrolls the whole restored screen, including its status line, until a full redraw.

The SHAR-07 reconnect path never had these problems because it reuses the same xterm instance.
**Fix:** Pick one of these (cheapest first):
- (a) Don't snapshot when the state can't be serialized faithfully. In the cleanup, skip `putPeerSnapshot` if `term.buffer.active.type === 'alternate' || term.modes.mouseTrackingMode !== 'none'`, so TUIs rejoin with the hub's full replay, which does restore `?1006`, `?25` and the other DEC modes from `h.modes`. Also keep the trailing incomplete bytes of the last output frame (a small ESC/CSI/OSC/DCS/UTF-8 tail scanner in `onOutput`) and append them to `snap.data`. Incomplete sequences have no buffer effect, so replaying them reproduces the parser state.
- (b) Server side, append the hub's DEC mode restore (the `h.modes` loop from `renderScreenLocked`) after the chunks of a `ReplayDelta`. It is idempotent for a live reconnect. Track DECSTBM with a `RegisterCsiHandler('r', ...)` that returns false, and include it.

### WR-02: WARNING: the alt-exit rebuild loses emulator state the rendered seed doesn't carry, so later rendered replays and history diverge from the host

**File:** `internal/relay/hub.go:517-523`, `:536-544`
**Issue:** `rebuildAfterAltExitLocked` re-creates the emulator from `renderScreenLocked()` (text, CUP, DEC modes). These probes against the pinned vt compare a reference emulator with a seeded one after `ESC[?1049h ALT ESC[?1049l`:

| Case | Reference | Rebuilt |
|---|---|---|
| Read ends `$ \x1b[3`, next read `2mgreen` | `$ green` | `$ 2mgreen` |
| Read ends mid-UTF-8 `\xe2\x94`, next `\x80ok` | `$ ─ok` | `$ ok` |
| Pen left red by the prompt, then `after` | red `after` | default `after` |
| Shell-set `ESC[1;4r`, then 6 lines | region scroll | full-screen scroll, different lines into history |
| `ESC(0` then `qqq` | `───` | `qqq` |
| DECSC before the app, DECRC after | cursor at saved row | cursor at (0,0) |
| Custom tab stop at column 20 | `T` at col 20 | `T` at col 9 |

Failure scenario: a script exits `less` and keeps printing in the same read, and the 32 KiB read ends mid-sequence. Every later viewer whose replay takes the rendered path (ring wrapped, or after a genuine resize) gets the corrupted line, and the corruption is baked into `h.history` for up to 10,000 lines. The same goes for tools that keep a DECSTBM region on the main screen (progress bars): the emulator scrolls different rows into history than the host shows.
**Fix:** Defer the rebuild to a clean boundary. Keep `h.altExited` set until a frame ends in ground state, using a cheap tail check on `raw` (no unterminated ESC/CSI/OSC/DCS, no incomplete UTF-8 lead), and only then rebuild. Track the state vt hides with handlers that return false, re-emitting it in the seed: DECSTBM via CSI `r`, and G0 via ESC `(`. Document the remaining losses (pen, DECSC, tab stops) beside the existing `shortcut:` note.

### WR-03: WARNING: a resize while an app is in the alternate screen bypasses HARD-01, losing the main screen and pushing the TUI's lines into history

**File:** `internal/relay/hub.go:654-679` (interacts with `:517-523`)
**Issue:** `resizeLiveEmulator` pushes rows to history only `if !old.IsAltScreen()`, then builds a fresh emulator that starts on the **main** screen. While vim or less is up:
- the main screen behind the alt screen (the pre-app shell rows) is dropped from both the screen and history;
- the SIGWINCH redraw lands on the new emulator's main screen, so lines the TUI scrolls drain into `h.history`;
- the app's later `ESC[?1049l` is a no-op on vt, because `setAltScreenMode` returns early when already on the main screen. So `AltScreen(false)` never fires, no rebuild runs, and the emulator's "shell screen" is the TUI's last frame.

Failure scenario: the host resizes the window while `less` is open, then quits less. A viewer joining afterwards (rendered path, since a genuine resize sets `geomEnd`) sees less's last page as the shell screen, history contains less's scrolled lines, and the pre-less shell lines are gone. HARD-01 promises "main-screen history is not lost when the emulator is rebuilt". This code path predates Phase 7, but the phase is where that promise was made.
**Fix:** In the alt branch, take the old emulator back to the main screen before discarding it, keep its rows, and put the new emulator into the alt screen:
```go
if old := h.liveEmu; old.IsAltScreen() {
	old.Write([]byte("\x1b[?1049l")) //nolint:errcheck
	h.altExited = false // this exit is ours, not the app's
	h.drainScrollbackLocked()
	// push old's main rows down to the cursor row, as the main-screen branch does
	h.liveEmu = h.newLiveEmulator(cols, rows)
	h.liveEmu.Write([]byte(altScreenEnterSeq)) //nolint:errcheck
	return
}
```

### WR-04: WARNING: the HARD-01 "fixed bound" applies per session and scales with width, reaching about 270 MiB per session at the 220-column fallback

**File:** `internal/relay/hub.go:561-565` (fallback width at `:978-985`)
**Issue:** While any app sits in the alternate screen, vt's alt scrollback grows to `DefaultScrollbackSize` (10,000 lines) × cols × about 112-120 B. H-05 measured 128 MiB at 100 columns and exceeded the plan's own ceiling. The disposition recorded about 270 MiB at 220 columns, which is `Cols()`'s fallback until a local viewer sizes the PTY. The cost is per session, so it multiplies: three sessions running `less +F` on logs overnight cost about 0.4-0.8 GiB of daemon heap on an 8 GB MacBook Air. The OS only gets the memory back after the exit-rebuild, and RSS stayed at 277 MB after the run. That cuts against the "rock-solid on a Mac nobody can reach" priority, and nothing alerts on it.
**Fix:** Bound it independently of vt. At alt entry, split the write at the `?1049h`/`?1047h`/`?47h` boundary and keep the main screen's `renderScreenLocked()` output. While still in the alt screen, rebuild every N fed lines or bytes, seeding the new emulator with the saved main screen, then `altScreenEnterSeq`, then the current alt render. Alternatively, upstream or vendor a vt change that applies `SetScrollbackSize` to `scrs[1]` (or 0 for the alt screen). At minimum, state the per-session figure in HARD-01's verification and log a diag line when a session has been in the alt screen for more than X lines.

## Info

### IN-01: Phantom requirement (unproven): launch respawn grace guards a blip that was never reproduced, and judges time from before `Health()`

**File:** `app.go:310-349`
**Category:** necessity
**Issue:** The 07-03 summary says the root cause was "not found". H-04 recorded 0 `not answering right after launch` lines across three relaunches, so the gate has never fired outside unit tests. The bug was seen once, so whether the requirement has lapsed rests on judgment. Separately, `now` is captured by the caller before `a.client.Health()`. If Health blocks for its full client timeout inside the 10 s window, the grace is still judged against the pre-call time, which is harmless but not what the comment says.
**Fix:** Keep it if desired, but pass `time.Now()` after `Health()` returns (or recompute it there). Add a TODO tied to the logged health error, so the gate can be removed once the cause is known.

### IN-02: `peerSnapshots` caps entries, not bytes, and keeps closed tabs' content for the app's lifetime

**File:** `frontend/src/lib/peerSnapshots.ts:18-36`; `frontend/src/components/TerminalPanel.tsx:562`
**Issue:** Each entry is a full `serialize()` of up to 10,000 scrollback lines, stored as UTF-16, often several MB and tens of MB for colour-heavy output. Eight entries can hold hundreds of MB in the webview. A closed peer tab's snapshot, which may hold secrets printed on the remote host, stays until evicted.
**Fix:** Pass `{ scrollback: N }` to `serialize`, cap the total bytes (evict oldest until under budget), and delete the entry when a peer tab is closed (not just hidden).

### IN-03: Deferred `putPeerSnapshot` races a quick remount

**File:** `frontend/src/components/TerminalPanel.tsx:559-565`, `:410`
**Issue:** The put runs in a `term.write('')` callback, after xterm drains its queue. If the user switches back before that callback (a heavy write backlog), the new mount's `takePeerSnapshot` finds nothing and does a full replay. The late put then leaves a stale entry. That entry is still consistent by the offset argument, but it defeats HARD-05 for that switch and holds memory.
**Fix:** Record a pending entry synchronously in the cleanup (for example a `Promise<PeerSnapshot>` in the map) and have the mount await it briefly, or skip the restore when a pending put exists.

### IN-04: Replay strip still lets some answers through to the shared shell

**File:** `internal/relay/hub.go:146-173`, `:911-913`
**Issue:** Gaps that are not hangs:
- `OSC 10;?;?` (multi-query) is kept.
- A mixed OSC 4 set+query is kept whole (documented).
- `CSI 13/19/21 t` are kept. `21 t` (title report) is the classic echo-back vector if a `portty attach` viewer's outer terminal allows window ops.
- A query split across two ring frames arrives as `ESC [ SOH 6 n`. VT500-style parsers, xterm.js included, execute C0 inside CSI and still dispatch, so the CPR answer is typed into the shell (the absorber only drops OSC 10/11 and DA1 replies).

**Fix:** Drop `0x01` bytes from the raw replay before `StripTerminalQueries` in `Join`, as the engine already does. The client takes its position from the offset meta, never by counting replay bytes. Also add `1[39]|2[01]` to the XTWINOPS alternative and `(?:;\?)+` to the OSC 10/11/12 alternative.

### IN-05: Splitting a paste removes its atomicity on a socket drop

**File:** `frontend/src/lib/relayClient.ts:528-533`
**Issue:** A single frame used to arrive whole or not at all. Now a 1 MB paste is 31 frames buffered by the browser, and a drop mid-transfer delivers a prefix with no indication. Without bracketed paste on the remote side, the complete newline-terminated lines of that prefix execute.
**Fix:** Document it, or when `bufferedAmount` is non-zero at the next `onclose`, show a "paste may be incomplete" notice in the reconnect banner.

### IN-06: The alt-exit seed writes DEC modes after the text, so width modes apply too late and "nothing scrolls" isn't guaranteed

**File:** `internal/relay/hub.go:538-543`, `:820-837`
**Issue:** `?2027` (grapheme width) is restored after the rows are written. A row with emoji or ZWJ clusters can lay out wider under the default WcWidth and wrap, scrolling the seed by a line: the top row goes to history and the screen shifts. The comment at `:543` asserts this can't happen.
**Fix:** Emit width-affecting modes (`?2027`) before the text in the seed. Origin and cursor modes stay after the CUP. Or soften the comment.

### IN-07: `testmain_test.go` ignores errors and skips cleanup on panic

**File:** `internal/daemon/testmain_test.go:18-28`
**Issue:** `os.Setenv` errors are discarded. A test panic exits inside `m.Run`, so `os.RemoveAll(home)` never runs and `/tmp/pdh-*` dirs (with sockets) accumulate. On darwin, `os.UserConfigDir` ignores `XDG_CONFIG_HOME`, so that Setenv does nothing (harmless).
**Fix:** `if err := os.Setenv(...); err != nil { panic(err) }`. Use `defer os.RemoveAll(home)` inside a helper that returns the code before `os.Exit`.

---

_Reviewed: 2026-11-19T00:06:05Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
