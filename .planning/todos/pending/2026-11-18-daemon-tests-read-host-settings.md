---
created: 2026-11-18T14:00:00Z
title: TestAPIAppearance_GetPatch and TestAPITerminalFont_GetPatch read the host's real settings.json
area: testing
severity: minor
files:
  - internal/daemon/api_test.go
---

## Problem
`testDaemon` in internal/daemon/api_test.go doesn't isolate appearance and terminal-font fields, so the host's `~/Library/Application Support/portty/settings.json` (e.g. appearance dark, font 16) leaks in and both tests fail on a developer Mac. Every gate currently skips them with `-skip '^(TestAPIAppearance_GetPatch|TestAPITerminalFont_GetPatch)$'`.

## Solution
Point the test daemon's config dir at t.TempDir() (or reset those fields in testDaemon), then drop the skip from TESTING.md and the gates.
