# Phase 15: v1.2 Hardening - Context

**Gathered:** 2026-11-22
**Status:** Ready for planning

<domain>
## Phase Boundary

Close the bugs and test flakes found during v1.2 so the milestone ships clean. Source: the pending todos in `.planning/todos/pending/` (attach-statusbar-overwritten, ended-peer-tab-banner, killsession-stale-pgid, daemon-socket-hardening, update-notification-click, row0-relative-path-start-dir, first-cmd-click-same-cell-after-leave, linkifier-reconnect-flake, paste-backlog-flake). Out of scope: dompurify-3418 (blocked until 2026-11-28) and v12-release-checks (needs the user on a real Mac). No new features; fixes against existing behaviour only.

</domain>

<decisions>
## Implementation Decisions

### Update notification click
- Clicking the "Portty X is available" notification brings Portty forward (shows the window if hidden/tray-only) with the existing UpdateBanner in its "available" state; it does NOT start the download. Nothing downloads without the user pressing Download. (User said "You decide"; chosen for consent and smallest change.)
- Full click verification needs a notarized build in the Tart VM (ad-hoc builds can't post notifications); automate what can be automated (handler wiring, routing to show-window) and record the live click check as a manual TESTING.md item.

### Decided autonomously
- [Attach bar] Q: "How to keep full-screen apps off the status bar row?" → Selected: "Send rows-1 (bar shown) in the initial resize frame and on every SIGWINCH claim, for bottom and --status-top; the existing hub grid-owner model (claims + NoteTyping) makes the claiming/typing viewer's reduced height win; a test pins rows-1" — two call sites in cmd_attach.go / internal/attach, local revert
- [Ended peer tab] Q: "What does clicking an already-ended peer tab show?" → Selected: "Keep the tab's ended state; once a tab saw the session-ended close, no_such_session maps to 'session ended', not 'stopped sharing'" — frontend verdict mapping only
- [Ended peer tab] Q: "Fix the brief 'Can't reach this Mac' sidebar line after a host kill (todo part 2)?" → Selected: "Yes: find the ListSessions/teardown race and keep the last good peer state through a short grace; if the cause can't be found, split it into a new todo" — local UI state
- [Stale pgid] Q: "How to stop signalling a stale process group on dismiss?" → Selected: "Skip the group signal once the created session's wait has returned (mirror killAdopted's exited guard), test with a reaped session; items 2 and 3 get a guard or doc comment only (no caller today)" — small internal guard
- [Socket hardening] Q: "Which Phase 9 audit items (O-1, O-2, O-6)?" → Selected: "All three: chmod the socket 0600 right after listen with a mode test; close every fd in unparsed control messages in fdpass.Recv; read the old daemon pid via fdpass.PeerPID on takeover and refuse a mismatch with HELLO-OK (old daemon keeps serving)" — internal; a refused takeover falls back to the existing DEBT-08 guard
- [Path click] Q: "F-1: what directory applies to a path on row 0 printed before the first cwd report?" → Selected: "Fix in the relay cwd log (internal/relay/cwd.go and Hub.PathClick lookup) so output printed after a cwd report resolves against that report even when it lands on an earlier absolute row (e.g. after clear); test in internal/relay/pathclick_test.go" — internal lookup
- [Path click] Q: "F-2: first Cmd-click on the same cell after leave + text change" → Selected: "Call PathLinksHandle.invalidate() on mouseleave; add a real-xterm-Linkifier case (leave, change row, return to same cell, Cmd-click) to pathLinks.linkifier.test.ts" — one event hook
- [Flakes] Q: "How to make the two flaky tests deterministic?" → Selected: "Wait on events, not wall-clock: renderOnce resolves on term.onRender (with timeout); TestShare_PasteBacklogKeepsViewer's heartbeat limit becomes load-robust (event wait or scaled) without changing product heartbeat behaviour; then a loop run plus one clean full Go (-race) and vitest run" — test-only

### Claude's Discretion
- Notification click behaviour (user said "You decide"): show window + banner chosen, recorded above.

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/attach/attach.go` `MakeClientResizeFrame` / `MakeClientClaimFrame`; `internal/attach/attach_unix.go` `watchResize`; `cmd_attach.go` `runAttach` onConnect (initial resize at ~L397).
- `internal/statusbar/bar.go` reserves the row with DECSTBM (L116); knows Position Top/Bottom.
- `internal/relay/hub.go` `sizeClient` / `setOwnerLocked` / `applyGridLocked`: grid owner model, claims vs reports, ownerStale + NoteTyping.
- `app_update.go` `notifyUpdateOnce` and `sendNotificationFunc`; `notification_darwin.go`; `frontend/src/components/UpdateBanner.tsx`.
- `frontend/src/lib/pathLinks.ts` `PathLinksHandle.invalidate()`; test file `frontend/src/lib/__tests__/pathLinks.linkifier.test.ts`.
- `internal/pty/cleanup.go` (killAdopted exited guard), `internal/daemon/ipc.go`, `internal/fdpass/fdpass.go` (Recv, PeerPID), `internal/daemon/handoff.go`.
- `internal/webserver/input_queue_test.go` (paste backlog flake).

### Established Patterns
- Go tests use `internal/testutil.WaitFor` for event waits rather than sleeps.
- Peer tab verdicts: `peerConnectVerdict` maps close reasons; `CLOSE_REASON_SHARE_STOPPED` in frontend/src/lib.

### Integration Points
- TESTING.md standing rule: new test files go into Section 2 / Section 4; manual items (notification click) into Section 5; run `bash tests/check-traceability-paths.sh`.
- Fixed todos move from `.planning/todos/pending/` to `.planning/todos/completed/`.

</code_context>

<specifics>
## Specific Ideas

- Do not loosen product behaviour to fix flakes (heartbeat limits, render timing stay as shipped).
- Reliability first: the remote Mac can't be fixed by hand, so a hardening change that can fail (O-6 pid check) must fail toward the old daemon keeping its sessions.

</specifics>

<deferred>
## Deferred Ideas

- A flag to turn the `portty attach` status bar off (mentioned in the attach todo; new capability).
- dompurify-3418 (blocked until 2026-11-28) and v12-release-checks (real-Mac checks) stay in pending.

</deferred>
