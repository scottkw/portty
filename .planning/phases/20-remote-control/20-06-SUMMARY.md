---
phase: 20-remote-control
plan: 06
subsystem: frontend
tags: [input, data-channel, pointer, keyboard, ime, cmd-chord, swap, notices, desktop-viewer]
requires:
  - phase: 20
    plan: 01
    provides: "frozen v1 wire contract (input-v1.jsonl, signal-v1.jsonl), hello caps and rights, ordered reliable input channel"
provides:
  - "lib/desktopInput.ts: createDesktopInput over the stage and its hidden text input, v1 encoders checked against the Go golden file, splitText, swapCode, isReleaseChord"
  - "lib/desktopClient.ts: viewer hello caps {video:1,input:1}, ordered input channel before createOffer, rights/input/surface/control/cursor events, sendInput, send, currentEpoch"
  - "DesktopView: .desktop-stage, hidden input, ring, dot, notice slot, controlMode / menuOpen / onControlChange / onReleaseChord props, exported ControlMode"
affects: [20-07, 20-08, 20-11, 21, 22]
tech-stack:
  added: []
  patterns:
    - "controller with injectable deps (send, getRect, getEpoch, raf, setInterval, storage, win, doc); listeners on the stage and its hidden input only, window/document get blur, pagehide, visibilitychange"
    - "forwarded-down map: a keyup follows the code its keydown was sent with"
    - "a picture is the track plus ICE connected, never ontrack alone"
key-files:
  created:
    - frontend/src/lib/desktopInput.ts
    - frontend/src/lib/__tests__/desktopInput.test.ts
  modified:
    - frontend/src/lib/desktopClient.ts
    - frontend/src/lib/__tests__/desktopClient.test.ts
    - frontend/src/components/DesktopView.tsx
    - frontend/src/components/__tests__/DesktopView.test.tsx
    - frontend/src/style.css
key-decisions:
  - "Releases the user did not choose (window blur, hidden tab, pagehide, channel close) and the user's own (View Only, menu, chord) hand the keyboard back; lock, surface switch and reconnect keep the stage focus (UI-SPEC: paused with focus keeps the ring)"
  - "The viewer learns the epoch only from a surface message; until the first one arrives it sends nothing"
  - "picture = track + ICE connected (additional task, see below)"
requirements-completed: [DCTL-01, DCTL-02, DCTL-03]
status: complete
duration: ~1h30
completed: 2026-11-26
estimate:
  tokens: 95000
  raw_tokens: 95000
  tasks: 3
  confidence: low
actuals:
  tokens: 24200
  tasks: 3
  commits: 4
plan_head_before: 6662f19644b8914f539b0952f62f526d6d7e3711
plan_head_after: a9b4e9b8
---

# Phase 20 Plan 06: Browser control - input channel, pointer, keyboard, ring, dot and notices Summary

A click on the picture now leaves the browser as a golden-shaped `btn` on an ordered `input` data channel that exists in the offer, keys and composed text follow the same channel with the Cmd-chord tap rule, swap, release chord, release triggers and heartbeat, and the stage shows who owns the keyboard with a ring, a local dot and one notice chip.

## Reconcile gate (Task 1 precondition)

`git log --oneline d792ca07..HEAD -- frontend/src/components/DesktopView.tsx frontend/src/lib/desktopClient.ts frontend/src/style.css` listed three Phase 19 commits, all already in this worktree's base: `95d6306e` (19-13: Desktop group and setup sheet styles in style.css), `08d2ee85` (viewer card buttons moved out of the live status region) and `01bd357f` (Test Capture thumbnail styles). I read the current files (not the old ones) before editing; the Phase 20 CSS is one block appended after the `.desktop-*` rules and no Phase 19 rule was touched.

## What was built

- **Task 1 (tracer, fc87909f):** viewer hello caps `{video:1, input:1}`; with server caps input 1, `createDataChannel('input', {ordered:true})` before `addTransceiver` and `createOffer` (a FakePC test records the call order); `rights`, `input {open}` and `surface` events; `sendInput(msg, droppable)` (false before open, a droppable move skipped over 64 KiB, never a press, release or wheel); `send`; `currentEpoch()` (null until the first surface message and again after a teardown). `desktopInput.ts` encoders are tested against every valid viewer line of `internal/desktop/testdata/input-v1.jsonl` (the file is read directly; `move-clamped` and `move-extra-field` are skipped as the plan says). A press inside the picture sends `btn {b,d,x,y,e,ts}` with x and y normalised in `videoContentRect`; a press in a letterbox bar sends nothing; a move in a bar is forwarded only with a button held and then clamps to the picture edge; pointer capture keeps a drag going; buttons 3 and 4 are dropped; the context menu is suppressed while control is on; ten moves in one frame send one droppable `move`.
- **Task 2 (c344b5bf):** keys on the hidden textarea only (so the 19-06 listener test, which forbids key listeners on window and document, still passes). `key` messages carry the physical code, down/up, repeat, modifier mask (1 shift, 2 ctrl, 4 alt, 8 meta), Caps Lock and the typed character. Composing (`isComposing`, `keyCode === 229`, `key === 'Dead'`) sends no key; `compositionend` and an unmatched `insertText` send `text` through `splitText` (at most 1024 UTF-8 bytes, a ZWJ, variation selector or combining mark stays with the code point before it, a ZWJ also keeps the code point it joins). Cmd-chord tap rule on the physical Meta, forwarded-down map, Swap Ctrl and Cmd resolved at keydown (mask bits too; stored under `portty:desktop-swap-ctrl-cmd`; a throwing storage reads as off), release chord checked on physical modifiers and never forwarded, wheel conversion (lines to 16 px, pages to stage height, clamp 2400, pinch swallowed), `sendChord` for the Send Keys menu, release on window blur, pagehide, hidden tab, channel close and View Only, and a 1000 ms heartbeat only while something is held.
- **Task 3 (a9b4e9b8):** control state, the 2 px ring (only while the keyboard is captured), the 8 px dot positioned straight on the element on the animation frame of the move, one notice slot below the state chip (3 s, 5 s with the hint, fade 150 ms, role status or alert), the first-capture hint (`portty:desktop-release-hint-seen`), "Typing stays on this device", "Held keys were released" (lock, epoch change, reconnect, channel close, only if something was held; a user's own blur says nothing), the two window state chips and the Accessibility chip, and a phone stays the Phase 19 page (no tabindex, no role, no dot, no ring, no controller). `desktopClient` parses `control` and `cursor` from the input channel (known enum values and finite numbers only; the golden server lines parse into the typed events).

## Verification

- `pnpm --dir frontend exec tsc --noEmit -p tsconfig.json`: clean.
- `pnpm --dir frontend test`: 127 files, 2051 tests pass (includes the no-literal-colour source gate and every 19-06 case).
- `bash tests/check-brand.sh`: clean. Acceptance greps: `createDataChannel('input'` 1, `input-v1.jsonl` in the test, `export function createDesktopInput` 1, no window/document key or pointer listener in desktopInput.ts, `portty:desktop-swap-ctrl-cmd` 1, `keyCode === 229` 1, `guardPasteInsert` used, `desktop-ring` in style.css, `Held keys were released` 1, no Wails import in DesktopView.
- Tracer gate: not auto mode and `<verify>` is automated-only, so it was re-run end to end after Task 1 and passed before expanding.
- Host safety: only vitest in jsdom ran; no viewer, capture or injection was opened on the host.

## Additional task: Phase 19 review follow-up, `ontrack` counted as "picture"

**Finding: real.** In `desktopClient.ts` the old `onTrack` set `hasPicture = true`, reset the backoff (`attempt = 0`), cancelled the 30 s unreachable timer and set status `streaming`. `ontrack` fires when the answer is applied (`setRemoteDescription`), long before, and without, any working media path. So when ICE then failed: the unreachable clock was cancelled on every cycle, the backoff restarted at 1 s every cycle, and the page showed a stale "streaming" state between failures. The viewer never reached "Can't reach this Mac", never backed off past 1 s and redialled about every ICE-failure interval (about 30 s in a browser), and each redial is a new offer that can restart capture on the Mac. That matches the reviewer's description.

**Reproduction (RED).** Four tests against the old code: a track alone produced a `streaming` status; track-first streaming did not wait for ICE; repeated ontrack-then-ICE-failure redialled at 1 s each time ("no redial before 2000 ms: expected 3 to be 2"); and ICE that never connected never produced `unreachable` (statuses `streaming, reconnecting, ...` with no `unreachable`).

**Fix (6ae4ca65, `desktopClient.ts` plus its test only).** "Picture" now means both the track arrived and the peer connection reached `connected`, in either order. `onTrack` only records the track and emits `track` (the page still needs the stream to set `srcObject` and start playback); `maybePicture()` sets `hasPicture`, resets the backoff, cancels the unreachable timer and sets `streaming` once both flags are set; both flags reset on teardown. A failure with no picture keeps the existing 1, 2, 4, 8, 10 s backoff and the 30 s unreachable clock. The 19-06 tests that used `fireTrack()` for a streaming client now use a `fireStreaming()` helper (track plus `connected`), which is the intended meaning.

**Outcome.** The four new tests pass, all 40 earlier client tests pass. Not verified on a real browser (no live peer connection was opened, by rule): a VM row should confirm that Chrome and Safari report `connectionState === 'connected'` before the first frame plays, otherwise the chip and the Streaming announcement would appear a moment after the first frame (the Connecting card is driven by the video `playing` event, not by this status, so the picture itself is unaffected).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing critical] Keyboard handed back on every release that is not a pause**
- **Found during:** Task 3
- **Issue:** UI-SPEC says a window blur, hidden tab, pagehide or closed channel leaves the page in "Control, keyboard off" and one click restores it. Releasing without taking focus off the hidden input would let Chrome restore focus when the window returns and capture the keyboard again silently.
- **Fix:** `releaseAll` drops focus from the hidden input for every reason except lock, surface switch and reconnect (those keep the ring and the focus, as the Control paused row requires). The Task 2 controller tests still pass unchanged.
- **Files modified:** frontend/src/lib/desktopInput.ts
- **Commit:** a9b4e9b8

**2. [Rule 3 - Blocking] `DesktopInput.setPaused` is not in the plan's interface list**
- **Found during:** Task 3
- **Issue:** The "Control paused" state (locked, reconnecting, Control needs Accessibility) must stop the viewer sending while the stage keeps focus; the listed methods (`setControl`, `releaseAll`, ...) cannot express that.
- **Fix:** added `setPaused(on)`. Keys, text, presses, moves and wheel are gated; releases and heartbeats are not.
- **Files modified:** frontend/src/lib/desktopInput.ts
- **Commit:** c344b5bf, a9b4e9b8

**3. [Rule 2 - Missing critical] Safari with storage blocked throws on `window.localStorage`**
- **Found during:** Task 3
- **Issue:** reading the property itself can throw, which would have broken controller creation.
- **Fix:** `safeStorage()` in DesktopView; the controller already treats a throwing storage as off.
- **Commit:** a9b4e9b8

### Other adjustments (not bugs)

- Commit trailer names `Claude Sonnet 5.5` (the model that wrote the code), not the plan's `Claude Opus 5.5`, per the dispatch instruction.
- TDD: the Task 2 and 3 tests were written before the implementations they cover, but the RED run was not committed separately (Task 2's tests and implementation went into one commit); Task 1's additional-task fix has a committed RED-then-GREEN pair in one commit.
- Props for 20-07: `controlMode` (default `'control'`), `menuOpen`, `onControlChange({available, captured})`, `onReleaseChord`. The plan said only "onControlChange and an exported way for the bar to set the mode and open menus"; 20-07 can reshape these.
- The Phase 19 `desktop-chip` position is `top: calc(8px + ...)` and the notice chip sits at `calc(48px + ...)`. 20-07's reserved 44 px bar row will need to move both (UI-SPEC Surface C gives `52px` for the state chip on bar-capable viewers).
- `gsd_run windows append` was not run (the helper is not available in this worktree shell); the stub list is empty so there is nothing to record.

## Risks and notes for the orchestrator

- **Epoch dependency:** the viewer sends nothing until a `surface` message arrives (20-03 sends it on join, after `size`). Until 20-03 merges, a real viewer is effectively view-only. The Go side starts at epoch 1.
- **Wheel sign:** the viewer sends the browser's own sign (`deltaY` positive when the page would scroll down); 20-02 documents "browser sign" and inverts for CoreGraphics. No inversion here.
- **Interaction row 25** says control returns as "keyboard off" after a reconnect, while the Control paused row keeps the ring and focus. I followed the paused row (focus kept on lock, surface and reconnect); a one-line change in `KEEP_FOCUS` flips it.
- Backstop truths (unverified by design): Retina and external non-Retina pointer landing at any window size, and input-method, emoji picker, dictation and dead keys in Chrome and Safari arriving exactly once. The emoji picker and dictation path relies on `beforeinput` with `inputType: 'insertText'`; other input types are ignored, which a real-browser row should confirm.

## TESTING.md rows for plan 20-11 (not edited here, parallel plans)

- Section 2, frontend group: `frontend/src/lib/__tests__/desktopInput.test.ts` (new file, 54 cases). Extended files: `desktopClient.test.ts` (49 cases), `DesktopView.test.tsx` (51 cases).
- Section 4 traceability, repo-relative paths only: DCTL-01 `frontend/src/lib/__tests__/desktopInput.test.ts`; DCTL-02 `frontend/src/lib/__tests__/desktopInput.test.ts` and `frontend/src/components/__tests__/DesktopView.test.tsx`; DCTL-03 `frontend/src/lib/__tests__/desktopInput.test.ts` (swap, Send Keys chord taps).
- Section 5 manual items (new M-NN): (a) pointer lands on the right spot on the Retina display and on an external non-Retina display at a narrow and a wide viewer window; (b) Japanese or Chinese input, emoji picker, dictation and Option+E dead keys typed in Chrome and Safari arrive on the Mac once; (c) a Cmd chord (Cmd+C then letters) leaves nothing held on the Mac in Chrome, Safari and Firefox; (d) after an ICE failure the viewer shows "Can't reach this Mac" within about 30 s and backs off (VM row for the additional-task fix).

## Threat Flags

None. The only new trust boundary is server messages on the input channel, which are covered by T-20-33 (known enum values and finite numbers only; text rendered as React text).

## Self-Check: PASSED

Created files exist (`desktopInput.ts`, `desktopInput.test.ts`); commits 6ae4ca65, fc87909f, c344b5bf and a9b4e9b8 are on the branch; the full frontend suite and `tests/check-brand.sh` pass.
