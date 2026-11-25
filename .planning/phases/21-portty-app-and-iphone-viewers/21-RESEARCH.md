# Phase 21: Portty App and iPhone Viewers - Research

**Researched:** 2026-11-25
**Domain:** Wails v2 (WKWebView) Desktop tab over a loopback signalling proxy, macOS key-equivalent routing, iOS Safari / Home Screen touch viewer
**Confidence:** MEDIUM overall. HIGH for the in-repo plumbing (read this session). MEDIUM for WKWebView key routing (WebKit source plus field reports that disagree, so a VM probe decides). LOW for iOS touch, keyboard and resume details (web reports only; real-iPhone rows decide).

Phases 19 and 20 are DESIGNED, NOT BUILT. Everything below that touches `DesktopView.tsx`, `lib/desktopClient.ts`, `lib/desktopInput.ts`, `internal/desktop`, `/desktop/ws`, the input data channel or the desktop token is researched against `19-CONTEXT.md` and `20-CONTEXT.md`, not against code. No experiments were run (reading and web research only). No file other than this one was modified, and nothing was committed.

## Findings that change CONTEXT decisions or create risk

Ranked, plain language. Items 1 to 4 change a CONTEXT decision or add a requirement to Phase 20. Items 5 to 9 are risks the plan must carry.

1. **The control-mode mechanism in CONTEXT is probably more machinery than needed, and the ordering of its candidates is wrong.** CONTEXT lists (1) swap the whole app menu with `MenuSetApplicationMenu`, then (2) an NSEvent monitor. WebKit's own code hands a key equivalent to the page first when the web view is focused, and only passes it to the menu if the page did not handle it, so a plain `preventDefault()` in the Desktop tab's keydown handler may already stop Cmd+W/Q/C from reaching the menu, with no native code and nothing that can be left switched. But two Tauri/WKWebView field reports contradict each other on who goes first, so this is MEDIUM until a VM probe in the real Portty.app settles it. Recommended order: (0) page-level `preventDefault`, then (3) blank the live menu items' key equivalents in place (small ObjC, no rebuild), and keep (1) last: Wails' rebuild leaks menu-item map entries on every call and the app already has a menu-bar glitch after Accessory-to-Regular switches. (1)/(2) stay as written only if the probe fails both (0) and (3). See Pattern 1 and Pitfall 1.
2. **macOS never delivers keyup for a normal key while Cmd is held.** In control mode Cmd+C arrives as keydown(Meta), keydown(C), then only keyup(Meta). Forwarded naively, the remote Mac is left with C "held" in the daemon's per-viewer held set, and Phase 20's heartbeat keeps it alive. Neither 20-CONTEXT nor 21-CONTEXT mentions this. The established fix (noVNC) is to send a press and an immediate release for any non-modifier key pressed with Meta down. DVWR-04's "no key left held" safety rests on it. It belongs in Phase 20's `lib/desktopInput.ts`; Phase 21 must require and test it. See Pitfall 2.
3. **The Cmd+V decision in 21-CONTEXT is already decided by Phase 20, the other way.** 21-CONTEXT says Cmd+V in control mode sends the home clipboard through the paste message, unless Phase 20 rules a browser Cmd+V is not an explicit paste. 20-CONTEXT (Claude's Discretion) rules exactly that: Cmd+C and Cmd+V typed in the viewer go to the remote Mac as keys and never sync clipboards; Paste to Mac is a toolbar action. So plan the fallback branch (plain key). The primary branch would also need new ObjC, because Wails' `ClipboardGetText` shells out to `pbpaste` and cannot skip concealed items. It is a product call the user may want to revisit; see Open Question 1.
4. **iPhone trackpad mode needs the host cursor position, and Phase 20's protocol cannot give it.** Phase 20 sends only absolute normalized pointer positions and returns no cursor position. A client-side virtual pointer therefore starts at a guess and drifts whenever the real cursor is moved by the office Mac's own mouse, another viewer, or the remote app. The next finger move makes the real cursor jump to the stale virtual position. CONTEXT notes the relative-motion case but treats it as a planning note. Because Phase 20 is not yet planned, adding an optional relative move (or a cursor-position report) there now is cheap and costly later. Recommendation in Open Question 2; client-only fallback is "first touch after connect, resume or reconnect jumps the pointer to the touch point, then moves relatively".
5. **Mount model: "mounted like terminal panels" is not how peer tabs work today, and the existing render loop would mis-render a desktop tab.** Peer (`web-session`) tabs render only while active and drop their socket on a tab switch (HARD-05). Local terminals stay mounted. The panel loop's only exclusion is `if (tab.type === 'web-session' || tab.type === 'preview') return null` (App.tsx:1409), so a new `'desktop'` type would render a `TerminalPanel` unless that line is changed. The Desktop tab needs its own always-mounted branch. Hidden by `display:none` is the risk to probe for a live `<video>` in WKWebView (decode and frame timing), so prefer a visibility-based hide and verify.
6. **A hidden Portty window keeps the office display lit.** `HideWindowOnClose: true` means closing the window leaves the app running with the Desktop tab connected, and a connected viewer holds the office Mac's display-awake assertion (DPRM-04). CONTEXT only defers pausing a hidden tab. The window-hidden case is sharper because nothing on the home Mac shows it. Decision needed (Open Question 3); recommendation is a small disconnect-after-hidden rule.
7. **Phase 19 does not yet promise the seams this phase reuses.** Phase 19 designs `DesktopView` for the browser: same-origin `/desktop/ws?cap=` with the token in the URL. The Wails tab needs an injectable transport (a URL provider with no token, a re-auth step before each redial like `beforeReconnect`, the `portty:wake` window event). `detectMode()` returns `'desktop'` (meaning Wails) for any path outside `/app/`, so a `/desktop/` browser page would be misread as the Wails shell unless Phase 19 adds a mode or its own entry. The strict share-server CSP (`default-src 'none'`, no `media-src`) has never been exercised with a `<video>` in any browser: R13 ran under the lax Wails meta CSP and R11/R12/R14 under the spike server. Record these as Phase 19 asks or as Phase 21 Wave 0 work (Open Question 4).
8. **Token lifecycle and failure mapping in the proxy.** The existing remote-session proxy collapses every dial failure into close code 1013 "remote unreachable" and discards the HTTP response (remote_ws_proxy.go:118 and :126). The desktop twin must map the peer's status to fixed reasons (cap rejected, desktop off, unreachable) so the tab can re-run `ConnectPeerDesktop` on a rejected cap (always-available turned off then on, Regenerate Signing Key, home daemon restart wipes the in-memory token). `POST /peers/desktop` must be added to the `freezeGate` exemption beside `/peers/connect` (api.go:139) or it gets a 503 during a handoff. R13 also showed the page did not recover by itself after the user clicked Allow on the Local Network prompt, so the tab must keep retrying underneath the "needs Local Network" line.
9. **iPhone specifics that will bite.** iOS Safari does not fire `contextmenu` on long press, so right-click needs a touch timer. `lib/visualViewportInset.ts` is hard-wired to `.app` and `.xterm-helper-textarea`. `KeyBar.tsx` sends terminal bytes, so only its pointerdown firing rule and CSS carry over, not the component. Keep one `<video>` element for the life of the page so iOS does not demand a second tap after sleep ([ASSUMED], device row). The Home Screen icon keeps the exact link including `?cap=` today only because the manifest has no `start_url` (Phase 14, M-145 PASS); a Phase 19 `/desktop/` manifest that adds one would break saved links.

Smaller items (Phase 20 mismatches, Local Network key wording, M-NN numbering, transparent web view) are in Pitfalls and Open Questions.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

User delegation (2026-11-25): "I don't need to answer a bunch of questions". Reversible technical choices were decided autonomously, product and UX choices are under Claude's Discretion, and the five hard-to-reverse recommendations below were accepted by the user with "Accept all" (2026-11-25).

#### Peer discovery and authorization
- The sidebar learns that a peer has an always-available desktop from a response header `X-Portty-Desktop: always` on the existing GET /api/sessions/meta. The JSON body stays a bare array, so v1.x peers still parse it, and no extra request is made per peer per poll. The header is sent only while always-available is on, Phase 22 may add a value, and readers ignore values they do not know — **Reversibility:** costly — it becomes a wire contract between peers on different versions, and every later build has to keep sending and parsing it.
- The Portty app gets the right to open a peer's always-available desktop through a new POST /api/peer/desktop/cap behind requireSameUser, the twin of handlePeerCap (whois.go). It returns the standing always-available desktop token to the peer's daemon, which keeps it in memory beside RemoteCapStore and never puts it on disk or in the web view URL. The WhoIs same-user check runs on this endpoint and again on /desktop/ws. Turning always-available off or Regenerate Signing Key kills the token. This matches how terminal peer tabs work today — **Reversibility:** costly — a cross-version peer API route that every later build serves, and it fixes where the standing desktop token may travel.

#### Desktop tab in the Portty app
- The macOS Local Network prompt that WebRTC in the Portty web view triggers on the viewer Mac (R13: ICE stays at new until Allow; the grant survives a same-identity rebuild) is accepted on the first Desktop tab open. An NSLocalNetworkUsageDescription string in Info.plist explains why. If ICE stays at new past a timeout, the tab shows a short state with Open System Settings (Privacy & Security ▸ Local Network) and Open in Browser. No other grant is needed on the viewer Mac — **Reversibility:** costly — it adds a TCC entry and a System Settings row on every viewer Mac, and a denial can only be undone by the user in System Settings.

#### Control mode (keyboard routing in the Desktop tab)
- While control mode is on, every key chord the Portty menu owns goes to the remote Mac: Cmd+Q, W, C, V, X, Z, A, T, H, M, comma, Ctrl+Cmd+S and the rest. Control mode ends when the user clicks outside the video, switches tabs, when Portty stops being the active app (Cmd+Tab stays local because the OS owns it), when the stream is not connected (Reconnecting or ended), or on one reserved release chord that is never forwarded. Every exit sends release-all so no remote key stays held (DCTL-02). There is no system-wide event tap — **Reversibility:** costly — it is a safety boundary over which local keys a remote session can swallow, and users learn the escape path.
- In control mode, a typed Cmd+V counts as the explicit paste action. The app sends the home clipboard's plain text (never an item marked concealed) through Phase 20's paste message, then the keystroke. If Phase 20 rules that a browser Cmd+V is not an explicit paste, the app follows Phase 20 and forwards a plain key — **Reversibility:** costly — it sets when clipboard data leaves the viewer Mac, which users and the DCTL-06 explicit-only rule depend on.

#### Decided autonomously
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

### Deferred Ideas (OUT OF SCOPE)
- Direct-touch mode on iPhone (REQUIREMENTS Future; trackpad first).
- Three-finger iPhone gestures for Mission Control and Spaces (research FEATURES, deferred if time is short).
- The audio unmute control on iPhone and in the tab (DCTL-07, Phase 23).
- Desktop rows for other tailnet people's shares, admit and guest view-only (Phase 22).
- Pausing or disconnecting a hidden Desktop tab after an idle time so the office display can sleep; revisit after the Phase 23 soak.
- The Keyboard Lock API for Chrome fullscreen viewers (browser viewer, not this phase's surfaces).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DVWR-01 | Portty app on another Mac opens a peer's desktop in a Desktop tab from the sidebar | Discovery header and cap route (Pattern 4, 5); loopback proxy twin (Pattern 3); tab and mount model (Pattern 6); Wails WebRTC proven by R13, GO; Local Network prompt behaviour (Pitfall 7); injectable transport gap in Phase 19 (Open Question 4) |
| DVWR-03 | iPhone (Safari or Home Screen) viewer has trackpad mode, pinch-zoom, long-press right-click, two-finger scroll and soft keyboard; video starts from a tap | R14 GO (tap needed); gesture recognizer and coordinate mapping (Pattern 7); resume (Pattern 8); iOS pitfalls 8 to 11; cursor-sync gap in Phase 20 (Open Question 2); Home Screen link behaviour (M-145, Pitfall 12) |
| DVWR-04 | In control mode in the Portty app, Cmd+W, Cmd+Q and Cmd+C go to the remote Mac instead of Portty | Key routing in WKWebView (Pattern 1, Pitfall 1); Cmd-held keyup swallowing (Pattern 2, Pitfall 2); release path and release chord (Pattern 1); Cmd+V follows Phase 20 (Open Question 1) |
</phase_requirements>

## Summary

The phase is mostly plumbing that has direct twins in the code today: a sidebar group row (Sidebar.tsx), a cap-holding peer route (`handlePeerCap` / `POST /peers/connect`), and a loopback WebSocket proxy (`handleRemoteSessionWS`). Each twin is understood and has an established test shape. The new work is (a) a header on `/api/sessions/meta`, (b) a cap route and an in-memory cap store keyed by host, (c) a proxy that carries signalling only, (d) a `desktop` tab type with its own always-mounted branch, (e) control-mode key routing, and (f) an iPhone touch layer around `DesktopView`.

The two real unknowns are both platform behaviour, not design. First, which side of a WKWebView wins a Cmd chord (page or menu): WebKit's source says the page, two field reports say opposite things, so Wave 0 must probe it in the real Portty.app in the Tart VMs before any native code is written. Second, how iOS Safari and the Home Screen app behave for long-press, the hidden-input soft keyboard and resume after sleep. These rest on web reports and the existing Phase 14 evidence, so every iPhone behaviour needs a user-session row.

Phase 21 sits on top of unbuilt Phases 19 and 20, and there are gaps between what CONTEXT says Phase 21 consumes and what the Phase 19/20 designs promise (see Findings 4, 7, and the Phase 20 mismatch list under Pitfall 13). None block planning, but the plan should schedule a Wave 0 that either adds the seams to the Phase 19/20 plans or builds them here.

**Primary recommendation:** Wave 0 = a VM probe of Cmd-chord routing in the real app plus a transport seam in `desktopClient`; then build the discovery, cap, proxy and tab plumbing as twins of the existing code, implement control mode as page-level key capture (native fallback only if the probe demands it), and write the iPhone gesture layer as a pure, unit-tested state machine.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Desktop-available discovery | Office share server (`handleSessionsMeta`) | Home GUI process (`tailnet.FetchAllPeerSessionsMeta`, called from `App.GetRemoteSessionsWithMeta`) | The peer states it in a header; the GUI process already polls every peer, so no new request |
| Authorization for the standing desktop token | Office share server (`requireSameUser`, new cap route) | Home daemon (in-memory cap store, `POST /peers/desktop`) | Same pattern as terminal peer tabs; the token never reaches the web view |
| Signalling reach (wails:// to peer) | Home daemon relay loopback mux (new proxy route) | Office share server (`/desktop/ws`, Phase 19) | The web view reaches only 127.0.0.1; the daemon holds the cap and injects Origin |
| Media (video, data channel) | Web view to office host, UDP over the tailnet (Phase 19 host) | OS Tailscale interface | R13: UDP goes web view to peer directly; the proxy never carries media |
| Desktop tab state and rendering | Frontend (`DesktopView` reused from Phase 19, plus a tab wrapper) | App.tsx tab list | One viewer for every surface |
| Control-mode key capture | Frontend (keydown handler, Phase 20 `desktopInput`) | Wails GUI native layer only if the VM probe forces it | WebKit routes a focused key equivalent to the page first (MEDIUM) |
| Local Network permission | macOS TCC on the viewer Mac (prompt owned by Portty) | Info.plist usage string | Triggered by the web view's ICE, not by Go code |
| iPhone touch, zoom, keyboard | Browser (`DesktopView` touch layer, mobile Safari / Home Screen) | Share server serves the page (`/desktop/`, Phase 19) | No native iOS app; same viewer and protocol as the browser link |
| Resume after phone sleep | Browser (reconnect on `pageshow`/`visibilitychange`, liveness probe) | Share server (accepts a fresh `/desktop/ws`) | iOS drops sockets on lock and reports dead ones as open |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Wails | v2.10.2 | GUI shell, menu, bindings | In `go.mod` `github.com/wailsapp/wails/v2 v2.10.2` [VERIFIED: go.mod:16]. The Wails WebRTC path is proven by R13 on this exact version |
| coder/websocket | v1.8.15 | Loopback proxy WebSocket (both legs) | Already used by `remote_ws_proxy.go` [VERIFIED: go.mod:14] |
| React | ^19.2.4 | Tab, row, DesktopView, touch layer | Existing [VERIFIED: frontend/package.json] |
| @heroicons/react | ^2.2.0 | `ComputerDesktopIcon` already imported by Sidebar.tsx | No new dependency [VERIFIED: Sidebar.tsx:2] |
| vitest | ^4.1.0 (jsdom ^29) | Unit tests for tab, row, control mode, touch math | Existing runner; touch tests use mock containers and plain event objects because jsdom's TouchEvent is incomplete [VERIFIED: pinchZoom.test.ts header comment] |
| Playwright (WebKit, `iphone` project) | ^1.59.1 | iPhone 15 emulation for layout checks only | Existing `iphone` project matches `mobile-.*\.spec\.ts` [VERIFIED: playwright.config.ts:49-52] |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| pion/webrtc v4 | v4.2.29 (Phase 19, host side only) | Office-side WebRTC | Not touched in Phase 21; the viewer uses the browser's own `RTCPeerConnection` |
| cgo ObjC (`.m` files at repo root) | n/a | Only if the Wave 0 probe forces a native key-equivalent fallback | `tray_objc_darwin.m` and `appearance_darwin.m` show the bridging style |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Page-level `preventDefault` for control mode | `MenuSetApplicationMenu` swap (CONTEXT candidate 1) | Rebuilds the NSMenu each toggle, leaks Wails' menu-item maps (no deletes), and a stuck swap leaves the user unable to Cmd+Q |
| Page-level `preventDefault` | NSEvent local monitor (CONTEXT candidate 2) | More native code and the same stuck-state risk, with no gain if (0) works |
| Blank key equivalents in place | Disable menu items | Field reports disagree on whether a disabled item still swallows its key; clearing `keyEquivalent` is unambiguous |
| Client-side virtual pointer (CONTEXT) | Relative pointer message in Phase 20 | Removes the drift/jump problem; needs a Phase 20 protocol addition now (Open Question 2) |

**Installation:** none. No new packages in this phase (no `npm install`, no `go get`).

**Version verification:** Wails v2.10.2 read from `go.mod` this session. Go toolchain is pinned by `build.sh` (go.mod `toolchain go1.26.11`, local `go1.27.4`): run Go tests through the project's pinned toolchain path, not bare `go test`, if a tailscale JSON decoding failure appears. [VERIFIED: go.mod:3-5 (`go 1.26.4`, `toolchain go1.26.11`); `go version` output]

## Package Legitimacy Audit

No external packages are installed in this phase, so the slopsquatting gate does not apply. Nothing was added to `package.json` or `go.mod`.

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
HOME MAC (viewer)                                              OFFICE MAC (shared, unattended)
-----------------                                              --------------------------------
Portty.app GUI process                                         Share server :7444 (Phase 19: kept up while
  Sidebar.tsx                                                    always-available is on)
   |  poll (5-30 s)  GetRemoteSessionsWithMeta  ------HTTPS---> GET /api/sessions/meta (requireSameUser)
   |   (tailnet.FetchAllPeerSessionsMeta)         <---header---   body: [] or sessions
   |   reads X-Portty-Desktop: always                             header X-Portty-Desktop: always
   v   renders "Desktop" row first in the peer group
 click Desktop row
   |
   v  binding ConnectPeerDesktop(host)
Portty daemon (unix socket, 0600)
   POST /peers/desktop  -> matchPeer -> peerClient -----HTTPS--> POST /api/peer/desktop/cap
        stores token in DesktopCapStore (memory only)  <-token--  requireSameUser (WhoIs) -> standing token
   |
   v  tab opens: Tab{type:'desktop', id:'desktop:<fullhost>', peerHost}
Wails web view (origin wails://wails)
   DesktopView (Phase 19 viewer, injected transport)
    |  WS ws://127.0.0.1:<relayPort>/api/relay/remote-desktop/<host>/ws   (no token in URL)
    v
 Daemon relay loopback mux --- wss://peer:7444/desktop/ws?cap=<held> ---> /desktop/ws
   Origin allowlist (LoopbackOriginPatterns)   Origin = BaseURL injected   requireAllowedOrigin
   long-lived ctx, upstream ping                                            -> requireSameUser -> desktop token
    ^   signalling only (offer/answer, state, surface, clipboard)
    |
 web view RTCPeerConnection  ====== UDP media + input data channel ======>  host ICE-lite on tailnet IP
   (macOS Local Network prompt on first ICE; R13: ICE stays "new" until Allow)

iPHONE (Safari or Home Screen)
   https://<mac>.<tailnet>.ts.net:7444/desktop/?cap=<token>  (Phase 19 link, saved on Home Screen)
   DesktopView + touch layer (trackpad, pinch, long-press, 2-finger scroll, hidden input)
   tap to start video -> same /desktop/ws and data channel as the browser; no proxy
```

### Recommended Project Structure
```
internal/
  tailnet/sessions.go            # + PeerMeta result (sessions + Desktop bool); old funcs stay as wrappers
  webserver/server.go            # + X-Portty-Desktop header in handleSessionsMeta; + POST /api/peer/desktop/cap
  webserver/whois.go             # + handlePeerDesktopCap (twin of handlePeerCap) beside SetShareTokenProvider-style setter
  daemon/desktop_caps.go         # DesktopCapStore, memory only (sibling of remote_caps.go) + no-disk grep test
  daemon/peer_desktop.go         # POST /peers/desktop (twin of handlePeerConnect)
  daemon/remote_desktop_proxy.go # GET /api/relay/remote-desktop/{host}/ws (twin of remote_ws_proxy.go)
  daemon/relay_remote_files.go   # + one mux.HandleFunc line
  daemon/api.go                  # + route; + freezeGate exemption for /peers/desktop
app.go                           # + ConnectPeerDesktop, (+ SetControlMode only if the probe forces it)
build/darwin/Info.plist          # + NSLocalNetworkUsageDescription (also Info.dev.plist)
frontend/src/
  components/Sidebar.tsx         # Desktop row; "Nothing shared" must not show beside a Desktop row
  components/TabBar.tsx          # Tab.type gains 'desktop'
  lib/tabs.ts, sidebarPeers.ts   # tabLabel, peerTabsRemoved, peerView carry the desktop flag
  components/DesktopTab.tsx      # control-mode wrapper, ring and label, state lines, Local Network line
  lib/controlMode.ts             # control-mode state machine (pure)
  lib/touchTrackpad.ts           # gesture recognizer (pure, injectable clock)
  lib/desktopCoords.ts           # touch point -> normalized surface point under zoom, pan, letterbox (pure)
  components/DesktopKeyBar.tsx   # iPhone key bar (sends Phase 20 key messages; not KeyBar's bytes)
```

### Pattern 1: Control mode as page-level key capture (probe first)
**What:** While control mode is on, the Desktop tab's keydown/keyup handler always calls `preventDefault()` and forwards the key, including Meta chords. `preventDefault` is the whole local-interception removal. No menu is touched. Leaving control mode (click outside, blur, tab switch, release chord, stream not connected) removes the handler and sends release-all.
**When to use:** First, behind a Wave 0 VM probe. WebKit's `WebViewImpl::performKeyEquivalent` forwards the event to the page when the web view is the first responder and returns YES; `doneWithKeyEvent` only re-sends the event to AppKit (and so to the menu) when the page did not handle it. [CITED: github.com/WebKit/WebKit Source/WebKit/UIProcess/mac/WebViewImpl.mm, `performKeyEquivalent` and `doneWithKeyEvent`, read this session] Apple's archived guide gives the same order: view hierarchy, key window, then the menu bar. [CITED: developer.apple.com/library/archive/documentation/Cocoa/Conceptual/EventOverview/HandlingKeyEvents/HandlingKeyEvents.html] A Tauri/WKWebView app reports "WKWebView hands a key equivalent to the page first and passes it to the menu only if the page leaves it unhandled" and a `preventDefault` there stopped both. [CITED: github.com/superbiche/marque/pull/47] Another Tauri app states the opposite ("macOS resolves a menu key equivalent before the webview receives any key event") without a measurement. [CITED: github.com/block/buzz/pull/4875] Treat as MEDIUM: decide by probe.
**Wave 0 probe (VM only, real Portty.app on portty-test-2, Developer ID signed):** a throwaway keydown logger in the Desktop tab that records every chord and calls `preventDefault`. Chords: Cmd+Q, W, C, V, X, Z, A, T, H, M, comma, Ctrl+Cmd+S, Ctrl+Cmd+F, Cmd+Option+H, plain Backspace (the Wails Edit menu has a bare-Backspace Delete item). For each: did the page see keydown, and did the menu action also fire (tab closed, window hid, app quit prompt)? Record in the matrix whether any chord still reaches the menu.
**Fallback if the probe shows leaks (3):** one ObjC function that walks `NSApp.mainMenu` and sets each item's `keyEquivalent` to the empty string, remembering `(item, key, modifierMask)`, and a restore that puts them back. Do not disable items (field reports disagree on whether a disabled item swallows its key). Pair it with the Go-side guard CONTEXT already specifies (restore on window resign key, app deactivate, tab close). This is far smaller than rebuilding the menu.
**Why not rebuild the menu (candidate 1):** `UpdateApplicationMenu` builds a new NSMenu and calls `[app setMainMenu:ctx.applicationMenu]` (Application.m:326-330), and every rebuild adds entries to the package-level `idToMenuItem` and `menuItemToID` maps, which have no `delete` (menuitem.go:23-26 and 44-46, no `delete(` in menuitem.go or menu.go). The app also already works around a menu bar that goes unresponsive after Accessory-to-Regular switches (tray_objc_darwin.m:226-241). A failure there leaves the user without Cmd+Q, whereas a page-level handler fails open (page dies, menu works).
**Example:**
```typescript
// Pattern, not final code. Message shape and sendKey() belong to Phase 20's lib/desktopInput.ts.
// Release chord is the CONTEXT candidate; it is never forwarded.
const isReleaseChord = (e: KeyboardEvent) =>
  e.ctrlKey && e.altKey && e.metaKey && e.code === 'Enter'

function onKeyDown(e: KeyboardEvent) {
  if (!controlMode) return
  if (isReleaseChord(e)) { e.preventDefault(); exitControlMode('chord'); return }
  e.preventDefault()               // stops the menu (probe-verified) and any browser default
  e.stopPropagation()
  forwardKeyDown(e)                // Phase 20 key message, Meta flag set
}
```

### Pattern 2: Cmd-held keys are taps
**What:** macOS does not deliver keyup for a non-modifier key pressed while Cmd is held; browsers and WKWebView inherit this. For any keydown with `metaKey` set whose `code` is not `MetaLeft`/`MetaRight`, send keydown immediately followed by keyup and do not add it to the viewer's held set. On Meta keyup, release any non-modifier key the viewer still believes held. Release-all on every control-mode exit stays as the backstop.
**When to use:** every Meta chord in control mode (Cmd+C, Cmd+W, Cmd+Q) and in the iPhone key bar's one-shot Cmd. Also covers key repeat of Cmd+key.
**Source:** noVNC `core/input/keyboard.js` `_handleKeyDown`: condition `(browser.isMac() || browser.isIOS()) && (e.metaKey && code !== 'MetaLeft' && code !== 'MetaRight')`, sends press then release, `stopEvent(e)`. [CITED: github.com/novnc/noVNC core/input/keyboard.js] The platform behaviour is documented in Firefox bug 1299553 and WebKit bug 55291 (comment notes it affects noVNC; Chromium and Firefox too). [CITED: bugzilla.mozilla.org/show_bug.cgi?id=1299553; bugs.webkit.org/show_bug.cgi?id=55291] Whether WKWebView in a Wails app shows the same is part of the Wave 0 probe (log keyup after a Cmd+C hold).

### Pattern 3: Desktop signalling proxy as a twin of `handleRemoteSessionWS`
**What:** `GET /api/relay/remote-desktop/{host}/ws` on the relay loopback mux. Reuse as written: `websocket.Accept` with `relay.LoopbackOriginPatterns(r.Host)`, the cap lookup before the upgrade, the 10 s dial context under the long-lived request context, Origin injected as `strings.TrimRight(baseURL, "/")`, `a.peerClient(u.Hostname(), ip)`, `copyWS` both ways, `relay.Heartbeat` upstream, and the redaction of the cap from every error and log line. [VERIFIED: remote_ws_proxy.go:60-221, read this session]
**Differences from the terminal proxy (these are the plan's new work):**
- Path is `/desktop/ws`; the query carries only `cap` (no `resume`).
- Keep the `Dial` response: the existing code discards it (`upstream, _, derr := websocket.Dial(...)` at remote_ws_proxy.go:118) and closes with `websocket.StatusTryAgainLater, "remote unreachable"` for every failure (remote_ws_proxy.go:126). The desktop twin should map the peer's HTTP status to fixed, token-free close reasons the tab understands: cap rejected (401/403 with the marker), desktop off, unreachable. The tab re-runs `ConnectPeerDesktop` on "cap rejected" and keeps its backoff otherwise.
- The cap store lookup is by peer host, not session id; a miss returns the same JSON 404 contract so the frontend treats it as "no cap held, call ConnectPeerDesktop".
- Both legs keep coder/websocket's 32768-byte default read limit unless Phase 19/20 messages exceed it (20-CONTEXT chunks clipboard to 32 KiB, and the terminal proxy raises the upstream limit to `relay.ViewerReadLimit`). Check Phase 19's offer/answer size; R13 offers were 4946 to 5208 bytes. [VERIFIED: r13-shell.txt]
**Route registration:** one line in `wrapRelayWithRemoteFiles` beside `mux.HandleFunc("GET /api/relay/remote/{sessionID}/ws", a.handleRemoteSessionWS)` [VERIFIED: relay_remote_files.go:29].

### Pattern 4: Cap route and cap store
**What:** `POST /api/peer/desktop/cap` registered next to `POST /api/peer/sessions/{id}/cap` with `ws.requireSameUser(...)` [VERIFIED: server.go:570 area, registered as `mux.HandleFunc("POST /api/peer/sessions/{id}/cap", ws.requireSameUser(ws.handlePeerCap))`]. Copy `handlePeerCap` exactly: JSON content type required (preflight blocks cross-site reads), `http.MaxBytesReader(w, r.Body, 8<<10)`, `Cache-Control: no-store`, 404 when the provider says always-available is off (`ws.shareToken == nil` / provider returns false). The provider is a setter like `SetShareTokenProvider` set once before Start. [VERIFIED: whois.go:46-48 and 172-190]
**Store:** `DesktopCapStore` is a sibling of `RemoteCapStore` keyed by peer host: in memory, `https://` baseURL validated, no disk, with its own copy of the no-disk source-grep test. [VERIFIED: remote_caps.go:1-106]
**Daemon route:** `POST /peers/desktop` is the twin of `handlePeerConnect`: `matchPeer(peers, host)`, build the base URL from the peer's DNS name (never a peer-supplied URL), `a.peerClient`, `FetchPeerDesktopCap`, store, return a body with no token. [VERIFIED: peer_connect.go:60-217] Add `/peers/desktop` to the `freezeGate` exemption (api.go:139 lists `r.URL.Path == "/peers/connect"`), since it only caches a cap that no handoff moves; otherwise it is 503'd during a handoff.
**Token lifecycle:** the held token dies when always-available is turned off or the signing key regenerates (Phase 19). The tab must call `ConnectPeerDesktop` before every redial (same idea as `reconnectPeerTab` for terminals, App.tsx:1026-1034), because a daemon restart also empties the in-memory store.

### Pattern 5: Discovery header with no test churn
**What:** `handleSessionsMeta` sets `X-Portty-Desktop: always` only while always-available is on, before writing the body. The body stays `json.NewEncoder(w).Encode(items)` [VERIFIED: server.go:819-820]. `doFetchSessionsMeta` reads the header from `resp` and `PeerSessionMetaGroup` gains `Desktop bool` [VERIFIED: sessions.go:38-43, 74-104].
**Avoid churn:** `doFetchSessionsMeta`, `FetchPeerSessionsMeta`, `FetchPeerSessionsMetaWithClient` and the `fetch func(ctx, Peer) ([]ShareableSessionMeta, MetaStatus)` seam of `fetchAllPeerSessionsMeta` are pinned by existing tests (`tailnet_test.go` calls `FetchPeerSessionsMetaWithClient` and the seam). Add a struct-returning variant (sessions, status, desktop) and keep the old signatures as thin wrappers.
**Frontend:** `RemotePeerSessions` (remoteSession.ts) and the Wails models gain `desktop?: boolean`; `peerView` passes it through.
**Sidebar edge:** with a Desktop row present, the line `PEER_NOTHING_SHARED` ("Nothing shared") must not show for a peer whose only content is the desktop. [VERIFIED: Sidebar.tsx, `g.reachable && g.peer.sessions.length === 0`]

### Pattern 6: Desktop tab type and mount model
**What:** `Tab.type` gains `'desktop'` (currently `'terminal' | 'web-session' | 'preview'`, TabBar.tsx:23). Tab id `desktop:<fullhost>`, `sessionId` the same string (the panel loop keys by `tab.sessionId`, App.tsx:1413, so it must be unique). The App render has three paths today: the single active `__websession__` slot (App.tsx:1301), the always-mounted terminal loop that skips `web-session` and `preview` (App.tsx:1409), and the preview panel. A desktop tab needs a fourth branch that keeps `DesktopView` mounted when hidden, and the terminal loop's exclusion must name `'desktop'`.
**Integration points to touch (grep this session):** `tabLabel` (lib/tabs.ts:28 returns `tab.name` for any non-terminal type, so fine), `peerTabParts` (web-session only), `peerTabsRemoved` (web-session only, so a desktop tab needs no snapshot drop), `isLocal` in TabBar (non-local gives only Close), TabBar line 345 (`activeTab?.type === 'web-session'` shows peer-specific chrome), Sidebar `activeLocalId` and `activePeer` (add a desktop-active check so the Desktop row shows as current), `removeTabState` (closing must unmount `DesktopView`, which disconnects and releases keys).
**Hidden video:** keep a hidden Desktop tab connected, but prefer a hide that keeps the element laid out (for example zero opacity, `pointer-events:none`, absolutely positioned) over `display:none`, and verify decode and `requestVideoFrameCallback` continue. [ASSUMED] that `display:none` can stall WebKit video; the VM row checks the frame counter after a tab round trip.
**Reconnecting indicator:** `reconnectingHost` is a single string state set by `onReconnectingChange` (App.tsx:127, 1364). A Desktop tab and a terminal peer tab on the same host would clear each other. Either skip the amber dot for desktop tabs or make it a count.

### Pattern 7: iPhone gesture recognizer as a pure state machine
**What:** `lib/touchTrackpad.ts` takes touch events and an injectable clock and emits intents: `move(dx,dy)`, `tap`, `doubleTap`, `longPress`, `dragStart/dragMove/dragEnd`, `scroll(dx,dy)`, `pinch(scale, cx, cy)`, `resetZoom`. Tests drive it with plain event objects on a mock container, exactly like `pinchZoom.test.ts` (jsdom's TouchEvent is incomplete; handlers captured off a mock container). [VERIFIED: pinchZoom.test.ts:1-30]
**Long press:** timer-based (about 500 ms with a small movement threshold). iOS Safari generally does not dispatch `contextmenu` on long press and `-webkit-touch-callout: none` has been unreliable, so do not rely on the event. [CITED: bugs.webkit.org/show_bug.cgi?id=213953; developer.apple.com/forums/thread/808606] `preventDefault()` on `touchstart` (non-passive) is the usual way to suppress the callout and selection for that touch, but no source found confirms it for the current iOS: [ASSUMED], device row.
**Coordinate mapping:** `lib/desktopCoords.ts` maps a client point to normalized (0..1) surface coordinates given the video's letterboxed content rect, the CSS zoom and pan. Pointer messages carry normalized x, y and the surface epoch (Phase 20). Unit-test the mapping at 1x, at 5x with pan, and with letterboxing in both orientations.
**Pointer:** keep the virtual pointer clamped to 0..1. See Finding 4 for the drift problem and the first-touch fallback.
**Zoom:** CSS transform on the video element (CONTEXT), `touch-action: none` on the container, and a `gesturestart` `preventDefault` as in `pinchZoom.ts` [VERIFIED: pinchZoom.ts header]. Note `attachPinchZoom` maps distance to a whole-pixel font size; only the structure (non-passive listeners, `d0`, end timer) carries over, not the module.

### Pattern 8: Resume after phone sleep
**What:** reuse the wake triggers `relayClient.ts` already proved on a real iPhone: `pageshow`, `visibilitychange`, `online`, a watchdog tick-gap, and (in the Wails tab) the window event `portty:wake` that App.tsx re-dispatches from Go's `system:wake` [VERIFIED: relayClient.ts:309-312, 498-520; App.tsx:491-493]. Phase 19 lists `visibilitychange`/`pageshow` already; add `portty:wake` for the Wails tab (viewer Mac sleep otherwise leaves ICE to time out). Reports of iOS Safari killing WebSockets on lock, dead sockets still reporting OPEN, and `onclose` never firing after long inactivity mean a readyState check is not enough: on becoming visible, run a liveness probe (a ping on the signalling socket or data channel with a short deadline, plus inbound-rtp bytes advancing) and redial if it fails. [CITED: github.com/3ll3d00d/ezbeq/pull/169; github.com/enisdenjo/graphql-ws/issues/289; developer.apple.com/forums/thread/127900] `RECONNECT` in relayClient (1 s to 30 s, `connectMs` 15 s, "stuck" after two connect timeouts) is the model; Phase 19's desktop cap is about 1 s to 10 s.
**One `<video>` element for the life of the page:** after the first tap-started `play()`, assigning a new `srcObject` to the same element and calling `play()` should not need another tap. [ASSUMED] (WebKit tracks the user-gesture restriction per media element); if iOS still pauses it, show the existing Tap to start overlay (CONTEXT already specifies that fallback). Real-iPhone row decides.
**Wake Lock:** `navigator.wakeLock` works in Safari tabs from iOS 16.4 and in Home Screen apps from iOS 18.4. It is not needed for SC3 (resume after sleep) and is left out under the radical-simplicity rule. [CITED: caniuse.com/wake-lock; webkit.org/b/254545 (as summarised by search)]

### Pattern 9: iPhone soft keyboard
**What:** a visually-hidden but real `<input>`/`<textarea>` (not `display:none`), font-size 16 px or more, with `autocorrect="off"`, `autocapitalize="off"`, `spellcheck="false"` (xterm sets exactly these on its helper textarea [VERIFIED: node_modules/@xterm/xterm/lib/xterm.js], and Phase 14's M-146 passed plain ASCII typing with them [VERIFIED: TESTING.md M-146]). Focus it synchronously inside the keyboard button's click handler (iOS refuses programmatic focus outside a gesture). Read text from `beforeinput` and `input`, send each commit as a Phase 20 `text` message (at most 1 KiB each). Backspace on an empty field produces no event on some iOS versions, so keep a one-character sentinel in the field and send a Backspace `key` message when it is deleted; web reports say `beforeinput` with `deleteContentBackward` fires on recent iOS but is not guaranteed on every version, so listen to both events. [CITED: phabricator.wikimedia.org/T69262; bugs.webkit.org/show_bug.cgi?id=268988] Enter and arrows come as `keydown` or from the key bar. Predictive text cannot be turned off for web inputs, so a replaced word arrives as new text and cannot be un-typed remotely: accept and document, as the terminal does.
**Layout:** `useVisualViewportInset(enabled)` selects `.app` and `.xterm-helper-textarea` literally (visualViewportInset.ts: `document.querySelector<HTMLElement>('.app')`, `matches('.xterm-helper-textarea')`). Parameterize the root selector and the focused-field selector instead of copying the hook. Its own header says the iOS 26 standalone behaviour is `[ASSUMED]` until a device shows it. [VERIFIED: visualViewportInset.ts:1-12]

### Anti-Patterns to Avoid
- **Rebuilding the Wails menu to toggle control mode** (leak, menu-bar fragility, fails closed to "cannot quit"): probe page-level capture first.
- **Putting the desktop token in the web view URL, a binding result or a log line.** Terminal peer tabs do carry the session cap in the tab (`capToken` on `Tab`); the desktop decision deliberately does not. Keep it out of `ConnectPeerDesktop`'s return value.
- **Reusing `KeyBar.tsx`.** It sends terminal byte sequences (`send(seq: string)`, `ctrlMap`). The desktop bar sends Phase 20 `key` messages; share the pointerdown firing rule and the CSS only.
- **Recreating the `<video>` element on reconnect** (may cost a second tap on iPhone).
- **Detecting the phone by user agent.** iOS 27 Safari reports a frozen UA (`iPhone OS 18_7`) [VERIFIED: r14-iphone-reports.json userAgent]; use `PHONE_QUERY` [VERIFIED: phoneQuery.ts:6].
- **Calling `preventDefault` on `touchstart` of the keyboard button or toolbar.** It would block focus and click. Scope the non-passive handlers to the video container.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Loopback WebSocket reverse proxy with Origin injection, long-lived ctx, upstream heartbeat | A new proxy design | Copy `handleRemoteSessionWS` and `copyWS` | Two production bugs are already fixed in it (empty Origin 403, 10 s dial ctx killing the stream) [VERIFIED: remote_ws_proxy.go:20-27] |
| Peer matching and verified TLS to a tailnet peer | New host resolution | `matchPeer`, `a.peerClient(fqdn, ip)` | Dials the Tailscale IP from tailscaled's own list and verifies the cert against the MagicDNS name [VERIFIED: peer_client.go, peer_connect.go:80-108] |
| In-memory secrets store with no-disk test | A map in the proxy | `DesktopCapStore` modelled on `RemoteCapStore` | The grep test pins "nothing here writes a cap to disk" |
| Reconnect backoff, wake handling, liveness probe | New timers | `RECONNECT` constants shape and the `onWake`/`onSystemWake` logic in `relayClient.ts` | Proven on a real iPhone (M-148 PASS) |
| Cmd-key release workaround | Own heuristics | noVNC's press-then-release for Meta chords | Established, documented platform bug |
| Phone detection | UA sniffing | `PHONE_QUERY`, `usePhone`, `isStandalone` | Phase 14 rule |
| Key code mapping | A table in the viewer | Phase 20's closed `KeyboardEvent.code` table in the daemon | Phase 21 only forwards `code` and a modifier snapshot |
| Clipboard concealed-type filtering | `runtime.ClipboardGetText` | Nothing in this phase (plain Cmd+V) | Wails' darwin implementation runs `exec.Command("pbpaste")` and cannot see pasteboard types [VERIFIED: clipboard.go:9-15] |

**Key insight:** every network and auth piece in this phase already exists once for terminals; the risk is in the places where the desktop deliberately differs (no token in the web view, status-mapped failures, always-mounted tab, input that must never stick).

## Common Pitfalls

### Pitfall 1: Assuming the menu or the page wins a Cmd chord
**What goes wrong:** the plan builds native menu switching that is unnecessary, or skips it and finds Cmd+W still closes the tab.
**Why it happens:** the order is contested. WebKit source and Apple's guide say view first; one Tauri report says menu first; another says view first.
**How to avoid:** Wave 0 VM probe in the real Portty.app (chord list in Pattern 1) before any native code. Record per-chord results in the plan's evidence.
**Warning signs:** a keydown logger shows no event for a chord, or the tab closes while the logger shows the event.

### Pitfall 2: Cmd-held keyup never arrives
**What goes wrong:** Cmd+C leaves C held on the office Mac; the heartbeat keeps the held set alive; later typing misbehaves on an unattended Mac.
**Why it happens:** AppKit/browsers drop keyup for non-modifier keys while Cmd is held.
**How to avoid:** Pattern 2, plus release-all on every control-mode exit and on Meta keyup. Add a VM row that holds Cmd+C and reads the daemon's held-key count.
**Warning signs:** the held set grows after chords; a "stuck" key after Cmd+Tab (blur) rather than a clean release.

### Pitfall 3: A `'desktop'` tab rendered as a terminal
**What goes wrong:** the always-mounted loop treats any tab that is not `web-session` or `preview` as a terminal and mounts a `TerminalPanel` for `desktop:<host>`.
**How to avoid:** extend the exclusion at App.tsx:1409, add the desktop branch, and add a vitest case that a desktop tab never mounts `TerminalPanel` (same style as `App.open-remote.test.tsx`).

### Pitfall 4: Sidebar says "Nothing shared" beside a Desktop row
**How to avoid:** render `PEER_NOTHING_SHARED` only when the peer has neither sessions nor a desktop; add the row first in the group (CONTEXT). Pin both with a vitest case.

### Pitfall 5: `POST /peers/desktop` blocked during a handoff
**What goes wrong:** a mutating route not in `freezeGate`'s allow list returns 503 `Retry-After: 1` while a handoff holds the sessions frozen. [VERIFIED: api.go:123-147]
**How to avoid:** exempt it with `/peers/connect` (it only caches a cap) and pin it with a test next to the existing freeze tests.

### Pitfall 6: Stale or dead desktop cap after a token event or daemon restart
**What goes wrong:** the tab loops "Reconnecting" forever: the proxy returns 404 (store emptied by a daemon restart) or the peer rejects the cap (always-available toggled, key regenerated).
**How to avoid:** call `ConnectPeerDesktop` before each redial; map close reasons (cap rejected, desktop off, unreachable) to states; "desktop off" ends the retry loop with the Phase 19 wording and the sidebar row disappears on the next poll.

### Pitfall 7: Local Network prompt behaviour
**What goes wrong:** the first Desktop tab open hangs at ICE `new` while the prompt is up, and does not recover after Allow.
**Facts:** in R13 the prompt was owned by the app ("Allow “SpikeShell” to find devices on local networks?"), appeared with no usage key in the shell's plist (generic body text), left ICE at `new` and 0 frames while unanswered, signalling kept working, and the page did not recover on its own after Allow (needed a fresh load); the grant survived a rebuild with the same signing identity. [VERIFIED: r13-shell.txt "Attempts in order" 1 to 3; no `NSLocalNetwork` key found by grep in `build/` or `spike/desktop`]
**How to avoid:** (a) the key `NSLocalNetworkUsageDescription` only customizes the body; add it to both `build/darwin/Info.plist` and `Info.dev.plist`; (b) after an ICE timeout of about 8 to 10 s show the CONTEXT line but keep redialling with backoff so Allow heals the tab without a click; (c) Wave 0 VM row records what Deny looks like (ICE `new` versus `failed`), because nothing found says; (d) the app is `LSUIElement` and toggles Accessory/Regular (Info.plist:26, tray_objc_darwin.m:226-243), so confirm in the VM that the prompt appears in front when the window is opened from the tray. PITFALLS research flagged that an Accessory app may not surface prompts in front. [VERIFIED: PITFALLS.md line 104]; Apple's TN3179 could not be read this session, so which traffic triggers the prompt is taken from R13 only. Use a Developer ID build for this row: an ad-hoc rebuild loses TCC (R10).

### Pitfall 8: iOS long-press produces no `contextmenu`
See Pattern 7. Build the timer; scope `preventDefault` to the video container.

### Pitfall 9: iOS soft keyboard layout and focus
**What goes wrong:** the page jumps, the keyboard covers the key bar, focus fails.
**How to avoid:** hidden input stays real and at 16 px or more; focus in the click handler; reuse the `--vv-height`/`--vv-top` approach with parameterized selectors. Phase 14 accepted a gap above the keyboard for iOS's own form-assist bar (M-146 accepted limit); expect the same here.

### Pitfall 10: Dead sockets after sleep report OPEN
See Pattern 8. Never trust `readyState`; probe on becoming visible.

### Pitfall 11: Pinch and double-tap fight with Safari
**What goes wrong:** a whole-view pinch opened Safari's tab overview in the simulator in Phase 14 (M-147 note), and page zoom competes with the video transform.
**How to avoid:** non-passive container listeners, `gesturestart` guard, `touch-action: none`; keep the real-iPhone pinch row (M-147 was never run on a real phone).

### Pitfall 12: Home Screen link and manifest
**Facts:** the existing manifest has no `start_url` and no `id` on purpose, "so the launch URL stays the exact share link" (csp_mw.go Amendment 4 comment, lines 100-108), and Phase 14's real-iPhone run confirmed the icon relaunch kept the query parameters (M-145: "the standalone launch kept the query"). R14's Home Screen runs carried per-icon `ice` and `tag` query values [VERIFIED: 17-SPIKE-MATRIX.md R14-homescreen row].
**Risk:** Phase 19's `/desktop/` manifest "scope" decision could add a `start_url`; search sources do not confirm per-version iOS handling of a `start_url` with a query, so it is untested ([ASSUMED] on iOS 27). Keep the `/desktop/` manifest free of `start_url` and `id`, and add a real-iPhone row that saves the always-available link and relaunches it after the viewer page is rebuilt.
**CSP:** `cspHeaders` writes `default-src 'none'` with no `media-src` (csp_mw.go:156-177). A `<video>` playing a `MediaStream` through `srcObject` is not governed by `media-src` in the spec, but no run of the strict policy with a video exists in this repo (R13 used the Wails meta CSP `frame-src blob:; child-src blob:; worker-src 'self'`; R11, R12 and R14 used the spike server). Phase 19's e2e owns the check; Phase 21's iPhone rows depend on it, so run the first iPhone row against the real `/desktop/` page and watch for a console CSP violation.

### Pitfall 13: Where CONTEXT's upstream list differs from the Phase 19/20 designs
| 21-CONTEXT assumes | Phase 19/20 design says | Handling |
|--------------------|-------------------------|----------|
| Pointer message carries click count | The daemon computes click count from timing and distance (20-CONTEXT, Input wire protocol) | Send plain down/up; double-tap is two down/up pairs; note cellular jitter can lengthen the gap |
| An explicit release-all message, or release on data-channel close | Release-all on blur, hide, pagehide, channel close, peer failure, revoke, surface switch and lock; heartbeat dead-man while anything is held; no named viewer-to-daemon message | Ask Phase 20 to name one `release` message (cheap now); otherwise send individual key-ups |
| Send-keys menu as a placeable component | Built "in the viewer from ordinary key messages", inside `DesktopView`'s top bar | Ask Phase 20 to export it as a component; else the iPhone key bar rebuilds three items |
| Control mode "offered only when the viewer has control" | Control starts on for own-account viewers, with a View only/Control toggle and a "Control needs Accessibility" state | Control mode follows the toggle and the Accessibility/locked states |
| Absolute pointer is enough for trackpad mode | Absolute only, no cursor report | Finding 4 / Open Question 2 |
| Same viewer reuse in the Wails tab | Designed for same-origin `/desktop/ws?cap=` | Finding 7 / Open Question 4 |

### Pitfall 14: Real-app items R13 did not reproduce
R13's shell had a different bundle id, no React frontend or bindings, default window options, and no signalling proxy. [VERIFIED: r13-shell.txt "Not reproduced"] Portty's window is transparent and translucent (`WebviewIsTransparent: true`, `WindowIsTranslucent: true`, main.go:149-153), so the real-app row must confirm video actually composites (a frame counter and a screenshot of the VM), not only that `RTCPeerConnection` exists.

### Pitfall 15: M-NN numbering and TESTING.md conventions
Highest existing manual item is M-164 (Phase 17), and Phases 19 and 20 will add rows before this phase runs. Allocate numbers at execution time from the then-current TESTING.md (`grep` the highest `**M-`), start a new category letter after BH or the then-latest, and follow the standing rule (Section 2 manifest, Section 4 traceability row per new test file with a repo-relative path only, Section 5 rows; run `bash tests/check-traceability-paths.sh` before committing). [CITED: ./CLAUDE.md; VERIFIED: TESTING.md Category BH ends at M-164]

### Pitfall 16: Wails bindings are committed
`frontend/src/wailsjs/go/main/App.d.ts` and `App.js` and `go/models.ts` (plus a nested `wailsjs/wailsjs/...` copy) are tracked files. `ConnectPeerDesktop` (and `SetControlMode` if needed) must be added there, and `RemotePeerSessions` in models gains `desktop`. [VERIFIED: `git ls-files frontend/src/wailsjs`]

## Code Examples

### Cap route twin (shape only, copy the real function)
```go
// Source: internal/webserver/whois.go handlePeerCap (read this session). Differences: no session id,
// the provider answers for the standing always-available token, 404 when always-available is off.
mux.HandleFunc("POST /api/peer/desktop/cap", ws.requireSameUser(ws.handlePeerDesktopCap))
```

### Discovery header
```go
// handleSessionsMeta, before the body is encoded. Only while always-available is on (CONTEXT).
if ws.desktopAlwaysOn != nil && ws.desktopAlwaysOn() {
    w.Header().Set("X-Portty-Desktop", "always")
}
```

### Frontend reads the header through the existing poll
```typescript
// RemotePeerSessions gains: desktop?: boolean  (remoteSession.ts, wailsjs models)
// Sidebar: render <DesktopRow/> first in the peer group when g.peer.desktop is true,
// and show "Nothing shared" only when there are no sessions and no desktop.
```

### Meta-held tap (Pattern 2)
```typescript
// Source: noVNC core/input/keyboard.js _handleKeyDown. sendKey is Phase 20's message sender.
if (isMac && e.metaKey && e.code !== 'MetaLeft' && e.code !== 'MetaRight') {
  sendKey(e.code, 'down', mods(e)); sendKey(e.code, 'up', mods(e))
  e.preventDefault(); return                // not added to the held set
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| iOS autoplay of muted `srcObject` video | Needs a user tap for this stream | Measured in R14 on iOS 27.2.1 | The viewer starts from a Tap to start overlay (already in Phase 19) |
| Screen Wake Lock not in Home Screen apps | Works from iOS 18.4 | iOS 18.4 | Optional; not needed for SC3 |
| Terminal-only peer tabs | Peer desktop tab (this phase) | v2.0 | Needs its own mount path |

**Deprecated/outdated:** none relevant. Wails v2 stays (v3 is not in `go.mod`).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `preventDefault` on keydown stops Cmd+W/Q/C reaching the Wails menu in Portty's WKWebView | Pattern 1 | A native fallback is needed (planned: blank key equivalents) |
| A2 | WKWebView in the Wails app swallows keyup for keys pressed with Cmd held, as Chromium and Firefox do | Pattern 2 | The tap workaround would double-release (harmless) |
| A3 | `display:none` on a live `<video>` can stall WebKit decode or frame callbacks | Pattern 6 | The visibility-based hide is unnecessary |
| A4 | `preventDefault` on `touchstart` suppresses the iOS long-press callout and selection for that touch | Pattern 7 | Long press shows iOS's callout; add `-webkit-user-select:none` and CSS callout rules |
| A5 | Reusing one `<video>` element avoids a second tap after sleep | Pattern 8 | A second Tap to start after every sleep (CONTEXT already allows the overlay) |
| A6 | A sentinel character makes Backspace observable on iOS soft keyboards | Pattern 9 | Backspace needs the key bar |
| A7 | `/desktop/` Home Screen launch keeps the query on iOS 27 if the manifest has no `start_url` | Pitfall 12 | Saved links stop working (M-145 evidence is for `/app/`) |
| A8 | The strict share CSP (`default-src 'none'`, no `media-src`) does not block a `MediaStream` `<video>` | Pitfall 12 | An amendment to the share CSP is needed (Phase 19 e2e) |
| A9 | Local Network Deny leaves ICE at `new` (not `failed`) | Pitfall 7 | The "needs Local Network" line triggers on the wrong state |
| A10 | Two-finger scroll direction through Phase 20's wheel message matches macOS natural scrolling on the remote Mac | Pitfall 13 / test rows | Inverted scroll; flip the sign in one place |
| A11 | Cellular jitter between a double tap's two clicks stays under the Mac's double-click interval | Pitfall 13 | Double-tap acts as two single clicks |
| A12 | Phase 19 will serve the same SPA bundle under `/desktop/` and tolerate an injected transport | Findings 7 | Wave 0 builds the seam here instead |

## Open Questions / Decisions Needed

1. **Cmd+V in control mode.**
   - What we know: 21-CONTEXT says explicit paste when Phase 20 agrees; Phase 20 says plain key and never sync. Wails' clipboard cannot filter concealed items.
   - What's unclear: whether the user wants Cmd+V to carry the home clipboard (FEATURES research suggested it).
   - **Recommendation:** ship the plain key (follows Phase 20, no new ObjC, no pasteboard privacy alert). If the user wants Cmd+V to paste across Macs, amend 20-CONTEXT first, then read the clipboard with a small ObjC that skips `org.nspasteboard.ConcealedType`.
2. **Host cursor position or relative pointer (Phase 20 amendment).**
   - What we know: absolute normalized positions only; no cursor report; Phase 20 is context only.
   - What's unclear: whether the user accepts virtual-pointer drift on the iPhone.
   - **Recommendation:** amend 20-CONTEXT to add an optional relative move message (`dx, dy`, daemon adds to the current cursor and clamps), which also fixes drift after another viewer or the office mouse moves the cursor. If declined, build the first-touch-jump fallback and say so in the docs.
3. **Hidden Portty window with a Desktop tab connected.**
   - What we know: `HideWindowOnClose: true`; a connected viewer holds the office display awake; CONTEXT defers hidden-tab pausing.
   - **Recommendation:** disconnect Desktop tabs after about 2 minutes of `document.visibilityState === 'hidden'` and redial when visible (about 15 lines). If the user prefers CONTEXT as written, record the cost in Help.
4. **Seams Phase 21 needs from Phases 19 and 20.**
   - Needed: (a) `desktopClient` takes a signalling URL provider and a before-redial hook, with `portty:wake` as a trigger; (b) a mode or entry so `/desktop/` is not read as the Wails shell by `detectMode()`; (c) the SPA bundle (or a second entry) served under `/desktop/` with relative assets; (d) a `/desktop/` manifest with no `start_url`/`id`; (e) Phase 20: a named release message, a placeable Send-keys component, Meta-chord tap handling, and `preventDefault` on all forwarded keydown.
   - **Recommendation:** add (a) to (e) to the Phase 19/20 plans now; if they ship first without them, Phase 21 Wave 0 owns them.
5. **Release chord.** Recommendation: keep the CONTEXT candidate Ctrl+Option+Cmd+Return (`code === 'Enter'`). Avoid Esc (vim needs it) and anything in the system shortcut set. Confirm no clash on the home Mac during the Wave 0 probe.
6. **Wake Lock on the iPhone.** Recommendation: skip (not needed for SC3; radical simplicity).

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | Go tests | yes | go1.27.4 local (module pins go1.26.4, toolchain go1.26.11) | Use the project's pinned toolchain path from `build.sh` |
| Node and pnpm | Vitest, build | yes | node v24.22.1, pnpm 12.6.0 | none needed |
| Wails CLI | Bindings, builds | yes | `~/go/bin/wails` (v2 library 2.10.2) | none needed |
| Xcode | cgo ObjC fallback, builds | yes | 27.2 | none needed |
| Tart VMs | GUI row (real Portty.app, Local Network prompt, key routing) | yes | `portty-test` and `portty-test-2` running (`tart list`, macOS 27.2 per Phase 17) | none; host never used for GUI checks |
| Real iPhone with the Tailscale app | iPhone gesture, keyboard and sleep rows | user session | iOS 27.2.1 in Phase 17 | Playwright WebKit iPhone emulation for layout only |
| Phase 19/20 code (DesktopView, desktopClient, /desktop/ws, input channel, desktop token) | Everything in the Desktop tab | no (designed, not built) | none | Wave 0 stubs behind the same interfaces; real-app row waits for them |

**Missing dependencies with no fallback:** Phase 19/20 implementation (a sequencing dependency, not an environment gap). The Desktop-tab VM row cannot run until the office VM runs a Phase 19/20 daemon.
**Missing dependencies with fallback:** none.

## Security Domain

`security_enforcement: true`, ASVS level 1, block on high (config.json).

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | Tailscale WhoIs same-user gate (`requireSameUser`) on the cap route and `/desktop/ws`; desktop token is a second factor |
| V3 Session Management | yes | Standing token in daemon memory only; killed by always-available off or Regenerate Signing Key; never in the web view URL |
| V4 Access Control | yes | Same-account peers only; desktop token refused on `/sessions/*`, `/api/sessions*` and port routes (DSHR-08, tested in Phase 19, re-pinned here for the new route) |
| V5 Input Validation | yes | 8 KiB body cap and JSON content type on the cap route; host matched only against tailscaled's peer list; proxy never builds a URL from peer-supplied data |
| V6 Cryptography | yes | HKDF-derived token is Phase 19's; this phase adds none. TLS verified against the MagicDNS name via `peerClient` |
| V7 Error handling and logging | yes | Never log the cap, URL query, key codes, text or clipboard (existing `redact` helper; Phase 20 logging rule) |
| V14 Configuration | yes | Info.plist usage string; no new entitlements; CSP unchanged on the Wails page |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| A local process opens `ws://127.0.0.1:<relayPort>/api/relay/remote-desktop/<host>/ws` and drives the office Mac | Spoofing / Elevation | Same exposure as the terminal proxy today (an empty Origin passes `websocket.Accept`'s check); accepted parity because same-uid processes already reach the 0600 daemon socket, but other local users can reach the TCP relay port. Note in the security review; do not claim stronger |
| Standing token leaks through logs or error text | Information disclosure | `redact` path from the terminal proxy; token only in the dial query; test that no log line or close reason contains it |
| Cross-token use (desktop token on terminal routes, session cap on desktop routes) | Elevation | Cross-token refusal tests in both directions on the new routes |
| Stuck remote key or modifier on an unattended Mac | Denial of service / Tampering | Meta-chord tap rule, release-all on every exit and on Meta keyup, Phase 20 dead-man |
| Control mode swallowing local escape keys | Denial of service | Reserved release chord never forwarded; click-outside and blur exits; page-level capture fails open |
| Clipboard exfiltration by auto-read | Information disclosure | No automatic clipboard read in this phase (Cmd+V is a plain key); explicit toolbar actions only (Phase 20) |
| Peer header spoofing makes a row appear | Spoofing | The row only leads to the same-user-gated route; a spoofed header on another account's Mac cannot pass `requireSameUser` (403 hides the group) |
| Local Network denial or prompt fatigue | Availability | Plain explanatory line plus Open System Settings; keep retrying |

## Test and TESTING.md Obligations

`workflow.nyquist_validation` is `false`, so there is no Validation Architecture section. The project's standing rule still applies. [CITED: ./CLAUDE.md]
- **Go (unit, no live tailnet, test binary refuses `tailnet.LiveAllowed()`):** header present only when on; body stays a bare array; v1.x-shaped client ignores the header; `FetchPeerSessionsMetaWithClient` and the 2-value seam unchanged; cap route same-user only, 404 when off, JSON content type enforced, `no-store`, desktop token refused on `/sessions/*`; `DesktopCapStore` no-disk grep test; `POST /peers/desktop` through `SetPeerSeamsForTest`, exempt from `freezeGate`; proxy Origin injection byte-exact, long-lived ctx, no token in the client URL or logs, status-to-close-reason mapping.
- **Vitest (mock containers and plain event objects for touch):** Desktop row first in the group and focus-on-second-click, "Nothing shared" rule, desktop tab never mounts `TerminalPanel`, control-mode state machine (entry on click, exits on outside click, tab switch, blur, not-connected, release chord; release-all on each), Meta-chord tap rule, gesture recognizer, coordinate mapping, `portty:wake` redial, `ConnectPeerDesktop` before each redial.
- **Playwright iPhone project:** layout only (safe areas, bottom bar, key bar position); gestures are not reliable there.
- **Tart VM rows (real Portty.app, Developer ID signed, `portty-test-2` viewing `portty-test`):** Wave 0 key-routing probe, Cmd+Q/W/C forwarded and given back, hold-Cmd+C held-key count, Local Network prompt (appears in front, Allow heals the tab, what Deny does), transparent-window video composites.
- **User-session M-NN rows:** iPhone trackpad, tap, double-tap, long-press right-click, drag, two-finger scroll direction, pinch and pan to 5x, soft keyboard (typing, Backspace, Enter, arrows), sleep and resume with and without the Tap to start overlay, Home Screen relaunch keeps `?cap=`, saved link survives a viewer-page rebuild.
- Each new test file gets a Section 4 row (repo-relative path only) and `bash tests/check-traceability-paths.sh` runs before commit.

## Sources

### Primary (HIGH confidence, read this session)
- In-repo: `internal/daemon/remote_ws_proxy.go`, `remote_caps.go`, `peer_connect.go`, `peer_client.go`, `relay_remote_files.go`, `api.go` (freezeGate and routes); `internal/webserver/whois.go`, `server.go` (routes, `handleSessionsMeta`), `csp_mw.go`; `internal/tailnet/sessions.go`; `internal/relay/server.go` (`loopbackOriginPatterns`); `app.go` (`GetRemoteSessionsWithMeta`, `ConnectPeerSession`); `main.go` (menu, window options); `build/darwin/Info.plist`; `tray_objc_darwin.m`; frontend `App.tsx`, `Sidebar.tsx`, `TabBar.tsx`, `lib/tabs.ts`, `sidebarPeers.ts`, `relayClient.ts`, `phoneQuery.ts`, `pinchZoom.ts`, `visualViewportInset.ts`, `KeyBar.tsx`, `remoteSession.ts`, `webMode.ts`, `index.html`.
- Wails v2.10.2 source in the module cache: `internal/frontend/desktop/darwin/{Application.m,window.go,menu.go,menuitem.go,WailsMenu.m,clipboard.go}`.
- Planning: `21-CONTEXT.md`, `19-CONTEXT.md`, `20-CONTEXT.md`, `17-SPIKE-MATRIX.md` (R13, R13-lite, R13-full, R14 rows), `evidence/r13-shell.txt`, `evidence/r14-iphone-reports.json`, `.planning/research/{ARCHITECTURE,FEATURES,PITFALLS,STACK}.md`, `TESTING.md` Category BE and BH.
- WebKit source `WebViewImpl.mm` (`performKeyEquivalent`, `doneWithKeyEvent`): raw.githubusercontent.com/WebKit/WebKit/main/Source/WebKit/UIProcess/mac/WebViewImpl.mm.
- noVNC `core/input/keyboard.js`: raw.githubusercontent.com/novnc/noVNC/master/core/input/keyboard.js.

### Secondary (MEDIUM confidence)
- Apple archived Cocoa guide, Handling Key Events: developer.apple.com/library/archive/documentation/Cocoa/Conceptual/EventOverview/HandlingKeyEvents/HandlingKeyEvents.html (from search summary, not opened in full).
- github.com/superbiche/marque/pull/47 (page first); github.com/block/buzz/pull/4875 (menu first, unmeasured): in conflict.
- bugs.webkit.org/show_bug.cgi?id=55291; bugzilla.mozilla.org/show_bug.cgi?id=1299553 (Cmd-held keyup).
- github.com/3ll3d00d/ezbeq/pull/169; github.com/enisdenjo/graphql-ws/issues/289 (iOS socket drops, dead-but-open sockets).

### Tertiary (LOW confidence, web search only, marked for device validation)
- iOS `contextmenu` absence and callout CSS: bugs.webkit.org/show_bug.cgi?id=213953; developer.apple.com/forums/thread/808606.
- iOS hidden-input Backspace behaviour: phabricator.wikimedia.org/T69262; bugs.webkit.org/show_bug.cgi?id=268988.
- Wake Lock in Home Screen apps from iOS 18.4: webkit.org/b/254545.
- Apple TN3179 (local network privacy) could not be read (page did not render); no claim here depends on it.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH, no new packages; versions read from `go.mod` and `package.json`.
- Architecture (twins, discovery, tab model): HIGH for the in-repo twins; MEDIUM where Phase 19/20 are unbuilt.
- Control-mode routing: MEDIUM (source plus conflicting field reports; probe decides).
- iPhone touch, keyboard, resume: LOW to MEDIUM (web reports and Phase 14 evidence; device rows decide).
- Pitfalls: MEDIUM to HIGH, most come from reading the code and R13 evidence.

**Research date:** 2026-11-25
**Valid until:** 2026-12-25 for in-repo findings (code moves with Phases 19 and 20); iOS and WebKit behaviour should be re-checked on the device at execution time.
