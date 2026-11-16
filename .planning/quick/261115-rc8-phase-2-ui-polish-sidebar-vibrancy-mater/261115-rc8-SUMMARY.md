---
phase: quick-261115-rc8
plan: 01
subsystem: ui
tags: [macos, vibrancy, css, settings, sharing]
status: complete
requires: []
provides:
  - "setSidebarMaterial / applySidebarMaterial native bridge"
  - "UI-SPEC terminal padding 12px 16px"
  - "LAN-mode Web server description"
affects: [LOOK-01, NET-01]
tech-stack:
  added: []
  patterns: ["main-queue dispatch_async AppKit mutation from Wails startup (same as setAppAppearance)"]
key-files:
  created: []
  modified:
    - appearance_darwin.m
    - appearance_darwin.go
    - app.go
    - frontend/src/style.css
    - frontend/src/components/__tests__/TerminalPanel.test.tsx
    - frontend/src/components/SettingsSheet.tsx
    - frontend/src/components/__tests__/SettingsSheet.tailscale-status.test.tsx
    - TESTING.md
decisions:
  - "The sidebar effect view uses NSVisualEffectStateFollowsWindowActiveState: vibrant while the window is key, flat when another app is frontmost (Finder/Mail behaviour, matches M-61 step 4). One line to flip back to NSVisualEffectStateActive."
  - "LAN-mode copy is built inline in the existing webServerReason ternary; no new helper."
metrics:
  duration: ~20min
  completed: 2026-11-15
actuals:
  tokens: 6000
  tasks: 3
  commits: 3
plan_head_before: 41435e6a87476efb1270ee79c7c6ca3c140570f0
plan_head_after: 252827d428738aad3253650b34ad4d1dd3c40bbd
---

# Quick 261115-rc8 Plan 01: Phase 2 UI Polish Summary

Native Sidebar material on the Wails window's NSVisualEffectView (set from `App.startup` on the main queue), UI-SPEC 12px/16px terminal padding with the leftover off-contract type and spacing fixed in `style.css`, and a Sharing pane that says "Running on your local network at <url>. Tailscale sharing is off." in LAN mode.

## Tasks

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Native Sidebar material (tracer) | 194fcc56 | appearance_darwin.m, appearance_darwin.go, app.go |
| 2 | style.css padding, type, spacing | 3d63e136 | frontend/src/style.css, TerminalPanel.test.tsx |
| 3 | LAN-mode Sharing copy + TESTING.md | 252827d4 | SettingsSheet.tsx, SettingsSheet.tailscale-status.test.tsx, TESTING.md |

## What changed

- **Task 1:** `setSidebarMaterial()` in `appearance_darwin.m` walks `NSApp.windows`, keeps only `WailsWindow` instances, and sets `NSVisualEffectMaterialSidebar`, behind-window blending and follows-active state on each direct `contentView` `NSVisualEffectView`. It logs via `NSLog` when it updates none, so a Wails upgrade that renames the class or drops the view shows up in the log. All work runs inside `dispatch_async(main)`, and the existing deadlock paragraph is unchanged. `applySidebarMaterial()` (Go) is called in `App.startup` before `initTray`. The built binary exports `_setSidebarMaterial`.
- **Task 2:** `.terminal-session-container` padding is now `12px 16px`. The rename input is 13px, `--hub-font-size-base/sm` are 13/12, and the write-gate heading is weight 600 (Chesterton comment rewritten). The nine off-scale 2/6/10px values moved to the 4/8/12 scale. Gates: the terminal-padding grep matches, the type-literal count is 0, and the off-scale count is 0. Only `padding: 2px;` remains, at lines 1523 (`.find-bar__close`) and 1971 (`.share-segbar`).
- **Task 3:** The running arm of `webServerReason` branches on `webServerMode === 'local'`. Every other mode keeps the `Running on <url>` / `Running` text. `renderSheet` takes an optional `webServerMode`. Two new cases use `mockResolvedValueOnce`. TESTING.md now has the Section 2 note (counts unchanged: 108/91/7/1 = 207, checked against `git ls-files`), the extended NET-01 row, and the extended M-61 step 5 plus its "Why not automatable" line.

## Verification

- `go build ./... && go vet ./...`: green
- `go test -race -short ./...`: all packages ok
- `pnpm exec tsc --noEmit && pnpm exec vitest run`: 91 files, 1162 tests passed (includes `style.tokens.test.ts`)
- `wails build -platform darwin/arm64 -tags wailsassets`: built `build/bin/agenthub.app` at 19:54. `frontend/src/wailsjs/wailsjs` restored, and `git status` shows only the pre-existing `.ignore` and `.planning/state.json`.
- `bash tests/check-traceability-paths.sh`: OK, 92 paths

## Pending human check

The Task 1 visual check still needs to be done by the orchestrator in the Tart VM with the build above. Set Dark mode, put colourful content behind the window, and make Portty frontmost. The sidebar should be dark and tinted like Finder's, not flat ~#474747, and the toolbar and terminal should stay opaque. With another app frontmost, the sidebar goes flat, which is expected.

## Deviations from Plan

None. The plan was executed as written. TDD RED and GREEN for Tasks 2 and 3 were each committed as one task commit, not as separate test/feat commits.

## Known Stubs

None.

## Self-Check: PASSED

- All 8 modified files exist. Commits 194fcc56, 3d63e136 and 252827d4 are ancestors of HEAD.
