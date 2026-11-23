---
phase: 15-v1-2-hardening
reviewed: 2026-11-23T18:00:00Z
depth: standard
files_reviewed: 47
files_reviewed_list:
  - TESTING.md
  - app_update.go
  - app_update_test.go
  - cmd_attach.go
  - cmd_attach_resize.go
  - cmd_attach_test.go
  - frontend/src/App.tsx
  - frontend/src/components/TabBar.tsx
  - frontend/src/components/TerminalPanel.tsx
  - frontend/src/components/WebShareSessionView.tsx
  - frontend/src/components/__tests__/App.open-remote.test.tsx
  - frontend/src/components/__tests__/App.update.test.tsx
  - frontend/src/components/__tests__/TerminalPanel.guestClose.test.tsx
  - frontend/src/lib/__tests__/closeReasons.test.ts
  - frontend/src/lib/__tests__/pathLinks.linkifier.test.ts
  - frontend/src/lib/closeReasons.ts
  - frontend/src/lib/pathLinks.ts
  - internal/attach/attach.go
  - internal/attach/attach_test.go
  - internal/attach/attach_unix.go
  - internal/daemon/engine.go
  - internal/daemon/handoff.go
  - internal/daemon/handoff_it_test.go
  - internal/daemon/handoff_test.go
  - internal/daemon/ipc.go
  - internal/daemon/socket.go
  - internal/daemon/socket_test.go
  - internal/fdpass/fdpass.go
  - internal/fdpass/fdpass_test.go
  - internal/pty/adopt_test.go
  - internal/pty/cleanup.go
  - internal/pty/native_test.go
  - internal/pty/reap_other.go
  - internal/pty/session.go
  - internal/relay/cwd_test.go
  - internal/relay/history.go
  - internal/relay/hub.go
  - internal/relay/hub_grid_owner_test.go
  - internal/relay/manager.go
  - internal/relay/pathclick_test.go
  - internal/statusbar/bar.go
  - internal/statusbar/bar_test.go
  - internal/tailnet/sessions.go
  - internal/tailnet/tailnet_test.go
  - internal/webserver/capability_helpers_test.go
  - internal/webserver/input_queue_test.go
  - tray.go
  - tray_objc_darwin.m
findings:
  critical: 0
  warning: 0
  info: 7
  total: 7
  necessity: 0
status: issues_found
---

# Phase 15: Code Review Report

**Reviewed:** 2026-11-23T18:00:00Z
**Depth:** standard
**Files Reviewed:** 47
**Status:** issues_found

## Summary

Iteration 4 (incremental). Scope since 6d95ea64: 9a277a74 (comment-only fix in `internal/daemon/ipc.go`), 1c8d3a09 (`TestHandoffIT` compares `/health` API with `APIVersion`), and 7ca43451 (`peerConnectVerdict` maps `not_sharing` to `ended` after a session-ended close). The other files keep their earlier assessment. The necessity reference was not re-read; no necessity findings.

**Verification of 7ca43451**
- Could `ended` now wrongly show for a live session that was unshared after an earlier ended close? No. `sawEnded` is `!!activeWebTab?.sessionEnded` (`App.tsx:1363`). The only writer is `markPeerSessionEnded` (`App.tsx:1053-1056`), keyed by `peerSnapshotHost` and session id, so it is per tab and per session. It fires only on the guest's session-ended close, which the relay sends from the write pump on `hub.Done` (`internal/relay/pong.go:77`); share-off sends `share stopped` instead (`daemon/share.go:353,370`, `webserver/server.go:987`). A session whose hub is done cannot come back under the same id, so for that tab "not shared now" and "gone" are the same fact, and a later `not_sharing` (409, `peer_connect.go:169-171`, either an empty share list or ECONNREFUSED) adds no information. The old behaviour was the bug M-154 reproduced.
- A host that is down or asleep returns `unreachable` (502) or times out, not `not_sharing`, and `unreachable` still retries after the ended close (tested). ECONNREFUSED on a live host that merely stopped its share server maps to `not_sharing`, which is the M-154 case.
- `forbidden` stays `share-stopped`. That is a deliberate access answer (different tailnet account or revoked), and a tab that ended and is now forbidden showing "stopped sharing" is acceptable.
- Tests: `closeReasons.test.ts` covers `no_such_session` and `not_sharing` with `sawEnded` true, `forbidden` and `unreachable` unchanged with it true, and the no-flag mapping for `not_sharing`, `no_such_session`, `forbidden`. `App.open-remote.test.tsx` drives the real `reconnectPeerTab` through the stub for `no_such_session`, `not_sharing` and `forbidden` after `fire-session-ended`, and keeps the no-flag `it.each` (still `share-stopped`). The two layers agree. Both files pass (72 tests, vitest 4.1.0). The comment block on `peerConnectVerdict` was updated with the change and is accurate. The CLI counterpart `peerVerdict` (`cmd_attach.go:236`) is unaffected: a session-ended close is terminal there and does not redial.

**Verification of 1c8d3a09 and 9a277a74**
- `APIVersion` is `5` (`types.go:64`); the stale literal `2` is gone and the failure message now prints the expected value. The test is env-gated and skipped here; `go vet ./internal/daemon` is clean. See IN-07 for what the change gives up.
- The `ipc.go` comment now states that the control socket has no peer-UID check and that a custom path in a looser directory gets no 0700 fence. That resolves the accuracy half of the previous WR-01.

**Resolved: WR-01 (iteration 3)**, the `listenDaemonSocket` comment citing a non-existent peer-UID check. Fixed in 9a277a74. The optional parent-directory enforcement in `Start` was not adopted; the comment now documents the limit honestly and the production path is covered by `DefaultSocketPath`, so no open finding remains.

No Critical or Warning findings. IN-01 to IN-06 are carried forward unchanged; IN-07 is new.

## Info

### IN-01: A reaped session's leftover process group is no longer signalled on dismiss

**File:** `internal/pty/cleanup.go:40-46` (guard), `internal/pty/cleanup.go:20-28` (rationale)
**Issue:** Carried forward, unchanged. The new `reaped` branch skips SIGHUP/SIGKILL for any created session whose shell was already `Wait`ed, on the grounds that the pgid may have been reused. The kernel does not hand out a pid that is still a live process-group id, so while background jobs remain in the group, killing it was safe and was the only way dismiss cleaned them up. They are now orphaned on dismiss. Practical impact is small and the guard cannot tell an empty group from a live one without extra bookkeeping, so this is a trade-off, not a defect.
**Fix:** Add a sentence to the `killSession` doc comment recording that leftover members of a reaped shell's group are intentionally left running.

### IN-02: Duplicate cleanup branches in `killSession`

**File:** `internal/pty/cleanup.go:30-46`
**Issue:** Carried forward, unchanged. The `s.cmd == nil || s.cmd.Process == nil` branch and the `s.reaped.Load()` branch have identical bodies.
**Fix:**

```go
if s.cmd == nil || s.cmd.Process == nil || s.reaped.Load() {
	s.closePTY()
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}
```

### IN-03: ED 3 `History.Skip` counts main-screen scrollback even while the alternate screen is active

**File:** `internal/relay/hub.go:1301-1305`
**Issue:** Carried forward, unchanged. The hook adds `emu.ScrollbackLen()` (always the main screen's) to `pushed`, but vt's ED 3 handler clears the current screen's scrollback. With the alternate screen active the main-screen lines are not dropped, then drain into history later and are counted twice, so absolute rows overshoot. The window is narrow (an alt-screen switch plus ED 3 in the same write chunk, with undrained main scrollback). The older `h.history.Clear()` on the preceding line has the same alt-screen looseness.
**Fix:** `if !emu.IsAltScreen() { h.history.Skip(emu.ScrollbackLen()) }`, and add a case to `cwd_test.go`.

### IN-04: A tailnet peer that does not run Portty at all now reads as "reachable, nothing shared"

**File:** `internal/tailnet/sessions.go:83-86, 236`, `frontend/src/lib/sidebarPeers.ts:54-58`, `cmd_cli.go:242-246`
**Issue:** Carried forward, unchanged. `discoverPeers` keeps every online peer, and a refused TCP connection is now `MetaRefused`, rendered "Nothing shared". A phone, NAS or Linux box that RSTs the share port looks the same as a Portty Mac with its share server stopped. Acceptable on a two-Mac tailnet and matches the stated intent of 15-07; recorded as a known limit.
**Fix:** No code change needed. If a larger tailnet gets noisy, filter by `peer.OS == "macOS"` or by a prior successful probe that carried `X-Portty: 1`.

### IN-05: Notification delegate is registered after launch, so the cold-launch click path is unproven

**File:** `tray_objc_darwin.m:173-180`
**Issue:** Carried forward, unchanged. Registration runs inside a `dispatch_async` block queued from `initStatusItem`, so it can run after `applicationDidFinishLaunching` returns. Whether a late assignment still receives a pending launch-from-notification response is system behaviour not confirmed from the code. M-152's relaunch step is the right check and is still "not yet run".
**Fix:** Make M-152's relaunch step a hard PASS condition. If it fails, call `registerNotificationDelegate()` synchronously from the earliest cgo init point.

### IN-06: `joinPasteViewer` copies `dialResume` instead of extending it

**File:** `internal/webserver/input_queue_test.go:46-50`, `internal/webserver/resume_test.go:117-124`
**Issue:** Carried forward, unchanged. To pass an `OnPingReceived` hook, `joinPasteViewer` re-implements `dialResume` (Origin header, URL, `SetReadLimit`), so a later change to `dialResume` would silently diverge for the paste tests only.
**Fix:** Give `dialResume` an optional ping hook and have `joinPasteViewer` use it.

### IN-07: `TestHandoffIT` now compares a prebuilt binary's API against the test binary's own constant

**File:** `internal/daemon/handoff_it_test.go:64-65`
**Issue:** The test drives prebuilt binaries (`PORTTY_HANDOFF_IT_B`) but now asserts `h.API == APIVersion`, where `APIVersion` is compiled into the test binary, not read from B. If B is built from a different commit than the tests, the check fails or passes by coincidence, and a regression that bumps or lowers `APIVersion` in both is no longer pinned here. The hard-coded literal rotted (the M-155 finding), so a constant is the right direction, and `client_test.go:50` still pins `APIVersion != 5` in process, so the number is guarded elsewhere. This only matters if the rig builds B from a different tree.
**Fix:** State in the `TestHandoffIT` doc comment that A, B, BFAULT and the test must be built from the same commit, or have the rig script pass the expected API through an env var (`PORTTY_HANDOFF_IT_API`) read by the test.

---

_Reviewed: 2026-11-23T18:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
