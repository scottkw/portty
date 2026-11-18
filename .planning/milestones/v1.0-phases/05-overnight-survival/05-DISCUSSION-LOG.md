# Phase 5: Overnight Survival - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-11-17
**Phase:** 05-overnight-survival
**Areas discussed:** Scrollback, Verification gate, Reconnect UI, Give-up policy

---

## Scrollback

| Option | Description | Selected |
|--------|-------------|----------|
| Memory only, ~10k-line emulator scrollback (Recommended) | Send scrollback + screen on reattach; GUI keeps xterm buffer; nothing on disk | ✓ |
| Disk-backed scrollback | Survives daemon restart; terminal contents on disk | |
| Just a bigger raw ring | Simplest; still loses history once wrapped | |

**User's choice:** Accept (Recommended)

---

## Verification gate

| Option | Description | Selected |
|--------|-------------|----------|
| Keep real overnight | VERF-01 as written | |
| Accelerated + sleep/wake | Automated drops/restart/scrollback + short two-Mac sleep/wake gate | |
| You decide | | |

**User's choice:** "Let's defer for now." — VERF-01 wording unchanged; decide at verification.

---

## Reconnect UI

| Option | Description | Selected |
|--------|-------------|----------|
| Status dot only | Quiet | |
| Inline banner | Immediate banner | |
| Banner after ~3 s | Blips silent | |
| You decide | | ✓ |

---

## Give-up policy

| Option | Description | Selected |
|--------|-------------|----------|
| Give up after ~10 min | Then Reconnect button | |
| Retry forever | Until share stopped / session ended | |
| You decide | | ✓ |

---

## Claude's Discretion

- Reconnect UI (default: banner after ~3 s)
- Give-up policy (default: retry forever, 30 s cap)

## Decided autonomously

[auto] [Liveness] — Q: "How are dead connections detected?" → Selected: "Server WebSocket pings + read deadline; clients treat pongs/frames as liveness" (reversible technical — decided autonomously: internal, no persisted state)
[auto] [Liveness] — Q: "CLI [reconnecting] after 5 s idle" → Selected: "Watcher keys on ping/pong liveness" (reversible technical — decided autonomously: one watcher)
[auto] [Reconnect] — Q: "Retry timing" → Selected: "Exponential backoff 1 s to 30 s; online/visibilitychange immediate retry" (reversible technical — decided autonomously: constants in one client)
[auto] [Reconnect] — Q: "Network drop vs real end" → Selected: "Close codes: drop retries, share-stopped/session-ended stop" (reversible technical — decided autonomously: builds on closeReasons.ts)
[auto] [Reconnect] — Q: "Daemon restarted, cap store empty" → Selected: "Re-run /peers/connect before redial" (reversible technical — decided autonomously: existing endpoint)
[auto] [Reconnect] — Q: "Typing while disconnected" → Selected: "Input disabled, never queued" (reversible technical — decided autonomously: current behavior)
[auto] [Scope] — Q: "Office Mac sleeping" → Selected: "Out of scope; document caffeinate" (reversible technical — decided autonomously: docs only)

## Deferred Ideas

- VERF-01 gate rewrite — decide at verification
- Disk-backed scrollback
- Keep-awake toggle (CONV-01, v2)
