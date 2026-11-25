# Phase 22: Click-to-Share for Tailnet People - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-11-25
**Phase:** 22-click-to-share-for-tailnet-people
**Areas discussed:** Guest link and token, Who may join and how they are identified, Admit grants and stop, Audit log, Control hand-off (DCTL-04)

**Mode:** autonomous smart discuss. The user delegated on 2026-11-25 ("I don't need to answer a bunch of questions"). Reversible technical choices were decided autonomously, product and UX choices went to Claude's Discretion, and only hard-to-reverse questions were presented, each with a recommendation. The user replied "Accept all" for all seven.

---

## Guest link and token

Q1: What is the guest link URL?

| Option | Description | Selected |
|--------|-------------|----------|
| `/desktop/guest/?cap=<token>`, signalling `/desktop/guest/ws` | Route table carries the tailnet-member gate; same-user gate on /desktop/ never depends on token parsing; no share ID in path (costly) | ✓ |
| Same `/desktop/?cap=` path, token mode selects the gate | One route, but the gate decision moves inside token parsing | |
| New top-level `/share/?cap=` family | Clean split, but breaks the 19-CONTEXT route-family plan | |

Q2: How is the guest token keyed and what does it claim?

| Option | Description | Selected |
|--------|-------------|----------|
| Own HKDF info `portty/desktop-guest-token/v1` | kind desktop-guest, share_id, iat, v 1, no exp; cross-mode use fails the HMAC by construction (one-way) | ✓ |
| Phase 19 desktop key plus a mode "share" claim | One key, but separation rests on a mode check never being forgotten | |
| Add an exp claim (e.g. 12 h) | Belt-and-braces, but a second expiry rule beside Stop that the UI must explain | |

**User's choice:** Accept all (recommended options)

---

## Who may join and how they are identified

Q3: Who may open a guest link?

| Option | Description | Selected |
|--------|-------------|----------|
| Untagged user nodes of this tailnet, not shared in, non-zero user ID; fail closed | New requireTailnetPeer(Page) on guest routes only; route-table test pins all other routes to same-user (costly) | ✓ |
| Also accept shared-in nodes | Lets outside people in; contradicts tailnet-only | |
| Also accept tagged nodes | A server or CI box could join with no person behind the prompt | |

Q4: What identity does the Admit prompt show, and what is an admission keyed to?

| Option | Description | Selected |
|--------|-------------|----------|
| Display name, login name, device name; keyed to (share_id, user ID, stable node ID), bound to the signalling WebSocket | Same-node reconnect re-admitted with grants; second device prompts again; names only on the shared Mac (costly) | ✓ |
| Key by user ID only | Fewer prompts, but a second device gets in silently | |
| Prompt on every connection | Strictest, but every network blip re-prompts and drops grants | |

**User's choice:** Accept all (recommended options)

---

## Admit, grants and stop

Q5: Who may admit and grant, and must Portty's menu-bar process be running?

| Option | Description | Selected |
|--------|-------------|----------|
| Only the Portty GUI on the shared Mac; the share ends if the GUI process goes away | No remote admit API; never watched without the indicator on screen (costly) | ✓ |
| Also a same-user peer admit API | Admit from iPhone or home Mac, but a new write API on the share server | |
| Daemon-only admission via actionable notifications | Works with the GUI closed, but people could watch with no indicator | |

**User's choice:** Accept all (recommended option)

---

## Audit log

Q6: What is the audit log's format, place, scope and retention?

| Option | Description | Selected |
|--------|-------------|----------|
| JSON Lines `desktop-audit.jsonl` (0600) in the config dir, v 1 per line | Share, admit, join, leave, revoke and grant events incl. own-account viewers; no tokens, input or clipboard; ~5 MB rotation (costly) | ✓ |
| Plain text lines inside daemon.log | No new file, but mixed with debug noise and rotated away fast | |
| macOS unified log (os_log) | Searchable in Console, but retention is out of Portty's control | |

**User's choice:** Accept all (recommended option)

---

## Control hand-off (DCTL-04)

Q7: How does Portty tell physical input from remote input?

| Option | Description | Selected |
|--------|-------------|----------|
| No new permission: tagged injected events plus HID idle time, or an Accessibility event tap ignoring tagged events | Remote input dropped within ~1 s of physical input; research proves the method in the VM first (costly) | ✓ |
| Add Input Monitoring and a listen-only event tap | Well-trodden, but a third permission and another trip to the office Mac | |
| Lease only, no physical detection | No permission, but fails "physical input always wins" | |

**User's choice:** Accept all (recommended option)

---

## Decided autonomously

[auto] [Gate] — Q: "Guest route gate order?" → Selected: "Page: cspHeaders, requireTailnetPeerPage, guest-token page check; WebSocket: requireAllowedOrigin, requireTailnetPeer, guest-token check, then the admission wait" (reversible technical — decided autonomously: middleware composition, local)
[auto] [Gate] — Q: "Where does requireTailnetPeer live?" → Selected: "whois.go beside sameUser, reusing sameUserStatus's fail-closed shape; sameUser untouched" (reversible technical — decided autonomously: one function, local)
[auto] [Gate] — Q: "How is the narrowness of the widening proven?" → Selected: "Route-table test of every route's gate; other-user fake WhoIs gets 403 everywhere but the guest routes" (reversible technical — decided autonomously: tests only)
[auto] [Tokens] — Q: "Which cross-token cases are tested?" → Selected: "Guest token refused on /desktop/, /sessions/*, /api/sessions*, ports; always, session and port tokens refused on /desktop/guest/*; stopped-share and pre-regenerate tokens refused" (reversible technical — decided autonomously: tests only)
[auto] [Grants] — Q: "Where do guest shares live?" → Selected: "Daemon memory only; a restart or upgrade ends the share" (reversible technical — decided autonomously: narrower than persisting; persistence could be added later)
[auto] [Grants] — Q: "How many click-to-shares at once?" → Selected: "One active share; a new Share after Stop mints a new share_id" (reversible technical — decided autonomously: runtime policy)
[auto] [Revoke] — Q: "How do Stop and revoke cut connections?" → Selected: "Clear the grant, then close PeerConnection and WebSocket, releasing held keys (unshareLocked order, revokeSignal pattern)" (reversible technical — decided autonomously: local ordering)
[auto] [Media] — Q: "How is media bound to the admitted identity?" → Selected: "PeerConnection only after admission; selected ICE pair remote IP must equal the WebSocket's tailnet RemoteAddr" (reversible technical — decided autonomously: server-side check)
[auto] [Admit] — Q: "Does the host's own account skip Admit on a guest link?" → Selected: "No, same rules for everyone on a guest link" (reversible technical — decided autonomously: narrower and simpler; easy to relax)
[auto] [Arbiter] — Q: "How is the control lease implemented?" → Selected: "Daemon-side arbiter, 2 s after last key, click or scroll; changes release held keys; constants" (reversible technical — decided autonomously: runtime policy)
[auto] [Arbiter] — Q: "Does the lease apply to own-account viewers too?" → Selected: "Yes, one lease for all controlling viewers" (reversible technical — decided autonomously: runtime policy)
[auto] [Signalling] — Q: "How do viewers learn their state?" → Selected: "New versioned admission, grant and lease-holder messages" (reversible technical — decided autonomously: additive to the Phase 19 protocol)
[auto] [GUI] — Q: "How does the GUI hear about guests?" → Selected: "Daemon socket endpoints plus a GUI liveness subscription" (reversible technical — decided autonomously: internal API)
[auto] [Notifications] — Q: "Who posts join and leave notifications?" → Selected: "The GUI via sendNotificationFunc" (reversible technical — decided autonomously: internal routing)
[auto] [Audit] — Q: "How is the audit log written?" → Selected: "One daemon appender, 0600 O_APPEND, rotation copied from daemon_log.go; failures never block a join" (reversible technical — decided autonomously: implementation behind the accepted format)
[auto] [Testing] — Q: "What test approach?" → Selected: "Fake WhoIs identities, loopback integration tests, Vitest, VM rows for physical input, a real two-account M-NN row, gsd-secure-phase review, TESTING.md updates" (reversible technical — decided autonomously: test layout)

## Claude's Discretion

User delegated: 'I don't need to answer a bunch of questions' (2026-11-25). Entry points (tray item and Settings button opening one sheet), CT acknowledgement reuse, Admit sheet layout with no default button, viewer list rows, tray indicator, Stop semantics (guests only; always-available unchanged), coalesced join and leave notifications, control-grant warning copy, guest page states, pending-admission timeout and caps, no re-knock after deny or revoke. Details and reasons in CONTEXT.md.

## Deferred Ideas

- Share timer or idle auto-stop
- Several concurrent or per-guest links
- Persistent allowlist that skips Admit
- Remote admit from iPhone or home Mac
- Guest audio and per-guest quality (Phase 23)
- Guests outside the tailnet or from shared-in nodes (out of scope)
- Guest shares in other Macs' sidebars
