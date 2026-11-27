---
status: awaiting_human_verify
trigger: "Real-iPhone UAT (iPhone 16 Pro, Safari, build cabf716f, which includes ddcc2b48): with the iPhone keyboard up, tapping Esc in the key row closed the keyboard, but the key row stayed and the bottom bar (with Show Keyboard) never came back, so the keyboard could not be reopened; a reload recovered."
created: 2026-11-27T00:00:00Z
updated: 2026-11-27T00:00:00Z
---

## Current Focus

hypothesis: ddcc2b48 (M-197) made "keyboard up" the hidden field's focus, and only the field's blur brings the bottom bar back. iOS hid the keyboard without blurring the field, so nothing ever ran keyboardDown().
bug_class: regression (ddcc2b48) on a platform behaviour (iOS can hide the soft keyboard while the input keeps focus)

reasoning_checkpoint:
  hypothesis: "keyboardUp stays true while document.activeElement is the field; iOS dismissed the keyboard without a blur event; no other signal resets the slot."
  confirming_evidence:
    - "DesktopPhoneLayer: setKeyboardUp(true) on focus, keyboardDown() only from the field's blur listener (and the effect cleanup)."
    - "The user saw the key row with no keyboard: keyboardUp was still true, so no blur reached the field (a blur would have shown the bottom bar)."
  why_esc_closed_the_keyboard: "Not found in the page. Tapping Esc runs pointerdown preventDefault -> onKey('Escape') -> sendKey -> keyTap (two key messages on the input channel) -> disarm; the row cancels touchstart and mousedown, nothing calls blur() or focus(), no page listener reacts to Escape (no real keydown is generated) and the hidden field is not re-mounted (canControl unchanged). So iOS hid the keyboard by itself. Open candidates for the device: Esc is the leftmost key, inside Safari's left-edge (back-swipe) touch zone; or the tap fell on iOS's own accessory/keyboard chrome while --vv-height lagged. The re-check below separates these."
  falsification_test: "On the fix build, if the keyboard closes and the key row still stays more than half a second, the visual viewport did not grow back (check whether Safari reported a resize) or the field lost focus without a blur event."
  fix_rationale: "A keyboard that is really gone shows in the visual viewport: it grows back to its tallest height at this width. When the field still has focus, the viewport had shown a keyboard during this focus, and it stays back for 400 ms, the field is blurred, and the existing blur path (keyboardDown: bottom bar back, modifiers disarmed, everything released) runs. The tallest height, not innerHeight, is the baseline because Chrome for iOS can shrink the layout viewport with the keyboard; a focus that never showed a keyboard (Chrome's misses that M-197 fixed, a hardware keyboard) is never blurred, so ddcc2b48's behaviour stands."
  blind_spots: "Not run on a device. Rotating with the keyboard up and then losing it without a blur is not covered until the new orientation's full height has been seen once. If Chrome's viewport never shrinks for its keyboard, Chrome keeps only the blur path (unchanged from ddcc2b48)."

## Resolution

root_cause: "ddcc2b48 keyed the key row on the field's focus alone; iOS can hide the keyboard without a blur, which left the row up and Show Keyboard unreachable."
fix: "frontend/src/components/DesktopPhoneLayer.tsx: a visualViewport resize watcher; once the viewport has shown a keyboard (more than 100 px under its tallest at this width) during the current focus and then stays back for 400 ms with the field still focused, it blurs the field (blur -> keyboardDown)."
verification: "Failing first: 'keyboard hidden without a blur: once the viewport stays back at full height for 400 ms the field blurs and the bottom bar returns'. Guards (pass before and after): a return shorter than 400 ms keeps the row; a focus whose viewport never showed a keyboard is never blurred. All ddcc2b48 M-197 tests still pass. tsc clean, full vitest pass."
files_changed:
  - frontend/src/components/DesktopPhoneLayer.tsx
  - frontend/src/components/__tests__/DesktopPhoneLayer.test.tsx

## Re-check on the real iPhone (Safari, then Chrome for iOS)

Build to test: v23.8.2 from 6ea4e35e (p22-rig build22, tags "wailsassets vmdesktop", Developer ID), deployed to portty-test 2026-11-27 with a fresh always-available link; TextEdit frontmost with ~/p21/scroll-test.txt.

1. Show Keyboard, then tap Esc in the key row. Note whether the keyboard closes. If it does: within about half a second the bottom bar with Show Keyboard comes back; tap it and the keyboard and key row return.
2. Repeat with Tab (second key) and the Right arrow (far right). If only Esc closes the keyboard, note it (left screen edge); if all do, note it.
3. Arm Cmd, then let the keyboard close by any means: the bottom bar returns and Cmd is no longer armed when the keyboard is reopened.
4. Regression check (M-197): Show Keyboard five times, the key row appears every time; Cmd then a selects all; the keyboard does not close by itself while open (watch it for 5 s, also in Chrome for iOS with its autofill bar).
