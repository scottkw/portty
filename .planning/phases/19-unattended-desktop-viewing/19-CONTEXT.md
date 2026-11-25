# Phase 19: Unattended Desktop Viewing - Context

**Gathered:** 2026-11-25
**Status:** Ready for planning

<domain>
## Phase Boundary

Turn on always-available desktop on the office Mac once, then open its live main display from home in desktop Chrome or Safari (and from a saved iPhone Home Screen link) any time, with the Portty GUI quit and nobody at the Mac. In scope: guided Screen Recording and Accessibility grants with a test capture (DPRM-01); the desktop state shown to the host and to viewers (DPRM-02); approval-lapse detection and a notification (DPRM-03); display-awake and system-sleep assertions plus a stream restart after wake (DPRM-04); a sharp, fitted view (DESK-01); tailnet-only media, asserted by tests (DESK-02); automatic reconnect (DESK-03); several viewers, each with a picture on join (DESK-04); the daemon under launchd for the always-on path (DSHR-01); links that survive daemon restarts and upgrades (DSHR-02); desktop and terminal tokens never cross (DSHR-08); Chrome and Safari viewers (DVWR-02). The largest changes to existing code are the share server's lifetime (it stays up while a terminal share or the desktop needs it) and the daemon's start path. Terminal sessions and the v1.2 upgrade handoff must not regress. Not in this phase: input and control, display or window choice and switching (Phase 20); the Portty app Desktop tab and the iPhone touch viewer (Phase 21); click-to-share for other tailnet people, admit, revoke, indicator and audit log (Phase 22); audio, adaptive quality, unlock and the arm64 release (Phase 23).

</domain>

<decisions>
## Implementation Decisions

User delegation (2026-11-25): "I don't need to answer a bunch of questions". Reversible technical choices were decided autonomously, product and UX choices are under Claude's Discretion, and the six hard-to-reverse recommendations below were accepted by the user with "Accept all" (2026-11-25).

### Always-available link and token
- The always-available desktop link is `https://<mac>.<tailnet>.ts.net:7444/desktop/?cap=<token>`. It uses the same share server, port and TLS name as terminal links, with its own /desktop/ path, gate and manifest scope. It is not under /app/, whose whole bundle sits behind the terminal same-user page gate, so Phase 22 guest links can share the /desktop/ route family with a different gate. There is no session ID in the path because the token names the grant. The page is served no-store with Referrer-Policy no-referrer — **Reversibility:** one-way — iPhone Home Screen links and browser bookmarks save this URL, so a change breaks every saved link.
- The link carries a desktop token signed under a key derived by HKDF from the existing capability.key (info `portty/desktop-token/v1`, the porttoken.go precedent). Claims: kind "desktop", mode "always", grant_id, iat, v 1; no expiry. The token stays valid while always-available is on with that grant ID; turning always-available off revokes it, and Regenerate Signing Key kills it. The WhoIs same-user check still runs first, so the token is a second factor. HKDF separation makes DSHR-08 hold by construction — **Reversibility:** one-way — the token format is embedded in every saved link and verified by every later build.
- The always-available setting and its grant ID live in a separate `desktop.json` (0600) in the config dir, holding alwaysAvailable, grantID and schemaVersion, written atomically by the daemon. Not settings.json: engine.go drops unknown fields on save, so a Phase 18 rollback (UPD-07) or any older daemon would silently erase always-available and break saved links — **Reversibility:** costly — a persisted on-disk schema; moving it later needs a migration in every build that reads it.

### Daemon under launchd
- Portty writes and bootstraps its own LaunchAgent the first time the GUI starts, for every user, not only when always-available is on. Label `app.portty.Portty.daemon` in ~/Library/LaunchAgents, ProgramArguments `<bundle>/Contents/MacOS/portty daemon`, RunAtLoad true, KeepAlive {SuccessfulExit false} (a crash restarts the daemon with nobody at the Mac, and Quit All with exit 0 stays down), no ProcessType key (R04 reference rows). Every start (GUI ensure, CLI ensure, GUI respawn poller) becomes `launchctl kickstart gui/<uid>/<label>`. A legacy kardianos `portty-daemon` plist from `portty daemon install` is booted out and removed. One start path for everyone removes the SSH attribution trap (R06) for terminal-only users too — **Reversibility:** costly — a persisted plist and label on every user's disk and in Login Items, and the process start model every start path depends on.
- The daemon stays launchd-managed across an upgrade by re-homing. The v1.2 takeover and HandoffVersion 1 stay exactly as they are (GUI-spawned takeover, two-phase commit; R05 shows a GUI-spawned daemon has the same TCC identity). Once the new daemon is up, the GUI kickstarts the launchd job, whose `portty daemon` sees a same-version daemon that launchd does not manage holding the lock, and takes its sessions over with the same handoff protocol. The cost is a second millisecond-scale handoff per upgrade, on code paths that are already tested — **Reversibility:** costly — it fixes the upgrade sequence and process architecture that Phase 18 self-install and every later upgrade rely on.

### Permissions and approval lapse
- Portty asks only for Screen Recording and Accessibility, never Full Disk Access. Lapse detection uses three signals: (a) the approval-store date, only if it happens to be readable; (b) Portty's own record of when it last saw a fresh approval, with a warning near 30 days; (c) while a viewer is connected and frames stop changing, a one-pixel pointer nudge and return (Accessibility) plus a frame check. A nudge that brings no new frame reads as waiting for approval at the Mac — **Reversibility:** costly — it sets the permission set users are walked through and the System Settings entries on the corporate Mac, and adding a grant later means another in-person trip.

### Decided autonomously
- [Link] Q: "Does the desktop get its own server?" → Selected: "Same share server and listener as terminal links; serverNeeded() (any terminal share or always-available) replaces len(a.shares)==0 in unshareLocked and in the handoff rebuild path; tests pin both directions" — internal lifetime logic
- [Link] Q: "Route gate order for desktop routes?" → Selected: "Page: cspHeaders, requireSameUserPage, desktop-token page check; WebSocket: requireAllowedOrigin, requireSameUser, desktop-token check (mirrors /sessions/{id}/ws)" — middleware composition, local
- [Link] Q: "How is DSHR-08 proven?" → Selected: "Cross-token tests: a session cap on /desktop/* and a desktop token on /sessions/*, /api/sessions* and port routes all refuse; a port token refuses on both" — tests only
- [Link] Q: "When can always-available start?" → Selected: "Needs Tailscale connected and tailnet HTTPS certs on (the shareOn checks); the daemon health poller restarts the share server when the tailnet IP or cert changes and retries while Tailscale is down" — runtime policy
- [Link] Q: "What if Tailscale runs in userspace-networking mode?" → Selected: "Detect it and refuse always-available with a clear state (milestone research gap; M-164)" — a local refusal, easy to relax
- [CSP] Q: "Does the desktop page need a CSP change?" → Selected: "No new directive; a test pins the /desktop policy equal to the existing one; amend only if the browser e2e shows a block (precedent Amendments 1-4; R13 ran under the app's CSP meta)" — a header string
- [Launchd] Q: "How does the CLI respawn the daemon from an SSH shell?" → Selected: "EnsureDaemon in the CLI only kickstarts the launchd job and never execs a daemon itself; with the agent missing it prints how to fix it instead of spawning (R06, R07-B)" — local logic
- [Launchd] Q: "How do tests avoid the user's launchd domain?" → Selected: "Keep the testing.Testing() refusal; launchctl calls go through the Runner seam" — test seam
- [Crash containment] Q: "How does in-daemon capture avoid taking terminal sessions down?" → Selected: "ObjC @try around every bridge call, Go recover in desktop goroutines, capture or encoder errors become a state and never an exit; KeepAlive is the backstop for a hard crash (17-SPIKE-MATRIX: recover, do not panic)" — local code
- [Permissions] Q: "Which process checks grants and runs the test capture?" → Selected: "The daemon (the grant holder; GUI and daemon share the bundle ID, R05): CGPreflightScreenCaptureAccess and AXIsProcessTrusted polled about once a second while onboarding is open; the GUI calls a daemon socket endpoint and opens the x-apple.systempreferences pane URLs" — internal API
- [Permissions] Q: "Who posts the lapse notification with the GUI closed?" → Selected: "The GUI when it runs, otherwise the daemon via UNUserNotificationCenter under the same bundle identity; one sender at a time" — internal routing
- [Power] Q: "How are the DPRM-04 assertions held?" → Selected: "IOPMAssertion PreventUserIdleDisplaySleep and PreventUserIdleSystemSleep from the first viewer join to the last leave; IOPMAssertionDeclareUserActivity on join to light a sleeping display (SCK needs the display on); restart the stream after the wake notification on -3815; no monotonic timers across sleep (R18-host)" — runtime policy
- [Lock] Q: "Where does the locked state come from?" → Selected: "CGSessionCopyCurrentDictionary key CGSSessionScreenIsLocked, polled; lock-screen frames keep streaming while the display is awake (R15, R17)" — a local read
- [State] Q: "What is the state model?" → Selected: "Daemon enum off, needs_permission, awaiting_approval, display_asleep, locked, idle, streaming(n), pushed to viewers on the signalling socket and to the GUI over the daemon socket" — internal enum; wording is under Claude's Discretion
- [Packages] Q: "How is the code laid out?" → Selected: "internal/screencap (cgo ObjC: ScreenCaptureKit, VideoToolbox, IOKit power, CGSession) and internal/desktop (pion peers, fan-out, state, signalling handlers wired into the share server through setters like SetShareTokenProvider)" — internal names
- [Capture] Q: "When does capture run?" → Selected: "Only while at least one viewer is connected, plus the onboarding test capture" — runtime policy
- [Capture] Q: "Which display?" → Selected: "The main display (CGMainDisplayID) only; DESK-05/06 add choice and switching in Phase 20" — swap later
- [Encode] Q: "What encoder settings?" → Selected: "VideoToolbox hardware H.264 Constrained Baseline, realtime, AllowFrameReordering false, Annex-B, SPS/PPS on every IDR, IDR on join and on coalesced PLI/FIR, fixed target bitrate and keyframe ceiling as constants (adaptive quality is DESK-07, Phase 23)" — encoder constants
- [WebRTC] Q: "How are peers set up?" → Selected: "One PeerConnection per viewer sharing one encoded track; server ICE-lite with one host candidate on the tailnet IP (lite and full both GO in R11, R12, R14); UDP4 only; mDNS off; iceServers empty; IP filter to this Mac's tailnet address; one UDP mux socket on the tailnet IP at a fixed port beside the 7444/tcp share server" — server-side; the port travels in SDP
- [Signalling] Q: "What signalling protocol?" → Selected: "Non-trickle JSON offer/answer plus state messages with a version field, over WSS at /desktop/ws on the share server; the same host serves the viewer page, so both ends ship together" — replaceable
- [DESK-02] Q: "How is tailnet-only asserted?" → Selected: "Tests: every desktop socket binds only the tailnet IP (refuse 0.0.0.0, ::, non-100.64/10, the R23 pattern); no packet to STUN/TURN ports (3478, 19302) or 5353; iceServers empty; real-Mac M-157 checks tailscale ping reads direct" — tests and manual rows
- [Viewer] Q: "How is the viewer built?" → Selected: "React DesktopView.tsx plus lib/desktopClient.ts in the existing frontend bundle, vanilla RTCPeerConnection, video letterboxed to fit the window at device-pixel sharpness, muted autoplay with a tap-to-start overlay fallback (iOS, R14); Phase 21 reuses it in the Wails tab" — frontend code
- [Reconnect] Q: "How do viewers reconnect?" → Selected: "Retry with capped backoff (about 1 s to 10 s) after ICE failure, socket close, page wake (visibilitychange/pageshow) or a server restart, showing Reconnecting; after an upgrade the new daemon's server answers the same link" — client logic
- [Build] Q: "What deployment target?" → Selected: "Desktop cgo built with -mmacosx-version-min=14.0 and LSMinimumSystemVersion raised from 10.13.0 to 14.0 (PROJECT Key Decision); the universal build keeps compiling until REL-01" — build flags
- [Testing] Q: "What test approach?" → Selected: "A synthetic frame source behind the capture interface (the 17-01 tracer pattern) for loopback integration tests of fan-out, join IDR, reconnect, share-server lifetime and handoff; Tart VM rows for SCK and TCC (R00 GO); real-Mac M-NN rows for the battery sleep test, direct path, FA-1 attribution (M-163) and Tailscale mode (M-164); TESTING.md updated per the standing rule" — test layout

### Claude's Discretion
User delegated: 'I don't need to answer a bunch of questions' (2026-11-25). Claude may adjust these during planning:
- Always-available is switched on in a Desktop group in Settings ▸ Sharing, with the switch, a live state line and a Copy Link button; there is no new toolbar button. Reason: the Sharing pane already holds share settings, and the toolbar Share button is per session.
- The link is delivered by Copy Link only (the user moves it by Universal Clipboard, AirDrop or Messages); no QR code. Reason: a QR library would be a new dependency for a convenience.
- The first enable reuses the existing Certificate Transparency acknowledgement (ct_disclosed) if no terminal share has shown it yet. Reason: the share server publishes the tailnet name the same way.
- With always-available off, an old link shows a static page saying desktop sharing is off on this Mac, not the generic Link No Longer Works page. Reason: it tells the user what to fix at the Mac.
- Onboarding copy explains that the Login Items entry appears under the signing developer's name and that its status text does not follow the process (R09-B). Reason: this keeps the user from disabling an entry that looks foreign.
- Phase 19 onboarding asks for Accessibility as well as Screen Recording, although control lands in Phase 20. Reason: DPRM-01 names both, and the office Mac then needs one in-person trip, not two.
- Onboarding is a sheet with one row per permission (status dot polled live, Open System Settings button, short how-to), then a Test Capture button that shows a small thumbnail from the daemon. There is no timeout, and the copy mentions the once-per-bundle "bypass the system private window picker" dialog on macOS 27.2 (R00, R10). Reason: matches the macOS-native grouped-form Settings style.
- State wording for host and viewers: Needs permission, Waiting for approval at the Mac, Display asleep, Locked, Streaming to N viewers; viewers also show Reconnecting. Reason: the requirement text already uses these phrases.
- Lapse notification: one native notification per lapse (Portty desktop needs approval at the Mac), none on recovery. Reason: avoids noise on an unattended Mac.
- Host state is shown in the Settings Desktop group and as one line in the tray menu, with no new tray icon state in Phase 19 (the shared-with-N indicator with Stop is DSHR-06, Phase 22). Reason: radical simplicity.
- USER DECISION after research (2026-11-25, 'Only while plugged in'): while always-available desktop is on and the Mac is on AC power, the daemon holds a system-sleep assertion so the Mac stays reachable (the display may still sleep); on battery it sleeps normally. Viewer-connected display-awake assertions (DPRM-04) are unchanged. Rationale: the office Mac's power policy idle-sleeps 1 min after display sleep, and nothing can wake a sleeping Mac over the tailnet.
- After research (user delegated): permission state is read by a short-lived child process (portty daemon perm-probe) and confirmed by effect, never by in-process preflight calls (they cache their first answer); 'awaiting approval' is classified only after a stream restart; product code never uses launchctl kickstart -k; desktop.json carries iat so the copied link survives restarts.
</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- internal/daemon/share.go: shareOn (health check, cert warm, startMu and writeGate discipline), startShareServer, wireShareServer, installShareServer, unshareLocked (stops the server when len(a.shares)==0, the line serverNeeded() replaces), remintSharesLocked.
- internal/daemon/handoff.go: two-phase handoff, HandoffVersion 1, share-server rebuild from the received records (around lines 1147-1298); the header shortcut note says launchd does not manage the takeover and to special-case that if the agent becomes the default.
- internal/daemon/process.go and process_unix.go: EnsureDaemon, startDetachedDaemon (Setsid), spawnTakeover, respawn checks. internal/daemon/service.go plus cmd_daemon.go: the opt-in kardianos `portty-daemon` LaunchAgent (install, uninstall, start, stop).
- internal/capability: Claims, Sign and Verify; porttoken.go (HKDF-separated key and Kind check, the pattern for the desktop token); keystore.go (capability.key, 0600, LoadOrGenerate). daemon api.go handleRegenerateSigningKey re-mints shares.
- internal/webserver: server.go setupRoutes (route model and gate order), whois.go (sameUser, requireSameUser, requireSameUserPage, refusal and retry pages), origin_mw.go, capability_mw.go, csp_mw.go (policy and amendment history), portproxy.go, BaseURL; tailnet.DefaultProbePort is 7444.
- internal/daemon/engine.go: settings.json (unknown fields dropped on save, SchemaVersion migration); share.go: the ct_disclosed acknowledgement.
- notification_darwin.go (UNUserNotificationCenter, needs a bundle identifier); tray.go and tray_common.go (menu items); app.go bindings (GetSessionShare, SetSessionShared, HasCTDisclosure, RegenerateSigningKey, health and tray pollers).
- frontend: SettingsSheet.tsx Sharing pane (Row, settings-group, footnote), SharePopover.tsx, Sidebar.tsx, lib/webMode.ts (URL parameter parsing), lib/hostReconnect.ts, public/manifest.json (scope ./), vite base ./.
- spike/desktop (reference only, never imported): media_darwin.m and bridge_darwin.go (SCK, VT, IOKit and CGSession bridges; forceKeyframe through a cached refresh), server.go (pion SettingEngine, single tailnet bind and refusals), viewer.html (no-store, no-referrer, null frame handling), bundle/build.sh (inside-out signing).

### Established Patterns
- Seams are package vars copied into the API (defaultShareHealth, defaultNewShareServer, defaultWarmCert) with SetXForTest; exec'd tools go through a Runner seam.
- Lock order is startMu, then a.mu; a.mu is never held across network or cert work; writeGate freezes during a handoff.
- Every share-server route checks Origin, then WhoIs same-user, then the capability; refusals are static pages with the X-Portty marker; token-bearing pages are no-store.
- Daemon and GUI are one Mach-O; subcommands dispatch in main.go and cmd_daemon.go; unknown leading flags exit 2 (Phase 16).
- cgo ObjC .m files at the repo root (tray.go, notification_darwin.go, appearance_darwin.go) show the bridging style.

### Integration Points
- Share server lifetime: share.go unshareLocked and the handoff.go server rebuild gain serverNeeded(); webserver.setupRoutes gains the /desktop/ page, /desktop/ws and the static assets routes.
- Daemon socket API (api.go): desktop status, enabling and disabling always-available, permission preflight, test capture.
- Daemon start: process.go ensureDaemon and startDetachedDaemon, and app.go startDaemon and respawnDaemonIfDown, switch to launchctl kickstart; the re-home step follows the handoff commit.
- GUI: a Desktop group and onboarding sheet in the SettingsSheet Sharing pane; a tray menu state line; notifications.
- Frontend bundle: DesktopView.tsx and lib/desktopClient.ts, entered from the /desktop/ route.
- Build: Info.plist LSMinimumSystemVersion, cgo flags, and the ScreenCaptureKit, VideoToolbox, CoreMedia and IOKit frameworks in the root module.

</code_context>

<specifics>
## Specific Ideas

Settled by prior decisions (not re-asked):
- Capture host is in-daemon: capture, encode and WebRTC serve run inside the existing daemon, one Mach-O, one TCC identity (the app bundle ID), no helper bundle, a legacy LaunchAgent and not SMAppService (17-SPIKE-MATRIX FEAS-02 DECIDED, rule row 7; 17-12-SUMMARY).
- pion/webrtc v4 is a new root-module dependency, pinned at v4.2.29 with pion/interceptor v0.1.51, not the v5 RCs (PROJECT Constraints; PROJECT Key Decision 2026-11-24; research SUMMARY Recommended Stack; spike go.mod).
- Tailnet only: no STUN, TURN or cloud, empty iceServers, plus a direct-path success criterion (PROJECT Key Decision; DESK-02 as amended by R01; ROADMAP Phase 19 SC4).
- H.264 Constrained Baseline under the 42e01f SDP entry, no SPS patch needed (research C5; R11, R12, R13, R14). HEVC is deferred (17-CONTEXT deferred; REQUIREMENTS Future).
- Default cap of 2560 px on the long edge at 30 fps (research C4; R20 GO on the MacBook Air: 29.15 fps, 12.85 ms mean latency, 27 percent own CPU).
- The cursor is drawn into the frames (research C6; a data-channel cursor is a Future requirement).
- The daemon is never started from an SSH shell; the CLI respawns it through launchd with launchctl kickstart (R06 NO-GO, R07-B GO; DSHR-01 and DPRM-01 as amended).
- Grants are confirmed by polling the non-prompting preflight, the System Settings path is offered, and there is no short timeout (host session 1; DPRM-01 as amended).
- Display-awake and system-sleep assertions while a viewer is connected, a restart on SCError -3815 after wake, lid-closed sleep excepted, and waking a sleeping Mac not promised (R18 NO-GO; DPRM-04 as amended).
- Lapse detection must not rest on zero frames (R19; DPRM-03 as amended).
- Always-available opens only to the host's own Tailscale account (DSHR-01; MOB-05 same-user gate); other tailnet people come only through click-to-share in Phase 22 (PROJECT Key Decisions 2026-11-24).
- Desktop sessions are not handed off on upgrade; viewers reconnect; HandoffVersion stays 1 (ROADMAP Phase 19 Notes; research Architecture item 5).
- Minimum macOS 14 with an Apple Silicon target (PROJECT Key Decision 2026-11-24); the arm64-only asset rename waits for REL-01 in Phase 23 (STATE blocker on the v1.2 updater asset names).
- One shared encoder with a forced IDR on join (research Architecture item 2; R22 GO, first frame about 0.8 to 1.2 s on a still screen).
- iOS starts video only from a user tap (R14), so the Phase 19 viewer page has a tap-to-start fallback; SC3 requires a saved iPhone Home Screen link to keep working.

Other specifics:
- The office Mac is corporate and unattended, and nobody can get in to fix it, so reliability choices favour self-recovery (KeepAlive, re-home after upgrade, stream restart after wake).
- R18-host: on battery the MacBook Air idle-slept 5.5 s after the display slept; SC2 requires the DPRM-04 fix to be tested on battery.
- A connecting viewer lights the office display (SCK needs it on); curtain mode is out of scope.
- Apple's persistent-content-capture entitlement request is filed (T558S9Q5B3, commit 396e6379). Until Apple grants it, someone approves Screen Recording at the office Mac about once a month, which DPRM-03 surfaces. Wiring the entitlement is a Future requirement.
- Open office-Mac follow-ups this phase rests on: M-157 (direct path), M-159 (office display sleep policy), M-160 (the real 30-day prompt), M-161 (encoder at native size), M-163 (FA-1 attribution and preflight), M-164 (Tailscale mode).
- Research rules carry over: subagents never contact the host's tailscaled or 100.x addresses; nothing mounts, copies or modifies ~/Downloads or /Applications on the host; capture checks run in the VMs, or on the user's Macs with their go-ahead.

- Office Mac facts (user, 2026-11-25): the user is CTO and configures the office Mac themselves; it mirrors the personal Mac. Read from the personal Mac: Tailscale is the standalone app (io.tailscale.ipn.macsys, Developer ID, not App Store; CLI at /usr/local/bin/tailscale), answering M-164. Power policy (pmset -g custom): system sleep 1 min and display sleep 10 min on both battery and AC, Low Power Mode on battery, Power Nap on. So the Mac idle-sleeps right after the display sleeps, matching R18-host; DPRM-04's system-sleep assertion is mandatory, not optional (M-159 context).
- Seams later phases reuse (from Phase 21 research, 2026-11-25): build the /desktop/ viewer with a pluggable connection, a hook before redial and a wake event; fix web-mode detection so a /desktop/ page is not read as the Wails shell (today any path outside /app/ is); prove video plays under the strict share CSP (no media-src today); the web-app manifest has no start_url, so saved Home Screen links keep their token; desktop proxy errors distinguish rejected token, desktop off and unreachable.
</specifics>

<deferred>
## Deferred Ideas

- Display or window choice and mid-session switching (DESK-05, DESK-06), and all input and clipboard (Phase 20).
- The Portty app Desktop tab from the sidebar, peer discovery of the desktop, and iPhone trackpad and touch controls (Phase 21).
- Click-to-share links for other tailnet people, the admit prompt, revoke, the shared-with-N indicator with Stop, and the audit log (Phase 22).
- Audio, adaptive quality, remote unlock, and the arm64-only release with its asset rename (Phase 23).
- Persistent-content-capture entitlement wiring, a data-channel cursor, HEVC and H.264 High (REQUIREMENTS Future).
- A QR code for the link and new `portty desktop` CLI verbs (not needed for the phase goal).

</deferred>
