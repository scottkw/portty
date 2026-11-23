# Phase 10: Input Path Reliability — Context

**Gathered:** 2026-11-20 (autonomous smart discuss; reversible technical choices decided and logged; the one product decision, INPT-06 "viewer takes over size", was made by the user on 2026-11-19)
**Status:** Ready for planning

<domain>
Pasting into a shared session is trustworthy everywhere, and the viewer you're typing on sets the session's size (INPT-01..06). The todos in .planning/todos/pending (paste-heartbeat-drop, paste-notice-false-positive, peer-snapshot-tab-replace, web-viewer-paste-notice, exitcode-stopped-flake) hold the problem analysis and file pointers for INPT-01..05; read them in full. INPT-06 comes from a v1.1.0 report: Cmd+ in a peer tab can't resize the office session (`frontend/src/components/WebShareSessionView.tsx:197` has `onFontSizeChange={() => {}}`; `internal/relay/hub.go:479` ResizeClient drops non-local origins, VIEW-02; `internal/webserver/server.go:972` drops web resizes).
</domain>

<decisions>

### Locked by the user and the roadmap
- **INPT-06 (user, 2026-11-19: "Viewer takes over size"):** the viewer you are using sets the session's grid. Typing in it, or zooming/resizing it, makes the shell reflow to that viewer's grid. When another viewer (the host's local tab included) types or zooms/resizes, it takes the size back. Works for local tabs, peer tabs, browser viewers and `portty attach`.
- Keep the 32 KiB per-frame guard, share tokens, CSP and the tailnet-only listener unchanged (INPT-03 must not touch CSP or use innerHTML).

### Decided autonomously (reversible)
| Question | Decision | Why |
|---|---|---|
| INPT-01 input path | A bounded per-connection input queue (about 1,024 frames / 32 MiB) between the read pump and `hub.WriteInput`, drained in order by a writer goroutine that outlives the socket, so nothing already read is lost. The pump blocks only when the queue is full. The daemon proxy (`internal/daemon/remote_ws_proxy.go`) gets the same treatment so its own upstream ping can still be written. | The todo's analysis: it is the only fix that covers all three liveness paths (server heartbeat, proxy ping, browser watchdog). |
| What a pong acknowledges after INPT-01 | "Read by the server and queued for the PTY in order". The writer outlives the socket, so queued bytes still reach the shell after a drop; the DEBT-05 notice keeps its "may have been cut short" copy for anything sent after the last pong. Update the comments and tests in `relayClient.ts` (`ackPing`). | Honest under the new lifetime; no copy change. |
| INPT-02 pong delivery | The pong must not be dropped when `sub.Msgs` is full: send it out of band (owed-pong flag resent on the next drain, or a blocking send with a short timeout). Planner picks after reading the write path; the test is a full `sub.Msgs` when the ping arrives and the client clearing the pending paste without a heartbeat-length wait. | Either satisfies SC2; the choice is internal. |
| INPT-03 browser notice | terminal.js sends `[MsgPing]` after a paste of 2+ frames, tracks the pong, and on close with a pending paste shows "Paste may have been cut short" in the status bar via textContent. Same copy as the app. | Matches the GUI's DEBT-05 design; CSP unchanged. |
| INPT-04 peer tab identity | Key `webSessionTabId` by host plus session everywhere (restore, close, snapshot keys), and `retryInit` drops the snapshot of every peer tab it removes. | The todo's solution; prevents two Macs' equal ids from sharing a tab. |
| INPT-05 flake | Reproduce with `-count=20` over the whole package, capture the message, then make the test hermetic (no live process racing SetState) or wait for the session's wait loop before SetState. | The todo's sketch. |
| INPT-06 grid authority model | The hub keeps a **grid owner**: the subscriber that most recently sent input or made a user-initiated size change. The grid is the owner's last reported size. Joining, reconnecting and passive output never claim (keeps Phase 9's delta resumes after a handoff). When the owner leaves, the grid stays as it is (D-01 freeze) until someone claims. This replaces D-02 min-among-local. | Directly implements the user's decision; "passive join never claims" keeps reconnects from moving the grid. |
| INPT-06 telling a claim from a size report | Each viewer keeps reporting its size (as today) so the hub knows it; a size change the user caused (window resize, Cmd+/Cmd−, attach's SIGWINCH) is sent as a **claim**. Input from a non-owner claims with that viewer's last reported size. Older viewers' plain resize frames are treated as reports only (no claim), and old hosts ignore claims, so mixed versions degrade to today's behavior. The planner picks the wire form (a new frame type or a flag) and whether `/health` `api` goes to 3. | Needed so the initial fit on join is not a claim. Backward compatible both ways. |
| INPT-06 other viewers | A viewer that is not the owner shows the session at the owner's grid: peer tabs and the browser use the existing guest scaling (`computeGuestScale` / readability floor); a local tab that loses ownership uses the same guest scaling instead of fitting the PTY to itself. `portty attach` shows it as a terminal shows any foreign-size program. | Reuses the existing guest path; no new UI. |
| INPT-06 safety | A guest already has full write access to the shell (one read,write capability per share), so letting it size the PTY is no escalation. Clamp claimed sizes to sane bounds (for example 20..500 cols, 5..200 rows) and ignore zero sizes. | Trust boundary validation. |
| INPT-06 vs the Phase 9 settle window | With "joins never claim", the post-handoff settle window may become redundant. The planner decides whether to keep it, simplify it or remove it; Phase 9's V-02 behavior (no grid change and delta resumes after a handoff) must still hold, pinned by its existing test. | Chesterton's Fence: it exists for reconnect order; the new model may cover that. |

### Decided after research (2026-11-20, autonomous; see 10-RESEARCH.md)
| Question | Decision | Why |
|---|---|---|
| INPT-03 target (Open Question 1) | Don't edit `web/assets/terminal.js`: `/sessions/{id}` redirects to `/app/` (`webserver/server.go:787-795`), so the live browser viewer is `WebShareSessionView` → `TerminalPanel` → `RelayClient`, which already has the notice (`TerminalPanel.tsx:1156`). Prove SC3 with a vitest on the guest `wsURL` path plus the VM `tailscale down` drop in a real browser. Leave terminal.js as is (Chesterton: it is dead but removing it is out of scope). | The requirement is about what the user sees in a browser; that path is the React viewer. |
| INPT-01 writer placement (A5) | One hub-owned FIFO with per-connection quotas (1,024 frames / 32 MiB) and one writer goroutine, instead of per-connection writers. Both pumps (webserver and the local relay pump `relay/server.go:170-176`) feed it. The daemon proxy gets a backpressure test first and a queue only if it fails. | Keeps "bounded, in order, outlives the socket" and adds global order so a dropped viewer's backlog can't interleave with a reconnect's input. |
| INPT-06 details | Accept the research design: claim bit in byte 5 of `MsgResize2`; `/health` api stays 2; owner replaces min-among-local; a local report bootstraps only a never-sized grid; typing claims only for real user input (classifier excludes xterm auto-replies); read-only caps never claim; clamp 20..500 x 5..200; claims from window resize/font change, never ResizeObserver; per-tab font size for peer tabs. Remove the Phase 9 settle window and rewrite its test as a rejoin test; re-run V-02 in the VMs. | Follows the user's "viewer takes over size" decision; the settle window's cause (a join moving the grid) no longer exists. |
| Accepted limits | A relaunched GUI shows the current grid (letterboxed/scaled) until you type, zoom or resize; `portty attach` can show a grid wider than its terminal until its user types; a paste over the 32 MiB quota into a slow reader can still drop the viewer; an upgrade during a slow paste can lose bytes still queued in the old daemon (bounded wait in `oldHandoff.finish`, loss logged). Each gets a `shortcut:` comment. | Edge cases; the alternative designs cost more than they save. |

### Claude's Discretion
- How to split the plans, file names, test names, queue sizes, timeouts.
</decisions>

<code_context>
- internal/webserver/server.go (read pump MsgInput and ping cases, ~line 972 resize drop), internal/relay/server.go:181 (local resize → ResizeClient), internal/relay/hub.go (ResizeClient ~479, applyGridLocked, WriteInput ~1427, the handoff settle window from 50903bee)
- internal/daemon/remote_ws_proxy.go (peer proxy copy loops and ping)
- frontend/src/lib/relayClient.ts (paste, ackPing, watchdog), web/assets/terminal.js (browser viewer, makeInputFrames, updateStatusBar)
- frontend/src/App.tsx (retryInit, webSessionTabId, openWebSessionTab, peer snapshots), frontend/src/components/WebShareSessionView.tsx:197, TerminalPanel.tsx (local fit), frontend/src/lib/terminalScale.ts
- internal/attach (CLI resize/SIGWINCH), internal/daemon/engine_exit_test.go (INPT-05)
</code_context>

<specifics>
- Toolchain: `export GOTOOLCHAIN=go1.26.11`.
- Gates: `go build ./... && go vet ./...`, `go test -race -short ./...` (nothing skipped), `cd frontend && pnpm vitest run` and tsc, check-brand, check-traceability. Run each full suite at most once per plan (each test binary fires a LuLu alert for the user); save full output to files, never pipe through tail. Keep the tailnet guard.
- VM checks on the two Tart VMs (office `portty-test` 192.168.64.108, home `portty-test-2` 192.168.64.109; ephemeral tailnet nodes: never stop them, never `tailscale down/logout` except the scripted INPT-03 drop on the office VM followed by `tailscale up` within seconds — confirm `tailscale up` works without a key before relying on it). M-101 paste run for SC1, SC3's `tailscale down` drop, SC6 zoom/resize checks with `tput cols`.
- Ad-hoc signing only; fault injection only inside the VMs; never touch the user's running Portty on the host.
- Worktrees start at an old commit: fast-forward to gsd/v1.2 first. Don't commit planning files on gsd/v1.2 while executors run.
</specifics>

<deferred>
- A visible "who has the size" indicator in the UI (not asked for; add only if the VM check shows it is confusing).
</deferred>
