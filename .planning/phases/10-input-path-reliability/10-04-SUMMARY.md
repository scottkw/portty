---
phase: 10-input-path-reliability
plan: 04
subsystem: relay hub grid, local relay pump, share server pump, handoff import
status: complete
tags: [inpt-06, grid-owner, resize, claim, handoff, v-02]
requires: [10-03]
provides:
  - "relay.ResizeClaim and relay.ParseResize (claim = MsgResize2 payload[4] bit 0)"
  - "Hub.ClaimGrid, Hub.ResizeClient that only stores sizes (plus the bootstrap), Hub.GridKnown, grid clamp constants"
  - "both read pumps send claims and reports to the hub; read-only viewers skipped"
  - "Phase 9 settle window removed; local join pushes no fallback grid frame"
affects: [10-05 (viewer sends claims; already merged, wire matches), 10-06 (typing claims go through the owner/target fields; NoteTyping), 10-07 (VM greps the diag lines below, TESTING.md rows, V-02 re-run)]
tech-stack:
  added: []
  patterns: ["grid owner + grid target under mu; only claims and the bootstrap set the target; applyGridLocked moves the PTY to the target"]
key-files:
  created:
    - internal/relay/hub_grid_owner_test.go
    - internal/webserver/claim_test.go
  modified:
    - internal/relay/protocol.go
    - internal/relay/protocol_test.go
    - internal/relay/hub.go
    - internal/relay/server.go
    - internal/relay/handoff_state.go
    - internal/relay/handoff_state_test.go
    - internal/relay/history_test.go
    - internal/relay/hub_test.go
    - internal/relay/server_test.go
    - internal/webserver/server.go
    - internal/webserver/server_test.go
    - internal/webserver/revoke_test.go
    - internal/daemon/attach_viewers_test.go
decisions:
  - "Grid owner model replaces VIEW-02 origin gate and D-02 min-among-local: a plain report only stores the viewer's size, owner or not; only a claim (or the first local report on a never-sized, ownerless grid) moves the grid"
  - "The diag line's by= is the owner's origin and client= its name (both empty at a thaw after the owner left); conn= and why= are appended"
  - "Phase 9 settle window removed: an imported hub's grid is known, so joins and reports cannot move it; TestHub_HandoffRejoinKeepsGrid pins V-02"
  - "settleHub fixture renamed importedGridHub, not importedHub: handoff_state_test.go already has an importedHub(t, orig) helper"
metrics:
  duration: "~70 min (about 10 min of it waiting for 10-05's full gate run to free the host PTY pool)"
  completed: 2026-11-20
estimate:
  tokens: 140000
  tasks: 3
actuals:
  tokens: 14600
  tasks: 3
  commits: 6
plan_head_before: 1d72a6e5
plan_head_after: ba9811bc324c562c9319197eedb41242f192a4d1
---

# Phase 10 Plan 04: Grid owner (server half of INPT-06) Summary

The hub now has a grid owner and a grid target. The PTY grid is whatever the owner last claimed (or what the first local report bootstrapped on a never-sized grid). A plain report from any viewer, the owner included, only stores that viewer's size. Claims arrive as bit 0 of byte 5 of `MsgResize2` from any write-capable viewer, local or web. Read-only viewers can neither report nor claim. Sizes are clamped to 20..500 x 5..200. The Phase 9 settle window is gone, and a handoff's rejoining viewers still get deltas with no grid change.

## What was built

- **Wire** (`protocol.go`): `ResizeClaim = 0x01` and `ParseResize`. Fewer than 4 payload bytes is not ok, exactly 4 bytes is a report, and other bits of byte 5 are ignored. `MakeResizeFrame` (0x02) is unchanged and `APIVersion` stays 2.
- **Hub** (`hub.go`): new fields under `mu`: `owner`, `wantCols`, `wantRows`. `ResizeClient` and `ClaimGrid` share `sizeClient`, which applies the read-only gate, ignores zero or negative sizes, clamps, and stores the size on the sub. A claim sets owner and target and applies. A report returns right after storing, except for the bootstrap (local origin, no owner, `ptyCols == 0 || ptyRows == 0`). On a frozen hub the owner and target are recorded and Thaw applies them. A report sets no target, so Thaw never applies a report. `Unsubscribe` clears the owner under `mu` only, and the grid and target stay (D-01). `applyGridLocked(why)` moves the PTY to the target and never reads a sub's reported size. Lock order is unchanged (gridMu then mu, and mu is released before broadcastResize, resizeLiveEmulator and resizeFn). The arbiter doc comment carries the Chesterton notes (VIEW-02, D-02) and the `shortcut:` for "relaunched GUI shows the current grid, scaled, until its user types, zooms or resizes".
- **Pumps**: `relay/server.go` and `webserver/server.go` both run `ParseResize`, then call `ClaimGrid` (claim bit set) or `ResizeClient` (report), and skip `sub.ReadOnly`. The share pump still logs the first resize per connection only (CR-01), now with true wording, and the `resizes=` total is unchanged. The VIEW-02 comment block is rewritten for INPT-06.
- **Settle window removed**: `settleCols`, `settleRows`, `settleTimer`, `handoffSettle` and `endSettle` are deleted (the fields, the const, the timer arm in `Start`, the stop in `Shutdown` and the assignment in `NewHubFromState`). `Start` is now `startOnce.Do(func() { go h.Run() })`, with a comment explaining why no grace period is needed. That comment names none of the removed identifiers (the gate greps for 0 matches and found 0).
- **Join push guard**: `Hub.GridKnown()`. The local relay pushes its join grid frame only when the grid is known. The share server keeps its push, including the 220x50 fallback for web guests, as before.

## Exported API

```go
// internal/relay/protocol.go
const ResizeClaim byte = 0x01 // MsgResize2 payload[4], bit 0
func ParseResize(payload []byte) (cols, rows int, claim, ok bool)

// internal/relay/hub.go
const (
	MinGridCols = 20
	MaxGridCols = 500
	MinGridRows = 5
	MaxGridRows = 200
)
func (h *Hub) ClaimGrid(sub *Subscriber, cols, rows int) error
func (h *Hub) ResizeClient(sub *Subscriber, cols, rows int) error // now: store only, plus the bootstrap
func (h *Hub) GridKnown() bool
```

Claim frame on the wire (client to server): `[0x11, cols_hi, cols_lo, rows_hi, rows_lo, 0x01]`. A report is `[0x11, cols_hi, cols_lo, rows_hi, rows_lo]`.

## Log lines (verbatim formats)

```
viewer-diag: grid session=%s %dx%d -> %dx%d by=%s client=%q conn=%d why=%s   // why = bootstrap | claim | thaw (10-06 adds input); by/client/conn = the owner's, empty/0 if it left before a thaw
viewer-diag: web resize session=%s conn=%d cols=%d rows=%d claim=%t            // first resize frame per share connection only (replaces "web resize ignored"); cols/rows as sent, before the clamp
```

`daemon_log_test.go`'s three substrings still match (`viewer-diag: grid session=<id8>`, `0x0 -> 120x40`, `by=local`).

## Tests added or changed (for 10-07's TESTING.md)

| File | Tests |
|------|-------|
| internal/relay/protocol_test.go (extended) | TestParseResize (subtests: 4 bytes is a report, claim bit, zero byte 5 is a report, other bits ignored, big-endian, 3 bytes is not ok) |
| internal/relay/hub_grid_owner_test.go (new) | TestHub_GridBootstrapFirstLocalReport, TestHub_GridReportFromNonOwnerIgnored, TestHub_GridClaimTakesOwnership, TestHub_GridOwnerReportStoredOnly, TestHub_GridFrozenReportNotAppliedOnThaw, TestHub_GridOwnerLeavesFreezes, TestHub_GridWebClaim, TestHub_GridReadOnlyIgnored, TestHub_GridClamp, TestHub_GridFrozenClaimAppliedOnThaw, TestRelayPump_ClaimTakesGrid |
| internal/webserver/claim_test.go (new) | TestWebClaim_WriteCapTakesGrid, TestWebClaim_ReadOnlyCapIgnored, TestWebReport_KnownGridUnchanged, TestWebClaim_Clamped |
| internal/relay/handoff_state_test.go (rewritten part) | TestHub_HandoffRejoinKeepsGrid (replaces TestHub_HandoffSettleKeepsGrid), TestHub_HandoffFrozenClaimAppliedOnThaw (replaces TestHub_HandoffSettleSkipsFrozen); fixture settleHub renamed importedGridHub |
| internal/relay/server_test.go (extended) | TestRelayJoin_NoGridPushWhenUnknown |
| internal/relay/hub_test.go (changed) | TestHub_ResizeOwner_SecondLocalReportKeepsGrid replaces TestHub_ResizeHostAuthority_MinAmongLocal; TestHub_ResizeFreezeLastHostSize and TestHub_ResizeIgnoresWebOrigin unchanged in assertions, comments now say "a web report never moves the grid" |
| internal/relay/history_test.go (changed) | helper resizeHub calls ClaimGrid (a window resize is a claim); every relay test that uses it passes unchanged |
| internal/webserver/revoke_test.go (changed) | TestResizeFlood_LogsOncePerConnection counts `viewer-diag: web resize ` (still exactly 1, `resizes=50`) |
| internal/webserver/server_test.go (comments only) | TestWebReadPump_DropsGuestResize: now pins "a plain report never moves a known grid" |
| internal/daemon/attach_viewers_test.go (changed) | TestCLIAttach_OtherViewersKeepOutput's timeline regex now matches `viewer-diag: web resize session=<id8> conn=N cols=80 rows=24 claim=false`; H-grid/H-resize comments reworded; no assertion loosened |

## Coordinator check: the 10-05 wire contract

All three assumptions hold, and each one is pinned by a test from this plan:

1. A claim is `[0x11, cols_hi, cols_lo, rows_hi, rows_lo, 0x01]` and a 4-byte payload is a report. Pinned by TestParseResize, by TestRelayPump_ClaimTakesGrid (sends those exact bytes through the local pump, and a 4-byte frame afterwards changes nothing) and by TestWebClaim_WriteCapTakesGrid / TestWebReport_KnownGridUnchanged.
2. The claiming viewer gets the 0x02 frame too. `broadcastResize` goes to every subscriber, the claimer included. Pinned by TestHub_GridClaimTakesOwnership (both viewers get exactly one 80x24 frame), TestRelayPump_ClaimTakesGrid (viewer B claims and reads 80x24 over the socket) and TestWebClaim_WriteCapTakesGrid (the claiming web viewer reads 120x30). Caveats that are by design:
   - A claim equal to the current grid sends no frame, because nothing changed and the viewer already shows that grid.
   - A clamped claim echoes the clamped size (a 4000x4000 claim gets a 500x200 frame).
   - A claim on a frozen hub (during a handoff) echoes only at Thaw. If the handoff commits instead, it is lost with the old daemon: no frame arrives and the viewer keeps showing the imported grid until its next claim.
   - A read-only viewer's claim gets no frame.
3. No join-time grid frame while the grid is unknown, on the local relay path. Pinned by TestRelayJoin_NoGridPushWhenUnknown. The share server still pushes the 220x50 fallback to web guests on a never-sized hub (plan decision, unchanged behavior). A web report never bootstraps the grid, so a browser on a never-sized hub follows 220x50 until a local viewer reports or the browser claims.

## TDD Gate Compliance

| Task | RED commit | Target | RED evidence | GREEN commit |
|------|-----------|--------|--------------|--------------|
| 1 | 4e00de89 | TestParseResize, TestHub_Grid*, TestRelayPump_ClaimTakesGrid | 11 failures on assertions, e.g. `a non-owner's report moved the grid: [80 24]`, `claim 5000x5000: grid [5000 5000], want 500x200`, `ParseResize([0 80 0 24 1]) = 80, 24, false, true`. TestHub_GridBootstrapFirstLocalReport passed on RED: today's code already behaves that way. | 0d20731d |
| 2 | e544666e | TestWebClaim_WriteCapTakesGrid, TestWebClaim_Clamped | `hub grid 100x40, want 120x30` (the share pump dropped the claim). ReadOnly and 4-byte-report cases passed on RED: they pin behavior that must not change. | 032457ce |
| 3 | 60860c59 | TestRelayJoin_NoGridPushWhenUnknown | `a never-sized hub pushed a grid frame [0 220 0 50] at join`. TestHub_HandoffRejoinKeepsGrid and TestHub_HandoffFrozenClaimAppliedOnThaw passed on RED because the owner model from Task 1 already covers them (the plan expected this). | ba9811bc |

RED commit 4e00de89 ships compile-only stubs: `ParseResize` reports only, and `ClaimGrid` just calls `ResizeClient`. That way the commit builds and fails on assertions. 0d20731d replaces both.

## Gate results

- Task 1 targeted run (the plan's -run set): ok, log `/private/tmp/claude-501/-Users-ken-dev-portty/66f0384b-8902-40e9-9d84-68c47b57f8f8/scratchpad/p10-04-t1.log`. It also served as the tracer re-verify after GREEN.
- Task 2 targeted runs: ok, log `/private/tmp/claude-501/-Users-ken-dev-portty/66f0384b-8902-40e9-9d84-68c47b57f8f8/scratchpad/p10-04-t2.log`.
- `go build ./... && go vet ./...`: clean.
- The single full run `go test -race -short -count=1 ./...`: exit 0, 0 `--- FAIL` / `^FAIL`, 0 DATA RACE, 13 packages ok, no reruns. Log: `/private/tmp/claude-501/-Users-ken-dev-portty/66f0384b-8902-40e9-9d84-68c47b57f8f8/scratchpad/p10-04-gate-go.log`.
- check-brand and check-traceability-paths: pass.
- Guards: 0 settle identifiers in `internal/relay/*.go`. `APIVersion` is 2 and `MaxInputPayload` is 32767. `git diff d027e86d` on csp_mw.go, capability_mw.go and tailnet/live.go is empty. `git diff 1d72a6e5 -- frontend/` is empty.

## Deviations from Plan

1. **Fixture name.** The plan said to rename `settleHub` to `importedHub`, but `handoff_state_test.go` already defines `importedHub(t, orig)`. Used `importedGridHub` instead.
2. **[Style] gofmt on webserver/server.go.** The file had a pre-existing misaligned comment from 10-03 on the `in := hub.OpenInput(sub)` line. Running gofmt on the edited file realigned it (whitespace only).
3. **Ledger and logs in the scratchpad.** The worktree sandbox blocks writing under `.git/worktrees/...`, so the plan base `1d72a6e5` is in `/private/tmp/claude-501/-Users-ken-dev-portty/66f0384b-8902-40e9-9d84-68c47b57f8f8/scratchpad/p10-04-base` (not /tmp/p10-04-base). Gate logs are in the same scratchpad, not /tmp.
4. **Comment-only touch on server_test.go** (not in files_modified). The plan's text asks to update the comments of TestWebReadPump_DropsGuestResize; no assertion changed.
5. **Waited for 10-05's full gate run** to finish before starting this plan's single full run, to avoid PTY pool pressure. Nothing was rerun.

## Known Stubs

None.

## Threat Flags

None new. T-10-12 is mitigated: read-only is skipped in both pumps and in the hub (TestHub_GridReadOnlyIgnored, TestWebClaim_ReadOnlyCapIgnored). T-10-13 is mitigated: sizes are clamped at intake (TestHub_GridClamp, TestWebClaim_Clamped). T-10-15 is mitigated by TestHub_HandoffRejoinKeepsGrid, with the V-02 VM re-run owed by 10-07. T-10-16 holds: the log lines carry sizes, conn ids and a flag only. T-10-14 (claim storm) is accepted as planned.

## Self-Check: PASSED

- Created files exist: internal/relay/hub_grid_owner_test.go, internal/webserver/claim_test.go.
- Commits 4e00de89, 0d20731d, e544666e, 032457ce, 60860c59 and ba9811bc are ancestors of HEAD.
