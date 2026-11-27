---
phase: 22-click-to-share-for-tailnet-people
plan: 05
subsystem: desktop-input
tags: [rights, revoke, lease, arbiter, input-tap, window-gate, m-05]
requires:
  - phase: 22-03
    provides: arbiter (physical wins, 2 s lease) and the listen-only input tap
  - phase: 22-04
    provides: GuestShare, guest viewers, attach and detach, SetGrants
  - phase: 20-remote-control
    provides: the one press gate, input goroutine, window gate, clipboard handlers
provides:
  - "Manager.SetRights, rightsOf, ErrOwnViewer, ErrNoViewer: live per-guest control and clipboard switches"
  - "Manager.CloseViewer, CloseViewerWith, CloseGuests, GuestShare.RevokeUser: cut at once, in order, bounded by closeCeiling"
  - "decideLease in pressAllowed, the lease message, the tap life cycle, Manager.SecureInput and AnyController"
  - "WindowBridge.KeyFocus and the window_key_focus drop (Phase 20 review M-05)"
affects: [22-06, 22-07, 22-10, 22-11, 22-12, 22-13]
key-files:
  created:
    - internal/desktop/rights.go
    - internal/desktop/revoke.go
    - internal/desktop/lease.go
    - internal/desktop/rights_test.go
    - internal/desktop/revoke_test.go
    - internal/desktop/lease_test.go
    - internal/desktop/window_keyfocus_test.go
    - internal/desktop/guest_session_test.go
    - internal/desktop/export_lease_test.go
  modified:
    - internal/desktop/manager.go
    - internal/desktop/gates.go
    - internal/desktop/input.go
    - internal/desktop/clipboard.go
    - internal/desktop/surface.go
    - internal/desktop/guest_viewer.go
    - internal/desktop/guestshare.go
    - internal/desktop/window.go
    - internal/desktop/probe_vmdesktop.go
    - internal/desktop/testdata/signal-guest-v1.jsonl
    - internal/screencap/window_darwin.go
    - internal/screencap/window_darwin.m
key-decisions:
  - "A control-off, lease change or physical drop never posts from the caller's goroutine: SetRights queues a release item the input goroutine runs after any press it is handling, so a press that passed the right check just before the change cannot leave a button down"
  - "CloseGuests returns when every guest's input and media are cut; the socket closes run on their own goroutines with a 2 s ceiling then CloseNow, so three silent peers cost no time and the call returns in milliseconds"
  - "A guest that joins after End or a revoke fails a re-check made after it is registered (4433 share_ended, 4432 revoked), so neither a late join nor a racing admission is missed; the leave hook fires only for a viewer that joined"
  - "Lease messages are sent only after the answer, from one loop, once per change per recipient (deduped), never from the input goroutine"
  - "A physical or physical_unknown drop releases everything every viewer holds, not only the lease holder"
requirements-completed: [DSHR-04, DSHR-05, DCTL-04]
status: complete
estimate:
  tokens: 115000
  tasks: 3
actuals:
  tokens: 60000
  tasks: 4
  commits: 4
plan_head_before: 14977d9633389a9e8df2cea2997f58da9d146dc0
plan_head_after: d18a8e04f24922d5c52510dd039b002f7e3ffa75
completed: 2026-11-27
---

# Phase 22 Plan 05: Rights, revoke and the lease on the input path Summary

The sharer's per-guest control and clipboard switches, revoke and Stop, and the 2 s lease with physical-wins are enforced in the daemon's one press gate, and window-share keys now also require the shared window to hold keyboard focus (M-05).

## What was built

- **Rights (`rights.go`).** `viewer.rights` atomic, read through `rightsOf` by `pressAllowed`, both clipboard handlers and the surface handlers. `SetRights(id, control, clipboard)` sets the two bits independently, never grants surface, refuses an own-account viewer (`ErrOwnViewer`) and an unknown ID (`ErrNoViewer`), stores the grants on the admission (a reconnect returns with them), forgets the viewer in the arbiter and queues a release when control goes off, and sends `{type: rights}`. `ViewerList` reports the live rights.
- **Revoke and Stop (`revoke.go`).** `cutViewer` runs once per viewer: rights cleared, input life ended and the input goroutine flushed (so an in-flight message finishes and nothing starts after), arbiter forget, `GuestShare.RevokeUser` (deny-list, pending requests refused, stored admissions forgotten), `pc.Close`, tap sync, then the socket close on its own goroutine with `closeCeiling = 2 s` and `CloseNow`. `CloseViewer` uses 4432 `revoked` plus deny-list for a guest and 1001 `disconnected` for an own-account device. `CloseViewerWith(id, code, reason)` is the same with the caller's code and no deny-list (22-06's own-account close needs it). `CloseGuests(code, reason)` cuts every guest concurrently and leaves own-account viewers.
- **Lease (`lease.go`).** `decideLease` sits in `pressAllowed` after every existing drop check and immediately before `takeToken`. A physical or physical_unknown drop releases every viewer's held input; a lease change releases the previous holder first. `{type: lease, state, name}` goes to each controlling viewer (you, other with a guest holder's display name, owner for an own-account holder shown to guests, physical, free), sent after the answer and only when that viewer's state changed. View-only guests get none.
- **Tap life cycle.** `Deps.Physical`, `StartTap`, `StopTap`, `TapState`, `SecureInputOn` seams (defaults are the screencap functions; `StartInputTap` still refuses under tests). `syncTap` starts the tap synchronously when the first controller appears (an own-account viewer joins, a guest is granted control) and stops it when the last leaves or loses control; a start that fails is retried by the one-second poll. The poll also reads `TapState` and Secure Event Input. `Manager.SecureInput()` and `Manager.AnyController()` expose the two facts.

## Task commits

1. Task 1 (tracer): `b570ae27` - rights and `SetRights`. Tracer gate re-run end to end (vet, gofmt, 7 `TestGuestRights_` tests `-race -count=3`, full package `-race`) and passed before expanding.
2. Task 2: `76bc8b51` - revoke and Stop.
3. Task 3: `88e098af` - lease, physical-wins, lease message, tap life cycle.
4. M-05 task: `d18a8e04` - window-share key focus gate.

## Precondition record

Gate files 20-02, 20-04, 20-05, 22-03 and 22-04 SUMMARY.md all present; `RightsClipboard` present in `clipboard.go`. `git log --oneline 07ad7d21..HEAD -- internal/desktop/gates.go clipboard.go manager.go input.go` listed, all read before editing: `a4fb5d09` (22-04 guest wait at Admit), `d47bd7eb` (quick-261127-bb7 input channel close and flush helper), `efcc60d2` (M-04), `1dc75276` (M-01), `a8c3ceb0` (H-01), `6c11d3b8` and `65c8bda4` (20-05 clipboard), `9f5809a6`, `40781a4d`, `bd7b4248` (20-04 window gate).

## Where the arbiter sits in pressAllowed

Order is rights, lock, epoch, accessibility, then (window share only) window chord and window gate, then `decideLease`, then `takeToken`. No deviation from the plan. A press the lock, epoch, accessibility or window checks drop never takes the lease (tested). A press the arbiter allows can still be dropped by the rate token; it has then taken the lease, which is intended (the sender is controlling).

## M-05

- **Fix.** `WindowBridge.KeyFocus() (pid, x, y, w, h, ok, err)` reads the system-wide AX focused UI element (`kAXFocusedUIElementAttribute`), its owner pid (`AXUIElementGetPid`) and the frame of its `kAXWindowAttribute` window; if the element has no window attribute but is itself an `AXWindow` its own frame is used; no element, no window or any error is not ok (fail closed). Implemented in `internal/screencap/window_darwin.go` and `window_darwin.m` (`pc_ax_key_focus`, wrapped in `@autoreleasepool` and `@try`, refuses under `testing.Testing()`). In `windowAllows` the key and text branch now also requires `keyFocused`: the pid must be the shared window's and the frame within the 2 pt tolerance of the freshly read shared frame. Otherwise the message drops with the new counted reason `window_key_focus`. No raise starts: a panel on top is the user's own local action, and raising the shared window would take focus away from what the person at the Mac just opened. It applies to every viewer, own-account included, through the one code path. Releases never reach `pressAllowed`, so a held key is still let go.
- **Commit.** `d18a8e04`. The tests were written first against a fake with the new `KeyFocus` method and the gate unchanged: the panel, fail-closed (five variants) and held-key cases failed (drops 0 where 1 or 3 expected, keys posted), then passed with the gate change.
- **Tests** (`internal/desktop/window_keyfocus_test.go`): `TestWindowKeyFocus_PanelHoldsFocusDropsKeys` (keys, text and a repeat drop, nothing posts, no raise, a click still posts), `TestWindowKeyFocus_SharedWindowFocusedPasses` (including 1 pt off), `TestWindowKeyFocus_FailsClosed` (error, no element, other process, other process same frame, same app other window), `TestWindowKeyFocus_ReleaseOfHeldKeyStillPosts`, `TestWindowKeyFocus_DisplayShareNeverReadsIt`. `TestWindowBridge_RefusesUnderTest` now covers `KeyFocus` and `TestObjC_EntryPointsDrainAutoreleasePool` lists `pc_ax_key_focus`. The `fakeWindow` in `window_test.go` gained `KeyFocus`, which follows `Focus()` unless a test overrides it, so every Phase 20 window test passes unchanged.
- **Live verification is deferred** (VM or real-Mac item, 22-12): share a window, open a non-activating panel such as Spotlight locally while a viewer types into the window share, confirm the keys are dropped (`window_key_focus` counted, nothing typed into the panel), close the panel and confirm keys flow again. Never run on this host. Also check an app whose focused element reports no window (for example some custom views) does not make legitimate typing drop; that would show as `window_key_focus` while typing into the shared window.

## Verification

- `go vet ./internal/...`, `go vet -tags vmdesktop ./internal/desktop/`, `gofmt -l` (empty), `golangci-lint run --new-from-rev=07ad7d21 ./internal/desktop/... ./internal/screencap/...` (0 issues).
- `go test -race -count=1` of `./internal/desktop/ ./internal/screencap/` passes after every task; `-race -count=3` of the new test groups passes. 7 `TestGuestRights_`, 7 `TestGuestRevoke_`/`TestGuestStop_`, 8 `TestLease_`/`TestPhysical_`, 5 `TestWindowKeyFocus_` tests.
- Acceptance greps met: `rightsOf(` in gates.go and clipboard.go, `v.Rights&Rights` absent from gates.go, `CloseNow` and `endViewerInput(` in revoke.go, `decideLease(` in gates.go, `physical_unknown` in lease_test.go, five `"type":"lease"` golden lines, `SecureInput()` in lease.go.
- `bash tests/check-brand.sh` clean before each commit. Nothing touched real capture, the input tap, Accessibility, injection, the host tailscaled or any 100.x address: every test uses fakes and loopback, and the real `KeyFocus` refuses under tests.

## Existing tests changed

- `TestInputTracer_KeyHeldReleasedOnClose` (`input_test.go`): the second viewer now starts after the fake clock passes the first viewer's lease. Before this plan two own-account viewers on a stopped clock both posted; now the lease correctly holds the second back.
- Test helpers that set `Viewer.Rights` directly (`surface_export_test.go`, `input_test.go` no_right row) also set the live atomic. `newTracerServer` and `newGuestEnv` inject a healthy `Physical` source so existing input tests are not dropped as physical_unknown.

## New test files for 22-13 (TESTING.md traceability; DSHR-04, DSHR-05, DCTL-04)

- `internal/desktop/rights_test.go` (DSHR-04)
- `internal/desktop/revoke_test.go` (DSHR-05)
- `internal/desktop/lease_test.go` (DCTL-04)
- `internal/desktop/window_keyfocus_test.go` (M-05; window-share input safety)
- Support only: `internal/desktop/guest_session_test.go`, `internal/desktop/export_lease_test.go`. Golden data: `testdata/signal-guest-v1.jsonl` gained five lease lines (`TestGuestViewer_Golden` checks them).
- Manual checklist item for TESTING.md Section 5: the M-05 live check above.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Releases from the caller's goroutine could strand a button**
- **Found during:** Task 1 design.
- **Issue:** the plan says `SetRights` calls `releaseHeld`, which posts through the bridge. The bridge is documented as the input goroutine's alone, and a press already past the right check could record a hold after the release took an empty snapshot.
- **Fix:** `release` gained an `rv` field; `SetRights` queues it and the input goroutine runs `releaseHeld` after the press it is handling. The same goroutine runs the lease and physical releases (`decideLease` is called from there).
- **Files modified:** `input.go`, `rights.go`.
- **Commit:** `b570ae27`.

**2. [Rule 2 - Missing critical functionality] Post-registration re-check for joins racing Stop or revoke**
- **Found during:** Task 2.
- **Issue:** a guest registered after `CloseGuests` took its snapshot, or one of a just-revoked person's other admitted devices, would join and live on.
- **Fix:** `serveAdmitted` re-checks `GuestShare.Ended()` and `IsDenied` after registration (4433, 4432), and `OnViewerLeave` fires only for a viewer whose join hook fired.
- **Files modified:** `manager.go`. **Commit:** `76bc8b51`.

**3. [Rule 2 - Missing critical functionality] Physical drops release every viewer, not only the lease holder**
- **Found during:** Task 3.
- **Issue:** the arbiter returns only the lease holder; a viewer whose lease had lapsed but who still held a key would keep it down while the person at the Mac typed.
- **Fix:** `decideLease` releases everything every viewer holds on `physical` and `physical_unknown`.
- **Commit:** `88e098af`.

**4. [Interpretation] Counting, `SetGrants` shape, and CloseViewer API**
- `decideLease` returns the reason and the existing `allowed()` counts it once (the plan text says decideLease counts; counting in both would double). `GuestShare.SetGrants` already takes a `Rights` value (22-04), so it was reused as is. `CloseViewerWith` was added beyond the plan's artifact list because 22-06 needs a caller-chosen code for own-account viewers.
- Lease messages are sent only after the answer and by the lease loop, because a send from the input goroutine could stall input behind a slow socket.

## Issues for later plans

- **22-06:** call `GuestShare.End` then `CloseGuests` (which returns before the socket closes finish; `Manager.Close` or `CloseAll` still waits for them). Audit records (`revoked`, `control_granted`, and so on) are 22-06's: `SetRights` and `CloseViewer` do not record anything. `RevokeUser` fires `denied` events for the person's pending requests, as `Deny` does.
- **22-11/22-12:** `Manager.SecureInput()` and `AnyController()` are ready for the GUI warning. The tap runs only while a controller is connected, so VM rows must have one connected; the `vmdesktop` probe's pseudo-viewer is never in `Manager.viewers`, so with the real tap its input is dropped as `physical_unknown` unless a controlling viewer is also connected (or the probe is given a `Deps.Physical` seam). Phase 20 probe rows that drive input through the probe alone need that adjustment.
- **22-12:** live rows for the real tap, Secure Event Input and the M-05 panel check; the stored `lease` state is not set by an unhealthy tap (viewers see `free` while input silently drops as `physical_unknown`), as noted in 22-03.
- The lease messages are also sent to own-account viewers (additive; the existing page ignores unknown types).

## Known Stubs

None.

## Threat Flags

None. The only new surface is `KeyFocus`, a read-only Accessibility call covered by T-22-17, and it narrows what input can reach.

## Self-Check: PASSED

All created files exist and commits b570ae27, 76bc8b51, 88e098af and d18a8e04 are on the branch.
