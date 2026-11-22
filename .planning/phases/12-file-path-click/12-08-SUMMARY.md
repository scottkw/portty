---
phase: 12-file-path-click
plan: 08
subsystem: frontend-viewer
tags: [xterm, linkifier, path-click, peer-tab, gap-closure]
requires:
  - phase: 12-file-path-click
    provides: "12-06 viewer path links and capability bit; 12-VERIFICATION gaps 1 and 2"
provides:
  - "PathLinksHandle.invalidate(): link state refreshed on every capability report and every terminal resize"
  - "Desktop peer tabs register path and URL links from this Mac's own link settings, no cross-origin request"
affects: [12-09]
tech-stack:
  added: []
  patterns:
    - "Real xterm 6 Terminal in jsdom (stubbed char-measure size) driven through its own Linkifier by DOM mouse events"
key-files:
  created:
    - frontend/src/lib/__tests__/pathLinks.linkifier.test.ts
    - frontend/src/components/__tests__/WebShareSessionView.peerLinks.test.tsx
  modified:
    - frontend/src/lib/pathLinks.ts
    - frontend/src/components/TerminalPanel.tsx
    - frontend/src/App.tsx
    - frontend/src/components/WebShareSessionView.tsx
    - frontend/src/lib/__tests__/pathLinks.test.ts
    - frontend/src/components/__tests__/TerminalPanel.pathClick.test.tsx
    - frontend/src/components/__tests__/App.open-remote.test.tsx
    - TESTING.md
key-decisions:
  - "Peer tabs use this Mac's own webLinks and webLinksConfig only (user-ratified 2026-11-21): no CORS header, no Go route, no cross-origin fetch"
  - "Link refresh resets xterm's private Linkifier fields behind a presence guard and re-hovers with a non-bubbling mousemove, so a mouse-tracking program gets no input"
requirements-completed: []
status: complete
duration: ~45 min
completed: 2026-11-21
commits: 6
plan_head_before: c8287c4dc5e586da3dfe8d2f318544b2d48b5899
plan_head_after: 0a76d02b
actuals:
  tokens: 8500
  tasks: 3
  commits: 6
---

# Phase 12 Plan 08: Viewer link gaps Summary

A surviving viewer keeps its clickable paths and URLs after a reconnect or a grid resize (xterm Linkifier state refreshed), and a desktop peer tab gets path and URL links from this Mac's own link settings instead of a refused cross-origin config fetch.

## Commits

| Task | Commit | Message |
|------|--------|---------|
| 1 RED | 523fb5d5 | test(12-08): reproduce reconnect loss of path links in a real xterm Linkifier |
| 1 GREEN | 8fd350e6 | fix(12-08): refresh xterm link state whenever the path-click capability is reported |
| 2 RED | 1823a698 | test(12-08): reproduce resize loss of path and URL links in a real xterm Linkifier |
| 2 GREEN | 290e5860 | fix(12-08): refresh xterm link state after every terminal resize |
| 3 | f1afc2e7 | fix(12-08): give desktop peer tabs this Mac's link settings |
| 3 docs | 0a76d02b | docs(12-08): TESTING.md Sections 2 and 4 for the gap tests |

## RED evidence (detached worktree, node_modules symlinked; no frozen install needed)

Log directory (private, mktemp -d): `/var/folders/lb/2__vrh3n2n155kwhrvz08lv80000gn/T/tmp.xZH3Kl4ygj` (red.json, red2.json, vitest.log, tsc.log, trace.log, go.log).

RED 1 (523fb5d5), both reproduced:
- passed: `control: a Cmd-click on a printed path sends one path click`; `while the capability is false, no link is offered and nothing is sent`
- failed: `after a reconnect, a Cmd-click at the same cell sends the path click with no pointer move`: `AssertionError: expected [] to deeply equal [ 'docs/diagram.png' ]`
- failed: `after a reconnect, another cell of the same row offers the link`: `AssertionError: expected [] to deeply equal [ 'docs/diagram.png' ]`

RED 2 (1823a698), run with Task 1's fix in the tree:
- passed: control, both reconnect tests, `the link refresh sends no input to a program with mouse tracking on`, `xterm 6 still has the Linkifier fields the refresh resets`, read-only guard
- failed: `after a resize, a Cmd-click at the same cell sends the path click with no pointer move`: `expected [ 'docs/diagram.png' ] to deeply equal [ 'docs/diagram.png', …(1) ]`
- failed: `after a resize while the pointer was outside, returning to the same cell offers the link`: same message
- failed: `after a resize, a Cmd-click on a URL at the same cell opens it`: `expected [ [ 'https://example.com/docs' ] ] to deeply equal [ Array(2) ]`

Every scenario reproduced; none is "not reproduced". Both mechanisms (reconnect cache poisoning, resize same-cell memory) match the planner's probe.

## Fix shapes committed

- **pathLinks.ts**: `registerPathLinks` returns `PathLinksHandle` (`IDisposable & { invalidate() }`). `invalidate()` queues a microtask (after dispose it does nothing; it runs after xterm's own resize listener), resets `_lastBufferCell` to undefined and `_activeLine` to -1 on `_core.linkifier`, and, if the pointer is inside `.xterm-screen`, dispatches a non-bubbling `mousemove` with the last pointer's coordinates and modifiers. A `mouseleave` listener tracks the pointer; the module's own event is ignored by its capture listener. `term.onResize(invalidate)` covers resizes. `provideLinks` is unchanged.
- **TerminalPanel.tsx**: `onPathClickCapability` sets the flag then calls `pathLinksRef.current?.invalidate()` (every replay meta, close and abandon).
- **App.tsx / WebShareSessionView.tsx**: `peerPluginConfig = daemon.PluginSettings.createFrom({ webLinks, webLinksConfig })` memoized on `pluginConfig`, passed to remote peer tabs; `isWebGuest = pluginConfig === undefined && remote !== true`, so a remote tab never self-fetches `/api/plugin-config` or opens its EventSource. The browser viewer at `/app/` still self-fetches from its own origin.

## Peer-tab config decision (for 12-09's security review)

User-ratified 2026-11-21 ("This Mac's setting"). Only `webLinks` and `webLinksConfig` cross over; webgl, clipboard, search, serialize and progress stay off (pinned by `a peer tab follows this Mac's web-links toggle and modifier and turns on no other addon`). No CORS header was added, no Go file changed, CSP files unchanged (guards pass). The host still enforces the printed-token check and write capability on every path click. It removes a failing cap-bearing cross-origin request and its retry loop (T-12-41). Mutation check: reverting either the App or the WebShareSessionView change fails three of the new tests.

## Deviations from Plan

1. **[Rule 1 - Bug] Linkifier field guard.** The plan said production code requires both `_lastBufferCell` and `_activeLine` to be present. In xterm 6.0.0's bundle `_lastBufferCell` is declared without an initializer and is not an own property until the first hover, so a "both present" guard would skip a valid refresh. The guard checks `'_activeLine' in linkifier` (initialized to -1 in the constructor) and assigns both. The pin test hovers first and then asserts both fields exist, so a rename in an xterm upgrade still fails loudly. Commit 8fd350e6.
2. **[Rule 1 - Bug] Existing source gate.** Task 1 changed TerminalPanel's `import { registerPathLinks }` line to also import the handle type, which broke `TerminalPanel.web-links.test.tsx` (a source regex for the exact import line; not in Task 1's verify file list). Fixed in Task 2's GREEN commit (290e5860) by keeping the value import line unchanged and adding a separate `import type { PathLinksHandle }` line.
3. **Counts.** TESTING.md totals are vitest 108 and total 279 (not 278): the selection-offset debug session had already added a Playwright spec (8 files). Go 162, build-script 1 unchanged.
4. Task 1 RED/GREEN commits are as planned; `state.record-session` skipped per the dispatch instruction.

## Verification

- vitest full run: 108 files, 1600 tests, all pass. tsc --noEmit clean. `bash tests/check-traceability-paths.sh`: OK, 199 paths.
- `GOTOOLCHAIN=go1.26.11 go test -race -short ./...`: all 14 packages ok (no Go change in this plan).
- Plan guards: no `Access-Control-Allow-Origin` in non-test `internal/webserver`; `frontend/index.html` and `internal/webserver/csp_mw.go` identical to 81ada526; no `.go` file changed since the plan's first commit.

## Known Stubs

None.

## Not verified

- No real Safari, Wails web view or tailnet peer run: the jsdom rig stubs xterm's char measurement (8 x 16 cells) and mouse geometry. 12-09 re-runs VM rows C-02, C-04, C-09 and U-01.
- The reconnect test feeds the replay meta and the offset meta in separate steps with a render wait; on a real browser the same ordering holds (separate message tasks), but the exact timing against real renders is only checked in the VM.

## Threat Flags

None. The synthetic `mousemove` is non-bubbling (T-12-40, pinned by the zero-onData test); no new endpoint, header or auth path.

## Self-Check: PASSED

All six commits exist on gsd/v1.2; both new test files exist; `.planning/phases/12-file-path-click/12-08-SUMMARY.md` written.
