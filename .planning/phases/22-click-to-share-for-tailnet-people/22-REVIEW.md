---
phase: 22-click-to-share-for-tailnet-people
reviewed: 2026-11-27T20:47:59Z
depth: standard
files_reviewed: 113
files_reviewed_list:
  - app_desktop_share_test.go
  - app_desktop_share.go
  - app_update.go
  - app.go
  - assets/tray_icon_sharing.png
  - assets/tray_icon_sharing@2x.png
  - frontend/src/App.tsx
  - frontend/src/components/__tests__/AdmitSheet.test.tsx
  - frontend/src/components/__tests__/DesktopControlBar.guest.test.tsx
  - frontend/src/components/__tests__/DesktopPhoneBars.test.tsx
  - frontend/src/components/__tests__/DesktopView.guest.test.tsx
  - frontend/src/components/__tests__/QuitConfirmModal.test.tsx
  - frontend/src/components/__tests__/SettingsSheet.share.test.tsx
  - frontend/src/components/__tests__/SettingsSheet.test.tsx
  - frontend/src/components/__tests__/ShareBanner.test.tsx
  - frontend/src/components/__tests__/ShareDesktopSheet.test.tsx
  - frontend/src/components/__tests__/ViewerList.test.tsx
  - frontend/src/components/AdmitSheet.tsx
  - frontend/src/components/DesktopControlBar.tsx
  - frontend/src/components/DesktopPhoneBars.tsx
  - frontend/src/components/DesktopPhoneLayer.tsx
  - frontend/src/components/DesktopSendKeysMenu.tsx
  - frontend/src/components/DesktopView.tsx
  - frontend/src/components/QuitConfirmModal.tsx
  - frontend/src/components/SettingsRow.tsx
  - frontend/src/components/SettingsSheet.tsx
  - frontend/src/components/ShareBanner.tsx
  - frontend/src/components/ShareDesktopSheet.tsx
  - frontend/src/components/Switch.tsx
  - frontend/src/components/ViewerList.tsx
  - frontend/src/lib/__tests__/desktopClient.guest.test.ts
  - frontend/src/lib/__tests__/useDialogFocus.test.tsx
  - frontend/src/lib/desktopClient.ts
  - frontend/src/lib/desktopShare.ts
  - frontend/src/lib/shareDesktopCopy.ts
  - frontend/src/lib/tailscale.ts
  - frontend/src/lib/useDialogFocus.ts
  - frontend/src/style.css
  - frontend/src/wailsjs/go/main/App.d.ts
  - frontend/src/wailsjs/go/main/App.js
  - internal/capability/desktopguesttoken.go
  - internal/daemon/api.go
  - internal/daemon/client_desktop_share.go
  - internal/daemon/desktop_api.go
  - internal/daemon/desktop_share_api_test.go
  - internal/daemon/desktop_share_api.go
  - internal/daemon/desktop_share_audit_test.go
  - internal/daemon/peer_connect_test.go
  - internal/daemon/peer_desktop_test.go
  - internal/desktop/arbiter_test.go
  - internal/desktop/arbiter.go
  - internal/desktop/audit_test.go
  - internal/desktop/audit.go
  - internal/desktop/clipboard.go
  - internal/desktop/export_guest_test.go
  - internal/desktop/export_input_test.go
  - internal/desktop/export_lease_test.go
  - internal/desktop/gates.go
  - internal/desktop/guest_admission_test.go
  - internal/desktop/guest_golden_test.go
  - internal/desktop/guest_session_test.go
  - internal/desktop/guest_viewer_test.go
  - internal/desktop/guest_viewer.go
  - internal/desktop/guestshare_test.go
  - internal/desktop/guestshare.go
  - internal/desktop/input_test.go
  - internal/desktop/input.go
  - internal/desktop/lease_test.go
  - internal/desktop/lease.go
  - internal/desktop/loopback_test.go
  - internal/desktop/manager.go
  - internal/desktop/media_binding_test.go
  - internal/desktop/media_binding.go
  - internal/desktop/names_test.go
  - internal/desktop/names.go
  - internal/desktop/probe_vmdesktop.go
  - internal/desktop/revoke_test.go
  - internal/desktop/revoke.go
  - internal/desktop/rights_test.go
  - internal/desktop/rights.go
  - internal/desktop/surface_export_test.go
  - internal/desktop/surface.go
  - internal/desktop/testdata/signal-guest-v1.jsonl
  - internal/desktop/viewer.go
  - internal/desktop/window_keyfocus_test.go
  - internal/desktop/window_test.go
  - internal/desktop/window.go
  - internal/screencap/inputtap_darwin_test.go
  - internal/screencap/inputtap_darwin.go
  - internal/screencap/inputtap_darwin.m
  - internal/screencap/screencap_darwin_test.go
  - internal/screencap/window_darwin_test.go
  - internal/screencap/window_darwin.go
  - internal/screencap/window_darwin.m
  - internal/tailnet/tailnet_test.go
  - internal/tailnet/tailnet.go
  - internal/webserver/desktop_guest_crosstoken_test.go
  - internal/webserver/desktop_guest_gate_test.go
  - internal/webserver/desktop_guest_routes_test.go
  - internal/webserver/desktop_guest_routes.go
  - internal/webserver/desktop_routes.go
  - internal/webserver/server.go
  - internal/webserver/webservertest/whois.go
  - internal/webserver/whois.go
  - README.md
  - scripts/verify-icons.sh
  - TESTING.md
  - tray_objc_darwin.m
  - tray_test.go
  - tray.go
  - web/desktop-guest-link-invalid.html
  - web/desktop-guest-refused.html
  - web/embed.go
findings:
  critical: 0
  warning: 8
  info: 4
  total: 12
  necessity: 0
status: issues_found
---

# Phase 22: Code Review Report

**Reviewed:** 2026-11-27T20:47:59Z
**Depth:** standard
**Files Reviewed:** 113
**Status:** issues_found

## Summary

This review covers the Phase 22 changes only. The range `5bdeb6eb~1..HEAD` also contains Phase 19-21 and 23 work, so changed files were read through the Phase 22 commits (`bb341f1d` .. `e0ae21ac`). Audio, adaptive quality and unlock code in the same files (Phase 23) was not reviewed. Areas covered:
- the guest token, `requireTailnetPeer(Page)` and the `/desktop/guest/` routes
- `GuestShare` admission, `ServeGuestViewer` and `serveAdmitted`
- rights, revoke and Stop, the arbiter, lease and input tap, and media binding
- the audit appender and name sanitizer, and the daemon share API and GUI liveness stream
- the Wails bindings, tray and notifications
- the React share sheet, Admit sheet, Watching list, banners and guest page states
- the M-05 key-focus gate and the AR-21-01 peer filter

Test files were read only to judge coverage. `go vet` passes on all touched packages. The targeted `-race` tests for `TestGuest|TestMediaBinding|TestLease|TestPhysical|TestArbiter|TestAudit|TestNames|TestWindowKeyFocus` in `internal/desktop` and `TestDesktopShareAPI|TestGuiLiveness|TestShareAudit` in `internal/daemon` pass.

The core security path holds up. No path was found that lets a tailnet stranger reach media, input or clipboard without Admit and the matching grant:
- The guest token is HKDF-separated and bound to the share ID, and is checked again at join.
- WhoIs fails closed: a fault is 503, a refusal is 403 with the marker, and a tagged, shared-in, profile-less or zero-user node is refused.
- No PeerConnection exists before admission, and a waiting guest gets only ping.
- `pressAllowed`, `clipGate` and the surface handlers read the live rights atomic.
- `cutViewer` clears rights, ends input, flushes, then closes media and the socket.
- Stop drops the share ID before cutting guests.
- The GUI stream ending ends the share.

The findings fall into five groups:
- Replaced connections retry, and knocks have no rate limit. Both feed the Admit sheet, the window and the audit log.
- Deny and revoke work per person for later knocks but per device for live connections.
- The media-identity cut skips the ordered revoke.
- The Admit sheet does not move focus back to the dialog when the next person appears.
- Two spec gaps: own-account viewers get no lease or physical state, and the share sheet's Status row is dead when always-available is off.

No BLOCKER-class defect was found.

## Warnings

### WR-01: A "replaced" close is retried, so two tabs of one device take each other's place forever (waiting or admitted)

**File:** `frontend/src/lib/desktopClient.ts:363-370, 654-665`; `internal/desktop/guestshare.go:204-213, 236-240`; `internal/desktop/guest_viewer.go:152-153`
**Issue:** `GuestShare.Knock` gives one (user, node) key a single connection. A new knock from the same node either resolves the older pending request as `ResultReplaced` (closed `1000 replaced`), or, when the key is already admitted, closes the older live connection with `1000 replaced`. On the page, `GUEST_CLOSE` has no entry for 1000. Because the replaced socket already had a hello, `onClose` runs `retry()`. That reconnect knocks again and replaces the other tab, which then retries in turn. The 22-07 SUMMARY flagged this ("they will take turns replacing each other at the 1 to 10 s backoff") but it was never fixed. Each cycle has visible effects on the shared Mac:
- **Waiting:** every cycle is a new request ID. That fires `admit_request`, so `announceShareEvent` calls `showWindow()` and may post a banner, and it resets the Admit sheet's 600 ms guard (`guardKey` is `request.id:run`). It also appends two audit lines (`denied/replaced` and `admit_request`).
- **Admitted:** every cycle appends `leave` and `join` audit lines and posts join and leave banners. The coalescer reports a growing "N viewers joined/left" even though one person is going back and forth.

Over hours this fills `desktop-audit.jsonl` and rotates it. With only one `.1` file kept, older history is lost.
**Fix:** Make the replacement final for the page that lost it. Give it its own close code (for example 4434 `replaced`), add it to `GUEST_CLOSE` with a card such as "Opened in another tab", and send it from both places:
```go
// guest_viewer.go
case ResultReplaced:
    _ = c.Close(CloseGuestReplaced, guestReasonReplaced) // 4434, final on the page
// manager.go attach closeFn
v.gs.attach(key, v.ID, func() { go closeSocket(c, CloseGuestReplaced, guestReasonReplaced) })
```
Then update `testdata/signal-guest-v1.jsonl` and `desktopClient.guest.test.ts`.

### WR-02: Knocks are not rate-limited: someone with the link can keep the Admit sheet's buttons locked, pull the window forward over and over, and flush the audit log

**File:** `internal/desktop/guestshare.go:185-246`; `app_desktop_share.go:193-197`; `frontend/src/components/AdmitSheet.tsx:41-42, 51-54`; `internal/desktop/audit.go:157-159`
**Issue:** A knock costs one WebSocket and a hello, with no offer. A re-knock from the same node replaces the pending request with a new ID, and nothing limits how often this can happen. A tailnet member who holds the guest link (links are pasted into chats) can script re-knocks every few hundred milliseconds:
- The Admit sheet shows that request, and every new ID restarts the 600 ms guard. Neither Deny nor Admit ever becomes clickable, so the sharer cannot deny the person, and the deny-list never applies. The only way out is Stop Sharing.
- Every knock runs `a.showWindow()` on the shared Mac, which also appears on the shared screen, and posts or replaces a notification.
- Every knock appends two audit lines. At 5 MiB with one `.1` kept, a few minutes of this push earlier `join`, `control_granted` and similar lines out of the audit trail. An already-admitted guest can do the same from a second device after acting, so DSHR-07's record can be erased by the person it records.

Denied users are refused before any event fires, so this works only for someone the sharer has not yet refused.
**Fix:** Rate-limit knocks per user ID inside `GuestShare` (for example 1 per 2 s, with a small burst). Over the limit, close with `CloseFull` without firing an event or writing audit. Coalesce `admit_request` window-forwarding in the GUI to once per few seconds. In `AdmitSheet`, restart the guard only when the displayed person changes (key it on login + node), not on every new request ID from the same node.

### WR-03: Deny and revoke are per person for future knocks but per device for live connections, so "final for this share" is not true

**File:** `internal/desktop/revoke.go:24-35, 93-95`; `internal/desktop/guestshare.go:197-200, 332-364`; `internal/daemon/desktop_share_api.go:624-640`; copy `frontend/src/lib/shareDesktopCopy.ts:41-42, 55`
**Issue:** `CloseViewer` cuts one viewer and then calls `GuestShare.RevokeUser(userID)`. That deny-lists the person and forgets every stored admission of theirs, but it does not cut their other live connections. Two cases follow:
1. **Revoke leaves a second device connected.** The person is admitted on a laptop and on a phone, and the sharer clicks Disconnect on one row. The other device keeps watching, keeping whatever Allow control or clipboard grant it had. The sheet says "Denying or disconnecting someone is final for this share", the error copy says "Couldn't disconnect this person", and the audit line records `revoked` for the person.
2. **Denying a second device breaks the first device's reconnect.** The person is watching on a laptop and their phone knocks. The sharer denies the phone, perhaps meaning only "not a second device". `denyUserLocked` deny-lists the user ID, and `Knock` checks `denied` before the admitted-key re-admit path. The laptop's next network blip therefore ends with `4430 denied` and "You can't rejoin", which breaks the DESK-03 re-admit promise for a device the sharer never refused.

No test covers either case (`TestGuestRevoke_DenyListsTheUser` checks only new knocks).
**Fix:** Pick one model and enforce it in both directions. If the person is the unit (as the copy says), cut every live viewer with that user ID:
```go
func (m *Manager) CloseViewer(id ViewerID) error {
    v := m.lookupViewer(id)
    ...
    if v.Mode == ModeGuest {
        for _, o := range m.guestViewersOf(v.Identity.UserID) { // includes v
            m.cutViewer(o, CloseGuestRevoked, guestReasonRevoked, true)
        }
    }
```
Also make the Admit copy for a second device say that Deny refuses the person on every device. If the device is the unit, key `denied` by `AdmissionKey` and change the copy.

### WR-04: The media-identity cut skips the ordered revoke: rights, input and media stay live until the close handshake ends

**File:** `internal/desktop/media_binding.go:48-57`
**Issue:** When `mediaMismatch` finds a wrong user, a wrong node, a failed lookup or no pair, `checkMediaIdentity` does only two things: it emits an audit event and runs `go v.ws.Close(4432)`. It does not clear the viewer's rights, end its input, close `v.pc`, or put a ceiling on the close. With coder/websocket v1.8.15, `Close` writes the close frame (up to 5 s) and then waits up to 5 s for the peer's reply. Until the signalling read loop returns and `serveAdmitted`'s deferred `pc.Close()` runs, three things continue on the path just judged not to be the admitted node:
- the video track (and audio) keeps flowing;
- data-channel input is still accepted, gated only by the stored rights;
- clipboard requests are still served.

A peer that never answers the close holds this open for about 5-10 s. The CONTEXT decision is "the selected ICE pair's remote IP must equal ... or the peer is closed". The revoke path already does this correctly with `cutViewer` and `closeCeiling`.
**Fix:** Route the cut through the existing ordered path (rights, input, flush, arbiter, pc, socket with ceiling):
```go
if kind == "" || !v.mediaCut.CompareAndSwap(false, true) || v.cut.Load() {
    return
}
slog.Warn("desktop: guest media identity", "viewer", string(v.ID)[:8], "kind", kind)
v.gs.emit(GuestEvent{Kind: AuditRevoked, Reason: ReasonMediaIdentityMismatch, Ident: v.Identity})
m.cutViewer(v, CloseGuestRevoked, guestReasonRevoked, false)
```

### WR-05: A reconnecting guest waits on a synchronous close handshake of its own dead previous socket

**File:** `internal/desktop/guestshare.go:204-213, 419-434`; `internal/desktop/manager.go:848`
**Issue:** When an admitted key knocks again (DESK-03), `Knock` calls `old.closeFn()` synchronously on the new guest's goroutine, before the new socket's hello is sent. `attach` does the same for a recorded live viewer. `closeFn` is `c.Close(1000, "replaced")`, a full coder/websocket close handshake. In the usual reconnect case (Wi-Fi change, sleep, roaming) the old TCP connection is half-open, so the handshake waits out its 5 s read timeout, or longer if the write blocks. The reconnecting guest therefore sits about 5 s on "Connecting" before its hello arrives, against a 10 s client hello timeout. Meanwhile the old viewer stays registered with its rights and any lease it holds. `revoke.go` added `closeSocket` with a 2 s ceiling on its own goroutine for this exact reason, but the replacement path does not use it.
**Fix:** Do not block the knock. Close the old socket asynchronously with the ceiling. Ideally cut it first, so its held keys are released and its lease is forgotten before the new connection joins:
```go
// Knock / attach
if old != nil {
    go old.closeFn() // closeFn = func() { closeSocket(c, code, guestReasonReplaced) }
}
```

### WR-06: The Admit sheet keeps keyboard focus on Deny or Admit when the next person replaces the one shown, and may switch people mid-read

**File:** `frontend/src/components/AdmitSheet.tsx:16-23, 33, 49`
**Issue:**
1. `AdmitSheet` renders `<AdmitSheetContent request={oldest(pending)} />` with no `key`, so one component instance serves every queued person. `useDialogFocus` (deps `[active, initialFocus, ref]`) moves initial focus to the dialog only once, at mount. Suppose a sharer answers the first person from the keyboard (Tab to Admit, Space) while a second person is queued. The sheet re-renders with the second person and focus stays on the same Admit button. After the 600 ms guard, a stray Return or Space admits the second person. This is exactly what UI-SPEC Surface 3 ("focus starts on the dialog ... an accidental Return must not admit anyone") means to prevent. The tests check initial focus only for the first person (`AdmitSheet.test.tsx:72, 112`).
2. `oldest()` re-sorts by `since`, which the daemon formats at RFC 3339 *seconds*, and breaks ties by the random hex request ID. The daemon already sends `pending` in arrival (`Seq`) order. If two people knock within the same second, a newer request with a smaller ID takes the place of the person already on screen. The guard restarts, but someone partway through reading the first name can end up admitting the second.

**Fix:** Key the content per request so focus resets to the dialog for each person, and use the daemon's order:
```tsx
const request = pending[0] // daemon order is Seq order
return <AdmitSheetContent key={request.id} request={request} others={pending.length - 1} />
```

### WR-07: Own-account viewers are never told about the lease or physical input, though Phase 22 now drops their input for both

**File:** `frontend/src/lib/desktopClient.ts:829-833`; `frontend/src/components/DesktopView.tsx:1154`; `internal/desktop/gates.go:68-70`; `internal/desktop/arbiter.go:97-103`
**Issue:** Phase 22 put `decideLease` in the one press gate for every viewer, so the owner's own remote input is now dropped in three cases:
- `physical`: someone touches the Mac;
- `lease_held`: a guest typed within 2 s;
- `physical_unknown`: the tap is unhealthy or failed to start, and every press is dropped.

The daemon sends `lease` messages to own-account controllers (`pushLease`), but the client discards them outside guest mode (`case 'lease': if (guest && ...)`). The chip is also limited to guests (`guest && canControl && someoneUsing`). UI-SPEC's chip table says "Someone is using this Mac" is shown to "every viewer with a control grant (and own-account viewers)". The result is that the author, controlling the office Mac from home (the primary scenario and the core value), sees clicks and keys silently do nothing, with no explanation. In the `physical_unknown` case (tap start failure, logged once to daemon.log) all remote control is dead with no visible state at all, and `arbiter.state` does not even set `Physical` for an unhealthy tap (22-03 SUMMARY, "Issues for later plans").
**Fix:** Emit `lease` in both modes and show the physical and `other` chips to own-account viewers as the spec says. Add a distinct state for an unhealthy tap (for example `leaseUnknown = "unavailable"`, sent when `!r.Healthy`), so a controller sees "Control is paused: Portty can't tell whether someone is at this Mac" instead of silent drops.

### WR-08: The Share Desktop sheet's Status row never shows a state when always-available is off, the main guest case

**File:** `frontend/src/components/ShareDesktopSheet.tsx:130-143`; `internal/daemon/desktop_api.go:161-183`
**Issue:** While sharing, the sheet polls `GetDesktopStatus()` every 2 s to show `needs_permission`, `awaiting_approval`, `display_asleep` and `capture_paused` (with "Set Up Permissions"), so that the sharer "finds out first". `handleGetDesktop` fills `State` from the Manager only `if cfg.AlwaysAvailable`. Otherwise it returns `StateOff`, even when a guest share is running the Manager. With always-available off (the common click-to-share setup), `STATUS_ROWS['off']` is undefined and no Status row ever renders. A guest then sees "needs permission" or "display asleep" cards while the sharer's sheet shows nothing. The sheet tests mock `GetDesktopStatus`, so the gap is not caught.
**Fix:** Report the Manager's state whenever one runs, for example `if cfg.AlwaysAvailable || guestActive { ... mgr.Status() ... }`, keeping `AlwaysAvailable` truthful. Add a daemon test: a guest share with always-available off, where GET /desktop returns the Manager state.

## Info

### IN-01: The media check records "revoked / media_identity_mismatch" for guests who only left, or whose share is stopping

**File:** `internal/desktop/media_binding.go:33-41, 60-79`; `internal/daemon/desktop_share_api.go:958-966`
**Issue:** `checkSelectedPair` and `bindGuestMedia` run on their own goroutines with a WhoIs lookup of up to 3 s. If the PeerConnection closes first (the guest left, `cutViewer`, Stop), `GetSelectedCandidatePair` errors and the result is `no_pair`. If the share server has stopped, `peerIdentity` returns "the share server is not running" and the result is `lookup_failed`. Either way an audit `revoked` line with `media_identity_mismatch` is written for a guest who was never mismatched, and the check does not look at `v.cut`. These spurious lines make the audit log misleading.
**Fix:** Return early when `v.cut.Load()` or the pc is closed (`pc.ConnectionState() == webrtc.PeerConnectionStateClosed`), and treat "server not running" as no verdict rather than a mismatch.

### IN-02: `SetRights` does not check `v.cut`, so a grant can be re-armed on a viewer being cut

**File:** `internal/desktop/rights.go:30-61`
**Issue:** `cutViewer` stores view-only first, but a `SetRights` that runs after it swaps control or clipboard back on. The signalling read loop keeps serving `clip_get`/`clip_put` until the close finishes (up to `closeCeiling`), and `clipGate` would pass. `ViewerList` serializes switch and Disconnect calls per row, so this is hard to reach from the GUI. It is still an ordering hole in the daemon API.
**Fix:** In `SetRights`, return `ErrNoViewer` when `v.cut.Load()` is true. Check it again under `rmu` after the swap and undo the swap if a cut won the race.

### IN-03: Every guest reconnect flashes "Clipboard was turned off", then "You can now copy text..."

**File:** `internal/desktop/guest_viewer.go:122-126`; `frontend/src/components/DesktopView.tsx:620-631`
**Issue:** A guest's hello always carries view-only rights (`rightsMap(RightsViewOnly)`), and the real grants follow in a `rights` message after admission. The client emits `rights` for the hello too. For a guest with a clipboard grant, `prevClipboard` therefore flips true to false (an "off" notice) and back to true (an "on" notice) on every reconnect. The control chip also flickers to View only.
**Fix:** In guest mode, ignore the hello's rights (or ignore `rights` until `admission: admitted`). Alternatively, have the server leave `rights` out of a guest hello.

### IN-04: Audit lines for admitted and granted are written before the change, with nothing to correct them if the change fails

**File:** `internal/daemon/desktop_share_api.go:529-536, 597-617`
**Issue:** `answerRequest` writes `admitted` and then calls `gs.Admit(id)`. `handleShareRights` writes `control_granted` and similar lines and then calls `mgr.SetRights`. If the request was withdrawn or the viewer left in between (`ErrNoRequest`, `ErrNoViewer`), the route returns 404 but the log still says the person was admitted or granted control, and nothing records the correction. Writing before the change is the stated intent (a crash must not leave an unrecorded guest), but a failure leaves the record wrong.
**Fix:** When the operation fails after the line is written, append a compensating line, for example `denied` with reason `viewer_closed` for an admit that did not happen, or the opposite `*_revoked` for a grant that did not apply.

---

_Reviewed: 2026-11-27T20:47:59Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
