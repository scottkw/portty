---
phase: 21-portty-app-and-iphone-viewers
plan: 12
subsystem: vm-proof
tags: [vm, tart, local-network, desktop-tab, control-mode, held-keys, paste, system-settings, accessibility]
requires:
  - phase: 21-11
    provides: p21-rig.sh (ax2, stats2, office-viewers, office-assertions, tailnet-health), the denied Local Network state, rows D1 and D2
  - phase: 21-09
    provides: control mode in the Desktop tab and the Ctrl+Option+Cmd+Shift+Return release chord
  - phase: 20-09
    provides: p20-rig.sh (textedit-new, textedit-read, pb-set, pb-get, probe, pf through p19)
provides:
  - "21-VM-CHECKS.md rows D3-pre, D3, D4, D5, D6, K6, H2, P2 and the lines DVWR01_VM: pass, DVWR04_VM: pass, PASTE_ALERT: none, DEFERRED_UAT"
  - "p21-rig.sh commands tools12, axswitch, keystate (rig/axswitch.swift, rig/keystate.swift)"
affects: [21-13]
key-files:
  created:
    - .planning/phases/21-portty-app-and-iphone-viewers/rig/axswitch.swift
    - .planning/phases/21-portty-app-and-iphone-viewers/rig/keystate.swift
    - .planning/phases/21-portty-app-and-iphone-viewers/evidence/ (D3-pre, D3-run1, D3-run2, D3-dark, D4, D5, D6, K6, H2, P2 files, scrubbed)
  modified:
    - .planning/phases/21-portty-app-and-iphone-viewers/21-VM-CHECKS.md
    - .planning/phases/21-portty-app-and-iphone-viewers/rig/p21-rig.sh
key-decisions:
  - "The Local Network switch is pressed with Accessibility (AXPress on the AXSwitch that follows the label Portty) instead of a screen-position click: a notification banner slid over the switch on the first attempt and the click turned into a banner click that moved System Settings to Login Items"
  - "Held keys are read from the office's OS key state (keystate) because the daemon's probe has no held-key count; an instrument check during a 3000 ms hold shows it sees a held Cmd"
  - "No code change: every row passed on the builds 21-11 left in place"
requirements-completed: []
status: complete
duration: about 1 h 55 min
completed: 2026-11-27
commits: 3
plan_head_before: 0cd51fb497cb253b7db23c2078de8c86fe32e81e
plan_head_after: e48e08c6805805330997ac79df906a8bacfe9ce5
actuals:
  tokens: 36000
  tasks: 3
  commits: 3
---

# Phase 21 Plan 12: Allow heals the tab, hidden tab and window, one tab per peer, control mode, held keys, Cmd+V Summary

**In the real Portty.app on real macOS, turning Local Network on heals the open Desktop tab on the first redial with no click on it (picture streams, office counts 1 viewer); a Desktop tab behind a terminal tab keeps decoding; a closed window releases the office in about 2 minutes and the picture is back 1.3 s after reopening; control mode sends Cmd+W, Cmd+Q and Cmd+C to the office only and gives the keys back on every exit with nothing held; Cmd+V pastes new text, sends only the key for unchanged text and refuses a hidden item with the notice.**

## Closure verdicts (what 21-13 reads)

- `DVWR01_VM: pass` (rows D3, D4, D5, D6)
- `DVWR04_VM: pass` (rows K6, H2, P2)
- `PASTE_ALERT: none` (no macOS paste alert on Portty's read of the home pasteboard in portty-test-2)
- `DEFERRED_UAT:` label, chip and card legibility in Light and Dark over the translucent window

## What was done

- **Task 1 (tracer, row D3-pre):** both guests online. The app had already connected on its own when the plan started (the denied state decays, as 21-11 warned), so the run restarted it: GUI quit, `start2`, a click on the Desktop row, the card after 12 s, and in the next 30 s two new peer connections that logged `gathering` and nothing else. Office TextEdit and the office probe were ready.
- **Task 2 (checkpoint:human-action, done by the executor in the guest):** Portty's Local Network switch pressed twice in fresh launches with AXPress. In both runs the first peer connection built after the press completed gathering within about 130 ms and connected, and none built before it did. Press to ICE connected: 2 to 8 s (run 1), 14 to 19 s (run 2). Picture confirmed from the guest's own screenshot (the office desktop drawn inside Portty's translucent frame) and from framesDecoded rising; office reported 1 viewer. Light and Dark: see the deferral.
- **Task 3:** rows D4 (hidden tab keeps decoding: framesDecoded 3065 to 3172 in 11 s behind a terminal tab, 1 viewer), D5, D6 (one Desktop tab, 1 viewer), K6, H2, P2, and the closure lines.

Numbers worth keeping, all in `21-VM-CHECKS.md`:

- **D5:** `WINHIDDEN hidden=true` at app time 569.5 s; the page's own stats stopped 120.9 s later; the office showed 0 viewers and no display-sleep assertion at the poll 131 s after the click (still 1 at 113 s). Reopen: peer connection 911.3 s, ICE connected 911.9 s, picture back.
- **K6:** Cmd+C copied on the office (18 bytes) with the home pasteboard unchanged (same hash); Cmd+W removed the office `[Untitled]` window with Portty keeping its tab and pid; Cmd+Q quit office TextEdit; the release chord returned the label to `Click the picture to control` and the next Cmd+W closed the Desktop tab; with pf blocking, the tab showed `Reconnecting`, the label `Not connected`, and Cmd+W closed the dead tab.
- **H2:** office OS key state `none` after a 1000 ms Cmd+C hold; `Cmd` held while a 3000 ms hold was running, `none` after (the C goes out as a tap under the Cmd-chord rule).
- **P2:** `alpha` pasted, then `alphabeta` (key only, the office pasted its own clipboard), then no change and the notice `Your clipboard holds a hidden item (password manager); not sent` for a concealed home item.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] The key tool was gone from portty-test-2.** 21-02's `remove2` had deleted `~/p21/key`. Rebuilt it from `rig/key.swift` on the host (compile only) and installed it with the existing `tools2`.
- **Found during:** pre-task read of the rig
- **Files modified:** none

**2. [Rule 2 - Missing functionality in the rig] A held-key probe did not exist.** The plan's assumption table allows the daemon's input summary at viewer leave, but the summary (and `probe control`) carry message-kind counters only. Added `rig/keystate.swift` (OS key state of a guest) and `rig/axswitch.swift` (read or AXPress the switch after a label), both compiled on the host and run only in the guests, with `tools12`, `axswitch` and `keystate` commands in `p21-rig.sh`. This measures the thing that matters (no key left down on the office) and was checked against a live hold.
- **Files modified:** `rig/p21-rig.sh`, `rig/axswitch.swift`, `rig/keystate.swift`
- **Commit:** `ae1a1790`

**3. [Process] The Task 2 checkpoint was not returned.** The standing rule (never hand a native-GUI check to the user) and the orchestrator's checkpoint policy had the executor toggle the switch in the guest. No guest was logged out of the tailnet, and the GUI route could flip the switch, so no re-key checkpoint applied. The Light and Dark sub-item is deferred (below), as the policy allows.

**4. [Process] Commit trailer.** The harness prescribes `Claude Sonnet 5.5`, which wrote these commits; the plan's `Claude Opus 5.5` was not used (same call 21-10 and 21-11 made).

**5. [Process] Task 1's precondition text assumed the card was still showing.** It was not (the app had connected unaided), so Task 1 started from a fresh launch before recording D3-pre.

No FAIL verdicts, so there is nothing for the plan to report as failing and no code workaround was applied.

## Findings for 21-13 and later

- **A denied tab does not stay denied.** With the switch off, ICE connected on its own in this plan too (5th peer connection at 93 s in D3-pre, 4th at 61 s in a discarded attempt; 21-11 saw 43 s, 93 s, 235 s). Cause not found. It means the Local Network card is not a stable state in the real app and a single run cannot prove that Allow was the cause. D3's evidence is two presses healing on the first redial (2 to 8 s and 14 to 19 s) versus 43 to 235 s unaided, with every peer connection built before a press stuck at `gathering`.
- **Screenshots in the guests omit the page's DOM.** The guest's `screencapture` shows the video layer and the window frame but none of the page's controls, so on-screen readability of the label and cards cannot be judged there. Contrast computed from the shipped CSS tokens is 4.77 or more in Light and 4.97 or more in Dark for every pair used. Deferred to real-Mac UAT.
- **Banners can intercept a screen-position click.** The macOS "App Background Activity" banners that appear after launch can cover a settings switch; use AXPress for settings switches.
- **Cmd+T opens the New Session dialog**, not a tab (Create Session is the next click).
- **Stuck Control on the home guest, once.** After the run's last release chord the home guest's OS key state read `Control` held (the office never did); a plain key tap cleared it and three later identical chords left nothing. It would also explain one Desktop-row click that did nothing. Suspect the rig key tool, not Portty; worth one line in real-Mac UAT: after the release chord, does the next click act as a plain click?

## Items deferred to real-Mac UAT

- Legibility of the toolbar label, chips and cards in Light and Dark over the translucent window (measured contrast only; recorded on the `DEFERRED_UAT:` line).
- After the release chord, a plain click behaves as a plain click (observation above).

## VM end state

- Both guests running and `online=yes` (`tailnet-health` at the end); neither stopped.
- Office (portty-test): vmdesktop Developer ID build 21.11.2 untouched, daemon running, always-available on and its own assertion unbroken, GUI not running, TextEdit not running, 0 viewers, `keystate` none, clipboard holds a neutral word.
- Home (portty-test-2): Developer ID build of HEAD `b982e560` plus the stats probe at `/Applications/Portty.app`, GUI running (pid unchanged since the second D3 run), no Desktop tab open (sidebar still lists the `shell 1` session), appearance Light, **Portty's Local Network switch is ON** (Google Chrome's also on), System Settings open on the Local Network page, `keystate` none, pf anchor empty, pasteboard holds a neutral word, `~/p21/key`, `~/p21/axswitch`, `~/p21/keystate` installed, `~/p21/keyprobe.log` holding the session's stats lines.

## Known Stubs

None. No source file under `frontend/`, `internal/` or the Go root changed; only the rig and `.planning` files.

## Threat Flags

None new. Evidence was scrubbed on capture (`scrub_ax`); the verify grep for tailnet addresses, `.ts.net` names, `cap=` values and home paths found none, and `bash tests/check-brand.sh` is clean. Every key, click and read ran inside the guests; nothing ran against the host, the host's tailscaled or any 100.x address, and pf (never `tailscale down`) simulated the network loss. The one notable security-adjacent observation is the paste path: a concealed item sent nothing and sent no key (fail closed), matching the threat model.

Verification run: the plan's automated checks (rows present, each closure line exactly once, no tailnet address, `.ts.net` name or cap value in evidence or the checks file, `check-brand.sh`) pass; `bash -n` on `p21-rig.sh` passes; `git status --porcelain` is clean after the commits. No Go or frontend test was run because nothing in them changed.

## Self-Check: PASSED

Created files (`rig/axswitch.swift`, `rig/keystate.swift`, the evidence files named in the rows, this summary) and the three task commits (bf2ac2e4, ae1a1790, e48e08c6) verified present.
