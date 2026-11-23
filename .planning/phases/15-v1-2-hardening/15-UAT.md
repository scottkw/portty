---
status: testing
phase: 15-v1-2-hardening
source: [15-VERIFICATION.md]
started: 2026-11-23T17:00:00Z
updated: 2026-11-23T17:00:00Z
---

## Current Test

number: 1
name: M-152 notification click on a notarized build
expected: |
  Window comes forward with 'Portty X is available.' and Download; no download starts until Download is pressed; the post-relaunch click also works.
awaiting: user response

## Tests

### 1. M-152: click the 'Portty X is available' notification on a notarized build in Tart VM portty-test; then dismiss the banner, quit, relaunch, click the notification left in Notification Center
expected: Window comes forward with 'Portty X is available.' and Download; no download starts until Download is pressed; the post-relaunch click also works (covers review IN-05)
result: [pending]

### 2. M-153: attach to a session from a real terminal, run vim then Claude Code, resize, open the same session in a taller GUI tab; record the passive-watch case
expected: Nothing draws on the bar row once the CLI has typed or resized; `tput lines` = terminal rows - 1 after typing in the CLI; --status-top overdraw is the accepted limit
result: [pending]

### 3. M-154: over the two-VM tailnet, end a shared session, switch away and back to the ended peer tab; sample the sidebar for 40 s after the last share ends; then pf-drop the host
expected: Ended tab reads 'Session ended', never 'stopped sharing'; host row reads 'Nothing shared', never 'Can't reach this Mac'; with pf dropping, the can't-reach line shows
result: [pending]

### 4. M-155: TestHandoffIT (upgrade, fault, from-v1.1) in portty-test, plus the pre-phase daemon upgraded twice (socket 755, then 600, then 600; no 'does not match the socket peer' in daemon.log)
expected: All pass; HELLO-OK pid check accepts a genuine old daemon across real processes including a second upgrade; a killed takeover leaves A serving; socket tightened to 0600. Ship blocker per TESTING.md.
result: [pending]

## Summary

total: 4
passed: 0
issues: 0
pending: 4
skipped: 0
blocked: 0

## Gaps
