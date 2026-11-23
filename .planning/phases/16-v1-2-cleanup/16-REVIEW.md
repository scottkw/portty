---
phase: 16-v1-2-cleanup
reviewed: 2026-11-23T00:00:00Z
depth: standard
files_reviewed: 13
files_reviewed_list:
  - cmd_cli.go
  - cmd_cli_test.go
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
  warning: 2
  info: 2
  total: 4
status: issues_found
---

# Phase 16: Code Review Report

**Reviewed:** 2026-11-23
**Depth:** standard
**Files Reviewed:** 13
**Status:** issues_found

## Summary

Reviewed the phase-16 diff (`cc595307..HEAD`) with surrounding code. Verified by running `go test . -run 'TestLaunchMode|TestCmdKill|TestCmdRename|TestDispatch'` under GOTOOLCHAIN=go1.26.11 (pass; the repo's `toolchain_guard.go` rejects the local Go 1.27 on purpose). `bash tests/check-traceability-paths.sh` reports OK.

**launchMode (main.go):** no defect found. With no args it opens the GUI. A leading `-psn_*`, `-NS*` or `-Apple*` argument opens the GUI, and only `args[0]` is inspected, so the value arguments that follow such a flag are harmless. `--help`/`-h`, `--version`/`-v` and the daemon flags are matched before the allowlist, and every other leading flag returns `modeHelp` with an error, which `main` turns into exit 2 with no GUI. Nothing in the repo launches the app with other flags: `os.Args` is read only in `main.go`, and `wails.json`, `Info.plist` and the Go sources pass no `--args`. `--version` maps to `["version"]`, which `cliNeedsDaemon` skips, so it starts neither the daemon nor the GUI.

**hub.go ED 3 guard:** correct. In the vt version in `go.sum`, `ScrollbackLen` always reads `scrs[0]`, and ED 3 clears the current screen's scrollback. I checked the new test is discriminating by removing the guard in a throwaway worktree: `TestCwd_AltScreenScrollbackEraseKeepsRows` then fails with `MARK-A resolves to "/b"`.

**cleanup.go, types.go, test-helper changes:** the `killSession` branch merge is behaviour-identical. `PathClickAPI` has no remaining production or test reader.

**cmd_cli.go kill/rename:** the ambiguous-name and unknown-name paths are safe and tested. The two warnings below concern the unique-ID-prefix and empty-argument paths, which `lookupSession` takes from share/attach where they are harmless. They are not harmless for destructive commands.

## Warnings

### WR-01: `portty kill` / `rename` accept a one-character ID prefix, so a name abbreviation can kill an unrelated session

**File:** `cmd_cli.go:317-330` (resolution in `cmd_share.go:36-50`)
**Issue:** `resolveSession` falls back to a unique ID prefix with no minimum length, and session IDs are hex (`internal/pty/native.go:170`). Typing a short name abbreviation that does not equal any full name, such as `portty kill b` for a session named `build`, falls through to the prefix pass. If exactly one session's hex ID starts with `b`, that session is killed. The same applies to `a`, `c`, `d`, `e`, `f` and digits, and to `rename`. With a handful of sessions the chance of a unique one-character hit is high. Killing is irreversible and ends a live shell, which is the "wrong session" outcome this phase was meant to prevent. The same fallback is harmless for share/attach, but not here. The ambiguity error already prints 8-character prefixes, so 8 is the natural minimum.
**Fix:** For kill and rename, accept an ID prefix only at 8 characters or more, and give any shorter non-matching argument the no-session error. For example, add an options argument or a second helper:
```go
// in resolveSession, prefix pass:
if len(arg) >= minPrefixLen && strings.HasPrefix(s.ID, arg) { ... }
```
Use `minPrefixLen = 8` for kill and rename. Add a table case in `TestCmdKill_ByName`: with sessions "build" and one whose ID starts with `b`, `cmdKill(["b"])` kills nothing.

### WR-02: Empty argument matches a session with an empty name

**File:** `cmd_share.go:18-23` (reached from `cmd_cli.go:319,332`)
**Issue:** The name pass has no `arg != ""` guard (the prefix pass does). Sessions can have an empty name: `RenameSession` stores whatever it is given (`internal/daemon/engine.go:449-455`, with no empty check), and `ListSessions` returns `tabNames[id]` unmodified. If exactly one session has an empty name, `portty kill "$NAME"` with an unset or empty variable kills it, and `portty rename "" x` renames it. A script with an unset variable is a typical way to hit this. Before this phase, `portty kill ""` hit the daemon as `DELETE /sessions/` and failed harmlessly.
**Fix:** Reject an empty argument at the top of `resolveSession`:
```go
if arg == "" {
    return daemon.SessionInfo{}, noSessionError{arg}
}
```
Add a `TestResolveSession` case with an unnamed session and `arg == ""`.

## Info

### IN-01: Exact name shadows an exact full ID for destructive commands

**File:** `cmd_share.go:16-31` (used by `cmd_cli.go:319,332`)
**Issue:** Resolution order is name, then exact ID, then prefix. If a session is named with another session's full ID, which the user can do with `rename`, then `portty kill <full-id>` kills the named session instead of the session with that ID. This is contrived, but it is the only way to name a session precisely, and kill gains nothing from the shadowing.
**Fix:** Optional. Check exact ID before name for kill and rename, or document the order in the usage text.

### IN-02: ED 3 in the alternate screen still wipes history, though vt keeps the main screen's scrollback

**File:** `internal/relay/hub.go:1297-1312`
**Issue:** The IN-03 guard correctly stops `Skip` from counting scrollback that vt does not drop. The line above it, `h.history.Clear()`, and `h.wraps.clear()` below still run unconditionally. In the alternate screen, vt leaves the main screen's scrollback untouched, but the hub's drained history (the viewers' replay scrollback) is emptied. A full-screen app that emits ESC[3J while in the alternate screen therefore removes the main-screen transcript from replay, while `TestCwd_AltScreenScrollbackEraseKeepsRows` asserts that the cwd records for that transcript are kept. This is not a regression, since it predates the phase, and clearing history may be the intended conservative choice for T-05-10. It is flagged because the new comment says nothing is dropped on the main screen while the history is dropped anyway.
**Fix:** Either confirm the behaviour is intended and say so in the comment, or move `h.history.Clear()` and `h.wraps.clear()` under the same `!emu.IsAltScreen()` condition and add a test for the replayed history.

---

_Reviewed: 2026-11-23_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
