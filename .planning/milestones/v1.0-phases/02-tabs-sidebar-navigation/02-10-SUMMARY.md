---
phase: 02-tabs-sidebar-navigation
plan: 10
subsystem: frontend-banners
status: complete
tags: [look-01, net-01, banners, tailscale, tokens, testing]
requires: ["02-09"]
provides:
  - "(*App).OpenTailscale() error: fixed argv `open -a Tailscale`, falls back to the fixed URL https://tailscale.com/download/mac via BrowserOpenURL; openAppFunc/openURLFunc seams"
  - "One banner style on tokens: .banner, .banner--warning/--info, .banner__icon/__message/__action/__dismiss"
  - "Tailscale banner: warning variant, id=ts-off-reason, role=status, 'Open Tailscale' action, no dismiss"
  - "Banner stack order Tailscale, WebGL recovery, update, plugin toggle, save feedback"
  - "Daemon error and save-feedback notices on the banner style; App.tsx has no inline hex and no onMouseEnter"
  - "No ⚠ glyph left in frontend source (Share segmented control and Funnel heading use ExclamationTriangleIcon)"
affects: ["02-11", "02-12", "02-13"]
actuals:
  tokens: 15000
  tasks: 2
  commits: 2
commits: 2
plan_head_before: 7984d162e7a20133c45b94b3f2242e6138299b85
plan_head_after: f467658e37f41900f2d50128f6fe25f3a1a83d67
tech-stack:
  added: []
  patterns:
    - "Banner markup: <div class='banner banner--{warning|info}'> icon.banner__icon, span.banner__message, optional button.banner__action, optional button.banner__dismiss[aria-label=Dismiss] > XMarkIcon (24/outline)"
    - "Exec seams on App: nil func field means the real call; tests inject fakes and assert the exact fixed arguments"
key-files:
  created: []
  modified:
    - app.go
    - app_test.go
    - frontend/src/wailsjs/go/main/App.d.ts
    - frontend/src/wailsjs/go/main/App.js
    - frontend/src/components/TailscaleBanner.tsx
    - frontend/src/components/UpdateBanner.tsx
    - frontend/src/components/WebGLRecoveryBanner.tsx
    - frontend/src/components/PluginToggleBanner.tsx
    - frontend/src/components/SessionEndedBanner.tsx
    - frontend/src/components/Share/ShareSegmentedControl.tsx
    - frontend/src/components/SessionShare/InternetFullAccessTab.tsx
    - frontend/src/App.tsx
    - frontend/src/style.css
    - frontend/src/components/__tests__/TailscaleBanner.test.tsx
    - frontend/src/components/__tests__/UpdateBanner.test.tsx
    - frontend/src/components/__tests__/WebGLRecoveryBanner.test.tsx
    - frontend/src/components/__tests__/PluginToggleBanner.test.tsx
    - frontend/src/components/__tests__/SessionEndedBanner.test.tsx
    - frontend/src/components/__tests__/ShareSegmentedControl.test.tsx
    - frontend/src/components/__tests__/App.test.tsx
    - TESTING.md
    - .planning/phases/02-tabs-sidebar-navigation/deferred-items.md
decisions:
  - "OpenTailscale returns nil after the URL fallback, so the frontend never sees an error for a missing app; the banner's catch only logs binding failures"
  - "The save-error banner and the daemon error notice use the warning variant (both are problem notices); the save info notice uses info"
  - "UpdateBanner's text 'Dismiss' button became the shared XMarkIcon Dismiss; 'Download Update' became the banner action link"
  - "The ⚠ glyphs in ShareSegmentedControl and InternetFullAccessTab were removed here (deferred-items assigned them to 02-10), keeping the SM-07 colorblind cue as is-danger ring plus an ExclamationTriangleIcon"
metrics:
  duration: "about 35 min"
  completed: 2026-11-15
---

# Phase 2 Plan 10: Banners Summary

All banners now share one token-based strip style (warning or info, outline icon, 24px XMarkIcon Dismiss). The Tailscale-off banner gets an "Open Tailscale" action, backed by a fixed-argv Go binding that runs `open -a Tailscale` and falls back to the download page.

## What was built

**Task 1 (tracer), commit 78504c37.**
- `app.go`: new `OpenTailscale()` with `openAppFunc` / `openURLFunc` seams. It calls `exec.Command("open", "-a", "Tailscale")`. If that fails, it calls `runtime.BrowserOpenURL(ctx, "https://tailscale.com/download/mac")` (only when `a.ctx != nil`) and returns nil. It takes no parameters, so nothing from the frontend reaches exec (T-02-30). It only launches an installed app or opens a web page, so it never installs or configures Tailscale (T-02-31).
- Tests: `TestOpenTailscale_OpensApp` checks that the app name is exactly "Tailscale" and the URL path is not called. `TestOpenTailscale_FallsBackToDownloadURL` checks the exact URL and that no error is returned.
- `OpenTailscale` was hand-added to `App.d.ts` (`Promise<void>`) and `App.js`.
- `style.css`: the new `.banner*` block replaces the Phase 97 save-banner rules and the `.tailscale-banner*` hex rules.
- `TailscaleBanner.tsx`: now `.banner.banner--warning`. It keeps `id="ts-off-reason"`, `role="status"`, the icon and `TAILSCALE_OFF_REASON`, and adds the action button. It has no dismiss.
- Tracer gate: the automated verify was re-run end-to-end and passed before Task 2.

**Task 2, commit f467658e.**
- Banner variants:
  - WebGL recovery is `banner--warning` with `ExclamationTriangleIcon`.
  - Update, plugin toggle and session ended are `banner--info` with `InformationCircleIcon`.
  - Every dismissible banner uses `.banner__dismiss` with `aria-label="Dismiss"` and an outline `XMarkIcon`.
  - Session ended keeps a positioning-only `.session-ended-banner` rule, because it overlays the top of the terminal viewport.
- `App.tsx`:
  - The stack is reordered to Tailscale, WebGL, update, plugin toggle, save. The INVARIANT gating expression is unchanged.
  - The save banner uses warning or info with its matching icon, and an XMarkIcon Dismiss replaces the text "×".
  - The daemon error is now a `banner--warning` (`role="alert"`) with its bold title, `{daemonError}` and a "Retry Connection" action. All inline styles and the hover handlers are gone.
- `style.css`: removed the `.update-banner*`, `.webgl-recovery-banner__message` and `.session-ended-banner*` colour rules.
- Tests:
  - Each banner test asserts its variant class, svg icon and an empty-text XMarkIcon Dismiss.
  - `App.test.tsx` asserts the order, each INVARIANT trigger, the save and daemon-error banner markup, and no inline hex, 6-digit hex or `onMouseEnter`.
- TESTING.md: a Section 2 note (counts unchanged: Go 107, vitest 92, total 207), a Section 4 LOOK-01 row for `TailscaleBanner.test.tsx` and a NET-01 row for `app_test.go`, and Section 5 Category AI with M-65. `tests/check-traceability-paths.sh` reports OK (87 paths).

## Verification

- `go build ./... && go vet ./... && go test -race -short ./...`: all packages ok. `TestOpenTailscale_*`: 2 PASS.
- `pnpm exec tsc --noEmit`: clean. `pnpm test`: 92 files, 1250 tests passed.
- `pnpm exec playwright test --project=chromium`: 7 passed, 9 skipped (the existing skips).
- `bash tests/check-traceability-paths.sh`: OK.
- `wails build -platform darwin/arm64 -tags wailsassets`: built `build/bin/agenthub.app`. The generated `frontend/src/wailsjs/wailsjs/` tree was restored afterwards.
- Acceptance greps, all at the planned values: OpenTailscale in app.go 1 and App.d.ts 1; "Open Tailscale" 1; `ts-off-reason` 1; download URL 1; 6-digit hex in App.tsx 0; onMouseEnter 0; banner--info count 3; M-65 1.

## Needs VM check

The app was built at `build/bin/agenthub.app` but not launched on the host. Run M-65 in the Tart VM, in Light and then Dark:
1. **Tailscale off** (quit Tailscale): the banner is a thin yellow strip under the toolbar on tokens, not the old dark `#16161e` box. It has a triangle icon, the NET-01 sentence and a bold "Open Tailscale" link that underlines on hover and focus, and no ×.
2. **Open Tailscale** with Tailscale installed: the Tailscale app opens. With it not installed or renamed: https://tailscale.com/download/mac opens in the browser. Nothing gets installed.
3. **Toolbar Share button** while Tailscale is off: VoiceOver still reads the banner sentence as its description (`aria-describedby="ts-off-reason"`).
4. **Stack order and look:** trigger as many banners as you can (Tailscale off, a plugin toggle saved in Settings, a save error from Save Terminal As… into an unwritable folder, an update notice from a fake feed, a WebGL fallback if reproducible). They stack as Tailscale, WebGL, update, plugin toggle, save. Each is a 12px-text strip with a 16px outline icon and a 24×24 × "Dismiss" button that works (none on the Tailscale banner).
5. **Daemon error** (start with the daemon unreachable): a yellow warning strip with "Unable to connect to session daemon", the error text and a "Retry Connection" link. No dark box, no hover colour flash.
6. **Share dialog ▸ Internet Full access** segment: a small triangle icon before "Full access", no ⚠ glyph. When active, the icon is red with the label. The Funnel write-gate heading shows only the icon, not "⚠ You are…".
7. **Remote "Session ended"** banner on a peer tab (end the session on the host): a blue info strip across the top of the terminal area with an × button.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing critical / scope assigned by orchestrator] Removed the remaining ⚠ glyphs**
- **Found during:** Task 2. `deferred-items.md` (from 02-09) and the orchestrator's brief both made the Share segmented control's "⚠" this plan's job.
- **Fix:**
  - `ShareSegmentedControl.tsx` renders an aria-hidden `ExclamationTriangleIcon` (`.share-seg__icon`, 12px, red with the label when active) before the danger sub label, instead of prefixing "⚠".
  - The `is-danger` ring and the "Full access" text are kept, so the SM-07 colorblind cue still has two non-colour signals.
  - The duplicate "⚠ " in the `InternetFullAccessTab` warning heading is removed (the icon was already there).
  - The test now pins the svg and the absence of ⚠.
- **Files modified:** ShareSegmentedControl.tsx, InternetFullAccessTab.tsx, style.css, ShareSegmentedControl.test.tsx
- **Commit:** f467658e

**2. [Rule 3 - Blocking] Kept `.webgl-recovery-banner` base and `__dismiss` rules**
- **Issue:** the plan says to delete the per-banner colour rules that the shared style replaces, but `ShellWebShareBanner` (inside the Share dialog) still uses `.webgl-recovery-banner`, `--shell-warning`, `--block` and `__dismiss`.
- **Fix:** removed only `.webgl-recovery-banner__message` (now unused) and relabelled the block's comment. The rest stays for ShellWebShareBanner and is logged in deferred-items for 02-12's no-hex sweep.
- **Commit:** f467658e

Other notes: the daemon error notice keeps `role="alert"` (it was a plain div before). The action/dismiss focus ring uses the existing `--color-focus-ring` 3px pattern.

## Known Stubs

None.

## Threat Flags

None. The only new surface, the OpenTailscale exec, is T-02-30/T-02-31 in the plan's threat model, and both are mitigated as planned.

## Self-Check: PASSED

- Files exist: app.go, TailscaleBanner.tsx, TESTING.md, 02-10-SUMMARY.md
- Commits on HEAD: 78504c37, f467658e
