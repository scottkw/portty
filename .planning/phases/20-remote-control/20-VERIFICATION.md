---
phase: 20-remote-control
verified: 2026-11-26T22:40:00Z
status: human_needed
score: 4/4 must-haves verified
covered_files:
  - ".planning/phases/20-remote-control/20-01-PLAN.md"
  - ".planning/phases/20-remote-control/20-01-SUMMARY.md"
  - ".planning/phases/20-remote-control/20-02-PLAN.md"
  - ".planning/phases/20-remote-control/20-02-SUMMARY.md"
  - ".planning/phases/20-remote-control/20-03-PLAN.md"
  - ".planning/phases/20-remote-control/20-03-SUMMARY.md"
  - ".planning/phases/20-remote-control/20-04-PLAN.md"
  - ".planning/phases/20-remote-control/20-04-SUMMARY.md"
  - ".planning/phases/20-remote-control/20-05-PLAN.md"
  - ".planning/phases/20-remote-control/20-05-SUMMARY.md"
  - ".planning/phases/20-remote-control/20-06-PLAN.md"
  - ".planning/phases/20-remote-control/20-06-SUMMARY.md"
  - ".planning/phases/20-remote-control/20-07-PLAN.md"
  - ".planning/phases/20-remote-control/20-07-SUMMARY.md"
  - ".planning/phases/20-remote-control/20-08-PLAN.md"
  - ".planning/phases/20-remote-control/20-08-SUMMARY.md"
  - ".planning/phases/20-remote-control/20-09-PLAN.md"
  - ".planning/phases/20-remote-control/20-09-SUMMARY.md"
  - ".planning/phases/20-remote-control/20-10-PLAN.md"
  - ".planning/phases/20-remote-control/20-10-SUMMARY.md"
  - ".planning/phases/20-remote-control/20-11-PLAN.md"
  - ".planning/phases/20-remote-control/20-11-SUMMARY.md"
  - "frontend/src/components/DesktopClipboardPopover.tsx"
  - "frontend/src/components/DesktopControlBar.tsx"
  - "frontend/src/components/DesktopSendKeysMenu.tsx"
  - "frontend/src/components/DesktopView.tsx"
  - "frontend/src/lib/desktopClient.ts"
  - "frontend/src/lib/desktopInput.ts"
  - "internal/desktop/clipboard.go"
  - "internal/desktop/control.go"
  - "internal/desktop/gates.go"
  - "internal/desktop/input.go"
  - "internal/desktop/input_map.go"
  - "internal/desktop/manager.go"
  - "internal/desktop/surface.go"
  - "internal/desktop/window.go"
  - "internal/screencap/input_darwin.m"
  - "internal/screencap/keytable.go"
  - "internal/screencap/pasteboard_darwin.m"
  - "internal/screencap/window_darwin.m"
covered_digest: "v4:sha256:b96349f862c24644a32bc9849303504052a7cc33f03c9bf6f181bdfe9c639e8b"
behavior_unverified: 0
overrides_applied: 0
human_verification:
  - test: "M-180: pointer accuracy on the real Retina display and on an external display, at several viewer window sizes, plus scrolling with a real trackpad (natural scrolling on and off)"
    expected: "A click and a drag land on the pixel under the viewer's pointer on both displays; scroll direction matches the viewer's gesture"
    why_human: "The VM has no Retina or external display and no trackpad. The code maps normalized viewer coordinates onto display bounds in points (input_map.go mapPoint, surface.go target), which is correct by reading and by TestInputMap_Point, but the hardware result is unmeasured. VM row S1 only shows the sign rule for posted pixel scrolls."
  - test: "M-181: input methods, emoji picker, dictation and dead keys from a non-US viewer layout"
    expected: "Composed text arrives once and as typed, with no doubled or dropped characters"
    why_human: "The IME guard (isComposing, keyCode 229, Dead) is unit-tested with synthetic events only. Real IME timing differs per browser (Safari fires compositionend before the committing keydown)."
  - test: "M-182 and M-183: nothing stays held after a Cmd chord, a closed tab, a sleeping home Mac or a dropped network; Send Keys and the Ctrl/Cmd swap from a non-Mac keyboard. Repeat this after the post-VM fix H-02 (the Cmd-releases-non-modifiers rule in desktopInput.ts)"
    expected: "A key or modifier pressed before Cmd and released while Cmd is down is not left held; Cmd+Tab, Cmd+Space and Lock Screen work from the menu; the swap maps Ctrl to Cmd"
    why_human: "H-02 is viewer-side only, covered by two vitest cases and not re-run in a VM. Real-keyboard modifier ordering and macOS dropping the keyup under Cmd cannot be reproduced in jsdom."
  - test: "M-184, M-185, M-186, M-187: window share on the office Mac (raise, confinement, banner, a hung app); what a sheet, an Open/Save panel and a pop-up menu look like in the decoded window frame; clicking a pop-up menu of the shared app (post-fix e9dfa052, layer 101 and up only) while another app's window still cannot be clicked; the AX focus read staying valid (VM row X3 saw -25204 for about 10 minutes)"
    expected: "Pop-up menu item click lands; a sheet, Fonts panel or other app's window drops as covered; the gate does not fail closed for long periods on the real Macs"
    why_human: "VM row W3 was NOT RUN (the decoded frame could not be read) and row X1 was recorded before the carve-out was narrowed. The T-20-21 narrowing is unit-tested (TestWindow_SameAppPopupAllowed and five drop rows) but never re-run on a Mac."
  - test: "M-188: display switching with an external display, an unplugged display, a closed shared window, and the in-band resolution change on iPhone Safari and in Portty's own window. Re-run after the post-VM fixes M-01 (failed switch resync and fallback restart), M-03 (resize deferred while a button is held) and M-04 (off-display moves are not effect misses)"
    expected: "The viewer never reconnects; the picture and the pointer mapping always agree after a switch, including a switch that fails; a remote drag-resize is not cut by the resize; moves onto off-screen parts of a window do not trip Control needs Accessibility"
    why_human: "VM rows D1 and D2 ran before these four fixes and covered Safari and Chrome only. The fixes have Go unit tests, with fakes for the capture source and the display list."
  - test: "M-189 and M-190: clipboard both ways in Chrome and Safari, with a real click on Copy from Mac and Safari's own Paste prompt; a password-manager item that never copies; a 1 MB text; Paste from Other Apps on the corporate Mac"
    expected: "Copy and Paste work by an explicit action only; a concealed item answers the hidden-item notice and the viewer's clipboard keeps its old content; a 1 MB text is refused cleanly; the Mac shows no paste alert, or a recorded one"
    why_human: "VM row P2 drove the menu with JavaScript click(), which gives no Safari user activation, so only the fallback popovers were exercised. The waiting_approval path (TestClipboard_WaitingApproval) was never reached with a real macOS paste alert. The M-06 changeCount snapshot (7970be7f) is source-checked only, because no test may read the host pasteboard."
  - test: "M-191 and M-192: Chrome Keyboard Lock and the full-screen notice in Safari and Firefox; the control bar, menus, popovers and chips in light and dark; typing into a password field and Caps Lock reaching the office Mac. Also the hidden control bar (post-fix UI-1, 299d842f): a press inside the bar's box while it is hidden reveals the bar and posts nothing to the Mac"
    expected: "Keyboard Lock behaves as documented; the bar's reveal zone matches its real box; a password field fills (a field that stays empty means Secure Event Input is dropping posted keys); Caps Lock state arrives"
    why_human: "Visual and browser-specific behaviour. Research assumptions A5 (posts into Secure Event Input fields) and A8 (Caps Lock through alpha-shift) were not run. UI-1 has two jsdom cases with a mocked bounding rect only."
  - test: "M-193: a viewer that cannot get a picture says so and never says Streaming ahead of one (VM row X2 inferred connected-before-first-frame but could not read connectionState)"
    expected: "The chip never reads Streaming before the first decoded frame, in Safari, Chrome, the Wails web view and iPhone"
    why_human: "The debug hook does not expose connectionState, and the Wails web view and iPhone were not run."
  - test: "Decision on deferred review finding M-05 (window-share key gate versus global hotkeys and non-activating panels such as Alfred or Raycast) and on the deferred lows L-01 to L-06 and infos I-01 and I-02"
    expected: "The owner accepts these as follow-ups, or asks for a gap-closure plan. M-05 is the one that matters: after Option+Space opens a launcher panel, keys can pass the window gate and land in a panel the window capture does not show. Clicks stay confined (the topmost hit test is unaffected). The blocklist covers only Cmd+Tab, Cmd+Space and Ctrl+Cmd+Q."
    why_human: "Deferred by the orchestrator's explicit decision (20-REVIEW-FIX.md). The ROADMAP wording for window shares promises that clicks never land outside the window, which holds, and says nothing about keys. Whether to accept the key-gate residual risk is the owner's call."
---

# Phase 20: Remote Control Verification Report

**Phase Goal:** A viewer drives the shared Mac as if sitting at it, with pointer, keyboard, system shortcuts and clipboard. The sharer chooses a whole display or a single window, and displays can be switched mid-session.
**Verified:** 2026-11-26T22:40:00Z
**Status:** human_needed
**Re-verification:** No, initial verification

Everything that can be verified without real hardware passes: code on disk, unit tests and the six blocking VM rows. What is left is hardware-only (Retina and external display mapping, real IME, trackpad, Safari clipboard gesture, real pop-up menu) and the post-VM fixes that have unit tests but no VM re-run. Nothing automatable failed.

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | A viewer with control moves, clicks, scrolls and drags, and the pointer lands on the right spot on Retina and non-Retina displays at any viewer window size (DCTL-01) | VERIFIED (hardware half to human) | Viewer: `createDesktopInput` in `frontend/src/lib/desktopInput.ts` normalizes against the letterbox rect from `videoContentRect` (`DesktopView.tsx:54`, wired at `:437` and `:480`). Daemon: `mapPoint` (`internal/desktop/input_map.go:26`) multiplies the normalized point by the surface bounds in global points and clamps one point inside the far edge, and `surface.go:target()` takes the bounds from `DisplayBounds`. Points, not pixels, so Retina scale cancels. Stale-epoch presses are dropped (`gates.go`). Wheel sign rule is `scrollDelta` (`input_map.go`). VM E2: a click at a normalized point put the caret at the character under it. VM E1 and S1 also passed. Retina and external display: M-180. |
| 2 | Text with shortcuts and non-ASCII arrives as typed. No key or modifier stays held after blur, close or disconnect. Send Keys sends Cmd+Tab, Cmd+Space and Lock Screen. Optional Ctrl to Cmd swap (DCTL-02, DCTL-03) | VERIFIED (IME and real keyboard to human) | Text: `handleText` in `input.go`, `splitText` and `MAX_TEXT_BYTES` in `desktopInput.ts`. VM T1: a 1024-byte accents, emoji and ZWJ string arrived byte-identical (sha1 match). Keys go through the closed table `internal/screencap/keytable.go`. Held-state release: `releaseHeld`, `endViewerInput`, `releaseAllViewers` and the 3 s dead-man (`input_map.go deadManAfter`, `watchdog.go`); viewer release triggers blur, pagehide, visibilitychange, channel loss, view-only and the release chord (`desktopInput.ts:376-432`). VM H1: the viewer left with Shift held, and the next posted letter was unshifted. H-01 (daemon shutdown cancelled the input goroutine before the releases posted) is fixed in a8c3ceb0, `TestInput_CloseReleasesHeldAfterStartContextCancelled` PASS. H-02 (key pressed before Cmd) is fixed viewer-side in `releaseUnderCmd` (`desktopInput.ts:300`). Send Keys: `DesktopSendKeysMenu.tsx` (`CMD_TAB`, `CMD_SPACE`, Lock confirm), wired in `DesktopControlBar.tsx:563`. The Lock Screen confirm sends Ctrl+Cmd+Q, and the daemon sets an optimistic lock (`gates.go inputLocked`). VM C1 (Cmd+Space, Cmd+Tab) and C2 (Lock Screen locked in under 1 s, typing refused while locked) PASS. Swap: `swapCode` and `maskOf` in `desktopInput.ts`, persisted under `SWAP_KEY`. |
| 3 | The sharer picks a display or a single window, and displays can be switched mid-session without the viewer reconnecting. In a window share the window comes to the front when controlled and clicks never land outside it (DESK-05, DESK-06, DCTL-05) | VERIFIED (window-frame content and post-fix rechecks to human) | Picker: `handleSurfaces` and the select handler in `surface.go`, with `Surface` rights; the bar menu in `DesktopControlBar.tsx:335-361` calls `clientRef.selectSurface` (`DesktopView.tsx:993-998`). Mid-session switch is on the running stream, with an epoch bump and release (no renegotiation). VM D1: display, window, display, window in Safari and Chrome, with one `viewer joined` and four `surface switched`, no second join, `framesDecoded` rising, no Reconnecting. D2: the new size reached the viewer in 0.84 s. Failure paths (M-01) fixed in 1dc75276. Confinement: `windowAllows` (`window.go:155-223`) refuses a press unless a live AX read says the shared app is focused, raises first (`startRaise`, drop then retry), checks `outside` against the previous and fresh frame, then a topmost hit test (`covered`). `windowChord` drops Cmd+Tab, Cmd+Space and Ctrl+Cmd+Q. `clampToWindow` clamps moves. `sameAppPopup` (T-20-21) is narrowed to the same pid at layer 101 and up. VM W1: raise from the launchd daemon by AX rung 1. W2: inside click landed; a click on the Fonts panel and one on a covered part dropped. W3 NOT RUN, X1 OBSERVED and recorded as M-185/M-186. |
| 4 | Text copies from the shared Mac to the viewer and pastes from the viewer to the Mac only by an explicit action and never in the background. Items a password manager marks concealed never sync (DCTL-06) | VERIFIED (real Safari gesture to human) | Daemon reads only inside `handleClipGet`, which is triggered by a viewer's `clip_get`; there is no pasteboard poll anywhere in the daemon (`pasteboard_darwin.m` header says so; `changeCount` is read only around one read, M-06). Types are checked first: `org.nspasteboard.ConcealedType`, `TransientType` and `com.agilebits.onepassword` answer `concealed` with no content read (`pasteboard_darwin.m:47-56`, `clipboard.go:178`). Chunking is 16 KiB with strict reassembly (`chunkText`, `accept`, `TestClipboard_ChunkBoundaries/PutReassembly/PutRejects`). Viewer: Copy/Paste items in the bar (`DesktopControlBar.tsx:317-318`), notices and fallback popovers in `DesktopView.tsx`. VM P2: `ok bytes=8`, `ok bytes=6`, `concealed bytes=0`, with the viewer's clipboard keeping its marker. P1 and P3: no paste alert, `accessBehavior` alwaysAllow. |

**Score:** 4/4 truths verified (0 present-but-behavior-unverified). The hardware-only halves are in `human_verification`.

### Required Artifacts (spot-verified on disk, all three levels)

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/desktop/input.go` (911 lines) | single `handleInput`, queue, release-all, dead-man | VERIFIED | Substantive and wired. `handleInput` dispatches move, rmove, wheel, btn, key, text, release and hb. |
| `internal/desktop/gates.go` | lock, epoch, accessibility, window, rate gates | VERIFIED | `pressAllowed` runs every gate in order. |
| `internal/desktop/window.go` | window confinement | VERIFIED | `windowAllows`, `startRaise`, `clampToWindow`, `windowLoop`. |
| `internal/desktop/surface.go` | picker and switch | VERIFIED | `target`, `handleSurfaces`, resize deferral (M-03), failed-switch resync (M-01). |
| `internal/desktop/clipboard.go` | request-only clipboard | VERIFIED | `handleClipGet`, `handleClipPut`, `chunkText`, `clipGate`. |
| `internal/screencap/pasteboard_darwin.m` | types-first read | VERIFIED | `pcPbReadOnce`, retry once, `@autoreleasepool` around every entry. |
| `internal/screencap/keytable.go`, `input_darwin.m`, `window_darwin.m` | native bridges | VERIFIED | Compiled and wired through `Deps` defaults (`manager.go:421-431` sets `PointOnDisplay` and `Window`). Bridge calls refuse under `go test`. |
| `frontend/src/lib/desktopInput.ts` | viewer input layer | VERIFIED | IME guard, swap, heartbeat, release triggers, Cmd-chord handling. |
| `DesktopControlBar.tsx`, `DesktopSendKeysMenu.tsx`, `DesktopClipboardPopover.tsx`, `DesktopView.tsx` | control bar, Send Keys, clipboard UI | VERIFIED | Wired in `DesktopView.tsx` (`createDesktopInput` at `:472`, bar props at `:989-1005`). |

### Key Link Verification

| From | To | Via | Status |
|------|----|-----|--------|
| `DesktopView` | `createDesktopInput` | `desktopInput.ts` | WIRED (`DesktopView.tsx:33,472`) |
| `DesktopView` | `desktopClient.selectSurface` | signalling `select` | WIRED (`:998`) |
| `DesktopControlBar` | `DesktopSendKeysMenu` | sendChord | WIRED (`:563`) |
| data channel `input` | `Manager.handleInput` | `acceptDataChannel` to `enqueueInput` to `inputLoop` | WIRED (`input.go:283-395`, VM E1 `key` +20) |
| `Manager` Deps | native bridges | `manager.go:421-431` defaults | WIRED |
| `pressAllowed` | `windowAllows` | `gates.go` | WIRED when `windowShared()` |

### Data-Flow Trace (Level 4)

| Artifact | Data | Source | Real data | Status |
|----------|------|--------|-----------|--------|
| Pointer target | `inputTarget` | `DisplayBounds` or the AX window frame (`readWindow`) | yes (VM E2) | FLOWING |
| Surface menu | displays and windows | `ListSurfaces` native list | yes (VM D1) | FLOWING |
| Clipboard copy | `clip_data` text | `NSPasteboard` read on request | yes (VM P2) | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Go desktop, screencap, daemon, tailnet packages | `GOTOOLCHAIN=go1.26.11 go test -race -short -count=1 ./internal/desktop/ ./internal/screencap/ ./internal/daemon/ ./internal/tailnet/` | all four ok (desktop 64s, screencap 2s, daemon 118s, tailnet 7s) | PASS |
| Named post-fix tests (H-01, M-01, M-03, M-04, M-02, clipboard) | `go test -run '<names>\|TestClipboard' -v ./internal/desktop/` | 13 PASS, 0 FAIL | PASS |
| Phase 20 frontend suites | `vitest run` on desktopInput, desktopClient, DesktopControlBar, DesktopSendKeysMenu, DesktopView | 5 files, 349 tests pass | PASS |
| Full frontend | `pnpm --dir frontend test` | 135 files, 2408 tests pass (includes Phase 21 files) | PASS |
| Traceability | `bash tests/check-traceability-paths.sh` | OK: 295 paths exist | PASS |

### Probe Execution

Step 7c: SKIPPED. No `probe-*.sh` is declared by any Phase 20 plan. The runtime rig is `rig/p20-rig.sh`, whose results are in `20-VM-CHECKS.md`.

### VM Evidence (20-VM-CHECKS.md; evidence files exist in `evidence/`)

The six blocking rows E1, C2, W2, D1, P2 and H1 passed on macOS 27.2 in Tart VMs. E2, C1, T1, W1, D2, P3, A1 and A2 also passed. S1, P1 and X1 to X3 are OBSERVED. W3 is NOT RUN. These runs predate the later fixes H-01, H-02, M-01, M-02/T-20-21, M-03, M-04, M-06, M-07 and UI-1, which have unit tests and no VM re-run (listed in `human_verification`).

### Requirements Coverage

| Requirement | Source Plans | Description | Status | Evidence |
|-------------|--------------|-------------|--------|----------|
| DCTL-01 | 20-01, 02, 06, 09, 10, 11 | Pointer move, click, scroll, drag; right spot on Retina | SATISFIED in code, Retina hardware to human (M-180) | Truth 1 |
| DCTL-02 | 20-01, 02, 06, 10, 11 | Typing, shortcuts, non-ASCII, no stuck keys | SATISFIED, IME to human (M-181, M-182) | Truth 2 |
| DCTL-03 | 20-01, 06, 07, 10, 11 | Send Keys menu and Ctrl to Cmd swap | SATISFIED, real keyboard to human (M-183) | Truth 2 |
| DCTL-05 | 20-04, 07, 08, 10, 11 | Window raised when controlled, clicks confined | SATISFIED for clicks; key-gate residual risk M-05 deferred; pop-up and sheet behaviour to human (M-184 to M-187) | Truth 3 |
| DCTL-06 | 20-05, 08, 09, 10, 11 | Explicit clipboard, concealed items never sync | SATISFIED, Safari gesture to human (M-189, M-190) | Truth 4 |
| DESK-05 | 20-03, 04, 07, 09, 10, 11 | Pick a whole display or a single window | SATISFIED, window-frame content to human (M-185, M-188) | Truth 3 |
| DESK-06 | 20-03, 07, 10, 11 | Displays switch mid-session | SATISFIED, post-fix and iPhone rechecks to human (M-188) | Truth 3 |

All seven IDs in the ROADMAP Phase 20 line appear in the PLAN `requirements` fields (20-10 and 20-11 declare all seven). No orphaned requirement: REQUIREMENTS.md maps exactly these seven to Phase 20, and DCTL-04 belongs to Phase 22. The REQUIREMENTS.md checkboxes and traceability table for these seven still read Pending and are not updated here (the orchestrator updates them at phase close).

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| (none) | n/a | `TBD`, `FIXME` and `XXX` grep over the phase's implementation files: no match (one false hit, `\udXXX` in a comment at `desktopClient.ts:182`). TODO/HACK: none | none | n/a |

Review status (`20-REVIEW.md`, `20-REVIEW-FIX.md`): H-01, H-02, M-01 to M-04, M-06, M-07, UI-1 and T-20-21 are fixed with regression tests. `20-SECURITY.md` reads SECURED, 52 of 52 threats closed. Deferred by decision: M-05 (window-share key gate versus global hotkeys and non-activating panels), L-01 to L-06 and I-01, I-02. UI review scored 18/24.

### Human Verification Required

See the `human_verification` list in the frontmatter (owner-run TESTING.md Section 5 items M-180 to M-193, which are the manual checklist). These are listed because only real hardware or a real browser gesture can show them, or because the fix landed after the VM run. None is a known failure.

### Gaps Summary

No gaps: every ROADMAP success criterion has code on disk, passing unit tests, and (for the six blocking rows) real macOS VM evidence. The status is `human_needed` because of:

1. Hardware-only behaviour: Retina and external display mapping, real IME and dictation, real trackpad scroll, Safari clipboard from a real click, a real macOS paste alert, pop-up menu in a decoded window frame (W3, never run).
2. Post-VM fixes with unit tests only: H-02, M-01, M-02/T-20-21, M-03, M-04, UI-1 (plus the source-checked M-06 and M-07).
3. One owner decision: accept or schedule M-05, the window-share key-gate hardening. It does not break the stated criterion ("clicks never land outside it") but it is a confinement weakness for keystrokes in a window share.

---

_Verified: 2026-11-26T22:40:00Z_
_Verifier: Claude (gsd-verifier)_
