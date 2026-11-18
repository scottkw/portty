---
phase: 04-one-click-share
plan: 05
subsystem: sharing-ui
tags: [react, wails, popover, accessibility, cleanup]
status: complete

requires:
  - phase: 04-one-click-share
    provides: "04-02: GetSessionShare / SetSessionShared bindings (ShareResult, errorCode https_off / tailscale_off / start_failed / stop_failed), daemon kill path drops the share; 04-04: ConnectPeerSession stub"
provides:
  - "SharePopover: the one-switch share popover with every UI-SPEC state"
  - "lib/shareCopy.ts: fixed share copy and errorCode-to-copy mapping (also used by the sidebar banner)"
  - "Switch blocked / describedBy props; ContextMenuItem disabled / describedBy"
  - "Sidebar Share / Stop Sharing with the Tailscale-off disabled state; App menu-share failure banner"
  - "Settings ▸ Sharing is the Tailscale row plus one explanation; Security CT copy updated"
  - "Frontend free of the share dialog, its tabs, risk panel, hold-to-confirm, link cards, reminder banner and switch, web-server Settings rows, their stubs, models fields, CSS and tests"
affects: [04-06 peer tabs and web page, 04-06 removal gate (tests/check-share-removal.sh), M-81 / M-82 VM and Mac checks]

actuals:
  tokens: 120800
  tasks: 2
  commits: 2
plan_head_before: 91849fd578d975d36479263bd76be9928e3e24b8
plan_head_after: 16f61840bcdfe7fb288918e37b882d3943c16023

tech-stack:
  added: []
  patterns:
    - "Hand-written portal popover anchored on a ref'd toolbar button (createPortal, fixed position computed from getBoundingClientRect, re-placed on resize)"
    - "Inert-but-focusable controls: aria-disabled + aria-describedby to visible text (Switch blocked, ContextMenu disabled item)"

key-files:
  created:
    - frontend/src/components/SharePopover.tsx
    - frontend/src/components/__tests__/SharePopover.test.tsx
    - frontend/src/lib/shareCopy.ts
  modified:
    - frontend/src/App.tsx
    - frontend/src/components/Switch.tsx
    - frontend/src/components/TabBar.tsx
    - frontend/src/components/ContextMenu.tsx
    - frontend/src/components/Sidebar.tsx
    - frontend/src/components/SettingsSheet.tsx
    - frontend/src/components/RegenerateKeyModal.tsx
    - frontend/src/lib/remoteAdapter.ts
    - frontend/src/lib/useDialogFocus.ts
    - frontend/src/lib/webMode.ts
    - frontend/src/style.css
    - frontend/src/wailsjs/go/main/App.d.ts
    - frontend/src/wailsjs/go/main/App.js
    - frontend/src/wailsjs/go/models.ts
    - frontend/src/wailsjs/wailsjs/go/models.ts
    - TESTING.md
  deleted:
    - frontend/src/components/Share/{SessionShareModal,FunnelRiskPanel,ShareSegmentedControl}.tsx
    - frontend/src/components/SessionShare/{TailnetTab,InternetReadOnlyTab,InternetFullAccessTab,ShareLinkCard,shared}.tsx
    - frontend/src/components/ShellWebShareBanner.tsx
    - frontend/src/lib/helpUrl.ts
    - 14 test files (the 12 share-dialog tests plus SettingsSheet.shell-warn-toggle and SettingsSheet.web-link-ux)

key-decisions:
  - "SharePopover takes a `shared` prop (the session's webEnabled from the poll) in addition to the planned props: it seeds the switch so a shared session never flashes off, and a change refetches GetSessionShare so a share from the sidebar menu or `portty share` updates an open popover"
  - "The popover stays open when Tailscale drops (UI-SPEC states) instead of App closing it; the old close-on-disconnect effect is gone"
  - "Esc, outside pointerdown and window blur are handled inside SharePopover; App closes it on activeId change and when Settings opens"
  - "Sidebar derives Tailscale-off from its existing tailscaleHealth prop instead of taking a new tailscaleOff prop"
  - "The toolbar Share button keeps its Share / Shared look when Tailscale is off (UI-SPEC: 'its current look'); it used to always read Share"
  - "The popover's reason line has its own id (share-popover-reason) so it never duplicates the banner's ts-off-reason id"
  - "The shared .dialog--exiting close-fade rule stays: it is the Phase 2 dialog-system rule, even though the removed share dialog was its only user"
  - "--hub-radius-md and --hub-font-size-sm are now consumed by the popover; --hub-radius-sm and --hub-font-weight-emphasis lost their last consumers and stay declared (UI-SPEC: delete no token)"

patterns-established:
  - "Removed bindings are pinned by negative source assertions naming SetSessionShared (the live binding) rather than the deleted names"

requirements-completed: []

coverage:
  - id: D1
    description: "Share button opens a 320px non-modal popover 8px under it with a centred arrow and exactly one switch"
    covered_by: [frontend/src/components/__tests__/SharePopover.test.tsx, frontend/src/components/__tests__/TabBar.test.tsx]
  - id: D2
    description: "Off / starting / on / stopping / start-failed / stop-failed / copy-failed / Tailscale-off states and copy"
    covered_by: [frontend/src/components/__tests__/SharePopover.test.tsx]
  - id: D3
    description: "Sidebar menu Share / Stop Sharing, disabled Share with Tailscale off"
    covered_by: [frontend/src/components/__tests__/Sidebar.test.tsx, frontend/src/components/__tests__/ContextMenu.test.tsx]
  - id: D4
    description: "Settings ▸ Sharing shrunk, Security CT copy"
    covered_by: [frontend/src/components/__tests__/SettingsSheet.tailscale-status.test.tsx, frontend/src/components/__tests__/SettingsSheet.test.tsx]
  - id: D5
    description: "Popover look in Light/Dark, long link scrolls and copies in full, Copy never wraps"
    covered_by: [TESTING.md M-81 (backstop, VM)]

metrics:
  duration: 36min
  completed: 2026-11-16
---

# Phase 4 Plan 05: One-Switch Share Popover Summary

The toolbar Share button now opens a hand-written portal popover with one "Share this session" switch on GetSessionShare/SetSessionShared. When shared it shows the full link, a Copy button and the locked warning line. The sidebar menu gained Share / Stop Sharing. Settings ▸ Sharing is down to the Tailscale row plus one explanation. The old share dialog, its tabs, the risk panel, hold-to-confirm, link cards, the share reminder, the web-server Settings rows and their stubs, models fields, CSS and tests are all deleted (57 files, +1211 / −6952).

## Popover prop contract

```ts
interface SharePopoverProps {
  sessionId: string
  sessionName: string
  shared: boolean            // added: webEnabled from the sessions poll
  anchor: HTMLElement | null // the toolbar Share button (TabBar shareButtonRef)
  popoverId: string          // 'share-popover'; TabBar aria-controls
  tailscaleOff: boolean
  tailscaleDomain: string    // tailscaleHealth.domain, used in the CT line
  onClose: () => void
  onSharedChange: (sessionId: string, shared: boolean) => void // App setSessionWebEnabled
}
```

## Close triggers as implemented

| Trigger | Where | Focus |
|---------|-------|-------|
| Share button click / Enter / Space | App toggles `sharePopoverOpen` (pointerdown on the anchor is ignored by the popover) | stays on the button |
| Esc | SharePopover document keydown | back to the button |
| Pointerdown outside popover and button | SharePopover (capture phase) | unchanged |
| Window blur (loses key status) | SharePopover window `blur` | unchanged |
| Active tab changes (incl. a peer tab, or the session ending, which moves the active tab) | App effect on `activeId` | n/a |
| Settings opens | App effect on `showSettings` | n/a |
| Active session no longer in the list | the render guard (`sessions.find(activeId)`) renders nothing | n/a |

A pending start or stop keeps running after close and still calls `onSharedChange`.

## TESTING.md

- Retired: Category A (M-01, Share Modal), Category J (M-16, share reminder switch), Category T (M-37..M-40, public-sharing GUI).
- Added: Category AR with **M-81** (Share popover in the Tart VM) and **M-82** (Share controls follow Tailscale, carrying M-40 step 1).
- 41 Section 4 rows of deleted files removed, stale rows rewritten, 6 SHAR-01 rows added. Section 5 items that still described the dialog (M-42, M-44, M-63, M-64, M-65, M-66, M-67 and a NET-01 step) were rewritten.
- Counts: vitest 93 → 80, total 210 → 197.

**M-81 result:** not run by this executor (the orchestrator runs it in the Tart VM with the vmpreview build).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] The VM item is M-81, not M-78**
- **Found during:** Task 1 (TESTING.md)
- **Issue:** 04-02 already used M-78 ("Share from the CLI…"), and 04-06 has M-79 and M-80 booked. The plan's "baseline 0" for `**M-78**` was wrong.
- **Fix:** The popover VM check is M-81. The acceptance grep `grep -c '\*\*M-78\*\*'` still prints 1 (04-02's item). 04-06's text that calls the popover check "M-78" should read M-81.
- **Commit:** 08532ab2

**2. [Rule 1 - Scope of gate] The Task 1 grep matches the Sidebar's GlobeAltIcon**
- **Found during:** Task 1 verify
- **Issue:** The verify grep bans `GlobeAltIcon` everywhere in frontend/src. Sidebar.tsx uses it as the "Other Macs" peer-group header icon, a kept feature that has nothing to do with the removed tab exposure icons. UI-SPEC lists only the tab icons as removed.
- **Fix:** The Sidebar icon stays (Chesterton). The tab icons are gone from TabBar.tsx and its test checks they are absent. The stale GlobeAltIcon comments in remoteAdapter.ts were reworded. Every other alternative in the Task 1 grep matches nothing; the Task 2 grep exits 1.
- **Commit:** 08532ab2, 16f61840

**3. [Rule 2 - Correctness] SharePopover gets a `shared` prop**
- **Issue:** With only the planned props, a shared session's popover would show the switch off until GetSessionShare answered. It also could not follow a share made from the sidebar or the CLI while it was open (UI Considerations: share state sync).
- **Fix:** `shared` seeds the switch, and a change refetches GetSessionShare.
- **Commit:** 08532ab2

**4. [Rule 1 - Dead code] lib/helpUrl.ts deleted**
- **Issue:** HELP_URL's only user was the removed share dialog's help link. The popover has no help link (UI-SPEC: no footer).
- **Fix:** The file was deleted, and App.nav.test now checks that App has no help-link handler.
- **Commit:** 08532ab2

**5. [Rule 1] Negative test pins on removed bindings**
- **Issue:** App.shellExit, App.test and TabBar tests asserted `not.toContain('ToggleWebServing')`, which the Task 2 grep forbids. They also became vacuous once the stub was deleted.
- **Fix:** They now pin `SetSessionShared`, the live binding that closing a tab or session exit must not call.
- **Commit:** 16f61840

**6. [Rule 1] RegenerateKeyModal override test**
- **Fix:** The test now uses End Session copy, because End Session is still a real copy-override consumer.
- **Commit:** 16f61840

**7. SettingsSheet.persistence.test.tsx not edited**
- **Reason:** It has no sharing, web-server or reminder references, so nothing needed to change.

## Known Stubs

None.

## Threat Flags

None. T-04-22 is mitigated: error copy is chosen by errorCode in shareCopy.ts, the only interpolated detail comes from the local binding, and it renders as React text. T-04-23 is mitigated: the button, switch and menu item are aria-disabled with Tailscale off, and SharePopover.test.tsx and Sidebar.test.tsx cover it. No packages were added.

## Verification

- `pnpm -C frontend exec tsc --noEmit && pnpm -C frontend exec vitest run`: 80 files, 1055 tests passed.
- `go build ./... && go vet ./... && go test -race -short ./...`: green. Both tagged vets (playwrightfixture, vmpreview) are clean. `pgrep -f '\.test daemon'` prints nothing.
- `bash tests/check-traceability-paths.sh && bash tests/check-brand.sh`: OK.
- `wails build -platform darwin/arm64 -tags wailsassets` built. The regenerated wailsjs/wailsjs/go/models.ts added only TerminalFont, PeerConnectResult and ShareResult and none of the removed types, which confirms the stub and model removals match Go. It was restored with `git checkout -- frontend/src/wailsjs/wailsjs/`.

## Self-Check: PASSED

- FOUND: frontend/src/components/SharePopover.tsx, frontend/src/components/__tests__/SharePopover.test.tsx, frontend/src/lib/shareCopy.ts
- FOUND: 08532ab2, 16f61840 (ancestors of HEAD)
