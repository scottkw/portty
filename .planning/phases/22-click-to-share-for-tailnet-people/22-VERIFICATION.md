---
phase: 22-click-to-share-for-tailnet-people
verified: 2026-11-27T21:39:52Z
status: human_needed
score: 5/7 must-haves verified
covered_files:
  - ".planning/phases/22-click-to-share-for-tailnet-people/22-01-PLAN.md"
  - ".planning/phases/22-click-to-share-for-tailnet-people/22-01-SUMMARY.md"
  - ".planning/phases/22-click-to-share-for-tailnet-people/22-02-PLAN.md"
  - ".planning/phases/22-click-to-share-for-tailnet-people/22-02-SUMMARY.md"
  - ".planning/phases/22-click-to-share-for-tailnet-people/22-03-PLAN.md"
  - ".planning/phases/22-click-to-share-for-tailnet-people/22-03-SUMMARY.md"
  - ".planning/phases/22-click-to-share-for-tailnet-people/22-04-PLAN.md"
  - ".planning/phases/22-click-to-share-for-tailnet-people/22-04-SUMMARY.md"
  - ".planning/phases/22-click-to-share-for-tailnet-people/22-05-PLAN.md"
  - ".planning/phases/22-click-to-share-for-tailnet-people/22-05-SUMMARY.md"
  - ".planning/phases/22-click-to-share-for-tailnet-people/22-06-PLAN.md"
  - ".planning/phases/22-click-to-share-for-tailnet-people/22-06-SUMMARY.md"
  - ".planning/phases/22-click-to-share-for-tailnet-people/22-07-PLAN.md"
  - ".planning/phases/22-click-to-share-for-tailnet-people/22-07-SUMMARY.md"
  - ".planning/phases/22-click-to-share-for-tailnet-people/22-08-PLAN.md"
  - ".planning/phases/22-click-to-share-for-tailnet-people/22-08-SUMMARY.md"
  - ".planning/phases/22-click-to-share-for-tailnet-people/22-09-PLAN.md"
  - ".planning/phases/22-click-to-share-for-tailnet-people/22-09-SUMMARY.md"
  - ".planning/phases/22-click-to-share-for-tailnet-people/22-10-PLAN.md"
  - ".planning/phases/22-click-to-share-for-tailnet-people/22-10-SUMMARY.md"
  - ".planning/phases/22-click-to-share-for-tailnet-people/22-11-PLAN.md"
  - ".planning/phases/22-click-to-share-for-tailnet-people/22-11-SUMMARY.md"
  - ".planning/phases/22-click-to-share-for-tailnet-people/22-12-PLAN.md"
  - ".planning/phases/22-click-to-share-for-tailnet-people/22-12-SUMMARY.md"
  - ".planning/phases/22-click-to-share-for-tailnet-people/22-13-PLAN.md"
  - ".planning/phases/22-click-to-share-for-tailnet-people/22-13-SUMMARY.md"
  - ".planning/phases/22-click-to-share-for-tailnet-people/22-REVIEW-FIX.md"
  - ".planning/phases/22-click-to-share-for-tailnet-people/22-SECURITY.md"
  - ".planning/phases/22-click-to-share-for-tailnet-people/22-UAT.md"
  - ".planning/phases/22-click-to-share-for-tailnet-people/22-VM-CHECKS.md"
  - "README.md"
  - "TESTING.md"
  - "app.go"
  - "app_desktop_share.go"
  - "app_update.go"
  - "assets/tray_icon_sharing.png"
  - "assets/tray_icon_sharing@2x.png"
  - "frontend/src/App.tsx"
  - "frontend/src/components/AdmitSheet.tsx"
  - "frontend/src/components/DesktopControlBar.tsx"
  - "frontend/src/components/DesktopPhoneBars.tsx"
  - "frontend/src/components/DesktopPhoneLayer.tsx"
  - "frontend/src/components/DesktopSendKeysMenu.tsx"
  - "frontend/src/components/DesktopView.tsx"
  - "frontend/src/components/QuitConfirmModal.tsx"
  - "frontend/src/components/SettingsRow.tsx"
  - "frontend/src/components/SettingsSheet.tsx"
  - "frontend/src/components/ShareBanner.tsx"
  - "frontend/src/components/ShareDesktopSheet.tsx"
  - "frontend/src/components/Switch.tsx"
  - "frontend/src/components/ViewerList.tsx"
  - "frontend/src/lib/desktopClient.ts"
  - "frontend/src/lib/desktopShare.ts"
  - "frontend/src/lib/shareDesktopCopy.ts"
  - "frontend/src/lib/tailscale.ts"
  - "frontend/src/lib/useDialogFocus.ts"
  - "frontend/src/style.css"
  - "frontend/src/wailsjs/go/main/App.d.ts"
  - "frontend/src/wailsjs/go/main/App.js"
  - "internal/capability/desktopguesttoken.go"
  - "internal/daemon/api.go"
  - "internal/daemon/client_desktop_share.go"
  - "internal/daemon/desktop_api.go"
  - "internal/daemon/desktop_share_api.go"
  - "internal/desktop/arbiter.go"
  - "internal/desktop/audit.go"
  - "internal/desktop/clipboard.go"
  - "internal/desktop/gates.go"
  - "internal/desktop/guest_viewer.go"
  - "internal/desktop/guestshare.go"
  - "internal/desktop/input.go"
  - "internal/desktop/lease.go"
  - "internal/desktop/manager.go"
  - "internal/desktop/media_binding.go"
  - "internal/desktop/names.go"
  - "internal/desktop/probe_vmdesktop.go"
  - "internal/desktop/revoke.go"
  - "internal/desktop/rights.go"
  - "internal/desktop/surface.go"
  - "internal/desktop/testdata/signal-guest-v1.jsonl"
  - "internal/desktop/viewer.go"
  - "internal/desktop/window.go"
  - "internal/screencap/inputtap_darwin.go"
  - "internal/screencap/inputtap_darwin.m"
  - "internal/screencap/window_darwin.go"
  - "internal/screencap/window_darwin.m"
  - "internal/tailnet/tailnet.go"
  - "internal/webserver/desktop_guest_routes.go"
  - "internal/webserver/desktop_routes.go"
  - "internal/webserver/server.go"
  - "internal/webserver/webservertest/whois.go"
  - "internal/webserver/whois.go"
  - "scripts/verify-icons.sh"
  - "tray.go"
  - "tray_objc_darwin.m"
  - "web/desktop-guest-link-invalid.html"
  - "web/desktop-guest-refused.html"
  - "web/embed.go"
covered_digest: "v4:sha256:14166ebe0c2061a15a77091ea1455b11e73e0ce6a57cd1f44e0fadc0135ce5c9"
behavior_unverified: 2
overrides_applied: 0
behavior_unverified_items:
  - truth: "SC1 (DSHR-03): a person signed in to a different Tailscale account on the same tailnet can open the Share Desktop link, and the same person still gets the own-devices refusal on any terminal link"
    test: "M-206 (TESTING.md Section 5): a second Tailscale user opens the guest link, waits at Admit, watches View only after Admit, then opens a terminal link from the same device"
    expected: "Waiting for <Mac> to let you in; the Admit sheet on the shared Mac names the second user (display name, login, device); View only after Admit; 403 own-devices refusal on the terminal link"
    why_human: "The gate (tailnetPeer, requireTailnetPeer, route table) is present, wired and tested, but every identity test feeds a fake WhoIs. Whether real tailscaled answers a same-tailnet, different-user node with Sharer 0, a non-zero User and a UserProfile is a property of the live control plane. The VM rows used two nodes of the same account only; SC1 is UNVERIFIED in 22-VM-CHECKS.md and T-22-02/13/16 are accepted-pending-real-check (AR-22-09) in 22-SECURITY.md."
  - truth: "Carried-in M-05: while a window is shared, a controlling viewer's keys are dropped unless the shared window holds keyboard focus (a non-activating panel such as Alfred or Raycast must not receive them)"
    test: "M-214: share a window to a controlling viewer, open an Alfred or Raycast style non-activating panel locally, have the viewer type, then close the panel and type again"
    expected: "Keys are dropped and counted window_key_focus while the panel holds focus; nothing types into the panel or the window; typing reaches the window again after the panel closes"
    why_human: "windowAllows calls keyFocused (window.go:229-235) backed by a real AXFocusedUIElement read (window_darwin.m:167), and TestWindowKeyFocus_* pass against a fake bridge. VM G-11 is PARTIAL: Spotlight and the Fonts panel on macOS 27.2 both take the focused application, so the Phase 20 'raising' check dropped the keys and the window_key_focus branch was never reached on a real system."
human_verification:
  - test: "M-206 SC1 live two-account guest flow (DSHR-03)"
    expected: "A second Tailscale user opens the guest link, waits at Admit, watches view-only after Admit, and is refused on a terminal link from the same device"
    why_human: "Needs a second Tailscale account; all automated identity tests use a fake WhoIs (SC1 UNVERIFIED in the VM rows)"
  - test: "M-207 AR-21-01 own-account peer filter on a real tailnet"
    expected: "A node from another account shows no Desktop row and no terminal peer; a hand-typed connect to it is refused unknown_host"
    why_human: "Needs a node from another account online; discovery filter and connect re-resolve are unit-tested against fake ipnstate only"
  - test: "M-208 restrictive tailnet ACL"
    expected: "With tcp/udp 7444 blocked to the guest, the guest page shows Can't reach this Mac with the access-rules hint"
    why_human: "No restrictive ACL on the test tailnet (NOT RUN)"
  - test: "M-209 tray sharing icon in light and dark menu bars"
    expected: "The sharing variant is legible beside other tray icons in both appearances"
    why_human: "Visual judgement; G-08 saw the light menu bar only"
  - test: "M-210 Admit sheet arriving while typing in a real terminal"
    expected: "No stray Return, Space or Esc answers it; a click within about 600 ms does nothing"
    why_human: "Real keyboard timing and feel"
  - test: "M-211 right-to-left or look-alike display name on the real Admit sheet"
    expected: "Name renders isolated, login name still visible, layout not spoofed"
    why_human: "Real text rendering of hostile names"
  - test: "M-212 Someone is using this Mac with real hardware input"
    expected: "Real typing or mouse movement drops remote input and the guest sees the chip"
    why_human: "G-05 used a software untagged CGEvent stand-in"
  - test: "M-213 warp check and real Secure Keyboard Entry (completes RESEARCH Appendix B)"
    expected: "A real mouse move right after a remote pointer move registers locally; with Secure Keyboard Entry on, the banner shows while remote keys still post"
    why_human: "Appendix B PARTIAL; real hardware only"
  - test: "M-214 M-05 window-share key gate with a real non-activating panel"
    expected: "Remote keys drop as window_key_focus and nothing types into the panel; closing it restores typing"
    why_human: "G-11 PARTIAL; the window_key_focus branch was never reached on a real system"
  - test: "M-215 Stop Sharing with two real guests on a quiet machine"
    expected: "Both guests see Sharing has ended within about 2 s"
    why_human: "G-04 PARTIAL (one 6.9 s straggler under host load 36-55); 22-REVIEW-FIX documents a remaining ~5 s socket linger for a silent peer"
  - test: "M-216 Admit sheet stacking, window forward on a knock, notification click routing"
    expected: "Admit sheet above Share Desktop and Settings; a knock brings Portty forward; a notification click opens the sheet only when a share is active and nothing is pending"
    why_human: "Window ordering on a real desktop; not driven in the VM rows"
  - test: "M-217 join, leave and admit banners while the display is captured"
    expected: "Banners delivered (or consistently suppressed) during capture; coalescing within 5 s works"
    why_human: "G-07 proved delivery records, not visible banners on a real display"
  - test: "M-218 Share stopped banner after the daemon dies; upgrade handoff with a share"
    expected: "Killing the daemon mid-share shows the banner; an upgrade ends the share cleanly"
    why_human: "Not driven in the VM rows"
  - test: "M-219 Quit GUI Only while sharing; GUI crash"
    expected: "Quit GUI Only ends the share, clears the tray and hides the window; a crash ends the share (gui_gone) with no stale tray state on relaunch"
    why_human: "G-09 covered SIGTERM only; relaunch needed open -n in the VM (LaunchServices -600), real double-click relaunch unobserved"
  - test: "M-220 VoiceOver on the Admit sheet and grant switches"
    expected: "VoiceOver reads name, login, device, join/leave announcements and the Allow control / Allow clipboard switches"
    why_human: "Screen-reader behaviour; UI review flags live-region timing issues"
  - test: "NOT IN 22-UAT.md: re-check the code-review fixes WR-01..WR-08 and IN-02 in the real app"
    expected: "Two tabs of one device: the older shows Opened in another tab and stops retrying (WR-01, also the X-1 flapping); repeated knocks are rate-limited and the Admit guard does not restart (WR-02); Deny or Disconnect cuts every device of that person (WR-03); a media-identity cut ends input and media at once (WR-04); a reconnect is not held up by its dead old socket (WR-05); focus returns to the dialog for the next person (WR-06); own-account controllers see the lease and physical chips and Control paused when the tap is unhealthy (WR-07); the Share Desktop Status row shows a state with always-available off (WR-08)"
    why_human: "Each fix is marked 'requires human verification' in 22-REVIEW-FIX.md and landed after the VM rows ran on build dfa1a208; WR-01 and WR-07 change the wire protocol and add new UI strings no VM or real Mac has shown"
---

# Phase 22: Click-to-Share for Tailnet People Verification Report

**Phase Goal:** The sharer shows their desktop to a colleague on the tailnet with one click. Anyone on the tailnet can open the link, but each guest waits at an Admit prompt that names them, watches view-only until the sharer grants control for that person, and can be cut off at once. The shared Mac always shows who is watching and keeps a local audit log. Terminal links stay limited to the host's own account.
**Verified:** 2026-11-27T21:39:52Z
**Status:** human_needed
**Re-verification:** No, initial verification

## Goal Achievement

The code for every success criterion exists, is substantive, is wired end to end (guest route, daemon Manager, daemon socket API, Wails bindings, GUI sheets, tray), and passes its targeted tests at HEAD `5d772970`, which contains the review-fix merge `0fa18d33`. Nine VM rows prove the flow between two nodes of one account on real macOS 27.2. Two truths are present and wired but have no behavioural proof on a real system: the cross-account half of SC1 (no second Tailscale account, all identity tests use a fake WhoIs) and the M-05 window key-focus gate (G-11 never reached the branch). I found no missing, stubbed or broken implementation, so this is `human_needed`, not `gaps_found`.

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | SC1 (DSHR-03): Share Desktop creates a link a person on a different Tailscale account on the same tailnet can open; that person still gets the own-devices refusal on any terminal link | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | Present and wired: `tailnetPeer` (`whois.go:171`) refuses tagged, shared-in (`Sharer != 0`), zero-user and profile-less nodes and does not compare the user with self; `requireTailnetPeer(Page)` (`whois.go:252,269`) is used only by `setupDesktopGuestRoutes` (`desktop_guest_routes.go:89,104,108`); guest token under its own HKDF key (`capability/desktopguesttoken.go`), checked against the active share ID (`guestTokenOutcomeFor`). `TestGuestRoutes_OtherAccountRefusedElsewhere` gives an `Other` and a `Colleague` identity 403 with the marker on `/app/`, `/sessions/*`, `/api/*`, the port route and `/desktop/`, and 200/204/101 on the guest family; `TestGuestRouteTable_AST` pins that only the guest family widens and that no share-server route admits, denies, grants or stops; `TestGuestCrossToken_*` pass. The refusal half is proven at test depth; the positive half rests on a fake WhoIs. VM rows G-01/G-02/G-04/G-07 ran between two nodes of the same account only; SC1 is UNVERIFIED (22-VM-CHECKS.md). Routed to M-206. |
| 2 | SC2 (DSHR-04): each guest waits at an Admit prompt showing their tailnet identity; an admitted guest is view-only until the sharer allows control, and separately clipboard, for that person | ✓ VERIFIED | `GuestShare.Knock` queue with per-user and per-share limits, 5-minute timeout, deny by user ID, rate limit (WR-02); no PeerConnection before admit (`guest_viewer.go`); `AdmitSheet.tsx` shows display name, login and device in `<bdi>`, no default button, mounted only in the app (`App.tsx:1661`, `mode !== 'web'`). `pressAllowed` drops every press without `RightsControl` (`gates.go:51`); `clipGate` refuses without `RightsClipboard` (`clipboard.go:117`); `SetRights` is guest-only and refuses a viewer being cut (IN-02). Tests: `TestGuestRights_*` (incl. `EveryKindDropsWithoutControl`, `ClipboardIndependentOfControl`, `ReconnectKeepsGrants`), `TestGuestShare_*`, `TestGuestViewer_*`, AdmitSheet and ViewerList vitest. VM G-01 (Waiting card, Admit sheet in AX, View only after Admit) and G-02 (nothing posted view-only; `Portty 22` typed after Allow control) PASS. |
| 3 | SC3 (DSHR-05, DSHR-06): stop or revoke ends a connection at once, the link dies with the share, join/leave notifications and a persistent "shared with N" indicator with one-click Stop | ✓ VERIFIED | `cutViewer` (`revoke.go:121`) clears rights, ends and flushes input, forgets the lease, deny-lists, closes the PeerConnection, then closes the socket off-thread with a 2 s ceiling; `CloseGuests` cuts concurrently; `CloseViewer` and `DenyGuest` cut every device of the person (WR-03). Link dies: `guestTokenOutcomeFor` requires the active share ID. GUI liveness: the events stream closing ends the share with `gui_gone` (`desktop_share_api.go:229`). Tray: `desktopShareTitle` gives "Desktop shared with nobody yet / 1 person / N people" (`app_desktop_share.go:47-51`) with Stop Sharing. Tests: `TestGuestRevoke_*`, `TestGuestStop_ThreeSilentPeersDoNotHoldItUp`, `TestDesktopShareAPI_StopCutsGuests`, `TestGuiLiveness_StreamCloseEndsShare`. VM G-03 (held A released 0.14 s after Disconnect), G-07 (knock, join, leave notifications delivered during capture), G-08 (tray line, Stop directly under it, icon variant), G-09 (GUI SIGTERM ends share in 0.51 s) PASS; G-04 PARTIAL (old link refused, both pages ended, one leave 6.9 s late under load). See Warnings. |
| 4 | SC4 (DSHR-07): every join, leave and control grant is written to a local audit log on the shared Mac | ✓ VERIFIED | `desktop.OpenAudit` 0600 JSON Lines with 5 MiB rotation (`audit.go`); daemon records share_start, admitted, granted/revoked (control and clipboard), revoked, share_stop, leave, join and the GuestShare events (`desktop_share_api.go:486-964`); own-account joins and leaves too (`:921`). Tests `TestAudit_*`, `TestShareAudit_*` pass. VM G-10: mode 600, 147 lines with every required event and reason, no `cap=` or token-shaped string. |
| 5 | SC5 (DCTL-04): with several controllers the last typer holds control about 2 s; physical input at the Mac always wins | ✓ VERIFIED | `leaseHold = 2s`, `physicalHold = 1s` (`arbiter.go:32-33`); `decideLease` inside `pressAllowed`; listen-only tap started by `syncTap` through `screencap.StartInputTap` (`manager.go:522`), fails closed when unhealthy and tells viewers `unavailable` (WR-07). Behavioural tests pass: `TestArbiter_PhysicalWins`, `TestArbiter_LeaseTakeRenewAndHold`, `TestArbiter_LeaseExpiryHandsOver`, `TestLease_TwoGuestsTakeTurns`, `TestPhysical_UntaggedInputDropsAndReleases`, `TestPhysical_UnhealthyTapFailsClosed`. VM G-05 (untagged click drops the guest's typing, chip shown, typing resumes after the lease) and G-06 (Secure Event Input banner, remote keys still post) PASS. Real-hardware confirmation is M-212/M-213. |
| 6 | Carried-in M-05: window-share keys are dropped unless the shared window holds keyboard focus | ⚠️ PRESENT_BEHAVIOR_UNVERIFIED | `windowAllows` keys/text branch returns `window_key_focus` when `keyFocused` fails (`window.go:229-235`); real bridge reads `kAXFocusedUIElementAttribute` (`window_darwin.m:167`), fails closed on error. `TestWindowKeyFocus_*` (5 tests) pass on a fake bridge. G-11 PARTIAL: the panels available on macOS 27.2 took the focused app, so `raising` dropped the keys and `window_key_focus` stayed 0. Routed to M-214. |
| 7 | Carried-in AR-21-01: the app offers and connects to peer desktops and terminal peers of the viewer's own Tailscale account only | ✓ VERIFIED | `discoverPeers` keeps only `p.UserID == Self.UserID` and fails closed with no self identity (`tailnet.go:63-89`); daemon connect re-resolves against discovery before any cap is fetched. Tests `TestDiscoverPeers_OwnAccountOnly`, `TestDiscoverPeers_FailsClosedWithoutSelfIdentity`, `TestPeerConnect_RefusesHostMissingFromDiscovery`, `TestPeerDesktop_RefusesHostMissingFromDiscovery` pass. Live check with a real other-account node is M-207 (UNVERIFIED in the VM rows). |

**Score:** 5/7 truths verified (2 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/webserver/whois.go`, `desktop_guest_routes.go` | Tailnet-member gate, guest page/ws/check routes | VERIFIED | Wired from `server.go:753`; tests above |
| `internal/capability/desktopguesttoken.go` | Guest token, own HKDF key | VERIFIED | Used by `guestTokenOutcomeFor` |
| `internal/desktop/guestshare.go`, `guest_viewer.go` | Admission queue, limits, timeout, deny, re-admit | VERIFIED | 474 and 231 lines; `ServeGuestViewer` reached from the ws route |
| `internal/desktop/rights.go`, `gates.go`, `clipboard.go` | Per-person control and clipboard grants enforced daemon-side | VERIFIED | Single gate on every press |
| `internal/desktop/revoke.go`, `media_binding.go` | Ordered cut, concurrent Stop, media bound to admitted node | VERIFIED | WR-03/04/05 fixes present in HEAD |
| `internal/desktop/arbiter.go`, `lease.go`, `internal/screencap/inputtap_darwin.*` | Lease and physical-wins arbiter, listen-only tap | VERIFIED | Default `StartTap` is the real tap |
| `internal/desktop/audit.go` | JSON Lines audit log | VERIFIED | Opened by the daemon on first event |
| `internal/daemon/desktop_share_api.go` | Socket-only admit/deny/rights/disconnect/stop, GUI liveness, audit wiring | VERIFIED | Routes on the daemon socket (`:178-185`), none on the share server |
| `app_desktop_share.go`, `tray.go`, `tray_objc_darwin.m`, `assets/tray_icon_sharing*.png` | Bindings, tray line, Stop, icon variant, notifications | VERIFIED | Registered in `wailsjs/go/main/App.js` |
| `frontend/src/components/AdmitSheet.tsx`, `ShareDesktopSheet.tsx`, `ViewerList.tsx`, `ShareBanner.tsx` | Admit prompt, share sheet, watching list with grant switches, banners | VERIFIED | Mounted from `App.tsx:1289-1291,1475,1661` |
| `frontend/src/lib/desktopClient.ts`, `DesktopView.tsx` | Guest waiting, view-only, lease and close-reason states | VERIFIED | `4434: 'replaced'` and Control paused chip present |
| `internal/desktop/window.go`, `internal/screencap/window_darwin.m` | M-05 key-focus gate | VERIFIED (wired, behaviour unverified) | See truth 6 |
| `internal/tailnet/tailnet.go` | AR-21-01 own-account filter | VERIFIED | See truth 7 |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| Guest browser | guest admission | `GET /desktop/guest/ws` behind Origin, `requireTailnetPeer`, token, then `ServeGuestViewer` | WIRED | `desktop_guest_routes.go:103-104,211`; `TestGuestRoutes_TracerWS` |
| Daemon GuestShare events | GUI | `GET /desktop/share/events` stream held by the GUI (`startDesktopShareEvents`) | WIRED | Also GUI liveness; `TestGuiLiveness_*` |
| Admit / Deny buttons | daemon | `AdmitDesktopGuest` / `DenyDesktopGuest` bindings to `POST /desktop/share/requests/{id}/admit|deny` | WIRED | `app_desktop_share.go:413-418`; G-01 |
| Allow control / clipboard switches | Manager rights | `SetDesktopViewerRights` to `PUT /desktop/share/viewers/{id}/rights` to `Manager.SetRights` | WIRED | G-02 |
| Disconnect / Stop Sharing | ordered cut | `DisconnectDesktopViewer` / `StopDesktopShare` / tray `stopSharingFromTray` to `CloseViewer` / `CloseGuests` | WIRED | G-03, G-04, G-01 |
| Every press | arbiter, rights, window gate | `pressAllowed` | WIRED | `gates.go:49-75` |
| Viewer lifecycle and grants | audit file | `API.record` | WIRED | G-10 |
| Discovery | Desktop rows and connect | `discoverPeers` own-account filter, re-resolved on connect | WIRED | unit tests |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| AdmitSheet | `pending[0]` identity | WhoIs on the guest's connection, sanitized (`names.go`), sent on the events stream | Yes (G-01 sheet in AX) | FLOWING |
| Tray line | viewer count | `setTrayViewers(n)` from the events stream | Yes (G-08 "1 person") | FLOWING |
| Guest page chips | `rights`, `lease` messages | `SetRights`, `pushLease` | Yes (G-02 You have control, G-05 Someone is using this Mac) | FLOWING |
| Audit file | `AuditRecord` | daemon `record` calls | Yes (G-10, 147 lines) | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Guest gate, route table, cross-token | `GOTOOLCHAIN=go1.26.11 go test -run 'TestGuest\|TestTailnetPeer\|TestDesktopRoutes' -count=1 ./internal/webserver` | ok | PASS |
| Admission, rights, revoke, lease, physical, audit, media binding, names, key focus | `GOTOOLCHAIN=go1.26.11 go test -run 'TestArbiter\|TestLease\|TestPhysical\|TestGuest\|TestAudit\|TestMediaBinding\|TestNames\|TestWindowKeyFocus' -count=1 ./internal/desktop` | ok (25.2 s) | PASS |
| Rights and clipboard gate | `go test -run 'TestGuestRights\|TestClipboard_RightsLockAndBusy' ./internal/desktop` | ok | PASS |
| Daemon share API, audit, GUI liveness, AR-21-01 connect refusal | `go test -run 'TestShareAudit\|TestDesktopShareAPI\|TestGuiLiveness\|TestPeerConnect\|TestPeerDesktop' ./internal/daemon` | ok | PASS |
| Root bindings and tray titles | `go test -run 'DesktopShare\|Share\|Tray\|Quit' .` | ok | PASS |
| AR-21-01 discovery filter | `go test -run TestDiscoverPeers ./internal/tailnet` | ok | PASS |
| Frontend (one file per run) | `vitest run` AdmitSheet (21), ViewerList (16), ShareDesktopSheet (28), DesktopView.guest (25), desktopClient.guest (28) | all passed | PASS |
| Traceability paths | `bash tests/check-traceability-paths.sh` | OK: 340 paths | PASS |
| Review-fix commits in HEAD | `git cat-file -t` on 8d3b702e, 0c4df9a6, d67f1f91, 53b5b644, fe0c97fa, 18dc125e, b5a46789, f1805c78, adec3daf, 80b0a083, fa49eb00; `merge-base --is-ancestor 80b0a083 HEAD` | all commits, in HEAD | PASS |

Full suites were not run (the orchestrator runs regression). Nothing was run that starts Portty, captures the screen, taps input or contacts tailscaled or a 100.x address.

### Probe Execution

SKIPPED: the phase's probes are the Tart VM rig (`rig/p22-rig.sh`, `rig/axp22.swift`) that only runs inside the guests. Results are in `22-VM-CHECKS.md` with `evidence/G-01..G-11.txt`; I read them (including `evidence/G-04.txt`) and did not re-run them.

### Requirements Coverage

| Requirement | Source Plans | Description | Status | Evidence |
|-------------|--------------|-------------|--------|----------|
| DSHR-03 | 22-01, 06, 07, 10, 12, 13 | Link other tailnet people can open; terminal links stay own-account | NEEDS HUMAN (code and refusal half verified) | Truth 1; M-206, M-208 |
| DSHR-04 | 22-01, 02, 04, 05, 06, 07, 09, 10, 11, 12, 13 | Admit prompt with identity; view-only until control and clipboard granted per person | SATISFIED in code and VM; real-Mac M-210, M-211, M-216, M-220 deferred | Truth 2 |
| DSHR-05 | 22-01, 04, 05, 06, 07, 09, 10, 12, 13 | Stop or revoke at any time, connection ends at once, link dies with the share | SATISFIED in code and VM (G-03, G-09); G-04 PARTIAL, M-215, M-218, M-219 deferred | Truth 3 |
| DSHR-06 | 22-06, 08, 09, 10, 11, 12, 13 | Join/leave notifications, "shared with N" indicator, one-click Stop | SATISFIED in code and VM (G-07, G-08); M-209, M-217 deferred | Truth 3 |
| DSHR-07 | 22-02, 06, 12, 13 | Local audit log of joins, leaves, grants | SATISFIED | Truth 4 |
| DCTL-04 | 22-03, 05, 07, 11, 12, 13 | Last typer holds about 2 s; physical wins | SATISFIED in code, tests and VM stand-in; M-212, M-213 deferred | Truth 5 |

All six IDs appear in plan frontmatter and map to Phase 22 in REQUIREMENTS.md; no orphaned requirements. REQUIREMENTS.md keeps them `[ ]` Pending, which is right until the deferred items pass.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| Phase 22 source files (72 checked) | - | TBD, FIXME, XXX | none | Two regex hits are not markers (`desktopClient.ts:236` `\udXXX` in a comment, `TESTING.md:2105` `mktemp ... XXXXXX`) |
| Phase 22 source files | - | TODO, HACK, placeholder | none found | - |
| `internal/desktop/revoke.go` | 145-158 | `closeSocket` ceiling does not shorten coder/websocket's ~5 s `Close` wait while the read loop holds `readMu` | Warning | Rights, input and media are cut at once (security holds); only the socket and the viewer registration linger for a silent peer. Matches the G-04 straggler. Documented in 22-REVIEW-FIX "Found while fixing", left for a later change because Phase 23 edits `serveAdmitted` |
| `frontend/src/components/ShareDesktopSheet.tsx` | - | UI-REVIEW warnings 1-3 open: Secure Event Input banner hidden behind the open sheet (where control is granted); empty body during `checking`; status live region mounted with its text | Warning | Not fixed by 22-REVIEW-FIX (only UI warning #6, the uncaught bindings, was). G-06 shows the banner works when the sheet is closed |
| `frontend/src/components/ViewerList.tsx` | 99, 168, 176 | Grant caption name not bold; double announcement on join | Info | UI-REVIEW warnings 4-5, open |

### Gate Results Recorded

- Code review: 0 critical, 8 warnings WR-01..WR-08 plus UI #6 and IN-02, all fixed test-first and present in HEAD (`22-REVIEW-FIX.md`, `all_fixed`); every fix except UI #6 is marked "requires human verification". IN-03 (clipboard notice flash on reconnect) and IN-04 (audit lines written before the change) remain open, Info.
- Security: `threats_open: 0`, ASVS L1. T-22-02, T-22-13, T-22-16 are accepted-pending-real-check (AR-22-09) until M-206 and M-207 run.
- UI review: 18/24, code-only; light/dark, VoiceOver and real display unobserved.
- Decision coverage: not evaluated. `check.decision-coverage-verify` could not parse the CONTEXT.md decisions block (format mismatch, no D-NN ids); `{honored: 0, total: 0}` is not a count. Spot check by hand: the guest URL contract, own HKDF key, requireTailnetPeer narrowness, GUI-only admit with GUI liveness, audit format and path, listen-only tap, 2 s lease, M-05 and AR-21-01 decisions are all honoured in code.
- VM rows: G-01, G-02, G-03, G-05, G-06, G-07, G-08, G-09, G-10 PASS; G-04, G-11 PARTIAL; SC1 and AR-21-01 UNVERIFIED; Appendix B PARTIAL. Build under test was `dfa1a208`, before the review fixes.

### Human Verification Required

See the `human_verification` list in the frontmatter: M-206..M-220 (all 15 are in 22-UAT.md and TESTING.md Section 5), plus one item missing from 22-UAT.md, a real-app re-check of WR-01..WR-08 and IN-02. The VM proof cannot stand in for these: a second Tailscale account, a real non-activating panel, real keyboard and mouse hardware, a quiet machine for the two-guest Stop, a dark menu bar, VoiceOver, and the protocol and UI changes that landed after the VM build.

Observations from 22-VM-CHECKS.md with no matching M-NN (owner may want rows for them):
- A long-lived guest viewer once stalled with `status streaming` and no Reconnecting while bytes stopped (G-07 first attempt; not reproduced, not investigated).
- Guest Allow clipboard was never driven end to end in a VM or listed as an M-NN; it rests on unit tests (`TestGuestRights_ClipboardIndependentOfControl`, `TestClipboard_RightsLockAndBusy`).

### Gaps Summary

No gaps. Every artifact for the six requirements and both carried-in items exists, is substantive, is wired and passes its targeted tests. What remains is evidence code checks cannot give: the cross-account guest flow (SC1, DSHR-03), the live own-account peer filter (AR-21-01), the M-05 key-focus branch on a real non-activating panel, real-hardware physical-input checks, and the post-VM review fixes. Keep DSHR-03..07 and DCTL-04 Pending until M-206..M-220 and the review-fix re-check pass.

---

_Verified: 2026-11-27T21:39:52Z_
_Verifier: Claude (gsd-verifier)_
