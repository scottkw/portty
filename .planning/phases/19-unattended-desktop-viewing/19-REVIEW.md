---
phase: 19-unattended-desktop-viewing
reviewed: 2026-11-26T00:00:00Z
depth: standard
files_reviewed: 99
files_reviewed_list:
  - app_desktop_test.go
  - app_desktop.go
  - app_lapse_test.go
  - app_launchd_test.go
  - app.go
  - cmd_cli.go
  - cmd_daemon_test.go
  - cmd_daemon.go
  - frontend/src/components/__tests__/DesktopSetupSheet.test.tsx
  - frontend/src/components/__tests__/DesktopView.test.tsx
  - frontend/src/components/__tests__/SettingsSheet.desktop.test.tsx
  - frontend/src/components/__tests__/SettingsSheet.tailscale-status.test.tsx
  - frontend/src/components/__tests__/SettingsSheet.test.tsx
  - frontend/src/components/DesktopSetupSheet.tsx
  - frontend/src/components/DesktopView.tsx
  - frontend/src/components/RegenerateKeyModal.tsx
  - frontend/src/components/SettingsSheet.tsx
  - frontend/src/lib/__tests__/desktopClient.test.ts
  - frontend/src/lib/__tests__/webMode.test.ts
  - frontend/src/lib/desktopClient.ts
  - frontend/src/lib/webMode.ts
  - frontend/src/main.tsx
  - frontend/src/style.css
  - frontend/src/wailsjs/go/main/App.d.ts
  - frontend/src/wailsjs/go/main/App.js
  - go.mod
  - go.sum
  - internal/capability/desktoptoken_test.go
  - internal/capability/desktoptoken.go
  - internal/daemon/api.go
  - internal/daemon/client.go
  - internal/daemon/desktop_api_test.go
  - internal/daemon/desktop_api.go
  - internal/daemon/desktop_lapse_test.go
  - internal/daemon/desktop_permissions_test.go
  - internal/daemon/desktop_vmdesktop.go
  - internal/daemon/handoff_desktop_test.go
  - internal/daemon/handoff.go
  - internal/daemon/launchd_test.go
  - internal/daemon/launchd.go
  - internal/daemon/process_test.go
  - internal/daemon/process.go
  - internal/daemon/rehome_test.go
  - internal/daemon/share_lifetime_test.go
  - internal/daemon/share.go
  - internal/daemon/types.go
  - internal/desktop/bind_test.go
  - internal/desktop/bind.go
  - internal/desktop/config_test.go
  - internal/desktop/config.go
  - internal/desktop/export_test.go
  - internal/desktop/fakes_test.go
  - internal/desktop/fanout_test.go
  - internal/desktop/lapse_hook.go
  - internal/desktop/lapse_vmdesktop_test.go
  - internal/desktop/lapse_vmdesktop.go
  - internal/desktop/loopback_test.go
  - internal/desktop/manager.go
  - internal/desktop/peer_test.go
  - internal/desktop/peer.go
  - internal/desktop/permissions.go
  - internal/desktop/power_test.go
  - internal/desktop/power.go
  - internal/desktop/runtime_state.go
  - internal/desktop/state_test.go
  - internal/desktop/state.go
  - internal/desktop/viewer.go
  - internal/desktop/watchdog_test.go
  - internal/desktop/watchdog.go
  - internal/screencap/frames_test.go
  - internal/screencap/frames.go
  - internal/screencap/media_darwin.go
  - internal/screencap/media_darwin.m
  - internal/screencap/preflight_darwin.go
  - internal/screencap/preflight_darwin.m
  - internal/screencap/screencap_darwin_test.go
  - internal/screencap/screencap.go
  - internal/screencap/synthetic_test.go
  - internal/screencap/synthetic.go
  - internal/screencap/sys_darwin.go
  - internal/screencap/sys_darwin.m
  - internal/webserver/crosstoken_test.go
  - internal/webserver/desktop_csp_e2e_test.go
  - internal/webserver/desktop_routes_test.go
  - internal/webserver/desktop_routes.go
  - internal/webserver/server.go
  - internal/webserver/whois_gate_test.go
  - notification_darwin_test.go
  - notification_darwin.go
  - TESTING.md
  - tests/check-brand.sh
  - tray_common_test.go
  - tray_common.go
  - tray_objc_darwin.m
  - tray.go
  - web/desktop-link-invalid.html
  - web/desktop-off.html
  - web/embed.go
  - web/service-retry.html
findings:
  critical: 1
  warning: 7
  info: 5
  total: 13
  necessity: 0
status: issues_found
---

# Phase 19: Code Review Report

**Reviewed:** 2026-11-26
**Depth:** standard
**Files Reviewed:** 99
**Status:** issues_found

## Summary

Production code was read in full for `internal/desktop`, `internal/screencap` (Go and Objective-C), `internal/webserver/desktop_routes.go`, `internal/capability/desktoptoken.go`, `internal/daemon/{desktop_api,launchd}.go`, and the Phase 19 diffs of `api.go`, `handoff.go`, `process.go`, `share.go`, `client.go`, `app.go`, `cmd_daemon.go`, the tray files and the frontend desktop client, view and Settings group. Test files were only skimmed. `go vet` on the changed Go packages is clean (the root package only trips the Go 1.27 toolchain guard, which is intended). `tests/check-brand.sh` and `tests/check-traceability-paths.sh` pass. No test that touches capture was run, and nothing contacted a tailnet address.

The overall design is sound where it matters most for security:
- Desktop tokens are HKDF-separated from the session key, so cross-token acceptance is blocked by the HMAC and not by the Kind field.
- The route stack runs Origin, then same-user WhoIs, then the token, all before the upgrade.
- The media socket binds only a CGNAT address.
- ICE-lite, UDP4 only, no mDNS, and a candidate filter are in place.
- The CSP is untouched.
- The lock-order discipline (mu, then vmu, with smu, kmu and lmu as leaf locks) holds on every path I traced.

The defects are concentrated in revocation, the lifecycle lock, and media timing:
- One revocation gap defeats the signing-key panic button for the desktop link.
- One lifecycle-lock hold can both stall the whole desktop and falsely trip the crash-loop breaker on an unattended Mac.
- One RTP timestamp bug probably produces visible lag after idle gaps.

## Critical Issues

### CR-01: Regenerating the signing key does not revoke a viewer that is mid-handshake (revocation bypass)

**File:** `internal/daemon/api.go:820-831`, `internal/webserver/desktop_routes.go:169-186`, `internal/desktop/manager.go:460-562`
**Issue:** The token is verified once, in `handleDesktopWS`, before the WebSocket upgrade. `ServeDesktopViewer` then spends up to `helloTimeout + offerTimeout` (about 40 s) in the pre-join handshake. A viewer in that state is not in `m.viewers`, so `handleRegenerateSigningKey`'s `mgr.CloseAll(CloseTokenRejected, ...)` does not see it. After CloseAll returns, the viewer sends its offer, `m.join` succeeds (the Manager is not closed), and it streams the screen indefinitely under a link the user just revoked.

An attacker holding a leaked link, from a device on the same Tailscale account, can keep several sockets parked just before the offer. After the user presses "Regenerate…" they release the offer and keep access. `disableDesktop` is not affected, because it calls `Close()`, which sets `closed`, so a later `join` fails. Only the key-regeneration path has the hole.

**Fix:** Give the Manager a revocation epoch and have the join check it.
```go
// Manager
revokeSeq atomic.Uint64

// Revoke closes every joined viewer and invalidates every handshake in flight.
func (m *Manager) Revoke(code websocket.StatusCode, reason string) {
	m.revokeSeq.Add(1)
	m.CloseAll(code, reason)
}

// ServeDesktopViewer: capture at entry (right after the route's token check)
seq := m.revokeSeq.Load()
// join(v, seq): under m.mu, before registering
if m.revokeSeq.Load() != seq { return errRevoked }   // close with CloseTokenRejected
```
Then call `mgr.Revoke(desktop.CloseTokenRejected, "token rejected")` from `handleRegenerateSigningKey`. As a belt-and-braces alternative, have `desktopAdapter.ServeDesktopViewer` re-verify the grant and key generation immediately before `join`.

## Warnings

### WR-01: `Manager.mu` is held across blocking native capture starts (up to about 35 s). This stalls the state loop and Close, and can falsely trip the crash-loop breaker

**File:** `internal/desktop/manager.go:592-647, 880-894`, `internal/desktop/watchdog.go:68-76, 309-329`, `internal/screencap/media_darwin.m:392, 505`
**Issue:** `join` and `restartSource` call `startSourceLocked` with `m.mu` held. That reaches `pc_stream_start`, which waits up to 15 s for `SCShareableContent` and up to 20 s for `startCapture`. A pending macOS approval dialog is exactly the case where these calls hang, and during awaiting-approval the watchdog retries a restart every 60 s.

Consequences:
1. `watchdogStep`, `ensureSource` and `forceKeyframeNow` all take `m.mu`, so the state loop tick, keyframe requests, `leave` and every other `join` stall for the duration.
2. `Close()` takes `m.mu` before it calls `noteCaptureStopped()` (the file comment says it clears the flag "before any viewer is closed", but the lock comes first). If a daemon exit or handoff drain (`restartDrain`) or `/shutdown` (500 ms grace) fires while a start is blocked, the process exits with `captureActive=true` still on disk. The next process counts an abnormal exit. Three such exits within an hour open the breaker, and `capture_paused` needs a Test Capture at the Mac. That defeats the "unattended Mac" goal.
3. The first viewer's join does not return its answer until the start finishes. The client has no post-hello watchdog, and its 30 s unreachable timer then shows "Can't reach this Mac" instead of the Waiting for approval card.

**Fix:**
- Clear `captureActive` before taking `m.mu` in `Close()` (the file write needs no lifecycle lock). Use an `atomic.Bool` for the closed flag so the order does not depend on `mu`.
- Better, start the source outside `m.mu`. Reserve under the lock (set a `starting` marker and a fresh `fanDone`), run `src.Start` unlocked, then re-lock to publish `m.src`.
- Until then, make the watchdog and `forceKeyframeNow` use `TryLock` or a separate `srcMu` so that a slow start cannot freeze the tick.

### WR-02: RTP timestamps lag one access unit behind capture time, and the tick count can overflow after long idle gaps

**File:** `internal/desktop/manager.go:758-760`, `internal/screencap/media_darwin.go:111-117`
**Issue:** `AccessUnit.Duration` is the gap since the previous unit (`now - prev`). `fanOut` passes it as `samples` to `pkt.Packetize`, but pion's `Packetize` stamps the packets with the current `p.Timestamp` and only then adds `samples` (rtp `packetizer.go`: `Timestamp: p.Timestamp` ... `p.Timestamp += samples`). So unit N is stamped with the time of unit N-1, and the long gap is carried by unit N+1.

Capture is variable-rate (frames only on change). After an idle gap g, the first frame arrives g late but is stamped about 33 ms after its predecessor. The next frame, which arrives about 33 ms later, is stamped g after it. A WebRTC receiver's timestamp extrapolator and jitter buffer will plausibly hold that frame for up to g seconds (and reset timing above about 10 s). The visible symptom is lag after idle, in exactly the still-screen use case this phase targets. No test asserts RTP timestamps (grep for `Timestamp` in `internal/desktop/*_test.go` finds none).

Separately, `uint32(au.Duration.Seconds()*90000)` is a float-to-uint32 conversion. Go leaves it implementation-defined when the value exceeds the uint32 range. That range is 2^32 / 90000 s, about 13.25 h, and "survives overnight" is the core value. A viewer attached to a quiet screen for more than 13.25 h in a single source run, followed by a forced keyframe, hits it. On arm64 the result saturates to 0xFFFFFFFF, which is a -1 tick step. A backward step may make the receiver drop the frame.

**Fix:** Advance the clock before packetizing, and do the arithmetic in integers.
```go
ticks := uint32(uint64(au.Duration.Nanoseconds()) * uint64(h264Capability.ClockRate) / 1e9) // wraps mod 2^32, well-defined
pkt.SkipSamples(ticks)           // move the clock first: this unit is stamped at its own capture time
pkts := pkt.Packetize(au.Data, 0)
```
Add a test that feeds units with a 5 s gap and asserts the RTP timestamp delta equals the arrival gap.

### WR-03: `Manager.RefreshPermissions` is never called, so the Needs permission state stays stale for up to 5 minutes after the user grants Screen Recording

**File:** `internal/desktop/state.go:241-246`, `internal/daemon/desktop_api.go:713-721`
**Issue:** `RefreshPermissions` is documented as "the onboarding endpoint's poll", but `handleDesktopPermissions` only calls `desktopProber.Read`. A repo-wide grep finds no production caller, and only the plan mentions it. `permDenied` is cleared either by a successful start (which needs a viewer) or by `idleProbe`, which runs at most every `idleProbeEvery` (5 min) and only while nobody watches. After the user grants Screen Recording in the setup sheet, the Settings Status row, the tray line and any viewer's state keep saying "Needs permission" and "Choose Set Up Permissions below" for up to 5 minutes, while the sheet itself shows Granted. It is also dead code today.

**Fix:** In `handleDesktopPermissions`, after a successful read call `mgr.RefreshPermissions(r.Context())` when `a.desktopMgr != nil` (outside `a.mu`). The Prober's 1 s cache already bounds the spawns. Alternatively, delete the method and drive `permDenied` from the handler's result.

### WR-04: launchd job liveness depends on one regex. A parse miss lets `bootout` run against a live daemon that holds every terminal session

**File:** `internal/daemon/launchd.go:153-169, 242-262, 383-393`
**Issue:** The file's own safety rule is "nothing here may end a daemon that holds sessions", but the rule rests on `pidRE = (?m)^\s*pid = (\d+)\s*$` over `launchctl print` output. `jobLoaded` returns `pid = 0` both for "idle" and for "running but the regex did not match" (a format change in a macOS update, or a locale or indentation difference). `EnsureAgent` (changed plist, `loaded`, `pid == 0`) and `uninstallAgent` then run `bootout`, which SIGTERMs the running daemon. There is also a TOCTOU window between the `print` and the `bootout`, during which a `kickstart` or KeepAlive start can launch the daemon. Both losses are silent, and the sessions are gone.

**Fix:** Fail closed. Treat `state = running` (also parse it) with no parsed pid as running and return `AgentReloadDeferred` or `ErrAgentRunning`. Before `bootout`, also confirm the daemon is not answering on its socket, or that the lock holder is not this job's pid. Add a test fixture for a running job whose `print` output has no `pid =` line.

### WR-05: The offer-wait loop reads without a deadline once the offer deadline has passed

**File:** `internal/desktop/manager.go:498-523, 425-431`
**Issue:** `read(ctx, c, time.Until(offerDeadline))` only applies a timeout when `d > 0`. If the previous iteration (a `ping`) returns right at the deadline, `time.Until` is zero or negative and the next `c.Read` is unbounded. An authenticated client can time pings so that a handshake goroutine and socket never expire. Handshaking sockets are not counted by `Full()` (only joined viewers are), so this is an unbounded resource hold for a same-account device.

**Fix:**
```go
rem := time.Until(offerDeadline)
if rem <= 0 { return }
in, err := read(ctx, c, rem)
```
Also consider a separate cap on concurrent pre-join handshakes.

### WR-06: `fanOut` does not handle the Errors channel being closed, so it logs spurious `capture error <nil>` lines on every stop

**File:** `internal/desktop/manager.go:761-765`, `internal/screencap/media_darwin.go:388-396`
**Issue:** `DisplaySource.closeChannels` closes both `frames` and `errs`. `fanOut` does `case err := <-errs:` without checking `ok`. When both channels are closed the select picks randomly among ready cases, so on average one iteration logs `slog.Warn("desktop: capture error", "err", nil)`, calls `noteSourceError(nil)` and runs `pushIfChanged`. On a stop that races with a pending state change this also broadcasts. It is log noise at best. A source that closed only `errs` would spin the loop at 100% CPU.

**Fix:**
```go
case err, ok := <-errs:
	if !ok { errs = nil; continue } // a nil channel blocks, so the frames case ends the loop
```

### WR-07: The GUI's lapse-claim guard `lapseTried` is never reset when the daemon's `LapseSeq` restarts, so later lapses are not claimed by the GUI

**File:** `app.go` (`claimAndPostLapse`, new in Phase 19), `internal/desktop/manager.go:286-287`
**Issue:** `lapseSeq` is a per-Manager counter that restarts at 0 whenever the daemon restarts or the Manager is rebuilt (address change, off then on, handoff). The GUI keeps `lapseTried` as the highest sequence it tried. After a rebuild, `seq <= tried` suppresses every claim, so the GUI never sends the notification. The daemon still posts after `guiClaimWait` (15 s), because `guiSeenAt` is fresh and the GUI never claims, so the notification is delayed rather than lost. The one-sender rule silently degrades to "daemon after 15 s" for the rest of that GUI run.

**Fix:** Reset `lapseTried` when the status shows a lower `LapseSeq` than the last one seen, or when the status stops being `awaiting_approval`. Alternatively, key the guard on `(daemon pid, LapseSeq)`.

## Info

### IN-01: Doc comment promises a retry that is not implemented (viewer power holds)

**File:** `internal/desktop/power.go:39-42`, `internal/desktop/manager.go:604-607`
**Issue:** `takeViewerHolds` says "one that failed is tried again on the next join", but `join` only calls it when `first := m.Viewers() == 0`. A failed display or system assertion for the first viewer is never retried while that viewer stays, so the Mac can idle-sleep mid-session.
**Fix:** Call `takeViewerHolds()` on every join. It is idempotent via `hold(cur != nil)`. Or fix the comment.

### IN-02: `LoadConfig` does not reject trailing data, unlike `readRuntimeState` and `decodeProbe`

**File:** `internal/desktop/config.go:75-80`
**Issue:** A `desktop.json` with extra bytes after the first JSON object is accepted. The other two strict readers check `dec.Token() == io.EOF`.
**Fix:** Add the same trailing-data check for consistency.

### IN-03: The debug hook ships in the release viewer

**File:** `frontend/src/lib/desktopClient.ts:452-484`
**Issue:** `#debug` in the URL fragment or `portty:desktop-debug` in localStorage turns on a 1 s stats loop, rewrites the page title and exposes `window.__desktopStats`. It holds counts only (no token or SDP), so there is no leak, but it is test-only behaviour in the production bundle. Also, the `securitypolicyviolation` listener it adds is never removed on `stop`.
**Fix:** Gate it on `import.meta.env.DEV`, or accept it and remove the listener in `shutdown`.

### IN-04: Production code branches on `testing.Testing()`

**File:** `internal/desktop/peer.go:41`, `internal/desktop/permissions.go:69,135`, `internal/daemon/launchd.go:54,127`, `app.go` (`openSystemURL`)
**Issue:** These are deliberate safety rails (no test may capture the screen, run launchctl or write a real LaunchAgents folder), and they work. They do link the `testing` package into the shipped binary and make production behaviour depend on how the binary was built. Prefer injecting the seam (the packages already have `SetLaunchctlForTest` and similar) and dropping the in-line checks.
**Fix:** Optional. Move the guards into the test-only setup (`TestMain`) that already replaces the seams.

### IN-05: A persistently failing encoder produces one error log line per captured frame

**File:** `internal/screencap/media_darwin.m:435`, `internal/desktop/manager.go:761-765`
**Issue:** If the VideoToolbox session turns invalid (for example after a GPU reset), `[enc encode:]` returns `kVTInvalidSessionErr` for every frame. The handler reports each one, and `fanOut` logs each at Warn, up to about 30 lines per second until the stall watchdog restarts the stream (5 s or more). The error channel itself is bounded and non-blocking, so there is no stall, only log growth on a Mac nobody watches.
**Fix:** Rate-limit logging in `fanOut` (once per distinct error per second), or have `DisplaySource` request a restart after N consecutive encode failures.

---

_Reviewed: 2026-11-26_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
