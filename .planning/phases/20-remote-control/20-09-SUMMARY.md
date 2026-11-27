---
phase: 20-remote-control
plan: 09
subsystem: vm-proof
tags: [vm, tart, macos-27, cgevent, accessibility, pasteboard, safari, chrome, webdriver]
requires:
  - phase: 20
    provides: "input path, window gate, clipboard, surface switch, viewer, control bar (20-01 to 20-08)"
  - phase: 19
    provides: "p19-rig.sh, the vmdesktop tag and its brand guard, the two Tart VMs, Developer ID build flow"
provides:
  - "VM-only Manager.DebugVM20 and POST /desktop/debug/vm20 behind the vmdesktop tag"
  - "p20-rig.sh, ax20.swift, pbset.js: a VM rig that drives the Safari viewer and the office Mac"
  - "20-VM-CHECKS.md: 17 plan rows, 3 extra rows, research A1-A15 answers, PASTE_ROW: none"
  - "fix: the /desktop/ viewer no longer sits under index.html's opaque splash"
affects: [20-10, 20-11, 21, 22]
key-files:
  created:
    - internal/desktop/probe_vmdesktop.go
    - internal/daemon/desktop_probe_vmdesktop.go
    - .planning/phases/20-remote-control/rig/p20-rig.sh
    - .planning/phases/20-remote-control/rig/ax20.swift
    - .planning/phases/20-remote-control/rig/pbset.js
    - .planning/phases/20-remote-control/20-VM-CHECKS.md
    - .planning/phases/20-remote-control/evidence/ (20 files)
  modified:
    - frontend/src/components/DesktopView.tsx
    - frontend/src/components/__tests__/DesktopView.test.tsx
key-decisions:
  - "PASTE_ROW: none. The launchd daemon raised no paste-privacy alert for reading types, reading text or writing text (accessBehavior alwaysAllow); no System Settings paste setting exists for Portty"
  - "Build tags are wailsassets vmdesktop, not the plan's wailsassets vmpreview vmdesktop: vmpreview fakes Tailscale and the desktop will not start on it"
  - "A pop-up of the shared app is unclickable in a window share (dropped as covered); a same-app carve-out is the user's decision"
  - "Accessibility was re-granted by automating System Settings inside the VM (AX press and the click tool, password typed in the VM), so the human-action checkpoint was not needed"
requirements-completed: [DCTL-01, DCTL-02, DCTL-03, DCTL-05, DCTL-06, DESK-06]
status: complete
actuals:
  tokens: 60000
  tasks: 3
  commits: 3
plan_head_before: 2aad4e8d69ab1f341adde10a7e4cbcf40af01668
plan_head_after: b80053ad19cbc8ea6a406a1d5e5b5780ba989500
duration: about 4 hours
completed: 2026-11-26
---

# Phase 20 Plan 09: VM proof rows Summary

The whole Phase 20 stack was proven on real macOS 27.2 between the two Tart VMs: typing and clicking in the Safari viewer at home land in TextEdit on the office VM through the launchd daemon, system chords and the lock gate behave, a 1 KiB emoji string arrives exactly, a window share raises, confines and refuses correctly, in-band display/window switches work in Safari and Chrome, clipboard moves both ways, held keys are released, and Control needs Accessibility appears and clears on a real grant change. Decision: `PASTE_ROW: none`.

## What was done

- **Task 1 (tracer, c8876c2d and b80053ad):** the vmdesktop probe (ops list, key, chord, text, scroll, release, select, raise, minimized, topmost, focus, behavior, pbread, pbwrite, control; input goes through `parseInput`, the per-viewer queue, the gate and the post, as a pseudo-viewer; `select` runs the real `handleSelect` over a loopback socket), the rig, and rows E1, E2, C1, C2, T1, S1, W1, W2, W3, D1, D2, P1, P2, H1, A1. The tracer gate (E1 end to end) passed before the other rows.
- **Task 2 (automated, no human):** `tccutil reset` left Portty's row in the Accessibility list switched off. The switch was clicked with the VM click tool, the administrator sheet found with the AX helper, the password typed as key codes inside the VM, Modify Settings pressed with AX. `GET /desktop/permissions` went to `ax:true`. No paste alert appeared, and none was ever on screen.
- **Task 3 (recovery rows and closure):** A2 (control input back to `ok` at most 4 s after the grant, no viewer action), P3, the closure with `PASTE_ROW: none`.
- Extra rows: X1 (pop-up of the shared app dropped as covered), X2 (status `streaming` appears with `framesDecoded` 0, so connected precedes the first frame), X3 (the AX focus read failed with -25204 for about ten minutes).
- VM end state: untagged build 20.9.4 installed and answering 404 to the route, always-available off, link files removed from both guests, Accessibility granted, neither VM stopped.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] The desktop viewer page was covered by an opaque splash**
- **Found during:** Task 1, row E1 (WebDriver clicks reached no handler)
- **Issue:** `index.html` has `#splash-static` (fixed, z-index 9999, background Canvas). `App` hides it on mount; the /desktop/ viewer renders `DesktopView` and never `App`, so the splash stayed over the picture and took every pointer event. Phase 19's rows read `__desktopStats` only, so none saw it.
- **Fix:** `DesktopView` removes `#splash-static` in its mount effect; one test.
- **Files modified:** frontend/src/components/DesktopView.tsx, frontend/src/components/__tests__/DesktopView.test.tsx
- **Commit:** f97d700e

**2. [Rule 3 - Blocking] The plan's build tags cannot run the desktop**
- **Found during:** Task 1 step 3
- **Issue:** `vmpreview` fakes Tailscale (media bind 127.0.0.1); enable answers `media bind address is not a tailnet address`.
- **Fix:** build20 uses `wailsassets vmdesktop`, as 19-12 did. Documented in the rig header and the checks file.

**3. [Rule 3 - Blocking] The fresh worktree had no frontend dependencies or dist**
- **Found during:** build
- **Fix:** `pnpm install --frozen-lockfile --offline` (lockfile versions only, nothing added) and one `pnpm run build` so Wails could embed a `frontend/dist`. Both are ignored by git.

**4. [Rule 3 - Blocking] Rig workarounds** (details in 20-VM-CHECKS Notes): Safari sends code `Unidentified` for a bare space, so `safari-type` uses the WebDriver Space key; the control bar's hidden state and the Clipboard trigger defeat WebDriver clicks, so P2 and P3 activate the menu with JavaScript `click()`; TextEdit restores old text and autocapitalises, so `textedit-new` kills it with SIGKILL, clears its state and turns that default off.

### Plan statements that did not hold

- The viewer's debug hook does not count hellos or expose `connectionState`; "no second hello" was read from the daemon log (one `viewer joined`, none during four switches) and "ICE connected" is inferred from the 20-06 rule.
- "Click outside the window" (W2) cannot be made through the viewer in a window share (its picture is the window); the unit tests keep that case.
- W3's decoded-frame part is NOT RUN (Safari canvas readback fails in the VM, no DevTools client in the guests).
- The plan's trailer names another model; the commits carry Claude Sonnet 5.5, the model that wrote them.

## Findings for the orchestrator and the user

- **Fixed:** the viewer splash (above).
- **Needs a decision:** X1, pop-ups, menus and sheets of the shared app cannot be clicked in a window share (dropped as `covered`, 20-04's id-only comparison). A same-app carve-out is needed if the user wants them usable.
- **Watch:** X3, on 27.2 the system-wide AX focus read failed (-25204) for about ten minutes in one VM session, from the daemon and from a trusted ssh process alike; the window gate failed closed. Not reproduced afterwards; cause unknown.
- **Decision for 20-10:** `PASTE_ROW: none`.
- **Not run:** A5, A8, A9, A14, A15; the Wails web view and iPhone halves of A11; W3's pixels.

## TESTING.md rows for plan 20-11 (not edited here)

Section 2 (Suite Manifest): no test files were added or removed except the new case in the existing `frontend/src/components/__tests__/DesktopView.test.tsx` (frontend group, no new file). The probe files are VM-only with no tests.

Section 4 (traceability, path column file only): DCTL-06 and DESK-05 gain nothing new; the splash fix is covered by `frontend/src/components/__tests__/DesktopView.test.tsx` (already mapped).

Section 5 (manual, new M-NN, from the NOT RUN and OBSERVED rows of 20-VM-CHECKS):

- M-NN: sheet, Open and Save panel and pop-up menu in the decoded frame of a window share (W3).
- M-NN: a pop-up menu of the shared app clicked through the viewer in a window share (X1), after the carve-out decision.
- M-NN: AX system-wide focus read stays valid across lock/unlock, scroll preference changes and app restarts on the real Macs (X3).
- M-NN: Safari Copy from Mac written to the clipboard from a real click and Safari's Paste prompt (P2).
- M-NN: a real paste-privacy alert unanswered shows `waiting_approval` after 2 s (P1), if one can be produced.
- M-NN: scroll direction on a trackpad Mac with natural scrolling on and off (S1).

## Known Stubs

None.

## Threat Flags

None beyond the plan's register: the probe route (T-20-43) is behind `//go:build vmdesktop` in both files, on the 0600 daemon socket, absent from the default build, and the installed untagged build answers 404 (`evidence/TAGCHECK.txt`); `tests/check-brand.sh` passes.

## Self-Check: PASSED

Created files exist (probe_vmdesktop.go, desktop_probe_vmdesktop.go, p20-rig.sh, ax20.swift, pbset.js, 20-VM-CHECKS.md, evidence/); commits f97d700e, c8876c2d and b80053ad are on the branch; `go vet -tags vmdesktop`, `go build ./...`, `golangci-lint` (0 issues), `tests/check-brand.sh` and the row-verdict greps pass; the evidence holds no address, tailnet name, token or home path.
