---
phase: 21-portty-app-and-iphone-viewers
plan: 02
subsystem: desktop-viewer-input
tags: [wkwebview, key-routing, wails, tart-vm, probe]
requires: []
provides:
  - "KEYROUTE: page verdict (page-level preventDefault keeps all 15 chords away from Portty's menu in the real app)"
  - "CMDKEYUP: swallowed, RELEASE_CHORD: taken, HIDDEN_VISIBILITY facts for 21-08 and 21-09"
  - "rig/p21-rig.sh and rig/key.swift for later Phase 21 VM rows on portty-test-2"
affects: [21-08, 21-09]
tech-stack:
  added: []
  patterns: ["throwaway probe applied only to a scratch git-archive export", "control matrix (focus outside the stage) to prove each detector fires"]
key-files:
  created:
    - .planning/phases/21-portty-app-and-iphone-viewers/rig/p21-rig.sh
    - .planning/phases/21-portty-app-and-iphone-viewers/rig/key.swift
    - .planning/phases/21-portty-app-and-iphone-viewers/rig/keyprobe/apply.sh
    - .planning/phases/21-portty-app-and-iphone-viewers/rig/keyprobe/KeyProbe.tsx
    - .planning/phases/21-portty-app-and-iphone-viewers/rig/keyprobe/probe_darwin.go
    - .planning/phases/21-portty-app-and-iphone-viewers/rig/keyprobe/probe_darwin.m
    - .planning/phases/21-portty-app-and-iphone-viewers/21-VM-CHECKS.md
    - .planning/phases/21-portty-app-and-iphone-viewers/evidence/
  modified: []
decisions:
  - "Control mode uses page-level preventDefault and stopPropagation only; no native menu code (KEYROUTE: page)"
  - "Ctrl+Option+Cmd+Return cannot be the release chord: the keydown for Return never reaches the page (RELEASE_CHORD: taken); 21-09 must pick another"
  - "A covered (occluded) window stays visibilityState visible, so 21-08's hidden-window rule cannot rely on visibility for occlusion"
metrics:
  duration: "about 1h45m (VM matrix runs dominate)"
  completed: 2026-11-27
status: complete
actuals:
  tokens: 8700
  tasks: 3
  commits: 2
plan_head_before: 6662f19644b8914f539b0952f62f526d6d7e3711
plan_head_after: e0d0accaf7e0adcc24be24f340500de1bbd8c5b0
---

# Phase 21 Plan 02: Key-routing probe in the real Portty.app Summary

A throwaway probe in the real Portty.app (Wails v2.10.2, bundle app.portty.Portty) in portty-test-2 shows that a keydown handler calling `preventDefault` and `stopPropagation` on a hidden textarea keeps every Cmd chord away from Portty's menu, so control mode needs no native code.

## Verdict lines (exact, from 21-VM-CHECKS.md)

```
KEYROUTE: page
CMDKEYUP: swallowed
RELEASE_CHORD: taken
HIDDEN_VISIBILITY: closed=hidden menubar=hidden minimized=hidden occluded=visible
```

## What was done

- **Task 1 (tracer), commit 3b31e5a5.** `rig/p21-rig.sh` (build-probe, tools2, vm2-idle, deploy2, start2, quit-all2, remove2, key2, mode2, focus2, log2, alive2, matrix2, click2, winfo2, sh2), `rig/key.swift` (key and winfo subcommands), and `rig/keyprobe/` (KeyProbe.tsx, probe_darwin.go, probe_darwin.m, apply.sh). Row K0: a KeyA posted in the VM reached the page log, and with focus outside the textarea Cmd+comma reached the menu (`MENU menu:settings`), so INJECT was ok.
- **Task 2 (checkpoint:human-action) was not needed.** K0 read INJECT: ok, the ssh login process was already trusted for event posting in the VM, so no Accessibility grant was made and nothing was returned to the orchestrator.
- **Task 3, commit e0d0acca.** Rows K1-page, K1-ctl, K1-div, K2 to K5, the four verdict lines, evidence logs, and cleanup (`remove2`: Quit All, bundle and ~/p21 deleted, GUI none; both VMs still running).

## Findings that matter to later plans

- **K1-page:** all 15 chords (Cmd+Q, W, C, V, X, Z, A, T, H, M, comma, Ctrl+Cmd+S, Ctrl+Cmd+F, Cmd+Option+H, Backspace) reach the page and none fires a menu action (no MENU, VIS, COPY, CUT, PASTE, SELECTSTART or BEFOREINPUT line, GUI and window unchanged). The WebKit "page first" reading (Pattern 1, A1) holds in this app.
- **K1-ctl (control, focus outside the textarea):** the same chords do fire the menu (quit modal event, close tab, copy, paste, cut, select all, new session, hide, minimize, settings, toggle sidebar, full screen), which proves the detectors. Cmd+Z, Cmd+Option+H and Backspace show no observable effect even in the control, so their "no leak" is "no observable action"; Backspace is additionally covered by the filled textarea.
- **K2 (A2 confirmed):** keyup for a key pressed with Cmd held never arrives (only KEYUP MetaLeft), so Pattern 2's tap workaround is required.
- **K3:** Ctrl+Option+Cmd+Return never delivers a Return keydown to the page (modifiers arrive). Plain Return, Ctrl+Cmd+Return, Option+Cmd+Return, Ctrl+Option+Cmd+Tab and Ctrl+Option+Cmd+Shift+Return arrive. 21-09 needs a different release chord; this is the one verdict that contradicts the CONTEXT candidate.
- **K4:** VIS hidden after the red close button, after Quit GUI Only, after Cmd+M, and after Cmd+H; a window completely covered by another app's window stayed visible for 35+ seconds. After Quit GUI Only, `open` does not re-show the window; the tray's "Open Portty" does.
- **K5:** BLUR arrives when another app is activated, FOCUS when Portty is reactivated.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] pnpm run build tried to write node_modules**
- **Found during:** Task 1 (first build-probe run)
- **Issue:** `pnpm run build` re-verified dependencies and failed on the symlinked `node_modules` ("workspace hoist directory is not a real directory"), which would also have risked touching the real directory.
- **Fix:** build-probe runs the package.json build script directly (`./node_modules/.bin/tsc && ./node_modules/.bin/vite build`) in the scratch tree, then `wails build -s`. No package was installed.
- **Files modified:** rig/p21-rig.sh
- **Commit:** 3b31e5a5

**2. [Rule 2 - Missing coverage] Added a control matrix (K1-ctl) and extra rig helpers**
- **Issue:** K1-page alone cannot distinguish "no leak" from "detector blind". 
- **Fix:** ran the matrix with focus outside the textarea as K1-ctl (information row); added `matrix2`, `click2`, `winfo2`, `sh2` helpers to the rig.
- **Commit:** 3b31e5a5, e0d0acca

### Plan variations (not defects)

- K1-blank and K1-rebuild were not run (the plan runs them only when K1-page shows a leak; it showed none). Their ObjC and Go code compiled into the probe build but was never exercised.
- K3 gained variant posts beyond the one the plan asked for, to show which chords do arrive.
- K4 occluded: osascript control of TextEdit was refused by a VM Automation prompt, which was answered "Don't Allow" (no grant); the TextEdit window was dragged over Portty instead.

## Side effects in portty-test-2

- `deploy2` ran Quit All, which ended a long-running Portty GUI and daemon that were running from `~/ptest/Portty.app` (about 1 day 20 hours old, not started by any current rig). The bundle itself was left in place; nothing else under `~/ptest` was touched. A stale Safari `--automation` process from earlier work was also left untouched.
- vm2-idle found no WebDriver process and no Phase 20 rig process before and during the run.

## Known Stubs

None. The probe code is intentionally throwaway and lives only under `.planning/.../rig/keyprobe/`; it never entered `frontend/`, `internal/`, `app.go` or `main.go` (`git status` for those paths stayed clean).

## Threat Flags

None. No new network endpoint, auth path or schema; all input posting ran inside portty-test-2 via ssh, nothing on the host posted events or captured the screen, and no tailnet address, ts.net name, home path or token is in the committed evidence (checked by grep and `tests/check-brand.sh`).

## Self-Check: PASSED

- FOUND: rig/p21-rig.sh, rig/key.swift, rig/keyprobe/{apply.sh,KeyProbe.tsx,probe_darwin.go,probe_darwin.m}, 21-VM-CHECKS.md, evidence/K0.log to K5.log
- FOUND commits: 3b31e5a5, e0d0acca
- Verdict lines each appear exactly once; `bash tests/check-brand.sh` clean; evidence grep for addresses, ts.net names and home paths empty; portty-test and portty-test-2 both running; alive2 prints none.
