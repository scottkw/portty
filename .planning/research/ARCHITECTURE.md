# Architecture Research: v2.0 Desktop Sharing + DMG Self-Install

**Domain:** macOS terminal app (Go daemon + Wails GUI) gaining WebRTC desktop sharing over the tailnet and in-app DMG self-install
**Researched:** 2026-11-24
**Confidence:** MEDIUM overall. Repo-derived integration facts are HIGH (read in code). macOS TCC/Aqua-session behavior and browser ICE-over-utun behavior are MEDIUM/LOW (thin Apple documentation, web-only sources) and are deliberately turned into spike exit criteria below.

Confidence legend: **HIGH** = read in this repo's code. **MEDIUM** = web source cross-checked or strong prior. **LOW** = single/unverified web source or inference; must be proven in the spike.

---

## 0. Corrections to the brief (read first)

1. **There is no tsnet in this codebase (HIGH).** `grep tsnet` over all `.go` files returns nothing. The share server (`internal/webserver/server.go` `listenTCP`/`StartOn`) does `net.Listen("tcp", "<tailnet-ip>:7444")` on the **OS Tailscale interface** (utun), takes the IP/MagicDNS name from the system tailscaled via `tailscale.com/client/local` (`webserver/tailscale.go` `CheckHealth`), and gets its TLS cert from `lc.GetCertificate`. Peers are dialed the same way (`daemon/peer_client.go` dials `peerIP:7444` with the cert verified against the MagicDNS name). So the "can pion use tsnet's PacketConn" question is moot: **pion uses ordinary `net.ListenUDP` bound to the tailnet IPv4 address**, exactly like the TCP listener. This removes the hardest networking risk the brief feared.
2. **The share server only exists while something is shared (HIGH).** `share.go` starts it on the first share and `unshareLocked` stops it when `len(a.shares)==0`. "Always-available" desktop mode breaks that invariant, and the handoff code (`handoff.go` `global()`, `finalize()`) hard-codes it. This is the biggest change to existing code (see section 6).
3. **Binary/process facts that drive the design (HIGH).** One Mach-O (`Portty.app/Contents/MacOS/portty`) is GUI, CLI and daemon, chosen by `launchMode` in `main.go`. The daemon is a `Setsid` child of whichever process ran `EnsureDaemon` (GUI poller **or a CLI call, possibly over SSH**), not a launchd job (`process_unix.go` `startDetachedDaemon`). The optional `portty daemon install` LaunchAgent (`kardianos/service`, `KeepAlive=false`) is not part of the normal path. `LSUIElement=true`; `LSMinimumSystemVersion` is 10.13; release is `darwin/universal` signed with `codesign --deep` (`release.yml:206`).

---

## 1. Standard Architecture

### System overview (target state)

```
 VIEWERS (any of the three speak the same protocol)
 ┌────────────────────┐ ┌────────────────────┐ ┌──────────────────────┐
 │ Wails app tab      │ │ Browser /app/?desk │ │ iPhone Safari (same  │
 │ (WKWebView, same   │ │ link (Chrome/Safari│ │ SPA, touch input)    │
 │ React DesktopView) │ │ /Firefox)          │ │                      │
 └───┬───────────┬────┘ └───┬───────────┬────┘ └───┬───────────┬──────┘
     │signal     │media     │signal     │media     │signal     │media
     │(loopback  │(UDP,     │(WSS)      │(UDP)     │(WSS)      │(UDP)
     │ proxy via │ direct)  │           │          │           │
     │ local     │          │           │          │           │
     │ daemon)   │          │           │          │           │
═════╪═══════════╪══════════╪═══════════╪══════════╪═══════════╪═══ tailnet (utun, WireGuard; DERP fallback is transparent)
     ▼           ▼          ▼           ▼          ▼           ▼
 HOST MAC (office)
 ┌──────────────────────────────────────────────────────────────────────┐
 │ portty DAEMON (Go; owns PTYs; NOT given Screen Rec/Accessibility)    │
 │  webserver (TCP 7444, TLS, tailnet IP)   internal/desktop (pion)     │
 │   /desktop/{id}        page+CSP           ICE UDP mux (udp4, tailnet │
 │   /desktop/{id}/ws     signaling WSS       IP:7444, host cand only)  │
 │   /api/peer/desktop/cap same-user cap     PeerConnection per viewer  │
 │   /api/sessions/meta   +X-Portty-Desktop  1 shared H.264 track       │
 │  capability: desktop HKDF key + grants    data channels: input/move/ │
 │  consent + viewer registry + quality ctl   clip/ctl;  Opus audio     │
 │  unix socket API (GUI/CLI)  ◄─ GUI binds: enable/disable/share/list  │
 └──────────────▲───────────────────────────────────────────────────────┘
                │ unix socket  ~/Library/Application Support/Portty/desktop-helper.sock (0600)
                │ framed: H.264 AUs + Opus in; control/input/clipboard out (helper dials the daemon)
 ┌──────────────┴───────────────────────────────────────────────────────┐
 │ PorttyDesktop HELPER (launchd LaunchAgent, gui/<uid>, in the .app)   │
 │  ScreenCaptureKit → VideoToolbox H.264 (Annex-B, SPS/PPS on IDR)     │
 │  SCK audio → Opus   CGEventPost input   NSPasteboard   lock/idle     │
 │  holds TCC: Screen Recording + Accessibility.  NO network sockets.   │
 └──────────────────────────────────────────────────────────────────────┘
 Wails GUI (menu-bar app): settings, permission status, share popover, viewer list,
 desktop tab (viewer only), updater/installer.   Terminal relay path is untouched.
```

### Component responsibilities

| Component | Responsibility | Why here |
|-----------|----------------|----------|
| **Helper** (`cmd/portty-desktop`, new, in-bundle) | Capture, encode, input injection, pasteboard, audio, lock/permission state | TCC identity + Aqua session + crash isolation (section 2) |
| **Daemon `internal/desktop`** (new) | pion PeerConnections, fan-out of one encode to N viewers, per-viewer permissions, quality policy, helper supervision | Reuses the daemon's tailnet listener, tokens, peer discovery and WhoIs gating |
| **Share server routes** (modified `internal/webserver`) | Auth + signaling transport, page, meta advertisement | Same listener/cert/origin/CSP stack as terminal shares |
| **`internal/capability` desktop token** (new file) | Separately-keyed desktop capability | Terminal cap must never open a desktop and vice versa |
| **GUI + `frontend/`** (modified) | Settings, permission UX, viewer tab, share/consent UI | Viewer is the existing React SPA |
| **Updater/installer** (modified `internal/updater`, new `app_install.go`) | Mount, verify, swap, relaunch, clean up | Runs in the GUI, hands sessions off via the existing takeover |

---

## 2. Key decision: which process captures/encodes/injects

**Decision: a dedicated helper process, registered as a LaunchAgent inside the signed `.app`; pion stays in the daemon.** Not the daemon, not the GUI.

Reasons, strongest first:

1. **Crash isolation protects terminal sessions (HIGH, project priority "rock-solid").** The daemon owns every PTY master fd. ScreenCaptureKit and VideoToolbox are cgo/ObjC with a record of stalls and error states. A SIGSEGV or `fatalerror` in cgo in the daemon ends every terminal session on a remote Mac nobody can log into. In a helper, launchd just restarts it.
2. **TCC attribution depends on how the process was spawned, and the daemon's spawn path is uncontrolled (MEDIUM).** Apple DTS: a launchd agent that is a helper embedded in an app is attributed to that app ("responsible code"), and the grant and alert name the app (forum threads 694948, 731504). A process spawned by Terminal, `sshd` or another app is attributed through the **responsible-process** chain, so a daemon respawned by `portty` typed over SSH would be checked against the wrong identity (Qt "responsible process" write-up; Apple forum 805245 shows grants not inherited by children). The daemon's spawn path today is "whoever called `EnsureDaemon`". A launchd-owned helper removes the variable.
3. **Aqua session (MEDIUM).** Capture needs the logged-in user's WindowServer session. A `gui/<uid>` LaunchAgent is in it by construction; an SSH-spawned child is in a background session. The unattended case (office Mac, daemon respawned from a remote shell) is exactly where it would silently fail.
4. **Least privilege.** The process holding Screen Recording + Accessibility has **no network listener**; the network-facing process (daemon) holds neither grant. Compromise of the daemon still needs the helper socket to reach capture/inject.
5. **Daemon stays unattended-capable.** GUI closed is fine: helper and daemon don't depend on the GUI.

Why not the GUI: it fails the "GUI closed" requirement. Why pion in the daemon rather than the helper: one place for tokens/WhoIs/consent/ICE-bind/handoff, helper remains a small replaceable "media device" (could be rewritten in Swift later with zero protocol change).

### Helper packaging (build, sign, register)

- Source: `cmd/portty-desktop/` Go `main` + cgo ObjC (`capture_darwin.m`, `encode_darwin.m`, `input_darwin.m`, `pasteboard_darwin.m`, `audio_darwin.m`). Matches the inherited stack (root package already compiles `.m` files). It is a **small separate binary** (no Wails/tailscale), not a second copy of the 60 MB+ `portty` Mach-O.
- Location: `Portty.app/Contents/Helpers/PorttyDesktop.app` (own `Info.plist`, `LSUIElement`, bundle id `app.portty.Portty.Desktop`), plus `Contents/Library/LaunchAgents/app.portty.desktop-helper.plist` with `BundleProgram`, registered by `SMAppService.agent(plistName:)` called from the GUI (small cgo ObjC wrapper). `BundleProgram` is bundle-relative, so it survives the DMG-to-/Applications move and in-place updates (hard-coded `ProgramArguments` paths, as `kardianos/service` writes, would not).
- Needs macOS 13+ (`SMAppService`; SCK audio capture is also 13+). Raise `LSMinimumSystemVersion` from 10.13 (**user decision needed: 13.0 vs 14.0**). Release build is universal, so the helper must be built for arm64+x86_64 and `lipo`'d; sign **inside-out** (helper, then app) instead of today's single `codesign --deep`.
- Start policy: helper runs while the desktop feature is on (Always-available) or while a click-to-share is active; the daemon starts it with `launchctl kickstart gui/<uid>/app.portty.desktop-helper` and it dials the daemon socket (daemon never dials out to the helper). Capture is **demand-driven**: SCStream starts on first viewer, stops on last (so the macOS purple recording indicator only shows during real use).
- **TCC entry name is unknown until spiked** (it may list "Portty" or "Portty Desktop"). So the **helper is the only authority on permission state**: it calls `CGPreflightScreenCaptureAccess()` / `AXIsProcessTrusted()` and reports over IPC; the GUI shows status and deep-links to `Privacy_ScreenCapture` / `Privacy_Accessibility`. The GUI/daemon never call those APIs themselves (they would answer for the wrong identity).
- Updating: after the app bundle is swapped (section 8), the running helper still executes the old inode. The daemon sends `quit` when idle and launchd restarts it on the new binary; protocol version in the helper `hello` mismatching triggers the same.

**Spike exit criteria for this decision (must pass before phase 19):** (a) grant lands on the expected TCC entry when the helper is launchd-started and the daemon was spawned from `ssh host portty ...`; (b) capture works with the GUI closed; (c) capture survives lock/unlock; (d) `launchctl procinfo` shows the helper as its own responsible process.

---

## 3. WebRTC over the existing tailnet path

### ICE: use the OS tailscale interface (HIGH for pion API, MEDIUM for browser behavior)

pion/webrtc v4.2.x (pkg.go.dev lists v4.2.29; a v5 major is tagged, verify before pinning). The relevant `SettingEngine` surface exists: `SetICEUDPMux`, `NewICEUDPMux(logger, net.PacketConn)`, `SetInterfaceFilter`, `SetIPFilter`, `SetNetworkTypes`, `SetICEMulticastDNSMode`, `SetNAT1To1IPs`, `SetLite`.

```go
// internal/desktop/ice.go (sketch)
udp, _ := net.ListenUDP("udp4", &net.UDPAddr{IP: tailnetIPv4, Port: 7444})   // same IP the share server bound
se := webrtc.SettingEngine{}
se.SetICEUDPMux(webrtc.NewICEUDPMux(nil, udp))                  // one fixed port, host candidates only
se.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeUDP4})
se.SetIPFilter(func(ip net.IP) bool { return tailscaleCGNAT.Contains(ip) })  // belt and braces = assertTailnetBindIP
se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)         // never resolve/advertise .local over a tailnet
cfg := webrtc.Configuration{ICEServers: nil}                    // no STUN, no TURN, no cloud
```

- **Tailnet-only is enforced by the bind**, not by trust: UDP socket on the 100.x address, plus the IP filter, plus `CheckListener`-style assertion (reuse `webserver.tailscaleCGNAT`). Never bind `0.0.0.0` (the macOS app firewall prompt and LAN exposure both follow).
- **No STUN/TURN needed:** Tailscale already does NAT traversal and DERP relaying below the interface; to ICE it is a flat network. A single host candidate is the whole candidate set.
- **UDP port: 7444/udp**, same number as the TCP share port. Default Tailscale ACLs cover both protocols; document that a restrictive ACL must allow `:7444` for udp (MEDIUM).
- **Non-trickle, viewer offers, daemon answers.** With one host candidate, gathering is instant; the viewer waits for `iceGatheringState==complete`, POSTs the offer over the signaling WS, the daemon returns the complete answer. Removes candidate-ordering races and half the signaling states.
- **Browser side (LOW, spike):** browsers hide their own host candidates behind mDNS names; the daemon (mDNS disabled) ignores them and learns the viewer as a **peer-reflexive** candidate from its first STUN check, which pion supports. That requires the browser to *gather a candidate on its utun interface*. Chromium's WebRTC classifies `utun*` as a VPN adapter (commit "Detect Tailscale interfaces as VPN", chromium:474435503) but nothing found says VPN interfaces are skipped; one discuss-webrtc thread reports stuck gathering on macOS with VPN. Must verify in Chrome, Safari, **WKWebView (Wails)** and iOS Safari during the spike. If a browser does not gather a utun candidate, advertising extra host addresses on the daemon side does not help; the only real fallback is a WSS-tunnelled media transport (a large change), so this is a go/no-go item.
- **Rebind on IP change:** the tailnet IP can change or vanish (tailscaled restart, `wakewatch`). `internal/desktop` re-reads `CheckHealth` on the same triggers the share server uses and rebuilds the mux; viewers see a drop and ICE-restart through signaling.

### Signaling: rides the share server's authenticated WebSocket (new routes, same middleware stack)

| Route | Gate (outermost first) | Purpose |
|-------|------------------------|---------|
| `GET /desktop/{id}` | `cspHeaders` → peer gate → desktop-cap page | Static viewer entry; redirects into `/app/?desktop={id}&cap=…` so the **one** SPA bundle serves terminal and desktop (see `lib/webMode.ts`) |
| `GET /desktop/{id}/ws` | `requireAllowedOrigin` → peer gate → desktop-cap | JSON signaling: `hello` → (`pending`/`approved{perms,displays}`) → `offer`/`answer` → `bye`; host-initiated `revoked`/`stopped` close reasons mirror `relay.CloseReason*` |
| `POST /api/peer/desktop/cap` | `requireSameUser` | Same-account peer gets the **always-available** token with nothing to type (mirror of `handlePeerCap`) |
| `GET /api/sessions/meta` | unchanged | Add response header `X-Portty-Desktop: always\|share` (do **not** change the JSON array shape: older peers parse a bare array, `tailnet/sessions.go`); `GET /api/desktop/info` for detail on demand |

After connect, everything except teardown rides data channels, so the WS can drop without killing media.

**Wails viewer signaling** must go through the local daemon, as terminal peer tabs do: the webview origin is `wails://…`, and the peer's `requireAllowedOrigin` demands `Origin == BaseURL` byte-for-byte. Add `GET /api/relay/remote-desktop/{id}/ws` to the relay loopback mux (Origin allowlist `relay.LoopbackOriginPatterns`) next to `handleRemoteSessionWS`; the daemon holds the desktop token server-side (extend `RemoteCapStore` or add a sibling store; no on-disk persistence, same invariant as `remote_caps.go`) and dials `wss://peer:7444/desktop/{id}/ws` with the verified `peerClient` (+ injected Origin). **Only signaling is proxied; media flows UDP directly webview → host** (WebRTC cannot be proxied this way). The WKWebView must expose `RTCPeerConnection` and decode H.264 receive-only without a capture prompt: forum reports of `RTCPeerConnection` being undefined in some Mac WKWebView contexts make this a **spike item (LOW)**. Fallback: viewer tab hosts the SPA in the Wails `AssetServer` with the WebKit preference enabled.

---

## 4. Capability/token model

Follow the Phase 13 **port-token precedent** (`capability/porttoken.go`): a separately keyed token so one type can never authenticate as another.

- New `capability/desktoptoken.go`: `DesktopClaims{ID, Kind ("always"|"share"), Perms ("view","control","clipboard","audio"), IAT, GrantID, V}`; key = `hkdf(sessionKey, info="portty/desktop-token/v1")`. **Reason is concrete:** `capability.Verify` ignores unknown fields and the session routes only check `sid`/grant, so a desktop token signed under the session key could open `/sessions/{id}/ws`, a **shell**. The HKDF split makes `Verify` fail on desktop tokens and `VerifyDesktop` fail on session tokens. Regenerate-key (panic button) kills both.
- Grant registry in `internal/desktop` (not in `a.shares`: desktop is not a session). `webserver` gets `AddDesktopGrant/ClearDesktopGrants/isDesktopGrantActive` + revoke signals mirroring `AddGrant/ClearGrants/revokeSignal`.

| | **Always-available (unattended)** | **Click-to-share** |
|---|---|---|
| Who may connect | Same Tailscale user only: `requireSameUser` (WhoIs numeric user, refuses tagged/shared-in nodes) **plus** token | Any non-tagged tailnet node with the link: new `requireTailnetPeer` (WhoIs must resolve to a node; ACLs still apply) **plus** token |
| Token | Standing; `GrantID` **persisted** in daemon settings so the iPhone Home Screen link / bookmark survives daemon restart, upgrade and respawn (terminal shares do not persist; unattended desktop must). Revoked by disabling the mode or regenerate-key | Ephemeral random `GrantID`, memory only, dies with the share, daemon restart, or Stop |
| Consent | None (unattended by definition); notify host on connect | **Per-viewer host approval** on first connect: daemon uses WhoIs to show "Alice (alice@corp), Alice's MacBook wants to view your screen" in the GUI (menu-bar notification + sheet), defaults **view-only**, host may grant control/clipboard per viewer; `pending` state held in signaling until approved/denied/timeout |
| Browser/iPhone link | "Copy link / QR" in the app, standing | Generated per share |
| Perms | Full | Chosen by host per viewer |

- **Widening MOB-05 is scoped to desktop-share routes only.** `requireSameUser` stays on every terminal route, `/app/` stays same-user-gated for terminal links. The desktop SPA entry for **share** links needs the peer gate instead of same-user; `/app/` static bundle is shared, so give the desktop page its own route (`/desktop/{id}`) and let it load the bundle assets (`/assets/` is already open) rather than loosening `/app/`. Add a regression test that a share-kind token on `/sessions/*`, and a session cap on `/desktop/*`, are both 401.
- Re-check on **every WS upgrade and every ICE restart** (WhoIs again, grant active), and close PCs on revoke. Per-viewer permissions are enforced in the daemon before anything reaches the helper (the helper trusts the daemon).
- Host-side indicators are mandatory for share mode: persistent "Your screen is shared with N" in the tray and window, one-click Stop (also `portty desktop stop`), and macOS's own recording indicator.

---

## 5. CSP

**Expected delta: none (MEDIUM).** Current policy: `default-src 'none'; script-src 'self' 'wasm-unsafe-eval'; connect-src 'self' wss://<host>; img-src 'self' data: blob:; worker-src 'self'; manifest-src 'self'; …` (`csp_mw.go`, Amendment 4).

- Signaling is the existing `wss://<host>` → inside `connect-src`.
- `RTCPeerConnection` with `iceServers: []` has no URL to check (the WebRTC spec ties `connect-src` to ICE server URLs only); UDP media is not a CSP-governed fetch.
- `<video srcObject=stream>` does not fetch a URL, so `media-src` (falls back to `default-src 'none'`) is not consulted. **No blob: video, no MediaSource.**
- Keep `'unsafe-eval'` absent; do not use WebCodecs/canvas rendering (that would pull in workers/blob).
- Process: add a **Chromium e2e + Safari manual check** to the existing CSP harness (`browser_csp_e2e_test.go`) asserting zero CSP violations on a desktop session, and a unit test `TestCSPHeaders_DesktopDelta` pinning "no new directive". If Safari insists, the single permitted addition is `media-src 'self'` (document as Amendment 5). The Wails page has no CSP.
- Clipboard (`navigator.clipboard`) and Fullscreen are Permissions-Policy/user-gesture matters, not CSP. Wails WKWebView clipboard needs a user gesture; use Wails runtime clipboard as the fallback.

---

## 6. Share-server lifetime and the upgrade handoff

**Problem (HIGH):** `unshareLocked` stops the server at `len(a.shares)==0`; `handoff.go` `global()` passes the share listener only `if ws != nil && len(g.Shares) > 0`; `finalize()` calls `dropShares()` when no imported session was shared; `shareOn` is the only starter. Always-available desktop needs a live server with no terminal share.

**Decision:** keep **one** server/port/origin (separate port would force a second probe, second cert warm-up and a second origin for peers) and generalize its lifetime:

- `serverNeeded() = len(a.shares)>0 || desktop.AlwaysOn() || desktop.ShareActive()`; `unshareLocked` consults it instead of `len(a.shares)==0`. Desktop enable/disable calls the same `startShareServer`/`startMu` path (lock order startMu → a.mu preserved).
- `handoff.go` touch points: `global()` passes the listener whenever `ws != nil`; `finalize()` keeps `shareLn` when present and the new daemon's desktop settings say always-on (even with zero shares); `commit()` starts the server and **then** desktop; `closeListenerCopies` unchanged. `hsGlobal` gains an optional field; because frames are JSON-ish structs, an older old-daemon simply omits it, so **do not bump `HandoffVersion`** (a bump would force the DEBT-08 consent path for every upgrade).
- **Desktop sessions are not handed off.** The UDP mux is closed by the old daemon at `finish` and opened by the new daemon after COMMIT; the helper keeps running, redials the new daemon's `desktop-helper.sock` with backoff; viewers see a drop and auto-reconnect (signaling WS close → SPA re-offers). The UDP bind is new on the new side, so retry the bind briefly until the old daemon exits (same shape as share `port_in_use` handling). Persisted always-grant means links survive.
- **Must not regress terminal handoff:** all desktop work after COMMIT runs in a goroutine started after `serveOn` and never blocks `commit`; any desktop failure logs and leaves terminals untouched (hard rule: a desktop fault never changes a handoff outcome code). Extend `handoff_it_test.go` and the VM two-build chain (M-155 style) with a case "always-on desktop, zero shares" and "desktop session active during upgrade".
- Freeze gate: desktop mutating routes use `enterWrite()` like share routes so a frozen daemon answers 503 `frozen` rather than racing the handoff.

---

## 7. GUI: the desktop tab

- `Tab.type` gains `'desktop'` (`TabBar.tsx`); id `desktop:<host>`; icon `ComputerDesktopIcon`; fields reuse `peerHost`, `baseURL`, `capToken`-less (token stays in the daemon for Wails).
- Sidebar: each reachable same-user peer group gets a **Desktop** row when its meta response carries `X-Portty-Desktop` (parsed in `tailnet/sessions.go` into `PeerSessionMetaGroup.Desktop`; `lib/sidebarPeers.ts` renders it). Click → binding `ConnectPeerDesktop(host)` → daemon `POST /peers/desktop` (twin of `POST /peers/connect`, calls `/api/peer/desktop/cap`) → tab opens `DesktopView` pointing at the loopback signaling proxy.
- `frontend/src/components/DesktopView.tsx` + `lib/desktopClient.ts` (RTCPeerConnection, signaling, data channels), `lib/desktopInput.ts` (pointer/wheel/key mapping), shared by Wails, browser and iPhone. Mounted like terminal panels (kept mounted when the tab is hidden so a tab switch does not renegotiate).
- Browser/iPhone: `webMode.ts` gets a `desktopId` param; `detectMode` still `'web'`. iPhone: touch to pointer (tap, long-press = right click, two-finger scroll, pinch zoom), hidden input for the soft keyboard; autoplay policy means audio starts muted with an unmute control.
- Host side: Settings section "Desktop Sharing" (Always-available switch, permission status rows from helper, display picker, copy link/QR), share popover entry "Share my desktop…" (display/window picker built from the helper's `list` reply, not `SCContentSharingPicker`, which needs a UI-bearing process and per-use user pick), viewer list with Disconnect, tray state. Optional `portty desktop status|on|off|share|stop` CLI (same pattern as `cmd_share.go`; `launchMode` already routes subcommands to the CLI).
- Keyboard pitfalls to design for: Cmd-chords are intercepted by the Wails menu/browser (Cmd+W/Q/T); use the Keyboard Lock API where available (Chromium, fullscreen) and a "send to remote" capture mode in the Wails tab.

---

## 8. DMG self-install (independent of desktop except one coupling)

Lives in the **GUI process** (updater service is GUI-owned: `app_update.go`), not the daemon. Existing flow `available → downloading → verifying → revealed`; add `installing → relaunching`, keeping reveal-in-Finder as the fallback for every failure (M-93 path stays valid).

**Update flow**
1. Reuse verified cached DMG (digest + codesign + Team ID + identifier + notarization already done in `updater.Verify`).
2. `hdiutil attach -nobrowse -readonly -noverify -mountpoint <private dir>`; locate `Portty.app`; verify the **app inside**: `codesign --verify --deep --strict`, TeamIdentifier, identifier `app.portty.Portty`, `spctl -a -t exec`, and version ≥ running (downgrade guard), nested helper signed.
3. Copy with `ditto` to a sibling temp dir **on the same volume** (`/Applications/.Portty-update-XXXX/Portty.app`), then **atomic directory swap** (`renamex_np(RENAME_SWAP)` or rename old aside → rename new in). **Never overwrite files in place:** the running daemon/GUI are executing the old inodes, and an in-place write invalidates the code signature and macOS SIGKILLs them (the terminal sessions with them). A whole-bundle rename leaves old inodes valid.
4. If `/Applications/Portty.app` is not writable (root-owned): stop and fall back to reveal.
5. Relaunch the **new** app with LaunchServices (`NSWorkspace` `createsNewApplicationInstance`, or `open -n`) from a small detached relauncher, then quit the old GUI with the normal GUI-only quit (daemon keeps running, UPGR-05). Do **not** pass a new CLI flag: `launchMode` (Phase 16) makes any unknown leading flag a usage error by design, so use a **marker file** (`pending-install.json` in the config dir: dmg path, mountpoint, old version) read at startup.
6. The new GUI's startup `EnsureDaemon` sees `StaleVersion` and runs the existing **takeover handoff**; sessions survive with zero new daemon code. Verify the takeover path uses the new bundle's executable (it does: `os.Executable()` of the new process).
7. New instance finishes the marker: `hdiutil detach`, delete the cached DMG, restart the desktop helper (the one coupling) once the daemon is idle on the new build, re-link the CLI symlink if it pointed at an old path (`app_cli_install.go` resolves via `EvalSymlinks`, so a link into `/Applications/Portty.app` keeps working; one made from a DMG path does not).

**First launch from the mounted DMG**
- Detect: bundle path under `/Volumes/` (read-only image) or an `AppTranslocation` path. Offer "Move to Applications" (native dialog, same pattern as `a.dialog`), copy/swap as above (if an older Portty exists in `/Applications`, treat as update; refuse a downgrade), write the marker including the **DMG source file and mount**, launch the installed copy, quit. The installed copy ejects the volume and **trashes** (`NSFileManager trashItemAtURL`, recoverable) the user's downloaded DMG; the updater's own cache DMG is simply deleted.
- Eject only works after the first instance exits (it has files open on the volume), hence the new-instance finish step. The daemon must never be started from the DMG path: skip `EnsureDaemon` and Desktop registration while running from a read-only volume, or the daemon pins the volume and, worse, would be the old-version daemon the handoff then has to replace.

**Dependency note for roadmap:** because the helper lives inside the bundle, self-install must ship before (or together with) the first release that contains the helper; otherwise the first helper-bearing update is the one that cannot restart it safely.

---

## 9. Data flow

### Viewing a desktop (always-available, Wails tab)

```
click "Desktop" on peer row
  GUI binding ConnectPeerDesktop ─► local daemon POST /peers/desktop
     └─► peer :7444 POST /api/peer/desktop/cap   (requireSameUser, WhoIs)  ─► desktop token (kept in daemon)
  DesktopView ─► ws://127.0.0.1:<relay>/api/relay/remote-desktop/{id}/ws   (loopback, Origin allowlist)
     └─► daemon dials wss://peer:7444/desktop/{id}/ws?cap=…   (Origin = peer BaseURL; peer client verified)
  viewer: createOffer(recvonly video/audio + 3 data channels) → gather (1 host cand) → send offer
  host daemon: verify (origin→WhoIs→desktop cap→grant) → pion answer → ICE over udp4 100.x:7444 (direct, webview→host)
  daemon ► helper: StartCapture{display|window, bitrate}   helper ► daemon: Annex-B AUs (+Opus)
  daemon: TrackLocalStaticSample.WriteSample → SRTP to every viewer PC
  viewer input: "move" (unordered, maxRetransmits 0) / "input" (ordered reliable) channel
     → daemon validates perm + rate + epoch/display id → helper → CGEventPost
```

### Adaptive quality ("sharp when still, smooth when moving")
- One encode shared by all viewers. pion `interceptor/pkg/cc` (GCC) + TWCC extension per PC; **daemon target bitrate = min over viewers' estimates** (floor/ceiling clamps), pushed to the helper as `SetBitrate`. PLI/FIR from any viewer → one rate-limited `RequestKeyframe` (≥500 ms apart); a new viewer triggers an IDR.
- Helper policy (it sees the pixels): SCK only emits frames on change; low-latency realtime VT session (no B-frames, no reorder, long GOP) while moving; after ~300 ms idle emit one high-quality refresh frame. Resolution/FPS caps come from the daemon's estimate and viewer window size.
- SPS/PPS must be re-emitted with every IDR (display switch changes resolution mid-stream; browsers handle a new SPS at an IDR). Advertise H.264 packetization-mode=1 and offer both Constrained Baseline (`42e01f`) and High (`640c1f`) so SDP picks per viewer; verify per browser (LOW).

### Display/window switch
Viewer (or host) sends `select{displayID|windowID}` on the control channel → daemon checks perm → helper `updateContentFilter` (no renegotiation), bumps an **epoch** carried in input messages so a mouse event aimed at the old display is dropped. Click-to-share: only the host may switch unless granted.

### Clipboard / audio / lock
- Clipboard: helper watches `NSPasteboard.changeCount`; text only (size cap ~1 MiB) both ways over the reliable channel, gated by the `clipboard` perm; images deferred.
- Audio: SCK system audio (13+) → Opus. Opus encode path is **LOW**: either AudioToolbox `kAudioFormatOpus` via `AudioConverter` (no dependency, needs spike) or a vendored static libopus in the helper. Decide in the spike; do audio last.
- Lock/unlock: helper reports `locked` (distributed notification/`CGSessionCopyCurrentDictionary`) and permission-lapse states to the viewer over `ctl`. **Remote unlock needs no extra component**: if `CGEventPost` reaches the lock screen and SCK still delivers frames, it is ordinary typing; reports say synthetic input works on a locked session but not on the logged-out login window (single LOW source). The spike must test both facts on a real Mac before any unlock UI is planned. **Never store a password in Portty.** Declare "wake display on viewer connect" via `IOPMAssertionDeclareUserActivity` (pairs with `wakewatch`).

---

## 10. New vs modified components

### New

| Path | What |
|------|------|
| `cmd/portty-desktop/` (+ `.m` files) | Helper: SCK, VideoToolbox, CGEvent, NSPasteboard, audio, permissions, lock state; dials daemon socket |
| `internal/desktopproto/` | Wire protocol shared by daemon and helper (frame types: `hello`, `caps`, `start`, `stop`, `bitrate`, `keyframe`, `select`, `list`, `video`, `audio`, `input`, `clipboard`, `state`; version byte) |
| `internal/desktop/` | `service.go` (state, grants, viewers, consent), `ice.go` (UDP mux), `rtc.go` (PeerConnection per viewer, track fan-out), `signal.go`, `helperlink.go` (socket server + `launchctl kickstart` supervision), `quality.go`, `input.go` (validation, key map) |
| `internal/capability/desktoptoken.go` | HKDF-keyed desktop token |
| `internal/daemon/desktop_api.go`, `peer_desktop.go` | Socket routes (`/desktop/*`: enable, disable, share start/stop, list, status, viewers, kick), `POST /peers/desktop`, loopback signaling proxy |
| `internal/webserver/desktop_routes.go` | `/desktop/{id}`, `/desktop/{id}/ws`, `/api/peer/desktop/cap`, `/api/desktop/info`, `requireTailnetPeer` |
| `app_install.go` + `internal/updater/mount.go` | Mount/verify/swap/relaunch/cleanup, first-launch move |
| `frontend/src/components/DesktopView.tsx`, `DesktopSharePopover.tsx`, `lib/desktopClient.ts`, `lib/desktopInput.ts` | Viewer and host UI |
| `build/darwin/helper/` (Info.plist, LaunchAgent plist) | Bundle assets |

### Modified

| Path | Change |
|------|--------|
| `internal/daemon/share.go` | `serverNeeded()` lifetime predicate; `wireShareServer` wires desktop; `unshareLocked` no longer stops on `len(a.shares)==0` alone |
| `internal/daemon/handoff.go` | `global()`, `finalize()`, `commit()` as in section 6; optional `hsGlobal` field; no version bump |
| `internal/daemon/api.go` | Register desktop socket routes (socket mux only) and the relay-mux signaling proxy; settings load/save for desktop state |
| `internal/daemon/remote_caps.go` | Desktop token store or sibling store |
| `internal/webserver/server.go`, `whois.go`, `capability_mw.go` | Routes, `requireTailnetPeer`, desktop verify; meta response header |
| `internal/tailnet/sessions.go` | Parse `X-Portty-Desktop` into the peer group |
| `internal/updater/service.go`, `install.go`, `app_update.go` | New states and install steps; reveal fallback kept |
| `app.go`, `tray.go`, `frontend/src/App.tsx`, `TabBar.tsx`, `Sidebar.tsx`, `SettingsSheet.tsx`, `SharePopover.tsx`, `lib/webMode.ts`, `lib/sidebarPeers.ts` | Bindings, desktop tab, settings, discovery |
| `build/darwin/Info.plist`, `build.sh`, `.github/workflows/release.yml` | Min OS bump, build helper (universal), inside-out signing, notarize unchanged |
| `TESTING.md` (+ `tests/check-traceability-paths.sh`) | Project standing rule: new suites, M-NN manual items (TCC prompts, real displays, remote unlock, DMG on a real Mac), traceability rows |

### Deliberately unchanged
`internal/relay` (terminal hub/replay, input queue), `internal/fdpass`, `launchMode`, the terminal share routes and their same-user gating, the CSP (expected).

---

## 11. Suggested build order (de-risk first)

1. **Spike A: capture to browser (timeboxed, throwaway shell around kept pieces).** Helper skeleton (SCK → VT H.264 Annex-B → unix socket) → daemon-side pion with the UDP mux bound to the tailnet IP → one HTML page. Exit criteria: the four helper/TCC items in section 2; H.264 plays in Chrome, Safari, WKWebView (Wails) and iOS Safari; ICE connects over utun in each (peer-reflexive path); latency and CPU on a **real** Mac (a Tart VM may only have software H.264 and gives non-representative numbers; use VM rows only for connectivity); lock-screen frame/input behavior; behavior of the macOS 15+ monthly screen-recording re-approval for a SCK helper; Opus path choice. Real-tailnet measurements go to VM rows, never the host tailscaled (project rule).
2. **DMG self-install** (phase of its own, low risk, independent, and prerequisite for shipping a bundle-embedded helper safely). Includes the real two-build upgrade check in the VM.
3. **Desktop core:** productionize helper (supervision, version handshake, permission reporting), `internal/desktop` single viewer, desktop token, **share-server lifetime + handoff changes with tests**, signaling routes, browser viewer for Always-available same-user, Settings + permission UX, CSP e2e. Highest-leverage phase for correctness; it is where terminal-handoff regressions would be introduced.
4. **Control:** input (key map, pointer, scroll, epoch/display id), clipboard, display/window switch.
5. **Wails desktop tab:** loopback signaling proxy, `POST /peers/desktop`, sidebar discovery header, tray.
6. **Click-to-share + tailnet people:** `requireTailnetPeer`, ephemeral grants, consent sheet, per-viewer perms, indicators, security tests (cross-token matrix).
7. **Multi-viewer, adaptive quality, audio:** shared track fan-out, GCC min-of-viewers, refine-on-idle, Opus.
8. **iPhone touch viewer.**
9. **Hardening:** two-build upgrade with an active desktop session, helper restart after update, sleep/wake and display-wake, monthly re-approval UX, optional remote-unlock only if the spike proved both required facts.

Ordering rationale: spike first because its answers (helper vs in-daemon, ICE over utun in WKWebView, TCC attribution) can invalidate the architecture; self-install second because it is independent and de-risks shipping the bundle; the token model's *types* (kind, perms) land in step 3 so click-to-share (step 6) adds policy, not a schema change; the Wails tab waits until the protocol is stable because it adds the proxy hop.

---

## 12. Patterns to follow

1. **Derived-key token per capability type** (precedent: `porttoken.go`). Next types do the same.
2. **Fail toward the old behavior in handoff** (precedent: Phase 15 checks): any desktop error during takeover logs and is dropped; terminals win.
3. **Authority at the edge that knows the truth:** permission state from the helper, tailnet IP from `CheckHealth`, identity from WhoIs. No cached assumptions.
4. **Demand-driven media:** capture, encoder and indicator exist only while a viewer is attached.
5. **One SPA, three viewers:** desktop viewer is a component in the existing bundle; `webMode` selects it.
6. **Server-side secrets for the app viewer:** the Wails webview never sees the desktop token (same as remote terminal caps).

## 13. Anti-patterns to avoid

| Anti-pattern | Why bad | Instead |
|--------------|---------|---------|
| Capture/encode (cgo) inside the daemon | A crash kills every PTY; TCC attribution depends on spawn path | Helper process |
| Signing desktop tokens with the session key | A desktop token could pass `verifyCapability` on `/sessions/{id}/ws` (shell) | HKDF-separate key |
| Loosening `requireSameUser` globally for click-to-share | Reopens terminal routes to other accounts (MOB-05) | New peer gate on desktop routes only |
| Binding the UDP mux to `0.0.0.0` | LAN exposure, app-firewall prompt, breaks "tailnet only" | Bind tailnet IPv4 + IP filter |
| STUN/TURN servers | Violates "no cloud" and is unnecessary | `ICEServers: nil` |
| Trickle ICE | More states for one candidate | Non-trickle, viewer offers |
| Changing `/api/sessions/meta` array shape | Older peers' parsers break | Response header + separate info route |
| Overwriting the `.app` in place | Invalidates running binaries' signatures; SIGKILL | Whole-bundle atomic rename |
| New GUI launch flag for post-install | Breaks Phase 16 invariant | Marker file |
| Per-viewer encoders | CPU/thermal on an Air | One encode, min-of-viewers bitrate |
| Forwarding raw viewer events to CGEvent | Remote synthetic input is shell-equivalent | Typed, rate-limited, perm-checked, epoch-checked events |
| Storing a Mac password for unlock | Credential at rest on the machine being protected | Viewer types it live |
| Hard-coded `ProgramArguments` path in a LaunchAgent | Breaks after the DMG move or update | `SMAppService` + `BundleProgram` |

## 14. Scalability

| Concern | 1 viewer (typical) | 3 to 5 viewers | Note |
|---------|-------------------|----------------|------|
| Encode | one VT hardware session | same session | Shared track |
| Egress | 2 to 8 Mbps (adaptive) | N x that (SRTP per PC) | Tailnet LAN or DERP |
| Bitrate policy | that viewer's GCC | min over viewers | Slowest viewer limits all; acceptable at this scale |
| Daemon CPU | RTP packetize/encrypt trivial | linear, still small | No simulcast needed |
| Helper | demand-driven | same | Idle = no capture |

Realistic ceiling is a handful of viewers on one Mac; do not design simulcast/SFU.

## 15. Integration points

### External services

| Service | Pattern | Notes |
|---------|---------|-------|
| System tailscaled (LocalAPI) | `CheckHealth`, WhoIs, `GetCertificate` | Already used; the only network authority. No tsnet |
| macOS TCC / SCK / VideoToolbox / CGEvent / NSPasteboard | In the helper only | Monthly re-approval on macOS 15+ is a standing unattended-mode risk (single-source reports of SCK apps being prompted too) |
| launchd / `SMAppService` | Helper registration; user may need to approve in Login Items | Approval state surfaced in Settings |
| GitHub Releases | Existing updater | Unchanged |

### Internal boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| helper to daemon | Unix socket `desktop-helper.sock` 0600, helper dials, versioned frames | Socket only, never the relay port (browser-reachable) |
| daemon `desktop` to webserver | Narrow interface (grant registry, signaling callbacks) as `SetShareTokenProvider` does | Avoid daemon-webserver-daemon import cycle (existing note in `server.go`) |
| webview to daemon | Loopback relay mux + Origin allowlist | Signaling only |
| desktop service to share lifetime | `serverNeeded()` | Only coupling into `share.go`/`handoff.go` |
| updater to daemon | None new; existing `EnsureDaemon` takeover | Helper restart is the only post-swap action |

## 16. Open questions for phase-level research

| Question | Where it is answered | Confidence now |
|----------|----------------------|----------------|
| Which TCC identity does the launchd helper get (app vs helper bundle), and is the SSH-respawned-daemon path actually mis-attributed? | Spike A | LOW/MEDIUM |
| Do Chrome/Safari/WKWebView/iOS gather and use a utun candidate with mDNS obfuscation? | Spike A | LOW |
| Is `RTCPeerConnection` + H.264 recv available in Wails' WKWebView on target macOS? | Spike A | LOW |
| Opus: AudioToolbox vs vendored libopus | Spike A / phase 7 | LOW |
| Does SCK deliver frames on the lock screen; does `CGEventPost` reach it? | Spike A | LOW |
| Monthly screen-recording re-approval cadence and any supported suppression for an unattended SCK helper on the user's current macOS (dev machine reports a newer Darwin than the research covers) | Spike A + phase 9 | LOW |
| Minimum macOS: 13 or 14 | User decision | n/a |
| pion v4 vs v5 | Pin at phase 3 start | MEDIUM |

## Sources

- Repo (HIGH): `internal/webserver/{server,whois,capability_mw,csp_mw,origin_mw,tailscale}.go`, `internal/daemon/{share,handoff,process,process_unix,api,remote_ws_proxy,remote_caps,peer_client,service}.go`, `internal/capability/{capability,porttoken}.go`, `internal/updater/{install,service}.go`, `app_update.go`, `app_cli_install.go`, `main.go`, `build/darwin/Info.plist`, `.github/workflows/release.yml`, `frontend/src/components/{TabBar,WebShareSessionView}.tsx`, `frontend/src/lib/webMode.ts`.
- [Apple DTS: Screen capture with LaunchAgent (embedded helper attributed to enclosing app)](https://developer.apple.com/forums/thread/694948) (MEDIUM)
- [Apple Developer Forums: What is a responsible process?](https://developer.apple.com/forums/thread/731504), [TCC inheritance failure, Swift parent to child](https://developer.apple.com/forums/thread/805245), [Qt: the curious case of the responsible process](https://www.qt.io/blog/the-curious-case-of-the-responsible-process) (MEDIUM)
- [9to5Mac](https://9to5mac.com/2024/08/14/macos-sequoia-screen-recording-prompt-monthly/), [TidBITS](https://tidbits.com/2024/08/19/apple-reduces-excessive-sequoia-permission-requests-shifts-to-monthly): Sequoia monthly screen-recording prompt, applies to SCK apps per developer reports (MEDIUM); [launch-agent re-prompt report](https://developer.apple.com/forums/thread/762532) (LOW)
- [pion/webrtc v4 on pkg.go.dev](https://pkg.go.dev/github.com/pion/webrtc/v4): SettingEngine method list (MEDIUM); [ice-single-port example / UDP mux notes](https://pkg.go.dev/github.com/pion/webrtc/v3@v3.1.7/examples/ice-single-port), [pion/ice issue 400: UDPMux host candidates only](https://github.com/pion/ice/issues/400) (MEDIUM); [pion/interceptor gcc/cc packages](https://pkg.go.dev/github.com/pion/interceptor@v0.1.12/pkg/gcc) (MEDIUM)
- [WebRTC: Detect Tailscale interfaces as VPN](https://webrtc.googlesource.com/src/+/6f37672d35); [Chrome mDNS host-candidate obfuscation PSA](https://groups.google.com/g/discuss-webrtc/c/4Yggl6ZzqZk) (MEDIUM for classification; consequence for gathering is LOW)
- [Apple forums: RTCPeerConnection undefined in Mac WKWebView](https://developer.apple.com/forums/thread/695871) (LOW; spike)
- [Apple forums: keyboard/mouse simulation in macOS pre-login window](https://developer.apple.com/forums/thread/62687) and related lock-screen threads (LOW)

---
*Architecture research for: Portty v2.0 Desktop Sharing + DMG self-install*
*Researched: 2026-11-24*
