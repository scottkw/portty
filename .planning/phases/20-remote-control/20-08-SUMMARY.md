---
phase: 20-remote-control
plan: 08
subsystem: frontend
tags: [clipboard, safari-gesture, popover, state-chips, desktop-viewer]
requires:
  - phase: 20
    plan: 07
    provides: "DesktopControlBar, barAvailability clipboard row, ContextMenu with caption rows, bar menus portaled into the full-screen element"
  - phase: 20
    plan: 06
    provides: "notice slot, window and Accessibility chips, releaseAll, picture = track + ICE connected"
  - phase: 20
    plan: 01
    provides: "clip_get, clip_put, clip_data, clip_result wire shapes (signal-v1.jsonl)"
provides:
  - "desktopClient.copyFromMac / pasteToMac with xid, seq, total, len checks, 20 s timeout, failed on disconnect; chunkClipText; CLIP_* constants; ClipOutcome"
  - "DesktopControlBar: Clipboard button and menu (Copy from Mac, Paste to Mac, caption); props onCopy, onPaste, clipBusy, clipboardRef, popoverOpen; BarState.canClipboard"
  - "DesktopClipboardPopover: DesktopCopyPopover and DesktopPastePopover"
  - "DesktopView: Safari-safe copy, paste flow, one notice per outcome, CLIP_SLOW_MS, popover lifecycle, state chip stacking"
affects: [20-11, 21, 22]
tech-stack:
  added: []
  patterns:
    - "the browser clipboard write starts inside the click with a ClipboardItem whose text/plain promise resolves when the Mac answers (rejects on any outcome but ok)"
    - "popovers belong to the bar: they portal into document.fullscreenElement ?? body, close with the bar, and are closed by any bar menu"
key-files:
  created:
    - frontend/src/components/DesktopClipboardPopover.tsx
    - frontend/src/components/__tests__/DesktopClipboardPopover.test.tsx
  modified:
    - frontend/src/lib/desktopClient.ts
    - frontend/src/lib/__tests__/desktopClient.test.ts
    - frontend/src/components/DesktopControlBar.tsx
    - frontend/src/components/__tests__/DesktopControlBar.test.tsx
    - frontend/src/components/DesktopView.tsx
    - frontend/src/components/__tests__/DesktopView.test.tsx
    - frontend/src/style.css
key-decisions:
  - "Clipboard messages travel on the signalling WebSocket (transport.send), as in signal-v1.jsonl, not on the input data channel"
  - "One transfer at a time in the client: a second call resolves busy at once and sends nothing (the daemon would answer busy too)"
  - "A copy whose text arrived but whose write was refused opens the Copy popover only if the bar is up and no bar menu is open; otherwise it falls back to the failure notice rather than stacking two things"
  - "Clipboard sits in the same bar group as Send Keys (UI-SPEC layout: group 2), with no divider between them"
requirements-completed: [DCTL-06, DCTL-05]
status: complete
duration: ~1h30
completed: 2026-11-26
estimate:
  tokens: 85000
  raw_tokens: 85000
  tasks: 3
  confidence: low
actuals:
  tokens: 26000
  tasks: 3
  commits: 5
plan_head_before: 36b965ceaf33f4b5c89dbed36d82f70fde850f4d
plan_head_after: db356d0b64302267241fae393586680dfbac73af
---

# Phase 20 Plan 08: Clipboard menu, Copy and Paste popovers and state chips Summary

The viewer's Clipboard menu now moves text between the two Macs only by an explicit click: Copy from Mac starts the browser write inside the click with a promise for the Mac's text (Safari's gesture rule), Paste to Mac reads the viewer's clipboard in the click and sends it in 16 KiB `clip_put` chunks, every outcome is one notice in words, a refused write or read opens a small popover, and the window-share and Accessibility problems are persistent state chips with a stated priority.

## Reconcile gate (Task 1 precondition)

`git log --oneline d792ca07..HEAD -- frontend/src/style.css frontend/src/components/DesktopView.tsx` lists the Phase 19 commits `95d6306e` (19-13 Desktop group styles), `08d2ee85` (viewer card buttons moved out of the live region) and `01bd357f` (Test Capture thumbnail), all already in this base, and the 20-06 and 20-07 commits (`fc87909f`, `c344b5bf`, `a9b4e9b8`, `60e1e93c`, `26f58bf2`, `e31f3825`). I read the current DesktopView, DesktopControlBar and style.css before editing. The new CSS is appended after the 20-07 block; the only earlier rules touched are the two chip rules this plan owns (`.desktop-chip` gains `max-width`; the notice `top` offsets use the chip-height variable). No unmerged Phase 19 plan blocks this plan.

## What was built

- **Task 1 (tracer, dd7a6347):** `copyFromMac` sends `clip_get` with a fresh `xid`, collects `clip_data` for that xid and resolves on `clip_result`. `ok` only when seq ran 0..total-1 with the same total and len on every chunk, no chunk over 16384 bytes, and the assembled UTF-8 length equals len; a gap, a duplicate, a changed total or len, an oversized chunk, a non-string text, `CLIP_TIMEOUT_MS` (20 s) or a dropped connection is `failed`. `pasteToMac` measures UTF-8 with `TextEncoder` (0 is `empty`, over 1048576 is `too_large`, neither sends), cuts with `chunkClipText` (code-point cuts, at most 16384 UTF-8 bytes and at most 30000 encoded-frame bytes by counting JSON escapes per code point), sends all chunks under one xid, and resolves on that xid's `clip_result`. The Clipboard button and menu sit after Send Keys in the same group; both items grey while a transfer is in flight, the button follows the availability table and is hidden without `rights.clipboard`. DesktopView's `copyFromMac` runs inside the click: `navigator.clipboard.write([new ClipboardItem({'text/plain': promise})])` starts synchronously (the blob promise rejects on any outcome but ok), with a `writeText` fallback after the answer where `ClipboardItem` is missing; `Copying from the Mac…` appears after `CLIP_SLOW_MS` = 400 and stays until the result replaces it; each outcome maps to exactly one notice. The tracer gate (not auto mode, automated-only verify) was re-run end to end and passed before expanding.
- **Task 2 (RED 653a2a44, GREEN 9d0bff43):** `DesktopCopyPopover` (read-only textarea, text selected and focused, Done) and `DesktopPastePopover` (focused empty textarea with the placeholder, Discard Text, Send to Mac with `aria-disabled` while empty or over 1 MB, inline `Too long to send (limit 1 MB).` as role alert). Both are non-modal `role="dialog"`, 4 px under the Clipboard button, portaled into `document.fullscreenElement ?? body`, closed by Esc or a press outside, Tab wrapping inside, focus returned to the Clipboard button. Paste to Mac reads in the click and sends at once (`Sent to the Mac's clipboard; press ⌘V`, 5 s); a rejected or missing `readText` opens the Paste popover; the empty and too-large notices send nothing. At most one popover or menu is open and opening one releases everything held.
- **Task 3 (RED 60b11eed, GREEN db356d0b):** the window-raise, minimized and Accessibility chips and their priority were already in DesktopView from 20-06; this task pinned them with tests (priority pairs from the plan, notice below chip and both at once, role status and no focus, phone rules) and added what was missing: `.desktop-chip` wraps at `min(480px, calc(100% - 48px))` for state chips too (20-06 only had it on notice chips), and the notice chip now sits below a state chip that wraps (see deviation 3).

## Verification

- `pnpm --dir frontend exec tsc --noEmit -p tsconfig.json`: clean.
- `pnpm --dir frontend test`: 133 files, 2350 tests pass (includes the token source gate). `pnpm --dir frontend build`: succeeds (chunk-size warning is pre-existing). `bash tests/check-brand.sh`: clean.
- Acceptance greps: `new ClipboardItem` 1; `function chunkClipText` 1; caption text 1 in DesktopControlBar; `Discard Text` 1; `Sent to the Mac's clipboard; press ⌘V` 1; the three chip strings 1 each; console-call-with-text grep 0; `dangerouslySetInnerHTML` 0.
- Host safety: only vitest in jsdom with a mocked `navigator.clipboard` ran. No viewer, capture, injection, real clipboard access, tailscaled or 100.x address.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing critical] The bar never showed a control the viewer cannot use**
- **Found during:** Task 1. UI-SPEC says capabilities the hello did not grant hide the item, and 20-07 left the `clipboard` row without right-based hiding for this plan.
- **Fix:** `BarState.canClipboard` (required; the view passes `rights.clipboard`), `barAvailability().clipboard.hidden = !canClipboard`. The existing test helper gained the field.
- **Commit:** dd7a6347

**2. [Rule 2 - Missing critical] The bar could hide under an open popover**
- **Found during:** Task 2. UI-SPEC: "a menu or popover is open" keeps the bar revealed; the 20-07 reveal only knew its own menus, so the button a popover hangs from would fade while the popover stayed.
- **Fix:** optional `popoverOpen` prop on the bar, folded into its open-menu reveal rule; tested.
- **Commit:** 9d0bff43

**3. [Rule 1 - Bug] A wrapped state chip overlapped the notice chip**
- **Found during:** Task 3. The state chip and notice chip are both absolutely positioned (20-06) with the notice at a fixed 32 px chip height; the long window chip (70+ characters) wraps at 480 px to two lines, so any notice (a copy result) landed on top of it.
- **Fix:** DesktopView measures the state chip's height after each render and on resize and sets `--desktop-chip-h` on `.desktop-status` only when it exceeds one line; both notice `top` rules add that height plus the 8 px gap, defaulting to 32 px (the old offsets). Tested by stubbing `offsetHeight`. Not verified visually in a real browser.
- **Commit:** db356d0b

**4. [Rule 3 - Blocking] The DesktopView test mocked the whole client module**
- The mock returned only `createDesktopClient`, so the real `CLIP_MAX_BYTES` and `clipByteLength` the view now imports would be undefined. The factory now spreads `importOriginal()`, and the mock client gained `copyFromMac` and `pasteToMac`. **Commit:** dd7a6347

**5. [Rule 3 - Blocking] A hook ordering change in DesktopView**
- The `other_account` early return sat above the chip computation; the popover clean-up effect needs `showBar`, which comes from it. The early return moved to just after `showBar` is computed (no hook follows it). Behaviour unchanged; the other_account test passes. **Commit:** 9d0bff43

**6. [Rule 3 - Blocking] No `node_modules` in the worktree**
- `pnpm --dir frontend install --frozen-lockfile --offline` (lockfile only, no new package; `node_modules` is gitignored).

### Other adjustments (not bugs)

- Commit trailer names `Claude Sonnet 5.5` (the model that wrote it), not the plan's Opus.
- Clipboard shares the Send Keys bar group with no divider (UI-SPEC layout group 2). Plan text only said "after Send Keys".
- Failure and too-large clipboard notices dwell 5 s (the table says "3 s or 5 s where marked"): they give a next step, and 20-07's switch-failure notice already uses 5 s.
- `too_large` on a copy is a status notice with the warning icon; only the generic failure is role alert (as in the plan's truths).
- The window and Accessibility chips stay gated on `canControl`, as 20-06 built them. UI-SPEC Surface F says a phone shows "every state chip", but both chips tell the user to click the picture or to act in the Mac's Privacy settings with a control surface; a phone has no control surface. Locked, Reconnecting, Streaming and every `Showing …` notice do render on a phone (tested). If the orchestrator wants the Accessibility chip on phones too, it is the `canControl &&` in one line of DesktopView.
- Task 1's tests and implementation were committed together (tracer); Tasks 2 and 3 have separate RED and GREEN commits. Task 3's RED run showed only 3 failing tests because 20-06 already built the chips themselves.
- `gsd_run windows append` was not run (helper unavailable in this worktree shell); no stubs to record.

## Known Stubs

None. The `placeholder="Paste text here"` is the specified textarea placeholder, not a stub.

## Risks and notes for the orchestrator

- **Backstop (unverified by design):** Safari's rule that a clipboard write must start in the click is implemented as research Pattern 8 describes, but only a real Safari in the VM can show the write succeeding after a 1 to 2 s wait, and Safari's Paste prompt for `readText`. A VM row is listed below.
- **Clipboard text lives only in memory:** the Copy popover keeps it in React state while open; closing clears it (tested: it is gone from the page). `chunkClipText` and `clipByteLength` exist in desktopClient.ts, so Phase 21 can reuse them.
- **20-05 interplay:** the client follows `signal-v1.jsonl` exactly (xid as a string, `clip_data` and `clip_result` shapes, outcomes). It enforces the daemon's own limits on what it receives (chunk at most 16384 bytes, total at most 1024 chunks, len at most 1 MiB); if 20-05 ever sends a bigger chunk the copy will say `Couldn't copy from the Mac. Try again.`.
- **Phase 21 merge:** DesktopView edits are additive (new state and refs, clipboard handlers before the chip computation, the popovers and one `style` prop on `.desktop-status`); the one structural move is deviation 5. desktopClient.ts additions are new exports, new `clip_data` and `clip_result` cases and two methods on the client.

## TESTING.md rows for plan 20-11 (not edited here)

- Section 2, frontend group: new `frontend/src/components/__tests__/DesktopClipboardPopover.test.tsx` (21 cases). Extended: `frontend/src/lib/__tests__/desktopClient.test.ts` (clipboard transfer describe), `frontend/src/components/__tests__/DesktopControlBar.test.tsx` (Clipboard menu describe plus availability and reveal cases), `frontend/src/components/__tests__/DesktopView.test.tsx` (Copy, Paste and popovers, and state chips describes).
- Section 4 traceability (path column file only): DCTL-06 `frontend/src/lib/__tests__/desktopClient.test.ts`; DCTL-06 `frontend/src/components/__tests__/DesktopClipboardPopover.test.tsx`; DCTL-06 `frontend/src/components/__tests__/DesktopControlBar.test.tsx`; DCTL-06 `frontend/src/components/__tests__/DesktopView.test.tsx`; DCTL-05 `frontend/src/components/__tests__/DesktopView.test.tsx`.
- Section 5 manual (new M-NN, VM browser): (a) Safari: Copy from Mac with a 1 to 2 s Mac answer writes to the clipboard from the click (or opens the Copy popover when refused), and Paste to Mac works through Safari's Paste prompt or the Paste popover; (b) Chrome: first Paste to Mac asks for clipboard permission, a refusal opens the Paste popover; (c) Copy from Mac with a password-manager item on the Mac shows the hidden-item notice and nothing reaches the viewer's clipboard; (d) a 1 MB text round trips both ways and a 1 MB + 1 byte text is refused with the notice; (e) the Clipboard menu, both popovers and the stacked chips read correctly in light and dark appearance and in full screen (popovers visible inside the full-screen element); (f) the window-raise chip, the minimized chip and the Accessibility chip show on a real window share and clear when the condition does.

## Threat Flags

None. The new trust boundaries are the ones in the plan's register: clipboard read and write only inside the user's click on the menu item or popover button (T-20-39), text only in memory and never in a log call (T-20-40, grep criterion and a console-spy test), chunk assembly checks xid, seq, total and len with a 1 MiB cap before sending (T-20-41), clipboard text only ever set as a textarea value (T-20-42, markup test). No package added (T-20-SC).

## Self-Check: PASSED

Created files exist (DesktopClipboardPopover.tsx and its test); commits dd7a6347, 653a2a44, 9d0bff43, 60b11eed and db356d0b are on the branch; the full frontend suite, tsc, the production build and `tests/check-brand.sh` pass. No tracked file was deleted.
