---
phase: 22-click-to-share-for-tailnet-people
fixed_at: 2026-11-27T15:30:00Z
review_path: .planning/phases/22-click-to-share-for-tailnet-people/22-REVIEW.md
iteration: 1
findings_in_scope: 9
fixed: 9
skipped: 0
status: all_fixed
---

# Phase 22: Code Review Fix Report

**Fixed at:** 2026-11-27
**Source review:** .planning/phases/22-click-to-share-for-tailnet-people/22-REVIEW.md (plus 22-UI-REVIEW.md warning #6)
**Iteration:** 1

**Summary:**
- Findings in scope: 9 (WR-01..WR-08 from 22-REVIEW.md, and warning #6 from 22-UI-REVIEW.md; there were no Critical findings)
- Fixed: 9
- Skipped: 0
- Also fixed, because they sit in the same code: IN-01 (folded into WR-04) and IN-02 (its own commit). IN-03 and IN-04 are out of scope and were not touched.

Every fix was written test-first: the new or changed test failed before the change and passed after it. The fixer ran in the existing agent worktree (branch `worktree-agent-acfd7a7cf0e6c14c1`), so it did not create a second worktree, recovery sentinel or cleanup tail. All gates ran in that worktree (see Verification).

## Fixed Issues

### WR-01: A "replaced" close is retried, so two tabs of one device take each other's place forever

**Files modified:** `internal/desktop/guest_viewer.go`, `internal/desktop/manager.go`, `internal/desktop/testdata/signal-guest-v1.jsonl`, `internal/desktop/guest_admission_test.go`, `internal/desktop/guest_golden_test.go`, `frontend/src/lib/desktopClient.ts`, `frontend/src/lib/shareDesktopCopy.ts`, `frontend/src/components/DesktopView.tsx`, `frontend/src/lib/__tests__/desktopClient.guest.test.ts`, `frontend/src/components/__tests__/DesktopView.guest.test.tsx`
**Commit:** 8d3b702e
**Status:** fixed: requires human verification (protocol change)
**Applied fix:** A new final guest close code `CloseGuestReplaced = 4434` (reason `replaced`). Both places send it: the pending path (`ResultReplaced`) and the admitted path (the `attach` closeFn). `GUEST_CLOSE` maps 4434 to `replaced`, which is a final reason, so the page does not retry. The page shows a new card, **Opened in another tab** / "This share is open in another tab or window on this device. Reload this page to watch here instead.", with Reload Page. These strings are new (UI-SPEC has no replaced card) and live in `shareDesktopCopy.ts`. The golden line `close-replaced` is now 4434. Tests: `TestGuestViewer_NewKnockFromTheSameNodeClosesTheWaitingOne` and `TestGuestViewer_ReconnectIsAdmittedAtOnceAndTheOlderSocketCloses` now expect 4434. `TestGuestViewer_Golden` and the vitest golden test check the new code. The vitest close-reason table adds 4434. A DesktopView card test was added ("Opened in another tab").

### WR-02: Knocks are not rate-limited

**Files modified:** `internal/desktop/guestshare.go`, `internal/desktop/guestshare_test.go`, `frontend/src/components/AdmitSheet.tsx`, `frontend/src/components/__tests__/AdmitSheet.test.tsx`
**Commit:** 0c4df9a6
**Status:** fixed: requires human verification (logic change)
**Applied fix:** `GuestShare.Knock` applies a token bucket per user ID: a burst of 3, then one knock per 2 s. It is kept as one time per user (GCRA) on the share's injected clock. The check runs after the ended and denied checks, so a denied user still gets 4430. A knock over the limit returns `ErrShareFull`, so it closes 4429 `full` like the existing full path, with no event, no audit line and no window pull. In `AdmitSheet`, the 600 ms guard key is now login + device + run instead of the request ID, so a re-knock from the same device does not restart the guard. Tests: `TestGuestShare_KnocksAreRateLimitedPerUser`, and two vitest cases (a re-knock from the same person and device keeps the guard; the same login on another device restarts it).
**Not done:** coalescing `showWindow` in the GUI. The rate limit already bounds it to about one call per 2 s per person.

### WR-03: Deny and revoke are per person for future knocks but per device for live connections

**Files modified:** `internal/desktop/revoke.go`, `internal/desktop/revoke_test.go`, `internal/daemon/desktop_share_api.go`
**Commit:** d67f1f91
**Status:** fixed: requires human verification (logic change)
**Applied fix:** The person is the unit.
- `CloseViewer` of a guest cuts that viewer with the deny-list entry, then cuts every other live guest viewer with the same user ID (`closeGuestUser`, through `cutViewer`, 4432).
- A new `Manager.DenyGuest(gs, reqID)` denies the request, which deny-lists the user, and then cuts that user's live guest viewers. The daemon's deny route uses it whenever a Manager exists.
- The daemon gives each extra device's leave line the reason `revoked` or `denied` (`markPersonLeaves`).

No copy changed. "Denying or disconnecting someone is final for this share" and "Deny is final for this share" are now true for live connections too.
Tests: `TestGuestRevoke_CutsEveryDeviceOfThePerson` and `TestGuestDeny_CutsThePersonsLiveDevices`. Both run in `internal/desktop`, where each loopback guest joins with a matching media identity.
**Note:** I first tried these as daemon tests, using the `Other` and `OtherSecondNode` WhoIs identities. They were unreliable: the daemon fake answers the media check with whichever identity is current, so switching it to the second device cut the first one as `wrong_node`. That gave a false pass in one case and a false fail in the other. Only the test rig has this problem; it is not a product bug.

### WR-04: The media-identity cut skips the ordered revoke (+ IN-01)

**Files modified:** `internal/desktop/media_binding.go`, `internal/desktop/media_binding_test.go`, `internal/daemon/desktop_share_api.go`
**Commit:** 53b5b644
**Status:** fixed: requires human verification (security logic change)
**Applied fix:** `checkMediaIdentity` now cuts through `cutViewer`: rights, input end and flush, arbiter, `pc.Close()`, then the socket under `closeSocket`. The audit event is still emitted once (`mediaCut` CAS).

IN-01: a check that finishes after the viewer was cut, or after its PeerConnection closed, is no verdict. A new sentinel `desktop.ErrPeerLookupStopped` is returned by the daemon's `peerIdentity` when the share server is not running, and `mediaMismatch` treats it as no verdict.

Fail closed is kept for everything else:
- `lookup_failed` from any other error (including a selected-pair address tailscaled does not know, such as a non-tailnet address) still cuts;
- `no_pair` on a live PeerConnection still cuts;
- a missing seam still cuts.

The existing `TestMediaBinding_LookupErrorFailsClosed` and `_NoSelectedPairFailsClosed` still pass. The `kind` is still logged to daemon.log.
Tests: `TestMediaBinding_MismatchRunsTheOrderedCut` (rights are view-only when the check returns, and the guest's input data channel closes within 2 s while its socket never answers the close; before the fix both failed) and `TestMediaBinding_NoVerdictWhenTheShareIsStoppingOrTheViewerIsCut`.
**G-10 false positive (evidence/G-10.txt:30):** not diagnosed. The audit line does not carry the `kind`, and this fix does not guess at the cause. If the cause was a check finishing on a PeerConnection that was already closed, the new no-verdict rule covers it. If it was `lookup_failed` on a live pair, it still cuts, by design. The `kind` in daemon.log for that time would tell which.

### WR-05: A reconnecting guest waits on a synchronous close handshake of its own dead previous socket

**Files modified:** `internal/desktop/guestshare.go`, `internal/desktop/manager.go`, `internal/desktop/guestshare_test.go`, `internal/desktop/guest_admission_test.go`
**Commit:** fe0c97fa
**Status:** fixed: requires human verification (concurrency change)
**Applied fix:** `Knock` and `attach` now call the replaced connection's `closeFn` on its own goroutine. The admitted path's `closeFn` is now `m.cutViewer(v, CloseGuestReplaced, guestReasonReplaced, false)`. That cuts the old viewer first (rights, held input released, lease forgotten, pc closed) and then closes its socket through `closeSocket`. Before the fix, the new test measured the reconnect's hello at 5.0 s behind a silent old socket; now it is well under 2 s. Tests: `TestGuestShare_ReplacedConnectionClosesWithoutBlocking`, `TestGuestViewer_ReconnectIsNotHeldUpByASilentOldSocket`, and `TestGuestShare_ReadmitsTheSameKeyWithItsGrantsAndClosesTheOlderSocket`, which now waits for the async close.

### WR-06: The Admit sheet keeps keyboard focus on Deny or Admit when the next person replaces the one shown

**Files modified:** `frontend/src/components/AdmitSheet.tsx`, `frontend/src/components/__tests__/AdmitSheet.test.tsx`
**Commit:** 18dc125e
**Status:** fixed: requires human verification (UI logic change)
**Applied fix:** `AdmitSheet` shows `pending[0]`, the daemon's Seq order. The `oldest()` re-sort by whole-second `since` and random ID is gone. `AdmitSheetContent` is keyed per person (login + device), so focus returns to the dialog for each new person. It is keyed by person, not by request ID as the decision text said, because a request-ID key would remount on every re-knock and restart the guard, which undoes WR-02. Same login and device means the same person on the same device, so focus is still reset for every different person. Tests: "focus goes back to the dialog when the next person replaces the one answered from the keyboard" and "shows the first request in the daemon's (arrival) order, even when two knocked in the same second". The latter replaces "shows the oldest request first", which fed an order the daemon never sends.

### WR-07: Own-account viewers are never told about the lease or physical input

**Files modified:** `internal/desktop/arbiter.go`, `internal/desktop/lease.go`, `internal/desktop/lease_test.go`, `internal/desktop/guest_golden_test.go`, `internal/desktop/testdata/signal-guest-v1.jsonl`, `frontend/src/lib/desktopClient.ts`, `frontend/src/lib/shareDesktopCopy.ts`, `frontend/src/components/DesktopView.tsx`, `frontend/src/lib/__tests__/desktopClient.guest.test.ts`, `frontend/src/components/__tests__/DesktopView.guest.test.tsx`
**Commit:** b5a46789
**Status:** fixed: requires human verification (protocol + UI change)
**Applied fix:**
- The daemon already sent `lease` to own-account controllers (`pushLease` filters only on control). The client now passes `lease` on in both modes.
- In `DesktopView`, the **Someone is using this Mac** chip and the **<name> is controlling** chip (shown as "Someone is controlling" when an own-account device holds the lease) now show to every viewer that can control, as the UI-SPEC chip table says.
- New lease state `unavailable`. `arbiter.state` sets `LeaseState.Unavailable` once the tap has been unhealthy for 2 s. That is longer than the tap's 1 s settle after a start, so a normal start never flashes it. `leaseFor` sends it ahead of every other state.
- The chip for it is **"Control paused. Portty can't tell if someone is at the Mac."** UI-SPEC has no string for this state, so the string is new: `CHIP_CONTROL_UNAVAILABLE` in `shareDesktopCopy.ts`, marked as added by this fix.
- Golden line `lease-unavailable` added.

Tests: `TestPhysical_UnhealthyTapIsToldAsUnavailable`, an own-account lease test in `desktopClient.guest.test.ts`, an own-account chips test, and a guest Control paused chip test.

### WR-08: The Share Desktop sheet's Status row never shows a state when always-available is off

**Files modified:** `internal/daemon/desktop_api.go`, `internal/daemon/desktop_share_api_test.go`
**Commit:** f1805c78
**Status:** fixed: requires human verification (logic change)
**Applied fix:** `handleGetDesktop` fills State and the rest from the Manager when always-available is on **or** a guest share runs. `AlwaysAvailable` is always the config value. Other readers of this response stay correct with always-available off: the tray line (`desktopTrayLine`) and the Settings Status and Showing rows all gate on `AlwaysAvailable`, so they are unchanged. The GUI can now also claim an approval-lapse notification during a guest share; before, the daemon posted that notification itself, so it is posted once either way. Test: `TestDesktopShareAPI_StatusReportsTheGuestShareManager` (off before, the Manager's state during a guest share with always-available off, off after Stop).

### UI-REVIEW #6: StartDesktopShare / StopDesktopShare awaited without a catch

**Files modified:** `frontend/src/components/ShareDesktopSheet.tsx`, `frontend/src/components/__tests__/ShareDesktopSheet.test.tsx`
**Commit:** adec3daf
**Status:** fixed
**Applied fix:** Both bindings now catch a rejection and turn it into a failed answer (`errorCode: 'unavailable'`, `detail: String(err)`). The existing `shareDesktopStartError` and `shareDesktopStopError` copy shows with the cleaned detail, and the sheet leaves Getting a link… or the stopping state, so the action can be retried. Tests: "a rejected StartDesktopShare shows the start error and leaves Getting a link…" and "a rejected StopDesktopShare shows the stop error and leaves the stopping state". Before the fix both failed with an unhandled rejection.

### IN-02 (Info, in scope because it was small): `SetRights` does not check `v.cut`

**Files modified:** `internal/desktop/rights.go`, `internal/desktop/revoke_test.go`
**Commit:** 80b0a083
**Status:** fixed: requires human verification (concurrency change)
**Applied fix:** `SetRights` returns `ErrNoViewer` for a viewer being cut. If a cut races the swap, it puts view-only back under `rmu`. `cutViewer` sets `cut` before it clears the rights, so the cut always wins. Test: `TestGuestRevoke_GrantAfterTheCutIsRefused`.

### TESTING.md

**Commit:** fa49eb00. No test file was added or renamed. The new test names were appended to the existing Section 4 rows for each touched file. `bash tests/check-traceability-paths.sh` reports OK (340 paths).

## Found while fixing, not fixed

- **The `closeSocket` ceiling does not cut the wait short when the read loop is blocked in `Read`.** In coder/websocket v1.8.15, `CloseNow` called after `Close` has started is a no-op wait (`casClosing` is already set). `Close` itself waits up to 5 s for `readMu`, which the viewer's blocked read loop holds. A silent peer's viewer therefore stays registered for about 5 s, not the 2 s `closeCeiling`. This was measured in the WR-04 test and agrees with G-04 (one Stop took 6.9 s). WR-04, WR-05 and IN-02 make this harmless for security: rights, input and media are cut at once, and only the socket and the registration linger. A real fix needs a per-viewer read context that the ceiling cancels (a cancelled read context makes coder/websocket close the connection). That would touch `serveAdmitted` in `manager.go`, which Phase 23 is editing at the same time, so it is left for a later change.
- **IN-03 and IN-04** were not touched (out of scope). IN-04 now also covers a `SetRights` that IN-02 refuses: the grant audit line is already written when the route returns 404.

## Verification (run in the agent worktree, not the main checkout)

- `gofmt -l` on the touched packages: clean. `GOTOOLCHAIN=go1.26.11 go vet ./...`: exit 0.
- `GOTOOLCHAIN=go1.26.11 go test -race -count=1 ./internal/desktop ./internal/daemon ./internal/webserver .`:
  - `internal/daemon` and `internal/webserver`: ok.
  - `internal/desktop`: one failure in the full run, `TestSurfaceTracer_SelectWindowBumpsEpoch` ("2 sources built, want 1"). It is a load flake in capture-source counting, not touched by these fixes, and passed when re-run alone `-count=3`. Earlier full runs of the package after WR-07 were green.
  - root package: `TestNoFakeWhoIsInRelease` fails because this worktree has no `frontend/dist` (`assets_prod.go: pattern all:frontend/dist: no matching files found` under `-tags wailsassets`). That is a worktree environment gap and is not related to these changes. Every other root-package test passed.
- `cd frontend && pnpm exec tsc --noEmit`: exit 0. `pnpm exec vitest run` (whole suite): 152 files, 2777 tests passed.
- `bash tests/check-brand.sh`: clean. `bash tests/check-traceability-paths.sh`: OK.
- Nothing ran Portty, screen capture or a real input tap, and nothing contacted tailscaled or a 100.x address. Test addresses such as 100.64.0.9 are strings passed to a fake lookup.

---

_Fixed: 2026-11-27_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
