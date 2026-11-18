---
phase: 01-plain-macos-terminal
plan: 06
subsystem: daemon status / tabs / exit toast / hub
tags: [strip-down, status-model, colorblind-safe, hub, exit-behavior]
status: complete
requires:
  - phase: 01-05
    provides: Shell-only sessions, no CLI detection; engine status map never written since 01-04
provides:
  - Daemon status derived from process state (running / exited / errored) via deriveStatus(stopped, exitCode) in ListSessions and GetSessionStatus
  - internal/status (agent output heuristics) deleted
  - Shared TS normalizer normalizeStatus/statusLabel in lib/hubStatus.ts (legacy/unknown -> running)
  - Tab dot shapes: filled circle Running, hollow circle Exited, rounded square Error (title + aria-label)
  - Exit toast meta "Exited" / "Exited with error"; session:exit payload has no cli/finalStatus
  - Hub: three-state cards, no status pill group, errored-only attention, search-only filter bar, UI-SPEC empty-state copy
affects: [01-07, 01-10]
actuals:
  tokens: 40900
  tasks: 2
  commits: 3
plan_head_before: 82d38088c36b0f684ad94d5eddc4a5f1b3e1391c
plan_head_after: c8296637c28c5f30ebe853a974c67770db019616
tech-stack:
  added: []
  removed: ["internal/status heuristic detector", "maybeNotifyWaiting edge trigger", "Hub status filter pills + counts", "Needs-input badge CSS", "status icon spin animation"]
  patterns: ["Status is a pure function of (State, ExitCode); renderers only ever use the 3-value union, never the raw wire string"]
key-files:
  created: []
  modified:
    - internal/daemon/engine.go
    - internal/daemon/types.go
    - internal/daemon/api.go
    - app.go
    - frontend/src/lib/hubStatus.ts
    - frontend/src/lib/hubGroupCounts.ts
    - frontend/src/components/TabBar.tsx
    - frontend/src/components/ExitToast.tsx
    - frontend/src/App.tsx
    - frontend/src/components/Hub/SessionCard.tsx
    - frontend/src/components/Hub/HubFilterBar.tsx
    - frontend/src/components/Hub/HubPanel.tsx
    - frontend/src/components/Hub/HubEmptyState.tsx
    - frontend/src/components/Hub/HubModal.tsx
    - frontend/src/components/QuitConfirmModal.tsx
    - frontend/src/components/Sidebar.tsx
    - frontend/src/style.css
    - TESTING.md
  deleted:
    - internal/status/detector.go
    - internal/status/detector_test.go
decisions:
  - "Killed sessions read as exited: deriveStatus returns exited when stopped with no exit code, and App.KillSession now emits session:status 'exited' (was 'errored')"
  - "GroupCounts drops the waiting field entirely (Sidebar/App types follow) rather than keeping a always-zero count"
  - "Webserver session resolver uses s.Status from ListSessions instead of a second GetSessionStatus lookup"
metrics:
  duration: 14m
  completed: 2026-11-14
---

# Phase 1 Plan 06: Running / Exited / Errored Status Model Summary

The daemon now derives status from process state alone (running, exited on code 0, errored on non-zero). The tabs, the exit toast and the Hub render that three-state model with colorblind-safe shapes. Unknown values sent by older daemons or peers fall back to Running. The agent heuristics package and the waiting/idle UI are gone.

## What changed

**Task 1 (tracer), fe93388d.** Status is derived in the daemon and reaches the tab dot and the exit toast.
- `git rm internal/status/`. The engine loses `statusMu`/`sessionStatuses`. The new `deriveStatus(stopped, exitCode *int)` feeds `ListSessions`, and `GetSessionStatus` looks up the session through `ListSessions` (unknown id returns "running").
- `app.go`: removed `displayNameForCLI` (with its OpenCode and `pty.DetectCLI` leftovers), `maybeNotifyWaiting`, `lastWaitingStatus` and the internal/status import. `emitExitEvent` no longer sends `cli` or `finalStatus`. All notification plumbing stays (`notifyOnWaiting`, `sendNotificationFunc`, `requestNotificationAuthFunc`, Get/SetNotifyOnWaiting, settings key).
- `lib/hubStatus.ts`: new `normalizeStatus` and `statusLabel`. TabBar uses them for the dot class, `role="img"`, title and aria-label. `style.css`: `.tab__status--exited` is a hollow circle (#565f89, 2px border, border-box) and `--errored` is a 2px-radius square. The idle and waiting rules are gone.
- ExitToast meta reads "Exited" or "Exited with error". App.tsx drops cli and finalStatus from the payload type and from ExitState. The auto-close branching is unchanged.

**Task 2, c8296637.** The Hub shows the three states.
- `HubStatus` is now `running | exited | errored`. `deriveHubStatus` maps a stopped session to errored when the exit code is non-zero, otherwise exited. A live session goes through the normalizer. `isAttentionStatus` is true only for errored. `hubGroupCounts` has no waiting count.
- SessionCard and HubModal: running shows PlayCircleIcon (no spin). Exited shows StopCircleIcon and the card dims. Errored shows ExclamationCircleIcon labeled "Exited {code}" for a stopped non-zero exit, otherwise "Error".
- HubFilterBar keeps only the search input (aria-label "Search sessions by name or host") and New session. HubPanel no longer has status-filter state, and `filterSessions(sessions, search)` takes no filter argument. HubEmptyState copy follows UI-SPEC, including the "Clear search" CTA. QuitConfirmModal drops the idle and waiting colors.
- CSS: removed the pill rules and tokens, the needs-input badge rules and tokens, and the spin rules and keyframes. The `--hub-attn-*` tokens stay because the errored attention icon still uses them.

## Verification

- `go build ./... && go vet ./... && go test -race -short ./...`: all ok.
- `go list ./... | grep 'internal/status$'`: empty. `git ls-files internal/status`: 0.
- `pnpm -C frontend exec tsc --noEmit`: clean. `pnpm -C frontend test`: 127 files, 2054 tests passed.
- The linux cross-builds (playwright fixture and internal/...) pass. `tests/check-traceability-paths.sh` reports OK with 121 paths. `tests/build-script.test.sh` passes 37 of 37.
- `wails build -platform darwin/arm64 -tags wailsassets` succeeded, and `frontend/src/wailsjs/wailsjs/` was restored. The tree is clean afterwards.
- Both tasks pass every acceptance grep (baseline counts all moved to their targets).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Plan verify `go list ./... | grep internal/status` also matches `internal/statusbar`**
- **Found during:** Task 1 verify
- **Issue:** The check could never pass, because the kept package `internal/statusbar` matches the pattern.
- **Fix:** Verified with the anchored `grep 'internal/status$'` plus `git ls-files internal/status` (prints nothing).
- **Commit:** none (verification only)

**2. [Rule 1 - Bug] Spy-backend shell tests would read "exited"**
- **Found during:** Task 1
- **Issue:** `newShellTestEngine` uses a spy session with no PTY, so `hub.Done` fires at once and the session is stopped. With derived status, `TestListSessions_ShellStatusRunning` would read "exited".
- **Fix:** That test now uses a real `/bin/cat` PTY. `TestCreateSession_ShellSkipsStatusWatch` was deleted, because its premise (no watcher populating the map) no longer exists.
- **Files:** internal/daemon/engine_test.go
- **Commit:** fe93388d

**3. [Rule 3 - Blocking] GroupCounts shape change reaches files outside the plan's list**
- **Found during:** Task 2
- **Issue:** Removing `waiting` from `GroupCounts` broke the type literals in `Sidebar.tsx` and `Sidebar.test.tsx`. The comment in `HubBriefingModal.tsx` also said "waiting / errored".
- **Fix:** Removed `waiting` from those types and fixtures (Sidebar never read it) and updated the comment.
- **Commit:** c8296637

**4. [Scope note] KillSession status event**
- `App.KillSession` emitted `session:status "errored"`. It now emits `"exited"`, so it matches the plan's rule that killed sessions read as exited. The tab is closed on kill anyway.

**5. [Scope note] SessionCardGrid**
- SessionCardGrid had no status-filter props to remove. Only its tests changed (attention fixtures moved from `waiting` to `errored`).

**6. [Scope note] Dead spin CSS removed**
- Running no longer spins, so `.hub-card__status-icon--spin` (both motion blocks) and `@keyframes hub-spin` became dead. They were removed and the style.hub tests updated.

## Notes for later plans

- **Awaiting-input notifications are absent until plan 01-10** adds bell/OSC 9 notifications. Shell sessions never produced a waiting status, so nothing is lost for users. The Settings toggle "Notify me when a session is awaiting input" and its plumbing still exist and currently trigger nothing.
- README.md still describes the waiting/idle dots, the filter pills and awaiting-input alerts (lines ~23, 53, 79, 128). It was not in this plan's files; a docs plan should rewrite it.
- `.hub__group-sidebar-item__attn-badge*` CSS appears unused by any TSX, but it is attention-related rather than waiting-related, so it was left for the Phase 2 Hub deletion.
- The search still matches `cli` (the shell path) as well as name and host, even though the aria-label now says "name or host". The plan did not ask to change matching.

## Human check (deferred to end-of-phase verify)

Task 2's `<human-check>`: in the built app, open three sessions. In one, turn auto-close off and run `exit`. In another, run `exit 3`. Leave the third running. The tabs should show a filled circle, a hollow circle and a rounded square. The Hub should read Running, Exited (dimmed) and Exited 3. There should be no pills. An unmatched search should show "No matching sessions" with a working "Clear search". This needs the native webview and the user's eyes (shape legibility for a colorblind user).

## Known Stubs

None.

## Threat Flags

None. T-01-21 is mitigated: class names come only from the three-value union, and vitest covers `bogus`, `""`, `idle` and `waiting`. T-01-22 is mitigated: pollSessionStatus still emits session:status before session:exit, and the App.shellExit tests pass.

## Self-Check: PASSED
- FOUND: internal/daemon/engine.go deriveStatus; frontend/src/lib/hubStatus.ts normalizeStatus
- MISSING (intended): internal/status/
- FOUND commits: fe93388d, c8296637 (ancestors of HEAD)
