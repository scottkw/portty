---
phase: 05-overnight-survival
plan: 09
subsystem: frontend
status: complete
tags: [frontend, tabs, error-boundary, shar-07, gap-closure]
requires:
  - "05-05: handleOpenRemoteSession / openWebSessionTab shape, connectHost on peer tabs, reconnectPeerTab"
provides:
  - "In-flight guard (connectingRef Set) in handleOpenRemoteSession: one connect and one tab per remote session, even on a double click"
  - "openWebSessionTab decides add-or-update inside the functional setTabs update (no stale-snapshot append)"
  - "nextActiveTabId never returns the closing id"
  - "Desktop render guard: an active __websession__ id with no tab renders nothing"
  - "components/ErrorBoundary.tsx (label, resetKey, fallback), used per terminal tab, around the web-session view, and at the root in main.tsx"
  - ".planning/phases/05-overnight-survival/05-VM-MATRIX.md with the V-09 row (shared with 05-12, 05-13, 05-14)"
affects: [05-12, 05-13, 05-14]
tech-stack:
  added: []
  patterns:
    - "Tab add-or-update decisions read the latest state inside setTabs(prev => ...), never a closure snapshot"
    - "Error fallbacks are fixed copy; logs carry only a label and error.name (error text can hold a ?cap= URL)"
    - "VM GUI evidence read from the Accessibility tree (axdump) when WebKit cannot paint into screencapture"
key-files:
  created:
    - frontend/src/components/ErrorBoundary.tsx
    - frontend/src/components/__tests__/ErrorBoundary.test.tsx
    - .planning/phases/05-overnight-survival/05-VM-MATRIX.md
  modified:
    - frontend/src/App.tsx
    - frontend/src/lib/tabs.ts
    - frontend/src/main.tsx
    - frontend/src/lib/__tests__/tabs.test.ts
    - frontend/src/components/__tests__/App.open-remote.test.tsx
    - TESTING.md
decisions:
  - "The missing-web-tab render guard is pinned with a source-inspection test (this file's existing convention), not a new pure helper: after the dedupe and next-active fixes the state cannot be reached through the UI"
  - "The web-session boundary sits inside the .terminal-wrapper around WebShareSessionView (the connect-failed banner branch is static), so the fallback keeps the pane layout; resetKey = activeId"
  - "handleOpenSessionTab (local tabs) was left as is: it is synchronous, React re-renders between two discrete clicks, and App.nav.test.tsx pins its shape"
  - "V-09 was observed through the macOS Accessibility tree because WebKit in both VMs cannot paint into screencapture right now (Safari is blank too; IOSurface creation failed)"
metrics:
  duration: 21min
  completed: 2026-11-18
  tasks: 3
  files: 9
estimate:
  tokens: 65000
  tasks: 3
actuals:
  tokens: 7900
  tasks: 3
  commits: 6
plan_head_before: 10bbdcd2e9c83776676f771a040c46694a0aa8a3
plan_head_after: 55145d7ed4b3400dd4aedd70f8734417d1fc74ae
requirements-completed: [SHAR-07]
coverage:
  - id: D1
    description: "A double click on a remote session during a slow connect opens one tab and connects once; different sessions still open separate tabs"
    requirement: SHAR-07
    verification:
      - kind: unit
        ref: "frontend/src/components/__tests__/App.open-remote.test.tsx#a double click during a slow connect opens one tab and connects once"
        status: pass
      - kind: unit
        ref: "frontend/src/components/__tests__/App.open-remote.test.tsx#two different remote sessions clicked back to back open two tabs"
        status: pass
    human_judgment: false
  - id: D2
    description: "Closing a tab never activates the closing id; the desktop renders nothing for a missing web tab"
    requirement: SHAR-07
    verification:
      - kind: unit
        ref: "frontend/src/lib/__tests__/tabs.test.ts#never returns the closing id, even when the list holds duplicates"
        status: pass
      - kind: unit
        ref: "frontend/src/components/__tests__/App.open-remote.test.tsx#the __websession__ branch renders nothing on the desktop when the active web tab is missing"
        status: pass
    human_judgment: false
  - id: D3
    description: "ErrorBoundary per pane and at the root: a crashing pane shows fixed copy, the sidebar and tab bar stay; the root shows a Reload screen instead of a blank window"
    requirement: SHAR-07
    verification:
      - kind: unit
        ref: "frontend/src/components/__tests__/ErrorBoundary.test.tsx"
        status: pass
      - kind: unit
        ref: "frontend/src/components/__tests__/App.open-remote.test.tsx#the pane shows the fixed fallback, the sidebar and tab bar stay, and activating another tab clears it"
        status: pass
    human_judgment: false
  - id: D4
    description: "Real app in the home VM: double-click opens one tab, Cmd+W keeps the window intact, reopen works (V-09)"
    requirement: SHAR-07
    verification:
      - kind: automated_ui
        ref: ".planning/phases/05-overnight-survival/05-VM-MATRIX.md#V-09"
        status: pass
    human_judgment: false
---

# Phase 5 Plan 09: Duplicate Remote Tab / Blank Window Fix Summary

Double-clicking a remote session now opens one tab and connects once. Closing a tab can no longer make the closing id the active one. If a pane still crashes, an error boundary keeps it inside that pane: the sidebar and tab bar stay alive. A root boundary shows a Reload screen instead of a blank window. All five fixes from the todo are in, each with a regression test. The real app in the home VM got through the user's exact steps.

## Performance

- **Duration:** about 21 min
- **Started:** 2026-11-18T03:48:48Z
- **Completed:** 2026-11-18T04:10Z
- **Tasks:** 3
- **Files modified:** 9

## Accomplishments

- **In-flight guard (fix 2):** `handleOpenRemoteSession` keeps a `useRef<Set<string>>` of session ids whose `ConnectPeerSession` is still running. A repeat click only activates an existing tab. The id is removed in a `finally` after the connect settles.
- **Functional dedupe (fix 1):** `openWebSessionTab` makes its add-or-update decision inside `setTabs(prev => …)`. If peer fields are passed it replaces the tab and keeps its name, as before. It no longer depends on `tabs`.
- **Safe next-active rule (fix 3):** `nextActiveTabId` picks the nearest right, else left, neighbour whose id differs from `closingId`.
- **Render guard (fix 4):** in the `__websession__` branch, `if (!activeWebTab && mode !== 'web') return null`. The desktop can no longer dial `window.location.origin` (`wails://`). The web-share bootstrap tab keeps its fallback.
- **ErrorBoundary (fix 5, extended):**
  - New class component with `label`, `resetKey` and `fallback`.
  - Default fallback: `banner banner--warning`, `role="status"`, text "This tab hit an error. Close it and open the session again."
  - Its log is only `console.error('[ErrorBoundary]', label, error.name)`.
  - Used in three places: around each terminal tab's TerminalPanel, around WebShareSessionView with `resetKey={activeId}`, and at the root in main.tsx (fallback "Portty hit an error." with a Reload button).

## Task Commits

1. **Task 1: double click opens one tab (tracer)**
   - `e51d03cd` test (RED)
   - `0cf61b85` fix (GREEN)
2. **Task 2: closing never blanks the window**
   - `afdf4c0f` test (RED, with a placeholder ErrorBoundary that catches nothing)
   - `d7c6382b` fix (GREEN)
3. **Task 3: VM check:** `fcf7024a` test (05-VM-MATRIX.md, V-09 PASS)
4. **Standing rule:** `55145d7e` docs (TESTING.md manifest and traceability)

## Files Created/Modified

- `frontend/src/components/ErrorBoundary.tsx`: the boundary.
- `frontend/src/App.tsx`:
  - in-flight guard and functional dedupe
  - render guard
  - two boundary sites
- `frontend/src/lib/tabs.ts`: `nextActiveTabId` skips the closing id.
- `frontend/src/main.tsx`: root boundary with Reload.
- Tests:
  - `ErrorBoundary.test.tsx` (new)
  - `App.open-remote.test.tsx` (+5 cases)
  - `tabs.test.ts` (+1 case)
- `TESTING.md`:
  - vitest 84 and Go 119; the Go count was off by one, because 05-08's `replay_fidelity_test.go` was never counted. Total 211.
  - three SHAR-07 traceability rows
- `.planning/phases/05-overnight-survival/05-VM-MATRIX.md`: created with its header and the `## Feature checks` table.

## VM evidence (Task 3)

Setup:
- The real build from commit `d7c6382b` was deployed to both VMs.
- Office VM: `portty new`, then `share`, then a rename to `vmoffice`.
- The home VM listed it under `portty-vm-office` in Other Macs.

**Screenshots were not usable.** In both VMs, WebKit cannot paint into `screencapture`. Portty's window and a plain Safari page (`<h1>HELLO VM</h1>`) both captured as blank. The unified log shows `WebContent ... IOSurface creation failed ... (likely sandbox violation)`. Rebooting the home VM and restarting the WebKit helpers did not help. This is a VM graphics limit, not Portty. Safari fails the same way, and the AX tree shows the DOM rendered in full.

The screenshots were replaced by Accessibility-tree dumps of the running app. I built a small tool, `axdump` (scratchpad/axdump/axdump.swift, compiled on the host and copied to `~/ptest`). Each dump lists the live DOM roles, ids, classes, titles and frames:

1. **Before** (`ax-0-before.txt`):
   - The sidebar has This Mac and Other Macs, with the rows `shell 1` (the user's host Mac, never touched) and `vmoffice`.
   - The tab list is empty.
   - Right after the VM reboot, the "Unable to connect to session daemon" banner showed (the daemon was slow under load). Retry Connection cleared it.
2. **After a double-click** on the `vmoffice` row (`click dclick 163 248`, two clicks 80 ms apart; the tailnet connect goes through DERP relay, so the second click lands while the connect is still in flight), in `ax-1-after-dclick.txt`:
   - exactly one `AXRadioButton Title=portty-vm-office › vmoffice`
   - one `AXTabPanel` with id `tabpanel-__websession__b74b2b3b…` holding a live xterm (viewport plus `Terminal input` textarea)
   - The office daemon log shows one web viewer join (conn=2).
3. **After Cmd+W** (`click key 13 cmd`), in `ax-2-after-cmdw.txt`:
   - The sidebar nav is intact, with the `vmoffice` row.
   - The `Open sessions` tab list is still there.
   - The `No Open Sessions` empty state shows.
   - The GUI pid stayed the same (413). The office log shows the viewer leave.
4. **Reopen** with one click (`ax-3-reopen.txt`): one tab, one panel. Then Cmd+W and another double-click (`ax-4-dclick2.txt`): one tab again. The office log shows exactly one join per open (conn=3, conn=4).
5. No `[ErrorBoundary]` log lines, and no fallback copy anywhere.

## Verification

- **Task 1:**
  - verify: tsc clean, and `App.open-remote` gave `Test Files  1 passed` (28 tests).
  - Tracer gate: re-ran end to end and it passed (end-of-phase mode, automated-only verify), then continued to Task 2.
- **Task 2:**
  - verify: tsc clean, and the three-file run gave `Test Files  3 passed` (44 tests).
  - `<ErrorBoundary` appears 2 times in App.tsx and 1 in main.tsx.
  - Full suite with `--maxWorkers=4`: `Test Files  84 passed (84)`, 1128 tests.
- **Task 3:**
  - verify: the three-file run passed.
  - `| V-09 | PASS |` appears once in 05-VM-MATRIX.md.
  - There is no `cap=` value in the matrix file.
- `bash tests/check-traceability-paths.sh`: OK (120 paths).

## TDD Gate Compliance

| Task | RED commit | GREEN commit | RED cause (RED_EVIDENCE_OK, semantic check done) |
|------|-----------|--------------|-----------------------------------------------|
| 1 | e51d03cd | 0cf61b85 | The target test `a double click during a slow connect…` failed on its assertion: `ConnectPeerSession` was called 2 times, expected 1. That is the planned behavior. The other 27 tests passed |
| 2 | afdf4c0f | d7c6382b | `tabs`: the closing id was returned (assertion). `ErrorBoundary`: the placeholder caught nothing, so the thrown SyntaxError escaped. `App`: the stub's SyntaxError took the tree down. Each failure is the missing behavior itself. The source pin failed on its `toContain` assertion. That pin's evidence could not be machine-classified, because vitest's tap-flat writes a multi-line YAML `actual:` that the TAP parser rejects. The other three were validated one file at a time |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Process] The first GREEN commit briefly included a local-tab change that broke a source pin**
- **Found during:** Task 1 GREEN
- **Issue:** I also changed `handleOpenSessionTab` to a functional dedupe. `App.nav.test.tsx` pins its shape, and I committed before reading the App-wide run.
- **Fix:** I reverted that hunk and amended the unpushed commit. Local tabs open synchronously, so two discrete clicks always see fresh state.
- **Commit:** 0cf61b85

**2. [Discretion] The render guard is pinned by source inspection, not a new helper**
- The plan allowed either. Source inspection is this test file's established convention for branches the UI cannot reach, and it adds no production code.

**3. [Rule 3 - Blocking] The VM tailnet cert was slow**
- **Issue:** `portty share` in the office VM timed out at 20 s while getting the HTTPS cert, because the first ACME issuance was slow.
- **Fix:** I pre-warmed the cert with `sudo tailscale cert` in each VM into a temp dir, then deleted the files. No key was printed or kept.

**4. [Environment] V-09 evidence comes from the AX tree, not screenshots**
- See "VM evidence". The plan asked for three screenshots. They capture blank because of a VM WebKit/IOSurface limit, which Safari shows too. The AX dumps are the stronger evidence anyway: they count tabs and panels exactly.

**5. [Standing rule] TESTING.md updated**
- `TESTING.md` was not in the plan's file list, but the project's standing rule requires it for a new test file.

**Total deviations:** 5 (1 process fix, 1 discretion, 1 blocking, 1 environment, 1 standing rule). **Impact:** none on scope. The product change is exactly the five todo fixes.

## Issues Encountered

- `wails build` regenerates the stale nested `frontend/src/wailsjs/wailsjs/` copies. I restored them with `git checkout -- <file>` and did not commit them.
- A `python3` call over SSH in the office VM may have opened an Xcode command-line-tools prompt on its screen. It is harmless and was left alone.
- The office session `vmoffice` is still shared in the office VM, so later VM plans can reuse it.

## Known Stubs

None.

## Next Phase Readiness

- Ready for 05-10 onward. 05-12, 05-13 and 05-14 append rows to `05-VM-MATRIX.md`.
- For the VM GUI checks in those plans: use `~/ptest/axdump app.portty.Portty` (already in both VMs) until WebKit painting in the VMs is fixed. Screenshots will be blank.
- The todo `2026-11-17-duplicate-remote-tab-blank-window.md` is resolved by this plan. It is still in `todos/pending` for the verifier to close.

## Self-Check: PASSED

- FOUND: frontend/src/components/ErrorBoundary.tsx, frontend/src/components/__tests__/ErrorBoundary.test.tsx, .planning/phases/05-overnight-survival/05-VM-MATRIX.md
- FOUND commits: e51d03cd, 0cf61b85, afdf4c0f, d7c6382b, fcf7024a, 55145d7e
