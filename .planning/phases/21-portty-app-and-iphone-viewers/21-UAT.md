---
status: testing
phase: 21-portty-app-and-iphone-viewers
source: [21-VERIFICATION.md]
started: 2026-11-27T16:51:36Z
updated: 2026-11-27T16:51:36Z
---

## Current Test

number: 1
name: M-194 iPhone one-finger trackpad, tap, double-tap, press-and-hold right click with no iOS callout
expected: |
  Pointer moves like a trackpad; tap clicks; double-tap double-clicks and selects a word; long press right-clicks with no callout
awaiting: user response

## Tests

### 1. M-194 iPhone one-finger trackpad, tap, double-tap, press-and-hold right click with no iOS callout
expected: Pointer moves like a trackpad; tap clicks; double-tap double-clicks and selects a word; long press right-clicks with no callout
result: pass (2026-11-27, iPhone 16 Pro, Chrome for iOS, office VM as the shared Mac)

### 2. M-195 iPhone tap-then-drag a window, two-finger scroll direction
expected: Window moves by its title bar; two fingers scroll in the direction the fingers move
result: pass (2026-11-27, build v23.8.2 = 6ea4e35e, office VM): tap-then-drag moves the window (fixed in fecaa066: stale cursor report + rest-before-move); two-finger scroll passes (an open Fonts panel had absorbed it earlier)

### 3. M-196 iPhone pinch to 5x, pan-follow, reset, no Safari tab overview
expected: Pinch zooms to 5x and back; two-finger double-tap and Fit to Screen reset; Safari tab overview never opens
result: pass (2026-11-27, Chrome for iOS)

### 4. M-197 iPhone soft keyboard, Backspace, Return, armed modifiers
expected: Typing, Backspace and Return work; an armed modifier sends a chord; the form-assist bar gap is the accepted one
result: pass (2026-11-27, build v23.8.2): key row appears every time, Cmd then a selects all, Esc/Tab/arrow keep or restore the bar (fixes ddcc2b48, 6ea4e35e). Earlier run failed: see debug/resolved/phone-keyrow-*.md

### 5. M-198 iPhone Send Keys, Lock Screen confirm, surface picker
expected: Cmd+Tab goes to the Mac; Lock Screen asks first; picker lists the office Mac's displays
result: pass (2026-11-27): Cmd+Tab reached the Mac; Lock Screen asked first; picker opened (VM has one display)

### 6. M-199 iPhone sleep and resume
expected: After 2 minutes asleep the picture comes back (Reconnecting, or the Tap to Start card only when iOS needs the tap)
result: pass (2026-11-27): picture came back after 2 minutes asleep

### 7. M-200 iPhone Home Screen relaunch and CSP
expected: Icon keeps the same link; Safari console shows no CSP violation with the video element
result: pass for the Safari Home Screen web app (2026-11-27); CSP console check not run. Finding: a Chrome for iOS Home Screen shortcut opened black; the VM saw only TLS handshakes reset by the phone, no page load

### 8. M-201 two real Macs: Desktop row, Local Network prompt, sharp picture, direct tailnet path
expected: Row opens the office Mac in a tab; first-run prompt appears and Allow heals the tab; picture is sharp; path is direct
result: [pending]

### 9. M-202 control mode on real Macs: Cmd+W, Cmd+Q, Cmd+C, release chord, plain click after
expected: Chords act on the office Mac only; Ctrl+Option+Cmd+Shift+Return gives Portty its shortcuts back; the next click is a plain click (VM K6 saw a Control key held after the release chord once, not reproduced, suspected rig artefact)
result: [pending]

### 10. M-203 Cmd+V in control mode including a password-manager item
expected: Home clipboard text pastes into the office Mac; a concealed item is refused with the hidden-item notice; no macOS paste alert
result: [pending]

### 11. M-204 hidden window lets the office display sleep after about 2 minutes, on battery and on power
expected: Display sleeps about 2 minutes after the window is hidden; reopening brings the picture back
result: [pending]

### 12. M-205 label, chips and cards legible in Light and Dark over the translucent window
expected: Toolbar label, chip and cards legible in both appearances
result: [pending]

### 13. Re-check the five code-review fixes WR-01..WR-05 in the real app
expected: Refused share port retries instead of showing the Desktop sharing is off card; Cmd+V after control ends sends nothing; the bar Paste to Mac refuses a concealed item; a later rights downgrade removes Control; closing a Desktop tab clears its sidebar line
result: [pending]

## Summary

total: 13
passed: 0
issues: 0
pending: 13
skipped: 0
blocked: 0

## Gaps
