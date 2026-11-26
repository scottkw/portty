---
phase: 19-unattended-desktop-viewing
plan: 08
subsystem: daemon
tags: [desktop-json, daemon-api, share-server-lifetime, health-poller, capability-token, go]
status: complete

requires:
  - phase: 19-01
    provides: desktop.Manager, desktop token, webserver.DesktopBackend and SetDesktopBackend
  - phase: 19-02
    provides: launchdManaged flag and GET /health launchd field
  - phase: 19-03
    provides: tailnet-only media bind, Manager.Full, viewer cap 8
  - phase: 19-04
    provides: /desktop/ route gates, 401/410 refusals, 429 before upgrade
  - phase: 19-05
    provides: screencap.NewDisplaySource
provides:
  - "internal/desktop/config.go: desktop.json (0600, schema 1, grantID, iat) with LoadConfig, SaveConfig (temp plus rename), NewGrantID, ErrConfigUnsafe"
  - "daemon socket routes GET/PUT /desktop and GET /desktop/link, DesktopStatusResponse, DesktopResponse, client methods DesktopStatus, SetDesktopAlwaysAvailable, DesktopLink"
  - "serverNeededLocked: the share server runs exactly while a terminal share or always-available needs it"
  - "desktopAdapter (DesktopGrant, ServeDesktopViewer, Full) wired into every share server through wireShareServer"
  - "runDesktopPoller: starts the server and Manager at daemon start with no GUI, retries every 10 s, restarts both on an address or name change"
  - "ShareErrUserspace (tailscale_userspace), SetDesktopNotifier, SetDesktopSeamsForTest"
affects: [19-09, 19-10, 19-11, 19-13, 19-14, 19-16]

actuals:
  tokens: 19000
  tasks: 3
  commits: 3
plan_head_before: f8f4953b09919cdec0c652e1ac65d926a3beee79
plan_head_after: defc86d60f5f5af2892273c9bf1c9a00a09a03f5

tech-stack:
  added: []
  patterns:
    - "Setting saved before anything starts, so a terminal unshare racing a desktop turn-on finds the server needed"
    - "One bring-up path (bringUpDesktopLocked) for the PUT and the poller, under writeGate then startMu"
    - "A busy media port is probed with a throwaway UDP bind so the answer is immediate; Manager.Start is given the daemon-lifetime context"

key-files:
  created:
    - internal/desktop/config.go
    - internal/desktop/config_test.go
    - internal/daemon/desktop_api.go
    - internal/daemon/desktop_api_test.go
    - internal/daemon/share_lifetime_test.go
  modified:
    - internal/daemon/share.go
    - internal/daemon/api.go
    - internal/daemon/client.go
    - internal/daemon/types.go

key-decisions:
  - "The Manager is started with the API's own long-lived context, not a 5 s one: Manager.Start keeps that context as the capture lifetime, so a 5 s context would end capture 5 s after enabling. The plan's 5 s window is met by probing the UDP port first and answering port_in_use at once"
  - "desktop.json is saved before the server and Manager start (the plan saves after the server): this closes the window where a concurrent terminal unshare saw no desktop and stopped the server; a failed start rolls the file back to off"
  - "An unusable desktop.json also refuses PUT on (start_failed with the reason) instead of overwriting it; the user decides what happens to that file"
  - "The poller starts from both API.Start and serveOn (the takeover daemon's serve path), so a handed-off daemon polls too; it skips ticks while frozen or committed"
  - "Key regeneration closes desktop viewers with 4401 after a.mu is released, not inside the a.mu section: CloseAll waits for each viewer's close handshake and must not run under the share lock"

requirements-completed: [DSHR-01, DSHR-02, DESK-03]

coverage:
  - id: F1
    description: "PUT /desktop on with Tailscale connected writes desktop.json 0600 (schema 1, 128-bit hex grantID, iat), starts the share server and the Manager on the tailnet address, returns the /desktop/?cap= link, the page opens and a pion viewer receives an IDR from a synthetic source; GET /desktop reports streaming with 1 viewer"
    requirement: "DSHR-01"
    verification:
      - kind: integration
        ref: "internal/daemon/desktop_api_test.go#TestDesktopAPI_OnLinkStreamTracer"
        status: pass
    human_judgment: false
  - id: F2
    description: "The link is byte-for-byte identical after a daemon restart on the same capability.key and desktop.json, and the saved link opens 200"
    requirement: "DSHR-02"
    verification:
      - kind: integration
        ref: "internal/daemon/desktop_api_test.go#TestDesktopAPI_LinkStableAcrossRestart"
        status: pass
    human_judgment: false
  - id: F3
    description: "A daemon that starts with always-available on brings the server and Manager up by itself once Tailscale is connected, starts nothing while it is down, restarts both when the tailnet name changes, and never asks Tailscale while the desktop is off; userspace is recorded and starts nothing"
    requirement: "DSHR-01"
    verification:
      - kind: integration
        ref: "internal/daemon/desktop_api_test.go#TestDesktopAPI_PollerStartsAtDaemonStart"
        status: pass
      - kind: integration
        ref: "internal/daemon/desktop_api_test.go#TestDesktopAPI_PollerIgnoresOffAndUserspace"
        status: pass
    human_judgment: false
  - id: F4
    description: "tailscale_off, https_off and tailscale_userspace refuse a turn-on and leave desktop.json absent, no server running and status off"
    requirement: "DESK-03"
    verification:
      - kind: integration
        ref: "internal/daemon/desktop_api_test.go#TestDesktopAPI_Refusals"
        status: pass
    human_judgment: false
  - id: F5
    description: "Off clears the grant (grantID empty on disk), closes viewers with 4410, stops capture, gives the old link a 410 off page and keeps a terminal share and its server; regeneration closes viewers with 4401, the old link gets 401 token and Copy Link returns a new URL"
    requirement: "DSHR-02"
    verification:
      - kind: integration
        ref: "internal/daemon/desktop_api_test.go#TestDesktopAPI_OffClosesViewers"
        status: pass
      - kind: integration
        ref: "internal/daemon/desktop_api_test.go#TestDesktopAPI_RegenerateKey"
        status: pass
      - kind: integration
        ref: "internal/daemon/desktop_api_test.go#TestDesktopAPI_OffStopsServerWhenNothingElseNeedsIt"
        status: pass
    human_judgment: false
  - id: F6
    description: "The share server runs exactly while a terminal share or the desktop needs it, in all four on/off orders, with the same server object across a desktop off, a viewer still streaming across a terminal off, a session exit with the desktop on, and one server built when both start at once"
    requirement: "DSHR-01"
    verification:
      - kind: integration
        ref: "internal/daemon/share_lifetime_test.go#TestShareLifetime_DesktopOffKeepsTerminal"
        status: pass
      - kind: integration
        ref: "internal/daemon/share_lifetime_test.go#TestShareLifetime_TerminalOffKeepsDesktop"
        status: pass
      - kind: integration
        ref: "internal/daemon/share_lifetime_test.go#TestShareLifetime_AllOrders"
        status: pass
      - kind: integration
        ref: "internal/daemon/share_lifetime_test.go#TestShareLifetime_SessionExitWithDesktopOn"
        status: pass
      - kind: integration
        ref: "internal/daemon/share_lifetime_test.go#TestShareLifetime_ConcurrentStartsShareOneServer"
        status: pass
    human_judgment: false
  - id: F7
    description: "Through the real Manager, the production adapter and 19-04's route, eight viewers stream and a ninth /desktop/ws gets HTTP 429 with no upgrade; GET /desktop carries the daemon's launchd flag; a malformed, wrong-mode or unknown-schema desktop.json keeps the desktop off with a reason, refuses turn-on and is never rewritten"
    requirement: "DESK-03"
    verification:
      - kind: integration
        ref: "internal/daemon/desktop_api_test.go#TestDesktopAPI_ViewerCapRefusedBeforeUpgrade"
        status: pass
      - kind: integration
        ref: "internal/daemon/desktop_api_test.go#TestDesktopAPI_StatusReportsLaunchd"
        status: pass
      - kind: integration
        ref: "internal/daemon/desktop_api_test.go#TestDesktopAPI_BadConfigIsExplicit"
        status: pass
    human_judgment: false
  - id: F8
    description: "desktop.json: a symlink, directory, mode 0644 or 0640, size over 4 KiB, schemaVersion 2 or missing, an unknown field, invalid JSON, an empty file and alwaysAvailable without a grantID each return an error; a missing file is the zero Config; SaveConfig then LoadConfig round-trips at mode 0600 with no temp file left"
    requirement: "DESK-03"
    verification:
      - kind: unit
        ref: "internal/desktop/config_test.go#TestDesktopConfig_Unsafe"
        status: pass
      - kind: unit
        ref: "internal/desktop/config_test.go#TestDesktopConfig_RoundTrip"
        status: pass
    human_judgment: false

duration: 70min
completed: 2026-11-26
---

# Phase 19 Plan 08: Daemon Always-Available Desktop Summary

**The daemon now owns the always-available desktop: its setting lives in its own `desktop.json`, a socket API turns it on and off and returns a link that is identical after every restart, a health poller brings it up at daemon start with no GUI, and the share server's lifetime follows `serverNeededLocked` so the desktop and terminal shares never take it from each other.**

## Accomplishments

- **desktop.json (T-19-26):** `LoadConfig` accepts only a regular, 0600, under-4-KiB file with schema 1 and no unknown fields (and a grant when on); a swap between the Lstat and the open fails the `SameFile` check. `SaveConfig` writes a 0600 temp file in the same directory, syncs and renames. Nothing about the setting is in `settings.json` (the engine.go guard grep is 0).
- **Socket API:** `GET /desktop` (status, including the daemon's own `launchd` flag and the last Tailscale verdict), `PUT /desktop` (`DisallowUnknownFields`, ShareError codes through the existing `shareStatus`), `GET /desktop/link` (409 while off). Client methods mirror `SetSessionShared`'s `*ShareError` decoding.
- **Stable link (DSHR-02):** the link is `SignDesktop` over the persisted grantID and iat with the current signing key; the restart test asserts byte-for-byte equality.
- **Lifetime (T-19-28):** `unshareLocked` now calls `stopServerIfIdleLocked` (`!serverNeededLocked`), and so do desktop off and the turn-on rollback. `wireShareServer` installs the desktop backend, so a handoff-built server gets it too.
- **Poller (DSHR-01):** an immediate tick then every 10 s while the desktop is on; Tailscale off, HTTPS off or userspace are recorded and retried; the server and Manager are rebuilt when the tailnet name or address differs from what they were built for. A server whose build address is unknown (installed by a handoff) is never treated as moved.
- **Revocation (T-19-27):** off clears the grant, closes viewers 4410 and stops capture; regeneration closes viewers 4401.
- **Userspace (T-19-29):** refused with `tailscale_userspace` when the tailnet address is on no local interface, before anything binds or is written.

## Task Commits

1. **Task 1 (tracer): desktop.json, socket API, share server wiring, Manager, link, plus the off, regeneration and poller code** - `ccbb6590` (feat)
2. **Task 2 tests: refusals, off, regeneration, stable link, poller, cap, launchd, bad config** - `6a0841c2` (test)
3. **Task 3 tests: share server lifetime matrix** - `defc86d6` (test)

## TDD Gate Compliance

Tasks 2 and 3 are `tdd="true"` but there is no RED commit before their implementation. The implementation each task names (turn-off, regeneration closing, the poller, `serverNeededLocked` at every stop site) is small and was needed to make Task 1's tracer coherent, so it landed in the Task 1 commit. Tasks 2 and 3 therefore consist of tests that passed on first run, the "unexpected GREEN" case. I did not trust that; each test was shown not to be vacuous by mutating the implementation and re-running (every mutation restored with `git checkout -- <file>`):

- Removing the `CloseTokenRejected` close in `handleRegenerateSigningKey`: `TestDesktopAPI_RegenerateKey` fails ("viewer socket still open after 10s").
- Removing the `CloseDesktopOff` close in `disableDesktop`: `TestDesktopAPI_OffClosesViewers` fails (viewer closed with 1012, want 4410).
- Skipping the userspace check: `TestDesktopAPI_Refusals/userspace` and `TestDesktopAPI_PollerIgnoresOffAndUserspace` fail.
- Making `desktopMovedLocked` always false: `TestDesktopAPI_PollerStartsAtDaemonStart` fails ("timed out waiting for the poller to restart both").
- Swallowing the config load error in `NewAPI`: all three `TestDesktopAPI_BadConfigIsExplicit` cases fail.
- `serverNeededLocked` ignoring the desktop: `TestShareLifetime_TerminalOffKeepsDesktop`, `_AllOrders` (two orders) and `_SessionExitWithDesktopOn` fail. Ignoring terminal shares: `_DesktopOffKeepsTerminal`, `_AllOrders` (two orders) and the existing `TestShare_ExitCleanupDropsRecord` fail.

The git-log gate order (`test` before `feat`) is therefore not met for Tasks 2 and 3; flagged here rather than hidden.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] A 5 s start context would end capture 5 s after enabling**
- **Found during:** Task 1 design (reading `Manager.Start`).
- **Issue:** `Manager.Start(ctx)` stores `ctx` as `m.ctx`, and every capture run derives from it. The plan says to start the Manager with "a 5 s start context".
- **Fix:** `startDesktopManager` passes the API's own context (cancelled in `Stop`), and a throwaway `net.ListenUDP` probe answers `port_in_use` at once when the port is busy. `Manager.Start` itself is unchanged (19-09 is editing manager.go).
- **Files modified:** internal/daemon/desktop_api.go. **Commit:** `ccbb6590`

**2. [Rule 1 - Race] Save desktop.json before starting anything, not after the server**
- **Found during:** Task 1 design. With the plan's order (server first, setting second) a terminal unshare between the two sees no desktop in `serverNeededLocked` and stops the server just started.
- **Fix:** the new setting is saved and put in memory first; a failed start calls `disableDesktop` to roll it back (file and memory, server stopped if idle). Refusals happen before the save, so they still leave no file.
- **Commit:** `ccbb6590`

**3. [Rule 2 - Missing safety] Turn-on refused while desktop.json is unusable**
- The plan says an unusable file is never silently replaced and always-available reads off with a Reason. A turn-on would have overwritten it, so it answers `start_failed` with the reason instead. Tested in `TestDesktopAPI_BadConfigIsExplicit`.

**4. [Plan interpretation] Regeneration closes viewers after releasing a.mu**
- The plan says "under the same a.mu section". `Manager.CloseAll` waits on each viewer's close handshake (up to 5 s each), which must not run under the share lock. The key is swapped and shares re-minted under a.mu as before, then the viewers are closed. Viewers on the old link can stay for milliseconds longer; new requests already fail the token check.

**5. [Rule 3 - Blocking] types.go and a poller start in serveOn**
- `types.go` (the `ShareErrUserspace` constant, as the plan's Artifacts require) is not in `files_modified`. The poller is started from `Start` and from `serveOn` (the takeover daemon's serve path, which does not call `Start`); both are in api.go. No change to cmd_daemon.go or process.go was needed.

**Total deviations:** 5, none changing a contract 19-09 to 19-14 read.

## Notes for later plans

- **19-09:** `GET /desktop` builds its body from `Manager.Status()`; when 19-09 fills `Locked`, `ApproachingLapse`, `LapseSeq` and `Reason` they pass straight through. No change is needed in this file.
- **19-10:** `API.Stop` closes the Manager and cancels the poller unconditionally. The handoff old side sets `a.webServer = nil` itself, so `Manager` and `desktopMgr` are not handed over here; the poller skips ticks while `committed` is set and the takeover daemon's poller retries `port_in_use` every 10 s until the old daemon's sockets are gone.
- **19-11:** call `SetDesktopNotifier` from cmd_daemon.go; the Manager reads it at call time.
- **Poller and Tailscale:** `CheckHealth` returns "not connected" under a test binary, so no test touches the host's tailscaled. The poller never asks Tailscale while the desktop is off (asserted).

## Known Stubs

None. `Manager.Status()` still reports only idle or streaming until 19-09; the API passes the other fields through.

## Threat Flags

None beyond the plan's register. T-19-26 (desktop.json), T-19-27 (revocation), T-19-28 (lifetime), T-19-29 (userspace) are implemented and tested. T-19-30: the URL and token are not logged (the only log lines are the poller's error code and a rollback error); status carries no token. T-19-SC: no package added.

## Verification

- `go vet`, `gofmt -l` clean; `golangci-lint run --new-from-rev=89d29266 ./internal/daemon/... ./internal/desktop/...` 0 issues.
- `go test -race -short -count=1 ./internal/daemon/ ./internal/desktop/` pass (daemon 99 s). `TestDesktopAPI_*` pass at `-race -count=3`, `TestShareLifetime_*` at `-race -count=5`.
- `go build ./...` ok; `bash tests/check-brand.sh` clean.
- Acceptance greps: `serverNeededLocked() bool` 1; `serverNeededLocked()` in share.go 1; `len(a.shares) == 0` in share.go 0; `SetDesktopBackend(` 1; `screencap.NewDisplaySource` in desktop_api.go 1; `alwaysAvailable` in engine.go 0; `CloseTokenRejected` in api.go 1; `runDesktopPoller` in desktop_api.go 4.
- Host safety: only synthetic sources, loopback sockets and fakes ran. `defaultNewDesktopSource` (the real capture) refuses under a test binary and was never exercised; no VM, launchd, TCC or tailnet work.
- TESTING.md was not touched (19-16 owns the consolidated update: it should list `desktop_api_test.go`, `share_lifetime_test.go`, `internal/desktop/config_test.go` and traceability rows for DSHR-01, DSHR-02, DESK-03). No STATE.md or ROADMAP.md changes.

## Self-Check: PASSED

- Files exist: config.go, config_test.go, desktop_api.go, desktop_api_test.go, share_lifetime_test.go.
- Commits `ccbb6590`, `6a0841c2`, `defc86d6` are ancestors of HEAD.
