---
phase: 15-v1-2-hardening
reviewed: 2026-11-23T15:30:00Z
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

**Reviewed:** 2026-11-23T15:30:00Z
**Depth:** standard
**Files Reviewed:** 46
**Status:** issues_found

## Summary

Reviewed the Phase 15 diff (`b0cb288f..HEAD`) of the 46 files in scope, reading each changed hunk in the context of its callers. The necessity reference was read; no necessity findings resulted.

Verification run (read-only, no tailnet contact, `GOTOOLCHAIN=go1.26.11`):
- `go test -count=1` for `internal/attach`, `internal/fdpass`, `internal/pty`, `internal/statusbar`, `internal/tailnet`: all pass.
- Targeted `go test` for the relay cwd/pathclick/grid tests, the daemon handoff/socket tests and the root `NotificationClick`/`InitialResize` tests: pass.
- `vitest` for pathLinks.linkifier, closeReasons, App.update and TerminalPanel.guestClose: 58 pass.
- `bash tests/check-traceability-paths.sh`: OK.
- A throwaway probe (scratchpad, unix socket only) of macOS `LOCAL_PEERPID` on the dialing side after a listener fd was inherited by a second process: it reported the accepting process, not the listener's creator. This supports the 15-08 pid check for the handoff chain; M-155 still has to confirm it across real daemons.

No correctness or security defects that would block shipping were found. Every fix does what its commit says, the fd-leak, stale-pgid, pid-check and rows-1 paths are tested, and the failure modes are mostly handled in the safe direction. The one Warning is that the 0600 socket hardening (O-2) has a gap the new test comment claims it does not have. The Info items are behaviour trade-offs and small quality notes.

## Warnings

### WR-01: Daemon socket is group/world-connectable between `bind` and `chmod`, and the "0700 second fence" is not enforced

**File:** `internal/daemon/ipc.go:17-30`, `internal/daemon/socket.go:14-22`
**Issue:** `listenDaemonSocket` binds with `net.Listen` (the socket inode is created with `0777 &^ umask`, typically 0755) and only then calls `os.Chmod(path, 0600)`. For that interval the control socket, which gives write access to every shell, has its looser mode. The comment on `daemonSocketMode` and the test comment ("private to its user from the moment it listens") say the parent directory is the second fence, but `DefaultSocketPath` calls `os.MkdirAll(dir, 0700)` with the error discarded. `MkdirAll` does not tighten a directory that already exists, so a `portty` config dir created by an earlier build or by hand with a looser mode is never repaired. On macOS connecting to a unix socket needs write permission on the socket file, so 0755 only admits the owner and root, but a umask of 000 or 002 (some shell profiles set it) makes the window real, and `tightenSocketMode` on the handoff path has the same post-hoc shape. The test `TestListenDaemonSocket_Mode0600` can only observe the end state, so it cannot catch this.
**Fix:** Close the window instead of narrowing it afterwards. Either bind under a restrictive umask held for the `Listen` call only, serialised with a package mutex because umask is process-wide:

```go
var umaskMu sync.Mutex

func listenDaemonSocket(path string) (net.Listener, error) {
	umaskMu.Lock()
	old := syscall.Umask(0o177) // socket is created 0600
	ln, err := net.Listen("unix", path)
	syscall.Umask(old)
	umaskMu.Unlock()
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, daemonSocketMode); err != nil { // belt and braces
		ln.Close()
		return nil, fmt.Errorf("daemon socket: set mode %o on %s: %w", daemonSocketMode, path, err)
	}
	return ln, nil
}
```

and make `DefaultSocketPath` (or `Start`) `os.Chmod(dir, 0o700)` after `MkdirAll`, logging a failure. Alternatively, soften the comments so they do not claim a guarantee the code does not provide.

## Info

### IN-01: A reaped session's leftover process group is no longer signalled on dismiss

**File:** `internal/pty/cleanup.go:40-46` (guard), `internal/pty/cleanup.go:20-28` (rationale)
**Issue:** The new `reaped` branch skips SIGHUP/SIGKILL for any created session whose shell was already `Wait`ed. The rationale is that the pgid may have been reused. That is only possible once the group is empty: the kernel does not hand out a pid that is still a live process-group id, so while background jobs (`nohup`/`disown`ed servers, a `sleep &` after the shell exited) remain in the group, killing it was both safe and the only way dismiss cleaned them up. After this change they are orphaned on dismiss. Practical impact is small (a shell that exits normally usually HUPs its jobs, and `TestKillClean_NoOrphans` still covers the running case), and the guard cannot tell an empty group from a live one without extra bookkeeping, so this is a trade-off, not a defect. It is worth recording next to the existing "accepted residual windows" note so the behaviour change is a decision on the record.
**Fix:** Add a sentence to the `killSession` doc comment that leftover members of a reaped shell's group are intentionally left running, or, if that is not wanted, signal only when `kill(-pgid, 0)` succeeds and `ps` confirms the leader pid still maps to this child (not worth the complexity unless dismiss orphans show up in practice).

### IN-02: Duplicate cleanup branches in `killSession`

**File:** `internal/pty/cleanup.go:30-46`
**Issue:** The `s.cmd == nil || s.cmd.Process == nil` branch and the new `s.reaped.Load()` branch have identical bodies (`closePTY`, then `cancel`).
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
**Issue:** The hook adds `emu.ScrollbackLen()` to `pushed`, on the premise that vt's own ED 3 handler then drops exactly those lines. vt's handler clears `e.scr.Scrollback()`, the current screen's buffer, whereas `emu.ScrollbackLen()` reads `scrs[0]` (the main screen) always (`vt/emulator.go:451-463`, `vt/handlers.go:559-566`). With the alternate screen active, the alt screen has no scrollback, so the main screen's undrained lines are not dropped; they are drained into history later and counted a second time, so absolute rows overshoot by that many. The window is narrow because `drainScrollbackLocked` empties the main scrollback after every write, so it takes lines scrolled into the main scrollback and an alt-screen switch plus ED 3 in the same write chunk. The pre-existing `h.history.Clear()` on the line above has the same alt-screen looseness.
**Fix:** Guard the count the same way `row0ErasedLocked` does: `if !emu.IsAltScreen() { h.history.Skip(emu.ScrollbackLen()) }`, and add a case to `cwd_test.go`.

### IN-04: A tailnet peer that does not run Portty at all now reads as "reachable, nothing shared"

**File:** `internal/tailnet/sessions.go:83-86, 236`, `frontend/src/lib/sidebarPeers.ts:54-58`, `cmd_cli.go:242-246`
**Issue:** `discoverPeers` keeps every online peer (`tailnet.go:63-67`, no OS or Portty filter), and a refused TCP connection is now `MetaRefused`, rendered "Nothing shared" and included in `portty list --json` `remote` as an empty group. A phone, NAS or Linux box that RSTs the share port is indistinguishable from a Portty Mac with its share server stopped. Before, such peers showed the "Can't reach this Mac" line, so neither message was right for them. This matches the stated intent of 15-07 (a stopped share server must not look like an unreachable Mac) and is acceptable on a two-Mac tailnet; recording it as a known limit.
**Fix:** No code change needed. If a larger tailnet ever makes it noisy, filter the sidebar by `peer.OS == "macOS"`, or by a prior successful probe that carried `X-Portty: 1`.

### IN-05: Notification delegate is registered after launch, so the cold-launch click path is unproven

**File:** `tray_objc_darwin.m:173-180`
**Issue:** The comment says registering at launch lets a click on a notification from an earlier run reach the delegate. The registration runs inside a `dispatch_async` block queued from `initStatusItem` (called from Wails startup), so it can run after `applicationDidFinishLaunching` returns. Apple documents that a delegate meant to receive a launch-from-notification response should be set before launch finishes; whether a late assignment still receives the pending response is system behaviour I could not confirm from the code. The Go side drops the click if `trayCallbackApp` is nil. M-152 ("the click after relaunch also works") is the right check and is still "not yet run".
**Fix:** Make M-152's relaunch step a hard PASS condition. If it fails, call `registerNotificationDelegate()` synchronously (it is idempotent and bundle-guarded) from the earliest cgo init point rather than from the main-queue block.

### IN-06: `joinPasteViewer` copies `dialResume` instead of extending it

**File:** `internal/webserver/input_queue_test.go:46-50`, `internal/webserver/resume_test.go:117-124`
**Issue:** To pass an `OnPingReceived` hook, `joinPasteViewer` re-implements `dialResume` (Origin header, URL, `SetReadLimit`). A later change to `dialResume` (for example a new required header) would silently diverge for the paste tests only.
**Fix:** Give `dialResume` an optional ping hook (a variadic option or a sibling that `dialResume` calls) and have `joinPasteViewer` use it.

---

_Reviewed: 2026-11-23T15:30:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
