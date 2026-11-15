---
phase: 01-plain-macos-terminal
plan: 11
subsystem: daemon spawn and settings / app bindings / native notifications / frontend New Session and Settings
tags: [strip-down, fixed-zsh, settings-removal, notifications, settings-migration]
status: complete
requires:
  - phase: 01-10
    provides: settings_removed_test.go negative-route harness; legacy AgentHub v4.2 settings test; close-on-exit state
provides:
  - "Every session spawns the compile-time constant /bin/zsh with argv exactly [-l]; $SHELL and any legacy settings shellPath are ignored"
  - "New Session dialog is header, Working Directory picker and footer only"
  - "Settings → Paths is gone end to end: engine fields/keys, stale-path filter, resolver, shell-path and cli-paths routes, types, client methods, Wails bindings (incl. OpenFileDialog), CheckHealthWithCustomPath, cmdSettings cli-paths block, jump-bar link, search entry, CSS"
  - "The awaiting-input notification toggle is gone end to end: engine field/key, routes, client methods, app cache and bindings, proactive authorization (requestNotificationAuthorization / onNotificationAuthResult), permission hint and event subscription"
  - "sendNotification stays as the one native path; QuitGUIOnly calls it through a.sendNotificationFunc (Phase 6 reuses it)"
  - "Legacy cliPaths, shellPath and notifyOnWaiting keys load harmlessly and drop on the next save; their routes return 404/405"
affects: [01-12, end-of-phase verify, Phase 6 update notices]
actuals:
  tokens: 57000
  tasks: 3
  commits: 3
plan_head_before: a0bdb3b9d18797fd484f1ff7b44c2bb92ad47d32
plan_head_after: 4782ac1ccaeaacff8e35db828d4fd8839aadb37e
tech-stack:
  added: []
  removed: ["engine shellPath/cliPaths/notifyOnWaiting fields and settings keys", "knownShells stale-path filter and resolveDefaultShellPath/GetShellPath/SetShellPath", "UpdateCLIPath/GetCLIPaths (engine, client, app)", "/settings/shell-path, /settings/cli-paths, /settings/notify-on-waiting routes", "App.OpenFileDialog, Get/SetShellPath, Get/SetNotifyOnWaiting bindings", "webserver.CheckHealthWithCustomPath", "requestNotificationAuth / onNotificationAuthResult / requestNotificationAuthorization()", "Settings → Paths section, awaiting-input toggle, permission-denied hint"]
  patterns: ["Fixed spawn constant (sessionShell) instead of a resolver", "Kept native path behind a function seam (sendNotificationFunc) with a production reader"]
key-files:
  created: []
  modified:
    - internal/daemon/engine.go
    - internal/daemon/api.go
    - internal/daemon/client.go
    - internal/daemon/types.go
    - internal/daemon/plugin_settings.go
    - internal/daemon/settings_removed_test.go
    - internal/daemon/engine_test.go
    - internal/daemon/engine_settings_test.go
    - internal/daemon/engine_migration_test.go
    - internal/daemon/api_test.go
    - internal/webserver/tailscale.go
    - app.go
    - app_test.go
    - cmd_cli.go
    - cmd_cli_test.go
    - notification_darwin.go
    - notification_darwin_test.go
    - tray_objc_darwin.m
    - frontend/src/components/NewSessionModal.tsx
    - frontend/src/components/SettingsTab.tsx
    - frontend/src/components/SettingsJumpBar.tsx
    - frontend/src/components/SettingsSearch.tsx
    - frontend/src/wailsjs/go/main/App.d.ts
    - frontend/src/wailsjs/go/main/App.js
    - frontend/src/content/help/getting-started.md
    - frontend/src/style.css
    - TESTING.md
  deleted:
    - frontend/src/components/__tests__/NewSessionModal.shellRow.test.tsx
    - frontend/src/components/__tests__/SettingsTab.shellPath.test.tsx
    - frontend/src/components/__tests__/SettingsTab.notify-toggle.test.tsx
    - frontend/src/components/__tests__/SettingsTab.notify-permission-hint.test.tsx
    - internal/daemon/engine_notify_test.go
    - internal/daemon/api_notify_test.go
key-decisions:
  - "The spawn binary is an unexported constant sessionShell = \"/bin/zsh\"; SetSpawnCommandForTest stays as the only override"
  - "loadSettingsFromDisk now rewrites settings.json only on an old schemaVersion (the stale-path filter was the only other writer)"
  - "TestSettingsPersistence (CLI path persistence) was deleted with the feature; TestSettingsFilePermissions now writes via SetStartMinimized"
  - "SET-03 save-confirmation source tests now target PluginsSection.tsx, the only Save button left in Settings"
  - "Stale NTF traceability rows for TestMaybeNotifyWaiting*/TestDisplayNameForCLI (functions already gone before this plan) were dropped with the toggle rows"
patterns-established:
  - "Removed settings keep a 404/405 negative-route test in settings_removed_test.go and a dropped-on-save assertion in TestSettingsMigration_LegacyAgentHubV42"
requirements-completed: [TERM-01, STRP-01, STRP-04]
coverage:
  - id: D1
    description: "Spawn is always /bin/zsh [-l], whatever $SHELL or a legacy shellPath says; the real shell is a zsh login shell"
    requirement: TERM-01
    verification:
      - kind: integration
        ref: "internal/daemon/engine_test.go#TestCreateSession_LoginShellArgv, TestCreateSession_IgnoresShellEnvAndLegacySetting, TestCreateSession_LoginShell"
  - id: D2
    description: "Removed shell-path, cli-paths and notify-on-waiting routes return 404/405"
    requirement: STRP-04
    verification:
      - kind: integration
        ref: "internal/daemon/settings_removed_test.go#TestSettingsRemoved_ShellPathRoutes, TestSettingsRemoved_CLIPathsRoutes, TestSettingsRemoved_NotifyOnWaitingRoutes"
  - id: D3
    description: "Legacy AgentHub settings load; cliPaths/shellPath/notifyOnWaiting drop on save while kept keys survive"
    requirement: STRP-04
    verification:
      - kind: unit
        ref: "internal/daemon/engine_migration_test.go#TestSettingsMigration_LegacyAgentHubV42, TestSettingsMigrationV3_1ToV3_2"
  - id: D4
    description: "New Session shows only the directory picker; Settings has no Paths section and Behavior has one toggle"
    requirement: STRP-01
    verification:
      - kind: unit
        ref: "frontend/src/components/__tests__/NewSessionModal.test.tsx, SettingsTab.test.tsx, SettingsTab.hyperlinked-index.test.tsx, SettingsTab.persistence.test.tsx"
  - id: D5
    description: "sendNotification bundle guard still holds"
    requirement: STRP-01
    verification:
      - kind: unit
        ref: "notification_darwin_test.go#TestHasAppBundleID_FalseWhenUnbundled, TestSendNotification_NoBundleReturnsCleanly"
metrics:
  duration: 12min
  completed: 2026-11-15
---

# Phase 1 Plan 11: Fixed zsh, no Settings → Paths, no awaiting-input toggle Summary

Every session now spawns the constant `/bin/zsh -l`; the shell-path, CLI-path and notify-on-waiting settings are removed from daemon, client, Wails bindings, CLI and UI; legacy keys load and drop on save; `sendNotification` is the single native notification path, reached from QuitGUIOnly through `a.sendNotificationFunc`.

## Tasks

| Task | Name | Commit | Key files |
|------|------|--------|-----------|
| 1 | Every new session is /bin/zsh -l; New Session shows only the directory picker (tracer) | 744a6efd | engine.go, engine_test.go, NewSessionModal.tsx, style.css, TESTING.md |
| 2 | Settings → Paths and all shellPath / cliPaths / OpenFileDialog plumbing removed | 6b7cf0c6 | engine.go, api.go, client.go, types.go, app.go, tailscale.go, cmd_cli.go, SettingsTab.tsx, App.d.ts/js |
| 3 | Awaiting-input toggle removed; sendNotification kept | 4782ac1c | app.go, notification_darwin.go, tray_objc_darwin.m, engine.go, api.go, client.go, SettingsTab.tsx |

Tracer gate (Task 1): human_verify_mode is end-of-phase and `<verify>` is automated-only; it was run and passed before Task 2 started.

## Verification

- `go build ./... && go vet ./... && go test -race -short ./...`: pass
- `go test -race -run 'SettingsRemoved|LegacyAgentHubV42' -v ./internal/daemon/`: all PASS (auto-close, shell-path, cli-paths, notify-on-waiting)
- `go test -race -run 'TestCreateSession_LoginShell|TestCreateSession_IgnoresShellEnvAndLegacySetting' -v ./internal/daemon/`: PASS (real zsh prints LOGIN_OK_42 with SHELL=/bin/bash)
- `go test -race -run 'TestHasAppBundleID|TestSendNotification' -v .`: PASS
- Linux cross-builds (playwright fixture, `./internal/...`): pass
- `pnpm -C frontend exec tsc --noEmit`: clean; `pnpm -C frontend test`: 116 files, 1828 tests pass (SM-02 did not flake this run)
- `bash tests/build-script.test.sh`: 37 passed; `bash tests/check-traceability-paths.sh`: OK (118 paths)
- `wails build -platform darwin/arm64 -tags wailsassets`: built; generated App.d.ts has no GetShellPath, GetCLIPaths, OpenFileDialog or NotifyOnWaiting; `frontend/src/wailsjs/wailsjs/` restored afterwards
- All acceptance-criteria grep counts hit their targets (Go non-test references 0, frontend non-test references 0, `settings-panel__table` 0, `save-paths-row` 1, tray ObjC references 0, `func sendNotification` 1, `a.sendNotificationFunc(` 1)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Tests that only made sense with the removed features were rewritten, not just trimmed**
- **Found during:** Task 2
- **Issue:** `TestSettingsPersistence` existed only to persist a CLI path; `TestSettingsFilePermissions` and `TestSettingsLoadMissingFile` used `UpdateCLIPath`/`cliPaths`; `TestSettingsMigrationV3_1ToV3_2` asserted `cliPaths` survived; the SET-03 save-confirmation tests read `setSaved`/`Saved!` from SettingsTab, which no longer has a Save button; two CSS gate tests would have passed vacuously.
- **Fix:** Deleted `TestSettingsPersistence`; permissions test writes via `SetStartMinimized`; missing-file test checks the startMinimized default; v3.1 migration now asserts `cliPaths` is dropped on the upgrade re-write; SET-03 tests target PluginsSection.tsx; CSS gates now assert the removed rules are absent.
- **Files modified:** internal/daemon/engine_settings_test.go, engine_migration_test.go, frontend/src/components/__tests__/SettingsTab.persistence.test.tsx, style.contrast.test.ts, style.settings.test.ts
- **Commit:** 6b7cf0c6

**2. [Rule 1 - Bug] Stale comments pointing at removed code**
- **Found during:** Tasks 2 and 3
- **Issue:** Comments in app.go (SaveTerminalSession, cancel-path), App.tsx, app_save_terminal_test.go, tray_objc_darwin.m, and three SettingsTab test files referred to OpenFileDialog, GetCLIPaths, TestUpdateCLIPath, handleToggleNotifyOnWaiting or the permission subscription.
- **Fix:** Reworded each one. No behavior change.
- **Commits:** 6b7cf0c6, 4782ac1c

**3. [Rule 1 - Bug] Orphaned NTF traceability rows**
- **Found during:** Task 3
- **Issue:** TESTING.md NTF rows cited `TestMaybeNotifyWaiting*` and `TestDisplayNameForCLI`, which were already gone before this plan.
- **Fix:** Dropped them with the toggle rows. The `notification_darwin_test.go` bundle-guard row stays. Recorded in the Section 2 note.
- **Commit:** 4782ac1c

Out of scope, not touched: `tray.go`, `internal/daemon/client_test.go` and `internal/relay/scrollback_altscreen_test.go` were already not gofmt-clean at HEAD.

## Known Stubs

None.

## Human-check items for the end-of-phase verify

1. **M-54 (TERM-01):** the New Session dialog shows only the Working Directory picker; a long path ellipsizes inside the 460px modal; a new session answers `[[ -o login ]] && echo login; echo $ZSH_VERSION` with `login` and a zsh version even when the account login shell is bash. Quit any installed AgentHub and stop its daemon first.
2. **M-41 (kept notification path):** on a Developer-ID-signed build, Settings → Behavior shows only "Start minimized to system tray"; "Quit GUI only" posts one "AgentHub is still running in the background. N session(s) active." notification and the tray icon stays.
3. **Settings visual check:** no Paths section, no Paths jump-bar link, and settings search for "path" or "notify" finds nothing from the removed rows.

## State notes

`state.advance-plan` declined (`reason: plans_outstanding`, 01-12-PLAN.md has no SUMMARY yet), so STATE.md's plan counter was left as is, per the orchestrator's instruction not to hand-edit it. `roadmap.update-plan-progress` recorded 11/12 summaries; metrics, decisions, session and TERM-01 completion were recorded (STRP-01 and STRP-04 were already marked complete).

## Self-Check: PASSED
