---
phase: 01-plain-macos-terminal
plan: 05
subsystem: daemon / wails bindings / settings ui
tags: [strip-down, shell-resolver, cli-detection-removal, settings-paths, security]
status: complete
requires:
  - phase: 01-04
    provides: Shell-only CreateSession; spawn = (GetShellPath(), ["-l"])
provides:
  - Single checked resolver resolveDefaultShellPath ($SHELL if absolute+existing+executable+non-dir, else /bin/zsh) shared by display and spawn
  - No agent-CLI detection or shell discovery anywhere (internal/pty detect/shells, GET /shells, DetectCLIs, ListShells, ResolveCLI, DiscoverShells gone)
  - Settings → Paths table = Program/Path headers, exactly shell + tailscale rows
  - Daemon PATH augmentation limited to /opt/homebrew/bin and /usr/local/bin
affects: [01-06, 01-07, 01-10]
actuals:
  tokens: 22400
  tasks: 3
  commits: 3
plan_head_before: 91a0e0a266e3133cda5a1225d3de4f939ffa5d73
plan_head_after: c87cc786264da00c55bf09a35dd4cd4b596b1ff9
tech-stack:
  added: []
  removed: ["internal/pty CLI detection (knownCLIs)", "internal/pty shell discovery (/etc/shells scan)", "GET /shells route"]
  patterns: ["One resolver function feeds both the displayed value and the spawned binary; tests assert spawn path == GetShellPath()"]
key-files:
  created: []
  modified:
    - internal/daemon/engine.go
    - internal/daemon/api.go
    - internal/daemon/types.go
    - internal/daemon/client.go
    - internal/daemon/path.go
    - internal/pty/session.go
    - app.go
    - frontend/src/wailsjs/go/main/App.d.ts
    - frontend/src/wailsjs/go/main/App.js
    - frontend/src/wailsjs/go/models.ts
    - frontend/src/wailsjs/wailsjs/go/models.ts
    - frontend/src/App.tsx
    - frontend/src/components/SettingsTab.tsx
    - TESTING.md
  deleted:
    - internal/pty/detect.go
    - internal/pty/detect_test.go
    - internal/pty/shells.go
    - internal/pty/shells_test.go
key-decisions:
  - "resolveDefaultShellPath rules: settings shellPath wins (in GetShellPath); else $SHELL only when filepath.IsAbs, os.Stat succeeds, not a directory, and any execute bit set; else /bin/zsh. Never returns empty"
  - "Kept the knownShells stale-path filter in loadSettingsFromDisk (Chesterton: it still drops shell-valued overrides for non-shell keys such as tailscale); removed only the pwsh/powershell/.exe names and the Windows comment"
  - "Legacy agent cliPaths keys (e.g. claude) still load as inert data; SettingsTab ignores them for display and save"
  - "TestEngineResolveCLI rewritten as TestEngineUpdateCLIPath against the tailscale key instead of deleted, so UpdateCLIPath/GetCLIPaths keep engine-level coverage"
patterns-established:
  - "assertSpawnMatchesDisplay(t, shellPath, want) helper in engine_test.go for resolver cases"
requirements-completed: [STRP-01, TERM-01]
coverage:
  - id: D1
    description: "Shown shell == spawned shell for every resolver branch, argv exactly [-l]"
    requirement: TERM-01
    verification:
      - kind: unit
        ref: "internal/daemon/engine_test.go#TestResolveDefaultShellPath_*"
        status: pass
  - id: D2
    description: "No detection/discovery symbols in non-test Go or frontend code"
    requirement: STRP-01
    verification:
      - kind: other
        ref: "grep acceptance checks (Task 2 / Task 3) print 0; wails build generates no DetectCLIs/ListShells/DetectedShell"
        status: pass
  - id: D3
    description: "Settings → Paths shows exactly shell + tailscale with Program/Path headers"
    requirement: STRP-01
    verification:
      - kind: unit
        ref: "frontend/src/components/__tests__/SettingsTab.shellPath.test.tsx#STRP-01"
        status: pass
  - id: D4
    description: "Tailscale override survives; legacy agent keys load; stale shell-valued overrides dropped"
    requirement: STRP-01
    verification:
      - kind: unit
        ref: "internal/daemon/engine_migration_test.go#TestSettingsMigration_LegacyAgentHubV42"
        status: pass
      - kind: unit
        ref: "internal/daemon/engine_settings_test.go#TestLoadSettingsDropsStaleShellPaths"
        status: pass
duration: 9min
completed: 2026-11-14
---

# Phase 1 Plan 05: Detection Removal and Single Shell Resolver Summary

**The shell shown in the New Session modal and Settings is now the exact binary the daemon spawns, resolved by one checked function: settings shellPath, else `$SHELL` if it is an absolute path to an existing executable file, else `/bin/zsh`. Agent-CLI detection and shell discovery are gone from the daemon, the Wails bindings and the UI. Settings → Paths now has exactly two rows: shell and tailscale.**

## Performance

- **Duration:** about 9 min
- **Started:** 2026-11-14T22:18:25Z
- **Completed:** 2026-11-14T22:26:58Z
- **Tasks:** 3
- **Files changed:** 32 (222 insertions, 1449 deletions)

## Accomplishments

- `resolveDefaultShellPath` no longer calls `pty.DiscoverShells`. A relative, nonexistent, directory, non-executable or unset `$SHELL` now falls back to `/bin/zsh`, where before it was passed straight to spawn (T-01-18). Seven `TestResolveDefaultShellPath_*` cases assert that the spawned binary equals `GetShellPath()` and that argv is `["-l"]`.
- Deleted `internal/pty/detect.go`, `shells.go` and their tests; engine `ResolveCLI` / `DiscoverShells`; `GET /shells`; `DetectedShell` / `ShellsResponse`; client `ListShells`; app `DetectCLIs` / `ListShells`.
- `AugmentServicePath` now adds only `/opt/homebrew/bin` and `/usr/local/bin`. `nvmActiveBin` and the volta/cargo/`~/.local/bin` entries are gone.
- On the frontend: removed the bindings (including the tracked dead-tree `models.ts`) and the `detectedCLIs` state. `tailscaleInstalled` is now `!!tailscaleHealth?.installed`; the old `detectedCLIs.some(...tailscale)` branch could never be true. The SettingsTab `clis` prop, agent rows and the "No CLIs detected" paragraph are gone, and the headers now read Program / Path.

## Task Commits

1. **Task 1: One resolver feeds both the Shell line and the spawned login shell** - `72e1c482` (feat)
2. **Task 2: Daemon and GUI bindings carry no CLI detection or shell discovery** - `a33cc75f` (feat)
3. **Task 3: Frontend drops detection state; Settings → Paths shows exactly shell and tailscale** - `c87cc786` (feat)

## Decisions Made

See `key-decisions` in the frontmatter.

## Deviations from Plan

### Minor

**1. NewSessionModal.test.tsx not edited**
- The plan said to "keep (or add)" an assertion that the Shell line renders exactly the GetShellPath result. That assertion already exists in `NewSessionModal.shellRow.test.tsx` ('/opt/homebrew/bin/zsh' resolved and rendered verbatim), and both files pass, so I added nothing.

**2. App tests needed no edits**
- No App test mocked `DetectCLIs` / `ListShells`, so `App.test.tsx` and `App.shellWebShare.test.tsx` stayed unchanged. The full vitest suite passes.

**3. Dead-tree `App.d.ts` / `App.js` are gitignored (generated)**
- Of the dead tree under `frontend/src/wailsjs/wailsjs/go/`, only `models.ts` is tracked. I committed it with `git add -u`. The `wails build` output confirms the generated `App.d.ts` / `models.ts` have no detection symbols. Afterwards I restored the dead tree with `git checkout -- frontend/src/wailsjs/wailsjs/`.

**4. Extra resolver cases**
- Besides the planned cases, I added tests for a directory `$SHELL` and a non-executable `$SHELL`, since the resolver checks both.

## Verification

- `go build ./... && go vet ./... && go test -race -short ./...`: all packages ok
- Linux cross-builds (playwright-fixture, internal/...): ok
- `pnpm -C frontend exec tsc --noEmit && pnpm -C frontend test`: 127 files, 2056 tests passed. The flaky SM-02 case did not fail.
- `bash tests/check-traceability-paths.sh`: OK (121 paths). `bash tests/build-script.test.sh`: 37 passed
- `wails build -platform darwin/arm64 -tags wailsassets`: built. The dead tree was restored afterwards.
- Acceptance greps: detection symbols in non-test Go = 0; in non-test frontend = 0; `pwsh|powershell` in engine.go = 0; agent dirs in path.go = 0; `/opt/homebrew/bin` = 1; `paths["tailscale"]` in app.go = 2; `<th>Program</th>` = 1; `clis` in SettingsTab = 0.

## Deferred Issues

- `.settings-panel__empty` in `frontend/src/style.css:755` is now unused; a contrast source-gate test in `style.contrast.test.ts` still pins it. It was out of this plan's file scope, so I left it for a later CSS cleanup.
- `app.go:139` has a comment that names `pty.DetectCLI`, inside `displayNameForCLI`. Plan 01-06 deletes that function.
- TESTING.md M-15 (live Antigravity launch, Category I) describes a removed feature. It is historical and marked PASSED, so I left it for the phase's TESTING.md cleanup.

## Known Stubs

None.

## Next Phase Readiness

- `cliPaths` now only matters for `tailscale`. Legacy agent keys stay in settings.json as inert data until a later plan prunes them.

## Self-Check: PASSED

Task commits 72e1c482 / a33cc75f / c87cc786 are ancestors of HEAD. The four deleted internal/pty files are gone, and every modified file is present.
