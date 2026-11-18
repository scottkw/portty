---
phase: 01-plain-macos-terminal
plan: 07
subsystem: frontend tabs / hub / web-share warning / css / help
tags: [strip-down, agent-identity-removal, web-share-warning, colorblind-safe, hub]
status: complete
requires:
  - phase: 01-06
    provides: Three-state status model; tabs and Hub already free of agent status heuristics
provides:
  - Frontend reads no SessionInfo.cli / RemoteSession.cliType (bindings, remote adapter, App, TabBar, Hub, share modal)
  - Session names fall back to 'shell' (tab bootstrap x2, quit modal, Hub open)
  - onOpenSession / handleOpenSessionTab take (sessionId, name) only
  - Web-share warning gate is `next && shellWebShareWarningEnabled && !shellWebShareWarned` for every session
  - Banner secondary button reads "Keep sharing off" (focus on mount, Esc, onCancel unchanged)
  - No agent palettes in style.css (tab dot, card spine, agent chip, legacy badge)
  - getting-started.md describes New Session as picking a working directory for a shell
affects: [01-08, 01-10]
actuals:
  tokens: 35500
  tasks: 2
  commits: 2
plan_head_before: 6ebeac81829c0ceef08b7b66dbbe3ab33c61af10
plan_head_after: f27ab63e0745108d4ec8cbbca5a238bfa9b6eb8c
tech-stack:
  added: []
  removed: ["lib/agentBadge (agent color classifier)", "lib/shellCli (shell-type gate)", "agent badge/spine/chip CSS palettes", "TabBar agentDisplayName tooltip suffix"]
  patterns: ["Daemon still sends cli/cliType until 01-08; frontend types simply omit them (JSON extra fields ignored)"]
key-files:
  created: []
  modified:
    - frontend/src/wailsjs/go/main/App.d.ts
    - frontend/src/wailsjs/wailsjs/go/models.ts
    - frontend/src/App.tsx
    - frontend/src/components/TabBar.tsx
    - frontend/src/components/Hub/SessionCard.tsx
    - frontend/src/components/Hub/SessionCardGrid.tsx
    - frontend/src/components/Hub/HubModal.tsx
    - frontend/src/components/Hub/HubPanel.tsx
    - frontend/src/components/Hub/SessionShareModal.tsx
    - frontend/src/components/ShellWebShareBanner.tsx
    - frontend/src/lib/remoteAdapter.ts
    - frontend/src/lib/remoteSession.ts
    - frontend/src/style.css
    - frontend/src/content/help/getting-started.md
    - TESTING.md
  deleted:
    - frontend/src/lib/agentBadge.ts
    - frontend/src/lib/agentBadge.test.ts
    - frontend/src/lib/shellCli.ts
    - frontend/src/lib/shellCli.test.ts
decisions:
  - "TabBar name tooltip is the constant 'Double-click or right-click to rename' (agent suffix gone, titleText variable removed)"
  - "Also deleted the now-consumerless .hub-card__chip--agent and .hub-card__badge base rules, not only the data-agent / tab__agent-badge selectors (their only consumers were removed in Task 1)"
  - "getting-started.md says 'starts a shell' (not 'login shell') and drops the Settings -> Paths sentence, so it stays true under the fixed-/bin/zsh amendment"
metrics:
  duration: 15m
  completed: 2026-11-14
---

# Phase 1 Plan 07: Agent identity removed from the UI; one-time web-share warning for every session Summary

The frontend no longer reads or shows a session agent/CLI anywhere (tabs, Hub cards, Hub modal, Hub search, remote adapter). The raw-shell web-share warning now fires on the first share of any session while it is enabled and not yet acknowledged, and its secondary button reads "Keep sharing off". Agent color palettes and agent help sentences are gone.

## Tasks

| # | Task | Commit | Key files |
|---|------|--------|-----------|
| 1 | Tabs, Hub, search and web-share gate work without cli (tracer) | fd8a731e | App.tsx, TabBar.tsx, Hub/*, SessionShareModal.tsx, remoteAdapter.ts, remoteSession.ts, App.d.ts, models.ts; lib/agentBadge + lib/shellCli deleted |
| 2 | "Keep sharing off" copy, agent palettes, help text, TESTING.md | f27ab63e | ShellWebShareBanner.tsx, style.css, getting-started.md, CSS gate tests, TESTING.md |

Tracer gate (Task 1): the `<verify>` ran clean end-to-end (tsc, the full vitest suite, the three targeted files, and the non-test grep returned 0) before Task 2 started.

## Verification

- `pnpm -C frontend exec tsc --noEmit`: clean
- `pnpm -C frontend test`: 125 files / 2011 tests passed
- `go build ./... && go vet ./... && go test -race -short ./...`: green (Go unchanged)
- `bash tests/check-traceability-paths.sh`: OK, 119 paths
- `bash tests/build-script.test.sh`: 37 passed
- `wails build -platform darwin/arm64 -tags wailsassets`: built; `frontend/src/wailsjs/wailsjs/` restored afterward
- Acceptance greps: non-test cli/agent-helper refs 0; agentBadge/shellCli paths untracked; `cli: string` in App.d.ts 0; gate string 1; `s.name || 'shell'` 3; data-agent/tab__agent-badge in style.css 0; old Cancel text 0; "AI coding|choose an agent" 0; agentBadge/shellCli test names in TESTING.md 0

## Web-share warning test coverage (T-01-23)

- SessionShareModal.test.tsx: first share with enabled && !warned shows the banner and blocks ToggleWebServing; warned=true shares straight through; warningEnabled=false shares straight through; "Keep sharing off" (now asserted, previously guarded by an `if`) calls onShellWebShareCancel and keeps sharing off; confirm calls onShellWebShareConfirm.
- App.shellWebShare.test.tsx: re-arm path (onShellWarnEnabledChange(true) re-fetches GetShellWebShareWarned); no isShellCli in App.tsx.
- ShellWebShareBanner.test.tsx: "Keep sharing off" text, focus on mount, click and Esc call onCancel.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] SessionCardGrid.tsx onOpenSession signature**
- **Found during:** Task 1
- **Issue:** SessionCardGrid declares the same `onOpenSession(sessionId, name, cli)` prop type that passes through to SessionCard; it was not in the plan's file list.
- **Fix:** Signature changed to `(sessionId, name)`.
- **Files modified:** frontend/src/components/Hub/SessionCardGrid.tsx
- **Commit:** fd8a731e

**2. [Rule 1 - Bug] Vacuous cancel assertion in SessionShareModal.test.tsx**
- **Found during:** Task 2
- **Issue:** The cancel test wrapped its assertion in `if (cancelBtn)`, so it passed even when the button was missing, which would have hidden the button rename.
- **Fix:** The test now finds "Keep sharing off" and asserts it exists before clicking.
- **Commit:** f27ab63e

**3. [Rule 3 - Blocking] themeTokens.test.ts fence entry and dead agent CSS**
- **Found during:** Task 2
- **Issue:** themeTokens.test.ts fenced the `.tab__agent-badge--` selector, which no longer exists. `.hub-card__chip--agent` and `.hub-card__badge` had no consumers left after Task 1.
- **Fix:** Removed the fence entry and deleted both rules. style.hub.test.ts now asserts all three are absent.
- **Commit:** f27ab63e

Other mock edits beyond the listed files (mechanical `cli` key removal): SessionCard.share, SessionShareModal.disconnect, App.open-remote, HubInteractiveModal, useChatUnreadListeners, SessionCardGrid, HubBriefingModal tests and hubGroupCounts.test.ts.

## Known Stubs

None.

## Deferred / Notes

- TESTING.md Category I (M-15, live Antigravity launch) still describes agent behavior. This plan did not cover it, so it was left for the plans that finish the strip-down.
- getting-started.md line 23 still lists "Paths (CLI and shell binary locations)" under Settings. The amendment plans (after 01-10) remove Settings -> Paths, so it was left for them.
- Go still sends `cli`/`cliType`. Plan 01-08 removes them from the wire. `wails build` regenerates models.ts with the field, which is why the dead tree is restored after each build.

## Self-Check: PASSED

- FOUND: fd8a731e, f27ab63e (ancestors of HEAD)
- FOUND: frontend/src/components/ShellWebShareBanner.tsx contains "Keep sharing off"
- FOUND: frontend/src/components/Hub/SessionShareModal.tsx contains "shellWebShareWarningEnabled && !shellWebShareWarned"
- MISSING (intended): frontend/src/lib/agentBadge.ts, agentBadge.test.ts, shellCli.ts, shellCli.test.ts
