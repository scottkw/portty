# Phase 19: Unattended Desktop Viewing - Research

**Researched:** 2026-11-25
**Domain:** macOS in-daemon screen capture (ScreenCaptureKit, VideoToolbox) served to browsers over WebRTC (pion) on the tailnet; launchd-managed daemon; TCC permission detection; share-server lifetime and handoff
**Confidence:** MEDIUM-HIGH (permission-detection and power findings rest on in-repo spike evidence read this session; launchd/notification/CSP items that no spike measured are tagged `[ASSUMED]` and listed as early VM rows)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

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

Settled by prior decisions (not re-asked) — from CONTEXT `<specifics>`:
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

### Deferred Ideas (OUT OF SCOPE)
- Display or window choice and mid-session switching (DESK-05, DESK-06), and all input and clipboard (Phase 20).
- The Portty app Desktop tab from the sidebar, peer discovery of the desktop, and iPhone trackpad and touch controls (Phase 21).
- Click-to-share links for other tailnet people, the admit prompt, revoke, the shared-with-N indicator with Stop, and the audit log (Phase 22).
- Audio, adaptive quality, remote unlock, and the arm64-only release with its asset rename (Phase 23).
- Persistent-content-capture entitlement wiring, a data-channel cursor, HEVC and H.264 High (REQUIREMENTS Future).
- A QR code for the link and new `portty desktop` CLI verbs (not needed for the phase goal).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DPRM-01 | Guided Screen Recording + Accessibility grant, test capture confirms; grants confirmed by polling the permission state, no short timeout | Section "Permission detection": in-process preflights are cached, so poll through a short-lived `portty daemon perm-probe` child; confirm by effect with an in-daemon test capture and a nudge-and-return |
| DPRM-02 | Host and viewers see desktop state (permission missing, waiting for approval, display asleep, locked, streaming N) | "State model" table: each state mapped to its source (SCK -3801, -3815, CGDisplayIsAsleep, CGSSessionScreenIsLocked, stall watchdog) |
| DPRM-03 | Approval lapse shows "waiting for approval at the Mac" + native notification; detection not on zero frames | R19 evidence: the prompt does not stop frames and a replayd restart freezes a stream silently; signals (a)(b)(c) refined, VM row for a prompt-window detector, VM row for daemon-posted notification |
| DPRM-04 | Display-awake and system-sleep assertions while viewers connected; restart after wake; tested on battery | IOPMLib.h semantics, R18-host caffeinate caveat, `kIOPMUserActiveRemote`, reuse `internal/wakewatch` |
| DESK-01 | Sharp on Retina, fitted to window | SCK `pointPixelScale` sizing with 2560 long-edge cap (spike media_darwin.m), viewer `object-fit: contain` |
| DESK-02 | Media stays on tailnet; no STUN/TURN/cloud; no non-tailnet bind; direct path | pion SettingEngine pattern (spike server.go:312-317), lsof-based bind tests, M-157 |
| DESK-03 | Viewer reconnects by itself (network drop, sleep/wake, upgrade) | Tie PeerConnection lifetime to the signalling socket; backoff client; handoff 1012 close; UDP port rebind retry |
| DESK-04 | Several viewers; new viewer gets a picture at once | One shared track, IDR on join from cached frame (R22), rate-limited PLI coalescing |
| DSHR-01 | Always-available for own devices, GUI closed, daemon under launchd only | LaunchAgent plist design, kickstart semantics (no `-k`), serverNeeded lifetime, exit-status audit |
| DSHR-02 | Links survive daemon restarts and upgrades | desktop.json (add `iat`), HKDF token, handoff passes the listener when always-available, takeover rebuild path |
| DSHR-08 | Desktop link never opens a terminal and vice versa | HKDF `desktop-token/v1` kind separation mirrors `porttoken.go`; cross-token test matrix |
| DVWR-02 | Desktop Chrome and Safari open the desktop from a link | `/desktop/` route serving the SPA bundle, `detectMode` extension, viewer-side Chrome Local Network prompt, tap-to-start |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

Extracted from `~/dev/CLAUDE.md`, `CLAUDE.md`, `.claude/CLAUDE.md` and the user's auto-memory. Treat as locked.

- Go: `go fmt`, `golangci-lint`, context-aware functions (`ctx context.Context`). JS/TS: ESLint + Prettier, TypeScript types, `pnpm` preferred.
- Testing: 80%+ coverage in critical components. **TESTING.md standing rule (portty/CLAUDE.md):** every phase that adds tests must (1) add new test files to the right suite group in Section 2 with no build tags or file moves (the existing `e2e` tag is the only exception already in use), (2) add an M-NN item to Section 5 for anything not automatable (live GUI, remote peer, hardware), (3) add a Section 4 traceability row per new test file covering a v2.0 requirement (repo-relative path ending `.go`, `.ts`, `.tsx` or `.sh`, run `bash tests/check-traceability-paths.sh` before committing), (4) re-apply branch protection if `build.yml` matrix entries change.
- Core principles: Chesterton's Fence (know why code exists before removing it, especially daemon/relay/capability code shared by kept and removed features), no premature abstraction (3 real examples), "Silent Fallbacks: `or {}` converts hard failures into silent corruption: let it crash" (so capture errors become an explicit *state with a reason*, never a swallowed nil), RULE 0 (stop on unknown catastrophic failures), never `kill node.exe`.
- Project (.claude/CLAUDE.md): MIT licence (pion is MIT, see audit), macOS only, keep the Go/Wails/React/tsnet stack, tailnet-only network with no new relay or cloud dependency, keep token-gated tailnet-only share links and the CSP intact, Chesterton deletion discipline, GSD workflow enforcement.
- Memory constraints: reliability first (nobody can get in to fix the remote Mac), macOS-native look for GUI, subagents never touch the host's tailscaled or 100.x, no host side effects (no Spike/capture runs, nothing mounted/copied/modified in `~/Downloads` or `/Applications`), Tart VMs are ephemeral on the tailnet (never `tailscale down` or stop them for hours; use pf to drop traffic).
- LSP preferred for code navigation where available; GSD entry points required before file edits (this research writes only RESEARCH.md).

## Summary

Phase 19 is mostly integration of pieces the Phase 17 spike already proved (SCK capture, VideoToolbox H.264, pion ICE-lite on the tailnet IP, tailnet bind refusals, viewer page behaviour in Safari, Chrome, Wails and iPhone) into the existing daemon, plus three architectural changes to long-lived code: the share server's lifetime (`serverNeeded()`), the daemon's start model (launchd + re-home), and a new desktop token and route family. The risky parts are not the media path; they are (1) detecting permission and approval state correctly, (2) keeping an unattended Mac reachable and self-recovering, and (3) not regressing terminal sessions when the share server, handoff and start path all change at once.

The orchestrator's extra input is confirmed and extended by the spike's own evidence. In the daemon shape on macOS 27.2, **`CGPreflightScreenCaptureAccess` is cached per process exactly like `AXIsProcessTrusted` and `CGPreflightPostEventAccess`** (pid 3092 read `sr_preflight` false for minutes after the grant while capture delivered Complete frames; pid 3402 read it true after the toggle was turned off while capture failed with -3801). `SCShareableContent`/`SCStream` follow the live setting in the same process (pid 3402's display list errored with -3801 within one 5 s record of the toggle going off and recovered when it went on). A fresh process reads the live value (host session 1 polled with a new process every 8 s). So the CONTEXT decision "the daemon polls CGPreflightScreenCaptureAccess and AXIsProcessTrusted about once a second" must change: poll through a short-lived child of the same Mach-O, and confirm by effect (in-daemon test capture, nudge-and-return).

Three further evidence-backed findings shape the plan: the monthly approval prompt does **not** interrupt frames in the VM (R19-b), and a `replayd` restart silently freezes a running stream (R19-c1), so a frame-stall watchdog with restart is the universal recovery and diagnostic path; a system-sleep assertion from another process did not stop idle sleep 5.5 s after an explicit display sleep on battery (R18-host), so the display-awake assertion is the primary guard and wake recovery must be designed for the Mac sleeping anyway; and the spike's viewer pages carried no CSP header, so "no CSP change needed" is unmeasured in Chrome and Safari (only R13 ran under a CSP, as a meta tag in WKWebView).

**Primary recommendation:** Build on the spike's proven shapes, but (a) detect permissions by effect plus a fresh `portty daemon perm-probe` child, never by in-process preflight polling; (b) make a frame-stall watchdog (nudge, then SCStream restart, then classify by the restart's error) the single source of `needs_permission` and `awaiting_approval` while viewers are connected; (c) schedule the unmeasured launchd, notification, CSP and replayd-recovery behaviours as early VM rows before dependent code is written; and (d) treat every daemon exit path as a launchd restart decision (exit 0 means "stay down").

### Findings that change or refine CONTEXT decisions (call-outs for the planner)

| # | CONTEXT decision | Finding | Effect |
|---|------------------|---------|--------|
| C1 | [Permissions] "The daemon polls CGPreflightScreenCaptureAccess and AXIsProcessTrusted about once a second while onboarding is open" | In-process preflights are cached for the process lifetime (SR, AX and PostEvent all measured, both directions). **REVERSED:** onboarding polls a short-lived child `portty daemon perm-probe` (fresh process, non-prompting preflights only); "Test Capture" confirms by effect inside the daemon (SCK follows the live setting); Accessibility is additionally confirmed by a nudge-and-return effect check. Poll cadence: 2 s while the sheet is open (8 s was measured, 1 s is not). | Changes DPRM-01 implementation and M-163's wording ("grant is confirmed by preflight polling" must say fresh-process preflight or effect) |
| C2 | [Permissions] lapse signal (c): nudge plus frame check means "waiting for approval" | R19-b: the aged-approval prompt appeared while frames kept flowing, and R19-c1: a replayd restart froze the stream with no error and no frames. So "no new frame after the nudge" can mean a frozen stream, not an unanswered prompt. **REFINED:** classify only after a stream restart: restart error -3801 = `needs_permission`; restart OK but still no frame after a nudge = `awaiting_approval`; restart recovers = recovered (log a stall). Add as an optional extra signal (d): a VM-measured "approval prompt window is on screen" detector (window owner and layer need no Screen Recording grant `[ASSUMED]`) | Keeps (a)(b)(c); adds a restart step and a candidate positive signal; M-160 still needed |
| C3 | [Token/file] `desktop.json` holds alwaysAvailable, grantID, schemaVersion | Token claims include `iat`. Without a persisted `iat`, "Copy Link" after a daemon restart mints a different-looking (still valid) URL. **REFINED:** add `iat` to `desktop.json` schema version 1 now (free before first ship, costly after) so the link text is stable | One extra field in a "costly to change" file; do it in the first commit of the schema |
| C4 | [Launchd] "Every start becomes `launchctl kickstart gui/<uid>/<label>`; one start path for everyone" | (1) The spike only exercised `kickstart -k`; `-k` kills the running job, which would kill every terminal session (the daemon holds the PTY masters). Product code must **never** pass `-k`. Plain `kickstart` on a loaded idle job is unmeasured `[ASSUMED]`. (2) With `KeepAlive {SuccessfulExit false}` a daemon that returns normally after a fatal start error exits 0 and is *not* restarted; `runDaemonCore` has several such `return` paths. (3) A failed re-home must exit 0 or launchd retries the takeover every 10 s forever. (4) The legacy `portty-daemon` bootout would SIGTERM a legacy-managed daemon that still holds sessions. (5) If `bootstrap` is refused (corporate Mac, Login Items disabled), a GUI fallback to the existing detached spawn keeps terminals working. **REFINED:** "one start path" holds except bootstrap-refused, where the existing detached spawn remains | Exit-status audit task; bootout ordering rule; fallback path retained (Chesterton: `startDetachedDaemon` also serves takeover) |
| C5 | [Power] "PreventUserIdleDisplaySleep and PreventUserIdleSystemSleep from first join to last leave" | R18-host: a PreventUserIdleSystemSleep from another process did not prevent Idle Sleep 5.5 s after an explicit display sleep on battery. IOPMLib.h: the display assertion prevents idle sleep while held but "does not light up the display" if already off, and the header says to call `IOPMAssertionDeclareUserActivity` with `kIOPMUserActiveRemote` for remote clients that need the framebuffer. **REFINED:** display assertion is the primary guard; hold both anyway; join calls DeclareUserActivity with `kIOPMUserActiveRemote`; after any wake (wall-clock gap, reuse `internal/wakewatch`) restart the stream; do not promise the Mac stays awake after an explicit display sleep, lid close or Apple-menu sleep | Wording of DPRM-04 acceptance in the M-159 battery row |
| C6 | [Permissions] "The GUI when it runs, otherwise the daemon via UNUserNotificationCenter" | Daemon-posted UNUserNotificationCenter from a launchd-run Mach-O with no NSApplication is unmeasured. **REFINED:** make it an early VM row; the daemon is the single sender if it passes (one path, GUI-closed case works), otherwise the GUI sends and the GUI-closed case degrades to the state line only (and the DPRM-03 acceptance needs a user decision) | Early VM row gates DPRM-03's notification half |
| C7 | [CSP] "No new directive; amend only if the browser e2e shows a block" | The spike viewer served no CSP header (viewer.html handler sets only Cache-Control and Referrer-Policy), so R11, R12 and R14 ran without one; only R13 ran under a CSP (meta tag, WKWebView). The decision is unmeasured for Chrome and Safari. **REFINED:** keep the decision, but run the header-CSP check (chromedp e2e with a `securitypolicyviolation` listener, plus a Safari/iPhone M row) in the first wave so a needed amendment (a security-reviewed change) does not land late | Early e2e + M row |
| C8 | [Viewer] "DesktopView.tsx in the existing bundle" | `detectMode()` returns `'web'` only for `/app` and `'desktop'` (the Wails shell) for every other path, so a bundle served at `/desktop/` would boot the Wails-coupled `App`. **REFINED:** add a distinct mode for `/desktop/` and branch in `main.tsx` before `App` renders; also skip the `is-desktop` class | Small but breaking if missed |
| C9 | [Crash containment] "ObjC @try around every bridge call … KeepAlive is the backstop" | The spike's own SIGTRAP (AppKit `_crashOnException` from a nil in an `@{}` literal inside a Core Animation commit) shows `@try` in your own frames does not cover exceptions raised on framework threads. A hard crash kills every terminal session (the daemon owns the PTY masters), so KeepAlive restarts a daemon with no sessions. **REFINED:** nil-guard every ObjC collection literal, keep callbacks free of throwing paths, and add a crash-loop breaker (capture disables itself after repeated abnormal exits and says so in the state line) | Extra small task; protects terminals |
| C10 | SC1 "open any time" | While the office Mac is in system sleep the daemon is suspended and the link is simply unreachable; nothing in the tailnet wakes it (out of scope per REQUIREMENTS). Availability therefore depends on the Mac's sleep settings (M-159). **REFINED:** viewer page shows "Can't reach this Mac" with a "may be asleep" hint after the backoff passes about 30 s; the Desktop group's footnote tells the user which Energy settings keep it awake. Whether to hold a system-sleep assertion permanently while always-available is on is a **product decision not in CONTEXT** (Open Question 1) | Copy plus one open decision |

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Screen capture, H.264 encode | Daemon (cgo, in-process) | OS (SCK, VideoToolbox) | FEAS-02: in-daemon, one TCC identity |
| Permission state (preflight) | Short-lived child of the daemon binary | Daemon (effect checks) | Preflights are cached per process, so a fresh process is the only non-prompting live read |
| Desktop state machine, viewer count | Daemon (`internal/desktop`) | GUI (display), viewers (display) | Daemon is the only always-running process |
| Share server lifetime, `/desktop/` routes, gates | Daemon HTTP tier (`internal/webserver`) | OS (tailnet bind) | Existing share server owns TLS, WhoIs and token gates |
| Desktop token and grant | Daemon (`internal/capability`, `desktop.json`) | — | Signing key already lives in the daemon |
| WebRTC media, signalling | Daemon (pion, WSS `/desktop/ws`) | Browser (RTCPeerConnection) | Server ICE-lite on the tailnet IP |
| Power assertions, wake, lock, display sleep | Daemon (cgo IOKit/CoreGraphics) | OS | Held while viewers are present, no GUI needed |
| LaunchAgent plist, `kickstart`, legacy cleanup | GUI first start + CLI/GUI ensure paths | Daemon (`/health` managed flag) | Plist install needs the app bundle path; daemon must know it is launchd-managed |
| Onboarding sheet, Settings Desktop group, tray line | GUI (React in Wails) | Daemon API | Matches Settings ▸ Sharing pane |
| Lapse notification | Daemon (if the VM row passes) | GUI | Must work with the GUI closed |
| Viewer page, reconnect, tap-to-start | Browser (React `DesktopView`) | Share server (static bundle) | Frontend bundle shared with `/app/` |
| Tailnet-only enforcement | Daemon (bind refusals, pion filters) | Tests, M-157 | Defence in depth |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| github.com/pion/webrtc/v4 | v4.2.29 (published 2026-11-22) | Server-side WebRTC: ICE-lite, H.264 track, data channel later | Locked by user decision; spike-proven in Safari, Chrome, Wails, iPhone [VERIFIED: proxy.golang.org/github.com/pion/webrtc/v4/@v/v4.2.29.info and @latest] |
| github.com/pion/interceptor | v0.1.51 (2026-10-23) | NACK/RTCP interceptors (PLI handling) | Locked pin, spike go.mod [VERIFIED: proxy.golang.org; spike/desktop/go.mod] |
| github.com/pion/ice/v4 | v4.4.9 (2026-11-09) | `UDPMuxDefault` for one fixed UDP port | Used by the spike server [VERIFIED: spike/desktop/go.mod; proxy.golang.org] |
| github.com/pion/rtp, rtcp | v1.10.6, v1.2.20 | Test-side depacketizing (NAL type checks), PLI | Spike go.mod pins [VERIFIED: spike/desktop/go.mod] |
| github.com/coder/websocket | v1.8.15 (existing) | `/desktop/ws` signalling | Already the share server's WebSocket library; 32 KiB read limit (HARD-02) is enough for SDP [VERIFIED: go.mod] |
| System frameworks via cgo | macOS SDK 26.5 present locally | ScreenCaptureKit, VideoToolbox, CoreMedia, CoreVideo, CoreGraphics, ApplicationServices, IOKit, UserNotifications | No third-party capture code; spike bridge files are the reference [VERIFIED: spike/desktop/media_darwin.m, sys_darwin.m] |
| Frontend | none new | Vanilla `RTCPeerConnection`, existing React 19.2, Vite 8 | Decision: no new npm packages [VERIFIED: frontend/package.json] |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| internal/wakewatch (existing) | — | Wall-clock gap wake detector, pure Go | Stream restart after Mac wake; works with no AppKit run loop [VERIFIED: internal/wakewatch/wakewatch.go header] |
| internal/capability (existing) | — | HKDF-separated token pattern | New `desktoptoken.go` mirroring `porttoken.go` [VERIFIED: internal/capability/porttoken.go] |
| kardianos/service v1.2.4 (existing) | — | Only for uninstalling the legacy `portty-daemon` agent | Its `KeepAlive` option is a bool and cannot express `{SuccessfulExit false}`, so the new plist is written by Portty [VERIFIED: service_darwin.go:189-197, 308-309] |
| chromedp (existing, `//go:build e2e`) | v0.15.1 | Header-CSP browser check | Needs installed Google Chrome for H.264; Chromium builds usually lack it `[ASSUMED]` |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Fresh-process preflight child | In-daemon `SCShareableContent` polling | SCK probes are live, but macOS 27 beta reports show an alert on every call and the first call while ungranted raises the system prompt [CITED: developer.apple.com/forums/thread/841530, beta report, unconfirmed by Apple]; use SCK only for the explicit Test Capture and capture start |
| Fresh-process preflight child | `AXIsProcessTrusted` in-process | Cached; stale in both directions (R08-A/B) |
| Own plist | kardianos `Install()` | kardianos writes `KeepAlive` as a bool (service_darwin.go:308-309), not the `SuccessfulExit` dict CONTEXT requires |
| SMAppService | Legacy LaunchAgent | Already decided against (FEAS-02) |

**Installation:**
```bash
# Root module, pinned (versions from spike/desktop/go.mod, verified on the Go proxy)
go get github.com/pion/webrtc/v4@v4.2.29 github.com/pion/interceptor@v0.1.51 github.com/pion/ice/v4@v4.4.9
# Frontend: nothing to install
```

**Version verification:** `curl -s https://proxy.golang.org/github.com/pion/webrtc/v4/@latest` returned `v4.2.29` (2026-11-22); `@v/list` tops out at v4.2.29 [VERIFIED: proxy.golang.org, this session]. Repo `github.com/pion/webrtc`: created 2018-05-18, 16.9k stars, MIT, not archived [VERIFIED: api.github.com/repos/pion/webrtc].

## Package Legitimacy Audit

The `package-legitimacy` seam covers `npm|pypi|crates` only (`gsd-tools package-legitimacy check --ecosystem go …` answers `Usage: … --ecosystem <npm|pypi|crates>`) [VERIFIED: tool run this session], so the Go modules were checked on the Go module proxy and GitHub instead. All are user-locked decisions (19-CONTEXT, PROJECT Key Decision 2026-11-24) and the spike's `go.sum` already pins hashes (32 pion lines).

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| github.com/pion/webrtc/v4 | Go proxy | 8 yrs (repo 2018) | n/a (16.9k stars) | github.com/pion/webrtc (MIT) | OK (manual) | Approved, locked decision |
| github.com/pion/interceptor | Go proxy | 71 versions | n/a | github.com/pion/interceptor | OK (manual) | Approved, locked decision |
| github.com/pion/ice/v4 | Go proxy | 34 v4 versions | n/a | github.com/pion/ice | OK (manual) | Approved, transitive of webrtc |
| github.com/pion/rtp, rtcp | Go proxy | — | n/a | github.com/pion/* | OK (manual) | Approved, spike-pinned |
| npm packages | — | — | — | — | — | None added |

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none. (An `npm` lookup of the bare name `pion` returned SUS, which is an unrelated npm package; it is not used. Do not `npm install` anything for this phase.)

## Architecture Patterns

### System Architecture Diagram

```
 Viewer browser (Chrome/Safari/iPhone Home Screen)                Office Mac (launchd gui/<uid>/app.portty.Portty.daemon)
 ───────────────────────────────────────────────                  ───────────────────────────────────────────────────────
 GET https://mac.tailnet.ts.net:7444/desktop/?cap=T ─── TCP/TLS ─▶ share server (existing listener, serverNeeded())
                                                                     Origin? ▶ cspHeaders ▶ requireSameUserPage(WhoIs) ▶ desktop-token page check
                                                                     ▶ SPA bundle (index.html + /desktop/assets/*, same-user gate only)
 main.tsx: mode=desktopview ▶ DesktopView
 WSS /desktop/ws?cap=T ───────────────────────────────────────────▶ requireAllowedOrigin ▶ requireSameUser ▶ desktop-token check
   hello{v} ◀──── state{streaming n | locked | display_asleep | needs_permission | awaiting_approval | idle}
   offer(SDP, non-trickle) ───────────────────────────────────────▶ internal/desktop: new PeerConnection (ICE-lite, iceServers [])
   answer(SDP, 1 host candidate = tailnet IP:UDP port) ◀───────────         │ one shared TrackLocalStaticSample
 UDP STUN/DTLS/SRTP ─────── tailnet (direct or DERP, outside our control) ─▶ UDP mux socket (tailnet IP only)
                                                                              ▲ H.264 access units (Annex-B, IDR on join/PLI, rate limited)
                                                                  ┌───────────┴───────────┐
                                                                  │ encoder (VideoToolbox) │◀── frames ◀── SCStream (main display, 30 fps, ≤2560 px)
                                                                  └───────────────────────┘                  ▲ started on first viewer, stopped after last
   viewer count n ──▶ power: IOPM PreventUserIdleDisplaySleep + PreventUserIdleSystemSleep, DeclareUserActivity(Remote) on join
                      watchdog: no frame for N s ─▶ nudge(±1 px, return) ─▶ restart SCStream ─▶ classify (-3801 / -3815 / still no frame)
                      wakewatch / CGDisplayIsAsleep poll / CGSession lock poll ─▶ state
 Daemon socket API ◀── GUI: onboarding sheet polls /desktop/permissions (daemon runs `portty daemon perm-probe` child, 2 s), Test Capture,
                       enable/disable always-available, status; tray line; lapse notification
 Config dir: desktop.json (0600) {alwaysAvailable, grantID, iat, schemaVersion}   capability.key ─HKDF(portty/desktop-token/v1)─▶ desktop key

 Upgrade: GUI (new) ▶ EnsureDaemon ▶ takeover handoff (v1, GUI-spawned, listener fd passed when serverNeeded) ▶ old daemon exits
          ▶ GUI `launchctl kickstart gui/<uid>/<label>` (no -k) ▶ `portty daemon` (launchd) sees same-version unmanaged daemon ▶ re-home handoff
          ▶ desktop peers are NOT handed off: UDP port re-bound (retry) by the new owner, viewers reconnect via the same link
```

### Recommended Project Structure
```
internal/screencap/           # cgo, //go:build darwin (+ stub for other OS so `go vet ./...` stays green) [ASSUMED]
├── screencap.go              # interfaces: FrameSource, Encoder, PowerController, PermissionProber, Notifier; error classification
├── capture_darwin.m/.go      # SCStream (pointPixelScale sizing, 2560 cap), delegate -> typed errors (-3801, -3815)
├── encode_darwin.m/.go       # VideoToolbox H.264 CB realtime, forced IDR via cached frame
├── power_darwin.m/.go        # IOPM assertions, DeclareUserActivity, CGDisplayIsAsleep
├── session_darwin.m/.go      # CGSessionCopyCurrentDictionary lock key
├── nudge_darwin.m/.go        # CGEvent nudge-and-return (Accessibility effect)
├── preflight_darwin.m/.go    # CGPreflightScreenCaptureAccess / AXIsProcessTrusted / CGPreflightPostEventAccess, used ONLY by the probe child
└── synthetic.go              # pure-Go frame source + fixture H.264 NALs for tests
internal/desktop/             # pure Go, testable with synthetic sources
├── manager.go                # state machine, viewer registry, capture start/stop, watchdog
├── peers.go                  # pion API/SettingEngine, PC per viewer, shared track, PLI coalescing
├── signalling.go             # /desktop/ws handler (versioned JSON), state push
├── config.go                 # desktop.json load/save (atomic, 0600)
├── permissions.go            # runs `portty daemon perm-probe`, caches answer ≤ 1 s
└── launchd.go (or internal/daemon/launchd.go)  # plist render, bootstrap/kickstart via Runner seam, legacy cleanup
internal/capability/desktoptoken.go   # DesktopKey, SignDesktop, VerifyDesktop (mirror porttoken.go)
internal/webserver/desktop_routes.go  # /desktop/ page, /desktop/ws, /desktop/assets, off-page, setters
internal/daemon/desktop_api.go        # daemon socket endpoints; serverNeeded(); health poller
frontend/src/components/DesktopView.tsx, frontend/src/lib/desktopClient.ts, + detectMode extension
```

### Pattern 1: Permission detection by effect plus a fresh child (replaces C1)
**What:** Three tiers of truth, each used where it is valid.
1. *Live by construction (use in-daemon):* SCK. Start the stream (or `SCShareableContent`) and read the error. -3801 means Screen Recording is off right now. It recovers without a daemon restart [VERIFIED: evidence/r04-daemon.jsonl, pid 3402: records at 03:21:15 and 03:21:20 show `"code":-3801`, 03:21:40 shows no error again; pid 3092 captured Complete frames from 03:18:16 with `sr_preflight` still false].
2. *Fresh-process preflight (use for polling and idle checks):* `portty daemon perm-probe` prints one JSON object `{"sr":bool,"ax":bool,"pe":bool}` and exits. A new process reads the live value (pid 3402, started after the grant, read true from its first record; host session 1 polled a new process every 8 s and read true after the user added the grant by hand, evidence/host-run-notes.txt lines 47 and 66). Never call the request/prompt variants in the probe.
3. *Effect check for Accessibility:* nudge-and-return and compare `CGEventGetLocation` (R08-B: nudge followed the live toggle, cached `ax_trusted` did not).

**When to use:** Onboarding sheet open: spawn the probe at most every 2 s (daemon caches the answer for 1 s so several GUI polls share one spawn). Idle (no viewer): probe on GUI open and every few minutes `[ASSUMED]` so `needs_permission` shows without capture running. Streaming: rely on the stall watchdog, not the probe.

**Evidence the polling statement matters:** pid 3092 `sr_preflight` false while capturing and pid 3402 `sr_preflight` true while `capture.error.code` was -3801 [VERIFIED: evidence/r04-daemon.jsonl; SPIKE-MATRIX R08-A/R08-B text "AXIsProcessTrusted and CGPreflightPostEventAccess keep the first answer for the life of a process"; the Screen Recording case is the same pattern in r04-daemon.jsonl].

**Caveat:** a fresh child's attribution to the bundle ID is inferred from R05 (GUI-spawned exec of the same Mach-O resolved to `app.portty.spike` in tccd, evidence/r05-r07-attribution.txt) and the bare-Mach-O-to-bundle resolution (R09-B); a child exec'd by the launchd daemon is not directly measured `[ASSUMED]` (VM row V6). The in-process preflight must stay out of every long-running path; add a lint-style test that greps for the three call names outside `preflight_darwin.*`.

```go
// internal/desktop/permissions.go  (pattern; values marked [ASSUMED] are design choices)
type Probe struct{ SR, AX, PE bool }

func (p *Prober) Read(ctx context.Context) (Probe, error) {
	if c, ok := p.cached(time.Second); ok { return c, nil } // GUI polls share one spawn
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	exe, err := os.Executable()
	if err != nil { return Probe{}, err }
	out, err := p.run(ctx, exe, "daemon", "perm-probe") // Runner seam; never the in-process calls
	if err != nil { return Probe{}, fmt.Errorf("perm-probe: %w", err) } // explicit error, not a default-false
	var pr Probe
	if err := json.Unmarshal(out, &pr); err != nil { return Probe{}, err }
	p.store(pr)
	return pr, nil
}
```
`cmdDaemon` in `cmd_daemon.go` is the right dispatch point: `portty daemon <sub>` never runs EnsureDaemon and never starts the GUI (`main.go` `runCLI` handles `daemon` before EnsureDaemon) [VERIFIED: main.go:262-272, cmd_daemon.go:20-30]. Add `perm-probe` there and to the usage string.

### Pattern 2: State model and sources (DPRM-02)
| State | Source of truth | Notes |
|-------|----------------|-------|
| `off` | desktop.json alwaysAvailable false | Old link serves the static "off" page (discretion item) |
| `needs_permission` | SCK -3801 (SCStream start or `SCShareableContent`) or probe `sr:false` when idle | Fresh probe at idle; effect while capturing |
| `display_asleep` | `CGDisplayIsAsleep(CGMainDisplayID())` polled 1 s, or SCK -3815 "Failed to find any displays or windows to capture" with an empty display list | Display list empty while asleep (r15-r17-B rows); only restart after the display is back |
| `locked` | `CGSessionCopyCurrentDictionary` key `CGSSessionScreenIsLocked`, present only while locked | Stream keeps flowing while locked and display awake (R15, R17); treat as an attribute that coexists with `streaming(n)` for viewer counts `[ASSUMED]` |
| `awaiting_approval` | Stall watchdog classification (Pattern 3) plus the 30-day own timer (b) | Not derivable from frame counts alone (R19) |
| `idle` | enabled, no viewer, permission ok | Capture not running |
| `streaming(n)` | n = live signalling sockets with an established PC | Tie to the socket so a dead PC (up to 25 s ICE failure) does not inflate n `[ASSUMED]` |

Precedence suggestion: `needs_permission` > `display_asleep` > `awaiting_approval` > `locked`/`streaming`/`idle`.

### Pattern 3: Stall watchdog (single recovery and classification path)
**What:** While at least one viewer is connected and no frame (Complete) has arrived for T seconds, a still screen and a dead stream look identical (R19: still screen gives no frames; replayd restart gives "0 Complete and 0 Idle for 100 s or more" with `running` true and no error).
1. Nudge: post a 1-px move and move back (spike `spk_mouse_nudge` moves +1 right and does not return it, and reports 0 at the right screen edge [VERIFIED: sys_darwin.m:62-76]; production must return the pointer and pick the direction away from an edge). A frame within about 1 s means a still screen: done. The nudge is skipped when Accessibility reads false (child probe).
2. No frame after the nudge: stop and restart the SCStream (new `SCStream`, forced IDR on the first encoded frame).
3. Classify by the restart: error -3801 → `needs_permission`; error -3815 or empty display list → `display_asleep`; started but still no frame after a second nudge → `awaiting_approval`; frames resume → recovered, log one stall line (counts only).
**T** is a constant; choose 5 s `[ASSUMED]` (the spike's per-record loops used 1-5 s).
Whether a *new* SCStream recovers after a replayd restart is unmeasured (VM row V5, method: R19-c1 with `killall replayd` in a disposable VM, then restart in the same pid).

### Pattern 4: Share-server lifetime and handoff (`serverNeeded`)
**What:** One predicate replaces the three places that infer "something is shared".
- `unshareLocked` stops the server only when `len(a.shares) == 0` [VERIFIED: internal/daemon/share.go:355-359]; change to `!a.serverNeededLocked()`.
- `oldHandoff.global` passes the share listener only when `ws != nil && len(g.Shares) > 0` [VERIFIED: handoff.go:647-653]; change to `serverNeeded`.
- `takeover.finalize` drops everything when `len(shares) == 0` [VERIFIED: handoff.go:1148-1156]; it must instead build the server when always-available is on, using desktop.json loaded in `finalize` (it already calls `loadSettingsFromDisk` and `BootstrapCapabilityState`).
- Also check `API.Stop` and the handoff old-side `ws.Stop()` (api.go:343, handoff.go:402) so shutdown still stops the server unconditionally.
- The gob `hsGlobal` already carries `ShareListener bool` and `Shares`; setting `ShareListener` true with zero `Shares` needs no new field, so `HandoffVersion` stays 1 [VERIFIED: handoff.go:196-197, 913-916]. A v1.2 old daemon passes no listener when nothing is shared, so a new daemon taking over from it must start its own always-available server after the old one exits (port 7444 retry), which the health poller already has to handle.
- The UDP mux port cannot be inherited (no fd frame for it, and the protocol must not change). The new owner binds it with retry until the old daemon has exited (old side exits 0 right after COMMITTED, `handoffExit(0)` [VERIFIED: handoff.go:799]); viewers see a close and reconnect.
- New daemon health poller (does not exist today; `shareHealth` is only read on share-on and takeover [VERIFIED: grep of internal/daemon]): start the always-available server when Tailscale is connected with certs and not already running; restart it on IP/domain change; if the UDP mux read loop dies treat it as server failure `[ASSUMED]`; detect userspace networking by checking the tailnet IP is assigned to a local interface (`net.InterfaceAddrs`), otherwise refuse with a clear state (M-164) `[ASSUMED mechanism]`.

### Pattern 5: Desktop token (DSHR-08)
Mirror `porttoken.go` exactly [VERIFIED: internal/capability/porttoken.go:14-19, 34-39, 66-96]: `crypto/hkdf` `hkdf.Key(sha256.New, sessionKey, nil, info, 32)` with info `portty/desktop-token/v1` (CONTEXT-locked string), HMAC-SHA256 over the marshalled claims, `hmac.Equal`, a `Kind` check returning an `ErrWrongKind`. Claims field order is load-bearing (the HMAC signs the marshalled bytes). Because `Verify` ignores unknown fields, the HKDF separation (not the Kind field) is what stops cross-acceptance, exactly as the port token comment explains. Tests: 3 token kinds × all route families (cross-token matrix), plus Regenerate Signing Key kills the desktop token and `Verify`/`VerifyPort` reject a desktop token.

### Pattern 6: LaunchAgent plist and start path
**What (design; label from CONTEXT):**
```xml
<!-- ~/Library/LaunchAgents/app.portty.Portty.daemon.plist  — keys from man launchd.plist; verify SuccessfulExit semantics in the VM -->
<dict>
  <key>Label</key><string>app.portty.Portty.daemon</string>
  <key>ProgramArguments</key><array><string>/Applications/Portty.app/Contents/MacOS/portty</string><string>daemon</string></array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><dict><key>SuccessfulExit</key><false/></dict>
  <key>AbandonProcessGroup</key><true/>   <!-- recommended, see Pitfall 4 -->
</dict>
```
- `SuccessfulExit false`: "the job will be restarted in the inverse condition" of exit status zero, so non-zero exits restart, zero stays down [CITED: man launchd.plist, macOS 27.2.1 host]. Default `ThrottleInterval` is 10 s; `ExitTimeOut` is system-defined [CITED: same man page].
- Unspecified `ProcessType` applies light resource limits, throttling CPU and I/O [CITED: man launchd.plist "ProcessType"]. CONTEXT and R04-none/R04-interactive found no difference in the VM (28.57 vs 28.60 fps) but exclude real-hardware throttling; make the key a one-line constant and read it back in M-161 under launchd.
- `launchctl kickstart [-kp] service-target`: "run the specified service immediately"; `-k` kills a running instance first [CITED: man launchctl, macOS 27.2.1 host]. Use `-p` to read the pid. Never `-k` in product code (kills sessions). Bootstrap only when `launchctl print gui/<uid>/<label>` says the service is missing (a second `bootstrap` of a loaded label fails) `[ASSUMED]`, and check `print-disabled` for a user-disabled label.
- Write the plist atomically; rewrite and re-bootstrap if `ProgramArguments[0]` differs from the current executable; **refuse to write it when the running executable is on a mounted DMG or an App Translocation path** (Phase 18 UPD-08 moves the app first) `[ASSUMED path shapes]`.
- GUI first start: ensure plist and job, then `kickstart`; wait for `daemonReady`. If `bootstrap` fails, fall back to the existing `startDetachedDaemon` and show a state line (C4-5).
- Legacy cleanup: kardianos names the job `portty-daemon` and writes `~/Library/LaunchAgents/portty-daemon.plist` [VERIFIED: service.go:55; service_darwin.go:110]. Boot it out only after the daemon holding the lock is not that job's pid.
- `RunAtLoad` + `KeepAlive` mean launchd starts the daemon at login with nobody at the Mac, which is the point.
- Daemon must know it is launchd-managed for the re-home: publish a `launchd` flag in `/health` (additive field) from its own start context. Detection candidates: `XPC_SERVICE_NAME` equals the label `[ASSUMED]`, or `launchctl print` pid equals own pid. A GUI-spawned detached daemon is not reliably identifiable by `ppid == 1` (R05's child was reparented to 1 after the registrar exited).
- Re-home in `portty daemon` under launchd: if a healthy same-version daemon holds the lock and is not launchd-managed, run the takeover protocol in-process (`runTakeover`); on refusal or abort exit **0** (launchd would otherwise retry every 10 s, repeatedly freezing writes); never exit 0 on a real startup failure (Pitfall 3). `olderRelease` returns false for equal builds, so a same-version takeover is accepted [VERIFIED: handoff.go:330-348 and the refuse list at 466-474].

### Pattern 7: Power, wake, display (DPRM-04)
- Assertions: `kIOPMAssertPreventUserIdleDisplaySleep` = `CFSTR("PreventUserIdleDisplaySleep")` (IOPMLib.h:314) and `kIOPMAssertPreventUserIdleSystemSleep` = `CFSTR("PreventUserIdleSystemSleep")` (IOPMLib.h:292), created with `IOPMAssertionCreateWithName` at first viewer join, released at last leave and on every shutdown path [VERIFIED: IOPMLib.h in MacOSX26.5.sdk, lines 275-345].
- Header semantics to honour [VERIFIED: IOPMLib.h:275-345]: the display assertion "does not light up the display" if already off; the display may still sleep for lid close or machine sleep; "While the display is prevented from dimming, the system cannot go into idle sleep"; "no effect if the system is in Dark Wake"; the system-sleep assertion permits the display to dim while preventing idle system sleep, and "may still sleep for lid close, Apple menu, low battery".
- Join: `IOPMAssertionDeclareUserActivity(name, kIOPMUserActiveRemote, &id)` (kIOPMUserActiveRemote at IOPMLib.h:439; the header recommends it for remote clients that need the framebuffer, lines 611-613) lights a sleeping display; it postpones display sleep only up to the user's Energy Saver timer (header lines 541-575), so the persistent hold is the assertion. On the VM the wake call returned IOReturn 0 and the display read awake at the next poll (R18-vm); real-hardware lighting is unmeasured because the host run called it after the Mac had already woken (R18-host) (M-159).
- Evidence limits (do not over-promise): R18-host on battery: display slept by `pmset displaysleepnow`, SCK ended 0.70 s later with -3815, the Mac locked at once, and 5.5 s after the display went off it entered Idle Sleep despite a PreventUserIdleSystemSleep from another process [VERIFIED: SPIKE-MATRIX R18-host row]. A locked Mac's display turns off about 31 s after the lock unless held awake; with `caffeinate -d` the stream survived a 45 s lock with 0 errors (R17-A/B awake variants) [VERIFIED: SPIKE-MATRIX R17-A, R17-B rows].
- Wake: reuse `internal/wakewatch.Watch` (wall-clock gap, no monotonic timers, works with no run loop) to restart capture and re-derive state after sleep; also react to SCK -3815.
- A monotonic ticker pauses across sleep on this platform, so every timeout in the watchdog and viewer-count logic should compare wall clock or be re-armed after a wake event.

### Pattern 8: WebRTC server (from the spike, reference only, never imported)
```go
// Source: spike/desktop/server.go:312-317 (read this session)
se := webrtc.SettingEngine{}
se.SetICEUDPMux(s.mux)                                   // ice.NewUDPMuxDefault over one *net.UDPConn on the tailnet IP
se.SetLite(lite)                                         // ICE-lite: one host candidate, no checks from the server side
se.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeUDP4})
se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)  // no 5353 socket
se.SetIPFilter(func(ip net.IP) bool { return ip.Equal(s.bind.AsSlice()) })
```
Other spike constants worth keeping as named constants: 8 Mbps average bitrate default (`main.go:114`), 30 fps, queueDepth 5, `showsCursor = YES`, `RealTime` true, `AllowFrameReordering` false, `ProfileLevel` `kVTProfileLevel_H264_ConstrainedBaseline_AutoLevel`, SCK size = `display.width * filter.pointPixelScale` capped at 2560 long edge [VERIFIED: spike/desktop/media_darwin.m:166-173, 380-382, 915-920]. The spike used `MaxKeyFrameInterval = fps*3600` (keyframes on demand only); Phase 19 needs a real ceiling constant (suggest a few seconds `[ASSUMED]`).
- Bind the UDP mux to the tailnet IP only; the mux port is fixed and travels in the SDP. Use the same number as the TCP share port (7444, `tailnet.DefaultProbePort`) or another fixed one `[ASSUMED]`; document it next to the share port.
- One PeerConnection per viewer; close it when its signalling socket closes (do not wait for ICE failure). A cap on concurrent viewers (suggest 8 `[ASSUMED]`) bounds per-peer packetization and bandwidth (8 Mbps × N).
- Coalesce PLI/FIR into at most one forced IDR per interval (suggest 1 s `[ASSUMED]`); an IDR on join comes from the cached last frame (R22: 0.8-1.2 s first picture on a still screen).

### Pattern 9: Viewer route and entry (DVWR-02)
- `/desktop/` serves the existing SPA bundle (`getStaticAppFS()`), `GET /desktop/` and assets behind `cspHeaders` + `requireSameUserPage`; only the entry document additionally checks the desktop token (hashed assets are fetched without `?cap=`). Mirror the `/app/` handler (server.go:675-720), including `w.Header().Del("Cache-Control")` for `assets/` and `no-cache` for the manifest/icons, so the Vite hash contract holds [VERIFIED: server.go:544-720].
- Vite `base: './'` resolves assets relative to the page, so they load from `/desktop/assets/…` [VERIFIED: frontend/vite.config.ts base comment]. The manifest has `scope: "./"`, `display: standalone` and no `start_url`/`id`, so the Home Screen launch URL is the exact saved link, token included [VERIFIED: frontend/public/manifest.json; csp_mw.go Amendment 4 text].
- `detectMode()` returns `'web'` only under `/app` and `'desktop'` for everything else [VERIFIED: frontend/src/lib/webMode.ts:46-52]. Add a mode for `/desktop/` and branch in `main.tsx` before rendering `App` (the `is-desktop` class is also keyed on `detectMode() === 'desktop'`, main.tsx:9).
- Viewer: `<video muted playsinline autoplay>` with `srcObject`, `object-fit: contain`; an explicit `play()` in `ontrack` and a "Tap to play" overlay (R14: iPhone Safari, Home Screen and Chrome iOS never showed a picture from autoplay; a tap fixed it); recreate the PeerConnection from scratch on every failure; retry with capped backoff 1-10 s on socket close, ICE failed/disconnected, `visibilitychange` to visible, `pageshow` and the `online` event; after about 30 s of failures show "Can't reach this Mac" with a "may be asleep" hint (C10).
- Chrome on macOS shows "Allow Google Chrome to find devices on local networks?" at the first connection on the *viewer's* Mac; the connection did not wait for the answer in R12, so show a one-line hint in the viewer's connecting state and in Copy Link help [VERIFIED: SPIKE-MATRIX R12-lite/R12-full text; R13 shows the Wails shell needed the Allow before ICE left `new`, which is Phase 21's concern].

### Pattern 10: Crash containment (C9)
- Nil-guard every ObjC collection literal and factory result before use; the spike's SIGTRAP came from a nil font passed into an `@{}` literal, and the fix was to skip when nil [VERIFIED: evidence/host-run-crash-diagnosis.txt "Root cause" and "Spike fix"].
- `@try/@catch` in bridge entry points and in every callback you own; `recover()` in every desktop goroutine and in cgo callbacks (VT output, SCK sample handler) before they touch daemon state.
- Crash-loop breaker: persist a counter file or field when capture starts, clear it on clean stop; after N abnormal exits within a window, start with capture disabled and show it in the state line `[ASSUMED design]`. Hard crashes still end all terminal sessions (the daemon holds the PTY masters; share.go comment: "a daemon restart ends every session"), so prevention beats restart.

### Anti-Patterns to Avoid
- **In-process preflight polling** (`CGPreflightScreenCaptureAccess`, `AXIsProcessTrusted`, `CGPreflightPostEventAccess`) anywhere except the probe child.
- **`launchctl kickstart -k`** or `bootout` on a job whose daemon holds sessions.
- **Early `return` for fatal startup errors in `runDaemonCore`** (exit status 0 = stay down under `SuccessfulExit false`); **`os.Exit(1)` for "already running"** (would loop).
- **A second share server or listener for the desktop** (decision: same listener; Phase 22 relies on the shared route family).
- **A monotonic ticker across sleep** for watchdog or viewer logic.
- **Default-false on a failed probe** (`or {}` pattern): a probe error is its own error state with the reason.
- **Writing the plist from a DMG or translocated path.**
- **Logging tokens, SDP bodies or tailnet IPs.** Only counts, state names and error codes.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| WebRTC stack, ICE-lite, DTLS-SRTP, RTP packetizing, NACK | Any custom transport | pion/webrtc v4 + interceptors | Locked; spike-proven in 4 viewer engines |
| H.264 encoding | Software encoder, ffmpeg | VideoToolbox hardware H.264 | R20: 27 % own CPU at 2560 px, 30 fps |
| Screen capture | CGDisplayStream, CGWindowList polling | ScreenCaptureKit | Frames are IOSurface-backed, follows live TCC |
| Wake detection | NSWorkspace notifications | `internal/wakewatch` | Works without a run loop; already tested |
| Token signing | New crypto | `crypto/hkdf`, `crypto/hmac` mirroring `porttoken.go` | Same separation property by construction |
| LaunchAgent management | Shelling a hand-built plist inline everywhere | One `launchd` helper behind the Runner seam | Idempotent bootstrap, print, kickstart, cleanup; testable without the user's domain |
| Permission polling | In-process preflight timer | `portty daemon perm-probe` child | Cached per process |
| WebSocket | Own framing | coder/websocket (existing) | HARD-02 read limit pattern exists |
| QR code, MDNS, STUN | Anything | Out of scope | Decisions and DESK-02 |

**Key insight:** every piece that looks deceptively simple here (preflight, kickstart, sleep, exit codes) has a measured or documented behaviour that differs from the obvious one; the work is mostly in respecting those behaviours, not in writing new capability.

## Common Pitfalls

### Pitfall 1: Cached permission answers in a long-running process
**What goes wrong:** Onboarding shows "not granted" after the user granted, or "granted" after a revoke; the lapse logic trusts the wrong answer.
**Why it happens:** Preflights keep their first answer for the process lifetime (R04 pids 3092 and 3402; R08-A/B).
**How to avoid:** Pattern 1; a test that fails if the three preflight symbols are called outside the probe.
**Warning signs:** The sheet needs a daemon restart to turn green; `capture` works while the dot is red.

### Pitfall 2: SCK stream silently frozen or stalled
**What goes wrong:** A viewer sees a frozen picture indefinitely.
**Why it happens:** `replayd` restart freezes a running stream with no error (R19-c1); still screens also deliver no frames; display sleep ends it with -3815; a lock does not end it while awake.
**How to avoid:** Pattern 3; do not equate "no frames" with "still screen" or with "approval pending".
**Warning signs:** `running` true with zero Complete and zero Idle frames.

### Pitfall 3: launchd restart semantics
**What goes wrong:** The daemon never comes back after a startup failure, or retries forever after a refused re-home, or a CLI start kills sessions.
**Why it happens:** `SuccessfulExit false` restarts on non-zero only; `runDaemonCore` returns normally after printing fatal errors [VERIFIED: process.go:102-146: capability bootstrap, relay and API start failures all `return`]; `kickstart -k` kills the job.
**How to avoid:** Audit every exit path: "already running (lock)" and "re-home refused" exit 0; genuine startup failures exit non-zero (throttled to every 10 s, so persistent failures do not spin). Never `-k`. Test exit codes through a seam.
**Warning signs:** `launchctl print` shows `last exit code = 0` with no process after a failure; log shows repeated takeover attempts every 10 s.

### Pitfall 4: launchd kills what the job leaves behind
**What goes wrong:** When a launchd-managed old daemon exits 0 after a handoff, launchd kills "any remaining processes with the same process group ID as the job" unless `AbandonProcessGroup` is true [CITED: man launchd.plist "AbandonProcessGroup"].
**Why it happens:** Terminal children are started in their own sessions so they should have their own group, but this is not measured under launchd.
**How to avoid:** Set `AbandonProcessGroup` true (defensive; it only changes cleanup of leftovers) and add VM row V3: a launchd-managed daemon hands off sessions, and shells keep their pids and history after the old daemon exits. This guards UPGR-01..05 (SC5).

### Pitfall 5: Legacy agent cleanup stops the daemon that holds the sessions
**What goes wrong:** `launchctl bootout gui/<uid>/portty-daemon` SIGTERMs a daemon that still owns terminals.
**How to avoid:** Remove the legacy plist and job only after the lock-holding daemon is not that job's pid (the upgrade handoff naturally moves it); otherwise delete the file and defer the bootout.

### Pitfall 6: Display sleep, lock and wake interplay
**What goes wrong:** The stream ends 0.7 s after display sleep and never returns; on battery the Mac idle-sleeps 5.5 s later; a click or key press may not wake a dark display (only a user-activity assertion did in the VM).
**How to avoid:** Pattern 7. Restart only when `CGDisplayIsAsleep` is false and SCK lists a display; show `display_asleep` meanwhile. A stale cached picture is sent to late joiners by the spike's keepalive (R18-vm viewer saw a stale chart), so a new viewer joining during `display_asleep` must see the state, not the cached frame.

### Pitfall 7: Nudge side effects
**What goes wrong:** The spike nudge drifts the pointer one point right each time and fails at the right edge; a posted event also counts as user activity.
**How to avoid:** Move and return, choose the direction away from the edge, compare locations; use the nudge only while a viewer is connected and frames stalled; never on a physical-use Mac without need (Phase 22 will need to tell remote from physical input, which Phase 20 handles with an event tag).

### Pitfall 8: pion and UDP mux handoff and network churn
**What goes wrong:** During an upgrade the new daemon cannot bind the fixed UDP port until the old exits; after a Tailscale restart the mux read loop may die while the TCP listener lives `[ASSUMED]`.
**How to avoid:** Retry bind with backoff; health poller recreates the mux and PeerConnections; viewers reconnect via the socket they already have or the page's backoff.
**Test constraint:** VM rows must not use `tailscale down` (memory rule); use pf drops for network loss, unit seams for IP change.

### Pitfall 9: Share-server lifetime regressions
**What goes wrong:** Turning off the last terminal share stops the server and kills the desktop link, or toggling desktop stops terminal viewers; handoff drops the listener.
**How to avoid:** Pattern 4 with tests in both directions (SC5), including: terminal share on, desktop on, then each off in both orders; handoff with desktop-only; handoff with both.

### Pitfall 10: CSP and WebRTC in the actual browsers
**What goes wrong:** `default-src 'none'` with no `media-src` could block `<video>` or the manifest, or a future browser could start applying `connect-src` to WebRTC `[ASSUMED]`.
**How to avoid:** C7: chromedp e2e with a violation listener under the real header CSP plus M rows for Safari desktop and iPhone; pin the policy string equal in a unit test (csp tests exist).

### Pitfall 11: Frontend boot mode
**What goes wrong:** `/desktop/` boots the Wails-coupled `App` and fails calling Wails bindings (C8).
**How to avoid:** New mode in `detectMode`, branch in `main.tsx`, unit test for the three paths.

### Pitfall 12: Playwright's Chromium cannot decode H.264
**What goes wrong:** A decode assertion in Playwright/Chromium fails for the wrong reason `[ASSUMED]`.
**How to avoid:** Fan-out, IDR-on-join and reconnect are Go-to-Go pion tests (count RTP, check NAL types 5/7/8 with `pion/rtp/codecs`); CSP and page-boot checks use chromedp; decode is VM and M-row territory (R11-R14 precedent).

## Code Examples

### Desktop token mirror of the port token
```go
// Source pattern: internal/capability/porttoken.go:14-19,34-39 (read this session)
const DesktopTokenKind = "desktop"
const desktopKeyInfo = "portty/desktop-token/v1" // CONTEXT-locked string

func DesktopKey(sessionKey []byte) ([]byte, error) {
	if len(sessionKey) == 0 { return nil, errors.New("capability: empty session key") }
	return hkdf.Key(sha256.New, sessionKey, nil, desktopKeyInfo, 32)
}
// DesktopClaims field order is load-bearing (the HMAC signs the marshalled bytes):
// Kind "desktop", Mode "always", GrantID, IAT (persisted in desktop.json), V 1.
```

### Nudge and return (production shape)
```objc
// Source pattern: spike/desktop/sys_darwin.m:62-76 (moves +1 and does not return; fails at the right edge)
// Production: pick dx = (p.x + 1 < maxX) ? +1 : -1, post p+dx, verify moved, post p back, verify returned.
```

### serverNeeded
```go
// Replace internal/daemon/share.go:355 `len(a.shares) == 0` and handoff.go:647-653 / 1148-1156
func (a *API) serverNeededLocked() bool { return len(a.shares) > 0 || a.desktop.alwaysAvailable }
```

### Fresh-process probe output (child side)
```objc
// preflight_darwin.m: the ONLY file that may call these three, never request/prompt variants
// printf("{\"sr\":%s,\"ax\":%s,\"pe\":%s}\n", CGPreflightScreenCaptureAccess()?"true":"false", AXIsProcessTrusted()?"true":"false", CGPreflightPostEventAccess()?"true":"false");
```

### Detect mode extension
```ts
// Source: frontend/src/lib/webMode.ts:46-52 (current behaviour: '/app' -> 'web', everything else -> 'desktop')
export type AppMode = 'desktop' | 'web' | 'desktopview'
// '/desktop' or '/desktop/...' -> 'desktopview' (checked before the fallthrough to 'desktop')
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Permission prompt once | Monthly "Allow For One Month" re-approval for ScreenCaptureKit apps | macOS Sequoia (2024) [CITED: 9to5mac.com/2024/08/14/macos-sequoia-screen-recording-prompt-monthly] | DPRM-03 exists because of it; on macOS 27.2 the VM showed a "bypass the system private window picker" prompt on capture start with a 30-day policy (R19) |
| One Screen Recording category | macOS 27 betas also show a "Remote Desktop" category that `CGPreflightScreenCaptureAccess` does not reflect; Apple DTS: "no API that specifically returns the state" | Aug 2026 forum thread (macOS 27 beta 4) [CITED: developer.apple.com/forums/thread/841530] | Not seen in the 27.2 VMs (R09-B lists only Screen & System Audio Recording and Accessibility); M-163 on the office Mac must confirm |
| SMAppService helper | Legacy LaunchAgent daemon, in-process capture | FEAS-02 (Phase 17) | One TCC identity; Login Items shows the signing developer's name |

**Deprecated/outdated:** `kIOPMAssertionTypePreventSystemSleep` ("not supported in any OS X releases", IOPMLib.h) - do not use; use `PreventUserIdleSystemSleep` [VERIFIED: IOPMLib.h in MacOSX26.5.sdk].

## Runtime State Inventory

Not a rename or refactor phase, but it migrates start-model state, so the same questions were answered.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | `settings.json` (engine drops unknown fields on save); new `desktop.json` 0600 | Code: new file with atomic write; data migration: none (greenfield file) |
| Live service config | Tailscale: tailnet HTTPS certs and CT acknowledgement `ct_disclosed` | None; reuse `ct_disclosed` |
| OS-registered state | Possible legacy `~/Library/LaunchAgents/portty-daemon.plist` from `portty daemon install`; new `app.portty.Portty.daemon` plist; Login Items row under the signing developer; TCC rows for the bundle ID | Plist write + bootstrap (code); legacy cleanup ordering (Pitfall 5); onboarding copy for Login Items |
| Secrets/env vars | `capability.key` (0600) drives the desktop key; none new | None |
| Build artifacts | `build/darwin/Info.plist` `LSMinimumSystemVersion` 10.13.0 → 14.0; cgo flags | Edit Info.plist and cgo CFLAGS/LDFLAGS (frameworks) |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | An exec'd child of the launchd daemon (`portty daemon perm-probe`) reads the live TCC state attributed to the app bundle ID | Pattern 1 | Probe reads wrong identity; onboarding stuck. VM row V6 |
| A2 | UNUserNotificationCenter can post from the launchd-run Mach-O with no NSApplication | C6 | DPRM-03 notification fails with the GUI closed |
| A3 | `launchctl kickstart` without `-k` starts a loaded idle job and is a no-op on a running one | C4, Pattern 6 | CLI/GUI cannot restart the daemon |
| A4 | launchd sets `XPC_SERVICE_NAME` to the label for agents | Pattern 6 | Daemon cannot tell it is launchd-managed; use `launchctl print` pid instead |
| A5 | launchd only kills the old job's process group, and PTY children are in their own sessions | Pitfall 4 | Handoff exit under launchd kills shells (UPGR regression); mitigate with `AbandonProcessGroup` |
| A6 | The `x-apple.systempreferences:` Privacy_ScreenCapture/Privacy_Accessibility pane URLs open on 27.2 | Onboarding | Button opens the wrong pane |
| A7 | The pion UDP mux read loop dies when the utun interface disappears | Pitfall 8 | Media never returns after a Tailscale restart without the poller |
| A8 | `default-src 'none'` does not block a `<video srcObject>` MediaStream or WebRTC in Chrome and Safari | C7, Pitfall 10 | Black video; needs a CSP amendment |
| A9 | A new SCStream recovers after a replayd restart | Pattern 3 | Watchdog restarts do not recover; needs daemon-level recovery |
| A10 | Playwright's bundled Chromium lacks H.264; installed Chrome has it | Pitfall 12 | Wrong test harness choice |
| A11 | Viewer cap of 8, PLI coalescing 1 s, stall threshold 5 s, keyframe ceiling a few seconds | Pattern 3, 8 | Tuning only |
| A12 | `kIOPMUserActiveRemote` lights a sleeping display on real hardware | Pattern 7 | Display stays dark; fall back to `kIOPMUserActiveLocal` (M-159) |
| A13 | The approval prompt window can be detected by owner and layer without Screen Recording | C2 | Signal (d) unavailable |
| A14 | `SCShareableContent` while ungranted may raise a prompt per call on 27.2 | Alternatives | Do not poll it ungranted (already the design) |
| A15 | Idle probe every few minutes is cheap enough | Pattern 1 | Cadence tuning |
| A16 | Lock state can coexist with `streaming(n)` as one attribute | Pattern 2 | UX wording |
| A17 | Detecting userspace Tailscale via "tailnet IP not on a local interface" | Pattern 4 | False refusal/accept |
| A18 | A `!darwin` stub for `internal/screencap` keeps `go vet` portable | Structure | Build noise only |

## Open Questions

1. **Should always-available hold a system-sleep assertion permanently (not only while a viewer is connected)?**
   - What we know: DPRM-04 and CONTEXT scope assertions to "first viewer join to last leave"; a sleeping Mac cannot be woken from the tailnet (REQUIREMENTS Out of Scope); R18-host showed the Mac idle-sleeps within seconds of display sleep on battery.
   - What's unclear: whether "any time" (SC1) means the Mac must be kept awake by Portty or by the user's Energy settings. This is a product choice, not decided in CONTEXT.
   - Recommendation: keep CONTEXT's scope, put an Energy-settings hint in the Desktop group footnote, and surface "Can't reach this Mac" in the viewer. Raise it with the user if M-159 shows the office Mac sleeps overnight.
2. **What is the actual behaviour of the real 30-day prompt?** Frames flowed through the aged-approval prompt in the VM (R19-b), but the real monthly prompt was never observed (M-160). Plan the detector so (c) and (d) can be tuned after M-160 without schema changes.
3. **Does the monthly dialog block remote clicks?** Phase 20 will have control; whether a synthetic click can press "Allow" in a system dialog is unknown. Not Phase 19's decision; note for Phase 20.
4. **Is "Remote Desktop" a separate TCC row on the shipping office Mac?** Seen in macOS 27 betas only (CITED forum); not in the 27.2 VMs. M-163 confirms.
5. **Is bootstrap of a user LaunchAgent allowed on the corporate Mac, and can Login Items be disabled by policy?** Unknown; C4-5 keeps a detached-spawn fallback.

### Measurements to schedule early (VM rows, no host side effects)
Run in the disposable Tart VMs only (R00 GO), using the R19 vanilla-clone method for approval-prompt work (the approval store was writable there; backdating raised the prompt at once). Do not use `tailscale down`.
- **V1** Daemon-posted UNUserNotificationCenter notification from a launchd job (gates DPRM-03 notification).
- **V2** `launchctl kickstart` (no `-k`) on a loaded idle job; and on a running job; `bootstrap` twice; `print-disabled` behaviour.
- **V3** launchd-managed daemon hands off sessions; shells keep pids and history after the old daemon exits (with and without `AbandonProcessGroup`).
- **V4** `XPC_SERVICE_NAME` in a launchd agent (or use `launchctl print` pid).
- **V5** replayd killed under a running stream, then a new SCStream in the same pid.
- **V6** Probe child exec'd from the launchd daemon: tccd subject, fresh answer both directions, no prompt at 2 s cadence.
- **V7** Header-CSP check in Chrome (chromedp e2e) and Safari/iPhone (M row) with a real stream.
- **V8** Approval-prompt window owner and layer visible without Screen Recording (signal d), using the backdating method.
- **V9** `x-apple.systempreferences:` pane URLs open the right panes on 27.2.
- **V10** SR toggled off while an SCStream is running: error callback or silent stall.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | build and test | ✓ | go1.27.4 darwin/arm64 (go.mod: 1.26.4, toolchain go1.26.11) | — |
| macOS SDK with ScreenCaptureKit | cgo | ✓ | MacOSX26.5.sdk (CLT) | — |
| clang / xcrun | cgo ObjC | ✓ | present | — |
| Node 24.22.1, pnpm 12.6.0 | frontend | ✓ | — | npm |
| Tart | VM rows | ✓ | 2.41.1 | — (VMs not probed this session) |
| lsof | tailnet-bind tests | ✓ | /usr/sbin/lsof | — |
| Google Chrome, Safari | e2e, M rows | ✓ on this host (Chrome.app, Safari.app present) | not executed by research | headless Chromium lacks H.264 `[ASSUMED]` |
| launchctl, pmset | start path, power rows | ✓ | — | — |
| Real office Mac | M-157, M-159, M-160, M-161, M-163, M-164 | ✗ not reachable by research | macOS 27.2.x per notes | user-run rows |

**Missing dependencies with no fallback:** the office Mac rows are user-run (as already recorded in TESTING.md Category BH).
**Missing dependencies with fallback:** none.

## Validation Architecture

Skipped: `workflow.nyquist_validation` is `false` in `.planning/config.json` [VERIFIED: config.json]. Test layout is already fixed by CONTEXT [Testing]; the concrete additions this research implies are listed in the pitfalls (cross-token matrix, lifetime both directions, exit-code seam, probe-symbol lint, CSP pin, `detectMode`, Go-to-Go loopback) and in the VM rows above. Per the standing rule, add Section 2 manifest entries, Section 4 traceability rows and Section 5 M-NN items (and reword M-163, which currently says the grant "is confirmed by preflight polling").

## Security Domain

`security_enforcement` is enabled (ASVS level 1, block on high) [VERIFIED: config.json].

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | Tailscale WhoIs same-user gate (`requireSameUser*`) plus HKDF-separated desktop token as second factor |
| V3 Session Management | yes | Viewer session = signalling socket lifetime; PeerConnection closed with it; revoke by grant ID off/regenerate key |
| V4 Access Control | yes | Kind separation by HKDF (desktop vs session vs port tokens); route gate order; own-account only in this phase |
| V5 Input Validation | yes | Signalling JSON with version field and size limit (32 KiB read limit), strict decode; SDP parsed by pion; no view-only violations since no input path exists in Phase 19 |
| V6 Cryptography | yes | `crypto/hkdf`, `crypto/hmac`, `hmac.Equal`; DTLS-SRTP by pion; TLS by Tailscale certs; never hand-rolled |
| V7 Error handling and logging | yes | Never log tokens, SDP, tailnet IPs; counts and state names only |
| V9 Communications | yes | TLS 1.2+ on the share server; media on the tailnet only |
| V14 Configuration | yes | `desktop.json` 0600 atomic; plist 0644 in user LaunchAgents; CSP pinned |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Token leaks via URL (Referer, cache, history) | Information disclosure | `Referrer-Policy: no-referrer`, `Cache-Control: no-store` on the page (`cspHeaders`), token only in query of same-origin requests; saved links accepted by design |
| Cross-site WebSocket hijack | Spoofing/Tampering | `requireAllowedOrigin` outermost, then same-user, then token |
| Token confusion between kinds | Elevation of privilege | HKDF key separation + kind check; cross-token tests (DSHR-08) |
| Media egress off-tailnet (STUN/TURN, wrong interface) | Information disclosure | empty iceServers, UDP4, mDNS off, IP filter, bind refusals, tests asserting sockets and ports |
| Unauthenticated UDP to the mux | Denial of service | pion drops STUN with unknown ufrag; viewer cap; bind only to tailnet IP |
| Viewer sees sensitive screen content (lock screen, notifications, tokens in terminals) | Information disclosure | Inherent to the feature; own-account-only gate; visible capture indicator on the Mac; guest paths are Phase 22 |
| Unattended capture without consent | Repudiation | Capture only while a viewer is connected; Screen Recording grant is explicit; monthly re-approval is a feature |
| Stale grant after disable | Elevation of privilege | Turning always-available off clears the grant ID and closes sockets/PCs immediately |
| Daemon crash cascades to terminals | Denial of service | Crash containment (Pattern 10), crash-loop breaker |
| Local Network permission prompts on viewer devices | — | Documented, no mitigation needed |

## Sources

### Primary (HIGH confidence, read this session)
- In-repo: `.planning/phases/19-unattended-desktop-viewing/19-CONTEXT.md`, `20-CONTEXT.md`, `.planning/REQUIREMENTS.md`, `17-SPIKE-MATRIX.md` (rows R00, R01, R04-R10, R14-R19, FEAS-02 and requirement-changes sections), `17-04-SUMMARY.md`, `17-12-SUMMARY.md`, `.planning/config.json`, `.planning/STATE.md` (blockers), `TESTING.md` Category BH (M-157..M-164)
- Evidence files: `evidence/r04-daemon.jsonl`, `r02-helper.jsonl`, `r05-spawn.jsonl`, `r04-attribution.txt`, `r05-r07-attribution.txt`, `r09-B-settings.txt`, `r19-approvals.txt`, `host-run-notes.txt`, `host-run-crash-diagnosis.txt`; `.planning/debug/spike-hostrun-sigtrap.md`
- Code: `internal/daemon/share.go`, `handoff.go` (lines 1-80, 330-348, 440-500, 600-662, 780-802, 905-945, 970-1000, 1120-1320), `process.go`, `process_unix.go`, `service.go`, `api.go` (routes, Stop, regenerate key), `cmd_daemon.go`, `main.go`; `internal/webserver/server.go` (routes), `capability_mw.go`, `csp_mw.go`, `browser_csp_e2e_test.go`; `internal/capability/porttoken.go`; `internal/wakewatch/wakewatch.go`; `spike/desktop/{server.go,sys_darwin.m,media_darwin.m,main.go,go.mod}`; `frontend/src/{main.tsx,lib/webMode.ts}`, `frontend/{vite.config.ts,package.json,public/manifest.json}`; `build/darwin/Info.plist`, `build/entitlements.plist`, `notification_darwin.go`
- Apple docs on this Mac: `man launchd.plist`, `man launchctl`; `IOPMLib.h` (MacOSX26.5.sdk IOKit headers)
- Go module proxy and GitHub API for pion versions and repo facts

### Secondary (MEDIUM confidence)
- Apple Developer Forums thread 841530 (macOS 27 beta "Remote Desktop" category; Quinn: no API) - https://developer.apple.com/forums/thread/841530
- 9to5Mac, "macOS Sequoia screen recording prompt monthly" - https://9to5mac.com/2024/08/14/macos-sequoia-screen-recording-prompt-monthly/
- Apple Developer Forums on stale AX/CG preflight symptoms (relaunch is the common workaround; no Apple-documented per-process cache statement found): https://developer.apple.com/forums/thread/727984, https://developer.apple.com/forums/thread/735204

### Tertiary (LOW confidence)
- Web search results on CGPreflightScreenCaptureAccess behaviour were mixed (one poster reports it always correct; signing identity issues cause false negatives). The spike's own measurements above supersede them for this stack.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH - versions verified on the Go proxy, spike-proven pins, user-locked.
- Permission-detection finding: HIGH for macOS 27.2 in the VM daemon shape (measured both directions); MEDIUM for the office Mac (FA-1/M-163).
- Architecture (lifetime, handoff, token): HIGH - read the code paths to change.
- launchd start/re-home, notifications, CSP in real browsers, replayd recovery: MEDIUM-LOW until the VM rows V1-V10 run.
- Pitfalls: HIGH where cited to evidence, tagged `[ASSUMED]` otherwise.

**Research date:** 2026-11-25
**Valid until:** 2026-12-25 (macOS 27.x minor releases may change TCC prompts; pion v4 patch releases are frequent)
