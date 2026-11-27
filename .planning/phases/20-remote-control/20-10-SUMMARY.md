---
phase: 20-remote-control
plan: 10
subsystem: host-ui
tags: [settings, tray, daemon-status, consent-copy, accessibility, surface]
requires:
  - phase: 20-02
    provides: "Manager.ControlState() (measured control: ok or needs_accessibility)"
  - phase: 20-03
    provides: "Manager.CurrentSurface() (display or window, with the picker's name)"
  - phase: 20-09
    provides: "20-VM-CHECKS.md verdict PASTE_ROW: none"
  - phase: 19
    provides: "GET /desktop, desktopTrayLine, Settings Desktop group, DesktopSetupSheet, OpenDesktopPermissionPane"
provides:
  - "GET /desktop additive surface {kind,name} and control, set only while viewers are connected"
  - "tray line suffix ', window only' for a single shared window"
  - "Settings Showing row, Permissions row control branch, consent copy that says control"
affects: [20-11, 21, 22]
tech-stack:
  added: []
  patterns:
    - "additive daemon status fields; older readers ignore them"
    - "measured control state overrides the cached permission preflight in the GUI"
key-files:
  created: []
  modified:
    - internal/daemon/desktop_api.go
    - internal/daemon/desktop_api_test.go
    - internal/daemon/desktop_permissions_test.go
    - tray_common.go
    - tray_common_test.go
    - app_desktop.go
    - app_desktop_test.go
    - frontend/src/wailsjs/go/main/App.d.ts
    - frontend/src/components/SettingsSheet.tsx
    - frontend/src/components/__tests__/SettingsSheet.desktop.test.tsx
    - frontend/src/components/DesktopSetupSheet.tsx
    - frontend/src/components/__tests__/DesktopSetupSheet.test.tsx
key-decisions:
  - "PASTE_ROW: none, so no Paste from Other Apps row, no paste field in GET /desktop/permissions, and no third pane kind; tests pin all three absences"
  - "surface and control are filled only while viewers > 0: the control state is measured by effect and never resets when the last viewer leaves, so reporting it with nobody watching could leave Settings stuck on a stale warn"
  - "The Accessibility how-to keeps Phase 19's merged pane name 'Device Control and Data Access (called Accessibility on earlier macOS)' and takes the new UI-SPEC tail; the plan's pinned lead ('then Accessibility') is stale against 19-UI-SPEC"
requirements-completed: [DESK-05, DCTL-06, DCTL-01]
status: complete
duration: ~35m
completed: 2026-11-26
estimate:
  tokens: 70000
  raw_tokens: 70000
  tasks: 3
  confidence: low
actuals:
  tokens: 9500
  tasks: 3
  commits: 3
plan_head_before: 5293739642d805dcbb92168785e183bed550364a
plan_head_after: 72ec674197852ecf5c977145c276acadc2bd3718
---

# Phase 20 Plan 10: The shared Mac shows what is shared and whether control works Summary

The host now reports what is being shared and whether remote control works (daemon status, tray suffix, Settings Showing row, Permissions warn), and every consent string says the link can control the Mac.

## What was built

- **Task 1 (tracer, ec2de385):** `DesktopStatusResponse` gained `Surface *DesktopSurface` and `Control string`, both `omitempty` and set only while `viewers > 0` (from `mgr.CurrentSurface()` and `mgr.ControlState()`). `desktopTrayLine` appends ", window only" for the `streaming` state with a window surface; `locked` is unchanged. Tests: `TestDesktopAPI_StatusSurfaceAndControl` drives a real window pick over the viewer socket (a source wrapper that accepts windows plus a fake `ListSurfaces` through `desktopTestDeps`), and `TestDesktopTrayLine_WindowOnly` is a table. Tracer gate re-run end to end before expanding.
- **Task 2 (0a95e689):** `DesktopStatusResult` gained `SurfaceKind`, `SurfaceName`, `Control` (mapped in `GetDesktopStatus`, typed in `App.d.ts`). `SettingsSheet.tsx`: a `Showing` row directly under Status (Caption via the existing `.settings-row__description`, `aria-live="polite"`, "Main Display", "Display 2" or "Window: <name>"), absent with nobody watching; the Permissions row turns warn with "Control needs Accessibility. Choose Set Up Permissions…" when `control` is `needs_accessibility`, and returns to Granted when it is `ok`; Row 1 and Row 3 strings changed.
- **Task 3 (72ec6741):** setup sheet intro and Accessibility how-to changed; PASTE_ROW branch below.

## PASTE_ROW branch taken

`PASTE_ROW: none` (20-VM-CHECKS.md line 87; the launchd daemon raised no paste-privacy alert and `accessBehavior` read `alwaysAllow`). Nothing from the ship branch was built: no third setup row, no `paste` field on `GET /desktop/permissions`, no "paste" pane kind, no System Settings path. Row 4's description stays "Screen Recording and Accessibility." Absence is pinned by `TestOpenDesktopPermissionPane` (added "paste" to the refused kinds), `TestDesktopPermissions_NoPasteField` (answer has exactly `sr` and `ax`) and a `not.toContain('Paste from Other Apps')` assertion in the setup sheet test.

## Lines replaced (grep of the four old strings, as the cross-phase note asked)

| Old string | Was at | Now |
|-----------|--------|-----|
| "Open this Mac's screen from your other devices any time…" | SettingsSheet.tsx:93 (`DESKTOP_OFF_DESCRIPTION`, the only place; the empty-state body is the same constant) and SettingsSheet.desktop.test.tsx:174 | "View and control this Mac's screen…" |
| "…Treat it like a password." | SettingsSheet.tsx:358 (Link row) | "…It can control this Mac, so treat it like a password." |
| "Portty needs two permissions to show this Mac's screen…" | DesktopSetupSheet.tsx:111, DesktopSetupSheet.test.tsx:268 | "…to show and control this Mac's screen…" |
| Accessibility how-to tail "Portty uses it to tell a still screen from an unapproved one. Asked now so a later update does not need another trip to this Mac." | DesktopSetupSheet.tsx:124, DesktopSetupSheet.test.tsx:26 | "Portty uses it to let your other devices control this Mac, and to tell a still screen from an unapproved one." |

Both Phase 19 merges (19-13, 19-14) were present, so the precondition held and no names had changed.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Plan's pinned Accessibility how-to would regress a VM-verified pane name**
- **Found during:** Task 3
- **Issue:** The plan (and the Phase 20 UI-SPEC "Old" column) pins the how-to lead "then Accessibility". The merged Phase 19 sheet, and 19-UI-SPEC line 451, say "then Device Control and Data Access (called Accessibility on earlier macOS)", the name verified on the real Mac in Phase 19. Writing the plan's lead verbatim would point the user at a pane that is not called that.
- **Fix:** kept the Phase 19 lead and applied the new UI-SPEC tail only. Both acceptance greps (`Portty uses it to let your other devices control this Mac`) still pass.
- **Files modified:** frontend/src/components/DesktopSetupSheet.tsx, its test
- **Commit:** 72ec6741

**2. [Rule 2 - Missing critical] Control reported only while viewers are connected**
- **Found during:** Task 1
- **Issue:** `ControlState()` never returns empty (`orOK`), and the needs_accessibility value is never reset when the last viewer leaves, so reporting it unconditionally would leave Settings warn with no one using control and no way to re-measure.
- **Fix:** `control` and `surface` are set only while `viewers > 0`; with nobody watching both are empty (matches UI-SPEC "empty when no viewer controls").
- **Commit:** ec2de385

No other deviations. `needs_accessibility` through the daemon is passed straight from `ControlState()` and is covered by the binding test and the Settings tests, not by a daemon test (driving a real missed-move measurement needs the 20-02 effect harness, which is internal to the desktop package).

## Known Stubs

None.

## Threat Flags

None. The new fields travel the existing 0600 daemon socket (T-20-49, accepted); no new endpoint, auth path or pane kind (T-20-48 satisfied by the refused "paste" kind).

## Verification run

- `go vet . ./internal/daemon/`, gofmt clean
- `go test -race -short`: `TestDesktopAPI_`, `TestShareLifetime_`, `TestDesktopPermissions_`, `TestBuildMenuItems`, `TestDesktopTrayLine`, `TestApp_GetDesktopStatus`, `TestDesktopSettingsBindings`, `TestOpenDesktopPermissionPane` all pass
- `tsc --noEmit` clean; whole vitest suite 133 files, 2359 tests pass
- both old Row 1 and Row 3 sentences absent from frontend/src; `bash tests/check-brand.sh` clean

## TESTING.md rows for plan 20-11 to apply (not edited here)

Section 2 (Suite Manifest) and Section 4 (traceability, DESK-05, DCTL-06, DCTL-01), all existing files with new tests:
- `internal/daemon/desktop_api_test.go`: `TestDesktopAPI_StatusSurfaceAndControl`
- `internal/daemon/desktop_permissions_test.go`: `TestDesktopPermissions_NoPasteField`
- `tray_common_test.go`: `TestDesktopTrayLine_WindowOnly`
- `app_desktop_test.go`: `TestApp_GetDesktopStatusSurfaceAndControl`
- `frontend/src/components/__tests__/SettingsSheet.desktop.test.tsx`: Showing row, Permissions control, consent copy
- `frontend/src/components/__tests__/DesktopSetupSheet.test.tsx`: changed intro and how-to, no paste row

Manual checklist (Section 5): no new M-NN item; the real-Mac check that the Showing row, tray suffix and Permissions warn appear under a live window share needs a viewer on a physical Mac and belongs with the existing Phase 20 manual rows.

## Self-Check: PASSED

All twelve modified files exist in the worktree; commits ec2de385, 0a95e689 and 72ec6741 are ancestors of HEAD.
