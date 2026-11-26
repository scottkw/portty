# Stack Research

**Domain:** Desktop sharing (WebRTC screen/audio/input/clipboard) and DMG self-install for an existing macOS Go + Wails app
**Milestone:** v2.0 Desktop Sharing (additions only; v1.x stack is unchanged and not re-researched)
**Researched:** 2026-11-24
**Overall confidence:** MEDIUM. Library versions, pion APIs, DMG mechanics and Opus encoding were verified first-hand (module resolution, local macOS 27.2.1 experiments, pion source). TCC behavior, WKWebView WebRTC and unattended re-authorization rest on Apple forum threads and must be proven by a spike on a real Mac (not a Tart VM) before the roadmap commits to them.

Confidence tags: [HIGH] verified first-hand or from official source; [MED] consistent secondary sources or training knowledge, spike to confirm; [LOW] forum anecdote or beta report, treat as a question, not a fact.

---

## Decisions at a glance

| Question | Decision | Confidence |
|----------|----------|------------|
| WebRTC library | `github.com/pion/webrtc/v4` **v4.2.29** (2026-11-22). Do not use v5.0.0-rc.x yet. | HIGH |
| Capture + encode bridge | **cgo + Objective-C inside the daemon process** (new package, `.m` files, same pattern as `tray_objc_darwin.m`). No Swift, no helper binary. | MED-HIGH |
| Who holds TCC (Screen Recording, Accessibility) | **The Portty app identity (`app.portty.Portty`)**. The daemon is the same signed Mach-O as the GUI (`Portty.app/Contents/MacOS/portty daemon`), so one grant covers both. Capture and input run **in the daemon**. The GUI only triggers the prompts during onboarding. | MED (spike) |
| Video codec | VideoToolbox H.264, **Constrained Baseline by default**, no B-frames, Annex-B out, fed to `TrackLocalStaticSample`. | HIGH (profile), MED (VT keys) |
| Audio | ScreenCaptureKit audio (48 kHz stereo) then **AudioToolbox `AudioConverter` Opus encoder** (OS-native). No libopus. | HIGH (encoder exists), MED (API recipe) |
| Input | CoreGraphics `CGEvent` from the daemon, over WebRTC DataChannels. No Go input library. | HIGH |
| Clipboard | `NSPasteboard` `changeCount` polling in ObjC, text first, over a DataChannel. | MED |
| Viewer | Vanilla browser WebRTC (`RTCPeerConnection`, `<video>`, DataChannels). No JS WebRTC library. Wails (WKWebView) viewer is a **risk to prove first**; fallback is opening the browser link. | MED/LOW |
| DMG self-install | Go + system tools (`hdiutil`, `ditto`, `xattr`, `codesign`, `spctl`, `plutil`) plus `unix.RenamexNp(RENAME_SWAP)` from the already-present `golang.org/x/sys`. **No Sparkle**, no new Go dependency. | HIGH |
| Remote unlock | **Do not plan it into v2.0 delivery.** Treat as a post-MVP spike. See "Remote unlock". | MED (that it is hard), LOW (details) |

New Go dependencies for the whole milestone: **one** direct (`pion/webrtc/v4`) plus `pion/interceptor` (already pulled transitively, used directly for bandwidth estimation). Everything else is Apple system frameworks and `x/sys` already in `go.mod`.

---

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| `github.com/pion/webrtc/v4` | **v4.2.29** (go 1.24+; repo is on 1.26.4) | Pure-Go WebRTC: ICE, DTLS-SRTP, SCTP DataChannels, RTP packetizers | The only maintained pure-Go WebRTC stack. v4.2.x ships patch releases every few days (4.2.27 to 4.2.29 in Nov 2026). v5.0.0-rc.4 (2026-11-22) is a release candidate that removes `SetNAT1To1IPs` and Plan B and bumps the module path to `/v5`; wait for a stable v5, then migrate. |
| `github.com/pion/interceptor` | v0.1.51 (transitive; import `pkg/cc`, `pkg/gcc` directly) | Send-side bandwidth estimation (GCC with TWCC) | Gives `GetTargetBitrate()` per peer connection, which is the signal for adaptive quality. Same pattern as pion's `bandwidth-estimation-from-disk` example. |
| ScreenCaptureKit | macOS 13+ (system) | Window or display capture, system audio, live display switch | Apple's supported capture API. Delivers `420v` (NV12) frames straight into VideoToolbox. `SCStream updateContentFilter:` and `updateConfiguration:` change the display or window without tearing the stream down (display switch). Audio needs macOS 13. |
| VideoToolbox `VTCompressionSession` | system | Hardware H.264 encode | Apple Silicon hardware encoder, low latency with `RealTime` and no reordering. |
| AudioToolbox `AudioConverter` (`kAudioFormatOpus`) | system (since 10.13) | Opus encode of SCK audio | **Verified locally on macOS 27.2.1**: `afconvert -d opus@48000 -c 2 -b 64000` produced 48 kHz stereo Opus at 960 frames/packet (20 ms). Raw Opus packets drop straight into RTP. Avoids libopus entirely (see "What NOT to Use"). |
| CoreGraphics `CGEvent*` | system | Mouse, scroll, keyboard injection | `CGEventCreateMouseEvent`, `CGEventCreateScrollWheelEvent2`, `CGEventCreateKeyboardEvent`, `CGEventKeyboardSetUnicodeString`, posted at `kCGHIDEventTap`. Needs Accessibility only. |
| AppKit `NSPasteboard` | system | Clipboard read/write | No change notification exists; poll `changeCount`. |
| IOKit power assertions | system | Keep display awake while sharing, wake it for a viewer | `IOPMAssertionCreateWithName(kIOPMAssertionTypePreventUserIdleDisplaySleep)` while a share is live; `IOPMAssertionDeclareUserActivity` to wake. A sleeping display yields no frames. |
| Browser WebRTC (`RTCPeerConnection`) | n/a | Viewer in desktop browsers, iPhone Safari, Wails WKWebView | Receive-only video and audio plus DataChannels. Viewer is the SDP **offerer** (`recvonly` transceivers), the daemon answers. |

### Supporting Libraries (Go)

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/coder/websocket` | v1.8.15 (already in go.mod) | WebRTC signaling (JSON offer/answer) | Reuse the authenticated share-server WSS pattern. No new signaling dependency. |
| `golang.org/x/sys/unix` | v0.46.0 (already in go.mod) | `unix.RenamexNp(from, to, unix.RENAME_SWAP)` for atomic app swap; `Statfs` for mount detection | Verified present in v0.46.0. |
| `pion/webrtc/v4/pkg/media` | with webrtc | `media.Sample` for `WriteSample` | Always. |
| `pion/rtp`, `pion/rtcp` | v1.10.6 / v1.2.20 (transitive) | RTCP PLI/FIR handling | Read RTCP from `RTPSender` to force a keyframe. |

Transitive set pulled by `go get github.com/pion/webrtc/v4@v4.2.29` (checked in a scratch copy of this repo's `go.mod`): ice/v4 v4.4.9, dtls/v3 v3.1.16, srtp/v3 v3.1.3, sctp v1.12.1, datachannel v1.6.4, transport/v5 v5.1.5, turn/v5 v5.1.3, stun/v4 v4.0.1, mdns/v2 v2.2.2, sdp/v3 v3.0.20, logging v0.2.4, randutil v0.1.0, wlynxg/anet v0.0.5. **Only one existing module moves: `golang.org/x/time` v0.12.0 to v0.14.0.** `x/net` (v0.55.0), `x/sys`, `x/crypto` stay. No conflict with `tailscale.com` v1.98.3 or Wails.

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| pion as a **test viewer** | Go integration tests with no browser | A second `PeerConnection` (recvonly offerer) in `go test` proves signaling, RTP flow, DataChannel input and keyframe-on-PLI. Pair with a fixture Annex-B H.264 file in place of the real capturer. |
| Existing chromedp CSP e2e (`browser_csp_e2e_test.go`) | Guard the CSP | The page CSP is `default-src 'none'`, so `media-src` falls back to `'none'`. `video.srcObject = MediaStream` is generally not a `media-src` fetch, but verify; add at most `media-src 'self'`. WebRTC ICE is not governed by `connect-src` in Chromium. |
| `afconvert`, `hdiutil`, `plutil`, `xattr`, `ditto` | DMG and audio experiments | All ship with macOS; used to verify this research. |
| Real Mac (not Tart VM) for TCC/capture spike | Prove permission and encoder behavior | A VM guest likely has no hardware H.264 encoder (set `RequireHardwareAcceleratedVideoEncoder=false` with software fallback for VM tests only) and TCC grants cannot be driven automatically. Per project memory, no live-tailnet measurements from the host; two-peer numbers go to the VM rows. |

---

## Process and TCC architecture (the key decision)

**Facts from the repo [HIGH]:** the daemon is `portty daemon`, the same Mach-O as the GUI. It is started either by a user LaunchAgent (`kardianos/service`, `UserService=true`, `RunAtLoad`, `Arguments: ["daemon"]`, executable = `os.Executable()`) or detached by the GUI/CLI (`startDetachedDaemon`, `Setsid`). The webserver binds the host tailscaled's tailnet IP directly with `net.Listen` (no `tsnet`), so pion can bind a real UDP socket on the tailnet address.

**Decision [MED, spike]:** run capture, encode, input injection, clipboard and power assertions **inside the daemon**, in a new internal cgo package (suggested `internal/screencap`, with a thin pure-Go interface so everything above it is testable with fakes).

Why this process:

1. **One identity, one grant.** TCC keys the grant on the code identity. GUI and daemon are the same signed binary inside `Portty.app`, so Screen Recording and Accessibility granted to "Portty" apply to whichever process uses them. A separate helper would be a second identity with a second pair of grants and a second set of prompts on a Mac the user cannot reach.
2. **Availability.** The daemon is the process that survives GUI quit, respawns, and is handed live to the new version on upgrade. A GUI-hosted capturer dies whenever the user quits the tray app, which defeats "always-available".
3. **Apple's own guidance.** Apple DTS (forum thread on login-window capture) says GUI-type work such as ScreenCaptureKit and input posting belongs in a **LaunchAgent**, not a LaunchDaemon, with a root daemon only for global tasks. Portty's daemon is already a user LaunchAgent. [MED: forum answer, not documentation]
4. **No IPC to design.** Frames go from an ObjC callback to Go in the same address space.

Rules that follow (put these in the phase plans):

- **Never run the daemon from a copied or relocated executable.** On Tahoe 26.1 reports show bare Unix executables not appearing in the Screen & System Audio Recording pane. A binary inside the signed bundle is attributed to the bundle. [LOW-MED: forum reports; verify on the current macOS 27]
- **Trigger the prompts from the GUI** during an onboarding screen (user is physically at the Mac): `CGRequestScreenCaptureAccess()` and `AXIsProcessTrustedWithOptions({prompt:true})` or `CGRequestPostEventAccess()`. The daemon then only *checks* (`CGPreflightScreenCaptureAccess`, `AXIsProcessTrusted`) and reports `needsPermission` to viewers instead of calling request APIs.
- **The daemon must live in the user's Aqua (GUI) session.** A daemon spawned by `portty attach` or any CLI call over SSH may not reach WindowServer and capture will fail. Make capture readiness an explicit health state ("capture unavailable: daemon not in a GUI session") and prefer the LaunchAgent (`RunAtLoad`) as the always-on mode. [MED]
- **Accessibility is the only permission for posting events** (`kTCCServicePostEvent`, shown in the Accessibility pane). Input Monitoring is for listening only and is not needed. [MED-HIGH: Apple DTS replies]
- **Signature stability keeps grants across updates.** The grant survives an in-app update only because the Developer ID identity and bundle ID stay the same. Dev/ad-hoc builds lose grants on every rebuild (known; document for contributors).
- **No new entitlements** are required by SCK, VideoToolbox, CGEvent or NSPasteboard under hardened runtime. `build/entitlements.plist` stays as is. System-audio capture through SCK falls under Screen Recording; do **not** add `NSMicrophoneUsageDescription`.

### Unattended re-authorization risk (flag for roadmap) [MED-LOW]

- Since macOS 15, apps using screen-capture APIs get a recurring "Allow For One Month / Open System Settings" prompt (monthly; 15.1 reduced frequency for regularly used apps). No public developer opt-out. On the unattended office Mac a pending prompt blocks capture and cannot be answered remotely, because the screen cannot be captured until it is allowed.
- Two mitigations to investigate in Phase 1, neither confirmed: (a) the restricted entitlement `com.apple.developer.persistent-content-capture` (Apple request form, scoped by Apple to headless VNC-style apps; as a restricted entitlement it also means an embedded provisioning profile in the Developer ID signing flow, and forum threads say MDM cannot pre-grant it); (b) an MDM key reported as `forceBypassScreenCaptureAlert` for managed Macs (single forum source, unverified, and the corporate Mac's policy is not under the user's control).
- macOS 27 beta reports show a separate **"Remote Desktop"** entry in Privacy & Security that `CGPreflightScreenCaptureAccess()` does not reflect, and `SCShareableContent` showing a consent alert on every call. [LOW: beta reports; re-test on the shipping macOS 27.2.x the user runs]
- **Action:** submit the entitlement request early (Apple lead time), and design a visible "capture permission expired" state plus a notification, so the failure is at least diagnosable from home.

### Alternatives rejected

| Alternative | Why not |
|-------------|---------|
| Separate Swift helper binary (or helper `.app`) talking to the daemon | Second TCC identity and grants; Tahoe plain-executable attribution problems; extra codesign/notarize target; IPC protocol for 30 to 60 fps of H.264; Swift toolchain in CI. Gains nothing the cgo path lacks. |
| Capture in the GUI (Wails) process | GUI is `LSUIElement` and can be quit; sharing would end with it; GUI may not be running on the unattended Mac. |
| Capture via `tsnet`-style userspace networking | Not applicable: Portty uses the host tailscaled, so UDP sockets on the tailnet IP work directly. |

---

## Capture and encode pipeline (inside the cgo package)

### ScreenCaptureKit configuration [MED: standard API, confirm in spike]

- Content: `SCShareableContent` for displays and windows. Display mode: `SCContentFilter initWithDisplay:excludingWindows:`. Window mode: `initWithDesktopIndependentWindow:` (captures even when occluded; a minimized window stops producing frames).
- Config: `pixelFormat = kCVPixelFormatType_420YpCbCr8BiPlanarVideoRange` (`'420v'`, no BGRA-to-NV12 conversion), `minimumFrameInterval` about 1/30 s (1/60 later), `queueDepth` 3 to 5, `showsCursor = YES` for the first version (cursor in video is simplest; a separate cursor channel is a later latency optimization), `capturesAudio = YES`, `sampleRate = 48000`, `channelCount = 2`, `excludesCurrentProcessAudio = YES`.
- Tag color as BT.709 (`colorMatrix` and matching VT color keys) so browsers do not shift colors.
- **Resolution cap:** scale in SCK (`width`/`height`) so the long edge stays at or below about 2560 to 3840. The hardware H.264 encoder tops out near level 5.1/5.2 (about 4096x2304); a 5K/6K external display would otherwise fail to encode. Retina capture at 2x is 4x the pixels; cap, do not default to native.
- Frames arrive **only on change**; idle frames have status `idle` and no pixel buffer. Handle this (see "sharp when still").
- Display switch: `updateContentFilter:` plus `updateConfiguration:`. If output dimensions change, recreate the VT session and force an IDR. `stream:didStopWithError:` fires on display unplug or window close; restart or report.
- Load cost: link with `-weak_framework ScreenCaptureKit` (and VideoToolbox, AudioToolbox) so every `portty attach`/CLI invocation does not pay for or depend on these frameworks.
- The `Info.plist` `LSMinimumSystemVersion` is currently 10.13.0. Raise to **13.0** (SCK audio) and build the cgo files with `-mmacosx-version-min=13.0`. The user's machines run macOS 26/27, so this costs nothing.

### VideoToolbox [MED: from API knowledge, set in spike]

- `VTCompressionSessionCreate` with encoder spec `EnableHardwareAcceleratedVideoEncoder = true` (require hardware on real Macs).
- Properties: `RealTime = true`, `AllowFrameReordering = false` (no B-frames), profile `H264_ConstrainedBaseline_AutoLevel` (fall back to `Baseline_AutoLevel`), `AverageBitRate` plus `DataRateLimits`, `ExpectedFrameRate`, long `MaxKeyFrameInterval` (keyframes on demand, not periodic), `MaxFrameDelayCount = 0`.
- Output is AVCC (length-prefixed). **Convert to Annex-B** and prepend SPS+PPS (from `CMVideoFormatDescriptionGetH264ParameterSetAtIndex`) on every keyframe. pion's `H264Payloader` takes Annex-B, caches SPS/PPS and emits them as a STAP-A before the next NAL; it drops AUD and filler NALs and has no keyframe awareness, so **you** must resend parameter sets with each IDR.
- Keyframe on demand: `kVTEncodeFrameOptionKey_ForceKeyFrame` when a viewer joins or sends PLI/FIR. **Rate-limit** (e.g. at most one IDR per 1 to 2 s): IDR bursts of hundreds of KB are the main packet-loss source on a tailnet path.
- Why Constrained Baseline: RFC 7742 makes it the one H.264 profile every WebRTC browser must decode (`profile-level-id` `42e01f`); pion's default registrations include `42e01f`, `42001f`, `4d001f`, `64001f`. High profile is more efficient for text but Chrome/Firefox/Safari High-profile receive behavior is only anecdotal in sources found [LOW]; make profile a constant and evaluate High in a measured phase. pion's fmtp matching compares only the first two bytes of `profile-level-id` (level ignored), so a high level on large frames does not break negotiation.
- Encoder ownership: **one encoder per share, many viewers.** Target bitrate = the minimum (or a low percentile) of the viewers' GCC estimates, with a floor.

### "Sharp when still, smooth when moving" (adaptive quality) [MED]

- On idle (no new SCK frame for about 250 to 400 ms), re-submit the last pixel buffer once at higher quality (lower QP via `kVTCompressionPropertyKey_Quality` / temporary bitrate lift). This also serves as the keep-alive that stops browsers from treating a static screen as a stalled stream.
- While moving: drop the frame rate or scale before dropping quality, driven by `estimator.GetTargetBitrate()` polled on a ticker (pion's example polls rather than subscribing).

### Audio [HIGH that Opus encoding works; MED on API recipe]

- SCK delivers 48 kHz stereo PCM `CMSampleBuffer`s in irregular sizes. Re-chunk into **960-frame (20 ms)** blocks in a small ring buffer, feed `AudioConverterFillComplexBuffer`, emit each Opus packet as one `media.Sample{Duration: 20ms}` on an Opus `TrackLocalStaticSample` (`MimeTypeOpus`, 48000/2).
- Expect about 312 priming frames at start; discard or ignore. Apple's encoder bitrate/CBR control is via `kAudioConverterEncodeBitRate`; fine for 64 to 96 kbps.
- A/V sync: sample `Duration` must be the **real elapsed time** between samples. `TrackLocalStaticSample.WriteSample` derives the RTP timestamp only from `Duration` (plus a carried remainder); it ignores `PacketTimestamp`. A variable-frame-rate screen stream written with a fixed 33 ms duration will drift against audio. Compute duration from SCK presentation timestamps.
- Fallback if the AudioConverter recipe fails in the spike: `github.com/hraban/opus` (cgo, libopus via pkg-config). Costs: libopus must be built for **both arm64 and amd64** because the release is `wails build -platform darwin/universal`, plus static-link and notarization hygiene. Prefer to avoid.

---

## pion integration details

- **Registry and engine:** `webrtc.MediaEngine` with `RegisterDefaultCodecs`, `RegisterDefaultInterceptors`, `ConfigureTWCCHeaderExtensionSender`, and a `cc.NewInterceptor(... gcc.NewSendSideBWE(gcc.SendSideBWEInitialBitrate(...)))`; hand the registry to `webrtc.NewAPI(WithInterceptorRegistry, WithMediaEngine)`. This is the documented pattern in pion's bandwidth-estimation example (v4.2.29).
- **Tailnet-only ICE, no STUN/TURN/cloud:** `Configuration{ICEServers: nil}`. `SettingEngine`: `SetNetworkTypes(UDP4)`, `SetIPFilter` allowing only the tailnet ranges (100.64.0.0/10, and fd7a:115c:a1e0::/48 if IPv6 is ever enabled), `SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)` (pion cannot resolve a browser's `.local` obfuscated host candidate over a tailnet; the browser's checks arrive from its 100.x address and pion learns it as a peer-reflexive candidate), and `SetICEUDPMux` on one UDP socket bound to the tailnet IP (one port, easy to reason about) or `SetEphemeralUDPPortRange`. **Do not use `SetNAT1To1IPs`**: it is removed in v5. `SetLite(true)` is an option since the server has one fixed reachable address; not required. [MED]
- **Browser candidate gathering on `utun`:** that Chrome and Safari include the Tailscale interface as a host candidate is the usual case for WebRTC over Tailscale but is **not verified here**; validate on two real machines in the VM rows. [LOW]
- **Signaling:** viewer opens WSS on the authenticated share server, sends the SDP offer (non-trickle: wait for gathering complete; with no ICE servers that is immediate), daemon answers. Reuse `internal/capability` tokens and the `whois` gate for the two share modes; the Wails viewer reaches a remote peer's signaling through the existing remote WS proxy path in the daemon rather than a new transport.
- **Tracks:** one `TrackLocalStaticSample` for video (`MimeTypeH264`, fmtp `level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f`) and one for Opus; add the same track objects to every viewer's `PeerConnection` (pion binds per-PC payload types). Retransmission works through the default NACK responder.
- **DataChannels** (SCTP is inside pion/webrtc; no extra dependency): `control` (JSON: display/window switch, quality, viewer role), `input-key` (ordered, reliable), `input-move` (unordered, `maxRetransmits: 0`, mouse moves), `clipboard` (ordered, reliable, size-capped).
- **PLI/FIR:** call `sender.ReadRTCP()` in a goroutine; on `*rtcp.PictureLossIndication` or `*rtcp.FullIntraRequest` request a rate-limited IDR.
- **MTU:** pion packetizes at about 1200 bytes; tailscale's 1280-byte tunnel MTU leaves room. Do not raise it.
- **macOS app firewall:** the existing TCP share server already binds the tailnet IP; a UDP listener behaves the same for a Developer-ID-signed app. Note for corporate Macs with the firewall on.

## Input injection (CGEvent) [MED-HIGH]

- Map browser `KeyboardEvent.code` (layout-independent physical key) to macOS virtual key codes on the host; send modifier flags with `CGEventSetFlags`. Use `CGEventKeyboardSetUnicodeString` for composed/IME text. Post with `CGEventPost(kCGHIDEventTap, ev)` (MacDeck, the reference app, does the same).
- Mouse: absolute position in global display coordinates (points, top-left origin; add the shared display's origin; scale from video pixels to points). Set `kCGMouseEventClickState` for double clicks; use `*Dragged` event types while a button is held. Scroll: `CGEventCreateScrollWheelEvent2` with pixel units.
- Window mode: raise/activate the target app before injecting and clamp the pointer to the window frame (SCK reports content rect per frame); `CGEvent` has no per-window targeting for the mouse.
- Browsers cannot capture some host-bound shortcuts (Cmd-Tab, Cmd-W); Chrome's `navigator.keyboard.lock()` helps only in fullscreen. Document it; do not build workarounds.
- Do **not** add `robotgo`, `go-vgo` or similar: a few hundred lines of ObjC beat a cgo dependency tree.

## Clipboard (NSPasteboard) [MED]

- Host side, in ObjC: poll `[NSPasteboard generalPasteboard].changeCount` every 250 to 500 ms; on change read `public.utf8-plain-text`, cap size (e.g. 1 MiB), send on the `clipboard` DataChannel. Record the `changeCount` after your own writes to avoid echo loops. Plain text first; images and files are later work.
- Viewer side, in the browser: use DOM `paste` / `copy` / `cut` events, which expose `clipboardData` **without a permission prompt**, and write to the viewer's clipboard with `navigator.clipboard.writeText` inside a user gesture. Browsers require a secure context (tailnet HTTPS provides it) and transient user activation, so host-to-viewer sync is "fires right after the viewer's own Cmd-C/Cmd-V", not truly background. Safari and WKWebView show a Paste affordance for programmatic reads. [MED: standard web platform behavior, confirm per browser]
- Make sync a per-share, per-direction toggle (clipboard is a data-exfiltration path next to full shell control).

## Browser-side decode and support [MED/LOW]

| Viewer | H.264 receive | Notes |
|--------|---------------|-------|
| Safari macOS / iOS (iPhone) | Yes (hardware). Constrained Baseline is safe; Safari's own default has been High. | `<video autoplay playsinline muted>` for iOS autoplay; start audio on the first tap (autoplay policy). On macOS 26+, any `RTCPeerConnection` use can trigger the OS **Local Network** prompt because of mDNS candidates, unless `getUserMedia` was granted first (W3C webrtc-pc issue, May 2026) [LOW-MED]. One-time per viewer. |
| Chrome desktop | Yes (mandatory Constrained Baseline). | Hardware or software decode depending on platform; use `chrome://webrtc-internals` during the spike. |
| Firefox | Constrained Baseline via OpenH264/platform. | Not a named target; do not optimize. |
| Wails WKWebView (macOS) | **Unproven.** Forum reports of `RTCPeerConnection` undefined are for Mac **Catalyst**; Tauri (wry) macOS reportedly works; nothing found for Wails. | **First spike item:** `typeof RTCPeerConnection`, H.264 receive, DataChannel, and whether the Local Network prompt names Portty (add `NSLocalNetworkUsageDescription` to `Info.plist` if so). If it fails, the Portty app opens the share link in the default browser (the click-to-share link path already exists). |

Viewer code is vanilla JS/TS using browser APIs only. Do **not** add `livekit-client`, `simple-peer`, `mediasoup-client` or similar.

---

## DMG self-install mechanics

All steps use absolute-path system tools through the existing `updater.Runner` (`ExecRunner`) seam, matching `Verify`/`Reveal`. No Sparkle (separate appcast and EdDSA scheme, ObjC framework, would duplicate the existing SHA-256 + Team ID + identifier + notarization verification). No new Go module.

### In-app update (replace `/Applications/Portty.app`, relaunch, delete DMG)

1. **Verify** (existing): codesign, Team ID, signing identifier, notarized `spctl`. Note `Verify` currently sets the `com.apple.quarantine` xattr on the DMG "so it behaves like a browser download" (Chesterton's Fence: that exists for the manual Reveal path). For self-install it propagates, see step 4.
2. **Mount:** `hdiutil attach -nobrowse -readonly -noverify -noautoopen -plist -mountpoint <private dir> <dmg>`. **Parse stdout only.** On this macOS 27.2.1, `hdiutil attach/create/detach` print `deprecated, use diskutil image ...` to **stderr**; `ExecRunner` uses `CombinedOutput()`, which would corrupt the plist. Add a stdout-only runner. `diskutil image attach --mountOptions nobrowse --readOnly --mountPoint` is Apple's replacement but is not present on older macOS 13/14; keep `hdiutil` and tolerate the warning. Convert plist to JSON with `/usr/bin/plutil -convert json -o - -` rather than adding a plist dependency.
3. **Stage next to the target on the same volume** (so the swap is an atomic rename): `ditto <mount>/Portty.app /Applications/.Portty-update-<rand>/Portty.app`. `ditto` preserves xattrs, resource forks and the code signature.
4. **Clear quarantine on the staged copy.** Verified experimentally: with a quarantined DMG, a `ditto` copy out of the mounted volume carries `com.apple.quarantine` on **every file** of the app. Left in place, the relaunch triggers a Gatekeeper "downloaded from the Internet" dialog that nobody can click on the unattended Mac. Since the DMG was already verified, run `xattr -dr com.apple.quarantine <staged app>` (what Sparkle does after validation).
5. **Verify the staged app:** `codesign --verify --deep --strict`, `spctl -a -t exec -vv` (expect `source=Notarized Developer ID`), and read `Info.plist` to confirm `CFBundleIdentifier == app.portty.Portty` and the version equals the release. Refuse on any mismatch.
6. **Swap atomically, from a detached finisher after the GUI exits:** `unix.RenamexNp(staged, target, unix.RENAME_SWAP)` (verified: `RenamexNp` and `RENAME_SWAP` exist in `golang.org/x/sys` v0.46.0). Rename keeps the old bundle's inodes valid for the **still-running old daemon**; never `ditto` over a live bundle, which can invalidate code pages and get the process killed. Then relaunch with `/usr/bin/open -n /Applications/Portty.app`, delete the swapped-out old bundle later (or next launch), `hdiutil detach <mount>` (retry `-force`), and delete the cached DMG with `os.Remove` (it is in Portty's own cache dir, not Downloads).
   - The finisher can be a hidden internal subcommand of the new binary (same precedent as `daemon --takeover`: "internal, not in the usage"), so the logic is Go and testable. It must be exempt from the "unknown leading flag is a usage error" rule (Phase 16) the same way `--takeover` is.
7. **Daemon continuity:** the LaunchAgent plist and `startDetachedDaemon` both resolve to `/Applications/Portty.app/Contents/MacOS/portty`, which keeps its path across the swap. The relaunched GUI sees a version mismatch and runs the v1.2 fd-passing handoff.
8. **Fallbacks, fail toward today's behavior:** if the target is not writable (not owned by the user or admin group), `RenamexNp` returns EPERM/EACCES, or the App Management check refuses, leave the verified DMG and fall back to the current Reveal in Finder flow. A failed update must never leave a missing app: stage, verify, then swap; keep the old bundle until the new one has launched.

**App Management (Ventura+, `kTCCServiceSystemPolicyAppBundles`)** [LOW-MED]: protects app bundles from modification by other developers' code; the same-Team-ID exemption is documented only in a 2022 secondary source, and one Apple forum thread (updater daemon, same Team ID, Sonoma 14.1+) reports it failing intermittently. Portty replaces itself with same-team code and a user-session process, which is the favorable case, but **prove it on a real Mac** and keep the Reveal fallback.

### First launch from the mounted DMG (move to /Applications, eject, trash)

- **Detect:** `os.Executable()` under a mount point listed by `hdiutil info -plist` (`system-entities[].mount-point`); the same JSON gives `image-path`, the source `.dmg` (Apple DTS: there is no public API; `hdiutil info -plist` is the sanctioned route). Verified: this Mac currently lists `~/Downloads/portty-v1.1.0-darwin-universal.dmg` as an image path.
- **Do:** same stage, clear-quarantine, verify, swap-or-move steps as above (if `/Applications/Portty.app` does not exist, a plain rename of the staged copy suffices); launch the installed copy with a flag naming the mount point and DMG path.
- **Eject and trash from the new instance**, not the one running off the volume: `hdiutil detach` fails while the old process executes from it. Trash the downloaded DMG with `NSWorkspace recycleURLs:completionHandler:` (ObjC, matching existing `.m` files; supports Put Back, no Automation prompt). Avoid `osascript tell Finder`, which asks for Automation consent.
- **Downloads folder TCC:** touching a file in `~/Downloads` may produce a one-time "access files in your Downloads folder" prompt for Portty. Treat a refusal as non-fatal (install still succeeded; leave the DMG).
- **Translocation:** a quarantined app launched from `~/Downloads` or the Desktop (not from a mounted DMG) runs from a randomized read-only `/AppTranslocation/` path. If `os.Executable()` contains `/AppTranslocation/`, do not attempt self-install; show "drag Portty to Applications". (`SecTranslocateCreateOriginalPathForURL` exists to recover the original path if this ever matters.)
- Run the "am I on a DMG" check only for GUI launches (no args or macOS launch args), never for CLI/daemon invocations.

---

## Remote unlock (feasibility notes that affect the stack)

**Verdict for planning: MAYBE at best, LOW confidence; do not put it on the v2.0 critical path.** Treat as a standalone spike after capture/input work.

- A user LaunchAgent in the **logged-in Aqua session** (everything above) is **not** the same as the login window. Lock-screen state versus login window state matters: a screen lock keeps the user's session; a fresh boot or logout shows the login window (and FileVault pre-boot cannot be driven at all by any app).
- Apple DTS (login-window capture thread): ScreenCaptureKit works pre-login on **macOS 14.4+** from an **agent** with `LimitLoadToSession = [Aqua, LoginWindow]`, which means installing under `/Library/LaunchAgents` (admin install, outside the current user-LaunchAgent model). Whether a Developer-ID helper can capture the *lock screen overlay inside an Aqua session* is not documented; no source found.
- Posting a password through `CGEvent` into the lock UI is not documented as supported, and a macOS 27 beta forum report shows the lock UI itself being unreliable (black screen after the second lock, fixed in beta 5 per the reporter).
- Third-party tools that do this (AnyDesk, Jump Desktop; per a forum poster's *hypothesis*) appear to use a root helper, a LoginWindow-scoped agent and the persistent-content-capture entitlement. That is a privileged-install architecture the corporate Mac may refuse and the user cannot recover remotely. [LOW]
- Cheap, in-scope substitute that **is** feasible: keep the display awake during a share (`IOPMAssertionPreventUserIdleDisplaySleep`) and wake it on viewer connect (`IOPMAssertionDeclareUserActivity`), and show "Mac is locked" as a state. Whether SCK returns frames, black frames or stops at the lock screen was not found in any source; test it.

---

## Installation

```bash
# Go (the only new direct module for the milestone)
go get github.com/pion/webrtc/v4@v4.2.29
go get github.com/pion/interceptor@v0.1.51   # imported directly for pkg/cc and pkg/gcc
go mod tidy                                  # expect only golang.org/x/time v0.12.0 -> v0.14.0 among existing deps

# Build flags for the new cgo package (declare in the .go file, not the shell)
#   #cgo darwin CFLAGS:  -fobjc-arc -mmacosx-version-min=13.0
#   #cgo darwin LDFLAGS: -weak_framework ScreenCaptureKit -weak_framework VideoToolbox \
#                        -framework CoreMedia -framework CoreVideo -framework CoreGraphics \
#                        -framework AudioToolbox -framework AppKit -framework ApplicationServices \
#                        -framework IOKit
# Info.plist: LSMinimumSystemVersion 13.0; NSScreenCaptureUsageDescription;
#             NSLocalNetworkUsageDescription (if the Wails viewer prompts)
# No npm packages, no Homebrew libraries, no new entitlements.
```

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| pion/webrtc v4.2.29 | pion/webrtc v5.0.0-rc.x | After v5 is stable and the DTLS 1.3 / stricter validation changes matter; avoid `SetNAT1To1IPs`/Plan B now so migration is cheap. |
| In-process cgo ObjC | Swift helper process | Only if a future need (e.g., root login-window capture for unlock) forces a separate privileged process anyway. |
| AudioConverter Opus | `hraban/opus` (libopus cgo) | If Apple's encoder cannot hit latency/bitrate needs; then build libopus universal in CI. |
| Constrained Baseline H.264 | High profile | After measuring text sharpness at equal bitrate and confirming Chrome/Safari/WKWebView decode High. |
| Browser WebRTC viewer | Native Go/ObjC viewer in the GUI | Only if WKWebView WebRTC fails and the browser-fallback UX is rejected. |
| `hdiutil` + `ditto` + `RenamexNp` | Sparkle | If Portty ever wants delta updates and appcast signing; not for this milestone. |
| System tools via `Runner` | `howett.net/plist` etc. | Never for a few `plutil` calls. |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| STUN/TURN, public ICE servers, any relay | Violates the tailnet-only constraint; also pion examples default to `stun.l.google.com` | Empty `ICEServers`, `SetIPFilter` to tailnet ranges |
| `SetNAT1To1IPs` | Removed in pion v5 | `SetIPFilter` plus binding the UDP mux to the tailnet IP |
| libopus / `hraban/opus` | Needs universal static libopus for the universal DMG build | AudioConverter Opus |
| `pion/opus` | v0.1.0 (2026-06-08) exposes a decoder only; no encoder in its API | AudioConverter Opus |
| `pion/mediadevices`, `go-vgo/robotgo`, ffmpeg bindings | Dependency weight for a few hundred lines; capture/encode need to be ScreenCaptureKit/VT specific | cgo ObjC package |
| `CGWindowListCreateImage` / `CGDisplayStream` | Deprecated; reports of gray images on macOS 27; ScreenCaptureKit is the supported path | ScreenCaptureKit |
| Rewriting the video through BGRA | Extra GPU/CPU copy | Ask SCK for `420v` |
| `ditto` over a running app, or `rm -rf` then copy | Can invalidate a live process's code pages; leaves no app on failure | Stage, verify, `RENAME_SWAP` |
| `osascript`/Finder AppleEvents to trash or quit | Triggers Automation consent prompts | `NSWorkspace recycleURLs`; process signals |
| `CombinedOutput()` for `hdiutil -plist` | Deprecation warnings on stderr corrupt the plist | stdout-only runner |
| Sparkle, Electron-updater, Homebrew | Duplicate the verification model; Homebrew is out of scope per PROJECT.md | Existing updater plus install steps |
| Trickle ICE and a custom signaling protocol | Unneeded with a single known host candidate | Non-trickle offer/answer over existing WSS |

## Stack Patterns by Variant

**If the daemon is not in an Aqua session (CLI-started over SSH):**
- Report capture unavailable and offer to start via the LaunchAgent; do not fall back to GUI capture silently.
- Because ScreenCaptureKit needs WindowServer access.

**If the corporate Mac enforces the monthly re-authorization (no persistent-content-capture entitlement):**
- Surface "capture permission expired" to viewers and send a notification; accept a reduced "available" guarantee.
- Because the prompt cannot be answered from home.

**If the Wails viewer cannot create `RTCPeerConnection`:**
- "Open in browser" with the share link (existing flow) for the Portty app viewer.
- Because the browser and iPhone paths use the same page and already pass the same CSP.

**If a viewer path cannot establish UDP (rare on a tailnet):**
- Consider `SetICETCPMux` (pion supports passive ICE-TCP) as a later fallback; tailscale already relays UDP via DERP when no direct path exists, so this is likely unnecessary.

## Version Compatibility

| Package | Compatible With | Notes |
|---------|-----------------|-------|
| `pion/webrtc/v4` v4.2.29 | Go 1.26.4 / toolchain go1.26.11 (repo), minimum go 1.24 | Resolved cleanly in a scratch copy of this repo's `go.mod`: only `x/time` moves. |
| `pion/webrtc/v4` v4.2.29 | `tailscale.com` v1.98.3, `wails/v2` v2.10.2, `coder/websocket` v1.8.15 | No version conflicts found. Keep the existing `GOTOOLCHAIN` pin in `build.sh` (tailscale JSON decoding vs Go 1.27). |
| pion `H264Payloader` | Annex-B input only | Needs SPS/PPS resent with every IDR; AUD/filler are dropped. |
| `TrackLocalStaticSample` | Real wall-clock `Duration` | RTP timestamp is derived only from `Duration`; a fixed-rate fake breaks A/V sync for variable-rate screen video. |
| ScreenCaptureKit audio | macOS 13+ | Raise `LSMinimumSystemVersion` from 10.13.0 to 13.0. |
| AudioConverter Opus | macOS 10.13+ | Verified on macOS 27.2.1. |
| `wails build -platform darwin/universal` | cgo code compiled for arm64 and amd64 | System frameworks exist for both; any static third-party lib would have to be universal. |
| macOS 26.1+ | Plain Unix executables missing from the Screen Recording pane (forum reports) | Always run the daemon from inside `Portty.app`. |
| macOS 27 | Separate "Remote Desktop" TCC entry; `SCShareableContent` alert reports (beta) | Re-test on the shipping 27.2.x. |

## Sources

- pkg.go.dev `github.com/pion/webrtc/v4` versions and GitHub releases API (v4.2.29 on 2026-11-22; v5.0.0-rc.4 same day) [HIGH, official]
- `github.com/pion/webrtc` v4.2.29: `go.mod`, `track_local_static.go` (`WriteSample`), `mediaengine.go` (default H264 registrations, RTCP feedback), `internal/fmtp/h264.go` (level ignored in match), `examples/bandwidth-estimation-from-disk/main.go` (cc/gcc registry pattern) [HIGH, source]
- `github.com/pion/rtp` v1.10.6 `codecs/h264_packet.go` (Annex-B input, STAP-A of cached SPS/PPS, AUD/filler dropped) [HIGH, source]
- pkg.go.dev `github.com/pion/opus` v0.1.0 (decoder only) [HIGH]
- Local experiments on macOS 27.2.1 arm64: `afconvert` Opus encode (48 kHz stereo, 960 frames/packet); `hdiutil attach/info -plist` including stderr deprecation warnings; `ditto` from a quarantined mounted DMG propagating `com.apple.quarantine`; scratch `go get`/`go mod tidy` against this repo's `go.mod`; `golang.org/x/sys` v0.46.0 `RenamexNp` and `RENAME_SWAP` [HIGH, first-hand]
- Repo reading: `internal/updater/install.go`, `internal/daemon/service.go`, `process_unix.go`, `internal/webserver` (CSP, tailscale/listener), `build.sh`, `release.yml`, `build/entitlements.plist`, `build/darwin/Info.plist` [HIGH]
- Apple Developer Forums (Quinn/DTS): login-window capture with SCK and agents (`LimitLoadToSession`, SCK pre-login on 14.4+); `hdiutil info -plist` as the supported image-path source; Accessibility vs Input Monitoring for posting events [MED]
- Apple Developer Forums: Tahoe 26.1 plain executables missing from Screen Recording list; macOS 27 "Remote Desktop" TCC entry and `SCShareableContent` alert; Persistent Content Capture MDM thread; App Management updater failure on Sonoma 14.1+; macOS 27 beta lock-screen black screen [LOW-MED, forum/beta]
- TidBits, MacRumors, Jamf community: Sequoia monthly screen-recording re-authorization, 15.1 reduction, `forceBypassScreenCaptureAlert` MDM key (single secondary source) [LOW-MED]
- W3C `webrtc-pc` issue #3109 (May 2026): macOS 26 Local Network prompt from `RTCPeerConnection` mDNS [LOW-MED]
- WebKit/webrtcHacks/IETF: Constrained Baseline mandatory (RFC 7742), Safari H.264 defaults [MED]
- MacDeck (MIT, reference only): ScreenCaptureKit + VideoToolbox (`AllowFrameReordering=false`) + CGEvent at `.cghidEventTap` [MED]
- `gsd-tools classify-confidence`: websearch/webfetch results are LOW unless cross-verified (MEDIUM); every LOW/MEDIUM item above that gates a phase is listed as a spike, not a fact.

---
*Stack research for: v2.0 Desktop Sharing and DMG self-install*
*Researched: 2026-11-24*
