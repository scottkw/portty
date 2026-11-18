---
phase: 02-tabs-sidebar-navigation
plan: 01
subsystem: ui
tags: [wails, macos, nsmenu, titlebar, vibrancy, css, react]

requires:
  - phase: 01-strip-down
    provides: Wails v2.10.2 build (`-tags wailsassets`), App.tsx desktop EventsOn effect, banner stack, Sidebar/TabBar
provides:
  - Hidden-inset title bar (mac.TitleBarHiddenInset ships), translucent window, transparent webview
  - html.is-desktop CSS scope with 52px drag strips (tab bar, sidebar) and no-drag controls
  - Native menu bar emitting menu:new-session, menu:settings, menu:close-tab, menu:toggle-sidebar
  - helpURL constant (https://portty.app) behind Help > Portty Help
affects: [02-03 light/dark tokens, 02-04 sidebar rewrite and toggle, 02-08 tab bar rewrite and Close Tab]

actuals:
  tokens: 8700
  tasks: 2
  commits: 3
plan_head_before: 07412d211ab07fb1a079074da07da61ef29028ac
plan_head_after: 5b5876b091ecfb9b3bff661cc2fffa9a683028ad

tech-stack:
  added: []
  patterns:
    - "menuEmit/menuOpenURL package-var seams make native menu callbacks testable without a Wails runtime"
    - "Desktop-only CSS is scoped under html.is-desktop (set in main.tsx from detectMode())"
    - "Drag regions: --wails-draggable: drag on the strip, no-drag on every interactive descendant"
    - "[] -deps EventsOn handlers call the latest callback through a ref (openSettingsRef, like removeTabStateRef)"

key-files:
  created:
    - menu_test.go
  modified:
    - main.go
    - frontend/src/main.tsx
    - frontend/src/App.tsx
    - frontend/src/style.css
    - frontend/src/components/__tests__/App.test.tsx
    - TESTING.md
    - .planning/phases/02-tabs-sidebar-navigation/02-UI-SPEC.md

key-decisions:
  - "HiddenInset ships: macOS 27.2 click test showed the NSToolbar does not swallow clicks, so the TitleBarHidden fallback was not needed"
  - "A custom first submenu replaces the AppMenu role so Settings… can sit in the app menu; Hide Others/Show All dropped (no Go API), About deferred to Phase 3"
  - "menu:close-tab and menu:toggle-sidebar are emitted but not listened to; 02-08 and 02-04 wire them (closing a tab still kills the shell today)"

patterns-established:
  - "Native menu item -> emitMenuEvent(name) -> EventsOn(name) in App.tsx's desktop effect"

requirements-completed: [LOOK-01]

coverage:
  - id: D1
    description: "Window options: hidden-inset title bar, transparent webview, translucent window, system appearance, alpha-0 background"
    requirement: LOOK-01
    verification:
      - kind: unit
        ref: "menu_test.go#TestMacWindowOptions"
        status: pass
    human_judgment: false
  - id: D2
    description: "Menu bar structure, accelerators, emitted events and Help URL"
    requirement: LOOK-01
    verification:
      - kind: unit
        ref: "menu_test.go#TestAppMenu_Structure, TestAppMenu_Accelerators, TestAppMenu_EventsEmitted, TestAppMenu_NewSessionEmitsEvent, TestAppMenu_HelpOpensPorttyURL"
        status: pass
    human_judgment: false
  - id: D3
    description: "Frontend menu listeners (new-session, settings), banner stack under the tab bar, drag-strip CSS (collapsed-sidebar inset, no text-select)"
    requirement: LOOK-01
    verification:
      - kind: unit
        ref: "frontend/src/components/__tests__/App.test.tsx"
        status: pass
    human_judgment: false
  - id: D4
    description: "Native chrome in the built app: traffic lights in the sidebar strip, first-click tabs, window drag, Cmd+T"
    requirement: LOOK-01
    verification:
      - kind: manual_procedural
        ref: "TESTING.md M-58 (click-tested by the orchestrator in a macOS 27.2 Tart VM with CGEvent clicks)"
        status: pass
    human_judgment: true
    rationale: "Native NSToolbar hit-testing and window drag cannot run under vitest, Go tests or Playwright"
  - id: D5
    description: "Sidebar vibrancy behind a translucent window"
    requirement: LOOK-01
    verification:
      - kind: manual_procedural
        ref: "TESTING.md M-58 step 6"
        status: unknown
    human_judgment: true
    rationale: "Vibrancy unverified in the VM (sidebar pixels sample #EDEDED with or without content behind); needs a glance on a real host"
  - id: D6
    description: "Menu bar in the built app: Cmd+, Settings, Help opens portty.app, Cmd+H hides, Cmd+Q shows quit dialog"
    requirement: LOOK-01
    verification:
      - kind: manual_procedural
        ref: "TESTING.md M-59"
        status: unknown
    human_judgment: true
    rationale: "Native NSMenu accelerators and Wails event delivery need the built app; the Task 2 menu was not rebuilt or run"

duration: 64min
completed: 2026-11-15
status: complete
---

# Phase 2 Plan 01: Native Window Chrome and Menu Bar Summary

**Hidden-inset translucent window (HiddenInset ships, click-tested on macOS 27.2), 52px drag strips with no-drag controls, and a rebuilt native menu bar where Cmd+T opens New Session, Cmd+, opens Settings and Help > Portty Help opens https://portty.app.**

## Performance

- **Duration:** about 64 min, including the orchestrator's VM click test
- **Started:** 2026-11-15T18:29:49Z
- **Completed:** 2026-11-15T19:34:00Z
- **Tasks:** 2 of 2
- **Files modified:** 8

## Human-check outcome (Task 1 tracer gate)

**Approved: HiddenInset ships.** The orchestrator click-tested the built app in a macOS 27.2 Tart VM with scripted CGEvent clicks:

1. Traffic lights sit in the sidebar's top strip, no title shows. PASS.
2. All three tabs activate on the first click, Welcome's x closes on the first click, no click starts a drag. PASS. The NSToolbar does not swallow clicks.
3. The toggle collapses and expands the sidebar. PASS. One expand click right after a layout change was missed; a repeat worked on the first click.
4. Dragging empty top-bar space and the empty sidebar strip moves the window. PASS. Only horizontal movement could be verified: the helper's vertical drags move no window, Finder's included.
5. Cmd+T opens New Session, through the menu item and the key chord. PASS.
6. **Vibrancy unverified in VM, needs a host glance.** Sidebar pixels sample identical (#EDEDED) with or without colourful content behind the window. Implementation left as is.

The check found defects A and B (below), fixed in `c709c3a6`.

## Accomplishments

- `macWindowOptions()` returns `mac.TitleBarHiddenInset()`, a transparent webview and a translucent window; `BackgroundColour` is alpha 0.
- `appMenu()` rebuilt: AgentHub (Settings… Cmd+,, Hide Cmd+H, Quit Cmd+Q), File (New Session Cmd+T, Close Tab Cmd+W), Edit, View (Toggle Sidebar Ctrl+Cmd+S), Window, Help (Portty Help, Check for Updates, AgentHub on GitHub). Quit still goes through `OnBeforeClose` and the quit-confirm modal.
- App.tsx listens for `menu:new-session` and `menu:settings`; the banner stack sits under the tab bar.
- Desktop-only CSS: transparent html/body/#root/.app/.sidebar, opaque main column, 52px drag strips, an 80px traffic-light reserve in both sidebar states.

## Task Commits

1. **Task 1: Cmd+T opens New Session in a hidden-inset translucent window** - `3e8066ad` (feat)
2. **Task 1 UAT fixes: defects A and B** - `c709c3a6` (fix)
3. **Task 2: menu bar matches the UI-SPEC** - `5b5876b0` (feat)

## Files Created/Modified

- `main.go` - window options, menu seams, `helpURL`, rebuilt `appMenu()`
- `menu_test.go` - window option and menu tests (6 tests)
- `frontend/src/main.tsx` - `html.is-desktop` class in desktop mode
- `frontend/src/App.tsx` - menu listeners, `openSettingsRef`, banner stack moved under TabBar
- `frontend/src/style.css` - desktop chrome block (transparency, drag strips, traffic-light inset, no text-select)
- `frontend/src/components/__tests__/App.test.tsx` - listener, banner-stack and drag-strip CSS assertions
- `TESTING.md` - Go count 122, two LOOK-01 rows, Category AC with M-58 and M-59
- `.planning/phases/02-tabs-sidebar-navigation/02-UI-SPEC.md` - three deviation rows

## Decisions Made

- HiddenInset over the Hidden fallback (human check passed). No "traffic lights at standard height" deviation row was needed, since that row only applied to the fallback.
- UI-SPEC deviation rows added: About item omitted (Phase 3, BRND-01); Hide Others/Show All dropped (custom first submenu, no Go API); static "Toggle Sidebar" label instead of Hide/Show Sidebar.

## Deviations from Plan

### Auto-fixed and UAT-reported issues

**1. [Rule 1 - Bug] Defect A: collapsed sidebar put the first tab under the traffic lights**
- **Found during:** Task 1 human check
- **Issue:** The collapsed sidebar is 48px wide, so the tab strip started well inside the ~80px traffic-light area and the lights overlapped the first tab's label.
- **Fix:** `html.is-desktop .sidebar--collapsed + .app__content .tab-bar { padding-left: 32px }` (48 + 32 = 80). The padding is part of the drag region.
- **Files modified:** frontend/src/style.css, frontend/src/components/__tests__/App.test.tsx
- **Committed in:** `c709c3a6`

**2. [Rule 1 - Bug] Defect B: dragging the sidebar strip text-selected "Home"**
- **Found during:** Task 1 human check
- **Fix:** `-webkit-user-select: none; user-select: none` on `html.is-desktop :is(.sidebar, .tab-bar)`. WebKit needs the prefix. Inputs inside both (group create, tab rename) opt back in with `user-select: text` so they stay editable.
- **Committed in:** `c709c3a6`

**3. `.tab-bar` uses `height: 52px`, not `min-height: 52px`**
- `.tab-list` and `.tab` size off `height: 100%`, which needs a definite parent height.

**4. Sidebar strip built from padding plus an absolutely positioned toggle**
- `.sidebar` gets `padding-top: 52px` and is the drag region; the expanded toggle is absolutely positioned at top 12px, left 80px. A mousedown in the padding targets `.sidebar` (drag), and all children opt out with no-drag. Collapsed, the toggle stays in flow below the strip.

**5. The index.html splash still paints dark**
- RESEARCH Pitfall 2. Left for a later plan (light/dark tokens arrive in 02-03).

**6. Vibrancy unverified in VM**
- See the human-check outcome above. M-58 step 6 keeps the manual vibrancy check for a real host.

**7. Extra UI-SPEC deviation row for the static "Toggle Sidebar" label**
- The plan asked for two rows (About, Hide Others/Show All). RESEARCH Pattern 3 says the label difference must also be recorded, so a third row was added.

---

**Total deviations:** 2 UAT bugs fixed, 5 documented implementation notes. No scope creep.

## Issues Encountered

- `menu.AppMenu()` appeared in a rewritten code comment and tripped the acceptance grep (must be 0). The comment now says "built-in AppMenu role".
- The Task 2 menu changes were not rebuilt into the .app or run (the orchestrator said a rebuild was optional unless acceptance criteria required it). Go tests cover structure, accelerators and events; M-59 covers the live check.

## Verification

- `go build ./... && go vet ./...` clean; `go test -race -short ./...` exit 0
- `go test -race -run 'TestAppMenu|TestMacWindowOptions' -v .` - 6 PASS (5 `TestAppMenu_*`)
- `pnpm exec tsc --noEmit` clean; `pnpm exec vitest run` 117 files, 1832 tests passed
- `bash tests/check-traceability-paths.sh` - OK: 121 traceability paths exist
- All Task 1 and Task 2 acceptance greps meet their targets

## Threat Model

- T-02-03 (drag strip DoS): mitigated. Every interactive descendant sets no-drag, and the human check confirmed first-click tabs and buttons.
- T-02-04 (Cmd+W reaching kill-on-close): mitigated. No `menu:close-tab` listener exists; App.test.tsx asserts its absence.

## Next Phase Readiness

- 02-04 (sidebar rewrite) should keep the `.sidebar` 52px strip, the 80px reserve and the `user-select: none` rule, and add the `menu:toggle-sidebar` listener. The UI-SPEC collapses the sidebar to 0 width, which means the tab bar's collapsed inset becomes 80px rather than 32px.
- 02-08 (tab bar rewrite) wires `menu:close-tab` once closing a tab only detaches.
- Vibrancy needs a glance on a real host.

## Self-Check: PASSED

- FOUND: menu_test.go, main.go, frontend/src/style.css, TESTING.md, 02-UI-SPEC.md
- FOUND commits: 3e8066ad, c709c3a6, 5b5876b0
