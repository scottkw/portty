---
created: 2026-11-21
source: Phase 10 RF-03 VM re-run (10-VM-CHECKS.md), seen outside the PASS bar
area: frontend / peer tabs
---

## Problem
1. Clicking an already-ended peer tab makes one reconnect attempt; the host answers `no_such_session`, which `peerConnectVerdict` maps to share-stopped, so the tab's "Session ended — the owner stopped this session." banner is replaced by "<host> stopped sharing this session." The session was ended, not unshared.
2. For a few seconds after a session is killed on the host, the home sidebar shows "Can't reach this Mac…" under the host although it is reachable with 0 sessions.

## Solution
1. Once a tab has received the session-ended close, keep its ended state: don't redial on click (or map `no_such_session` to "ended" when the tab already saw "session ended").
2. Find why the peer list errors briefly after a kill (likely the ListSessions race during teardown) and keep the last good state for a short grace.
