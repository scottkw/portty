---
status: awaiting_human_verify
trigger: "M-197 real-iPhone UAT (iPhone 16 Pro, Chrome for iOS; Safari not tried): after Show Keyboard the key row (Esc, Tab, Ctrl, Opt, Cmd, arrows, Send Keys) often never replaced the bottom bar; when it did show, tapping Cmd dismissed the iPhone keyboard, so Cmd+A could never be sent. Typing, Backspace and Return worked. Chrome's autofill accessory bar (key/card/location) sat above the keyboard."
created: 2026-11-27T00:00:00Z
updated: 2026-11-27T00:00:00Z
---

## Current Focus

hypothesis: two causes. (1) The key row was keyed on a visual-viewport reading (innerHeight - visualViewport.height > 100), which is not a reliable "keyboard up" signal in Chrome for iOS (its accessory bar, the keyboard's animation, the layout viewport). (2) A key-row tap relied only on pointerdown preventDefault to stop iOS moving focus off the hidden field; iOS's own tap path (synthetic mouse events after touchend) can still blur it, and the stage already uses the stronger signal, a cancelled touchstart.
bug_class: platform-behaviour (iOS WebKit in a WKWebView host); not reproducible on this host (no iPhone, no Portty run allowed)

reasoning_checkpoint:
  hypothesis: "The row shows only when visualViewport says the keyboard is up, and any viewport change (accessory bar, animation) flips it; a viewport 'down' also ran keyboardDown(), disarming an armed Cmd. The key row's buttons cancel pointerdown but not touchstart or mousedown, so on iOS a tap can still reach the focus change that blurs the hidden field."
  confirming_evidence:
    - "DesktopPhoneLayer: keyboardUp came only from useVisualViewportInset; the slot showed the key row only while it was true (DesktopPhoneBars: keyboardUp ? keyBar : bottomBar)"
    - "A wasUp effect called keyboardDown() (disarm + releaseAll) on every viewport falling edge, on top of the field's blur listener: a keyboard-down sent two releases (the old 'keyboard going down sends release' test got 2 once raiseKeyboard/lowerKeyboard also moved focus, as on a phone)"
    - "touchTrackpad cancels the stage's touchstart (non-passive), so tapping the picture never blurred the field; the key row only cancelled pointerdown"
    - "Phase 14 research and S-01 cover only Safari with the terminal KeyBar; nothing ever tested Chrome for iOS or the desktop key row on a phone"
  falsification_test: "If on the real iPhone (fix build) the key row still fails to show with the keyboard up, focus is not reaching the field (check document.activeElement); if Cmd still dismisses the keyboard with touchstart and mousedown cancelled, something else blurs the field (look for a re-mount or a focus() elsewhere)"
  fix_rationale: "Focus is the hidden field's own state and is exact: the soft keyboard is up on an iPhone exactly while a text field has focus (a hardware keyboard gets the key row too, which is useful). The viewport hook still sizes the root. A cancelled touchstart is the iOS-documented way to keep a tap from becoming mouse events and a focus change; mousedown is cancelled too as a belt."
  blind_spots: "Not run on a device. Chrome for iOS's autofill accessory bar is not removed (autocomplete=off is already set and Chrome ignores it for this; switching the field to a textarea may drop it, untested). Cancelling touchstart on the key row stops a horizontal scroll of the row from starting on a key (ten keys fit 360 pt; iPhone portrait is 393 pt+)."
  candidate_causes:
    - "keyboardUp from visualViewport unreliable in Chrome for iOS - FIXED (focus-based)"
    - "viewport falling edge disarming an armed modifier - FIXED (only the blur disarms)"
    - "key-row tap not cancelling iOS's synthetic mouse/focus path - FIXED (touchstart + mousedown cancelled)"
    - "hidden field re-mounted on arming - ruled out: canControl is the only condition, effects keep stable deps; test pins the same node"

## Resolution

Fix (commit in the M-197 fix):
- frontend/src/components/DesktopPhoneLayer.tsx: keyboardUp is the hidden field's focus (focus sets it, blur clears it and runs keyboardDown); the viewport falling-edge effect is gone; useVisualViewportInset only sizes the root.
- frontend/src/components/DesktopPhoneBars.tsx: the key row cancels touchstart (native, non-passive) while shown; every key cancels mousedown.

Tests (frontend/src/components/__tests__/DesktopPhoneLayer.test.tsx, written failing first, 3 failed before the fix):
- M-197: the key row shows whenever the hidden field has focus, even while the visual viewport reports no keyboard
- M-197: a viewport change while the field keeps focus neither swaps the row nor disarms a modifier
- M-197: arming a modifier keeps focus on the same hidden field: touchstart and mousedown are cancelled, no blur
- M-197: Cmd armed from the key row then a typed a sends Cmd+A
- raiseKeyboard/lowerKeyboard now focus/blur the field as a phone does.
tsc clean; full vitest 154 files / 2809 tests pass.

## Re-check on the real iPhone (Safari AND Chrome for iOS)

1. Tap Show Keyboard 5 times (dismiss with Done/tap elsewhere between): the key row replaces the bottom bar every time, as soon as the keyboard opens.
2. With the keyboard up, tap Cmd: the keyboard stays up and Cmd shows armed. Then type a: Cmd+A selects all on the Mac and Cmd disarms. Repeat with Ctrl and Opt, and Esc/Tab/an arrow (keyboard stays up for each).
3. Tap Send Keys in the key row: the menu opens and an item works; the keyboard state afterwards is noted.
4. Dismiss the keyboard (Done in the accessory bar, or tap the picture if that blurs): the bottom bar comes back and any armed modifier is off.
5. Note whether Chrome's autofill bar (key/card/location) still shows above the keyboard and its height; if it does and it matters, the follow-up is trying a textarea for the hidden field.
6. Rotate to landscape with the keyboard up: the row stays and all ten keys are reachable (the row no longer scrolls from a key).
