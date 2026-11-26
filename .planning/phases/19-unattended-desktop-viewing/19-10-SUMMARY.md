---
phase: 19-unattended-desktop-viewing
plan: 10
subsystem: daemon
tags: [handoff, upgrade, always-available, share-listener, media-port, go]
status: complete

requires:
  - phase: 19-08
    provides: serverNeededLocked, desktopCfg, runDesktopPoller, desktopTick, the desktop test seams
  - phase: 19-09
    provides: Manager.Close clears captureActive (clean stop), runtime state file
provides:
  - "oldHandoff.global passes the share listener whenever serverNeededLocked holds (desktop-only included); wire format unchanged"
  - "takeover.finalize loads desktop.json and builds the server for a desktop-only handoff; no listener or no certificate defers the server to the poller"
  - "API.closeDesktopForHandoff: after COMMITTED the old daemon closes desktop viewers (1012), its Manager and its media socket"
  - "desktopWhenOldGone: the takeover brings the Manager up the moment the old daemon exits"
affects: [19-13, 19-15, 19-16]

actuals:
  tokens: 9000
  tasks: 2
  commits: 2
plan_head_before: 3ac1a7855d976d26fb7ce15791de4a07a9c52467
plan_head_after: a1691033c61ab4d4d2ff293305f44bb20dbe2b4e

tech-stack:
  added: []
  patterns:
    - "Desktop-only handoff reuses ShareListener with an empty Shares map: no gob change, HandoffVersion stays 1"
    - "Media socket never crosses the handoff; its new owner binds the same address and port after the old one closes it"

key-files:
  created:
    - internal/daemon/handoff_desktop_test.go
  modified:
    - internal/daemon/handoff.go
    - internal/daemon/desktop_api.go

key-decisions:
  - "A desktop-only handoff never fails for want of a certificate or a listener: the server is dropped and the poller rebuilds it once the old daemon exits. A share-record handoff still fails as before (its links would be lost)"
  - "The takeover runs one desktopTick as soon as watchOldExit fires, so the Manager is up in milliseconds after the old daemon exits instead of at the poller's next 10 s tick"
  - "A viewer that reaches a daemon whose desktop is on but whose Manager is not up yet is closed with 1013 (retry), not 4410 (final desktop_off in desktopClient.ts)"

requirements-completed: [DSHR-02, DESK-03, DESK-02]

coverage:
  - id: H1
    description: "With always-available on, two live sessions and one desktop viewer survive a v1 handoff: same PIDs, history is a prefix-extension of what it was, the share listener passes with zero terminal shares, the same link is byte-for-byte equal and answers 200 on the new daemon, the old viewer is closed with 1012, and a fresh viewer on the same link receives an IDR within 5 s"
    requirement: "DSHR-02"
    verification:
      - kind: integration
        ref: "internal/daemon/handoff_desktop_test.go#TestHandoffDesktop_SessionsAndLinkSurvive"
        status: pass
    human_judgment: false
  - id: H2
    description: "A v1.2-shaped old side (no listener) with desktop.json on commits; the takeover logs that the server starts later and the poller brings up the server, the Manager and a link that answers 200, sessions keep their PIDs"
    requirement: "DESK-03"
    verification:
      - kind: integration
        ref: "internal/daemon/handoff_desktop_test.go#TestHandoffDesktop_NoListenerFromOldSide"
        status: pass
    human_judgment: false
  - id: H3
    description: "A fault before COMMIT rolls back and leaves the old daemon's Manager the same object, the viewer connected and streaming (status streaming, 1 viewer, more video), and no close frame"
    requirement: "DESK-03"
    verification:
      - kind: integration
        ref: "internal/daemon/handoff_desktop_test.go#TestHandoffDesktop_RollbackKeepsDesktop"
        status: pass
    human_judgment: false
  - id: H4
    description: "While the old Manager holds the media socket the takeover's attempts report port_in_use and bind nothing; after the old one closes, the takeover binds the same loopback address and port (a second bind of that port is EADDRINUSE)"
    requirement: "DESK-02"
    verification:
      - kind: integration
        ref: "internal/daemon/handoff_desktop_test.go#TestHandoffDesktop_MediaPortRetry"
        status: pass
    human_judgment: false
  - id: H5
    description: "captureActive is true while streaming, false after COMMITTED, and the takeover's Manager records no abnormal exit"
    requirement: "DESK-03"
    verification:
      - kind: integration
        ref: "internal/daemon/handoff_desktop_test.go#TestHandoffDesktop_OldSideClearsCaptureActive"
        status: pass
    human_judgment: false
  - id: H6
    description: "global() passes no listener with nothing shared and the desktop off (3 fds), passes it for one terminal share (4 fds, 1 share), drops it when that share ends, and passes it for the desktop alone (4 fds, 0 shares)"
    requirement: "DSHR-02"
    verification:
      - kind: unit
        ref: "internal/daemon/handoff_desktop_test.go#TestHandoffDesktop_DesktopOffNoListener"
        status: pass
    human_judgment: false
  - id: H7
    description: "NOT run: the real two-build upgrade with a live browser viewer, a real tailnet address on 7444/udp and real capture. That is VM row work in 19-15"
    requirement: "DSHR-02"
    verification: []
    human_judgment: true
    rationale: "Host safety: no daemon, capture, launchd or tailnet on the host"

duration: 45min
completed: 2026-11-26
---

# Phase 19 Plan 10: Handoff with the desktop on Summary

**An upgrade with always-available on now keeps every terminal session and serves the same desktop link from the new daemon: the share listener passes with zero terminal shares (HandoffVersion still 1, no gob change), the old daemon closes desktop viewers with 1012 and releases the media port, and the takeover re-binds that same address and port the moment the old daemon exits.**

## Accomplishments

- **Listener rule:** `global()` passes the share listener when `ws != nil && serverNeededLocked()`, read under the same `a.mu.RLock` that reads the shares. Desktop-only is `ShareListener: true` with an empty `Shares` map.
- **finalize:** reloads desktop.json (an unusable file reads as off with the reason kept, as `NewAPI` does). With no imported shares and the desktop on, it builds the server through the shares path (`newServer`, `CheckListener`, `wireShareServer`). With no listener it logs "always-available server starts after the old daemon exits" and returns nil.
- **Old side:** after COMMITTED and `closeListenerCopies`, `closeDesktopForHandoff` takes the Manager under `a.mu`, nils it, then `CloseAll(1012, "server restarting")` and `Close` outside the lock. It runs beside the terminal viewers' drain (same `restartDrain` bound), never on a rollback path.
- **Takeover:** the poller was already started by `serveOn` (19-08). Its first tick happens while the old daemon still holds 7444/udp and reports `port_in_use`. `desktopWhenOldGone` runs one more tick when `watchOldExit` fires, so the Manager is up right after the old daemon exits.

## Task Commits

1. **Task 1 (tracer): listener pass, desktop-only finalize, old-side close, takeover bring-up, viewer retry code** - `e32ac7d8` (feat)
2. **Task 2: handoff edge tests** - `a1691033` (test)

## TDD Gate Compliance

Task 2 is `tdd="true"` but has no RED commit: the code it exercises was written with the Task 1 tracer (the same shape as 19-08's Tasks 2 and 3), so the five edge tests passed on first run. I did not trust that; each was shown to fail by mutating the code it covers, then restoring it (confirmed by a clean `git diff`):

- Removing the `closeDesktopForHandoff` call in `finish`: `SessionsAndLinkSurvive` ("old desktop viewer was never closed") and `OldSideClearsCaptureActive` fail.
- Calling it from `rollback`: `RollbackKeepsDesktop` fails (Manager dropped, viewer closed 1012).
- Making the no-listener branch of `finalize` return an error: `SessionsAndLinkSurvive`, `NoListenerFromOldSide` and `OldSideClearsCaptureActive` fail (handoff code 5).
- `needed` ignoring the desktop: `DesktopOffNoListener` fails at the last assertion, and the three handoff tests fail as above.
- `needed && len(g.Shares) > 0` (the old condition): `SessionsAndLinkSurvive` fails.

The git-log gate order (`test` before `feat`) is not met for Task 2; flagged here.

## Deviations from Plan

**1. [Rule 1 - Bug] A reconnecting viewer would have read "desktop off" as final**
- **Found during:** Task 1 design (reading `desktopClient.ts` and `desktopAdapter.ServeDesktopViewer`).
- **Issue:** the old daemon closes viewers with 1012, which the page treats as an immediate redial. The new daemon owns the listener at once but has no Manager until the old daemon exits and releases 7444/udp, and `ServeDesktopViewer` answered 4410, which the page treats as final (`desktop_off`). Viewers would not come back by themselves (DESK-03).
- **Fix:** with a grant on and no Manager, `desktopAdapter.ServeDesktopViewer` closes with 1013 (try again later); the page's existing retry path (`classify`, then backoff) handles it. 4410 still means off.
- **Files modified:** `internal/daemon/desktop_api.go` (not in the plan's `files_modified`; a four-line hunk in `desktopAdapter.ServeDesktopViewer`, away from the routes 19-11 edits). **Commit:** `e32ac7d8`

**2. [Plan interpretation] Desktop-only finalize degrades instead of failing**
- The plan says build the server "exactly like the shares case". A share-record handoff still fails when the certificate is unavailable or the listener is missing (its links would be lost). For the desktop-only case I drop the server and let the poller rebuild it, so a Tailscale or certificate hiccup at upgrade time cannot block moving the terminal sessions (the project's reliability-first priority). `CheckListener` failure (a foreign fd) still fails the handoff.

**3. [Plan interpretation] Takeover Manager start is event-driven**
- The plan only says to ensure the poller runs on the takeover daemon (it already does, from `serveOn`). With only the poller, a viewer would wait up to 10 s after the old daemon exits. `desktopWhenOldGone` runs the same `desktopTick` when `watchOldExit` fires.

**4. [Plan interpretation] `MediaPortRetry` at the daemon level**
- The daemon's turn-on path probes the UDP port first and answers `port_in_use` at once (19-08), so the 30 s `listenMedia` retry is not what the takeover waits on; the poller and the old-exit tick are. The test asserts that behaviour and the same-address-and-port binding (not the `listenMedia` loop, which 19-03 tests).

**5. [Instruction conflict] Commit trailer**
- The shared instructions say `Claude Opus 5.5`; the dispatch said to use exactly `Claude Sonnet 5.5`. I used the dispatch's (the accurate model).

**Total deviations:** 5 (1 bug, 3 interpretation, 1 note). None changes the wire protocol or a contract 19-13 to 19-16 read.

## Deferred Issues

- `golangci-lint run --new-from-rev=89d29266 ./internal/daemon/...` reports one `errcheck` (`internal/daemon/daemon_log.go:64`, `defer f.Close()`), from commit `91c154e3` (before this plan). Not touched here. The plan's Task 2 lint gate therefore exits non-zero until that line is fixed by its owner.

## Known Stubs

None.

## Threat Flags

None beyond the plan's register. T-19-35: protocol unchanged, listener passed under `serverNeededLocked`, existing `TestHandoff_*` and `TestTakeover*` pass, plus `SessionsAndLinkSurvive`. T-19-36: the takeover reads the grant only from desktop.json via `LoadConfig` (0600, validated) and the token from the same capability.key; nothing about the desktop crosses the socket. T-19-37: the old side closes its Manager and socket after COMMITTED and the new side binds only the same address and port. T-19-SC: no package added.

## Verification

- `go vet`, `gofmt -l` clean. `go test -race -count=1 -run 'TestHandoffDesktop_|TestHandoff|TestTakeover' ./internal/daemon/` passes; `TestHandoffDesktop_*` passes at `-race -count=5`; `go test -race -short -count=1 ./internal/daemon/` passes (102 s).
- Acceptance: `closeDesktopForHandoff` in handoff.go 3 (>= 2); `len(g.Shares) > 0` 0; `const HandoffVersion = 1` 1; no added `hsGlobal`/`hsShare`/`hsHello` struct definition and no added `HandoffVersion` line in the diff against 89d29266. Six `TestHandoffDesktop_` tests.
- Host safety: only synthetic sources, loopback sockets and in-process fakes ran; the real capture source refuses under a test binary. No VM, launchd, TCC or tailnet work.
- TESTING.md was not touched (19-16 owns the consolidated update). It should list `internal/daemon/handoff_desktop_test.go` and add a traceability row for DSHR-02, DESK-03 and DESK-02.

## Self-Check: PASSED

- Files exist: `internal/daemon/handoff_desktop_test.go`, modified `handoff.go` and `desktop_api.go`.
- Commits `e32ac7d8`, `a1691033` are ancestors of HEAD.
