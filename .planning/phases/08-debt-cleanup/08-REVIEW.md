---
phase: 08-debt-cleanup
reviewed: 2026-11-19T18:45:41Z
depth: standard
files_reviewed: 31
files_reviewed_list:
  - TESTING.md
  - frontend/src/App.tsx
  - frontend/src/components/SessionEndedBanner.tsx
  - frontend/src/components/TerminalPanel.tsx
  - frontend/src/components/WebShareSessionView.tsx
  - frontend/src/components/__tests__/App.open-remote.test.tsx
  - frontend/src/components/__tests__/TerminalPanel.reconnect.test.tsx
  - frontend/src/components/__tests__/TerminalPanel.scale.test.tsx
  - frontend/src/components/__tests__/TerminalPanel.tabSwitch.test.tsx
  - frontend/src/lib/__tests__/fixtures/charset-link-replay.json
  - frontend/src/lib/__tests__/fixtures/ris-replay.json
  - frontend/src/lib/__tests__/fixtures/softwrap-replay.json
  - frontend/src/lib/__tests__/fixtures/wrapped-replay.json
  - frontend/src/lib/__tests__/peerSnapshots.test.ts
  - frontend/src/lib/__tests__/replayFidelity.test.ts
  - frontend/src/lib/peerSnapshots.ts
  - frontend/src/lib/relayClient.test.ts
  - frontend/src/lib/relayClient.ts
  - internal/daemon/process.go
  - internal/daemon/process_test.go
  - internal/relay/altscreen_heap_test.go
  - internal/relay/charstate.go
  - internal/relay/emu_nohang_test.go
  - internal/relay/history.go
  - internal/relay/history_test.go
  - internal/relay/hub.go
  - internal/relay/replay_fidelity_test.go
  - internal/relay/softwrap_test.go
  - internal/relay/vtalt.go
  - internal/relay/vtalt_test.go
  - internal/relay/wraps.go
findings:
  critical: 0
  warning: 5
  info: 8
  total: 13
  necessity: 0
status: issues_found
---

# Phase 8: Code Review Report

**Reviewed:** 2026-11-19T18:45:41Z
**Depth:** standard (plus cross-file reads of the pinned vt, ultraviolet, x/ansi and xterm 6.0.0 sources)
**Files Reviewed:** 31
**Status:** issues_found

## Summary

Scope: `git diff 292daeea..HEAD -- . ':!.planning'` (HEAD d7012004). I read every production file in full. I spot-checked the test files and fixtures and did not audit them line by line. Connected code I read: vt `b57e5e6d29bb` (emulator.go, screen.go, handlers.go, cc.go, esc.go, osc.go, utf8.go, scrollback.go), ultraviolet `renderLine`, the x/ansi v0.11.7 parser transition table and `SetHyperlink`, xterm.js 6.0.0 (`Clipboard.ts`, `CoreBrowserTerminal.ts`, `EscapeSequenceParser.ts`), `internal/webserver/server.go` and `internal/relay/server.go` (ping handling), `internal/daemon/remote_ws_proxy.go`, `app.go` (startup, respawn, Retry, QuitAll) and `process_unix.go`.

Probes ran as one overlay test file (`-overlay`, scratchpad `zz_probe_test.go`, `-run TestProbe_`, one test binary). Results are cited in the findings as P1..P7.

Overall: the hard parts hold up. DEBT-01's reflect/unsafe reach is memory-safe. vt nil-checks every alt-scrollback push (ClearWithScrollback, DeleteLine, ED 3), and Reset and Resize leave the scrollback alone. A 3,000-stream random alt-screen fuzz with the scrollback disabled produced 0 panics (P5). The generation guard, the byte budget, the 1 s flush timeout and the 250 ms save wait in peerSnapshots/TerminalPanel have no race that loses or corrupts a snapshot; a stale entry stays self-consistent. Pong counting in relayClient cannot acknowledge a paste early. Pongs are only ever dropped (non-blocking send on a full queue), never duplicated, and the daemon proxy forwards them end to end. So a pong count of at least `ackPing` always means a pong to a ping sent after the paste.

The defects are in edge semantics:
- The OSC 8 tracker re-emits an unvalidated, attacker-shaped URI at the end of every replay.
- The soft-wrap detector records some non-wraps, which makes false joins that show up as merged lines after an xterm.js reflow.
- DEBT-06's 15 s wait has no early exit for a daemon that died at spawn.
- The seeded textarea/caret CPU burn (WR-01) predates Phase 8.

**Unchanged surfaces confirmed:** `git diff --stat 292daeea..HEAD` is empty for `internal/webserver/` (CSP middleware `csp_mw.go`, share-token/cap verification, the WR-07 tailnet guard in `whois.go`/`tailscale.go`), `internal/relay/protocol.go` (MaxInputPayload 32767), `internal/relay/scrollback.go` (ReplayChunk, 32 KiB read-limit notes), `internal/relay/server.go`, `internal/daemon/remote_ws_proxy.go` (32768-byte client read limit, `ViewerReadLimit` upstream), `internal/attach/`, `main.go`, `app.go`, `go.mod`/`go.sum` and `frontend/package.json`. `encodeInputFrames` (32767-byte split) is untouched. The 32 KiB limits, share tokens, CSP and the WR-07 guard are unchanged.

## Warnings

### WR-01: A large paste stays in xterm's helper textarea; WebKit re-shapes it on every caret blink (seeded from V-05; predates Phase 8)

**File:** `frontend/src/components/TerminalPanel.tsx:391` (no paste handling after `term.open`); root cause `frontend/node_modules/@xterm/xterm/src/browser/Clipboard.ts:43-56`, compiled identically in `lib/xterm.mjs` (`function qs` / `function Cn`)

**Issue:** xterm's `handlePasteEvent` calls `ev.stopPropagation()`, reads `clipboardData`, hands the text to `triggerDataEvent` and sets `textarea.value = ''`. It never calls `ev.preventDefault()`. The paste event fires before the browser's default action, so WebKit inserts the whole clipboard text into the focused `xterm-helper-textarea` after xterm has cleared it. xterm's `_inputEvent` (`CoreBrowserTerminal.ts:1192`) only handles `inputType === 'insertText'`, so the `insertFromPaste` insertion stays. The textarea is then cleared only on blur (`:292`) or on an Enter / Ctrl+C keydown (`:1087`). The paste's own CRs are not keydowns, so they do not clear it.

Failure scenario (measured in V-05):
1. Cmd+V of 20 MB (native Edit-menu paste → WKWebView paste command → DOM `paste` on the focused textarea).
2. The textarea now holds 20 MB.
3. `OpacityCaretAnimator::updateAnimationProperties` → `FrameSelection::recomputeCaretRect` → `CTFontShapeGlyphs` re-shapes it on every caret blink frame.
4. WebContent sits at about 100% CPU with 1.8 GB RSS for 15+ minutes. The relay watchdog, banners and every UI update stall.

5 MB cost about 19 s. Any paste in a tab that has not seen Enter since keeps its text resident.

**Predates Phase 8:** yes. Phase 8 changed neither `frontend/package.json` (xterm 6.0.0) nor the paste input path. `term.onData → client.sendInput` is unchanged; 08-03 added only accounting inside `sendInput`. Every Portty build that uses xterm's default paste path has this.

**Fix (smallest safe):** cancel the browser's default insertion and leave xterm's own handler alone. xterm reads `clipboardData` synchronously and only calls `stopPropagation`, not `stopImmediatePropagation`, so a second listener on the same textarea still runs.

```ts
// TerminalPanel.tsx, right after term.open(containerRef.current)
const helper = term.textarea
const stopPasteInsert = (e: ClipboardEvent) => e.preventDefault() // xterm already read clipboardData
helper?.addEventListener('paste', stopPasteInsert)
// in the mount effect's cleanup, before term.dispose():
helper?.removeEventListener('paste', stopPasteInsert)
```

The same fix covers context-menu paste. Pin it with a vitest that dispatches a `ClipboardEvent('paste')` with `clipboardData` on `term.textarea` and asserts `defaultPrevented`, `onData` fired once and `textarea.value === ''`. Then run a VM check: a 20 MB Cmd+V, WebContent CPU idle within a few seconds, and an AX read of the textarea value empty. Belt and braces, if WebKit ever ignores `preventDefault` here: also clear `term.textarea.value` on an `input` event whose `inputType === 'insertFromPaste'`.

### WR-02: The OSC 8 tracker stores and re-emits an unvalidated URI (C1 controls that xterm.js executes, up to vt's 4 MB OSC buffer) at the end of every replay

**File:** `internal/relay/charstate.go:50-61` (`osc8`), `:95-97` (`suffix`); emitted at `internal/relay/hub.go:1177`, `:1438` (and fed back into vt at `:814`, `:1061`)

**Issue:** The vt and xterm.js parsers disagree on an OSC payload:
- The x/ansi parser (`parser/transition_table.go:264`) puts every byte 0x20-0xFF except 0x9C into OSC data.
- xterm.js decodes UTF-8 first, and its "anywhere" C1 transitions (`EscapeSequenceParser.ts:102-108`) apply inside `OSC_STRING`. A U+009B (bytes C2 9B) aborts the OSC and starts a CSI; U+0090 starts a DCS.

`charState.osc8` copies `parts[2]` verbatim, and `suffix()` appends it to every rendered replay, after the CUP. The only shape check is "exactly three `;`-separated parts", which blocks OSC payloads with `;` but no `;`-free CSI or DCS.

- **P1:** feeding `ESC]8;;http://x/\u009b2J BEL` leaves `cs.link = ["", "http://x/\u009b2J"]`. `StripTerminalQueries` keeps it, and the replay ends with `ESC[1;3H ESC]8;;http://x/\u009b2J BEL`. A joining xterm.js viewer then runs CSI 2J right after painting the replay. `\u009b3J` would wipe the history it was just sent; `\u009b?1049h` would strand it in the alt screen; `\u009b6n` / `\u009bc` make the viewer type a CPR/DA reply into the host shell.
- **DCS variant:** `ESC]8;;http://x/\u0090$qm\u009c` is stored as `http://x/\u0090$qm\xc2`. Replayed, the viewer opens a DCS (DECRQSS) that swallows live output until the next ESC, then types the reply into the PTY. That ESC can come long after the server's join-time input absorber stops looking.
- **P2:** a 1 MiB URI opened and never closed (then `clear`) adds about 1 MiB to every Join replay. vt's parser data size is 4 MB (`emulator.go:90`), so up to 4 MB per join persists until RIS or a close.

The attacker only needs to get bytes printed in the host terminal (`cat`, `curl`, `git log`, an agent's tool output). Anyone who can print can already send these bytes live. The tracker turns that into a sequence that re-executes on every join and survives `clear`. Rendered rows already carried the URI before Phase 8 (`renderLine`), but `fixRenderedLinks` passes it through unchanged.

**Fix:** validate in one place and use it on both paths:

```go
const maxLinkField = 2048 // xterm.js links beyond this are useless anyway
func safeLinkField(s string) bool {
	if len(s) > maxLinkField || !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if r < 0x20 || (r >= 0x7f && r <= 0x9f) {
			return false
		}
	}
	return true
}
// charState.osc8: if !safeLinkField(parts[1]) || !safeLinkField(parts[2]) { c.link = [2]string{}; return }
// fixRenderedLinks: in the ReplaceAllStringFunc callback, return "" (drop the link) when either field fails.
```

Add a test: the P1, DCS and 1 MiB cases leave `cs.link` empty, and a rendered row with such a link renders without an OSC 8.

### WR-03: The soft-wrap detector records LF/IND/CUD + CHA/CUP-to-column-1/2 from a full row as a wrap ("harmless" in the comment is wrong)

**File:** `internal/relay/hub.go:878-895` (CursorPosition callback; claim at `:882-883`)

**Issue:** The callback accepts any move pair (W-1,y)→(W-1,y+1) followed by (W-1,y+1)→(1|2,y+1). LF, IND, CUD and VPA all keep X at W-1, and CHA/CUP/HPA then land on column 1 or 2. The "full row and key" checks do not help, because the recorded key is that same full row.

- **P3a:** `20×"-" LF ESC[2G x` at 20 columns gives a history row `--------------------` with `wrapped=true`, and `History.Render` emits `-------------------- x` with no CRLF.
- **CUD variant:** `20×"=" ESC[B ESC[2G |` gives `wrapped=true` too.

Effect: the viewer's xterm.js marks the pair as one logical line. At the host grid it looks identical. When the host resizes, GUI viewers follow the 0x02 grid and xterm.js reflows its buffer, so the two lines merge (`-------------------- x` on one row). Copy and selection also lose the newline. Bare LF reaches the PTY output whenever ONLCR is off, or from IND, CUD or VPA in cursor-addressed main-screen UIs (ncurses `-X`-style programs, box drawing).

**Fix:** only count moves that vt's print path makes. vt's own cursor-motion CSIs can be marked, because registered handlers run before vt's:

```go
nonPrint := func(ansi.Params) bool { h.nonPrintMove = true; return false }
for _, f := range []byte{'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'f', 'd', 'e', '`', 'a'} {
	emu.RegisterCsiHandler(int(f), nonPrint)
}
// ESC D/E/M and ESC 8 likewise via RegisterEscHandler.
// In CursorPosition: if h.nonPrintMove { h.nonPrintMove = false; h.lastMove = [2]uv.Position{{X: -1}, {X: -1}}; return }
```

A cheaper partial fix also handles P3a: require `emu.CellAt(new.X-1, new.Y)` to be the cell just printed (non-empty), and pin it with the P3a and CUD cases in softwrap_test.go. Correct the comment either way.

### WR-04: DEBT-06's 15 s wait has no early exit when the spawned daemon dies, and it now gates Quit All, Retry and every CLI command

**File:** `internal/daemon/process.go:162-184`; `internal/daemon/process_unix.go:19-34`; holders of `daemonMu` at `app.go:322`, `:400`, `:477`

**Issue:** `startDetachedDaemon` calls `cmd.Process.Release()`, so `waitForDaemon` cannot tell "slow to bind" from "already dead". These daemons now cost the full 15 s instead of 3 s:
- one that exits at once: crash at init, `lockDaemon` failing after its 2 s when a version-mismatched old daemon is slow to stop, a bad config;
- one whose socket file is stale (refused).

Consequences:
- The startup error UI appears about 15 s after launch. `domReady` shows the window at 5 s, then `GetDaemonError` blocks on `startupDone`.
- Each Retry click blocks 15 s, or up to 30 s when a poller respawn holds `daemonMu`.
- `QuitAll` takes `daemonMu` "to wait out a respawn" and now hangs up to 15 s on a crash-looping daemon.
- `portty list` and friends hang 15 s before printing the error.
- `Release()` also leaves each dead daemon a zombie of the GUI, which compounds over respawn streaks (pre-existing).

A slow but live daemon still benefits from 15 s (B-07: 10 s at login), so keep the bound and add the early exit.

**Fix:**

```go
// process_unix.go
func startDetachedDaemon(exe string) (<-chan struct{}, error) {
	// ... as now up to cmd.Start()
	exited := make(chan struct{})
	go func() { _ = cmd.Wait(); close(exited) }() // reaps; no zombie
	return exited, nil
}
// waitForDaemon: in the poll loop
select {
case <-exited:
	if client.Health() == nil { // another EnsureDaemon's daemon won the lock
		return nil
	}
	return fmt.Errorf("EnsureDaemon: daemon exited during startup (see daemon.log)")
default:
}
```

Test it with a seam that returns a closed `exited` and a refusing socket: the error must come in well under 1 s.

### WR-05: TESTING.md M-101 still requires the "N of M KB sent" wording that the V-05 disposition calls unreachable on that path

**File:** `TESTING.md:1244`

**Issue:** M-101 step 1 runs the GUI peer path, which goes through the loopback daemon proxy, and requires `Paste interrupted: N of M KB sent` with N < M and S < (N+1) × 1024. The orchestrator's V-05 disposition (08-VM-CHECKS.md, "Orchestrator disposition") explains why the browser's `bufferedAmount` is 0 when the tailnet side dies on that path, so only "Paste may have been cut short (M KB)" can appear there. Every future regression run of M-101 will record a FAIL that is not a regression. That trains verifiers to ignore the item, which is the standing regression suite's only DEBT-05 end-to-end check.

**Fix:** rewrite step 1's pass rule to match the requirement and the disposition. The notice appears (either wording), M = ceil(P/1024), the office file is an exact prefix that never grows, nothing is resent, and typing works. Move the N-of-M rule to a browser web-share variant, where the socket itself can back up, or drop it. Update "_Last run_" to note that the FAIL was dispositioned.

## Info

### IN-01: The hash multiset gives false joins for an identical hard row above a wrapped one, and line-editor redraws inflate counts

**File:** `internal/relay/wraps.go:34`, `:27-40`; `internal/relay/hub.go:971`

**Issue:** `take` consumes keys in drain order, not row identity.
- **P3b:** `20×"-" CRLF 40×"-" CRLF` at 20 columns gives history `wrapped=[true false false …]`. The hard first row takes the key the second row's wrap added (false join), and the real wrap is then missed.
- **P6:** a zle-style redraw (`CUU CR` + reprint) of one wrapped prompt line four times leaves count 5 for one physical row. The extras linger until 4,096 later adds evict them, which widens the window for identical-row false joins.

The comment "a missed join, never a false one" is true only for eviction. joinLocked's shortcut note covers the identical-row case, but not this ordering form. Same impact as WR-03 (reflow/copy), rarer trigger.

**Fix:** correct the comment. If it ever matters, key wraps by absolute line number (lines drained so far + row) instead of by content hash.

### IN-02: `History.EndsWrapped` is not invalidated when screen row 0 is erased without scrolling

**File:** `internal/relay/history.go:69`; used at `internal/relay/hub.go:1431-1436`

**Issue:** **P7:** a 30-character line wraps across the history/screen boundary, then the screen is erased with `ESC[H ESC[J` (ED 0, which pushes nothing, unlike ED 2) and `$ prompt` is printed. History still ends `wrapped=true`, row 0 is `$ prompt`, and the Join replay joins the old line to the new prompt with no CRLF. Impact as WR-03.

**Fix:** when joining history to row 0, also require that the key recorded for the history line's continuation matches. A simpler option clears the flag: have the `J` handler (n = 0 at home, 1, 2) and EL on row 0 call `h.history.ClearEndWrap()` (set the last `wrapped` to false).

### IN-03: The GL scan misses a RIS that follows an OSC terminated by a bare ESC, and an SO inside a malformed UTF-8 tail

**File:** `internal/relay/charstate.go:70-81`

**Issue:** **P4:** `SO ESC]0;title ESC c` leaves `gl1=true`, so `suffix()` emits `\x0e`. vt and xterm.js both end the OSC at the ESC and run RIS. The check `c.vt.state == scanEsc` reads the pre-step state `scanStrEsc`. The RIS handler resets G0/G1, so the stale SO prints ASCII until a later `ESC ) 0`. From then on every replay puts the viewer in line drawing until the host sends SI. Separately, an SO arriving while `need > 0` (a truncated UTF-8 lead byte) is dropped by the scan but executed by xterm.js. Both are rare.

**Fix:** check `x == 'c' && c.esc && (c.vt.state == scanEsc || c.vt.state == scanStrEsc)`. For the UTF-8 case, mirror xterm.js and treat a non-continuation byte as ending the character before testing for SO.

### IN-04: `altScreenOf` checks the field type but not which element is the alt screen

**File:** `internal/relay/vtalt.go:26-32`

**Issue:** If a future vt kept `scrs [2]Screen` but swapped roles, the type check would pass and `SetScrollback(nil)` would disable the main screen's scrollback, so all history would be lost silently. The version pin catches this only when someone runs the test after a bump. The runtime has no semantic check, and it is the only guard in a dependency-bump PR built without the tests.

**Fix:** in `altScreenOf`, also require `(*xvt.Screen)(unsafe.Pointer(f.Index(0).UnsafeAddr())).Scrollback() == emu.Scrollback()`, and return nil (fallback plus log) otherwise.

### IN-05: (pre-existing, interacts with DEBT-05) A paste into a slow PTY reader blocks the server's read loop, so its WebSocket heartbeat drops the viewer

**File:** `internal/webserver/server.go:896-904` (synchronous `hub.WriteInput` in the read loop); `internal/relay/heartbeat.go:13-16`

**Issue:** coder/websocket handles the pong to the server's `Ping` only inside `Read`. While `WriteInput` blocks on a full PTY input queue (an app that consumes a multi-MB paste slowly), the read loop does not read, `Ping` times out after 15 s, and `CloseNow` drops a healthy viewer mid-paste. DEBT-05 then shows "may have been cut short". That wording is honest, but the drop is self-inflicted. Not Phase 8; not verified on the VMs.

**Fix:** move the PTY writes off the read goroutine (a bounded channel drained by a writer goroutine, or `Write` with a deadline), so the read loop keeps servicing control frames. Track as a separate item.

### IN-06: The paste notice can fire for a paste that fully arrived (dropped pong or a slow-consumer kick)

**File:** `frontend/src/lib/relayClient.ts:369-375`, `:594-614`; server `internal/webserver/server.go:926-931`

**Issue:** The server answers a ping with a non-blocking send and drops the pong when `sub.Msgs` is full. A large paste into a shell that echoes fills that queue, so the ack waits for the next 15 s heartbeat pong. A socket loss in that window, or the hub's "too slow" kick triggered by the paste's own echo, shows "Paste may have been cut short" for a complete paste. This is conservative (never a false "complete"), and the copy says "may".

**Fix:** none required. Optionally send the pong blocking with a short timeout, or count the pong as owed and resend it on the next drain.

### IN-07: Snapshot host keying (W1) does not cover the paths that replace tabs

**File:** `frontend/src/App.tsx:535-537`, `:944` (`retryInit`'s `setTabs(restoredTabs)`); `webSessionTabId(sessionId)` at `:767`

**Issue:** `retryInit` replaces every tab and drops peer tabs without `dropPeerSnapshot`. Peer tab ids are keyed by session id alone, so two Macs that hand out equal ids still share one tab (`openWebSessionTab` replaces its peer fields). The old host's snapshot is then orphaned under its old key. Both cases are bounded by `PEER_SNAPSHOT_MAX` and `PEER_SNAPSHOT_MAX_BYTES`, and a stale entry stays self-consistent, so this costs memory and a delta, not correctness.

**Fix:** drop snapshots for removed peer tabs in `retryInit` too. If equal ids across hosts are real, key `webSessionTabId` by host as well.

### IN-08: No fuzz target for the new byte-level parsers

**File:** `internal/relay/charstate.go`, `internal/relay/hub.go:621-682` (`vtScan`)

**Issue:** `charState.scan`, `osc8` and `fixRenderedLinks` are covered by table tests and the randomized `rebuildFuzzStream`. That stream checks the rebuild, not GL tracking against a reference. Nothing checks the tracker against xterm.js on adversarial input, which would have caught IN-03 and WR-02.

**Fix:** add `FuzzCharState`: feed arbitrary bytes to a hub and to a reference that tracks GL the xterm.js way (C0 executed in ground/escape/CSI states, strings ended by ESC/BEL/ST/CAN/SUB, UTF-8 decoded first), and assert equal `gl1` and that the replay suffix contains no byte < 0x20 other than ESC, BEL and SO/SI, and no C1 code point.

---

_Reviewed: 2026-11-19T18:45:41Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
