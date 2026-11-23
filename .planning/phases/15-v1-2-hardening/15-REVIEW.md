---
phase: 15-v1-2-hardening
reviewed: 2026-11-23T17:05:00Z
depth: standard
files_reviewed: 46
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
  warning: 1
  info: 6
  total: 7
  necessity: 0
status: issues_found
---

# Phase 15: Code Review Report

**Reviewed:** 2026-11-23T17:05:00Z
**Depth:** standard
**Files Reviewed:** 46
**Status:** issues_found

## Summary

Iteration 3 (final) of the `--auto` loop. `git diff --stat cc7e67fb~1 HEAD` outside `.planning` shows only `internal/daemon/ipc.go` and `internal/daemon/socket_test.go` changed, so the other 44 files keep their earlier assessment. The necessity reference was not re-read because no new code outside the fix was assessed; no necessity findings.

**Verification of cc7e67fb**
- The process-wide umask is gone. `grep` finds no `syscall.Umask` or `umaskMu` anywhere in the repo. The previous WR-01 (umask leaking into forked shells and concurrent `Mkdir` calls) is resolved.
- `listenDaemonSocket` is `net.Listen`, then `os.Chmod(0600)`. A failed chmod closes the listener and fails the start, so the socket is never served wider than 0600. This matches the old behaviour, and the only caller is `API.Start` (`api.go:319`).
- `DefaultSocketPath` creates the config dir 0700 and tightens an existing one (`socket.go:16-28`). On the production path that closes the bind-to-chmod window, because another user cannot traverse to the inode.
- `go vet ./internal/daemon` is clean. `TestListenDaemonSocket_Mode0600`, `TestDefaultSocketPath_TightensExistingDir`, `TestSocketPath*` and `TestTestMain_IsolatesConfigDir` pass with `GOTOOLCHAIN=go1.26.11`. Removing `TestListenDaemonSocket_RestoresUmask` was correct, since it tested code that no longer exists.
- One claim in the new comment does not hold (WR-01 below). It does not change runtime behaviour, but it misstates the security model.

No Critical findings. IN-01 to IN-06 are carried forward unchanged from the previous review.

## Warnings

### WR-01: The `listenDaemonSocket` comment cites a peer-UID check that does not exist on the control socket, and the 0700 fence is assumed rather than enforced

**File:** `internal/daemon/ipc.go:16-21` (comment), `internal/daemon/api.go:313-324` (`Start`), `internal/daemon/socket.go:21-26`
**Issue:** The comment, and the commit message of cc7e67fb, say the bind-to-chmod window is fenced by the 0700 parent directory "and the daemon also checks the peer UID". The only `peerUID` use is in `serveHandoff` (`handoff.go:465`), which guards the takeover handshake. The control socket is served by plain `http.Serve(ln, a.handler)` (`api.go:310, 324`) with no `ConnState`, `ConnContext` or peer-credential check. `grep` for `PeerUID`/`Getuid` in non-test code finds only the handoff site. So for control-socket requests the protection is the 0600 mode and the 0700 directory, and nothing else. A future change that loosens the directory would be justified by a check that is not there.

The fence also depends on callers. `Start(socketPath)` validates only the path length and does not check that the parent directory is private. `DefaultSocketPath` ignores a `MkdirAll` failure, and an `os.Chmod` failure is only logged (`socket.go:23-26`). If the chmod fails, for example on a directory owned by another uid, the daemon still binds in a directory it did not secure. The earlier review suggested stat-ing the parent and rejecting group/other bits. That was not adopted, so the guarantee has no enforcement point. For the production path (the user's own `~/Library/Application Support/portty`) the exposure is a microsecond window inside a 0700 directory the user owns. This is a robustness and accuracy defect, not an exploitable hole.
**Fix:** Correct the comment to say what is true. For example: "The bind-to-chmod window is fenced only by the 0700 parent directory; the control socket does not check the peer UID (only the handoff handshake does)." Then add a cheap enforcement in `Start`, or at the top of `listenDaemonSocket`, that fails closed:

```go
fi, err := os.Stat(filepath.Dir(path))
if err != nil { return nil, err }
if fi.Mode().Perm()&0o077 != 0 {
	return nil, fmt.Errorf("daemon socket: parent dir %s is mode %o; refusing to bind", filepath.Dir(path), fi.Mode().Perm())
}
```

Tests that bind under `shortSocketPath` temp dirs need to create those dirs 0700 (`MkdirTemp` already does).

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

---

_Reviewed: 2026-11-23T17:05:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
