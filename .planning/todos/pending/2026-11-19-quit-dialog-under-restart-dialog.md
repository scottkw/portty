---
created: 2026-11-19
title: Cmd-Q while the daemon restart dialog is open draws the Quit dialog underneath it
area: frontend
resolves_phase: null
---

## Problem
Found in VM G-04 (2026-11-19, dcbe999d): pressing Cmd-Q while the DEBT-08 "Update Portty's background service?" dialog is open draws the Quit dialog underneath it, at the same position. Clicking Quit GUI Only then does nothing. The restart dialog stays usable.

## Solution sketch
Only one modal at a time: either queue the Quit dialog until the restart dialog closes, or let Cmd-Q answer "Not Now" and then show Quit. Add a vitest.
