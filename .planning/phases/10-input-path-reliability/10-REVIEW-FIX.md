---
phase: 10-input-path-reliability
fixed_at: 2026-11-21T06:45:53Z
review_path: .planning/phases/10-input-path-reliability/10-REVIEW.md
iteration: 1
findings_in_scope: 12
fixed: 10
skipped: 2
status: partial
---

# Phase 10: Code Review Fix Report

**Fixed at:** 2026-11-21T06:45:53Z
**Source review:** .planning/phases/10-input-path-reliability/10-REVIEW.md (committed as fcab16d3)
**Iteration:** 1

**Summary:**
- Findings in scope: 12 (all tiers, as the orchestrator asked)
- Fixed: 10 (CR-01, WR-01..WR-04, IN-02..IN-06)
- Skipped: 2 (IN-01 documented with a `shortcut:` comment instead of a code change; IN-07 left as asked)

## Fixed Issues

### CR-01: Revoking a share does not cancel the revoked viewer's queued input

**Files modified:** `internal/relay/input.go`, `internal/relay/hub.go`, `internal/relay/input_test.go`
**Commit:** e3e2eb9b
**Applied fix:** `RevokeWebViewers` collects the web viewers under `mu`, releases it, then calls `revokeInput`. Under `inMu` (a leaf), that marks each `Subscriber.inputRevoked` and drops the viewers' queued frames before the kick goes out. Later `Write`s from a revoked pump return nil and are dropped, and a pump waiting at quota wakes. Only the frame already in the PTY write can still land. An ordinary disconnect (`InputConn.Close`) still delivers its backlog (INPT-01). The revoke diag line gains `input-dropped frames= bytes=`. Test: `TestHubInput_RevokeDropsBacklog`. The PTY is stuck with one web frame in flight, then more web frames and one local frame are queued and the share is revoked. After that the PTY gets exactly the in-flight frame plus the local frame, and a web write after the revoke is dropped.

### WR-01: The input queue has no hub-wide or daemon-wide bound

**Files modified:** `internal/relay/input.go`, `internal/relay/input_test.go`
**Commit:** be893a7b
**Applied fix:** Adds `InputHubMaxBytes = 64 MiB`. Once a connection has a frame queued, `Write` also blocks (never drops) while `inBytes+len(b)` would pass the hub cap. The per-connection quota is unchanged. The bound is the cap plus one frame per connection. Test: `TestHubInput_HubCapBlocks`. Two connections at 1,024 full-size frames fill the hub; a third gets one frame in, blocks on its second, and is released once the PTY drains.

### WR-02: Across a handoff, the old daemon's backlog interleaves with the takeover's input

**Files modified:** `internal/relay/input.go`, `internal/relay/hub.go`, `internal/daemon/handoff.go`, `internal/daemon/handoff_test.go`
**Commit:** 1857e271 (includes a fixup for a race the gate found: the release goroutine read `maxFrozen()` while tests changed the timeouts; it now captures the value before it starts)
**Applied fix:** This is the review's option (a), not the drain-before-COMMIT option the orchestrator preferred. The new `Hub.HoldInput`/`ReleaseInput` stop the writer taking frames while `Write` still queues them within the quotas. `takeover.commit(withCommitted)` holds every imported hub before `serveOn`, so no viewer can reach a hub unheld. It releases them when `watchOldExit(oldPID)` (`pty.WatchExit`, kqueue NOTE_EXIT) fires, or after `maxFrozen()` with a log line if the exit is never seen. The `lost()` path (old daemon already gone) does not hold.
Why this choice and not drain-before-COMMIT with rollback: viewers stay connected to the old daemon and keep typing until `finish` sends them 1012, which is after COMMITTED. A queue drained before COMMIT therefore refills. Closing that gap would mean blocking or holding those keystrokes in the old daemon, and they die with it on commit. Pongs could not cover single keystrokes either, so that loss would be silent. Holding the takeover keeps strict order: every old-daemon write happens before the old process exits, and every takeover write after. It adds no new loss, and the only cost is that the takeover's input is delayed until the old daemon exits (at most `restartDrain` after COMMITTED, usually milliseconds). No freeze timeout changed, so `TestHandoff_TimeoutsFitClientRetry` holds. The existing accepted limit is unchanged: a backlog still queued in the old daemon at `restartDrain` is lost and logged. The comments in `finish` and input.go now say "nothing reorders".
Test: `TestHandoff_TakeoverHoldsInputUntilOldExits`. The old side is parked at `committed-received`. The takeover's input stays queued (not idle, 1 byte) for 200 ms, and is written once the old daemon's exit seam fires. The scene gains a `watchOldExit` seam that is closed at the first `handoffExit`.

### WR-03: The window-resize claim fires on resizes the OS makes

**Files modified:** `frontend/src/components/TerminalPanel.tsx`, `frontend/src/components/__tests__/TerminalPanel.scale.test.tsx`
**Commit:** fb90a8fa
**Applied fix:** After the 150 ms debounce, the window-resize handler claims only when `document.visibilityState === 'visible' && document.hasFocus()`; otherwise it calls `reportFit()`. The existing three-resizes test now stubs `hasFocus` to true (jsdom returns false with nothing focused). The new test (focus false) checks that an unfocused resize reports 80x30 and sends no claim.

### WR-04: The `ErrInputClosed` exit races WritePump's "session ended" close

**Files modified:** `internal/relay/server.go`, `internal/webserver/server.go`, `internal/relay/input.go`, `internal/relay/server_input_queue_test.go`
**Commit:** ac85ae30
**Applied fix:** Both read pumps now ignore `ErrInputClosed` (input dropped) instead of returning, so `readDone` never closes because the input died. When the hub shuts down, `WritePump`'s `hub.Done` case is the only end, and it always sends 1000 "session ended". This is deterministic, so the nested select the review suggested is not needed. When a PTY write fails on a live hub, the viewer stays connected and keeps getting output and pongs, so typing no longer drops it into a reconnect loop. `killInputLocked` logs the death once. Tests: `TestRelay_InputClosedDefersToSessionEnd` (a pump blocked at quota when the hub shuts down still gets 1000 + `CloseReasonSessionEnded`) and `TestRelay_DeadInputKeepsViewer` (after the PTY pipe closes and the input dies, more input plus a ping still gets a pong).

### IN-02: `inMu` is held across `log.Printf`

**Files modified:** `internal/relay/input.go`, `internal/relay/hub.go`
**Commit:** 33736a61
**Applied fix:** `Close`, the done watcher and the writer now format their diag lines under `inMu` and log them after unlocking. `killInputLocked` returns its line instead of logging it. The writer unlocks just to log and re-checks its state at the top of the loop. The log strings are unchanged.

### IN-03: `Hub.WriteInput` has no production callers and bypasses the FIFO

**Files modified:** `internal/relay/hub.go`, `internal/relay/hub_test.go`, `internal/daemon/engine_test.go`
**Commit:** 10e54401
**Applied fix:** Chesterton check: the only callers were `TestCreateSession_LoginShell` (engine_test.go) and `TestHubWriteInput`, and 10-03 had kept the method only for them. `WriteInput` is removed. The login-shell test now types through `hub.OpenInput(nil).Write`. `TestHubWriteInput` is removed with the method: it only checked that one direct write returned without error. The FIFO is covered by input_test.go.

### IN-04: Peer-row matching still keys by `hostname` while tab ids key by `connectHost`

**Files modified:** `frontend/src/App.tsx`, `frontend/src/components/Sidebar.tsx`, `frontend/src/components/__tests__/Sidebar.test.tsx`, `frontend/src/components/__tests__/App.open-remote.test.tsx`
**Commit:** eb3f43f6
**Applied fix:** `dropPeerSession` now takes the connect host (`peerSnapshotHost(activeWebTab)`). It matches peers by `peer.dnsName || peer.hostname` and tabs by `peerSnapshotHost(tab)`. The Sidebar's `aria-current` compares the same pair. (The review's `peerDNSName` exists only on adapted sessions; the peer object's field is `dnsName`.) Tests: two peers named `air` with different MagicDNS names highlight only the tab's row. When the host stops sharing on one of two `Air` peers, the other peer's row and live tab stay (re-clicking it does not reconnect).

### IN-05: The web-session fallback id now yields `host/id`

**Files modified:** `frontend/src/App.tsx`, `frontend/src/lib/tabs.ts`, `frontend/src/lib/__tests__/tabs.test.ts`
**Commit:** 828574c3
**Applied fix:** The new `hostlessWebSessionId(tabId)` returns the session id of a hostless tab id, or null for a peer tab's `host/id`. The render branch returns null when the tab is missing and its id is a peer id, and falls back only to a hostless id. Unit tests cover both shapes. The existing source pin for the desktop guard still passes.

### IN-06: `retryInit` drops snapshots of replaced peer tabs but not their font sizes

**Files modified:** `frontend/src/App.tsx`, `frontend/src/components/__tests__/App.daemon-recovered.test.tsx`
**Commit:** c9ddaeba
**Applied fix:** `retryInit` keeps the `peerTabsRemoved` list. It drops each snapshot as before and also deletes those tabs' `fontSizes` entries outside the tab updater. Test: a peer tab opened during Retry and zoomed by +1 is replaced by Retry, and the reopened tab starts at the default size. The `WebShareSessionView` mock in that file now exposes `fontSize` and a font-up button.

## Skipped Issues

### IN-01: A claim made while the hub is frozen is lost on commit

**File:** `internal/relay/hub.go` (`sizeClient`, frozen branch)
**Reason:** Not cheap to fix correctly, so it is documented as allowed. Export runs during the state send, so exporting `WantCols/WantRows` would cover only claims made before Export, not those during the state send, READY or the commit window. The owner cannot move either, because subscribers reconnect. A `shortcut:` comment at the frozen branch records the limit and the upgrade path: TerminalPanel re-sends its last unanswered claim on reconnect. Commit c86efe48 (comment only).
**Original issue:** On a frozen hub, ClaimGrid and NoteTyping only record the target. A committed handoff never thaws, and HubState carries no target, so the claim disappears.

### IN-07: The INPT-06 frontend invariants are partly pinned by source-text assertions

**File:** `frontend/src/components/__tests__/TerminalPanel.scale.test.tsx:224-271`
**Reason:** Left as the orchestrator asked ("leave unless trivially replaceable"). Replacing the string pins means auditing each one against the behavioral block, which is not trivial. The pins still pass after the WR-03 change.
**Original issue:** Several tests slice the component source and `toContain` code strings, so they break on harmless refactors and do not prove behavior.

## Verification

All gates ran in this isolated worktree (`.claude/worktrees/agent-a797c581865c5c899`, fast-forwarded to gsd/v1.2 9a18160c), not in the main checkout. Output files are in `/private/tmp/claude-501/-Users-ken-dev-portty/66f0384b-8902-40e9-9d84-68c47b57f8f8/scratchpad/`:

| Gate | Result | File |
|---|---|---|
| `GOTOOLCHAIN=go1.26.11 go build ./... && go vet ./...` | rc 0 | gate-build-vet.txt |
| `go test -race -short -count=1 ./...` | rc 1: one failure, `TestHandoffFault_AbortBeforeThaw`, a DATA RACE in my WR-02 code (the release goroutine read `maxFrozen()`). Fixed (now in 1857e271). Every other package was ok | gate-go-test.txt |
| `go test -race -short -count=1 ./internal/daemon/` (rerun of the failing package after the fix) | ok, rc 0 | gate-go-test-daemon-rerun.txt |
| `cd frontend && pnpm vitest run` | 95 files, 1338 tests passed | gate-vitest.txt |
| `pnpm tsc --noEmit` | rc 0 | gate-tsc.txt |
| `bash tests/check-brand.sh` | OK: brand gate clean | gate-check-brand.txt |
| `bash tests/check-traceability-paths.sh` | OK: 174 traceability paths exist | traceability.txt |

TESTING.md: no test files were added or removed. A Phase 10 code-review note was added, and the Section 4 INPT-01/04/06 rows were extended (commit aeabd4ed).

Logic fixes that need human verification (they pass the tests, but they change behavior on the real two-Mac path): CR-01 (revoke drops the backlog), WR-02 (takeover input held until the old daemon exits: worth a VM V-02 rerun while typing), WR-03 (focus gating on a real display sleep or unplug), WR-04 (the session-ended close while pasting).

---

_Fixed: 2026-11-21T06:45:53Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_

## Iteration 2

**Fixed at:** 2026-11-21T07:10:00Z
**Source review:** 10-REVIEW.md "## Iteration 2" (committed as 22a3d51e)
**Findings in scope:** 4 (CR-02, WR-05, IN-08, IN-09). **Fixed:** 4 (IN-08 as a comment, as asked). **Skipped:** 0.

Lock order is unchanged: the new `webEpoch` is read and written only under `inMu` (a leaf; `OpenInput` takes it alone), and the new `Subscriber.revoked` only under `mu` (set in `RevokeWebViewers` before it releases `mu`, read in `NoteTyping` and `sizeClient` under `gridMu` then `mu`). No IO runs under either lock.

### CR-02: Stop Sharing still types the queued input of a web viewer that already disconnected

**Status:** fixed: requires human verification
**Files modified:** `internal/relay/input.go`, `internal/relay/hub.go`, `internal/relay/input_test.go`
**Commit:** 973af88b
**Applied fix:** the reviewer's epoch design. `Hub.webEpoch` (under `inMu`) counts `revokeInput` calls; `OpenInput` records it in `InputConn.epoch`. `InputConn.revoked()` is now true for a web conn opened before the last revoke, or whose viewer was a revoke target (`inputRevoked`, which still covers Join before the revoke and OpenInput after). So the revoke's compaction loop drops every queued web frame of the hub, including those of closed and unsubscribed conns, and a pump blocked in `Write` after its handler unsubscribed wakes on the revoke's `Broadcast` and returns nil (dropped). Local-origin input and `nil`-sub conns are never revoked. A disconnect without a revoke still delivers (`TestHubInput_CloseKeepsBacklog` unchanged). Both production callers (`unshareLocked`, `remintSharesLocked`) revoke every web link of the session, so dropping all web input is the intended scope.
Tests: `TestHubInput_RevokeDropsDepartedBacklog` (web frames queued behind an in-flight frame, then the conn is closed and unsubscribed, then revoked: the PTY gets the in-flight frame, the local frame, and a frame from a web viewer that joined after the revoke, nothing else). `TestHubInput_RevokeDropsBlockedPumpAfterUnsubscribe` (a pump at quota blocked in Write, unsubscribed, revoked: the Write returns nil, and after the in-flight frame the PTY gets only a later local frame).

### WR-05: The hub cap makes a typing viewer block behind another connection's backlog

**Status:** fixed
**Files modified:** `internal/relay/input.go`, `internal/relay/input_test.go`
**Commit:** 4d32b433
**Applied fix:** `InputConnHubAllowance = 64 KiB`. The hub-cap term in `Write` applies only once the connection's own queue would pass the allowance, so typing and small pastes never wait on another connection's backlog. Hard bound: `InputHubMaxBytes` + 64 KiB per connection (a frame is at most `MaxInputPayload` = 32,767 bytes, below the allowance). The shortcut comment now says a viewer with over 64 KiB queued can be dropped because of another connection's backlog, and that the partial-paste notice covers only split pastes.
Tests: `TestHubInput_HubCapAllowsTyping` (new `fillHub` puts the hub exactly at the cap; a fresh connection's 100 one-byte writes all return). `TestHubInput_HubCapBlocks` now uses `fillHub`: a connection gets two full frames in (the allowance), blocks on the third, and is released once the PTY drains.

### IN-09: A revoked viewer can still move the grid until its socket closes

**Status:** fixed
**Files modified:** `internal/relay/hub.go`, `internal/relay/hub_grid_owner_test.go`
**Commit:** 4df30b42
**Applied fix:** the reviewer's flag option, not removal from `h.subscribers` (removal would drop the viewer's output and its `leave` diag line before the kick). `RevokeWebViewers` sets `Subscriber.revoked` on each target under `mu`. `NoteTyping` and `sizeClient` ignore a revoked viewer, and `sizeClient` now also ignores a viewer that is no longer subscribed, as `NoteTyping` already did, so a late claim cannot leave a departed owner behind. The relay package passed in full with the membership check before the commit (`it2-in09-relay.txt`), and the full gate covers the daemon and webserver callers.
Test: `TestHub_GridRevokedOrGoneViewerCannotClaim` (after a revoke, a web viewer's claim and typing, and a departed local viewer's claim, leave the owner and the 100x30 grid unchanged and call resizeFn no more).

### IN-08: WR-03's focus gate stands in for "a person is here"

**Status:** fixed (comment only, as asked)
**Files modified:** `frontend/src/components/TerminalPanel.tsx`
**Commit:** 1d9cf77d
**Applied fix:** behavior unchanged. The WR-03 comment now says visible-and-focused is a proxy for a person being present, and a `shortcut:` comment records both residual gaps (a key window on an unlocked, unattended Mac still claims on a display change; a drag of an inactive window only reports) and the upgrade (gate on recent `pointerdown`/`keydown`).

### TESTING.md

No test files added or removed (Go 152, vitest 95). An iteration-2 note was added and the Section 4 INPT-01 `internal/relay/input_test.go` and INPT-06 `internal/relay/hub_grid_owner_test.go` rows extended (commit feea6a73). `bash tests/check-traceability-paths.sh`: OK, 174 traceability paths exist (`it2-traceability.txt`).

### Verification

Gates ran once each in this isolated worktree (`.claude/worktrees/agent-a3113eab6d807198a`, fast-forwarded to gsd/v1.2 46f49642), not in the main checkout. Output files are in `/private/tmp/claude-501/-Users-ken-dev-portty/66f0384b-8902-40e9-9d84-68c47b57f8f8/scratchpad/`:

| Gate | Result | File |
|---|---|---|
| `GOTOOLCHAIN=go1.26.11 go build ./... && go vet ./...` | rc 0 | it2-gate-build-vet.txt |
| `go test -race -short -count=1 ./...` | rc 0, every package ok, no reruns needed | it2-gate-go-test.txt |
| `cd frontend && pnpm vitest run` (after `pnpm install --frozen-lockfile --offline`) | 95 files, 1338 tests passed | it2-gate-vitest.txt |
| `pnpm tsc --noEmit` | rc 0 | it2-gate-tsc.txt |
| `bash tests/check-brand.sh` | OK: brand gate clean | it2-gate-check-brand.txt |

Focused runs during the fixes: it2-cr02-focused.txt, it2-wr05-focused.txt, it2-in09-relay.txt, it2-in09-focused.txt (all ok).

Needs human verification: CR-02 changes what Stop Sharing discards on the real two-Mac path (worth a VM check: paste into `sleep 600` from a guest, close the guest tab, Stop Sharing, confirm nothing runs when `sleep` ends, while the host's own tab still types).

---

_Fixed: 2026-11-21T07:10:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 2_
