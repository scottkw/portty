---
phase: 10-input-path-reliability
reviewed: 2026-11-21T06:25:02Z
depth: standard
files_reviewed: 55
files_reviewed_list:
  - frontend/src/App.tsx
  - frontend/src/components/Sidebar.tsx
  - frontend/src/components/TerminalPanel.tsx
  - frontend/src/components/WebShareSessionView.tsx
  - frontend/src/components/__tests__/App.daemon-recovered.test.tsx
  - frontend/src/components/__tests__/App.open-remote.test.tsx
  - frontend/src/components/__tests__/App.test.tsx
  - frontend/src/components/__tests__/Sidebar.test.tsx
  - frontend/src/components/__tests__/TerminalPanel.contextLoss.test.tsx
  - frontend/src/components/__tests__/TerminalPanel.guestClose.test.tsx
  - frontend/src/components/__tests__/TerminalPanel.reconnect.test.tsx
  - frontend/src/components/__tests__/TerminalPanel.scale.test.tsx
  - frontend/src/components/__tests__/TerminalPanel.tabSwitch.test.tsx
  - frontend/src/components/__tests__/TerminalPanel.test.tsx
  - frontend/src/components/__tests__/WebShareSessionView.test.tsx
  - frontend/src/lib/__tests__/tabs.test.ts
  - frontend/src/lib/relayClient.test.ts
  - frontend/src/lib/relayClient.ts
  - frontend/src/lib/tabs.ts
  - internal/attach/attach.go
  - internal/attach/attach_test.go
  - internal/attach/attach_unix.go
  - internal/attach/loop_test.go
  - internal/daemon/api_test.go
  - internal/daemon/attach_viewers_test.go
  - internal/daemon/engine_exit_test.go
  - internal/daemon/engine_fdleak_test.go
  - internal/daemon/handoff.go
  - internal/daemon/handoff_fault_test.go
  - internal/daemon/handoff_test.go
  - internal/daemon/process_test.go
  - internal/daemon/remote_ws_proxy_test.go
  - internal/relay/handoff_state.go
  - internal/relay/handoff_state_test.go
  - internal/relay/history_test.go
  - internal/relay/hub.go
  - internal/relay/hub_grid_owner_test.go
  - internal/relay/hub_test.go
  - internal/relay/input.go
  - internal/relay/input_test.go
  - internal/relay/oscabsorb.go
  - internal/relay/oscabsorb_test.go
  - internal/relay/pong.go
  - internal/relay/pong_test.go
  - internal/relay/protocol.go
  - internal/relay/protocol_test.go
  - internal/relay/replay_fidelity_test.go
  - internal/relay/server.go
  - internal/relay/server_input_queue_test.go
  - internal/relay/server_test.go
  - internal/webserver/claim_test.go
  - internal/webserver/input_queue_test.go
  - internal/webserver/revoke_test.go
  - internal/webserver/server.go
  - internal/webserver/server_test.go
findings:
  critical: 1
  warning: 4
  info: 7
  total: 12
  necessity: 0
status: issues_found
---

# Phase 10: Code Review Report

**Reviewed:** 2026-11-21T06:25:02Z
**Depth:** standard (plus targeted cross-file tracing of the lock order, the handoff tail and the revoke path)
**Files Reviewed:** 55
**Status:** issues_found

## Summary

Scope: `git diff d2998ad2..9a18160c` for `*.go`, `*.ts`, `*.tsx` and `*.js`. I read the connected code too: `Hub.RevokeWebViewers`, `Freeze`/`unfreeze`/`Export`, `oldHandoff.finish`, `remote_ws_proxy.go`, `pty.Session.Write`, `daemonLog.Write`, `cmd_attach.go`, the attach loop and the App/TerminalPanel wiring. `GOTOOLCHAIN=go1.26.11 go vet ./...` returned rc 0 (output in `/private/tmp/claude-501/-Users-ken-dev-portty/66f0384b-8902-40e9-9d84-68c47b57f8f8/scratchpad/review-vet.txt`). Per the instructions, no test binaries were run.

What holds up:
- **Lock order.** gridMu → mu → (release) → emuMu holds everywhere. `inMu` is a real leaf; it is only ever held while logging, see IN-02. Nothing that holds emuMu takes gridMu: Run, Join and Export were checked. No deadlock path was found. `WaitInputIdle`'s deferred `t.Stop()` runs after the `inMu` unlock, so a timer that already fired cannot deadlock it.
- **Input writer lifetime.** There is one writer and one done-watcher per hub, both started lazily. `HubManager.Remove`/`Shutdown` close `done`, which kills the queue. A writer blocked in a PTY write is freed when the engine closes the fd. No leak was found.
- **Pong accounting.** Each `MsgPing` is answered by exactly one pong. `Owe` runs after `in.Write` returns, so the pong means "queued". The pong never goes through `sub.Msgs`, so a full queue cannot drop it. `relayClient`'s count-based `ackPing` still matches.
- **Read-only caps.** They are blocked twice: the pumps skip them and so does `sizeClient`/`NoteTyping`. Claims are clamped to 20..500 x 5..200. The CSP, the read limits and the 32 KiB frame guard are unchanged. No new file reads were added.
- **The `>=` geomEnd change (10-07-FIX).** It is sound. Every ring byte at or after `geomEnd` follows the resize, and both pumps send the current grid frame before the delta (the local relay sends it whenever `GridKnown`, which any genuine resize implies).
- **The IsUserInput classifier.** It correctly rejects every xterm.js auto-reply I could list. The absorber holds a lone ESC, so the Escape key still claims on its next byte. X10 DEFAULT-encoded mouse reports go through `onBinary`, which TerminalPanel never sends, so no false positive comes from mouse reports.

Key concerns:
1. **CR-01 (security):** Stop Sharing / revoke no longer stops input already queued from the revoked viewer. That can be up to 1,024 frames / 32 MiB per connection, typed into the host shell after the host revoked access.
2. **WR-01:** Memory has no hub-wide or daemon-wide bound.
3. **WR-02:** The global-FIFO ordering guarantee breaks across an upgrade handoff.
4. **WR-03:** The window-resize claim fires for resizes the OS makes, so the unattended office tab can take the grid away from the home user.
5. **WR-04:** The new `ErrInputClosed` exit races the BUG-02 "session ended" close.

## Narrative Findings (AI reviewer)

## Critical Issues

### CR-01: Revoking a share does not cancel the revoked viewer's queued input

**File:** `internal/relay/input.go:87-100`, `internal/webserver/server.go:868-870` and `935-938`, `internal/relay/hub.go:444-466`
**Issue:** `RevokeWebViewers` (share off, FIX-02 / D-05) calls `sub.Kick`, which only runs `conn.Close`. The read pump then exits and runs `defer in.Close()`. `InputConn.Close` keeps every queued frame "to be delivered" (input.go:87: "Its queued frames are still written"). The writer only drops the queue on a PTY write error or hub shutdown, and share-off does neither.

Failure scenario: a write-cap guest pastes a large blob, or a script of commands, while the foreground program is not reading stdin, for example during a long `make` or `sleep`. The macOS tty input queue fills after about 1 KiB, the PTY write blocks, and the rest of the paste waits in the hub FIFO: up to 1,024 frames / 32 MiB for this connection. The host sees it and clicks Stop Sharing. The guest is disconnected and its cap is cleared, but once the foreground program exits, the shell reads and runs every queued byte.

Before Phase 10 the pump blocked in `WriteInput`, so at most one 32 KiB frame was in flight. Bytes still in TCP buffers died with the socket. The host's revoke was therefore an effective stop. The CONTEXT's "writer outlives the socket" decision covered network drops, not revocation. No test covers revoke with a backlog (`revoke_test.go` only had a log-string change).

**Fix:** Let a revoke discard that connection's queued (not in-flight) frames. Keep backlog delivery for ordinary drops only:
```go
// input.go
// Discard drops c's queued frames (a revoked viewer): only the frame already
// in the PTY write may still land.
func (c *InputConn) Discard() {
	h := c.h
	h.inMu.Lock()
	defer h.inMu.Unlock()
	c.closed = true
	kept := h.inq[:h.inHead]
	for _, r := range h.inq[h.inHead:] {
		if r.c == c {
			c.frames--; c.bytes -= len(r.data); h.inBytes -= len(r.data)
			continue
		}
		kept = append(kept, r)
	}
	h.inq = kept
	h.inCond.Broadcast()
}
```
Have the share handler call `in.Discard()` from `sub.Kick` before `conn.Close`. Alternatively, have `RevokeWebViewers` drop input by `*Subscriber` through a hub method. Add a test: a stuck-PTY hub, a web viewer queues N frames, `RevokeWebViewers`, then assert that at most one frame reaches the PTY after the revoke.

## Warnings

### WR-01: The input queue has no hub-wide or daemon-wide bound

**File:** `internal/relay/input.go:29-32`, `68-85`
**Issue:** The quota is per `InputConn` only (1,024 frames / 32 MiB). Every connection, and every reconnect, gets a fresh quota, while the closed connections' frames stay queued until the PTY drains. With N viewers, or one viewer whose connection flaps during a long paste into a stalled reader, the daemon holds N x 32 MiB on the heap. Before Phase 10 the daemon buffered nothing; TCP windows did the backpressure. The office daemon is the machine the user cannot reach physically ("rock-solid priority"). A jetsam kill there ends every session, which is much worse than a dropped paste.
**Fix:** Add a hub-wide cap next to the per-connection one. Have `Write` wait while `h.inBytes+len(b) > InputHubMaxBytes && c.frames > 0`. 64 MiB would do, and a process-wide atomic counter is optional. Add a test: two connections at quota, a third blocks.

### WR-02: Across a handoff, the old daemon's backlog interleaves with the takeover's input

**File:** `internal/daemon/handoff.go:756-785` (`finish`); the design claim is in `internal/relay/input.go:15-18`
**Issue:** The hub-global FIFO was chosen (10-CONTEXT A5) so that "a dropped viewer's backlog can't interleave with its reconnect's input". After COMMITTED, the takeover is already serving: the test trace point is `new:after-serve-before-committed`. Meanwhile the old daemon sends 1012 to every viewer and then keeps writing its queued input to the same PTY master for up to `restartDrain` (1 s). `relayClient` redials quickly after a 1012. A user who keeps typing, or a second viewer, gets bytes written by the new daemon's writer while the old writer is still mid-backlog. The two processes write the same fd with no ordering between them, so a paste can have keystrokes spliced into it. This happens on any upgrade during a slow paste. The documented accepted limit covers losing bytes, not reordering them.
**Fix:** Either (a) have the takeover hold its imported hubs' input writers until the old daemon's pid exits (the takeover already tracks the peer pid; start writers paused and unpause on exit or on a "drained" frame), or (b) add this to the accepted limits with a `shortcut:` comment next to `waitHubsInputIdle`, so the FIFO comment in input.go stops promising global order.

### WR-03: The window-resize claim fires on resizes the OS makes, so an unattended viewer takes the grid

**File:** `frontend/src/components/TerminalPanel.tsx:997-1005`
**Issue:** Every `window` `resize` on the active panel becomes a claim 150 ms later, whether or not a person caused it. macOS resizes windows by itself on display reconfiguration: an external monitor unplugged or powering down (many DisplayPort monitors disconnect when they sleep), a resolution change, Stage Manager or Mission Control rearranging, or a window restored after wake. In this user's setup the office MacBook Air sits unattended with Portty's local tab active, while the user types from home. One display event at the office makes that tab claim, and the home user's shell reflows to the office window's fit mid-session. Each claim also bumps `geomEnd`, so later reconnects get full replays. This is exactly the "idle viewers fight over the grid" outcome the classifier was built to prevent, but it comes in through the resize path.
**Fix:** Claim only when a person can have caused it. For example, in `onWindowResize` (or in `claimFit`), require `document.visibilityState === 'visible' && document.hasFocus()`. Otherwise fall back to `reportFit()`:
```ts
const onWindowResize = () => {
  clearTimeout(claimTimer)
  claimTimer = setTimeout(() => {
    if (document.visibilityState === 'visible' && document.hasFocus()) claimFit()
    else reportFit()
  }, CLAIM_DEBOUNCE_MS)
}
```
A user drag-resizing or zooming the window always has it key and visible. Add a vitest that blurs the document and asserts no claim is sent.

### WR-04: The `ErrInputClosed` exit races WritePump's "session ended" close

**File:** `internal/relay/pong.go:49-81`, `internal/relay/server.go:186`, `internal/webserver/server.go:969`
**Issue:** When the hub shuts down, the done-watcher kills the input. A pump blocked in `in.Write`, or one that writes after the kill, gets `ErrInputClosed` and returns, which closes `readDone`. In `WritePump`, both `<-hub.Done()` and `<-readDone` are now ready, and Go's `select` picks one at random. About half the time it returns through `readDone`. The deferred `conn.CloseNow()` then sends an abnormal close with no code or reason. That is the BUG-02 (#125) failure the `hub.Done` branch exists to prevent: the viewer shows "reconnecting"/"unreachable" instead of "session ended". It hits exactly the viewers that were typing or pasting when the shell exited.

A second case: if a PTY write fails while Run is still alive, every pump on that hub exits on its next keystroke. The viewer then reconnects, types, and is dropped again, in a loop with no clear end state.
**Fix:** Give the hub end priority when the read pump finishes:
```go
case <-readDone:
	select {
	case <-hub.Done():
		conn.Close(websocket.StatusNormalClosure, CloseReasonSessionEnded)
	default:
	}
	return
```
Also consider closing with a fixed reason (for example `CloseReasonSessionEnded`) when `in.Write` returns `ErrInputClosed`, instead of returning silently.

## Info

### IN-01: A claim made while the hub is frozen is lost on commit

**File:** `internal/relay/hub.go:556-560`, `601-606`; `internal/relay/handoff_state.go:113-170` (Export)
**Issue:** On a frozen hub, `ClaimGrid` and `NoteTyping` record `owner`/`wantCols`/`wantRows` and leave applying them to Thaw. A committed handoff never thaws the old hub, and `HubState` carries neither `want` nor the owner, so the claim disappears. `claimFit` deliberately does not resize xterm ("the claim's 0x02 echo makes the panel follow"), so the zoom or resize the user just made shows only as a scaled view until they claim again. The window is short (at most `maxFrozen`), but the outcome is silent.
**Fix:** Export `WantCols/WantRows` and apply them in the takeover as a claim when they differ from `PtyCols/PtyRows`. Or have TerminalPanel re-send its last claim on reconnect when no 0x02 matching it arrived.

### IN-02: `inMu` is held across `log.Printf`

**File:** `internal/relay/input.go:98`, `151`, `186`
**Issue:** The hub's "unlock before IO" rule is broken under the leaf input lock. `daemonLog.Write` takes its own mutex and can rotate under it (`os.Rename`, `OpenFile`, `Dup2`). Every pump's `Write` and the writer itself stall behind that log I/O.
**Fix:** Format the line under the lock and call `log.Printf` after unlocking, as `kickSlow` does.

### IN-03: `Hub.WriteInput` has no production callers and bypasses the FIFO

**File:** `internal/relay/hub.go:1493-1499`
**Issue:** Only tests call it now. A future caller would write around the FIFO, interleaving with queued input mid-paste, and would skip `NoteTyping`.
**Fix:** Remove it and port the tests to `OpenInput(nil).Write`, or make it a wrapper over the FIFO.

### IN-04: Peer-row matching still keys by `hostname` while tab ids key by `connectHost`

**File:** `frontend/src/App.tsx:941-949`, `frontend/src/components/Sidebar.tsx:80-84`, `223`
**Issue:** INPT-04 keys tabs by `connectHost` (MagicDNS, unique). `dropPeerSession` and the Sidebar's `aria-current` match on `peer.hostname`, and two MacBook Airs can share a hostname (the tailnet disambiguates only the DNS name). If both peers also have a session with the same id, as in INPT-04's premise, "share stopped" marks both tabs and both rows highlight.
**Fix:** Compare `peerSnapshotHost(tab)` with `g.peer.peerDNSName || g.peer.hostname`, and pass the connect host to `dropPeerSession`.

### IN-05: The web-session fallback id now yields `host/id`

**File:** `frontend/src/App.tsx:1177`
**Issue:** If `activeWebTab` is ever missing, `activeId.slice('__websession__'.length)` returns `"<host>/<id>"` for a peer tab, and `WebShareSessionView` would dial that string as a session id. This was unreachable in practice before; the new id shape makes the fallback wrong.
**Fix:** Fall back only for hostless ids (`!rest.includes('/')`), or render nothing when `activeWebTab` is undefined.

### IN-06: `retryInit` drops snapshots of replaced peer tabs but not their font sizes

**File:** `frontend/src/App.tsx:993-998` (compare `removeTabState` at 591)
**Issue:** `fontSizes[tabId]` entries for peer tabs removed by Retry are never deleted. This is a small leak, and a reopened tab with the same id inherits the old zoom.
**Fix:** Delete `fontSizes[t.id]` for each `peerTabsRemoved` entry, or reuse `removeTabState`'s side-effect half.

### IN-07: The INPT-06 frontend invariants are partly pinned by source-text assertions

**File:** `frontend/src/components/__tests__/TerminalPanel.scale.test.tsx:224-271`
**Issue:** Tests like "relayout fits only before any grid is known" slice the component source and `toContain` exact code strings. They break on a harmless refactor, and they pass if the logic is wrong but spelled the same. The behavioral block at lines 609-766 covers most of the same cases, so these are noise rather than coverage.
**Fix:** Drop the string pins that the behavioral tests already cover. Convert the rest (for example, "reports carry the measured fit") into behavior assertions on `sendResize` arguments.

---

_Reviewed: 2026-11-21T06:25:02Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_


## Iteration 2

**Reviewed:** 2026-11-21T06:55:38Z
**Scope:** the review-fix commits only, `git diff 9a18160c..46f49642 -- '*.go' '*.ts' '*.tsx'` (HEAD 46f49642), plus the code they connect to: `RevokeWebViewers` and its callers (`unshareLocked`, `remintSharesLocked`), both read pumps and their defer order, `WritePump`, `pty.WatchExit`, `takeover.commit`/`lost`/`oldGone`, `oldHandoff.finish`, `watchSessionExit`, `relayClient` liveness, and the `peerSnapshotHost`/`connectHost` wiring.
**Depth:** standard, with cross-file tracing of the revoke path, the handoff tail and the lock order.
**Status:** issues_found
**Counts:** critical 1, warning 1, info 2 (total 4, necessity 0). IN-07 and IN-01 (the shortcut comment) are accepted and not raised again.

Checks: `GOTOOLCHAIN=go1.26.11 go vet ./...` returned rc 0 (`/private/tmp/claude-501/-Users-ken-dev-portty/66f0384b-8902-40e9-9d84-68c47b57f8f8/scratchpad/review2-vet.txt`). `go test -race -count=3 -run 'TestHubInput_' ./internal/relay/` returned ok (`.../scratchpad/review2-relay-input.txt`). No full suites were run.

### What the fixes get right

- **Lock order still holds.** `revokeInput`, `HoldInput` and `ReleaseInput` take only `inMu`. `RevokeWebViewers` releases `mu` before it calls `revokeInput`, and nothing takes `mu`, `gridMu` or `emuMu` under `inMu`. `unshareLocked` holds `a.mu` across the call, but `inMu` is a leaf and is never held across IO, so this is safe. IN-02 is complete: `Close`, the done watcher and the writer all log after unlocking. The writer re-locks and re-checks `inDead`/`inHeld` at the top of its loop, so a kill or hold that happens while it logs is seen.
- **No lost wakeups.** Every state change a waiting `Write` or the writer depends on calls `Broadcast`: a written frame, kill, revoke and release. `HoldInput` needs none because it only adds a wait.
- **The hub cap (WR-01) cannot deadlock.** A blocked `Write` waits only on the writer, never on another connection. A connection with nothing queued always gets one frame in. The only time no frame can drain is the takeover hold, and that ends at the old daemon's exit or after `maxFrozen()`.
- **WR-02 orders writes correctly.** The hold is set before `serveOn` (and before `hub.Start`). `WatchExit` returns -1 at once on ESRCH, so an old daemon that exits before the watch is registered cannot leave the hold stuck, and NOTE_EXIT fires at exit even if nobody reaps the process. Every failure path after the hold ends this process (`postStep` death, the watchdog's `die`, or "COMMITTED not delivered" while the old daemon is alive), so the release goroutine and the kqueue thread do not leak in production. `commit(false)` skips the hold only when `pidAlive` reports the old daemon gone; `kill(pid, 0)` still succeeds on a zombie, so "gone" means the process was really reaped. Both of the old daemon's writers (its FIFO writer, or a pre-Phase-10 build's blocking `WriteInput`) live inside the old process, so "old daemon exited" really does mean "old writes are done".
- **WR-04 is sound.** `Hub.Done` closes when the PTY read loop returns (`watchSessionExit`), and a write error on the master (EIO, no slave open) comes with a read EIO, so a hub whose input died while the hub stays live is at most a transient state. `readDone` now closes only when `conn.Read` fails, so the `hub.Done` branch reliably wins and sends 1000 "session ended".
- **The frontend fixes.** IN-04, IN-05 and IN-06 are correct. `connectHost = peerDNSName || hostname` (App.tsx:891), `remoteAdapter` maps `peerDNSName` from `peer.dnsName`, and `peerSnapshotHost` falls back to `peerHost`, so the tab key, the Sidebar `aria-current` key and the `dropPeerSession` key are now built the same way. The zoom path (the font-size effect, TerminalPanel.tsx:1031-1045) still claims unconditionally, so WR-03 does not affect zoom by menu or by Cmd+=.

### Critical Issues

#### CR-02: Stop Sharing still types the queued input of a web viewer that already disconnected (CR-01 is incomplete)

**File:** `internal/relay/hub.go:451-458` (targets come only from `h.subscribers`), `internal/relay/input.go:102-146`, `internal/webserver/server.go:886-890` and `939`
**Issue:** `revokeInput` drops only frames whose `InputConn.sub` is in `targets`, and `targets` holds only the viewers *currently subscribed*. When a web pump ends, `defer in.Close()` keeps its queued frames on purpose (input.go:102, "Its queued frames are still written"). The handler's deferred `hub.Unsubscribe(sub)` (webserver/server.go:887) then removes the viewer from `h.subscribers`. Its backlog stays in `h.inq`, with `inputRevoked == false`, and no later revoke can reach it.

That is the most common way a backlog exists at all. A pump that sits at its quota stops reading, so the server heartbeat (45 s) or the client's liveness probe drops the socket. The pump then closes with frames queued, and `relayClient` redials.

Failure scenario: a write-cap guest pastes a script while the foreground program is not reading stdin (`sleep 600`, a long `make`). The guest then closes the browser tab, or its connection drops at quota. The host sees zero web viewers and clicks Stop Sharing. `RevokeWebViewers` finds no targets, the diag line says `input-dropped frames=0`, and when `sleep` exits the shell runs the whole queued script. This is the same threat as CR-01. It is still present for any viewer that disconnected before the revoke, including one between reconnects.

A second path: `WritePump` can return first (ctx cancelled, or a write error). The handler returns and unsubscribes while the read-pump goroutine is still blocked in `in.Write` at quota. That pump is not a target either, so once it is unblocked it queues another frame after the revoke.

`TestHubInput_RevokeDropsBacklog` covers only a viewer that is still subscribed.
**Fix:** Revoke web input by origin and by epoch, not by subscriber set. A viewer's backlog belongs to the share whatever its socket state is. Both callers (`unshareLocked`, `remintSharesLocked`) revoke every web link of the session, so every web frame queued before the revoke should go:
```go
// hub.go, Hub (under inMu): webEpoch counts RevokeWebViewers calls.
webEpoch uint64

// input.go
type InputConn struct {
	// ...
	epoch uint64 // h.webEpoch when opened
}

func (h *Hub) OpenInput(sub *Subscriber) *InputConn {
	h.inMu.Lock()
	c := &InputConn{h: h, sub: sub, epoch: h.webEpoch}
	h.inMu.Unlock()
	if sub != nil {
		c.id = sub.ID
	}
	return c
}

// revoked: a web conn opened before the last revoke, or whose viewer was
// subscribed at it (Join before the revoke, OpenInput after).
func (c *InputConn) revoked() bool {
	return c.sub != nil && c.sub.Origin == "web" &&
		(c.epoch < c.h.webEpoch || c.sub.inputRevoked)
}

func (h *Hub) revokeInput(subs []*Subscriber) (frames, bytes int) {
	h.inMu.Lock()
	defer h.inMu.Unlock()
	h.webEpoch++
	for _, s := range subs {
		s.inputRevoked = true
	}
	// ... the existing compaction loop, unchanged: it now also drops closed
	// and unsubscribed web conns' frames, since revoked() is epoch-based.
}
```
`Origin` is set when the `Subscriber` is built and never changes, so reading it under `inMu` is safe. Add a test: a web conn queues frames on a stuck PTY, then `Close()`s and `Unsubscribe`s; next `RevokeWebViewers` runs; assert that at most the in-flight frame reaches the PTY. Also cover a pump blocked in `Write` after `Unsubscribe`.

### Warnings

#### WR-05: The hub cap makes a viewer that is only typing block behind another connection's backlog, and its keystrokes are lost when it is dropped

**File:** `internal/relay/input.go:84-87` (the `h.inBytes+len(b) > InputHubMaxBytes` term), with the shortcut comment at `input.go:27-31`
**Issue:** Before WR-01, a pump blocked only on its *own* quota, so a viewer that was just typing never stopped reading. Now any connection with one frame queued blocks while the hub holds 64 MiB, and those 64 MiB can all belong to other connections. A blocked pump does not read. On the share path it misses WebSocket pings (`relay.Heartbeat`, dropped after about 45 s). On the local relay it stops answering `MsgPing`, so `relayClient` probes after 40 s of silence and abandons the socket 5 s later. Every frame the client sent after the pump blocked is still unread in that socket and dies with it. `relayClient` never resends, and the DEBT-05 notice covers only split pastes, so the lost keystrokes are silent.

Failure scenario: this is exactly the WR-01 state. A guest's connection flaps during a large paste into a stalled reader, so a closed connection still holds about 32 MiB and its redial queues another 32 MiB. The owner, at home on the peer tab, presses Ctrl-C (one frame gets in), then types a few more keys. Its pump blocks on the hub cap and its pongs stop. After about 45 s the owner's socket is dropped, those keys are gone, and the tab shows "reconnecting". Before this fix, the same keys would have queued behind the backlog and landed once it drained. The shortcut comment still says only the pasting "viewer" can be dropped, which is no longer true.
**Fix:** Keep the cap for bulk input, but give each connection a small allowance that ignores it, so typing never blocks a pump:
```go
const inputHubExempt = 64 << 10 // per conn: typing and small pastes never wait on the hub cap
for !h.inDead && !c.revoked() && (c.frames >= InputMaxFrames ||
	(c.frames > 0 && (c.bytes+len(b) > InputMaxBytes ||
		(c.bytes+len(b) > inputHubExempt && h.inBytes+len(b) > InputHubMaxBytes)))) {
```
The bound becomes the cap plus 64 KiB per connection. If you keep the current behaviour instead, at least fix the shortcut comment so it says a viewer can be dropped because of *another* connection's backlog. Add a test: fill the hub to the cap with two connections, then check that a third connection's second small frame does not block.

### Info

#### IN-08: WR-03's focus gate stands in for "a person is here"; a key window on an unattended Mac passes it, and a real resize of an inactive window fails it

**File:** `frontend/src/components/TerminalPanel.tsx:1003-1010`
**Issue:** `document.hasFocus()` is true whenever Portty is the active app and its window is key. That says nothing about whether anyone is at the Mac. An office Air left unlocked with Portty in front stays focused. A display change while it is awake and unlocked therefore still claims: a monitor that powers itself off, a dock or KVM switch, or a resolution change. When the display sleeps or the screen locks, WebKit marks the window occluded and `visibilityState` becomes `hidden`, so the common case is covered and the remaining gap is narrow. The opposite case: macOS lets a user drag the edge of an *inactive* window without making it key, and that deliberate resize now only reports, so the user's panel stays scaled to the other viewer's grid. Zoom, fullscreen and the Window > Zoom menu all run with the window key, so they are not affected.
**Fix:** A tighter signal is "user input on this document within the last N seconds": record `pointerdown`/`keydown` timestamps on `window`, and claim only if `Date.now() - lastInputAt < 2000` or the window has focus *and* an input arrived since the last claim. At minimum, record this residual gap in the WR-03 comment.

#### IN-09: A revoked viewer can still move the grid until its socket closes

**File:** `internal/webserver/server.go:965-967` and `998-1003`; `internal/relay/hub.go:554-573` (`NoteTyping` checks only that the viewer is subscribed), `hub.go:542` (`ClaimGrid`/`sizeClient` does not check that)
**Issue:** CR-01 drops a revoked viewer's input, but the pump still calls `hub.NoteTyping(sub)` before `in.Write` (which is then dropped), and it still applies `MsgResize2` claims. The viewer stays subscribed until `go sub.Kick(...)` finishes the close handshake and the handler unsubscribes it. A guest client that ignores the close frame can resize the host's shell, and take grid ownership, for the length of the handshake timeout. Each claim also bumps `geomEnd` (full replays for later reconnects). This is minor: it changes only the grid, not the shell's input.
**Fix:** In `RevokeWebViewers`, under `mu`, either remove the targets from `h.subscribers` before kicking (so `NoteTyping`'s membership check fails), or set a `revoked` flag that `sizeClient` and `NoteTyping` check under `mu`. Also have `sizeClient` ignore viewers that are not subscribed, as `NoteTyping` already does.

---

_Iteration 2 reviewed: 2026-11-21T06:55:38Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
