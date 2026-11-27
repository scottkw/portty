---
phase: 21-portty-app-and-iphone-viewers
plan: 09
subsystem: desktop-control-mode
tags: [control-mode, release-all, cmd-v-paste, clipboard, keyroute-page, release-chord]
requires:
  - phase: 21-02
    provides: "KEYROUTE: page, CMDKEYUP: swallowed, RELEASE_CHORD: taken"
  - phase: 21-08
    provides: DesktopTab, DesktopView onCaptureChange and controlRef handle, desktopTabLabel
provides:
  - controlReducer (six events to controlling and ordered actions, release-all first)
  - DesktopTab control-mode exits (click away, tab switch, window blur or hide, stream not connected, hidden-window pause, tab close)
  - Cmd+V flow (pasteDecision, isPasteChord, PASTE_NOTICES, ReadHomeClipboard binding)
  - the app's release chord Ctrl+Option+Cmd+Shift+Return in the label, tooltips and stage description
affects: [21-12, 21-13]
tech-stack:
  added: []
  patterns: [capture-phase listener on the tab's own wrapper, replay of the keydown into Phase 20's handler, fail-closed paste decision]
key-files:
  created:
    - frontend/src/lib/controlMode.ts
    - frontend/src/lib/__tests__/controlMode.test.ts
    - app_clipboard_paste.go
    - app_clipboard_paste_test.go
  modified:
    - frontend/src/components/DesktopTab.tsx
    - frontend/src/components/__tests__/DesktopTab.test.tsx
    - frontend/src/components/DesktopView.tsx
    - frontend/src/components/DesktopControlBar.tsx
    - frontend/src/lib/peerDesktop.ts
    - frontend/src/lib/desktopInput.ts
    - frontend/src/lib/__tests__/peerDesktop.test.ts
    - frontend/src/components/__tests__/TabBar.test.tsx
    - frontend/src/wailsjs/go/main/App.d.ts
    - frontend/src/wailsjs/go/main/App.js
key-decisions:
  - "KEYROUTE page: no control_menu_darwin.* and main.go untouched; the reducer runs with native false (a NATIVE_MENU constant) and never returns menuOn or menuOff to a caller that acts on them"
  - "After the paste, the V tap is sent by replaying the user's keydown into Phase 20's own stage handler (guarded by a flag so the capture listener skips it), not by calling encodeKey directly: the Swap setting, the Cmd-chord tap rule and the modifier mask then come from the one place that owns them"
  - "App release chord Ctrl+Option+Cmd+Shift+Return (CONTEXT, a4798f16): named in the label, label tooltip, Control tooltip and stage description through an optional releaseChord prop; the browser viewer keeps its strings; isReleaseChord already ignores Shift so no behaviour change was needed"
  - "Tab close releases from a layout-effect cleanup because desktopInput.destroy() sends no release and the viewer handle is already detached by the time passive cleanups run"
patterns-established:
  - "A paste outcome other than ok sends no key (too_large keeps its own notice, every other outcome reads as the failed-send notice)"
requirements-completed: [DVWR-04]
status: complete
actuals:
  tokens: 42000
  tasks: 3
  commits: 4
plan_head_before: cb12fb488fa559a542c2da5647acc64912789f2c
plan_head_after: c292ab307df94505cbe031da4dfdcbfafe72e7ac
---

# Phase 21 Plan 09: Control mode in the Desktop tab Summary

**Control mode is Phase 20's keyboard-captured state: Cmd+W, Cmd+Q and Cmd+C go to the remote Mac as taps with the local default stopped, every exit releases everything first, and Cmd+V sends the home clipboard's plain text through Phase 20's paste message and then the key, never a concealed item.**

## Precondition record

Phase 20 SUMMARYs 20-05 through 20-08 exist and 21-VM-CHECKS.md carries `KEYROUTE: page`, `CMDKEYUP: swallowed`, `RELEASE_CHORD: taken`. Phase 20's built code matched the plan's assumptions: `screencap.NewPasteboard().ReadText` with ReadOK, ReadConcealed, ReadEmpty, ReadTooLarge, ReadDenied and `ErrRefusedInTest`; the Cmd-chord tap rule and `releaseAll` in `desktopInput.ts`; `DesktopClient.pasteToMac` and its outcomes; `encodeKey` (not called directly, see decisions).

## What was built

- **controlReducer** (`frontend/src/lib/controlMode.ts`): capture-on turns control on; every other event turns it off with `releaseAll` first, then `releaseKeyboard` for tab-inactive, window-blur and not-connected, then `menuOff` only when native. Only tab-close acts while already off.
- **DesktopTab**: the reducer is fed by `onCaptureChange`, the `active` prop, `window` blur, `visibilitychange` to hidden, any status other than streaming, the 2-minute hidden pause, and unmount. Actions are carried out through the viewer's `controlRef` handle (its own merged ref, so a caller's `controlRef` still works). The tab never reacts to locked, so a Mac that locks keeps control mode on. No `window`/`document` key listener exists.
- **Cmd+V**: one capture-phase `keydown` listener on a `display: contents` wrapper of the tab handles only `isPasteChord` while controlling. It stops the event (also when ignored, so a plain V never leaks behind a paste), ignores repeats and a paste already in flight, reads through `ReadHomeClipboard`, applies `pasteDecision`, calls `pasteToMac` for new text, and then replays the keydown into Phase 20's handler so the V tap goes out with the right mask and Swap. `lastSent` is one in-memory ref per tab. If control mode ended during the read or send, no key goes out.
- **ReadHomeClipboard** (`app_clipboard_paste.go`): maps the bridge's outcomes to ok, concealed, empty, too_large and failed; text is carried only for ok, a concealed item never carries text, a refused read or an error is failed, and a backstop makes 1048577 bytes too_large while 1048576 passes. Nothing is logged. Under a test binary the real bridge refuses and the binding reports failed.
- **Release chord**: the label line 2, label tooltip, Control tooltip and the stage's screen-reader description name Ctrl+Option+Cmd+Shift+Return in the app. `DesktopView` and `DesktopControlBar` gained an optional `releaseChord` prop (default unchanged), and the viewer handle's `notify` an optional dwell so paste notices keep their 5 s.
- **KEYROUTE page (Task 3):** page-level handling suffices, no native code. `control_menu_darwin.go`, `control_menu_darwin.m` and `control_menu_darwin_test.go` were not created and `main.go` is untouched (`git status` for it prints nothing). There is no `SetControlMode` binding and no heartbeat.

## Tests added

- `controlMode.test.ts`: every event from on and off, `isPasteChord`, `pasteDecision` (each row of the UI-SPEC Cmd+V table, concealed never carrying text, unknown outcome failing closed).
- `DesktopTab.test.tsx` (control mode block, 22 cases): the tracer (click, Cmd+W as a tap with `defaultPrevented`, tab switch sends release and blurs the input), click away, window blur and focus back, hidden window, stream leaving Streaming, close sends release before `client.stop`, lock keeps control, the Shift release chord, the Control tooltip, Cmd+C as a tap with no clipboard read, and the Cmd+V cases (new text and mask 8, same text, no text, concealed, too large, failed read, rejected read, failed send not remembered, second press in flight, leaving control mid-paste, Cmd+Shift+V plain, Cmd+V while not controlling).
- `app_clipboard_paste_test.go`: five `TestReadHomeClipboard_*` tests through the seam, including the 1048576 and 1048577 boundary and the real bridge refusing under test.

Verification: `tsc --noEmit` clean; `vitest run` 2447 passed (136 files); `go vet .` and `gofmt` clean; `go test -race -run TestReadHomeClipboard .` passes; `bash tests/check-brand.sh` clean. `go test -race -count=1 .` passes except `TestNoFakeWhoIsInRelease`, which fails only because `frontend/dist` is not built in this worktree (`pattern all:frontend/dist: no matching files found`); the same unrelated failure 21-08 recorded. `pnpm exec` refuses in this worktree (the symlinked `node_modules` makes pnpm try to reinstall), so tsc and vitest were run from `frontend/node_modules/.bin` directly; the symlink is untracked and was never staged.

## TDD Gate Compliance

Task 2 (`tdd="true"`) ran RED then GREEN. The plan itself is `type: execute`, so the plan-level gate does not apply.

- **RED** `0298897d`: compile-only stubs (`pasteDecision`, `isPasteChord`, empty notices, a `ReadHomeClipboard` that always says failed) so the target tests load and run. 14 vitest assertions failed on the planned behaviour (the pasteDecision and isPasteChord cases, and the paste flow in DesktopTab) and 5 Go assertions failed (ok text, concealed, empty, too_large, ok with no text, the inclusive limit). Semantic assessment: every failure is an assertion on missing behaviour in code that compiled and ran; none is a load, import or fixture fault. The tests that passed at RED are the regression guards that hold with the stubs (Cmd+V while not controlling, Cmd+Shift+V, the real bridge refusing). The RED evidence classifier (`gsd_run check tdd-red-evidence`) was not run; RED was read from the vitest and `go test` output by hand.
- **GREEN** `6ca45021`.
- Task 1 is a tracer (no RED commit); its tracer test was mutation-checked (disabling the `tab-inactive` call makes it fail).

## Deviations from Plan

**1. [Rule 1 - Bug] Release chord strings named a chord the system takes.** Ctrl+Option+Cmd+Return is taken by the system in the app (21-02), the decision is Ctrl+Option+Cmd+Shift+Return (a4798f16), and 21-08 handed the application to this plan. The plan's `files_modified` lacks `peerDesktop.ts`, `DesktopView.tsx`, `DesktopControlBar.tsx`, `desktopInput.ts` (comment only), `peerDesktop.test.ts` and `TabBar.test.tsx`; without the change the label would tell the user an escape that does not work.
- **Commit:** c292ab30

**2. [Rule 3 - Blocking] Wrapper element and a layout-effect close.** The paste listener needs the tab's own element, so DesktopTab now renders a `display: contents` div around `DesktopView` (no layout effect). Tab-close release uses `useLayoutEffect` cleanup because the viewer handle is detached before passive cleanups.
- **Commit:** d01693f9, 6ca45021

**3. [Rule 2 - Missing functionality] `notify` dwell.** The viewer handle's `notify` fixed 3 s; the UI-SPEC gives the paste notices 5 s, so it takes an optional dwell.
- **Commit:** 6ca45021

Judgement call, not a deviation: the V tap after a paste is the replayed keydown through Phase 20's handler rather than a direct `encodeKey` + `sendInput` (the plan's wording). The messages that go out are the same (key V down then up with the keydown's mask) and the Swap setting is honoured. The tab's own exit repeats the chord's release-all, so two release messages go out on a chord exit; both are idempotent.

## Notes for later plans

- **21-12 (VM rows):** the held count after Cmd+C, the Mac locking while controlling, and the Cmd+V rows (different text, same text, concealed, over 1 MB) are covered here only by unit tests with a fake client; the real-app rows are still to run. Also check the label's line 2 (`Release: Ctrl+Option+Cmd+Shift+Return`, 37 characters at 12 px) against the fixed 232 px chip (`nowrap`, `overflow: visible`), as 21-08 flagged.
- **21-13:** `controlMode.test.ts` and `app_clipboard_paste_test.go` need Section 2 and Section 4 entries (DVWR-04); this plan did not touch TESTING.md.
- **Browser viewer:** left on Ctrl+Option+Cmd+Return. Whether that chord reaches a browser page is unmeasured (21-02 measured only the app's web view); a future change can pass a different `releaseChord` through the same prop.
- DVWR-04 is not marked complete in REQUIREMENTS.md here (the orchestrator owns shared files, and the real-app rows are 21-12's).

## Known Stubs

None. (The RED stubs existed only in commit `0298897d` and were replaced in `6ca45021`.)

## Threat Flags

None. `ReadHomeClipboard` is a new Wails binding but is the one the plan's threat model covers (T-21-32, T-21-33): types-first bridge, no text for concealed, a single call site on a captured Cmd+V, no timer or focus read. No endpoint, auth path or schema was added, and no package was installed.

## Self-Check: PASSED

Created files, `frontend/src/wailsjs/go/main/App.{d.ts,js}` entries and all four task commits (d01693f9, 0298897d, 6ca45021, c292ab30) verified present.
