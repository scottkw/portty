---
status: testing
phase: 20-remote-control
source: [20-VERIFICATION.md]
started: 2026-11-27T04:26:47Z
updated: 2026-11-27T04:26:47Z
---

## Current Test

number: 1
name: M-180: pointer accuracy on the real Retina display and on an external display, at several viewer window sizes, plus scrolling with a real trackpad (natural scrolling on and off)
expected: |
  A click and a drag land on the pixel under the viewer's pointer on both displays; scroll direction matches the viewer's gesture
awaiting: user response

## Tests

### 1. M-180: pointer accuracy on the real Retina display and on an external display, at several viewer window sizes, plus scrolling with a real trackpad (natural scrolling on and off)
expected: A click and a drag land on the pixel under the viewer's pointer on both displays; scroll direction matches the viewer's gesture
result: [pending]

### 2. M-181: input methods, emoji picker, dictation and dead keys from a non-US viewer layout
expected: Composed text arrives once and as typed, with no doubled or dropped characters
result: [pending]

### 3. M-182 and M-183: nothing stays held after a Cmd chord, a closed tab, a sleeping home Mac or a dropped network; Send Keys and the Ctrl/Cmd swap from a non-Mac keyboard. Repeat this after the post-VM fix H-02 (the Cmd-releases-non-modifiers rule in desktopInput.ts)
expected: A key or modifier pressed before Cmd and released while Cmd is down is not left held; Cmd+Tab, Cmd+Space and Lock Screen work from the menu; the swap maps Ctrl to Cmd
result: [pending]

### 4. M-184, M-185, M-186, M-187: window share on the office Mac (raise, confinement, banner, a hung app); what a sheet, an Open/Save panel and a pop-up menu look like in the decoded window frame; clicking a pop-up menu of the shared app (post-fix e9dfa052, layer 101 and up only) while another app's window still cannot be clicked; the AX focus read staying valid (VM row X3 saw -25204 for about 10 minutes)
expected: Pop-up menu item click lands; a sheet, Fonts panel or other app's window drops as covered; the gate does not fail closed for long periods on the real Macs
result: [pending]

### 5. M-188: display switching with an external display, an unplugged display, a closed shared window, and the in-band resolution change on iPhone Safari and in Portty's own window. Re-run after the post-VM fixes M-01 (failed switch resync and fallback restart), M-03 (resize deferred while a button is held) and M-04 (off-display moves are not effect misses)
expected: The viewer never reconnects; the picture and the pointer mapping always agree after a switch, including a switch that fails; a remote drag-resize is not cut by the resize; moves onto off-screen parts of a window do not trip Control needs Accessibility
result: [pending]

### 6. M-189 and M-190: clipboard both ways in Chrome and Safari, with a real click on Copy from Mac and Safari's own Paste prompt; a password-manager item that never copies; a 1 MB text; Paste from Other Apps on the corporate Mac
expected: Copy and Paste work by an explicit action only; a concealed item answers the hidden-item notice and the viewer's clipboard keeps its old content; a 1 MB text is refused cleanly; the Mac shows no paste alert, or a recorded one
result: [pending]

### 7. M-191 and M-192: Chrome Keyboard Lock and the full-screen notice in Safari and Firefox; the control bar, menus, popovers and chips in light and dark; typing into a password field and Caps Lock reaching the office Mac. Also the hidden control bar (post-fix UI-1, 299d842f): a press inside the bar's box while it is hidden reveals the bar and posts nothing to the Mac
expected: Keyboard Lock behaves as documented; the bar's reveal zone matches its real box; a password field fills (a field that stays empty means Secure Event Input is dropping posted keys); Caps Lock state arrives
result: [pending]

### 8. M-193: a viewer that cannot get a picture says so and never says Streaming ahead of one (VM row X2 inferred connected-before-first-frame but could not read connectionState)
expected: The chip never reads Streaming before the first decoded frame, in Safari, Chrome, the Wails web view and iPhone
result: [pending]

### 9. Decision on deferred review finding M-05 (window-share key gate versus global hotkeys and non-activating panels such as Alfred or Raycast) and on the deferred lows L-01 to L-06 and infos I-01 and I-02
expected: The owner accepts these as follow-ups, or asks for a gap-closure plan. M-05 is the one that matters: after Option+Space opens a launcher panel, keys can pass the window gate and land in a panel the window capture does not show. Clicks stay confined (the topmost hit test is unaffected). The blocklist covers only Cmd+Tab, Cmd+Space and Ctrl+Cmd+Q.
result: [pending]

## Summary

total: 9
passed: 0
issues: 0
pending: 9
skipped: 0
blocked: 0

## Gaps
