---
phase: "14"
slug: "mobile-web-app"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-11-22"
---

# Phase 14 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

The register comes from the `<threat_model>` blocks of 14-01..14-14-PLAN.md: T-14-01..T-14-43 plus T-14-SC, which is counted once, for 44 threats (40 mitigate, 4 accept). The audited code is 2153501a..cf2e1899, which includes the review fixes f35a0498..8f111eea (WR-01..03, IN-02). Config: enforcement on, ASVS 1, block on `high`. All 12 high threats were traced to the place where each control sits. Per-threat file:line evidence is in the audit return of 2026-11-22. 14-SECURITY-REVIEW.md is the narrative review, with the VM, simulator and real-iPhone evidence and an approved verdict.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Browser/phone → share server | Every session-data route is gated by `requireSameUser` (WhoIs, numeric user ID, fail closed) before the capability check | Share token, session data |
| Viewer input → PTY | The lone-Esc flush in `InputAbsorber.Filter`; OSC/CSI states still span frames | Keystrokes and terminal replies |
| Home Screen icon → `/app/` | The token stays in the launch URL (no `start_url`); the manifest is allowed only by `manifest-src 'self'` | Share token at rest on the phone |
| Test/VM fakes → release builds | `webservertest` and `vmwhois` are behind build tags, guarded by the go list -deps test and check-brand | Identity checks |

---

## Threat Register (summary)

All 44 threats are **closed**. Key controls:
- **Gated routes:** meta, peer cap, ports, probe, info, plugin-config (+stream), the link page, the WebSocket (Origin → WhoIs → cap) and all of `/app/` (server.go). Only `/`, `/dashboard` and `/assets/` stay open, and they carry no session data.
- **Fail closed (whois.go):** a status or WhoIs fault gives 503; a not-found, tagged or shared-in node, zero user or user mismatch gives 403; `next` runs only on 0. The refusal and retry pages are fixed embedded bytes.
- **Release exclusion:** release_guard_test.go checks the go list -deps graph for no tags, `wailsassets`, `vmwhois` and both together; a failing `go list` counts as an error. check-brand checks the vm-tag and fake-WhoIs imports. Release builds use only `-tags wailsassets`.
- **CSP:** the only change is `manifest-src 'self'` (csp_mw.go:173), pinned by TestCSPHeaders_ManifestDelta and approved by the user.
- **No `Access-Control-Allow-*`** header was added. Dependency manifests are unchanged (T-14-SC).
- **Procedural VM and simulator rows** (T-14-15, -28..-41): VMs never stopped, links kept only in 0600 temp files that are now deleted, throwaway simulator devices and CA deleted, secret greps 0.

| Severity | Count | Open |
|---|---|---|
| high | 12 | 0 |
| medium | 19 | 0 |
| low | 13 | 0 |

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-14-01 | T-14-02 | The source address is trusted as tailscaled reports it | plan (14-01) | 2026-11-22 |
| AR-14-02 | T-14-04 | The Home Screen icon stores the share URL with the token; it dies when sharing stops or a new key is made | user ("continue", approve) | 2026-11-22 |
| AR-14-03 | T-14-07 | No WhoIs verdict cache; 82.8 ms per cold load against a 300 ms rule | user ("keep uncached") | 2026-11-22 |
| AR-14-04 | T-14-43 | Probe gated: during a host tailscaled fault, peers drop the host's sidebar group for about 20–30 s | user ("Gate it") | 2026-11-22 |
| AR-14-05 | T-14-09 | A raw terminal behind `portty attach` that splits a reply exactly between ESC and `]`/`[` now leaks the rest as text (no newline) | plan (14-02) | 2026-11-22 |
| AR-14-06 | — | UI-A: another account was shown only through the vmwhois build; tagged and shared-in nodes are covered by tests only | user | 2026-11-22 |
| AR-14-07 | — | UI-C: simulator rows are the evidence for the key bar, keyboard inset, pinch and safe areas, backed by real-iPhone PHONE-1/2 | user | 2026-11-22 |
| AR-14-08 | — | S-02: the key bar sits above iOS's own form-assist bar (⌃⌄✓); web pages cannot hide it | user | 2026-11-22 |

**Open observations (non-blocking):**
- IN-05: the guards cover the `webservertest` import but not the exported `SetWhoIsClientForTest` setter. Both current non-test callers are behind build tags.
- IN-06: the fixture CA has no name constraints, and its admin listener has no authentication. It is fixture-only and short-lived.

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-11-22 | 44 | 44 | 0 | gsd-security-auditor |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
