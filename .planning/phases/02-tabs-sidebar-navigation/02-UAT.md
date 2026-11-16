---
status: testing
phase: 02-tabs-sidebar-navigation
source: [02-VERIFICATION.md]
started: 2026-11-16T01:30:00Z
updated: 2026-11-16T01:30:00Z
---

## Current Test

number: 1
name: Sidebar vibrancy and window chrome on real hardware (Light and Dark)
expected: |
  On your Mac, in Light and then Dark, check four things:
  - the sidebar is translucent, so content behind the window shows through blurred
  - the selected row turns grey when another app is in front
  - a single tab pill is no wider than about 220px
  - long session names end in an ellipsis
awaiting: user response

## Tests

### 1. Sidebar vibrancy and window chrome on real hardware (Light and Dark)
expected: Translucent sidebar showing blurred content behind it. Grey selected row when the window is inactive. Single tab pill at most 220px. Long names truncated with an ellipsis.
result: [pending]

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
passed: 0
issues: 0
pending: 5
skipped: 0
blocked: 0

## Gaps
