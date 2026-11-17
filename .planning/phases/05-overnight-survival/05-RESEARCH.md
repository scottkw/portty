# Phase 5: Overnight Survival - Research

**Researched:** 2026-11-17
**Domain:** WebSocket liveness and reconnect (Go coder/websocket + browser WebSocket), terminal history replay (charmbracelet/x/vt to xterm.js), resumable output streams
**Confidence:** HIGH for code facts and measurements; MEDIUM for browser and OS sleep behavior

## Summary

Nothing reconnects today, and the code has three gaps the scout did not list. First, raising `liveEmulatorScrollbackLines` to 10,000 as written costs about **126 MiB per session** (measured: 10,000 lines × 107 columns × 112 bytes per `uv.Cell`). Every session builds its emulator eagerly from its first byte, so every session would pay that cost. Rendering the scrolled-off lines to ANSI strings as they leave the screen holds the same 10,000 lines in **1.1 MiB** (measured). Second, the current wrapped-ring replay (`RenderSnapshot`) joins rows with a bare `\n` and puts no cursor-position sequence at the end. xterm.js has `convertEol` off by default, so a main-screen replay comes out as a staircase and the cursor lands in the wrong column. That breaks Claude Code, which redraws its input box relative to the cursor. Third, `resizeLiveEmulator` throws the emulator away on every host resize, so any history stored inside the emulator would be lost at each window resize.

The recommended design has four parts and adds no new dependencies. (1) **Two-layer liveness.** The servers and the viewing daemon's proxy send WebSocket-level pings with a timeout and call `CloseNow` on a dead peer. Clients send the existing app-level `MsgPing` (0x12), which the server now answers with a new `MsgPong` frame, so JS and the CLI can see end-to-end liveness. (2) **One reconnect loop per client.** RelayClient (GUI and browser) and a new loop in `internal/attach` (CLI) back off from 1 s to 30 s and classify closes. Before each redial they re-authenticate: GUI and CLI re-run `/peers/connect` and `GetRelayPort`, and the browser probes `/api/sessions/{id}/info`. (3) **Resumable output.** The hub counts output by frame-byte offset, and a client reconnects with `?resume=<offset>`. If the ring still covers that offset, the server sends only the missed bytes, so the GUI keeps its xterm buffer with nothing duplicated (the locked decision). Otherwise the server sends a full replay with an explicit `replay:"full"` marker, and the client resets first. (4) **Hub-owned rendered history.** The emulator's scrollback is drained after every write into a 10,000-line, byte-capped string ring that survives emulator rebuilds and is cleared on `ESC[3J`. The full replay is sent in chunks of 16 KiB or less, and it ends by restoring the cursor position and the DEC modes the hub has tracked.

**Primary recommendation:** Build the resume-offset protocol and the hub history ring in `internal/relay` first. Every viewer (GUI, CLI, browser) depends on that one contract. Then wire liveness and reconnect into each client.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### Scrollback on reattach
- History stays in memory only: raise the live emulator's scrollback (`liveEmulatorScrollbackLines`, `internal/relay/hub.go:107`, currently 50) to ~10,000 lines, and when the raw ring has wrapped, `ReconnectPreamble` sends emulator scrollback + current screen instead of the screen alone. The GUI keeps its xterm buffer across a reconnect (no clear + duplicate). Nothing is written to disk; a daemon restart still loses history (as today). — **Reversibility:** costly — changes the reattach replay contract every viewer (GUI, CLI, browser) depends on, and per-session memory budget; disk-backed was rejected for privacy (terminal contents incl. secrets on disk).

#### Verification gate (VERF-01)
- Deferred by the user ("Let's defer for now"). Do NOT rewrite VERF-01 or ROADMAP success criteria in this phase. Build the reconnect/scrollback work with automated tests (forced drop, half-open/idle liveness, daemon restart, scrollback diff), and leave the human gate decision (real overnight vs accelerated + two-Mac sleep/wake) to verification time. Human two-Mac items go into the UAT/manual checklist as deferred-capable items.

#### Decided autonomously
- [Liveness] Q: "How are dead connections detected?" → Selected: "Servers (relay + webserver + daemon remote proxy as needed) send WebSocket-level pings with a read deadline; clients treat pongs/frames as liveness" — internal to relay/webserver, no persisted state
- [Liveness] Q: "CLI shows [reconnecting] after 5 s idle" → Selected: "cmd_attach.go watcher keys on ping/pong liveness, not only output frames" — one watcher
- [Reconnect] Q: "Retry timing" → Selected: "Exponential backoff 1 s to 30 s cap; browser online/visibilitychange triggers immediate retry" — constants in one client
- [Reconnect] Q: "Network drop vs real end" → Selected: "Close codes/reasons: drop retries; share-stopped and session-ended keep today's banners and stop" — builds on existing closeReasons.ts
- [Reconnect] Q: "Daemon restarted, cap store empty" → Selected: "Re-run /peers/connect before redialing (proxy or CLI)" — existing endpoint
- [Reconnect] Q: "Typing while disconnected" → Selected: "Input stays disabled; keystrokes never queued into a live shell" — matches current behavior, queueing could fire stale commands
- [Scope] Q: "Office Mac sleeping" → Selected: "Out of scope (CONV-01 v2); document caffeinate for manual testing" — docs only

### Claude's Discretion
- Reconnect UI (GUI tab + browser): Claude picks. Default plan: silent for brief blips, "Reconnecting…" banner after ~3 s, consistent with existing banner components (SessionEndedBanner style, macOS-native look); tab/sidebar status reflects it.
- Give-up policy: Claude picks. Default plan: retry forever (30 s cap) until share stopped or session ended — a morning wake must just work, which is the core value.

### Deferred Ideas (OUT OF SCOPE)
- VERF-01 gate rewrite (real overnight vs accelerated + two-Mac sleep/wake) — user deferred; decide at Phase 5 verification.
- Disk-backed scrollback surviving daemon restart — rejected for now (privacy).
- Keep-awake toggle (CONV-01, v2).

**Note on the scrollback decision (mechanism vs intent):** the decision's *outcome* is about 10,000 lines of history, kept in memory and replayed on reattach after the ring wraps. This research recommends reaching that outcome with a hub-owned *rendered* history ring that drains the emulator, not with `SetScrollbackSize(10000)` on the emulator. The literal change costs about 126 MiB per busy session (measured, see Pitfall 1). The planner should treat this as a "how" choice inside the decision. If the user wants the literal emulator setting regardless, they need to accept that memory cost.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SHAR-07 | A remote attach left idle for 12+ hours stays connected or reconnects automatically. Detaching and reattaching restores scrollback. | Liveness (Pattern 1), reconnect loops (Patterns 2, 6, 7), resume protocol (Pattern 3), hub history ring and full-replay format (Patterns 4, 5), CLI restructure (Pattern 8). Automated coverage in the Testing Strategy |
| VERF-01 | Overnight end-to-end test passes (Claude Code on the office Mac, home Mac GUI attach, idle overnight, typing next morning with scrollback, also opens from a browser). | Gate wording is deferred. Plan accelerated automated tests (forced drop, half-open, daemon restart, scrollback diff) plus deferred-capable manual items M-84..M-87 (Testing Strategy). Office Mac stays awake through `caffeinate` (docs only) |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- **Regression test convention (portty/CLAUDE.md, TESTING.md §6):** every new test file goes into the TESTING.md §2 suite manifest. Every behavior that can't be automated gets a new M-NN item in §5 (next free number: **M-84**; highest existing is M-83). Every new test file covering SHAR-07/VERF-01 gets a §4 traceability row (path column holds a repo-relative `.go/.ts/.tsx/.sh` path only). Run `bash tests/check-traceability-paths.sh` before committing.
- **Tech stack:** keep Go/Wails/React/tsnet. This is a strip-down, not a rewrite. No new relay servers or cloud dependencies; remote access stays tailnet only.
- **Security:** keep token-gated, tailnet-only share links and the CSP intact. Remote write access to a shell is powerful.
- **Deletion discipline (Chesterton's Fence):** the "never wired to auto-reconnect" comments (`TerminalPanel.tsx:227-232`, `SessionEndedBanner.tsx` doc) describe the old model. Update them and say why. Don't just delete them.
- **Simplicity:** the user wants radical simplicity (memory note), and Ponytail is active for executors. Use no new libraries, keep constants in one place per client, and use the smallest mechanism that meets the decision.
- **Go conventions:** `go fmt`, `golangci-lint`, context-aware functions (`ctx context.Context`).
- **JS/TS:** camelCase, PascalCase components, ESLint + Prettier, TypeScript types; Node package manager is `pnpm`.
- **Silent fallbacks forbidden (`or {}` rule):** a meta frame that fails to parse, a bad `resume` value or an unknown close must take an explicit path (full replay, retry), never be silently ignored into corrupt state.
- **GSD workflow:** edits happen through `/gsd-execute-phase`.
- **UAT in browser:** `/gsd-verify-work` browser checks use dev-browser. Native GUI checks run in the Tart VM `portty-test`, not on the host (memory note).
- **Testing:** aim for 80%+ coverage in critical components; integration tests for routes.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Output history (raw ring + rendered history + offsets) | Host daemon: `internal/relay` Hub | — | Only the hub sees every PTY byte. History must not depend on any viewer |
| Full replay / delta replay construction | Host daemon: Hub (`Join`) | Share server and relay server call it | One contract for all viewers, built atomically under `emuMu` |
| Reaping dead viewers (WS ping + timeout) | Host daemon: webserver (share server), relay server | TCP keepalive (≈150 s backstop) | The host must drop zombie subscribers (viewer count, fan-out) |
| Upstream liveness for proxied viewers | Viewing daemon: `remote_ws_proxy.go` | — | The proxy owns the tailnet socket. The GUI and CLI only see loopback |
| Capability refresh after a daemon restart | Viewing daemon: `/peers/connect` (RemoteCapStore) | GUI binding `ConnectPeerSession`, CLI `client.ConnectPeer` | The cap stays server-side (T-134-06-03). Clients only trigger the refresh |
| Reconnect loop, backoff, close classification | Client: RelayClient (GUI + browser), `internal/attach` (CLI) | — | Only the client knows it was interrupted and what it already shows |
| Resume offset bookkeeping | Client (counts frame bytes) | Hub (authoritative end offset in meta) | The client knows what it rendered; the server confirms the position after each replay |
| Reconnecting UI, input disable | Client: TerminalPanel + banner; CLI status bar | — | Presentation only |
| Re-auth probe (browser) | Browser: `GET /api/sessions/{id}/info?cap=` | Share server `requireCapability` | Tells revoked (401/403) or gone (404) apart from a network drop, which the WebSocket API cannot report |

## Standard Stack

No new libraries. Everything below is already in `go.mod` / `frontend/package.json`.

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/coder/websocket | v1.8.15 [VERIFIED: go.mod] | Server/client WS; `Conn.Ping`, `CloseNow`, `SetReadLimit`, `CloseError` | Already used by relay, webserver, proxy and CLI. `Ping` + concurrent reader is its documented liveness API [VERIFIED: conn.go:216-223 in module cache] |
| github.com/charmbracelet/x/vt | v0.0.0-20260615092313-b57e5e6d29bb [VERIFIED: go.mod] | Live per-hub emulator: scrolled-off lines, cursor, modes | Already feeds `RenderSnapshot`. Exposes `Scrollback()`, `CursorPosition()`, `IsAltScreen()`, `SetCallbacks(EnableMode/DisableMode)`, `RegisterCsiHandler`, `RegisterEscHandler` [VERIFIED: emulator.go, callbacks.go, handlers.go] |
| github.com/charmbracelet/ultraviolet | v0.0.0-20260615092913-2399af76d5b1 [VERIFIED: go.mod] | `uv.Line.Render()` turns a scrolled line into self-contained ANSI (resets style and hyperlink at the end of the line) | Same renderer `Emulator.Render()` uses [VERIFIED: buffer.go:135-197] |
| @xterm/xterm | 6.0.0 [VERIFIED: node_modules/@xterm/xterm/package.json] | Client terminal; `reset()`, `write(Uint8Array)` | Existing. `scrollback: 10000` already set (TerminalPanel.tsx:293) |
| vitest | 4.1.0 [VERIFIED: node_modules] | Frontend tests, `vi.useFakeTimers` | Existing |
| Go | go.mod `go 1.26.4`; local toolchain go1.27.4 | `testing/synctest` is available for pure-logic timer tests | Existing |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Resume-offset delta on reconnect | Client `term.reset()` + full replay on every reconnect | Much simpler, with no protocol change, but it breaks the locked "GUI keeps its xterm buffer, no clear" decision. It also loses scroll position and client-side mode state, and it replays MBs on every blip. Keep it only as the fallback path when the offset falls outside the ring |
| Hub-owned rendered history ring | `emu.SetScrollbackSize(10000)` (literal decision) | Measured 126 MiB per busy session vs 1.1 MiB. It also gets thrown away on every resize (`resizeLiveEmulator` rebuild) and has an O(n) `slices.Delete` per line once full |
| App-level `MsgPong` reply | Server-originated heartbeat frame every N s | A server heartbeat costs bandwidth on every idle session and can't measure RTT. Ping/pong lets the client control timing (needed for wake probes) |
| Hand-written backoff | cenkalti/backoff, reconnecting-websocket npm | About 15 lines each side, and both are new dependencies (simplicity rule). Not worth it |

**Installation:** none.

## Package Legitimacy Audit

This phase installs no external packages. Every capability uses modules already in `go.mod` / `frontend/package.json`.

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| *(none)* | — | — | — | — | — | — |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
HOME MAC (viewer)                                         OFFICE MAC (host, kept awake via caffeinate)
-----------------                                         -----------------------------------------
 GUI tab (WKWebView)          CLI `portty attach`          Browser (on any tailnet Mac)
   RelayClient                  attach.Run loop               RelayClient (wsURL ?cap=)
   | reconnect loop             | reconnect loop              | reconnect loop
   | app ping 0x12 / pong 0x04  | app ping 0x12 / pong 0x04   | app ping/pong
   | before redial:             | before redial:              | before redial:
   |  GetRelayPort()            |  client.GetRelayPort()      |  GET /api/sessions/{id}/info?cap=
   |  ConnectPeerSession()      |  client.ConnectPeer()       |   401/403 -> share stopped (stop)
   v  ?resume=<off>             v  ?resume=<off>              |   404 -> session ended (stop)
 ws://127.0.0.1:<relayPort>/api/relay/remote/<sid>/ws         |   network error -> retry
   |                                                          |
 VIEWING DAEMON proxy (remote_ws_proxy.go)                    |
   - cap from RemoteCapStore (refilled by /peers/connect)     |
   - forwards validated resume=<uint64> upstream              |
   - WS Ping upstream every 30 s (timeout 15 s) -> CloseNow   |
   - forwards upstream CloseError (code+reason) to client     |
   |                                                          |
   +--------- wss://office:7444/sessions/<sid>/ws?cap=&resume=+  (tailnet, WireGuard)
                                   |
                     SHARE SERVER (webserver.handleWSSRelay): requireAllowedOrigin -> requireCapability
                       - WS Ping viewer every 30 s (timeout 15 s) -> CloseNow -> Unsubscribe
                       - MsgPing 0x12 -> enqueue MsgPong 0x04
                       - hub.Join(sub, resume) under emuMu:
                           delta (resume within raw ring)  -> Meta{replay:"delta"}, raw bytes since offset
                           full  (no/old offset)           -> Meta{replay:"full"}, raw ring if intact,
                                                              else rendered history + screen + CUP + modes
                         chunks of 16 KiB or less, then Meta{offset:N}, then live frames from sub.Msgs
                                   |
                     HUB (one per PTY): Run() -> recordFrame (ring append, offset += len(frame),
                       emulator write, drain scrolled-off lines -> rendered history ring) -> broadcast
                       (record + broadcast under emuMu so Join is exactly-once)
                                   |
                                  PTY (zsh / Claude Code)
```

### Recommended file touch map
```
internal/relay/
  hub.go            # history ring, mode tracking, Join (atomic subscribe+replay), newLiveEmulator helper
  history.go        # NEW: rendered-line ring (lines cap + byte cap), Clear on ED3/RIS
  scrollback.go     # end offset counter; SnapshotFrom(off)
  protocol.go       # MsgPong = 0x04; MetaPayload.Replay, MetaPayload.Offset; chunk helper
  heartbeat.go      # NEW: Heartbeat(ctx, conn, every, timeout) -> CloseNow on failure
  close_reasons.go  # IsTerminalClose(err) helper (share stopped / session ended)
  server.go         # local relay: Join, pong reply, heartbeat, resume param
internal/webserver/server.go   # share server: same as relay server
internal/daemon/remote_ws_proxy.go  # forward resume; heartbeat upstream
internal/attach/    # NEW reconnect loop (Run), single stdin goroutine, offset tracking
cmd_attach.go       # runAttach uses attach loop; watcher replaced; re-connect seam
frontend/src/lib/relayClient.ts     # reconnect loop, pong/meta parsing, watchdog, resume
frontend/src/lib/closeReasons.ts    # isTerminalClose()
frontend/src/components/TerminalPanel.tsx   # reconnect state, banner, reset-on-full, stdin gating
frontend/src/components/ReconnectingBanner.tsx (NEW) or a SessionEndedBanner kind
frontend/src/components/WebShareSessionView.tsx + App.tsx  # beforeReconnect hooks
```

### Pattern 1: Two-layer liveness

**What:** (a) WS-level pings from every Go endpoint that owns a long-lived socket to a remote peer: the share server and relay server toward viewers, and the proxy toward the upstream. (b) App-level `MsgPing` from clients with a `MsgPong` reply, so JS (which can't see WS control frames) and the CLI (whose socket is loopback to the proxy) both see end-to-end liveness.

**Why both:** browsers answer WS pings automatically, but JS never sees them [CITED: RFC 6455 §5.5.2; ASSUMED for the "JS can't observe" detail, standard WebSocket API has no ping event]. The CLI's and GUI's WS ping only reaches the local proxy, because control frames are answered by the proxy's own reader and are not forwarded (copyWS forwards data messages only [VERIFIED: remote_ws_proxy.go copyWS uses `src.Read`, which consumes control frames internally]).

**coder/websocket rules [VERIFIED: module cache conn.go:216-258, read.go:20-48, close.go:86-140]:**
- `Ping(ctx)` "must be called concurrently with Reader". Every endpoint here already runs a read goroutine (relay/webserver read pump, proxy `copyWS` on both sides, CLI `WsOutputPump`), so pongs get processed.
- A context that expires during `Read` **closes the connection** (`setupReadTimeout` → `c.close()`). Never put a short deadline on the read pump's ctx. Use Ping for liveness.
- `Close()` writes the close frame with a 5 s timeout and then waits 5 s for the peer. Use `CloseNow()` for a dead peer.

**Timing (discretion, all constants in one place):** server/proxy ping every **30 s**, timeout **15 s**, so a dead peer is reaped within 45 s. Client app ping every **15 s**. The client declares the socket dead when no frame of any kind has arrived for **40 s** *and* the page is visible. On wake, visibility or online: send a probe ping, then reconnect if nothing arrives within **5 s**.

**Version-skew guard:** arm the client watchdog only after the **first `MsgPong`** arrives. An older host never sends one, so a new client would otherwise reconnect every 40 s.

```go
// Source: pattern from coder/websocket Ping docs (conn.go:216-223); new file internal/relay/heartbeat.go
func Heartbeat(ctx context.Context, c *websocket.Conn, every, timeout time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			pctx, cancel := context.WithTimeout(ctx, timeout)
			err := c.Ping(pctx) // needs a concurrent Read; every call site has one
			cancel()
			if err != nil {
				c.CloseNow() // no handshake with a dead peer (Close waits 5 s + 5 s)
				return
			}
		}
	}
}
```

Server pong reply (read pump, webserver and relay):
```go
case relay.MsgPing:
	select { // never block the read pump; a full queue means a kick is coming anyway
	case sub.Msgs <- relay.PongFrame:
	default:
	}
```

### Pattern 2: Client reconnect loop (state machine)

States: `connecting → open → reconnecting → (open | stopped)`, plus `closedByUser`.

- On close or watchdog death: if `closedByUser`, stop. If the close is terminal (Pattern 6), go to `stopped` and show today's banner. Otherwise go to `reconnecting`, wait `backoff` (1, 2, 4, 8, 16, 30 s cap, ±20 % jitter), run `beforeReconnect()` (Pattern 7), then redial with `?resume=<offset>`.
- **Reset backoff only after the connection has been healthy ≥ 30 s** (or after receiving the post-replay `offset` meta and one live frame), not on `onopen`. Otherwise a "too slow" kick loop becomes a 1 Hz storm.
- `online` and `visibilitychange→visible`: if `reconnecting`, skip the remaining backoff and retry now. If `open`, send a probe ping (5 s grace).
- **Abandon a dead socket without waiting:** null out its `onclose/onmessage/onerror`, call `close()`, and dial the new one at once. A browser `close()` on a half-open socket can sit in CLOSING for a long time [ASSUMED].
- **Watchdog uses wall-clock deltas (`Date.now()`)**, not tick counts. Chrome throttles hidden-tab timer chains to one wake-up per minute after 5 minutes hidden [CITED: developer.chrome.com/blog/timer-throttling-in-chrome-88], and JS/Go timers do not run during system sleep. Never decide "dead" from a large delta alone (after a clock jump the socket may be fine). Send a probe and wait the 5 s grace.
- Input: while not `open`, set `term.options.disableStdin = true` and drop `onData` output. Never queue it (locked decision). Re-enable on `open`.
- Reconnect is **opt-in** (constructor option). Host-path local tabs keep today's behavior. A local relay drop means the daemon died, and the session died with it.

### Pattern 3: Resume-offset protocol (exactly-once, no duplicate)

**Offsets are in frame bytes**, the unit the raw ring already stores (`Scrollback.Append(frame)` [VERIFIED: hub.go recordFrame]). Every live `MsgOutput` frame is `1 + len(payload)` bytes, so a client adds `payload.length + 1` per live output frame. The ring keeps its framed format, because `engine.go` `GetSessionTailLines`/`GetSessionStyledTailLines` depend on `ScrollbackSnapshot()` returning framed bytes [VERIFIED: engine.go:472, 532].

Wire additions (new values are proposals, `[ASSUMED]` until written):
- `MsgPong byte = 0x04`: server→client. It sits in the server→client range next to `MsgOutput 0x01`, `MsgResize 0x02` and `MsgTitle 0x03`; 0x04 is unused [VERIFIED: protocol.go:11-22 quoted below].
- `MetaPayload` gains `Replay *string \`json:"replay,omitempty"\`` (`"full"` | `"delta"`) and `Offset *uint64 \`json:"offset,omitempty"\``. Old clients ignore unknown meta fields: RelayClient drops 0x20 entirely (`default: unknown`), and the CLI reads only `ViewerCount` [VERIFIED: relayClient.ts parseServerFrame; attach.go WsOutputPump].
- Query parameter `resume=<decimal uint64>` on `GET /sessions/{id}/ws` (relay and share server) and `GET /api/relay/remote/{sid}/ws` (proxy forwards it after `strconv.ParseUint` validation).

Join sequence (server, after the existing VIEW-03 grid frame):
```
MsgResize(grid)                         // unchanged, still first
Meta{"replay":"delta"|"full"}           // client resets xterm before writing if "full" AND it already has content
MsgOutput chunk ... (each payload <= 16 KiB)
Meta{"offset": N}                       // N = ring end offset captured atomically with Subscribe
live frames from sub.Msgs (all strictly after N)
```

**Atomicity:** today `Subscribe` happens before the snapshot, and `recordFrame` and `broadcast` are separate steps. A frame recorded between them is both in the snapshot and in `sub.Msgs`, which is a small duplicate window that exists today. Fix: in `Run`, hold `emuMu` across `recordFrame` **and** `broadcast`, which keeps the lock order `emuMu → hub.mu` already used through `h.Cols()`. Add `Hub.Join(sub, resume)`, which takes `emuMu`, subscribes, captures (bytes, end) and builds the replay, then releases. Every frame then lands in exactly one of {replay, live}. Rendering history under `emuMu` is a string concatenation over at most 10,000 strings and stalls the drain loop for milliseconds only.

Delta rules: `start = end - len(ring)`. If `start <= resume <= end`, send `ring[resume-start:]`, run through the existing `liveEmuQueryStripPattern` (stale queries must not be answered hours later), with `replay:"delta"`. Otherwise (resume missing, older than the ring, larger than end, or unparsable) send a full replay. An unparsable `resume` is a full replay, not an error (explicit path, no silent corruption).

Old-host fallback in the client: if the client sent `resume` and an output frame arrives before any `replay` meta, the host is old. The client resets and treats that output as a full replay.

### Pattern 4: Hub-owned rendered history (the decision's ~10,000 lines)

```go
// Source: measured design (scratch benchmark this session); emulator APIs VERIFIED in module cache
// recordFrame, under emuMu, after h.liveEmu.Write(clean):
sb := h.liveEmu.Scrollback()           // main screen scrollback (emulator.go:451)
if n := sb.Len(); n > 0 {
	ls := sb.Lines()                    // returns the INTERNAL slice (scrollback.go:112-117)
	for _, l := range ls {
		h.history.Push(l.Render())      // self-contained ANSI per line
	}
	clear(ls[:cap(ls)])                 // drop stale uv.Line refs in the backing array (Pitfall 2)
	sb.Clear()
}
```

- `history`: ring of strings, **max 10,000 lines and max 4 MiB** (byte cap is a discretion value [ASSUMED]; it bounds truecolor-heavy lines, which can reach several KB each).
- Keep the emulator's own scrollback max **equal to the history line cap (10,000)**, not 50. It is drained after every write, so it stays near empty. The O(n) `slices.Delete` in `Push` [VERIFIED: scrollback.go:50-53] fires only if one 32 KiB PTY read pushes more than 10,000 lines, and then only the newest 10,000 survive either way.
- **Clear history on `ESC[3J`** with `RegisterCsiHandler('J', ...)`. Read param 0, and if it is 3 clear `h.history`, then `return false` so the default handler still runs [VERIFIED: handlers chain in reverse, returning false falls through, handlers.go:55-60, 95-110; verified working in scratch test]. Claude Code's clear routine emits `ESC[2J ESC[3J ESC[H` on `/clear`, `/compact` and some redraws [CITED: claudeissues.com reports; LOW confidence on current version behavior]. Honoring 3J keeps the replay from showing the transcript twice after Claude Code reprints it. Optionally do the same for RIS (`RegisterEscHandler('c', …)`).
- **Mode tracking:** `SetCallbacks(vt.Callbacks{EnableMode, DisableMode})` writes into a hub-owned `map[ansi.Mode]bool`, so the map survives emulator rebuilds [VERIFIED: callbacks.go; scratch test showed modes 1, 25, 2004 reported].
- **One constructor:** a `newLiveEmulator(cols, rows)` helper sets the scrollback size, callbacks and the `J` handler. Use it at all three construction sites: `recordFrame` eager build (:424), `resizeLiveEmulator` (:465) and `EnsureLiveEmulator` (:548) [VERIFIED: hub.go lines]. If one site misses the hook, history silently stops clearing or tracking after the first resize.
- **Resize:** before `resizeLiveEmulator` discards a **main-screen** emulator, push its non-empty screen rows (up to the cursor row) into `history`, or the visible lines at resize time vanish from history. Keep the rebuild itself, because it is the M-51 fix for alt-screen TUIs. Accept a few duplicated prompt lines at resize points.

### Pattern 5: Full-replay byte format (wrapped ring)

The raw-ring-intact path stays byte-faithful raw replay, which is the #109 fix (Chesterton: keep it). When the ring has wrapped:

```
main screen:   history lines joined "\r\n" + "\r\n"
               + all Height() screen rows joined "\r\n" (keep trailing blank rows, no newline after last)
               + CSI <row+1>;<col+1> H                  (emu.CursorPosition())
               + DEC mode restores (CSI ? n h for tracked non-default modes; CSI ? 25 l if hidden)
alt screen:    history lines joined "\r\n" + "\r\n"
               + CSI ? 1049 h + alt screen rows "\r\n" + CUP + mode restores
```
- **CRLF is required.** `uv.Lines.Render()` joins with bare `\n` [VERIFIED: ultraviolet buffer.go:230-239; scratch: `Render()` = `"first\nsecond\n%\n\n"`], and xterm.js defaults to `convertEol: false`, where LF only moves down [VERIFIED: xterm.mjs default `convertEol:!1`, LF handler sets x=0 only if convertEol]. The current `RenderSnapshot` therefore already staircases on main-screen wraps.
- **CUP is required:** the trailing space of `"% "` is trimmed, so without CUP the client cursor sits one column left of the host's (scratch: cursor reported (2,2)). Claude Code (Ink) redraws its input area with relative cursor moves [ASSUMED], so a wrong cursor garbles every keystroke.
- Never emit `?2048` (in-band resize, stripped on purpose) or `?1047/?1049/?47` from the mode map (handled by `IsAltScreen`).
- The main-screen rows under an alt screen can't be rendered (`Render`/`CellAt` use the current screen; `scrs` is unexported). Accept that this content is lost, as it is today.
- Send as `MsgOutput` frames with **payload ≤ 16 KiB** (any value up to 32767 bytes fits coder/websocket's 32768-byte default per message). This closes the pending todo "Send the catch-up replay in chunks of 32 KiB or less". Mid-sequence splits are safe, because live frames already split at arbitrary 32 KiB PTY read boundaries and every client copes [VERIFIED: hub.go Run frames `buf[:n]`].

### Pattern 6: Close classification (one helper per language)

| Close | Seen as | Action |
|-------|---------|--------|
| 1000 + `"share stopped"` | GUI, CLI, browser (proxy forwards CloseError) | **Stop**: stopped-sharing banner / message (today's behavior) |
| 1000 + `"session ended"` | all | **Stop**: session-ended banner |
| client-initiated (unmount, tab close, detach key) | — | Stop, no banner |
| 1006 abnormal (TCP drop, proxy `CloseNow`, 404 before upgrade) | all | **Retry** |
| 1008 `"too slow"`, 1013 `"remote unreachable"`, 1011 `"bad base url"`, 1001, anything else | all | **Retry** |
| watchdog / probe timeout | all | Abandon socket, **retry** |

Keep reasons on 1000 (the existing contract, pinned by tests on both sides). Don't invent new 4xxx codes. `TerminalPanel` today treats *every* guest close as "session ended" (`onClose` at :381-390). That shows the wrong copy for a network drop, and this phase fixes it. `TerminalPanel.guestClose.test.tsx` pins "any other close keeps the session-ended banner" and must be rewritten.

### Pattern 7: Re-authenticate before each redial

- **GUI remote tab:** call `GetRelayPort()` first, because the relay listener is `127.0.0.1:0` and its port changes when the daemon restarts [VERIFIED: daemon/api.go:187 `net.Listen("tcp", "127.0.0.1:0")`]. Then call `ConnectPeerSession(peerDNSName||host, sessionId)`, which refills the RemoteCapStore and re-checks WhoIs and that the share is on. Map `errorCode`: `not_sharing`, `no_such_session` → stop (stopped-sharing / ended banner); `forbidden` → stop; `unreachable`, `tailscale_off`, `unknown_host`, binding throw → retry. Error code values [VERIFIED: internal/daemon/types.go:158-167]: `PeerErrTailscaleOff = "tailscale_off"`, `PeerErrUnknownHost = "unknown_host"`, `PeerErrNotSharing = "not_sharing"`, `PeerErrNoSuchSession = "no_such_session"`, `PeerErrAmbiguous = "ambiguous"`, `PeerErrMultiple = "multiple"`, `PeerErrForbidden = "forbidden"`, `PeerErrUnreachable = "unreachable"`. Wire this as a `beforeReconnect` callback from App.tsx, which knows `peerHost`/`peerDNSName`, through WebShareSessionView to TerminalPanel to RelayClient.
- **CLI remote:** `client.GetRelayPort()` + `client.ConnectPeer(typedHost, res.SessionID)` (exact ID resolves without ambiguity, resolvePeerSession), with the same mapping.
- **Browser:** reuse the bootstrap probe `GET /api/sessions/{id}/info?cap=` (App.tsx:760-775 already maps 401/403 → link dead). 401/403 → stopped-sharing; 404 → ended; ok → redial; network error → retry. Caveat: if the host turned Share off while the browser was asleep **and** no other session is shared, the share server's port is closed. The probe gets a network error, and the browser keeps showing "Reconnecting…". That is acceptable under retry-forever; document it.

### Pattern 8: CLI loop restructure (`internal/attach`)

- **One long-lived stdin goroutine** reads `os.Stdin` into a channel. A blocking `Read` on stdin can't be cancelled, so starting a `StdinPump` per connection would leak goroutines and steal keystrokes.
- A per-connection writer forwards input while `open`, discards it while reconnecting (never queued), and **still checks the detach key** while reconnecting, so Ctrl-\ always exits.
- The status bar takes its state from the loop (`SetConnectionState("reconnecting")` / `""`). The 5 s no-frame watcher at cmd_attach.go:291-321 is deleted. Liveness is a `MsgPing` every 15 s with `MsgPong` counted as a frame (armed after the first pong). This is the decided fix for the UAT-gap-1 idle false positive.
- On `replay:"full"` after a reconnect, the CLI writes `\x1b[2J\x1b[H` before the replay (same as the first attach today, cmd_attach.go:249). On `replay:"delta"` it just writes.
- Raw mode and the bar persist across reconnects. Session ended on remote prints a message like the share-stopped path (`remoteAttachEnd`) instead of "Detached.".
- Local attach (`remote=false`) keeps a single connection (no reconnect).

### Anti-Patterns to Avoid
- **Read-pump context deadlines as liveness:** in coder/websocket an expired Read ctx closes the conn. Use Ping.
- **Raising emulator scrollback in place:** see Pitfall 1.
- **Waiting for `onclose` on a socket you've declared dead:** abandon it and dial.
- **Resetting backoff on `onopen`:** this creates storm loops with "too slow" or immediate-close servers.
- **Queueing keystrokes while disconnected:** forbidden by the locked decision; they could fire stale commands.
- **Classifying from close code alone:** 1000 is also used for client "detach". Classify on code + reason + who initiated.
- **Rendering wire text:** banners keep fixed copy. `reason` is for branching only (T-175-06-02 rule).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| WS liveness probe | Custom timestamped app frames on the Go side | `websocket.Conn.Ping` + `CloseNow` | Built-in, answered by any RFC 6455 peer including old Portty and browsers |
| ANSI rendering of scrolled lines | Own SGR/OSC-8 serializer | `uv.Line.Render()` | Handles style diffs, hyperlinks, wide-char placeholder cells, and resets at end of line |
| Mode tracking | Regex over the raw stream for `CSI ? n h/l` | `vt.Callbacks{EnableMode, DisableMode}` | Splits across reads, alt-screen switches and RIS are already handled by the parser |
| ED3 detection | Regex over raw PTY bytes | `RegisterCsiHandler('J', …)` returning false | Parser-accurate and handles sequences split across reads |
| Half-open TCP detection | OS socket options tuning | WS Ping (30 s/15 s) on top of Go's default TCP keepalive | Go already enables keepalive 15 s idle / 15 s interval / 9 probes ≈ 150 s [VERIFIED: GOROOT net/dial.go:18-27]; ping gives 45 s and app-level coverage |
| Backoff library | — | 10–15 lines per client | No new dependencies (simplicity rule) |

**Key insight:** the emulator already knows the cursor, the modes and the scrolled lines. The work is moving that state into a compact, rebuild-proof hub structure and replaying it faithfully, not re-parsing bytes.

## Common Pitfalls

### Pitfall 1: Emulator scrollback memory (measured)
**What goes wrong:** `SetScrollbackSize(10000)` keeps `uv.Line` cell slices. `unsafe.Sizeof(uv.Cell{}) = 112` bytes, and 10,000 lines of about 107 visible columns measured a **126.6 MiB** heap delta. The same history as rendered strings measured **1.10 MiB** (scratch benchmark, this session).
**Why:** `uv.Cell` holds a content string, style colors as interfaces, link strings and width per cell. The emulator is built eagerly for *every* session (M-51 fix in recordFrame).
**How to avoid:** Pattern 4 drain into rendered strings with a byte cap.
**Warning signs:** daemon RSS growing by about 100 MB per busy session; a heap test in hub_test exceeding its bound.

### Pitfall 2: Stale-line retention after `Scrollback.Clear()` (measured)
**What goes wrong:** `Clear()` is `s.lines = s.lines[:0]` [VERIFIED: scrollback.go:120-124]. The backing array still references every old `uv.Line`. After one 9,000-line burst, a plain Clear kept **110.2 MiB** live. Zeroing the backing array (`clear(ls[:cap(ls)])`) brought it to **1.5 MiB** (measured).
**How to avoid:** zero the full capacity before `Clear()`. Add a regression test that measures heap after a burst followed by a drain.

### Pitfall 3: Alt-screen scrollback grows unbounded today (pre-existing, measured)
**What goes wrong:** every vt `Screen` is created with `NewScrollback(DefaultScrollbackSize)` = 10,000 lines [VERIFIED: screen.go:23-27, scrollback.go `DefaultScrollbackSize = 10000`]. `Emulator.SetScrollbackSize` only touches the main screen (`e.scrs[0]`, emulator.go:477-479). A TUI scrolling in the alt screen grew the heap by **126.6 MiB** (20,000 scrolled lines, measured). The alt-screen scrollback is never read by anyone.
**Why it matters here:** overnight sessions running a scrolling alt-screen TUI (less, logs in a TUI) hold this memory indefinitely. Only a resize rebuild resets it today.
**How to avoid:** no public API reaches `scrs[1]`. Options: (a) accept and document; (b) rebuild the emulator on `AltScreen(false)` (callback) after draining main history. That costs the main-screen rows, which are mostly already in history. (c) upstream issue/PR. See Open Questions. Claude Code itself runs on the main screen [ASSUMED], so the core flow is unaffected.

### Pitfall 4: Bare-LF replay staircase + wrong cursor (pre-existing, verified)
See Pattern 5. The existing wrapped-ring tests assert `bytes.Contains`, so they never caught it (scrollback_altscreen_test.go). Add exact-format tests: CRLF between rows, CUP at the end, and a Height()-row viewport.

### Pitfall 5: History lost on every host resize
`resizeLiveEmulator` builds a new empty emulator [VERIFIED: hub.go:459-468]. History must live outside the emulator (Pattern 4), and visible main-screen rows must be pushed into it before the rebuild.

### Pitfall 6: Viewer read limits vs bigger replays
10,000 lines of about 110 bytes each is about 1.1 MiB (up to the 4 MiB cap). That **exceeds `ViewerReadLimit` = 1 MiB** (`const ViewerReadLimit = 4 * DefaultScrollbackBytes`, `const DefaultScrollbackBytes = 256 * 1024` [VERIFIED: scrollback.go:6, 25]). Sent as one message, it would bring back UAT gap 4 (proxied/CLI joiners dropped). Chunking (Pattern 5) is mandatory, not a nice-to-have. Keep the 1 MiB limit as a backstop.

### Pitfall 7: Duplicate output on reconnect
Without Pattern 3, every reconnect re-sends up to 4 MiB and the GUI shows history twice, which violates the locked decision. A subtler issue: the existing subscribe-then-snapshot ordering already allows a duplicated frame (Pattern 3 atomicity).

### Pitfall 8: Version skew between the two Macs
A new client against an old host gets no `MsgPong` and no `replay` meta. Guard the watchdog behind the first pong, and treat "output before replay meta" as a full replay (Pattern 3). Document that both Macs should run the same build. The CLI and GUI already require it for `/peers/connect`.

### Pitfall 9: Timers during sleep and hidden tabs
Go on darwin uses `mach_absolute_time` for its monotonic clock [VERIFIED: GOROOT runtime/sys_darwin_arm64.s nanotime_trampoline]. That clock does not advance during sleep [CITED: golang.org/issue/24595], so Go tickers resume where they paused and fire within one interval of waking. Chrome throttles hidden tabs to one wake-up per minute after 5 minutes [CITED: developer.chrome.com/blog/timer-throttling-in-chrome-88]. Use wall-clock deltas, probe before declaring death, and use visibility and online events for fast checks.

### Pitfall 10: Retry storms and preamble-time "too slow" kicks
The host writes the replay synchronously before draining `sub.Msgs` (buffer 256 [VERIFIED: relay/server.go:87, webserver/server.go:791]). On a slow DERP-relayed path with a busy session, the queue can fill during a multi-MB full replay → 1008 "too slow" → retry → loop. Mitigations: resume-delta makes most replays tiny, backoff resets only after 30 s healthy, and jitter. Watch the existing `viewer-diag: slow` log line in the manual tests.

### Pitfall 11: Revoked cap retried forever in the browser
WebSocket upgrade failures (401/403) show up only as close 1006. Without the `/info` probe (Pattern 7), a browser that slept through a share-off retries forever. The probe fixes every case except the share server's port being fully closed.

### Pitfall 12: Forgetting the hook on emulator rebuild
Callbacks and the `J` handler are per-emulator instance. Use the single `newLiveEmulator` helper (Pattern 4) and test that history clearing and mode tracking still work *after* a resize.

### Pitfall 13: Tests pinning old behavior
`TerminalPanel.guestClose.test.tsx` ("any other close keeps the session-ended banner"), the relayClient onClose tests, and the hub preamble tests (trimmed-RenderSnapshot contract, `ReconnectPreamble` single-frame return) will change. Plan their rewrite explicitly, not as incidental fixes.

## Code Examples

### Server join (share server; relay server mirrors it)
```go
// Source: proposal built on webserver/server.go:833-851 existing VIEW-03 ordering
resume, hasResume := parseResume(r.URL.Query().Get("resume")) // strconv.ParseUint(s, 10, 64); bad -> hasResume=false
replay := hub.Join(sub, resume, hasResume) // atomic subscribe + replay under emuMu (replaces Subscribe + EnsureLiveEmulator + ReconnectPreamble)
defer func() { hub.Unsubscribe(sub); relay.NotifyViewerCount(hub) }()
// grid frame first (unchanged), then:
writes := append([][]byte{relay.MakeMeta(relay.MetaPayload{Replay: &replay.Mode})}, replay.Chunks...)
writes = append(writes, relay.MakeMeta(relay.MetaPayload{Offset: &replay.End}))
for _, f := range writes {
	if err := conn.Write(ctx, websocket.MessageBinary, f); err != nil {
		return
	}
}
go relay.Heartbeat(ctx, conn, relay.PingEvery, relay.PingTimeout)
```
(Keep the TOCTOU grant re-check that runs *after* subscribing, webserver/server.go:820-827, in the same position relative to `Join`.)

### Proxy resume pass-through
```go
// Source: remote_ws_proxy.go:86 today: u.RawQuery = "cap=" + url.QueryEscape(capToken)
q := url.Values{"cap": {capToken}}
if s := r.URL.Query().Get("resume"); s != "" {
	if _, err := strconv.ParseUint(s, 10, 64); err == nil {
		q.Set("resume", s)
	}
}
u.RawQuery = q.Encode()
// after upstream dial + SetReadLimit:
go relay.Heartbeat(ctx, upstream, relay.PingEvery, relay.PingTimeout)
```
`redact` already replaces the whole `u.RawQuery`, so the new parameter stays out of logs.

### RelayClient frame handling (sketch)
```ts
// Source: proposal on frontend/src/lib/relayClient.ts parseServerFrame
case MSG_PONG: this.lastFrameAt = Date.now(); this.pongSeen = true; break
case MSG_META: {
  let m: unknown
  try { m = JSON.parse(new TextDecoder().decode(payload)) } catch { this.forceFullOnNextOutput = true; break }
  const meta = m as { replay?: unknown; offset?: unknown }
  if (meta.replay === 'full' && this.hasRendered) this.callbacks.onReset?.()   // term.reset()
  if (typeof meta.offset === 'number' && Number.isSafeInteger(meta.offset)) this.offset = meta.offset
  break
}
case MSG_OUTPUT: this.offset += payload.length + 1; this.hasRendered = true; this.callbacks.onOutput(payload); break
```

### Constants in one place (proposal)
```ts
export const RECONNECT = { minMs: 1_000, maxMs: 30_000, jitter: 0.2, healthyResetMs: 30_000,
  pingMs: 15_000, deadMs: 40_000, probeGraceMs: 5_000, bannerDelayMs: 3_000 } as const
```
```go
const ( PingEvery = 30 * time.Second; PingTimeout = 15 * time.Second
        HistoryMaxLines = 10_000; HistoryMaxBytes = 4 << 20; ReplayChunk = 16 << 10 )
```
Make the Go intervals overridable in tests (package vars or Server fields), so tests run with roughly 50 ms ping and 100 ms timeout.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| One socket per view, every close = "session ended" | Reconnect loop with close classification | This phase | Overnight core value |
| Wrapped ring → current screen only (`RenderSnapshot`, 50-line emulator scrollback) | Rendered 10k-line history + screen + CUP + modes | This phase | Real history after wrap |
| Replay as one WS message (needs 1 MiB read limit) | Chunks of 16 KiB or less + offset meta | This phase | Closes WR-03 todo; any reader limit works |
| CLI `[reconnecting]` after 5 s with no output | Loop-driven state + ping/pong liveness | This phase | Fixes UAT gap 1 false positive |

**Deprecated/outdated in this tree:**
- `web/terminal.html` + `web/assets/terminal.js`: still embedded, but `GET /sessions/{id}` redirects to `/app/` (webserver/server.go:722-730), so the browser client is the React app. Do **not** add reconnect to terminal.js. Leave it alone (Chesterton), or flag it for a later removal phase.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `MsgPong = 0x04`, `Meta.replay/offset`, `?resume=` names and values | Pattern 3 | Cosmetic; pick once and pin in tests on both sides |
| A2 | History byte cap 4 MiB; ping 30 s/15 s; client ping 15 s, dead 40 s, probe 5 s; backoff 1–30 s ±20 %, healthy reset 30 s; banner 3 s; chunk 16 KiB | Patterns 1, 2, 4, 5 | Tuning only; all constants in one place |
| A3 | A browser `close()` on a half-open socket can stay in CLOSING for a long time, so abandon instead of waiting | Pattern 2 | If wrong, abandoning is still harmless |
| A4 | JS cannot observe WS protocol pings | Pattern 1 | None: app-level pong works either way |
| A5 | Claude Code redraws its input area with relative cursor moves (so CUP restore matters) and runs on the main screen | Patterns 4/5, Pitfall 3 | If it uses absolute positioning, CUP is still correct; if alt screen, Pitfall 3 matters more |
| A6 | Claude Code emits `ESC[2J ESC[3J ESC[H` on /clear, /compact, some redraws (community reports, version-dependent) | Pattern 4 | If it never emits 3J, history just keeps more; if it emits it often, history is often legitimately short. Verify with the real Claude Code in M-84 |
| A7 | `tart suspend` works for the macOS guest on this host and resumes with TCP state intact | Testing Strategy | Fall back to `kill -STOP/-CONT` of the daemon in the VM |
| A8 | macOS default pf configuration filters lo0 (for a pf blackhole test) | Testing Strategy | Use the `kill -STOP` variant, which needs no root |
| A9 | Version skew is rare (single user), so the "output before replay meta = old host" fallback suffices | Pitfall 8 | Duplicate history after a reconnect to an old host; documented |

## Open Questions

1. **Alt-screen scrollback leak (Pitfall 3, pre-existing): fix in this phase?**
   - What we know: 126.6 MiB for 20k scrolled alt-screen lines. There is no public API to cap `scrs[1]`.
   - What's unclear: whether the user runs long scrolling alt-screen TUIs overnight.
   - Recommendation: add a heap soak test that documents it, and do the cheap mitigation (rebuild on `AltScreen(false)` after draining) only if the planner has room. Otherwise record it in deferred-items and file an upstream issue.
2. **Visible main-screen rows at resize time: push to history or not?**
   - Recommendation: push (avoids losing the visible screen), accept a few duplicated prompt lines. Pin it with a test.
3. **Wake signal for the GUI:** an optional Wails event from `NSWorkspaceDidWakeNotification` would trigger an immediate probe. Without it, detection takes up to one ping interval (≤15 s app ping / ≤45 s proxy) after wake. Recommendation: skip it (simplicity) unless the manual test shows an annoying delay.
4. **VERF-01 gate:** deferred by the user. Plan both the overnight item (M-84) and the accelerated items, and decide at verification.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | all Go work/tests | ✓ | go1.27.4 (go.mod 1.26.4) | — |
| Node / pnpm | frontend build/tests | ✓ | v24.22.1 / 12.6.0 | — |
| vitest | frontend tests | ✓ | 4.1.0 | — |
| Tart + VM `portty-test` | VM browser reconnect / suspend check | ✓ | tart 2.41.1; VM running | Go harness only |
| caffeinate / pmset / pfctl | Manual overnight + blackhole checks | ✓ | system | `kill -STOP` variant |
| Tailscale in VM | GUI peer tab / CLI remote in VM | ✗ (vmpreview fakes only the share server, `IP: "127.0.0.1"`) | — | Go in-process harness (attach_viewers_test style) + two-Mac manual items |
| Two Macs on the same tailnet | VERF-01 | User's hardware | — | Deferred-capable manual items |

**Missing dependencies with no fallback:** none for automated work. The two-Mac checks need the user.

## Testing Strategy (accelerated coverage; nyquist_validation is off, so this replaces the Validation Architecture section)

**Go (fast, in-process; `go test -race -short ./...`):**
1. `internal/relay` history: 10,000-line retention, byte cap trims oldest, ED3 clears (also after a resize rebuild), RIS optional, modes tracked across rebuild, CRLF + full-height viewport + CUP exact bytes, alt-screen variant, a heap bound after a 9k-line burst (Pitfall 2) and an emulator drain test.
2. `internal/relay` Join: exactly-once under concurrent output (writer goroutine spamming numbered lines; client concatenation must equal the PTY stream, no dupes or gaps, `-race`), delta when resume is in the ring, full when out of range/garbage/future, chunks ≤ 16 KiB, meta ordering (replay before output, offset after).
3. Server liveness: a **blackhole TCP forwarder** in the test (accepts, forwards, then `Freeze()` stops copying without closing). The server reaps the frozen viewer within ping+timeout (shortened to ~50/100 ms) and the hub subscriber count drops. `Kill()` (closes both sides) gives a prompt close. `MsgPing` → `MsgPong`.
4. Proxy: resume forwarded only when numeric; upstream freeze → proxy closes client within the ping window (non-1000 code); upstream `share stopped` still forwarded (existing test); no-cap path still JSON 404.
5. Full chain (extend `internal/daemon/attach_viewers_test.go` harness): CLI loop + GUI-like tap through the proxy. Kill the upstream mid-stream → automatic reconnect with **delta** and byte-exact output (scrollback diff: client transcript == PTY transcript). Simulate a **viewing-daemon restart** (fresh RemoteCapStore + new relay port; the reconnect seam re-runs connect) → reconnect succeeds. Share-off during the gap → loop stops with the share-stopped message (revoked cap is not reused).
6. `cmd_attach`/`internal/attach`: idle for > 3× ping interval with pongs flowing → no `[reconnecting]` (UAT gap 1 regression). Detach key works while reconnecting. Input typed while disconnected never reaches the PTY.
7. Pure logic (backoff schedule, healthy-reset, watchdog decisions): table tests, or `testing/synctest` for fake time. synctest can't be used with real sockets, because network I/O "is not durably blocking" [VERIFIED: GOROOT testing/synctest/synctest.go doc].

**Vitest (`cd frontend && pnpm test`), fake timers + the existing `vi.stubGlobal('WebSocket', …)` mock pattern (relayClient.test.ts:110-124):**
- relayClient: retry on 1006/1008/1013; stop on share stopped / session ended / user close; backoff sequence and cap; healthy reset; watchdog armed only after first pong; visibility/online → immediate retry or probe; dead-socket abandonment (old socket's late onclose ignored); `resume` appended with the counted offset; `onReset` on `replay:"full"` only when content exists; old-host fallback; `beforeReconnect` verdicts (`stop-shared`, `stop-ended`, `retry`, `ok` with a new URL/port).
- TerminalPanel: banner appears after 3 s, not for a 1 s blip; stdin disabled while reconnecting and re-enabled after; `term.reset` only on full; terminal banners unchanged. Rewrite `TerminalPanel.guestClose.test.tsx`.
- App/WebShareSessionView: the GUI hook calls `GetRelayPort` then `ConnectPeerSession` and maps error codes; the browser hook probes `/info` and maps 401/403/404.

**VM (Tart `portty-test`, vmpreview build, Safari in the VM to the 127.0.0.1:7444 share link), proposed M-items:**
- **M-84** Overnight two-Mac GUI + browser (VERF-01 as written; office Mac on `caffeinate -dims`). Deferred-capable.
- **M-85** Two-Mac short sleep/wake: close the home Mac lid for 10+ minutes with the GUI tab, CLI attach and browser open → on wake all three reconnect without action, scrollback intact, typing works into Claude Code. Deferred-capable accelerated alternative to M-84.
- **M-86** VM browser freeze: `kill -STOP <daemon pid>` for 60 s → "Reconnecting…" banner appears (watchdog), `kill -CONT` → reconnects with no duplicated history. Variant: `sudo pfctl` blackhole on lo0:7444 [ASSUMED feasible].
- **M-87** VM clock jump: `tart suspend portty-test` for several minutes, then `tart run` → the session continues with no reconnect storm and no duplicate output [ASSUMED tart suspend works for this guest].
- Update M-80/M-83 wording where they say "no reconnect". Add §4 rows for new test files (SHAR-07), and update the §2 counts.

## Security Domain

`security_enforcement` enabled, ASVS level 1.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes (indirect) | Unchanged: cap HMAC (`requireCapability`) + same-account WhoIs on peer routes. Reconnect re-runs `/peers/connect` (re-checks WhoIs) for GUI/CLI. The browser reuses its cap and is re-verified on every upgrade |
| V3 Session Management | yes | Caps have no TTL (existing). Reconnect must not extend access past revocation: share-off clears grants → a reconnecting socket fails `requireCapability` → the client stops (test it) |
| V4 Access Control | yes | Delta/full replay only after cap verification. `ReadOnly` still comes from claims. History replay goes only to cap holders |
| V5 Input Validation | yes | `resume`: `strconv.ParseUint(s,10,64)` with range check against [start,end], and anything else → full replay. Client meta parsing: `JSON.parse` in try/catch, `typeof` checks, `Number.isSafeInteger`. Close `reason` never rendered |
| V6 Cryptography | no | No new crypto |
| V7 Logging | yes | Proxy `redact` already strips the whole upstream query (cap + resume). Never log input bytes or history content. viewer-diag lines may add reconnect/heartbeat-close counts only |

### Known Threat Patterns

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Stale keystrokes replayed into the shell after a reconnect | Tampering | Input disabled while not open; never queued (locked decision) |
| Revoked viewer silently reconnecting | Elevation | Grants cleared before kick (existing order); reconnect re-verifies the cap; GUI/CLI re-fetch the cap through WhoIs-gated `/peers/connect` |
| Bigger history exposure to a newly joined cap holder (10,000 lines vs current screen after wrap) | Information disclosure | Intended by the decision; tailnet-only + cap-gated links; `clear` (zsh `clear` sends ED3) wipes history, so document that. Nothing goes to disk |
| Replayed terminal queries answered into the shared shell | Tampering | Rendered history contains no queries. Delta and raw paths keep `liveEmuQueryStripPattern` (extend per the pending WR-02 todo if in scope) |
| Ping flood from a guest | DoS | Pong is enqueued non-blocking; a flood only fills the guest's own queue → self-kick |
| Reconnect storm load on the host | DoS | Exponential backoff + jitter, healthy-reset rule, delta replays |
| Resume offset probing | Information disclosure | The offset only selects bytes inside the ring the cap holder can already read; out-of-range → full replay |

## Sources

### Primary (HIGH confidence)
- In-repo source read this session: `internal/relay/{hub.go,scrollback.go,protocol.go,close_reasons.go,server.go}`, `internal/webserver/server.go`, `internal/daemon/{remote_ws_proxy.go,peer_connect.go,peer_client.go,types.go,engine.go}`, `cmd_attach.go`, `internal/attach/attach.go`, `frontend/src/lib/{relayClient.ts,closeReasons.ts}`, `frontend/src/components/{TerminalPanel.tsx,WebShareSessionView.tsx,SessionEndedBanner.tsx}`, `frontend/src/App.tsx`, `TESTING.md`
- Module cache source: coder/websocket v1.8.15 (`conn.go`, `read.go`, `close.go`, `dial.go`), charmbracelet/x/vt (`scrollback.go`, `screen.go`, `emulator.go`, `handlers.go`, `callbacks.go`, `mode.go`), ultraviolet (`buffer.go`, `cell.go`)
- GOROOT: `net/dial.go` keepalive defaults; `runtime/sys_darwin_arm64.s` nanotime; `testing/synctest/synctest.go`
- `node_modules/@xterm/xterm` 6.0.0: `typings/xterm.d.ts` (`scrollOnEraseInDisplay`, `convertEol`), `lib/xterm.mjs` defaults
- Scratch measurements (this session): Cell size 112 B; 10k-line emulator scrollback 126.6 MiB vs rendered 1.10 MiB; stale retention 110.2 MiB vs 1.5 MiB; alt-screen 126.6 MiB; Render uses bare LF; mode callbacks fire

### Secondary (MEDIUM confidence)
- [Chrome 88 timer throttling](https://developer.chrome.com/blog/timer-throttling-in-chrome-88): intensive throttling after 5 minutes hidden, 1 wake-up per minute
- [golang.org/issue/24595](https://golang.org/issue/24595): mach_absolute_time vs mach_continuous_time (sleep)
- [golang.org/issue/35012](https://golang.org/issue/35012): timers vs macOS sleep reports

### Tertiary (LOW confidence)
- [Claude Code clearTerminal source mirror](https://tangled.org/nonbinary.computer/claude-code/blob/main/ink/clearTerminal.ts), [scrollback cleared on /compact](https://claudeissues.com/issue/7597-bug-compact-and-clear-commands-clear-terminal-scrollback-buffer-not-just-visible), [scrollback wiped during long sessions](https://claudeissues.com/issue/43643-claude-code-cli-erases-terminal-scrollback-history-during-long-sessions): ED3 usage, version-dependent
- [Solace: WebSocket disconnects when tab hidden](https://community.solace.com/t/client-websocket-disconnects-when-browser-is-minimized-or-not-in-focus-chrome/690), [XWiki-23928](https://jira.xwiki.org/browse/XWIKI-23928): app keepalives vs throttled timers

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH (no new dependencies; APIs read in the module cache)
- Architecture: HIGH for server/proxy/hub mechanics (code read + scratch-verified); MEDIUM for browser wake/visibility timing
- Pitfalls: HIGH (memory, LF/CUP and retention issues measured this session); MEDIUM for Claude Code escape behavior

**Research date:** 2026-11-17
**Valid until:** 2026-12-17 (stable stack; recheck if charmbracelet/x/vt is bumped, since its scrollback internals are relied upon)
