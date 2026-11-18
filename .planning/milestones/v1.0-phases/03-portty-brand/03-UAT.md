---
status: testing
phase: 03-portty-brand
source: [03-VERIFICATION.md]
started: 2026-11-16T20:00:00Z
updated: 2026-11-16T20:00:00Z
audit_acknowledged:
  milestone: v1.0
  at: 2026-11-18
  gap_snapshot: "testing::scenarios=3"
---

## Current Test

number: 1
name: Two-Mac peer discovery on port 7444 (M-70)
awaiting: user response (needs both Macs on the tailnet, each running this build)

## Tests

### 1. Two-Mac peer discovery on port 7444 (M-70)
expected: Each Portty lists the other under Other Macs (the probe on 7444 sees X-Portty: 1). A Mac running only the old app is not listed. Both apps run side by side on one Mac without a port collision.
result: [pending]

### 2. Notifications under the Portty name (M-73)
expected: With a Developer-ID-signed Portty.app, Quit GUI Only with 0, 1 and 2 sessions shows a Portty-attributed notification with the Portty icon and the 0/1/n body copy; first use asks permission for Portty.
result: [pending]

### 3. README logo on GitHub (M-74)
expected: github.com/scottkw/portty shows the light logo in the light theme and the dark logo in the dark theme, 400 px wide, alt text Portty. Needs the repo to be viewable (private until Phase 6).
result: [pending]

## Summary

total: 3
passed: 0
issues: 0
pending: 3
skipped: 0
blocked: 0

## Gaps
