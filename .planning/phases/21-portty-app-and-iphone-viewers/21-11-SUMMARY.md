---
phase: 21-portty-app-and-iphone-viewers
plan: 11
subsystem: vm-proof
tags: [vm, tart, local-network, desktop-tab, wails, cgo, system-settings, safari, accessibility]
requires:
  - phase: 21-02
    provides: p21-rig.sh, keyprobe/apply.sh, the VM click and AX helpers
  - phase: 21-08
    provides: DesktopTab, window occlusion observer (this plan fixed it)
  - phase: 21-09
    provides: control mode in the Desktop tab (the stray click below landed in it)
  - phase: 20-09
    provides: p19/p20 rigs, office Developer ID grants, vmdesktop build
provides:
  - "p21-rig.sh commands build-home, ax2, wait-text2, stats2, office-viewers, office-assertions, safari2-front, ln-prompt2, tailnet-health"
  - "keyprobe/apply.sh --stats and StatsProbe.ts (scratch-only counts probe: ICE states, __desktopStats, window-hidden event)"
  - "21-VM-CHECKS.md rows D1 and D2, LN_DENY_ICE: new, LN_PANE: fallback"
  - "fix: the real app no longer dies at launch (window occlusion event emitted off the Cocoa callback)"
  - "fix: Open System Settings opens the plain Privacy & Security pane"
affects: [21-12, 21-13]
key-files:
  created:
    - .planning/phases/21-portty-app-and-iphone-viewers/rig/keyprobe/StatsProbe.ts
    - .planning/phases/21-portty-app-and-iphone-viewers/evidence/D1.ax
    - .planning/phases/21-portty-app-and-iphone-viewers/evidence/D1-connecting.ax
    - .planning/phases/21-portty-app-and-iphone-viewers/evidence/D1.log
    - .planning/phases/21-portty-app-and-iphone-viewers/evidence/D2-card.ax
    - .planning/phases/21-portty-app-and-iphone-viewers/evidence/D2-localnet.ax
    - .planning/phases/21-portty-app-and-iphone-viewers/evidence/D2-settings.ax
    - .planning/phases/21-portty-app-and-iphone-viewers/evidence/D2.log
  modified:
    - .planning/phases/21-portty-app-and-iphone-viewers/rig/p21-rig.sh
    - .planning/phases/21-portty-app-and-iphone-viewers/rig/keyprobe/apply.sh
    - .planning/phases/21-portty-app-and-iphone-viewers/21-VM-CHECKS.md
    - window_occlusion_darwin.go
    - window_occlusion_darwin_test.go
    - app_peer_desktop.go
    - app_peer_desktop_test.go
key-decisions:
  - "The window occlusion event is queued from the Cocoa callback onto one pump goroutine (ordered, 16 deep) instead of emitted inline: inline EventsEmit from the main-thread notification block killed the real app with SIGSEGV at launch"
  - "paneLocalNetwork is the plain x-apple.systempreferences:com.apple.preference.security URL: macOS 27.2 ignores the Privacy_LocalNetwork anchor and lands on the top Privacy & Security page either way (LN_PANE: fallback)"
  - "Task 2 (human-action) was performed by the executor inside the VM per the standing rule never to hand a native-GUI check to the user: a real click on the prompt's Don't Allow button"
requirements-completed: []
status: complete
duration: about 45 min
completed: 2026-11-27
commits: 5
plan_head_before: 51d8f9f94040cf981d221cca58305f5e907d63cb
plan_head_after: 3f47214f8ac50e957506775ed06becd2a28e8204
actuals:
  tokens: 24000
  tasks: 3
  commits: 5
---

# Phase 21 Plan 11: Real app, Desktop tab and a denied Local Network Summary

**The real Portty.app on the home VM lists the office Desktop row first, opens its tab on the Connecting card, raises its own Local Network prompt in front with the usage string, and, once denied, shows the Local Network card in about 12 s with ICE stuck at `new`; the run found and fixed a launch-time SIGSEGV in the 21-08 occlusion observer and a Local Network pane URL that macOS ignores.**

## What was done

- **Task 1 (tracer):** the rig commands and `apply.sh --stats`. Office build `wailsassets vmdesktop` Developer ID (the `p20-rig.sh build20` call, made through `p19-rig.sh build --skip-frontend`), home build Developer ID with the scratch-only stats probe. Row D1 recorded: Desktop row first in the OFFICE group above the shared `shell` row, one tab `Desktop · OFFICE`, first card `Connecting…`, prompt `Allow “Portty” to find devices on local networks?` with body `Portty needs this to show your other Mac's desktop in a tab.` (matches the usage string), topmost on screen, ICE new, framesDecoded 0.
- **Task 2 (checkpoint:human-action, done by the executor in the VM):** both guests were `online=yes`, the office already held Screen Recording and Accessibility, so no re-key or grant was needed. Don't Allow was answered with a real click on the prompt's button in portty-test-2; the prompt closed and the Local Network pane then listed Portty with the switch off.
- **Task 3:** row D2 and the closure lines `LN_DENY_ICE: new` and `LN_PANE: fallback`, each once in `21-VM-CHECKS.md`. Details are in the rows; the numbers 21-12 and 21-13 need:
  - card in 11 to 12 s after the click on the row, armed by ICE still new 10 s after the answer, so it shows the same while the prompt is unanswered or denied;
  - every denied peer connection logs `gathering` and nothing else (no `checking`, no `failed`); a new peer connection every 15 to 24 s; status connecting, reconnecting after about 16 s, unreachable after about 30 s, the card stays up;
  - Open System Settings: the anchored URL, the extension-id URL and `Privacy_LocalNetwork_Apps` all open the top `Privacy & Security` page; the constant is now the plain pane URL;
  - Open in Browser: Safari opens one window on `/desktop/` (path only recorded), no Safari Local Network prompt, the app page's DOM holds no cap value.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] The real app died with SIGSEGV seconds after launch.** Found at the first deploy: the GUI exited on both guests (Go `SIGSEGV ... signal arrived during cgo execution`, pc in system code, different fault address every run). Bisected with scratch builds on portty-test-2: without `startWindowOcclusionObserver()` the app lived; with the observer but no Go call from the block it lived; with the Go callback emitting nothing it lived; with the emit on a goroutine it lived; inline `runtime.EventsEmit` from the NSNotificationCenter block (main thread inside a cgo callback) died every time (3 of 3). Cause below the app code is not established; the cure is established. `emitWindowOcclusion` now queues onto one pump goroutine (ordered, 16 deep, marked `shortcut:` in code). RED test `TestEmitWindowOcclusion_ReturnsBeforeTheEmitFinishes` failed on the old code, passes now with `-race`; the ordering test was kept.
- **Found during:** Task 1 (first deploy). 21-08's own unit tests could not see it: the observer refuses to run under `go test`.
- **Files modified:** `window_occlusion_darwin.go`, `window_occlusion_darwin_test.go`
- **Commits:** `0095a65d` (RED), `b845c93f` (fix)

**2. [Rule 1 - Bug, the plan's planned fallback] Open System Settings did not open the Local Network page.** `paneLocalNetwork` carried a `Privacy_LocalNetwork` anchor assumed in 21-07; macOS 27.2 ignores it. Changed to the plain Privacy & Security URL, test pinned to the literal, home app rebuilt at that commit and redeployed, the card's button re-clicked with System Settings quit first: title `Privacy & Security`.
- **Files modified:** `app_peer_desktop.go`, `app_peer_desktop_test.go`
- **Commit:** `b982e560`

**3. [Rule 3 - Blocking] pnpm cannot run in the worktree.** `p20-rig.sh build20` runs `wails build` with `pnpm run build`. Used a temporary `frontend/node_modules` symlink to the main checkout, `tsc` and `vite build` run directly, then `p19-rig.sh build ... --tags "wailsassets vmdesktop" --sign devid --skip-frontend` (what `build20` calls). The symlink was never staged and is removed.

**4. [Rule 2 - Missing functionality in the rig] Small additions beyond the listed commands:** `start2` moves an older `keyprobe.log` aside so an old `PROBE ready` line cannot end its wait; `StatsProbe.ts` also logs the `portty:window-hidden` window event (`WINHIDDEN`) so 21-12 can see the occlusion bridge work; `scratch_build` was factored out of `build-probe` so `build-probe` and `build-home` share it (`build-probe` behaviour unchanged).

**5. [Process] The plan's checkpoint was not returned.** The standing rule (never hand a native-GUI check to the user) and the orchestrator's checkpoint policy had the executor answer the prompt in the VM. No VM was logged out, so no re-key checkpoint applied.

**6. [Process] Commit trailer.** The plan names `Claude Opus 5.5`; the harness prescribes `Claude Sonnet 5.5`, which is the model that wrote these commits, so every commit carries the Sonnet trailer (same call 21-10 made).

### Incident

One click aimed at the card's Open System Settings button arrived after the stream had connected and the card was gone, so it landed on the live picture in a Control-mode tab and was sent to the office desktop as a control click. Nothing was open on the office desktop; no harm. Later clicks checked the card with `wait-text2` first.

## Findings for 21-12 and 21-13

- **The denied state is not stable.** With Portty's Local Network switch off and no prompt, ICE connected on its own in two launches (5th peer connection at 93 s; 11th at 235 s), and the first launch after deploying a fresh build connected on its first peer connection with no prompt at all. Other launches stayed denied for 200+ s (10 peer connections). Cause not found (one run followed a System Settings relaunch, the other followed ssh reads only). 21-12 should start its Allow row from a fresh `start2` + Desktop row click and read ICE from `stats2`, not assume the card is still up.
- `winfo2` still depends on `~/p21/key`, which `remove2` deleted after 21-02; it prints `none`. `ax2` and `~/p20/ax20 windows app.portty.Portty` give the window frame instead.
- Click targets used in the home VM at the 1280x720 screen: Desktop row (163, 188) when the sidebar has no `New Session` row above it (it moved to y 202 when an empty This Mac group showed the row), card buttons Open System Settings (764, 448) and Open in Browser (764, 508).
- A scratch crash report from the failed first builds was read through `~/Library/Logs/DiagnosticReports` (GOTRACEBACK=crash); it only showed the Go crash handler's thread, not the original fault, so it is not kept.

## VM end state left for 21-12

- Both VMs running, both `online=yes`; neither stopped.
- Office (portty-test): vmdesktop Developer ID build 21.11.2 (HEAD `b845c93f`, contains the occlusion fix), launchd daemon running, always-available on, GUI quit, one shared `shell` session, link files in `~/p19` of both guests (never read into evidence).
- Home (portty-test-2): Developer ID build of HEAD `b982e560` plus the stats probe at `/Applications/Portty.app`, GUI running, the `Desktop · OFFICE` tab open on the Local Network card (card showing at hand-off; it can decay as described above), System Settings open on the Local Network page, Safari open on the office's `/desktop/` page, `~/p21/keyprobe.log` holding the stats lines.
- Local Network: Portty's switch is off in System Settings; Google Chrome's is on.

## Known Stubs

None. (`StatsProbe.ts` and the stats patch exist only in scratch copies of the tree; `git status --porcelain --untracked-files=no -- frontend internal` printed nothing.)

## Threat Flags

None new. The occlusion change reduces risk (T-21-26 path no longer crashes the app). Evidence was scrubbed on capture (`scrub_ax` also maps the two guests' short names to `OFFICE` and `HOME`); the verify grep for tailnet addresses, `.ts.net` names and `cap=` values found none, and no home path or account name appears in the committed files.

Verification run: `go test -race -count=1 .` passes (25.7 s, with `frontend/dist` built, so `TestNoFakeWhoIsInRelease` runs too); `bash tests/check-brand.sh` clean; `p21-rig.sh` passes `bash -n`; the D1 and D2 verify greps (row present, each closure line once, no tailnet address, `.ts.net` name or cap value) hold; `git status --porcelain --untracked-files=no -- frontend internal` printed nothing.

## Self-Check: PASSED

Created evidence and probe files and all five commits (0095a65d, b845c93f, 3b6bc235, b982e560, 3f47214f) verified present.
