---
phase: 02-tabs-sidebar-navigation
plan: 13
subsystem: appearance
status: complete
tags: [THEM-01, appearance, NSApp.appearance, settings, daemon, cgo, wails]
requires:
  - 02-11 (Settings sheet)
  - 02-12 (token gate, style.tokens.test.ts)
provides:
  - settings.json "appearance" (system|light|dark) with daemon validation and GET/PATCH /settings/appearance
  - NSApp.appearance applied at startup (before the first visible paint) and live on change
  - Settings ▸ General ▸ Appearance radiogroup (Automatic / Light / Dark)
  - useColorScheme(override); App appearance state (desktop only)
  - TESTING.md M-68 and THEM-01 rows
affects:
  - app.go startup/domReady ordering (startupDone channel)
  - SettingsSheet General pane
tech-stack:
  added: []
  patterns:
    - cgo + separate .m file, dispatch_async to the main queue (as tray_objc_darwin.m)
    - App seam field applyAppearanceFunc (nil = real native call)
key-files:
  created:
    - appearance_darwin.go
    - appearance_darwin.m
    - internal/daemon/appearance_settings_test.go
  modified:
    - internal/daemon/engine.go
    - internal/daemon/api.go
    - internal/daemon/client.go
    - app.go
    - app_test.go
    - frontend/src/wailsjs/go/main/App.d.ts
    - frontend/src/wailsjs/go/main/App.js
    - frontend/src/components/SettingsSheet.tsx
    - frontend/src/components/__tests__/SettingsSheet.test.tsx
    - frontend/src/components/__tests__/SettingsSheet.shell-warn-toggle.test.tsx
    - frontend/src/components/__tests__/SettingsSheet.tailscale-status.test.tsx
    - frontend/src/lib/useColorScheme.ts
    - frontend/src/lib/__tests__/useColorScheme.test.tsx
    - frontend/src/App.tsx
    - frontend/src/components/__tests__/App.test.tsx
    - frontend/src/style.css
    - TESTING.md
decisions:
  - "NSApp.appearance is set with dispatch_async on the main queue, never dispatch_sync: Wails runs OnStartup on a goroutine, and a synchronous hop could deadlock against the main thread"
  - "domReady waits on a startupDone channel so the stored appearance is queued before WindowShow (Wails runs startup and domReady concurrently)"
  - "No data-appearance CSS fallback: M-68 showed WKWebView follows NSApp.appearance live"
  - "The Settings Appearance segments use equal grid columns sized to the widest label, scoped to .settings-appearance (the Share dialog is unchanged)"
metrics:
  duration: ~45 min (17:33 base commit to 18:18 last code commit, including the VM check)
  completed: 2026-11-15
estimate:
  tokens: 90000
  tasks: 2
actuals:
  tokens: 14500
  tasks: 2
  commits: 2
plan_head_before: 095d64231f9b71237cf483777622bf8ff44082dc
plan_head_after: 5e5d9ea858c264714afe96944a9c476e98e2a34c
---

# Phase 2 Plan 13: Appearance Override Summary

Settings ▸ Appearance now offers Automatic / Light / Dark. The choice is stored in settings.json, validated by the daemon, and applied through NSApp.appearance, both at startup (before the window shows) and live. CSS, terminals and sidebar vibrancy follow that one native source, so no CSS fallback was needed. The web share page always follows the browser.

## Tasks

| Task | Name | Commit |
|------|------|--------|
| 1 | Settings ▸ Appearance → daemon stores it → NSApp.appearance flips; a relaunch opens dark | a24453b4 |
| 2 | Terminals follow the override through the hook; no fallback; equal-width segments; TESTING.md M-68; full phase gate | 5e5d9ea8 |

## Task 1 notes

- **Main thread: dispatch_async everywhere.** The plan said "direct if already on the main thread, else dispatch_sync". Task 1 uses `dispatch_async(dispatch_get_main_queue(), …)` unconditionally. Evidence from Wails v2.10.2 (`internal/frontend/desktop/darwin`):
  - `frontend.go:209-213` runs `OnStartup` in `go func() {…}()` while the main thread runs the AppKit loop.
  - `frontend.go:451-453` (`processMessage`) is called from the main thread and pushes into `messageBuffer`, a channel with a buffer of 100 (`frontend.go:42`). The consumer of that channel can wait on startup, so a `dispatch_sync` from the startup goroutine could deadlock.
  - Ordering still holds: `WailsContext.h:20` defines `ON_MAIN_THREAD` as `dispatch_async(dispatch_get_main_queue(), …)`, and `Application.m:225-229` `Show` uses it. The main queue is serial, so an appearance queued before `WindowShow` runs first.
- **startupDone channel fixes a startup/domReady race.** Wails calls `startup` and `domReady` concurrently. `NewApp` makes `startupDone`, `startup` closes it after `EnsureDaemon` plus `applyStoredAppearance()` (or after the daemon error), and `domReady` waits on it before `WindowShow`. A nil channel (in tests) skips the wait.
- **@available guard.** The build's deployment target is 10.13, and `NSApp.appearance` needs 10.14, so the assignment sits inside `if (@available(macOS 10.14, *))`.
- **A missing settings.json defaults to "system"**, as do a missing key and an unknown value (`normalizeAppearance`).
- **PATCH /settings/appearance returns 204** on success (the plan's test text said 200). This matches the other settings PATCH handlers. A bad body or a value outside the enum returns 400, and the stored value is unchanged.
- **Two more test files needed mocks.** `SettingsSheet.shell-warn-toggle.test.tsx` and `SettingsSheet.tailscale-status.test.tsx` mock the bindings with explicit factories, so each gained `GetAppearance: vi.fn().mockResolvedValue('system')`.
- **gofmt in client_test.go is pre-existing.** It is not from this plan and was left alone.
- `macWindowOptions()` still leaves `Appearance` unset (`grep -cE 'Appearance:\s*mac\.' main.go` = 0). A window-level appearance would override NSApp.appearance.

## Native check M-68 (orchestrator, macOS 27.2 Tart VM, scripted real input)

**Result: the webview follows NSApp.appearance, so there is no fallback.**

1. The control shows Automatic / Light / Dark, with Automatic selected and the correct description. PASS.
2. macOS Light, app set to Dark: the window chrome, toolbar, sidebar, the open Settings sheet and the terminal all went dark at once, with no restart. PASS. CSS tokens and the terminal followed NSApp.appearance. The terminal used the DOM renderer because plugins are zero-valued on a fresh install (an inherited bug, out of scope).
3. macOS Dark, app set to Light: everything went light. PASS.
4. Automatic: the app followed macOS Dark → Light live. PASS.
5. Stored Dark (`settings.json` holds `"appearance":"dark"`): after killing the GUI and daemon and relaunching, 12 burst screenshots show no light flash. The first visible frame is dark grey (the fade-in), then fully dark. PASS.
6. The Safari web share page was not tested natively, because Tailscale is off in the VM. This rests on `web-color-scheme.spec.ts` (Playwright, passing) and on Task 2's source checks that web mode never calls `GetAppearance` and passes `'system'` to `useColorScheme`.

Cosmetic finding: the "Automatic" segment label was cramped against its segment edges. This is fixed in Task 2 (see Deviations).

## Task 2 notes

- `useColorScheme(override = 'system')` returns the override for `'light'`/`'dark'`. Otherwise it returns the media value. The listener is unchanged and keeps tracking under an override, so switching back to `'system'` is immediately correct.
- `App.tsx`: `appearance` state, default `'system'`, is set from `GetAppearance()` inside init, after the `if (mode === 'web') return`. A failure keeps `'system'`. `onAppearanceChange={setAppearance}` goes to SettingsSheet, and `terminalTheme` uses `useColorScheme(mode === 'web' ? 'system' : appearance)`.
- **Fallback skipped**, because M-68 step 2 passed. `grep -c 'data-appearance' frontend/src/style.css` = 0, and App.test.tsx asserts that neither App.tsx nor style.css contains `data-appearance`.
- TESTING.md: Go 107 → 108 and total 206 → 207 (vitest 91, Playwright 7, build-script 1). The 02-13 note is in Section 2. Section 4 has new THEM-01 rows for `internal/daemon/appearance_settings_test.go`, `app_test.go` and `App.test.tsx`, and extended rows for `useColorScheme.test.tsx` and `SettingsSheet.test.tsx`. Section 5 has Category AL with **M-68**.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] "Automatic" segment cramped against its edges (M-68 finding)**
- **Found during:** the Task 1 VM check
- **Issue:** `.share-seg { flex: 1 }` has a zero basis. The Settings control shrink-wraps, so each segment got an equal third of the total width, and the longest label ("Automatic", bold when selected) overflowed its third.
- **Fix:** `className="share-segbar settings-appearance"`. `.settings-appearance` uses `display: grid; grid-auto-flow: column; grid-auto-columns: 1fr` (equal columns, each at least the widest label), and `.settings-appearance .share-seg` uses `padding: 0 12px`. It sets no colours, and the 02-12 token gate stays green. The Share dialog's control is unchanged. A SettingsSheet test asserts the class and the rule.
- **Files modified:** frontend/src/components/SettingsSheet.tsx, frontend/src/style.css, frontend/src/components/__tests__/SettingsSheet.test.tsx
- **Commit:** 5e5d9ea8

**2. [Rule 1] Existing App.test.tsx assertion pinned the old derivation**
- The THM-01 test expected `useColorScheme() === 'dark' ? …`. It was updated to the new desktop/web expression. Commit 5e5d9ea8.

The Task 1 departures from the plan text (dispatch_async instead of direct/dispatch_sync, the startupDone channel, 204 instead of 200) are explained in Task 1 notes.

## Full phase gate (re-run after Task 2, HEAD 5e5d9ea8 code)

| Check | Result |
|-------|--------|
| `go build ./... && go vet ./...` | pass |
| `go test -race -short ./...` | all packages ok |
| Linux cross-compile: `-tags playwrightfixture ./cmd/playwright-fixture` and `./internal/...` | pass |
| `pnpm -C frontend install --frozen-lockfile` | up to date |
| `pnpm -C frontend exec tsc --noEmit` | pass |
| `pnpm -C frontend test` | 91 files, 1153 tests passed |
| `pnpm -C frontend exec playwright test --project=chromium` | 7 passed, 9 skipped (same as 02-12); `web-color-scheme.spec.ts` both cases pass |
| `bash tests/build-script.test.sh` | 37 passed, 0 failed |
| `bash tests/check-traceability-paths.sh` | OK: 92 traceability paths exist |
| `bash scripts/grep-gate.sh` | PASS: all action refs SHA-pinned |
| `wails build -platform darwin/arm64 -tags wailsassets` | built `/Users/ken/dev/portty/build/bin/agenthub.app`; wailsjs tree restored, `git status --porcelain -- frontend/src/wailsjs` empty |
| Acceptance: `useColorScheme(override` / `GetAppearance` in App.tsx / `**M-68**` / THEM-01 appearance_settings row / `data-appearance` in style.css | 1 / 1 / 1 / 1 / 0 |

## End-of-phase human-check list addition

Add to the 02-12 SUMMARY list for /gsd-verify-work:
- **M-68:** Settings ▸ Appearance Automatic / Light / Dark, live, with relaunch and no flash, and the web page following the browser. Steps 1-5 passed in the VM during this plan. Still to check: step 6 (Safari web share with Tailscale on) and the label spacing after the fix (step 1) in the rebuilt app.

## Known Stubs

None.

## Threat Flags

None. T-02-42: server-side enum validation, normalisation on load, and a pre-RPC check in the binding (tests cover all three). T-02-43: only three fixed appearance names, set on the main queue. T-02-44: web mode never calls GetAppearance and passes 'system' (App.test.tsx).

## Self-Check: PASSED

- FOUND: appearance_darwin.go, appearance_darwin.m, internal/daemon/appearance_settings_test.go, frontend/src/lib/useColorScheme.ts
- FOUND: a24453b4, 5e5d9ea8 (ancestors of HEAD)
