---
phase: 03-portty-brand
plan: 03
subsystem: brand-assets-tray
status: complete
tags: [brand, icons, tray, retina, iconutil, sips]
requires:
  - "Port 7444 / update feed plan (03-02), so the tree is green before the art swap"
provides:
  - "Six tray states embedded as 1x (18 px) + @2x (36 px) pairs; trayIconBytesForState(connected) (x1, x2 []byte)"
  - "makeTrayImage(d1, d2) in tray_objc_darwin.m: one 18x18 pt template NSImage with two bitmap reps"
  - "Portty app icon (assets/, build/), 10-file @2x-named iconset, iconutil icns (181234 bytes)"
  - "Six Portty title logos (docs/, frontend/public/, frontend/src/assets/); README light/dark header"
  - "scripts/verify-icons.sh: 27 sips/cmp/stat checks, no ImageMagick"
affects: [03-04, 03-06]
tech-stack:
  added: []
  patterns: ["Multi-rep template NSImage for Retina menu bar icons", "Brand asset gate on macOS built-ins (sips)"]
key-files:
  created:
    - assets/tray_icon@2x.png
    - assets/tray_icon_error@2x.png
    - assets/tray_icon_progress_25@2x.png
    - assets/tray_icon_progress_50@2x.png
    - assets/tray_icon_progress_75@2x.png
    - assets/tray_icon_progress_100@2x.png
    - docs/portty-title-logo.png
    - docs/portty-title-logo-dark.png
    - frontend/public/portty-title-logo.png
    - frontend/public/portty-title-logo-dark.png
    - frontend/src/assets/portty-title-logo.png
    - frontend/src/assets/portty-title-logo-dark.png
  modified:
    - assets/tray_icon.png (and the five other 1x tray states)
    - tray.go
    - tray_objc_darwin.m
    - tray_test.go
    - app_set_tray_progress_test.go
    - assets/appicon.png
    - build/appicon.png
    - build/AppIcon.iconset/ (10 files)
    - build/darwin/iconfile.icns
    - scripts/verify-icons.sh
    - README.md
    - TESTING.md
  deleted:
    - portty-brand/ (34 files deleted, 2 title logos moved to docs/)
    - build/gen_icon.go
    - build/gen_progress_icons.go
    - docs/agenthub-title-logo.png, frontend/public/agenthub-title-logo.png, frontend/src/assets/agenthub-title-logo.png
key-decisions:
  - "Tray icons load as one template NSImage holding an 18 px and a 36 px NSBitmapImageRep, both sized 18x18 pt; AppKit picks the 36 px rep on Retina (confirmed visually by M-71)"
  - "trayIconBytesForState returns the (1x, @2x) pair; the error pair still wins over any progress quartile"
  - "portty-brand/, build/gen_icon.go and build/gen_progress_icons.go deleted: each asset has one source and no generator can overwrite the art"
  - "scripts/verify-icons.sh uses only macOS built-ins (sips, cmp, stat); Windows/Linux/ImageMagick checks dropped"
requirements-completed: []
duration: 10 min
completed: 2026-11-16
commits: 2
plan_head_before: 88a7c5e7256202beef81b1a298936435ee6a144e
plan_head_after: 728977b352cb78227960698d3af28429ecf58db2
actuals:
  tokens: 9000
  tasks: 2
  commits: 2
---

# Phase 3 Plan 03: Portty Brand Assets Summary

The menu bar, app icon, icns and title logos now carry the Portty art from portty-brand/. Each tray state loads as one template image with an 18 px and a 36 px rep, so it should draw crisp on Retina. The brand folder and both icon generators are gone, and scripts/verify-icons.sh checks all of it using only macOS built-in tools.

## Performance

- Duration: about 10 min
- Tasks: 2 of 2
- Files changed: 69 (most are binary assets and deletions)

## Accomplishments

- Copied the 12 tray PNGs byte for byte (`cmp` matches portty-brand for all 6 pairs). tray.go has 6 new `//go:embed …@2x.png` vars, and the cgo calls now pass two (pointer, length) pairs.
- tray_objc_darwin.m `makeTrayImage(d1, d2)` builds an 18x18 pt NSImage with two NSBitmapImageReps, each set to 18x18 pt, with `setTemplate:YES`. Both Go buffers are still copied into NSData before `dispatch_async` (T-03-09).
- TestTrayIconAsset checks all twelve files with a table (18x18 / 36x36, file name in every message). TestApp_SetTrayProgress_ErrorPrecedence checks both slices for the disconnected, quartile-3 and quartile-0 cases.
- App icon copied to assets/ and build/ (identical files). The iconset holds 10 Portty files with @2x names. `iconutil -c icns` produced build/darwin/iconfile.icns at **181234 bytes**.
- Six Portty title logos are in place (1400x360) and the three old-name copies are deleted. The README header uses the `<picture>` block (dark srcset, alt "Portty", width 400).
- TESTING.md has the plan note, three BRND-02 traceability rows, and a new Category AM with M-71 (tray icons on Retina in light and dark, run in the Tart VM).

## Task Commits

1. **Task 1 (tracer): tray loads Portty 18 px + 36 px art as one template image**: `40cc4e46`
2. **Task 2: app icon, icns, title logos; generators and portty-brand/ removed**: `728977b3`

## Gate Results

- Tracer gate (Task 1): the full `<verify>` was re-run end to end and passed (4 PASS lines: TestTrayIconAsset, TestApp_SetTrayProgress_Idempotent/BoundsCheck/ErrorPrecedence; 6 @2x embeds; makeTrayImage present; sips reports 36 px).
- `go build ./... && go vet ./... && go test -race -short ./...`: all packages ok (after each task).
- `pnpm -C frontend exec tsc --noEmit`: ok. `pnpm -C frontend exec vitest run`: 91 files, 1173 tests passed.
- `bash tests/check-traceability-paths.sh`: `OK: 101 traceability paths exist`.
- `gofmt -l tray.go tray_test.go app_set_tray_progress_test.go`: empty. This also fixes tray.go's spacing issue from before this plan.
- `git ls-files portty-brand build/gen_icon.go build/gen_progress_icons.go`: empty. `git ls-files -- '*title-logo*'`: exactly the six Portty paths.
- Acceptance greps: trayIconBytesForState pair signature 1, four-arg updateTrayIcon 1, setTemplate:YES 1, M-71 1, BRND-02 rows 3.

verify-icons.sh output:

```
=== Portty Icon Verification ===

  PASS: build/appicon.png is 1024x1024
  PASS: assets/appicon.png is byte-identical to build/appicon.png
  PASS: AppIcon.iconset holds exactly 10 PNGs
  PASS: AppIcon.iconset has no -2x.png names
  PASS: iconfile.icns exists and is >100000 bytes (multi-entry)
  PASS: assets/tray_icon.png is 18x18
  PASS: assets/tray_icon@2x.png is 36x36
  ... (all six tray states, 1x 18x18 and @2x 36x36)
  PASS: docs/portty-title-logo.png is 1400x360
  ... (all six title logos 1400x360)
  PASS: exactly six title-logo files, all Portty-named
  PASS: portty-brand/ is gone
  PASS: build/gen_icon.go is gone
  PASS: build/gen_progress_icons.go is gone

=== Results: 27 passed, 0 failed ===
All checks passed.
```

## Decisions Made

See key-decisions in the frontmatter. One small choice: makeTrayImage returns an autoreleased image, and each rep is released after `addRepresentation:`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Tray NSImage leaked on every icon update**
- **Found during:** Task 1
- **Issue:** The old initStatusItem/updateTrayIcon called `[[NSImage alloc] initWithData:]` in a manual retain/release file and never released the image. The tray poller calls updateTrayIcon repeatedly, so every call leaked one image.
- **Fix:** makeTrayImage releases each rep after adding it and returns `[icon autorelease]`. The button's image property keeps its own reference.
- **Files modified:** tray_objc_darwin.m
- **Commit:** 40cc4e46

**2. [Clarity] verify-icons.sh helper named `dims`, not `size`**
- A shell function named `size` would hide `/usr/bin/size` (Xcode's `size` tool). It worked, but the name was misleading, so it was renamed before the commit. A negative check (18 px file tested against 36x36) fails as expected.

The plan's verification is otherwise unchanged.

## Issues Encountered

None.

## Known Stubs

None. Nothing imports the frontend title-logo copies; UI-SPEC says that is intended for this phase.

## Human Verification Pending

- M-71 (Tart VM, end of phase): checks that the tray picks the Retina rep and tints correctly on light and dark menu bars and when highlighted. This is the only check for the [ASSUMED, MEDIUM] two-rep behaviour. If it fails, the icons look blurry; nothing breaks.
- Wails builds the bundle icns from build/appicon.png, so the iconutil icns reaches the bundle only after plan 03-06 updates release.yml.

## Next Phase Readiness

- 03-04 can now edit tray_objc_darwin.m class, log and menu names. This plan left them alone (`AgentHubMenuDelegate`, the `@"AgentHub"` tooltip and the NSLog prefixes are still there).
- BRND-02 is done for assets. It is not marked complete in REQUIREMENTS.md until the phase-wide checks (M-71, 03-06) pass. BRND-05 stays open until the 03-06 zero-hit gate.

## Self-Check: PASSED

- FOUND: tray.go, tray_objc_darwin.m, scripts/verify-icons.sh, build/darwin/iconfile.icns, all 6 @2x tray files, all 6 title logos
- FOUND: 40cc4e46, 728977b3 (ancestors of HEAD)
