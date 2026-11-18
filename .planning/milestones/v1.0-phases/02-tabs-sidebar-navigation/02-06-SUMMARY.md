---
phase: 02-tabs-sidebar-navigation
plan: 06
subsystem: settings
status: complete
tags: [nav-03, hub-removal, settings, daemon, wails-bindings, testing]
requires: ["02-05"]
provides:
  - "No stay-on-Hub preference in any layer; createTab always activates the new tab"
  - "Legacy settings.json key stayOnHubAfterCreate ignored on load and dropped on save"
  - "No GetSessionStyledTailLines Wails binding (daemon route and engine method kept)"
affects: ["02-07", "02-08", "02-12"]
actuals:
  tokens: 19200
  tasks: 2
  commits: 2
commits: 2
plan_head_before: ea2d44bd43f00058c6af35d1eacccb4b36bb3b74
plan_head_after: 3186266567544ade72fa1fa63d4beab32fb033ab
tech-stack:
  added: []
  patterns:
    - "Removed setting = delete daemonSettings field + routes + client + binding; migration test lists the key in its dropped-on-save loop; settings_removed_test.go proves the routes 404/405 next to a kept sibling at 200"
key-files:
  created:
    - frontend/src/components/__tests__/App.createTab.sessionsSeed.test.tsx (renamed from App.createTab.hubSessionsSeed.test.tsx)
  modified:
    - app.go
    - internal/daemon/api.go
    - internal/daemon/engine.go
    - internal/daemon/client.go
    - internal/daemon/plugin_settings.go
    - internal/daemon/engine_migration_test.go
    - internal/daemon/settings_removed_test.go
    - internal/relay/hub.go
    - frontend/src/App.tsx
    - frontend/src/components/SettingsTab.tsx
    - frontend/src/wailsjs/go/main/App.d.ts
    - frontend/src/wailsjs/go/main/App.js
    - frontend/src/wailsjs/go/models.ts
    - frontend/src/components/__tests__/SettingsTab.test.tsx
    - frontend/src/components/__tests__/SettingsTab.shell-warn-toggle.test.tsx
    - frontend/src/components/__tests__/SettingsTab.tailscale-status.test.tsx
    - frontend/src/components/__tests__/StatusBar.shareSession.test.tsx
    - TESTING.md
key-decisions:
  - "The StyledSpan TS interface was removed from models.ts: after the binding went, git grep found no other frontend reference."
  - "DaemonClient.GetSessionStyledTailLines was kept with the route and engine method, although app.go was its only caller. It is part of the daemon-side styled-tail surface recorded as a later cut candidate."
  - "Two internal/relay/hub.go comments were reworded to name the daemon styled-tail renderer instead of the method name. They stay accurate, and the plan's git grep gate passes."
requirements-completed: [NAV-03]
coverage:
  - id: D1
    description: "Creating a session always activates its tab; App.tsx reads no stay-on-Hub preference"
    requirement: NAV-03
    verification:
      - kind: unit
        ref: "frontend/src/components/__tests__/App.createTab.sessionsSeed.test.tsx#createTab always activates the new tab and reads no stay-on-Hub preference (NAV-03)"
        status: pass
  - id: D2
    description: "Settings shows no stay-on-Hub toggle"
    requirement: NAV-03
    verification:
      - kind: unit
        ref: "frontend/src/components/__tests__/SettingsTab.test.tsx#renders no \"Stay on Hub\" text and has no stay-on-Hub bindings"
        status: pass
  - id: D3
    description: "Legacy settings.json with stayOnHubAfterCreate loads; key dropped on save; shellWebShareWarned and plugins survive"
    requirement: NAV-03
    verification:
      - kind: unit
        ref: "internal/daemon/engine_migration_test.go#TestSettingsMigration_LegacyAgentHubV42"
        status: pass
  - id: D4
    description: "GET/PATCH /settings/stay-on-hub-after-create gone (404/405); GET /settings/start-minimized still 200"
    requirement: NAV-03
    verification:
      - kind: unit
        ref: "internal/daemon/settings_removed_test.go#TestSettingsRemoved_StayOnHubAfterCreateRoutes"
        status: pass
metrics:
  duration: "15 min"
  completed: 2026-11-15
---

# Phase 2 Plan 06: Stay-on-Hub Preference Removal Summary

The "Stay on Hub after creating a session" preference is gone from Settings, App.tsx, the Wails bindings, the daemon client, the API and the engine, so a new session always opens as the active tab. An old settings.json still loads and loses `stayOnHubAfterCreate` on the next save. The Hub-only `GetSessionStyledTailLines` Wails binding is removed too.

## Tasks

| Task | Name | Commit |
|------|------|--------|
| 1 | Remove stay-on-Hub end to end; createTab always activates; migration test flipped | 63408439 |
| 2 | Removed-route test, styled-tail binding and StyledSpan removed, stale mocks cleaned, TESTING.md | 31862665 |

## Deleted files

- `internal/daemon/engine_stayonhub_test.go` (covered only the preference. Generic no-rewrite-at-current-schema behaviour is still covered by `TestSettingsMigrationIdempotent`)
- `internal/daemon/api_stayonhub_test.go`
- `frontend/src/components/__tests__/App.createTab.stayOnHub.test.tsx`
- `frontend/src/components/__tests__/SettingsTab.stay-on-hub-toggle.test.tsx`

## Renamed files

- `frontend/src/components/__tests__/App.createTab.hubSessionsSeed.test.tsx` → `App.createTab.sessionsSeed.test.tsx`. The header was rewritten, `hubSessions` wording became `sessions`, and a new always-activate case was added.

## Deleted symbols

- Go: `App.GetStayOnHubAfterCreate`, `App.SetStayOnHubAfterCreate`, `App.GetSessionStyledTailLines`, `DaemonClient.Get/SetStayOnHubAfterCreate`, `SessionEngine.Get/SetStayOnHubAfterCreate`, the `stayOnHubAfterCreate` engine field, `daemonSettings.StayOnHubAfterCreate`, `API.handleGet/SetStayOnHubAfterCreate` and their two routes.
- Frontend: `stayOnHubAfterCreateRef` and its init load, the createTab `if` guard, the SettingsTab toggle (state, load effect, handler, markup), the declarations in App.d.ts/App.js for the three bindings, and `daemon.StyledSpan` in models.ts.

## Verification

- `go build ./... && go vet ./...`: ok
- `go test -race -short ./...`: all packages ok
- `--- PASS: TestSettingsMigration_LegacyAgentHubV42`, `--- PASS: TestSettingsRemoved_StayOnHubAfterCreateRoutes`
- `pnpm exec tsc --noEmit`: ok. `pnpm test`: 97 files, 1390 tests passed
- `bash tests/check-traceability-paths.sh`: OK, 93 paths
- `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags playwrightfixture ./cmd/playwright-fixture`: ok
- Plan grep gates: no stay-on-Hub reference in non-test code anywhere (exit 1). `GetSessionStyledTailLines` count across app.go/App.d.ts/App.js is 0. 2 NAV-03 rows for settings_removed/engine_migration in TESTING.md.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Relay comments tripped the Task 2 grep gate**
- **Found during:** Task 2 verify
- **Issue:** `internal/relay/hub.go` (kept relay code) had two comments naming `GetSessionStyledTailLines`, which here means the engine method that stays. The plan's gate only excludes `internal/daemon/*`, so it failed.
- **Fix:** Reworded both comments to "the daemon's styled-tail renderer (internal/daemon/engine.go, GET /sessions/{id}/styled-tail)". The code is unchanged.
- **Files modified:** internal/relay/hub.go
- **Commit:** 31862665

**2. [Scope note] SettingsTab.test.tsx and StatusBar.shareSession.test.tsx had no binding mocks**
- SettingsTab.test.tsx auto-mocks every binding via `importOriginal`, so there was nothing to remove. It got only the new NAV-03 render case. StatusBar.shareSession.test.tsx had only a stale file reference in a comment, now pointed at the renamed seed test.

## Later cut candidates

- Daemon `GET /sessions/{id}/styled-tail` and `/tail` routes, `SessionEngine.GetSessionStyledTailLines`, `DaemonClient.GetSessionStyledTailLines` and the `daemon.StyledSpan` Go type. No GUI consumer remains. They were kept per the plan's flagged assumption (tests, possible CLI use).
- The `App.GetSessionTailLines` Wails binding (plain tail) was not inspected for remaining GUI consumers. It was out of this plan's scope.

## Deferred / out of scope

- `gofmt -l` flags `internal/relay/scrollback_altscreen_test.go` and `internal/daemon/client_test.go`. Both were untouched by this plan, so they are pre-existing and left alone.
- Historical Section 2 notes in TESTING.md (Phase 168 entries) still name the deleted files. They are history, matching how 02-05 handled earlier notes.

## Needs VM check

None required: every behaviour change is pinned by unit tests. Optional light check in a built app: Settings → Session Behavior shows only the shell web-share warning toggle, and New Session switches straight to the new tab. No .app was built for this plan.

## Known Stubs

None.

## Self-Check: PASSED

- FOUND: frontend/src/components/__tests__/App.createTab.sessionsSeed.test.tsx
- FOUND: internal/daemon/settings_removed_test.go (TestSettingsRemoved_StayOnHubAfterCreateRoutes)
- MISSING as intended: App.createTab.hubSessionsSeed.test.tsx, engine_stayonhub_test.go, api_stayonhub_test.go, App.createTab.stayOnHub.test.tsx, SettingsTab.stay-on-hub-toggle.test.tsx
- FOUND commits: 63408439, 31862665 (ancestors of HEAD)
