---
phase: 16-v1-2-cleanup
reviewed: 2026-11-23T00:00:00Z
depth: standard
files_reviewed: 15
files_reviewed_list:
  - cmd_cli.go
  - cmd_cli_test.go
  - cmd_share.go
  - cmd_share_test.go
  - dispatch_test.go
  - frontend/src/lib/__tests__/localhostUrl.test.ts
  - frontend/src/lib/__tests__/markdown.test.ts
  - internal/daemon/client_test.go
  - internal/daemon/types.go
  - internal/pty/cleanup.go
  - internal/relay/cwd_test.go
  - internal/relay/hub.go
  - internal/webserver/input_queue_test.go
  - internal/webserver/resume_test.go
  - main.go
findings:
  critical: 0
  warning: 0
  info: 3
  total: 3
  necessity: 1
status: issues_found
---

# Phase 16: Code Review Report (iteration 2)

**Reviewed:** 2026-11-23
**Depth:** standard
**Files Reviewed:** 15
**Status:** issues_found

## Summary

Re-review after the WR-01 and WR-02 fixes (commits c8158c94 and 0e08afca). Under GOTOOLCHAIN=go1.26.11, `go vet .` is clean and `go test . -run 'TestResolveSession|TestCmdKill|TestCmdRename|TestCmdShare|TestCmdUnshare|TestAttach|TestDispatch'` passes. I did not re-read the files from iteration 1 that the fixes did not touch (main.go, hub.go, cleanup.go, types.go, test helpers); the iteration-1 conclusion for them stands. Only the IN-02 location in hub.go was re-checked.

**WR-01 (short ID prefix on kill/rename): fixed and complete.** `lookupSessionStrict` (`cmd_share.go:77-86`) delegates to `lookupSession`, so ambiguity and no-match errors are unchanged. It then rejects a result unless the argument equals the session's name, equals its full ID, or is at least 8 characters. Traced cases:
- A name match of any length passes, because `s.Name == arg`.
- A full-ID match passes.
- An ID-prefix match under 8 characters returns `noSessionError{arg}`, so nothing is killed or renamed. The error text is identical to the genuine no-match text.
- A prefix of 8 or more characters still works. It must be unique, because `resolveSession` already rejected ambiguous prefixes.

There is no way to reach a prefix match with a short argument without hitting the guard. The kill and rename call sites (`cmd_cli.go:320,337`) are the only two destructive paths. `share`, `unshare` and `attach` still call the unchanged resolver. `TestCmdKillRename_ShortPrefixRefused` asserts both halves: kill and rename refuse, and `lookupSession` still accepts `id[:1]`.

**WR-02 (empty argument): fixed.** `resolveSession` now returns `noSessionError{""}` before the name pass, so an empty argument cannot match an unnamed session in kill, rename, share, unshare or attach. The `TestResolveSession` case with an unnamed session and `arg == ""` covers it. Effect on the other commands, apart from the intended rejection:
- `share ""` and `unshare ""` report "no session named". Before, they would have acted on a lone unnamed session.
- `attach ""` with a lone unnamed session no longer attaches to it. It falls through to `parseRemoteID("")`, which is non-remote, and then `attachRemote(client, "", "", ...)`. That is the same path `attach ""` already took when no unnamed session existed, so no new behaviour. `ConnectPeer("")` fails with a peer error or falls back to the `onNoHost` message.
- The ambiguity error and the 8-character prefix list are unchanged.

No new defects were introduced by the fixes. The remaining items are Info only.

## Info

### IN-01: Exact name shadows an exact full ID for destructive commands (carried forward)

**File:** `cmd_share.go:20-28` (used by `cmd_cli.go:320,337`)
**Issue:** The resolution order is name, then exact ID, then prefix. If a session has been renamed to another session's full ID, `portty kill <full-id>` kills the renamed session instead of the one with that ID. This is contrived, and `lookupSessionStrict` does not change it: `s.Name == arg` passes the guard. Nothing in kill or rename benefits from the shadowing.
**Fix:** Optional. Check the exact ID before the name for kill and rename, or note the order in the usage text.

### IN-02: ED 3 in the alternate screen still wipes history, though vt keeps the main screen's scrollback (carried forward)

**File:** `internal/relay/hub.go:1297-1312`
**Issue:** The IN-03 guard `if !emu.IsAltScreen()` correctly stops `Skip` from counting scrollback that vt does not drop. `h.history.Clear()` at line 1304 and `h.wraps.clear()` at line 1314 still run unconditionally. In the alternate screen, vt leaves the main screen's scrollback untouched, but the hub's drained history (the replay scrollback) is emptied. A full-screen app that emits ESC[3J in the alternate screen removes the main-screen transcript from replay. `TestCwd_AltScreenScrollbackEraseKeepsRows` asserts that the cwd rows for that transcript are kept. This is not a regression, since it predates the phase, and clearing history may be the intended conservative choice for T-05-10. The new comment does not say so.
**Fix:** Either state in the comment that the unconditional history clear is intentional, or put `Clear()` and `wraps.clear()` under the same `!emu.IsAltScreen()` condition and add a replay test.

### IN-03: `arg != ""` guard in the prefix pass is now dead

**File:** `cmd_share.go:44`
**Category:** necessity
**Issue:** The condition `arg != "" && strings.HasPrefix(s.ID, arg)` can no longer see an empty `arg`, because `resolveSession` returns on `arg == ""` at lines 16-19. The extra check is leftover from before commit c8158c94. Evidence it cannot occur: `arg` is not reassigned between line 16 and line 44, and the function has no other entry path.
**Fix:** Drop the redundant `arg != "" &&` so the early return is the single place that handles empty input:
```go
if strings.HasPrefix(s.ID, arg) {
```

---

_Reviewed: 2026-11-23_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
