---
phase: 22-click-to-share-for-tailnet-people
plan: 04
subsystem: desktop
tags: [admission, guest, websocket, webrtc, tailnet, whois]
requires:
  - phase: 22-02
    provides: Identity, NewIdentity, Audit* and Reason* constants
  - phase: 20-remote-control
    provides: input, clipboard and surface read-loop cases that serveAdmitted keeps
provides:
  - "GuestShare: in-memory admission queue (Knock, Admit, Deny, Pending, End, Await, Withdraw, SetGrants) with limits, 5-minute timeout, deny-list and re-admit rule"
  - "Manager.ServeGuestViewer: hello, admission waiting, ping-only wait, then the shared offer path as a view-only guest"
  - "serveAdmitted shared by both modes; ServeDesktopViewerAs; Deps.OnViewerJoin, OnViewerLeave, PeerIdentity"
  - "Viewer.Mode and Viewer.Identity"
  - "Media binding: selected ICE pair checked by WhoIs at Connected and on every pair change, fail closed"
  - "signal-guest-v1.jsonl golden file"
affects: [22-05, 22-06, 22-07, 22-10, 22-13]
tech-stack:
  added: []
  patterns:
    - "Request answered through a closed channel (subscribe-then-recheck by construction); events collected under the lock and fired after it"
    - "One websocket read kept in flight during the wait and handed to serveAdmitted, because a blocked read cannot be cancelled without closing the socket"
key-files:
  created:
    - internal/desktop/guestshare.go
    - internal/desktop/guest_viewer.go
    - internal/desktop/media_binding.go
    - internal/desktop/guestshare_test.go
    - internal/desktop/guest_viewer_test.go
    - internal/desktop/guest_admission_test.go
    - internal/desktop/guest_golden_test.go
    - internal/desktop/media_binding_test.go
    - internal/desktop/export_guest_test.go
    - internal/desktop/testdata/signal-guest-v1.jsonl
  modified:
    - internal/desktop/manager.go
    - internal/desktop/viewer.go
key-decisions:
  - "msgRights wire is {type: rights, rights: {control, clipboard, surface}}, the same map the hello carries"
  - "A re-admission (same key) skips the waiting message: hello, then admitted, then rights"
  - "Deny also refuses the same user's other waiting requests, not only the one clicked"
  - "A guest socket that drops while waiting is withdrawn and reported as denied/viewer_closed so the sharer's prompt vanishes"
  - "An identity with user ID 0 or an empty stable node ID, a nil GuestShare or a nil admit all close 4401 token_invalid (cannot key an admission: fail closed)"
requirements-completed: [DSHR-04, DSHR-05]
status: complete
estimate:
  tokens: 110000
  tasks: 3
actuals:
  tokens: 52000
  tasks: 3
  commits: 3
plan_head_before: 07ee3f2117af5d574a92c7c05a6debc0e1e93cf7
plan_head_after: 71de540f4e2bb3d3d2059624744046065b13ce70
completed: 2026-11-27
---

# Phase 22 Plan 04: Guest admission life cycle and media binding Summary

A guest knocks, waits at Admit with no PeerConnection and no offer accepted, and only after the sharer admits joins as a view-only viewer carrying its sanitized tailnet identity; its media path is tied to the admitted node by WhoIs on the selected ICE pair.

## What was built

- `guestshare.go`: `GuestShare` with ID (128-bit hex), `Knock`, `Await`, `Admit`, `Deny`, `Withdraw`, `Pending` (by monotonic Seq), `IsDenied`, `SetGrants` (control and clipboard only, never surface), `End`, `Ended`. Limits: 1 pending per (user, node) (newer replaces older), 2 per user (oldest replaced), 8 per share (ninth gets `ErrShareFull`, checked before anything is replaced). The 5-minute timeout runs off the injected clock and an injectable timer. Deny-list by user ID; admissions by `AdmissionKey{UserID, StableID}`; a reconnect with the same key is answered admitted at once with its stored grants and the older live socket is closed (`replaced`). Every event reaches `Notify` as `GuestEvent{Kind, RequestID, Reason, Ident}` using the 22-02 `Audit*` and `Reason*` constants.
- `guest_viewer.go`: `ServeGuestViewer`, close codes 4430 denied, 4431 admit_timeout, 4432 revoked, 4433 share_ended (plus 4401 token_invalid, 4429 full, 1000 replaced), `msgAdmission`, `msgRights`. While waiting only ping is answered; anything else closes with bad_message.
- `manager.go`: `serveAdmitted` (offer to read loop, extracted unchanged and keeping every Phase 20 read-loop case), `ServeDesktopViewerAs`, `Deps.OnViewerJoin/OnViewerLeave` (once per viewer, both modes, outside locks) and `Deps.PeerIdentity`. A guest whose token fails at join closes 4401 `token_invalid`; own-account keeps `token rejected` byte for byte.
- `viewer.go`: `Viewer.Mode` (`ModeOwn`, `ModeGuest`) and `Viewer.Identity`.
- `media_binding.go`: `bindGuestMedia` registers `OnSelectedCandidatePairChange`; the Connected state runs the same check. User mismatch, node mismatch, lookup error, missing pair and a nil `PeerIdentity` close the guest 4432 `revoked` and fire `revoked/media_identity_mismatch` once. Logs carry the viewer ID prefix and the mismatch kind only.
- `testdata/signal-guest-v1.jsonl`: guest hello, admission waiting and admitted, rights (view-only and control+clipboard), and every close reason.

## Task commits

1. Task 1 (tracer): `a4fb5d09` - guest knocks, waits, is admitted, watches view-only. Tracer verify re-run end to end (vet, gofmt, `-race -count=3`, full `./internal/desktop/` run) passed before Task 2 started.
2. Task 2: `b75fc488` - limits, timeout, deny-list, re-admit, golden file.
3. Task 3: `71de540f` - media binding.

## Precondition record

Gate files 20-01, 20-03, 20-04, 20-05 and 22-02 SUMMARY.md all present. `git log 07ad7d21..HEAD -- manager.go viewer.go input.go gates.go` listed (all read before editing manager.go; the extraction kept every case they added): `d47bd7eb` quick-261127-bb7 input channel close and flush helper, `efcc60d2` M-04 edge-clamped moves, `1dc75276` M-01 input mapping on the shown surface, `a8c3ceb0` H-01 release held input on quit, `65c8bda4` 20-05 Copy from Mac, `9f5809a6` 20-04 confine presses to the shared window, `40781a4d` 20-04 native window bridge, `bd7b4248` 20-04 raise a background shared window.

## Verification

- `go vet`, `gofmt -l` (empty), `golangci-lint run --new-from-rev=07ad7d21 ./internal/desktop/...` (0 issues).
- 25 `TestGuestShare_` and `TestGuestViewer_` tests plus 7 `TestMediaBinding_` tests, all pass with `-race -count=3`.
- `go test -race -count=1 ./internal/desktop/` passes after each task (every Phase 19 and 20 desktop test unchanged). `go vet ./internal/...` compiles every caller.
- `testdata/signal-v1.jsonl` and `input-v1.jsonl` untouched (git diff against the plan base is empty).
- `bash tests/check-brand.sh` clean before each commit. No real capture, taps, injection, tailscaled or 100.x address was touched; tests use the synthetic source, fakes and loopback.

## New test files for 22-13 (TESTING.md traceability, DSHR-04 and DSHR-05)

- `internal/desktop/guestshare_test.go`
- `internal/desktop/guest_viewer_test.go`
- `internal/desktop/guest_admission_test.go`
- `internal/desktop/guest_golden_test.go`
- `internal/desktop/media_binding_test.go`
- Support only: `internal/desktop/export_guest_test.go`; golden data `internal/desktop/testdata/signal-guest-v1.jsonl`.

## Deviations from Plan

**1. [Rule 3 - Blocking] media_binding.go committed with Task 1.** `manager.go` calls `bindGuestMedia` and `checkSelectedPair`, so the file had to exist for Task 1 to compile. Its tests and the media behavior review are Task 3's commit.

**2. Extra test files.** The plan lists `guest_viewer_test.go` and `media_binding_test.go`; the golden test is internal-package (it reads unexported message types and `readGolden`) so it lives in `guest_golden_test.go`, and the Task 2 socket cases are in `guest_admission_test.go` (`guest_viewer_test.go` holds the harness and the tracer). `export_guest_test.go` is the test-only seam file (timer, attach, media check), following `export_test.go`.

**3. [Rule 1 - Bug, found in test] Media cut close ran synchronously.** The first `checkMediaIdentity` closed the socket inline, which blocks up to 5 s waiting for the peer's close frame. Moved to a goroutine, as the existing "media failed" close does.

## Notes for downstream plans

- 22-05: `GuestShare.SetGrants(key, rights)` and the unexported `attach`/`detach` already exist (surface bit is stripped). The rights message shape is `{type: rights, rights: {...}}`; reuse `rightsMap` and `msgRights`. Viewer rights for a guest start at the stored grants masked to control and clipboard.
- 22-06: Notify kinds are `admit_request`, `admitted`, `denied` (reason `timeout`, `replaced`, `denied` or `viewer_closed`) and `revoked` (reason `media_identity_mismatch`; `RequestID` empty). A re-admission fires nothing from the share; the `join` audit comes from `OnViewerJoin`. Waiting requests resolved by `End` fire no event. `ServeGuestViewer` takes the `GuestShare` and the route's resolved `Identity`; `Deps.PeerIdentity` must be wired or every guest is cut at Connected.
- The admit-time wait is also bounded by `ctx` (the HTTP request context), so a route that cancels it withdraws the request.

## Known Stubs

None.

## Threat Flags

None beyond the plan's threat model. T-22-12 (no video before admission): `TestGuestViewer_TracerAdmit` asserts zero viewers while pending and an offer while waiting closes bad_message. T-22-13: media identity tests. T-22-14: limits, timeout and deny-list tests. T-22-15: `TestGuestViewer_SelectRefused`. T-22-16: re-admit and second-node tests.

## UAT candidates (real Mac and real tailnet only)

- The real selected-pair WhoIs lookup against live tailscaled (the lookup seam is faked in every test; `Deps.PeerIdentity` is wired in 22-06).
- A guest on another machine over a real tailnet: Connected check passes and media is not cut; and one from a node that is not the admitted one is cut. Pion's pair-change callback was exercised only through the test hook plus the real Connected-state check over loopback.

## Self-Check: PASSED

All created files exist; commits `a4fb5d09`, `b75fc488`, `71de540f` are ancestors of HEAD.
