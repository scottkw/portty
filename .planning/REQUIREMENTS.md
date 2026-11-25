# Requirements: Portty v2.0 Desktop Sharing

**Defined:** 2026-11-24
**Core Value:** A terminal session started on one Mac can be attached to from another Mac (or a browser) over the tailnet, and it survives any idle gap, from minutes to overnight, with history intact.
**Milestone goal:** Use the office Mac's whole desktop from home (Portty app, browser, iPhone) as easily as its terminal today, and let Portty updates install themselves.

## v2.0 Requirements

### Feasibility spike (FEAS)

- [x] **FEAS-01**: On a real Mac, a throwaway end-to-end path (ScreenCaptureKit → VideoToolbox H.264 → pion → browser) shows the desktop live over the tailnet, with a recorded go/no-go for: which process holds Screen Recording/Accessibility (helper vs daemon, incl. SSH-spawned daemon), WebRTC in the Wails web view, browser connections over the Tailscale interface (Chrome, Safari, Portty app, iPhone Safari), lock-screen capture and CGEvent typing, display sleep/wake, monthly re-approval behaviour, CPU/latency on the MacBook Air, Opus audio encode
- [x] **FEAS-02**: The capture-host decision (separate bundled helper vs in-daemon) is recorded with evidence before the capture host is built

### Self-install (UPD — continues v1.0 UPD-01..04)

- [ ] **UPD-05**: After the user clicks Update and the DMG passes the existing checks (SHA-256, Team ID, signing identifier, notarization), Portty replaces itself in `/Applications` and relaunches; terminal sessions survive (v1.2 takeover)
- [ ] **UPD-06**: The update DMG is ejected and deleted after a successful install
- [ ] **UPD-07**: If the new version fails to start, Portty rolls back to the previous version by itself
- [ ] **UPD-08**: Opening Portty from a mounted DMG moves it to `/Applications` (or `~/Applications` if not writable), relaunches from there, ejects the DMG, and moves the downloaded DMG to the Trash
- [ ] **UPD-09**: Any install failure falls back to today's Reveal-in-Finder flow with a clear message; a relaunch never stops at a Gatekeeper dialog

### Permissions & state (DPRM)

- [ ] **DPRM-01**: User is guided to grant Screen Recording and Accessibility, and a test capture confirms they work; grants are confirmed by polling the permission state (not by the system dialog), with the System Settings page offered and no short timeout
- [ ] **DPRM-02**: Host and viewers see the desktop state: permission missing, waiting for approval at the Mac, display asleep, locked, streaming (N viewers)
- [ ] **DPRM-03**: When the macOS Screen Recording approval lapses, Portty shows "waiting for approval at the Mac" and sends a native notification; lapse detection does not rely on zero frames (approval-store date where readable, otherwise a pointer nudge plus frame check)
- [ ] **DPRM-04**: While viewers are connected, the shared Mac holds display-awake and system-sleep assertions so the display stays on (lid-closed sleep excepted), and a stream that ends on display sleep restarts by itself after the wake; waking an already sleeping Mac from a viewer is not promised

### Viewing (DESK)

- [ ] **DESK-01**: Viewer sees the shared display live, sharp on Retina, fitted to the window
- [ ] **DESK-02**: Desktop media stays on the tailnet: no STUN/TURN/cloud, no desktop socket binds a non-tailnet address, and the two real Macs reach each other on a direct tailnet path
- [ ] **DESK-03**: Viewer reconnects by itself after a network drop, Mac sleep/wake or Portty upgrade, showing "Reconnecting"
- [ ] **DESK-04**: Several viewers can watch one share at once; a new viewer gets a picture right away
- [ ] **DESK-05**: Sharer picks a whole display or a single window to share
- [ ] **DESK-06**: Displays can be switched mid-session
- [ ] **DESK-07**: Picture is sharp when the screen is still and smooth when moving, adapting to the network

### Control (DCTL)

- [ ] **DCTL-01**: Viewer with control can move, click, scroll and drag; the pointer lands at the right spot on Retina displays
- [ ] **DCTL-02**: Viewer can type, including shortcuts and non-ASCII text; no keys stay stuck after blur or disconnect
- [ ] **DCTL-03**: Viewer can send Cmd+Tab, Cmd+Space and Lock Screen from a Send-keys menu; optional Ctrl→Cmd swap for non-Mac viewers
- [ ] **DCTL-04**: With several controlling viewers, whoever typed last holds control for about 2 s; physical input at the Mac always wins
- [ ] **DCTL-05**: In a window share, the window is raised when controlled and clicks are confined to it
- [ ] **DCTL-06**: Viewer can copy text from and paste text to the shared Mac by explicit action; concealed (password-manager) items never sync
- [ ] **DCTL-07**: Viewer hears the shared Mac's audio; it starts muted on the viewer with tap to unmute

### Viewers (DVWR)

- [ ] **DVWR-01**: Portty app on another Mac opens a peer's desktop in a Desktop tab from the sidebar
- [ ] **DVWR-02**: Desktop Chrome and Safari open a desktop from a link
- [ ] **DVWR-03**: iPhone (Safari or Home Screen) viewer has trackpad mode, pinch-zoom, long-press right-click, two-finger scroll and soft keyboard; video starts from a tap (iOS will not autoplay it)
- [ ] **DVWR-04**: In control mode in the Portty app, Cmd+W, Cmd+Q and Cmd+C go to the remote Mac instead of Portty

### Share modes & trust (DSHR)

- [ ] **DSHR-01**: User can turn on always-available desktop; their own devices (same Tailscale account) can open it any time with nobody at the Mac and the GUI closed; the daemon runs under launchd only (the CLI restarts it with `launchctl kickstart`; a daemon started from an SSH shell is unsupported)
- [ ] **DSHR-02**: Always-available links, including iPhone Home Screen links, survive daemon restarts and upgrades
- [ ] **DSHR-03**: Share Desktop creates a link that other tailnet people can open; terminal links stay limited to the host's own account
- [ ] **DSHR-04**: Each guest waits at an Admit prompt showing their tailnet identity; guests are view-only until the sharer allows control (and clipboard) for that person
- [ ] **DSHR-05**: Sharer can stop the share or revoke one viewer at any time, ending their connection immediately; links die with the share
- [ ] **DSHR-06**: The shared Mac shows join/leave notifications and a persistent "shared with N" indicator with one-click stop
- [ ] **DSHR-07**: Joins, leaves and control grants are written to a local audit log
- [ ] **DSHR-08**: A desktop link never opens a terminal session, and a terminal link never opens the desktop

### Remote unlock (UNLK — feasible per FEAS-01)

- [ ] **UNLK-01**: A viewer on the host's own account sees the lock screen of a locked, logged-in Mac and types the password to unlock it (feasible per FEAS-01: typing starts after a wake key, the lock state is checked afterwards and the unlock retried once); the password is never stored or logged

### Release (REL)

- [ ] **REL-01**: Releases are Apple Silicon only with minimum macOS 14; every nested binary is signed, notarized and verified in CI

## Future Requirements

- Data-channel cursor (client-drawn) if baked-in cursor lag is measured as a problem
- HEVC (H.265) when every viewer supports it, H.264 fallback (user: "Not now", revisit after v2.0)
- H.264 High profile if a measured sharpness gain and confirmed decode in Chrome/Safari/WKWebView
- Direct-touch mode on iPhone (trackpad mode first)
- Persistent-content-capture entitlement wiring (provisioning profile in signing) once Apple grants it

## Out of Scope

| Feature | Reason |
|---------|--------|
| Desktop access before login, after reboot, or with FileVault locked | ScreenCaptureKit is unavailable pre-login; needs root pre-login agents |
| Stored-password unlock | Security: the viewer types the password live each time |
| Curtain / privacy mode | Not needed; high complexity |
| File transfer, microphone/voice, session recording | Not needed for remote control |
| STUN, TURN or any relay; links for people outside the tailnet | Tailnet-only constraint |
| Waking a sleeping Mac (lid closed) | No API; show "asleep" instead |
| Silent update install while viewers are connected | Would cut off an active viewer |
| Multi-cursor co-control, gaming tuning (120 fps, 4:4:4, gamepads) | Not the use case |
| Sparkle | Existing verified updater is reused |
| Intel Macs | Both target Macs are Apple Silicon (user decision 2026-11-24) |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| FEAS-01 | Phase 17 | Complete |
| FEAS-02 | Phase 17 | Complete |
| UPD-05 | Phase 18 | Pending |
| UPD-06 | Phase 18 | Pending |
| UPD-07 | Phase 18 | Pending |
| UPD-08 | Phase 18 | Pending |
| UPD-09 | Phase 18 | Pending |
| DPRM-01 | Phase 19 | Pending |
| DPRM-02 | Phase 19 | Pending |
| DPRM-03 | Phase 19 | Pending |
| DPRM-04 | Phase 19 | Pending |
| DESK-01 | Phase 19 | Pending |
| DESK-02 | Phase 19 | Pending |
| DESK-03 | Phase 19 | Pending |
| DESK-04 | Phase 19 | Pending |
| DESK-05 | Phase 20 | Pending |
| DESK-06 | Phase 20 | Pending |
| DESK-07 | Phase 23 | Pending |
| DCTL-01 | Phase 20 | Pending |
| DCTL-02 | Phase 20 | Pending |
| DCTL-03 | Phase 20 | Pending |
| DCTL-04 | Phase 22 | Pending |
| DCTL-05 | Phase 20 | Pending |
| DCTL-06 | Phase 20 | Pending |
| DCTL-07 | Phase 23 | Pending |
| DVWR-01 | Phase 21 | Pending |
| DVWR-02 | Phase 19 | Pending |
| DVWR-03 | Phase 21 | Pending |
| DVWR-04 | Phase 21 | Pending |
| DSHR-01 | Phase 19 | Pending |
| DSHR-02 | Phase 19 | Pending |
| DSHR-03 | Phase 22 | Pending |
| DSHR-04 | Phase 22 | Pending |
| DSHR-05 | Phase 22 | Pending |
| DSHR-06 | Phase 22 | Pending |
| DSHR-07 | Phase 22 | Pending |
| DSHR-08 | Phase 19 | Pending |
| UNLK-01 | Phase 23 | Pending (feasible per FEAS-01: ships) |
| REL-01 | Phase 18.1 | Pending (moved from Phase 23 on 2026-11-25; Phase 23 ships v2.0 on this pipeline) |

**Coverage:**
- v2.0 requirements: 39 total
- Mapped to phases: 39
- Unmapped: 0
- Per phase: 17 (2), 18 (5), 19 (12), 20 (7), 21 (3), 22 (6), 23 (4)

---
*Requirements defined: 2026-11-24*
*Last updated: 2026-11-25 after Phase 17 (requirement changes from the spike matrix)*
