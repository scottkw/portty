---
phase: "22"
slug: "click-to-share-for-tailnet-people"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-11-27"
---

# Phase 22 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

Register taken from the `<threat_model>` blocks of 22-01 through 22-13 (authored at plan time): T-22-01 to T-22-40 and the recurring T-22-SC supply-chain row. ASVS L1, block_on high. Every mitigation was confirmed at grep depth or by a named test, and the phase gate was run fresh on 2026-11-27 (see the Audit Trail). This phase widens who can reach a desktop (any tailnet person, behind an Admit prompt), so the ten review cases from 22-RESEARCH "Recorded security review" are recorded below, each with the test or VM row that proves it.

**What the evidence does not cover.** The live two-account proof (SC1, M-206) and the live own-account peer filter (AR-21-01, M-207) were never run: the test tailnet has no second Tailscale account. Every automated identity test uses the fake WhoIs in `internal/webserver/webservertest`. The identity threats (T-22-02, T-22-13, T-22-16) therefore have the status **accepted-pending-real-check**: the mitigation is present and passes at test depth, and the live proof is owner-run (M-206 for the two-account guest flow, M-207 for the own-account peer filter), logged as AR-22-09. They are not counted as closed. `threats_open: 0` stands because an accepted-pending item is a documented accepted risk, not an open one; an owner who wants a live proof before shipping to other people should run M-206 and M-207 first.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| tailnet person's browser to the share server's guest routes | Requests to /desktop/guest/, /desktop/guest/ws and the link check, from any user of the tailnet | Guest capability token (secret), static page, signalling |
| WhoIs identity to the admission decision | The caller's node, user and profile as tailscaled reports them | User ID, stable node ID, login and display name (untrusted text) |
| guest to the Admit queue | A knock, then a wait with no media until the sharer answers | Identity, prompt requests |
| guest viewer to input, clipboard and surface channels | Pointer, key and clipboard messages from a modified page | Untrusted protocol input (gated by rights, lease and physical-input state) |
| guest media path to the admitted node | The selected ICE pair's remote address against the admitted user and node | Screen video (sensitive) |
| GUI to the daemon socket (0600, same user) | Start, stop, admit, deny, rights, disconnect and the liveness stream | Share state, link (fetched on demand) |
| daemon to the audit file | One JSON line per event, mode 0600 | Identity, event, share ID, mode, reason (no secret) |
| daemon to the macOS input event system | A listen-only tap for untagged (physical) input | One timestamp per event, never content |
| GUI to Notification Center and the tray | Join, leave and admit text; the share line and icon | Sanitised login names |
| host to VMs; evidence and TESTING.md to the repo | The rig builds on the host and acts only in the VMs; captures and docs are committed | Addresses, names, tokens (must not appear) |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation (evidence) | Status |
|-----------|----------|-----------|----------|-------------|-----------------------|--------|
| T-22-01 | Elevation of privilege | requireTailnetPeer reaching terminal or port routes | high | mitigate | Called only inside setupDesktopGuestRoutes: TestGuestRouteTable_AST (internal/webserver/desktop_guest_routes_test.go:365); TestGuestRoutes_OtherAccountRefusedElsewhere (:305) asserts 403 with the marker on every other route family; the gate itself is internal/webserver/whois.go:252 and sameUser is untouched | closed |
| T-22-02 | Spoofing | tagged, shared-in or profile-less nodes on the guest routes | high | mitigate | tailnetPeer (internal/webserver/whois.go:172-192) fails closed and tailnetPeerStatus answers 403/503 (:199-232); TestTailnetPeer_Refuses (internal/webserver/desktop_guest_gate_test.go:74) covers tagged, shared-in, other-nil-profile, not-found, self-tagged and self-shared-in as 403 with the marker and whois-fault and status-fault as 503; TestTailnetPeer_Unit. Proven only against the fake WhoIs (internal/webserver/webservertest/whois.go); live proof M-206, AR-22-09 | accepted-pending-real-check |
| T-22-03 | Elevation of privilege | guest token replayed on another route family, or a foreign token on the guest family | high | mitigate | Own HKDF info `portty/desktop-guest-token/v1` (internal/capability/desktopguesttoken.go:19); TestGuestCrossToken_TokenLayer and TestGuestCrossToken_Matrix (internal/webserver/desktop_guest_crosstoken_test.go:114) with accepting cells asserted | closed |
| T-22-04 | Elevation of privilege | stale guest link after Stop, a new share or a key regeneration | high | mitigate | guestTokenOutcomeFor compares claims.ShareID with the running share's ID and verifies under the current key (internal/webserver/desktop_guest_routes.go:38-59, compare at :55); 410 when no share is active (:71); re-checked at admit (internal/desktop/guest_viewer.go:80, manager.go:918); TestDesktopShareAPI_StopCutsGuests, TestDesktopShareAPI_RegenerateEndsShare (internal/daemon/desktop_share_api_test.go); VM G-04 "This link no longer works" after stop | closed |
| T-22-05 | Information disclosure | names or tokens in pages, refusals or logs | medium | mitigate | Static pages web/desktop-guest-refused.html and web/desktop-guest-link-invalid.html hold no request text (TestTailnetPeer_RefusedPageIsStatic); no log call in internal/webserver/desktop_guest_routes.go; `Cache-Control: no-store` at :113 | closed |
| T-22-06 | Spoofing | display names with bidi overrides or look-alike text | medium | mitigate | SanitizeName drops Cc and Cf and caps length (internal/desktop/names.go:48); TestNames_Sanitize, TestNames_CapsAtRuneLimit, TestNames_NewIdentitySanitizesNames; the login name is always shown beside the display name (T-22-34, T-22-37). Look-alike glyphs themselves are a person's judgement: M-211 | closed |
| T-22-07 | Information disclosure | secrets in the audit log | high | mitigate | AuditRecord (internal/desktop/audit.go:74-87) has no field that can hold a token, input, clipboard text, SDP or frames; Record (:135) takes identity, event, share ID, mode and reason; TestShareAudit_GuestSession; VM G-10: no `cap=` or token-shaped string in 147 lines | closed |
| T-22-08 | Tampering | audit edits by a guest who holds control | low | accept | AR-22-02 | closed |
| T-22-09 | Information disclosure | the tap as a keylogger-shaped capability | high | mitigate | The callback reads only the source-user-data marker and stores a timestamp (internal/screencap/inputtap_darwin.m:58); 0 matches for Keycode, mouse click state or location reads in the file; the tap runs only while a controller exists (TestLease_TapLifeCycle) | closed |
| T-22-10 | Denial of service | an active tap hanging system input on an Accessibility revoke | high | mitigate | Listen-only option only (internal/screencap/inputtap_darwin.m:108); `kCGEventTapOptionDefault` appears nowhere in the file; a disabled tap is re-enabled (:50) and recreated by the next poll (TestLease_TapStartRetriedAfterFailure); TestArbiter_DisabledTapDrops | closed |
| T-22-11 | Spoofing | a local process posting untagged events to block remote input | low | accept | AR-22-03 | closed |
| T-22-12 | Information disclosure | video before admission | high | mitigate | No offer accepted and no PeerConnection while waiting: TestGuestViewer_TracerAdmit (internal/desktop/guest_viewer_test.go:169) asserts zero viewers while pending; TestGuestViewer_AnythingButPingWhileWaitingIsBadMessage; VM G-01 (`framesDecoded` 0 before Admit) | closed |
| T-22-13 | Spoofing | media from a node other than the admitted one | high | mitigate | checkSelectedPair, checkMediaIdentity and mediaMismatch compare user and stable node ID at connect and on change and fail closed on no lookup, no pair, a lookup error, the wrong user or the wrong node (internal/desktop/media_binding.go, wired at manager.go:795 and :805); DTLS fingerprint verification is not disabled; TestMediaBinding_* (7). Code review WR-04: the cut now goes through the ordered revoke (cutViewer), see 22-REVIEW-FIX.md. Proven only against a fake `Deps.PeerIdentity` and the fake WhoIs; the selected pair was never from another account. The VM G-10 `media_identity_mismatch` line is NOT evidence for this threat (it cut the legitimate same-account guest, O-22-06). Live proof M-206, AR-22-09 | accepted-pending-real-check |
| T-22-14 | Denial of service | prompt spam and squatting | medium | mitigate | admitTimeout 5 min, maxPendingPerShare 8, maxPendingPerUser 2 (internal/desktop/guestshare.go:17-19); TestGuestShare_NinthPendingKnockIsRefusedAndReplacesNobody, _ThirdKnockFromOneUserReplacesTheOldest, _TimeoutFromTheInjectedTimer, _DenyRefusesTheUserFromAnyNode | closed |
| T-22-15 | Elevation of privilege | a guest switching the shared surface | medium | mitigate | Guests never hold RightsSurface; handleSelect checks it (internal/desktop/surface.go:269); guest rights masked at guest_viewer.go:163 and guestshare.go:395; TestGuestViewer_SelectRefused (internal/desktop/guest_admission_test.go:214), TestGuestRights_SurfaceNeverGranted (internal/desktop/rights_test.go:140) | closed |
| T-22-16 | Spoofing | re-admission of a different person on reconnect | high | mitigate | Re-admit only for the identical (share ID, user ID, stable node ID): TestGuestShare_ReadmitsTheSameKeyWithItsGrantsAndClosesTheOlderSocket (internal/desktop/guestshare_test.go:234), TestGuestShare_SecondNodeOfAnAdmittedUserWaits (:268), TestGuestViewer_ReconnectIsAdmittedAtOnceAndTheOlderSocketCloses. Identities are synthetic or from the fake WhoIs only; live proof M-206, AR-22-09 | accepted-pending-real-check |
| T-22-17 | Elevation of privilege | a view-only guest sending input or clipboard messages from a modified page | high | mitigate | rightsOf reads the live atomic (internal/desktop/rights.go:20) and is checked server-side in the press gate (internal/desktop/gates.go:51) and the clipboard handler (internal/desktop/clipboard.go:117); TestGuestRights_EveryKindDropsWithoutControl, TestGuestRights_ClipboardIndependentOfControl, TestGuestRights_RaceWithInput; VM G-02 (view-only counters stay empty) | closed |
| T-22-18 | Elevation of privilege | a revoked viewer's in-flight input | high | mitigate | endViewerInput runs before any transport close (internal/desktop/revoke.go:90); TestGuestRevoke_QueuedInputNeverPostsAndHeldIsReleased; VM G-03 (held key released 0.14 s after the press) | closed |
| T-22-19 | Denial of service | Stop delayed by unresponsive guests | medium | mitigate | Concurrent close with a 2 s ceiling and CloseNow (internal/desktop/revoke.go:18, :52-71, :118); TestGuestStop_ThreeSilentPeersDoNotHoldItUp. The 2 s ceiling bounds only the socket handshake: flushInput, pc.Close and syncTap run before the timer and are unbounded, and the test only requires the guests gone within 8 s (revoke_test.go:259). The code-review fixer also found that with coder/websocket v1.8.15 `CloseNow` after `Close` has started does nothing and `Close` waits up to 5 s for the read lock, so a silent peer stays registered about 5 s (fits G-04's 6.9 s). After WR-04, WR-05 and IN-02 rights, input and media are cut at once and only the socket lingers; the real fix (a per-viewer read context the ceiling cancels, in serveAdmitted) is deferred because Phase 23 is editing that function. Live: VM G-04 PARTIAL, two repeats within 1.5 s and one tray run with a leave 6.9 s after the stop under a host load average of 36 to 55 (Observation O-22-01, M-215) | open — below high threshold (non-blocking) |
| T-22-20 | Tampering | stuck keys or buttons after a grant, lease or physical change | high | mitigate | releaseHeld (internal/desktop/input.go:817) runs on control off, lease change and physical takeover; releases bypass gates; TestLease_TwoGuestsTakeTurns, TestPhysical_UntaggedInputDropsAndReleases, TestGuestRevoke_ConcurrentAndRepeatedCallsReleaseOnce; VM G-03 | closed |
| T-22-21 | Elevation of privilege | a remote admit or grant path | high | mitigate | Admit, deny, rights, disconnect and stop are registered on the daemon unix socket's mux only (internal/daemon/desktop_share_api.go:177-186), served only on the 0600 unix socket (internal/daemon/api.go, ipc.go); the share server's route table is read from source by TestGuestRouteTable_AST, which forbids them; TestDesktopShareAPI_GuiRequired | closed |
| T-22-22 | Repudiation | joins and grants without a record | medium | mitigate | The audit line is written before the change applies (internal/daemon/desktop_share_api.go:486-683); TestShareAudit_GuestSession, _OwnAccount, _DeniedKnock, _TimedOutKnock, _DaemonExitLogsLeaves; VM G-10 (every event kind present) | closed |
| T-22-23 | Information disclosure | people watching with no indicator | high | mitigate | `gui_required` on start (internal/daemon/desktop_share_api.go:31); the GUI's liveness stream closing ends the share at once: TestDesktopShareAPI_GuiRequired, TestGuiLiveness_StreamCloseEndsShare, _SecondHolderIsBusy; VM G-09 (share ended 0.51 s after the GUI died) | closed |
| T-22-24 | Elevation of privilege | guest tokens surviving key regeneration or Stop | high | mitigate | endGuestShare on Stop, regenerate, GUI gone and exit (internal/daemon/api.go:854, internal/daemon/desktop_share_api.go:229, :551, internal/daemon/desktop_api.go:590); the check route answers 410 afterwards; TestDesktopShareAPI_StopCutsGuests, _RegenerateEndsShare, _DaemonExit | closed |
| T-22-25 | Information disclosure | tokens or addresses in the GUI stream or logs | medium | mitigate | The stream carries no user ID, IP or token; the link is fetched on demand; TestDesktopShareAPI_Tracer greps every stream line for `cap=`, token, `100.`, `127.0.0.1`, userId and tailnetIp (internal/daemon/desktop_share_api_test.go:268) | closed |
| T-22-26 | Information disclosure | names on the guest page | medium | mitigate | Only the lease holder's display name is rendered, as text (frontend/src/components/DesktopView.tsx:1155; the server sends a name only for a guest holder, internal/desktop/lease.go:168-172); DesktopView.guest.test.tsx "renders no identity of its own or the sharer: only the lease holder display name appears" | closed |
| T-22-27 | Elevation of privilege | a modified page sending input without a grant | high | accept | AR-22-07: enforced server-side by T-22-17, so the page restrictions are usability only | closed |
| T-22-28 | Spoofing | close reasons parsed from text | low | mitigate | States chosen by close code and fixed reason only; the golden file internal/desktop/testdata/signal-guest-v1.jsonl is checked by TestGuestViewer_Golden and frontend/src/lib/__tests__/desktopClient.guest.test.ts:365 | closed |
| T-22-29 | Information disclosure | someone watching without the person at the Mac knowing | high | mitigate | Icon variant and the shared-with line while anyone is connected (tray.go embeds tray_icon_sharing.png); the GUI's stream is the liveness signal (T-22-23); TestTrayIconSharing, TestTrayShare_Line, TestDesktopShareApp_TracerTray; VM G-08 | closed |
| T-22-30 | Denial of service | a hung GUI keeping the stream alive while the indicator is stale | low | accept | AR-22-04 | closed |
| T-22-31 | Spoofing | look-alike display names in notifications | medium | mitigate | Notifications use the sanitised login name or device name capped at 80 characters (app_desktop_share.go:118, :154-171); TestShareNotification_NameIsCappedAtEightyWithEllipsis, _JoinAndLeaveSentences | closed |
| T-22-32 | Information disclosure | the guest link in logs, events or banners | medium | mitigate | The URL is returned only by GetDesktopShare for Copy Link and never emitted or logged (app_desktop_share.go:382-405); TestShareNotification_NoActionButtonsOrLinkInBody, TestDesktopShareBinding_StartAndGet | closed |
| T-22-33 | Information disclosure | a share left running behind a hidden window | medium | mitigate | QuitGUIOnly stops the share first (app.go:1010 calls stopShareForQuit, app_desktop_share.go:232-249; app.go:1012 is the only WindowHide call); TestDesktopShareQuit_StopsAnActiveShareFirst, _NoShareNoStop, _SlowStopDoesNotBlockHiding; the tray indicator stays while the GUI lives. Real QuitGUIOnly needs AppKit: M-219 | closed |
| T-22-34 | Spoofing | imitation display names in the Watching list | medium | mitigate | Login name always visible, names inside `bdi` as text (frontend/src/components/ViewerList.tsx:116, :125); ViewerList.test.tsx bidi, names-are-never-markup and login-visible cases; second sanitising pass in the GUI | closed |
| T-22-35 | Elevation of privilege | an accidental control grant | medium | mitigate | Off by default; the switch carries the caption "Control lets <name> use this Mac as you, so they can do anything you can" (frontend/src/lib/shareDesktopCopy.ts:40); one click off releases held input (T-22-20); ViewerList.test.tsx grant cases | closed |
| T-22-36 | Tampering | click-through or stray-key Admit | high | mitigate | ADMIT_GUARD_MS 600 (frontend/src/components/AdmitSheet.tsx:8), focus on the dialog, no default button, aria-disabled during answers; AdmitSheet.test.tsx "stray Enter, Space and Escape answer nobody", "knock then deliberate admit", "both buttons are aria-disabled inside the guard"; useDialogFocus.test.tsx dialog mode. Real typing timing: M-210 | closed |
| T-22-37 | Spoofing | imitation display names on the Admit sheet | high | mitigate | Login name always shown, never truncated; paragraph 2 reads "Check the login name. Anyone can set a display name that looks like someone else's." (frontend/src/lib/shareDesktopCopy.ts:73); AdmitSheet.test.tsx bidi, 200-character login and login-row cases; daemon and GUI sanitising. Real glyph shaping: M-211 | closed |
| T-22-38 | Information disclosure | real addresses, names or guest tokens in committed evidence | medium | mitigate | The rig scrubs before writing (rig/p22-rig.sh `scrub`); 22-12 grepped every committed evidence file for the real name, login, `cap=`, tailnet addresses and `.ts.net` names: none; `bash tests/check-brand.sh` clean in this run | closed |
| T-22-39 | Tampering | host side effects during VM rows | medium | mitigate | 22-12-SUMMARY "Threat Flags": compile-only on the host, no capture, input, tap or tailscaled contact on the host, nothing in ~/Downloads or /Applications; TestInputTap_RefusesInTests (internal/screencap/inputtap_darwin_test.go) refuses the real tap under go test | closed |
| T-22-40 | Repudiation | an unrecorded security review for the widening | medium | mitigate | This file: the ten cases below each name a passing test or VM row; threats_open gate | closed |
| T-22-SC | Tampering | npm/pip/cargo installs (22-01 to 22-13) | high | mitigate | `git diff 07ad7d21 HEAD` (07ad7d21 is the last planning commit before 22-01 ran) of go.mod, go.sum, frontend/package.json, frontend/pnpm-lock.yaml, .github and internal/webserver/csp_mw.go is empty: no package was added (AR-22-08). The diff from 5bdeb6eb^ is not empty because Phase 19-21 merges landed later | closed |

*Status: open · closed · accepted-pending-real-check (mitigation present at test depth, live proof owner-run; logged in the Accepted Risks Log) · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## The Ten Review Cases

Each case lists the proof and its result from the phase-gate run of 2026-11-27 (`go test -race -count=1 ./...`; every package ok once the four load-induced timing failures were re-run, see the Audit Trail) or from the 22-12 VM rows.

| # | Case | Proof | Result |
|---|------|-------|--------|
| 1 | Admit: a guest waits with no PeerConnection and no media; Admit creates it; View only; control and clipboard are separate grants | TestGuestViewer_TracerAdmit, TestGuestRights_TracerControl, TestGuestRights_ClipboardIndependentOfControl; VM G-01, G-02 | pass (unit and VM) |
| 2 | Deny, timeout and spam: a denied user cannot knock again from any device; caps and the 5 minute timeout; audit lines | TestGuestShare_DenyRefusesTheUserFromAnyNode, TestGuestViewer_DeniedUserIsRefusedAtOnceFromAnotherNode, TestGuestShare_NinthPendingKnockIsRefusedAndReplacesNobody, TestGuestShare_TimeoutFromTheInjectedTimer, TestGuestShare_RequestOlderThanFiveMinutesTimesOutByTheClock, TestShareAudit_DeniedKnock, TestShareAudit_TimedOutKnock | pass |
| 3 | Revoke mid-input: held keys released, next input dropped, sockets closed, link still valid for others, the revoked user cannot knock again | TestGuestRevoke_QueuedInputNeverPostsAndHeldIsReleased, TestGuestRevoke_DenyListsTheUser, TestGuestRevoke_UnknownViewer; VM G-03 (key released 0.14 s after the press) | pass (unit and VM) |
| 4 | Stop with viewers mid-frame: all viewers closed concurrently within 2 s; the old link refused; a new share has a new share ID | TestGuestStop_ThreeSilentPeersDoNotHoldItUp, TestGuestStop_LateGuestsAreRefused, TestDesktopShareAPI_StopCutsGuests (includes the new-share-ID check), TestDesktopShareAPI_RegenerateEndsShare; VM G-04 | unit pass; VM PARTIAL (see O-22-01) |
| 5 | Cross-token in every direction: the guest token opens only the guest socket and no other token opens it | TestGuestCrossToken_TokenLayer, TestGuestCrossToken_Matrix | pass |
| 6 | Wrong identity: another user's node, tagged, shared-in, nil profile, not-found and a WhoIs fault are refused (403 with the marker, or 503) and no page names an account | TestTailnetPeer_Refuses, TestTailnetPeer_Unit, TestTailnetPeer_BidiIdentityIsRaw, TestTailnetPeer_RefusedPageIsStatic, TestGuestRoutes_OtherAccountRefusedElsewhere, TestDiscoverPeers_OwnAccountOnly | pass at test depth; live UNVERIFIED (M-206, M-207) |
| 7 | Reconnect from a different node of the same person prompts again; from the same node it is re-admitted with its grants and the older socket closes | TestGuestShare_ReadmitsTheSameKeyWithItsGrantsAndClosesTheOlderSocket, TestGuestShare_SecondNodeOfAnAdmittedUserWaits, TestGuestViewer_ReconnectIsAdmittedAtOnceAndTheOlderSocketCloses | pass |
| 8 | Media identity mismatch: a selected pair from another node or user closes the peer; lookup failure and no pair fail closed | TestMediaBinding_PairChangeToAnotherNodeCutsTheGuest, _AnotherUserIsCut, _LookupErrorFailsClosed, _NoSelectedPairFailsClosed, _TwoMismatchesCutOnceAndReportOnce. (The VM G-10 `media_identity_mismatch` line was a cut of the legitimate same-account guest, not a detection: O-22-06) | pass at test depth; live UNVERIFIED (M-206) |
| 9 | A controlling guest can self-grant through the Portty UI: possible, documented as inherent | Not preventable by design: control lets the guest do anything the sharer's account can do. The copy exists: "Control lets <name> use this Mac as you, so they can do anything you can." (frontend/src/lib/shareDesktopCopy.ts:40); see AR-22-01 | accepted and documented |
| 10 | Audit: no secret in any line, sanitised names, mode 0600, rotation | TestAudit_TracerOneLine, TestAudit_OpenNarrowsExistingMode, TestAudit_RotatesAtCap, TestShareAudit_GuestSession, TestDesktopShareAPI_Tracer (stream grep); VM G-10 (mode 600, 147 lines, no token-shaped string) | pass (unit and VM) |

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-22-01 | Case 9 | A guest who holds control equals the sharer's account on that Mac, so they can open the Portty window and grant themselves clipboard or add control for another guest, or stop the share. The sharer grants control per person, off by default, with the caption that says so (T-22-35), and the grant can be revoked in one click (T-22-20). | 22-CONTEXT [Admit] (plan-time decision) | 2026-11-27 |
| AR-22-02 | T-22-08 | A controlling guest could edit or delete the 0600 local audit file. Control equals the sharer's account, so no local file can be protected from that guest; the audit log records what happened up to that point. | Plan 22-02 threat model | 2026-11-27 |
| AR-22-03 | T-22-11 | A local process posting untagged input can keep blocking a guest's remote input. It fails safe: remote input is dropped, never the person at the Mac. | Plan 22-03 threat model | 2026-11-27 |
| AR-22-04 | T-22-30 | A hung (not dead) GUI keeps its liveness stream open, so the share continues and the indicator may be stale. RESEARCH A5 residual; a dead GUI ends the share (T-22-23, VM G-09). | Plan 22-08 threat model | 2026-11-27 |
| AR-22-05 | DCTL-04 | While Secure Event Input is on (a password field or Secure Keyboard Entry), macOS withholds keyboard events from a listen-only tap, so physical keyboard input at the Mac is not detected; remote keys keep posting and the sharer sees the banner "Typing at this Mac isn't detected right now". Pointer detection is unaffected. Appendix B and M-213 cover the real-hardware case. | 22-CONTEXT (DCTL-04 call-order decisions), 22-11 banner | 2026-11-27 |
| AR-22-06 | Identity | Identity is node-level: Admit names a user and a stable node, not a person. A person who can sit at an admitted user's node is that guest. The login name next to the display name is what the sharer judges (T-22-37). | 22-CONTEXT [Who may join] | 2026-11-27 |
| AR-22-07 | T-22-27 | A modified guest page can try to send input without a grant; the page restrictions are usability only and the server enforces rights (T-22-17). | Plan 22-07 threat model | 2026-11-27 |
| AR-22-08 | T-22-SC | Plans 22-01 through 22-13 add no Go, npm, pip or cargo package. | Plan threat models | 2026-11-27 |
| AR-22-09 | T-22-02, T-22-13, T-22-16 | Live-identity gap: every identity test uses the fake WhoIs (internal/webserver/webservertest) or a fake `Deps.PeerIdentity`; the test tailnet has no second Tailscale account, so the two-account guest flow, the media binding against another account's node and same-key re-admission were never proven live. Status accepted-pending-real-check until the owner runs M-206 (two-account guest flow, SC1) and M-207 (own-account peer filter, AR-21-01) and records the result in 22-UAT.md. | Phase 22 gate (secure-phase re-audit), owner-run items M-206/M-207 | 2026-11-27 |

*Accepted risks do not resurface in future audit runs.*

---

## Flagged Probe Assumptions

These are the assumptions 22-01's spec-less probe coverage table flagged for this review. Each is documented here and was built as stated.

| # | Requirement | Assumption | Where it holds |
|---|-------------|-----------|----------------|
| 1 | DSHR-03 | An own-account device on a guest link follows the same Admit and view-only rules as anyone else; a tagged or shared-in Mac cannot host a guest share. | TestGuestViewer_OwnAccountViewerKeepsModeOwnAndFiresTheHooks, TestTailnetPeer_Refuses (self-tagged, self-shared-in) |
| 2 | DSHR-06 | The indicator counts guests and own-account viewers together and never pending requests; it shows while anyone is connected. | TestTrayShare_Line, TestDesktopShareApp_StateDrivesCountAndIconRefresh; VM G-08 |
| 3 | DSHR-07 | A write failure never blocks a join; a crash leaves a share_start with no share_stop (an unclean end), which this file and the audit file's reader should treat as such. | TestAudit_WriteFailureNeverBlocksAndLogsOncePerMinute, TestShareAudit_FailedWriteNeverBlocks, TestShareAudit_DaemonExitLogsLeaves (graceful exit only) |
| 4 | DCTL-04 | Two inputs arriving together are decided in input-goroutine order (the first decided takes a free lease). | TestArbiter_SameInstantFirstDecidedWins |
| 5 | DCTL-04 | While Secure Event Input is on, the keyboard half of physical-wins is blind. | TestPhysical_SecureInputStillPosts; AR-22-05; VM G-06 |

---

## Observations (not threats)

| ID | Source | Observation | Disposition |
|----|--------|-------------|-------------|
| O-22-01 | VM G-04 | One tray Stop with two guests ended one guest's connection 6.9 s after the stop, under a host load average of 36 to 55; two timed repeats were within 1.5 s and 1.0 s. Cause not shown; in this review's own first full test run, under a similar load, `TestDesktopShareAPI_StopCutsGuests` timed out at 5 s once and passed on every re-run, which fits the load explanation but does not prove it. | Owner-run re-measure on a quiet machine: M-215. Tied to T-22-19 (medium, non-blocking). |
| O-22-02 | VM G-11 | The `window_key_focus` counter was not reached on macOS 27.2: Spotlight and the Fonts panel take the focused application, so the Phase 20 frontmost check dropped the keys first. The safety outcome held (nothing posted). | Owner-run item with a real non-activating panel: M-214. |
| O-22-03 | VM X-1 | Two windows of one device on one share make the daemon log a join and a leave every 1 to 3 s and coalesced notifications. Cause not investigated. | Not a security finding; recorded for a later phase. |
| O-22-04 | VM notes | A long-lived viewer's picture stalled once with `status streaming` and frozen `bytesReceived`; a reload fixed it. Not reproduced. | Not a security finding. |
| O-22-05 | Repo | The tracked stray file `.claire/worktrees/agent-aac25194/internal/webserver/csp_integration_test.go` (a one-word placeholder swept into inherited commit 08fc2be9, June 2026) makes `gofmt -l` print a parse error on stderr. Not a Phase 22 change. | Removed in the Phase 22 close. |
| O-22-06 | VM G-10 | The G-10 audit line `revoked` / `media_identity_mismatch` (evidence/G-10.txt:30, 18:28:49Z) cut the guest VM, the legitimately admitted same-account guest, which rejoined at 18:29:17Z without a knock. It is a live false-positive cut, not a detection of a foreign node. One possible cause, not investigated: the selected pair's remote address is not a tailnet address, so WhoIs fails and the check returns `lookup_failed`. | Reliability follow-up; re-check with M-206. Not evidence for T-22-13. |
| O-22-07 | VM G-10 | 22-VM-CHECKS.md's G-10 row says `revoked` 5; the G-10.txt events line says `revoked=3`. | Record discrepancy; evidence file is the source. |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open (below high) | Open (high or above) | Run By |
|------------|---------------|--------|-------------------|----------------------|--------|
| 2026-11-27 | 41 | 40 | 1 (T-22-19, medium) | 0 | 22-13 executor (ASVS L1, block_on high; plan-time register confirmed at grep and test depth; auditor subagent not spawned) |
| 2026-11-27 | 41 | 37 (+3 accepted-pending-real-check: T-22-02, T-22-13, T-22-16) | 1 (T-22-19, medium) | 0 | gsd-secure-phase re-audit at the Phase 22 gate: gsd-security-auditor against 597ca5a7, every mitigation re-located and every named test run (webserver 15, desktop 77, daemon 22, app 20, tailnet and screencap pass, vitest 147/147, brand gate clean); line drift and the T-22-13 and T-22-SC evidence corrected |

Commands run for this review (all in the 22-13 worktree, 2026-11-27):

- `GOTOOLCHAIN=go1.26.11 go vet ./...`: clean.
- `gofmt -l $(git ls-files '*.go')`: no file listed (one parse error on stderr for the stray `.claire/` placeholder, O-22-05).
- `GOTOOLCHAIN=go1.26.11 ~/go/bin/golangci-lint run --new-from-rev=07ad7d21 ./...`: 0 issues.
- `GOTOOLCHAIN=go1.26.11 go test -race -count=1 ./...`: 14 packages ok; four timing-sensitive tests failed in that first run while the host load average read 16 to 31 and vitest ran beside it (internal/daemon `TestDesktopShareAPI_StopCutsGuests` timed out at 5 s waiting for the inactive state line, and `TestGetSessionStyledTailLines_UnstrippedQueryNeverHangs`; internal/desktop `TestQualityShaped_SlowViewerDoesNotStallOthers` and `TestQualityShaped_StillScreenDoesNotCollapseEstimate`, both Phase 23 tests). The two failing packages were re-run alone, `go test -race -count=1 ./internal/daemon ./internal/desktop`, at a host load average of 8 to 20: both ok, and the two daemon tests also passed on their own.
- `tsc --noEmit -p tsconfig.json` in frontend: clean.
- `vitest run` in frontend: first run 148 of 149 files passed, the one failure a 5 s timeout in `pathLinks.linkifier.test.ts` (an existing test, unrelated to this phase) under the same host load; the file passed alone (10 of 10) and a second full run passed: 149 files, 2719 tests.
- `bash tests/check-brand.sh`: clean. `bash tests/check-traceability-paths.sh`: OK, 340 paths exist.
- `git diff` from the start of Phase 22 of go.mod, go.sum, frontend/package.json, frontend/pnpm-lock.yaml, internal/webserver/csp_mw.go and `.github`: empty (no package added, CSP middleware and CI unchanged).

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed (no open threat at or above high)
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-11-27 (re-audited at the phase gate), at test depth; T-22-02, T-22-13 and T-22-16 accepted-pending-real-check (AR-22-09). The live second-account proof (M-206) and the live own-account peer filter (M-207) are owner-run and not yet run.
