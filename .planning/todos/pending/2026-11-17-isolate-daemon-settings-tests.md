---
created: 2026-11-17T20:32:19Z
title: Isolate daemon settings tests from the real config dir
area: testing
severity: major
files:
  - internal/daemon
---

## Problem

TestAPIAppearance_GetPatch and TestAPITerminalFont_GetPatch read the user's real ~/Library/Application Support/portty/settings.json (found 04-08), and the full suite prints 'daemon already running at' the real socket path. They are PATCH tests, so they may also write the real file. Skipped by name in 04-10's gate.

## Solution

Point testDaemon at a temp config dir for settings and the socket (ConfigDirForTest exists), then drop the -skip from gates.
