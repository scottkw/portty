---
phase: quick-261127-lub
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - internal/desktop/revoke.go
  - internal/desktop/media_binding.go
  - internal/desktop/manager.go
  - internal/desktop/guestshare.go
  - internal/screencap/inputtap_darwin.go
  - internal/screencap/inputtap_darwin.m
  - internal/daemon/desktop_share_api.go
  - internal/daemon/client.go
  - frontend/src/lib/desktopClient.ts
  - frontend/src/lib/desktopShare.ts
  - frontend/src/components/DesktopView.tsx
  - frontend/src/components/ShareDesktopSheet.tsx
  - (matching *_test.go and __tests__ files)
autonomous: true
requirements: [quick-261127-lub]
---

# Phase 22 pre-ship ponytail-review fixes

Source: three parallel ponytail reviews of the Phase 22 diff (internal/desktop; daemon/web/app; frontend), run after the
code-review fixes (22-REVIEW-FIX.md). Only real bugs and clearly safe simplifications. Every behavior fix is test-first
(the new test fails before the change). Chesterton's Fence: do not delete anything whose reason you cannot state.

Phase 23 is executing concurrently on this branch (manager.go, viewer.go, guest_viewer.go, lease.go, arbiter.go,
DesktopView.tsx). Keep diffs tight; no refactors.

## Task 1: Go bug fixes (internal/desktop, internal/screencap, internal/daemon)

1. **Disconnect of an already-cut viewer skips the deny-list** (revoke.go CloseViewer / cutViewer). A viewer that was already
   cut (WR-05 replace cut, media cut, CloseViewerWith) lingers in the Watching list until its read loop ends; Disconnect on
   that ghost row calls cutViewer(deny=true), which returns early on v.cut, so RevokeUser never runs: the person can reload
   and is re-admitted with old grants. Fix: in CloseViewer for a guest, call v.gs.RevokeUser(v.Identity.UserID) up front,
   unconditionally, then cut the person's live viewers; remove cutViewer's deny parameter and update its call sites.
   Test: CloseViewerWith(id, replaced) then CloseViewer(id) => IsDenied(uid) true and a reconnect is refused (shape of the
   reviewer's scratch overlay test TestPTGhost in the scratchpad ov/ dir).
2. **Media check: transient lookup failure shows a final "the sharer ended your connection" card and writes a false revoke**
   (media_binding.go). Keep failing closed (still cut through cutViewer), but for kinds lookup_failed and no_pair close with
   a retryable code (websocket.StatusGoingAway, as the PeerConnectionStateFailed path does) and write NO revoked audit
   line. wrong_user / wrong_node keep 4432 (CloseGuestRevoked) and the revoked/media_identity_mismatch audit line. Tests in
   media_binding_test.go for both branches. (Fail-closed must hold: lookup_failed still cuts.)
3. **Input tap rebuilt on every input event** (inputtap_darwin.m pc_tap_state, reached via PhysicalState from arbiter.decide
   under the arbiter mutex). Add a non-rebuilding peek (read g_running/g_dead/CGEventTapIsEnabled only) used by
   PhysicalState; the rebuild stays only in the 1 s poll path (TapState used by pollTap). Do NOT start a real tap in tests
   (StartInputTap refuses under go test; keep it that way). Add a unit test if there is a Go seam for it; otherwise state
   in the summary that it is covered by build + existing tests only.
4. **Daemon answerRequest / deny / disconnect** (desktop_share_api.go): look the pending request up once at the top and
   return 404 early for an unknown id (removes the uid=0 path and repeated Pending() scans); when DenyGuest / CloseViewer
   fails after leave reasons were put into audit.cut, remove those reasons so a later normal leave is not logged as
   denied/revoked. Add a daemon audit test in desktop_share_audit_test.go: admit Other, connect OtherSecondNode
   (webservertest fake), deny or disconnect one, assert both leave lines carry the right reason (if the fake WhoIs makes the
   two-device daemon test impossible, as the code-fixer found for the media check, say so and cover it at the
   internal/desktop level instead).
5. Fix the stale comment at internal/daemon/client.go:54-55 ("there is no streaming endpoint on this client").

## Task 2: Frontend bug fixes

1. **Guest waiting at Admit never pings on wake** (desktopClient.ts onWake ~966-989, admission case ~820-826). Track a
   waiting flag (set on admission waiting, cleared on connect() and admitted); onWake pings when
   (status streaming || waiting) && transport. Test in desktopClient.guest.test.ts.
2. **Phone guests always see "View only"** (DesktopView.tsx chip chain). Use `phoneMode ? phoneRights?.control === true :
   canControl` for the control / lease / guest chip lines. Test in DesktopView.guest.test.tsx.
3. **Esc closes the share sheet while inert** (ShareDesktopSheet.tsx useDialogFocus onCancel). Ignore Esc while
   checking/starting (ref). Test.

## Task 3: Safe lean

1. DesktopView: use copy.GUEST_CHIP_LOCKED and delete the duplicate LOCKED_GUEST constant.
2. desktopShare.ts: delete the dead lastEvent / ShareEvent state and the desktopshare:event subscription (nothing reads
   lastEvent; the Go side keeps emitting). Grep first to confirm no reader.

Skipped on purpose (listed in SUMMARY): unused shareDesktopCopy exports (copy-contract mirror of Go/HTML strings; tests
pin them), fetchGuestCheck merge, the two name sanitizers, BarState.guest double guard (defense in depth), rightsMap reuse
in manager.go, endReason field, arbiter unused bits, heartbeat removal, doShare merge, desktopShareStatus dead cases,
duplicated state structs, whois.go near-copies.

## Verify

GOTOOLCHAIN=go1.26.11 go vet ./... ; go test -race -count=1 ./internal/desktop ./internal/daemon ./internal/webserver
./internal/screencap . (re-run known load flakes alone); gofmt; cd frontend && pnpm exec tsc --noEmit && pnpm vitest run;
bash tests/check-brand.sh; bash tests/check-traceability-paths.sh. TESTING.md: add new test names to Section 4 rows.
