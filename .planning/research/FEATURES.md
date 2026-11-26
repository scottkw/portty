# Feature Research

**Domain:** macOS-hosted remote desktop sharing (WebRTC, tailnet-only) plus DMG self-install, added to an existing terminal app
**Milestone:** v2.0 Desktop Sharing
**Researched:** 2026-11-24
**Confidence:** MEDIUM overall. Product behavior is well corroborated. The lock-screen and macOS 27 permission details are LOW (forum posts and beta reports). No live-tailnet or on-device measurements were made (desk research only).

Confidence tags used below follow the `classify-confidence` seam: a single web source is LOW, a cross-checked one is MEDIUM. `[PRIOR]` marks a claim from training knowledge that I did not re-verify this session. Treat `[PRIOR]` and LOW items as hypotheses for phase-level research, not facts.

## How Comparable Products Behave (answers to the research question)

### 1. Consent and indicator UX on the shared Mac

| Product | Consent model | Indicator on the shared Mac |
|---------|---------------|-----------------------------|
| macOS Screen Sharing | "Anyone may request permission" shows a prompt on the host; otherwise password or account login with no prompt [PRIOR; Apple's Mac Help page confirms the setting exists, MEDIUM] | System menu-bar sharing status [PRIOR] |
| Parsec (Mac host) | Host sees waiting guests and uses Accept All / Kick All; host locks automatically after the last guest disconnects (MEDIUM, Parsec support snippets) | Host-side guest list |
| RustDesk | Accept/deny dialog unless a permanent password is set; per-connection toggles for keyboard, clipboard, audio, file [PRIOR] | In-app connection list; "Lock after session end" option exists (MEDIUM, GitHub issue 11802) |
| Chrome Remote Desktop | Attended "share" needs a one-time code the host confirms. Unattended needs a PIN set at install time and shows no prompt [PRIOR] | Banner with a Stop Sharing button for attended sessions [PRIOR] |
| Sill (iPhone to Mac) | One-time pairing by scanning a code. Paired devices are pinned by key and listed in Settings > Devices. Pairing can be turned off (HIGH, primary README) | Menu-bar app. **Captures nothing while no device is connected** (HIGH, primary README) |
| macOS itself (ScreenCaptureKit) | Screen & System Audio Recording permission, plus a re-confirm prompt about monthly on Sequoia (MEDIUM, multiple outlets) | Purple recording dot in Control Center since 15.1 (LOW, one forum post) |

**Pattern:** unattended modes show no per-connection prompt but always show persistent state. Attended or link modes make the host admit each joiner. Every serious product has a host-visible "who is connected" list and a way to kick.

**Portty implication:** the always-available mode (own tailnet Macs) needs no prompt, but must show persistent tray state ("Desktop: 1 viewer") and fire a notification on connect and leave. The click-to-share mode needs a host admit prompt per joiner, showing the joiner's tailnet identity.

### 2. Multi-viewer control arbitration

- Classic VNC and macOS Screen Sharing: every viewer injects input and events interleave, with no arbitration [PRIOR].
- Parsec and RustDesk: host-side per-guest permission toggles for keyboard and mouse. No handover protocol [PRIOR / LOW].
- Chrome Remote Desktop: effectively one controlling connection at a time [PRIOR].
- Sill: one device streams at a time; two-Mac simultaneity is undocumented (LOW).

**Recommendation:** soft "active controller." The viewer that last clicked or typed holds control for about 2 seconds of idle. Other viewers are view-only until then. This is the same rule Portty already uses for PTY resize ("the viewer you type in sets the size"), so users and code share one mental model. Host physical input always wins and is never blocked. Do not build per-viewer cursors.

### 3. Touch-to-mouse mapping on iPhone

Screens (Edovia, documented, MEDIUM) and Sill (primary README, HIGH) agree on the core vocabulary:

| Gesture | Action |
|---------|--------|
| Tap | Left click |
| Double-tap | Double click |
| Two-finger tap (Screens), or lift after a held finger without moving (Sill) and long-press (Jump) | Right click |
| Two-finger swipe | Scroll |
| Touch and hold until an indicator appears, then drag | Left drag |
| Pinch | Zoom the remote view (Sill goes to 5x, two-finger pan) |
| Three-finger swipe, edge swipes | Mission Control / Spaces / Dock via the Mac's own keyboard shortcuts (differentiator) |

Two cursor modes exist: **Touch** (the cursor jumps to the tap) and **Trackpad** (relative movement). Jump documents that pinch-zoom is not available in its direct-touch mode (LOW, support forum). On an iPhone a 13 to 14 inch Mac screen is roughly 1/4 scale, so direct touch is imprecise without zoom.

**Recommendation:** ship trackpad mode plus pinch-zoom first. Direct touch is a later toggle. The iOS soft keyboard does not emit reliable key codes [PRIOR]. Send composed text as a separate "type this string" message (Sill does this for IME, HIGH) and keep Portty's existing key bar for Esc, Tab, Ctrl, Opt, Cmd and arrows.

### 4. Keyboard mapping (Cmd vs Ctrl, browser viewers)

- The Keyboard Lock API exists only in Chromium (Chrome 68+, Edge 79+), only in JS-initiated fullscreen, with a 2-second Esc escape hatch. Firefox and Safari do not support it (MEDIUM, Chrome docs and MDN agree). Browsers also reserve some shortcuts regardless.
- So a Safari or iPhone viewer can never forward Cmd+W, Cmd+Q, Cmd+T or Cmd+Tab. Provide a **"Send keys" menu** (Cmd+Tab, Cmd+Space, Ctrl+Cmd+Q lock, Esc) as Chrome Remote Desktop and RustDesk do [PRIOR].
- In the Portty Wails app (WKWebView) the app owns its menu bar. It can drop Cmd+W and Cmd+Q key equivalents while the desktop view is focused. Cmd+Tab stays OS-level, so it still needs the menu.
- RustDesk offers Legacy / Map / Translate keyboard modes and users still report layout problems (LOW, GitHub issues). The robust approach is to send **physical key codes** (`KeyboardEvent.code`) plus modifier state and let the host synthesize with its own layout. Send composed text separately.
- Mac viewer to Mac host: map 1:1, Meta to Cmd. Non-Mac browser viewers (colleagues on click-to-share links may use Windows) need an optional **Ctrl to Cmd swap** (Jump, RealVNC and RDP clients all document this pain, MEDIUM). Default it on for non-Mac viewers but make it a visible toggle. Portty is terminal-first, and a Windows viewer's Ctrl+C reaching a remote terminal as Cmd+C would be wrong.
- **Stuck-modifier bug class:** release all keys on viewer blur, disconnect and display switch. RustDesk shipped a fix named "reset modifiers' state after locking screen" (MEDIUM, GitHub issue 11802).

### 5. Retina and scaling

- Captured pixels on a MacBook Air are 2x the logical points. CGEvent mouse coordinates are in points, not pixels [PRIOR]. This is a classic source of off-by-2x click bugs.
- Send pointer position as normalized 0..1 of the shared frame. The host converts to global display points (accounting for display origin and, for window share, the window's current frame).
- Sill sends native-resolution HEVC, drops frames rather than queueing them, and tells the device when to lower quality (HIGH, primary README). ScreenCaptureKit emits "idle" frames when nothing changed, so a still screen costs near zero (MEDIUM, Apple forum plus enum docs).
- **Recommendation:** capture at point size or cap the long edge near 2560. Default the viewer to "fit". Offer "actual size" plus zoom on iPhone. Send a size/scale message over the data channel on every display or window change.

### 6. Clipboard direction

- Sill: text both ways, up to 256 KB. It never reads the device clipboard on its own. "Paste in Sill" is explicit (HIGH, primary README).
- Chrome Remote Desktop: one "Enable clipboard synchronization" toggle, both ways, plus separate file upload and download (LOW, third-party guides).
- RustDesk and AnyDesk: per-session permission for clipboard, separate from files [PRIOR].
- Browser constraints (MEDIUM, WebKit and MDN agree): Safari rejects programmatic clipboard reads outside a user gesture and shows a "Paste" bubble on each read. Safari has no clipboard permission model.
- **Recommendation:** text only. Host to viewer syncs automatically, driven by NSPasteboard changeCount. Viewer to host happens **on an explicit paste gesture** (Cmd+V keydown, or a Paste button on iPhone) that sets the host pasteboard and then forwards the key. Cap the size. Skip pasteboard items flagged concealed or transient [PRIOR; nspasteboard.org convention]. Clipboard is off by default on click-to-share links (a colleague should not receive what you last copied).

### 7. Audio default

- Parsec: guests hear the host. The host's local speakers keep playing when guests connect (MEDIUM, support snippet).
- Sill: sound on by default, captures the streamed window's app audio or all app audio for Desktop. Played slightly behind video. Marked beta (HIGH).
- Browser reality (MEDIUM, WebKit bug threads): iOS Safari needs a user gesture to start audio, and a gesture before the WebRTC connection exists may not count. Use a **"tap to unmute" overlay**.
- **Recommendation:** negotiate the audio track always. The viewer **starts muted** with a visible unmute control. The host picker has a "Share computer audio" checkbox, default on for own-account unattended and off for click-to-share links. Windows share captures that app's audio only. No microphone and no two-way voice.

### 8. Lock screen and unlock (the hard one)

What the evidence says about the macOS login window and lock screen:

| Fact | Source | Confidence |
|------|--------|------------|
| ScreenCaptureKit is **not available before login**, so a logged-out or rebooted Mac cannot be viewed. Apple DTS called this "the biggest issue" and recommended a bug report. | Apple Developer Forums thread 756908 | MEDIUM (Apple staff statement) |
| In a logged-in session that is locked, DTS "expects" ScreenCaptureKit to work, but warned the lock-screen UI has special security restrictions | Same thread | LOW (expectation, not a guarantee) |
| Parsec and RustDesk both ship **separate login-screen access** (Parsec needs a different installer with extra requirements). RustDesk notes the screen stays black before login unless its service runs pre-login. | Parsec support (snippet), ghacks RustDesk guide | MEDIUM |
| CGEvent injection works at the lock screen of a logged-in session but fails at the full pre-login window. Posters report "prohibited" and pre-login failures. | Apple forum 62687, Stack Overflow summary | LOW |
| RustDesk could not type a password on a locked Mac. The fix (modifier reset) landed May 2025 in 1.4.0. A user first had to type one character physically. | GitHub issue 11802 | MEDIUM |
| Chrome Remote Desktop on Mac: reports conflict. One says it reaches a Mac with no user logged in. Another says it sits on "connecting" until someone logs in. After login, one user reported the screen re-locking with "Your screen is being observed". | Apple Communities, Zoho forum | LOW (anecdotes conflict) |
| Jump Desktop Fluid supports logging in with no user logged in. It will not log in to an account with no password. Screen locking on disconnect is limited and RDP/VNC are the recommended paths. | Jump support (search snippets) | LOW |
| Sill documents no lock-screen behavior. Mac must be awake. | Primary README | HIGH (absence) |

**Classification for Portty:**
- "Remote unlock" means **view the lock window, then type the password with the normal keyboard path**. It needs no new unlock feature if capture and input work on the lock window of a logged-in locked session. Prove this in the Tart VM first.
- Pre-login (after reboot or logout) is **out of scope**: SCK is unavailable, it needs a root LaunchDaemon and admin install on a corporate Mac, and FileVault gates pre-boot anyway.
- A corporate Mac almost certainly auto-locks on a short idle policy [PRIOR]. So "view lock screen and unlock" is table stakes for the unattended mode, not an edge case.

### 9. Display sleep and caffeinate

- Sill: the Mac must be awake with the app open. It may not be able to wake it (HIGH). Parsec: the screen must be on before connecting (MEDIUM). Chrome Remote Desktop guides: change sleep settings so the machine stays reachable (LOW). RustDesk guide: a headless Mac mini needs a dummy HDMI plug so a framebuffer exists (MEDIUM).
- Apple's power-management notes: display-sleep and idle-sleep assertions hold while taken, and **no API can prevent forced sleep** such as lid close or the Apple menu (MEDIUM). `IOPMAssertionDeclareUserActivity` can wake the display. Hammerspoon uses it for that (MEDIUM).
- **Recommendation:** take PreventUserIdleDisplaySleep and PreventUserIdleSystemSleep assertions only while at least one viewer is connected, and release them when the last one leaves. When a viewer connects and the display is asleep, declare user activity to wake it. Show an honest **"Mac is asleep, cannot wake"** state in the sidebar. Lid-close and clamshell sleep is a limit to document, not a bug (it overlaps the deferred M-85 lid-sleep check). Waking a sleeping Mac over the tailnet is an anti-feature.

### 10. What happens on disconnect

- Parsec Mac host locks after the last guest disconnects (MEDIUM). RustDesk and TeamViewer have "lock after session end" options. TeamViewer users complain they cannot disable it (LOW).
- Sill stops capturing when no device is connected (HIGH).
- **Recommendation for Portty:**
  - Viewer gone: release assertions, stop capture and encode when the last viewer leaves, notify the host.
  - Viewer network drop (iPhone sleep): auto-reconnect with an ICE restart, as the terminal viewers already resume with replay offsets. Show "Reconnecting".
  - Sharer stops sharing: viewers see "Sharing ended" and the click-to-share link dies.
  - **Lock on disconnect only if the screen was locked when the session began** (the user unlocked it remotely). This restores prior state and never locks a Mac someone is sitting at. Use Ctrl+Cmd+Q (the Lock Screen shortcut) via the same input path [PRIOR].

### 11. DMG self-install (Sparkle and LetsMove)

| Behavior | Sparkle | LetsMove |
|----------|---------|----------|
| Trigger | Update check | Launch from a non-Applications location, asked at the top of `applicationWillFinishLaunching` (MEDIUM) |
| Mechanism | Separate installer process; downloads move into its own managed directory so attackers cannot tamper; after success signals the progress agent and may relaunch (MEDIUM, Sparkle docs snippet) | Copy the bundle to /Applications, delete or trash the original, spawn a child to relaunch (MEDIUM, secondary blog) |
| Disk image | n/a | Detects running from a disk image, copies, then unmounts (MEDIUM, changelog: 1.2 and 1.9) |
| Writability | Authorization prompt when needed [PRIOR] | Checks `isWritableFileAtPath` and requests elevated privileges (MEDIUM) |
| Trash vs delete | Not found in search | Varies by fork and version (MEDIUM) |
| Opt-out | "Skip" options | "Do Not Move" remembered [PRIOR] |
| Sandbox | n/a | **Does not support sandboxed apps** (MEDIUM). Portty is not sandboxed. |

**Portty-specific recommendations:**
- Trigger the first-launch move **only** when running from a mounted disk image or Downloads/Desktop. Never from /Applications, ~/Applications or a dev tree. Remember "Do not move."
- If /Applications is not writable (a standard, non-admin user on a managed Mac is the likely case, [PRIOR]), offer **~/Applications** rather than an admin password prompt. LetsMove also supports ~/Applications [PRIOR].
- In-app update flow: reuse the existing verification (SHA-256, Team ID, signing identifier, notarization). Then mount with `-nobrowse`, re-verify the mounted app, copy next to the destination, atomic swap, relaunch, detach, delete the downloaded DMG. Keep the old bundle until the new one reports healthy and **roll back on failed launch**, because nobody can log in to fix the remote Mac.
- Ejecting uses the volume under the running bundle's path. Trashing a user-downloaded DMG goes to the Trash (recoverable). The updater's own download is deleted outright. If the DMG sits on a read-only or network volume, skip with a log line rather than failing the install.
- Check for **App Translocation** (a quarantined app run from Downloads executes from a randomized read-only path). Self-replace and DMG-location lookups fail there [PRIOR; LOW]. Needs phase-level research.
- TCC grants (Screen Recording, Accessibility) key on signing identity and bundle identity. They persist across updates only if the signing identity stays the same (MEDIUM, Apple forum: permissions lost when the signing identity changed). Keep the same Developer ID and the same /Applications bundle path.

## Feature Landscape

### Table Stakes (users expect these)

| Feature | Why expected | Complexity | Notes / dependencies on existing Portty |
|---------|--------------|------------|------------------------------------------|
| Live view of a display, with Retina-aware scaling and fit-to-window | Core of every product above | HIGH | Pion in daemon, ScreenCaptureKit, hardware H.264. Normalized coordinates (see section 5). |
| Mouse and keyboard control, key codes plus separate text path | "Full control" is the stated goal | HIGH | Needs Accessibility. Release-all-keys on blur and disconnect. |
| Guided permission onboarding (Screen Recording, Accessibility) with a status check | Every Mac remote tool starts with "waiting for image" until granted (MEDIUM, ghacks) | MEDIUM | macOS 27 adds a separate "Remote Desktop" TCC category with **no silent status API**, and one forum report says a status call prompts every time (LOW, Apple forum 841530, beta). Plan a test-capture probe, not a preflight call. Plain executables may not appear in the permission list on Tahoe 26.1, so the capturing process must live in a signed app bundle (LOW). |
| Display picker, switch displays mid-session without dropping viewers | CRD, Jump and Sill all do it (Sill steps displays, HIGH) | MEDIUM | Unattended mode has no one at the Mac, so the **viewer** must be able to choose display or window. Click-to-share uses the host's pick. Update the capture filter, resend size, force a keyframe. |
| Window share (sharer picks a window) | Explicit scope item | HIGH | Capture is easy. **Input is global**, so a click lands on whatever window is on top. Raise the window on control and clamp to its frame. Sill avoids this with a private virtual display (fragile, README admits macOS updates can break it). Do not copy that. |
| Always-available mode for own tailnet Macs | Explicit scope; the main user story | MEDIUM | Reuses tailnet peer sidebar and the existing "limited to host's Tailscale account" check. Needs a tray indicator, connect/leave notification, and a global on/off. |
| Click-to-share link joinable by other tailnet users | Explicit scope; decision logged 2026-11-24 | MEDIUM | Reuses `capability` tokens and the token-gated web share server. Add host **admit prompt** with tailnet identity, ephemeral and revocable link, and **view-only by default with an "Allow control" checkbox** (recommended). Pattern follows the per-port consent used for the localhost proxy. |
| Multi-viewer (fan-out) | Explicit scope | MEDIUM | One encoder, one track per viewer, keyframe on join. Soft active-controller rule (section 2). |
| Viewers in Portty app, browser link, iPhone | Explicit scope | MEDIUM | Same web viewer code in WKWebView and the share server's browser page. iPhone gets touch gestures (section 3) and Home Screen constraints. |
| Remote view and unlock of the lock window (logged-in, locked) | Corporate auto-lock makes this routine | HIGH, **feasibility unproven** | See section 8. Needs the lock-window capture and input proof in the Tart VM before committing the phase. |
| Display and system wake while viewers are connected | Dead screen means black stream | LOW-MEDIUM | Power assertions plus declare-user-activity on connect. |
| Stuck-key and modifier recovery | Classic remote-desktop bug | LOW | Part of the input path. |
| Disconnect handling and auto-reconnect | Terminal viewers already reconnect; users will expect the same | MEDIUM | ICE restart, "Reconnecting" state, host stops capture when no viewer remains. Reuse the existing reconnect-after-sleep behavior from the iPhone view. |
| In-app update installs, relaunches, deletes the DMG | Explicit scope; Sparkle sets the expectation | MEDIUM | Builds on the existing verify step and **the v1.2 daemon handoff** so sessions survive. Rollback on failed launch. |
| First launch from the mounted DMG moves to /Applications, ejects, trashes the download | Explicit scope; LetsMove sets the expectation | MEDIUM | Writability fallback to ~/Applications; translocation check; handle an existing older install and a running old daemon (handoff plus the DEBT-08 consent). |

### Differentiators

| Feature | Value proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| One app: terminal and desktop, one Share, three viewers, tailnet only | No account, no relay, no Screen Sharing needed on a locked-down corporate Mac | (the product) | Aligns with Core Value. |
| Adaptive quality: sharp when still, smooth when moving | Explicit scope; Parsec favors constant low latency, CRD and RustDesk expose manual quality modes [PRIOR] | HIGH | SCK idle frames give a free "still" signal. Raise bitrate or quality after about 300 to 500 ms of stillness, cap bitrate when moving, congestion feedback via pion interceptors, drop frames rather than queue (Sill's approach, HIGH). Everything else in the milestone works without this, so build last. |
| Tailnet-identity admit and per-viewer list | Admit "alice@corp" rather than an anonymous link holder | MEDIUM | Needs tsnet WhoIs on the share server (design inference; not exercised here). Own-account detection must treat shared-in and tagged nodes as "other". |
| Lock on disconnect only if the session started locked | Restores prior state safely | LOW-MEDIUM | See section 10. |
| Clipboard sync with concealed-item filtering | Safer than naive sync | LOW-MEDIUM | See section 6. |
| System audio, start-muted viewer | Hear video and meeting audio | MEDIUM | SCK audio capture. Autoplay overlay for iOS. |
| iPhone: pinch-zoom to 5x, long-press drag, three-finger Mission Control and Spaces | Sill-grade mobile control | MEDIUM | Gestures send existing Mac keyboard shortcuts. Defer the three-finger set if time is short. |
| Win/Linux viewer Ctrl to Cmd swap | Colleagues on browser links | LOW | Visible toggle. |
| "Send keys" menu | Cmd+Tab, Cmd+Space, lock | LOW | Required anyway for browser viewers. |
| Click Update from the desktop view | A remote user can finish an update on the office Mac | LOW | Falls out of desktop sharing. No extra build. |

### Anti-Features (commonly requested, often problematic)

| Feature | Why requested | Why problematic | Alternative |
|---------|---------------|-----------------|-------------|
| Pre-login / post-reboot / FileVault access | "I rebooted and I'm locked out" | SCK unavailable pre-login (Apple DTS). Needs a root LaunchDaemon and admin install on a managed corporate Mac. FileVault pre-boot is impossible. Parsec and RustDesk each need a separate privileged installer. | Keep the Mac running; document it. Handle only the locked, logged-in case. |
| Stored-password one-click unlock | Convenience | A Keychain-held login password that can open a corporate laptop remotely is a large blast radius. | Type the password over the normal input path; never log keystrokes; never persist it. |
| Privacy / "curtain" mode (blank the host display, block local input) | Hide remote work from people at the Mac | Parsec needs virtual displays (MEDIUM). CRD curtain reportedly unsupported on newer macOS (LOW, conflicting). Needs invasive input blocking. | None. State in the UI that the Mac's screen is visible locally. |
| STUN/TURN/cloud relay | "Works anywhere" | Violates tailnet-only and "no new cloud dependencies". | Restrict ICE to the tailnet interface. Tailscale already provides direct and relayed paths. |
| Wake a sleeping Mac | "It went to sleep" | No API prevents forced sleep. A sleeping Mac runs no daemon. | Show "asleep" state; assertions only while viewers are connected. |
| File transfer and file clipboard | Parity with CRD and AnyDesk | Large new attack surface, and Portty has deliberately removed its file browser. | `scp` over the tailnet from the terminal. |
| Microphone, two-way voice, webcam | Call-app creep | Needs Microphone permission, echo handling, more consent UI. | Use a call app next to it. |
| Session recording | "Audit" or demo | Legal and privacy risk on a corporate Mac. | None. |
| Independent multi-cursor co-control | Collaboration | Large UX and input-injection complexity. | One soft active controller. |
| Private virtual display for window share (Sill's trick) | Window keeps updating when covered | Private APIs, fragile across macOS updates (Sill's README concedes this). | Capture the window as is; raise it on control. |
| Gaming-grade tuning (120 fps, 4:4:4, gamepads) | Parsec niche | Wrong target for office terminals and apps. | Cap about 60 fps; keep the sharp-when-still goal. |
| Silent update install while viewers are connected | "Always current" | Would drop an active desktop session unattended. | Consent first, or install when no viewers are connected. |
| Desktop chat | Habit | Session chat was removed on purpose (2026-11-15). | None. |

## Feature Dependencies

```
Screen Recording permission + signed stable bundle identity
    └──requires──> ScreenCaptureKit capture ──requires──> H.264 encode ──requires──> pion WebRTC (tailnet-only ICE)
                                                                  └──enables──> Multi-viewer fan-out
                                                                  └──enables──> Adaptive quality (last)

Accessibility permission ──requires──> Input injection (mouse/keyboard/text)
    └──requires──> Stuck-key recovery
    └──enables──> Lock-window unlock (also needs lock-window capture; feasibility unproven)
    └──enables──> "Send keys" menu, lock on disconnect

Display/window picker ──requires──> Capture filter update + size message + keyframe (no renegotiation)
    └──requires──> Window share input clamping

Existing tailnet peer sidebar ──enables──> Always-available mode ──requires──> own-account check (existing MOB-05 mechanism)
Existing token share server + capability ──enables──> Click-to-share link ──requires──> admit prompt + revocation + view-only default

Power assertions ──enables──> unattended usefulness (wake display, no idle sleep while viewers)
Existing reconnect-after-sleep (iPhone) ──enables──> Disconnect/auto-reconnect

Existing updater verify + v1.2 daemon handoff ──requires──> DMG in-app install ──enables──> first-launch move (shares copy/swap code)
```

### Dependency notes

- **Capture and TCC must be in a signed app bundle.** On Tahoe 26.1, plain executables reportedly do not appear in the Screen Recording list (LOW). Permission grants also reset when the signing identity changes (MEDIUM). This constrains where the pion and capture code runs relative to the Portty.app bundle. It is the single biggest architectural dependency in this milestone, so it belongs in ARCHITECTURE research.
- **Monthly re-confirm on Sequoia** (Allow For One Month) can silently break an unattended Mac, because no one is there to click it (MEDIUM, many outlets). Apple offers a `persistent-content-capture` entitlement aimed at VNC apps, through a request form. Whether it removes the prompt is unconfirmed (LOW). Detect "capture stopped producing frames", notify the user, and treat the entitlement request as a phase-level research item.
- **Always-available needs a trust test.** Own-account is decided by tailnet identity (WhoIs on the share server). Shared-in and tagged nodes must not count as "own".
- **Click-to-share widens a security boundary** (decision 2026-11-24). Admit prompt, expiry, revoke, and view-only default keep that widening reviewable.
- **DMG install must not break session survival.** The new binary has to take over via the v1.2 handoff, and the DEBT-08 consent applies to a version-mismatch restart.
- **Adaptive quality conflicts with schedule.** It touches the encoder, bitrate control and feedback. Everything else works at fixed quality, so keep it last.

## MVP Definition

All items below are in the user-confirmed scope. This section recommends ordering and where the riskiest items sit, not cuts.

### Launch With (v2.0 core)

- [ ] Permission onboarding and status via a test capture
- [ ] Display capture, hardware H.264, Retina-aware scaling, viewer in the Portty app
- [ ] Mouse and keyboard control with key codes, text path, stuck-key recovery
- [ ] Always-available mode for own tailnet Macs, with tray state and notifications
- [ ] Power assertions while viewers are connected, display wake
- [ ] Browser viewer and iPhone viewer (trackpad mode plus pinch-zoom plus key bar)
- [ ] Disconnect handling and auto-reconnect
- [ ] Display switch mid-session
- [ ] DMG in-app install with rollback, and first-launch move, eject, trash (self-contained, can run in parallel with the capture work)

### Add After Core Works (v2.0 later phases)

- [ ] Click-to-share link, admit prompt, view-only default
- [ ] Multi-viewer with soft active-controller
- [ ] Clipboard sync (text, gesture-based paste, concealed filter)
- [ ] Audio with start-muted overlay
- [ ] Window share, with input raising and clamping
- [ ] Lock-window unlock (gate on the VM feasibility proof) and lock-on-disconnect-if-started-locked
- [ ] Ctrl to Cmd swap, "Send keys" menu

### Last

- [ ] Adaptive quality (sharp when still, smooth when moving)
- [ ] iPhone three-finger and edge gestures, direct-touch toggle

### Not in this milestone (see Anti-Features)

Pre-login access, stored-password unlock, curtain mode, file transfer, microphone and voice, recording, TURN and STUN.

## Feature Prioritization Matrix

| Feature | User value | Implementation cost | Priority |
|---------|------------|---------------------|----------|
| Capture + encode + view + control (one display) | HIGH | HIGH | P1 |
| Permission onboarding with probe | HIGH | MEDIUM | P1 |
| Always-available mode and indicator | HIGH | MEDIUM | P1 |
| Power assertions and display wake | HIGH | LOW | P1 |
| DMG in-app install with rollback | HIGH | MEDIUM | P1 |
| First-launch move, eject, trash | MEDIUM | MEDIUM | P1 |
| Lock-window unlock | HIGH | HIGH (unproven) | P1 feasibility gate, P2 build |
| iPhone viewer, trackpad mode, zoom | HIGH | MEDIUM | P1 |
| Click-to-share with admit and view-only default | HIGH | MEDIUM | P2 |
| Multi-viewer and active controller | MEDIUM | MEDIUM | P2 |
| Display switch | MEDIUM | MEDIUM | P2 |
| Clipboard sync | MEDIUM | LOW-MEDIUM | P2 |
| Audio | MEDIUM | MEDIUM | P2 |
| Window share with input | MEDIUM | HIGH | P2 |
| Adaptive quality | MEDIUM | HIGH | P3 (in scope, build last) |
| Three-finger and edge gestures | LOW | MEDIUM | P3 |

## Competitor Feature Analysis

| Feature | Parsec | RustDesk | Chrome Remote Desktop | Sill | Our approach |
|---------|--------|----------|-----------------------|------|--------------|
| Mac permissions | Screen Recording plus Accessibility, restart after grant | Accessibility, Screen Recording, sometimes Input Monitoring | Accessibility plus Screen Recording for the host helper | Screen Recording, Accessibility, Local Network | Same two, probe by test capture |
| Login-screen access | Separate installer | Service must run pre-login | Conflicting reports | Not covered | Out of scope |
| Lock on disconnect | Automatic after last guest | Optional | Not found | Not covered | Only if session started locked |
| Multi-viewer | Guests with permissions, Accept/Kick | Per-connection permissions | One at a time [PRIOR] | One device | Soft active controller |
| Clipboard | Not covered in sources | Both ways, per-session permission | Toggle, files separate | Text both ways, 256 KB, explicit paste | Text, gesture-based paste, concealed filter |
| Audio | Host audio to guests | Optional [PRIOR] | Unverified on Mac | On by default, beta | Start-muted viewer |
| Quality | Constant low latency | Manual modes | Adaptive [PRIOR] | Drop frames, hint lower quality | Sharp-when-still plus bitrate cap |
| Touch (iPhone) | n/a | n/a | n/a | Trackpad, hold-drag, pinch to 5x, 3-finger shortcuts | Same vocabulary, trackpad first |
| Transport | Cloud-assisted P2P | Relay or P2P | Google relay | Direct TLS or your VPN | WebRTC over tailnet only |

## Sources

- Apple Developer Forums, "New security feature prevents remote access apps from working" (pre-login SCK, locked screen, DTS replies): https://developer.apple.com/forums/thread/756908 (MEDIUM)
- Apple Developer Forums, macOS 27 beta "Remote Desktop" TCC category: https://developer.apple.com/forums/thread/841530 (LOW, beta)
- Apple Developer Forums, pre-login CGEvent injection: https://developer.apple.com/forums/thread/62687 (LOW)
- Apple Developer Forums, Tahoe 26.1 plain executables missing from Screen Recording list: https://developer.apple.com/forums/thread/807898 and https://developer.apple.com/forums/thread/807323 (LOW)
- Apple Developer Forums, SCK permissions tied to signing identity: https://developer.apple.com/forums/thread/819406 (LOW)
- 9to5Mac, MacRumors, TidBits, Michael Tsai on Sequoia monthly prompts and the persistent-content-capture entitlement: https://9to5mac.com/2024/08/14/macos-sequoia-screen-recording-prompt-monthly/ , https://tidbits.com/2024/09/23/how-to-avoid-sequoias-repetitive-screen-recording-permissions-prompts/ , https://mjtsai.com/blog/2024/08/08/sequoia-screen-recording-prompts-and-the-persistent-content-capture-entitlement/ (MEDIUM)
- RustDesk macOS docs: https://rustdesk.com/docs/en/client/mac/ ; lock-screen password issue 11802: https://github.com/rustdesk/rustdesk/issues/11802 (MEDIUM) ; ghacks guide: https://www.ghacks.net/2024/10/17/how-to-fix-rustdesk-not-working-on-macos/ (MEDIUM)
- Sill README (primary): https://raw.githubusercontent.com/Saffsanity/sill/main/README.md (HIGH for Sill's own behavior)
- Screens (Edovia) cursor modes and gestures: https://help.edovia.com/en/screens-5/features/cursor-control-modes-and-other-gestures (MEDIUM)
- Parsec support, macOS app, login screen, host behavior (pages returned 403 to direct fetch, so only search snippets were used): https://support.parsec.app/hc/en-us/articles/32381394408596-Parsec-App-for-macOS and https://support.parsec.app/hc/en-us/articles/32381618319124-Access-Login-Screen-on-macOS (LOW to MEDIUM)
- Jump Desktop support (Fluid, lock screen, gestures; direct fetch returned 403, so search snippets only): https://support.jumpdesktop.com/hc/en-us (LOW)
- Chrome Keyboard Lock API: https://developer.chrome.com/docs/capabilities/web-apis/keyboard-lock and https://developer.mozilla.org/docs/Web/API/Keyboard/lock (MEDIUM)
- WebKit and Safari autoplay and clipboard behavior: https://webrtchacks.com/autoplay-restrictions-and-webrtc/ , https://developer.mozilla.org/en-us/docs/web/api/clipboard_api (MEDIUM)
- Apple IOKit power management release notes (no API for forced sleep): https://developer.apple.com/library/archive/releasenotes/Darwin/RN-IOKitPowerManagment/ (MEDIUM)
- LetsMove README and changelog; Sparkle installer docs: https://github.com/MacPass/LetsMove , https://christiantietze.de/posts/2020/01/appmover-swift/ (MEDIUM)
- Chrome Remote Desktop on macOS guide and forum reports: https://answers.uillinois.edu/161455 , https://discussions.apple.com/thread/255553632 (LOW)
- Apple Mac Help, Screen Sharing setting: https://support.apple.com/guide/mac-help/mh11848 (MEDIUM for the setting's existence only)

---
*Feature research for: Portty v2.0 Desktop Sharing and DMG self-install*
*Researched: 2026-11-24*
