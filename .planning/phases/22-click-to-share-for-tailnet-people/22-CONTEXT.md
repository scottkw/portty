# Phase 22: Click-to-Share for Tailnet People - Context

**Gathered:** 2026-11-25
**Status:** Ready for planning

<domain>
## Phase Boundary

The sharer clicks Share Desktop and gets one guest link that any person on the tailnet can open (DSHR-03). Each guest waits at an Admit prompt on the shared Mac that shows their tailnet identity, and watches view-only until the sharer allows control, and separately clipboard, for that person (DSHR-04). The sharer can stop the share or revoke one viewer at any time, which ends that connection at once, and the link stops working when the share ends (DSHR-05). The shared Mac shows join and leave notifications and a persistent "shared with N" indicator with one-click Stop (DSHR-06), and writes joins, leaves and grants to a local audit log (DSHR-07). With several controlling viewers, the last typer holds control for about 2 s and physical input at the Mac always wins (DCTL-04). The phase gets a recorded security review covering admit, revoke, cross-token and wrong-identity cases. The only widening is a new tailnet-member gate on the guest desktop routes; every terminal, /app/, /api/ and port route keeps requireSameUser(Page) exactly as today (MOB-05). Not in this phase: the input, clipboard and display-picker protocol itself (Phase 20, upstream); the Portty app Desktop tab and the iPhone touch viewer (Phase 21); audio, adaptive quality, unlock and the release (Phase 23); always-available mode and its link and token (Phase 19, settled).

</domain>

<decisions>
## Implementation Decisions

User delegation (2026-11-25): "I don't need to answer a bunch of questions". Reversible technical choices were decided autonomously, product and UX choices are under Claude's Discretion, and the seven hard-to-reverse recommendations below were accepted by the user with "Accept all" (2026-11-25).

### Guest link and token
- The guest link is `https://<mac>.<tailnet>.ts.net:7444/desktop/guest/?cap=<token>`, with signalling at `/desktop/guest/ws`. A separate path lets the route table carry the different gate (tailnet member instead of same user), so the same-user gate on `/desktop/` never depends on parsing a token first, and a route-table test pins which gate each path has. There is no share ID in the path because the token names the grant. The page is served no-store with Referrer-Policy no-referrer, as in Phase 19 — **Reversibility:** costly — a public URL contract that the Phase 21 app and iPhone viewers and every later build must route; guest links are short-lived, so old saved links matter less than for always-available.
- The guest token is signed under its own key, derived by HKDF from capability.key with info `portty/desktop-guest-token/v1` (not the always-available desktop key). Claims in fixed order: kind "desktop-guest", share_id (128-bit random, names the share), iat, v 1; no exp, because the token dies when the share's grant is cleared and Regenerate Signing Key kills it. The separate key makes an always token on the guest route, and a guest token on `/desktop/`, fail the HMAC by construction (the 19-CONTEXT DSHR-08 argument) — **Reversibility:** one-way — the token format is verified by every later build and is pasted into chats and browsers.

### Who may join and how they are identified
- A new requireTailnetPeer(Page) beside requireSameUser admits to the Admit queue only a WhoIs answer whose node is not tagged (no person to name), not shared in from another tailnet (Sharer == 0; those people are outside this tailnet, which REQUIREMENTS puts out of scope), and has a non-zero user ID. Every error fails closed (403 with the marker, 503 retry, as today). Only the guest routes use it, and a route-table test pins every existing route to requireSameUser(Page) — **Reversibility:** costly — widening later is easy, but narrowing after colleagues rely on it, or after a shared-in node has been admitted, is a trust regression.
- The Admit prompt shows the WhoIs display name, the login name (authoritative, from the identity provider; display names can be set to imitate someone, so the login name is always visible) and the device name. They are shown only on the shared Mac, never rendered into any served page or refusal (refuseOtherUser still names no one). An admission and its control and clipboard grants are keyed to (share_id, numeric user ID, stable node ID) and bound to the signalling WebSocket; a reconnect from the same node in the same share (DESK-03) is re-admitted with its grants, and a second device of the same person prompts again — **Reversibility:** costly — it fixes what the sharer trusts when clicking Admit and what the audit log records; changing the key later changes who silently gets back in.

### Admit, grants and stop
- Only the Portty GUI on the shared Mac admits, grants control and clipboard, revokes and stops; there is no remote admit or grant API. A click-to-share exists only while the GUI process (which owns the tray indicator) is alive: if it quits or crashes, the daemon ends the share and disconnects guests, so other people are never watching without the indicator on screen. The host's own account can still admit from home by controlling the Mac through always-available. A guest granted control can click anything on screen, including Portty's own Admit and Allow control buttons, so the grant copy says control means they can do anything you can do on this Mac — **Reversibility:** costly — adding a remote admit path later is a new authenticated write API on the share server, and removing one after release would break users' workflow.

### Audit log
- The audit log is JSON Lines at `~/Library/Application Support/portty/desktop-audit.jsonl`, mode 0600, written by the daemon, one object per event with v 1, ts (RFC 3339, UTC), event, share_id (empty for always-available), mode (guest or own), user_id, login_name, display_name, node_name, node_stable_id and tailnet_ip. Events: share_start, share_stop, admit_request, admitted, denied, join, leave, revoked, control_granted, control_revoked, clipboard_granted, clipboard_revoked. Own-account always-available joins and leaves are logged too, so the Mac records everyone who watched. Tokens, typed input, clipboard content and frames are never logged. Size-capped rotation like daemon.log (about 5 MB, one .1 file) — **Reversibility:** costly — a persisted on-disk format that users or scripts may read and that later builds must keep appending to compatibly.

### Control hand-off (DCTL-04)
- Physical input is told apart from remote input without any new TCC permission. Remote events are posted from Portty's own event source and tagged in the event's source user-data field; physical activity is read from the HID system state (seconds since the last physical key or mouse event) or, if research shows posted events pollute that, from an event tap under the existing Accessibility grant that ignores Portty-tagged events. Remote input is dropped while physical input happened in the last ~1 s. Research must prove which works on macOS 27.2 in the VM before planning locks it — **Reversibility:** costly — adding a permission means another in-person trip to the corporate office Mac and another System Settings entry, the reason 19-CONTEXT rated the permission set costly.

### Decided autonomously
- [Gate] Q: "Guest route gate order?" → Selected: "Page: cspHeaders, requireTailnetPeerPage, guest-token page check (static Link No Longer Works page on failure); WebSocket: requireAllowedOrigin, requireTailnetPeer, guest-token check, then the admission wait (mirrors /sessions/{id}/ws)" — middleware composition, local
- [Gate] Q: "Where does requireTailnetPeer live?" → Selected: "whois.go beside sameUser, reusing sameUserStatus's fail-closed shape (503 on own tailscaled fault, 403 marker otherwise); sameUser itself is not touched" — one function, local
- [Gate] Q: "How is the narrowness of the widening proven?" → Selected: "A route-table test lists every route and its gate and fails if anything other than the guest routes uses requireTailnetPeer; an other-user fake WhoIs gets 403 on /app/, /sessions/*, /api/*, ports and /desktop/" — tests only
- [Tokens] Q: "Which cross-token cases are tested?" → Selected: "Guest token refused on /desktop/, /sessions/*, /api/sessions*, port routes; always token, session capability and port token refused on /desktop/guest/*; a guest token from a stopped share refused; a token from before Regenerate Signing Key refused" — tests only, extends the Phase 19 DSHR-08 tests
- [Grants] Q: "Where do guest shares live?" → Selected: "In daemon memory only (share_id, grant, admissions, per-viewer grants); a daemon restart or upgrade ends the share and its link; nothing goes into desktop.json or settings.json" — narrower than persisting; persistence could be added later
- [Grants] Q: "How many click-to-shares at once?" → Selected: "One active share; Share Desktop while one is active shows the same link; a new Share after Stop mints a new share_id" — runtime policy
- [Revoke] Q: "How do Stop and revoke cut connections?" → Selected: "Clear the grant first, then close the viewer's PeerConnection and signalling WebSocket, releasing any held keys and buttons (the unshareLocked order and revokeSignal pattern); a revoked viewer's next input is dropped even if a frame is in flight" — local ordering
- [Media] Q: "How is media bound to the admitted identity?" → Selected: "A guest's PeerConnection is created only after admission, over its own signalling WebSocket, and the selected ICE pair's remote IP must equal that WebSocket's tailnet RemoteAddr or the peer is closed" — server-side check
- [Admit] Q: "Does the host's own account skip Admit on a guest link?" → Selected: "No: everyone on a guest link follows the same admit and view-only rules; own devices use the always-available link" — narrower and simpler; easy to relax
- [Arbiter] Q: "How is the control lease implemented?" → Selected: "A daemon-side arbiter: a viewer with a control grant takes the lease on a key, click or scroll and holds it until 2 s after its last input; others' input is dropped meanwhile; a lease change and every grant change release held keys; constants, not settings" — runtime policy
- [Arbiter] Q: "Does the lease apply to own-account viewers too?" → Selected: "Yes, all controlling viewers share one lease so two people never type into the Mac at once" — runtime policy
- [Signalling] Q: "How do viewers learn their state?" → Selected: "New versioned signalling messages: admission (waiting, admitted, denied, ended), grants (control, clipboard), lease holder name" — additive to the Phase 19 protocol
- [GUI] Q: "How does the GUI hear about guests?" → Selected: "Daemon socket endpoints for share start and stop, viewer list, admit and deny, grant and revoke, plus a GUI liveness subscription the daemon watches to end the share when the GUI goes away" — internal API
- [Notifications] Q: "Who posts join and leave notifications?" → Selected: "The GUI through the existing sendNotificationFunc seam (the GUI must be alive for a share)" — internal routing
- [Audit] Q: "How is the audit log written?" → Selected: "One appender in the daemon, opened 0600 O_APPEND, one json.Marshal per line, size-capped rotation copied from daemon_log.go; write failures are logged to daemon.log and never block a join" — implementation behind the accepted format
- [Testing] Q: "What test approach?" → Selected: "Fake whoIsClient with other-user, tagged and shared-in identities; loopback integration tests with the synthetic frame source for admit, deny, revoke, Stop, reconnect re-admit and lease hand-off; Vitest for the sheet and guest page states; VM rows for physical-input detection; a real two-account tailnet M-NN row for SC1; the security review via gsd-secure-phase; TESTING.md and the traceability map updated per the standing rule" — test layout

### Claude's Discretion
User delegated: 'I don't need to answer a bunch of questions' (2026-11-25). Claude may adjust these during planning:
- Entry point: a Share Desktop item in the tray menu and a Share Desktop button in the Settings Desktop group, both opening one sheet with Copy Link, the viewer list and Stop Sharing; no new toolbar button. Reason: the toolbar Share is per session and Phase 19 put desktop controls in Settings ▸ Sharing.
- The first Share Desktop reuses the existing Certificate Transparency acknowledgement (ct_disclosed). Reason: the guest link publishes the same tailnet name.
- Admit prompt: the Portty window comes forward with a sheet showing the display name in bold, then login name and device, with Deny and Admit buttons and no default button; a native notification also fires in case the window is hidden. Reason: an accidental Return must not admit anyone.
- Viewer list rows: name, device, a View only or Controlling badge, Allow control and Allow clipboard switches, and Disconnect. Reason: matches the existing Switch and Row grouped-form style.
- Indicator: a tray icon variant while anyone (guest or own account) is watching, and a top tray menu line "Desktop shared with N" with Stop Sharing directly under it. Reason: radical simplicity; the tray is always on screen while a share exists.
- Stop Sharing ends the click-to-share and disconnects every guest; own-account viewers appear in the list with Disconnect, and always-available stays as set in Settings. Reason: the guest share and always-available are separate modes, and turning always-available off from the tray would cut the author's remote path by surprise.
- Notifications: one per join and per leave, for guests and own-account viewers, coalesced when several arrive within a few seconds. Reason: DSHR-06 asks to show who is watching; coalescing avoids noise.
- Control grant copy: "Allow control lets <name> use this Mac as you" plus a one-line warning that they can do anything you can. Reason: a control grant is effectively full account access.
- Guest page states: Waiting for <Mac> to let you in, Not admitted, View only, You have control, <name> is controlling, Sharing has ended. Reason: plain words; the requirement text already uses "view-only".
- A pending admission times out after 5 minutes with Not admitted; at most one pending prompt per (user, node) and 8 per share. Reason: stops prompt spam without a config knob.
- A denied or revoked person cannot knock again for the rest of that share. Reason: prevents a refused guest from re-prompting the sharer.
- After research (2026-11-25, user delegated): local input is detected with a listen-only event tap under the existing Accessibility grant (VM-proven: no Input Monitoring prompt); any event without Portty's tag counts as local; CGEventTapIsEnabled is checked after creation; the mask includes drag types. HID idle time is not used (injected events reset it).
- After research: while Secure Event Input is on, physical typing is invisible to the tap; accept the weaker keyboard half and show a GUI warning, rather than drop all remote keys.
- After research: guests get their own asset route behind requireTailnetPeer (the /app/ bundle is same-user only); join/leave signal lives in the Admit sheet, tray icon and tray line, not banners (macOS may silence notifications while the screen is shared); Stop and Revoke close all peers concurrently and also close hijacked WebSockets; media is tied to the guest's node identity, not address strings; guest names are stripped of bidi and control characters.
- After research: any controller input takes or renews the 2 s lease and others' pointer moves are dropped while it is held; deny and revoke are keyed by user ID; at most 2 pending Admit requests per user; audit lines carry a reason field and graceful leaves are logged.
- Real-Mac follow-up: the research Appendix B 2-minute physical-input check runs on a real Mac as an M-NN item (needs Terminal Accessibility temporarily).
</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- internal/webserver/whois.go: whoIsClient seam (SetWhoIsClientForTest; noLiveWhoIs fails tests closed), sameUser (numeric IDs; refuses tagged, shared-in and a tagged or shared-in self), sameUserStatus (0/403/503), requireSameUser and requireSameUserPage, refuseOtherUser (marker, never names an account), handlePeerCap (POST plus JSON content type blocks cross-site reads). requireTailnetPeer is a sibling of these.
- apitype.WhoIsResponse already carries UserProfile.LoginName and DisplayName and Node.ComputedName and StableID; no code reads names today (refusal logging is numeric IDs only), so showing them on the Admit prompt is new.
- internal/capability/porttoken.go: HKDF-derived key, fixed claim order, Kind and V check, ErrWrongKind; the guest token mirrors it under its own info string.
- internal/daemon/share.go: mintShareRecord (128-bit grant ID), unshareLocked ordering (clear grants, kick viewers, then stop the server), remintSharesLocked (Regenerate Signing Key re-mints; for a guest share it should simply end the share).
- internal/daemon/daemon_log.go: 0600 append log with size-capped rotation to .1, the pattern for the audit log; daemonConfigDir in engine.go is ~/Library/Application Support/portty (0700).
- internal/relay/hub.go RevokeWebViewers: the kick-on-revoke precedent for terminal viewers.
- frontend/src/components: SharePopover.tsx (Switch, Copy link, CT line, strings in lib/shareCopy.ts) is the model for the Share Desktop sheet; UserRefused.tsx and LinkNoLongerWorks.tsx are the refusal-page precedents; SettingsSheet.tsx Sharing pane holds the Phase 19 Desktop group.

### Established Patterns
- Every share-server route checks Origin, then WhoIs, then the capability; refusals are static pages with the X-Portty marker; token-bearing pages are no-store.
- Grants are tracked in the web server (AddGrant, ClearGrants, revokeSignal, isGrantActive, session-keyed); callers re-check the grant after subscribing so a concurrent revoke is never missed.
- Tokens are HMAC-signed JSON with fixed field order; each token family gets its own HKDF-derived key so a token of one kind cannot verify as another.
- Seams are package vars or setters with SetXForTest; tests never reach the live tailscaled.
- tray_common.go BuildMenuItems is the single source of truth for the tray menu.

### Integration Points
- internal/webserver/server.go setupRoutes: new /desktop/guest/ page and /desktop/guest/ws routes behind requireTailnetPeer(Page); the /app/ bundle stays behind requireSameUserPage.
- internal/desktop (Phase 19/20, not yet built): share registry, admission queue, per-viewer grants and the lease arbiter on the input path.
- Daemon socket API (api.go): share start and stop, viewer list, admit and deny, grant and revoke, GUI liveness.
- app.go: new bindings beside GetSessionShare, SetSessionShared, HasCTDisclosure and AcknowledgeCTDisclosure; join and leave notifications through sendNotificationFunc.
- tray.go, tray_objc_darwin.m and tray_common.go: icon variant, "Desktop shared with N" line and Stop Sharing.
- Frontend: Share Desktop sheet, Admit sheet and viewer list; the Phase 19 DesktopView.tsx and lib/desktopClient.ts gain guest waiting, view-only and lease states.

</code_context>

<specifics>
## Specific Ideas

Settled by prior decisions (not re-asked):
- Terminal links stay limited to the host's own Tailscale account; only desktop links widen (DSHR-03; PROJECT Key Decision 2026-11-24 widening MOB-05 for desktop links only; ROADMAP Phase 22 SC1 and Notes).
- Guests are view-only until the sharer allows control per person, and clipboard separately per person (DSHR-04; PROJECT Key Decision 2026-11-24; ROADMAP SC2).
- Clipboard moves only by explicit action, concealed items never sync, and it is off by default for guests (DCTL-06; research SUMMARY C2).
- One link per share; anyone holding it waits at Admit, per person (ROADMAP Phase 22 goal and SC1-2).
- Stop and revoke end connections immediately and the link dies with the share (DSHR-05; ROADMAP SC3).
- Join and leave notifications plus a persistent "shared with N" indicator with one-click Stop on the shared Mac (DSHR-06).
- The audit log is local to the shared Mac and never sent anywhere (DSHR-07; PROJECT Network constraint).
- The last typer holds control about 2 s, physical input at the Mac always wins, and there is no multi-cursor (DCTL-04; research FEATURES; REQUIREMENTS Out of Scope).
- Guests never get remote unlock (ROADMAP Phase 23 SC3).
- Nobody outside the tailnet; no STUN, TURN or relay (REQUIREMENTS Out of Scope; PROJECT Network constraint; DESK-02).
- Guest links live in the /desktop/ route family on the same share server, port 7444 and TLS name; /app/ stays behind the same-user page gate (19-CONTEXT Always-available link and token).
- Desktop token keys derive from capability.key by HKDF; Regenerate Signing Key kills every desktop token; WhoIs runs before any token work (19-CONTEXT; porttoken.go precedent).
- desktop.json holds only always-available state (19-CONTEXT).
- In-daemon capture with one shared encoder fanned out to every viewer and an IDR on join; guests ride the same stream (19-CONTEXT; 17-SPIKE-MATRIX FEAS-02).
- Desktop sessions are not handed off on upgrade and HandoffVersion stays 1 (19-CONTEXT; ROADMAP Phase 19 Notes).
- Signalling is versioned JSON over WSS on the share server; new message types extend it (19-CONTEXT Signalling).
- The CSP gets no new directive (PROJECT Security constraint; 19-CONTEXT CSP decision).
- Identity comes from the system tailscaled LocalAPI WhoIs through the existing whois client, no tsnet (research SUMMARY C10).
- No new dependency: pion is already in (19-CONTEXT) and WhoIs data already arrives in apitype.WhoIsResponse.
- The phase gets a recorded security review covering admit, revoke, cross-token and wrong identity (ROADMAP Phase 22 Notes).

Upstream dependencies on Phase 20 (Phase 20 decides these; Phase 22 plans against them and does not change the wire):
- Every pointer, key, Send-keys and clipboard message should pass through one daemon-side entry point that knows the sending viewer, so Phase 22 can add the admission check, per-person grants and the lease arbiter without touching the protocol.
- A server-side call that releases everything held for a viewer (keys, modifiers, mouse buttons), usable by revoke, Stop and a lease hand-off, beyond the client-side release on blur or close.
- Injected CGEvents tagged in the source user-data field from Portty's own event source; if Phase 20 does not tag them, Phase 22 adds it before physical-input detection.
- Clipboard actions addressed per viewer, so a separate clipboard grant can gate them.
- Whether Phase 20 assumes one controller or many, Phase 22 adds the multi-controller lease on top.
- The display or window picker and the mid-session switch reach guests unchanged; guests never pick the display.
- Own-account viewers are expected to have full control with no grant step (DSHR-04 applies to guests only); Phase 22 only adds the lease for them.
- Phase 22's admission, grant and lease messages extend the signalling version and message types additively.

Other specifics:
- Primary scenario: the author at the office Mac shows a colleague their screen. Secondary: the author at home (own account, always-available) admits a colleague by clicking Admit on the remote screen.
- Granting control to a guest is effectively granting the author's whole macOS account (they could open Terminal, Portty Settings or copy a terminal link); the UI says so plainly and control stays per person and revocable.
- Guest reachability depends on the tailnet ACLs allowing tcp/7444 and the desktop UDP media port to the shared Mac; a corporate tailnet may block it and Portty cannot see why, so the guest page's failure state and the docs mention ACLs.
- The macOS screen-recording indicator is already on while anyone views; Portty's indicator adds who and how many.
- Display names are user-settable and can imitate someone; the login name is the identity that matters and is always shown and logged.
- Real-tailnet two-account checks need a second Tailscale user on the test tailnet; research and subagents never contact the host's tailscaled or 100.x addresses and never touch ~/Downloads or /Applications on the host.
- Security review cases: admit, deny, revoke mid-input, Stop with viewers mid-frame, cross-token in every direction, wrong identity (another user's node, tagged, shared-in, WhoIs fault), a reconnect from a different node, and a guest with control trying to self-grant through the Portty UI (expected: possible, documented as inherent).

</specifics>

<deferred>
## Deferred Ideas

- A share timer or auto-stop after idle (Stop and GUI liveness cover v2.0).
- Several concurrent click-to-share links, or per-guest links.
- A persistent allowlist of trusted colleagues who skip Admit.
- Admitting from the host's iPhone or home Mac through a remote API.
- Guest audio (Phase 23 starts it muted for everyone) and per-guest quality.
- Guests outside the tailnet or from shared-in nodes (REQUIREMENTS Out of Scope).
- Showing a guest share in other Macs' sidebars (Phase 21 discovery is own-account only).

</deferred>
