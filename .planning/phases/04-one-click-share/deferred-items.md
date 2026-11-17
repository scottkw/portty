## Deferred Items

- Proxied joiners are dropped when the catch-up replay is over 32 KiB (found during 04-07)
  status: fixed in 04-10
  **Fix:** `relay.ViewerReadLimit` (4x the ring, 1 MiB) on the proxy upstream and in `attach.AttachSession`; `TestCLIAttach_LongHistoryJoiners`; input-side readers keep 32 KiB.
  **What:** No Go WebSocket reader in the attach path calls `SetReadLimit`, so coder/websocket's default 32768-byte message limit applies. The host sends the whole raw scrollback ring (up to 256 KiB) as one preamble message. The viewing daemon's proxy (`internal/daemon/remote_ws_proxy.go`, `upstream.Read` in `copyWS`) fails on a larger preamble and tears down the joiner. A local CLI attach (`attach.WsOutputPump`) has the same limit.
  **Evidence:** a scratch run of the 04-07 harness with about 52 KiB of history: the CLI through `/api/relay/remote/{id}/ws` got the grid frame, then EOF. GUI tabs already connected were unaffected, and browsers have no such limit.
  **Why deferred:** pre-existing, outside 04-07's scope. It kills the joiner's own connection, not the other viewers (not UAT gap 1's signature). It does match "a GUI tab through the proxy can't rejoin a long session", so it is worth checking against the 04-08 diagnostic log.
  **Fix sketch:** raise the read limit on the proxy's upstream conn and on the CLI conn (for example `conn.SetReadLimit(1 << 20)`, above the 256 KiB ring), plus a harness variant with more than 32 KiB of history.

- `internal/relay/scrollback_altscreen_test.go` is not gofmt-clean on the 04-07 base
  status: open
  **What:** `gofmt -l internal/relay` lists it. 04-07 did not touch the file. Out of scope.

- `TestRevoke_InputAfterRevokeNeverReachesPTY` (internal/webserver/revoke_test.go) failed once under the full `go test -race -short ./...` run during 04-07
  status: open
  **What:** `input after revoke reached the PTY: "after-revoke\n"`. It passed 30/30 in isolation and on a full-suite rerun. 04-07 changed nothing under internal/webserver. Likely cause: `Hub.RevokeWebViewers` runs `Kick` in a goroutine, so a frame written right after the revoke can reach the read pump before the close lands. Worth a look, because share-off is meant to stop input at once.

- `TestAPIAppearance_GetPatch` and `TestAPITerminalFont_GetPatch` (internal/daemon) read the real settings.json (found during 04-08)
  status: open
  **What:** both fail with `default GET: want system, got "dark"` and `want {"" 13}, got {Family: Size:16}`. The values match the user's real `~/Library/Application Support/portty/settings.json` (appearance dark), so `testDaemon` reads the real config dir for these settings instead of a temp dir. They fail the same way on the 04-08 base commit 0a6e451d (checked in a temporary worktree), so 04-08 did not cause it; they passed during 04-07 only because the real settings were still at their defaults.
  **Also seen:** the full run prints `daemon: daemon already running at /Users/ken/Library/Application Support/portty/daemon.sock`, so some daemon test still resolves the real socket path.
  **Fix sketch:** point the settings path used by `testDaemon` at `t.TempDir()` (as `ConfigDirForTest` does for the engine), then re-run the full suite with a non-default real settings.json.

- `TestExitEvent_ListSessions_ExitCodePopulatedForStopped` (internal/daemon/engine_exit_test.go) failed once under the full `go test -race -short ./...` run during 04-10
  status: open
  **What:** `ExitCode for running session: want nil, got 0x...` right after `CreateSession`, so the `/bin/cat` session already read as stopped. It passed 20/20 in isolation and on a full-suite rerun. 04-10 changed only WebSocket read limits, nothing in the engine. Likely a timing race in the test under full-suite load. Out of scope.
