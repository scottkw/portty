---
phase: 01-plain-macos-terminal
plan: 10
subsystem: app shell exit detection / frontend tabs, settings, Hub / daemon settings
tags: [strip-down, exit-watcher, status-ui-removal, settings-migration, hub]
status: complete
requires:
  - phase: 01-09
    provides: chat inject removed; Phase 1 strip-down state that this plan builds on
provides:
  - "One app-wide exit watcher in app.go (startExitWatcher/checkExits, 500 ms, no deadline) emits session:exit {sessionId} once per stopped session, for every session regardless of age or how it was created"
  - "session:exit always closes the tab; handleCloseTab calls KillSession unconditionally, so tab-less (Hub-only, CLI-created) sessions are removed too"
  - "init and retryInit restore only non-stopped sessions; sessions already stopped at restore are passed to KillSession"
  - "No exit toast, countdown banner, Keep Open, tab status dot, tab countdown, quit-dialog status, or session:status feed in the GUI"
  - "Auto-close setting removed end to end (Settings toggle, bindings, app.go, engine field/key, api routes, client methods); legacy autoCloseSession loads and is dropped on save; /settings/auto-close-session returns 404/405"
  - "Hub and sidebar show no status: hubStatus helpers and HubBriefingModal deleted; no card status icon/label, exit chip, dim or attention UI; no attention sort/FLIP; group counts are { total } only"
affects: [01-11, 01-12, end-of-phase verify]
actuals:
  tokens: 68000
  tasks: 3
  commits: 3
plan_head_before: f41a78ebc65d8be6f1c286d4e2d5b086069944ac
plan_head_after: 5c54c337a55f73763d058f4144d539bd7a284872
tech-stack:
  added: []
  removed: ["app.go pollSessionStatus / shouldContinuePolling / emitExitEvent (300 s per-session poll)", "app.go GetSessionStatus, Get/SetAutoCloseSession bindings and KillSession's session:status emit", "daemon autoCloseSession engine field, settings key, getters/setters, /settings/auto-close-session routes and client methods", "ExitToast, ExitCountdownBanner, frontend/src/lib/hubStatus.ts, HubBriefingModal", "SessionCardGrid sortSessionsForDisplay + useFLIPAnimation, HubPanel useDebouncedValue + attention set", "status/exit/attention/briefing CSS and --hub-attn-* / --hub-dim-opacity / --hub-card-dim-bg tokens"]
  patterns: ["App.emitEvent seam (emitEventFunc test injection, otherwise runtime.EventsEmit behind the frontend guard)", "One app-wide poller with an emitted-set dedupe and prune instead of one goroutine per created session", "Restore filter: the GUI removes sessions that stopped while no GUI was watching"]
key-files:
  created:
    - app_exit_test.go
    - internal/daemon/settings_removed_test.go
  modified:
    - app.go
    - app_test.go
    - frontend/src/App.tsx
    - frontend/src/components/TabBar.tsx
    - frontend/src/components/QuitConfirmModal.tsx
    - frontend/src/components/SettingsTab.tsx
    - frontend/src/components/SettingsSearch.tsx
    - frontend/src/components/Sidebar.tsx
    - frontend/src/components/Hub/SessionCard.tsx
    - frontend/src/components/Hub/SessionCardGrid.tsx
    - frontend/src/components/Hub/HubPanel.tsx
    - frontend/src/components/Hub/HubModal.tsx
    - frontend/src/lib/hubGroupCounts.ts
    - frontend/src/style.css
    - frontend/src/wailsjs/go/main/App.d.ts
    - frontend/src/wailsjs/go/main/App.js
    - frontend/src/content/help/getting-started.md
    - internal/daemon/engine.go
    - internal/daemon/api.go
    - internal/daemon/client.go
    - internal/daemon/engine_migration_test.go
    - TESTING.md
  deleted:
    - app_poll_test.go
    - frontend/src/components/__tests__/App.exit.test.tsx
    - frontend/src/components/ExitToast.tsx
    - frontend/src/components/ExitCountdownBanner.tsx
    - frontend/src/components/__tests__/ExitToast.test.tsx
    - frontend/src/components/__tests__/ExitCountdownBanner.test.tsx
    - frontend/src/lib/hubStatus.ts
    - frontend/src/lib/hubStatus.test.ts
    - frontend/src/components/Hub/HubBriefingModal.tsx
    - frontend/src/components/Hub/HubBriefingModal.test.tsx
key-decisions:
  - "Exit detection is one app-wide 500 ms watcher with no deadline and no cold-start baseline; a session already stopped at first sight is reported so the GUI removes it"
  - "SessionCardGrid's sort and FLIP reorder animation were deleted with the attention key: attention was the only sort key, so the grid now keeps arrival order"
  - "CSS used only by the deleted HubBriefingModal (tail, respond, send/close buttons, error banner) and the dim tokens used only by .hub-card--dim were deleted as orphans"
  - "Daemon-side derived status and its GetSessionStatus route stay (CLI, web list, peers); only the GUI binding and display are gone"
patterns-established:
  - "Exit/close cleanup never depends on a tab existing: handleCloseTab kills first, then edits tabs"
requirements-completed: [TERM-05, TERM-02, STRP-01, STRP-04]
coverage:
  - id: D1
    description: "Every shell exit (any code, any age) produces exactly one session:exit; the watcher prunes removed sessions"
    requirement: TERM-05
    verification:
      - kind: integration
        ref: "app_exit_test.go#TestCheckExits_EmitsOnceWhenShellExits, TestCheckExits_LongLivedSessionStillReported, TestCheckExits_StoppedAtFirstSight, TestCheckExits_PrunesRemovedSessions"
        status: pass
    human_judgment: false
  - id: D2
    description: "A relaunched GUI on the same daemon keeps running sessions with scrollback and emits no exit for them"
    requirement: TERM-02
    verification:
      - kind: integration
        ref: "app_exit_test.go#TestExitWatcher_GUIRelaunchKeepsRunningSessions"
        status: pass
    human_judgment: false
  - id: D3
    description: "session:exit closes the tab and removes the session (also tab-less sessions); restore removes stopped sessions"
    requirement: TERM-05
    verification:
      - kind: unit
        ref: "frontend/src/components/__tests__/App.shellExit.test.tsx"
        status: pass
    human_judgment: false
  - id: D4
    description: "Auto-close setting gone; legacy key loads and is dropped on save; routes return 404/405"
    requirement: STRP-04
    verification:
      - kind: unit
        ref: "internal/daemon/settings_removed_test.go#TestSettingsRemoved_AutoCloseSessionRoutes"
        status: pass
      - kind: unit
        ref: "internal/daemon/engine_migration_test.go#TestSettingsMigration_LegacyAgentHubV42"
        status: pass
      - kind: unit
        ref: "frontend/src/components/__tests__/SettingsTab.test.tsx#TERM-05: no auto-close toggle"
        status: pass
    human_judgment: false
  - id: D5
    description: "No session status on tabs, quit dialog, Hub cards, Hub modal or sidebar (unit level)"
    requirement: STRP-01
    verification:
      - kind: unit
        ref: "frontend/src/components/__tests__/TabBar.test.tsx#TabBar shows no session status"
        status: pass
      - kind: unit
        ref: "frontend/src/components/Hub/SessionCard.test.tsx#a stopped session with a non-zero exit code renders no status text"
        status: pass
      - kind: unit
        ref: "frontend/src/components/Hub/HubModal.test.tsx#TERM-05: a stopped session with a non-zero exit code opens the interactive modal"
        status: pass
      - kind: unit
        ref: "frontend/src/components/__tests__/QuitConfirmModal.test.tsx, Sidebar.test.tsx, hubGroupCounts.test.ts, style.hub*.test.ts"
        status: pass
    human_judgment: false
  - id: D6
    description: "Built app: tabs close on exit (including a shell older than 5 minutes) with no toast/countdown/status, Settings has no toggle, quit dialog shows names only (M-55)"
    requirement: TERM-05
    verification: []
    human_judgment: true
    rationale: "Needs the native webview, a live daemon and real wall-clock time past 5 minutes (TESTING.md M-55)"
  - id: D7
    description: "Hide to tray / Finder relaunch keeps running sessions with scrollback; a session that exited while hidden is gone (M-56)"
    requirement: TERM-02
    verification: []
    human_judgment: true
    rationale: "Needs the tray, a Finder relaunch and a live daemon (TESTING.md M-56)"
duration: 19min
completed: 2026-11-15
---

# Phase 1 Plan 10: Close tabs on every shell exit, remove all status and exit UI Summary

**One app-wide 500 ms exit watcher (no deadline) closes the tab and removes the session on every shell exit; the exit toast, countdown, Keep Open, auto-close setting, tab dots, quit-dialog status, Hub status/attention UI and the briefing modal are deleted.**

## Performance

- **Duration:** 19 min
- **Started:** 2026-11-14T23:58:48Z
- **Completed:** 2026-11-15T00:18:13Z
- **Tasks:** 3
- **Files modified:** 60 (600 insertions, 3643 deletions)

## Accomplishments

- `app.go`: `startExitWatcher(ctx)` (started in `startup` right after the client exists, and in `RetryDaemon`, guarded by `sync.Once`) runs `checkExits` at once and every 500 ms. `checkExits` emits `session:exit {sessionId}` once per stopped session via the new `emitEvent` seam, and prunes ids the daemon no longer lists. The 300 s per-session `pollSessionStatus` path is gone.
- `App.tsx`: the `session:exit` handler always calls `handleCloseTabRef.current?.(data.sessionId)`. `init` and `retryInit` restore only sessions whose state is not `stopped` and pass stopped ones to `KillSession`. All exit, countdown, auto-close and session-status state is gone.
- The auto-close setting is removed end to end. The legacy `autoCloseSession` key loads and is dropped on the next save, and the routes return 404/405.
- Hub: no status icon, label, exit chip, dim, attention border/icon/badge, attention sort or briefing modal. The sidebar group aria-label is `${label} group, N sessions`.
- OSC 9;4 progress still drives the tab underline (asserted in TabBar.test.tsx). The tray glyph code was not touched.

## Task Commits

1. **Task 1: App-wide exit watcher closes the tab on every shell exit** - `3976cefe` (feat)
2. **Task 2: Remove exit toast, countdown, auto-close toggle and status dots** - `c5d2361b` (feat)
3. **Task 3: Hub and sidebar show no session status** - `5c54c337` (feat)

## Decisions Made

See `key-decisions` in the frontmatter. The main judgment call: `SessionCardGrid` sorted only by attention, so the sort memo, debounced sort key and FLIP reorder animation were deleted with it rather than kept as an identity sort.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing coverage] Plan-checker advisory: tab-less session cleanup asserted**
- **Found during:** Task 1
- **Issue:** The exit cleanup for sessions with no open tab (Hub-only, CLI-created) relied on an untested side effect of `handleCloseTab`.
- **Fix:** App.shellExit.test.tsx asserts that `handleCloseTab` calls `await KillSession(id)` before `setTabs(`, with no `tabs` reference and no early `return` before the kill.
- **Files modified:** frontend/src/components/__tests__/App.shellExit.test.tsx
- **Commit:** 3976cefe

**2. [Rule 1 - Cleanup of orphans this plan created] Briefing-only CSS and dim tokens deleted**
- **Found during:** Task 3
- **Issue:** Deleting HubBriefingModal and `.hub-card--dim` left unreferenced rules: `.hub-modal__tail*`, `.hub-modal__respond*`, `.hub-modal__send-btn`, `.hub-modal__close-btn`, `.hub-modal__error-banner` (plus their focus-visible entries) and the `--hub-dim-opacity` / `--hub-card-dim-bg` tokens. The plan did not list them.
- **Fix:** Deleted them (they existed only for the deleted code, so the Chesterton check is met). The style.hub*.test.ts pins now assert they are absent.
- **Files modified:** frontend/src/style.css, style.hub.test.ts, style.hub.modal.test.ts
- **Commit:** 5c54c337

**3. [Rule 1 - Bug, self-inflicted] An over-broad CSS cut was caught and reverted before commit**
- **Found during:** Task 3
- **Issue:** One cut (from the status-icon block to `/* --- Uptime / duration --- */`) also removed `.hub-card__name`, `.hub-card__conn*`, `.hub-card__menu-item--destructive`, `.hub-card__menu-item-sub` and `.hub-card__viewers*`. style.hub.test.ts caught it.
- **Fix:** Restored that span from HEAD. The final `git diff` shows only intended selectors removed.
- **Commit:** 5c54c337 (never committed broken)

**4. [Rule 3 - Blocking] Extra test and comment references updated**
- Removed `autoCloseSession` resets in `internal/daemon/api_test.go` and `engine_test.go`, the `GetAutoCloseSession` mocks in 7 SettingsTab test files, and stale comments that named deleted code (TerminalChatHost.tsx, vtColor.ts, App.createTab.stayOnHub.test.tsx, App.shellWebShare.test.tsx). The HubPanel group-count effect now keys on `id + memberKey` instead of `status`, because counts no longer depend on status.

**Total deviations:** 4 auto-fixed. **Impact:** no scope change beyond deleting code this plan orphaned. `themeTokens.test.ts` needed no edit (it pinned no attention tokens).

## Issues Encountered

- `internal/daemon/client_test.go` is not gofmt-clean on HEAD. It was already that way and is not touched here.
- `wails build` regenerated `frontend/src/wailsjs/wailsjs/`. Restored with `git checkout`.

## Human-check items for the end-of-phase verify

- **M-55 (TERM-05):** in the built app, `exit`, `exit 3` and `sleep 400; exit 0` (wait about 7 min). Each tab closes immediately with no toast, countdown, Keep Open or error tab, and tabs show no dot. The quit dialog lists names only. Settings has no auto-close toggle.
- **M-56 (TERM-02):** run `sleep 20; exit` in one session and print output in another. Quit GUI only, wait 30 s, reopen from the tray, then relaunch from Finder. The running session is present with its scrollback, and the exited one is gone.
- **M-50:** rewritten as the standing watch for the new watcher.

## Verification

- `go build ./... && go vet ./... && go test -race -short ./...`: pass
- `go test -race -run 'CheckExits|ExitWatcher' -v .`: all 5 PASS
- `go test -race -run 'SettingsRemoved|LegacyAgentHubV42' -v ./internal/daemon/`: PASS
- Linux cross-builds (playwright-fixture, internal/...): pass
- `pnpm -C frontend exec tsc --noEmit`: pass. `pnpm -C frontend test`: 120 files, 1884 tests pass (SM-02 did not flake)
- `bash tests/build-script.test.sh`: 37 passed. `bash tests/check-traceability-paths.sh`: OK, 124 paths
- `wails build -platform darwin/arm64 -tags wailsassets`: built
- Every acceptance-criteria grep in Tasks 1-3: pass (counts 0, or 1 where expected, M-55/M-56 = 2)

## Known Stubs

None.

## Next Phase Readiness

Ready for 01-11 (it removes the awaiting-input notification toggle and the remaining Settings plumbing).

## Self-Check: PASSED
