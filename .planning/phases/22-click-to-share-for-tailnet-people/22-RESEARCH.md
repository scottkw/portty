# Phase 22: Click-to-Share for Tailnet People - Research

**Researched:** 2026-11-25
**Domain:** Tailnet-identity-gated desktop sharing (admit, per-person grants, revoke, audit) on the Phase 19 share server, plus physical-vs-remote input arbitration on macOS 27.2
**Confidence:** MEDIUM-HIGH (the key physical-input question is answered with VM evidence for the *injected* half only; the *physical* half could not be generated and needs a short user-run check, see Open Questions)

## Findings that change CONTEXT decisions or create risk (ranked)

1. **Physical-input detection works with no new permission, but only through an event tap, not through HID idle time.** In the macOS 27.2 VM a listen-only `CGEventTap` created under Accessibility alone (no Input Monitoring row anywhere in TCC) received every injected key, mouse and scroll event, and the Portty tag in `kCGEventSourceUserData` arrived intact. Apple DTS confirms "Accessibility basically grants event posting and listening". HID idle time is **out**: every injected event I posted (all three source states, all three post locations) reset both `CGEventSourceSecondsSinceLastEventType` and IOKit `HIDIdleTime`, so idle cannot tell remote from physical. CONTEXT listed idle first and the tap second; **reverse that order: the tap is the mechanism.** Decision recorded in Standard Stack / Pattern 1.
2. **The "physical" half is unproven.** No true physical stand-in was possible: the two running macOS VMs have no VNC/graphics and Apple's two-macOS-VM limit blocked a third (`tart run` refused). So the VM proves "tagged injected events are seen and identifiable" and "untagged events are visible with a different signature", not "a real finger on the real trackpad shows up as untagged". Residual risk is small (an untagged-event rule fails safe) but a 2-minute user-run check on a real Mac is proposed (Appendix B, Open Question 1).
3. **Keyboard blind spot during Secure Event Input (measured).** While any process holds Secure Event Input (Terminal "Secure Keyboard Entry", password fields, the login window) the tap received **zero** keyDown/keyUp events but still got flagsChanged, mouse and scroll. Physical typing into a password field or a secure Terminal is therefore invisible to the tap, so "physical always wins" is degraded for the *keyboard only* in exactly the situations where it matters most. Needs a user decision (Open Question 2); a mitigation (secure-input detect plus HID key-idle comparison) is described and is unverified for physical keys.
4. **Use a listen-only tap, never an active (default) tap.** An Apple forum report (macOS 27 "Golden Gate" included) shows an *active* tap hangs system-wide input when Accessibility is revoked. Listen-only is also what the VM proved works. Also: when Accessibility is missing a listen-only tap is created **non-NULL but disabled** (`created=YES`, `enabled=0` in the VM), so the code must check `CGEventTapIsEnabled`, not just the return value.
5. **The guest page cannot load its own JavaScript if the bundle stays behind the same-user gate.** `/app/` (the whole React bundle, including its hashed `assets/`) sits behind `requireSameUserPage`. A guest from another account would get the refusal page for every chunk. The Phase 19 `/desktop/` route family must serve its assets behind its own gate, and the guest family must serve them behind `requireTailnetPeer` (or from an ungated path like `/assets/`). This route is not in CONTEXT's route list; add `/desktop/guest/assets/...` (or equivalent) to the route table and its pinning test.
6. **Join/leave banners may be suppressed while the display is being captured.** The 27.2 Notifications pane shows "when mirroring or sharing the display: Notifications Off" (screenshot in the VM). Whether a ScreenCaptureKit stream by the daemon engages it is **not measured**. Treat banners as best effort; the in-window Admit sheet, the tray icon variant and the tray "shared with N" line are the load-bearing signals. Add a VM row that measures it.
7. **Stop and revoke must not close WebSockets one after another.** `coder/websocket` `Close` waits up to 5 s to write and 5 s for the peer's reply. Three unresponsive guests closed in sequence is 30 s, not "at once". Flip viewer state first (drop input, stop media), then close all peers concurrently with `CloseNow` as the backstop. Also, existing code notes hijacked WebSockets survive `ws.Stop()`, so every guest connection must be tracked and closed explicitly.
8. **Bind media to the admitted person by node identity, not by IP string equality.** The selected ICE pair's remote address and the WebSocket's `RemoteAddr` can be different tailnet addresses of the same node (IPv4 vs IPv6). Run `WhoIs` on the pair's remote address and compare numeric user ID plus stable node ID. The stronger real binding is the DTLS certificate fingerprint pinned from the guest's offer; the IP check is defence in depth.
9. **CONTEXT's "single source of truth" tray claim is stale.** `BuildMenuItems` in `tray_common.go` is referenced only by tests; the live menu is built in `tray_objc_darwin.m` `menuWillOpen`. The indicator line, Stop item and icon variant must be added in the ObjC file (and `BuildMenuItems` either updated to match or left alone knowingly).
10. **Smaller corrections.** (a) `daemon.log` rolls at 10 MiB, not "about 5 MB" (`daemonLogMax = 10 << 20`); pick the audit cap deliberately. (b) The existing WhoIs test fake returns no `UserProfile` and answers every caller with one identity, so it cannot test the Admit prompt or two simultaneous guests; it needs extending. (c) Deny/revoke should be keyed on numeric **user ID**, while admission stays keyed on (user ID, stable node ID); otherwise a refused person re-knocks from a second device. (d) A physical takeover must release everything the remote viewer holds, or a dropped mouse-up leaves a button stuck. (e) Display and login names are attacker-influenced strings: strip bidi/format/control characters and cap length before showing or logging. (f) Phases 19, 20 and 21 are not built; section "Upstream contract checks" lists what Phase 22 plans must re-verify at plan time.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

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

### Deferred Ideas (OUT OF SCOPE)
- A share timer or auto-stop after idle (Stop and GUI liveness cover v2.0).
- Several concurrent click-to-share links, or per-guest links.
- A persistent allowlist of trusted colleagues who skip Admit.
- Admitting from the host's iPhone or home Mac through a remote API.
- Guest audio (Phase 23 starts it muted for everyone) and per-guest quality.
- Guests outside the tailnet or from shared-in nodes (REQUIREMENTS Out of Scope).
- Showing a guest share in other Macs' sidebars (Phase 21 discovery is own-account only).
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DSHR-03 | Share Desktop creates a link other tailnet people can open; terminal links stay limited to the host's own account | `requireTailnetPeer(Page)` modelled on `sameUser` (Pattern 2); route-table pin test; guest asset route (Finding 5); WhoIs field facts (Standard Stack); two-account real-tailnet M-NN row |
| DSHR-04 | Each guest waits at an Admit prompt showing tailnet identity; view-only until the sharer allows control (and clipboard) per person | Admission registry keyed (share_id, user ID, stable node ID) (Pattern 3); WhoIs name fields and sanitization (Pitfall 7); per-viewer grant gate in front of Phase 20's single input entry point (Pattern 5); Admit sheet click-guard (Pitfall 9) |
| DSHR-05 | Stop the share or revoke one viewer at any time, ending the connection immediately; links die with the share | Revoke ordering and concurrent close (Pattern 4, Finding 7); `coder/websocket` Close semantics verified; pion `Close`; token dies on grant clear (token design) |
| DSHR-06 | Join/leave notifications and persistent "shared with N" indicator with one-click stop | `tray_objc_darwin.m` menu and icon mechanics (Pattern 6); notification seam and click routing; banner suppression risk (Finding 6); GUI liveness stream design |
| DSHR-07 | Joins, leaves and control grants written to a local audit log | JSON Lines appender, rotation, sanitization, event ordering, `reason` field recommendation (Pattern 7); verified config dir path |
| DCTL-04 | Several controllers: last typer holds ~2 s; physical input at the Mac always wins | VM-proven tap mechanism and tag, idle-time rejection, secure-input limit (Findings 1-4, Appendix A); lease arbiter state machine (Pattern 5); physical takeover release-all (Pitfall 6) |
</phase_requirements>

## Summary

Phase 22 is mostly orchestration on top of Phase 19-20 pieces that do not exist yet (`internal/desktop`, `internal/screencap`, the desktop token, the single input entry point). The new code is a small set of well-bounded units: one new WhoIs gate (`requireTailnetPeer`) next to `sameUser`, a guest token family under its own HKDF key, an in-memory share registry with an admission queue and per-viewer grants, a daemon-side input arbiter (grant gate plus the 2 s lease plus physical-wins), a JSON Lines audit appender, GUI-facing daemon socket endpoints with a liveness stream, and tray/sheet UI. Every one of them has a close precedent in the repo (`sameUser`, `porttoken.go`, `unshareLocked`/`revokeSignal`, `daemon_log.go`, `setTrayUpdateItem`), so the plan can copy shapes rather than invent them.

The one genuinely open technical question, how to tell physical input from injected input without a new permission, is answered for macOS 27.2: a listen-only event tap under the Accessibility grant Phase 19 already asks for sees injected events with their tag, and HID idle time cannot discriminate. Two limits were measured and must reach the plan: the tap is blind to keyDown/keyUp during Secure Event Input, and the physical half could not be driven in the VM, so a short user-run check on a real Mac is proposed.

The security-relevant design choices that CONTEXT leaves open are resolved here: identity is established per TCP connection by `WhoIs` on `RemoteAddr` and bound to the admission record (not re-derived from the token); Stop/revoke flip server-side state first and close peers concurrently; media is bound by DTLS fingerprint plus a node-identity (not string-IP) check on the selected ICE pair; the guest asset route needs its own gate; and the review matrix (admit, deny, revoke mid-input, Stop mid-frame, cross-token, wrong identity) is enumerated below for `gsd-secure-phase`.

**Primary recommendation:** Build the arbiter around a listen-only `CGEventTap` that records only "last untagged event time" (never key codes), keep HID idle out of the design, extend the WhoIs test fake before writing any gate code, and add the guest asset route to the route table in the same plan as `requireTailnetPeer`.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Guest identity (who is connecting) | Share server (Go, `internal/webserver`) | System tailscaled LocalAPI | `WhoIs` on the connection's `RemoteAddr`; fail closed; no tsnet |
| Admit queue, admissions, per-viewer grants | Daemon (`internal/desktop`, in memory) | GUI (decision UI only) | The daemon owns the share lifetime and must act with the GUI gone; the GUI only decides |
| Admit prompt, viewer list, Stop, indicator | GUI (Wails/React + ObjC tray) | Daemon socket API | Indicator must be on screen while anyone watches; GUI liveness ties the share to it |
| Revoke / Stop (cut connections) | Daemon | Share server (WS), pion (PeerConnection) | Clear state first, then close transports concurrently |
| Input gating (admission, control grant, clipboard grant, lease, physical-wins) | Daemon input path (`internal/desktop` arbiter) | `internal/screencap` tap thread | One entry point from Phase 20; arbiter is pure Go with injected clock and physical-source |
| Physical-input detection | `internal/screencap` (ObjC tap thread, atomic timestamp) | Go arbiter reads it | Tap callback must stay in C/ObjC and touch only an atomic; no per-event cgo callback |
| Audit log | Daemon (`internal/desktop` appender) | Config dir (0700) | Must record even with the GUI gone; local only |
| Guest page and states | Browser (React `DesktopView` guest mode) | Share server serves page and assets | Same viewer as Phase 19; states pushed over signalling |
| Join/leave notifications | GUI (`sendNotificationFunc`) | Daemon events stream | Existing single native notification path |

## Standard Stack

### Core

No new Go modules and no new npm packages. Everything below is already a dependency or a system framework. [VERIFIED: go.mod lines `go 1.26.4`, `github.com/coder/websocket v1.8.15`, `tailscale.com v1.98.3`]

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `tailscale.com/client/local` + `apitype` | v1.98.3 (in go.mod) | `WhoIs`, `StatusWithoutPeers` | Already the identity seam (`whoIsClient`); `WhoIsResponse{Node, UserProfile, CapMap}` carries everything the prompt needs |
| `github.com/coder/websocket` | v1.8.15 | Guest signalling socket | Existing; `Close` = 5 s write + 5 s reply, `CloseNow` = immediate |
| `github.com/pion/webrtc/v4` | v4.2.29 (pinned by Phase 19, not yet in root go.mod) | Per-viewer PeerConnection | `pc.SCTP().Transport().ICETransport().GetSelectedCandidatePair()` and `OnSelectedCandidatePairChange` exist |
| CoreGraphics / ApplicationServices (cgo, ObjC) | system | `CGEventTapCreate` listen-only tap, `CGEventGetIntegerValueField(kCGEventSourceUserData)` | Proven in VM under Accessibility only |
| `encoding/json`, `os`, `sync`, `crypto/hkdf` | stdlib | Audit JSONL, files, HKDF guest key | Same as `porttoken.go` / `daemon_log.go` |
| `IsSecureEventInputEnabled` (Carbon HIToolbox) or `CGSessionCopyCurrentDictionary` | system | Detect Secure Event Input | `IsSecureEventInputEnabled` verified in VM (0 to 1 to 0); the dictionary key route is an unverified alternative (Assumptions A3) |

[VERIFIED: tailcfg.UserProfile in ~/go/pkg/mod/tailscale.com@v1.98.3/tailcfg/tailcfg.go:285-297] `type UserProfile struct { ID UserID; LoginName string // "alice@smith.com"; for display purposes only (provider is not listed); DisplayName string // "Alice Smith"; ... }`

[VERIFIED: tailcfg.go `func (n *Node) IsTagged() bool { return len(n.Tags) > 0 }` at line 594-596; Node fields `ID NodeID`, `StableID StableNodeID`, `User UserID`, `Sharer UserID`, `Tags []string`, `ComputedName string` at lines 349-479]

[VERIFIED: pion/webrtc v4.2.29 icetransport.go:47-69 `func (t *ICETransport) GetSelectedCandidatePair() (*ICECandidatePair, error)`; icetransport.go:280 `func (t *ICETransport) OnSelectedCandidatePairChange(f func(*ICECandidatePair))`; dtlstransport.go:137 `func (t *DTLSTransport) ICETransport() *ICETransport`; peerconnection.go:3625 `func (pc *PeerConnection) SCTP() *SCTPTransport`; sctptransport.go:114 `func (r *SCTPTransport) Transport() *DTLSTransport`; icecandidatepair.go:9-13 `type ICECandidatePair struct { statsID string; Local *ICECandidate; Remote *ICECandidate }`]

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `webservertest` (internal, test-only) | repo | WhoIs fake | Extend (per-address identity, `UserProfile`, `Node.ID/StableID/ComputedName`) before gate tests |
| Vitest | ^4.1.0 (frontend/package.json) | Admit sheet, guest states | Frontend unit tests; `pnpm --dir frontend vitest run <file>` |
| Playwright | existing (10 specs) | Guest page e2e | Optional: guest page refusal and states |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Listen-only tap (recommended) | HID idle time | Rejected by measurement: injected events reset it (Appendix A.2) |
| Listen-only tap | Active (default) tap | Needs nothing more, but can hang all system input if Accessibility is revoked; no benefit here |
| Listen-only tap | `NSEvent addGlobalMonitorForEvents` | Same Input-Monitoring family, no tag access advantage, and cannot see events in some secure contexts; not measured, not recommended |
| Own tap | IOHIDManager input value callbacks | Needs Input Monitoring (`IOHIDRequestAccess`) per Apple DTS; adds exactly the permission CONTEXT rules out |
| Tag-only classification (recommended) | tag AND `sourceUnixProcessID == 0` | PID==0 for hardware is unverified (the in-VM IOHIDPostEvent stand-in carried a non-zero PID); if wrong, physical would never be detected (fail-open). Tag-only fails safe |

**Installation:** none (no new packages). New cgo link flags only: `-framework Carbon` if `IsSecureEventInputEnabled` is used; CoreGraphics/ApplicationServices are already needed by Phase 20.

**Version verification:** no packages added; module versions read from `go.mod` and `~/go/pkg/mod`. Phase 19 owns adding pion v4.2.29.

## Package Legitimacy Audit

No external packages are installed by this phase. Nothing to audit.

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
Guest browser (other tailnet user)                    Shared Mac
   |  https://<mac>.<tailnet>.ts.net:7444/desktop/guest/?cap=<tok>
   v
 tailnet (WireGuard; source IP = guest node's tailnet IP)
   v
 Share server listener (tailnet IP only, TLS)
   |-- GET /desktop/guest/            -> cspHeaders -> requireTailnetPeerPage -> guest-token page check -> static guest page (no-store, no-referrer)
   |-- GET /desktop/guest/assets/...  -> requireTailnetPeer(Page) (NEW, Finding 5)
   |-- GET /desktop/guest/ws          -> requireAllowedOrigin -> requireTailnetPeer -> guest-token check
   |                                       |  WhoIs(r.RemoteAddr): not tagged, Sharer==0, User!=0, UserProfile!=nil, else 403/503
   |                                       v
   |                              Admission registry (daemon, in memory)
   |                                key = (share_id, user ID, stable node ID); deny-list by user ID
   |                                state: pending -> admitted | denied   (5 min timeout, 1 per user+node, 8 per share)
   |                                       |  admit_request event ---------------+
   |                                       v                                     |
   |                              (wait; no PeerConnection yet)                  |
   |                                                                             v
   |                                                         GUI liveness+events stream (daemon unix socket)
   |                                                            |  sheet: name, login name, device; Deny / Admit
   |                                                            |  viewer list: Allow control / Allow clipboard / Disconnect
   |                                                            |  tray: icon variant, "Desktop shared with N", Stop Sharing
   |                                                            |  notifications via sendNotificationFunc
   |                                                            |  stream EOF => daemon ends share (GUI gone)
   |                                                            v
   |                                              Admit/Deny/Grant/Revoke/Stop calls -> daemon
   |
   |   after Admit: offer/answer over the same WebSocket -> per-viewer PeerConnection (ICE-lite, UDP4, tailnet IP)
   |       media: shared encoder track  ;  data channel `input` (Phase 20)
   |       selected ICE pair remote -> WhoIs -> must equal admitted (user ID, stable node ID) else pc.Close()
   v
 Phase 20 single input entry point  Input(viewerID, msg)
   |-> ARBITER (NEW): admitted? -> control grant? (clipboard grant for clip msgs) -> lock gate (Phase 20)
   |                  -> physical-wins (tap timestamp, 1 s) -> lease (2 s, shared by all controllers)
   |                  -> release-all on any lease/grant/physical change
   v
 CGEventPost (tagged, private source)  -----------------+
                                                         v
 Listen-only CGEventTap (session, Accessibility only): untagged event => lastPhysical = now (atomic)
 Audit appender (daemon): desktop-audit.jsonl 0600 O_APPEND, one line per event
```

### Recommended Project Structure

```
internal/
  capability/
    desktoptoken.go        # guest token under HKDF info portty/desktop-guest-token/v1 (mirror porttoken.go)
  webserver/
    whois.go               # + requireTailnetPeer, requireTailnetPeerPage, tailnetPeerStatus, peerIdentity()
    server.go              # + /desktop/guest/ page, /desktop/guest/assets/, /desktop/guest/ws (Phase 19 owns /desktop/)
    webservertest/whois.go # extend fake: UserProfile, Node.ID/StableID/ComputedName, identity per remote address
  desktop/                 # (Phase 19 creates the package)
    share.go               # guest share registry: share_id, token, one active share
    admission.go           # queue, limits, timeout, deny-list, re-admit on reconnect
    arbiter.go             # grant gate + lease + physical-wins; injected clock and PhysicalSource
    audit.go               # JSONL appender + rotation + sanitize
    revoke.go              # ordered, concurrent teardown
  screencap/
    inputtap_darwin.m      # listen-only tap thread, atomic last-untagged timestamp, secure-input probe
  daemon/
    api.go                 # + /desktop/share endpoints and the GUI liveness/events stream
app.go                     # + bindings; join/leave notifications; click routing for new notification IDs
tray_objc_darwin.m, tray.go  # indicator line, Stop item, icon variant (menu is built in menuWillOpen)
frontend/src/components/   # ShareDesktopSheet, AdmitSheet, ViewerList; DesktopView guest states
```

### Pattern 1: Physical-vs-remote detection (VM-proven mechanism)

**What:** Remote events are posted by Portty from its own private `CGEventSource` with a tag in `kCGEventSourceUserData` (Phase 20 owns the posting; the tap only needs the same marker constant). A **listen-only** session tap runs on its own thread; its callback does one thing: if the event's `kCGEventSourceUserData` != Portty's marker, store `mach_absolute_time()` into a global atomic. Go asks "was there untagged input in the last 1 s?" through one cheap cgo call (or reads the atomic through a tiny C accessor). The callback never reads key codes and never logs.

**When to use:** Create the tap only while at least one remote viewer holds control (guest with a grant, or an own-account viewer), destroy it after the last controller leaves. A tap on every keystroke is a keylogger-shaped capability; keeping it scoped is cheap privacy hygiene.

**Evidence (all macOS 27.2 build 26B123 VM `portty-test`, see Appendix A):**
- Accessibility only (row for the probe's own path/identity; no ListenEvent row in TCC, no Input Monitoring prompt): `AXIsProcessTrusted=1 CGPreflightListenEventAccess=1 CGPreflightPostEventAccess=1`, tap `created=YES enabled=1`, events delivered.
- No Accessibility: `AX=0 Listen=0`, listen tap `created=YES enabled=0` (silent), active tap `created=NULL`.
- tccd shows WindowServer asking for `kTCCServiceListenEvent` for the probe and getting `authValue=2` while the DB holds only an Accessibility row.
- Tag survives for every poster variant: private source with `CGEventSourceSetUserData`, `CGEventSetIntegerValueField(kCGEventSourceUserData)` on the event, hidsrc/combsrc/private sources, post locations hid/session/annotated, key/mouse/scroll/unicode/modifier events, key hold 1.5 s (no autorepeat events appear), drag and mouse-up. No untagged extra events were produced by any tagged post.
- Taps at HID, session and annotated-session locations all worked, listen and active, under Accessibility only.

**Example (verified shape; trimmed from the probe that ran in the VM):**
```c
// Source: VM probe p22probe (Appendix A.1), run in macOS 27.2 VM; Apple DTS thread 828052 for the permission model
static volatile uint64_t g_last_untagged;          // mach_absolute_time of last non-Portty event
static CGEventRef cb(CGEventTapProxy p, CGEventType t, CGEventRef e, void *ud) {
    if (t == kCGEventTapDisabledByTimeout) { CGEventTapEnable(g_tap, true); return e; }
    if (t == kCGEventTapDisabledByUserInput) { g_dead = 1; return e; }       // recreate on next poll
    if (CGEventGetIntegerValueField(e, kCGEventSourceUserData) != PORTTY_MARKER)
        __atomic_store_n(&g_last_untagged, mach_absolute_time(), __ATOMIC_RELAXED);
    return e;                                       // listen-only: never modified, never dropped
}
// create: CGEventTapCreate(kCGSessionEventTap, kCGHeadInsertEventTap, kCGEventTapOptionListenOnly, mask, cb, NULL)
// ALWAYS check CGEventTapIsEnabled(tap) after enabling: non-NULL does not mean enabled.
```

**Mask:** keyDown, keyUp, flagsChanged, mouseMoved, left/right/other button down and up, left/right/other dragged, scrollWheel. (My probe's mask omitted dragged types; Portty's must include them.)

### Pattern 2: `requireTailnetPeer` (sibling of `sameUser`)

**What:** Same fail-closed shape as `sameUserStatus`, different predicate. [VERIFIED: internal/webserver/whois.go:54-68] the repo's `sameUser` predicate, quoted so the new gate mirrors it exactly:
```go
if who.Node.IsTagged() || who.Node.Sharer != 0 {
	return false
}
if self.Tags != nil && self.Tags.Len() > 0 {
	return false
}
if self.AltSharerUserID != 0 || self.UserID == 0 {
	return false
}
return who.Node.User == self.UserID
```
```go
// tailnetPeer: the caller is a person on THIS tailnet. Untagged, not shared in, user known, profile known.
// Deliberately does not compare to self.User (that is the widening). Returns the identity for the admission.
func tailnetPeer(who *apitype.WhoIsResponse, self *ipnstate.PeerStatus) (peerIdentity, bool) {
	if who == nil || who.Node == nil || who.UserProfile == nil || self == nil { return peerIdentity{}, false }
	if who.Node.IsTagged() || who.Node.Sharer != 0 || who.Node.User == 0 { return peerIdentity{}, false }
	if self.Tags != nil && self.Tags.Len() > 0 { return peerIdentity{}, false }   // a tagged self has no person
	if self.AltSharerUserID != 0 || self.UserID == 0 { return peerIdentity{}, false } // same self-sanity as sameUser
	return identityFrom(who), true
}
```
Decisions embedded: (a) nil `UserProfile` is a refusal, because the Admit prompt could not name the person (docs promise non-nil on success, the repo's fake returns nil, so test both); (b) the self-sanity checks are copied from `sameUser` so a shared-in or tagged Mac cannot host a guest share; (c) keep `ErrPeerNotFound` handling as in `sameUserStatus` ([VERIFIED: whois.go:97] `if err != nil && !errors.Is(err, local.ErrPeerNotFound) {` -> 503, anything else -> 403); (d) the `WhoIs` call is `lc.WhoIs(ctx, r.RemoteAddr)`, which looks up TCP first then UDP [VERIFIED: tailscale.com@v1.98.3/client/local/local.go:316-318].

**Identity is per connection, not per token:** the admission record stores `(share_id, user ID, stable node ID, display name, login name, node name, tailnet IP)` captured at WebSocket upgrade; later messages on that socket are attributed to it. The token only names the share.

### Pattern 3: Admission registry

- States: `pending`, `admitted`, `denied` (deny-list keyed by **user ID** for the share's lifetime, covers revoke too), plus `ended`.
- Key for an admission: `(share_id, user ID, stable node ID)`; a reconnect from the same key rebinds the grants to the new socket and **closes the old socket** (one live connection per key; otherwise two PeerConnections share one grant).
- Limits: the CONTEXT numbers (1 pending per (user, node), 8 per share, 5 min timeout) plus a recommended per-user cap of 2 pending, so one person with several devices cannot fill the 8 slots (Pitfall 8).
- No PeerConnection exists before admission; before admit the socket accepts only the knock/hello and ping messages, everything else closes the socket.
- All of this is in daemon memory (CONTEXT); a daemon restart ends the share, and the GUI must show "Sharing stopped because Portty restarted" so the sharer is not left believing the link is live.

### Pattern 4: Revoke and Stop ordering

Build on the existing order. [VERIFIED: internal/daemon/share.go:341-344] `// Order matters: grants clear before the kick so a viewer subscribing // mid-revoke fails its post-subscribe re-check, and the kick precedes Stop // because Stop closes the listener and plain connections; hijacked WebSockets // survive it.` [VERIFIED: server.go:299-308 `revokeSignal`, 313-321 `isGrantActive`].

Per viewer, in order:
1. Mark the viewer revoked in the arbiter (atomic flag) so any in-flight input message is dropped; release-all held keys/buttons.
2. Clear the admission/grant (`ClearGrants`-style; closes the revoke signal so waiters wake).
3. `pc.Close()` (stops media and the data channel).
4. Close the signalling WebSocket: send a fixed reason, then `CloseNow` after a short deadline.
5. Audit `revoked` (or `leave` with reason) and update GUI list.

For Stop: do steps 1-4 for **all** viewers concurrently (goroutine per viewer, `WaitGroup`, overall ~2 s ceiling), then stop accepting the guest routes. A guest that reconnects mid-teardown must fail its post-subscribe re-check (the `isGrantActive` pattern in `handleWSSRelay`: re-check after subscribing, before writing any byte).
[VERIFIED: coder/websocket@v1.8.15 close.go:86-99] `// It will write a WebSocket close frame with a timeout of 5s and then wait 5s for // the peer to send a close frame.` so sequential `Close` over N unresponsive peers costs up to N x 10 s.

### Pattern 5: Arbiter (grant gate, lease, physical-wins)

One pure-Go struct in front of Phase 20's single input entry point, with an injected clock and an injected `physicalSince() time.Duration`. Phase 20 already serialises posts on one input goroutine per share, so arbiter state needs no lock except grant/viewer changes arriving from the API (use a mutex or a command channel).

```go
const (
	physicalHold = 1 * time.Second // CONTEXT: remote input dropped while physical input within ~1 s
	leaseHold    = 2 * time.Second // CONTEXT/DCTL-04: holder keeps control 2 s after its last input
)

// allow decides one input message from viewer v of the given class at time now.
func (a *arbiter) allow(v viewerID, kind inputKind, now time.Time) (ok bool, releaseFirst []viewerID) {
	vw := a.viewers[v]
	if vw == nil || vw.revoked || !vw.admitted { return false, nil }
	if kind.isClipboard() { if !vw.clipboardGrant { return false, nil } } else if !vw.controlGrant { return false, nil }
	if a.physicalSince() < physicalHold {              // physical wins over everyone
		return false, a.releaseAllHeld()                // drop AND release what remote viewers hold
	}
	if a.holder != "" && a.holder != v && now.Before(a.leaseUntil) { return false, nil }
	var rel []viewerID
	if a.holder != v { rel = a.releaseHeldBy(a.holder); a.holder = v } // lease change releases the old holder's keys
	a.leaseUntil = now.Add(leaseHold)
	return true, rel
}
```
Recommended semantics beyond CONTEXT (planner to confirm, Claude's discretion):
- **What counts as input for taking and renewing the lease:** key down/up, button down/up, scroll, and pointer moves from the holder. A non-holder's plain pointer moves are dropped while a holder exists; when nobody holds the lease, the first controlling viewer's event of any kind (including a move) takes it. (CONTEXT says "key, click or scroll"; if moves cannot take the lease, two idle viewers' moves interleave and the cursor jitters.)
- **Own-account viewers are in the same lease** (CONTEXT) and have `controlGrant` implicitly true; clipboard likewise.
- **Physical takeover** sets `holder = ""`, releases everything held by every remote viewer, and pushes a "Someone is using this Mac" state to viewers so the video does not just feel dead.
- **Lease constants are not settings** (CONTEXT). Keep `physicalHold` and `leaseHold` as named constants so tests can use a fake clock.

### Pattern 6: Menu-bar indicator, tray menu, notifications

- The tray menu is rebuilt on every open from C globals in `menuWillOpen`. [VERIFIED: tray_objc_darwin.m:35-75] the delegate builds `Open Portty`, an optional update item (`trayUpdateTitle`), the session items, a separator, and `Quit Portty` directly; `BuildMenuItems` (tray_common.go:39) has no non-test caller. Add `setTrayShareState(count, active)` in the style of `setTrayUpdateItem`, and insert `Desktop shared with N` plus `Stop Sharing` under `Open Portty` when active.
- Icon variants: `trayIconBytesForState(connected)` returns error, progress quartiles or normal. [VERIFIED: tray.go:147 `// Error precedence (Pitfall #8): when connected=false, always returns` the error pair] Add a "sharing" pair and put it between error and progress in precedence; it needs two new template PNGs (18 px and 36 px).
- Notifications: one native path. [VERIFIED: app_update.go:254-260] `func (a *App) handleNotificationClick(id string) { if id != updateNotificationID { return } ...` so new notification IDs must be routed there (bring the window forward and emit an event) or clicks do nothing. [VERIFIED: tray_objc_darwin.m:102] `if ([response.actionIdentifier isEqualToString:UNNotificationDefaultActionIdentifier]) {` so only `UNNotificationDefaultActionIdentifier` is handled, so there are **no action buttons**: Admit/Deny cannot be answered from a banner, the sheet in the window is the only prompt. Unique identifiers per event avoid the "same identifier replaces" collapse; use a per-share coalescing identifier only when you want collapse.
- macOS may suppress banners while the display is shared (Finding 6). Do not rely on them for the Admit prompt.

### Pattern 7: Audit log

- Path: `~/Library/Application Support/portty/desktop-audit.jsonl`. [VERIFIED: internal/daemon/engine.go:61-69] `func daemonConfigDir() string { base, err := os.UserConfigDir() ... dir := filepath.Join(base, "portty"); _ = os.MkdirAll(dir, 0700)` (on macOS `os.UserConfigDir` is `~/Library/Application Support`).
- Writer: one goroutine or a mutex-guarded appender; `os.OpenFile(path, O_CREATE|O_WRONLY|O_APPEND, 0600)`; **one `Write` call per event** (a single marshalled line plus `\n`, well under PIPE_BUF, so concurrent appenders (old and new daemon overlapping during an upgrade takeover) cannot interleave inside a line).
- Rotation: copy the shape of `daemon_log.go` (rename to `.1`, reopen) but without the `Dup2` part. [VERIFIED: daemon_log.go:14 `const daemonLogMax = 10 << 20`] daemon.log is 10 MiB, not 5 MB; the audit cap is a free choice, recommended 5 MiB as CONTEXT says (about 17k events at roughly 300 bytes). One `.1` file means history older than about two caps is deleted; state this in the docs.
- Fields (CONTEXT): v, ts, event, share_id, mode, user_id, login_name, display_name, node_name, node_stable_id, tailnet_ip. **Recommended additive field `reason`** on `leave`, `share_stop`, `revoked`, `denied`, `control_revoked`, `clipboard_revoked` (values like `viewer_closed`, `revoked`, `share_stopped`, `gui_gone`, `daemon_exit`, `key_regenerated`, `timeout`, `physical_takeover` is NOT an event). Additive fields do not break v1 readers; decide now because the format is rated costly.
- Sanitize `login_name`/`display_name`/`node_name` before writing (and before showing): drop Unicode categories Cc and Cf (bidi overrides U+202A-202E, U+2066-2069, zero-width), collapse whitespace, cap at 128 runes. `json.Marshal` already escapes `\n`, so there is no line injection, but a raw U+202E renders reversed when someone `cat`s the file.
- Never log: token, `cap=` query, key codes, typed text, clipboard, frame data, SDP. The viewer-diag precedent logs only counts. Write failures: log to daemon.log once per minute, never block (CONTEXT). Recommended order for security events: append the audit line **before** applying `control_granted`, `admitted`, `revoked` so a crash cannot leave a grant active with no record; still never block on failure.
- Log own-account always-available `join`/`leave` with `mode: "own"` and empty `share_id` (CONTEXT).
- On graceful daemon exit emit `leave` for every viewer and `share_stop` (reason `daemon_exit`). On a crash the log will lack the leaves; document that a `share_start` without a matching `share_stop` followed by a later `share_start` means an unclean end.

Example lines (fake identities):
```json
{"v":1,"ts":"2026-11-25T22:41:07Z","event":"admit_request","share_id":"9f3c…","mode":"guest","user_id":1234,"login_name":"colleague@example.com","display_name":"Pat Colleague","node_name":"pat-laptop","node_stable_id":"nABC123CNTRL","tailnet_ip":"100.64.0.7"}
{"v":1,"ts":"2026-11-25T22:41:12Z","event":"control_granted","share_id":"9f3c…","mode":"guest","user_id":1234,"login_name":"colleague@example.com","display_name":"Pat Colleague","node_name":"pat-laptop","node_stable_id":"nABC123CNTRL","tailnet_ip":"100.64.0.7"}
```

### Pattern 8: GUI liveness and events

CONTEXT leaves the mechanism open ("a GUI liveness subscription"). The daemon API has no streaming endpoint today (routes in api.go are all request/response; `/health`, `/sessions`, `/peers/connect`, `/capability/regenerate-key` etc.). Recommended: one long-lived `GET /desktop/share/events` on the daemon unix socket carrying NDJSON (admit requests, viewer list changes, share state) with a periodic heartbeat line; **the stream's close is the liveness signal**. The daemon ends the guest share when the stream holding the "GUI" role closes (immediate, no grace: a unix-socket EOF is definitive). Only one GUI stream may hold the role. Admit/deny/grant/revoke/stop calls are ordinary POSTs. The GUI forwards events to React with `emitEvent`. Known residual: a hung (beachballing) GUI keeps the stream alive from a goroutine while the main thread cannot draw; accept and document (low).

### Anti-Patterns to Avoid

- **Deriving the admission from the token.** The token names the share; identity comes from `WhoIs` per connection.
- **Comparing `RemoteAddr` strings to the ICE remote.** Different address families for the same node; use `WhoIs` on both and compare node identity.
- **Closing guest sockets in a loop with `Close`.** Concurrent, state-first.
- **HID idle for physical detection.** Measured to be polluted by every injected event.
- **An active tap "for safety".** It can hang input system-wide when Accessibility is revoked.
- **Per-event cgo callbacks from the tap.** Atomic in C, poll from Go.
- **Rendering names into any served page or refusal.** Names go only to the GUI (CONTEXT).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Tailnet identity | Own login/session system or token-borne identity | `whoIsClient.WhoIs(ctx, r.RemoteAddr)` | Source IP is cryptographically bound by WireGuard; tokens can be forwarded |
| Guest token signing | New MAC scheme | Copy `porttoken.go` (`hkdf.Key(sha256.New, sessionKey, nil, info, 32)`, `hmac`, fixed-order claims, `ErrWrongKind`) | Separation by derived key is the repo's proven cross-token defence |
| Key separation proof | Reasoning only | Cross-token matrix test (below) | `Verify` ignores unknown fields; only separate keys make cross-use fail by construction |
| Revoke fan-out | Ad hoc channels | `revokeSignal` / `isGrantActive` pattern plus a tracked connection set | Existing TOCTOU-safe idiom |
| Log rotation | A rotation library | Copy `daemon_log.go` rename-and-reopen | One extra file, no dependency |
| Physical vs remote | Idle heuristics, key-rate heuristics | Tag + untagged-event tap | Idle is polluted (measured) |
| Sanitizing names | Regexes for "bad characters" | Drop Unicode Cc/Cf with `unicode.Is`, cap runes | Bidi and zero-width are format characters, not "unusual ASCII" |
| WebRTC peer binding | Custom DTLS checks | pion's SDP fingerprint verification plus the selected-pair node check | The offer pins the guest's certificate |

**Key insight:** every security property here is a restatement of an existing repo idiom (same-user gate, HKDF-separated token, grants cleared before kick). The risky new surface is small: the identity-to-admission binding, the arbiter, and the physical-input tap.

## Common Pitfalls

### Pitfall 1: Guest page loads no JavaScript
**What goes wrong:** A guest opens `/desktop/guest/`, gets the page, then every script/CSS request returns the same-user refusal, and they see a blank page or an error.
**Why it happens:** `/app/` and its `assets/` are behind `requireSameUserPage` [VERIFIED: server.go:691 `mux.HandleFunc("GET /app/", ws.cspHeaders(ws.requireSameUserPage(func(w http.ResponseWriter, r *http.Request) {`]; `/assets/` is ungated but serves only `web/assets` (dashboard CSS, terminal JS), not the Vite bundle [VERIFIED: server.go:655 `mux.Handle("GET /assets/", http.StripPrefix("/assets/", assetsNoStore(http.FileServerFS(assetsFS))))`].
**How to avoid:** Phase 19's `/desktop/` must carry its own asset subtree; Phase 22 adds a sibling under `/desktop/guest/` gated by `requireTailnetPeer`. Vite `base ./` means relative asset URLs resolve under the page path. Add the asset path to the route-table test with its gate.
**Warning signs:** Playwright guest-page spec sees 403s for `.js`; works only for the host's own account.

### Pitfall 2: Listen tap "created" but dead
**What goes wrong:** The daemon creates the tap, sees non-NULL, assumes physical detection works, and never detects anything.
**Why it happens:** Without Accessibility a listen-only tap is non-NULL but disabled (`created=YES enabled=0` in the VM). `AXIsProcessTrusted` also caches its first answer per process (Phase 20 finding R08-A/B).
**How to avoid:** After `CGEventTapEnable`, require `CGEventTapIsEnabled`; poll about once a second; recreate after `kCGEventTapDisabledByUserInput`; re-enable on `...ByTimeout`.
**Warning signs:** `last_untagged` never changes while someone types at the Mac.

### Pitfall 3: Secure Event Input blinds the keyboard half
**What goes wrong:** The sharer types a password or works in a Secure-Keyboard-Entry Terminal while a guest controls; the tap sees nothing, so remote typing keeps flowing into the same field.
**Why it happens:** Measured: with `EnableSecureEventInput` held by another Aqua process the tap received 0 keyDown/keyUp but still saw flagsChanged and mouse events (run A: 2 keyDown, 2 keyUp; run B: 0, 0). `kCGSSessionSecureInputPID` appeared in `IOConsoleUsers`.
**How to avoid:** Detect with `IsSecureEventInputEnabled()`; while true show "Local keyboard is not detected" in the GUI and decide the policy in Open Question 2. HID key idle still updates during secure input (measured), so a degraded comparison against Portty's own last key post is possible but only works when remote typing pauses by more than the counter latency (about 0.1-0.5 s measured in the VM, so use roughly 1 s slack).
**Warning signs:** Manual test: a physical keystroke in a secure Terminal does not take the lease.

### Pitfall 4: Warp moves can suppress local mouse events
**What goes wrong:** Phase 20 (decided) warps the pointer with `CGWarpMouseCursorPosition`. macOS suppresses local (hardware) mouse events for a short interval after a warp (documented default 0.25 s via `CGSetLocalEventsSuppressionInterval`) [ASSUMED: training knowledge; not measurable without physical input]. Physical mouse input right after a remote move would then be invisible to the tap *and* ignored by the system.
**How to avoid:** Prefer posting tagged `mouseMoved` events (they traverse the tap and are measured to carry the tag) or set the suppression interval to 0 once. Verify in the user-run check (Appendix B: move the real mouse immediately after a remote move).

### Pitfall 5: Idle-time "fix" resurfaces
**What goes wrong:** Someone re-adds `CGEventSourceSecondsSinceLastEventType` as a cheap fallback.
**Why it fails:** Measured: HID-state any/key idle and IOKit `HIDIdleTime` all reset after tagged and untagged injected events at every post location; the reset appears 100-500 ms after the post (the counters lag), so even "compare with my own last post" has a half-second blind window and no resolution during continuous remote input.
**How to avoid:** A comment and a test that the arbiter takes `PhysicalSource` only from the tap implementation.

### Pitfall 6: Stuck keys and buttons after a physical takeover or grant change
**What goes wrong:** Remote has a button or modifier held; physical input arrives; remote events are now dropped, including the later up event, so the button stays down at the Mac.
**How to avoid:** Every transition that drops a viewer's events (physical takeover, lease change, grant revoke, revoke, Stop, lock) calls Phase 20's server-side release-all for that viewer first. Test with a held key plus each transition.

### Pitfall 7: Names as attack surface
**What goes wrong:** A colleague sets a display name that imitates the sharer or uses a right-to-left override, or a name with newlines or control codes lands in the audit log and notifications.
**How to avoid:** Sanitize (Pattern 7) for GUI, notifications and log; always show the login name next to the display name; show the device name (a subnet-routed or shared machine looks different); the numeric user ID goes to the audit log only.

### Pitfall 8: Prompt spam and squatting
**What goes wrong:** One person (or a forwarded link) fills the 8 pending slots, blocking real guests; or knocks again after a Deny.
**How to avoid:** Per-user pending cap (recommended 2), oldest-replaced; deny-list by user ID; one visible sheet at a time with "N waiting" and a pending list; entries older than 5 min vanish with an audit `denied` reason `timeout`.

### Pitfall 9: Accidental or click-through Admit
**What goes wrong:** The window comes forward just as the sharer was clicking something else, and the click lands on Admit.
**How to avoid:** No default button (CONTEXT) plus: initial focus on Deny, Esc = Deny, Admit disabled for about 600 ms after the sheet appears or the pending person changes, and Admit/Allow control require a pointer click (no Enter activation of a default). Name shown in bold, login name and device always visible (CONTEXT).

### Pitfall 10: Stop that takes ten seconds per guest
See Finding 7 and Pattern 4. Add a test: Stop with three silent peers completes in under 2 s.

### Pitfall 11: Identity is node-level, not person-level
The Admit prompt names the node's owner. Anyone using that node (or reaching the tailnet through that user's subnet router with SNAT) appears as that user. Mitigation is the visible device name and the per-person grant; this is inherent to tailnet identity. [ASSUMED: subnet-router SNAT behaviour from training; rare configuration.]

### Pitfall 12: Share ends silently on daemon restart
Guest shares live only in daemon memory (CONTEXT). A KeepAlive restart, an upgrade or a crash ends the share with guests connected; the GUI would still show "sharing" until it notices the stream drop. The GUI must treat events-stream loss as share-ended and say why.

## Code Examples

### Guest token (mirror of `porttoken.go`)
```go
// Source: internal/capability/porttoken.go (shape); info string and claim order are CONTEXT decisions
const DesktopGuestTokenKind = "desktop-guest"
const desktopGuestKeyInfo = "portty/desktop-guest-token/v1"

type DesktopGuestClaims struct {
	Kind    string `json:"kind"`     // "desktop-guest"
	ShareID string `json:"share_id"` // 128-bit random hex, names the share
	IAT     int64  `json:"iat"`
	V       int    `json:"v"`        // 1
}

func DesktopGuestKey(sessionKey []byte) ([]byte, error) {
	if len(sessionKey) == 0 { return nil, errors.New("capability: empty session key") }
	return hkdf.Key(sha256.New, sessionKey, nil, desktopGuestKeyInfo, 32)
}
// SignDesktopGuest / VerifyDesktopGuest: identical structure to SignPort / VerifyPort (HMAC over the marshalled
// payload, "b64Payload.b64Sig", ErrWrongKind when Kind != "desktop-guest" or V != 1).
```
[VERIFIED: porttoken.go:19 `const portKeyInfo = "portty/port-token/v1"` and :46 `return hkdf.Key(sha256.New, sessionKey, nil, portKeyInfo, 32)`; the same file's doc comment states why separate derived keys are needed: "Verify ignores unknown fields and the session routes' verifyCapability never reads perms: a token signed under the session key that carried sid and grant_id would authenticate on session routes."]

### Media binding check (selected pair to admitted node)
```go
// Source: pion/webrtc v4.2.29 icetransport.go (API names verified); decision logic is recommended
ice := pc.SCTP().Transport().ICETransport()
ice.OnSelectedCandidatePairChange(func(p *webrtc.ICECandidatePair) {
	if p == nil || p.Remote == nil { return }
	who, err := whois.WhoIs(ctx, net.JoinHostPort(p.Remote.Address, strconv.Itoa(int(p.Remote.Port))))
	if err != nil || !sameAdmittedNode(who, adm) { revoke(adm, "media_identity_mismatch"); pc.Close() }
})
// Also check once at the connected state (GetSelectedCandidatePair) in case the handler registered late.
```
The real binding is that pion verifies the remote DTLS certificate against the fingerprint in the guest's offer, which only the admitted WebSocket's browser holds the key for; this check closes the "someone else replays ufrag/pwd" gap in depth and keeps tests honest.

### Route table with gates (what the pinning test lists)
| Route | Gate |
|-------|------|
| `GET /desktop/guest/` (page) | `cspHeaders` > `requireTailnetPeerPage` > guest-token page check |
| `GET /desktop/guest/assets/...` | `requireTailnetPeerPage`/`requireTailnetPeer` (no token) |
| `GET /desktop/guest/ws` | `requireAllowedOrigin` > `requireTailnetPeer` > guest-token check > admission wait |
| every other existing route and all of `/desktop/` (not `/desktop/guest/`) | unchanged `requireSameUser(Page)` or open as today |
| any admit/grant/stop path on the share server | **none must exist** (CONTEXT: no remote admit API); the test asserts the mux has no such route |

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Listen-only taps need Input Monitoring | Accessibility already covers listening; Input Monitoring is the narrower grant | Apple DTS statement, forum thread 828052 (date not recorded); measured on 27.2 | No new permission for physical detection |
| Active tap for any event observation | Listen-only for observation | Forum report of system-wide input hang on revoke (thread 844416), macOS 27 | Listen-only only |
| Poll HID idle for "user present" | Event tap with provenance tag | Measured here | Idle cannot discriminate injected input |

**Deprecated/outdated:** `IOHIDPostEvent` is deprecated since macOS 11 (compiler warning in the VM build); I used it only as an in-VM stand-in for HID-originated events, not as a recommendation.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Real hardware keyboard/mouse events arrive at the tap with `kCGEventSourceUserData == 0` (untagged) | Findings 2, Pattern 1 | If hardware events somehow carried a non-zero value equal to Portty's marker (impossible by construction unless the marker is 0) detection fails; choose a non-zero marker. Otherwise low |
| A2 | `CGWarpMouseCursorPosition` suppresses local mouse events for ~0.25 s by default | Pitfall 4 | If untrue, nothing to fix; if true and Phase 20 warps, physical mouse input is partly lost right after remote moves. Verified only via the user-run check |
| A3 | `CGSessionCopyCurrentDictionary` carries `kCGSSessionSecureInputPID` (seen in `IOConsoleUsers`) and could replace the Carbon call | Standard Stack | Wasted time; fall back to `IsSecureEventInputEnabled` (verified) |
| A4 | Tailscale subnet-router SNAT makes LAN devices appear as the router's tailnet identity | Pitfall 11 | Low; rare configuration, only affects wording of docs |
| A5 | A hung GUI main thread keeps the liveness stream alive from a goroutine | Pattern 8 | Low; indicator stale during a hang |
| A6 | Approximate counter latency (0.1-0.5 s) for HID idle after injected events generalises from the VM to real hardware | Pitfall 3, 5 | Only affects the optional degraded keyboard fallback |
| A7 | macOS banners are suppressed during ScreenCaptureKit capture because of the "when mirroring or sharing the display: Notifications Off" setting | Finding 6 | Not measured; affects only how much to rely on banners |
| A8 | Tailnet ACLs on a corporate tailnet may block tcp/7444 and the media UDP port for guests; Portty cannot see why | Pitfall list, Open Questions | Guests fail with an unhelpful state; mitigated by the page failure copy and docs (CONTEXT specifics) |

## Open Questions / Decisions Needed (RESOLVED)

All nine were settled after research by the CONTEXT "After research" bullets (2026-11-25, user delegated) or by planning; each carries its resolution inline.

1. **Run the physical-input user check on a real Mac (needs the user).** RESOLVED (partial outcome): the Appendix B run on a real Mac (2026-11-25, evidence/appendix-b-real-mac.txt) showed untagged hardware keys and mouse at a listen-only tap under Accessibility only, with no Input Monitoring prompt; it did not cover the warp check or the Secure Keyboard Entry case. Those are planned as the owner-run real-Mac item "warp and Secure Keyboard Entry physical-input check" at the 22-12 checkpoint, recorded as an M-NN item by 22-13 (number assigned at run time) if deferred. CONTEXT After research: "Real-Mac follow-up: the research Appendix B 2-minute physical-input check runs on a real Mac as an M-NN item".
   - What we know: tag and untagged injected events are distinguishable in the VM; the physical half could not be generated (no VNC on `portty-test`/`portty-test-2`, third macOS VM refused).
   - What's unclear: that real hardware events are untagged and visible to a listen-only tap under Accessibility; that a warped move does not eat local events.
   - Recommendation: user runs Appendix B (about 2 minutes, Terminal temporarily granted Accessibility, then revoked). Record as M-NN. Do not block planning: the rule is tag-only, which fails safe.
2. **Keyboard blind spot during Secure Event Input: accept degraded detection, or drop remote keyboard input while secure input is on?** RESOLVED: accept and warn. CONTEXT After research: "while Secure Event Input is on, physical typing is invisible to the tap; accept the weaker keyboard half and show a GUI warning, rather than drop all remote keys" (22-05, 22-11 banner).
   - What we know: measured blind to keyDown/keyUp; mouse still seen; HID key idle still updates but cannot be told from remote posts except across pauses.
   - Recommendation: **accept and document** (remote typing into password fields is a legitimate use, and dropping it also breaks Phase 23 unlock-adjacent flows); show "Local keyboard not detected (Secure Input on)" in the GUI list while the condition holds; keep mouse physical-wins. Needs the user's nod because it weakens "always wins" for the keyboard.
3. **Lease semantics for plain pointer moves.** Recommendation in Pattern 5 (any input takes/renews; non-holder moves dropped while held). Confirm. RESOLVED: CONTEXT After research: "any controller input takes or renews the 2 s lease and others' pointer moves are dropped while it is held" (22-03, 22-05).
4. **Deny/revoke key: user ID (recommended) or (user, node).** CONTEXT says "person"; recommend user ID. RESOLVED: user ID. CONTEXT After research: "deny and revoke are keyed by user ID" (22-04).
5. **Per-user pending cap of 2 in addition to the CONTEXT limits.** Recommendation: yes. RESOLVED: yes. CONTEXT After research: "at most 2 pending Admit requests per user" (22-04).
6. **Audit `reason` field and graceful-exit leave events** (additive to the v1 format, rated costly). Recommendation: add now. RESOLVED: added. CONTEXT After research: "audit lines carry a reason field and graceful leaves are logged" (22-02, 22-06).
7. **Audit retention:** one `.1` file at 5 MiB. Recommendation: keep as CONTEXT; document that older history is dropped; consider two backups only if the user wants longer audit history. RESOLVED: one .1 file at 5 MiB as CONTEXT Audit log states ("Size-capped rotation like daemon.log (about 5 MB, one .1 file)"); planned in 22-02.
8. **Do banners survive capture? ** (A7) Plan a VM row that sends a `UNUserNotificationCenter` notification from the real app while a capture is running. RESOLVED by measurement: CONTEXT After research makes the Admit sheet, tray icon and tray line the load-bearing signals ("join/leave signal lives in the Admit sheet, tray icon and tray line, not banners"); the banner-during-capture behaviour is measured as VM row G-07 in 22-12.
9. **Guest asset route naming** is Phase 19's: plan must read Phase 19's final `/desktop/` asset layout before adding the guest sibling. RESOLVED: CONTEXT After research: "guests get their own asset route behind requireTailnetPeer"; 22-01 serves /desktop/guest/assets/ behind requireTailnetPeerPage, read against Phase 19's built /desktop/ layout.

## Upstream contract checks (re-verify at plan time; Phases 19-21 are unbuilt)

| Needed from | Contract | If missing |
|-------------|----------|-----------|
| Phase 20 | Every injected CGEvent (mouse, key, scroll, unicode, modifier) carries the marker in `kCGEventSourceUserData`, from one dedicated private `CGEventSource` | Phase 22 adds it before the tap (CONTEXT). VM shows all event kinds keep a tag set on the source or the event |
| Phase 20 | One daemon-side input entry point that knows the sending viewer | Arbiter cannot be inserted without touching the protocol |
| Phase 20 | Server-side release-all per viewer | Needed by every transition in Pitfall 6 |
| Phase 20 | Clipboard actions addressed per viewer | Needed for the clipboard grant |
| Phase 19 | `serverNeeded()` (terminal share or always-available) and the share-server rebuild | Guest share must count as "server needed"; Regenerate Signing Key must end the guest share (`remintSharesLocked` analogue) |
| Phase 19 | Signalling hello with a version field | Admission/grant/lease messages extend it additively |
| Phase 19 | Per-viewer PeerConnection after offer/answer on `/desktop/ws` | Guest creates its PC only after admit |

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| macOS VM `portty-test` (Tart) | VM probes and rows | yes | macOS 27.2 (26B123), arm64, ssh admin | none needed |
| macOS VM `portty-test-2` | Two-Mac flows | yes (not used here) | same | |
| Third macOS VM | VNC "physical" stand-in | **no** | Apple's two-macOS-VM limit: `The number of VMs exceeds the system limit (other running VMs: portty-test, portty-test-2)` | User-run check (Appendix B) |
| VNC/graphics on running VMs | Physical stand-in | **no** (`tart run --no-graphics`) | | Same |
| Compiler in VM | Probes | no (`xcode-select: note: No developer tools were found`) | | Compile on host (clang/swiftc, no run), `scp` the binary |
| Go on host | Tests | yes | go1.27.4 (go.mod says 1.26.4) | |
| Node/pnpm on host | Vitest | yes | node v24.22.1, pnpm 12.6.0 | |
| Real two-account tailnet | SC1 | not touched by research | | M-NN row |

**Missing dependencies with no fallback:** none blocks planning.
**Missing dependencies with fallback:** physical input stand-in (user-run check).

Side effects left in the VM during research (all cleaned): LaunchAgent plists for the probes removed; System Settings Accessibility rows for the probes removed through the UI; `/tmp` probe files removed; the system TCC DB read back identical to the starting state. One accidental `xcode-select` call raised the "Install Command Line Developer Tools" dialog in the VM; its process was killed. macOS keeps a few stale "App Background Activity" Login Items records for the removed probe plists.

## Validation Architecture

> `.planning/config.json` has `workflow.nyquist_validation: false`; this section is included because the orchestrator asked for it explicitly.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` (module go 1.26.4), Vitest ^4.1.0, Playwright (existing), shell tests (`tests/*.sh`) |
| Config file | none per suite (`frontend/package.json` scripts: `"test": "vitest run"`) |
| Quick run command | `go test ./internal/webserver/ ./internal/capability/ ./internal/desktop/ -run 'Guest\|Admit\|Arbiter\|Lease\|TailnetPeer\|Audit\|Revoke' -count=1` |
| Full suite command | `go test ./... -count=1` and `pnpm --dir frontend vitest run` |

### Phase Requirements to Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DSHR-03 | Other-user untagged node gets the guest page; same node gets 403+marker on `/app/`, `/sessions/*`, `/api/*`, ports, `/desktop/`; tagged, shared-in, user 0, nil profile, not-found, status fault, whois fault refused/503 on guest routes | integration (loopback, fake WhoIs) | `go test ./internal/webserver/ -run 'TailnetPeer\|GuestGate\|RouteTable'` | no, Wave 0 |
| DSHR-03 | Route-table pin: only guest routes use `requireTailnetPeer`; no admit/grant/stop route exists on the share server | unit | `go test ./internal/webserver/ -run RouteTable` | no, Wave 0 |
| DSHR-03 | Real second Tailscale account opens the link, still gets own-devices refusal on a terminal link | manual M-NN (real tailnet) | n/a | M-NN new |
| DSHR-03/04 | Cross-token matrix (below) | unit + integration | `go test ./internal/capability/ ./internal/webserver/ -run 'CrossToken\|GuestToken'` | no, Wave 0 |
| DSHR-04 | Pending guest gets no PeerConnection; admit creates it; deny/timeout/limits; reconnect same node re-admitted with grants; second device prompts; denied person (by user ID) cannot knock again | integration (synthetic frame source) | `go test ./internal/desktop/ -run Admit` | no, Wave 0 |
| DSHR-04 | View-only: pointer/key/clipboard from an admitted guest without grant does nothing; control and clipboard grants independent; grant change releases held keys | unit (fake poster) | `go test ./internal/desktop/ -run 'Arbiter\|Grant'` | no, Wave 0 |
| DSHR-05 | Revoke one viewer / Stop: grant cleared, PC and WS closed, next input dropped, Stop with 3 silent peers under 2 s, link refused after Stop, new share new `share_id`, key regenerate ends share | integration | `go test ./internal/desktop/ ./internal/webserver/ -run 'Revoke\|Stop'` | no, Wave 0 |
| DSHR-05 | Media identity mismatch closes the peer | unit with fake ICE pair | `go test ./internal/desktop/ -run MediaBinding` | no, Wave 0 |
| DSHR-06 | Viewer count and list events; tray strings; notification IDs routed; coalescing; events-stream EOF ends share | unit + app test | `go test . -run 'TrayShare\|ShareNotification\|GuiLiveness'` and `go test ./internal/daemon/ -run GuiLiveness` | no, Wave 0 |
| DSHR-06 | Indicator on screen, Stop from tray, banners during capture | VM row + manual | n/a | M-NN new |
| DSHR-07 | One valid JSON line per event, fields and mode, 0600, sanitization, rotation at cap, write failure does not block, no secrets | unit | `go test ./internal/desktop/ -run Audit` | no, Wave 0 |
| DCTL-04 | Lease: take, hold 2 s, expire, handoff releases keys, moves semantics; physical-wins: untagged within 1 s drops all remote and releases held; tagged never counts | unit with fake clock and fake `PhysicalSource` | `go test ./internal/desktop/ -run 'Lease\|Physical'` | no, Wave 0 |
| DCTL-04 | Real tap in the VM: tagged posts never register, untagged posts (the existing `~/ptest/click` tool, stand-in for local input) register, tap disabled state surfaces, secure-input flag | VM row (documented stand-in) | script under `tests/` or the VM-checks doc | no, Wave 0 |
| DCTL-04 | Real hardware untagged and visible; warp does not eat local input | manual user-run (Appendix B) | n/a | M-NN new |
| Guest UI | Admit sheet: click guard, Deny focus, Esc, name rendering, login name always shown; guest page states; viewer list switches | Vitest | `pnpm --dir frontend vitest run src/components/__tests__/AdmitSheet.test.tsx` etc. | no, Wave 0 |

**Cross-token matrix (every cell must be a test; expected result = refused with the same generic response as an invalid token):**

| Token presented | `/desktop/` page+ws | `/desktop/guest/` page+ws | `/sessions/{id}`, `/api/sessions*` | port routes |
|-----------------|---------------------|---------------------------|------------------------------------|-------------|
| guest token (live share) | refuse | **accept** (after identity gate) | refuse | refuse |
| guest token, stopped share | refuse | refuse (Link No Longer Works) | refuse | refuse |
| guest token, older share_id after a new share | refuse | refuse | refuse | refuse |
| guest token minted before Regenerate Signing Key | refuse | refuse | refuse | refuse |
| always-available desktop token | **accept** (same-user) | refuse | refuse | refuse |
| session capability | refuse | refuse | **accept** (own gates) | refuse |
| port token | refuse | refuse | refuse | **accept** (own gates) |
| no token, other-user identity | 403 marker | page after identity gate, then token check fails (generic) | 403 marker | 403 marker |

### Sampling Rate
- **Per task commit:** the quick run command for the package touched.
- **Per wave merge:** `go test ./... -count=1` plus `pnpm --dir frontend vitest run`.
- **Phase gate:** full suite green, `bash tests/check-traceability-paths.sh` exit 0, VM rows recorded, then `/gsd-secure-phase 22`.

### Wave 0 Gaps
- [ ] Extend `internal/webserver/webservertest/whois.go`: `UserProfile{ID, LoginName, DisplayName}`, `Node{ID, StableID, ComputedName, Addresses}`, per-remote-address identity map (two guests at once), constants for `OtherWithProfile`, nil-profile case. Current fake returns `&apitype.WhoIsResponse{Node: node}` with no profile and one identity for all callers. Keep `release_guard_test.go` happy (never in the release graph).
- [ ] `internal/desktop` test helpers: fake clock, fake `PhysicalSource`, fake poster (Phase 20), fake `AuditSink`, synthetic frame source (Phase 19).
- [ ] Route-table test scaffolding that enumerates the mux (or a maintained list with a completeness check).
- [ ] Tap probe script for the VM row (adapt Appendix A.1; keep out of the release build, and the standing TESTING.md rules apply).
- [ ] TESTING.md: add files to Section 2, rows to Section 4 (path column repo-relative file only), M-NN items to Section 5 (next free number after M-164 is M-165, **recheck at execution**, Phases 19-21 will add some), run `bash tests/check-traceability-paths.sh`; branch protection untouched unless `build.yml` matrix changes.

## Security Domain

> `security_enforcement` is not set to false in the visible config; treated as enabled. This phase widens who can reach a desktop and gets a recorded security review.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | Tailnet identity via `WhoIs` per connection (no passwords); guest token is a second factor naming the share only |
| V3 Session Management | yes | Admission and grants bound to the signalling WebSocket and (share_id, user ID, node ID); end on Stop, revoke, key regenerate, daemon restart |
| V4 Access Control | yes | Route-table-pinned gates; per-viewer control and clipboard grants enforced server-side in the arbiter; no remote admit API |
| V5 Input Validation | yes | Name sanitization (Cc/Cf, length); WebSocket/data-channel size limits (Phase 20 constants); fixed message schema |
| V6 Cryptography | yes | HKDF-separated HMAC token (copy `porttoken.go`), DTLS fingerprint pinning (pion), Tailscale TLS certs; never hand-roll |
| V7 Logging | yes | Audit JSONL, 0600, no tokens/input/clipboard/frames |
| V9 Communications | yes | WSS only on the share server, tailnet-IP bind, UDP4 tailnet-only ICE |
| V13 API | yes | Daemon socket endpoints are same-user local; share server exposes none of them |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Other-account node reaches terminal links | Elevation | Unchanged `requireSameUser(Page)` on every terminal, `/app/`, `/api/`, port route; route-table test |
| Tagged or shared-in node joins guest share | Spoofing | `requireTailnetPeer` refuses tagged, `Sharer != 0`, user 0, nil profile |
| Forwarded guest link used by an unwelcome person | Spoofing | Admit prompt names the person; deny-list by user ID |
| Guest token replayed on another route family | Elevation | Separate HKDF key; matrix tests |
| Guest token of a stopped or older share | Elevation | Grant cleared on Stop; `share_id` equality; key regenerate ends share |
| Media hijack by someone with ufrag/pwd | Spoofing | PC only after admit; DTLS fingerprint pinning; selected-pair node check |
| Revoked viewer keeps typing | Elevation | State flipped first; per-message check; release-all; concurrent close |
| Admit prompt click-through or spam | Tampering | Click guard, focus on Deny, per-user cap, timeout |
| Name spoofing (bidi, lookalike) | Spoofing | Sanitize; always show login name and device; audit carries numeric ID |
| Audit tampering by a controlling guest | Tampering | Out of scope: control equals full account access (CONTEXT, documented). File is 0600 and local |
| Guest with control self-grants through Portty UI or the daemon socket | Elevation | Inherent (CONTEXT); copy says control means they can do anything you can. A view-only guest has no input and cannot reach the unix socket |
| Physical detection spoofed by a local process posting untagged events | DoS (of remote input) | Fail-safe direction; acceptable |
| Tap hang on Accessibility revoke | DoS | Listen-only tap only |

### Recorded security review (for `gsd-secure-phase`): cases and expected results

1. **Admit:** guest waits at pending with no PeerConnection and no media; Admit creates the PC; view-only; control/clipboard grants separate.
2. **Deny / timeout / spam:** denied user (any device) cannot re-knock this share; pending cap and 5 min timeout; audit lines.
3. **Revoke mid-input:** held keys released, next input dropped, WS and PC closed, link still valid for others, revoked user cannot knock again.
4. **Stop with viewers mid-frame:** all viewers closed concurrently under 2 s; link refused afterwards; new share has a new `share_id`.
5. **Cross-token:** the matrix above.
6. **Wrong identity:** another user's node on terminal, `/app/`, `/api/*`, ports, `/desktop/` = 403 with marker; tagged, shared-in, user 0, nil profile on guest routes = 403; not-found and tailscaled faults = 403/503 per `sameUserStatus` shape; no page names an account.
7. **Reconnect from a different node of the same person:** prompts again (CONTEXT); from the same node: re-admitted with grants; old socket closed.
8. **Media identity mismatch:** simulated selected pair from another node closes the peer.
9. **Self-grant by a controlling guest:** possible and documented (CONTEXT); verify the copy exists.
10. **Audit:** no secret in any line; sanitized names; 0600; rotation.

## Sources

### Primary (HIGH confidence)
- VM measurements, macOS 27.2 (26B123) `portty-test`: Appendix A (own probes, run this session).
- Repo files read this session: `internal/webserver/whois.go`, `server.go` (routes at 544-737, 890-1124), `internal/daemon/share.go` (296-381), `daemon_log.go`, `engine.go:59-69`, `internal/capability/porttoken.go`, `webservertest/whois.go`, `tray.go`, `tray_objc_darwin.m`, `tray_common.go`, `notification_darwin.go`, `app_update.go:248-265`, `go.mod`, `.planning/phases/19|20|21|22-CONTEXT.md`, `17-SPIKE-MATRIX.md` (rows R08, R04 evidence).
- Go module cache: `tailscale.com@v1.98.3` (`client/tailscale/apitype`, `tailcfg`, `client/local/local.go`), `github.com/coder/websocket@v1.8.15/close.go`, `github.com/pion/webrtc/v4@v4.2.29` (`icetransport.go`, `dtlstransport.go`, `peerconnection.go`, `sctptransport.go`).

### Secondary (MEDIUM confidence)
- [CITED: https://developer.apple.com/forums/thread/828052] Apple DTS (Kevin Elliott): "Accessibility basically grants event posting and listening, while Input Monitoring only grants listening ... the behavior here is intentional: after Accessibility is granted, the tap succeeds."
- [CITED: https://developer.apple.com/forums/thread/844416] active `.defaultTap` plus Accessibility revoke hangs system input; thread does not test listen-only.
- [CITED: https://tailscale.com/kb/1068/acl-tags] tagged devices have tags, not a user owner.
- [CITED: https://pkg.go.dev/tailscale.com/client/tailscale/apitype#WhoIsResponse] Node and UserProfile never nil on success; says nothing on tagged or shared nodes.

### Tertiary (LOW confidence)
- Training knowledge flagged `[ASSUMED]` in the Assumptions Log (warp suppression interval, subnet-router SNAT, banner suppression during capture).

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH, no new dependencies; every API name checked in the module cache.
- Architecture: MEDIUM-HIGH, built on in-repo precedents, but Phases 19-21 are unbuilt so the integration seams are contracts, not code.
- Physical-input mechanism: MEDIUM-HIGH, injected half measured, authoritative Apple statement, physical half pending the user-run check.
- Pitfalls: HIGH for the measured ones (tap enabled flag, secure input, idle pollution, Close timing, asset gating), MEDIUM for the assumed ones.

**Research date:** 2026-11-25
**Valid until:** 2026-12-25 (macOS 27.x behaviour and the unbuilt upstream phases can shift sooner; recheck the Upstream contract table at plan time)

---

## Appendix A: VM evidence (all run by me, macOS 27.2 build 26B123, arm64, VM `portty-test`)

### A.0 Rig and how input was generated
- Host ran no probe, no capture, no input; probes were compiled on the host (compile only) and run only inside the VM (`ssh admin@<vm>`, `scp -i ~/.ssh/transom_demo_ed25519`). No tailnet or 100.x address was contacted.
- **"Injected" input** = `CGEventPost` from another process in the VM (an ssh-launched poster; Accessibility attributed to `sshd-keygen-wrapper`, which already has it).
- **Listener process** ran **in the Aqua session** as a LaunchAgent (`launchctl bootstrap gui/501`, `LimitLoadToSessionType Aqua`) with **its own TCC identity** (a path/ad-hoc-signed binary the System Settings UI granted Accessibility to; granting was driven by the existing `~/ptest/click` tool over ssh plus the "would like to control this Mac" prompt). `launchctl asuser` from ssh failed without root, and attribution would have ridden on `sshd-keygen-wrapper` (earlier R08-B rows were confounded the same way), which is why the LaunchAgent form was used. The first grant did not take because the linker-signed ad-hoc binary's cdhash requirement did not match (`Failed to match existing code requirement ... kTCCServiceAccessibility` in tccd); `codesign -s - -i <id>` fixed it.
- **Physical stand-in: none could be produced.** `tart run --vnc` on a clone failed: `The number of VMs exceeds the system limit (other running VMs: portty-test, portty-test-2)`; the running VMs are `--no-graphics` with no VNC. The VM has "Virtual USB Keyboard", "Virtual USB Digitizer", "Virtual Keyboard", "Virtual Trackpad" HID devices (`ioreg`), reachable only from the host side. SIP is enabled (`csrutil status`), so a virtual HID device via `IOHIDUserDevice` (restricted entitlement) and TCC.db edits (`attempt to write a readonly database`) were unavailable. Closest stand-in tried: `IOHIDPostEvent` through IOHIDSystem as admin (kr=0): the tap saw `userData=0x0 srcStateID=1 srcUserID=0 srcGroupID=0` but `srcPID` = the poster's PID, so it is not a physical stand-in and PID==0 cannot be assumed for hardware.

### A.1 Permissions (probe with its own identity, Accessibility toggled on in System Settings only)
```
before any grant (LaunchAgent): AXIsProcessTrusted=0 CGPreflightListenEventAccess=0 CGPreflightPostEventAccess=0 CGPreflightScreenCaptureAccess=0
                                tap listen: created=YES enabled=0 | tap active: created=NULL
after Accessibility only:       AXIsProcessTrusted=1 CGPreflightListenEventAccess=1 CGPreflightPostEventAccess=1 CGPreflightScreenCaptureAccess=0
                                tap listen (session/hid/annotated) created=YES enabled=1 ; tap active (session/hid) created=YES enabled=1
TCC rows after the grant: kTCCServiceAccessibility|/private/tmp/<probe>|2   (no ListenEvent row anywhere; sudo sqlite3 read of the system DB)
tccd: WindowServer -> kTCCServiceListenEvent for the probe: AUTHREQ_RESULT authValue=2 authReason=4 ; PostEvent: "Evaluated composed authorization from kTCCServicePostEvent to parent service kTCCServiceAccessibility: Auth:Allowed (System Set)"
No Input Monitoring prompt appeared.
```

### A.2 HID idle (injected posts; ssh-launched, 3 s quiet before each post; times are idle seconds)
All variants (nil source@hid, private+tag@hid, private+tag@session, private+tag@annotated, hidsrc+tag@hid, combsrc+tag@session, combsrc+tag@hid, mouse move and key F19): immediately after the post the counters had not moved; at +100 ms IOKit `HIDIdleTime` read 0.10 s in **every** variant; at +500 ms `hid(any)` and `comb(any)` read 0.5-0.9 s in every variant (and `hid(key)`/`comb(key)` after a key post). Result: injected events at any location reset both the HID-state and combined-session idle counters and IOKit HIDIdleTime, with a 0.1-0.5 s lag. Idle cannot discriminate. Also observed: after a key post under Secure Event Input `hid(key)` still reset, so idle counters keep working during secure input.

### A.3 Tag and source fields as the tap sees them (listen-only, session; selected rows)
```
poster variant                       type  userData              srcStateID   srcPID  srcUserID srcGroupID
nil source @hid, move                5     0x0                   0            <poster> 501      20
private source + source tag @hid     5     0x504f5254595f5252    random       <poster> 501      20
nil source + per-event tag @hid      5     0x504f5254595f5252    0            <poster> 501      20
private+tag key F19 down/up          10/11 0x504f5254595f5252    random       <poster> 501      20
hidsrc+tag @hid                      5     0x504f5254595f5252    1            <poster> 501      20
combsrc+tag @session                 5     0x504f5254595f5252    0            <poster> 501      20
IOHIDPostEvent (HID stand-in)        5     0x0                   1            <poster> 0        0
tagged: key hold 1.5 s               10, 11 only (no autorepeat), both tagged
tagged: drag (down, 5 dragged, up, move)  1, 2, 5 seen (dragged not in my mask) all tagged; no untagged extras
tagged: unicode string key event, Shift flagsChanged, scroll  all tagged
```
`srcStateID` is useless as a discriminator (a HID-state source yields 1, probably the same as hardware). `srcPID` is the poster's PID for every injected variant, including the IOHIDSystem stand-in. Marker used by the probe: `0x504F5254595F5252` ("PORTTY_RR"); production marker is Phase 20's choice, non-zero.

### A.4 Secure Event Input (probe process in the Aqua session called `EnableSecureEventInput()`; tap listen-only session; same five tagged posts each run)
```
A no secure input : types seen = 1 leftDown, 2 keyDown, 2 keyUp, 2 flagsChanged, 1 leftUp, 1 mouseMoved
B secure input on : types seen = 1 leftDown,                       2 flagsChanged, 1 leftUp, 1 mouseMoved
IsSecureEventInputEnabled: 0 -> 1 after Enable -> 0 after Disable ; ioreg IOConsoleUsers gained "kCGSSessionSecureInputPID"=<pid> while on
```
keyDown/keyUp disappeared; modifiers and mouse stayed. (The scroll event was not captured in either of these two runs, an unexplained miss; an earlier run did capture type 22 with its tag.)

### A.5 Other
- Notifications pane of the 27.2 VM shows `Show Notifications: when display is sleeping = off; when screen is locked = on; when mirroring or sharing the display = Notifications Off` (screenshot read during the grant flow).
- `~/ptest/click` (posts untagged `CGEventPost(kCGHIDEventTap)` events) is a ready stand-in for "local input" in later VM rows, with the honest caveat that it is software injection.

## Appendix B: proposed user-run physical check (needs the user; I did not run it on the host)

Goal: confirm on a real Mac that hardware input shows up untagged at a listen-only tap under Accessibility only, that Portty-tagged posts do not, and that a warp does not eat local input.
1. Compile (any Mac with Xcode CLT): `clang -O1 -o phystap phystap.c -framework ApplicationServices -framework CoreFoundation`
2. System Settings > Privacy & Security > Accessibility: temporarily allow the Terminal you run it from (revoke afterwards). Do not grant Input Monitoring.
3. `./phystap 20` then within 20 s: (a) move the trackpad and type in a normal app, (b) type in a Terminal with Secure Keyboard Entry on (expect no key lines), (c) from a second Terminal run any tool that posts untagged events (e.g. `osascript -e 'tell application "System Events" to key code 0'`).
4. Expected: physical lines show `userData=0x0`; the Portty posts (Phase 20 build, later) show the marker. Report whether `srcPID` is 0 for physical lines (informational only; the design does not depend on it).
```c
// phystap.c: prints one line per event: type, userData, srcPID, srcStateID. Never prints key codes.
#include <ApplicationServices/ApplicationServices.h>
#include <stdio.h>
#include <stdlib.h>
static CGEventRef cb(CGEventTapProxy p, CGEventType t, CGEventRef e, void *u) {
    if (t == kCGEventTapDisabledByTimeout || t == kCGEventTapDisabledByUserInput) { printf("TAP DISABLED %d\n", t); return e; }
    printf("type=%d userData=0x%llx srcPID=%lld srcStateID=%lld\n", t,
        (unsigned long long)CGEventGetIntegerValueField(e, kCGEventSourceUserData),
        CGEventGetIntegerValueField(e, kCGEventSourceUnixProcessID), CGEventGetIntegerValueField(e, kCGEventSourceStateID));
    fflush(stdout); return e;
}
int main(int argc, char **argv) {
    CGEventMask m = 0; for (int t = 1; t <= 13; t++) m |= CGEventMaskBit(t);   // buttons, moved, dragged, keys, flags, scroll(22) added below
    m |= CGEventMaskBit(kCGEventScrollWheel);
    CFMachPortRef tap = CGEventTapCreate(kCGSessionEventTap, kCGHeadInsertEventTap, kCGEventTapOptionListenOnly, m, cb, NULL);
    if (!tap || !CGEventTapIsEnabled(tap)) { printf("tap not enabled: grant Accessibility to this Terminal\n"); return 3; }
    CFRunLoopAddSource(CFRunLoopGetCurrent(), CFMachPortCreateRunLoopSource(NULL, tap, 0), kCFRunLoopCommonModes);
    CFRunLoopRunInMode(kCFRunLoopDefaultMode, argc > 1 ? atoi(argv[1]) : 20, false);
    return 0;
}
```
(Note: `CGEventTapIsEnabled` is checked after creation here, matching Pitfall 2; the VM probe enabled it explicitly with `CGEventTapEnable`, add that call if the tap reports disabled on your Mac.)
