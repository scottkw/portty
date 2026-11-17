---
phase: "04"
slug: "one-click-share"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-11-17"
---

# Phase 04 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Tailnet peer → share server (7444) | Discovery, peer capability, web link, WebSocket | Same-user WhoIs (numeric IDs, fail closed); full-control capability token |
| Viewing daemon → peer share server | `/peers/connect`, remote WS proxy | TLS verified against the peer's MagicDNS name, dialled at its Tailscale IP |
| Browser → /app/ and /sessions/{id} | Link open, info pre-check | Capability in URL; revoked → static "Link No Longer Works" |
| Local GUI/CLI → daemon socket | Share on/off, connect | Local user only |

## Threat Register (summary)

28 threats (T-04-01..T-04-27, T-04-SC): 24 mitigate (closed, file:line evidence in the 2026-11-17 audit), 3 accept (logged below), 1 originally transfer.

| Threat ID | Category | Severity | Disposition | Status |
|-----------|----------|----------|-------------|--------|
| T-04-02, -05, -06, -10, -11, -12, -SC | EoP / Spoofing / Info / Tampering | high | mitigate | closed |
| T-04-03, -04, -08, -13, -16, -17, -24, -25, -27 | various | medium | mitigate | closed |
| T-04-07, -14, -15, -18, -22, -23, -26 | various | low | mitigate | closed |
| T-04-20 | DoS / Spoofing (peer TLS) | medium | mitigate (was transfer) | closed — CR-02 removed InsecureSkipVerify; peer client dials the Tailscale IP with ServerName = MagicDNS FQDN |
| T-04-01 | Info Disclosure (stale public serve entry in tailscaled) | medium | mitigate | open — below high threshold (non-blocking); code half done, closes when the user runs M-77 (`tailscale serve status` on both Macs) |
| T-04-09, -19, -21 | Info / EoP | low–medium | accept | closed (AR-01..03) |

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-01 | T-04-09 | Share records live only in memory; the only file written is `ct_disclosed` ("1"); the link is printed only to the owner's terminal | orchestrator (plan disposition) | 2026-11-17 |
| AR-02 | T-04-19 | The relay listens on 127.0.0.1 only; the remote proxy accepts loopback origins only; the peer cap stays inside the daemon | orchestrator (plan disposition) | 2026-11-17 |
| AR-03 | T-04-21 | The share link lives in SharePopover component state; no localStorage/sessionStorage | orchestrator (plan disposition) | 2026-11-17 |

## Notes

- T-04-10 relies on both Macs reporting the same numeric user ID; the user confirmed both are on one Tailscale account (2026-11-16). M-80 verifies live.
- WR-03's fix has no test (no seam lands in the window).

## Security Audit 2026-11-17

| Metric | Count |
|--------|-------|
| Threats found | 28 |
| Closed | 27 |
| Open | 1 (non-blocking, medium) |

Auditor verdict: no blocking threats at ASVS L1 (block_on: high). Ran go build/vet, targeted `go test -race` (webserver, daemon, relay), targeted vitest, check-brand and check-share-removal.
