---
phase: "19"
slug: "unattended-desktop-viewing"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-11-26"
---

# Phase 19 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

Register taken from the `<threat_model>` blocks of 19-01 through 19-17 (authored at plan time). ASVS L1, block_on high. Every mitigation was confirmed at grep depth, and the anchor tests were re-run (`go test` on capability, daemon, desktop and webserver: all pass; each refuses host side effects under `testing.Testing()`). Because 0 threats were open with a plan-time register at L1, the workflow short-circuited and the auditor subagent was not spawned.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| tailnet browser to share server | Requests to /desktop/, /desktop/{path} and /desktop/ws with a token in the query | Desktop capability token (secret); static page assets |
| viewer to signalling / pion | JSON messages, SDP offers, RTCP feedback and data channels from the viewer | Untrusted protocol input |
| tailnet UDP to the media socket | STUN, DTLS and SRTP from any tailnet address | Screen video (sensitive) |
| GUI and CLI to the user's launchd domain | Portty writes a plist and asks launchd to run its job | Executable path; process lifecycle |
| launchd daemon to an unmanaged daemon / old daemon to takeover | Re-home and upgrade handoff over the existing same-uid handoff socket | Listener fds, session and share records |
| daemon to ScreenCaptureKit / TCC | Screen content and permission state cross from the OS into the daemon | Screen pixels (sensitive); permission state |
| ObjC framework threads to Go | Dispatch-queue callbacks call exported Go functions | Frames, errors |
| page URL to client / share server to page | Token from the page's own query; state and size messages from the server | Token (secret); enums and integers |
| GUI and CLI to the daemon socket (0600, same user) | PUT /desktop, permissions, test capture, lapse claim | Grant state, thumbnail (sensitive) |
| config dir to the daemon | desktop.json and desktop-state.json read at start | Grant and breaker state |
| daemon to the perm-probe child | The child's stdout JSON is parsed | Permission answers |
| daemon to the OS power manager and event system | Assertions and a posted pointer event | One-point pointer nudge |
| GUI to /usr/bin/open and clipboard | Fixed System Settings URLs; link copied on explicit click | Link with token (secret) |
| process to Notification Center | Fixed text under the app bundle identity | Fixed strings |
| VM-only build tag to release builds | The simulated-lapse route must never ship | Debug route |
| host to VMs; evidence and TESTING.md to the repo | Rig builds on the host and acts only in the VMs; captures and docs are committed | Addresses, names, tokens, home paths (must not appear) |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-19-01 | Elevation of privilege | VerifyDesktop / Verify / VerifyPort (token confusion) | high | mitigate | HKDF info `portty/desktop-token/v1` (internal/capability/desktoptoken.go:23); kind/mode/v/grant checks; TestDesktopToken_RoundTripAndSeparation (internal/capability/desktoptoken_test.go:35) | closed |
| T-19-02 | Spoofing | /desktop/ws cross-site WebSocket hijack | high | mitigate | requireAllowedOrigin, requireSameUser, token outcome, all before Accept; TestDesktopWS_RefusesBeforeUpgrade (internal/desktop/loopback_test.go:473) | closed |
| T-19-03 | Denial of service | ServeDesktopViewer oversized/slow messages | medium | mitigate | readLimit 32768 and SetReadLimit (internal/desktop/manager.go:85, :463); hello/offer deadlines, strict first-message order, recoverLog per goroutine | closed |
| T-19-04 | Information disclosure | logs and page headers | medium | mitigate | No token/SDP/address logged; page through cspHeaders (internal/webserver/desktop_routes.go:106) | closed |
| T-19-05 | Denial of service | Kickstart / bootout session loss | high | mitigate | No `-k` in any launchctl call (TestLaunchd_NoKillFlag, internal/daemon/launchd_test.go:662); ErrAgentRunning refuses uninstall of a running job (internal/daemon/launchd.go:45). The `launchctl kill SIGTERM` at launchd.go:372 is the explicit `portty daemon stop` user action, not the kickstart kill flag | closed |
| T-19-06 | Tampering | plist content and path | medium | mitigate | Rendered from the running executable with XML escaping, 0644 temp+rename, /Volumes and App Translocation refused (internal/daemon/launchd.go) | closed |
| T-19-07 | Denial of service | launchd restart loop | medium | mitigate | Exit 1 only for real startup failures; lock-held and refused re-home exit 0 | closed |
| T-19-08 | Elevation of privilege | TCC attribution of a non-launchd daemon | medium | mitigate | Kickstart first, typed-error fallback surfaced; TestEnsureDaemon_KickstartFirstFallbackOnTypedError (internal/daemon/launchd_test.go:367) | closed |
| T-19-09 | Tampering | tests touching the user's launchd domain | high | mitigate | defaultLaunchctl and fallbackSpawn refuse under testing.Testing() (internal/daemon/launchd.go:54, :127); TestEnsureDaemon_FallbackRefusesUnderTest (internal/daemon/launchd_test.go:471) | closed |
| T-19-10 | Information disclosure | media egress off the tailnet | high | mitigate | checkBind 100.64.0.0/10 only (internal/desktop/bind.go:41), UDP4, mDNS off, no ICE servers; TestCheckBind_Boundaries (internal/desktop/bind_test.go:26), TestPeer_NoStunTurnMdnsTraffic (internal/desktop/peer_test.go:84) | closed |
| T-19-11 | Denial of service | viewer flood and PLI storms | medium | mitigate | MaxViewers 8 with refusal (internal/desktop/manager.go:54); 1 s keyframe coalescing; per-viewer PC closed with its socket | closed |
| T-19-12 | Denial of service | data channel messages | medium | mitigate | SetSCTPMaxMessageSize(8192) (internal/desktop/peer.go:64); Phase 19 hook closes every channel | closed |
| T-19-13 | Spoofing | unauthenticated STUN to the mux | low | accept | See Accepted Risks Log AR-19-01 | closed |
| T-19-14 | Elevation of privilege | token kinds across route families | high | mitigate | TestCrossToken_Matrix (internal/webserver/crosstoken_test.go:16) | closed |
| T-19-15 | Spoofing | other tailnet accounts on /desktop/ | high | mitigate | requireSameUserPage / requireSameUser before the token (internal/webserver/desktop_routes.go:106) | closed |
| T-19-16 | Information disclosure | static pages | low | mitigate | Static bundle holds no request data; no-store/no-referrer from cspHeaders | closed |
| T-19-17 | Tampering | CSP drift | medium | mitigate | TestDesktopRoutes_CSPPinned (internal/webserver/desktop_routes_test.go:221); `git diff 89d29266 -- internal/webserver/csp_mw.go` empty | closed |
| T-19-18 | Denial of service | ObjC exceptions / nil literals crashing the daemon | high | mitigate | @try/@catch in internal/screencap/media_darwin.m and preflight_darwin.m; Go recover in exported callbacks; typed errors | closed |
| T-19-19 | Information disclosure | host screen captured by tests | high | mitigate | TestDisplaySource_RefusesUnderTest (internal/screencap/screencap_darwin_test.go:36), TestSysBridges_RefuseUnderTest (:96) | closed |
| T-19-20 | Elevation of privilege | stale cached permission answers / prompting API | medium | mitigate | Preflight confined to preflight_darwin.m, child process only; repo-wide symbol test forbids prompting variants | closed |
| T-19-21 | Information disclosure | token in URL | medium | mitigate | Same-origin socket and fetch only (frontend/src/lib/desktopClient.ts); page no-store/no-referrer; debug hook exposes counts only | closed |
| T-19-22 | Tampering | server messages rendered into the page | low | mitigate | Only enums and integers read; unknown state dropped; no server text rendered (frontend/src/lib/desktopClient.ts) | closed |
| T-19-23 | Elevation of privilege | CSP weakened for media | medium | mitigate | No CSP change; TestDesktopCSP_E2E (internal/webserver/desktop_csp_e2e_test.go:85) | closed |
| T-19-24 | Tampering | host launchd domain and /Applications (rig) | high | mitigate | Rig acts only through the VM and refuses when the VM is not running (.planning/phases/19-unattended-desktop-viewing/rig/p19-rig.sh:52) | closed |
| T-19-25 | Information disclosure | committed evidence | medium | mitigate | Evidence grep for tailnet addresses, ts.net names, `cap=` tokens and home paths returns nothing; check-brand clean | closed |
| T-19-26 | Tampering | desktop.json | high | mitigate | 0600 regular file, size cap, strict schema, atomic writes; TestDesktopConfig_Unsafe (internal/desktop/config_test.go:59) | closed |
| T-19-27 | Elevation of privilege | stale grant after off or key regeneration | high | mitigate | TestDesktopAPI_OffClosesViewers (internal/daemon/desktop_api_test.go:532), TestDesktopAPI_RegenerateKey (:592) | closed |
| T-19-28 | Denial of service | share server lifetime regressions | high | mitigate | serverNeededLocked (internal/daemon/desktop_api.go:117) at every stop site; TestShareLifetime_ matrix (internal/daemon/share_lifetime_test.go) | closed |
| T-19-29 | Information disclosure | always-available on userspace Tailscale | medium | mitigate | ShareErrUserspace refused before bind (internal/daemon/desktop_api.go:353); test row internal/daemon/desktop_api_test.go:505 | closed |
| T-19-30 | Information disclosure | link in logs | medium | mitigate | URL and token never logged; status carries no token | closed |
| T-19-31 | Denial of service | capture crash loop | high | mitigate | Breaker after 3 abnormal exits/hour, reset only by Test Capture; TestBreaker (internal/desktop/watchdog_test.go:473) | closed |
| T-19-32 | Tampering | perm-probe output and desktop-state.json | medium | mitigate | Strict decode, 0600 regular file, known schema; TestProber_StrictAndCached (internal/desktop/state_test.go:141), TestRuntimeState_File (internal/desktop/watchdog_test.go:529) | closed |
| T-19-33 | Repudiation | unattended capture without consent | medium | mitigate | Capture and assertions only with a viewer (plus the user-approved AC assertion); macOS capture indicator | closed |
| T-19-34 | Elevation of privilege | remote input via the nudge | low | mitigate | One-point move-and-return, only with a viewer, a stall and Accessibility; no other input path | closed |
| T-19-35 | Denial of service | sessions during an upgrade with desktop on | high | mitigate | Protocol unchanged; TestHandoffDesktop_SessionsAndLinkSurvive (internal/daemon/handoff_desktop_test.go:92) | closed |
| T-19-36 | Elevation of privilege | desktop grant surviving where it should not | medium | mitigate | New daemon reads the grant only from desktop.json via LoadConfig; no grant crosses the socket | closed |
| T-19-37 | Denial of service | media port held by two daemons | low | mitigate | Old side closes Manager and socket after COMMITTED; new side binds the same address/port only | closed |
| T-19-38 | Elevation of privilege | OpenDesktopPermissionPane | medium | mitigate | Two fixed URLs by kind; TestOpenDesktopPermissionPane (app_desktop_test.go:91) | closed |
| T-19-39 | Information disclosure | Test Capture thumbnail | low | mitigate | Returned only over the 0600 daemon socket, no-store, never written or logged | closed |
| T-19-40 | Tampering | perm-probe output | low | mitigate | Strict decode; explicit probe_failed (internal/daemon/desktop_api.go:700) | closed |
| T-19-41 | Denial of service | probe spawn storms | low | mitigate | 1 s cache, one shared spawn; TestDesktopPermissions_SharedSpawn (internal/daemon/desktop_permissions_test.go:100) | closed |
| T-19-42 | Elevation of privilege | POST /desktop/debug/lapse in a release build | high | mitigate | `//go:build vmdesktop` (internal/daemon/desktop_vmdesktop.go:1); tests/check-brand.sh:8 fails on the tag in .github, build.sh or scripts | closed |
| T-19-43 | Information disclosure | evidence and links (VM checks) | medium | mitigate | Link stays inside the VM; evidence grep clean; thumbnails are VM screens | closed |
| T-19-44 | Information disclosure | the link in the DOM or logs | medium | mitigate | Read in handleCopy and passed to the clipboard only (frontend/src/components/SettingsSheet.tsx:282); tracer test asserts the DOM never holds it | closed |
| T-19-45 | Repudiation | always-available turned on unknowingly | low | mitigate | One explicit switch with consequence text; status and tray show it is on | closed |
| T-19-46 | Repudiation | duplicate or missing lapse notifications | low | mitigate | Atomic lapse claim (POST /desktop/lapse-claim, internal/daemon/desktop_api.go; app_lapse_test.go:42); daemon 15 s fallback; fixed identifier | closed |
| T-19-47 | Information disclosure | notification and tray text | low | mitigate | Fixed strings only | closed |
| T-19-48 | Tampering | host state during VM checks | high | mitigate | Only `wails build`, `codesign` and `swiftc` on the host; all capture/TCC/launchctl/pf/browser work in the VMs (19-15-SUMMARY) | closed |
| T-19-49 | Information disclosure | links, addresses and screens in evidence | medium | mitigate | Links compared in the VM; evidence grep clean; VM names scrubbed | closed |
| T-19-50 | Information disclosure | values in TESTING.md | low | mitigate | Items use placeholders; `bash tests/check-brand.sh` passes | closed |
| T-19-51 | Elevation of privilege | GUI fallback daemon without KeepAlive | medium | mitigate | Typed-error fallback surfaced, re-home retry; TestStartDaemon_DisabledAgentFallsBackAndMarksRepair (app_launchd_test.go:124), TestApp_RehomeRetryBackoff (app_launchd_test.go:301) | closed |
| T-19-52 | Tampering | launchd pointed at a build tree | low | mitigate | installedExecutable (app.go:523) allows only /Applications or ~/Applications bundles; TestInstalledExecutable (app_launchd_test.go:420) | closed |
| T-19-SC | Tampering | supply chain (Go modules; npm/pip/cargo) | medium | mitigate (19-01) / accept (19-02..19-17) | 19-01: pion pinned to the spike's versions (go.mod:20) with go.sum hashes; later plans add no package (AR-19-02) | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-19-01 | T-19-13 | pion drops STUN binding requests with an unknown ufrag; only viewers past the token gate get ICE credentials. Low severity. | Plan 19-03 threat model (plan-time disposition) | 2026-11-26 |
| AR-19-02 | T-19-SC | Plans 19-02 through 19-17 add no npm, pip, cargo or Go package (19-03 only moved pion/rtcp from indirect to direct). | Plan threat models (plan-time disposition) | 2026-11-26 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-11-26 | 53 | 53 | 0 | /gsd-secure-phase orchestrator (ASVS L1, block_on high; plan-time register, L1 short-circuit, auditor not spawned) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-11-26
