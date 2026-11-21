---
phase: 10-input-path-reliability
plan: 05
subsystem: frontend (relay client, terminal panel, peer tabs)
status: complete
tags: [grid, resize, claim, zoom, peer-tabs, INPT-06]
requires: [10-02, 10-03]
provides:
  - "encodeClaimFrame(cols, rows) and RelayClient.sendClaim: [0x11, cols_hi, cols_lo, rows_hi, rows_lo, 0x01]"
  - "TerminalPanel: measureFit, serverGridRef, reportFit, relayout, claimFit; one follow-the-grid path for local tabs, peer tabs and the browser viewer"
  - "App keeps a desktop peer tab's font size under its tab id; WebShareSessionView forwards onFontSizeChange"
affects: [10-04 (hub parses the claim bit), 10-06 (typing claims use the reported fit), 10-07 (gate, VM rows, TESTING.md)]
tech-stack:
  added: []
  patterns: ["reports carry measureFit's result, never term.cols/rows", "claims only from a window resize (150 ms trailing debounce) or a font change on the shown panel; ResizeObserver only reports"]
key-files:
  created: []
  modified:
    - frontend/src/lib/relayClient.ts
    - frontend/src/lib/relayClient.test.ts
    - frontend/src/components/TerminalPanel.tsx
    - frontend/src/components/__tests__/TerminalPanel.scale.test.tsx
    - frontend/src/components/__tests__/TerminalPanel.test.tsx
    - frontend/src/components/__tests__/TerminalPanel.guestClose.test.tsx
    - frontend/src/components/__tests__/TerminalPanel.contextLoss.test.tsx
    - frontend/src/components/WebShareSessionView.tsx
    - frontend/src/components/__tests__/WebShareSessionView.test.tsx
    - frontend/src/App.tsx
    - frontend/src/components/__tests__/App.open-remote.test.tsx
    - frontend/src/components/__tests__/App.test.tsx
decisions:
  - "Claim frame is exactly Research Pattern 3: the 5-byte report plus payload[4] = 0x01; encodeResizeFrame unchanged; no version check"
  - "sendClaim is dropped unless open, like sendInput (reconnect state 'open' and readyState OPEN); sendResize keeps its readyState-only check"
  - "No optimistic xterm resize on a claim: the panel always shows the server grid and follows the claim's 0x02 echo"
  - "A font change applied when a hidden panel becomes active (a Settings family change made while it was hidden) only reports; only a change on the already-shown panel claims"
  - "measureFit returns null for an empty content box as well as zero cell metrics, so a zero-size container never reports or fits to 2x1"
metrics:
  duration: ~75m
  completed: 2026-11-20
estimate:
  tokens: 130000
  tasks: 3
actuals:
  tokens: 14900
  tasks: 3
  commits: 7
plan_head_before: 1d72a6e5d1cc1ccfffb0ab73d9af5c2a4eebd786
plan_head_after: f52099501823793cc24801a00439c3b5ac78270e
---

# Phase 10 Plan 05: Viewer side of "viewer takes over size" Summary

Every terminal panel (local tab, peer tab, browser viewer) now shows the session at the server's grid, scaled down when the grid is not its own fit. It reports only its own measured fit (on open and when its container changes), and it claims the grid with a 6-byte `MsgResize2` frame when its user resizes the window or zooms. Cmd+ / Cmd− in a desktop peer tab now changes that tab's own font size, which makes it claim.

## Wire form sent (for 10-04 and 10-07)

| Frame | Bytes | When |
|-------|-------|------|
| Report (unchanged) | `[0x11, cols_hi, cols_lo, rows_hi, rows_lo]` | `onOpen` on every (re)connect; every container change whose fit differs from the last one sent on this socket |
| Claim (new) | `[0x11, cols_hi, cols_lo, rows_hi, rows_lo, 0x01]` | a `window` `resize` on the active panel, 150 ms trailing debounce (`CLAIM_DEBOUNCE_MS`); a font size or family change on the shown panel |

Example: `sendClaim(120, 30)` sends `[0x11, 0, 120, 0, 30, 0x01]`. The claim carries `measureFit`'s result, never `term.cols/rows`.

**Wire-contract assumptions (not verified against 10-04's hub; it runs in parallel):**
- The hub reads byte 5 (payload[4]) bit 0 as the claim flag and treats a 4-byte payload as a report (Research Pattern 3).
- The hub echoes a claimed (or bootstrap-adopted) grid back as a 0x02 frame to the claiming viewer too. The client never resizes xterm on its own after a claim; without the echo the claimer stays at the old grid. If 10-04 skips the echo to the claimer, this is a phase finding for 10-07.
- The local relay sends no grid frame at join while the grid was never set (10-04's `GridKnown` guard). Until then, a brand-new local tab follows the hub's 220x50 fallback push for a moment, then its report sets the grid (the old min-among-local hub adopts it and broadcasts it).

## Panel model (TerminalPanel.tsx)

- `measureFit(term)`: the measuring half of `fitTerminal`. It returns null when the cell size is 0, there is no parent, or the content box is empty. Optional chaining on `_core`. `fitTerminal` now measures through it.
- `serverGridRef`: the last 0x02 grid, null before any. Every panel passes `onResize`, which stores the grid and calls `applyHostGrid(term, cols, rows, recomputeScale)`. A restored peer snapshot sets it from the snapshot's cols/rows.
- `reportFit()`: sends `sendResize(fit)` when the fit is measurable and differs from the last one sent. `onOpen` (every panel) clears the last-sent value first, so every (re)connect reports once.
- `relayout()`: runs on activation (the rAF `tryFit` loop), on every ResizeObserver fire and in the theme effect. With no server grid it fits xterm (today's host path), otherwise it calls `recomputeScale()`. Then it reports.
- `claimFit()`: sends `sendClaim(fit)` and records the fit as the last sent.
- Removed: the `term.onResize` → `sendResize` wiring and the guest/host split in the `[isActive]` effect. `recomputeScale` toggles the scroll-x class and sets the element width for every panel. `isGuestRef` now gates only the session-ended banner. Inside the mount effect, `isGuest` still gates `onClose` handling and the reconnect verdict.

## Tests added or changed (for 10-07's TESTING.md)

| File | Cases |
|------|-------|
| frontend/src/lib/relayClient.test.ts | `encodeClaimFrame and sendClaim (INPT-06)`: "encodeClaimFrame is [MSG_RESIZE2, cols_hi, cols_lo, rows_hi, rows_lo, 0x01]", "sendClaim(120, 30) on an open socket sends exactly [0x11, 0, 120, 0, 30, 0x01]; sendResize still sends 5 bytes", "sendClaim on a closed socket sends nothing"; in `RelayClient reconnect mode (SHAR-07)`: "sendClaim is dropped while reconnecting or not yet open, and sent once open (INPT-06)" |
| frontend/src/components/__tests__/TerminalPanel.scale.test.tsx | new describe `INPT-06 TerminalPanel — report the fit, follow the grid, claim on zoom`: "local tab: onOpen reports the measured fit (100x30), not the 120x40 grid it shows", the same for "peer tab (remote)" and "browser viewer (wsURL)", "local tab follows a 120x40 grid frame: term.resize(120, 40) and a scale below 1", "local tab with no grid frame yet fits itself and reports (the hub bootstraps from it)", "three window resizes within 150 ms on the active panel send one claim, with the fit measured after the last", "an inactive panel sends no claim on a window resize", "a panel that becomes inactive stops claiming on window resizes", "a container change (banner, sidebar) reports the new fit once and never claims; an unchanged fit sends nothing", "a theme change on a panel following a 120x40 grid does not resize xterm to its own fit", "a font size change on the active panel claims its new fit once, without resizing xterm before the next grid frame". Source gates, new: "every panel's onResize stores the server grid and goes through applyHostGrid with recomputeScale (05-12, INPT-06)", "reports carry the measured fit, never the grid shown (INPT-06 invariant)", "relayout fits only before any grid is known, otherwise rescales, then reports". The host describe is renamed `Phase 157 TerminalPanel — local tab before any grid frame (VIEW-04/05)`. |
| frontend/src/components/__tests__/TerminalPanel.test.tsx | changed pins: "font effect sets family and size, lays out, then claims (INPT-06)", "reports its measured fit on WebSocket open, never the grid it shows (INPT-06)" |
| frontend/src/components/__tests__/TerminalPanel.guestClose.test.tsx, TerminalPanel.contextLoss.test.tsx | RelayClient mocks gain `sendClaim() {}` (no new cases) |
| frontend/src/components/__tests__/WebShareSessionView.test.tsx | `WebShareSessionView — font size handler (INPT-06)`: "a passed onFontSizeChange reaches TerminalPanel", "without one (the browser viewer zooms with the browser), TerminalPanel still gets a callable function" |
| frontend/src/components/__tests__/App.open-remote.test.tsx | `App — peer tab font size per tab (INPT-06)`: "Cmd+ raises only that peer tab's font; a second peer tab keeps the default; a reopened tab starts from the default" (the WebShareSessionView stub gains `data-font-size` and a `fire-font-up` button) |
| frontend/src/components/__tests__/App.test.tsx | changed pin "passes the viewer font to WebShareSessionView only on the desktop" (now the per-tab size and the forwarded handler) |

Removed cases (replaced by the INPT-06 ones above): scale "guest (remote=true): sendResize is never called (onOpen is undefined or no-op)", "guest (wsURL set): sendResize is never called", "host: sendResize is called when onOpen fires (host drives PTY grid)", "host: onResize callback is not provided (host never honors server 0x02)", and the source gates "guest path: does not call client.sendResize from onOpen", "guest ResizeObserver path does NOT call fitTerminal", "host ResizeObserver path still calls fitTerminal". The old onResize gate became the new "every panel's onResize ..." gate.

## Source pins that changed

- TerminalPanel.test.tsx "sends terminal dimensions on WebSocket open to sync PTY size" (`sendResize(term.cols, term.rows)`) became "reports its measured fit on WebSocket open, never the grid it shows (INPT-06)": `onOpen` calls `reportFit(`, and the source has no `sendResize(term.cols`.
- TerminalPanel.test.tsx font-effect pin: it used to check the `isGuestRef` branch (recomputeScale, else fitTerminal). It now checks that the body has no `isGuestRef`, and that `relayout()` follows the option sets and `claimFit()` follows `relayout()`.
- App.test.tsx LOOK-01 pin: `fontSize={mode !== 'web' ? terminalFont.size : undefined}` became `fontSize={mode !== 'web' && activeWebTab ? fontSizes[activeWebTab.id] ?? terminalFont.size : undefined}`, plus the `onFontSizeChange` line.
- Kept unchanged: `fitTerminal(termRef.current!)` (now in `relayout`), `new ResizeObserver`, `MAX_ATTEMPTS = 20`, `tryFit`, rAF before the observer in the `[isActive]` effect, `}, [isActive])`, `}, [fontFamily, fontSize, isActive])`, the Shift+= / Shift+- handler, `gridForContentBox(parentW, parentH, dims.css.cell.width, dims.css.cell.height)` inside the `fitTerminal`..`interface` slice, `isGuest = remote || !!wsURL`.

## Assumption A8 (synchronous font metrics)

It holds in the tests by construction: the mock changes the cell size before the prop change, and the claim carries the new fit (90x30). jsdom cannot show what real xterm does. The code relies on A8 as today's host zoom did: it measures right after setting `term.options.fontSize`, with no extra rAF. 10-07's VM zoom check is the real test. If a claim lands one step stale there, measure inside one `requestAnimationFrame`.

## TDD record

| Task | RED | GREEN | Notes |
|------|-----|-------|-------|
| 1 (tracer) | `8cdcf21a`: 13 cases failed (4 relayClient, 9 scale) | `ceda51cf`: the 4 listed files pass, 163 tests | Tracer gate: the automated verify re-run passed, so expansion continued |
| 2 | `e62572ec`: the window-resize claim and the theme case failed | `a8583970`: the 5 listed files pass, 118 tests | The ResizeObserver report cases and the inactive-panel cases already passed at RED: Task 1's unified `[isActive]` effect already routed the observer through `relayout`, and with no listener nothing could claim |
| 3 | `f3508813`: the forwarding case and the App per-tab case failed (13 vs 14) | `a71e8a88`: both files pass, 62 tests | |
| pin fix | `f5209950` | | see Deviations 2 |

## Gate (run once)

**Result: PASS.** One full-run failure was a code pin this plan broke; it was fixed and only that file was re-run. One Go failure was the known INPT-05 flake, which passed on its single allowed rerun. Output files are in `/private/tmp/claude-501/-Users-ken-dev-portty/66f0384b-8902-40e9-9d84-68c47b57f8f8/scratchpad/`.

| Step | Result | Output file |
|------|--------|-------------|
| `pnpm vitest run` (full, once) | 1331 passed, 1 failed (App.test.tsx LOOK-01 pin, see Deviations 2), 95 files | p10-05-vitest.log |
| App.test.tsx re-run after the pin fix | PASS, 97 tests | p10-05-vitest-apptest-rerun.log |
| `pnpm exec tsc --noEmit` | PASS | p10-05-tsc.log |
| `go build ./...` / `go vet ./...` | PASS / PASS | p10-05-build.log, p10-05-vet.log |
| `go test -race -short -count=1 ./...` (once) | 14 packages ok. `internal/daemon` failed only `TestExitEvent_ListSessions_ExitCodePopulatedForStopped` ("ExitCode for running session: want nil"), the known INPT-05 flake that 10-01 is fixing | p10-05-gate-go.log |
| that single test, re-run once | PASS (`ok internal/daemon 1.043s`) | p10-05-gate-go-rerun.log |
| check-brand / check-traceability | PASS / PASS (164 paths) | p10-05-brand.log, p10-05-trace.log |
| greps: no `onFontSizeChange={() => {}}` (0); `handleFontSizeChange(activeWebTab.id` (1); no `sendResize(term.cols` and no `term.onResize(` in code (0/0); `sendClaim` in relayClient.ts; `function measureFit`; `addEventListener('resize'`; `CLAIM_DEBOUNCE_MS`; `sendClaim` in the guestClose and contextLoss mocks | PASS | |
| `git diff d027e86d -- frontend/src/wailsjs web/assets/terminal.js internal/webserver/csp_mw.go` | empty (unchanged) | |
| Go files changed since 1d72a6e5 | none (`git diff --stat` lists only frontend files) | |

The host showed no PTY pressure: 0 `device not configured` lines.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Stable theme object in the scale test helper**
- **Found during:** Task 1 GREEN
- **Issue:** `makeBaseProps` built a new `theme` object on every render. Each re-render re-ran the theme effect, which at that point still called `fitTerminal`. It resized xterm and masked the font-claim behaviour under test.
- **Fix:** One module-level `THEME` object, as App passes. Task 2's theme case then covers the theme effect itself.
- **Files modified:** frontend/src/components/__tests__/TerminalPanel.scale.test.tsx
- **Commit:** ceda51cf

**2. [Rule 1 - Bug] App.test.tsx LOOK-01 source pin not listed in the plan**
- **Found during:** Task 3 gate (full vitest)
- **Issue:** `App.test.tsx` "passes the viewer font to WebShareSessionView only on the desktop" pinned the old shared `fontSize` expression. It was the full run's only failure (1 of 1332).
- **Fix:** The pin now checks the per-tab expression and the forwarded handler, still desktop only. Only that file was re-run (97 pass). The full suite was not re-run, per the once-per-gate rule.
- **Files modified:** frontend/src/components/__tests__/App.test.tsx
- **Commit:** f5209950

**3. [Rule 2 - Correctness] A font change applied on activation does not claim**
- **Found during:** Task 1
- **Issue:** The `[fontFamily, fontSize, isActive]` effect also runs when a hidden panel becomes active, and applies a Settings family change made while it was hidden. Claiming there would let a tab switch take the size, which the plan rules out ("tab switches only report").
- **Fix:** `fontShownRef` records whether the panel was already shown. The effect lays out on every applied change but claims only when the panel was already shown.
- **Commit:** ceda51cf

**4. [Rule 2 - Correctness] measureFit returns null for an empty content box**
- **Issue:** A zero-size container (or jsdom's unset padding `''`, which `parseFloat` turns into NaN) would have reported a 2x1 or NaN grid.
- **Fix:** Padding is parsed with `|| 0` (as `recomputeScale` already did), and a non-positive content box returns null.
- **Commit:** ceda51cf

### Other notes

- Gate output files are in the session scratchpad, not `/tmp`. The paths are listed above. The plan's `/tmp/p10-05-base` is the scratchpad's `p10-05-base` (1d72a6e5).
- `CLAIM_DEBOUNCE_MS` was added in Task 2 rather than Task 1, because tsc rejects an unused constant.

## Known Stubs

None.

## Threat Flags

None. No new endpoint, DOM injection or CSP change. The claim is a new client-to-server frame on the existing socket, and 10-04 owns its validation and clamp (T-10-17 and T-10-18 are mitigated here as planned).

## Self-Check: PASSED

- All 12 modified files exist; all 7 commits (8cdcf21a, ceda51cf, e62572ec, a8583970, f3508813, a71e8a88, f5209950) are on HEAD.
