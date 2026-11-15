---
phase: 01-plain-macos-terminal
plan: 04
subsystem: daemon / cli / wails bindings / ui
tags: [login-shell, session-create, strip-down, opencode-removal, new-session-modal, security]
status: complete
requires:
  - phase: 01-03
    provides: files API removed; capability mint read / read,write
provides:
  - Shell-only CreateSession contract end to end (engine, HTTP, client, Wails, TS)
  - Every session spawns GetShellPath() with argv exactly ["-l"] (login shell)
  - SetSpawnCommandForTest engine seam for deterministic tests
  - `agenthub new [dir]` as the only CLI create form
  - New Session modal per UI-SPEC section 1 (Working Directory + read-only Shell line)
  - No OpenCode config, SIGUSR2 theme broadcast or sessionCLIs map
affects: [01-05, 01-06, 01-07, 01-10]
actuals:
  tokens: 45700
  tasks: 3
  commits: 3
plan_head_before: f7781993d592a9d5ec567d9dcdc702c230ab0bf0
plan_head_after: 410445d1301f3488684664aab44ceb8b0547e5d9
tech-stack:
  added: []
  removed: ["OpenCode TUI config writer (opencode-tui.json)", "POST /theme/notify", "SIGUSR2 theme broadcast"]
  patterns: ["Fixed spawn argv behind an unexported func field with an exported ForTest override", "Legacy request keys ignored by plain json.Decoder; asserted by a spy-backend API test"]
key-files:
  created: []
  modified:
    - internal/daemon/engine.go
    - internal/daemon/types.go
    - internal/daemon/api.go
    - internal/daemon/client.go
    - app.go
    - main.go
    - cmd_cli.go
    - frontend/src/wailsjs/go/main/App.d.ts
    - frontend/src/wailsjs/go/main/App.js
    - frontend/src/App.tsx
    - frontend/src/components/NewSessionModal.tsx
    - frontend/src/style.css
    - TESTING.md
  deleted:
    - internal/daemon/notify_theme_unix.go
    - internal/daemon/opencode_ansi_test.go
key-decisions:
  - "Spawn is (GetShellPath(), [\"-l\"]) via an unexported spawnCommand field; tests override with SetSpawnCommandForTest(\"/bin/cat\", nil)"
  - "TestCreateSession_LoginShell uses the existing hubReadUntil helper (not GetSessionTailLines) and an empty .zshrc in ZDOTDIR so zsh-newuser-install cannot eat the input"
  - "Remaining non-test 'opencode' strings (app.go displayNameForCLI, internal/pty/detect.go knownCLIs) left for 01-06 / 01-05, which own and delete those symbols"
  - "Shell line and folder display share .new-session-modal__folder-display; added min-width: 0 so long paths ellipsize inside the flex row instead of widening the modal"
patterns-established:
  - "New engine/API/client/cmd/app test setups call SetSpawnCommandForTest(\"/bin/cat\", nil) for a deterministic echo process"
requirements-completed: [TERM-01, STRP-01, TERM-04]
coverage:
  - id: D1
    description: "New sessions (GUI and CLI) start the resolved shell as a login shell in the chosen dir, home when empty"
    requirement: TERM-01
    verification:
      - kind: integration
        ref: "internal/daemon/engine_test.go#TestCreateSession_LoginShell"
        status: pass
      - kind: unit
        ref: "internal/daemon/engine_test.go#TestCreateSession_LoginShellArgv"
        status: pass
      - kind: unit
        ref: "internal/daemon/engine_test.go#TestCreateSession_ShellEmptyWorkDirHome"
        status: pass
  - id: D2
    description: "Legacy POST /sessions bodies with cli/args cannot inject argv"
    requirement: TERM-01
    verification:
      - kind: integration
        ref: "internal/daemon/api_test.go#TestHandleCreateSession_LegacyBodyIgnored"
        status: pass
  - id: D3
    description: "`agenthub new [dir]` is the only CLI create form"
    requirement: STRP-01
    verification:
      - kind: integration
        ref: "cmd_cli_test.go#TestCmdNew_WithDir"
        status: pass
      - kind: integration
        ref: "cmd_cli_test.go#TestCmdNew_NoDir"
        status: pass
  - id: D4
    description: "New Session modal: Working Directory + read-only Shell line with loading/error/empty states, no agent list or args field"
    requirement: STRP-01
    verification:
      - kind: unit
        ref: "frontend/src/components/__tests__/NewSessionModal.test.tsx"
        status: pass
      - kind: unit
        ref: "frontend/src/components/__tests__/NewSessionModal.shellRow.test.tsx"
        status: pass
      - kind: manual_procedural
        ref: "TESTING.md M-54 (long-path truncation and login shell in the built app)"
        status: unknown
  - id: D5
    description: "Theme switching still re-themes terminals with no OpenCode broadcast"
    requirement: TERM-04
    verification:
      - kind: unit
        ref: "frontend/src/components/__tests__/App.test.tsx#TERM-04: terminal theme change re-themes open terminals"
        status: pass
duration: 17min
completed: 2026-11-14
---

# Phase 1 Plan 04: Login-Shell Sessions Summary

**Every new session, from the New Session modal or `agenthub new [dir]`, now runs `GetShellPath()` with exactly `-l`. A real-zsh test proves it is a login shell. The modal is now a directory picker plus a read-only shell line, and the OpenCode theme/config code is gone.**

## Performance

- **Duration:** about 17 min
- **Started:** 2026-11-14T21:58:12Z
- **Completed:** 2026-11-14T22:15:39Z
- **Tasks:** 3
- **Files modified:** 35 (585 insertions, 2113 deletions)

## New CreateSession contract (for later plans)

| Layer | Signature |
|-------|-----------|
| Engine | `func (e *SessionEngine) CreateSession(ctx context.Context, name, workDir string, cols, rows int, onExit func(string, int)) (string, error)` |
| Test seam | `func (e *SessionEngine) SetSpawnCommandForTest(path string, args []string)` |
| HTTP | `POST /sessions` body `CreateRequest{Name, WorkDir, Cols, Rows}` (legacy `cli` / `args` keys are ignored) |
| Client | `func (c *DaemonClient) CreateSession(name, workDir string, cols, rows int) (string, error)` |
| Wails (Go) | `func (a *App) CreateSession(name, workDir string, cols, rows int) (string, error)` (still starts pollSessionStatus) |
| Wails (TS) | `CreateSession(name: string, workDir: string, cols: number, rows: number): Promise<string>` |
| App.tsx | `createTab(workDir)`; default name `shell {n}`. Tab and hubSessions seed set `cli: 'shell'` until 01-07 removes the field |
| Modal | `NewSessionModal({ isOpen, onConfirm(workDir), onClose })` |
| CLI | `agenthub new [<path>]`; name = basename(path) or `shell`; error prefix `agenthub new:` |

## Accomplishments

- Engine spawn is fixed to `(GetShellPath(), ["-l"])`, and the WR-01 empty-dir to `$HOME` guard now covers every session. `resolveShellSpawn`, `isShellSession`, the OpenCode env injection and `status.Watch` are gone from the create path.
- `TestCreateSession_LoginShell` starts a real `/bin/zsh` (with an isolated `ZDOTDIR`) and waits for `LOGIN_OK_42`. To confirm the test can fail, I temporarily changed the argv to `-i`; the test then failed. I reverted the change before committing.
- `TestHandleCreateSession_LegacyBodyIgnored` posts `{"cli":"claude","args":["--evil"]}`. The spy backend saw exactly `["-l"]` (T-01-15 mitigated).
- New Session modal follows UI-SPEC section 1. Covered states: "Home directory (default)"; "Default login shell" while the path is loading, on error and on an empty result; "Browsing…" disabled while the picker is open; "Creating…" while a session is created.
- The OpenCode TUI config writer, the `sessionCLIs` map, `NotifyThemeChange` (engine, API route, client, app, binding, App.tsx call) and the SIGUSR2 helper are removed. `handleThemeChange` still persists the theme and calls `setTerminalThemeName`.

## Task Commits

1. **Task 1: New session = login shell, end to end** - `5559ee45` (feat)
2. **Task 2: New Session modal per UI-SPEC section 1** - `e26df137` (feat)
3. **Task 3: Per-agent spawn behavior gone, theme unchanged** - `410445d1` (feat)

## Decisions Made

See `key-decisions` in the frontmatter.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Folder display could not shrink in its flex row**
- **Found during:** Task 2
- **Issue:** `.new-session-modal__folder-display` is a `flex: 1` item with `white-space: nowrap` and the default `min-width: auto`. A long path would push the Browse button out instead of showing an ellipsis.
- **Fix:** Added `min-width: 0`, with a CSS source-gate test in `style.redesign.test.ts`.
- **Files modified:** frontend/src/style.css, frontend/src/components/__tests__/style.redesign.test.ts
- **Commit:** e26df137

**2. [Rule 3 - Blocking] Tests depending on removed code removed in Task 1, not Task 3**
- **Found during:** Task 1
- **Issue:** Some engine tests relied on the OpenCode env injection or on `sessionCLIs` being filled at create time (`TestCreateSession_OpenCodeEnv`, `TestNotifyThemeChange_BroadcastsToOpenCodeOnly` / `_NoOpenCodeSessions` / `_RealProcess_Integration`, `TestSessionCLIs_TrackedAndCleanedUp`). Task 1 removes both, so these tests could not pass after it.
- **Fix:** Deleted them in Task 1. The remaining theme/OpenCode tests were deleted in Task 3 as planned.
- **Commit:** 5559ee45

**3. [Scope] Task 3's opencode grep check prints 3 instead of 0**
- `grep -rniE 'opencode' --include='*.go' . | grep -v _test.go` still matches `app.go:150-151` (`displayNameForCLI`) and `internal/pty/detect.go:29` (`knownCLIs`). Plan 01-06 deletes `displayNameForCLI` and plan 01-05 deletes `detect.go`; each has acceptance checks with HEAD baselines. I left both alone so those plans' checks stay valid. Every OpenCode spawn, config and theme reference is gone.

**4. [Minor] Test details differ from the plan text**
- `TestCreateSession_LoginShell` waits with the existing `hubReadUntil` helper instead of polling `GetSessionTailLines`. Both read real PTY output.
- `App.createTab.*.test.tsx` needed no edits: they only check that `createTab` exists and its `setTabs` / `setActiveId` / `setHubSessions` behavior, not its arguments. The source assertions that did check arguments (App.test.tsx ARGS-02) were rewritten as TERM-01 assertions.
- `TestUpdateCLIPath` (app_test.go) no longer creates a session through the override, since overrides no longer affect spawning. It still checks `UpdateCLIPath` until 01-05 removes that method.

## Verification

- `go build ./... && go vet ./... && go test -race -short ./...`: all packages ok
- `go test -race -run 'TestCreateSession_LoginShell|LegacyBodyIgnored|TestCmdNew' -v ./internal/daemon/ .`: all PASS
- `pnpm -C frontend exec tsc --noEmit && pnpm -C frontend test`: 127 files, 2054 tests passed. The flaky SM-02 test did not fail on any run.
- `bash tests/check-traceability-paths.sh`: OK (120 paths). `bash tests/build-script.test.sh`: 37 passed
- Linux cross-builds (playwright-fixture, internal/...): ok
- `wails build -platform darwin/arm64 -tags wailsassets`: built. The generated binding has `CreateSession(arg1:string,arg2:string,arg3:number,arg4:number)` and no NotifyThemeChange. `frontend/src/wailsjs/wailsjs/` was restored afterwards.

## Deferred Issues

- README.md still documents `agenthub new <agent> <path> -- args` and `new shell`. That is out of this plan's file scope; the Phase 3 README rewrite (ROADMAP Phase 3 criterion 4) covers it.
- Manual M-54 (truncation and login shell in the built app) has not been run yet. It is the end-of-phase human check (`human_verify_mode: end-of-phase`).

## Known Stubs

None.

## Next Phase Readiness

- 01-05 can delete `ResolveCLI` (no callers remain), `DiscoverShells`, `internal/pty/detect.go`/`shells.go`, `DetectCLIs`/`ListShells` and App.tsx `detectedCLIs` (now used only by SettingsTab).
- `sessionStatuses` and its `internal/status` type are still on the engine. Nothing writes to them now (status.Watch is gone), so `ListSessions` always reports "running" for live sessions. Plan 01-06 owns that cleanup.

## Self-Check: PASSED

All modified files present, deleted files gone, task commits 5559ee45 / e26df137 / 410445d1 are ancestors of HEAD.
