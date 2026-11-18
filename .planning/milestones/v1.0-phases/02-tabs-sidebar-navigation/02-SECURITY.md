---
phase: "02"
slug: "tabs-sidebar-navigation"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-11-16"
---

# Phase 02 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Tailnet / web guest → webserver + relay | Capability-gated WebSocket read pumps (chat frames retired) | Terminal I/O (high: remote shell write) |
| Peer metadata → sidebar | Other Macs' session lists rendered as peer groups | Host and session names (untrusted text) |
| Frontend → Wails bindings | Settings, appearance, OpenTailscale, session RPCs | Enum values, session IDs (local user) |
| Local GUI → daemon Unix socket | Settings API incl. new `appearance` key | Settings values (validated server-side) |
| Go → AppKit | NSApp.appearance bridge via cgo/ObjC | Fixed enum 0/1/2 |
| Supply chain → bundle | npm/pnpm lockfile, go.mod | Removals only this phase |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-02-01 | Spoofing | Native menu events | low | accept | In-process callbacks only; web runtime stub no-op (AR-10) | closed |
| T-02-02 | Information Disclosure | Help URL | low | accept | Fixed constant helpURL, no data (AR-11) | closed |
| T-02-03 | Denial of Service | Drag regions | low | mitigate | style.css:2395-2411 no-drag on every control | closed |
| T-02-04 | Elevation of Privilege | ⌘W close-tab | medium | mitigate | App.tsx:527-530 → removeTabState, UI-only | closed |
| T-02-05 | Tampering | Retired frames 0x30-0x37 | medium | mitigate | No case/default in both read pumps; TestRetiredFrames_* both paths | closed |
| T-02-06 | Denial of Service | Read-pump JSON decode | low | mitigate | Decode arms removed; FuzzParseFrame clean | closed |
| T-02-07 | Information Disclosure | Chat CORS helpers | medium | mitigate | FilesCORS/withCORS deleted; loopbackOriginPatterns unchanged | closed |
| T-02-08 | Elevation of Privilege | Read-only input gate | high | mitigate | MsgInput arm byte-identical; RO tests assert 0 PTY writes | closed |
| T-02-09 | Information Disclosure | WhoIs / alias identity | low | mitigate | WhoIs, AliasStore, self frame removed | closed |
| T-02-10 | Repudiation | TESTING.md traceability | low | mitigate | check-traceability-paths.sh OK (92) | closed |
| T-02-11 | Tampering | Legacy theme localStorage | low | accept | Keys never read (AR-12) | closed |
| T-02-12 | Denial of Service | Palette swap | low | mitigate | Module-constant palettes; isActive-guarded effect | closed |
| T-02-13 | Information Disclosure | index.html / CSP | low | mitigate | No new script/font; csp_mw.go unchanged | closed |
| T-02-14 | Tampering | Lockfile (02-03) | medium | mitigate | Removals only; lockfileVersion 9.0 | closed |
| T-02-15 | Information Disclosure | Sidebar in web mode | high | mitigate | App.tsx:1074 gate; web-share-scope.spec.ts:35 | closed |
| T-02-16 | Spoofing | Peer attach | high | mitigate | handleOpenRemoteSession handshake only (App.tsx:939-1003) | closed |
| T-02-17 | Tampering | Peer names rendering | medium | mitigate | React text only; no dangerouslySetInnerHTML | closed |
| T-02-18 | Denial of Service | Session/peer polling | low | accept | 3 s local, 30 s peer skipped when TS off (AR-13) | closed |
| T-02-19 | Tampering | stayOnHub legacy key | low | mitigate | Dropped on save; migration test passes | closed |
| T-02-20 | Elevation of Privilege | Funnel consent (move) | high | mitigate | Markup-only diff; consent tests pass | closed |
| T-02-21 | Repudiation | TESTING.md traceability | low | mitigate | Script passes | closed |
| T-02-22 | Tampering | Frontend relay parser | low | mitigate | Retired bytes → unknown; relayClient.test.ts:160-169 | closed |
| T-02-23 | Information Disclosure | Web share chat/nav | high | mitigate | App.tsx:1074; web-share-scope.spec.ts:32,35 | closed |
| T-02-24 | Tampering | Lockfile (02-07) | medium | mitigate | Removals only; 9.0 | closed |
| T-02-25 | Information Disclosure | Markdown deps | low | mitigate | react-markdown/rehype/remark gone | closed |
| T-02-26 | Information Disclosure | End Session while shared | medium | mitigate | ToggleWebServing(false) before KillSession (App.tsx:649-657) | closed |
| T-02-27 | Denial of Service | Accidental kill | medium | mitigate | closeTab no RPC; confirm focuses Cancel | closed |
| T-02-28 | Tampering | Rename input | low | mitigate | Trim/empty guard; revert on failure | closed |
| T-02-29 | Elevation of Privilege | Share while TS off | low | mitigate | aria-disabled button has no handler | closed |
| T-02-30 | Elevation of Privilege | OpenTailscale exec | medium | mitigate | Fixed argv /usr/bin/open -a Tailscale; fixed URL | closed |
| T-02-31 | Tampering | Tailscale install path | medium | mitigate | Launch/URL only; no installer | closed |
| T-02-32 | Elevation of Privilege | Funnel switch restyle | high | mitigate | Native disabled + funnelDisabled check | closed |
| T-02-33 | Spoofing | Sheet scrim/Esc | low | mitigate | No scrim onClick; Esc → cancel | closed |
| T-02-34 | Elevation of Privilege | Start Server blocked | high | mitigate | startBlocked early return; backend TS check unchanged | closed |
| T-02-35 | Repudiation | Share-warning opt-out | low | mitigate | Confirm with Stop/Keep Asking | closed |
| T-02-36 | Denial of Service | Key regeneration | medium | mitigate | Destructive confirm, Cancel focused | closed |
| T-02-37 | Tampering | join/dashboard CSS | low | mitigate | No @import/url/remote fonts; CSP unchanged | closed |
| T-02-38 | Repudiation | Token gate | low | mitigate | style.tokens.test.ts:233-275; failing-first in 02-12-SUMMARY | closed |
| T-02-39 | Repudiation | TESTING.md Section 5 | low | mitigate | Region grep 0 over 399 lines | closed |
| T-02-40 | Repudiation | Removed bindings | low | mitigate | stayOnHub/GetSessionStyledTailLines absent | closed |
| T-02-41 | Repudiation | Banner invariant | low | mitigate | App.tsx:1118-1126; App.test.tsx asserts | closed |
| T-02-42 | Tampering | appearance setting | low | mitigate | 400 on invalid; normalizeAppearance; pre-RPC switch | closed |
| T-02-43 | Denial of Service | NSApp.appearance thread | low | mitigate | dispatch_async main queue; fixed constants | closed |
| T-02-44 | Information Disclosure | Override in web mode | low | mitigate | useColorScheme('system') in web; GetAppearance desktop-only | closed |
| T-02-SC | Tampering | Supply chain | high | accept | No packages added; go.mod unchanged (AR-14) | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

Notes from the audit (non-blocking):
- T-02-01: the App mounts `EventsOn` in both modes. It is inert in web mode only because the aliased runtime stub needs `window.runtime`.
- T-02-34: the CT acknowledgment is enforced in the UI only. The backend `StartWebServer` enforces the Tailscale-connected check alone, as it already did at the phase base. Only the local user can reach it.
- T-02-43: the bridge always uses `dispatch_async`, not the planned sync-or-direct. This avoids a deadlock during Wails startup.
- T-02-40: the stale `GetSessionQRCode` declaration predates this phase (Phase 1 IN-09). It has no Go method and adds no capability.
- T-02-26: if `ToggleWebServing(false)` fails, `KillSession` still runs, and both write pumps close their viewers on `hub.Done()`.

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-10 | T-02-01 | Menu events come only from the in-process native menu. The web share page has no Wails runtime, so `EventsOn` there is a stub no-op. | plan 02-01 | 2026-11-16 |
| AR-11 | T-02-02 | Help opens a fixed constant URL carrying no session, host or user data. | plan 02-01 | 2026-11-16 |
| AR-12 | T-02-11 | Legacy theme localStorage keys are never read, so a tampered value has no effect. | plan 02-03 | 2026-11-16 |
| AR-13 | T-02-18 | 3 s local-socket ListSessions poll. The 30 s peer poll is skipped while Tailscale is off. Both stop on unmount. | plan 02-04 | 2026-11-16 |
| AR-14 | T-02-SC | Phase 2 adds no packages: removals only and a frozen lockfile. go.mod is unchanged. | all plans | 2026-11-16 |

*Accepted risks do not resurface in future audit runs. Phase 1 AR-07 (LAN fallback when Tailscale is off) is still owned by Phase 4 and is unchanged here.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-11-16 | 45 | 45 | 0 | gsd-security-auditor (HEAD 75fedd97) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
