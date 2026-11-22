# Phase 13: Remote Localhost URLs - Discussion Log

> Audit trail only. Decisions live in 13-CONTEXT.md.

**Date:** 2026-11-22
**Mode:** smart discuss (autonomous)

## Area 1: Proxy wire contract (hard to reverse)

| Question | Options | Choice |
|---|---|---|
| How do requests reach the remote loopback? | ✅ Per-request HTTP reverse proxy, both hops; raw TCP tunnel; multiplex over the terminal stream | Accept all (recommended) |
| Which local port does the browser use? | ✅ Same port number; random port; same port, refuse if taken | Accept all (recommended) |

## Area 2: Security contract (hard to reverse)

| Question | Options | Choice |
|---|---|---|
| How does the host know a port was approved? | ✅ Port-scoped signed token tied to the share; host allowlist; session token only | Accept all |
| Anti-scan | ✅ Printed-in-session check; consent + rate limit | Accept all |
| Who may use it? | ✅ Same Tailscale user + write; any link holder | Accept all |
| Protecting this Mac's listener | ✅ 127.0.0.1 + Host header check; one-time cookie; no check | Accept all |

## Area 3: Consent (preference)

| Question | Options | Choice |
|---|---|---|
| Consent UI | Native alert; in-window sheet; link popover | You decide → in-window sheet |
| Copy | "Allow access…"; "Open localhost…"; "Open port…" | You decide → "Allow access…" |
| Lifetime | Until tabs close; on disk; until share stops | You decide → until share stops (memory only) |
| Revoke | Tab indicator; Settings list; implicit only | You decide → implicit only |

## Area 4: Scope and failures (preference)

| Question | Options | Choice |
|---|---|---|
| Scope | Plus [::1]/0.0.0.0; exactly URL-01; plus browser view | You decide → URL-01 plus [::1], 0.0.0.0, [::] |
| Failures | Fall back; notice + error page; alert + error page | You decide → notice + error page |

## Decided autonomously

[auto] [Proxy] — Q: "Where does the viewing-side listener live?" → Selected: "The viewing Mac's daemon" (reversible technical — decided autonomously: internal to one Mac)
[auto] [Proxy] — Q: "WebSockets, streaming and stopping the share" → Selected: "Track and close on revoke; dial timeout only" (reversible technical — decided autonomously: internal plumbing)
[auto] [Proxy] — Q: "Version gating and upgrades" → Selected: "APIVersion 5, typed errors at click time, forwards end on handoff" (reversible technical — decided autonomously: additive gate)
[auto] [URL] — Q: "Rewriting Host, Origin and bodies" → Selected: "Host/Origin set to localhost:PORT, no body or cookie rewriting" (reversible technical — decided autonomously: host-side header logic)
