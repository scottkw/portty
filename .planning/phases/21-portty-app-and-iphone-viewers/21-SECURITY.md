---
phase: "21"
slug: "portty-app-and-iphone-viewers"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-11-27"
---

# Phase 21 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> Register source: the `<threat_model>` blocks of 21-01..21-13-PLAN.md (authored at plan time). Verified by gsd-security-auditor at ASVS L1, block_on high, against HEAD after the code review fixes (WR-01..WR-05).

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| peer Mac to share server cap route | POST /api/peer/desktop/cap, same-user only (whois numeric user ID) | desktop token (high) |
| home daemon to office share server | verified TLS to the matched peer's MagicDNS name over its Tailscale IP | desktop token, input, video |
| local processes to relay loopback port | any local process can reach 127.0.0.1:<relayPort>; cross-site browser origins refused | desktop WebSocket |
| daemon socket (0600) to GUI | the link route returns a URL with the token to the same user only | desktop link (high) |
| app web view to Go bridge | ReadHomeClipboard, ConnectPeerDesktop bindings; no token crosses into JS | clipboard text, host names |
| test rig to VMs | synthetic input and evidence capture run only inside Tart guests over ssh; evidence scrubbed | evidence files |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-21-01 | Information disclosure | POST /api/peer/desktop/cap | high | mitigate | requireSameUser (server.go:581, whois.go:55-112), JSON-only, no-store, no logging, no CORS | closed |
| T-21-02 | Elevation of privilege | desktop token on terminal routes | high | mitigate | HKDF DesktopKey (desktoptoken.go:48-52); desktop_peer_test.go:205-226 | closed |
| T-21-03 | Spoofing | forged X-Portty-Desktop header | low | accept | see Accepted Risks AR-21-01 (corrected rationale) | closed |
| T-21-04 | Information disclosure | discovery header | medium | mitigate | meta route behind requireSameUser (server.go:570, :840-844); sessions.go:115-132 | closed |
| T-21-SC | Tampering | dependency installs | high | mitigate | no phase-21 commit touches go.mod/go.sum/package.json/pnpm-lock.yaml | closed |
| T-21-05 | Tampering | probe code reaching shipped tree | high | mitigate | apply.sh:20-24 scratch-only guards; p21-rig.sh:91-110 git archive outside repo | closed |
| T-21-06 | Tampering | synthetic input on host | high | mitigate | p21-rig.sh require_vm2 + ssh-only helpers | closed |
| T-21-07 | Information disclosure | evidence files | low | mitigate | p21-rig.sh scrub; evidence grep clean | closed |
| T-21-08 | Tampering | pinch misread as drag | medium | mitigate | touchTrackpad.ts:166-171; DesktopPhoneLayer.tsx:149-166 | closed |
| T-21-09 | Denial of service | Safari zoom/scroll fight | low | mitigate | touchTrackpad.ts:340-344 | closed |
| T-21-10 | Tampering | host names in row/tab | low | mitigate | JSX text only, no raw-HTML sinks | closed |
| T-21-11 | Spoofing | row for unreachable peer | low | mitigate | Sidebar.tsx:208 | closed |
| T-21-12 | Information disclosure | token in web view URL | high | mitigate | peerDesktop.ts:21-25, :248-251, :318; peerDesktop.test.ts:166 | closed |
| T-21-13 | Denial of service | hidden window keeps office display lit | medium | mitigate | peerDesktop.ts:120-161; DesktopTab.tsx:242-253 | closed |
| T-21-14 | Denial of service | Local Network denial stuck tab | medium | mitigate | peerDesktop.ts:30, :72-78, :330-336; DesktopTab.tsx:273-280 | closed |
| T-21-15 | Elevation of privilege | local process opens desktop proxy | medium | accept | see Accepted Risks AR-21-02 | closed |
| T-21-16 | Information disclosure | token in logs/close reasons/bodies | high | mitigate | remote_desktop_ws_proxy.go fixed close reasons and redaction; peer_desktop.go token-free responses | closed |
| T-21-17 | Information disclosure | token on disk | high | mitigate | desktop_caps.go in-memory map; desktop_caps_test.go source guard | closed |
| T-21-18 | Spoofing | proxy dials peer-chosen URL | high | mitigate | matchPeer against tailscaled list; TLS ServerName = FQDN | closed |
| T-21-19 | Elevation of privilege | link route reachable from browser | medium | mitigate | unix-socket mux only (api.go:271); relay mux has only WS routes | closed |
| T-21-20 | Denial of service | refresh loop | low | mitigate | one refresh then 4401 (remote_desktop_ws_proxy.go:135-145) | closed |
| T-21-21 | Information disclosure | token via binding result | high | mitigate | app_peer_desktop.go returns only a code; link goes straight to openURL | closed |
| T-21-22 | Tampering | crafted host string | low | mitigate | lower-case, trim, PathEscape; matchPeer | closed |
| T-21-23 | Elevation of privilege | new permissions | medium | mitigate | only NSLocalNetworkUsageDescription added to Info*.plist; no entitlement change | closed |
| T-21-24 | Repudiation | browser link opened without action | low | mitigate | single onClick call site (DesktopTab.tsx:278) | closed |
| T-21-25 | Information disclosure | token in tab state/DOM | high | mitigate | desktop Tab carries no token/baseURL (App.tsx:1045) | closed |
| T-21-26 | Denial of service | hidden window keeps display lit | medium | mitigate | hidden pause + window occlusion event (DesktopTab.tsx:226-258, window_occlusion_darwin.*) | closed |
| T-21-27 | Tampering | markup via state words/hosts | low | mitigate | fixed card copy keyed by enum | closed |
| T-21-28 | Spoofing | Refused shown as browser page | low | mitigate | embedded other_account shows token_rejected card | closed |
| T-21-29 | Denial of service | desktop tab rendered as terminal | medium | mitigate | terminal loop skips 'desktop' (App.tsx:1464) | closed |
| T-21-30 | Tampering (availability) | held key on office Mac | high | mitigate | controlMode.ts releaseAll first on every exit; layout cleanup on tab close; H2 evidence | closed |
| T-21-31 | Denial of service | user trapped without shortcuts | high | mitigate | release chord never forwarded; every exit restores shortcuts; KEYROUTE: page (no native switch) | closed |
| T-21-32 | Information disclosure | concealed clipboard item sent | high | mitigate | types-first Go bridge; WR-02 and WR-03 route every app paste through it | closed |
| T-21-33 | Information disclosure | background clipboard reads | medium | mitigate | reads only on captured Cmd+V or the bar's Paste to Mac click (both user actions; the second since WR-03) | closed |
| T-21-34 | Elevation of privilege | system-wide keyboard hook | medium | mitigate | no event tap in phase 21; keys from the focused hidden input only | closed |
| T-21-35 | Tampering (availability) | held button after phone sleeps | high | mitigate | desktopPhoneInput.ts heartbeat + release on blur/pagehide/hidden | closed |
| T-21-36 | Tampering | input from pinch/page scroll | medium | mitigate | stage-only listeners, touch-action none | closed |
| T-21-37 | Tampering | markup from surface names | low | mitigate | typed/enum-checked surface messages, JSX text | closed |
| T-21-38 | Information disclosure | typed text in logs | low | mitigate | no console calls in phone input path | closed |
| T-21-39 | Information disclosure | links/tokens/names in evidence | high | mitigate | scrub_ax, path-only safari2-front; evidence grep clean | closed |
| T-21-40 | Tampering | input/grants on host | high | mitigate | vm_ssh checks; host only compiles Swift | closed |
| T-21-41 | Tampering | stats patch shipping | medium | mitigate | --stats applies to scratch copies only | closed |
| T-21-42 | Information disclosure | hostnames/links in docs | medium | mitigate | check-brand.sh clean | closed |
| T-21-43 | Repudiation | failing device row dropped | low | mitigate | all 12 device rows recorded DEFERRED; DEVICE_ROWS line | closed |
| T-21-44 | Tampering | stale traceability path | low | mitigate | check-traceability-paths.sh OK (313) | closed |
| T-21-45 | Information disclosure | links/tokens/clipboard in evidence | high | mitigate | same scrub as T-21-39; test words only | closed |
| T-21-46 | Tampering | input on host | high | mitigate | same as T-21-40 | closed |
| T-21-47 | Tampering (availability) | key held on office VM after rows | medium | mitigate | H2 evidence held=none; 21-12 end state | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-21-01 | T-21-03 | Corrected at audit. The plan's rationale ("a different account cannot pass the same-user cap route or /desktop/ws") holds only against an honest peer: same-user is enforced by the *serving* Mac's whois check. The *viewing* side does not check the peer's account: discoverPeers returns every online peer and tailnet.Peer has no user field. A different-account node visible on the tailnet (e.g. a shared-in node) could run its own HTTPS server on the share port with a valid cert for its MagicDNS name, announce a Desktop row, hand back any token, and then receive control-mode keystrokes and, on Cmd+V, the home clipboard's plain text once the user opens that row and clicks into it. This is the same trust model the existing terminal peer path already uses. Accepted for Phase 21 (the target setup is two Macs on one account); flagged for the user and for Phase 22 (guest sharing changes the account model) to decide whether the viewer should filter peers to its own UserID before connectPeerDesktop.  **Resolved 2026-11-27 (orchestrator decision):** the viewer-side filter is now a required Phase 22 item (commit f4c2f8e1, 22-CONTEXT.md): Desktop rows filter to the viewer's own Tailscale account, so a different-account peer gets no Desktop row. Accepted for Phase 21 until Phase 22 ships that filter. | orchestrator (Phase 21 run); resolved by orchestrator decision 2026-11-27 | 2026-11-27 |
| AR-21-02 | T-21-15 | Same exposure as the terminal proxy today: websocket.Accept refuses cross-site browser origins (TestRemoteDesktopWS_RejectsCrossSiteOrigin); same-uid processes can already use the 0600 socket; other local users on a single-user MacBook Air are the residual. | plan 21-06 (accepted at plan time) | 2026-11-27 |

*Accepted risks do not resurface in future audit runs.*

### Auditor notes (non-blocking)

- IN-03 from the code review (`desktopInput.destroy()` does not release held input) remains unfixed; unreachable in the app because the tab-close layout cleanup releases first, but latent for other callers (relates to T-21-30).
- HEAD also contains Phase 22 code (e.g. a listen-only input tap from 22-03); it is outside Phase 21 and not counted against T-21-34.

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-11-27 | 48 | 48 | 0 | gsd-security-auditor (ASVS L1, block_on high) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-11-27 (AR-21-01 rationale corrected; resolved 2026-11-27 by making the own-account Desktop-row filter a required Phase 22 item, f4c2f8e1)
