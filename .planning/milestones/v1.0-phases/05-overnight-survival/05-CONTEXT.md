# Phase 5: Overnight Survival - Context

**Gathered:** 2026-11-17
**Status:** Ready for planning

<domain>
## Phase Boundary

A remote attach (peer GUI tab, `portty attach` CLI, browser web-share view) survives any idle gap: it stays connected, or detects a dropped/half-open connection and reconnects on its own without a manual reattach. Detach + reattach (or reconnect) restores real scrollback history, not just the current screen. Requirements: SHAR-07, VERF-01.

Today nothing reconnects: GUI (`frontend/src/lib/relayClient.ts`), CLI (`cmd_attach.go` `runAttach`) and browser (`WebShareSessionView.tsx` → same RelayClient) each open one WebSocket and stop on close. Reattach replays a 256 KiB raw ring; once it wraps, only the current screen comes back.

Not in this phase: office-Mac keep-awake (CONV-01, v2); disk-backed scrollback; the VERF-01 gate rewrite (deferred, see below).

</domain>

<decisions>
## Implementation Decisions

### Scrollback on reattach
- History stays in memory only: raise the live emulator's scrollback (`liveEmulatorScrollbackLines`, `internal/relay/hub.go:107`, currently 50) to ~10,000 lines, and when the raw ring has wrapped, `ReconnectPreamble` sends emulator scrollback + current screen instead of the screen alone. The GUI keeps its xterm buffer across a reconnect (no clear + duplicate). Nothing is written to disk; a daemon restart still loses history (as today). — **Reversibility:** costly — changes the reattach replay contract every viewer (GUI, CLI, browser) depends on, and per-session memory budget; disk-backed was rejected for privacy (terminal contents incl. secrets on disk).

- Mechanism amended after research (2026-11-17, same outcome: 10k lines, memory only): raising emulator scrollback measured 126.6 MiB/session, so the hub owns a ~10k-line history of ANSI-rendered lines captured as they scroll off (~1.1 MiB), surviving emulator rebuilds on resize. GUI keeps its buffer via a resume offset (`?resume=<n>`): gap-only replay when the ring still covers it, else a full replay the client resets for. See 05-RESEARCH.md.
- Research open questions, decided by Claude: on resize, visible main-screen rows go into history (accept a few duplicate prompt lines); no NSWorkspace wake hook (backoff + liveness suffice); alt-screen scrollback growth in the vt library is pre-existing — add a heap soak test only if cheap, otherwise defer as a todo.
- Amended 2026-11-17 (gap closure, user requirement): 'no NSWorkspace wake hook' is superseded. The user requires recovery from sleep/wake and network or access-point switches without waiting out the watchdog. A wake/network-change trigger is added: internal/wakewatch (05-10, CLI) and a Wails event to the GUI and browser views (05-13). Mechanism: a pure-Go wall-clock tick-gap and interface-address detector rather than NSWorkspace, so the CLI gets it too.

### Verification gate (VERF-01)
- Deferred by the user ("Let's defer for now"). Do NOT rewrite VERF-01 or ROADMAP success criteria in this phase. Build the reconnect/scrollback work with automated tests (forced drop, half-open/idle liveness, daemon restart, scrollback diff), and leave the human gate decision (real overnight vs accelerated + two-Mac sleep/wake) to verification time. Human two-Mac items go into the UAT/manual checklist as deferred-capable items.

### Decided autonomously
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

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `frontend/src/lib/relayClient.ts:115` RelayClient (already sends MSG_PING 0x12 every 30 s, no liveness check) — add reconnect loop here; shared by GUI and browser.
- `frontend/src/lib/closeReasons.ts` — `CLOSE_REASON_SHARE_STOPPED`; extend for session-ended vs drop.
- `internal/relay/hub.go:626` `ReconnectPreamble`, `:567` `RenderSnapshot`, `internal/relay/scrollback.go` (256 KiB ring, 1 MiB ViewerReadLimit).
- `internal/daemon/remote_ws_proxy.go` (dumb pipe, cap from in-memory `RemoteCapStore`), `internal/daemon/peer_connect.go:199`.
- `internal/statusbar` `SetConnectionState` for CLI.
- `internal/testutil.WaitFor`.

### Established Patterns
- `TerminalPanel.tsx:227-232` and `SessionEndedBanner` explicitly say "never wired to auto-reconnect (RESEARCH anti-pattern)" — Chesterton's fence: that note referred to the old status model; this phase deliberately changes it, update the comments.
- `TerminalPanel.tsx:381-390` onClose disables stdin and treats every close as session ended.
- Slow viewers kicked with "too slow" at 256 queued msgs (`relay/server.go:85-91`) — watch for resume-after-sleep backlog.
- Tokens have no TTL (`internal/capability`); shares and remote caps are memory-only.

### Integration Points
- GUI: `TerminalPanel.tsx:373` creates RelayClient for remote sessions via loopback `ws://127.0.0.1:<port>/api/relay/remote/<sid>/ws`.
- Browser: `WebShareSessionView.tsx:90` `wss://…/sessions/<id>/ws?cap=` — cap reusable on reconnect.
- CLI: `cmd_attach.go:230` runAttach / `:236` Dial / `:291-321` watcher; `internal/attach/attach.go`.
- Servers: `internal/relay/server.go:169`, `internal/webserver/server.go:906` (MsgPing no-op).

</code_context>

<specifics>
## Specific Ideas

- STATE.md Phase 5 notes: the user finds a real overnight wait excessive; accelerated tests (forced drop, sleep/wake in VM, daemon restart, scrollback diff) are wanted as automated coverage regardless of the deferred gate decision.
- Fix the known CLI `[reconnecting]`-on-idle false positive (UAT gap 1, 04-07).

</specifics>

<deferred>
## Deferred Ideas

- VERF-01 gate rewrite (real overnight vs accelerated + two-Mac sleep/wake) — user deferred; decide at Phase 5 verification.
- Disk-backed scrollback surviving daemon restart — rejected for now (privacy).
- Keep-awake toggle (CONV-01, v2).

</deferred>
