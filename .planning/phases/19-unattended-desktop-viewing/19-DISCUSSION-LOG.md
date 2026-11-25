# Phase 19: Unattended Desktop Viewing - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-11-25
**Phase:** 19-unattended-desktop-viewing
**Areas discussed:** Always-available link and token, Daemon under launchd, Permissions and approval lapse (Media pipeline, viewer and reconnect was all reversible and not presented)

**Mode:** autonomous smart discuss. The user delegated on 2026-11-25 ("I don't need to answer a bunch of questions"). Reversible technical choices were decided autonomously, product and UX choices went to Claude's Discretion, and only hard-to-reverse questions were presented, each with a recommendation. The user replied "Accept all" for all six.

---

## Always-available link and token

Q1: What URL does an always-available desktop link have?

| Option | Description | Selected |
|--------|-------------|----------|
| `/desktop/?cap=<token>` on the share server (7444) | Own path, gate and manifest scope; not under /app/; no session ID in the path; no-store, no-referrer (one-way) | ✓ |
| `/app/?desktop=1&cap=<token>` | Reuses the SPA manifest and launcher, but ties desktop links to the terminal SPA's same-user gate; Phase 22 would need a second path | |
| `/desktop/<id>?cap=` redirecting to /app/ | Terminal-style redirect; the Home Screen saves the post-redirect URL, so two shapes become contracts | |

Q2: What token does the link carry, and how long does it live?

| Option | Description | Selected |
|--------|-------------|----------|
| HKDF-separated desktop token from capability.key | Info portty/desktop-token/v1, kind desktop, mode always, grant_id, no expiry; revoked when always-available is turned off or the key is regenerated; same-user check first; DSHR-08 by construction (one-way) | ✓ |
| No token, WhoIs same-user only | Simplest and never breaks, but drops the second factor and leaves DSHR-08 to route checks alone | |
| Reuse capability.Claims under the session key | No new code, but a desktop token would verify on session routes unless every route special-cases it | |

Q3: Where do the always-available setting and its grant ID live on disk?

| Option | Description | Selected |
|--------|-------------|----------|
| Separate desktop.json (0600) | Survives a Phase 18 rollback or older daemon, which drop unknown settings.json fields on save (costly) | ✓ |
| omitempty `desktop` object in settings.json | One config file, but a rollback or older daemon erases it | |
| Not persisted | Breaks DSHR-01 with the GUI closed and DSHR-02 across restarts | |

**User's choice:** Accept all (recommended options)
**Notes:** None beyond the acceptance.

---

## Daemon under launchd

Q4: When is the LaunchAgent installed, and which start paths go through it?

| Option | Description | Selected |
|--------|-------------|----------|
| Portty-written agent for every user on first GUI start | Label app.portty.Portty.daemon, RunAtLoad, KeepAlive SuccessfulExit false, every start through launchctl kickstart, legacy portty-daemon plist removed (costly) | ✓ |
| Install only while always-available is on | Smaller change for terminal-only users, but two start paths and the SSH trap stays for them | |
| Keep the kardianos portty-daemon label, KeepAlive false | No new plist writer, but no crash restart on an unattended Mac | |

Q5: How does the daemon stay launchd-managed across a v1.2 upgrade handoff?

| Option | Description | Selected |
|--------|-------------|----------|
| Re-home after the takeover | Keep the v1.2 takeover and HandoffVersion 1, then kickstart the launchd job, which takes over the sessions with the same protocol; a second millisecond-scale handoff (costly) | ✓ |
| Accept a GUI-spawned daemon until the next login | No new code, but a crash after an upgrade with the GUI closed leaves the Mac unreachable | |
| Takeover through launchd under a second label used in turn | One handoff, but two plists and two Login Items entries | |

**User's choice:** Accept all (recommended options)
**Notes:** None beyond the acceptance.

---

## Permissions and approval lapse

Q6: Does Portty ask for Full Disk Access to read the approval store for lapse detection?

| Option | Description | Selected |
|--------|-------------|----------|
| No; Screen Recording and Accessibility only | Store date if readable, Portty's own last-approval date with a 30-day warning, pointer nudge plus frame check while a viewer is connected (costly) | ✓ |
| Optional Full Disk Access grant | Exact LastAlerted date, at the cost of a broad permission and a third Settings entry | |
| Frames and errors only, no nudge | No pointer movement, but a lapse can go unseen (R19) | |

**User's choice:** Accept all (recommended option)
**Notes:** None beyond the acceptance.

---

## Decided autonomously

[auto] [Link] — Q: "Does the desktop get its own server?" → Selected: "Same share server; serverNeeded() replaces len(a.shares)==0" (reversible technical — decided autonomously: internal lifetime logic)
[auto] [Link] — Q: "Route gate order for desktop routes?" → Selected: "Origin, same-user, desktop token, mirroring /sessions/{id}/ws" (reversible technical — decided autonomously: middleware composition)
[auto] [Link] — Q: "How is DSHR-08 proven?" → Selected: "Cross-token refusal tests for session, desktop and port tokens" (reversible technical — decided autonomously: tests only)
[auto] [Link] — Q: "When can always-available start?" → Selected: "Tailscale and HTTPS certs required; health poller rebinds on IP or cert change" (reversible technical — decided autonomously: runtime policy)
[auto] [Link] — Q: "What if Tailscale runs in userspace-networking mode?" → Selected: "Refuse always-available with a clear state (M-164)" (reversible technical — decided autonomously: local refusal)
[auto] [CSP] — Q: "Does the desktop page need a CSP change?" → Selected: "No new directive; a test pins it; amend only on an e2e block" (reversible technical — decided autonomously: header string)
[auto] [Launchd] — Q: "How does the CLI respawn the daemon from SSH?" → Selected: "Kickstart only, never exec; guidance if the agent is missing" (reversible technical — decided autonomously: local logic)
[auto] [Launchd] — Q: "How do tests avoid the user's launchd domain?" → Selected: "testing.Testing() refusal plus Runner seam" (reversible technical — decided autonomously: test seam)
[auto] [Crash containment] — Q: "How does in-daemon capture avoid taking sessions down?" → Selected: "ObjC @try, Go recover, errors become state; KeepAlive backstop" (reversible technical — decided autonomously: local code)
[auto] [Permissions] — Q: "Which process checks grants and runs the test capture?" → Selected: "The daemon, polled about once a second; GUI opens Settings panes" (reversible technical — decided autonomously: internal API)
[auto] [Permissions] — Q: "Who posts the lapse notification with the GUI closed?" → Selected: "GUI if running, else the daemon under the same bundle identity" (reversible technical — decided autonomously: internal routing)
[auto] [Power] — Q: "How are the DPRM-04 assertions held?" → Selected: "Display and system idle-sleep assertions while viewers are connected, user activity on join, restart on -3815 after wake" (reversible technical — decided autonomously: runtime policy)
[auto] [Lock] — Q: "Where does the locked state come from?" → Selected: "CGSSessionScreenIsLocked, polled; lock-screen frames stream" (reversible technical — decided autonomously: local read)
[auto] [State] — Q: "What is the state model?" → Selected: "Daemon enum pushed to viewers and the GUI" (reversible technical — decided autonomously: internal enum)
[auto] [Packages] — Q: "How is the code laid out?" → Selected: "internal/screencap and internal/desktop" (reversible technical — decided autonomously: internal names)
[auto] [Capture] — Q: "When does capture run?" → Selected: "Only while viewers are connected, plus the test capture" (reversible technical — decided autonomously: runtime policy)
[auto] [Capture] — Q: "Which display?" → Selected: "Main display only until Phase 20" (reversible technical — decided autonomously: swap later)
[auto] [Encode] — Q: "What encoder settings?" → Selected: "VT H.264 CB realtime, no reordering, Annex-B, IDR on join and PLI, fixed bitrate" (reversible technical — decided autonomously: encoder constants)
[auto] [WebRTC] — Q: "How are peers set up?" → Selected: "Peer per viewer, shared track, ICE-lite, UDP4, mDNS off, tailnet IP only, fixed UDP mux port" (reversible technical — decided autonomously: server-side, port in SDP)
[auto] [Signalling] — Q: "What signalling protocol?" → Selected: "Non-trickle JSON over WSS at /desktop/ws with a version field" (reversible technical — decided autonomously: both ends ship together)
[auto] [DESK-02] — Q: "How is tailnet-only asserted?" → Selected: "Bind and no-STUN/TURN tests plus M-157 direct-path check" (reversible technical — decided autonomously: tests and manual rows)
[auto] [Viewer] — Q: "How is the viewer built?" → Selected: "React DesktopView.tsx and lib/desktopClient.ts, fit letterbox, muted autoplay with tap fallback" (reversible technical — decided autonomously: frontend code)
[auto] [Reconnect] — Q: "How do viewers reconnect?" → Selected: "Capped backoff 1-10 s with Reconnecting shown" (reversible technical — decided autonomously: client logic)
[auto] [Build] — Q: "What deployment target?" → Selected: "macOS 14 cgo target and LSMinimumSystemVersion 14.0; universal until REL-01" (reversible technical — decided autonomously: build flags)
[auto] [Testing] — Q: "What test approach?" → Selected: "Synthetic frame source loopback tests, VM rows, real-Mac M-NN rows, TESTING.md updates" (reversible technical — decided autonomously: test layout)

## Claude's Discretion

User delegated: 'I don't need to answer a bunch of questions' (2026-11-25). Covered: the Settings ▸ Sharing Desktop group with Copy Link (no QR code); reuse of the CT acknowledgement; a static off page for old links; Login Items onboarding copy; asking for Accessibility in Phase 19; the onboarding sheet layout; state wording; one notification per lapse; host state in Settings and a tray menu line. The full list with reasons is in 19-CONTEXT.md.

## Deferred Ideas

Display and window choice and input (Phase 20); Portty app tab and iPhone touch viewer (Phase 21); click-to-share (Phase 22); audio, adaptive quality, unlock, arm64 release (Phase 23); entitlement wiring, data-channel cursor, HEVC, H.264 High (Future); QR code and `portty desktop` CLI verbs.
