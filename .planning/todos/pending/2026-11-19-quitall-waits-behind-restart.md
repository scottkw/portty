---
created: 2026-11-19
title: Quit All can wait up to ~30 s behind an approved daemon restart (08-06 IN-07)
area: app
resolves_phase: null
audit_acknowledged:
  milestone: v1.1
  at: 2026-11-19
---

## Problem
If the user approves Restart Now and then chooses Quit All, Quit All waits on the restart lock (up to about 30 s), then stops the daemon the restart just started. The window can sit on "Restarting…" in the meantime.

## Solution sketch
Give EnsureDaemon a cancellation hook that every caller passes, so Quit All can cancel an in-flight restart.
