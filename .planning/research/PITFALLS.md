# Pitfalls Research

**Domain:** Adding WebRTC desktop sharing (ScreenCaptureKit, VideoToolbox H.264, CGEvent input, clipboard, audio, multi-viewer, remote unlock) and DMG self-install to an existing macOS Go daemon + Wails v2 GUI app (milestone v2.0)
**Researched:** 2026-11-24
**Confidence:** MEDIUM overall. Apple DTS statements and Apple Developer Forums threads give HIGH confidence on the permission, lock-screen and code-signing facts they cover. Several macOS 26/27 behaviours come from beta-era forum threads and vendor docs (MEDIUM-LOW) and MUST be re-verified on the release macOS in a Tart VM. Anything tagged `[spike]` is a question research cannot answer from a desk.

Confidence tags used below: **[H]** Apple DTS or Apple docs. **[M]** consistent third-party or forum evidence, or established engineering practice. **[L]** single anecdote or inference. Tiers were assigned by judgement. The `classify-confidence` seam was not run because only built-in WebSearch and WebFetch were available.

---

## Read This First: Facts That Change the Plan

### F1. The repo does not use tsnet (the milestone brief assumes it does)

`grep` finds no `tsnet` import. Portty talks to the **system `tailscaled` through LocalAPI** (`internal/tailnet`, `lc.GetCertificate`) and binds the share server with `net.Listen("tcp", BindIP:port)`, guarded by `CheckListener` / `assertTailnetBindIP`. The daemon dials peers through the OS network. So:
- "WebRTC over tsnet userspace networking" is **not** the risk. pion will run on the kernel network stack and see the `utun` Tailscale interface plus Wi-Fi/LAN interfaces.
- The real risks are: (a) pion gathers candidates and binds UDP on **every** interface unless restricted, which breaks the "tailnet-only" trust boundary; (b) the viewer's browser must put the tailnet interface into its candidate list; (c) UDP sockets are not covered by the v1.2 fd handoff.
- If the office Mac ever runs Tailscale in userspace-networking mode, a kernel-bound pion socket will not reach the tailnet at all. Detect and refuse with a clear message.

### F2. The "always-available unattended" Mac is a locked, display-asleep Mac most of the time

A corporate MDM Mac normally enforces screen-saver / display-sleep / lock timeouts. After those fire, three things hold:
1. ScreenCaptureKit cannot capture while the display is asleep (Apple DTS: "still impossible" [H], thread 827059).
2. The wake lands on the lock screen.
3. A lid-closed MacBook Air with no external display sleeps entirely. Nothing in user space runs, so the terminal sessions and Portty are unreachable too. [M]

So remote unlock is the **default case** for unattended mode, not an edge case. If unlock proves infeasible, "always-available" really means "available while the Mac is awake and unlocked". That changes the product claim. **The lock-screen spike must come first, before UI or share-mode work.**

### F3. Hard limits (cannot be engineered around in user space)

| Limit | Evidence | Consequence |
|-------|----------|-------------|
| No monthly-consent exemption without Apple's restricted entitlement `com.apple.developer.persistent-content-capture` | MacRumors/9to5Mac: monthly prompt, "no way to grant permanent access"; entitlement is request-gated and reported 2+ months pending [M] | Plan for a monthly re-consent, apply for the entitlement now, treat it as upside |
| macOS 27 betas show a separate "Remote Desktop" privacy category; `CGPreflightScreenCaptureAccess()` does not reflect it and `SCShareableContent` shows an alert on every probe | Apple forum 841530 (Quinn: "no API that specifically returns the state") [H for "no API", L for release behaviour] | Cannot silently health-check the grant on macOS 27 |
| Pre-login / logged-out / FileVault pre-boot: needs a root pre-login agent. Apple says ScreenCaptureKit works there on 14.4+, but Sequoia reports say it fails. CGEvent taps do not work pre-login (rdar 5636091) and the workaround is DTS-incident only | Forum 814152, 768146, 726470 [H/M] | Out of scope. A reboot or logout can never be recovered remotely by Portty. |
| ScreenCaptureKit in a daemon or a process without a window-server connection does not work | Quinn: "Daemons should not be messing around with GUI stuff, and that includes ScreenCaptureKit" [H] | Capture and input must live in a process in the user's Aqua session |
| Lid closed, no external display, no AC: the Mac sleeps | Apple clamshell requirements (display + power + input) [M] | "Always-available" cannot hold for a closed MacBook Air. Document it. |
| An overwritten running Mach-O or a modified bundle breaks code-signature validation | Standard code-signing behaviour [M] | Update by building a new bundle and atomically swapping, never copying over |

### F4. Remote unlock verdict (feasibility): MAYBE, narrowly, and only for "locked, user still logged in"

| Scenario | Capture | Typing the password | Needs root daemon? | Verdict |
|----------|---------|---------------------|--------------------|---------|
| Screen locked, user session alive (the realistic case) | Apple DTS: ScreenCaptureKit "should work in GUI login sessions, but the unlock UI has special security restrictions that may interfere" [H, 756908] | Stack Overflow report: `CGEventPost(kCGHIDEventTap)` works on the lock screen but not at the pre-login window [L]. Accessibility is required. | No. An in-session process with Screen Recording and Accessibility. | **MAYBE `[spike]`**, provable in a Tart VM |
| Logged out / after reboot / FileVault pre-boot | Pre-login agent running as root; Sequoia regression reports [M] | Fails by default (rdar 5636091), DTS incident for the workaround [H] | Yes (root pre-login agent plus daemon) | **NO** for Portty |
| Corporate lock UI (Jamf Connect, Platform SSO, `SFAuthorizationPluginView`) | The lock UI is a plug-in. A macOS 27 beta regression showed a black screen with `screenUnlockMode=2`, fixed in beta 5, still broken with PSSO [M, forum 838847] | Synthetic events into a custom plug-in UI are unproven | n/a | **Unknown on the office Mac**. Check whether it uses a custom lock plug-in. |

What other tools do:
- RustDesk: login-window access requires its installed service/helper; without it the login screen is black [M, RustDesk docs and blog]. It needs Screen Recording plus Accessibility, and Input Monitoring on newer macOS.
- Chrome Remote Desktop: the host shows offline when nobody is logged in, and Google calls the login screen "not a security boundary" [L/M].
- Jump Desktop and Apple Screen Sharing: not found documented. TeamViewer and AnyDesk reportedly use a LoginWindow-session LaunchAgent (forum poster's claim, unconfirmed by Apple) [L].
- Apple's recommended architecture [H]: a **daemon** for global tasks (network) and an **agent** for GUI work (capture, input), joined by XPC. For pre-login the agent has `LimitLoadToSession` = Aqua + LoginWindow and runs as root. Not realistic on an MDM Mac where the user cannot install root daemons.

Recommended scope: ship unlock as "type your password through the viewer into the lock screen of a still-logged-in session". Never store the macOS password and never keep an auto-unlock credential. Gate it behind the early spike. If the spike fails, cut it and say "unattended works while awake".

---

## Critical Pitfalls

### Pitfall 1: Screen Recording / Accessibility granted to the wrong binary (TCC attribution)

**What goes wrong:** The grant is requested for, or attributed to, a different process than the one that captures. Examples: the daemon respawned by `portty` over SSH or from Terminal is attributed to `sshd`/Terminal; a bare executable never appears in the Screen Recording list (Tahoe 26.1 reports: non-bundled executables get the prompt and capture works, but are not listed, so the user cannot revoke or re-add) [M, forum 807898]; macOS 27.3 beta reports `CGRequestScreenCaptureAccess()` showing no prompt and no list entry, so the user must press "+" and add the app by hand [M, forum 858698].
**Why it happens:** The daemon is today the same `Portty` executable started with `exec.Command(exe, "daemon")` and `Setsid`, from the GUI poller, the CLI, or an opt-in launchd agent. Three launch paths mean three possible "responsible processes". TCC attributes a child to its responsible process, so the app bundle is attributed when the GUI spawned it but Terminal or sshd when the CLI did. `[spike]`
**How to avoid:**
- Pick ONE capture host with a fixed identity: either the GUI app process, or a helper inside `Portty.app/Contents/MacOS` started by LaunchServices or `SMAppService` (never `exec` from a CLI/SSH context). Make the daemon an orchestrator (WebRTC signaling, tokens) that asks the capture host over a Unix socket. This is also Apple's recommended split.
- If a child must be spawned from ambiguous contexts, test `responsibility_spawnattrs_setdisclaim` (private, used by Chromium and Firefox). Do not make it the plan.
- Ship an onboarding panel that deep-links to Privacy_ScreenCapture and Privacy_Accessibility and shows "not in the list? click + and choose Portty.app".
- A grant usually takes effect only after the process restarts. Use the existing v1.2 takeover handoff to restart the capture host without losing sessions, never kill it.
**Warning signs:** `SCShareableContent` returns "user declined TCCs" while System Settings shows the toggle on; the prompt names "Terminal" or "sshd-keygen-wrapper"; capture works from the GUI but not after a CLI-spawned daemon respawn.
**Phase to address:** P2 (spike with all three launch paths in a Tart VM before any pion work).

---

### Pitfall 2: Grants reset on every update (unstable code identity)

**What goes wrong:** After an update the user is re-prompted or capture silently fails, and nobody is at the office Mac.
**Why it happens:** TCC tracks identity by **designated requirement**. Ad-hoc code has no stable DR, so a new build looks like a different app and the grant is not carried over [H, Quinn on forum 795739]. Release builds are Developer ID signed (stable DR from Team ID plus bundle ID), but the Tart test flow uses "ad-hoc or notarized builds installed with `rm -rf` and `ditto`" (TESTING.md), and the helper or daemon may be signed differently from the app. Release signing uses `codesign --deep --force` with one entitlements file, so a nested helper inherits the same entitlements and identity, but a helper added later with a different identifier changes what TCC sees.
**How to avoid:**
- Keep every Mach-O in the bundle signed with the same Developer ID and a stable `--identifier`. Do not rename the bundle ID or helper identifier between releases.
- In the VM, sign test builds with a **stable self-signed code-signing certificate** (the DR is leaf-cert based and survives rebuilds) so grants persist. Otherwise every build re-prompts and the TCC DB cannot be edited with SIP on.
- Add a release-time check that dumps `codesign -d -r-` for the app and every nested executable and diffs against the previous release's DR.
**Warning signs:** Screen Recording listed but capture denied after an update; the same app appearing twice in the list.
**Phase to address:** P2 (test-build signing), P1 (updater verifies the DR matches before swapping), P7 (release gate).

---

### Pitfall 3: Monthly Screen Recording re-consent kills unattended capture, and you cannot see the prompt

**What goes wrong:** Capture stops roughly monthly. The system alert ("bypass the system private window picker / Allow For One Month") appears on the office Mac's screen, and capture is blocked while it shows. The remote viewer sees a black or empty stream, and cannot click the prompt it cannot see.
**Why it happens:** macOS 15 added a recurring reminder for apps that capture without `SCContentSharingPicker`; it was weekly in betas, monthly in release, and reduced in 15.1 for regularly used apps. There is no permanent opt-out [M]. Apple's answer for VNC-style products is the restricted entitlement `com.apple.developer.persistent-content-capture` (macOS 14.4+), granted through a request form. Developers report requests pending 2+ months with no word [M, forums 770675, 816646]. Enterprises cannot pre-grant it through MDM yet [M, forum 831795]. macOS 27 betas add a separate "Remote Desktop" category [L].
**How to avoid:**
- **Start the entitlement request at milestone start**, in parallel, as a non-blocking upside. If granted it adds a provisioning profile to the signing pipeline: embed the profile, sign with the restricted entitlement, and re-run notarization. Budget that as its own plan.
- Design for the monthly prompt even if the entitlement arrives: a viewer state "Host is waiting for Screen Recording approval at the Mac", a host-side native notification, and no black video. Detect "SCStream started but zero frames and display awake" as this state.
- Do not poll for permission health on macOS 27 (each `SCShareableContent` probe can raise an alert). Probe only after a user action, cache the result, and never loop.
- Tell the user plainly in docs: once a month someone must be at the office Mac.
**Warning signs:** Frames stop at day ~30; the stream starts but delivers zero frames; first-launch onboarding differs between macOS 15, 26 and 27.
**Phase to address:** P2 (detection, onboarding, notification), P5 (viewer state), P7 (entitlement packaging if granted).

---

### Pitfall 4: Capture/input hosted in the wrong process context (daemon, SSH session, no WindowServer)

**What goes wrong:** ScreenCaptureKit returns nothing, or `CGEventPost` is dropped, because the process has no window-server connection (a daemon, or a process spawned from an SSH session that runs in a Background bootstrap session).
**Why it happens:** `portty daemon` is spawned detached with `Setsid` from whatever invoked it. When no GUI is running (the interesting unattended case), the respawn comes from the CLI, possibly over SSH. Quinn's guidance is explicit that GUI work belongs in an agent in the Aqua session [H]. Also, `Wails` toggles `NSApplicationActivationPolicy` (LSUIElement plus Accessory), and an Accessory app may not surface TCC prompts in front.
**How to avoid:**
- The capture host must be started in the Aqua session: the GUI app itself (login item via `SMAppService.mainApp`, which a Portty user must enable for unattended mode), or an `SMAppService.agent` plist in the bundle. The daemon never links ScreenCaptureKit or posts CGEvents.
- Make unattended mode explicit: "Start Portty at login" is a prerequisite shown in the share UI. If the GUI is not running, the daemon reports "desktop sharing unavailable: Portty app not running".
- Do not let a CLI-over-SSH respawn start a capture-capable process.
**Warning signs:** works when Portty launched from Finder, fails after `portty` respawned the daemon over SSH; `launchctl managername` prints `Background`.
**Phase to address:** P2.

---

### Pitfall 5: Display asleep, lid closed, no display, lock: frames stop and nobody says why

**What goes wrong:**
- Display sleeps: `SCStream` delivers nothing; `SCScreenshotManager` fails [H, forum 827059].
- Lid closed on a MacBook Air without a display: the whole Mac sleeps, so no capture and no terminal either.
- External display unplugged/replugged or clamshell docking: the display list changes and the stream errors.
- ScreenCaptureKit delivers frames **only on change** (`idle` status). A static desktop yields no frames, so a new viewer sees nothing until something moves, and jitter-buffer "frozen video" logic misfires.
**How to avoid:**
- On viewer connect, wake the display with a user-activity power assertion (`IOPMAssertionDeclareUserActivity`, the `caffeinate -u` mechanism) `[spike]`, and hold a prevent-display-sleep assertion only while at least one viewer is connected. Release it when the last viewer leaves, since the office Mac's idle policy is the user's.
- Never promise lid-closed access. Say so in docs and in the host-status line.
- Cache the last frame. On each new viewer or PLI, re-submit it to the encoder to force an IDR. Send a periodic refresh frame (for example every 2 s idle) so Safari does not mark the stream stalled.
- Subscribe to `CGDisplayRegisterReconfigurationCallback` and rebuild the stream on change. Surface "no display attached" as a state.
- Mac mini/headless (not the target): a zero-display `SCShareableContent` needs a dummy plug or a virtual display. Out of scope unless the user adds such a Mac.
**Warning signs:** viewer black after the Mac idles; works while you sit at the Mac; stream drops after docking.
**Phase to address:** P2 (spike), P5 (display reconfiguration and idle refresh).

---

### Pitfall 6: pion binds and gathers on every interface; tailnet-only trust boundary leaks

**What goes wrong:** pion's default gathers host candidates from all interfaces and listens on `0.0.0.0` UDP ephemeral ports. The Mac then exposes an ICE/DTLS endpoint on the office LAN and Wi-Fi, outside the "tailnet-only, token-gated" promise that the TCP share server upholds with `CheckListener`.
**How to avoid:**
- Configure `webrtc.SettingEngine` explicitly: `SetInterfaceFilter` + `SetIPFilter` to the Tailscale `utun` and CGNAT/ULA ranges (100.64.0.0/10, fd7a:115c:a1e0::/48), `SetNetworkTypes` (UDP only), `SetICEMulticastDNSMode(Disabled)`, and a fixed UDP range or `SetICEUDPMux` on one socket bound to the tailnet IP. Add an `assertTailnetBindIP` analogue plus a test that no UDP socket is bound to 0.0.0.0 or a LAN address.
- Pass empty `ICEServers` on both sides. Add a test that the PeerConnection never sends to a public IP (no STUN/TURN, no public mDNS).
- Reuse the capability token for signaling and bind the DTLS fingerprint to the token so a port scanner cannot complete a session.
- Keep a `vmpreview`-tagged override for the interface filter (the repo already uses `*_vmpreview.go`) so Tart VM-to-VM tests can use the VM LAN while production stays tailnet-only.
**Warning signs:** `lsof -i UDP` shows the Portty process on `*:port` or 192.168.x.
**Phase to address:** P3.

---

### Pitfall 7: Viewer cannot connect: candidate gaps, mDNS, DERP, MTU

**What goes wrong:**
- Browsers hide host IPs behind mDNS names (`xxxx.local`) unless capture permission was granted. Over a tailnet there is no multicast, so the Mac cannot resolve them.
- The viewer may not offer a candidate on its `utun` interface. Under restrictive IP-handling policies (a "default public interface only" policy), the browser uses only the default-route interface. A packet sourced from the Wi-Fi IP toward 100.x is dropped by WireGuard cryptokey routing. `[spike]` for Safari, iOS Safari and WKWebView.
- If Tailscale falls back to DERP, UDP media runs over a relay. Tailscale says DERP is not designed for media [M] and throughput and latency fall off a cliff.
- Tailscale MTU is 1280. Anything above about 1230-1250 UDP payload fragments or drops. pion's default RTP MTU of 1200 plus SRTP overhead fits; do not raise it. Keep DTLS and SCTP at their 1200 defaults. Do not enable jumbo SCTP messages for clipboard payloads over the data channel (chunk them).
**How to avoid:**
- Make the Mac ICE-lite (`SetLite(true)`) advertising only its tailnet IP:port host candidate. Browsers (full ICE) then connect to it and the Mac learns the viewer's peer-reflexive address, so mDNS names never need resolving. Wait for gathering before answering (non-trickle) to keep signaling one round trip.
- Plan B transport if a viewer class cannot produce a working candidate: H.264 over the existing token-gated WSS plus WebCodecs `VideoDecoder`. It rides the TCP path that already works for every viewer. Keep the capture/encode pipeline transport-agnostic so this is a swap.
- Show path quality in the UI: read connection type (direct vs relay) from LocalAPI status and warn "Relayed connection: video will be poor".
- Test the join matrix: Chrome, Safari macOS, iOS Safari, Wails WKWebView, on-LAN and off-LAN.
**Warning signs:** ICE stuck in `checking`; works from Chrome on the same Mac but not from the iPhone; works on LAN, fails from home.
**Phase to address:** P3 (spike the join matrix before building viewer UI).

---

### Pitfall 8: H.264 that VideoToolbox emits is not what pion or Safari expects

**What goes wrong:** Black video, frozen first frame, or a Safari decode failure.
**Why it happens:**
- VideoToolbox outputs **AVCC** (length-prefixed). pion's H.264 payloader expects **Annex B** with SPS/PPS before each IDR. Forgetting the conversion, or sending SPS/PPS only once, means late joiners never decode.
- Profile negotiation: Safari and WKWebView require the SDP `profile-level-id` and `packetization-mode=1` to match what is encoded. A negotiated Constrained Baseline `42e01f` carrying High-profile bitstream fails on WebKit [M, WebKit bug 195124; Wowza thread]. Level must cover the resolution: a 2560x1600 Retina capture needs Level 5.0+, not 3.1.
- B-frames and reordering add latency and break strict decoders. VideoToolbox defaults may enable them.
- Setting `RequireHardwareAcceleratedVideoEncoder` fails in a macOS VM (no media engine) and on some machines; use "enable", not "require".
- Color: untagged BT.601/709 produces washed-out colors in Safari.
**How to avoid:**
- Register the codec in pion's `MediaEngine` explicitly with the exact fmtp the encoder emits (profile-level-id derived from the real SPS, `packetization-mode=1`, `level-asymmetry-allowed=1`). Prefer Constrained Baseline or Main at a level derived from the capture size.
- VideoToolbox settings: `RealTime=true`, `AllowFrameReordering=false`, `ProfileLevel` set explicitly, `MaxKeyFrameInterval` of a few seconds, explicit color-primaries/transfer/matrix tags (BT.709), bitrate via `AverageBitRate` plus `DataRateLimits`.
- Unit-test the Annex B converter with golden bitstreams; test the SDP against the real SPS at runtime and log a mismatch.
- Downscale Retina captures (`SCStreamConfiguration.width/height`) to a sane maximum (for example 1920 px wide) for bandwidth and decoder level.
**Warning signs:** works in Chrome, black in Safari; first frame only; viewer needs a screen change to start; profile in `chrome://webrtc-internals` differs from SDP.
**Phase to address:** P2 (encoder output), P3 (SDP, viewers).

---

### Pitfall 9: Multi-viewer fan-out and keyframe storms

**What goes wrong:** One encoder feeds N viewers. Each viewer's PLI/FIR triggers an IDR, so three viewers that join together or lose a packet produce an IDR storm (huge bitrate spikes, stutter for everyone). One slow viewer (iPhone on cellular via Tailscale DERP) forces quality down for all, or is starved.
**How to avoid:**
- Coalesce keyframe requests: at most one IDR per 500-1000 ms regardless of how many viewers asked. Read RTCP via `RTPSender.ReadRTCP` and route PLI/FIR into one rate-limited `RequestKeyframe()`. Use `kVTEncodeFrameOptionKey_ForceKeyFrame` on the next frame; on a static screen re-encode the cached frame (Pitfall 5).
- One encoder, one `TrackLocalStaticSample`, many PeerConnections (pion fans out). Drive the bitrate from the minimum of per-peer estimates, or cap the viewer count. A second low-bitrate encoder tier is a later optimisation, not v2.0.
- Use pion's congestion-control interceptor (TWCC/GCC) per PeerConnection for estimates, and apply hysteresis so bitrate does not flap.
- Per-viewer teardown must not touch the shared encoder: close the PeerConnection, decrement a refcount, stop the encoder and the SCStream when it reaches zero.
- Use wall-clock sample durations for variable frame rate, or A/V drift appears.
**Warning signs:** CPU spikes on each join; viewer A stutters when viewer B joins.
**Phase to address:** P3 (keyframe coalescing), P5 (multi-viewer, adaptive quality).

---

### Pitfall 10: Wails WKWebView as a WebRTC viewer

**What goes wrong:** The in-app viewer connects but is unusable.
**Why it happens and fixes:**
- **Keyboard capture:** the Wails menu owns Cmd+Q, Cmd+W, Cmd+C, and the system owns Cmd+Tab and Cmd+Space. A viewer forwarding "Cmd+C" gets the menu action instead. Add a "control mode" that disables menu key equivalents while the viewer has focus, plus a modifier bar for combos the OS will not deliver (the iPhone key bar exists and can be reused).
- **Clipboard:** `navigator.clipboard` needs a secure context and user gesture. The Wails page origin (custom scheme) may not qualify. Use Wails `runtime.ClipboardGetText/SetText` from Go for the app viewer and the Clipboard API only on the HTTPS tailnet page. `[spike]`
- **Media permissions:** receive-only WebRTC needs no `getUserMedia`, so Wails' lack of a WKUIDelegate media-capture handler does not matter. WebRTC in WKWebView has a history of prompt and audio regressions [M], so test on the exact macOS used. Host candidates are mDNS-masked without capture permission, so the ICE-lite design in Pitfall 7 is what makes WKWebView work.
- **Autoplay audio:** WebKit blocks unmuted autoplay. Start muted and enable on a click.
- **Reachability:** the WKWebView reaches 127.0.0.1 and the OS network. The existing code notes "the webview can only reach 127.0.0.1:<relayPort>, never the peer, so the daemon must proxy" for the TCP/WSS path. For WebRTC, signaling should go through the daemon proxy (capability stays server-side, Origin handled as in `remote_ws_proxy.go`) while UDP media flows direct from WKWebView to the peer through the system Tailscale. Verify the WKWebView picks the `utun` candidate.
- **CSP:** a `<video>` with `srcObject = MediaStream` is not fetched, so `media-src` should not apply, but the current CSP is `default-src 'none'` and Safari behaviour is `[spike]`. CSP does not govern ICE/UDP traffic at all, so it will not stop a compromised viewer page from exfiltrating through a PeerConnection. Keep `script-src 'self'` strict, add no `blob:` to script/worker, and avoid AudioWorklet and canvas-based decoders, which would need `worker-src`/`blob:`. Signaling uses the existing `connect-src 'self' wss://host`. Add the viewer page to the existing `browser_csp_e2e` tests, assert zero violations, and add a `TestCSPHeaders_*` regression for any delta (the file's history shows every delta was argued and guarded).
**Phase to address:** P3 (viewer, CSP), P4 (clipboard, keys).

---

### Pitfall 11: Input injection that is wrong, stuck, or escapes its sandbox

**What goes wrong:** Stuck modifier keys after a dropped connection; wrong characters from a different keyboard layout; clicks off by 2x on Retina; negative display origins; events leaking outside a shared window.
**How to avoid:**
- Send `KeyboardEvent.code` (physical) for shortcuts and the Unicode string for text (`CGEventKeyboardSetUnicodeString`); map Cmd/Ctrl explicitly per viewer platform.
- **Release all keys and buttons on data-channel close, peer failure, or viewer switch.** Add a dead-man timer.
- Normalize pointer coordinates to the captured display's point space, honouring per-display origin and scale; rebuild the mapping on display-switch or reconfiguration events.
- **Window-share mode:** CGEvent posts are global. Drop pointer events outside the shared window frame, and before keystrokes confirm the frontmost app is the shared window's owner. Otherwise a viewer typing into a "shared terminal window" types into the password manager.
- Rate-limit and size-limit input messages (the existing `input_limit` and bounded-queue patterns from v1.2 are the model).
- Accessibility must be trusted for the process that posts. A distinct `AXIsProcessTrusted` check per process.
**Warning signs:** a key repeats forever after Wi-Fi drops; clicks land in the wrong place after a display change.
**Phase to address:** P4.

---

### Pitfall 12: Clipboard sync triggers privacy prompts and leaks secrets

**What goes wrong:** macOS pasteboard privacy (introduced in 15.4, opt-in via defaults on some versions [L]) shows an "allow paste" alert when an app reads the pasteboard programmatically without a paste gesture. That alert would hang on an unattended Mac. Polling `changeCount` and copying every change ships password-manager contents to every viewer, including click-to-share guests.
**How to avoid:**
- Sync only on an explicit viewer action ("Paste to host", "Copy from host") or on viewer focus, never continuous. Use `NSPasteboard.changeCount` for cheap change detection and the "detect patterns" APIs before reading.
- Mark and respect `org.nspasteboard.ConcealedType` and `TransientType` (password managers). Skip them.
- Cap size (for example 1 MB), text-only first, chunk over the data channel.
- Prevent echo loops: tag outbound clipboard writes with a sequence ID and ignore the resulting change.
- Disable clipboard in click-to-share (guest) mode by default.
**Phase to address:** P4 (basic), P5 (guest-mode policy).

---

### Pitfall 13: Click-to-share for other tailnet people widens the trust model to a whole desktop

**What goes wrong:** MOB-05 limited browser links to the host's Tailscale account. The user decided on 2026-11-24 that click-to-share desktop links may be opened by other people on the tailnet, with full mouse/keyboard control. A leaked link, a wrong tailnet member, or a long-lived token equals a full desktop on the corporate Mac, including the open terminal and any logged-in apps.
**How to avoid:**
- Per-session **host consent** for guests ("Alex (alex@…) wants to view your screen: Allow view / Allow control / Deny") that needs someone at the Mac. Bind the token to the caller's tailnet identity with LocalAPI `WhoIs`, not only to the link.
- Default guests to view-only. Control is a separate, revocable, visible state with a persistent menu-bar indicator and a one-click stop. Short expiry (minutes to hours), single-use join.
- Unattended mode is **only** for the host's own tailnet identity (reuse the existing account-limit check). Guests never get unlock capability or clipboard by default.
- Audit log of joins and control grants.
- Revoking a link tears down its PeerConnections (mirror `revoke_test`/`portproxy_revoke_test`).
**Phase to address:** P5 (design), P7 (security review, `gsd-secure-phase`).

---

### Pitfall 14: First launch from the mounted DMG: translocation, busy volume, dangling paths

**What goes wrong:**
- `os.Executable()` returns a randomized read-only AppTranslocation path (or `/Volumes/…`) and "move to /Applications" fails or moves the wrong thing. Translocation hits a quarantined app first run from where it arrived, and only a Finder move reliably clears it [M, Eclectic Light, Apple forum 724969].
- **The daemon pins the volume.** The GUI spawns `exe daemon` (Setsid, outlives the GUI) from `/Volumes/Portty/Portty.app/...`. The long-lived daemon keeps the DMG busy, so eject fails with "disk in use", and later its binary path vanishes.
- `Install Command-Line Tool` writes `/usr/local/bin/portty` pointing at `os.Executable()`. Run from the DMG or a translocated path, it dangles after eject.
- The updater deliberately quarantines the DMG (`Verify`, UPD-03: "so Gatekeeper still checks the app on first run"). That is a Chesterton's Fence: it exists for the reveal-in-Finder flow, not for self-install.
**How to avoid:**
- At startup detect "running from a volume under /Volumes" or a translocated path (path contains `/AppTranslocation/`) **before** touching the daemon, tray, CLI link or settings. In that mode: copy to `/Applications` (or `~/Applications` if not writable), strip quarantine from the copy after verifying, `open` the installed copy, then quit. The installed copy starts the daemon.
- Only after the installed copy is up: `hdiutil detach` (retry with backoff; `-force` only as a last resort and never while any Portty process holds files there), then move the DMG to the Trash only if it lives in `~/Downloads`/`~/Desktop` and its SHA/identifier matches the running build. Never delete an arbitrary path the user pointed at.
- Compute the CLI link and daemon path from a "canonical install path" helper, not `os.Executable()`. Refuse to install the CLI link from a non-canonical location.
- Handle "already installed, older version present, daemon running" by going through the v1.2 version-mismatch handoff, not by killing the daemon.
**Phase to address:** P1.

---

### Pitfall 15: Replacing the installed bundle: overwrite, permissions, quarantine, Gatekeeper dialog

**What goes wrong:** The update copies over a running bundle and the app crashes with a code-signature error; or `/Applications` is not writable; or the relaunch shows "downloaded from the Internet, are you sure?" on an unattended Mac.
**How to avoid:**
- Stage the verified new bundle **next to** the old one on the same volume (`/Applications/.Portty.new`), verify (`codesign --verify --deep --strict`, Team ID, identifier, `spctl`), then swap with `rename`/`renamex_np(RENAME_SWAP)`, keeping the old bundle as `Portty.app.prev`. Never `cp` over files of a running executable.
- Reuse the existing verify chain (`updater.Verify`: Team ID, signing identifier, notarized spctl) on the **installed** bundle, not only the DMG, and compare DR with the running app (Pitfall 2).
- Check writability first. `/Applications` is `root:admin` 775: an admin user can swap, a standard user (possible on the MDM Mac) cannot replace a bundle someone else owns. Fall back to `~/Applications` or ask once, and never prompt for admin from the headless path. `[spike]` on the office Mac's account type. Same-Team-ID self-update normally avoids the App Management prompt, but verify in a clean VM. [M]
- After verification, remove `com.apple.quarantine` from the **installed** bundle so relaunch does not raise a Gatekeeper dialog or translocation. This is safe only because Portty verified Team ID, identifier and notarization itself, and is what Sparkle does. Keep the DMG quarantine as is.
- Copy with `ditto` (preserves symlinks and xattrs); never `cp -r` or archive tools that follow symlinks (breaks the seal).
**Phase to address:** P1.

---

### Pitfall 16: Relaunch races, the quit modal, rollback, and session survival

**What goes wrong:**
- `open Portty.app` runs while the old process is still alive: LaunchServices activates the old instance and nothing relaunches; or the old instance exits after the new one saw it and quit.
- The updater calls the normal quit path and the **quit-confirm modal** (`beforeClose` always prevents default) waits forever on a hidden window. `QuitAll` ends all sessions.
- The new build crashes at launch on an unattended Mac, with no previous version to fall back to.
- The daemon: `UPGR` takeover hands PTYs and TCP listeners to the new daemon. **UDP/WebRTC sockets and live PeerConnections cannot be handed over**, so every desktop viewer drops during an update.
**How to avoid:**
- A small standalone helper (Apple's recommended shape): the app spawns it detached (own process group), the helper waits for the app pid to exit (poll with timeout), then swaps (Pitfall 15), then `open`s the new app, then verifies a **healthy marker** (the new GUI writes a file or answers a socket within N seconds). If not healthy, swap back to `.prev` and relaunch it. Keep `.prev` until the next successful launch.
- Quit with the GUI-only semantics (`app.quitting` set, as `onTerminate` does). Never `QuitAll`. Add a test that an update relaunch leaves daemon sessions alive and the new GUI re-attaches through the handoff.
- Tell desktop viewers about the restart ("Host updating, reconnecting") and make viewers auto-reconnect with backoff and a fresh keyframe.
- Provide a user-visible "Install and relaunch" confirm with a count of connected desktop viewers; the unattended path (auto-install) must be opt-in because failure has no human recovery.
- Single-instance guard: Wails `SingleInstanceLock` is not in use today (the daemon uses a flock). Without a guard, a double relaunch yields two GUIs. Add one for the GUI or rely on the helper to launch exactly once.
**Warning signs:** after update there is no menu-bar icon; two icons; `.prev` accumulates; viewers hang after update.
**Phase to address:** P1, with P3 for viewer reconnect.

---

### Pitfall 17: Notarization, signing, and build pipeline for new code

**What goes wrong:** A new helper, ScreenCaptureKit/VideoToolbox cgo code, or libopus breaks the universal build or notarization.
**Why it happens:** Release builds are `darwin/universal` DMGs signed with `codesign --deep --force` and one `entitlements.plist` (network client/server only). `--deep` is deprecated guidance and signs nested code with the same flags; hardened runtime and a secure timestamp are required for every Mach-O or `notarytool` rejects the upload. A universal cgo build needs each dependency built for arm64 and x86_64 (libopus, if used for audio). `LSMinimumSystemVersion` is 10.13 but ScreenCaptureKit needs 12.3+, system-audio capture 13+, the picker 14+, the persistent-capture entitlement 14.4+.
**How to avoid:**
- Decide early: **arm64-only** (both target Macs are Apple Silicon MacBook Airs) removes the universal-cgo burden and halves test surface. If Intel stays, build libopus statically for both and `lipo`.
- Sign nested executables inside-out explicitly (helper first, app last) with hardened runtime, timestamp, and a stable identifier. Add `codesign --verify --deep --strict` and `spctl -a -t exec` on the final app plus a check that no Mach-O in the bundle is unsigned (a find-based test in CI).
- Raise `LSMinimumSystemVersion` to the lowest macOS that actually supports the used APIs (14.4 if the entitlement is pursued) and gate the UI on it.
- Put helper plists (if `SMAppService.agent`) in `Contents/Library/LaunchAgents`; test registration in a clean VM.
- No JIT or library-validation exceptions should be needed. If a PR adds `com.apple.security.cs.disable-library-validation`, challenge it.
- Entitlements for screen recording/Accessibility are TCC, not entitlements; do not add fake keys.
**Phase to address:** P2 (arch decision, helper signing), P7 (pipeline gates).

---

### Pitfall 18: Tart VM testing limits for ScreenCaptureKit, audio and lock screen

**What goes wrong:** Tests pass in the VM and the feature fails on real hardware (or the reverse), and VM TCC state cannot be scripted.
**Known limits (several `[spike]`, none verified for Tart specifically):**
- **TCC cannot be pre-granted** with SIP on (the system TCC DB is protected) and the user DB approach broke on a macOS 27 beta [M, forum 834560]. Grant once through the GUI, then snapshot the disk. Cloning a running VM duplicates its ephemeral tailnet node identity, so snapshot **before** joining the tailnet and re-key after. A stable test-signing identity keeps the grant across builds (Pitfall 2).
- **No hardware video encoder** is expected in a macOS guest, so VideoToolbox uses software or fails if hardware is required. CPU, latency and thermals measured in the VM are meaningless.
- **Audio:** Virtualization.framework provides a virtio sound device and audio drifts relative to video in guests [M, Eclectic Light]. ScreenCaptureKit system-audio capture inside a guest is unproven. Treat audio as a real-Mac test.
- **Displays:** one virtual display. Display switching, hot-plug, Retina scale changes and clamshell cannot be tested. A private `CGVirtualDisplay` can fake a second display for tests only.
- **Display sleep and lid:** the virtual display does not behave like a panel; display-sleep, clamshell and wake-for-viewer need real-Mac manual items.
- **Lock screen:** the VM does have a real lock screen, so lock-screen capture and typing **can** be spiked there. Check the same macOS version as the office Mac, and its lock plug-in if any.
- **Network:** per the repo rule, no live tailnet from subagents. WebRTC tests use VM-to-VM on Tart's LAN with the `vmpreview` interface-filter override. Real-tailnet and DERP-path behaviour goes to manual VM rows or real-Mac rows.
- **DMG self-install** is a good fit for the VM: Quinn recommends a clean VM, install N then upgrade to N+1, because dev machines hide Launch Services, Gatekeeper and background-task state [H].
- Add manual M-NN items to TESTING.md Section 5 for everything unautomatable and a traceability row for each new test file (standing rule in CLAUDE.md).
**Phase to address:** every phase, with the rig defined in P2.

---

## Moderate Pitfalls

### Pitfall 19: Audio capture and encode
**What goes wrong:** No Go Opus encoder in the stdlib; cgo libopus adds build and signing weight. ScreenCaptureKit captures the app's own audio unless excluded (`excludesCurrentProcessAudio`), which can echo viewer audio. Audio needs Screen Recording permission too. iPhone Safari needs a user gesture and obeys the mute switch; A/V sync drifts if timestamps are not wall-clock based.
**Prevention:** Make audio a separate toggle shipped after video works; exclude current process audio; unmute on tap; start silent. If libopus is a burden, defer audio rather than dragging it into the first phase.
**Phase:** P4.

### Pitfall 20: cgo callbacks and CVPixelBuffer lifetime
**What goes wrong:** ScreenCaptureKit and VideoToolbox call back on their own threads. Holding Go pointers across cgo, forgetting `CFRelease`, or skipping autorelease pools leaks IOSurface-backed buffers (hundreds of MB) and crashes the capture host. The v1.1 hardening work (bounded memory) is the standard to keep.
**Prevention:** Hand off through a bounded channel with drop-oldest, retain/release buffers symmetrically, wrap each callback in an autorelease pool, run a long soak test (RSS stays flat over 1 h of idle and motion), and cap `queueDepth`.
**Phase:** P2, P7.

### Pitfall 21: Adaptive quality "sharp when still, smooth when moving"
**What goes wrong:** Because idle frames are not delivered, there is no "still" signal except the absence of frames. Re-encoding at higher quality after idle needs the cached frame; bitrate changes mid-stream cause visible pumping; changing resolution or display needs a new SPS plus IDR and a track renegotiation or at least a resolution-change message.
**Prevention:** State machine: moving (target fps, bitrate cap, QP up) vs idle (after ~300 ms with no frames, one high-quality refinement frame, then hold); update bitrate only through VideoToolbox properties, not by recreating the session; on display/window switch, recreate the SCStream, emit SPS/PPS+IDR, and notify viewers via data channel.
**Phase:** P5.

### Pitfall 22: Cursor and pointer
**What goes wrong:** `showsCursor` bakes a cursor into the video and the viewer sees it late; a viewer's own cursor lags too, so control feels rubbery.
**Prevention:** Send the host cursor position and shape over the data channel and draw it client-side; disable `showsCursor`.
**Phase:** P4/P5.

### Pitfall 23: pion version and dependency drift
**What goes wrong:** pion/webrtc is at v4.2.x; a v5 major was mentioned but not found. Interceptor APIs (`gcc`, `intervalpli`) and `SettingEngine` signatures move between releases.
**Prevention:** Pin v4, read source before relying on blog snippets, and wrap pion behind one internal package so the rest of Portty does not import it. Run `go mod tidy` and the existing license/brand/grep gates, since pion pulls many transitive modules.
**Phase:** P3.

### Pitfall 24: Health and state honesty
**What goes wrong:** The UI says "Sharing" while capture is blocked (TCC), display is asleep, or no viewer can connect.
**Prevention:** One state machine surfaced in the tray and the share view: Permission missing, Needs approval, Display asleep, No display, Waiting for viewer, Streaming (N viewers, direct|relayed). Never show a black video as "connected".
**Phase:** P2/P5.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Capture inside the existing daemon process | No new IPC | TCC misattribution, no window server in SSH respawns, ScreenCaptureKit unsupported in daemons | Never |
| Rely on the monthly prompt, skip the entitlement request | No signing-profile work | Capture silently dies monthly on an unattended Mac | Acceptable if the viewer shows an honest state and the request is filed |
| `cp -R` the new app over the old one | 3 lines | Code-signature crash, half-updated bundle, no rollback | Never |
| Auto-install without opt-in or rollback | "Just works" | A bad build bricks the only remote path to the Mac | Never. Opt-in plus `.prev` rollback. |
| Ad-hoc signed VM builds | Fast iteration | Grants reset per build, unreliable TCC tests | Only for non-TCC tests |
| One global encoder bitrate | Simple | One slow viewer degrades all | OK for v2.0 with a viewer cap |
| Poll the pasteboard every 50 ms | Feels instant | Privacy prompts, secrets leak, CPU | Never |
| Store the macOS password for auto-unlock | One-click unlock | Credential on a corporate Mac, trivially abused | Never |
| Universal (arm64+x86_64) with cgo deps | Matches current packaging | libopus/VT/SCK dual-arch build and test cost | Only if an Intel Mac must be supported |
| pion `ICEServers` left default or interfaces unfiltered | Works on LAN | Leaks beyond the tailnet, public STUN traffic | Never |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| ScreenCaptureKit | Treating `SCShareableContent` as a free permission probe | One probe per user action; on macOS 27 each call can alert |
| ScreenCaptureKit | Expecting steady frames | Frames only on change; cache last frame for IDR/refresh |
| VideoToolbox to pion | Sending AVCC, SPS/PPS once | Annex B, SPS/PPS before every IDR |
| pion | Default SettingEngine | Interface/IP filter, UDP only, ICE-lite, no mDNS, no ICE servers |
| Browsers | Waiting for the browser's host candidate | Let the browser connect to the Mac's candidate; learn prflx |
| Wails | Web Clipboard API in `wails://` page | Wails runtime clipboard from Go |
| Wails menu | Cmd+key combos eaten by menu | Control mode disables key equivalents |
| Daemon handoff | Assuming UDP/PeerConnections migrate | Reconnect viewers after takeover; fds handed over are TCP/PTY only |
| `updater.Verify` | Reusing its quarantine behaviour for installs | Quarantine the DMG for reveal; strip quarantine from the verified installed bundle |
| Quit path | Using Quit All or the modal for relaunch | GUI-only quit with `quitting` set, helper waits on pid |
| `portty` CLI link | Pointing at `os.Executable()` | Canonical install path helper |
| MDM | Assuming profile can pre-grant the persistent-capture entitlement | It cannot yet; plain PPPC Screen Capture grants may exist but are not guaranteed to cover new categories |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Full Retina capture at 60 fps | High CPU/heat on MacBook Air, throttling | Cap resolution (about 1920 wide), 30 fps when moving, idle refresh | Immediately on a 2560x1664 panel |
| Per-viewer encode | CPU scales with viewers | One encoder, fan-out | 2+ viewers |
| IDR storms from PLI | Bandwidth spikes, stutter | Coalesce to 1 per 0.5-1 s | 2+ viewers or any packet loss |
| DERP-relayed UDP | Lag, low fps | Show direct/relay, warn | When direct path fails |
| Fragmented RTP over MTU 1280 | Loss, jitter | Keep 1200 MTU, chunk data channel | Any large clipboard/data message |
| IOSurface buffer leaks | RSS grows, daemon killed | Autorelease pool, bounded queue, soak test | Hours of uptime |
| Display held awake for hours | Battery/thermal on the office Mac | Assertion only while a viewer is connected | Unattended overnight |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| UDP bound on all interfaces | LAN/internet exposure of a desktop-control endpoint | Tailnet-only bind with test (Pitfall 6) |
| Guest link grants control without host consent | Full desktop takeover by a tailnet colleague | Consent, view-only default, `WhoIs`-bound token (Pitfall 13) |
| Remote unlock available to guests or via stored credential | Account takeover | Owner identity only, password typed by the owner, never stored or logged |
| Logging key events or clipboard | Password capture in `daemon.log` | Never log input payloads |
| CSP loosened for the viewer (`unsafe-eval`, `blob:` script) | XSS to desktop control | Keep strict CSP; guard with regression tests |
| Window-share input escaping the window | Typing into other apps | Frontmost-app and bounds checks (Pitfall 11) |
| Updater trusts any DMG or the quarantine strip is applied before verification | Malicious app replaces Portty | Strip only after Team ID, identifier, notarization and DR checks |
| Token reuse after revoke | Ghost viewers | Revoke tears down PeerConnections |
| Persistent unattended mode with no indicator | User unaware of being viewed | Menu-bar indicator while any viewer is connected |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Black video when permission is missing/asleep/locked | User thinks the app is broken | Explicit state messages (Pitfall 24) |
| No guidance for "app not in Screen Recording list" | Stuck at onboarding | Show the click-+ instructions with a deep link |
| Update restarts the app with no warning | Desktop viewers drop | Warn with viewer count; auto-reconnect |
| DMG first-launch asks the user to drag, eject, delete manually | Friction and stale DMGs | Self-install flow with progress and a failure fallback |
| Remote unlock presented as always possible | False trust | Label: "Works when the Mac is locked, not after a restart" |
| Silent monthly capture failure | No remote access discovered at the worst time | Notification at the Mac plus a viewer state; docs |
| Cmd-key combos do nothing in the in-app viewer | Unusable control | Control mode plus modifier bar |

## "Looks Done But Isn't" Checklist

- [ ] **Permissions:** Often missing the update path. Verify grants survive an N to N+1 upgrade in a clean VM with the stable test identity.
- [ ] **Capture:** Often missing the CLI/SSH respawn case. Verify capture after the daemon is respawned over SSH.
- [ ] **Lock/sleep:** Often missing wake-on-viewer. Verify lock screen visible and typeable, display sleep recovery, and a clear message when the lid is closed.
- [ ] **Join matrix:** Often only Chrome. Verify Chrome, Safari, iOS Safari, WKWebView, on and off the LAN.
- [ ] **Late joiner:** Often missing an immediate IDR on a static screen. Verify a viewer joining to an unchanged desktop gets video within about 1 s.
- [ ] **Multi-viewer:** Verify three viewers joining at once do not storm IDRs and one slow viewer does not break others.
- [ ] **UDP binding:** Verify no UDP socket on non-tailnet addresses and no packets to public IPs.
- [ ] **Input:** Verify keys are released on disconnect, Retina and multi-display coordinates, window-share confinement.
- [ ] **Clipboard:** Verify no prompt on an unattended Mac, concealed types skipped, no echo loop.
- [ ] **CSP:** Verify zero violations on the viewer page in Chromium and WebKit e2e, and guarded deltas.
- [ ] **Self-install:** Verify first launch from DMG, eject succeeds with the daemon running from `/Applications`, the DMG trashed, CLI link correct, no quarantine dialog on relaunch.
- [ ] **Update relaunch:** Verify the helper relaunches exactly once, rollback works when the new build refuses to start, sessions survive, the `.prev` copy is cleaned after success.
- [ ] **Release:** Verify every nested Mach-O is signed and notarized, DR unchanged versus the previous release, `LSMinimumSystemVersion` correct.
- [ ] **Memory:** Verify flat RSS over a long soak with and without viewers.
- [ ] **TESTING.md:** Verify new automated files are in the manifest and traceability map and the new M-NN items exist (standing rule).

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| TCC grant lost/stale | MEDIUM (needs someone at the Mac) | Notification plus onboarding panel; `tccutil reset ScreenCapture app.portty.Portty` then re-grant |
| Monthly prompt blocks capture | MEDIUM | Click Allow at the Mac; honest viewer state |
| Bad update, new build won't start | HIGH if no rollback | Helper swaps back to `.prev`; otherwise the user must reinstall from the DMG at the Mac |
| Update interrupted mid-swap | MEDIUM | Stage-then-rename leaves either old or new complete; startup reconciles stray `.Portty.new`/`.prev` |
| Viewer cannot connect (ICE) | LOW | Fall back to Plan B WSS transport; show path diagnostics |
| IDR storm | LOW | Raise the coalesce interval remotely via settings; restart the stream |
| Capture host crash | LOW | Daemon respawns it via the Aqua-session launcher; sessions unaffected because terminals live in the daemon |
| Unlock infeasible on the office Mac | LOW (scope change) | Cut the feature; keep unattended-while-awake |
| Daemon pinned DMG | LOW | Detach after relaunch; if busy, report which process holds it (`lsof +D`) |

## Pitfall-to-Phase Mapping

Phase names are suggestions for the roadmapper. P2 should start immediately, in parallel with P1, because it contains the go/no-go spikes and the long-lead entitlement request.

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| 1 TCC attribution | P2 | VM: grant works for GUI-, CLI- and SSH-spawned paths; list shows Portty.app |
| 2 Unstable code identity | P2, P1, P7 | Upgrade N to N+1 keeps grants; DR diff gate |
| 3 Monthly re-consent | P2, P5, P7 | Simulated zero-frame state shows the viewer message; entitlement request filed |
| 4 Wrong process context | P2 | Capture works with no GUI window; daemon contains no ScreenCaptureKit |
| 5 Sleep/lid/display/idle | P2, P5 | Wake on viewer; static-screen late joiner gets IDR |
| 6 Interface binding | P3 | Test asserts tailnet-only UDP and no public egress |
| 7 ICE/MTU/DERP | P3 | Join matrix; direct/relay indicator; Plan B documented |
| 8 H.264 compatibility | P2, P3 | Golden Annex B tests; Safari and iOS decode; SDP equals SPS |
| 9 Fan-out/keyframes | P3, P5 | 3-viewer join test; coalesced IDR count |
| 10 WKWebView/CSP | P3, P4 | Wails viewer M-item; CSP e2e zero violations |
| 11 Input correctness | P4 | Release-all-on-drop test; coordinate and window-confinement tests |
| 12 Clipboard | P4, P5 | No-prompt test on unattended VM; concealed-type test |
| 13 Click-to-share trust | P5, P7 | Consent flow, WhoIs binding, revoke test, secure-phase review |
| 14 First launch from DMG | P1 | Clean-VM walkthrough: eject succeeds, CLI link valid |
| 15 Bundle replacement | P1 | Atomic swap, writable and non-writable cases, no quarantine dialog |
| 16 Relaunch/rollback/sessions | P1, P3 | Helper tests, forced bad build rolls back, sessions survive |
| 17 Signing/notarization/build | P2, P7 | CI checks for nested signing, arch, minimum OS |
| 18 Tart limits | all | Documented rig; real-Mac M-items for audio, displays, sleep, encoder perf |
| 19 Audio | P4 | Mute and gesture on iPhone; A/V sync check on a real Mac |
| 20 cgo/memory | P2, P7 | 1 h soak, flat RSS |
| 21 Adaptive quality | P5 | Motion/still transitions without pumping |
| 22 Cursor | P4, P5 | Cursor via data channel |
| 23 pion drift | P3 | Single wrapper package, pinned v4 |
| 24 State honesty | P2, P5 | State machine unit tests and UI |
| Remote unlock | P2 (spike), P6 | Spike result recorded; feature only if the in-session lock screen works on the office macOS and lock plug-in |

## Open Questions (unverified, need spikes or the user)

1. Does ScreenCaptureKit capture the lock screen and does `CGEventPost` type into it for a logged-in locked session on the target macOS (including with the office Mac's lock plug-in, if any)? Only anecdotal evidence found.
2. Which process do TCC and the "responsible process" rules attribute for the GUI-spawned vs CLI-spawned daemon on the target macOS? Not documented; test it.
3. Does the persistent-content-capture entitlement stop the monthly prompt and map to the new "Remote Desktop" category on release macOS 27? Forum speculation only; the entitlement's own docs were not retrievable.
4. Do Safari, iOS Safari and WKWebView offer a candidate on the Tailscale interface and can they reach the Mac's 100.x ICE-lite candidate?
5. Is a hardware H.264 encoder available in Tart guests, and does system-audio capture produce samples there?
6. Is the office Mac user an admin (can swap `/Applications/Portty.app`), and is the Mac's Tailscale a network extension rather than userspace mode?
7. How RustDesk and Jump Desktop actually implement unlock was not verified from primary sources.

## Sources

Apple Developer Forums and Apple docs (HIGH where Apple DTS speaks):
- Capturing screen buffer at macOS Login Window with ScreenCaptureKit and PrivilegedHelper: https://developer.apple.com/forums/thread/814152
- New security feature prevents remote access apps from working: https://developer.apple.com/forums/thread/756908
- Capture Screen pre-login context on macOS 15 Sequoia: https://developer.apple.com/forums/thread/768146
- Interacting with Login Window using third party screen share app (rdar 5636091): https://developer.apple.com/forums/thread/726470
- Keyboard and Mouse simulation not working in pre-login window: https://developer.apple.com/forums/thread/62687
- ScreenCaptureKit and the persistent-content-capture entitlement: https://developer.apple.com/forums/thread/761641 , https://developer.apple.com/forums/thread/770675 , https://developer.apple.com/forums/thread/816646
- MDM: Persistent Content Capture (aka Remote Desktop): https://developer.apple.com/forums/thread/831795
- Public API to silently query "Remote Desktop" TCC status: https://developer.apple.com/forums/thread/841530
- Plain executables not listed under Screen & System Audio Recording (26.1): https://developer.apple.com/forums/thread/807898
- Screen Recording popup not shown (27.3 beta 2): https://developer.apple.com/forums/thread/858698
- Why does my app lose Screen Recording permission after updating (ad hoc): https://developer.apple.com/forums/thread/795739
- Daemon in an app with a self-update feature: https://developer.apple.com/forums/thread/758329
- SCScreenshotManager.captureImage and display sleep: https://developer.apple.com/forums/thread/827059
- screenUnlockMode black screen on macOS 27 beta: https://developer.apple.com/forums/thread/838847
- User TCC DB inaccessible for CI setups: https://developer.apple.com/forums/thread/834560
- App Translocation Notes: https://developer.apple.com/forums/thread/724969

Press and third-party (MEDIUM/LOW):
- Monthly Screen Recording prompt: https://www.macrumors.com/2024/08/15/macos-sequoia-screen-recording-app-permissions/ , https://macrumors.com/2024/10/07/apple-screen-recording-popup-update
- Pasteboard privacy in macOS 15.4: https://mjtsai.com/blog/2025/05/12/pasteboard-privacy-preview-in-macos-15-4
- App Translocation: https://eclecticlight.co/2023/05/09/what-causes-app-translocation , https://eclecticlight.co/2024/05/14/lost-in-translocation/
- N-able macOS remote desktop permissions: https://documentation.n-able.com/takecontrol/userguide/Content/system-requirements/macOS-security-settings.htm
- RustDesk macOS docs and unattended setup: https://rustdesk.com/docs/en/client/mac/ , https://rustdesk.com/blog/rustdesk-unattended-access-setup
- WebKit bug 195124 (negotiated Baseline, encoded High): https://bugs.webkit.org/show_bug.cgi?id=195124
- pion/webrtc SettingEngine (v4.2.x): https://pkg.go.dev/github.com/pion/webrtc/v4#SettingEngine
- Tailscale connection types and performance (DERP not for media): https://tailscale.com/docs/reference/connection-types

Repo evidence read for this file: `internal/daemon/process_unix.go` (detached `exe daemon` spawn), `internal/daemon/handoff.go` (fd handoff scope), `internal/daemon/remote_ws_proxy.go` (webview reaches loopback only), `internal/webserver/server.go` and `csp_mw.go` (tailnet bind, CSP history), `internal/updater/install.go` (Verify and quarantine), `app_cli_install.go` (`os.Executable` link), `app.go` and `main.go` (quit modal, `quitting`), `build/entitlements.plist`, `build/darwin/Info.plist`, `.github/workflows/release.yml` (universal build, `--deep` signing), `TESTING.md` (Tart rig).

---
*Pitfalls research for: desktop sharing (WebRTC) and DMG self-install in Portty v2.0*
*Researched: 2026-11-24*
