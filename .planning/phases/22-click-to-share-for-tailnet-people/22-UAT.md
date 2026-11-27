---
status: testing
phase: 22-click-to-share-for-tailnet-people
source: [22-12-SUMMARY.md, 22-VM-CHECKS.md, 22-13-SUMMARY.md, TESTING.md M-206..M-220]
started: 2026-11-27T00:00:00Z
updated: 2026-11-27T00:00:00Z
---

Deferred human items from Phase 22 execution. The 22-12 checkpoint (VM permissions, second Tailscale user, real-Mac items) was answered by the orchestrator on the user's behalf: SC1 skipped, all real-Mac items deferred. Each item matches a TESTING.md Section 5 row.

## Current Test

number: 1
name: M-206 SC1 live two-account guest flow
expected: |
  A second Tailscale user opens the guest link, waits at Admit, watches view-only after Admit, and is still refused a terminal link from the same device
awaiting: user response

## Tests

### 1. M-206 SC1 live two-account guest flow (DSHR-03)
expected: A second Tailscale user opens the guest link, waits at "Waiting for <Mac> to let you in", watches view-only after Admit, and gets the own-devices refusal on a terminal link from the same device
result: [pending]

### 2. M-207 AR-21-01 own-account peer filter on a real tailnet
expected: A node shared in from another account shows no Desktop row and no terminal peer in the Portty app; a hand-typed connect to it is refused (unknown_host)
result: [pending]

### 3. M-208 Restrictive tailnet ACL
expected: With an ACL that blocks tcp/udp 7444 to the guest, the guest sees "Can't reach this Mac" (README guidance holds)
result: [pending]

### 4. M-209 Tray sharing icon in light and dark menu bars
expected: The sharing icon variant is legible beside the other tray icons in both appearances (VM G-08 covered light only)
result: [pending]

### 5. M-210 Admit sheet while typing in a real terminal
expected: The Admit sheet arrives while you type; no stray key answers it; the 600 ms guard feels right
result: [pending]

### 6. M-211 Right-to-left or look-alike display name on the real Admit sheet
expected: The name renders isolated (bdi), the login is shown, nothing spoofs the layout
result: [pending]

### 7. M-212 "Someone is using this Mac" with real hardware input
expected: Typing or moving the real mouse while a guest controls drops remote input and the guest sees "Someone is using this Mac"
result: [pending]

### 8. M-213 Warp check and real Secure Keyboard Entry (completes RESEARCH Appendix B)
expected: The real mouse moved right after a remote pointer move registers locally; with Secure Keyboard Entry on, the Secure Event Input banner shows on the Mac while remote keys still post
result: [pending]

### 9. M-214 M-05 window-share key gate with a real non-activating panel
expected: With a window shared to a controlling viewer, opening an Alfred/Raycast-style panel locally makes remote keys drop (counted window_key_focus) and nothing types into the panel; closing it restores typing into the shared window (VM G-11 PARTIAL: keys dropped via the raising check)
result: [pending]

### 10. M-215 Stop Sharing with two real guests
expected: Both guests see "Sharing has ended" within about 2 s on a quiet machine (VM G-04 PARTIAL: one 6.9 s straggler under host load 36-55)
result: [pending]

### 11. M-216 Admit sheet stacking, window forward on a knock, banner click routing
expected: The Admit sheet sits above Share Desktop and Settings; a knock brings Portty forward; a notification click opens the sheet only when a share is active and nothing is pending
result: [pending]

### 12. M-217 Join, leave and admit banners while the display is captured
expected: Banners are delivered (or consistently suppressed) during capture; coalescing within 5 s works
result: [pending]

### 13. M-218 Share stopped banner after the daemon dies; upgrade handoff with a share
expected: Killing the daemon mid-share shows the Share stopped banner; an upgrade handoff ends the share cleanly
result: [pending]

### 14. M-219 Quit GUI Only while sharing; GUI crash
expected: Quit GUI Only ends the share, clears the tray indicator and hides the window; a GUI crash ends the share (gui_gone) with no stale tray state on relaunch
result: [pending]

### 15. M-220 VoiceOver on the Admit sheet and grant switches
expected: VoiceOver reads the Admit sheet name, login and device, and the Allow control / Allow clipboard switches with their labels
result: [pending]

## Summary

total: 15
passed: 0
issues: 0
pending: 15
skipped: 0
