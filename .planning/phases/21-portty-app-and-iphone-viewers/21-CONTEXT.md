# Phase 21: Portty App and iPhone Viewers - Context

**Gathered:** 2026-11-25
**Status:** Ready for planning

<domain>
## Phase Boundary

Open the office Mac's desktop in a Desktop tab of the Portty app on the home Mac, straight from the sidebar, and on an iPhone with touch controls, using the same viewer and protocol as the browser link. In scope: a Desktop row for each same-account peer in the sidebar that opens a Desktop tab showing and controlling that desktop, with the same state and reconnect behaviour as the browser viewer (DVWR-01); control mode in the Desktop tab, where Cmd+W, Cmd+Q and Cmd+C go to the remote Mac and Portty gets its shortcuts back when control mode ends (DVWR-04); the iPhone viewer in Safari and from the Home Screen with trackpad mode, pinch-zoom, long-press right-click, two-finger scroll, the soft keyboard, video started from a tap, and recovery after the phone sleeps (DVWR-03); and the plumbing the app needs, namely peer desktop discovery, authorization for the always-available desktop, and a loopback signalling proxy from the web view to the peer. Not in this phase: the input, Send-keys, clipboard and display/window-switch protocol and their DesktopView controls (Phase 20, upstream); click-to-share for other tailnet people, admit, revoke and the indicator (Phase 22); audio and the unmute control, adaptive quality, unlock and release (Phase 23); direct-touch mode on iPhone (REQUIREMENTS Future).

</domain>

<decisions>
## Implementation Decisions

User delegation (2026-11-25): "I don't need to answer a bunch of questions". Reversible technical choices were decided autonomously, product and UX choices are under Claude's Discretion, and the five hard-to-reverse recommendations below were accepted by the user with "Accept all" (2026-11-25).

### Peer discovery and authorization
- The sidebar learns that a peer has an always-available desktop from a response header `X-Portty-Desktop: always` on the existing GET /api/sessions/meta. The JSON body stays a bare array, so v1.x peers still parse it, and no extra request is made per peer per poll. The header is sent only while always-available is on, Phase 22 may add a value, and readers ignore values they do not know — **Reversibility:** costly — it becomes a wire contract between peers on different versions, and every later build has to keep sending and parsing it.
- The Portty app gets the right to open a peer's always-available desktop through a new POST /api/peer/desktop/cap behind requireSameUser, the twin of handlePeerCap (whois.go). It returns the standing always-available desktop token to the peer's daemon, which keeps it in memory beside RemoteCapStore and never puts it on disk or in the web view URL. The WhoIs same-user check runs on this endpoint and again on /desktop/ws. Turning always-available off or Regenerate Signing Key kills the token. This matches how terminal peer tabs work today — **Reversibility:** costly — a cross-version peer API route that every later build serves, and it fixes where the standing desktop token may travel.

### Desktop tab in the Portty app
- The macOS Local Network prompt that WebRTC in the Portty web view triggers on the viewer Mac (R13: ICE stays at new until Allow; the grant survives a same-identity rebuild) is accepted on the first Desktop tab open. An NSLocalNetworkUsageDescription string in Info.plist explains why. If ICE stays at new past a timeout, the tab shows a short state with Open System Settings (Privacy & Security ▸ Local Network) and Open in Browser. No other grant is needed on the viewer Mac — **Reversibility:** costly — it adds a TCC entry and a System Settings row on every viewer Mac, and a denial can only be undone by the user in System Settings.

### Control mode (keyboard routing in the Desktop tab)
- While control mode is on, every key chord the Portty menu owns goes to the remote Mac: Cmd+Q, W, C, V, X, Z, A, T, H, M, comma, Ctrl+Cmd+S and the rest. Control mode ends when the user clicks outside the video, switches tabs, when Portty stops being the active app (Cmd+Tab stays local because the OS owns it), when the stream is not connected (Reconnecting or ended), or on one reserved release chord that is never forwarded. Every exit sends release-all so no remote key stays held (DCTL-02). There is no system-wide event tap — **Reversibility:** costly — it is a safety boundary over which local keys a remote session can swallow, and users learn the escape path.
- In control mode, a typed Cmd+V counts as the explicit paste action. The app sends the home clipboard's plain text (never an item marked concealed) through Phase 20's paste message, then the keystroke. If Phase 20 rules that a browser Cmd+V is not an explicit paste, the app follows Phase 20 and forwards a plain key — **Reversibility:** costly — it sets when clipboard data leaves the viewer Mac, which users and the DCTL-06 explicit-only rule depend on.

### Decided autonomously
- [Proxy] Q: "How does the Wails tab reach the peer's /desktop/ws?" → Selected: "A new GET /api/relay/remote-desktop/{host}/ws on the relay loopback mux (Origin allowlist relay.LoopbackOriginPatterns), a twin of handleRemoteSessionWS: it dials wss://peer:7444/desktop/ws?cap=<held token> with the verified peerClient, injects Origin equal to BaseURL, uses the long-lived request ctx and pings upstream; signalling only, media goes UDP straight from the web view to the peer" — a loopback-only route whose two ends ship in the same binary
- [Proxy] Q: "Where is the peer desktop token held?" → Selected: "A sibling of RemoteCapStore keyed by peer host, in memory only, covered by the existing no-disk source-grep test, refreshed through POST /peers/desktop on every connect" — an internal store
- [Binding] Q: "How does the click become a connection?" → Selected: "Sidebar row, then binding ConnectPeerDesktop(host), then daemon socket POST /peers/desktop (twin of POST /peers/connect: matchPeer, fetch the cap, store it), then open the desktop tab pointed at the loopback proxy URL; the web view URL never carries the token" — an internal API and binding
- [Discovery] Q: "Where is the header parsed?" → Selected: "tailnet/sessions.go doFetchSessionsMeta sets PeerSessionMetaGroup.Desktop; App.GetRemoteSessionsWithMeta and RemotePeerSessions carry it; lib/sidebarPeers.ts renders the row" — internal fields
- [Tab] Q: "How is the tab typed?" → Selected: "Tab.type gains 'desktop' (TabBar.tsx, lib/tabs.ts) with peerHost and name and no token; it is mounted like terminal panels; peerTabsRemoved and tabLabel handle the new type; tabs are not persisted across launches today, so no stored format changes" — frontend types, nothing persisted
- [Tab] Q: "Does the Wails page need its own CSP or WebKit preference?" → Selected: "No: R13 decoded under default Wails v2.10.2 WebKit settings and the Wails page has no CSP (research ARCHITECTURE section 5); add a preference only if the real-app check shows RTCPeerConnection missing" — build options
- [Control] Q: "How are menu key equivalents switched off in control mode?" → Selected: "Plan-phase research picks the mechanism, candidates in order: (1) swap to a control-mode application menu without accelerators via runtime.MenuSetApplicationMenu and MenuUpdateApplicationMenu; (2) an ObjC NSEvent local monitor that hands Cmd keyDowns to the web view before menu dispatch in control mode; the frontend calls a SetControlMode(bool) binding, and a Go-side guard restores the normal menu on blur, tab close and app deactivation even if the frontend misses it" — local GUI code behind one binding
- [Control] Q: "How do forwarded keys reach the remote?" → Selected: "The web view's keydown/keyup (meta flag set) go through Phase 20's key message unchanged; control mode only removes the local interception" — no new protocol
- [iPhone] Q: "How does trackpad mode talk to the host?" → Selected: "The client keeps a virtual pointer position, integrates finger deltas with light acceleration and sends the absolute positions Phase 20's pointer message takes; the cursor drawn into frames confirms it" — client-side math, no protocol change
- [iPhone] Q: "How is phone mode detected?" → Selected: "lib/phoneQuery.ts PHONE_QUERY (coarse pointer, no hover), never the user agent (Phase 14 rule)" — reuse
- [iPhone] Q: "How do touch handlers avoid page zoom and scroll fights?" → Selected: "Non-passive touch listeners on the video container that preventDefault multi-touch, plus a gesturestart guard (the lib/pinchZoom.ts and touchScrollHandler.ts pattern); zoom is a CSS transform on the video element, not page zoom" — local handler code
- [Testing] Q: "How are DVWR-01/03/04 proven?" → Selected: "Vitest for the tab, row, control-mode state and touch math; Go tests for the proxy (Origin injection, long-lived ctx, no token in the URL), the meta header and the peer cap route (same-user only, 404 when off, a desktop token refused on /sessions/*); a Tart VM GUI row with the real Portty.app on portty-test-2 viewing portty-test (row, tab, Local Network prompt, Cmd+Q/W/C forwarded and given back); iPhone gestures as user-session M-NN rows; TESTING.md Sections 2, 4 and 5 updated per the standing rule" — test layout

### Claude's Discretion
User delegated: 'I don't need to answer a bunch of questions' (2026-11-25). Claude may adjust these during planning:
- The Desktop row is the first row inside each peer group, above that peer's sessions, labelled Desktop with a display glyph (heroicons ComputerDesktopIcon, already a dependency) and the peer's PeerDot colour. Reason: SC1 says "lists a peer's desktop next to its sessions", and this adds no dependency.
- The row appears only while the peer's meta carries the desktop header. It shows a muted state when the last attempt found Waiting for approval at the Mac, Display asleep or Locked. Reason: the user sees why a desktop will not open before clicking.
- There is one Desktop tab per peer (tab id desktop:<fullhost>), and clicking the row again focuses the open tab. Reason: two streams from one Mac waste the office Mac's encoder.
- The tab stays connected while it is open, also when hidden behind another tab, and closing it disconnects. Reason: tab switches stay instant. The cost, an office display kept lit while a tab is open, is the user's to end by closing the tab.
- The tab uses the browser viewer's state wording (Connecting, Reconnecting, Waiting for approval at the Mac, Display asleep, Locked) and its letterboxed, fitted, Retina-sharp video. Reason: SC1 asks for the same state and reconnect behaviour as the browser viewer.
- Control mode starts when the user clicks into the video. While on, it shows a thin accent-coloured ring around the video and a small toolbar label naming the peer and the release chord. Reason: the user must always see who owns the keyboard, in a native-looking way.
- The reserved release chord is picked during planning from chords the remote Mac rarely needs (candidate Ctrl+Option+Cmd+Return), and is shown in the label and in Help. Reason: the exact keys are a small UX choice once the decision above fixes that a release chord exists.
- When Local Network is denied, the tab shows one plain line, "Portty needs Local Network access to show this desktop", with Open System Settings and Open in Browser. Reason: it tells the user the fix without jargon.
- iPhone gestures: one-finger drag moves the pointer relatively (trackpad), tap is a left click, double-tap is a double click, long-press is a right click, tap then touch-and-drag drags, and two-finger drag scrolls. Reason: the standard trackpad vocabulary (research FEATURES Touch row) plus the gestures DVWR-03 names.
- iPhone pinch zooms the video locally from 1x to 5x around the pinch centre. While zoomed, the view pans to keep the pointer on screen, and a two-finger double-tap resets to fit. Reason: a 13-inch Mac screen is about quarter scale on a phone (research FEATURES), and 5x matches Sill.
- iPhone soft keyboard: a keyboard button in a slim bottom bar focuses a hidden input, and composed text goes as Phase 20 text messages. A desktop key bar sits above the keyboard with Esc, Tab, one-shot Ctrl, Opt and Cmd, the arrows, and Phase 20's Send-keys menu. Reason: iOS gives no reliable key codes (research FEATURES), and this reuses the Phase 14 key-bar look.
- After the phone sleeps, the viewer reconnects by itself. If iOS needs a new tap to play, it shows the same Tap to start overlay as the first load. Reason: one familiar recovery screen.
- The iPhone layout is full-bleed video with safe-area insets and the Phase 14 visualViewport keyboard inset, with no sidebar. Reason: the phone has no room for chrome, and the patterns exist.

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- internal/daemon/remote_ws_proxy.go: handleRemoteSessionWS (Origin injection byte-exact, long-lived request ctx, upstream ping, copyWS), the twin for the desktop signalling proxy; relay_remote_files.go registers loopback relay routes.
- internal/daemon/remote_caps.go: RemoteCapStore, in memory only, with a source-grep no-disk test; peer_connect.go: matchPeer, handlePeerConnect, SetPeerSeamsForTest.
- internal/webserver/whois.go handlePeerCap (JSON content type, 8 KiB body cap, no-store, returns the existing token); server.go setupRoutes: GET /api/sessions/meta behind requireSameUser (line 565), POST /api/peer/sessions/{id}/cap (line 570).
- internal/tailnet/sessions.go: doFetchSessionsMeta, PeerSessionMetaGroup, MetaStatus (MetaForbidden hides other-account peers), FetchPeerCap.
- app.go bindings GetRemoteSessionsWithMeta and ConnectPeerSession (the pattern for ConnectPeerDesktop); main.go appMenu with Quit Cmd+Q, Close Tab Cmd+W, menu.EditMenu for Cmd+C/V/X/Z, Settings Cmd+comma, Hide Cmd+H, Toggle Sidebar Ctrl+Cmd+S.
- frontend: Sidebar.tsx (peer groups, PeerDot, "Nothing shared"), lib/sidebarPeers.ts (peerView, peerColorIndex), TabBar.tsx and lib/tabs.ts (web-session peer tabs, peerTabsRemoved), lib/phoneQuery.ts (usePhone, isStandalone), components/KeyBar.tsx (one-shot Ctrl, pointerdown firing rule), lib/pinchZoom.ts, touchScrollHandler.ts, visualViewportInset.ts, lib/webMode.ts.
- Arriving upstream: DesktopView.tsx and lib/desktopClient.ts (Phase 19); the input layer, Send-keys menu, clipboard actions and display switch (Phase 20).
- Reference only, never imported: spike/desktop/wailsshell (Wails v2.10.2 shell with a Go-side signalling proxy in the AssetServer handler) and spike/desktop/viewer.html (explicit play() in ontrack, Tap to play button).

### Established Patterns
- The web view reaches only 127.0.0.1; anything on a peer goes through a daemon loopback proxy that holds the cap server-side and injects Origin.
- Peer endpoints are requireSameUser twins (sessions meta, peer cap, port grant); refusals carry the X-Portty marker.
- Seams are package vars with SetXForTest; GUI click checks run in the Tart VMs, not on the host.
- Phone mode is a media query, never the user agent; touch handlers are non-passive and scoped to their container.
- macOS-native look: system font, system accent, light and dark, source-list sidebar, SF Symbols-style glyphs, heroicons already bundled.

### Integration Points
- Share server (office side): the X-Portty-Desktop header in handleSessionsMeta and the new POST /api/peer/desktop/cap route.
- Daemon (home side): POST /peers/desktop on the daemon socket, the desktop cap store, and GET /api/relay/remote-desktop/{host}/ws on the relay loopback mux.
- GUI: ConnectPeerDesktop and SetControlMode bindings in app.go; the control-mode menu switch in main.go (or an ObjC event monitor); NSLocalNetworkUsageDescription in build/darwin/Info.plist (only NSAllowsLocalNetworking for ATS there today).
- Frontend: the 'desktop' tab type, a Desktop row in Sidebar.tsx, the control-mode wrapper and iPhone touch layer around DesktopView.
- TESTING.md: suite manifest, traceability rows for DVWR-01/03/04, and M-NN rows for the iPhone and Local Network checks.

</code_context>

<specifics>
## Specific Ideas

Settled by prior decisions (not re-asked):
- The iPhone viewer is the web viewer in Safari or from the Home Screen, not a native iOS app (REQUIREMENTS DVWR-03; PROJECT Active "iPhone share view"; ROADMAP Phase 21 goal "same viewer and protocol as the browser link").
- The iPhone starts video only from a user tap (REQUIREMENTS DVWR-03 as updated 2026-11-25; 17-SPIKE-MATRIX R14, R14-safari, R14-homescreen); Phase 19 already has the tap-to-start fallback (19-CONTEXT [Viewer]).
- WebRTC works inside the Wails WKWebView, so DVWR-01 ships as an in-app tab and the browser-link fallback is not taken (17-SPIKE-MATRIX R13, R13-lite and R13-full GO: H.264 42e01f decoded, data-channel RTT 59 ms). The real app still has to be proven here, because R13 did not reproduce the bundle ID app.portty.Portty, the React frontend, the window options or the real signalling (17-CONTEXT Portty-app check substitution).
- The iPhone and browser use the always-available link `https://<mac>.<tailnet>.ts.net:7444/desktop/?cap=<token>` with the /desktop/ manifest scope, and Home Screen links survive restarts and upgrades (19-CONTEXT Always-available link and token; DSHR-02).
- One viewer for every surface: DesktopView.tsx plus lib/desktopClient.ts, reused in the Wails tab (19-CONTEXT [Viewer]; research ARCHITECTURE section 7).
- Reconnect is capped backoff of about 1 s to 10 s after ICE failure, socket close, page wake or a server restart, showing Reconnecting (19-CONTEXT [Reconnect]; DESK-03).
- Signalling is non-trickle JSON over WSS at /desktop/ws with a version field, and media goes UDP straight to the host over the tailnet with empty iceServers (19-CONTEXT [Signalling], [WebRTC]; DESK-02).
- The web view origin is wails:// and the peer's requireAllowedOrigin demands Origin equal to BaseURL, so Wails signalling goes through the local daemon as terminal peer tabs do (research ARCHITECTURE section 3; remote_ws_proxy.go header).
- Peer caps held by the home daemon stay in memory and never reach disk (remote_caps.go invariant and its test).
- Only same-account peers appear in the sidebar, because /api/sessions/meta is behind requireSameUser and MetaForbidden hides the rest; other tailnet people reach a desktop only through Phase 22 links (PROJECT Key Decisions 2026-11-24; DSHR-01).
- Cmd+Tab and Cmd+Space belong to the OS and go through Phase 20's Send-keys menu, so there is no event tap and no new Input Monitoring or Accessibility grant on the viewer Mac (DCTL-03; research FEATURES; research PITFALLS keyboard capture).
- Cmd+C in control mode copies on the remote Mac only. Text comes back to the viewer only through Phase 20's explicit Copy action, and concealed items never sync (DVWR-04; DCTL-06).
- macOS-native look (memory portty-macos-native-look; PROJECT Key Decision 2026-11-15) and Apple Silicon with minimum macOS 14 (PROJECT Key Decision 2026-11-24).

Upstream dependencies on Phase 20 (Phase 20 owns the input protocol; Phase 21 only consumes it):
- An absolute pointer message with button down/up and click count; a wheel message for two-finger scroll; key down/up with physical code and modifiers including Meta; a composed-text message for the iOS soft keyboard; and release-all (or a guaranteed release on data-channel close).
- The Send-keys menu as a component the Desktop tab toolbar and the iPhone key bar can place.
- The explicit Copy and Paste actions and the paste message (text only, concealed excluded). The Cmd+V decision follows Phase 20's ruling on whether a browser Cmd+V is an explicit paste.
- The display and window switch control inside DesktopView, which the tab and iPhone inherit.
- The control/permission state for the own-account viewer, so control mode is offered only when the viewer has control.
- If Phase 20 sends relative pointer motion instead of absolute positions, the iPhone trackpad math moves host-side. That is a planning note, not a reopened decision.

Other specifics:
- R13 measured ICE connect at about 430 ms and first frame at 0.77 to 0.88 s in the web view. R14 measured 13 to 24 fps on the iPhone with first frame 1.2 to 2.2 s after the tap.
- A connected viewer holds the office Mac's display-awake and system-sleep assertions (DPRM-04), so an open Desktop tab keeps the office display lit.
- The office Mac is unattended and nobody can get in to fix it, so a Desktop tab must never leave a key held there: release-all on every control-mode exit and on disconnect.
- Research rules carry over: subagents never contact the host's tailscaled or 100.x addresses and never touch ~/Downloads or /Applications on the host; GUI checks run in the Tart VMs, and iPhone checks are user sessions.

</specifics>

<deferred>
## Deferred Ideas

- Direct-touch mode on iPhone (REQUIREMENTS Future; trackpad first).
- Three-finger iPhone gestures for Mission Control and Spaces (research FEATURES, deferred if time is short).
- The audio unmute control on iPhone and in the tab (DCTL-07, Phase 23).
- Desktop rows for other tailnet people's shares, admit and guest view-only (Phase 22).
- Pausing or disconnecting a hidden Desktop tab after an idle time so the office display can sleep; revisit after the Phase 23 soak.
- The Keyboard Lock API for Chrome fullscreen viewers (browser viewer, not this phase's surfaces).

</deferred>
