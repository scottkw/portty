---
phase: 04-one-click-share
reviewed: 2026-11-17T00:00:00Z
depth: standard
scope: incremental (gap-closure plans 04-07..04-10, diff 3d0c1d5c..HEAD)
files_reviewed: 21
files_reviewed_list:
  - internal/attach/attach.go
  - internal/daemon/attach_viewers_test.go
  - internal/daemon/daemon_log.go
  - internal/daemon/daemon_log_test.go
  - internal/daemon/engine.go
  - internal/daemon/peer_connect.go
  - internal/daemon/process.go
  - internal/daemon/process_unix.go
  - internal/daemon/remote_ws_proxy.go
  - internal/daemon/remote_ws_proxy_test.go
  - internal/daemon/share.go
  - internal/daemon/share_test.go
  - internal/relay/hub.go
  - internal/relay/hub_test.go
  - internal/relay/scrollback.go
  - internal/webserver/server.go
  - frontend/src/App.tsx
  - frontend/src/components/TabBar.tsx
  - frontend/src/components/__tests__/App.tailscale.test.tsx
  - frontend/src/components/__tests__/App.test.tsx
  - TESTING.md
findings:
  critical: 1
  warning: 5
  info: 5
  total: 11
  necessity: 0
status: issues_found
---

# Phase 4: Code Review Report (gap closure 04-07..04-10)

**Reviewed:** 2026-11-17
**Depth:** standard (incremental: `git diff 3d0c1d5c HEAD`, surrounding code read where needed)
**Files Reviewed:** 21
**Status:** issues_found

## Summary

This review covers the four gap-closure plans: the query strip on the catch-up replay (04-07), daemon.log and the `viewer-diag:` lines (04-08), the tailnet host name and the Tailscale-off Shared button (04-09), and `relay.ViewerReadLimit` (04-10). The previous 04-REVIEW.md (04-01..04-06) is replaced; its fixes are recorded in 04-REVIEW-FIX.md.

Items the orchestrator asked about, checked against the code:

- **Tokens and input in daemon.log:** clean. The proxy removes `u.RawQuery` (never empty, since it is always `cap=...`) and then the raw token before logging a dial or end error. Client names and close reasons are logged with `%q`, so a guest cannot forge log lines. Input is logged only as counts. The file is created 0600 inside the 0700 config dir. `TestRemoteWSProxy_DiagLog` and `TestCLIAttach_OtherViewersKeepOutput` check that the token never appears. The problem is volume, not secrecy: see CR-01 and WR-01.
- **ViewerReadLimit:** 1 MiB, set only on readers of host-to-viewer frames (the proxy's upstream conn and the CLI conn in `AttachSession`). The share server, the local relay and the proxy's client side keep the 32 KiB default, so guests cannot make the host buffer large messages. The limit's own bound is only estimated (WR-03).
- **Query-strip regex:** `liveEmuQueryStripPattern` (hub.go:122-133) and `queryStripPattern` (engine.go:448-459) are byte-identical. Every alternative is a reply-eliciting query: `CSI > q` cannot match DECSCUSR (`CSI SP q`) or DECSCA (`CSI " q`), and `CSI ? u` cannot match the kitty push, pop or set forms. No legitimate rendering output is stripped. The list of queries is incomplete, though (WR-02).
- **Tailscale-off Share:** `shareDisabled={tailscaleOff && !activeShared}` cannot enable Share-on. The popover's own `blocked = tailscaleOff && !shared && !busy` (SharePopover.tsx:266) still blocks the switch once the session is unshared. One UX gap remains (WR-05).
- **magicDNSLabel:** correct for an FQDN, a trailing dot, a bare label, an IP literal and an empty string. `h.Domain` is guaranteed non-empty before `startShareServer` (share.go:140).

`go vet` is clean on the changed packages. `tests/check-traceability-paths.sh` passes. The TESTING.md counts (Go 112, vitest 82) match `git ls-files`.

## Narrative Findings (AI reviewer)

## Critical Issues

### CR-01: A read-only guest can fill the host's disk via one unthrottled log line per resize frame

**File:** `internal/webserver/server.go:895-902` (with `internal/daemon/daemon_log.go:15-23`)
**Issue:** The `relay.MsgResize2` arm of the share server's read pump writes a `viewer-diag: web resize ignored` line for every frame of 4 or more bytes. Nothing checks `sub.ReadOnly`, nothing rate-limits, and nothing logs only once per connection. Any holder of a share link can send these frames as fast as the tailnet carries them, including a `perms="read"` viewer that is meant to have no effect on the host. A 5-byte frame produces about 120 bytes of log, a 24x amplification, and daemon.log has no size cap during a daemon run (WR-01). It is rotated only when a daemon is spawned. A modified client could write gigabytes per hour into `~/Library/Application Support/portty/daemon.log` on the office Mac. Before 04-08 this output went to /dev/null, so this phase introduced the issue. The comment's premise ("Guests never send this frame") holds only for the bundled frontend, not for an untrusted client at a trust boundary.
**Fix:** Log once per connection, as `first input` already does, and fold the count into the end-of-connection line:
```go
case relay.MsgResize2:
    resizeFrames++
    if resizeFrames == 1 && len(payload) >= 4 {
        log.Printf("viewer-diag: web resize ignored session=%s conn=%d cols=%d rows=%d", ...)
    }
// and in the deferred summary:
log.Printf("viewer-diag: input total session=%s conn=%d frames=%d bytes=%d resizes=%d", ..., resizeFrames)
```
Also add a size cap to daemon.log (WR-01) so no future per-frame line can turn into a disk-fill.

## Warnings

### WR-01: daemon.log is never rotated during a run, and every spawn attempt rotates away the previous run's log

**File:** `internal/daemon/daemon_log.go:15-23`, `internal/daemon/process_unix.go:19-26`
**Issue:** There are two defects:
1. **No size cap.** The daemon runs for days or weeks, and the file is only rotated when `startDetachedDaemon` runs. Besides CR-01, the share server's `http.Server` (server.go:348) has no `ErrorLog`, so every failed TLS handshake from any device that can reach the port is now persisted ("http: TLS handshake error from ..."). These connections are unauthenticated, and before this phase their log lines went to /dev/null.
2. **Rotation destroys the evidence it exists to keep.** `openDaemonLog` runs in the parent on every spawn attempt, before anyone knows whether the child starts. Two `EnsureDaemon` callers racing (GUI launch plus a CLI command), or one retry after a spawned daemon fails "did not become ready within 3s", each rename `daemon.log` to `daemon.log.1`. The second rename overwrites the run that crashed, which is the run M-83 step 4 tells the user to read. With one generation kept, two quick spawns lose the post-mortem.
**Fix:** Cap the file size. The simplest option keeps the descriptor: in the daemon itself, have `log.SetOutput` use a small writer that truncates or rotates past, say, 10 MiB. Also keep more than one generation, or rotate only when the existing file is non-empty and older than the live daemon (for example, rotate inside `RunDaemon` after the socket bind succeeds, not in the spawning parent). Set `ErrorLog` on the share server's `http.Server` to a filtered or rate-limited logger.

### WR-02: The replay query strip misses several common reply-eliciting queries, so H-replies is only partly fixed

**File:** `internal/relay/hub.go:122-133` (mirror `internal/daemon/engine.go:448-459`); comment at `internal/relay/hub.go:615-621`
**Issue:** The denylist covers DA1, DA2, DSR/DECXCPR, DECRQM, 2048h/l, XTVERSION, kitty `CSI ? u` and OSC 10/11/12. Queries that common TUIs send at startup are not covered, so they still sit in the raw ring and a joining CLI's terminal answers them into the shared shell:
- XTWINOPS report requests `CSI 14 t`, `CSI 16 t`, `CSI 18 t` (window or cell size; image tools, yazi, neovim).
- XTGETTCAP `DCS + q ... ST` and DECRQSS `DCS $ q ... ST` (neovim sends both on startup).
- DA3 `CSI = c`, OSC 4 palette queries `OSC 4;n;? ST`, and OSC 52 clipboard reads `OSC 52;c;? ST`. If a joining terminal permits clipboard reads, a replayed OSC 52 query types the viewer's clipboard into the host shell.

The doc comment says "A joining terminal answers every historic query ... (DSR, DA2, DECRQM, XTVERSION, kitty)" and "mode sets and colours stay". The first implies the list is complete. The second is inaccurate because `CSI ? 2048 h` is a mode set and is stripped. `InputAbsorber` only drops OSC 10/11 and DA1 replies, so nothing on the host catches the rest.
**Fix:** Extend both patterns identically. The XTWINOPS alternative must leave the state-changing `CSI 8;r;c t` alone:
```go
`|\x1b\[=[0-9;]*c` +                         // DA3
`|\x1b\[(?:1[1-9]|2[01])t` +                 // XTWINOPS report requests (11t..21t)
`|\x1bP[+$]q[^\x1b\x07]*(?:\x07|\x1b\\)` +   // XTGETTCAP / DECRQSS
`|\x1b\](?:4;[0-9]+|52;[a-z0-9]*);\?(?:\x07|\x1b\\)` // OSC 4 / OSC 52 queries
```
Add each case to `TestReconnectPreamble_StripsTerminalQueries` and the `queryReplies` table. Correct the comment to name 2048h/l as the one stripped mode set.

### WR-03: ViewerReadLimit's bound is estimated, not enforced; a large styled screen reproduces UAT gap 4

**File:** `internal/relay/scrollback.go:8-25`; `internal/relay/hub.go` `RenderSnapshot` / `ReconnectPreamble`
**Issue:** The raw-ring preamble is hard-capped at 256 KiB, but once the ring has wrapped, the preamble is `RenderSnapshot()` of the live screen sent as one message, and its size is unbounded. The comment's "about 20 bytes per styled cell" is a guess. A truecolor cell transition (`ESC[38;2;r;g;b;48;2;r;g;bm` plus the glyph) is 35-40 bytes, so a 400x120 screen of per-cell gradients (btop graphs, chafa, any image-like TUI on an external monitor), plus up to 50 scrollback lines, passes 1 MiB. The proxy upstream or CLI then fails with `message too big` immediately after the grid frame, which is exactly the gap-4 signature the plan closed.
**Fix:** Remove the dependency on a per-message limit by sending the preamble in chunks. MsgOutput payloads can be concatenated, so at both send sites (webserver/server.go and relay/server.go) split the preamble into frames of at most 32 KiB: `0x01` plus up to 32767 payload bytes, preserving byte order. Then 32 KiB readers work, and ViewerReadLimit is defence-in-depth only. If chunking is deferred, at least make `RenderSnapshot` refuse or truncate past `ViewerReadLimit-1` and log a `viewer-diag:` line, so the failure is visible and not a silent drop.

### WR-04: Viewer input over 32 KiB still drops the session; the new proxy comment treats this as harmless

**File:** `internal/daemon/remote_ws_proxy.go:66-67` (evidence: `frontend/src/lib/relayClient.ts:155-158`, `frontend/src/components/TerminalPanel.tsx:403`, `internal/attach/attach.go:73-108`)
**Issue:** The new comment says the proxy's client side can keep the 32 KiB limit because it "only carries viewer input, which the host's share server limits to 32 KiB anyway". But xterm's `onData` delivers a paste as one string, and `sendInput` sends it as one `0x00`-framed message. A paste of 32768 bytes or more in a peer GUI tab exceeds the default limit (coder/websocket's limitReader allows at most 32768 bytes per message). The proxy closes the tab with 1009, and a browser guest is closed by the share server the same way. The CLI's `StdinPump` uses a 32 KiB buffer, so a full read produces a 32769-byte frame, the same off-by-one 04-10 fixed in the other direction. The code predates this phase, but 04-10 reasoned about this direction and wrote down the wrong conclusion.
**Fix:** Chunk on the sender. In `relayClient.sendInput`, split UTF-8-encoded input into payloads of at most 32767 bytes. In `StdinPump`, use `make([]byte, 32*1024-1)`. Update the comment to say input frames are chunked by every sender.

### WR-05: After turning Share off with Tailscale off, the toolbar button can no longer close its own open popover

**File:** `frontend/src/App.tsx:916`, `frontend/src/components/TabBar.tsx:301-303`
**Issue:** Turning the switch off calls `onSharedChange`, so `activeShared` becomes false and `shareDisabled` becomes true while the popover is still open. TabBar then drops `onClick` from the button. SharePopover's outside-press handler deliberately ignores presses on its anchor (`SharePopover.tsx:143`: `anchor?.contains(t)`) and expects the button's click to close it. Clicking the now-dimmed Share button therefore does nothing, and the popover stays open with a blocked switch until the user presses Esc or clicks elsewhere. The button still describes itself as the popover's controller.
**Fix:** Keep the button working as a close control while the popover is open:
```tsx
shareDisabled={tailscaleOff && !activeShared && !sharePopoverOpen}
```
Alternatively, have TabBar keep `onClick` when `shareOpen` is true. Add a mounted case to App.tailscale.test.tsx: switch off, then click the button, and the popover closes.

## Info

### IN-01: Stale doc comment above magicDNSLabel

**File:** `internal/daemon/peer_connect.go:45-49`
**Issue:** The old `// peerShortName is the MagicDNS short name...` comment now sits directly above `magicDNSLabel`'s own comment, so godoc for `magicDNSLabel` begins with text about `peerShortName`, and `peerShortName` has no doc comment.
**Fix:** Move lines 45-46 down to just above `func peerShortName`.

### IN-02: The grid line's `by=` is always "local" and names the caller, not the size source

**File:** `internal/relay/hub.go:322-323`
**Issue:** `ResizeClient` returns early for any non-local origin (hub.go:288), so `by=%s` is always `local`. When a larger window triggers the recompute, the new minimum comes from a different subscriber, but the line names the caller. For H-grid diagnosis the useful fact is which client owns the minimum.
**Fix:** Record the subscriber that supplied `minCols`/`minRows` in the loop and log its `ID` and `Name`. Drop `by=`.

### IN-03: copyEnd.side blames the read side for a write failure

**File:** `internal/daemon/remote_ws_proxy.go:159-170`
**Issue:** `side` names the conn being read, but the second `errc <-` reports a failed `dst.Write`. If the peer drops while client input is being forwarded, the line says `first=client`, which points M-83's diagnosis at the wrong Mac.
**Fix:** Report `side+"-write"` (or the dst side) on the write-error path.

### IN-04: Microsecond timestamps only on the RunDaemon entry point

**File:** `internal/daemon/process.go:23`; `internal/daemon/service.go:25`
**Issue:** `log.SetFlags(... | log.Lmicroseconds)` is set in `RunDaemon` only. The service-manager path calls `runDaemonCore` directly and logs viewer-diag lines at second resolution, so lines cannot be ordered the way M-83 assumes.
**Fix:** Move the `SetFlags` call to the top of `runDaemonCore`.

### IN-05: The close error is extracted twice in the proxy end path

**File:** `internal/daemon/remote_ws_proxy.go:131-147`
**Issue:** `websocket.CloseStatus(err)` plus `errors.As(err, &ce)` in the log branch is repeated a few lines later for the forwarded close, with a shadowed `ce`.
**Fix:** Run `errors.As` once before logging and reuse `ce` for both the log line and `clientConn.Close`.

---

_Reviewed: 2026-11-17_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
