---
created: 2026-11-19
title: No API-compatibility check after "Not Now" keeps an older daemon (08-06 IN-03)
area: daemon
resolves_phase: null
---

## Problem
After the DEBT-08 dialog's "Not Now", a newer GUI talks to an older daemon and nothing checks API compatibility. It works today, because the routes and payloads haven't changed since v1.0. The first new route or payload change will break the GUI silently.

## Solution sketch
Do it with the v1.2 session handoff: add a protocol/API version to /health and have the GUI show a clear "update needed" state instead of failing calls.
