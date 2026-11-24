# Project Research Summary

**Project:** Portty
**Milestone:** v2.0 Desktop Sharing (plus DMG self-install)
**Domain:** macOS remote desktop sharing (WebRTC, tailnet-only) and in-app DMG self-install, added to an existing Go daemon + Wails app
**Researched:** 2026-11-24
**Confidence:** MEDIUM overall. Repo facts HIGH; Apple TCC, lock-screen and WKWebView behaviour rests on forum threads and beta reports and must be proven on a real Mac before the roadmap commits to it.

## Executive Summary

Portty v2.0 adds whole-desktop view and control to a terminal app that already has tailnet-only shares, a token-gated share server, and a daemon that survives upgrades. Comparable products (Parsec, RustDesk, Chrome Remote Desktop, Sill) share one shape: ScreenCaptureKit capture, hardware H.264, WebRTC transport, CGEvent input. Portty's differentiators: one app for terminal and desktop, browser and iPhone viewers, no relay or cloud, admit-by-tailnet-identity for click-to-share. The repo does **not** use tsnet: it uses the system tailscaled and binds the share server to the tailnet IP, so pion can bind UDP on the same IP with no STUN/TURN/public ICE servers.

Recommended stack: `pion/webrtc/v4` v4.2.29 (empty ICE servers, UDP mux on the tailnet IP, mDNS off); ScreenCaptureKit (`420v`) + VideoToolbox H.264 Constrained Baseline, no B-frames, Annex-B; AudioToolbox Opus `AudioConverter`; CGEvent; NSPasteboard; plain browser `RTCPeerConnection` shared by Wails tab, browser link and iPhone view. DMG self-install reuses the existing verify chain and adds `hdiutil` mount, `ditto` staging, quarantine clearing, atomic `RENAME_SWAP`, relaunch, rollback to `.prev`. Independent of desktop work; rides the v1.2 takeover handoff.

Main risks: (1) which process holds Screen Recording/Accessibility is an open conflict (C1) that drives architecture; (2) monthly Screen Recording re-consent can block an unattended Mac; (3) lock-screen capture/typing and WKWebView WebRTC are unproven. Mitigation: real-Mac go/no-go spike first; file Apple's `persistent-content-capture` entitlement request now; keep remote unlock off the critical path until proven.

## Conflicts Between Research Files (surfaced, not silently resolved)

### C1. Where capture, encode and input run (blocking decision)

| | ARCHITECTURE.md (helper process) | STACK.md (in-daemon cgo) |
|---|---|---|
| Where | `PorttyDesktop` helper in `Portty.app/Contents/Helpers/`, LaunchAgent via `SMAppService` `BundleProgram`, id `app.portty.Portty.Desktop` | cgo ObjC package `internal/screencap` inside the `portty` daemon, same signed binary |
| Main argument | Crash isolation: daemon owns every PTY; a cgo crash would end every terminal session on an unreachable Mac | One TCC identity: one Screen Recording + one Accessibility grant covers GUI, daemon, capture |
| TCC risk cited | Daemon spawn path uncontrolled (GUI/CLI/SSH); SSH-respawned daemon may be attributed to Terminal/sshd; LaunchAgent helper removes that | DTS: GUI work belongs in an Aqua LaunchAgent; daemon already is one (forum guidance, MEDIUM) |
| Cost accepted | Second TCC identity, second prompts; Tahoe may not list a plain executable | Crash risk in terminal-owning process; SSH-spawned respawn fails "unavailable" |

PITFALLS.md leans ARCHITECTURE (daemon never links SCK or posts CGEvents). **Resolution:** Phase 0 real-Mac spike decides. Keep capture behind a process-agnostic interface (`desktopproto` frames) usable in-process or over a socket. Spike answers: (a) where the grant lands for a launchd helper and an SSH-spawned daemon; (b) capture with GUI closed; (c) capture across lock/unlock; (d) whether a second identity is avoidable/acceptable.

### C2. Clipboard sync
STACK/FEATURES: poll `changeCount`, auto host→viewer. PITFALLS: explicit gestures only (macOS 15.4+ paste alert would hang unattended; leaks password managers). **Recommendation:** explicit gesture both directions, skip Concealed/Transient types, test background `changeCount` on 27.2.x; off by default for click-to-share.

### C3. Full ICE vs ICE-lite
Test both in the spike join matrix; per-browser choice in Phase 0 exit criteria.

### C4. Resolution / frame rate
Default long-edge cap 2560, 30 fps; measure on real Mac; constant in encoder config.

### C5. H.264 profile
Constrained Baseline only (`42e01f`) for v2.0; High only after measured gain and confirmed decode.

### C6. Cursor
Baked-in cursor (`showsCursor = YES`) for v2.0; data-channel cursor only if lag is measured.

### C7. Audio encoder
AudioConverter Opus first; final choice in the late audio phase; libopus only if it fails.

### C8. DMG rollback
Adopt rollback: keep `Portty.app.prev` until new bundle reports healthy.

### C9. Relaunch signalling
Marker file (`pending-install.json`) plus detached finisher, respecting the Phase 16 unknown-flag rule.

### C10. Identity lookup
Repo fact wins: system tailscaled LocalAPI `WhoIs` via existing whois gate. No tsnet.

### C11. Universal vs arm64 — pending user decision (research recommends arm64-only).

### C12. Minimum macOS — pending user decision (13 supports SCK audio + SMAppService; 14.4 if the entitlement is pursued).

## User Facts Since Research (applied)
- Office Mac uses the plain macOS lock screen (no Jamf Connect / Platform SSO).
- User is admin on the office Mac; `/Applications` writable. `~/Applications` fallback stays for other Macs.
- Remote unlock: only "locked, still logged in" in scope; pre-login/reboot/FileVault excluded.
- Click-to-share may be used by other tailnet people; view-only default pending confirmation.

## Key Findings

### Recommended Stack
- `github.com/pion/webrtc/v4` v4.2.29 (not v5 RCs; avoid `SetNAT1To1IPs`); `pion/interceptor` v0.1.51 (`pkg/cc`, `pkg/gcc`). Only new direct Go dependency is pion. `coder/websocket`, `golang.org/x/sys` (`RenamexNp`) already present.
- ScreenCaptureKit (13+), VideoToolbox H.264 (`AllowFrameReordering=false`, Annex-B, SPS/PPS each IDR), CGEvent (Accessibility only), NSPasteboard, IOKit power assertions.
- Viewer: vanilla browser WebRTC; no JS WebRTC libraries.
- Rejected: Sparkle, libopus (unless needed), robotgo, CGWindowListCreateImage/CGDisplayStream, pion/opus (decoder only), STUN/TURN, `ditto` over a running app.

### Expected Features
**Table stakes:** live display view (Retina-aware, fit); mouse/keyboard (key codes + text path, stuck-key release); permission onboarding with test-capture probe; always-available mode for own tailnet Macs with tray state and notifications; click-to-share link with admit prompt, view-only default, expiry, revoke; multi-viewer fan-out (one encoder, keyframe on join); display switch; browser + iPhone viewers (trackpad mode, pinch-zoom, key bar); reconnect; power assertions while viewers connected; DMG in-app install with rollback + first-launch move/eject/trash.

**Differentiators:** adaptive quality (last); tailnet-identity admit with per-viewer permissions; soft active controller (~2 s idle; host input wins); clipboard text by explicit gesture; system audio start-muted; window share with raise + clamp; Ctrl→Cmd swap and "Send keys" menu; lock on disconnect only if session started locked.

**Anti-features:** pre-login/reboot/FileVault access; stored-password unlock; curtain mode; file transfer; mic/voice; recording; STUN/TURN/relay; waking a sleeping Mac; silent update while viewers connected; private virtual display; multi-cursor; gaming tuning.

### Architecture Approach
Viewers speak one protocol. Signalling over the authenticated share-server WSS (Wails via loopback daemon proxy, origin `wails://`); media UDP direct over the tailnet. Daemon owns pion PeerConnections, tokens, grants, consent, quality policy. Capture behind C1.

1. Capture host (helper or in-daemon): SCK, VT, CGEvent, NSPasteboard, permission reporting; demand-driven.
2. `internal/desktop` (new): PeerConnection per viewer, shared encoder track, per-viewer perms, consent, min-of-viewers bitrate, PLI/FIR coalescing.
3. Share-server routes: `/desktop/{id}`, `/desktop/{id}/ws`, `/api/peer/desktop/cap`, `requireTailnetPeer` for click-to-share; share-kind tokens out of `/sessions/*`.
4. `internal/capability/desktoptoken.go`: HKDF-separated desktop token (precedent `porttoken.go`).
5. Share-server lifetime (`share.go`, `handoff.go`): `serverNeeded()` replaces `len(a.shares)==0`. Largest change to existing code. Desktop sessions not handed off; do not bump `HandoffVersion`.
6. GUI/frontend: settings, permission status, share popover, desktop tab, `DesktopView.tsx`, `lib/desktopClient.ts`, `lib/desktopInput.ts`.
7. Updater (`app_install.go`): mount, verify staged app, ditto, quarantine clear, RENAME_SWAP, relaunch, detach, trash, rollback.

Unchanged: `internal/relay`, `internal/fdpass`, `launchMode`, terminal share routes, CSP (pin "no new directive" with a test).

## Implications for Roadmap

- **Phase 0: Real-Mac go/no-go spike (timeboxed).** Throwaway SCK→VT→pion→HTML path. Exit criteria: TCC attribution (C1); WKWebView `RTCPeerConnection` + H.264 + DataChannel + Local Network prompt; browser utun ICE in Chrome/Safari/WKWebView/iOS Safari (C3); lock-screen frames and CGEvent password typing on the office Mac's macOS; display sleep/wake; monthly re-consent; latency/CPU/sharpness (C4); audio path (C7). Parallel: file the persistent-content-capture entitlement.
- **Phase 1: DMG self-install.** Independent, low risk; prerequisite for shipping an embedded helper. In-app install + relaunch + delete DMG; first-launch move (or `~/Applications`), eject after new instance starts, trash via `NSWorkspace recycleURLs`; rollback to `.prev`; marker file; translocation check; quarantine stripped after verify; stdout-only runner for `hdiutil -plist`; Reveal fallback. Acceptance: clean-VM install N → N+1 two-build upgrade; forced bad build rolls back; eject with daemon running; no Gatekeeper dialog; real-Mac check on office Mac.
- **Phase 2: Capture host, permissions and state.** Process-agnostic interface; onboarding prompts from GUI; user-triggered test capture; state machine (permission missing, needs approval, display asleep, no display, waiting, streaming); zero-frame detection for monthly expiry with notification; power assertions; raise `LSMinimumSystemVersion`.
- **Phase 3: Desktop core.** `internal/desktop` with pion (tailnet-only UDP, IP filter, UDP4, mDNS off; test no 0.0.0.0/LAN bind); non-trickle signalling over WSS; desktop token; grant registry; `serverNeeded()` + handoff tests; always-available mode with persisted standing grant, same-user only; browser viewer; CSP e2e; cross-token 401 regression; fan-out + IDR coalescing; GCC min-of-viewers.
- **Phase 4: Input, display switch, clipboard.** Key/pointer mapping, Retina correctness, release-all on close + dead-man timer, input limits, display switch with epoch, clipboard per C2.
- **Phase 5: Wails desktop tab and iPhone viewer.** Loopback signalling proxy, sidebar desktop row via `X-Portty-Desktop` header, desktop tab type, control mode disabling menu key equivalents, iPhone touch controls, unmute overlay.
- **Phase 6: Click-to-share (tailnet people).** `requireTailnetPeer`, ephemeral grant, per-viewer admit sheet (view-only default), revoke, "shared with N" indicator, audit log, clipboard off for guests. Security review.
- **Phase 7: Multi-viewer control, audio, adaptive quality, window share.**
- **Phase 8: Hardening and release.** Two-build upgrade with live desktop session; capture-host restart after update; 1-hour soak; nested signing inside-out, DR diff; TESTING.md updates and M-NN real-Mac items.
- **Remote unlock (gated after Phase 0).** Only if SCK lock-screen frames and CGEvent typing are proven on the office Mac. Never store or log the password; guests never get unlock. If it fails, document "unattended works while awake and unlocked."

### Research Flags
Deeper research during planning: Phase 0, 1, 2, 3, 7, 8/unlock. Standard patterns: Phase 4, 5, 6.

## Pending User Decisions
1. Minimum macOS: 13 or 14 (14.4 if entitlement granted).
2. View-only default for click-to-share viewers.
3. arm64-only vs universal (research: arm64-only).
4. C1 capture host — decided by Phase 0.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | MEDIUM | Versions/APIs verified from source; DMG mechanics and Opus verified locally on macOS 27.2.1; TCC/WKWebView/re-consent are spike items |
| Features | MEDIUM | Corroborated across products; lock-screen and macOS 27 permission details LOW |
| Architecture | MEDIUM | Repo facts HIGH; TCC attribution and browser utun MEDIUM/LOW; C1 unresolved |
| Pitfalls | MEDIUM | DTS statements HIGH; macOS 26/27 behaviours beta-era |

### Gaps to Address
- C1 TCC identity (Phase 0, before Phase 2)
- Lock screen on office Mac (Phase 0, gates unlock)
- WKWebView and utun (Phase 0; fallback: open browser link)
- Display sleep/wake (Phase 0/2)
- Monthly re-consent (someone at the Mac monthly unless entitlement granted)
- Tailscale mode on office Mac (network extension vs userspace; detect and refuse userspace)
- App Management same-Team-ID swap on office Mac
- Opus API recipe (Phase 7)
- pion stays v4.2.29
- Entitlement request not yet filed
- Test rigs: Tart has no hardware encoder or TCC automation; real-Mac rows required

## Sources
Primary: repo code (webserver, daemon share/handoff/process/proxy, capability, updater, app_update, app_cli_install, main.go, Info.plist, entitlements, release.yml, TESTING.md); pion v4.2.29 source; local experiments on macOS 27.2.1 (afconvert, hdiutil, ditto quarantine, go get).
Secondary: Apple Developer Forums (DTS) on login-window capture, responsible process, LaunchAgent helpers, persistent-content-capture; Sequoia monthly prompt reporting; RustDesk/Parsec/Sill/CRD/Screens docs; WebKit, webrtcHacks, MDN; Chromium Tailscale-VPN detection; W3C Local Network issue.
Tertiary (LOW): Tahoe 26.1 executables missing from Screen Recording; macOS 27 "Remote Desktop" category; 27.3 beta prompts; lock-screen anecdotes; `forceBypassScreenCaptureAlert`.

---
*Research completed: 2026-11-24*
*Ready for roadmap: yes, after Phase 0 decides C1 and pending user decisions are recorded*
