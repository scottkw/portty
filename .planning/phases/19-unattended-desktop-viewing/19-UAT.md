---
status: testing
phase: 19-unattended-desktop-viewing
source: [19-VERIFICATION.md]
started: 2026-11-26T23:36:12Z
updated: 2026-11-26T23:36:12Z
---

## Current Test

number: 1
name: Daemon-posted lapse notification with the GUI closed, on a notarized build on the office Mac (TESTING.md M-176)
expected: |
  One real notification banner 'Portty desktop needs approval at the Mac', none on recovery
awaiting: user response

## Tests

### 1. Daemon-posted lapse notification with the GUI closed, on a notarized build on the office Mac (TESTING.md M-176)
expected: One real notification banner 'Portty desktop needs approval at the Mac', none on recovery
result: [pending]

### 2. Direct tailnet path between the office and home Macs, and first-frame time on it (M-157, M-173 step 3)
expected: tailscale ping reads direct; no STUN, TURN or cloud packet; first frame within about 2 s
result: [pending]

### 3. Battery display-sleep test with a viewer connected, and the AC system-sleep hold released on battery (DPRM-04)
expected: Display stays awake while a viewer is attached; stream restarts by itself after a wake; Mac sleeps normally on battery with no viewer
result: [pending]

### 4. Retina sharpness at the office display's native size (M-161)
expected: Picture is sharp and fitted to the window
result: [pending]

### 5. iPhone Home Screen link across a daemon restart and across an upgrade
expected: The saved link opens the live desktop after both, with tap-to-start
result: [pending]

### 6. Real 30-day Screen Recording re-approval prompt and capture behaviour while it waits (M-160)
expected: Waiting for approval at the Mac shows on host and viewer and a notification is sent
result: [pending]

### 7. Office Mac display sleep policy, and what a viewer sees across display sleep and wake there (M-159)
expected: display_asleep shown, then automatic restart after the wake
result: [pending]

### 8. FA-1 attribution and a fresh probe on the office Mac (M-163)
expected: In-daemon capture host listed under Portty in Screen Recording; grant read live
result: [pending]

### 9. Office Mac Tailscale mode and the Settings refusal for userspace networking (M-164)
expected: Network-extension mode accepted; userspace mode refused with a clear state
result: [pending]

### 10. Look and feel of the Desktop group, setup sheet, tray line, 256 x 160 capture box and viewer in real windows (M-179), including the setup sheet's how-to wording with a permission turned off
expected: Matches 19-UI-SPEC and the macOS-native feel
result: [pending]

### 11. Repeat the F5 upgrade with a viewer attached on the office Mac and read the shell tick gap
expected: Every shell keeps its PID and history; tick gap at most 3 s
result: [pending]

## Summary

total: 11
passed: 0
issues: 0
pending: 11
skipped: 0
blocked: 0

## Gaps
