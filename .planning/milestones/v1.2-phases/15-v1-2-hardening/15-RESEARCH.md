# Phase 15: v1.2 Hardening - Research

**Researched:** 2026-11-22
**Domain:** Go daemon/relay/PTY hardening, `portty attach` CLI, React/xterm peer-tab and link handling, macOS native notifications, test-flake removal
**Confidence:** HIGH for the code paths and the fixes I prototyped; MEDIUM for the sidebar "Can't reach this Mac" cause (derived from code, not reproduced) and for the macOS notification-click delegate (no bundled run possible here)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Update notification click**
- Clicking the "Portty X is available" notification brings Portty forward (shows the window if hidden/tray-only) with the existing UpdateBanner in its "available" state; it does NOT start the download. Nothing downloads without the user pressing Download. (User said "You decide"; chosen for consent and smallest change.)
- Full click verification needs a notarized build in the Tart VM (ad-hoc builds can't post notifications); automate what can be automated (handler wiring, routing to show-window) and record the live click check as a manual TESTING.md item.

**Decided autonomously**
- [Attach bar] Q: "How to keep full-screen apps off the status bar row?" → Selected: "Send rows-1 (bar shown) in the initial resize frame and on every SIGWINCH claim, for bottom and `--status-top`; the existing hub grid-owner model (claims + NoteTyping) makes the claiming/typing viewer's reduced height win; a test pins rows-1" — two call sites in cmd_attach.go / internal/attach, local revert
- [Ended peer tab] Q: "What does clicking an already-ended peer tab show?" → Selected: "Keep the tab's ended state; once a tab saw the session-ended close, no_such_session maps to 'session ended', not 'stopped sharing'" — frontend verdict mapping only
- [Ended peer tab] Q: "Fix the brief 'Can't reach this Mac' sidebar line after a host kill (todo part 2)?" → Selected: "Yes: find the ListSessions/teardown race and keep the last good peer state through a short grace; if the cause can't be found, split it into a new todo" — local UI state
- [Stale pgid] Q: "How to stop signalling a stale process group on dismiss?" → Selected: "Skip the group signal once the created session's wait has returned (mirror killAdopted's exited guard), test with a reaped session; items 2 and 3 get a guard or doc comment only (no caller today)" — small internal guard
- [Socket hardening] Q: "Which Phase 9 audit items (O-1, O-2, O-6)?" → Selected: "All three: chmod the socket 0600 right after listen with a mode test; close every fd in unparsed control messages in fdpass.Recv; read the old daemon pid via fdpass.PeerPID on takeover and refuse a mismatch with HELLO-OK (old daemon keeps serving)" — internal; a refused takeover falls back to the existing DEBT-08 guard
- [Path click] Q: "F-1: what directory applies to a path on row 0 printed before the first cwd report?" → Selected: "Fix in the relay cwd log (internal/relay/cwd.go and Hub.PathClick lookup) so output printed after a cwd report resolves against that report even when it lands on an earlier absolute row (e.g. after clear); test in internal/relay/pathclick_test.go" — internal lookup
- [Path click] Q: "F-2: first Cmd-click on the same cell after leave + text change" → Selected: "Call PathLinksHandle.invalidate() on mouseleave; add a real-xterm-Linkifier case (leave, change row, return to same cell, Cmd-click) to pathLinks.linkifier.test.ts" — one event hook
- [Flakes] Q: "How to make the two flaky tests deterministic?" → Selected: "Wait on events, not wall-clock: renderOnce resolves on term.onRender (with timeout); TestShare_PasteBacklogKeepsViewer's heartbeat limit becomes load-robust (event wait or scaled) without changing product heartbeat behaviour; then a loop run plus one clean full Go (-race) and vitest run" — test-only

### Claude's Discretion
- Notification click behaviour (user said "You decide"): show window + banner chosen, recorded above.

### Deferred Ideas (OUT OF SCOPE)
- A flag to turn the `portty attach` status bar off (mentioned in the attach todo; new capability).
- dompurify-3418 (blocked until 2026-11-28) and v12-release-checks (real-Mac checks) stay in pending.
</user_constraints>

<phase_requirements>
## Phase Requirements

No new requirement IDs (bug fixes and hardening against existing behaviour). Success criteria 1-8 from ROADMAP Phase 15 map to the items below.

| SC | Item | Research Support |
|----|------|------------------|
| 1 | Attach rows-1 | Section "Item A": frames at `cmd_attach.go:394-401` and `internal/attach/attach_unix.go:30-41`; hub model verified |
| 2 | Ended peer tab | Item B1: flag on the Tab, mapping in `peerConnectVerdict` |
| 3 | Stale pgid | Item C: `killSession` has no reaped guard; a `reaped` flag set by `ReapNaturalExit` |
| 4 | Socket hardening | Item D: O-2/O-1/O-6, with a measured `LOCAL_PEERPID` probe |
| 5 | Notification click | Item E: ObjC delegate has no `didReceiveNotificationResponse` |
| 6 | F-1, F-2 | Items F and G, both reproduced and fix-validated with throwaway tests (reverted) |
| 7 | Flakes + clean suites | Item H, plus a third known flake (G-1) the planner must know about |
| 8 | Todos + TESTING.md | Item I |
</phase_requirements>

## Summary

Phase 15 is nine small, mostly independent fixes. I read every code path named in the brief and, for the two path-click bugs, reproduced the failure with a throwaway test and confirmed the proposed fix, then reverted both (the working tree is back to its starting `git status`). The headline findings: (1) the attach fix is exactly the two frame call sites, because the hub already makes a claiming or typing viewer win; the initial frame is a plain report and only becomes the grid on an ownerless, never-sized hub, which is the right size to adopt. (2) F-1 is not a lookup bug: `ESC[3J` in the same write as `ESC[2J` drops emulator-scrollback lines without counting them in `History.Pushed()`, so absolute rows run backwards and new output reuses rows that older cwd records already cover. A one-line fix at the `J` handler keeps rows monotonic. (3) The "Can't reach this Mac" flash is most likely the viewer's 30-second peer-probe cache listing a host whose share server the host stopped when its last share ended; the live meta fetch then fails. There is no `ListSessions` in that path, so the todo's guess is wrong, and the grace must be about 35 s, not a few seconds. (4) `LOCAL_PEERPID` as seen by a dialing client returns the accepting process's pid even when the listener was created by another process (measured), so the O-6 check will not break a second upgrade.

Two risks need planner attention. The `--status-top` bar cannot be protected by rows-1 alone: nothing enables origin mode, so an app that addresses absolute rows still draws on row 1 (see Pitfall 1). And the full-suite gate in SC7 can still trip on the known, unrelated `internal/pty` `TestKillClean_NoOrphans` flake (G-1), in the very package Item C edits.

**Primary recommendation:** Implement the nine items as independent tasks with a test written first for each; reuse the existing test seams (`signalGroup`, `handoffScene`, `pcHub`/`cwdHub`, `emitEventFunc`/`showWindowFunc`); keep the product heartbeat, render timing and grid-owner model untouched.

## Project Constraints (from CLAUDE.md)

- **TESTING.md standing rule** (`CLAUDE.md`): new test files go in Section 2 (manifest + counts note), a Section 4 row per new test file covering a later-milestone requirement (path column is a repo-relative `.go/.ts/.tsx/.sh` path only), a Section 5 `M-NN` item for anything not automatable, and run `bash tests/check-traceability-paths.sh` before committing (baseline today: `OK: 228 traceability paths exist`).
- **Chesterton's Fence / deletion discipline:** read before removing; Phase 15 should delete nothing. Item C deliberately keeps `killAdopted` and the `exited` guard.
- **Silent fallbacks:** "`or {}` converts hard failures into silent corruption". For O-6, a `PeerPID` error must refuse the takeover, never fall back to trusting HELLO-OK.
- **Reliability first** (memory): a hardening change that can fail must fail toward the old daemon keeping its sessions.
- **No live tailnet in research; GUI checks in the Tart VM `portty-test`** (memory): none of the research ran against tailscaled; the manual items go to VM rows.
- **Go:** `go fmt`, `golangci-lint`, context-aware functions. **JS/TS:** ESLint + Prettier, camelCase. **Tests:** 80%+ coverage in critical components.
- **Security:** keep token-gated, tailnet-only links and the CSP untouched (Phase 15 touches neither).
- **GSD enforcement:** no direct edits outside a GSD workflow; the executor works through `/gsd-execute-phase`.
- **Ponytail:** smallest change that fully solves each item; no abstraction below three real uses.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Attach tells the PTY rows-1 | CLI process (`cmd_attach.go`, `internal/attach`) | Relay hub (grid owner) | The CLI knows whether its bar is shown; the hub already decides which viewer's size becomes the PTY grid |
| Ended-peer-tab verdict | Browser/Client (React: `closeReasons.ts`, `App.tsx`, `TerminalPanel.tsx`) | Daemon (`ConnectPeerSession` error code) | The daemon only reports `no_such_session`; what it means depends on what this tab already saw |
| Sidebar refused-vs-unreachable | Go fetch layer (`internal/tailnet` sessions fetch, called from app.go) | Daemon `tailnetCache` (30 s) | Updated per 15-CONTEXT.md Post-research decisions [Sidebar]: classify at the fetch site, no UI grace |
| Skip stale pgid signal | Daemon PTY layer (`internal/pty`) | Engine exit watcher | Only the PTY session knows whether its child was reaped |
| Socket mode, fd closing, pid check | Daemon IPC (`internal/daemon`, `internal/fdpass`) | OS (unix socket, `LOCAL_PEERPID`) | Kernel-attested identity is the trust anchor |
| Notification click | Native macOS shell (`tray_objc_darwin.m`) | Go app (`app_update.go`), React banner | ObjC receives the click, Go shows the window, React un-dismisses the banner |
| Relative-path cwd (F-1) | Relay hub (`internal/relay`) | — | The host owns the absolute-row log; viewers only send text |
| First Cmd-click (F-2) | Browser/Client (`pathLinks.ts`) | xterm Linkifier internals | Client-only event handling |
| Flake fixes | Test code only | — | No product change allowed |

## Standard Stack

No new libraries. Everything below is already in `go.mod` / `frontend/package.json`.

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go toolchain | go1.26.11 (pin; default `go` here is 1.27.4) | Go builds and tests | `[VERIFIED: go.mod]` `toolchain go1.26.11`; `toolchain_guard.go` refuses Go 1.27+ builds of the root package |
| `golang.org/x/sys/unix` | v0.46.0 | cmsg parsing, `LOCAL_PEERPID` | `[VERIFIED: go list -m]` already used by `internal/fdpass` |
| `github.com/coder/websocket` | v1.8.15 | WS client/server; `DialOptions.OnPingReceived` exists | `[VERIFIED: go.mod, module source dial.go:51-65]` |
| `@xterm/xterm` | 6.0.0 | Terminal and Linkifier | `[VERIFIED: frontend/node_modules/@xterm/xterm/package.json]` |
| vitest | 4.1.0 | Frontend tests (`vi.waitFor` available) | `[VERIFIED: frontend/node_modules/vitest/package.json]` |
| jsdom env | via `vite.config.ts` `test.environment: 'jsdom'` | DOM for tests | `[VERIFIED: frontend/vite.config.ts:14-19]` |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Frontend grace for the sidebar flash | Invalidate/shorten the daemon `tailnetCache` | Daemon change is wider (all callers of `probePeers`); the user locked the frontend-grace approach |
| `History.Skip` for F-1 | Re-anchor the cwd log on ED2 | Re-anchoring needs the cursor row at a moment it is not yet final (CUP may follow); keeping rows monotonic is the root-cause fix |
| `chmod` after `net.Listen` | `syscall.Umask` around `Listen` | umask is process-global and racy with other goroutines creating files; parent dir is already 0700 so the chmod window is harmless |

**Installation:** none. `Version verification:` n/a (no packages added).

## Package Legitimacy Audit

No external packages are installed or added by this phase, so the legitimacy gate does not apply.

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
 portty attach (CLI)                     relay hub (host daemon)                     other viewers
 ───────────────────                     ───────────────────────                     ─────────────
 term.GetSize ──► rows
   │ bar shown? (Bar.ReservedRows)
   ▼
 report: MsgResize2(cols, rows-1) ─────► sizeClient(claim=false)
   (onConnect, every (re)connect)          stores sub.Cols/Rows; grid moves only on bootstrap
 SIGWINCH ──► claim: MsgResize2(…,rows-1,0x01) ─► sizeClient(claim=true) ─► setOwner ─► applyGridLocked ─► PTY resize + MsgResize broadcast
 typing ──► MsgInput ──────────────────► NoteTyping ─► owner=CLI, want=sub.Rows (rows-1) ─► applyGridLocked
                                                                           GUI/browser claims or types later ─► they take the grid back (by design)

 Dismiss session ─► Engine.KillSession ─► backend.Kill ─► killSession(s)
                                              reaped? (set by ReapNaturalExit) ─yes─► no group signal
                                                                              └─no──► SIGHUP -pgid, close PTY, wait 2 s, SIGKILL

 New daemon (takeover) ─ dial ─► HELLO ─► HELLO-OK{PID} ─► [NEW] PeerPID(conn) == PID ? ─no─► ABORT (old daemon rolls back, keeps serving)
                                                                                      └yes─► global fds ─► sessions ─► READY ─► COMMIT

 OSC 7 / output ─► emu.Write ─► cwdLog.add(absRow) ;  ESC[3J ─► history.Clear() + [NEW] history.Skip(emu.ScrollbackLen())  (rows stay monotonic)

 macOS notification click ─► PorttyNotificationDelegate didReceiveNotificationResponse ─► onNotificationClick(id) (cgo export)
        ─► go app.handleNotificationClick(id) ─► showWindow() + emit "update:show" ─► React: setDismissedUpdate(null) ─► UpdateBanner (available)
```

### Recommended Project Structure
No new directories. Touch points per item (all existing files):
```
cmd_attach.go, cmd_attach_resize.go, internal/attach/{attach.go,attach_unix.go}, internal/statusbar/bar.go      # A
frontend/src/lib/closeReasons.ts, components/{TabBar.tsx,TerminalPanel.tsx,WebShareSessionView.tsx}, App.tsx   # B
frontend/src/lib/sidebarPeers.ts (+ App.tsx poll)                                                               # B2
internal/pty/{cleanup.go,session.go,reap_other.go}                                                              # C
internal/daemon/{ipc.go,handoff.go,engine.go(doc)}, internal/fdpass/fdpass.go, internal/relay/manager.go(doc)  # D, C-doc
tray_objc_darwin.m, tray.go or notification_darwin.go, app_update.go, frontend/src/App.tsx                      # E
internal/relay/{history.go,hub.go}                                                                              # F
frontend/src/lib/pathLinks.ts                                                                                   # G
frontend/src/lib/__tests__/pathLinks.linkifier.test.ts, internal/webserver/{input_queue_test.go,capability_helpers_test.go}   # H
TESTING.md, .planning/todos/{pending→completed}                                                                 # I
```
Suggested ordering for the planner: A, C, D, F, H-Go are disjoint Go work and can run in parallel. B, E, G, H-vitest all touch the frontend; `App.tsx` is touched by B and E and `pathLinks.linkifier.test.ts` by G and H, so serialize those pairs. I last.

### Item A: attach sends rows-1 `[VERIFIED: files read this session]`

Call sites (all read):
- `cmd_attach.go:394-401` `onConnect`: `term.GetSize(int(os.Stdin.Fd()))` then `conn.Write(... attach.MakeClientResizeFrame(uint16(cols), uint16(rows)))` then `watchResize(ctx, conn)`. Runs on every (re)connection (`internal/attach/loop.go:201-202`).
- `internal/attach/attach_unix.go:19-26` `WatchResize` builds the size lookup `term.GetSize(int(os.Stdin.Fd()))`; `:30-41` `watchResize` sends `MakeClientClaimFrame(uint16(cols), uint16(rows))` at `:38`.
- `internal/attach/attach.go:160-173`: `MakeClientResizeFrame` (report) and `MakeClientClaimFrame` (report plus `relay.ResizeClaim`).
- The bar: `cmd_attach.go:367-381` creates it only when stdout is a TTY, then `bar.Start()`; `internal/statusbar/bar.go:103-117` `Start()` returns without drawing when `getSize()` fails, and reserves one row for both positions (`setScrollRegion, 2, rows` for Top at `:114`, `setScrollRegion, 1, rows-1` for Bottom at `:116`). So "bar shown" is not `bar != nil`; it is "`Start` sized successfully".

Hub behaviour (read, `internal/relay/hub.go`): `sizeClient` (`:618`) stores `sub.Cols, sub.Rows`; for a plain report it returns after storing, except the bootstrap (`:633`: `if sub.Origin != "local" || h.owner != nil || (h.ptyCols > 0 && h.ptyRows > 0)`), so a report moves the grid only on an ownerless hub whose grid was never known and only for a local viewer. `NoteTyping` (`:580`) makes the typer the owner and `setOwnerLocked` (`:604`) sets `wantCols, wantRows = sub.Cols, sub.Rows`, i.e. the stored report. So storing rows-1 in the initial report is enough for typing to apply rows-1, and the SIGWINCH claim applies it at once. The remote-attach path (`internal/webserver/server.go:1072-1090`) and the loopback relay (`internal/relay/server.go:198-207`) call the same `ClaimGrid`/`ResizeClient`.

**Answer to "does the initial frame ever become the grid?":** only via the bootstrap, when the CLI is the first local viewer on a never-sized hub. rows-1 is the right size there too. The existing `shortcut:` comment on `MakeClientResizeFrame` (a CLI can show a larger GUI grid until it types or resizes) stays an accepted limit; do not turn the initial frame into a claim.

Recommended change (two call sites plus one tiny pure helper):
```go
// internal/attach/attach.go  (Source: derived from the verified call sites above)
// BarRows is the PTY height to report: the terminal's rows minus the rows the
// status bar reserves. A terminal of reserve rows or fewer is reported as is,
// so the uint16 frame can never wrap.
func BarRows(rows, reserve int) int {
	if reserve > 0 && rows > reserve {
		return rows - reserve
	}
	return rows
}
```
- `statusbar.Bar` gets `func (b *Bar) ReservedRows() int` returning 1 under `b.mu` once `Start` stored a size (`b.rows > 0`), else 0. A nil `*Bar` must return 0 (call sites pass `bar` that is nil on a non-TTY).
- `onConnect` uses `attach.BarRows(rows, bar.ReservedRows())`; `WatchResize(ctx, conn, reserve func() int)` (or an int) applies the same in its size closure, so the existing `watchResize(ctx, conn, winch, size)` test seam keeps its signature. Update `cmd_attach_resize.go` and its one caller (`cmd_attach.go:400`).
- Tests (pin rows-1): (1) table test for `BarRows` including rows 0, 1, 2, reserve 0; (2) extend `TestWatchResize_SendsClaim` (`internal/attach/attach_test.go:60-117`) or add a sibling that feeds `size` 90x30 with reserve 1 and expects `MakeClientClaimFrame(90, 29)`, with reserve 0 expecting 30; (3) a root-package test (`cmd_attach_test.go`) for the initial frame built by `onConnect`'s logic (extract a small `initialResizeFrame(cols, rows int, reserve int) []byte` if needed); (4) `Bar.ReservedRows` test with `FallbackCols/FallbackRows` (the existing `Options` fields at `bar.go:36-37`) and one with neither, expecting 0; (5) a hub-level pin in `internal/relay/hub_grid_owner_test.go` style: owner GUI 120x40, CLI reports 80x23 (grid stays 120x40), CLI claims 80x23 (grid 80x23), GUI reclaims (grid 120x40). The generic model is already tested (`TestHub_GridTypingClaims`, `TestRelayPump_ClaimTakesGrid`, `:401,:298`); the new test only fixes the CLI-shaped sizes.

### Item B1: ended peer tab keeps "session ended" `[VERIFIED: files read]`

Mechanism: a desktop peer tab is unmounted when it is not the active tab (`App.tsx` renders only `activeWebTab`'s `WebShareSessionView`, keyed `${sessionTabId}:${wsCapToken}`, around `:1320-1355`). On return the panel remounts, its `sessionEnded` state (`TerminalPanel.tsx:374`, initial `{ ended: false }`) is lost, the `RelayClient` dials once, fails, and `redial()` (`relayClient.ts:608-632`) calls `beforeReconnect`. That is `reconnectPeerTab` (`App.tsx:1022-1031`), which maps the daemon's `errorCode` through `peerConnectVerdict` (`closeReasons.ts:28-31`): `no_such_session` is `{ kind: 'stop', reason: 'share-stopped' }`, so `stop(1000, CLOSE_REASON_SHARE_STOPPED)` shows "`<host> stopped sharing this session.`". `ReconnectVerdict` already supports `reason: 'ended'` (`relayClient.ts:621` maps it to `CLOSE_REASON_SESSION_ENDED`), and `shareInfoVerdict` already returns it for 404 (`closeReasons.ts:44`).

Recommended change (frontend verdict mapping, per the locked decision):
1. `Tab` (`components/TabBar.tsx:37` next to `shareStopped?: boolean`): add `sessionEnded?: boolean` ("this peer tab saw the session-ended close").
2. `TerminalPanel.onClose` guest branch (`TerminalPanel.tsx:663-665`): when `reason === CLOSE_REASON_SESSION_ENDED`, call a new `onSessionEnded?.()` prop (mirror `onShareStopped`, passed through `WebShareSessionView.tsx:63,113,258`). `App.tsx` sets the flag with the same host-plus-session match `dropPeerSession` uses (`:1038-1046`).
3. `peerConnectVerdict(errorCode, sawEnded = false)`: `no_such_session` with `sawEnded` returns `{ kind: 'stop', reason: 'ended' }`; every other case unchanged. `reconnectPeerTab` takes the flag from the tab at call time (`beforeReconnect` is read through a ref that is refreshed every render, `TerminalPanel.tsx:205-206`, so the closure sees the latest tab).
4. Tests: `lib/__tests__/closeReasons.test.ts:23-28` (add `no_such_session` with and without `sawEnded`; `not_sharing` and `forbidden` stay share-stopped even with `sawEnded`); `TerminalPanel.guestClose.test.tsx` (the ended close calls `onSessionEnded`, the share-stopped close does not); an App-level test next to `App.open-remote.test.tsx` that ends a peer tab, switches away and back, answers `no_such_session` and expects the "Session ended" copy (`SessionEndedBanner.tsx:129`), not the stopped-sharing copy (`:128`). Pin the exact strings from the UI-SPEC.

### Item B2: sidebar "Can't reach this Mac" after a host kill `[VERIFIED code path; cause MEDIUM, not reproduced]`

Findings, all read this session:
- The todo's guess ("ListSessions race during teardown") is wrong for this path: `handleSessionsMeta` (`internal/webserver/server.go:797-820`) iterates `ws.webEnabledSessions()` and never calls the engine's `ListSessions` for the list.
- `unshareLocked` (`internal/daemon/share.go:345-360`) stops the share server when the last share ends: `if ws != nil && len(a.shares) == 0 { _ = ws.Stop() ... a.webServer = nil }`. A kill reaches it ("Shared by share-off, session exit and kill"). After that the host has no listener on 7444.
- The viewing Mac's daemon discovers Portty peers with `a.tailnetCache.getOrRefresh(ctx, tailnet.DiscoverAndProbe)` (`internal/daemon/api.go:115-117`) and the cache is `const cacheTTL = 30 * time.Second` (`internal/daemon/tailnet_cache.go:11`). The GUI poll then calls `tailnet.FetchAllPeerSessionsMeta` live (`app.go:1408-1445`), which yields `Reachable: status == MetaOK` (`internal/tailnet/sessions.go:222`).
- Net effect: for up to 30 s after the host's last share stops, the cache still lists the host, the live meta fetch is refused, `Reachable=false`, and `Sidebar.tsx:216` renders `PEER_UNREACHABLE`. Then the cache expires, the host is no longer probed as Portty and its group disappears. This matches the VM observation (two 5 s samples, then gone, `10-VM-CHECKS.md` RF-03 re-run note). It also means a host with 0 shared sessions is not "reachable with 0 sessions" at all in steady state.
- A second, smaller contributor: `peerView` (`sidebarPeers.ts:48-51`) synthesizes `{ hostname: reconnectingHost, reachable: false, sessions: [] }` while that host's tab is reconnecting.

Recommendation (matches the locked "short grace", but size it to the cause): in the poll handler (`App.tsx:798-806`) route results through a small pure function in `sidebarPeers.ts`, e.g. `applyPeerGrace(prev, next, now, graceMs)`: when a peer that was `reachable: true` in the previous result comes back `reachable: false`, keep it as `{ ...prev, reachable: true, sessions: [] }` until `now - lastReachableAt >= graceMs`; a genuinely unreachable host shows the line after the grace. Use `graceMs` of about 35_000 (cache TTL 30 s plus one 5 s poll). Reset on any reachable answer. Sessions are emptied rather than kept so a killed session's row does not linger. Test it as a pure function (fake `now`) plus one `App.remote-poll.test.tsx`-style case with fake timers. If the executor wants evidence before coding, the host-side half is checkable without a tailnet: unit test that `unshareLocked` on the last share stops the server (already implied by existing share tests); the cache half needs only the constant. Record the cause and the 35 s choice in the SUMMARY; if the reviewer rejects the grace length, the fallback in the CONTEXT decision is to split a todo.

### Item C: stale pgid on dismiss `[VERIFIED: files read]`

- `internal/pty/cleanup.go:20-37`: `killSession` branches to `killAdopted` when `s.exited != nil`; the created path then does `_ = syscall.Kill(-pgid, syscall.SIGHUP)` at `:36` and `syscall.Kill(-pgid, syscall.SIGKILL)` at `:68`, with no guard. `killAdopted` (`:97-125`) signals only `if alive()` and uses the test seam `var signalGroup = syscall.Kill` (`:89`).
- "Wait returned" for a created session is `ReapNaturalExit` (`internal/pty/reap_other.go:19-41`), which calls `cmd.Wait()` then `SetExitCode`. It is called by `watchSessionExit` (`internal/daemon/engine.go:285-298`) after `hub.Done`, only if `!sess.IsKilled()`.
- Do not infer "reaped" from `ExitCode() != -1`: `Session.ExitCode` stores `ProcessState.ExitCode()`, which is -1 for a signal-terminated child, and `cmd.ProcessState` is written without our mutex (`session.go` comment above `ExitCode`).
- Fix: add `reaped bool` (under `s.mu`) or `atomic.Bool` to `Session`, set it right after `cmd.Wait()` returns in `ReapNaturalExit` (created branch) and in `killSession`'s own wait goroutine. In `killSession`, skip both group signals when it is set, and route the two created-path signals through `signalGroup` so the existing `recordSignals` helper (`adopt_test.go:231-250`) sees them.
- Test (mirror `TestKillAdopted`, `adopt_test.go:272-310`): `recordSignals(t)`; `b := NewNativePTYBackend()`; `sess := startSh(t, b, "read x; exit 0")` (`native_test.go:342`); `readAll(sess)`, write `"\n"`, wait for `eof`; `sess.ReapNaturalExit()`; `before := len(sent())`; `b.Kill(sess.ID)`; assert no new signals. Control: a running `sleep 30` session still gets `SIGHUP` (and the existing `TestKillClean*` tests stay green).
- Residual window (document, do not fix): a dismiss that lands between the child exiting and `ReapNaturalExit` finishing signals a zombie group, which holds its pid, so no reuse can happen yet.
- Todo items 2 and 3, doc comments only (no caller today, per the decision): `HubManager.Shutdown` (`internal/relay/manager.go:73`) / `watchSessionExit` note that Shutdown leaves exit watchers blocked in `cmd.Wait` on live shells, so a later `KillSession` races them (harmless in production, the process exits); `closeSessionFiles` (`internal/daemon/engine.go:376-380`) comment that `HandoffFiles()` (dup + slave) would miss go-pty's original master on a created session and is only called for adopted sessions (`engine.go:363`, `handoff.go:1397`).

### Item D: socket hardening `[VERIFIED: files read; PEERPID semantics measured]`

**O-2 (mode 0600).** `internal/daemon/ipc.go:10-12` is `return net.Listen("unix", path)`; the only caller is `internal/daemon/api.go:319`. Add `os.Chmod(path, 0o600)` right after a successful listen; if chmod fails, close the listener and return the error (do not serve with a wider mode silently). Test in `socket_test.go` style (it uses `shortSocketPath`, `/tmp/dtestN_name`, because of the 103-char `sun_path` limit): listen, `os.Stat` mode `& 0o777 == 0o600`. Note a takeover receives the old daemon's listener fd, so the first upgrade from an older (0755) daemon leaves its mode; see Open Questions.

**O-1 (unparsed cmsgs).** `internal/fdpass/fdpass.go:76-88`. Read from x/sys v0.46.0 `unix/sockcmsg_unix.go:39-52`: `ParseSocketControlMessage` returns `nil, err` on the first malformed header, discarding every message it had already parsed, so those fds are never closed. In `Recv` the `ParseUnixRights` error branch (`perr = e; break`) is unreachable, because the level and type were just checked and `ParseUnixRights` only errors on those two. The real gap is the first one. Fix: walk `oob[:oobn]` with `unix.ParseOneSocketControlMessage` (returns header, data, remainder), collecting SCM_RIGHTS ints as it goes, so an error mid-buffer still leaves every earlier fd in `fds` to be closed. Test as a pure function: factor the walk into `rightsFromOOB(oob []byte) (fds []int, err error)` and feed it `unix.UnixRights(fdA, fdB)` followed by garbage bytes, with `fdA/fdB` real dups of `os.Pipe` ends; assert an error and that both fds are closed afterwards (`fcntl(F_GETFD)` returns EBADF), as the existing `openFds` counter in `fdpass_test.go:92-124` does. The kernel will not let a test send a malformed cmsg through a real socket, hence the pure-function seam.

**O-6 (pid from the kernel).** Takeover side (`internal/daemon/handoff.go`): `writeFrame(t.c, frameHello, ...)` at `:873`, then `t.oldPID = ok.PID` at `:890`. Add, immediately after reading HELLO-OK and before `preStep("after-hello")`:
```go
// Source: derived from handoff.go:873-890 and fdpass.PeerPID (internal/fdpass/fdpass.go)
kernelPID, err := takeoverPeerPID(t.c) // seam, = fdpass.PeerPID
if err != nil || kernelPID != ok.PID {
	return t.fail(takeoverRefused, fmt.Sprintf("HELLO-OK pid %d does not match the socket peer (%d, %v)", ok.PID, kernelPID, err))
}
t.oldPID = kernelPID
```
`t.fail` (`:1200-1206`) sends ABORT and gives everything back; the old daemon rolls back and keeps serving. A `PeerPID` error refuses (fail toward the old daemon). Use a new seam variable next to `peerPID` (`handoff.go:130`), not `peerPID` itself: `peerPID` is the old side's lookup of the takeover's pid and tests replace it (`handoff_test.go:195-250` `installSeams`).
In tests both daemons share one process (comment above the seams), so the real `fdpass.PeerPID` already equals `os.Getpid()` and existing handoff tests keep passing unchanged; a mismatch test replaces the new seam with a function returning another pid and asserts outcome `takeoverRefused` plus that the old API still lists its sessions (use `handoffScene`, `failAt` and the `oldExits/newExits` channels).

**Measured, so it is safe:** I ran a probe (scratchpad, not in the repo): process P creates a unix listener and passes the fd to child C, which accepts; a client then dials. `LOCAL_PEERPID` on the client's socket was C's pid (the accepting process), not P's. Output: `parent(listener creator)=96815 child(accepter)=96817 client sees LOCAL_PEERPID=96817`. So after a daemon A hands its listener to B, a client of B sees B's pid, which equals B's `os.Getpid()` in HELLO-OK: the check will not refuse every second upgrade. `[VERIFIED: probe run this session on macOS]`. Read it after the 101 upgrade completes (accept has happened by then). The existing `TestPeerPID` (`fdpass_test.go:139-182`) only covers the accepted side's view of a dialing client, so add a test for the dialing side's view if cheap (needs a helper process, `nc` is used there); the probe result is the evidence otherwise.

### Item E: update-notification click `[VERIFIED code; delegate behaviour ASSUMED]`

State (read): `tray_objc_darwin.m:84-95` `PorttyNotificationDelegate` implements only `willPresentNotification:`. It is installed lazily by `registerNotificationDelegate()` (`:104-117`), whose only call is inside `sendNotification` (`:296`). Go posts the notice from `notifyUpdateOnce` (`app_update.go:128-134`): `a.sendNotificationFunc("portty.update-available", "Portty", "Version "+version+" is available.")`. Show-window is `a.showWindow()` (`app_update.go:229-238`: `WindowUnminimise`, `WindowShow`, `setDockVisible(true)`, guarded by `a.ctx.Value("frontend") != nil` and by the `showWindowFunc` seam). The tray-update path already calls `a.offerUpdate()` on a goroutine (`tray.go:109-114`, "the alert blocks; never on the main thread"). The banner shows when `showUpdateBanner` (`App.tsx:880-883`): state in `available|downloading|verifying|revealed` and `updateStatus.latest !== dismissedUpdate`; `dismissedUpdate` is only cleared when a download starts (`App.tsx:868-870`).

Implementation (ObjC + Go + one React line):
1. `tray_objc_darwin.m`: add `extern void onNotificationClick(char *identifier);` beside the other forward declarations (`:13-16`), and in `PorttyNotificationDelegate` implement `userNotificationCenter:didReceiveNotificationResponse:withCompletionHandler:`: if `response.actionIdentifier` is the default action (`UNNotificationDefaultActionIdentifier`), call `onNotificationClick((char *)[response.notification.request.identifier UTF8String])` inside an `@autoreleasepool`, and **always** call `completionHandler()`. `[ASSUMED]` these Apple API names and the background-queue delivery; the VM check confirms them.
2. Register the delegate at launch, not only at first send: call `registerNotificationDelegate()` from the tray init path (it is idempotent and bundle-guarded). Otherwise a click on a notification left in Notification Center from a previous run reaches an app with no delegate yet.
3. Go: `//export onNotificationClick` in `tray.go` next to `onTrayUpdate` (it already owns `trayCallbackApp`). Capture `app := trayCallbackApp` before the goroutine, then `go app.handleNotificationClick(C.GoString(id))`. Keep the C string conversion on the callback thread.
4. `app_update.go`: unexported (Wails must not bind it) `handleNotificationClick(id string)`: for `"portty.update-available"` call `a.showWindow()` and `a.emitEvent("update:show", nil)`; ignore other ids. It must not call `DownloadUpdate` (locked decision).
5. `frontend/src/App.tsx`: `EventsOn('update:show', () => setDismissedUpdate(null))` next to the `update:status` subscription (`:865-878`) so a banner the user dismissed earlier comes back. No new copy or layout (UI-SPEC row 3). The `available` banner then shows `Portty {latest} is available.` with `Download`.
6. Tests: Go (`app_update_test.go` harness already sets `showWindowFunc` at `:169` and has `emitEventFunc`): `handleNotificationClick("portty.update-available")` shows the window once and emits `update:show` once, and starts no download (assert `DownloadUpdate`'s service is untouched); an unknown id does nothing. Vitest (`App.update.test.tsx`): dismiss the banner, emit `update:show`, banner visible in `available` state, `DownloadUpdate` not called. The ObjC delegate cannot run in `go test` (unbundled process; `TestHasAppBundleID_FalseWhenUnbundled` exists for that reason), so add manual item **M-152** (next free number; M-151 is the highest today) under a new category after BE, or under Category U/AU: with a notarized build in the Tart VM, hide the window (start minimized or Quit GUI Only), let the update notice post, click it, expect Portty forward with the "available" banner and no download.

### Item F: F-1 relative path on row 0 `[VERIFIED by reproduction and fix prototype, both reverted]`

Reproduction (throwaway test using `cwdHub`, `osc7`, `feedHubs`, `cwdOf` from `cwd_test.go:18-58`): feed `osc7("/start")+"% cd proj\r\n"+osc7("/proj")+"% clear; ls\r\n"+"\x1b[H\x1b[2J\x1b[3J"+"docs-x\r\ndocs-y\r\n"+osc7("/proj")+"% "`. Result before the fix: `pushed=0 scrollbackLen=0 recs=[{0 /start} {1 /proj}]`, `docs-x row=0 -> "/start"`, `docs-y row=1 -> "/proj"`. With `ESC[H ESC[2J` only (no `3J`) the screen lines are pushed and rows keep moving (`docs-x row=2 -> "/proj"`), so the bug needs the `3J` in the same write.

Root cause: `ED 2` pushes the visible lines into the emulator's scrollback; `ED 3` then drops them. The `J` handler (`internal/relay/hub.go:1297-1302`) already calls `h.history.Clear()` on `n == 3`, but `History.Clear` (`history.go:71-73`) leaves `pushed` alone and the lines still in `emu`'s scrollback were never drained into history, so they are never counted. `row = history.Pushed() + emu.ScrollbackLen() + cursorY` (`hub.go` OSC 7 handler; `cwdRowOfLocked`, `cwd_test.go:31-45`) goes back to 0. The rows the output now occupies (0 and 1) were already covered by older records.

Fix (prototyped, then reverted): a `History.Skip(n int)` that adds `n` to `pushed`, and in the `J` handler after the clear `h.history.Skip(emu.ScrollbackLen())`, because vt's own handler runs after ours and will drop exactly those lines. With it, `docs-x -> "/proj"`, `docs-y -> "/proj"`, and `TestCwd*`, `TestPathClick*`, `TestHistory*`, `TestHub_Handoff*` all passed (`ok ... 5.036s`). `FirstRow() = pushed - Len()` stays consistent because `Clear` empties the lines first. `pushed` is only a number in the handoff state, so the format is compatible.
Tests: (1) `internal/relay/cwd_test.go`: the repro above through `wantCwd(t, h, "docs-x", "/proj")` (this is the cheapest place; add the click-level version in `pathclick_test.go` using `pcHub`/`pcClick`/`pcPreviewed` with real temp dirs as `TestPathClick_Relative` does at `:384-433`, asserting the click on row 0 previews the file under `/proj`, not under the start dir); (2) a variant with `ED3` alone on a screen that has undrained scrollback; (3) keep `TestCwd_SurvivesResizeAndClear` (`cwd_test.go:305`) green: it asserts rows stay valid across `ESC[3J`.
Why not a lookup change: `Hub.PathClick` (`pathclick.go:57-117`) finds the row by text (`findPrinted`) then asks `cwdLog.at(row)` (`cwd.go` `at`). The log is correct for monotonic rows; the defect is the row numbers, so fixing the lookup would paper over it and break other cases.

### Item G: F-2 first Cmd-click on the same cell `[VERIFIED by reproduction and fix prototype, both reverted]`

Cause confirmed in xterm 6.0.0's `src/browser/Linkifier.ts`: `mouseleave` only does `_isMouseOut = true; _clearCurrentLink()` (`:51-54`); `_handleMouseMove` ignores a move to the same cell as `_lastBufferCell` (`:83-86`); `_handleMouseUp` returns when there is no `_currentLink` (`:221-224`). So after leave plus a text change under the cell, returning to the same cell asks for no link and the click goes nowhere.
Reproduction (temporary copy of the linkifier test): hover (3,1), click, dispatch `mouseleave` on `.xterm-screen`, `write('\x1b[1;1H\x1b[2Kdocs/other.png')`, `renderOnce()`, hover (3,1), click: `pathClicks()` was `['docs/diagram.png']`, expected `['docs/diagram.png','docs/other.png']` (FAIL before the fix).
Fix: `frontend/src/lib/pathLinks.ts:113` `const onLeave = () => { pointerInside = false }` becomes `() => { pointerInside = false; invalidate() }`. `invalidate()` (`:134-150`) resets `_lastBufferCell` and `_activeLine` in a microtask and, because `pointerInside` is false, does not re-hover. With the change the probe passed together with the other nine linkifier tests and `pathLinks.test.ts` (`43 passed`). ESLint: `invalidate` is declared after `onLeave` (`:134`); it is only called at event time so it is safe, but move `onLeave` below `invalidate` or hoist to avoid a `no-use-before-define` warning.
Test: add the probe above to `pathLinks.linkifier.test.ts` as "after leaving and returning to the same cell with changed text, the first Cmd-click sends the new path". It must fail on the unfixed code first.

### Item H: the two flakes `[VERIFIED files read]`

**H1 vitest `pathLinks.linkifier` "after a reconnect, another cell of the same row offers the link".** `pathLinks.linkifier.test.ts:93-98`:
```ts
const renderOnce = async () => {
  const before = renders
  term.refresh(0, term.rows - 1)
  await sleep(100)
  expect(renders, 'term.onRender fired').toBeGreaterThan(before)
}
```
Replace the sleep with `await vi.waitFor(() => expect(renders, 'term.onRender fired').toBeGreaterThan(before), { timeout: 5000, interval: 10 })` (vitest 4.1.0; `vi` is already imported at `:23`). The same wall-clock pattern guards other assertions in this file and will flake the same way under load: `sleep(100)` at `:143,:169,:234`, `sleep(20)` at `:165,:252` (the redial after `portty:wake`, asserted as `WS.instances.length === 2`), and `resizeTo`'s `sleep(150)` at `:202` followed by `expect(term.cols)` and `expect(renders)`. Convert those assertions to `vi.waitFor` on the condition they check, in the same task; keep `sleep` only where no condition exists (a "nothing happened" window such as the capability-false test).
**H2 Go `TestShare_PasteBacklogKeepsViewer`** (`internal/webserver/input_queue_test.go:94-155`). It sets `heartbeatEvery = 50 ms`, `heartbeatTimeout = 500 ms` (`joinPasteViewer`, `:32-45`; `const timeout = 500 * time.Millisecond` at `:97`) and holds the queued paste for `3 * timeout` of wall clock. The failure line `viewer dropped while the paste is queued (heartbeat timeout)` is one ping not answered within 500 ms because the in-process test client's `conn.Read` goroutine (which answers pings) was starved under `-race` while `internal/relay` ran for about 281 s alongside. The product heartbeat (`relay.Heartbeat`, `internal/relay/heartbeat.go`, ping-and-wait with a context timeout) must not change.
Load-robust shape without touching product code:
- Make the timeout a ceiling that load cannot reach: `const timeout = 5 * time.Second` (it is only the per-ping limit; `heartbeatEvery` stays 50 ms so pings still flow every 50 ms).
- Replace the `3 * timeout` wall-clock hold with an event wait: count pings the client has received and hold until N (for example 8) arrive after `WaitInputIdle` reports the whole paste queued. coder/websocket v1.8.15 supports `DialOptions.OnPingReceived func(ctx, payload) bool` (`[VERIFIED: module source dial.go:51-65]`); add an optional variant of `dialWebServerWS` (`capability_helpers_test.go:213-229`) or a package-level helper that sets it. `Heartbeat` is synchronous (`c.Ping` blocks for the pong), so N pings seen implies N-1 completed round trips while the backlog was queued, which is exactly what the test must prove. Keep the `alive()` check inside the wait loop. A stuck read pump (the bug the test guards) now fails after the 5 s ceiling instead of 0.5 s, still well inside the test's `WaitFor` budgets.
- Do not edit `TestShare_DeadViewerReapedBacklogDelivered` (it uses a 100 ms timeout on purpose, to be reaped).
- Evidence: run `go test -race -count=30 -run TestShare_PasteBacklogKeepsViewer ./internal/webserver` while `go test -race -count=1 ./internal/relay/...` runs in another shell (the failing condition), before and after.
**Third, known flake the planner must plan for (G-1):** `internal/pty` `TestKillClean_NoOrphans` failed in a full run in Phases 12, 13 and 14 (`STATE.md:301`, `14-12-SUMMARY.md:57`, `v1.2-MILESTONE-AUDIT.md:37`: "accepted by the user as unrelated"). It is a fixed-sleep test (`native_test.go:251-290`: `time.Sleep(200ms)` before `Kill`, `time.Sleep(300ms)` after, then `killSignal0(-pgid)` must fail). It lives in the package Item C edits and can break SC7's "full suite passes once, clean". Precedent: re-run it alone with `-count=5` and record. Recommended, cheap, and inside "make flakes deterministic": replace its two sleeps with `testutil.WaitFor` on the child being up and on the group being gone. That is not in CONTEXT's decisions, so flag it to the user (Open Question 1) rather than assume.

### Item I: housekeeping

- Move the nine fixed todos from `.planning/todos/pending/` to `.planning/todos/completed/` (the directory exists; `done/` also exists but CONTEXT names `completed/`): `2026-11-21-attach-statusbar-overwritten.md`, `2026-11-21-ended-peer-tab-banner.md`, `2026-11-20-killsession-stale-pgid.md`, `2026-11-20-daemon-socket-hardening.md`, `2026-11-18-update-notification-click.md`, `2026-11-22-row0-relative-path-start-dir.md`, `2026-11-22-first-cmd-click-same-cell-after-leave.md`, `2026-11-22-linkifier-reconnect-flake.md`, `2026-11-22-paste-backlog-flake.md`. Leave `2026-11-21-dompurify-3418.md` and `2026-11-21-v12-release-checks.md`. If B2's cause analysis changes the plan, put the outcome in the moved todo; if the sidebar fix is rejected, split a new pending todo per the locked decision.
- TESTING.md (all edits per the standing rule): Section 2 table counts (today Go 174 and vitest 122 `*_test.go`/`*.test.ts(x)` files, total 307; recount with `git ls-files` after the phase) and a new `> Note: Portty Phase 15 ...` line in the existing style listing new and edited files; Section 4 rows only for **new** test files (the traceability path column accepts only a repo-relative `.go/.ts/.tsx/.sh` path, test names go in Notes; reuse existing requirement IDs such as INPT-01, INPT-06, PATH-01, PATH-02, UPGR-01, UPD-01 since Phase 15 adds none); Section 5 new items from **M-152**: notification click (needs a notarized build in the VM), and an attach check that a real full-screen app (vim, then Claude Code) stays off the bar row for the bottom bar in a real terminal, plus `--status-top` recorded as an accepted limit (Pitfall 1). Run `bash tests/check-traceability-paths.sh` (baseline OK, 228 paths).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Wait for an async render, redial or ping in a test | `sleep(N)` plus an assertion | `vi.waitFor` (vitest) / `testutil.WaitFor` (Go, `internal/testutil/wait.go:16`) | Fixed sleeps are the cause of both phase flakes |
| Detect who is on the other end of a unix socket | Trusting a pid written in a protocol frame | `fdpass.PeerPID` (`LOCAL_PEERPID`) | The kernel attests it; the frame is peer-controlled |
| Walk control messages | Manual cmsg header arithmetic | `unix.ParseOneSocketControlMessage` + `unix.ParseUnixRights` | Alignment and length checks are easy to get wrong |
| Reset xterm Linkifier state | A second mousemove hack | The existing `PathLinksHandle.invalidate()` | Already guarded, pinned by the "xterm 6 still has the Linkifier fields" test |
| Signal-seam for tests | A new mock | `signalGroup` + `recordSignals` (`adopt_test.go:231-250`) | Already used by `TestKillAdopted` |
| Notification routing | A new event bus | `emitEvent` + `EventsOn` (`update:status` pattern) | The banner already subscribes the same way |

**Key insight:** every fix here has an existing seam or helper; adding new abstractions would violate the "3 real examples" rule in CLAUDE.md.

## Common Pitfalls

### Pitfall 1: `--status-top` is not protected by rows-1 alone
**What goes wrong:** with the bar on row 1 (`bar.go:114` `setScrollRegion, 2, rows`), an app told rows-1 that addresses absolute rows (vim's `CUP 1;1`) draws on screen row 1, over the bar; it merely leaves the last row empty.
**Why it happens:** the bar sets a scroll region but never enables origin mode; a grep of `internal/statusbar` and `cmd_attach.go` for `?6h`/DECOM finds nothing, and `bar.go` states "Exactly 8 [escape constants], no others per UI-SPEC". The claim that absolute CUP is not offset without DECOM is standard VT behaviour `[ASSUMED]`.
**How to avoid:** send rows-1 for both positions as decided (it is harmless for Top), add a comment at the call site, and record Top as a limit: fully fixing it needs DECOM (a ninth escape and a VM check), which is a design change outside this phase. Bottom is fully covered because the app's rows 1..rows-1 map onto the region above the bar.
**Warning signs:** the VM check with vim under `--status-top` shows the bar row repainting once a second.

### Pitfall 2: `uint16(rows-1)` wraps on a zero-row terminal
**What goes wrong:** `term.GetSize` can return rows 0 in a pty with no size; `uint16(-1)` is 65535, which the hub clamps to `MaxGridRows = 200`.
**How to avoid:** `BarRows` only subtracts when `rows > reserve`; test rows 0, 1, 2.

### Pitfall 3: "bar shown" is not `bar != nil`
`Bar.Start` silently does nothing when `getSize` fails (`bar.go:103-108`). Take the reserve from the bar after `Start`, not from the existence of the object, and read it per connection (`onConnect` runs after `Start`).

### Pitfall 4: the bar re-issues its scroll region once a second, not on SIGWINCH
`Bar.draw` compares sizes on the 1 s ticker (`bar.go:145-157`), while the claim reaches the PTY at once. For up to a second after a resize the local region is stale. Pre-existing; do not try to fix it here.

### Pitfall 5: a second `Wait` or a racing watcher in `killSession`
`watchSessionExit` calls `ReapNaturalExit` (a `cmd.Wait`) and `killSession`'s goroutine calls `cmd.Wait` again; go-pty's `Wait` is not safe for concurrent callers (the engine comment says so). Set `reaped` after each `Wait` returns and do not add a third `Wait`.

### Pitfall 6: O-6 must not reuse the `peerPID` seam
`peerPID` is the old side's lookup of the takeover (`handoff.go:473`); tests replace it. Reusing it on the takeover side would make the existing handoff tests fail or silently pass. Use a separate seam.

### Pitfall 7: `Recv` fd-closing can double-close
When refactoring `Recv` onto `ParseOneSocketControlMessage`, each fd must be closed exactly once on every error path (the existing `TestRecv_TruncatedClosesFds` and `TestRecv_CountMismatch` count open fds before and after; keep them green).

### Pitfall 8: the update banner can be dismissed
Showing the window is not enough if the user dismissed the banner earlier (`dismissedUpdate === latest`, `App.tsx:880-883`). Without the `update:show` un-dismiss the click would show a window with no banner.

### Pitfall 9: notification delegate lifetime
`registerNotificationDelegate` runs only on first send today. Register it at launch too, or a click on an old notification after a relaunch is dropped. Always call the `completionHandler`.

### Pitfall 10: grace length vs cache TTL
A grace shorter than 30 s does not cover the stale-cache window (B2). Do not size it from the 5 s poll interval.

### Pitfall 11: changing `WatchResize`'s signature
Only `cmd_attach_resize.go` and `cmd_attach.go:400` call it (`internal/attach/attach_unix.go` carries the `!windows` tag). Keep `watchResize(ctx, conn, winch, size)` unchanged so `TestWatchResize_SendsClaim` still drives it.

## Code Examples

### The rows-1 claim test shape (Item A)
```go
// Source: derived from internal/attach/attach_test.go:60-117 (watchResize takes a size func)
sizes <- size{cols: 90, rows: 31} // the terminal
winch <- syscall.SIGWINCH
// the closure given to watchResize subtracts the bar's reserved rows:
// next() must equal MakeClientClaimFrame(90, 30) with reserve 1, (90, 31) with reserve 0
```

### Reaped-session test shape (Item C)
```go
// Source: derived from internal/pty/adopt_test.go:272-310 and native_test.go:342
sent := recordSignals(t)
b := NewNativePTYBackend()
sess := startSh(t, b, `read x; exit 0`)
out := readAll(sess)
_, _ = sess.Write([]byte("\n"))
<-out.eof // (with a timeout, as in TestAdopted_ReapNaturalExit)
sess.ReapNaturalExit()
before := len(sent())
if err := b.Kill(sess.ID); err != nil { t.Fatal(err) }
if got := sent(); len(got) != before { t.Fatalf("Kill of a reaped session sent %v", got[before:]) }
```

### F-1 fix (Item F, prototyped and reverted)
```go
// internal/relay/history.go
// Skip counts n lines that were dropped before they reached history, so
// Pushed keeps the absolute row numbering monotonic (ESC[3J).
func (h *History) Skip(n int) { h.pushed += uint64(n) }

// internal/relay/hub.go, in the 'J' handler (hub.go:1297-1302)
if n == 3 {
	h.history.Clear()
	h.history.Skip(emu.ScrollbackLen()) // vt's own handler drops these next; count them
	h.wraps.clear()
}
```

### F-2 fix (Item G, prototyped and reverted)
```ts
// frontend/src/lib/pathLinks.ts:113
const onLeave = () => { pointerInside = false; invalidate() } // invalidate re-hovers only while pointerInside
```

### Event-based wait (Item H1)
```ts
// Source: vitest vi.waitFor; pathLinks.linkifier.test.ts:93-98
const renderOnce = async () => {
  const before = renders
  term.refresh(0, term.rows - 1)
  await vi.waitFor(() => expect(renders, 'term.onRender fired').toBeGreaterThan(before), { timeout: 5000, interval: 10 })
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Fixed `sleep` before asserting a render | `vi.waitFor` on the observable | vitest 0.34+ | No wall-clock flake |
| Trust a pid in a handshake frame | Kernel `LOCAL_PEERPID` | Phase 9 already added `PeerPID`; O-6 applies it on the takeover side | Spoofed or stale pid can no longer steer `lost`/COMMIT decisions |
| `didReceiveNotificationResponse` missing | Implement it on the existing delegate | This phase | Click routes to app code |

**Deprecated/outdated:** the todo's "ListSessions race" explanation for the sidebar flash (see B2).

## Runtime State Inventory

Not a rename/refactor/migration phase; omitted. One data-shape note: `History.pushed` is a plain counter in the relay handoff state; `Skip` only changes its value, no format change.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | macOS zsh/terminfo `clear` emits `ESC[H ESC[2J ESC[3J` as one write (the todo says `clear` reproduces; I verified vt's behaviour for those bytes, not the terminfo output) | Item F | The fix still holds for any sequence where ED2 and ED3 share a write; the test uses the bytes directly |
| A2 | `UNNotificationDefaultActionIdentifier`, the `didReceiveNotificationResponse` selector shape and delivery on a non-main queue | Item E | Handler never fires or crashes; caught by the notarized-build VM check (M-152) |
| A3 | A 35 s grace is acceptable UX for a genuinely unreachable host to keep showing as reachable | Item B2 | The user may prefer a shorter grace plus a daemon-side fix; the locked decision lets the planner choose |
| A4 | The B2 cause (stopped share server + 30 s probe cache) matches what the VMs saw; it fits the two 5 s samples but was not reproduced | Item B2 | If wrong, the grace does not remove the flash; CONTEXT says split a todo in that case |
| A5 | Absolute `CUP` is not offset without DECOM, so `--status-top` still overdraws row 1 for absolute-addressing apps | Pitfall 1 | If wrong, Top is also fully fixed; the manual VM check would show it |
| A6 | Clicking the notification activates the app with Accessory activation policy (tray-only) once `showWindow` runs `setDockVisible(true)` | Item E | Window may not come forward; VM check, then add `activateIgnoringOtherApps` |

## Open Questions (RESOLVED)

1. **G-1 flake (`TestKillClean_NoOrphans`) and SC7.**
   - What we know: it failed in the full run in Phases 12, 13 and 14, passes alone, and the user accepted it as unrelated. Item C edits the same package.
   - What's unclear: whether the user wants it made deterministic here (replace its two fixed sleeps with `WaitFor`) or kept as the documented re-run precedent.
   - Recommendation: make it deterministic (test-only, tiny, same spirit as H); otherwise plan the documented re-run `-count=5`.
   - RESOLVED: make it deterministic (15-CONTEXT.md Post-research decisions, [Flakes]).
2. **`--status-top` and DECOM.**
   - What we know: rows-1 stops overdraw at the bottom row only; Top needs origin mode for absolute-addressing apps.
   - What's unclear: whether the user accepts Top as a recorded limit.
   - Recommendation: ship rows-1 for both, record the Top limit in TESTING.md M-153 and the SUMMARY; do not add DECOM now.
   - RESOLVED: ship rows-1 for both; Top overdraw recorded as an accepted limit (15-CONTEXT.md Post-research decisions, [Attach bar]).
3. **Takeover of a pre-fix daemon's socket mode.**
   - What we know: the first upgrade from an older daemon keeps its 0755 socket (the listener fd is passed, not recreated).
   - What's unclear: whether a one-line `os.Chmod(socketPath, 0o600)` after COMMIT is wanted. Recommendation: yes, best-effort and logged; the parent dir is 0700 either way, so this is documentation accuracy, not an open threat.
   - RESOLVED: yes, best-effort post-COMMIT chmod 0600, logged, never fatal (15-CONTEXT.md Post-research decisions, [Socket hardening]).
4. **Grace length for B2** (A3/A4). Recommendation: 35 s, pure function, injectable clock.
   - RESOLVED (superseded): no time-based grace; a refused connection reads as reachable with 0 shared sessions, a timeout stays unreachable, classified at the internal/tailnet fetch site (15-CONTEXT.md Post-research decisions, [Sidebar]).

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go pinned toolchain | all Go tests | ✓ | go1.26.11 (`GOTOOLCHAIN=go1.26.11`; default `go` is 1.27.4) | none; root package refuses 1.27+ |
| Node / pnpm | vitest | ✓ | node 24.22.1, pnpm 12.6.0 | — |
| vitest | frontend tests | ✓ | 4.1.0 | — |
| Tart | manual VM rows (M-152, attach check) | ✓ | `~/.local/bin/tart` | — |
| Notarized build + the `portty-test` VMs | Item E live click | not checked (no live tailnet or VM use in research, by rule) | — | record as a manual M-NN, as decided |

**Missing dependencies with no fallback:** none for automated work.
**Missing dependencies with fallback:** the live notification click can only be verified in the VM with a notarized build (an ad-hoc build cannot post notifications, per the todo); automated tests cover Go routing and the React un-dismiss.

## Test Commands and Gates

(`workflow.nyquist_validation` is `false` in `.planning/config.json`, so the Validation Architecture section is intentionally omitted; these are the commands the planner needs.)

Go (always pin the toolchain; Phases 10-14 used exactly these, `[VERIFIED: TESTING.md:28 and 14-0x SUMMARYs]`):
- One package: `GOTOOLCHAIN=go1.26.11 go test -race -short -count=1 ./internal/pty/...` (also `./internal/attach/...`, `./internal/statusbar/...`, `./internal/fdpass/...`, `./internal/daemon/...`, `./internal/relay/...`, `./internal/webserver/...`).
- Root package (`cmd_attach_test.go`, `app_update_test.go`, `notification_darwin_test.go`): `GOTOOLCHAIN=go1.26.11 go test -race -short -count=1 .`
- Single test loop for the flake: `GOTOOLCHAIN=go1.26.11 go test -race -count=30 -run TestShare_PasteBacklogKeepsViewer ./internal/webserver`, run once more while `GOTOOLCHAIN=go1.26.11 go test -race -short -count=1 ./internal/relay/...` runs in a second shell.
- Phase gate, run once: `GOTOOLCHAIN=go1.26.11 go test -race -short -count=1 ./...` (14 `ok` packages; `internal/relay` alone took about 281 s under -race).
- Branch is `gsd/v1.2`, not a phase branch; commit docs with `gsd_run query commit`.

Frontend (from `frontend/`; `test: vitest run`, `[VERIFIED: frontend/package.json]`):
- One file: `cd frontend && pnpm exec vitest run src/lib/__tests__/pathLinks.linkifier.test.ts`
- Loop: `for i in $(seq 1 20); do pnpm exec vitest run src/lib/__tests__/pathLinks.linkifier.test.ts || break; done`; also run it beside a full `pnpm test` to recreate load.
- Full: `cd frontend && pnpm test` (122 files today).
- Type check is part of `pnpm build` (`tsc && vite build`); run `pnpm exec tsc --noEmit` after the `Tab`/prop changes.

Repo checks: `bash tests/check-traceability-paths.sh`; `golangci-lint run` if configured locally.

## Security Domain

`security_enforcement` is enabled (ASVS level 1, block on high).

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | unchanged (token-gated share links untouched) |
| V3 Session Management | no | unchanged |
| V4 Access Control | yes | daemon socket mode 0600 plus the existing `PeerUID` check (`handoff.go:462` path) and 0700 parent dir |
| V5 Input Validation | yes | fd and cmsg handling in `fdpass.Recv`; the handshake pid is validated against the kernel value |
| V6 Cryptography | no | none touched |
| V12 Files/Resources | yes | socket file permissions; fds closed on every error path |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Another local user connects to the daemon socket | Elevation/Info disclosure | chmod 0600 right after listen (O-2) on top of the 0700 parent directory and the `LOCAL_PEERCRED` uid check |
| fd leak through a malformed control message | DoS | close every fd collected before an error; walk cmsgs one by one (O-1) |
| Spoofed pid in HELLO-OK steering the takeover's "old daemon is gone" decisions | Spoofing/Tampering | compare to `LOCAL_PEERPID`, refuse and ABORT on mismatch (O-6); on a lookup error also refuse |
| Signal to an unrelated process group via a reused pgid | Tampering/DoS | skip the group signal once the child is reaped (C) |
| Notification click routing | Tampering | the handler acts only on the exact id `portty.update-available`, never downloads, never passes the id to a shell or URL |
| Path click resolving against the wrong directory | Info disclosure (wrong file previewed) | F-1 fix keeps rows monotonic; the host still enforces `previewfile` type and size checks unchanged |

The CSP, capability tokens and tailnet-only binding are not touched by any item.

## Sources

### Primary (HIGH confidence, files read this session)
- `.planning/phases/15-v1-2-hardening/15-CONTEXT.md`, `15-UI-SPEC.md`; the nine todos under `.planning/todos/pending/`; `.planning/config.json`
- Go: `cmd_attach.go`, `cmd_attach_resize.go`, `internal/attach/{attach.go,attach_unix.go,attach_test.go,loop.go}`, `internal/statusbar/bar.go`, `internal/relay/{hub.go,history.go,cwd.go,pathclick.go,cwd_test.go,pathclick_test.go,server.go}`, `internal/webserver/{server.go,input_queue_test.go,capability_helpers_test.go}`, `internal/pty/{cleanup.go,session.go,reap_other.go,adopt_test.go,native_test.go}`, `internal/daemon/{ipc.go,handoff.go,engine.go,share.go,tailnet_cache.go,api.go,socket.go,socket_test.go,handoff_test.go}`, `internal/fdpass/{fdpass.go,fdpass_test.go}`, `internal/tailnet/sessions.go`, `app.go`, `app_update.go`, `tray.go`, `tray_objc_darwin.m`, `notification_darwin.go`
- Frontend: `src/lib/{pathLinks.ts,closeReasons.ts,relayClient.ts,sidebarPeers.ts,tabs.ts}`, `src/components/{TerminalPanel.tsx,SessionEndedBanner.tsx,UpdateBanner.tsx,Sidebar.tsx,TabBar.tsx,WebShareSessionView.tsx}`, `src/App.tsx`, `src/lib/__tests__/pathLinks.linkifier.test.ts`, `frontend/package.json`, `frontend/vite.config.ts`
- Third-party source on disk: `golang.org/x/sys@v0.46.0/unix/sockcmsg_unix.go`, `github.com/coder/websocket@v1.8.15/dial.go`, `frontend/node_modules/@xterm/xterm/src/browser/Linkifier.ts`
- `TESTING.md` (Sections 2, 4, 5, 6), `tests/check-traceability-paths.sh`, `.planning/STATE.md`, `.planning/phases/10-input-path-reliability/10-VM-CHECKS.md` (RF-03 re-run note), `.planning/phases/09-*/09-SECURITY.md` (O-1, O-2, O-6)

### Experiments run this session (all reverted or outside the repo)
- Throwaway relay test for F-1 (reproduced, then fix validated); throwaway vitest for F-2 (reproduced, then fix validated); `LOCAL_PEERPID` probe in the scratchpad. `git status` afterwards matches the starting state.

### Tertiary (LOW confidence, not verified this session)
- Apple `UserNotifications` delegate API names and threading (A2); VT origin-mode semantics (A5); the exact bytes of macOS `clear` (A1).

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH, nothing new; versions read from manifests and module source
- Architecture / fixes A, C, D, F, G, H: HIGH, code read and (F, G, PEERPID) prototyped
- B1: HIGH on mechanism; B2: MEDIUM on cause
- E: MEDIUM, ObjC delegate semantics assumed until the VM check
- Pitfalls: HIGH where tied to read code; Pitfall 1 MEDIUM

**Research date:** 2026-11-22
**Valid until:** 2026-12-22 (stable code; re-check line numbers if other phases land first)
