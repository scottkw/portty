---
status: testing
phase: 02-tabs-sidebar-navigation
source: [02-VERIFICATION.md]
started: 2026-11-16T01:30:00Z
updated: 2026-11-16T07:55:00Z
---

## Current Test

number: 2
name: Peer attach from a second Mac (M-03, M-04, M-05, M-13)
awaiting: user response (needs both Macs)

## Tests

### 1. Sidebar vibrancy and window chrome on real hardware (Light and Dark)
expected: Translucent sidebar showing blurred content behind it. Grey selected row when the window is inactive. Single tab pill at most 220px. Long names truncated with an ellipsis.
result: pass
note: 2026-11-16 host full-screen capture (Dark, Tailscale connected) shows the Sidebar material blurring the wallpaper after quick task 261115-rc8. Traffic lights stay grey on the host, but Finder's do too, so it is a host-wide setting and not Portty; the same build shows coloured lights in the VM. The 220px tab cap and ellipsis are in style.css (.tab max-width, .tab__label text-overflow) and were checked in the VM.

### 2. Peer attach from a second Mac (M-03, M-04, M-05, M-13)
expected: The other Mac's shared sessions appear in its own sidebar group.
- The first open uses a join code.
- Reopening uses the saved capability.
- A sleeping peer, or one that shares nothing, shows a calm state.
- With Tailscale off, the peer group shows the muted line.
- Two peer tabs stay separate.
result: [pending]

### 3. Share dialog and Tailscale live state (M-01, M-57)
expected: With Tailscale connected, the toolbar Share button opens the Share dialog and the Funnel consent gate is reachable. Turning Tailscale off and on updates the banner, dims Share, and shows the reason in Settings, live.
result: [pending]

### 4. Web share page ignores the desktop Appearance override
expected: With the desktop set to Dark and the browser set to Light, a shared session's web page renders Light, following the browser.
result: [pending]

### 5. VoiceOver
expected:
- The dimmed Share and Start Server buttons are read together with their reasons.
- The active sidebar row is announced as current.
- Sheets announce their title and keep focus.
result: [pending]

## Summary

total: 5
passed: 1
issues: 0
pending: 4
skipped: 0
blocked: 0

## Gaps
