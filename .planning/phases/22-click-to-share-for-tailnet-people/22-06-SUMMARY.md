---
phase: 22-click-to-share-for-tailnet-people
plan: 06
subsystem: daemon
tags: [guest-share, events-stream, gui-liveness, audit, admit, whois]
requires:
  - phase: 22-01
    provides: DesktopGuestBackend, /desktop/guest/ routes, guest token
  - phase: 22-02
    provides: Identity, Audit, Audit* and Reason* constants
  - phase: 22-04
    provides: GuestShare, ServeGuestViewer, Deps.OnViewerJoin/Leave/PeerIdentity
  - phase: 22-05
    provides: SetRights, CloseViewer, CloseViewerWith, CloseGuests, SecureInput, AnyController
provides:
  - "POST/GET/DELETE /desktop/share, admit and deny, viewer rights and disconnect, all daemon-socket only"
  - "GET /desktop/share/events?gui=1: the single-holder NDJSON stream whose end is the GUI liveness signal"
  - "API.endGuestShare(reason): the one place a guest share ends (stopped, gui_gone, key_regenerated, daemon_exit)"
  - "desktopAdapter GuestShareID, ServeGuestViewer, ServeDesktopViewerAs"
  - "DaemonClient StartDesktopShare, DesktopShare, StopDesktopShare, AdmitDesktopGuest, DenyDesktopGuest, SetDesktopViewerRights, DisconnectDesktopViewer, StreamDesktopShareEvents"
  - "the audit appender wired to every join, leave, admission and grant, own-account viewers included"
  - "WebServer.LookupPeer; handleDesktopWS names own-account viewers through an optional ServeDesktopViewerAs"
affects: [22-07, 22-08, 22-09, 22-10, 22-11, 22-12, 22-13]
key-files:
  created:
    - internal/daemon/desktop_share_api.go
    - internal/daemon/client_desktop_share.go
    - internal/daemon/desktop_share_api_test.go
    - internal/daemon/desktop_share_audit_test.go
  modified:
    - internal/daemon/api.go
    - internal/daemon/desktop_api.go
    - internal/webserver/desktop_routes.go
key-decisions:
  - "An ending share cuts its guests and then WAITS (up to 3 s) for them to leave before it stops the Manager, because Manager.Close/CloseAll would otherwise put their own close code (4410, 1012) ahead of 4433 share_ended: CloseGuests returns before the socket closes finish"
  - "Regenerate Signing Key uses an own-account-only close after a guest share ended, for the same reason; with no guest share it keeps CloseAll as before"
  - "The audit file opens at the first event, not at API start, so a daemon that never shares writes nothing; an unopenable file is logged once and every later event is dropped"
  - "share_stop and the leaves of cut guests carry the share's end reason; the leave reason is set by whoever closes the viewer (revoked, share_stopped, key_regenerated, daemon_exit) and defaults to viewer_closed"
  - "The share_stop audit reason is the Reason* constant share_stopped (the plan prose says stopped); the constants are the persisted v1 vocabulary"
requirements-completed: [DSHR-03, DSHR-04, DSHR-05, DSHR-06, DSHR-07]
status: complete
estimate:
  tokens: 120000
  tasks: 3
actuals:
  tokens: 68000
  tasks: 3
  commits: 3
plan_head_before: a0b538578378149cdc272671cbb04c9135463abe
plan_head_after: 9efc672d590c01721dcb8ea1ec0637f9585f6043
completed: 2026-11-27
---

# Phase 22 Plan 06: Guest share in the daemon, GUI liveness stream and audit Summary

The daemon now runs the one memory-only click-to-share: it starts only while a GUI holds the events stream, ends the moment that stream drops, exposes every sharer action only on its own socket, and writes a local audit line for every join, leave, admission and grant (own devices included).

## What was built

- **Share life cycle (`desktop_share_api.go`).** `POST /desktop/share` follows `desktopOn`'s order (health and interface check through `desktopTailnet`, certificate warm-up, write gate, `startMu`), writes nothing to `desktop.json`, creates the `GuestShare`, brings the server and Manager up with `bringUpDesktopLocked`, records the CT acknowledgement and returns `{url, firstShare}`. A second POST returns the same link (the issued-at is kept, so the text is stable). `gui_required` (409) is checked before the slow work and again under `startMu`. A failed start undoes the share and a server it started for nothing.
- **Events stream.** NDJSON on the unix socket, one holder (second gets 409 `gui_busy` and its refusal does not end the first's share). State line on connect and on change (unchanged states are not repeated; at most 10 a second), event lines (`admit_request`, `join`, `leave`, `share_stop`) with the viewer and reason, a 15 s heartbeat, and a 1 s poll for Secure Event Input and `anyController`. No user ID, tailnet address or token is ever in a line; the link comes from `GET /desktop/share`. The stream's end calls `endGuestShare("gui_gone")`.
- **Ending a share (`endGuestShare`).** Drops the share ID first (link stops verifying), records `share_stop`, `GuestShare.End`, `CloseGuests` (4433), then, if always-available is off, waits for guests to leave and stops the Manager, and stops the server if nothing else needs it. Own-account viewers are untouched. Callers: `DELETE /desktop/share`, the stream's end, Regenerate Signing Key, `stopDesktop` (daemon exit).
- **Per-viewer routes.** `POST /desktop/share/requests/{id}/admit|deny`, `PUT /desktop/share/viewers/{id}/rights`, `DELETE /desktop/share/viewers/{id}`; unknown request or viewer is 404 with no state change and no audit line; rights on an own-account viewer is 409 `own_viewer`. None exists on the share server.
- **Other daemon behavior.** `serverNeededLocked`, `desktopWanted` and the poller count the guest share; `disableDesktop` with a running share closes only own-account viewers (4410) and keeps the Manager, server and guests; `desktopOff` treats "only a guest Manager running" as already off; the `freezeGate` exempts `POST /desktop/share` (it takes the gate itself after the slow check, like `PUT /desktop`).
- **Audit.** Lazy single appender. `admitted`, `control_granted`, `control_revoked`, `clipboard_granted`, `clipboard_revoked` and `revoked` are appended before the change is applied; only changing switches are logged. `join` and `leave` for guests (with share ID) and own-account viewers (`mode own`, empty share ID, WhoIs identity). `stopDesktop` flushes a leave for every viewer still on the books with reason `daemon_exit`.
- **Webserver.** `LookupPeer(ctx, addr)` for `Deps.PeerIdentity`; `handleDesktopWS` calls an optional `ServeDesktopViewerAs` with a lenient WhoIs identity (zero identity on any failure, never refuses).
- **Client.** The methods above; `StreamDesktopShareEvents` uses a copy of the HTTP client with no timeout (the client's 30 s bound would kill the stream), a 1 MiB line limit, returns nil when its context ends and `io.ErrUnexpectedEOF` when the daemon closes it; refusals come back as `*ShareError` with the daemon's code.

## Task commits

1. Task 1 (tracer): `818115c1` - GUI holds the stream, starts a share, a guest knocks and is admitted. Tracer gate re-run end to end (vet, gofmt, `-race -count=10` of `TestDesktopShareAPI_`, `TestDesktopAPI_|TestShareLifetime_` regression, full daemon and webserver suites, brand check) before Task 2 started.
2. Task 2: `b3f14cde` - stop, revoke, grants, every way a share ends.
3. Task 3: `9efc672d` - audit and own-account identities.

## Precondition record

Gate files 20-10, 21-06, 22-01 and 22-05 SUMMARY.md all present. `git log --oneline 07ad7d21..HEAD -- internal/daemon/desktop_api.go internal/daemon/api.go internal/daemon/client.go internal/daemon/share.go internal/webserver/desktop_routes.go` listed three commits, all read before editing: `ec2de385` (20-10 surface and control in the desktop status), `3fbed4ef` (21-06 peer desktop link route, freeze exemption and client methods), `e9cc6911` (21-06 loopback desktop proxy and POST /peers/desktop). The 21-06 freeze exemption for `/peers/desktop` shares the `freezeGate` condition I extended.

## Verification

- `go vet ./internal/...`, `gofmt -l internal/daemon internal/webserver` (empty), `golangci-lint run --new-from-rev=07ad7d21 ./internal/daemon/... ./internal/webserver/...` (0 issues), `go build ./...`.
- `go test -race -short -count=3` of `TestDesktopShareAPI_|TestGuiLiveness_|TestShareAudit_` passes; `go test -race -short -count=1 ./internal/daemon/ ./internal/webserver/... ./internal/desktop/` passes (every Phase 19, 20, 21 and 22 test unchanged, including `TestDesktopAPI_` and `TestShareLifetime_`).
- 12 `TestDesktopShareAPI_`, 3 `TestGuiLiveness_` and 7 `TestShareAudit_` tests (the audit group has two subtests). `bash tests/check-brand.sh` clean before each commit.
- Nothing touched the real screen, input taps, AX, the host tailscaled or any 100.x address: tests use the loopback desktop env, the synthetic source and the webservertest WhoIs fake.

## New test files for 22-13 (TESTING.md traceability)

- `internal/daemon/desktop_share_api_test.go` (DSHR-03 start/stop/link, DSHR-04 admit/rights/disconnect, DSHR-05 stop/regenerate/always-off, DSHR-06 GUI liveness)
- `internal/daemon/desktop_share_audit_test.go` (DSHR-07)
- Support only: none (helpers live inside those two files). Manual checklist: nothing new from this plan beyond the live-tailnet rows below.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Manager close races the guests' share_ended close code.**
- **Found during:** Task 2 (`TestDesktopShareAPI_StopCutsGuests` saw 4410 instead of 4433).
- **Issue:** `CloseGuests` returns once input and media are cut; the socket close runs on its own goroutine. The plan's "CloseAll then Close" on the Manager (and Regenerate's existing `CloseAll`) can send its own close frame first, so a guest would see 4410/1012/4401 instead of 4433.
- **Fix:** `endGuestShare` waits (bounded, 3 s, above the Manager's 2 s close ceiling) for guests to leave before stopping the Manager; Regenerate, after ending a guest share, closes only own-account viewers (`closeOwnViewers`); without a guest share it keeps the old `CloseAll`.
- **Commits:** `b3f14cde`.

**2. [Rule 3 - Blocking] Client methods and `LookupPeer` landed in Task 1, not Tasks 2 and 3.**
- The tracer test drives the share through `DaemonClient`, and `startDesktopManager` wires `Deps.PeerIdentity` through `WebServer.LookupPeer`, so both had to exist for Task 1 to compile and run. Their tests ride with the tasks that exercise them.

**3. [Rule 2 - Missing critical functionality] `stopDesktop` calls `endGuestShare` in Task 1, and flushes leaves in Task 3.**
- Without it the stream-close handler's `endGuestShare` could still be running after `Stop` returned and wrote into a removed TempDir. `stopDesktop` serialises with it through `startMu`.

**4. [Rule 2] `desktopOff` and `desktopTick` also consider the guest share.**
- `desktopOff`'s "already off" test now treats a Manager that only exists for a guest share as off (otherwise it would write an empty `desktop.json`); `desktopTick` uses the guest share for its "wanted" test so the poller follows an address change while a share runs (the plan names `desktopWanted` only).

**5. [Interpretation] Audit opens lazily, not at API start.** Avoids an empty audit file in every daemon that never shares and in every test config dir. The `OpenAudit(` grep acceptance still holds (in `desktop_share_api.go`).

**6. [Interpretation] `share_stop` reason is `share_stopped`** (the `Reason*` constant) rather than the prose word `stopped`.

## Known limits and risks

- **Re-admission race (audit).** `admitted` is appended before `GuestShare.Admit` runs (so a crash cannot leave a guest in without a record). If the request times out in the microseconds between the pending lookup and `Admit`, the log shows `denied (timeout)` and then a stray `admitted`; the API still answers 404.
- **`endGuestShare` takes `startMu`.** A hung start (certificate warm-up happens before `startMu`, but `bringUpDesktopLocked` is inside it) delays an end, including `Stop()` at daemon shutdown; `/shutdown` exits after its 500 ms grace regardless.
- **Stop with silent guests.** With always-available off, `DELETE /desktop/share` can take up to the 2 s close ceiling (it waits for guests to leave before stopping the Manager); the GUI's 2 s quit-time timeout (22-09) will then time out although the share is ending. The share_stop event and state line are pushed before the wait.
- **Upgrade handoff.** A guest share is not handed to the takeover daemon (memory only, per CONTEXT). `closeDesktopForHandoff` is untouched, so the old daemon's stream EOF is what tells the GUI (22-08 treats stream loss as share-ended, Pitfall 12); no `share_stop` audit line is written on that path, only the leaves from the hooks while the process lives.
- **Own-account events.** The events stream also carries `join`/`leave` for own-account viewers (`viewer.mode = "own"`, they are in the state's `viewers` too). 22-08/22-09 must filter on `mode` so an own device joining does not raise a guest notification.
- **Media-binding mismatch audit.** `revoked` with reason `media_identity_mismatch` is recorded from the share's event; its leave carries the default `viewer_closed` because the event has no viewer ID.

## Notes for downstream plans

- 22-08/22-09: `StreamDesktopShareEvents(ctx, fn)` returns nil on cancel, `*ShareError{gui_busy}` for a second holder, `io.ErrUnexpectedEOF` when the daemon closes. State line shape: `{type, active, viewers[], pending[], secureInput, anyController}`; viewer: `{id, mode, displayName, loginName, nodeName, control, clipboard, since}`; pending has the same fields and `id` is the request ID for admit and deny. The viewer ID in `viewers` is what rights and disconnect take.
- `GET /desktop/share` (`DesktopShareStatus`) returns the state plus `url` while active; `POST /desktop/share` is idempotent while a share runs.
- Error codes on the daemon socket: `gui_required`, `gui_busy`, `no_request`, `no_viewer`, `own_viewer`, plus `tailscale_off`, `https_off`, `tailscale_userspace`, `port_in_use`, `start_failed`.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register. T-22-21 (no remote admit/grant path): every new route is registered on the daemon mux only, and 22-01's route-table test still passes. T-22-22: audit-before-apply tests. T-22-23: `TestDesktopShareAPI_GuiRequired`, `TestGuiLiveness_*`. T-22-24: stop, regenerate, GUI-gone and exit tests plus the 410/401 checks on the old link. T-22-25: the tracer asserts no `cap=`, token, address or user ID in any stream line, and the audit test greps the file.

## UAT candidates (real Mac and real tailnet only)

- The real WhoIs on a real selected ICE pair (`LookupPeer` is faked by the webservertest WhoIs in every test).
- A real GUI closing (window quit, crash) with a guest watching: the unix-socket EOF ends the share within a second.
- A real upgrade handoff with a guest share running (the share ends; the GUI says so).

## Self-Check: PASSED

Created files exist (`desktop_share_api.go`, `client_desktop_share.go`, `desktop_share_api_test.go`, `desktop_share_audit_test.go`); commits `818115c1`, `b3f14cde` and `9efc672d` are ancestors of HEAD.
