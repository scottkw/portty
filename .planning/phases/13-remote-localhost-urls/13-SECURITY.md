---
phase: "13"
slug: "remote-localhost-urls"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-11-22"
---

# Phase 13 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

The register comes from the `<threat_model>` blocks of 13-01..13-07-PLAN.md: T-13-01..T-13-41 plus T-13-SC, counted once (42 threats). The audited code is 9574fd49..d09e7c47, including the review fixes 32a2b551..82811b55 (CR-01, WR-01..04). The working tree held no product changes. Config: enforcement on, ASVS 1, block on `high`. The high threats were traced to the boundary where each control sits. 13-SECURITY-REVIEW.md is the narrative review, with the VM evidence and an "approved" verdict. Full per-threat evidence (file:line) is in the audit return of 2026-11-22.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Browser → viewing forward (127.0.0.1/[::1]:P) | Any local process or web page can reach the loopback listener | Host allowlist, Sec-Fetch-Site and Origin checks; no token in browser URLs |
| Viewing daemon → host share server (tailnet, TLS) | Grant and proxy routes behind requireSameUser + write capability + port token | Session capability and port token in headers; target in X-Portty-Target |
| Host share server → host loopback dev server | The dial is limited to ::1 then 127.0.0.1, port re-checked against the deny set | Arbitrary dev-server content, outside the CSP wrapper |
| Session output → printed check | A port is grantable only if the session printed its URL | Untrusted terminal text with URL token boundaries |

---

## Threat Register

Key controls:
- **Check order.** Every refusal goes through one identical `refusePort` (portproxy.go:47).
  - Grant: content type, body, write capability, deny set, printed check, then mint. It never dials.
  - Proxy: port, write capability, `VerifyPort` (session, port and GrantID), deny set, target bytes, revoke and grant re-check, and only then `ReverseProxy`.
- **Token separation.** Port tokens use an HKDF-derived `PortKey`, so each token kind fails the other's HMAC.
- **Revoke.** On the host, `revokeSignal` and `stopCh` cancel proxied requests. On the viewing Mac, `RevokeSession` runs on a terminal stream close or a capability change, and `CloseAll` runs on Stop and handoff.

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-13-01 | Info disclosure | printed check | high | mitigate | hasURLPort boundaries | closed |
| T-13-02 | DoS | printed check | medium | mitigate | busy CAS | closed |
| T-13-03 | EoP | token kinds | high | mitigate | HKDF PortKey, separate headers | closed |
| T-13-04 | Spoofing | port token | high | mitigate | hmac.Equal, kind/v after sig | closed |
| T-13-05 | Tampering | pathclick refactor | medium | mitigate | path tests unchanged | closed |
| T-13-06 | Spoofing | DNS rebinding | high | mitigate | forward Host allowlist | closed |
| T-13-07 | Tampering | cross-site to forward | medium | mitigate | Sec-Fetch-Site + Origin (WR-02) | closed |
| T-13-08 | Tampering | port shadowing | high | mitigate | probe then tcp4/tcp6 bind | closed |
| T-13-09 | Info disclosure | listener exposure | high | mitigate | 127.0.0.1 and [::1] only | closed |
| T-13-10 | Info disclosure | logs/disk | high | mitigate | id, port, outcome only; nothing on disk | closed |
| T-13-11 | Tampering | X-Portty header injection | high | mitigate | inbound/response X-Portty stripped | closed |
| T-13-12 | EoP | local API exposure | high | mitigate | unix-socket mux only | closed |
| T-13-13 | EoP | stale approval | high | mitigate | capability fingerprint, revoke hooks | closed |
| T-13-14 | Tampering | error pages | medium | mitigate | escaped, default-src 'none' | closed |
| T-13-15 | Spoofing | other tailnet user | high | mitigate | requireSameUser, fail closed | closed |
| T-13-16 | Info disclosure | port scan | high | mitigate | printed check before mint, one refusal, no dial | closed |
| T-13-17 | EoP | relay/share port | high | mitigate | deny set incl. takeover (CR-01) | closed |
| T-13-18 | Tampering | target smuggling | medium | mitigate | target validation + WR-01 byte filter | closed |
| T-13-19 | Spoofing | Host header | medium | mitigate | Host set to localhost:P | closed |
| T-13-20 | Tampering | Origin rewrite | high | mitigate | rewrite only exact http://127.0.0.1:P | closed |
| T-13-21 | EoP | non-loopback dial | high | mitigate | dialer ignores host, ::1 then 127.0.0.1 | closed |
| T-13-22 | Tampering | cap in URL | medium | mitigate | header-only credentials and target | closed |
| T-13-23 | EoP | capability_mw split | high | mitigate | same sequence; tests unchanged | closed |
| T-13-24 | Spoofing | old daemon | medium | mitigate | API 5 gate | closed |
| T-13-25 | Info disclosure | binding result | high | mitigate | status/url/host/port/errorCode only | closed |
| T-13-26 | DoS | binding errors | low | mitigate | codes only, rejection caught | closed |
| T-13-27 | Spoofing | URL classifier | medium | mitigate | shared Go/TS fixture | closed |
| T-13-28 | Tampering | opened URL | high | mitigate | open only the binding's URL | closed |
| T-13-29 | EoP | consent bypass | high | mitigate | one sheet, Don't Allow focused, Esc cancels | closed |
| T-13-30 | Tampering | local link change | medium | mitigate | remote && !wsURL gate | closed |
| T-13-31 | Info disclosure | notices | low | mitigate | fixed copy, text nodes | closed |
| T-13-32 | EoP | live streams after revoke | high | mitigate | revokeSignal/stopCh/RevokeSession | closed |
| T-13-33 | EoP | key regeneration | high | mitigate | current key + remint clears grants | closed |
| T-13-34 | DoS | in-flight streams | low | accept | AR-13-01 | closed |
| T-13-35 | Repudiation | review doc | medium | mitigate | 13-SECURITY-REVIEW.md complete | closed |
| T-13-36 | Tampering | host app during VM run | high | mitigate | procedural | closed |
| T-13-37 | Info disclosure | VM docs | high | mitigate | secret greps 0 | closed |
| T-13-38 | DoS | ephemeral VMs | high | mitigate | procedural; never stopped | closed |
| T-13-39 | Tampering | VM state | medium | mitigate | procedural; cmp clean | closed |
| T-13-40 | Tampering | build worktrees | medium | mitigate | none left | closed |
| T-13-41 | EoP | helper process | low | mitigate | procedural; none left | closed |
| T-13-SC | Tampering | supply chain | high | mitigate | go.mod/go.sum/package.json/lockfile unchanged | closed |

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-13-01 | T-13-34 | No in-flight cap on proxied streams; the caller already controls the shell | plan (13-06) | 2026-11-22 |
| AR-13-02 | — | No hard-deny list for sensitive ports (9222, 2375, 5432); controls are printed check, same-user, write, consent | plan (13-CONTEXT); surfaced to user, not overridden | 2026-11-22 |
| AR-13-03 | — | localhost cookies and storage are shared with this Mac's local apps | plan | 2026-11-22 |
| AR-13-04 | — | A second port in a multi-port app reaches this Mac's own localhost unless it is printed and approved too | plan | 2026-11-22 |
| AR-13-05 | — | https://localhost, portless, upper-case and *.localhost forms keep today's behaviour | plan (13-CONTEXT) | 2026-11-22 |
| AR-13-06 | — | Any local process can use an approved forward while it is open | plan | 2026-11-22 |
| AR-13-07 | — | The host dials [::1] then 127.0.0.1 for every accepted form | plan (13-CONTEXT) | 2026-11-22 |
| AR-13-08 | — | Per-request WhoIs can turn a tailscaled hiccup into a 503 on asset-heavy pages | plan | 2026-11-22 |
| AR-13-09 | — | Approvals are memory-only, with no indicator or list | plan (13-CONTEXT, Claude's discretion) | 2026-11-22 |
| AR-13-10 | — | No-stream window: the forward stays listening until its first request; nothing is proxied (fresh GrantID) | plan (13-02) | 2026-11-22 |
| AR-13-11 | — | A ::1-only page started more slowly once (L-05), not investigated | VM run | 2026-11-22 |
| AR-13-12 | — | UI-1, another tailnet user: automated tests only | user ("Accept automated-only") | 2026-11-22 |
| AR-13-13 | — | UI-2, real Vite/webpack HMR: manual check M-140 | user ("Check on my Macs") | 2026-11-22 |
| AR-13-14 | — | G-1 internal/pty TestKillClean_NoOrphans flake under load; package unchanged | user ("approve") | 2026-11-22 |

**Open observations, non-blocking (medium):**
- T-13-07: a cross-site no-cors subresource GET (`<img>`, `<script>`) sends no Origin, so it is stopped only by Sec-Fetch-Site. Safari's Sec-Fetch-Site behaviour was not shown in the VM evidence.
- WR-03: the re-grant runs once, immediately. A sustained WhoIs miss still closes the forward. In a narrow race (host stops sharing, re-shares and the peer reconnects while a grant is in flight), the forward continues under the new share without asking again.

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-11-22 | 42 | 42 | 0 | gsd-security-auditor |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
