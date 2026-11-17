---
status: testing
phase: 04-one-click-share
source: [04-VERIFICATION.md]
started: 2026-11-17T07:00:00Z
updated: 2026-11-17T07:00:00Z
---

## Current Test

number: 1
name: Two-Mac share flow (M-80)
awaiting: user response (needs both Macs on the tailnet, each running this build)

## Tests

### 1. Two-Mac share flow (M-80)
expected: Share on the office Mac shows the session in the home Mac's sidebar; one click opens it with no code; the browser link works from another tailnet device; from Warp, `portty attach <host>:<session>` shows the status bar, and bare `portty attach <host>` attaches (one share) or lists (several); Share off shows the stop banner in the GUI tab and browser, removes the sidebar row, and the CLI prints "Portty: <host> stopped sharing <session>." and exits 0; re-share then clicking the row reconnects to the right Mac (WR-05/WR-06); a Mac on another account sees nothing. Check `tailscale status` shows no tags on either Mac.
result: [pending]

### 2. No lingering public serve config (M-77)
expected: `tailscale serve status` prints "No serve config" on both Macs (run `tailscale serve reset` if anything is listed). Closes T-04-01.
result: [pending]

### 3. Share controls follow real Tailscale (M-82)
expected: With real Tailscale disconnected, Share is dimmed with its reason and Stop Sharing still works; reconnecting re-enables it.
result: [pending]

## Summary

total: 3
passed: 0
issues: 0
pending: 3
skipped: 0
blocked: 0

## Gaps
