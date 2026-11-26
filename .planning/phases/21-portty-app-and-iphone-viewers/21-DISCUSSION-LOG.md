# Phase 21: Portty App and iPhone Viewers - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-11-25
**Phase:** 21-portty-app-and-iphone-viewers
**Areas discussed:** Peer discovery and authorization, Desktop tab in the Portty app, Control mode (iPhone viewer was all reversible or Claude's Discretion and not presented)

**Mode:** autonomous smart discuss. The user delegated on 2026-11-25 ("I don't need to answer a bunch of questions"). Reversible technical choices were decided autonomously, product and UX choices went to Claude's Discretion, and only hard-to-reverse questions were presented, each with a recommendation. The user replied "Accept all" for all five.

---

## Peer discovery and authorization

Q1: How does the home Mac's sidebar learn that a peer has an always-available desktop?

| Option | Description | Selected |
|--------|-------------|----------|
| Header `X-Portty-Desktop: always` on GET /api/sessions/meta | Body stays a bare array so v1.x peers parse it; no extra request per peer; sent only while always-available is on; unknown values ignored (costly) | ✓ |
| New GET /api/desktop/info per peer | Clean, 404 means no desktop, but doubles the sidebar poll and adds a route every later build keeps | |
| Change the meta JSON to an object | Breaks older peers' bare-array parse (tailnet/sessions.go) | |

Q2: How does the Portty app get the right to open a peer's always-available desktop?

| Option | Description | Selected |
|--------|-------------|----------|
| POST /api/peer/desktop/cap behind requireSameUser | Twin of handlePeerCap; returns the standing token to the peer daemon, held in memory only, never in the web view URL; WhoIs checked again on /desktop/ws (costly) | ✓ |
| Short-lived peer token (mode peer, exp claim) per connect | More defence in depth, but adds an expiry to Phase 19's v1 token format and verifier | |
| No peer endpoint; user pastes the link | Fails "straight from the sidebar" | |

**User's choice:** Accept all (recommended options)
**Notes:** None beyond the acceptance.

---

## Desktop tab in the Portty app

Q3: How is the macOS Local Network prompt from WebRTC in the Portty web view handled on the viewer Mac?

| Option | Description | Selected |
|--------|-------------|----------|
| Accept the prompt with NSLocalNetworkUsageDescription | Prompt on first Desktop tab open; on ICE stuck at new, Open System Settings and Open in Browser; no other grant (costly) | ✓ |
| Sidebar row always opens the browser link | No prompt in Portty, but no in-app tab, so DVWR-01's tab is dropped | |

**User's choice:** Accept all (recommended option)
**Notes:** WebKit gives the page no way to limit ICE gathering to the tailnet interface, so there was no third option.

---

## Control mode

Q4: Which shortcuts does control mode take from Portty, and how does the user always get the app back?

| Option | Description | Selected |
|--------|-------------|----------|
| Forward every chord the Portty menu owns | Ends on click outside, tab switch, app deactivation, stream not connected, or a reserved never-forwarded release chord; release-all on exit; no event tap (costly) | ✓ |
| Forward only Cmd+W, Q and C | Smaller, but Cmd+V, Z and T act locally while typing remotely | |
| System-wide CGEventTap | Also captures Cmd+Tab and Cmd+Space, but needs Accessibility or Input Monitoring on the viewer Mac and duplicates the Send-keys menu | |

Q5: In control mode, does Cmd+V send the home Mac's clipboard to the remote?

| Option | Description | Selected |
|--------|-------------|----------|
| Yes, a typed Cmd+V is the explicit paste | Home clipboard plain text (never concealed) via Phase 20's paste message, then the keystroke; follows Phase 20 if it rules otherwise for browsers (costly) | ✓ |
| Plain key only | The remote pastes its own clipboard; home paste only from Phase 20's Paste menu item | |

**User's choice:** Accept all (recommended options)
**Notes:** None beyond the acceptance.

---

## Decided autonomously

[auto] [Proxy] — Q: "How does the Wails tab reach the peer's /desktop/ws?" → Selected: "GET /api/relay/remote-desktop/{host}/ws on the relay loopback mux, twin of handleRemoteSessionWS, signalling only" (reversible technical — decided autonomously: loopback-only route, both ends in one binary)
[auto] [Proxy] — Q: "Where is the peer desktop token held?" → Selected: "In-memory sibling of RemoteCapStore keyed by host, refreshed per connect" (reversible technical — decided autonomously: internal store)
[auto] [Binding] — Q: "How does the click become a connection?" → Selected: "ConnectPeerDesktop binding, daemon POST /peers/desktop, tab on the loopback proxy URL with no token" (reversible technical — decided autonomously: internal API and binding)
[auto] [Discovery] — Q: "Where is the header parsed?" → Selected: "tailnet/sessions.go sets PeerSessionMetaGroup.Desktop; sidebarPeers.ts renders the row" (reversible technical — decided autonomously: internal fields)
[auto] [Tab] — Q: "How is the tab typed?" → Selected: "Tab.type 'desktop' with peerHost and name, not persisted" (reversible technical — decided autonomously: frontend types, nothing stored)
[auto] [Tab] — Q: "Does the Wails page need its own CSP or WebKit preference?" → Selected: "No, per R13 defaults; add only if the real-app check fails" (reversible technical — decided autonomously: build options)
[auto] [Control] — Q: "How are menu key equivalents switched off in control mode?" → Selected: "Research picks: menu swap without accelerators, else an NSEvent local monitor; SetControlMode binding with a Go-side restore guard" (reversible technical — decided autonomously: local GUI code behind one binding)
[auto] [Control] — Q: "How do forwarded keys reach the remote?" → Selected: "Phase 20's key message unchanged" (reversible technical — decided autonomously: no new protocol)
[auto] [iPhone] — Q: "How does trackpad mode talk to the host?" → Selected: "Client-side virtual pointer sending absolute positions" (reversible technical — decided autonomously: client-side math)
[auto] [iPhone] — Q: "How is phone mode detected?" → Selected: "lib/phoneQuery.ts PHONE_QUERY, never the user agent" (reversible technical — decided autonomously: reuse)
[auto] [iPhone] — Q: "How do touch handlers avoid page zoom and scroll fights?" → Selected: "Non-passive container listeners, gesturestart guard, CSS-transform zoom" (reversible technical — decided autonomously: local handler code)
[auto] [Testing] — Q: "How are DVWR-01/03/04 proven?" → Selected: "Vitest, Go proxy/header/route tests, a Tart VM real-app GUI row, iPhone M-NN rows, TESTING.md updates" (reversible technical — decided autonomously: test layout)

## Claude's Discretion

User delegated: 'I don't need to answer a bunch of questions' (2026-11-25). Covered: Desktop row placement, glyph and muted states; one tab per peer; the tab staying connected while open; state wording; control-mode entry, ring and label; the release chord choice; the Local Network denial copy; iPhone gestures, pinch to 5x, soft keyboard and desktop key bar, tap-to-start after sleep, and the full-bleed phone layout. The full list with reasons is in 21-CONTEXT.md.

## Deferred Ideas

Direct-touch mode on iPhone (Future); three-finger iPhone gestures; audio unmute (Phase 23); desktop rows for other people's shares (Phase 22); idle disconnect of a hidden Desktop tab; the Keyboard Lock API for Chrome fullscreen.
